// Package signer defines the interface used by Quidnug integrations
// that want to sign protocol bytes with a key that isn't directly
// accessible to the Go SDK — for example a key living in a PKCS#11
// HSM, a FIDO2 / WebAuthn authenticator, a cloud KMS, or a hardware
// wallet device.
//
// The SDK's client package accepts a *client.Quid for signing, which
// assumes the private key is material available in process memory.
// That's fine for many use cases, but disallowed by most security-
// sensitive deployments. Signer decouples "who holds the key" from
// "what gets signed":
//
//	type Signer interface {
//	    PublicKeyHex() string                // SEC1 hex, for node registration
//	    QuidID() string                      // sha256(pub)[:16]
//	    Sign(data []byte) (string, error)    // hex-encoded DER P-256 sig
//	    Close() error                        // release device handles
//	}
//
// Concrete implementations live in subpackages:
//
//   - pkg/signer/hsm       — PKCS#11 wrapper (Go miekg/pkcs11 based)
//   - pkg/signer/webauthn  — server-side WebAuthn (FIDO2) wrapper
//
// Additional backends (cloud KMS, Ledger, remote signing RPC) are a
// SMOP — just implement Signer and the rest of the SDK is agnostic.
package signer

// Signer abstracts the ability to sign protocol bytes with a quid's
// private key, regardless of where that key physically lives.
//
// Conventions:
//   - Signatures are hex-encoded DER, matching the Go reference and
//     the canonical client.Quid.Sign output.
//   - PublicKeyHex is SEC1 uncompressed (04||X||Y), matching the wire
//     format used throughout the protocol.
//   - QuidID is sha256(pub)[:16] in hex.
//   - Sign accepts the canonical signable bytes (from
//     client.CanonicalBytes) — the implementation must hash with
//     SHA-256 internally and produce a P-256 ECDSA signature.
//   - Close must be safe to call multiple times. It releases any
//     external resources (HSM session, WebAuthn challenge store, etc.).
type Signer interface {
	PublicKeyHex() string
	QuidID() string
	Sign(data []byte) (string, error)
	Close() error
}

// SignableQuid is an adapter that wraps a Signer in a struct matching
// the *client.Quid surface sufficient for client-package convenience
// helpers that don't need key material in-process. Prefer using the
// Signer interface directly where possible.
type SignableQuid struct {
	s Signer
}

// NewSignableQuid returns a quid-shaped wrapper around a Signer.
func NewSignableQuid(s Signer) *SignableQuid {
	return &SignableQuid{s: s}
}

// ID returns the quid ID.
func (q *SignableQuid) ID() string { return q.s.QuidID() }

// PublicKeyHex returns the SEC1 hex public key.
func (q *SignableQuid) PublicKeyHex() string { return q.s.PublicKeyHex() }

// HasPrivateKey always returns true for a SignableQuid — the key is
// just not in-process.
func (q *SignableQuid) HasPrivateKey() bool { return true }

// Sign delegates to the underlying Signer.
func (q *SignableQuid) Sign(data []byte) (string, error) {
	return q.s.Sign(data)
}

// Close releases the underlying signer.
func (q *SignableQuid) Close() error { return q.s.Close() }
