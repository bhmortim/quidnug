// Package core — handlers_gossip_push.go
//
// HTTP endpoints for push-based gossip (QDP-0005 §6.2). Wired
// through registerCrossDomainRoutes alongside the existing pull
// endpoints.
//
//	POST /api/v2/gossip/push-anchor
//	  Receive a push-anchor envelope. 200 on dedup, 202 on
//	  accept, 4xx on validation failure.
//
//	POST /api/v2/gossip/push-fingerprint
//	  Same, for fingerprint pushes.
//
// Both endpoints are thin wrappers around ReceiveAnchorPush /
// ReceiveFingerprintPush; the heavy logic (dedup ordering,
// subscription filter, TTL clamp, rate limit, fan-out) lives
// in gossip_push.go so it can be unit-tested in-process
// without spinning up HTTP servers.
package core

import (
	"net/http"

	"github.com/gorilla/mux"
)

// registerPushGossipRoutes mounts the two push endpoints.
// Called from registerCrossDomainRoutes when the feature flag
// is on.
func (node *QuidnugNode) registerPushGossipRoutes(router *mux.Router) {
	router.HandleFunc("/gossip/push-anchor", node.ReceiveAnchorPushHandler).Methods("POST")
	router.HandleFunc("/gossip/push-fingerprint", node.ReceiveFingerprintPushHandler).Methods("POST")
}

// ReceiveAnchorPushHandler is the HTTP receiver for push-anchor.
func (node *QuidnugNode) ReceiveAnchorPushHandler(w http.ResponseWriter, r *http.Request) {
	var msg AnchorPushMessage
	if err := DecodeJSONBody(w, r, &msg); err != nil {
		return
	}
	if node.NonceLedger == nil {
		WriteError(w, http.StatusServiceUnavailable, "LEDGER_UNAVAILABLE", "nonce ledger not initialized")
		return
	}
	dup, err := node.ReceiveAnchorPush(msg)
	if err != nil {
		WriteError(w, http.StatusBadRequest, "INVALID_PUSH", err.Error())
		return
	}
	if dup {
		WriteSuccessWithStatus(w, http.StatusOK, map[string]interface{}{
			"messageId": msg.Payload.MessageID,
			"duplicate": true,
		})
		return
	}
	WriteSuccessWithStatus(w, http.StatusAccepted, map[string]interface{}{
		"messageId": msg.Payload.MessageID,
		"ttl":       msg.TTL,
		"hopCount":  msg.HopCount,
	})
}

// ReceiveFingerprintPushHandler is the HTTP receiver for
// push-fingerprint.
func (node *QuidnugNode) ReceiveFingerprintPushHandler(w http.ResponseWriter, r *http.Request) {
	var msg FingerprintPushMessage
	if err := DecodeJSONBody(w, r, &msg); err != nil {
		return
	}
	if node.NonceLedger == nil {
		WriteError(w, http.StatusServiceUnavailable, "LEDGER_UNAVAILABLE", "nonce ledger not initialized")
		return
	}
	dup, err := node.ReceiveFingerprintPush(msg)
	if err != nil {
		WriteError(w, http.StatusBadRequest, "INVALID_PUSH", err.Error())
		return
	}
	if dup {
		WriteSuccessWithStatus(w, http.StatusOK, map[string]interface{}{
			"domain":      msg.Payload.Domain,
			"blockHeight": msg.Payload.BlockHeight,
			"duplicate":   true,
		})
		return
	}
	WriteSuccessWithStatus(w, http.StatusAccepted, map[string]interface{}{
		"domain":      msg.Payload.Domain,
		"blockHeight": msg.Payload.BlockHeight,
		"ttl":         msg.TTL,
		"hopCount":    msg.HopCount,
	})
}
