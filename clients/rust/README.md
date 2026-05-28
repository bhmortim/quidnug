# Quidnug Rust SDK

`quidnug` — the official Rust crate for [Quidnug](https://github.com/bhmortim/quidnug),
a decentralized protocol for relational, per-observer trust.

Covers the full v2 protocol surface (QDPs 0001–0010): identity,
trust, titles, event streams, anchors, guardian sets + recovery,
cross-domain gossip, K-of-K bootstrap, fork-block activation, and
compact Merkle inclusion proof verification.

Transaction submission for `TRUST` and `IDENTITY` is fully typed
and signs v1.0-conformant bytes (cross-verifies against every
other SDK via the shared test vectors at
`docs/test-vectors/v1.0/`). `TITLE` and `EVENT` submission, plus
the guardian / gossip / bootstrap / fork-block POSTs, currently
take a caller-built `serde_json::Value` envelope — typed helpers
for those are tracked as a follow-up.

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
| `quidnug::{TrustResult, TrustEdge, Title, IdentityRecord, Event, ...}` | Wire types. |
| `quidnug::{Error, Result}` | Structured error taxonomy + result alias. |

### `Client` method index

| Area | Methods |
| --- | --- |
| Health | `health`, `info`, `nodes`, `blocks` |
| Identity | `register_identity`, `get_identity` |
| Trust | `grant_trust`, `get_trust`, `query_relational_trust`, `get_trust_edges` |
| Title | `get_title` |
| Events | `get_event_stream`, `get_stream_events` |
| Domains | `register_domain`, `ensure_domain` |
| Commit-wait helpers | `wait_for_identity`, `wait_for_identities`, `wait_for_title` |
| Guardians (QDP-0002 / 0006) | `submit_guardian_set_update`, `submit_recovery_init/veto/commit`, `submit_guardian_resignation`, `get_guardian_set` |
| Gossip (QDP-0003 / 0005) | `submit_domain_fingerprint`, `get_latest_domain_fingerprint`, `submit_anchor_gossip` |
| Bootstrap (QDP-0008) | `submit_nonce_snapshot`, `get_latest_nonce_snapshot`, `bootstrap_status` |
| Fork-block (QDP-0009) | `submit_fork_block`, `fork_block_status` |
| Registry | `registry_trust` |
| IPFS | `ipfs_pin`, `ipfs_get` |

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
