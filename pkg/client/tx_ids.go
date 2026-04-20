package client

// tx_ids.go — transaction-ID derivation helpers.
//
// Each function here MUST produce byte-identical bytes to
// the equivalent AddXxxTransaction function in
// internal/core/transactions.go. Any drift between client and
// server derivation means the client's ID will not match what
// the server computes on receipt — the server rejects
// pre-computed IDs that don't match its own derivation.
//
// The internal/core/vectors_test.go suite locks these
// derivations in; if any change, the vector tests fail.

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
)

// deriveTrustID mirrors AddTrustTransaction's ID derivation
// in internal/core/transactions.go. The hashed payload is a
// Go-struct-ordered JSON of (Truster, Trustee, TrustLevel,
// TrustDomain, Timestamp).
func deriveTrustID(tx *trustTxWire) string {
	seed, _ := json.Marshal(struct {
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
	sum := sha256.Sum256(seed)
	return hex.EncodeToString(sum[:])
}

// deriveIdentityID mirrors AddIdentityTransaction. Payload:
// (QuidID, Name, Creator, TrustDomain, UpdateNonce, Timestamp).
func deriveIdentityID(tx *identityTxWire) string {
	seed, _ := json.Marshal(struct {
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
	sum := sha256.Sum256(seed)
	return hex.EncodeToString(sum[:])
}

// deriveTitleID mirrors AddTitleTransaction. Payload:
// (AssetID, Owners, TrustDomain, Timestamp).
//
// Owners is serialized as the field value; its ordering
// matters for ID stability — callers should not reorder.
func deriveTitleID(tx *titleTxWire) string {
	seed, _ := json.Marshal(struct {
		AssetID     string
		Owners      []ownershipStakeWire
		TrustDomain string
		Timestamp   int64
	}{
		AssetID:     tx.AssetID,
		Owners:      tx.Owners,
		TrustDomain: tx.TrustDomain,
		Timestamp:   tx.Timestamp,
	})
	sum := sha256.Sum256(seed)
	return hex.EncodeToString(sum[:])
}

// deriveEventID mirrors AddEventTransaction. Payload:
// (SubjectID, EventType, Sequence, TrustDomain, Timestamp).
func deriveEventID(tx *eventTxWire) string {
	seed, _ := json.Marshal(struct {
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
	sum := sha256.Sum256(seed)
	return hex.EncodeToString(sum[:])
}

// deriveModerationActionID mirrors AddModerationActionTransaction.
// Payload: (ModeratorQuid, TargetType, TargetID, Scope,
// ReasonCode, Nonce, Timestamp).
func deriveModerationActionID(tx *moderationActionTxWire) string {
	seed, _ := json.Marshal(struct {
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
	sum := sha256.Sum256(seed)
	return hex.EncodeToString(sum[:])
}

// --- DSR family IDs ------------------------------------------------------
//
// The privacy family in internal/core/transactions.go uses an
// internal helper `addPrivacyTxToPool` that derives IDs from a
// type-specific "seed" struct. The seeds below are copied
// verbatim from the internal/core call sites.

func deriveDSRRequestID(tx *dsrRequestTxWire) string {
	seed, _ := json.Marshal(struct {
		Subject     string
		Controller  string
		RequestType string
		Nonce       int64
		Timestamp   int64
	}{
		Subject:     tx.SubjectQuid,
		Controller:  tx.ControllerQuid,
		RequestType: tx.RequestType,
		Nonce:       tx.Nonce,
		Timestamp:   tx.Timestamp,
	})
	sum := sha256.Sum256(seed)
	return hex.EncodeToString(sum[:])
}

func deriveConsentGrantID(tx *consentGrantTxWire) string {
	seed, _ := json.Marshal(struct {
		Subject    string
		Controller string
		Scope      []string
		PolicyHash string
		Nonce      int64
		Timestamp  int64
	}{
		Subject:    tx.SubjectQuid,
		Controller: tx.ControllerQuid,
		Scope:      tx.Scope,
		PolicyHash: tx.PolicyHash,
		Nonce:      tx.Nonce,
		Timestamp:  tx.Timestamp,
	})
	sum := sha256.Sum256(seed)
	return hex.EncodeToString(sum[:])
}

func deriveConsentWithdrawID(tx *consentWithdrawTxWire) string {
	seed, _ := json.Marshal(struct {
		Subject   string
		Withdraw  string
		Nonce     int64
		Timestamp int64
	}{
		Subject:   tx.SubjectQuid,
		Withdraw:  tx.WithdrawsGrantTxID,
		Nonce:     tx.Nonce,
		Timestamp: tx.Timestamp,
	})
	sum := sha256.Sum256(seed)
	return hex.EncodeToString(sum[:])
}

func deriveProcessingRestrictionID(tx *processingRestrictionTxWire) string {
	seed, _ := json.Marshal(struct {
		Subject   string
		Uses      []string
		Nonce     int64
		Timestamp int64
	}{
		Subject:   tx.SubjectQuid,
		Uses:      tx.RestrictedUses,
		Nonce:     tx.Nonce,
		Timestamp: tx.Timestamp,
	})
	sum := sha256.Sum256(seed)
	return hex.EncodeToString(sum[:])
}

func deriveDSRComplianceID(tx *dsrComplianceTxWire) string {
	seed, _ := json.Marshal(struct {
		RequestTxID string
		Operator    string
		CompletedAt int64
		Nonce       int64
		Timestamp   int64
	}{
		RequestTxID: tx.RequestTxID,
		Operator:    tx.OperatorQuid,
		CompletedAt: tx.CompletedAt,
		Nonce:       tx.Nonce,
		Timestamp:   tx.Timestamp,
	})
	sum := sha256.Sum256(seed)
	return hex.EncodeToString(sum[:])
}
