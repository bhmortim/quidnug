# Changelog

All notable changes to Quidnug are documented here. The format is based on
[Keep a Changelog](https://keepachangelog.com/en/1.1.0/) and the project
follows [Semantic Versioning](https://semver.org/spec/v2.0.0.html) from 1.0
onward.

## [Unreleased]

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
