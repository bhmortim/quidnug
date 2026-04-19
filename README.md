# Quidnug

[![CI](https://github.com/quidnug/quidnug/actions/workflows/ci.yml/badge.svg)](https://github.com/quidnug/quidnug/actions/workflows/ci.yml)
[![codecov](https://codecov.io/gh/quidnug/quidnug/branch/main/graph/badge.svg)](https://codecov.io/gh/quidnug/quidnug)
[![Go Report Card](https://goreportcard.com/badge/github.com/quidnug/quidnug)](https://goreportcard.com/report/github.com/quidnug/quidnug)
[![License: Apache 2.0](https://img.shields.io/badge/License-Apache%202.0-blue.svg)](LICENSE)

> **A decentralized protocol for relational trust, identity, ownership, and
> auditable state — where trust is personal, cryptographic, and contextual.**

## TL;DR

Quidnug is a **P2P protocol and Go reference node** that lets your application
answer questions like:

- "From **my** perspective, how much should I trust this counterparty?"
- "Did **this specific key** authorize this action, and can I recover if it's
  lost or compromised?"
- "Has this asset been passed through a chain of parties I can verify?"
- "Has **this** event truly happened in a tamper-evident ledger?"

It does this with a typed trust graph, per-signer replay-safe nonces,
M-of-N guardian-based key recovery with time-lock vetoes, cross-domain gossip
with signed fingerprints, and a proof-of-trust consensus where **each node
independently decides which blocks to trust based on its own relational view
of the signer**.

Think of it as: **a trust graph + a tamper-evident event log + key
lifecycle primitives, designed for systems where trust is not one-size-
fits-all and where losing a key shouldn't lose the business.**

---

## Who should use Quidnug?

You'll get the most value if you're building:

| If you build...                                    | Quidnug gives you...                                         |
|----------------------------------------------------|--------------------------------------------------------------|
| A **financial system** with multi-party approval    | M-of-N signing + time-locked recovery + replay protection    |
| An **AI platform** where provenance matters         | Signed supply chain of training data → model → inference     |
| A **credentialing system**                          | Revocable issuer trust + transitive verification             |
| A **marketplace** with reputation                   | Trust personalized to each viewer's network                  |
| A **custody / escrow / vault** product              | Guardian recovery, epoch-based key rotation                  |
| Any system where **"who signed this?"** matters     | ECDSA P-256 identities + on-chain rotation audit trail       |

You're **probably not** the target audience if:

- You need a high-throughput payment chain (Quidnug targets auditability and
  correctness over raw TPS).
- You want a single universal reputation score (the protocol deliberately
  refuses to produce one).
- Your users expect "forgot password" email flows for key recovery (you can
  build that on top, but the protocol is cryptographic).

---

## A 30-second demo

Start a node, create an identity, record a trust relationship, query it back:

```bash
# 1. Run a node
make build && ./bin/quidnug &
# -> listening on :8080

# 2. Create Alice's identity (your application would keep the key)
curl -X POST http://localhost:8080/api/identities \
  -d '{"quidId":"alice","name":"Alice","creator":"alice"}'

# 3. Alice declares she trusts Bob at 0.9 for "contractors.home"
curl -X POST http://localhost:8080/api/trust \
  -d '{"truster":"alice","trustee":"bob","trustLevel":0.9,"domain":"contractors.home"}'

# 4. Query trust FROM Alice's perspective
curl "http://localhost:8080/api/trust/alice/bob?domain=contractors.home"
# -> {"trustLevel":0.9,"trustPath":["alice","bob"],"pathDepth":1,...}
```

Now from **Carol's** perspective — if Carol trusts Alice at 0.8 and Alice
trusts Bob at 0.9, Carol transitively trusts Bob at 0.72:

```
curl "http://localhost:8080/api/trust/carol/bob?domain=contractors.home"
# -> {"trustLevel":0.72,"trustPath":["carol","alice","bob"],"pathDepth":2}
```

**The same target (Bob) has different trust levels from different observers.
That's the whole point.**

---

## Key capabilities

| Capability                 | What it means for you                                                               | See                                                         |
|----------------------------|-------------------------------------------------------------------------------------|-------------------------------------------------------------|
| **Relational trust**       | Every node computes trust from **its own perspective**. No universal score.         | [Core concepts](#core-concepts)                             |
| **Typed transactions**     | Trust, Identity, Title, Event, Anchor, Guardian, ForkBlock — all signed.            | [Transactions](#transactions)                               |
| **Proof-of-Trust consensus** | Acceptance tiered (Trusted / Tentative / Untrusted / Invalid) per observer.        | [docs/architecture.md](docs/architecture.md)                |
| **Per-signer monotonic nonces** | Strong replay protection without locking or global coordination.                 | [QDP-0001](docs/design/0001-global-nonce-ledger.md)         |
| **Guardian-based recovery** | Lost or compromised keys? Recover via M-of-N guardian quorum with time-lock veto.  | [QDP-0002](docs/design/0002-guardian-based-recovery.md)     |
| **Cross-domain gossip**    | Rotations in one domain propagate to others via signed fingerprints.                | [QDP-0003](docs/design/0003-cross-domain-nonce-scoping.md)  |
| **Push gossip**            | Fresh rotations reach interested peers in seconds, not polling cycles.              | [QDP-0005](docs/design/0005-push-based-gossip.md)           |
| **K-of-K bootstrap**       | Fresh nodes seed state from quorum of trusted peers — no blind single-source trust.| [QDP-0008](docs/design/0008-kofk-bootstrap.md)              |
| **Fork-block activation**  | Coordinate protocol changes on-chain at a future block height.                      | [QDP-0009](docs/design/0009-fork-block-trigger.md)          |
| **Compact Merkle proofs**  | Light-client-friendly anchor verification — ~70% less gossip bandwidth.             | [QDP-0010](docs/design/0010-compact-merkle-proofs.md)       |
| **Event streams**          | Tamper-evident append-only event logs bound to any quid or title.                   | [Transactions](#transactions)                               |

---

## Use cases

A dozen detailed use-case designs live under [`UseCases/`](UseCases/). Each
folder is a self-contained implementation plan with problem statement,
architecture, Quidnug-specific integration, concrete code, and threat model.

**FinTech:**

- [`UseCases/interbank-wire-authorization/`](UseCases/interbank-wire-authorization/)
  — M-of-N cosigning for wire transfers with replay protection & guardian
  recovery for stale signing keys.
- [`UseCases/merchant-fraud-consortium/`](UseCases/merchant-fraud-consortium/)
  — Cross-merchant fraud signal sharing with relational trust in the
  reporter, not global reputation.
- [`UseCases/defi-oracle-network/`](UseCases/defi-oracle-network/) —
  Decentralized price/data oracles where consumers personalize trust in each
  reporter.
- [`UseCases/institutional-custody/`](UseCases/institutional-custody/) —
  High-value crypto custody with guardian-recovery and epoch-based
  key-rotation auditing.
- [`UseCases/b2b-invoice-financing/`](UseCases/b2b-invoice-financing/) —
  Supply-chain invoice validation with multi-party trust chains.

**AI:**

- [`UseCases/ai-model-provenance/`](UseCases/ai-model-provenance/) — Signed
  supply chain from training-data attestation through model weights to
  inference outputs.
- [`UseCases/ai-agent-authorization/`](UseCases/ai-agent-authorization/) —
  Time-locked capability grants for autonomous AI agents with guardian
  veto / emergency revocation.
- [`UseCases/federated-learning-attestation/`](UseCases/federated-learning-attestation/)
  — Gradient contribution proofs across untrusting participants.
- [`UseCases/ai-content-authenticity/`](UseCases/ai-content-authenticity/) —
  C2PA-style media provenance with editing trust chains.

**Government / Elections:**

- [`UseCases/elections/`](UseCases/elections/) — Full-stack
  election design: bring-your-own voter quid, authority-signed
  registration trust edges replacing the voter registration
  database, precinct-scoped poll books as queries, blind-
  signature ballot issuance for anonymity with eligibility,
  votes-as-trust-edges, paper-ballot parity, instant recount
  anyone can run, and individual voter verification. The most
  detailed use case in the library.

**Cross-industry high-value:**

- [`UseCases/healthcare-consent-management/`](UseCases/healthcare-consent-management/)
  — Patient-controlled record access with M-of-N emergency override.
- [`UseCases/credential-verification-network/`](UseCases/credential-verification-network/)
  — Revocable issuer trust for diplomas, licenses, certifications.
- [`UseCases/developer-artifact-signing/`](UseCases/developer-artifact-signing/)
  — Code-signing with guardian-recoverable keys — kill the GPG single-point-
  of-failure problem.

Each folder contains a full `README.md` (problem + why Quidnug + high-level
design), `architecture.md` (data model + sequence diagrams), `implementation.md`
(concrete API calls + pseudocode), and `threat-model.md`.

---

## Core concepts

### Quids: cryptographic identities

A **quid** is the primitive identity object. It's a public key plus
metadata. The quid ID is the first 16 hex chars of `sha256(publicKey)`.

```go
type Quid struct {
    ID        string  // 16-char hex derived from public key
    PublicKey []byte  // ECDSA P-256 (32 bytes compressed)
    Created   int64
    MetaData  map[string]interface{}
}
```

A quid can represent a person, organization, device, AI agent, document,
contract, or any entity that needs to sign things.

### Trust: relational, observer-centric

Trust in Quidnug is **always** a statement "observer O trusts target T at
level L in domain D", cryptographically signed by O. There is no global
"what is Bob's trust score" — only "from my perspective..." answers.

```
Direct:       A ──0.9──► B                                   = 0.9
Two-hop:      A ──0.9──► B ──0.8──► C                        = 0.72
Three-hop:    A ──0.9──► B ──0.8──► C ──0.7──► D             = 0.504
```

Multiple paths? The algorithm takes the **max** — a single strong
recommendation beats many weak ones. Depth-bounded BFS; default 5 hops.

### Domains: hierarchical context

Domains give trust context. A DNS-like naming scheme lets you scope trust
by use:

```
contractors.home.services
doctors.credentials.texas.medical-board.gov
oracles.price-feeds.ethereum.mainnet
```

A doctor trusted in `doctors.credentials.texas.medical-board.gov` is not
automatically trusted in `oracles.price-feeds.ethereum.mainnet`. Domains
can inherit along a parent/child axis when it makes sense.

### Proof-of-Trust consensus

This is the part that surprises people. Quidnug does **not** produce a
single globally-agreed chain. Instead, each node tiers incoming blocks:

| Tier           | Condition                              | Behavior                                  |
|----------------|----------------------------------------|-------------------------------------------|
| **Trusted**    | Trust in validator ≥ domain threshold  | Added to main chain; transactions applied |
| **Tentative**  | Between distrust and trust thresholds  | Stored separately, not built on           |
| **Untrusted**  | Below distrust threshold               | Only trust-edge data extracted            |
| **Invalid**    | Cryptographic verification fails       | Rejected                                  |

**Why different nodes see different chains.** Alice trusts validators A and
B; Bob trusts A and D. They agree on A-sealed blocks. They disagree about
B's and D's — which is correct, because they disagree about those parties.

This is **not Byzantine Fault Tolerance**. It's trust-aware consensus where
the observer's context determines acceptance. It fits domains where "one
chain for everyone" isn't the goal — private/consortium/ federated
deployments, where parties disagree about who to trust.

### Transactions

Seven core transaction types, each signed and anchored into blocks:

| Type                    | Purpose                                                   |
|-------------------------|-----------------------------------------------------------|
| `TRUST`                 | Declare trust from one quid to another in a domain.       |
| `IDENTITY`              | Name, attributes, home domain for a quid.                 |
| `TITLE`                 | Asset ownership (full or fractional).                     |
| `EVENT`                 | Append a signed event to a subject's stream.              |
| `ANCHOR`                | Rotate key epoch, cap old epoch, invalidate epoch.        |
| `GUARDIAN_SET_UPDATE`   | Install/replace guardian quorum for a quid.               |
| `GUARDIAN_RECOVERY_*`   | Init / Veto / Commit time-locked guardian recovery.       |
| `GUARDIAN_RESIGN`       | Guardian withdraws consent from a subject's quorum.       |
| `FORK_BLOCK`            | Coordinate on-chain protocol feature activation.          |

### Event streams

Every quid or title can have an append-only, monotonically-sequenced
event stream — tamper-evident audit log. Payload inline up to 64 KB, or
referenced via IPFS for larger data.

```go
type EventTransaction struct {
    SubjectID   string  // quid or title ID
    SubjectType string  // "QUID" or "TITLE"
    Sequence    int64   // monotonic per subject
    EventType   string  // app-defined, e.g. "order.placed"
    Payload     map[string]interface{}
    PayloadCID  string  // optional IPFS CID
}
```

### Key lifecycle: rotation, recovery, revocation

The protocol treats keys as having a lifecycle. When a key is compromised
or retired:

- **Rotation (`AnchorRotation`).** Signer publishes a signed anchor moving
  from epoch `n` to `n+1`. Optionally caps old-epoch nonces so in-flight
  transactions under the old key have a bounded window.
- **Invalidation (`AnchorInvalidation`).** Freezes an epoch so no further
  transactions at that epoch are admitted.
- **Guardian recovery (`GuardianRecoveryInit`).** M-of-N guardians can
  initiate a rotation on behalf of a subject who's lost their key. A
  time-lock delay (default 1h to 1yr) gives the legitimate owner a window
  to veto. Guardians must have on-chain consented to their role.
- **Guardian resignation.** Guardians can withdraw consent without the
  subject's cooperation; prospective only — doesn't unwind in-flight
  recoveries.

---

## Quick start

### Build & run

Prerequisites: **Go 1.23+**.

```bash
git clone https://github.com/bhmortim/quidnug.git
cd quidnug
make build
./bin/quidnug
```

Or Docker:

```bash
make docker-build
make docker-run
```

Health check:

```bash
curl http://localhost:8080/api/health
# {"success":true,"data":{"status":"ok",...}}
```

### First interactions

Your first five API calls — an identity, a trust declaration, a trust
query, an event, and a key rotation:

```bash
# 1. Create an identity
curl -X POST http://localhost:8080/api/identities -d '{
  "quidId":"alice","name":"Alice","creator":"alice","updateNonce":1
}'

# 2. Declare trust
curl -X POST http://localhost:8080/api/trust -d '{
  "truster":"alice","trustee":"bob","trustLevel":0.9,
  "domain":"contractors.home","nonce":1
}'

# 3. Query trust (relational, from Alice's view)
curl "http://localhost:8080/api/trust/alice/bob?domain=contractors.home&maxDepth=5"

# 4. Record an event against Alice's stream
curl -X POST http://localhost:8080/api/events -d '{
  "subjectId":"alice","subjectType":"QUID",
  "eventType":"profile.updated","payload":{"name":"Alice Chen"}
}'

# 5. Rotate Alice's key (epoch 0 → 1)
curl -X POST http://localhost:8080/api/anchors -d '{
  "kind":"rotation","signerQuid":"alice",
  "fromEpoch":0,"toEpoch":1,
  "newPublicKey":"<hex>","anchorNonce":1,...
}'
```

Full API reference: [`docs/openapi.yaml`](docs/openapi.yaml) (OpenAPI 3.0).

---

## Architecture at a glance

```
               ┌──────────── Client SDKs (JS, Go) ─────────────┐
               │                                                │
               ▼                                                ▼
        ┌──────────────────────────────────────────────────────────┐
        │                     HTTP REST API                         │
        │  /api/v1 (core)   /api/v2 (guardian + gossip + bootstrap) │
        └──────────────────────────────────────────────────────────┘
                                │
                                ▼
        ┌──────────────────────────────────────────────────────────┐
        │                   QuidnugNode (Go)                         │
        │                                                            │
        │  ┌──────────────┐ ┌────────────┐ ┌──────────────────┐     │
        │  │Trust Engine  │ │Nonce Ledger│ │Guardian Registry │     │
        │  │(BFS pathing) │ │(QDP-0001)  │ │(QDP-0002)        │     │
        │  └──────────────┘ └────────────┘ └──────────────────┘     │
        │                                                            │
        │  ┌──────────────┐ ┌────────────┐ ┌──────────────────┐     │
        │  │Block Engine  │ │Push Gossip │ │Bootstrap / Forks │     │
        │  │(PoT tiered)  │ │(QDP-0005)  │ │(QDP-0008/9/10)   │     │
        │  └──────────────┘ └────────────┘ └──────────────────┘     │
        └──────────────────────────────────────────────────────────┘
                    ▲         ▲         ▲         ▲
                    │         │         │         │
             HTTP+sig    Gossip    Probes    Snapshot pull
                    │         │         │         │
                    ▼         ▼         ▼         ▼
        ┌──────────────────────────────────────────────────────────┐
        │             Peer QuidnugNode instances (P2P)              │
        └──────────────────────────────────────────────────────────┘
```

Detailed design: [`docs/architecture.md`](docs/architecture.md).

Protocol evolution is tracked in versioned design docs under
[`docs/design/`](docs/design/):

| QDP   | Title                                   | Status   |
|-------|-----------------------------------------|----------|
| 0001  | Global Nonce Ledger                     | Landed   |
| 0002  | Guardian-Based Recovery                 | Landed   |
| 0003  | Cross-Domain Nonce Scoping              | Landed   |
| 0004  | Phase H Roadmap                         | Landed   |
| 0005  | Push-Based Gossip (H1)                  | Landed   |
| 0006  | Guardian Resignation (H6)               | Landed   |
| 0007  | Lazy Epoch Propagation (H4)             | Landed   |
| 0008  | K-of-K Snapshot Bootstrap (H3)          | Landed   |
| 0009  | Fork-Block Migration Trigger (H5)       | Landed   |
| 0010  | Compact Merkle Proofs (H2)              | Landed   |

---

## When to use Quidnug — and when not to

**Strong fit:**

- **Your data model has "who trusts whom" as a first-class question.**
  Reputation, credentials, consortium fraud detection, cross-org approvals.
- **Your keys must be recoverable without central escrow.** Institutional
  custody, high-value signing, long-lived credentials.
- **You need replay-safe, auditable state transitions without global
  consensus.** Most internal/consortium systems don't need "everyone agrees
  on one chain" — they need "this specific party signed this specific
  action and I can verify it didn't get replayed."
- **You need coordinated protocol upgrades across a federated set of
  operators.** Fork-block transactions (QDP-0009) give you on-chain,
  operator-queryable activation boundaries.

**Weak fit:**

- **High-TPS payment rails** — Quidnug prioritizes auditability and
  correctness over raw throughput. Target: thousands of tx/sec per node
  with aggressive tuning, not millions.
- **Fully-public permissionless chains** — the proof-of-trust model
  assumes some initial trust seeding. A truly open network without prior
  trust relationships would behave as an untrusted gossip graph (still
  correct, just not useful).
- **Systems that need a single universal score** — Quidnug refuses to
  produce one by design. You can build an aggregator on top, but the
  protocol itself won't give you "what is Alice's reputation?"

---

## Installation & deployment

### Dependencies

Go 1.23+, optional IPFS node for large-payload event streams.

### Binary

```bash
make build        # → bin/quidnug
make test         # fast unit tests
make test-integration
make lint
```

### Config

Environment variables (see [`config.example.yaml`](config.example.yaml)
for YAML equivalents):

```
PORT=8080
SEED_NODES=["peer1.example:8080","peer2.example:8080"]
LOG_LEVEL=info
RATE_LIMIT_PER_MINUTE=100
NODE_AUTH_SECRET=<32-byte hex>    # for inter-node HMAC auth
REQUIRE_NODE_AUTH=true
IPFS_ENABLED=true
IPFS_GATEWAY_URL=http://localhost:5001
SUPPORTED_DOMAINS=*.finance.example,*.ai.example
ENABLE_NONCE_LEDGER=false         # QDP-0001 enforcement
ENABLE_PUSH_GOSSIP=false          # QDP-0005 (H1)
ENABLE_LAZY_EPOCH_PROBE=false     # QDP-0007 (H4)
ENABLE_KOFK_BOOTSTRAP=false       # QDP-0008 (H3)
```

### Deployment patterns

See [`docs/integration-guide.md`](docs/integration-guide.md) for:

- Single-node development
- Three-node consortium setup
- TLS termination & reverse-proxy configuration
- Inter-node HMAC authentication
- Monitoring (Prometheus scraping)

---

## Integration

### JavaScript/TypeScript client

```bash
npm install  # from clients/js/
```

```javascript
import { QuidnugClient } from 'quidnug-client';

const c = new QuidnugClient('http://localhost:8080');
const quid = await c.generateQuid();
await c.submitTransaction({ type: 'IDENTITY', quidId: quid.id, name: 'Alice' });
const trust = await c.getTrust('alice', 'bob', { domain: 'contractors.home' });
```

### Go embed

```go
import "github.com/quidnug/quidnug/internal/core"

cfg := config.LoadConfig()
node, _ := core.NewQuidnugNode(cfg)
// Use node.NonceLedger, node.TrustRegistry, etc. directly or start the HTTP server
go node.StartServer(cfg.Port)
```

### Prometheus metrics

All operational metrics exposed at `/metrics`. Grafana dashboard
definitions live under `docs/dashboards/` (TODO).

---

## Comparison with alternative systems

|                       | Quidnug                    | Bitcoin/Ethereum          | Traditional DB            | OAuth/OIDC                |
|-----------------------|----------------------------|---------------------------|---------------------------|---------------------------|
| Trust model           | Relational, per-observer   | Universal consensus       | Centralized               | Federated central         |
| Identity              | Self-sovereign (quids)     | Self-sovereign (addrs)    | Platform-owned            | Provider-owned            |
| Key recovery          | Guardian M-of-N + time-lock | Seed phrase               | Email reset               | Provider reset            |
| Throughput            | Moderate (consortium-scale)| Low (mainnet)             | Very high                 | Very high                 |
| Replay protection     | Per-signer monotonic nonce | Transaction hash uniqueness| App-level                 | JWT `jti`                 |
| Multi-party approval  | First-class (M-of-N, guardians) | Smart contract            | App-level                 | Not native                |
| Best for              | Cross-org trust, high-value | Global money / dApps      | Single-org data           | Login federation          |

---

## Contributing

See [`CONTRIBUTING.md`](CONTRIBUTING.md) and [`CODE_OF_CONDUCT.md`](CODE_OF_CONDUCT.md).

```bash
make test       # unit tests
make test-integration
make lint
```

Design docs for new features go in `docs/design/` as a numbered QDP (Quidnug
Design Proposal). Current template: look at any of 0001–0010.

## Security

Vulnerability reports: see [`SECURITY.md`](SECURITY.md). Do **not** open
public issues for security problems.

## License

Apache License, Version 2.0. See [`LICENSE`](LICENSE) and [`NOTICE`](NOTICE).
