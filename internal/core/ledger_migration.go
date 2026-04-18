package core

// MigrateLedgerFromBlocks reconstructs a NonceLedger from a pre-QDP-0001
// block history, as defined in QDP-0001 §10.2.1. For each block in
// `blocks` (ordered oldest → newest), the function walks every
// transaction and raises the accepted counter for its signer.
//
// The rule:
//
//	ledger.accepted[(signer, domain, 0)] = max observed nonce
//
// where "observed nonce" comes from:
//   - TrustTransaction.Nonce (keyed by Truster)
//   - IdentityTransaction.UpdateNonce (keyed by Creator, or QuidID if Creator is empty)
//   - EventTransaction.Sequence (keyed by the tx PublicKey-derived quid)
//
// The resulting ledger is **deterministic**: two honest nodes given the
// same `blocks` slice in the same order compute byte-identical accepted
// maps. Any non-determinism here is a consensus bug.
//
// Callers typically invoke this once at the QDP-0001 fork block to seed
// the ledger before switching to checkpoint-based updates. After the
// fork, ApplyCheckpoints is the authoritative update path.
//
// See the design doc: docs/design/0001-global-nonce-ledger.md §10.2.1.
func MigrateLedgerFromBlocks(blocks []Block) *NonceLedger {
	ledger := NewNonceLedger()

	// We deliberately bypass the ledger's validation (Admit) here and
	// only write to the accepted map. Pre-fork transactions were
	// validated by the pre-fork rules; the migration's job is just to
	// snapshot the final high-water mark so post-fork validators agree
	// on where to resume.
	for _, block := range blocks {
		domain := block.TrustProof.TrustDomain
		for _, raw := range block.Transactions {
			switch tx := raw.(type) {
			case TrustTransaction:
				applyMigrationMax(ledger, tx.Truster, domain, tx.Nonce)
			case *TrustTransaction:
				if tx != nil {
					applyMigrationMax(ledger, tx.Truster, domain, tx.Nonce)
				}
			case IdentityTransaction:
				signer := tx.Creator
				if signer == "" {
					signer = tx.QuidID
				}
				applyMigrationMax(ledger, signer, domain, tx.UpdateNonce)
			case *IdentityTransaction:
				if tx != nil {
					signer := tx.Creator
					if signer == "" {
						signer = tx.QuidID
					}
					applyMigrationMax(ledger, signer, domain, tx.UpdateNonce)
				}
			case EventTransaction:
				// Event streams don't carry an explicit signer-quid
				// field today (the identity is derived from PublicKey
				// at runtime). Conservatively scope events under their
				// SubjectID so that post-fork the subject's own
				// per-stream sequence remains the dominant constraint.
				applyMigrationMax(ledger, tx.SubjectID, domain, tx.Sequence)
			case *EventTransaction:
				if tx != nil {
					applyMigrationMax(ledger, tx.SubjectID, domain, tx.Sequence)
				}
				// TitleTransaction has no pre-fork nonce; nothing to do.
			}
		}
	}

	return ledger
}

func applyMigrationMax(l *NonceLedger, signer, domain string, nonce int64) {
	if signer == "" || nonce <= 0 {
		return
	}
	key := NonceKey{Quid: signer, Domain: domain, Epoch: 0}
	// Direct writes under lock — bypasses Admit's gap and epoch rules
	// because pre-fork data predates both.
	l.mu.Lock()
	defer l.mu.Unlock()
	if cur, ok := l.accepted[key]; !ok || nonce > cur {
		l.accepted[key] = nonce
	}
	if cur, ok := l.tentative[key]; !ok || nonce > cur {
		l.tentative[key] = nonce
	}
}
