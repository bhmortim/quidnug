// Package core — handlers_gossip_v2.go
//
// HTTP endpoints for cross-domain fingerprint and anchor gossip
// (QDP-0003). Kept under /api/v2/ and registered through the
// registerCrossDomainRoutes helper so the new surface is clearly
// separated from the v1 per-domain endpoints.
//
// Two deliberately thin operations:
//
//	POST /api/v2/domain-fingerprints
//	  Submit a signed DomainFingerprint. Validator signature is
//	  verified against the ledger's recorded key for
//	  fp.ProducerQuid at the producer's current epoch. Monotonic
//	  by block height — older heights are silently accepted as
//	  valid but do not overwrite newer entries.
//
//	GET /api/v2/domain-fingerprints/{domain}/latest
//	  Return the latest fingerprint known to this node for the
//	  named domain, or 404 if none. Useful for peers doing lazy
//	  subscription.
//
//	POST /api/v2/anchor-gossip
//	  Submit a signed AnchorGossipMessage. The full validation
//	  chain (gossip-sig, fingerprint, block integrity, tx index,
//	  dedup) runs; on success the referenced anchor is applied to
//	  the local ledger.
package core

import (
	"net/http"
	"time"

	"github.com/gorilla/mux"
)

// registerCrossDomainRoutes mounts QDP-0003 endpoints. Called from
// StartServerWithConfig's v2 subrouter alongside the guardian routes.
// QDP-0005 push endpoints are always mounted so receivers can
// process pushes from early-adopter producers during the shadow
// rollout. The PushGossipEnabled flag only controls whether THIS
// node ORIGINATES pushes (PushAnchor/PushFingerprint).
func (node *QuidnugNode) registerCrossDomainRoutes(router *mux.Router) {
	router.HandleFunc("/domain-fingerprints", node.SubmitDomainFingerprintHandler).Methods("POST")
	router.HandleFunc("/domain-fingerprints/{domain}/latest", node.GetLatestDomainFingerprintHandler).Methods("GET")
	router.HandleFunc("/anchor-gossip", node.SubmitAnchorGossipHandler).Methods("POST")
	node.registerPushGossipRoutes(router)
}

// SubmitDomainFingerprintHandler accepts a signed DomainFingerprint
// and stores it. Returns 202 on success, 400 on invalid input.
func (node *QuidnugNode) SubmitDomainFingerprintHandler(w http.ResponseWriter, r *http.Request) {
	var fp DomainFingerprint
	if err := DecodeJSONBody(w, r, &fp); err != nil {
		return
	}
	if node.NonceLedger == nil {
		WriteError(w, http.StatusServiceUnavailable, "LEDGER_UNAVAILABLE", "nonce ledger not initialized")
		return
	}
	if err := VerifyDomainFingerprint(node.NonceLedger, fp, time.Now()); err != nil {
		WriteError(w, http.StatusBadRequest, "INVALID_FINGERPRINT", err.Error())
		return
	}
	node.NonceLedger.StoreDomainFingerprint(fp)
	WriteSuccessWithStatus(w, http.StatusAccepted, map[string]interface{}{
		"domain":      fp.Domain,
		"blockHeight": fp.BlockHeight,
		"blockHash":   fp.BlockHash,
	})
}

// GetLatestDomainFingerprintHandler returns the latest stored
// fingerprint for the requested domain, 404 if none.
func (node *QuidnugNode) GetLatestDomainFingerprintHandler(w http.ResponseWriter, r *http.Request) {
	domain := mux.Vars(r)["domain"]
	if node.NonceLedger == nil {
		WriteError(w, http.StatusServiceUnavailable, "LEDGER_UNAVAILABLE", "nonce ledger not initialized")
		return
	}
	fp, ok := node.NonceLedger.GetDomainFingerprint(domain)
	if !ok {
		WriteError(w, http.StatusNotFound, "NOT_FOUND", "no fingerprint stored for this domain")
		return
	}
	WriteSuccess(w, fp)
}

// SubmitAnchorGossipHandler accepts a cross-domain anchor gossip
// message, validates + applies it. The applied anchor mutates the
// node's nonce-ledger global-per-signer state (epoch, keys, caps);
// domain-scoped nonce counters are NOT mutated by a gossip message
// because that's the local domain's consensus business.
//
// Duplicate MessageIDs return 200 Accepted to support idempotent
// retries from relays, but with a message body noting dedup.
func (node *QuidnugNode) SubmitAnchorGossipHandler(w http.ResponseWriter, r *http.Request) {
	var m AnchorGossipMessage
	if err := DecodeJSONBody(w, r, &m); err != nil {
		return
	}
	if node.NonceLedger == nil {
		WriteError(w, http.StatusServiceUnavailable, "LEDGER_UNAVAILABLE", "nonce ledger not initialized")
		return
	}

	// Check dedup up front so retries get a clean 200 rather than a
	// duplicate-label 400.
	if node.NonceLedger.seenGossip(m.MessageID) {
		WriteSuccessWithStatus(w, http.StatusOK, map[string]interface{}{
			"messageId": m.MessageID,
			"duplicate": true,
		})
		return
	}

	if err := node.ApplyAnchorGossip(m); err != nil {
		WriteError(w, http.StatusBadRequest, "INVALID_GOSSIP", err.Error())
		return
	}
	WriteSuccessWithStatus(w, http.StatusAccepted, map[string]interface{}{
		"messageId":         m.MessageID,
		"originDomain":      m.OriginDomain,
		"originBlockHeight": m.OriginBlockHeight,
	})
}
