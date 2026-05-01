package core

import (
	"testing"
)

// seedTrust populates node.TrustRegistry with a (truster →
// trustee → weight) edge. Test helper; bypasses the normal
// validation chain because we just want to exercise the
// aggregate function.
func seedTrust(node *QuidnugNode, truster, trustee string, weight float64) {
	node.TrustRegistryMutex.Lock()
	defer node.TrustRegistryMutex.Unlock()
	if node.TrustRegistry[truster] == nil {
		node.TrustRegistry[truster] = make(map[string]float64)
	}
	node.TrustRegistry[truster][trustee] = weight
}

// TestOperatorReputation_NoTrustReturnsZero: an operator with
// no incoming TRUST edges from anyone reports 0.
func TestOperatorReputation_NoTrustReturnsZero(t *testing.T) {
	node := newTestNode()
	got := node.operatorReputation("nobody")
	if got != 0 {
		t.Fatalf("got %v want 0", got)
	}
}

// TestOperatorReputation_DirectTrustWinsImmediately: when this
// node directly trusts the operator, the aggregate equals the
// direct grant.
func TestOperatorReputation_DirectTrustWinsImmediately(t *testing.T) {
	node := newTestNode()
	seedTrust(node, node.NodeID, "operatorX", 0.7)
	got := node.operatorReputation("operatorX")
	if got != 0.7 {
		t.Fatalf("got %v want 0.7", got)
	}
}

// TestOperatorReputation_WeightedByMyTrust: when an indirect
// truster T has trust m_t from this node, T's grant g_t to
// operator contributes m_t * g_t to numerator and m_t to
// denominator.
//
// Setup:
//   me trusts T1 at 0.5
//   me trusts T2 at 1.0
//   T1 trusts operator at 0.4
//   T2 trusts operator at 0.8
//
// Aggregate = (0.5*0.4 + 1.0*0.8) / (0.5 + 1.0)
//           = (0.2 + 0.8) / 1.5
//           ≈ 0.667
func TestOperatorReputation_WeightedByMyTrust(t *testing.T) {
	node := newTestNode()
	seedTrust(node, node.NodeID, "T1", 0.5)
	seedTrust(node, node.NodeID, "T2", 1.0)
	seedTrust(node, "T1", "operatorX", 0.4)
	seedTrust(node, "T2", "operatorX", 0.8)

	got := node.operatorReputation("operatorX")
	want := (0.5*0.4 + 1.0*0.8) / (0.5 + 1.0)
	if abs(got-want) > 1e-9 {
		t.Fatalf("got %v want %v", got, want)
	}
}

// TestOperatorReputation_IgnoresUntrustedTrusters: a truster I
// don't trust at all contributes nothing to the aggregate.
func TestOperatorReputation_IgnoresUntrustedTrusters(t *testing.T) {
	node := newTestNode()
	seedTrust(node, node.NodeID, "T1", 0.5)
	// T_random trusts operator with weight 1.0, but I don't
	// trust T_random.
	seedTrust(node, "T_random", "operatorX", 1.0)
	seedTrust(node, "T1", "operatorX", 0.4)

	got := node.operatorReputation("operatorX")
	// Aggregate = (0.5 * 0.4) / 0.5 = 0.4
	if abs(got-0.4) > 1e-9 {
		t.Fatalf("got %v want 0.4 (T_random should be ignored)", got)
	}
}

// TestOperatorReputation_DirectAndIndirectStack: when this
// node trusts the operator directly AND has indirect trusters,
// both contributions count.
func TestOperatorReputation_DirectAndIndirectStack(t *testing.T) {
	node := newTestNode()
	// Direct: me → operator at 0.6
	seedTrust(node, node.NodeID, "operatorX", 0.6)
	// Indirect: me → T1 at 1.0, T1 → operator at 1.0
	seedTrust(node, node.NodeID, "T1", 1.0)
	seedTrust(node, "T1", "operatorX", 1.0)

	got := node.operatorReputation("operatorX")
	// Aggregate = (1.0*0.6 + 1.0*1.0) / (1.0 + 1.0) = 0.8
	if abs(got-0.8) > 1e-9 {
		t.Fatalf("got %v want 0.8", got)
	}
}

// TestOperatorReputation_CacheReturnsStale: the cache is
// supposed to hold values for 5 min. We don't fast-forward
// time here, but we can verify the cache is populated by
// changing the trust graph after a query and confirming the
// query result doesn't reflect the change (until TTL elapses,
// which we don't simulate).
func TestOperatorReputation_CacheReturnsStale(t *testing.T) {
	node := newTestNode()
	seedTrust(node, node.NodeID, "operatorX", 0.5)

	first := node.operatorReputation("operatorX")
	if first != 0.5 {
		t.Fatalf("first call: got %v want 0.5", first)
	}
	// Change the underlying trust.
	seedTrust(node, node.NodeID, "operatorX", 0.9)
	second := node.operatorReputation("operatorX")
	// Cache is fresh, so we still see 0.5.
	if second != 0.5 {
		t.Fatalf("second call (cached): got %v want 0.5", second)
	}
}

// TestHasIncomingTrustToOperator_StillWorks: the deprecated
// helper still functions as a binary threshold check, just
// implemented via the new aggregate.
func TestHasIncomingTrustToOperator_StillWorks(t *testing.T) {
	node := newTestNode()
	seedTrust(node, node.NodeID, "operatorX", 0.7)

	if !node.hasIncomingTrustToOperator("operatorX", 0.5) {
		t.Fatal("operatorX should clear 0.5 threshold")
	}
	if node.hasIncomingTrustToOperator("operatorX", 0.8) {
		t.Fatal("operatorX should fail 0.8 threshold")
	}
}

func abs(x float64) float64 {
	if x < 0 {
		return -x
	}
	return x
}
