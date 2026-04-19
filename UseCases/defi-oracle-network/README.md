# DeFi Oracle Network

**FinTech · DeFi · Decentralized data feeds · Aggregation trust**

## The problem

DeFi protocols consume off-chain data: crypto prices, FX rates,
sports outcomes, weather for parametric insurance, real-world
asset data. The industry's oracle solutions (Chainlink, Pyth, API3,
Band) each make specific security trade-offs:

- **Single-operator oracles** (an exchange's own API) = total
  centralization.
- **Staked committee oracles** (Chainlink-style) = requires
  a native token and introduces the "stake-based slashing"
  economic model, which has its own failure modes (validator
  extraction, correlated failures during crashes).
- **Proof-of-stake signed feeds** (Pyth) = consumers trust the
  chain's validator set to have vetted signers.
- **First-price commit-reveal schemes** = increase latency, and
  are complex.

**The missing piece is subjectivity.** A DeFi lending protocol
with $500M TVL has different risk tolerance than a micro-
application with $5k. They should weigh oracle reporters
differently. Today they largely use the same feeds with the
same aggregation.

## Why Quidnug fits

Oracle feeds are **signed claims from identifiable signers**. A
consumer's acceptance of a feed should depend on the consumer's
trust in the signers. That's relational trust.

| Problem                                         | Quidnug primitive                                 |
|-------------------------------------------------|---------------------------------------------------|
| "Which reporters do I trust, and by how much?"  | Trust edges in `oracles.price-feeds.*`            |
| "Has each report been signed by the reporter?"  | ECDSA-signed events                                |
| "Has this report been replayed?"                | Monotonic anchor nonces per reporter               |
| "How do I aggregate many reports?"              | Consumer-side weighted by each reporter's trust    |
| "A reporter went rogue — how do I kick them?"   | Lower trust edge / guardian recovery on their side |
| "A new protocol wants to use these feeds"       | K-of-K bootstrap from trusted peers                |

## High-level architecture

```
                   Off-chain data sources
               ┌──────────┬──────────┬──────────┐
               │          │          │          │
          Exchange A  Exchange B  Exchange C  Reference
               │          │          │          │
               ▼          ▼          ▼          ▼
          ┌────────┐  ┌────────┐  ┌────────┐  ┌────────┐
          │Oracle-1│  │Oracle-2│  │Oracle-3│  │Oracle-4│  (reporter quids)
          │Quid    │  │Quid    │  │Quid    │  │Quid    │
          └────┬───┘  └────┬───┘  └────┬───┘  └────┬───┘
               │           │           │           │
               │ push      │ push      │ push      │ push
               │ gossip    │ gossip    │ gossip    │ gossip
               ▼           ▼           ▼           ▼
          ┌──────────────────────────────────────────────────┐
          │      Quidnug Network (consortium)                 │
          │                                                   │
          │  Events on "oracles.price-feeds.ethereum.btc-usd" │
          └──────────────────────────────────────────────────┘
               │           │           │           │
               │ pull      │ pull      │ pull      │ pull
               │           │           │           │
          ┌────▼───┐  ┌────▼───┐  ┌────▼───┐  ┌────▼───┐
          │Lending │  │DEX     │  │Insur-  │  │Stable- │
          │Protocol│  │        │  │ance    │  │coin    │
          │(cons.) │  │        │  │        │  │        │
          └────────┘  └────────┘  └────────┘  └────────┘
        
  Each consumer computes their OWN aggregate:
     weighted-sum(reports) weighted by their OWN trust
     in each reporter in oracles.price-feeds.*
```

## Data model

### Quids

- **Reporters**: one per oracle node operator
  (`oracle-chainlink-geth`, `oracle-pyth-eth-1`, etc.).
- **Consumers**: one per DeFi protocol
  (`lending-protocol-aave-fork-x`).
- **Feed identity** (optional): one per named feed
  (e.g., `btc-usd-oracle-feed` as a quid whose events are
  the feed), with the feed's operators as its guardian set.

### Domain

```
oracles.price-feeds.ethereum
├── oracles.price-feeds.ethereum.btc-usd
├── oracles.price-feeds.ethereum.eth-usd
├── oracles.price-feeds.ethereum.stablecoins
└── ...

oracles.weather-feeds.parametric-insurance
oracles.sports-feeds.prediction-markets
```

Domain scoping lets a consumer subscribe only to the feeds they
care about and weigh reporters differently per domain.

### A price report

```json
{
  "type": "EVENT",
  "subjectId": "btc-usd-oracle-feed",    /* the feed's quid */
  "subjectType": "QUID",
  "eventType": "oracle.price-report",
  "payload": {
    "reporter": "oracle-chainlink-geth",
    "symbol": "BTC-USD",
    "price": "67423.50",
    "timestamp": 1713400000,
    "source": "aggregate of Binance, Coinbase, Kraken",
    "confidence": 0.95,
    "roundId": 12345
  },
  "signature": "<reporter's ECDSA over canonical bytes>"
}
```

Each report is one event on the feed's stream. Events are
naturally ordered by the stream's monotonic sequence; consumers
process the most recent window.

## Consumer-side aggregation

A consumer's logic for determining the "effective price":

```go
type WeightedReport struct {
    Reporter     string
    Price        float64
    Trust        float64
    Age          time.Duration
}

func (c *Consumer) AggregatePrice(ctx context.Context, feedID string, window time.Duration) (float64, error) {
    events, err := c.quidnug.GetRecentEvents(feedID, "QUID", window)
    if err != nil { return 0, err }

    reports := make([]WeightedReport, 0, len(events))
    for _, ev := range events {
        if ev.EventType != "oracle.price-report" { continue }
        reporter := ev.Payload["reporter"].(string)
        price, _ := strconv.ParseFloat(ev.Payload["price"].(string), 64)

        // Relational trust in this reporter FROM THIS CONSUMER's view.
        trust, err := c.quidnug.GetTrust(ctx, c.selfQuid, reporter,
            "oracles.price-feeds.ethereum.btc-usd", nil)
        if err != nil { continue }

        reports = append(reports, WeightedReport{
            Reporter: reporter,
            Price:    price,
            Trust:    trust.TrustLevel,
            Age:      time.Since(time.Unix(int64(ev.Payload["timestamp"].(float64)), 0)),
        })
    }

    return c.weightedMedian(reports), nil
}

// weightedMedian picks the price at the middle of trust-weighted
// cumulative distribution. Resistant to outliers AND to low-
// trust reporters' input.
func (c *Consumer) weightedMedian(reports []WeightedReport) float64 {
    // Filter out reports older than some max age, below some min trust.
    filtered := filter(reports, func(r WeightedReport) bool {
        return r.Trust >= c.MinReporterTrust && r.Age < c.MaxReportAge
    })
    sort.Slice(filtered, func(i, j int) bool { return filtered[i].Price < filtered[j].Price })

    var totalWeight float64
    for _, r := range filtered { totalWeight += r.Trust }

    cumulative := 0.0
    for _, r := range filtered {
        cumulative += r.Trust
        if cumulative >= totalWeight/2 {
            return r.Price
        }
    }
    return 0
}
```

### Consumer variations

- **Large lending protocol**: requires ≥ 5 reports with trust
  ≥ 0.8 from reporters whose feeds haven't been countered recently.
- **Stablecoin**: extra conservative. Weighted median of trust
  ≥ 0.9 reporters. Also checks for staleness: if no reporter
  has updated in > 1 minute, circuit-break.
- **Small experimental DEX**: uses trust ≥ 0.5 minimum,
  tolerates one-off outliers.

**Same data feed, very different effective prices for different
consumers.** That's the whole point.

## Reporter failure handling

### Scenario: exchange API outage

Oracle-1 sources from Exchange A. Exchange A goes down; Oracle-1
stops reporting. Consumers that had Oracle-1 at high trust will
see their aggregate drop one report.

**Automatic recovery:** as long as enough other reporters are
active, consumers' aggregation still works.

**Trust-edge decay:** consumers might naturally apply a staleness
discount — a reporter silent for > 5 minutes has effective trust
0.

### Scenario: reporter compromised

Oracle-2's signing key is stolen. Attacker starts reporting
manipulated prices.

**Detection:** consumers can detect outliers (Oracle-2's price
diverges from the weighted-median of trusted reporters).

**Response path:**
1. Consumer lowers trust in Oracle-2 to 0 locally.
2. Oracle-2's operator is notified (out-of-band).
3. Operator initiates guardian recovery to rotate Oracle-2's
   key to a new HSM.
4. During the time-lock window, the compromised key still
   signs but its reports are low-weight from most consumers'
   perspectives — operator plus consumer reactions limit
   damage.
5. Post-rotation, old key is invalid; attacker is locked out.

### Scenario: reporter goes rogue (not compromised — malicious by choice)

Oracle-3 deliberately reports inflated prices.

**Response:** same as compromised, except the operator itself
is the attacker. Consumers simply set Oracle-3's trust to 0 and
look for new reporters to trust. No protocol-level "slashing"
needed — market-level "not trusted anymore" is sufficient.

## Key Quidnug features used

- **Relational trust per-consumer** — different DeFi protocols
  weight the same reporters differently.
- **Event streams** — feed = subject quid + append-only stream.
- **Push gossip (QDP-0005)** — feeds propagate within seconds.
- **Monotonic nonces (QDP-0001)** — replay protection on reports.
- **Guardian recovery (QDP-0002)** — reporter key recovery.
- **Domain hierarchy** — per-asset-pair subdomains.
- **K-of-K bootstrap (QDP-0008)** — new consumer joins by
  fetching trust snapshots from K existing consumers.
- **Lazy epoch probe (QDP-0007)** — detect stale reporter state
  when a consumer hasn't seen a reporter in a while.

## Scale estimates

Representative deployment:
- 50 reporters across major exchanges/aggregators
- 200 feeds (BTC-USD, ETH-USD, 100+ alt pairs, FX, commodities)
- 1 Hz update rate per feed = 200 events/second globally
- 500 consumers (protocols)

Workload:
- 17 M events/day
- Each consumer node sees all feeds they subscribed to (sub-MB
  bandwidth)
- Trust graph: 50 × 500 = 25,000 edges

Push gossip fanout latency: ~1 second per hop, ~3 hops max to
full coverage. Comfortable.

## Economic model

**Reporters' incentive to be honest:** reporter fees paid
in-band by protocols that use their feeds. Protocols with tighter
trust requirements pay premium rates to reporters whose signatures
carry high trust. Market-based reputation → market-based pricing.

This is *not* baked into the protocol. It's an application-layer
billing and contract concern. Quidnug's relational trust is the
substrate that makes the economic model work cleanly.

## Value delivered

| Dimension                              | Before                                     | With Quidnug                                          |
|----------------------------------------|--------------------------------------------|-------------------------------------------------------|
| Consumer customization                 | Single "Chainlink feed" for everyone        | Each consumer picks and weighs own reporters          |
| Cross-chain feed                       | Chain-specific oracle contracts            | Reporter quids are cross-chain; consumer bridge reads |
| Reporter compromise recovery           | Protocol-level reconfiguration, downtime   | Guardian recovery + market-level trust degrade         |
| Onboarding new reporters               | Token staking + governance vote            | Build reputation, earn trust edges organically         |
| Feed resilience                        | Tied to one oracle contract                | 200 feeds × N reporters = high redundancy              |
| Auditability of "why was this price?"  | Aggregator's internal algorithm            | Per-consumer weighting is public + reproducible       |

## What's in this folder

- [`README.md`](README.md) — this document
- [`implementation.md`](implementation.md) — concrete code
- [`threat-model.md`](threat-model.md) — security analysis

## Related

- [QDP-0005 Push Gossip](../../docs/design/0005-push-based-gossip.md) — propagation
- [QDP-0008 K-of-K Bootstrap](../../docs/design/0008-kofk-bootstrap.md) — new-consumer onboarding
- [`../merchant-fraud-consortium/`](../merchant-fraud-consortium/) — same
  pattern for fraud signals
