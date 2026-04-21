"""Typed wire dataclasses for every v1.0 transaction type.

These are the Python counterpart to ``pkg/client/types_wire.go``
in the Go SDK. Each class mirrors a struct in
``internal/core/types.go`` (or the privacy / moderation files)
with EXACTLY the same field order and JSON tag names.

Why this matters
----------------

The reference node verifies signatures by calling
``json.Marshal`` on the typed tx struct with ``Signature``
cleared. Go's ``encoding/json`` emits fields in struct
declaration order. For the signature to round-trip, the Python
SDK MUST produce identical bytes.

Python dicts are insertion-ordered since 3.7, and
``dataclasses.asdict`` preserves declaration order. We exploit
this: building a dataclass with the field order matching the Go
struct + serializing via ``json.dumps(..., sort_keys=False,
separators=(",", ":"), ensure_ascii=False)`` produces bytes
byte-identical to Go's ``json.Marshal`` on the typed struct.

Field name mapping
------------------

Go uses camelCase json tags (``"trustDomain"``). Python
dataclasses use snake_case by convention (``trust_domain``).
We carry the camelCase name in ``WIRE_FIELD_ORDER`` on each
class so ``canonical_signable_bytes()`` can assemble the dict
in the correct order with the correct key names without
reflection trickery.

Per-type ID derivation follows the seed fields the server uses
in ``internal/core/transactions.go`` (e.g., ``AddTrustTransaction``
hashes ``(Truster, Trustee, TrustLevel, TrustDomain, Timestamp)``).
"""

from __future__ import annotations

import hashlib
import json
from dataclasses import dataclass, field
from typing import Any, Dict, List, Mapping, Optional, Sequence, Tuple

# ---------------------------------------------------------------
# Shared serialization helper
# ---------------------------------------------------------------


def _emit_signable(fields: Sequence[Tuple[str, Any, bool]]) -> bytes:
    """Build canonical signable bytes from an ordered field list.

    ``fields`` is a sequence of (json_key, value, omitempty_flag)
    tuples in struct declaration order. Empty values are omitted
    when ``omitempty_flag`` is True and the value is zero/empty
    (matching Go's ``omitempty`` semantics).

    Float values are serialized the way Go's ``encoding/json``
    does: integer-valued finite floats drop the trailing ``.0``
    so ``1.0`` emits as ``"1"``. Python's default
    ``json.dumps(1.0)`` returns ``"1.0"``, which diverges.

    Returns UTF-8 bytes of the compact JSON representation.
    """
    d: Dict[str, Any] = {}
    for key, value, omitempty in fields:
        if omitempty and _is_zero(value):
            continue
        d[key] = _go_compat_value(value)
    return json.dumps(
        d,
        separators=(",", ":"),
        sort_keys=False,
        ensure_ascii=False,
    ).encode("utf-8")


def _go_compat_value(v: Any) -> Any:
    """Normalize a value to match Go's ``encoding/json`` output.

    Two divergences matter:

    1. Floats: Go emits ``float64(1.0)`` as ``"1"``, Python
       emits it as ``"1.0"``. For integer-valued finite floats
       in int64 range we cast to int so Python's JSON encoder
       emits the integer form.

    2. Map-key ordering: Go's ``encoding/json`` sorts
       ``map[string]interface{}`` keys alphabetically, while
       Python's ``json.dumps(..., sort_keys=False)`` preserves
       insertion order. Any nested dict that will be signed (and
       re-marshaled server-side into a Go ``map``) must be
       emitted with alphabetically sorted keys to match.

    Struct-typed fields are NOT sorted; their order is dictated
    by Go struct field declaration order and preserved by the
    explicit ``(key, value, omitempty)`` field tuple in each
    ``signable_bytes`` method.

    Strings, ints, bools, None, and non-integer floats pass
    through unchanged. Dicts are sorted; lists recurse.
    """
    if isinstance(v, bool):
        return v
    if isinstance(v, float):
        if v == v and v != float("inf") and v != float("-inf"):
            if v == int(v) and abs(v) < 1e15:
                return int(v)
        return v
    if isinstance(v, dict):
        # Sort to match Go's encoding/json map marshal order.
        return {k: _go_compat_value(sub) for k, sub in sorted(v.items())}
    if isinstance(v, list):
        return [_go_compat_value(x) for x in v]
    return v


def _is_zero(v: Any) -> bool:
    """Match Go's ``omitempty`` zero check."""
    if v is None:
        return True
    if v == "":
        return True
    if v == 0:
        return True
    if v is False:
        return True
    if isinstance(v, (list, tuple, dict)) and not v:
        return True
    return False


def _sha256_hex(data: bytes) -> str:
    return hashlib.sha256(data).hexdigest()


def _seed_id(fields: Sequence[Tuple[str, Any]]) -> str:
    """Hash a Go-struct-ordered JSON of ``fields`` (key, value)
    pairs. Matches ``AddXxxTransaction``'s ID-derivation seed
    format in ``internal/core/transactions.go``.

    Applies the same Go-compat float normalization as
    ``_emit_signable`` so IDs match across SDKs for
    integer-valued float fields (e.g., ``trust_level == 1.0``).
    """
    # Capitalized keys because the Go seed struct uses Go field
    # names (no json tags).
    d: Dict[str, Any] = {k: _go_compat_value(v) for k, v in fields}
    payload = json.dumps(
        d,
        separators=(",", ":"),
        sort_keys=False,
        ensure_ascii=False,
    ).encode("utf-8")
    return _sha256_hex(payload)


# ---------------------------------------------------------------
# TRUST
# ---------------------------------------------------------------


@dataclass
class TrustTx:
    """Mirror of ``core.TrustTransaction``."""

    trust_domain: str = ""
    timestamp: int = 0
    public_key: str = ""
    truster: str = ""
    trustee: str = ""
    trust_level: float = 0.0
    nonce: int = 0
    description: str = ""
    valid_until: int = 0
    id: str = ""
    signature: str = ""

    def derive_id(self) -> str:
        return _seed_id([
            ("Truster", self.truster),
            ("Trustee", self.trustee),
            ("TrustLevel", self.trust_level),
            ("TrustDomain", self.trust_domain),
            ("Timestamp", self.timestamp),
        ])

    def signable_bytes(self) -> bytes:
        return _emit_signable([
            ("id", self.id, False),
            ("type", "TRUST", False),
            ("trustDomain", self.trust_domain, False),
            ("timestamp", self.timestamp, False),
            ("signature", "", False),
            ("publicKey", self.public_key, False),
            ("truster", self.truster, False),
            ("trustee", self.trustee, False),
            ("trustLevel", self.trust_level, False),
            ("nonce", self.nonce, False),
            ("description", self.description, True),
            ("validUntil", self.valid_until, True),
        ])

    def to_wire(self) -> Dict[str, Any]:
        """Signed wire form (with signature filled). Used when
        submitting to the node."""
        d = json.loads(self.signable_bytes())
        d["signature"] = self.signature
        return d


# ---------------------------------------------------------------
# IDENTITY
# ---------------------------------------------------------------


@dataclass
class IdentityTx:
    """Mirror of ``core.IdentityTransaction``."""

    trust_domain: str = ""
    timestamp: int = 0
    public_key: str = ""
    quid_id: str = ""
    name: str = ""
    description: str = ""
    attributes: Optional[Dict[str, Any]] = None
    creator: str = ""
    update_nonce: int = 0
    home_domain: str = ""
    id: str = ""
    signature: str = ""

    def derive_id(self) -> str:
        return _seed_id([
            ("QuidID", self.quid_id),
            ("Name", self.name),
            ("Creator", self.creator),
            ("TrustDomain", self.trust_domain),
            ("UpdateNonce", self.update_nonce),
            ("Timestamp", self.timestamp),
        ])

    def signable_bytes(self) -> bytes:
        return _emit_signable([
            ("id", self.id, False),
            ("type", "IDENTITY", False),
            ("trustDomain", self.trust_domain, False),
            ("timestamp", self.timestamp, False),
            ("signature", "", False),
            ("publicKey", self.public_key, False),
            ("quidId", self.quid_id, False),
            ("name", self.name, False),
            ("description", self.description, True),
            ("attributes", self.attributes, True),
            ("creator", self.creator, False),
            ("updateNonce", self.update_nonce, False),
            ("homeDomain", self.home_domain, True),
        ])

    def to_wire(self) -> Dict[str, Any]:
        d = json.loads(self.signable_bytes())
        d["signature"] = self.signature
        return d


# ---------------------------------------------------------------
# TITLE
# ---------------------------------------------------------------


@dataclass
class OwnershipStake:
    owner_id: str = ""
    percentage: float = 0.0
    stake_type: str = ""

    def to_dict(self) -> Dict[str, Any]:
        d: Dict[str, Any] = {
            "ownerId": self.owner_id,
            "percentage": self.percentage,
        }
        if self.stake_type:
            d["stakeType"] = self.stake_type
        return d


@dataclass
class TitleTx:
    """Mirror of ``core.TitleTransaction``."""

    trust_domain: str = ""
    timestamp: int = 0
    public_key: str = ""
    asset_id: str = ""
    owners: List[OwnershipStake] = field(default_factory=list)
    previous_owners: List[OwnershipStake] = field(default_factory=list)
    signatures: Dict[str, str] = field(default_factory=dict)
    expiry_date: int = 0
    title_type: str = ""
    id: str = ""
    signature: str = ""

    def derive_id(self) -> str:
        # Go marshals []OwnershipStake with SEC1-compat field order.
        # We emit via to_dict() list to preserve it.
        owners_list = [o.to_dict() for o in self.owners]
        return _seed_id([
            ("AssetID", self.asset_id),
            ("Owners", owners_list),
            ("TrustDomain", self.trust_domain),
            ("Timestamp", self.timestamp),
        ])

    def signable_bytes(self) -> bytes:
        fields = [
            ("id", self.id, False),
            ("type", "TITLE", False),
            ("trustDomain", self.trust_domain, False),
            ("timestamp", self.timestamp, False),
            ("signature", "", False),
            ("publicKey", self.public_key, False),
            ("assetId", self.asset_id, False),
            ("owners", [o.to_dict() for o in self.owners], False),
        ]
        if self.previous_owners:
            fields.append(("previousOwners",
                           [o.to_dict() for o in self.previous_owners], True))
        fields.append(("signatures", self.signatures, False))
        if self.expiry_date:
            fields.append(("expiryDate", self.expiry_date, True))
        if self.title_type:
            fields.append(("titleType", self.title_type, True))
        return _emit_signable(fields)

    def to_wire(self) -> Dict[str, Any]:
        d = json.loads(self.signable_bytes())
        d["signature"] = self.signature
        return d


# ---------------------------------------------------------------
# EVENT
# ---------------------------------------------------------------


@dataclass
class EventTx:
    """Mirror of ``core.EventTransaction``."""

    trust_domain: str = ""
    timestamp: int = 0
    public_key: str = ""
    subject_id: str = ""
    subject_type: str = ""
    sequence: int = 0
    event_type: str = ""
    payload: Optional[Dict[str, Any]] = None
    payload_cid: str = ""
    previous_event_id: str = ""
    id: str = ""
    signature: str = ""

    def derive_id(self) -> str:
        return _seed_id([
            ("SubjectID", self.subject_id),
            ("EventType", self.event_type),
            ("Sequence", self.sequence),
            ("TrustDomain", self.trust_domain),
            ("Timestamp", self.timestamp),
        ])

    def signable_bytes(self) -> bytes:
        fields = [
            ("id", self.id, False),
            ("type", "EVENT", False),
            ("trustDomain", self.trust_domain, False),
            ("timestamp", self.timestamp, False),
            ("signature", "", False),
            ("publicKey", self.public_key, False),
            ("subjectId", self.subject_id, False),
            ("subjectType", self.subject_type, False),
            ("sequence", self.sequence, False),
            ("eventType", self.event_type, False),
        ]
        if self.payload is not None:
            fields.append(("payload", self.payload, True))
        if self.payload_cid:
            fields.append(("payloadCid", self.payload_cid, True))
        if self.previous_event_id:
            fields.append(("previousEventId", self.previous_event_id, True))
        return _emit_signable(fields)

    def to_wire(self) -> Dict[str, Any]:
        d = json.loads(self.signable_bytes())
        d["signature"] = self.signature
        return d


# ---------------------------------------------------------------
# Sign helper
# ---------------------------------------------------------------


def sign_wire(tx: Any, quid: Any) -> Dict[str, Any]:
    """Sign a wire dataclass in-place and return the wire dict.

    ``tx`` must have ``derive_id()``, ``signable_bytes()``, and
    ``to_wire()``. ``quid`` must have ``.sign(data: bytes) -> str``
    returning an IEEE-1363 hex signature.

    Side effect: sets ``tx.id`` and ``tx.signature``.
    """
    if not quid.has_private_key:
        from quidnug.errors import CryptoError
        raise CryptoError("quid is read-only")
    tx.id = tx.derive_id()
    tx.signature = quid.sign(tx.signable_bytes())
    return tx.to_wire()


__all__ = [
    "TrustTx",
    "IdentityTx",
    "TitleTx",
    "OwnershipStake",
    "EventTx",
    "sign_wire",
]
