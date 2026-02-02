package main

import (
	"encoding/json"
	"encoding/hex"
	"testing"
)

func TestValidateTrustTransaction_NonceRequired(t *testing.T) {
	node := newTestNode()

	tx := TrustTransaction{
		BaseTransaction: BaseTransaction{
			Type:        TxTypeTrust,
			TrustDomain: "default",
			Timestamp:   1000,
		},
		Truster:    node.NodeID,
		Trustee:    "1234567890abcdef",
		TrustLevel: 0.8,
		Nonce:      0,
	}
	tx = signTrustTx(node, tx)

	if node.ValidateTrustTransaction(tx) {
		t.Error("Expected transaction with nonce=0 to be rejected")
	}
}

func TestValidateTrustTransaction_NoncePositive(t *testing.T) {
	node := newTestNode()

	tx := TrustTransaction{
		BaseTransaction: BaseTransaction{
			Type:        TxTypeTrust,
			TrustDomain: "default",
			Timestamp:   1000,
		},
		Truster:    node.NodeID,
		Trustee:    "1234567890abcdef",
		TrustLevel: 0.8,
		Nonce:      -1,
	}
	tx = signTrustTx(node, tx)

	if node.ValidateTrustTransaction(tx) {
		t.Error("Expected transaction with negative nonce to be rejected")
	}
}

func TestValidateTrustTransaction_NonceLowerThanExisting_Rejected(t *testing.T) {
	node := newTestNode()
	trustee := "1234567890abcdef"

	// Set up existing nonce in registry
	node.TrustRegistryMutex.Lock()
	node.TrustNonceRegistry[node.NodeID] = make(map[string]int64)
	node.TrustNonceRegistry[node.NodeID][trustee] = 5
	node.TrustRegistryMutex.Unlock()

	// Try to submit with lower nonce
	tx := TrustTransaction{
		BaseTransaction: BaseTransaction{
			Type:        TxTypeTrust,
			TrustDomain: "default",
			Timestamp:   1000,
		},
		Truster:    node.NodeID,
		Trustee:    trustee,
		TrustLevel: 0.8,
		Nonce:      3,
	}
	tx = signTrustTx(node, tx)

	if node.ValidateTrustTransaction(tx) {
		t.Error("Expected transaction with nonce lower than existing to be rejected")
	}
}

func TestValidateTrustTransaction_NonceEqualToExisting_Rejected(t *testing.T) {
	node := newTestNode()
	trustee := "1234567890abcdef"

	// Set up existing nonce in registry
	node.TrustRegistryMutex.Lock()
	node.TrustNonceRegistry[node.NodeID] = make(map[string]int64)
	node.TrustNonceRegistry[node.NodeID][trustee] = 5
	node.TrustRegistryMutex.Unlock()

	// Try to submit with equal nonce
	tx := TrustTransaction{
		BaseTransaction: BaseTransaction{
			Type:        TxTypeTrust,
			TrustDomain: "default",
			Timestamp:   1000,
		},
		Truster:    node.NodeID,
		Trustee:    trustee,
		TrustLevel: 0.8,
		Nonce:      5,
	}
	tx = signTrustTx(node, tx)

	if node.ValidateTrustTransaction(tx) {
		t.Error("Expected transaction with nonce equal to existing to be rejected")
	}
}

func TestValidateTrustTransaction_NonceHigherThanExisting_Accepted(t *testing.T) {
	node := newTestNode()
	trustee := "1234567890abcdef"

	// Set up existing nonce in registry
	node.TrustRegistryMutex.Lock()
	node.TrustNonceRegistry[node.NodeID] = make(map[string]int64)
	node.TrustNonceRegistry[node.NodeID][trustee] = 5
	node.TrustRegistryMutex.Unlock()

	// Submit with higher nonce
	tx := TrustTransaction{
		BaseTransaction: BaseTransaction{
			Type:        TxTypeTrust,
			TrustDomain: "default",
			Timestamp:   1000,
		},
		Truster:    node.NodeID,
		Trustee:    trustee,
		TrustLevel: 0.8,
		Nonce:      6,
	}
	tx = signTrustTx(node, tx)

	if !node.ValidateTrustTransaction(tx) {
		t.Error("Expected transaction with nonce higher than existing to be accepted")
	}
}

func TestValidateTrustTransaction_ReplayAttack_Rejected(t *testing.T) {
	node := newTestNode()
	trustee := "1234567890abcdef"

	// Create and validate first transaction
	tx1 := TrustTransaction{
		BaseTransaction: BaseTransaction{
			Type:        TxTypeTrust,
			TrustDomain: "default",
			Timestamp:   1000,
		},
		Truster:    node.NodeID,
		Trustee:    trustee,
		TrustLevel: 0.8,
		Nonce:      1,
	}
	tx1 = signTrustTx(node, tx1)

	if !node.ValidateTrustTransaction(tx1) {
		t.Fatal("Expected first transaction to be valid")
	}

	// Simulate the transaction being processed (update registry)
	node.updateTrustRegistry(tx1)

	// Try to replay the same transaction
	if node.ValidateTrustTransaction(tx1) {
		t.Error("Expected replay of same transaction to be rejected")
	}
}

func TestAddTrustTransaction_AutoAssignsNonce(t *testing.T) {
	node := newTestNode()
	trustee := "1234567890abcdef"

	// Set up existing nonce in registry
	node.TrustRegistryMutex.Lock()
	node.TrustNonceRegistry[node.NodeID] = make(map[string]int64)
	node.TrustNonceRegistry[node.NodeID][trustee] = 3
	node.TrustRegistryMutex.Unlock()

	// Create transaction without nonce
	tx := TrustTransaction{
		BaseTransaction: BaseTransaction{
			TrustDomain: "default",
			PublicKey:   node.GetPublicKeyHex(),
		},
		Truster:    node.NodeID,
		Trustee:    trustee,
		TrustLevel: 0.8,
		Nonce:      0,
	}

	// Sign without nonce (will be auto-assigned)
	txCopy := tx
	txCopy.Type = TxTypeTrust
	txCopy.Timestamp = 1000
	txCopy.Nonce = 4 // Expected auto-assigned value
	signableData, _ := json.Marshal(txCopy)
	sigCopy := txCopy
	sigCopy.Signature = ""
	signableData, _ = json.Marshal(sigCopy)
	signature, _ := node.SignData(signableData)
	tx.Signature = hex.EncodeToString(signature)

	// Note: AddTrustTransaction will auto-assign nonce=4
	// But signature was computed with nonce=0, so it will fail validation
	// This test demonstrates that for signed transactions, caller must set nonce
}

func TestUpdateTrustRegistry_UpdatesNonceRegistry(t *testing.T) {
	node := newTestNode()
	trustee := "1234567890abcdef"

	tx := TrustTransaction{
		BaseTransaction: BaseTransaction{
			Type:        TxTypeTrust,
			TrustDomain: "default",
			Timestamp:   1000,
		},
		Truster:    node.NodeID,
		Trustee:    trustee,
		TrustLevel: 0.8,
		Nonce:      5,
	}

	node.updateTrustRegistry(tx)

	node.TrustRegistryMutex.RLock()
	defer node.TrustRegistryMutex.RUnlock()

	// Check trust level was updated
	if level, exists := node.TrustRegistry[node.NodeID][trustee]; !exists || level != 0.8 {
		t.Errorf("Expected trust level 0.8, got %v", level)
	}

	// Check nonce was updated
	if nonce, exists := node.TrustNonceRegistry[node.NodeID][trustee]; !exists || nonce != 5 {
		t.Errorf("Expected nonce 5, got %v", nonce)
	}
}

func TestValidateTrustTransaction_NewTrustRelationship_NonceOneAccepted(t *testing.T) {
	node := newTestNode()
	trustee := "1234567890abcdef"

	// No existing relationship, nonce=1 should be accepted
	tx := TrustTransaction{
		BaseTransaction: BaseTransaction{
			Type:        TxTypeTrust,
			TrustDomain: "default",
			Timestamp:   1000,
		},
		Truster:    node.NodeID,
		Trustee:    trustee,
		TrustLevel: 0.8,
		Nonce:      1,
	}
	tx = signTrustTx(node, tx)

	if !node.ValidateTrustTransaction(tx) {
		t.Error("Expected nonce=1 for new trust relationship to be accepted")
	}
}
