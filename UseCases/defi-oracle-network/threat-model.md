# Threat Model: DeFi Oracle Network

## Assets

1. **Price accuracy** — DeFi protocols relying on a feed can
   be drained via manipulated prices.
2. **Reporter identity integrity** — a valid-looking signature
   that's actually from a compromised key enables false reports.
3. **Consumer protocol treasury** — ultimate target; an
   attacker manipulating a feed causes the protocol's logic to
   transfer funds to the attacker.

## Attackers

| Attacker                     | Capability                                         | Motivation                |
|------------------------------|----------------------------------------------------|---------------------------|
| Flash-loan arbitrage attacker | Can execute in a single tx, query oracles          | Profit per block          |
| Reporter compromise           | Valid reporter key                                | Market manipulation        |
| MEV bot operator              | Sees pending txs                                  | Sandwich oracle updates    |
| Malicious consortium member   | Operates own reporter                            | Reputation destruction     |
| External market manipulator   | Controls underlying source (e.g. a thin market)  | Manipulate reporter input  |

## Threats and mitigations

### T1. Compromised reporter reports manipulated prices

**Attack.** Attacker steals Oracle-X's signing key and submits
reports at $200 instead of $67,400 for BTC.

**Mitigations.**
- **Consumer-side outlier rejection** — weighted median across
  trusted reporters. One outlier from one reporter doesn't move
  the aggregate significantly.
- **Min-reporters requirement** — consumer doesn't trust an
  aggregate based on fewer than N reporters. Attacker has to
  compromise multiple reporters simultaneously.
- **Trust-based weighting** — compromised reporter's trust
  typically drops after first outlier (auto-de-weighting).
- **Emergency rotation** — operator rotates the key via
  guardian recovery. Attacker's signatures stop validating.

**Residual risk.** If attacker compromises the majority-by-trust
of reporters a consumer relies on, they can manipulate. Hence the
"diversify trust across many reporters" principle.

### T2. Reporter operator going rogue

**Attack.** A reporter's legitimate operator deliberately reports
manipulated prices. Same on-wire effect as compromise but no
"recovery" path (the operator is the attacker).

**Mitigation.**
- Consumer-level: reduce that reporter's trust to 0 locally.
  Other consumers make their own decisions.
- The Quidnug network doesn't require unanimous "kick" —
  bad actors naturally get routed around by consumers who
  observe the bad behavior.

**Residual risk.** Informed consumers adapt; uninformed ones
(not monitoring) might keep trusting. Operational discipline.

### T3. Correlated reporter compromises

**Attack.** Attacker coordinates the compromise of 3 or 4 of
the top-trust reporters at once to push a coordinated fake
price through.

**Mitigations.**
- **Reporter diversity** — consumers should explicitly spread
  trust across uncorrelated operators (different jurisdictions,
  different source feeds, different infrastructure vendors).
- **Cross-feed sanity** — if btc-usd reports deviate from
  eth-btc × eth-usd by > 1%, halt.
- **Circuit-breaker** — if price moves > X% in Y seconds,
  pause lending. This is app-layer but widely recommended.

**Residual risk.** A sufficiently resourced attacker who
compromises top reporters is a sovereign-scale threat.
Protocol can only do so much; protocol + external oracle
(e.g., centralized backup) hybrid models exist.

### T4. Replay attack

**Attack.** Attacker replays a valid old report from 10
seconds ago to try to pin the price.

**Mitigation.**
- **Consumer-side staleness check** (`MaxReportAge`) — old
  reports don't count toward aggregation.
- **Anchor nonces** — even if a consumer's aggregation window
  is wider than 10s, the nonce monotonicity check would flag
  a duplicate from the ledger.

**Residual risk.** None.

### T5. Report front-running

**Attack.** Observer sees a reporter's price update in
push-gossip layer before it's incorporated into a consumer
aggregate. Submits a front-running tx based on the new price.

**Mitigation.**
- **This is a DeFi architectural issue, not a Quidnug one.**
  Push gossip propagates publicly; MEV-resistant designs need
  commit-reveal or time-delay at the consumer contract layer.
- Quidnug doesn't make this worse or better than any other
  public oracle.

### T6. Feed identity takeover

**Attack.** Attacker compromises the feed identity (e.g.,
`btc-usd-eth-feed` quid) itself, and modifies its guardian set
to remove legitimate reporters and add attacker-controlled ones.

**Mitigation.**
- Feed identity's own guardian set (typically = consortium
  governance) gates set updates.
- Typical setup: feed guardians = full consortium quorum
  (e.g., 5-of-7 of the reporter operators themselves plus
  some industry observers).
- Subject-level `RequireGuardianRotation` flag can be set on
  high-value feeds so only guardian-quorum approved rotations
  work — no primary-key fast path.

### T7. Source manipulation (e.g., wash trading on a thin exchange)

**Attack.** Attacker manipulates the underlying data source
(a low-liquidity exchange) that Oracle-X aggregates from.
Oracle-X faithfully reports the manipulated price.

**Mitigation.**
- **Reporters should source from multiple exchanges** and
  compute volume-weighted prices, making thin-market
  manipulation expensive.
- **Consumer-side source diversity** — prefer reporters who
  aggregate from 3+ major exchanges.
- Reporter metadata declares sources; consumers can refuse
  reporters whose sources are too narrow.

**Residual risk.** Source manipulation is an orthogonal
problem (market-microstructure), not a protocol problem.

### T8. Denial of service on oracle feed

**Attack.** Flood the Quidnug network with junk traffic to
prevent reports from propagating.

**Mitigation.**
- **Push gossip rate limiting** — per-producer caps prevent
  any single party (including attackers) from monopolizing
  bandwidth.
- **Consumer fallback** — app-layer circuit-breaker when no
  recent reports.

### T9. New-consumer onboarding attack

**Attack.** Malicious bootstrap peer returns a snapshot with
a compromised trust-list that points the new consumer at
attacker-controlled reporters.

**Mitigation.**
- K-of-K bootstrap (QDP-0008) requires K peers to agree. Three
  malicious peers are the attack prerequisite — a much higher
  bar than one.
- Trust list is manually seeded from a known-good source
  (consortium website signed with multiple consortium members'
  keys, obtained out-of-band).

### T10. Smart-contract-level issues (out of scope)

- Reentrancy on the bridge contract
- Pausable / upgradeable contract misuse
- Flash-loan-against-oracle tx ordering

These are the DeFi protocol's contract-layer concerns.
Quidnug doesn't affect them.

## Not defended against

1. **Off-chain source truth.** If every exchange reports a
   wrong BTC-USD price (wouldn't happen but theoretically),
   every reporter would faithfully relay wrongness. Market-
   structure integrity is outside Quidnug's scope.

2. **Majority-of-trusted-reporters collusion.** If 60% of
   consumer's trusted reporters are colluding, they win.
   Mitigation is consumer-side: spread trust widely.

3. **Consumer's own misconfiguration.** If a consumer sets
   minTrust=0.1, they get what they asked for.

4. **Privacy of which consumer uses which feed.** Public by
   design — all queries go through the same nodes.

## Monitoring

Critical Prometheus metrics:
- `quidnug_events_total{eventType="oracle.price-report",reporter="..."}`
  — detect reporter going silent.
- `quidnug_gossip_rate_limited_total{producer=<reporter>}` —
  compromised reporter flooding.
- Application metric: price-variance across reporters per
  feed. Rising variance = market chaos or attack precursor.

## References

- [QDP-0005 Push Gossip](../../docs/design/0005-push-based-gossip.md)
- [QDP-0008 K-of-K Bootstrap](../../docs/design/0008-kofk-bootstrap.md)
- [`../merchant-fraud-consortium/threat-model.md`](../merchant-fraud-consortium/threat-model.md) — similar consortium threat model
