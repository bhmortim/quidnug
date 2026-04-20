// blind_signatures.go — QDP-0021 Phase 1: blind-signature
// ballot issuance protocol layer.
//
// Phase 1 scope (docs/design/0021-blind-signatures.md Phase 1
// + Phase 2 merged): crypto primitives (`pkg/crypto/blindrsa`)
// + BLIND_KEY_ATTESTATION event type + ballot-proof
// verification helper that the elections tally code consumes.
//
// What the node does at Phase 1:
//   - Accept + validate + store BLIND_KEY_ATTESTATION events.
//   - Expose a lookup by (ElectionID, RSAKeyFingerprint) so
//     ballot proofs can resolve their key reference.
//   - Provide VerifyBallotProof helper that:
//       1. Resolves the referenced BlindKeyAttestation
//       2. Confirms the referenced key is still in its
//          ValidFrom/Until window
//       3. Runs the RSA-FDH verification from
//          pkg/crypto/blindrsa.Verify
//
// What the node does NOT do at Phase 1:
//   - Issue ballots (that's the authority-side job; HTTP
//     endpoint lives in the elections authority service,
//     which exists in examples/elections/).
//   - Tally (also examples/elections/clients/tally.py).
//   - Anything about voter check-in or eligibility
//     (that's QDP-0017 consent + domain governance).

package core

import (
	"crypto/rsa"
	"encoding/hex"
	"fmt"
	"math/big"
	"sync"
	"time"

	"github.com/quidnug/quidnug/pkg/crypto/blindrsa"
)

// BlindKeyRegistry stores BLIND_KEY_ATTESTATION events
// indexed by key fingerprint + by election. Lookups are O(1)
// in the common path.
type BlindKeyRegistry struct {
	mu sync.RWMutex

	// byFingerprint: RSAKeyFingerprint -> attestation.
	byFingerprint map[string]BlindKeyAttestationTransaction

	// byElection: ElectionID -> set of fingerprints. Elections
	// typically use one key but the model supports rotation
	// mid-cycle (revoke old, attest new).
	byElection map[string]map[string]bool

	// nonces per authority quid.
	nonces map[string]int64
}

// NewBlindKeyRegistry returns an empty registry.
func NewBlindKeyRegistry() *BlindKeyRegistry {
	return &BlindKeyRegistry{
		byFingerprint: make(map[string]BlindKeyAttestationTransaction),
		byElection:    make(map[string]map[string]bool),
		nonces:        make(map[string]int64),
	}
}

// GetByFingerprint returns the attestation for a key
// fingerprint, or false if not found.
func (r *BlindKeyRegistry) GetByFingerprint(fp string) (BlindKeyAttestationTransaction, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	att, ok := r.byFingerprint[fp]
	return att, ok
}

// GetForElection returns all attestations for a given
// election ID.
func (r *BlindKeyRegistry) GetForElection(electionID string) []BlindKeyAttestationTransaction {
	r.mu.RLock()
	defer r.mu.RUnlock()
	fps := r.byElection[electionID]
	out := make([]BlindKeyAttestationTransaction, 0, len(fps))
	for fp := range fps {
		if att, ok := r.byFingerprint[fp]; ok {
			out = append(out, att)
		}
	}
	return out
}

func (r *BlindKeyRegistry) currentNonce(authority string) int64 {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.nonces[authority]
}

func (r *BlindKeyRegistry) admit(tx BlindKeyAttestationTransaction) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.byFingerprint[tx.RSAKeyFingerprint] = tx
	if r.byElection[tx.ElectionID] == nil {
		r.byElection[tx.ElectionID] = make(map[string]bool)
	}
	r.byElection[tx.ElectionID][tx.RSAKeyFingerprint] = true
	if cur, ok := r.nonces[tx.AuthorityQuid]; !ok || tx.Nonce > cur {
		r.nonces[tx.AuthorityQuid] = tx.Nonce
	}
}

// AddBlindKeyAttestationTransaction validates + stores a
// BLIND_KEY_ATTESTATION event.
func (node *QuidnugNode) AddBlindKeyAttestationTransaction(tx BlindKeyAttestationTransaction) (string, error) {
	if node.BlindKeyRegistry == nil {
		return "", ErrTxTypeUnsupported("BLIND_KEY_ATTESTATION: registry not initialized")
	}
	signed := tx.Signature != ""
	if !signed && tx.Timestamp == 0 {
		tx.Timestamp = time.Now().Unix()
	}
	if !signed {
		tx.Type = TxTypeBlindKeyAttestation
	}
	if !signed && tx.Nonce == 0 {
		tx.Nonce = node.BlindKeyRegistry.currentNonce(tx.AuthorityQuid) + 1
	}
	if !signed && tx.ID == "" {
		tx.ID = seedID(struct {
			ElectionID        string
			AuthorityQuid     string
			RSAKeyFingerprint string
			Nonce             int64
			Timestamp         int64
		}{tx.ElectionID, tx.AuthorityQuid, tx.RSAKeyFingerprint, tx.Nonce, tx.Timestamp})
	}
	if !node.ValidateBlindKeyAttestation(tx) {
		return "", ErrInvalidTx("BLIND_KEY_ATTESTATION")
	}
	node.BlindKeyRegistry.admit(tx)
	return tx.ID, nil
}

// ValidateBlindKeyAttestation sanity-checks the fields +
// signature.
func (node *QuidnugNode) ValidateBlindKeyAttestation(tx BlindKeyAttestationTransaction) bool {
	if tx.ElectionID == "" {
		return false
	}
	if !IsValidQuidID(tx.AuthorityQuid) {
		return false
	}
	if tx.RSAPublicKeyNHex == "" {
		return false
	}
	if tx.RSAPublicKeyE <= 0 {
		return false
	}
	if len(tx.RSAKeyFingerprint) != 64 { // hex of sha256
		return false
	}
	if tx.ValidFrom == 0 || tx.ValidUntil == 0 {
		return false
	}
	if tx.ValidUntil <= tx.ValidFrom {
		return false
	}
	if tx.Nonce <= 0 {
		return false
	}
	if tx.Signature == "" || tx.PublicKey == "" {
		return false
	}

	// Confirm fingerprint matches the declared (n, e).
	n := new(big.Int)
	nBytes, err := hex.DecodeString(tx.RSAPublicKeyNHex)
	if err != nil {
		return false
	}
	n.SetBytes(nBytes)
	rsaPub := &rsa.PublicKey{N: n, E: tx.RSAPublicKeyE}
	computedFP := blindrsa.PublicKeyFingerprint(rsaPub)
	if computedFP != tx.RSAKeyFingerprint {
		return false
	}

	return verifyStructSig(tx.PublicKey, tx.Signature, func() any {
		copy := tx
		copy.Signature = ""
		return copy
	})
}

// VerifyBallotProof verifies that the unblinded RSA-FDH
// signature in `proof` is valid under the BlindKeyAttestation
// referenced by `proof.RSAKeyFingerprint`.
//
// Returns nil on valid; an error describing the failure
// mode otherwise. Used by tally + audit clients.
//
// nowNs is the reference time for the ValidFrom/Until check.
// Pass time.Now().UnixNano() in production; a fixed value in
// tests for determinism.
func (node *QuidnugNode) VerifyBallotProof(proof BallotProof, nowNs int64) error {
	if node.BlindKeyRegistry == nil {
		return fmt.Errorf("ballot-proof: blind-key registry unavailable")
	}
	att, ok := node.BlindKeyRegistry.GetByFingerprint(proof.RSAKeyFingerprint)
	if !ok {
		return fmt.Errorf("ballot-proof: unknown key fingerprint %s", proof.RSAKeyFingerprint)
	}
	if att.ElectionID != proof.ElectionID {
		return fmt.Errorf("ballot-proof: key/election mismatch (key %s, proof %s)",
			att.ElectionID, proof.ElectionID)
	}
	if nowNs < att.ValidFrom || nowNs > att.ValidUntil {
		return fmt.Errorf("ballot-proof: key fingerprint %s outside validity window",
			proof.RSAKeyFingerprint)
	}
	// Reconstruct the RSA public key.
	nBytes, err := hex.DecodeString(att.RSAPublicKeyNHex)
	if err != nil {
		return fmt.Errorf("ballot-proof: bad modulus hex: %w", err)
	}
	rsaPub := &rsa.PublicKey{
		N: new(big.Int).SetBytes(nBytes),
		E: att.RSAPublicKeyE,
	}
	// Decode the ballot token + signature from hex.
	tokenBytes, err := hex.DecodeString(proof.BallotToken)
	if err != nil {
		return fmt.Errorf("ballot-proof: bad token hex: %w", err)
	}
	sigBytes, err := hex.DecodeString(proof.BlindSignature)
	if err != nil {
		return fmt.Errorf("ballot-proof: bad signature hex: %w", err)
	}
	s := new(big.Int).SetBytes(sigBytes)
	if err := blindrsa.Verify(rsaPub, tokenBytes, s); err != nil {
		return fmt.Errorf("ballot-proof: RSA-FDH verify failed: %w", err)
	}
	return nil
}
