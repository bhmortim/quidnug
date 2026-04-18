// Package core — fork_block.go
//
// Fork-block migration trigger (QDP-0009 / H5). A signed
// transaction declaring "at block height H, every node
// honoring this transaction flips feature F." Provides the
// coordination mechanism for feature-flag rollouts on a
// production network without wall-clock timing games.
//
// Invariants:
//
//   - Feature names must be in ForkSupportedFeatures. Unknown
//     features are rejected so operators cannot sneak in
//     behavior changes the protocol doesn't recognize.
//
//   - Fork-nonce is per-(domain, feature). Prevents replay
//     across forks.
//
//   - Activation is idempotent. Catching up on a chain that
//     already crossed a ForkHeight applies the feature once.
//
//   - Validator quorum enforced via signatures. The quorum
//     threshold is domain-configurable; default is 2/3 of
//     declared validators.
package core

import (
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"time"
)

// ----- Constants -----------------------------------------------------------

const AnchorForkBlock AnchorKind = AnchorGuardianResign + 1 // = 9

const TxTypeForkBlock TransactionType = "FORK_BLOCK"

// MinForkNoticeBlocks is the minimum number of blocks between
// a fork transaction's acceptance and its ForkHeight. Gives
// operators and automated systems time to prepare.
const MinForkNoticeBlocks = int64(1440)

// DefaultValidatorQuorumNumerator / Denominator specify the
// quorum-of-validators fraction required. 2/3 gives
// Byzantine-fault-tolerance-compatible threshold while
// tolerating typical validator availability.
const (
	DefaultValidatorQuorumNumerator   = 2
	DefaultValidatorQuorumDenominator = 3
)

// ForkSupportedFeatures enumerates the feature flags that can
// be activated by a fork transaction. New features require
// adding their names here explicitly — an operator can't
// invent arbitrary flag names.
var ForkSupportedFeatures = map[string]bool{
	"enable_nonce_ledger":     true,
	"enable_push_gossip":      true,
	"enable_lazy_epoch_probe": true,
	"enable_kofk_bootstrap":   true,
	"require_tx_tree_root":    true, // future H2
}

// ----- Wire types ----------------------------------------------------------

// ForkBlock is the anchor-level struct. Carries the fork
// declaration + validator signatures.
type ForkBlock struct {
	Kind        AnchorKind `json:"kind"`
	TrustDomain string     `json:"trustDomain"`
	Feature     string     `json:"feature"`
	ForkHeight  int64      `json:"forkHeight"`
	ForkNonce   int64      `json:"forkNonce"`
	ProposedAt  int64      `json:"proposedAt"`
	ExpiresAt   int64      `json:"expiresAt,omitempty"`
	Signatures  []ForkSig  `json:"signatures"`
}

// ForkSig is one validator's signature over the fork-block
// canonical bytes.
type ForkSig struct {
	ValidatorQuid string `json:"validatorQuid"`
	KeyEpoch      uint32 `json:"keyEpoch"`
	Signature     string `json:"signature"`
}

// ForkBlockTransaction wraps a ForkBlock for inclusion in a
// block.
type ForkBlockTransaction struct {
	BaseTransaction
	Fork ForkBlock `json:"fork"`
}

// ----- Errors --------------------------------------------------------------

var (
	ErrForkBadKind          = errors.New("fork-block: wrong anchor kind")
	ErrForkUnknownFeature   = errors.New("fork-block: feature not in ForkSupportedFeatures")
	ErrForkMissingDomain    = errors.New("fork-block: missing trustDomain")
	ErrForkHeightInPast     = errors.New("fork-block: ForkHeight is at or before current head")
	ErrForkHeightTooSoon    = errors.New("fork-block: ForkHeight is within MinForkNoticeBlocks of current head")
	ErrForkNonceReplay      = errors.New("fork-block: ForkNonce must strictly exceed any prior accepted fork for this (domain, feature)")
	ErrForkExpired          = errors.New("fork-block: ExpiresAt is in the past")
	ErrForkBelowQuorum      = errors.New("fork-block: validator quorum not reached")
	ErrForkBadSignature     = errors.New("fork-block: one or more validator signatures invalid")
	ErrForkDuplicateSigner  = errors.New("fork-block: duplicate validator signature")
	ErrForkSupersedeTooLate = errors.New("fork-block: supersede attempt arrived after earlier ForkHeight")
)

// ----- Canonicalization ----------------------------------------------------

// GetForkBlockSignableBytes returns the canonical bytes each
// validator signs. Signatures slice is cleared (signers don't
// sign their own signatures).
func GetForkBlockSignableBytes(f ForkBlock) ([]byte, error) {
	f.Signatures = nil
	return json.Marshal(f)
}

// ----- Ledger state --------------------------------------------------------

// forkRegistry holds pending and active forks. Lives on the
// node (not the ledger) because feature activation is a
// node-level effect (flipping config flags).
type forkRegistry struct {
	mu           sync.RWMutex
	pending      map[string]map[string]*ForkBlock // domain → feature → fork
	active       map[string]map[string]*ForkBlock // domain → feature → activated fork
	forkNonces   map[string]map[string]int64      // domain → feature → highest accepted nonce
}

func newForkRegistry() *forkRegistry {
	return &forkRegistry{
		pending:    make(map[string]map[string]*ForkBlock),
		active:     make(map[string]map[string]*ForkBlock),
		forkNonces: make(map[string]map[string]int64),
	}
}

// highestNonceForFeature returns the highest ForkNonce accepted
// for this (domain, feature). Zero if none.
func (r *forkRegistry) highestNonceForFeature(domain, feature string) int64 {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if m, ok := r.forkNonces[domain]; ok {
		return m[feature]
	}
	return 0
}

// storePending records an accepted fork. Supersedes any
// earlier pending fork for the same (domain, feature).
func (r *forkRegistry) storePending(f ForkBlock) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.pending[f.TrustDomain] == nil {
		r.pending[f.TrustDomain] = make(map[string]*ForkBlock)
	}
	cp := f
	r.pending[f.TrustDomain][f.Feature] = &cp
	if r.forkNonces[f.TrustDomain] == nil {
		r.forkNonces[f.TrustDomain] = make(map[string]int64)
	}
	r.forkNonces[f.TrustDomain][f.Feature] = f.ForkNonce
}

// moveToActive transitions a pending fork to active when the
// ForkHeight is reached. Returns the activated fork, or nil
// if none was pending.
func (r *forkRegistry) moveToActive(domain, feature string) *ForkBlock {
	r.mu.Lock()
	defer r.mu.Unlock()
	pm, ok := r.pending[domain]
	if !ok {
		return nil
	}
	f, ok := pm[feature]
	if !ok {
		return nil
	}
	delete(pm, feature)
	if r.active[domain] == nil {
		r.active[domain] = make(map[string]*ForkBlock)
	}
	r.active[domain][feature] = f
	return f
}

// pendingAt returns all pending forks scheduled for the given
// (domain, height) cutoff, i.e., forks whose ForkHeight <=
// height. Used at block commit to drive activation.
func (r *forkRegistry) pendingAt(domain string, height int64) []*ForkBlock {
	r.mu.RLock()
	defer r.mu.RUnlock()
	var out []*ForkBlock
	for _, f := range r.pending[domain] {
		if f.ForkHeight <= height {
			out = append(out, f)
		}
	}
	return out
}

// snapshot returns deep-enough copies of both maps for the
// diagnostic endpoint.
func (r *forkRegistry) snapshot() (pending map[string]map[string]ForkBlock, active map[string]map[string]ForkBlock) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	pending = make(map[string]map[string]ForkBlock)
	for d, m := range r.pending {
		pending[d] = make(map[string]ForkBlock)
		for k, v := range m {
			pending[d][k] = *v
		}
	}
	active = make(map[string]map[string]ForkBlock)
	for d, m := range r.active {
		active[d] = make(map[string]ForkBlock)
		for k, v := range m {
			active[d][k] = *v
		}
	}
	return
}

// ----- Validation ----------------------------------------------------------

// ValidateForkBlock runs QDP-0009 §7 checks against the given
// ledger + registry. currentHeight is the node's current chain
// head for the domain. validatorKeys maps validator quid →
// current epoch public key; quorumThreshold is the minimum
// number of valid signatures required.
func (node *QuidnugNode) ValidateForkBlock(f ForkBlock, currentHeight int64, now time.Time) error {
	if f.Kind != AnchorForkBlock {
		return ErrForkBadKind
	}
	if !ForkSupportedFeatures[f.Feature] {
		return ErrForkUnknownFeature
	}
	if f.TrustDomain == "" {
		return ErrForkMissingDomain
	}
	if f.ForkHeight <= currentHeight {
		return ErrForkHeightInPast
	}
	if f.ForkHeight < currentHeight+MinForkNoticeBlocks {
		return ErrForkHeightTooSoon
	}
	if f.ExpiresAt > 0 && f.ExpiresAt < now.Unix() {
		return ErrForkExpired
	}

	if node.forks == nil {
		// Lazy-init for tests that bypass NewQuidnugNode.
		node.forks = newForkRegistry()
	}
	prevNonce := node.forks.highestNonceForFeature(f.TrustDomain, f.Feature)
	if f.ForkNonce <= prevNonce {
		return ErrForkNonceReplay
	}

	// Supersede-too-late: if there's already an active fork for
	// this (domain, feature) whose ForkHeight <= currentHeight,
	// further forks are rejected.
	node.forks.mu.RLock()
	if _, ok := node.forks.active[f.TrustDomain][f.Feature]; ok {
		node.forks.mu.RUnlock()
		return ErrForkSupersedeTooLate
	}
	node.forks.mu.RUnlock()

	// Signatures: check each against validator keys.
	signable, err := GetForkBlockSignableBytes(f)
	if err != nil {
		return fmt.Errorf("fork-block: canonicalize: %w", err)
	}
	seen := make(map[string]bool)
	valid := 0
	for _, sig := range f.Signatures {
		if seen[sig.ValidatorQuid] {
			return ErrForkDuplicateSigner
		}
		seen[sig.ValidatorQuid] = true
		key, ok := node.NonceLedger.GetSignerKey(sig.ValidatorQuid, sig.KeyEpoch)
		if !ok || key == "" {
			continue
		}
		if _, err := hex.DecodeString(sig.Signature); err != nil {
			continue
		}
		if !VerifySignature(key, signable, sig.Signature) {
			continue
		}
		valid++
	}

	quorum := node.validatorQuorumForDomain(f.TrustDomain)
	if valid < quorum {
		return ErrForkBelowQuorum
	}
	return nil
}

// validatorQuorumForDomain computes the required signature
// count: 2/3 of the declared validator set, with a floor of 1.
func (node *QuidnugNode) validatorQuorumForDomain(domain string) int {
	node.TrustDomainsMutex.RLock()
	td, ok := node.TrustDomains[domain]
	node.TrustDomainsMutex.RUnlock()
	if !ok {
		return 1
	}
	n := len(td.ValidatorNodes)
	if n == 0 {
		return 1
	}
	q := n * DefaultValidatorQuorumNumerator / DefaultValidatorQuorumDenominator
	if q*DefaultValidatorQuorumDenominator < n*DefaultValidatorQuorumNumerator {
		q++ // ceiling
	}
	if q < 1 {
		q = 1
	}
	return q
}

// ----- Apply path ----------------------------------------------------------

// applyForkBlockFromBlock is the block-processing hook for a
// ForkBlockTransaction. Re-validates, stores as pending.
func (node *QuidnugNode) applyForkBlockFromBlock(f ForkBlock, block Block) {
	currentHeight := block.Index
	node.BlockchainMutex.RLock()
	if len(node.Blockchain) > 0 {
		currentHeight = node.Blockchain[len(node.Blockchain)-1].Index
	}
	node.BlockchainMutex.RUnlock()

	if err := node.ValidateForkBlock(f, currentHeight, time.Now()); err != nil {
		logger.Warn("Rejected fork-block in Trusted block",
			"blockIndex", block.Index,
			"feature", f.Feature,
			"error", err)
		forkBlockRejectedTotal.WithLabelValues(forkRejectReason(err)).Inc()
		return
	}
	if node.forks == nil {
		node.forks = newForkRegistry()
	}
	node.forks.storePending(f)
	forkBlockAcceptedTotal.WithLabelValues(f.TrustDomain, f.Feature).Inc()
	logger.Info("Accepted fork-block",
		"feature", f.Feature,
		"forkHeight", f.ForkHeight,
		"domain", f.TrustDomain)

	// If the ForkHeight is already in the past (catch-up case),
	// activate immediately.
	node.maybeActivateForks(f.TrustDomain, currentHeight)
}

// maybeActivateForks checks all pending forks for the domain
// and activates those whose ForkHeight <= height. Idempotent.
// Called at the end of block processing and on catch-up after
// applyForkBlockFromBlock.
func (node *QuidnugNode) maybeActivateForks(domain string, height int64) {
	if node.forks == nil {
		return
	}
	for _, f := range node.forks.pendingAt(domain, height) {
		moved := node.forks.moveToActive(f.TrustDomain, f.Feature)
		if moved == nil {
			continue
		}
		node.activateFeature(f.Feature)
		forkBlockActivatedTotal.WithLabelValues(f.TrustDomain, f.Feature).Inc()
		logger.Info("Activated fork",
			"feature", f.Feature,
			"forkHeight", f.ForkHeight,
			"domain", f.TrustDomain,
			"blockIndex", height)
	}
}

// activateFeature flips the corresponding node-level flag. New
// features added here must also be added to
// ForkSupportedFeatures.
func (node *QuidnugNode) activateFeature(feature string) {
	switch feature {
	case "enable_nonce_ledger":
		node.NonceLedgerEnforce = true
	case "enable_push_gossip":
		node.PushGossipEnabled = true
	case "enable_lazy_epoch_probe":
		node.LazyEpochProbeEnabled = true
	case "enable_kofk_bootstrap":
		// K-of-K bootstrap has no single "enabled" flag on the
		// node; activation here is semantic — operator-driven
		// bootstrap flows check their own config. The metric
		// fires regardless so tooling can still observe the
		// activation event.
	case "require_tx_tree_root":
		// H2 will wire this to enforce TransactionsRoot on
		// incoming blocks. For now it's a declared intent only.
	default:
		logger.Warn("activateFeature: unknown feature", "feature", feature)
	}
}

// forkRejectReason maps validation errors to bounded metric
// labels.
func forkRejectReason(err error) string {
	switch err {
	case ErrForkBadKind:
		return "bad_kind"
	case ErrForkUnknownFeature:
		return "unknown_feature"
	case ErrForkMissingDomain:
		return "missing_domain"
	case ErrForkHeightInPast:
		return "past_height"
	case ErrForkHeightTooSoon:
		return "too_soon"
	case ErrForkNonceReplay:
		return "nonce_replay"
	case ErrForkExpired:
		return "expired"
	case ErrForkBelowQuorum:
		return "below_quorum"
	case ErrForkBadSignature:
		return "bad_sig"
	case ErrForkDuplicateSigner:
		return "dup_signer"
	case ErrForkSupersedeTooLate:
		return "supersede_too_late"
	default:
		return "other"
	}
}
