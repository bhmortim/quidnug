// Top-domain statistics for the operator-facing dashboard.
//
// Three metrics surface per trust domain:
//
//   * ChainLength  — number of blocks where Block.TrustProof.TrustDomain
//                     matches the domain. Counts the chain depth this
//                     domain has accumulated.
//   * QueriesServed — count of /api/v1/domains/{name}/query calls
//                     served for the domain since this process booted.
//                     Tracked via QuidnugNode.DomainQueryCounts (atomic
//                     map keyed by domain name).
//   * TxVolume     — total transactions in the chain whose
//                     BaseTransaction.TrustDomain equals the domain.
//                     Independent of which block they landed in.
//
// `/api/v1/domains/top` returns three top-10 lists (one per metric).
// The landing page renders a condensed version inline. Computation
// walks the blockchain once per refresh and is cached for 30s to
// keep the landing page cheap.
package core

import (
	"sort"
	"sync"
	"sync/atomic"
	"time"
)

// topDomainCacheTTL is how long the top-domain computation result is
// considered fresh. 30s matches the landing-page cache convention
// elsewhere in the codebase: long enough to absorb burst traffic,
// short enough that operators see fresh-ish numbers.
const topDomainCacheTTL = 30 * time.Second

// DomainStats is one row of the per-domain metrics. Used by both
// /api/v1/domains/top and the landing-page renderer.
type DomainStats struct {
	Domain        string `json:"domain"`
	ChainLength   int    `json:"chainLength"`
	QueriesServed int64  `json:"queriesServed"`
	TxVolume      int    `json:"txVolume"`
}

// TopDomains is the response shape for /api/v1/domains/top: three
// pre-sorted top-N lists, one per metric.
type TopDomains struct {
	ByChainLength   []DomainStats `json:"byChainLength"`
	ByQueriesServed []DomainStats `json:"byQueriesServed"`
	ByTxVolume      []DomainStats `json:"byTxVolume"`
	GeneratedAt     int64         `json:"generatedAt"`
	WindowSeconds   int64         `json:"windowSeconds"` // process lifetime for QueriesServed
}

// topDomainCache memoizes the result of TopDomains() computations.
// Updated every 30s; reads under read lock for cheap concurrent
// access from the landing-page handler.
type topDomainCache struct {
	mu       sync.RWMutex
	value    *TopDomains
	expiresAt time.Time
}

// IncrementDomainQueryCount bumps the per-domain query counter.
// Called by the QueryDomainHandler at the top of every successful
// query for a domain this node manages. Cheap and lock-free.
func (node *QuidnugNode) IncrementDomainQueryCount(domain string) {
	if node == nil || domain == "" {
		return
	}
	if node.DomainQueryCounts == nil {
		// Lazy init for tests that bypass NewQuidnugNode.
		node.domainQueryCountsInitMu.Lock()
		if node.DomainQueryCounts == nil {
			node.DomainQueryCounts = &sync.Map{}
		}
		node.domainQueryCountsInitMu.Unlock()
	}
	v, _ := node.DomainQueryCounts.LoadOrStore(domain, new(atomic.Int64))
	v.(*atomic.Int64).Add(1)
}

// domainQueryCountSnapshot returns the current per-domain query
// counts as a regular map for sort/render. Counts are read
// atomically; concurrent increments may produce a snapshot one or
// two events stale, which is fine for top-N display.
func (node *QuidnugNode) domainQueryCountSnapshot() map[string]int64 {
	out := make(map[string]int64)
	if node == nil || node.DomainQueryCounts == nil {
		return out
	}
	node.DomainQueryCounts.Range(func(k, v interface{}) bool {
		domain, ok := k.(string)
		if !ok {
			return true
		}
		ctr, ok := v.(*atomic.Int64)
		if !ok {
			return true
		}
		out[domain] = ctr.Load()
		return true
	})
	return out
}

// TopDomainsTopN returns the top-N domains for each of the three
// metrics. Uses the per-node cache when fresh; otherwise walks the
// blockchain once and rebuilds. n=10 is the operator-facing
// default; tests may pass other values.
func (node *QuidnugNode) TopDomainsTopN(n int) *TopDomains {
	if node == nil {
		return &TopDomains{}
	}
	now := time.Now()
	// Fast path: cache hit.
	node.topDomainCache.mu.RLock()
	if node.topDomainCache.value != nil && now.Before(node.topDomainCache.expiresAt) {
		out := node.topDomainCache.value
		node.topDomainCache.mu.RUnlock()
		return out
	}
	node.topDomainCache.mu.RUnlock()

	// Cache miss: recompute. Only one goroutine should walk the
	// chain at a time; the second caller blocks on the write
	// lock and gets the recomputed result for free.
	node.topDomainCache.mu.Lock()
	defer node.topDomainCache.mu.Unlock()
	// Re-check inside the write lock — a concurrent recompute
	// may have populated by now.
	if node.topDomainCache.value != nil && now.Before(node.topDomainCache.expiresAt) {
		return node.topDomainCache.value
	}

	stats := node.computeAllDomainStats()
	out := &TopDomains{
		ByChainLength:   topNByChainLength(stats, n),
		ByQueriesServed: topNByQueriesServed(stats, n),
		ByTxVolume:      topNByTxVolume(stats, n),
		GeneratedAt:     now.Unix(),
	}
	if !node.processStartedAt.IsZero() {
		out.WindowSeconds = int64(now.Sub(node.processStartedAt).Seconds())
	}

	node.topDomainCache.value = out
	node.topDomainCache.expiresAt = now.Add(topDomainCacheTTL)
	return out
}

// computeAllDomainStats walks the blockchain once and collates
// chain-length + tx-volume per domain, then merges in the
// QueriesServed counter. Returns the un-sorted map; caller does
// the per-metric sort. O(blocks * average tx count).
func (node *QuidnugNode) computeAllDomainStats() map[string]*DomainStats {
	out := make(map[string]*DomainStats)
	upsert := func(domain string) *DomainStats {
		if domain == "" {
			return nil
		}
		s, ok := out[domain]
		if !ok {
			s = &DomainStats{Domain: domain}
			out[domain] = s
		}
		return s
	}

	// Walk blockchain. Hold the read lock for the duration; the
	// chain is rarely mutated during a read window.
	node.BlockchainMutex.RLock()
	for _, b := range node.Blockchain {
		// Block-level domain (TrustProof.TrustDomain) drives
		// ChainLength.
		if s := upsert(b.TrustProof.TrustDomain); s != nil {
			s.ChainLength++
		}
		// Per-tx domain drives TxVolume. Re-marshal the
		// interface-typed transaction is overkill; we only need
		// the BaseTransaction.TrustDomain field, which is in
		// the JSON-decoded map at key "trustDomain".
		for _, tx := range b.Transactions {
			d := extractTxDomain(tx)
			if s := upsert(d); s != nil {
				s.TxVolume++
			}
		}
	}
	node.BlockchainMutex.RUnlock()

	// Merge in per-domain query counters.
	for domain, count := range node.domainQueryCountSnapshot() {
		s := upsert(domain)
		if s != nil {
			s.QueriesServed = count
		}
	}

	return out
}

// extractTxDomain pulls the trustDomain field out of a transaction
// regardless of whether it was unmarshaled into a typed struct or
// left as a generic map. Returns the empty string when no domain
// field is present (e.g. anchor transactions which carry domain
// inside a nested record).
func extractTxDomain(tx interface{}) string {
	switch v := tx.(type) {
	case map[string]interface{}:
		if d, ok := v["trustDomain"].(string); ok {
			return d
		}
	case BaseTransaction:
		return v.TrustDomain
	case TrustTransaction:
		return v.TrustDomain
	case IdentityTransaction:
		return v.TrustDomain
	case TitleTransaction:
		return v.TrustDomain
	case EventTransaction:
		return v.TrustDomain
	case NodeAdvertisementTransaction:
		return v.TrustDomain
	case ModerationActionTransaction:
		return v.TrustDomain
	}
	return ""
}

// topNByChainLength sorts a stats map descending by ChainLength,
// tie-breaking by TxVolume then by domain name for determinism.
// Returns at most n entries; entries with ChainLength == 0 are
// excluded so the list reflects domains the node actually has
// chain history for.
func topNByChainLength(stats map[string]*DomainStats, n int) []DomainStats {
	rows := make([]DomainStats, 0, len(stats))
	for _, s := range stats {
		if s.ChainLength == 0 {
			continue
		}
		rows = append(rows, *s)
	}
	sort.Slice(rows, func(i, j int) bool {
		if rows[i].ChainLength != rows[j].ChainLength {
			return rows[i].ChainLength > rows[j].ChainLength
		}
		if rows[i].TxVolume != rows[j].TxVolume {
			return rows[i].TxVolume > rows[j].TxVolume
		}
		return rows[i].Domain < rows[j].Domain
	})
	if len(rows) > n {
		rows = rows[:n]
	}
	return rows
}

// topNByQueriesServed sorts descending by QueriesServed, tie-
// breaking by ChainLength. Excludes domains with zero queries so
// the list shows what's actually being asked about.
func topNByQueriesServed(stats map[string]*DomainStats, n int) []DomainStats {
	rows := make([]DomainStats, 0, len(stats))
	for _, s := range stats {
		if s.QueriesServed == 0 {
			continue
		}
		rows = append(rows, *s)
	}
	sort.Slice(rows, func(i, j int) bool {
		if rows[i].QueriesServed != rows[j].QueriesServed {
			return rows[i].QueriesServed > rows[j].QueriesServed
		}
		if rows[i].ChainLength != rows[j].ChainLength {
			return rows[i].ChainLength > rows[j].ChainLength
		}
		return rows[i].Domain < rows[j].Domain
	})
	if len(rows) > n {
		rows = rows[:n]
	}
	return rows
}

// topNByTxVolume sorts descending by TxVolume, tie-breaking by
// ChainLength. Excludes domains with zero transactions.
func topNByTxVolume(stats map[string]*DomainStats, n int) []DomainStats {
	rows := make([]DomainStats, 0, len(stats))
	for _, s := range stats {
		if s.TxVolume == 0 {
			continue
		}
		rows = append(rows, *s)
	}
	sort.Slice(rows, func(i, j int) bool {
		if rows[i].TxVolume != rows[j].TxVolume {
			return rows[i].TxVolume > rows[j].TxVolume
		}
		if rows[i].ChainLength != rows[j].ChainLength {
			return rows[i].ChainLength > rows[j].ChainLength
		}
		return rows[i].Domain < rows[j].Domain
	})
	if len(rows) > n {
		rows = rows[:n]
	}
	return rows
}
