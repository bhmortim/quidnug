"""Invoice factoring decision logic (standalone, no SDK dep).

A financier considering buying an invoice at a discount must
decide whether the risk is acceptable and at what discount. The
decision flows from a handful of signals:

  1. Is the invoice already factored? (Double-factoring veto.)
  2. Has the carrier confirmed delivery?
  3. Has the buyer acknowledged the invoice?
  4. What is the financier's trust in the supplier and buyer?
  5. Are there outstanding fraud signals against the supplier?

Given those, the financier either:
  - rejects (due to fraud signals, double-factoring, or
    fundamental trust gap),
  - waits (missing delivery or ack events),
  - or approves with a computed discount.

This module is pure policy. It takes a snapshot (invoice, events,
trust scores, fraud signals) and returns a FactoringDecision.
"""

from __future__ import annotations

from dataclasses import dataclass, field
from typing import Callable, Dict, List, Optional


# ---------------------------------------------------------------------------
# Domain model
# ---------------------------------------------------------------------------

@dataclass(frozen=True)
class InvoiceV1:
    """On-chain invoice metadata."""

    invoice_id: str
    supplier_quid: str
    buyer_quid: str
    amount_cents: int
    currency: str
    issued_at_unix: int
    due_date_unix: int
    po_reference: str = ""

    def describe(self) -> str:
        return (
            f"invoice={self.invoice_id} "
            f"supplier={self.supplier_quid[:12]} "
            f"buyer={self.buyer_quid[:12]} "
            f"amt=${self.amount_cents/100:.2f} {self.currency} "
            f"net-{(self.due_date_unix - self.issued_at_unix) // 86400}d"
        )


@dataclass
class FactoringDecision:
    verdict: str                # "approve" | "reject" | "pending"
    invoice_id: str
    discount_fraction: float = 0.0
    offer_price_cents: int = 0
    supplier_trust: float = 0.0
    buyer_trust: float = 0.0
    reasons: List[str] = field(default_factory=list)
    fraud_flags: List[str] = field(default_factory=list)

    def short(self) -> str:
        if self.verdict == "approve":
            return (
                f"APPROVE invoice={self.invoice_id} "
                f"discount={self.discount_fraction*100:.2f}% "
                f"offer=${self.offer_price_cents/100:.2f}"
            )
        return f"{self.verdict.upper():8s} invoice={self.invoice_id}"


TrustFn = Callable[[str, str], float]


# ---------------------------------------------------------------------------
# Event-stream feature extraction
# ---------------------------------------------------------------------------

@dataclass
class InvoiceFeatures:
    has_ship_event: bool = False
    has_delivery_event: bool = False
    has_buyer_ack: bool = False
    prior_factor_count: int = 0
    carrier_quid: str = ""
    fraud_signals: List[Dict[str, str]] = field(default_factory=list)


def extract_features(events: List[dict]) -> InvoiceFeatures:
    out = InvoiceFeatures()
    for ev in events:
        et = ev.get("eventType") or ev.get("event_type") or ""
        p = ev.get("payload") or {}
        if et == "carrier.shipped":
            out.has_ship_event = True
            out.carrier_quid = p.get("carrierQuid", out.carrier_quid)
        elif et == "carrier.delivered":
            out.has_delivery_event = True
            out.carrier_quid = p.get("carrierQuid", out.carrier_quid)
        elif et == "buyer.acknowledged":
            out.has_buyer_ack = True
        elif et == "factor.purchased":
            out.prior_factor_count += 1
        elif et == "fraud.signal.invoice-forged":
            out.fraud_signals.append({
                "type": "invoice-forged",
                "evidence": p.get("evidence", ""),
            })
    return out


# ---------------------------------------------------------------------------
# Policy
# ---------------------------------------------------------------------------

@dataclass
class FactoringPolicy:
    """Knobs the financier tunes per-deal."""

    min_supplier_trust: float = 0.5
    min_buyer_trust: float = 0.6
    require_delivery: bool = True
    require_buyer_ack: bool = True
    # Base discount applied before risk adjustments.
    base_discount_fraction: float = 0.01
    # Max additional discount added for low-trust parties.
    max_risk_premium_fraction: float = 0.10
    # How much buyer vs supplier contribute to the blended risk.
    buyer_weight: float = 0.7
    supplier_weight: float = 0.3


def _validate_policy(p: FactoringPolicy) -> None:
    if abs(p.buyer_weight + p.supplier_weight - 1.0) > 1e-6:
        raise ValueError(
            f"buyer_weight + supplier_weight must sum to 1.0 "
            f"(got {p.buyer_weight} + {p.supplier_weight})"
        )


# ---------------------------------------------------------------------------
# Main entry point
# ---------------------------------------------------------------------------

def evaluate_factoring(
    financier: str,
    invoice: InvoiceV1,
    events: List[dict],
    trust_fn: TrustFn,
    policy: Optional[FactoringPolicy] = None,
) -> FactoringDecision:
    """Pure decision function."""
    p = policy or FactoringPolicy()
    _validate_policy(p)

    features = extract_features(events)
    reasons: List[str] = []
    fraud_flags: List[str] = []

    # Step 1: double-factoring veto.
    if features.prior_factor_count >= 1:
        return FactoringDecision(
            verdict="reject",
            invoice_id=invoice.invoice_id,
            reasons=[
                f"double-factoring: {features.prior_factor_count} "
                f"prior factor.purchased event(s) on this invoice"
            ],
            fraud_flags=["double-factoring"],
        )

    # Step 2: fraud signals against supplier.
    if features.fraud_signals:
        return FactoringDecision(
            verdict="reject",
            invoice_id=invoice.invoice_id,
            reasons=["outstanding fraud signals on this invoice"],
            fraud_flags=[fs["type"] for fs in features.fraud_signals],
        )

    # Step 3: missing lifecycle events.
    if p.require_delivery and not features.has_delivery_event:
        return FactoringDecision(
            verdict="pending",
            invoice_id=invoice.invoice_id,
            reasons=["no carrier.delivered event yet"],
        )
    if p.require_buyer_ack and not features.has_buyer_ack:
        return FactoringDecision(
            verdict="pending",
            invoice_id=invoice.invoice_id,
            reasons=["no buyer.acknowledged event yet"],
        )

    # Step 4: trust.
    supplier_trust = trust_fn(financier, invoice.supplier_quid)
    buyer_trust = trust_fn(financier, invoice.buyer_quid)
    for name, t in (("supplier", supplier_trust), ("buyer", buyer_trust)):
        if t < 0.0 or t > 1.0:
            raise ValueError(f"{name} trust out of range: {t}")

    reasons.append(f"delivery confirmed")
    reasons.append(f"buyer acknowledged")
    reasons.append(f"supplier trust = {supplier_trust:.3f}")
    reasons.append(f"buyer trust = {buyer_trust:.3f}")

    if supplier_trust < p.min_supplier_trust:
        return FactoringDecision(
            verdict="reject",
            invoice_id=invoice.invoice_id,
            supplier_trust=supplier_trust,
            buyer_trust=buyer_trust,
            reasons=reasons + [
                f"supplier trust {supplier_trust:.3f} below "
                f"threshold {p.min_supplier_trust}"
            ],
        )
    if buyer_trust < p.min_buyer_trust:
        return FactoringDecision(
            verdict="reject",
            invoice_id=invoice.invoice_id,
            supplier_trust=supplier_trust,
            buyer_trust=buyer_trust,
            reasons=reasons + [
                f"buyer trust {buyer_trust:.3f} below "
                f"threshold {p.min_buyer_trust}"
            ],
        )

    # Step 5: compute discount.
    # Blended risk in [0, 1]. 0 risk = max trust.
    risk = 1.0 - (
        p.buyer_weight * buyer_trust + p.supplier_weight * supplier_trust
    )
    discount = p.base_discount_fraction + p.max_risk_premium_fraction * risk
    discount = min(discount, p.base_discount_fraction + p.max_risk_premium_fraction)
    offer = int(invoice.amount_cents * (1.0 - discount))

    reasons.append(
        f"blended risk = {risk:.3f}; "
        f"discount = {discount*100:.2f}%; "
        f"offer = ${offer/100:.2f}"
    )

    return FactoringDecision(
        verdict="approve",
        invoice_id=invoice.invoice_id,
        discount_fraction=discount,
        offer_price_cents=offer,
        supplier_trust=supplier_trust,
        buyer_trust=buyer_trust,
        reasons=reasons,
    )


# ---------------------------------------------------------------------------
# Batch
# ---------------------------------------------------------------------------

def evaluate_batch(
    financier: str,
    items: List,   # list of (invoice, events) tuples
    trust_fn: TrustFn,
    policy: Optional[FactoringPolicy] = None,
) -> Dict[str, int]:
    """Evaluate a batch; return a summary dict."""
    approve = pending = reject = 0
    for inv, evs in items:
        v = evaluate_factoring(financier, inv, evs, trust_fn, policy).verdict
        if v == "approve":
            approve += 1
        elif v == "pending":
            pending += 1
        else:
            reject += 1
    return {"approve": approve, "pending": pending, "reject": reject}
