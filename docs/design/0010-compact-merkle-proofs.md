# QDP-0010: Compact Merkle Proofs (H2)

| Field      | Value                                                |
|------------|------------------------------------------------------|
| Status     | Draft                                                |
| Track      | Protocol (soft fork)                                 |
| Author     | The Quidnug Authors                                  |
| Created    | 2026-04-18                                           |
| Requires   | QDP-0003, QDP-0005 (H1), QDP-0009 (H5)               |
| Implements | Phase H2 of QDP-0004 roadmap                         |
| Target     | v2.6                                                 |

## 1. Summary

Anchor gossip today (QDP-0003, H1) ships an entire origin block
with every `AnchorGossipMessage`. The bandwidth and memory cost
is acceptable because rotations are rare, but a block with 1000
transactions eats far more envelope than the single anchor it
documents. More importantly, it prevents light-client
verification: a receiver must hold the whole block in memory to
walk its transactions.

This document specifies **compact Merkle inclusion proofs**. A
new `TransactionsRoot` field on `Block` is the root of a binary
Merkle tree over canonical transaction bytes. Gossip messages
ship only the anchor transaction plus its inclusion proof; the
receiver verifies the proof against `TransactionsRoot` without
needing the rest of the transactions.

The change is backward-compatible via a soft-fork path: blocks
emit both forms during a shadow window, then the old form is
retired at a coordinated `ForkHeight` via QDP-0009.

## 2. Problem statement

- **Bandwidth.** A typical `AnchorGossipMessage` is ~8 KB today.
  For a block with many transactions, it can balloon to 50–100
  KB. Compact proofs collapse that to the anchor + ~10 hash
  frames ≈ 1 KB.
- **Memory.** Receivers of gossip must deserialize the entire
  block to access the anchor. A light client with RAM
  constraints can't participate.
- **Light-client story absent.** Without inclusion proofs,
  there's no way to verify a specific transaction's membership
  in a block without all transactions. This blocks the future
  mobile / thin-client use case.

## 3. Goals and non-goals

**Goals.**

- **G1.** Reduce gossip envelope size by ~70% for typical
  anchor-gossip traffic.
- **G2.** Backward-compatible rollout: producers emit both
  full-block and proof-enabled gossip for a shadow period;
  receivers prefer proof when present.
- **G3.** Hard-fork-compatible: activation of "proof-only"
  mode is coordinated via QDP-0009 fork-block transaction at
  a future `ForkHeight`.
- **G4.** Merkle tree canonicalization is deterministic and
  survives JSON round-trips — the same lesson as QDP-0003 §8.3.
- **G5.** No new cryptographic primitives. SHA-256 binary
  Merkle tree.

**Non-goals.**

- **NG1.** Sparse Merkle trees / absence proofs. Future work;
  binary is enough for inclusion.
- **NG2.** Retroactive proofs for pre-H2 blocks. Pre-fork
  blocks don't have `TransactionsRoot`; gossip for those uses
  the old full-block path.
- **NG3.** Transaction-level proofs for things other than
  anchors. Focused on the anchor-gossip path first.

## 4. Threat model

| Threat                                                         | Mitigation                                                                                                    |
|----------------------------------------------------------------|---------------------------------------------------------------------------------------------------------------|
| Forged proof passing verification                              | Proof verification recomputes the root; any forged path fails.                                                |
| Overly long proof path (amplification)                         | Proof length capped at `ceil(log2(MaxTxsPerBlock))` — proofs exceeding that are rejected.                     |
| Leaf canonicalization drift                                    | Leaves are `sha256(canonical-marshal(tx))` with map-round-trip canonicalization (same as gossip signatures).  |
| Receiver verifies proof against wrong root                     | The gossip message carries the full OriginBlock OR just TransactionsRoot + the signed BlockHash; both bind the root to the signed block. |
| Attacker produces a block with mismatched root                 | Block hash includes TransactionsRoot; mismatched root produces a different block hash that doesn't verify.     |

## 5. Data model

### 5.1 Block changes

Add one field:

```go
type Block struct {
    // existing fields...

    // TransactionsRoot (QDP-0010 / H2): root of the binary
    // Merkle tree over canonical transaction bytes. Empty
    // string when the block was produced under the pre-H2
    // protocol. Receivers that have seen the
    // `require_tx_tree_root` fork activation reject blocks
    // with empty TransactionsRoot.
    TransactionsRoot string `json:"transactionsRoot,omitempty"`
}
```

`calculateBlockHash` is updated to include `TransactionsRoot`
in the hashed payload. Older blocks (without the field) hash
to the same value as before because the field is omitted from
JSON serialization when empty (Go's `omitempty`).

### 5.2 Merkle tree construction

- Leaves: `leafHash(tx) = sha256(canonicalMarshal(tx))`.
- Canonical marshal: same round-trip-through-map pattern
  used by `calculateBlockHash` — `json.Marshal(tx)` → unmarshal
  to `map[string]interface{}` → `json.Marshal` again. This
  ensures leaf bytes are stable across producer/receiver even
  if `tx` came in as a typed struct at one end and a generic
  map at the other.
- Internal nodes: `sha256(left || right)` where `||` is byte
  concatenation.
- Odd-size levels: duplicate the last hash to balance. This is
  the Bitcoin convention; simple and unambiguous.
- Empty block: `TransactionsRoot = ""` (not `sha256("")`);
  empty blocks are rare enough to treat specially.

### 5.3 AnchorGossipMessage changes

Add an optional field:

```go
type AnchorGossipMessage struct {
    // existing fields...

    // MerkleProof (QDP-0010 / H2): when present, the receiver
    // prefers proof-based verification over full-block
    // verification. Each frame is a hex-encoded sha256 hash
    // plus a left/right bit indicating which side of the
    // concat it goes on.
    MerkleProof []MerkleProofFrame `json:"merkleProof,omitempty"`
}

type MerkleProofFrame struct {
    Hash string `json:"hash"`       // hex
    Side string `json:"side"`       // "left" | "right"
}
```

When `MerkleProof` is populated, the receiver:

1. Extracts the anchor transaction at `AnchorTxIndex` from
   `OriginBlock.Transactions` (or from a separate field if we
   decide to omit the full slice in proof-only mode — see
   §10).
2. Computes `leafHash(anchorTx)`.
3. Walks the proof frames, concatenating per `side` at each
   step, and applying sha256.
4. Compares the final hash against `OriginBlock.TransactionsRoot`.
5. If match, proceeds with standard anchor application.

### 5.4 Proof-only mode (post-fork)

When the `require_tx_tree_root` fork has activated and the
receiver has flipped its "proof-required" flag, incoming
`AnchorGossipMessage` must populate `MerkleProof` OR carry
a full `OriginBlock.Transactions` for backward compatibility.
Full-transactions path is the fallback; proof is preferred.

A future message variant (`AnchorGossipLite`) can omit
`OriginBlock.Transactions` entirely — an optimization deferred
past H2.

## 6. Protocol

### 6.1 Producer side

Block-seal time:

```go
seal(block):
    block.TransactionsRoot = merkleRoot(block.Transactions)
    block.Hash = calculateBlockHash(block)   // now includes root
    signBlock(block)
```

Gossip production time:

```go
pushAnchor(block, anchorTxIndex):
    payload := standard fields
    if block.TransactionsRoot != "":
        payload.MerkleProof = buildProof(block.Transactions, anchorTxIndex)
    // For shadow period, also include OriginBlock so pre-H2
    // peers can still verify the old way.
    payload.OriginBlock = block
    sign and push
```

### 6.2 Receiver side

```go
verify(m AnchorGossipMessage):
    if m.OriginBlock.TransactionsRoot != "" AND len(m.MerkleProof) > 0:
        // Proof path.
        anchorTx := m.OriginBlock.Transactions[m.AnchorTxIndex]
        leaf := leafHash(anchorTx)
        computed := walk(leaf, m.MerkleProof)
        if computed != m.OriginBlock.TransactionsRoot:
            return ErrGossipBadProof
        // Remaining signature checks unchanged.
    else:
        // Legacy full-block path (pre-H2 or unspecified proof).
```

### 6.3 Soft-fork rollout via QDP-0009

Stage 1 — v2.5.x. Producers emit blocks with
`TransactionsRoot` populated (field is empty in pre-H2 code).
Receivers ignore the field. No behavior change.

Stage 2 — v2.6.0-alpha. Producers emit `MerkleProof` in
gossip when the root is available. Receivers with H2 code
prefer proof; fall back to full-block. Both producers and
receivers are in shadow — emit metric on every proof-based
verification so operators can see adoption.

Stage 3 — `require_tx_tree_root` activated via QDP-0009 at
`ForkHeight`. From that height:

- Producers MUST include `TransactionsRoot`. A block with an
  empty root is rejected by receivers that have observed the
  fork activation.
- Receivers that see a non-empty root in a gossip message
  prefer proof-based verification.

## 7. Validation rules

New validations introduced by H2:

- **Block integrity (post-fork only).** If the receiver has
  seen `require_tx_tree_root` fork activation AND the block
  has empty `TransactionsRoot`, reject with
  `ErrBlockMissingTxRoot`.
- **Block hash stability.** `calculateBlockHash` now includes
  `TransactionsRoot` in its canonical form. Blocks with
  inconsistent root vs. computed-root hash are rejected by
  existing `ErrGossipBadBlockHash`.
- **Proof well-formed.** `len(MerkleProof) <=
  ceil(log2(MaxTxsPerBlock))` where `MaxTxsPerBlock = 4096`
  (generous bound). Each frame's `Side` is one of
  `"left"|"right"`. `Hash` decodes as 32-byte hex.
- **Proof verifies.** Walking the proof from the anchor leaf
  ends at `TransactionsRoot`. Any mismatch rejected.

## 8. Canonicalization (leaf bytes)

Same pattern as block hash:

```go
canonicalTxBytes(tx) =
    tmp := json.Marshal(tx)
    var generic interface{}
    json.Unmarshal(tmp, &generic)
    return json.Marshal(generic)
```

This is the map-round-trip trick from QDP-0003 §8.3 applied to
a single transaction instead of a whole block. Tested with the
same property test structure.

## 9. Migration

Additive until stage 3. Stage 3 requires QDP-0009 activation.

### 9.1 Pre-fork blocks

Blocks produced before H2 have empty `TransactionsRoot`. These
remain valid indefinitely; gossip messages for these blocks
simply can't use proofs. The `omitempty` tag means old-format
wire payloads don't carry the field at all, which keeps JSON
parsing of old data working.

### 9.2 Operator workflow

1. Deploy v2.6.0-alpha. Monitor `merkle_proof_used_total` and
   `merkle_proof_fallback_total` to confirm shadow coverage.
2. Once all validators are on v2.6.x for a quiet period,
   submit a fork-block transaction via QDP-0009:
   ```
   {
     feature: "require_tx_tree_root",
     forkHeight: <future>,
     signatures: [...quorum]
   }
   ```
3. At `ForkHeight`, receivers start rejecting blocks with
   empty `TransactionsRoot`. Producers must emit the field.

## 10. Test plan

### 10.1 Unit — Merkle tree

- **MerkleRoot_EmptyBlock** returns empty string.
- **MerkleRoot_SingleTx** equals `leafHash(tx)`.
- **MerkleRoot_BalancedTree** — 4 transactions, verify
  tree structure by recomputing.
- **MerkleRoot_OddCount** — 3 transactions: last leaf
  duplicated at level 1.
- **MerkleProof_Verify** — for each tx in an N-tx block,
  build a proof, walk it, confirm root.
- **MerkleProof_Tampered** — flip one bit in a proof frame
  → verification fails.
- **LeafCanonicalization_RoundTrip** — leaf hash stable after
  JSON marshal/unmarshal.

### 10.2 Integration — gossip with proof

- **AnchorGossipWithProof_HappyPath** — producer attaches
  proof; receiver verifies and applies.
- **AnchorGossipWithProof_TamperedTxRejected** — if the tx
  embedded in OriginBlock is swapped after signing, leaf
  hash changes, proof fails.
- **AnchorGossipWithProof_LegacyFallback** — receiver handles
  message with empty MerkleProof via old full-block path.

### 10.3 Hard-fork activation

- **BlockRejected_WhenForkActiveAndRootEmpty** — after
  `require_tx_tree_root` activation, a block with empty root
  is rejected.
- **BlockAccepted_WhenRootPopulated** — same block with root
  populated is accepted.
- **Stage2_BothFormsAccepted** — before fork activation,
  blocks with or without root both accepted.

## 11. Metrics

```
quidnug_merkle_proof_used_total
quidnug_merkle_proof_fallback_total{reason}  // no_root | no_proof | bad_proof
quidnug_merkle_proof_verify_fail_total{reason}
quidnug_block_missing_tx_root_rejected_total
```

## 12. Alternatives considered

### 12.1 Sparse Merkle tree (deferred)

Keyed-by-txID sparse tree would enable efficient absence
proofs ("tx X was NOT in block N"). Useful for future light-
client applications. Deferred — binary is simpler and covers
inclusion which is the H2 goal.

### 12.2 Merkle DAG with pointer compression (rejected)

Content-addressable tree nodes stored externally (IPFS-style).
Rejected because it moves verification state off-chain and
complicates the trust story. Binary Merkle root in the block
header is self-contained.

### 12.3 BLS aggregate signatures over leaves (deferred)

Would let the anchor-gossip carry one aggregate proof instead
of a chain of hashes. Much more interesting crypto, much larger
implementation cost. Deferred.

## 13. Open questions

1. **Proof frame encoding.** `Side` as string is friendly;
   `bool` is tighter. Proposal: stick with string for
   readability; the difference is ~2 bytes per frame.
2. **Full-block retention in proof mode.** Stage 2 keeps the
   full block in gossip messages so pre-H2 receivers can still
   validate. Stage 3 could drop that. Deferred to a future
   `AnchorGossipLite` variant.
3. **Merkle root on Block vs. a separate hash.** We're
   including the root inside the signed block. Alternative is
   to compute it on demand. In-block is self-documenting and
   lets receivers skip recomputation.

## 14. References

- [QDP-0001: Global Nonce Ledger](0001-global-nonce-ledger.md)
- [QDP-0003: Cross-Domain Nonce Scoping](0003-cross-domain-nonce-scoping.md) §8.3 (canonicalization)
- [QDP-0009: Fork-Block Migration Trigger](0009-fork-block-trigger.md)
- [QDP-0004: Phase H Roadmap](0004-phase-h-roadmap.md) §3.2
