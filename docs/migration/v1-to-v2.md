# Migrating from Quidnug SDK v1 to v2

Quidnug SDK v2 expands the protocol surface from *identity / trust /
title / events* to the full QDPs 0001–0010 suite (anchors, guardians,
recovery, cross-domain gossip, bootstrap, fork-block, Merkle proofs).

This guide walks through what changed in each language and how to
port code.

## TL;DR

- **JS / TypeScript users**: v2 adds a *mixin* that layers new
  capabilities onto your existing v1 client. Your v1 code still
  works unchanged.
- **Python users**: v1 didn't exist as a published package. v2 is
  the first release; no migration needed.
- **Go users**: the new `pkg/client/` is external-consumer-safe
  (importable from outside the module). If you were reaching into
  `internal/core/`, switch.
- **No breaking wire changes.** A v2 SDK talking to a v1 node falls
  back gracefully; a v1 SDK talking to a v2 node works unchanged.

## JavaScript / TypeScript

### v1 was

```js
import { QuidnugClient } from "@quidnug/client";

const c = new QuidnugClient({ defaultNode: "http://localhost:8080" });
const quid = await c.generateQuid();
await c.submitTransaction({ type: "TRUST", truster, trustee, level });
```

### v2 adds a mixin — v1 API is unchanged

```js
import QuidnugClient from "@quidnug/client";
import "@quidnug/client/v2";   // side-effect import installs v2 methods

const c = new QuidnugClient({ defaultNode: "http://localhost:8080" });

// All v1 methods still work
const quid = await c.generateQuid();

// New v2 capabilities on the same client
const gs = await c.getGuardianSet(quid.id);
const fp = await c.getLatestDomainFingerprint("my.domain");
const ok = await QuidnugClient.verifyInclusionProof(txBytes, frames, rootHex);
```

The `./v2` side-effect import attaches new methods to the
`QuidnugClient` prototype. **Your v1 imports never changed.**
Upgrade strategy: bump `@quidnug/client` to `^2.0.0` and add the
side-effect import only in modules that use guardian / gossip /
merkle features.

### TypeScript module augmentation

The v2 entry also augments the `.d.ts` so TypeScript sees the new
methods after you import the side-effect module. If you don't
import `@quidnug/client/v2`, none of the new types appear — so v1
code stays strictly v1 in the type system.

### Canonical bytes: cross-SDK fix

v2 JS `QuidnugClient.canonicalBytes(obj, excludeFields)` emits
**raw UTF-8** for non-ASCII characters — matching Go / Python /
Rust / .NET / Java byte-for-byte. v1 used implicit UTF-16 via
`JSON.stringify`, which already behaved correctly. No action needed.

## Python

Python v2.0.0 is the first public release. There is no v1 to
migrate from.

If you were consuming the old `quidnug` package from an internal
mirror, the main breaking changes are:

- Namespace: `from quidnug import QuidnugClient, Quid` (not
  `from quidnug.client import ...`).
- Method names: `client.grant_trust(signer, trustee=, level=, ...)`.
- Async: sync-only in 2.0.0; async client planned for 2.1.0.

## Go

### v1: reach-into-internal pattern (discouraged)

```go
import "github.com/quidnug/quidnug/internal/core"

cfg := config.Load()
node, _ := core.NewQuidnugNode(cfg)
node.RegisterTransaction(/* ... */)
```

This only worked if your code lived inside the same Go module.
External consumers couldn't import `internal/core`.

### v2: `pkg/client` is the public surface

```go
import "github.com/quidnug/quidnug/pkg/client"

c, _ := client.New("http://localhost:8080")
q, _ := client.GenerateQuid()
c.GrantTrust(ctx, q, client.TrustParams{
    Trustee: "bob", Level: 0.9, Domain: "demo.home",
})
```

Migrate by:

1. Replace `github.com/quidnug/quidnug/internal/core` imports with
   `github.com/quidnug/quidnug/pkg/client`.
2. Replace direct struct access with method calls (`c.GrantTrust`,
   `c.GetTrust`, `c.RegisterIdentity`, etc.).
3. Drop in-process state — the public client is HTTP-only.

In-process use of `internal/core/` is still supported for node
operators — only external SDK consumers are encouraged to move to
`pkg/client/`.

## Rust

Rust didn't have a v1. 2.0.0 is the first release via
`crates.io/quidnug`.

## Java / Kotlin, C# / .NET, Swift

First public releases. No migration.

## CLI (`quidnug-cli`)

The CLI is new in v2. The node binary (`quidnug`) remains compatible
across v1 → v2 — same config file, same ports, same on-disk data
layout. You can keep using your v1 `quidnug` binary and only upgrade
when you need the new QDP features (gossip rate-limits, fork-block
activation tracking).

## Breaking changes on the node

**Only one** field-level breaking change landed between v1 and v2,
and it's guarded by a fork-block (QDP-0009):

- `Block.transactions_root` (QDP-0010). Nodes that activate the
  `require_tx_tree_root` feature reject blocks without this field.
  Inactive nodes continue to accept both forms. Activation is
  explicit per-domain — existing v1 domains stay on the old format
  until their operators coordinate a fork-block.

Every other v2 addition is **additive**. A v1 node rejects v2
transactions it doesn't recognize (guardian set updates, fork-block
submissions, etc.); a v2 node accepts every v1 transaction
unchanged.

## Signature compatibility

Critical point: **canonical-bytes output is identical across v1 and
v2 SDKs.** A signature produced by v1 verifies on a v2 node and
vice-versa. The interop tests at `tests/interop/` lock this
property across languages; the lock also covers v1 → v2 signatures.

## Upgrade checklist

| Action | Required? | Notes |
| --- | --- | --- |
| Bump SDK dependency to ^2.0.0 | yes | pin the major version |
| Rewrite HTTP endpoint paths | no | v1 paths all still work |
| Re-generate wallet quids | no | ECDSA P-256 key format unchanged |
| Update node binary | optional | v1 node accepts v2 SDK traffic |
| Activate QDP-0010 fork | optional | only if your domain wants tx-root enforcement |
| Enable guardian sets | optional | QDP-0002 is opt-in per quid |

## If you hit a problem

- Check [`docs/faq.md`](../faq.md) for common error codes.
- Report regressions at https://github.com/bhmortim/quidnug/issues —
  a minimal repro helps.
- v1 SDKs remain supported through 2026-10 for bug fixes. New
  features only land in v2.

## License

Apache-2.0.
