//! Rust cross-SDK interop vector producer.
//!
//! Emits the same 4 canonical test cases as the Python/Go producers.
//!
//!   cargo run --manifest-path tests/interop/produce-rs/Cargo.toml -- \
//!       > tests/interop/vectors-rust.json

use quidnug::{canonical_bytes, Quid};
use serde_json::{json, Value};
use std::{fs, path::PathBuf};

const KEYPAIR_FILE: &str = "tests/interop/.keypair-rust";

fn deterministic_quid() -> Quid {
    let p = PathBuf::from(KEYPAIR_FILE);
    if let Ok(hex) = fs::read_to_string(&p) {
        if let Ok(q) = Quid::from_private_hex(hex.trim()) {
            return q;
        }
    }
    let q = Quid::generate();
    let _ = fs::write(&p, q.private_key_hex().unwrap());
    q
}

fn make_case(name: &str, tx: Value, q: &Quid, exclude: &[&str]) -> Value {
    let bytes = canonical_bytes(&tx, exclude).unwrap();
    let sig = q.sign(&bytes).unwrap();
    json!({
        "name": name,
        "tx": tx,
        "excludeFields": exclude,
        "canonicalBytesHex": hex::encode(&bytes),
        "signatureHex": sig,
    })
}

fn main() {
    let q = deterministic_quid();
    let cases = vec![
        make_case(
            "trust-basic",
            json!({
                "type": "TRUST",
                "timestamp": 1_700_000_000_i64,
                "trustDomain": "interop.test",
                "signerQuid": q.id(),
                "truster": q.id(),
                "trustee": "abc0123456789def",
                "trustLevel": 0.9,
                "nonce": 1_i64,
            }),
            &q,
            &["signature", "txId"],
        ),
        make_case(
            "identity-with-attrs",
            json!({
                "type": "IDENTITY",
                "timestamp": 1_700_000_000_i64,
                "trustDomain": "interop.test",
                "signerQuid": q.id(),
                "definerQuid": q.id(),
                "subjectQuid": q.id(),
                "updateNonce": 1_i64,
                "schemaVersion": "1.0",
                "name": "Alice",
                "homeDomain": "interop.home",
                "attributes": { "role": "admin", "tier": 3 },
            }),
            &q,
            &["signature", "txId"],
        ),
        make_case(
            "event-unicode-nested",
            json!({
                "type": "EVENT",
                "timestamp": 1_700_000_000_i64,
                "trustDomain": "interop.test",
                "subjectId": q.id(),
                "subjectType": "QUID",
                "eventType": "NOTE",
                "sequence": 1_i64,
                "payload": {
                    "message": "hello 世界 🌍",
                    "nested": { "z": 1, "a": "x", "m": [1, 2, 3] },
                },
            }),
            &q,
            &["signature", "txId", "publicKey"],
        ),
        make_case(
            "numerical-edge",
            json!({
                "type": "EVENT",
                "timestamp": 1_700_000_000_i64,
                "trustDomain": "interop.test",
                "subjectId": q.id(),
                "subjectType": "QUID",
                "eventType": "MEASURE",
                "sequence": 42_i64,
                "payload": {
                    "count": 0,
                    "weight": 0.5,
                    "n64": 9_007_199_254_740_991_i64,
                },
            }),
            &q,
            &["signature", "txId", "publicKey"],
        ),
    ];

    let out = json!({
        "sdk": "rust",
        "version": "2.0.0",
        "quid": { "id": q.id(), "publicKeyHex": q.public_key_hex() },
        "cases": cases,
    });
    println!("{}", serde_json::to_string_pretty(&out).unwrap());
}
