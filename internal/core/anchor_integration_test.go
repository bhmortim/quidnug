package core

import (
	"testing"
	"time"
)

// TestApplyAnchorFromBlock_EndToEnd exercises the full path: an
// anchor wrapped in a TxTypeAnchor transaction, included in a
// processed block, updates the ledger's epoch state. This is the
// QDP-0001 §6.5 block-inclusion flow.
func TestApplyAnchorFromBlock_EndToEnd(t *testing.T) {
	node := newTestNode()

	// The test node can rotate its own key (self-rotation).
	nodeQuid := node.NodeID
	_, newPubHex := keypairHex(t)

	anchor := NonceAnchor{
		Kind:                AnchorRotation,
		SignerQuid:          nodeQuid,
		FromEpoch:           0,
		ToEpoch:             1,
		NewPublicKey:        newPubHex,
		MinNextNonce:        1,
		MaxAcceptedOldNonce: 5,
		ValidFrom:           time.Now().Unix(),
		AnchorNonce:         1,
	}
	// Sign with node's own key (node.NodeID at epoch 0 was seeded at
	// NewQuidnugNode time).
	signable, _ := GetAnchorSignableData(anchor)
	sig, err := node.SignData(signable)
	if err != nil {
		t.Fatalf("sign anchor: %v", err)
	}
	anchor.Signature = hexEncode(sig)

	// Wrap in a block transaction.
	anchorTx := AnchorTransaction{
		BaseTransaction: BaseTransaction{
			ID:        "tx-anchor-1",
			Type:      TxTypeAnchor,
			Timestamp: time.Now().Unix(),
		},
		Anchor: anchor,
	}

	block := Block{
		Index:     1,
		Timestamp: time.Now().Unix(),
		Transactions: []interface{}{
			anchorTx,
		},
		TrustProof: TrustProof{TrustDomain: "test.domain.com"},
	}

	// Process it via the same entry point that ReceiveBlock calls.
	node.processBlockTransactions(block)

	if got := node.NonceLedger.CurrentEpoch(nodeQuid); got != 1 {
		t.Fatalf("expected CurrentEpoch=1 after rotation, got %d", got)
	}
	if got := node.NonceLedger.EpochCap(nodeQuid, 0); got != 5 {
		t.Fatalf("expected EpochCap(epoch 0)=5, got %d", got)
	}
	if got := node.NonceLedger.LastAnchorNonce(nodeQuid); got != 1 {
		t.Fatalf("expected LastAnchorNonce=1, got %d", got)
	}
}

func TestApplyAnchorFromBlock_RejectsBadSignature(t *testing.T) {
	node := newTestNode()

	// Anchor signed by a different key.
	otherPriv, _ := keypairHex(t)
	_, newPubHex := keypairHex(t)
	anchor := NonceAnchor{
		Kind:                AnchorRotation,
		SignerQuid:          node.NodeID, // claiming to be node
		FromEpoch:           0,
		ToEpoch:             1,
		NewPublicKey:        newPubHex,
		MinNextNonce:        1,
		MaxAcceptedOldNonce: 5,
		ValidFrom:           time.Now().Unix(),
		AnchorNonce:         1,
	}
	anchor = signAnchor(t, otherPriv, anchor) // signed with attacker's key

	anchorTx := AnchorTransaction{
		BaseTransaction: BaseTransaction{Type: TxTypeAnchor, Timestamp: time.Now().Unix()},
		Anchor:          anchor,
	}
	block := Block{
		Index:        1,
		Transactions: []interface{}{anchorTx},
		TrustProof:   TrustProof{TrustDomain: "test.domain.com"},
	}

	node.processBlockTransactions(block)

	// Epoch must NOT have advanced — bad signature is rejected in
	// applyAnchorFromBlock's defense-in-depth ValidateAnchor call.
	if got := node.NonceLedger.CurrentEpoch(node.NodeID); got != 0 {
		t.Fatalf("epoch advanced despite bad signature: got %d", got)
	}
}

// hexEncode is a thin test-local wrapper; we don't want to import
// encoding/hex into the test file and pollute its import block unless
// necessary.
func hexEncode(b []byte) string {
	const hexTable = "0123456789abcdef"
	out := make([]byte, len(b)*2)
	for i, v := range b {
		out[i*2] = hexTable[v>>4]
		out[i*2+1] = hexTable[v&0x0f]
	}
	return string(out)
}
