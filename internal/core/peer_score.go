// Per-peer quality scoring.
//
// Phase 4 of the peering plan: every interaction with a peer
// (handshake, gossip post, query, broadcast, validation outcome)
// nudges that peer's composite score. The score is a single
// 0.0-1.0 number that other subsystems consult to:
//
//   - Quarantine peers whose score crosses peer_quarantine_threshold
//     (kept in KnownNodes but excluded from routing). Phase 4b.
//   - Evict peers whose score stays below peer_eviction_threshold
//     for peer_eviction_grace. Phase 4b.
//   - Sort routing candidates so high-quality peers get traffic
//     first (gossip, queries, broadcasts). Phase 4d.
//
// This file is Phase 4a — the scoring core: PeerScore struct,
// the Recorder API the rest of the codebase calls into, and the
// persistence loop that snapshots scores to data_dir/peer_scores.json
// every 5 min so reputation survives restart.
//
// Design constraints:
//
//   - The hot-path (Record(...)) MUST be fast and contention-cheap.
//     Sharded mutex keyed by NodeQuid keeps contention bounded
//     even under 1000-peer fan-out.
//   - Decay is lazy: we don't tick a global timer. When Record is
//     called, the affected counter computes elapsed-since-LastTick
//     and applies the decay before adding the new event. This is
//     O(1) and accurate to the same millisecond resolution as
//     time.Now().
//   - Severe events (fork claims, signature fails, ad revocations)
//     accumulate without decay. They represent Byzantine-or-
//     compromised signals that shouldn't fade.
//
// Composite score (defaults; tunable via config):
//
//	weighted = 0.35*validation + 0.20*handshake + 0.20*query
//	         + 0.15*gossip    + 0.10*broadcast
//	severe   = 0.20*forkClaims + 0.10*sigFails + 0.30*adRevocations
//	composite = max(0, min(1, weighted - severe))
//
// where each per-class score is successes / (successes + failures),
// or 0.5 (neutral) when the class has zero events.
package core

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"sync"
	"time"

	"github.com/quidnug/quidnug/internal/safeio"
)

// EventClass is the kind of interaction that the scoring system
// records. Each class has its own decaying success/failure
// counter so we can distinguish "peer's network is flaky" from
// "peer's data fails validation."
type EventClass string

const (
	EventClassHandshake  EventClass = "handshake"
	EventClassGossip     EventClass = "gossip"
	EventClassQuery      EventClass = "query"
	EventClassBroadcast  EventClass = "broadcast"
	EventClassValidation EventClass = "validation"
)

// SevereEvent is the discriminant for non-decaying penalties.
type SevereEvent string

const (
	SevereForkClaim    SevereEvent = "fork-claim"
	SevereSignatureFail SevereEvent = "signature-fail"
	SevereAdRevocation  SevereEvent = "ad-revocation"
)

// EventCounter is a pair of exponentially-decayed accumulators
// for one EventClass. Decay is applied lazily on Record/Snapshot
// — we never run a background ticker.
type EventCounter struct {
	Successes float64       `json:"successes"`
	Failures  float64       `json:"failures"`
	HalfLife  time.Duration `json:"-"` // tunable but rarely changed
	LastTick  time.Time     `json:"lastTick"`
}

// PeerScoreEvent is one record in the per-peer ring buffer
// surfaced to operators via /api/v1/peers/{nodeQuid}.
type PeerScoreEvent struct {
	Timestamp time.Time `json:"timestamp"`
	Class     string    `json:"class"`
	OK        bool      `json:"ok"`
	Severe    string    `json:"severe,omitempty"`
	Note      string    `json:"note,omitempty"`
}

// PeerScore is the per-peer state. Mutex-protected; any caller
// that wants a consistent snapshot must call Snapshot() rather
// than reading fields directly.
type PeerScore struct {
	mu sync.RWMutex

	NodeQuid string `json:"nodeQuid"`

	// AdmittedAt is when this peer first entered KnownNodes.
	// Useful for "how long have I trusted you" diagnostics.
	AdmittedAt time.Time `json:"admittedAt"`

	// LastUpdated is the wall-clock time of the last Record call.
	LastUpdated time.Time `json:"lastUpdated"`

	// Per-class counters.
	Handshake  EventCounter `json:"handshake"`
	Gossip     EventCounter `json:"gossip"`
	Query      EventCounter `json:"query"`
	Broadcast  EventCounter `json:"broadcast"`
	Validation EventCounter `json:"validation"`

	// Severe-event totals (cumulative, no decay).
	ForkClaims     int `json:"forkClaims"`
	SignatureFails int `json:"signatureFails"`
	AdRevocations  int `json:"adRevocations"`

	// Bounded ring buffer of recent events. Used by the
	// /api/v1/peers/{nodeQuid} endpoint and by `quidnug-cli
	// peer show` for diagnostics.
	events    []PeerScoreEvent // ring; len capped at PeerScoreEventRingSize
	eventHead int              // next-write index

	// Quarantined captures the peer's quarantine state set by
	// the Phase 4b eviction loop. Stored here (not in
	// KnownNodes) so it survives KnownNodes mutations and is
	// uniformly readable by routing/eviction code.
	Quarantined        bool      `json:"quarantined"`
	QuarantinedAt      time.Time `json:"quarantinedAt,omitempty"`
	QuarantineReason   string    `json:"quarantineReason,omitempty"`
	BelowEvictionSince time.Time `json:"belowEvictionSince,omitempty"`
}

// PeerScoreEventRingSize is the cap on the per-peer event ring.
// 50 is enough for operator post-mortem and small enough to keep
// the persistence file bounded even with thousands of peers.
const PeerScoreEventRingSize = 50

// PeerScoreWeights is the weighted aggregation config for the
// composite score. Defaults match the audit document; operators
// can override via config.
type PeerScoreWeights struct {
	Validation float64
	Handshake  float64
	Query      float64
	Gossip     float64
	Broadcast  float64

	// Severe-event subtractions per occurrence. Composite
	// is floored at 0 regardless.
	ForkClaim     float64
	SignatureFail float64
	AdRevocation  float64
}

// DefaultPeerScoreWeights returns the audit-document defaults.
// Sums to 1.0 across the five classes so a peer with all-success
// rates of 1.0 hits a 1.0 weighted aggregate.
func DefaultPeerScoreWeights() PeerScoreWeights {
	return PeerScoreWeights{
		Validation: 0.35,
		Handshake:  0.20,
		Query:      0.20,
		Gossip:     0.15,
		Broadcast:  0.10,

		ForkClaim:     0.20,
		SignatureFail: 0.10,
		AdRevocation:  0.30,
	}
}

// DefaultEventHalfLife is how fast successes/failures decay.
// 15 minutes means a transient outage's impact is largely gone
// in an hour; sustained issues continue to drag the score down.
const DefaultEventHalfLife = 15 * time.Minute

// PeerScoreSnapshot is a JSON-marshalable view of a PeerScore.
// Used by the persistence layer and the API endpoints. Mirrors
// PeerScore field-for-field minus the mutex and ring internals.
type PeerScoreSnapshot struct {
	NodeQuid string    `json:"nodeQuid"`
	Composite float64  `json:"composite"`

	AdmittedAt  time.Time `json:"admittedAt"`
	LastUpdated time.Time `json:"lastUpdated"`

	Handshake  EventCounter `json:"handshake"`
	Gossip     EventCounter `json:"gossip"`
	Query      EventCounter `json:"query"`
	Broadcast  EventCounter `json:"broadcast"`
	Validation EventCounter `json:"validation"`

	ForkClaims     int `json:"forkClaims"`
	SignatureFails int `json:"signatureFails"`
	AdRevocations  int `json:"adRevocations"`

	Quarantined        bool      `json:"quarantined"`
	QuarantinedAt      time.Time `json:"quarantinedAt,omitempty"`
	QuarantineReason   string    `json:"quarantineReason,omitempty"`
	BelowEvictionSince time.Time `json:"belowEvictionSince,omitempty"`

	RecentEvents []PeerScoreEvent `json:"recentEvents,omitempty"`
}

// applyDecay updates the counter to reflect time elapsed since
// LastTick. Safe to call repeatedly; idempotent within the same
// instant. Caller holds the parent PeerScore.mu in write mode.
func (ec *EventCounter) applyDecay(now time.Time) {
	if ec.LastTick.IsZero() {
		ec.LastTick = now
		if ec.HalfLife <= 0 {
			ec.HalfLife = DefaultEventHalfLife
		}
		return
	}
	if ec.HalfLife <= 0 {
		ec.HalfLife = DefaultEventHalfLife
	}
	dt := now.Sub(ec.LastTick)
	if dt <= 0 {
		return
	}
	// Decay factor = 0.5^(dt/halfLife). math.Exp2 is faster
	// than math.Pow(0.5, x).
	factor := math.Exp2(-float64(dt) / float64(ec.HalfLife))
	ec.Successes *= factor
	ec.Failures *= factor
	ec.LastTick = now
}

// rate returns the Laplace-smoothed (Beta(1,1) prior) success
// rate. With zero events the result is 0.5 (neutral); with many
// events it converges to successes/(successes+failures); under
// exponential decay (which shrinks both counters proportionally)
// the smoothing pulls confident-but-stale ratings back toward
// neutral, which matches the operator-intuitive semantic that
// "old data shouldn't keep a peer's halo alive forever."
func (ec *EventCounter) rate() float64 {
	return (ec.Successes + 1.0) / (ec.Successes + ec.Failures + 2.0)
}

// counterFor returns a pointer to the EventCounter for class.
// Caller holds the parent PeerScore mutex.
func (s *PeerScore) counterFor(class EventClass) *EventCounter {
	switch class {
	case EventClassHandshake:
		return &s.Handshake
	case EventClassGossip:
		return &s.Gossip
	case EventClassQuery:
		return &s.Query
	case EventClassBroadcast:
		return &s.Broadcast
	case EventClassValidation:
		return &s.Validation
	default:
		return nil
	}
}

// Composite returns the current composite score using the
// supplied weights. Read-only; safe to call concurrently with
// Record() because the read lock is acquired internally.
func (s *PeerScore) Composite(w PeerScoreWeights) float64 {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.compositeLocked(w)
}

// compositeLocked is the no-lock implementation. Caller holds
// s.mu (read or write).
func (s *PeerScore) compositeLocked(w PeerScoreWeights) float64 {
	now := time.Now()
	// Compute rates. We don't apply decay to a copy because
	// the mutation is fine — but we may not hold the write
	// lock. Compute decay-adjusted rate inline without
	// mutating.
	rateOf := func(ec EventCounter) float64 {
		if ec.LastTick.IsZero() {
			return 0.5
		}
		hl := ec.HalfLife
		if hl <= 0 {
			hl = DefaultEventHalfLife
		}
		dt := now.Sub(ec.LastTick)
		factor := math.Exp2(-float64(dt) / float64(hl))
		s := ec.Successes * factor
		f := ec.Failures * factor
		// Laplace smoothing — same logic as
		// EventCounter.rate(), inlined so we don't have to
		// allocate a copy of the EventCounter.
		return (s + 1.0) / (s + f + 2.0)
	}
	weighted := w.Validation*rateOf(s.Validation) +
		w.Handshake*rateOf(s.Handshake) +
		w.Query*rateOf(s.Query) +
		w.Gossip*rateOf(s.Gossip) +
		w.Broadcast*rateOf(s.Broadcast)
	severe := w.ForkClaim*float64(s.ForkClaims) +
		w.SignatureFail*float64(s.SignatureFails) +
		w.AdRevocation*float64(s.AdRevocations)
	c := weighted - severe
	if c < 0 {
		c = 0
	}
	if c > 1 {
		c = 1
	}
	return c
}

// snapshotLocked builds an immutable view. Caller holds s.mu
// (read or write).
func (s *PeerScore) snapshotLocked(w PeerScoreWeights) PeerScoreSnapshot {
	out := PeerScoreSnapshot{
		NodeQuid:           s.NodeQuid,
		Composite:          s.compositeLocked(w),
		AdmittedAt:         s.AdmittedAt,
		LastUpdated:        s.LastUpdated,
		Handshake:          s.Handshake,
		Gossip:             s.Gossip,
		Query:              s.Query,
		Broadcast:          s.Broadcast,
		Validation:         s.Validation,
		ForkClaims:         s.ForkClaims,
		SignatureFails:     s.SignatureFails,
		AdRevocations:      s.AdRevocations,
		Quarantined:        s.Quarantined,
		QuarantinedAt:      s.QuarantinedAt,
		QuarantineReason:   s.QuarantineReason,
		BelowEvictionSince: s.BelowEvictionSince,
	}
	// Copy ring contents in chronological order. Two cases:
	//   * Buffer not yet full: events are in slice order from
	//     index 0 to len(events)-1.
	//   * Buffer full: events form a ring; oldest is at
	//     eventHead, newest is at eventHead-1 mod len.
	n := len(s.events)
	if n == 0 {
		return out
	}
	out.RecentEvents = make([]PeerScoreEvent, 0, n)
	if n < PeerScoreEventRingSize {
		out.RecentEvents = append(out.RecentEvents, s.events...)
		return out
	}
	idx := s.eventHead
	for i := 0; i < n; i++ {
		out.RecentEvents = append(out.RecentEvents, s.events[idx])
		idx = (idx + 1) % n
	}
	return out
}

// recordEventLocked appends to the ring. Caller holds s.mu in
// write mode.
//
// Until the slice reaches PeerScoreEventRingSize entries, we
// just append; eventHead is meaningless because reads use
// slice order. Once full, eventHead points at the slot that
// will be overwritten next (also the oldest entry).
func (s *PeerScore) recordEventLocked(ev PeerScoreEvent) {
	if len(s.events) < PeerScoreEventRingSize {
		s.events = append(s.events, ev)
		// eventHead stays at 0; we'll set it the first
		// time we wrap.
		return
	}
	s.events[s.eventHead] = ev
	s.eventHead = (s.eventHead + 1) % PeerScoreEventRingSize
}

// PeerScoreboard is the per-node container. Indexed by NodeQuid;
// scores persist across the lifetime of the node process and,
// via the persistence loop, across restarts.
type PeerScoreboard struct {
	mu      sync.RWMutex
	scores  map[string]*PeerScore
	weights PeerScoreWeights
	// Persistence
	persistPath     string
	persistInterval time.Duration
}

// NewPeerScoreboard constructs a scoreboard. Pass empty
// persistPath to disable persistence (useful in tests).
func NewPeerScoreboard(weights PeerScoreWeights, persistPath string, persistInterval time.Duration) *PeerScoreboard {
	if persistInterval <= 0 {
		persistInterval = 5 * time.Minute
	}
	return &PeerScoreboard{
		scores:          make(map[string]*PeerScore),
		weights:         weights,
		persistPath:     persistPath,
		persistInterval: persistInterval,
	}
}

// getOrCreate returns the score record for nodeQuid, creating
// it if absent. Caller need not hold the scoreboard lock.
func (sb *PeerScoreboard) getOrCreate(nodeQuid string) *PeerScore {
	sb.mu.RLock()
	if p, ok := sb.scores[nodeQuid]; ok {
		sb.mu.RUnlock()
		return p
	}
	sb.mu.RUnlock()
	sb.mu.Lock()
	defer sb.mu.Unlock()
	if p, ok := sb.scores[nodeQuid]; ok {
		return p
	}
	now := time.Now()
	p := &PeerScore{
		NodeQuid:    nodeQuid,
		AdmittedAt:  now,
		LastUpdated: now,
	}
	sb.scores[nodeQuid] = p
	return p
}

// Record applies one observation to the peer's score. Class
// MUST be one of the EventClass constants. ok=true is a
// successful interaction; ok=false a failure. note is a
// short free-form description that lands in the event ring.
//
// Hot-path safe: O(1), no I/O, single peer-level lock.
func (sb *PeerScoreboard) Record(nodeQuid string, class EventClass, ok bool, note string) {
	if nodeQuid == "" {
		return
	}
	p := sb.getOrCreate(nodeQuid)
	now := time.Now()
	p.mu.Lock()
	defer p.mu.Unlock()
	c := p.counterFor(class)
	if c == nil {
		return
	}
	c.applyDecay(now)
	if ok {
		c.Successes += 1
	} else {
		c.Failures += 1
	}
	p.LastUpdated = now
	p.recordEventLocked(PeerScoreEvent{
		Timestamp: now,
		Class:     string(class),
		OK:        ok,
		Note:      note,
	})
}

// RecordSevere increments a non-decaying severe-event counter.
// Used for fork claims, signature failures, ad revocations —
// signals that a peer is Byzantine, compromised, or has lost
// operator attestation.
func (sb *PeerScoreboard) RecordSevere(nodeQuid string, kind SevereEvent, note string) {
	if nodeQuid == "" {
		return
	}
	p := sb.getOrCreate(nodeQuid)
	now := time.Now()
	p.mu.Lock()
	defer p.mu.Unlock()
	switch kind {
	case SevereForkClaim:
		p.ForkClaims++
	case SevereSignatureFail:
		p.SignatureFails++
	case SevereAdRevocation:
		p.AdRevocations++
	default:
		return
	}
	p.LastUpdated = now
	p.recordEventLocked(PeerScoreEvent{
		Timestamp: now,
		Severe:    string(kind),
		Note:      note,
	})
}

// SetAdmittedAt is called by the admit pipeline when a peer
// first enters KnownNodes. Idempotent: only updates if the
// existing record has zero AdmittedAt (i.e., the score record
// was created lazily via Record before AdmitPeer ran).
func (sb *PeerScoreboard) SetAdmittedAt(nodeQuid string, t time.Time) {
	if nodeQuid == "" {
		return
	}
	p := sb.getOrCreate(nodeQuid)
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.AdmittedAt.IsZero() {
		p.AdmittedAt = t
	}
}

// Snapshot returns a sorted slice of all peer snapshots,
// ascending by composite score (worst first — what operators
// want to see). Read-only; safe to call concurrently with
// Record().
func (sb *PeerScoreboard) Snapshot() []PeerScoreSnapshot {
	sb.mu.RLock()
	keys := make([]string, 0, len(sb.scores))
	for k := range sb.scores {
		keys = append(keys, k)
	}
	sb.mu.RUnlock()
	out := make([]PeerScoreSnapshot, 0, len(keys))
	for _, k := range keys {
		p := sb.lookup(k)
		if p == nil {
			continue
		}
		p.mu.RLock()
		out = append(out, p.snapshotLocked(sb.weights))
		p.mu.RUnlock()
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].Composite < out[j].Composite
	})
	return out
}

// SnapshotOne returns the snapshot for a single peer, or nil
// if no record exists.
func (sb *PeerScoreboard) SnapshotOne(nodeQuid string) *PeerScoreSnapshot {
	p := sb.lookup(nodeQuid)
	if p == nil {
		return nil
	}
	p.mu.RLock()
	defer p.mu.RUnlock()
	out := p.snapshotLocked(sb.weights)
	return &out
}

// Composite returns the current composite for nodeQuid. Returns
// 0.5 (neutral) when no record exists, matching the cold-start
// behavior described in the audit.
func (sb *PeerScoreboard) Composite(nodeQuid string) float64 {
	p := sb.lookup(nodeQuid)
	if p == nil {
		return 0.5
	}
	return p.Composite(sb.weights)
}

// IsQuarantined returns true if the peer is currently
// quarantined. Used by routing code (Phase 4d) to filter
// candidate lists.
func (sb *PeerScoreboard) IsQuarantined(nodeQuid string) bool {
	p := sb.lookup(nodeQuid)
	if p == nil {
		return false
	}
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.Quarantined
}

// SetQuarantined transitions a peer's quarantine state.
// Returns the prior state. Used by the Phase 4b eviction loop
// (and operator-CLI commands once they exist).
func (sb *PeerScoreboard) SetQuarantined(nodeQuid string, quarantined bool, reason string) bool {
	p := sb.getOrCreate(nodeQuid)
	now := time.Now()
	p.mu.Lock()
	defer p.mu.Unlock()
	prior := p.Quarantined
	p.Quarantined = quarantined
	if quarantined {
		p.QuarantinedAt = now
		p.QuarantineReason = reason
	} else {
		p.QuarantinedAt = time.Time{}
		p.QuarantineReason = ""
	}
	return prior
}

// MarkBelowEviction is called by the eviction loop to start (or
// extend, or clear) the grace clock for peers below the
// eviction threshold. Returns the time the peer first dropped
// below threshold, useful for logging.
func (sb *PeerScoreboard) MarkBelowEviction(nodeQuid string, below bool) time.Time {
	p := sb.getOrCreate(nodeQuid)
	now := time.Now()
	p.mu.Lock()
	defer p.mu.Unlock()
	if below && p.BelowEvictionSince.IsZero() {
		p.BelowEvictionSince = now
	} else if !below {
		p.BelowEvictionSince = time.Time{}
	}
	return p.BelowEvictionSince
}

// Forget drops a peer's record entirely. Called when the peer
// is evicted from KnownNodes — we keep a few seconds of grace
// before forgetting so a re-admit can resume the score, but the
// final cleanup happens here.
//
// Forget is also used by tests to reset state between cases.
func (sb *PeerScoreboard) Forget(nodeQuid string) {
	sb.mu.Lock()
	defer sb.mu.Unlock()
	delete(sb.scores, nodeQuid)
}

// lookup returns the score record for nodeQuid without
// creating one. Concurrent-safe.
func (sb *PeerScoreboard) lookup(nodeQuid string) *PeerScore {
	sb.mu.RLock()
	defer sb.mu.RUnlock()
	return sb.scores[nodeQuid]
}

// --- Persistence ----------------------------------------------------------
//
// Scores are written to data_dir/peer_scores.json every
// persistInterval (default 5 min). Atomic write through safeio
// so a crash mid-flush can't corrupt the file.

type persistedScoreboard struct {
	Version int                 `json:"version"`
	SavedAt time.Time           `json:"savedAt"`
	Scores  []PeerScoreSnapshot `json:"scores"`
}

const peerScoreboardSchemaVersion = 1

// persistOnce writes the current scoreboard to disk. Used by
// the periodic loop and (tests can call directly).
func (sb *PeerScoreboard) persistOnce() error {
	if sb.persistPath == "" {
		return nil
	}
	doc := persistedScoreboard{
		Version: peerScoreboardSchemaVersion,
		SavedAt: time.Now().UTC(),
		Scores:  sb.Snapshot(),
	}
	body, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal peer scoreboard: %w", err)
	}
	body = append(body, '\n')
	if err := safeio.WriteFileMode(sb.persistPath, body, 0o600); err != nil {
		return fmt.Errorf("write %s: %w", sb.persistPath, err)
	}
	return nil
}

// runPeerScorePersistLoop is the per-node ticker that flushes
// the scoreboard to disk every interval. Designed to live as a
// dedicated goroutine spawned from Run(). Idempotent no-op when
// path is empty.
func (node *QuidnugNode) runPeerScorePersistLoop(ctx context.Context, path string, interval time.Duration) {
	if path == "" || node.PeerScoreboard == nil {
		return
	}
	if interval <= 0 {
		interval = 5 * time.Minute
	}
	// Update the scoreboard's persistPath in case Run() has a
	// different path than NewQuidnugNode received (e.g. a
	// chrooted layout).
	node.PeerScoreboard.persistPath = path
	tk := time.NewTicker(interval)
	defer tk.Stop()
	for {
		select {
		case <-ctx.Done():
			// Final flush before shutdown so the next
			// boot has the latest reputation snapshot.
			if err := node.PeerScoreboard.persistOnce(); err != nil {
				if logger != nil {
					logger.Warn("Final peer scoreboard flush failed", "error", err)
				}
			}
			return
		case <-tk.C:
			if err := node.PeerScoreboard.persistOnce(); err != nil {
				if logger != nil {
					logger.Warn("Peer scoreboard flush failed", "error", err)
				}
			}
		}
	}
}

// recordPeerScore is the node-side convenience method the
// rest of core/ calls into. Unlike PeerScoreboard.Record, this
// tolerates a nil scoreboard (typical in tests) so callers
// don't need to nil-check at every site.
func (node *QuidnugNode) recordPeerScore(nodeQuid string, class EventClass, ok bool, note string) {
	if node == nil || node.PeerScoreboard == nil || nodeQuid == "" {
		return
	}
	node.PeerScoreboard.Record(nodeQuid, class, ok, note)
}

// recordPeerSevere is the node-side convenience for severe
// (non-decaying) events. Same nil-tolerance as recordPeerScore.
func (node *QuidnugNode) recordPeerSevere(nodeQuid string, kind SevereEvent, note string) {
	if node == nil || node.PeerScoreboard == nil || nodeQuid == "" {
		return
	}
	node.PeerScoreboard.RecordSevere(nodeQuid, kind, note)
}

// LoadFrom rehydrates the scoreboard from disk. Missing file is
// treated as a clean start (no error). Used at boot.
func (sb *PeerScoreboard) LoadFrom(path string) error {
	if path == "" {
		return nil
	}
	raw, err := safeio.ReadFile(path)
	if err != nil {
		// Missing file is fine — first boot, or operator
		// wiped the data dir. Return nil so callers can
		// distinguish "real error" from "no prior state".
		return nil
	}
	var doc persistedScoreboard
	if err := json.Unmarshal(raw, &doc); err != nil {
		return fmt.Errorf("parse peer scoreboard: %w", err)
	}
	if doc.Version != peerScoreboardSchemaVersion {
		// Schema bump → we don't try to migrate; operators
		// regain reputation as peers re-interact.
		return fmt.Errorf("peer scoreboard: schema version %d not supported", doc.Version)
	}
	sb.mu.Lock()
	defer sb.mu.Unlock()
	for _, s := range doc.Scores {
		p := &PeerScore{
			NodeQuid:           s.NodeQuid,
			AdmittedAt:         s.AdmittedAt,
			LastUpdated:        s.LastUpdated,
			Handshake:          s.Handshake,
			Gossip:             s.Gossip,
			Query:              s.Query,
			Broadcast:          s.Broadcast,
			Validation:         s.Validation,
			ForkClaims:         s.ForkClaims,
			SignatureFails:     s.SignatureFails,
			AdRevocations:      s.AdRevocations,
			Quarantined:        s.Quarantined,
			QuarantinedAt:      s.QuarantinedAt,
			QuarantineReason:   s.QuarantineReason,
			BelowEvictionSince: s.BelowEvictionSince,
		}
		// Restore the event ring. We don't preserve the
		// circular write-head; reload effectively appends.
		for _, ev := range s.RecentEvents {
			p.recordEventLocked(ev)
		}
		sb.scores[s.NodeQuid] = p
	}
	return nil
}
