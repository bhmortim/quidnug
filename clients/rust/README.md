# Quidnug Rust SDK

`quidnug` — the official Rust crate for [Quidnug](https://github.com/bhmortim/quidnug),
a decentralized protocol for relational, per-observer trust.

Covers the full v1.0 OpenAPI surface: identity, trust, titles, event
streams, IPFS-backed payloads, domains, blocks, registries, and node
metrics — alongside ECDSA P-256 signing and compact QDP-0010 Merkle
inclusion proofs.

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

    client.ensure_domain("contractors.home").await?;
    client.register_identity(&alice, "Alice", "contractors.home").await?;
    client.register_identity(&bob, "Bob", "contractors.home").await?;
    client.wait_for_identities(
        &[alice.id(), bob.id()],
        "contractors.home",
        std::time::Duration::from_secs(30),
        std::time::Duration::from_millis(500),
    ).await?;

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
| `quidnug::wire` | Typed v1.0 wire structs (`TrustTx`, `IdentityTx`, `TitleTx`, `EventTx`). |
| `quidnug::{TrustResult, TrustEdge, Title, IdentityRecord, Event, EventStream, OwnershipStake, ...}` | Read-model types. |
| `quidnug::{Error, Result}` | Structured error taxonomy + result alias. |

## `Client` — method index

Every node endpoint in the v1.0 OpenAPI spec has a corresponding typed
method. All methods are `async` and return `Result<T, Error>`.

| Area | Methods |
| --- | --- |
| Health / info / metrics | `health`, `info`, `nodes`, `metrics` |
| Identity | `register_identity`, `get_identity`, `wait_for_identity`, `wait_for_identities`, `create_quid`, `query_identity_registry` |
| Trust | `grant_trust`, `get_trust`, `query_relational_trust`, `get_trust_edges`, `query_trust_registry` |
| Title | `register_title`, `get_title`, `wait_for_title`, `query_title_registry` |
| Events / streams | `emit_event`, `get_event_stream`, `get_stream_events` |
| Storage | `ipfs_pin`, `ipfs_get` |
| Domains | `list_domains`, `register_domain`, `ensure_domain`, `query_domain`, `get_node_domains`, `update_node_domains`, `receive_domain_gossip` |
| Blocks / pending tx | `get_blocks`, `get_tentative_blocks`, `get_pending_transactions` |

### Notes on individual methods

- **`register_title`** — `OwnershipStake.percentage` may be supplied on
  either the fraction scale (sum ≈ 1.0) or the percent scale (sum ≈
  100.0); the client normalizes to fraction for the wire. Use
  `OwnershipStake::normalize_percentages` directly if you need the
  normalized stakes for another purpose.
- **`emit_event`** — exactly one of `payload` (inline JSON) or
  `payload_cid` (IPFS reference) must be supplied. If `sequence` is
  `None`, the client fetches the current stream's `latest_sequence`
  and uses `latest_sequence + 1`.
- **`wait_for_identity` / `wait_for_title`** — call after submitting an
  identity or title tx if you need to immediately reference the
  subject from a trust or event tx; the subject lives in the pending
  pool until the next block is sealed.
- **`metrics`** — returns the raw Prometheus text-exposition string
  served at `/metrics` (note: not `/api/metrics`).
- Paginated methods (`nodes`, `get_blocks`, `get_pending_transactions`,
  `query_*_registry`, `get_stream_events`) take `Option<u32>` for
  `limit` / `offset`; omitting both yields the server defaults.

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

| Variant | When |
| --- | --- |
| `Error::Validation(String)` | Local precondition failed before any network call, or the node returned a 4xx without a conflict code. |
| `Error::Conflict { code, message }` | Node logically rejected — `NONCE_REPLAY`, `GUARDIAN_SET_MISMATCH`, `QUORUM_NOT_MET`, `VETOED`, `INVALID_SIGNATURE`, `FORK_ALREADY_ACTIVE`, `DUPLICATE`, `ALREADY_EXISTS`, `INVALID_STATE_TRANSITION`. |
| `Error::Unavailable { code, message }` | HTTP 503 or feature-not-active (`FEATURE_NOT_ACTIVE`, `NOT_READY`, `BOOTSTRAPPING`). |
| `Error::Node { status, message }` | Transport, 5xx, or unexpected response shape. |
| `Error::Crypto(String)` | Signature / key derivation failed. |
| `Error::Transport`, `Error::Json`, `Error::Url`, `Error::UnexpectedResponse` | Lower-level wrappers around `reqwest`, `serde_json`, and `url`. |

## Retry policy

The client issues each request **exactly once**. There is no implicit
retry loop — applications that need exponential backoff for transient
5xx or 429 should wrap calls themselves, replaying only idempotent
GETs.

This mirrors the Go SDK and is intentional for non-idempotent writes
(`register_*`, `grant_trust`, `emit_event`, …): a transparent retry of
a write that *did* land would surface as `NONCE_REPLAY` on the second
attempt, but only after the side effect occurred. Repeat a write only
once you've confirmed the server's view via a GET.

## Features

| Feature | Purpose |
| --- | --- |
| `default = ["blocking"]` | Enable blocking HTTP (also requires `tokio`). |
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

For v1.0 transaction submission the SDK uses the typed wire structs in
`quidnug::wire` (`TrustTx`, `IdentityTx`, `TitleTx`, `EventTx`) so the
signable bytes match the server's Go-struct declaration order
byte-for-byte. The `canonical_bytes` path remains available for
ad-hoc / read-model use.

## Tests

```bash
cd clients/rust
cargo test
```

Integration tests use `wiremock` to stub node responses, so no
running node is required.

## License

Apache-2.0.
