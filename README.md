# Quidnug

[![CI](https://github.com/quidnug/quidnug/actions/workflows/ci.yml/badge.svg)](https://github.com/quidnug/quidnug/actions/workflows/ci.yml)
[![JS Client CI](https://github.com/quidnug/quidnug/actions/workflows/js-client.yml/badge.svg)](https://github.com/quidnug/quidnug/actions/workflows/js-client.yml)
[![codecov](https://codecov.io/gh/quidnug/quidnug/branch/main/graph/badge.svg)](https://codecov.io/gh/quidnug/quidnug)
[![Go Report Card](https://goreportcard.com/badge/github.com/quidnug/quidnug)](https://goreportcard.com/report/github.com/quidnug/quidnug)

Quidnug is a Trust protocol. A cryptographically secured implementation for the Quidnug network, providing a foundation for trust, identity, and ownership management through a hierarchical domain structure.

## What is Quidnug?

Quidnug is a security and encryption platform for establishing cryptographic trust relationships between entities (quids). Similar to how Bitcoin wallets serve as cryptographic identities in the cryptocurrency space, quids function as the base entities in the Quidnug system. A quid's relevance or authority in the network is determined by the trust relationships extended to it by other quids.

## Core Concepts

### Quids
- Cryptographic identities with public/private key pairs
- Similar to Bitcoin wallets - the private key is used to sign transactions
- Each quid has a unique ID derived from its public key

### Trust Relationships
- Explicit trust levels (0.0 to 1.0) between quids
- Domain-specific and can have expiration dates
- Can propagate transitively through the network

### Hierarchical Trust Domains
- Structured domains like `2025-spring.elections.williamson.counties.texas.us.gov`
- Each domain can have its own validation rules and trust thresholds
- Domains can inherit properties from parent domains

### Transaction Types

1. **Trust Transactions**: Establish trust relationships between quids
   ```
   Quid A trusts Quid B at level 0.8 for domain X
   ```

2. **Identity Transactions**: Define attributes for quids
   ```
   Quid A declares that Quid B has attributes {...} for domain X
   ```

3. **Title Transactions**: Define ownership relationships
   ```
   Quid A declares that Asset C is owned by Quids D (60%) and E (40%) for domain X
   ```

## Project Structure

```
quidnug/
├── src/core/           # Go node implementation
│   ├── node.go         # Main entry point, QuidnugNode struct
│   ├── types.go        # Type definitions (transactions, blocks, domains)
│   ├── config.go       # Configuration loading from environment
│   ├── handlers.go     # HTTP API handlers
│   ├── validation.go   # Transaction and block validation
│   ├── crypto.go       # ECDSA signing and verification
│   ├── network.go      # Node discovery and broadcasting
│   ├── registry.go     # State registry operations
│   ├── middleware.go   # Rate limiting, request validation
│   └── persistence.go  # Pending transaction persistence
├── clients/js/         # JavaScript client library
├── docs/               # Documentation
│   ├── api-spec.yaml   # OpenAPI 3.0 specification
│   └── integration-guide.md
├── go.mod              # Go module definition
├── Makefile            # Build automation
└── Dockerfile          # Container build
```

## Features

- **Cryptographically Secure**: All transactions are signed with quid private keys
- **Verifiable Trust**: Trust relationships can be cryptographically verified
- **Domain Authority**: Nodes can manage specific trust domains
- **Transitive Trust**: Trust scores propagate through the network
- **Hierarchical Structure**: Support for domain hierarchies like DNS
- **Proof of Trust Consensus**: Blocks are validated by trusted quids in each domain
- **Cross-Domain Queries**: Recursive querying across domain boundaries

## Implementation Details

### Quid Implementation
```go
type Quid struct {
    ID            string                 `json:"id"`        
    PublicKey     []byte                 `json:"publicKey"` 
    Created       int64                  `json:"created"`   
    MetaData      map[string]interface{} `json:"metaData,omitempty"`
}

type QuidKeypair struct {
    Quid         Quid
    PrivateKey   []byte   // Private key used for signing
}
```

### Trust Graph
```go
type TrustEdge struct {
    TrustLevel    float64 `json:"trustLevel"`
    TxID          string  `json:"txId"`
    LastUpdated   int64   `json:"lastUpdated"`
    ValidUntil    int64   `json:"validUntil,omitempty"`
}

type TrustGraph struct {
    // Maps: truster -> trustee -> domain -> edge
    Edges       map[string]map[string]map[string]TrustEdge
}
```

### Trust Domain Structure
```go
type TrustDomain struct {
    FullPath        string             `json:"fullPath"`
    ParentDomain    string             `json:"parentDomain"`
    SubDomains      []string           `json:"subDomains"`
    ValidatorQuids  []string           `json:"validatorQuids"`
    TrustThreshold  float64            `json:"trustThreshold"`
    BlockchainHead  string             `json:"blockchainHead"`
    Validators      map[string]float64 `json:"validators"`
    MerkleRoot      string             `json:"merkleRoot,omitempty"`
}
```

## Getting Started

### Installation

```bash
# Clone the repository
git clone https://github.com/quidnug/quidnug.git
cd quidnug

# Download dependencies
go mod tidy

# Build the node
go build -o quidnug-node ./src/core

# Or use make
make build  # Creates bin/quidnug
```

### Running a Node

```bash
# Run with default settings
./quidnug-node

# Run with custom port
PORT=9000 ./quidnug-node

# Run with a specific domain
DOMAIN="elections.williamson.counties.texas.us.gov" ./quidnug-node
```

## Configuration Reference

| Variable | Default | Description |
|----------|---------|-------------|
| `PORT` | `8080` | HTTP server port |
| `SEED_NODES` | `["seed1.quidnug.net:8080","seed2.quidnug.net:8080"]` | JSON array of seed node addresses |
| `LOG_LEVEL` | `info` | Logging level: `debug`, `info`, `warn`, `error` |
| `BLOCK_INTERVAL` | `60s` | How often to generate new blocks |
| `RATE_LIMIT_PER_MINUTE` | `100` | Max requests per minute per IP |
| `MAX_BODY_SIZE_BYTES` | `1048576` | Max request body size (1MB) |
| `DATA_DIR` | `./data` | Directory for persisted data |
| `SHUTDOWN_TIMEOUT` | `30s` | Graceful shutdown timeout |

### Standalone Mode

For local development without external seed nodes:

```bash
# Windows PowerShell
$env:SEED_NODES='[]'
$env:LOG_LEVEL='debug'
.\quidnug-node.exe

# Linux/macOS
SEED_NODES='[]' LOG_LEVEL=debug ./quidnug-node
```

### Docker Support

```bash
# Build Docker image
docker build -t quidnug-node .

# Run Docker container
docker run -p 8080:8080 quidnug-node
```

## API Endpoints

### Transaction Endpoints
- `POST /api/transactions/trust` - Submit a trust transaction
- `POST /api/transactions/identity` - Submit an identity transaction
- `POST /api/transactions/title` - Submit a title transaction

### Query Endpoints
- `GET /api/trust/{truster}/{trustee}?domain=X` - Get trust level between quids
- `GET /api/identity/{quidId}?domain=X` - Get quid identity
- `GET /api/title/{assetId}?domain=X` - Get asset ownership

### Domain Endpoints
- `GET /api/domains` - List managed domains
- `POST /api/domains` - Register a new domain
- `GET /api/domains/{domain}/query` - Query a specific domain

## Example Usage

### Establishing Trust
```bash
curl -X POST http://localhost:8080/api/transactions/trust -d '{
  "trustee": "quid_b_id",
  "domain": "example.com",
  "trustLevel": 0.8
}'
```

### Defining an Identity
```bash
curl -X POST http://localhost:8080/api/transactions/identity -d '{
  "subjectQuid": "quid_id",
  "domain": "example.com",
  "name": "Example Entity",
  "attributes": {
    "type": "organization",
    "location": "Austin, TX"
  }
}'
```

### Declaring Ownership
```bash
curl -X POST http://localhost:8080/api/transactions/title -d '{
  "assetQuid": "asset_id",
  "domain": "example.com",
  "ownershipMap": [
    {"ownerId": "owner1_id", "percentage": 60.0},
    {"ownerId": "owner2_id", "percentage": 40.0}
  ],
  "titleType": "deed"
}'
```

## Building Applications on Quidnug

The Quidnug platform serves as a foundation for various trust-based applications:

- **Identity Verification Systems**: Establish verifiable digital identities
- **Decentralized Authorization**: Permission systems based on trust levels
- **Supply Chain Tracking**: Verify asset provenance and ownership
- **Credential Issuance**: Issue and verify credentials based on trust
- **Governance Systems**: Create voting and decision-making structures
- **Trust-Based Access Control**: Control resource access based on trust relationships

## License

[MIT License](LICENSE)
