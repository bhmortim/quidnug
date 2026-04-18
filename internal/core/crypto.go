package core

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"math/big"
)

// GetBlockSignableData returns canonical bytes for signing a block.
// This excludes the Hash field (not set during signing) and ValidatorSigs
// (signatures should not sign themselves).
//
// NOTE for QDP-0001: Block.NonceCheckpoints is deliberately NOT
// included in the signable envelope in the foundation / Phase-0
// deployment. Including it is a hard-fork boundary and will be done
// by a coordinated network-wide switch-over (QDP-0001 §10.2). Until
// then, checkpoints are populated for local ledger bookkeeping only.
func GetBlockSignableData(block Block) []byte {
	// Create a copy of TrustProof without signatures for signing
	trustProofForSigning := TrustProof{
		TrustDomain:             block.TrustProof.TrustDomain,
		ValidatorID:             block.TrustProof.ValidatorID,
		ValidatorPublicKey:      block.TrustProof.ValidatorPublicKey,
		ValidatorTrustInCreator: block.TrustProof.ValidatorTrustInCreator,
		// ValidatorSigs intentionally excluded - signatures don't sign themselves
		ConsensusData:  block.TrustProof.ConsensusData,
		ValidationTime: block.TrustProof.ValidationTime,
	}

	blockData, _ := json.Marshal(struct {
		Index        int64
		Timestamp    int64
		Transactions []interface{}
		TrustProof   TrustProof
		PrevHash     string
	}{
		Index:        block.Index,
		Timestamp:    block.Timestamp,
		Transactions: block.Transactions,
		TrustProof:   trustProofForSigning,
		PrevHash:     block.PrevHash,
	})

	return blockData
}

// calculateBlockHash calculates the hash for a block.
//
// Block.Transactions is typed as []interface{}, so json.Marshal
// produces struct-declaration-order bytes for typed wrapper structs
// (TrustTransaction, AnchorTransaction, etc.) but alphabetical order
// for the map[string]interface{} values that result from a JSON
// round-trip. The hash must be stable across both shapes — otherwise
// any process that serializes, transmits, and re-hashes a block
// (cross-node block sync, QDP-0003 anchor gossip, any future replay
// diagnostic) would compute a different hash for the same logical
// block.
//
// We canonicalize by round-tripping through map[string]interface{}
// ourselves. The resulting bytes are always alphabetical-order JSON
// regardless of the input's typed shape. This is a behavior change
// vs. the pre-canonicalization impl but is internal (no on-wire
// hash format change — block hashes are stored AND verified with
// the same function).
func calculateBlockHash(block Block) string {
	blockData, _ := canonicalBlockBytes(block)
	hash := sha256.Sum256(blockData)
	return hex.EncodeToString(hash[:])
}

// canonicalBlockBytes marshals the block's hashable fields in a form
// that's stable under JSON round-tripping. See calculateBlockHash for
// the rationale.
func canonicalBlockBytes(block Block) ([]byte, error) {
	// Stage 1: marshal the typed structure.
	typed, err := json.Marshal(struct {
		Index        int64
		Timestamp    int64
		Transactions []interface{}
		TrustProof   TrustProof
		PrevHash     string
	}{
		Index:        block.Index,
		Timestamp:    block.Timestamp,
		Transactions: block.Transactions,
		TrustProof:   block.TrustProof,
		PrevHash:     block.PrevHash,
	})
	if err != nil {
		return nil, err
	}
	// Stage 2: round-trip through interface{} / map so every sub-
	// value is normalized to alphabetical key ordering. Re-marshaling
	// the resulting value produces canonical bytes.
	var generic interface{}
	if err := json.Unmarshal(typed, &generic); err != nil {
		return nil, err
	}
	return json.Marshal(generic)
}


// SignData signs data with the node's private key
func (node *QuidnugNode) SignData(data []byte) ([]byte, error) {
	hash := sha256.Sum256(data)

	r, s, err := ecdsa.Sign(rand.Reader, node.PrivateKey, hash[:])
	if err != nil {
		return nil, err
	}

	// Pad r and s to 32 bytes each for P-256 (64 bytes total)
	signature := make([]byte, 64)
	rBytes := r.Bytes()
	sBytes := s.Bytes()
	copy(signature[32-len(rBytes):32], rBytes)
	copy(signature[64-len(sBytes):64], sBytes)

	return signature, nil
}

// GetPublicKeyHex returns the hex-encoded public key in uncompressed format.
// Returns an empty string when the node has no public key set (e.g. test nodes
// constructed without NewQuidnugNode).
func (node *QuidnugNode) GetPublicKeyHex() string {
	if node == nil || node.PublicKey == nil {
		return ""
	}
	publicKeyBytes := elliptic.Marshal(node.PublicKey.Curve, node.PublicKey.X, node.PublicKey.Y)
	return hex.EncodeToString(publicKeyBytes)
}

// VerifySignature verifies an ECDSA P-256 signature
// publicKeyHex: hex-encoded public key in uncompressed format (65 bytes: 0x04 || X || Y)
// data: the data that was signed
// signatureHex: hex-encoded signature (64 bytes: r || s, each padded to 32 bytes)
func VerifySignature(publicKeyHex string, data []byte, signatureHex string) bool {
	if publicKeyHex == "" || signatureHex == "" {
		return false
	}

	// Decode public key from hex
	publicKeyBytes, err := hex.DecodeString(publicKeyHex)
	if err != nil {
		if logger != nil {
			logger.Debug("Failed to decode public key hex", "error", err)
		}
		return false
	}

	// Unmarshal the public key
	x, y := elliptic.Unmarshal(elliptic.P256(), publicKeyBytes)
	if x == nil {
		if logger != nil {
			logger.Debug("Failed to unmarshal public key")
		}
		return false
	}

	publicKey := &ecdsa.PublicKey{
		Curve: elliptic.P256(),
		X:     x,
		Y:     y,
	}

	// Decode signature from hex
	signatureBytes, err := hex.DecodeString(signatureHex)
	if err != nil {
		if logger != nil {
			logger.Debug("Failed to decode signature hex", "error", err)
		}
		return false
	}

	// For P-256, signature should be 64 bytes (32 for r, 32 for s)
	if len(signatureBytes) != 64 {
		if logger != nil {
			logger.Debug("Invalid signature length", "expected", 64, "got", len(signatureBytes))
		}
		return false
	}

	r := new(big.Int).SetBytes(signatureBytes[:32])
	s := new(big.Int).SetBytes(signatureBytes[32:])

	// Hash the data
	hash := sha256.Sum256(data)

	// Verify the signature
	return ecdsa.Verify(publicKey, hash[:], r, s)
}
