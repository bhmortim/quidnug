# B2B invoice financing, POC demo

Runnable proof-of-concept for the
[`UseCases/b2b-invoice-financing/`](../../UseCases/b2b-invoice-financing/)
use case. Demonstrates cross-party invoice lifecycle attestation
and relational-trust-gated factoring decisions, with
double-factoring prevention and fraud-signal propagation.

## What this POC proves

Six actors (supplier, buyer, carrier, credit bureau, two
competing financiers) sharing a factoring domain. Key claims the
demo verifies:

1. **Cross-party lifecycle attestation works.** The supplier
   issues the invoice, the carrier signs shipped + delivered
   events, the buyer signs an acknowledgment: four independent
   parties each attesting to a segment of the invoice's
   lifecycle on the invoice's event stream.
2. **Financiers gate on complete signals.** Missing delivery or
   missing buyer acknowledgment returns `pending`, not `approve`.
   A financier who relaxes either via policy (e.g. PO-financing)
   can opt out of those requirements.
3. **Discount tracks relational risk.** High trust in both
   supplier and buyer -> lower discount. Weaker trust -> larger
   discount, with a hard reject below the financier's minimum
   threshold.
4. **Double-factoring is caught via stream check.** Once a
   `factor.purchased` event lands, any subsequent financier
   reading the stream sees it and rejects. No central registry
   required.
5. **Fraud signals propagate.** A `fraud.signal.invoice-forged`
   event from any party pollutes the stream for every
   downstream verifier.

## What's in this folder

| File | Purpose |
|---|---|
| `invoice_factor.py` | Pure decision logic. `InvoiceV1`, `FactoringPolicy`, `evaluate_factoring`, `evaluate_batch`, feature extractor. No SDK dep. |
| `invoice_factor_test.py` | 15 pytest cases: happy path, risk-tracks-discount, missing events pending, policy relaxation, double-factoring, fraud signals, trust thresholds, policy validation, extraction, batch. |
| `demo.py` | End-to-end runnable against a live node. Twelve steps: register actors, set up cross-bureau trust, issue invoice, ship/deliver/ack lifecycle, two financiers race for the same invoice, fraud-flagged second invoice. |

## Running

```bash
# 1. Start a local node.
cd deploy/compose && docker compose up -d

# 2. Install Python SDK.
cd clients/python && pip install -e .

# 3. Run the demo.
cd examples/b2b-invoice-financing
python demo.py
```

## Testing without a live node

```bash
cd examples/b2b-invoice-financing
python -m pytest invoice_factor_test.py -v
```

## QDP catalog audit

| Feature | Purpose | Status |
|---|---|---|
| `IDENTITY` tx | Register supplier, buyer, carrier, bureau, financiers | v1.0 |
| `TITLE` tx | Invoice as an on-chain asset owned by the supplier | v1.0 |
| `TRUST` tx | Credit-bureau trust in parties, financier trust in bureau | v1.0 |
| Transitive trust query | Financier's effective trust through the bureau | v1.0 |
| `EVENT` tx streams | Lifecycle: issued, shipped, delivered, acknowledged, factored | v1.0 |
| QDP-0005 push gossip | Fast cross-party propagation of factor events | v1.0 |
| QDP-0002 guardian recovery | Supplier finance-team key recovery | v1.0 (not exercised) |
| QDP-0007 lazy epoch probe | Cross-jurisdiction validation | v1.0 (not exercised) |
| QDP-0019 decay | Stale invoices fade naturally | Phase 1 landed; optional |

No protocol gaps.

## What a production deployment would add

- **Ownership transfer on factor.** The demo records a
  `factor.purchased` event but leaves the TITLE's ownership on
  the supplier. A production system would follow the event with
  a transfer to the financier so the on-chain ownership record
  matches the economic reality (and blocks any cute side-channel
  attempts to factor again).
- **Carrier rating agency as another trust source.** The demo
  has the bureau trust the supplier and buyer but doesn't model
  a carrier-rating agency. Adding one is a two-line change:
  another grant_trust edge from the rating agency to the carrier.
- **Real PO / invoice PDF hashing.** The demo stores only the
  structured payload in events. Production would hash the actual
  PDF + line items and stash the hash on the `invoice.issued`
  event for dispute-time verification.
- **Buyer dispute events.** `buyer.disputed` with a reason code,
  and a financier policy that treats a dispute as a hard
  reject-unless-resolved.
- **Chain of custody proofs.** `carrier.scanned` checkpoint
  events between shipped and delivered, with geolocation and
  signer identity, let a financier see partial progress.

## Related

- Use case: [`UseCases/b2b-invoice-financing/`](../../UseCases/b2b-invoice-financing/)
- Related POC: [`examples/merchant-fraud-consortium/`](../merchant-fraud-consortium/)
  for the cross-party fraud-signal propagation pattern; the
  pattern applies directly here against forged invoices
- Related POC: [`examples/credential-verification-network/`](../credential-verification-network/)
  for the transitive-trust-through-a-bureau pattern, which is
  the same shape as the financier -> bureau -> supplier chain here
- Protocol: [QDP-0005 Push Gossip](../../docs/design/0005-push-based-gossip.md)
