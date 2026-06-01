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
    #[serde(rename = "stakeType", skip_serializing_if = "Option::is_none", default)]
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
    #[serde(default)]
    /// Creator quid.
    pub creator: String,
    #[serde(default)]
    /// Signature.
    pub signature: String,
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
    #[serde(rename = "addedAtBlock", default, skip_serializing_if = "Option::is_none")]
    /// Block at which the guardian was added.
    pub added_at_block: Option<i64>,
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
    #[serde(rename = "requireGuardianRotation", default, skip_serializing_if = "is_false")]
    /// Whether a recovery must also rotate the guardian set.
    pub require_guardian_rotation: bool,
    #[serde(rename = "updatedAtBlock", default, skip_serializing_if = "Option::is_none")]
    /// Block at which the set was last updated.
    pub updated_at_block: Option<i64>,
}

fn is_false(v: &bool) -> bool {
    !*v
}

/// Subject's own signature on a guardian-set update (QDP-0002).
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct PrimarySignature {
    #[serde(rename = "keyEpoch")]
    /// Key epoch the signature is valid under.
    pub key_epoch: u32,
    /// IEEE-1363 hex signature.
    pub signature: String,
}

/// Per-guardian signature with its key epoch (QDP-0002).
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct GuardianSignature {
    #[serde(rename = "guardianQuid")]
    /// Guardian quid.
    pub guardian_quid: String,
    #[serde(rename = "keyEpoch")]
    /// Key epoch.
    pub key_epoch: u32,
    /// IEEE-1363 hex signature.
    pub signature: String,
}

/// Guardian-set update — install or rotate (QDP-0002).
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct GuardianSetUpdate {
    #[serde(rename = "subjectQuid")]
    /// Subject quid.
    pub subject_quid: String,
    #[serde(rename = "newSet")]
    /// New guardian set.
    pub new_set: GuardianSet,
    #[serde(rename = "anchorNonce")]
    /// Anchor nonce.
    pub anchor_nonce: i64,
    #[serde(rename = "validFrom")]
    /// Unix timestamp at which the update becomes valid.
    pub valid_from: i64,
    #[serde(rename = "primarySignature", default, skip_serializing_if = "Option::is_none")]
    /// Subject's primary signature (required when available).
    pub primary_signature: Option<PrimarySignature>,
    #[serde(rename = "newGuardianConsents", default, skip_serializing_if = "Vec::is_empty")]
    /// Consents from incoming guardians.
    pub new_guardian_consents: Vec<GuardianSignature>,
    #[serde(rename = "currentGuardianSigs", default, skip_serializing_if = "Vec::is_empty")]
    /// Signatures from the currently active set.
    pub current_guardian_sigs: Vec<GuardianSignature>,
}

/// Start the M-of-N delayed recovery flow (QDP-0002).
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct GuardianRecoveryInit {
    #[serde(rename = "subjectQuid")]
    /// Subject quid.
    pub subject_quid: String,
    #[serde(rename = "fromEpoch")]
    /// Current key epoch.
    pub from_epoch: u32,
    #[serde(rename = "toEpoch")]
    /// New key epoch.
    pub to_epoch: u32,
    #[serde(rename = "newPublicKey")]
    /// SEC1 hex public key to install.
    pub new_public_key: String,
    #[serde(rename = "minNextNonce")]
    /// Minimum nonce accepted under the new epoch.
    pub min_next_nonce: i64,
    #[serde(rename = "maxAcceptedOldNonce")]
    /// Last accepted nonce under the old epoch.
    pub max_accepted_old_nonce: i64,
    #[serde(rename = "anchorNonce")]
    /// Anchor nonce.
    pub anchor_nonce: i64,
    #[serde(rename = "validFrom")]
    /// Unix timestamp at which the init becomes valid.
    pub valid_from: i64,
    #[serde(rename = "guardianSigs", default, skip_serializing_if = "Vec::is_empty")]
    /// Per-guardian signatures.
    pub guardian_sigs: Vec<GuardianSignature>,
    #[serde(rename = "expiresAt", default, skip_serializing_if = "Option::is_none")]
    /// Expiry timestamp (unix seconds).
    pub expires_at: Option<i64>,
}

/// Abort an in-flight recovery (QDP-0002).
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct GuardianRecoveryVeto {
    #[serde(rename = "subjectQuid")]
    /// Subject quid.
    pub subject_quid: String,
    #[serde(rename = "recoveryAnchorHash")]
    /// Hash of the recovery-init anchor being vetoed.
    pub recovery_anchor_hash: String,
    #[serde(rename = "anchorNonce")]
    /// Anchor nonce.
    pub anchor_nonce: i64,
    #[serde(rename = "validFrom")]
    /// Unix timestamp at which the veto becomes valid.
    pub valid_from: i64,
    #[serde(rename = "primarySignature", default, skip_serializing_if = "Option::is_none")]
    /// Subject's veto signature.
    pub primary_signature: Option<PrimarySignature>,
    #[serde(rename = "guardianSigs", default, skip_serializing_if = "Vec::is_empty")]
    /// Per-guardian veto signatures.
    pub guardian_sigs: Vec<GuardianSignature>,
}

/// Finalize a delayed recovery (QDP-0002).
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct GuardianRecoveryCommit {
    #[serde(rename = "subjectQuid")]
    /// Subject quid.
    pub subject_quid: String,
    #[serde(rename = "recoveryAnchorHash")]
    /// Hash of the original recovery-init anchor.
    pub recovery_anchor_hash: String,
    #[serde(rename = "anchorNonce")]
    /// Anchor nonce.
    pub anchor_nonce: i64,
    #[serde(rename = "validFrom")]
    /// Unix timestamp at which the commit becomes valid.
    pub valid_from: i64,
    #[serde(rename = "committerQuid")]
    /// Quid of the committer.
    pub committer_quid: String,
    #[serde(rename = "committerSig")]
    /// Committer's IEEE-1363 hex signature.
    pub committer_sig: String,
}

/// Guardian-resignation message (QDP-0002).
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct GuardianResignation {
    #[serde(rename = "guardianQuid")]
    /// Resigning guardian.
    pub guardian_quid: String,
    #[serde(rename = "subjectQuid")]
    /// Subject the guardian is resigning from.
    pub subject_quid: String,
    #[serde(rename = "guardianSetHash")]
    /// Hash of the guardian set the resignation refers to.
    pub guardian_set_hash: String,
    #[serde(rename = "resignationNonce")]
    /// Monotonic nonce.
    pub resignation_nonce: i64,
    #[serde(rename = "effectiveAt")]
    /// Unix timestamp at which the resignation takes effect.
    pub effective_at: i64,
    #[serde(default)]
    /// IEEE-1363 hex signature.
    pub signature: String,
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
    #[serde(default)]
    /// IEEE-1363 hex signature.
    pub signature: String,
    #[serde(rename = "schemaVersion", default, skip_serializing_if = "is_zero_i32")]
    /// Schema version (defaults to 1 on emit).
    pub schema_version: i32,
}

fn is_zero_i32(v: &i32) -> bool {
    *v == 0
}

/// One sibling hash + side in a Merkle inclusion proof (QDP-0010).
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct MerkleProofFrameWire {
    /// Sibling hash.
    pub hash: String,
    /// `"left"` or `"right"`.
    pub side: String,
}

/// Cross-domain anchor gossip message (QDP-0003 / QDP-0005).
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct AnchorGossipMessage {
    #[serde(rename = "messageId")]
    /// Globally unique message id (idempotency key).
    pub message_id: String,
    #[serde(rename = "originDomain")]
    /// Origin domain.
    pub origin_domain: String,
    #[serde(rename = "originBlockHeight")]
    /// Origin block height.
    pub origin_block_height: i64,
    #[serde(rename = "originBlock")]
    /// Origin block payload.
    pub origin_block: serde_json::Value,
    #[serde(rename = "anchorTxIndex")]
    /// Index of the anchor tx within the origin block.
    pub anchor_tx_index: i32,
    #[serde(rename = "domainFingerprint")]
    /// Signed fingerprint of the origin domain at this height.
    pub domain_fingerprint: DomainFingerprint,
    /// Unix timestamp.
    pub timestamp: i64,
    #[serde(rename = "gossipProducerQuid")]
    /// Quid of the gossip producer.
    pub gossip_producer_quid: String,
    #[serde(rename = "gossipSignature", default)]
    /// IEEE-1363 hex signature.
    pub gossip_signature: String,
    #[serde(rename = "schemaVersion", default, skip_serializing_if = "is_zero_i32")]
    /// Schema version.
    pub schema_version: i32,
    #[serde(rename = "merkleProof", default, skip_serializing_if = "Option::is_none")]
    /// Optional Merkle proof of inclusion.
    pub merkle_proof: Option<Vec<MerkleProofFrameWire>>,
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
    #[serde(rename = "blockHash", default)]
    /// Snapshot block hash.
    pub block_hash: String,
    #[serde(default)]
    /// Unix timestamp.
    pub timestamp: i64,
    #[serde(rename = "trustDomain", default)]
    /// Domain the snapshot is for.
    pub trust_domain: String,
    /// Entries.
    pub entries: Vec<NonceSnapshotEntry>,
    #[serde(rename = "producerQuid", default)]
    /// Producing node quid.
    pub producer_quid: String,
    #[serde(default)]
    /// IEEE-1363 hex signature.
    pub signature: String,
    #[serde(rename = "schemaVersion", default, skip_serializing_if = "is_zero_i32")]
    /// Schema version.
    pub schema_version: i32,
}

/// One validator's signature on a fork-activation block (QDP-0009).
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct ForkSig {
    #[serde(rename = "validatorQuid")]
    /// Validator quid.
    pub validator_quid: String,
    #[serde(rename = "keyEpoch")]
    /// Key epoch.
    pub key_epoch: u32,
    /// IEEE-1363 hex signature.
    pub signature: String,
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
    #[serde(rename = "forkNonce", default)]
    /// Monotonic nonce.
    pub fork_nonce: i64,
    #[serde(rename = "proposedAt", default)]
    /// Unix timestamp at which the fork was proposed.
    pub proposed_at: i64,
    #[serde(default, skip_serializing_if = "Vec::is_empty")]
    /// Validator signatures.
    pub signatures: Vec<ForkSig>,
    #[serde(rename = "expiresAt", default, skip_serializing_if = "Option::is_none")]
    /// Expiry timestamp (unix seconds).
    pub expires_at: Option<i64>,
}
