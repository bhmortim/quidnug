"""Tests for consent_evaluate.py. No Quidnug node required."""

import pytest

from consent_evaluate import (
    AccessPolicy,
    AccessRequest,
    AccessVerdict,
    CATEGORIES,
    ConsentGrant,
    EmergencyOverride,
    Revocation,
    evaluate_access,
    extract_emergency_overrides,
    extract_revocations,
)


NOW = 1_700_000_000
IN_90_DAYS = NOW + 90 * 86400
LONG_PAST = NOW - 90 * 86400


def _request(
    provider: str, category: str = "clinical-notes",
    patient: str = "patient-alice",
) -> AccessRequest:
    return AccessRequest(
        provider_quid=provider, patient_quid=patient,
        category=category, requested_at_unix=NOW,
        purpose="routine care",
    )


def _grant(
    provider: str, *, category: str = "clinical-notes",
    trust: float = 0.9, patient: str = "patient-alice",
    valid_until: int = IN_90_DAYS, granted_at: int = NOW - 86400,
) -> ConsentGrant:
    return ConsentGrant(
        patient_quid=patient, provider_quid=provider,
        category=category, trust_level=trust,
        granted_at_unix=granted_at, valid_until_unix=valid_until,
        description="test grant",
    )


def _revocation_event(
    provider: str, category: str = "",
    patient: str = "patient-alice", revoked_at: int = NOW,
) -> dict:
    return {
        "eventType": "consent.revoked",
        "timestamp": revoked_at,
        "payload": {
            "patient": patient, "provider": provider,
            "category": category, "revokedAt": revoked_at,
            "reason": "patient decision",
        },
    }


def _emergency_event(
    provider: str, guardian_sigs: list, *,
    valid_until: int = NOW + 3600,
    patient: str = "patient-alice",
) -> dict:
    return {
        "eventType": "consent.emergency-override",
        "timestamp": NOW,
        "payload": {
            "patient": patient, "provider": provider,
            "guardianSignatures": guardian_sigs,
            "validUntil": valid_until,
            "reason": "patient unconscious in ER",
        },
    }


def _trust(mapping):
    return lambda a, b: mapping.get((a, b), 0.0)


# ---------------------------------------------------------------------------
# Direct consent
# ---------------------------------------------------------------------------

def test_direct_consent_allows():
    v = evaluate_access(
        _request("dr-jones"),
        [_grant("dr-jones")],
        events=[], trust_fn=_trust({}),
    )
    assert v.verdict == "allow"
    assert v.consent_path == ["dr-jones"]


def test_no_consent_denies():
    v = evaluate_access(
        _request("dr-random"), [], events=[], trust_fn=_trust({}),
    )
    assert v.verdict == "deny"


def test_expired_consent_denies():
    v = evaluate_access(
        _request("dr-jones"),
        [_grant("dr-jones", valid_until=LONG_PAST)],
        events=[], trust_fn=_trust({}),
    )
    assert v.verdict == "deny"


def test_low_trust_direct_denies():
    v = evaluate_access(
        _request("dr-jones"),
        [_grant("dr-jones", trust=0.2)],
        events=[], trust_fn=_trust({}),
    )
    assert v.verdict == "deny"


# ---------------------------------------------------------------------------
# Category filter
# ---------------------------------------------------------------------------

def test_category_mismatch_denies():
    v = evaluate_access(
        _request("dr-jones", category="mental-health"),
        [_grant("dr-jones", category="clinical-notes")],
        events=[], trust_fn=_trust({}),
    )
    assert v.verdict == "deny"


def test_unknown_category_denies():
    v = evaluate_access(
        _request("dr-jones", category="astrology-readings"),
        [_grant("dr-jones", category="astrology-readings")],
        events=[], trust_fn=_trust({}),
    )
    assert v.verdict == "deny"
    assert "unknown category" in v.reasons[0]


# ---------------------------------------------------------------------------
# Revocation
# ---------------------------------------------------------------------------

def test_revoked_consent_denies():
    v = evaluate_access(
        _request("dr-jones"),
        [_grant("dr-jones", granted_at=NOW - 86400 * 30)],
        events=[_revocation_event("dr-jones")],
        trust_fn=_trust({}),
    )
    assert v.verdict == "deny"
    assert any("revoked" in r.lower() for r in v.reasons)


def test_all_category_revocation_denies_any_category():
    v = evaluate_access(
        _request("dr-jones", category="labs"),
        [_grant("dr-jones", category="labs",
                 granted_at=NOW - 86400 * 30)],
        events=[_revocation_event("dr-jones", category="")],   # all
        trust_fn=_trust({}),
    )
    assert v.verdict == "deny"


# ---------------------------------------------------------------------------
# Transitive consent
# ---------------------------------------------------------------------------

def test_transitive_consent_through_pcp_allows():
    """Patient consents to PCP at 0.9; PCP trusts specialist at
    0.8. Composed trust 0.72 -> allow."""
    grants = [_grant("pcp", trust=0.9)]
    trust = _trust({("pcp", "specialist"): 0.8})
    v = evaluate_access(
        _request("specialist"),
        grants, events=[], trust_fn=trust,
    )
    assert v.verdict == "allow"
    assert "pcp" in v.consent_path
    assert v.effective_trust == pytest.approx(0.72)


def test_transitive_chain_below_threshold_denies():
    """Composed trust 0.9 * 0.3 = 0.27 is below threshold."""
    grants = [_grant("pcp", trust=0.9)]
    trust = _trust({("pcp", "specialist"): 0.3})
    v = evaluate_access(
        _request("specialist"),
        grants, events=[], trust_fn=trust,
    )
    assert v.verdict == "deny"


# ---------------------------------------------------------------------------
# Emergency override
# ---------------------------------------------------------------------------

def test_emergency_override_with_quorum_allows():
    guardian_set = {"spouse", "adult-child", "healthcare-proxy"}
    events = [_emergency_event(
        "er-doc-cooper",
        guardian_sigs=["spouse", "healthcare-proxy"],
    )]
    v = evaluate_access(
        _request("er-doc-cooper"),
        consents=[], events=events, trust_fn=_trust({}),
        guardian_set=guardian_set, guardian_threshold=2,
    )
    assert v.verdict == "emergency-allowed"
    assert "emergency" in v.reasons[-1].lower()


def test_emergency_override_below_quorum_denies():
    guardian_set = {"spouse", "adult-child", "healthcare-proxy"}
    events = [_emergency_event(
        "er-doc-cooper",
        guardian_sigs=["spouse"],   # only 1
    )]
    v = evaluate_access(
        _request("er-doc-cooper"),
        consents=[], events=events, trust_fn=_trust({}),
        guardian_set=guardian_set, guardian_threshold=2,
    )
    assert v.verdict == "deny"


def test_emergency_override_expired_denies():
    guardian_set = {"spouse", "adult-child"}
    events = [_emergency_event(
        "er-doc-cooper",
        guardian_sigs=["spouse", "adult-child"],
        valid_until=NOW - 3600,
    )]
    v = evaluate_access(
        _request("er-doc-cooper"),
        consents=[], events=events, trust_fn=_trust({}),
        guardian_set=guardian_set, guardian_threshold=2,
    )
    assert v.verdict == "deny"


def test_emergency_override_with_unknown_signatures_denies():
    """Guardian signature set is enforced: signatures from
    non-guardians don't count."""
    guardian_set = {"spouse"}
    events = [_emergency_event(
        "er-doc-cooper",
        guardian_sigs=["attacker-quid", "random-quid"],
    )]
    v = evaluate_access(
        _request("er-doc-cooper"),
        consents=[], events=events, trust_fn=_trust({}),
        guardian_set=guardian_set, guardian_threshold=1,
    )
    assert v.verdict == "deny"


# ---------------------------------------------------------------------------
# Stream extraction
# ---------------------------------------------------------------------------

def test_extract_revocations():
    revs = extract_revocations([_revocation_event("dr-jones")])
    assert len(revs) == 1
    assert revs[0].provider_quid == "dr-jones"


def test_extract_emergency_overrides():
    ovs = extract_emergency_overrides([_emergency_event(
        "er-doc", ["spouse", "proxy"],
    )])
    assert len(ovs) == 1
    assert "spouse" in ovs[0].guardian_signatures
