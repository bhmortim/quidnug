// Package core — handlers_guardian_resign.go
//
// HTTP surface for guardian resignation (QDP-0006 / H6).
// Deliberately separate from handlers_guardian.go so H6 can be
// read (and if needed reverted) as a single coherent addition.
package core

import (
	"net/http"
	"time"

	"github.com/gorilla/mux"
)

// SubmitGuardianResignationHandler accepts a signed
// GuardianResignation, validates it against the current
// ledger, and enqueues it as a pending transaction.
//
// Responses:
//   - 202 Accepted — validated and enqueued.
//   - 200 OK with duplicate:true — the resignation nonce has
//     already been accepted (idempotent retry).
//   - 400 Bad Request — validation failed (error in body).
//   - 503 Service Unavailable — nonce ledger not initialized.
func (node *QuidnugNode) SubmitGuardianResignationHandler(w http.ResponseWriter, r *http.Request) {
	var a GuardianResignation
	if err := DecodeJSONBody(w, r, &a); err != nil {
		return
	}
	a.Kind = AnchorGuardianResign

	if node.NonceLedger == nil {
		WriteError(w, http.StatusServiceUnavailable, "LEDGER_UNAVAILABLE", "nonce ledger not initialized on this node")
		return
	}

	// Idempotent replay: same nonce already accepted for this
	// pair returns 200 with duplicate flag instead of 400. This
	// matches the pattern set by SubmitAnchorGossipHandler.
	if prev := node.NonceLedger.GuardianResignationNonce(a.GuardianQuid, a.SubjectQuid); prev >= a.ResignationNonce && prev > 0 {
		WriteSuccessWithStatus(w, http.StatusOK, map[string]interface{}{
			"subjectQuid":      a.SubjectQuid,
			"guardianQuid":     a.GuardianQuid,
			"resignationNonce": a.ResignationNonce,
			"duplicate":        true,
		})
		return
	}

	if err := ValidateGuardianResignation(node.NonceLedger, a, time.Now()); err != nil {
		WriteError(w, http.StatusBadRequest, "INVALID_RESIGNATION", err.Error())
		return
	}

	tx := GuardianResignationTransaction{
		BaseTransaction: BaseTransaction{
			Type:      TxTypeGuardianResign,
			Timestamp: time.Now().Unix(),
		},
		Resignation: a,
	}
	node.PendingTxsMutex.Lock()
	node.PendingTxs = append(node.PendingTxs, tx)
	node.PendingTxsMutex.Unlock()

	WriteSuccessWithStatus(w, http.StatusAccepted, map[string]interface{}{
		"subjectQuid":      a.SubjectQuid,
		"guardianQuid":     a.GuardianQuid,
		"resignationNonce": a.ResignationNonce,
		"effectiveAt":      a.EffectiveAt,
	})
}

// GetGuardianResignationsHandler returns the list of
// resignations recorded for a subject. Order is append order
// (stable across restarts because it's replayed from the
// chain).
func (node *QuidnugNode) GetGuardianResignationsHandler(w http.ResponseWriter, r *http.Request) {
	quid := mux.Vars(r)["quid"]
	if node.NonceLedger == nil {
		WriteError(w, http.StatusServiceUnavailable, "LEDGER_UNAVAILABLE", "nonce ledger not initialized")
		return
	}
	resignations := node.NonceLedger.GuardianResignationsOf(quid)
	weakened := node.NonceLedger.GuardianSetIsWeakened(quid, time.Now())
	WriteSuccess(w, map[string]interface{}{
		"subjectQuid":  quid,
		"resignations": resignations,
		"weakened":     weakened,
	})
}
