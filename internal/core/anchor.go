package core

import (
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"time"
)

// AnchorKind is the discriminator for the three anchor types defined in
// QDP-0001 §6.5.
type AnchorKind int

const (
	// AnchorRotation advances a signer to a new key epoch. The
	// NewPublicKey and ToEpoch fields are mandatory. Must be signed by
	// the signer's current (FromEpoch) private key.
	AnchorRotation AnchorKind = iota + 1

	// AnchorInvalidation freezes the signer's current epoch: no
	// transaction in FromEpoch with nonce > MaxAcceptedOldNonce is valid
	// thereafter. ToEpoch == FromEpoch. Use as the emergency button.
	AnchorInvalidation

	// AnchorEpochCap caps the signer's current epoch at
	// MaxAcceptedOldNonce without freezing or rotating. A precautionary
	// watermark — limits the blast radius of a future compromise.
	AnchorEpochCap
)

// String makes log output readable.
func (k AnchorKind) String() string {
	switch k {
	case AnchorRotation:
		return "rotation"
	case AnchorInvalidation:
		return "invalidation"
	case AnchorEpochCap:
		return "epoch_cap"
	}
	if s, ok := k.guardianString(); ok {
		return s
	}
	return fmt.Sprintf("unknown(%d)", int(k))
}

// NonceAnchor is a standalone signed message that updates the nonce-
// ledger's epoch state for a single signer. Anchors are embedded in a
// block's transactions list (wrapped as AnchorTransaction so they share
// the common transaction envelope) and processed at Trusted inclusion.
//
// This structure mirrors the wire format documented in QDP-0001 §9.
type NonceAnchor struct {
	Kind                AnchorKind `json:"kind"`
	SignerQuid          string     `json:"signerQuid"`
	FromEpoch           uint32     `json:"fromEpoch"`
	ToEpoch             uint32     `json:"toEpoch"`
	NewPublicKey        string     `json:"newPublicKey,omitempty"` // hex SPKI; required for Rotation only
	MinNextNonce        int64      `json:"minNextNonce"`
	MaxAcceptedOldNonce int64      `json:"maxAcceptedOldNonce"`
	ValidFrom           int64      `json:"validFrom"`
	AnchorNonce         int64      `json:"anchorNonce"`
	Signature           string     `json:"signature"`
}

// AnchorMaxFutureSkew is how far into the future an anchor's ValidFrom
// may be. Guards against an attacker pre-signing an anchor and then
// delivering it at an inconvenient moment.
const AnchorMaxFutureSkew = 5 * time.Minute

// AnchorMaxAge bounds how old an anchor's ValidFrom can be before the
// network refuses to apply it. Without this, an attacker could replay
// a stale (but otherwise valid) anchor years after the fact. Per
// QDP-0002 §12.3 "validFrom within 30 days" recommendation.
const AnchorMaxAge = 30 * 24 * time.Hour

// Errors returned by ValidateAnchor. Use errors.Is.
var (
	ErrAnchorUnknownKind         = errors.New("anchor: unknown kind")
	ErrAnchorMissingSigner       = errors.New("anchor: missing signerQuid")
	ErrAnchorBadEpochProgression = errors.New("anchor: illegal epoch progression for kind")
	ErrAnchorMissingNewKey       = errors.New("anchor: rotation anchor requires NewPublicKey")
	ErrAnchorSpuriousNewKey      = errors.New("anchor: non-rotation anchor must not carry NewPublicKey")
	ErrAnchorBadNewKey           = errors.New("anchor: NewPublicKey is not a valid P-256 SPKI")
	ErrAnchorStaleValidFrom      = errors.New("anchor: validFrom is too old or too far in the future")
	ErrAnchorBadMaxOld           = errors.New("anchor: MaxAcceptedOldNonce is nonsensical")
	ErrAnchorBadMinNext          = errors.New("anchor: MinNextNonce is nonsensical")
	ErrAnchorNonceNotMonotonic   = errors.New("anchor: AnchorNonce must strictly increase per signer")
	ErrAnchorSignerKeyUnknown    = errors.New("anchor: no public key recorded for signer's FromEpoch")
	ErrAnchorBadSignature        = errors.New("anchor: signature verification failed")
)

// GetAnchorSignableData returns canonical bytes for signing or verifying
// an anchor. Equivalent to marshaling the struct with Signature cleared.
func GetAnchorSignableData(a NonceAnchor) ([]byte, error) {
	a.Signature = ""
	return json.Marshal(a)
}

// ValidateAnchor performs structural + cryptographic + monotonicity
// checks on an anchor against the current ledger state. It is
// read-only: callers pair it with NonceLedger.ApplyAnchor once the
// anchor has been accepted in a Trusted block.
//
// The `now` parameter is supplied by the caller rather than read from
// time.Now() inside the function; this is the test-friendly shape.
func ValidateAnchor(l *NonceLedger, a NonceAnchor, now time.Time) error {
	// 1. Kind and shape.
	switch a.Kind {
	case AnchorRotation:
		if a.ToEpoch <= a.FromEpoch {
			return ErrAnchorBadEpochProgression
		}
		if a.NewPublicKey == "" {
			return ErrAnchorMissingNewKey
		}
		if _, err := decodeP256PublicKey(a.NewPublicKey); err != nil {
			return fmt.Errorf("%w: %v", ErrAnchorBadNewKey, err)
		}
	case AnchorInvalidation, AnchorEpochCap:
		if a.ToEpoch != a.FromEpoch {
			return ErrAnchorBadEpochProgression
		}
		if a.NewPublicKey != "" {
			return ErrAnchorSpuriousNewKey
		}
	default:
		return ErrAnchorUnknownKind
	}

	// 2. Identity / fields.
	if a.SignerQuid == "" {
		return ErrAnchorMissingSigner
	}
	if a.MaxAcceptedOldNonce < 0 {
		return ErrAnchorBadMaxOld
	}
	if a.MinNextNonce < 1 {
		return ErrAnchorBadMinNext
	}
	if a.Kind == AnchorRotation && a.MinNextNonce <= 0 {
		return ErrAnchorBadMinNext
	}

	// 3. ValidFrom window.
	if a.ValidFrom <= 0 {
		return ErrAnchorStaleValidFrom
	}
	validFrom := time.Unix(a.ValidFrom, 0)
	if now.Sub(validFrom) > AnchorMaxAge {
		return ErrAnchorStaleValidFrom
	}
	if validFrom.Sub(now) > AnchorMaxFutureSkew {
		return ErrAnchorStaleValidFrom
	}

	// 4. Strict anchor-nonce monotonicity.
	if l != nil && a.AnchorNonce <= l.LastAnchorNonce(a.SignerQuid) {
		return ErrAnchorNonceNotMonotonic
	}

	// 5. Signature: must verify against the signer's key for FromEpoch.
	if l == nil {
		return ErrAnchorSignerKeyUnknown
	}
	signerKey, ok := l.GetSignerKey(a.SignerQuid, a.FromEpoch)
	if !ok || signerKey == "" {
		return ErrAnchorSignerKeyUnknown
	}
	signable, err := GetAnchorSignableData(a)
	if err != nil {
		return fmt.Errorf("anchor: canonicalization: %w", err)
	}
	sig, err := hex.DecodeString(a.Signature)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrAnchorBadSignature, err)
	}
	if !VerifySignature(signerKey, signable, hex.EncodeToString(sig)) {
		return ErrAnchorBadSignature
	}

	return nil
}

// decodeP256PublicKey sanity-checks a hex-encoded uncompressed P-256
// public key. It does NOT keep the parsed key; it just confirms that
// VerifySignature won't explode when handed this value later.
func decodeP256PublicKey(hexKey string) ([]byte, error) {
	raw, err := hex.DecodeString(hexKey)
	if err != nil {
		return nil, err
	}
	if len(raw) != 65 || raw[0] != 0x04 {
		return nil, fmt.Errorf("public key must be 65 bytes starting with 0x04 (uncompressed P-256), got %d bytes", len(raw))
	}
	return raw, nil
}
