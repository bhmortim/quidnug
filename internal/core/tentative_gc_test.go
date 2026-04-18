// Package core — tentative_gc_test.go
//
// Methodology
// -----------
// Tests the synchronous pruneExpiredTentativeBlocks function — the
// long-running runTentativeBlockGC goroutine is just a time-ticker
// around it, so unit-testing the prune function covers the
// interesting logic without timing flakes.
//
// Invariants checked:
//   * Age-based eviction: blocks older than maxAge are removed, newer
//     ones stay.
//   * Empty domain keys are deleted (no phantom empty slices left
//     behind to bloat the map).
//   * Nonce reservations held by pruned blocks are released from the
//     ledger's tentative map, but never below the accepted floor —
//     ReleaseTentative's clamp-to-accepted behavior is verified on a
//     scenario where accepted=10, tentative=15, and the pruned block
//     held the 15 reservation. Post-prune tentative must clamp to 10.
package core

import (
	"testing"
	"time"
)

func TestPruneExpiredTentativeBlocks_RemovesAged(t *testing.T) {
	node := newTestNode()

	domain := "test.domain.com"
	now := time.Now().Unix()
	node.TentativeBlocks[domain] = []Block{
		{Index: 1, Timestamp: now - 3600, TrustProof: TrustProof{TrustDomain: domain}},   // old
		{Index: 2, Timestamp: now - 60, TrustProof: TrustProof{TrustDomain: domain}},     // young
	}

	pruned := node.pruneExpiredTentativeBlocks(30 * time.Minute)
	if pruned != 1 {
		t.Fatalf("want 1 pruned, got %d", pruned)
	}
	if got := len(node.TentativeBlocks[domain]); got != 1 {
		t.Fatalf("want 1 remaining block, got %d", got)
	}
	if node.TentativeBlocks[domain][0].Index != 2 {
		t.Fatalf("wrong block remained: %+v", node.TentativeBlocks[domain])
	}
}

func TestPruneExpiredTentativeBlocks_DeletesEmptyDomains(t *testing.T) {
	node := newTestNode()
	domain := "empty-after-prune.example"
	old := time.Now().Add(-1 * time.Hour).Unix()

	node.TentativeBlocks[domain] = []Block{
		{Index: 1, Timestamp: old, TrustProof: TrustProof{TrustDomain: domain}},
	}

	node.pruneExpiredTentativeBlocks(30 * time.Minute)

	if _, still := node.TentativeBlocks[domain]; still {
		t.Fatalf("expected domain key to be removed after all blocks pruned")
	}
}

func TestPruneExpiredTentativeBlocks_ReleasesNonceReservations(t *testing.T) {
	node := newTestNode()

	domain := "release-test.example"
	old := time.Now().Add(-1 * time.Hour).Unix()

	// Block reserved some tentative nonce space; simulate that first.
	key := NonceKey{Quid: "quidA", Domain: domain, Epoch: 0}
	node.NonceLedger.ReserveTentative(key, 5)
	if got := node.NonceLedger.Tentative(key); got != 5 {
		t.Fatalf("pre-GC tentative should be 5, got %d", got)
	}

	node.TentativeBlocks[domain] = []Block{
		{
			Index: 1, Timestamp: old, TrustProof: TrustProof{TrustDomain: domain},
			NonceCheckpoints: []NonceCheckpoint{{Quid: "quidA", Domain: domain, Epoch: 0, MaxNonce: 5}},
		},
	}

	node.pruneExpiredTentativeBlocks(30 * time.Minute)

	// Nothing else reserved nonce 5, so tentative should collapse to
	// the accepted floor (0).
	if got := node.NonceLedger.Tentative(key); got != 0 {
		t.Fatalf("post-GC tentative should revert to 0, got %d", got)
	}
}

func TestPruneExpiredTentativeBlocks_NoReservationsSurviveAcceptedFloor(t *testing.T) {
	node := newTestNode()

	domain := "floor-test.example"
	old := time.Now().Add(-1 * time.Hour).Unix()

	// Commit 10 as accepted, reserve 15 tentatively via the block.
	key := NonceKey{Quid: "quidA", Domain: domain, Epoch: 0}
	node.NonceLedger.CommitAccepted(key, 10)
	node.NonceLedger.ReserveTentative(key, 15)

	node.TentativeBlocks[domain] = []Block{
		{
			Index: 1, Timestamp: old, TrustProof: TrustProof{TrustDomain: domain},
			NonceCheckpoints: []NonceCheckpoint{{Quid: "quidA", Domain: domain, Epoch: 0, MaxNonce: 15}},
		},
	}

	node.pruneExpiredTentativeBlocks(30 * time.Minute)

	// Released tentative clamps to the accepted floor (10), not zero.
	if got := node.NonceLedger.Tentative(key); got != 10 {
		t.Fatalf("post-GC tentative should clamp to accepted floor (10), got %d", got)
	}
	if got := node.NonceLedger.Accepted(key); got != 10 {
		t.Fatalf("accepted must not be touched by prune: got %d", got)
	}
}
