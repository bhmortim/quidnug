// Block-level catch-up sync between admitted peers (ENG-78).
//
// Existing gossip primitives (anchor gossip, push gossip, K-of-K
// bootstrap, domain fingerprints) propagate metadata about block
// state — they do not move the actual block bodies between peers
// during steady-state operation. The result was observable as:
// node1 produces blocks, node2/3 stay at genesis indefinitely
// despite seeing each other in /api/v1/nodes.
//
// This file fills that gap with a polling pull-sync: every
// blockSyncInterval (30s by default), each node walks
// KnownNodes, asks each peer for blocks beyond the local tip via
// GET /api/v1/blocks?offset=tip+1, and feeds returned blocks
// through ReceiveBlock. Blocks already accepted under a higher
// tier are no-ops; new blocks pass validation and merge.
//
// The pull-sync is fire-and-forget per peer: a slow or
// unresponsive peer doesn't block the cycle for the others.
// Quarantined peers are skipped (Phase 4d routing-preference
// semantics).
package core

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"
)

// blockSyncInterval is how often the catch-up loop runs.
// Synchronized roughly with the gossip cadence elsewhere; tighter
// would create a flood when peers boot, looser would slow
// recovery from partition.
const blockSyncInterval = 30 * time.Second

// blockSyncBatchLimit caps how many blocks we pull from a single
// peer in one round. Avoids saturating the local apply pipeline
// when catching up from a long-running peer.
const blockSyncBatchLimit = 50

// runBlockSyncLoop is the per-node catch-up ticker. Spawned by
// Run(). Idempotent no-op when KnownNodes is empty (typical
// before discovery completes).
func (node *QuidnugNode) runBlockSyncLoop(ctx context.Context) {
	tk := time.NewTicker(blockSyncInterval)
	defer tk.Stop()
	// First sync runs ~5s after boot so the discovery loop has
	// a chance to populate KnownNodes. After that, steady cadence.
	select {
	case <-ctx.Done():
		return
	case <-time.After(5 * time.Second):
	}
	node.runBlockSyncOnce(ctx)
	for {
		select {
		case <-ctx.Done():
			return
		case <-tk.C:
			node.runBlockSyncOnce(ctx)
		}
	}
}

// runBlockSyncOnce performs one sync pass. For each non-
// quarantined peer in KnownNodes, asks for blocks at indexes
// strictly greater than this node's tip and applies the
// returned blocks through ReceiveBlock.
func (node *QuidnugNode) runBlockSyncOnce(ctx context.Context) {
	node.BlockchainMutex.RLock()
	tipIdx := int64(0)
	if len(node.Blockchain) > 0 {
		tipIdx = node.Blockchain[len(node.Blockchain)-1].Index
	}
	node.BlockchainMutex.RUnlock()

	// Snapshot peer list under the lock so the sync run
	// doesn't hold KnownNodesMutex during HTTP calls.
	type peerView struct {
		ID      string
		Address string
	}
	node.KnownNodesMutex.RLock()
	peers := make([]peerView, 0, len(node.KnownNodes))
	for _, n := range node.KnownNodes {
		if n.Address == "" {
			continue
		}
		peers = append(peers, peerView{ID: n.ID, Address: n.Address})
	}
	node.KnownNodesMutex.RUnlock()

	if len(peers) == 0 {
		return
	}

	// Pull from each non-quarantined peer concurrently. Capped
	// at the existing httpClient's connection pool size (no
	// explicit limit here).
	for _, p := range peers {
		if node.PeerScoreboard != nil && node.PeerScoreboard.IsQuarantined(p.ID) {
			continue
		}
		go node.pullBlocksFromPeer(ctx, p.ID, p.Address, tipIdx)
	}
}

// PullBlocksFromPeerForTest is the test-only entry point for
// the block-pull path. Production code calls pullBlocksFromPeer
// (lowercase) via runBlockSyncOnce.
func (node *QuidnugNode) PullBlocksFromPeerForTest(ctx context.Context, nodeQuid, addr string, tipIdx int64) {
	node.pullBlocksFromPeer(ctx, nodeQuid, addr, tipIdx)
}

// pullBlocksFromPeer fetches blocks at indexes > tipIdx from one
// peer and feeds them through ReceiveBlock. Failure is logged
// at debug; the peer's score takes a hit on transport failures.
func (node *QuidnugNode) pullBlocksFromPeer(ctx context.Context, nodeQuid, addr string, tipIdx int64) {
	// SSRF gate: peer-advertised address goes through the
	// sanitizer just like every other outbound dial.
	safeAddr, err := ValidatePeerAddress(addr)
	if err != nil {
		logger.Debug("block sync: refusing peer with invalid address",
			"nodeQuid", nodeQuid, "address", addr, "error", err)
		return
	}

	// Pagination: blocks are 0-indexed; we want everything >
	// tipIdx. The blocks endpoint accepts offset/limit. offset
	// is into the list, not the block index — so just ask for
	// the page starting at tipIdx+1 (we'll filter on receive).
	url := fmt.Sprintf("http://%s/api/v1/blocks?limit=%d&offset=%d",
		safeAddr.String(),
		blockSyncBatchLimit,
		tipIdx+1)
	reqCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(reqCtx, "GET", url, nil) // #nosec -- url built from sanitized address
	if err != nil {
		return
	}
	resp, err := node.httpClient.Do(req) // #nosec -- url built from sanitized address; transport enforces safedial
	if err != nil {
		logger.Debug("block sync: dial failed",
			"nodeQuid", nodeQuid, "error", err)
		node.recordPeerScore(nodeQuid, EventClassQuery, false, "block-sync dial: "+err.Error())
		return
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		logger.Debug("block sync: non-2xx",
			"nodeQuid", nodeQuid, "status", resp.StatusCode)
		node.recordPeerScore(nodeQuid, EventClassQuery, false,
			"block-sync status "+strconv.Itoa(resp.StatusCode))
		return
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 16<<20))
	if err != nil {
		return
	}

	// /api/v1/blocks envelope: { success, data: { data: [Block...], pagination } }
	var env struct {
		Success bool `json:"success"`
		Data    struct {
			Data []Block `json:"data"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &env); err != nil {
		logger.Debug("block sync: decode failed",
			"nodeQuid", nodeQuid, "error", err)
		return
	}
	if len(env.Data.Data) == 0 {
		// Peer is at or behind us; nothing to pull.
		node.recordPeerScore(nodeQuid, EventClassQuery, true, "")
		return
	}

	applied := 0
	skipped := 0
	for _, b := range env.Data.Data {
		// Defensive: only apply blocks strictly beyond the
		// local tip. The peer might have given us blocks at
		// or before tipIdx if pagination drifted.
		if b.Index <= tipIdx {
			skipped++
			continue
		}
		acceptance, err := node.ReceiveBlock(b)
		if err != nil {
			logger.Debug("block sync: peer-served block rejected",
				"nodeQuid", nodeQuid,
				"blockIndex", b.Index,
				"acceptance", acceptance,
				"error", err)
			// A peer serving us blocks we can't accept gets
			// a validation hit. Repeat offenders fall through
			// the scoring system into quarantine.
			node.recordPeerScore(nodeQuid, EventClassValidation, false,
				fmt.Sprintf("block %d rejected: %v", b.Index, err))
			continue
		}
		if acceptance == BlockTrusted {
			applied++
			logger.Info("Received block from peer",
				"nodeQuid", nodeQuid,
				"blockIndex", b.Index,
				"domain", b.TrustProof.TrustDomain,
				"acceptance", "trusted")
		}
		// Tentative / Untrusted blocks are recorded by
		// ReceiveBlock for trust-extraction purposes; we don't
		// log per-block at info to keep the log rate sane.
	}
	if applied > 0 || skipped > 0 {
		logger.Info("Block sync cycle complete",
			"nodeQuid", nodeQuid,
			"applied", applied,
			"skipped", skipped,
			"localTip", tipIdx)
	}
	node.recordPeerScore(nodeQuid, EventClassQuery, true, "")
}
