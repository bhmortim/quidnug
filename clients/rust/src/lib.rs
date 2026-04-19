//! # Quidnug Rust SDK
//!
//! The official Rust client for [Quidnug], a decentralized protocol
//! for relational, per-observer trust. Covers the full protocol
//! surface (QDPs 0001–0010): identity, trust, titles, event streams,
//! anchors, guardian sets + recovery, cross-domain gossip, K-of-K
//! bootstrap, fork-block activation, and compact Merkle inclusion
//! proofs.
//!
//! ## Thirty-second example
//!
//! ```no_run
//! use quidnug::{Client, Quid, TrustParams};
//!
//! # async fn demo() -> Result<(), quidnug::Error> {
//! let client = Client::new("http://localhost:8080")?;
//! let alice = Quid::generate();
//! let bob = Quid::generate();
//!
//! client.register_identity(&alice, "Alice", "contractors.home").await?;
//! client.register_identity(&bob, "Bob", "contractors.home").await?;
//! client.grant_trust(&alice, TrustParams {
//!     trustee: bob.id(),
//!     level: 0.9,
//!     domain: "contractors.home",
//!     nonce: 1,
//! }).await?;
//!
//! let tr = client.get_trust(alice.id(), bob.id(), "contractors.home", 5).await?;
//! println!("trust = {:.3}", tr.trust_level);
//! # Ok(()) }
//! ```
//!
//! [Quidnug]: https://github.com/bhmortim/quidnug

#![warn(missing_docs)]

mod canonical;
mod client;
mod crypto;
mod error;
mod merkle;
mod types;

pub use canonical::canonical_bytes;
pub use client::{Client, TrustParams};
pub use crypto::Quid;
pub use error::{Error, Result};
pub use merkle::{verify_inclusion_proof, MerkleProofFrame};
pub use types::{
    DomainFingerprint, Event, ForkBlock, GuardianRef, GuardianSet, IdentityRecord,
    NonceSnapshot, OwnershipStake, Title, TrustEdge, TrustResult,
};
