package core

import "time"

// Apply functions for guardian anchors. Each function re-validates the
// anchor before mutating ledger state (defense in depth: a malformed
// anchor in a Trusted block is logged, not fatal, and state is left
// untouched).
//
// These mirror the `applyAnchorFromBlock` pattern used by QDP-0001
// simple anchors.

// applyGuardianSetUpdate applies a validated GuardianSetUpdate.
// Called from processBlockTransactions on TxTypeGuardianSetUpdate.
func (node *QuidnugNode) applyGuardianSetUpdate(u GuardianSetUpdate, block Block) {
	if node.NonceLedger == nil {
		return
	}
	if err := ValidateGuardianSetUpdate(node.NonceLedger, u, time.Now()); err != nil {
		logger.Warn("Rejected guardian set update in Trusted block",
			"blockIndex", block.Index, "subject", u.SubjectQuid, "error", err)
		return
	}

	// Stamp the updated-at block height so downstream tooling can tell
	// when the set was last changed.
	newSet := u.NewSet
	newSet.UpdatedAtBlock = block.Index
	node.NonceLedger.setGuardianSet(u.SubjectQuid, &newSet)
	node.NonceLedger.mu.Lock()
	node.NonceLedger.lastAnchorNonce[u.SubjectQuid] = u.AnchorNonce
	node.NonceLedger.mu.Unlock()

	logger.Info("Installed guardian set",
		"subject", u.SubjectQuid,
		"guardians", len(newSet.Guardians),
		"threshold", newSet.Threshold,
		"blockIndex", block.Index)
}

// applyGuardianRecoveryInit applies a validated Init, opening the
// delay window. The Init is stored verbatim so the commit step can
// mirror its recorded rotation fields without re-deriving anything.
func (node *QuidnugNode) applyGuardianRecoveryInit(a GuardianRecoveryInit, block Block) {
	if node.NonceLedger == nil {
		return
	}
	if err := ValidateGuardianRecoveryInit(node.NonceLedger, a, time.Now()); err != nil {
		logger.Warn("Rejected guardian recovery init in Trusted block",
			"blockIndex", block.Index, "subject", a.SubjectQuid, "error", err)
		return
	}

	hash, err := GuardianRecoveryInitHash(a)
	if err != nil {
		logger.Warn("Failed to hash guardian recovery init",
			"blockIndex", block.Index, "subject", a.SubjectQuid, "error", err)
		return
	}

	set := node.NonceLedger.GuardianSetOf(a.SubjectQuid)
	if set.Empty() {
		// Validator already checked, but belt + suspenders.
		return
	}
	maturity := time.Unix(a.ValidFrom, 0).Add(set.RecoveryDelay).Unix()

	pr := &PendingRecovery{
		InitHash:        hash,
		Init:            a,
		InitBlockHeight: block.Index,
		MaturityUnix:    maturity,
		State:           RecoveryPending,
	}
	node.NonceLedger.beginPendingRecovery(a.SubjectQuid, pr)

	node.NonceLedger.mu.Lock()
	node.NonceLedger.lastAnchorNonce[a.SubjectQuid] = a.AnchorNonce
	node.NonceLedger.mu.Unlock()

	logger.Info("Guardian recovery initiated",
		"subject", a.SubjectQuid,
		"fromEpoch", a.FromEpoch,
		"toEpoch", a.ToEpoch,
		"hash", hash,
		"maturityUnix", maturity,
		"blockIndex", block.Index)
}

// applyGuardianRecoveryVeto cancels a pending recovery. Does nothing
// if validation fails or no matching pending record exists.
func (node *QuidnugNode) applyGuardianRecoveryVeto(v GuardianRecoveryVeto, block Block) {
	if node.NonceLedger == nil {
		return
	}
	if err := ValidateGuardianRecoveryVeto(node.NonceLedger, v, time.Now()); err != nil {
		logger.Warn("Rejected guardian recovery veto in Trusted block",
			"blockIndex", block.Index, "subject", v.SubjectQuid, "error", err)
		return
	}

	p, err := node.NonceLedger.finalizePendingRecovery(v.SubjectQuid, v.RecoveryAnchorHash, RecoveryVetoed)
	if err != nil {
		logger.Warn("Guardian veto referenced unknown or terminal recovery",
			"subject", v.SubjectQuid, "hash", v.RecoveryAnchorHash, "error", err)
		return
	}
	node.NonceLedger.mu.Lock()
	node.NonceLedger.lastAnchorNonce[v.SubjectQuid] = v.AnchorNonce
	node.NonceLedger.mu.Unlock()

	logger.Info("Guardian recovery vetoed",
		"subject", v.SubjectQuid,
		"hash", p.InitHash,
		"originalBlock", p.InitBlockHeight,
		"blockIndex", block.Index)
}

// applyGuardianRecoveryCommit finalizes a mature pending recovery.
// The effect is equivalent to applying an AnchorRotation — in fact we
// construct one from the stored Init and call the existing
// ApplyAnchor machinery, so rotation semantics stay in exactly one
// place (the anchor.go / ledger.go rotation path).
func (node *QuidnugNode) applyGuardianRecoveryCommit(c GuardianRecoveryCommit, block Block) {
	if node.NonceLedger == nil {
		return
	}
	if err := ValidateGuardianRecoveryCommit(node.NonceLedger, c, time.Now()); err != nil {
		logger.Warn("Rejected guardian recovery commit in Trusted block",
			"blockIndex", block.Index, "subject", c.SubjectQuid, "error", err)
		return
	}

	p, err := node.NonceLedger.finalizePendingRecovery(c.SubjectQuid, c.RecoveryAnchorHash, RecoveryCommitted)
	if err != nil {
		logger.Warn("Guardian commit referenced unknown or terminal recovery",
			"subject", c.SubjectQuid, "hash", c.RecoveryAnchorHash, "error", err)
		return
	}

	// Reconstruct the equivalent rotation anchor. The Init's
	// AnchorNonce was already consumed at init time, so we use a
	// strictly-greater one — the commit's own AnchorNonce serves.
	// (Note: this means the signer's anchor-nonce advances twice per
	// recovery: once at Init, once at Commit. Both are the signer's,
	// so monotonicity holds.)
	rotation := NonceAnchor{
		Kind:                AnchorRotation,
		SignerQuid:          c.SubjectQuid,
		FromEpoch:           p.Init.FromEpoch,
		ToEpoch:             p.Init.ToEpoch,
		NewPublicKey:        p.Init.NewPublicKey,
		MinNextNonce:        p.Init.MinNextNonce,
		MaxAcceptedOldNonce: p.Init.MaxAcceptedOldNonce,
		ValidFrom:           c.ValidFrom,
		AnchorNonce:         c.AnchorNonce,
		// No Signature here — we're applying state directly, not
		// revalidating against the Init's original guardian
		// signatures (already validated at init time).
	}
	if err := node.NonceLedger.ApplyAnchor(rotation); err != nil {
		logger.Warn("Failed to apply rotation from guardian commit",
			"subject", c.SubjectQuid, "error", err)
		return
	}

	logger.Info("Guardian recovery committed",
		"subject", c.SubjectQuid,
		"fromEpoch", p.Init.FromEpoch,
		"toEpoch", p.Init.ToEpoch,
		"hash", p.InitHash,
		"committer", c.CommitterQuid,
		"blockIndex", block.Index)
}
