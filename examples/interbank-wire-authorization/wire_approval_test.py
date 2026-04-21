"""Tests for wire_approval.py. No Quidnug node required."""

import pytest

from wire_approval import (
    ApprovalTier,
    NonceLedger,
    WireApproval,
    WireInstruction,
    WirePolicy,
    WireSigner,
    WireVerdict,
    evaluate_wire,
    extract_approvals,
    receiver_verify,
)


def _policy(frozen: bool = False) -> WirePolicy:
    return WirePolicy(
        bank_quid="bank-us",
        signers=[
            WireSigner("alice", weight=1, role="officer",     current_epoch=0),
            WireSigner("bob",   weight=1, role="officer",     current_epoch=0),
            WireSigner("dave",  weight=1, role="officer",     current_epoch=0),
            WireSigner("carol", weight=2, role="compliance",  current_epoch=0),
        ],
        tiers=[
            # Tier 1: $0 - $1,000 -- single officer.
            ApprovalTier(
                min_amount_cents=0,
                max_amount_cents=100_000_00,
                required_weight=1,
            ),
            # Tier 2: $1,000,000 - $10,000,000 -- two officers.
            ApprovalTier(
                min_amount_cents=100_000_00,
                max_amount_cents=1_000_000_00,
                required_weight=2,
            ),
            # Tier 3: above $10M -- compliance required.
            ApprovalTier(
                min_amount_cents=1_000_000_00,
                max_amount_cents=0,
                required_weight=3,
                required_roles=["compliance"],
            ),
        ],
        frozen=frozen,
    )


def _wire(wire_id: str = "w-1", amount_cents: int = 50_000_00) -> WireInstruction:
    return WireInstruction(
        wire_id=wire_id,
        sender_bank_quid="bank-us",
        receiver_bank_quid="bank-counterparty",
        amount_cents=amount_cents,
        currency="USD",
        beneficiary_account="0x...",
        reference="Q2-supplier-payment",
        proposed_at_unix=1_700_000_000,
    )


def _approval(signer: str, wire_id: str = "w-1", nonce: int = 1) -> WireApproval:
    return WireApproval(
        signer_quid=signer, wire_id=wire_id,
        signer_nonce=nonce, signer_epoch=0,
        approved_at_unix=1_700_000_000,
    )


# ---------------------------------------------------------------------------
# Tier routing
# ---------------------------------------------------------------------------

def test_tier1_one_officer_approves_small_wire():
    policy = _policy()
    # $500 -- tier 1, weight 1.
    v = evaluate_wire(
        _wire(amount_cents=500_00), policy,
        [_approval("alice", nonce=1)],
    )
    assert v.verdict == "approved"
    assert v.collected_weight == 1


def test_tier2_needs_two_officers():
    policy = _policy()
    # $5M -- tier 2, weight 2.
    w = _wire(amount_cents=500_000_00)
    one = evaluate_wire(w, policy, [_approval("alice", w.wire_id, 1)])
    two = evaluate_wire(
        w, policy,
        [_approval("alice", w.wire_id, 1), _approval("bob", w.wire_id, 1)],
    )
    assert one.verdict == "pending"
    assert two.verdict == "approved"


def test_tier2_compliance_alone_approves_on_weight():
    """Compliance officer has weight=2, so a single signature
    from carol satisfies tier 2."""
    policy = _policy()
    w = _wire(amount_cents=500_000_00)
    v = evaluate_wire(w, policy, [_approval("carol", w.wire_id, 1)])
    assert v.verdict == "approved"
    assert v.collected_weight == 2


def test_tier3_requires_compliance_role():
    """$50M wire: two officers' weight=2 is not enough because
    tier 3 explicitly requires role=compliance."""
    policy = _policy()
    w = _wire(amount_cents=5_000_000_00)
    # 3 regular officers -- total weight 3, meets weight threshold.
    # But no compliance role -> pending.
    v = evaluate_wire(
        w, policy,
        [
            _approval("alice", w.wire_id, 1),
            _approval("bob",   w.wire_id, 1),
            _approval("dave",  w.wire_id, 1),
        ],
    )
    assert v.verdict == "pending"
    assert "required role" in v.reasons[0].lower()


def test_tier3_compliance_plus_one_officer_approves():
    policy = _policy()
    w = _wire(amount_cents=5_000_000_00)
    v = evaluate_wire(
        w, policy,
        [
            _approval("alice", w.wire_id, 1),
            _approval("carol", w.wire_id, 1),
        ],
    )
    # weight = 1 (alice) + 2 (carol) = 3 -> meets weight 3
    # compliance role present -> approved.
    assert v.verdict == "approved"


# ---------------------------------------------------------------------------
# Replay / nonces
# ---------------------------------------------------------------------------

def test_nonce_replay_rejected_on_same_wire():
    """If a signer's nonce reuses a value, the second approval
    is not counted."""
    policy = _policy()
    w = _wire(amount_cents=500_000_00)
    ledger = NonceLedger()
    # First approval with nonce 1.
    evaluate_wire(w, policy, [_approval("alice", w.wire_id, 1)], ledger)
    # Re-submitted with nonce 1 again: should not count toward any wire.
    v = evaluate_wire(
        _wire("w-2", amount_cents=500_000_00), policy,
        [_approval("alice", "w-2", 1)], ledger,
    )
    # Alice's approval gets rejected as replay. Verdict depends on
    # whether we have other approvals -- we don't here.
    assert v.collected_weight == 0
    # Audit entry shows the replay rejection.
    alice_audit = [a for a in v.audit if a["signer"] == "alice"][0]
    assert alice_audit["counted"] is False
    assert "replay" in alice_audit["reason"].lower()


def test_nonce_monotonic_accept_allows_higher():
    policy = _policy()
    ledger = NonceLedger()
    # First approval at nonce 3 ...
    evaluate_wire(
        _wire("w-A", amount_cents=500_000_00), policy,
        [_approval("alice", "w-A", 3)], ledger,
    )
    # ... later approval at nonce 7 is fine.
    v = evaluate_wire(
        _wire("w-B", amount_cents=500_000_00), policy,
        [_approval("alice", "w-B", 7), _approval("bob", "w-B", 1)], ledger,
    )
    assert v.verdict == "approved"


# ---------------------------------------------------------------------------
# Edge cases
# ---------------------------------------------------------------------------

def test_non_policy_signer_ignored():
    policy = _policy()
    w = _wire(amount_cents=500_00)
    v = evaluate_wire(
        w, policy,
        [
            _approval("mallory", w.wire_id, 1),  # impostor
            _approval("alice", w.wire_id, 1),
        ],
    )
    # Only alice counts -> tier 1 -> approved.
    assert v.verdict == "approved"
    assert v.collected_weight == 1
    mallory_audit = [a for a in v.audit if a["signer"] == "mallory"][0]
    assert mallory_audit["counted"] is False


def test_frozen_bank_denies():
    policy = _policy(frozen=True)
    w = _wire(amount_cents=500_000_00)
    v = evaluate_wire(
        w, policy,
        [_approval("alice", w.wire_id, 1), _approval("bob", w.wire_id, 1)],
    )
    assert v.verdict == "denied"


def test_duplicate_signer_dedup():
    policy = _policy()
    w = _wire(amount_cents=500_000_00)
    v = evaluate_wire(
        w, policy,
        [_approval("alice", w.wire_id, 1),
         _approval("alice", w.wire_id, 2)],  # alice re-signs
    )
    # Only the first alice is counted; below tier 2 threshold.
    assert v.verdict == "pending"
    assert v.collected_weight == 1


def test_out_of_band_wire_approvals_ignored():
    policy = _policy()
    w = _wire("w-target", amount_cents=500_000_00)
    v = evaluate_wire(
        w, policy,
        [
            _approval("alice", "w-other", 1),    # different wire
            _approval("bob",   "w-target", 1),
        ],
    )
    # Only bob counts against w-target -> pending.
    assert v.verdict == "pending"


def test_no_matching_tier_denies():
    """A policy with no tier covering the amount denies."""
    policy = WirePolicy(
        bank_quid="bank-us",
        signers=[WireSigner("alice", 1, "officer")],
        tiers=[ApprovalTier(min_amount_cents=0, max_amount_cents=10_00,
                             required_weight=1)],
    )
    w = _wire(amount_cents=1_000_00)   # $1000 -- above the single tier
    v = evaluate_wire(w, policy, [_approval("alice", w.wire_id, 1)])
    assert v.verdict == "denied"
    assert "no policy tier" in v.reasons[0]


# ---------------------------------------------------------------------------
# Counterparty verification
# ---------------------------------------------------------------------------

def test_receiver_sees_same_verdict():
    policy = _policy()
    w = _wire(amount_cents=500_000_00)
    approvals = [_approval("alice", w.wire_id, 1),
                  _approval("bob", w.wire_id, 1)]
    sender = evaluate_wire(w, policy, approvals)
    receiver = receiver_verify(w, policy, approvals)
    assert sender.verdict == receiver.verdict == "approved"
    assert sender.collected_weight == receiver.collected_weight


# ---------------------------------------------------------------------------
# Stream extraction
# ---------------------------------------------------------------------------

def test_extract_approvals_from_stream():
    events = [
        {
            "eventType": "wire.proposed",
            "payload": {"wireId": "w-1", "amountCents": 500_000_00},
        },
        {
            "eventType": "wire.cosigned",
            "payload": {"signerQuid": "alice", "wireId": "w-1",
                        "signerNonce": 5, "signerEpoch": 0},
        },
        {
            "eventType": "wire.cosigned",
            "payload": {"signerQuid": "bob", "wireId": "w-1",
                        "signerNonce": 3, "signerEpoch": 0},
        },
    ]
    approvals = extract_approvals(events)
    assert len(approvals) == 2
    assert {a.signer_quid for a in approvals} == {"alice", "bob"}
    assert approvals[0].signer_nonce == 5
