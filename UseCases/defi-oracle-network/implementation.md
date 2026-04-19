# Implementation: DeFi Oracle Network

Concrete steps to deploy a Quidnug-based oracle network.

## 0. Architecture topology

Four component roles:

1. **Reporter nodes** — one per oracle operator. Each has a
   quid, runs its own Quidnug node, and periodically submits
   signed price reports.
2. **Feed identities** — one quid per feed (BTC-USD, ETH-USD…).
   A feed's stream accumulates reports from many reporters. The
   feed's guardian set is the operators authorized to participate
   in that feed.
3. **Consumer nodes** — DeFi protocols. Run Quidnug client code
   (or full node) to read feeds.
4. **Cross-chain bridge** (out of scope for this doc) — a
   separate component that reads Quidnug feeds and writes
   canonical prices to EVM smart contracts.

## 1. Set up a reporter

```bash
# Reporter-side Quidnug node configuration
ENABLE_NONCE_LEDGER=true
ENABLE_PUSH_GOSSIP=true
ENABLE_LAZY_EPOCH_PROBE=true
SUPPORTED_DOMAINS=oracles.price-feeds.*,oracles.weather-feeds.*
SEED_NODES=[...]
./bin/quidnug
```

Create the reporter quid with a guardian set for recovery:

```bash
curl -X POST http://localhost:8080/api/identities -d '{
  "quidId": "oracle-operator-alpha",
  "name": "Oracle Alpha",
  "creator": "oracle-operator-alpha",
  "updateNonce": 1,
  "homeDomain": "oracles.price-feeds.ethereum",
  "attributes": {
    "operatorType": "independent",
    "jurisdictions": ["US", "CH"],
    "sourceExchanges": ["Binance", "Coinbase", "Kraken"]
  }
}'

# Install guardian set — operator's security team is the guardians
curl -X POST http://localhost:8080/api/v2/guardian/set-update -d '{...}'
```

## 2. Create a feed identity (one-time setup, done by consortium)

A feed like "BTC-USD on Ethereum" is its own quid. Consortium
governance (or an organizing entity) creates it:

```bash
curl -X POST http://localhost:8080/api/identities -d '{
  "quidId": "btc-usd-eth-feed",
  "name": "BTC-USD Ethereum Oracle Feed",
  "creator": "oracle-consortium",
  "updateNonce": 1,
  "homeDomain": "oracles.price-feeds.ethereum.btc-usd"
}'

# The feed's guardian set lists authorized reporter operators
# (weight=1 each; threshold=1 since a single trusted reporter
# submitting a report is sufficient to accept into the stream).
curl -X POST http://localhost:8080/api/v2/guardian/set-update -d '{
  "subjectQuid": "btc-usd-eth-feed",
  "newSet": {
    "guardians": [
      {"quid":"oracle-operator-alpha","weight":1,"epoch":0},
      {"quid":"oracle-operator-beta","weight":1,"epoch":0},
      {"quid":"oracle-operator-gamma","weight":1,"epoch":0},
      {"quid":"oracle-operator-delta","weight":1,"epoch":0},
      {"quid":"oracle-operator-epsilon","weight":1,"epoch":0}
    ],
    "threshold": 1,
    "recoveryDelay": 3600000000000
  },
  ...
}'
```

Alternatively, feeds can be open: any reporter can submit, and
consumers individually decide whose reports to trust. Simpler but
no protocol-level filter.

## 3. Reporter submits price reports

Reporter's local process pulls prices from its sources and
submits events every tick:

```go
package reporter

import (
    "context"
    "encoding/json"
    "net/http"
    "time"
)

type Reporter struct {
    quid         string
    feedID       string
    nodeAPI      string
    sources      []PriceSource
    anchorNonce  int64
}

func (r *Reporter) Tick(ctx context.Context) error {
    // 1. Collect from sources.
    prices, err := r.collectPrices(ctx)
    if err != nil { return err }

    // 2. Aggregate (reporter's own heuristic — median, VWAP, etc.)
    aggregated := r.aggregate(prices)

    // 3. Build and sign an event.
    r.anchorNonce++
    payload := map[string]interface{}{
        "reporter":    r.quid,
        "symbol":      "BTC-USD",
        "price":       formatPrice(aggregated.Value),
        "timestamp":   time.Now().Unix(),
        "source":      "median of " + strings.Join(r.sourceNames(), ","),
        "confidence":  aggregated.Confidence,
        "roundId":     aggregated.Round,
        "anchorNonce": r.anchorNonce,
    }
    event := map[string]interface{}{
        "type":        "EVENT",
        "subjectId":   r.feedID,
        "subjectType": "QUID",
        "eventType":   "oracle.price-report",
        "payload":     payload,
        "creator":     r.quid,
    }
    event["signature"] = r.signEvent(event)

    // 4. Submit.
    body, _ := json.Marshal(event)
    resp, err := http.Post(r.nodeAPI+"/api/v1/events", "application/json", bytes.NewReader(body))
    if err != nil { return err }
    defer resp.Body.Close()
    return nil
}
```

Reporter ticks once per second for BTC-USD (or whatever the feed
cadence is). Subscribers see the push gossip propagate within
seconds.

## 4. Consumer subscribes and aggregates

Consumer (a DeFi protocol) runs a Quidnug read-mostly node:

```go
package consumer

type Consumer struct {
    selfQuid       string
    nodeAPI        string
    feedID         string
    minTrust       float64       // e.g., 0.7
    maxReportAge   time.Duration // e.g., 30 * time.Second
    minReporters   int           // e.g., 3
}

func (c *Consumer) LatestPrice(ctx context.Context) (float64, error) {
    // 1. Fetch recent events in the feed.
    events, err := c.readEvents(ctx)
    if err != nil { return 0, err }

    // 2. For each report, fetch relational trust in the reporter.
    var reports []WeightedReport
    for _, ev := range events {
        if ev.EventType != "oracle.price-report" { continue }
        reporter := ev.Payload["reporter"].(string)

        trust, err := c.getTrust(ctx, c.selfQuid, reporter,
            "oracles.price-feeds.ethereum.btc-usd")
        if err != nil {
            continue // reporter unknown → skip
        }
        if trust.TrustLevel < c.minTrust {
            continue // not trusted enough
        }

        ts := time.Unix(int64(ev.Payload["timestamp"].(float64)), 0)
        if time.Since(ts) > c.maxReportAge {
            continue // stale
        }

        reports = append(reports, WeightedReport{
            Reporter: reporter,
            Price:    parsePrice(ev.Payload["price"].(string)),
            Trust:    trust.TrustLevel,
            Age:      time.Since(ts),
        })
    }

    if len(reports) < c.minReporters {
        return 0, ErrInsufficientReporters
    }

    // 3. Weighted median.
    return c.weightedMedian(reports), nil
}

func (c *Consumer) weightedMedian(reports []WeightedReport) float64 {
    sort.Slice(reports, func(i, j int) bool { return reports[i].Price < reports[j].Price })
    var total float64
    for _, r := range reports { total += r.Trust }
    cum := 0.0
    for _, r := range reports {
        cum += r.Trust
        if cum >= total/2 {
            return r.Price
        }
    }
    return 0
}
```

Consumer can (and should) configure additional sanity checks:
- Staleness circuit-breaker ("no update in 60s → halt lending")
- Cross-feed sanity ("if btc-usd and eth-usd drift
  differently from expected correlation, pause")

## 5. Consumer trust declarations

Consumer initially declares trust in a set of known-good reporters:

```bash
# For each reporter the consumer vouches for
curl -X POST http://localhost:8080/api/trust -d '{
  "truster":"lending-protocol-aave-fork",
  "trustee":"oracle-operator-alpha",
  "trustLevel":0.95,
  "domain":"oracles.price-feeds.ethereum.btc-usd",
  "nonce":1,
  "validUntil":<now + 90d>
}'
```

Consumer can also leverage transitive trust. If Lending
Protocol doesn't know Reporter Gamma directly, but Reporter
Alpha (whom Lending trusts) declared trust in Gamma, Lending
gets a transitive view.

## 6. Cross-chain bridge (brief sketch)

A bridge component reads Quidnug feeds and writes to EVM:

```go
func (b *Bridge) Tick() {
    for _, feed := range b.feeds {
        price, err := b.consumer.LatestPrice(context.Background())
        if err != nil { continue }

        // Submit to on-chain price-feed contract.
        tx, err := b.contract.UpdatePrice(context.Background(),
            feed.OnChainID, price)
        ...
    }
}
```

This is a trusted bridge — the bridge operator is a single point
for this specific on-chain contract. DeFi protocols that can't
tolerate bridge risk would run the bridge themselves.

## 7. Reporter compromise response

Oracle Alpha's key is stolen. Incident response:

### Alpha's side
```bash
# Guardian recovery to rotate to a new key
curl -X POST http://localhost:8080/api/v2/guardian/recovery/init -d '{
  "subjectQuid":"oracle-operator-alpha",
  "fromEpoch":0,
  "toEpoch":1,
  "newPublicKey":"<hex>",
  ...
}'
# Time-lock elapses → commit → Alpha's epoch is now 1
```

### Consumer-side (automatic)
Consumer's next `LatestPrice()` call uses Quidnug's epoch-
validated signatures. Reports signed by Alpha's epoch-0 key
after the rotation will fail verification (they'd need to
be signed at epoch 1). Attacker's signatures are invalid;
real Alpha's new-epoch signatures are valid.

## 8. Bootstrap a new consumer

```go
cfg := core.DefaultBootstrapConfig()
cfg.Quorum = 3

// Seed trust list — known-good existing consumers who can
// attest to the reporter set. The new consumer might get this
// list from the oracle consortium's website or from a trade
// association.
node.SeedBootstrapTrustList([]core.BootstrapTrustEntry{
    {Quid:"oracle-operator-alpha",PublicKey:"<hex>"},
    {Quid:"oracle-operator-beta", PublicKey:"<hex>"},
    {Quid:"oracle-operator-gamma",PublicKey:"<hex>"},
}, 3)

sess, err := node.BootstrapFromPeers(ctx,
    "oracles.price-feeds.ethereum.btc-usd", cfg)
if err != nil || sess.State != core.BootstrapQuorumMet {
    log.Fatal("bootstrap failed: ", err)
}
node.ApplyBootstrapSnapshot(cfg.ShadowBlocks)

// After bootstrap, consumer declares its own trust edges
// (initially to the same reporters it bootstrapped from).
```

## 9. Monitoring

Prometheus metrics to alert on:
- `quidnug_events_total{type="oracle.price-report"}` per reporter —
  detect a reporter going silent.
- `quidnug_gossip_rate_limited_total{producer="..."}` — reporter
  exceeding rate; potentially compromised.
- `quidnug_probe_failure_total` for high-value reporters —
  connectivity issue may hide a rotation.

App-layer alerts:
- "Aggregate price diverges from external reference by > 1% for
  > 30s" → circuit-break.
- "Fewer than N reporters active in the last minute" → halt
  price updates.
- "Outlier report from a high-trust reporter" → alert; possible
  compromise.

## 10. Testing

```go
func TestOracle_WeightedAggregation(t *testing.T) {
    // 5 reporters with varying trust from consumer's view
    // Submit reports at slightly different prices
    // Expected: weighted median closer to higher-trust reporters' prices
}

func TestOracle_OutlierRejection(t *testing.T) {
    // 5 trusted reporters report $67,000-$67,500
    // 1 reporter reports $200 (attack / bug)
    // Verify: weighted median still in trusted range
}

func TestOracle_ReporterKeyCompromise(t *testing.T) {
    // Reporter's key rotated via guardian recovery
    // Post-rotation, old-key signatures are rejected
}

func TestOracle_StalenessDetection(t *testing.T) {
    // No new reports for >30s → consumer circuit-break
}
```

## Where to go next

- [`threat-model.md`](threat-model.md)
- [`../ai-model-provenance/`](../ai-model-provenance/) — similar
  pattern for AI model provenance
