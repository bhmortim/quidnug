"""Verify a compact Merkle inclusion proof (QDP-0010).

When a node publishes an anchor gossip message, it optionally attaches
a Merkle proof that binds the anchor transaction to a published
``Block.transactions_root``. Downstream peers verify the proof locally
— they do not need the full origin block.

This example builds a small 4-leaf tree, constructs a proof, and
verifies it. In production you'd pull the frames from
``AnchorGossipMessage.merkle_proof`` and the root from the
corresponding ``origin_block.transactions_root``.
"""

from __future__ import annotations

import hashlib

from quidnug import MerkleProofFrame, verify_inclusion_proof


def main() -> None:
    leaves = [hashlib.sha256(f"tx-{i}".encode()).digest() for i in range(4)]
    lvl0_pair0 = hashlib.sha256(leaves[0] + leaves[1]).digest()
    lvl0_pair1 = hashlib.sha256(leaves[2] + leaves[3]).digest()
    root = hashlib.sha256(lvl0_pair0 + lvl0_pair1).digest()

    # Build proof for tx at index 2. Its sibling at level 0 is leaves[3]
    # on the right side; at level 1 the sibling is the left subtree.
    frames = [
        MerkleProofFrame(hash=leaves[3].hex(), side="right"),
        MerkleProofFrame(hash=lvl0_pair0.hex(), side="left"),
    ]
    ok = verify_inclusion_proof(b"tx-2", frames, root.hex())
    print(f"Proof verifies: {ok}")

    # Sanity: a tampered leaf does not verify.
    tampered = verify_inclusion_proof(b"tx-forged", frames, root.hex())
    print(f"Tampered proof rejected: {not tampered}")


if __name__ == "__main__":
    main()
