# Quidnug Project Roadmap

The strategic view of where Quidnug is going. Updated to reflect
ground truth as of Q2 2026.

For the per-feature implementation history, see the numbered
design docs under [`design/`](design/), the unreleased block in
[`../CHANGELOG.md`](../CHANGELOG.md), and the individual
`README.md` files under `clients/`, `integrations/`, and
`UseCases/`.

---

## Where we actually are

The protocol is stable. The reference node, full multi-language
SDK surface, and first-wave integrations are all shipped. The
current frontier is in **usability and ecosystem**, not in
protocol gaps.

### Protocol (done)

All ten core design proposals have landed and are reflected in
the live code:

| QDP | Title | Status |
| --- | --- | --- |
| 0001 | Global Nonce Ledger | Landed |
| 0002 | Guardian-Based Recovery | Landed |
| 0003 | Cross-Domain Nonce Scoping | Landed |
| 0004 | Phase H Roadmap | Landed |
| 0005 | Push-Based Gossip (H1) | Landed |
| 0006 | Guardian Resignation (H6) | Landed |
| 0007 | Lazy Epoch Propagation (H4) | Landed |
| 0008 | K-of-K Snapshot Bootstrap (H3) | Landed |
| 0009 | Fork-Block Migration Trigger (H5) | Landed |
| 0010 | Compact Merkle Proofs (H2) | Landed |
| 0011 | Client Libraries & Integrations roadmap | Landed (all four tiers shipped) |
| 0012 | Domain Governance (cache replicas / consortium / governors) | Draft — design only, scheduled for Phase 1 implementation after public-network launch |
| 0013 | Network Federation Model (one protocol, many networks) | Draft — mostly clarifies existing uniformity; new surface is `external_trust_sources` config + `TRUST_IMPORT` transaction |
| 0014 | Node Discovery + Domain Sharding | **Landed** — `NODE_ADVERTISEMENT` tx + registry + expiry GC, five discovery endpoints, per-domain quid index, CLI + client SDK, signed `.well-known/quidnug-network.json` generator |
| 0015 | Content Moderation & Takedowns | Draft — `MODERATION_ACTION` tx with suppress/hide/annotate scopes, reason-code taxonomy, federation-aware propagation, GDPR-compatible erasure via cryptographic shredding |
| 0016 | Abuse Prevention & Resource Limits | Draft — five-layer rate limits (IP, quid, epoch, operator, domain), reputation-weighted graduation, progressive slowdown, proof-of-work challenges, sybil-resistance primitives |
| 0017 | Data Subject Rights & Privacy | Draft — `DATA_SUBJECT_REQUEST`, `CONSENT_GRANT/WITHDRAW`, `PROCESSING_RESTRICTION` tx types + operator DSR workflow + pseudonymity-by-default config |
| 0018 | Observability + Tamper-Evident Operator Log | Draft — per-operator hash-chained audit log, periodic on-chain anchoring, five verification endpoints, standardized metric label set |
| 0019 | Reputation Decay & Time-Weighted Trust | Draft — two-layer decay (edge-level exponential + quid dormancy), observer-configurable per-domain, passive re-endorsement detection |
| 0020 | Protocol Versioning & Deprecation | Draft — SemVer-based protocol version, capability negotiation, 18-month deprecation timeline, release workflow |

### Client SDKs (done)

Four tier-1 SDKs are feature-complete with byte-compatible
signatures across the network. Seven additional-language scaffolds
ship a keypair + sign + verify foundation.

- Full: Python 3.9+, Go 1.25+, JavaScript/TypeScript (v1 + v2
  mixin), Rust stable, Quidnug CLI (Go).
- Scaffolds: Java / Kotlin, C# / .NET 8, Swift, Android, browser
  extension, ISO 20022 mapping.

See the root [`README.md`](../README.md) "Client SDKs" table for
install paths.

### Reviews use case (done)

QRP-0001 (the Quidnug Reviews Protocol) is specified, implemented,
and has a live end-to-end demo:

- Full protocol spec and four-factor rating algorithm with
  reference implementations in Python and Go.
- Drop-in packages for every major web framework: web-components,
  React, Vue, Astro, WordPress; Shopify scaffold.
- Three SVG visualization primitives (`<qn-aurora>`,
  `<qn-constellation>`, `<qn-trace>`) used across all framework
  adapters — see [`reviews/rating-visualization.md`](reviews/rating-visualization.md).
- Schema.org JSON-LD integration so SEO-aware search engines
  still get rich-result stars.
- Working end-to-end demo against a live node with three
  divergent per-observer ratings.

### Integrations (done)

Tier-3 domain integrations are shipped:

- Sigstore / cosign (artifact signing)
- C2PA (media provenance)
- HL7 FHIR (healthcare records)
- Chainlink External Adapter (on-chain trust queries)
- Kafka bridge (event streaming)
- ISO 20022 (bank wire messaging)
- Schema.org reviews (SEO)

### Deployment / enterprise identity (done)

- Production-grade Helm chart with StatefulSet, PVCs, PDB, anti-affinity
- Docker Compose dev cluster (three-node + IPFS + Prometheus + Grafana)
- PKCS#11 / HSM signing backend (SoftHSM, YubiHSM, CloudHSM, Azure Key Vault, GCP HSM)
- WebAuthn / FIDO2 bridge (Touch ID, Windows Hello, passkeys, YubiKey)
- OIDC bridge (Okta, Auth0, Azure AD, Keycloak, Google Workspace)
- Grafana dashboard + Prometheus alert rules over the `quidnug_*` metric family
- Postman collection covering every endpoint

---

## Where we're going

The remaining scaffolds + roadmap items, grouped by what they
unlock.

### Launch-gating: implement QDPs 0015 / 0016 / 0017

Three protocol-layer pieces are required before a public
reviews network can safely carry real users and content:

- **QDP-0015 (Content Moderation)** — without a takedown
  workflow the operator has no legal response to DMCA
  notices, court orders, or CSAM reports. Phase 1
  implementation (state + validation) is ~1 person-week.
- **QDP-0016 (Abuse Prevention)** — single-layer rate
  limiting is insufficient for a public network. Phase 1
  (multi-layer buckets) is ~1.5 person-weeks.
- **QDP-0017 (Data Subject Rights)** — GDPR / CCPA / LGPD
  compliance requires honoring access / erasure / consent
  withdrawal requests within statutory windows. Phase 1
  (tx types + validation) is ~1.5 person-weeks.

Total: ~4 person-weeks of implementation for pre-launch
legal + operational readiness. Design docs are complete;
implementation is sequenced after QDP-0012 Phase 1 (domain
governance state extension).

### Near-term: close the scaffold gap

Currently scaffolded but not production-complete. Each needs a
feature-parity sweep.

- **Mobile SDKs** (Swift iOS / Kotlin Android) — full protocol
  parity + platform keystore integration. Target: a mobile
  reviewer / patient / voter app experience on par with the JS SDK.
- **Additional-language SDKs** (Java/Kotlin, C#/.NET) — feature
  parity with Python/Go/JS. Driven by enterprise FinTech + public-
  sector adoption.
- **Browser extension wallet** — MetaMask-style quid manager with
  per-site signing prompts.
- **ISO 20022 mapping** — full SWIFT MX / pacs.008 round-trip.

### Near-term: additional interface surfaces

Building on the REST API + JSON schemas:

- **gRPC gateway** — higher-throughput consortium ops
- **GraphQL gateway** — joined queries for application integration
- **WebSocket push subscriptions** — real-time event streams
- **Terraform provider** — IaC management of domains + guardian sets
- **MQTT bridge** — IoT-device event mirroring
- **PostgreSQL extension** — SQL-exposed relational trust for BI
- **Elasticsearch ingester** — full-text search over events

### Near-term: additional frontend framework adapters

Mirror the Vue/Astro pattern for:

- Svelte / SvelteKit
- SolidJS
- Angular
- Ember
- Qwik

Each adapter is ~2 days of work now that the SVG primitives exist.

### Medium-term: zero-knowledge + advanced crypto

- **Selective disclosure** — privacy-preserving trust-path proofs;
  prove "I'm trusted at ≥ L without revealing my intermediaries"
- **ZK-SNARKs for confidential transactions** — especially for
  the credit-reputation and elections use cases
- **Post-quantum signature migration** — track NIST PQC finalists,
  plan a migration path via fork-block (QDP-0009)
- **Threshold signatures** — single-shot M-of-N signing with no
  distinguishable participants (useful for guardian recovery that
  hides which guardians participated)

### Medium-term: additional vertical integrations

- **Yotpo / Trustpilot / Bazaarvoice** — enrichment layer on
  imported reviews
- **Judge.me / Product Reviews Shopify apps** — trust-weighting overlay
- **LinkedIn-style skill endorsements** — expert credibility
- **Yelp / TripAdvisor / Google Maps** — via browser extension
- **Healthgrades / Zocdoc** — practitioner trust
- **Upwork / Fiverr / TaskRabbit** — freelancer reputation
- **Reddit / Lemmy / Mastodon** — comment-quality weighting
- **Debezium CDC schema** — downstream ETL
- **Ruby / PHP SDKs** — smaller ecosystems

### Medium-term: governance

- **Domain governance transactions** — on-chain voting for domain
  parameters (thresholds, validator sets)
- **Trust-weighted DAO frameworks** — reference library for apps
  built on Quidnug

### Long-term: research

- **Mathematical modeling of trust propagation** — formal bounds
  on decay functions, Sybil resistance, path-selection strategies
- **Cognitive trust modeling** — user-study-driven refinements to
  the four-factor rating algorithm
- **Homomorphic trust computation** — ciphertext-preserving trust
  queries so a node can answer queries without decrypting edges

### Long-term: scale

- **Edge / geo-distributed deployment** — jurisdiction-scoped
  nodes with cross-domain gossip
- **High-performance validator networks** — specialized validator
  deployments for high-volume domains

---

## Success metrics

Technical:

- Transaction validation latency (target: < 10 ms on average)
- Cross-domain query performance (target: < 50 ms for depth-5)
- Trust-path calculation time (target: < 100 ms for 10k-node graphs)

Adoption:

- Number of public nodes
- Number of unique quids across all public domains
- Number of active trust domains
- Transaction volume split by type
- Reviews written under `reviews.public.*`
- Active developer community (issues, PRs, discussions, SDK downloads)

Security:

- Days since last critical advisory
- CVE response time (target: < 72h to patch, < 7d to disclose)
- Formal verification coverage of crypto primitives
- Third-party audit cadence

---

## How to propose changes

Two paths:

1. **New QDP.** For anything that changes the protocol wire
   format, consensus rules, or node behavior. Submit as a
   numbered proposal under `docs/design/`. Template: copy any
   existing QDP.
2. **Feature PR.** For everything else — SDK additions, new
   framework adapters, new integrations. Open a PR with the
   code + tests + README. The review pass confirms scope and
   naming conventions.

Things we're explicitly **not** pursuing:

- High-TPS payment-chain optimization — the protocol prioritizes
  auditability over raw throughput.
- A universal reputation score — by design, Quidnug refuses to
  produce one.
- Single-signer key-recovery via email/SMS — recovery is
  cryptographic (guardian M-of-N) by design.

See [`../CONTRIBUTING.md`](../CONTRIBUTING.md) for process details.
