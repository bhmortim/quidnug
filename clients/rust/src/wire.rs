#![allow(missing_docs)] // wire structs are internal-facing mirrors of core tx types.

//! Typed wire structs for v1.0 transaction submission.
//!
//! Each struct mirrors the corresponding type in
//! `internal/core/types.go` of the reference Go implementation,
//! with identical field order and JSON tag names. `serde_json`
//! serializes struct fields in declaration order, so the
//! canonical signable bytes produced here are byte-identical to
//! the server's `json.Marshal` output on its typed struct. This
//! is the invariant the v1.0 cross-SDK test vectors verify.
//!
//! Why not just use the generic `serde_json::Value` construct +
//! `canonical_bytes` (alphabetical round-trip)? Because the
//! server verifies signatures by re-marshaling its typed struct,
//! which is declaration-order, not alphabetical. The two
//! orderings produce different bytes for the same logical tx,
//! so signatures from the alphabetical path don't verify. The
//! vector harness at `docs/test-vectors/v1.0/` locks this in.
//!
//! For read-model dataclasses (e.g., `IdentityRecord`,
//! `TrustEdge`), keep using the existing `types.rs`; they
//! carry `#[serde]` attributes with alphabetical tolerance
//! because they're deserialized, not signed.

use serde::{Serialize, Serializer};
use sha2::{Digest, Sha256};

/// Serialize an `f64` the way Go's `encoding/json` does.
///
/// Go elides the decimal point on integer-valued floats:
/// `json.Marshal(float64(1.0))` produces `"1"`, not `"1.0"`.
/// `serde_json` by default emits `"1.0"`, which diverges.
///
/// We match Go: integer-valued finite floats in the int64 range
/// serialize as an integer; otherwise as a float.
fn serialize_go_compat_f64<S: Serializer>(v: &f64, s: S) -> Result<S::Ok, S::Error> {
    if v.is_finite() && v.fract() == 0.0 && v.abs() < 1e15 {
        s.serialize_i64(*v as i64)
    } else {
        s.serialize_f64(*v)
    }
}

/// Mirror of `core.TrustTransaction`.
#[derive(Debug, Serialize)]
pub struct TrustTx<'a> {
    pub id: String,
    #[serde(rename = "type")]
    pub tx_type: &'a str,
    #[serde(rename = "trustDomain")]
    pub trust_domain: &'a str,
    pub timestamp: i64,
    pub signature: String,
    #[serde(rename = "publicKey")]
    pub public_key: &'a str,
    pub truster: &'a str,
    pub trustee: &'a str,
    #[serde(rename = "trustLevel", serialize_with = "serialize_go_compat_f64")]
    pub trust_level: f64,
    pub nonce: i64,
    #[serde(skip_serializing_if = "str::is_empty")]
    pub description: &'a str,
    #[serde(rename = "validUntil", skip_serializing_if = "is_zero_i64")]
    pub valid_until: i64,
}

impl<'a> TrustTx<'a> {
    /// Derive the transaction ID per `AddTrustTransaction` in
    /// `internal/core/transactions.go`. Payload:
    /// `(Truster, Trustee, TrustLevel, TrustDomain, Timestamp)`.
    pub fn derive_id(&self) -> String {
        #[derive(Serialize)]
        #[allow(non_snake_case)]
        struct Seed<'b> {
            Truster: &'b str,
            Trustee: &'b str,
            #[serde(serialize_with = "serialize_go_compat_f64")]
            TrustLevel: f64,
            TrustDomain: &'b str,
            Timestamp: i64,
        }
        let seed = Seed {
            Truster: self.truster,
            Trustee: self.trustee,
            TrustLevel: self.trust_level,
            TrustDomain: self.trust_domain,
            Timestamp: self.timestamp,
        };
        let bytes = serde_json::to_vec(&seed).expect("seed serialize");
        hex::encode(Sha256::digest(&bytes))
    }
}

/// Mirror of `core.IdentityTransaction`.
///
/// The `attributes` map uses `serde_json::Value` to accept
/// arbitrary JSON payloads without constraining the caller.
#[derive(Debug, Serialize)]
pub struct IdentityTx<'a> {
    pub id: String,
    #[serde(rename = "type")]
    pub tx_type: &'a str,
    #[serde(rename = "trustDomain")]
    pub trust_domain: &'a str,
    pub timestamp: i64,
    pub signature: String,
    #[serde(rename = "publicKey")]
    pub public_key: &'a str,
    #[serde(rename = "quidId")]
    pub quid_id: &'a str,
    pub name: &'a str,
    #[serde(skip_serializing_if = "str::is_empty")]
    pub description: &'a str,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub attributes: Option<serde_json::Value>,
    pub creator: &'a str,
    #[serde(rename = "updateNonce")]
    pub update_nonce: i64,
    #[serde(rename = "homeDomain", skip_serializing_if = "str::is_empty")]
    pub home_domain: &'a str,
}

impl<'a> IdentityTx<'a> {
    /// Derive the tx ID per `AddIdentityTransaction`. Payload:
    /// `(QuidID, Name, Creator, TrustDomain, UpdateNonce, Timestamp)`.
    pub fn derive_id(&self) -> String {
        #[derive(Serialize)]
        #[allow(non_snake_case)]
        struct Seed<'b> {
            QuidID: &'b str,
            Name: &'b str,
            Creator: &'b str,
            TrustDomain: &'b str,
            UpdateNonce: i64,
            Timestamp: i64,
        }
        let seed = Seed {
            QuidID: self.quid_id,
            Name: self.name,
            Creator: self.creator,
            TrustDomain: self.trust_domain,
            UpdateNonce: self.update_nonce,
            Timestamp: self.timestamp,
        };
        let bytes = serde_json::to_vec(&seed).expect("seed serialize");
        hex::encode(Sha256::digest(&bytes))
    }
}

// ---------------------------------------------------------------
// serde helpers
// ---------------------------------------------------------------

#[allow(clippy::trivially_copy_pass_by_ref)]
fn is_zero_i64(v: &i64) -> bool {
    *v == 0
}
