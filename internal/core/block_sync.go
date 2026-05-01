// Block-level catch-up sync between admitted peers (ENG-78,
// ENG-82).
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
// KnownNodes, asks each peer for their /api/v1/blocks page-by-
// page, and feeds returned blocks through ReceiveBlock after
// hash-dedup against the local chain. Blocks already accepted
// under a higher tier are no-ops; new blocks pass validation and
// merge.
//
// The pull-sync is fire-and-forget per peer: a slow or
// unresponsive peer doesn't block the cycle for the others.
// Quarantined peers are skipped (Phase 4d routing-preference
// semantics).
//
// ENG-82: dedup is hash-based, not index-based. The previous
// scheme used (offset = localTipIdx + 1) and filtered returned
// blocks by (b.Index <= localTipIdx). This worked for a single
// validator producing a single domain's chain, but ENG-80
// introduced per-domain Index semantics where each domain has
// its own anchor and counter. localTipIdx is the per-domain
// index of whichever domain was appended last, peer offset is
// into the cross-domain flat slice, and b.Index is the per-
// domain index — three different things being compared as if
// they were the same. The visible symptom in a multi-domain
// mesh was silent rejection of valid blocks whose per-domain
// index happened to be lower than the local tail's per-domain
// index. Block hashes are unambiguous identifiers across
// domains, so we dedup on those instead.
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
// peer in one HTTP page. Avoids saturating the local apply
// pipeline when catching up from a long-running peer.
const blockSyncBatchLimit = 50

// blockSyncMaxPages bounds the per-peer pagination loop.
// 100 pages × 50 blocks = 5000 blocks max per peer per cycle.
// Operators on chains larger than that should crank
// blockSyncBatchLimit rather than blockSyncMaxPages — the latter
// is a runaway-defense bound, not a tuning knob.
const blockSyncMaxPages = 100

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
// quarantined peer in KnownNodes, walks the peer's chain and
// applies any blocks not already present locally (hash-dedup'd).
//
// ENG-82: the previous incarnation snapshotted a tipIdx here
// and passed it to each peer pull. After the per-domain Index
// refactor (ENG-80) that snapshot was meaningless and is gone.
// pullBlocksFromPeer now builds its own dedup set from the
// local chain at the start of each pull.
func (node *QuidnugNode) runBlockSyncOnce(ctx context.Context) {
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
		go node.pullBlocksFromPeer(ctx, p.ID, p.Address)
	}
}

// PullBlocksFromPeerForTest is the test-only entry point for
// the block-pull path. Production code calls pullBlocksFromPeer
// (lowercase) via runBlockSyncOnce.
//
// ENG-82: signature dropped the tipIdx int64 parameter that no
// longer drives anything inside (hash-dedup superseded it).
func (node *QuidnugNode) PullBlocksFromPeerForTest(ctx context.Context, nodeQuid, addr string) {
	node.pullBlocksFromPeer(ctx, nodeQuid, addr)
}

// pullBlocksFromPeer paginates through one peer's /api/v1/blocks
// and feeds previously-unseen blocks (hash-dedup'd against the
// local chain) into ReceiveBlock.
//
// Pagination strategy: start at offset=0 and walk pages of
// blockSyncBatchLimit until either (a) a partial page indicates
// we've reached the peer's tail, or (b) blockSyncMaxPages
// hops as a safety bound. Per-cycle worst case is one full
// chain walk; in steady state most blocks dedup as duplicates,
// so the cost is dominated by the initial catch-up rather than
// by ongoing churn.
//
// Hash-dedup rationale: see the package-level comment. A block
// hash uniquely identifies it across all (domain, validator,
// index) triples, so equality-by-hash is the only reliable
// "we already have this" predicate after ENG-80.
//
// Failure is logged at warn (ENG-79: previously debug, which
// masked the silent-non-convergence bug); the peer's score
// takes a hit on transport failures.
func (node *QuidnugNode) pullBlocksFromPeer(ctx context.Context, nodeQuid, addr string) {
	// SSRF gate: peer-advertised address goes through the
	// sanitizer just like every other outbound dial.
	// ENG-79: use the node-method variant so per-peer
	// allow_private (set when the operator added a static
	// peer with the flag, or when an mDNS peer was admitted)
	// short-circuits the global blocked-range check.
	safeAddr, err := node.validatePeerAddress(addr)
	if err != nil {
		// WARN level (ENG-79): a non-convergence regression
		// here was previously invisible at INFO. Operators
		// running a healthy mesh should NOT see this line.
		logger.Warn("block sync: refusing peer with invalid address",
			"nodeQuid", nodeQuid, "address", addr, "error", err)
		return
	}

	// Snapshot local block hashes once at cycle start. New
	// blocks accepted during the cycle are added to this set as
	// we apply them, so a peer serving the same block in two
	// pages (or two peers with overlapping chains in successive
	// goroutines) doesn't double-apply. The dedup is purely
	// belt-and-suspenders for ReceiveBlock idempotence — the
	// receive path itself is idempotent for already-present
	// blocks, but skipping the work avoids per-block validation
	// overhead.
	node.BlockchainMutex.RLock()
	seenHashes := make(map[string]struct{}, len(node.Blockchain))
	for _, b := range node.Blockchain {
		seenHashes[b.Hash] = struct{}{}
	}
	node.BlockchainMutex.RUnlock()

	var (
		peerOffset    int64
		totalApplied  int
		totalSkipped  int
		pagesFetched  int
		lastHTTPError error
	)

	for pagesFetched < blockSyncMaxPages {
		pagesFetched++

		url := fmt.Sprintf("http://%s/api/v1/blocks?limit=%d&offset=%d",
			safeAddr.String(),
			blockSyncBatchLimit,
			peerOffset)
		reqCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
		req, err := http.NewRequestWithContext(reqCtx, "GET", url, nil) // #nosec -- url built from sanitized address
		if err != nil {
			cancel()
			lastHTTPError = err
			break
		}
		resp, err := node.httpClient.Do(req) // #nosec -- url built from sanitized address; transport enforces safedial
		if err != nil {
			cancel()
			logger.Debug("block sync: dial failed",
				"nodeQuid", nodeQuid, "page", pagesFetched, "error", err)
			node.recordPeerScore(nodeQuid, EventClassQuery, false, "block-sync dial: "+err.Error())
			return
		}
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			logger.Debug("block sync: non-2xx",
				"nodeQuid", nodeQuid, "page", pagesFetched, "status", resp.StatusCode)
			node.recordPeerScore(nodeQuid, EventClassQuery, false,
				"block-sync status "+strconv.Itoa(resp.StatusCode))
			_ = resp.Body.Close()
			cancel()
			return
		}
		body, err := io.ReadAll(io.LimitReader(resp.Body, 16<<20))
		_ = resp.Body.Close()
		cancel()
		if err != nil {
			lastHTTPError = err
			break
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
				"nodeQuid", nodeQuid, "page", pagesFetched, "error", err)
			lastHTTPError = err
			break
		}

		pageSize := len(env.Data.Data)
		if pageSize == 0 {
			// Past the peer's tail. Done.
			break
		}

		for _, b := range env.Data.Data {
			// ENG-82: hash-dedup replaces the old index filter.
			// A block we already hold (by hash) is a no-op
			// regardless of which domain or validator it
			// belongs to.
			if _, dup := seenHashes[b.Hash]; dup {
				totalSkipped++
				continue
			}
			// Reserve the hash before calling ReceiveBlock so
			// retries inside the same cycle don't re-attempt.
			seenHashes[b.Hash] = struct{}{}

			acceptance, err := node.ReceiveBlock(b)
			if err != nil {
				logger.Debug("block sync: peer-served block rejected",
					"nodeQuid", nodeQuid,
					"blockIndex", b.Index,
					"blockHash", b.Hash,
					"domain", b.TrustProof.TrustDomain,
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
				totalApplied++
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

		peerOffset += int64(pageSize)
		// Partial page: we've reached the peer's tail. Stop
		// without making another request.
		if pageSize < blockSyncBatchLimit {
			break
		}
	}

	if lastHTTPError != nil {
		logger.Debug("block sync: pagination ended on error",
			"nodeQuid", nodeQuid, "page", pagesFetched, "error", lastHTTPError)
	}
	if totalApplied > 0 || totalSkipped > 0 {
		logger.Info("Block sync cycle complete",
			"nodeQuid", nodeQuid,
			"applied", totalApplied,
			"skipped", totalSkipped,
			"pages", pagesFetched,
			"localChainLen", len(seenHashes))
	}
	node.recordPeerScore(nodeQuid, EventClassQuery, true, "")
}
