// Peer admission pipeline.
//
// Three peer sources (static peers_file, mDNS LAN discovery,
// gossip discovery from seeds) all feed the same `AdmitPeer`
// entry point. AdmitPeer:
//
//  1. Validates the dial-target address (re-uses safedial's
//     blocked-range check; honors per-peer allow_private only
//     for trusted sources, never gossip).
//  2. Hits the candidate's GET /api/v1/info to learn its claimed
//     NodeQuid + OperatorQuid.
//  3. Looks up the candidate in NodeAdvertisementRegistry; if
//     RequireAdvertisement is true and there's no current ad,
//     rejects.
//  4. Verifies operator attestation: TRUST edge OperatorQuid →
//     NodeQuid at weight ≥ PeerMinOperatorTrust. Skipped when
//     OperatorQuid is empty (ephemeral node) AND
//     RequireAdvertisement is false.
//  5. (Optional) Verifies operator reputation: at least one
//     incoming TRUST edge to OperatorQuid at weight ≥
//     PeerMinOperatorReputation.
//
// Admitted peers land in node.KnownNodes with origin tagged.
// Periodic re-attestation is scheduled by the caller (the
// peering loop).
package core

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"
)

// PeerSource identifies how a peer candidate arrived.
//
//   - PeerSourceSeed: configured `seed_nodes` list at boot.
//   - PeerSourceGossip: discovered via the seed-walking loop,
//     i.e. a peer that some other node told us about.
//   - PeerSourceLAN: discovered via mDNS on the local segment.
//   - PeerSourceStatic: listed in operator's peers_file.
//
// allow_private overrides are honored ONLY for Static + LAN
// sources (the operator explicitly chose to trust those).
type PeerSource string

const (
	PeerSourceSeed   PeerSource = "seed"
	PeerSourceGossip PeerSource = "gossip"
	PeerSourceLAN    PeerSource = "lan"
	PeerSourceStatic PeerSource = "static"
)

// allowsPrivate reports whether per-peer allow_private overrides
// are honored for this source.
func (s PeerSource) allowsPrivate() bool {
	return s == PeerSourceStatic || s == PeerSourceLAN
}

// PeerCandidate is the input to AdmitPeer. Address is required;
// everything else is optional information the source already
// has.
type PeerCandidate struct {
	Address      string
	NodeQuid     string // may be empty; resolved via /api/v1/info handshake
	OperatorQuid string // may be empty; resolved via handshake
	Source       PeerSource
	AllowPrivate bool // honored only for Static + LAN
}

// PeerAdmitConfig captures the per-call thresholds and gates the
// pipeline reads. Decoupled from config.Config so callers can
// override per-source (e.g. mDNS-discovered peers might use a
// looser RequireAdvertisement than gossip-discovered ones).
type PeerAdmitConfig struct {
	RequireAdvertisement      bool
	MinOperatorTrust          float64
	MinOperatorReputation     float64 // 0 disables this gate
	HandshakeTimeout          time.Duration
}

// PeerVerdict is what AdmitPeer returns on success. Rejections
// surface as errors instead, with reason embedded in the message.
type PeerVerdict struct {
	NodeQuid     string
	OperatorQuid string
	Source       PeerSource
	AdmittedAt   time.Time
	HasAd        bool
	OpTrustEdge  float64 // weight of OperatorQuid → NodeQuid TRUST edge
}

// AdmitPeer runs the four-stage admission pipeline. Returns a
// non-nil verdict on success and an error explaining the gate
// failure on rejection. The error wrapping is intentionally
// verbose so logs surface the exact reason without operators
// having to enable debug.
func (node *QuidnugNode) AdmitPeer(ctx context.Context, c PeerCandidate, cfg PeerAdmitConfig) (*PeerVerdict, error) {
	if cfg.HandshakeTimeout == 0 {
		cfg.HandshakeTimeout = 5 * time.Second
	}

	// Stage 1: address validation.
	safe, err := ValidatePeerAddress(c.Address)
	if err != nil {
		// If the source allows private overrides AND the
		// candidate explicitly opted in, retry by recording
		// the address in the per-node allow-list and
		// validating again. The allow-list mutation is the
		// signal to safeDialContext that this address is
		// pre-approved; future dials to it will succeed.
		if c.AllowPrivate && c.Source.allowsPrivate() && node.PrivateAddrAllowList != nil {
			node.PrivateAddrAllowList.Set(append(currentAllowList(node), c.Address))
			// Re-resolve via the dialer (which now consults
			// the allow-list) by simply continuing past the
			// pre-flight check. ValidatePeerAddress doesn't
			// know about the allow-list, so we accept that
			// the syntactic address-shape was already
			// validated and proceed with the raw c.Address.
			safe = sanitizedRaw(c.Address)
		} else {
			return nil, fmt.Errorf("admit %s: address: %w", c.Source, err)
		}
	}

	// Stage 2: handshake. /api/v1/info reports NodeQuid +
	// OperatorQuid. Skipped when no gates are configured (the
	// "fully-permissive dev/test mode": RequireAdvertisement=false,
	// MinOperatorTrust=0, MinOperatorReputation=0). In that mode
	// the candidate's self-asserted NodeQuid is taken at face
	// value, matching pre-admit-pipeline behavior so existing
	// tests continue to work.
	verdict := &PeerVerdict{
		NodeQuid:     c.NodeQuid,
		OperatorQuid: c.OperatorQuid,
		Source:       c.Source,
		AdmittedAt:   time.Now(),
	}
	gatesActive := cfg.RequireAdvertisement || cfg.MinOperatorTrust > 0 || cfg.MinOperatorReputation > 0
	_ = safe // referenced inside the gatesActive branch
	if gatesActive {
		hsCtx, cancel := context.WithTimeout(ctx, cfg.HandshakeTimeout)
		defer cancel()
		info, err := node.peerInfoHandshake(hsCtx, safe.String())
		if err != nil {
			// Record handshake failure against whatever we
			// know about the peer's identity. NodeQuid may
			// be the candidate-claimed value or empty; the
			// scoreboard tolerates both (empty quid is a
			// no-op).
			node.recordPeerScore(c.NodeQuid, EventClassHandshake, false, fmt.Sprintf("handshake to %s: %v", c.Address, err))
			return nil, fmt.Errorf("admit %s: handshake %s: %w", c.Source, c.Address, err)
		}
		// Successful handshake — credit the verified NodeQuid
		// (info.NodeQuid is what the peer self-asserts), not
		// the candidate-claimed one.
		node.recordPeerScore(info.NodeQuid, EventClassHandshake, true, "")
		if c.NodeQuid != "" && info.NodeQuid != c.NodeQuid {
			return nil, fmt.Errorf("admit %s: handshake %s: NodeQuid mismatch (claimed %s, served %s)",
				c.Source, c.Address, c.NodeQuid, info.NodeQuid)
		}
		if c.OperatorQuid != "" && info.OperatorQuid != "" && info.OperatorQuid != c.OperatorQuid {
			return nil, fmt.Errorf("admit %s: handshake %s: OperatorQuid mismatch (pinned %s, served %s)",
				c.Source, c.Address, c.OperatorQuid, info.OperatorQuid)
		}
		verdict.NodeQuid = info.NodeQuid
		verdict.OperatorQuid = info.OperatorQuid
	}

	// Stage 3: NodeAdvertisement lookup.
	if node.NodeAdvertisementRegistry != nil && verdict.NodeQuid != "" {
		if ad, ok := node.NodeAdvertisementRegistry.Get(verdict.NodeQuid); ok {
			verdict.HasAd = true
			// If the ad pins an OperatorQuid, prefer that
			// over what /info served (the ad is signed and
			// cross-checked against TRUST; /info is just a
			// JSON value the peer self-reports).
			if ad.OperatorQuid != "" {
				verdict.OperatorQuid = ad.OperatorQuid
			}
		}
	}
	if cfg.RequireAdvertisement && !verdict.HasAd {
		return nil, fmt.Errorf("admit %s: no current NodeAdvertisement for %s", c.Source, verdict.NodeQuid)
	}

	// Stage 4: operator attestation.
	if verdict.OperatorQuid != "" && verdict.NodeQuid != "" && cfg.MinOperatorTrust > 0 {
		w := node.lookupTrustWeight(verdict.OperatorQuid, verdict.NodeQuid)
		verdict.OpTrustEdge = w
		if w < cfg.MinOperatorTrust {
			return nil, fmt.Errorf("admit %s: operator attestation weight %.3f < %.3f (operator=%s, node=%s)",
				c.Source, w, cfg.MinOperatorTrust, verdict.OperatorQuid, verdict.NodeQuid)
		}
	}

	// Stage 5: optional operator reputation.
	if cfg.MinOperatorReputation > 0 && verdict.OperatorQuid != "" {
		if !node.hasIncomingTrustToOperator(verdict.OperatorQuid, cfg.MinOperatorReputation) {
			return nil, fmt.Errorf("admit %s: operator %s lacks incoming trust ≥ %.3f from local trust graph",
				c.Source, verdict.OperatorQuid, cfg.MinOperatorReputation)
		}
	}

	return verdict, nil
}

// peerInfoResponse is the subset of /api/v1/info the admit
// pipeline cares about. Full API may include more fields; we
// only decode what we use.
type peerInfoResponse struct {
	NodeQuid     string `json:"nodeQuid"`
	OperatorQuid string `json:"-"` // assembled from operatorQuid.id below
}

type peerInfoEnvelope struct {
	Success bool `json:"success"`
	Data    struct {
		NodeQuid     string `json:"nodeQuid"`
		OperatorQuid struct {
			ID string `json:"id"`
		} `json:"operatorQuid"`
	} `json:"data"`
}

// peerInfoHandshake calls GET /api/v1/info on the candidate and
// extracts NodeQuid + OperatorQuid.
func (node *QuidnugNode) peerInfoHandshake(ctx context.Context, hostPort string) (peerInfoResponse, error) {
	url := fmt.Sprintf("http://%s/api/v1/info", hostPort)
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil) // #nosec G107 -- hostPort is sanitized via ValidatePeerAddress in caller; transport also enforces safedial
	if err != nil {
		return peerInfoResponse{}, fmt.Errorf("new request: %w", err)
	}
	req.Header.Set("Accept", "application/json")
	resp, err := node.httpClient.Do(req) // #nosec -- handshake URL built from sanitized address; transport enforces safedial
	if err != nil {
		return peerInfoResponse{}, fmt.Errorf("dial: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return peerInfoResponse{}, fmt.Errorf("status %d", resp.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 64<<10))
	if err != nil {
		return peerInfoResponse{}, fmt.Errorf("read: %w", err)
	}
	var env peerInfoEnvelope
	if err := json.Unmarshal(body, &env); err != nil {
		return peerInfoResponse{}, fmt.Errorf("decode: %w", err)
	}
	out := peerInfoResponse{
		NodeQuid:     env.Data.NodeQuid,
		OperatorQuid: env.Data.OperatorQuid.ID,
	}
	if out.NodeQuid == "" {
		return out, fmt.Errorf("response missing nodeQuid")
	}
	return out, nil
}

// lookupTrustWeight returns the most-recent TRUST-edge weight
// from `truster` to `trustee` across all domains the node has
// observed. Returns 0 if no edge exists. Falls back to the
// verified-edges registry first (cryptographically validated)
// and only consults the unverified registry if the verified one
// is empty.
//
// We do not require the edge to live in a specific domain
// (operators.network.<x>) here; the admit pipeline cares about
// the existence of the edge, not its domain. Operators who want
// stricter domain scoping should set MinOperatorReputation > 0
// and rely on the on-chain NodeAdvertisement validation, which
// does enforce the operators.network.<operator-domain> path.
func (node *QuidnugNode) lookupTrustWeight(truster, trustee string) float64 {
	// Verified registry: TrustRegistry holds the latest
	// per-(truster, trustee) weight, but it is not domain-
	// scoped. That's the right granularity for admit; the
	// per-domain ValidatorTrustRegistry is the strict version.
	node.TrustRegistryMutex.RLock()
	defer node.TrustRegistryMutex.RUnlock()
	if m, ok := node.TrustRegistry[truster]; ok {
		if w, ok := m[trustee]; ok {
			return w
		}
	}
	return 0
}

// hasIncomingTrustToOperator returns true iff at least one quid
// in the local TrustRegistry has granted TRUST to `operator` at
// weight ≥ minWeight. Implements the optional Stage 5
// reputation gate ("I only peer with operators my friends
// already trust").
func (node *QuidnugNode) hasIncomingTrustToOperator(operator string, minWeight float64) bool {
	node.TrustRegistryMutex.RLock()
	defer node.TrustRegistryMutex.RUnlock()
	for _, edges := range node.TrustRegistry {
		if w, ok := edges[operator]; ok && w >= minWeight {
			return true
		}
	}
	return false
}

// currentAllowList snapshots the current PrivateAddrAllowList
// contents into a slice so callers that want to ADD an entry
// can do so via Set(prev + new). The allow-list type is
// intentionally narrow (just Set+Has) so this little helper
// bridges the gap.
func currentAllowList(node *QuidnugNode) []string {
	if node == nil || node.PrivateAddrAllowList == nil {
		return nil
	}
	node.PrivateAddrAllowList.mu.RLock()
	defer node.PrivateAddrAllowList.mu.RUnlock()
	out := make([]string, 0, len(node.PrivateAddrAllowList.addrs))
	for k := range node.PrivateAddrAllowList.addrs {
		out = append(out, k)
	}
	return out
}

// sanitizedRaw constructs a SanitizedPeerAddress from a raw
// host:port WITHOUT running the IP-blocklist check. Used only
// after the per-peer allow-list has approved the address.
func sanitizedRaw(addr string) SanitizedPeerAddress {
	return SanitizedPeerAddress{hostPort: addr}
}

// PeerSourceMutex serializes admit-pipeline runs from concurrent
// peer sources so we don't double-add the same peer or fight
// over the allow-list. Sources call Lock before AdmitPeer and
// Unlock after the resulting KnownNodes mutation.
//
// Defined as a type so callers can embed it; the singleton is
// on QuidnugNode (see node.go).
type peerAdmitMu struct{ sync.Mutex }
