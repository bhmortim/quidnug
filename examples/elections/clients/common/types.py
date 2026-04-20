"""
Data classes for events, transactions, and ballot artifacts.

These mirror the Go struct definitions the reference node uses,
adapted to Python dataclasses for readability. When the node
returns JSON from the API, we parse into these; when we need to
submit signed events, we serialize from these.
"""
from __future__ import annotations

import json
import time
from dataclasses import dataclass, field, asdict
from typing import Any, Optional


# ---------------------------------------------------------------
# Quid (identity)
# ---------------------------------------------------------------

@dataclass
class Quid:
    """A Quidnug cryptographic identity. The id is 16 hex chars
    derived from sha256 of the public key (SEC1 uncompressed)."""
    id: str
    public_key_hex: str
    name: Optional[str] = None
    attributes: dict = field(default_factory=dict)


# ---------------------------------------------------------------
# Events (per architecture.md Events schema)
# ---------------------------------------------------------------

@dataclass
class EventTx:
    """A generic event transaction. The schema mirrors the Go
    `EventTransaction` struct. Payload's shape depends on
    eventType."""
    subject_id: str
    subject_type: str         # "QUID" or "TITLE"
    sequence: int
    event_type: str           # "VOTER_REGISTERED", "CHECK_IN", etc.
    domain: str
    payload: dict
    timestamp: int = field(default_factory=lambda: int(time.time() * 1_000_000_000))
    public_key_hex: Optional[str] = None
    signature_hex: Optional[str] = None
    id: Optional[str] = None

    def to_signable_dict(self) -> dict:
        """Return the dict representation with signature cleared,
        ready for canonical serialization + ECDSA signing. Matches
        the Go node's json.Marshal ordering."""
        return {
            "id":          self.id or "",
            "type":        "EVENT",
            "trustDomain": self.domain,
            "timestamp":   self.timestamp,
            "signature":   "",
            "publicKey":   self.public_key_hex or "",
            "subjectId":   self.subject_id,
            "subjectType": self.subject_type,
            "sequence":    self.sequence,
            "eventType":   self.event_type,
            "payload":     self.payload,
        }

    def canonical_bytes(self) -> bytes:
        """Canonical signable byte form: compact JSON with key
        order matching Go struct field order. Identical to what
        the node uses internally."""
        d = self.to_signable_dict()
        return json.dumps(d, separators=(",", ":")).encode("utf-8")


# ---------------------------------------------------------------
# Trust transactions (votes are these, per the elections design)
# ---------------------------------------------------------------

@dataclass
class TrustTx:
    """A TRUST transaction. In the elections design, a vote is a
    TRUST edge from a BQ to a candidate with level 1.0. The
    `ballot_proof` payload field is the QDP-0021 addition."""
    truster: str
    trustee: str
    trust_level: float
    domain: str
    nonce: int
    ballot_proof: Optional[dict] = None   # QDP-0021
    timestamp: int = field(default_factory=lambda: int(time.time() * 1_000_000_000))
    public_key_hex: Optional[str] = None
    signature_hex: Optional[str] = None
    id: Optional[str] = None

    def to_signable_dict(self) -> dict:
        d = {
            "id":          self.id or "",
            "type":        "TRUST",
            "trustDomain": self.domain,
            "timestamp":   self.timestamp,
            "signature":   "",
            "publicKey":   self.public_key_hex or "",
            "truster":     self.truster,
            "trustee":     self.trustee,
            "trustLevel":  self.trust_level,
            "nonce":       self.nonce,
        }
        if self.ballot_proof is not None:
            d["ballotProof"] = self.ballot_proof
        return d

    def canonical_bytes(self) -> bytes:
        return json.dumps(
            self.to_signable_dict(), separators=(",", ":")
        ).encode("utf-8")


# ---------------------------------------------------------------
# Identity transactions
# ---------------------------------------------------------------

@dataclass
class IdentityTx:
    """A quid registration transaction. For the VRQ + BQ cases
    in elections."""
    quid_id: str
    name: str
    creator: str
    domain: str
    update_nonce: int = 1
    attributes: dict = field(default_factory=dict)
    timestamp: int = field(default_factory=lambda: int(time.time() * 1_000_000_000))
    public_key_hex: Optional[str] = None
    signature_hex: Optional[str] = None
    id: Optional[str] = None

    def to_signable_dict(self) -> dict:
        return {
            "id":          self.id or "",
            "type":        "IDENTITY",
            "trustDomain": self.domain,
            "timestamp":   self.timestamp,
            "signature":   "",
            "publicKey":   self.public_key_hex or "",
            "quidId":      self.quid_id,
            "name":        self.name,
            "attributes":  self.attributes,
            "creator":     self.creator,
            "updateNonce": self.update_nonce,
        }

    def canonical_bytes(self) -> bytes:
        return json.dumps(
            self.to_signable_dict(), separators=(",", ":")
        ).encode("utf-8")


# ---------------------------------------------------------------
# Ballot artifacts (client-side only; not serialized on-chain
# except embedded in TrustTx.ballot_proof)
# ---------------------------------------------------------------

@dataclass
class BallotArtifact:
    """Everything the voter holds after successful ballot
    issuance. Used at vote-cast time to build the ballot_proof
    block in their TRUST transactions.

    The ballot_signature is the authority's unblinded RSA
    signature on the ballot_token. Preservation of this artifact
    is the voter's individual receipt (the "I can verify my vote
    was counted" primitive)."""
    election_id: str
    ballot_token: bytes              # 32 bytes
    ballot_signature: bytes          # 384 bytes for RSA-3072
    rsa_key_fingerprint: str
    bq_pubkey_hex: str
    bq_private_key_hex: str          # voter holds, signs votes

    def to_proof_dict(self) -> dict:
        """Serialize into the ballotProof field embedded in
        each vote's TRUST transaction."""
        return {
            "electionId":         self.election_id,
            "ballotToken":        self.ballot_token.hex(),
            "blindSignature":     self.ballot_signature.hex(),
            "rsaKeyFingerprint":  self.rsa_key_fingerprint,
            "bqEphemeralPubkey":  self.bq_pubkey_hex,
        }


# ---------------------------------------------------------------
# Blind-issuance request payload (HTTP, not on-chain)
# ---------------------------------------------------------------

@dataclass
class BallotIssuanceRequest:
    """The signed HTTP request body the voter's app sends to the
    authority to request a blind-signed ballot. Spec in QDP-0021
    §6.2."""
    election_id: str
    vrq_public_id: str
    checkin_event_id: str
    blinded_ballot_token_hex: str
    blinding_key_fingerprint: str
    timestamp: int
    vrq_signature_hex: Optional[str] = None

    def to_signable_dict(self) -> dict:
        return {
            "electionId":           self.election_id,
            "vrqPublicId":          self.vrq_public_id,
            "checkinEventId":       self.checkin_event_id,
            "blindedBallotToken":   self.blinded_ballot_token_hex,
            "blindingKeyFingerprint": self.blinding_key_fingerprint,
            "timestamp":            self.timestamp,
        }

    def canonical_bytes(self) -> bytes:
        return json.dumps(
            self.to_signable_dict(), separators=(",", ":")
        ).encode("utf-8")

    def to_full_dict(self) -> dict:
        d = self.to_signable_dict()
        d["vrqSignature"] = self.vrq_signature_hex or ""
        return d
