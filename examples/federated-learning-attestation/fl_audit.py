"""Federated-learning round audit logic (standalone, no SDK dep).

A federated-learning round runs across N participants that each
train on their own local data and submit gradient updates to a
coordinator. The coordinator aggregates. After a dispute or on
a routine audit, an auditor (or any participant) wants to know:

  - Did all registered participants actually submit?
  - Are any gradients suspiciously large (potential poisoning)?
  - Did the coordinator actually emit an aggregation event?
  - Does the aggregate's weight distribution match the attested
    data sizes?

This module is pure audit logic. Given the round's event stream
and a RoundPolicy, it returns a RoundVerdict.
"""

from __future__ import annotations

from dataclasses import dataclass, field
from typing import Dict, List, Optional


# ---------------------------------------------------------------------------
# Domain model
# ---------------------------------------------------------------------------

@dataclass(frozen=True)
class Registration:
    participant_quid: str
    attested_data_size: int
    registered_at_unix: int = 0


@dataclass(frozen=True)
class GradientSubmission:
    participant_quid: str
    gradient_cid: str
    gradient_hash: str
    # Pre-committed L2 norm of the gradient. Used as a proxy for
    # poisoning detection: a participant submitting a gradient
    # massively larger than its peers is suspicious.
    gradient_norm: float
    training_data_size: int
    submitted_at_unix: int = 0


@dataclass(frozen=True)
class Aggregation:
    coordinator_quid: str
    aggregate_hash: str
    participant_weights: Dict[str, float] = field(default_factory=dict)
    aggregated_at_unix: int = 0


@dataclass
class RoundBreakdown:
    registered_participants: List[str] = field(default_factory=list)
    submitted_participants: List[str] = field(default_factory=list)
    missing_participants: List[str] = field(default_factory=list)
    suspicious_gradients: List[str] = field(default_factory=list)
    aggregation_present: bool = False


@dataclass
class RoundVerdict:
    verdict: str                    # "valid" | "insufficient" | "integrity-violation" | "incomplete"
    round_id: str
    breakdown: RoundBreakdown = field(default_factory=RoundBreakdown)
    reasons: List[str] = field(default_factory=list)

    def short(self) -> str:
        return (
            f"{self.verdict.upper():21s} "
            f"round={self.round_id} "
            f"submitted={len(self.breakdown.submitted_participants)} "
            f"missing={len(self.breakdown.missing_participants)} "
            f"suspicious={len(self.breakdown.suspicious_gradients)}"
        )


@dataclass
class RoundPolicy:
    min_participants: int = 5
    # A gradient is flagged as suspicious if its norm is more than
    # this multiple of the median norm across the round.
    suspicious_norm_multiplier: float = 5.0
    require_aggregation_event: bool = True
    # If True, any registered participant who doesn't submit is
    # an integrity violation. If False, missing submitters are
    # just warnings.
    strict_registration: bool = False


# ---------------------------------------------------------------------------
# Stream extraction
# ---------------------------------------------------------------------------

def extract_registrations(events: List[dict]) -> List[Registration]:
    out: List[Registration] = []
    for ev in events:
        et = ev.get("eventType") or ev.get("event_type") or ""
        if et != "participant.registered":
            continue
        p = ev.get("payload") or {}
        out.append(Registration(
            participant_quid=p.get("participant", ""),
            attested_data_size=int(p.get("attestedDataSize") or 0),
            registered_at_unix=int(p.get("registeredAt") or ev.get("timestamp") or 0),
        ))
    return out


def extract_submissions(events: List[dict]) -> List[GradientSubmission]:
    out: List[GradientSubmission] = []
    for ev in events:
        et = ev.get("eventType") or ev.get("event_type") or ""
        if et != "gradient.submitted":
            continue
        p = ev.get("payload") or {}
        out.append(GradientSubmission(
            participant_quid=p.get("participant", ""),
            gradient_cid=p.get("gradientCID", ""),
            gradient_hash=p.get("gradientHash", ""),
            gradient_norm=float(p.get("gradientNorm") or 0.0),
            training_data_size=int(p.get("trainingDataSize") or 0),
            submitted_at_unix=int(p.get("submittedAt") or ev.get("timestamp") or 0),
        ))
    return out


def extract_aggregation(events: List[dict]) -> Optional[Aggregation]:
    # Return the LAST aggregation event if any (a round should
    # only have one; if more exist we log them all for audit).
    found: Optional[Aggregation] = None
    for ev in events:
        et = ev.get("eventType") or ev.get("event_type") or ""
        if et != "round.aggregated":
            continue
        p = ev.get("payload") or {}
        weights = p.get("participantWeights") or {}
        found = Aggregation(
            coordinator_quid=p.get("coordinator", ""),
            aggregate_hash=p.get("aggregateHash", ""),
            participant_weights={k: float(v) for k, v in weights.items()},
            aggregated_at_unix=int(p.get("aggregatedAt") or ev.get("timestamp") or 0),
        )
    return found


# ---------------------------------------------------------------------------
# Main entry point
# ---------------------------------------------------------------------------

def audit_round(
    round_id: str,
    events: List[dict],
    policy: Optional[RoundPolicy] = None,
) -> RoundVerdict:
    """Pure audit of an FL round."""
    p = policy or RoundPolicy()
    reasons: List[str] = []
    bd = RoundBreakdown()

    regs = extract_registrations(events)
    subs = extract_submissions(events)
    agg = extract_aggregation(events)

    bd.registered_participants = sorted({r.participant_quid for r in regs if r.participant_quid})
    bd.submitted_participants = sorted({s.participant_quid for s in subs if s.participant_quid})
    bd.missing_participants = [
        x for x in bd.registered_participants
        if x not in bd.submitted_participants
    ]
    bd.aggregation_present = agg is not None

    reasons.append(
        f"{len(bd.registered_participants)} registered, "
        f"{len(bd.submitted_participants)} submitted"
    )

    # Suspicious gradient check: any whose norm exceeds
    # (multiplier * median norm).
    if subs:
        norms = sorted(s.gradient_norm for s in subs if s.gradient_norm > 0)
        if norms:
            median = norms[len(norms) // 2]
            cap = p.suspicious_norm_multiplier * median
            for s in subs:
                if s.gradient_norm > 0 and s.gradient_norm > cap:
                    bd.suspicious_gradients.append(
                        f"{s.participant_quid[:12]} norm={s.gradient_norm:.3f} "
                        f"(median {median:.3f}, cap {cap:.3f})"
                    )
            if bd.suspicious_gradients:
                reasons.append(
                    f"{len(bd.suspicious_gradients)} suspicious gradients "
                    f"(median norm {median:.3f})"
                )

    # Verdict.
    if len(bd.submitted_participants) < p.min_participants:
        return RoundVerdict(
            verdict="insufficient",
            round_id=round_id,
            breakdown=bd,
            reasons=reasons + [
                f"only {len(bd.submitted_participants)} submitted, "
                f"need {p.min_participants}"
            ],
        )

    if p.strict_registration and bd.missing_participants:
        return RoundVerdict(
            verdict="integrity-violation",
            round_id=round_id,
            breakdown=bd,
            reasons=reasons + [
                f"strict policy: missing submitters {bd.missing_participants}"
            ],
        )

    if p.require_aggregation_event and not bd.aggregation_present:
        return RoundVerdict(
            verdict="incomplete",
            round_id=round_id,
            breakdown=bd,
            reasons=reasons + ["coordinator has not emitted round.aggregated"],
        )

    # A few suspicious gradients don't flip the verdict to
    # violation, but they are surfaced for the caller.
    return RoundVerdict(
        verdict="valid",
        round_id=round_id,
        breakdown=bd,
        reasons=reasons + ["round passes all audit gates"],
    )


# ---------------------------------------------------------------------------
# Convenience: compute fair-weight contribution shares
# ---------------------------------------------------------------------------

def fair_weights_by_data_size(
    regs: List[Registration],
    subs: List[GradientSubmission],
) -> Dict[str, float]:
    """Return the fair weight each participant should have in
    the aggregate, proportional to the MIN of (attested data
    size at registration, training_data_size at submission).

    The fair weight can be compared against the coordinator's
    actual aggregate.participant_weights to detect coordinator
    bias."""
    attested = {r.participant_quid: r.attested_data_size for r in regs}
    submitted = {s.participant_quid: s.training_data_size for s in subs}
    share: Dict[str, int] = {}
    for pq in set(attested) & set(submitted):
        share[pq] = min(attested[pq], submitted[pq])
    total = sum(share.values())
    if total <= 0:
        return {pq: 1.0 / len(share) for pq in share} if share else {}
    return {pq: v / total for pq, v in share.items()}
