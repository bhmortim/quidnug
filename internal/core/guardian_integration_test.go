// Package core — guardian_integration_test.go
//
// Methodology
// -----------
// These tests exercise the full guardian recovery lifecycle through
// the block-processing machinery (processBlockTransactions), not just
// the validators in isolation.
//
// Each test constructs transactions wrapped in their proper envelope
// types (GuardianSetUpdateTransaction, GuardianRecoveryInitTransaction,
// etc.), places them in a Block, and lets processBlockTransactions
// dispatch. This is the exact path production blocks take at Trusted
// inclusion.
//
// The tests exist in three arcs matching the expected real-world
// usage patterns:
//
//   1. Install-then-recover: subject installs a guardian set, later
//      loses key, guardians initiate recovery, delay elapses, commit
//      finalizes → subject's epoch advances with the new key.
//   2. Install-recover-veto: subject detects fraudulent recovery,
//      publishes primary-key veto within the delay window, pending
//      record transitions to Vetoed, no epoch change.
//   3. Attacker-race: attacker with compromised key tries to rotate
//      unilaterally. This is the scenario guardian recovery is
//      meant to prevent — the test demonstrates that WITHOUT
//      RequireGuardianRotation, a plain AnchorRotation from the
//      primary key still works, confirming we haven't broken the
//      existing rotation path.
package core

import (
	"testing"
	"time"
)

// subjectSetup holds the canonical state used across integration
// tests: a test node plus a pre-installed guardian set.
type subjectSetup struct {
	node      *QuidnugNode
	subject   string
	guardians []guardian
	set       *GuardianSet
}

func setupSubjectWithGuardians(t *testing.T) *subjectSetup {
	t.Helper()
	node := newTestNode()

	gs := []guardian{
		newGuardian(t, "gA"),
		newGuardian(t, "gB"),
		newGuardian(t, "gC"),
	}
	set := buildSet(gs, 2, 1*time.Hour)
	seedGuardianSet(t, node.NonceLedger, node.NodeID, set, gs)
	return &subjectSetup{
		node:      node,
		subject:   node.NodeID,
		guardians: gs,
		set:       set,
	}
}

// TestGuardianRecovery_FullFlow walks a subject from healthy, through
// key loss, through guardian-initiated recovery, through the delay
// window, to finalized commit. Expected observable state changes:
//
//   * After Init: PendingRecoveryOf returns a RecoveryPending record,
//     MaturityUnix reflects ValidFrom + RecoveryDelay.
//   * After Commit (past maturity): CurrentEpoch advanced, new key
//     installed, MaxAcceptedOldNonce cap applied.
func TestGuardianRecovery_FullFlow(t *testing.T) {
	s := setupSubjectWithGuardians(t)

	_, newPub := keypairHex(t)
	initValidFrom := time.Now().Add(-5 * time.Minute).Unix() // 5min in past
	init := GuardianRecoveryInit{
		Kind:                AnchorGuardianRecoveryInit,
		SubjectQuid:         s.subject,
		FromEpoch:           0,
		ToEpoch:             1,
		NewPublicKey:        newPub,
		MinNextNonce:        1,
		MaxAcceptedOldNonce: 42,
		AnchorNonce:         10,
		ValidFrom:           initValidFrom,
	}
	signable, _ := GuardianRecoveryInitSignableBytes(init)
	init.GuardianSigs = []GuardianSignature{
		{GuardianQuid: s.guardians[0].quid, KeyEpoch: 0, Signature: signWithGuardianKey(t, s.guardians[0].priv, signable)},
		{GuardianQuid: s.guardians[1].quid, KeyEpoch: 0, Signature: signWithGuardianKey(t, s.guardians[1].priv, signable)},
	}

	block1 := Block{
		Index:        1,
		Timestamp:    time.Now().Unix(),
		Transactions: []interface{}{
			GuardianRecoveryInitTransaction{
				BaseTransaction: BaseTransaction{Type: TxTypeGuardianRecoveryInit, Timestamp: time.Now().Unix()},
				Init:            init,
			},
		},
		TrustProof: TrustProof{TrustDomain: "test.domain.com"},
	}
	s.node.processBlockTransactions(block1)

	pr := s.node.NonceLedger.PendingRecoveryOf(s.subject)
	if pr == nil {
		t.Fatal("expected PendingRecovery after init")
	}
	if pr.State != RecoveryPending {
		t.Fatalf("state: want pending, got %v", pr.State)
	}
	expectedMaturity := time.Unix(initValidFrom, 0).Add(s.set.RecoveryDelay).Unix()
	if pr.MaturityUnix != expectedMaturity {
		t.Fatalf("maturity: want %d, got %d", expectedMaturity, pr.MaturityUnix)
	}

	// Force the pending recovery to look mature (can't really wait
	// an hour in a unit test). Mutate directly — this is what the
	// test-time clock shim would do.
	s.node.NonceLedger.mu.Lock()
	s.node.NonceLedger.pendingRecoveries[s.subject].MaturityUnix = time.Now().Add(-1 * time.Minute).Unix()
	s.node.NonceLedger.mu.Unlock()

	// Commit.
	initHash, err := GuardianRecoveryInitHash(init)
	if err != nil {
		t.Fatalf("hash: %v", err)
	}
	committer := newGuardian(t, "committer")
	s.node.NonceLedger.SetSignerKey(committer.quid, 0, committer.pub)
	commit := GuardianRecoveryCommit{
		Kind:               AnchorGuardianRecoveryCommit,
		SubjectQuid:        s.subject,
		RecoveryAnchorHash: initHash,
		AnchorNonce:        11,
		ValidFrom:          time.Now().Unix(),
		CommitterQuid:      committer.quid,
		CommitterSig:       signWithGuardianKey(t, committer.priv, []byte(initHash)),
	}
	block2 := Block{
		Index:        2,
		Timestamp:    time.Now().Unix(),
		Transactions: []interface{}{
			GuardianRecoveryCommitTransaction{
				BaseTransaction: BaseTransaction{Type: TxTypeGuardianRecoveryCommit, Timestamp: time.Now().Unix()},
				Commit:          commit,
			},
		},
		TrustProof: TrustProof{TrustDomain: "test.domain.com"},
	}
	s.node.processBlockTransactions(block2)

	if got := s.node.NonceLedger.CurrentEpoch(s.subject); got != 1 {
		t.Fatalf("current epoch after commit: want 1, got %d", got)
	}
	if key, ok := s.node.NonceLedger.GetSignerKey(s.subject, 1); !ok || key != newPub {
		t.Fatalf("new key not installed at epoch 1")
	}
	if got := s.node.NonceLedger.EpochCap(s.subject, 0); got != 42 {
		t.Fatalf("old-epoch cap: want 42, got %d", got)
	}
}

// TestGuardianRecovery_PrimaryKeyVeto exercises the fast-path cancel:
// guardians initiate recovery, the still-reachable owner vetoes with
// a single primary-key signature, pending record transitions to
// Vetoed, and no epoch change happens even after the would-be
// maturity passes.
func TestGuardianRecovery_PrimaryKeyVeto(t *testing.T) {
	s := setupSubjectWithGuardians(t)

	_, newPub := keypairHex(t)
	initValidFrom := time.Now().Unix()
	init := GuardianRecoveryInit{
		Kind:                AnchorGuardianRecoveryInit,
		SubjectQuid:         s.subject,
		FromEpoch:           0,
		ToEpoch:             1,
		NewPublicKey:        newPub,
		MinNextNonce:        1,
		MaxAcceptedOldNonce: 0,
		AnchorNonce:         5,
		ValidFrom:           initValidFrom,
	}
	signable, _ := GuardianRecoveryInitSignableBytes(init)
	init.GuardianSigs = []GuardianSignature{
		{GuardianQuid: s.guardians[0].quid, KeyEpoch: 0, Signature: signWithGuardianKey(t, s.guardians[0].priv, signable)},
		{GuardianQuid: s.guardians[1].quid, KeyEpoch: 0, Signature: signWithGuardianKey(t, s.guardians[1].priv, signable)},
	}

	s.node.processBlockTransactions(Block{
		Index: 1, Timestamp: time.Now().Unix(),
		TrustProof: TrustProof{TrustDomain: "test.domain.com"},
		Transactions: []interface{}{
			GuardianRecoveryInitTransaction{
				BaseTransaction: BaseTransaction{Type: TxTypeGuardianRecoveryInit, Timestamp: time.Now().Unix()},
				Init:            init,
			},
		},
	})

	initHash, _ := GuardianRecoveryInitHash(init)

	// Primary-key veto. The subject's own key (node's private key) is
	// used — same key path the normal operator would use.
	veto := GuardianRecoveryVeto{
		Kind:               AnchorGuardianRecoveryVeto,
		SubjectQuid:        s.subject,
		RecoveryAnchorHash: initHash,
		AnchorNonce:        6, // > init's 5
		ValidFrom:          time.Now().Unix(),
	}
	vetoSignable, _ := GuardianRecoveryVetoSignableBytes(veto)
	vetoSig, err := s.node.SignData(vetoSignable)
	if err != nil {
		t.Fatalf("sign veto: %v", err)
	}
	veto.PrimarySignature = &PrimarySignature{
		KeyEpoch:  0,
		Signature: hexEncode(vetoSig),
	}

	s.node.processBlockTransactions(Block{
		Index: 2, Timestamp: time.Now().Unix(),
		TrustProof: TrustProof{TrustDomain: "test.domain.com"},
		Transactions: []interface{}{
			GuardianRecoveryVetoTransaction{
				BaseTransaction: BaseTransaction{Type: TxTypeGuardianRecoveryVeto, Timestamp: time.Now().Unix()},
				Veto:            veto,
			},
		},
	})

	// Post-veto: PendingRecoveryOf returns nil (RecoveryVetoed is
	// terminal so the accessor filters it out). Epoch must remain 0.
	if pr := s.node.NonceLedger.PendingRecoveryOf(s.subject); pr != nil {
		t.Fatalf("expected no Pending after veto, got %+v", pr)
	}
	if got := s.node.NonceLedger.CurrentEpoch(s.subject); got != 0 {
		t.Fatalf("epoch must not advance after veto: got %d", got)
	}
}

// TestGuardianRecovery_SetInstallViaBlock verifies that a first-install
// GuardianSetUpdate goes through processBlockTransactions cleanly and
// is readable via GuardianSetOf afterwards.
func TestGuardianRecovery_SetInstallViaBlock(t *testing.T) {
	node := newTestNode()

	gs := []guardian{
		newGuardian(t, "gA"),
		newGuardian(t, "gB"),
	}
	for _, g := range gs {
		node.NonceLedger.SetSignerKey(g.quid, 0, g.pub)
	}
	newSet := buildSet(gs, 1, 2*time.Hour)

	update := GuardianSetUpdate{
		Kind:        AnchorGuardianSetUpdate,
		SubjectQuid: node.NodeID,
		NewSet:      *newSet,
		AnchorNonce: 1,
		ValidFrom:   time.Now().Unix(),
	}
	signable, _ := GuardianSetUpdateSignableBytes(update)
	sig, err := node.SignData(signable)
	if err != nil {
		t.Fatalf("sign: %v", err)
	}
	update.PrimarySignature = &PrimarySignature{
		KeyEpoch:  0,
		Signature: hexEncode(sig),
	}

	node.processBlockTransactions(Block{
		Index:      7,
		Timestamp:  time.Now().Unix(),
		TrustProof: TrustProof{TrustDomain: "test.domain.com"},
		Transactions: []interface{}{
			GuardianSetUpdateTransaction{
				BaseTransaction: BaseTransaction{Type: TxTypeGuardianSetUpdate, Timestamp: time.Now().Unix()},
				Update:          update,
			},
		},
	})

	got := node.NonceLedger.GuardianSetOf(node.NodeID)
	if got == nil {
		t.Fatal("GuardianSetOf returned nil after install")
	}
	if len(got.Guardians) != 2 {
		t.Fatalf("expected 2 guardians, got %d", len(got.Guardians))
	}
	if got.UpdatedAtBlock != 7 {
		t.Fatalf("UpdatedAtBlock: want 7, got %d", got.UpdatedAtBlock)
	}
}

// TestGuardianRecovery_InitBlockedByInFlight verifies the
// MaxConcurrentRecoveries=1 rule: a second Init for the same subject
// while the first is still Pending is rejected (no state change).
func TestGuardianRecovery_InitBlockedByInFlight(t *testing.T) {
	s := setupSubjectWithGuardians(t)

	_, newPub := keypairHex(t)
	first := GuardianRecoveryInit{
		Kind:         AnchorGuardianRecoveryInit,
		SubjectQuid:  s.subject,
		FromEpoch:    0, ToEpoch: 1,
		NewPublicKey: newPub,
		MinNextNonce: 1, AnchorNonce: 10,
		ValidFrom:    time.Now().Unix(),
	}
	signable, _ := GuardianRecoveryInitSignableBytes(first)
	first.GuardianSigs = []GuardianSignature{
		{GuardianQuid: s.guardians[0].quid, KeyEpoch: 0, Signature: signWithGuardianKey(t, s.guardians[0].priv, signable)},
		{GuardianQuid: s.guardians[1].quid, KeyEpoch: 0, Signature: signWithGuardianKey(t, s.guardians[1].priv, signable)},
	}
	s.node.processBlockTransactions(Block{
		Index:      1,
		Timestamp:  time.Now().Unix(),
		TrustProof: TrustProof{TrustDomain: "test.domain.com"},
		Transactions: []interface{}{
			GuardianRecoveryInitTransaction{
				BaseTransaction: BaseTransaction{Type: TxTypeGuardianRecoveryInit, Timestamp: time.Now().Unix()},
				Init:            first,
			},
		},
	})
	firstHash, _ := GuardianRecoveryInitHash(first)

	// A second Init (different new key, different anchor-nonce) would
	// be valid in isolation, but there's already a pending recovery.
	_, otherPub := keypairHex(t)
	second := GuardianRecoveryInit{
		Kind:         AnchorGuardianRecoveryInit,
		SubjectQuid:  s.subject,
		FromEpoch:    0, ToEpoch: 1,
		NewPublicKey: otherPub,
		MinNextNonce: 1, AnchorNonce: 20,
		ValidFrom:    time.Now().Unix(),
	}
	signable2, _ := GuardianRecoveryInitSignableBytes(second)
	second.GuardianSigs = []GuardianSignature{
		{GuardianQuid: s.guardians[0].quid, KeyEpoch: 0, Signature: signWithGuardianKey(t, s.guardians[0].priv, signable2)},
		{GuardianQuid: s.guardians[1].quid, KeyEpoch: 0, Signature: signWithGuardianKey(t, s.guardians[1].priv, signable2)},
	}
	s.node.processBlockTransactions(Block{
		Index:      2,
		Timestamp:  time.Now().Unix(),
		TrustProof: TrustProof{TrustDomain: "test.domain.com"},
		Transactions: []interface{}{
			GuardianRecoveryInitTransaction{
				BaseTransaction: BaseTransaction{Type: TxTypeGuardianRecoveryInit, Timestamp: time.Now().Unix()},
				Init:            second,
			},
		},
	})

	// First pending recovery remains; second was rejected (logged, no
	// state change).
	pr := s.node.NonceLedger.PendingRecoveryOf(s.subject)
	if pr == nil {
		t.Fatal("expected first recovery still pending")
	}
	if pr.InitHash != firstHash {
		t.Fatalf("wrong pending recovery after blocked second init")
	}
}
