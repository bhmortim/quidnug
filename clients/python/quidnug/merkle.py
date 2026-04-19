"""Compact Merkle inclusion-proof verification (QDP-0010).

QDP-0010 adds a ``transactions_root`` to every ``Block`` and allows
anchor gossip messages to carry a Merkle proof that binds the anchor
transaction to that root. Verifying the proof at the receiving end lets
peers accept cross-domain gossip without downloading the full block.

Leaf hashing, frame ordering, and the siblings-only (no-self)
convention must match the Go reference. In particular::

    leaf   = SHA256(tx_bytes)
    frame  = {"hash": hex, "side": "left" | "right"}
    parent = SHA256(sibling || self) if side == "left"
             SHA256(self || sibling) if side == "right"
    root   = parent after walking every frame

The input ``tx_bytes`` is the **canonical** (round-trip-normalized)
signable encoding of the transaction — see
``schemas/types/canonicalization.md`` for the exact rules.
"""

from __future__ import annotations

import hashlib
from dataclasses import dataclass
from typing import Iterable, List, Sequence, Union

from quidnug.errors import CryptoError, ValidationError
from quidnug.types import MerkleProofFrame


# --- Leaf / internal hashing -----------------------------------------------


def leaf_hash(tx_bytes: bytes) -> bytes:
    """SHA-256 of the canonical transaction bytes."""
    return hashlib.sha256(tx_bytes).digest()


def _parent(self_hash: bytes, sibling: bytes, *, side: str) -> bytes:
    if side == "left":
        concat = sibling + self_hash
    elif side == "right":
        concat = self_hash + sibling
    else:
        raise ValidationError(f"invalid proof frame side: {side!r}")
    return hashlib.sha256(concat).digest()


# --- Public verifier -------------------------------------------------------


@dataclass(frozen=True)
class _NormalizedFrame:
    hash: bytes
    side: str


def _normalize_frames(
    frames: Sequence[Union[MerkleProofFrame, dict]],
) -> List[_NormalizedFrame]:
    out: List[_NormalizedFrame] = []
    for i, f in enumerate(frames):
        if isinstance(f, MerkleProofFrame):
            h_raw: object = f.hash
            side = f.side
        elif isinstance(f, dict):
            h_raw = f.get("hash", "")
            side = f.get("side", "")
        else:
            raise ValidationError(f"proof frame {i} is not a MerkleProofFrame or dict")
        if not isinstance(h_raw, str) or not h_raw:
            raise ValidationError(f"proof frame {i} hash must be a hex string")
        try:
            h = bytes.fromhex(h_raw)
        except ValueError as exc:
            raise ValidationError(f"proof frame {i} hash is not valid hex") from exc
        if len(h) != 32:
            raise ValidationError(f"proof frame {i} hash is not 32 bytes (got {len(h)})")
        if side not in ("left", "right"):
            raise ValidationError(f"proof frame {i} side must be 'left' or 'right'")
        out.append(_NormalizedFrame(hash=h, side=side))
    return out


def verify_inclusion_proof(
    tx_bytes: bytes,
    proof_frames: Iterable[Union[MerkleProofFrame, dict]],
    expected_root: Union[str, bytes],
) -> bool:
    """Verify an inclusion proof.

    Parameters
    ----------
    tx_bytes:
        Canonical signable bytes of the transaction being proved.
    proof_frames:
        Ordered list of sibling hashes + sides, from leaf-parent up to
        the root (excluding both leaf and root).
    expected_root:
        The Merkle root published in ``Block.transactions_root``. Hex
        string or 32 raw bytes are both accepted.

    Returns
    -------
    True if the proof reconstructs exactly ``expected_root``, else False.

    Raises
    ------
    ValidationError
        If the proof is malformed (bad hex, wrong side token, wrong hash
        length). A proof that merely does not match a different root
        returns False rather than raising, so callers can distinguish
        "client is wrong" from "server is wrong."
    CryptoError
        If ``tx_bytes`` is empty.
    """
    if not tx_bytes:
        raise CryptoError("tx_bytes must be non-empty")

    if isinstance(expected_root, str):
        try:
            root = bytes.fromhex(expected_root)
        except ValueError as exc:
            raise ValidationError("expected_root is not valid hex") from exc
    else:
        root = bytes(expected_root)
    if len(root) != 32:
        raise ValidationError(f"expected_root must be 32 bytes (got {len(root)})")

    frames = _normalize_frames(list(proof_frames))

    current = leaf_hash(tx_bytes)
    for f in frames:
        current = _parent(current, f.hash, side=f.side)

    return current == root


__all__ = ["verify_inclusion_proof", "leaf_hash"]
