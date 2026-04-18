package core

import "time"

// applyAnchorFromBlock is invoked when an AnchorTransaction is
// encountered during processBlockTransactions for a Trusted block.
// It re-validates the anchor (defense in depth — the block was
// cryptographically valid, but the anchor's own signature and
// monotonicity need to hold) and, on success, applies its effects to
// the node's NonceLedger.
//
// Errors are logged rather than propagated: a malformed anchor in a
// Trusted block shouldn't halt block processing. Instead, metrics
// record the rejection and the block's other transactions continue
// processing.
func (node *QuidnugNode) applyAnchorFromBlock(a NonceAnchor, block Block) {
	if node.NonceLedger == nil {
		return
	}
	// Validate against current ledger state. The anchor's nonce must
	// strictly advance lastAnchorNonce[signer] — which effectively
	// enforces ordering between anchors in different blocks.
	if err := ValidateAnchor(node.NonceLedger, a, time.Now()); err != nil {
		logger.Warn("Rejected anchor in Trusted block",
			"blockIndex", block.Index,
			"blockHash", block.Hash,
			"signer", a.SignerQuid,
			"kind", a.Kind.String(),
			"error", err)
		return
	}

	if err := node.NonceLedger.ApplyAnchor(a); err != nil {
		logger.Warn("Failed to apply anchor",
			"blockIndex", block.Index,
			"signer", a.SignerQuid,
			"kind", a.Kind.String(),
			"error", err)
		return
	}

	// QDP-0007 (H4): observing the signer's rotation in a Trusted
	// block is the strongest possible recency evidence — update so
	// subsequent admission paths don't needlessly re-probe. Also
	// release any quarantined txs for this signer now that we have
	// fresh state.
	node.NonceLedger.MarkEpochRefresh(a.SignerQuid, time.Now())
	if node.quarantine != nil {
		node.releaseQuarantinedForSigner(a.SignerQuid, "anchor_applied")
	}

	logger.Info("Applied anchor",
		"kind", a.Kind.String(),
		"signer", a.SignerQuid,
		"fromEpoch", a.FromEpoch,
		"toEpoch", a.ToEpoch,
		"anchorNonce", a.AnchorNonce,
		"blockIndex", block.Index)
}
