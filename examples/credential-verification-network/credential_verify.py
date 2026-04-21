"""Credential-verification logic — standalone (no SDK dep).

The core question: given a signed credential (e.g., a
university degree), an observer (employer) who has NEVER
heard of the issuing university, can they decide to trust
the credential based on transitive trust through an
accreditor they recognize?

Inputs:
    - A credential: (issuer_quid, subject_quid, type, grade, issued_at).
    - A signed revocation status (if any).
    - A ``trust_path(observer, issuer)`` function returning a
      transitive-trust score on [0, 1] computed over the
      trust graph.

Output:
    - Verification verdict: accept / reject / indeterminate.
    - Per-step breakdown explaining the decision.

This module encapsulates the semantic layer on top of the
raw Quidnug primitives: credentials are modeled as events on
the issuer's credential stream; revocations as subsequent
events with a specific event type; the accept/reject logic
is the employer-side policy.
"""

from __future__ import annotations

from dataclasses import dataclass
from typing import Callable, List, Optional


@dataclass(frozen=True)
class CredentialV1:
    """A signed credential (e.g., university degree). v1 shape."""

    credential_id: str
    issuer_quid: str
    subject_quid: str
    credential_type: str
    grade: str = ""
    issued_at_unix: int = 0

    def describe(self) -> str:
        return (
            f"credential={self.credential_id} "
            f"type={self.credential_type} "
            f"issuer={self.issuer_quid[:12]} "
            f"subject={self.subject_quid[:12]} "
            f"grade={self.grade or '-'}"
        )


@dataclass
class VerificationVerdict:
    """Output of a verify call."""

    verdict: str             # "accept" | "reject" | "indeterminate"
    score: float             # [0, 1] confidence
    reasons: List[str]       # human-readable explanation
    trust_path_score: float  # transitive trust the observer assigns


TrustPathFn = Callable[[str, str], float]
RevocationFn = Callable[[str], Optional[str]]   # credential_id -> revocation reason or None


def verify_credential(
    observer: str,
    credential: CredentialV1,
    trust_path_fn: TrustPathFn,
    revocation_fn: RevocationFn,
    *,
    min_accept_score: float = 0.6,
) -> VerificationVerdict:
    """Verify a single credential from an observer's perspective.

    Steps:
      1. Check revocation. If revoked → reject.
      2. Compute observer → issuer transitive trust.
      3. If trust >= min_accept_score → accept.
         If 0 < trust < min_accept_score → indeterminate.
         If trust == 0 → reject.
    """
    reasons: List[str] = []

    # Step 1: revocation.
    rev_reason = revocation_fn(credential.credential_id)
    if rev_reason:
        return VerificationVerdict(
            verdict="reject",
            score=0.0,
            reasons=[f"credential revoked: {rev_reason}"],
            trust_path_score=0.0,
        )
    reasons.append("credential not revoked")

    # Step 2: trust-path score.
    trust = trust_path_fn(observer, credential.issuer_quid)
    if trust < 0.0 or trust > 1.0:
        raise ValueError(f"trust path score out of range: {trust}")
    reasons.append(
        f"transitive trust {observer[:12]} -> issuer {credential.issuer_quid[:12]} = {trust:.3f}"
    )

    # Step 3: policy.
    if trust == 0.0:
        return VerificationVerdict(
            verdict="reject",
            score=0.0,
            reasons=reasons + ["no trust path to issuer"],
            trust_path_score=0.0,
        )
    if trust >= min_accept_score:
        return VerificationVerdict(
            verdict="accept",
            score=trust,
            reasons=reasons + [
                f"accept: trust {trust:.3f} >= threshold {min_accept_score}"
            ],
            trust_path_score=trust,
        )
    return VerificationVerdict(
        verdict="indeterminate",
        score=trust,
        reasons=reasons + [
            f"indeterminate: trust {trust:.3f} below acceptance threshold "
            f"but non-zero (manual review warranted)"
        ],
        trust_path_score=trust,
    )


@dataclass
class VerificationSummary:
    """Aggregate result for a batch of credentials (e.g., an
    applicant presenting multiple degrees / certifications)."""

    total: int
    accepted: int
    rejected: int
    indeterminate: int
    verdicts: List[VerificationVerdict]

    def short(self) -> str:
        return (
            f"{self.accepted}/{self.total} accepted "
            f"(rejected={self.rejected}, indeterminate={self.indeterminate})"
        )


def verify_batch(
    observer: str,
    credentials: List[CredentialV1],
    trust_path_fn: TrustPathFn,
    revocation_fn: RevocationFn,
    *,
    min_accept_score: float = 0.6,
) -> VerificationSummary:
    """Verify a list of credentials. Each one independently."""
    results = [
        verify_credential(
            observer, c, trust_path_fn, revocation_fn,
            min_accept_score=min_accept_score,
        )
        for c in credentials
    ]
    return VerificationSummary(
        total=len(results),
        accepted=sum(1 for r in results if r.verdict == "accept"),
        rejected=sum(1 for r in results if r.verdict == "reject"),
        indeterminate=sum(1 for r in results if r.verdict == "indeterminate"),
        verdicts=results,
    )
