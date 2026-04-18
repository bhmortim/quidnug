// Package core — handlers_fork_block.go
//
// HTTP surface for fork-block transactions (QDP-0009 / H5).
package core

import (
	"net/http"
	"time"

	"github.com/gorilla/mux"
)

// registerForkBlockRoutes mounts the /api/v2/fork-block/*
// endpoints. Called from registerCrossDomainRoutes.
func (node *QuidnugNode) registerForkBlockRoutes(router *mux.Router) {
	router.HandleFunc("/fork-block", node.SubmitForkBlockHandler).Methods("POST")
	router.HandleFunc("/fork-block/status", node.GetForkBlockStatusHandler).Methods("GET")
}

// SubmitForkBlockHandler accepts a signed ForkBlock. Validates
// against the current head and enqueues as a pending tx.
//
// Responses:
//   - 202 Accepted — validated and enqueued.
//   - 400 Bad Request — validation failed.
//   - 503 Service Unavailable — node not initialized.
func (node *QuidnugNode) SubmitForkBlockHandler(w http.ResponseWriter, r *http.Request) {
	var f ForkBlock
	if err := DecodeJSONBody(w, r, &f); err != nil {
		return
	}
	f.Kind = AnchorForkBlock
	if node.NonceLedger == nil {
		WriteError(w, http.StatusServiceUnavailable, "LEDGER_UNAVAILABLE", "nonce ledger not initialized")
		return
	}

	// Use current chain head of the domain for the validation
	// currentHeight parameter.
	node.BlockchainMutex.RLock()
	var currentHeight int64
	for i := len(node.Blockchain) - 1; i >= 0; i-- {
		if node.Blockchain[i].TrustProof.TrustDomain == f.TrustDomain {
			currentHeight = node.Blockchain[i].Index
			break
		}
	}
	node.BlockchainMutex.RUnlock()

	if err := node.ValidateForkBlock(f, currentHeight, time.Now()); err != nil {
		WriteError(w, http.StatusBadRequest, "INVALID_FORK_BLOCK", err.Error())
		return
	}

	tx := ForkBlockTransaction{
		BaseTransaction: BaseTransaction{
			Type:      TxTypeForkBlock,
			Timestamp: time.Now().Unix(),
		},
		Fork: f,
	}
	node.PendingTxsMutex.Lock()
	node.PendingTxs = append(node.PendingTxs, tx)
	node.PendingTxsMutex.Unlock()

	WriteSuccessWithStatus(w, http.StatusAccepted, map[string]interface{}{
		"feature":    f.Feature,
		"domain":     f.TrustDomain,
		"forkHeight": f.ForkHeight,
		"forkNonce":  f.ForkNonce,
	})
}

// GetForkBlockStatusHandler returns current pending + active
// forks for operator visibility.
func (node *QuidnugNode) GetForkBlockStatusHandler(w http.ResponseWriter, r *http.Request) {
	if node.forks == nil {
		node.forks = newForkRegistry()
	}
	pending, active := node.forks.snapshot()
	WriteSuccess(w, map[string]interface{}{
		"pending": pending,
		"active":  active,
	})
}
