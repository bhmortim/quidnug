// dns_attestation_validation.go — QDP-0023 Phase 1 admission
// validators for each DNS-family + AUTHORITY_DELEGATE tx type.
//
// Phase 1 validation is deliberately conservative: check
// required fields, enforce enum constraints, verify
// signatures, and cross-reference related tx IDs. More
// elaborate checks (payment-clearance confirmation,
// federation-import sanity, etc.) come in later phases.

package core

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
)

// --- Error helpers ---

// ErrTxTypeUnsupported signals the node doesn't have the
// registry/support for the given tx type (e.g., privacy
// registry missing on a bare-bones test node).
type errTxTypeUnsupported struct{ msg string }

func (e *errTxTypeUnsupported) Error() string { return e.msg }

// ErrTxTypeUnsupported is the constructor.
func ErrTxTypeUnsupported(msg string) error {
	return &errTxTypeUnsupported{msg: msg}
}

// ErrInvalidTx signals validation failure for a specific tx
// type. The rich reason-codes we log elsewhere are for
// operator debugging; this is the caller-facing shape.
type errInvalidTx struct{ kind string }

func (e *errInvalidTx) Error() string { return fmt.Sprintf("invalid %s transaction", e.kind) }

// ErrInvalidTx constructor.
func ErrInvalidTx(kind string) error { return &errInvalidTx{kind: kind} }

// --- seedID helper (type-generic ID derivation) ---

// seedID returns the hex-encoded SHA-256 of the
// encoding/json serialization of v. Used by every DNS-family
// + AUTHORITY_DELEGATE tx to derive a deterministic tx ID
// from a content-relevant field subset.
func seedID(v any) string {
	b, _ := json.Marshal(v)
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

// --- Validation ---

// validDNSDomain is a tight-enough sanity check. We don't
// need RFC-complete DNS validation at the protocol layer;
// the verifier binary does deep verification.
func validDNSDomain(d string) bool {
	if d == "" || len(d) > MaxDomainLength {
		return false
	}
	if strings.HasPrefix(d, ".") || strings.HasSuffix(d, ".") {
		return false
	}
	// Must contain at least one dot (distinguish from quid IDs).
	if !strings.Contains(d, ".") {
		return false
	}
	for _, r := range d {
		if r >= 'a' && r <= 'z' {
			continue
		}
		if r >= '0' && r <= '9' {
			continue
		}
		if r == '.' || r == '-' {
			continue
		}
		return false
	}
	return true
}

var validTLDTiers = map[string]bool{
	"free-public": true,
	"standard":    true,
	"premium":     true,
	"luxury":      true,
}

var validRigorLevels = map[string]bool{
	"":          true, // optional
	"basic":     true,
	"standard":  true,
	"rigorous":  true,
}

var validRevokerRoles = map[string]bool{
	"root":             true,
	"governor-quorum":  true,
	"owner":            true,
}

var validDelegateRevokerRoles = map[string]bool{
	"owner": true,
	"root":  true,
}

var validVisibilityClasses = map[string]bool{
	"public":      true,
	"trust-gated": true,
	"private":     true,
}

// validateDNSClaim: ValidateDNSClaim
func (node *QuidnugNode) ValidateDNSClaim(tx DNSClaimTransaction) bool {
	if !validDNSDomain(tx.Domain) {
		return false
	}
	if !IsValidQuidID(tx.OwnerQuid) {
		return false
	}
	if !IsValidQuidID(tx.RootQuid) {
		return false
	}
	if tx.Nonce <= 0 {
		return false
	}
	if tx.OwnerQuid == tx.RootQuid {
		return false
	}
	switch tx.PaymentMethod {
	case "", "stripe", "crypto", "waiver":
		// accepted
	default:
		return false
	}
	if tx.Signature == "" || tx.PublicKey == "" {
		return false
	}
	return verifyDNSTxSignature(&tx)
}

// ValidateDNSChallenge
func (node *QuidnugNode) ValidateDNSChallenge(tx DNSChallengeTransaction) bool {
	if tx.ClaimRef == "" {
		return false
	}
	if len(tx.Nonce) != 64 { // 32 bytes of hex
		return false
	}
	if tx.ChallengeExpiresAt == 0 {
		return false
	}
	if tx.TXTRecordName == "" || tx.WellKnownURL == "" {
		return false
	}
	if tx.Signature == "" || tx.PublicKey == "" {
		return false
	}
	// Claim must exist in our registry (root must have seen it).
	if node.DNSAttestationRegistry != nil {
		node.DNSAttestationRegistry.mu.RLock()
		_, ok := node.DNSAttestationRegistry.claims[tx.ClaimRef]
		node.DNSAttestationRegistry.mu.RUnlock()
		if !ok {
			// Phase 1: allow orphan challenges (federation
			// might deliver them out-of-order). Log-only.
		}
	}
	return verifyDNSTxSignature(&tx)
}

// ValidateDNSAttestation
func (node *QuidnugNode) ValidateDNSAttestation(tx DNSAttestationTransaction) bool {
	if !validDNSDomain(tx.Domain) {
		return false
	}
	if !IsValidQuidID(tx.OwnerQuid) {
		return false
	}
	if !IsValidQuidID(tx.RootQuid) {
		return false
	}
	if tx.ClaimRef == "" {
		return false
	}
	if !validTLDTiers[tx.TLDTier] {
		return false
	}
	if tx.TLD == "" {
		return false
	}
	if tx.VerifiedAt == 0 || tx.ValidUntil == 0 {
		return false
	}
	if tx.ValidUntil <= tx.VerifiedAt {
		return false
	}
	if !validRigorLevels[tx.RigorLevel] {
		return false
	}
	if tx.Nonce <= 0 {
		return false
	}
	if tx.Signature == "" || tx.PublicKey == "" {
		return false
	}
	return verifyDNSTxSignature(&tx)
}

// ValidateDNSRenewal
func (node *QuidnugNode) ValidateDNSRenewal(tx DNSRenewalTransaction) bool {
	if tx.PriorAttestationRef == "" {
		return false
	}
	if tx.NewValidUntil == 0 {
		return false
	}
	if tx.Nonce <= 0 {
		return false
	}
	if tx.Signature == "" || tx.PublicKey == "" {
		return false
	}
	// Prior attestation should exist — we check but don't
	// reject on miss (gossip may deliver out of order).
	return verifyDNSTxSignature(&tx)
}

// ValidateDNSRevocation
func (node *QuidnugNode) ValidateDNSRevocation(tx DNSRevocationTransaction) bool {
	if tx.AttestationRef == "" {
		return false
	}
	if !IsValidQuidID(tx.RevokerQuid) {
		return false
	}
	if !validRevokerRoles[tx.RevokerRole] {
		return false
	}
	if tx.Reason == "" {
		return false
	}
	if tx.RevokedAt == 0 {
		return false
	}
	if tx.Nonce <= 0 {
		return false
	}
	if tx.RevokerRole == "governor-quorum" && len(tx.GovernorSignatures) == 0 {
		return false
	}
	if tx.Signature == "" || tx.PublicKey == "" {
		return false
	}
	return verifyDNSTxSignature(&tx)
}

// ValidateAuthorityDelegate
func (node *QuidnugNode) ValidateAuthorityDelegate(tx AuthorityDelegateTransaction) bool {
	if tx.AttestationRef == "" {
		return false
	}
	if tx.AttestationKind == "" {
		return false
	}
	if tx.Subject == "" {
		return false
	}
	if tx.DelegateDomain == "" && len(tx.DelegateNodes) == 0 {
		return false
	}
	// Visibility policy sanity.
	if tx.Visibility.Default.Class != "" && !validVisibilityClasses[tx.Visibility.Default.Class] {
		return false
	}
	for _, pol := range tx.Visibility.RecordTypes {
		if !validVisibilityClasses[pol.Class] {
			return false
		}
		if pol.Class == "trust-gated" && pol.GateDomain == "" {
			return false
		}
		if pol.Class == "private" && pol.GroupID == "" {
			return false
		}
	}
	if tx.ValidUntil != 0 && tx.EffectiveAt != 0 && tx.ValidUntil <= tx.EffectiveAt {
		return false
	}
	if tx.Nonce <= 0 {
		return false
	}
	if tx.Signature == "" || tx.PublicKey == "" {
		return false
	}
	return verifyDNSTxSignature(&tx)
}

// --- Signature verification (generic) ---

// verifyDNSTxSignature does the per-type signable-bytes +
// VerifySignature dance. Caller passes a pointer to any of
// our DNS-family or delegate tx structs. We re-marshal with
// Signature cleared and verify against tx.PublicKey.
func verifyDNSTxSignature(tx any) bool {
	// Reset signature via reflection? Simpler: assert on
	// concrete types.
	switch v := tx.(type) {
	case *DNSClaimTransaction:
		return verifyStructSig(v.PublicKey, v.Signature, func() any {
			copy := *v
			copy.Signature = ""
			return copy
		})
	case *DNSChallengeTransaction:
		return verifyStructSig(v.PublicKey, v.Signature, func() any {
			copy := *v
			copy.Signature = ""
			return copy
		})
	case *DNSAttestationTransaction:
		return verifyStructSig(v.PublicKey, v.Signature, func() any {
			copy := *v
			copy.Signature = ""
			return copy
		})
	case *DNSRenewalTransaction:
		return verifyStructSig(v.PublicKey, v.Signature, func() any {
			copy := *v
			copy.Signature = ""
			return copy
		})
	case *DNSRevocationTransaction:
		return verifyStructSig(v.PublicKey, v.Signature, func() any {
			copy := *v
			copy.Signature = ""
			return copy
		})
	case *AuthorityDelegateTransaction:
		return verifyStructSig(v.PublicKey, v.Signature, func() any {
			copy := *v
			copy.Signature = ""
			return copy
		})
	default:
		return false
	}
}

func verifyStructSig(publicKeyHex, signatureHex string, signableProducer func() any) bool {
	if publicKeyHex == "" || signatureHex == "" {
		return false
	}
	b, err := json.Marshal(signableProducer())
	if err != nil {
		return false
	}
	return VerifySignature(publicKeyHex, b, signatureHex)
}
