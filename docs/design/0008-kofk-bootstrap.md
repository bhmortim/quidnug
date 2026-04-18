# QDP-0008: Snapshot K-of-K Bootstrap Protocol (H3)

| Field      | Value                                            |
|------------|--------------------------------------------------|
| Status     | Draft                                            |
| Track      | Protocol                                         |
| Author     | The Quidnug Authors                              |
| Created    | 2026-04-18                                       |
| Requires   | QDP-0001 §7 snapshots (landed)                   |
| Implements | Phase H3 of QDP-0004 roadmap                     |
| Target     | v2.4                                             |

## 1. Summary

QDP-0001 §7 defined the on-wire `NonceSnapshot` format but left
the consumer-side bootstrap protocol as a deferred item. A
fresh node joining the network today has no authoritative way
to seed its nonce ledger from a consistent snapshot — it must
either start blank (replay the entire chain) or trust a single
peer blindly.

This document specifies a K-of-K bootstrap protocol: a
bootstrapping node queries at least `K` peers for their latest
snapshot of a domain; if all `K` responses agree on block
height and block hash within a small tolerance window, the
node seeds its ledger. Any disagreement escalates to operator
intervention.

## 2. Background

Snapshots are produced every `DefaultSnapshotInterval` (64)
blocks by the domain's validators. Each snapshot lists every
`(signer, epoch)` key's max accepted nonce as of a named block.
Two honest producers at the same height yield byte-identical
snapshots because entry sort order is deterministic (QDP-0001
§7.2). Any mismatch between two snapshots for the same height
is evidence of state divergence — either a partition, a
Byzantine producer, or a bug.

## 3. Problem statement

Three scenarios today:

1. **Fresh node.** Joins a domain with 10k+ existing signers.
   Without a snapshot, it cannot validate the next transaction
   until it has replayed the entire chain. Initial sync is
   hours.
2. **Restored node.** A node comes back after an outage. Same
   problem, maybe smaller scale.
3. **Validating an incoming snapshot.** A lone signed snapshot
   from an unknown peer is untrustworthy — we need evidence
   from multiple sources.

The QDP-0001 §7.4 sketch proposed a K-of-K agreement rule but
didn't specify:

- How to discover peers to query.
- How to handle K-of-K disagreement.
- What "agreement" means (exact byte equality? hash equality?
  bounded tolerance on a specific field?).
- What happens if fewer than K snapshots are available.
- How the newly-bootstrapped node validates incoming traffic
  while the bootstrap is being verified.

## 4. Goals and non-goals

**Goals.**

- **G1.** Fresh node bootstrap completes in under 2 seconds
  for a domain with 10k signers when 3 honest peers are
  available.
- **G2.** K-of-K consensus required by default (K=3). Any
  disagreement fails closed — no silent bootstrap from a
  minority.
- **G3.** Disagreement is loud: specific error includes the
  peer IDs whose snapshots differed so operators can
  investigate.
- **G4.** Signature validation per peer — an unsigned or
  badly-signed snapshot excludes that peer from the quorum
  count.
- **G5.** Post-bootstrap, the node runs in shadow-verify mode
  for the first `N` blocks (default 64 = one snapshot
  interval). If live block replay diverges from the
  snapshot-seeded state at any point, the node halts with a
  specific error.
- **G6.** Operator override: explicit "trust this peer" config
  path for small / development deployments where K<3.

**Non-goals.**

- **NG1.** Byzantine fault tolerance under M malicious peers.
  K-of-K assumes the operator chose reasonable seed peers. A
  PBFT-style consensus for snapshots is a v3 concern.
- **NG2.** Incremental bootstrap (only seed what changed since
  last restart). Full-seed every time is simpler and
  acceptable given we aim for under 2s.
- **NG3.** Cross-domain bootstrap. Per-domain bootstrap; a
  node that participates in multiple domains runs the
  protocol once per domain.

## 5. Threat model

| Threat                                                      | Mitigation                                                                                                       |
|-------------------------------------------------------------|------------------------------------------------------------------------------------------------------------------|
| Single peer returns a forged snapshot                        | K-of-K — requires `K` peers to agree. Malicious peer in isolation fails quorum.                                  |
| All K peers are colluding                                    | Out of scope. K-of-K assumes operator-chosen seeds.                                                              |
| Peers return snapshots at different heights                  | "Within tolerance" rule — responses within `heightTolerance=4` blocks are treated as compatible if their hashes match at the common height via chain query.      |
| Peer returns an outdated snapshot                            | Accept if still within `staleTolerance=30d` AND K agree.                                                         |
| Peer returns empty snapshot                                  | Empty is a valid response. Must still be signed and K-agree.                                                     |
| MITM alters snapshot in transit                              | Signature on the snapshot covers its contents. Any alteration breaks the signature.                              |
| New node bootstraps from unrelated domain                    | Explicit domain parameter; the endpoint filters by domain.                                                       |

## 6. Protocol

### 6.1 Endpoint

```
GET /api/v2/nonce-snapshots/{domain}/latest
```

Response body: a single signed `NonceSnapshot`, or 404 if no
snapshot has been produced yet. Snapshots are already produced
via the existing `ProduceSnapshot` + `SignSnapshot` flow.

### 6.2 Bootstrap session state machine

```go
type BootstrapSession struct {
    Domain        string
    State         BootstrapState
    K             int                   // quorum target
    PeerSet       []Node                // candidates
    Responses     map[string]NonceSnapshot  // peerID → snapshot
    Started       time.Time
    Error         error
}

type BootstrapState int
const (
    BootstrapIdle BootstrapState = iota
    BootstrapFetching
    BootstrapQuorumMet
    BootstrapQuorumMissed
    BootstrapSeeding
    BootstrapShadowVerify
    BootstrapDone
    BootstrapFailed
)
```

Transitions:

```
Idle → Fetching  (StartBootstrap)
Fetching → QuorumMet (K agreeing responses received)
Fetching → QuorumMissed (K-of-K not achieved within timeout)
QuorumMet → Seeding (apply snapshot to ledger)
Seeding → ShadowVerify (first N blocks after seed)
ShadowVerify → Done (N blocks replayed without divergence)
ShadowVerify → Failed (divergence detected)
QuorumMissed → Failed (no operator override)
QuorumMissed → Seeding (operator override via config)
```

### 6.3 Fetch and agreement

```
1. PeerSet ← discover peers serving this domain (DomainRegistry).
   If |PeerSet| < K: return BootstrapQuorumMissed.
2. For each peer (bounded to max(K, 2K) total):
     GET /api/v2/nonce-snapshots/{domain}/latest
     VerifySnapshot signature; drop if invalid.
     Store response keyed by peerID.
3. Group responses by BlockHash. Pick the largest group.
4. If |largest group| >= K: accept. Apply that snapshot to
   the ledger. Emit metric.
5. Else: BootstrapQuorumMissed. Log each peer's BlockHash /
   BlockHeight so the operator can see who diverged.
```

### 6.4 Height tolerance

Peers may produce snapshots at slightly different heights if
they all land within the same snapshot interval but respond
to the bootstrap request at different times. Acceptance rule:

- All peers in the winning group MUST agree on `BlockHash`.
- Their `BlockHeight` values must differ by ≤ `heightTolerance`
  (default 4) — any larger and we suspect real divergence.

### 6.5 Shadow-verify

After seeding, block replay proceeds as usual but each block's
effect on the nonce ledger is compared against the snapshot-
seeded state:

```
for each new block in [H_snap+1 ... H_snap+N]:
    apply block normally (mutates ledger)
    expectedFromSnapshot = derive(block.NonceCheckpoints + initial seed)
    if currentLedgerState != expectedFromSnapshot:
        halt with BootstrapDivergenceError
        Include block hash, signer, epoch where divergence occurred
```

On completion (no divergence across N blocks), the session
transitions to Done and shadow-verify is disabled.

### 6.6 Operator override

When K-of-K fails AND the operator has explicitly set
`BootstrapTrustedPeer=<nodeID>`, the node may accept that
peer's snapshot alone. Warning metric fires; a log line names
the trusted peer. Intended for development / small-cluster
use; production should leave this unset.

## 7. Data model

### 7.1 Bootstrap state

Kept in-memory per QuidnugNode:

```go
type BootstrapState struct {
    sync.RWMutex
    CurrentSession *BootstrapSession   // nil if not running
    CompletedFor   map[string]time.Time  // domain → time of last success
}
```

Not persisted — a restart triggers a fresh bootstrap.

### 7.2 Config

```go
EnableKofKBootstrap      bool          // default false
BootstrapQuorum          int           // K, default 3
BootstrapPeerTimeout     time.Duration // per-peer GET timeout, default 5s
BootstrapTotalTimeout    time.Duration // total session budget, default 30s
BootstrapHeightTolerance int           // default 4
BootstrapStaleTolerance  time.Duration // default 30d
BootstrapTrustedPeer     string        // operator override, default empty
BootstrapShadowBlocks    int           // N, default 64
```

## 8. Validation rules

- **Snapshot signature**: via existing `VerifySnapshot`.
- **Schema version**: `SnapshotSchemaVersion` == 1.
- **Producer known**: producer's key must be in the
  bootstrapping node's ledger. How does a fresh node know
  producer keys? Via a pre-configured "bootstrap trust list"
  — a set of (quid, public-key-hex) pairs the operator
  hard-codes for K seed peers. Similar to TLS root-CA trust.
- **BlockHash non-empty**, `BlockHeight > 0`, `Timestamp`
  within `BootstrapStaleTolerance` of now.

### 8.1 Bootstrap trust list

```go
BootstrapTrustList []BootstrapTrustEntry

type BootstrapTrustEntry struct {
    Quid        string
    PublicKey   string  // hex
}
```

At bootstrap time the list is seeded into the ledger's
`signerKeys` with `epoch=0`. This gives `VerifySnapshot` the
keys it needs. After the bootstrap completes, these entries
are preserved (same signers still operating) but the node's
live observation can advance them to later epochs if
rotations land.

## 9. HTTP surface

Single new endpoint for bootstrapping clients:

```
GET /api/v2/nonce-snapshots/{domain}/latest
```

Returns the node's latest stored snapshot for the domain, or
404. Does not trigger production — callers that want fresh
snapshots should `POST /api/v2/nonce-snapshots` (already
exists in some form).

Diagnostic:

```
GET /api/v2/bootstrap/status
```

Returns the current session state (or empty if none in flight).
Operator visibility.

## 10. Migration

Additive, flag-gated:

1. **v2.4.0-alpha.** Endpoint and client code land behind
   `EnableKofKBootstrap`. Off by default.
2. **v2.4.0.** Operators enabling fresh bootstrap flip the
   flag. Pre-H3 nodes that don't serve the snapshot
   endpoint simply aren't counted toward quorum.
3. **v2.5.0.** Default on for fresh nodes. Existing nodes
   with state don't trigger bootstrap (their ledger is
   authoritative).

No hard fork. No changes to existing snapshot wire format.

## 11. Test plan

### 11.1 Unit tests

- **PeerDiscoveryLessThanK** — `|PeerSet| < K` returns
  `BootstrapQuorumMissed` without making HTTP calls.
- **AllPeersAgree** — 3 peers return identical snapshots →
  session transitions Fetching → QuorumMet → Seeding → Done
  (shadow-verify not triggered because no new blocks follow
  in this unit).
- **OnePeerDisagrees** — 2 peers agree, 1 returns different
  BlockHash → quorum met (2-of-3 is a majority but we
  require K-of-K which is 3-of-3) → QuorumMissed.
- **HeightToleranceWithin** — 3 peers at heights
  [100, 101, 102] with identical state → accept.
- **HeightToleranceExceeded** — 3 peers at heights
  [100, 200, 300] → reject (gap too big).
- **InvalidSignatureExcluded** — 3 peers respond but one's
  snapshot has a bad signature → that peer is dropped; if
  remaining 2 is < K, reject.
- **StaleSnapshotRejected** — snapshot older than
  `staleTolerance` → reject.
- **TrustListSeeding** — before bootstrap the signer keys
  from the trust list are in the ledger; snapshot verify
  succeeds.
- **OperatorOverride** — K-of-K fails but `BootstrapTrustedPeer`
  matches one of the respondents → accept that peer's
  snapshot with warning metric.

### 11.2 Integration tests

- **ThreeNodeBootstrap** — 3 seed peers running httptest
  servers return identical snapshots; a fresh 4th node
  runs bootstrap end-to-end and reports QuorumMet + Seeding.
- **ShadowVerifyPass** — after seeding, replay N blocks
  with matching state; session transitions to Done.
- **ShadowVerifyDivergence** — after seeding, replay a
  block with deliberately mismatched nonce → halt with
  `BootstrapDivergenceError`.

### 11.3 Adversarial tests

- **ByzantinePeer** — 1 of 3 peers returns a snapshot
  claiming a rotation that never happened → quorum fails,
  operator sees which peer disagreed.
- **AllPeersMalicious** — 3 peers agree but on a forged
  snapshot → bootstrap "succeeds" from the node's perspective
  (K-of-K doesn't protect here). Out of QDP-0008 scope; logged
  as NG1.

## 12. Metrics

```
quidnug_bootstrap_attempts_total{domain}
quidnug_bootstrap_success_total{domain}
quidnug_bootstrap_failure_total{domain, reason}
quidnug_bootstrap_peers_queried{domain}
quidnug_bootstrap_peers_agreed{domain}
quidnug_bootstrap_duration_seconds{domain}
quidnug_bootstrap_shadow_divergence_total{domain}
```

## 13. Alternatives considered

### 13.1 M-of-N with M < N (rejected for v1)

Accept 2-of-3 or majority-of-N. Simpler but weaker: 1 Byzantine
peer could dominate agreement with 1 honest collaborator.
K-of-K trades availability (any disagreement fails) for
safety (any disagreement is loud).

### 13.2 Snapshot provenance proof (deferred)

Require the snapshot to include a Merkle path linking its
contents to a block that's already quorum-verified. Much more
complex and depends on H2. Deferred — K-of-K is the v1
floor.

### 13.3 Lazy bootstrap (deferred)

Seed from one peer immediately, cross-verify asynchronously.
Rejected: defeats the whole safety argument. Node could be
accepting transactions against forged state while the
verification is still pending.

## 14. Open questions

1. **Trust list seeding vs. identity transactions.** The
   bootstrap trust list bypasses identity-transaction
   machinery. Is that safe? Answer: yes — trust list is
   operator-asserted root trust, same role as TLS roots.
   Identity transactions advance from there.
2. **Bootstrap for a new domain.** If the node is the FIRST
   node in a domain, there are no peers to query. Handling:
   return `BootstrapNoPeers` immediately; operator uses
   the empty-start path with the genesis block they created.
3. **Retry strategy.** On transient failure (fewer than K
   peers available), do we retry with backoff? Proposal:
   yes, expoonential backoff up to 1 hour, at which point
   fail the session and require manual restart.

## 15. References

- [QDP-0001: Global Nonce Ledger](0001-global-nonce-ledger.md) §7
- [QDP-0004: Phase H Roadmap](0004-phase-h-roadmap.md) §3.3
- [`internal/core/snapshot.go`](../../internal/core/snapshot.go)

---

**Review status.** Draft. Sign-off on K-of-K vs. M-of-N
default and bootstrap trust list handling before
implementation.
