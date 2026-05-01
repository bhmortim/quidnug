// HTTP handlers for the peer-quality API surface (Phase 4e).
//
//   GET /api/v1/peers
//     Returns the full peer scoreboard, sorted ascending by
//     composite (worst-first). Each entry includes the score
//     breakdown, severe-event counters, quarantine state, and
//     the most recent events from the per-peer ring buffer.
//
//   GET /api/v1/peers/{nodeQuid}
//     Returns the same shape for a single peer. 404 when no
//     score record exists (peer was admitted but no
//     interactions have been recorded).
//
// Both endpoints are read-only and unauthenticated, matching
// the rest of the /api/v1 surface. Operators concerned about
// fingerprint exposure should put the node behind an
// authenticated reverse proxy.
package core

import (
	"net/http"

	"github.com/gorilla/mux"
)

// GetPeersHandler serves the full scoreboard.
func (node *QuidnugNode) GetPeersHandler(w http.ResponseWriter, r *http.Request) {
	if node.PeerScoreboard == nil {
		WriteSuccess(w, map[string]interface{}{
			"peers": []any{},
			"note":  "scoring disabled",
		})
		return
	}
	snaps := node.PeerScoreboard.Snapshot()
	WriteSuccess(w, map[string]interface{}{
		"peers": snaps,
		"count": len(snaps),
	})
}

// GetPeerByQuidHandler serves a single peer's full record.
func (node *QuidnugNode) GetPeerByQuidHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	quid := vars["nodeQuid"]
	if quid == "" {
		WriteError(w, http.StatusBadRequest, "BAD_REQUEST", "nodeQuid is required")
		return
	}
	if node.PeerScoreboard == nil {
		WriteError(w, http.StatusServiceUnavailable, "SCORING_DISABLED", "peer scoring is not enabled")
		return
	}
	snap := node.PeerScoreboard.SnapshotOne(quid)
	if snap == nil {
		WriteError(w, http.StatusNotFound, "PEER_NOT_FOUND", "no score record for this peer")
		return
	}
	WriteSuccess(w, snap)
}
