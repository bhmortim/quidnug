# DeFi oracle network, POC demo

Runnable proof-of-concept for the
[`UseCases/defi-oracle-network/`](../../UseCases/defi-oracle-network/)
use case. Demonstrates consumer-side weighted aggregation of
price reports with relational trust, a robust (MAD-based)
outlier filter, and per-consumer subjectivity.

## What this POC proves

Four reporter oracles emit signed price reports onto a shared
feed's event stream. Two DeFi consumer protocols (conservative
+ permissive) independently aggregate those reports with their
own trust graphs. Key claims:

1. **No central aggregator.** Each reporter's price is a signed
   event on the feed's stream. Consumers pull the stream and
   compute their own aggregate locally.
2. **Relational subjectivity.** The same reports produce
   different effective prices across consumers whose trust
   graphs differ: a mainstream-biased protocol lands on the
   mainstream cluster; a contrarian-biased protocol lands on
   the fringe cluster.
3. **Freshness + trust floor + minimum-reporter count** are all
   policy knobs on the consumer side.
4. **MAD-based outlier rejection** is robust: a single wild
   report gets excluded without being pulled into the central
   tendency calculation first.
5. **Compromised reporter handling.** Lowering a trust edge
   below the floor (or to 0) excludes that reporter from all
   future aggregations. No token slashing, no protocol-level
   eviction ceremony.

## What's in this folder

| File | Purpose |
|---|---|
| `oracle_aggregation.py` | Pure aggregation logic. `PriceReport`, `AggregationPolicy`, `aggregate_price`, stream extractor. Weighted median + MAD-based outlier filter. |
| `oracle_aggregation_test.py` | 10 pytest cases: median, outlier exclusion, staleness filter, trust floor, insufficient reporters, subjectivity, dedup, symbol filter. |
| `demo.py` | End-to-end runnable against a live node. Five steps: register, trust graphs, consensus reports, outlier scenario with three policy variants. |

## Running

```bash
# 1. Start a local node.
cd deploy/compose && docker compose up -d

# 2. Install Python SDK.
cd clients/python && pip install -e .

# 3. Run the demo.
cd examples/defi-oracle-network
python demo.py
```

## Testing without a live node

```bash
cd examples/defi-oracle-network
python -m pytest oracle_aggregation_test.py -v
```

## QDP catalog audit

| Feature | Purpose | Status |
|---|---|---|
| `IDENTITY` tx | Register reporters, consumers, feed | v1.0 |
| `TRUST` tx | Per-consumer weighting of each reporter | v1.0 |
| `EVENT` tx streams | Price reports on the feed's stream | v1.0 |
| QDP-0001 nonce ledger | Monotonic nonces prevent replay of old reports | v1.0 |
| QDP-0005 push gossip | Low-latency report propagation | v1.0 |
| QDP-0007 lazy epoch probe | Detect stale-key reporters across network boundaries | v1.0 (not exercised) |
| QDP-0008 K-of-K bootstrap | New consumer protocol joins the network | v1.0 (not exercised) |
| QDP-0019 decay | Stale reporter trust fades without continuous trust-edge refresh | Phase 1 landed; optional |

No protocol gaps.

## What a production deployment would add

- **Round-ID coordination.** The POC treats each price report
  independently. Production would have reporters coordinate on
  a round-id so consumers know which reports belong to which
  round, and aggregate a complete round rather than a rolling
  window.
- **Hardware-root trust.** Reporters whose nodes run in TEEs
  (AWS Nitro, Intel SGX) attest their enclave measurements
  on-chain; consumers can gate on that attestation.
- **Confidence-weighted aggregation.** Each report carries a
  confidence value; production would weight by trust *times*
  confidence.
- **Explicit slashing-free eviction.** When a reporter's
  long-term outlier rate crosses a threshold, consumers'
  governance quorums trim the trust edge. A reputation stream
  on each reporter quid captures this history.
- **Cross-chain feed publishing.** A consumer-side relay
  forwards the aggregated price into an on-chain smart contract
  with the aggregate signed by the consumer quid.

## Related

- Use case: [`UseCases/defi-oracle-network/`](../../UseCases/defi-oracle-network/)
- Related POC: [`examples/merchant-fraud-consortium/`](../merchant-fraud-consortium/)
  uses the same trust-weighted aggregation in a different
  domain (fraud signals rather than prices)
- Related POC: [`examples/credential-verification-network/`](../credential-verification-network/)
  is the same observer-relative verdict pattern for credentials
- Protocol: [QDP-0001 Nonce Ledger](../../docs/design/0001-global-nonce-ledger.md)
