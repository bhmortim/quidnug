"""Tests for invoice_factor.py. No Quidnug node required."""

import pytest

from invoice_factor import (
    FactoringDecision,
    FactoringPolicy,
    InvoiceFeatures,
    InvoiceV1,
    evaluate_batch,
    evaluate_factoring,
    extract_features,
)


NOW = 1_700_000_000
NET_60 = 60 * 86400


def _invoice(inv_id: str = "inv-1") -> InvoiceV1:
    return InvoiceV1(
        invoice_id=inv_id,
        supplier_quid="supplier-a",
        buyer_quid="buyer-x",
        amount_cents=50_000_00,
        currency="USD",
        issued_at_unix=NOW,
        due_date_unix=NOW + NET_60,
        po_reference="PO-789",
    )


def _trust(mapping):
    return lambda obs, target: mapping.get((obs, target), 0.0)


def _lifecycle_events(
    shipped: bool = True, delivered: bool = True, acked: bool = True,
    carrier: str = "carrier-z",
) -> list:
    out = [{"eventType": "invoice.issued",
            "payload": {"issuer": "supplier-a"}}]
    if shipped:
        out.append({
            "eventType": "carrier.shipped",
            "payload": {"carrierQuid": carrier, "bol": "BOL-001"},
        })
    if delivered:
        out.append({
            "eventType": "carrier.delivered",
            "payload": {"carrierQuid": carrier, "deliveryProof": "POD-001"},
        })
    if acked:
        out.append({
            "eventType": "buyer.acknowledged",
            "payload": {"acknowledgedAmount": 50000, "ackDate": NOW + 86400},
        })
    return out


# ---------------------------------------------------------------------------
# Happy path
# ---------------------------------------------------------------------------

def test_full_lifecycle_approves():
    trust = _trust({
        ("financier-f", "supplier-a"): 0.85,
        ("financier-f", "buyer-x"): 0.90,
    })
    d = evaluate_factoring(
        "financier-f", _invoice(), _lifecycle_events(), trust,
    )
    assert d.verdict == "approve"
    assert d.discount_fraction > 0.01   # at least the base
    assert d.offer_price_cents < 50_000_00


def test_higher_risk_higher_discount():
    """Lower buyer trust -> higher discount."""
    ev = _lifecycle_events()
    d_high_trust = evaluate_factoring(
        "f", _invoice(),
        ev,
        _trust({("f", "supplier-a"): 0.95, ("f", "buyer-x"): 0.95}),
    )
    d_low_trust = evaluate_factoring(
        "f", _invoice(),
        ev,
        _trust({("f", "supplier-a"): 0.7, ("f", "buyer-x"): 0.65}),
    )
    assert d_high_trust.verdict == "approve"
    assert d_low_trust.verdict == "approve"
    assert d_low_trust.discount_fraction > d_high_trust.discount_fraction


# ---------------------------------------------------------------------------
# Missing events
# ---------------------------------------------------------------------------

def test_missing_delivery_is_pending():
    trust = _trust({("f", "supplier-a"): 0.9, ("f", "buyer-x"): 0.9})
    d = evaluate_factoring(
        "f", _invoice(), _lifecycle_events(delivered=False), trust,
    )
    assert d.verdict == "pending"
    assert "no carrier.delivered" in d.reasons[0]


def test_missing_buyer_ack_is_pending():
    trust = _trust({("f", "supplier-a"): 0.9, ("f", "buyer-x"): 0.9})
    d = evaluate_factoring(
        "f", _invoice(), _lifecycle_events(acked=False), trust,
    )
    assert d.verdict == "pending"
    assert "no buyer.acknowledged" in d.reasons[0]


def test_policy_can_relax_requirements():
    """Some financiers are comfortable factoring before delivery
    (e.g. against PO), so policy lets them waive the requirement."""
    policy = FactoringPolicy(require_delivery=False, require_buyer_ack=False)
    trust = _trust({("f", "supplier-a"): 0.9, ("f", "buyer-x"): 0.9})
    d = evaluate_factoring(
        "f", _invoice(),
        _lifecycle_events(delivered=False, acked=False),
        trust, policy,
    )
    assert d.verdict == "approve"


# ---------------------------------------------------------------------------
# Double-factoring
# ---------------------------------------------------------------------------

def test_double_factoring_rejects():
    events = _lifecycle_events() + [{
        "eventType": "factor.purchased",
        "payload": {"financier": "financier-g", "purchasePrice": 48500},
    }]
    trust = _trust({("f", "supplier-a"): 0.9, ("f", "buyer-x"): 0.9})
    d = evaluate_factoring("f", _invoice(), events, trust)
    assert d.verdict == "reject"
    assert "double-factoring" in d.fraud_flags


# ---------------------------------------------------------------------------
# Fraud signals
# ---------------------------------------------------------------------------

def test_fraud_signal_rejects():
    events = _lifecycle_events() + [{
        "eventType": "fraud.signal.invoice-forged",
        "payload": {"evidence": "PDF signed with a key not belonging to supplier-a"},
    }]
    trust = _trust({("f", "supplier-a"): 0.9, ("f", "buyer-x"): 0.9})
    d = evaluate_factoring("f", _invoice(), events, trust)
    assert d.verdict == "reject"
    assert "invoice-forged" in d.fraud_flags


# ---------------------------------------------------------------------------
# Trust thresholds
# ---------------------------------------------------------------------------

def test_low_supplier_trust_rejects():
    trust = _trust({("f", "supplier-a"): 0.2, ("f", "buyer-x"): 0.9})
    d = evaluate_factoring(
        "f", _invoice(), _lifecycle_events(), trust,
    )
    assert d.verdict == "reject"
    assert "supplier trust" in " ".join(d.reasons).lower()


def test_low_buyer_trust_rejects():
    trust = _trust({("f", "supplier-a"): 0.9, ("f", "buyer-x"): 0.3})
    d = evaluate_factoring(
        "f", _invoice(), _lifecycle_events(), trust,
    )
    assert d.verdict == "reject"
    assert "buyer trust" in " ".join(d.reasons).lower()


def test_out_of_range_trust_raises():
    trust = _trust({("f", "supplier-a"): 1.5, ("f", "buyer-x"): 0.9})
    with pytest.raises(ValueError):
        evaluate_factoring("f", _invoice(), _lifecycle_events(), trust)


# ---------------------------------------------------------------------------
# Policy validation
# ---------------------------------------------------------------------------

def test_invalid_policy_weights_raises():
    trust = _trust({("f", "supplier-a"): 0.9, ("f", "buyer-x"): 0.9})
    bad = FactoringPolicy(buyer_weight=0.5, supplier_weight=0.5 + 0.01)
    with pytest.raises(ValueError):
        evaluate_factoring("f", _invoice(), _lifecycle_events(), trust, bad)


def test_custom_policy_thresholds():
    """Conservative policy rejects the same deal a permissive one approves."""
    events = _lifecycle_events()
    trust = _trust({("f", "supplier-a"): 0.55, ("f", "buyer-x"): 0.65})

    strict = FactoringPolicy(min_supplier_trust=0.8, min_buyer_trust=0.8)
    permissive = FactoringPolicy(min_supplier_trust=0.5, min_buyer_trust=0.5)

    d_strict = evaluate_factoring("f", _invoice(), events, trust, strict)
    d_permissive = evaluate_factoring("f", _invoice(), events, trust, permissive)

    assert d_strict.verdict == "reject"
    assert d_permissive.verdict == "approve"


# ---------------------------------------------------------------------------
# Feature extraction
# ---------------------------------------------------------------------------

def test_extract_features():
    f = extract_features(_lifecycle_events())
    assert f.has_ship_event
    assert f.has_delivery_event
    assert f.has_buyer_ack
    assert f.prior_factor_count == 0
    assert f.carrier_quid == "carrier-z"


def test_extract_features_with_factor_and_fraud():
    events = _lifecycle_events() + [
        {"eventType": "factor.purchased",
         "payload": {"financier": "fin-g"}},
        {"eventType": "fraud.signal.invoice-forged",
         "payload": {"evidence": "..."}},
    ]
    f = extract_features(events)
    assert f.prior_factor_count == 1
    assert len(f.fraud_signals) == 1


# ---------------------------------------------------------------------------
# Batch
# ---------------------------------------------------------------------------

def test_batch_summary():
    trust = _trust({
        ("f", "supplier-a"): 0.9,
        ("f", "buyer-x"): 0.9,
        # supplier-b unknown -> trust 0 -> reject
    })
    items = [
        (_invoice("inv-1"), _lifecycle_events()),
        # Pending (no delivery).
        (_invoice("inv-2"), _lifecycle_events(delivered=False)),
        # Reject (unknown supplier).
        (InvoiceV1("inv-3", "supplier-b", "buyer-x",
                    10_000_00, "USD", NOW, NOW + NET_60),
         _lifecycle_events()),
    ]
    summary = evaluate_batch("f", items, trust)
    assert summary == {"approve": 1, "pending": 1, "reject": 1}
