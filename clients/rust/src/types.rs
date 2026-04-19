//! Wire types: serde-friendly structs mirroring the Go reference.

use serde::{Deserialize, Serialize};
use std::collections::HashMap;

/// Per-owner ownership stake on a title.
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct OwnershipStake {
    #[serde(rename = "ownerId")]
    /// Quid ID of the owner.
    pub owner_id: String,
    /// Ownership percentage (summing to 100 across a title).
    pub percentage: f64,
    #[serde(rename = "stakeType", skip_serializing_if = "Option::is_none")]
    /// Optional stake type discriminator.
    pub stake_type: Option<String>,
}

/// Title record as returned by the node.
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct Title {
    #[serde(rename = "assetId")]
    /// Asset quid ID.
    pub asset_id: String,
    #[serde(default)]
    /// Domain the title belongs to.
    pub domain: String,
    #[serde(rename = "titleType", default)]
    /// Type discriminator.
    pub title_type: String,
    #[serde(rename = "ownershipMap", default)]
    /// Per-owner stakes.
    pub owners: Vec<OwnershipStake>,
    #[serde(rename = "issuerQuid", default)]
    /// Quid that issued the title.
    pub creator: String,
    #[serde(rename = "transferSigs", default)]
    /// Prior-owner transfer signatures.
    pub signatures: HashMap<String, String>,
    #[serde(default)]
    /// Free-form attributes.
    pub attributes: HashMap<String, serde_json::Value>,
}

/// Identity record as returned by the node.
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct IdentityRecord {
    #[serde(rename = "quidId")]
    /// Quid ID.
    pub quid_id: String,
    /// Creator quid.
    #[serde(default)]
    pub creator: String,
    #[serde(rename = "updateNonce", default)]
    /// Monotonic update nonce.
    pub update_nonce: i64,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    /// Human-readable name.
    pub name: Option<String>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    /// Description.
    pub description: Option<String>,
    #[serde(default)]
    /// Free-form attributes.
    pub attributes: HashMap<String, serde_json::Value>,
    #[serde(rename = "homeDomain", default, skip_serializing_if = "Option::is_none")]
    /// QDP-0007 home domain.
    pub home_domain: Option<String>,
    #[serde(rename = "publicKey", default, skip_serializing_if = "Option::is_none")]
    /// SEC1 hex public key.
    pub public_key: Option<String>,
}

/// Direct outbound trust edge.
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct TrustEdge {
    /// Truster quid.
    pub truster: String,
    /// Trustee quid.
    pub trustee: String,
    #[serde(rename = "trustLevel")]
    /// Trust level in [0, 1].
    pub trust_level: f64,
    /// Domain.
    pub domain: String,
    /// Monotonic nonce.
    pub nonce: i64,
    #[serde(default)]
    /// Signature.
    pub signature: String,
    #[serde(rename = "validUntil", default)]
    /// Optional expiry (unix seconds).
    pub valid_until: i64,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    /// Optional description.
    pub description: Option<String>,
}

/// Relational trust query result.
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct TrustResult {
    /// Observer quid.
    pub observer: String,
    /// Target quid.
    pub target: String,
    #[serde(rename = "trustLevel")]
    /// Computed relational trust in [0, 1].
    pub trust_level: f64,
    #[serde(rename = "trustPath", default)]
    /// Best path observer → target.
    pub path: Vec<String>,
    #[serde(rename = "pathDepth", default)]
    /// Path depth (0 = direct; -1 when no path).
    pub path_depth: i64,
    /// Domain.
    pub domain: String,
}

/// Event-stream row.
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct Event {
    #[serde(rename = "subjectId")]
    /// Subject quid / title id.
    pub subject_id: String,
    #[serde(rename = "subjectType")]
    /// `"QUID"` or `"TITLE"`.
    pub subject_type: String,
    #[serde(rename = "eventType")]
    /// Event type discriminator.
    pub event_type: String,
    #[serde(default)]
    /// Inline payload (mutually exclusive with `payload_cid`).
    pub payload: HashMap<String, serde_json::Value>,
    #[serde(rename = "payloadCid", default, skip_serializing_if = "Option::is_none")]
    /// IPFS CID of payload.
    pub payload_cid: Option<String>,
    #[serde(default)]
    /// Unix timestamp.
    pub timestamp: i64,
    #[serde(default)]
    /// Sequence number in the subject's stream.
    pub sequence: i64,
}

/// Guardian (QDP-0002).
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct GuardianRef {
    /// Guardian quid.
    pub quid: String,
    #[serde(default = "one_u32")]
    /// Effective weight (min 1).
    pub weight: u32,
    #[serde(default)]
    /// Key epoch the guardian's signature is valid under.
    pub epoch: u32,
}

fn one_u32() -> u32 {
    1
}

/// Guardian set (QDP-0002).
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct GuardianSet {
    #[serde(rename = "subjectQuid")]
    /// Subject quid.
    pub subject_quid: String,
    /// Guardians.
    pub guardians: Vec<GuardianRef>,
    /// Weighted quorum threshold.
    pub threshold: u32,
    #[serde(rename = "recoveryDelaySeconds")]
    /// Delay between recovery init and commit.
    pub recovery_delay_seconds: i64,
}

/// Domain fingerprint (QDP-0003).
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct DomainFingerprint {
    /// Domain.
    pub domain: String,
    #[serde(rename = "blockHeight")]
    /// Fingerprinted block height.
    pub block_height: i64,
    #[serde(rename = "blockHash")]
    /// Fingerprinted block hash.
    pub block_hash: String,
    #[serde(rename = "producerQuid")]
    /// Producing node quid.
    pub producer_quid: String,
    /// Unix timestamp.
    pub timestamp: i64,
}

/// Nonce snapshot entry (QDP-0008).
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct NonceSnapshotEntry {
    /// Quid.
    pub quid: String,
    /// Epoch.
    pub epoch: u32,
    #[serde(rename = "maxNonce")]
    /// Max nonce observed.
    pub max_nonce: i64,
}

/// Nonce snapshot (QDP-0008).
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct NonceSnapshot {
    #[serde(rename = "blockHeight")]
    /// Snapshot block height.
    pub block_height: i64,
    /// Entries.
    pub entries: Vec<NonceSnapshotEntry>,
}

/// Fork-activation block (QDP-0009).
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct ForkBlock {
    #[serde(rename = "trustDomain")]
    /// Domain.
    pub trust_domain: String,
    /// Feature name.
    pub feature: String,
    #[serde(rename = "forkHeight")]
    /// Activation height.
    pub fork_height: i64,
}
