# QDP-0016: Abuse Prevention & Resource Limits

| Field      | Value                                                            |
|------------|------------------------------------------------------------------|
| Status     | Draft — design only                                              |
| Track      | Protocol + ops                                                   |
| Author     | The Quidnug Authors                                              |
| Created    | 2026-04-20                                                       |
| Requires   | QDP-0001 (nonce ledger), QDP-0013 (federation), QDP-0015 (moderation) |
| Implements | Multi-layer rate limiting + reputation-weighted resource allocation + anti-sybil |

## 1. Summary

The current node enforces a single per-minute rate limit at the
HTTP layer, applied by source IP. That's enough for a local dev
node and a handful of test users; it is not enough for a public
review network facing real traffic. A production-ready operator
needs:

- **Rate limits at multiple scales** — per IP, per quid, per
  epoch-key, per operator, per domain.
- **Reputation-weighted allowances** — a trusted quid gets more
  throughput than a brand-new quid.
- **Progressive slowdown for suspicious patterns** — rather than
  hard-cut at the limit, smoothly degrade responsiveness to
  abusive sources.
- **Challenge-response for uncertain traffic** — proof-of-work
  or human-verification challenge before an untrusted write.
- **Payload-size caps at every layer** — prevents
  memory-exhaustion attacks.
- **Sybil-resistance primitives** — bootstrap the trust graph
  against mass identity creation.

QDP-0016 specifies the primitives plus the default limits an
operator should pick before launch.

## 2. Goals and non-goals

**Goals:**

- A layered rate-limit system that scales from hobbyist
  single-node to multi-region consortium.
- Explicit knobs for each layer so operators can tune without
  touching code.
- Integration with QDP-0012 domain governance (governance can
  adjust limits) and QDP-0013 federation (federation trust
  affects allowances).
- Sybil-resistance strategies that don't require KYC.
- No new transaction types — everything is configurable state
  + runtime behavior.

**Non-goals:**

- CAPTCHAs for every write. Good for some paths, user-hostile
  for most. The protocol supports them; the policy is
  operator-choice.
- Blockchain-native economic rate limiting (staking, fees).
  Those are separate concerns; QDP-0016 is about resource
  limits not economic incentives.
- Machine-learning bot detection. Operators can run it, the
  protocol doesn't mandate or encode it.
- Global coordination on per-user limits. Limits are
  per-operator; federation can share signals but doesn't
  enforce.

## 3. The five rate-limit scales

### 3.1 Per IP (network perimeter)

First line of defense. Existing; QDP-0016 formalizes defaults.

```yaml
rate_limits:
    per_ip:
        requests_per_minute: 60        # anonymous read throughput
        writes_per_minute: 10          # anonymous write throughput
        burst: 15                      # short-term burst window
        block_duration_seconds: 300    # after exceeding, silent block
```

IP-based limits are necessary but insufficient — easy to bypass
via distributed sources. They exist primarily to stop incidental
misconfiguration (a buggy client in a loop) rather than
determined attackers.

### 3.2 Per quid (identity-scoped)

The meaningful layer for accountability. Every signed write
reveals its signer's quid; limits apply per-quid.

```yaml
rate_limits:
    per_quid:
        writes_per_minute: 10          # base rate for new quids
        writes_per_hour: 200
        writes_per_day: 2000
        graduate_at_age_days: 30       # after this many days, raise
        graduate_at_helpful_votes: 25  # or this many positive votes
        graduated_writes_per_minute: 60
        graduated_writes_per_hour: 1000
        graduated_writes_per_day: 20000
```

"Graduation" is reputation-based: a quid that's been around for
30+ days OR has accumulated 25+ helpful-vote endorsements from
trusted reviewers gets 6-10x the base allowance. This is the
core sybil resistance — fresh quids are throttled into
usefulness.

### 3.3 Per epoch-key (rotation-aware)

Prevents attackers from abusing key rotation to bypass per-quid
limits. When a quid rotates its key via anchor (QDP-0001), the
per-quid counter carries over; the new key inherits the old
key's used quota.

```yaml
rate_limits:
    per_epoch_key:
        inherit_prior_epoch: true      # default; counters don't reset on rotation
        cooldown_after_rotation_minutes: 5  # brief freeze to slow rotation-based evasion
```

### 3.4 Per operator (federation-aware)

For apps that use the "no-node participation" path (QDP-0014
§14) — all their users' quids flow through the operator's
single API-gateway access. A misbehaving app could swamp the
public network by relaying abuse.

```yaml
rate_limits:
    per_operator:
        writes_per_minute: 2000        # aggregated quota across operator's quids
        concurrent_pending_txs: 500    # max in flight
        max_payload_bytes_per_minute: 10485760  # 10 MB/minute payload through this operator
```

An operator exceeding the limit gets HTTP 429 with
`Retry-After` headers, plus an optional `X-Quidnug-Operator-Quota`
header showing current/max usage.

### 3.5 Per domain (target-scoped)

Protects specific domains from becoming DoS targets. A
viral product page shouldn't be able to overwhelm the
consortium.

```yaml
rate_limits:
    per_domain:
        writes_per_minute: 1000
        events_per_minute_per_subject: 30  # prevents event-stream flooding
        max_open_streams: 10000
```

### 3.6 Layer composition

A request passes only if it's under every applicable layer's
limit. In practice most reviews will easily clear all five;
the layers exist to catch pathological traffic, not typical
use.

## 4. Progressive slowdown

Hard rate limits (HTTP 429) create a binary failure mode: a
client either succeeds or fails. Progressive slowdown is more
forgiving and more effective at deterring abuse:

### 4.1 The model

For each actor (IP, quid, operator, etc.), maintain a usage
"budget" that refills over time. Each request deducts from the
budget.

- **Budget full (>50%):** responses normal-speed (median 20ms).
- **Budget 10-50%:** responses artificially delayed by
  50-200ms proportional to consumption.
- **Budget 0-10%:** delays 500-2000ms + occasional 429 injection.
- **Budget exhausted:** hard 429 until refill.

This makes an attack much more expensive (the slowdown
consumes their resources) without impacting legitimate users
who rarely hit the throttle.

### 4.2 Implementation sketch

Use a token-bucket per actor with configurable:

- **Capacity**: how large the bucket can get (burst size)
- **Refill rate**: tokens per second
- **Slow-start**: how many tokens new actors start with

```go
type rateBucket struct {
    capacity    int
    tokens      float64
    refillPerSec float64
    lastRefill  time.Time
}

func (b *rateBucket) take(n int) (ok bool, delay time.Duration) {
    b.refill()
    if b.tokens >= float64(n) {
        b.tokens -= float64(n)
        frac := b.tokens / float64(b.capacity)
        if frac > 0.5 { return true, 0 }
        if frac > 0.1 {
            // Slowdown region; delay proportional to depletion.
            return true, time.Duration((1-frac*2) * float64(200*time.Millisecond))
        }
        return true, time.Duration((0.1-frac) * float64(5*time.Second))
    }
    return false, 0
}
```

Buckets are kept in a bounded LRU to prevent the memory of
rate-tracking itself becoming a DoS vector.

## 5. Proof-of-work challenges for uncertain writes

For writes from quids with no reputation and no operator
attestation, the node can optionally require a proof-of-work
solution before accepting the tx. Opt-in per operator.

### 5.1 The challenge

On a write request from a low-reputation source:

1. Node responds with HTTP 429 + a `challenge` header
   containing a random 32-byte nonce and a difficulty target.
2. Client computes `sha256(challenge || extraNonce)` until
   finding an extraNonce whose hash has N leading zero bits.
3. Client re-submits with the `X-Quidnug-Challenge` header
   containing the challenge + the extraNonce.
4. Node verifies the PoW and admits the tx.

### 5.2 Tuning

Difficulty target auto-tunes based on legitimate-user latency
budget:

```yaml
proof_of_work:
    enabled: false                     # opt-in
    base_difficulty: 20                # bits; ~1 second at default CPU
    max_difficulty: 24
    apply_when_reputation_below: 0.2   # skip for trusted quids
```

Cost to operator: zero (only verification is cheap).
Cost to legitimate user: ~1 second of CPU per write from a
brand-new quid. Cost to attacker: ~N * 1 second for N sybil
identities, compounding with rate limits.

### 5.3 When to apply

Only on writes (POST/PUT/DELETE), only to low-reputation
sources, only when the operator has opted in. Operators with
low-volume / high-trust user bases leave it off. Operators
facing spam floods turn it on.

## 6. Payload-size caps

Per-layer caps prevent memory exhaustion + amplification attacks.

| Layer | Default cap | Notes |
|---|---|---|
| HTTP body | 1 MB | Already enforced by `MaxBodySizeBytes` |
| Per-tx body after unmarshal | 1 MB | New: catches JSON-bomb expansion |
| `EventTransaction.Payload` inline | 64 KB | Already enforced (`MaxPayloadSize`) |
| IPFS payload fetched by CID | 16 MB | New: caps what gets pinned per event |
| Per-domain aggregate payload/minute | 10 MB | New: prevents a single domain hogging bandwidth |
| Moderation annotation text | 2 KB | Per QDP-0015 |

All are overridable via `rate_limits.payload` YAML config.

## 7. Sybil resistance

The "create 10,000 fake quids" problem. Five layers:

### 7.1 Birth cost

Per §3.2, brand-new quids get baseline rate limits. Creating a
quid is cheap but **using** a quid is rate-limited. To get
leverage, the attacker has to spread traffic across thousands
of quids — each individually slow.

### 7.2 Reputation graduation

A quid graduates to higher limits only via demonstrated
trust. The trust graph's shape makes this expensive to fake:

- Direct trust requires an existing graduated quid to endorse
  the new one.
- Transitive trust decays rapidly (0.8 per hop).
- Sybil-ring endorsements (attacker-controlled quids endorsing
  each other) don't help because the nodes aren't themselves
  trusted by any honest quid.

### 7.3 Operator attestation

The strongest sybil resistance: an operator (via OIDC bridge
or manual review) explicitly attests that a quid corresponds
to a real verified person. Such quids graduate immediately.

### 7.4 Proof-of-work (§5)

Deters bulk creation by making fresh-quid writes computationally
expensive.

### 7.5 Community flagging

Per QRP-0001, the `FLAG` event type propagates dissatisfaction
from a reviewer's audience back to the reviewer's reputation.
A sybil ring's coordinated reviews get flagged by the real
trusted reviewers, and their reputation collapses.

### 7.6 Expected outcome

No single defense is perfect. Combining all five means:

- Mass identity creation is cheap but yields low-reputation
  quids.
- Low-reputation quids are throttled into uselessness.
- To graduate, quids need real social attestation, which
  adversaries can't fake at scale.
- Outliers (coordinated attacks) get flagged and demoted.

Result: a reviewer with no existing social connections to the
public network takes ~2-4 weeks of organic activity to earn
meaningful influence. That's slow enough to frustrate
adversaries, fast enough for genuine users.

## 8. Federation-aware behavior

Rate limits are **per operator**. Federation doesn't share the
rate-limit state directly, but federation can share:

- **Reputation signals** — a quid trusted by a federated network
  graduates faster on my network.
- **Abuse signals** — a federated network reporting abuse from
  a specific quid can lower my rate-limit allowance for that
  quid.
- **Coordinated responses** — during a multi-network attack,
  operators can push shared flagging state via the moderation
  import mechanism (QDP-0015 §6).

Config:

```yaml
federation_signals:
    trust_inheritance:
        enabled: true
        inheritance_decay: 0.6       # weight multiplier per federation hop
    abuse_inheritance:
        enabled: true
        sources:
            - url: "https://api.quidnug.com"
              pubkey: "<operator-pubkey>"
        cache_ttl: "5m"
```

## 9. Monitoring signals

Per QDP-0016, the node exports rich Prometheus metrics for
operators to alert on:

```
quidnug_ratelimit_decisions_total{outcome="allow|delay|deny", layer="ip|quid|operator|domain|epoch"}
quidnug_ratelimit_bucket_depletion_ratio{layer, actor_class}
quidnug_writes_per_quid_histogram{age_bucket}  # reveals bot-like patterns
quidnug_pow_challenges_issued_total
quidnug_pow_challenges_solved_total
quidnug_pow_time_to_solve_seconds
quidnug_sybil_flags_raised_total{source}
quidnug_new_quids_per_hour
quidnug_graduated_quids_per_hour
```

Recommended alerts:

- `quidnug_writes_per_quid_histogram` right-skew above baseline
  → likely bot activity.
- `quidnug_new_quids_per_hour` > 10x 24h-median → possible
  sybil wave.
- `quidnug_pow_challenges_solved_total{difficulty=max}` rising
  steadily → genuine or well-funded attacker; escalate.
- `quidnug_ratelimit_decisions_total{outcome="deny", layer="operator"}`
  spiking for a specific operator → app misbehavior; reach out.

## 10. Attack vectors (and what this QDP doesn't solve)

### 10.1 Distributed slow-drip attack

**Attack:** 10,000 IPs each sending 1 write per hour — under
every per-IP limit.

**Mitigation:** Per-quid / per-operator layers catch this. A
determined attacker can work around by using 10,000 quids,
each operated by fresh, unaged identities — but §7 (sybil
resistance) throttles them into uselessness.

Untreated risk: an attacker with 10,000 OIDC-bridged identities
could in theory bypass. That's the cost of reputation
bootstrapping — someone had to vouch for each one, and there's
a paper trail.

### 10.2 Timing attack

**Attack:** Attacker learns the limit exactly and pulses just
under it.

**Mitigation:** Progressive slowdown degrades response time
even under the limit, making perfect-pulse attacks slow. Not a
hard block but sufficient for deterrence.

### 10.3 Operator-level blame misattribution

**Attack:** Attacker uses a single operator's key to flood the
network, getting that operator rate-limited and preventing
legitimate users from the same operator.

**Mitigation:** This is the operator's problem to solve (their
key was compromised). Operators should monitor for this via
metrics and rotate keys or revoke compromised sub-operator
attestations.

### 10.4 Legitimate high-traffic source

**Attack:** A popular app / site hits the rate limit during a
legitimate traffic spike.

**Mitigation:** Per-operator quotas can be raised individually
via governance (QDP-0012) or a direct operator-to-operator
arrangement. The default quotas are chosen for a "typical"
app; high-volume apps should request a higher tier.

### 10.5 Rate-limit-state memory exhaustion

**Attack:** Attacker creates enough distinct IPs/quids that the
rate-limit tracking itself exhausts node memory.

**Mitigation:** Bounded LRU cache (default 100k entries per
layer). Overflow evicts oldest entries; evicted actors
effectively reset their budget. That's fine — rate limiting is
an optimization, not a correctness guarantee.

## 11. Implementation plan

### Phase 1: Multi-layer rate-limit infrastructure

- Refactor `internal/ratelimit/` into per-layer token buckets.
- Add `RateLimitConfig` struct + YAML binding with defaults
  matching §3.1-3.6.
- Middleware updates: inspect signing pubkey / operator id /
  domain from tx body to apply appropriate layers.

Effort: ~1.5 person-weeks.

### Phase 2: Progressive slowdown

- Replace hard 429 with token-bucket-depletion-aware delay.
- Observability for time-in-delay-path.

Effort: ~3-5 days.

### Phase 3: Proof-of-work challenge

- Challenge-issuance middleware.
- Verification middleware.
- Client SDK support (retry with challenge response).

Effort: ~1 person-week including client SDK.

### Phase 4: Reputation graduation

- Per-quid age + helpful-vote tracking (reuses QRP-0001
  helpfulness index).
- Graduation logic + metric surfacing.

Effort: ~1 person-week.

### Phase 5: Federation-aware abuse signals

- Consume abuse signals from federation trust sources
  (QDP-0013).
- Apply signals as additional rate-limit reductions for
  flagged quids.

Effort: ~5 days.

### Phase 6: Metrics + alert rules

- All Prometheus metrics per §9.
- `deploy/observability/prometheus-alerts.yml` updates with
  the five recommended alerts.

Effort: ~2 days.

## 12. Open questions

1. **Should operators be able to opt out of per-quid graduation
   and only enforce per-IP?** Probably yes for private networks;
   expose a `disable_reputation_graduation: true` flag.

2. **Should graduation reverse?** If a quid goes quiet for 6
   months, does it return to the new-quid rate limit? Lean
   toward no — graduation is earned, not maintained.

3. **Challenge-response difficulty floor.** In 2026 a 24-bit
   challenge takes ~1 second on a modern CPU. By 2030 it'll be
   ~0.1 second. Should difficulty adapt to real-time CPU
   benchmarks? Yes, via the `quidnug_pow_time_to_solve_seconds`
   histogram.

4. **Per-domain sub-operator quota.** When operators run many
   apps under one operator quid (via domain-tree delegation),
   should there be a per-(operator, domain) sub-quota rather
   than a single operator-level cap? Probably yes; add in a
   phase-7 follow-up.

5. **Fee-based rate-limit bypass.** Nodes could sell
   "premium" quota (pay X tokens → get N extra requests).
   Out of scope for this QDP; would require an economic /
   billing primitive.

## 13. Review status

Draft. Needs:

- Operator review of the default limits (§3). Probably too
  conservative for some use cases, too lax for others.
- Load testing of the progressive-slowdown algorithm under
  realistic traffic mixes.
- Security review of the proof-of-work challenge construction
  (avoiding length-extension, replay, etc.).

Implementation sequencing is flexible: Phase 1 is mandatory
for launch, Phases 2-6 can land incrementally.

## 14. References

- [Token bucket algorithm](https://en.wikipedia.org/wiki/Token_bucket)
- [Hashcash proof-of-work](http://www.hashcash.org/papers/hashcash.pdf)
- [QDP-0001 (Nonce ledger)](0001-global-nonce-ledger.md) —
  per-signer nonce state; rate-limit buckets attach here
- [QDP-0013 (Network federation)](0013-network-federation.md) —
  federation-aware abuse signals
- [QDP-0015 (Content moderation)](0015-content-moderation.md) —
  the complement to this QDP
- [QDP-0017 (Data subject rights, planned)](0017-data-subject-rights.md)
  — privacy counterpart
