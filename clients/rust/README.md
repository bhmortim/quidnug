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

## `Client` method reference

HTTP client for a running Quidnug node. Every node endpoint has a
corresponding typed `async` method on `Client`.

| Area | Methods |
| --- | --- |
| Health / info | `health`, `info`, `nodes` |
| Identity | `register_identity`, `get_identity`, `query_identity_registry` |
| Trust | `grant_trust`, `get_trust`, `query_relational_trust`, `get_trust_edges`, `query_trust_registry` |
| Title | `register_title`, `get_title`, `query_title_registry` |
| Events | `emit_event`, `get_event_stream`, `get_stream_events` |
| IPFS | `ipfs_pin`, `ipfs_get` |
| Guardians | `submit_guardian_set_update`, `submit_recovery_init`, `submit_recovery_veto`, `submit_recovery_commit`, `submit_guardian_resignation`, `get_guardian_set`, `get_pending_recovery`, `get_guardian_resignations` |
| Gossip | `submit_domain_fingerprint`, `get_latest_domain_fingerprint`, `submit_anchor_gossip`, `push_anchor`, `push_fingerprint` |
| Bootstrap | `submit_nonce_snapshot`, `get_latest_nonce_snapshot`, `bootstrap_status` |
| Fork-block | `submit_fork_block`, `fork_block_status` |
| Blocks | `get_blocks`, `get_tentative_blocks`, `get_pending_transactions` |
| Domains | `list_domains`, `register_domain`, `ensure_domain`, `get_node_domains`, `update_node_domains` |
| Commit-wait helpers | `wait_for_identity`, `wait_for_identities`, `wait_for_title` |

Method-by-method signatures (return-type elided as `Result<_>`):

### Health, info, nodes

| Method | HTTP | Notes |
| --- | --- | --- |
| `health()` | GET `/api/health` | Liveness probe. |
| `info()` | GET `/api/info` | Node identity, version, features, domains. |
| `nodes(limit, offset)` | GET `/api/nodes` | Paginated peer listing. |

### Identity

| Method | HTTP | Notes |
| --- | --- | --- |
| `register_identity(signer, name, home_domain)` | POST `/api/transactions/identity` | Submit a signed IDENTITY tx for the signer. |
| `get_identity(quid_id, domain)` | GET `/api/identity/{quid}` | Returns `None` on 404. |
| `query_identity_registry(domain, limit, offset)` | GET `/api/registry/identity` | Paginated dump. |

### Trust

| Method | HTTP | Notes |
| --- | --- | --- |
| `grant_trust(signer, TrustParams)` | POST `/api/transactions/trust` | Signer becomes the truster. |
| `get_trust(observer, target, domain, max_depth)` | GET `/api/trust/{obs}/{tgt}` | Path-style relational query. |
| `query_relational_trust(observer, target, domain, max_depth, include_unverified)` | POST `/api/trust/query` | JSON-body variant. |
| `get_trust_edges(quid_id)` | GET `/api/trust/edges/{quid}` | Direct outbound edges. |
| `query_trust_registry(domain, limit, offset)` | GET `/api/registry/trust` | Paginated edge listing. |

### Title

| Method | HTTP | Notes |
| --- | --- | --- |
| `register_title(signer, asset_id, owners, domain)` | POST `/api/transactions/title` | `owners` accepts 1.0 or 100.0 scale; normalized to fraction on the wire. |
| `get_title(asset_id, domain)` | GET `/api/title/{asset}` | Returns `None` on 404. |
| `query_title_registry(domain, limit, offset)` | GET `/api/registry/title` | Paginated dump. |

### Events

| Method | HTTP | Notes |
| --- | --- | --- |
| `emit_event(signer, subject_id, subject_type, event_type, domain, payload, payload_cid)` | POST `/api/events` | Exactly one of `payload` or `payload_cid` required. |
| `get_event_stream(subject_id, domain)` | GET `/api/streams/{subject}` | Returns `None` on 404. |
| `get_stream_events(subject_id, domain, limit, offset)` | GET `/api/streams/{subject}/events` | Returns `Vec<Event>`. |

### IPFS

| Method | HTTP | Notes |
| --- | --- | --- |
| `ipfs_pin(content)` | POST `/api/ipfs/pin` | Returns the CID as `String`. |
| `ipfs_get(cid)` | GET `/api/ipfs/{cid}` | Returns raw `Vec<u8>` (no envelope). |

### Guardians (QDP-0002 / QDP-0006)

| Method | HTTP | Notes |
| --- | --- | --- |
| `submit_guardian_set_update(update)` | POST `/api/guardian/set-update` | Install or rotate guardians. |
| `submit_recovery_init(init)` | POST `/api/guardian/recovery/init` | Start the M-of-N recovery delay. |
| `submit_recovery_veto(veto)` | POST `/api/guardian/recovery/veto` | Owner or guardian aborts recovery. |
| `submit_recovery_commit(commit)` | POST `/api/guardian/recovery/commit` | Finalize a delayed recovery. |
| `submit_guardian_resignation(r)` | POST `/api/guardian/resign` | Guardian leaves the set. |
| `get_guardian_set(quid)` | GET `/api/guardian/set/{quid}` | Returns `None` on 404. |
| `get_pending_recovery(quid)` | GET `/api/guardian/pending-recovery/{quid}` | Returns `None` on 404. |
| `get_guardian_resignations(quid)` | GET `/api/guardian/resignations/{quid}` | Returns `Vec<Value>`. |

### Gossip (QDP-0003 / QDP-0005)

| Method | HTTP | Notes |
| --- | --- | --- |
| `submit_domain_fingerprint(fp)` | POST `/api/domain-fingerprints` | Publish a signed fingerprint. |
| `get_latest_domain_fingerprint(domain)` | GET `/api/domain-fingerprints/{domain}/latest` | Returns `None` on 404. |
| `submit_anchor_gossip(msg)` | POST `/api/anchor-gossip` | Idempotent (duplicates flagged in response). |
| `push_anchor(msg)` | POST `/api/gossip/push-anchor` | Push-gossip variant. |
| `push_fingerprint(fp)` | POST `/api/gossip/push-fingerprint` | Push-gossip variant. |

### Bootstrap (QDP-0008)

| Method | HTTP | Notes |
| --- | --- | --- |
| `submit_nonce_snapshot(s)` | POST `/api/nonce-snapshots` | Publish a K-of-K bootstrap snapshot. |
| `get_latest_nonce_snapshot(domain)` | GET `/api/nonce-snapshots/{domain}/latest` | Returns `None` on 404. |
| `bootstrap_status()` | GET `/api/bootstrap/status` | Node bootstrap-phase status. |

### Fork-block (QDP-0009)

| Method | HTTP | Notes |
| --- | --- | --- |
| `submit_fork_block(fb)` | POST `/api/fork-block` | Submit a signed fork-activation block. |
| `fork_block_status()` | GET `/api/fork-block/status` | Activation status across features. |

### Blocks + pending transactions

| Method | HTTP | Notes |
| --- | --- | --- |
| `get_blocks(limit, offset)` | GET `/api/blocks` | Paginated block listing. |
| `get_tentative_blocks(domain)` | GET `/api/blocks/tentative/{domain}` | Tentative-block tip per domain. |
| `get_pending_transactions(domain, limit, offset)` | GET `/api/transactions` | Mempool listing. |

### Domains

| Method | HTTP | Notes |
| --- | --- | --- |
| `list_domains()` | GET `/api/domains` | All registered trust domains. |
| `register_domain(domain)` | POST `/api/domains` | Errors if already exists. |
| `ensure_domain(domain)` | POST `/api/domains` | Idempotent wrapper around `register_domain`. |
| `get_node_domains()` | GET `/api/node/domains` | Domains this node currently manages. |
| `update_node_domains(domains)` | POST `/api/node/domains` | Replace the node's managed-domain list. |

### Commit-wait helpers

Just-submitted identity / title transactions live in the pending
pool until the next block is sealed. Code that immediately emits
events or references the new subject must wait for commit first;
these helpers poll until the record is visible in the committed
registry.

| Method | Notes |
| --- | --- |
| `wait_for_identity(quid_id, domain, timeout, poll_interval)` | Poll `get_identity` until visible or timeout. |
| `wait_for_identities(quid_ids, domain, timeout, poll_interval)` | Batch wait sharing one deadline. |
| `wait_for_title(asset_id, domain, timeout, poll_interval)` | Poll `get_title` until visible or timeout. |

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
