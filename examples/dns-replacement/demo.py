"""DNS-replacement, Phase 0 end-to-end demo.

Flow:
  1. Register actors: two governors for `example.quidnug`,
     the zone quid itself, a resolver consumer, and a
     rogue-quid (cache poisoning attacker).
  2. Governor-primary publishes A, AAAA, MX records on the
     zone's event stream.
  3. Resolver queries A, AAAA, MX -> ok.
  4. Rogue quid tries to publish a competing A record
     pointing at their own IP. Resolver still returns the
     governor's A record.
  5. Governor rotates: revokes the original A record and
     publishes a new one. Resolver sees the new value.
  6. Observer with weak trust in the governor runs the same
     query and gets `indeterminate` instead of `ok`.

Prerequisites:
  - Local Quidnug node at http://localhost:8080.
  - Python SDK installed.

Run:
    python demo.py
"""

from __future__ import annotations

import os
import sys
import time
import uuid
from dataclasses import dataclass
from typing import List, Set

sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))
from dns_resolve import (
    ResolvePolicy,
    resolve,
)

from quidnug import Quid, QuidnugClient

NODE_URL = os.environ.get("QUIDNUG_NODE", "http://localhost:8080")
DOMAIN = "dns.quidnug"


@dataclass
class Actor:
    name: str
    role: str
    quid: Quid


def banner(msg: str) -> None:
    print()
    print("=" * 72)
    print(f"  {msg}")
    print("=" * 72)


def register(client: QuidnugClient, name: str, role: str) -> Actor:
    q = Quid.generate()
    try:
        client.register_identity(
            q, name=name, domain=DOMAIN, home_domain=DOMAIN,
            attributes={"role": role},
        )
    except Exception as e:
        print(f"  (register {name}: {e})")
    return Actor(name=name, role=role, quid=q)


def publish_record(
    client: QuidnugClient, governor: Actor, zone: Actor,
    name: str, rtype: str, value: str, ttl: int = 300,
) -> None:
    client.emit_event(
        signer=governor.quid,
        subject_id=zone.quid.id,
        subject_type="QUID",
        event_type="dns.record-published",
        domain=DOMAIN,
        payload={
            "recordType": rtype,
            "name": name,
            "value": value,
            "ttl": ttl,
            "signerQuid": governor.quid.id,
            "signedAt": int(time.time()),
        },
    )
    print(f"  {governor.name} published {rtype} {name} -> {value}")


def revoke_record(
    client: QuidnugClient, governor: Actor, zone: Actor,
    name: str, rtype: str, revokes_seq: int,
) -> None:
    client.emit_event(
        signer=governor.quid,
        subject_id=zone.quid.id,
        subject_type="QUID",
        event_type="dns.record-revoked",
        domain=DOMAIN,
        payload={
            "recordType": rtype,
            "name": name,
            "revokesSeq": revokes_seq,
            "signerQuid": governor.quid.id,
            "revokedAt": int(time.time()),
        },
    )
    print(f"  {governor.name} revoked {rtype} {name} seq={revokes_seq}")


def load_events(client: QuidnugClient, zone: Actor) -> List[dict]:
    events, _ = client.get_stream_events(zone.quid.id, limit=500)
    out: List[dict] = []
    for ev in events or []:
        out.append({
            "eventType": ev.event_type,
            "payload": ev.payload or {},
            "timestamp": ev.timestamp,
            "sequence": ev.sequence,
            "creator": ev.creator,
        })
    return out


def node_trust_fn(client: QuidnugClient):
    def fn(obs: str, target: str) -> float:
        try:
            r = client.get_trust(obs, target, domain=DOMAIN, max_depth=5)
            return r.trust_level if r else 0.0
        except Exception:
            return 0.0
    return fn


def resolve_and_show(
    client: QuidnugClient, resolver: Actor, zone: Actor,
    governors: Set[str], query_name: str, query_type: str, label: str,
    policy: ResolvePolicy = None,
) -> None:
    events = load_events(client, zone)
    r = resolve(
        resolver.quid.id, governors, query_name, query_type,
        events, node_trust_fn(client),
        now_unix=int(time.time()), policy=policy,
    )
    print(f"\n  [{label}]")
    print(f"    {r.short()}")
    for rec in r.records:
        print(f"      {rec.record_type:5s} {rec.name:30s} -> {rec.value}")
    for reason in r.reasons:
        print(f"      - {reason}")


def main() -> None:
    print(f"Connecting to Quidnug node at {NODE_URL}")
    client = QuidnugClient(NODE_URL)
    try:
        client.info()
    except Exception as e:
        print(f"node unreachable: {e}", file=sys.stderr)
        sys.exit(1)

    # -----------------------------------------------------------------
    banner("Step 1: Register actors")
    zone       = register(client, "example.quidnug",    "zone")
    gov_p      = register(client, "gov-example-primary", "governor")
    gov_b      = register(client, "gov-example-backup",  "governor")
    rogue      = register(client, "rogue-cache-poisoner","attacker")
    resolver   = register(client, "resolver-acme",       "resolver")
    weak_obs   = register(client, "resolver-weak-trust", "resolver")
    for a in (zone, gov_p, gov_b, rogue, resolver, weak_obs):
        print(f"  {a.role:12s} {a.name:28s} -> {a.quid.id}")

    governors: Set[str] = {gov_p.quid.id, gov_b.quid.id}

    # -----------------------------------------------------------------
    banner("Step 2: Resolver declares trust in the governors")
    client.grant_trust(
        signer=resolver.quid, trustee=gov_p.quid.id, level=0.95,
        domain=DOMAIN, description="primary governor",
    )
    client.grant_trust(
        signer=resolver.quid, trustee=gov_b.quid.id, level=0.85,
        domain=DOMAIN, description="backup governor",
    )
    # Weak-trust observer only weakly trusts the primary.
    client.grant_trust(
        signer=weak_obs.quid, trustee=gov_p.quid.id, level=0.3,
        domain=DOMAIN, description="unfamiliar governor",
    )
    print(f"  resolver  -[0.95]-> gov-primary")
    print(f"  resolver  -[0.85]-> gov-backup")
    print(f"  weak_obs  -[0.30]-> gov-primary")

    time.sleep(1)

    # -----------------------------------------------------------------
    banner("Step 3: Governor publishes zone records")
    publish_record(client, gov_p, zone,
                    "example.quidnug", "A",    "192.0.2.1")
    publish_record(client, gov_p, zone,
                    "example.quidnug", "AAAA", "2001:db8::1")
    publish_record(client, gov_p, zone,
                    "example.quidnug", "MX",   "mail.example.quidnug")
    publish_record(client, gov_p, zone,
                    "www.example.quidnug", "A", "192.0.2.2")

    time.sleep(0.5)

    # -----------------------------------------------------------------
    banner("Step 4: Resolver resolves each record type")
    resolve_and_show(client, resolver, zone, governors,
                      "example.quidnug", "A",    "A @ example.quidnug")
    resolve_and_show(client, resolver, zone, governors,
                      "example.quidnug", "AAAA", "AAAA @ example.quidnug")
    resolve_and_show(client, resolver, zone, governors,
                      "example.quidnug", "MX",   "MX @ example.quidnug")
    resolve_and_show(client, resolver, zone, governors,
                      "www.example.quidnug", "A", "A @ www.example.quidnug")

    # -----------------------------------------------------------------
    banner("Step 5: Rogue quid attempts cache poisoning")
    client.emit_event(
        signer=rogue.quid,
        subject_id=zone.quid.id,
        subject_type="QUID",
        event_type="dns.record-published",
        domain=DOMAIN,
        payload={
            "recordType": "A",
            "name": "example.quidnug",
            "value": "198.51.100.99",   # attacker's IP
            "ttl": 300,
            "signerQuid": rogue.quid.id,
            "signedAt": int(time.time()),
        },
    )
    print(f"  {rogue.name} published A example.quidnug -> 198.51.100.99")
    time.sleep(0.5)
    resolve_and_show(
        client, resolver, zone, governors,
        "example.quidnug", "A",
        "A @ example.quidnug AFTER POISONING ATTEMPT",
    )

    # -----------------------------------------------------------------
    banner("Step 6: Governor rotates A record")
    # Find the original sequence for the A record we published
    # first. For the POC, we know it was seq 1 in this run's context,
    # but let's scan the stream to be correct.
    events = load_events(client, zone)
    orig_seq = None
    for ev in events:
        p = ev["payload"]
        if (ev["eventType"] == "dns.record-published"
                and p.get("recordType") == "A"
                and p.get("name") == "example.quidnug"
                and p.get("signerQuid") == gov_p.quid.id
                and p.get("value") == "192.0.2.1"):
            orig_seq = ev["sequence"]
            break
    assert orig_seq is not None, "couldn't locate original A record in stream"

    revoke_record(client, gov_p, zone, "example.quidnug", "A", orig_seq)
    publish_record(client, gov_p, zone,
                    "example.quidnug", "A", "192.0.2.100")

    time.sleep(0.5)
    resolve_and_show(
        client, resolver, zone, governors,
        "example.quidnug", "A",
        "A @ example.quidnug AFTER KEY ROTATION",
    )

    # -----------------------------------------------------------------
    banner("Step 7: Weak-trust observer queries the same zone")
    resolve_and_show(
        client, weak_obs, zone, governors,
        "example.quidnug", "A",
        "WEAK OBSERVER (expect indeterminate)",
    )

    # -----------------------------------------------------------------
    banner("Demo complete")
    print()
    print("Insights:")
    print(" - A zone's records are signed events on the zone's own stream.")
    print("   No separate recursive / authoritative server tier is needed.")
    print(" - Cache poisoning is caught: the resolver checks that the")
    print("   record's signer is in the zone's governor set. Non-governor")
    print("   records are ignored.")
    print(" - Record rotation is a revoke + publish pair. The revoked")
    print("   record is filtered out when materializing the zone.")
    print(" - Relational trust gates resolution. The same events produce")
    print("   different verdicts for different observers based on their")
    print("   trust in the governors. This is the property DNSSEC does")
    print("   not have: an observer can reject a technically-signed answer")
    print("   from a signer they no longer trust.")
    print()


if __name__ == "__main__":
    main()
