// Operator-reputation weighted aggregate (Phase 4f).
//
// The Phase 1 admit pipeline included a binary
// PeerMinOperatorReputation gate: any single locally-trusted
// quid granting TRUST ≥ threshold to the candidate's operator
// unlocked admission. That's coarse — one drive-by trust grant
// from any quid we happen to trust opens the door.
//
// This file replaces that binary check with a weighted aggregate:
//
//	operator_reputation(O) =
//	  Σ over (truster T): trust_to(O) × my_trust_in(T)
//	  / Σ over T: my_trust_in(T)
//
// "How much do my friends, weighted by how much I trust them,
// trust this operator?" The aggregate is in [0, 1]; the
// existing config knob's units are unchanged.
//
// my_trust_in(T) is itself a relational-trust query, but we cap
// it to a one-hop direct lookup here (operators we directly
// trust) to keep the aggregate cheap. A cache with 5-minute TTL
// (per the audit decision) avoids hot-path cost.
package core

import (
	"sync"
	"time"
)

// operatorReputationCacheTTL is how long an aggregate stays
// fresh before recomputation. The audit signed off on 5 min;
// operator reputation moves slowly, and the cache spares the
// admit hot path from repeated trust-graph walks.
const operatorReputationCacheTTL = 5 * time.Minute

// operatorReputationCache is the per-node memoization layer.
// Keyed by operator quid, value is the most-recent aggregate +
// the time it was computed.
type operatorReputationCache struct {
	mu    sync.RWMutex
	cache map[string]opRepEntry
}

type opRepEntry struct {
	value     float64
	computed  time.Time
}

// newOperatorReputationCache constructs an empty cache.
func newOperatorReputationCache() *operatorReputationCache {
	return &operatorReputationCache{cache: make(map[string]opRepEntry)}
}

// get returns a cached aggregate if it's still fresh; otherwise
// (false, ...).
func (c *operatorReputationCache) get(operator string) (float64, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	e, ok := c.cache[operator]
	if !ok {
		return 0, false
	}
	if time.Since(e.computed) > operatorReputationCacheTTL {
		return 0, false
	}
	return e.value, true
}

// put stores an aggregate.
func (c *operatorReputationCache) put(operator string, value float64) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.cache[operator] = opRepEntry{value: value, computed: time.Now()}
}

// operatorReputation returns the weighted-aggregate operator
// reputation for `operator` per the formula above. Returns 0
// when no quid we trust has any TRUST edge to operator.
//
// This replaces the binary semantics of
// hasIncomingTrustToOperator. The admit pipeline (Stage 5) is
// updated in the same change to consult this function and
// compare against PeerMinOperatorReputation as a continuous
// score rather than a yes/no signal.
func (node *QuidnugNode) operatorReputation(operator string) float64 {
	if operator == "" {
		return 0
	}
	if node.opReputationCache == nil {
		node.opReputationCache = newOperatorReputationCache()
	}
	if v, ok := node.opReputationCache.get(operator); ok {
		return v
	}
	v := node.computeOperatorReputation(operator)
	node.opReputationCache.put(operator, v)
	return v
}

// computeOperatorReputation runs the actual aggregation under
// the trust-registry lock. We cap "my trust in T" to a single
// hop: T must be in this node's direct TrustRegistry.
//
// The math: for each truster T that has granted TRUST to
// operator, weight T's grant by this node's trust in T.
//
// To avoid double-counting (e.g. if `node.NodeID` itself trusts
// operator directly), the node's own NodeID is treated as a
// trusted truster with weight 1.0. That makes a directly-
// granted edge from this node always pass any reasonable
// threshold, which is the operator-intuitive behavior: "I
// trust this operator, so I trust them."
func (node *QuidnugNode) computeOperatorReputation(operator string) float64 {
	node.TrustRegistryMutex.RLock()
	defer node.TrustRegistryMutex.RUnlock()

	var weightedSum, totalWeight float64

	// Direct-trust contribution from THIS node.
	if myEdges, ok := node.TrustRegistry[node.NodeID]; ok {
		if direct, hasDirect := myEdges[operator]; hasDirect {
			weightedSum += direct * 1.0
			totalWeight += 1.0
		}
	}

	// Indirect contribution from each (T → operator) edge,
	// weighted by my_trust_in(T).
	for trusterID, trusterEdges := range node.TrustRegistry {
		if trusterID == node.NodeID {
			continue
		}
		grant, hasGrant := trusterEdges[operator]
		if !hasGrant {
			continue
		}
		myWeight := myTrustInLocked(node, trusterID)
		if myWeight <= 0 {
			continue
		}
		weightedSum += grant * myWeight
		totalWeight += myWeight
	}

	if totalWeight <= 0 {
		return 0
	}
	return weightedSum / totalWeight
}

// myTrustInLocked is the one-hop trust query the aggregate
// uses. Caller holds node.TrustRegistryMutex (read or write).
// Returns 0 when this node has no direct TRUST edge to t.
func myTrustInLocked(node *QuidnugNode, t string) float64 {
	if t == node.NodeID {
		return 1.0
	}
	myEdges, ok := node.TrustRegistry[node.NodeID]
	if !ok {
		return 0
	}
	return myEdges[t]
}
