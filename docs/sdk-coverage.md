# SDK Coverage Matrix

This document is the authoritative cross-client reference for the Quidnug
client SDKs: which API endpoints each client implements, what naming
convention each uses, and how Tier-1 full-protocol SDKs map onto the
canonical Python surface.

The canonical surface is defined by:

- [`docs/openapi.yaml`](openapi.yaml) — 28 HTTP operations.
- [`clients/python/quidnug/client.py`](../clients/python/quidnug/client.py) —
  reference implementation, 47 public methods including a handful of
  ergonomic helpers (`ensure_domain`, `wait_for_identity`, …).

Whenever a new operation is added to the protocol, this matrix gets a
new row and every Tier-1 SDK is expected to gain the corresponding
method before the change ships.

## Client tiers

| Tier | Purpose | Coverage expectation |
| --- | --- | --- |
| **Tier 1 — Full protocol** | Stand-alone language SDKs targeting backend / CLI / native apps. | Full parity with the canonical Python surface. |
| **Tier 2 — Platform wrappers** | Mobile / browser-extension shells that wrap a Tier-1 SDK. | Native ergonomics + signing; full SDK reachable as an escape hatch. |
| **Tier 3 — Component libraries** | Framework UI kits for embedding trust + reviews into apps. | Hooks/components for the reads + writes their UI exposes. |
| **Tier 4 — Turnkey integrations** | CMS / e-commerce plugins for non-developer merchants. | Delegate to Tier-3 web-components; no language-specific API of their own. |

| Tier | Client |
| --- | --- |
| 1 | [`python`](../clients/python/) |
| 1 | [`js`](../clients/js/) |
| 1 | [`java`](../clients/java/) |
| 1 | [`rust`](../clients/rust/) |
| 1 | [`swift`](../clients/swift/) |
| 1 | [`dotnet`](../clients/dotnet/) |
| 2 | [`android`](../clients/android/) — wraps `java` |
| 2 | [`browser-extension`](../clients/browser-extension/) — signing proxy |
| 3 | [`react`](../clients/react/) |
| 3 | [`react-reviews`](../clients/react-reviews/) |
| 3 | [`vue-reviews`](../clients/vue-reviews/) |
| 3 | [`astro-reviews`](../clients/astro-reviews/) |
| 3 | [`web-components`](../clients/web-components/) |
| 3 | [`reviews-widget`](../clients/reviews-widget/) |
| 4 | [`wordpress-plugin`](../clients/wordpress-plugin/) |
| 4 | [`shopify-app`](../clients/shopify-app/) |
| 4 | [`iso20022`](../clients/iso20022/) → points to `integrations/iso20022/` |

## Tier 1 — Full protocol API matrix

All cells link an HTTP operation to the language-specific method name.
A blank cell means the SDK does not (yet) implement that operation.

Naming conventions per language:

- **Python**: `snake_case`, kw-only args for clarity.
- **JS**: `camelCase`. Core methods on `QuidnugClient`; v2 protocol
  extensions (guardian / gossip / bootstrap / fork-block) installed by
  the side-effect import `@quidnug/client/v2`.
- **Java**: `camelCase`, builder structs for write parameters.
- **Rust**: `snake_case`, async (tokio); reads return typed structs,
  writes return `serde_json::Value`.
- **Swift**: `camelCase`, `async throws` actor; uses Foundation types.
- **.NET**: `PascalCaseAsync`, `CancellationToken ct = default` last
  arg on every method.

### Health & node introspection

| HTTP | Python | JS | Java | Rust | Swift | .NET |
| --- | --- | --- | --- | --- | --- | --- |
| `GET /api/health` | `health` | `health` | `health` | `health` | `health` | `HealthAsync` |
| `GET /api/info` | `info` | `info` | `info` | `info` | `info` | `InfoAsync` |
| `GET /api/nodes` | `nodes` | `getNodes` | `nodes` | `nodes` | `nodes` | `NodesAsync` |
| `GET /api/node/domains` | `get_node_domains` | `getNodeDomains` | `nodeDomains` | `get_node_domains` | `nodeDomains` | `NodeDomainsAsync` |
| `POST /api/node/domains` | `update_node_domains` | `updateNodeDomains` | `updateNodeDomains` | `update_node_domains` | `updateNodeDomains` | `UpdateNodeDomainsAsync` |

### Domains

| HTTP | Python | JS | Java | Rust | Swift | .NET |
| --- | --- | --- | --- | --- | --- | --- |
| `GET /api/domains` | `list_domains` | `listDomains` | `listDomains` | `list_domains` | `listDomains` | `ListDomainsAsync` |
| `POST /api/domains` | `register_domain` | `registerDomain` | `registerDomain` | `register_domain` | `registerDomain` | `RegisterDomainAsync` |
| _(idempotent helper)_ | `ensure_domain` | `ensureDomain` | `ensureDomain` | `ensure_domain` | `ensureDomain` | `EnsureDomainAsync` |
| `GET /api/domains/{name}/query` | _(via registries)_ | `queryDomain` | _(via registries)_ | _(via registries)_ | _(via registries)_ | _(via registries)_ |

### Identity

| HTTP | Python | JS | Java | Rust | Swift | .NET |
| --- | --- | --- | --- | --- | --- | --- |
| `POST /api/transactions/identity` | `register_identity` | `createIdentityTransaction` + `submitTransaction` | `registerIdentity` | `register_identity` | `registerIdentity` | `RegisterIdentityAsync` |
| `GET /api/identity/{quid}` | `get_identity` | `getIdentity` | `getIdentity` | `get_identity` | `getIdentity` | `GetIdentityAsync` |
| `GET /api/registry/identity` | `query_identity_registry` | `queryIdentityRegistry` | `queryIdentityRegistry` | `query_identity_registry` | `queryIdentityRegistry` | `QueryIdentityRegistryAsync` |

### Trust

| HTTP | Python | JS | Java | Rust | Swift | .NET |
| --- | --- | --- | --- | --- | --- | --- |
| `POST /api/transactions/trust` | `grant_trust` | `createTrustTransaction` + `submitTransaction` | `grantTrust` | `grant_trust` | `grantTrust` | `GrantTrustAsync` |
| `GET /api/trust/{observer}/{target}` | `get_trust` | `getTrustLevel` | `getTrust` | `get_trust` | `getTrust` | `GetTrustAsync` |
| `POST /api/trust/query` | `query_relational_trust` | `queryRelationalTrust` | `queryRelationalTrust` | `query_relational_trust` | `queryRelationalTrust` | `QueryRelationalTrustAsync` |
| `GET /api/trust/edges/{quid}` | `get_trust_edges` | _(via path helpers)_ | `getTrustEdges` | `get_trust_edges` | `getTrustEdges` | `GetTrustEdgesAsync` |
| `GET /api/registry/trust` | `query_trust_registry` | `queryTrustRegistry` | `queryTrustRegistry` | `query_trust_registry` | `queryTrustRegistry` | `QueryTrustRegistryAsync` |

### Title / ownership

| HTTP | Python | JS | Java | Rust | Swift | .NET |
| --- | --- | --- | --- | --- | --- | --- |
| `POST /api/transactions/title` | `register_title` | `createTitleTransaction` + `submitTransaction` | `registerTitle` | `register_title` | `registerTitle` | `RegisterTitleAsync` |
| `GET /api/title/{asset}` | `get_title` | `getAssetOwnership` | `getTitle` | `get_title` | `getTitle` | `GetTitleAsync` |
| `GET /api/registry/title` | `query_title_registry` | `queryTitleRegistry` | `queryTitleRegistry` | `query_title_registry` | `queryTitleRegistry` | `QueryTitleRegistryAsync` |

### Events / streams

| HTTP | Python | JS | Java | Rust | Swift | .NET |
| --- | --- | --- | --- | --- | --- | --- |
| `POST /api/events` | `emit_event` | `createEventTransaction` + `submitTransaction` | `emitEvent` | `emit_event` | `emitEvent` | `EmitEventAsync` |
| `GET /api/streams/{subject}` | `get_event_stream` | `getEventStream` | `getEventStream` | `get_event_stream` | `getEventStream` | `GetEventStreamAsync` |
| `GET /api/streams/{subject}/events` | `get_stream_events` | `getStreamEvents` | `getStreamEvents` | `get_stream_events` | `getStreamEvents` | `GetStreamEventsAsync` |

### Blocks / transactions / pending

| HTTP | Python | JS | Java | Rust | Swift | .NET |
| --- | --- | --- | --- | --- | --- | --- |
| `GET /api/blocks` | `get_blocks` | `getBlocks` | `blocks` | `get_blocks` | `blocks` | `BlocksAsync` |
| `GET /api/blocks/tentative/{domain}` | `get_tentative_blocks` | `getTentativeBlocks` | `tentativeBlocks` | `get_tentative_blocks` | `tentativeBlocks` | `TentativeBlocksAsync` |
| `GET /api/transactions` | `get_pending_transactions` | `getPendingTransactions` | `pendingTransactions` | `get_pending_transactions` | `pendingTransactions` | `PendingTransactionsAsync` |

### IPFS

| HTTP | Python | JS | Java | Rust | Swift | .NET |
| --- | --- | --- | --- | --- | --- | --- |
| `POST /api/ipfs/pin` | `ipfs_pin` | `pinToIPFS` | `ipfsPin` | `ipfs_pin` | `ipfsPin` | `IpfsPinAsync` |
| `GET /api/ipfs/{cid}` | `ipfs_get` | `getFromIPFS` | `ipfsGet` | `ipfs_get` | `ipfsGet` | `IpfsGetAsync` |

### Guardians (QDP-0002 / QDP-0006)

| HTTP | Python | JS | Java | Rust | Swift | .NET |
| --- | --- | --- | --- | --- | --- | --- |
| `POST /api/guardian/set-update` | `submit_guardian_set_update` | `submitGuardianSetUpdate` | `submitGuardianSetUpdate` | `submit_guardian_set_update` | `submitGuardianSetUpdate` | `SubmitGuardianSetUpdateAsync` |
| `POST /api/guardian/recovery/init` | `submit_recovery_init` | `submitRecoveryInit` | `submitRecoveryInit` | `submit_recovery_init` | `submitRecoveryInit` | `SubmitRecoveryInitAsync` |
| `POST /api/guardian/recovery/veto` | `submit_recovery_veto` | `submitRecoveryVeto` | `submitRecoveryVeto` | `submit_recovery_veto` | `submitRecoveryVeto` | `SubmitRecoveryVetoAsync` |
| `POST /api/guardian/recovery/commit` | `submit_recovery_commit` | `submitRecoveryCommit` | `submitRecoveryCommit` | `submit_recovery_commit` | `submitRecoveryCommit` | `SubmitRecoveryCommitAsync` |
| `POST /api/guardian/resign` | `submit_guardian_resignation` | `submitGuardianResignation` | `submitGuardianResignation` | `submit_guardian_resignation` | `submitGuardianResignation` | `SubmitGuardianResignationAsync` |
| `GET /api/guardian/set/{quid}` | `get_guardian_set` | `getGuardianSet` | `getGuardianSet` | `get_guardian_set` | `getGuardianSet` | `GetGuardianSetAsync` |
| `GET /api/guardian/pending-recovery/{quid}` | `get_pending_recovery` | `getPendingRecovery` | `getPendingRecovery` | `get_pending_recovery` | `pendingRecovery` | `PendingRecoveryAsync` |
| `GET /api/guardian/resignations/{quid}` | `get_guardian_resignations` | `getGuardianResignations` | `guardianResignations` | `get_guardian_resignations` | `guardianResignations` | `GuardianResignationsAsync` |

### Cross-domain gossip (QDP-0003 / QDP-0005)

| HTTP | Python | JS | Java | Rust | Swift | .NET |
| --- | --- | --- | --- | --- | --- | --- |
| `POST /api/domain-fingerprints` | `submit_domain_fingerprint` | `submitDomainFingerprint` | `submitDomainFingerprint` | `submit_domain_fingerprint` | `submitDomainFingerprint` | `SubmitDomainFingerprintAsync` |
| `GET /api/domain-fingerprints/{domain}/latest` | `get_latest_domain_fingerprint` | `getLatestDomainFingerprint` | `getLatestDomainFingerprint` | `get_latest_domain_fingerprint` | `getLatestDomainFingerprint` | `GetLatestDomainFingerprintAsync` |
| `POST /api/anchor-gossip` | `submit_anchor_gossip` | `submitAnchorGossip` | `submitAnchorGossip` | `submit_anchor_gossip` | `submitAnchorGossip` | `SubmitAnchorGossipAsync` |
| `POST /api/gossip/push-anchor` | `push_anchor` | `pushAnchor` | `pushAnchor` | `push_anchor` | `pushAnchor` | `PushAnchorAsync` |
| `POST /api/gossip/push-fingerprint` | `push_fingerprint` | `pushFingerprint` | `pushFingerprint` | `push_fingerprint` | `pushFingerprint` | `PushFingerprintAsync` |

### Bootstrap (QDP-0008)

| HTTP | Python | JS | Java | Rust | Swift | .NET |
| --- | --- | --- | --- | --- | --- | --- |
| `POST /api/nonce-snapshots` | `submit_nonce_snapshot` | `submitNonceSnapshot` | `submitNonceSnapshot` | `submit_nonce_snapshot` | `submitNonceSnapshot` | `SubmitNonceSnapshotAsync` |
| `GET /api/nonce-snapshots/{domain}/latest` | `get_latest_nonce_snapshot` | `getLatestNonceSnapshot` | `getLatestNonceSnapshot` | `get_latest_nonce_snapshot` | `latestNonceSnapshot` | `LatestNonceSnapshotAsync` |
| `GET /api/bootstrap/status` | `bootstrap_status` | `getBootstrapStatus` | `bootstrapStatus` | `bootstrap_status` | `bootstrapStatus` | `BootstrapStatusAsync` |

### Fork-block (QDP-0009)

| HTTP | Python | JS | Java | Rust | Swift | .NET |
| --- | --- | --- | --- | --- | --- | --- |
| `POST /api/fork-block` | `submit_fork_block` | `submitForkBlock` | `submitForkBlock` | `submit_fork_block` | `submitForkBlock` | `SubmitForkBlockAsync` |
| `GET /api/fork-block/status` | `fork_block_status` | `getForkBlockStatus` | `forkBlockStatus` | `fork_block_status` | `forkBlockStatus` | `ForkBlockStatusAsync` |

### Merkle proofs (QDP-0010)

QDP-0010 is purely client-side verification — the node already emits
the proofs as part of `streams/{subject}/events`. Every Tier-1 SDK
provides a `verify_inclusion_proof` / `verifyInclusionProof` /
`VerifyInclusionProof` static helper. See each SDK's `Merkle` module.

### Ergonomic helpers (non-protocol)

These are convenience wrappers that don't map 1:1 onto an HTTP endpoint.
They're nice-to-haves rather than required — feel free to skip if you're
writing a new SDK.

| Helper | Python | Rust | Other |
| --- | --- | --- | --- |
| `wait_for_identity(quid, timeout, poll)` | ✓ | ✓ | demos in JS/Java/.NET use raw polling |
| `wait_for_identities([quid])` | ✓ | ✓ | — |
| `wait_for_title(asset, timeout, poll)` | ✓ | ✓ | — |
| `compute_transitive_trust(graph, …)` | — | — | JS only (`computeTransitiveTrust`) |
| `find_trust_path(…)` | — | — | JS only (`findTrustPath`) |

## Tier 2 — Platform wrappers

### Android (`clients/android/`)

Kotlin wrapper over the Java SDK adding:

- `AndroidKeystoreSigner` — StrongBox-backed P-256 keys.
- `QuidnugAndroidClient` — `suspend` bridges around `Dispatchers.IO` for
  `registerIdentity`, `grantTrust`, `getTrust`, `getTrustEdges`,
  `getIdentity`, `getTitle`, `emitEvent`, `streamEvents`,
  `getGuardianSet`, `submitGuardianSetUpdate`,
  `submitRecoveryInit/Veto/Commit`, `health`, `info`.
- `QuidVault` — dev-grade SharedPreferences-backed key store.

For every method not exposed as a `suspend` bridge, call through to
the underlying Java SDK via `client.raw()` and wrap in
`withContext(Dispatchers.IO) { ... }`.

### Browser extension (`clients/browser-extension/`)

Signing-only proxy; no protocol-method surface. Injects
`window.quidnug` with:

| Method | Purpose |
| --- | --- |
| `isUnlocked()` | Is the vault unlocked? |
| `listQuids()` | List `{ alias, id, publicKeyHex }`. |
| `sign(quidId, canonicalHex)` | Returns hex-DER signature. |
| `getNodeInfo()` | `{ url, token }` for the configured node. |

DApps should still use a Tier-1 SDK for everything except signing —
the extension is a key-custody boundary, not a wire-protocol client.

## Tier 3 — Component libraries

### `react` (`clients/react/`)

Hooks: `useQuid`, `useQuidnug`, `useTrust`, `useIdentity`, `useStream`,
`useGuardianSet`, `useRegisterIdentity`, `useGrantTrust`, `useEmitEvent`.

Components: `<TrustBadge>`, `<TrustPath>`, `<GuardianSetCard>`.

Provider: `<QuidnugProvider node initialQuid defaultDomain maxRetries retryBaseDelayMs>`.

For any other protocol method, call `useQuidnug().client.*` directly —
the full Tier-1 JS SDK is reachable through context.

### `react-reviews` (`clients/react-reviews/`)

Reviews-only React surface (delegates to `web-components`).

Hooks: `useTrustWeightedRating`, `useReviews`, `useWriteReview`.

Components: `<QuidnugStars>`, `<QuidnugReviewPanel>`, `<QuidnugReviewList>`,
`<QuidnugWriteReview>`.

Primitives: `<QnAurora>`, `<QnConstellation>`, `<QnTrace>`.

### `vue-reviews` (`clients/vue-reviews/`)

Vue 3 wrappers — currently only the low-level primitives:
`<QnAurora>`, `<QnConstellation>`, `<QnTrace>`. Composables
(`useTrustWeightedRating`, `useReviews`, `useWriteReview`) are
on the roadmap.

### `astro-reviews` (`clients/astro-reviews/`)

SSR-friendly Astro components: `<QnAurora>`, `<QnConstellation>`,
`<QnTrace>`. Each ships server-rendered SVG plus a hydrating custom
element.

### `web-components` (`clients/web-components/`)

Framework-agnostic custom elements:

| Element | Purpose |
| --- | --- |
| `<quidnug-review>` | Full product-review panel (computed + networked). |
| `<quidnug-stars>` | Weighted star display. |
| `<quidnug-write-review>` | Inline review-writing form. |
| `<quidnug-review-list>` | Per-review weighted list. |
| `<qn-aurora>` | Headline rating glyph (pure SVG). |
| `<qn-constellation>` | Trust-graph bullseye drilldown. |
| `<qn-trace>` | Horizontal stacked weight bar. |

Plus design tokens and standalone `renderAuroraSVG` /
`renderConstellationSVG` / `renderTraceSVG` helpers for build-time use.

### `reviews-widget` (`clients/reviews-widget/`)

Single-`<script>` embed with automatic upgrade between native
custom-element and iframe-fallback paths. Hosts the
`<quidnug-review>` element from `web-components`.

## Tier 4 — Turnkey integrations

These ship no language-specific protocol API — they orchestrate the
Tier-3 web-components via templating / theme extensions.

### `wordpress-plugin` (`clients/wordpress-plugin/`)

Shortcodes:

| Shortcode | Renders |
| --- | --- |
| `[quidnug-reviews product="..." topic="..."]` | `<quidnug-review>` panel. |
| `[quidnug-stars product="..." topic="..."]` | `<quidnug-stars>` widget. |

Plus a settings page (node URL, default topic), a WooCommerce
review-tab replacement, and Schema.org `AggregateRating` emission.

### `shopify-app` (`clients/shopify-app/`)

Scaffold for a Shopify-native app:

- **Theme app extension** — Liquid blocks dropping `<quidnug-review>`
  onto product pages.
- **Checkout UI extension** — post-purchase prompt.
- **Admin UI** — node URL + per-collection topic mapping.

Shopify App Store submission is on the roadmap; the scaffold is
sideloadable today.

### `iso20022` (`clients/iso20022/`)

A pointer file. The Go integration lives at
[`integrations/iso20022/`](../integrations/iso20022/) and wraps the
JSON event surface that every Tier-1 SDK already exposes — no
language-specific port required.

## Adding a new method

When a new HTTP operation lands on the node:

1. Update `docs/openapi.yaml` with the operation spec.
2. Add the canonical Python method to `clients/python/quidnug/client.py`
   (it's the reference).
3. Add the equivalent method to every Tier-1 SDK:
   `js`, `java`, `rust`, `swift`, `dotnet`. Use the naming convention
   for that language (see "Naming conventions per language" above).
4. Add a row to this matrix.
5. If the new method belongs to a guardian / gossip / bootstrap /
   fork-block flow that mobile apps care about, add a `suspend` bridge
   in `clients/android/src/main/java/com/quidnug/android/QuidnugAndroid.kt`.
   Otherwise it remains reachable via `client.raw().*`.
