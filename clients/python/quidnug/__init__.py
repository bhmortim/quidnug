"""Quidnug Python SDK — client library for a decentralized trust protocol.

Covers the full v2.x protocol surface:
    - Identity + Trust + Title (v1 surface, plus improvements)
    - Event streams
    - Key lifecycle: Anchors (rotation, invalidation, epoch-cap)
    - Guardian sets + Recovery + Resignation (QDP-0002 / QDP-0006)
    - Cross-domain fingerprint + anchor gossip (QDP-0003)
    - Push gossip submit (QDP-0005)
    - K-of-K bootstrap snapshots (QDP-0008)
    - Fork-block activation (QDP-0009)
    - Compact Merkle proof verification (QDP-0010)

Typical usage::

    from quidnug import QuidnugClient, Quid

    client = QuidnugClient("http://localhost:8080")
    alice = Quid.generate()
    client.register_identity(alice, name="Alice", home_domain="contractors.home")
    client.grant_trust(alice, trustee="bob", level=0.9, domain="contractors.home")
    result = client.get_trust("alice", "bob", domain="contractors.home")
    print(result.trust_level, result.path)
"""

from quidnug.crypto import Quid, canonical_bytes, sign_bytes, verify_signature
from quidnug.client import QuidnugClient
from quidnug.errors import (
    QuidnugError,
    ValidationError,
    ConflictError,
    UnavailableError,
    NodeError,
    CryptoError,
)
from quidnug.types import (
    TrustEdge,
    IdentityRecord,
    Event,
    Anchor,
    GuardianSet,
    GuardianRef,
    GuardianSetUpdate,
    GuardianRecoveryInit,
    GuardianRecoveryVeto,
    GuardianRecoveryCommit,
    GuardianResignation,
    OwnershipStake,
    Title,
    TrustResult,
    Block,
    DomainFingerprint,
    AnchorGossipMessage,
    NonceSnapshot,
    ForkBlock,
    MerkleProofFrame,
)
from quidnug.merkle import verify_inclusion_proof

def __getattr__(name: str):
    # `AsyncQuidnugClient` pulls in httpx, which is an optional
    # extra (`quidnug[async]`). Defer the import until first access
    # so the rest of the package stays usable without it.
    if name == "AsyncQuidnugClient":
        from quidnug.async_client import AsyncQuidnugClient as _A
        return _A
    raise AttributeError(f"module 'quidnug' has no attribute {name!r}")


__version__ = "2.0.0"
__all__ = [
    "__version__",
    "QuidnugClient",
    "AsyncQuidnugClient",
    "Quid",
    "canonical_bytes",
    "sign_bytes",
    "verify_signature",
    "verify_inclusion_proof",
    "QuidnugError",
    "ValidationError",
    "ConflictError",
    "UnavailableError",
    "NodeError",
    "CryptoError",
    "TrustEdge",
    "IdentityRecord",
    "Event",
    "Anchor",
    "GuardianSet",
    "GuardianRef",
    "GuardianSetUpdate",
    "GuardianRecoveryInit",
    "GuardianRecoveryVeto",
    "GuardianRecoveryCommit",
    "GuardianResignation",
    "OwnershipStake",
    "Title",
    "TrustResult",
    "Block",
    "DomainFingerprint",
    "AnchorGossipMessage",
    "NonceSnapshot",
    "ForkBlock",
    "MerkleProofFrame",
]
