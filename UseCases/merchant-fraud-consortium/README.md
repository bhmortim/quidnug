# Merchant Fraud Consortium

**FinTech · Cross-organization data sharing · Relational trust**

## The problem

Merchants and payment processors collectively see billions of
fraud signals per day. An attacker card-testing at Merchant A
today hits Merchant B tomorrow. Current industry attempts at
shared-fraud databases all founder on the same wall:

**If I share my data, my competitors get value.  If I don't,
we're all worse off.  And if I share bad data (either by
accident or malice), I poison everyone else.**

Concretely:
- **Visa/Mastercard fraud feeds** exist but are reactive,
  aggregated, and delayed. Real-time cross-merchant signal isn't
  there.
- **Third-party data consortiums** (e.g., Sift, Signifyd
  networks) require every participant to trust the central
  operator, who now has the keys to a very valuable database.
- **Direct merchant-to-merchant sharing** exists at the informal
  level (private Slack channels for security leads) and at the
  heavyweight level (ISACs) but with no structured trust model.

The fundamental issue is that **trust in fraud reports isn't
universal**. A 500-merchant consortium will have:
- A few very-high-signal reporters (large merchants with
  sophisticated fraud teams).
- Many medium-quality reporters.
- Some noisy / adversarial / compromised accounts whose reports
  should be ignored or even reversed.

A single "is this reporter trusted" toggle doesn't capture reality.
Each consumer of a fraud signal should weight it differently.

## Why Quidnug fits

This is **relational trust's natural shape**. Each merchant
computes their own trust in every other merchant, based on
direct experience and transitive endorsements through mutual
business partners.

| Problem                                          | Quidnug primitive                              |
|--------------------------------------------------|------------------------------------------------|
| "Do I trust signals from merchant X?"            | Trust edge in `fraud.signals.us-retail`        |
| "Did X actually sign this report?"               | ECDSA signature on event-stream entry          |
| "Has X been compromised and is re-playing?"      | Monotonic anchor nonces                        |
| "How do I weigh a new merchant's reports?"       | Transitive trust through mutual peers          |
| "How do signals propagate in near-real-time?"    | Push gossip (QDP-0005)                         |
| "How do I bootstrap a new member?"               | K-of-K snapshot (QDP-0008)                     |

## High-level architecture

```
                          Consortium
                         ─────────────
                    (members run their own
                     Quidnug nodes, no central
                     operator)

  ┌────────────────┐   gossip   ┌────────────────┐
  │ Acme Retail    │ ◄────────► │  BigBox Inc    │
  │ (Quidnug node) │            │  (Quidnug node)│
  └───────┬────────┘            └────────┬───────┘
          │                              │
          │ gossip                       │ gossip
          │                              │
          ▼                              ▼
  ┌────────────────┐            ┌────────────────┐
  │ Fin-Tech #1    │ ◄────────► │  Bank.com      │
  │ (Quidnug node) │    gossip  │  (Quidnug node)│
  └────────────────┘            └────────────────┘

         Each member emits fraud signals as events
         into a shared "fraud.signals.us-retail" domain.
         Each member computes trust FROM THEIR OWN PERSPECTIVE
         when evaluating a signal.
```

## Data model

### Quids
- One per member organization (Acme, BigBox, Fin-Tech #1, etc.).
- Optionally, sub-quids per fraud-team analyst (so compromises are
  contained to one analyst's key).

### Domain
- `fraud.signals.us-retail` (main).
- Sub-domains by vertical: `fraud.signals.us-retail.apparel`,
  `.electronics`, `.travel`.
- Members subscribe to subdomains relevant to them.

### Signal as event stream entry

Every fraud signal is an event on a subject quid that represents
the flagged entity (card fingerprint, IP, device ID, email
pattern). Example:

```json
{
  "type": "EVENT",
  "subjectId": "card-fp-8f3a9b...",
  "subjectType": "QUID",
  "eventType": "fraud.signal.card-testing",
  "payload": {
    "reporter": "acme-retail",
    "severity": 0.9,
    "evidence": {
      "pattern": "multiple-CVV-retries",
      "window": "5min",
      "observedAt": 1713400000
    },
    "actionTaken": "decline",
    "comment": "Testing stolen card numbers against $1 donations."
  },
  "signature": "<Acme's signature at its current epoch>"
}
```

### Trust edges

Each member declares trust in peers they find reliable:

```
acme-retail ──0.9──► bigbox-inc    (work with them for years)
acme-retail ──0.7──► fin-tech-1    (known good reputation)
acme-retail ──0.3──► newcomer-ltd  (just joined, unproven)

bigbox-inc ──0.95──► acme-retail   (mutual)
bigbox-inc ──0.8──► newcomer-ltd   (BigBox did their DD)
```

Now when Newcomer submits a signal, Acme (direct trust 0.3) can
also weigh through BigBox's endorsement (0.95 × 0.8 = 0.76) and
take the max — Newcomer's signals get effective trust 0.76 from
Acme's perspective, not the pessimistic 0.3.

## Signal consumption flow

```
1. Acme sees a card-testing pattern hit their checkout.
2. Acme submits a fraud signal event against the card fingerprint:
     POST /api/v1/events
     eventType: "fraud.signal.card-testing"

3. Push gossip (QDP-0005) propagates the event to peer nodes
   within DomainGossipInterval * hops.

4. BigBox's node receives it. BigBox's fraud system polls for
   new events in `fraud.signals.us-retail.*`.

5. For each received event, BigBox computes:
     relTrust(bigbox-inc → reporter) in the signal's domain.

6. BigBox's decision engine blocks / flags / allows the card
   based on severity × trust.

7. BigBox optionally emits their own corroborating event.
   Now the card fingerprint's stream has TWO corroborating
   signals, which raises the effective score for everyone
   seeing both.
```

## Counter-signals ("this wasn't fraud")

Recovery path for false positives. If BigBox discovers a flagged
card was actually a legitimate customer, they emit:

```json
{
  "eventType": "fraud.signal.counter",
  "payload": {
    "reporter": "bigbox-inc",
    "counters": ["<event-id of original signal>"],
    "evidence": "customer verified; purchased $2000 of verified goods"
  }
}
```

Consumers see both the original signal and the counter, and can
make a better-informed decision. The original signal isn't
retracted (that would let malicious reporters hide their history);
it's complemented.

## Reporter reputation management

A reporter whose signals are frequently countered by trusted
peers sees their **effective incoming trust** decay naturally.
Each consumer can apply their own heuristic:

```
effective_trust(reporter) = base_trust(reporter) 
                          - 0.5 * fraction_countered
```

Publish this effective weighting via lower trust edges. Over
time, a noisy reporter gets deprioritized by the network without
any central operator making that call.

## Compromised reporter detection

If Acme's signing key is compromised and an attacker floods
fake signals ("this legitimate card is fraud"), there are
several responses:

1. **Rate limit on the gossip layer** (QDP-0005 §7) — per-producer
   message rate limit means one compromised member can only flood
   so fast.
2. **Counter-signal cascade** — peers that see legit cards flagged
   emit counter-signals, degrading the attacker's effective
   reputation.
3. **Emergency rotation via guardian recovery** — Acme's fraud
   team's personal guardians (security team, CTO) can rotate Acme's
   key, invalidating the attacker's signing authority.

The critical point: **no central operator can be coerced into
kicking Acme off the consortium**. Recovery is member-driven.

## Key Quidnug features used

- **Relational trust in `fraud.signals.us-retail`** — each member
  has their own view, so a bad actor can't poison universally.
- **Push gossip (QDP-0005)** — signals propagate within ~30 seconds.
  Faster than polling a central feed.
- **Anchor nonces (QDP-0001)** — replay protection on signals. A
  signal can't be re-submitted twice for double-counting.
- **Guardian recovery (QDP-0002)** — if a member's signing key is
  compromised, their security team can rotate without disrupting
  the consortium's operation.
- **Domain hierarchy** — members subscribe to vertical-specific
  subdomains. An apparel retailer doesn't need electronics-
  fraud signals.
- **K-of-K bootstrap (QDP-0008)** — a new member joins by getting
  snapshots of the trust graph from K existing trusted members.
- **Fork-block trigger (QDP-0009)** — when the consortium agrees
  to (e.g.) require signal severity ≥ 0.5 to propagate beyond one
  hop, a fork-block activates the change uniformly.

## Scale estimates

Typical large consortium:
- 500 member organizations
- 100,000+ fraud signals/day across the network
- ~2 MB/day of signal events per member on average
- Trust graph: ~500 × ~50 trust edges each = 25,000 edges

This is well within a single Quidnug node's comfort zone. Push
gossip handles propagation; each member node processes only
signals in their subscribed subdomains.

## Value delivered

| Dimension                              | Before                                     | With Quidnug                                          |
|----------------------------------------|--------------------------------------------|-------------------------------------------------------|
| Real-time cross-merchant signals       | Delayed batches or informal Slack          | ~30-second propagation                                 |
| Trust in a new member's signals        | Binary (in consortium or not)              | Relational, personalized, adapts over time             |
| Compromised member recovery            | Central operator kicks them (if available) | Member rotates their own key via guardian             |
| False-positive correction              | Email / phone call to affected merchant    | On-chain counter-signal visible to all                 |
| Data sovereignty                       | Central DB owner sees everything           | Each member holds their own node + data                |
| Audit trail for a specific decision    | Merchant's internal logs only              | Signed signal stream verifiable by anyone              |
| New member onboarding                  | Manual vetting + central key provisioning  | K-of-K snapshot + existing members declare trust       |

## What's in this folder

- [`README.md`](README.md) — this document
- [`implementation.md`](implementation.md) — Quidnug API calls & sample code
- [`threat-model.md`](threat-model.md) — attackers & mitigations

## Runnable POC

A working end-to-end demo lives at
[`examples/merchant-fraud-consortium/`](../../examples/merchant-fraud-consortium/):

- `fraud_weighting.py` — standalone weighted-aggregation
  math (no SDK dependency).
- `fraud_weighting_test.py` — pytest suite, 9 tests
  covering observer-relative correctness + decay.
- `demo.py` — end-to-end flow against a live Quidnug node
  exercising all four actors (bootstrapping merchants,
  asymmetric trust edges, emitting signals, observer-
  relative aggregate scoring).

Running:

```bash
cd deploy/compose && docker compose up -d   # start local node
cd examples/merchant-fraud-consortium
python demo.py
```

## Related

- [`../interbank-wire-authorization/`](../interbank-wire-authorization/) —
  similar consortium pattern; different threshold model (wires need
  internal bank quorum; fraud signals are single-party statements
  weighted per-consumer).
- [QDP-0005 Push Gossip](../../docs/design/0005-push-based-gossip.md)
- [QDP-0008 K-of-K Bootstrap](../../docs/design/0008-kofk-bootstrap.md)
