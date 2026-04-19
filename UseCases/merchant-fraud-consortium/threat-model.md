# Threat Model: Merchant Fraud Consortium

## Assets

1. **Signal accuracy** — the consortium's collective ability to
   detect fraud depends on signals being truthful.
2. **Member reputation** — a member wrongly accused of a false
   positive loses commercial value.
3. **Cardholder privacy** — the underlying PAN data is not
   on-chain, but patterns of flagging are.
4. **Cross-member trust graph** — who trusts whom is market-
   sensitive information.

## Attackers

| Attacker                | Capability                                        | Goal                                 |
|-------------------------|--------------------------------------------------|--------------------------------------|
| Compromised member key   | Valid signing key of one member                  | Flood false signals / reverse reals   |
| Malicious joiner         | Legitimate new member, bad intent                | Poison the network                    |
| Competitor               | Legitimate member, mixed motives                 | Hurt a competitor's reputation        |
| Fraudster                | No consortium membership                        | Manipulate signal network             |
| Central authority demand | Law enforcement / regulator                      | Access member data                    |

## Threats

### T1. Compromised member emits false signals

**Attack.** Attacker gains control of one member's signing key
and emits thousands of fake signals flagging legitimate cards.

**Mitigation.**
- **Relational trust** — other members weigh this reporter's
  signals through their own lens. The more signals they emit,
  the more are countered by trusted peers, the lower their
  effective weight becomes.
- **Push gossip rate limit** (QDP-0005 §7) — per-producer token
  bucket means the attacker can only emit so many signals per
  minute before their forwarded traffic is rate-limited.
- **Counter-signal cascade** — trusted peers seeing obvious
  false positives emit counters, which degrade the attacker's
  effective trust.
- **Emergency rotation** — member's security team rotates their
  signing key via guardian recovery. Old key becomes invalid
  within the delay window.

**Residual risk.** Window of attack between compromise and
detection. Mitigated by monitoring (signal-rate anomalies,
counter-signal-volume alerts).

### T2. Malicious new member

**Attack.** Attacker establishes a shell merchant, gets
minimal vetting, joins the consortium, and emits malicious
signals.

**Mitigation.**
- **Starting trust is low** — new members typically get
  trust 0.1–0.3 from existing members. Their signals have
  low effective weight until they build reputation.
- **Transitive trust degrades** — if the malicious member can't
  get a trusted peer to endorse them, their signals stay
  low-weight.
- **Any member who vouched can revoke** — trust edges are
  time-limited; `validUntil` enforces periodic re-affirmation.

**Residual risk.** If the malicious actor successfully
compromises or social-engineers several existing high-trust
members into vouching, they can accumulate reputation. This
is a long-term con, detectable by auditing trust-graph
changes.

### T3. False-positive weaponization

**Attack.** A legitimate but aggressive fraud team at Merchant
A regularly emits signals that turn out to be false positives
affecting Merchant B's customer base.

**Mitigation.**
- **Per-consumer reputation tracking** — each member can
  internally compute "how often does Acme's signals get
  countered by my trusted peers?" and lower their local trust
  in Acme accordingly.
- **Public counter-signal trail** — A's false positives
  accumulate visibly in A's signal history.

**Residual risk.** Not a cryptographic problem; a market-
incentive one. The transparency makes it manageable.

### T4. Signal-flow correlation attack

**Attack.** Observer of gossip traffic tries to correlate
which members are sharing signals with which others, inferring
commercial relationships.

**Mitigation.**
- **Push gossip fanout** means every member receives every
  signal, not just "interested" subsets — no pairwise
  correlation from gossip patterns.
- Content-level correlation (who emits signals about which
  cards) is inherent to the domain; consortium membership is
  public by design.
- **Sub-domains** let members scope what they emit publicly
  (vertical-specific subdomains).

**Residual risk.** An observer with gossip-level network view
can see who emits and when. If that's a concern, Tor-like
anonymizing relays are future work beyond Quidnug.

### T5. Replay attack

**Attack.** Attacker captures a legit signal from Acme and
re-submits it to have signals double-counted.

**Mitigation.**
- **Dedup-first processing** (QDP-0005 §6.5) — message ID
  dedup rejects replays in O(1) before even checking
  signatures.
- **Per-signer anchor nonce** (QDP-0001) — each signal is
  bound to an anchor-nonce; a replay has the same nonce,
  immediately rejected.

**Residual risk.** None.

### T6. Sybil — one person, many "members"

**Attack.** Attacker registers 50 shell merchants, each with
a quid, and has them mutually endorse each other to build
fake-trust graph.

**Mitigation.**
- **Out-of-band vetting** — existing members don't issue
  trust edges to shell entities they don't know. Sybils can
  endorse each other all they want; without an edge from a
  real member, they remain unreachable in the trust graph.
- **Trust edges are public** — a mutual-endorsement ring is
  visually obvious in the graph.
- **Periodic trust expiration** (`validUntil`) forces
  re-attestation, raising the ongoing cost of maintaining
  sybils.

**Residual risk.** If a single legitimate member is compromised
and starts endorsing sybils, the ring gets a foothold. See T1.

### T7. Legal / regulatory subpoena

**Attack.** Government serves a subpoena on the central
operator.

**Mitigation.**
- **There is no central operator.** Each member holds their
  own data. Subpoenas go to individual members, who can
  decide with counsel.

**Residual risk.** Individual members can still be compelled.
But the consortium isn't a single target.

### T8. Denial of service on one member

**Attack.** Flood one member's node with traffic to prevent
them from emitting / receiving signals.

**Mitigation.**
- IP-level rate limiting on HTTP surface.
- Push gossip is resilient to one-peer outage (messages
  propagate through other peers).
- Member's signal consumption is not latency-sensitive
  (seconds to minutes is fine).

**Residual risk.** Standard DoS mitigations (CDN, WAF) apply.

### T9. Data poisoning via severity inflation

**Attack.** Attacker submits every signal with severity=1.0
to maximize disruption.

**Mitigation.**
- **Consumer-side normalization** — consumers don't have to
  trust reporter-declared severity. Severity × reporter-trust
  is the effective weight.
- **Counter-signals** include a rebuttal severity, so a
  counter can match and neutralize.

**Residual risk.** Mostly structural — not a cryptographic
problem.

### T10. Consortium governance capture

**Attack.** A coalition of members passes a fork-block that
changes the signal-validity rules in their favor (e.g.,
lowering the counter-signal weight).

**Mitigation.**
- **2/3 quorum** required for fork-block transactions (QDP-0009
  §7). Capturing 2/3 of the validator set is itself a massive
  Sybil attack.
- **MinForkNoticeBlocks = 1440** ~= 24h. Every operator sees
  the change 24h before activation. Smart operators halt and
  investigate suspicious forks.
- **Members can operator-override** their node's flag
  settings, refusing to honor a controversial fork locally.

**Residual risk.** Majority-validator compromise is the
prerequisite.

## Not defended against

1. **Raw PII leaks** — the protocol stores hashes and
   patterns, but if a member wraps a full PAN in an event
   payload, it's on-chain. This is a member-application-layer
   responsibility.

2. **Collusion between multiple members** — the protocol
   assumes each member is their own principal. Explicit
   collusion is out of scope.

3. **Gaming the trust algorithm** — the BFS-pathing with max
   aggregation can be gamed by carefully structuring
   endorsements. Mitigations are part of the trust-algorithm
   design, not this use case.

## Monitoring

Critical Prometheus metrics:
- `quidnug_gossip_rate_limited_total{producer=...}` — a
  member being rate-limited may be under attack or may be
  compromised.
- `quidnug_nonce_replay_rejections_total` — rising replays
  indicate replay attempts.
- Application-layer: counter-signal ratio per reporter, FP
  rate, trust edge churn.

## Operator playbook

1. **Unusual signal rate from a member.** Emit an alert to
   that member's security team. They have options:
   - Confirm expected burst (nothing to do).
   - Rotate their key via guardian recovery (emergency path).
2. **Counter-signal rate spiking.** One reporter's signals are
   being contested. Reduce local trust in that reporter.
3. **Suspicious fork-block transaction.** Review signatures,
   communicate with other members, use operator-override if
   needed.

## References

- [QDP-0005 Push Gossip §5 Threat Model](../../docs/design/0005-push-based-gossip.md)
- [QDP-0008 Bootstrap §5 Threat Model](../../docs/design/0008-kofk-bootstrap.md)
