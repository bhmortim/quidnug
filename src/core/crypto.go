package main

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"log"
	"math/big"

	"crypto/rand"
)

// calculateBlockHash calculates the hash for a block
func calculateBlockHash(block Block) string {
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
		TrustProof:   block.TrustProof,
		PrevHash:     block.PrevHash,
	})

	hash := sha256.Sum256(blockData)
	return hex.EncodeToString(hash[:])
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

// GetPublicKeyHex returns the hex-encoded public key in uncompressed format
func (node *QuidnugNode) GetPublicKeyHex() string {
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
		log.Printf("Failed to decode public key hex: %v", err)
		return false
	}

	// Unmarshal the public key
	x, y := elliptic.Unmarshal(elliptic.P256(), publicKeyBytes)
	if x == nil {
		log.Printf("Failed to unmarshal public key")
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
		log.Printf("Failed to decode signature hex: %v", err)
		return false
	}

	// For P-256, signature should be 64 bytes (32 for r, 32 for s)
	if len(signatureBytes) != 64 {
		log.Printf("Invalid signature length: expected 64, got %d", len(signatureBytes))
		return false
	}

	r := new(big.Int).SetBytes(signatureBytes[:32])
	s := new(big.Int).SetBytes(signatureBytes[32:])

	// Hash the data
	hash := sha256.Sum256(data)

	// Verify the signature
	return ecdsa.Verify(publicKey, hash[:], r, s)
}
