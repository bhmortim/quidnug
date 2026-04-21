"""Institutional custody authorization logic (standalone, no SDK dep).

A custody firm holds crypto assets in wallets, each wallet having
a fixed set of authorized signers and an M-of-N threshold. To
authorize a transfer, a quorum of signers must cosign. The firm
wants:

  - Exactly the declared signers count (not more, not fewer).
  - Each cosign must be from a live epoch (not an already-rotated
    or invalidated key).
  - An explicit kill-switch: an ops officer can freeze a wallet
    against any further transfers.
  - A rich audit trail: who signed, at what epoch, when.

This module is the policy layer. It takes:
  - A WalletPolicy: the declared signer set, threshold, current
    rotation epoch per signer, and frozen state.
  - A TransferAuthorization proposal.
  - A list of SignerApproval records (each ties a signer to the
    epoch they signed at).
  - Wall-clock time.

And returns a TransferVerdict (authorized | pending | denied).
Pure data + functions, runnable without a live node.
"""

from __future__ import annotations

from dataclasses import dataclass, field
from typing import Dict, List, Optional


# ---------------------------------------------------------------------------
# Domain model
# ---------------------------------------------------------------------------

@dataclass(frozen=True)
class SignerConfig:
    """One signer's entry in a wallet policy."""

    signer_quid: str
    current_epoch: int
    # If set, older epochs up to and including this are still
    # accepted for cosignatures -- covers the rotation grace
    # window. None means only current_epoch is accepted.
    max_accepted_old_epoch: Optional[int] = None
    role: str = ""


@dataclass(frozen=True)
class WalletPolicy:
    """The declared signing policy for a single custody wallet."""

    wallet_quid: str
    threshold: int
    signers: List[SignerConfig]
    frozen: bool = False

    def signer(self, quid: str) -> Optional[SignerConfig]:
        for s in self.signers:
            if s.signer_quid == quid:
                return s
        return None

    def epoch_accepted(self, signer_quid: str, epoch: int) -> bool:
        s = self.signer(signer_quid)
        if s is None:
            return False
        if epoch == s.current_epoch:
            return True
        if s.max_accepted_old_epoch is not None:
            return s.max_accepted_old_epoch >= epoch >= 0 \
                   and epoch < s.current_epoch
        return False


@dataclass(frozen=True)
class TransferAuthorization:
    """A proposed transfer. Typically lives as a TITLE on the
    node; the POC treats it as a bag of attributes."""

    transfer_id: str
    wallet_quid: str
    target_chain: str
    target_address: str
    amount_units: int          # satoshis, wei, lamports; integer
    currency: str
    proposed_at_unix: int
    proposer_quid: str = ""
    purpose: str = ""


@dataclass(frozen=True)
class SignerApproval:
    """One signer has cosigned this transfer at a given epoch."""

    signer_quid: str
    transfer_id: str
    signer_epoch: int
    approved_at_unix: int


@dataclass
class TransferVerdict:
    verdict: str               # "authorized" | "pending" | "denied"
    transfer_id: str
    collected_weight: int = 0
    required_weight: int = 0
    reasons: List[str] = field(default_factory=list)
    # Full breakdown for audit: signer, epoch they signed at,
    # whether it was counted.
    audit: List[Dict[str, object]] = field(default_factory=list)

    def short(self) -> str:
        return (
            f"{self.verdict.upper():10s} transfer={self.transfer_id} "
            f"weight={self.collected_weight}/{self.required_weight}"
        )


# ---------------------------------------------------------------------------
# Main entry point
# ---------------------------------------------------------------------------

def evaluate_transfer(
    transfer: TransferAuthorization,
    policy: WalletPolicy,
    approvals: List[SignerApproval],
    now_unix: int,
) -> TransferVerdict:
    """Pure decision function.

    Order of checks:
      1. Wallet frozen -> denied.
      2. Dedup approvals by signer_quid (first wins).
      3. For each dedup'd approval, annotate whether it counts:
         - signer in the policy?
         - approval epoch acceptable?
      4. Sum counted weight (each signer = 1).
      5. Compare against threshold.
    """
    if policy.frozen:
        return TransferVerdict(
            verdict="denied",
            transfer_id=transfer.transfer_id,
            required_weight=policy.threshold,
            reasons=["wallet is frozen"],
        )

    # Dedup by signer.
    seen: set = set()
    unique: List[SignerApproval] = []
    for ap in approvals:
        if ap.transfer_id != transfer.transfer_id:
            continue
        if ap.signer_quid in seen:
            continue
        seen.add(ap.signer_quid)
        unique.append(ap)

    # Build the audit entries and count weight.
    audit: List[Dict[str, object]] = []
    counted = 0
    for ap in unique:
        signer = policy.signer(ap.signer_quid)
        if signer is None:
            audit.append({
                "signer": ap.signer_quid,
                "epoch": ap.signer_epoch,
                "counted": False,
                "reason": "not in policy signer set",
            })
            continue
        if not policy.epoch_accepted(ap.signer_quid, ap.signer_epoch):
            audit.append({
                "signer": ap.signer_quid,
                "epoch": ap.signer_epoch,
                "counted": False,
                "reason": (
                    f"epoch {ap.signer_epoch} not accepted "
                    f"(current={signer.current_epoch})"
                ),
            })
            continue
        audit.append({
            "signer": ap.signer_quid,
            "epoch": ap.signer_epoch,
            "role": signer.role,
            "counted": True,
        })
        counted += 1

    reasons: List[str] = [
        f"wallet threshold: {policy.threshold}-of-{len(policy.signers)}",
        f"counted approvals: {counted}",
    ]
    verdict = (
        "authorized" if counted >= policy.threshold
        else "pending"
    )
    return TransferVerdict(
        verdict=verdict,
        transfer_id=transfer.transfer_id,
        collected_weight=counted,
        required_weight=policy.threshold,
        reasons=reasons,
        audit=audit,
    )


# ---------------------------------------------------------------------------
# Audit helpers
# ---------------------------------------------------------------------------

def audit_report(verdict: TransferVerdict) -> str:
    """Pretty-print the verdict + audit trail."""
    lines = [verdict.short()]
    for r in verdict.reasons:
        lines.append(f"  - {r}")
    if verdict.audit:
        lines.append("  audit:")
        for entry in verdict.audit:
            mark = "✓" if entry.get("counted") else "✗"
            lines.append(
                f"    {mark} {entry['signer']:24s} "
                f"epoch={entry['epoch']}"
                f"{('  ' + entry.get('role', '')).rstrip()}"
                f"{('  -> ' + entry['reason']) if not entry.get('counted') else ''}"
            )
    return "\n".join(lines)


def stale_epoch_signers(
    policy: WalletPolicy, last_rotation_by_signer: Dict[str, int], now_unix: int,
    max_age_seconds: int = 90 * 86400,
) -> List[str]:
    """Return signers whose last rotation is older than the policy
    (default: 90 days)."""
    out: List[str] = []
    for s in policy.signers:
        last = last_rotation_by_signer.get(s.signer_quid, 0)
        if (now_unix - last) > max_age_seconds:
            out.append(s.signer_quid)
    return out


# ---------------------------------------------------------------------------
# Stream helpers: build approvals from event-stream dicts
# ---------------------------------------------------------------------------

def extract_approvals(events: List[dict]) -> List[SignerApproval]:
    """Scan a transfer's event stream for `transfer.cosigned` events."""
    out: List[SignerApproval] = []
    for ev in events:
        et = ev.get("eventType") or ev.get("event_type") or ""
        if et != "transfer.cosigned":
            continue
        p = ev.get("payload") or {}
        out.append(SignerApproval(
            signer_quid=p.get("signerQuid", ""),
            transfer_id=p.get("transferId", ""),
            signer_epoch=int(p.get("signerEpoch") or 0),
            approved_at_unix=int(p.get("approvedAt") or ev.get("timestamp") or 0),
        ))
    return out


def wallet_frozen_by_events(events: List[dict]) -> bool:
    """Return True if the wallet has had a `wallet.frozen` event,
    unless a later `wallet.unfrozen` event exists."""
    frozen = False
    for ev in events:
        et = ev.get("eventType") or ev.get("event_type") or ""
        if et == "wallet.frozen":
            frozen = True
        elif et == "wallet.unfrozen":
            frozen = False
    return frozen
