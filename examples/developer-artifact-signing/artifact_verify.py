"""Developer-artifact verification logic (standalone, no SDK dep).

The consumer's question: given a tarball (or wheel, or jar, or any
build artifact), an advertised release on the Quidnug network, and
the consumer's own trust graph, should the artifact be accepted?

Inputs:
  - ReleaseV1: the on-chain metadata for the release (package,
    version, expected artifact hash, maintainer).
  - The actual bytes of the artifact the consumer has.
  - The release's event stream (lifecycle events: published,
    vulnerability-reported, patched, revoked).
  - A trust_path(observer, maintainer) -> float function.

Output:
  - ArtifactVerdict: accept | reject | warn, with reasons.

Policy:
  - Hash mismatch -> reject. No room for negotiation.
  - Revoked release -> reject.
  - Unpatched high-sev CVE -> warn (not reject; consumers may
    accept known vulnerabilities with mitigations).
  - Maintainer trust below threshold -> reject.
  - Otherwise -> accept.
"""

from __future__ import annotations

import hashlib
from dataclasses import dataclass, field
from typing import Callable, List, Optional, Tuple


# ---------------------------------------------------------------------------
# Domain model
# ---------------------------------------------------------------------------

@dataclass(frozen=True)
class ReleaseV1:
    """On-chain metadata for a single software release."""

    release_id: str              # "webapp-js-2.3.1"
    package_name: str
    version: str
    maintainer_quid: str
    artifact_hash_hex: str       # sha256 of the tarball, hex, no prefix
    repository: str = ""
    commit_hash: str = ""
    published_at_unix: int = 0

    def describe(self) -> str:
        return (
            f"{self.package_name}@{self.version} "
            f"(maintainer={self.maintainer_quid[:12]}, "
            f"commit={self.commit_hash[:8] if self.commit_hash else '-'})"
        )


@dataclass
class ArtifactVerdict:
    """Output of a verify call."""

    verdict: str                  # "accept" | "reject" | "warn"
    release_id: str
    reasons: List[str] = field(default_factory=list)
    maintainer_trust: float = 0.0
    unpatched_vulns: List[str] = field(default_factory=list)

    def short(self) -> str:
        return (
            f"{self.verdict.upper():7s} {self.release_id} "
            f"(trust={self.maintainer_trust:.2f}, "
            f"vulns={len(self.unpatched_vulns)})"
        )


TrustPathFn = Callable[[str, str], float]


# ---------------------------------------------------------------------------
# Helpers: extract status from a release's event stream
# ---------------------------------------------------------------------------

def sha256_hex(data: bytes) -> str:
    return hashlib.sha256(data).hexdigest()


def is_revoked(events: List[dict]) -> Optional[str]:
    """Return the revocation reason if any, else None."""
    for ev in events:
        if (ev.get("eventType") or ev.get("event_type")) == "release.revoked":
            return (ev.get("payload") or {}).get("reason", "revoked")
    return None


def unpatched_vulnerabilities(events: List[dict]) -> List[str]:
    """Return the list of CVE IDs reported against this release
    that have *not* been patched in the same stream."""
    reported: List[Tuple[str, str]] = []  # (cveId, severity)
    patched: set = set()
    for ev in events:
        et = ev.get("eventType") or ev.get("event_type") or ""
        p = ev.get("payload") or {}
        if et == "release.vulnerability-reported":
            cve = p.get("cveId", "")
            sev = (p.get("severity") or "").upper()
            if cve:
                reported.append((cve, sev))
        elif et == "release.vulnerability-patched":
            cve = p.get("cveId", "")
            if cve:
                patched.add(cve)
    return [cve for (cve, _sev) in reported if cve not in patched]


def unpatched_high_sev(events: List[dict]) -> List[str]:
    """Same as ``unpatched_vulnerabilities`` but only HIGH / CRITICAL."""
    severity_by_cve = {}
    for ev in events:
        et = ev.get("eventType") or ev.get("event_type") or ""
        p = ev.get("payload") or {}
        if et == "release.vulnerability-reported":
            cve = p.get("cveId", "")
            if cve:
                severity_by_cve[cve] = (p.get("severity") or "").upper()
    out = []
    for cve in unpatched_vulnerabilities(events):
        sev = severity_by_cve.get(cve, "")
        if sev in ("HIGH", "CRITICAL"):
            out.append(cve)
    return out


# ---------------------------------------------------------------------------
# Main entry point
# ---------------------------------------------------------------------------

def verify_artifact(
    observer: str,
    release: ReleaseV1,
    artifact_bytes: bytes,
    events: List[dict],
    trust_path_fn: TrustPathFn,
    *,
    min_trust: float = 0.5,
    warn_on_unpatched_high_sev: bool = True,
) -> ArtifactVerdict:
    """Verify a single artifact from an observer's perspective.

    Semantics documented at the top of this module.
    """
    reasons: List[str] = []

    # Step 1: hash match. Non-negotiable.
    actual_hash = sha256_hex(artifact_bytes)
    if actual_hash != release.artifact_hash_hex.lower():
        return ArtifactVerdict(
            verdict="reject",
            release_id=release.release_id,
            reasons=[
                f"artifact hash mismatch: expected {release.artifact_hash_hex[:16]}..., "
                f"got {actual_hash[:16]}..."
            ],
        )
    reasons.append(f"artifact hash matches ({actual_hash[:16]}...)")

    # Step 2: revocation.
    rev = is_revoked(events)
    if rev:
        return ArtifactVerdict(
            verdict="reject",
            release_id=release.release_id,
            reasons=reasons + [f"release revoked: {rev}"],
        )
    reasons.append("release not revoked")

    # Step 3: maintainer trust.
    trust = trust_path_fn(observer, release.maintainer_quid)
    if trust < 0.0 or trust > 1.0:
        raise ValueError(f"trust path score out of range: {trust}")
    reasons.append(f"maintainer trust = {trust:.3f}")
    if trust < min_trust:
        return ArtifactVerdict(
            verdict="reject",
            release_id=release.release_id,
            reasons=reasons + [
                f"maintainer trust {trust:.3f} below threshold {min_trust}"
            ],
            maintainer_trust=trust,
        )

    # Step 4: unpatched high-sev vulnerabilities.
    high_sev = unpatched_high_sev(events)
    if high_sev and warn_on_unpatched_high_sev:
        return ArtifactVerdict(
            verdict="warn",
            release_id=release.release_id,
            reasons=reasons + [
                f"warn: unpatched HIGH/CRITICAL CVEs: {', '.join(high_sev)}"
            ],
            maintainer_trust=trust,
            unpatched_vulns=high_sev,
        )

    reasons.append("no unpatched high-severity vulnerabilities")
    return ArtifactVerdict(
        verdict="accept",
        release_id=release.release_id,
        reasons=reasons,
        maintainer_trust=trust,
    )


# ---------------------------------------------------------------------------
# Batch
# ---------------------------------------------------------------------------

@dataclass
class BatchVerificationSummary:
    total: int
    accepted: int
    warned: int
    rejected: int
    verdicts: List[ArtifactVerdict]

    def short(self) -> str:
        return (
            f"{self.accepted}/{self.total} accepted "
            f"(warn={self.warned}, reject={self.rejected})"
        )


def verify_batch(
    observer: str,
    items: List[Tuple[ReleaseV1, bytes, List[dict]]],
    trust_path_fn: TrustPathFn,
    *,
    min_trust: float = 0.5,
) -> BatchVerificationSummary:
    """Verify a set of (release, artifact_bytes, events) triples.

    Useful for dependency-tree verification: pull a lock file,
    fetch each referenced release's title + events + tarball, run
    this over the lot."""
    verdicts = [
        verify_artifact(
            observer, r, b, ev, trust_path_fn, min_trust=min_trust,
        )
        for (r, b, ev) in items
    ]
    return BatchVerificationSummary(
        total=len(verdicts),
        accepted=sum(1 for v in verdicts if v.verdict == "accept"),
        warned=sum(1 for v in verdicts if v.verdict == "warn"),
        rejected=sum(1 for v in verdicts if v.verdict == "reject"),
        verdicts=verdicts,
    )
