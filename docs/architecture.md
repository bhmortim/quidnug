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
| `EVENT` | Record events for subjects | `subjectId`, `subjectType`, `sequence`, `eventType`, `payload`/`payloadCid` |

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
   - Events: see Event Transaction validation below

#### Event Transaction Validation

Event transactions have additional validation rules:

| Rule | Description |
|------|-------------|
| **TrustDomain** | Must exist (no empty/default domain for events) |
| **SubjectID** | Must be valid 16-character hex quid format |
| **SubjectType** | Must be `"QUID"` or `"TITLE"` |
| **EventType** | Required, maximum 64 characters |
| **Payload/PayloadCID** | Either `payload` or `payloadCid` must be provided (not both empty) |
| **PayloadCID Format** | If provided, must be a valid IPFS CID (CIDv0 or CIDv1) |
| **Payload Size** | Inline payload maximum 64KB (`MaxPayloadSize`) |
| **Sequence** | Must be monotonically increasing (> `LatestSequence`, or 0/1 for new streams) |
| **Signature** | Required, signer must be the subject owner |

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

## IPFS Integration

Quidnug integrates with IPFS for storing large event payloads that exceed the inline size limit.

### IPFSClient Interface

```go
type IPFSClient interface {
    Pin(ctx context.Context, data []byte) (cid string, err error)
    Get(ctx context.Context, cid string) (data []byte, err error)
    IsAvailable() bool
}
```

| Method | Purpose |
|--------|---------|
| `Pin(ctx, data)` | Store data in IPFS and return the content identifier (CID) |
| `Get(ctx, cid)` | Retrieve data from IPFS by CID |
| `IsAvailable()` | Check if IPFS integration is enabled and reachable |

### Implementations

| Implementation | Description |
|----------------|-------------|
| `HTTPIPFSClient` | Production client that communicates with an IPFS gateway via HTTP API |
| `NoOpIPFSClient` | Stub implementation when IPFS is disabled; `IsAvailable()` returns `false` |

The implementation is selected at startup based on the `QUIDNUG_IPFS_ENABLED` configuration:
- When `QUIDNUG_IPFS_ENABLED=true`: `HTTPIPFSClient` connects to `QUIDNUG_IPFS_GATEWAY_URL`
- When `QUIDNUG_IPFS_ENABLED=false` (default): `NoOpIPFSClient` is used

### Payload Storage Strategy

Event payloads can be stored in two ways:

| Strategy | Condition | Storage |
|----------|-----------|---------|
| **Inline** | Payload ≤ 64KB (`MaxPayloadSize`) | Stored directly in the `payload` field |
| **IPFS** | Payload > 64KB or explicitly pinned | Content stored in IPFS, CID in `payloadCid` field |

When processing an event with `payloadCid`:
1. The node retrieves the content from IPFS using `IPFSClient.Get()`
2. Content is validated and processed
3. Applications can cache retrieved payloads locally

**Note**: If IPFS is unavailable and only `payloadCid` is provided, the event data cannot be fully retrieved until IPFS becomes available.

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

## Event Stream Registry

The node maintains registries for tracking event streams associated with quids and titles.

### Data Structures

```go
// Metadata about each subject's event stream
EventStreamRegistry map[string]*EventStream  // keyed by subjectId

// Actual event transactions for each subject
EventRegistry map[string][]EventTransaction  // keyed by subjectId
```

#### EventStream Metadata

```go
type EventStream struct {
    SubjectID      string `json:"subjectId"`      // Quid or asset ID
    SubjectType    string `json:"subjectType"`    // "QUID" or "TITLE"
    LatestSequence int64  `json:"latestSequence"` // Highest sequence number
    EventCount     int64  `json:"eventCount"`     // Total events in stream
    CreatedAt      int64  `json:"createdAt"`      // Unix timestamp of first event
    UpdatedAt      int64  `json:"updatedAt"`      // Unix timestamp of last event
    LatestEventID  string `json:"latestEventId"`  // Transaction ID of most recent event
}
```

### Registry Update Process

When processing a block containing event transactions, `updateEventStreamRegistry` performs:

1. **Lock acquisition**: Acquires write lock on `EventStreamMutex`
2. **Stream initialization**: Creates new `EventStream` if subject has no prior events
3. **Event append**: Adds the `EventTransaction` to `EventRegistry[subjectId]`
4. **Metadata update**: Updates `EventStreamRegistry` with:
   - Incremented `EventCount`
   - Updated `LatestSequence` to the event's sequence
   - Updated `UpdatedAt` timestamp
   - Updated `LatestEventID`

### Querying Events

| Function | Purpose |
|----------|---------|
| `GetEventStream(subjectId)` | Returns stream metadata, or `false` if no stream exists |
| `GetStreamEvents(subjectId, limit, offset)` | Returns paginated events ordered by sequence (ascending) |

Events are returned in sequence order to support chronological replay of a subject's history.

## Proof of Trust Consensus

Quidnug implements a novel consensus mechanism called **Proof of Trust** where block validation is subjective—each node validates blocks based on its own relational trust in the block's validator.

### Cryptographic vs Trust Validation

Block validation is split into two distinct phases:

```
┌─────────────────────────────────────────────────────────────────┐
│                  ValidateBlockCryptographic()                   │
│  • Hash verification                                            │
│  • Signature verification                                       │
│  • Chain linkage (prevHash, index)                              │
│  • Transaction format validation                                │
│                                                                 │
│  UNIVERSAL: All honest nodes agree on this                      │
└─────────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────────┐
│                  ValidateTrustProofTiered()                     │
│  • Compute relational trust: node → validator                   │
│  • Compare against domain threshold                             │
│  • Return acceptance tier                                       │
│                                                                 │
│  SUBJECTIVE: Different nodes may have different results         │
└─────────────────────────────────────────────────────────────────┘
```

This separation allows nodes to:
1. Agree on cryptographic validity (objective)
2. Disagree on whether to accept the block (subjective, based on trust)

### Tiered Block Acceptance

The `BlockAcceptance` type defines four tiers:

```go
type BlockAcceptance int

const (
    BlockTrusted   BlockAcceptance = iota // Integrate into main chain
    BlockTentative                        // Store separately, don't build on
    BlockUntrusted                        // Extract data, relay, don't store block
    BlockInvalid                          // Reject entirely
)
```

#### ReceiveBlock() Decision Flow

```
                    Incoming Block
                          │
                          ▼
              ┌───────────────────────┐
              │ ValidateBlockCrypto() │
              └───────────────────────┘
                          │
                    ┌─────┴─────┐
                    │   Valid?  │
                    └─────┬─────┘
                     No   │   Yes
              ┌───────────┴───────────┐
              ▼                       ▼
        BlockInvalid     ┌────────────────────────┐
                         │ Extract trust edges    │
                         │ (store as unverified)  │
                         └────────────────────────┘
                                      │
                                      ▼
                         ┌────────────────────────┐
                         │ ValidateTrustProof     │
                         │ Tiered()               │
                         └────────────────────────┘
                                      │
              ┌───────────────────────┼───────────────────────┐
              │                       │                       │
              ▼                       ▼                       ▼
    trust >= threshold     distrust < trust      trust <= distrust
              │             < threshold                       │
              ▼                       │                       ▼
        BlockTrusted                  ▼                 BlockUntrusted
              │              BlockTentative                   │
              │                       │                       │
              ▼                       ▼                       ▼
    ┌─────────────────┐    ┌─────────────────┐    ┌─────────────────┐
    │ Add to main     │    │ Store in        │    │ Edges already   │
    │ chain           │    │ TentativeBlocks │    │ extracted       │
    │ Process txs     │    │ Don't build on  │    │ Block discarded │
    │ Promote edges   │    └─────────────────┘    └─────────────────┘
    │ to verified     │
    └─────────────────┘
```

### Dual-Layer Trust Registry

The node maintains two separate trust edge registries:

```go
// Edges from trusted validators (high-assurance decisions)
VerifiedTrustEdges map[string]map[string]TrustEdge

// Edges from all cryptographically valid blocks (discovery with discounting)
UnverifiedTrustRegistry map[string]map[string]TrustEdge
```

#### When to Use Each Registry

| Registry | Source | Use Case |
|----------|--------|----------|
| **VerifiedTrustEdges** | Blocks from trusted validators | High-stakes decisions, authentication |
| **UnverifiedTrustRegistry** | All cryptographically valid blocks | Discovery, exploration, lower-stakes queries |

When querying trust with `includeUnverified=true`, the algorithm can traverse unverified edges but applies appropriate discounting to the trust levels.

### Trust Edge Provenance

Every trust edge tracks its origin:

```go
type TrustEdge struct {
    Truster       string  // Who is trusting
    Trustee       string  // Who is being trusted
    TrustLevel    float64 // Trust level 0.0-1.0
    SourceBlock   string  // Block hash where this edge was recorded
    ValidatorQuid string  // Quid of validator who signed the block
    Verified      bool    // True if from a trusted validator
    Timestamp     int64   // When recorded
}
```

This provenance enables:
- Auditing trust relationships back to their source
- Promoting edges when validator trust changes
- Demoting edges if a validator becomes untrusted

### Tentative Block Storage

Blocks from partially-trusted validators are stored separately:

```go
TentativeBlocks map[string][]Block // keyed by trust domain
```

#### Management Functions

| Function | Purpose |
|----------|---------|
| `StoreTentativeBlock(block)` | Add block to tentative storage |
| `GetTentativeBlocks(domain)` | Retrieve tentative blocks for a domain |
| `ReEvaluateTentativeBlocks(domain)` | Check if any can now be promoted |

#### Promotion Flow

When trust relationships change (e.g., you establish trust in a new validator), call `ReEvaluateTentativeBlocks()` to check if any tentative blocks can now be promoted to the main chain:

```go
// After establishing trust in a new validator
node.AddTrustTransaction(trustTx)
node.ReEvaluateTentativeBlocks("example.com")
```

### Trust-Aware Transaction Filtering

During block generation, `FilterTransactionsForBlock()` ensures only trusted transactions are included:

```go
func (node *QuidnugNode) FilterTransactionsForBlock(
    txs []interface{}, 
    domain string,
) []interface{}
```

For each pending transaction:
1. Extract the creator quid (truster, creator, or first owner depending on type)
2. Compute relational trust from node to creator
3. Include only if `trustLevel >= node.TransactionTrustThreshold`

This prevents a node from propagating transactions it doesn't trust, even if they're cryptographically valid.

### Enhanced Trust Queries

The `EnhancedTrustResult` provides additional provenance information:

```go
type EnhancedTrustResult struct {
    RelationalTrustResult
    Confidence       string            // "high", "medium", "low"
    UnverifiedHops   int               // Number of unverified edges traversed
    VerificationGaps []VerificationGap // Details of unverified hops
}

type VerificationGap struct {
    From           string  // Source quid of the gap
    To             string  // Target quid of the gap
    ValidatorQuid  string  // Validator who recorded this edge
    ValidatorTrust float64 // Node's trust in that validator
}
```

Confidence levels are determined by:
- **High**: All edges in path are verified
- **Medium**: Some unverified edges, but validators have partial trust
- **Low**: Significant unverified hops or low validator trust

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
| `TentativeBlocksMutex` | `TentativeBlocks` map |
| `UnverifiedRegistryMutex` | `UnverifiedTrustRegistry` map |
| `EventStreamMutex` | `EventStreamRegistry` and `EventRegistry` maps |

Always acquire read locks for queries and write locks for mutations.

Note: `ComputeRelationalTrust` acquires a read lock on `TrustRegistryMutex` via `GetDirectTrustees()` for each node visited during BFS traversal.
