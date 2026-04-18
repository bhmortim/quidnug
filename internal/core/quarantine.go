// Package core — quarantine.go
//
// Lazy epoch propagation quarantine (QDP-0007 / H4).
//
// A transaction whose signer's local epoch state is older than
// EpochRecencyWindow is held here pending a fingerprint probe
// against the signer's home domain. Probes run asynchronously.
// Release triggers:
//
//   - Probe returns a fresh fingerprint (signer state refreshed).
//   - Push gossip for the signer arrives (also refreshes
//     recency — evidence of a live path).
//   - Operator manually marks the signer recent.
//
// Drop triggers:
//
//   - Age-out: entry older than QuarantineMaxAge.
//   - Overflow: quarantine hit QuarantineMaxSize; oldest dropped
//     to make room (never the newest — keeps the flood-attack
//     blast radius bounded).
//   - Probe timeout with ProbeTimeoutPolicyReject policy.
//
// Design notes:
//
//   - In-memory only. Like PendingTxs, quarantine is lost on
//     restart. QDP-0007 §14.2 flagged this as a conscious
//     trade-off: persistence is a separate concern, not a H4
//     blocker.
//
//   - One queue per node. Per-signer sub-queues would give finer
//     release granularity but add complexity. Release scans the
//     whole queue and pulls matching-signer entries — O(N) per
//     release but N is bounded by QuarantineMaxSize.
package core

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"sync"
	"time"
)

// ----- Constants -----------------------------------------------------------

const (
	// DefaultEpochRecencyWindow is how long we trust a cached
	// epoch before re-probing. 7d matches the QDP-0007 §3
	// default.
	DefaultEpochRecencyWindow = 7 * 24 * time.Hour

	// DefaultEpochProbeTimeout is the overall budget for
	// probe-the-home-domain. Aggregated across per-peer
	// attempts.
	DefaultEpochProbeTimeout = 30 * time.Second

	// DefaultEpochProbePeerTimeout is the per-peer HTTP
	// timeout during probe.
	DefaultEpochProbePeerTimeout = 5 * time.Second

	// DefaultQuarantineMaxSize bounds memory use. 1024 is
	// plenty for normal operation; a flood is visible via
	// the quarantine_size metric.
	DefaultQuarantineMaxSize = 1024

	// DefaultQuarantineMaxAge drops entries that have been
	// sitting too long. 1h matches the QDP-0007 §3 default.
	DefaultQuarantineMaxAge = 1 * time.Hour
)

// ProbeTimeoutPolicy values.
const (
	// ProbeTimeoutPolicyReject drops the tx when the probe
	// times out. Safer default — a rotation may have happened
	// and the probe is exactly what would catch it.
	ProbeTimeoutPolicyReject = "reject"

	// ProbeTimeoutPolicyAdmitWarn admits the tx anyway with a
	// metric + warning log. Permissive for deployments where
	// availability matters more than rotation coverage.
	ProbeTimeoutPolicyAdmitWarn = "admit_warn"
)

// ----- Types ---------------------------------------------------------------

// QuarantinedTx holds one held transaction plus the metadata
// needed to dispatch when clearance arrives.
type QuarantinedTx struct {
	Tx         interface{}
	TxHash     string    // stable id — sha256 of canonical marshal
	EnqueuedAt time.Time
	Signer     string
	HomeDomain string
	Retries    int
}

// QuarantineRelease describes the reason a set of txs were
// released. Used by callers to choose whether to re-admit.
type QuarantineRelease struct {
	Signer  string
	Trigger string // "probe" | "gossip" | "manual" | "age_out" | "overflow"
}

// QuarantineDropped is returned from ReleaseAged for metric
// emission.
type QuarantineDropped struct {
	Tx         interface{}
	Signer     string
	Reason     string
	EnqueuedAt time.Time
}

// ----- State ---------------------------------------------------------------

// quarantineState is the per-node quarantine queue. Lives as a
// QuidnugNode field (see node.go additions).
type quarantineState struct {
	mu sync.Mutex

	// byHash keyed on TxHash for O(1) dedup and lookup.
	byHash map[string]*QuarantinedTx

	// bySigner keyed on Signer quid for O(1) release-by-signer.
	// Values are lists of TxHash strings — the authoritative tx
	// record lives in byHash.
	bySigner map[string][]string

	maxSize int
	maxAge  time.Duration
}

func newQuarantineState() *quarantineState {
	return &quarantineState{
		byHash:   make(map[string]*QuarantinedTx),
		bySigner: make(map[string][]string),
		maxSize:  DefaultQuarantineMaxSize,
		maxAge:   DefaultQuarantineMaxAge,
	}
}

// ----- Operations ----------------------------------------------------------

// enqueue adds a transaction to the quarantine. If the hash is
// already present, it's a no-op (dedup via hash). If the queue
// is at capacity, the oldest entry is evicted first.
func (q *quarantineState) enqueue(tx QuarantinedTx) (inserted bool, evicted *QuarantinedTx) {
	q.mu.Lock()
	defer q.mu.Unlock()

	if _, exists := q.byHash[tx.TxHash]; exists {
		return false, nil
	}

	// Overflow: evict oldest BEFORE inserting so a flood can't
	// push capacity past the cap even momentarily.
	if len(q.byHash) >= q.maxSize {
		evicted = q.evictOldestLocked()
	}

	q.byHash[tx.TxHash] = &tx
	q.bySigner[tx.Signer] = append(q.bySigner[tx.Signer], tx.TxHash)
	return true, evicted
}

// evictOldestLocked finds and removes the oldest entry.
// Caller holds q.mu. Returns the evicted entry, or nil if
// empty.
func (q *quarantineState) evictOldestLocked() *QuarantinedTx {
	var oldestHash string
	var oldestTime time.Time
	for h, e := range q.byHash {
		if oldestHash == "" || e.EnqueuedAt.Before(oldestTime) {
			oldestHash = h
			oldestTime = e.EnqueuedAt
		}
	}
	if oldestHash == "" {
		return nil
	}
	ev := q.byHash[oldestHash]
	q.removeLocked(oldestHash)
	return ev
}

// removeLocked drops an entry from both indexes. Caller holds
// q.mu. Safe when the hash is not present.
func (q *quarantineState) removeLocked(txHash string) {
	entry, ok := q.byHash[txHash]
	if !ok {
		return
	}
	delete(q.byHash, txHash)
	hashes := q.bySigner[entry.Signer]
	for i, h := range hashes {
		if h == txHash {
			q.bySigner[entry.Signer] = append(hashes[:i], hashes[i+1:]...)
			break
		}
	}
	if len(q.bySigner[entry.Signer]) == 0 {
		delete(q.bySigner, entry.Signer)
	}
}

// releaseSigner returns and removes all entries for the named
// signer. Caller is responsible for re-admitting the returned
// transactions.
func (q *quarantineState) releaseSigner(signer string) []QuarantinedTx {
	q.mu.Lock()
	defer q.mu.Unlock()
	hashes := q.bySigner[signer]
	out := make([]QuarantinedTx, 0, len(hashes))
	for _, h := range hashes {
		if e, ok := q.byHash[h]; ok {
			out = append(out, *e)
		}
	}
	for _, h := range hashes {
		delete(q.byHash, h)
	}
	delete(q.bySigner, signer)
	return out
}

// releaseAged drops entries older than maxAge. Returns the
// list of dropped entries so the caller can emit metrics.
func (q *quarantineState) releaseAged(now time.Time) []QuarantineDropped {
	q.mu.Lock()
	defer q.mu.Unlock()
	cutoff := now.Add(-q.maxAge)
	var dropped []QuarantineDropped
	for h, e := range q.byHash {
		if e.EnqueuedAt.Before(cutoff) {
			dropped = append(dropped, QuarantineDropped{
				Tx:         e.Tx,
				Signer:     e.Signer,
				Reason:     "age_out",
				EnqueuedAt: e.EnqueuedAt,
			})
			q.removeLocked(h)
		}
	}
	return dropped
}

// size returns the current queue size. Thread-safe read.
func (q *quarantineState) size() int {
	q.mu.Lock()
	defer q.mu.Unlock()
	return len(q.byHash)
}

// ----- Hashing -------------------------------------------------------------

// quarantineTxHash produces a stable identifier for a
// transaction for quarantine dedup. sha256 over canonical JSON.
// Not cryptographically meaningful — just a collision-resistant
// ID. Callers who already have a tx hash should pass it
// straight through.
func quarantineTxHash(tx interface{}) string {
	b, err := json.Marshal(tx)
	if err != nil {
		// Fall back to timestamp — best-effort; duplicate
		// enqueues would miss dedup but that's not a correctness
		// issue, just a missed optimization.
		return "err-" + time.Now().Format(time.RFC3339Nano)
	}
	h := sha256.Sum256(b)
	return hex.EncodeToString(h[:])
}
