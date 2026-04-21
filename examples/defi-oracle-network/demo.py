"""DeFi oracle network, end-to-end runnable demo.

Flow:
  1. Register four reporter oracles and two DeFi consumer
     protocols (conservative + permissive).
  2. Each consumer sets up its own trust graph over the
     reporters.
  3. Reporters emit `oracle.price-report` events on a shared
     feed stream (the feed itself is a quid with reporters as
     event signers).
  4. Both consumers independently aggregate the same reports
     and reach different effective prices because their trust
     graphs differ.
  5. Outlier scenario: one reporter emits a wild price; the
     conservative policy excludes it; the permissive policy
     (no outlier rejection) includes it.

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
from typing import List

sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))
from oracle_aggregation import (
    AggregationPolicy,
    aggregate_price,
    extract_reports,
)

from quidnug import Quid, QuidnugClient

NODE_URL = os.environ.get("QUIDNUG_NODE", "http://localhost:8080")
DOMAIN = "oracles.price-feeds.ethereum"


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


def emit_price(
    client: QuidnugClient, reporter: Actor, feed: Actor,
    symbol: str, price: float,
) -> None:
    """Each reporter publishes price reports on their OWN quid
    stream (since QUID streams are writable only by the quid
    itself). The feed_id and symbol are carried in the payload;
    consumers aggregate by pulling every reporter's stream."""
    client.emit_event(
        signer=reporter.quid,
        subject_id=reporter.quid.id,
        subject_type="QUID",
        event_type="oracle.price-report",
        domain=DOMAIN,
        payload={
            "reporter": reporter.quid.id,
            "feedQuid": feed.quid.id,
            "symbol": symbol,
            "price": price,
            "timestamp": int(time.time()),
            "source": reporter.name,
            "confidence": 0.95,
        },
    )
    print(f"  {reporter.name:20s} -> {symbol} = ${price:,.2f}")


def load_reports(
    client: QuidnugClient, feed: Actor, reporters: List[Actor],
) -> List[dict]:
    """Merge price-report events from every reporter's stream,
    filtered to this feed."""
    out: List[dict] = []
    for r in reporters:
        events, _ = client.get_stream_events(r.quid.id, limit=500)
        for ev in events or []:
            if ev.event_type != "oracle.price-report":
                continue
            p = ev.payload or {}
            if p.get("feedQuid") and p.get("feedQuid") != feed.quid.id:
                continue
            out.append({
                "eventType": ev.event_type,
                "payload": p,
                "timestamp": ev.timestamp,
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


def aggregate_and_show(
    client: QuidnugClient, consumer: Actor, feed: Actor, symbol: str,
    policy: AggregationPolicy, label: str,
    reporters: List[Actor] = None,
) -> None:
    events = load_reports(client, feed, reporters or [])
    reports = extract_reports(events)
    a = aggregate_price(
        consumer.quid.id, symbol, reports, node_trust_fn(client),
        now_unix=int(time.time()), policy=policy,
    )
    print(f"\n  [{label}] consumer={consumer.name}")
    print(f"    {a.short()}")
    for r in a.reasons:
        print(f"      - {r}")
    for x in a.excluded:
        print(f"      excluded: {x}")


def main() -> None:
    print(f"Connecting to Quidnug node at {NODE_URL}")
    client = QuidnugClient(NODE_URL)
    try:
        client.info()
    except Exception as e:
        print(f"node unreachable: {e}", file=sys.stderr)
        sys.exit(1)

    client.ensure_domain(DOMAIN)

    # ---------------------------------------------------------------
    banner("Step 1: Register actors")
    feed = register(client, "btc-usd-feed",            "feed")
    r1   = register(client, "chainlink-style-oracle",  "reporter")
    r2   = register(client, "pyth-style-oracle",       "reporter")
    r3   = register(client, "api3-style-oracle",       "reporter")
    r4   = register(client, "alt-market-oracle",       "reporter")
    cons = register(client, "lending-protocol-aave",   "consumer")
    perm = register(client, "micro-dapp-stablecoin",    "consumer")
    for a in (feed, r1, r2, r3, r4, cons, perm):
        print(f"  {a.role:10s} {a.name:24s} -> {a.quid.id}")
    client.wait_for_identities([a.quid.id for a in
        (feed, r1, r2, r3, r4, cons, perm)])

    # ---------------------------------------------------------------
    banner("Step 2: Consumer trust graphs")
    # Conservative: very high trust in r1, r2, r3; low trust in r4.
    for reporter, level in [(r1, 0.95), (r2, 0.95), (r3, 0.9), (r4, 0.2)]:
        client.grant_trust(
            signer=cons.quid, trustee=reporter.quid.id, level=level,
            domain=DOMAIN, description=f"conservative trust in {reporter.name}",
        )
        print(f"  conservative -[{level}]-> {reporter.name}")
    # Permissive: broad trust in everyone; takes outliers at face value.
    for reporter, level in [(r1, 0.8), (r2, 0.8), (r3, 0.8), (r4, 0.8)]:
        client.grant_trust(
            signer=perm.quid, trustee=reporter.quid.id, level=level,
            domain=DOMAIN, description=f"permissive trust in {reporter.name}",
        )
        print(f"  permissive   -[{level}]-> {reporter.name}")

    time.sleep(1)

    # ---------------------------------------------------------------
    banner("Step 3: Reporters emit a consensus set of price reports")
    emit_price(client, r1, feed, "BTC-USD", 67400.0)
    emit_price(client, r2, feed, "BTC-USD", 67420.0)
    emit_price(client, r3, feed, "BTC-USD", 67410.0)
    emit_price(client, r4, feed, "BTC-USD", 67425.0)  # consensus

    time.sleep(3)

    banner("Step 4: Both consumers aggregate -- similar effective prices")
    default_policy = AggregationPolicy(
        window_seconds=300, min_reporters=3,
        min_reporter_trust=0.25, outlier_stddev_threshold=3.0,
    )
    aggregate_and_show(client, cons, feed, "BTC-USD", default_policy, "CONSERVATIVE", reporters=[r1, r2, r3, r4])
    aggregate_and_show(client, perm, feed, "BTC-USD", default_policy, "PERMISSIVE", reporters=[r1, r2, r3, r4])

    # ---------------------------------------------------------------
    banner("Step 5: Outlier scenario  (r4 reports a wild price)")
    # Wait so the consensus reports age out of the "latest-wins"
    # dedup for r4, or just note r4's next report is what matters.
    emit_price(client, r4, feed, "BTC-USD", 95000.0)  # wildly off
    time.sleep(3)

    print("\n  Conservative protocol: trust in r4 is 0.2 (below 0.25 floor)")
    print("  -> r4 excluded from aggregation regardless of outlier logic.")
    aggregate_and_show(client, cons, feed, "BTC-USD", default_policy,
                       "CONSERVATIVE after outlier",
                       reporters=[r1, r2, r3, r4])

    print("\n  Permissive protocol: trust in r4 is 0.8 (above floor).")
    print("  With default policy (MAD outlier rejection), r4's report gets pruned:")
    aggregate_and_show(client, perm, feed, "BTC-USD", default_policy,
                       "PERMISSIVE with outlier rejection",
                       reporters=[r1, r2, r3, r4])

    print("\n  Same permissive protocol with outlier rejection DISABLED:")
    permissive_loose = AggregationPolicy(
        window_seconds=300, min_reporters=3,
        min_reporter_trust=0.25, outlier_stddev_threshold=0.0,
    )
    aggregate_and_show(client, perm, feed, "BTC-USD", permissive_loose,
                       "PERMISSIVE NO outlier rejection",
                       reporters=[r1, r2, r3, r4])

    banner("Demo complete")
    print()
    print("Insights:")
    print(" - Each reporter's report is a signed event on the feed's stream.")
    print("   No central aggregator required.")
    print(" - Consumers compute their OWN weighted aggregate based on their")
    print("   OWN trust graph. Same source reports, different verdicts.")
    print(" - MAD (median absolute deviation) is robust against a single")
    print("   outlier even when the outlier is in the same sample.")
    print(" - A compromised reporter is handled by lowering the trust edge")
    print("   to them. Gossip propagates that change network-wide.")
    print()


if __name__ == "__main__":
    main()
