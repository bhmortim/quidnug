package main

import (
	"testing"
)

func TestValidateTrustProof_SelfAsValidator(t *testing.T) {
	node := newTestNode()

	// The default domain already has this node as validator
	proof := TrustProof{
		TrustDomain:             "default",
		ValidatorID:             node.NodeID,
		ValidatorTrustInCreator: 0.8, // This field is now informational only
		ValidatorSigs:           []string{"somesignature"},
		ValidationTime:          1234567890,
	}

	if !node.ValidateTrustProof(proof) {
		t.Error("Expected trust proof from self to be valid")
	}
}

func TestValidateTrustProof_UnknownDomain(t *testing.T) {
	node := newTestNode()

	proof := TrustProof{
		TrustDomain:   "unknowndomain",
		ValidatorID:   node.NodeID,
		ValidatorSigs: []string{"somesignature"},
	}

	if node.ValidateTrustProof(proof) {
		t.Error("Expected trust proof from unknown domain to be invalid")
	}
}

func TestValidateTrustProof_ValidatorNotInDomain(t *testing.T) {
	node := newTestNode()

	node.TrustDomains["testdomain"] = TrustDomain{
		Name:           "testdomain",
		ValidatorNodes: []string{"validator1234567"},
		TrustThreshold: 0.5,
	}

	proof := TrustProof{
		TrustDomain:   "testdomain",
		ValidatorID:   "notavalidator123",
		ValidatorSigs: []string{"somesignature"},
	}

	if node.ValidateTrustProof(proof) {
		t.Error("Expected trust proof from non-validator to be invalid")
	}
}

func TestValidateTrustProof_NoSignatures(t *testing.T) {
	node := newTestNode()

	proof := TrustProof{
		TrustDomain:   "default",
		ValidatorID:   node.NodeID,
		ValidatorSigs: []string{},
	}

	if node.ValidateTrustProof(proof) {
		t.Error("Expected trust proof without signatures to be invalid")
	}
}

func TestValidateTrustProof_RelationalTrustRequired(t *testing.T) {
	node := newTestNode()

	validatorID := "validator1234567"

	// Set up a trust domain with an external validator
	node.TrustDomains["testdomain"] = TrustDomain{
		Name:           "testdomain",
		ValidatorNodes: []string{validatorID},
		TrustThreshold: 0.5,
	}

	proof := TrustProof{
		TrustDomain:   "testdomain",
		ValidatorID:   validatorID,
		ValidatorSigs: []string{"somesignature"},
	}

	// No trust relationship exists - should fail
	if node.ValidateTrustProof(proof) {
		t.Error("Expected trust proof to be invalid when no trust relationship exists")
	}

	// Add trust relationship: this node trusts the validator with 0.6
	node.TrustRegistry[node.NodeID] = map[string]float64{
		validatorID: 0.6,
	}

	// Now should pass (0.6 >= 0.5 threshold)
	if !node.ValidateTrustProof(proof) {
		t.Error("Expected trust proof to be valid when trust relationship meets threshold")
	}
}

func TestValidateTrustProof_TrustBelowThreshold(t *testing.T) {
	node := newTestNode()

	validatorID := "validator1234567"

	node.TrustDomains["testdomain"] = TrustDomain{
		Name:           "testdomain",
		ValidatorNodes: []string{validatorID},
		TrustThreshold: 0.8,
	}

	// Trust level below threshold
	node.TrustRegistry[node.NodeID] = map[string]float64{
		validatorID: 0.5,
	}

	proof := TrustProof{
		TrustDomain:   "testdomain",
		ValidatorID:   validatorID,
		ValidatorSigs: []string{"somesignature"},
	}

	if node.ValidateTrustProof(proof) {
		t.Error("Expected trust proof to be invalid when trust is below threshold")
	}
}

func TestValidateTrustProof_TransitiveTrust(t *testing.T) {
	node := newTestNode()

	intermediary := "intermediary12345"
	validatorID := "validator1234567"

	// Set up a trust domain with an external validator
	node.TrustDomains["testdomain"] = TrustDomain{
		Name:           "testdomain",
		ValidatorNodes: []string{validatorID},
		TrustThreshold: 0.5,
	}

	// Set up transitive trust: node -> intermediary -> validator
	// 0.9 * 0.9 = 0.81, which is > 0.5 threshold
	node.TrustRegistry[node.NodeID] = map[string]float64{
		intermediary: 0.9,
	}
	node.TrustRegistry[intermediary] = map[string]float64{
		validatorID: 0.9,
	}

	proof := TrustProof{
		TrustDomain:   "testdomain",
		ValidatorID:   validatorID,
		ValidatorSigs: []string{"somesignature"},
	}

	// Should pass via transitive trust (0.81 >= 0.5)
	if !node.ValidateTrustProof(proof) {
		t.Error("Expected trust proof to be valid via transitive trust")
	}
}

func TestValidateTrustProof_TransitiveTrustBelowThreshold(t *testing.T) {
	node := newTestNode()

	intermediary := "intermediary12345"
	validatorID := "validator1234567"

	node.TrustDomains["testdomain"] = TrustDomain{
		Name:           "testdomain",
		ValidatorNodes: []string{validatorID},
		TrustThreshold: 0.9, // Higher than transitive trust
	}

	// Transitive trust: 0.9 * 0.9 = 0.81 < 0.9 threshold
	node.TrustRegistry[node.NodeID] = map[string]float64{
		intermediary: 0.9,
	}
	node.TrustRegistry[intermediary] = map[string]float64{
		validatorID: 0.9,
	}

	proof := TrustProof{
		TrustDomain:   "testdomain",
		ValidatorID:   validatorID,
		ValidatorSigs: []string{"somesignature"},
	}

	if node.ValidateTrustProof(proof) {
		t.Error("Expected trust proof to be invalid when transitive trust is below threshold")
	}
}

func TestValidateTrustProof_IgnoresStaticScore(t *testing.T) {
	node := newTestNode()

	validatorID := "validator1234567"

	node.TrustDomains["testdomain"] = TrustDomain{
		Name:           "testdomain",
		ValidatorNodes: []string{validatorID},
		TrustThreshold: 0.5,
	}

	// No trust relationship in registry
	proof := TrustProof{
		TrustDomain:             "testdomain",
		ValidatorID:             validatorID,
		ValidatorTrustInCreator: 1.0, // High static score should be ignored
		ValidatorSigs:           []string{"somesignature"},
	}

	// Should fail because relational trust is 0, regardless of ValidatorTrustInCreator
	if node.ValidateTrustProof(proof) {
		t.Error("Expected trust proof to be invalid - should use relational trust, not static score")
	}
}

// ============================================================================
// ValidateTrustProofTiered Tests
// ============================================================================

func TestValidateTrustProofTiered_UnknownDomain(t *testing.T) {
	node := newTestNode()

	proof := TrustProof{
		TrustDomain:   "unknowndomain",
		ValidatorID:   node.NodeID,
		ValidatorSigs: []string{"somesignature"},
	}

	if result := node.ValidateTrustProofTiered(proof); result != BlockInvalid {
		t.Errorf("Expected BlockInvalid for unknown domain, got %v", result)
	}
}

func TestValidateTrustProofTiered_ValidatorNotInDomain(t *testing.T) {
	node := newTestNode()

	node.TrustDomains["testdomain"] = TrustDomain{
		Name:           "testdomain",
		ValidatorNodes: []string{"validator1234567"},
		TrustThreshold: 0.5,
	}

	proof := TrustProof{
		TrustDomain:   "testdomain",
		ValidatorID:   "notavalidator123",
		ValidatorSigs: []string{"somesignature"},
	}

	if result := node.ValidateTrustProofTiered(proof); result != BlockInvalid {
		t.Errorf("Expected BlockInvalid for validator not in domain, got %v", result)
	}
}

func TestValidateTrustProofTiered_NoSignatures(t *testing.T) {
	node := newTestNode()

	proof := TrustProof{
		TrustDomain:   "default",
		ValidatorID:   node.NodeID,
		ValidatorSigs: []string{},
	}

	if result := node.ValidateTrustProofTiered(proof); result != BlockInvalid {
		t.Errorf("Expected BlockInvalid for no signatures, got %v", result)
	}
}

func TestValidateTrustProofTiered_SelfAsValidator(t *testing.T) {
	node := newTestNode()

	proof := TrustProof{
		TrustDomain:   "default",
		ValidatorID:   node.NodeID,
		ValidatorSigs: []string{"somesignature"},
	}

	if result := node.ValidateTrustProofTiered(proof); result != BlockTrusted {
		t.Errorf("Expected BlockTrusted for self as validator, got %v", result)
	}
}

func TestValidateTrustProofTiered_TrustMeetsThreshold(t *testing.T) {
	node := newTestNode()

	validatorID := "validator1234567"

	node.TrustDomains["testdomain"] = TrustDomain{
		Name:           "testdomain",
		ValidatorNodes: []string{validatorID},
		TrustThreshold: 0.5,
	}

	// Trust level exactly meets threshold
	node.TrustRegistry[node.NodeID] = map[string]float64{
		validatorID: 0.5,
	}

	proof := TrustProof{
		TrustDomain:   "testdomain",
		ValidatorID:   validatorID,
		ValidatorSigs: []string{"somesignature"},
	}

	if result := node.ValidateTrustProofTiered(proof); result != BlockTrusted {
		t.Errorf("Expected BlockTrusted when trust meets threshold, got %v", result)
	}
}

func TestValidateTrustProofTiered_TrustAboveThreshold(t *testing.T) {
	node := newTestNode()

	validatorID := "validator1234567"

	node.TrustDomains["testdomain"] = TrustDomain{
		Name:           "testdomain",
		ValidatorNodes: []string{validatorID},
		TrustThreshold: 0.5,
	}

	// Trust level exceeds threshold
	node.TrustRegistry[node.NodeID] = map[string]float64{
		validatorID: 0.8,
	}

	proof := TrustProof{
		TrustDomain:   "testdomain",
		ValidatorID:   validatorID,
		ValidatorSigs: []string{"somesignature"},
	}

	if result := node.ValidateTrustProofTiered(proof); result != BlockTrusted {
		t.Errorf("Expected BlockTrusted when trust exceeds threshold, got %v", result)
	}
}

func TestValidateTrustProofTiered_Tentative(t *testing.T) {
	node := newTestNode()

	validatorID := "validator1234567"
	node.DistrustThreshold = 0.1

	node.TrustDomains["testdomain"] = TrustDomain{
		Name:           "testdomain",
		ValidatorNodes: []string{validatorID},
		TrustThreshold: 0.8,
	}

	// Trust level between DistrustThreshold and TrustThreshold
	node.TrustRegistry[node.NodeID] = map[string]float64{
		validatorID: 0.5, // 0.1 < 0.5 < 0.8
	}

	proof := TrustProof{
		TrustDomain:   "testdomain",
		ValidatorID:   validatorID,
		ValidatorSigs: []string{"somesignature"},
	}

	if result := node.ValidateTrustProofTiered(proof); result != BlockTentative {
		t.Errorf("Expected BlockTentative for trust between thresholds, got %v", result)
	}
}

func TestValidateTrustProofTiered_Untrusted_BelowDistrustThreshold(t *testing.T) {
	node := newTestNode()

	validatorID := "validator1234567"
	node.DistrustThreshold = 0.3

	node.TrustDomains["testdomain"] = TrustDomain{
		Name:           "testdomain",
		ValidatorNodes: []string{validatorID},
		TrustThreshold: 0.8,
	}

	// Trust level below DistrustThreshold
	node.TrustRegistry[node.NodeID] = map[string]float64{
		validatorID: 0.2, // 0.2 < 0.3 (DistrustThreshold)
	}

	proof := TrustProof{
		TrustDomain:   "testdomain",
		ValidatorID:   validatorID,
		ValidatorSigs: []string{"somesignature"},
	}

	if result := node.ValidateTrustProofTiered(proof); result != BlockUntrusted {
		t.Errorf("Expected BlockUntrusted for trust below DistrustThreshold, got %v", result)
	}
}

func TestValidateTrustProofTiered_Untrusted_AtDistrustThreshold(t *testing.T) {
	node := newTestNode()

	validatorID := "validator1234567"
	node.DistrustThreshold = 0.3

	node.TrustDomains["testdomain"] = TrustDomain{
		Name:           "testdomain",
		ValidatorNodes: []string{validatorID},
		TrustThreshold: 0.8,
	}

	// Trust level exactly at DistrustThreshold
	node.TrustRegistry[node.NodeID] = map[string]float64{
		validatorID: 0.3, // 0.3 == DistrustThreshold, should be Untrusted
	}

	proof := TrustProof{
		TrustDomain:   "testdomain",
		ValidatorID:   validatorID,
		ValidatorSigs: []string{"somesignature"},
	}

	if result := node.ValidateTrustProofTiered(proof); result != BlockUntrusted {
		t.Errorf("Expected BlockUntrusted for trust at DistrustThreshold, got %v", result)
	}
}

func TestValidateTrustProofTiered_Untrusted_ZeroTrust(t *testing.T) {
	node := newTestNode()

	validatorID := "validator1234567"

	node.TrustDomains["testdomain"] = TrustDomain{
		Name:           "testdomain",
		ValidatorNodes: []string{validatorID},
		TrustThreshold: 0.5,
	}

	// No trust relationship (trust == 0)
	proof := TrustProof{
		TrustDomain:   "testdomain",
		ValidatorID:   validatorID,
		ValidatorSigs: []string{"somesignature"},
	}

	if result := node.ValidateTrustProofTiered(proof); result != BlockUntrusted {
		t.Errorf("Expected BlockUntrusted for zero trust, got %v", result)
	}
}

func TestValidateTrustProofTiered_ThresholdBoundary_JustAboveDistrust(t *testing.T) {
	node := newTestNode()

	validatorID := "validator1234567"
	node.DistrustThreshold = 0.3

	node.TrustDomains["testdomain"] = TrustDomain{
		Name:           "testdomain",
		ValidatorNodes: []string{validatorID},
		TrustThreshold: 0.8,
	}

	// Trust just above DistrustThreshold should be Tentative
	node.TrustRegistry[node.NodeID] = map[string]float64{
		validatorID: 0.31,
	}

	proof := TrustProof{
		TrustDomain:   "testdomain",
		ValidatorID:   validatorID,
		ValidatorSigs: []string{"somesignature"},
	}

	if result := node.ValidateTrustProofTiered(proof); result != BlockTentative {
		t.Errorf("Expected BlockTentative for trust just above DistrustThreshold, got %v", result)
	}
}

func TestValidateTrustProofTiered_ThresholdBoundary_JustBelowTrust(t *testing.T) {
	node := newTestNode()

	validatorID := "validator1234567"
	node.DistrustThreshold = 0.1

	node.TrustDomains["testdomain"] = TrustDomain{
		Name:           "testdomain",
		ValidatorNodes: []string{validatorID},
		TrustThreshold: 0.8,
	}

	// Trust just below TrustThreshold should be Tentative
	node.TrustRegistry[node.NodeID] = map[string]float64{
		validatorID: 0.79,
	}

	proof := TrustProof{
		TrustDomain:   "testdomain",
		ValidatorID:   validatorID,
		ValidatorSigs: []string{"somesignature"},
	}

	if result := node.ValidateTrustProofTiered(proof); result != BlockTentative {
		t.Errorf("Expected BlockTentative for trust just below threshold, got %v", result)
	}
}

// ============================================================================
// ValidateBlockCryptographic Tests
// ============================================================================

func TestValidateBlockCryptographic_EmptyBlockchain(t *testing.T) {
	node := newTestNode()

	// Clear blockchain
	node.BlockchainMutex.Lock()
	node.Blockchain = []Block{}
	node.BlockchainMutex.Unlock()

	block := Block{
		Index:    1,
		PrevHash: "somehash",
		Hash:     "blockhash",
	}

	if node.ValidateBlockCryptographic(block) {
		t.Error("Expected false for empty blockchain")
	}
}

func TestValidateBlockCryptographic_WrongIndex(t *testing.T) {
	node := newTestNode()

	// Block with wrong index (should be 1)
	block := Block{
		Index:        5,
		Timestamp:    1234567890,
		Transactions: []interface{}{},
		PrevHash:     node.Blockchain[0].Hash,
	}
	block.Hash = calculateBlockHash(block)

	if node.ValidateBlockCryptographic(block) {
		t.Error("Expected false for wrong block index")
	}
}

func TestValidateBlockCryptographic_WrongPrevHash(t *testing.T) {
	node := newTestNode()

	block := Block{
		Index:        1,
		Timestamp:    1234567890,
		Transactions: []interface{}{},
		PrevHash:     "wronghash",
	}
	block.Hash = calculateBlockHash(block)

	if node.ValidateBlockCryptographic(block) {
		t.Error("Expected false for wrong prev hash")
	}
}

func TestValidateBlockCryptographic_WrongBlockHash(t *testing.T) {
	node := newTestNode()

	block := Block{
		Index:        1,
		Timestamp:    1234567890,
		Transactions: []interface{}{},
		PrevHash:     node.Blockchain[0].Hash,
		Hash:         "wronghash",
	}

	if node.ValidateBlockCryptographic(block) {
		t.Error("Expected false for wrong block hash")
	}
}

func TestValidateBlockCryptographic_Valid(t *testing.T) {
	node := newTestNode()

	block := Block{
		Index:        1,
		Timestamp:    1234567890,
		Transactions: []interface{}{},
		PrevHash:     node.Blockchain[0].Hash,
	}
	block.Hash = calculateBlockHash(block)

	if !node.ValidateBlockCryptographic(block) {
		t.Error("Expected true for valid cryptographic block")
	}
}

// ============================================================================
// ValidateBlockTiered Tests
// ============================================================================

func TestValidateBlockTiered_CryptographicallyInvalid(t *testing.T) {
	node := newTestNode()

	block := Block{
		Index:        1,
		Timestamp:    1234567890,
		Transactions: []interface{}{},
		PrevHash:     "wronghash",
		Hash:         "somehash",
		TrustProof: TrustProof{
			TrustDomain:   "default",
			ValidatorID:   node.NodeID,
			ValidatorSigs: []string{"sig"},
		},
	}

	if result := node.ValidateBlockTiered(block); result != BlockInvalid {
		t.Errorf("Expected BlockInvalid for cryptographically invalid block, got %v", result)
	}
}

func TestValidateBlockTiered_InvalidTransaction(t *testing.T) {
	node := newTestNode()

	// Create a trust transaction with invalid trust level
	invalidTx := TrustTransaction{
		BaseTransaction: BaseTransaction{
			ID:          "tx1",
			Type:        TxTypeTrust,
			TrustDomain: "default",
			Timestamp:   1234567890,
		},
		Truster:    "truster123456789",
		Trustee:    "trustee123456789",
		TrustLevel: 2.0, // Invalid: > 1.0
	}

	block := Block{
		Index:        1,
		Timestamp:    1234567890,
		Transactions: []interface{}{invalidTx},
		PrevHash:     node.Blockchain[0].Hash,
		TrustProof: TrustProof{
			TrustDomain:   "default",
			ValidatorID:   node.NodeID,
			ValidatorSigs: []string{"sig"},
		},
	}
	block.Hash = calculateBlockHash(block)

	if result := node.ValidateBlockTiered(block); result != BlockInvalid {
		t.Errorf("Expected BlockInvalid for invalid transaction, got %v", result)
	}
}

func TestValidateBlockTiered_TrustedBlock(t *testing.T) {
	node := newTestNode()

	block := Block{
		Index:        1,
		Timestamp:    1234567890,
		Transactions: []interface{}{},
		PrevHash:     node.Blockchain[0].Hash,
		TrustProof: TrustProof{
			TrustDomain:   "default",
			ValidatorID:   node.NodeID,
			ValidatorSigs: []string{"sig"},
		},
	}
	block.Hash = calculateBlockHash(block)

	if result := node.ValidateBlockTiered(block); result != BlockTrusted {
		t.Errorf("Expected BlockTrusted for valid block from self, got %v", result)
	}
}

func TestValidateBlockTiered_TentativeBlock(t *testing.T) {
	node := newTestNode()

	validatorID := "validator1234567"
	node.DistrustThreshold = 0.1

	node.TrustDomains["testdomain"] = TrustDomain{
		Name:           "testdomain",
		ValidatorNodes: []string{validatorID},
		TrustThreshold: 0.8,
	}

	// Trust level between thresholds
	node.TrustRegistry[node.NodeID] = map[string]float64{
		validatorID: 0.5,
	}

	block := Block{
		Index:        1,
		Timestamp:    1234567890,
		Transactions: []interface{}{},
		PrevHash:     node.Blockchain[0].Hash,
		TrustProof: TrustProof{
			TrustDomain:   "testdomain",
			ValidatorID:   validatorID,
			ValidatorSigs: []string{"sig"},
		},
	}
	block.Hash = calculateBlockHash(block)

	if result := node.ValidateBlockTiered(block); result != BlockTentative {
		t.Errorf("Expected BlockTentative, got %v", result)
	}
}

func TestValidateBlockTiered_UntrustedBlock(t *testing.T) {
	node := newTestNode()

	validatorID := "validator1234567"

	node.TrustDomains["testdomain"] = TrustDomain{
		Name:           "testdomain",
		ValidatorNodes: []string{validatorID},
		TrustThreshold: 0.5,
	}

	// No trust relationship
	block := Block{
		Index:        1,
		Timestamp:    1234567890,
		Transactions: []interface{}{},
		PrevHash:     node.Blockchain[0].Hash,
		TrustProof: TrustProof{
			TrustDomain:   "testdomain",
			ValidatorID:   validatorID,
			ValidatorSigs: []string{"sig"},
		},
	}
	block.Hash = calculateBlockHash(block)

	if result := node.ValidateBlockTiered(block); result != BlockUntrusted {
		t.Errorf("Expected BlockUntrusted, got %v", result)
	}
}

func TestValidateBlockTiered_SeparatesCryptoFromTrust(t *testing.T) {
	node := newTestNode()

	validatorID := "validator1234567"

	node.TrustDomains["testdomain"] = TrustDomain{
		Name:           "testdomain",
		ValidatorNodes: []string{validatorID},
		TrustThreshold: 0.5,
	}

	// Cryptographically valid but untrusted validator
	block := Block{
		Index:        1,
		Timestamp:    1234567890,
		Transactions: []interface{}{},
		PrevHash:     node.Blockchain[0].Hash,
		TrustProof: TrustProof{
			TrustDomain:   "testdomain",
			ValidatorID:   validatorID,
			ValidatorSigs: []string{"sig"},
		},
	}
	block.Hash = calculateBlockHash(block)

	// Cryptographic validation should pass
	if !node.ValidateBlockCryptographic(block) {
		t.Error("Expected cryptographic validation to pass")
	}

	// But overall tiered validation returns Untrusted (not Invalid)
	result := node.ValidateBlockTiered(block)
	if result == BlockInvalid {
		t.Error("Expected block to NOT be Invalid - crypto is valid, only trust fails")
	}
	if result != BlockUntrusted {
		t.Errorf("Expected BlockUntrusted for untrusted validator, got %v", result)
	}
}

// ============================================================================
// Backward Compatibility Tests
// ============================================================================

func TestValidateTrustProof_BackwardCompatibility(t *testing.T) {
	node := newTestNode()

	// Test that ValidateTrustProof returns true only for BlockTrusted
	proof := TrustProof{
		TrustDomain:   "default",
		ValidatorID:   node.NodeID,
		ValidatorSigs: []string{"sig"},
	}

	if !node.ValidateTrustProof(proof) {
		t.Error("Expected ValidateTrustProof to return true for trusted proof")
	}

	// Test that it returns false for tentative
	validatorID := "validator1234567"
	node.DistrustThreshold = 0.1
	node.TrustDomains["testdomain"] = TrustDomain{
		Name:           "testdomain",
		ValidatorNodes: []string{validatorID},
		TrustThreshold: 0.8,
	}
	node.TrustRegistry[node.NodeID] = map[string]float64{
		validatorID: 0.5, // tentative
	}

	tentativeProof := TrustProof{
		TrustDomain:   "testdomain",
		ValidatorID:   validatorID,
		ValidatorSigs: []string{"sig"},
	}

	if node.ValidateTrustProof(tentativeProof) {
		t.Error("Expected ValidateTrustProof to return false for tentative proof")
	}
}

// ============================================================================
// ReceiveBlock Tests
// ============================================================================

func TestReceiveBlock_Invalid_BadHash(t *testing.T) {
	node := newTestNode()

	block := Block{
		Index:        1,
		Timestamp:    1234567890,
		Transactions: []interface{}{},
		PrevHash:     node.Blockchain[0].Hash,
		Hash:         "badhash",
		TrustProof: TrustProof{
			TrustDomain:   "default",
			ValidatorID:   node.NodeID,
			ValidatorSigs: []string{"sig"},
		},
	}

	acceptance, err := node.ReceiveBlock(block)
	if err == nil {
		t.Error("Expected error for invalid block")
	}
	if acceptance != BlockInvalid {
		t.Errorf("Expected BlockInvalid, got %v", acceptance)
	}

	// Verify block not added
	node.BlockchainMutex.RLock()
	if len(node.Blockchain) != 1 {
		t.Errorf("Expected blockchain length 1, got %d", len(node.Blockchain))
	}
	node.BlockchainMutex.RUnlock()
}

func TestReceiveBlock_Invalid_WrongPrevHash(t *testing.T) {
	node := newTestNode()

	block := Block{
		Index:        1,
		Timestamp:    1234567890,
		Transactions: []interface{}{},
		PrevHash:     "wrongprevhash",
	}
	block.Hash = calculateBlockHash(block)

	acceptance, err := node.ReceiveBlock(block)
	if err == nil {
		t.Error("Expected error for invalid block")
	}
	if acceptance != BlockInvalid {
		t.Errorf("Expected BlockInvalid, got %v", acceptance)
	}
}

func TestReceiveBlock_Trusted(t *testing.T) {
	node := newTestNode()

	block := Block{
		Index:        1,
		Timestamp:    1234567890,
		Transactions: []interface{}{},
		PrevHash:     node.Blockchain[0].Hash,
		TrustProof: TrustProof{
			TrustDomain:   "default",
			ValidatorID:   node.NodeID,
			ValidatorSigs: []string{"sig"},
		},
	}
	block.Hash = calculateBlockHash(block)

	acceptance, err := node.ReceiveBlock(block)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if acceptance != BlockTrusted {
		t.Errorf("Expected BlockTrusted, got %v", acceptance)
	}

	// Verify block added to chain
	node.BlockchainMutex.RLock()
	if len(node.Blockchain) != 2 {
		t.Errorf("Expected blockchain length 2, got %d", len(node.Blockchain))
	}
	if node.Blockchain[1].Hash != block.Hash {
		t.Error("Block not added to chain correctly")
	}
	node.BlockchainMutex.RUnlock()

	// Verify not in tentative storage
	tentative := node.GetTentativeBlocks("default")
	if len(tentative) != 0 {
		t.Errorf("Expected no tentative blocks, got %d", len(tentative))
	}
}

func TestReceiveBlock_Tentative(t *testing.T) {
	node := newTestNode()

	validatorID := "validator1234567"
	node.DistrustThreshold = 0.1

	node.TrustDomains["testdomain"] = TrustDomain{
		Name:           "testdomain",
		ValidatorNodes: []string{validatorID},
		TrustThreshold: 0.8,
	}

	// Trust between thresholds (tentative)
	node.TrustRegistry[node.NodeID] = map[string]float64{
		validatorID: 0.5,
	}

	block := Block{
		Index:        1,
		Timestamp:    1234567890,
		Transactions: []interface{}{},
		PrevHash:     node.Blockchain[0].Hash,
		TrustProof: TrustProof{
			TrustDomain:   "testdomain",
			ValidatorID:   validatorID,
			ValidatorSigs: []string{"sig"},
		},
	}
	block.Hash = calculateBlockHash(block)

	acceptance, err := node.ReceiveBlock(block)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if acceptance != BlockTentative {
		t.Errorf("Expected BlockTentative, got %v", acceptance)
	}

	// Verify block NOT in main chain
	node.BlockchainMutex.RLock()
	if len(node.Blockchain) != 1 {
		t.Errorf("Expected blockchain length 1, got %d", len(node.Blockchain))
	}
	node.BlockchainMutex.RUnlock()

	// Verify block IS in tentative storage
	tentative := node.GetTentativeBlocks("testdomain")
	if len(tentative) != 1 {
		t.Errorf("Expected 1 tentative block, got %d", len(tentative))
	}
	if tentative[0].Hash != block.Hash {
		t.Error("Wrong block in tentative storage")
	}
}

func TestReceiveBlock_Untrusted(t *testing.T) {
	node := newTestNode()

	validatorID := "validator1234567"

	node.TrustDomains["testdomain"] = TrustDomain{
		Name:           "testdomain",
		ValidatorNodes: []string{validatorID},
		TrustThreshold: 0.5,
	}

	// No trust relationship (untrusted)
	block := Block{
		Index:        1,
		Timestamp:    1234567890,
		Transactions: []interface{}{},
		PrevHash:     node.Blockchain[0].Hash,
		TrustProof: TrustProof{
			TrustDomain:   "testdomain",
			ValidatorID:   validatorID,
			ValidatorSigs: []string{"sig"},
		},
	}
	block.Hash = calculateBlockHash(block)

	acceptance, err := node.ReceiveBlock(block)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if acceptance != BlockUntrusted {
		t.Errorf("Expected BlockUntrusted, got %v", acceptance)
	}

	// Verify block NOT in main chain
	node.BlockchainMutex.RLock()
	if len(node.Blockchain) != 1 {
		t.Errorf("Expected blockchain length 1, got %d", len(node.Blockchain))
	}
	node.BlockchainMutex.RUnlock()

	// Verify block NOT in tentative storage
	tentative := node.GetTentativeBlocks("testdomain")
	if len(tentative) != 0 {
		t.Errorf("Expected no tentative blocks, got %d", len(tentative))
	}
}

func TestReceiveBlock_EdgeExtraction(t *testing.T) {
	node := newTestNode()

	// Create a trust transaction (edges extracted even from untrusted blocks)
	tx := TrustTransaction{
		BaseTransaction: BaseTransaction{
			ID:          "tx1",
			Type:        TxTypeTrust,
			TrustDomain: "default",
			Timestamp:   1234567890,
		},
		Truster:    "truster123456789",
		Trustee:    "trustee123456789",
		TrustLevel: 0.7,
	}

	// Create block from untrusted validator
	validatorID := "validator1234567"
	node.TrustDomains["testdomain"] = TrustDomain{
		Name:           "testdomain",
		ValidatorNodes: []string{validatorID},
		TrustThreshold: 0.5,
	}

	block := Block{
		Index:        1,
		Timestamp:    1234567890,
		Transactions: []interface{}{tx},
		PrevHash:     node.Blockchain[0].Hash,
		TrustProof: TrustProof{
			TrustDomain:   "testdomain",
			ValidatorID:   validatorID,
			ValidatorSigs: []string{"sig"},
		},
	}
	block.Hash = calculateBlockHash(block)

	// ReceiveBlock extracts edges even if block is Invalid/Untrusted
	node.ReceiveBlock(block)

	// Check that unverified edge was extracted
	edges := node.GetTrustEdges("truster123456789", true)
	if len(edges) == 0 {
		t.Error("Expected edge to be extracted to unverified registry")
	} else {
		edge, exists := edges["trustee123456789"]
		if !exists {
			t.Error("Expected edge truster->trustee to exist")
		} else if edge.TrustLevel != 0.7 {
			t.Errorf("Expected trust level 0.7, got %f", edge.TrustLevel)
		}
	}
}

func TestStoreTentativeBlock_Duplicate(t *testing.T) {
	node := newTestNode()

	block := Block{
		Index:        1,
		Timestamp:    1234567890,
		Transactions: []interface{}{},
		PrevHash:     node.Blockchain[0].Hash,
		TrustProof: TrustProof{
			TrustDomain: "default",
		},
	}
	block.Hash = calculateBlockHash(block)

	// First store should succeed
	err := node.StoreTentativeBlock(block)
	if err != nil {
		t.Fatalf("First store failed: %v", err)
	}

	// Second store should fail (duplicate)
	err = node.StoreTentativeBlock(block)
	if err == nil {
		t.Error("Expected error for duplicate block")
	}
}

func TestReEvaluateTentativeBlocks_Promotion(t *testing.T) {
	node := newTestNode()

	validatorID := "validator1234567"
	node.DistrustThreshold = 0.1

	node.TrustDomains["testdomain"] = TrustDomain{
		Name:           "testdomain",
		ValidatorNodes: []string{validatorID},
		TrustThreshold: 0.8,
	}

	// Start with tentative trust
	node.TrustRegistry[node.NodeID] = map[string]float64{
		validatorID: 0.5, // Below 0.8 threshold
	}

	block := Block{
		Index:        1,
		Timestamp:    1234567890,
		Transactions: []interface{}{},
		PrevHash:     node.Blockchain[0].Hash,
		TrustProof: TrustProof{
			TrustDomain:   "testdomain",
			ValidatorID:   validatorID,
			ValidatorSigs: []string{"sig"},
		},
	}
	block.Hash = calculateBlockHash(block)

	// Receive as tentative
	acceptance, _ := node.ReceiveBlock(block)
	if acceptance != BlockTentative {
		t.Fatalf("Expected BlockTentative, got %v", acceptance)
	}

	// Verify in tentative storage
	tentative := node.GetTentativeBlocks("testdomain")
	if len(tentative) != 1 {
		t.Fatalf("Expected 1 tentative block, got %d", len(tentative))
	}

	// Increase trust to meet threshold
	node.TrustRegistry[node.NodeID][validatorID] = 0.9

	// Re-evaluate
	err := node.ReEvaluateTentativeBlocks("testdomain")
	if err != nil {
		t.Fatalf("ReEvaluate failed: %v", err)
	}

	// Verify block promoted to main chain
	node.BlockchainMutex.RLock()
	if len(node.Blockchain) != 2 {
		t.Errorf("Expected blockchain length 2, got %d", len(node.Blockchain))
	}
	node.BlockchainMutex.RUnlock()

	// Verify removed from tentative storage
	tentative = node.GetTentativeBlocks("testdomain")
	if len(tentative) != 0 {
		t.Errorf("Expected no tentative blocks after promotion, got %d", len(tentative))
	}
}

func TestReEvaluateTentativeBlocks_StillTentative(t *testing.T) {
	node := newTestNode()

	validatorID := "validator1234567"
	node.DistrustThreshold = 0.1

	node.TrustDomains["testdomain"] = TrustDomain{
		Name:           "testdomain",
		ValidatorNodes: []string{validatorID},
		TrustThreshold: 0.8,
	}

	node.TrustRegistry[node.NodeID] = map[string]float64{
		validatorID: 0.5,
	}

	block := Block{
		Index:        1,
		Timestamp:    1234567890,
		Transactions: []interface{}{},
		PrevHash:     node.Blockchain[0].Hash,
		TrustProof: TrustProof{
			TrustDomain:   "testdomain",
			ValidatorID:   validatorID,
			ValidatorSigs: []string{"sig"},
		},
	}
	block.Hash = calculateBlockHash(block)

	node.ReceiveBlock(block)

	// Re-evaluate without changing trust
	node.ReEvaluateTentativeBlocks("testdomain")

	// Should still be tentative
	tentative := node.GetTentativeBlocks("testdomain")
	if len(tentative) != 1 {
		t.Errorf("Expected block to remain tentative, got %d", len(tentative))
	}

	// Not in main chain
	node.BlockchainMutex.RLock()
	if len(node.Blockchain) != 1 {
		t.Errorf("Expected blockchain length 1, got %d", len(node.Blockchain))
	}
	node.BlockchainMutex.RUnlock()
}

func TestReEvaluateTentativeBlocks_Demotion(t *testing.T) {
	node := newTestNode()

	validatorID := "validator1234567"
	node.DistrustThreshold = 0.3

	node.TrustDomains["testdomain"] = TrustDomain{
		Name:           "testdomain",
		ValidatorNodes: []string{validatorID},
		TrustThreshold: 0.8,
	}

	// Start with tentative trust
	node.TrustRegistry[node.NodeID] = map[string]float64{
		validatorID: 0.5,
	}

	block := Block{
		Index:        1,
		Timestamp:    1234567890,
		Transactions: []interface{}{},
		PrevHash:     node.Blockchain[0].Hash,
		TrustProof: TrustProof{
			TrustDomain:   "testdomain",
			ValidatorID:   validatorID,
			ValidatorSigs: []string{"sig"},
		},
	}
	block.Hash = calculateBlockHash(block)

	node.ReceiveBlock(block)

	// Decrease trust below DistrustThreshold
	node.TrustRegistry[node.NodeID][validatorID] = 0.2

	// Re-evaluate
	node.ReEvaluateTentativeBlocks("testdomain")

	// Should be removed from tentative (demoted to untrusted)
	tentative := node.GetTentativeBlocks("testdomain")
	if len(tentative) != 0 {
		t.Errorf("Expected block to be removed from tentative, got %d", len(tentative))
	}

	// Not in main chain either
	node.BlockchainMutex.RLock()
	if len(node.Blockchain) != 1 {
		t.Errorf("Expected blockchain length 1, got %d", len(node.Blockchain))
	}
	node.BlockchainMutex.RUnlock()
}

func TestReEvaluateTentativeBlocks_EmptyDomain(t *testing.T) {
	node := newTestNode()

	// Re-evaluate non-existent domain should not error
	err := node.ReEvaluateTentativeBlocks("nonexistent")
	if err != nil {
		t.Errorf("Unexpected error for empty domain: %v", err)
	}
}

// ============================================================================
// Backward Compatibility Tests
// ============================================================================

func TestValidateBlock_BackwardCompatibility(t *testing.T) {
	node := newTestNode()

	// Valid trusted block
	block := Block{
		Index:        1,
		Timestamp:    1234567890,
		Transactions: []interface{}{},
		PrevHash:     node.Blockchain[0].Hash,
		TrustProof: TrustProof{
			TrustDomain:   "default",
			ValidatorID:   node.NodeID,
			ValidatorSigs: []string{"sig"},
		},
	}
	block.Hash = calculateBlockHash(block)

	if !node.ValidateBlock(block) {
		t.Error("Expected ValidateBlock to return true for trusted block")
	}

	// Tentative block should return false from ValidateBlock
	validatorID := "validator1234567"
	node.DistrustThreshold = 0.1
	node.TrustDomains["testdomain"] = TrustDomain{
		Name:           "testdomain",
		ValidatorNodes: []string{validatorID},
		TrustThreshold: 0.8,
	}
	node.TrustRegistry[node.NodeID] = map[string]float64{
		validatorID: 0.5,
	}

	tentativeBlock := Block{
		Index:        1,
		Timestamp:    1234567890,
		Transactions: []interface{}{},
		PrevHash:     node.Blockchain[0].Hash,
		TrustProof: TrustProof{
			TrustDomain:   "testdomain",
			ValidatorID:   validatorID,
			ValidatorSigs: []string{"sig"},
		},
	}
	tentativeBlock.Hash = calculateBlockHash(tentativeBlock)

	if node.ValidateBlock(tentativeBlock) {
		t.Error("Expected ValidateBlock to return false for tentative block")
	}
}
