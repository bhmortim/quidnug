//! Compact Merkle inclusion-proof verifier (QDP-0010).

use crate::error::{Error, Result};
use serde::{Deserialize, Serialize};
use sha2::{Digest, Sha256};

/// One frame in a Merkle inclusion proof.
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct MerkleProofFrame {
    /// Hex-encoded 32-byte sibling hash.
    pub hash: String,
    /// "left" or "right" — which side the sibling sits on.
    pub side: String,
}

/// Verify a proof reconstructs the expected root.
///
/// Returns `Ok(true)` on success, `Ok(false)` if the proof does not
/// match, and `Err(_)` on malformed input.
pub fn verify_inclusion_proof(
    tx_bytes: &[u8],
    frames: &[MerkleProofFrame],
    expected_root_hex: &str,
) -> Result<bool> {
    if tx_bytes.is_empty() {
        return Err(Error::crypto("tx_bytes is empty"));
    }
    let expected = hex::decode(expected_root_hex)
        .map_err(|e| Error::validation(format!("expected_root hex: {e}")))?;
    if expected.len() != 32 {
        return Err(Error::validation(format!(
            "expected_root must be 32 bytes (got {})",
            expected.len()
        )));
    }

    let mut current = Sha256::digest(tx_bytes).to_vec();
    for (i, f) in frames.iter().enumerate() {
        let sib = hex::decode(&f.hash)
            .map_err(|e| Error::validation(format!("frame {i} hash hex: {e}")))?;
        if sib.len() != 32 {
            return Err(Error::validation(format!(
                "frame {i} hash must be 32 bytes (got {})",
                sib.len()
            )));
        }
        let parent = match f.side.as_str() {
            "left" => {
                let mut concat = Vec::with_capacity(64);
                concat.extend_from_slice(&sib);
                concat.extend_from_slice(&current);
                Sha256::digest(&concat).to_vec()
            }
            "right" => {
                let mut concat = Vec::with_capacity(64);
                concat.extend_from_slice(&current);
                concat.extend_from_slice(&sib);
                Sha256::digest(&concat).to_vec()
            }
            other => {
                return Err(Error::validation(format!(
                    "frame {i} side must be 'left' or 'right', got {other:?}"
                )))
            }
        };
        current = parent;
    }
    Ok(current == expected)
}

#[cfg(test)]
mod tests {
    use super::*;

    fn sh(data: &[u8]) -> Vec<u8> {
        Sha256::digest(data).to_vec()
    }

    #[test]
    fn single_sibling_right() {
        let tx = b"tx-1";
        let sib = sh(b"tx-2");
        let leaf = sh(tx);
        let mut concat = leaf.clone();
        concat.extend_from_slice(&sib);
        let root = sh(&concat);
        let frames = vec![MerkleProofFrame {
            hash: hex::encode(&sib),
            side: "right".into(),
        }];
        assert!(verify_inclusion_proof(tx, &frames, &hex::encode(&root)).unwrap());
    }

    #[test]
    fn tampered_tx_rejected() {
        let tx = b"tx-1";
        let sib = sh(b"tx-2");
        let leaf = sh(tx);
        let mut concat = leaf.clone();
        concat.extend_from_slice(&sib);
        let root = sh(&concat);
        let frames = vec![MerkleProofFrame {
            hash: hex::encode(&sib),
            side: "right".into(),
        }];
        assert!(!verify_inclusion_proof(b"tampered", &frames, &hex::encode(&root)).unwrap());
    }

    #[test]
    fn malformed_frame_errs() {
        let frames = vec![MerkleProofFrame {
            hash: "nothex".into(),
            side: "right".into(),
        }];
        assert!(verify_inclusion_proof(b"x", &frames, &hex::encode(sh(b"r"))).is_err());

        let frames = vec![MerkleProofFrame {
            hash: hex::encode(sh(b"s")),
            side: "middle".into(),
        }];
        assert!(verify_inclusion_proof(b"x", &frames, &hex::encode(sh(b"r"))).is_err());
    }

    #[test]
    fn empty_tx_errs() {
        let res = verify_inclusion_proof(&[], &[], &hex::encode(sh(b"r")));
        assert!(res.is_err());
    }
}
