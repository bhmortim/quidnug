# QDP-0022: Timed Trust & TTL Semantics

| Field      | Value                                                            |
|------------|------------------------------------------------------------------|
| Status     | Landed                                                           |
| Track      | Protocol                                                         |
| Author     | The Quidnug Authors                                              |
| Created    | 2026-04-20                                                       |
| Requires   | QDP-0001 (nonce ledger)                                          |
| Implements | End-to-end enforcement of `ValidUntil` on TRUST edges + `expiresAt` on EventTransaction payloads |

## 1. Summary

The `TrustTransaction` struct has carried a `ValidUntil` field
(Unix seconds, optional, zero means "no expiry") since the
protocol's first version. The field was serialized, signed, and
accepted by every client SDK, but no layer of the reference
node actually *enforced* it. Transactions with past-timestamped
`ValidUntil` were accepted, persisted, and walked during trust
computation exactly like non-expiring ones.

QDP-0022 fixes that gap. It also adds the symmetric capability
for event payloads: an optional `expiresAt` field (Unix
nanoseconds) that hides an event from the default API response
once its deadline passes. Neither mechanism deletes data — the
chain remains strictly append-only — but both suppress
expired records at every read boundary (trust-graph walk, API
serving layer, SDK).

This unblocks three upcoming features:

- **QDP-0017 consent expiry.** GDPR / CCPA / LGPD consent
  records have mandatory expiration semantics; the
  `CONSENT_GRANT` transaction carries `expiresAt` in its
  payload and relies on this layer to hide withdrawn / lapsed
  consent.
- **QDP-0014 node advertisement refresh.** Advertisements
  declare a `validUntil`; stale advertisements should drop out
  of discovery responses automatically.
- **Short-lived session / capability tokens.** Application
  layers can publish time-bounded capabilities as events
  without writing bespoke expiration logic.

## 2. Goals and non-goals

**Goals:**

- Block submission of TRUST transactions whose `ValidUntil` is
  already in the past.
- Skip expired TRUST edges when computing `ComputeRelationalTrust`
  (and the enhanced variant), `GetDirectTrustees`, `GetTrustLevel`,
  and `GetTrustEdges`.
- Filter EventTransactions with past `expiresAt` from default
  API responses.
- Provide a test-friendly clock override so expiry-dependent
  tests can run deterministically.
- Preserve append-only ledger semantics; expiry is a read-time
  filter, never a mutation.

**Non-goals:**

- Garbage-collection of expired records from block storage.
- Network-wide agreement on "now" — each node uses its local
  clock to evaluate `ValidUntil`. Small clock skew is tolerated
  because TRUST edges typically expire on day / week / month
  granularity, not seconds.
- Cryptographic shredding of expired data. That's QDP-0015 /
  QDP-0017 territory (erasure via key deletion), not TTL.
- Automatic renewal. If an operator wants to extend an
  expiring edge they publish a new TRUST transaction; the
  registry replaces the prior expiry with the new one.

## 3. Data model

### 3.1 TRUST edge expiry

```go
type TrustTransaction struct {
    BaseTransaction
    Truster     string  `json:"truster"`
    Trustee     string  `json:"trustee"`
    TrustLevel  float64 `json:"trustLevel"`
    Nonce       int64   `json:"nonce"`
    ValidUntil  int64   `json:"validUntil,omitempty"` // Unix seconds; 0 = no expiry
    // ...
}
```

Semantics:

- **0** = edge never expires.
- **>0** = edge is valid while `nowUnix() < ValidUntil`. The
  boundary is exclusive on the `ValidUntil` side: an edge with
  `ValidUntil=T` is expired at `nowUnix()==T`.
- **<0 or <= tx.Timestamp at submission** = rejected by the
  block-level validator. An edge cannot be expired at birth.

### 3.2 Expiry registry

The reference node maintains a parallel map on `QuidnugNode`:

```go
// TrustExpiryRegistry tracks each edge's ValidUntil timestamp.
// Zero means no expiry.
// Guarded by the same TrustRegistryMutex as TrustRegistry.
TrustExpiryRegistry map[string]map[string]int64
```

Keyed by `[truster][trustee]`, matching the shape of
`TrustRegistry`. It is populated in `updateTrustRegistry` on
every block-applied TRUST tx, and the *latest* value wins. This
handles two lifecycle flows:

- **Renewal**: a new TRUST with a later `ValidUntil` replaces
  the prior expiry, extending the edge.
- **Shortening**: a new TRUST with a sooner `ValidUntil` also
  replaces the prior value, allowing operators to revoke
  trust ahead of the original schedule.

### 3.3 Event payload expiry

EventTransactions carry a free-form `payload map[string]interface{}`.
Any payload that sets:

```json
{ "expiresAt": 1712345678901234567 }
```

(Unix nanoseconds, int64 or JSON-unmarshaled float64) is
considered expired once `nowNano() > expiresAt`. Payloads
without the key, with `expiresAt=0`, or with non-numeric values
are never treated as expired.

The field is intentionally looser than the TRUST `ValidUntil`:
events are used for a wide variety of domain-specific payloads,
and the helper opts for "fail open" on malformed values so that
operators can diagnose rather than silently losing data.

## 4. Implementation

Five files carry the TTL logic. All of them live under
`internal/core/`.

### 4.1 `ttl.go` — central clock + helpers

- `setTestClockNano(int64)` — freezes `now` for tests.
- `nowNano() int64` / `nowUnix() int64` — the only clock source
  the TTL layer reads. Falls through to `time.Now()` in
  production.
- `IsTrustEdgeValid(truster, trustee) bool` — public, acquires
  `TrustRegistryMutex` internally.
- `isTrustEdgeValidLocked(truster, trustee) bool` — internal
  variant for callers already holding the registry mutex
  (avoids re-entering an RLock under a pending writer).
- `GetTrustEdgeExpiry(truster, trustee) (int64, bool)` — query
  for observability tooling.
- `IsEventPayloadExpired(payload map[string]interface{}) bool`
  — static helper used by the event-filter.

### 4.2 `validation.go` — submission-time check

`ValidateTrustTransaction` rejects any tx whose `ValidUntil != 0`
and `<= max(tx.Timestamp, nowUnix())`. The comparison uses the
*later* of the tx's own declared timestamp or the node's
current wall clock, so that a tx cannot claim an old timestamp
to slip past the filter.

### 4.3 `registry.go` — graph walk filter

Three lookup methods apply the filter inline while they already
hold the registry mutex:

- `GetTrustLevel(truster, trustee)` returns 0 for expired edges.
- `GetDirectTrustees(truster)` drops expired trustees from the
  returned map.
- `GetTrustEdges(truster, includeUnverified)` drops expired
  edges from both the verified and unverified views.

`ComputeRelationalTrust` and `ComputeRelationalTrustEnhanced`
both reach the graph through these methods, so TTL is enforced
for every path BFS automatically, with no code changes inside
the search loops.

### 4.4 `registry.go` — event filter

```go
func FilterExpiredEvents(events []EventTransaction) []EventTransaction
```

Returns a new slice with expired payloads omitted, preserving
the input order. Used by the HTTP serving layer.

### 4.5 `handlers.go` — API surface

The stream-events endpoint applies the filter by default:

```
GET /streams/{subjectId}/events                 → filtered
GET /streams/{subjectId}/events?include_expired=true → raw
```

The opt-in `include_expired` parameter is intended for audit
tooling, incident forensics, and the operator admin console.
Normal application traffic gets filtered output.

## 5. Observer-clock semantics

There is no consensus on "now." Each reader node evaluates
`ValidUntil` and `expiresAt` against its own local clock.
Implications:

- An edge with `ValidUntil = T` may still be visible on a node
  with a clock skewed 5 seconds behind, then disappear on the
  same second as a node with a correctly-set clock. For
  typical TTL granularity (hours / days / months) this is
  harmless.
- A node with a badly-skewed clock (minutes+ out) will produce
  incorrect trust computations. Operators are expected to run
  NTP; QDP-0018's observability plane exposes a clock-skew
  metric on every federation handshake that alerts when
  drift exceeds 2 seconds.
- Block validation still uses the tx's own `Timestamp` for
  most consistency checks; only the TTL filter uses wall
  clock. This limits the blast radius of clock skew to
  "which edges are currently considered valid," not "which
  blocks are accepted."

## 6. Pagination with filtering

`GetStreamEventsHandler` paginates before filtering. This means
that a response page can contain fewer records than the
`limit` even when more pages exist. Rationale:

- Expired events are expected to be rare relative to the full
  set; paging pre-filter avoids loading the whole registry
  into memory on each query.
- Consumers that need exact pagination use
  `include_expired=true` to see a stable, un-filtered view.
- The `total` field in the pagination metadata reflects the
  unfiltered count, so consumers can tell whether more data
  exists.

If a domain accumulates enough expired events that this
becomes user-visible, the remedy is an offline compaction
pass (future work), not a change to the query semantics.

## 7. Test harness

Every TTL-sensitive test imports `ttl_test.go`'s helpers:

```go
func resetTestClock()                    // revert to real time
func seedTrustEdge(node, t, r, level, validUntil)  // seed an edge

// Production code calls nowNano() / nowUnix() — tests call:
setTestClockNano(fixedNano)
```

`setTestClockNano` is intentionally package-private (`setTest...`
prefix) and never exported; release builds cannot accidentally
freeze the clock. This is enforced by static analysis at the
linter layer.

Coverage is in `internal/core/ttl_test.go` — 19 cases covering
every helper branch, plus integration tests that walk the
graph and the event filter.

## 8. Migration & rollout

TTL enforcement is backward-compatible:

- Existing edges in the registry with no `ValidUntil` (the
  common case today) behave exactly as before: the expiry
  registry records `0` and `IsTrustEdgeValid` short-circuits
  to `true`.
- Existing EventTransactions without `expiresAt` in their
  payload behave exactly as before.
- Clients that don't know about the new behavior keep
  working. Only clients that explicitly set `ValidUntil` /
  `expiresAt` see the new filter.

There is no wire-format change, no new transaction type, and
no chain-replay requirement. A node can adopt QDP-0022 by
upgrading its binary.

## 9. Future work (not in scope)

- **On-chain cleanup trigger.** A `GC_ANCHOR` transaction that
  records "all TRUST edges with `ValidUntil < X` are
  permanently discarded by consensus" — useful for pruning
  storage in very long-lived domains. Deferred; requires
  further thought on how clients reconstruct state from a
  pruned chain.
- **Typed event-payload schemas.** `expiresAt` is currently a
  magic field name. A typed `TypedEvent` extension with
  schema validation (schema registry per domain) would make
  TTL discoverable to SDK clients. Parked pending enough
  user demand to justify the complexity.
- **Expiry alerts.** An operator could subscribe to "notify me
  when trust edges I rely on are 7 days from expiring."
  Implementable at the observability layer (QDP-0018) without
  protocol changes.

## 10. Status

Landed in `internal/core/` on 2026-04-20.

- `ttl.go` (new, 129 lines)
- `ttl_test.go` (new, 285 lines, 19 test cases)
- `node.go`: added `TrustExpiryRegistry` field + initializer
- `registry.go`: filter wired into graph lookups + `FilterExpiredEvents`
- `validation.go`: TTL rejection at submission
- `handlers.go`: `include_expired` query param on stream events

All tests pass; no regressions in the wider suite.
