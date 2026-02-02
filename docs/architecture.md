# Quidnug Architecture

## Overview

Quidnug is a trust protocol implementation consisting of a Go-based node server and optional client libraries. This document describes the internal architecture for developers implementing or extending Quidnug.

## Component Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                      HTTP Layer                              │
│  handlers.go (API endpoints) + middleware.go (rate limiting) │
└─────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────┐
│                    Business Logic                            │
│  node.go (QuidnugNode) - Transaction processing, blocks     │
└─────────────────────────────────────────────────────────────┘
                              │
              ┌───────────────┼───────────────┐
              ▼               ▼               ▼
┌─────────────────┐ ┌─────────────────┐ ┌─────────────────┐
│   validation.go │ │   registry.go   │ │   crypto.go     │
│  Tx validation  │ │  State storage  │ │  ECDSA signing  │
└─────────────────┘ └─────────────────┘ └─────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────┐
│                    Network Layer                             │
│  network.go - Node discovery, broadcasting, cross-domain    │
└─────────────────────────────────────────────────────────────┘
```

## Core Components

### QuidnugNode (`node.go`)

The central struct managing all node state:

- **Blockchain**: Ordered list of validated blocks
- **PendingTxs**: Transactions awaiting inclusion in blocks
- **TrustDomains**: Domains this node manages/participates in
- **KnownNodes**: Discovered network peers
- **Registries**: Materialized views of blockchain state (Trust, Identity, Title)

Each registry has its own `sync.RWMutex` for granular concurrent access.

### Relational Trust Model

Trust in Quidnug is **relational**, not absolute. There is no global "trust score" for any quid—trust is always computed dynamically from an observer's perspective to a target.

#### Key Principles

1. **Observer → Target**: Every trust query specifies an observer (who is asking) and a target (who is being assessed). The same target may have different trust levels when viewed by different observers.

2. **Transitive with Multiplicative Decay**: Trust propagates through the network with decay. If A trusts B at 0.8 and B trusts C at 0.7, then A's transitive trust in C is 0.8 × 0.7 = 0.56.

3. **Best Path Selection**: When multiple paths exist, the algorithm finds and returns the path with maximum trust.

4. **Same Entity**: An observer has full trust (1.0) in itself.

5. **No Path**: If no path exists from observer to target, trust level is 0.

#### Trust Graph Structure

Trust edges are stored in the registry:

```go
// TrustRegistry maps: truster -> trustee -> trustLevel
TrustRegistry map[string]map[string]float64
```

Trust *values* are computed on-demand via `ComputeRelationalTrust()`:

```go
type RelationalTrustResult struct {
    Observer   string   // Who is asking
    Target     string   // Who is being assessed
    TrustLevel float64  // Computed transitive trust (0.0-1.0)
    TrustPath  []string // Path of quid IDs for best route
    PathDepth  int      // Number of hops
    Domain     string   // Trust domain (optional)
}
```

### Transaction Types (`types.go`)

| Type | Purpose | Key Fields |
|------|---------|------------|
| `TRUST` | Establish trust between quids | `truster`, `trustee`, `trustLevel` (0.0-1.0) |
| `IDENTITY` | Define quid attributes | `quidId`, `name`, `attributes`, `updateNonce` |
| `TITLE` | Declare asset ownership | `assetId`, `owners` (must sum to 100%) |

All transactions require cryptographic signatures (ECDSA P-256).

### Validation (`validation.go`)

Transactions are validated before entering the pending pool:

1. **Trust Domain Check**: Domain must exist or be empty (default domain)
2. **Signature Verification**: ECDSA P-256 signature must be valid
3. **Format Validation**: Quid IDs must be 16-char lowercase hex
4. **Business Rules**:
   - Trust levels: 0.0 <= level <= 1.0, no NaN/Inf
   - Identity updates: `updateNonce` must increase
   - Titles: ownership percentages must sum to exactly 100.0

#### Block Validation with Relational Trust

Validators assess blocks based on their **relational trust** in the block creator:

```go
type TrustProof struct {
    TrustDomain             string  // Domain for this block
    ValidatorID             string  // QuidID of the validator
    ValidatorTrustInCreator float64 // Validator's relational trust in block creator
    ValidatorSigs           []string
    ValidationTime          int64
}
```

The `ValidatorTrustInCreator` field is computed at validation time using `ComputeRelationalTrust(validatorID, creatorID, maxDepth)`. This means:

- Different validators may have different trust levels for the same block creator
- Trust is evaluated dynamically, reflecting the current state of the trust graph
- There is no static "trust score" stored for any quid

### Block Generation

Blocks are generated periodically (configurable via `BLOCK_INTERVAL`):

1. Filter pending transactions by trust domain
2. Create block with trust proof (validator signature)
3. Add to blockchain
4. Process transactions to update registries
5. Broadcast to domain peers

### Network Operations (`network.go`)

**Node Discovery**: Periodically queries seed nodes for peer lists.

**Transaction Broadcasting**: Fire-and-forget POST to domain peers.

**Cross-Domain Queries**: Hierarchical domain walking (e.g., `sub.domain.com` -> `domain.com` -> `com`) to find authoritative nodes.

## Cryptographic Operations (`crypto.go`)

| Function | Purpose |
|----------|---------|
| `SignData(data []byte)` | Sign with node's private key (ECDSA P-256) |
| `VerifySignature(pubKey, data, sig)` | Verify signature against public key |
| `calculateBlockHash(block)` | SHA-256 hash of block contents |

**Key Format**: Public keys are uncompressed P-256 (65 bytes: 0x04 || X || Y), hex-encoded.

**Signature Format**: 64 bytes (r || s, each 32 bytes), hex-encoded.

## State Persistence (`persistence.go`)

Pending transactions are saved to `DATA_DIR/pending_transactions.json` on shutdown and restored on startup.

## HTTP Middleware (`middleware.go`)

- **Rate Limiting**: Token bucket per IP (default 100 req/min)
- **Body Size Limit**: Prevents oversized payloads (default 1MB)
- **Input Validation**: Quid ID format, string sanitization

## Relational Trust Algorithm (`registry.go`)

The `ComputeRelationalTrust` function computes transitive trust from an observer to a target:

```go
func (node *QuidnugNode) ComputeRelationalTrust(
    observer, target string, 
    maxDepth int,
) (float64, []string, error)
```

### Algorithm Details

1. **BFS Traversal**: Uses breadth-first search starting from the observer
2. **Multiplicative Decay**: Path trust = product of all edge trust levels along the path
3. **Cycle Avoidance**: Tracks visited nodes in each path to prevent infinite loops
4. **Best Path Selection**: Returns the maximum trust found across all explored paths
5. **Depth Limiting**: Respects `maxDepth` parameter (defaults to 5 if not specified)

### Example Computation

```
Trust Graph:
  A → B (0.8)
  A → C (0.6)
  B → D (0.7)
  C → D (0.9)

Query: ComputeRelationalTrust("A", "D", 5)

Paths explored:
  A → B → D: 0.8 × 0.7 = 0.56
  A → C → D: 0.6 × 0.9 = 0.54

Result:
  TrustLevel: 0.56 (maximum)
  TrustPath: ["A", "B", "D"]
  PathDepth: 2
```

### Special Cases

| Scenario | Result |
|----------|--------|
| Observer equals target | TrustLevel: 1.0, Path: [observer] |
| No path exists | TrustLevel: 0.0, Path: empty |
| Direct trust only | TrustLevel: direct edge value, Path: [observer, target] |

## Thread Safety

The codebase uses granular mutexes following Go best practices:

| Mutex | Protects |
|-------|----------|
| `BlockchainMutex` | `Blockchain` slice |
| `PendingTxsMutex` | `PendingTxs` slice |
| `TrustDomainsMutex` | `TrustDomains` map |
| `KnownNodesMutex` | `KnownNodes` map |
| `TrustRegistryMutex` | `TrustRegistry` map |
| `IdentityRegistryMutex` | `IdentityRegistry` map |
| `TitleRegistryMutex` | `TitleRegistry` map |

Always acquire read locks for queries and write locks for mutations.

Note: `ComputeRelationalTrust` acquires a read lock on `TrustRegistryMutex` via `GetDirectTrustees()` for each node visited during BFS traversal.
