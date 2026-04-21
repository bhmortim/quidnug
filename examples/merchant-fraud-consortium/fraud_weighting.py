"""Weighted-aggregation math for the merchant fraud consortium.

This module is the observer-relative trust computation that
makes merchant-fraud-consortium's value prop concrete. It's
independent of the Quidnug HTTP client so it can be tested
in-process (``fraud_weighting_test.py``).

The core question: given a set of fraud signals emitted by
different merchants about the same card / entity, what's the
overall confidence that the target is fraudulent from *my*
perspective?

Inputs:
    - A list of signals: (reporter_quid, severity, pattern_type).
    - A function ``trust(observer, target)`` returning the
      observer's relational trust in ``target`` in [0, 1].

Output:
    - An aggregate confidence score in [0, 1] combining
      severity weighted by trust.
    - A per-signal breakdown so UIs can explain the score.

The aggregation formula is deliberately simple: weighted
average where each signal's weight is ``trust(observer,
reporter)``. Signals from strongly-trusted reporters dominate;
signals from low-trust or unknown reporters contribute little
but aren't silently dropped (transparency requirement).
"""

from __future__ import annotations

from dataclasses import dataclass
from typing import Callable, Dict, List, Optional, Tuple


@dataclass(frozen=True)
class FraudSignal:
    """A single fraud report emitted by a merchant."""

    reporter_quid: str
    severity: float          # 0.0 (no-op) .. 1.0 (highest confidence)
    pattern_type: str        # "card-testing", "velocity-abuse", etc.
    observed_at_unix: int    # when the reporter observed the event


@dataclass
class WeightedContribution:
    """Per-signal breakdown of the aggregate score."""

    reporter_quid: str
    severity: float
    observer_trust: float
    weight: float            # severity * observer_trust
    pattern_type: str


@dataclass
class AggregateScore:
    """The overall fraud-confidence score for a target + its
    contributing signals."""

    score: float                              # [0, 1]
    contributions: List[WeightedContribution]
    signal_count: int
    total_trust_weight: float

    def summary(self) -> str:
        """Human-readable one-liner for demo output."""
        return (
            f"score={self.score:.3f} "
            f"from {self.signal_count} signals "
            f"(total trust weight {self.total_trust_weight:.2f})"
        )


TrustFn = Callable[[str, str], float]


def aggregate_fraud_score(
    observer: str,
    signals: List[FraudSignal],
    trust_fn: TrustFn,
    *,
    min_trust_to_count: float = 0.0,
) -> AggregateScore:
    """Compute the observer-relative fraud confidence score.

    Formula:
        score = sum(severity_i * trust_i) / sum(trust_i)

    Where ``trust_i`` is ``trust_fn(observer, reporter_i)``.
    Signals with ``trust_i < min_trust_to_count`` are
    excluded from the denominator (they still appear in the
    contributions list so the UI can show them as
    "discarded").

    If total trust weight is zero (no signals from trusted
    reporters), the score is 0.0.
    """
    contributions: List[WeightedContribution] = []
    total_weighted_severity = 0.0
    total_trust = 0.0

    for sig in signals:
        t = trust_fn(observer, sig.reporter_quid)
        if t < 0.0 or t > 1.0:
            raise ValueError(f"trust out of [0,1]: {t}")
        w = sig.severity * t
        contributions.append(
            WeightedContribution(
                reporter_quid=sig.reporter_quid,
                severity=sig.severity,
                observer_trust=t,
                weight=w,
                pattern_type=sig.pattern_type,
            )
        )
        if t >= min_trust_to_count:
            total_weighted_severity += w
            total_trust += t

    score = 0.0
    if total_trust > 0:
        score = total_weighted_severity / total_trust

    return AggregateScore(
        score=score,
        contributions=contributions,
        signal_count=len(signals),
        total_trust_weight=total_trust,
    )


def aggregate_with_decay(
    observer: str,
    signals: List[FraudSignal],
    trust_fn: TrustFn,
    now_unix: int,
    *,
    half_life_seconds: int = 30 * 24 * 3600,   # 30 days: signals "expire" fast
    min_trust_to_count: float = 0.0,
) -> AggregateScore:
    """Variant of ``aggregate_fraud_score`` that additionally
    decays each signal's severity by its age.

    Fraud patterns are usually short-lived (card-testing
    lasts hours, velocity-abuse minutes). A 30-day half-life
    mirrors the intuition that a signal from 30 days ago is
    half as relevant as one from today; 60 days ago is a
    quarter, etc.

    Not the same concept as QDP-0019 trust-edge decay. That
    decays the *trust edges*; this decays the *signal
    observations*. Both are observer-local policies.
    """
    import math
    contributions: List[WeightedContribution] = []
    total_weighted_severity = 0.0
    total_trust = 0.0

    for sig in signals:
        t = trust_fn(observer, sig.reporter_quid)
        age = max(0, now_unix - sig.observed_at_unix)
        # Age decay: 0.5^(age / half_life)
        decay = math.exp(-age / half_life_seconds * math.log(2))
        effective_severity = sig.severity * decay
        w = effective_severity * t
        contributions.append(
            WeightedContribution(
                reporter_quid=sig.reporter_quid,
                severity=effective_severity,
                observer_trust=t,
                weight=w,
                pattern_type=sig.pattern_type,
            )
        )
        if t >= min_trust_to_count:
            total_weighted_severity += w
            total_trust += t

    score = 0.0
    if total_trust > 0:
        score = total_weighted_severity / total_trust

    return AggregateScore(
        score=score,
        contributions=contributions,
        signal_count=len(signals),
        total_trust_weight=total_trust,
    )
