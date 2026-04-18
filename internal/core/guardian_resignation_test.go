// Package core — guardian_resignation_test.go
//
// Methodology
// -----------
// Guardian resignation (QDP-0006 / H6) is the first lifecycle-
// revocation path in the guardian system. These tests guard:
//
//   - The resignation signature chain is independent of the
//     subject. A guardian can resign with only their OWN key;
//     the subject's cooperation is never required.
//
//   - Set-hash binding prevents stale resignations: if the set
//     is updated after a resignation is signed, the resignation
//     is rejected with a specific error so the guardian re-signs.
//
//   - Prospective effect (not retroactive). A resignation
//     AFTER a recovery Init does NOT unwind the Init's
//     authorization. Tested explicitly because this is the
//     subtle invariant from QDP-0006 §7.
//
//   - The raw GuardianSet is never mutated by resignations —
//     only the overlay-accessor EffectiveGuardianSet returns
//     the zeroed-weight view. Audit integrity preserved.
//
//   - Weakened-set metric triggers iff effective weight drops
//     below threshold.
//
// Each test isolates exactly one concern so regressions are
// pinpoint-diagnosable.
package core

import (
	"encoding/json"
	"errors"
	"testing"
	"time"
)

// ----- helpers -------------------------------------------------------------

// newResignSetup creates a subject + N guardians, installs a
// simple guardian set with equal weights, and seeds the ledger
// with each guardian's epoch-0 key so signatures validate.
type resignSetup struct {
	subject   *QuidnugNode
	guardians []*QuidnugNode
	ledger    *NonceLedger
	setHash   string
	set       *GuardianSet
}

func newResignSetup(t *testing.T, numGuardians int, threshold uint16) *resignSetup {
	t.Helper()
	s := &resignSetup{
		subject: newTestNode(),
		ledger:  NewNonceLedger(),
	}
	// Seed the subject's own epoch-0 key for subject-side
	// sigs (none needed here, but matches realistic setup).
	s.ledger.SetSignerKey(s.subject.NodeID, 0, s.subject.GetPublicKeyHex())

	guardians := make([]GuardianRef, 0, numGuardians)
	for i := 0; i < numGuardians; i++ {
		g := newTestNode()
		s.guardians = append(s.guardians, g)
		s.ledger.SetSignerKey(g.NodeID, 0, g.GetPublicKeyHex())
		guardians = append(guardians, GuardianRef{
			Quid:   g.NodeID,
			Weight: 1,
			Epoch:  0,
		})
	}

	set := &GuardianSet{
		Guardians:     guardians,
		Threshold:     threshold,
		RecoveryDelay: MinRecoveryDelay,
	}
	s.set = set
	s.ledger.setGuardianSet(s.subject.NodeID, set)

	hash, err := GuardianSetHashOf(set)
	if err != nil {
		t.Fatalf("set hash: %v", err)
	}
	s.setHash = hash
	return s
}

// buildResignation constructs a signed resignation for guardians[idx].
func (s *resignSetup) buildResignation(t *testing.T, idx int, nonce int64, effectiveAt int64) GuardianResignation {
	t.Helper()
	r := GuardianResignation{
		Kind:             AnchorGuardianResign,
		GuardianQuid:     s.guardians[idx].NodeID,
		SubjectQuid:      s.subject.NodeID,
		GuardianSetHash:  s.setHash,
		ResignationNonce: nonce,
		EffectiveAt:      effectiveAt,
	}
	signed, err := s.guardians[idx].SignGuardianResignation(r)
	if err != nil {
		t.Fatalf("sign: %v", err)
	}
	return signed
}

// ----- Validation rejection paths ------------------------------------------

func TestResignation_UnknownSubject(t *testing.T) {
	s := newResignSetup(t, 3, 2)
	r := s.buildResignation(t, 0, 1, time.Now().Unix())
	r.SubjectQuid = "nonexistent-subject"

	err := ValidateGuardianResignation(s.ledger, r, time.Now())
	if !errors.Is(err, ErrResignationSubjectUnknown) {
		t.Fatalf("want ErrResignationSubjectUnknown, got %v", err)
	}
}

func TestResignation_NotMember(t *testing.T) {
	s := newResignSetup(t, 3, 2)
	stranger := newTestNode()
	s.ledger.SetSignerKey(stranger.NodeID, 0, stranger.GetPublicKeyHex())

	r := GuardianResignation{
		Kind:             AnchorGuardianResign,
		GuardianQuid:     stranger.NodeID,
		SubjectQuid:      s.subject.NodeID,
		GuardianSetHash:  s.setHash,
		ResignationNonce: 1,
		EffectiveAt:      time.Now().Unix(),
	}
	r, _ = stranger.SignGuardianResignation(r)

	err := ValidateGuardianResignation(s.ledger, r, time.Now())
	if !errors.Is(err, ErrResignationNotMember) {
		t.Fatalf("want ErrResignationNotMember, got %v", err)
	}
}

func TestResignation_SetHashMismatch(t *testing.T) {
	s := newResignSetup(t, 3, 2)
	r := s.buildResignation(t, 0, 1, time.Now().Unix())
	r.GuardianSetHash = "stale-hash-of-a-prior-set-version"
	r, _ = s.guardians[0].SignGuardianResignation(r)

	err := ValidateGuardianResignation(s.ledger, r, time.Now())
	if !errors.Is(err, ErrResignationSetHashMismatch) {
		t.Fatalf("want ErrResignationSetHashMismatch, got %v", err)
	}
}

func TestResignation_NonceReplay(t *testing.T) {
	s := newResignSetup(t, 3, 2)
	r := s.buildResignation(t, 0, 5, time.Now().Unix())

	// First accept: store nonce at 5.
	s.ledger.storeGuardianResignation(r)

	// Replay with same nonce.
	err := ValidateGuardianResignation(s.ledger, r, time.Now())
	if !errors.Is(err, ErrResignationReplay) {
		t.Fatalf("want ErrResignationReplay, got %v", err)
	}
	// Nonce below stored: also replay.
	r2 := s.buildResignation(t, 0, 3, time.Now().Unix())
	err = ValidateGuardianResignation(s.ledger, r2, time.Now())
	if !errors.Is(err, ErrResignationReplay) {
		t.Fatalf("want ErrResignationReplay on lower nonce, got %v", err)
	}
}

func TestResignation_EffectiveAtPast(t *testing.T) {
	s := newResignSetup(t, 3, 2)
	past := time.Now().Add(-10 * time.Minute).Unix()
	r := s.buildResignation(t, 0, 1, past)

	err := ValidateGuardianResignation(s.ledger, r, time.Now())
	if !errors.Is(err, ErrResignationEffectiveAtPast) {
		t.Fatalf("want ErrResignationEffectiveAtPast, got %v", err)
	}
}

func TestResignation_EffectiveAtTooFar(t *testing.T) {
	s := newResignSetup(t, 3, 2)
	future := time.Now().Add(400 * 24 * time.Hour).Unix()
	r := s.buildResignation(t, 0, 1, future)

	err := ValidateGuardianResignation(s.ledger, r, time.Now())
	if !errors.Is(err, ErrResignationEffectiveAtTooFar) {
		t.Fatalf("want ErrResignationEffectiveAtTooFar, got %v", err)
	}
}

func TestResignation_BadSignature(t *testing.T) {
	s := newResignSetup(t, 3, 2)
	r := s.buildResignation(t, 0, 1, time.Now().Unix())
	// Tamper after signing.
	r.EffectiveAt += 60

	err := ValidateGuardianResignation(s.ledger, r, time.Now())
	if !errors.Is(err, ErrResignationBadSignature) {
		t.Fatalf("want ErrResignationBadSignature, got %v", err)
	}
}

// ----- Happy path: effective set reflects resignation ----------------------

func TestResignation_EffectiveSetReducesWeight(t *testing.T) {
	s := newResignSetup(t, 3, 2)
	r := s.buildResignation(t, 0, 1, time.Now().Unix())

	if err := ValidateGuardianResignation(s.ledger, r, time.Now()); err != nil {
		t.Fatalf("validate: %v", err)
	}
	s.ledger.storeGuardianResignation(r)

	// Raw set still has 3 guardians at weight 1.
	raw := s.ledger.GuardianSetOf(s.subject.NodeID)
	if len(raw.Guardians) != 3 {
		t.Fatalf("raw guardians count: want 3, got %d", len(raw.Guardians))
	}

	// Effective set zeros the resigned guardian.
	effective := s.ledger.EffectiveGuardianSet(s.subject.NodeID, time.Now())
	var zeroed int
	for _, g := range effective.Guardians {
		if g.Weight == 0 {
			zeroed++
		}
	}
	if zeroed != 1 {
		t.Fatalf("effective set should have 1 zeroed guardian, got %d", zeroed)
	}
}

// Future-effective resignations are stored but not yet active.
func TestResignation_FutureEffectiveAtDelayed(t *testing.T) {
	s := newResignSetup(t, 3, 2)
	future := time.Now().Add(24 * time.Hour).Unix()
	r := s.buildResignation(t, 0, 1, future)

	s.ledger.storeGuardianResignation(r)

	// Before effective: no zeroing.
	effective := s.ledger.EffectiveGuardianSet(s.subject.NodeID, time.Now())
	for _, g := range effective.Guardians {
		if g.Weight == 0 {
			t.Fatal("future resignation should not yet reduce weight")
		}
	}

	// After effective: zeroing kicks in.
	later := time.Now().Add(25 * time.Hour)
	effective = s.ledger.EffectiveGuardianSet(s.subject.NodeID, later)
	var zeroed int
	for _, g := range effective.Guardians {
		if g.Weight == 0 {
			zeroed++
		}
	}
	if zeroed != 1 {
		t.Fatalf("after effective, want 1 zeroed, got %d", zeroed)
	}
}

// ----- Set-hash binding ----------------------------------------------------

// A resignation signed against a set that's since been replaced
// with a different one is no longer valid — the old set's
// hash doesn't match the new installed set.
func TestResignation_StaleAgainstNewSet(t *testing.T) {
	s := newResignSetup(t, 3, 2)
	r := s.buildResignation(t, 0, 1, time.Now().Unix())

	// Replace the set with a different one.
	s.ledger.setGuardianSet(s.subject.NodeID, &GuardianSet{
		Guardians: []GuardianRef{
			{Quid: s.guardians[0].NodeID, Weight: 2, Epoch: 0},
			{Quid: s.guardians[1].NodeID, Weight: 2, Epoch: 0},
		},
		Threshold:     3,
		RecoveryDelay: MinRecoveryDelay,
	})

	err := ValidateGuardianResignation(s.ledger, r, time.Now())
	if !errors.Is(err, ErrResignationSetHashMismatch) {
		t.Fatalf("want ErrResignationSetHashMismatch against new set, got %v", err)
	}
}

// Resignations for OLD set versions are ignored by
// EffectiveGuardianSet (stored for audit but don't reduce
// current-set weight).
func TestResignation_OldVersionIgnoredByEffectiveSet(t *testing.T) {
	s := newResignSetup(t, 3, 2)
	r := s.buildResignation(t, 0, 1, time.Now().Unix())
	s.ledger.storeGuardianResignation(r)

	// New set with different hash.
	newSet := &GuardianSet{
		Guardians: []GuardianRef{
			{Quid: s.guardians[0].NodeID, Weight: 1, Epoch: 0},
			{Quid: s.guardians[1].NodeID, Weight: 1, Epoch: 0},
			{Quid: s.guardians[2].NodeID, Weight: 1, Epoch: 0},
		},
		Threshold:     2,
		RecoveryDelay: 2 * MinRecoveryDelay, // different to change hash
	}
	s.ledger.setGuardianSet(s.subject.NodeID, newSet)

	effective := s.ledger.EffectiveGuardianSet(s.subject.NodeID, time.Now())
	for _, g := range effective.Guardians {
		if g.Weight == 0 {
			t.Fatal("old-version resignation should not affect new set")
		}
	}
}

// ----- Weakened detection --------------------------------------------------

// 2-of-3: one resignation leaves 2 — threshold still met, NOT
// weakened.
func TestWeakened_OneOfThreeResigns(t *testing.T) {
	s := newResignSetup(t, 3, 2)
	r := s.buildResignation(t, 0, 1, time.Now().Unix())
	s.ledger.storeGuardianResignation(r)

	if s.ledger.GuardianSetIsWeakened(s.subject.NodeID, time.Now()) {
		t.Fatal("2-of-3 with 1 resigned should not be weakened")
	}
}

// 2-of-3: two resignations leave 1 — threshold unreachable,
// WEAKENED.
func TestWeakened_TwoOfThreeResigns(t *testing.T) {
	s := newResignSetup(t, 3, 2)
	s.ledger.storeGuardianResignation(s.buildResignation(t, 0, 1, time.Now().Unix()))
	s.ledger.storeGuardianResignation(s.buildResignation(t, 1, 1, time.Now().Unix()))

	if !s.ledger.GuardianSetIsWeakened(s.subject.NodeID, time.Now()) {
		t.Fatal("2-of-3 with 2 resigned should be weakened")
	}
}

// ----- Idempotency / marshaling --------------------------------------------

// JSON roundtrip preserves all fields including the signature.
// Guards against a silent struct-tag regression.
func TestResignation_JSONRoundtrip(t *testing.T) {
	s := newResignSetup(t, 3, 2)
	r := s.buildResignation(t, 0, 42, time.Now().Unix())

	b, err := json.Marshal(r)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var back GuardianResignation
	if err := json.Unmarshal(b, &back); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if back != r {
		t.Fatalf("roundtrip changed struct: want=%+v got=%+v", r, back)
	}
}

// ----- Apply-path smoke ----------------------------------------------------

// End-to-end: applyGuardianResignation via a Trusted block
// produces the same effect as storeGuardianResignation directly.
// Lightweight because processBlockTransactions wraps apply in an
// error-tolerant log path; we just verify the stored state.
func TestResignation_ApplyViaBlock(t *testing.T) {
	s := newResignSetup(t, 3, 2)
	node := &QuidnugNode{NonceLedger: s.ledger}

	r := s.buildResignation(t, 0, 1, time.Now().Unix())
	block := Block{Index: 42, TrustProof: TrustProof{TrustDomain: "d"}}
	node.applyGuardianResignation(r, block)

	stored := s.ledger.GuardianResignationsOf(s.subject.NodeID)
	if len(stored) != 1 || stored[0].ResignationNonce != 1 {
		t.Fatalf("applyGuardianResignation did not store: %+v", stored)
	}
	if got := s.ledger.GuardianResignationNonce(s.guardians[0].NodeID, s.subject.NodeID); got != 1 {
		t.Fatalf("nonce not tracked: %d", got)
	}
}

// ----- Mid-flight recovery semantics (QDP-0006 §7) -------------------------

// The key QDP-0006 invariant: a resignation AFTER an Init is
// prospective only. The Init's authorization was computed
// against the set at Init time, and a subsequent resignation
// does NOT invalidate it.
//
// We verify this at the data-model level: after a resignation
// is stored, the PendingRecovery record (which was created
// from the pre-resignation Init) is still present and
// unchanged.
func TestResignation_DoesNotUnwindPendingRecovery(t *testing.T) {
	s := newResignSetup(t, 3, 2)

	// Simulate a pending recovery (bypass Init validation since
	// that's not the subject of this test — we just need a
	// PendingRecovery record in the ledger).
	s.ledger.beginPendingRecovery(s.subject.NodeID, &PendingRecovery{
		InitHash:        "pretend-init-hash",
		InitBlockHeight: 10,
		MaturityUnix:    time.Now().Add(time.Hour).Unix(),
		State:           RecoveryPending,
	})

	// Then a guardian resigns.
	r := s.buildResignation(t, 0, 1, time.Now().Unix())
	s.ledger.storeGuardianResignation(r)

	// Pending recovery is untouched.
	pr := s.ledger.PendingRecoveryOf(s.subject.NodeID)
	if pr == nil {
		t.Fatal("pending recovery should still exist after guardian resignation")
	}
	if pr.InitHash != "pretend-init-hash" {
		t.Fatalf("pending recovery mutated: %+v", pr)
	}
}
