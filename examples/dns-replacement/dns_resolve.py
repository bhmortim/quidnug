"""DNS-replacement resolver logic (standalone, no SDK dep).

Phase 0 proof of concept: DNS records as signed events on a
zone's event stream, resolved by any observer with per-observer
trust gating.

The Quidnug-side data model:
  - Each zone is a quid (e.g., `example.quidnug` is a quid).
  - The zone has a set of *governors* -- quids allowed to
    publish records. In production this is a GuardianSet on
    the zone's quid. The POC passes it in as a list.
  - Records are events on the zone's stream. Each event's
    payload carries type / name / value / TTL.
  - Revocations are `dns.record-revoked` events referencing a
    prior record by its event sequence or by a (name, type)
    tuple.

The resolver's job:
  - Given an observer quid, a zone, a (name, type) query, the
    full event stream, a trust function, and a policy, return
    the current resolved value plus a verdict.

Verdicts:
  - `ok`: records exist and the observer trusts their signer
  - `indeterminate`: records exist but trust below threshold
  - `nxdomain`: no records of that type at that name
  - `tampered`: attempted record from a non-governor
"""

from __future__ import annotations

from dataclasses import dataclass, field
from typing import Callable, Dict, List, Optional, Set, Tuple


# ---------------------------------------------------------------------------
# Record types we support in the POC
# ---------------------------------------------------------------------------

VALID_RECORD_TYPES = frozenset({
    "A", "AAAA", "MX", "TXT", "SRV", "CNAME", "TLSA",
})


@dataclass(frozen=True)
class DNSRecord:
    name: str                       # "www.example.quidnug"
    record_type: str                # "A" / "AAAA" / ...
    value: str                      # "192.0.2.1"
    ttl_seconds: int                # cache TTL hint
    signer_quid: str                # publishing governor
    sequence: int                   # event sequence on the zone's stream
    signed_at_unix: int


@dataclass
class ResolveResult:
    verdict: str                    # "ok" | "indeterminate" | "nxdomain" | "tampered"
    query_name: str
    query_type: str
    records: List[DNSRecord] = field(default_factory=list)
    observer_trust_to_signer: float = 0.0
    reasons: List[str] = field(default_factory=list)

    def short(self) -> str:
        return (
            f"{self.verdict.upper():14s} {self.query_type:4s} "
            f"{self.query_name} "
            f"({len(self.records)} record{'s' if len(self.records) != 1 else ''}, "
            f"trust={self.observer_trust_to_signer:.3f})"
        )


TrustFn = Callable[[str, str], float]


@dataclass
class ResolvePolicy:
    min_signer_trust: float = 0.5
    # If True, a record whose TTL has expired produces
    # `nxdomain` instead of returning the stale record.
    enforce_ttl: bool = False


# ---------------------------------------------------------------------------
# Stream extraction
# ---------------------------------------------------------------------------

def extract_records(events: List[dict]) -> List[DNSRecord]:
    """Extract current DNS records from a zone's event stream,
    applying revocations in sequence order."""
    all_records: List[DNSRecord] = []
    revoked_keys: Set[Tuple[str, str, int]] = set()

    # Walk in sequence order.
    ordered = sorted(events, key=lambda e: e.get("sequence") or 0)

    for ev in ordered:
        et = ev.get("eventType") or ev.get("event_type") or ""
        p = ev.get("payload") or {}
        seq = int(ev.get("sequence") or 0)
        ts = int(ev.get("timestamp") or 0)
        signer = p.get("signerQuid") or p.get("signer") or ev.get("creator") or ""

        if et == "dns.record-published":
            rt = p.get("recordType", "")
            if rt not in VALID_RECORD_TYPES:
                continue
            rec = DNSRecord(
                name=p.get("name", ""),
                record_type=rt,
                value=str(p.get("value", "")),
                ttl_seconds=int(p.get("ttl") or 300),
                signer_quid=signer,
                sequence=seq,
                signed_at_unix=int(p.get("signedAt") or ts),
            )
            all_records.append(rec)
        elif et == "dns.record-revoked":
            # Revoke by exact (name, type, sequence) tuple.
            target_seq = int(p.get("revokesSeq") or 0)
            name = p.get("name", "")
            rt = p.get("recordType", "")
            revoked_keys.add((name, rt, target_seq))

    return [r for r in all_records if (r.name, r.record_type, r.sequence) not in revoked_keys]


# ---------------------------------------------------------------------------
# Main entry point
# ---------------------------------------------------------------------------

def resolve(
    observer: str,
    governors: Set[str],
    query_name: str,
    query_type: str,
    events: List[dict],
    trust_fn: TrustFn,
    *,
    now_unix: int,
    policy: Optional[ResolvePolicy] = None,
) -> ResolveResult:
    """Pure resolver."""
    p = policy or ResolvePolicy()

    if query_type not in VALID_RECORD_TYPES:
        return ResolveResult(
            verdict="tampered",
            query_name=query_name,
            query_type=query_type,
            reasons=[f"unsupported record type: {query_type}"],
        )

    records = extract_records(events)

    # Step 1: filter to the query name and type.
    matched = [r for r in records if r.name == query_name and r.record_type == query_type]

    # Step 2: drop records published by non-governors. These are
    # cache-poisoning attempts (the authoritative governors have
    # a closed signer set published on-chain).
    tampered_attempts: List[DNSRecord] = []
    authentic: List[DNSRecord] = []
    for r in matched:
        if r.signer_quid not in governors:
            tampered_attempts.append(r)
        else:
            authentic.append(r)

    if not authentic:
        if tampered_attempts:
            return ResolveResult(
                verdict="tampered",
                query_name=query_name, query_type=query_type,
                reasons=[
                    f"{len(tampered_attempts)} non-governor signer(s) attempted "
                    f"to publish {query_type} for {query_name}; ignored"
                ],
            )
        return ResolveResult(
            verdict="nxdomain",
            query_name=query_name, query_type=query_type,
            reasons=[f"no {query_type} records for {query_name}"],
        )

    # Step 3: take only the latest per record identity (name + type
    # + signer). The resolver picks the highest-sequence record
    # for each unique (name, type), but multiple records of the
    # same type at the same name are legitimate (e.g. round-robin
    # A records). For the POC we keep all the latest-sequence
    # records from governors.
    latest_seq = max(r.sequence for r in authentic)
    # Keep all records at that sequence plus any records from a
    # newer (name, type, value) triple that haven't been revoked.
    # Simpler: return all authentic records that weren't revoked;
    # TTL-aware filter comes next.
    returned = authentic

    # Step 4: TTL filter.
    if p.enforce_ttl:
        returned = [
            r for r in returned
            if (now_unix - r.signed_at_unix) <= r.ttl_seconds
        ]
        if not returned:
            return ResolveResult(
                verdict="nxdomain",
                query_name=query_name, query_type=query_type,
                reasons=["all records TTL-expired"],
            )

    # Step 5: trust check. Compute the MAX trust the observer has
    # in any signer of the returned records. (Could also be MIN
    # for strict policy; MAX matches a "resolver takes the best
    # trust available" model.)
    signer_trusts = {r.signer_quid: trust_fn(observer, r.signer_quid) for r in returned}
    for s, t in signer_trusts.items():
        if t < 0.0 or t > 1.0:
            raise ValueError(f"trust out of range for {s}: {t}")
    max_trust = max(signer_trusts.values())

    if max_trust < p.min_signer_trust:
        return ResolveResult(
            verdict="indeterminate",
            query_name=query_name, query_type=query_type,
            records=returned,
            observer_trust_to_signer=max_trust,
            reasons=[
                f"highest signer trust {max_trust:.3f} below "
                f"threshold {p.min_signer_trust}"
            ],
        )

    return ResolveResult(
        verdict="ok",
        query_name=query_name, query_type=query_type,
        records=returned,
        observer_trust_to_signer=max_trust,
        reasons=[
            f"{len(returned)} {query_type} record(s) from governors "
            f"(max trust {max_trust:.3f})"
        ],
    )
