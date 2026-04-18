# Changelog

All notable changes to Quidnug are documented here. The format is based on
[Keep a Changelog](https://keepachangelog.com/en/1.1.0/) and the project
follows [Semantic Versioning](https://semver.org/spec/v2.0.0.html) from 1.0
onward.

## [Unreleased]

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
