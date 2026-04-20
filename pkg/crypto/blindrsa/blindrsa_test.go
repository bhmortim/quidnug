package blindrsa

import (
	"crypto/rand"
	"crypto/rsa"
	"math/big"
	"testing"
)

// Use a small modulus for fast tests. Production requires
// 2048 minimum; QDP-0021 mandates 3072. This test is about
// protocol correctness, not modulus size.
func makeTestKey(t *testing.T) *rsa.PrivateKey {
	t.Helper()
	k, err := GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("keygen: %v", err)
	}
	return k
}

func TestFullRoundtrip(t *testing.T) {
	k := makeTestKey(t)
	pub := &k.PublicKey

	token := []byte("ballot-token-32-bytes-long-token")
	if len(token) < 16 {
		t.Fatal("token too short for the test")
	}

	// 1. Voter encodes + blinds.
	m, err := FDHEncode(token, pub.N)
	if err != nil {
		t.Fatalf("encode: %v", err)
	}
	blinded, r, err := Blind(rand.Reader, pub, m)
	if err != nil {
		t.Fatalf("blind: %v", err)
	}

	// 2. Authority signs blinded (never sees the token).
	sBlinded := SignBlinded(k, blinded)

	// 3. Voter unblinds.
	s := Unblind(pub, sBlinded, r)
	if s == nil {
		t.Fatal("unblind returned nil")
	}

	// 4. Anyone verifies.
	if err := Verify(pub, token, s); err != nil {
		t.Errorf("verify: %v", err)
	}
}

func TestTamperedSignatureFails(t *testing.T) {
	k := makeTestKey(t)
	pub := &k.PublicKey
	token := []byte("ballot-token-A")

	m, _ := FDHEncode(token, pub.N)
	blinded, r, _ := Blind(rand.Reader, pub, m)
	sBlinded := SignBlinded(k, blinded)
	s := Unblind(pub, sBlinded, r)

	// Tamper s by adding 1 mod n.
	sBad := new(big.Int).Add(s, big.NewInt(1))
	sBad.Mod(sBad, pub.N)
	if err := Verify(pub, token, sBad); err == nil {
		t.Error("tampered signature incorrectly verified")
	}
}

func TestDifferentTokenFails(t *testing.T) {
	k := makeTestKey(t)
	pub := &k.PublicKey
	tokenA := []byte("token-A")
	tokenB := []byte("token-B")

	m, _ := FDHEncode(tokenA, pub.N)
	blinded, r, _ := Blind(rand.Reader, pub, m)
	sBlinded := SignBlinded(k, blinded)
	s := Unblind(pub, sBlinded, r)

	// Signature on tokenA should NOT verify against tokenB.
	if err := Verify(pub, tokenB, s); err == nil {
		t.Error("signature on token A verified against token B")
	}
	// And it should verify against tokenA.
	if err := Verify(pub, tokenA, s); err != nil {
		t.Errorf("valid signature rejected: %v", err)
	}
}

func TestBlindingHidesTokenFromSigner(t *testing.T) {
	// Protocol guarantee: the authority signing the blinded
	// value cannot recover the original token.
	//
	// We simulate this by showing that for a fixed token, two
	// different blindings produce two different blinded
	// values that both look random to the signer, yet both
	// unblind to valid signatures over the same token.
	k := makeTestKey(t)
	pub := &k.PublicKey
	token := []byte("unique-ballot")

	m, _ := FDHEncode(token, pub.N)

	blinded1, r1, _ := Blind(rand.Reader, pub, m)
	blinded2, r2, _ := Blind(rand.Reader, pub, m)

	if blinded1.Cmp(blinded2) == 0 {
		t.Error("two blindings of the same message produced identical bytes — catastrophic")
	}

	s1 := Unblind(pub, SignBlinded(k, blinded1), r1)
	s2 := Unblind(pub, SignBlinded(k, blinded2), r2)

	// Both signatures should validate against the original token.
	if err := Verify(pub, token, s1); err != nil {
		t.Errorf("s1 verify: %v", err)
	}
	if err := Verify(pub, token, s2); err != nil {
		t.Errorf("s2 verify: %v", err)
	}
	// And both signatures happen to be the same mathematical
	// value (RSA-FDH is deterministic for a given message).
	if s1.Cmp(s2) != 0 {
		t.Errorf("RSA-FDH signatures should be deterministic: %v vs %v", s1, s2)
	}
}

func TestPublicKeyFingerprint_Stable(t *testing.T) {
	k := makeTestKey(t)
	pub := &k.PublicKey
	fp1 := PublicKeyFingerprint(pub)
	fp2 := PublicKeyFingerprint(pub)
	if fp1 != fp2 {
		t.Errorf("fingerprint unstable: %s vs %s", fp1, fp2)
	}
	if len(fp1) != 64 {
		t.Errorf("fingerprint wrong length: %d", len(fp1))
	}
	// Different key → different fingerprint.
	k2 := makeTestKey(t)
	fp3 := PublicKeyFingerprint(&k2.PublicKey)
	if fp1 == fp3 {
		t.Error("different keys produced same fingerprint")
	}
}

func TestFDHEncode_Deterministic(t *testing.T) {
	k := makeTestKey(t)
	pub := &k.PublicKey
	token := []byte("same-token")
	m1, _ := FDHEncode(token, pub.N)
	m2, _ := FDHEncode(token, pub.N)
	if m1.Cmp(m2) != 0 {
		t.Error("FDH encoding not deterministic")
	}
}
