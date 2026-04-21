"""Healthcare consent evaluation logic (standalone, no SDK dep).

The decision every provider needs before accessing a patient's
record: is this access authorized right now? Sources:

  - A direct consent: the patient has granted access to the
    provider for a specific category, with an expiry.
  - Transitive consent via a referring provider the patient
    trusts, up to a bounded depth and a minimum composed
    trust.
  - Emergency override: the patient's guardian quorum has
    issued an emergency-access authorization.
  - Revocation: a signed event on the patient's stream that
    terminates a specific consent immediately.

The resolver is pure. Actual access enforcement (don't hand
over the record unless this returns `allow`) happens at the
caller.
"""

from __future__ import annotations

from dataclasses import dataclass, field
from typing import Callable, Dict, List, Optional, Set


# ---------------------------------------------------------------------------
# Domain model
# ---------------------------------------------------------------------------

CATEGORIES = frozenset({
    "clinical-notes", "labs", "prescriptions",
    "mental-health", "imaging", "billing",
})


@dataclass(frozen=True)
class ConsentGrant:
    patient_quid: str
    provider_quid: str
    category: str
    trust_level: float              # 0..1
    granted_at_unix: int
    valid_until_unix: int
    description: str = ""


@dataclass(frozen=True)
class Revocation:
    patient_quid: str
    provider_quid: str
    category: str               # empty means "all categories"
    revoked_at_unix: int
    reason: str = ""


@dataclass(frozen=True)
class EmergencyOverride:
    patient_quid: str
    provider_quid: str
    guardian_signatures: List[str]  # guardian quids who authorized
    valid_until_unix: int
    reason: str = ""


@dataclass(frozen=True)
class AccessRequest:
    provider_quid: str
    patient_quid: str
    category: str
    requested_at_unix: int
    purpose: str = ""


@dataclass
class AccessVerdict:
    verdict: str                    # "allow" | "deny" | "emergency-allowed"
    effective_trust: float = 0.0
    consent_path: List[str] = field(default_factory=list)
    reasons: List[str] = field(default_factory=list)

    def short(self) -> str:
        return (
            f"{self.verdict.upper():18s} "
            f"trust={self.effective_trust:.3f} "
            f"path={' -> '.join(self.consent_path) or '(direct)'}"
        )


TrustFn = Callable[[str, str], float]


@dataclass
class AccessPolicy:
    min_direct_trust: float = 0.5
    # How many hops of transitive consent we'll chase
    # (1 = direct only; 2 = one referral; 3 = referral of referral).
    max_hops: int = 3
    # Minimum composed trust after multiplication to accept
    # a transitive path.
    min_composed_trust: float = 0.5


# ---------------------------------------------------------------------------
# Stream extraction
# ---------------------------------------------------------------------------

def extract_revocations(events: List[dict]) -> List[Revocation]:
    out: List[Revocation] = []
    for ev in events:
        et = ev.get("eventType") or ev.get("event_type") or ""
        if et != "consent.revoked":
            continue
        p = ev.get("payload") or {}
        out.append(Revocation(
            patient_quid=p.get("patient", ""),
            provider_quid=p.get("provider", ""),
            category=p.get("category", ""),
            revoked_at_unix=int(p.get("revokedAt") or ev.get("timestamp") or 0),
            reason=p.get("reason", ""),
        ))
    return out


def extract_emergency_overrides(events: List[dict]) -> List[EmergencyOverride]:
    out: List[EmergencyOverride] = []
    for ev in events:
        et = ev.get("eventType") or ev.get("event_type") or ""
        if et != "consent.emergency-override":
            continue
        p = ev.get("payload") or {}
        out.append(EmergencyOverride(
            patient_quid=p.get("patient", ""),
            provider_quid=p.get("provider", ""),
            guardian_signatures=list(p.get("guardianSignatures") or []),
            valid_until_unix=int(p.get("validUntil") or 0),
            reason=p.get("reason", ""),
        ))
    return out


# ---------------------------------------------------------------------------
# Main entry point
# ---------------------------------------------------------------------------

def evaluate_access(
    request: AccessRequest,
    consents: List[ConsentGrant],
    events: List[dict],
    trust_fn: TrustFn,
    *,
    guardian_set: Optional[Set[str]] = None,
    guardian_threshold: int = 2,
    policy: Optional[AccessPolicy] = None,
    now_unix: Optional[int] = None,
) -> AccessVerdict:
    """Pure decision function.

    Order of checks:
      1. Category must be a known category.
      2. Revocations on the stream block matching consents.
      3. Emergency override takes precedence if in effect.
      4. Direct consent with sufficient trust -> allow.
      5. Transitive consent via a referring provider up to
         max_hops and min_composed_trust -> allow.
      6. Otherwise deny.
    """
    p = policy or AccessPolicy()
    now = now_unix if now_unix is not None else request.requested_at_unix
    reasons: List[str] = []

    if request.category not in CATEGORIES:
        return AccessVerdict(
            verdict="deny",
            reasons=[f"unknown category: {request.category}"],
        )

    # Step 2: revocations.
    revocations = extract_revocations(events)
    revoked_direct = any(
        r.patient_quid == request.patient_quid
        and r.provider_quid == request.provider_quid
        and (r.category == "" or r.category == request.category)
        and r.revoked_at_unix <= now
        for r in revocations
    )
    if revoked_direct:
        reasons.append("consent revoked on patient stream")

    # Step 3: emergency override.
    overrides = extract_emergency_overrides(events)
    for ov in overrides:
        if ov.patient_quid != request.patient_quid:
            continue
        if ov.provider_quid not in ("", request.provider_quid):
            continue
        if ov.valid_until_unix < now:
            continue
        if guardian_set is not None:
            valid_sigs = [g for g in ov.guardian_signatures if g in guardian_set]
        else:
            valid_sigs = list(ov.guardian_signatures)
        if len(valid_sigs) >= guardian_threshold:
            return AccessVerdict(
                verdict="emergency-allowed",
                effective_trust=1.0,
                consent_path=[f"emergency:{request.provider_quid}"],
                reasons=reasons + [
                    f"emergency override by {len(valid_sigs)} guardian(s): "
                    f"{ov.reason}"
                ],
            )

    # Step 4: direct consent.
    # Pick the most recent valid, non-revoked, non-expired grant
    # that matches (patient, provider, category).
    valid_grants = [
        g for g in consents
        if g.patient_quid == request.patient_quid
        and g.provider_quid == request.provider_quid
        and g.category == request.category
        and g.valid_until_unix >= now
    ]
    # Drop revoked.
    valid_grants = [
        g for g in valid_grants
        if not any(
            r.patient_quid == g.patient_quid
            and r.provider_quid == g.provider_quid
            and (r.category == "" or r.category == g.category)
            and r.revoked_at_unix >= g.granted_at_unix
            for r in revocations
        )
    ]
    if valid_grants:
        best = max(valid_grants, key=lambda g: g.granted_at_unix)
        if best.trust_level >= p.min_direct_trust:
            return AccessVerdict(
                verdict="allow",
                effective_trust=best.trust_level,
                consent_path=[request.provider_quid],
                reasons=reasons + [
                    f"direct consent: trust {best.trust_level:.3f} "
                    f"valid until {best.valid_until_unix}"
                ],
            )

    # Step 5: transitive consent. Walk consents where the patient
    # has granted access to some referring provider, and that
    # referring provider in turn has a trust edge to the
    # requesting provider. Cap at `max_hops` total hops.
    # For simplicity, use the trust_fn to gate transitive hops.
    transitive_verdict = _walk_transitive(
        request, consents, revocations, trust_fn, p, now,
    )
    if transitive_verdict is not None:
        return transitive_verdict

    return AccessVerdict(
        verdict="deny",
        reasons=reasons + ["no applicable consent; no transitive path"],
    )


def _walk_transitive(
    request: AccessRequest,
    consents: List[ConsentGrant],
    revocations: List[Revocation],
    trust_fn: TrustFn,
    policy: AccessPolicy,
    now_unix: int,
) -> Optional[AccessVerdict]:
    """Find a referral chain from the patient through other
    providers to the requesting provider. Returns a verdict if
    a chain exists, else None."""
    # The patient's directly-consented providers act as starting
    # points. Each provides a base trust; we multiply along the
    # chain using `trust_fn(hop_a, hop_b)`.
    consented_providers = {
        g.provider_quid: g.trust_level
        for g in consents
        if g.patient_quid == request.patient_quid
        and g.category == request.category
        and g.valid_until_unix >= now_unix
    }
    if not consented_providers:
        return None

    # Drop providers whose consent was revoked.
    for r in revocations:
        if r.patient_quid != request.patient_quid:
            continue
        if r.category not in ("", request.category):
            continue
        consented_providers.pop(r.provider_quid, None)

    # BFS up to max_hops.
    # State: (current_provider, composed_trust, path_so_far).
    from collections import deque
    start_paths = [(pq, t, [pq]) for pq, t in consented_providers.items()]
    frontier = deque(start_paths)
    # Track best-known composed trust per provider to short-circuit.
    best_trust: Dict[str, float] = dict(consented_providers)

    while frontier:
        cur, cur_trust, path = frontier.popleft()
        if cur == request.provider_quid and cur_trust >= policy.min_composed_trust:
            return AccessVerdict(
                verdict="allow",
                effective_trust=cur_trust,
                consent_path=path,
                reasons=[
                    f"transitive consent: trust {cur_trust:.3f} via "
                    f"{len(path)} hop(s)"
                ],
            )
        if len(path) >= policy.max_hops:
            continue
        # Explore: from `cur`, any provider `cur` trusts is a hop.
        # We don't enumerate the world; instead we only try the
        # request.provider_quid. That's sufficient for the POC's
        # topology (referrals are known).
        edge = trust_fn(cur, request.provider_quid)
        if 0.0 <= edge <= 1.0:
            composed = cur_trust * edge
            if composed < policy.min_composed_trust:
                continue
            next_path = path + [request.provider_quid]
            prev_best = best_trust.get(request.provider_quid, 0.0)
            if composed > prev_best:
                best_trust[request.provider_quid] = composed
                frontier.append((request.provider_quid, composed, next_path))

    return None
