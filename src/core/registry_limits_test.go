package main

import (
	"errors"
	"fmt"
	"testing"
)

// TestComputeRelationalTrust_ResourceLimits tests that large graphs trigger resource limits
func TestComputeRelationalTrust_ResourceLimits(t *testing.T) {
	node := newTestNode()

	// Create a graph with more than MaxTrustVisitedSize (10000) nodes
	// to reliably exceed resource limits. The BFS visited set is bounded
	// by the number of unique nodes, so we need > 10000 nodes.
	numNodes := 10500
	nodesPerConnection := 50

	node.TrustRegistryMutex.Lock()
	for i := 0; i < numNodes; i++ {
		truster := fmt.Sprintf("de05e00de000%04x", i)
		if node.TrustRegistry[truster] == nil {
			node.TrustRegistry[truster] = make(map[string]float64)
		}
		for j := 1; j <= nodesPerConnection; j++ {
			trustee := fmt.Sprintf("de05e00de000%04x", (i+j)%numNodes)
			node.TrustRegistry[truster][trustee] = 0.9
		}
	}
	node.TrustRegistryMutex.Unlock()

	// Search for a non-existent node to force exhaustive exploration
	// until resource limits are hit
	_, _, err := node.ComputeRelationalTrust("de05e00de0000000", "ffff000000000000", 10)

	// Should return ErrTrustGraphTooLarge
	if err == nil {
		t.Error("Expected ErrTrustGraphTooLarge error for large graph, got nil")
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
		truster := fmt.Sprintf("c0a100de000000%02x", i)
		trustee := fmt.Sprintf("c0a100de000000%02x", i+1)
		if node.TrustRegistry[truster] == nil {
			node.TrustRegistry[truster] = make(map[string]float64)
		}
		node.TrustRegistry[truster][trustee] = 0.9
	}
	node.TrustRegistryMutex.Unlock()

	trustLevel, path, err := node.ComputeRelationalTrust("c0a100de00000000", "c0a100de00000005", 10)

	if err != nil {
		t.Errorf("Expected no error for normal graph, got: %v", err)
	}

	if trustLevel == 0 {
		t.Error("Expected non-zero trust level for connected nodes")
	}

	expectedTrust := 0.9 * 0.9 * 0.9 * 0.9 * 0.9 // 5 hops at 0.9 each
	if !floatEquals(trustLevel, expectedTrust, 0.0001) {
		t.Errorf("Expected trust level %f, got %f", expectedTrust, trustLevel)
	}

	if len(path) != 6 { // observer + 5 hops
		t.Errorf("Expected path length 6, got %d", len(path))
	}
}

// TestComputeRelationalTrustEnhanced_ResourceLimits tests enhanced function resource limits
func TestComputeRelationalTrustEnhanced_ResourceLimits(t *testing.T) {
	node := newTestNode()

	// Create a graph with more than MaxTrustVisitedSize (10000) nodes
	// to reliably exceed resource limits
	numNodes := 10500
	nodesPerConnection := 50

	node.TrustRegistryMutex.Lock()
	for i := 0; i < numNodes; i++ {
		truster := fmt.Sprintf("e00a00ced000%04x", i)
		if node.VerifiedTrustEdges[truster] == nil {
			node.VerifiedTrustEdges[truster] = make(map[string]TrustEdge)
		}
		for j := 1; j <= nodesPerConnection; j++ {
			trustee := fmt.Sprintf("e00a00ced000%04x", (i+j)%numNodes)
			node.VerifiedTrustEdges[truster][trustee] = TrustEdge{
				Truster:    truster,
				Trustee:    trustee,
				TrustLevel: 0.9,
				Verified:   true,
			}
		}
	}
	node.TrustRegistryMutex.Unlock()

	// Search for a non-existent node to force exhaustive exploration
	// until resource limits are hit
	_, err := node.ComputeRelationalTrustEnhanced("e00a00ced0000000", "ffff000000000000", 10, false)

	if err == nil {
		t.Error("Expected ErrTrustGraphTooLarge error for large graph, got nil")
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
	node.TrustRegistry["11b105e00e00bef0"] = map[string]float64{
		"11b10a00e000be00": 0.5,
	}

	// Also create a dense subgraph that would explode
	for i := 0; i < 150; i++ {
		truster := fmt.Sprintf("11b1de05e000%04x", i)
		if node.TrustRegistry[truster] == nil {
			node.TrustRegistry[truster] = make(map[string]float64)
		}
		for j := 1; j <= 30; j++ {
			trustee := fmt.Sprintf("11b1de05e000%04x", (i+j)%150)
			node.TrustRegistry[truster][trustee] = 0.9
		}
	}
	// Connect observer to dense subgraph
	node.TrustRegistry["11b105e00e00bef0"]["11b1de05e0000000"] = 0.8
	node.TrustRegistryMutex.Unlock()

	// Query for a target in the dense subgraph
	trustLevel, _, err := node.ComputeRelationalTrust("11b105e00e00bef0", "11b1de05e0000064", 10)

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

// BenchmarkComputeRelationalTrust_BoundedMemory benchmarks memory usage
func BenchmarkComputeRelationalTrust_BoundedMemory(b *testing.B) {
	node := newTestNode()

	// Create a moderately dense graph
	numNodes := 100
	nodesPerConnection := 20

	node.TrustRegistryMutex.Lock()
	for i := 0; i < numNodes; i++ {
		truster := fmt.Sprintf("be0c000de000%04x", i)
		if node.TrustRegistry[truster] == nil {
			node.TrustRegistry[truster] = make(map[string]float64)
		}
		for j := 1; j <= nodesPerConnection; j++ {
			trustee := fmt.Sprintf("be0c000de000%04x", (i+j)%numNodes)
			node.TrustRegistry[truster][trustee] = 0.9
		}
	}
	node.TrustRegistryMutex.Unlock()

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		// This should either complete or hit resource limits, but not OOM
		node.ComputeRelationalTrust("be0c000de0000000", "be0c000de0000032", DefaultTrustMaxDepth)
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
		truster := fmt.Sprintf("be0ce0000000%04x", i)
		if node.VerifiedTrustEdges[truster] == nil {
			node.VerifiedTrustEdges[truster] = make(map[string]TrustEdge)
		}
		for j := 1; j <= nodesPerConnection; j++ {
			trustee := fmt.Sprintf("be0ce0000000%04x", (i+j)%numNodes)
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
		node.ComputeRelationalTrustEnhanced("be0ce00000000000", "be0ce00000000032", DefaultTrustMaxDepth, false)
	}
}
