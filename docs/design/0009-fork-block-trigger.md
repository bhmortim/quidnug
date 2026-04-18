# QDP-0009: Fork-Block Migration Trigger (H5)

| Field      | Value                                                |
|------------|------------------------------------------------------|
| Status     | Draft                                                |
| Track      | Protocol                                             |
| Author     | The Quidnug Authors                                  |
| Created    | 2026-04-18                                           |
| Requires   | QDP-0001 §10                                         |
| Implements | Phase H5 of QDP-0004 roadmap                         |
| Target     | v2.5                                                 |

## 1. Summary

QDP-0001 §10 described a shadow → enforce rollout: configured
flags like `EnableNonceLedger` are off by default; operators
flip them per-node. For a production network, feature
activation must be coordinated across all nodes at a specific
block height so consensus is preserved. A mismatched flip
means one node rejects a transaction another accepts — chain
fork.

This document specifies `ForkBlockTransaction`, a special tx
type that declares "at block height H, every node honoring
this transaction flips feature F." Once the transaction lands
in a Trusted block and the height arrives, the node's
behavior changes uniformly.

## 2. Background — what we have today

- Feature flags (`EnableNonceLedger`, `EnableLazyEpochProbe`,
  `EnablePushGossip`, `EnableKofKBootstrap`) are per-node
  config booleans.
- Flipping a flag on one node while others have it off either
  (a) has no consensus effect because the feature is
  receiver-only (push gossip receive, probes), or (b) risks
  consensus divergence (nonce ledger enforcement).
- For category (b), the only safe coordination today is
  "everyone flips at the same wall-clock time," which is
  brittle and a production hazard.

## 3. Problem statement

An operator decides it's time to enforce the nonce ledger in a
network of 10 validators. Options today:

1. SSH each node, flip the flag, restart. Good luck
   coordinating within the block time (60s default). Any node
   that flips "early" starts rejecting transactions the others
   accept → fork.
2. Do a scheduled maintenance window, shut everything, flip
   flags, restart. Downtime the network can ill afford.
3. Accept inconsistency during the window. Recover afterward.

Fork-block transactions are the on-chain version: a single
signed transaction declaring "at block height H, every node
that has accepted this transaction into its Trusted chain will
start enforcing feature F." Block H becomes the synchronization
boundary.

## 4. Goals and non-goals

**Goals.**

- **G1.** A single submitted `ForkBlockTransaction` coordinates
  feature activation across all participating nodes at a named
  future block height.
- **G2.** Pre-fork blocks are validated under old rules;
  post-fork blocks under new rules. The transition is
  deterministic and node-local.
- **G3.** Authorization requires a quorum of domain validators
  — prevents a single rogue validator from triggering a fork.
- **G4.** Forks scheduled too soon are rejected (`ForkHeight`
  must be at least 24h worth of blocks in the future) —
  coordination requires notice.
- **G5.** Superseding forks: a later `ForkBlockTransaction`
  with the same `Feature` but different `ForkHeight` is accepted
  only if it arrives strictly before the earlier height, so
  operators can push a fork out if there's a problem.
- **G6.** Observability: each node logs fork acceptance, the
  scheduled height, and the actual transition at that height.

**Non-goals.**

- **NG1.** Reversible forks. Once a fork lands at height H
  and the node transitions, there's no "un-fork." Rolling back
  is a separate, heavier protocol.
- **NG2.** Per-domain forks. Features are global; activating
  them in one domain but not another would itself be a split.
  Reject as out-of-scope.
- **NG3.** Cryptographic guarantees about which validators
  actually signed. We rely on the validator's private key as
  authorization evidence, same as any other signed tx.

## 5. Threat model

| Threat                                                                 | Mitigation                                                                                                    |
|------------------------------------------------------------------------|---------------------------------------------------------------------------------------------------------------|
| Rogue validator submits a fork-block transaction                       | Requires signatures from a configured quorum of validators, not a single signer.                              |
| Fork transaction with past ForkHeight                                  | Validation rejects ForkHeight ≤ currentHeight.                                                                |
| Fork transaction with ForkHeight too soon                              | Validation requires ForkHeight ≥ currentHeight + MinForkNoticeBlocks.                                         |
| Competing forks (same feature, different heights)                      | Second fork must arrive before earlier ForkHeight; it supersedes. After the earlier ForkHeight has passed, the fork is historical fact.        |
| Replay of an old fork transaction                                      | `ForkNonce` per (domain, feature) stream prevents replay. Same pattern as anchor nonces.                      |
| Unknown feature name                                                   | Nodes validate feature against a built-in allow-list. Forks with unknown features are rejected as malformed.  |
| Majority of validators maliciously fork at a bad time                  | Out of scope. K-of-K trust is the operator's responsibility. Minority forks their own chain via tentative.     |

## 6. Data model

### 6.1 Wire format

```go
const AnchorForkBlock AnchorKind = AnchorGuardianResign + 1 // = 9

const TxTypeForkBlock TransactionType = "FORK_BLOCK"

type ForkBlockTransaction struct {
    BaseTransaction
    Fork ForkBlock `json:"fork"`
}

type ForkBlock struct {
    Kind        AnchorKind `json:"kind"`
    TrustDomain string     `json:"trustDomain"`
    Feature     string     `json:"feature"`          // e.g. "enable_nonce_ledger"
    ForkHeight  int64      `json:"forkHeight"`       // absolute block index at which activation happens
    ForkNonce   int64      `json:"forkNonce"`        // per-(domain, feature) monotonic
    ProposedAt  int64      `json:"proposedAt"`       // unix ts when signed
    ExpiresAt   int64      `json:"expiresAt,omitempty"`
    Signatures  []ForkSig  `json:"signatures"`       // validator quorum signatures
}

type ForkSig struct {
    ValidatorQuid string `json:"validatorQuid"`
    KeyEpoch      uint32 `json:"keyEpoch"`
    Signature     string `json:"signature"`
}
```

### 6.2 Supported features

```go
var ForkSupportedFeatures = map[string]bool{
    "enable_nonce_ledger":      true,
    "enable_push_gossip":       true,
    "enable_lazy_epoch_probe":  true,
    "enable_kofk_bootstrap":    true,
    "require_tx_tree_root":     true, // will be used by H2
}
```

### 6.3 Ledger state

```go
// pendingForks[domain][feature] is the accepted but not-yet-
// activated fork. When the block at ForkHeight is processed,
// the node transitions the feature.
pendingForks map[string]map[string]*ForkBlock

// activeForks[domain][feature] is the feature state after
// activation. Kept for audit.
activeForks map[string]map[string]*ForkBlock
```

## 7. Validation rules

`ValidateForkBlock` checks:

1. **Kind** is `AnchorForkBlock`.
2. **Feature** is in `ForkSupportedFeatures`.
3. **Domain** matches a known trust domain.
4. **ForkNonce** strictly increases over any prior accepted
   fork for the same `(domain, feature)`.
5. **ForkHeight** is at least `currentHeight +
   MinForkNoticeBlocks` (default: 1440 blocks ≈ 24h).
6. **ExpiresAt**, when set, is after `ProposedAt`.
7. **Validator quorum**: signatures from at least
   `DomainValidatorQuorum` (e.g., 2/3 of the declared
   validators for the domain) are present. Each signature
   validates against the signer's current-epoch key.
8. **No duplicate signers**.

### 7.1 Superseding rules

- A later fork arriving before the earlier `ForkHeight` is
  accepted when its `ForkNonce` is strictly higher AND it
  references the same feature. The earlier entry is replaced.
- A later fork arriving AFTER the earlier `ForkHeight` is
  rejected: the earlier fork has already activated; a new
  fork would be "re-activation" (semantically no-op) or
  "override an already-committed change" (not allowed).

## 8. Activation path

At block commit, after `processBlockTransactions` applies a
`ForkBlockTransaction`, the fork is stored in `pendingForks`.

Block-processing hooks the fork check at the END of each
block's processing:

```
onBlockCommitted(block):
    processBlockTransactions(block)
    for each (domain, feature, fork) in pendingForks:
        if block.Index >= fork.ForkHeight AND domain == block.TrustDomain:
            activateFeature(feature)
            move fork → activeForks
            log + metric
```

Activation is idempotent: moving to `activeForks` twice is a
no-op.

### 8.1 Feature activation semantics

`activateFeature("enable_nonce_ledger")` sets
`node.NonceLedgerEnforce = true`. Symmetric for other features.

A node that has NOT yet applied the fork transaction (e.g.,
joined after the fork block committed) finds it during block
replay — the moment `processBlockTransactions` runs on the
fork-transaction block, the fork is recorded in
`pendingForks`; the catch-up loop checks `pendingForks` against
its current block index and immediately triggers activation if
past-height.

## 9. HTTP surface

```
POST /api/v2/fork-block       — submit a signed ForkBlockTransaction
GET  /api/v2/fork-block/status — list pending + active forks
```

Submit validates, enqueues as pending tx. The usual block-
production flow picks it up.

## 10. Migration

Additive:

1. **v2.4.0-alpha** — new AnchorKind and transaction type
   land. Nodes that don't recognize them reject as invalid
   (existing behavior); mixed networks don't fork.
2. **v2.5.0** — no flag change. The feature is "always
   active" in the sense that it interprets fork-block
   transactions; whether any specific fork has been requested
   is orthogonal.

## 11. Test plan

### 11.1 Unit tests

- **ValidateForkBlock_HappyPath** — valid tx with quorum.
- **ForkBlock_UnknownFeature** — rejected.
- **ForkBlock_PastHeight** — rejected.
- **ForkBlock_TooSoon** — rejected.
- **ForkBlock_NonceReplay** — rejected.
- **ForkBlock_BelowQuorum** — fewer signatures than required.
- **ForkBlock_DuplicateSigner** — rejected.
- **ForkBlock_Supersedes** — second fork with higher nonce and
  a later-but-still-before-first-height overrides.
- **ForkBlock_LateSupersedeRejected** — after first ForkHeight
  passed, new fork rejected.

### 11.2 Activation tests

- **Activation_AtForkHeight** — block at height H triggers
  `activateFeature`; `NonceLedgerEnforce` flips from false
  to true.
- **Activation_CatchUp** — node replays a chain where fork tx
  committed at height 100 and current index is 200; activation
  fires during replay.
- **Activation_Idempotent** — applying the same fork twice
  does not toggle anything back.

### 11.3 HTTP tests

- **POST /api/v2/fork-block** — valid tx enqueued.
- **GET status** — shows pending + active forks.

## 12. Metrics

```
quidnug_fork_block_accepted_total{domain, feature}
quidnug_fork_block_rejected_total{reason}
quidnug_fork_block_activated_total{domain, feature}
```

## 13. Alternatives considered

### 13.1 Wall-clock activation (rejected)

Use `ProposedAt` + a delay for activation. **Rejected**
because clock skew between nodes is unbounded; using a block
index is deterministic across the network.

### 13.2 Per-validator fork votes (rejected for v1)

Each validator submits a separate vote; fork activates once
N votes arrive. **Rejected** — adds O(N) on-chain messages
for what a single signed message with N signatures achieves.

## 14. Open questions

1. **What if the fork-tx never lands in a Trusted block?**
   The fork doesn't activate. Operator retries with a new
   nonce. Metric exposes unreached forks.
2. **Can a node manually override (flip the flag without
   a fork)?** Yes — `NonceLedgerEnforce` is still a
   per-node field the operator can set directly. Fork-block
   tx is the COORDINATED path; override is operator
   responsibility.
3. **What about the flip from `true` back to `false`?**
   Not currently supported. Fork tx only activates; a
   rollback is a future (rare) concern.

## 15. References

- [QDP-0001: Global Nonce Ledger](0001-global-nonce-ledger.md) §10
- [QDP-0004: Phase H Roadmap](0004-phase-h-roadmap.md) §3.5
