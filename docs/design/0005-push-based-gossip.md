# QDP-0005: Push-Based Gossip for Anchors and Fingerprints (H1)

| Field         | Value                                                |
|---------------|------------------------------------------------------|
| Status        | Draft                                                |
| Track         | Protocol                                             |
| Author        | The Quidnug Authors                                  |
| Created       | 2026-04-18                                           |
| Supersedes    | —                                                    |
| Requires      | QDP-0003 (cross-domain nonce scoping, landed)        |
| Implements    | Phase H1 of QDP-0004 roadmap                         |
| Target        | v2.3                                                 |

## 1. Summary

QDP-0003 shipped pull-based cross-domain gossip: peers query
`GET /api/v2/domain-fingerprints/{domain}/latest` and operators
submit anchors via `POST /api/v2/anchor-gossip`. This works for
small, tight clusters but has two structural problems:

1. **No rotation delivery guarantee.** A compromised-key rotation
   sealed in domain A does not propagate to domain B until some
   node in B pulls. Nothing forces a pull.
2. **No fan-out scaling.** Producers cannot address the set of
   nodes that actually care about a specific signer without each
   one polling on its own schedule.

This document specifies push-based gossip: the producing node fans
out new anchors and fingerprints over the existing
`runDomainGossip` HTTP-push channel, with deduplication, TTL
propagation, and producer-keyed rate limiting.

## 2. Background — what already exists

Today's gossip path (from `internal/core/network.go`,
`anchor_gossip.go`, `domain_fingerprint.go`):

| Mechanism                 | Transport                   | Dedup                        | Status               |
|---------------------------|-----------------------------|------------------------------|----------------------|
| `DomainGossip` (domain availability) | HTTP POST `/api/v1/gossip/domains` | node `GossipSeen`, 30 min | **Live** (push)       |
| `AnchorGossipMessage`     | HTTP POST `/api/v2/anchor-gossip` | ledger `seenGossipMessages`, 14 d | **Live** (pull-submit)|
| `DomainFingerprint` (latest) | HTTP GET `/api/v2/domain-fingerprints/{d}/latest` | n/a — single value | **Live** (pull)       |

The first row already has push semantics: a node actively POSTs
its domain set to every known node, with a hop-limited TTL and
message-ID dedup. We extend that pattern to the other two.

## 3. Problem statement

A rotation arrives in domain A. Domain B has state about the
rotating signer (their key is in B's `signerKeys`) but has not
polled recently. Two failure modes:

- **Silent staleness.** B continues to accept signatures under the
  rotated-out key until its next pull. For a compromise-motivated
  rotation (§ QDP-0002 threat model), this is a window the
  attacker controls.
- **Thundering herd on a schedule.** If every node in B pulls on a
  fixed cadence, the producing node in A takes a synchronized
  burst of requests at each cadence tick — wasteful and spiky.

Push gossip closes both: the producer emits one message, the
network fans it out in `O(log N)` hops, every interested node
learns within a small multiple of the base gossip interval.

## 4. Goals and non-goals

**Goals.**

- **G1.** New `AnchorGossipMessage` and `DomainFingerprint`
  payloads propagate from producing domain to every node with
  state about the affected signer/domain within
  `3 × DomainGossipInterval` (≤ 6 min at defaults) under
  best-case topology.
- **G2.** Propagation is authenticated end-to-end: only the
  producer's signature matters; intermediate hops cannot forge.
- **G3.** Cost per node is bounded: per-producer and
  per-signer rate limits prevent a compromised validator from
  flooding.
- **G4.** No new cryptographic primitives. Uses the existing
  ECDSA P-256 + signature envelope from QDP-0003.
- **G5.** No schema breakage. Extends the existing message types
  with a `ForwardingMetadata` envelope; old clients that can't
  parse the envelope still validate the wrapped payload.
- **G6.** Dedup costs are `O(1)` per message; no quadratic
  tracking structures.

**Non-goals.**

- **NG1.** Replacing HTTP with QUIC / gRPC-streams. Transport
  overhaul is explicitly out of scope. All pushes stay on the
  existing HTTP POST envelope.
- **NG2.** Pub/sub brokers. No central router.
- **NG3.** Signed forwarding (each hop cryptographically
  attesting to the forward). The producer signature is
  sufficient; hop attestation is attack-surface without benefit.
- **NG4.** Delivery guarantees beyond "best-effort within TTL."
  Pull remains available as the authoritative fallback for
  nodes that missed a push.

## 5. Threat model

| Threat                                         | Mitigation                                                                                               |
|------------------------------------------------|----------------------------------------------------------------------------------------------------------|
| Forged anchor from unknown producer             | Receivers drop gossip whose `GossipProducerQuid` key is not in their ledger. Unknown producers don't forward. |
| Amplification DDoS via high-TTL forging        | TTL capped at `DomainGossipTTL` (default 3) **client-side at receipt**, not just on send. Any message arriving with TTL > cap is clamped. |
| Replay of a valid old message                  | Existing `seenGossipMessages` dedup (14 d) kills replays deterministically.                              |
| Flood from a compromised validator             | Per-producer rate limit: N messages per window, enforced at receipt. Excess is silently dropped and metric-logged. |
| Poisoning via inconsistent fingerprints        | Fingerprint monotonicity check rejects `BlockHeight < stored`. A signer can only ever raise the stored fingerprint, not lower it. |
| Gossip-loop between two peers                  | Dedup kills the loop at the first repeat; TTL decrement kills it structurally on the second hop.        |
| Producer impersonation via MessageID collision | `MessageID = sha256(Domain ‖ BlockHash ‖ AnchorIndex ‖ Producer ‖ Timestamp)` — collision requires hash break. |

## 6. Protocol

### 6.1 Wire format

Two new gossip message types, both wrapping the existing v2
payloads. Each wraps its payload rather than inlining to preserve
signability:

```go
// Package internal/core

// AnchorPushMessage is the push-gossip envelope around
// AnchorGossipMessage. The payload is the exact same bytes that
// POST /api/v2/anchor-gossip accepts, so validation code is
// shared.
type AnchorPushMessage struct {
    SchemaVersion int                   `json:"schemaVersion"` // 1
    Payload       AnchorGossipMessage   `json:"payload"`
    TTL           int                   `json:"ttl"`           // hops remaining
    HopCount      int                   `json:"hopCount"`      // hops taken
    ForwardedBy   string                `json:"forwardedBy"`   // immediate sender node ID (advisory only)
}

// FingerprintPushMessage is the push-gossip envelope around a
// DomainFingerprint update.
type FingerprintPushMessage struct {
    SchemaVersion int                `json:"schemaVersion"` // 1
    Payload       DomainFingerprint  `json:"payload"`
    TTL           int                `json:"ttl"`
    HopCount      int                `json:"hopCount"`
    ForwardedBy   string             `json:"forwardedBy"`
}
```

**Only the payload is signed.** Envelope fields (`TTL`,
`HopCount`, `ForwardedBy`) are mutated per hop and explicitly
excluded from the signature coverage. This is the same pattern
the existing `DomainGossip` uses.

### 6.2 New HTTP endpoints

```
POST /api/v2/gossip/push-anchor        (receives AnchorPushMessage)
POST /api/v2/gossip/push-fingerprint   (receives FingerprintPushMessage)
```

Response semantics: `202 Accepted` on new, `200 OK` on dedup
(idempotent retries), `400 Bad Request` on schema failure,
`409 Conflict` on payload validation failure (the specific
validation error is surfaced in the body so operators can debug).

### 6.3 Subscription semantics

A node "subscribes" implicitly:

- **For anchor push:** the receiver's ledger contains
  `signerKeys[producerQuid]` (any epoch). Producer-less nodes
  drop the message.
- **For fingerprint push:** the receiver has at least one block
  in `OriginDomain` OR has `latestFingerprints[OriginDomain]`
  set (i.e., it has previously seen fingerprint activity for
  that domain).

No explicit subscription registration. This matters because:

1. It avoids state accumulation for subscriptions — the
   interest set is derivable from existing ledger state.
2. It prevents a discovery leak: an attacker cannot enumerate
   which nodes care about which signers by probing a
   subscription list.

### 6.4 Fan-out algorithm

On a node producing a new anchor or fingerprint:

```
1. Compute payload (anchor: seal block + sign; fingerprint: sign).
2. Build AnchorPushMessage / FingerprintPushMessage with
   TTL = DomainGossipTTL (default 3), HopCount = 0.
3. For each peer in KnownNodes (deterministic order):
     POST to peer's /api/v2/gossip/push-{anchor|fingerprint}
     Fire-and-forget (like existing DomainGossip).
4. Mark local MessageID as seen so our own forwards don't loop.
```

On a node receiving a push (middleware order matters — see §6.5):

```
1. Parse envelope. If schema unknown → reject.
2. Clamp TTL to DomainGossipTTL (defense against forged TTL).
3. Check dedup: if seenGossipMessages[payload.MessageID] →
   return 200 OK, DO NOT validate, DO NOT forward.
4. Check subscription match (§6.3). If no match → return 202
   (so forwarder isn't blamed), DO NOT validate, DO NOT forward.
5. Validate payload (existing validation chain: schema →
   producer key lookup → signature → monotonicity / anchor
   integrity).
6. Apply payload (existing apply path: ApplyAnchorFromGossip or
   SetLatestDomainFingerprint).
7. Record MessageID in seenGossipMessages.
8. Rate-limit check: if producer has exceeded cap in window →
   STOP (do not forward).
9. If TTL > 1 and HopCount < DomainGossipTTL:
     Decrement TTL, increment HopCount, set ForwardedBy to self.
     For each peer in KnownNodes EXCEPT the one that sent this:
       POST forward.
10. Return 202 Accepted.
```

### 6.5 Dedup-before-validate ordering

Dedup MUST run before signature validation. This is the same
lesson as QDP-0003 §8.3: signature verification is 20–50x more
expensive than a map lookup, and a flood of duplicates is the
most likely DoS vector against a push-gossip system.

Step 4 (subscription check) also runs before validation because
it's a simple map lookup; no point verifying a signature for a
signer we don't care about.

## 7. Rate limiting

Two caps, both enforced at receive time:

```go
const (
    GossipProducerRateWindow = 60 * time.Second
    GossipProducerRateMax    = 30   // messages per window per producer
    GossipSignerRateWindow   = 60 * time.Second
    GossipSignerRateMax      = 10   // messages per window per affected signer
)
```

Rate-limit state is per-node, kept in a dedicated
`producerRateLimiter map[string]*tokenBucket` structure mutated
under a new `gossipRateMutex`. Buckets are evicted on LRU basis
with a cap of `10 × expected_active_producers` to bound memory.

Rate-limit decisions are "drop + metric" rather than
"reject + blame": forwarding stops but the receiving node still
applies the payload if it's genuinely new (there's no value in
discarding a valid rotation because a producer is chatty).

Metrics:

- `gossip_rate_limited_total{producer="..."}`
- `gossip_forward_dropped_total{reason="rate_limit|ttl|unknown_producer|not_subscribed"}`
- `gossip_push_received_total{type="anchor|fingerprint", status="new|dup|invalid"}`

## 8. Canonicalization

We inherit the QDP-0003 canonicalization discipline verbatim:

- `AnchorGossipMessage` signable bytes: unchanged from QDP-0003.
  The payload is already signed over
  `canonicalize(stripSignature(payload))`. The push envelope
  adds no new signed fields.
- `DomainFingerprint` signable bytes: unchanged
  (`GetDomainFingerprintSignableBytes`).
- Envelope fields (TTL/HopCount/ForwardedBy) are explicitly
  **not** covered by any signature. They are advisory metadata
  for routing; a malicious forwarder can mutate them but cannot
  break payload authenticity.

## 9. Data model changes

Additions to `internal/core/ledger.go` (inside `NonceLedger`):

```go
// Per-producer token bucket for gossip rate limiting.
// Not persisted — reconstructed on node restart.
gossipProducerBuckets map[string]*gossipBucket
gossipRateMutex       sync.Mutex
```

No changes to `signerKeys`, `latestFingerprints`, or
`seenGossipMessages` — the existing structures already cover the
state we need.

Additions to `internal/core/network.go`:

```go
// Deterministic peer iteration order for fan-out. Avoids
// relying on Go map iteration order so tests are repeatable.
func (qn *QuidnugNode) sortedKnownNodes() []string
```

## 10. Migration

Push gossip is **additive**: existing pull endpoints stay live
indefinitely. The rollout is:

1. **v2.3.0-alpha1** — Code merged behind `EnablePushGossip`
   config flag (default **off**). Nodes with flag on emit and
   accept push. Nodes with flag off ignore — push traffic
   simply doesn't reach them.
2. **v2.3.0-alpha2** — Shadow observation window (minimum 2
   weeks). Operators report via metrics on `gossip_push_*`
   counters. Expected outcomes:
   - Observed gossip-latency drop for rotations.
   - Zero unvalidated-payload applies (confirms dedup
     placement).
   - Per-producer rate usage stays well under cap in
     steady-state.
3. **v2.3.0** — Flag default flips to **on**. Operators who
   opted out explicitly keep the off value in config. Pull
   endpoints remain.

There is no hard-fork dependency. This is a
transport-enhancement, not a consensus change.

## 11. Test plan

### 11.1 Unit tests (`internal/core/gossip_push_test.go`)

- **TestAnchorPushEnvelopeRoundtrip** — envelope marshals /
  unmarshals without losing payload.
- **TestPushGossipDedupBeforeValidate** — receiver with invalid
  producer key returns 200 on duplicate even though signature
  would fail (proves dedup runs first).
- **TestPushGossipSubscriptionFilter** — receiver without
  `signerKeys[producer]` drops anchor push without validating.
- **TestPushGossipTTLClamp** — sender-forged `TTL = 100`
  arrives and is clamped to `DomainGossipTTL` before forwarding.
- **TestPushGossipNoLoop** — A → B → A → B … converges after
  one hop because dedup stops the second arrival.
- **TestPushGossipRateLimitDrop** — 40 messages from same
  producer in 60s → 30 applied, 10 rate-limited and dropped
  from forwarding.
- **TestPushGossipForwardExcludesSender** — node B receives
  from A, fans out to C/D/E but not back to A.

### 11.2 Integration tests (`internal/core/gossip_push_integration_test.go`)

- **TestThreeHopPropagation** — 4-node simulated network
  (A producer, B/C intermediate, D sink). Anchor sealed in A
  reaches D in ≤ 3 hops. Measured ledger state convergence.
- **TestPushAndPullCoexist** — in a 2-node network with push
  on, pull still works (compatibility).
- **TestPushThenPull** — push enabled for one node, pull for
  other; anchor still reaches the pull-only side (pull-only
  node missed the push; verified a subsequent poll retrieves
  it).

### 11.3 Adversarial tests

- **TestPushForgedProducer** — push with `GossipProducerQuid`
  that's not in receiver's ledger → dropped + metric
  `gossip_forward_dropped_total{reason="unknown_producer"}`
  increments.
- **TestPushTamperedEnvelope** — attacker flips
  `payload.Timestamp` after signing → signature fails →
  rejected.
- **TestPushReplayKilled** — legitimate message replayed 100×
  → dedup kills all 99 extras, producer rate counter
  unaffected (dedup runs first).
- **TestPushFloodByCompromisedValidator** — 1000 valid
  distinct messages from one producer in 60s → 30 applied,
  forwarding halts at bucket exhaustion.

### 11.4 Property tests

- **PushEnvelopeNeverAltersPayload** — fuzz: any envelope
  mutation that changes `HopCount`/`TTL`/`ForwardedBy` must
  not change the output of payload-signable-bytes.
- **MessageIDStableUnderRoundTrip** — JSON marshal/unmarshal
  of `AnchorGossipMessage.MessageID` is stable across N
  iterations (we already rely on this; explicit property
  test locks it in).

## 12. Rejection paths (explicit list)

Every path below is a silent drop with metric increment
(operator-observable, not client-observable):

| Condition                                      | Counter                                                           |
|------------------------------------------------|-------------------------------------------------------------------|
| Envelope schema version unknown                | `gossip_forward_dropped_total{reason="schema"}`                    |
| TTL ≤ 0 on receipt                             | `gossip_forward_dropped_total{reason="ttl"}`                       |
| Payload producer unknown                       | `gossip_forward_dropped_total{reason="unknown_producer"}`          |
| Payload signature invalid                      | `gossip_forward_dropped_total{reason="sig"}`                       |
| Fingerprint BlockHeight ≤ stored               | `gossip_forward_dropped_total{reason="monotonicity"}`              |
| Anchor already applied (seen in ledger)        | `gossip_forward_dropped_total{reason="dup"}`                       |
| Rate-limit bucket exhausted                    | `gossip_rate_limited_total{producer="..."}`                        |
| Receiver not subscribed to producer/domain     | `gossip_forward_dropped_total{reason="not_subscribed"}`            |

## 13. Success criteria

Measured on a 10-node simulated network with realistic
topology (one "home" domain per region, two regions):

1. A rotation sealed in region-A reaches 100% of region-B
   nodes within `3 × DomainGossipInterval` = 6 minutes at
   defaults. **Baseline (pull-only):** up to
   `DomainGossipInterval × (1 + polling_jitter)` per region =
   2–4 minutes per pull, plus region-crossing delay = worst
   case ~20 min.
2. Per-node CPU overhead < 2% at steady state (10 anchors/hr
   producer rate).
3. Per-node network egress ≤ 150 KB/min at steady state.
4. Zero missed rotations over 72h of fuzz traffic (10k
   messages, 10 producers, mixed valid/invalid).

## 14. Operational considerations

- **Default off for v2.3.0-alpha.** Rollout under feature flag.
- **Metrics dashboards.** Ship a Grafana dashboard definition
  alongside the code:
  - `gossip_push_received_total` (rate by type)
  - `gossip_push_applied_total` (rate by type)
  - `gossip_forward_dropped_total` (rate by reason)
  - `gossip_rate_limited_total` (rate by producer)
  - `gossip_propagation_latency_seconds` (histogram,
    producer timestamp → receiver apply time)
- **Runbook entries** (add to `docs/ops/runbooks.md`):
  - "Push gossip emitting but not propagating" — check TTL
    config, peer count, producer key seeding.
  - "High rate-limit drops" — legitimate burst (raise cap) vs.
    compromise (investigate producer).
  - "Gossip latency spike" — check producer → first-hop
    latency, then first-hop → second-hop, etc.

## 15. Alternatives considered

### 15.1 Signed forwarding (rejected)

Require each forwarder to re-sign the message before
forwarding. This would let receivers trace the exact path.
**Rejected** because:

- Attack surface expands: a bug in signature
  re-serialization is a chain-break vector.
- Signatures are the expensive operation, not the dedup.
  Adding a signature per hop turns an `O(hops)` transport
  cost into `O(hops × signature)` — a regression.
- The producer signature already proves authenticity; hop
  signatures prove nothing useful in absence of a PKI for
  forwarders.

### 15.2 Pub/sub broker (rejected)

A central message bus to fan messages out. **Rejected**
because it contradicts the decentralized premise of the
protocol. A broker is a single point of failure and a
censorship lever.

### 15.3 Pull-on-signal (rejected)

Broadcast only a lightweight "something changed in domain D"
hint; receivers pull the full payload themselves. **Rejected**
because:

- Two round-trips per update (hint + pull) vs. one (push).
- Producer can't rate-limit pulls; receivers stampede.
- Pull endpoints remain the fallback regardless.

### 15.4 QUIC-based push (deferred)

A QUIC push path would reduce handshake cost, especially for
large fanouts. **Deferred** to a future transport QDP; H1
stays on HTTP for wire-level compatibility with existing
infrastructure.

### 15.5 Subscription by predicate (deferred)

"Receive all rotations where `signer.weight ≥ 0.8`". A
scaling optimization for nodes observing a huge signer pool.
**Deferred** — the implicit subscription-by-ledger-state
approach in §6.3 covers the immediate case.

## 16. Open questions

1. **Fan-out selection.** Current design fans out to every
   known node each hop. For very large networks this is
   `O(N²)` total traffic. A bounded-fanout design
   (`sqrt(N)` peers per hop) is an optimization we should
   revisit if a deployment exceeds ~50 nodes. Defer for now.
2. **Per-signer rate limiting.** Specified at
   `GossipSignerRateMax = 10/min`. Is this too tight for a
   signer doing rapid successive rotations under active
   compromise? Operator override: a config knob
   `GossipSignerRateMax` lets specific deployments raise it.
3. **TTL calibration for larger topologies.** Default
   `DomainGossipTTL = 3` assumes ≤ 3-hop worst-case. A
   6-region deployment likely needs 4 or 5. Add a
   deployment-sizing guide to the runbook.
4. **Interaction with H4 (lazy epoch propagation).** H4
   proposes on-demand pulls when state is stale; push gossip
   reduces but does not eliminate the need. Coordinate
   timing with QDP-0008 so we don't over-engineer one at the
   expense of the other.

## 17. Timeline

| Phase         | Work                                           | Duration |
|---------------|------------------------------------------------|----------|
| Design        | This document → review → sign-off              | 1 week   |
| Implementation| §6 wire + §9 data model + §7 rate limit        | 2 weeks  |
| Test          | §11 unit + integration + adversarial           | 1 week   |
| Alpha rollout | v2.3.0-alpha1 flag-gated; metrics observation  | 2 weeks  |
| Flip default  | v2.3.0 default-on                              | post-review |

## 18. References

- [QDP-0003: Cross-Domain Nonce Scoping](0003-cross-domain-nonce-scoping.md)
- [QDP-0004: Phase H Roadmap](0004-phase-h-roadmap.md) §3.1
- [`internal/core/network.go`](../../internal/core/network.go) — existing `DomainGossip` push path
- [`internal/core/anchor_gossip.go`](../../internal/core/anchor_gossip.go) — current pull-submit anchor flow
- [`internal/core/domain_fingerprint.go`](../../internal/core/domain_fingerprint.go) — fingerprint monotonic store

---

**Review status.** Draft. Sign-off required from maintainers on
§6.3 (implicit subscription semantics) and §7 (rate limit cap
calibration) before implementation begins.
