"""Enterprise domain authority, end-to-end runnable demo.

Flow:
  1. Register actors: BigCorp governor, partner org, employee,
     outsider. Plus a partners-trust-group quid and an
     employees-group quid.
  2. Governor publishes three records on the zone:
       - bigcorp.com         A  (public)
       - api.bigcorp.com     A  (trust-gated:bigcorp-partners)
       - internal.bigcorp.com A (private:bigcorp-employees)
  3. Governor grants trust to the partner org (through the
     partners quid).
  4. Governor adds the employee to the employees group via
     group.member-added events on the group's quid.
  5. Each observer (outsider, partner, employee) queries the
     zone and receives different result sets.

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
from dataclasses import dataclass
from typing import List

sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))
from split_horizon import (
    ERecord,
    VisibilityPolicy,
    extract_group_memberships,
    extract_records,
    query_zone,
)

from quidnug import Quid, QuidnugClient

NODE_URL = os.environ.get("QUIDNUG_NODE", "http://localhost:8080")
DOMAIN = "enterprise.bigcorp.com"


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
    name: str, rtype: str, value: str, visibility: str,
) -> None:
    client.emit_event(
        signer=governor.quid,
        subject_id=zone.quid.id,
        subject_type="QUID",
        event_type="edns.record-published",
        domain=DOMAIN,
        payload={
            "name": name, "recordType": rtype,
            "value": value, "visibility": visibility,
            "signerQuid": governor.quid.id,
            "signedAt": int(time.time()),
        },
    )
    print(f"  {name:30s} {rtype:4s} -> {value:20s} [{visibility}]")


def add_member(
    client: QuidnugClient, governor: Actor, group: Actor, member: Actor,
) -> None:
    client.emit_event(
        signer=governor.quid,
        subject_id=group.quid.id,
        subject_type="QUID",
        event_type="group.member-added",
        domain=DOMAIN,
        payload={"memberQuid": member.quid.id,
                  "addedBy": governor.quid.id,
                  "addedAt": int(time.time())},
    )
    print(f"  {member.name} added to group {group.name}")


def load_events(client: QuidnugClient, subject: Actor) -> List[dict]:
    events, _ = client.get_stream_events(subject.quid.id, limit=200)
    out: List[dict] = []
    for ev in events or []:
        out.append({
            "eventType": ev.event_type,
            "payload": ev.payload or {},
            "timestamp": ev.timestamp,
            "sequence": ev.sequence,
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


def run_query(
    client: QuidnugClient, observer: Actor, zone: Actor,
    employees_group: Actor, label: str,
) -> None:
    zone_events = load_events(client, zone)
    group_events = load_events(client, employees_group)
    records = extract_records(zone_events)
    memberships = extract_group_memberships(group_events)

    def groups_fn(obs: str, group_label: str) -> bool:
        # Our POC uses the group's quid id as the visibility suffix.
        # Resolve the group_label to the actual group quid.
        # For the demo we match against employees_group by name.
        if group_label == employees_group.quid.id:
            return obs in memberships
        return False

    results = query_zone(
        observer.quid.id, records,
        node_trust_fn(client), groups_fn,
        policy=VisibilityPolicy(min_trust_for_gated=0.5),
    )
    print(f"\n  [{label}] observer={observer.name}")
    if not results:
        print(f"    (no visible records)")
    for r in results:
        print(f"    {r.short()}")


def main() -> None:
    print(f"Connecting to Quidnug node at {NODE_URL}")
    client = QuidnugClient(NODE_URL)
    try:
        client.info()
    except Exception as e:
        print(f"node unreachable: {e}", file=sys.stderr)
        sys.exit(1)

    banner("Step 1: Register actors")
    gov         = register(client, "bigcorp-gov",         "governor")
    zone        = register(client, "bigcorp.com-zone",    "zone")
    partners_g  = register(client, "bigcorp-partners",    "trust-gating-quid")
    employees_g = register(client, "bigcorp-employees",   "group")
    partner     = register(client, "vendor-xyz-corp",     "partner")
    employee    = register(client, "employee-alice",      "employee")
    outsider    = register(client, "random-visitor",      "outsider")
    for a in (gov, zone, partners_g, employees_g, partner, employee, outsider):
        print(f"  {a.role:18s} {a.name:24s} -> {a.quid.id}")

    banner("Step 2: Governor publishes three records with different visibility")
    publish_record(client, gov, zone, "bigcorp.com",
                    "A", "203.0.113.1", "public")
    publish_record(client, gov, zone, "api.bigcorp.com",
                    "A", "203.0.113.10",
                    f"trust-gated:{partners_g.quid.id}")
    publish_record(client, gov, zone, "internal.bigcorp.com",
                    "A", "10.0.0.5",
                    f"private:{employees_g.quid.id}")

    banner("Step 3: Trust graph for the partner org")
    client.grant_trust(
        signer=partners_g.quid, trustee=partner.quid.id, level=0.9,
        domain=DOMAIN, description="approved partner",
    )
    # Make the trust flow both directions: partner trusts the
    # partners-group at strong level, so the trust-gated check
    # walks from the partner observer to the gating quid.
    client.grant_trust(
        signer=partner.quid, trustee=partners_g.quid.id, level=0.9,
        domain=DOMAIN, description="we are a partner",
    )
    print(f"  partner <-> partners-group mutual trust 0.9")

    banner("Step 4: Employee group membership")
    add_member(client, gov, employees_g, employee)

    time.sleep(1)

    banner("Step 5: Each observer queries the zone")
    run_query(client, outsider, zone, employees_g, "OUTSIDER")
    run_query(client, partner,  zone, employees_g, "PARTNER")
    run_query(client, employee, zone, employees_g, "EMPLOYEE")

    banner("Demo complete")
    print()
    print("Insights:")
    print(" - One zone stream holds all records. Their visibility is")
    print("   declared on each event; the resolver applies it.")
    print(" - An outsider resolves only the public record. Partner adds")
    print("   access to the trust-gated API endpoint. Employee adds access")
    print("   to the private internal record.")
    print(" - No separate internal-DNS / VPN-gated-DNS / partner-portal")
    print("   stack. One system, three visibility tiers.")
    print(" - In a production deployment the 'private' tier is actually")
    print("   encrypted (QDP-0024). This POC simulates the decision via")
    print("   group membership events; the encryption layer would then be")
    print("   applied at publish / decrypt at the member side.")
    print()


if __name__ == "__main__":
    main()
