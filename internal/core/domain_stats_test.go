package core

import (
	"sync"
	"testing"
	"time"
)

// helperBlockForDomain builds a Block whose TrustProof.TrustDomain
// matches `domain` and whose `Transactions` carry `txCount`
// fake transactions for that same domain. Test scaffolding —
// nothing here has to validate cryptographically.
func helperBlockForDomain(idx int64, domain string, txCount int) Block {
	txs := make([]interface{}, 0, txCount)
	for i := 0; i < txCount; i++ {
		txs = append(txs, map[string]interface{}{
			"trustDomain": domain,
			"id":          "tx-test",
		})
	}
	return Block{
		Index:        idx,
		Timestamp:    time.Now().Unix(),
		Transactions: txs,
		TrustProof:   TrustProof{TrustDomain: domain},
		Hash:         "h",
	}
}

func newDomainStatsTestNode() *QuidnugNode {
	return &QuidnugNode{
		Blockchain:        []Block{},
		DomainQueryCounts: &sync.Map{},
		processStartedAt:  time.Now(),
	}
}

// TestTopDomains_ChainLengthRanking verifies that domains with
// more blocks sealed in them sort higher.
func TestTopDomains_ChainLengthRanking(t *testing.T) {
	n := newDomainStatsTestNode()
	for i := int64(0); i < 5; i++ {
		n.Blockchain = append(n.Blockchain, helperBlockForDomain(i, "alpha.example", 0))
	}
	for i := int64(5); i < 8; i++ {
		n.Blockchain = append(n.Blockchain, helperBlockForDomain(i, "beta.example", 0))
	}
	n.Blockchain = append(n.Blockchain, helperBlockForDomain(8, "gamma.example", 0))

	top := n.TopDomainsTopN(10)
	if len(top.ByChainLength) != 3 {
		t.Fatalf("expected 3 ranked domains, got %d", len(top.ByChainLength))
	}
	if top.ByChainLength[0].Domain != "alpha.example" || top.ByChainLength[0].ChainLength != 5 {
		t.Fatalf("unexpected #1: %+v", top.ByChainLength[0])
	}
	if top.ByChainLength[1].Domain != "beta.example" || top.ByChainLength[1].ChainLength != 3 {
		t.Fatalf("unexpected #2: %+v", top.ByChainLength[1])
	}
	if top.ByChainLength[2].Domain != "gamma.example" || top.ByChainLength[2].ChainLength != 1 {
		t.Fatalf("unexpected #3: %+v", top.ByChainLength[2])
	}
}

// TestTopDomains_QueryRanking verifies that the per-domain query
// counter feeds the QueriesServed metric and the ranking sorts
// descending.
func TestTopDomains_QueryRanking(t *testing.T) {
	n := newDomainStatsTestNode()
	// Need at least one block per domain so chain-length is
	// non-zero (the QueriesServed list doesn't filter on chain
	// length, but we still need entries in the stats map to
	// rank).
	n.Blockchain = append(n.Blockchain,
		helperBlockForDomain(0, "alpha.example", 0),
		helperBlockForDomain(1, "beta.example", 0))
	for i := 0; i < 100; i++ {
		n.IncrementDomainQueryCount("alpha.example")
	}
	for i := 0; i < 25; i++ {
		n.IncrementDomainQueryCount("beta.example")
	}
	// gamma never queried — should be excluded from ranking.
	n.Blockchain = append(n.Blockchain, helperBlockForDomain(2, "gamma.example", 0))

	top := n.TopDomainsTopN(10)
	if len(top.ByQueriesServed) != 2 {
		t.Fatalf("expected 2 queried domains, got %d", len(top.ByQueriesServed))
	}
	if top.ByQueriesServed[0].Domain != "alpha.example" || top.ByQueriesServed[0].QueriesServed != 100 {
		t.Fatalf("unexpected #1: %+v", top.ByQueriesServed[0])
	}
	if top.ByQueriesServed[1].Domain != "beta.example" || top.ByQueriesServed[1].QueriesServed != 25 {
		t.Fatalf("unexpected #2: %+v", top.ByQueriesServed[1])
	}
}

// TestTopDomains_TxVolumeRanking verifies that transactions are
// counted by their own per-tx trustDomain field, not the block's
// TrustProof.TrustDomain. A block sealed in domain X may carry
// transactions for domains Y and Z.
func TestTopDomains_TxVolumeRanking(t *testing.T) {
	n := newDomainStatsTestNode()
	// Block sealed in alpha with 5 alpha-domain transactions.
	n.Blockchain = append(n.Blockchain, helperBlockForDomain(0, "alpha.example", 5))
	// Block sealed in alpha but carrying beta-domain
	// transactions (this is the cross-domain case).
	betaBlock := helperBlockForDomain(1, "alpha.example", 0)
	for i := 0; i < 8; i++ {
		betaBlock.Transactions = append(betaBlock.Transactions, map[string]interface{}{
			"trustDomain": "beta.example",
		})
	}
	n.Blockchain = append(n.Blockchain, betaBlock)

	top := n.TopDomainsTopN(10)
	if len(top.ByTxVolume) != 2 {
		t.Fatalf("expected 2 tx-volume entries, got %d: %+v", len(top.ByTxVolume), top.ByTxVolume)
	}
	if top.ByTxVolume[0].Domain != "beta.example" || top.ByTxVolume[0].TxVolume != 8 {
		t.Fatalf("unexpected #1: %+v", top.ByTxVolume[0])
	}
	if top.ByTxVolume[1].Domain != "alpha.example" || top.ByTxVolume[1].TxVolume != 5 {
		t.Fatalf("unexpected #2: %+v", top.ByTxVolume[1])
	}
}

// TestTopDomains_CacheHit confirms that two calls within the TTL
// window return the same pointer (i.e. the cache short-circuits
// the chain walk).
func TestTopDomains_CacheHit(t *testing.T) {
	n := newDomainStatsTestNode()
	n.Blockchain = append(n.Blockchain, helperBlockForDomain(0, "alpha.example", 1))
	a := n.TopDomainsTopN(10)
	b := n.TopDomainsTopN(10)
	if a != b {
		t.Fatal("expected cached pointer-equal result")
	}
}

// TestTopDomains_CacheRespectsTTL confirms that after the TTL
// elapses, recomputation produces a fresh result reflecting new
// state. Forces TTL by manipulating expiresAt directly.
func TestTopDomains_CacheRespectsTTL(t *testing.T) {
	n := newDomainStatsTestNode()
	n.Blockchain = append(n.Blockchain, helperBlockForDomain(0, "alpha.example", 0))
	first := n.TopDomainsTopN(10)
	// Add new block AND backdate the cache.
	n.Blockchain = append(n.Blockchain, helperBlockForDomain(1, "alpha.example", 0))
	n.topDomainCache.mu.Lock()
	n.topDomainCache.expiresAt = time.Now().Add(-1 * time.Second)
	n.topDomainCache.mu.Unlock()
	second := n.TopDomainsTopN(10)
	if first == second {
		t.Fatal("expected fresh pointer after TTL expiry")
	}
	if second.ByChainLength[0].ChainLength != 2 {
		t.Fatalf("expected refreshed chain length 2, got %d", second.ByChainLength[0].ChainLength)
	}
}

// TestTopDomains_CapAtN verifies that the top-N cap is enforced.
func TestTopDomains_CapAtN(t *testing.T) {
	n := newDomainStatsTestNode()
	for i := int64(0); i < 15; i++ {
		domain := "domain-" + string(rune('a'+int(i)))
		n.Blockchain = append(n.Blockchain, helperBlockForDomain(i, domain, 0))
	}
	top := n.TopDomainsTopN(10)
	if len(top.ByChainLength) != 10 {
		t.Fatalf("expected cap at 10, got %d", len(top.ByChainLength))
	}
}

// TestIncrementDomainQueryCount_Concurrent confirms the counter
// is safe under concurrent increments from multiple goroutines.
func TestIncrementDomainQueryCount_Concurrent(t *testing.T) {
	n := newDomainStatsTestNode()
	const goroutines = 8
	const perGoroutine = 1000
	var wg sync.WaitGroup
	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < perGoroutine; j++ {
				n.IncrementDomainQueryCount("alpha.example")
			}
		}()
	}
	wg.Wait()
	snap := n.domainQueryCountSnapshot()
	if snap["alpha.example"] != int64(goroutines*perGoroutine) {
		t.Fatalf("expected %d, got %d", goroutines*perGoroutine, snap["alpha.example"])
	}
}
