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

## Client API

The Rust client is at full parity with the Python reference. All
methods are `async` and return `Result<T, quidnug::Error>`.

### Health, info, nodes

| Method | Endpoint |
| --- | --- |
| `health()` | `GET /api/health` |
| `info()` | `GET /api/info` |
| `nodes(limit, offset)` | `GET /api/nodes` |

### Identity

| Method | Endpoint |
| --- | --- |
| `register_identity(signer, name, home_domain)` | `POST /api/transactions/identity` |
| `get_identity(quid_id, domain)` | `GET /api/identity/{quid}` |
| `query_identity_registry(limit, offset, quid_id)` | `GET /api/registry/identity` |
| `wait_for_identity(quid_id, domain, timeout, poll)` | polls `get_identity` |
| `wait_for_identities(quid_ids, domain, timeout, poll)` | polls `get_identity` |

### Trust

| Method | Endpoint |
| --- | --- |
| `grant_trust(signer, params)` | `POST /api/transactions/trust` |
| `get_trust(observer, target, domain, max_depth)` | `GET /api/trust/{observer}/{target}` |
| `query_relational_trust(observer, target, domain, max_depth)` | `POST /api/trust/query` |
| `get_trust_edges(quid_id)` | `GET /api/trust/edges/{quid}` |
| `query_trust_registry(limit, offset, truster, trustee)` | `GET /api/registry/trust` |

### Title / ownership

| Method | Endpoint |
| --- | --- |
| `register_title(signer, params)` | `POST /api/transactions/title` |
| `get_title(asset_id, domain)` | `GET /api/title/{asset}` |
| `query_title_registry(limit, offset, asset_id, owner_id)` | `GET /api/registry/title` |
| `wait_for_title(asset_id, domain, timeout, poll)` | polls `get_title` |

### Events

| Method | Endpoint |
| --- | --- |
| `emit_event(signer, params)` | `POST /api/events` |
| `get_event_stream(subject_id, domain)` | `GET /api/streams/{subject}` |
| `get_stream_events(subject_id, domain, limit, offset)` | `GET /api/streams/{subject}/events` |

### IPFS

| Method | Endpoint |
| --- | --- |
| `ipfs_pin(content)` | `POST /api/ipfs/pin` |
| `ipfs_get(cid)` | `GET /api/ipfs/{cid}` |

### Guardians + recovery (QDP-0002 / QDP-0006)

| Method | Endpoint |
| --- | --- |
| `submit_guardian_set_update(update)` | `POST /api/guardian/set-update` |
| `submit_recovery_init(init)` | `POST /api/guardian/recovery/init` |
| `submit_recovery_veto(veto)` | `POST /api/guardian/recovery/veto` |
| `submit_recovery_commit(commit)` | `POST /api/guardian/recovery/commit` |
| `submit_guardian_resignation(resignation)` | `POST /api/guardian/resign` |
| `get_guardian_set(quid_id)` | `GET /api/guardian/set/{quid}` |
| `get_pending_recovery(quid_id)` | `GET /api/guardian/pending-recovery/{quid}` |
| `get_guardian_resignations(quid_id)` | `GET /api/guardian/resignations/{quid}` |

### Cross-domain gossip + fingerprints (QDP-0003 / QDP-0005)

| Method | Endpoint |
| --- | --- |
| `submit_domain_fingerprint(fp)` | `POST /api/domain-fingerprints` |
| `get_latest_domain_fingerprint(domain)` | `GET /api/domain-fingerprints/{domain}/latest` |
| `submit_anchor_gossip(message)` | `POST /api/anchor-gossip` |
| `push_anchor(message)` | `POST /api/gossip/push-anchor` |
| `push_fingerprint(fp)` | `POST /api/gossip/push-fingerprint` |

### Bootstrap (QDP-0008)

| Method | Endpoint |
| --- | --- |
| `submit_nonce_snapshot(snapshot)` | `POST /api/nonce-snapshots` |
| `get_latest_nonce_snapshot(domain)` | `GET /api/nonce-snapshots/{domain}/latest` |
| `bootstrap_status()` | `GET /api/bootstrap/status` |

### Fork-block (QDP-0009)

| Method | Endpoint |
| --- | --- |
| `submit_fork_block(fb)` | `POST /api/fork-block` |
| `fork_block_status()` | `GET /api/fork-block/status` |

### Blocks + transactions + domains

| Method | Endpoint |
| --- | --- |
| `get_blocks(limit, offset)` | `GET /api/blocks` |
| `get_tentative_blocks(domain)` | `GET /api/blocks/tentative/{domain}` |
| `get_pending_transactions(limit, offset)` | `GET /api/transactions` |
| `list_domains()` | `GET /api/domains` |
| `register_domain(domain)` | `POST /api/domains` |
| `ensure_domain(domain)` | idempotent register |
| `get_node_domains()` | `GET /api/node/domains` |
| `update_node_domains(domains)` | `POST /api/node/domains` |
| `query_domain(domain, type, param)` | `GET /api/domains/{name}/query` |

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
