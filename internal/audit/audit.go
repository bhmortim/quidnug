// Package audit implements the QDP-0018 Phase 1 tamper-evident
// operator audit log.
//
// A standalone package (no dependency on internal/core) so the
// log primitive can be unit-tested in isolation and reused by
// any future tooling that needs to append structured operator
// events to the same file format.
//
// The log is conceptually similar to RFC 6962 Certificate
// Transparency: an append-only, hash-chained sequence of
// structured entries. Tamper-evidence comes from two layers:
//
//   - Intra-log: each entry's `PrevHash` commits to the prior
//     entry's `Hash`, so any retroactive edit invalidates every
//     subsequent entry.
//   - On-chain: periodically (Phase 3) the log's head hash is
//     committed as an `AUDIT_ANCHOR` EventTransaction, so an
//     auditor pinned to an older anchor can detect any later
//     log rewrite.
//
// Phase 1 ships the in-memory + disk-backed log; Phase 3 adds
// anchor publishing; Phase 4 exposes HTTP audit endpoints (the
// basic head+entries endpoints are already wired in
// internal/core so early operators can start inspecting their
// logs).
package audit

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
	"sync"
	"time"
)

// Entry categories (QDP-0018 §3.3). Stable enum; new categories
// land via QDP amendment, not ad-hoc operator labels.
const (
	CategoryConfigChange     = "CONFIG_CHANGE"
	CategoryValidatorEdit    = "VALIDATOR_EDIT"
	CategoryPeerChange       = "PEER_CHANGE"
	CategoryKeyRotation      = "KEY_ROTATION"
	CategoryModerationAction = "MODERATION_ACTION"
	CategoryGovernanceVote   = "GOVERNANCE_VOTE"
	CategoryNodeLifecycle    = "NODE_LIFECYCLE"
	CategorySigningQuorum    = "SIGNING_QUORUM"
	CategoryAbuseResponse    = "ABUSE_RESPONSE"
	CategoryDSRFulfillment   = "DSR_FULFILLMENT"
	CategoryForkDecision     = "FORK_DECISION"
	CategoryOperatorOther    = "OPERATOR_OTHER"
)

// validCategories is the set of accepted category strings.
var validCategories = map[string]struct{}{
	CategoryConfigChange:     {},
	CategoryValidatorEdit:    {},
	CategoryPeerChange:       {},
	CategoryKeyRotation:      {},
	CategoryModerationAction: {},
	CategoryGovernanceVote:   {},
	CategoryNodeLifecycle:    {},
	CategorySigningQuorum:    {},
	CategoryAbuseResponse:    {},
	CategoryDSRFulfillment:   {},
	CategoryForkDecision:     {},
	CategoryOperatorOther:    {},
}

// ZeroPrevHash is the placeholder for Entry 0's PrevHash field.
// Consumers can assert against this constant to recognize the
// log's genesis entry.
const ZeroPrevHash = "0000000000000000000000000000000000000000000000000000000000000000"

// MaxNoteLength bounds the human-readable note field to prevent
// the append-only file from being bloated by a runaway
// logger.
const MaxNoteLength = 1024

// Entry is one record in the audit log. Canonical shape matches
// docs/design/0018-observability-and-audit.md §3.2.
type Entry struct {
	// Sequence is monotonic per-log; first entry is 0.
	Sequence int64 `json:"sequence"`
	// PrevHash is the Hash of the prior entry, or ZeroPrevHash
	// for entry 0.
	PrevHash string `json:"prevHash"`
	// Timestamp is nanoseconds since Unix epoch when the entry
	// was recorded. UnixNano precision so rapid-fire entries
	// stay ordered in the log even when two share a Unix-second.
	Timestamp int64 `json:"timestamp"`
	// OperatorQuid is the quid the emitting node considers its
	// operator identity. Plumbed in from QuidnugNode.
	OperatorQuid string `json:"operatorQuid"`
	// Category is one of the Category* constants.
	Category string `json:"category"`
	// Payload is an arbitrary structured body keyed by category.
	// See QDP-0018 §3.4 for per-category examples.
	Payload map[string]interface{} `json:"payload"`
	// Note is an optional human-readable comment. Capped at
	// MaxNoteLength chars.
	Note string `json:"note,omitempty"`
	// Hash is sha256 of the canonical-JSON of all preceding
	// fields (everything in the struct except Hash itself).
	// The library fills this in on Append.
	Hash string `json:"hash"`
}

// entrySignableFields is the tuple that feeds the sha256 that
// becomes Entry.Hash. Kept as its own type so the canonicalizer
// is obvious.
type entrySignableFields struct {
	Sequence     int64                  `json:"sequence"`
	PrevHash     string                 `json:"prevHash"`
	Timestamp    int64                  `json:"timestamp"`
	OperatorQuid string                 `json:"operatorQuid"`
	Category     string                 `json:"category"`
	Payload      map[string]interface{} `json:"payload"`
	Note         string                 `json:"note,omitempty"`
}

// computeEntryHash returns the sha256 of the canonical JSON
// of the signable fields. Payload keys are sorted so the hash
// is stable across Go map-iteration randomness.
func computeEntryHash(e Entry) (string, error) {
	// Clone the payload through a sorted re-serialization so
	// iteration order doesn't leak into the hash.
	stablePayload, err := stableMarshalMap(e.Payload)
	if err != nil {
		return "", fmt.Errorf("stable-encode payload: %w", err)
	}

	// Re-unmarshal so json.Marshal downstream keeps stable order.
	var stableMap map[string]interface{}
	if len(stablePayload) > 0 {
		if err := json.Unmarshal(stablePayload, &stableMap); err != nil {
			return "", fmt.Errorf("re-unmarshal stable payload: %w", err)
		}
	}

	signable := entrySignableFields{
		Sequence:     e.Sequence,
		PrevHash:     e.PrevHash,
		Timestamp:    e.Timestamp,
		OperatorQuid: e.OperatorQuid,
		Category:     e.Category,
		Payload:      stableMap,
		Note:         e.Note,
	}
	raw, err := json.Marshal(signable)
	if err != nil {
		return "", fmt.Errorf("marshal signable: %w", err)
	}
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:]), nil
}

// stableMarshalMap JSON-encodes m with its keys sorted
// alphabetically. Nested maps are NOT recursively sorted in
// Phase 1 — payloads that carry nested maps will still hash
// stably as long as each map's key iteration order is
// deterministic within a single Marshal call, which Go's
// encoding/json guarantees for string-keyed maps. The
// top-level sort is enough for the simple category-keyed
// payloads QDP-0018 §3.4 describes.
func stableMarshalMap(m map[string]interface{}) ([]byte, error) {
	if len(m) == 0 {
		return []byte("{}"), nil
	}
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	ordered := make(map[string]json.RawMessage, len(m))
	for _, k := range keys {
		v, err := json.Marshal(m[k])
		if err != nil {
			return nil, err
		}
		ordered[k] = v
	}
	// Even json.Marshal of a map writes keys in sorted order
	// for Go ≥ 1.12 (stdlib doc: "map keys are sorted"), so
	// this re-pass is a belt-and-braces guarantee.
	return json.Marshal(ordered)
}

// Log is the in-memory append-only audit log. Owns its own
// lock. Callers wire it to a disk-backed Store (see store.go)
// so entries survive process restarts.
type Log struct {
	mu           sync.RWMutex
	entries      []Entry
	operatorQuid string
	store        Store // optional; if nil, the log is memory-only
}

// Store is the persistence interface Log uses to flush entries
// to disk. Implementations must be safe for concurrent use by
// a single Log (the Log serializes calls internally).
type Store interface {
	// Append writes one entry to the persistent tail.
	Append(e Entry) error
	// Load reads every persisted entry in order for replay on
	// startup.
	Load() ([]Entry, error)
	// Close releases any underlying resources.
	Close() error
}

// NewLog constructs an in-memory log. The operatorQuid is
// stamped on every entry; pass the node's operator-identity quid.
func NewLog(operatorQuid string) *Log {
	return &Log{
		operatorQuid: operatorQuid,
	}
}

// NewLogWithStore constructs a log backed by the given Store.
// On construction the store is loaded and its contents replay
// into the in-memory index.
func NewLogWithStore(operatorQuid string, store Store) (*Log, error) {
	l := &Log{
		operatorQuid: operatorQuid,
		store:        store,
	}
	if store != nil {
		persisted, err := store.Load()
		if err != nil {
			return nil, fmt.Errorf("load audit store: %w", err)
		}
		l.entries = persisted
	}
	return l, nil
}

// Append records a new entry. Sequence and PrevHash and Hash
// are filled in automatically; callers provide Category,
// Payload, and optional Note. Timestamp defaults to time.Now()
// if zero.
//
// Returns the completed Entry so callers can inspect the
// assigned sequence + hash.
func (l *Log) Append(e Entry) (Entry, error) {
	if err := validateIncomingEntry(e); err != nil {
		return Entry{}, err
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	// Fill in metadata.
	e.Sequence = int64(len(l.entries))
	if e.Timestamp == 0 {
		e.Timestamp = time.Now().UnixNano()
	}
	if e.OperatorQuid == "" {
		e.OperatorQuid = l.operatorQuid
	}
	if e.Sequence == 0 {
		e.PrevHash = ZeroPrevHash
	} else {
		e.PrevHash = l.entries[e.Sequence-1].Hash
	}

	hash, err := computeEntryHash(e)
	if err != nil {
		return Entry{}, fmt.Errorf("compute hash: %w", err)
	}
	e.Hash = hash

	// Persist before in-memory commit so a crash after Append
	// won't leave the memory log ahead of the disk.
	if l.store != nil {
		if err := l.store.Append(e); err != nil {
			return Entry{}, fmt.Errorf("persist entry: %w", err)
		}
	}
	l.entries = append(l.entries, e)
	return e, nil
}

// validateIncomingEntry enforces what the caller MUST supply
// (category + payload). The library sets the rest.
func validateIncomingEntry(e Entry) error {
	if _, ok := validCategories[e.Category]; !ok {
		return fmt.Errorf("unknown audit category %q", e.Category)
	}
	if e.Payload == nil {
		// Allow empty-but-present payloads; nil is disallowed
		// so the hash is stable.
		return fmt.Errorf("audit entry payload must not be nil (use empty map)")
	}
	if len(e.Note) > MaxNoteLength {
		return fmt.Errorf("audit entry note exceeds %d chars", MaxNoteLength)
	}
	return nil
}

// Head returns the most recent entry (zero Entry + false if
// the log is empty).
func (l *Log) Head() (Entry, bool) {
	l.mu.RLock()
	defer l.mu.RUnlock()
	if len(l.entries) == 0 {
		return Entry{}, false
	}
	return l.entries[len(l.entries)-1], true
}

// Height returns the number of entries in the log.
func (l *Log) Height() int64 {
	l.mu.RLock()
	defer l.mu.RUnlock()
	return int64(len(l.entries))
}

// Get returns a specific entry by sequence. Second return is
// false if the sequence is out of range.
func (l *Log) Get(sequence int64) (Entry, bool) {
	l.mu.RLock()
	defer l.mu.RUnlock()
	if sequence < 0 || sequence >= int64(len(l.entries)) {
		return Entry{}, false
	}
	return l.entries[sequence], true
}

// EntriesSince returns entries with sequence > since (i.e. the
// response starts at `since+1`), capped at `limit`. The caller
// typically passes the sequence of the last entry they've
// already ingested, so pagination is a forward cursor walk.
//
// Returns a fresh slice so mutation doesn't affect the log.
func (l *Log) EntriesSince(since, limit int64) []Entry {
	l.mu.RLock()
	defer l.mu.RUnlock()
	start := since + 1
	if start < 0 {
		start = 0
	}
	if start >= int64(len(l.entries)) {
		return nil
	}
	end := start + limit
	if limit <= 0 || end > int64(len(l.entries)) {
		end = int64(len(l.entries))
	}
	out := make([]Entry, end-start)
	copy(out, l.entries[start:end])
	return out
}

// VerifyChain walks the log and checks that every entry's
// PrevHash matches the prior entry's Hash and that every
// entry's self-hash matches its declared field set. Returns
// the first sequence where tampering is detected, or -1 if
// the chain is intact.
//
// Intended for startup replay, periodic self-audit, and the
// external CLI verifier (QDP-0018 §5.1).
func (l *Log) VerifyChain() (int64, error) {
	l.mu.RLock()
	defer l.mu.RUnlock()

	var prevHash string
	for i, e := range l.entries {
		want := ZeroPrevHash
		if i > 0 {
			want = prevHash
		}
		if e.PrevHash != want {
			return int64(i), fmt.Errorf(
				"entry %d: prevHash mismatch (have %s, want %s)",
				i, e.PrevHash, want)
		}
		// Re-compute the hash from the canonical fields.
		probe := e
		probe.Hash = ""
		expected, err := computeEntryHash(probe)
		if err != nil {
			return int64(i), err
		}
		if e.Hash != expected {
			return int64(i), fmt.Errorf(
				"entry %d: hash mismatch (stored %s, computed %s)",
				i, e.Hash, expected)
		}
		prevHash = e.Hash
	}
	return -1, nil
}

// OperatorQuid returns the operator identity this log was
// constructed with.
func (l *Log) OperatorQuid() string {
	return l.operatorQuid
}

// Close releases the underlying store. Callers should call
// Close on graceful shutdown so the file handle releases
// cleanly.
func (l *Log) Close() error {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.store != nil {
		return l.store.Close()
	}
	return nil
}
