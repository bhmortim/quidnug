// Package hsm is a PKCS#11 Signer implementation.
//
// It wraps any PKCS#11-compliant HSM or software token (SoftHSM,
// YubiHSM, Thales, AWS CloudHSM, Azure Key Vault HSM, Google Cloud HSM,
// etc.) and exposes it behind the signer.Signer interface used by the
// rest of the Quidnug Go SDK.
//
// Usage:
//
//	s, err := hsm.Open(hsm.Config{
//	    ModulePath: "/usr/lib/softhsm/libsofthsm2.so",
//	    TokenLabel: "quidnug-prod",
//	    KeyLabel:   "node-01",
//	    PIN:        os.Getenv("HSM_PIN"),
//	})
//	if err != nil { log.Fatal(err) }
//	defer s.Close()
//
//	sig, err := s.Sign(canonicalBytes)
//
// The package depends on the MIT-licensed github.com/miekg/pkcs11. To
// keep the main module dep-light, this package is guarded by a build
// tag (-tags=pkcs11). Without the tag, Open returns an explanatory
// error so the rest of the SDK still compiles on systems where pkcs11
// is unavailable.
//
// PKCS#11 mechanism: CKM_ECDSA on a P-256 key (CKK_EC,
// CKA_EC_PARAMS=prime256v1). The SDK pre-hashes the signable bytes
// with SHA-256 and submits the digest, matching the ECDSA P-256 +
// SHA-256 primitive used across Quidnug.
package hsm

// Config describes which HSM / token / key to use.
type Config struct {
	// ModulePath is the filesystem path to the PKCS#11 shared library
	// (e.g. /usr/lib/softhsm/libsofthsm2.so,
	// /usr/local/lib/libykcs11.dylib).
	ModulePath string

	// TokenLabel matches a slot by its token label. Overridden by
	// Slot if non-zero.
	TokenLabel string
	// Slot, if > 0, selects the slot by ID rather than by label.
	Slot uint

	// KeyLabel is the CKA_LABEL of the P-256 private key. Overridden
	// by KeyID (CKA_ID) when provided.
	KeyLabel string
	// KeyID is the CKA_ID (binary) of the key. Pass as lowercase hex.
	KeyID string

	// PIN authenticates the user session. For HSMs with
	// attestation-protected auth, leave empty and use the
	// SessionHandle from your HSM driver.
	PIN string
}

// Validate returns an error if Config is missing required fields.
func (c Config) Validate() error {
	if c.ModulePath == "" {
		return errf("ModulePath is required")
	}
	if c.TokenLabel == "" && c.Slot == 0 {
		return errf("TokenLabel or Slot is required")
	}
	if c.KeyLabel == "" && c.KeyID == "" {
		return errf("KeyLabel or KeyID is required")
	}
	return nil
}

type stringError struct{ s string }

func (e *stringError) Error() string { return e.s }

func errf(s string) error { return &stringError{s: s} }
