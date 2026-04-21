"""Tests for fl_audit.py. No Quidnug node required."""

import pytest

from fl_audit import (
    Aggregation,
    GradientSubmission,
    Registration,
    RoundBreakdown,
    RoundPolicy,
    RoundVerdict,
    audit_round,
    extract_aggregation,
    extract_registrations,
    extract_submissions,
    fair_weights_by_data_size,
)


NOW = 1_700_000_000


def _regs(participants: list) -> list:
    return [
        {
            "eventType": "participant.registered",
            "payload": {
                "participant": p, "attestedDataSize": 1_000_000,
                "registeredAt": NOW,
            },
        }
        for p in participants
    ]


def _subs(participants_and_norms: list) -> list:
    out = []
    for (p, norm) in participants_and_norms:
        out.append({
            "eventType": "gradient.submitted",
            "payload": {
                "participant": p,
                "gradientCID": f"bafy...{p}",
                "gradientHash": f"hash-{p}",
                "gradientNorm": norm,
                "trainingDataSize": 1_000_000,
                "submittedAt": NOW + 300,
            },
        })
    return out


def _agg(coordinator: str = "coord") -> list:
    return [{
        "eventType": "round.aggregated",
        "payload": {
            "coordinator": coordinator,
            "aggregateHash": "hash-agg",
            "participantWeights": {},
            "aggregatedAt": NOW + 600,
        },
    }]


# ---------------------------------------------------------------------------
# Happy path
# ---------------------------------------------------------------------------

def test_full_round_valid():
    events = (
        _regs(["A", "B", "C", "D", "E"])
        + _subs([("A", 1.0), ("B", 1.1), ("C", 1.05), ("D", 0.95), ("E", 1.0)])
        + _agg()
    )
    v = audit_round("r-1", events)
    assert v.verdict == "valid"
    assert len(v.breakdown.submitted_participants) == 5
    assert v.breakdown.aggregation_present


# ---------------------------------------------------------------------------
# Insufficient participation
# ---------------------------------------------------------------------------

def test_too_few_submissions_insufficient():
    events = _regs(["A", "B", "C"]) + _subs([("A", 1.0), ("B", 1.0)]) + _agg()
    policy = RoundPolicy(min_participants=5)
    v = audit_round("r-2", events, policy)
    assert v.verdict == "insufficient"


# ---------------------------------------------------------------------------
# Missing participants
# ---------------------------------------------------------------------------

def test_missing_registered_submitter_strict_policy():
    """Strict policy: registered-but-no-submit is a violation."""
    events = (
        _regs(["A", "B", "C", "D", "E"])
        + _subs([("A", 1.0), ("B", 1.0), ("C", 1.0), ("D", 1.0)])
        # E registered but didn't submit.
        + _agg()
    )
    policy = RoundPolicy(min_participants=3, strict_registration=True)
    v = audit_round("r-3", events, policy)
    assert v.verdict == "integrity-violation"
    assert "E" in v.breakdown.missing_participants


def test_missing_registered_submitter_non_strict_still_valid():
    events = (
        _regs(["A", "B", "C", "D", "E"])
        + _subs([("A", 1.0), ("B", 1.0), ("C", 1.0), ("D", 1.0)])
        + _agg()
    )
    policy = RoundPolicy(min_participants=3, strict_registration=False)
    v = audit_round("r-4", events, policy)
    assert v.verdict == "valid"


# ---------------------------------------------------------------------------
# Suspicious gradients
# ---------------------------------------------------------------------------

def test_suspicious_gradient_flagged_but_not_invalidating():
    events = (
        _regs(["A", "B", "C", "D", "E"])
        + _subs([
            ("A", 1.0), ("B", 1.0), ("C", 1.05), ("D", 0.95),
            ("E", 50.0),   # wildly large norm
        ])
        + _agg()
    )
    v = audit_round("r-5", events)
    # Still valid (just flagged).
    assert v.verdict == "valid"
    assert len(v.breakdown.suspicious_gradients) == 1
    assert "E" in v.breakdown.suspicious_gradients[0]


def test_all_gradients_within_bounds_no_flags():
    events = (
        _regs(["A", "B", "C", "D", "E"])
        + _subs([(p, 1.0) for p in "ABCDE"])
        + _agg()
    )
    v = audit_round("r-6", events)
    assert v.verdict == "valid"
    assert v.breakdown.suspicious_gradients == []


# ---------------------------------------------------------------------------
# Missing aggregation
# ---------------------------------------------------------------------------

def test_missing_aggregation_is_incomplete():
    events = (
        _regs(["A", "B", "C", "D", "E"])
        + _subs([(p, 1.0) for p in "ABCDE"])
        # no _agg()
    )
    v = audit_round("r-7", events)
    assert v.verdict == "incomplete"


def test_policy_can_waive_aggregation_requirement():
    events = (
        _regs(["A", "B", "C", "D", "E"])
        + _subs([(p, 1.0) for p in "ABCDE"])
    )
    policy = RoundPolicy(require_aggregation_event=False)
    v = audit_round("r-8", events, policy)
    assert v.verdict == "valid"


# ---------------------------------------------------------------------------
# Fair weights
# ---------------------------------------------------------------------------

def test_fair_weights_match_data_size_proportion():
    regs = extract_registrations([
        {"eventType": "participant.registered",
         "payload": {"participant": "A", "attestedDataSize": 100}},
        {"eventType": "participant.registered",
         "payload": {"participant": "B", "attestedDataSize": 300}},
    ])
    subs = extract_submissions([
        {"eventType": "gradient.submitted",
         "payload": {"participant": "A", "trainingDataSize": 100}},
        {"eventType": "gradient.submitted",
         "payload": {"participant": "B", "trainingDataSize": 300}},
    ])
    fair = fair_weights_by_data_size(regs, subs)
    assert fair["A"] == pytest.approx(0.25)
    assert fair["B"] == pytest.approx(0.75)


def test_fair_weights_takes_min_of_attested_and_submitted():
    regs = extract_registrations([
        {"eventType": "participant.registered",
         "payload": {"participant": "A", "attestedDataSize": 100}},
        {"eventType": "participant.registered",
         "payload": {"participant": "B", "attestedDataSize": 10000}},
    ])
    # B claimed 10000 at registration but only used 200 at submission.
    subs = extract_submissions([
        {"eventType": "gradient.submitted",
         "payload": {"participant": "A", "trainingDataSize": 100}},
        {"eventType": "gradient.submitted",
         "payload": {"participant": "B", "trainingDataSize": 200}},
    ])
    fair = fair_weights_by_data_size(regs, subs)
    # Fair weight uses min, so A=100, B=200 -> share 1/3 vs 2/3.
    assert fair["A"] == pytest.approx(1/3)
    assert fair["B"] == pytest.approx(2/3)


# ---------------------------------------------------------------------------
# Stream extraction
# ---------------------------------------------------------------------------

def test_extract_aggregation_returns_last():
    events = [
        {"eventType": "round.aggregated",
         "payload": {"coordinator": "c1", "aggregateHash": "h1"}},
        {"eventType": "round.aggregated",
         "payload": {"coordinator": "c2", "aggregateHash": "h2"}},
    ]
    agg = extract_aggregation(events)
    assert agg.coordinator_quid == "c2"
    assert agg.aggregate_hash == "h2"
