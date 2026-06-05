//! # Quidnug Rust SDK
//!
//! The official Rust client for [Quidnug], a decentralized protocol
//! for relational, per-observer trust.
//!
//! Implements the v1.0-conformant identity / trust / title surface
//! plus compact Merkle inclusion-proof verification (QDP-0010).
//! Wire-level types for the full v2 surface (guardians, gossip,
//! bootstrap, fork-block) are exported so callers can construct and
//! inspect those payloads today; the matching client methods that
//! submit and query them are tracked in the README under "Coverage".
//! See the Go (`pkg/client`) or Python (`clients/python`) SDKs for
//! the reference v2 implementation.
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
pub mod wire;

pub use canonical::canonical_bytes;
pub use client::{Client, TrustParams};
pub use crypto::Quid;
pub use error::{Error, Result};
pub use merkle::{verify_inclusion_proof, MerkleProofFrame};
pub use types::{
    DomainFingerprint, Event, ForkBlock, GuardianRef, GuardianSet, IdentityRecord,
    NonceSnapshot, OwnershipStake, Title, TrustEdge, TrustResult,
};
