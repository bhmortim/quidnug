// Package core — handlers_snapshot.go
//
// HTTP endpoints for snapshot distribution (QDP-0008 / H3).
// Complements the existing pull-based gossip surface. A
// bootstrapping peer fetches the latest snapshot for each
// domain it cares about via these endpoints.
package core

import (
	"net/http"

	"github.com/gorilla/mux"
)

// registerSnapshotRoutes mounts the snapshot endpoints. Called
// from registerCrossDomainRoutes alongside the fingerprint and
// gossip endpoints.
func (node *QuidnugNode) registerSnapshotRoutes(router *mux.Router) {
	router.HandleFunc("/nonce-snapshots/{domain}/latest", node.GetLatestNonceSnapshotHandler).Methods("GET")
	router.HandleFunc("/nonce-snapshots", node.SubmitNonceSnapshotHandler).Methods("POST")
	router.HandleFunc("/bootstrap/status", node.GetBootstrapStatusHandler).Methods("GET")
}

// GetLatestNonceSnapshotHandler returns the latest stored
// snapshot for the requested domain. 404 if none.
func (node *QuidnugNode) GetLatestNonceSnapshotHandler(w http.ResponseWriter, r *http.Request) {
	domain := mux.Vars(r)["domain"]
	if node.NonceLedger == nil {
		WriteError(w, http.StatusServiceUnavailable, "LEDGER_UNAVAILABLE", "nonce ledger not initialized")
		return
	}
	snap, ok := node.NonceLedger.GetLatestSnapshot(domain)
	if !ok {
		WriteError(w, http.StatusNotFound, "NOT_FOUND", "no snapshot stored for this domain")
		return
	}
	WriteSuccess(w, snap)
}

// SubmitNonceSnapshotHandler accepts a signed NonceSnapshot for
// a domain the receiver may or may not already have state for.
// Validates the signature, then stores monotonically. Returns
// 202 on new, 200 with duplicate on stale.
func (node *QuidnugNode) SubmitNonceSnapshotHandler(w http.ResponseWriter, r *http.Request) {
	var snap NonceSnapshot
	if err := DecodeJSONBody(w, r, &snap); err != nil {
		return
	}
	if node.NonceLedger == nil {
		WriteError(w, http.StatusServiceUnavailable, "LEDGER_UNAVAILABLE", "nonce ledger not initialized")
		return
	}
	if err := VerifySnapshot(node.NonceLedger, snap); err != nil {
		WriteError(w, http.StatusBadRequest, "INVALID_SNAPSHOT", err.Error())
		return
	}
	if cur, ok := node.NonceLedger.GetLatestSnapshot(snap.TrustDomain); ok && cur.BlockHeight >= snap.BlockHeight {
		WriteSuccessWithStatus(w, http.StatusOK, map[string]interface{}{
			"domain":      snap.TrustDomain,
			"blockHeight": snap.BlockHeight,
			"duplicate":   true,
		})
		return
	}
	node.NonceLedger.StoreLatestSnapshot(snap)
	WriteSuccessWithStatus(w, http.StatusAccepted, map[string]interface{}{
		"domain":      snap.TrustDomain,
		"blockHeight": snap.BlockHeight,
	})
}

// GetBootstrapStatusHandler returns the current/last bootstrap
// session for operator visibility.
func (node *QuidnugNode) GetBootstrapStatusHandler(w http.ResponseWriter, r *http.Request) {
	sess := node.GetBootstrapSession()
	if sess == nil {
		WriteSuccess(w, map[string]interface{}{
			"hasSession": false,
		})
		return
	}
	peerIDs := make([]string, 0, len(sess.Responses))
	for id := range sess.Responses {
		peerIDs = append(peerIDs, id)
	}
	var errStr string
	if sess.Error != nil {
		errStr = sess.Error.Error()
	}
	WriteSuccess(w, map[string]interface{}{
		"hasSession":       true,
		"domain":           sess.Domain,
		"state":            sess.State.String(),
		"k":                sess.K,
		"peerCount":        len(sess.PeerSet),
		"responseCount":    len(sess.Responses),
		"respondedPeerIds": peerIDs,
		"startedUnix":      sess.Started.Unix(),
		"endedUnix":        sess.Ended.Unix(),
		"shadowLeft":       sess.ShadowLeft,
		"error":            errStr,
	})
}
