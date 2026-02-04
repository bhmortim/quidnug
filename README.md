# Quidnug

[![CI](https://github.com/quidnug/quidnug/actions/workflows/ci.yml/badge.svg)](https://github.com/quidnug/quidnug/actions/workflows/ci.yml)
[![JS Client CI](https://github.com/quidnug/quidnug/actions/workflows/js-client.yml/badge.svg)](https://github.com/quidnug/quidnug/actions/workflows/js-client.yml)
[![codecov](https://codecov.io/gh/quidnug/quidnug/branch/main/graph/badge.svg)](https://codecov.io/gh/quidnug/quidnug)
[![Go Report Card](https://goreportcard.com/badge/github.com/quidnug/quidnug)](https://goreportcard.com/report/github.com/quidnug/quidnug)

**A decentralized protocol for relational trust, identity, and ownership—where trust is personal, not universal.**

## What is Quidnug?

Quidnug is a cryptographic protocol for establishing and computing trust relationships between entities. Unlike traditional reputation systems that assign a single "trust score" to everyone, Quidnug computes trust *from your perspective* based on your personal network of relationships.

**Think about how trust works in real life:**

When a close friend recommends a contractor, you trust that recommendation more than a stranger's online review. When your doctor refers you to a specialist, that carries more weight than a random name from a directory. Trust isn't universal—it flows through relationships.

Quidnug brings this natural model of trust to digital systems. Every entity (person, organization, device, document) is represented as a **quid**—a cryptographic identity similar to a Bitcoin wallet. Quids establish explicit trust relationships with each other, and the system computes transitive trust through these networks on demand.

```
Traditional System:           Quidnug:
                              
"Bob has 4.5 stars"           "From YOUR perspective, 
                               Bob has 0.72 trust
                               (via your colleague Carol)"
```

This fundamental shift—from absolute scores to relational trust—makes systems more resistant to gaming, more contextual, and more aligned with how humans actually evaluate trustworthiness.

## Why Relational Trust?

### The Problem with Absolute Trust Scores

Traditional reputation systems assign a single, universal score to each entity:

| Problem | Example |
|---------|---------|
| **Gaming & Sybil Attacks** | Fake reviews, purchased followers, bot armies can inflate scores |
| **Context Blindness** | A 5-star plumber rating doesn't tell you if they're good for *your* specific job |
| **No Accountability** | Anonymous reviewers have no stake in their recommendations |
| **Platform Lock-in** | Your reputation is owned by the platform, not you |
| **One Size Fits All** | Everyone sees the same score regardless of their context |

### The Relational Trust Solution

Quidnug addresses these problems by making trust **observer-centric**:

| Principle | How It Works |
|-----------|--------------|
| **Personal** | Trust is computed from *your* perspective through *your* network |
| **Accountable** | Every trust relationship is signed by a real cryptographic identity |
| **Transitive** | Trust flows through intermediaries with natural decay |
| **Contextual** | Different trust domains for different purposes |
| **Portable** | You own your identity and relationships, not any platform |

### Real-World Parallel

Consider how you'd evaluate a potential business partner:

1. You don't know them directly, so you ask around
2. Your trusted colleague Carol says "I worked with them—they're solid"
3. You trust Carol's judgment, so you extend some trust to the partner
4. But not *as much* as if you knew them directly—there's natural decay through intermediaries

Quidnug formalizes this process cryptographically:

```
Your trust in Partner = Your trust in Carol × Carol's trust in Partner
                      = 0.9 × 0.8 = 0.72
```

## Core Concepts

### Quids

A **quid** is a cryptographic identity—the fundamental entity in Quidnug:

- **Public/private key pair** (ECDSA P-256) for signing and verification
- **Unique ID** derived from the SHA-256 hash of the public key (16 hex characters)
- **Self-sovereign**: You control your identity without any central authority
- **Universal**: Can represent people, organizations, devices, documents, or any entity

```go
type Quid struct {
    ID        string                 `json:"id"`        // 16-char hex from public key hash
    PublicKey []byte                 `json:"publicKey"` // ECDSA P-256 public key
    Created   int64                  `json:"created"`   // Unix timestamp
    MetaData  map[string]interface{} `json:"metaData,omitempty"`
}
```

### Trust Relationships

Trust in Quidnug has these key characteristics:

| Characteristic | Description |
|----------------|-------------|
| **Relational** | Always computed from an observer's perspective to a target |
| **Explicit** | Trust levels range from 0.0 (no trust) to 1.0 (full trust) |
| **Signed** | Every trust statement is cryptographically signed |
| **Domain-specific** | Trust can be scoped to specific contexts |
| **Expirable** | Trust relationships can have expiration dates |
| **Transitive** | Trust propagates through networks with multiplicative decay |

### Transitive Trust Computation

When you query trust in someone you don't know directly, Quidnug finds the best path through your network:

```
Direct trust:     A → B (0.8)           = 0.8
Two-hop trust:    A → B (0.8) → C (0.7) = 0.56
Three-hop trust:  A → B → C → D         = 0.8 × 0.7 × 0.9 = 0.504
```

The algorithm:
1. Uses breadth-first search from observer to target
2. Multiplies trust levels along each path (natural decay)
3. Returns the **maximum** trust path when multiple paths exist
4. Respects depth limits (default: 5 hops)

### Proof of Trust Consensus

Unlike traditional blockchains where all nodes agree on the same chain, Quidnug uses **Proof of Trust**—a consensus mechanism where each node validates blocks based on its own relational trust in the block's validator.

#### How It Works

When a node receives a block, it:
1. **Validates cryptographically** (hash, signatures, chain linkage) — all honest nodes agree on this
2. **Evaluates trust subjectively** — computes its relational trust in the block's validator
3. **Assigns an acceptance tier** based on the trust level

#### The Four Acceptance Tiers

| Tier | Condition | Action |
|------|-----------|--------|
| **Trusted** | Trust in validator ≥ domain threshold | Add to main chain, process transactions |
| **Tentative** | Trust > distrust threshold but < trust threshold | Store separately, don't build on it yet |
| **Untrusted** | Trust ≤ distrust threshold | Extract trust data only, don't store block |
| **Invalid** | Cryptographic validation fails | Reject entirely |

#### Why Different Nodes May Have Different Chains

This is **by design**. Consider:

- Alice trusts validators A, B, and C
- Bob trusts validators A, B, and D

Alice and Bob will agree on blocks from A and B, but:
- Alice accepts blocks from C that Bob ignores
- Bob accepts blocks from D that Alice ignores

This isn't a bug—it reflects the reality that trust is relational. Each node maintains a view of the world consistent with entities it trusts.

#### Trust Data Extraction

Even when a block is untrusted, the node extracts trust relationship data from it. This preserves the complete trust graph for pathfinding while maintaining consensus integrity:

```
Cryptographically Valid Block from Untrusted Validator
                    │
                    ▼
    ┌───────────────────────────────────────┐
    │  Extract trust edges as "unverified"  │
    │  (available for discovery queries)    │
    └───────────────────────────────────────┘
                    │
                    ▼
    Block is NOT stored, NOT built upon
```

When querying trust, you can choose whether to include unverified edges (with appropriate discounting) or only use edges from trusted validators.

### Hierarchical Trust Domains

Domains organize trust into contexts, similar to DNS:

```
elections.williamson.counties.texas.us.gov
credentials.doctors.texas.medical-board.gov
titles.travis-county.texas.property
degrees.university-of-texas.edu
certifications.aws.cloud-providers.tech
```

Each domain can have:
- **Independent validators**: Trusted quids that validate transactions
- **Custom trust thresholds**: Minimum trust required for various operations
- **Inheritance**: Child domains can inherit rules from parents

### Domain Configuration & Discovery

Quidnug nodes can be configured to support specific trust domains, enabling domain-focused deployments:

**Domain Restriction**
- Nodes can restrict which domains they process via `SUPPORTED_DOMAINS`
- Supports exact matches (`example.com`) and wildcard patterns (`*.example.com`)
- Empty list means all domains are accepted

**Subdomain Authorization**
- By default, registering a subdomain requires authorization from parent domain validators
- For example, registering `api.example.com` requires at least one `example.com` validator to trust the new domain's validators
- This prevents unauthorized subdomain squatting while maintaining the hierarchical trust model

**Domain Gossip Protocol**
- Nodes automatically advertise their supported domains to the network
- Domain information propagates via gossip with configurable TTL to prevent flooding
- Enables efficient discovery of which nodes support which domains
- New domains are announced immediately when registered

### Transaction Types

| Type | Purpose | Example |
|------|---------|---------|
| **Trust** | Establish trust between quids | "I trust Dr. Smith at 0.9 for medical advice" |
| **Identity** | Define attributes for a quid | "This quid represents Acme Corp, located in Austin" |
| **Title** | Establish ownership relationships | "Property X is owned 60% by Alice, 40% by Bob" |
| **Event** | Record events for subjects | "Profile updated: name changed to Alice Chen" |

### Event Streams

**Event streams** provide an immutable, append-only history of events for any quid or title:

| Characteristic | Description |
|----------------|-------------|
| **Subject-based** | Events are recorded against a subject (quid or title) |
| **Sequenced** | Each event has a monotonically increasing sequence number |
| **Immutable** | Once recorded, events cannot be modified or deleted |
| **Flexible payloads** | Inline payloads up to 64KB, or reference larger content via IPFS |
| **Signed** | Every event is cryptographically signed by the subject owner |

Event streams enable audit trails, activity logs, state change histories, and any use case requiring a verifiable sequence of events tied to an identity or asset.

```go
type EventTransaction struct {
    SubjectID   string                 `json:"subjectId"`   // Quid or asset ID
    SubjectType string                 `json:"subjectType"` // "QUID" or "TITLE"
    Sequence    int64                  `json:"sequence"`    // Auto-assigned if not provided
    EventType   string                 `json:"eventType"`   // e.g., "profile.updated"
    Payload     map[string]interface{} `json:"payload"`     // Inline data (max 64KB)
    PayloadCID  string                 `json:"payloadCid"`  // Or IPFS CID for larger content
}
```

## How It Works

### Scenario: Alice Wants to Hire Bob as a Contractor

Alice doesn't know Bob directly, but wants to assess whether she can trust him for a home renovation project.

**Step 1: Alice queries her trust in Bob**

```bash
curl "http://localhost:8080/api/trust/alice_quid/bob_quid?domain=contractors.home&maxDepth=5"
```

**Step 2: The system traverses Alice's trust network**

```
Alice's Trust Network:
                                    
    Alice ──0.9──► Carol ──0.8──► Bob
      │                            ▲
      └──0.7──► Dave ──0.6──► Eve ─┘
                              0.5
```

**Step 3: The system finds multiple paths and selects the best one**

```
Path 1: Alice → Carol → Bob     = 0.9 × 0.8 = 0.72  ← Best path
Path 2: Alice → Dave → Eve → Bob = 0.7 × 0.6 × 0.5 = 0.21
```

**Step 4: Alice receives the result with full transparency**

```json
{
  "observer": "alice_quid",
  "target": "bob_quid",
  "trustLevel": 0.72,
  "trustPath": ["alice_quid", "carol_quid", "bob_quid"],
  "pathDepth": 2,
  "domain": "contractors.home"
}
```

**Step 5: Alice can make an informed decision**

Alice sees that her trust in Bob (0.72) comes through Carol, a longtime colleague she trusts highly. This context helps her understand *why* she might trust Bob—it's essentially Carol's recommendation, weighted by Alice's trust in Carol.

## Use Cases & Applications

Quidnug provides infrastructure for building trust-aware applications across many domains.

### Identity & Authentication

**Self-Sovereign Identity**

Users control their own cryptographic identity without relying on central authorities like Google, Facebook, or government databases.

```javascript
// User creates their own identity
const myQuid = await quidnugClient.generateQuid();
// ID: "a1b2c3d4e5f6g7h8" - derived from public key, owned by user

// Register identity attributes
await quidnugClient.submitTransaction({
  type: "IDENTITY",
  quidId: myQuid.id,
  name: "Alice Chen",
  attributes: { profession: "Software Engineer", location: "Austin, TX" }
});
```

**Passwordless Authentication**

Authenticate users through cryptographic signatures instead of passwords:

```javascript
// Server sends a challenge
const challenge = crypto.randomBytes(32).toString('hex');

// User signs the challenge with their quid's private key
const signature = await quidnugClient.signData(challenge, userQuid.privateKey);

// Server verifies the signature matches the quid's public key
const isValid = await quidnugClient.verifySignature(userQuid.id, challenge, signature);
```

**Example: Job Platform with Verified Credentials**

A job platform where employers verify candidates through trusted credential issuers:

```javascript
// Employer queries trust in a candidate's university credential
const credentialTrust = await quidnugClient.getTrustLevel(
  employerQuid,              // Observer: the employer
  universityQuid,            // Target: the university that issued the degree
  "credentials.education",
  { maxDepth: 4 }
);

if (credentialTrust.trustLevel >= 0.8) {
  // Employer trusts this university (directly or through accreditation bodies)
  const degree = await quidnugClient.getIdentity(candidateDegreeQuid, "credentials.education");
  // Verify the degree was actually issued by this trusted university
}
```

### Decentralized Reputation & Reviews

**Personalized Product Reviews**

Instead of seeing the same 4.5-star average as everyone else, see ratings weighted by your trust network:

```javascript
// Traditional: "This product has 4.2 stars from 1,247 reviews"

// Quidnug: "From YOUR network's perspective..."
const reviewers = await getProductReviewers(productId);
let weightedScore = 0;
let totalWeight = 0;

for (const reviewer of reviewers) {
  const trust = await quidnugClient.getTrustLevel(
    myQuid,           // Your perspective
    reviewer.quidId,  // The reviewer
    "reviews.products"
  );
  
  if (trust.trustLevel > 0) {
    weightedScore += reviewer.rating * trust.trustLevel;
    totalWeight += trust.trustLevel;
  }
}

const myPersonalizedRating = weightedScore / totalWeight;
// "Based on people YOU trust, this product rates 4.7"
```

**Example: Service Provider Marketplace**

A marketplace where contractor ratings come from your trusted network:

```javascript
// Find contractors for home renovation
const contractors = await searchContractors("plumbing", "Austin, TX");

// Rank by YOUR trust, not global average
const rankedContractors = await Promise.all(contractors.map(async (contractor) => {
  const trust = await quidnugClient.getTrustLevel(
    homeownerQuid,
    contractor.quidId,
    "contractors.home-services.texas"
  );
  return { ...contractor, personalTrust: trust.trustLevel, trustPath: trust.trustPath };
}));

// Sort by personal trust score
rankedContractors.sort((a, b) => b.personalTrust - a.personalTrust);

// Display with trust context:
// "Mike's Plumbing - 0.81 trust (via your neighbor Sarah)"
// "Joe's Pipes - 0.65 trust (via your coworker Tom → his brother)"
```

### Supply Chain & Provenance

**Asset Tracking with Cryptographic Proof**

Track ownership history with immutable, signed records:

```javascript
// Register a new asset
await quidnugClient.submitTransaction({
  type: "TITLE",
  assetId: "wine_bottle_lot_2024_001",
  domain: "provenance.wine.napa-valley",
  owners: [{ ownerId: vineyardQuid, percentage: 100.0 }],
  titleType: "certificate_of_origin"
});

// Transfer through supply chain (each step is signed by current owner)
await quidnugClient.submitTransaction({
  type: "TITLE",
  assetId: "wine_bottle_lot_2024_001",
  domain: "provenance.wine.napa-valley",
  owners: [{ ownerId: distributorQuid, percentage: 100.0 }],
  previousOwners: [{ ownerId: vineyardQuid, percentage: 100.0 }],
  signatures: { [vineyardQuid]: vineyardSignature }
});
```

**Example: Luxury Goods Authentication**

Verify authenticity through a chain of trusted parties:

```javascript
// Consumer wants to verify a luxury watch is authentic
const watchProvenance = await quidnugClient.getAssetOwnership(
  watchSerialQuid,
  "authenticity.luxury.watches"
);

// Check if the manufacturer in the provenance is trusted
const manufacturerTrust = await quidnugClient.getTrustLevel(
  consumerQuid,                    // Buyer's perspective
  watchProvenance.originalOwner,   // Claimed manufacturer
  "authenticity.luxury.watches"
);

if (manufacturerTrust.trustLevel >= 0.9) {
  // Trace the chain: Manufacturer → Authorized Dealer → Current Seller
  const chainValid = await verifyOwnershipChain(watchProvenance.ownershipHistory);
  // Each transfer was signed by the previous owner
}
```

### Professional Credentials

**License Verification Through Trust Chains**

Verify professional licenses through trusted regulatory authorities:

```javascript
// Healthcare platform verifying a doctor's license
const doctorLicense = await quidnugClient.getIdentity(
  doctorQuid,
  "credentials.doctors.texas.medical-board.gov"
);

// Platform checks if it trusts the issuing medical board
const medicalBoardTrust = await quidnugClient.getTrustLevel(
  healthcarePlatformQuid,  // Platform's perspective
  doctorLicense.issuer,    // Texas Medical Board quid
  "credentials.medical"
);

// Also verify the license hasn't expired
if (medicalBoardTrust.trustLevel >= 0.95 && doctorLicense.validUntil > Date.now()) {
  // License is valid and issued by a trusted authority
}
```

**Example: Academic Credential Verification**

Verify degrees through trusted educational institutions:

```javascript
// Employer verifies a candidate's degree
const degree = await quidnugClient.getIdentity(
  candidateDegreeQuid,
  "credentials.education.degrees"
);

// Check trust path: Employer → Accreditation Body → University
const universityTrust = await quidnugClient.getTrustLevel(
  employerQuid,
  degree.issuerQuid,  // University that issued the degree
  "credentials.education.degrees"
);

console.log(`Trust in ${degree.institutionName}: ${universityTrust.trustLevel}`);
console.log(`Trust path: ${universityTrust.trustPath.join(" → ")}`);
// "Trust path: employer_quid → accreditation_board → university_quid"
```

### Governance & Voting

**Trust-Weighted Voting**

Weight votes by the voter's trust relationship to the decision context:

```javascript
// DAO proposal voting where votes are weighted by trust
const proposal = await getProposal(proposalId);
const votes = await getVotesForProposal(proposalId);

let weightedYes = 0;
let weightedNo = 0;

for (const vote of votes) {
  // Compute voter's trust from the DAO's perspective
  const voterTrust = await quidnugClient.getTrustLevel(
    daoQuid,           // DAO's perspective
    vote.voterQuid,    // The voter
    "governance.dao.proposals"
  );
  
  if (vote.choice === "yes") {
    weightedYes += voterTrust.trustLevel;
  } else {
    weightedNo += voterTrust.trustLevel;
  }
}

const result = weightedYes > weightedNo ? "PASSED" : "FAILED";
```

**Example: Open Source Project Governance**

Maintainer decisions weighted by community trust:

```javascript
// Evaluate a pull request merge decision
const prApprovals = await getPullRequestApprovals(prId);

let totalApprovalWeight = 0;
for (const approval of prApprovals) {
  const maintainerTrust = await quidnugClient.getTrustLevel(
    projectQuid,           // Project's perspective
    approval.reviewerQuid, // The approving maintainer
    "governance.opensource.code-review"
  );
  totalApprovalWeight += maintainerTrust.trustLevel;
}

// Require approval weight >= 2.0 (e.g., two highly trusted maintainers or several contributors)
if (totalApprovalWeight >= 2.0) {
  await mergePullRequest(prId);
}
```

### Financial & Legal

**Multi-Signature Escrow**

Require multiple trusted parties to release funds:

```javascript
// Create escrow requiring signatures from buyer, seller, and arbiter
const escrowTx = await quidnugClient.submitTransaction({
  type: "TITLE",
  assetId: escrowFundsQuid,
  domain: "financial.escrow",
  owners: [{ ownerId: escrowContractQuid, percentage: 100.0 }],
  signatures: {
    [buyerQuid]: buyerSignature,
    [sellerQuid]: sellerSignature,
    [arbiterQuid]: arbiterSignature
  },
  attributes: {
    releaseConditions: "2_of_3_signatures",
    participants: [buyerQuid, sellerQuid, arbiterQuid]
  }
});
```

**Example: Peer-to-Peer Lending**

Assess creditworthiness through trust networks:

```javascript
// Lender evaluates borrower through their trust network
const borrowerTrust = await quidnugClient.getTrustLevel(
  lenderQuid,
  borrowerQuid,
  "financial.lending.p2p"
);

// Also check trust in borrower's references
const references = await getBorrowerReferences(borrowerQuid);
let referenceScore = 0;

for (const ref of references) {
  const refTrust = await quidnugClient.getTrustLevel(lenderQuid, ref.quidId, "financial.lending.p2p");
  referenceScore += refTrust.trustLevel * ref.endorsementStrength;
}

// Combine direct trust and reference trust for lending decision
const creditScore = (borrowerTrust.trustLevel * 0.6) + (referenceScore * 0.4);
const maxLoanAmount = creditScore * BASE_LOAN_LIMIT;
```

## Quick Start

### 1. Start a Node

```bash
# Build and run
go build -o quidnug-node ./src/core
SEED_NODES='[]' LOG_LEVEL=debug ./quidnug-node

# Or use Docker
docker run -p 8080:8080 quidnug-node
```

### 2. Create Your Identity

```bash
# Generate a new quid (your cryptographic identity)
curl -X POST http://localhost:8080/api/quids -H "Content-Type: application/json" -d '{
  "name": "Alice",
  "metadata": {"type": "individual", "location": "Austin, TX"}
}'

# Response:
# {
#   "quid": {
#     "id": "a1b2c3d4e5f6g7h8",
#     "publicKey": "BHx2F8...",
#     "created": 1699900000
#   },
#   "privateKey": "MHQCAQEEIGx2..."  <-- Store this securely!
# }
```

### 3. Establish Trust

```bash
# Trust someone you know directly
curl -X POST http://localhost:8080/api/transactions/trust -H "Content-Type: application/json" -d '{
  "truster": "a1b2c3d4e5f6g7h8",
  "trustee": "b2c3d4e5f6g7h8i9",
  "trustDomain": "professional.network",
  "trustLevel": 0.9,
  "description": "Worked together for 3 years at Acme Corp",
  "signature": "MEUCIQDx2...",
  "publicKey": "BHx2F8..."
}'

# Response:
# {
#   "txId": "tx_abc123...",
#   "status": "accepted",
#   "timestamp": 1699900100
# }
```

### 4. Query Relational Trust

```bash
# Find your trust in someone you don't know directly
curl "http://localhost:8080/api/trust/a1b2c3d4e5f6g7h8/c3d4e5f6g7h8i9j0?domain=professional.network&maxDepth=5"

# Response:
# {
#   "observer": "a1b2c3d4e5f6g7h8",
#   "target": "c3d4e5f6g7h8i9j0",
#   "trustLevel": 0.72,
#   "trustPath": ["a1b2c3d4e5f6g7h8", "b2c3d4e5f6g7h8i9", "c3d4e5f6g7h8i9j0"],
#   "pathDepth": 2,
#   "domain": "professional.network"
# }
```

### 5. Define Identity Attributes

```bash
curl -X POST http://localhost:8080/api/transactions/identity -H "Content-Type: application/json" -d '{
  "quidId": "org_quid_id",
  "trustDomain": "business.example.com",
  "name": "Acme Corporation",
  "description": "Leading provider of anvils and rocket-powered products",
  "attributes": {
    "type": "organization",
    "industry": "manufacturing",
    "founded": 1920,
    "headquarters": "Austin, TX"
  },
  "creator": "a1b2c3d4e5f6g7h8",
  "signature": "MEUCIQDy3...",
  "publicKey": "BHx2F8..."
}'
```

### 6. Declare Asset Ownership

```bash
curl -X POST http://localhost:8080/api/transactions/title -H "Content-Type: application/json" -d '{
  "assetId": "property_123_main_st",
  "trustDomain": "titles.travis-county.texas.property",
  "owners": [
    {"ownerId": "alice_quid", "percentage": 60.0},
    {"ownerId": "bob_quid", "percentage": 40.0}
  ],
  "titleType": "deed",
  "signatures": {
    "alice_quid": "MEUCIQDa1...",
    "bob_quid": "MEUCIQDb2..."
  }
}'
```

### 7. Record an Event

```bash
# Record an event for a quid
curl -X POST http://localhost:8080/api/transactions/event -H "Content-Type: application/json" -d '{
  "subjectId": "a1b2c3d4e5f6g7h8",
  "subjectType": "QUID",
  "eventType": "profile.updated",
  "trustDomain": "example.com",
  "payload": {"field": "name", "newValue": "Alice Chen"},
  "signature": "...",
  "publicKey": "..."
}'

# Response:
# {
#   "status": "success",
#   "transaction_id": "evt_abc123...",
#   "sequence": 1
# }
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
make build  # Creates bin/quidnug

# Or build directly
go build -o quidnug-node ./src/core
```

### Running a Node

```bash
# Run with default settings (connects to seed nodes)
./quidnug-node

# Run in standalone mode (local development)
SEED_NODES='[]' LOG_LEVEL=debug ./quidnug-node

# Run with custom port and domain
PORT=9000 DOMAIN="myapp.example.com" ./quidnug-node
```

### Docker Support

```bash
# Build Docker image
docker build -t quidnug-node .

# Run container
docker run -p 8080:8080 -e SEED_NODES='[]' quidnug-node

# Run with persistent data
docker run -p 8080:8080 -v quidnug-data:/data quidnug-node
```

## Configuration Reference

Quidnug can be configured through environment variables and/or a YAML/JSON configuration file. The configuration precedence is (highest to lowest):

1. **Environment variables** - Always take priority
2. **Config file** - Values from YAML or JSON file
3. **Default values** - Built-in defaults

### Config File

Create a `config.yaml` file (see `config.example.yaml` for a documented template):

```yaml
port: "8080"
seed_nodes:
  - "seed1.quidnug.net:8080"
  - "seed2.quidnug.net:8080"
log_level: "info"
block_interval: "60s"
rate_limit_per_minute: 100
max_body_size_bytes: 1048576
data_dir: "./data"
shutdown_timeout: "30s"
http_client_timeout: "5s"
node_auth_secret: ""
require_node_auth: false
trust_cache_ttl: "60s"
supported_domains: []
allow_domain_registration: true
require_parent_domain_auth: true
domain_gossip_interval: "2m"
domain_gossip_ttl: 3
```

**Config file search order:**
1. Path specified by `CONFIG_FILE` environment variable
2. `./config.yaml`
3. `./config.json`
4. `/etc/quidnug/config.yaml`

### Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `CONFIG_FILE` | *(auto-detected)* | Path to config file (YAML or JSON) |
| `PORT` | `8080` | HTTP server port |
| `SEED_NODES` | `["seed1.quidnug.net:8080","seed2.quidnug.net:8080"]` | JSON array of seed node addresses |
| `LOG_LEVEL` | `info` | Logging level: `debug`, `info`, `warn`, `error` |
| `BLOCK_INTERVAL` | `60s` | How often to generate new blocks |
| `RATE_LIMIT_PER_MINUTE` | `100` | Max requests per minute per IP |
| `MAX_BODY_SIZE_BYTES` | `1048576` | Max request body size (1MB) |
| `DATA_DIR` | `./data` | Directory for persisted data |
| `SHUTDOWN_TIMEOUT` | `30s` | Graceful shutdown timeout |
| `HTTP_CLIENT_TIMEOUT` | `5s` | Timeout for outgoing HTTP requests |
| `NODE_AUTH_SECRET` | *(empty)* | Shared secret for node-to-node authentication |
| `REQUIRE_NODE_AUTH` | `false` | Set to `true` to require authenticated node communication |
| `QUIDNUG_IPFS_ENABLED` | `false` | Enable IPFS integration for large payloads |
| `QUIDNUG_IPFS_GATEWAY_URL` | `http://localhost:5001` | IPFS API gateway URL |
| `QUIDNUG_IPFS_TIMEOUT` | `30s` | Timeout for IPFS operations |
| `TRUST_CACHE_TTL` | `60s` | TTL for cached trust computation results |
| `SUPPORTED_DOMAINS` | `[]` | JSON array of supported domain patterns (empty = all allowed) |
| `ALLOW_DOMAIN_REGISTRATION` | `true` | Set to `false` to disable dynamic domain registration |
| `REQUIRE_PARENT_DOMAIN_AUTH` | `true` | Set to `false` to allow subdomains without parent validator trust |
| `DOMAIN_GOSSIP_INTERVAL` | `2m` | Interval between domain gossip broadcasts |
| `DOMAIN_GOSSIP_TTL` | `3` | Hop count before gossip messages stop propagating |

### Configuration Examples

```bash
# Windows PowerShell - Standalone mode
$env:SEED_NODES='[]'
$env:LOG_LEVEL='debug'
.\bin\quidnug.exe

# Linux/macOS - Standalone mode
SEED_NODES='[]' LOG_LEVEL=debug ./bin/quidnug

# Production with custom settings
PORT=443 \
SEED_NODES='["node1.example.com:8080","node2.example.com:8080"]' \
LOG_LEVEL=info \
DATA_DIR=/var/lib/quidnug \
./bin/quidnug
```

## API Endpoints

### Transaction Endpoints

| Method | Endpoint | Description |
|--------|----------|-------------|
| `POST` | `/api/transactions/trust` | Submit a trust transaction |
| `POST` | `/api/transactions/identity` | Submit an identity transaction |
| `POST` | `/api/transactions/title` | Submit a title transaction |
| `POST` | `/api/transactions/event` | Submit an event transaction |

### Event Endpoints

| Method | Endpoint | Description |
|--------|----------|-------------|
| `GET` | `/api/events/streams/{subjectId}` | Get event stream metadata |
| `GET` | `/api/events/streams/{subjectId}/events` | Get paginated events |

### IPFS Endpoints

| Method | Endpoint | Description |
|--------|----------|-------------|
| `POST` | `/api/ipfs/pin` | Pin content to IPFS |
| `GET` | `/api/ipfs/{cid}` | Retrieve content from IPFS |

### Query Endpoints

| Method | Endpoint | Description |
|--------|----------|-------------|
| `GET` | `/api/trust/{observer}/{target}` | Get relational trust from observer to target |
| `POST` | `/api/trust/query` | Query relational trust with full options |
| `GET` | `/api/trust/edges/{quidId}` | Get trust edges with provenance |
| `GET` | `/api/identity/{quidId}` | Get quid identity attributes |
| `GET` | `/api/title/{assetId}` | Get asset ownership information |

### Block Endpoints

| Method | Endpoint | Description |
|--------|----------|-------------|
| `GET` | `/api/blocks` | Get blockchain data |
| `GET` | `/api/blocks/tentative/{domain}` | Get tentative blocks for a domain |

### Domain Endpoints

| Method | Endpoint | Description |
|--------|----------|-------------|
| `GET` | `/api/domains` | List domains managed by this node |
| `POST` | `/api/domains` | Register a new domain |
| `GET` | `/api/domains/{domain}/query` | Query a specific domain |
| `GET` | `/api/v1/node/domains` | Get this node's supported domains |
| `POST` | `/api/v1/node/domains` | Update this node's supported domains |
| `POST` | `/api/v1/gossip/domains` | Receive domain gossip (node-to-node) |

### System Endpoints

| Method | Endpoint | Description |
|--------|----------|-------------|
| `GET` | `/api/health` | Node health check |
| `GET` | `/api/status` | Node status and statistics |

## Example API Responses

### Querying Relational Trust

```bash
curl "http://localhost:8080/api/trust/alice123/charlie789?domain=contractors.example.com&maxDepth=4"
```

**Response:**
```json
{
  "observer": "alice123",
  "target": "charlie789",
  "trustLevel": 0.72,
  "trustPath": ["alice123", "bob456", "charlie789"],
  "pathDepth": 2,
  "domain": "contractors.example.com"
}
```

### Getting Identity

```bash
curl "http://localhost:8080/api/identity/org_abc123?domain=business.example.com"
```

**Response:**
```json
{
  "quidId": "org_abc123",
  "name": "Acme Corporation",
  "description": "Leading provider of innovative solutions",
  "attributes": {
    "type": "organization",
    "industry": "technology",
    "employees": 500,
    "headquarters": "Austin, TX"
  },
  "creator": "founder_quid",
  "created": 1699900000,
  "lastUpdated": 1699950000
}
```

### Getting Asset Ownership

```bash
curl "http://localhost:8080/api/title/property_123?domain=titles.travis-county.texas.property"
```

**Response:**
```json
{
  "assetId": "property_123",
  "owners": [
    {"ownerId": "alice_quid", "percentage": 60.0, "stakeType": "fee_simple"},
    {"ownerId": "bob_quid", "percentage": 40.0, "stakeType": "fee_simple"}
  ],
  "titleType": "deed",
  "created": 1699900000,
  "lastTransfer": 1699950000,
  "domain": "titles.travis-county.texas.property"
}
```

## Comparison with Other Systems

| Feature | Quidnug | Traditional Reputation | Blockchain Identity |
|---------|---------|------------------------|---------------------|
| **Trust Model** | Relational (observer-specific) | Absolute (global score) | Absolute (attestations) |
| **Gaming Resistance** | High (can't fake relationships) | Low (Sybil attacks, fake reviews) | Medium (attestation spam) |
| **Context-Aware** | Yes (domain-specific trust) | No (same score everywhere) | Sometimes (depends on implementation) |
| **Privacy** | You control your data | Platform controls your data | Public by default |
| **Decentralized** | Yes (no central authority) | No (platform-controlled) | Yes |
| **Transitive Trust** | Yes (with natural decay) | No | Rarely |
| **Trust Explanation** | Yes (shows trust path) | No (opaque algorithm) | Sometimes |
| **Portability** | Full (cryptographic ownership) | None (locked to platform) | Partial (chain-specific) |

## Trust Domain Examples

Domains organize trust into hierarchical contexts. Here are examples across industries:

### Government
```
elections.williamson.counties.texas.us.gov
permits.building.austin.cities.texas.us.gov
licenses.drivers.texas.dmv.gov
```

### Healthcare
```
credentials.doctors.texas.medical-board.gov
credentials.nurses.texas.nursing-board.gov
prescriptions.controlled-substances.dea.gov
```

### Real Estate
```
titles.travis-county.texas.property
deeds.residential.travis-county.texas.property
liens.commercial.travis-county.texas.property
```

### Education
```
degrees.undergraduate.university-of-texas.edu
transcripts.graduate.stanford.edu
certifications.continuing-education.coursera.edu
```

### Professional & Technology
```
certifications.aws.cloud-providers.tech
certifications.kubernetes.cncf.io
licenses.cpa.texas.accountancy-board.gov
```

### Finance
```
accounts.checking.chase.banks.us
loans.mortgage.wells-fargo.banks.us
insurance.auto.state-farm.insurance.us
```

## Project Structure

```
quidnug/
├── src/core/              # Go node implementation
│   ├── node.go            # Main entry point, QuidnugNode struct
│   ├── types.go           # Type definitions (transactions, blocks, domains)
│   ├── config.go          # Configuration loading from environment
│   ├── handlers.go        # HTTP API handlers
│   ├── validation.go      # Transaction and block validation
│   ├── crypto.go          # ECDSA signing and verification
│   ├── network.go         # Node discovery and broadcasting
│   ├── registry.go        # State registry and trust computation
│   ├── middleware.go      # Rate limiting, request validation
│   └── persistence.go     # Transaction persistence
├── clients/js/            # JavaScript client library
│   ├── quidnug-client.js  # Client implementation
│   └── quidnug-client.test.js
├── docs/                  # Documentation
│   ├── api-spec.yaml      # OpenAPI 3.0 specification
│   ├── integration-guide.md
│   └── architecture.md
├── go.mod                 # Go module definition
├── Makefile               # Build automation
├── Dockerfile             # Container build
└── README.md
```

## Building from Source

```bash
# Prerequisites: Go 1.21+

# Clone and enter directory
git clone https://github.com/quidnug/quidnug.git
cd quidnug

# Download dependencies
go mod tidy

# Run tests
make test
# Or: go test -race ./...

# Build binary
make build
# Or: go build -o bin/quidnug ./src/core

# Run
./bin/quidnug
```

## Running Tests

### Unit Tests

Unit tests run quickly and don't require network access:

```bash
make test
# Or: go test -v -race ./...
```

### Integration Tests

Integration tests verify distributed protocol behavior across multiple nodes. They are tagged separately to allow faster unit test runs:

```bash
make test-integration
# Or: go test -v -race -tags=integration ./...
```

Integration tests cover:
- **Transaction propagation**: Verifies transactions broadcast to all cluster nodes
- **Block synchronization**: Tests block generation and reception across nodes
- **Cross-domain queries**: Tests hierarchical domain walking for trust queries
- **Node discovery**: Verifies nodes can discover each other through seed nodes
- **Graceful shutdown**: Ensures clean shutdown during block generation
- **Multi-node trust computation**: Validates consistent trust computation across nodes

**Note**: Integration tests start multiple HTTP servers on high ports (19000+) and may take longer to run.

## Additional Resources

- **[Integration Guide](docs/integration-guide.md)**: Detailed guide for building applications
- **[OpenAPI Specification](docs/openapi.yaml)**: OpenAPI 3.0 specification for the REST API
- **[Architecture](docs/architecture.md)**: System design and internals
- **[JavaScript Client](clients/js/)**: Browser and Node.js client library

## Contributing

Contributions are welcome! Please see our contributing guidelines and code of conduct.

```bash
# Run tests before submitting
make test

# Run linter
make lint
```

## License

[MIT License](LICENSE)
