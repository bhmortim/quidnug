// Package core — merkle.go
//
// Compact Merkle inclusion proofs for block transactions
// (QDP-0010 / H2). Binary SHA-256 Merkle tree with last-leaf
// duplication for odd counts (Bitcoin convention, unambiguous
// and simple).
//
// Leaf canonicalization uses the same map-round-trip pattern as
// calculateBlockHash (see QDP-0003 §8.3): the transaction is
// marshaled, unmarshaled into a generic interface{}, then
// marshaled again. Otherwise typed-struct and generic-map
// encodings of the same transaction produce different bytes
// and the receiver's recomputed leaf won't match the producer's.
package core

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math"
)

// ----- Errors --------------------------------------------------------------

var (
	ErrMerkleEmptyBlock     = errors.New("merkle: cannot build tree from empty transactions")
	ErrMerkleIndexOutOfRange = errors.New("merkle: proof index out of range")
	ErrMerkleProofTooLong   = errors.New("merkle: proof exceeds maximum path length")
	ErrMerkleBadSide        = errors.New("merkle: proof frame has unknown side")
	ErrMerkleBadHash        = errors.New("merkle: proof frame hash malformed")
	ErrMerkleProofMismatch  = errors.New("merkle: proof does not reconstruct the expected root")
)

// ----- Constants -----------------------------------------------------------

// MerkleMaxTxsPerBlock is an upper bound on the number of
// transactions we'll build a tree over. A proof longer than
// ceil(log2(this)) is rejected as malformed.
const MerkleMaxTxsPerBlock = 4096

// MerkleSideLeft / MerkleSideRight are the two legal values for
// a proof frame's Side field.
const (
	MerkleSideLeft  = "left"
	MerkleSideRight = "right"
)

// ----- Wire types ----------------------------------------------------------

// MerkleProofFrame is one step in an inclusion proof. Hash is
// the sibling at that level; Side tells the verifier which
// side of the concat the sibling is on.
type MerkleProofFrame struct {
	Hash string `json:"hash"` // hex-encoded sha256 (64 chars)
	Side string `json:"side"` // "left" | "right"
}

// ----- Leaf canonicalization -----------------------------------------------

// canonicalTxBytes produces deterministic byte representation
// for a transaction across environments. Typed structs and
// their map-decoded counterparts must produce identical leaf
// hashes — otherwise a producer's tree disagrees with a
// receiver that decoded the same block via JSON.
func canonicalTxBytes(tx interface{}) ([]byte, error) {
	tmp, err := json.Marshal(tx)
	if err != nil {
		return nil, fmt.Errorf("merkle: first marshal: %w", err)
	}
	var generic interface{}
	if err := json.Unmarshal(tmp, &generic); err != nil {
		return nil, fmt.Errorf("merkle: unmarshal to generic: %w", err)
	}
	return json.Marshal(generic)
}

// leafHash returns sha256(canonicalTxBytes(tx)).
func leafHash(tx interface{}) (string, error) {
	data, err := canonicalTxBytes(tx)
	if err != nil {
		return "", err
	}
	h := sha256.Sum256(data)
	return hex.EncodeToString(h[:]), nil
}

// ----- Tree construction ---------------------------------------------------

// MerkleRoot returns the root of a binary Merkle tree over the
// canonical bytes of each transaction. Empty slice returns
// empty string (not sha256("")); the Empty case is treated
// specially by block-level validation.
func MerkleRoot(txs []interface{}) (string, error) {
	if len(txs) == 0 {
		return "", nil
	}
	if len(txs) > MerkleMaxTxsPerBlock {
		return "", fmt.Errorf("merkle: too many transactions: %d > %d", len(txs), MerkleMaxTxsPerBlock)
	}

	// Level 0: leaf hashes.
	level := make([]string, len(txs))
	for i, tx := range txs {
		h, err := leafHash(tx)
		if err != nil {
			return "", err
		}
		level[i] = h
	}

	for len(level) > 1 {
		next := make([]string, 0, (len(level)+1)/2)
		for i := 0; i < len(level); i += 2 {
			left := level[i]
			right := left
			if i+1 < len(level) {
				right = level[i+1]
			}
			next = append(next, parentHash(left, right))
		}
		level = next
	}
	return level[0], nil
}

// parentHash returns sha256(leftBytes || rightBytes) as hex.
// Inputs are hex-encoded sha256 hashes; we decode, concatenate
// the raw bytes, and hash.
func parentHash(leftHex, rightHex string) string {
	left, _ := hex.DecodeString(leftHex)
	right, _ := hex.DecodeString(rightHex)
	combined := make([]byte, 0, len(left)+len(right))
	combined = append(combined, left...)
	combined = append(combined, right...)
	h := sha256.Sum256(combined)
	return hex.EncodeToString(h[:])
}

// ----- Proof construction --------------------------------------------------

// MerkleProof builds an inclusion proof for the transaction at
// `index` within the full tree. Returns the proof frames that,
// when walked from the target leaf, reconstruct the root.
func MerkleProof(txs []interface{}, index int) ([]MerkleProofFrame, error) {
	if index < 0 || index >= len(txs) {
		return nil, ErrMerkleIndexOutOfRange
	}
	if len(txs) > MerkleMaxTxsPerBlock {
		return nil, fmt.Errorf("merkle: too many transactions: %d > %d", len(txs), MerkleMaxTxsPerBlock)
	}

	// Build the full level-by-level tree, keeping per-level
	// slices so we can pick the sibling at each step.
	levels := [][]string{}
	current := make([]string, len(txs))
	for i, tx := range txs {
		h, err := leafHash(tx)
		if err != nil {
			return nil, err
		}
		current[i] = h
	}
	levels = append(levels, current)
	for len(current) > 1 {
		next := make([]string, 0, (len(current)+1)/2)
		for i := 0; i < len(current); i += 2 {
			left := current[i]
			right := left
			if i+1 < len(current) {
				right = current[i+1]
			}
			next = append(next, parentHash(left, right))
		}
		current = next
		levels = append(levels, current)
	}

	// Walk up the tree, recording sibling at each level.
	proof := make([]MerkleProofFrame, 0)
	idx := index
	for level := 0; level < len(levels)-1; level++ {
		siblingIdx := idx ^ 1
		// Odd-tail duplication: when the actual sibling index
		// is beyond the level's length, the sibling is the
		// node itself (last-duplicate convention).
		if siblingIdx >= len(levels[level]) {
			siblingIdx = idx
		}
		side := MerkleSideRight
		if idx%2 == 1 {
			side = MerkleSideLeft
		}
		proof = append(proof, MerkleProofFrame{
			Hash: levels[level][siblingIdx],
			Side: side,
		})
		idx /= 2
	}
	return proof, nil
}

// ----- Proof verification --------------------------------------------------

// VerifyMerkleProof walks the proof from `leafHex` (hex sha256
// of the canonical tx bytes) and returns the reconstructed
// root. Returns an error if any frame is malformed.
func VerifyMerkleProof(leafHex string, proof []MerkleProofFrame) (string, error) {
	// Cap proof length to prevent amplification.
	if len(proof) > int(math.Ceil(math.Log2(float64(MerkleMaxTxsPerBlock))))+1 {
		return "", ErrMerkleProofTooLong
	}
	if _, err := hex.DecodeString(leafHex); err != nil || len(leafHex) != 64 {
		return "", ErrMerkleBadHash
	}
	cur := leafHex
	for i, frame := range proof {
		if len(frame.Hash) != 64 {
			return "", fmt.Errorf("%w at frame %d: length %d", ErrMerkleBadHash, i, len(frame.Hash))
		}
		if _, err := hex.DecodeString(frame.Hash); err != nil {
			return "", fmt.Errorf("%w at frame %d: %v", ErrMerkleBadHash, i, err)
		}
		switch frame.Side {
		case MerkleSideLeft:
			cur = parentHash(frame.Hash, cur)
		case MerkleSideRight:
			cur = parentHash(cur, frame.Hash)
		default:
			return "", fmt.Errorf("%w: %q at frame %d", ErrMerkleBadSide, frame.Side, i)
		}
	}
	return cur, nil
}

// VerifyTransactionInclusion is the end-to-end inclusion check
// used by gossip-receive code: compute the leaf hash from the
// transaction, walk the proof, and compare against the
// expected root.
func VerifyTransactionInclusion(tx interface{}, proof []MerkleProofFrame, expectedRoot string) error {
	leaf, err := leafHash(tx)
	if err != nil {
		return fmt.Errorf("merkle: compute leaf: %w", err)
	}
	computed, err := VerifyMerkleProof(leaf, proof)
	if err != nil {
		return err
	}
	if computed != expectedRoot {
		return ErrMerkleProofMismatch
	}
	return nil
}
