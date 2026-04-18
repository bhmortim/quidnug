package core

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"time"
)

// Validation errors for guardian anchors. Kept separate from the
// generic nonce-ledger errors so test assertions can be precise.
var (
	ErrGuardianBadKind          = errors.New("guardian: wrong anchor kind for this validator")
	ErrGuardianMissingSubject   = errors.New("guardian: missing subjectQuid")
	ErrGuardianEmptySet         = errors.New("guardian: new set must contain at least one guardian")
	ErrGuardianBadThreshold     = errors.New("guardian: threshold must be in (0, TotalWeight]")
	ErrGuardianBadDelay         = errors.New("guardian: recovery delay outside [MinRecoveryDelay, MaxRecoveryDelay]")
	ErrGuardianBadPrimarySig     = errors.New("guardian: primary signature is absent or invalid")
	ErrGuardianInsufficientSigs  = errors.New("guardian: guardian signatures do not meet threshold")
	ErrGuardianUnknownGuardian   = errors.New("guardian: signing guardian is not in the current set")
	ErrGuardianDuplicateSigner   = errors.New("guardian: duplicate guardian signature")
	ErrGuardianStaleValidFrom    = errors.New("guardian: validFrom outside accepted window")
	ErrGuardianMaturityMissing   = errors.New("guardian: commit anchor predates its pending-recovery maturity")
	ErrGuardianCommitBadFields   = errors.New("guardian: commit anchor missing committer or signature")
	ErrGuardianVetoAmbiguous     = errors.New("guardian: veto must populate exactly one of primary or guardian signatures")
	ErrGuardianMissingConsent    = errors.New("guardian: every guardian in the new set must consent on-chain")
	ErrGuardianRotationForbidden = errors.New("guardian: plain AnchorRotation forbidden for subjects with RequireGuardianRotation")
)

// The windowing rules are shared with regular anchors so the same 5-min
// future skew and 30-day stale cap apply.
//   - AnchorMaxFutureSkew
//   - AnchorMaxAge

// ----- Canonicalization helpers -------------------------------------------

// GuardianRecoveryInitSignableBytes returns the canonical bytes a
// guardian signs. The GuardianSigs slice is cleared (signatures don't
// sign themselves, same rule as blocks and node anchors).
func GuardianRecoveryInitSignableBytes(a GuardianRecoveryInit) ([]byte, error) {
	a.GuardianSigs = nil
	return json.Marshal(a)
}

// GuardianRecoveryInitHash is the canonical identifier that Veto and
// Commit anchors reference. SHA-256 over the signable bytes.
func GuardianRecoveryInitHash(a GuardianRecoveryInit) (string, error) {
	data, err := GuardianRecoveryInitSignableBytes(a)
	if err != nil {
		return "", err
	}
	h := sha256.Sum256(data)
	return hex.EncodeToString(h[:]), nil
}

// GuardianRecoveryVetoSignableBytes — same pattern as above for veto.
func GuardianRecoveryVetoSignableBytes(v GuardianRecoveryVeto) ([]byte, error) {
	v.PrimarySignature = nil
	v.GuardianSigs = nil
	return json.Marshal(v)
}

// GuardianSetUpdateSignableBytes — same pattern for set update. All
// three signature fields are cleared so the canonical bytes the
// subject, the new guardians, and the current guardians all sign are
// byte-identical. Nobody's signature is allowed to alter what others
// are signing.
func GuardianSetUpdateSignableBytes(u GuardianSetUpdate) ([]byte, error) {
	u.PrimarySignature = nil
	u.NewGuardianConsents = nil
	u.CurrentGuardianSigs = nil
	return json.Marshal(u)
}

// ----- Validators ----------------------------------------------------------

// ValidateGuardianSetUpdate performs all structural, authorization,
// and temporal checks for an AnchorGuardianSetUpdate. See QDP-0002
// §6.4.4 for the authorization rules:
//
//   - Always: primary-key signature + consent from every guardian in
//     the new set.
//   - When replacing an existing set: also a threshold of the
//     CURRENT set's guardians.
//
// The three layers are enforced in the order above; the failure mode
// returned is the first unmet condition, so test assertions can be
// specific without having to construct worst-case anchors.
func ValidateGuardianSetUpdate(l *NonceLedger, u GuardianSetUpdate, now time.Time) error {
	if u.Kind != AnchorGuardianSetUpdate {
		return ErrGuardianBadKind
	}
	if u.SubjectQuid == "" {
		return ErrGuardianMissingSubject
	}
	if err := validateTimeWindow(u.ValidFrom, now); err != nil {
		return err
	}
	if err := validateGuardianSetShape(u.NewSet); err != nil {
		return err
	}
	if l == nil {
		return ErrGuardianSetNotFound // no ledger → can't check anything; conservative
	}
	if l.LastAnchorNonce(u.SubjectQuid) >= u.AnchorNonce {
		return ErrNonceNotMonotonic
	}

	// Layer 1: primary signature.
	if u.PrimarySignature == nil {
		return ErrGuardianBadPrimarySig
	}
	primaryKey, ok := l.GetSignerKey(u.SubjectQuid, u.PrimarySignature.KeyEpoch)
	if !ok {
		return ErrAnchorSignerKeyUnknown
	}
	signable, err := GuardianSetUpdateSignableBytes(u)
	if err != nil {
		return fmt.Errorf("guardian: canonicalization: %w", err)
	}
	if !VerifySignature(primaryKey, signable, u.PrimarySignature.Signature) {
		return ErrGuardianBadPrimarySig
	}

	// Layer 2: every guardian in the new set must have consented.
	if err := verifyNewGuardianConsents(l, u.NewSet, signable, u.NewGuardianConsents); err != nil {
		return err
	}

	// Layer 3: if a current set exists, require threshold-of-current.
	if current := l.GuardianSetOf(u.SubjectQuid); !current.Empty() {
		if err := verifyGuardianThreshold(l, current, signable, u.CurrentGuardianSigs); err != nil {
			return err
		}
	}
	return nil
}

// ValidateGuardianRecoveryInit validates an Init anchor: the subject
// must have a current guardian set, the supplied signatures must meet
// threshold, and there must be no in-flight recovery (the M=1
// concurrent-recovery constraint — QDP-0002 §6.1.1 leaves
// MaxConcurrentRecoveries configurable; this implementation treats 0
// as 1).
func ValidateGuardianRecoveryInit(l *NonceLedger, a GuardianRecoveryInit, now time.Time) error {
	if a.Kind != AnchorGuardianRecoveryInit {
		return ErrGuardianBadKind
	}
	if a.SubjectQuid == "" {
		return ErrGuardianMissingSubject
	}
	if a.ToEpoch <= a.FromEpoch {
		return ErrAnchorBadEpochProgression
	}
	if a.NewPublicKey == "" {
		return ErrAnchorMissingNewKey
	}
	if _, err := decodeP256PublicKey(a.NewPublicKey); err != nil {
		return fmt.Errorf("%w: %v", ErrAnchorBadNewKey, err)
	}
	if a.MinNextNonce < 1 {
		return ErrAnchorBadMinNext
	}
	if a.MaxAcceptedOldNonce < 0 {
		return ErrAnchorBadMaxOld
	}
	if err := validateTimeWindow(a.ValidFrom, now); err != nil {
		return err
	}
	if l == nil {
		return ErrGuardianSetNotFound
	}
	if a.AnchorNonce <= l.LastAnchorNonce(a.SubjectQuid) {
		return ErrNonceNotMonotonic
	}

	set := l.GuardianSetOf(a.SubjectQuid)
	if set.Empty() {
		return ErrGuardianSetNotFound
	}
	// Concurrent-recovery guard. Treat 0 as 1 (the sensible default).
	// A new Init is OK if the prior pending-recovery is terminal, but
	// not if it's still RecoveryPending.
	if p := l.PendingRecoveryOf(a.SubjectQuid); p != nil {
		return ErrGuardianRecoveryInFlight
	}

	signable, err := GuardianRecoveryInitSignableBytes(a)
	if err != nil {
		return fmt.Errorf("guardian: canonicalization: %w", err)
	}
	return verifyGuardianThreshold(l, set, signable, a.GuardianSigs)
}

// ValidateGuardianRecoveryVeto validates a Veto anchor. Exactly one
// authorization path must be present — either a primary-key signature
// OR a threshold of guardian signatures (QDP-0002 §6.4.2).
func ValidateGuardianRecoveryVeto(l *NonceLedger, v GuardianRecoveryVeto, now time.Time) error {
	if v.Kind != AnchorGuardianRecoveryVeto {
		return ErrGuardianBadKind
	}
	if v.SubjectQuid == "" {
		return ErrGuardianMissingSubject
	}
	if v.RecoveryAnchorHash == "" {
		return ErrGuardianRecoveryNotPending
	}
	if err := validateTimeWindow(v.ValidFrom, now); err != nil {
		return err
	}
	if l == nil {
		return ErrGuardianRecoveryNotPending
	}

	// Exactly-one-path enforcement.
	primaryPresent := v.PrimarySignature != nil
	guardianPresent := len(v.GuardianSigs) > 0
	if primaryPresent == guardianPresent {
		return ErrGuardianVetoAmbiguous
	}

	// There must be a matching pending recovery.
	p := l.PendingRecoveryOf(v.SubjectQuid)
	if p == nil || p.InitHash != v.RecoveryAnchorHash {
		return ErrGuardianRecoveryNotPending
	}
	// Monotonicity: the veto's AnchorNonce must strictly advance
	// lastAnchorNonce[subject], same rule as any other anchor.
	if v.AnchorNonce <= l.LastAnchorNonce(v.SubjectQuid) {
		return ErrNonceNotMonotonic
	}

	signable, err := GuardianRecoveryVetoSignableBytes(v)
	if err != nil {
		return fmt.Errorf("guardian: canonicalization: %w", err)
	}

	if primaryPresent {
		key, ok := l.GetSignerKey(v.SubjectQuid, v.PrimarySignature.KeyEpoch)
		if !ok {
			return ErrAnchorSignerKeyUnknown
		}
		if !VerifySignature(key, signable, v.PrimarySignature.Signature) {
			return ErrGuardianBadPrimarySig
		}
		return nil
	}

	// Guardian-threshold path.
	set := l.GuardianSetOf(v.SubjectQuid)
	if set.Empty() {
		return ErrGuardianSetNotFound
	}
	return verifyGuardianThreshold(l, set, signable, v.GuardianSigs)
}

// ValidateGuardianRecoveryCommit validates a Commit anchor. It only
// checks that a matching pending recovery exists and that the delay
// has elapsed; the committer signature is audit-only, not
// authorization, so it can be from any quid. We still verify it
// resolves against a known key to avoid accepting a commit from a
// ghost signer.
func ValidateGuardianRecoveryCommit(l *NonceLedger, c GuardianRecoveryCommit, now time.Time) error {
	if c.Kind != AnchorGuardianRecoveryCommit {
		return ErrGuardianBadKind
	}
	if c.SubjectQuid == "" {
		return ErrGuardianMissingSubject
	}
	if c.RecoveryAnchorHash == "" {
		return ErrGuardianRecoveryNotPending
	}
	if c.CommitterQuid == "" || c.CommitterSig == "" {
		return ErrGuardianCommitBadFields
	}
	if err := validateTimeWindow(c.ValidFrom, now); err != nil {
		return err
	}
	if l == nil {
		return ErrGuardianRecoveryNotPending
	}

	p := l.PendingRecoveryOf(c.SubjectQuid)
	if p == nil || p.InitHash != c.RecoveryAnchorHash {
		return ErrGuardianRecoveryNotPending
	}
	if now.Unix() < p.MaturityUnix {
		return ErrGuardianRecoveryImmature
	}
	// Commit's AnchorNonce must strictly advance — the ApplyAnchor
	// rotation it drives relies on this.
	if c.AnchorNonce <= l.LastAnchorNonce(c.SubjectQuid) {
		return ErrNonceNotMonotonic
	}

	// The committer is any signer whose epoch-0 key we know. We use
	// epoch 0 here for simplicity — committer authority is audit-only.
	committerKey, ok := l.GetSignerKey(c.CommitterQuid, 0)
	if !ok {
		return ErrAnchorSignerKeyUnknown
	}
	// Commit signs over the recovery-anchor hash itself to bind the
	// audit trail to the specific Init being finalized.
	if !VerifySignature(committerKey, []byte(c.RecoveryAnchorHash), c.CommitterSig) {
		return ErrGuardianBadPrimarySig
	}
	return nil
}

// ----- Helpers --------------------------------------------------------------

// validateTimeWindow enforces the shared "within 30 days of the past,
// within 5 minutes of the future" rule for anchor-like messages.
func validateTimeWindow(validFrom int64, now time.Time) error {
	if validFrom <= 0 {
		return ErrGuardianStaleValidFrom
	}
	vf := time.Unix(validFrom, 0)
	if now.Sub(vf) > AnchorMaxAge {
		return ErrGuardianStaleValidFrom
	}
	if vf.Sub(now) > AnchorMaxFutureSkew {
		return ErrGuardianStaleValidFrom
	}
	return nil
}

// validateGuardianSetShape enforces bounds on a newly-proposed
// GuardianSet. Keeps the invariant (0 < Threshold ≤ TotalWeight) and
// the delay-range contract.
func validateGuardianSetShape(set GuardianSet) error {
	if len(set.Guardians) == 0 {
		return ErrGuardianEmptySet
	}
	totalWeight := set.TotalWeight()
	if set.Threshold == 0 || uint32(set.Threshold) > totalWeight {
		return ErrGuardianBadThreshold
	}
	if set.RecoveryDelay < MinRecoveryDelay || set.RecoveryDelay > MaxRecoveryDelay {
		return ErrGuardianBadDelay
	}
	// No duplicate guardian quids.
	seen := make(map[string]struct{}, len(set.Guardians))
	for _, g := range set.Guardians {
		if g.Quid == "" {
			return ErrGuardianEmptySet
		}
		if _, dup := seen[g.Quid]; dup {
			return ErrGuardianDuplicateSigner
		}
		seen[g.Quid] = struct{}{}
	}
	return nil
}

// verifyNewGuardianConsents confirms that every guardian listed in
// the proposed NewSet has signed the update. This is the
// "no unwitting guardian" rule from QDP-0002 §6.4.4: a guardian who
// hasn't explicitly consented cannot later be held responsible for
// authorizing a recovery.
//
// Compared to verifyGuardianThreshold this is STRICTER in one way
// (every guardian must sign, not just threshold-many) and LAXER in
// another (we don't compare against the set's weighted Threshold —
// consent is a yes/no predicate per guardian, not a weighted sum).
func verifyNewGuardianConsents(l *NonceLedger, set GuardianSet, signable []byte, consents []GuardianSignature) error {
	required := make(map[string]uint32, len(set.Guardians))
	for _, g := range set.Guardians {
		required[g.Quid] = uint32(g.Epoch)
	}
	seen := make(map[string]struct{}, len(consents))

	for _, c := range consents {
		if _, dup := seen[c.GuardianQuid]; dup {
			return ErrGuardianDuplicateSigner
		}
		seen[c.GuardianQuid] = struct{}{}

		wantEpoch, expected := required[c.GuardianQuid]
		if !expected {
			return ErrGuardianUnknownGuardian
		}
		if uint32(c.KeyEpoch) != wantEpoch {
			return ErrGuardianUnknownGuardian
		}
		key, known := l.GetSignerKey(c.GuardianQuid, c.KeyEpoch)
		if !known {
			return ErrAnchorSignerKeyUnknown
		}
		if !VerifySignature(key, signable, c.Signature) {
			return ErrAnchorBadSignature
		}
	}

	// Every guardian in the new set must have consented.
	if len(seen) != len(required) {
		return ErrGuardianMissingConsent
	}
	return nil
}

// verifyGuardianThreshold confirms that the supplied signatures cover
// the set's threshold. Each signature must:
//   - come from a guardian currently in the set,
//   - use the guardian's pinned epoch,
//   - be cryptographically valid over `signable`.
//
// Duplicates are rejected (each guardian can contribute once).
func verifyGuardianThreshold(l *NonceLedger, set *GuardianSet, signable []byte, sigs []GuardianSignature) error {
	if set == nil {
		return ErrGuardianSetNotFound
	}
	// Build a quick index of guardians for O(1) lookup by quid.
	byQuid := make(map[string]GuardianRef, len(set.Guardians))
	for _, g := range set.Guardians {
		byQuid[g.Quid] = g
	}

	var accumulated uint32
	signersSeen := make(map[string]struct{}, len(sigs))

	for _, sig := range sigs {
		if _, dup := signersSeen[sig.GuardianQuid]; dup {
			return ErrGuardianDuplicateSigner
		}
		signersSeen[sig.GuardianQuid] = struct{}{}

		g, ok := byQuid[sig.GuardianQuid]
		if !ok {
			return ErrGuardianUnknownGuardian
		}
		if sig.KeyEpoch != g.Epoch {
			return ErrGuardianUnknownGuardian
		}
		key, known := l.GetSignerKey(sig.GuardianQuid, sig.KeyEpoch)
		if !known {
			return ErrAnchorSignerKeyUnknown
		}
		if !VerifySignature(key, signable, sig.Signature) {
			return ErrAnchorBadSignature
		}
		accumulated += uint32(g.EffectiveWeight())
	}

	if accumulated < uint32(set.Threshold) {
		return ErrGuardianInsufficientSigs
	}
	return nil
}
