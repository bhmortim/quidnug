package main

import (
	"testing"
)

func TestGetDirectTrustees(t *testing.T) {
	node := newTestNode()

	node.TrustRegistry["aaaa111111111111"] = map[string]float64{
		"bbbb222222222222": 0.8,
		"cccc333333333333": 0.6,
	}

	trustees := node.GetDirectTrustees("aaaa111111111111")

	if len(trustees) != 2 {
		t.Errorf("Expected 2 trustees, got %d", len(trustees))
	}

	if trustees["bbbb222222222222"] != 0.8 {
		t.Errorf("Expected trust 0.8 for bbbb222222222222, got %f", trustees["bbbb222222222222"])
	}

	if trustees["cccc333333333333"] != 0.6 {
		t.Errorf("Expected trust 0.6 for cccc333333333333, got %f", trustees["cccc333333333333"])
	}

	// Test non-existent quid returns empty map
	empty := node.GetDirectTrustees("00000000000000ff")
	if len(empty) != 0 {
		t.Errorf("Expected 0 trustees for non-existent quid, got %d", len(empty))
	}
}

func TestComputeRelationalTrust_SameEntity(t *testing.T) {
	node := newTestNode()

	trust, path, err := node.ComputeRelationalTrust("aaaa111111111111", "aaaa111111111111", 5)

	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	if trust != 1.0 {
		t.Errorf("Expected trust 1.0 for same entity, got %f", trust)
	}

	if len(path) != 1 || path[0] != "aaaa111111111111" {
		t.Errorf("Expected path [aaaa111111111111], got %v", path)
	}
}

func TestComputeRelationalTrust_DirectTrust(t *testing.T) {
	node := newTestNode()

	node.TrustRegistry["aaaa111111111111"] = map[string]float64{
		"bbbb222222222222": 0.8,
	}

	trust, path, err := node.ComputeRelationalTrust("aaaa111111111111", "bbbb222222222222", 5)

	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	if trust != 0.8 {
		t.Errorf("Expected trust 0.8, got %f", trust)
	}

	if len(path) != 2 || path[0] != "aaaa111111111111" || path[1] != "bbbb222222222222" {
		t.Errorf("Expected path [aaaa111111111111, bbbb222222222222], got %v", path)
	}
}

func TestComputeRelationalTrust_TwoHopWithDecay(t *testing.T) {
	node := newTestNode()

	// A trusts B with 0.8, B trusts C with 0.5
	// Transitive trust A->C should be 0.8 * 0.5 = 0.4
	node.TrustRegistry["aaaa111111111111"] = map[string]float64{
		"bbbb222222222222": 0.8,
	}
	node.TrustRegistry["bbbb222222222222"] = map[string]float64{
		"cccc333333333333": 0.5,
	}

	trust, path, err := node.ComputeRelationalTrust("aaaa111111111111", "cccc333333333333", 5)

	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	expected := 0.8 * 0.5
	if trust != expected {
		t.Errorf("Expected trust %f (0.8 * 0.5), got %f", expected, trust)
	}

	if len(path) != 3 {
		t.Errorf("Expected path length 3, got %d", len(path))
	}

	if path[0] != "aaaa111111111111" || path[1] != "bbbb222222222222" || path[2] != "cccc333333333333" {
		t.Errorf("Expected path [A, B, C], got %v", path)
	}
}

func TestComputeRelationalTrust_NoPath(t *testing.T) {
	node := newTestNode()

	// A has no trust relationships
	trust, path, err := node.ComputeRelationalTrust("aaaa111111111111", "bbbb222222222222", 5)

	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	if trust != 0.0 {
		t.Errorf("Expected trust 0.0 for no path, got %f", trust)
	}

	if path != nil && len(path) != 0 {
		t.Errorf("Expected nil or empty path, got %v", path)
	}
}

func TestComputeRelationalTrust_CycleHandling(t *testing.T) {
	node := newTestNode()

	// Create a cycle: A -> B -> C -> A, and B -> D (target)
	node.TrustRegistry["aaaa111111111111"] = map[string]float64{
		"bbbb222222222222": 0.8,
	}
	node.TrustRegistry["bbbb222222222222"] = map[string]float64{
		"cccc333333333333": 0.6,
		"dddd444444444444": 0.9,
	}
	node.TrustRegistry["cccc333333333333"] = map[string]float64{
		"aaaa111111111111": 0.7, // cycle back to A
	}

	// Should find A -> B -> D with trust 0.8 * 0.9 = 0.72
	trust, path, err := node.ComputeRelationalTrust("aaaa111111111111", "dddd444444444444", 5)

	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	expected := 0.8 * 0.9
	if !floatEquals(trust, expected, 0.0001) {
		t.Errorf("Expected trust %f, got %f", expected, trust)
	}

	if len(path) != 3 || path[2] != "dddd444444444444" {
		t.Errorf("Expected path ending in dddd444444444444, got %v", path)
	}
}

func TestComputeRelationalTrust_DepthLimit(t *testing.T) {
	node := newTestNode()

	// Create a chain: A -> B -> C -> D -> E (4 hops)
	node.TrustRegistry["aaaa111111111111"] = map[string]float64{"bbbb222222222222": 0.9}
	node.TrustRegistry["bbbb222222222222"] = map[string]float64{"cccc333333333333": 0.9}
	node.TrustRegistry["cccc333333333333"] = map[string]float64{"dddd444444444444": 0.9}
	node.TrustRegistry["dddd444444444444"] = map[string]float64{"eeee555555555555": 0.9}

	// With maxDepth=2, should not reach E (4 hops away)
	trust, path, _ := node.ComputeRelationalTrust("aaaa111111111111", "eeee555555555555", 2)

	if trust != 0.0 {
		t.Errorf("Expected trust 0.0 with maxDepth=2, got %f", trust)
	}

	if path != nil && len(path) != 0 {
		t.Errorf("Expected no path with maxDepth=2, got %v", path)
	}

	// With maxDepth=4, should reach E
	trust, path, _ = node.ComputeRelationalTrust("aaaa111111111111", "eeee555555555555", 4)

	expected := 0.9 * 0.9 * 0.9 * 0.9
	if !floatEquals(trust, expected, 0.0001) {
		t.Errorf("Expected trust %f with maxDepth=4, got %f", expected, trust)
	}

	if len(path) != 5 {
		t.Errorf("Expected path length 5, got %d", len(path))
	}
}

func TestComputeRelationalTrust_DefaultDepth(t *testing.T) {
	node := newTestNode()

	// Create a chain of 5 hops
	node.TrustRegistry["aaaa111111111111"] = map[string]float64{"bbbb222222222222": 0.9}
	node.TrustRegistry["bbbb222222222222"] = map[string]float64{"cccc333333333333": 0.9}
	node.TrustRegistry["cccc333333333333"] = map[string]float64{"dddd444444444444": 0.9}
	node.TrustRegistry["dddd444444444444"] = map[string]float64{"eeee555555555555": 0.9}
	node.TrustRegistry["eeee555555555555"] = map[string]float64{"ffff666666666666": 0.9}

	// With maxDepth=0 (default 5), should reach F
	trust, path, _ := node.ComputeRelationalTrust("aaaa111111111111", "ffff666666666666", 0)

	if trust == 0.0 {
		t.Errorf("Expected non-zero trust with default depth, got 0")
	}

	if len(path) != 6 {
		t.Errorf("Expected path length 6, got %d", len(path))
	}
}

func TestComputeRelationalTrust_BestPathSelection(t *testing.T) {
	node := newTestNode()

	// Two paths to target:
	// A -> B -> D: 0.5 * 0.5 = 0.25
	// A -> C -> D: 0.9 * 0.9 = 0.81
	node.TrustRegistry["aaaa111111111111"] = map[string]float64{
		"bbbb222222222222": 0.5,
		"cccc333333333333": 0.9,
	}
	node.TrustRegistry["bbbb222222222222"] = map[string]float64{
		"dddd444444444444": 0.5,
	}
	node.TrustRegistry["cccc333333333333"] = map[string]float64{
		"dddd444444444444": 0.9,
	}

	trust, path, _ := node.ComputeRelationalTrust("aaaa111111111111", "dddd444444444444", 5)

	expected := 0.9 * 0.9
	if trust != expected {
		t.Errorf("Expected best trust %f, got %f", expected, trust)
	}

	// Path should go through C for best trust
	if len(path) != 3 || path[1] != "cccc333333333333" {
		t.Errorf("Expected path through cccc333333333333, got %v", path)
	}
}

func TestComputeRelationalTrust_Distrust(t *testing.T) {
	node := newTestNode()

	// A trusts B with 0.8, B "distrusts" C with 0.3 (< 0.5)
	// Transitive trust A->C should be 0.8 * 0.3 = 0.24 (low due to distrust)
	node.TrustRegistry["aaaa111111111111"] = map[string]float64{
		"bbbb222222222222": 0.8,
	}
	node.TrustRegistry["bbbb222222222222"] = map[string]float64{
		"cccc333333333333": 0.3,
	}

	trust, _, _ := node.ComputeRelationalTrust("aaaa111111111111", "cccc333333333333", 5)

	expected := 0.8 * 0.3
	if trust != expected {
		t.Errorf("Expected trust %f with distrust edge, got %f", expected, trust)
	}
}

func TestComputeRelationalTrust_LongerPathBetterTrust(t *testing.T) {
	node := newTestNode()

	// Short path with low trust: A -> D: 0.2
	// Longer path with higher trust: A -> B -> C -> D: 0.9 * 0.9 * 0.9 = 0.729
	node.TrustRegistry["aaaa111111111111"] = map[string]float64{
		"bbbb222222222222": 0.9,
		"dddd444444444444": 0.2,
	}
	node.TrustRegistry["bbbb222222222222"] = map[string]float64{
		"cccc333333333333": 0.9,
	}
	node.TrustRegistry["cccc333333333333"] = map[string]float64{
		"dddd444444444444": 0.9,
	}

	trust, path, _ := node.ComputeRelationalTrust("aaaa111111111111", "dddd444444444444", 5)

	expected := 0.9 * 0.9 * 0.9
	if !floatEquals(trust, expected, 0.0001) {
		t.Errorf("Expected best trust %f (longer path), got %f", expected, trust)
	}

	if len(path) != 4 {
		t.Errorf("Expected longer path of length 4, got %v", path)
	}
}
