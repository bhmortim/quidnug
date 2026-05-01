// Peering loop: orchestrates the static peers_file source.
//
// The static loop is the simplest of the three peer sources
// (file → admit pipeline → KnownNodes). It is wired up by Run()
// alongside the existing seed-based discoverFromSeeds loop.
//
// On boot:
//
//  1. Load peers_file (synchronous; fatal if path is set but
//     unparseable, since the operator wrote a config they
//     expect to be honored).
//  2. Run AdmitPeer on each entry, populating KnownNodes for
//     accepted peers and the PrivateAddrAllowList for entries
//     marked allow_private.
//
// Live-reload (fsnotify):
//
//  3. Each file-change event re-runs step 2 with the new
//     contents. Removed entries are evicted from KnownNodes
//     and the allow-list. Added entries go through the same
//     admit pipeline as boot.
//
// Periodic re-attestation (every PeerReattestationInterval) is
// handled by a separate ticker that walks KnownNodes and
// re-checks the operator-attestation TRUST edge for each
// admitted peer. Peers that fail re-attestation are evicted.
package core

import (
	"context"
	"sync"
	"time"

	"github.com/quidnug/quidnug/internal/peering"
)

// runStaticPeerLoop loads the operator's peers_file, applies it
// once at boot, and re-applies on every file-change event from
// the watcher. The loop terminates when ctx is cancelled.
//
// The loop is fire-and-forget: any individual peer admission
// failure is logged at warn level but does not tear down the
// loop, because the operator's intent (the file) might list
// dozens of peers and we should admit the ones that pass even
// if some fail.
func (node *QuidnugNode) runStaticPeerLoop(
	ctx context.Context,
	peersFile string,
	cfg PeerAdmitConfig,
) {
	if peersFile == "" {
		return
	}
	w := peering.NewWatcher(peersFile)
	if err := w.Start(ctx); err != nil {
		logger.Warn("Static peer loop disabled (watcher start failed)",
			"peersFile", peersFile, "error", err)
		return
	}
	defer w.Stop()

	// Track which peer NodeQuids the static file is currently
	// responsible for. On reload, peers that disappear from the
	// file are evicted from KnownNodes and the allow-list.
	staticOwned := make(map[string]struct{}) // nodeQuid set
	staticAddrs := make(map[string]struct{}) // address set, for allow-list mgmt
	var muOwned sync.Mutex

	for {
		select {
		case <-ctx.Done():
			logger.Info("Static peer loop stopped")
			return
		case entries, ok := <-w.Events():
			if !ok {
				return
			}
			node.applyStaticPeers(ctx, entries, cfg, staticOwned, staticAddrs, &muOwned)
		}
	}
}

// applyStaticPeers reconciles the current peers_file contents
// against the node's known-state. New entries go through the
// admit pipeline; entries that disappeared are evicted.
func (node *QuidnugNode) applyStaticPeers(
	ctx context.Context,
	entries []peering.PeerEntry,
	cfg PeerAdmitConfig,
	staticOwned map[string]struct{},
	staticAddrs map[string]struct{},
	muOwned *sync.Mutex,
) {
	// Pre-populate the allow-list with this reload's
	// allow_private entries so the admit-pipeline handshake can
	// reach LAN peers. This is the chicken-and-egg fix:
	// AdmitPeer dials /api/v1/info, which runs through
	// safeDialContext, which consults the allow-list. We must
	// add LAN entries to the allow-list BEFORE the handshake.
	muOwned.Lock()
	defer muOwned.Unlock()
	desiredAddrs := make(map[string]struct{}, len(entries))
	allowTokens := make([]string, 0)
	// Preserve allow-list entries owned by other sources (mDNS,
	// future additions).
	for tok := range currentAllowListSet(node) {
		if _, mine := staticAddrs[tok]; !mine {
			allowTokens = append(allowTokens, tok)
		}
	}
	for _, e := range entries {
		desiredAddrs[e.Address] = struct{}{}
		if e.AllowPrivate {
			allowTokens = append(allowTokens, e.Address)
		}
	}
	if node.PrivateAddrAllowList != nil {
		node.PrivateAddrAllowList.Set(allowTokens)
	}

	// Refresh staticAddrs to match the new desired set.
	for k := range staticAddrs {
		delete(staticAddrs, k)
	}
	for k := range desiredAddrs {
		staticAddrs[k] = struct{}{}
	}

	// Run admit pipeline for each entry.
	desiredQuids := make(map[string]struct{}, len(entries))
	for _, e := range entries {
		c := PeerCandidate{
			Address:      e.Address,
			OperatorQuid: e.OperatorQuid,
			Source:       PeerSourceStatic,
			AllowPrivate: e.AllowPrivate,
		}
		v, err := node.AdmitPeer(ctx, c, cfg)
		if err != nil {
			logger.Warn("Static peer admission failed",
				"address", e.Address, "error", err)
			continue
		}
		desiredQuids[v.NodeQuid] = struct{}{}

		// Insert / update KnownNodes.
		node.KnownNodesMutex.Lock()
		node.KnownNodes[v.NodeQuid] = Node{
			ID:               v.NodeQuid,
			Address:          e.Address,
			LastSeen:         time.Now().Unix(),
			ConnectionStatus: "static",
		}
		node.KnownNodesMutex.Unlock()
		logger.Info("Admitted static peer",
			"nodeQuid", v.NodeQuid, "operatorQuid", v.OperatorQuid,
			"address", e.Address, "trustEdge", v.OpTrustEdge,
			"hasAd", v.HasAd)
	}

	// Evict peers we previously owned but the new file doesn't
	// list. We only evict from KnownNodes if WE were the source;
	// peers also reachable via gossip stay.
	for prev := range staticOwned {
		if _, kept := desiredQuids[prev]; kept {
			continue
		}
		node.KnownNodesMutex.Lock()
		if cur, ok := node.KnownNodes[prev]; ok && cur.ConnectionStatus == "static" {
			delete(node.KnownNodes, prev)
			logger.Info("Evicted static peer (removed from peers_file)",
				"nodeQuid", prev)
		}
		node.KnownNodesMutex.Unlock()
		delete(staticOwned, prev)
	}
	for q := range desiredQuids {
		staticOwned[q] = struct{}{}
	}
}

// currentAllowListSet returns a set view of the per-node
// allow-list. Used by the static-peer reconciliation to
// distinguish "tokens we set last time" from "tokens other
// sources own."
func currentAllowListSet(node *QuidnugNode) map[string]struct{} {
	out := make(map[string]struct{})
	if node == nil || node.PrivateAddrAllowList == nil {
		return out
	}
	for _, k := range currentAllowList(node) {
		out[k] = struct{}{}
	}
	return out
}
