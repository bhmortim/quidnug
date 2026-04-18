// Package core — epoch_probe.go
//
// Client-side probe of a signer's home domain (QDP-0007 §6.2).
// Called when a transaction arrives from a signer whose local
// epoch state is older than EpochRecencyWindow.
//
// The probe issues `GET /api/v2/domain-fingerprints/{home}/latest`
// to up to N peers from the home domain. The first valid
// fingerprint updates the ledger and marks the signer's recency.
// All failures return an error; the caller decides whether to
// admit-with-warning or reject based on ProbeTimeoutPolicy.
//
// Validation is the same as inbound push gossip: the fingerprint
// must be signed by a validator whose key the receiver already
// knows. A malicious probe target can't forge results.
package core

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"
)

// ----- Recency tracking ----------------------------------------------------

// MarkEpochRefresh records that we have fresh state for the
// given signer, at the given time. Called from:
//
//   - Successful probe response.
//   - Push gossip arriving for a signer.
//   - Block commit that includes a rotation for the signer.
//
// The lastEpochRefresh map lives on the ledger so every code
// path that touches epoch state can update it.
func (l *NonceLedger) MarkEpochRefresh(signer string, at time.Time) {
	if signer == "" {
		return
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.lastEpochRefresh == nil {
		l.lastEpochRefresh = make(map[string]int64)
	}
	l.lastEpochRefresh[signer] = at.Unix()
}

// EpochRecent reports whether the signer's state was refreshed
// within `window` of `now`. Returns true when no refresh has
// ever been recorded for a signer whose identity we don't track
// — if we have no state at all, there's nothing to quarantine
// against. But if `signerKeys` exists for the signer, we treat
// never-refreshed as stale.
func (l *NonceLedger) EpochRecent(signer string, window time.Duration, now time.Time) bool {
	if signer == "" {
		return true
	}
	l.mu.RLock()
	defer l.mu.RUnlock()
	if _, hasKey := l.signerKeys[signer]; !hasKey {
		return true // no state to compare — let normal admission handle it
	}
	ts := l.lastEpochRefresh[signer]
	if ts == 0 {
		return false
	}
	return now.Sub(time.Unix(ts, 0)) <= window
}

// ----- Probe ---------------------------------------------------------------

// ErrProbeNoPeers is returned when the probe finds zero peers
// for the target home domain. Distinguished from a probe timeout
// so operators can tell "home is down" from "we don't know about
// home."
var ErrProbeNoPeers = errors.New("epoch-probe: no peers known for home domain")

// ErrProbeAllFailed is returned when every peer attempt failed
// or returned an invalid fingerprint.
var ErrProbeAllFailed = errors.New("epoch-probe: all peer attempts failed")

// ProbeHomeDomain attempts to fetch the latest fingerprint for a
// signer's home domain. Returns a validated fingerprint on
// success. Contract:
//
//   - Success: ledger has been updated with the fetched
//     fingerprint (via StoreDomainFingerprint, monotonicity-safe).
//   - Failure: no ledger mutation; error describes why.
//
// Not goroutine-safe against concurrent probes for the same
// signer — callers should deduplicate probes at the enqueue
// layer. A duplicate probe is safe but wasteful.
func (node *QuidnugNode) ProbeHomeDomain(ctx context.Context, signer string, homeDomain string) (DomainFingerprint, error) {
	if homeDomain == "" {
		homeDomain = node.defaultHomeDomain()
	}
	peers := node.peersForDomain(homeDomain)
	if len(peers) == 0 {
		return DomainFingerprint{}, ErrProbeNoPeers
	}

	// Cap the fanout at 3 peers per probe — enough for a quorum-
	// of-1, not so many that one probe eats all our HTTP budget.
	if len(peers) > 3 {
		peers = peers[:3]
	}

	var lastErr error
	for _, peer := range peers {
		fp, err := node.fetchFingerprintFromPeer(ctx, peer, homeDomain)
		if err != nil {
			lastErr = err
			continue
		}
		if err := VerifyDomainFingerprint(node.NonceLedger, fp, time.Now()); err != nil {
			lastErr = fmt.Errorf("epoch-probe: verify fingerprint from %s: %w", peer.ID, err)
			continue
		}
		// Success: store, mark recency.
		node.NonceLedger.StoreDomainFingerprint(fp)
		node.NonceLedger.MarkEpochRefresh(signer, time.Now())
		return fp, nil
	}
	if lastErr == nil {
		lastErr = ErrProbeAllFailed
	}
	return DomainFingerprint{}, lastErr
}

// defaultHomeDomain picks the node's best-guess home when a
// signer's identity record has no HomeDomain. Uses the first
// SupportedDomain, falling back to the genesis domain.
func (node *QuidnugNode) defaultHomeDomain() string {
	if len(node.SupportedDomains) > 0 {
		return node.SupportedDomains[0]
	}
	return "genesis"
}

// peersForDomain returns nodes known to serve the named domain.
// Uses the existing DomainRegistry reverse index that domain
// gossip maintains.
func (node *QuidnugNode) peersForDomain(domain string) []Node {
	node.DomainRegistryMutex.RLock()
	nodeIDs := append([]string(nil), node.DomainRegistry[domain]...)
	node.DomainRegistryMutex.RUnlock()

	node.KnownNodesMutex.RLock()
	defer node.KnownNodesMutex.RUnlock()

	out := make([]Node, 0, len(nodeIDs))
	for _, id := range nodeIDs {
		if n, ok := node.KnownNodes[id]; ok && n.Address != "" {
			out = append(out, n)
		}
	}
	return out
}

// fetchFingerprintFromPeer issues the HTTP GET against one peer
// with a bounded timeout.
func (node *QuidnugNode) fetchFingerprintFromPeer(ctx context.Context, peer Node, domain string) (DomainFingerprint, error) {
	url := "http://" + peer.Address + "/api/v2/domain-fingerprints/" + domain + "/latest"
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return DomainFingerprint{}, fmt.Errorf("epoch-probe: new request: %w", err)
	}
	resp, err := node.httpClient.Do(req)
	if err != nil {
		return DomainFingerprint{}, fmt.Errorf("epoch-probe: do: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusAccepted {
		return DomainFingerprint{}, fmt.Errorf("epoch-probe: %s status %d", peer.ID, resp.StatusCode)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return DomainFingerprint{}, fmt.Errorf("epoch-probe: read: %w", err)
	}
	// The server wraps successful responses in a standard envelope
	// (see response.go WriteSuccess). Parse that first, then unwrap.
	var envelope struct {
		Success bool            `json:"success"`
		Data    json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal(body, &envelope); err != nil {
		return DomainFingerprint{}, fmt.Errorf("epoch-probe: parse envelope: %w", err)
	}
	if !envelope.Success {
		return DomainFingerprint{}, fmt.Errorf("epoch-probe: peer %s reported failure", peer.ID)
	}
	var fp DomainFingerprint
	if err := json.Unmarshal(envelope.Data, &fp); err != nil {
		return DomainFingerprint{}, fmt.Errorf("epoch-probe: parse fp: %w", err)
	}
	return fp, nil
}
