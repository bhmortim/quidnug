# Contributing to Quidnug

Thanks for your interest in improving Quidnug. This document explains how to
propose changes, what we expect in a pull request, and how the project is
licensed.

## Licensing of contributions

Quidnug is licensed under the **Apache License, Version 2.0** (see `LICENSE`).

By submitting a contribution, you agree that your contribution is licensed
under the Apache License 2.0. The Apache 2.0 license includes an explicit
patent grant; no separate Contributor License Agreement (CLA) is required
for most contributions. We may add a DCO (Developer Certificate of Origin)
sign-off requirement later; if that happens, we will update this file.

## Ways to contribute

- **Bug reports** — open a GitHub issue with a minimal reproduction.
- **Security issues** — do not open a public issue. Follow `SECURITY.md`.
- **Feature proposals** — open an issue first to discuss the design before
  writing code. For anything that changes the protocol (block format,
  signature domains, transaction types, consensus rules), an issue is
  required.
- **Pull requests** — small, focused changes are the most useful. See below.

## Development workflow

Prerequisites:

- Go 1.23 or later (matching `go.mod`).
- Node.js 18 or later (for the JavaScript client).
- `make`, `git`, and optionally `docker`.

Clone and build:

```bash
git clone https://github.com/bhmortim/quidnug.git
cd quidnug
make build
make test
```

Before opening a pull request:

```bash
make fmt      # gofmt
make vet      # go vet
make lint     # golangci-lint (install: https://golangci-lint.run)
make test     # unit + race
```

For the JS client:

```bash
cd clients/js
npm test
```

## Pull request expectations

1. **One logical change per PR.** Refactors and behavior changes in separate
   PRs whenever possible.
2. **Tests.** New behavior ships with tests. Bug fixes include a regression
   test. We require `go test -race` to pass.
3. **Docs.** If you change user-visible behavior, update `README.md`,
   `config.example.yaml`, or the relevant file under `docs/`.
4. **No drive-by formatting** inside a substantive change. Run `gofmt`, then
   keep the PR focused.
5. **Commit messages.** Write imperative, short subject lines
   (`fix rate limiter eviction race`, not `fixes rate limiter`). Explain the
   *why* in the body if it isn't obvious from the diff.
6. **Security-sensitive changes** (`internal/core/auth.go`, `crypto.go`,
   `middleware.go`, anything touching block or transaction validation) must
   cite the threat model they address and include targeted tests. Expect a
   slower review.
7. **Public API.** Exported Go functions and types are considered API.
   Breaking changes must be called out in the PR description and will
   require a minor-version bump.

## Review and merge

- Two reviews required for protocol changes; one for everything else.
- CI must be green (lint, tests with race, security scans, Docker build).
- We prefer squash merges with a tidy commit message.

## Code style

- Go: standard `gofmt`. Exported identifiers have short doc comments starting
  with the identifier name.
- JavaScript: the existing style in `clients/js/` (no build step, ESM,
  minimal dependencies). Don't introduce a bundler.
- Keep comments sparse and load-bearing. Prefer self-explanatory code; use
  comments for non-obvious *why*, not *what*.

## Protocol-level changes

Any change to signature domains, block/transaction shape, consensus rules,
or network message formats must include:

- A short design note in `docs/` or a linked issue.
- A migration or compatibility story (can old nodes still talk to new
  nodes? if not, what's the upgrade path?).
- Test coverage that exercises the old and new behavior.

## Releases

See `CHANGELOG.md`. Until 1.0, the `main` branch is the release; after 1.0,
we will tag releases following Semantic Versioning.
