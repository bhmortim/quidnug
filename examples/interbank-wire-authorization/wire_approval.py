"""Interbank wire approval logic (standalone, no SDK dep).

A mid-sized bank submits a wire to Fedwire or CHIPS. Before the
wire reaches the settlement gateway, the bank's own internal
controls require:

  1. A *tier* of approvers, where the tier is selected by the
     wire amount.
  2. Per-signer monotonic nonces. A replayed approval from a
     captured session is rejected.
  3. The compliance officer's signature counts double (weight=2),
     so they can alone satisfy a 2-of-N threshold, but for the
     very-large tier their signature is explicitly required.

A receiving bank or counterparty, reading the same event stream,
runs the same verification to decide whether to accept the wire
on its side.

This module is the policy layer. The node enforces signatures,
nonce-uniqueness in its ledger, and key-epoch validity.
"""

from __future__ import annotations

from dataclasses import dataclass, field
from typing import Dict, List, Optional, Set


# ---------------------------------------------------------------------------
# Domain model
# ---------------------------------------------------------------------------

@dataclass(frozen=True)
class WireSigner:
    """One officer in the bank's wire-approval set."""

    signer_quid: str
    weight: int                       # compliance typically 2, others 1
    role: str                         # "officer" | "compliance" | "treasurer"
    current_epoch: int = 0
    max_accepted_old_epoch: Optional[int] = None


@dataclass(frozen=True)
class ApprovalTier:
    """One amount-bracketed rule for how many approvals are needed."""

    min_amount_cents: int
    max_amount_cents: int             # upper bound exclusive; None for infinity
    required_weight: int              # total weight threshold
    required_roles: List[str] = field(default_factory=list)

    def covers(self, amount_cents: int) -> bool:
        if amount_cents < self.min_amount_cents:
            return False
        if self.max_amount_cents == 0:
            return True   # 0 sentinel means "no upper bound"
        return amount_cents < self.max_amount_cents


@dataclass(frozen=True)
class WirePolicy:
    """The bank's entire wire-approval policy."""

    bank_quid: str
    signers: List[WireSigner]
    tiers: List[ApprovalTier]
    frozen: bool = False

    def signer(self, quid: str) -> Optional[WireSigner]:
        for s in self.signers:
            if s.signer_quid == quid:
                return s
        return None

    def tier_for(self, amount_cents: int) -> Optional[ApprovalTier]:
        for t in self.tiers:
            if t.covers(amount_cents):
                return t
        return None


@dataclass(frozen=True)
class WireInstruction:
    """A proposed interbank wire."""

    wire_id: str
    sender_bank_quid: str
    receiver_bank_quid: str
    amount_cents: int
    currency: str
    beneficiary_account: str
    reference: str = ""
    proposed_at_unix: int = 0


@dataclass(frozen=True)
class WireApproval:
    """One officer's cosignature for a wire."""

    signer_quid: str
    wire_id: str
    signer_nonce: int                 # must strictly advance per signer
    signer_epoch: int = 0
    approved_at_unix: int = 0


@dataclass
class WireVerdict:
    verdict: str                       # "approved" | "pending" | "denied"
    wire_id: str
    collected_weight: int = 0
    required_weight: int = 0
    tier_description: str = ""
    reasons: List[str] = field(default_factory=list)
    counted_signers: List[str] = field(default_factory=list)
    audit: List[Dict[str, object]] = field(default_factory=list)

    def short(self) -> str:
        return (
            f"{self.verdict.upper():9s} wire={self.wire_id} "
            f"weight={self.collected_weight}/{self.required_weight} "
            f"[{self.tier_description}]"
        )


# ---------------------------------------------------------------------------
# Nonce ledger: per-signer highest-seen nonce
# ---------------------------------------------------------------------------

class NonceLedger:
    """Tracks the highest nonce observed per signer. Used to
    reject replays. The real node's nonce ledger (QDP-0001) does
    this globally with cryptographic guarantees; here it is a
    pure-Python facsimile for the POC."""

    def __init__(self) -> None:
        self._high: Dict[str, int] = {}

    def accept(self, signer: str, nonce: int) -> bool:
        """Return True if the nonce is strictly greater than any
        seen before for this signer, and record it."""
        prev = self._high.get(signer, -1)
        if nonce <= prev:
            return False
        self._high[signer] = nonce
        return True

    def highest(self, signer: str) -> int:
        return self._high.get(signer, -1)


# ---------------------------------------------------------------------------
# Main entry point
# ---------------------------------------------------------------------------

def evaluate_wire(
    wire: WireInstruction,
    policy: WirePolicy,
    approvals: List[WireApproval],
    ledger: Optional[NonceLedger] = None,
) -> WireVerdict:
    """Pure decision function.

    Order of checks:
      1. Bank frozen -> denied.
      2. Find the tier covering the amount. No tier -> denied.
      3. Dedup approvals by signer (first wins).
      4. For each dedup'd approval:
         - reject if signer not in policy
         - reject if epoch not accepted
         - reject if nonce replay (via ledger, if provided)
         - else count its weight
      5. Check required_roles are each represented by at least
         one counted approval.
      6. Compare total weight to tier's required_weight.
    """
    if policy.frozen:
        return WireVerdict(
            verdict="denied",
            wire_id=wire.wire_id,
            reasons=["bank is frozen"],
        )

    tier = policy.tier_for(wire.amount_cents)
    if tier is None:
        return WireVerdict(
            verdict="denied",
            wire_id=wire.wire_id,
            reasons=[f"no policy tier covers amount ${wire.amount_cents/100:.2f}"],
        )

    tier_desc = (
        f"${tier.min_amount_cents/100:.0f}-{_tier_cap(tier)} "
        f"(need weight {tier.required_weight}"
        f"{', roles ' + '+'.join(tier.required_roles) if tier.required_roles else ''})"
    )

    # Dedup approvals by signer.
    seen: Set[str] = set()
    unique: List[WireApproval] = []
    for ap in approvals:
        if ap.wire_id != wire.wire_id:
            continue
        if ap.signer_quid in seen:
            continue
        seen.add(ap.signer_quid)
        unique.append(ap)

    audit: List[Dict[str, object]] = []
    counted = 0
    counted_roles: Set[str] = set()
    counted_signers: List[str] = []

    for ap in unique:
        signer = policy.signer(ap.signer_quid)
        if signer is None:
            audit.append({
                "signer": ap.signer_quid, "counted": False,
                "reason": "not in policy signer set",
            })
            continue
        epoch_ok = (
            ap.signer_epoch == signer.current_epoch
            or (signer.max_accepted_old_epoch is not None
                and 0 <= ap.signer_epoch <= signer.max_accepted_old_epoch
                and ap.signer_epoch < signer.current_epoch)
        )
        if not epoch_ok:
            audit.append({
                "signer": ap.signer_quid, "counted": False,
                "reason": (
                    f"epoch {ap.signer_epoch} not accepted "
                    f"(current={signer.current_epoch})"
                ),
            })
            continue
        if ledger is not None and not ledger.accept(ap.signer_quid, ap.signer_nonce):
            audit.append({
                "signer": ap.signer_quid, "counted": False,
                "reason": f"replay: nonce {ap.signer_nonce} <= "
                          f"prior-seen {ledger.highest(ap.signer_quid)}",
            })
            continue
        audit.append({
            "signer": ap.signer_quid,
            "role": signer.role,
            "weight": signer.weight,
            "nonce": ap.signer_nonce,
            "counted": True,
        })
        counted += signer.weight
        counted_roles.add(signer.role)
        counted_signers.append(ap.signer_quid)

    # Check required roles.
    missing_roles = [r for r in tier.required_roles if r not in counted_roles]
    if missing_roles:
        return WireVerdict(
            verdict="pending",
            wire_id=wire.wire_id,
            collected_weight=counted,
            required_weight=tier.required_weight,
            tier_description=tier_desc,
            reasons=[f"missing required role(s): {missing_roles}"],
            counted_signers=counted_signers,
            audit=audit,
        )

    verdict = "approved" if counted >= tier.required_weight else "pending"
    return WireVerdict(
        verdict=verdict,
        wire_id=wire.wire_id,
        collected_weight=counted,
        required_weight=tier.required_weight,
        tier_description=tier_desc,
        reasons=[
            f"tier requires weight {tier.required_weight}",
            f"counted weight {counted} from {len(counted_signers)} signer(s)",
        ],
        counted_signers=counted_signers,
        audit=audit,
    )


def _tier_cap(tier: ApprovalTier) -> str:
    if tier.max_amount_cents == 0:
        return "no cap"
    return f"{tier.max_amount_cents/100:.0f}"


# ---------------------------------------------------------------------------
# Counterparty verification: receiver runs the same logic to
# decide whether to credit.
# ---------------------------------------------------------------------------

def receiver_verify(
    wire: WireInstruction,
    sender_policy: WirePolicy,
    approvals: List[WireApproval],
) -> WireVerdict:
    """The receiving bank evaluates the sender bank's approvals
    on the same shared stream. Since nonce-replay is enforced by
    the shared ledger, the receiver does not need its own
    NonceLedger for this check."""
    return evaluate_wire(wire, sender_policy, approvals)


# ---------------------------------------------------------------------------
# Event-stream extraction
# ---------------------------------------------------------------------------

def extract_approvals(events: List[dict]) -> List[WireApproval]:
    out: List[WireApproval] = []
    for ev in events:
        et = ev.get("eventType") or ev.get("event_type") or ""
        if et != "wire.cosigned":
            continue
        p = ev.get("payload") or {}
        out.append(WireApproval(
            signer_quid=p.get("signerQuid", ""),
            wire_id=p.get("wireId", ""),
            signer_nonce=int(p.get("signerNonce") or 0),
            signer_epoch=int(p.get("signerEpoch") or 0),
            approved_at_unix=int(p.get("approvedAt") or ev.get("timestamp") or 0),
        ))
    return out
