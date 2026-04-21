"""Tests for artifact_verify.py. No Quidnug node required."""

import hashlib

import pytest

from artifact_verify import (
    ArtifactVerdict,
    ReleaseV1,
    is_revoked,
    sha256_hex,
    unpatched_vulnerabilities,
    unpatched_high_sev,
    verify_artifact,
    verify_batch,
)


TARBALL = b"fake tarball bytes for testing"
TARBALL_HASH = hashlib.sha256(TARBALL).hexdigest()


def _release(
    release_id: str = "webapp-js-2.3.1",
    maintainer: str = "alice",
    artifact_hash: str = TARBALL_HASH,
) -> ReleaseV1:
    return ReleaseV1(
        release_id=release_id,
        package_name="webapp-js",
        version="2.3.1",
        maintainer_quid=maintainer,
        artifact_hash_hex=artifact_hash,
        repository="github.com/acme/webapp-js",
        commit_hash="abc1234567",
        published_at_unix=1_700_000_000,
    )


def _trust(mapping):
    return lambda obs, maint: mapping.get((obs, maint), 0.0)


# ---------------------------------------------------------------------------
# Hash match
# ---------------------------------------------------------------------------

def test_hash_match_accepts():
    v = verify_artifact(
        "consumer", _release(), TARBALL, events=[],
        trust_path_fn=_trust({("consumer", "alice"): 0.9}),
    )
    assert v.verdict == "accept"


def test_hash_mismatch_rejects():
    v = verify_artifact(
        "consumer", _release(), b"tampered bytes", events=[],
        trust_path_fn=_trust({("consumer", "alice"): 0.9}),
    )
    assert v.verdict == "reject"
    assert "hash mismatch" in v.reasons[0].lower()


def test_hash_mismatch_overrides_trust():
    """Even at max trust, a hash mismatch is a hard reject."""
    v = verify_artifact(
        "consumer", _release(), b"tampered", events=[],
        trust_path_fn=_trust({("consumer", "alice"): 1.0}),
    )
    assert v.verdict == "reject"


# ---------------------------------------------------------------------------
# Revocation
# ---------------------------------------------------------------------------

def test_revoked_release_rejects():
    events = [{
        "eventType": "release.revoked",
        "payload": {"reason": "key-compromise"},
    }]
    v = verify_artifact(
        "consumer", _release(), TARBALL, events=events,
        trust_path_fn=_trust({("consumer", "alice"): 0.9}),
    )
    assert v.verdict == "reject"
    assert "revoked" in " ".join(v.reasons).lower()


def test_is_revoked_returns_reason():
    assert is_revoked([]) is None
    assert is_revoked([
        {"eventType": "release.published"},
        {"eventType": "release.revoked", "payload": {"reason": "compromise"}},
    ]) == "compromise"


# ---------------------------------------------------------------------------
# Trust
# ---------------------------------------------------------------------------

def test_low_trust_rejects():
    v = verify_artifact(
        "consumer", _release(), TARBALL, events=[],
        trust_path_fn=_trust({("consumer", "alice"): 0.3}),
        min_trust=0.5,
    )
    assert v.verdict == "reject"
    assert "below threshold" in " ".join(v.reasons).lower()


def test_no_trust_path_rejects():
    v = verify_artifact(
        "consumer", _release(), TARBALL, events=[],
        trust_path_fn=_trust({}),
    )
    assert v.verdict == "reject"


def test_custom_threshold():
    """Consumer with lenient policy accepts at trust 0.3."""
    v = verify_artifact(
        "consumer", _release(), TARBALL, events=[],
        trust_path_fn=_trust({("consumer", "alice"): 0.4}),
        min_trust=0.3,
    )
    assert v.verdict == "accept"


def test_trust_out_of_range_raises():
    with pytest.raises(ValueError):
        verify_artifact(
            "consumer", _release(), TARBALL, events=[],
            trust_path_fn=_trust({("consumer", "alice"): 1.5}),
        )


# ---------------------------------------------------------------------------
# Vulnerabilities
# ---------------------------------------------------------------------------

def test_unpatched_high_sev_warns():
    events = [
        {"eventType": "release.vulnerability-reported",
         "payload": {"cveId": "CVE-2025-1234", "severity": "HIGH"}},
    ]
    v = verify_artifact(
        "consumer", _release(), TARBALL, events=events,
        trust_path_fn=_trust({("consumer", "alice"): 0.9}),
    )
    assert v.verdict == "warn"
    assert "CVE-2025-1234" in v.unpatched_vulns


def test_patched_high_sev_accepts():
    events = [
        {"eventType": "release.vulnerability-reported",
         "payload": {"cveId": "CVE-2025-1234", "severity": "HIGH"}},
        {"eventType": "release.vulnerability-patched",
         "payload": {"cveId": "CVE-2025-1234",
                     "patchedInVersion": "2.3.2"}},
    ]
    v = verify_artifact(
        "consumer", _release(), TARBALL, events=events,
        trust_path_fn=_trust({("consumer", "alice"): 0.9}),
    )
    assert v.verdict == "accept"
    assert v.unpatched_vulns == []


def test_low_severity_vuln_does_not_warn():
    """LOW severity CVEs don't trigger the warn path by default."""
    events = [
        {"eventType": "release.vulnerability-reported",
         "payload": {"cveId": "CVE-2025-5678", "severity": "LOW"}},
    ]
    v = verify_artifact(
        "consumer", _release(), TARBALL, events=events,
        trust_path_fn=_trust({("consumer", "alice"): 0.9}),
    )
    assert v.verdict == "accept"


def test_critical_sev_warns():
    events = [
        {"eventType": "release.vulnerability-reported",
         "payload": {"cveId": "CVE-2025-9999", "severity": "CRITICAL"}},
    ]
    v = verify_artifact(
        "consumer", _release(), TARBALL, events=events,
        trust_path_fn=_trust({("consumer", "alice"): 0.9}),
    )
    assert v.verdict == "warn"


def test_unpatched_vulnerabilities_lists_all():
    events = [
        {"eventType": "release.vulnerability-reported",
         "payload": {"cveId": "CVE-1", "severity": "HIGH"}},
        {"eventType": "release.vulnerability-reported",
         "payload": {"cveId": "CVE-2", "severity": "LOW"}},
        {"eventType": "release.vulnerability-patched",
         "payload": {"cveId": "CVE-1", "patchedInVersion": "1.0.1"}},
    ]
    assert unpatched_vulnerabilities(events) == ["CVE-2"]
    assert unpatched_high_sev(events) == []


# ---------------------------------------------------------------------------
# Warn-suppression
# ---------------------------------------------------------------------------

def test_strict_mode_no_warn_on_unpatched():
    """Setting warn_on_unpatched_high_sev=False falls through to accept."""
    events = [
        {"eventType": "release.vulnerability-reported",
         "payload": {"cveId": "CVE-2025-1234", "severity": "HIGH"}},
    ]
    v = verify_artifact(
        "consumer", _release(), TARBALL, events=events,
        trust_path_fn=_trust({("consumer", "alice"): 0.9}),
        warn_on_unpatched_high_sev=False,
    )
    assert v.verdict == "accept"


# ---------------------------------------------------------------------------
# Batch
# ---------------------------------------------------------------------------

def test_batch_mixed_verdicts():
    tarball_a = b"release-A"
    tarball_b = b"release-B"
    tarball_c = b"release-C"
    rel_a = ReleaseV1("pkg-a-1.0", "pkg-a", "1.0", "alice",
                       sha256_hex(tarball_a))
    rel_b = ReleaseV1("pkg-b-1.0", "pkg-b", "1.0", "bob",
                       sha256_hex(tarball_b))
    rel_c = ReleaseV1("pkg-c-1.0", "pkg-c", "1.0", "carol",
                       sha256_hex(tarball_c))

    cve_events = [{
        "eventType": "release.vulnerability-reported",
        "payload": {"cveId": "CVE-X", "severity": "HIGH"},
    }]

    trust = _trust({
        ("consumer", "alice"): 0.9,
        ("consumer", "bob"): 0.9,
        # carol unknown -> trust 0 -> reject.
    })

    summary = verify_batch(
        "consumer",
        [
            (rel_a, tarball_a, []),
            (rel_b, tarball_b, cve_events),
            (rel_c, tarball_c, []),
        ],
        trust,
    )
    assert summary.total == 3
    assert summary.accepted == 1
    assert summary.warned == 1
    assert summary.rejected == 1
