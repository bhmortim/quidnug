# B2B Invoice Financing

**FinTech · Supply chain · Multi-party validation**

## The problem

Invoice factoring is a $3 trillion global industry. A supplier
ships goods to a buyer on NET-60 terms; the supplier needs cash
before 60 days; a financier buys the invoice at a discount,
assuming the buyer will pay in full.

Fraud risk is central:
- **Fake invoices** — supplier creates a fictional invoice
  and tries to factor it.
- **Double-factoring** — supplier factors the same invoice
  with two financiers.
- **Non-delivery** — supplier factors an invoice for goods
  that were never shipped.
- **Buyer dispute** — buyer refuses to pay due to quality or
  delivery issues.

Current industry solutions:
- **Central platforms** (Tradeshift, Taulia) — each platform
  has a walled garden. Multi-platform fraud is possible.
- **Bank-run programs** — each bank has its own factoring
  program; cross-bank fraud detection is minimal.
- **Paper / PDF trails** — still common, especially in
  emerging markets and small-to-medium B2B.

The fundamental need: **multiple independent parties should
attest to the same invoice lifecycle events, and financiers
should see those attestations before buying**.

## Why Quidnug fits

An invoice is a "thing with multiple stakeholders who all need
to attest to its state." That's what title + event streams +
relational trust are designed for.

| Problem                                           | Quidnug primitive                                 |
|---------------------------------------------------|---------------------------------------------------|
| "Is this invoice actually issued by the supplier?"| TITLE transaction signed by supplier              |
| "Did the buyer acknowledge receipt?"              | EVENT: buyer.acknowledged                         |
| "Was it factored before?"                         | EVENTS on same title tracked by unique ID        |
| "Is this supplier creditworthy?"                  | Relational trust in supplier via credit bureau    |
| "Is the carrier reliable?"                        | Relational trust in shipping carrier              |
| "Cross-platform fraud check"                      | Quidnug domain unifies participants               |

## High-level architecture

```
           ┌─────────────────────────────────────────┐
           │   factoring.supply-chain (domain)       │
           │                                          │
           │   Validators: credit-bureau, carrier-   │
           │   rating-agency, industry-group          │
           └─────────────────────────────────────────┘
                        │
        ┌───────────────┼───────────────┬─────────────┐
        │               │               │             │
        ▼               ▼               ▼             ▼
   Supplier A       Buyer X         Carrier Z      Financier F
   (quid)           (quid)          (quid)         (quid)
      │                │                │              │
      │ TITLE          │ EVENT          │ EVENT        │ EVENT
      │ (invoice)      │ (ack)          │ (ship)       │ (factor)
      │                │                │              │
      ▼                ▼                ▼              ▼
   ┌─────────────────────────────────────────────────────┐
   │         Invoice title + event stream                 │
   │  inv-2026-04-18-abc → [issued, shipped, delivered,   │
   │                         acked, factored, paid]       │
   └─────────────────────────────────────────────────────┘
```

## Data model

### Quids
- **Supplier**: company-level quid, with its own finance
  team's guardian set.
- **Buyer**: company-level quid.
- **Carrier**: shipping provider; signs delivery events.
- **Financier**: factoring company.
- **Credit bureau** (e.g., Dun & Bradstreet, rating agency):
  runs a Quidnug node, declares trust in suppliers/buyers
  based on their own data.

### Domain
```
factoring.supply-chain
├── factoring.supply-chain.us
├── factoring.supply-chain.eu
└── factoring.supply-chain.asia
```

### Invoice as title

```json
{
  "type": "TITLE",
  "assetId": "inv-supplier-a-2026-04-18-abc",
  "domain": "factoring.supply-chain.us",
  "titleType": "invoice",
  "owners": [{"ownerId": "supplier-a", "percentage": 100.0}],
  "attributes": {
    "buyer": "buyer-x",
    "amount": "50000.00",
    "currency": "USD",
    "dueDate": "2026-06-17",
    "terms": "NET-60",
    "lineItems": [
      {"description": "Widget A", "qty": 1000, "unitPrice": "50.00"}
    ],
    "poReference": "PO-BUYER-X-789",
    "invoiceHash": "<sha256 of full invoice PDF>"
  },
  "creator": "supplier-a",
  "signatures": {
    "supplier-a": "<supplier's signature>"
  }
}
```

The full PDF stays in the supplier's system; only a hash and
structured metadata go on-chain.

### Events through the lifecycle

```
Event stream for "inv-supplier-a-2026-04-18-abc":

1. invoice.issued
   payload: {issuer: supplier-a, amount: 50000, ...}
   signer: supplier-a

2. carrier.shipped
   payload: {carrier: carrier-z, bol: "BOL-...", shipDate: ...}
   signer: carrier-z

3. carrier.delivered
   payload: {deliveryProof: "hash-of-POD"}
   signer: carrier-z

4. buyer.acknowledged
   payload: {acknowledgedAmount: 50000, expectedPayDate: 2026-06-17}
   signer: buyer-x

5. factor.purchased
   payload: {financier: financier-f, discount: 0.03,
             purchasePrice: 48500}
   signer: financier-f
   (after this, financier is the entity buyer should pay)

6. payment.received
   payload: {amount: 50000, paidDate: 2026-06-15}
   signer: financier-f (confirming receipt)
```

### Validator trust

The domain has validators (credit bureau, carrier rating
agency, industry group). They emit trust-edge transactions
declaring their views on participants:

```
credit-bureau-dnb ──0.85──► supplier-a  (verified; good payment history)
carrier-rating-ag ──0.92──► carrier-z   (A-rated carrier)
industry-group ──0.8──► buyer-x         (member in good standing)
```

Financiers consume these + add their own direct trust.

## Financier decision flow

A financier considering buying the invoice:

```go
type FactoringDecision struct {
    BuyerCredit       float64
    SupplierReputation float64
    CarrierReputation  float64
    FraudSignals       []FraudSignal
    DiscountOffered    float64
    Decision           string
}

func (f *Financier) EvaluateInvoice(invoiceID string) FactoringDecision {
    invoice := f.quidnug.GetTitle(invoiceID)

    // Trust in supplier
    supplierTrust := f.quidnug.GetTrust(f.selfQuid,
        invoice.Attributes["creator"], "factoring.supply-chain.us")

    // Trust in buyer (creditworthiness)
    buyerTrust := f.quidnug.GetTrust(f.selfQuid,
        invoice.Attributes["buyer"], "factoring.supply-chain.us")

    // Check for prior factor events (double-factoring detection)
    events := f.quidnug.GetSubjectEvents(invoiceID, "TITLE")
    for _, ev := range events {
        if ev.EventType == "factor.purchased" {
            return FactoringDecision{Decision: "REJECT",
                FraudSignals: []FraudSignal{ {Type: "double-factoring"} }}
        }
    }

    // Check for delivery events
    hasDelivery := false
    for _, ev := range events {
        if ev.EventType == "carrier.delivered" {
            hasDelivery = true
        }
    }

    // Buyer acknowledgment?
    hasBuyerAck := false
    for _, ev := range events {
        if ev.EventType == "buyer.acknowledged" {
            hasBuyerAck = true
        }
    }

    // Decision logic
    if !hasDelivery {
        return FactoringDecision{Decision: "PENDING",
            Reason: "No delivery event yet"}
    }
    if !hasBuyerAck {
        return FactoringDecision{Decision: "PENDING",
            Reason: "Buyer hasn't acknowledged"}
    }

    risk := 1.0 - (supplierTrust.TrustLevel * 0.3 + buyerTrust.TrustLevel * 0.7)
    discount := 0.01 + risk * 0.1

    return FactoringDecision{
        BuyerCredit:        buyerTrust.TrustLevel,
        SupplierReputation: supplierTrust.TrustLevel,
        DiscountOffered:    discount,
        Decision:           "APPROVE",
    }
}
```

## Double-factoring prevention

Key pattern: a single `factor.purchased` event per invoice.
If a supplier tries to factor with a second financier:

- Financier-2 pulls the invoice's event stream before buying.
- Sees a prior `factor.purchased` event.
- Rejects.

Even across Quidnug nodes in different jurisdictions, push
gossip ensures the `factor.purchased` event reaches all
interested parties within seconds.

For additional safety, the title's ownership transfers:

```
factor.purchased event triggers app-layer logic to update
the invoice's TITLE ownership from supplier → financier.
A second financier sees the supplier no longer owns the
invoice and rejects.
```

## Fraud signal propagation

The `merchant-fraud-consortium` pattern applies here too.
Financiers can emit fraud signals against suppliers or
invoices they've caught:

```
eventType: "fraud.signal.invoice-forged"
subjectId: <supplier quid>
payload: {invoiceId: "...", evidence: "..."}
```

Other financiers consume these signals with their own trust
weighting.

## Key Quidnug features

- **TITLE transactions** for invoices with ownership
  transfer support.
- **Event streams** for the invoice lifecycle.
- **Relational trust** for supplier/buyer/carrier evaluation.
- **Push gossip (QDP-0005)** for fast cross-participant
  propagation.
- **Guardian sets** for supplier finance teams (HSM failure
  recovery).
- **Lazy epoch probe (QDP-0007)** for cross-jurisdiction
  validation.
- **Domain hierarchy** for regional scoping.

## Value delivered

| Dimension                              | Before                                    | With Quidnug                                          |
|----------------------------------------|-------------------------------------------|-------------------------------------------------------|
| Double-factoring detection             | Platform-specific                         | Cross-platform via unified event stream                |
| Invoice forgery detection              | After-the-fact reconciliation             | Cryptographic signatures at issuance                   |
| Buyer acknowledgment                   | Emailed confirmation                      | On-chain signed event                                  |
| Delivery verification                  | Paper bill-of-lading + trust              | Signed event from rated carrier                        |
| Financier time-to-decision             | Days (manual review)                      | Seconds (automated evaluation)                         |
| Cross-border invoice factoring         | Major integration project                 | Domain-scoped trust + gossip                           |
| Dispute resolution evidence            | Contested documentation                   | Signed event chain, tamper-evident                     |
| Credit bureau integration              | Bureau-specific API                       | Bureau publishes trust edges on-chain                  |

## What's in this folder

- [`README.md`](README.md) — this document
- [`implementation.md`](implementation.md) — concrete code
- [`threat-model.md`](threat-model.md) — security analysis

## Related

- [`../merchant-fraud-consortium/`](../merchant-fraud-consortium/) —
  fraud signal propagation
- [`../interbank-wire-authorization/`](../interbank-wire-authorization/) —
  M-of-N signing for high-value payments
- [QDP-0005 Push Gossip](../../docs/design/0005-push-based-gossip.md)
