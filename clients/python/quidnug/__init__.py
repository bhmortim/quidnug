"""Quidnug Python SDK — client library for a decentralized trust protocol.

The synchronous :class:`QuidnugClient` covers the full v2.x protocol surface:
    - Identity + Trust + Title (v1 surface, plus improvements)
    - Event streams
    - Key lifecycle: Anchors (rotation, invalidation, epoch-cap)
    - Guardian sets + Recovery + Resignation (QDP-0002 / QDP-0006)
    - Cross-domain fingerprint + anchor gossip (QDP-0003)
    - Push gossip submit (QDP-0005)
    - K-of-K bootstrap snapshots (QDP-0008)
    - Fork-block activation (QDP-0009)
    - Compact Merkle proof verification (QDP-0010)

The asyncio :class:`AsyncQuidnugClient` mirrors the v1 transaction
surface and the production-critical v2 read paths plus the most
common writes (guardian set updates, anchor gossip, fork-block submit).
See the README for the exact method list and parity status.

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

# AsyncQuidnugClient depends on the optional `httpx` extra. Surface a
# clear ImportError when callers reach for it without the extra
# installed, but don't punish the (much more common) sync-only user.
try:
    from quidnug.async_client import AsyncQuidnugClient
except ImportError:  # pragma: no cover - httpx not installed
    AsyncQuidnugClient = None  # type: ignore[assignment]

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
    Event,
    Anchor,
    GuardianSet,
    GuardianRef,
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
    "Event",
    "Anchor",
    "GuardianSet",
    "GuardianRef",
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
