"""Typed data classes for every Quidnug wire type.

These mirror the Go reference structs in ``internal/core/`` with
snake_case naming. All dataclasses are plain Python — no external
dependencies — so they serialize cleanly to/from JSON.
"""

from __future__ import annotations

from dataclasses import dataclass, field
from typing import Any, Dict, List, Literal, Optional


# --- Trust, Identity, Title ------------------------------------------------


@dataclass
class TrustEdge:
    truster: str
    trustee: str
    trust_level: float
    domain: str
    nonce: int
    signature: str = ""
    valid_until: Optional[int] = None
    description: Optional[str] = None
    attributes: Dict[str, Any] = field(default_factory=dict)


@dataclass
class IdentityRecord:
    quid_id: str
    creator: str
    update_nonce: int
    signature: str = ""
    name: Optional[str] = None
    description: Optional[str] = None
    attributes: Dict[str, Any] = field(default_factory=dict)
    home_domain: Optional[str] = None
    public_key: Optional[str] = None


@dataclass
class OwnershipStake:
    owner_id: str
    percentage: float
    stake_type: Optional[str] = None


@dataclass
class Title:
    asset_id: str
    domain: str
    title_type: str
    owners: List[OwnershipStake]
    attributes: Dict[str, Any] = field(default_factory=dict)
    creator: str = ""
    signatures: Dict[str, str] = field(default_factory=dict)


# --- Event streams --------------------------------------------------------


@dataclass
class Event:
    subject_id: str
    subject_type: Literal["QUID", "TITLE"]
    event_type: str
    payload: Dict[str, Any] = field(default_factory=dict)
    payload_cid: Optional[str] = None
    timestamp: Optional[int] = None
    sequence: Optional[int] = None
    creator: str = ""
    signature: str = ""


# --- Anchors (QDP-0001) ---------------------------------------------------


@dataclass
class Anchor:
    kind: Literal["rotation", "invalidation", "epoch-cap"]
    signer_quid: str
    anchor_nonce: int
    valid_from: int
    signature: str = ""

    # Rotation fields
    from_epoch: Optional[int] = None
    to_epoch: Optional[int] = None
    new_public_key: Optional[str] = None
    min_next_nonce: Optional[int] = None
    max_accepted_old_nonce: Optional[int] = None

    # Invalidation fields
    epoch_to_invalidate: Optional[int] = None

    # EpochCap fields
    epoch: Optional[int] = None
    max_nonce: Optional[int] = None


# --- Guardians (QDP-0002, QDP-0006) ---------------------------------------


@dataclass
class GuardianRef:
    quid: str
    weight: int = 1
    epoch: int = 0
    added_at_block: Optional[int] = None

    @property
    def effective_weight(self) -> int:
        return self.weight if self.weight > 0 else 1


@dataclass
class GuardianSet:
    subject_quid: str
    guardians: List[GuardianRef]
    threshold: int
    recovery_delay_seconds: int  # serialized as nanoseconds on wire
    require_guardian_rotation: bool = False
    updated_at_block: Optional[int] = None

    @property
    def total_weight(self) -> int:
        return sum(g.effective_weight for g in self.guardians)


@dataclass
class PrimarySignature:
    key_epoch: int
    signature: str


@dataclass
class GuardianSignature:
    guardian_quid: str
    key_epoch: int
    signature: str


@dataclass
class GuardianSetUpdate:
    subject_quid: str
    new_set: GuardianSet
    anchor_nonce: int
    valid_from: int
    primary_signature: Optional[PrimarySignature] = None
    new_guardian_consents: List[GuardianSignature] = field(default_factory=list)
    current_guardian_sigs: List[GuardianSignature] = field(default_factory=list)


@dataclass
class GuardianRecoveryInit:
    subject_quid: str
    from_epoch: int
    to_epoch: int
    new_public_key: str
    min_next_nonce: int
    max_accepted_old_nonce: int
    anchor_nonce: int
    valid_from: int
    guardian_sigs: List[GuardianSignature] = field(default_factory=list)
    expires_at: Optional[int] = None


@dataclass
class GuardianRecoveryVeto:
    subject_quid: str
    recovery_anchor_hash: str
    anchor_nonce: int
    valid_from: int
    primary_signature: Optional[PrimarySignature] = None
    guardian_sigs: List[GuardianSignature] = field(default_factory=list)


@dataclass
class GuardianRecoveryCommit:
    subject_quid: str
    recovery_anchor_hash: str
    anchor_nonce: int
    valid_from: int
    committer_quid: str
    committer_sig: str


@dataclass
class GuardianResignation:
    guardian_quid: str
    subject_quid: str
    guardian_set_hash: str
    resignation_nonce: int
    effective_at: int
    signature: str = ""


# --- Cross-domain gossip (QDP-0003, QDP-0005) -----------------------------


@dataclass
class DomainFingerprint:
    domain: str
    block_height: int
    block_hash: str
    producer_quid: str
    timestamp: int
    signature: str = ""
    schema_version: int = 1


@dataclass
class Block:
    index: int
    timestamp: int
    transactions: List[Any]
    prev_hash: str
    hash: str
    trust_proof: Dict[str, Any]
    nonce_checkpoints: List[Dict[str, Any]] = field(default_factory=list)
    transactions_root: Optional[str] = None  # QDP-0010


@dataclass
class AnchorGossipMessage:
    message_id: str
    origin_domain: str
    origin_block_height: int
    origin_block: Block
    anchor_tx_index: int
    domain_fingerprint: DomainFingerprint
    timestamp: int
    gossip_producer_quid: str
    gossip_signature: str = ""
    schema_version: int = 1
    merkle_proof: Optional[List["MerkleProofFrame"]] = None


@dataclass
class MerkleProofFrame:
    hash: str
    side: Literal["left", "right"]


# --- Snapshots + bootstrap (QDP-0008) -------------------------------------


@dataclass
class NonceSnapshotEntry:
    quid: str
    epoch: int
    max_nonce: int


@dataclass
class NonceSnapshot:
    block_height: int
    block_hash: str
    timestamp: int
    trust_domain: str
    entries: List[NonceSnapshotEntry]
    producer_quid: str
    signature: str = ""
    schema_version: int = 1


# --- Fork-block (QDP-0009) -------------------------------------------------


@dataclass
class ForkSig:
    validator_quid: str
    key_epoch: int
    signature: str


@dataclass
class ForkBlock:
    trust_domain: str
    feature: str
    fork_height: int
    fork_nonce: int
    proposed_at: int
    signatures: List[ForkSig] = field(default_factory=list)
    expires_at: Optional[int] = None


# --- Query results --------------------------------------------------------


@dataclass
class TrustResult:
    observer: str
    target: str
    trust_level: float
    path: List[str]
    path_depth: int
    domain: str


__all__ = [
    "TrustEdge",
    "IdentityRecord",
    "OwnershipStake",
    "Title",
    "Event",
    "Anchor",
    "GuardianRef",
    "GuardianSet",
    "PrimarySignature",
    "GuardianSignature",
    "GuardianSetUpdate",
    "GuardianRecoveryInit",
    "GuardianRecoveryVeto",
    "GuardianRecoveryCommit",
    "GuardianResignation",
    "DomainFingerprint",
    "Block",
    "AnchorGossipMessage",
    "MerkleProofFrame",
    "NonceSnapshotEntry",
    "NonceSnapshot",
    "ForkSig",
    "ForkBlock",
    "TrustResult",
]
