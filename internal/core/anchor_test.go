// Package core — anchor_test.go
//
// Methodology
// -----------
// Covers the QDP-0001 §6.5 anchor system across all three kinds
// (Rotation, Invalidation, EpochCap). Tests are split into three
// tranches:
//
//  1. ValidateAnchor happy paths — one per kind, all fields correct.
//  2. ValidateAnchor rejection paths — one test per error class
//     exported from anchor.go. This gives us a guarantee that every
//     public error is reachable and recognized by errors.Is.
//  3. Ledger.ApplyAnchor state-transition tests — what each kind
//     actually does to the ledger once validation passes.
//
// Why the three-kind coverage matters: a common class of bug in
// protocol code is "Rotation works, Invalidation and EpochCap don't"
// because the first is exercised in demos. Testing each explicitly
// catches that whole class.
//
// Signing helpers (keypairHex, signAnchor, signWithKey) use the same
// production SignData path so tests would catch a canonicalization
// drift between signer and verifier.
//
// The test clock (`time.Now()`) is passed in explicitly to
// ValidateAnchor; stale/future tests fabricate a ValidFrom value
// relative to the current moment and verify the window is enforced
// at the documented boundaries (5min future, 30d past).
package core

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"testing"
	"time"
)

// ----- test helpers --------------------------------------------------------

// keypairHex returns (privkeyHex, pubkeyHex) for a fresh P-256 keypair,
// with pubkey in the uncompressed hex form the codebase uses elsewhere.
func keypairHex(t *testing.T) (*ecdsa.PrivateKey, string) {
	t.Helper()
	priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("GenerateKey: %v", err)
	}
	pubBytes := elliptic.Marshal(priv.PublicKey.Curve, priv.PublicKey.X, priv.PublicKey.Y)
	return priv, hex.EncodeToString(pubBytes)
}

// signAnchor produces a signed anchor using the helper node's SignData.
// For test convenience: takes a filled-in anchor with Signature empty
// and returns it with Signature set.
func signAnchor(t *testing.T, priv *ecdsa.PrivateKey, a NonceAnchor) NonceAnchor {
	t.Helper()
	a.Signature = ""
	signable, err := GetAnchorSignableData(a)
	if err != nil {
		t.Fatalf("anchor canonicalization: %v", err)
	}
	// Duplicate of QuidnugNode.SignData but for a raw key, since the
	// anchor may be signed by a different key than the test node.
	sig, err := signWithKey(priv, signable)
	if err != nil {
		t.Fatalf("sign: %v", err)
	}
	a.Signature = hex.EncodeToString(sig)
	return a
}

func signWithKey(priv *ecdsa.PrivateKey, data []byte) ([]byte, error) {
	// Reuse the node helper by constructing a bare QuidnugNode. This
	// also guarantees the wire format matches production.
	n := &QuidnugNode{PrivateKey: priv}
	return n.SignData(data)
}

// ----- ValidateAnchor ------------------------------------------------------

func TestValidateAnchor_RotationHappyPath(t *testing.T) {
	ledger := NewNonceLedger()
	priv, pubHex := keypairHex(t)
	_, newPubHex := keypairHex(t)
	ledger.SetSignerKey("alice", 0, pubHex)

	a := signAnchor(t, priv, NonceAnchor{
		Kind:                AnchorRotation,
		SignerQuid:          "alice",
		FromEpoch:           0,
		ToEpoch:             1,
		NewPublicKey:        newPubHex,
		MinNextNonce:        1,
		MaxAcceptedOldNonce: 10,
		ValidFrom:           time.Now().Unix(),
		AnchorNonce:         1,
	})

	if err := ValidateAnchor(ledger, a, time.Now()); err != nil {
		t.Fatalf("rotation: unexpected error: %v", err)
	}
}

func TestValidateAnchor_InvalidationHappyPath(t *testing.T) {
	ledger := NewNonceLedger()
	priv, pubHex := keypairHex(t)
	ledger.SetSignerKey("alice", 0, pubHex)

	a := signAnchor(t, priv, NonceAnchor{
		Kind:                AnchorInvalidation,
		SignerQuid:          "alice",
		FromEpoch:           0,
		ToEpoch:             0,
		MinNextNonce:        1,
		MaxAcceptedOldNonce: 5,
		ValidFrom:           time.Now().Unix(),
		AnchorNonce:         1,
	})

	if err := ValidateAnchor(ledger, a, time.Now()); err != nil {
		t.Fatalf("invalidation: unexpected error: %v", err)
	}
}

func TestValidateAnchor_EpochCapHappyPath(t *testing.T) {
	ledger := NewNonceLedger()
	priv, pubHex := keypairHex(t)
	ledger.SetSignerKey("alice", 0, pubHex)

	a := signAnchor(t, priv, NonceAnchor{
		Kind:                AnchorEpochCap,
		SignerQuid:          "alice",
		FromEpoch:           0,
		ToEpoch:             0,
		MinNextNonce:        1,
		MaxAcceptedOldNonce: 100,
		ValidFrom:           time.Now().Unix(),
		AnchorNonce:         1,
	})

	if err := ValidateAnchor(ledger, a, time.Now()); err != nil {
		t.Fatalf("epoch cap: unexpected error: %v", err)
	}
}

func TestValidateAnchor_RejectsUnknownKind(t *testing.T) {
	ledger := NewNonceLedger()
	a := NonceAnchor{Kind: AnchorKind(99), SignerQuid: "alice",
		ValidFrom: time.Now().Unix(), AnchorNonce: 1, MinNextNonce: 1}
	if err := ValidateAnchor(ledger, a, time.Now()); !errors.Is(err, ErrAnchorUnknownKind) {
		t.Fatalf("want ErrAnchorUnknownKind, got %v", err)
	}
}

func TestValidateAnchor_RejectsRotationWithoutNewKey(t *testing.T) {
	ledger := NewNonceLedger()
	_, pubHex := keypairHex(t)
	ledger.SetSignerKey("alice", 0, pubHex)

	a := NonceAnchor{Kind: AnchorRotation, SignerQuid: "alice",
		FromEpoch: 0, ToEpoch: 1, MinNextNonce: 1, AnchorNonce: 1,
		ValidFrom: time.Now().Unix()}
	if err := ValidateAnchor(ledger, a, time.Now()); !errors.Is(err, ErrAnchorMissingNewKey) {
		t.Fatalf("want ErrAnchorMissingNewKey, got %v", err)
	}
}

func TestValidateAnchor_RejectsInvalidationWithSpuriousNewKey(t *testing.T) {
	ledger := NewNonceLedger()
	_, pubHex := keypairHex(t)
	_, junkKey := keypairHex(t)
	ledger.SetSignerKey("alice", 0, pubHex)

	a := NonceAnchor{Kind: AnchorInvalidation, SignerQuid: "alice",
		FromEpoch: 0, ToEpoch: 0, NewPublicKey: junkKey,
		MinNextNonce: 1, AnchorNonce: 1, ValidFrom: time.Now().Unix()}
	if err := ValidateAnchor(ledger, a, time.Now()); !errors.Is(err, ErrAnchorSpuriousNewKey) {
		t.Fatalf("want ErrAnchorSpuriousNewKey, got %v", err)
	}
}

func TestValidateAnchor_RejectsBadEpochProgression(t *testing.T) {
	ledger := NewNonceLedger()
	priv, pubHex := keypairHex(t)
	_, newPubHex := keypairHex(t)
	ledger.SetSignerKey("alice", 0, pubHex)

	// Rotation must advance epoch; ToEpoch == FromEpoch is illegal.
	a := signAnchor(t, priv, NonceAnchor{
		Kind: AnchorRotation, SignerQuid: "alice",
		FromEpoch: 0, ToEpoch: 0, NewPublicKey: newPubHex,
		MinNextNonce: 1, AnchorNonce: 1, ValidFrom: time.Now().Unix(),
	})
	if err := ValidateAnchor(ledger, a, time.Now()); !errors.Is(err, ErrAnchorBadEpochProgression) {
		t.Fatalf("want ErrAnchorBadEpochProgression, got %v", err)
	}
}

func TestValidateAnchor_RejectsStaleValidFrom(t *testing.T) {
	ledger := NewNonceLedger()
	priv, pubHex := keypairHex(t)
	ledger.SetSignerKey("alice", 0, pubHex)

	a := signAnchor(t, priv, NonceAnchor{
		Kind: AnchorEpochCap, SignerQuid: "alice",
		FromEpoch: 0, ToEpoch: 0,
		MinNextNonce: 1, MaxAcceptedOldNonce: 5, AnchorNonce: 1,
		ValidFrom: time.Now().Add(-60 * 24 * time.Hour).Unix(), // 60 days old
	})
	if err := ValidateAnchor(ledger, a, time.Now()); !errors.Is(err, ErrAnchorStaleValidFrom) {
		t.Fatalf("want ErrAnchorStaleValidFrom (too old), got %v", err)
	}
}

func TestValidateAnchor_RejectsFutureValidFrom(t *testing.T) {
	ledger := NewNonceLedger()
	priv, pubHex := keypairHex(t)
	ledger.SetSignerKey("alice", 0, pubHex)

	a := signAnchor(t, priv, NonceAnchor{
		Kind: AnchorEpochCap, SignerQuid: "alice",
		FromEpoch: 0, ToEpoch: 0,
		MinNextNonce: 1, MaxAcceptedOldNonce: 5, AnchorNonce: 1,
		ValidFrom: time.Now().Add(1 * time.Hour).Unix(), // 1 hour future (>5min skew)
	})
	if err := ValidateAnchor(ledger, a, time.Now()); !errors.Is(err, ErrAnchorStaleValidFrom) {
		t.Fatalf("want ErrAnchorStaleValidFrom (future), got %v", err)
	}
}

func TestValidateAnchor_RejectsNonMonotonicAnchorNonce(t *testing.T) {
	ledger := NewNonceLedger()
	priv, pubHex := keypairHex(t)
	ledger.SetSignerKey("alice", 0, pubHex)

	// Simulate a prior anchor at nonce 5.
	_ = ledger.ApplyAnchor(NonceAnchor{
		Kind: AnchorEpochCap, SignerQuid: "alice",
		FromEpoch: 0, ToEpoch: 0, AnchorNonce: 5,
	})

	a := signAnchor(t, priv, NonceAnchor{
		Kind: AnchorEpochCap, SignerQuid: "alice",
		FromEpoch: 0, ToEpoch: 0,
		MinNextNonce: 1, MaxAcceptedOldNonce: 10, AnchorNonce: 5, // same as last
		ValidFrom: time.Now().Unix(),
	})
	if err := ValidateAnchor(ledger, a, time.Now()); !errors.Is(err, ErrAnchorNonceNotMonotonic) {
		t.Fatalf("want ErrAnchorNonceNotMonotonic, got %v", err)
	}
}

func TestValidateAnchor_RejectsUnknownSignerKey(t *testing.T) {
	ledger := NewNonceLedger() // nothing seeded
	priv, _ := keypairHex(t)

	a := signAnchor(t, priv, NonceAnchor{
		Kind: AnchorEpochCap, SignerQuid: "ghost",
		FromEpoch: 0, ToEpoch: 0,
		MinNextNonce: 1, MaxAcceptedOldNonce: 5, AnchorNonce: 1,
		ValidFrom: time.Now().Unix(),
	})
	if err := ValidateAnchor(ledger, a, time.Now()); !errors.Is(err, ErrAnchorSignerKeyUnknown) {
		t.Fatalf("want ErrAnchorSignerKeyUnknown, got %v", err)
	}
}

func TestValidateAnchor_RejectsWrongKeySignature(t *testing.T) {
	ledger := NewNonceLedger()
	priv1, pubHex1 := keypairHex(t)
	priv2, _ := keypairHex(t)
	ledger.SetSignerKey("alice", 0, pubHex1) // alice's key is pubHex1
	_ = priv1

	// Sign with the *wrong* key (priv2).
	a := signAnchor(t, priv2, NonceAnchor{
		Kind: AnchorEpochCap, SignerQuid: "alice",
		FromEpoch: 0, ToEpoch: 0,
		MinNextNonce: 1, MaxAcceptedOldNonce: 5, AnchorNonce: 1,
		ValidFrom: time.Now().Unix(),
	})
	if err := ValidateAnchor(ledger, a, time.Now()); !errors.Is(err, ErrAnchorBadSignature) {
		t.Fatalf("want ErrAnchorBadSignature, got %v", err)
	}
}

// ----- ApplyAnchor ---------------------------------------------------------

func TestLedger_ApplyAnchor_Rotation(t *testing.T) {
	ledger := NewNonceLedger()
	_, newPubHex := keypairHex(t)

	a := NonceAnchor{
		Kind:                AnchorRotation,
		SignerQuid:          "alice",
		FromEpoch:           0,
		ToEpoch:             1,
		NewPublicKey:        newPubHex,
		MinNextNonce:        1,
		MaxAcceptedOldNonce: 10,
		AnchorNonce:         1,
	}
	if err := ledger.ApplyAnchor(a); err != nil {
		t.Fatalf("ApplyAnchor: %v", err)
	}
	if got := ledger.CurrentEpoch("alice"); got != 1 {
		t.Fatalf("current epoch: want 1, got %d", got)
	}
	if got, _ := ledger.GetSignerKey("alice", 1); got != newPubHex {
		t.Fatalf("new signer key not stored")
	}
	if got := ledger.EpochCap("alice", 0); got != 10 {
		t.Fatalf("epoch cap: want 10, got %d", got)
	}
	if got := ledger.LastAnchorNonce("alice"); got != 1 {
		t.Fatalf("anchor nonce counter: want 1, got %d", got)
	}
}

func TestLedger_ApplyAnchor_InvalidationFreezesEpoch(t *testing.T) {
	ledger := NewNonceLedger()

	a := NonceAnchor{
		Kind: AnchorInvalidation, SignerQuid: "alice",
		FromEpoch: 0, ToEpoch: 0,
		MaxAcceptedOldNonce: 5, AnchorNonce: 1,
	}
	if err := ledger.ApplyAnchor(a); err != nil {
		t.Fatalf("ApplyAnchor: %v", err)
	}
	if !ledger.IsEpochInvalidated("alice", 0) {
		t.Fatal("expected epoch 0 to be invalidated")
	}
	// Admit should now fail for alice@epoch 0.
	err := ledger.Admit(NonceKey{Quid: "alice", Epoch: 0}, 1)
	if !errors.Is(err, ErrNonceEpochFrozen) {
		t.Fatalf("want ErrNonceEpochFrozen, got %v", err)
	}
}

func TestLedger_ApplyAnchor_EpochCapIsRespectedByAdmit(t *testing.T) {
	ledger := NewNonceLedger()

	_ = ledger.ApplyAnchor(NonceAnchor{
		Kind: AnchorEpochCap, SignerQuid: "alice",
		FromEpoch: 0, ToEpoch: 0,
		MaxAcceptedOldNonce: 10, AnchorNonce: 1,
	})

	// Nonce 10 is at the cap: allowed.
	if err := ledger.Admit(NonceKey{Quid: "alice", Epoch: 0}, 10); err != nil {
		t.Fatalf("cap boundary: unexpected error: %v", err)
	}
	// Nonce 11 exceeds the cap: rejected.
	if err := ledger.Admit(NonceKey{Quid: "alice", Epoch: 0}, 11); !errors.Is(err, ErrNonceEpochCapped) {
		t.Fatalf("want ErrNonceEpochCapped, got %v", err)
	}
}

func TestLedger_ApplyAnchor_RejectsNonMonotonic(t *testing.T) {
	ledger := NewNonceLedger()

	if err := ledger.ApplyAnchor(NonceAnchor{
		Kind: AnchorEpochCap, SignerQuid: "alice", AnchorNonce: 5,
	}); err != nil {
		t.Fatalf("first apply: %v", err)
	}
	// Second apply with same or lower anchor-nonce must fail.
	err := ledger.ApplyAnchor(NonceAnchor{
		Kind: AnchorEpochCap, SignerQuid: "alice", AnchorNonce: 5,
	})
	if !errors.Is(err, ErrNonceNotMonotonic) {
		t.Fatalf("want ErrNonceNotMonotonic, got %v", err)
	}
}
