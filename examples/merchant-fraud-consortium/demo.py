"""Merchant fraud consortium — end-to-end runnable demo.

Walks through the four-party consortium flow end-to-end
against a live Quidnug node:

  1. Bootstrap four merchant quids:
       acme_retail, bigbox_corp, startup_inc, noisy_reporter
  2. Register a shared trust domain `fraud.signals.us-retail`.
  3. Establish asymmetric trust edges: acme + bigbox each
     rate the others differently (noisy_reporter gets low
     trust from both of them).
  4. Each merchant publishes a fraud signal (EVENT tx) about
     the same suspicious card fingerprint.
  5. An observer merchant queries the signal stream +
     weights each signal by their own relational trust in
     the reporter → aggregate fraud-confidence score.
  6. Demonstrate the "relational trust" property by
     computing the same aggregate from a different observer
     with a different trust graph — score differs.

Prerequisites:
  - Local Quidnug node reachable at http://localhost:8080
    (e.g., `cd deploy/compose && docker compose up -d`).
  - Python SDK installed: `pip install -e clients/python`.

Run:
    python demo.py

The demo is chatty by design so the output reads like a
tutorial.
"""

from __future__ import annotations

import os
import sys
import time
from dataclasses import dataclass
from typing import Dict, List

# Local import of the standalone weighting module.
sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))
from fraud_weighting import FraudSignal, aggregate_fraud_score

from quidnug import Quid, QuidnugClient


NODE_URL = os.environ.get("QUIDNUG_NODE", "http://localhost:8080")
DOMAIN = "fraud.signals.us-retail"


@dataclass
class Merchant:
    """A consortium member."""
    name: str
    quid: Quid
    vertical: str        # "apparel" / "big-box" / "fintech" / "unknown"
    trust_score: float = 0.0

    def __repr__(self) -> str:
        return f"Merchant({self.name}, quid={self.quid.id})"


def banner(msg: str) -> None:
    print()
    print("=" * 72)
    print(f"  {msg}")
    print("=" * 72)


def _bootstrap_merchant(
    client: QuidnugClient, name: str, vertical: str,
) -> Merchant:
    """Generate a quid + register identity in the consortium domain."""
    q = Quid.generate()
    try:
        client.register_identity(
            q,
            name=name,
            domain=DOMAIN,
            home_domain=DOMAIN,
            attributes={"orgType": "merchant", "vertical": vertical},
        )
    except Exception as e:
        # Already-exists is fine if the demo has been run before.
        print(f"  (register_identity {name}: {e})")
    return Merchant(name=name, quid=q, vertical=vertical)


def bootstrap_consortium(client: QuidnugClient) -> Dict[str, Merchant]:
    """Step 1: bring up four merchants + register them."""
    banner("Step 1: Bootstrap four merchants")
    merchants: Dict[str, Merchant] = {}
    for name, vertical in [
        ("acme-retail", "apparel"),
        ("bigbox-corp", "big-box"),
        ("startup-inc", "fintech"),
        ("noisy-reporter", "unknown"),
    ]:
        m = _bootstrap_merchant(client, name, vertical)
        merchants[name] = m
        print(f"  registered: {m}")
    # Wait for every identity to reach the committed registry
    # before any follow-on trust or event tx references them.
    client.wait_for_identities([m.quid.id for m in merchants.values()])
    return merchants


def establish_trust_graph(
    client: QuidnugClient, merchants: Dict[str, Merchant],
) -> None:
    """Step 2: each well-established merchant issues trust edges.

    Graph:
        acme-retail  ── 0.9 ──> bigbox-corp
        bigbox-corp  ── 0.9 ──> acme-retail
        acme-retail  ── 0.7 ──> startup-inc
        bigbox-corp  ── 0.6 ──> startup-inc
        acme-retail  ── 0.1 ──> noisy-reporter  (low: lots of false positives)
        bigbox-corp  ── 0.1 ──> noisy-reporter
    """
    banner("Step 2: Establish asymmetric trust edges")
    edges = [
        ("acme-retail", "bigbox-corp", 0.9, "longstanding partner"),
        ("bigbox-corp", "acme-retail", 0.9, "reciprocal"),
        ("acme-retail", "startup-inc", 0.7, "newer, decent fraud team"),
        ("bigbox-corp", "startup-inc", 0.6, "newer, decent fraud team"),
        ("acme-retail", "noisy-reporter", 0.1, "high false-positive rate"),
        ("bigbox-corp", "noisy-reporter", 0.1, "high false-positive rate"),
    ]
    for truster, trustee, level, desc in edges:
        client.grant_trust(
            signer=merchants[truster].quid,
            trustee=merchants[trustee].quid.id,
            level=level,
            domain=DOMAIN,
            description=desc,
        )
        print(f"  {truster} --[{level}]-> {trustee}  ({desc})")


def emit_fraud_signals(
    client: QuidnugClient, merchants: Dict[str, Merchant],
) -> List[FraudSignal]:
    """Step 3: each merchant emits a fraud signal about the
    same suspicious card."""
    banner("Step 3: All four merchants emit fraud signals about card-fp-A3F")
    signals_raw = [
        ("acme-retail",   0.9, "card-testing"),
        ("bigbox-corp",   0.9, "velocity-abuse"),
        ("startup-inc",   0.7, "geo-anomaly"),
        ("noisy-reporter", 0.5, "unclear"),
    ]
    now = int(time.time())
    signals_typed: List[FraudSignal] = []
    for reporter_name, severity, pattern in signals_raw:
        merchant = merchants[reporter_name]
        payload = {
            "targetCardFingerprint": "card-fp-A3F",
            "severity": severity,
            "patternType": pattern,
            "observedAt": now,
            "evidenceHash": "sha256:demo",
            "ipGeolocation": "US-CA",
            "actionTaken": "decline",
        }
        client.emit_event(
            signer=merchant.quid,
            subject_id=merchant.quid.id,
            subject_type="QUID",
            event_type="fraud.signal.card-testing",
            domain=DOMAIN,
            payload=payload,
        )
        signals_typed.append(
            FraudSignal(
                reporter_quid=merchant.quid.id,
                severity=severity,
                pattern_type=pattern,
                observed_at_unix=now,
            )
        )
        print(f"  {reporter_name:15s} severity={severity}  pattern={pattern}")
    return signals_typed


def observer_fraud_view(
    client: QuidnugClient,
    observer_merchant: Merchant,
    signals: List[FraudSignal],
    label: str,
) -> None:
    """Step 4: compute the aggregate fraud score from an
    observer's perspective."""
    banner(f"Step 4 ({label}): {observer_merchant.name} queries the signals")

    def trust_of(observer: str, reporter: str) -> float:
        """Closure capturing ``client`` — queries the node for
        relational trust between observer + reporter."""
        try:
            r = client.get_trust(
                observer, reporter, domain=DOMAIN, max_depth=5)
            return r.trust_level if r else 0.0
        except Exception:
            return 0.0

    agg = aggregate_fraud_score(
        observer=observer_merchant.quid.id,
        signals=signals,
        trust_fn=trust_of,
    )
    print()
    print(f"  AGGREGATE: {agg.summary()}")
    print()
    print(f"  {'reporter':30s}  {'severity':>8s}  {'trust':>6s}  {'weight':>6s}")
    print(f"  {'-' * 30:30s}  {'-' * 8:>8s}  {'-' * 6:>6s}  {'-' * 6:>6s}")
    for c in agg.contributions:
        print(
            f"  {c.reporter_quid:30s}  "
            f"{c.severity:>8.2f}  "
            f"{c.observer_trust:>6.2f}  "
            f"{c.weight:>6.3f}"
        )


def skeptical_observer_view(
    client: QuidnugClient,
    merchants: Dict[str, Merchant],
    signals: List[FraudSignal],
) -> None:
    """Step 5: demonstrate relational trust — a new merchant
    with a different trust graph sees a different score."""
    banner("Step 5: Skeptical newcomer with weak trust — different score")
    # Build a fresh merchant with no trust edges.
    skeptic = _bootstrap_merchant(client, "skeptic-new-entrant", "apparel")
    # The skeptic only trusts acme (just established a
    # relationship). No trust in bigbox, startup, or noisy.
    client.grant_trust(
        signer=skeptic.quid,
        trustee=merchants["acme-retail"].quid.id,
        level=0.5,
        domain=DOMAIN,
        description="new relationship; small initial trust",
    )
    observer_fraud_view(client, skeptic, signals, "SKEPTICAL OBSERVER")


def main() -> None:
    print(f"Connecting to Quidnug node at {NODE_URL}")
    client = QuidnugClient(NODE_URL)

    # Spot-check node is reachable.
    try:
        info = client.info()
        print(f"  node ok: {info}")
    except Exception as e:
        print(f"  node unreachable: {e}", file=sys.stderr)
        print("  (start one via: cd deploy/compose && docker compose up -d)", file=sys.stderr)
        sys.exit(1)

    # Register the shared trust domain (idempotent).
    client.ensure_domain(DOMAIN)

    merchants = bootstrap_consortium(client)
    establish_trust_graph(client, merchants)

    # Block interval buffer so gossip/registry catches up.
    time.sleep(1)

    signals = emit_fraud_signals(client, merchants)
    time.sleep(1)

    # Full-trust observer: acme-retail sees a clear fraud signal.
    observer_fraud_view(
        client, merchants["acme-retail"], signals, "ACME'S VIEW",
    )
    # Skeptical newcomer sees a different picture.
    skeptical_observer_view(client, merchants, signals)

    banner("Demo complete")
    print(
        "\nInsight: the same set of fraud signals produces different aggregate\n"
        "scores depending on whose trust graph you traverse. That's the\n"
        "'relational trust' property — no single global score exists or\n"
        "is needed.\n"
    )


if __name__ == "__main__":
    main()
