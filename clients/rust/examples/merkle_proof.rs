//! QDP-0010 Merkle-proof verifier demo.

use quidnug::{verify_inclusion_proof, MerkleProofFrame};
use sha2::{Digest, Sha256};

fn sh(data: &[u8]) -> Vec<u8> {
    Sha256::digest(data).to_vec()
}

fn main() {
    let leaves: Vec<Vec<u8>> = (0..4).map(|i| sh(format!("tx-{i}").as_bytes())).collect();
    let pair0 = sh(&[&leaves[0][..], &leaves[1][..]].concat());
    let pair1 = sh(&[&leaves[2][..], &leaves[3][..]].concat());
    let root = sh(&[&pair0[..], &pair1[..]].concat());

    let frames = vec![
        MerkleProofFrame {
            hash: hex::encode(&leaves[3]),
            side: "right".into(),
        },
        MerkleProofFrame {
            hash: hex::encode(&pair0),
            side: "left".into(),
        },
    ];

    let ok = verify_inclusion_proof(b"tx-2", &frames, &hex::encode(&root)).unwrap();
    println!("valid proof: {}", ok);

    let tampered = verify_inclusion_proof(b"tx-forged", &frames, &hex::encode(&root)).unwrap();
    println!("tampered rejected: {}", !tampered);
}
