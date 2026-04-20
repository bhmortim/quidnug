# QDP-0011: Client Libraries & Integrations Roadmap

| Field      | Value                                                         |
|------------|---------------------------------------------------------------|
| Status     | **Landed** — all four tiers shipped; see §9 for ship status   |
| Track      | Ecosystem                                                     |
| Author     | The Quidnug Authors                                           |
| Created    | 2026-04-19                                                    |
| Updated    | 2026-04-20 (post-implementation status)                       |
| Supersedes | —                                                             |
| Requires   | QDPs 0001–0010 (landed)                                       |
| Implements | Ecosystem roadmap (shipped)                                   |

> **Note on this document's role.** This was originally a
> pre-implementation research memo. The work it describes has
> since been executed. The memo is kept as-is for historical
> context; the §9 ship-status section below is the live view.

## 1. Summary

The protocol is stable. Ten QDPs have landed. Fourteen use-case
designs spanning FinTech, AI, government, healthcare, consumer
rights, and cross-industry domains are documented in `UseCases/`.
What's missing isn't protocol — it's **ergonomic surface area**
for the people who would actually build on top.

Today the repo ships:

- A full Go reference node (`cmd/quidnug/`).
- **One client library**: a JavaScript/TypeScript SDK
  (`clients/js/`, v1.0) covering only the `TRUST` /
  `IDENTITY` / `TITLE` transaction types. No event streams,
  no anchors, no guardians, no gossip submit, no fork-block
  — a partial coverage that dates from before Phase H.

That's it. No Python SDK, no Go client package, no CLI, no
dashboards, no HSM bindings, no identity-bridge integrations,
no IaC. Every use case in `UseCases/` currently requires the
consumer to write the integration code themselves.

This document is a research memo, not a design. It:

1. Audits the current state and identifies gaps.
2. Catalogs candidate client libraries and integrations.
3. Applies explicit evaluation criteria.
4. Produces a four-tier prioritized recommendation.
5. Maps recommendations to the fourteen use cases so the
   value of each is concrete.
6. Proposes a sequencing plan.

No code is written as part of this QDP. Each recommended item
would get its own dedicated design + implementation pass
afterward.

## 2. Current state

### 2.1 What's in the repo today

```
clients/
  js/                                  ← only first-party client
    quidnug-client.js                  (1,523 LOC)
    quidnug-client.d.ts                (  709 LOC — TypeScript types)
    quidnug-client.test.js
    quidnug-client.retry.test.js
    package.json                       (@quidnug/client v1.0.0)

docs/
  openapi.yaml                         (47 handlers, 27 paths)
  architecture.md
  integration-guide.md                 (JS-client tutorial only)

cmd/
  quidnug/                             (Go reference node binary)

internal/core/                         (Go implementation)
```

No CLI binary beyond running the node. No Python, Go-package,
Rust, Java, C#, Swift, Kotlin, PHP, or Ruby client. No
dashboards, no Kubernetes manifests, no Terraform provider,
no Helm chart, no HSM bindings.

### 2.2 JS client coverage gap

The v1 JS SDK predates Phase H. Measured against current
protocol:

| Protocol feature              | QDP(s)        | JS client?         |
|-------------------------------|---------------|--------------------|
| TRUST / IDENTITY / TITLE tx  | pre-QDPs      | ✅ Full support    |
| EVENT transactions            | —             | ❌ Missing         |
| Event stream subscription     | —             | ❌ Missing         |
| ANCHOR (rotation, cap, inval) | 0001          | ❌ Missing         |
| Guardian set update           | 0002          | ❌ Missing         |
| Guardian recovery init/veto/commit | 0002     | ❌ Missing         |
| Guardian resignation          | 0006          | ❌ Missing         |
| Cross-domain fingerprint      | 0003          | ❌ Missing         |
| Push gossip submit            | 0005          | ❌ Missing         |
| K-of-K snapshot bootstrap     | 0008          | ❌ Missing         |
| Fork-block transaction        | 0009          | ❌ Missing         |
| Compact Merkle proof verify   | 0010          | ❌ Missing         |

Only 3 of ~12 protocol surfaces are covered. Every
production use case in `UseCases/` requires at least one of
the missing surfaces. For example:

- **Interbank wire authorization** needs guardian set
  management + anchor rotation.
- **Institutional custody** needs the full anchor lifecycle.
- **Elections** needs events (ballot check-in), blind-sig
  client helpers, and the `POST /ballot-issuance` flow.
- **Decentralized credit** needs events + ECIES access grants.

### 2.3 Integration gap

Missing integrations — not language SDKs, but bridges to
other platforms/systems/protocols:

| Category                | Missing integrations                                        |
|-------------------------|-------------------------------------------------------------|
| **Observability**       | Grafana dashboards, OpenTelemetry, Datadog, New Relic       |
| **Deployment**          | Helm chart, Kubernetes operator, Terraform, Ansible          |
| **Key management**      | PKCS#11, AWS KMS, Azure Key Vault, GCP KMS, Ledger/Trezor   |
| **Identity bridging**   | OIDC, SAML, WebAuthn, SPIFFE                                |
| **Enterprise data**     | Kafka, PostgreSQL materialized views, CDC, Elasticsearch    |
| **Vertical**            | Sigstore, C2PA, HL7 FHIR, Chainlink, OpenCredential         |
| **Interface variants**  | gRPC, GraphQL, WebSocket subscriptions, MQTT                |
| **Developer tooling**   | CLI binary, VS Code extension, Postman collection           |
| **Storage backends**    | IPFS (partial), S3, Azure Blob, Filecoin, Arweave           |

Every one of the 14 use cases would benefit from at least 3
of these. Today each use case has to build its own.

## 3. Gap analysis by use case

For each use case, the critical blockers that would be lifted
by a specific library or integration:

| Use case                           | Critical missing pieces                                                                                  |
|------------------------------------|----------------------------------------------------------------------------------------------------------|
| Interbank wire authorization       | Guardian API in JS/Python/Go client; PKCS#11 bindings for HSMs; OpenBanking/ISO 20022 adapter.          |
| Merchant fraud consortium          | Event-stream subscription in JS+Python; Kafka connector for enterprise data platforms.                  |
| DeFi oracle network                | Chainlink external adapter; lightweight Rust client for performance-critical reporters; WebSocket push. |
| Institutional custody              | Full anchor API in Python/JS; HSM bindings (PKCS#11, CloudHSM); FIPS-compliant packaging.              |
| B2B invoice financing              | EDI-bridge for current invoice formats; SAP/Oracle-ERP connector; Python (supply-chain analytics).      |
| AI model provenance                | Python SDK (PyTorch/TensorFlow integration); signed-artifact hooks for HuggingFace, MLflow.             |
| AI agent authorization             | Python SDK; OpenAI-SDK-compatible wrapper that enforces authorization before any tool call.             |
| Federated learning attestation     | Python SDK (TF-Federated, PySyft, Flower integration); WebSocket for coordinator events.                |
| AI content authenticity            | C2PA plugin; Python + Swift/Kotlin mobile SDKs; IoT camera firmware bindings (C/C++).                   |
| Elections                          | Mobile SDKs (Swift/Kotlin for voter app); browser extension (voter wallet); blind-sig client helpers; CLI for pollworker tablets; paper-ballot QR rendering library. |
| Decentralized credit               | Python + JS SDK; ECIES helpers; mobile SDKs (consumer wallet); CSV/JSON import from current bureaus.    |
| Healthcare consent management      | HL7 FHIR bridge; Epic/Cerner adapter modules; mobile SDKs (patient app); WebAuthn.                     |
| Credential verification            | OIDC bridge so credential verify becomes drop-in for existing SSO; Java/C# SDKs for LMS/HR systems.    |
| Developer artifact signing         | Sigstore/cosign integration; GitHub Action; GitLab CI template; Docker-registry content-trust bridge.   |

Fourteen use cases, ~30 distinct missing pieces. The
distribution is heavily skewed: **Python, CLI, Grafana, and
HSM bindings** each unlock 5+ use cases by themselves.

## 4. Candidate inventory

All concrete candidates the research surfaced. Grouped by
category for orientation; priority comes later.

### 4.1 Language SDKs

| Candidate             | Rationale                                                          | Ecosystems it unlocks                              |
|-----------------------|--------------------------------------------------------------------|----------------------------------------------------|
| **Python**            | AI / data science / government / scripting                         | PyTorch, TensorFlow, HuggingFace, MLflow, Django   |
| **Go client package** | Ref impl is Go; no embeddable module for consumers                 | Kubernetes ecosystem, CNCF tools, Go microservices |
| **Rust**              | Security-first, high perf; growing blockchain/security adoption    | Substrate, Solana, security tools                  |
| **Java / Kotlin**     | Enterprise, especially banks                                       | Spring, Android, enterprise JVM                    |
| **C# / .NET**         | US enterprise, healthcare, government                              | ASP.NET, Unity, Azure Functions                    |
| **Swift**             | iOS consumer wallet apps                                           | iOS voter, patient, consumer apps                  |
| **Kotlin (Android)**  | Android consumer wallet apps                                       | Android voter, patient, consumer apps              |
| **C / C++**           | IoT, embedded, camera firmware, trading engines                    | Content-authenticity cameras, HFT                  |
| **PHP**               | E-commerce back-ends                                               | WooCommerce, Magento, legacy merchants             |
| **Ruby**              | Rails admin dashboards                                             | Startup back-office                                 |

### 4.2 Developer tooling

| Candidate                             | Rationale                                              |
|---------------------------------------|--------------------------------------------------------|
| **`quidnug` CLI binary**              | Ops + developer onboarding — lowers friction hugely    |
| **VS Code extension**                 | Browse chain, compose transactions in-editor           |
| **Postman / Bruno collection**        | Interactive API testing                                |
| **OpenAPI-generated clients**         | Auto-produce typed clients in 10+ languages            |
| **Protobuf / gRPC definitions**       | Foundation for cross-language RPC                      |

### 4.3 Deployment & infra

| Candidate                 | Rationale                                                     |
|---------------------------|---------------------------------------------------------------|
| **Helm chart**            | One-command consortium deploy on Kubernetes                   |
| **Kubernetes operator**   | Reconcile guardian sets, key rotations, fork-blocks as CRDs   |
| **Terraform provider**    | Infra-as-code for node + domain + guardian provisioning       |
| **Ansible role**          | On-prem banking / government deploys                          |
| **Systemd unit files**    | Bare-metal / VM deploys                                       |
| **Docker Compose**        | 3-node consortium quickstart for dev                          |

### 4.4 Observability

| Candidate                       | Rationale                                                     |
|---------------------------------|---------------------------------------------------------------|
| **Grafana dashboards**          | Prometheus metrics already emitted — ship ready-made panels   |
| **Prometheus alerting rules**   | Starter alerts for replay spikes, recovery storms, etc.       |
| **OpenTelemetry instrumentation** | Traces + metrics in modern observability stacks             |
| **Datadog / New Relic plugins** | Hosted observability drop-ins                                 |
| **Loki log schema**             | Structured log parsing for Grafana stacks                     |

### 4.5 Key management

| Candidate                           | Rationale                                                   |
|-------------------------------------|-------------------------------------------------------------|
| **PKCS#11 bindings**                | Thales, Utimaco, SafeNet — enterprise HSM standard          |
| **AWS CloudHSM / KMS integration**  | AWS tenants (largest cloud)                                 |
| **Azure Key Vault integration**     | Enterprise + government                                     |
| **GCP Cloud KMS integration**       | GCP tenants                                                 |
| **Ledger / Trezor (hardware wallet)** | Consumer + institutional DIY                              |
| **TPM 2.0 integration**             | Device identity, voting booths, IoT                         |
| **FIDO2 / WebAuthn bridge**         | Hardware keys as quid signing keys — UX win for end users   |

### 4.6 Identity & auth bridging

| Candidate                     | Rationale                                                      |
|-------------------------------|----------------------------------------------------------------|
| **OIDC / OAuth2 IdP**         | Quidnug as identity provider for existing OIDC clients         |
| **SAML bridge**               | Enterprise SSO (Okta, Azure AD, etc.)                          |
| **SPIFFE / SPIRE integration**| Workload identity in k8s                                       |
| **Sigstore / Fulcio bridge**  | Artifact-signing use case; OIDC-backed attestation             |

### 4.7 Data plane & messaging

| Candidate                           | Rationale                                                         |
|-------------------------------------|-------------------------------------------------------------------|
| **Kafka connector**                 | Stream events into enterprise event buses                          |
| **PostgreSQL materialized-view module** | SQL-friendly access to trust graph for BI                    |
| **Debezium CDC schema**             | Change-data-capture for downstream ETL                            |
| **Elasticsearch index template**    | Full-text search over event streams                                |
| **S3 / Azure Blob / Filecoin / Arweave** | Snapshot + encrypted-blob backends                          |

### 4.8 Interface surfaces beyond REST

| Candidate                   | Rationale                                                  |
|-----------------------------|------------------------------------------------------------|
| **gRPC API**                | Higher throughput + streaming, especially for consortium   |
| **GraphQL gateway**         | Client-friendly joined queries                             |
| **WebSocket subscription**  | Live event streams; today consumers must poll              |
| **MQTT bridge**             | IoT scenarios (content-authenticity cameras, sensors)      |

### 4.9 Vertical integrations

| Candidate                     | Use case unlocked                                          |
|-------------------------------|------------------------------------------------------------|
| **C2PA plugin**               | AI content authenticity                                    |
| **HL7 FHIR bridge**           | Healthcare consent                                         |
| **Chainlink external adapter**| DeFi oracle                                                |
| **Sigstore / cosign**         | Developer artifact signing                                 |
| **OpenBanking / ISO 20022**   | Interbank wire authorization                               |
| **SWIFT gpi bridge**          | International wire                                         |
| **OpenCredential / VC spec**  | Credential verification                                    |
| **GDPR / CCPA compliance module** | Any consumer-data use case                             |
| **SPDX / SBOM attestation**   | Developer artifact signing (SBOM on-chain)                 |

### 4.10 End-user surfaces

| Candidate                         | Rationale                                                  |
|-----------------------------------|------------------------------------------------------------|
| **Browser extension (quid wallet)** | Voter, patient, consumer wallets in the browser          |
| **iOS mobile SDK**                | Voter app, patient app, consumer wallet                    |
| **Android mobile SDK**            | Same, Android-side                                         |
| **React component library**       | Drop-in UI components for trust queries, quid lookup       |
| **Vue / Svelte / Angular ports**  | Same for other frameworks                                  |

## 5. Evaluation criteria

Every candidate was scored — informally — against these five
criteria:

1. **Use-cases unlocked.** How many of the 14 documented use
   cases does this candidate materially unblock? Integer
   score 1–14.
2. **Breadth of leverage.** Is this specific to one
   vertical, or does it apply across many? Score 1 (niche)
   to 5 (broad).
3. **Adoption accelerator.** Does it lower the time-to-first-
   integration for a new adopter? Score 1 (marginal) to 5
   (radical reduction).
4. **Build cost.** Rough effort estimate — person-weeks
   ballpark, for a protocol-literate implementer. Lower is
   better.
5. **Maintenance burden.** Ongoing cost of keeping in sync
   with protocol evolution. Score 1 (low) to 5 (high).

Composite priority score =
`unlocked × breadth × adoption_accel / (build_cost + 0.5 × maintenance)`.

This isn't a rigorous model — it's a framework for structured
judgment. The tier assignments below reflect it.

## 6. Tiered recommendations

### 6.1 Tier 1 — Ship first (essential)

These should be implemented before any deep vertical push
because every use case needs at least one of them.

#### 6.1.1 Python client SDK

- **Unlocks:** 4 AI use cases (model provenance, agent auth,
  federated learning, content authenticity), 2 FinTech
  analytics needs, elections (data-science recount tooling),
  credit (consumer-tools scripting), healthcare (institution
  batch scripts), government (automation) = **10+ use cases**.
- **Breadth:** Very broad. Python is the dominant language
  in data science, ML, scientific computing, security
  automation, government, and DevOps scripting.
- **Adoption accelerator:** Huge. Many prospective adopters
  will never write Go or even JS — they live in Python.
- **Build cost:** Moderate. Can mirror the JS client's
  feature surface plus fill the Phase-H gaps. ~4–6 person-
  weeks for v1.0 with full coverage.
- **Maintenance:** Low once shape is defined. Auto-gen from
  OpenAPI reduces future drift.
- **Format:** `pip install quidnug` → single package. Type
  stubs (`py.typed`) for Pyright / Pylance / Mypy.

#### 6.1.2 Go client package

- **Unlocks:** Internal tooling for operators, extensions to
  the reference node, any Go microservice wanting to
  embed — all 14 use cases at the ops layer benefit.
- **Breadth:** Mainly the CNCF / Kubernetes ecosystem plus
  Go-first startups, but those ecosystems are large.
- **Adoption accelerator:** Significant. Today someone who
  wants to embed Quidnug logic in a Go service has to
  depend on `internal/core` (officially private) or run an
  HTTP hop to a local node.
- **Build cost:** Low. The primitives exist; the work is
  extracting a public `pkg/quidnug` API surface from
  `internal/core`, with stability commitments.
- **Maintenance:** Moderate. Public API requires semver
  discipline.
- **Format:** New `pkg/client/` directory; module path
  `github.com/quidnug/quidnug/pkg/client`. Zero-dependency
  beyond stdlib and the Go crypto libs already vendored.

#### 6.1.3 JS client v2 — close the Phase-H gap

- **Unlocks:** Everything the existing client was meant to
  but doesn't yet — and it's the library most adopters will
  first touch for web-based use cases.
- **Breadth:** Browser + Node.js — enormous.
- **Adoption accelerator:** Radical. Every tutorial that
  currently says "and then write this part yourself" would
  be obsolete.
- **Build cost:** Moderate. 3–5 person-weeks to add events,
  anchors, guardians, gossip submit, bootstrap status,
  Merkle proof verify, and blind-sig helpers.
- **Maintenance:** Moderate. Breadth requires ongoing care.
- **Format:** Major version bump to 2.0.0; keep v1
  deprecated-but-working for 1 year.

#### 6.1.4 Quidnug CLI binary

- **Unlocks:** All 14 use cases benefit; ops teams
  specifically get a 10x onboarding win.
- **Breadth:** Ops, DevOps, support engineers, auditors,
  investigative journalists running tallies, etc.
- **Adoption accelerator:** Radical. "Give me the quid for
  alice@example.com" is `quidnug id alice@example.com`
  instead of a curl-jq pipeline.
- **Build cost:** Low. Wrap the Go client package behind
  cobra or urfave/cli. ~2 person-weeks for a solid v1.
- **Maintenance:** Low. Tracks the Go client.
- **Proposed subcommands:**
  ```
  quidnug identity create|show|import|export
  quidnug trust grant|query|revoke|edges
  quidnug event emit|tail|verify
  quidnug anchor rotate|invalidate|cap
  quidnug guardian set|recover|veto|commit|resign
  quidnug gossip push|status
  quidnug bootstrap|snapshot|status
  quidnug fork-block submit|list|status
  quidnug tally <domain>
  quidnug audit <subject>
  quidnug verify <proof-or-event-id>
  quidnug node run|health|peers|metrics
  ```
- **Format:** Single static binary. Man page + shell
  completions.

#### 6.1.5 Grafana dashboards + Prometheus alerting

- **Unlocks:** Every production deployment — operators
  currently start from zero visibility despite Prometheus
  metrics existing at `/metrics`.
- **Breadth:** Universal ops concern.
- **Adoption accelerator:** High. Visual proof-of-life is
  the difference between "evaluated and shelved" and
  "demoed and approved."
- **Build cost:** Low. ~1 person-week for the dashboards +
  ~1 person-week for alert rules with tuning guidance.
- **Maintenance:** Low to moderate — metrics churn
  occasionally.
- **Format:** JSON dashboard definitions and YAML alert
  rules in `integrations/grafana/` and `integrations/prometheus/`.

**Tier 1 total estimated effort: 11–16 person-weeks.**

### 6.2 Tier 2 — Production-readiness

These enable serious deployments but aren't as universally
prerequisite as Tier 1.

#### 6.2.1 PKCS#11 / HSM bindings

- **Unlocks:** Interbank wire (1), institutional custody (4),
  elections (10), credentials (13), artifact signing (14) —
  any use case where real money or real liability is at
  stake. **5 high-stakes use cases.**
- **Breadth:** Regulated industries specifically.
- **Adoption accelerator:** Radical for those industries —
  going to production in banking without an HSM story is
  usually a non-starter.
- **Build cost:** Moderate-high. PKCS#11 is fiddly; 4–6
  person-weeks for a clean abstraction + real-HSM tests.
- **Maintenance:** Moderate. Vendor HSM updates occur.
- **Format:** Go package `pkg/signer/pkcs11/` implementing a
  `Signer` interface; clients use it transparently.

#### 6.2.2 Kubernetes operator + Helm chart

- **Unlocks:** Any production consortium deployment.
- **Breadth:** High. Kubernetes is the modern default for
  consortium infra.
- **Adoption accelerator:** Large. "Run a 5-node consortium"
  goes from 2-day exercise to `helm install`.
- **Build cost:** Moderate. Helm chart alone: ~1 week.
  Operator (reconciling GuardianSet CRDs, handling fork-block
  activations, managing secrets): ~4–6 weeks.
- **Maintenance:** Moderate. Kubernetes churns.
- **Format:** `integrations/kubernetes/helm/` and
  `integrations/kubernetes/operator/`.

#### 6.2.3 WebAuthn / FIDO2 integration

- **Unlocks:** Elections (10), decentralized credit (11),
  healthcare consent (12), credentials (13). **4 consumer-
  facing use cases.**
- **Breadth:** Any end-user-facing scenario where users
  already have or will have hardware keys.
- **Adoption accelerator:** High — dramatically improves
  consumer UX vs. "manage your own private key file."
- **Build cost:** Moderate. ~3 person-weeks. Security review
  required.
- **Maintenance:** Low. Standard is stable.
- **Format:** Browser-side library bundled with JS client,
  plus server-side verification helpers.

#### 6.2.4 Rust client

- **Unlocks:** Performance-critical reporters (oracle
  networks), security-research tooling, IoT firmware (content
  authenticity). **3–4 use cases.**
- **Breadth:** Growing. Rust is becoming the default for
  new blockchain / security tooling.
- **Adoption accelerator:** Moderate — Rust adopters are
  capable but the community is vocal and influential.
- **Build cost:** Moderate. 4–5 person-weeks.
- **Maintenance:** Low once stable. Cargo publish ritual.
- **Format:** `crates/quidnug-client/`, `cargo install quidnug-cli`.

#### 6.2.5 OIDC identity-provider bridge

- **Unlocks:** Credentials (13), consumer-facing apps for
  elections (10), credit (11), healthcare (12). Lets
  existing SSO flows authenticate against a Quidnug quid.
- **Breadth:** Enterprise identity is huge.
- **Adoption accelerator:** High — "log in with your Quidnug
  identity" drops into any OIDC-speaking app.
- **Build cost:** Moderate. 3–4 person-weeks.
- **Maintenance:** Low. OIDC is stable.
- **Format:** `integrations/oidc-provider/` — Go service
  that speaks OIDC and delegates to a Quidnug node for
  verification.

**Tier 2 total estimated effort: 18–26 person-weeks.**

### 6.3 Tier 3 — Vertical integrations

These unlock specific use cases without broad impact
elsewhere — valuable but not before the foundational stack
above.

#### 6.3.1 Sigstore / cosign integration

- Use case: Developer artifact signing (14).
- Effort: ~3 person-weeks.
- Format: Cosign plugin + GitHub Action template.

#### 6.3.2 C2PA plugin

- Use case: AI content authenticity (9).
- Effort: ~3 person-weeks.
- Format: C2PA "X.509 + Ed25519" extension adapter + a
  Rust-based camera-firmware shim.

#### 6.3.3 HL7 FHIR bridge

- Use case: Healthcare consent management (12).
- Effort: ~4 person-weeks.
- Format: Go service implementing SMART-on-FHIR that
  maps patient consent requests to Quidnug trust edges.

#### 6.3.4 Chainlink external adapter

- Use case: DeFi oracle network (3).
- Effort: ~2 person-weeks.
- Format: Standard Chainlink external-adapter package,
  queries Quidnug for signed price events.

#### 6.3.5 Java / Kotlin SDK

- Unlocks: enterprise FinTech use cases (1, 4, 5), Android
  consumer apps (parts of 10, 11, 12).
- Effort: ~6 person-weeks for full feature parity.
- Format: Maven Central publish, Kotlin multiplatform.

#### 6.3.6 C# / .NET SDK

- Unlocks: US enterprise, government, healthcare.
- Effort: ~5 person-weeks.
- Format: NuGet package targeting .NET 8.

#### 6.3.7 ISO 20022 / OpenBanking adapter

- Use case: Interbank wire authorization (1).
- Effort: ~6 person-weeks (ISO 20022 message surface is
  large).
- Format: Go service that converts SWIFT MX /
  ISO 20022 pacs.008 messages to Quidnug wire-approval
  titles and back.

**Tier 3 total estimated effort: 29–35 person-weeks.**

### 6.4 Tier 4 — Future

Valuable but further out. Listed for completeness and to
prevent re-inventing them prematurely.

- **Swift iOS SDK** — mobile voter / patient wallets.
- **Kotlin Android SDK** — same for Android.
- **Browser extension (wallet)** — MetaMask-style for
  quid management.
- **gRPC API surface** — higher-throughput consortium ops.
- **GraphQL gateway** — for applications that want joined
  queries.
- **WebSocket event-subscription server** — live updates
  instead of polling.
- **Ledger / Trezor hardware-wallet integration** —
  consumer + institutional DIY custody.
- **Terraform provider** — IaC.
- **MQTT bridge** — IoT scenarios.
- **Elasticsearch index template** — full-text search.
- **PostgreSQL materialized-view extension** — BI-friendly
  trust-graph access.
- **Debezium-style CDC schema** — downstream ETL.
- **React / Vue / Svelte component library** — drop-in UI.
- **PHP / Ruby SDKs** — smaller ecosystems.

## 7. Use case × library/integration matrix

Shaded cells = critical unlock (without this, the use case
requires the adopter to write substantial boilerplate).

|                                  | Py | Go | JSv2 | CLI | Graf | HSM | K8s | WebAuthn | Rust | OIDC | Sigstore | C2PA | FHIR | Chainlink |
|----------------------------------|----|----|------|-----|------|-----|-----|----------|------|------|----------|------|------|-----------|
| 1. Interbank wire                 | ●  | ●  | ●    | ●   | ●    | ●   | ●   |          |      |      |          |      |      |           |
| 2. Merchant fraud                 | ●  |    | ●    | ●   | ●    |     | ●   |          |      |      |          |      |      |           |
| 3. DeFi oracle                    | ●  | ●  | ●    | ●   | ●    |     | ●   |          | ●    |      |          |      |      | ●         |
| 4. Institutional custody          | ●  | ●  | ●    | ●   | ●    | ●   | ●   |          |      |      |          |      |      |           |
| 5. B2B invoice financing          | ●  |    | ●    | ●   | ●    |     | ●   |          |      | ●    |          |      |      |           |
| 6. AI model provenance            | ●  |    |      | ●   | ●    |     |     |          |      |      | ●        |      |      |           |
| 7. AI agent authorization         | ●  | ●  |      | ●   | ●    |     |     |          |      |      |          |      |      |           |
| 8. Federated learning             | ●  |    |      | ●   | ●    |     | ●   |          |      |      |          |      |      |           |
| 9. AI content authenticity        | ●  |    | ●    | ●   | ●    |     |     |          | ●    |      |          | ●    |      |           |
| 10. Elections                     | ●  | ●  | ●    | ●   | ●    | ●   | ●   | ●        |      | ●    |          |      |      |           |
| 11. Decentralized credit          | ●  |    | ●    | ●   | ●    |     |     | ●        |      | ●    |          |      |      |           |
| 12. Healthcare consent            | ●  |    | ●    | ●   | ●    |     |     | ●        |      | ●    |          |      | ●    |           |
| 13. Credential verification       | ●  |    | ●    | ●   | ●    | ●   |     | ●        |      | ●    |          |      |      |           |
| 14. Developer artifact signing    | ●  | ●  | ●    | ●   | ●    | ●   |     |          |      |      | ●        |      |      |           |
| **Use cases unlocked**            | **14** | **6** | **10** | **14** | **14** | **5** | **7** | **4** | **2** | **5** | **2** | **1** | **1** | **1** |

Reading the bottom row: the top-5 highest-leverage
additions are Python SDK, CLI, Grafana dashboards, JS v2,
and Kubernetes operator/Helm — matching Tier 1 + Tier 2.
The vertical integrations (Tier 3) unlock 1–2 use cases
each and should wait for the foundation.

## 8. Sequencing recommendation

### 8.1 Quarter 1 — Foundation (Tier 1)

- Python SDK v1.0 with full protocol coverage
- Go client package at `pkg/client/`
- JS client v2.0 (closes the Phase-H gap)
- `quidnug` CLI binary
- Grafana starter dashboards + Prometheus alerts

Outcome at end of Q1: any developer in any of the major
ecosystems can build a proof-of-concept in an afternoon
instead of a week. Every documented use case has a working
path without reinventing client code.

### 8.2 Quarter 2 — Production-readiness (Tier 2)

- PKCS#11 / HSM bindings
- Kubernetes operator + Helm chart
- WebAuthn / FIDO2 integration
- Rust client
- OIDC identity-provider bridge

Outcome: a real consortium deployment can be stood up in a
week, with enterprise-grade key management and identity
federation.

### 8.3 Quarter 3 — Vertical depth (Tier 3 selections)

Pick 3 of 7 Tier-3 items based on market demand at that
point. Strong default picks:

- Sigstore integration (open-source community)
- HL7 FHIR bridge (healthcare)
- Chainlink external adapter (DeFi oracle — relatively
  small effort, high-visibility use case)

### 8.4 Quarter 4+ — Expansion (Tier 4 selections)

Mobile SDKs, browser extension, gRPC, specialty bridges as
demand materializes.

## 9. Repository structure proposal

To accommodate the growing ecosystem:

```
clients/
  js/                            ← existing, v2 upgrade
  python/                        ← new, pip install quidnug
  rust/                          ← new, cargo install
  java-kotlin/                   ← Tier 3
  dotnet/                        ← Tier 3
  swift/                         ← Tier 4
  android-kotlin/                ← Tier 4

cmd/
  quidnug/                       ← existing node binary
  quidnug-cli/                   ← new, Tier 1 CLI

pkg/
  client/                        ← new Go client package

integrations/
  grafana/                       ← dashboards
  prometheus/                    ← alert rules
  kubernetes/
    helm/
    operator/
  terraform/                     ← Tier 4
  pkcs11/                        ← Tier 2
  aws-kms/
  azure-key-vault/
  gcp-kms/
  oidc-provider/                 ← Tier 2
  saml-bridge/
  webauthn/                      ← Tier 2
  sigstore/                      ← Tier 3
  c2pa/                          ← Tier 3
  hl7-fhir/                      ← Tier 3
  chainlink-adapter/             ← Tier 3
  iso20022/
  opencredential/

docs/
  design/                        ← existing QDPs
  client-guides/                 ← new, per-language guides
  integration-guides/            ← new, per-integration guides
  dashboards/                    ← screenshot docs
```

No single-language monorepo per client (each lives in its
own subtree) so language-specific tooling (setup.py, Cargo,
Maven, etc.) can coexist cleanly.

## 10. Open questions

### 10.1 Auto-generate vs. hand-write clients

OpenAPI generators exist for 30+ languages. Pros: cheap,
breadth, stay-in-sync. Cons: generated code is generally
ugly, idiomatic wrappers are needed anyway for ergonomic
APIs, and the protocol has features (blind signatures,
ECIES access grants, trust-graph BFS, Merkle proof verify)
that aren't naturally expressed in OpenAPI.

**Tentative answer:** hybrid. Auto-generate the raw transport
layer; hand-write the ergonomic sugar. That's the approach
the current JS client uses implicitly.

### 10.2 Monorepo vs. split repos per language

The current repo is already Go-heavy. Adding Python + Rust
+ JS + potentially Java inflates size and CI time.

**Tentative answer:** Keep Tier 1 clients in the monorepo
for discoverability. Tier 3/4 language SDKs can move to
separate repos under a GitHub org once the ecosystem grows.
Critical: the reference node and the primary clients move
in lockstep for v2.x.

### 10.3 Versioning strategy

If the protocol ships QDP-0011 and higher, do clients
follow protocol versions or independent semver?

**Tentative answer:** Independent semver for each client.
Each release states "compatible with Quidnug node v2.3+"
in its README. Protocol fork-blocks (QDP-0009) are the
hard-compatibility boundary.

### 10.4 Commercial / open-source split

Some integrations (e.g., enterprise HSM bindings,
SWIFT gpi) might warrant commercial licensing separately
from the Apache-2.0 core.

**Tentative answer:** Everything stays Apache-2.0 for now.
Revisit if / when a commercial model emerges. Apache-2.0 is
compatible with commercial extensions.

### 10.5 Dedicated design QDP per Tier-1 item?

Should each Tier-1 item get its own full design QDP before
implementation (matching how protocol features are
designed)?

**Tentative answer:** Yes for anything that changes
protocol surface. No for pure client libraries that wrap
existing endpoints. CLI should get a QDP because it needs
a subcommand taxonomy. Grafana dashboards don't need a
QDP — just ship them.

## 9. Ship status (post-implementation)

All four tiers are now in-tree. This section is the live
status summary; the research-memo sections above are preserved
unchanged for historical context.

### Tier 1 — essential (shipped)

| Item | Status | Landed as |
| --- | --- | --- |
| Python client SDK | ✅ full | [`clients/python/`](../../clients/python/) |
| Go client package | ✅ full | [`pkg/client/`](../../pkg/client/) |
| JS client v2 (guardian/gossip/merkle mixin) | ✅ full | [`clients/js/quidnug-client-v2.js`](../../clients/js/) |
| Quidnug CLI binary | ✅ full | [`cmd/quidnug-cli/`](../../cmd/quidnug-cli/) |
| Grafana dashboards + Prometheus alerting | ✅ full | [`deploy/observability/`](../../deploy/observability/) |

### Tier 2 — production-readiness (shipped)

| Item | Status | Landed as |
| --- | --- | --- |
| PKCS#11 / HSM bindings | ✅ full | [`pkg/signer/hsm/`](../../pkg/signer/hsm/) |
| Kubernetes Helm chart | ✅ full | [`deploy/helm/quidnug/`](../../deploy/helm/quidnug/) |
| WebAuthn / FIDO2 integration | ✅ full | [`pkg/signer/webauthn/`](../../pkg/signer/webauthn/) |
| Rust client | ✅ full | [`clients/rust/`](../../clients/rust/) |
| OIDC identity-provider bridge | ✅ full | [`cmd/quidnug-oidc/`](../../cmd/quidnug-oidc/) |

### Tier 3 — vertical integrations (shipped)

| Item | Status | Landed as |
| --- | --- | --- |
| Sigstore / cosign | ✅ full | [`integrations/sigstore/`](../../integrations/sigstore/) |
| C2PA plugin | ✅ full | [`integrations/c2pa/`](../../integrations/c2pa/) |
| HL7 FHIR bridge | ✅ full | [`integrations/fhir/`](../../integrations/fhir/) |
| Chainlink external adapter | ✅ full | [`integrations/chainlink/`](../../integrations/chainlink/) |
| Kafka bridge | ✅ full | [`integrations/kafka/`](../../integrations/kafka/) |
| ISO 20022 mapping | ✅ full | [`integrations/iso20022/`](../../integrations/iso20022/) |
| Java / Kotlin SDK scaffold | ✅ scaffold | [`clients/java/`](../../clients/java/) |
| C# / .NET SDK scaffold | ✅ scaffold | [`clients/dotnet/`](../../clients/dotnet/) |

### Tier 4 — platform + framework (mixed)

| Item | Status | Landed as |
| --- | --- | --- |
| Swift iOS/macOS SDK | scaffold | [`clients/swift/`](../../clients/swift/) |
| Kotlin Android SDK | scaffold | [`clients/android/`](../../clients/android/) |
| Browser extension | scaffold | [`clients/browser-extension/`](../../clients/browser-extension/) |
| React component library | ✅ full | [`clients/react-reviews/`](../../clients/react-reviews/) |
| Vue component library | ✅ full | [`clients/vue-reviews/`](../../clients/vue-reviews/) |
| Astro SSR components | ✅ full | [`clients/astro-reviews/`](../../clients/astro-reviews/) |
| Web Components | ✅ full | [`clients/web-components/`](../../clients/web-components/) |
| Reviews widget (1-line embed) | ✅ full | [`clients/reviews-widget/`](../../clients/reviews-widget/) |
| WordPress plugin | ✅ full | [`clients/wordpress-plugin/`](../../clients/wordpress-plugin/) |
| Shopify app | scaffold | [`clients/shopify-app/`](../../clients/shopify-app/) |
| Schema.org reviews integration | ✅ full | [`integrations/schema-org/`](../../integrations/schema-org/) |
| gRPC / GraphQL / WebSocket / Terraform / Ledger / MQTT / Postgres / Elastic | scaffold | [`integrations/`](../../integrations/) |
| Svelte / SolidJS / Angular / Ember / Qwik adapters | not yet started | — |

### New work beyond the original 0011 scope

Executed in parallel with the 0011 rollout:

- **QRP-0001 Reviews Protocol** — a domain-level protocol spec
  (separate from the infrastructure QDPs) defining event types,
  topic tree, and trust-weighted rating algorithm. See
  [`../../examples/reviews-and-comments/`](../../examples/reviews-and-comments/).
- **Rating visualization primitives** — `<qn-aurora>`,
  `<qn-constellation>`, `<qn-trace>` — three zero-dependency
  SVG custom elements used across every framework adapter above.
  See [`../reviews/rating-visualization.md`](../reviews/rating-visualization.md).

## 10. Review status

Landed. Historical notes:

- Initial draft as a research memo and prioritization exercise.
- Tier 1 / 2 / 3 implemented in sequence without scope changes.
- Tier 4 partially shipped (all framework adapters done for the
  reviews use case) with the remaining platform scaffolds
  tracked in the main roadmap (`docs/roadmap.md`).
- Lessons learned are folded into the roadmap's "near-term"
  section.

## 11. References

- [Repo README](../../README.md) — capability table (current)
- [`../roadmap.md`](../roadmap.md) — live strategic roadmap
- [Python client](../../clients/python/) — `clients/python/`
- [Go client package](../../pkg/client/) — `pkg/client/`
- [JS client v1 + v2](../../clients/js/) — `clients/js/`
- [Rust client](../../clients/rust/) — `clients/rust/`
- [OpenAPI spec](../openapi.yaml) — v1 + v2 API surface
- [`../../UseCases/README.md`](../../UseCases/README.md) — 14
  use-case designs
- QDPs 0001–0010 — protocol features exposed by the clients
- [Integration Guide](../integration-guide.md) — multi-language
  client tutorials
