// Package core — bootstrap.go
//
// K-of-K snapshot bootstrap (QDP-0008 / H3). Lets a fresh node
// seed its nonce ledger from a consensus of K trusted seed
// peers rather than a single-peer trust decision.
//
// Invariants:
//
//   - All K peers must agree on BlockHash for the largest
//     response group. Any disagreement fails closed —
//     QuorumMissed state, operator sees which peers diverged.
//
//   - Peers with invalid signatures are excluded from the
//     quorum count. A byzantine peer can't mint votes by
//     re-sending other peers' snapshots with its own key —
//     each snapshot is signed by its stated producer.
//
//   - Trust list seeding occurs BEFORE snapshot validation.
//     Otherwise `VerifySnapshot` has no public keys to check
//     against and would reject everything.
//
//   - After seeding the node runs in shadow-verify for the
//     first N subsequent blocks. Divergence halts the node so
//     an operator can investigate before damage compounds.
package core

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"
)

// ----- Types ---------------------------------------------------------------

// BootstrapState is the session lifecycle state.
type BootstrapState int

const (
	BootstrapIdle BootstrapState = iota
	BootstrapFetching
	BootstrapQuorumMet
	BootstrapQuorumMissed
	BootstrapSeeding
	BootstrapShadowVerify
	BootstrapDone
	BootstrapFailed
)

func (s BootstrapState) String() string {
	switch s {
	case BootstrapIdle:
		return "idle"
	case BootstrapFetching:
		return "fetching"
	case BootstrapQuorumMet:
		return "quorum_met"
	case BootstrapQuorumMissed:
		return "quorum_missed"
	case BootstrapSeeding:
		return "seeding"
	case BootstrapShadowVerify:
		return "shadow_verify"
	case BootstrapDone:
		return "done"
	case BootstrapFailed:
		return "failed"
	}
	return "unknown"
}

// BootstrapTrustEntry is a single operator-asserted root trust
// entry used to bootstrap signature validation before the
// ledger knows any keys.
type BootstrapTrustEntry struct {
	Quid      string `json:"quid"`
	PublicKey string `json:"publicKey"`
}

// BootstrapSession tracks one bootstrap attempt against a
// named domain.
type BootstrapSession struct {
	Domain     string
	K          int
	State      BootstrapState
	PeerSet    []Node
	Responses  map[string]NonceSnapshot // peerID → snapshot
	Consensus  *NonceSnapshot            // chosen snapshot when state == QuorumMet or later
	Error      error
	Started    time.Time
	Ended      time.Time
	ShadowLeft int // blocks remaining in shadow-verify
}

// BootstrapConfig captures the session's tuning knobs.
type BootstrapConfig struct {
	Quorum          int
	PeerTimeout     time.Duration
	TotalTimeout    time.Duration
	HeightTolerance int
	StaleTolerance  time.Duration
	TrustedPeer     string
	ShadowBlocks    int
}

// DefaultBootstrapConfig returns the QDP-0008 §7.2 defaults.
func DefaultBootstrapConfig() BootstrapConfig {
	return BootstrapConfig{
		Quorum:          3,
		PeerTimeout:     5 * time.Second,
		TotalTimeout:    30 * time.Second,
		HeightTolerance: 4,
		StaleTolerance:  30 * 24 * time.Hour,
		ShadowBlocks:    64,
	}
}

// ----- Errors --------------------------------------------------------------

var (
	ErrBootstrapNoPeers            = errors.New("bootstrap: no peers known for domain")
	ErrBootstrapBelowQuorum        = errors.New("bootstrap: fewer than K peers responded with valid snapshots")
	ErrBootstrapDisagreement       = errors.New("bootstrap: K-of-K disagreement across peers")
	ErrBootstrapHeightTolerance    = errors.New("bootstrap: peers span more than heightTolerance")
	ErrBootstrapStale              = errors.New("bootstrap: all valid snapshots older than staleTolerance")
	ErrBootstrapInvalidOverride    = errors.New("bootstrap: BootstrapTrustedPeer set but that peer did not respond")
	ErrBootstrapDivergence         = errors.New("bootstrap: shadow-verify detected divergence from seed state")
	ErrBootstrapTrustListEmpty     = errors.New("bootstrap: trust list must contain at least K entries")
)

// ----- Session-level state -------------------------------------------------

// bootstrapRegistry is the per-node registry of in-flight and
// completed bootstrap sessions. Not persisted.
type bootstrapRegistry struct {
	mu             sync.RWMutex
	current        *BootstrapSession
	completed      map[string]time.Time // domain → completion time
}

func newBootstrapRegistry() *bootstrapRegistry {
	return &bootstrapRegistry{
		completed: make(map[string]time.Time),
	}
}

// ----- Bootstrap operation -------------------------------------------------

// BootstrapFromPeers runs a K-of-K bootstrap for the named
// domain. Returns the validated consensus snapshot on success,
// or an error describing why the quorum failed.
//
// Does NOT apply the snapshot to the ledger — that's the
// caller's responsibility. This separation lets tests verify
// the agreement logic without mutating state.
func (node *QuidnugNode) BootstrapFromPeers(ctx context.Context, domain string, cfg BootstrapConfig) (*BootstrapSession, error) {
	sess := &BootstrapSession{
		Domain:    domain,
		K:         cfg.Quorum,
		State:     BootstrapFetching,
		Responses: make(map[string]NonceSnapshot),
		Started:   time.Now(),
	}
	node.setBootstrapSession(sess)

	// 1. Discover peers.
	peers := node.peersForDomain(domain)
	sess.PeerSet = peers
	if len(peers) < cfg.Quorum && cfg.TrustedPeer == "" {
		sess.State = BootstrapQuorumMissed
		sess.Error = ErrBootstrapNoPeers
		sess.Ended = time.Now()
		return sess, ErrBootstrapNoPeers
	}

	// 2. Fetch snapshots (fan-out bounded at 2K so a single
	//    slow peer can't stall the entire session).
	maxFetch := cfg.Quorum * 2
	if maxFetch > len(peers) {
		maxFetch = len(peers)
	}
	fetchCtx, cancel := context.WithTimeout(ctx, cfg.TotalTimeout)
	defer cancel()

	var wg sync.WaitGroup
	var respMu sync.Mutex
	for i := 0; i < maxFetch; i++ {
		peer := peers[i]
		wg.Add(1)
		go func() {
			defer wg.Done()
			snap, err := node.fetchSnapshotFromPeer(fetchCtx, peer, domain, cfg.PeerTimeout)
			if err != nil {
				return
			}
			if err := VerifySnapshot(node.NonceLedger, snap); err != nil {
				return
			}
			if node.snapshotTooStale(snap, cfg.StaleTolerance, time.Now()) {
				return
			}
			respMu.Lock()
			sess.Responses[peer.ID] = snap
			respMu.Unlock()
		}()
	}
	wg.Wait()

	// 3. Apply operator override first (short-circuit).
	if cfg.TrustedPeer != "" {
		if snap, ok := sess.Responses[cfg.TrustedPeer]; ok {
			sess.Consensus = &snap
			sess.State = BootstrapQuorumMet
			sess.Ended = time.Now()
			return sess, nil
		}
		sess.State = BootstrapQuorumMissed
		sess.Error = ErrBootstrapInvalidOverride
		sess.Ended = time.Now()
		return sess, ErrBootstrapInvalidOverride
	}

	// 4. Group by BlockHash; pick the largest group.
	groups := make(map[string][]NonceSnapshot)
	for _, snap := range sess.Responses {
		groups[snap.BlockHash] = append(groups[snap.BlockHash], snap)
	}
	var winner []NonceSnapshot
	for _, g := range groups {
		if len(g) > len(winner) {
			winner = g
		}
	}

	if len(winner) < cfg.Quorum {
		sess.State = BootstrapQuorumMissed
		if len(sess.Responses) < cfg.Quorum {
			sess.Error = ErrBootstrapBelowQuorum
		} else {
			sess.Error = ErrBootstrapDisagreement
		}
		sess.Ended = time.Now()
		return sess, sess.Error
	}

	// 5. Height tolerance check across the winning group.
	if spread := heightSpread(winner); spread > int64(cfg.HeightTolerance) {
		sess.State = BootstrapQuorumMissed
		sess.Error = ErrBootstrapHeightTolerance
		sess.Ended = time.Now()
		return sess, ErrBootstrapHeightTolerance
	}

	// Winner reached.
	chosen := winner[0]
	sess.Consensus = &chosen
	sess.State = BootstrapQuorumMet
	sess.Ended = time.Now()
	return sess, nil
}

// snapshotTooStale reports whether a snapshot's Timestamp is
// older than staleTolerance.
func (node *QuidnugNode) snapshotTooStale(snap NonceSnapshot, tol time.Duration, now time.Time) bool {
	if snap.Timestamp <= 0 {
		return false
	}
	return now.Sub(time.Unix(snap.Timestamp, 0)) > tol
}

// heightSpread returns max-min BlockHeight across the group.
func heightSpread(g []NonceSnapshot) int64 {
	if len(g) == 0 {
		return 0
	}
	min := g[0].BlockHeight
	max := g[0].BlockHeight
	for _, s := range g {
		if s.BlockHeight < min {
			min = s.BlockHeight
		}
		if s.BlockHeight > max {
			max = s.BlockHeight
		}
	}
	return max - min
}

// ----- HTTP client ---------------------------------------------------------

// fetchSnapshotFromPeer issues the GET against one peer with a
// bounded timeout.
func (node *QuidnugNode) fetchSnapshotFromPeer(ctx context.Context, peer Node, domain string, timeout time.Duration) (NonceSnapshot, error) {
	reqCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	url := "http://" + peer.Address + "/api/v2/nonce-snapshots/" + domain + "/latest"
	req, err := http.NewRequestWithContext(reqCtx, "GET", url, nil)
	if err != nil {
		return NonceSnapshot{}, fmt.Errorf("bootstrap: new request: %w", err)
	}
	resp, err := node.httpClient.Do(req)
	if err != nil {
		return NonceSnapshot{}, fmt.Errorf("bootstrap: do: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		return NonceSnapshot{}, fmt.Errorf("bootstrap: peer %s has no snapshot", peer.ID)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return NonceSnapshot{}, fmt.Errorf("bootstrap: peer %s returned %d", peer.ID, resp.StatusCode)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return NonceSnapshot{}, fmt.Errorf("bootstrap: read: %w", err)
	}
	var envelope struct {
		Success bool            `json:"success"`
		Data    json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal(body, &envelope); err != nil {
		return NonceSnapshot{}, fmt.Errorf("bootstrap: parse envelope: %w", err)
	}
	var snap NonceSnapshot
	if err := json.Unmarshal(envelope.Data, &snap); err != nil {
		return NonceSnapshot{}, fmt.Errorf("bootstrap: parse snapshot: %w", err)
	}
	return snap, nil
}

// ----- Session registry ----------------------------------------------------

func (node *QuidnugNode) setBootstrapSession(sess *BootstrapSession) {
	if node.bootstrap == nil {
		node.bootstrap = newBootstrapRegistry()
	}
	node.bootstrap.mu.Lock()
	defer node.bootstrap.mu.Unlock()
	node.bootstrap.current = sess
}

// GetBootstrapSession returns the current (or last) bootstrap
// session for operator visibility. Nil if none has been run.
func (node *QuidnugNode) GetBootstrapSession() *BootstrapSession {
	if node.bootstrap == nil {
		return nil
	}
	node.bootstrap.mu.RLock()
	defer node.bootstrap.mu.RUnlock()
	return node.bootstrap.current
}

// ----- Trust list seeding --------------------------------------------------

// SeedBootstrapTrustList installs operator-asserted root trust
// entries into the ledger so that snapshot signature verification
// has public keys to check against. Called before
// BootstrapFromPeers.
//
// Returns ErrBootstrapTrustListEmpty if fewer than K entries
// are provided — without at least K trust roots, a K-of-K
// bootstrap can't possibly succeed.
func (node *QuidnugNode) SeedBootstrapTrustList(entries []BootstrapTrustEntry, k int) error {
	if len(entries) < k {
		return ErrBootstrapTrustListEmpty
	}
	for _, e := range entries {
		node.NonceLedger.SetSignerKey(e.Quid, 0, e.PublicKey)
	}
	return nil
}

// ApplyBootstrapSnapshot seeds the ledger from the consensus
// snapshot. Called after a successful BootstrapFromPeers when
// the caller has decided to commit. Transitions the session to
// BootstrapSeeding then BootstrapShadowVerify.
func (node *QuidnugNode) ApplyBootstrapSnapshot(shadowBlocks int) error {
	sess := node.GetBootstrapSession()
	if sess == nil || sess.Consensus == nil {
		return errors.New("bootstrap: no consensus snapshot to apply")
	}
	snap := *sess.Consensus
	for _, e := range snap.Entries {
		node.NonceLedger.seedAccepted(NonceKey{
			Quid:   e.Quid,
			Domain: snap.TrustDomain,
			Epoch:  e.Epoch,
		}, e.MaxNonce)
	}

	node.bootstrap.mu.Lock()
	sess.State = BootstrapShadowVerify
	sess.ShadowLeft = shadowBlocks
	node.bootstrap.completed[snap.TrustDomain] = time.Now()
	node.bootstrap.mu.Unlock()

	return nil
}

// seedAccepted is a low-level ledger mutator exposed so
// bootstrap can populate state without going through normal
// admission. Not exported publicly; bootstrap is the only
// legitimate caller.
func (l *NonceLedger) seedAccepted(k NonceKey, nonce int64) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.accepted == nil {
		l.accepted = make(map[NonceKey]int64)
	}
	if l.tentative == nil {
		l.tentative = make(map[NonceKey]int64)
	}
	if cur, ok := l.accepted[k]; !ok || nonce > cur {
		l.accepted[k] = nonce
	}
	if cur, ok := l.tentative[k]; !ok || nonce > cur {
		l.tentative[k] = nonce
	}
}

// ShadowVerifyStep is called on each new block after bootstrap
// completion. Returns nil if the block is compatible with the
// snapshot-seeded state, ErrBootstrapDivergence otherwise.
// When shadowLeft hits 0, the session transitions to Done.
func (node *QuidnugNode) ShadowVerifyStep(block Block) error {
	sess := node.GetBootstrapSession()
	if sess == nil || sess.State != BootstrapShadowVerify {
		return nil
	}
	// Basic shadow-verify: for each checkpoint in the new block,
	// confirm the pre-block accepted state matches what we seeded.
	// A disagreement would mean the seed was wrong or the chain
	// has diverged since the snapshot.
	for _, c := range block.NonceCheckpoints {
		key := NonceKey{Quid: c.Quid, Domain: c.Domain, Epoch: c.Epoch}
		// The block's MaxNonce is the post-block value. Our seed
		// is pre-block. We just verify the seed was a valid
		// predecessor — accepted[key] should be <= c.MaxNonce.
		if prior := node.NonceLedger.Accepted(key); prior > c.MaxNonce {
			node.bootstrap.mu.Lock()
			sess.State = BootstrapFailed
			sess.Error = fmt.Errorf("%w: key %+v seed=%d block=%d",
				ErrBootstrapDivergence, key, prior, c.MaxNonce)
			node.bootstrap.mu.Unlock()
			return sess.Error
		}
	}
	node.bootstrap.mu.Lock()
	sess.ShadowLeft--
	if sess.ShadowLeft <= 0 {
		sess.State = BootstrapDone
	}
	node.bootstrap.mu.Unlock()
	return nil
}
