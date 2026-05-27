# Quidnug Rust SDK

`quidnug` — the official Rust crate for [Quidnug](https://github.com/bhmortim/quidnug),
a decentralized protocol for relational, per-observer trust.

Covers the full protocol surface: identity, trust, titles, event
streams, anchors, guardian sets, recovery, cross-domain gossip,
K-of-K bootstrap, fork-block activation, compact Merkle inclusion
proofs, peer scoreboard, audit log, content moderation, privacy /
DSR / consent, network + operator discovery, and DNS attestation
(QDPs 0001–0023).

## Install

```toml
# Cargo.toml
[dependencies]
quidnug = "3"
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

## v2 endpoints

Methods that hit `/api/v2/<path>` on the node. All take a pre-signed
`&Value` body for `submit_*`/`push_*` calls; the SDK does not yet
build typed wire structs for these.

### Guardian sets + recovery (QDP-0002 / QDP-0006)

| Method | Endpoint |
| --- | --- |
| `submit_guardian_set_update(&Value)` | POST `guardian/set-update` |
| `submit_recovery_init(&Value)` | POST `guardian/recovery/init` |
| `submit_recovery_veto(&Value)` | POST `guardian/recovery/veto` |
| `submit_recovery_commit(&Value)` | POST `guardian/recovery/commit` |
| `submit_guardian_resignation(&Value)` | POST `guardian/resign` |
| `get_guardian_set(&str)` | GET `guardian/set/{quid}` |
| `get_pending_recovery(&str)` | GET `guardian/pending-recovery/{quid}` |
| `get_guardian_resignations(&str)` | GET `guardian/resignations/{quid}` |

### Cross-domain gossip (QDP-0003 / QDP-0005)

| Method | Endpoint |
| --- | --- |
| `submit_domain_fingerprint(&Value)` | POST `domain-fingerprints` |
| `get_latest_domain_fingerprint(&str)` | GET `domain-fingerprints/{domain}/latest` |
| `submit_anchor_gossip(&Value)` | POST `anchor-gossip` |
| `push_anchor(&Value)` | POST `gossip/push-anchor` |
| `push_fingerprint(&Value)` | POST `gossip/push-fingerprint` |

### K-of-K bootstrap (QDP-0008)

| Method | Endpoint |
| --- | --- |
| `submit_nonce_snapshot(&Value)` | POST `nonce-snapshots` |
| `get_latest_nonce_snapshot(&str)` | GET `nonce-snapshots/{domain}/latest` |
| `bootstrap_status()` | GET `bootstrap/status` |

### Fork-block (QDP-0009)

| Method | Endpoint |
| --- | --- |
| `submit_fork_block(&Value)` | POST `fork-block` |
| `fork_block_status()` | GET `fork-block/status` |

### Network + operator discovery (QDP-0014)

| Method | Endpoint |
| --- | --- |
| `get_discovery_domain(&str)` | GET `discovery/domain/{name}` |
| `get_discovery_node(&str)` | GET `discovery/node/{quid}` |
| `get_discovery_operator(&str)` | GET `discovery/operator/{quid}` |
| `discovery_quids(&[(&str,&str)])` | GET `discovery/quids` |
| `discovery_trusted_quids(&[(&str,&str)])` | GET `discovery/trusted-quids` |

### DNS attestation (QDP-0023)

| Method | Endpoint |
| --- | --- |
| `submit_dns_claim(&Value)` | POST `dns/claim` |
| `submit_dns_challenge(&Value)` | POST `dns/challenge` |
| `submit_dns_attestation(&Value)` | POST `dns/attestation` |
| `submit_dns_renewal(&Value)` | POST `dns/renewal` |
| `submit_dns_revocation(&Value)` | POST `dns/revocation` |
| `submit_dns_delegate(&Value)` | POST `dns/delegate` |
| `submit_dns_delegate_revocation(&Value)` | POST `dns/delegate-revocation` |
| `get_dns_attestations(&str)` | GET `dns/attestations/{domain}` |
| `get_dns_attestations_weighted(&str)` | GET `dns/attestations/{domain}/weighted` |
| `resolve_dns(&str, &str)` | GET `dns/resolve/{domain}/{recordType}` |

## v3 endpoints (`/api/<path>`)

### Peer scoreboard (QDP-0011)

| Method | Endpoint |
| --- | --- |
| `get_peers(Option<u32>, Option<u32>)` | GET `peers` |
| `get_peer(&str)` | GET `peers/{nodeQuid}` |

### Node + domain extras

| Method | Endpoint |
| --- | --- |
| `submit_node_advertisement(&Value)` | POST `node-advertisements` |
| `get_top_domains(Option<u32>, Option<u32>)` | GET `domains/top` |
| `submit_domain_gossip(&Value)` | POST `gossip/domains` |
| `get_tentative_blocks(&str)` | GET `blocks/tentative/{domain}` |
| `list_domains()` | GET `domains` |
| `nodes()` | GET `nodes` |
| `get_node_domains()` / `update_node_domains(&[String])` | GET/POST `node/domains` |

### Events + IPFS

| Method | Endpoint |
| --- | --- |
| `emit_event(&Value)` | POST `events` |
| `get_event_stream(&str)` | GET `streams/{subjectId}` |
| `get_stream_events(&str, Option<u32>, Option<u32>)` | GET `streams/{subjectId}/events` |
| `ipfs_pin(&[u8])` | POST `ipfs/pin` |
| `ipfs_get(&str)` | GET `ipfs/{cid}` |

### Registry queries + blocks/transactions

| Method | Endpoint |
| --- | --- |
| `query_trust_registry(&[(&str,&str)])` | GET `registry/trust` |
| `query_identity_registry(&[(&str,&str)])` | GET `registry/identity` |
| `query_title_registry(&[(&str,&str)])` | GET `registry/title` |
| `get_blocks(Option<u32>, Option<u32>)` | GET `blocks` |
| `get_pending_transactions(Option<u32>, Option<u32>)` | GET `transactions` |

### Title transactions

`Client::register_title(&Quid, TitleParams { .. })` builds the typed
`TitleTx` wire struct, derives the tx ID per `AddTitleTransaction`,
signs IEEE-1363, and POSTs to `transactions/title`.

### Content moderation (QDP-0015)

| Method | Endpoint |
| --- | --- |
| `submit_moderation_action(&Value)` | POST `moderation/actions` |
| `get_moderation_actions(&str, &str)` | GET `moderation/actions/{type}/{id}` |

### Operator audit log (QDP-0018)

| Method | Endpoint |
| --- | --- |
| `get_audit_head()` | GET `audit/head` |
| `get_audit_entries(Option<i64>, Option<u32>)` | GET `audit/entries` |
| `get_audit_entry(u64)` | GET `audit/entry/{sequence}` |

### Privacy / DSR / consent (QDP-0017)

| Method | Endpoint |
| --- | --- |
| `submit_dsr(&Value)` | POST `privacy/dsr` |
| `get_dsr_status(&str)` | GET `privacy/dsr/{requestTxId}` |
| `grant_consent(&Value)` | POST `privacy/consent/grants` |
| `withdraw_consent(&Value)` | POST `privacy/consent/withdraws` |
| `get_consent_history(Option<u32>, Option<u32>, Option<&str>)` | GET `privacy/consent/history` |
| `create_processing_restriction(&Value)` | POST `privacy/restrictions` |
| `get_processing_restrictions(&str)` | GET `privacy/restrictions/{subjectQuid}` |
| `submit_dsr_compliance(&Value)` | POST `privacy/compliance` |

## Tests

```bash
cd clients/rust
cargo test
```

Integration tests use `wiremock` to stub node responses, so no
running node is required.

## License

Apache-2.0.
