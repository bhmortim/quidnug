package client

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
)

// VerifyInclusionProof reconstructs a QDP-0010 compact Merkle root
// from a leaf transaction and proof frames, and compares to the
// expected root.
//
//	leaf   = sha256(txBytes)
//	parent = sha256(sibling || self) if side == "left"
//	         sha256(self || sibling) if side == "right"
//	root   = parent after walking every frame
//
// Returns (true, nil) on a clean verification, (false, nil) when the
// reconstructed root differs (wrong proof or tampered leaf), and a
// ValidationError for malformed inputs.
func VerifyInclusionProof(txBytes []byte, frames []MerkleProofFrame, expectedRoot string) (bool, error) {
	if len(txBytes) == 0 {
		return false, newCryptoError("txBytes is empty")
	}
	rootBytes, err := hex.DecodeString(expectedRoot)
	if err != nil {
		return false, newValidationError("expectedRoot is not valid hex: " + err.Error())
	}
	if len(rootBytes) != 32 {
		return false, newValidationError(fmt.Sprintf("expectedRoot must be 32 bytes (got %d)", len(rootBytes)))
	}

	h := sha256.Sum256(txBytes)
	current := h[:]
	for i, f := range frames {
		sib, err := hex.DecodeString(f.Hash)
		if err != nil {
			return false, newValidationError(fmt.Sprintf("frame %d hash hex: %v", i, err))
		}
		if len(sib) != 32 {
			return false, newValidationError(fmt.Sprintf("frame %d hash must be 32 bytes (got %d)", i, len(sib)))
		}
		switch f.Side {
		case "left":
			parent := sha256.Sum256(append(append([]byte{}, sib...), current...))
			current = parent[:]
		case "right":
			parent := sha256.Sum256(append(append([]byte{}, current...), sib...))
			current = parent[:]
		default:
			return false, newValidationError(fmt.Sprintf("frame %d: side must be 'left' or 'right', got %q", i, f.Side))
		}
	}
	return bytes.Equal(current, rootBytes), nil
}
