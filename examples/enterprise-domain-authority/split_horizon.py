"""Split-horizon record visibility logic (standalone, no SDK dep).

An enterprise runs its own Quidnug-backed DNS authority for its
domain. Three visibility tiers:

  - `public`: served to any resolver on the internet
  - `trust-gated:<gating-quid>`: served only to observers whose
    relational trust path to the gating quid meets a threshold
  - `private:<group-id>`: served only to members of the named
    group (encrypted to the group under QDP-0024)

The decision -- should the resolver return the record, or
NXDOMAIN -- is a pure function of the record's declared
visibility, the observer's trust graph, the observer's group
memberships, and a policy.

This module handles the policy decision. Actual encryption /
decryption of private records happens at the caller via
pkg/crypto/groupenc. A resolver without decrypt access sees
only the ciphertext and treats the record as NXDOMAIN.
"""

from __future__ import annotations

from dataclasses import dataclass, field
from typing import Callable, List, Optional, Set


# ---------------------------------------------------------------------------
# Domain model
# ---------------------------------------------------------------------------

VISIBILITY_PUBLIC = "public"


@dataclass(frozen=True)
class ERecord:
    """Enterprise-domain record with a declared visibility."""

    name: str
    record_type: str
    value: str
    visibility: str                 # "public" | "trust-gated:<quid>" | "private:<group>"
    signer_quid: str
    sequence: int = 0
    signed_at_unix: int = 0


@dataclass
class QueryResult:
    verdict: str                    # "ok" | "nxdomain" | "forbidden" | "invalid"
    record: Optional[ERecord] = None
    reasons: List[str] = field(default_factory=list)

    def short(self) -> str:
        if self.record:
            return (
                f"{self.verdict.upper():9s} {self.record.record_type:4s} "
                f"{self.record.name} -> {self.record.value}"
            )
        return f"{self.verdict.upper():9s} (no record)"


TrustFn = Callable[[str, str], float]
GroupMembershipFn = Callable[[str, str], bool]   # (observer, group_id) -> is_member


@dataclass
class VisibilityPolicy:
    min_trust_for_gated: float = 0.6


# ---------------------------------------------------------------------------
# Main entry point
# ---------------------------------------------------------------------------

def query_record(
    observer: str,
    record: ERecord,
    trust_fn: TrustFn,
    group_membership_fn: GroupMembershipFn,
    policy: Optional[VisibilityPolicy] = None,
) -> QueryResult:
    """Pure decision function. Returns what the resolver should
    return for this record, from this observer's perspective."""
    p = policy or VisibilityPolicy()
    vis = record.visibility

    if vis == VISIBILITY_PUBLIC:
        return QueryResult(
            verdict="ok",
            record=record,
            reasons=["visibility: public"],
        )

    if vis.startswith("trust-gated:"):
        gating = vis.split(":", 1)[1]
        if not gating:
            return QueryResult(
                verdict="invalid",
                reasons=["trust-gated visibility missing gating quid"],
            )
        t = trust_fn(observer, gating)
        if t < 0.0 or t > 1.0:
            raise ValueError(f"trust out of range for {gating}: {t}")
        if t >= p.min_trust_for_gated:
            return QueryResult(
                verdict="ok",
                record=record,
                reasons=[
                    f"visibility: trust-gated:{gating[:12]}; "
                    f"observer trust {t:.3f} >= {p.min_trust_for_gated}"
                ],
            )
        return QueryResult(
            verdict="nxdomain",
            reasons=[
                f"trust-gated: observer trust to {gating[:12]} is "
                f"{t:.3f} < threshold {p.min_trust_for_gated}"
            ],
        )

    if vis.startswith("private:"):
        group = vis.split(":", 1)[1]
        if not group:
            return QueryResult(
                verdict="invalid",
                reasons=["private visibility missing group id"],
            )
        if group_membership_fn(observer, group):
            return QueryResult(
                verdict="ok",
                record=record,
                reasons=[f"visibility: private:{group}; observer is a member"],
            )
        return QueryResult(
            verdict="nxdomain",
            reasons=[
                f"private: observer is not a member of group {group}"
            ],
        )

    return QueryResult(
        verdict="invalid",
        reasons=[f"unknown visibility scheme: {vis}"],
    )


# ---------------------------------------------------------------------------
# Batch / zone-scoped query
# ---------------------------------------------------------------------------

def query_zone(
    observer: str,
    records: List[ERecord],
    trust_fn: TrustFn,
    group_membership_fn: GroupMembershipFn,
    *,
    name_filter: Optional[str] = None,
    type_filter: Optional[str] = None,
    policy: Optional[VisibilityPolicy] = None,
) -> List[QueryResult]:
    """Run ``query_record`` over a set of records. Only returns
    the records the observer is permitted to see."""
    out: List[QueryResult] = []
    for r in records:
        if name_filter is not None and r.name != name_filter:
            continue
        if type_filter is not None and r.record_type != type_filter:
            continue
        q = query_record(observer, r, trust_fn, group_membership_fn, policy)
        if q.verdict == "ok":
            out.append(q)
    return out


# ---------------------------------------------------------------------------
# Stream extraction
# ---------------------------------------------------------------------------

def extract_records(events: List[dict]) -> List[ERecord]:
    """Extract records from a zone's event stream."""
    out: List[ERecord] = []
    for ev in events:
        et = ev.get("eventType") or ev.get("event_type") or ""
        if et != "edns.record-published":
            continue
        p = ev.get("payload") or {}
        out.append(ERecord(
            name=p.get("name", ""),
            record_type=p.get("recordType", ""),
            value=str(p.get("value", "")),
            visibility=p.get("visibility", VISIBILITY_PUBLIC),
            signer_quid=p.get("signerQuid", ""),
            sequence=int(ev.get("sequence") or 0),
            signed_at_unix=int(p.get("signedAt") or ev.get("timestamp") or 0),
        ))
    return out


def extract_group_memberships(events: List[dict]) -> dict:
    """Scan a group's administrative stream for added/removed
    members. Returns {member_quid: True} for current members."""
    current: Set[str] = set()
    ordered = sorted(events, key=lambda e: e.get("sequence") or 0)
    for ev in ordered:
        et = ev.get("eventType") or ev.get("event_type") or ""
        p = ev.get("payload") or {}
        if et == "group.member-added":
            current.add(p.get("memberQuid", ""))
        elif et == "group.member-removed":
            current.discard(p.get("memberQuid", ""))
    return {m: True for m in current if m}
