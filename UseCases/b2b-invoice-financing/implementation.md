# Implementation: B2B Invoice Financing

## 0. Participants and setup

Each role runs a Quidnug node or uses a shared consortium node:
- **Supplier** — small/medium businesses; often use a shared
  node operated by their industry group.
- **Buyer** — corporate; runs its own node or uses shared.
- **Carrier** — shipping provider; runs its own node and
  signs delivery events.
- **Financier** — factoring firm or bank.
- **Credit bureau** — validator, runs its own node, publishes
  trust edges on-chain.

## 1. Supplier issues an invoice

```bash
# Supplier's application creates a signed TITLE for the invoice
curl -X POST $NODE/api/v1/titles -d '{
  "type":"TITLE",
  "assetId":"inv-supplier-a-2026-04-18-abc",
  "domain":"factoring.supply-chain.us",
  "titleType":"invoice",
  "owners":[{"ownerId":"supplier-a","percentage":100.0}],
  "attributes":{
    "buyer":"buyer-x",
    "amount":"50000.00",
    "currency":"USD",
    "dueDate":"2026-06-17",
    "terms":"NET-60",
    "invoiceHash":"<sha256>",
    "poReference":"PO-BUYER-X-789"
  },
  "creator":"supplier-a",
  "signatures":{"supplier-a":"<sig>"}
}'
```

## 2. Carrier signs ship + delivery events

```bash
# When the carrier picks up the shipment
curl -X POST $NODE/api/v1/events -d '{
  "type":"EVENT",
  "subjectId":"inv-supplier-a-2026-04-18-abc",
  "subjectType":"TITLE",
  "eventType":"carrier.shipped",
  "payload":{
    "carrier":"carrier-z",
    "bol":"BOL-2026-04-18-XYZ",
    "shipDate":1713400000,
    "origin":"supplier-a-warehouse-1",
    "destination":"buyer-x-receiving"
  },
  "creator":"carrier-z",
  "signature":"<carrier sig>"
}'

# When delivery is confirmed
curl -X POST $NODE/api/v1/events -d '{
  "type":"EVENT",
  "subjectId":"inv-supplier-a-2026-04-18-abc",
  "subjectType":"TITLE",
  "eventType":"carrier.delivered",
  "payload":{
    "carrier":"carrier-z",
    "deliveryDate":1713600000,
    "podHash":"<sha256 of proof-of-delivery>"
  },
  "creator":"carrier-z",
  "signature":"<carrier sig>"
}'
```

## 3. Buyer acknowledges

```bash
curl -X POST $NODE/api/v1/events -d '{
  "type":"EVENT",
  "subjectId":"inv-supplier-a-2026-04-18-abc",
  "subjectType":"TITLE",
  "eventType":"buyer.acknowledged",
  "payload":{
    "buyer":"buyer-x",
    "acknowledgedAmount":"50000.00",
    "expectedPayDate":"2026-06-17",
    "notes":"All items received and inspected"
  },
  "creator":"buyer-x",
  "signature":"<buyer sig>"
}'
```

## 4. Financier evaluates and factors

```go
package financier

func (f *Financier) OfferFinancing(ctx context.Context, invoiceID string) (*Offer, error) {
    // 1. Get invoice title
    title, err := f.client.GetTitle(ctx, invoiceID)
    if err != nil { return nil, err }

    // 2. Get event stream
    events, err := f.client.GetSubjectEvents(ctx, invoiceID, "TITLE")
    if err != nil { return nil, err }

    // 3. Check double-factor
    for _, ev := range events {
        if ev.EventType == "factor.purchased" {
            return nil, fmt.Errorf("already factored")
        }
    }

    // 4. Prerequisite events
    hasDelivery := containsEvent(events, "carrier.delivered")
    hasBuyerAck := containsEvent(events, "buyer.acknowledged")
    if !hasDelivery || !hasBuyerAck {
        return &Offer{Decision: "PENDING"}, nil
    }

    // 5. Trust evaluation
    supplier := title.Owners[0].OwnerID
    buyer := title.Attributes["buyer"].(string)

    supTrust, _ := f.client.GetTrust(ctx, f.quid, supplier,
        "factoring.supply-chain.us", nil)
    buyTrust, _ := f.client.GetTrust(ctx, f.quid, buyer,
        "factoring.supply-chain.us", nil)

    // 6. Pricing
    risk := 1.0 - (supTrust.TrustLevel*0.3 + buyTrust.TrustLevel*0.7)
    discount := 0.015 + risk*0.08  // 1.5% - ~10%
    amount, _ := strconv.ParseFloat(title.Attributes["amount"].(string), 64)

    return &Offer{
        Decision:       "APPROVE",
        OfferAmount:    amount * (1 - discount),
        Discount:       discount,
        SupplierTrust:  supTrust.TrustLevel,
        BuyerTrust:     buyTrust.TrustLevel,
    }, nil
}

func (f *Financier) Factor(ctx context.Context, invoiceID string, offer *Offer) error {
    // Emit factor.purchased event — binds the invoice to this financier
    event := map[string]interface{}{
        "type":        "EVENT",
        "subjectId":   invoiceID,
        "subjectType": "TITLE",
        "eventType":   "factor.purchased",
        "payload": map[string]interface{}{
            "financier":      f.quid,
            "purchasePrice":  offer.OfferAmount,
            "discount":       offer.Discount,
            "purchasedAt":    time.Now().Unix(),
        },
        "creator":   f.quid,
        "signature": f.sign(/* ... */),
    }
    return f.submit(ctx, event)

    // In a mature implementation, also submit a TITLE transfer
    // moving ownership from supplier to financier.
}
```

## 5. Payment flow

```bash
# When the buyer pays the financier (now the invoice owner)
curl -X POST $NODE/api/v1/events -d '{
  "type":"EVENT",
  "subjectId":"inv-supplier-a-2026-04-18-abc",
  "subjectType":"TITLE",
  "eventType":"payment.received",
  "payload":{
    "financier":"financier-f",
    "amount":"50000.00",
    "paidDate":1716048000,
    "paymentRef":"WIRE-..."
  },
  "creator":"financier-f",
  "signature":"<financier sig>"
}'
```

## 6. Credit bureau trust declarations

The credit bureau periodically refreshes its trust edges:

```bash
curl -X POST $NODE/api/trust -d '{
  "type":"TRUST",
  "truster":"credit-bureau-dnb",
  "trustee":"supplier-a",
  "trustLevel":0.85,
  "domain":"factoring.supply-chain.us",
  "nonce":<next>,
  "description":"Verified credit: A- rating",
  "validUntil":<now + 90d>
}'
```

Expires in 90 days → forces re-attestation. Financiers see
the edge as stale after that.

## 7. Buyer-declared trust in supplier

Buyers with long relationships can directly endorse their
suppliers:

```bash
curl -X POST $NODE/api/trust -d '{
  "type":"TRUST",
  "truster":"buyer-x",
  "trustee":"supplier-a",
  "trustLevel":0.95,
  "domain":"factoring.supply-chain.us",
  "description":"5-year supplier; never defaulted"
}'
```

This transitive path gives supplier-a credit even if a
particular financier hasn't worked with them directly.

## 8. Dispute handling

Buyer disputes delivery quality:

```bash
curl -X POST $NODE/api/v1/events -d '{
  "type":"EVENT",
  "subjectId":"inv-...",
  "subjectType":"TITLE",
  "eventType":"buyer.disputed",
  "payload":{
    "reason":"quality-issue",
    "evidence":"<hash>",
    "disputedAmount":"5000.00",
    "notes":"100 of 1000 widgets defective"
  },
  "creator":"buyer-x",
  "signature":"<buyer sig>"
}'
```

Financier sees the dispute event. Contract terms define who
bears the risk; dispute events give financier auditable
reason for partial payment / return.

## 9. Fraud signal for forged invoice

Financier caught a forged invoice attempt:

```bash
curl -X POST $NODE/api/v1/events -d '{
  "type":"EVENT",
  "subjectId":"supplier-fraud-x",  /* the fraudulent supplier's quid */
  "subjectType":"QUID",
  "eventType":"fraud.signal.invoice-forged",
  "payload":{
    "reporter":"financier-f",
    "evidence":"Invoice claims PO-789, buyer denies ever issuing that PO",
    "invoiceIds":["inv-..."]
  },
  "creator":"financier-f",
  "signature":"<financier sig>"
}'
```

Propagates via push gossip to other financiers. See
[`../merchant-fraud-consortium/`](../merchant-fraud-consortium/)
for the full fraud-sharing pattern.

## 10. Testing

```go
func TestFactoring_DoubleFactorDetection(t *testing.T) {
    // First factor succeeds.
    // Second factor attempt on same invoice → fails at
    // event-stream check.
}

func TestFactoring_BuyerAckRequired(t *testing.T) {
    // Factor without buyer.acknowledged → PENDING, not approved.
}

func TestFactoring_StaleCreditBureauTrustExpires(t *testing.T) {
    // Credit bureau trust edge past validUntil → not counted.
}
```

## Where to go next

- [`threat-model.md`](threat-model.md)
- Similar fraud-propagation pattern: [`../merchant-fraud-consortium/`](../merchant-fraud-consortium/)
