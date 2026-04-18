# QDP-0001: Global Nonce Ledger

| Field         | Value                             |
|---------------|-----------------------------------|
| Status        | Draft                             |
| Track         | Protocol (hard fork)              |
| Author        | The Quidnug Authors               |
| Created       | 2026-04-18                        |
| Supersedes    | —                                 |
| Requires      | —                                 |
| Implements in | v2.0 (target)                     |

## 1. Summary

Today Quidnug's replay protection is a per-node, in-memory map of
`(truster, trustee) → last-seen nonce` for trust transactions, with ad-hoc
monotonic counters (`UpdateNonce`, `Sequence`) for identity and event
transactions, and no counter at all for title transactions. This design
document proposes replacing those mechanisms with a **single per-signer
monotonic nonce stream** whose authoritative state is reconstructed from
the blockchain itself, supplemented by **block-header checkpoints**,
**gossiped snapshots** for fast bootstrap, and **signed anchors** that let
signers explicitly reserve nonce ranges for key rotation and compromise
recovery.

Motivating attack scenario:

> An attacker captures a signed trust transaction `T` (with nonce `N`) that
> was accepted by the network six months ago. They wait for a new node `X`
> to join the network, then submit `T` to `X` before `X` has finished
> syncing. `X`'s in-memory nonce registry is empty, so it accepts `T` and
> rebroadcasts it. Depending on `X`'s trust tier, `T` may be re-processed
> on the network.

This scenario works today. After this proposal lands, it does not.

## 2. Background: current nonce handling

### 2.1 TrustTransaction

Defined in [src/core/types.go](../../src/core/types.go) as:

```go
type TrustTransaction struct {
    BaseTransaction
    Truster    string
    Trustee    string
    TrustLevel float64
    Nonce      int64  // monotonic per (Truster, Trustee) pair
    ...
}
```

Validation ([src/core/validation.go](../../src/core/validation.go)):

```go
if tx.Nonce <= 0 { return false }
currentNonce := 0
if trusterNonces, ok := node.TrustNonceRegistry[tx.Truster]; ok {
    if n, ok := trusterNonces[tx.Trustee]; ok { currentNonce = n }
}
if tx.Nonce <= currentNonce { return false }
```

`TrustNonceRegistry` is a `map[string]map[string]int64` held in process
memory on each node, rebuilt from blocks on startup.

### 2.2 IdentityTransaction.UpdateNonce

A per-quid counter held on the identity record itself. Enforces
monotonicity on identity updates ([validation.go:170](../../src/core/validation.go)).

### 2.3 EventTransaction.Sequence

Scoped per event stream ([validation.go:319](../../src/core/validation.go)):

```go
if streamExists {
    if tx.Sequence <= stream.LatestSequence { return false }
} else {
    if tx.Sequence != 0 && tx.Sequence != 1 { return false }
}
```

### 2.4 TitleTransaction

Has no nonce. `ExpiryDate` is the only time-bounding field.

### 2.5 Node-to-node auth

A separate HMAC-signed envelope ([auth.go](../../src/core/auth.go))
with a timestamp tolerance window, not a nonce. Out of scope for this doc,
but the design here does not conflict with it.

## 3. Problems

### 3.1 Fresh-join replay (CRITICAL)

Per §1. Any node whose `TrustNonceRegistry` is empty will accept a
transaction the network rejected long ago. This happens at every node
start before block sync completes, and every time a new node joins.

### 3.2 Per-pair scoping amplifies attack surface

The current design uses a nonce counter per `(truster, trustee)` pair.
If Alice has issued trust transactions to 100 different trustees, she has
100 independent nonce streams. An attacker capturing one
`(Alice → Bob, nonce=5)` transaction only needs to replay it into a window
where that specific pair's nonce is still ≤ 4. This gives attackers
100× more replay windows than a single per-signer counter would.

### 3.3 Partition-split nonce ambiguity (HIGH)

Under Proof-of-Trust, nodes diverge deliberately on which blocks they
accept. A block `B` observed only by partition `P1` advances nonce state on
`P1` but not on `P2`. A transaction reusing the same nonce as `B` is
rejected by `P1` but accepted by `P2`. When the partitions eventually
exchange state, the nonce stream is inconsistent.

### 3.4 No coverage for TitleTransaction (HIGH)

A signed title transfer can be replayed indefinitely. The `ExpiryDate`
field only prevents replays *after* expiry; any signed transfer with
expiry in the future is replayable.

### 3.5 No compromised-key recovery (MEDIUM)

If a quid's private key leaks, every historical signature is still valid
forever. There is no in-protocol way to say "invalidate everything I
signed before moment X." The only mitigation available today is
out-of-band key abandonment, which leaves all historic trust, identity,
and title state intact.

### 3.6 No key rotation (MEDIUM)

A quid's ID is the SHA-256 hash of its public key. Rotating the key
produces a different quid, breaking every existing relationship. This is
not a replay issue per se, but the solution — "version" the key — shares
mechanism with anchored nonce invalidation and belongs in the same
design.

### 3.7 Nonce-skip denial (MEDIUM)

A signer with access to their own key can publish a transaction with
`nonce = 2^62`. Every legitimate future transaction from that signer must
then exceed `2^62`, which constrains downstream tools (JSON numeric
precision breaks past `2^53`, and the logical "ordinal" becomes
meaningless). An attacker who briefly compromises a key can permanently
poison its nonce stream.

## 4. Goals and non-goals

### 4.1 Goals

- **G1.** A replay of any historical transaction against any honest node
  is rejected with high probability, regardless of that node's age or
  current sync state.
- **G2.** Nonce advancement during a network partition is consistent: a
  nonce consumed by a block observed in either partition is unusable in
  the other after partition heal.
- **G3.** Every transaction type (trust, identity, title, event) is
  covered by a single, uniform nonce mechanism.
- **G4.** A signer can publish a signed message that invalidates all of
  their prior signatures ("emergency anchor") — or a bounded subset
  ("rotation anchor") — without requiring network consensus on
  out-of-band authority.
- **G5.** Key rotation is supported: a quid can continue to be the same
  identity across a key change, with clear semantics for what an old-key
  signature means after rotation.
- **G6.** Bootstrap time for a new node is `O(minutes)` for typical
  network sizes, not `O(hours)`.
- **G7.** Migration from the current per-pair scheme is deterministic and
  testable.

### 4.2 Non-goals

- **N1.** Preventing a signer from intentionally burning their own nonce
  range. A signer with access to their own key can always do that.
- **N2.** Global total ordering of transactions across signers. Nonces
  are local to a signer; between-signer ordering remains the block's
  concern.
- **N3.** Social recovery / M-of-N guardian co-signing for anchors. This
  is a natural extension but is deliberately deferred to a later QDP to
  keep this proposal scoped.
- **N4.** Changing the signature scheme (still ECDSA-P256).
- **N5.** Soft-fork compatibility. Deploying this is a hard fork; nodes
  running pre-v2.0 are incompatible.

## 5. Threat model

Adversary capabilities:

| Capability                                             | In scope?        |
|--------------------------------------------------------|------------------|
| Observe all public network traffic                     | Yes              |
| Replay any historical transaction                      | Yes              |
| Delay or drop messages                                 | Yes              |
| Partition the network (Byzantine minority)             | Yes              |
| Compromise a minority of validator quids               | Yes              |
| Compromise a user's private key                        | Yes, recoverable |
| Break ECDSA-P256                                       | No               |
| Compromise a majority of validators in the target domain | No             |
| Predict future key material (bad RNG)                  | No               |

Honest-party assumptions:

- A fresh node bootstraps from at least one honest peer (or can verify
  snapshot agreement across multiple peers to detect disagreement).
- Clocks are loosely synchronized (within the existing
  `NodeAuthTimestampTolerance` window).
- Block signing keys are not compromised in bulk.

## 6. Design

### 6.1 Data model changes

#### 6.1.1 Per-transaction additions

Every transaction type gains a uniform envelope:

```go
type BaseTransaction struct {
    ID          string
    Type        TransactionType
    TrustDomain string
    Timestamp   int64
    Signature   string
    PublicKey   string

    // New fields (v2)
    SignerQuid  string  // explicit signer identity
    Nonce       int64   // per-signer monotonic (moved from TrustTransaction)
    KeyEpoch    uint32  // which key version; 0 is the genesis key
}
```

Notes:

- `Nonce` moves from `TrustTransaction` to `BaseTransaction`. Identity
  and event transactions drop their bespoke counters (`UpdateNonce`,
  `Sequence`) as *replay* protection but keep them as *ordering* hints
  (see §6.6).
- `SignerQuid` makes the signer explicit. Today it is inferred from
  `PublicKey` and the transaction type; making it explicit removes a
  class of parser ambiguity and lets validators look up the ledger entry
  in one step.
- `KeyEpoch` allows rotation (§6.5).

#### 6.1.2 New persistent structure: `NonceLedger`

```go
type NonceLedger struct {
    // Max nonce observed per (signer, keyEpoch) across the Trusted chain.
    accepted  map[NonceKey]int64

    // Max nonce observed across Trusted ∪ Tentative blocks.
    // Used to reject transactions that would collide with tentative txs.
    tentative map[NonceKey]int64

    // Active key epoch per signer. Writes here come from KEY_ROTATION
    // anchors only.
    currentEpoch map[string]uint32

    // Guarded by a single RWMutex. See §8.
    mu sync.RWMutex
}

type NonceKey struct {
    Quid  string
    Epoch uint32
}
```

`accepted` is the authoritative counter. `tentative` is an upper bound
used for validation only; it is rebuilt from `accepted` plus the current
tentative-block pool on each tier transition.

#### 6.1.3 Block-header checkpoints

Every block gains a `NonceCheckpoints` field summarizing the per-signer
nonce advance caused by that block:

```go
type Block struct {
    Index        int64
    Timestamp    int64
    Transactions []interface{}
    TrustProof   TrustProof
    PrevHash     string
    Hash         string

    // New (v2)
    NonceCheckpoints []NonceCheckpoint
}

type NonceCheckpoint struct {
    Quid     string
    Epoch    uint32
    MaxNonce int64  // max nonce for this (quid, epoch) in this block's txs
}
```

The checkpoint is computed by the block producer at seal time and signed
as part of the block. It is both:

- A commitment: "this block advances signer `Q` to nonce `N`."
- A compact audit artifact: a node syncing only headers can still
  reconstruct the `accepted` map without replaying transactions.

### 6.2 Validation algorithm

```
VALIDATE-TX(tx):
    1. Canonical checks: signature, quid ID format, field bounds, etc.
       (Existing validation, unchanged.)

    2. Look up signer's active key epoch E_active from ledger.currentEpoch.
       If tx.KeyEpoch > E_active: REJECT (future-epoch).
       If tx.KeyEpoch < E_active:
           Accept only if there exists an anchor for E_active that
           explicitly permits tx.KeyEpoch up to a maxNonce bound, and
           tx.Nonce ≤ that bound.
           Otherwise REJECT (stale-epoch).

    3. key = (tx.SignerQuid, tx.KeyEpoch)
       a. If tx.Nonce ≤ ledger.accepted[key]:   REJECT (replay).
       b. If tx.Nonce ≤ ledger.tentative[key]:  REJECT (reserved).
       c. If tx.Nonce - max(accepted[key], 0) > MaxNonceGap:
              REJECT (nonce-skip denial, §3.7).

    4. All other domain-specific validations (trust level in range,
       identity creator matches, title ownership totals, etc.).

    5. Admit to mempool. Reserve: ledger.tentative[key] = tx.Nonce.
```

`MaxNonceGap` is a network-wide parameter. Proposal: `1024`. Large enough
for reasonable burstiness, small enough that an attacker with a single
compromised signature cannot poison the stream up to astronomical
values.

### 6.3 Block-production validation

When a validator seals a block `B`:

```
SEAL-BLOCK(B):
    per_signer_max := {}
    for tx in B.Transactions:
        key = (tx.SignerQuid, tx.KeyEpoch)
        per_signer_max[key] = max(per_signer_max[key], tx.Nonce)

        // Enforce in-block strict monotonicity per signer.
        // Two txs from the same signer in the same block must have
        // strictly increasing nonces, in transaction-order.
        if seen[key] exists and tx.Nonce <= seen[key]:
            REJECT-BLOCK
        seen[key] = tx.Nonce

    B.NonceCheckpoints = []
    for (key, nonce) in per_signer_max:
        B.NonceCheckpoints.append(NonceCheckpoint{
            Quid: key.Quid,
            Epoch: key.Epoch,
            MaxNonce: nonce,
        })

    Sort B.NonceCheckpoints by (Quid, Epoch) for deterministic hashing.
```

`NonceCheckpoints` is included in the block's signable data (analogous
to how `TrustProof` is handled in
[crypto.go:16-43](../../src/core/crypto.go)).

### 6.4 Tier interaction (Trusted / Tentative / Untrusted)

Quidnug's Proof-of-Trust assigns each received block one of four
acceptance tiers ([types.go:181-186](../../src/core/types.go)). Nonce
semantics per tier:

| Tier       | Effect on `accepted` | Effect on `tentative` |
|------------|----------------------|-----------------------|
| Trusted    | Set to max(current, checkpoint) | Set to max(current, checkpoint) |
| Tentative  | Unchanged                       | Set to max(current, checkpoint) |
| Untrusted  | Unchanged                       | Unchanged                       |
| Invalid    | Unchanged                       | Unchanged                       |

Promotion (Tentative → Trusted) and demotion (Tentative → GC) update
`accepted` / `tentative` respectively. Tentative-block pruning (see
audit backlog) drops the checkpoint's contribution to `tentative` only
if no other tentative block referenced that signer at an equal or
higher nonce.

This rule directly resolves §3.3: a nonce used by a tentative block in
either partition is reserved everywhere, so the two partitions cannot
independently consume the same nonce.

### 6.5 Nonce anchors

An anchor is a standalone signed message — not wrapped in a transaction:

```go
type NonceAnchor struct {
    Kind               AnchorKind // "rotation" | "invalidation" | "epoch-cap"
    SignerQuid         string
    FromEpoch          uint32
    ToEpoch            uint32      // same as FromEpoch for "invalidation"
    NewPublicKey       string      // used only for "rotation"; hex SPKI
    MinNextNonce       int64       // next valid tx nonce in ToEpoch
    MaxAcceptedOldNonce int64      // the highest nonce in FromEpoch the
                                   // network should still honor
    ValidFrom          int64       // unix seconds
    Nonce              int64       // anchors are themselves nonced; must
                                   // strictly exceed the anchor-nonce
                                   // stored in ledger for this signer
    Signature          string
}

type AnchorKind int
const (
    AnchorRotation AnchorKind = iota + 1
    AnchorInvalidation
    AnchorEpochCap
)
```

Anchor processing, in ascending order of authority:

- **AnchorEpochCap.** Caps the old epoch: "no transaction with
  `KeyEpoch == FromEpoch` and `Nonce > MaxAcceptedOldNonce` is valid."
  Does not rotate keys; does not introduce a new epoch.

- **AnchorInvalidation.** Same as above with `MaxAcceptedOldNonce`
  set to `ledger.accepted[(signer, FromEpoch)]`. All *future* uses of
  that epoch are blocked. Effective "freeze" of an epoch.

- **AnchorRotation.** Introduces a new epoch. All future transactions
  from this signer must use `KeyEpoch == ToEpoch` and be signed with
  `NewPublicKey`. Transactions in `FromEpoch` are bounded by
  `MaxAcceptedOldNonce`. Must be signed by the old key (epoch
  `FromEpoch`) to prove authorization.

Anchors are broadcast like blocks, sealed into a block's transactions
list, and apply to the ledger at the point of their containing block's
Trusted acceptance. The anchor's own `Nonce` field is a per-signer
**anchor-nonce**, stored in a separate sub-map so that a key
compromise can't simultaneously max out both the regular nonce and the
anchor nonce.

Rationale for the three-tier anchor system:

- `AnchorEpochCap` lets a cautious signer publish a ceiling without
  abandoning the key — useful as a periodic "watermark" that limits
  blast radius of a future compromise.
- `AnchorInvalidation` is the emergency button. Nothing new is signable
  under the old key without rotation.
- `AnchorRotation` is the graceful upgrade path.

Known limitation: a compromised key *can* publish an `AnchorInvalidation`
of its own, preventing the legitimate user from using the key going
forward. This is the correct behavior — compromise means the attacker
has the legitimate user's abilities. What this proposal explicitly does
not try to solve is racing *between* the attacker and legitimate user
to publish an `AnchorRotation` first; a future QDP on guardian-based
recovery is the right venue for that.

### 6.6 Per-type sequencing becomes ordering-only

- `IdentityTransaction.UpdateNonce` is retained as an application-level
  version number (useful for clients deduplicating identity records)
  but **not** used for replay protection. Replay is now covered by
  `BaseTransaction.Nonce`.
- `EventTransaction.Sequence` is retained as the per-stream linearized
  sequence, which is still required so that event streams have a total
  order independent of the signer's overall nonce stream. Replay
  protection shifts to `Nonce`.
- `TitleTransaction` gains replay protection via `Nonce`, which it
  currently lacks.

## 7. Nonce snapshots: fast bootstrap

### 7.1 Snapshot format

```go
type NonceSnapshot struct {
    SchemaVersion int
    BlockHeight   int64
    BlockHash     string
    Timestamp     int64
    TrustDomain   string
    Entries       []NonceSnapshotEntry
    ProducerQuid  string
    Signature     string
}

type NonceSnapshotEntry struct {
    Quid     string
    Epoch    uint32
    MaxNonce int64
}
```

### 7.2 Production

A node whose `TrustDomain` it validates for publishes a signed snapshot
every `SnapshotInterval` blocks (proposal: 64). The snapshot is a pure
derivative of `accepted` at `BlockHeight` and is deterministic: two
honest nodes at the same height publish byte-identical snapshots (modulo
the producer's own ID and signature).

### 7.3 Consumption (fresh-join bootstrap)

A joining node:

1. Fetches the most recent `NonceSnapshot` from ≥ `K` peers (proposal:
   `K = 3`). It verifies each snapshot's signature and the producer's
   quid against its seed trust set.
2. Requires that all `K` snapshots agree on `BlockHash`, `BlockHeight`,
   and every `(Quid, Epoch, MaxNonce)` tuple it examines. Any
   disagreement falls back to full block sync for that signer.
3. Seeds its `ledger.accepted` from the agreed-upon snapshot.
4. Begins validating incoming transactions immediately; continues to
   backfill blocks in the background. Transactions with
   `nonce ≤ accepted[key]` are safely rejected from the first moment of
   operation.

Fallback: if a bootstrapping node cannot reach `K` agreeing peers, it
MUST fall back to full block sync. Refusal to fall back would let a
single malicious peer feed the new node a low-nonce snapshot and then
replay transactions against it.

## 8. Storage and performance

### 8.1 Memory

For `N` active `(signer, epoch)` pairs, `accepted` is a map of
`NonceKey → int64`. At 16-byte quid IDs plus 4-byte epoch, the Go map
overhead puts each entry at ~64 bytes. One million active signers per
domain ≈ 64 MB resident. This is comparable to what the current
`TrustNonceRegistry` uses at similar network sizes (and strictly smaller,
since we've collapsed per-pair to per-signer).

At 100M signers, sharding the ledger map by `hash(quid) mod 64` cuts
contention and makes the per-shard footprint tractable. This is a
near-free refactor and the 64-shard layout matches existing recommended
Go map-sharding patterns.

### 8.2 Persistence

The ledger is rebuildable from blocks, so durability is technically
optional. In practice we persist it anyway, for fast restart:

- `nonce_ledger.json` alongside `pending_transactions.json` in
  `DataDir`, following the pattern already used by
  [persistence.go](../../src/core/persistence.go).
- Written atomically (temp file + rename). On startup, load + verify
  against the last `W` block headers (proposal: `W = 256`). A verify
  mismatch triggers a full rebuild.

### 8.3 CPU

Validation cost per transaction: two map lookups + a constant-time
comparison. Strictly cheaper than the current per-pair two-level map
walk, because the current implementation also does a second map access
plus a zero-check branch.

Block sealing cost: `O(txs_per_block)` to build the per-signer max map,
`O(S log S)` to sort the checkpoint slice where `S` is distinct signers
in the block. For 10k txs/block and ~1k distinct signers, this is
sub-millisecond on a modern CPU.

### 8.4 Bloom filter

Optional addition: a per-signer bloom filter of "I have recently seen
exactly `(signer, epoch, nonce)`" accelerates the mempool's dedup path.
This is an optimization and is deliberately omitted from the hard
design here; it can be added without a protocol change.

## 9. Protocol wire changes

New or changed JSON fields. Existing field semantics not listed are
unchanged. "New" means added relative to v1.x; "renamed" means moved
(old field still accepted during migration, see §11).

Transactions (all types):

```json
{
  "id": "...",
  "type": "TRUST",
  "trustDomain": "example.com",
  "timestamp": 1760000000,
  "signature": "...",
  "publicKey": "04...",

  "signerQuid": "aaaaaaaaaaaaaaaa",   // new
  "nonce": 42,                        // moved here from TRUST body
  "keyEpoch": 0,                      // new

  ...type-specific fields...
}
```

Blocks:

```json
{
  "index": 123,
  "timestamp": 1760000000,
  "transactions": [...],
  "trustProof": {...},
  "prevHash": "...",
  "hash": "...",

  "nonceCheckpoints": [            // new
    {"quid": "aaaa...", "epoch": 0, "maxNonce": 42},
    {"quid": "bbbb...", "epoch": 1, "maxNonce": 17}
  ]
}
```

Nonce anchor (standalone):

```json
{
  "kind": "rotation",
  "signerQuid": "aaaaaaaaaaaaaaaa",
  "fromEpoch": 0,
  "toEpoch": 1,
  "newPublicKey": "04...",
  "minNextNonce": 1,
  "maxAcceptedOldNonce": 42,
  "validFrom": 1760000000,
  "nonce": 1,
  "signature": "..."
}
```

Nonce snapshot:

```json
{
  "schemaVersion": 1,
  "blockHeight": 12345,
  "blockHash": "...",
  "timestamp": 1760000000,
  "trustDomain": "example.com",
  "entries": [
    {"quid": "aaaa...", "epoch": 0, "maxNonce": 42}
  ],
  "producerQuid": "...",
  "signature": "..."
}
```

New HTTP endpoints (versioned under `/api/v2`):

- `GET /nonce-snapshots/latest?domain=example.com` — returns the most
  recent snapshot this node has produced or received.
- `GET /nonce-snapshots?domain=example.com&fromHeight=N` — paginated
  list, for bootstrap.
- `POST /anchors` — submit a nonce anchor to the mempool.

## 10. Migration from v1

A clean cut is the cheapest path; soft-fork compatibility is explicitly
not a goal (§4.2).

### 10.1 Phase 0 — v1.6 (opt-in, warning-only)

- Nodes emit `NonceCheckpoints` in newly-sealed blocks.
- Validators parse `NonceCheckpoints` if present but do **not** enforce.
- Nodes emit `nonce_ledger.json` snapshots but do not consume them.
- Anchors and snapshots are accepted at the wire level but ignored.
- A prometheus counter `quidnug_nonce_replay_rejections_would_be` is
  incremented whenever the v2 rules *would* have rejected a transaction.
  Operators use this to validate correctness before cutover.

### 10.2 Phase 1 — v2.0 (hard fork)

- `NonceCheckpoints` is required on every block.
- Ledger enforcement is active.
- `TrustTransaction.Nonce` (old location) is rejected; transactions must
  use `BaseTransaction.Nonce`.
- Existing `IdentityTransaction.UpdateNonce` and
  `EventTransaction.Sequence` are demoted to ordering-only.

#### 10.2.1 One-shot migration of existing state

At the hard-fork block `H`:

```
For each signer Q that appears in pre-H blocks:
    old_max = max across all (Truster=Q → Trustee=T) nonces in
              pre-H TrustNonceRegistry, for any T.
    old_id_max = max UpdateNonce observed for Q's identity records.
    old_ev_max = max Sequence observed for Q as author in event streams.

    seed_nonce = max(old_max, old_id_max, old_ev_max) + 1
    ledger.accepted[(Q, 0)] = seed_nonce - 1
```

This guarantees that no pre-H transaction remains replayable, at the
cost of advancing every signer's nonce counter by one. (Legitimate
signers simply resume from `seed_nonce`.)

The migration is deterministic — every honest node performs the same
computation on the same pre-H state and arrives at the same ledger.
Non-determinism here would be a consensus bug, so this function must
have test coverage before cutover (§13.3).

### 10.3 Phase 2 — v2.1 (anchor hardening)

- `AnchorRotation` introduced.
- Guardian co-signing deferred to a follow-up QDP.

### 10.4 Rollback

If a critical defect is discovered post-v2.0, a rollback hard-fork
returns to v1 semantics from block `R`. All transactions submitted in
`[H, R)` are invalidated. This is expensive and disruptive; it is
mentioned here only so operators understand the cost.

## 11. Security analysis

Threat → mitigation mapping:

| Threat (§3 reference)               | Mitigation                                     | Residual risk |
|--------------------------------------|------------------------------------------------|---------------|
| Fresh-join replay (§3.1)             | Ledger seeded from snapshot or block sync      | Bootstrap from `<K` honest peers (§7.3 fallback); user error |
| Per-pair amplification (§3.2)        | Single per-signer counter                      | None |
| Partition-split ambiguity (§3.3)     | Tentative-tier reserves nonces                 | Long-lived partitions may hold nonce space; resolved at merge |
| No title replay protection (§3.4)    | Uniform `BaseTransaction.Nonce`                | None |
| No compromise recovery (§3.5)        | `AnchorInvalidation` and `AnchorRotation`      | Attacker races the owner to publish an anchor (§6.5 known limitation) |
| No key rotation (§3.6)               | `AnchorRotation` + `KeyEpoch`                  | Same race condition |
| Nonce-skip denial (§3.7)             | `MaxNonceGap` cap                              | Attacker can still consume up to 1024 nonces at a time |
| Snapshot poisoning                   | K-of-K agreement or fall back to full sync     | K cooperating malicious peers (rare but possible) |
| Anchor-spam DoS                      | Anchor-nonce is itself monotonic per signer    | Normal block-level spam controls (rate limits, body size, etc.) apply |

### 11.1 Formal invariants

The design is intended to enforce the following invariants at all times
on an honest node's `ledger`:

- **I1 (Strict monotonicity).** For every `(signer, epoch)`, accepted
  nonces form a strictly increasing sequence.
- **I2 (Tentative dominates accepted).** For every key,
  `tentative[key] ≥ accepted[key]`.
- **I3 (Checkpoint consistency).** After applying a Trusted block `B`
  with checkpoints `C`, for every `(q, e) ∈ C`:
  `accepted[(q, e)] ≥ C[(q, e)]`.
- **I4 (Epoch monotonicity).** `currentEpoch[q]` is non-decreasing and
  only advances via a `Trusted` `AnchorRotation`.
- **I5 (No retrograde anchor).** For every new anchor, the anchor's
  `Nonce` exceeds the stored anchor-nonce for its signer.

These invariants are candidates for property-based tests (§13.3).

## 12. Alternatives considered

- **Keep per-pair nonces + gossip.** Equivalent to Ethereum's mempool
  nonce dissemination. Doesn't solve fresh-join replay unless gossip is
  authoritative, and making gossip authoritative is equivalent to
  introducing a ledger — so this reduces to the same design with
  weaker guarantees.

- **UUID-per-transaction + dup rejection.** Simple, but an attacker can
  generate infinitely many fresh UUIDs over old payloads. Only prevents
  *literal* duplicate submissions, not semantic replays.

- **Timestamp-only gating.** Reject anything with `timestamp <
  last_seen - δ`. Clock-skew sensitive; tricking a node by issuing
  future-dated signatures makes all honest past transactions un-
  submittable. Unworkable.

- **Per-transaction Merkle-tree nonce commitments.** Each signer
  maintains a deterministic sparse Merkle tree of consumed nonce
  slots. Elegant but overkill for replay protection; the verification
  cost dominates the actual attack surface we need to close.

- **Global sequencer.** A designated node (or rotating leader) assigns
  a monotonic global ID to every transaction. Works beautifully for
  replay prevention and for debugging. Incompatible with Quidnug's
  Proof-of-Trust premise of deliberately divergent per-node chains.

- **"Nonce as hash of previous transaction".** Every new transaction's
  nonce is `H(prev_tx_id)`, forming a per-signer chain. Compact, elegant,
  but forces strict in-order submission: if the signer has transactions
  `A, B, C` in flight and `B` is lost, `C` is un-submittable until `B` is
  re-signed. Rejected for ergonomics.

## 13. Test plan

Test coverage is load-bearing for this proposal because the migration
function (§10.2.1) must be deterministic across all honest nodes or
consensus breaks.

### 13.1 Unit tests

- `ledger.Accept` for fresh signer, repeat nonce, gap, gap-too-large,
  wrong epoch.
- `ledger.ReserveTentative` lifecycle: reserve, promote, demote.
- Anchor processing: each of the three anchor kinds, retrograde
  anchor-nonce rejection, rotation with a forged "new" key.
- Snapshot production determinism: same height + same accepted map
  produces byte-identical snapshots on two different nodes.

### 13.2 Integration tests

- **Fresh-join replay.** Two-node cluster; capture a TrustTransaction
  from `accepted`; start a third node from blank state, seed via
  snapshot; verify the captured transaction is rejected.
- **Partition-heal.** Start 4 nodes, split into two partitions of 2,
  submit conflicting-nonce transactions in each partition, heal; verify
  the merged ledger rejects the duplicate nonce.
- **Rotation under attacker load.** Legitimate owner publishes a
  rotation anchor while the attacker simultaneously floods old-epoch
  transactions; verify the attacker's transactions stop being accepted
  immediately after the anchor is Trusted.

### 13.3 Property-based tests

Generate random transaction streams and verify the invariants
enumerated in §11.1 hold on every reachable state. Use Go's native
`testing.F` fuzz harness seeded with the invariants as oracles.

### 13.4 Migration tests

- Synthetic pre-v2 block history of 10k blocks × 1k txs; run the
  migration function; assert that the resulting `accepted` map matches
  an independently-computed reference. Required pass rate: 100%,
  byte-equal.

### 13.5 Performance benchmarks

- `BenchmarkLedgerAccept` at 1k, 100k, 10M signer counts.
- `BenchmarkBlockSeal` with 1k, 10k txs/block.
- `BenchmarkSnapshotProduce` / `BenchmarkSnapshotVerify`.
- Target: end-to-end validation latency within 2× of the v1 per-pair
  implementation at equivalent network size.

## 14. Rollout plan

### 14.1 Engineering milestones

| Milestone | Gate                                                    |
|-----------|---------------------------------------------------------|
| M1        | Ledger data structure + unit tests behind a build tag   |
| M2        | Block-header checkpoint serialization (Phase 0, warn-only) |
| M3        | Migration function + tests (§13.4)                      |
| M4        | Snapshot production + peer consumption                  |
| M5        | Anchor types, processing, HTTP endpoint                 |
| M6        | Phase-1 enforcement behind a config flag                |
| M7        | Removal of v1 per-pair code path                        |

### 14.2 Network-level rollout

- **T-0:** ship v1.6 (Phase 0) to a test domain. Observe
  `would_be_rejected` metric for at least 2 weeks. Investigate every
  spike.
- **T+30d:** publish the fork block `H` at least 14 days in advance of
  cutover. Include it in release notes and the public
  [docs/roadmap.md](../roadmap.md).
- **T+44d:** `H` reached. v2.0 nodes enforce; v1 nodes stop being able
  to interop for transactions. Operators must upgrade.
- **T+90d:** all known v1 nodes have upgraded or are quarantined.
  Release v2.1 with anchors.

### 14.3 Observability

New metrics:

- `quidnug_nonce_replay_rejections_total{reason="stale"|"reserved"|"gap"}`
- `quidnug_ledger_entries{tier="accepted"|"tentative"}`
- `quidnug_snapshot_production_duration_seconds`
- `quidnug_snapshot_verify_disagreements_total{peer=...}`
- `quidnug_anchor_applied_total{kind=...}`

Alert: `snapshot_verify_disagreements_total` rising indicates either a
bug, a peer's clock is off, or an attempted snapshot-poisoning attack.

## 15. Open questions

1. **Guardian-based recovery.** `AnchorRotation` signed by an M-of-N
   guardian set, declared at quid creation or via a prior anchor. Do
   we want this in v2.0 or defer to v2.2? *Proposal: defer. Scoping it
   properly requires its own QDP — key management, guardian rotation,
   loss recovery, malicious guardian collusion.*

2. **Snapshot interval.** 64 blocks is a starting number. At typical
   block intervals of 60s, that's an hour between snapshots. Should
   high-churn domains override? *Proposal: per-domain config, default
   64, minimum 8.*

3. **Should anchors also commit to a block-height bound?** A long-dated
   anchor sitting un-submitted in a mempool for weeks is surprising.
   *Proposal: require `validFrom` within 30 days of current block
   timestamp at the moment of inclusion; reject otherwise.*

4. **Compact representation for `NonceCheckpoints`.** A block with
   10k transactions from 500 distinct signers carries 500 checkpoint
   entries × ~30 bytes = 15 KB overhead per block. At 60-second block
   intervals and month-long retention, that's ~650 MB per node just for
   checkpoints. Worth compressing? *Proposal: yes, varint encoding; a
   later optimization PR.*

5. **Cross-domain nonce reuse.** Today a quid that is a signer in
   multiple `TrustDomain`s has a single identity. Is their nonce
   stream global or per-domain? *Proposal: global. Per-domain
   semantics would let a compromised key in one domain not affect the
   others, which sounds attractive, but it also means a user has to
   track `N` nonce streams and makes anchors domain-scoped.
   Complexity buys little over the guardian-recovery design in Q1.*

## 16. References

- [Quidnug architecture](../architecture.md)
- [Rogue-node security model](../rogue-node-security.md)
- [src/core/validation.go](../../src/core/validation.go) — current
  nonce checks
- [src/core/registry.go:189](../../src/core/registry.go) — current
  nonce update
- [src/core/auth.go](../../src/core/auth.go) — node-auth timestamp
  tolerance (orthogonal)
- RFC 6979 (Deterministic ECDSA) — signing behavior unchanged
- Ethereum Yellow Paper, §4.1 (transaction nonce semantics) —
  prior-art comparison
- Heidhues et al., *"Replay Attack Detection in Distributed Ledger
  Systems"* (2023) — survey of replay-protection patterns

---

**Review status.** This document is a draft. Before merging to `main`:
comments from at least two maintainers on (a) the migration function
in §10.2.1, (b) the tentative-tier reservation rule in §6.4, and (c)
the anchor-race limitation in §6.5. File issues as
`design:nonce-ledger:<section>`.
