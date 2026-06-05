# Quidnug Rust SDK

`quidnug` — the official Rust crate for [Quidnug](https://github.com/bhmortim/quidnug),
a decentralized protocol for relational, per-observer trust.

Implements the v1.0-conformant identity / trust / title surface plus
QDP-0010 compact Merkle inclusion-proof verification. The crate
ships strongly-typed wire structs (`GuardianSet`, `DomainFingerprint`,
`NonceSnapshot`, `ForkBlock`, …) for the full v2 protocol so callers
can construct/inspect those payloads today; client methods that
submit/query them are tracked under [Coverage](#coverage) below.
The Go (`pkg/client`) and Python (`clients/python`) SDKs remain the
reference implementations for the full v2 surface.

## Install

```toml
# Cargo.toml
[dependencies]
quidnug = "2"
tokio = { version = "1", features = ["full"] }
```

Requires Rust 1.74+.

## Thirty-second example

```rust
use quidnug::{Client, Quid, TrustParams};

#[tokio::main]
async fn main() -> Result<(), quidnug::Error> {
    let client = Client::new("http://localhost:8080")?;

    let alice = Quid::generate();
    let bob = Quid::generate();

    client.register_identity(&alice, "Alice", "contractors.home").await?;
    client.register_identity(&bob, "Bob", "contractors.home").await?;

    client.grant_trust(&alice, TrustParams {
        trustee: bob.id(),
        level: 0.9,
        domain: "contractors.home",
        nonce: 1,
    }).await?;

    let tr = client.get_trust(alice.id(), bob.id(), "contractors.home", 5).await?;
    println!("{:.3} via {:?}", tr.trust_level, tr.path);
    Ok(())
}
```

Runnable examples live in `examples/`:

| File | Shows |
| --- | --- |
| `quickstart.rs` | End-to-end two-party trust against a local node. |
| `merkle_proof.rs` | Offline QDP-0010 proof verification. |

## What's in the crate

| Module | Contents |
| --- | --- |
| `quidnug::Client` | Async HTTP client (`reqwest` under the hood). |
| `quidnug::Quid` | ECDSA P-256 keypair + signing + verification. |
| `quidnug::canonical_bytes` | Canonical signable bytes (matches Go / Python byte-for-byte). |
| `quidnug::verify_inclusion_proof` | QDP-0010 Merkle proof verifier. |
| `quidnug::MerkleProofFrame` | Proof frame (`hash`, `side`). |
| `quidnug::{TrustResult, TrustEdge, Title, IdentityRecord, Event, GuardianSet, DomainFingerprint, NonceSnapshot, ForkBlock, ...}` | Wire types for v1 + v2 features. |
| `quidnug::{Error, Result}` | Structured error taxonomy + result alias. |

## Coverage

The async `Client` currently exposes the v1 transaction surface plus
commit-wait and domain-registration helpers:

| Area | Methods |
| --- | --- |
| Health | `health`, `info` |
| Identity | `register_identity`, `get_identity` |
| Trust | `grant_trust`, `get_trust`, `get_trust_edges` |
| Title | `get_title` |
| Domains | `register_domain`, `ensure_domain` |
| Commit-wait | `wait_for_identity`, `wait_for_identities`, `wait_for_title` |

The v2 surface (events, discovery, guardians, gossip, bootstrap,
fork-block, blocks) is **not yet wrapped** in this crate. Either:

- Call the node directly via your own `reqwest::Client` against
  the documented HTTP endpoints (see [`docs/api/`](../../docs/api)), or
- Use the Go (`pkg/client`), Python (`clients/python`), Java
  (`clients/java`), or .NET (`clients/dotnet`) SDKs which cover
  more of the v2 surface today.

Tracked in the project roadmap; contributions welcome.

## Error taxonomy

```rust
match err {
    Error::Validation(m) => eprintln!("bad input: {m}"),
    Error::Conflict { code, .. } => eprintln!("node rejected: {code}"),
    Error::Unavailable { .. } => eprintln!("retry later"),
    Error::Node { status, .. } => eprintln!("HTTP {status}"),
    Error::Crypto(m) => eprintln!("crypto: {m}"),
    other => eprintln!("{other:?}"),
}
```

## Features

| Feature | Purpose |
| --- | --- |
| `default = ["blocking"]` | Enable blocking HTTP (also requires `tokio`). Leave in if you don't know you don't need it. |
| `rustls` | Swap native TLS for rustls (pure Rust, better for static binaries). |

Enable with `cargo build --no-default-features --features rustls` etc.

## Canonicalization

```rust
use quidnug::canonical_bytes;

let tx = serde_json::json!({
    "type": "TRUST", "truster": "a", "trustee": "b",
    "trustLevel": 0.9, "nonce": 1, "trustDomain": "x",
    "timestamp": 1_700_000_000_i64,
});
let bytes = canonical_bytes(&tx, &["signature", "txId"])?;
```

See `schemas/types/canonicalization.md` in the repo root for the
cross-language specification.

## Tests

```bash
cd clients/rust
cargo test
```

Integration tests use `wiremock` to stub node responses, so no
running node is required.

## License

Apache-2.0.
