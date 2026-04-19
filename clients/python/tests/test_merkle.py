"""Tests for quidnug.merkle — inclusion proof verification."""

from __future__ import annotations

import hashlib

import pytest

from quidnug import MerkleProofFrame, verify_inclusion_proof
from quidnug.errors import CryptoError, ValidationError
from quidnug.merkle import leaf_hash


def _sha(data: bytes) -> bytes:
    return hashlib.sha256(data).digest()


# --- Known-good small tree ------------------------------------------------


def test_single_sibling_right():
    """Tree: leaf is left, sibling is right."""
    tx = b"transaction-1"
    sibling = _sha(b"transaction-2")
    our_leaf = _sha(tx)
    root = _sha(our_leaf + sibling)
    frames = [MerkleProofFrame(hash=sibling.hex(), side="right")]
    assert verify_inclusion_proof(tx, frames, root.hex()) is True


def test_single_sibling_left():
    """Tree: leaf is right child; sibling is on the left."""
    tx = b"transaction-2"
    sibling = _sha(b"transaction-1")
    our_leaf = _sha(tx)
    root = _sha(sibling + our_leaf)
    frames = [MerkleProofFrame(hash=sibling.hex(), side="left")]
    assert verify_inclusion_proof(tx, frames, root.hex()) is True


def test_four_leaf_tree():
    """Deterministic 4-leaf tree, proving leaf index 2."""
    leaves = [_sha(f"tx-{i}".encode()) for i in range(4)]
    lvl0_pair0 = _sha(leaves[0] + leaves[1])
    lvl0_pair1 = _sha(leaves[2] + leaves[3])
    root = _sha(lvl0_pair0 + lvl0_pair1)

    tx = b"tx-2"
    # From leaf[2]: sibling is leaves[3] on right, then pair0 on left.
    frames = [
        MerkleProofFrame(hash=leaves[3].hex(), side="right"),
        MerkleProofFrame(hash=lvl0_pair0.hex(), side="left"),
    ]
    assert verify_inclusion_proof(tx, frames, root.hex()) is True


# --- Failure modes ---------------------------------------------------------


def test_tampered_tx_fails():
    tx = b"tx-2"
    sibling = _sha(b"tx-3")
    our_leaf = _sha(tx)
    root = _sha(our_leaf + sibling)
    frames = [MerkleProofFrame(hash=sibling.hex(), side="right")]
    # Verify original OK, tampered fails.
    assert verify_inclusion_proof(tx, frames, root.hex()) is True
    assert verify_inclusion_proof(b"tampered-tx", frames, root.hex()) is False


def test_wrong_root_returns_false_not_exception():
    tx = b"tx-a"
    sibling = _sha(b"tx-b")
    frames = [MerkleProofFrame(hash=sibling.hex(), side="right")]
    # Use zero-root: should return False rather than raise.
    assert verify_inclusion_proof(tx, frames, "00" * 32) is False


def test_malformed_frame_raises_validation_error():
    tx = b"tx"
    with pytest.raises(ValidationError):
        verify_inclusion_proof(
            tx,
            [{"hash": "nothex", "side": "right"}],
            "00" * 32,
        )
    with pytest.raises(ValidationError):
        verify_inclusion_proof(
            tx,
            [{"hash": "aa" * 32, "side": "middle"}],  # bad side
            "00" * 32,
        )


def test_empty_tx_raises_crypto_error():
    with pytest.raises(CryptoError):
        verify_inclusion_proof(b"", [], "00" * 32)


def test_proof_frame_accepts_dict_form():
    tx = b"tx-1"
    sibling = _sha(b"tx-2")
    leaf = _sha(tx)
    root = _sha(leaf + sibling)
    frames = [{"hash": sibling.hex(), "side": "right"}]
    assert verify_inclusion_proof(tx, frames, root.hex()) is True


def test_leaf_hash_matches_sha256():
    assert leaf_hash(b"hello") == _sha(b"hello")
