//! ECDSA P-256 identity + signing.

use crate::error::{Error, Result};
use p256::ecdsa::{signature::Signer, signature::Verifier, Signature, SigningKey, VerifyingKey};
use p256::pkcs8::{DecodePrivateKey, EncodePrivateKey};
use p256::EncodedPoint;
use sha2::{Digest, Sha256};

/// A cryptographic identity. Holds an ECDSA P-256 keypair (or just
/// the public key for read-only / third-party quids).
#[derive(Debug, Clone)]
pub struct Quid {
    id: String,
    public_key_hex: String,
    private_key_hex: Option<String>,
    signing_key: Option<SigningKey>,
    verifying_key: VerifyingKey,
}

impl Quid {
    /// Generate a fresh P-256 keypair.
    pub fn generate() -> Self {
        let sk = SigningKey::random(&mut rand::thread_rng());
        Self::from_signing_key(sk)
    }

    /// Rebuild a Quid from a PKCS8 DER hex-encoded private key — the
    /// same format emitted by `quid.private_key_hex`.
    pub fn from_private_hex(private_hex: &str) -> Result<Self> {
        let der = hex::decode(private_hex).map_err(|e| Error::crypto(format!("hex: {e}")))?;
        let sk = SigningKey::from_pkcs8_der(&der)
            .map_err(|e| Error::crypto(format!("pkcs8: {e}")))?;
        Ok(Self::from_signing_key(sk))
    }

    /// Build a read-only Quid from a SEC1 uncompressed hex public key.
    pub fn from_public_hex(public_hex: &str) -> Result<Self> {
        let raw = hex::decode(public_hex).map_err(|e| Error::crypto(format!("hex: {e}")))?;
        let point = EncodedPoint::from_bytes(&raw)
            .map_err(|e| Error::crypto(format!("SEC1 decode: {e}")))?;
        let vk = VerifyingKey::from_encoded_point(&point)
            .map_err(|e| Error::crypto(format!("verifying key: {e}")))?;
        let id = derive_quid_id(&raw);
        Ok(Self {
            id,
            public_key_hex: public_hex.to_string(),
            private_key_hex: None,
            signing_key: None,
            verifying_key: vk,
        })
    }

    fn from_signing_key(sk: SigningKey) -> Self {
        let vk = VerifyingKey::from(&sk);
        let ep = vk.to_encoded_point(false); // uncompressed
        let pub_bytes = ep.as_bytes().to_vec();
        let id = derive_quid_id(&pub_bytes);
        let priv_der = sk
            .to_pkcs8_der()
            .expect("sk serializes to PKCS8")
            .as_bytes()
            .to_vec();
        Self {
            id,
            public_key_hex: hex::encode(&pub_bytes),
            private_key_hex: Some(hex::encode(&priv_der)),
            signing_key: Some(sk),
            verifying_key: vk,
        }
    }

    /// Quid ID = sha256(public_key)[0..8] in hex → 16 hex chars.
    pub fn id(&self) -> &str {
        &self.id
    }

    /// SEC1 uncompressed hex public key.
    pub fn public_key_hex(&self) -> &str {
        &self.public_key_hex
    }

    /// PKCS8 DER hex private key, or `None` on read-only Quids.
    pub fn private_key_hex(&self) -> Option<&str> {
        self.private_key_hex.as_deref()
    }

    /// Whether this Quid can sign (vs. read-only).
    pub fn has_private_key(&self) -> bool {
        self.signing_key.is_some()
    }

    /// Sign arbitrary bytes.
    ///
    /// v1.0 canonical form: returns hex-encoded 64-byte IEEE-1363
    /// raw signature (`r||s`, each zero-padded to 32 bytes).
    /// Matches the reference node's `VerifySignature` expectation.
    pub fn sign(&self, data: &[u8]) -> Result<String> {
        let sk = self
            .signing_key
            .as_ref()
            .ok_or_else(|| Error::crypto("quid is read-only"))?;
        let sig: Signature = sk.sign(data);
        // `Signature::to_bytes` returns the raw IEEE-1363 r||s form
        // (64 bytes for P-256). This is what we want on the wire.
        let raw = sig.to_bytes();
        debug_assert_eq!(raw.len(), 64, "P-256 Signature::to_bytes must be 64 bytes");
        Ok(hex::encode(raw))
    }

    /// Verify a hex-encoded IEEE-1363 raw 64-byte signature against
    /// this Quid's public key.
    ///
    /// v1.0 canonical form: expects exactly 64 bytes (32-byte `r`
    /// followed by 32-byte `s`). Anything else is rejected.
    pub fn verify(&self, data: &[u8], sig_hex: &str) -> bool {
        let raw = match hex::decode(sig_hex) {
            Ok(r) => r,
            Err(_) => return false,
        };
        if raw.len() != 64 {
            return false;
        }
        let sig = match Signature::from_slice(&raw) {
            Ok(s) => s,
            Err(_) => return false,
        };
        self.verifying_key.verify(data, &sig).is_ok()
    }

    /// Reconstruct a Quid from a raw private scalar in hex.
    ///
    /// Used primarily by the v1.0 test vectors, which check in
    /// deterministic keys as raw scalars rather than PKCS8 DER.
    /// For production keys, use `from_private_hex` with the
    /// PKCS8-encoded key.
    pub fn from_private_hex_scalar(scalar_hex: &str) -> Result<Self> {
        let scalar_bytes = hex::decode(scalar_hex)
            .map_err(|e| Error::crypto(format!("hex: {e}")))?;
        if scalar_bytes.len() > 32 {
            return Err(Error::crypto("private scalar must be <= 32 bytes"));
        }
        // Left-pad to 32 bytes (scalar may have leading zeros stripped
        // by the generator's big.Int.Bytes() call).
        let mut padded = [0u8; 32];
        let offset = 32 - scalar_bytes.len();
        padded[offset..].copy_from_slice(&scalar_bytes);
        let sk = SigningKey::from_bytes(&padded.into())
            .map_err(|e| Error::crypto(format!("invalid scalar: {e}")))?;
        Ok(Self::from_signing_key(sk))
    }
}

fn derive_quid_id(pub_bytes: &[u8]) -> String {
    let sum = Sha256::digest(pub_bytes);
    hex::encode(&sum[..8])
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn generate_produces_expected_id_format() {
        let q = Quid::generate();
        assert_eq!(q.id().len(), 16);
        assert!(q.has_private_key());
    }

    #[test]
    fn sign_verify_roundtrip() {
        let q = Quid::generate();
        let sig = q.sign(b"hello").unwrap();
        assert!(q.verify(b"hello", &sig));
        assert!(!q.verify(b"tampered", &sig));
    }

    #[test]
    fn private_hex_roundtrip() {
        let q = Quid::generate();
        let q2 = Quid::from_private_hex(q.private_key_hex().unwrap()).unwrap();
        assert_eq!(q.id(), q2.id());
        assert_eq!(q.public_key_hex(), q2.public_key_hex());
    }

    #[test]
    fn read_only_cannot_sign() {
        let q = Quid::generate();
        let ro = Quid::from_public_hex(q.public_key_hex()).unwrap();
        assert!(!ro.has_private_key());
        assert!(ro.sign(b"x").is_err());
    }
}
