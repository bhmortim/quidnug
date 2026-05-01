// Eviction + quarantine policy (Phase 4b).
//
// A goroutine ticks every 30s, walks every peer in
// PeerScoreboard, and applies the policy:
//
//   * Composite < quarantineThreshold and not currently
//     quarantined → mark quarantined.
//   * Composite ≥ quarantineThreshold + hysteresis and
//     currently quarantined → un-quarantine.
//   * Composite < evictionThreshold for ≥ evictionGrace →
//     evict from KnownNodes (drops the static-source check
//     for fork-action-induced eviction; otherwise honors
//     static-source immunity with a stern warning).
//
// Quarantined peers stay in KnownNodes; the routing-preference
// code (Phase 4d) is what filters them out of fan-out.
//
// Hysteresis: PeerQuarantineHysteresis is the gap between
// quarantine threshold and the un-quarantine threshold,
// preventing flap when a peer hovers right at the line.
package core

import (
	"context"
	"time"
)

// PeerQuarantineHysteresis is the threshold gap. Peer must rise
// to (quarantineThreshold + hysteresis) before un-quarantine.
const PeerQuarantineHysteresis = 0.10

// peerEvictionTickInterval is how often the loop wakes up.
// Coarse enough to be cheap on big peer sets; fine enough that
// a peer below threshold is acted on within a minute.
const peerEvictionTickInterval = 30 * time.Second

// runPeerEvictionLoop is the per-node ticker. Spawned by Run().
// Idempotent no-op when the scoreboard or thresholds aren't
// configured.
func (node *QuidnugNode) runPeerEvictionLoop(
	ctx context.Context,
	quarantineThreshold float64,
	evictionThreshold float64,
	evictionGrace time.Duration,
	staticImmune bool,
) {
	if node.PeerScoreboard == nil {
		return
	}
	if quarantineThreshold <= 0 && evictionThreshold <= 0 {
		// Both gates off — no work to do.
		return
	}
	if evictionGrace <= 0 {
		evictionGrace = 5 * time.Minute
	}
	tk := time.NewTicker(peerEvictionTickInterval)
	defer tk.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-tk.C:
			node.evaluatePeerScores(quarantineThreshold, evictionThreshold, evictionGrace, staticImmune)
		}
	}
}

// evaluatePeerScores applies the policy once. Walks
// KnownNodes, reads each peer's composite score, and
// quarantines / un-quarantines / evicts as appropriate. Logs
// every action at info level so operators can audit.
func (node *QuidnugNode) evaluatePeerScores(
	quarantineThreshold float64,
	evictionThreshold float64,
	evictionGrace time.Duration,
	staticImmune bool,
) {
	if node.PeerScoreboard == nil {
		return
	}
	// Snapshot KnownNodes IDs + sources under the lock so the
	// policy run doesn't hold the lock during scoreboard reads.
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

	for _, p := range peers {
		composite := node.PeerScoreboard.Composite(p.ID)
		isStatic := p.Source == "static"
		alreadyQuarantined := node.PeerScoreboard.IsQuarantined(p.ID)

		// Quarantine logic with hysteresis.
		if quarantineThreshold > 0 {
			if !alreadyQuarantined && composite < quarantineThreshold {
				node.PeerScoreboard.SetQuarantined(p.ID, true,
					"composite below quarantine threshold")
				logger.Info("Peer quarantined",
					"nodeQuid", p.ID,
					"source", p.Source,
					"composite", composite,
					"threshold", quarantineThreshold)
			} else if alreadyQuarantined && composite >= quarantineThreshold+PeerQuarantineHysteresis {
				node.PeerScoreboard.SetQuarantined(p.ID, false, "")
				logger.Info("Peer un-quarantined",
					"nodeQuid", p.ID,
					"source", p.Source,
					"composite", composite)
			}
		}

		// Eviction logic with grace clock.
		if evictionThreshold > 0 {
			if composite < evictionThreshold {
				since := node.PeerScoreboard.MarkBelowEviction(p.ID, true)
				if !since.IsZero() && time.Since(since) >= evictionGrace {
					if isStatic && staticImmune {
						// Operator-listed peer; we
						// don't auto-evict but we do
						// shout. This is the "fix
						// peers.yaml, not have it
						// silently fixed for you"
						// contract.
						logger.Warn("Static peer is below eviction threshold (immunity active — operator should investigate or remove from peers_file)",
							"nodeQuid", p.ID,
							"source", p.Source,
							"composite", composite,
							"threshold", evictionThreshold,
							"belowSince", since.Format(time.RFC3339))
						continue
					}
					// Evict.
					node.KnownNodesMutex.Lock()
					delete(node.KnownNodes, p.ID)
					node.KnownNodesMutex.Unlock()
					logger.Info("Peer evicted (composite below threshold beyond grace)",
						"nodeQuid", p.ID,
						"source", p.Source,
						"composite", composite,
						"threshold", evictionThreshold,
						"belowSince", since.Format(time.RFC3339))
					// Don't Forget the score record yet —
					// keep history so a re-admit can pick
					// up where we left off.
				}
			} else {
				// Score recovered; clear the grace clock.
				node.PeerScoreboard.MarkBelowEviction(p.ID, false)
			}
		}
	}
}
