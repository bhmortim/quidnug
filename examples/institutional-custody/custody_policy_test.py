"""Tests for custody_policy.py. No Quidnug node required."""

import pytest

from custody_policy import (
    SignerApproval,
    SignerConfig,
    TransferAuthorization,
    TransferVerdict,
    WalletPolicy,
    audit_report,
    evaluate_transfer,
    extract_approvals,
    stale_epoch_signers,
    wallet_frozen_by_events,
)


NOW = 1_700_000_000


def _policy(
    threshold: int = 5, n: int = 7, frozen: bool = False,
) -> WalletPolicy:
    return WalletPolicy(
        wallet_quid="wallet-cold-btc-001",
        threshold=threshold,
        signers=[
            SignerConfig(f"signer-{i}", current_epoch=i % 3, role=f"signer-{i}")
            for i in range(1, n + 1)
        ],
        frozen=frozen,
    )


def _transfer(transfer_id: str = "tx-1") -> TransferAuthorization:
    return TransferAuthorization(
        transfer_id=transfer_id,
        wallet_quid="wallet-cold-btc-001",
        target_chain="bitcoin",
        target_address="bc1qexample",
        amount_units=1_000_000_00,
        currency="BTC",
        proposed_at_unix=NOW,
        purpose="monthly rebalance",
    )


def _approval(i: int, epoch: Optional[int] = None, tid: str = "tx-1") -> SignerApproval:
    signer_id = f"signer-{i}"
    if epoch is None:
        epoch = i % 3  # matches the policy's current_epoch for each signer
    return SignerApproval(
        signer_quid=signer_id,
        transfer_id=tid,
        signer_epoch=epoch,
        approved_at_unix=NOW,
    )


# typing.Optional placeholder for test fixture -- keeps the helper clean.
try:
    from typing import Optional
except ImportError:  # pragma: no cover
    Optional = None  # type: ignore


# ---------------------------------------------------------------------------
# Basic flow
# ---------------------------------------------------------------------------

def test_insufficient_approvals_is_pending():
    policy = _policy(threshold=5)
    approvals = [_approval(i) for i in range(1, 4)]   # 3 of 5 needed
    v = evaluate_transfer(_transfer(), policy, approvals, now_unix=NOW)
    assert v.verdict == "pending"
    assert v.collected_weight == 3
    assert v.required_weight == 5


def test_threshold_met_authorizes():
    policy = _policy(threshold=5)
    approvals = [_approval(i) for i in range(1, 6)]   # 5
    v = evaluate_transfer(_transfer(), policy, approvals, now_unix=NOW)
    assert v.verdict == "authorized"
    assert v.collected_weight == 5


def test_over_threshold_also_authorizes():
    policy = _policy(threshold=5)
    approvals = [_approval(i) for i in range(1, 8)]   # all 7
    v = evaluate_transfer(_transfer(), policy, approvals, now_unix=NOW)
    assert v.verdict == "authorized"
    assert v.collected_weight == 7


# ---------------------------------------------------------------------------
# Signer constraints
# ---------------------------------------------------------------------------

def test_non_policy_signer_ignored():
    policy = _policy(threshold=5)
    approvals = [_approval(i) for i in range(1, 5)] + [
        SignerApproval("impostor", "tx-1", 0, NOW),
    ]
    v = evaluate_transfer(_transfer(), policy, approvals, now_unix=NOW)
    # 4 legit approvals, impostor ignored -> pending.
    assert v.verdict == "pending"
    assert v.collected_weight == 4
    # Audit should record that the impostor was rejected.
    impostor = [a for a in v.audit if a["signer"] == "impostor"][0]
    assert impostor["counted"] is False
    assert "not in policy" in impostor["reason"]


def test_duplicate_signer_counts_once():
    policy = _policy(threshold=5)
    approvals = [_approval(i) for i in range(1, 5)] + [
        _approval(1, tid="tx-1"),   # re-signs
        _approval(1, tid="tx-1"),
    ]
    v = evaluate_transfer(_transfer(), policy, approvals, now_unix=NOW)
    # 4 uniques, dup from signer-1 ignored.
    assert v.verdict == "pending"
    assert v.collected_weight == 4


def test_stale_epoch_rejected_by_default():
    """A signature at epoch older than signer's current_epoch is
    rejected unless the policy explicitly allows an old-epoch
    grace."""
    policy = _policy(threshold=2)
    # signer-1's current_epoch is 1 in the fixture.
    bad = SignerApproval("signer-1", "tx-1", signer_epoch=0, approved_at_unix=NOW)
    good = _approval(2)
    v = evaluate_transfer(_transfer(), policy, [bad, good], now_unix=NOW)
    assert v.verdict == "pending"
    assert v.collected_weight == 1
    # audit mentions the stale epoch.
    stale = [a for a in v.audit if a["signer"] == "signer-1"][0]
    assert stale["counted"] is False
    assert "epoch" in stale["reason"]


def test_stale_epoch_accepted_in_grace_window():
    """With max_accepted_old_epoch set, older epochs are accepted
    up to that value. Models the AnchorRotation grace behavior."""
    policy = WalletPolicy(
        wallet_quid="w",
        threshold=2,
        signers=[
            SignerConfig("signer-1", current_epoch=2,
                          max_accepted_old_epoch=1),
            SignerConfig("signer-2", current_epoch=0),
        ],
    )
    approvals = [
        SignerApproval("signer-1", "tx-1", 1, NOW),  # within grace
        SignerApproval("signer-2", "tx-1", 0, NOW),
    ]
    v = evaluate_transfer(_transfer(), policy, approvals, now_unix=NOW)
    assert v.verdict == "authorized"


def test_future_epoch_rejected():
    """Signer claiming to sign at a future epoch is rejected."""
    policy = WalletPolicy(
        wallet_quid="w",
        threshold=1,
        signers=[SignerConfig("signer-1", current_epoch=0)],
    )
    approvals = [SignerApproval("signer-1", "tx-1", 5, NOW)]
    v = evaluate_transfer(_transfer(), policy, approvals, now_unix=NOW)
    assert v.verdict == "pending"
    assert v.collected_weight == 0


# ---------------------------------------------------------------------------
# Frozen wallet
# ---------------------------------------------------------------------------

def test_frozen_wallet_denies_regardless():
    policy = _policy(threshold=5, frozen=True)
    # Full quorum of approvals, but the wallet is frozen.
    approvals = [_approval(i) for i in range(1, 8)]
    v = evaluate_transfer(_transfer(), policy, approvals, now_unix=NOW)
    assert v.verdict == "denied"
    assert "frozen" in v.reasons[0].lower()


# ---------------------------------------------------------------------------
# Cross-transfer pollution
# ---------------------------------------------------------------------------

def test_approvals_for_other_transfers_ignored():
    policy = _policy(threshold=2)
    approvals = [
        _approval(1, tid="tx-1"),
        _approval(2, tid="tx-99"),    # different transfer
        _approval(3, tid="tx-1"),
    ]
    v = evaluate_transfer(_transfer(), policy, approvals, now_unix=NOW)
    assert v.verdict == "authorized"
    assert v.collected_weight == 2


# ---------------------------------------------------------------------------
# Monitoring helpers
# ---------------------------------------------------------------------------

def test_stale_epoch_signers_flags_overdue():
    policy = _policy()
    last_rotation = {
        "signer-1": NOW - 10 * 86400,    # recent
        "signer-2": NOW - 100 * 86400,   # overdue
        "signer-3": NOW - 200 * 86400,   # overdue
    }
    stale = stale_epoch_signers(
        policy, last_rotation, now_unix=NOW, max_age_seconds=90 * 86400,
    )
    assert "signer-2" in stale
    assert "signer-3" in stale
    # signer-4..7 have no recorded rotation at all -> count as overdue.
    assert "signer-4" in stale


# ---------------------------------------------------------------------------
# Event-stream extraction
# ---------------------------------------------------------------------------

def test_extract_approvals_from_stream():
    events = [
        {
            "eventType": "transfer.proposed",
            "payload": {"transferId": "tx-1"},
        },
        {
            "eventType": "transfer.cosigned",
            "payload": {
                "signerQuid": "signer-1",
                "transferId": "tx-1",
                "signerEpoch": 0,
                "approvedAt": NOW,
            },
        },
        {
            "eventType": "transfer.cosigned",
            "payload": {
                "signerQuid": "signer-2",
                "transferId": "tx-1",
                "signerEpoch": 2,
                "approvedAt": NOW,
            },
        },
    ]
    out = extract_approvals(events)
    assert len(out) == 2
    assert {a.signer_quid for a in out} == {"signer-1", "signer-2"}
    assert any(a.signer_epoch == 2 for a in out)


def test_wallet_frozen_detection():
    assert wallet_frozen_by_events([]) is False
    events = [
        {"eventType": "wallet.frozen", "payload": {}},
    ]
    assert wallet_frozen_by_events(events) is True
    events.append({"eventType": "wallet.unfrozen", "payload": {}})
    assert wallet_frozen_by_events(events) is False


# ---------------------------------------------------------------------------
# Audit report
# ---------------------------------------------------------------------------

def test_audit_report_renders():
    policy = _policy(threshold=5)
    approvals = [_approval(i) for i in range(1, 6)]
    v = evaluate_transfer(_transfer(), policy, approvals, now_unix=NOW)
    text = audit_report(v)
    assert "AUTHORIZED" in text
    assert "signer-1" in text
    assert "epoch=" in text
