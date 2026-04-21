"""Institutional crypto custody, end-to-end runnable demo.

Flow:
  1. Register actors: a custody firm's ops officer, 7 signers, a
     compliance auditor, and a cold-storage wallet quid.
  2. Install the wallet's signing policy (5-of-7 threshold).
  3. Ops officer proposes a $5M-equivalent BTC transfer:
     register_title for the transfer proposal, then emit
     transfer.proposed on its stream.
  4. Signers cosign (signer.1 through signer.4).
     -> verdict: pending (4 of 5).
  5. signer-5 cosigns. -> verdict: authorized.
  6. Audit query shows who signed at what epoch, including the
     stale-epoch warning.
  7. Emergency: ops officer freezes the wallet.
  8. Signers 6 and 7 try to cosign a new transfer after the
     freeze. -> verdict: denied.

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
from typing import Dict, List

sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))
from custody_policy import (
    SignerApproval,
    SignerConfig,
    TransferAuthorization,
    WalletPolicy,
    audit_report,
    evaluate_transfer,
    extract_approvals,
    wallet_frozen_by_events,
)

from quidnug import OwnershipStake, Quid, QuidnugClient

NODE_URL = os.environ.get("QUIDNUG_NODE", "http://localhost:8080")
DOMAIN = "custody.acme.cold-storage"


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


def propose_transfer(
    client: QuidnugClient, proposer: Actor, wallet: Actor,
    transfer: TransferAuthorization, signers: List[Actor],
) -> None:
    """Register the transfer as a TITLE jointly owned by the
    wallet, the proposer, and every authorized signer so all of
    them can emit events on the title's stream. Then emit the
    transfer.proposed event."""
    # Distribute shares; wallet primary, proposer + signers each
    # a small equal share summing with wallet to 1.0.
    participant_count = 1 + len(signers)   # proposer + signers
    participant_share = 0.02
    wallet_share = round(1.0 - participant_share * participant_count, 6)
    owners = [OwnershipStake(wallet.quid.id, wallet_share, "custody-wallet")]
    owners.append(OwnershipStake(proposer.quid.id, participant_share, "proposer"))
    for s in signers:
        owners.append(OwnershipStake(s.quid.id, participant_share, "signer"))
    try:
        client.register_title(
            signer=proposer.quid,
            asset_id=transfer.transfer_id,
            owners=owners,
            domain=DOMAIN,
            title_type="transfer-authorization",
        )
    except Exception as e:
        print(f"  (register_title {transfer.transfer_id}: {e})")
    client.wait_for_title(transfer.transfer_id)

    client.emit_event(
        signer=proposer.quid,
        subject_id=transfer.transfer_id,
        subject_type="TITLE",
        event_type="transfer.proposed",
        domain=DOMAIN,
        payload={
            "transferId": transfer.transfer_id,
            "walletQuid": transfer.wallet_quid,
            "targetChain": transfer.target_chain,
            "targetAddress": transfer.target_address,
            "amountUnits": transfer.amount_units,
            "currency": transfer.currency,
            "proposedAt": transfer.proposed_at_unix,
            "proposerQuid": proposer.quid.id,
            "purpose": transfer.purpose,
        },
    )
    print(f"  {proposer.name} proposed {transfer.transfer_id}")
    print(f"    amount: {transfer.amount_units/1e8:.2f} {transfer.currency}")
    print(f"    to:     {transfer.target_address}")


def cosign(
    client: QuidnugClient, signer: Actor, transfer_id: str, signer_epoch: int,
) -> None:
    client.emit_event(
        signer=signer.quid,
        subject_id=transfer_id,
        subject_type="TITLE",
        event_type="transfer.cosigned",
        domain=DOMAIN,
        payload={
            "signerQuid": signer.quid.id,
            "transferId": transfer_id,
            "signerEpoch": signer_epoch,
            "approvedAt": int(time.time()),
        },
    )


def freeze_wallet(
    client: QuidnugClient, ops: Actor, wallet: Actor, reason: str,
) -> None:
    """Ops officer emits the freeze event on their OWN quid
    stream with the target wallet in the payload. (Wallet QUID
    streams are writable only by the wallet itself.)"""
    client.emit_event(
        signer=ops.quid,
        subject_id=ops.quid.id,
        subject_type="QUID",
        event_type="wallet.frozen",
        domain=DOMAIN,
        payload={
            "targetWalletQuid": wallet.quid.id,
            "frozenBy": ops.quid.id,
            "reason": reason,
            "frozenAt": int(time.time()),
        },
    )
    print(f"  {ops.name} froze {wallet.name}: {reason}")


def load_transfer_events(client: QuidnugClient, transfer_id: str) -> List[dict]:
    events, _ = client.get_stream_events(transfer_id, limit=200)
    out: List[dict] = []
    for ev in events or []:
        out.append({
            "eventType": ev.event_type,
            "payload": ev.payload or {},
            "timestamp": ev.timestamp,
            "sequence": ev.sequence,
        })
    return out


def load_wallet_events(client: QuidnugClient, ops: Actor, wallet: Actor) -> List[dict]:
    """Pull wallet-lifecycle events from the ops officer's own
    stream, filtered to those targeting this wallet."""
    events, _ = client.get_stream_events(ops.quid.id, limit=500)
    out: List[dict] = []
    for ev in events or []:
        p = ev.payload or {}
        if p.get("targetWalletQuid") and p.get("targetWalletQuid") != wallet.quid.id:
            continue
        out.append({
            "eventType": ev.event_type,
            "payload": ev.payload or {},
            "timestamp": ev.timestamp,
            "sequence": ev.sequence,
        })
    return out


def evaluate_and_show(
    client: QuidnugClient, ops: Actor, policy: WalletPolicy, wallet: Actor,
    transfer: TransferAuthorization, label: str,
) -> None:
    # Pull wallet-lifecycle events from the ops officer's stream.
    wallet_evs = load_wallet_events(client, ops, wallet)
    frozen = wallet_frozen_by_events(wallet_evs)
    # Pull the transfer's stream for cosignatures.
    transfer_evs = load_transfer_events(client, transfer.transfer_id)
    approvals = extract_approvals(transfer_evs)
    # Apply freeze from wallet events (overrides policy.frozen).
    effective_policy = WalletPolicy(
        wallet_quid=policy.wallet_quid,
        threshold=policy.threshold,
        signers=policy.signers,
        frozen=frozen or policy.frozen,
    )
    verdict = evaluate_transfer(
        transfer, effective_policy, approvals, now_unix=int(time.time()),
    )
    print(f"\n  [{label}]")
    print(audit_report(verdict))


def main() -> None:
    print(f"Connecting to Quidnug node at {NODE_URL}")
    client = QuidnugClient(NODE_URL)
    try:
        client.info()
    except Exception as e:
        print(f"node unreachable: {e}", file=sys.stderr)
        sys.exit(1)

    client.ensure_domain(DOMAIN)

    banner("Step 1: Register actors")
    ops      = register(client, "ops-officer-acme",  "ops-officer")
    auditor  = register(client, "compliance-audit",  "compliance-auditor")
    wallet   = register(client, "wallet-cold-btc-1", "custody-wallet")
    signers: List[Actor] = []
    for i in range(1, 8):
        signers.append(register(client, f"signer-{i}", "signer"))
    for a in [ops, auditor, wallet] + signers:
        print(f"  {a.role:20s} {a.name:20s} -> {a.quid.id}")
    client.wait_for_identities(
        [a.quid.id for a in [ops, auditor, wallet] + signers]
    )

    banner("Step 2: Install wallet signing policy (5-of-7)")
    # In a fuller deployment this would be a GuardianSet install
    # via /api/v2/guardian/set-install. For the POC, we keep the
    # policy client-side and the wallet.policy event carries the
    # public declaration for auditors.
    signer_configs = []
    for i, s in enumerate(signers, start=1):
        # Signer 1 and 2 have rotated once (epoch 1).
        # Signer 3 has never rotated (epoch 0) -- intentionally stale.
        epoch = 1 if i in (1, 2) else 0
        signer_configs.append(SignerConfig(
            signer_quid=s.quid.id,
            current_epoch=epoch,
            role=f"signer-{i}",
        ))
    policy = WalletPolicy(
        wallet_quid=wallet.quid.id,
        threshold=5,
        signers=signer_configs,
    )

    # Record the policy installation as an event on the ops
    # officer's own stream (with target-wallet pointer). Wallet
    # QUID streams are writable only by the wallet itself.
    client.emit_event(
        signer=ops.quid,
        subject_id=ops.quid.id,
        subject_type="QUID",
        event_type="wallet.policy-installed",
        domain=DOMAIN,
        payload={
            "targetWalletQuid": wallet.quid.id,
            "threshold": policy.threshold,
            "signerCount": len(policy.signers),
            "signers": [
                {"quid": s.signer_quid, "currentEpoch": s.current_epoch}
                for s in policy.signers
            ],
            "installedBy": ops.quid.id,
            "installedAt": int(time.time()),
        },
    )
    print(f"  threshold = {policy.threshold}-of-{len(policy.signers)}")
    for sc in signer_configs:
        print(f"    {sc.signer_quid[:30]:30s} current_epoch={sc.current_epoch}")

    time.sleep(1)

    banner("Step 3: Ops officer proposes a $5M BTC transfer")
    transfer_id = f"tx-{uuid.uuid4().hex[:8]}"
    transfer = TransferAuthorization(
        transfer_id=transfer_id,
        wallet_quid=wallet.quid.id,
        target_chain="bitcoin",
        target_address="bc1qexampledestinationaddressfordemo1234",
        amount_units=50_000_000_00,   # 500 BTC = $5M at ~$10k (just demo numbers)
        currency="BTC",
        proposed_at_unix=int(time.time()),
        proposer_quid=ops.quid.id,
        purpose="quarterly rebalance per Q2 plan",
    )
    propose_transfer(client, ops, wallet, transfer, signers)

    time.sleep(1)

    banner("Step 4: Signers 1..4 cosign; verdict should be pending")
    for i in range(0, 4):
        s = signers[i]
        cosign(client, s, transfer_id,
               signer_epoch=signer_configs[i].current_epoch)
        print(f"  {s.name:12s} cosigned (epoch {signer_configs[i].current_epoch})")
    time.sleep(0.5)
    evaluate_and_show(client, ops, policy, wallet, transfer, "AFTER 4 COSIGNS")

    banner("Step 5: Signer 5 cosigns; verdict should be authorized")
    cosign(client, signers[4], transfer_id,
           signer_epoch=signer_configs[4].current_epoch)
    print(f"  {signers[4].name} cosigned")
    time.sleep(0.5)
    evaluate_and_show(client, ops, policy, wallet, transfer, "AFTER 5 COSIGNS")

    banner("Step 6: Audit view  (retained for forensics)")
    events = load_transfer_events(client, transfer_id)
    print(f"  {len(events)} events on transfer {transfer_id}:")
    for ev in events:
        print(f"    {ev['eventType']:30s} "
              f"by {(ev['payload'].get('signerQuid') or ev['payload'].get('proposerQuid', ''))[:30]} "
              f"epoch={ev['payload'].get('signerEpoch', '-')}")

    banner("Step 7: Stale-epoch watch")
    # signer-3 hasn't rotated -- flag it.
    last_rotation = {s.signer_quid: int(time.time()) - 10 * 86400
                      for s in signer_configs[:2]}
    # Signers 3..7 have no recorded rotation (epoch 0, never rotated).
    from custody_policy import stale_epoch_signers
    stale = stale_epoch_signers(
        policy, last_rotation, now_unix=int(time.time()),
        max_age_seconds=90 * 86400,
    )
    print(f"  Overdue for rotation: {stale[:3]}...")
    print(f"  (Monitoring rule would alert compliance; may auto-freeze)")

    banner("Step 8: Emergency freeze + re-try a new transfer")
    freeze_wallet(client, ops, wallet, reason="suspicious-ip-detected-signer-3")
    time.sleep(1)

    # New transfer after freeze; signers try to push it through.
    transfer2_id = f"tx-{uuid.uuid4().hex[:8]}"
    transfer2 = TransferAuthorization(
        transfer_id=transfer2_id,
        wallet_quid=wallet.quid.id,
        target_chain="bitcoin",
        target_address="bc1qsecondaddress",
        amount_units=10_000_000_00,
        currency="BTC",
        proposed_at_unix=int(time.time()),
        proposer_quid=ops.quid.id,
        purpose="retry during freeze",
    )
    propose_transfer(client, ops, wallet, transfer2, signers)
    for i in range(5):
        cosign(client, signers[i], transfer2_id,
               signer_epoch=signer_configs[i].current_epoch)
    time.sleep(0.5)
    evaluate_and_show(client, ops, policy, wallet, transfer2, "POST-FREEZE (EXPECT DENIED)")

    banner("Demo complete")
    print()
    print("Insights:")
    print(" - Each transfer is a title; approvals + freeze events lie")
    print("   in the transfer's and wallet's streams respectively.")
    print(" - The authorization verdict is a pure function of the policy")
    print("   and the streams. It can be re-evaluated at any time; the")
    print("   forensic audit is just 'replay the verdict on historical")
    print("   stream state.'")
    print(" - Epoch tracking is explicit. An auditor immediately sees")
    print("   which signer hasn't rotated in too long.")
    print(" - A wallet freeze (single signed event) denies all future")
    print("   transfers atomically. No 'update every multisig' scramble.")
    print()


if __name__ == "__main__":
    main()
