// Package core. transactions.go — transaction ingestion.
package core

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"time"

	"github.com/quidnug/quidnug/internal/ratelimit"
)

// admitWriteOrReject consults the QDP-0016 multi-layer write
// limiter and returns a Go error describing the denial (if
// any) so mempool-admission functions can fail-fast uniformly.
// Callers that don't want IP-layer enforcement (e.g. writes
// that originate from the server itself, not over HTTP) should
// pass an empty IP string — the IP layer skips empty keys.
//
// If the limiter is nil, the call is a no-op.
func (node *QuidnugNode) admitWriteOrReject(keys ratelimit.ActorKeys) error {
	if node.WriteLimiter == nil {
		return nil
	}
	if got := node.WriteLimiter.AdmitWrite(keys); !got.Allowed {
		RecordRateLimitDenial(string(got.Layer))
		return fmt.Errorf("rate limit exceeded at %s layer", got.Layer)
	}
	return nil
}

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

	// QDP-0016 multi-layer rate limit at mempool admission.
	// Scoped to the signing quid + target domain; IP / operator
	// are not known at this layer so only two layers fire here.
	if err := node.admitWriteOrReject(ratelimit.ActorKeys{
		Quid:   tx.Truster,
		Domain: tx.TrustDomain,
	}); err != nil {
		RecordTransactionProcessed("trust", false)
		return "", err
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

	// QDP-0016 rate-limit. The signer's quid is derived from
	// PublicKey; if the tx is unsigned yet (internal callers)
	// we fall back to charging the domain layer only.
	signerQuid := ""
	if tx.PublicKey != "" {
		signerQuid = QuidIDFromPublicKeyHex(tx.PublicKey)
	}
	if err := node.admitWriteOrReject(ratelimit.ActorKeys{
		Quid:   signerQuid,
		Domain: tx.TrustDomain,
	}); err != nil {
		RecordTransactionProcessed("event", false)
		return "", err
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

// AddNodeAdvertisementTransaction adds a QDP-0014
// NodeAdvertisementTransaction to the pending pool after
// validating it. The tx proves the node's identity + the
// operator's attestation, then becomes discoverable via the
// discovery API once the block including it commits.
func (node *QuidnugNode) AddNodeAdvertisementTransaction(tx NodeAdvertisementTransaction) (string, error) {
	if tx.Timestamp == 0 {
		tx.Timestamp = time.Now().Unix()
	}
	tx.Type = TxTypeNodeAdvertisement

	// Generate transaction ID if not present. Hash the
	// content-relevant fields so honest ID assignment is
	// deterministic but non-colliding across advertisement
	// refreshes.
	if tx.ID == "" {
		txData, _ := json.Marshal(struct {
			NodeQuid           string
			OperatorQuid       string
			TrustDomain        string
			AdvertisementNonce int64
			Timestamp          int64
		}{
			NodeQuid:           tx.NodeQuid,
			OperatorQuid:       tx.OperatorQuid,
			TrustDomain:        tx.TrustDomain,
			AdvertisementNonce: tx.AdvertisementNonce,
			Timestamp:          tx.Timestamp,
		})
		hash := sha256.Sum256(txData)
		tx.ID = hex.EncodeToString(hash[:])
	}

	if !node.ValidateNodeAdvertisementTransaction(tx) {
		RecordTransactionProcessed("node_advertisement", false)
		return "", fmt.Errorf("invalid node advertisement transaction")
	}

	node.PendingTxsMutex.Lock()
	defer node.PendingTxsMutex.Unlock()

	node.PendingTxs = append(node.PendingTxs, tx)
	RecordTransactionProcessed("node_advertisement", true)
	UpdatePendingTransactionsGauge(len(node.PendingTxs))

	go node.BroadcastTransaction(tx)

	logger.Info("Added node advertisement to pending pool",
		"txId", tx.ID,
		"nodeQuid", tx.NodeQuid,
		"operatorQuid", tx.OperatorQuid,
		"endpoints", len(tx.Endpoints),
		"domain", tx.TrustDomain)
	return tx.ID, nil
}

// AddModerationActionTransaction admits a QDP-0015
// ModerationActionTransaction into the pending pool after
// full validation. On acceptance it is broadcast like any
// other signed transaction; once the containing block commits,
// serving-layer filters pick it up via the ModerationRegistry.
//
// Auto-filled fields follow the same signed/unsigned policy as
// AddTrustTransaction: if Signature is non-empty the tx is
// treated as client-signed and we leave Timestamp / Type / ID /
// Nonce exactly as provided. Mutating any of them would silently
// invalidate the signature.
func (node *QuidnugNode) AddModerationActionTransaction(tx ModerationActionTransaction) (string, error) {
	signed := tx.Signature != ""

	if !signed && tx.Timestamp == 0 {
		tx.Timestamp = time.Now().Unix()
	}
	if !signed {
		tx.Type = TxTypeModerationAction
	}

	// Nonce: if not set, use currentNonce+1 for the moderator.
	if !signed && tx.Nonce == 0 && node.ModerationRegistry != nil {
		tx.Nonce = node.ModerationRegistry.currentNonce(tx.ModeratorQuid) + 1
	}

	if !signed && tx.ID == "" {
		txData, _ := json.Marshal(struct {
			ModeratorQuid string
			TargetType    string
			TargetID      string
			Scope         string
			ReasonCode    string
			Nonce         int64
			Timestamp     int64
		}{
			ModeratorQuid: tx.ModeratorQuid,
			TargetType:    tx.TargetType,
			TargetID:      tx.TargetID,
			Scope:         tx.Scope,
			ReasonCode:    tx.ReasonCode,
			Nonce:         tx.Nonce,
			Timestamp:     tx.Timestamp,
		})
		hash := sha256.Sum256(txData)
		tx.ID = hex.EncodeToString(hash[:])
	}

	if err := node.admitWriteOrReject(ratelimit.ActorKeys{
		Quid:   tx.ModeratorQuid,
		Domain: tx.TrustDomain,
	}); err != nil {
		RecordTransactionProcessed("moderation_action", false)
		return "", err
	}

	if !node.ValidateModerationActionTransaction(tx) {
		RecordTransactionProcessed("moderation_action", false)
		return "", fmt.Errorf("invalid moderation action transaction")
	}

	node.PendingTxsMutex.Lock()
	defer node.PendingTxsMutex.Unlock()

	node.PendingTxs = append(node.PendingTxs, tx)
	RecordTransactionProcessed("moderation_action", true)
	UpdatePendingTransactionsGauge(len(node.PendingTxs))

	go node.BroadcastTransaction(tx)

	logger.Info("Added moderation action to pending pool",
		"txId", tx.ID,
		"moderator", tx.ModeratorQuid,
		"targetType", tx.TargetType,
		"targetId", tx.TargetID,
		"scope", tx.Scope,
		"reason", tx.ReasonCode,
		"domain", tx.TrustDomain)

	// QDP-0018: audit log entry. Category matches QDP-0015's
	// cross-reference pattern (on-chain tx id + local operator
	// context).
	node.emitAudit("MODERATION_ACTION", map[string]interface{}{
		"moderation_tx_id": tx.ID,
		"moderator":        tx.ModeratorQuid,
		"target_type":      tx.TargetType,
		"target_id":        tx.TargetID,
		"scope":            tx.Scope,
		"reason_code":      tx.ReasonCode,
		"domain":           tx.TrustDomain,
	}, "moderation action admitted to pending pool")

	return tx.ID, nil
}

// addPrivacyTxToPool is the shared mempool admission path used
// by every QDP-0017 tx type. Calls the caller-supplied
// validator; on success appends to PendingTxs and broadcasts.
// Returns the (possibly just-generated) tx ID.
//
// The mempool stores the dereferenced struct value so
// downstream block serialization matches the other tx types'
// convention.
func (node *QuidnugNode) addPrivacyTxToPool(
	txPtr interface{},
	validate func() bool,
	idSeed []byte,
	txKind string,
) (string, error) {
	if err := ensurePrivacyTxID(txPtr, idSeed); err != nil {
		return "", fmt.Errorf("ensure tx id: %w", err)
	}

	// QDP-0016 rate limit. Extracts actor keys via the same
	// type switch we use for id generation.
	if keys, ok := privacyActorKeys(txPtr); ok {
		if err := node.admitWriteOrReject(keys); err != nil {
			RecordTransactionProcessed(txKind, false)
			return "", err
		}
	}

	if !validate() {
		RecordTransactionProcessed(txKind, false)
		return "", fmt.Errorf("invalid %s transaction", txKind)
	}

	// Dereference the pointer back to its struct value so the
	// mempool entry has the same shape as every other tx type.
	txValue, err := derefPrivacyTx(txPtr)
	if err != nil {
		return "", err
	}

	node.PendingTxsMutex.Lock()
	node.PendingTxs = append(node.PendingTxs, txValue)
	RecordTransactionProcessed(txKind, true)
	UpdatePendingTransactionsGauge(len(node.PendingTxs))
	node.PendingTxsMutex.Unlock()

	go node.BroadcastTransaction(txValue)

	id, _ := extractPrivacyTxID(txPtr)
	logger.Info("Added "+txKind+" to pending pool", "txId", id)

	// QDP-0018: audit hook for every privacy tx. The
	// per-type payload shape is picked by privacyAuditPayload.
	if cat, payload, note := privacyAuditPayload(txPtr); cat != "" {
		node.emitAudit(cat, payload, note)
	}

	return id, nil
}

// privacyAuditPayload selects the right QDP-0018 category +
// payload for each QDP-0017 tx type. Returns an empty category
// when the type has no audit hook (currently all five types
// produce an entry, but the selector keeps the shape future-
// proof).
func privacyAuditPayload(txPtr interface{}) (category string, payload map[string]interface{}, note string) {
	switch v := txPtr.(type) {
	case *DataSubjectRequestTransaction:
		return "DSR_FULFILLMENT", map[string]interface{}{
			"request_tx_id": v.ID,
			"subject":       v.SubjectQuid,
			"request_type":  v.RequestType,
			"jurisdiction":  v.Jurisdiction,
			"phase":         "received",
		}, "DSR request admitted"
	case *ConsentGrantTransaction:
		return "DSR_FULFILLMENT", map[string]interface{}{
			"event":         "consent_grant",
			"grant_tx_id":   v.ID,
			"subject":       v.SubjectQuid,
			"controller":    v.ControllerQuid,
			"scope":         v.Scope,
			"policy_hash":   v.PolicyHash,
		}, "consent grant admitted"
	case *ConsentWithdrawTransaction:
		return "DSR_FULFILLMENT", map[string]interface{}{
			"event":                "consent_withdraw",
			"withdraw_tx_id":       v.ID,
			"subject":              v.SubjectQuid,
			"withdraws_grant_tx_id": v.WithdrawsGrantTxID,
		}, "consent withdraw admitted"
	case *ProcessingRestrictionTransaction:
		return "DSR_FULFILLMENT", map[string]interface{}{
			"event":            "processing_restriction",
			"restriction_tx_id": v.ID,
			"subject":          v.SubjectQuid,
			"restricted_uses":  v.RestrictedUses,
		}, "processing restriction admitted"
	case *DSRComplianceTransaction:
		return "DSR_FULFILLMENT", map[string]interface{}{
			"event":             "dsr_compliance",
			"compliance_tx_id":  v.ID,
			"request_tx_id":     v.RequestTxID,
			"operator":          v.OperatorQuid,
			"request_type":      v.RequestType,
			"completed_at":      v.CompletedAt,
			"actions_category":  v.ActionsCategory,
			"carve_outs_applied": v.CarveOutsApplied,
		}, "DSR compliance record published"
	}
	return "", nil, ""
}

// privacyActorKeys pulls (quid, domain) out of a privacy tx
// pointer for rate-limit charging. For operator-signed
// DSR_COMPLIANCE the OperatorQuid is used; every other privacy
// tx charges the subject.
func privacyActorKeys(txPtr interface{}) (ratelimit.ActorKeys, bool) {
	switch v := txPtr.(type) {
	case *DataSubjectRequestTransaction:
		return ratelimit.ActorKeys{Quid: v.SubjectQuid, Domain: v.TrustDomain}, true
	case *ConsentGrantTransaction:
		return ratelimit.ActorKeys{Quid: v.SubjectQuid, Domain: v.TrustDomain}, true
	case *ConsentWithdrawTransaction:
		return ratelimit.ActorKeys{Quid: v.SubjectQuid, Domain: v.TrustDomain}, true
	case *ProcessingRestrictionTransaction:
		return ratelimit.ActorKeys{Quid: v.SubjectQuid, Domain: v.TrustDomain}, true
	case *DSRComplianceTransaction:
		return ratelimit.ActorKeys{Quid: v.OperatorQuid, Operator: v.OperatorQuid, Domain: v.TrustDomain}, true
	}
	return ratelimit.ActorKeys{}, false
}

// derefPrivacyTx unwraps a *PrivacyTx pointer to its value.
func derefPrivacyTx(txPtr interface{}) (interface{}, error) {
	switch v := txPtr.(type) {
	case *DataSubjectRequestTransaction:
		return *v, nil
	case *ConsentGrantTransaction:
		return *v, nil
	case *ConsentWithdrawTransaction:
		return *v, nil
	case *ProcessingRestrictionTransaction:
		return *v, nil
	case *DSRComplianceTransaction:
		return *v, nil
	}
	return nil, fmt.Errorf("unknown privacy tx type %T", txPtr)
}

// ensurePrivacyTxID writes a sha256-derived ID into tx.ID if
// the tx is unsigned and has no ID yet. Signed transactions are
// left alone — mutating the ID on a signed struct would silently
// break signature verification because the validator re-marshals
// the whole struct (ID included) to recompute the signable bytes.
func ensurePrivacyTxID(txPtr interface{}, idSeed []byte) error {
	switch v := txPtr.(type) {
	case *DataSubjectRequestTransaction:
		if v.ID == "" && v.Signature == "" {
			v.ID = hashPrivacyID(idSeed)
		}
	case *ConsentGrantTransaction:
		if v.ID == "" && v.Signature == "" {
			v.ID = hashPrivacyID(idSeed)
		}
	case *ConsentWithdrawTransaction:
		if v.ID == "" && v.Signature == "" {
			v.ID = hashPrivacyID(idSeed)
		}
	case *ProcessingRestrictionTransaction:
		if v.ID == "" && v.Signature == "" {
			v.ID = hashPrivacyID(idSeed)
		}
	case *DSRComplianceTransaction:
		if v.ID == "" && v.Signature == "" {
			v.ID = hashPrivacyID(idSeed)
		}
	default:
		return fmt.Errorf("unknown privacy tx type %T", txPtr)
	}
	return nil
}

func extractPrivacyTxID(txPtr interface{}) (string, error) {
	switch v := txPtr.(type) {
	case *DataSubjectRequestTransaction:
		return v.ID, nil
	case *ConsentGrantTransaction:
		return v.ID, nil
	case *ConsentWithdrawTransaction:
		return v.ID, nil
	case *ProcessingRestrictionTransaction:
		return v.ID, nil
	case *DSRComplianceTransaction:
		return v.ID, nil
	}
	return "", fmt.Errorf("unknown privacy tx type %T", txPtr)
}

func hashPrivacyID(seed []byte) string {
	h := sha256.Sum256(seed)
	return hex.EncodeToString(h[:])
}

// AddDataSubjectRequestTransaction admits a DSR into the pending
// pool after validation.
func (node *QuidnugNode) AddDataSubjectRequestTransaction(tx DataSubjectRequestTransaction) (string, error) {
	signed := tx.Signature != ""
	if !signed && tx.Timestamp == 0 {
		tx.Timestamp = time.Now().Unix()
	}
	if !signed {
		tx.Type = TxTypeDataSubjectRequest
	}
	if !signed && tx.Nonce == 0 && node.PrivacyRegistry != nil {
		tx.Nonce = node.PrivacyRegistry.currentNonce(tx.SubjectQuid, TxTypeDataSubjectRequest) + 1
	}
	seed, _ := json.Marshal(struct {
		Subject     string
		Controller  string
		RequestType string
		Nonce       int64
		Timestamp   int64
	}{tx.SubjectQuid, tx.ControllerQuid, tx.RequestType, tx.Nonce, tx.Timestamp})
	return node.addPrivacyTxToPool(&tx, func() bool {
		return node.ValidateDataSubjectRequestTransaction(tx)
	}, seed, "data_subject_request")
}

// AddConsentGrantTransaction admits a CONSENT_GRANT.
func (node *QuidnugNode) AddConsentGrantTransaction(tx ConsentGrantTransaction) (string, error) {
	signed := tx.Signature != ""
	if !signed && tx.Timestamp == 0 {
		tx.Timestamp = time.Now().Unix()
	}
	if !signed {
		tx.Type = TxTypeConsentGrant
	}
	if !signed && tx.Nonce == 0 && node.PrivacyRegistry != nil {
		tx.Nonce = node.PrivacyRegistry.currentNonce(tx.SubjectQuid, TxTypeConsentGrant) + 1
	}
	seed, _ := json.Marshal(struct {
		Subject    string
		Controller string
		Scope      []string
		PolicyHash string
		Nonce      int64
		Timestamp  int64
	}{tx.SubjectQuid, tx.ControllerQuid, tx.Scope, tx.PolicyHash, tx.Nonce, tx.Timestamp})
	return node.addPrivacyTxToPool(&tx, func() bool {
		return node.ValidateConsentGrantTransaction(tx)
	}, seed, "consent_grant")
}

// AddConsentWithdrawTransaction admits a CONSENT_WITHDRAW.
func (node *QuidnugNode) AddConsentWithdrawTransaction(tx ConsentWithdrawTransaction) (string, error) {
	signed := tx.Signature != ""
	if !signed && tx.Timestamp == 0 {
		tx.Timestamp = time.Now().Unix()
	}
	if !signed {
		tx.Type = TxTypeConsentWithdraw
	}
	if !signed && tx.Nonce == 0 && node.PrivacyRegistry != nil {
		tx.Nonce = node.PrivacyRegistry.currentNonce(tx.SubjectQuid, TxTypeConsentWithdraw) + 1
	}
	seed, _ := json.Marshal(struct {
		Subject  string
		Withdraw string
		Nonce    int64
		Timestamp int64
	}{tx.SubjectQuid, tx.WithdrawsGrantTxID, tx.Nonce, tx.Timestamp})
	return node.addPrivacyTxToPool(&tx, func() bool {
		return node.ValidateConsentWithdrawTransaction(tx)
	}, seed, "consent_withdraw")
}

// AddProcessingRestrictionTransaction admits a
// PROCESSING_RESTRICTION.
func (node *QuidnugNode) AddProcessingRestrictionTransaction(tx ProcessingRestrictionTransaction) (string, error) {
	signed := tx.Signature != ""
	if !signed && tx.Timestamp == 0 {
		tx.Timestamp = time.Now().Unix()
	}
	if !signed {
		tx.Type = TxTypeProcessingRestriction
	}
	if !signed && tx.Nonce == 0 && node.PrivacyRegistry != nil {
		tx.Nonce = node.PrivacyRegistry.currentNonce(tx.SubjectQuid, TxTypeProcessingRestriction) + 1
	}
	seed, _ := json.Marshal(struct {
		Subject   string
		Uses      []string
		Nonce     int64
		Timestamp int64
	}{tx.SubjectQuid, tx.RestrictedUses, tx.Nonce, tx.Timestamp})
	return node.addPrivacyTxToPool(&tx, func() bool {
		return node.ValidateProcessingRestrictionTransaction(tx)
	}, seed, "processing_restriction")
}

// AddDSRComplianceTransaction admits an operator-signed
// DSR_COMPLIANCE record.
func (node *QuidnugNode) AddDSRComplianceTransaction(tx DSRComplianceTransaction) (string, error) {
	signed := tx.Signature != ""
	if !signed && tx.Timestamp == 0 {
		tx.Timestamp = time.Now().Unix()
	}
	if !signed {
		tx.Type = TxTypeDSRCompliance
	}
	if !signed && tx.Nonce == 0 && node.PrivacyRegistry != nil {
		tx.Nonce = node.PrivacyRegistry.currentNonce(tx.OperatorQuid, TxTypeDSRCompliance) + 1
	}
	seed, _ := json.Marshal(struct {
		RequestTxID  string
		Operator     string
		CompletedAt  int64
		Nonce        int64
		Timestamp    int64
	}{tx.RequestTxID, tx.OperatorQuid, tx.CompletedAt, tx.Nonce, tx.Timestamp})
	return node.addPrivacyTxToPool(&tx, func() bool {
		return node.ValidateDSRComplianceTransaction(tx)
	}, seed, "dsr_compliance")
}

// FilterTransactionsForBlock filters pending transactions based on trust.
// Only includes transactions from sources the node trusts (or sponsored transactions).
// For each transaction, extracts the creator quid and computes relational trust.
