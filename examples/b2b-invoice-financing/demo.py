"""B2B invoice factoring, end-to-end runnable demo.

Flow:
  1. Register actors: supplier, buyer, carrier, credit bureau,
     two competing financiers.
  2. Credit bureau asserts trust in supplier + buyer.
  3. Each financier declares trust in the credit bureau, giving
     them a transitive chain to the trading parties.
  4. Supplier issues an invoice (register_title + invoice.issued).
  5. Carrier emits carrier.shipped and carrier.delivered events.
  6. Buyer emits buyer.acknowledged.
  7. Financier F1 evaluates -> approve, emits factor.purchased.
  8. Financier F2 evaluates the SAME invoice -> reject (double-
     factoring detected via prior factor.purchased event).
  9. A second invoice is issued with a fraud signal; financier
     rejects.

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
from invoice_factor import (
    FactoringPolicy,
    InvoiceV1,
    evaluate_factoring,
)

from quidnug import OwnershipStake, Quid, QuidnugClient

NODE_URL = os.environ.get("QUIDNUG_NODE", "http://localhost:8080")
DOMAIN = "factoring.supply-chain.us"


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


def issue_invoice(
    client: QuidnugClient, supplier: Actor, buyer: Actor,
    invoice_id: str, amount_cents: int,
) -> InvoiceV1:
    client.register_title(
        signer=supplier.quid,
        asset_id=invoice_id,
        owners=[OwnershipStake(supplier.quid.id, 1.0, "supplier")],
        domain=DOMAIN,
        title_type="invoice",
    )
    issued_at = int(time.time())
    due_at = issued_at + 60 * 86400
    client.emit_event(
        signer=supplier.quid,
        subject_id=invoice_id,
        subject_type="TITLE",
        event_type="invoice.issued",
        domain=DOMAIN,
        payload={
            "supplierQuid": supplier.quid.id,
            "buyerQuid": buyer.quid.id,
            "amountCents": amount_cents,
            "currency": "USD",
            "issuedAt": issued_at,
            "dueAt": due_at,
            "terms": "NET-60",
        },
    )
    print(f"  {supplier.name} issued {invoice_id} to {buyer.name}")
    print(f"    amount=${amount_cents/100:.2f}  due in 60 days")
    return InvoiceV1(
        invoice_id=invoice_id,
        supplier_quid=supplier.quid.id,
        buyer_quid=buyer.quid.id,
        amount_cents=amount_cents,
        currency="USD",
        issued_at_unix=issued_at,
        due_date_unix=due_at,
    )


def emit(
    client: QuidnugClient, signer: Actor, invoice_id: str,
    event_type: str, payload: dict,
) -> None:
    client.emit_event(
        signer=signer.quid,
        subject_id=invoice_id,
        subject_type="TITLE",
        event_type=event_type,
        domain=DOMAIN,
        payload=payload,
    )


def load_events(client: QuidnugClient, invoice_id: str) -> List[dict]:
    events, _ = client.get_stream_events(invoice_id, limit=200)
    out: List[dict] = []
    for ev in events or []:
        out.append({
            "eventType": ev.event_type,
            "payload": ev.payload or {},
            "timestamp": ev.timestamp,
        })
    return out


def node_trust_fn(client: QuidnugClient):
    def fn(observer: str, target: str) -> float:
        try:
            r = client.get_trust(observer, target, domain=DOMAIN, max_depth=5)
            return r.trust_level if r else 0.0
        except Exception:
            return 0.0
    return fn


def evaluate_and_show(
    client: QuidnugClient, financier: Actor, invoice: InvoiceV1,
    policy: FactoringPolicy, label: str,
) -> str:
    events = load_events(client, invoice.invoice_id)
    d = evaluate_factoring(
        financier.quid.id, invoice, events,
        node_trust_fn(client), policy,
    )
    print(f"\n  [{label}] {invoice.describe()}")
    print(f"    {d.short()}")
    for r in d.reasons:
        print(f"      - {r}")
    if d.fraud_flags:
        print(f"      FRAUD FLAGS: {', '.join(d.fraud_flags)}")
    return d.verdict


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
    supplier    = register(client, "supplier-acme-widgets", "supplier")
    buyer       = register(client, "buyer-megacorp-usa",    "buyer")
    carrier     = register(client, "carrier-global-freight","carrier")
    bureau      = register(client, "credit-bureau-dnb",     "credit-bureau")
    financier_f = register(client, "financier-prime-cap",   "financier")
    financier_g = register(client, "financier-second-cap",  "financier")
    for a in (supplier, buyer, carrier, bureau, financier_f, financier_g):
        print(f"  {a.role:14s} {a.name:26s} -> {a.quid.id}")

    # -----------------------------------------------------------------
    banner("Step 2: Credit bureau asserts trust in trading parties")
    client.grant_trust(
        signer=bureau.quid, trustee=supplier.quid.id, level=0.85,
        domain=DOMAIN, description="verified; 7 yr good payment history",
    )
    client.grant_trust(
        signer=bureau.quid, trustee=buyer.quid.id, level=0.90,
        domain=DOMAIN, description="A+ rated; public company",
    )
    print(f"  bureau -[0.85]-> supplier  (good payment history)")
    print(f"  bureau -[0.90]-> buyer     (A+ rated)")

    # -----------------------------------------------------------------
    banner("Step 3: Financiers declare trust in the bureau")
    client.grant_trust(
        signer=financier_f.quid, trustee=bureau.quid.id, level=0.9,
        domain=DOMAIN, description="primary credit-data source",
    )
    client.grant_trust(
        signer=financier_g.quid, trustee=bureau.quid.id, level=0.85,
        domain=DOMAIN, description="secondary credit-data source",
    )
    print(f"  financier-F -[0.9]-> bureau")
    print(f"  financier-G -[0.85]-> bureau")

    time.sleep(1)

    # -----------------------------------------------------------------
    banner("Step 4: Supplier issues an invoice")
    invoice_id = f"inv-{uuid.uuid4().hex[:8]}"
    invoice = issue_invoice(client, supplier, buyer, invoice_id, 50_000_00)

    # -----------------------------------------------------------------
    banner("Step 5: Financier F evaluates PRE-DELIVERY (expect pending)")
    policy = FactoringPolicy(
        min_supplier_trust=0.4, min_buyer_trust=0.4,
    )
    evaluate_and_show(client, financier_f, invoice, policy, "F @ pre-delivery")

    # -----------------------------------------------------------------
    banner("Step 6: Carrier ships + delivers")
    emit(client, carrier, invoice_id, "carrier.shipped", {
        "carrierQuid": carrier.quid.id,
        "bol": f"BOL-{uuid.uuid4().hex[:6]}",
        "shipDate": int(time.time()),
    })
    emit(client, carrier, invoice_id, "carrier.delivered", {
        "carrierQuid": carrier.quid.id,
        "deliveryProof": f"POD-{uuid.uuid4().hex[:6]}",
        "deliveredAt": int(time.time()),
    })
    print(f"  {carrier.name} emitted shipped + delivered")

    time.sleep(0.5)

    # -----------------------------------------------------------------
    banner("Step 7: Financier F evaluates POST-DELIVERY (still pending, no ack)")
    evaluate_and_show(client, financier_f, invoice, policy, "F @ post-delivery")

    # -----------------------------------------------------------------
    banner("Step 8: Buyer acknowledges the invoice")
    emit(client, buyer, invoice_id, "buyer.acknowledged", {
        "acknowledgedAmount": 50_000_00,
        "expectedPayDate": invoice.due_date_unix,
    })
    print(f"  {buyer.name} acknowledged")

    time.sleep(0.5)

    # -----------------------------------------------------------------
    banner("Step 9: Financier F evaluates FULL (expect approve)")
    verdict_f = evaluate_and_show(client, financier_f, invoice, policy, "F @ full")

    if verdict_f == "approve":
        banner("Step 10: Financier F commits factor.purchased")
        emit(client, financier_f, invoice_id, "factor.purchased", {
            "financier": financier_f.quid.id,
            "purchasePrice": int(invoice.amount_cents * 0.97),
            "discount": 0.03,
            "factoredAt": int(time.time()),
        })
        print(f"  {financier_f.name} factored at 3% discount")

    time.sleep(0.5)

    # -----------------------------------------------------------------
    banner("Step 11: Financier G tries the SAME invoice (expect reject)")
    evaluate_and_show(client, financier_g, invoice, policy, "G @ double-factor")

    # -----------------------------------------------------------------
    banner("Step 12: Second invoice with a fraud signal")
    invoice2_id = f"inv-{uuid.uuid4().hex[:8]}"
    invoice2 = issue_invoice(client, supplier, buyer, invoice2_id, 80_000_00)
    emit(client, carrier, invoice2_id, "carrier.shipped", {
        "carrierQuid": carrier.quid.id, "bol": "BOL-x", "shipDate": int(time.time()),
    })
    emit(client, carrier, invoice2_id, "carrier.delivered", {
        "carrierQuid": carrier.quid.id, "deliveryProof": "POD-x",
        "deliveredAt": int(time.time()),
    })
    emit(client, buyer, invoice2_id, "buyer.acknowledged", {
        "acknowledgedAmount": 80_000_00, "expectedPayDate": invoice2.due_date_unix,
    })
    # A third-party financier who caught fraud elsewhere emits a signal.
    emit(client, financier_g, invoice2_id, "fraud.signal.invoice-forged", {
        "evidence": "PO number matches another invoice for different buyer",
        "reporter": financier_g.quid.id,
    })
    print(f"  {financier_g.name} emitted fraud.signal.invoice-forged")

    time.sleep(0.5)
    evaluate_and_show(client, financier_f, invoice2, policy, "F @ fraud-flagged")

    # -----------------------------------------------------------------
    banner("Demo complete")
    print()
    print("Insights:")
    print(" - The invoice is a TITLE; every lifecycle event is signed by")
    print("   the appropriate party and lives on the title's stream.")
    print(" - Any financier can reconstruct the full risk picture by")
    print("   reading the stream. No platform walled gardens.")
    print(" - Double-factoring is blocked by a simple stream check:")
    print("   any prior factor.purchased event means someone already")
    print("   bought it.")
    print(" - A fraud signal from any credible party (financier-G in the")
    print("   demo) is visible to every other financier. The merchant-")
    print("   fraud-consortium pattern applies directly here.")
    print(" - Trust is relational: financiers assign their own weights")
    print("   to credit bureaus, rating agencies, and suppliers.")
    print()


if __name__ == "__main__":
    main()
