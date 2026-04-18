package core

import (
	"errors"
	"fmt"
	"sync"
)

// NonceKey identifies a per-signer monotonic nonce stream.
//
// Per QDP-0003, the authoritative key is (Quid, Domain, Epoch); the
// "Domain" component is included here so the same structure will serve
// both QDP-0001's single-domain semantics (Domain left empty) and the
// QDP-0003 per-domain semantics (Domain populated). Until QDP-0003's
// domain propagation is wired up, callers SHOULD populate Domain — this
// is checked but not currently required.
type NonceKey struct {
	Quid   string
	Domain string
	Epoch  uint32
}

// NonceCheckpoint is the per-block summary of nonce advancement caused
// by that block, as defined in QDP-0001 §6.1.3. Every sealed block
// attaches one checkpoint per distinct (signer, epoch) present in its
// transactions.
type NonceCheckpoint struct {
	Quid     string `json:"quid"`
	Domain   string `json:"domain,omitempty"`
	Epoch    uint32 `json:"epoch"`
	MaxNonce int64  `json:"maxNonce"`
}

// Rejection reasons returned by NonceLedger.Admit. Callers compare with
// errors.Is.
var (
	ErrNonceReplay       = errors.New("nonce: replayed (accepted already ≥ provided)")
	ErrNonceReserved     = errors.New("nonce: reserved by a tentative block")
	ErrNonceGapTooLarge  = errors.New("nonce: gap exceeds MaxNonceGap")
	ErrNonceNotMonotonic = errors.New("nonce: anchor-nonce must strictly increase")
	ErrNonceEpochStale   = errors.New("nonce: transaction key-epoch is stale")
	ErrNonceEpochUnknown = errors.New("nonce: transaction key-epoch exceeds current")
	ErrNonceInvalidInput = errors.New("nonce: invalid input")
	ErrNonceEpochFrozen  = errors.New("nonce: transaction key-epoch has been invalidated")
	ErrNonceEpochCapped  = errors.New("nonce: transaction exceeds epoch cap")
)

// DefaultMaxNonceGap is the maximum advance any single transaction may
// make over the last-accepted nonce for its (signer, epoch) key. Bounds
// the damage from a one-off key compromise that submits an artificially
// high nonce. See QDP-0001 §3.7 and §6.2.
const DefaultMaxNonceGap = int64(1024)

// NonceLedger is the in-memory authoritative nonce-advancement structure
// described in QDP-0001 §6.1.2. It is deliberately small and opinionated:
// the authoritative state lives in the blockchain; this structure is a
// rebuilt cache with O(1) validation queries.
//
// The ledger is concurrent-safe. All mutators take an exclusive lock;
// reads use the RLock path via the methods that start with Get. The
// single-mutex design is acceptable up to ~100k signers per domain per
// node; sharding is a follow-up optimization (QDP-0001 §8.1).
type NonceLedger struct {
	mu sync.RWMutex

	// accepted[key] is the max nonce observed for `key` in the Trusted
	// chain. This advances only via CommitAccepted.
	accepted map[NonceKey]int64

	// tentative[key] is the max nonce observed for `key` in any block
	// currently at Trusted OR Tentative acceptance. Reads of this are
	// used to block a transaction whose nonce was already reserved by a
	// tentative block. Reserving here does NOT commit to accepted.
	tentative map[NonceKey]int64

	// currentEpoch[quid] is the active key-epoch for a signer's
	// identity. Advanced by AnchorRotation via ApplyAnchor.
	currentEpoch map[string]uint32

	// lastAnchorNonce[quid] is the strictly-monotonic anchor-nonce for
	// the signer, advanced by any applied anchor.
	lastAnchorNonce map[string]int64

	// signerKeys[quid][epoch] is the hex-encoded P-256 public key that
	// the signer has authorized for a given epoch. Populated by
	// SetSignerKey (from identity records and anchor processing).
	// Consulted by ValidateAnchor to verify anchor signatures.
	signerKeys map[string]map[uint32]string

	// epochCaps[quid][epoch] is the maximum nonce accepted for the
	// given (signer, epoch). Set by AnchorEpochCap/Invalidation/
	// Rotation's MaxAcceptedOldNonce. int64 zero value means "no cap".
	epochCaps map[string]map[uint32]int64

	// frozenEpochs[quid][epoch] records an AnchorInvalidation. When
	// present, no new transaction at that epoch is admitted regardless
	// of the cap.
	frozenEpochs map[string]map[uint32]bool

	// guardianSets[subject] is the declared guardian set for a
	// subject's recovery authority. Installed by AnchorGuardianSetUpdate
	// (QDP-0002 §6.4.4).
	guardianSets map[string]*GuardianSet

	// pendingRecoveries[subject] tracks an in-flight guardian recovery
	// (Init → delay → Commit/Veto). Terminal states are retained for
	// traceability; a new Init transitions the previous record to
	// RecoveryReplaced.
	pendingRecoveries map[string]*PendingRecovery

	// latestFingerprints[domain] is the most recent DomainFingerprint
	// we've received or produced for each domain. Used by
	// AnchorGossipMessage validation (QDP-0003 §7.3) to confirm that
	// an allegedly-cross-domain block is really the one the origin
	// domain has committed.
	latestFingerprints map[string]DomainFingerprint

	// seenGossipMessages is a bounded dedup set for cross-domain
	// anchor gossip (QDP-0003 §6.4). Keyed by MessageID, value is
	// the unix timestamp at which we first observed the message;
	// entries older than DomainFingerprintRetention are pruned
	// opportunistically.
	seenGossipMessages map[string]int64

	// guardianResignations[subject] is the append-only list of
	// resignations submitted against the subject's guardian set.
	// QDP-0006 (H6). Consulted by EffectiveGuardianSet at read
	// time; the installed guardianSets are never mutated by
	// resignations, so the audit trail is preserved.
	guardianResignations map[string][]GuardianResignation

	// guardianResignationNonces[guardian][subject] is the highest
	// ResignationNonce accepted for the pair. Replay protection
	// for resignations.
	guardianResignationNonces map[string]map[string]int64

	maxNonceGap int64
}

// NewNonceLedger creates an empty ledger.
func NewNonceLedger() *NonceLedger {
	return &NonceLedger{
		accepted:           make(map[NonceKey]int64),
		tentative:          make(map[NonceKey]int64),
		currentEpoch:       make(map[string]uint32),
		lastAnchorNonce:    make(map[string]int64),
		signerKeys:         make(map[string]map[uint32]string),
		epochCaps:          make(map[string]map[uint32]int64),
		frozenEpochs:       make(map[string]map[uint32]bool),
		guardianSets:       make(map[string]*GuardianSet),
		pendingRecoveries:  make(map[string]*PendingRecovery),
		latestFingerprints:        make(map[string]DomainFingerprint),
		seenGossipMessages:        make(map[string]int64),
		guardianResignations:      make(map[string][]GuardianResignation),
		guardianResignationNonces: make(map[string]map[string]int64),
		maxNonceGap:               DefaultMaxNonceGap,
	}
}

// SetMaxNonceGap overrides the default nonce-gap cap. Value must be
// positive; ≤ 0 is silently coerced to DefaultMaxNonceGap.
func (l *NonceLedger) SetMaxNonceGap(gap int64) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if gap <= 0 {
		l.maxNonceGap = DefaultMaxNonceGap
	} else {
		l.maxNonceGap = gap
	}
}

// CurrentEpoch returns the active key-epoch for a signer, or 0 if no
// anchor has yet advanced it.
func (l *NonceLedger) CurrentEpoch(quid string) uint32 {
	l.mu.RLock()
	defer l.mu.RUnlock()
	return l.currentEpoch[quid]
}

// Accepted returns the max accepted nonce for the given key, or 0 if
// none has been observed.
func (l *NonceLedger) Accepted(key NonceKey) int64 {
	l.mu.RLock()
	defer l.mu.RUnlock()
	return l.accepted[key]
}

// Tentative returns the max tentatively-observed nonce for the given
// key. Always ≥ Accepted for the same key (invariant I2 in QDP-0001
// §11.1).
func (l *NonceLedger) Tentative(key NonceKey) int64 {
	l.mu.RLock()
	defer l.mu.RUnlock()
	return l.tentative[key]
}

// Admit performs the replay-safety check for a single transaction,
// as specified in QDP-0001 §6.2. It is read-only: it does not modify
// the ledger. Callers should pair Admit with a subsequent
// ReserveTentative once the transaction is admitted to the mempool.
//
// The epoch argument is the transaction's declared `KeyEpoch`; it is
// checked against the signer's current epoch. If epoch is zero and the
// signer has no recorded epoch change, the check passes. Future anchor
// work will expand this.
func (l *NonceLedger) Admit(key NonceKey, nonce int64) error {
	if key.Quid == "" {
		return ErrNonceInvalidInput
	}
	if nonce <= 0 {
		return ErrNonceInvalidInput
	}

	l.mu.RLock()
	defer l.mu.RUnlock()

	activeEpoch, hasEpoch := l.currentEpoch[key.Quid]
	switch {
	case !hasEpoch && key.Epoch == 0:
		// No recorded epoch; accept epoch 0 as implicit current.
	case hasEpoch && key.Epoch == activeEpoch:
		// Matches current.
	case hasEpoch && key.Epoch < activeEpoch:
		return ErrNonceEpochStale
	case hasEpoch && key.Epoch > activeEpoch:
		return ErrNonceEpochUnknown
	case !hasEpoch && key.Epoch != 0:
		return ErrNonceEpochUnknown
	}

	// Anchor-imposed limits for this epoch.
	if m, ok := l.frozenEpochs[key.Quid]; ok && m[key.Epoch] {
		return ErrNonceEpochFrozen
	}
	if m, ok := l.epochCaps[key.Quid]; ok {
		if cap := m[key.Epoch]; cap > 0 && nonce > cap {
			return ErrNonceEpochCapped
		}
	}

	if accepted, ok := l.accepted[key]; ok && nonce <= accepted {
		return ErrNonceReplay
	}
	if tentative, ok := l.tentative[key]; ok && nonce <= tentative {
		return ErrNonceReserved
	}

	lastSeen := l.accepted[key]
	if lastSeen == 0 {
		lastSeen = l.tentative[key]
	}
	if lastSeen > 0 && nonce-lastSeen > l.maxNonceGap {
		return ErrNonceGapTooLarge
	}

	return nil
}

// ReserveTentative records that a nonce has been observed in a
// tentative-tier block (or admitted into the mempool). Reservation
// blocks any other transaction from using the same nonce for the same
// key until either the reservation is promoted to accepted or released.
//
// It is safe to call ReserveTentative with a nonce ≤ tentative[key]; the
// call is a no-op in that case.
func (l *NonceLedger) ReserveTentative(key NonceKey, nonce int64) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if cur, ok := l.tentative[key]; !ok || nonce > cur {
		l.tentative[key] = nonce
	}
}

// ReleaseTentative lowers the tentative[key] back to at most `nonce`.
// Intended for the demotion path: when a tentative block is pruned and
// no other tentative block references this key at an equal or higher
// nonce, tentative reverts to the accepted baseline (or whatever the
// caller determined is now the high-water mark).
//
// Passing nonce < accepted[key] is silently clamped; the ledger never
// rewinds past accepted.
func (l *NonceLedger) ReleaseTentative(key NonceKey, nonce int64) {
	l.mu.Lock()
	defer l.mu.Unlock()
	floor := l.accepted[key]
	if nonce < floor {
		nonce = floor
	}
	if cur, ok := l.tentative[key]; !ok || nonce < cur {
		if nonce == 0 && floor == 0 {
			delete(l.tentative, key)
		} else {
			l.tentative[key] = nonce
		}
	}
}

// CommitAccepted advances the authoritative accepted counter for a key
// to at most `nonce`. It also raises the tentative water-mark if the
// newly-accepted nonce is higher. This is called during Trusted block
// application (QDP-0001 §6.4).
//
// Like ReserveTentative, it is idempotent and monotonic: calls with a
// lower nonce than the current accepted[key] are no-ops.
func (l *NonceLedger) CommitAccepted(key NonceKey, nonce int64) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if cur, ok := l.accepted[key]; !ok || nonce > cur {
		l.accepted[key] = nonce
	}
	if cur, ok := l.tentative[key]; !ok || nonce > cur {
		l.tentative[key] = nonce
	}
}

// ApplyCheckpoints processes a slice of per-block checkpoints produced
// by sealing. trustedCommit == true advances accepted; false only
// reserves tentatively (corresponding to QDP-0001 §6.4's table).
func (l *NonceLedger) ApplyCheckpoints(checkpoints []NonceCheckpoint, trustedCommit bool) {
	if len(checkpoints) == 0 {
		return
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	for _, c := range checkpoints {
		key := NonceKey{Quid: c.Quid, Domain: c.Domain, Epoch: c.Epoch}
		if trustedCommit {
			if cur, ok := l.accepted[key]; !ok || c.MaxNonce > cur {
				l.accepted[key] = c.MaxNonce
			}
		}
		if cur, ok := l.tentative[key]; !ok || c.MaxNonce > cur {
			l.tentative[key] = c.MaxNonce
		}
	}
}

// Snapshot returns a deep copy of the accepted map. Used by snapshot
// production (future work) and by tests that need to assert ledger
// state without racing the mutator.
func (l *NonceLedger) Snapshot() map[NonceKey]int64 {
	l.mu.RLock()
	defer l.mu.RUnlock()
	out := make(map[NonceKey]int64, len(l.accepted))
	for k, v := range l.accepted {
		out[k] = v
	}
	return out
}

// Size returns the number of tracked (signer, domain, epoch) keys.
// Used by metrics and tests.
func (l *NonceLedger) Size() (accepted, tentative int) {
	l.mu.RLock()
	defer l.mu.RUnlock()
	return len(l.accepted), len(l.tentative)
}

// nonceRejectionReason maps a ledger error to a short, prometheus-safe
// label for quidnug_nonce_replay_rejections_total.
func nonceRejectionReason(err error) string {
	switch {
	case errors.Is(err, ErrNonceReplay):
		return "replay"
	case errors.Is(err, ErrNonceReserved):
		return "reserved"
	case errors.Is(err, ErrNonceGapTooLarge):
		return "gap"
	case errors.Is(err, ErrNonceEpochStale):
		return "epoch_stale"
	case errors.Is(err, ErrNonceEpochUnknown):
		return "epoch_unknown"
	case errors.Is(err, ErrNonceInvalidInput):
		return "invalid_input"
	case errors.Is(err, ErrNonceEpochFrozen):
		return "epoch_frozen"
	case errors.Is(err, ErrNonceEpochCapped):
		return "epoch_capped"
	default:
		return "other"
	}
}

// ----- Anchor state accessors ----------------------------------------------

// SetSignerKey records the hex-encoded P-256 public key authorized for
// (quid, epoch). Safe to call multiple times with the same value; a
// different value for the same (quid, epoch) silently overwrites — the
// caller is responsible for anchor-driven invariants, not this.
func (l *NonceLedger) SetSignerKey(quid string, epoch uint32, pubkeyHex string) {
	if quid == "" {
		return
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	if _, ok := l.signerKeys[quid]; !ok {
		l.signerKeys[quid] = make(map[uint32]string)
	}
	l.signerKeys[quid][epoch] = pubkeyHex
}

// GetSignerKey returns the hex-encoded public key recorded for
// (quid, epoch), or ("", false) if none.
func (l *NonceLedger) GetSignerKey(quid string, epoch uint32) (string, bool) {
	l.mu.RLock()
	defer l.mu.RUnlock()
	if m, ok := l.signerKeys[quid]; ok {
		k, ok := m[epoch]
		return k, ok
	}
	return "", false
}

// LastAnchorNonce returns the strictly-monotonic anchor-nonce counter
// for a signer, or 0 if no anchor has applied to them.
func (l *NonceLedger) LastAnchorNonce(quid string) int64 {
	l.mu.RLock()
	defer l.mu.RUnlock()
	return l.lastAnchorNonce[quid]
}

// EpochCap returns the maximum nonce allowed for (quid, epoch). A
// return of 0 means no cap has been applied; Admit treats that as
// "unlimited except by MaxNonceGap."
func (l *NonceLedger) EpochCap(quid string, epoch uint32) int64 {
	l.mu.RLock()
	defer l.mu.RUnlock()
	if m, ok := l.epochCaps[quid]; ok {
		return m[epoch]
	}
	return 0
}

// IsEpochInvalidated returns true if an AnchorInvalidation has been
// applied to (quid, epoch).
func (l *NonceLedger) IsEpochInvalidated(quid string, epoch uint32) bool {
	l.mu.RLock()
	defer l.mu.RUnlock()
	if m, ok := l.frozenEpochs[quid]; ok {
		return m[epoch]
	}
	return false
}

// ApplyAnchor installs an anchor's effects into the ledger. The anchor
// must already have been accepted into a Trusted block and validated
// with ValidateAnchor. Returns ErrNonceNotMonotonic if the anchor-nonce
// check has slipped through somehow; otherwise nil.
//
// Effects by kind:
//
//   - Rotation: currentEpoch[quid] = ToEpoch; signerKeys[quid][ToEpoch] =
//     NewPublicKey; epochCaps[quid][FromEpoch] = MaxAcceptedOldNonce.
//     lastAnchorNonce advanced.
//   - Invalidation: frozenEpochs[quid][FromEpoch] = true;
//     epochCaps[quid][FromEpoch] = MaxAcceptedOldNonce. lastAnchorNonce
//     advanced.
//   - EpochCap: epochCaps[quid][FromEpoch] = MaxAcceptedOldNonce.
//     lastAnchorNonce advanced.
func (l *NonceLedger) ApplyAnchor(a NonceAnchor) error {
	l.mu.Lock()
	defer l.mu.Unlock()

	if a.SignerQuid == "" {
		return ErrNonceInvalidInput
	}
	if a.AnchorNonce <= l.lastAnchorNonce[a.SignerQuid] {
		return ErrNonceNotMonotonic
	}

	switch a.Kind {
	case AnchorRotation:
		l.currentEpoch[a.SignerQuid] = a.ToEpoch
		if _, ok := l.signerKeys[a.SignerQuid]; !ok {
			l.signerKeys[a.SignerQuid] = make(map[uint32]string)
		}
		l.signerKeys[a.SignerQuid][a.ToEpoch] = a.NewPublicKey

		if a.MaxAcceptedOldNonce > 0 {
			if _, ok := l.epochCaps[a.SignerQuid]; !ok {
				l.epochCaps[a.SignerQuid] = make(map[uint32]int64)
			}
			l.epochCaps[a.SignerQuid][a.FromEpoch] = a.MaxAcceptedOldNonce
		}
	case AnchorInvalidation:
		if _, ok := l.frozenEpochs[a.SignerQuid]; !ok {
			l.frozenEpochs[a.SignerQuid] = make(map[uint32]bool)
		}
		l.frozenEpochs[a.SignerQuid][a.FromEpoch] = true

		if _, ok := l.epochCaps[a.SignerQuid]; !ok {
			l.epochCaps[a.SignerQuid] = make(map[uint32]int64)
		}
		if a.MaxAcceptedOldNonce > 0 {
			l.epochCaps[a.SignerQuid][a.FromEpoch] = a.MaxAcceptedOldNonce
		}
	case AnchorEpochCap:
		if _, ok := l.epochCaps[a.SignerQuid]; !ok {
			l.epochCaps[a.SignerQuid] = make(map[uint32]int64)
		}
		if a.MaxAcceptedOldNonce > 0 {
			l.epochCaps[a.SignerQuid][a.FromEpoch] = a.MaxAcceptedOldNonce
		}
	default:
		return fmt.Errorf("ApplyAnchor: unknown kind %d", a.Kind)
	}

	l.lastAnchorNonce[a.SignerQuid] = a.AnchorNonce
	return nil
}
