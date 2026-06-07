# Quidnug Rust SDK

`quidnug` — the official Rust crate for [Quidnug](https://github.com/bhmortim/quidnug),
a decentralized protocol for relational, per-observer trust.

Covers the full protocol surface: identity, trust, titles, event
streams, anchors, guardian sets, recovery, cross-domain gossip,
K-of-K bootstrap, fork-block activation, compact Merkle inclusion
proofs (QDPs 0001–0010).

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
| `quidnug::{TrustResult, TrustEdge, Title, IdentityRecord, Event, GuardianSet, DomainFingerprint, NonceSnapshot, ForkBlock, ...}` | Wire types. |
| `quidnug::{Error, Result}` | Structured error taxonomy + result alias. |

### `Client` HTTP surface

Every node endpoint has a corresponding method. Methods that
require a signed envelope (guardian set updates, recovery, gossip,
nonce snapshots, fork-block proposals) take a pre-signed
`serde_json::Value` — the caller assembles + signs, the SDK handles
transport and envelope/error translation.

| Area | Methods |
| --- | --- |
| Health / info | `health`, `info`, `nodes` |
| Identity | `register_identity`, `get_identity`, `query_identity_registry` |
| Trust | `grant_trust`, `get_trust`, `query_relational_trust`, `get_trust_edges`, `query_trust_registry` |
| Title | `get_title`, `query_title_registry` |
| Events | `emit_event`, `get_event_stream`, `get_stream_events` |
| Storage | `ipfs_pin`, `ipfs_get` |
| Guardians (QDP-0002 / 0006) | `submit_guardian_set_update`, `submit_recovery_init`, `submit_recovery_veto`, `submit_recovery_commit`, `submit_guardian_resignation`, `get_guardian_set`, `get_pending_recovery`, `get_guardian_resignations` |
| Gossip (QDP-0003 / 0005) | `submit_domain_fingerprint`, `get_latest_domain_fingerprint`, `submit_anchor_gossip`, `push_anchor`, `push_fingerprint` |
| Bootstrap (QDP-0008) | `submit_nonce_snapshot`, `get_latest_nonce_snapshot`, `bootstrap_status` |
| Fork-block (QDP-0009) | `submit_fork_block`, `fork_block_status` |
| Blocks | `get_blocks`, `get_tentative_blocks`, `get_pending_transactions` |
| Domains | `list_domains`, `register_domain`, `ensure_domain`, `get_node_domains`, `update_node_domains` |
| Commit-wait helpers | `wait_for_identity`, `wait_for_identities`, `wait_for_title` |

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
