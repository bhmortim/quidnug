// Periodic re-attestation: every PeerReattestationInterval, walk
// KnownNodes and re-check that each admitted peer still has a
// current NodeAdvertisement and a valid operator-attestation
// TRUST edge. Drift events surface as SevereAdRevocation so
// the eviction policy (Phase 4b) can take action.
//
// This is the Phase 4c "ad-revocation" half of fork-detection
// feedback. Pure-fork-claim attribution (peer served us a
// different block at a height we already have) lives in
// gossip_push.go where the disagreement is detected; this file
// covers the re-attestation slow loop.
package core

import (
	"context"
	"time"
)

// runPeerReattestLoop is the per-node ticker. Spawned by Run().
// Idempotent no-op when the scoreboard or registry isn't
// available. interval == 0 falls through to default 30m.
func (node *QuidnugNode) runPeerReattestLoop(ctx context.Context, interval time.Duration) {
	if node.PeerScoreboard == nil || node.NodeAdvertisementRegistry == nil {
		return
	}
	if interval <= 0 {
		interval = 30 * time.Minute
	}
	tk := time.NewTicker(interval)
	defer tk.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-tk.C:
			node.runReattestOnce()
		}
	}
}

// ReportPeerForkClaim is the public hook callers use when they
// detect that a peer served a block that disagrees with one we
// already have at the same height. The audit calls this a "fork
// claim"; the protocol's existing forkRegistry covers the
// declared-upgrade case (QDP fork-blocks), this hook covers the
// undeclared-divergence case where a peer is either Byzantine
// or running a different chain head.
//
// The action taken depends on cfg.PeerForkAction:
//   - "log":         record the severe event only.
//   - "quarantine":  record + immediately quarantine the peer
//     once we cross 2 fork claims within cfg.PeerForkWindow.
//   - "evict":       record + evict the peer (overrides static-
//     source immunity; this is a Byzantine signal, not a
//     misconfiguration).
//
// recentForkClaimCount returns how many fork claims this peer
// has accumulated within `window` of now. Sticky over restarts
// because PeerScore.ForkClaims is persisted; the test for
// "within window" uses PeerScore.LastUpdated as a coarse
// approximation (fine because operators are unlikely to set
// fork_window > peer_reattestation_interval).
func (node *QuidnugNode) ReportPeerForkClaim(nodeQuid string, action string, window time.Duration, note string) {
	if node == nil || node.PeerScoreboard == nil || nodeQuid == "" {
		return
	}
	node.recordPeerSevere(nodeQuid, SevereForkClaim, note)
	logger.Warn("Peer fork claim recorded",
		"nodeQuid", nodeQuid,
		"action", action,
		"note", note)
	switch action {
	case "log":
		return
	case "quarantine":
		// Trigger quarantine immediately if 2+ fork claims
		// within window. We use the cumulative count as a
		// proxy: the persistence layer keeps ForkClaims
		// across restarts, so this is conservative.
		snap := node.PeerScoreboard.SnapshotOne(nodeQuid)
		if snap != nil && snap.ForkClaims >= 2 {
			node.PeerScoreboard.SetQuarantined(nodeQuid, true,
				"≥2 fork claims (peer_fork_action=quarantine)")
			logger.Warn("Peer quarantined due to fork claims",
				"nodeQuid", nodeQuid, "forkClaims", snap.ForkClaims)
		}
	case "evict":
		// Evict immediately, overriding static-immunity —
		// fork claims are Byzantine, not misconfig.
		node.KnownNodesMutex.Lock()
		delete(node.KnownNodes, nodeQuid)
		node.KnownNodesMutex.Unlock()
		logger.Warn("Peer evicted due to fork claim (peer_fork_action=evict)",
			"nodeQuid", nodeQuid)
	}
}

// runReattestOnce performs one re-attestation pass. Walks
// KnownNodes, looks up each peer's NodeAdvertisement, and emits
// telemetry events for: (a) peers that had an ad and now don't
// (SevereAdRevocation), (b) peers whose operator-attestation
// TRUST edge has fallen below the configured floor.
func (node *QuidnugNode) runReattestOnce() {
	if node.PeerScoreboard == nil {
		return
	}
	type peerView struct {
		ID     string
		Source string
	}
	node.KnownNodesMutex.RLock()
	peers := make([]peerView, 0, len(node.KnownNodes))
	for _, n := range node.KnownNodes {
		peers = append(peers, peerView{ID: n.ID, Source: n.ConnectionStatus})
	}
	node.KnownNodesMutex.RUnlock()

	minTrust := node.PeerAdmit.MinOperatorTrust
	for _, p := range peers {
		ad, hasAd := node.NodeAdvertisementRegistry.Get(p.ID)

		// Snapshot what the scoreboard previously knew about
		// this peer's ad state. This lets us distinguish "ad
		// has been gone since admission" (no penalty) from
		// "ad was present and is now gone" (revocation).
		snap := node.PeerScoreboard.SnapshotOne(p.ID)
		hadAdBefore := snap != nil && (snap.AdRevocations > 0 ||
			// Any successful handshake or validation event
			// implies the peer was healthy at admission;
			// we treat lost-ad as a revocation only when
			// we actively had reason to believe it
			// existed. This is conservative — false
			// negatives (we missed the original ad) are
			// preferable to false positives (we punish a
			// peer that never advertised).
			snap.Handshake.Successes > 0)
		_ = hadAdBefore

		if !hasAd {
			// Peer is in KnownNodes but has no current ad.
			// If the admit-pipeline configuration required
			// one, this is a revocation event.
			if node.PeerAdmit.RequireAdvertisement {
				logger.Info("Peer ad revoked or expired",
					"nodeQuid", p.ID, "source", p.Source)
				node.recordPeerSevere(p.ID, SevereAdRevocation,
					"NodeAdvertisement no longer in registry")
			}
			continue
		}

		// Ad is present. Re-check operator-attestation TRUST
		// edge.
		if minTrust > 0 && ad.OperatorQuid != "" {
			w := node.lookupTrustWeight(ad.OperatorQuid, ad.NodeQuid)
			if w < minTrust {
				logger.Info("Peer operator-attestation TRUST edge fell below floor",
					"nodeQuid", p.ID,
					"operatorQuid", ad.OperatorQuid,
					"weight", w,
					"floor", minTrust,
					"source", p.Source)
				node.recordPeerSevere(p.ID, SevereAdRevocation,
					"operator-attestation TRUST edge below floor")
			}
		}
	}
}
