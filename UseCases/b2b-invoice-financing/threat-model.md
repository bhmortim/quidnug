# Threat Model: B2B Invoice Financing

## Assets

1. **Financier capital** — money paid to suppliers for invoices.
2. **Invoice event-stream integrity** — the cryptographic
   record of the invoice's lifecycle.
3. **Supplier / buyer reputation** — trust edges that drive
   future financing decisions.

## Attackers

| Attacker       | Capability                                    | Goal                         |
|----------------|-----------------------------------------------|------------------------------|
| Fraudster      | Can create fake supplier identity             | Factor fake invoices         |
| Rogue supplier | Legitimate supplier, dishonest intent         | Double-factor, forge docs    |
| Buyer          | Legitimate, may dispute for cash-flow reasons | Delay payment                |
| Carrier        | Legitimate, may mis-sign in exchange for bribes| Confirm non-delivery         |
| Financier      | Legitimate, competitive intel                 | Access others' invoice data  |

## Threats and mitigations

### T1. Fake invoice from fake supplier

**Attack.** Fraudster creates a quid called `supplier-fake`,
attempts to factor a fictitious invoice.

**Mitigations.**
- **Low starting trust.** Financiers use relational trust;
  a supplier with no credit-bureau or buyer endorsements has
  trust ~0. Their invoices aren't factored.
- **Credit bureau participation** — legitimate suppliers are
  endorsed by credit bureaus; fraudsters aren't.
- **Buyer acknowledgment required** — fraudster needs a
  real buyer to acknowledge a non-existent invoice.
  Financiers can require this before factoring.

### T2. Forged invoice from real supplier

**Attack.** Real supplier creates a fake invoice for non-
existent goods and tries to factor.

**Mitigations.**
- **Buyer acknowledgment** — buyer's signed event is the
  critical validation. Buyer won't acknowledge an invoice
  they didn't actually order.
- **Carrier delivery event** — carrier (with industry
  reputation trust) signs delivery.
- Both need to be compromised for a forged invoice to
  progress.

### T3. Double factoring

**Attack.** Supplier tries to factor the same invoice with
two financiers.

**Mitigations.**
- **Single event stream per invoice.** `factor.purchased`
  event is visible to all financiers. Second attempt sees
  the first and rejects.
- **Push gossip** propagates the factor event within seconds
  to all participating nodes.
- **Title-ownership transfer** (app-layer): after
  `factor.purchased`, the invoice TITLE's owner updates
  from supplier to financier. Second financier sees the
  supplier no longer owns it.

**Residual risk.** Narrow window (seconds) during push
propagation where a very fast fraudster could hit two
financiers simultaneously. Mitigated by operational delay
in financier decision (trust evaluation takes milliseconds
but approval workflow is minutes).

### T4. Buyer collusion with supplier

**Attack.** Buyer and supplier collude: supplier issues
invoice for fake goods, buyer acknowledges, they split the
financier's money.

**Mitigations.**
- **Carrier delivery event** — carrier is an independent
  third party (high trust); colluding suppliers can't fake
  the carrier's signature.
- **Credit bureau + industry group** as validators — they
  detect patterns (buyer suddenly ordering 10× normal volume,
  etc.).

**Residual risk.** Protocol can't prevent all collusion;
financier's own KYC/UBO checks + credit analysis are the
complementary controls.

### T5. Carrier collusion

**Attack.** Carrier and supplier collude: carrier signs
fake `carrier.delivered` events.

**Mitigations.**
- **Carrier rating trust** — industry rating agencies
  publish trust in carriers. A low-trust carrier's signature
  is weighted less.
- **Cross-reference** — carrier's own fleet / tracking data
  should match the event. If industry-group validators
  detect divergence, carrier's reputation tanks.

### T6. Replay

**Attack.** Attacker replays a valid `buyer.acknowledged`
event on a different invoice.

**Mitigations.**
- **Subject-ID binding** — the event's `subjectId` is
  the invoice ID; replaying to a different invoice requires
  re-signing (can't, no key).
- **Anchor nonce monotonicity.**

### T7. Buyer dispute abuse

**Attack.** Buyer routinely disputes invoices to delay
payment, manipulating cash flow.

**Mitigations.**
- **Dispute patterns visible** — an event-stream history
  shows a buyer's dispute-rate. Financiers raise the
  discount they charge on that buyer's invoices.
- **Buyer's reputation trust** drops with repeated disputes.

### T8. Cross-border regulatory/legal complexity

**Attack.** Invoice spans US and EU jurisdictions; one
party is subject to sanctions, injunction, etc.

**Mitigations.**
- **Domain scoping** — `.us` and `.eu` subdomains have
  different rules (e.g., GDPR-compliant data retention).
- **Jurisdiction-specific validators** at the domain level
  can refuse to admit sanctioned parties.

**Residual risk.** Legal compulsion is legal compulsion.
Protocol documents the events; compliance is on each party.

### T9. Privacy leaks

**Attack.** Competitor observes which financiers are buying
which invoices, inferring customer relationships.

**Mitigations.**
- Raw invoice contents stay off-chain (hashed).
- Structured metadata on-chain is more limited — amount,
  due date, counterparty identity.
- For high-privacy deployments, participants can use
  pseudonymous quids (supplier uses a different quid per
  financing relationship).

### T10. Compromised financier

**Attack.** Financier's signing key compromised; attacker
signs fake `factor.purchased` events to claim ownership
of invoices they didn't actually buy.

**Mitigations.**
- **Guardian recovery** — financier rotates away from the
  compromised key.
- **Anchor nonce check** — attacker's signature at an old
  nonce rejected.
- **Invalidation** — the compromised epoch frozen
  immediately after detection.

## Not defended against

1. **Physical commodity quality** — invoice may say "1000
   widgets," actual goods may vary. Inspection is physical-
   world.
2. **Supplier insolvency post-factor** — financier bought
   the invoice; if buyer doesn't pay (legitimate dispute,
   bankruptcy), supplier owes back. Contract-level concern.
3. **Nation-state-level forgery** — if a jurisdiction
   coerces a carrier or credit bureau to sign false events,
   Quidnug documents the coercion but can't prevent it.
4. **Insurance and underwriting** — which risks are taken
   by financier vs. supplier vs. insurance is contractual,
   not protocol.

## References

- [`../merchant-fraud-consortium/threat-model.md`](../merchant-fraud-consortium/threat-model.md) —
  fraud-signal propagation
- [QDP-0005 Push Gossip](../../docs/design/0005-push-based-gossip.md)
