"""Tests for credit_evaluate.py. No Quidnug node required."""

import pytest

from credit_evaluate import (
    CreditEvent,
    CreditVerdict,
    Dispute,
    EVENT_DEFAULT,
    EVENT_DISPUTE,
    EVENT_LOAN_CLOSED,
    EVENT_LOAN_ORIGINATED,
    EVENT_PAYMENT_MISSED,
    EVENT_PAYMENT_ON_TIME,
    EVENT_UTILITY_ON_TIME,
    LenderPolicy,
    evaluate_borrower,
    extract_credit_events,
    extract_disputes,
)


NOW = 1_700_000_000


def _ev(
    event_type: str, attester: str, *, loan_id: str = "loan-1",
    category: str = "auto-loan", subject: str = "subject-alice",
) -> dict:
    return {
        "eventType": event_type,
        "timestamp": NOW,
        "payload": {
            "attester": attester, "subject": subject,
            "category": category, "amountBand": "10k-30k",
            "loanId": loan_id, "timestamp": NOW,
        },
    }


def _dispute(
    event_type: str, attester: str, loan_id: str = "loan-1",
    reason: str = "not my loan",
) -> dict:
    return {
        "eventType": EVENT_DISPUTE,
        "timestamp": NOW + 3600,
        "payload": {
            "disputesEventType": event_type,
            "disputesAttester": attester,
            "disputesLoanId": loan_id,
            "reason": reason,
            "filedAt": NOW + 3600,
        },
    }


def _trust(mapping):
    return lambda obs, target: mapping.get((obs, target), 0.0)


# ---------------------------------------------------------------------------
# Clean positive history
# ---------------------------------------------------------------------------

def test_clean_payment_history_approves():
    events = (
        [_ev(EVENT_LOAN_ORIGINATED, "lender-a")]
        + [_ev(EVENT_PAYMENT_ON_TIME, "lender-a") for _ in range(12)]
        + [_ev(EVENT_LOAN_CLOSED, "lender-a")]
    )
    trust = _trust({("lender-b", "lender-a"): 0.9})
    v = evaluate_borrower("lender-b", "subject-alice", events, trust)
    assert v.verdict == "approve"
    assert v.positive_signal > 0
    assert v.negative_signal == 0


# ---------------------------------------------------------------------------
# Recent default
# ---------------------------------------------------------------------------

def test_recent_default_declines():
    events = [_ev(EVENT_DEFAULT, "lender-a")]
    trust = _trust({("lender-b", "lender-a"): 0.9})
    v = evaluate_borrower("lender-b", "subject-alice", events, trust)
    assert v.verdict == "decline"


# ---------------------------------------------------------------------------
# Disputed negative event is discounted
# ---------------------------------------------------------------------------

def test_dispute_discounts_negative_event():
    events = [
        _ev(EVENT_DEFAULT, "fraudulent-lender"),
        _dispute(EVENT_DEFAULT, "fraudulent-lender"),
        # And a handful of positive records from a trusted source.
        _ev(EVENT_LOAN_ORIGINATED, "lender-a", loan_id="loan-2"),
        _ev(EVENT_PAYMENT_ON_TIME, "lender-a", loan_id="loan-2"),
        _ev(EVENT_PAYMENT_ON_TIME, "lender-a", loan_id="loan-2"),
    ]
    trust = _trust({
        ("lender-b", "fraudulent-lender"): 0.9,
        ("lender-b", "lender-a"):          0.9,
    })
    v = evaluate_borrower("lender-b", "subject-alice", events, trust)
    assert v.verdict in ("indeterminate", "approve")
    # Without the dispute, the default would have made the
    # score negative enough to decline.
    assert v.negative_signal == 0.0


# ---------------------------------------------------------------------------
# Untrusted attester ignored
# ---------------------------------------------------------------------------

def test_untrusted_attester_ignored():
    events = [_ev(EVENT_DEFAULT, "rogue-lender")]
    # Default trust for "rogue-lender" is 0 -> below the 0.3 floor.
    trust = _trust({("lender-b", "rogue-lender"): 0.1})
    v = evaluate_borrower("lender-b", "subject-alice", events, trust)
    # The default should not count; score stays neutral -> indeterminate.
    assert v.verdict == "indeterminate"
    assert v.negative_signal == 0.0


# ---------------------------------------------------------------------------
# Observer-relative: two lenders reach different verdicts
# ---------------------------------------------------------------------------

def test_observer_relative_verdicts():
    events = (
        [_ev(EVENT_LOAN_ORIGINATED, "alt-lender-union")]
        + [_ev(EVENT_PAYMENT_ON_TIME, "alt-lender-union")
           for _ in range(8)]
    )
    # Big bank doesn't trust the alt lender much.
    trust_big_bank = _trust({
        ("big-bank", "alt-lender-union"): 0.25,   # just below the floor
    })
    # Credit union strongly trusts its peer.
    trust_alt_bank = _trust({
        ("alt-bank", "alt-lender-union"): 0.9,
    })
    big = evaluate_borrower("big-bank", "subject-alice", events, trust_big_bank)
    alt = evaluate_borrower("alt-bank", "subject-alice", events, trust_alt_bank)
    assert big.verdict == "indeterminate"
    assert alt.verdict == "approve"


# ---------------------------------------------------------------------------
# Alt data
# ---------------------------------------------------------------------------

def test_alt_data_helps_thin_file():
    """Subject has no loan history, but 24 months of utility
    on-time payments from a trusted utility -- should get
    above indeterminate threshold."""
    events = [_ev(EVENT_UTILITY_ON_TIME, "utility-conEd",
                   category="utility") for _ in range(24)]
    trust = _trust({
        ("lender-b", "utility-conEd"): 0.8,
    })
    v = evaluate_borrower("lender-b", "subject-alice", events, trust)
    assert v.alt_data_signal > 0
    assert v.verdict in ("indeterminate", "approve")


# ---------------------------------------------------------------------------
# Category filter
# ---------------------------------------------------------------------------

def test_category_filter_restricts_considered_events():
    events = [
        _ev(EVENT_LOAN_ORIGINATED, "lender-a", category="auto-loan"),
        _ev(EVENT_PAYMENT_ON_TIME, "lender-a", category="auto-loan"),
        _ev(EVENT_DEFAULT, "lender-m", category="mortgage"),
    ]
    trust = _trust({("lender-b", "lender-a"): 0.9,
                    ("lender-b", "lender-m"): 0.9})
    # A mortgage lender filtering by category "mortgage" should
    # only count the default.
    mortgage_policy = LenderPolicy(relevant_categories=["mortgage"])
    mortgage_view = evaluate_borrower(
        "lender-b", "subject-alice", events, trust, mortgage_policy,
    )
    assert mortgage_view.verdict == "decline"

    # An auto-loan lender only sees the clean auto record.
    auto_policy = LenderPolicy(relevant_categories=["auto-loan"])
    auto_view = evaluate_borrower(
        "lender-b", "subject-alice", events, trust, auto_policy,
    )
    assert auto_view.verdict != "decline"


# ---------------------------------------------------------------------------
# Out-of-range trust
# ---------------------------------------------------------------------------

def test_trust_out_of_range_raises():
    events = [_ev(EVENT_PAYMENT_ON_TIME, "lender-a")]
    trust = _trust({("lender-b", "lender-a"): 1.5})
    with pytest.raises(ValueError):
        evaluate_borrower("lender-b", "subject-alice", events, trust)


# ---------------------------------------------------------------------------
# Subject filter
# ---------------------------------------------------------------------------

def test_events_for_other_subjects_ignored():
    events = [
        _ev(EVENT_DEFAULT, "lender-a", subject="subject-bob"),
        _ev(EVENT_PAYMENT_ON_TIME, "lender-a", subject="subject-alice"),
    ]
    trust = _trust({("lender-b", "lender-a"): 0.9})
    v = evaluate_borrower("lender-b", "subject-alice", events, trust)
    assert v.negative_signal == 0


# ---------------------------------------------------------------------------
# Extraction
# ---------------------------------------------------------------------------

def test_extract_credit_events():
    events = [_ev(EVENT_PAYMENT_ON_TIME, "lender-a"),
              {"eventType": "something-else", "payload": {}}]
    out = extract_credit_events(events)
    assert len(out) == 1
    assert out[0].event_type == EVENT_PAYMENT_ON_TIME


def test_extract_disputes():
    events = [_dispute(EVENT_DEFAULT, "fraudulent-lender")]
    out = extract_disputes(events)
    assert len(out) == 1
    assert out[0].disputes_event_type == EVENT_DEFAULT
