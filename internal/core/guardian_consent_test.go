// Package core — guardian_consent_test.go
//
// Methodology
// -----------
// Separate file for the consent + weighted-voting + RequireGuardianRotation
// features added in the "finish guardian work" pass. Keeping them out of
// guardian_test.go (which covers the core validate/apply logic) makes
// the intent of each test obvious: these are the extended-authorization
// rules, not basic validation.
//
// Three tranches:
//
//   1. New-guardian consent: the network rejects a SetUpdate that
//      lists a guardian without that guardian's on-chain signature.
//      Also rejects the reverse error — a stray signature from a
//      quid not in the new set.
//
//   2. Weighted thresholds: TotalWeight / Threshold math works when
//      guardians have unequal weights, including the edge where a
//      single high-weight guardian can singlehandedly meet
//      threshold.
//
//   3. RequireGuardianRotation: a plain AnchorRotation is rejected
//      when the subject has opted into guardian-only rotation.
package core

import (
	"encoding/hex"
	"errors"
	"testing"
	"time"
)

// ----- New-guardian consent -------------------------------------------------

// TestValidateGuardianSetUpdate_RequiresConsentFromEveryNewGuardian
// omits the consent for one of the three proposed guardians; the
// validation must reject with ErrGuardianMissingConsent.
func TestValidateGuardianSetUpdate_RequiresConsentFromEveryNewGuardian(t *testing.T) {
	l := NewNonceLedger()
	subjectPriv, subjectPub := keypairHex(t)
	l.SetSignerKey("subject", 0, subjectPub)

	gs := []guardian{
		newGuardian(t, "g1"),
		newGuardian(t, "g2"),
		newGuardian(t, "g3"),
	}
	for _, g := range gs {
		l.SetSignerKey(g.quid, 0, g.pub)
	}
	set := buildSet(gs, 2, 1*time.Hour)

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
	// Only 2 of 3 guardians consent — intentionally missing gs[2].
	u.NewGuardianConsents = consentsFromAll(t, gs[:2], signable)

	if err := ValidateGuardianSetUpdate(l, u, time.Now()); !errors.Is(err, ErrGuardianMissingConsent) {
		t.Fatalf("want ErrGuardianMissingConsent, got %v", err)
	}
}

// TestValidateGuardianSetUpdate_RejectsConsentFromOutsider verifies
// the flip side: a signature from a quid not in the new set doesn't
// count, even if numerically we'd have N-1 "real" + 1 outsider = N.
func TestValidateGuardianSetUpdate_RejectsConsentFromOutsider(t *testing.T) {
	l := NewNonceLedger()
	subjectPriv, subjectPub := keypairHex(t)
	l.SetSignerKey("subject", 0, subjectPub)

	gs := []guardian{newGuardian(t, "g1"), newGuardian(t, "g2")}
	for _, g := range gs {
		l.SetSignerKey(g.quid, 0, g.pub)
	}
	outsider := newGuardian(t, "outsider")
	l.SetSignerKey(outsider.quid, 0, outsider.pub)
	set := buildSet(gs, 1, 1*time.Hour)

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
	// g1 signs legitimately; outsider adds a stray signature hoping to
	// count toward consent. The second one should be rejected as
	// UnknownGuardian.
	u.NewGuardianConsents = []GuardianSignature{
		{GuardianQuid: gs[0].quid, KeyEpoch: 0, Signature: signWithGuardianKey(t, gs[0].priv, signable)},
		{GuardianQuid: outsider.quid, KeyEpoch: 0, Signature: signWithGuardianKey(t, outsider.priv, signable)},
	}

	if err := ValidateGuardianSetUpdate(l, u, time.Now()); !errors.Is(err, ErrGuardianUnknownGuardian) {
		t.Fatalf("want ErrGuardianUnknownGuardian, got %v", err)
	}
}

// TestValidateGuardianSetUpdate_RejectsForgedConsent verifies that a
// guardian claim signed by the wrong key is rejected. This isn't a
// dramatic threat (the real guardian wouldn't have signed), but it
// guards against someone trying to consent on another guardian's
// behalf with their own signature.
func TestValidateGuardianSetUpdate_RejectsForgedConsent(t *testing.T) {
	l := NewNonceLedger()
	subjectPriv, subjectPub := keypairHex(t)
	l.SetSignerKey("subject", 0, subjectPub)

	gs := []guardian{newGuardian(t, "g1"), newGuardian(t, "g2")}
	for _, g := range gs {
		l.SetSignerKey(g.quid, 0, g.pub)
	}
	set := buildSet(gs, 1, 1*time.Hour)

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
	// g1 "claims" consent but the signature is g2's private key.
	u.NewGuardianConsents = []GuardianSignature{
		{
			GuardianQuid: gs[0].quid,
			KeyEpoch:     0,
			Signature:    signWithGuardianKey(t, gs[1].priv, signable), // wrong key
		},
		{
			GuardianQuid: gs[1].quid,
			KeyEpoch:     0,
			Signature:    signWithGuardianKey(t, gs[1].priv, signable),
		},
	}

	if err := ValidateGuardianSetUpdate(l, u, time.Now()); !errors.Is(err, ErrAnchorBadSignature) {
		t.Fatalf("want ErrAnchorBadSignature, got %v", err)
	}
}

// ----- Weighted thresholds --------------------------------------------------

// TestWeightedThreshold_SingleHighWeightGuardianMeetsAlone confirms
// that when one guardian carries weight ≥ threshold, that guardian's
// signature alone authorizes a recovery even though it's structurally
// a 1-of-N signing.
func TestWeightedThreshold_SingleHighWeightGuardianMeetsAlone(t *testing.T) {
	l := NewNonceLedger()
	subjectPriv, subjectPub := keypairHex(t)
	l.SetSignerKey("subject", 0, subjectPub)

	g := []guardian{newGuardian(t, "heavy"), newGuardian(t, "light1"), newGuardian(t, "light2")}
	for _, x := range g {
		l.SetSignerKey(x.quid, 0, x.pub)
	}
	set := &GuardianSet{
		Guardians: []GuardianRef{
			{Quid: g[0].quid, Weight: 5, Epoch: 0},
			{Quid: g[1].quid, Weight: 1, Epoch: 0},
			{Quid: g[2].quid, Weight: 1, Epoch: 0},
		},
		Threshold:     5,
		RecoveryDelay: 1 * time.Hour,
	}
	seedGuardianSet(t, l, "subject", set, g)
	_ = subjectPriv

	// Build an Init anchor signed only by the heavy guardian. Should
	// meet threshold exactly (5 == 5).
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
		{GuardianQuid: g[0].quid, KeyEpoch: 0, Signature: signWithGuardianKey(t, g[0].priv, signable)},
	}
	if err := ValidateGuardianRecoveryInit(l, a, time.Now()); err != nil {
		t.Fatalf("high-weight alone should meet threshold: %v", err)
	}
}

// TestWeightedThreshold_LightGuardianAloneFails checks the mirror: a
// single light guardian (weight=1) cannot meet a threshold of 5.
func TestWeightedThreshold_LightGuardianAloneFails(t *testing.T) {
	l := NewNonceLedger()

	g := []guardian{newGuardian(t, "heavy"), newGuardian(t, "light1"), newGuardian(t, "light2")}
	for _, x := range g {
		l.SetSignerKey(x.quid, 0, x.pub)
	}
	set := &GuardianSet{
		Guardians: []GuardianRef{
			{Quid: g[0].quid, Weight: 5, Epoch: 0},
			{Quid: g[1].quid, Weight: 1, Epoch: 0},
			{Quid: g[2].quid, Weight: 1, Epoch: 0},
		},
		Threshold:     5,
		RecoveryDelay: 1 * time.Hour,
	}
	seedGuardianSet(t, l, "subject", set, g)

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
		{GuardianQuid: g[1].quid, KeyEpoch: 0, Signature: signWithGuardianKey(t, g[1].priv, signable)},
	}
	if err := ValidateGuardianRecoveryInit(l, a, time.Now()); !errors.Is(err, ErrGuardianInsufficientSigs) {
		t.Fatalf("light guardian alone should fail threshold: %v", err)
	}
}

// TestWeightedThreshold_LightGuardiansCombineToMeet confirms weighted
// sums accumulate correctly across multiple signers.
func TestWeightedThreshold_LightGuardiansCombineToMeet(t *testing.T) {
	l := NewNonceLedger()

	g := []guardian{
		newGuardian(t, "w1"),
		newGuardian(t, "w2"),
		newGuardian(t, "w3"),
		newGuardian(t, "w4"),
	}
	for _, x := range g {
		l.SetSignerKey(x.quid, 0, x.pub)
	}
	set := &GuardianSet{
		Guardians: []GuardianRef{
			{Quid: g[0].quid, Weight: 2, Epoch: 0},
			{Quid: g[1].quid, Weight: 2, Epoch: 0},
			{Quid: g[2].quid, Weight: 1, Epoch: 0},
			{Quid: g[3].quid, Weight: 1, Epoch: 0},
		},
		Threshold:     4, // need any three of the light ones, or either pair of 2s
		RecoveryDelay: 1 * time.Hour,
	}
	seedGuardianSet(t, l, "subject", set, g)

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

	// 2 + 2 = 4 via the two heavy-ish ones. Meets threshold.
	a.GuardianSigs = []GuardianSignature{
		{GuardianQuid: g[0].quid, KeyEpoch: 0, Signature: signWithGuardianKey(t, g[0].priv, signable)},
		{GuardianQuid: g[1].quid, KeyEpoch: 0, Signature: signWithGuardianKey(t, g[1].priv, signable)},
	}
	if err := ValidateGuardianRecoveryInit(l, a, time.Now()); err != nil {
		t.Fatalf("two weight-2 guardians should meet threshold=4: %v", err)
	}
}

// ----- RequireGuardianRotation ---------------------------------------------

// TestAnchor_RequireGuardianRotationBlocksSelfRotation verifies that
// a subject whose GuardianSet carries RequireGuardianRotation=true
// cannot publish a plain AnchorRotation, even with a valid primary-
// key signature. Only the guardian-recovery path is admissible.
func TestAnchor_RequireGuardianRotationBlocksSelfRotation(t *testing.T) {
	l := NewNonceLedger()
	subjectPriv, subjectPub := keypairHex(t)
	l.SetSignerKey("subject", 0, subjectPub)

	gs := []guardian{newGuardian(t, "g1"), newGuardian(t, "g2")}
	set := &GuardianSet{
		Guardians: []GuardianRef{
			{Quid: gs[0].quid, Weight: 1, Epoch: 0},
			{Quid: gs[1].quid, Weight: 1, Epoch: 0},
		},
		Threshold:               1,
		RecoveryDelay:           1 * time.Hour,
		RequireGuardianRotation: true,
	}
	l.setGuardianSet("subject", set)

	// Build a plain AnchorRotation signed by the subject's primary key.
	_, newPub := keypairHex(t)
	a := NonceAnchor{
		Kind:         AnchorRotation,
		SignerQuid:   "subject",
		FromEpoch:    0,
		ToEpoch:      1,
		NewPublicKey: newPub,
		MinNextNonce: 1,
		AnchorNonce:  1,
		ValidFrom:    time.Now().Unix(),
	}
	signable, _ := GetAnchorSignableData(a)
	sig, err := signWithKey(subjectPriv, signable)
	if err != nil {
		t.Fatalf("sign: %v", err)
	}
	a.Signature = hex.EncodeToString(sig)

	if err := ValidateAnchor(l, a, time.Now()); !errors.Is(err, ErrGuardianRotationForbidden) {
		t.Fatalf("want ErrGuardianRotationForbidden, got %v", err)
	}
}

// TestAnchor_RequireGuardianRotationAllowsWhenFlagUnset is the
// control: with the flag false, self-rotation works normally.
func TestAnchor_RequireGuardianRotationAllowsWhenFlagUnset(t *testing.T) {
	l := NewNonceLedger()
	subjectPriv, subjectPub := keypairHex(t)
	l.SetSignerKey("subject", 0, subjectPub)

	gs := []guardian{newGuardian(t, "g1"), newGuardian(t, "g2")}
	set := &GuardianSet{
		Guardians: []GuardianRef{
			{Quid: gs[0].quid, Weight: 1, Epoch: 0},
			{Quid: gs[1].quid, Weight: 1, Epoch: 0},
		},
		Threshold:     1,
		RecoveryDelay: 1 * time.Hour,
		// RequireGuardianRotation: false (default)
	}
	l.setGuardianSet("subject", set)

	_, newPub := keypairHex(t)
	a := NonceAnchor{
		Kind:         AnchorRotation,
		SignerQuid:   "subject",
		FromEpoch:    0,
		ToEpoch:      1,
		NewPublicKey: newPub,
		MinNextNonce: 1,
		AnchorNonce:  1,
		ValidFrom:    time.Now().Unix(),
	}
	signable, _ := GetAnchorSignableData(a)
	sig, err := signWithKey(subjectPriv, signable)
	if err != nil {
		t.Fatalf("sign: %v", err)
	}
	a.Signature = hex.EncodeToString(sig)

	if err := ValidateAnchor(l, a, time.Now()); err != nil {
		t.Fatalf("self-rotation should be allowed when flag is unset: %v", err)
	}
}
