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
