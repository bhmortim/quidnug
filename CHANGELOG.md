# Changelog

All notable changes to Quidnug are documented here. The format is based on
[Keep a Changelog](https://keepachangelog.com/en/1.1.0/) and the project
follows [Semantic Versioning](https://semver.org/spec/v2.0.0.html) from 1.0
onward.

## [Unreleased]

### Operator identity + peering subsystem (Phases 1-4)

Adds operator-quid persistence, three-source peer discovery (static
`peers_file` + gossip + mDNS LAN), an admit pipeline that gates each
peer by handshake + advertisement + operator-attestation, per-peer
quality scoring with quarantine + eviction policy, and full
state-persistence for restart durability. The protocol-level peering
convention (`peering.network.*` TRUST edges) is unchanged; this is
the runtime machinery that uses it.

**Operator identity (separate from per-process NodeID):**

- `operator_quid_file:` / `OPERATOR_QUID_FILE` — long-lived operator
  identity, deployable on N nodes simultaneously. Trust grants
  accumulate against the operator regardless of which node a
  counterparty interacts with.
- Per-process `NodeID` is now persisted to `data_dir/node_key.json`
  on first boot so it stays stable across restarts. Previously
  regenerated every boot and silently orphaned trust grants.

**Three peer sources:**

- `peers_file:` — operator-managed YAML list, fsnotify-watched for
  live reload. Per-entry `allow_private: true` whitelists LAN peers.
- `lan_discovery: true` — mDNS / DNS-SD on `_quidnug._tcp.local.`,
  opt-in. Self-discovery filtered out. Pure-Go zeroconf, no Avahi
  dependency.
- `seed_nodes:` (existing) — gossip discovery, now hardened with
  exponential-backoff retry on early-boot DNS races (1s→30s) and
  cycle-summary INFO logging on every round.

**Admit pipeline** (`internal/core/peer_admit.go`):

1. Address validation (`safedial`) — refuses RFC1918/loopback/
   link-local/metadata IPs by default; per-peer `allow_private`
   override only for static + LAN sources.
2. Handshake — `GET /api/v1/info` to learn claimed NodeQuid +
   OperatorQuid.
3. NodeAdvertisement lookup — `require_advertisement: true` (default)
   gates gossip-learned peers.
4. Operator attestation — TRUST edge `OperatorQuid → NodeQuid` at
   weight ≥ `peer_min_operator_trust` (default 0.5).
5. Optional weighted-aggregate operator reputation gate.

**Per-peer scoring** (`internal/core/peer_score.go`):

- Five event classes (handshake, gossip, query, broadcast, validation)
  + three severe events (fork claim, signature fail, ad revocation).
- Composite score in `[0, 1]`, exponentially-decayed Laplace-smoothed
  per-class rates minus severe-event penalties.
- Quarantine at 0.4 (configurable, with hysteresis), eviction at 0.2
  for 5 minutes (configurable). Static-source peers are
  eviction-immune by default with a stern warning.
- Routing preference: `sortedForwardPeers` (gossip fan-out) and
  `preferByScore` (query candidate ordering) sort by composite
  descending and exclude quarantined peers.
- Persists to `data_dir/peer_scores.json` every 5 min + on shutdown.

**API + CLI surface:**

- `GET /api/v1/peers` — full scoreboard, worst-first.
- `GET /api/v1/peers/{nodeQuid}` — single peer + recent-event ring.
- Landing page at `/` shows operator quid + peer source breakdown +
  worst-scoring peers.
- `quidnug-cli peer list / show / add / remove / scan-lan`.
- `quidnug-cli quid generate` (replaces deprecated `keygen`).

**State persistence under `data_dir`:**

- `node_key.json` — per-process ECDSA keypair (NodeID stable across
  restarts).
- `blockchain.json` — block history snapshot, every 30s + on shutdown.
- `trust_domains.json` — TrustDomains + DomainRegistry (dynamic
  domain registrations now survive restart).
- `peer_scores.json` — peer scoreboard.
- `pending_transactions.json` — existing pending tx queue.

All writes are atomic through `internal/safeio`; files are
schema-versioned.

### QRP-0001 trust-weighted reviews protocol + rating visualization

The first domain-level protocol built on top of the Quidnug
infrastructure QDPs. Ships as a complete drop-in library
across every major web framework.

**Protocol + algorithm**

- `examples/reviews-and-comments/PROTOCOL.md` — QRP-0001 spec:
  six event types (REVIEW, HELPFUL_VOTE, UNHELPFUL_VOTE, REPLY,
  FLAG, PURCHASE), topic tree (`reviews.public.*`), domain
  inheritance with 0.8-per-hop decay.
- `examples/reviews-and-comments/algorithm.md` + `algorithm.py`
  — four-factor rating (T, H, A, R: topical trust, helpfulness,
  activity, recency) with 12 passing reference tests.
- `examples/reviews-and-comments/bootstrap-trust.md` — four
  mechanisms for new users to enter the trust graph (OIDC
  binding, cross-site import, social bootstrap, domain validator
  opt-in).

**Reference Go node fixes surfaced during QRP-0001 work**

- `GenerateBlock` now includes `EventTransaction` in its
  domain-extraction switch (previously events lingered in the
  pending pool indefinitely).
- `ValidateBlockTiered` now validates `EventTransaction` in its
  per-tx switch (previously blocks containing events were
  rejected as `BlockInvalid`).
- `cmd/quidnug/main.go` startup now passes
  `cfg.RateLimitPerMinute` and `cfg.MaxBodySizeBytes` to
  `StartServerWithConfig` instead of discarding them for the
  hardcoded defaults.
- `internal/core/reviews_integration_test.go` guards all three
  of the above against silent regression with an end-to-end
  in-process test of the full QRP-0001 round-trip.

**Rating visualization primitives** (`clients/web-components/src/primitives/`)

Three zero-dependency SVG custom elements that carry substantially
more signal than five stars while staying SEO-safe:

- `<qn-aurora>` — sentiment dot + confidence ring + delta chip
  + optional radial histogram. Three sizes (nano / standard /
  large) sharing one visual vocabulary so product grids and
  detail pages feel continuous.
- `<qn-constellation>` — bullseye drilldown. Concentric tiers
  encode trust-hop distance; each dot is one contributing
  reviewer (color = rating, size = weight, outline = direct vs.
  transitive).
- `<qn-trace>` — horizontal stacked weight bar. One segment per
  contributor. Good for side-by-side comparison.

All three expose pure `render*SVG()` functions used by the SSR
Astro adapter. Design tokens live in a single
`design-tokens.js` module overridable via CSS custom properties.
20 node:test cases cover the token bucket boundaries and the
pure renderer outputs. Accessibility guarantees (shape-redundant
sentiment encoding, aria-labels, keyboard focus) documented in
`docs/reviews/rating-visualization.md`.

**Framework adapters**

- `@quidnug/web-components` — custom elements + primitive exports
- `@quidnug/reviews-widget` — one-line HTML embed
- `@quidnug/react-reviews` — React hooks + components + primitive
  React wrappers
- `@quidnug/vue-reviews` (new package) — Vue 3 primitive wrappers
  with `isCustomElement` setup docs
- `@quidnug/astro-reviews` (new package) — SSR-first Astro
  components that emit real SVG at build time and hydrate the
  custom element on the client
- `clients/wordpress-plugin/` — WooCommerce integration
- `clients/shopify-app/` — Shopify scaffold

**Working end-to-end demo** (`examples/reviews-and-comments/demo/`)

- `demo.py` — posts 16 identities, product + title, 14 trust
  edges, 5 reviews, 8 helpfulness votes, and 10 activity fillers
  against a live Quidnug node, then computes three divergent
  per-observer ratings (Alice 4.53, Bob 4.34, Carol 4.50) from
  the same 5 raw reviews whose unweighted average is 3.96.
- `sign_helper/main.go` — Go subprocess that produces byte-compatible
  IEEE-1363 + SEC1 signatures, invoked from Python via stdin/stdout
  JSON lines. Closes the Python-DER vs Go-1363 incompatibility
  without reimplementing ECDSA.
- `index.html` — clickable per-observer UI with the full T/H/A/R
  factor breakdown.

**Go toolchain bump**

- `go.mod` now requires Go 1.25. Dockerfile, CI matrix, and
  `ci.yml` lint runner updated to match (`golangci-lint` bumped
  to v2.4.0 via `golangci-lint-action@v7`, config migrated to
  `version: "2"` format).

### QDP-0011 client libraries & integrations

The client/integration surface lands in a tiered rollout:

**Tier 1 — SDKs with full protocol parity**

- **Python 2.0.0** (`clients/python/`) — new packaged SDK with typed
  dataclasses, requests-based HTTP client, ECDSA P-256, canonical
  signable bytes matching Go byte-for-byte, QDP-0010 proof verifier,
  structured error taxonomy, four runnable examples, 42 passing tests.
- **Go client package** (`pkg/client/`) — external-consumer-safe
  package mirroring the Python API. Functional-options constructor,
  context-aware methods, typed errors, inclusion-proof verifier.
- **JS client v2** (`@quidnug/client@2.0.0`) — additive mixin adding
  guardian / gossip / bootstrap / fork-block / merkle methods on top
  of the existing v1 class. TypeScript module augmentation.
- **`quidnug-cli`** (`cmd/quidnug-cli/`) — operator CLI wrapping the
  Go SDK. Structured exit codes (0/2/3/4/5/6), offline Merkle verify.
- **Observability bundle** (`deploy/observability/`) — Grafana
  dashboard + Prometheus alert rules matching the live `quidnug_*`
  metric family.

**Tier 2 — deployment + enterprise identity**

- **Helm chart** (`deploy/helm/quidnug/`) — production-grade
  StatefulSet with PVCs, PDB, zone-spread anti-affinity,
  ServiceMonitor + PrometheusRule opt-ins.
- **Abstract signer interface** (`pkg/signer/`) — decouples key
  custody from signing.
- **PKCS#11 / HSM signer** (`pkg/signer/hsm/`) — wraps SoftHSM,
  YubiHSM, CloudHSM, Azure KV, GCP HSM. Build-tag-gated CGo.
- **WebAuthn / FIDO2 bridge** (`pkg/signer/webauthn/`) — server-side
  coordinator for passkey / Touch ID / Windows Hello signing.
- **Rust SDK** (`clients/rust/`) — new crate `quidnug` 2.0.0, async
  reqwest client, wiremock-tested, full protocol surface.
- **OIDC bridge** (`internal/oidc/` + `cmd/quidnug-oidc/`) — binds
  Okta / Auth0 / Azure / Keycloak / Google Workspace subjects to
  Quidnug quids with immutable bindings.

**Tier 3 — domain integrations + language scaffolds**

- **Sigstore / cosign** (`integrations/sigstore/`) — record verified
  cosign bundles as SIGSTORE_SIGNATURE events on artifact titles.
- **C2PA** (`integrations/c2pa/`) — record verified C2PA manifests as
  C2PA_MANIFEST events on media-asset titles.
- **HL7 FHIR** (`integrations/fhir/`) — record FHIR R4/R5 resources
  as FHIR_RESOURCE.{ResourceType} events on patient titles.
- **Chainlink External Adapter** (`integrations/chainlink/`) — expose
  relational-trust queries to on-chain smart contracts.
- **Java / Kotlin scaffold** (`clients/java/`) — Gradle Kotlin DSL,
  Quid keygen + sign + verify with BouncyCastle.
- **C# / .NET scaffold** (`clients/dotnet/`) — .NET 8 /
  netstandard2.1, Quid keygen + sign + verify with built-in ECDsa.
- **ISO 20022 mapping scaffold** (`clients/iso20022/`).

**Tier 4 — platform scaffolds**

- Swift (iOS/macOS), Android, browser extension, React hook library,
  gRPC gateway, GraphQL gateway, WebSocket subscriptions, Terraform
  provider, Ledger app, MQTT bridge, Postgres extension, Elastic
  ingester — all scaffolded with README roadmaps.

**Supporting infrastructure**

- Language-agnostic JSON schemas (`schemas/json/`) plus the canonical
  signable-bytes specification (`schemas/types/canonicalization.md`)
  that every SDK implements identically.
- Docker Compose consortium (`deploy/compose/`) for local dev.
- Postman collection (`docs/postman/`) covering every endpoint.
- Multi-language integration guide (`docs/integration-guide.md`).
- SDK matrix CI (`.github/workflows/sdk-matrix.yml`) covering Go,
  Python 3.9–3.13, Node 18/20/22, Rust stable, Helm lint.

### QDP-0010 compact Merkle proofs (H2, new, soft fork)

- **New Block field** `TransactionsRoot`: SHA-256 binary Merkle
  root over canonical transaction bytes. Computed at block seal
  time; omitempty for backward compatibility with pre-H2 blocks.
- **Leaf canonicalization**: `sha256(canonicalMarshal(tx))` where
  `canonicalMarshal` applies the QDP-0003 §8.3 map-round-trip
  pattern. Ensures typed-struct and JSON-unmarshaled leaf hashes
  match bit-for-bit across the network.
- **Inclusion proofs**: `MerkleProofFrame` records the sibling
  hash and its concat side (left/right) at each tree level.
  Bitcoin-style odd-tail duplication for non-power-of-2 block
  sizes.
- **AnchorGossipMessage.MerkleProof**: optional field. When
  populated, receivers verify inclusion via the proof and skip
  full-block reconstruction. Pre-H2 messages and shadow-period
  producers without proofs fall back to full-block
  verification.
- **Proof length cap**: `ceil(log2(MaxTxsPerBlock=4096)) + 1`
  frames. Longer proofs rejected as amplification attempts.
- **Block validation hook**: when the `require_tx_tree_root`
  feature has been activated via QDP-0009 fork, blocks with
  empty `TransactionsRoot` are rejected as malformed. Pre-fork
  activation the field is optional (shadow period).
- **Metrics**: `merkle_proof_used_total`,
  `merkle_proof_fallback_total{reason}`,
  `merkle_proof_verify_fail_total{reason}`,
  `block_missing_tx_root_rejected_total`.
- **Rollout**: soft-fork via QDP-0009. Stage 1 populates the
  field (shadow). Stage 2 attaches proofs to gossip. Stage 3
  activates `require_tx_tree_root` at a coordinated ForkHeight;
  receivers enforce the field from that block on.

### QDP-0009 fork-block migration trigger (H5, new)

- **New AnchorKind** `AnchorForkBlock` (value 9) + transaction
  type `FORK_BLOCK`. Declares "at block height H in domain D,
  every node honoring this tx flips feature F." The `ForkHeight`
  is the synchronization boundary; pre-fork blocks validate
  under old rules, post-fork under new.
- **Supported features** enumerated in `ForkSupportedFeatures`:
  `enable_nonce_ledger`, `enable_push_gossip`,
  `enable_lazy_epoch_probe`, `enable_kofk_bootstrap`,
  `require_tx_tree_root` (future H2). Unknown features rejected.
- **Validator quorum**: signatures from at least 2/3 (ceiling)
  of the domain's declared validators required. Each signature
  validates against the signer's current-epoch key; duplicate
  signers rejected.
- **Notice period**: `MinForkNoticeBlocks` (default 1440 ≈ 24h)
  between acceptance and `ForkHeight`. Forks scheduled too soon
  are rejected so operators have coordination time.
- **Nonce monotonicity** per `(domain, feature)` — replay
  protection across forks.
- **Supersede window**: a later fork with strictly-higher nonce
  arriving BEFORE the earlier `ForkHeight` replaces it. After
  activation, the fork is historical fact and new forks for the
  same (domain, feature) are rejected.
- **Activation path**: after block-tx processing, every pending
  fork whose `ForkHeight <= block.Index` activates idempotently
  — catches up correctly when a node replays a chain that has
  already crossed activation boundaries.
- **HTTP surface** `/api/v2/fork-block` (POST) and
  `/api/v2/fork-block/status` (GET).
- **Metrics**: `fork_block_accepted_total`,
  `fork_block_rejected_total`, `fork_block_activated_total`.

### QDP-0008 K-of-K snapshot bootstrap (H3, new, shadow flag)

- **BootstrapFromPeers**: fetches latest `NonceSnapshot` from
  up to 2K peers for a named domain, groups responses by
  `BlockHash`, requires the largest group to have >= K
  agreeing peers. Any disagreement fails closed (QuorumMissed).
- **Height tolerance**: peers in the winning group may differ
  by at most `HeightTolerance` (default 4) blocks. Larger
  spread is evidence of real divergence, rejected.
- **Stale tolerance**: snapshots older than `StaleTolerance`
  (default 30d) are excluded from the quorum count.
- **Signature validation per peer**: peers whose snapshot
  signature fails against the ledger's known public key for
  the producer are dropped from the quorum count, not treated
  as votes.
- **Trust list**: `SeedBootstrapTrustList` seeds operator-
  asserted root-trust (`BootstrapTrustEntry{Quid, PublicKey}`)
  into the ledger's epoch-0 signer keys before bootstrap, so
  snapshot signatures have something to verify against. Requires
  at least K entries.
- **Apply + shadow-verify**: `ApplyBootstrapSnapshot` seeds the
  ledger's `accepted` / `tentative` maps from the consensus
  snapshot. The session then enters shadow-verify for the first
  N blocks (default 64). `ShadowVerifyStep` catches a seed that
  is inconsistent with live chain state (halts the node with
  `ErrBootstrapDivergence`).
- **Operator override** `BootstrapTrustedPeer`: when set, a
  K-of-K failure that contains the trusted peer in the responses
  is accepted with a warning.
- **HTTP endpoints** under `/api/v2/`:
  - `GET nonce-snapshots/{domain}/latest` — serve stored
    snapshot.
  - `POST nonce-snapshots` — accept a snapshot; validates sig
    and stores monotonically.
  - `GET bootstrap/status` — operator visibility into current /
    last session.
- **Metrics** on session outcomes + shadow divergence.
- **Migration**: additive, flag-gated `EnableKofKBootstrap`.
  Pre-H3 peers that don't serve the endpoint are simply not
  counted toward quorum.

### QDP-0007 lazy epoch propagation (H4, new, shadow flag)

- **Stale-signer quarantine.** When `EnableLazyEpochProbe` is on,
  a transaction from a signer whose local epoch state is older
  than `EpochRecencyWindow` (default 7d) is held in an in-memory
  quarantine queue pending an asynchronous probe against the
  signer's home domain.
- **Home-domain probe.** New `ProbeHomeDomain` client issues
  `GET /api/v2/domain-fingerprints/{home}/latest` to up to 3
  peers that serve the home domain. A valid signed fingerprint
  updates the ledger and refreshes the signer's recency;
  quarantined txs are released back into `PendingTxs`.
- **HomeDomain field on IdentityTransaction.** Optional; empty
  falls back to the node's primary supported domain. Backward-
  compatible — pre-H4 identity records don't have the field.
- **Recency hooks.** `MarkEpochRefresh` is called from three
  paths: Trusted-block anchor application, push/pull gossip
  arrival, and successful probe. Any of these counts as
  evidence of a live path.
- **Overflow + age-out.** Quarantine is bounded to 1024 entries
  (oldest evicted on overflow) and entries older than 1h are
  swept periodically. Both emit metrics.
- **Timeout policy.** `ProbeTimeoutPolicy` = `reject` (default)
  drops stale txs when the probe fails; `admit_warn` admits
  with a warning log + metric.
- **Metrics**: `quarantine_size`, `quarantine_enqueued_total`,
  `quarantine_released_total`, `quarantine_rejected_total`,
  `probe_attempts_total`, `probe_success_total`,
  `probe_failure_total`.
- **Feature flag** `EnableLazyEpochProbe` (env
  `ENABLE_LAZY_EPOCH_PROBE`); default off. Timing knobs
  `EPOCH_RECENCY_WINDOW`, `EPOCH_PROBE_TIMEOUT`,
  `PROBE_TIMEOUT_POLICY` all env-configurable.

### QDP-0006 guardian resignation (H6, new)

- **New anchor kind** `AnchorGuardianResign` (value 8, append-only)
  and transaction type `GUARDIAN_RESIGN`. A guardian signs a
  `GuardianResignation` to withdraw their consent from a named
  subject's recovery quorum. No subject cooperation required.
- **Set-hash binding**: each resignation carries
  `GuardianSetHash`, the sha256 of the exact installed set it
  targets. If the subject updates the set after the resignation
  is signed, the resignation is stale and rejected with
  `ErrResignationSetHashMismatch`.
- **Per-(guardian, subject) monotonic nonce**: resignations use a
  dedicated nonce stream keyed by pair, so a guardian in
  multiple subjects' sets manages each independently. Replays
  rejected.
- **EffectiveAt window**: must be `>= now − 5 min` (clock skew
  tolerance) and `<= now + 365 days`. Future-effective
  resignations are stored but not active until the timestamp
  passes.
- **Prospective only**: a resignation does NOT retroactively
  invalidate a pending recovery's Init authorization (QDP-0006
  §7). Recovery proceeds on the set as-it-was at Init time.
- **EffectiveGuardianSet** accessor: returns the installed set
  with resigned guardians' weights zeroed. All threshold-checking
  code paths use this accessor; the raw `GuardianSetOf` remains
  for audit / GuardianSetUpdate authorization.
- **Weakened-set metric**: when the effective weight drops below
  threshold, `quidnug_guardian_set_weakened_total` fires. Set is
  still usable for recovery; the metric is operator visibility.
- **HTTP surface** under `/api/v2/guardian/`:
  - `POST resign` — submit a signed resignation. 202 on accept,
    200 with `duplicate: true` on idempotent replay.
  - `GET resignations/{quid}` — list all resignations for a
    subject plus the weakened flag.
- **Metrics**: `quidnug_guardian_resignations_total{subject}`,
  `quidnug_guardian_resignations_rejected_total{reason}`,
  `quidnug_guardian_set_weakened_total{subject}`.
- **Push-gossip integration**: resignations are emitted as
  anchor push gossip (H1) when sealed by a validator, so cross-
  domain nodes learn of resignations without polling.
- **Migration**: additive; no hard fork. Mixed-version nodes
  that don't recognize the anchor kind simply ignore it.

### QDP-0005 push-based gossip (H1, new, shadow flag)

- **Push envelopes** (`AnchorPushMessage`, `FingerprintPushMessage`):
  wrap the existing QDP-0003 payloads with routing metadata
  (`TTL`, `HopCount`, `ForwardedBy`) that is explicitly excluded
  from the producer's signature. Envelope fields can be mutated
  per hop without breaking authenticity.
- **Receive pipeline**: dedup → subscription filter → validate →
  apply → forward. Dedup runs first so replay floods are cheap;
  subscription runs before ECDSA so unsubscribed producers don't
  cost verification. TTL is clamped server-side on receipt to
  prevent amplification via a forged-high TTL.
- **Subscription (implicit)**: a node is "subscribed" to an anchor
  if its ledger has `signerKeys` for either the producer or the
  anchor subject. A node is subscribed to a fingerprint if it has
  any block in the domain or a previously-stored fingerprint for
  it. No explicit registration — interest is derivable from
  existing state.
- **Producer-side fan-out**: validators that seal a block
  containing an anchor (AnchorTransaction, GuardianSetUpdate,
  GuardianRecoveryInit/Veto/Commit) originate push messages over
  the existing `KnownNodes` mesh. Receivers decrement TTL and
  continue fan-out up to `DomainGossipTTL` hops.
- **Per-producer rate limit** (token-bucket, 30 msg / 60s default):
  apply-then-choke semantics — a genuinely new message from an
  over-cap producer is still applied locally, but forwarding is
  suppressed. Local truth is never sacrificed to rate control.
  LRU-evict with a 1024-bucket cap to bound memory.
- **HTTP endpoints** under `/api/v2/`:
  - `POST gossip/push-anchor` — receive a push-anchor envelope.
  - `POST gossip/push-fingerprint` — receive a push-fingerprint
    envelope.
  Both return 202 on first accept, 200 on dedup, 400 on schema /
  validation failure.
- **Metrics**: `quidnug_gossip_push_received_total`,
  `quidnug_gossip_push_forward_dropped_total`,
  `quidnug_gossip_push_rate_limited_total`,
  `quidnug_gossip_push_propagation_latency_seconds`.
- **Feature flag** `EnablePushGossip` (env var `ENABLE_PUSH_GOSSIP`,
  default **off**). Controls whether this node ORIGINATES pushes;
  receivers always accept so the rollout can proceed
  early-adopter → laggard without gating.
- **Canonicalization** inherited verbatim from QDP-0003 — payload
  signable bytes are unchanged, so a push envelope interoperates
  with existing pull endpoints on the wire.

### QDP-0003 cross-domain anchor gossip (new)

- **`DomainFingerprint`**: a signed claim by a domain validator that
  a specific block in that domain was sealed with a given hash and
  height. Serves as the trust anchor for cross-domain gossip (§7.3).
  Monotonic storage: older heights don't overwrite newer ones.
- **`AnchorGossipMessage`**: ships a full origin block + inline
  fingerprint + producer signature to any node that also cares about
  the signer. The receiver verifies the full chain (gossip sig →
  fingerprint sig → block self-hash → tx index → deduplication) before
  dispatching the anchor through the same apply path as an in-domain
  Trusted-block anchor. Global-per-signer ledger state (epoch, keys,
  caps) is updated; domain-scoped nonce counters are NOT (that remains
  each domain's local consensus concern).
- **Gossip-signature canonicalization**: signs over `OriginBlock.Hash`
  rather than the full block contents. `OriginBlock.Transactions` is
  `[]interface{}`, and JSON round-trips change the shape of its
  entries from typed wrapper structs to `map[string]interface{}`,
  which would make signatures un-verifiable after transmission.
  Signing the hash avoids this entirely.
- **HTTP endpoints** under `/api/v2/`:
  - `POST domain-fingerprints` — submit a signed fingerprint.
  - `GET domain-fingerprints/{domain}/latest` — fetch the stored
    latest fingerprint for a domain.
  - `POST anchor-gossip` — submit a cross-domain anchor gossip
    message. Returns 202 on first acceptance, 200 with `duplicate:
    true` on replay (so relay retries are idempotent).
- **Deduplication-first validation**: gossip MessageID is checked
  before the expensive ECDSA verification. This matters extra when
  the gossip carries the producer's own rotation: applying the
  rotation advances the producer's `currentEpoch`, which would make
  a naive re-verification of the same message fail against the new
  key. Dedup-first prevents that false-positive failure on replays.

### QDP-0002 guardian-based recovery (new)

- **Guardian set installation** (`AnchorGuardianSetUpdate`) with full
  three-layer authorization per QDP-0002 §6.4.4:
  - **Always:** primary-key signature.
  - **Always:** consent from every guardian in the new set (no
    "unwitting guardians" — a guardian who hasn't signed the
    installation cannot later be held responsible for authorizing a
    recovery).
  - **On replace:** threshold-of-current guardians also sign, so an
    attacker with the primary key cannot swap in colluder guardians.
- **Time-locked recovery** (`AnchorGuardianRecoveryInit` →
  `AnchorGuardianRecoveryCommit`): guardians jointly propose a new
  key, a `RecoveryDelay` elapses, then any committer publishes the
  finalization. Maturity + audit-signature enforced server-side.
- **Primary-key veto** (`AnchorGuardianRecoveryVeto`): a single
  signature from the subject's current key cancels a pending
  recovery. Guardian-threshold veto also supported (the
  "coerced-guardian disavows" path). Exactly one of the two
  authorization paths must be present.
- **RequireGuardianRotation flag**: subjects that opt in have their
  plain `AnchorRotation` rejected — the only permitted rotation path
  becomes guardian recovery. Useful for high-value quids that want
  multi-party approval even when their primary key is uncompromised.
- **Weighted guardians**: each `GuardianRef` may declare a weight
  (default 1). Thresholds are weighted sums.
- **HTTP endpoints under `/api/v2/guardian/`**:
  - `POST set-update` — submit a signed `GuardianSetUpdate`.
  - `POST recovery/init` — submit a signed `GuardianRecoveryInit`.
  - `POST recovery/veto` — submit a signed veto.
  - `POST recovery/commit` — submit a mature-delay commit.
  - `GET set/{quid}` — query the installed guardian set.
  - `GET pending-recovery/{quid}` — query in-flight recovery state.

### Licensing

- **BREAKING (legal):** Relicensed from AGPL-3.0 to Apache-2.0. Downstream
  users previously relying on AGPL terms (including the network-use clause)
  should review the new terms. See `LICENSE`.

### Added

- `NOTICE` file (Apache-2.0 attribution).
- `SECURITY.md` with a private-disclosure process.
- `CONTRIBUTING.md` describing the development workflow and contribution
  license terms.
- `CODE_OF_CONDUCT.md` (Contributor Covenant 2.1).
- HTTP server timeouts (`ReadHeaderTimeout`, `ReadTimeout`, `WriteTimeout`,
  `IdleTimeout`) to mitigate Slowloris and similar DoS vectors.
- Optional TLS: set `TLS_CERT_FILE` and `TLS_KEY_FILE` to serve HTTPS.
- Security headers middleware (`X-Content-Type-Options`, `X-Frame-Options`,
  `Referrer-Policy`, `Strict-Transport-Security` under TLS).
- Trusted-proxy gating for `X-Forwarded-For` / `X-Real-IP`: spoofed headers
  are ignored unless the request's immediate peer IP is inside
  `TRUSTED_PROXIES` (CIDR list).
- LRU/idle eviction on `IPRateLimiter` to bound memory under IP-rotation
  attacks (`RATE_LIMITER_MAX_IPS`, `RATE_LIMITER_IDLE_TTL`).
- `DisallowUnknownFields` on JSON request decoding.
- Configurable node-auth timestamp tolerance via
  `NODE_AUTH_TIMESTAMP_TOLERANCE_SECONDS` (default unchanged for
  compatibility; operators can tighten to 60s).
- Missing unit tests: `crypto_test.go`, `metrics_test.go`,
  `persistence_test.go`, `types_test.go`.
- `gosec`, `govulncheck`, and Trivy image scanning in CI.
- `npm audit` in the JS client workflow.
- Retry-After header honoring in the JS client's retry logic.
- Response-shape validation in the JS client.
- `package.json` metadata (license, author, repository, keywords,
  `publishConfig`).
- Docker `HEALTHCHECK` and non-root runtime user.
- `README.md` Quickstart section.

### Changed

- Go toolchain bumped from 1.21 to 1.23 in `go.mod`, `Dockerfile`, and CI
  matrix.
- `.golangci.yml` enables `ineffassign`, `misspell`, `contextcheck`,
  `nolintlint`, and `gocritic`.
- `Makefile` gains `fmt`, `vet`, `cover`, `run`, and `help` targets.
- `README.md` license reference now correctly points to Apache-2.0.

### Removed

- `docs/api-spec.yaml` (deprecated duplicate; `docs/openapi.yaml` is
  authoritative).

### Security

- Fixes the header-spoofing bypass of per-IP rate limiting.
- Fixes unbounded memory growth in `IPRateLimiter` under IP-rotation DoS.
- Mitigates Slowloris via explicit server timeouts.
- Adds an optional TLS code path (operators previously had to front the
  node with a reverse proxy to get transport security).

## [0.0.0] - pre-release

Initial public development. No tagged release yet.
