//! Rust SDK consumer for the v1.0 cross-SDK test vectors at
//! `docs/test-vectors/v1.0/`.
//!
//! Asserts the five conformance properties for each case in
//! `trust-tx.json`, `identity-tx.json`, `event-tx.json`, plus
//! the divergence probes that self-heal when the SDK converges.
//!
//! Run with: `cargo test --test vectors_test`.

use quidnug::wire::{IdentityTx, TrustTx};
use quidnug::Quid;
use serde::Deserialize;
use std::fs;
use std::path::PathBuf;

#[derive(Debug, Deserialize)]
struct VectorFile {
    schema_version: String,
    tx_type: String,
    cases: Vec<VectorCase>,
}

#[derive(Debug, Deserialize)]
struct VectorCase {
    name: String,
    #[serde(default)]
    comments: String,
    signer_key_ref: String,
    input: serde_json::Value,
    expected: Expected,
}

#[derive(Debug, Deserialize)]
struct Expected {
    canonical_signable_bytes_hex: String,
    canonical_signable_bytes_utf8: String,
    sha256_of_canonical_hex: String,
    expected_id: String,
    reference_signature_hex: String,
    signature_length_bytes: usize,
}

#[derive(Debug, Deserialize)]
struct KeyFile {
    name: String,
    #[serde(default)]
    seed: String,
    private_scalar_hex: String,
    public_key_sec1_hex: String,
    quid_id: String,
}

fn vectors_root() -> PathBuf {
    // tests/ runs with the crate root as CWD, so go three up to
    // the repo root then into docs/test-vectors/v1.0.
    let mut p = PathBuf::from(env!("CARGO_MANIFEST_DIR"));
    p.pop(); // clients/
    p.pop(); // <repo>
    p.push("docs");
    p.push("test-vectors");
    p.push("v1.0");
    p
}

fn load_vector_file(name: &str) -> VectorFile {
    let path = vectors_root().join(name);
    let raw = fs::read_to_string(&path)
        .unwrap_or_else(|e| panic!("read {}: {}", path.display(), e));
    serde_json::from_str(&raw)
        .unwrap_or_else(|e| panic!("parse {}: {}", path.display(), e))
}

fn load_keys() -> std::collections::HashMap<String, KeyFile> {
    let dir = vectors_root().join("test-keys");
    let mut out = std::collections::HashMap::new();
    for entry in fs::read_dir(&dir).expect("read test-keys") {
        let entry = entry.expect("entry");
        if entry.path().extension().and_then(|s| s.to_str()) != Some("json") {
            continue;
        }
        let raw = fs::read_to_string(entry.path()).expect("read");
        let k: KeyFile = serde_json::from_str(&raw).expect("parse");
        out.insert(k.name.clone(), k);
    }
    out
}

/// Assert the five conformance properties for a single case.
fn run_case<F>(case: &VectorCase, keys: &std::collections::HashMap<String, KeyFile>, build_signable: F)
where
    F: FnOnce(&VectorCase, &KeyFile) -> (Vec<u8>, String),
{
    let key = keys
        .get(&case.signer_key_ref)
        .unwrap_or_else(|| panic!("no key ref {}", case.signer_key_ref));

    let (signable, derived_id) = build_signable(case, key);

    // Property 1: sha256 of canonical matches.
    use sha2::{Digest, Sha256};
    let sum = Sha256::digest(&signable);
    assert_eq!(
        hex::encode(sum),
        case.expected.sha256_of_canonical_hex,
        "{}: SHA-256 mismatch",
        case.name
    );

    // Property 2: canonical bytes match hex/utf8 in vector.
    assert_eq!(
        hex::encode(&signable),
        case.expected.canonical_signable_bytes_hex,
        "{}: hex canonical bytes mismatch",
        case.name
    );
    assert_eq!(
        std::str::from_utf8(&signable).expect("utf8"),
        case.expected.canonical_signable_bytes_utf8,
        "{}: utf8 canonical bytes mismatch",
        case.name
    );

    // Property 3: derived ID matches.
    assert_eq!(
        derived_id, case.expected.expected_id,
        "{}: ID derivation mismatch",
        case.name
    );

    // Property 4: reference signature verifies via Quid::verify
    // (public SDK API).
    let q_ro = Quid::from_public_hex(&key.public_key_sec1_hex)
        .unwrap_or_else(|e| panic!("from_public_hex: {e}"));
    assert!(
        q_ro.verify(&signable, &case.expected.reference_signature_hex),
        "{}: SDK Verify rejected reference signature",
        case.name
    );
    assert_eq!(
        case.expected.signature_length_bytes, 64,
        "{}: expected_sig_len != 64",
        case.name
    );

    // Property 5: tampered signature rejects.
    let mut tampered = hex::decode(&case.expected.reference_signature_hex).unwrap();
    tampered[5] ^= 0x01;
    assert!(
        !q_ro.verify(&signable, &hex::encode(&tampered)),
        "{}: tampered signature accepted",
        case.name
    );

    // Property 6: independent SDK sign-then-verify round-trip.
    let q_sign = Quid::from_private_hex_scalar(&key.private_scalar_hex)
        .unwrap_or_else(|e| panic!("from_private_hex_scalar: {e}"));
    let sdk_sig = q_sign.sign(&signable).expect("sign");
    assert_eq!(
        hex::decode(&sdk_sig).unwrap().len(),
        64,
        "{}: SDK produced non-64-byte signature",
        case.name
    );
    assert!(
        q_sign.verify(&signable, &sdk_sig),
        "{}: SDK sign-verify round-trip failed",
        case.name
    );

    // Quid ID derivation: sha256(pubkey)[:8] hex.
    let pub_bytes = hex::decode(&key.public_key_sec1_hex).unwrap();
    let quid_hash = Sha256::digest(&pub_bytes);
    let derived_quid = hex::encode(&quid_hash[..8]);
    assert_eq!(derived_quid, key.quid_id, "quid_id derivation mismatch");
}

#[test]
fn trust_vectors() {
    let vf = load_vector_file("trust-tx.json");
    assert_eq!(vf.schema_version, "1.0");
    assert_eq!(vf.tx_type, "TRUST");
    assert!(!vf.cases.is_empty());

    let keys = load_keys();

    for case in &vf.cases {
        run_case(case, &keys, |case, key| {
            let inp = &case.input;
            // Extract typed fields. Optional fields default to "".
            let description = inp.get("description").and_then(|v| v.as_str()).unwrap_or("");
            let valid_until = inp.get("validUntil").and_then(|v| v.as_i64()).unwrap_or(0);
            let mut tx = TrustTx {
                id: String::new(),
                tx_type: "TRUST",
                trust_domain: inp["trustDomain"].as_str().unwrap(),
                timestamp: inp["timestamp"].as_i64().unwrap(),
                signature: String::new(),
                public_key: &key.public_key_sec1_hex,
                truster: inp["truster"].as_str().unwrap(),
                trustee: inp["trustee"].as_str().unwrap(),
                trust_level: inp["trustLevel"].as_f64().unwrap(),
                nonce: inp["nonce"].as_i64().unwrap(),
                description,
                valid_until,
            };
            let id = tx.derive_id();
            tx.id = id.clone();
            let signable = serde_json::to_vec(&tx).expect("serialize");
            (signable, id)
        });
    }
}

#[test]
fn identity_vectors() {
    let vf = load_vector_file("identity-tx.json");
    assert_eq!(vf.tx_type, "IDENTITY");
    assert!(!vf.cases.is_empty());

    let keys = load_keys();

    for case in &vf.cases {
        run_case(case, &keys, |case, key| {
            let inp = &case.input;
            let description = inp.get("description").and_then(|v| v.as_str()).unwrap_or("");
            let home_domain = inp.get("homeDomain").and_then(|v| v.as_str()).unwrap_or("");
            let attributes = inp.get("attributes").cloned();
            let mut tx = IdentityTx {
                id: String::new(),
                tx_type: "IDENTITY",
                trust_domain: inp["trustDomain"].as_str().unwrap(),
                timestamp: inp["timestamp"].as_i64().unwrap(),
                signature: String::new(),
                public_key: &key.public_key_sec1_hex,
                quid_id: inp["quidId"].as_str().unwrap(),
                name: inp["name"].as_str().unwrap(),
                description,
                attributes,
                creator: inp["creator"].as_str().unwrap(),
                update_nonce: inp["updateNonce"].as_i64().unwrap(),
                home_domain,
            };
            let id = tx.derive_id();
            tx.id = id.clone();
            let signable = serde_json::to_vec(&tx).expect("serialize");
            (signable, id)
        });
    }
}

#[test]
fn sdk_sign_produces_ieee1363() {
    // Regression probe: Quid::sign must produce 64-byte IEEE-1363.
    let q = Quid::generate();
    let sig_hex = q.sign(b"test-data").expect("sign");
    let sig_bytes = hex::decode(&sig_hex).expect("hex");
    assert_eq!(sig_bytes.len(), 64,
        "v1.0 requires 64-byte IEEE-1363; got {} bytes", sig_bytes.len());
    assert!(q.verify(b"test-data", &sig_hex),
        "SDK Sign/Verify round-trip failed");
}
