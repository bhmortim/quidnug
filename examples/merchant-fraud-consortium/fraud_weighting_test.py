"""Tests for fraud_weighting.py — no Quidnug node required."""

import pytest

from fraud_weighting import (
    AggregateScore,
    FraudSignal,
    aggregate_fraud_score,
    aggregate_with_decay,
)


# --- Fixtures ---


def make_signals() -> list[FraudSignal]:
    """4-signal scenario: all reporters flagging the same card."""
    return [
        FraudSignal("acme", 0.9, "card-testing", 1_700_000_000),
        FraudSignal("bigbox", 0.9, "velocity-abuse", 1_700_000_100),
        FraudSignal("startup", 0.7, "geo-anomaly", 1_700_000_200),
        FraudSignal("noisy", 0.5, "unclear", 1_700_000_300),
    ]


def static_trust(trust_map: dict[str, dict[str, float]]):
    """Build a ``trust_fn`` from a static (observer -> reporter -> level) map."""
    def fn(observer: str, reporter: str) -> float:
        return trust_map.get(observer, {}).get(reporter, 0.0)
    return fn


# --- Core tests ---


def test_single_signal_from_trusted_reporter():
    signals = [FraudSignal("acme", 0.9, "card-testing", 1_700_000_000)]
    tf = static_trust({"observer": {"acme": 1.0}})
    result = aggregate_fraud_score("observer", signals, tf)
    assert result.score == pytest.approx(0.9)
    assert result.signal_count == 1
    assert len(result.contributions) == 1


def test_no_signals_zero_score():
    result = aggregate_fraud_score("observer", [], static_trust({}))
    assert result.score == 0.0
    assert result.signal_count == 0


def test_all_zero_trust_zero_score():
    """When the observer trusts none of the reporters, score is 0
    (no evidence from their perspective)."""
    signals = make_signals()
    tf = static_trust({"observer": {}})  # no trust in anyone
    result = aggregate_fraud_score("observer", signals, tf)
    assert result.score == 0.0
    # But the contributions are still listed (transparency).
    assert len(result.contributions) == 4


def test_trusted_reporters_dominate_aggregate():
    """Signals from highly-trusted reporters should dominate
    the score. Low-trust reporters contribute little."""
    signals = make_signals()
    tf = static_trust({
        "observer": {
            "acme": 0.9,
            "bigbox": 0.9,
            "startup": 0.6,
            "noisy": 0.1,
        },
    })
    result = aggregate_fraud_score("observer", signals, tf)
    # Manual calc:
    #   w(acme)    = 0.9 * 0.9 = 0.81
    #   w(bigbox)  = 0.9 * 0.9 = 0.81
    #   w(startup) = 0.7 * 0.6 = 0.42
    #   w(noisy)   = 0.5 * 0.1 = 0.05
    #   total_trust = 0.9 + 0.9 + 0.6 + 0.1 = 2.5
    #   score = (0.81+0.81+0.42+0.05) / 2.5 = 2.09 / 2.5 = 0.836
    assert result.score == pytest.approx(0.836, abs=0.01)


def test_min_trust_threshold_excludes_noise():
    """With a min-trust filter, noisy reporters' signals are
    excluded from the denominator even while appearing in the
    contributions list."""
    signals = make_signals()
    tf = static_trust({
        "observer": {
            "acme": 0.9,
            "bigbox": 0.9,
            "startup": 0.6,
            "noisy": 0.1,
        },
    })
    result = aggregate_fraud_score(
        "observer", signals, tf, min_trust_to_count=0.5,
    )
    # Without noisy + excluding sub-0.5 trust:
    #   Contributing: acme, bigbox, startup
    #   numerator = 0.81 + 0.81 + 0.42 = 2.04
    #   denom = 0.9 + 0.9 + 0.6 = 2.4
    #   score = 2.04 / 2.4 = 0.85
    assert result.score == pytest.approx(0.85, abs=0.01)
    # Contributions still show noisy (with 0.1 trust).
    assert len(result.contributions) == 4


def test_opposing_observers_differ():
    """Two observers with different trust graphs see different
    aggregate scores for the same signals. This is the core
    'relational trust' property."""
    signals = make_signals()
    tf = static_trust({
        "trusting-observer": {
            "acme": 0.9, "bigbox": 0.9, "startup": 0.6, "noisy": 0.1,
        },
        "skeptical-observer": {
            # Skeptic only trusts acme; sees the other three as
            # unknown (trust 0).
            "acme": 0.5,
        },
    })
    r1 = aggregate_fraud_score("trusting-observer", signals, tf)
    r2 = aggregate_fraud_score("skeptical-observer", signals, tf)
    # Both converge on a "is this card fraud" number but from
    # their own lens.
    # r1's score should be about 0.836 (all 4 weighted in).
    assert r1.score == pytest.approx(0.836, abs=0.01)
    # r2's score = only acme's signal counts:
    #   num = 0.9 * 0.5 = 0.45
    #   denom = 0.5
    #   score = 0.9 (just acme's severity)
    assert r2.score == pytest.approx(0.9, abs=0.001)


def test_trust_out_of_range_rejects():
    signals = [FraudSignal("acme", 0.9, "x", 0)]
    tf = static_trust({"obs": {"acme": 1.5}})  # invalid
    with pytest.raises(ValueError):
        aggregate_fraud_score("obs", signals, tf)


# --- Decay tests ---


def test_decay_reduces_old_signals():
    signals = [
        FraudSignal("acme", 1.0, "card-testing", 1_700_000_000),
    ]
    tf = static_trust({"observer": {"acme": 1.0}})

    # Fresh (0 age): no decay → full 1.0 severity.
    fresh = aggregate_with_decay(
        "observer", signals, tf, now_unix=1_700_000_000,
        half_life_seconds=30 * 24 * 3600,
    )
    assert fresh.score == pytest.approx(1.0, abs=0.001)

    # One half-life later (30 days): severity = 0.5.
    aged = aggregate_with_decay(
        "observer", signals, tf, now_unix=1_700_000_000 + 30 * 24 * 3600,
        half_life_seconds=30 * 24 * 3600,
    )
    assert aged.score == pytest.approx(0.5, abs=0.001)

    # Many half-lives later: approaches 0.
    ancient = aggregate_with_decay(
        "observer", signals, tf, now_unix=1_700_000_000 + 365 * 24 * 3600,
        half_life_seconds=30 * 24 * 3600,
    )
    assert ancient.score < 0.01


def test_decay_and_trust_compose():
    """Decay and trust are independent dimensions; both apply."""
    signals = [FraudSignal("acme", 1.0, "x", 0)]
    tf = static_trust({"obs": {"acme": 0.5}})
    # 1 half-life old → severity halved; trust 0.5 does not
    # change the average (the only signal). Score == 0.5.
    r = aggregate_with_decay(
        "obs", signals, tf, now_unix=30 * 24 * 3600,
        half_life_seconds=30 * 24 * 3600,
    )
    assert r.score == pytest.approx(0.5, abs=0.001)
