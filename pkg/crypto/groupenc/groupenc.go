// Package groupenc implements the group-keyed encryption
// primitives for QDP-0024 Phase 1: private communications +
// encrypted records.
//
// Phase 1 ships the simpler "direct-wrap" key-distribution
// scheme (2-16 members, no tree). Each member's copy of the
// epoch secret is encrypted individually to their X25519
// public key via ECDH + HKDF-derived KEK + AES-GCM-256.
//
// This is the scheme QDP-0024 §16 Open Question 1 flagged as
// acceptable for small groups: "For very small groups (2-10
// members), a simple encrypt-to-each-member approach is
// simpler with negligible overhead." For groups of 100+, a
// full TreeKEM per RFC 9420 §7-8 would be required; that
// lands in Phase 2.
//
// The API is:
//
//   - KeyPair: X25519 keypair for group participants.
//   - GenerateKeyPair: fresh keypair.
//   - WrapEpochKey: encrypts a 32-byte epoch secret to a
//     member's public key, returning a ciphertext the member
//     can decrypt with their private key.
//   - UnwrapEpochKey: inverse.
//   - EncryptRecord: AES-GCM-256 encrypt with the epoch
//     secret + a fresh 12-byte nonce.
//   - DecryptRecord: inverse.
//
// Once a member has the epoch secret (via unwrap), they can
// decrypt any record encrypted under that epoch. Epoch
// rotation (forward-secrecy on membership change) produces a
// new secret + re-wraps to the updated member set, excluding
// removed members.
package groupenc

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/ecdh"
	"crypto/hkdf"
	"crypto/rand"
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"io"
)

// EpochSecretLength is the byte length of an epoch secret
// (AES-256 key size).
const EpochSecretLength = 32

// Nonce sizes for AES-GCM.
const (
	nonceSize = 12
	tagSize   = 16
)

// ---------------------------------------------------------------
// Key management
// ---------------------------------------------------------------

// KeyPair holds both halves of an X25519 keypair plus its
// canonical byte forms.
type KeyPair struct {
	Private *ecdh.PrivateKey
	Public  *ecdh.PublicKey
}

// GenerateKeyPair returns a fresh X25519 keypair.
func GenerateKeyPair(reader io.Reader) (*KeyPair, error) {
	if reader == nil {
		reader = rand.Reader
	}
	priv, err := ecdh.X25519().GenerateKey(reader)
	if err != nil {
		return nil, err
	}
	return &KeyPair{Private: priv, Public: priv.PublicKey()}, nil
}

// PublicKeyBytes returns the 32-byte X25519 public key.
func (kp *KeyPair) PublicKeyBytes() []byte {
	return kp.Public.Bytes()
}

// PrivateKeyBytes returns the 32-byte X25519 private key.
func (kp *KeyPair) PrivateKeyBytes() []byte {
	return kp.Private.Bytes()
}

// KeyPairFromPrivate reconstructs from a 32-byte X25519
// private scalar.
func KeyPairFromPrivate(privBytes []byte) (*KeyPair, error) {
	priv, err := ecdh.X25519().NewPrivateKey(privBytes)
	if err != nil {
		return nil, err
	}
	return &KeyPair{Private: priv, Public: priv.PublicKey()}, nil
}

// PublicKeyFromBytes parses a 32-byte X25519 public key.
func PublicKeyFromBytes(pubBytes []byte) (*ecdh.PublicKey, error) {
	return ecdh.X25519().NewPublicKey(pubBytes)
}

// ---------------------------------------------------------------
// Epoch secret + wrapping
// ---------------------------------------------------------------

// NewEpochSecret returns a fresh 32-byte secret suitable for
// use as an AES-256 group key.
func NewEpochSecret(reader io.Reader) ([]byte, error) {
	if reader == nil {
		reader = rand.Reader
	}
	secret := make([]byte, EpochSecretLength)
	if _, err := io.ReadFull(reader, secret); err != nil {
		return nil, err
	}
	return secret, nil
}

// WrappedSecret is the encrypted form of an epoch secret for
// a single member. Carries the ephemeral ECDH public key the
// sender used + the GCM nonce + the ciphertext.
//
// Serialization on the wire: ephemeral_pub (32 bytes) || nonce
// (12 bytes) || ciphertext (48 bytes: 32-byte secret + 16-byte
// GCM tag). Total fixed size: 92 bytes.
type WrappedSecret struct {
	EphemeralPub []byte // 32 bytes
	Nonce        []byte // 12 bytes
	Ciphertext   []byte // 48 bytes (32 plaintext + 16 tag)
}

// Marshal returns the concatenated wire form.
func (w *WrappedSecret) Marshal() []byte {
	out := make([]byte, 0, 32+nonceSize+len(w.Ciphertext))
	out = append(out, w.EphemeralPub...)
	out = append(out, w.Nonce...)
	out = append(out, w.Ciphertext...)
	return out
}

// UnmarshalWrappedSecret parses the wire form.
func UnmarshalWrappedSecret(b []byte) (*WrappedSecret, error) {
	if len(b) != 32+nonceSize+(EpochSecretLength+tagSize) {
		return nil, errors.New("groupenc: wrapped secret wrong length")
	}
	return &WrappedSecret{
		EphemeralPub: append([]byte(nil), b[:32]...),
		Nonce:        append([]byte(nil), b[32:32+nonceSize]...),
		Ciphertext:   append([]byte(nil), b[32+nonceSize:]...),
	}, nil
}

// WrapEpochKey encrypts `secret` (must be 32 bytes) to
// `memberPub` using the ECDH + HKDF + AES-GCM-256 scheme.
//
// The sender generates a fresh ephemeral X25519 keypair, does
// ECDH with the member, HKDF-derives a 32-byte KEK, and
// AES-GCM-encrypts the secret with a fresh nonce. The
// ephemeral public key + nonce + ciphertext all travel in the
// returned WrappedSecret.
func WrapEpochKey(reader io.Reader, memberPub *ecdh.PublicKey, secret []byte) (*WrappedSecret, error) {
	if len(secret) != EpochSecretLength {
		return nil, errors.New("groupenc: secret must be 32 bytes")
	}
	if reader == nil {
		reader = rand.Reader
	}
	ephemeral, err := ecdh.X25519().GenerateKey(reader)
	if err != nil {
		return nil, err
	}
	shared, err := ephemeral.ECDH(memberPub)
	if err != nil {
		return nil, err
	}
	kek, err := deriveKEK(shared, ephemeral.PublicKey().Bytes(), memberPub.Bytes())
	if err != nil {
		return nil, err
	}
	nonce := make([]byte, nonceSize)
	if _, err := io.ReadFull(reader, nonce); err != nil {
		return nil, err
	}
	block, err := aes.NewCipher(kek)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	ct := gcm.Seal(nil, nonce, secret, nil)
	return &WrappedSecret{
		EphemeralPub: ephemeral.PublicKey().Bytes(),
		Nonce:        nonce,
		Ciphertext:   ct,
	}, nil
}

// UnwrapEpochKey decrypts a WrappedSecret using the
// recipient's private key. Returns the 32-byte epoch secret
// or an error.
func UnwrapEpochKey(recipient *ecdh.PrivateKey, w *WrappedSecret) ([]byte, error) {
	ephemeralPub, err := ecdh.X25519().NewPublicKey(w.EphemeralPub)
	if err != nil {
		return nil, err
	}
	shared, err := recipient.ECDH(ephemeralPub)
	if err != nil {
		return nil, err
	}
	kek, err := deriveKEK(shared, w.EphemeralPub, recipient.PublicKey().Bytes())
	if err != nil {
		return nil, err
	}
	block, err := aes.NewCipher(kek)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	secret, err := gcm.Open(nil, w.Nonce, w.Ciphertext, nil)
	if err != nil {
		return nil, err
	}
	if len(secret) != EpochSecretLength {
		return nil, errors.New("groupenc: unwrapped secret wrong length")
	}
	return secret, nil
}

func deriveKEK(shared, ephemeralPub, recipientPub []byte) ([]byte, error) {
	// HKDF-SHA256 with salt = ephemeralPub || recipientPub
	// (binds the KEK to the exact ephemeral pair), info =
	// "quidnug-groupenc-kek-v1".
	salt := make([]byte, 0, 64)
	salt = append(salt, ephemeralPub...)
	salt = append(salt, recipientPub...)
	return hkdf.Key(sha256.New, shared, salt, "quidnug-groupenc-kek-v1", 32)
}

// ---------------------------------------------------------------
// Record encryption
// ---------------------------------------------------------------

// EncryptRecord encrypts `plaintext` with the group's
// `epochSecret` using AES-GCM-256 and a fresh random 12-byte
// nonce. The returned ciphertext is the GCM output (plaintext
// + 16-byte auth tag).
//
// Callers embed the nonce + ciphertext in their
// ENCRYPTED_RECORD event payload; decryption looks up the
// epoch's secret via the group's key-package chain.
//
// aad is optional associated data bound into the GCM tag;
// pass nil if none.
func EncryptRecord(reader io.Reader, epochSecret, plaintext, aad []byte) (nonce, ciphertext []byte, err error) {
	if len(epochSecret) != EpochSecretLength {
		return nil, nil, errors.New("groupenc: epoch secret must be 32 bytes")
	}
	if reader == nil {
		reader = rand.Reader
	}
	nonce = make([]byte, nonceSize)
	if _, err := io.ReadFull(reader, nonce); err != nil {
		return nil, nil, err
	}
	block, err := aes.NewCipher(epochSecret)
	if err != nil {
		return nil, nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, nil, err
	}
	ct := gcm.Seal(nil, nonce, plaintext, aad)
	return nonce, ct, nil
}

// DecryptRecord is the inverse of EncryptRecord. Returns the
// plaintext or an error (authentication failure counts).
func DecryptRecord(epochSecret, nonce, ciphertext, aad []byte) ([]byte, error) {
	if len(epochSecret) != EpochSecretLength {
		return nil, errors.New("groupenc: epoch secret must be 32 bytes")
	}
	if len(nonce) != nonceSize {
		return nil, errors.New("groupenc: nonce must be 12 bytes")
	}
	block, err := aes.NewCipher(epochSecret)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	return gcm.Open(nil, nonce, ciphertext, aad)
}

// ---------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------

// EpochSecretFingerprint returns a hex-encoded 16-char
// identifier for an epoch secret: first 8 bytes of SHA-256.
// Used in logs / error messages; MUST NOT be used to reveal
// the secret.
func EpochSecretFingerprint(secret []byte) string {
	h := sha256.Sum256(secret)
	return hexEnc(h[:8])
}

func hexEnc(b []byte) string {
	const hexdigits = "0123456789abcdef"
	out := make([]byte, len(b)*2)
	for i, x := range b {
		out[i*2] = hexdigits[x>>4]
		out[i*2+1] = hexdigits[x&0x0f]
	}
	return string(out)
}

// --- unused-import guard ---
var _ = binary.BigEndian
