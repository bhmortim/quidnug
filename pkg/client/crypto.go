package client

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"encoding/hex"
	"encoding/json"
	"fmt"
)

// Quid is a cryptographic identity holding an ECDSA P-256 keypair.
//
// Every principal in Quidnug is a Quid — users, orgs, agents,
// devices, titles. The ID is sha256(publicKey)[:16] in hex.
type Quid struct {
	ID            string
	PublicKeyHex  string
	PrivateKeyHex string // empty for read-only quids

	priv *ecdsa.PrivateKey
	pub  *ecdsa.PublicKey
}

// GenerateQuid creates a fresh P-256 keypair and derives the ID.
func GenerateQuid() (*Quid, error) {
	priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, newCryptoError(fmt.Sprintf("keygen: %v", err))
	}
	return quidFromKey(priv)
}

// QuidFromPrivateHex reconstructs a Quid from a PKCS8 DER hex-encoded
// private key (matching the Python SDK's on-disk format).
func QuidFromPrivateHex(privHex string) (*Quid, error) {
	raw, err := hex.DecodeString(privHex)
	if err != nil {
		return nil, newCryptoError("private key hex decode: " + err.Error())
	}
	key, err := x509.ParsePKCS8PrivateKey(raw)
	if err != nil {
		return nil, newCryptoError("PKCS8 parse: " + err.Error())
	}
	ec, ok := key.(*ecdsa.PrivateKey)
	if !ok {
		return nil, newCryptoError("key is not ECDSA")
	}
	return quidFromKey(ec)
}

// QuidFromPublicHex returns a read-only Quid (no signing capability).
// publicHex is the SEC1 uncompressed-point hex encoding.
func QuidFromPublicHex(publicHex string) (*Quid, error) {
	raw, err := hex.DecodeString(publicHex)
	if err != nil {
		return nil, newCryptoError("public key hex decode: " + err.Error())
	}
	x, y := elliptic.Unmarshal(elliptic.P256(), raw) //nolint:staticcheck // SEC1
	if x == nil {
		return nil, newCryptoError("SEC1 unmarshal failed")
	}
	pub := &ecdsa.PublicKey{Curve: elliptic.P256(), X: x, Y: y}
	pubBytes := elliptic.Marshal(elliptic.P256(), pub.X, pub.Y) //nolint:staticcheck
	sum := sha256.Sum256(pubBytes)
	return &Quid{
		ID:           hex.EncodeToString(sum[:8]),
		PublicKeyHex: publicHex,
		pub:          pub,
	}, nil
}

func quidFromKey(priv *ecdsa.PrivateKey) (*Quid, error) {
	pubBytes := elliptic.Marshal(elliptic.P256(), priv.PublicKey.X, priv.PublicKey.Y) //nolint:staticcheck
	sum := sha256.Sum256(pubBytes)
	privDER, err := x509.MarshalPKCS8PrivateKey(priv)
	if err != nil {
		return nil, newCryptoError("PKCS8 marshal: " + err.Error())
	}
	return &Quid{
		ID:            hex.EncodeToString(sum[:8]),
		PublicKeyHex:  hex.EncodeToString(pubBytes),
		PrivateKeyHex: hex.EncodeToString(privDER),
		priv:          priv,
		pub:           &priv.PublicKey,
	}, nil
}

// HasPrivateKey reports whether this quid can sign.
func (q *Quid) HasPrivateKey() bool { return q.priv != nil }

// Sign signs data with the quid's private key. Returns hex-encoded DER.
func (q *Quid) Sign(data []byte) (string, error) {
	if q.priv == nil {
		return "", newCryptoError("quid is read-only")
	}
	digest := sha256.Sum256(data)
	sig, err := ecdsa.SignASN1(rand.Reader, q.priv, digest[:])
	if err != nil {
		return "", newCryptoError("sign: " + err.Error())
	}
	return hex.EncodeToString(sig), nil
}

// Verify checks a hex-encoded DER signature against the quid's public key.
func (q *Quid) Verify(data []byte, sigHex string) bool {
	if q.pub == nil {
		return false
	}
	sig, err := hex.DecodeString(sigHex)
	if err != nil {
		return false
	}
	digest := sha256.Sum256(data)
	return ecdsa.VerifyASN1(q.pub, digest[:], sig)
}

// CanonicalBytes returns the canonical signable encoding of a value.
//
// Matches the Go reference's round-trip-through-generic-object rule,
// ported across SDKs: marshal → unmarshal to map → marshal again.
// encoding/json alphabetizes map keys, so the result is stable
// regardless of struct-field ordering at the source.
//
// excludeFields names top-level fields to omit (typically "signature"
// and "txId"). Fields are matched after the first marshal, on the
// generic object.
func CanonicalBytes(v any, excludeFields ...string) ([]byte, error) {
	first, err := json.Marshal(v)
	if err != nil {
		return nil, fmt.Errorf("canonical bytes first marshal: %w", err)
	}
	var generic map[string]any
	if err := json.Unmarshal(first, &generic); err != nil {
		return nil, fmt.Errorf("canonical bytes unmarshal: %w", err)
	}
	for _, f := range excludeFields {
		delete(generic, f)
	}
	return json.Marshal(generic) // alphabetized output from encoding/json
}
