package main

import (
	"testing"
)

func TestFilterTransactionsForBlock_EventTransaction_TrustedCreatorIncluded(t *testing.T) {
	node := newTestNode()

	// Create a trusted creator identity
	creatorID := node.NodeID // Use node's own ID as creator (self-trust is 1.0)

	// Register creator identity so event validation passes
	node.IdentityRegistry[creatorID] = IdentityTransaction{
		BaseTransaction: BaseTransaction{
			ID:          "identity-1",
			Type:        TxTypeIdentity,
			TrustDomain: "default",
			PublicKey:   node.GetPublicKeyHex(),
		},
		QuidID:  creatorID,
		Name:    "Test Creator",
		Creator: creatorID,
	}

	// Create and sign an event transaction
	eventTx := signEventTx(node, EventTransaction{
		BaseTransaction: BaseTransaction{
			TrustDomain: "default",
		},
		SubjectID:   creatorID,
		SubjectType: "QUID",
		Sequence:    1,
		EventType:   "test.event",
		Payload:     map[string]interface{}{"key": "value"},
	})

	txs := []interface{}{eventTx}

	// Filter with zero threshold (should include all)
	node.TransactionTrustThreshold = 0.0
	filtered := node.FilterTransactionsForBlock(txs, "default")

	if len(filtered) != 1 {
		t.Errorf("Expected 1 transaction, got %d", len(filtered))
	}
}

func TestFilterTransactionsForBlock_EventTransaction_UntrustedCreatorExcluded(t *testing.T) {
	node := newTestNode()

	// Create a different node to act as untrusted creator
	untrustedNode, err := NewQuidnugNode(nil)
	if err != nil {
		t.Fatalf("Failed to create untrusted node: %v", err)
	}

	untrustedCreatorID := untrustedNode.NodeID

	// Register untrusted creator identity so event validation passes
	node.IdentityRegistry[untrustedCreatorID] = IdentityTransaction{
		BaseTransaction: BaseTransaction{
			ID:          "identity-2",
			Type:        TxTypeIdentity,
			TrustDomain: "default",
			PublicKey:   untrustedNode.GetPublicKeyHex(),
		},
		QuidID:  untrustedCreatorID,
		Name:    "Untrusted Creator",
		Creator: untrustedCreatorID,
	}

	// Create and sign an event transaction from untrusted node
	eventTx := signEventTx(untrustedNode, EventTransaction{
		BaseTransaction: BaseTransaction{
			TrustDomain: "default",
		},
		SubjectID:   untrustedCreatorID,
		SubjectType: "QUID",
		Sequence:    1,
		EventType:   "test.event",
		Payload:     map[string]interface{}{"key": "value"},
	})

	txs := []interface{}{eventTx}

	// Set threshold above zero - untrusted creator has 0.0 trust
	node.TransactionTrustThreshold = 0.5
	filtered := node.FilterTransactionsForBlock(txs, "default")

	if len(filtered) != 0 {
		t.Errorf("Expected 0 transactions (untrusted creator filtered out), got %d", len(filtered))
	}
}

func TestFilterTransactionsForBlock_EventTransaction_MixedWithOtherTypes(t *testing.T) {
	node := newTestNode()

	// Use node's own ID as trusted creator
	creatorID := node.NodeID

	// Register creator identity
	node.IdentityRegistry[creatorID] = IdentityTransaction{
		BaseTransaction: BaseTransaction{
			ID:          "identity-1",
			Type:        TxTypeIdentity,
			TrustDomain: "default",
			PublicKey:   node.GetPublicKeyHex(),
		},
		QuidID:  creatorID,
		Name:    "Test Creator",
		Creator: creatorID,
	}

	// Create a trust transaction
	trustTx := signTrustTx(node, TrustTransaction{
		BaseTransaction: BaseTransaction{
			TrustDomain: "default",
		},
		Truster:    creatorID,
		Trustee:    "abcdef1234567890",
		TrustLevel: 0.8,
		Nonce:      1,
	})

	// Create an event transaction
	eventTx := signEventTx(node, EventTransaction{
		BaseTransaction: BaseTransaction{
			TrustDomain: "default",
		},
		SubjectID:   creatorID,
		SubjectType: "QUID",
		Sequence:    1,
		EventType:   "test.event",
		Payload:     map[string]interface{}{"key": "value"},
	})

	txs := []interface{}{trustTx, eventTx}

	// Filter with zero threshold
	node.TransactionTrustThreshold = 0.0
	filtered := node.FilterTransactionsForBlock(txs, "default")

	if len(filtered) != 2 {
		t.Errorf("Expected 2 transactions (trust + event), got %d", len(filtered))
	}

	// Verify both types are present
	hasTrust := false
	hasEvent := false
	for _, tx := range filtered {
		switch tx.(type) {
		case TrustTransaction:
			hasTrust = true
		case EventTransaction:
			hasEvent = true
		}
	}

	if !hasTrust {
		t.Error("Expected trust transaction in filtered results")
	}
	if !hasEvent {
		t.Error("Expected event transaction in filtered results")
	}
}

func TestFilterTransactionsForBlock_EventTransaction_EmptyPublicKey(t *testing.T) {
	node := newTestNode()

	// Create an event transaction with empty public key (should be skipped)
	eventTx := EventTransaction{
		BaseTransaction: BaseTransaction{
			ID:          "event-1",
			Type:        TxTypeEvent,
			TrustDomain: "default",
			PublicKey:   "", // Empty public key
		},
		SubjectID:   "1234567890abcdef",
		SubjectType: "QUID",
		Sequence:    1,
		EventType:   "test.event",
		Payload:     map[string]interface{}{"key": "value"},
	}

	txs := []interface{}{eventTx}

	// Filter - should skip due to empty creator
	node.TransactionTrustThreshold = 0.0
	filtered := node.FilterTransactionsForBlock(txs, "default")

	if len(filtered) != 0 {
		t.Errorf("Expected 0 transactions (empty public key should be skipped), got %d", len(filtered))
	}
}
