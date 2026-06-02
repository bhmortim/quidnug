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
    #[serde(rename = "addedAtBlock", default, skip_serializing_if = "is_zero_i64")]
    /// Block at which this guardian was added.
    pub added_at_block: i64,
}

fn one_u32() -> u32 {
    1
}

#[allow(clippy::trivially_copy_pass_by_ref)]
fn is_zero_i64(v: &i64) -> bool {
    *v == 0
}

#[allow(clippy::trivially_copy_pass_by_ref)]
fn is_false(v: &bool) -> bool {
    !*v
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
    /// Whether to require guardian rotation on recovery.
    pub require_guardian_rotation: bool,
    #[serde(rename = "updatedAtBlock", default, skip_serializing_if = "is_zero_i64")]
    /// Block at which this set was last updated.
    pub updated_at_block: i64,
}

/// Primary (subject) signature on a guardian-set update.
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct PrimarySignature {
    #[serde(rename = "keyEpoch")]
    /// Key epoch the signature is valid under.
    pub key_epoch: u32,
    /// Hex-encoded IEEE-1363 signature.
    pub signature: String,
}

/// Per-guardian signature on a guardian set update / recovery message.
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct GuardianSignature {
    #[serde(rename = "guardianQuid")]
    /// Guardian quid.
    pub guardian_quid: String,
    #[serde(rename = "keyEpoch")]
    /// Key epoch.
    pub key_epoch: u32,
    /// Signature.
    pub signature: String,
}

/// Install or rotate guardian configuration (QDP-0002).
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
    /// Unix-seconds from which this update is valid.
    pub valid_from: i64,
    #[serde(rename = "primarySignature", default, skip_serializing_if = "Option::is_none")]
    /// Subject's own signature, when available.
    pub primary_signature: Option<PrimarySignature>,
    #[serde(rename = "newGuardianConsents", default, skip_serializing_if = "Vec::is_empty")]
    /// Consents from each new guardian.
    pub new_guardian_consents: Vec<GuardianSignature>,
    #[serde(rename = "currentGuardianSigs", default, skip_serializing_if = "Vec::is_empty")]
    /// Signatures from current guardians authorizing the change.
    pub current_guardian_sigs: Vec<GuardianSignature>,
}

/// Start the delayed recovery flow (QDP-0006).
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct GuardianRecoveryInit {
    #[serde(rename = "subjectQuid")]
    /// Subject quid.
    pub subject_quid: String,
    #[serde(rename = "fromEpoch")]
    /// Current key epoch.
    pub from_epoch: u32,
    #[serde(rename = "toEpoch")]
    /// Target key epoch after recovery.
    pub to_epoch: u32,
    #[serde(rename = "newPublicKey")]
    /// SEC1 hex of the new public key.
    pub new_public_key: String,
    #[serde(rename = "minNextNonce")]
    /// Lowest acceptable post-recovery nonce.
    pub min_next_nonce: i64,
    #[serde(rename = "maxAcceptedOldNonce")]
    /// Highest nonce accepted from the old key.
    pub max_accepted_old_nonce: i64,
    #[serde(rename = "anchorNonce")]
    /// Anchor nonce.
    pub anchor_nonce: i64,
    #[serde(rename = "validFrom")]
    /// Unix-seconds from which this init is valid.
    pub valid_from: i64,
    #[serde(rename = "guardianSigs", default, skip_serializing_if = "Vec::is_empty")]
    /// Guardian signatures supporting the init.
    pub guardian_sigs: Vec<GuardianSignature>,
    #[serde(rename = "expiresAt", default, skip_serializing_if = "is_zero_i64")]
    /// Optional expiry.
    pub expires_at: i64,
}

/// Abort an in-flight recovery (QDP-0006).
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct GuardianRecoveryVeto {
    #[serde(rename = "subjectQuid")]
    /// Subject quid.
    pub subject_quid: String,
    #[serde(rename = "recoveryAnchorHash")]
    /// Recovery anchor being vetoed.
    pub recovery_anchor_hash: String,
    #[serde(rename = "anchorNonce")]
    /// Anchor nonce.
    pub anchor_nonce: i64,
    #[serde(rename = "validFrom")]
    /// Unix-seconds from which this veto is valid.
    pub valid_from: i64,
    #[serde(rename = "primarySignature", default, skip_serializing_if = "Option::is_none")]
    /// Subject signature if subject participated.
    pub primary_signature: Option<PrimarySignature>,
    #[serde(rename = "guardianSigs", default, skip_serializing_if = "Vec::is_empty")]
    /// Guardian signatures backing the veto.
    pub guardian_sigs: Vec<GuardianSignature>,
}

/// Finalize a recovery after the delay (QDP-0006).
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct GuardianRecoveryCommit {
    #[serde(rename = "subjectQuid")]
    /// Subject quid.
    pub subject_quid: String,
    #[serde(rename = "recoveryAnchorHash")]
    /// Recovery anchor hash.
    pub recovery_anchor_hash: String,
    #[serde(rename = "anchorNonce")]
    /// Anchor nonce.
    pub anchor_nonce: i64,
    #[serde(rename = "validFrom")]
    /// Unix-seconds from which this commit is valid.
    pub valid_from: i64,
    #[serde(rename = "committerQuid")]
    /// Committing quid.
    pub committer_quid: String,
    #[serde(rename = "committerSig")]
    /// Committer signature.
    pub committer_sig: String,
}

/// Voluntary guardian resignation (QDP-0002).
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct GuardianResignation {
    #[serde(rename = "guardianQuid")]
    /// Resigning guardian.
    pub guardian_quid: String,
    #[serde(rename = "subjectQuid")]
    /// Subject the guardian is leaving.
    pub subject_quid: String,
    #[serde(rename = "guardianSetHash")]
    /// Hash of the current guardian set.
    pub guardian_set_hash: String,
    #[serde(rename = "resignationNonce")]
    /// Monotonic nonce for this resignation.
    pub resignation_nonce: i64,
    #[serde(rename = "effectiveAt")]
    /// Unix-seconds when the resignation takes effect.
    pub effective_at: i64,
    #[serde(default, skip_serializing_if = "String::is_empty")]
    /// Resigning guardian's signature.
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
    #[serde(default, skip_serializing_if = "String::is_empty")]
    /// Producer signature.
    pub signature: String,
    #[serde(rename = "schemaVersion", default, skip_serializing_if = "is_zero_u32")]
    /// Schema version.
    pub schema_version: u32,
}

#[allow(clippy::trivially_copy_pass_by_ref)]
fn is_zero_u32(v: &u32) -> bool {
    *v == 0
}

/// One sibling-hash frame in a compact Merkle inclusion proof (QDP-0010).
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct MerkleProofFrameWire {
    /// Sibling hash (hex).
    pub hash: String,
    /// Side: `"left"` or `"right"`.
    pub side: String,
}

/// Cross-domain anchor gossip message (QDP-0005).
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct AnchorGossipMessage {
    #[serde(rename = "messageId")]
    /// Unique message ID.
    pub message_id: String,
    #[serde(rename = "originDomain")]
    /// Origin domain.
    pub origin_domain: String,
    #[serde(rename = "originBlockHeight")]
    /// Origin block height.
    pub origin_block_height: i64,
    #[serde(rename = "originBlock")]
    /// Raw origin block payload.
    pub origin_block: HashMap<String, serde_json::Value>,
    #[serde(rename = "anchorTxIndex")]
    /// Index of the anchor tx within the block.
    pub anchor_tx_index: i64,
    #[serde(rename = "domainFingerprint")]
    /// Accompanying signed fingerprint.
    pub domain_fingerprint: DomainFingerprint,
    /// Unix timestamp.
    pub timestamp: i64,
    #[serde(rename = "gossipProducerQuid")]
    /// Gossip producer quid.
    pub gossip_producer_quid: String,
    #[serde(rename = "gossipSignature", default, skip_serializing_if = "String::is_empty")]
    /// Producer signature on the gossip envelope.
    pub gossip_signature: String,
    #[serde(rename = "schemaVersion", default, skip_serializing_if = "is_zero_u32")]
    /// Schema version.
    pub schema_version: u32,
    #[serde(rename = "merkleProof", default, skip_serializing_if = "Vec::is_empty")]
    /// Optional inclusion proof.
    pub merkle_proof: Vec<MerkleProofFrameWire>,
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
    #[serde(rename = "blockHash", default, skip_serializing_if = "String::is_empty")]
    /// Hash of the snapshot block.
    pub block_hash: String,
    #[serde(default, skip_serializing_if = "is_zero_i64")]
    /// Unix timestamp.
    pub timestamp: i64,
    #[serde(rename = "trustDomain", default, skip_serializing_if = "String::is_empty")]
    /// Domain.
    pub trust_domain: String,
    /// Entries.
    pub entries: Vec<NonceSnapshotEntry>,
    #[serde(rename = "producerQuid", default, skip_serializing_if = "String::is_empty")]
    /// Producer quid.
    pub producer_quid: String,
    #[serde(default, skip_serializing_if = "String::is_empty")]
    /// Producer signature.
    pub signature: String,
    #[serde(rename = "schemaVersion", default, skip_serializing_if = "is_zero_u32")]
    /// Schema version.
    pub schema_version: u32,
}

/// One validator's signature on a fork-activation block (QDP-0009).
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct ForkSig {
    #[serde(rename = "validatorQuid")]
    /// Validator quid.
    pub validator_quid: String,
    #[serde(rename = "keyEpoch")]
    /// Key epoch the signature is valid under.
    pub key_epoch: u32,
    /// Signature.
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
    #[serde(rename = "forkNonce", default, skip_serializing_if = "is_zero_i64")]
    /// Monotonic fork nonce.
    pub fork_nonce: i64,
    #[serde(rename = "proposedAt", default, skip_serializing_if = "is_zero_i64")]
    /// Proposal timestamp.
    pub proposed_at: i64,
    #[serde(default, skip_serializing_if = "Vec::is_empty")]
    /// Validator signatures.
    pub signatures: Vec<ForkSig>,
    #[serde(rename = "expiresAt", default, skip_serializing_if = "is_zero_i64")]
    /// Optional expiry.
    pub expires_at: i64,
}

/// Standard pagination envelope returned alongside list endpoints.
#[derive(Debug, Clone, Default, Serialize, Deserialize)]
pub struct Pagination {
    #[serde(default)]
    /// Page size limit.
    pub limit: i64,
    #[serde(default)]
    /// Offset of the first row in the response.
    pub offset: i64,
    #[serde(default)]
    /// Total matching rows.
    pub total: i64,
}

/// Arguments for [`crate::Client::discover_quids`] (QDP-0014).
#[derive(Debug, Clone, Default)]
pub struct DiscoverQuidsParams {
    /// Required. Domain to query.
    pub domain: String,
    /// Only consider quids active since this UnixNano timestamp.
    pub since: i64,
    /// One of `"activity"`, `"last-seen"`, `"first-seen"`, `"trust-weight"`.
    pub sort: String,
    /// Observer quid (enables `trust-weight` sort).
    pub observer: String,
    /// Filter by event type.
    pub event_type: String,
    /// Floor on trust-weight (with observer).
    pub min_trust_weight: f64,
    /// Comma-joined quid IDs to exclude.
    pub exclude_quids: Vec<String>,
    /// Page size; server caps at 500, defaults to 50.
    pub limit: u32,
    /// Page offset.
    pub offset: u32,
}

/// Endpoint hint advertised by a node (QDP-0014).
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct NodeAdvertEndpoint {
    /// Endpoint URL.
    pub url: String,
    #[serde(default, skip_serializing_if = "String::is_empty")]
    /// Protocol hint (e.g. `https`, `quic`).
    pub protocol: String,
    #[serde(default, skip_serializing_if = "String::is_empty")]
    /// Region hint.
    pub region: String,
    /// Priority (lower is preferred).
    pub priority: i32,
    /// Weight among equal-priority endpoints.
    pub weight: i32,
}

/// Optional capabilities advertised by a node (QDP-0014).
#[derive(Debug, Clone, Default, Serialize, Deserialize)]
pub struct NodeAdvertCapabilities {
    #[serde(default, skip_serializing_if = "is_false")]
    /// Validator role.
    pub validator: bool,
    #[serde(default, skip_serializing_if = "is_false")]
    /// Cache / replica role.
    pub cache: bool,
    #[serde(default, skip_serializing_if = "is_false")]
    /// Archive role.
    pub archive: bool,
    #[serde(default, skip_serializing_if = "is_false")]
    /// Bootstrap role.
    pub bootstrap: bool,
    #[serde(rename = "gossipSink", default, skip_serializing_if = "is_false")]
    /// Gossip sink role.
    pub gossip_sink: bool,
    #[serde(rename = "ipfsGateway", default, skip_serializing_if = "is_false")]
    /// IPFS gateway role.
    pub ipfs_gateway: bool,
    #[serde(rename = "maxBodyBytes", default, skip_serializing_if = "is_zero_i64")]
    /// Maximum body bytes (0 means unset).
    pub max_body_bytes: i64,
    #[serde(rename = "minPeerProtocol", default, skip_serializing_if = "String::is_empty")]
    /// Minimum peer protocol version.
    pub min_peer_protocol: String,
}

/// Writable fields for [`crate::Client::register_title`].
#[derive(Debug, Clone)]
pub struct TitleParams<'a> {
    /// Asset (title) quid ID.
    pub asset_id: &'a str,
    /// Owner stakes; percentages must sum to 1.0 (or 100.0 for percent).
    pub owners: Vec<OwnershipStake>,
    /// Trust domain (empty → `"default"`).
    pub domain: &'a str,
    /// Discriminator (e.g. `"VEHICLE"`, `"DOMAIN"`).
    pub title_type: &'a str,
    /// Accepted for caller compat; not signed on the wire.
    pub prev_title_tx_id: &'a str,
}

/// Writable fields for [`crate::Client::emit_event`]. Exactly one of
/// `payload` or `payload_cid` must be set.
#[derive(Debug, Clone, Default)]
pub struct EventParams<'a> {
    /// Subject quid / title ID.
    pub subject_id: &'a str,
    /// `"QUID"` or `"TITLE"`.
    pub subject_type: &'a str,
    /// Event type discriminator.
    pub event_type: &'a str,
    /// Trust domain (empty → `"default"`).
    pub domain: &'a str,
    /// Inline payload (mutually exclusive with `payload_cid`).
    pub payload: Option<HashMap<String, serde_json::Value>>,
    /// IPFS CID of payload (mutually exclusive with `payload`).
    pub payload_cid: &'a str,
    /// Sequence number; 0 = auto-detect via stream lookup.
    pub sequence: i64,
}

/// Writable fields for [`crate::Client::publish_node_advertisement`]
/// (QDP-0014). Signer is the node's own keypair; `NodeQuid = signer.id()`.
#[derive(Debug, Clone)]
pub struct NodeAdvertisementParams<'a> {
    /// Operator quid (must have a current direct TRUST edge to the node).
    pub operator_quid: &'a str,
    /// trustDomain; typically `operators.network.<your-domain>`.
    pub domain: &'a str,
    /// At least one endpoint required.
    pub endpoints: Vec<NodeAdvertEndpoint>,
    /// Domains served.
    pub supported_domains: Vec<String>,
    /// Capabilities.
    pub capabilities: NodeAdvertCapabilities,
    /// Protocol version (default `"1.0"`).
    pub protocol_version: &'a str,
    /// TTL in seconds (default 21600; max 604800).
    pub ttl_seconds: i64,
    /// Strictly monotonic per NodeQuid.
    pub advertisement_nonce: i64,
}
