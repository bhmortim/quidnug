"""Cryptographic primitives for the Quidnug SDK.

The protocol uses ECDSA P-256 (a.k.a. secp256r1 / NIST prime256v1)
with SHA-256 hashing. Signatures are hex-encoded, DER-formatted for
interop with the Go reference implementation.

Canonicalization — critical for cross-implementation signature
verification — follows the round-trip-through-a-generic-object rule
documented at ``schemas/types/canonicalization.md``. This lives here
as ``canonical_bytes(obj, exclude_fields=...)``.
"""

from __future__ import annotations

import hashlib
import json
from dataclasses import dataclass
from typing import Any, Iterable, Optional

from cryptography.exceptions import InvalidSignature
from cryptography.hazmat.primitives import hashes, serialization
from cryptography.hazmat.primitives.asymmetric import ec

from quidnug.errors import CryptoError


# --- Canonicalization -------------------------------------------------------


def canonical_bytes(obj: Any, exclude_fields: Iterable[str] = ()) -> bytes:
    """Return the canonical JSON byte representation of *obj*.

    Matches Go's ``json.Marshal`` → unmarshal to ``interface{}`` →
    ``json.Marshal`` pattern. Fields named in *exclude_fields* are
    removed from the top-level dict before serialization.

    Uses ``sort_keys=False, separators=(',', ':')`` to match Go's
    encoding/json output.
    """
    if hasattr(obj, "__dict__"):
        payload: dict[str, Any] = {k: v for k, v in obj.__dict__.items() if k not in exclude_fields}
    elif isinstance(obj, dict):
        payload = {k: v for k, v in obj.items() if k not in exclude_fields}
    else:
        raise CryptoError(f"Cannot canonicalize type {type(obj).__name__}")

    # Round-trip: marshal → unmarshal to generic → marshal again. The
    # second marshal sorts keys, matching Go's map[string]interface{}
    # re-serialization (encoding/json alphabetizes map keys).
    first = json.dumps(payload, default=_json_default, separators=(",", ":"), sort_keys=False)
    generic = json.loads(first)
    return json.dumps(generic, separators=(",", ":"), sort_keys=True).encode("utf-8")


def _json_default(o: Any) -> Any:
    if hasattr(o, "__dict__"):
        return {k: v for k, v in o.__dict__.items() if not k.startswith("_")}
    if isinstance(o, bytes):
        return o.hex()
    raise TypeError(f"Type {type(o).__name__} not JSON serializable")


# --- Signature helpers ------------------------------------------------------


def sign_bytes(priv: ec.EllipticCurvePrivateKey, data: bytes) -> str:
    """Sign *data* with an ECDSA P-256 private key. Returns hex-encoded DER."""
    sig = priv.sign(data, ec.ECDSA(hashes.SHA256()))
    return sig.hex()


def verify_signature(pub: ec.EllipticCurvePublicKey, data: bytes, sig_hex: str) -> bool:
    """Verify an ECDSA P-256 signature. Returns True on valid signature."""
    try:
        pub.verify(bytes.fromhex(sig_hex), data, ec.ECDSA(hashes.SHA256()))
        return True
    except (InvalidSignature, ValueError):
        return False


# --- Quid identity ---------------------------------------------------------


@dataclass
class Quid:
    """A cryptographic identity (user-generated).

    In Quidnug every principal is a ``Quid`` — people, organizations,
    AI agents, devices, documents. Generated locally by the holder;
    the quid ID is ``sha256(publicKey)[:16]`` in hex.

    Args:
        id: 16-character hex identifier.
        public_key_hex: SEC1 uncompressed hex encoding of the P-256 pubkey.
        private_key_hex: Optional PKCS8 hex encoding of the private key.
            If absent, the Quid is read-only — it can be referenced in
            queries and as a trustee but cannot sign.
    """

    id: str
    public_key_hex: str
    private_key_hex: Optional[str] = None

    # Cached key objects. Not serialized.
    _priv: Optional[ec.EllipticCurvePrivateKey] = None
    _pub: Optional[ec.EllipticCurvePublicKey] = None

    @classmethod
    def generate(cls) -> "Quid":
        """Generate a fresh Quid with a new P-256 keypair."""
        priv = ec.generate_private_key(ec.SECP256R1())
        pub = priv.public_key()
        pub_bytes = pub.public_bytes(
            encoding=serialization.Encoding.X962,
            format=serialization.PublicFormat.UncompressedPoint,
        )
        priv_bytes = priv.private_bytes(
            encoding=serialization.Encoding.DER,
            format=serialization.PrivateFormat.PKCS8,
            encryption_algorithm=serialization.NoEncryption(),
        )
        quid_id = hashlib.sha256(pub_bytes).hexdigest()[:16]
        q = cls(
            id=quid_id,
            public_key_hex=pub_bytes.hex(),
            private_key_hex=priv_bytes.hex(),
        )
        q._priv = priv
        q._pub = pub
        return q

    @classmethod
    def from_private_hex(cls, private_key_hex: str) -> "Quid":
        """Reconstruct a Quid from a PKCS8 DER hex-encoded private key."""
        priv = serialization.load_der_private_key(
            bytes.fromhex(private_key_hex), password=None
        )
        if not isinstance(priv, ec.EllipticCurvePrivateKey):
            raise CryptoError("Private key is not an EC key")
        pub = priv.public_key()
        pub_bytes = pub.public_bytes(
            encoding=serialization.Encoding.X962,
            format=serialization.PublicFormat.UncompressedPoint,
        )
        quid_id = hashlib.sha256(pub_bytes).hexdigest()[:16]
        q = cls(
            id=quid_id,
            public_key_hex=pub_bytes.hex(),
            private_key_hex=private_key_hex,
        )
        q._priv = priv
        q._pub = pub
        return q

    @classmethod
    def from_public_hex(cls, public_key_hex: str) -> "Quid":
        """Reconstruct a read-only Quid from a SEC1 hex public key."""
        pub_bytes = bytes.fromhex(public_key_hex)
        pub = ec.EllipticCurvePublicKey.from_encoded_point(ec.SECP256R1(), pub_bytes)
        quid_id = hashlib.sha256(pub_bytes).hexdigest()[:16]
        q = cls(id=quid_id, public_key_hex=public_key_hex)
        q._pub = pub
        return q

    @property
    def has_private_key(self) -> bool:
        return self._priv is not None

    def sign(self, data: bytes) -> str:
        """Sign raw bytes with this quid's private key."""
        if self._priv is None:
            raise CryptoError("Quid is read-only (no private key)")
        return sign_bytes(self._priv, data)

    def verify(self, data: bytes, sig_hex: str) -> bool:
        """Verify a signature against this quid's public key."""
        if self._pub is None:
            raise CryptoError("Quid has no public key")
        return verify_signature(self._pub, data, sig_hex)


__all__ = [
    "Quid",
    "canonical_bytes",
    "sign_bytes",
    "verify_signature",
]
