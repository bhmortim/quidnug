// Package core — guardian_resignation.go
//
// Guardian-consent revocation (QDP-0006 / H6). A guardian can
// withdraw their consent to participate in a named subject's
// recovery quorum by submitting an AnchorGuardianResign
// transaction. Takes effect at the resignation's EffectiveAt
// timestamp; prospective only, in-flight recoveries proceed on
// the set-as-it-was at Init time.
//
// Invariants enforced:
//
//   - Only the guardian themselves can resign (signed at their
//     current epoch).
//
//   - Resignation is bound to a specific set version via
//     GuardianSetHash. A resignation against a now-superseded
//     set is rejected with ErrResignationSetHashMismatch so the
//     guardian is forced to re-sign against the current shape.
//
//   - Per-(guardian, subject) monotonic ResignationNonce prevents
//     replay.
//
//   - EffectiveAt must be in [now - 5min, now + 365d]. A tiny
//     past tolerance handles clock skew at submission time.
//
//   - The guardian set itself is not mutated. Resignations are
//     an overlay consulted by EffectiveGuardianSet at read time.
//     This preserves the "set as installed" audit trail.
package core

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"time"
)

// AnchorGuardianResign is the eighth anchor kind — the
// guardian-resignation addition in QDP-0006. Numerically
// append-only so existing anchors retain their meaning.
const AnchorGuardianResign AnchorKind = AnchorGuardianSetUpdate + 1

// TxTypeGuardianResign wraps GuardianResignation for block
// inclusion, matching the existing per-kind wrapper pattern.
const TxTypeGuardianResign TransactionType = "GUARDIAN_RESIGN"

// ResignationEffectiveAtPastTolerance is how far in the past
// an EffectiveAt may be. Covers clock skew at submission. A
// larger past value is rejected because it would invite
// backdating.
const ResignationEffectiveAtPastTolerance = 5 * time.Minute

// ResignationEffectiveAtMaxFuture is the maximum future horizon
// for an EffectiveAt. 1 year matches MaxRecoveryDelay; beyond
// that we want the guardian to re-sign with a fresher
// resignation.
const ResignationEffectiveAtMaxFuture = 365 * 24 * time.Hour

// ----- Wire types ----------------------------------------------------------

// GuardianResignation is the anchor a guardian submits to
// withdraw consent from a subject's recovery quorum.
type GuardianResignation struct {
	Kind             AnchorKind `json:"kind"`
	GuardianQuid     string     `json:"guardianQuid"`
	SubjectQuid      string     `json:"subjectQuid"`
	GuardianSetHash  string     `json:"guardianSetHash"`
	ResignationNonce int64      `json:"resignationNonce"`
	EffectiveAt      int64      `json:"effectiveAt"`
	Signature        string     `json:"signature"`
}

// GuardianResignationTransaction is the block-wrapper for a
// resignation anchor.
type GuardianResignationTransaction struct {
	BaseTransaction
	Resignation GuardianResignation `json:"resignation"`
}

// ----- Errors --------------------------------------------------------------

var (
	ErrResignationSubjectUnknown     = errors.New("guardian-resign: subject has no installed guardian set")
	ErrResignationNotMember          = errors.New("guardian-resign: resigning quid is not a member of the current set")
	ErrResignationSetHashMismatch    = errors.New("guardian-resign: GuardianSetHash does not match current installed set")
	ErrResignationReplay             = errors.New("guardian-resign: nonce must strictly increase")
	ErrResignationEffectiveAtPast    = errors.New("guardian-resign: EffectiveAt is beyond the permitted past tolerance")
	ErrResignationEffectiveAtTooFar  = errors.New("guardian-resign: EffectiveAt is farther than one year in the future")
	ErrResignationBadSignature       = errors.New("guardian-resign: guardian signature failed verification")
	ErrResignationNoGuardianKey      = errors.New("guardian-resign: no current public key recorded for the guardian")
	ErrResignationMissingGuardian    = errors.New("guardian-resign: missing guardianQuid")
	ErrResignationMissingSubject     = errors.New("guardian-resign: missing subjectQuid")
	ErrResignationMissingSetHash     = errors.New("guardian-resign: missing guardianSetHash")
	ErrResignationBadKind            = errors.New("guardian-resign: wrong anchor kind")
)

// ----- Canonicalization ----------------------------------------------------

// GuardianResignationSignableBytes returns the canonical bytes
// the guardian signs. Signature is cleared — the signature
// can't sign itself. All other fields are typed primitives so
// json.Marshal is deterministic across environments.
func GuardianResignationSignableBytes(r GuardianResignation) ([]byte, error) {
	r.Signature = ""
	return json.Marshal(r)
}

// GuardianSetHashOf returns the canonical hash of a guardian
// set used by resignations to pin a specific installed version.
// sha256(canonical-json). The GuardianSet struct has only typed
// fields and a typed Guardians slice, so this is deterministic.
func GuardianSetHashOf(set *GuardianSet) (string, error) {
	if set == nil {
		return "", errors.New("guardian-resign: nil set")
	}
	data, err := json.Marshal(set)
	if err != nil {
		return "", fmt.Errorf("guardian-resign: canonicalize set: %w", err)
	}
	h := sha256.Sum256(data)
	return hex.EncodeToString(h[:]), nil
}

// ----- Validation ----------------------------------------------------------

// ValidateGuardianResignation runs the full QDP-0006 §6 check
// chain. On success the caller (apply path) may store the
// resignation.
func ValidateGuardianResignation(l *NonceLedger, r GuardianResignation, now time.Time) error {
	if r.Kind != AnchorGuardianResign {
		return ErrResignationBadKind
	}
	if r.GuardianQuid == "" {
		return ErrResignationMissingGuardian
	}
	if r.SubjectQuid == "" {
		return ErrResignationMissingSubject
	}
	if r.GuardianSetHash == "" {
		return ErrResignationMissingSetHash
	}
	if r.Signature == "" {
		return ErrResignationBadSignature
	}

	// EffectiveAt window.
	nowUnix := now.Unix()
	if r.EffectiveAt < nowUnix-int64(ResignationEffectiveAtPastTolerance.Seconds()) {
		return ErrResignationEffectiveAtPast
	}
	if r.EffectiveAt > nowUnix+int64(ResignationEffectiveAtMaxFuture.Seconds()) {
		return ErrResignationEffectiveAtTooFar
	}

	if l == nil {
		return ErrResignationSubjectUnknown
	}

	// Subject has an installed set.
	set := l.GuardianSetOf(r.SubjectQuid)
	if set.Empty() {
		return ErrResignationSubjectUnknown
	}

	// Guardian is a member of the installed set.
	memberEpoch, isMember := guardianMemberEpoch(set, r.GuardianQuid)
	if !isMember {
		return ErrResignationNotMember
	}

	// Set hash binds resignation to exact installed version.
	currentHash, err := GuardianSetHashOf(set)
	if err != nil {
		return fmt.Errorf("guardian-resign: hash current set: %w", err)
	}
	if currentHash != r.GuardianSetHash {
		return ErrResignationSetHashMismatch
	}

	// Nonce monotonicity, keyed per (guardian, subject).
	if prev := l.GuardianResignationNonce(r.GuardianQuid, r.SubjectQuid); r.ResignationNonce <= prev {
		return ErrResignationReplay
	}

	// Signature against guardian's current epoch key.
	guardianKey, ok := l.GetSignerKey(r.GuardianQuid, memberEpoch)
	if !ok || guardianKey == "" {
		// Fall back to current epoch in case the guardian rotated
		// after joining the set — that's legal; the membership
		// Epoch in GuardianRef is the pinned key for RECOVERY
		// signatures (§QDP-0002), but for resignation we accept
		// either the pinned epoch or the current epoch.
		guardianKey, ok = l.GetSignerKey(r.GuardianQuid, l.CurrentEpoch(r.GuardianQuid))
		if !ok || guardianKey == "" {
			return ErrResignationNoGuardianKey
		}
	}
	signable, err := GuardianResignationSignableBytes(r)
	if err != nil {
		return fmt.Errorf("guardian-resign: canonicalize: %w", err)
	}
	if _, err := hex.DecodeString(r.Signature); err != nil {
		return fmt.Errorf("%w: %v", ErrResignationBadSignature, err)
	}
	if !VerifySignature(guardianKey, signable, r.Signature) {
		return ErrResignationBadSignature
	}
	return nil
}

// guardianMemberEpoch returns the membership epoch (the pinned
// key version at set-install time) for a guardian in the set,
// along with a boolean indicating membership. Zero-valued Epoch
// is a legitimate value (epoch-0 keys exist).
func guardianMemberEpoch(set *GuardianSet, guardian string) (uint32, bool) {
	if set == nil {
		return 0, false
	}
	for _, g := range set.Guardians {
		if g.Quid == guardian {
			return g.Epoch, true
		}
	}
	return 0, false
}
