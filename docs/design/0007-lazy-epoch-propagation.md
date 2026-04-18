# QDP-0007: Lazy Epoch Propagation (H4)

| Field      | Value                                                    |
|------------|----------------------------------------------------------|
| Status     | Draft                                                    |
| Track      | Protocol                                                 |
| Author     | The Quidnug Authors                                      |
| Created    | 2026-04-18                                               |
| Requires   | QDP-0003 (landed), QDP-0005 (H1, landed)                 |
| Implements | Phase H4 of QDP-0004 roadmap                             |
| Target     | v2.4                                                     |

## 1. Summary

QDP-0003 §15.4 identified a corner case: a signer who transacts
in domain B only quarterly may have rotated their key in domain
A months ago, and domain B's local ledger still records the old
epoch because push-gossip (H1) either wasn't deployed yet or
didn't reach B at rotation time. The signer's first
post-rotation transaction in B should not silently commit under
the stale epoch.

This document specifies a "lazy" propagation path: when a
transaction arrives at a domain and the signer's local
`CurrentEpoch` hasn't been refreshed recently, the node fires
an asynchronous fingerprint probe to the signer's home domain
before admitting the transaction. The transaction is held in a
quarantine queue until the probe completes or times out.

## 2. Problem statement

Push gossip (H1) handles the **steady-state** case: a rotation
propagates within minutes. Three failure modes remain:

1. **Pre-H1 or partition.** A cluster running a pre-H1 version,
   or cut off from the producing domain during the rotation
   window, never received the push.
2. **Silent drift.** A network-level issue drops the push. Dedup
   would accept a redelivery, but nothing triggers one.
3. **New node joins.** A fresh node joining after a rotation
   has no local state indicating the rotation happened unless
   the snapshot bootstrap (H3) carries it.

Without a lazy pull at admission time, a signer that rotated in
A but whose first post-rotation transaction lands in B would be
admitted against B's **stale** epoch — defeating the whole
point of the rotation.

## 3. Goals and non-goals

**Goals.**

- **G1.** A transaction whose signer's local epoch is older
  than `t_recency` (proposal: 7 days since last refresh) is
  quarantined pending a fingerprint probe.
- **G2.** The probe target is derivable without operator input:
  each signer's identity record carries an optional `HomeDomain`
  field; unset falls back to the node's own domain set.
- **G3.** Probe completes within a bounded time window
  (`t_probe = 30s` default). On timeout the admission decision
  falls back to operator policy.
- **G4.** Quarantine is bounded: a transaction sitting longer
  than `t_quarantine = 1 hour` is dropped with a metric.
- **G5.** No wire changes. Uses the existing QDP-0003 GET
  `/api/v2/domain-fingerprints/{domain}/latest` endpoint.
- **G6.** Coexists with H1. The push path and the lazy-pull
  path are both viable; H4 is purely for the case where push
  hasn't delivered.

**Non-goals.**

- **NG1.** Pre-warming (proactively fetching epochs for known-
  hot signers). A scaling optimization; QDP-0004 §3.4 defers it.
- **NG2.** Cross-domain anchor propagation via probe. The probe
  returns only a fingerprint; the actual anchor must still
  arrive via push or pull.
- **NG3.** Replacing H1. If push is on, most transactions never
  quarantine because their signer's state was refreshed recently
  via push.

## 4. Threat model

| Threat                                                | Mitigation                                                                                    |
|-------------------------------------------------------|-----------------------------------------------------------------------------------------------|
| Forged HomeDomain field                                | Identity record is on-chain and signed by the signer. Forgery requires signer compromise. Then the attacker could point HomeDomain anywhere — but the response fingerprint must still be signed by a validator we recognize, so a random server can't forge the probe response. |
| Probe target down → admission path stalls              | Bounded timeout `t_probe`. Fallback to configurable policy.                                    |
| Attacker floods with stale transactions to fill quarantine | Quarantine capacity is bounded. Overflow drops OLDEST (not newest) so an attacker can't evict legitimate txs.                                     |
| Probe response carries fingerprint older than local     | Monotonicity check (§QDP-0003): older heights don't overwrite newer. Probe is no-op in that case. |

## 5. Data model

### 5.1 Identity record extension

```go
// IdentityTransaction additions:
type IdentityTransaction struct {
    // existing fields...
    HomeDomain string `json:"homeDomain,omitempty"`
}
```

`HomeDomain` is optional. When set, it is the fully-qualified
trust domain where the signer "lives" — where they rotate and
where other domains should probe for their latest state. Empty
falls back to the node's local domain policy.

Setting `HomeDomain` is a trust declaration: the signer is
saying "trust the validators in this domain to tell you about
my epoch." An attacker-controlled `HomeDomain` only works if
the attacker also controls a validator in that domain AND that
validator's key is in the receiving node's ledger.

### 5.2 Quarantine queue

New per-node structure:

```go
// QuarantinedTx is a transaction held pending a recency
// probe. Stored with enough context to dispatch when the
// probe clears.
type QuarantinedTx struct {
    Tx         interface{}
    EnqueuedAt time.Time
    Signer     string
    HomeDomain string
    Retries    int
}

// Per-node state; not persisted across restarts (in-flight
// transactions at restart are lost, which matches the
// existing PendingTxs behavior).
quarantine         map[string]QuarantinedTx  // keyed by tx hash
quarantineMutex    sync.Mutex
```

### 5.3 Recency tracking

New per-ledger map:

```go
// lastEpochRefresh[signer] is the unix ts at which this node
// last confirmed the signer's current epoch — either by
// observing a block containing their anchor, or by receiving
// push gossip, or by a successful probe response.
lastEpochRefresh map[string]int64
```

## 6. Protocol

### 6.1 Admission decision

Replace the current synchronous admit path with:

```
admit(tx):
    signer = tx.PublicKeyQuid  (or tx.From / equivalent)
    if EpochRecent(signer) OR !EnableLazyEpochProbe:
        return admitNow(tx)
    if QuarantineFull():
        drop_oldest_quarantined()
    enqueue(tx, signer)
    fire_probe(signer)
    return 202-pending
```

### 6.2 Probe

```
probe(signer):
    home = IdentityOf(signer).HomeDomain
    if home == "" {
        home = this node's primary supported domain
    }
    peers = FindPeersForDomain(home)   (reuses DomainRegistry)
    for peer in peers (bounded: up to 3):
        resp = GET http://peer/api/v2/domain-fingerprints/{home}/latest  (timeout t_probe_peer)
        if resp.ok:
            if ValidateFingerprint(resp):
                StoreDomainFingerprint(resp)
                MarkRecencyRefresh(signer)
                release_quarantine(signer)
                return
    // No peer answered in time; record metric, wait for
    // quarantine aging to drop or operator override.
```

### 6.3 Release from quarantine

When the probe completes successfully (or recency is otherwise
refreshed — e.g., a push gossip arrives for the same signer):

- All quarantined txs for that signer are re-admitted through
  the normal path.
- If the fingerprint revealed a new epoch, the existing
  validation machinery handles the re-validation using the
  updated key.

### 6.4 Quarantine aging

A periodic sweep (every 5 min) drops entries older than
`t_quarantine`. Each drop emits a
`quarantine_drop_aged_total{reason}` metric.

## 7. Configuration

New config fields (all additive):

```go
EnableLazyEpochProbe      bool          // default false
EpochRecencyWindow        time.Duration // default 7d
EpochProbeTimeout         time.Duration // default 30s
EpochProbePeerTimeout     time.Duration // default 5s
QuarantineMaxSize         int           // default 1024
QuarantineMaxAge          time.Duration // default 1h
ProbeTimeoutPolicy        string        // "reject" | "admit_warn" — default reject
```

### 7.1 Timeout policy

- **`reject`** — if probe times out, quarantined tx is
  rejected with a `quarantine_probe_timeout` reason. Safer.
- **`admit_warn`** — if probe times out, admit the tx
  anyway and emit a warning log + metric. Permissive.

Default **reject**: lazy propagation's whole point is to catch
stale epochs. Admitting-on-timeout defeats the purpose.

## 8. Validation rules

- **Identity `HomeDomain`**: when set, must be a non-empty
  string matching the domain-name pattern used elsewhere
  (same validator as `TrustDomain`).
- **Fingerprint in probe response**: validated via
  existing `VerifyDomainFingerprint`. Invalid responses do
  NOT mark the signer recent.
- **Recency**: `lastEpochRefresh[signer] >= now - EpochRecencyWindow`.

## 9. HTTP surface

No new endpoints. The probe is a client-side GET against the
existing QDP-0003 fingerprint endpoint.

One diagnostic endpoint added:

```
GET /api/v2/quarantine          → list current quarantined txs (count + aged oldest)
GET /api/v2/quarantine/{signer} → details for a specific signer
```

For operator visibility only; not part of the protocol.

## 10. Migration

Additive, behind `EnableLazyEpochProbe` feature flag (default
off):

1. **v2.4.0-alpha.** Code lands. Operators can flip the flag on
   per-node. Mixed networks are fine — a probe against a node
   that doesn't recognize the fingerprint endpoint just fails
   over to the next peer.
2. **v2.4.0.** Default off for backwards compatibility. An
   operator who wants full coverage flips it on.
3. **v2.5.0.** Default on. By this point H1 has been on for two
   releases and rotation coverage in push is well-exercised; lazy
   probe is the belt to the H1 suspenders.

Identity records written under v2.3 and earlier have no
`HomeDomain`. That's fine: probe falls back to local domain.
New identity records should populate it.

## 11. Test plan

### 11.1 Unit

- **Admit path immediate** when signer is recent (no
  quarantine).
- **Quarantine on stale signer** when flag on and signer not
  recent.
- **Probe success → release** — quarantined txs admitted after
  successful probe.
- **Probe timeout + reject policy** — stale tx rejected.
- **Probe timeout + admit_warn** — stale tx admitted with
  metric.
- **Quarantine overflow evicts oldest** — 2000 txs with
  QuarantineMaxSize=1000 leaves newest 1000.
- **Age-out sweep** — tx older than QuarantineMaxAge dropped
  on periodic sweep.
- **HomeDomain fallback** — signer with empty HomeDomain uses
  node's primary domain.

### 11.2 Integration

- **2-node setup.** Node A rotates signer Q. Node B is
  partitioned (no push received). New tx from Q arrives at
  B. B probes A, gets fresh fingerprint, admits tx against
  new epoch. Verified via CurrentEpoch(Q) advancing on B.
- **Probe failure with reject policy** — B partitioned from A
  AND from A's peers → probe times out → tx rejected. Operator
  sees metric.

### 11.3 Adversarial

- **Probe response signed by stranger** — ignored, falls
  through to next peer.
- **Probe response with older fingerprint than local** —
  monotonicity rejects the update; tx still quarantined (no
  recency refresh).
- **HomeDomain points at attacker-controlled domain** —
  attacker's fingerprint covers a block with spoofed hash;
  `VerifyDomainFingerprint` fails because the producer isn't a
  recognized validator.

## 12. Metrics

```
quidnug_quarantine_size
quidnug_quarantine_enqueued_total{reason}
quidnug_quarantine_released_total{trigger}   // probe|gossip|manual
quidnug_quarantine_rejected_total{reason}
quidnug_probe_attempts_total
quidnug_probe_success_total
quidnug_probe_timeout_total
quidnug_probe_response_invalid_total
```

## 13. Alternatives considered

### 13.1 Synchronous probe (rejected)

Do the probe inline during `admit`. **Rejected** because it
would turn every stale-signer admission into a latency spike
— not just slow, but unpredictably slow (depends on peer
responsiveness). Quarantine lets the API return 202-accepted
immediately and process asynchronously.

### 13.2 Probe every tx unconditionally (rejected)

Skip the recency check; probe on every signer mention.
**Rejected**: a signer who transacts every minute would hammer
probes needlessly. Recency window amortizes.

### 13.3 Push-to-probe (rejected)

Replace the probe with an anchor-gossip PULL from the home
domain. **Rejected** because the fingerprint is enough — we
don't need the full anchor to learn "epoch has advanced."
Getting the anchor is the user's problem once they know to ask.

## 14. Open questions

1. **Recency refresh on push.** When a push gossip arrives for
   a signer, do we refresh recency for that signer even if the
   push was a no-op? Proposal: yes. Push is evidence of a live
   path.
2. **Quarantine persistence.** Lost on restart. Matches existing
   PendingTxs behavior. If we add persistence for PendingTxs we
   should extend to quarantine — same decision.
3. **Interaction with H3 bootstrap.** A freshly-bootstrapped
   node has all-stale signers. Are we about to quarantine every
   first transaction? Mitigation: mark bootstrap completion as a
   blanket recency refresh for all seeded signers.

## 15. References

- [QDP-0003: Cross-Domain Nonce Scoping](0003-cross-domain-nonce-scoping.md) §15.4
- [QDP-0005: Push-based gossip (H1)](0005-push-based-gossip.md)
- [QDP-0004: Phase H Roadmap](0004-phase-h-roadmap.md) §3.4

---

**Review status.** Draft. Sign-off on timeout policy default
(`reject` vs. `admit_warn`) and HomeDomain semantics before
implementation.
