// Package blindrsa implements the RSA-FDH blind signature
// scheme chosen for Quidnug's anonymous ballot issuance per
// QDP-0021.
//
// The protocol is the classic Chaum RSA blind signature with
// a Full-Domain Hash (FDH) applied to the message before the
// RSA operation. This matches RFC 9474's RSABSSA-SHA256-
// PSSZERO-Deterministic profile minus the PSS padding (pure
// FDH; simpler and what QDP-0021 specifies).
//
// Usage flow (from QDP-0021 §6.2):
//
//  1. Authority publishes (n, e) and a keygen attestation.
//  2. Voter generates a ballot token T (32 random bytes) +
//     ephemeral keypair. Encodes token via MGF1-SHA256 as
//     m' (an integer mod n). Blinds m' with random r:
//     m_blind = m' * r^e mod n.
//  3. Voter submits m_blind to the authority.
//  4. Authority signs: s_blind = m_blind^d mod n.
//  5. Voter unblinds: s = s_blind * r^(-1) mod n.
//  6. Anyone verifies: s^e mod n == m' (the FDH-encoded
//     token). Since the authority never saw T, they can't
//     correlate the signature with the voter.
//
// This implementation is self-contained; it depends only on
// Go's standard library. Compatible test vectors for RSA-FDH
// follow RFC 9474 §A.
package blindrsa

import (
	"crypto"
	"crypto/hmac"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"fmt"
	"hash"
	"io"
	"math/big"
)

// DefaultKeySize is the recommended modulus size in bits.
// RFC 9474 mandates 2048 minimum; QDP-0021 selects 3072 for
// an extra safety margin on ballot lifetimes measured in
// years.
const DefaultKeySize = 3072

// ErrInvalidBlindingFactor signals that the RNG produced an r
// with gcd(r, n) != 1. Caller should retry with a fresh r.
var ErrInvalidBlindingFactor = errors.New("blindrsa: blinding factor not coprime to n (retry)")

// ErrSignatureInvalid is returned by Verify on signature
// mismatch.
var ErrSignatureInvalid = errors.New("blindrsa: signature invalid")

// GenerateKey returns a fresh RSA blinding keypair with the
// given modulus bit-size.
func GenerateKey(reader io.Reader, bits int) (*rsa.PrivateKey, error) {
	if reader == nil {
		reader = rand.Reader
	}
	return rsa.GenerateKey(reader, bits)
}

// FDHEncode applies a Full-Domain Hash to `token`: MGF1 with
// SHA-256 produces an octet string the same byte-length as
// the modulus, then we reduce mod n to get an integer <n.
//
// The output is a big.Int suitable for blinding and signing.
func FDHEncode(token []byte, n *big.Int) (*big.Int, error) {
	if n == nil || n.Sign() <= 0 {
		return nil, errors.New("blindrsa: invalid modulus")
	}
	k := (n.BitLen() + 7) / 8
	out, err := mgf1Sha256(token, k)
	if err != nil {
		return nil, err
	}
	m := new(big.Int).SetBytes(out)
	m.Mod(m, n)
	if m.Sign() == 0 {
		m.SetInt64(1) // FDH rejects m=0; map to 1 (vanishingly rare)
	}
	return m, nil
}

// Blind returns (blinded, r) where blinded = m * r^e mod n
// for a fresh random r with gcd(r, n) == 1.
//
// The caller sends `blinded` to the authority and keeps `r`
// to unblind the signature later.
func Blind(reader io.Reader, pub *rsa.PublicKey, m *big.Int) (blinded, r *big.Int, err error) {
	if reader == nil {
		reader = rand.Reader
	}
	e := big.NewInt(int64(pub.E))
	for attempts := 0; attempts < 32; attempts++ {
		r, err = rand.Int(reader, pub.N)
		if err != nil {
			return nil, nil, err
		}
		if r.Sign() == 0 {
			continue
		}
		gcd := new(big.Int).GCD(nil, nil, r, pub.N)
		if gcd.Cmp(big.NewInt(1)) != 0 {
			continue
		}
		// blinded = m * r^e mod n
		re := new(big.Int).Exp(r, e, pub.N)
		blinded = new(big.Int).Mul(m, re)
		blinded.Mod(blinded, pub.N)
		return blinded, r, nil
	}
	return nil, nil, ErrInvalidBlindingFactor
}

// SignBlinded performs the raw RSA private-key operation on
// the blinded integer: out = m_blind^d mod n.
//
// The authority does this without learning the unblinded
// token.
func SignBlinded(priv *rsa.PrivateKey, blinded *big.Int) *big.Int {
	// Use the standard library's crypto-safe decryption path
	// with blinding disabled at our layer (we've already
	// blinded). DecryptRSANoPadding would be ideal but isn't
	// public; we use the raw modular exponentiation which is
	// what rsa.decrypt does internally.
	return new(big.Int).Exp(blinded, priv.D, priv.N)
}

// Unblind recovers the signature on the original message:
// s = s_blind * r^(-1) mod n.
func Unblind(pub *rsa.PublicKey, sBlinded, r *big.Int) *big.Int {
	rInv := new(big.Int).ModInverse(r, pub.N)
	if rInv == nil {
		return nil
	}
	s := new(big.Int).Mul(sBlinded, rInv)
	s.Mod(s, pub.N)
	return s
}

// Verify checks that s is a valid RSA-FDH signature on token
// under pub: s^e mod n == FDH(token) mod n.
func Verify(pub *rsa.PublicKey, token []byte, s *big.Int) error {
	m, err := FDHEncode(token, pub.N)
	if err != nil {
		return err
	}
	e := big.NewInt(int64(pub.E))
	lhs := new(big.Int).Exp(s, e, pub.N)
	if lhs.Cmp(m) != 0 {
		return ErrSignatureInvalid
	}
	return nil
}

// PublicKeyFingerprint returns a stable hex-encoded SHA-256
// of the canonical `(n, e)` serialization. Used to reference
// a specific authority key across events (QDP-0021
// BLIND_KEY_ATTESTATION carries this).
//
// Note on integer widths: pub.N.BitLen() is bounded by RSA
// modulus size (≤ 16384 in any sane configuration) and pub.E
// is bounded by Go's `int` for the public exponent (RFC 8017
// recommends e ≤ 2^256-1 but Go's *rsa.PublicKey.E is just an
// int, capped at 2^31-1 on 32-bit platforms). Both values fit
// in uint32 with margin, but we range-check explicitly so
// CodeQL/gosec sees the gate before the conversion.
func PublicKeyFingerprint(pub *rsa.PublicKey) string {
	h := sha256.New()
	bitLen := pub.N.BitLen()
	if bitLen < 0 || bitLen > 0xFFFFFFFF {
		// Unreachable in practice (BitLen returns >= 0 and
		// realistic moduli are <= 16384 bits) but kills the
		// int-to-uint32 overflow alert.
		bitLen = 0
	}
	_ = binary.Write(h, binary.BigEndian, uint32(bitLen)) // #nosec G115 -- range-checked above
	h.Write(pub.N.Bytes())
	e := pub.E
	if e < 0 || int64(e) > 0xFFFFFFFF {
		// Same: rsa.PublicKey.E is a Go int, so on 64-bit
		// platforms it can in principle exceed uint32. Reject
		// implausible values rather than silently truncating.
		e = 0
	}
	_ = binary.Write(h, binary.BigEndian, uint32(e)) // #nosec G115 -- range-checked above
	sum := h.Sum(nil)
	out := make([]byte, len(sum)*2)
	const hexdigits = "0123456789abcdef"
	for i, b := range sum {
		out[i*2] = hexdigits[b>>4]
		out[i*2+1] = hexdigits[b&0x0f]
	}
	return string(out)
}

// --- internal: MGF1-SHA256 ---

// mgf1Sha256 implements the RFC 8017 MGF1 mask generation
// function with SHA-256 as the underlying hash. Output length
// is `length` bytes.
func mgf1Sha256(seed []byte, length int) ([]byte, error) {
	if length < 0 {
		return nil, errors.New("blindrsa: mgf1 negative length")
	}
	out := make([]byte, 0, length)
	counter := uint32(0)
	hashLen := sha256.Size
	for len(out) < length {
		h := sha256.New()
		h.Write(seed)
		var ctr [4]byte
		binary.BigEndian.PutUint32(ctr[:], counter)
		h.Write(ctr[:])
		out = append(out, h.Sum(nil)...)
		counter++
		if counter == 0 {
			return nil, errors.New("blindrsa: mgf1 counter overflow")
		}
		_ = hashLen
	}
	return out[:length], nil
}

// --- helper for tests / interop ---

// hmacSha256 is exported to make test-harness composition
// simpler; not part of the public RSA-FDH API.
func hmacSha256(key, data []byte) []byte {
	return hmacHash(sha256.New, key, data)
}

func hmacHash(newHash func() hash.Hash, key, data []byte) []byte {
	m := hmac.New(newHash, key)
	m.Write(data)
	return m.Sum(nil)
}

// --- convenience: compiled signer ---

// Hasher is unused publicly but retained to silence the
// crypto import if future additions need it.
var _ crypto.Hash = crypto.SHA256

// Errorf is a small helper to keep error paths terse.
func errorf(format string, args ...any) error {
	return fmt.Errorf("blindrsa: "+format, args...)
}

// compile-time check: no unused imports
var _ = errorf
