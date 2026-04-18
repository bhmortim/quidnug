package core

import "errors"

// Guardian-related extensions to NonceLedger. Kept in a separate file
// from the bulk of ledger.go so the guardian feature can be reasoned
// about as a self-contained addition on top of the QDP-0001 core.

// Errors returned by guardian ledger operations. Use errors.Is.
var (
	ErrGuardianSetNotFound        = errors.New("guardian: no guardian set declared for subject")
	ErrGuardianSetExists          = errors.New("guardian: a guardian set is already declared for subject")
	ErrGuardianRecoveryInFlight   = errors.New("guardian: a pending recovery already exists for subject")
	ErrGuardianRecoveryNotPending = errors.New("guardian: no pending recovery found for hash")
	ErrGuardianRecoveryImmature   = errors.New("guardian: recovery delay has not elapsed")
	ErrGuardianRecoveryTerminal   = errors.New("guardian: recovery is already committed or vetoed")
)

// GuardianSetOf returns a deep-enough copy of the stored guardian set
// for the subject, or nil if none exists. The returned *GuardianSet is
// safe to read without holding the ledger lock but should not be
// mutated (callers that need to modify must go through an
// ApplyGuardianSetUpdate call).
func (l *NonceLedger) GuardianSetOf(subject string) *GuardianSet {
	l.mu.RLock()
	defer l.mu.RUnlock()
	set, ok := l.guardianSets[subject]
	if !ok {
		return nil
	}
	// Shallow-copy the struct; copy the slice too so callers can't
	// mutate our storage.
	cp := *set
	cp.Guardians = append([]GuardianRef(nil), set.Guardians...)
	return &cp
}

// PendingRecoveryOf returns the current pending-recovery record for
// the subject, or nil if none. Only RecoveryPending records are
// returned; terminal states (Committed / Vetoed / Replaced) are
// cleaned up at transition time.
func (l *NonceLedger) PendingRecoveryOf(subject string) *PendingRecovery {
	l.mu.RLock()
	defer l.mu.RUnlock()
	p, ok := l.pendingRecoveries[subject]
	if !ok {
		return nil
	}
	if p.State != RecoveryPending {
		return nil
	}
	cp := *p
	return &cp
}

// SetGuardianSet installs a guardian set for the subject. Unlike a
// public API, this is a low-level mutator: validation and
// authorization are the caller's responsibility.
func (l *NonceLedger) setGuardianSet(subject string, set *GuardianSet) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.guardianSets == nil {
		l.guardianSets = make(map[string]*GuardianSet)
	}
	l.guardianSets[subject] = set
}

// beginPendingRecovery stores a new pending-recovery record.
// Assumes the caller has already ensured no in-flight recovery exists.
func (l *NonceLedger) beginPendingRecovery(subject string, pr *PendingRecovery) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.pendingRecoveries == nil {
		l.pendingRecoveries = make(map[string]*PendingRecovery)
	}
	// If a previous record exists (Vetoed/Committed leftover), mark
	// it Replaced first so the historical trail is consistent.
	if prev, ok := l.pendingRecoveries[subject]; ok && prev.State == RecoveryPending {
		prev.State = RecoveryReplaced
	}
	l.pendingRecoveries[subject] = pr
}

// finalizePendingRecovery transitions the stored record to a terminal
// state (Committed or Vetoed). Returns the record so callers can log
// it. Returns ErrGuardianRecoveryNotPending if no matching pending
// recovery exists.
func (l *NonceLedger) finalizePendingRecovery(subject, hash string, newState RecoveryState) (*PendingRecovery, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	p, ok := l.pendingRecoveries[subject]
	if !ok || p.State != RecoveryPending || p.InitHash != hash {
		return nil, ErrGuardianRecoveryNotPending
	}
	p.State = newState
	return p, nil
}
