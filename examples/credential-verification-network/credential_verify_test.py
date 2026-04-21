"""Tests for credential_verify.py — no Quidnug node required."""

import pytest

from credential_verify import (
    CredentialV1,
    verify_credential,
    verify_batch,
)


def make_credential(
    issuer: str = "stanford",
    subject: str = "alice",
    ctype: str = "degree.bachelors.cs",
    grade: str = "3.8",
    credential_id: str = "cred-1",
) -> CredentialV1:
    return CredentialV1(
        credential_id=credential_id,
        issuer_quid=issuer,
        subject_quid=subject,
        credential_type=ctype,
        grade=grade,
        issued_at_unix=1_700_000_000,
    )


def static_trust(graph):
    def fn(observer, issuer):
        return graph.get((observer, issuer), 0.0)
    return fn


def no_revocations(credential_id):
    return None


def test_direct_trust_accepts():
    """Observer directly trusts the issuer at high confidence → accept."""
    cred = make_credential()
    trust = static_trust({("employer", "stanford"): 0.95})
    verdict = verify_credential("employer", cred, trust, no_revocations)
    assert verdict.verdict == "accept"
    assert verdict.trust_path_score == pytest.approx(0.95)


def test_transitive_trust_accepts():
    """Observer doesn't know issuer directly but trusts an
    accreditor who trusts the issuer. Composed trust 0.95*0.9 = 0.855 → accept."""
    cred = make_credential()
    # Simulate the trust-path-fn returning the composed trust
    # after the trust-graph walk. (Real node's ComputeRelationalTrust
    # does this for us.)
    trust = static_trust({("employer", "stanford"): 0.9 * 0.95})
    verdict = verify_credential("employer", cred, trust, no_revocations)
    assert verdict.verdict == "accept"


def test_no_trust_path_rejects():
    cred = make_credential()
    trust = static_trust({})  # no edge at all
    verdict = verify_credential("employer", cred, trust, no_revocations)
    assert verdict.verdict == "reject"
    assert "no trust path" in " ".join(verdict.reasons).lower()


def test_low_trust_indeterminate():
    """Non-zero but below threshold → indeterminate (manual
    review)."""
    cred = make_credential()
    trust = static_trust({("employer", "stanford"): 0.4})
    verdict = verify_credential("employer", cred, trust, no_revocations, min_accept_score=0.6)
    assert verdict.verdict == "indeterminate"
    assert verdict.score == pytest.approx(0.4)


def test_revocation_rejects_regardless_of_trust():
    """Even if trust is high, a revocation overrides."""
    cred = make_credential()
    trust = static_trust({("employer", "stanford"): 1.0})
    revoker = lambda cid: "disciplinary-action" if cid == "cred-1" else None
    verdict = verify_credential("employer", cred, trust, revoker)
    assert verdict.verdict == "reject"
    assert "revoked" in verdict.reasons[0].lower()


def test_custom_threshold():
    """Observer with lenient policy (min=0.3) accepts where the
    default (0.6) would say indeterminate."""
    cred = make_credential()
    trust = static_trust({("employer", "stanford"): 0.4})
    v1 = verify_credential("employer", cred, trust, no_revocations, min_accept_score=0.6)
    v2 = verify_credential("employer", cred, trust, no_revocations, min_accept_score=0.3)
    assert v1.verdict == "indeterminate"
    assert v2.verdict == "accept"


def test_trust_out_of_range_rejects():
    cred = make_credential()
    trust = static_trust({("employer", "stanford"): 1.5})
    with pytest.raises(ValueError):
        verify_credential("employer", cred, trust, no_revocations)


def test_batch_verification():
    creds = [
        make_credential(issuer="stanford",   credential_id="c1"),
        make_credential(issuer="mit",        credential_id="c2"),
        make_credential(issuer="unknownU",   credential_id="c3"),
    ]
    trust = static_trust({
        ("employer", "stanford"): 0.9,
        ("employer", "mit"): 0.8,
        # unknownU: no edge at all
    })
    summary = verify_batch("employer", creds, trust, no_revocations)
    assert summary.total == 3
    assert summary.accepted == 2
    assert summary.rejected == 1
    assert summary.indeterminate == 0


def test_observer_differs_yields_different_verdicts():
    """Core relational-trust property: two observers with
    different trust graphs reach different verdicts on the
    same credential."""
    cred = make_credential(issuer="accredited-only-in-APAC-uni")
    trust = static_trust({
        # US employer doesn't recognize APAC accreditation.
        ("us-employer", "accredited-only-in-APAC-uni"): 0.0,
        # APAC employer does.
        ("apac-employer", "accredited-only-in-APAC-uni"): 0.9,
    })
    us = verify_credential("us-employer", cred, trust, no_revocations)
    apac = verify_credential("apac-employer", cred, trust, no_revocations)
    assert us.verdict == "reject"
    assert apac.verdict == "accept"


def test_credential_describe():
    """Sanity check on the display helper."""
    cred = make_credential(
        issuer="stanford-12345678",
        subject="alice-abcdef12",
        ctype="degree.bachelors.cs",
        grade="3.8",
    )
    s = cred.describe()
    assert "degree.bachelors.cs" in s
    assert "3.8" in s
