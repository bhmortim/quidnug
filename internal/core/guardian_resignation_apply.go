// Package core — guardian_resignation_apply.go
//
// Ledger state for guardian resignations and the apply path
// called from processBlockTransactions.
package core

import "time"

// ----- Ledger accessors ----------------------------------------------------

// GuardianResignationNonce returns the highest resignation
// nonce seen for a (guardian, subject) pair, or 0 if none.
func (l *NonceLedger) GuardianResignationNonce(guardian, subject string) int64 {
	l.mu.RLock()
	defer l.mu.RUnlock()
	if l.guardianResignationNonces == nil {
		return 0
	}
	if m, ok := l.guardianResignationNonces[guardian]; ok {
		return m[subject]
	}
	return 0
}

// storeGuardianResignation appends a resignation to the subject's
// overlay list and records its nonce. Caller holds responsibility
// for validation; this is a low-level mutator.
func (l *NonceLedger) storeGuardianResignation(r GuardianResignation) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.guardianResignations == nil {
		l.guardianResignations = make(map[string][]GuardianResignation)
	}
	if l.guardianResignationNonces == nil {
		l.guardianResignationNonces = make(map[string]map[string]int64)
	}
	l.guardianResignations[r.SubjectQuid] = append(l.guardianResignations[r.SubjectQuid], r)
	if _, ok := l.guardianResignationNonces[r.GuardianQuid]; !ok {
		l.guardianResignationNonces[r.GuardianQuid] = make(map[string]int64)
	}
	l.guardianResignationNonces[r.GuardianQuid][r.SubjectQuid] = r.ResignationNonce
}

// GuardianResignationsOf returns a copy of all resignations
// recorded for the subject. Callers consulting for threshold
// purposes should use EffectiveGuardianSet instead.
func (l *NonceLedger) GuardianResignationsOf(subject string) []GuardianResignation {
	l.mu.RLock()
	defer l.mu.RUnlock()
	if src, ok := l.guardianResignations[subject]; ok {
		out := make([]GuardianResignation, len(src))
		copy(out, src)
		return out
	}
	return nil
}

// EffectiveGuardianSet returns the subject's installed set with
// resigned guardians' weights zeroed out. Resignations whose
// EffectiveAt is in the future are ignored (not yet in effect).
// Resignations whose GuardianSetHash does not match the current
// installed set are also ignored — they apply to a superseded
// version that no longer exists.
//
// Callers that need to know the raw installed set (for audit,
// diff, or GuardianSetUpdate authorization) still use
// GuardianSetOf directly. Threshold-checking callers use this
// accessor.
func (l *NonceLedger) EffectiveGuardianSet(subject string, now time.Time) *GuardianSet {
	set := l.GuardianSetOf(subject)
	if set.Empty() {
		return set
	}
	currentHash, err := GuardianSetHashOf(set)
	if err != nil {
		return set
	}

	resigned := l.GuardianResignationsOf(subject)
	if len(resigned) == 0 {
		return set
	}

	// Build a set of resigned guardian quids (only in-effect,
	// same-set-hash resignations).
	resignedNow := make(map[string]bool, len(resigned))
	for _, r := range resigned {
		if r.GuardianSetHash != currentHash {
			continue
		}
		if r.EffectiveAt > now.Unix() {
			continue
		}
		resignedNow[r.GuardianQuid] = true
	}
	if len(resignedNow) == 0 {
		return set
	}

	// Rebuild the guardians slice with resigned entries zeroed.
	newGuardians := make([]GuardianRef, 0, len(set.Guardians))
	for _, g := range set.Guardians {
		if resignedNow[g.Quid] {
			cp := g
			cp.Weight = 0 // zero-weight means present-but-inert
			newGuardians = append(newGuardians, cp)
			continue
		}
		newGuardians = append(newGuardians, g)
	}
	set.Guardians = newGuardians
	return set
}

// GuardianSetIsWeakened reports whether the subject's effective
// set has total weight below its installed threshold. A
// "weakened" set is still usable for recovery; the metric is
// intended for operator visibility.
//
// Computed directly against the raw installed set plus the
// resignation overlay to avoid the "Weight=0 means defaulted-
// to-1 vs. explicitly-resigned" ambiguity on the overlay view.
func (l *NonceLedger) GuardianSetIsWeakened(subject string, now time.Time) bool {
	raw := l.GuardianSetOf(subject)
	if raw.Empty() {
		return false
	}
	currentHash, err := GuardianSetHashOf(raw)
	if err != nil {
		return false
	}
	resignedSet := make(map[string]bool)
	for _, r := range l.GuardianResignationsOf(subject) {
		if r.GuardianSetHash != currentHash {
			continue
		}
		if r.EffectiveAt > now.Unix() {
			continue
		}
		resignedSet[r.GuardianQuid] = true
	}
	if len(resignedSet) == 0 {
		return false
	}
	var effective uint32
	for _, g := range raw.Guardians {
		if resignedSet[g.Quid] {
			continue
		}
		effective += uint32(g.EffectiveWeight())
	}
	return effective < uint32(raw.Threshold)
}

// ----- Apply path ----------------------------------------------------------

// applyGuardianResignation is the block-processing hook for a
// GuardianResignation. Matches the shape of the other
// guardian-anchor apply functions.
func (node *QuidnugNode) applyGuardianResignation(r GuardianResignation, block Block) {
	if node.NonceLedger == nil {
		return
	}
	if err := ValidateGuardianResignation(node.NonceLedger, r, time.Now()); err != nil {
		logger.Warn("Rejected guardian resignation in Trusted block",
			"blockIndex", block.Index,
			"guardian", r.GuardianQuid,
			"subject", r.SubjectQuid,
			"error", err)
		guardianResignationsRejected.WithLabelValues(resignationRejectReason(err)).Inc()
		return
	}
	node.NonceLedger.storeGuardianResignation(r)
	guardianResignationsTotal.WithLabelValues(r.SubjectQuid).Inc()
	if node.NonceLedger.GuardianSetIsWeakened(r.SubjectQuid, time.Now()) {
		guardianSetWeakened.WithLabelValues(r.SubjectQuid).Inc()
	}

	logger.Info("Applied guardian resignation",
		"subject", r.SubjectQuid,
		"guardian", r.GuardianQuid,
		"effectiveAt", r.EffectiveAt,
		"blockIndex", block.Index)
}

// resignationRejectReason maps validation errors to metric
// reason labels. Unmapped errors become "other" so the cardinal
// stays bounded.
func resignationRejectReason(err error) string {
	switch err {
	case ErrResignationBadKind:
		return "bad_kind"
	case ErrResignationMissingGuardian, ErrResignationMissingSubject, ErrResignationMissingSetHash:
		return "missing_field"
	case ErrResignationSubjectUnknown:
		return "unknown_subject"
	case ErrResignationNotMember:
		return "not_member"
	case ErrResignationSetHashMismatch:
		return "set_hash_mismatch"
	case ErrResignationReplay:
		return "replay"
	case ErrResignationEffectiveAtPast:
		return "effective_at_past"
	case ErrResignationEffectiveAtTooFar:
		return "effective_at_too_far"
	case ErrResignationBadSignature:
		return "bad_signature"
	case ErrResignationNoGuardianKey:
		return "no_guardian_key"
	default:
		return "other"
	}
}

// SignGuardianResignation is a convenience signer for the
// common case where this node is the guardian doing the
// resigning.
func (node *QuidnugNode) SignGuardianResignation(r GuardianResignation) (GuardianResignation, error) {
	data, err := GuardianResignationSignableBytes(r)
	if err != nil {
		return r, err
	}
	sig, err := node.SignData(data)
	if err != nil {
		return r, err
	}
	r.Signature = hexEncodeBytes(sig)
	return r, nil
}

// hexEncodeBytes is a tiny helper so this file doesn't depend on
// the top of the encoding/hex import chain. Mirrors how crypto.go
// does the same conversion.
func hexEncodeBytes(b []byte) string {
	const hextable = "0123456789abcdef"
	out := make([]byte, len(b)*2)
	for i, v := range b {
		out[i*2] = hextable[v>>4]
		out[i*2+1] = hextable[v&0x0f]
	}
	return string(out)
}
