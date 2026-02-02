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
