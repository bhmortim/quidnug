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
| `quidnug::{TrustResult, TrustEdge, Title, IdentityRecord, Event, ...}` | Wire types. |
| `quidnug::{Error, Result}` | Structured error taxonomy + result alias. |

## Full API surface

Every endpoint on the reference node has a corresponding typed method
on `Client`. Methods mirror the Python `QuidnugClient` one-for-one.
The cross-language matrix (Rust method ↔ JS / Java / Swift / .NET
counterpart) lives at
[`docs/sdk-coverage.md`](../../docs/sdk-coverage.md).

| Area | Method | Endpoint |
| --- | --- | --- |
| Health / info | `health` | `GET /api/health` |
| Health / info | `info` | `GET /api/info` |
| Health / info | `nodes` | `GET /api/nodes` |
| Identity | `register_identity` | `POST /api/transactions/identity` |
| Identity | `get_identity` | `GET /api/identity/{quid}` |
| Identity | `query_identity_registry` | `GET /api/registry/identity` |
| Identity | `wait_for_identity` | polls `get_identity` |
| Identity | `wait_for_identities` | polls `get_identity` over a batch |
| Trust | `grant_trust` | `POST /api/transactions/trust` |
| Trust | `get_trust` | `GET /api/trust/{observer}/{target}` |
| Trust | `query_relational_trust` | `POST /api/trust/query` |
| Trust | `get_trust_edges` | `GET /api/trust/edges/{quid}` |
| Trust | `query_trust_registry` | `GET /api/registry/trust` |
| Title | `register_title` | `POST /api/transactions/title` |
| Title | `get_title` | `GET /api/title/{asset}` |
| Title | `query_title_registry` | `GET /api/registry/title` |
| Title | `wait_for_title` | polls `get_title` |
| Events | `emit_event` | `POST /api/events` |
| Events | `get_event_stream` | `GET /api/streams/{subject}` |
| Events | `get_stream_events` | `GET /api/streams/{subject}/events` |
| Storage | `ipfs_pin` | `POST /api/ipfs/pin` |
| Storage | `ipfs_get` | `GET /api/ipfs/{cid}` |
| Guardians | `submit_guardian_set_update` | `POST /api/guardian/set-update` |
| Guardians | `submit_recovery_init` | `POST /api/guardian/recovery/init` |
| Guardians | `submit_recovery_veto` | `POST /api/guardian/recovery/veto` |
| Guardians | `submit_recovery_commit` | `POST /api/guardian/recovery/commit` |
| Guardians | `submit_guardian_resignation` | `POST /api/guardian/resign` |
| Guardians | `get_guardian_set` | `GET /api/guardian/set/{quid}` |
| Guardians | `get_pending_recovery` | `GET /api/guardian/pending-recovery/{quid}` |
| Guardians | `get_guardian_resignations` | `GET /api/guardian/resignations/{quid}` |
| Gossip | `submit_domain_fingerprint` | `POST /api/domain-fingerprints` |
| Gossip | `get_latest_domain_fingerprint` | `GET /api/domain-fingerprints/{domain}/latest` |
| Gossip | `submit_anchor_gossip` | `POST /api/anchor-gossip` |
| Gossip | `push_anchor` | `POST /api/gossip/push-anchor` |
| Gossip | `push_fingerprint` | `POST /api/gossip/push-fingerprint` |
| Bootstrap | `submit_nonce_snapshot` | `POST /api/nonce-snapshots` |
| Bootstrap | `get_latest_nonce_snapshot` | `GET /api/nonce-snapshots/{domain}/latest` |
| Bootstrap | `bootstrap_status` | `GET /api/bootstrap/status` |
| Fork-block | `submit_fork_block` | `POST /api/fork-block` |
| Fork-block | `fork_block_status` | `GET /api/fork-block/status` |
| Blocks | `get_blocks` | `GET /api/blocks` |
| Blocks | `get_tentative_blocks` | `GET /api/blocks/tentative/{domain}` |
| Blocks | `get_pending_transactions` | `GET /api/transactions` |
| Domains | `list_domains` | `GET /api/domains` |
| Domains | `register_domain` | `POST /api/domains` |
| Domains | `ensure_domain` | idempotent `register_domain` wrapper |
| Domains | `get_node_domains` | `GET /api/node/domains` |
| Domains | `update_node_domains` | `POST /api/node/domains` |

Methods returning rich wire types (`IdentityRecord`, `Title`,
`TrustResult`, `TrustEdge`) decode the server envelope into the
corresponding struct from `quidnug::types`. Everything else returns
`serde_json::Value` so SDK consumers never silently drop fields the
node added in a forward-compatible release.

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
