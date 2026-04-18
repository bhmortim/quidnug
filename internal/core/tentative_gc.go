package core

import (
	"context"
	"time"
)

// TentativeBlock retention defaults. Per QDP-0001 §6.4, tentative blocks
// whose TrustedCommit never happens must eventually be pruned or they
// accumulate unbounded and (with checkpoints) hold nonce space hostage.
//
// DefaultTentativeBlockMaxAge is deliberately conservative: it must be
// long enough that a tentative block has a realistic chance of being
// promoted to Trusted (via subsequent trust transactions lifting the
// creator's relational trust score above the domain threshold) but short
// enough that a rogue producer spamming tentative blocks cannot exhaust
// memory.
const (
	DefaultTentativeBlockMaxAge = 30 * time.Minute
	DefaultTentativeGCInterval  = 2 * time.Minute
)

// runTentativeBlockGC is a long-running goroutine that periodically
// prunes tentative blocks whose age exceeds maxAge. It is launched from
// Run() (see node.go) when the process starts.
//
// Each prune wakes on the ticker, iterates TentativeBlocks under the
// TentativeBlocksMutex, drops aged entries, and releases their nonce
// reservations from the ledger so the nonce stream can resume. A block
// may still be kept if another tentative block in the same domain holds
// an equal-or-higher reservation for the same signer — that
// bookkeeping happens inside ReleaseTentative.
func (node *QuidnugNode) runTentativeBlockGC(ctx context.Context, interval, maxAge time.Duration) {
	if interval <= 0 {
		interval = DefaultTentativeGCInterval
	}
	if maxAge <= 0 {
		maxAge = DefaultTentativeBlockMaxAge
	}

	logger.Info("Starting tentative-block GC loop",
		"interval", interval, "maxAge", maxAge)

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			logger.Info("Tentative-block GC loop stopped")
			return
		case <-ticker.C:
			node.pruneExpiredTentativeBlocks(maxAge)
		}
	}
}

// pruneExpiredTentativeBlocks removes tentative blocks older than maxAge
// and updates the nonce ledger to release their reservations.
//
// Returns the number of blocks pruned, for tests and observability.
func (node *QuidnugNode) pruneExpiredTentativeBlocks(maxAge time.Duration) int {
	cutoff := time.Now().Add(-maxAge).Unix()

	node.TentativeBlocksMutex.Lock()
	defer node.TentativeBlocksMutex.Unlock()

	pruned := 0
	for domain, blocks := range node.TentativeBlocks {
		kept := blocks[:0]
		for _, b := range blocks {
			if b.Timestamp < cutoff {
				pruned++
				// Release the nonce reservations this block held.
				// NOTE: the ledger's ReleaseTentative already clamps to
				// the accepted floor, so releasing to 0 collapses to
				// "whatever is actually accepted" — which is exactly
				// the right floor after the block disappears.
				if node.NonceLedger != nil {
					for _, cp := range b.NonceCheckpoints {
						node.NonceLedger.ReleaseTentative(
							NonceKey{Quid: cp.Quid, Domain: cp.Domain, Epoch: cp.Epoch},
							0,
						)
					}
				}
				continue
			}
			kept = append(kept, b)
		}
		if len(kept) == 0 {
			delete(node.TentativeBlocks, domain)
		} else {
			node.TentativeBlocks[domain] = kept
		}
	}

	if pruned > 0 {
		logger.Info("Pruned expired tentative blocks",
			"count", pruned, "maxAge", maxAge)
	}
	return pruned
}
