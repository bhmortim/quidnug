// rfc6979_test.go — correctness tests for the deterministic
// ECDSA implementation.
//
// The reference test vector is from RFC 6979 Appendix A.2.5
// (ECDSA with P-256 and SHA-256). Signing "sample" with the
// canonical test key produces a fixed (r, s) per the RFC.
//
// This test locks down the deterministic signer against the
// authoritative reference so any future change to the k
// derivation is detected immediately.

package core

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/sha256"
	"encoding/hex"
	"math/big"
	"testing"
)

// Appendix A.2.5 "Curve: NIST P-256" + "With SHA-256, message = 'sample'"
const (
	rfc6979TestX = "C9AFA9D845BA75166B5C215767B1D6934E50C3DB36E89B127B8A622B120F6721"
	rfc6979TestR = "EFD48B2AACB6A8FD1140DD9CD45E81D69D2C877B56AAF991C34D0EA84EAF3716"
	// Note: the RFC publishes s for the un-normalized signature.
	// Our signer applies low-s normalization per BIP-62; if the
	// RFC's s > n/2, we expect (n - s) instead.
	rfc6979TestS        = "F7CB1C942D657C41D436C7A1B6E29F65F3E900DBB9AFF4064DC4AB2F843ACDA8"
	rfc6979TestMessage  = "sample"
)

// TestRFC6979_AppendixA25_Sample verifies the RFC 6979 derivation
// (modulo low-s normalization) matches the published test vector.
func TestRFC6979_AppendixA25_Sample(t *testing.T) {
	d, _ := new(big.Int).SetString(rfc6979TestX, 16)
	curve := elliptic.P256()
	priv := &ecdsa.PrivateKey{
		D: d,
		PublicKey: ecdsa.PublicKey{
			Curve: curve,
		},
	}
	// Derive and populate Q so later tests can re-use priv.
	priv.PublicKey.X, priv.PublicKey.Y = curve.ScalarBaseMult(d.Bytes())

	digest := sha256.Sum256([]byte(rfc6979TestMessage))

	r, s := SignRFC6979(priv, digest[:])

	// r must match the RFC published value (never modified by
	// low-s normalization; only s flips).
	gotR := hex.EncodeToString(padTo32(r.Bytes()))
	wantR := hex.EncodeToString(mustDecodeHexLower(rfc6979TestR))
	if gotR != wantR {
		t.Errorf("r mismatch\n want: %s\n  got: %s", wantR, gotR)
	}

	// For s, compare either the RFC's value (if already low-s) or
	// n-s (if the RFC's value is high-s). Both are valid per the
	// original RFC; our signer always normalizes to low-s.
	rfcSBytes := mustDecodeHexLower(rfc6979TestS)
	rfcS := new(big.Int).SetBytes(rfcSBytes)
	halfN := new(big.Int).Rsh(curve.Params().N, 1)

	expectedS := new(big.Int).Set(rfcS)
	if rfcS.Cmp(halfN) > 0 {
		expectedS.Sub(curve.Params().N, rfcS)
	}
	if s.Cmp(expectedS) != 0 {
		t.Errorf("s mismatch (low-s normalized)\n want: %x\n  got: %x",
			expectedS, s)
	}

	// Verify via stdlib that the signature is valid.
	if !ecdsa.Verify(&priv.PublicKey, digest[:], r, s) {
		t.Error("RFC 6979 signature failed stdlib verification")
	}
}

// TestSignRFC6979_IsDeterministic exercises the core guarantee:
// same (key, message) pair produces bit-identical signatures
// across independent calls.
func TestSignRFC6979_IsDeterministic(t *testing.T) {
	curve := elliptic.P256()
	d, _ := new(big.Int).SetString(rfc6979TestX, 16)
	priv := &ecdsa.PrivateKey{D: d, PublicKey: ecdsa.PublicKey{Curve: curve}}
	priv.PublicKey.X, priv.PublicKey.Y = curve.ScalarBaseMult(d.Bytes())

	digest := sha256.Sum256([]byte("deterministic-test"))

	r1, s1 := SignRFC6979(priv, digest[:])
	r2, s2 := SignRFC6979(priv, digest[:])
	r3, s3 := SignRFC6979(priv, digest[:])

	if r1.Cmp(r2) != 0 || r2.Cmp(r3) != 0 {
		t.Error("r diverged across identical calls (not deterministic)")
	}
	if s1.Cmp(s2) != 0 || s2.Cmp(s3) != 0 {
		t.Error("s diverged across identical calls (not deterministic)")
	}
}

// TestSignRFC6979_LowS confirms s is always <= n/2.
func TestSignRFC6979_LowS(t *testing.T) {
	curve := elliptic.P256()
	halfN := new(big.Int).Rsh(curve.Params().N, 1)

	// Generate a few distinct keys + messages and verify each
	// signature has low s.
	for seed := 1; seed <= 20; seed++ {
		d := new(big.Int).SetInt64(int64(seed * 1000003)) // arbitrary-ish scalar
		d.Mod(d, curve.Params().N)
		if d.Sign() == 0 {
			continue
		}
		priv := &ecdsa.PrivateKey{D: d, PublicKey: ecdsa.PublicKey{Curve: curve}}
		priv.PublicKey.X, priv.PublicKey.Y = curve.ScalarBaseMult(d.Bytes())

		msg := []byte{byte(seed), byte(seed * 2), byte(seed * 3)}
		digest := sha256.Sum256(msg)
		_, s := SignRFC6979(priv, digest[:])
		if s.Cmp(halfN) > 0 {
			t.Errorf("seed=%d produced high-s signature: s=%x halfN=%x",
				seed, s, halfN)
		}
	}
}

// --- helpers ---

func padTo32(b []byte) []byte {
	if len(b) >= 32 {
		return b[len(b)-32:]
	}
	out := make([]byte, 32)
	copy(out[32-len(b):], b)
	return out
}

func mustDecodeHexLower(s string) []byte {
	b, err := hex.DecodeString(s)
	if err != nil {
		panic(err)
	}
	return b
}
