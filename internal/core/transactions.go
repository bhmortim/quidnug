// Package core. transactions.go — transaction ingestion.
package core

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"time"
)

func (node *QuidnugNode) AddTrustTransaction(tx TrustTransaction) (string, error) {
	// Auto-fill of Timestamp / Type / Nonce is a test and server-side
	// convenience. If the transaction is already signed, these fields
	// are part of the signable data and mutating them would silently
	// invalidate the signature — so we leave signed transactions
	// exactly as the client provided them. An unsigned tx may still be
	// coming from an internal caller or a pre-signing pipeline; we
	// help it out.
	signed := tx.Signature != ""

	if !signed && tx.Timestamp == 0 {
		tx.Timestamp = time.Now().Unix()
	}
	if !signed {
		tx.Type = TxTypeTrust
	}

	// Set nonce if not provided (use current highest nonce + 1 for this truster-trustee pair)
	if !signed && tx.Nonce == 0 {
		node.TrustRegistryMutex.RLock()
		currentNonce := int64(0)
		if trusterNonces, exists := node.TrustNonceRegistry[tx.Truster]; exists {
			if nonce, found := trusterNonces[tx.Trustee]; found {
				currentNonce = nonce
			}
		}
		node.TrustRegistryMutex.RUnlock()
		tx.Nonce = currentNonce + 1
	}

	// Generate transaction ID if not present. Same signed/unsigned
	// rationale as Timestamp/Type/Nonce above: mutating the ID on a
	// signed transaction would silently break signature verification,
	// because ValidateTrustTransaction re-marshals the struct to check
	// the signature.
	if !signed && tx.ID == "" {
		txData, _ := json.Marshal(struct {
			Truster     string
			Trustee     string
			TrustLevel  float64
			TrustDomain string
			Timestamp   int64
		}{
			Truster:     tx.Truster,
			Trustee:     tx.Trustee,
			TrustLevel:  tx.TrustLevel,
			TrustDomain: tx.TrustDomain,
			Timestamp:   tx.Timestamp,
		})

		hash := sha256.Sum256(txData)
		tx.ID = hex.EncodeToString(hash[:])
	}

	// Validate the transaction
	if !node.ValidateTrustTransaction(tx) {
		RecordTransactionProcessed("trust", false)
		return "", fmt.Errorf("invalid trust transaction")
	}

	// QDP-0001 nonce-ledger check. In shadow mode (enforce=false) we
	// count the would-be rejection and proceed. In enforcement mode we
	// reject. Either way the result is recorded for observability.
	if node.NonceLedger != nil {
		key := NonceKey{Quid: tx.Truster, Domain: tx.TrustDomain, Epoch: 0}
		if err := node.NonceLedger.Admit(key, tx.Nonce); err != nil {
			nonceReplayRejections.WithLabelValues(nonceRejectionReason(err), fmt.Sprintf("%t", node.NonceLedgerEnforce)).Inc()
			if node.NonceLedgerEnforce {
				RecordTransactionProcessed("trust", false)
				return "", fmt.Errorf("nonce ledger rejected transaction: %w", err)
			}
			logger.Warn("Nonce ledger would reject trust transaction (shadow mode)",
				"txId", tx.ID, "truster", tx.Truster, "domain", tx.TrustDomain,
				"nonce", tx.Nonce, "reason", err)
		}
	}

	node.PendingTxsMutex.Lock()
	defer node.PendingTxsMutex.Unlock()

	// Add transaction to pending pool
	node.PendingTxs = append(node.PendingTxs, tx)

	// Reserve the nonce tentatively so concurrent submissions can't
	// collide on it (QDP-0001 §6.2). Released if the tx is ultimately
	// pruned from a demoted tentative block.
	if node.NonceLedger != nil {
		node.NonceLedger.ReserveTentative(
			NonceKey{Quid: tx.Truster, Domain: tx.TrustDomain, Epoch: 0},
			tx.Nonce,
		)
	}

	// Record metrics
	RecordTransactionProcessed("trust", true)
	UpdatePendingTransactionsGauge(len(node.PendingTxs))

	// Broadcast to other nodes in the same trust domain
	go node.BroadcastTransaction(tx)

	logger.Info("Added trust transaction to pending pool", "txId", tx.ID, "domain", tx.TrustDomain)
	return tx.ID, nil
}

// AddIdentityTransaction adds an identity transaction to the pending pool
func (node *QuidnugNode) AddIdentityTransaction(tx IdentityTransaction) (string, error) {
	// Set timestamp if not set
	if tx.Timestamp == 0 {
		tx.Timestamp = time.Now().Unix()
	}

	// Set type
	tx.Type = TxTypeIdentity

	// Generate transaction ID if not present
	if tx.ID == "" {
		txData, _ := json.Marshal(struct {
			QuidID      string
			Name        string
			Creator     string
			TrustDomain string
			UpdateNonce int64
			Timestamp   int64
		}{
			QuidID:      tx.QuidID,
			Name:        tx.Name,
			Creator:     tx.Creator,
			TrustDomain: tx.TrustDomain,
			UpdateNonce: tx.UpdateNonce,
			Timestamp:   tx.Timestamp,
		})

		hash := sha256.Sum256(txData)
		tx.ID = hex.EncodeToString(hash[:])
	}

	// Validate the transaction
	if !node.ValidateIdentityTransaction(tx) {
		RecordTransactionProcessed("identity", false)
		return "", fmt.Errorf("invalid identity transaction")
	}

	node.PendingTxsMutex.Lock()
	defer node.PendingTxsMutex.Unlock()

	// Add transaction to pending pool
	node.PendingTxs = append(node.PendingTxs, tx)

	// Record metrics
	RecordTransactionProcessed("identity", true)
	UpdatePendingTransactionsGauge(len(node.PendingTxs))

	// Broadcast to other nodes in the same trust domain
	go node.BroadcastTransaction(tx)

	logger.Info("Added identity transaction to pending pool", "txId", tx.ID, "quidId", tx.QuidID, "domain", tx.TrustDomain)
	return tx.ID, nil
}

// AddEventTransaction adds an event transaction to the pending pool
func (node *QuidnugNode) AddEventTransaction(tx EventTransaction) (string, error) {
	// Set timestamp if not set
	if tx.Timestamp == 0 {
		tx.Timestamp = time.Now().Unix()
	}

	// Set type
	tx.Type = TxTypeEvent

	// Auto-assign sequence if not provided (latest + 1)
	if tx.Sequence == 0 {
		node.EventStreamMutex.RLock()
		events, exists := node.EventRegistry[tx.SubjectID]
		if exists && len(events) > 0 {
			tx.Sequence = events[len(events)-1].Sequence + 1
		} else {
			tx.Sequence = 1
		}
		node.EventStreamMutex.RUnlock()
	}

	// Generate transaction ID if not present
	if tx.ID == "" {
		txData, _ := json.Marshal(struct {
			SubjectID   string
			EventType   string
			Sequence    int64
			TrustDomain string
			Timestamp   int64
		}{
			SubjectID:   tx.SubjectID,
			EventType:   tx.EventType,
			Sequence:    tx.Sequence,
			TrustDomain: tx.TrustDomain,
			Timestamp:   tx.Timestamp,
		})

		hash := sha256.Sum256(txData)
		tx.ID = hex.EncodeToString(hash[:])
	}

	// Validate the transaction
	if !node.ValidateEventTransaction(tx) {
		RecordTransactionProcessed("event", false)
		return "", fmt.Errorf("invalid event transaction")
	}

	node.PendingTxsMutex.Lock()
	defer node.PendingTxsMutex.Unlock()

	// Add transaction to pending pool
	node.PendingTxs = append(node.PendingTxs, tx)

	// Record metrics
	RecordTransactionProcessed("event", true)
	UpdatePendingTransactionsGauge(len(node.PendingTxs))

	// Broadcast to other nodes in the same trust domain
	go node.BroadcastTransaction(tx)

	logger.Info("Added event transaction to pending pool", "txId", tx.ID, "subjectId", tx.SubjectID, "domain", tx.TrustDomain)
	return tx.ID, nil
}

// AddTitleTransaction adds a title transaction to the pending pool
func (node *QuidnugNode) AddTitleTransaction(tx TitleTransaction) (string, error) {
	// Set timestamp if not set
	if tx.Timestamp == 0 {
		tx.Timestamp = time.Now().Unix()
	}

	// Set type
	tx.Type = TxTypeTitle

	// Generate transaction ID if not present
	if tx.ID == "" {
		txData, _ := json.Marshal(struct {
			AssetID     string
			Owners      []OwnershipStake
			TrustDomain string
			Timestamp   int64
		}{
			AssetID:     tx.AssetID,
			Owners:      tx.Owners,
			TrustDomain: tx.TrustDomain,
			Timestamp:   tx.Timestamp,
		})

		hash := sha256.Sum256(txData)
		tx.ID = hex.EncodeToString(hash[:])
	}

	// Validate the transaction
	if !node.ValidateTitleTransaction(tx) {
		RecordTransactionProcessed("title", false)
		return "", fmt.Errorf("invalid title transaction")
	}

	node.PendingTxsMutex.Lock()
	defer node.PendingTxsMutex.Unlock()

	// Add transaction to pending pool
	node.PendingTxs = append(node.PendingTxs, tx)

	// Record metrics
	RecordTransactionProcessed("title", true)
	UpdatePendingTransactionsGauge(len(node.PendingTxs))

	// Broadcast to other nodes in the same trust domain
	go node.BroadcastTransaction(tx)

	logger.Info("Added title transaction to pending pool", "txId", tx.ID, "assetId", tx.AssetID, "domain", tx.TrustDomain)
	return tx.ID, nil
}

// FilterTransactionsForBlock filters pending transactions based on trust.
// Only includes transactions from sources the node trusts (or sponsored transactions).
// For each transaction, extracts the creator quid and computes relational trust.
