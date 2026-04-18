// Package core — handlers_guardian.go
//
// HTTP endpoints for operators to submit and query guardian anchors
// (QDP-0002). These live under /api/v2/guardian/ so they're opt-in
// and don't collide with the existing v1 surface.
//
// Submission semantics: a submitted anchor is validated against the
// current ledger state. If valid, it's added to the pending
// transaction pool via node.PendingTxs. Block processing applies it
// once the containing block reaches the Trusted tier — same flow as
// any other AnchorTransaction.
//
// Endpoints use DecodeJSONBody (DisallowUnknownFields) to reject
// silently-extended payloads, matching the project-wide strict
// decoding policy added during the audit.
package core

import (
	"net/http"
	"time"

	"github.com/gorilla/mux"
)

// registerGuardianRoutes mounts the /api/v2/guardian/* handlers.
// Called from registerAPIRoutes.
func (node *QuidnugNode) registerGuardianRoutes(router *mux.Router) {
	// Submit endpoints — each validates, then enqueues the anchor as
	// a pending transaction.
	router.HandleFunc("/guardian/set-update", node.SubmitGuardianSetUpdateHandler).Methods("POST")
	router.HandleFunc("/guardian/recovery/init", node.SubmitGuardianRecoveryInitHandler).Methods("POST")
	router.HandleFunc("/guardian/recovery/veto", node.SubmitGuardianRecoveryVetoHandler).Methods("POST")
	router.HandleFunc("/guardian/recovery/commit", node.SubmitGuardianRecoveryCommitHandler).Methods("POST")
	// QDP-0006: guardian resignation (H6).
	router.HandleFunc("/guardian/resign", node.SubmitGuardianResignationHandler).Methods("POST")

	// Query endpoints — read-only views of guardian state.
	router.HandleFunc("/guardian/set/{quid}", node.GetGuardianSetHandler).Methods("GET")
	router.HandleFunc("/guardian/pending-recovery/{quid}", node.GetPendingRecoveryHandler).Methods("GET")
	router.HandleFunc("/guardian/resignations/{quid}", node.GetGuardianResignationsHandler).Methods("GET")
}

// ----- Submission handlers -------------------------------------------------

// SubmitGuardianSetUpdateHandler accepts a signed GuardianSetUpdate,
// validates it against the current ledger, and enqueues it.
func (node *QuidnugNode) SubmitGuardianSetUpdateHandler(w http.ResponseWriter, r *http.Request) {
	var u GuardianSetUpdate
	if err := DecodeJSONBody(w, r, &u); err != nil {
		return
	}
	u.Kind = AnchorGuardianSetUpdate

	if node.NonceLedger == nil {
		WriteError(w, http.StatusServiceUnavailable, "LEDGER_UNAVAILABLE", "nonce ledger not initialized on this node")
		return
	}
	if err := ValidateGuardianSetUpdate(node.NonceLedger, u, time.Now()); err != nil {
		WriteError(w, http.StatusBadRequest, "INVALID_ANCHOR", err.Error())
		return
	}

	tx := GuardianSetUpdateTransaction{
		BaseTransaction: BaseTransaction{
			Type:      TxTypeGuardianSetUpdate,
			Timestamp: time.Now().Unix(),
		},
		Update: u,
	}
	node.PendingTxsMutex.Lock()
	node.PendingTxs = append(node.PendingTxs, tx)
	node.PendingTxsMutex.Unlock()

	WriteSuccessWithStatus(w, http.StatusAccepted, map[string]interface{}{
		"subjectQuid":     u.SubjectQuid,
		"guardians":       len(u.NewSet.Guardians),
		"threshold":       u.NewSet.Threshold,
		"recoveryDelayNs": u.NewSet.RecoveryDelay.Nanoseconds(),
	})
}

// SubmitGuardianRecoveryInitHandler accepts a signed recovery-init.
func (node *QuidnugNode) SubmitGuardianRecoveryInitHandler(w http.ResponseWriter, r *http.Request) {
	var a GuardianRecoveryInit
	if err := DecodeJSONBody(w, r, &a); err != nil {
		return
	}
	a.Kind = AnchorGuardianRecoveryInit

	if node.NonceLedger == nil {
		WriteError(w, http.StatusServiceUnavailable, "LEDGER_UNAVAILABLE", "nonce ledger not initialized on this node")
		return
	}
	if err := ValidateGuardianRecoveryInit(node.NonceLedger, a, time.Now()); err != nil {
		WriteError(w, http.StatusBadRequest, "INVALID_ANCHOR", err.Error())
		return
	}

	hash, _ := GuardianRecoveryInitHash(a)
	tx := GuardianRecoveryInitTransaction{
		BaseTransaction: BaseTransaction{
			Type:      TxTypeGuardianRecoveryInit,
			Timestamp: time.Now().Unix(),
		},
		Init: a,
	}
	node.PendingTxsMutex.Lock()
	node.PendingTxs = append(node.PendingTxs, tx)
	node.PendingTxsMutex.Unlock()

	WriteSuccessWithStatus(w, http.StatusAccepted, map[string]interface{}{
		"subjectQuid":        a.SubjectQuid,
		"toEpoch":            a.ToEpoch,
		"recoveryAnchorHash": hash,
	})
}

// SubmitGuardianRecoveryVetoHandler accepts a veto for a pending
// recovery.
func (node *QuidnugNode) SubmitGuardianRecoveryVetoHandler(w http.ResponseWriter, r *http.Request) {
	var v GuardianRecoveryVeto
	if err := DecodeJSONBody(w, r, &v); err != nil {
		return
	}
	v.Kind = AnchorGuardianRecoveryVeto

	if node.NonceLedger == nil {
		WriteError(w, http.StatusServiceUnavailable, "LEDGER_UNAVAILABLE", "nonce ledger not initialized on this node")
		return
	}
	if err := ValidateGuardianRecoveryVeto(node.NonceLedger, v, time.Now()); err != nil {
		WriteError(w, http.StatusBadRequest, "INVALID_ANCHOR", err.Error())
		return
	}

	tx := GuardianRecoveryVetoTransaction{
		BaseTransaction: BaseTransaction{
			Type:      TxTypeGuardianRecoveryVeto,
			Timestamp: time.Now().Unix(),
		},
		Veto: v,
	}
	node.PendingTxsMutex.Lock()
	node.PendingTxs = append(node.PendingTxs, tx)
	node.PendingTxsMutex.Unlock()

	WriteSuccessWithStatus(w, http.StatusAccepted, map[string]interface{}{
		"subjectQuid":        v.SubjectQuid,
		"recoveryAnchorHash": v.RecoveryAnchorHash,
	})
}

// SubmitGuardianRecoveryCommitHandler accepts a commit for a mature
// pending recovery.
func (node *QuidnugNode) SubmitGuardianRecoveryCommitHandler(w http.ResponseWriter, r *http.Request) {
	var c GuardianRecoveryCommit
	if err := DecodeJSONBody(w, r, &c); err != nil {
		return
	}
	c.Kind = AnchorGuardianRecoveryCommit

	if node.NonceLedger == nil {
		WriteError(w, http.StatusServiceUnavailable, "LEDGER_UNAVAILABLE", "nonce ledger not initialized on this node")
		return
	}
	if err := ValidateGuardianRecoveryCommit(node.NonceLedger, c, time.Now()); err != nil {
		WriteError(w, http.StatusBadRequest, "INVALID_ANCHOR", err.Error())
		return
	}

	tx := GuardianRecoveryCommitTransaction{
		BaseTransaction: BaseTransaction{
			Type:      TxTypeGuardianRecoveryCommit,
			Timestamp: time.Now().Unix(),
		},
		Commit: c,
	}
	node.PendingTxsMutex.Lock()
	node.PendingTxs = append(node.PendingTxs, tx)
	node.PendingTxsMutex.Unlock()

	WriteSuccessWithStatus(w, http.StatusAccepted, map[string]interface{}{
		"subjectQuid":        c.SubjectQuid,
		"recoveryAnchorHash": c.RecoveryAnchorHash,
	})
}

// ----- Query handlers -----------------------------------------------------

// GetGuardianSetHandler returns the current guardian set for a quid,
// or 404 if no set is declared. Only safe, public fields are emitted;
// the declaration is already on-chain so there's no secrecy at risk.
func (node *QuidnugNode) GetGuardianSetHandler(w http.ResponseWriter, r *http.Request) {
	quid := mux.Vars(r)["quid"]
	if node.NonceLedger == nil {
		WriteError(w, http.StatusServiceUnavailable, "LEDGER_UNAVAILABLE", "nonce ledger not initialized")
		return
	}
	set := node.NonceLedger.GuardianSetOf(quid)
	if set == nil {
		WriteError(w, http.StatusNotFound, "NOT_FOUND", "no guardian set declared for this quid")
		return
	}
	WriteSuccess(w, set)
}

// GetPendingRecoveryHandler returns the pending-recovery record for a
// quid, or 404 if none is pending.
func (node *QuidnugNode) GetPendingRecoveryHandler(w http.ResponseWriter, r *http.Request) {
	quid := mux.Vars(r)["quid"]
	if node.NonceLedger == nil {
		WriteError(w, http.StatusServiceUnavailable, "LEDGER_UNAVAILABLE", "nonce ledger not initialized")
		return
	}
	p := node.NonceLedger.PendingRecoveryOf(quid)
	if p == nil {
		WriteError(w, http.StatusNotFound, "NOT_FOUND", "no pending recovery for this quid")
		return
	}
	WriteSuccess(w, map[string]interface{}{
		"subjectQuid":     quid,
		"initHash":        p.InitHash,
		"initBlockHeight": p.InitBlockHeight,
		"maturityUnix":    p.MaturityUnix,
		"state":           p.State.String(),
		"init":            p.Init,
	})
}
