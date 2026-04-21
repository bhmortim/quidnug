"""Decentralized credit reputation evaluation (standalone, no SDK dep).

A lender evaluates whether to extend credit to a subject based
on:

  - The subject's signed event history (loans originated,
    payments on-time, missed payments, defaults, cures).
  - Trust edges from this lender to each attester -- more trust
    in the reporter increases the weight of their attestations.
  - The subject's disputes against specific events.

There is NO universal score. Every lender computes their own
verdict from the same signed stream, using their own trust graph
and policy. Alternative-data events (utility, rent, employment)
participate with a configurable weight.
"""

from __future__ import annotations

from dataclasses import dataclass, field
from typing import Callable, Dict, List, Optional, Set


# ---------------------------------------------------------------------------
# Domain model
# ---------------------------------------------------------------------------

EVENT_LOAN_ORIGINATED = "credit.loan.originated"
EVENT_PAYMENT_ON_TIME = "credit.payment.on-time"
EVENT_PAYMENT_MISSED  = "credit.payment.missed"
EVENT_LOAN_CLOSED     = "credit.loan.closed-in-good-standing"
EVENT_DEFAULT         = "credit.loan.default"
EVENT_CURED           = "credit.loan.cured"
EVENT_UTILITY_ON_TIME = "credit.utility.on-time"
EVENT_RENT_ON_TIME    = "credit.rent.on-time"
EVENT_DISPUTE         = "credit.dispute"

NEGATIVE_EVENTS = {EVENT_PAYMENT_MISSED, EVENT_DEFAULT}
POSITIVE_EVENTS = {
    EVENT_PAYMENT_ON_TIME, EVENT_LOAN_CLOSED,
    EVENT_UTILITY_ON_TIME, EVENT_RENT_ON_TIME, EVENT_CURED,
}
ALT_DATA_EVENTS = {EVENT_UTILITY_ON_TIME, EVENT_RENT_ON_TIME}


@dataclass(frozen=True)
class CreditEvent:
    event_type: str
    attester_quid: str         # the lender / utility / etc. signing
    subject_quid: str
    category: str = ""          # "auto-loan" | "mortgage" | "utility" | ...
    amount_band: str = ""       # "0-10k" | "10k-30k" | ...
    loan_id: str = ""
    timestamp_unix: int = 0


@dataclass(frozen=True)
class Dispute:
    """A subject-signed dispute against a prior event."""

    disputes_event_type: str
    disputes_attester: str
    disputes_loan_id: str
    reason: str = ""
    filed_at_unix: int = 0


@dataclass
class CreditVerdict:
    verdict: str                   # "approve" | "indeterminate" | "decline"
    subject_quid: str
    score: float = 0.0             # observer's weighted confidence [0, 1]
    positive_signal: float = 0.0
    negative_signal: float = 0.0
    alt_data_signal: float = 0.0
    reasons: List[str] = field(default_factory=list)
    disputes_considered: int = 0

    def short(self) -> str:
        return (
            f"{self.verdict.upper():13s} subject={self.subject_quid[:16]} "
            f"score={self.score:.3f} "
            f"+={self.positive_signal:.2f} -={self.negative_signal:.2f}"
        )


TrustFn = Callable[[str, str], float]


@dataclass
class LenderPolicy:
    """One lender's knobs."""

    min_attester_trust: float = 0.3
    approve_threshold: float = 0.6
    decline_threshold: float = 0.3
    # How much a negative event pulls the score down per unit.
    negative_weight: float = 0.3
    # How much a positive event contributes per unit.
    positive_weight: float = 0.08
    # Alt-data events' contribution is scaled down by this factor
    # since they're less diagnostic of credit behavior than actual
    # loan performance.
    alt_data_weight_scale: float = 0.5
    # If set, only events in these categories are counted.
    # Example: ["auto-loan"] when evaluating for an auto loan.
    relevant_categories: Optional[List[str]] = None


# ---------------------------------------------------------------------------
# Stream extraction
# ---------------------------------------------------------------------------

CREDIT_EVENT_TYPES = {
    EVENT_LOAN_ORIGINATED, EVENT_PAYMENT_ON_TIME, EVENT_PAYMENT_MISSED,
    EVENT_LOAN_CLOSED, EVENT_DEFAULT, EVENT_CURED,
    EVENT_UTILITY_ON_TIME, EVENT_RENT_ON_TIME,
}


def extract_credit_events(events: List[dict]) -> List[CreditEvent]:
    out: List[CreditEvent] = []
    for ev in events:
        et = ev.get("eventType") or ev.get("event_type") or ""
        if et not in CREDIT_EVENT_TYPES:
            continue
        p = ev.get("payload") or {}
        out.append(CreditEvent(
            event_type=et,
            attester_quid=p.get("attester") or p.get("signerQuid", ""),
            subject_quid=p.get("subject", ""),
            category=p.get("category", ""),
            amount_band=p.get("amountBand", ""),
            loan_id=p.get("loanId", ""),
            timestamp_unix=int(p.get("timestamp") or ev.get("timestamp") or 0),
        ))
    return out


def extract_disputes(events: List[dict]) -> List[Dispute]:
    out: List[Dispute] = []
    for ev in events:
        et = ev.get("eventType") or ev.get("event_type") or ""
        if et != EVENT_DISPUTE:
            continue
        p = ev.get("payload") or {}
        out.append(Dispute(
            disputes_event_type=p.get("disputesEventType", ""),
            disputes_attester=p.get("disputesAttester", ""),
            disputes_loan_id=p.get("disputesLoanId", ""),
            reason=p.get("reason", ""),
            filed_at_unix=int(p.get("filedAt") or ev.get("timestamp") or 0),
        ))
    return out


# ---------------------------------------------------------------------------
# Main entry point
# ---------------------------------------------------------------------------

def evaluate_borrower(
    lender: str,
    subject_quid: str,
    events: List[dict],
    trust_fn: TrustFn,
    policy: Optional[LenderPolicy] = None,
) -> CreditVerdict:
    """Pure evaluation function."""
    p = policy or LenderPolicy()
    credit_events = extract_credit_events(events)
    disputes = extract_disputes(events)

    # Build a set of (event_type, attester, loan_id) tuples the
    # subject has disputed. Disputed negative events are excluded.
    # Disputed positive events stay counted (no reason for the
    # subject to dispute a positive record).
    disputed_keys: Set = set()
    for d in disputes:
        if d.disputes_event_type in NEGATIVE_EVENTS:
            disputed_keys.add(
                (d.disputes_event_type, d.disputes_attester,
                 d.disputes_loan_id)
            )

    pos = 0.0
    neg = 0.0
    alt = 0.0
    reasons: List[str] = []
    considered = 0

    for e in credit_events:
        # Filter by category if policy specifies.
        if p.relevant_categories is not None:
            if e.category and e.category not in p.relevant_categories:
                continue

        # Filter by subject.
        if e.subject_quid != subject_quid:
            continue

        trust = trust_fn(lender, e.attester_quid)
        if trust < 0.0 or trust > 1.0:
            raise ValueError(f"trust out of range: {trust}")
        if trust < p.min_attester_trust:
            continue

        # Is this event disputed?
        key = (e.event_type, e.attester_quid, e.loan_id)
        is_disputed = key in disputed_keys

        if e.event_type in NEGATIVE_EVENTS:
            if is_disputed:
                considered += 1
                reasons.append(
                    f"disputed-negative from {e.attester_quid[:12]}: "
                    f"{e.event_type} (discounted)"
                )
                continue
            neg += p.negative_weight * trust
        elif e.event_type in ALT_DATA_EVENTS:
            alt += p.positive_weight * trust * p.alt_data_weight_scale
        elif e.event_type in POSITIVE_EVENTS:
            pos += p.positive_weight * trust
        considered += 1

    # Base score. Start at 0.5 (neutral). Positive and alt data
    # push up; negative pulls down. Alt data caps at a smaller
    # contribution by construction (scale factor).
    score = 0.5 + pos + alt - neg
    score = max(0.0, min(1.0, score))

    reasons.insert(0,
        f"considered {considered} events; "
        f"+pos={pos:.3f} +alt={alt:.3f} -neg={neg:.3f} -> score {score:.3f}"
    )

    if score >= p.approve_threshold:
        verdict = "approve"
    elif score <= p.decline_threshold:
        verdict = "decline"
    else:
        verdict = "indeterminate"

    return CreditVerdict(
        verdict=verdict, subject_quid=subject_quid,
        score=score,
        positive_signal=pos, negative_signal=neg,
        alt_data_signal=alt,
        reasons=reasons,
        disputes_considered=len(disputes),
    )
