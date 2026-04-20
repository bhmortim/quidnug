// Package core — QDP-0015 Content Moderation & Takedowns.
//
// This file implements the MODERATION_ACTION transaction type
// and the serving-time suppression filter. The chain itself
// stays append-only; every moderation action is a signed
// transaction whose validity is independently auditable. What
// changes is whether a node's HTTP API chooses to serve the
// targeted content.
//
// Companion file structure mirrors node_advertisement.go:
//
//   - types.go          : TxTypeModerationAction const
//   - moderation.go     : this file — struct, registry, validator
//   - transactions.go   : AddModerationActionTransaction (mempool)
//   - validation.go     : dispatch into ValidateModerationActionTransaction
//   - registry.go       : dispatch into updateModerationRegistry
//   - handlers.go       : POST submit handler + serving-time filter
//   - node.go           : ModerationRegistry field + init
//
// Phase 1 scope (matches QDP-0015 §8):
//   - Transaction shape + validation (all 12 rules).
//   - Target-indexed registry with supersede-chain resolution.
//   - Event stream serving filter (suppress / hide / annotate).
//
// Out of scope for phase 1: federation import endpoint,
// moderator delegation through a dedicated `moderators.*` domain
// (currently any validator-trusted quid can moderate), CLI
// tooling, and the dashboard. Those are phases 2-5.
package core

import (
	"encoding/json"
	"fmt"
	"strings"
	"sync"
)

// Reason codes from the QDP-0015 §4.5 taxonomy. Keep in sync
// with the design doc; new codes land via QDP amendment.
const (
	ReasonCodeDMCA                = "DMCA"
	ReasonCodeCourtOrder          = "COURT_ORDER"
	ReasonCodeCSAM                = "CSAM"
	ReasonCodeGDPRErasure         = "GDPR_ERASURE"
	ReasonCodeDataSubjectRequest  = "DATA_SUBJECT_REQUEST"
	ReasonCodeHateSpeech          = "HATE_SPEECH"
	ReasonCodeDefamation          = "DEFAMATION"
	ReasonCodeTOSViolation        = "TOS_VIOLATION"
	ReasonCodeSpam                = "SPAM"
	ReasonCodeMisinformation      = "MISINFORMATION"
	ReasonCodeVoluntary           = "VOLUNTARY"
	ReasonCodeOther               = "OTHER"
)

// Target types describe what a ModerationActionTransaction
// addresses. The registry is sharded by (TargetType, TargetID).
const (
	ModerationTargetTx               = "TX"
	ModerationTargetQuid             = "QUID"
	ModerationTargetDomain           = "DOMAIN"
	ModerationTargetReviewOfProduct  = "REVIEW_OF_PRODUCT"
)

// Scopes are the three severities from QDP-0015 §3.2. The
// composition rule is `suppress > hide > annotate`.
const (
	ModerationScopeSuppress = "suppress"
	ModerationScopeHide     = "hide"
	ModerationScopeAnnotate = "annotate"
)

// MaxAnnotationTextLength bounds the annotation text payload
// to prevent chain bloat (QDP-0015 §7.3 mitigation).
const MaxAnnotationTextLength = 2048

// reasonCodesRequiringEvidence is the subset of reason codes
// that MUST ship with a non-empty EvidenceURL. VOLUNTARY, SPAM,
// TOS_VIOLATION, and OTHER are exempted because the evidence
// is often purely internal (user request, classifier output,
// operator policy note) and may not be publicly linkable.
var reasonCodesRequiringEvidence = map[string]struct{}{
	ReasonCodeDMCA:               {},
	ReasonCodeCourtOrder:         {},
	ReasonCodeCSAM:               {},
	ReasonCodeGDPRErasure:        {},
	ReasonCodeDataSubjectRequest: {},
	ReasonCodeHateSpeech:         {},
	ReasonCodeDefamation:         {},
	ReasonCodeMisinformation:     {},
}

// validReasonCodes lists every reason code the enum accepts.
var validReasonCodes = map[string]struct{}{
	ReasonCodeDMCA:               {},
	ReasonCodeCourtOrder:         {},
	ReasonCodeCSAM:               {},
	ReasonCodeGDPRErasure:        {},
	ReasonCodeDataSubjectRequest: {},
	ReasonCodeHateSpeech:         {},
	ReasonCodeDefamation:         {},
	ReasonCodeTOSViolation:       {},
	ReasonCodeSpam:               {},
	ReasonCodeMisinformation:     {},
	ReasonCodeVoluntary:          {},
	ReasonCodeOther:              {},
}

// ModerationActionTransaction is the on-chain record of an
// operator or delegated moderator applying a policy action
// (suppress / hide / annotate) to a specific target.
//
// See docs/design/0015-content-moderation.md §4.1 for the full
// field semantics. Fields are JSON-tagged with the same names
// the design doc and client SDKs use.
type ModerationActionTransaction struct {
	BaseTransaction

	ModeratorQuid string `json:"moderatorQuid"`

	TargetType string `json:"targetType"`
	TargetID   string `json:"targetId"`

	Scope string `json:"scope"`

	ReasonCode  string `json:"reasonCode"`
	EvidenceURL string `json:"evidenceUrl,omitempty"`

	AnnotationText string `json:"annotationText,omitempty"`

	SupersedesTxID string `json:"supersedesTxId,omitempty"`

	EffectiveFrom  int64 `json:"effectiveFrom,omitempty"`
	EffectiveUntil int64 `json:"effectiveUntil,omitempty"`

	Nonce int64 `json:"nonce"`

	// DoNotFederate, when true, signals that gossip layers
	// MUST NOT propagate this action across federation
	// boundaries. Trust in the flag is social; for strict
	// confidentiality keep the action entirely off-chain. See
	// QDP-0015 §6.3.
	DoNotFederate bool `json:"doNotFederate,omitempty"`
}

// moderationKey is the composite key used to index the
// ModerationRegistry.
type moderationKey struct {
	targetType string
	targetID   string
}

func keyFor(targetType, targetID string) moderationKey {
	return moderationKey{targetType: targetType, targetID: targetID}
}

// ModerationRegistry indexes every accepted
// ModerationActionTransaction by its (TargetType, TargetID) so
// the serving layer can resolve "what's the effective scope
// for this target right now?" in O(actions-on-target) time.
//
// Actions are stored in append order. Supersede-chain walking
// is computed on read.
type ModerationRegistry struct {
	mu sync.RWMutex

	// actions keyed by (targetType, targetID) → append-order list.
	actions map[moderationKey][]ModerationActionTransaction

	// byID indexes every action by its tx ID for supersede-chain
	// validation + lookup.
	byID map[string]ModerationActionTransaction

	// nonces tracks the highest nonce we've accepted per
	// moderator quid, enforcing strict monotonicity.
	nonces map[string]int64
}

// NewModerationRegistry constructs an empty registry.
func NewModerationRegistry() *ModerationRegistry {
	return &ModerationRegistry{
		actions: make(map[moderationKey][]ModerationActionTransaction),
		byID:    make(map[string]ModerationActionTransaction),
		nonces:  make(map[string]int64),
	}
}

// currentNonce returns the highest accepted nonce for a
// moderator quid (0 if none).
func (r *ModerationRegistry) currentNonce(moderatorQuid string) int64 {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.nonces[moderatorQuid]
}

// hasAction returns whether an action with the given tx ID is
// already in the registry. Used for supersede-chain validation.
func (r *ModerationRegistry) hasAction(txID string) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	_, ok := r.byID[txID]
	return ok
}

// supersedes returns the action with the given ID (if it
// exists) and whether the caller moderator matches — used at
// validation time to enforce "supersede chains are single-parent
// and scoped to the same moderator."
func (r *ModerationRegistry) supersedes(txID string) (ModerationActionTransaction, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	act, ok := r.byID[txID]
	return act, ok
}

// upsert records a validated action into the registry. Idempotent
// on replay: the ID key deduplicates.
func (r *ModerationRegistry) upsert(tx ModerationActionTransaction) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, seen := r.byID[tx.ID]; seen {
		return
	}
	r.byID[tx.ID] = tx
	k := keyFor(tx.TargetType, tx.TargetID)
	r.actions[k] = append(r.actions[k], tx)
	if tx.Nonce > r.nonces[tx.ModeratorQuid] {
		r.nonces[tx.ModeratorQuid] = tx.Nonce
	}
}

// actionsFor returns a copy of every action touching the given
// target. Caller gets a fresh slice; mutation does not affect
// the registry.
func (r *ModerationRegistry) actionsFor(targetType, targetID string) []ModerationActionTransaction {
	r.mu.RLock()
	defer r.mu.RUnlock()
	src := r.actions[keyFor(targetType, targetID)]
	if len(src) == 0 {
		return nil
	}
	out := make([]ModerationActionTransaction, len(src))
	copy(out, src)
	return out
}

// EffectiveScope is the result of resolving every active
// MODERATION_ACTION on a target into a single serving-time
// decision. Exported so handlers can pattern-match on it.
type EffectiveScope struct {
	// Scope is "", "annotate", "hide", or "suppress" (ordered
	// by increasing severity).
	Scope string
	// ReasonCode carries the highest-severity active action's
	// reason code, for response-header use.
	ReasonCode string
	// AnnotationText is non-empty only when Scope == "annotate".
	AnnotationText string
}

// computeEffectiveScope walks a list of actions on the same
// target, builds the supersede-chain tips per moderator, and
// returns the highest-severity active scope at `now`.
//
// Per QDP-0015 §4.6:
//   1. Per (moderator, target), only the tip of the supersede
//      chain counts.
//   2. Union tips across moderators.
//   3. Effective scope = max severity across active tips.
//
// An action is "active" at `now` when:
//   - EffectiveFrom == 0 OR EffectiveFrom <= now, AND
//   - EffectiveUntil == 0 OR EffectiveUntil > now.
func computeEffectiveScope(actions []ModerationActionTransaction, nowUnixSec int64) EffectiveScope {
	if len(actions) == 0 {
		return EffectiveScope{}
	}

	// Mark every tx that is superseded by some other tx — the
	// "tips" are those not superseded.
	supersededIDs := make(map[string]struct{})
	for _, a := range actions {
		if a.SupersedesTxID != "" {
			supersededIDs[a.SupersedesTxID] = struct{}{}
		}
	}

	// severity assigns an integer rank; used for
	// max-composition.
	severity := func(scope string) int {
		switch scope {
		case ModerationScopeSuppress:
			return 3
		case ModerationScopeHide:
			return 2
		case ModerationScopeAnnotate:
			return 1
		default:
			return 0
		}
	}

	var (
		bestSeverity int
		bestScope    string
		bestReason   string
		bestAnnot    string
	)

	for _, a := range actions {
		if _, superseded := supersededIDs[a.ID]; superseded {
			continue
		}
		if a.EffectiveFrom != 0 && a.EffectiveFrom > nowUnixSec {
			continue
		}
		if a.EffectiveUntil != 0 && a.EffectiveUntil <= nowUnixSec {
			continue
		}
		s := severity(a.Scope)
		if s > bestSeverity {
			bestSeverity = s
			bestScope = a.Scope
			bestReason = a.ReasonCode
			bestAnnot = a.AnnotationText
		}
	}

	return EffectiveScope{
		Scope:          bestScope,
		ReasonCode:     bestReason,
		AnnotationText: bestAnnot,
	}
}

// EffectiveScopeFor returns the current serving-time decision
// for a target. Thin public wrapper; the HTTP layer uses this
// rather than touching computeEffectiveScope directly.
func (node *QuidnugNode) EffectiveScopeFor(targetType, targetID string) EffectiveScope {
	if node.ModerationRegistry == nil {
		return EffectiveScope{}
	}
	actions := node.ModerationRegistry.actionsFor(targetType, targetID)
	return computeEffectiveScope(actions, nowUnix())
}

// IsTargetSuppressed returns true if the target has an active
// scope of "suppress" — the strongest scope, treated as "do not
// serve at all." Handlers use this as a fast-path short-circuit.
func (node *QuidnugNode) IsTargetSuppressed(targetType, targetID string) bool {
	return node.EffectiveScopeFor(targetType, targetID).Scope == ModerationScopeSuppress
}

// updateModerationRegistry commits a validated
// ModerationActionTransaction into the registry. Called from
// processBlockTransactions when a block containing it has been
// accepted.
func (node *QuidnugNode) updateModerationRegistry(tx ModerationActionTransaction) {
	if node.ModerationRegistry == nil {
		return
	}
	node.ModerationRegistry.upsert(tx)
	logger.Debug("Updated moderation registry",
		"txId", tx.ID,
		"moderator", tx.ModeratorQuid,
		"targetType", tx.TargetType,
		"targetId", tx.TargetID,
		"scope", tx.Scope,
		"reason", tx.ReasonCode,
		"nonce", tx.Nonce)
}

// ValidateModerationActionTransaction enforces the QDP-0015 §4.2
// rules. Returns false on any violation; every failure is
// logged at Warn level so operators can diagnose rejected
// actions.
func (node *QuidnugNode) ValidateModerationActionTransaction(tx ModerationActionTransaction) bool {
	// 1. Domain must exist + be supported.
	if tx.TrustDomain == "" {
		logger.Warn("Moderation action missing trust domain", "txId", tx.ID)
		return false
	}
	node.TrustDomainsMutex.RLock()
	_, domainExists := node.TrustDomains[tx.TrustDomain]
	node.TrustDomainsMutex.RUnlock()
	if !domainExists {
		logger.Warn("Moderation action from unknown trust domain",
			"domain", tx.TrustDomain, "txId", tx.ID)
		return false
	}
	if !node.IsDomainSupported(tx.TrustDomain) {
		logger.Warn("Moderation action trust domain not supported by this node",
			"domain", tx.TrustDomain, "txId", tx.ID)
		return false
	}

	// 2. Required fields.
	if tx.ModeratorQuid == "" || !IsValidQuidID(tx.ModeratorQuid) {
		logger.Warn("Moderation action has invalid ModeratorQuid",
			"moderator", tx.ModeratorQuid, "txId", tx.ID)
		return false
	}
	if tx.Signature == "" || tx.PublicKey == "" {
		logger.Warn("Moderation action missing signature or public key", "txId", tx.ID)
		return false
	}

	// Self-sign consistency — the signing pubkey must derive
	// to ModeratorQuid.
	computedQuid := QuidIDFromPublicKeyHex(tx.PublicKey)
	if computedQuid == "" || computedQuid != tx.ModeratorQuid {
		logger.Warn("Moderation action ModeratorQuid does not match signing public key",
			"expected", tx.ModeratorQuid, "computed", computedQuid, "txId", tx.ID)
		return false
	}

	// 3. TargetType / TargetID format check.
	if err := validateModerationTarget(tx.TargetType, tx.TargetID); err != nil {
		logger.Warn("Moderation action target invalid",
			"targetType", tx.TargetType, "targetId", tx.TargetID,
			"err", err, "txId", tx.ID)
		return false
	}

	// 4. Scope enum.
	switch tx.Scope {
	case ModerationScopeSuppress, ModerationScopeHide, ModerationScopeAnnotate:
	default:
		logger.Warn("Moderation action has unknown scope",
			"scope", tx.Scope, "txId", tx.ID)
		return false
	}

	// 5. ReasonCode enum.
	if _, ok := validReasonCodes[tx.ReasonCode]; !ok {
		logger.Warn("Moderation action has unknown reasonCode",
			"reasonCode", tx.ReasonCode, "txId", tx.ID)
		return false
	}

	// 6. Evidence URL required for specific reason codes.
	if _, required := reasonCodesRequiringEvidence[tx.ReasonCode]; required {
		if strings.TrimSpace(tx.EvidenceURL) == "" {
			logger.Warn("Moderation action missing required EvidenceURL",
				"reasonCode", tx.ReasonCode, "txId", tx.ID)
			return false
		}
	}

	// 7. Annotation text length + no control chars.
	if len(tx.AnnotationText) > MaxAnnotationTextLength {
		logger.Warn("Moderation action AnnotationText exceeds max length",
			"length", len(tx.AnnotationText), "max", MaxAnnotationTextLength,
			"txId", tx.ID)
		return false
	}
	if containsControlChar(tx.AnnotationText) {
		logger.Warn("Moderation action AnnotationText contains control chars",
			"txId", tx.ID)
		return false
	}

	// 8. Supersede chain constraints.
	if tx.SupersedesTxID != "" {
		if node.ModerationRegistry == nil {
			logger.Warn("Moderation action supersedes without a registry",
				"txId", tx.ID)
			return false
		}
		prior, ok := node.ModerationRegistry.supersedes(tx.SupersedesTxID)
		if !ok {
			logger.Warn("Moderation action supersedes unknown tx",
				"supersedesTxId", tx.SupersedesTxID, "txId", tx.ID)
			return false
		}
		if prior.ModeratorQuid != tx.ModeratorQuid {
			logger.Warn("Moderation action supersedes tx from different moderator",
				"priorModerator", prior.ModeratorQuid,
				"selfModerator", tx.ModeratorQuid, "txId", tx.ID)
			return false
		}
	}

	// 9. Effective-range sanity.
	if tx.EffectiveFrom != 0 && tx.EffectiveUntil != 0 &&
		tx.EffectiveFrom >= tx.EffectiveUntil {
		logger.Warn("Moderation action has non-positive effective range",
			"from", tx.EffectiveFrom, "until", tx.EffectiveUntil, "txId", tx.ID)
		return false
	}

	// 10. Nonce strictly monotonic per moderator.
	if tx.Nonce <= 0 {
		logger.Warn("Moderation action has non-positive nonce",
			"nonce", tx.Nonce, "txId", tx.ID)
		return false
	}
	if node.ModerationRegistry != nil {
		prev := node.ModerationRegistry.currentNonce(tx.ModeratorQuid)
		if tx.Nonce <= prev {
			logger.Warn("Moderation action nonce must be strictly greater than previous",
				"previous", prev, "provided", tx.Nonce, "txId", tx.ID)
			return false
		}
	}

	// 11. Moderator is authorized. Phase 1: authority granted
	// to any quid that is a validator for the tx's domain OR
	// that is directly trusted by a validator at >= 0.7. This
	// is intentionally loose; a follow-up adds a dedicated
	// `moderators.*` domain semantic (see QDP-0015 §4.4).
	if !node.isAuthorizedModerator(tx.ModeratorQuid, tx.TrustDomain) {
		logger.Warn("Moderation action from unauthorized moderator",
			"moderator", tx.ModeratorQuid, "domain", tx.TrustDomain,
			"txId", tx.ID)
		return false
	}

	// 12. Signature verifies.
	txCopy := tx
	txCopy.Signature = ""
	signable, err := json.Marshal(txCopy)
	if err != nil {
		logger.Error("Moderation action marshal for signature failed",
			"txId", tx.ID, "err", err)
		return false
	}
	if !VerifySignature(tx.PublicKey, signable, tx.Signature) {
		logger.Warn("Moderation action signature invalid", "txId", tx.ID)
		return false
	}

	return true
}

// validateModerationTarget enforces the per-TargetType identifier
// format. Each TargetType has its own identifier shape.
func validateModerationTarget(targetType, targetID string) error {
	if targetID == "" {
		return fmt.Errorf("empty targetId")
	}
	switch targetType {
	case ModerationTargetTx:
		if len(targetID) != 64 || !isHex(targetID) {
			return fmt.Errorf("TX targetId must be 64-char hex")
		}
	case ModerationTargetQuid:
		if !IsValidQuidID(targetID) {
			return fmt.Errorf("QUID targetId must be a valid 16-char hex quid")
		}
	case ModerationTargetDomain:
		// Domain names accept anything up to DNS max length;
		// deeper semantic validation happens elsewhere.
		if len(targetID) > MaxSupportedDomainLength {
			return fmt.Errorf("DOMAIN targetId too long")
		}
	case ModerationTargetReviewOfProduct:
		if !IsValidQuidID(targetID) {
			return fmt.Errorf("REVIEW_OF_PRODUCT targetId must be a valid 16-char product quid")
		}
	default:
		return fmt.Errorf("unknown targetType %q", targetType)
	}
	return nil
}

// isAuthorizedModerator checks whether the moderator quid is
// entitled to publish moderation actions in the given domain.
// Phase 1 policy:
//
//  1. Domain validators are always authorized — operators who
//     consent-rule the domain can moderate it.
//  2. Quids directly trusted by any domain validator at
//     weight >= 0.7 are also authorized — gives operators a
//     simple delegation path.
//
// Phase 2 will add a reserved `moderators.*` domain semantic
// per QDP-0015 §4.4.
func (node *QuidnugNode) isAuthorizedModerator(moderatorQuid, domain string) bool {
	node.TrustDomainsMutex.RLock()
	td, ok := node.TrustDomains[domain]
	node.TrustDomainsMutex.RUnlock()
	if !ok {
		return false
	}

	// 1. Direct validator authority.
	if weight, ok := td.Validators[moderatorQuid]; ok && weight > 0 {
		return true
	}

	// 2. Delegation from a validator at >= 0.7.
	node.TrustRegistryMutex.RLock()
	defer node.TrustRegistryMutex.RUnlock()
	for validatorQuid := range td.Validators {
		trustees, ok := node.TrustRegistry[validatorQuid]
		if !ok {
			continue
		}
		if level, found := trustees[moderatorQuid]; found && level >= 0.7 {
			// Honor TTL on the delegation edge.
			if !node.isTrustEdgeValidLocked(validatorQuid, moderatorQuid) {
				continue
			}
			return true
		}
	}
	return false
}

// containsControlChar reports whether s contains any ASCII
// control character (0x00–0x1F or 0x7F) other than newline,
// tab, and carriage return. These are disallowed in annotation
// text to avoid rendering and injection attacks.
func containsControlChar(s string) bool {
	for _, r := range s {
		if r == '\n' || r == '\r' || r == '\t' {
			continue
		}
		if r < 0x20 || r == 0x7F {
			return true
		}
	}
	return false
}

// isHex returns true if every rune in s is 0-9 / a-f / A-F.
func isHex(s string) bool {
	for _, r := range s {
		switch {
		case r >= '0' && r <= '9':
		case r >= 'a' && r <= 'f':
		case r >= 'A' && r <= 'F':
		default:
			return false
		}
	}
	return true
}
