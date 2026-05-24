# Quidnug client API coverage matrix

This file is the single source of truth for what each Quidnug client
covers, and the canonical method names that every full SDK client
must expose. Server endpoints live in
[`internal/core/handlers*.go`](../internal/core/) of the Go reference
implementation; this matrix tracks the SDKs that wrap them.

Clients fall into three buckets:

1. **Full SDKs** — Python, JavaScript, Rust, Java, Android (Kotlin
   wrapper over Java), Swift, .NET. These must implement every
   stable v1 endpoint and every v2 endpoint a SDK consumer reasonably
   needs to call (writes that require keypairs, plus the readback
   queries for each registry). They are kept in lockstep by the
   reference table below.
2. **UI/framework clients** — `@quidnug/react`, `@quidnug/react-reviews`,
   `@quidnug/vue-reviews`, `@quidnug/astro-reviews`,
   `@quidnug/web-components`, `@quidnug/reviews-widget`. These build
   on top of the JS SDK and only need to expose the review/trust
   surface their README documents.
3. **Platform integrations** — `browser-extension` (signing provider),
   `wordpress-plugin`, `shopify-app` (scaffold), `iso20022` (pointer
   to `integrations/iso20022/`). These are scoped to their host
   platform; coverage is judged against the README contract.

## Reference: stable v1 endpoints

Every full SDK exposes a method for each of these. Canonical method
names follow the **language-idiomatic case** for the verb but keep
the **same noun and the same parameter names** across SDKs.

| Endpoint | Python | JavaScript | Rust | Java/Android | Swift | .NET |
| --- | --- | --- | --- | --- | --- | --- |
| `GET /health` | `health()` | `getHealth()` | `health()` | `health()` | `health()` | `HealthAsync()` |
| `GET /info` | `info()` | `getInfo()` | `info()` | `info()` | `info()` | `InfoAsync()` |
| `GET /nodes` | `nodes()` | `getNodes()` | `nodes()` | `nodes()` | `nodes()` | `NodesAsync()` |
| `GET /peers` | `peers()` | `getPeers()` | `peers()` | `peers()` | `peers()` | `PeersAsync()` |
| `GET /peers/{nodeQuid}` | `get_peer(node_quid)` | `getPeer(nodeQuid)` | `get_peer(node_quid)` | `getPeer(nodeQuid)` | `getPeer(nodeQuid:)` | `GetPeerAsync(nodeQuid)` |
| `GET /transactions` | `get_pending_transactions()` | `getPendingTransactions()` | `get_pending_transactions()` | `pendingTransactions()` | `pendingTransactions()` | `PendingTransactionsAsync()` |
| `POST /transactions/trust` | `grant_trust(...)` | `createTrustTransaction(...)` | `grant_trust(...)` | `grantTrust(...)` | `grantTrust(...)` | `GrantTrustAsync(...)` |
| `POST /transactions/identity` | `register_identity(...)` | `createIdentityTransaction(...)` | `register_identity(...)` | `registerIdentity(...)` | `registerIdentity(...)` | `RegisterIdentityAsync(...)` |
| `POST /transactions/title` | `register_title(...)` | `createTitleTransaction(...)` | `register_title(...)` | `registerTitle(...)` | `registerTitle(...)` | `RegisterTitleAsync(...)` |
| `GET /blocks` | `get_blocks()` | `getBlocks()` | `get_blocks()` | `blocks()` | `blocks()` | `BlocksAsync()` |
| `GET /blocks/tentative/{domain}` | `get_tentative_blocks(domain)` | `getTentativeBlocks(domain)` | `get_tentative_blocks(domain)` | `getTentativeBlocks(domain)` | `getTentativeBlocks(domain:)` | `GetTentativeBlocksAsync(domain)` |
| `GET /domains` | `list_domains()` | `listDomains()` | `list_domains()` | `listDomains()` | `listDomains()` | `ListDomainsAsync()` |
| `POST /domains` | `register_domain(domain)` | `registerDomain(domain)` | `register_domain(domain)` | `registerDomain(domain)` | `registerDomain(domain:)` | `RegisterDomainAsync(domain)` |
| `GET /domains/top` | `top_domains()` | `getTopDomains()` | `top_domains()` | `topDomains()` | `topDomains()` | `TopDomainsAsync()` |
| `GET /domains/{name}/query` | `query_domain(name, ...)` | `queryDomain(name, ...)` | `query_domain(name, ...)` | `queryDomain(name, ...)` | `queryDomain(name:...)` | `QueryDomainAsync(name, ...)` |
| `GET /registry/trust` | `query_trust_registry()` | `queryTrustRegistry()` | `query_trust_registry()` | `queryTrustRegistry()` | `queryTrustRegistry()` | `QueryTrustRegistryAsync()` |
| `GET /registry/identity` | `query_identity_registry()` | `queryIdentityRegistry()` | `query_identity_registry()` | `queryIdentityRegistry()` | `queryIdentityRegistry()` | `QueryIdentityRegistryAsync()` |
| `GET /registry/title` | `query_title_registry()` | `queryTitleRegistry()` | `query_title_registry()` | `queryTitleRegistry()` | `queryTitleRegistry()` | `QueryTitleRegistryAsync()` |
| `POST /events` | `emit_event(...)` | `createEventTransaction(...)` | `emit_event(...)` | `emitEvent(...)` | `emitEvent(...)` | `EmitEventAsync(...)` |
| `POST /node-advertisements` | `create_node_advertisement(...)` | `createNodeAdvertisement(...)` | `create_node_advertisement(...)` | `createNodeAdvertisement(...)` | `createNodeAdvertisement(...)` | `CreateNodeAdvertisementAsync(...)` |
| `GET /streams/{subjectId}` | `get_event_stream(subject_id)` | `getEventStream(subjectId)` | `get_event_stream(subject_id)` | `getEventStream(subjectId)` | `getEventStream(subjectId:)` | `GetEventStreamAsync(subjectId)` |
| `GET /streams/{subjectId}/events` | `get_stream_events(subject_id)` | `getStreamEvents(subjectId)` | `get_stream_events(subject_id)` | `getStreamEvents(subjectId)` | `getStreamEvents(subjectId:)` | `GetStreamEventsAsync(subjectId)` |
| `POST /moderation/actions` | `create_moderation_action(...)` | `createModerationAction(...)` | `create_moderation_action(...)` | `createModerationAction(...)` | `createModerationAction(...)` | `CreateModerationActionAsync(...)` |
| `GET /moderation/actions/{targetType}/{targetId}` | `get_moderation_actions(...)` | `getModerationActions(...)` | `get_moderation_actions(...)` | `getModerationActions(...)` | `getModerationActions(...)` | `GetModerationActionsAsync(...)` |
| `GET /audit/head` | `audit_head()` | `getAuditHead()` | `audit_head()` | `auditHead()` | `auditHead()` | `AuditHeadAsync()` |
| `GET /audit/entries` | `audit_entries(...)` | `getAuditEntries(...)` | `audit_entries(...)` | `auditEntries(...)` | `auditEntries(...)` | `AuditEntriesAsync(...)` |
| `GET /audit/entry/{sequence}` | `audit_entry(seq)` | `getAuditEntry(seq)` | `audit_entry(seq)` | `auditEntry(seq)` | `auditEntry(seq:)` | `AuditEntryAsync(seq)` |
| `POST /privacy/dsr` | `create_dsr(...)` | `createDSR(...)` | `create_dsr(...)` | `createDSR(...)` | `createDSR(...)` | `CreateDSRAsync(...)` |
| `GET /privacy/dsr/{requestTxId}` | `get_dsr_status(tx_id)` | `getDSRStatus(txId)` | `get_dsr_status(tx_id)` | `getDSRStatus(txId)` | `getDSRStatus(txId:)` | `GetDSRStatusAsync(txId)` |
| `POST /privacy/consent/grants` | `create_consent_grant(...)` | `createConsentGrant(...)` | `create_consent_grant(...)` | `createConsentGrant(...)` | `createConsentGrant(...)` | `CreateConsentGrantAsync(...)` |
| `POST /privacy/consent/withdraws` | `create_consent_withdraw(...)` | `createConsentWithdraw(...)` | `create_consent_withdraw(...)` | `createConsentWithdraw(...)` | `createConsentWithdraw(...)` | `CreateConsentWithdrawAsync(...)` |
| `GET /privacy/consent/history` | `get_consent_history(subject)` | `getConsentHistory(subject)` | `get_consent_history(subject)` | `getConsentHistory(subject)` | `getConsentHistory(subject:)` | `GetConsentHistoryAsync(subject)` |
| `POST /privacy/restrictions` | `create_processing_restriction(...)` | `createProcessingRestriction(...)` | `create_processing_restriction(...)` | `createProcessingRestriction(...)` | `createProcessingRestriction(...)` | `CreateProcessingRestrictionAsync(...)` |
| `GET /privacy/restrictions/{subjectQuid}` | `get_restrictions_for_subject(quid)` | `getRestrictionsForSubject(quid)` | `get_restrictions_for_subject(quid)` | `getRestrictionsForSubject(quid)` | `getRestrictionsForSubject(quid:)` | `GetRestrictionsForSubjectAsync(quid)` |
| `POST /privacy/compliance` | `create_dsr_compliance(...)` | `createDSRCompliance(...)` | `create_dsr_compliance(...)` | `createDSRCompliance(...)` | `createDSRCompliance(...)` | `CreateDSRComplianceAsync(...)` |
| `POST /ipfs/pin` | `ipfs_pin(content)` | `pinToIPFS(content)` | `ipfs_pin(content)` | `ipfsPin(content)` | `ipfsPin(content:)` | `IpfsPinAsync(content)` |
| `GET /ipfs/{cid}` | `ipfs_get(cid)` | `getFromIPFS(cid)` | `ipfs_get(cid)` | `ipfsGet(cid)` | `ipfsGet(cid:)` | `IpfsGetAsync(cid)` |
| `GET /node/domains` | `get_node_domains()` | `getNodeDomains()` | `get_node_domains()` | `getNodeDomains()` | `getNodeDomains()` | `GetNodeDomainsAsync()` |
| `POST /node/domains` | `update_node_domains(list)` | `updateNodeDomains(list)` | `update_node_domains(list)` | `updateNodeDomains(list)` | `updateNodeDomains(list:)` | `UpdateNodeDomainsAsync(list)` |
| `POST /gossip/domains` | `send_domain_gossip(gossip)` | `sendDomainGossip(gossip)` | `send_domain_gossip(gossip)` | `sendDomainGossip(gossip)` | `sendDomainGossip(gossip:)` | `SendDomainGossipAsync(gossip)` |
| `POST /quids` | `generate_quid(metadata?)` | `generateQuid(metadata?)` | `generate_quid(metadata?)` | `generateQuid(metadata?)` | `generateQuid(metadata:)` | `GenerateQuidAsync(metadata?)` |
| `POST /trust/query` | `query_relational_trust(...)` | `queryRelationalTrust(...)` | `query_relational_trust(...)` | `queryRelationalTrust(...)` | `queryRelationalTrust(...)` | `QueryRelationalTrustAsync(...)` |
| `GET /trust/edges/{quidId}` | `get_trust_edges(quid)` | `getTrustEdges(quid)` | `get_trust_edges(quid)` | `getTrustEdges(quid)` | `getTrustEdges(quidId:)` | `GetTrustEdgesAsync(quid)` |
| `GET /trust/{observer}/{target}` | `get_trust(o, t)` | `getTrustLevel(o, t)` | `get_trust(o, t)` | `getTrust(o, t)` | `getTrust(observer:target:)` | `GetTrustAsync(o, t)` |
| `GET /identity/{quidId}` | `get_identity(quid)` | `getIdentity(quid)` | `get_identity(quid)` | `getIdentity(quid)` | `getIdentity(quidId:)` | `GetIdentityAsync(quid)` |
| `GET /title/{assetId}` | `get_title(asset_id)` | `getAssetOwnership(asset_id)` | `get_title(asset_id)` | `getTitle(asset_id)` | `getTitle(assetId:)` | `GetTitleAsync(asset_id)` |

## Reference: v2 endpoints

### Guardians (QDP-0002)

| Endpoint | Method (snake_case) | Method (camelCase) |
| --- | --- | --- |
| `POST /api/v2/guardian/set-update` | `submit_guardian_set_update` | `submitGuardianSetUpdate` |
| `POST /api/v2/guardian/recovery/init` | `submit_recovery_init` | `submitRecoveryInit` |
| `POST /api/v2/guardian/recovery/veto` | `submit_recovery_veto` | `submitRecoveryVeto` |
| `POST /api/v2/guardian/recovery/commit` | `submit_recovery_commit` | `submitRecoveryCommit` |
| `POST /api/v2/guardian/resign` | `submit_guardian_resignation` | `submitGuardianResignation` |
| `GET  /api/v2/guardian/set/{quid}` | `get_guardian_set` | `getGuardianSet` |
| `GET  /api/v2/guardian/pending-recovery/{quid}` | `get_pending_recovery` | `getPendingRecovery` |
| `GET  /api/v2/guardian/resignations/{quid}` | `get_guardian_resignations` | `getGuardianResignations` |

### Cross-domain gossip + fingerprints (QDP-0003 / QDP-0005)

| Endpoint | Method (snake_case) | Method (camelCase) |
| --- | --- | --- |
| `POST /api/v2/domain-fingerprints` | `submit_domain_fingerprint` | `submitDomainFingerprint` |
| `GET  /api/v2/domain-fingerprints/{domain}/latest` | `get_latest_domain_fingerprint` | `getLatestDomainFingerprint` |
| `POST /api/v2/anchor-gossip` | `submit_anchor_gossip` | `submitAnchorGossip` |
| `POST /api/v2/gossip/push-anchor` | `push_anchor` | `pushAnchor` |
| `POST /api/v2/gossip/push-fingerprint` | `push_fingerprint` | `pushFingerprint` |

### Bootstrap + fork-block (QDP-0008 / QDP-0009)

| Endpoint | Method (snake_case) | Method (camelCase) |
| --- | --- | --- |
| `POST /api/v2/nonce-snapshots` | `submit_nonce_snapshot` | `submitNonceSnapshot` |
| `GET  /api/v2/nonce-snapshots/{domain}/latest` | `get_latest_nonce_snapshot` | `getLatestNonceSnapshot` |
| `GET  /api/v2/bootstrap/status` | `bootstrap_status` | `getBootstrapStatus` |
| `POST /api/v2/fork-block` | `submit_fork_block` | `submitForkBlock` |
| `GET  /api/v2/fork-block/status` | `fork_block_status` | `getForkBlockStatus` |

### Discovery (QDP-0014)

| Endpoint | Method (snake_case) | Method (camelCase) |
| --- | --- | --- |
| `GET /api/v2/discovery/domain/{name}` | `discover_domain(name)` | `discoverDomain(name)` |
| `GET /api/v2/discovery/node/{quid}` | `discover_node(quid)` | `discoverNode(quid)` |
| `GET /api/v2/discovery/operator/{quid}` | `discover_operator(quid)` | `discoverOperator(quid)` |
| `GET /api/v2/discovery/quids` | `discover_quids()` | `discoverQuids()` |
| `GET /api/v2/discovery/trusted-quids` | `discover_trusted_quids()` | `discoverTrustedQuids()` |

### DNS attestation (QDP-0023)

| Endpoint | Method (snake_case) | Method (camelCase) |
| --- | --- | --- |
| `POST /api/v2/dns/claim` | `submit_dns_claim` | `submitDNSClaim` |
| `POST /api/v2/dns/challenge` | `submit_dns_challenge` | `submitDNSChallenge` |
| `POST /api/v2/dns/attestation` | `submit_dns_attestation` | `submitDNSAttestation` |
| `POST /api/v2/dns/renewal` | `submit_dns_renewal` | `submitDNSRenewal` |
| `POST /api/v2/dns/revocation` | `submit_dns_revocation` | `submitDNSRevocation` |
| `POST /api/v2/dns/delegate` | `submit_authority_delegate` | `submitAuthorityDelegate` |
| `POST /api/v2/dns/delegate-revocation` | `submit_authority_delegate_revocation` | `submitAuthorityDelegateRevocation` |
| `GET  /api/v2/dns/attestations/{domain}` | `get_dns_attestations(domain)` | `getDNSAttestations(domain)` |
| `GET  /api/v2/dns/attestations/{domain}/weighted` | `get_dns_attestations_weighted(domain)` | `getDNSAttestationsWeighted(domain)` |
| `GET  /api/v2/dns/resolve/{domain}/{recordType}` | `resolve_dns_record(domain, type)` | `resolveDNSRecord(domain, type)` |

## Per-client status

### Full SDKs

- **Python** (`clients/python`) — covers every endpoint in the
  reference tables above. Reference implementation; other SDKs are
  brought into line with its method names and parameter names.
- **JavaScript** (`clients/js`) — covers every endpoint, with v2
  surface bolted on by importing `@quidnug/client/v2` as a side
  effect (extends `QuidnugClient.prototype`).
- **Java** (`clients/java`) — covers every endpoint via
  `QuidnugClient`.
- **Android** (`clients/android`) — thin Kotlin wrapper over the
  Java client; `QuidnugAndroidClient` exposes every Java method as
  a suspending function on `Dispatchers.IO`. All transport goes
  through the wrapped Java client, so coverage = Java's coverage.
- **Swift** (`clients/swift`) — covers every endpoint via
  `QuidnugClient` actor.
- **.NET** (`clients/dotnet`) — covers every endpoint via
  `QuidnugClient`.
- **Rust** (`clients/rust`) — covers every v1 endpoint and every v2
  endpoint listed above.

### UI / framework clients

These have a narrower, declared scope. Coverage is judged against
their README, not the full API.

- **`@quidnug/react`** — provider + hooks for the trust /
  identity / event / guardian subset. Coverage matches README.
- **`@quidnug/react-reviews`** — review-rendering hooks + components
  on top of `@quidnug/react`. Coverage matches README.
- **`@quidnug/vue-reviews`** — primitives only; composables /
  components are documented as roadmap in its README.
- **`@quidnug/astro-reviews`** — SSR primitives (pure SVG renderers).
  Coverage matches README.
- **`@quidnug/web-components`** — `<quidnug-review>` /
  `<quidnug-stars>` / `<quidnug-review-list>` / `<quidnug-write-review>`
  + 3 SVG primitives. Coverage matches README.
- **`@quidnug/reviews-widget`** — single-line drop-in for the web
  components; just a loader. Coverage matches README.

### Platform integrations

- **`browser-extension`** — Manifest V3 extension that exposes
  `window.quidnug.{listQuids, sign, getNodeInfo, isUnlocked}` to
  pages. Holds quids in an AES-GCM vault. No HTTP surface of its
  own. Coverage matches README.
- **`wordpress-plugin`** — drops `<quidnug-review>` / `<quidnug-stars>`
  into WooCommerce product pages via shortcodes, enqueues the
  reviews-widget script, emits Schema.org JSON-LD. No direct API
  calls from PHP; everything flows through the JS layer. Coverage
  matches README.
- **`shopify-app`** — scaffold only; the README is explicit that no
  runnable extension code exists yet.
- **`iso20022`** — pointer directory; the real package lives at
  `integrations/iso20022/`.

## Keeping clients in lockstep

When adding a new endpoint to the Go server:

1. Add the row to the reference table in this file.
2. Add the method to every full SDK using the canonical name
   from the table, with the language-idiomatic case (snake_case
   for Python/Rust, camelCase for JS/Java/Kotlin/Swift, PascalCase
   + `Async` suffix for .NET).
3. If the endpoint takes a body, define the transaction struct
   in each SDK's `types`/`Types` module before wiring it up.
4. Add (or extend) the method table in each SDK's README.
