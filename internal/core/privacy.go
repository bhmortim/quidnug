// Package core — QDP-0017 Data Subject Rights & Privacy.
//
// Phase 1 scope (matches QDP-0017 §10):
//
//   - Transaction types: DATA_SUBJECT_REQUEST, CONSENT_GRANT,
//     CONSENT_WITHDRAW, PROCESSING_RESTRICTION, DSR_COMPLIANCE.
//   - Validators for each, with jurisdiction + enum checks.
//   - Consent / restriction / DSR registries on the node.
//   - Read-side helpers the serving layer uses to decide
//     "can I process this subject's data for purpose X?"
//
// Phase 2 (CLI + operator handlers), Phase 3 (manifest
// generation + erasure helpers), and Phase 4-5 (docs +
// transparency reporting) layer on top of this file.
package core

import (
	"encoding/json"
	"fmt"
	"strings"
	"sync"
)

// Request types from QDP-0017 §4 / §7.
const (
	DSRTypeAccess         = "ACCESS"
	DSRTypeRectification  = "RECTIFICATION"
	DSRTypeErasure        = "ERASURE"
	DSRTypeRestriction    = "RESTRICTION"
	DSRTypePortability    = "PORTABILITY"
	DSRTypeObjection      = "OBJECTION"
)

// Consent scope enum from §5.2. Operators may publish custom
// scopes with the CUSTOM: prefix.
const (
	ConsentScopeProfileBuilding          = "PROFILE_BUILDING"
	ConsentScopeRecommendationComputation = "RECOMMENDATION_COMPUTATION"
	ConsentScopeThirdPartyAnalytics      = "THIRD_PARTY_ANALYTICS"
	ConsentScopeMarketingEmail           = "MARKETING_EMAIL"
	ConsentScopeFederationExport         = "FEDERATION_EXPORT"
	ConsentScopeAITraining               = "AI_TRAINING"
	ConsentScopeCustomPrefix             = "CUSTOM:"
)

// Processing restriction uses. Mirrors the consent scope enum;
// the two are intentionally isomorphic so operators can reason
// about them symmetrically ("no consent to X" ≈ "restricted
// from X").
var validRestrictionUses = map[string]struct{}{
	"reputation-computation":      {},
	"recommendation-aggregation":  {},
	"federation-export":           {},
	"profile-building":            {},
	"third-party-analytics":       {},
	"marketing":                   {},
	"ai-training":                 {},
}

// validDSRTypes is the exhaustive enum of acceptable DSR
// request types. Mirrors the design doc §4 rights list.
var validDSRTypes = map[string]struct{}{
	DSRTypeAccess:        {},
	DSRTypeRectification: {},
	DSRTypeErasure:       {},
	DSRTypeRestriction:   {},
	DSRTypePortability:   {},
	DSRTypeObjection:     {},
}

// validConsentScopes is the well-known scope set; anything not
// in this map must start with the CUSTOM: prefix to be accepted.
var validConsentScopes = map[string]struct{}{
	ConsentScopeProfileBuilding:           {},
	ConsentScopeRecommendationComputation: {},
	ConsentScopeThirdPartyAnalytics:       {},
	ConsentScopeMarketingEmail:            {},
	ConsentScopeFederationExport:          {},
	ConsentScopeAITraining:                {},
}

// MaxRequestDetailsLength caps the free-form rationale field on
// a DATA_SUBJECT_REQUEST. Bounded to prevent chain bloat.
const MaxRequestDetailsLength = 4096

// MaxPolicyHashLength bounds the policy-hash field; in practice
// this is a 64-char hex sha256, but we cap defensively.
const MaxPolicyHashLength = 128

// ---- transaction types ------------------------------------------------

// DataSubjectRequestTransaction is a signed request from a
// subject exercising one of the six data rights from QDP-0017
// §4. Operator's DSR workflow consumes the tx and publishes
// DSR_COMPLIANCE once done.
type DataSubjectRequestTransaction struct {
	BaseTransaction

	SubjectQuid  string `json:"subjectQuid"`
	ControllerQuid string `json:"controllerQuid,omitempty"` // which operator to serve the request

	RequestType    string `json:"requestType"`    // DSRType* enum
	RequestDetails string `json:"requestDetails,omitempty"` // free-form
	ContactEmail   string `json:"contactEmail,omitempty"`

	// Jurisdiction hint — purely advisory. Lets operators route
	// GDPR / CCPA / LGPD requests through the right workflow.
	// Two-letter ISO-3166 country codes or well-known regulator
	// tags (EU, CA, BR, etc.).
	Jurisdiction string `json:"jurisdiction,omitempty"`

	Nonce int64 `json:"nonce"`
}

// ConsentGrantTransaction records a data subject's opt-in to
// specific processing scopes against a specific policy version
// (PolicyHash pins the policy text at consent time).
type ConsentGrantTransaction struct {
	BaseTransaction

	SubjectQuid    string   `json:"subjectQuid"`
	ControllerQuid string   `json:"controllerQuid"`
	Scope          []string `json:"scope"`
	PolicyURL      string   `json:"policyUrl,omitempty"`
	PolicyHash     string   `json:"policyHash,omitempty"`

	// EffectiveUntil is a Unix-seconds expiry; zero means
	// "until withdrawn." QDP-0022 TTL applies symmetrically.
	EffectiveUntil int64 `json:"effectiveUntil,omitempty"`

	Nonce int64 `json:"nonce"`
}

// ConsentWithdrawTransaction revokes a prior CONSENT_GRANT.
// Honored by the serving layer immediately.
type ConsentWithdrawTransaction struct {
	BaseTransaction

	SubjectQuid        string `json:"subjectQuid"`
	WithdrawsGrantTxID string `json:"withdrawsGrantTxId"`
	Reason             string `json:"reason,omitempty"`
	Nonce              int64  `json:"nonce"`
}

// ProcessingRestrictionTransaction narrows specific uses of a
// subject's data without deleting the underlying records. Used
// for the GDPR Art. 18 right to restrict processing.
type ProcessingRestrictionTransaction struct {
	BaseTransaction

	SubjectQuid    string   `json:"subjectQuid"`
	ControllerQuid string   `json:"controllerQuid,omitempty"`
	RestrictedUses []string `json:"restrictedUses"` // validRestrictionUses enum
	Reason         string   `json:"reason,omitempty"`

	// EffectiveUntil applies the same TTL semantics as on
	// CONSENT_GRANT. Zero == indefinite restriction.
	EffectiveUntil int64 `json:"effectiveUntil,omitempty"`

	Nonce int64 `json:"nonce"`
}

// DSRComplianceTransaction is an operator-signed attestation
// that a DSR request was fulfilled. Published after the
// operator completes the workflow (manifest generated, erasure
// actions published, etc.).
type DSRComplianceTransaction struct {
	BaseTransaction

	RequestTxID      string   `json:"requestTxId"`
	RequestType      string   `json:"requestType"`
	OperatorQuid     string   `json:"operatorQuid"`
	CompletedAt      int64    `json:"completedAt"` // Unix seconds
	ActionsCategory  string   `json:"actionsCategory"` // e.g., "manifest-generated"
	CarveOutsApplied []string `json:"carveOutsApplied,omitempty"`
	// ManifestURL is an internal-only link to the generated
	// manifest; may return 403 to non-authenticated audit calls.
	ManifestURL string `json:"manifestUrl,omitempty"`

	Nonce int64 `json:"nonce"`
}

// ---- registry ---------------------------------------------------------

// PrivacyRegistry tracks every consent / restriction / DSR
// record the node has accepted. Shared lock; this is a low-
// frequency structure (most workloads don't churn these).
type PrivacyRegistry struct {
	mu sync.RWMutex

	// consentGrants indexed by grant tx ID. Lookup path:
	// CONSENT_WITHDRAW carries the grant tx ID so we can mark
	// withdrawn grants without scanning.
	consentGrants map[string]ConsentGrantTransaction

	// withdrawnGrants marks grant tx IDs that have been
	// explicitly revoked.
	withdrawnGrants map[string]struct{}

	// activeRestrictions indexed by subject quid. Multiple
	// restrictions may accumulate; the union of RestrictedUses
	// across non-expired entries defines the effective
	// restriction set.
	activeRestrictions map[string][]ProcessingRestrictionTransaction

	// dsrRequests indexed by request tx ID.
	dsrRequests map[string]DataSubjectRequestTransaction

	// dsrCompletions indexed by the request tx ID they resolve.
	dsrCompletions map[string]DSRComplianceTransaction

	// nonces tracks strict monotonicity per (subject, txType)
	// so replays and out-of-order submissions are rejected.
	nonces map[string]int64
}

// NewPrivacyRegistry constructs an empty registry.
func NewPrivacyRegistry() *PrivacyRegistry {
	return &PrivacyRegistry{
		consentGrants:      make(map[string]ConsentGrantTransaction),
		withdrawnGrants:    make(map[string]struct{}),
		activeRestrictions: make(map[string][]ProcessingRestrictionTransaction),
		dsrRequests:        make(map[string]DataSubjectRequestTransaction),
		dsrCompletions:     make(map[string]DSRComplianceTransaction),
		nonces:             make(map[string]int64),
	}
}

// nonceKey produces a stable key for per-(subject, type) nonce
// tracking. Moved to its own function so callers read cleanly.
func nonceKey(subjectQuid string, txType TransactionType) string {
	return string(txType) + ":" + subjectQuid
}

func (r *PrivacyRegistry) currentNonce(subjectQuid string, txType TransactionType) int64 {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.nonces[nonceKey(subjectQuid, txType)]
}

func (r *PrivacyRegistry) recordGrant(tx ConsentGrantTransaction) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.consentGrants[tx.ID] = tx
	r.bumpNonceLocked(tx.SubjectQuid, TxTypeConsentGrant, tx.Nonce)
}

func (r *PrivacyRegistry) recordWithdraw(tx ConsentWithdrawTransaction) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.withdrawnGrants[tx.WithdrawsGrantTxID] = struct{}{}
	r.bumpNonceLocked(tx.SubjectQuid, TxTypeConsentWithdraw, tx.Nonce)
}

func (r *PrivacyRegistry) recordRestriction(tx ProcessingRestrictionTransaction) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.activeRestrictions[tx.SubjectQuid] = append(r.activeRestrictions[tx.SubjectQuid], tx)
	r.bumpNonceLocked(tx.SubjectQuid, TxTypeProcessingRestriction, tx.Nonce)
}

func (r *PrivacyRegistry) recordDSR(tx DataSubjectRequestTransaction) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.dsrRequests[tx.ID] = tx
	r.bumpNonceLocked(tx.SubjectQuid, TxTypeDataSubjectRequest, tx.Nonce)
}

func (r *PrivacyRegistry) recordCompliance(tx DSRComplianceTransaction) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.dsrCompletions[tx.RequestTxID] = tx
	// Operator nonce tracking lives separately — keyed by
	// operator quid + DSR_COMPLIANCE.
	r.bumpNonceLocked(tx.OperatorQuid, TxTypeDSRCompliance, tx.Nonce)
}

// bumpNonceLocked updates the per-(subject, type) nonce high-
// water mark. Caller holds the write lock.
func (r *PrivacyRegistry) bumpNonceLocked(subjectQuid string, txType TransactionType, nonce int64) {
	k := nonceKey(subjectQuid, txType)
	if nonce > r.nonces[k] {
		r.nonces[k] = nonce
	}
}

// ---- read-side helpers ------------------------------------------------

// HasActiveConsent returns true if the subject has granted a
// still-effective consent that covers the given scope against
// the given controller. Withdrawn and expired grants don't count.
func (node *QuidnugNode) HasActiveConsent(subjectQuid, controllerQuid, scope string) bool {
	if node.PrivacyRegistry == nil {
		return false
	}
	r := node.PrivacyRegistry
	r.mu.RLock()
	defer r.mu.RUnlock()
	now := nowUnix()
	for id, grant := range r.consentGrants {
		if grant.SubjectQuid != subjectQuid {
			continue
		}
		if controllerQuid != "" && grant.ControllerQuid != controllerQuid {
			continue
		}
		if _, withdrawn := r.withdrawnGrants[id]; withdrawn {
			continue
		}
		if grant.EffectiveUntil != 0 && grant.EffectiveUntil <= now {
			continue
		}
		for _, s := range grant.Scope {
			if s == scope {
				return true
			}
		}
	}
	return false
}

// IsProcessingRestricted returns true if the subject has an
// active PROCESSING_RESTRICTION covering the given use. Expired
// restrictions drop out automatically (QDP-0022 TTL).
func (node *QuidnugNode) IsProcessingRestricted(subjectQuid, use string) bool {
	if node.PrivacyRegistry == nil {
		return false
	}
	r := node.PrivacyRegistry
	r.mu.RLock()
	defer r.mu.RUnlock()
	now := nowUnix()
	for _, rt := range r.activeRestrictions[subjectQuid] {
		if rt.EffectiveUntil != 0 && rt.EffectiveUntil <= now {
			continue
		}
		for _, u := range rt.RestrictedUses {
			if u == use {
				return true
			}
		}
	}
	return false
}

// RestrictedUsesFor returns the union of active restricted-use
// strings for a subject. Useful for transparency endpoints.
func (node *QuidnugNode) RestrictedUsesFor(subjectQuid string) []string {
	if node.PrivacyRegistry == nil {
		return nil
	}
	r := node.PrivacyRegistry
	r.mu.RLock()
	defer r.mu.RUnlock()
	now := nowUnix()
	seen := make(map[string]struct{})
	for _, rt := range r.activeRestrictions[subjectQuid] {
		if rt.EffectiveUntil != 0 && rt.EffectiveUntil <= now {
			continue
		}
		for _, u := range rt.RestrictedUses {
			seen[u] = struct{}{}
		}
	}
	out := make([]string, 0, len(seen))
	for u := range seen {
		out = append(out, u)
	}
	return out
}

// ConsentHistory returns every CONSENT_GRANT (and whether it's
// been withdrawn) for a subject. Used to serve
// `GET /api/v2/consent/history`.
type ConsentHistoryEntry struct {
	Grant     ConsentGrantTransaction `json:"grant"`
	Withdrawn bool                    `json:"withdrawn"`
}

// ConsentHistoryFor returns the subject's full consent history.
func (node *QuidnugNode) ConsentHistoryFor(subjectQuid string) []ConsentHistoryEntry {
	if node.PrivacyRegistry == nil {
		return nil
	}
	r := node.PrivacyRegistry
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]ConsentHistoryEntry, 0)
	for id, grant := range r.consentGrants {
		if grant.SubjectQuid != subjectQuid {
			continue
		}
		_, withdrawn := r.withdrawnGrants[id]
		out = append(out, ConsentHistoryEntry{Grant: grant, Withdrawn: withdrawn})
	}
	return out
}

// GetDSRStatus returns the request record + (optional)
// compliance record for the given request id.
func (node *QuidnugNode) GetDSRStatus(requestTxID string) (DataSubjectRequestTransaction, DSRComplianceTransaction, bool, bool) {
	if node.PrivacyRegistry == nil {
		return DataSubjectRequestTransaction{}, DSRComplianceTransaction{}, false, false
	}
	r := node.PrivacyRegistry
	r.mu.RLock()
	defer r.mu.RUnlock()
	req, reqOk := r.dsrRequests[requestTxID]
	comp, compOk := r.dsrCompletions[requestTxID]
	return req, comp, reqOk, compOk
}

// ---- block-processing dispatch ---------------------------------------

func (node *QuidnugNode) updatePrivacyRegistryDSR(tx DataSubjectRequestTransaction) {
	if node.PrivacyRegistry == nil {
		return
	}
	node.PrivacyRegistry.recordDSR(tx)
	logger.Debug("Recorded data subject request",
		"txId", tx.ID, "subject", tx.SubjectQuid, "type", tx.RequestType,
		"jurisdiction", tx.Jurisdiction, "nonce", tx.Nonce)
}

func (node *QuidnugNode) updatePrivacyRegistryGrant(tx ConsentGrantTransaction) {
	if node.PrivacyRegistry == nil {
		return
	}
	node.PrivacyRegistry.recordGrant(tx)
	logger.Debug("Recorded consent grant",
		"txId", tx.ID, "subject", tx.SubjectQuid,
		"controller", tx.ControllerQuid, "scope", tx.Scope, "nonce", tx.Nonce)
}

func (node *QuidnugNode) updatePrivacyRegistryWithdraw(tx ConsentWithdrawTransaction) {
	if node.PrivacyRegistry == nil {
		return
	}
	node.PrivacyRegistry.recordWithdraw(tx)
	logger.Debug("Recorded consent withdraw",
		"txId", tx.ID, "subject", tx.SubjectQuid,
		"withdrawsGrantTxId", tx.WithdrawsGrantTxID, "nonce", tx.Nonce)
}

func (node *QuidnugNode) updatePrivacyRegistryRestriction(tx ProcessingRestrictionTransaction) {
	if node.PrivacyRegistry == nil {
		return
	}
	node.PrivacyRegistry.recordRestriction(tx)
	logger.Debug("Recorded processing restriction",
		"txId", tx.ID, "subject", tx.SubjectQuid,
		"uses", tx.RestrictedUses, "nonce", tx.Nonce)
}

func (node *QuidnugNode) updatePrivacyRegistryCompliance(tx DSRComplianceTransaction) {
	if node.PrivacyRegistry == nil {
		return
	}
	node.PrivacyRegistry.recordCompliance(tx)
	logger.Debug("Recorded DSR compliance",
		"txId", tx.ID, "requestTxId", tx.RequestTxID,
		"type", tx.RequestType, "operator", tx.OperatorQuid, "nonce", tx.Nonce)
}

// ---- validators -------------------------------------------------------

// ValidateDataSubjectRequestTransaction enforces the phase-1
// rules from QDP-0017 §4. Subject must sign; request type is
// enum-checked; nonce monotonic per-subject.
func (node *QuidnugNode) ValidateDataSubjectRequestTransaction(tx DataSubjectRequestTransaction) bool {
	if !node.validatePrivacyDomain(tx.TrustDomain, tx.ID, "DSR") {
		return false
	}
	if tx.SubjectQuid == "" || !IsValidQuidID(tx.SubjectQuid) {
		logger.Warn("DSR has invalid SubjectQuid",
			"subject", tx.SubjectQuid, "txId", tx.ID)
		return false
	}
	if !validatePrivacyPubkeyBinding(tx.SubjectQuid, tx.PublicKey, tx.ID, "DSR") {
		return false
	}
	if _, ok := validDSRTypes[tx.RequestType]; !ok {
		logger.Warn("DSR has invalid request type",
			"requestType", tx.RequestType, "txId", tx.ID)
		return false
	}
	if len(tx.RequestDetails) > MaxRequestDetailsLength {
		logger.Warn("DSR request details exceed max length",
			"length", len(tx.RequestDetails), "txId", tx.ID)
		return false
	}
	if tx.ControllerQuid != "" && !IsValidQuidID(tx.ControllerQuid) {
		logger.Warn("DSR has invalid ControllerQuid",
			"controller", tx.ControllerQuid, "txId", tx.ID)
		return false
	}
	if !node.validatePrivacyNonce(tx.SubjectQuid, TxTypeDataSubjectRequest, tx.Nonce, tx.ID) {
		return false
	}
	return verifyPrivacyTxSignature(tx, tx.Signature, tx.PublicKey, tx.ID, "DSR")
}

// ValidateConsentGrantTransaction enforces QDP-0017 §5.1 rules.
func (node *QuidnugNode) ValidateConsentGrantTransaction(tx ConsentGrantTransaction) bool {
	if !node.validatePrivacyDomain(tx.TrustDomain, tx.ID, "consent grant") {
		return false
	}
	if tx.SubjectQuid == "" || !IsValidQuidID(tx.SubjectQuid) {
		logger.Warn("Consent grant has invalid SubjectQuid",
			"subject", tx.SubjectQuid, "txId", tx.ID)
		return false
	}
	if tx.ControllerQuid == "" || !IsValidQuidID(tx.ControllerQuid) {
		logger.Warn("Consent grant has invalid ControllerQuid",
			"controller", tx.ControllerQuid, "txId", tx.ID)
		return false
	}
	if !validatePrivacyPubkeyBinding(tx.SubjectQuid, tx.PublicKey, tx.ID, "consent grant") {
		return false
	}
	if len(tx.Scope) == 0 {
		logger.Warn("Consent grant has empty Scope", "txId", tx.ID)
		return false
	}
	for _, s := range tx.Scope {
		if _, ok := validConsentScopes[s]; ok {
			continue
		}
		if strings.HasPrefix(s, ConsentScopeCustomPrefix) && len(s) > len(ConsentScopeCustomPrefix) {
			continue
		}
		logger.Warn("Consent grant has invalid scope entry",
			"scope", s, "txId", tx.ID)
		return false
	}
	if len(tx.PolicyHash) > MaxPolicyHashLength {
		logger.Warn("Consent grant PolicyHash too long",
			"length", len(tx.PolicyHash), "txId", tx.ID)
		return false
	}
	if tx.EffectiveUntil != 0 && tx.EffectiveUntil <= tx.Timestamp {
		logger.Warn("Consent grant already expired at submission",
			"effectiveUntil", tx.EffectiveUntil, "ts", tx.Timestamp, "txId", tx.ID)
		return false
	}
	if !node.validatePrivacyNonce(tx.SubjectQuid, TxTypeConsentGrant, tx.Nonce, tx.ID) {
		return false
	}
	return verifyPrivacyTxSignature(tx, tx.Signature, tx.PublicKey, tx.ID, "consent grant")
}

// ValidateConsentWithdrawTransaction enforces QDP-0017 §5.3
// rules. The withdraw must reference an existing grant from
// the same subject.
func (node *QuidnugNode) ValidateConsentWithdrawTransaction(tx ConsentWithdrawTransaction) bool {
	if !node.validatePrivacyDomain(tx.TrustDomain, tx.ID, "consent withdraw") {
		return false
	}
	if tx.SubjectQuid == "" || !IsValidQuidID(tx.SubjectQuid) {
		logger.Warn("Consent withdraw has invalid SubjectQuid",
			"subject", tx.SubjectQuid, "txId", tx.ID)
		return false
	}
	if !validatePrivacyPubkeyBinding(tx.SubjectQuid, tx.PublicKey, tx.ID, "consent withdraw") {
		return false
	}
	if tx.WithdrawsGrantTxID == "" {
		logger.Warn("Consent withdraw missing WithdrawsGrantTxID",
			"txId", tx.ID)
		return false
	}
	if node.PrivacyRegistry != nil {
		node.PrivacyRegistry.mu.RLock()
		grant, ok := node.PrivacyRegistry.consentGrants[tx.WithdrawsGrantTxID]
		node.PrivacyRegistry.mu.RUnlock()
		if !ok {
			logger.Warn("Consent withdraw references unknown grant",
				"withdrawsGrantTxId", tx.WithdrawsGrantTxID, "txId", tx.ID)
			return false
		}
		if grant.SubjectQuid != tx.SubjectQuid {
			logger.Warn("Consent withdraw subject does not match grant subject",
				"withdrawSubject", tx.SubjectQuid, "grantSubject", grant.SubjectQuid,
				"txId", tx.ID)
			return false
		}
	}
	if !node.validatePrivacyNonce(tx.SubjectQuid, TxTypeConsentWithdraw, tx.Nonce, tx.ID) {
		return false
	}
	return verifyPrivacyTxSignature(tx, tx.Signature, tx.PublicKey, tx.ID, "consent withdraw")
}

// ValidateProcessingRestrictionTransaction enforces QDP-0017
// §4.4 rules.
func (node *QuidnugNode) ValidateProcessingRestrictionTransaction(tx ProcessingRestrictionTransaction) bool {
	if !node.validatePrivacyDomain(tx.TrustDomain, tx.ID, "processing restriction") {
		return false
	}
	if tx.SubjectQuid == "" || !IsValidQuidID(tx.SubjectQuid) {
		logger.Warn("Processing restriction has invalid SubjectQuid",
			"subject", tx.SubjectQuid, "txId", tx.ID)
		return false
	}
	if !validatePrivacyPubkeyBinding(tx.SubjectQuid, tx.PublicKey, tx.ID, "processing restriction") {
		return false
	}
	if len(tx.RestrictedUses) == 0 {
		logger.Warn("Processing restriction has empty RestrictedUses",
			"txId", tx.ID)
		return false
	}
	for _, u := range tx.RestrictedUses {
		if _, ok := validRestrictionUses[u]; !ok {
			logger.Warn("Processing restriction has invalid use",
				"use", u, "txId", tx.ID)
			return false
		}
	}
	if tx.EffectiveUntil != 0 && tx.EffectiveUntil <= tx.Timestamp {
		logger.Warn("Processing restriction already expired at submission",
			"effectiveUntil", tx.EffectiveUntil, "ts", tx.Timestamp, "txId", tx.ID)
		return false
	}
	if !node.validatePrivacyNonce(tx.SubjectQuid, TxTypeProcessingRestriction, tx.Nonce, tx.ID) {
		return false
	}
	return verifyPrivacyTxSignature(tx, tx.Signature, tx.PublicKey, tx.ID, "processing restriction")
}

// ValidateDSRComplianceTransaction is signed by the operator,
// not the subject; it attests to having processed a DSR. The
// operator must be a validator on the tx's domain.
func (node *QuidnugNode) ValidateDSRComplianceTransaction(tx DSRComplianceTransaction) bool {
	if !node.validatePrivacyDomain(tx.TrustDomain, tx.ID, "DSR compliance") {
		return false
	}
	if tx.OperatorQuid == "" || !IsValidQuidID(tx.OperatorQuid) {
		logger.Warn("DSR compliance has invalid OperatorQuid",
			"operator", tx.OperatorQuid, "txId", tx.ID)
		return false
	}
	if !validatePrivacyPubkeyBinding(tx.OperatorQuid, tx.PublicKey, tx.ID, "DSR compliance") {
		return false
	}
	if tx.RequestTxID == "" {
		logger.Warn("DSR compliance missing RequestTxID", "txId", tx.ID)
		return false
	}
	if _, ok := validDSRTypes[tx.RequestType]; !ok {
		logger.Warn("DSR compliance has invalid request type",
			"requestType", tx.RequestType, "txId", tx.ID)
		return false
	}
	if tx.CompletedAt <= 0 {
		logger.Warn("DSR compliance missing CompletedAt", "txId", tx.ID)
		return false
	}
	// Operator must be a validator on the domain.
	node.TrustDomainsMutex.RLock()
	td, domainOk := node.TrustDomains[tx.TrustDomain]
	node.TrustDomainsMutex.RUnlock()
	if !domainOk {
		return false
	}
	if _, isValidator := td.Validators[tx.OperatorQuid]; !isValidator {
		logger.Warn("DSR compliance operator is not a validator",
			"operator", tx.OperatorQuid, "domain", tx.TrustDomain, "txId", tx.ID)
		return false
	}
	if !node.validatePrivacyNonce(tx.OperatorQuid, TxTypeDSRCompliance, tx.Nonce, tx.ID) {
		return false
	}
	return verifyPrivacyTxSignature(tx, tx.Signature, tx.PublicKey, tx.ID, "DSR compliance")
}

// ---- shared helpers ---------------------------------------------------

// validatePrivacyDomain is the common domain-existence + node-
// support check used by every QDP-0017 validator.
func (node *QuidnugNode) validatePrivacyDomain(domain, txID, kind string) bool {
	if domain == "" {
		logger.Warn(kind+" missing trust domain", "txId", txID)
		return false
	}
	node.TrustDomainsMutex.RLock()
	_, ok := node.TrustDomains[domain]
	node.TrustDomainsMutex.RUnlock()
	if !ok {
		logger.Warn(kind+" from unknown trust domain",
			"domain", domain, "txId", txID)
		return false
	}
	if !node.IsDomainSupported(domain) {
		logger.Warn(kind+" trust domain not supported by this node",
			"domain", domain, "txId", txID)
		return false
	}
	return true
}

// validatePrivacyPubkeyBinding checks that the signing pubkey
// derives to the expected quid id — the "self-sign consistency"
// requirement that proves the signer controls the declared quid.
func validatePrivacyPubkeyBinding(expectedQuid, pubkey, txID, kind string) bool {
	if pubkey == "" {
		logger.Warn(kind+" missing PublicKey", "txId", txID)
		return false
	}
	computed := QuidIDFromPublicKeyHex(pubkey)
	if computed == "" || computed != expectedQuid {
		logger.Warn(kind+" quid does not match signing public key",
			"expected", expectedQuid, "computed", computed, "txId", txID)
		return false
	}
	return true
}

// validatePrivacyNonce enforces strict monotonic nonces per
// (actorQuid, txType). Blocks replays and out-of-order submits.
func (node *QuidnugNode) validatePrivacyNonce(actorQuid string, txType TransactionType, nonce int64, txID string) bool {
	if nonce <= 0 {
		logger.Warn("Privacy tx has non-positive nonce",
			"type", txType, "nonce", nonce, "txId", txID)
		return false
	}
	if node.PrivacyRegistry == nil {
		return true
	}
	prev := node.PrivacyRegistry.currentNonce(actorQuid, txType)
	if nonce <= prev {
		logger.Warn("Privacy tx nonce must be strictly greater than previous",
			"type", txType, "previous", prev, "provided", nonce, "txId", txID)
		return false
	}
	return true
}

// verifyPrivacyTxSignature is a shared signature-check helper.
// Receives the tx by value, clears the Signature field on a
// typed copy, and re-marshals the struct directly so key order
// exactly matches what the signer produced. Using a map
// round-trip here reorders keys alphabetically and silently
// breaks verification.
func verifyPrivacyTxSignature(tx interface{}, sig, pubkey, txID, kind string) bool {
	if sig == "" {
		logger.Warn(kind+" missing signature", "txId", txID)
		return false
	}
	var signable []byte
	var err error
	switch v := tx.(type) {
	case DataSubjectRequestTransaction:
		v.Signature = ""
		signable, err = json.Marshal(v)
	case ConsentGrantTransaction:
		v.Signature = ""
		signable, err = json.Marshal(v)
	case ConsentWithdrawTransaction:
		v.Signature = ""
		signable, err = json.Marshal(v)
	case ProcessingRestrictionTransaction:
		v.Signature = ""
		signable, err = json.Marshal(v)
	case DSRComplianceTransaction:
		v.Signature = ""
		signable, err = json.Marshal(v)
	default:
		logger.Error(kind+" unknown tx type for signature verification",
			"txId", txID, "type", fmt.Sprintf("%T", tx))
		return false
	}
	if err != nil {
		logger.Error(kind+" marshal failed", "txId", txID, "err", err)
		return false
	}
	if !VerifySignature(pubkey, signable, sig) {
		logger.Warn(kind+" signature invalid", "txId", txID)
		return false
	}
	return true
}

// HumanReadableRestrictedUses produces a stable comma-joined
// list for logging / telemetry. Kept here so external packages
// can reuse.
func HumanReadableRestrictedUses(uses []string) string {
	return fmt.Sprintf("[%s]", strings.Join(uses, ","))
}
