"""Interbank wire authorization, end-to-end runnable demo.

Flow:
  1. Register actors: sender bank, three officers, one compliance
     officer, one receiving bank.
  2. Install the sender bank's wire policy: three tiers by amount,
     weighted signers, compliance role required for tier 3.
  3. A $500 wire is approved by one officer alone (tier 1).
  4. A $5M wire needs two officers (tier 2); verdict flips from
     pending to approved when the second officer cosigns.
  5. A $50M wire requires a compliance cosignature (tier 3);
     two regular officers alone yields pending; compliance
     officer cosigns and it flips to approved.
  6. Receiving bank runs the same verification on the same
     stream and reaches the same verdict without a phone call.
  7. Replay defense: an attacker re-submits an already-used
     nonce; the evaluator flags it as a replay.

Prerequisites:
  - Local Quidnug node at http://localhost:8080.
  - Python SDK installed.

Run:
    python demo.py
"""

from __future__ import annotations

import os
import sys
import time
import uuid
from dataclasses import dataclass
from typing import List

sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))
from wire_approval import (
    ApprovalTier,
    NonceLedger,
    WireApproval,
    WireInstruction,
    WirePolicy,
    WireSigner,
    evaluate_wire,
    extract_approvals,
    receiver_verify,
)

from quidnug import OwnershipStake, Quid, QuidnugClient

NODE_URL = os.environ.get("QUIDNUG_NODE", "http://localhost:8080")
DOMAIN = "wires.federal.us"


@dataclass
class Actor:
    name: str
    role: str
    quid: Quid


def banner(msg: str) -> None:
    print()
    print("=" * 72)
    print(f"  {msg}")
    print("=" * 72)


def register(client: QuidnugClient, name: str, role: str) -> Actor:
    q = Quid.generate()
    try:
        client.register_identity(
            q, name=name, domain=DOMAIN, home_domain=DOMAIN,
            attributes={"role": role},
        )
    except Exception as e:
        print(f"  (register {name}: {e})")
    return Actor(name=name, role=role, quid=q)


def propose_wire(
    client: QuidnugClient, sender_bank: Actor, wire: WireInstruction,
    officers: List[Actor] = None,
) -> None:
    """Register the wire as a TITLE jointly owned by the sender
    bank and every officer who might cosign; emit the proposal."""
    officers = officers or []
    share = 0.02
    bank_share = round(1.0 - share * len(officers), 6)
    owners = [OwnershipStake(sender_bank.quid.id, bank_share, "sender-bank")]
    for o in officers:
        owners.append(OwnershipStake(o.quid.id, share, o.role))
    try:
        client.register_title(
            signer=sender_bank.quid,
            asset_id=wire.wire_id,
            owners=owners,
            domain=DOMAIN,
            title_type="wire-authorization",
        )
    except Exception as e:
        print(f"  (register_title {wire.wire_id}: {e})")
    client.wait_for_title(wire.wire_id)
    client.emit_event(
        signer=sender_bank.quid,
        subject_id=wire.wire_id,
        subject_type="TITLE",
        event_type="wire.proposed",
        domain=DOMAIN,
        payload={
            "wireId": wire.wire_id,
            "senderBank": wire.sender_bank_quid,
            "receiverBank": wire.receiver_bank_quid,
            "amountCents": wire.amount_cents,
            "currency": wire.currency,
            "beneficiary": wire.beneficiary_account,
            "reference": wire.reference,
            "proposedAt": wire.proposed_at_unix,
        },
    )
    print(f"  {sender_bank.name} proposed {wire.wire_id}")
    print(f"    amount ${wire.amount_cents/100:,.2f} {wire.currency}")
    print(f"    to     bank={wire.receiver_bank_quid[:12]}  acct={wire.beneficiary_account}")


def cosign_wire(
    client: QuidnugClient, signer: Actor, wire_id: str, nonce: int,
    signer_epoch: int = 0,
) -> None:
    client.emit_event(
        signer=signer.quid,
        subject_id=wire_id,
        subject_type="TITLE",
        event_type="wire.cosigned",
        domain=DOMAIN,
        payload={
            "signerQuid": signer.quid.id,
            "wireId": wire_id,
            "signerNonce": nonce,
            "signerEpoch": signer_epoch,
            "approvedAt": int(time.time()),
        },
    )


def load_events(client: QuidnugClient, wire_id: str) -> List[dict]:
    events, _ = client.get_stream_events(wire_id, limit=200)
    out: List[dict] = []
    for ev in events or []:
        out.append({
            "eventType": ev.event_type,
            "payload": ev.payload or {},
            "timestamp": ev.timestamp,
        })
    return out


def evaluate_and_show(
    client: QuidnugClient, policy: WirePolicy, wire: WireInstruction,
    label: str, ledger: NonceLedger = None,
) -> str:
    events = load_events(client, wire.wire_id)
    approvals = extract_approvals(events)
    v = evaluate_wire(wire, policy, approvals, ledger=ledger)
    print(f"\n  [{label}]")
    print(f"    {v.short()}")
    for r in v.reasons:
        print(f"      - {r}")
    if v.counted_signers:
        print(f"      counted: {', '.join(s[:12] for s in v.counted_signers)}")
    return v.verdict


def main() -> None:
    print(f"Connecting to Quidnug node at {NODE_URL}")
    client = QuidnugClient(NODE_URL)
    try:
        client.info()
    except Exception as e:
        print(f"node unreachable: {e}", file=sys.stderr)
        sys.exit(1)

    client.ensure_domain(DOMAIN)

    # -----------------------------------------------------------------
    banner("Step 1: Register actors")
    sender   = register(client, "bank-acme-usa",        "sender-bank")
    receiver = register(client, "bank-counterparty-eu", "receiver-bank")
    alice    = register(client, "alice-officer",        "officer")
    bob      = register(client, "bob-officer",          "officer")
    dave     = register(client, "dave-officer",         "officer")
    carol    = register(client, "carol-compliance",     "compliance")
    for a in (sender, receiver, alice, bob, dave, carol):
        print(f"  {a.role:13s} {a.name:22s} -> {a.quid.id}")
    client.wait_for_identities([a.quid.id for a in
        (sender, receiver, alice, bob, dave, carol)])

    # -----------------------------------------------------------------
    banner("Step 2: Install bank's wire-authorization policy")
    policy = WirePolicy(
        bank_quid=sender.quid.id,
        signers=[
            WireSigner(alice.quid.id, weight=1, role="officer",    current_epoch=0),
            WireSigner(bob.quid.id,   weight=1, role="officer",    current_epoch=0),
            WireSigner(dave.quid.id,  weight=1, role="officer",    current_epoch=0),
            WireSigner(carol.quid.id, weight=2, role="compliance", current_epoch=0),
        ],
        tiers=[
            ApprovalTier(0,                   100_000_00,    required_weight=1),
            ApprovalTier(100_000_00,          1_000_000_00,  required_weight=2),
            ApprovalTier(1_000_000_00,        0,             required_weight=3,
                          required_roles=["compliance"]),
        ],
    )

    # Publish the policy as an event on the sender's stream for auditors.
    client.emit_event(
        signer=sender.quid,
        subject_id=sender.quid.id,
        subject_type="QUID",
        event_type="bank.wire-policy-installed",
        domain=DOMAIN,
        payload={
            "signerCount": len(policy.signers),
            "tiers": [
                {"min": t.min_amount_cents, "max": t.max_amount_cents,
                 "requiredWeight": t.required_weight,
                 "requiredRoles": t.required_roles}
                for t in policy.tiers
            ],
            "installedAt": int(time.time()),
        },
    )
    print("  Tier 1  [$0          - $1,000,000   ]  1 officer")
    print("  Tier 2  [$1,000,000  - $10,000,000  ]  2 officers (weight)")
    print("  Tier 3  [$10,000,000 +              ]  3 weight + compliance role")

    time.sleep(1)

    # Live nonce ledger that mirrors the chain's per-signer highest
    # nonce. In production the node enforces this directly (QDP-0001).
    ledger = NonceLedger()

    # -----------------------------------------------------------------
    banner("Step 3: $500 wire (tier 1)")
    w_small = WireInstruction(
        wire_id=f"w-{uuid.uuid4().hex[:8]}",
        sender_bank_quid=sender.quid.id,
        receiver_bank_quid=receiver.quid.id,
        amount_cents=500_00,
        currency="USD",
        beneficiary_account="DE89 3704 0044 0532 0130 00",
        reference="office-supplies",
        proposed_at_unix=int(time.time()),
    )
    propose_wire(client, sender, w_small, [alice, bob, dave, carol])
    cosign_wire(client, alice, w_small.wire_id, nonce=1)
    print(f"  {alice.name} cosigned")
    time.sleep(3)
    evaluate_and_show(client, policy, w_small, "SMALL  (expect approved)", ledger)

    # -----------------------------------------------------------------
    banner("Step 4: $5M wire (tier 2, 2 officers required)")
    w_mid = WireInstruction(
        wire_id=f"w-{uuid.uuid4().hex[:8]}",
        sender_bank_quid=sender.quid.id,
        receiver_bank_quid=receiver.quid.id,
        amount_cents=500_000_00,
        currency="USD",
        beneficiary_account="DE89 3704 0044 0532 0130 01",
        reference="Q2-payables",
        proposed_at_unix=int(time.time()),
    )
    propose_wire(client, sender, w_mid, [alice, bob, dave, carol])

    cosign_wire(client, alice, w_mid.wire_id, nonce=2)
    print(f"  {alice.name} cosigned")
    time.sleep(3)
    evaluate_and_show(client, policy, w_mid, "MID @ 1 signer (expect pending)", ledger)

    cosign_wire(client, bob, w_mid.wire_id, nonce=1)
    print(f"  {bob.name} cosigned")
    time.sleep(3)
    evaluate_and_show(client, policy, w_mid, "MID @ 2 signers (expect approved)", NonceLedger())

    # -----------------------------------------------------------------
    banner("Step 5: $50M wire (tier 3, compliance required)")
    w_large = WireInstruction(
        wire_id=f"w-{uuid.uuid4().hex[:8]}",
        sender_bank_quid=sender.quid.id,
        receiver_bank_quid=receiver.quid.id,
        amount_cents=5_000_000_00,
        currency="USD",
        beneficiary_account="DE89 3704 0044 0532 0130 02",
        reference="acquisition-closing",
        proposed_at_unix=int(time.time()),
    )
    propose_wire(client, sender, w_large, [alice, bob, dave, carol])

    cosign_wire(client, alice, w_large.wire_id, nonce=3)
    cosign_wire(client, bob,   w_large.wire_id, nonce=2)
    cosign_wire(client, dave,  w_large.wire_id, nonce=1)
    print(f"  {alice.name}, {bob.name}, {dave.name} each cosigned (no compliance yet)")
    time.sleep(3)
    # NOTE: fresh ledger per evaluate to keep the demo hermetic.
    evaluate_and_show(client, policy, w_large,
                      "LARGE @ 3 officers (expect pending - need compliance role)",
                      NonceLedger())

    cosign_wire(client, carol, w_large.wire_id, nonce=1)
    print(f"  {carol.name} cosigned")
    time.sleep(3)
    evaluate_and_show(client, policy, w_large,
                      "LARGE @ +compliance (expect approved)",
                      NonceLedger())

    # -----------------------------------------------------------------
    banner("Step 6: Receiver bank independently verifies")
    events = load_events(client, w_large.wire_id)
    approvals = extract_approvals(events)
    receiver_verdict = receiver_verify(w_large, policy, approvals)
    print(f"\n  [RECEIVER] {receiver_verdict.short()}")
    print("  The receiver runs the same decision logic against the same")
    print("  signed event stream. No phone call, no PDF, no email.")

    # -----------------------------------------------------------------
    banner("Step 7: Replay defense")
    # Simulate an adversary re-posting alice's cosignature with a
    # nonce she already used -- the in-memory ledger catches it.
    replay_ledger = NonceLedger()
    # Accept alice's nonce=3 from the large wire first (as if it
    # had been gossiped to us already).
    replay_ledger.accept(alice.quid.id, 3)
    # Now a new wire tries to cosign with nonce=3 again.
    w_replay = WireInstruction(
        wire_id=f"w-{uuid.uuid4().hex[:8]}",
        sender_bank_quid=sender.quid.id,
        receiver_bank_quid=receiver.quid.id,
        amount_cents=500_000_00,
        currency="USD",
        beneficiary_account="DE89 0000 0000 0000 0000 00",
        reference="attacker-beneficiary",
        proposed_at_unix=int(time.time()),
    )
    propose_wire(client, sender, w_replay, [alice, bob, dave, carol])
    # Attacker re-publishes alice's old cosignature against the new wire.
    cosign_wire(client, alice, w_replay.wire_id, nonce=3)   # reused nonce
    cosign_wire(client, bob,   w_replay.wire_id, nonce=10)  # fresh for bob
    print(f"  Attacker replayed alice@nonce=3 + bob@nonce=10")
    time.sleep(3)
    evaluate_and_show(client, policy, w_replay,
                      "REPLAY ATTEMPT", replay_ledger)

    # -----------------------------------------------------------------
    banner("Demo complete")
    print()
    print("Insights:")
    print(" - One policy, three tiers -- amount-based routing plus role-")
    print("   specific requirements. No separate 'compliance workflow'")
    print("   system needed.")
    print(" - Receiver runs identical logic on the same stream. Both")
    print("   banks reach the same verdict without out-of-band coordination.")
    print(" - Per-signer nonces caught the replay. In production this")
    print("   enforcement is cryptographic via QDP-0001 on the node.")
    print(" - Forensic reconstruction is 'replay the wire's event stream.'")
    print("   Every signer, every nonce, every cosign timestamp is on-chain.")
    print()


if __name__ == "__main__":
    main()
