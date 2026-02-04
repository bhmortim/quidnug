# Rogue Node Security in Quidnug

## 1. Executive Summary

Quidnug's relational trust architecture **inherently neutralizes rogue nodes** without requiring special detection or blacklisting mechanisms. The protocol's fundamental design ensures that untrusted nodes—regardless of how they join the network—cannot:

- Influence blockchain state on any honest node
- Have their transactions included in blocks
- Inject false trust relationships into the network
- Affect trust computations for any observer

**Key Takeaway**: In Quidnug, "rogue node" is essentially a non-concept. An untrusted node is simply *ignored* by the trust computation—it has zero influence by default, not by configuration.

## 2. The Relational Trust Model

Trust in Quidnug is fundamentally different from most systems. There is no global reputation score, no centralized trust authority, and no "trusted node list" that can be compromised.

### Core Principles

From the [architecture documentation](architecture.md#relational-trust-model):

> Trust in Quidnug is **relational**, not absolute. There is no global "trust score" for any quid—trust is always computed dynamically from an observer's perspective to a target.

| Principle | Description |
|-----------|-------------|
| **Observer → Target** | Every trust query specifies who is asking (observer) and who is being assessed (target) |
| **Transitive Decay** | Trust propagates multiplicatively: A→B (0.8) and B→C (0.7) yields A→C trust of 0.56 |
| **No Path = Zero Trust** | If no trust path exists from observer to target, trust level is exactly 0 |
| **Self-Trust** | An entity always has full trust (1.0) in itself |

### Why This Matters for Rogue Nodes

When a rogue node joins the network, it has **no trust path** from any honest observer. The relational trust computation naturally returns 0:

```go
// From src/core/registry.go - ComputeRelationalTrust()

// Same entity has full trust in itself
if observer == target {
    return 1.0, []string{observer}, nil
}

// ... BFS traversal through trust graph ...

// If no path found, bestTrust remains 0.0
return bestTrust, bestPath, nil
```

A rogue node isn't "blocked" or "banned"—it simply doesn't exist in anyone's trust graph until someone explicitly trusts it.

## 3. What Happens When a Rogue Node Joins the Network

### 3.1 Block Submission

When a rogue node submits a block, Quidnug's validation separates **cryptographic validity** (universal) from **trust validity** (subjective).

#### Phase 1: Cryptographic Validation

`ValidateBlockCryptographic()` checks objective properties that all honest nodes agree on:

```go
// From src/core/validation.go

func (node *QuidnugNode) ValidateBlockCryptographic(block Block) bool {
    // Check block index and previous hash
    if block.Index != prevBlock.Index+1 || block.PrevHash != prevBlock.Hash {
        return false
    }

    // Verify the block hash
    if calculateBlockHash(block) != block.Hash {
        return false
    }

    // Verify validator signature against block content
    if !VerifySignature(validatorPubKey, signableData, block.TrustProof.ValidatorSigs[0]) {
        return false
    }

    return true
}
```

A rogue node's block may pass this phase if properly signed—cryptographic validity doesn't require trust.

#### Phase 2: Trust Validation

`ValidateTrustProofTiered()` computes the receiving node's **relational trust** in the validator:

```go
// From src/core/validation.go

func (node *QuidnugNode) ValidateTrustProofTiered(block Block) BlockAcceptance {
    // ... domain and validator checks ...

    // Node-relative trust validation: compute relational trust from this node to the validator
    trustLevel, _, err := node.ComputeRelationalTrust(node.NodeID, proof.ValidatorID, DefaultTrustMaxDepth)

    // Return tier based on trust level
    if trustLevel >= domain.TrustThreshold {
        return BlockTrusted
    }

    if trustLevel > node.DistrustThreshold {
        return BlockTentative
    }

    // trustLevel <= DistrustThreshold (includes trustLevel == 0)
    return BlockUntrusted
}
```

For a rogue validator with no trust path, `ComputeRelationalTrust()` returns 0, yielding `BlockUntrusted`.

#### Tiered Block Acceptance

The `BlockAcceptance` enum defines graduated responses:

```go
// From src/core/types.go

const (
    BlockTrusted   BlockAcceptance = iota // Integrate into main chain
    BlockTentative                        // Store separately, don't build on
    BlockUntrusted                        // Extract data, relay, don't store block
    BlockInvalid                          // Reject entirely
)
```

| Acceptance Level | Trust Condition | Action |
|------------------|-----------------|--------|
| `BlockTrusted` | `trustLevel >= TrustThreshold` | Add to main chain, process transactions |
| `BlockTentative` | `DistrustThreshold < trustLevel < TrustThreshold` | Store separately, don't build on it |
| `BlockUntrusted` | `trustLevel <= DistrustThreshold` | Extract data for discovery, discard block |
| `BlockInvalid` | Cryptographically invalid | Reject entirely |

**Result for rogue nodes**: `BlockUntrusted`—the block is **not** added to the main chain.

### 3.2 Transaction Submission

Even if a rogue node submits transactions directly to honest nodes, those transactions are filtered during block generation.

From the [architecture documentation](architecture.md#trust-aware-transaction-filtering):

> During block generation, `FilterTransactionsForBlock()` ensures only trusted transactions are included.

For each pending transaction:
1. Extract the creator quid (truster, creator, or first owner depending on type)
2. Compute relational trust from node to creator
3. Include only if `trustLevel >= node.TransactionTrustThreshold`

**Result for rogue nodes**: Transactions from untrusted creators are **filtered out** before block generation.

### 3.3 Trust Edge Extraction

Quidnug extracts trust edges even from untrusted blocks—but stores them in a separate registry with appropriate discounting.

#### Dual-Layer Trust Registry

```go
// From src/core/registry.go

// Edges from trusted validators (high-assurance decisions)
VerifiedTrustEdges map[string]map[string]TrustEdge

// Edges from all cryptographically valid blocks (discovery with discounting)
UnverifiedTrustRegistry map[string]map[string]TrustEdge
```

When a rogue submits a block claiming "A trusts B at 0.9", that edge goes to `UnverifiedTrustRegistry` with the rogue's validator ID recorded:

```go
// From src/core/registry.go

func (node *QuidnugNode) AddUnverifiedTrustEdge(edge TrustEdge) {
    // ...
    edge.Verified = false
    node.UnverifiedTrustRegistry[edge.Truster][edge.Trustee] = edge
}
```

#### Discounting by Validator Trust

When computing trust with unverified edges, `ComputeRelationalTrustEnhanced()` **discounts by the observer's trust in the recording validator**:

```go
// From src/core/registry.go - ComputeRelationalTrustEnhanced()

if edge.Verified {
    effectiveTrust = current.trust * edge.TrustLevel
} else {
    // Discount by validator trust
    validatorTrust, _, err := node.ComputeRelationalTrust(observer, edge.ValidatorQuid, DefaultTrustMaxDepth)
    effectiveTrust = current.trust * edge.TrustLevel * validatorTrust
}
```

**Result for rogue nodes**: Since `validatorTrust = 0`, the effective trust contribution is `claimed_trust × 0 = 0`. The rogue's claimed trust edges have **zero effect**.

## 4. Defense in Depth: Multiple Protection Layers

Quidnug implements security through multiple independent layers:

| Layer | Mechanism | Code Reference | What It Stops |
|-------|-----------|----------------|---------------|
| **Cryptographic Validation** | Hash, signature, chain verification | `ValidateBlockCryptographic()` | Forged blocks, tampered data |
| **Trust Validation** | Relational trust computation | `ValidateTrustProofTiered()` | Blocks from untrusted validators |
| **Block Acceptance Tiers** | Graduated response to trust levels | `BlockAcceptance` enum | Prevents untrusted blocks from affecting state |
| **Transaction Filtering** | Trust threshold for tx inclusion | `FilterTransactionsForBlock()` | Transactions from untrusted creators |
| **Edge Discounting** | Unverified edges weighted by validator trust | `ComputeRelationalTrustEnhanced()` | False trust claims |
| **Provenance Tracking** | Every edge records its validator | `TrustEdge.ValidatorQuid` | Enables trust re-evaluation |

Each layer operates independently—even if one fails, others prevent compromise.

## 5. Impact Assessment

What can a rogue node actually do?

| Rogue Action | Impact | Why |
|--------------|--------|-----|
| **Submit blocks** | None | Trust level 0 → `BlockUntrusted` → not added to chain |
| **Submit transactions** | None | Filtered by `TransactionTrustThreshold` during block generation |
| **Claim trust relationships** | None | Unverified edges discounted by validator trust (0) = 0 |
| **Participate in gossip** | Minimal | Can relay messages but cannot influence state |
| **Exist in `KnownNodes`** | None | Just another peer entry; trust is separate from discovery |
| **Flood the network** | Rate-limited | HTTP middleware applies per-IP rate limiting |
| **Announce fake domains** | Ignored | Domain gossip doesn't grant trust in those domains |

## 6. Local Deployment Considerations

Organizations often deploy Quidnug nodes on local networks with mDNS or UDP broadcast discovery. This is **safe by design**:

### Why Local Discovery Poses No Risk

1. **Discovery ≠ Trust**: Finding a node on the network adds it to `KnownNodes`, but this grants zero trust
2. **Same Protections Apply**: A locally-discovered node has the same (lack of) influence as an internet-discovered one
3. **No Special Handling Needed**: The protocol doesn't distinguish between local and remote nodes

### Secure Local Deployment

```
┌─────────────────────────────────────────────────────────────┐
│                    Local Network                             │
│                                                              │
│   ┌─────────┐     ┌─────────┐     ┌─────────┐               │
│   │ Node A  │────│ Node B  │────│ Rogue   │               │
│   │ (yours) │     │ (yours) │     │  Node   │               │
│   └─────────┘     └─────────┘     └─────────┘               │
│       │               │               │                      │
│       │  Trust: 0.9   │               │  Trust: 0            │
│       └───────────────┘               │                      │
│                                       │                      │
│   The rogue can see A and B, but     │                      │
│   cannot affect their trust graph    │                      │
└─────────────────────────────────────────────────────────────┘
```

If you trust Node B, you establish that trust explicitly via a signed `TrustTransaction`. The rogue being on the same network doesn't change this.

## 7. Global Network Security

The Quidnug network is resilient against large-scale attacks:

### Decentralized Trust Computation

Each node independently computes trust from its own perspective. There is:

- **No central trust authority** that can be compromised
- **No global "trusted validator list"** that attackers can target
- **No consensus on who is trusted**—every node maintains its own view

### Sybil Resistance

A Sybil attack (creating many fake identities) is ineffective because:

1. Each new identity has zero trust from all observers
2. Creating 1,000 fake nodes that trust each other doesn't affect anyone else's trust graph
3. Trust must originate from already-trusted entities to propagate

### Trust Cannot Be Injected

```
Attacker's View:           Honest Node's View:

  Attacker                   Attacker
     │                          │
     ▼ (self-declared)          │ (no path)
   Fake Trust ──► Target        │
                               ✗ Trust: 0
```

The attacker can claim any trust relationship—but without a path from honest observers to the attacker, those claims have zero weight.

## 8. The Only Way to Gain Trust

There is exactly one legitimate path for a new node to gain trust:

### Step-by-Step Process

1. **Existing trusted entity decides to trust the new node**
   - A human or organization makes this decision out-of-band

2. **Trusted entity creates a `TrustTransaction`**
   ```go
   TrustTransaction{
       Truster:    "existing_trusted_quid",
       Trustee:    "new_node_quid",
       TrustLevel: 0.7,  // or whatever level is appropriate
       Nonce:      1,
       // ...
   }
   ```

3. **Transaction is signed by the truster's private key**
   - Cryptographically proves the trusted entity authorized this

4. **Transaction is included in a block by a trusted validator**
   - Prevents untrusted validators from bootstrapping trust

5. **Trust propagates relationally**
   - Observers who trust the truster can now compute transitive trust to the new node
   - Observers who don't trust the truster still see trust level 0

### What This Means for Attackers

An attacker cannot:
- Create trust out of nothing (no trusted entity will sign for them)
- Use a compromised validator (blocks from untrusted validators are ignored)
- Exploit discovery mechanisms (discovery doesn't grant trust)
- Gradually build trust through activity (trust is explicit, not earned through behavior)

## 9. Conclusion

Quidnug's architecture makes the concept of a "rogue node" fundamentally meaningless:

| Traditional Systems | Quidnug |
|---------------------|---------|
| Need to detect bad actors | Bad actors are ignored by default |
| Maintain blocklists | No blocklists needed |
| Trust new nodes by default | Trust nothing by default |
| Central authority assigns trust | Trust is relational and decentralized |
| Security through configuration | Security through architecture |

### Key Design Principles

1. **Secure by Default**: Zero trust unless explicitly established
2. **Relational, Not Absolute**: No global trust scores to compromise
3. **Defense in Depth**: Multiple independent protection layers
4. **Fail Safe**: Unknown entities have zero influence, not partial influence

An untrusted node isn't "blocked" or "banned" in Quidnug—it simply doesn't exist in any honest node's trust graph. The architecture ensures that joining the network is trivial, but *mattering* to the network requires legitimate trust establishment.

---

*For more details on the trust architecture, see [architecture.md](architecture.md).*
