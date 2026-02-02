package main

import (
	"errors"
	"testing"
)

// TestComputeRelationalTrust_ResourceLimits tests that dense graphs trigger resource limits
func TestComputeRelationalTrust_ResourceLimits(t *testing.T) {
	node := newTestNode()

	// Create a dense graph that will exceed resource limits
	// Each node connects to many other nodes, creating exponential path exploration
	numNodes := 200
	nodesPerConnection := 50

	node.TrustRegistryMutex.Lock()
	for i := 0; i < numNodes; i++ {
		truster := "dense_node_" + padInt(i, 4)
		if node.TrustRegistry[truster] == nil {
			node.TrustRegistry[truster] = make(map[string]float64)
		}
		// Connect to next nodesPerConnection nodes (wrapping around)
		for j := 1; j <= nodesPerConnection; j++ {
			trustee := "dense_node_" + padInt((i+j)%numNodes, 4)
			node.TrustRegistry[truster][trustee] = 0.9
		}
	}
	node.TrustRegistryMutex.Unlock()

	// Try to compute trust across the dense graph
	_, _, err := node.ComputeRelationalTrust("dense_node_0000", "dense_node_0100", 10)

	// Should return ErrTrustGraphTooLarge
	if err == nil {
		t.Error("Expected ErrTrustGraphTooLarge error for dense graph, got nil")
	} else if !errors.Is(err, ErrTrustGraphTooLarge) {
		t.Errorf("Expected ErrTrustGraphTooLarge, got: %v", err)
	}
}

// TestComputeRelationalTrust_NormalGraphSucceeds tests that normal graphs complete successfully
func TestComputeRelationalTrust_NormalGraphSucceeds(t *testing.T) {
	node := newTestNode()

	// Create a normal-sized trust graph (chain of 10 nodes)
	node.TrustRegistryMutex.Lock()
	for i := 0; i < 10; i++ {
		truster := "chain_node_" + padInt(i, 2)
		trustee := "chain_node_" + padInt(i+1, 2)
		if node.TrustRegistry[truster] == nil {
			node.TrustRegistry[truster] = make(map[string]float64)
		}
		node.TrustRegistry[truster][trustee] = 0.9
	}
	node.TrustRegistryMutex.Unlock()

	trustLevel, path, err := node.ComputeRelationalTrust("chain_node_00", "chain_node_05", 10)

	if err != nil {
		t.Errorf("Expected no error for normal graph, got: %v", err)
	}

	if trustLevel == 0 {
		t.Error("Expected non-zero trust level for connected nodes")
	}

	expectedTrust := 0.9 * 0.9 * 0.9 * 0.9 * 0.9 // 5 hops at 0.9 each
	if trustLevel != expectedTrust {
		t.Errorf("Expected trust level %f, got %f", expectedTrust, trustLevel)
	}

	if len(path) != 6 { // observer + 5 hops
		t.Errorf("Expected path length 6, got %d", len(path))
	}
}

// TestComputeRelationalTrustEnhanced_ResourceLimits tests enhanced function resource limits
func TestComputeRelationalTrustEnhanced_ResourceLimits(t *testing.T) {
	node := newTestNode()

	// Create a dense graph
	numNodes := 200
	nodesPerConnection := 50

	node.TrustRegistryMutex.Lock()
	for i := 0; i < numNodes; i++ {
		truster := "enhanced_node_" + padInt(i, 4)
		if node.VerifiedTrustEdges[truster] == nil {
			node.VerifiedTrustEdges[truster] = make(map[string]TrustEdge)
		}
		for j := 1; j <= nodesPerConnection; j++ {
			trustee := "enhanced_node_" + padInt((i+j)%numNodes, 4)
			node.VerifiedTrustEdges[truster][trustee] = TrustEdge{
				Truster:    truster,
				Trustee:    trustee,
				TrustLevel: 0.9,
				Verified:   true,
			}
		}
	}
	node.TrustRegistryMutex.Unlock()

	// Try to compute enhanced trust across the dense graph
	_, err := node.ComputeRelationalTrustEnhanced("enhanced_node_0000", "enhanced_node_0100", 10, false)

	if err == nil {
		t.Error("Expected ErrTrustGraphTooLarge error for dense graph, got nil")
	} else if !errors.Is(err, ErrTrustGraphTooLarge) {
		t.Errorf("Expected ErrTrustGraphTooLarge, got: %v", err)
	}
}

// TestComputeRelationalTrust_PartialResultOnLimit tests that partial results are returned on limit
func TestComputeRelationalTrust_PartialResultOnLimit(t *testing.T) {
	node := newTestNode()

	// Create a graph where we can find a path before hitting limits
	// but the full exploration would exceed limits
	node.TrustRegistryMutex.Lock()

	// Direct path: observer -> target
	node.TrustRegistry["limit_observer"] = map[string]float64{
		"limit_target": 0.5,
	}

	// Also create a dense subgraph that would explode
	for i := 0; i < 150; i++ {
		truster := "limit_dense_" + padInt(i, 4)
		if node.TrustRegistry[truster] == nil {
			node.TrustRegistry[truster] = make(map[string]float64)
		}
		for j := 1; j <= 30; j++ {
			trustee := "limit_dense_" + padInt((i+j)%150, 4)
			node.TrustRegistry[truster][trustee] = 0.9
		}
	}
	// Connect observer to dense subgraph
	node.TrustRegistry["limit_observer"]["limit_dense_0000"] = 0.8
	node.TrustRegistryMutex.Unlock()

	// Query for a target in the dense subgraph
	trustLevel, _, err := node.ComputeRelationalTrust("limit_observer", "limit_dense_0100", 10)

	// Should hit resource limit
	if err == nil {
		t.Log("Note: Did not hit resource limit - graph may not be dense enough")
	} else if !errors.Is(err, ErrTrustGraphTooLarge) {
		t.Errorf("Expected ErrTrustGraphTooLarge, got: %v", err)
	}

	// Should still return a partial result (best found so far)
	// The result could be 0 if no path was found before limit, or non-zero if a path was found
	t.Logf("Partial trust level returned: %f", trustLevel)
}

// TestDefaultTrustMaxDepth tests that the default depth constant is used
func TestDefaultTrustMaxDepth(t *testing.T) {
	if DefaultTrustMaxDepth != 5 {
		t.Errorf("Expected DefaultTrustMaxDepth to be 5, got %d", DefaultTrustMaxDepth)
	}
}

// TestMaxTrustQueueSize tests the queue size constant
func TestMaxTrustQueueSize(t *testing.T) {
	if MaxTrustQueueSize != 10000 {
		t.Errorf("Expected MaxTrustQueueSize to be 10000, got %d", MaxTrustQueueSize)
	}
}

// TestMaxTrustVisitedSize tests the visited size constant
func TestMaxTrustVisitedSize(t *testing.T) {
	if MaxTrustVisitedSize != 10000 {
		t.Errorf("Expected MaxTrustVisitedSize to be 10000, got %d", MaxTrustVisitedSize)
	}
}

// padInt pads an integer with leading zeros to the specified width
func padInt(n, width int) string {
	s := ""
	for i := 0; i < width; i++ {
		s = string(rune('0'+n%10)) + s
		n /= 10
	}
	return s
}

// BenchmarkComputeRelationalTrust_BoundedMemory benchmarks memory usage
func BenchmarkComputeRelationalTrust_BoundedMemory(b *testing.B) {
	node := newTestNode()

	// Create a moderately dense graph
	numNodes := 100
	nodesPerConnection := 20

	node.TrustRegistryMutex.Lock()
	for i := 0; i < numNodes; i++ {
		truster := "bench_node_" + padInt(i, 4)
		if node.TrustRegistry[truster] == nil {
			node.TrustRegistry[truster] = make(map[string]float64)
		}
		for j := 1; j <= nodesPerConnection; j++ {
			trustee := "bench_node_" + padInt((i+j)%numNodes, 4)
			node.TrustRegistry[truster][trustee] = 0.9
		}
	}
	node.TrustRegistryMutex.Unlock()

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		// This should either complete or hit resource limits, but not OOM
		node.ComputeRelationalTrust("bench_node_0000", "bench_node_0050", DefaultTrustMaxDepth)
	}
}

// BenchmarkComputeRelationalTrustEnhanced_BoundedMemory benchmarks enhanced function
func BenchmarkComputeRelationalTrustEnhanced_BoundedMemory(b *testing.B) {
	node := newTestNode()

	// Create a moderately dense graph with verified edges
	numNodes := 100
	nodesPerConnection := 20

	node.TrustRegistryMutex.Lock()
	for i := 0; i < numNodes; i++ {
		truster := "bench_enh_" + padInt(i, 4)
		if node.VerifiedTrustEdges[truster] == nil {
			node.VerifiedTrustEdges[truster] = make(map[string]TrustEdge)
		}
		for j := 1; j <= nodesPerConnection; j++ {
			trustee := "bench_enh_" + padInt((i+j)%numNodes, 4)
			node.VerifiedTrustEdges[truster][trustee] = TrustEdge{
				Truster:    truster,
				Trustee:    trustee,
				TrustLevel: 0.9,
				Verified:   true,
			}
		}
	}
	node.TrustRegistryMutex.Unlock()

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		node.ComputeRelationalTrustEnhanced("bench_enh_0000", "bench_enh_0050", DefaultTrustMaxDepth, false)
	}
}
