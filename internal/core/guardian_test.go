// Package core — guardian_test.go
//
// Methodology
// -----------
// Covers QDP-0002 guardian-based recovery end-to-end. The test
// surface breaks into four tranches mirroring the four new anchor
// kinds:
//
//   1. GuardianSetUpdate  — install / replace authorization, shape
//      validation (non-empty set, threshold bounds, delay range),
//      monotonicity.
//   2. GuardianRecoveryInit — happy path, rejection paths (no set,
//      in-flight recovery, insufficient sigs, unknown guardian,
//      epoch mismatch, duplicate signer).
//   3. GuardianRecoveryVeto — primary-key fast path, guardian-
//      threshold path, exactly-one-path enforcement, referencing a
//      non-pending recovery.
//   4. GuardianRecoveryCommit — maturity gate, audit-signature
//      requirement, derived rotation advances ledger epoch.
//
// Helpers build a test ecosystem of keypairs + test nodes to stand
// in for the subject, guardians, and committer. Timing uses a
// deliberately-truncated past timestamp for `ValidFrom` plus the
// real `time.Now()` at the validation call — the RecoveryDelay
// bookkeeping math is exercised by setting MaturityUnix relative to
// the anchor's ValidFrom.
package core

import (
	"crypto/ecdsa"
	"encoding/hex"
	"errors"
	"testing"
	"time"
)

// ----- helpers -------------------------------------------------------------

type guardian struct {
	quid string
	priv *ecdsa.PrivateKey
	pub  string
}

// newGuardian generates a fresh keypair and assigns a quid derived
// from the public key (mirroring NewQuidnugNode's quid-from-pubkey
// scheme so ledger key lookups work naturally).
func newGuardian(t *testing.T, label string) guardian {
	t.Helper()
	priv, pub := keypairHex(t)
	return guardian{
		quid: label + "-" + pub[:12], // stable per test
		priv: priv,
		pub:  pub,
	}
}

// signWith signs `data` with the given private key using the
// production signing path.
func signWithGuardianKey(t *testing.T, priv *ecdsa.PrivateKey, data []byte) string {
	t.Helper()
	sig, err := (&QuidnugNode{PrivateKey: priv}).SignData(data)
	if err != nil {
		t.Fatalf("sign: %v", err)
	}
	return hex.EncodeToString(sig)
}

// seedGuardianSet installs a set in the ledger and registers each
// guardian's epoch-0 key so signature verification can resolve.
func seedGuardianSet(t *testing.T, l *NonceLedger, subject string, set *GuardianSet, guardians []guardian) {
	t.Helper()
	for _, g := range guardians {
		l.SetSignerKey(g.quid, 0, g.pub)
	}
	l.setGuardianSet(subject, set)
}

// buildSet constructs a GuardianSet from the provided guardians with
// simple equal-weight threshold M.
func buildSet(guardians []guardian, threshold uint16, delay time.Duration) *GuardianSet {
	refs := make([]GuardianRef, len(guardians))
	for i, g := range guardians {
		refs[i] = GuardianRef{Quid: g.quid, Weight: 1, Epoch: 0}
	}
	return &GuardianSet{
		Guardians:     refs,
		Threshold:     threshold,
		RecoveryDelay: delay,
	}
}

// ----- GuardianSetUpdate ---------------------------------------------------

func TestValidateGuardianSetUpdate_FirstInstall(t *testing.T) {
	l := NewNonceLedger()
	// Subject's own key must be known for PrimarySignature verification.
	subjectPriv, subjectPub := keypairHex(t)
	l.SetSignerKey("subject", 0, subjectPub)

	gs := []guardian{newGuardian(t, "g1"), newGuardian(t, "g2"), newGuardian(t, "g3")}
	set := buildSet(gs, 2, 1*time.Hour)
	for _, g := range gs {
		l.SetSignerKey(g.quid, 0, g.pub)
	}

	u := GuardianSetUpdate{
		Kind:        AnchorGuardianSetUpdate,
		SubjectQuid: "subject",
		NewSet:      *set,
		AnchorNonce: 1,
		ValidFrom:   time.Now().Unix(),
	}
	signable, _ := GuardianSetUpdateSignableBytes(u)
	u.PrimarySignature = &PrimarySignature{
		KeyEpoch:  0,
		Signature: signWithGuardianKey(t, subjectPriv, signable),
	}

	if err := ValidateGuardianSetUpdate(l, u, time.Now()); err != nil {
		t.Fatalf("first install: %v", err)
	}
}

func TestValidateGuardianSetUpdate_RejectsMissingPrimarySig(t *testing.T) {
	l := NewNonceLedger()
	_, subjectPub := keypairHex(t)
	l.SetSignerKey("subject", 0, subjectPub)

	set := buildSet([]guardian{newGuardian(t, "g1")}, 1, 1*time.Hour)
	u := GuardianSetUpdate{
		Kind:        AnchorGuardianSetUpdate,
		SubjectQuid: "subject",
		NewSet:      *set,
		AnchorNonce: 1,
		ValidFrom:   time.Now().Unix(),
	}
	if err := ValidateGuardianSetUpdate(l, u, time.Now()); !errors.Is(err, ErrGuardianBadPrimarySig) {
		t.Fatalf("want ErrGuardianBadPrimarySig, got %v", err)
	}
}

func TestValidateGuardianSetUpdate_ReplaceRequiresGuardianThreshold(t *testing.T) {
	l := NewNonceLedger()
	subjectPriv, subjectPub := keypairHex(t)
	l.SetSignerKey("subject", 0, subjectPub)

	// Install an initial set.
	gs := []guardian{newGuardian(t, "g1"), newGuardian(t, "g2"), newGuardian(t, "g3")}
	seedGuardianSet(t, l, "subject", buildSet(gs, 2, 1*time.Hour), gs)

	// New set (different guardians, still equal-weight).
	newGs := []guardian{newGuardian(t, "n1"), newGuardian(t, "n2")}
	for _, g := range newGs {
		l.SetSignerKey(g.quid, 0, g.pub)
	}
	newSet := buildSet(newGs, 1, 2*time.Hour)

	u := GuardianSetUpdate{
		Kind:        AnchorGuardianSetUpdate,
		SubjectQuid: "subject",
		NewSet:      *newSet,
		AnchorNonce: 1,
		ValidFrom:   time.Now().Unix(),
	}
	signable, _ := GuardianSetUpdateSignableBytes(u)
	u.PrimarySignature = &PrimarySignature{
		KeyEpoch:  0,
		Signature: signWithGuardianKey(t, subjectPriv, signable),
	}

	// Missing guardian sigs on replace → reject.
	if err := ValidateGuardianSetUpdate(l, u, time.Now()); !errors.Is(err, ErrGuardianInsufficientSigs) {
		t.Fatalf("want ErrGuardianInsufficientSigs, got %v", err)
	}

	// Now supply 2-of-3 existing guardian sigs → accepted.
	u.GuardianSigs = []GuardianSignature{
		{GuardianQuid: gs[0].quid, KeyEpoch: 0, Signature: signWithGuardianKey(t, gs[0].priv, signable)},
		{GuardianQuid: gs[1].quid, KeyEpoch: 0, Signature: signWithGuardianKey(t, gs[1].priv, signable)},
	}
	if err := ValidateGuardianSetUpdate(l, u, time.Now()); err != nil {
		t.Fatalf("replace with threshold sigs: %v", err)
	}
}

func TestValidateGuardianSetUpdate_RejectsBadShape(t *testing.T) {
	l := NewNonceLedger()
	_, subjectPub := keypairHex(t)
	l.SetSignerKey("subject", 0, subjectPub)

	cases := []struct {
		name string
		set  GuardianSet
		want error
	}{
		{"empty set", GuardianSet{Threshold: 1, RecoveryDelay: time.Hour}, ErrGuardianEmptySet},
		{"threshold zero", GuardianSet{
			Guardians:     []GuardianRef{{Quid: "g", Epoch: 0}},
			Threshold:     0,
			RecoveryDelay: time.Hour,
		}, ErrGuardianBadThreshold},
		{"threshold too high", GuardianSet{
			Guardians:     []GuardianRef{{Quid: "g", Epoch: 0}},
			Threshold:     2,
			RecoveryDelay: time.Hour,
		}, ErrGuardianBadThreshold},
		{"delay too small", GuardianSet{
			Guardians:     []GuardianRef{{Quid: "g", Epoch: 0}},
			Threshold:     1,
			RecoveryDelay: 1 * time.Minute,
		}, ErrGuardianBadDelay},
		{"delay too large", GuardianSet{
			Guardians:     []GuardianRef{{Quid: "g", Epoch: 0}},
			Threshold:     1,
			RecoveryDelay: 2 * 365 * 24 * time.Hour,
		}, ErrGuardianBadDelay},
		{"duplicate guardian", GuardianSet{
			Guardians: []GuardianRef{
				{Quid: "g", Epoch: 0},
				{Quid: "g", Epoch: 0},
			},
			Threshold:     1,
			RecoveryDelay: time.Hour,
		}, ErrGuardianDuplicateSigner},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			u := GuardianSetUpdate{
				Kind:             AnchorGuardianSetUpdate,
				SubjectQuid:      "subject",
				NewSet:           c.set,
				AnchorNonce:      1,
				ValidFrom:        time.Now().Unix(),
				PrimarySignature: &PrimarySignature{KeyEpoch: 0, Signature: "unused"},
			}
			if err := ValidateGuardianSetUpdate(l, u, time.Now()); !errors.Is(err, c.want) {
				t.Fatalf("want %v, got %v", c.want, err)
			}
		})
	}
}

// ----- GuardianRecoveryInit ------------------------------------------------

func TestValidateGuardianRecoveryInit_HappyPath(t *testing.T) {
	l := NewNonceLedger()
	_, subjectPub := keypairHex(t)
	l.SetSignerKey("subject", 0, subjectPub)

	gs := []guardian{newGuardian(t, "g1"), newGuardian(t, "g2"), newGuardian(t, "g3")}
	seedGuardianSet(t, l, "subject", buildSet(gs, 2, 1*time.Hour), gs)

	_, newPub := keypairHex(t)
	a := GuardianRecoveryInit{
		Kind:                AnchorGuardianRecoveryInit,
		SubjectQuid:         "subject",
		FromEpoch:           0,
		ToEpoch:             1,
		NewPublicKey:        newPub,
		MinNextNonce:        1,
		MaxAcceptedOldNonce: 10,
		AnchorNonce:         1,
		ValidFrom:           time.Now().Unix(),
	}
	signable, _ := GuardianRecoveryInitSignableBytes(a)
	a.GuardianSigs = []GuardianSignature{
		{GuardianQuid: gs[0].quid, KeyEpoch: 0, Signature: signWithGuardianKey(t, gs[0].priv, signable)},
		{GuardianQuid: gs[1].quid, KeyEpoch: 0, Signature: signWithGuardianKey(t, gs[1].priv, signable)},
	}
	if err := ValidateGuardianRecoveryInit(l, a, time.Now()); err != nil {
		t.Fatalf("happy path: %v", err)
	}
}

func TestValidateGuardianRecoveryInit_NoSet(t *testing.T) {
	l := NewNonceLedger()
	_, newPub := keypairHex(t)
	a := GuardianRecoveryInit{
		Kind:         AnchorGuardianRecoveryInit,
		SubjectQuid:  "subject",
		FromEpoch:    0,
		ToEpoch:      1,
		NewPublicKey: newPub,
		MinNextNonce: 1,
		AnchorNonce:  1,
		ValidFrom:    time.Now().Unix(),
	}
	if err := ValidateGuardianRecoveryInit(l, a, time.Now()); !errors.Is(err, ErrGuardianSetNotFound) {
		t.Fatalf("want ErrGuardianSetNotFound, got %v", err)
	}
}

func TestValidateGuardianRecoveryInit_InsufficientSigs(t *testing.T) {
	l := NewNonceLedger()
	gs := []guardian{newGuardian(t, "g1"), newGuardian(t, "g2"), newGuardian(t, "g3")}
	seedGuardianSet(t, l, "subject", buildSet(gs, 2, 1*time.Hour), gs)

	_, newPub := keypairHex(t)
	a := GuardianRecoveryInit{
		Kind:         AnchorGuardianRecoveryInit,
		SubjectQuid:  "subject",
		FromEpoch:    0,
		ToEpoch:      1,
		NewPublicKey: newPub,
		MinNextNonce: 1,
		AnchorNonce:  1,
		ValidFrom:    time.Now().Unix(),
	}
	signable, _ := GuardianRecoveryInitSignableBytes(a)
	a.GuardianSigs = []GuardianSignature{
		{GuardianQuid: gs[0].quid, KeyEpoch: 0, Signature: signWithGuardianKey(t, gs[0].priv, signable)},
	}
	if err := ValidateGuardianRecoveryInit(l, a, time.Now()); !errors.Is(err, ErrGuardianInsufficientSigs) {
		t.Fatalf("want ErrGuardianInsufficientSigs, got %v", err)
	}
}

func TestValidateGuardianRecoveryInit_UnknownGuardian(t *testing.T) {
	l := NewNonceLedger()
	gs := []guardian{newGuardian(t, "g1"), newGuardian(t, "g2")}
	seedGuardianSet(t, l, "subject", buildSet(gs, 1, 1*time.Hour), gs)

	outsider := newGuardian(t, "outsider")
	l.SetSignerKey(outsider.quid, 0, outsider.pub)

	_, newPub := keypairHex(t)
	a := GuardianRecoveryInit{
		Kind:         AnchorGuardianRecoveryInit,
		SubjectQuid:  "subject",
		FromEpoch:    0,
		ToEpoch:      1,
		NewPublicKey: newPub,
		MinNextNonce: 1,
		AnchorNonce:  1,
		ValidFrom:    time.Now().Unix(),
	}
	signable, _ := GuardianRecoveryInitSignableBytes(a)
	a.GuardianSigs = []GuardianSignature{
		{GuardianQuid: outsider.quid, KeyEpoch: 0, Signature: signWithGuardianKey(t, outsider.priv, signable)},
	}
	if err := ValidateGuardianRecoveryInit(l, a, time.Now()); !errors.Is(err, ErrGuardianUnknownGuardian) {
		t.Fatalf("want ErrGuardianUnknownGuardian, got %v", err)
	}
}

func TestValidateGuardianRecoveryInit_DuplicateSigner(t *testing.T) {
	l := NewNonceLedger()
	gs := []guardian{newGuardian(t, "g1"), newGuardian(t, "g2"), newGuardian(t, "g3")}
	seedGuardianSet(t, l, "subject", buildSet(gs, 2, 1*time.Hour), gs)

	_, newPub := keypairHex(t)
	a := GuardianRecoveryInit{
		Kind:         AnchorGuardianRecoveryInit,
		SubjectQuid:  "subject",
		FromEpoch:    0,
		ToEpoch:      1,
		NewPublicKey: newPub,
		MinNextNonce: 1,
		AnchorNonce:  1,
		ValidFrom:    time.Now().Unix(),
	}
	signable, _ := GuardianRecoveryInitSignableBytes(a)
	sig := signWithGuardianKey(t, gs[0].priv, signable)
	// Same guardian signs twice — should not count as reaching threshold.
	a.GuardianSigs = []GuardianSignature{
		{GuardianQuid: gs[0].quid, KeyEpoch: 0, Signature: sig},
		{GuardianQuid: gs[0].quid, KeyEpoch: 0, Signature: sig},
	}
	if err := ValidateGuardianRecoveryInit(l, a, time.Now()); !errors.Is(err, ErrGuardianDuplicateSigner) {
		t.Fatalf("want ErrGuardianDuplicateSigner, got %v", err)
	}
}

// ----- GuardianRecoveryVeto ------------------------------------------------

func TestValidateGuardianRecoveryVeto_PrimaryKeyPath(t *testing.T) {
	l := NewNonceLedger()
	subjectPriv, subjectPub := keypairHex(t)
	l.SetSignerKey("subject", 0, subjectPub)

	gs := []guardian{newGuardian(t, "g1"), newGuardian(t, "g2")}
	seedGuardianSet(t, l, "subject", buildSet(gs, 1, 1*time.Hour), gs)

	// Seed an in-flight recovery.
	initHash := "abc123"
	l.beginPendingRecovery("subject", &PendingRecovery{
		InitHash:     initHash,
		MaturityUnix: time.Now().Add(1 * time.Hour).Unix(),
		State:        RecoveryPending,
	})

	v := GuardianRecoveryVeto{
		Kind:               AnchorGuardianRecoveryVeto,
		SubjectQuid:        "subject",
		RecoveryAnchorHash: initHash,
		AnchorNonce:        1,
		ValidFrom:          time.Now().Unix(),
	}
	signable, _ := GuardianRecoveryVetoSignableBytes(v)
	v.PrimarySignature = &PrimarySignature{
		KeyEpoch:  0,
		Signature: signWithGuardianKey(t, subjectPriv, signable),
	}
	if err := ValidateGuardianRecoveryVeto(l, v, time.Now()); err != nil {
		t.Fatalf("primary-key veto: %v", err)
	}
}

func TestValidateGuardianRecoveryVeto_RejectsBothSigPaths(t *testing.T) {
	l := NewNonceLedger()
	v := GuardianRecoveryVeto{
		Kind:               AnchorGuardianRecoveryVeto,
		SubjectQuid:        "subject",
		RecoveryAnchorHash: "hash",
		AnchorNonce:        1,
		ValidFrom:          time.Now().Unix(),
		PrimarySignature:   &PrimarySignature{KeyEpoch: 0, Signature: "a"},
		GuardianSigs:       []GuardianSignature{{GuardianQuid: "g", KeyEpoch: 0, Signature: "b"}},
	}
	if err := ValidateGuardianRecoveryVeto(l, v, time.Now()); !errors.Is(err, ErrGuardianVetoAmbiguous) {
		t.Fatalf("want ErrGuardianVetoAmbiguous, got %v", err)
	}
}

func TestValidateGuardianRecoveryVeto_RejectsNeitherSigPath(t *testing.T) {
	l := NewNonceLedger()
	v := GuardianRecoveryVeto{
		Kind:               AnchorGuardianRecoveryVeto,
		SubjectQuid:        "subject",
		RecoveryAnchorHash: "hash",
		AnchorNonce:        1,
		ValidFrom:          time.Now().Unix(),
	}
	if err := ValidateGuardianRecoveryVeto(l, v, time.Now()); !errors.Is(err, ErrGuardianVetoAmbiguous) {
		t.Fatalf("want ErrGuardianVetoAmbiguous, got %v", err)
	}
}

func TestValidateGuardianRecoveryVeto_RejectsMissingPendingRecovery(t *testing.T) {
	l := NewNonceLedger()
	_, subjectPub := keypairHex(t)
	l.SetSignerKey("subject", 0, subjectPub)

	v := GuardianRecoveryVeto{
		Kind:               AnchorGuardianRecoveryVeto,
		SubjectQuid:        "subject",
		RecoveryAnchorHash: "nonexistent",
		AnchorNonce:        1,
		ValidFrom:          time.Now().Unix(),
		PrimarySignature:   &PrimarySignature{KeyEpoch: 0, Signature: "aa"},
	}
	if err := ValidateGuardianRecoveryVeto(l, v, time.Now()); !errors.Is(err, ErrGuardianRecoveryNotPending) {
		t.Fatalf("want ErrGuardianRecoveryNotPending, got %v", err)
	}
}

// ----- GuardianRecoveryCommit ----------------------------------------------

func TestValidateGuardianRecoveryCommit_RejectsImmature(t *testing.T) {
	l := NewNonceLedger()
	committer := newGuardian(t, "committer")
	l.SetSignerKey(committer.quid, 0, committer.pub)

	initHash := "immature-init"
	// Maturity 1 hour in the future.
	l.beginPendingRecovery("subject", &PendingRecovery{
		InitHash:     initHash,
		MaturityUnix: time.Now().Add(1 * time.Hour).Unix(),
		State:        RecoveryPending,
	})

	c := GuardianRecoveryCommit{
		Kind:               AnchorGuardianRecoveryCommit,
		SubjectQuid:        "subject",
		RecoveryAnchorHash: initHash,
		AnchorNonce:        1,
		ValidFrom:          time.Now().Unix(),
		CommitterQuid:      committer.quid,
		CommitterSig:       signWithGuardianKey(t, committer.priv, []byte(initHash)),
	}
	if err := ValidateGuardianRecoveryCommit(l, c, time.Now()); !errors.Is(err, ErrGuardianRecoveryImmature) {
		t.Fatalf("want ErrGuardianRecoveryImmature, got %v", err)
	}
}

func TestValidateGuardianRecoveryCommit_HappyPath(t *testing.T) {
	l := NewNonceLedger()
	committer := newGuardian(t, "committer")
	l.SetSignerKey(committer.quid, 0, committer.pub)

	initHash := "mature-init"
	// Maturity 1 hour in the past.
	l.beginPendingRecovery("subject", &PendingRecovery{
		InitHash:     initHash,
		MaturityUnix: time.Now().Add(-1 * time.Hour).Unix(),
		State:        RecoveryPending,
	})

	c := GuardianRecoveryCommit{
		Kind:               AnchorGuardianRecoveryCommit,
		SubjectQuid:        "subject",
		RecoveryAnchorHash: initHash,
		AnchorNonce:        1,
		ValidFrom:          time.Now().Unix(),
		CommitterQuid:      committer.quid,
		CommitterSig:       signWithGuardianKey(t, committer.priv, []byte(initHash)),
	}
	if err := ValidateGuardianRecoveryCommit(l, c, time.Now()); err != nil {
		t.Fatalf("mature commit: %v", err)
	}
}
