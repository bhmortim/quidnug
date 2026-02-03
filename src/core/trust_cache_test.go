package main

import (
	"testing"
	"time"
)

func TestTrustCache_BasicGetSet(t *testing.T) {
	cache := NewTrustCache(60 * time.Second)

	// Initially empty
	_, _, found := cache.Get("a:b:5")
	if found {
		t.Error("Expected cache miss for empty cache")
	}

	// Set and get
	cache.Set("a:b:5", 0.8, []string{"a", "c", "b"})

	trustLevel, trustPath, found := cache.Get("a:b:5")
	if !found {
		t.Fatal("Expected cache hit after set")
	}
	if trustLevel != 0.8 {
		t.Errorf("Expected trust level 0.8, got %f", trustLevel)
	}
	if len(trustPath) != 3 || trustPath[0] != "a" || trustPath[1] != "c" || trustPath[2] != "b" {
		t.Errorf("Unexpected trust path: %v", trustPath)
	}
}

func TestTrustCache_EnhancedGetSet(t *testing.T) {
	cache := NewTrustCache(60 * time.Second)

	// Initially empty
	_, found := cache.GetEnhanced("a:b:5:true")
	if found {
		t.Error("Expected cache miss for empty cache")
	}

	// Set and get
	result := EnhancedTrustResult{
		RelationalTrustResult: RelationalTrustResult{
			Observer:   "a",
			Target:     "b",
			TrustLevel: 0.75,
			TrustPath:  []string{"a", "x", "b"},
			PathDepth:  2,
		},
		Confidence:     "high",
		UnverifiedHops: 0,
		VerificationGaps: []VerificationGap{},
	}
	cache.SetEnhanced("a:b:5:true", result)

	retrieved, found := cache.GetEnhanced("a:b:5:true")
	if !found {
		t.Fatal("Expected cache hit after set")
	}
	if retrieved.TrustLevel != 0.75 {
		t.Errorf("Expected trust level 0.75, got %f", retrieved.TrustLevel)
	}
	if retrieved.Confidence != "high" {
		t.Errorf("Expected confidence 'high', got %s", retrieved.Confidence)
	}
}

func TestTrustCache_Expiration(t *testing.T) {
	cache := NewTrustCache(50 * time.Millisecond)

	cache.Set("a:b:5", 0.8, []string{"a", "b"})

	// Should hit immediately
	_, _, found := cache.Get("a:b:5")
	if !found {
		t.Error("Expected cache hit before expiration")
	}

	// Wait for expiration
	time.Sleep(60 * time.Millisecond)

	// Should miss after expiration
	_, _, found = cache.Get("a:b:5")
	if found {
		t.Error("Expected cache miss after expiration")
	}
}

func TestTrustCache_EnhancedExpiration(t *testing.T) {
	cache := NewTrustCache(50 * time.Millisecond)

	result := EnhancedTrustResult{
		RelationalTrustResult: RelationalTrustResult{
			TrustLevel: 0.5,
		},
	}
	cache.SetEnhanced("a:b:5:false", result)

	// Should hit immediately
	_, found := cache.GetEnhanced("a:b:5:false")
	if !found {
		t.Error("Expected cache hit before expiration")
	}

	// Wait for expiration
	time.Sleep(60 * time.Millisecond)

	// Should miss after expiration
	_, found = cache.GetEnhanced("a:b:5:false")
	if found {
		t.Error("Expected cache miss after expiration")
	}
}

func TestTrustCache_Invalidate(t *testing.T) {
	cache := NewTrustCache(60 * time.Second)

	cache.Set("a:b:5", 0.8, []string{"a", "b"})
	cache.SetEnhanced("x:y:3:true", EnhancedTrustResult{})

	basic, enhanced := cache.Size()
	if basic != 1 || enhanced != 1 {
		t.Errorf("Expected size (1, 1), got (%d, %d)", basic, enhanced)
	}

	cache.Invalidate()

	basic, enhanced = cache.Size()
	if basic != 0 || enhanced != 0 {
		t.Errorf("Expected size (0, 0) after invalidate, got (%d, %d)", basic, enhanced)
	}

	_, _, found := cache.Get("a:b:5")
	if found {
		t.Error("Expected cache miss after invalidate")
	}

	_, found = cache.GetEnhanced("x:y:3:true")
	if found {
		t.Error("Expected enhanced cache miss after invalidate")
	}
}

func TestTrustCache_PathCopyOnSet(t *testing.T) {
	cache := NewTrustCache(60 * time.Second)

	originalPath := []string{"a", "b", "c"}
	cache.Set("key", 0.5, originalPath)

	// Modify the original slice
	originalPath[1] = "modified"

	// Cached value should be unchanged
	_, retrievedPath, found := cache.Get("key")
	if !found {
		t.Fatal("Expected cache hit")
	}
	if retrievedPath[1] != "b" {
		t.Error("Cache should store a copy, not the original slice")
	}
}

func TestTrustCache_PathCopyOnGet(t *testing.T) {
	cache := NewTrustCache(60 * time.Second)

	cache.Set("key", 0.5, []string{"a", "b", "c"})

	// Get and modify
	_, retrievedPath, _ := cache.Get("key")
	retrievedPath[1] = "modified"

	// Get again - should be unchanged
	_, retrievedPath2, _ := cache.Get("key")
	if retrievedPath2[1] != "b" {
		t.Error("Cache should return a copy, not the stored slice")
	}
}

func TestComputeRelationalTrust_CacheHit(t *testing.T) {
	node := newTestNode()

	// Set up a simple trust graph: node -> A -> B
	aID := "aaaaaaaaaaaaaaaa"
	bID := "bbbbbbbbbbbbbbbb"

	node.TrustRegistry[node.NodeID] = map[string]float64{aID: 0.9}
	node.TrustRegistry[aID] = map[string]float64{bID: 0.8}

	// First call - should compute
	trust1, path1, err := node.ComputeRelationalTrust(node.NodeID, bID, 5)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if trust1 == 0 {
		t.Fatal("Expected non-zero trust")
	}

	// Verify cache was populated
	basic, _ := node.TrustCache.Size()
	if basic == 0 {
		t.Error("Expected cache to be populated after first call")
	}

	// Second call - should hit cache
	trust2, path2, err := node.ComputeRelationalTrust(node.NodeID, bID, 5)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if trust1 != trust2 {
		t.Errorf("Trust levels should match: %f vs %f", trust1, trust2)
	}
	if len(path1) != len(path2) {
		t.Errorf("Path lengths should match: %d vs %d", len(path1), len(path2))
	}
}

func TestComputeRelationalTrust_CacheInvalidatedOnTrustUpdate(t *testing.T) {
	node := newTestNode()

	// Set up initial trust
	aID := "aaaaaaaaaaaaaaaa"
	node.TrustRegistry[node.NodeID] = map[string]float64{aID: 0.5}

	// Compute and cache
	trust1, _, _ := node.ComputeRelationalTrust(node.NodeID, aID, 5)

	// Verify cache is populated
	basic, _ := node.TrustCache.Size()
	if basic == 0 {
		t.Error("Expected cache to be populated")
	}

	// Update trust registry (simulates processing a trust transaction)
	node.updateTrustRegistry(TrustTransaction{
		Truster:    node.NodeID,
		Trustee:    aID,
		TrustLevel: 0.9,
		Nonce:      1,
	})

	// Cache should be invalidated
	basic, _ = node.TrustCache.Size()
	if basic != 0 {
		t.Error("Expected cache to be invalidated after trust update")
	}

	// Recompute - should get new value
	trust2, _, _ := node.ComputeRelationalTrust(node.NodeID, aID, 5)
	if trust2 != 0.9 {
		t.Errorf("Expected trust 0.9 after update, got %f", trust2)
	}
	if trust1 == trust2 {
		t.Error("Trust should change after registry update")
	}
}

func TestComputeRelationalTrustEnhanced_CacheHit(t *testing.T) {
	node := newTestNode()

	// Set up trust
	aID := "aaaaaaaaaaaaaaaa"
	node.TrustRegistry[node.NodeID] = map[string]float64{aID: 0.8}

	// Add verified edge
	node.VerifiedTrustEdges[node.NodeID] = map[string]TrustEdge{
		aID: {
			Truster:    node.NodeID,
			Trustee:    aID,
			TrustLevel: 0.8,
			Verified:   true,
		},
	}

	// First call
	result1, err := node.ComputeRelationalTrustEnhanced(node.NodeID, aID, 5, false)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	// Verify cache was populated
	_, enhanced := node.TrustCache.Size()
	if enhanced == 0 {
		t.Error("Expected enhanced cache to be populated after first call")
	}

	// Second call - should hit cache
	result2, err := node.ComputeRelationalTrustEnhanced(node.NodeID, aID, 5, false)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if result1.TrustLevel != result2.TrustLevel {
		t.Errorf("Trust levels should match: %f vs %f", result1.TrustLevel, result2.TrustLevel)
	}
}

func TestComputeRelationalTrust_DifferentMaxDepthDifferentCacheKeys(t *testing.T) {
	node := newTestNode()

	aID := "aaaaaaaaaaaaaaaa"
	node.TrustRegistry[node.NodeID] = map[string]float64{aID: 0.8}

	// Call with depth 3
	node.ComputeRelationalTrust(node.NodeID, aID, 3)

	// Call with depth 5
	node.ComputeRelationalTrust(node.NodeID, aID, 5)

	// Should have 2 cache entries
	basic, _ := node.TrustCache.Size()
	if basic != 2 {
		t.Errorf("Expected 2 cache entries for different depths, got %d", basic)
	}
}

func TestComputeRelationalTrustEnhanced_DifferentIncludeUnverifiedDifferentCacheKeys(t *testing.T) {
	node := newTestNode()

	aID := "aaaaaaaaaaaaaaaa"
	node.TrustRegistry[node.NodeID] = map[string]float64{aID: 0.8}
	node.VerifiedTrustEdges[node.NodeID] = map[string]TrustEdge{
		aID: {Truster: node.NodeID, Trustee: aID, TrustLevel: 0.8, Verified: true},
	}

	// Call with includeUnverified=false
	node.ComputeRelationalTrustEnhanced(node.NodeID, aID, 5, false)

	// Call with includeUnverified=true
	node.ComputeRelationalTrustEnhanced(node.NodeID, aID, 5, true)

	// Should have 2 cache entries
	_, enhanced := node.TrustCache.Size()
	if enhanced != 2 {
		t.Errorf("Expected 2 enhanced cache entries, got %d", enhanced)
	}
}

func TestComputeRelationalTrust_SelfTrustNotCached(t *testing.T) {
	node := newTestNode()

	// Self-trust should return immediately without caching
	trust, path, err := node.ComputeRelationalTrust(node.NodeID, node.NodeID, 5)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if trust != 1.0 {
		t.Errorf("Expected self-trust 1.0, got %f", trust)
	}
	if len(path) != 1 || path[0] != node.NodeID {
		t.Errorf("Expected path [%s], got %v", node.NodeID, path)
	}

	// Cache should still be empty (self-trust is trivial, no need to cache)
	basic, _ := node.TrustCache.Size()
	if basic != 0 {
		t.Error("Self-trust should not be cached")
	}
}

func TestMakeTrustCacheKey(t *testing.T) {
	key := makeTrustCacheKey("observer1", "target1", 5)
	expected := "observer1:target1:5"
	if key != expected {
		t.Errorf("Expected key %s, got %s", expected, key)
	}
}

func TestMakeEnhancedTrustCacheKey(t *testing.T) {
	key1 := makeEnhancedTrustCacheKey("o", "t", 3, true)
	key2 := makeEnhancedTrustCacheKey("o", "t", 3, false)

	if key1 == key2 {
		t.Error("Keys should differ based on includeUnverified")
	}

	expected1 := "o:t:3:true"
	expected2 := "o:t:3:false"
	if key1 != expected1 {
		t.Errorf("Expected %s, got %s", expected1, key1)
	}
	if key2 != expected2 {
		t.Errorf("Expected %s, got %s", expected2, key2)
	}
}

func BenchmarkComputeRelationalTrust_WithCache(b *testing.B) {
	node := newTestNode()

	// Build a trust chain: node -> A -> B -> C -> D -> target
	ids := []string{
		"aaaaaaaaaaaaaaaa",
		"bbbbbbbbbbbbbbbb",
		"cccccccccccccccc",
		"dddddddddddddddd",
		"eeeeeeeeeeeeeeee",
	}

	node.TrustRegistry[node.NodeID] = map[string]float64{ids[0]: 0.9}
	for i := 0; i < len(ids)-1; i++ {
		node.TrustRegistry[ids[i]] = map[string]float64{ids[i+1]: 0.9}
	}

	target := ids[len(ids)-1]

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		node.ComputeRelationalTrust(node.NodeID, target, 5)
	}
}

func BenchmarkComputeRelationalTrust_WithoutCache(b *testing.B) {
	node := newTestNode()
	node.TrustCache = nil // Disable cache

	// Build a trust chain: node -> A -> B -> C -> D -> target
	ids := []string{
		"aaaaaaaaaaaaaaaa",
		"bbbbbbbbbbbbbbbb",
		"cccccccccccccccc",
		"dddddddddddddddd",
		"eeeeeeeeeeeeeeee",
	}

	node.TrustRegistry[node.NodeID] = map[string]float64{ids[0]: 0.9}
	for i := 0; i < len(ids)-1; i++ {
		node.TrustRegistry[ids[i]] = map[string]float64{ids[i+1]: 0.9}
	}

	target := ids[len(ids)-1]

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		node.ComputeRelationalTrust(node.NodeID, target, 5)
	}
}

func BenchmarkComputeRelationalTrust_RepeatedQueries(b *testing.B) {
	node := newTestNode()

	// Create a moderately complex graph
	numNodes := 20
	ids := make([]string, numNodes)
	for i := 0; i < numNodes; i++ {
		ids[i] = "abcdef1234567890"[:16]
		ids[i] = ids[i][:14] + string('a'+byte(i/10)) + string('0'+byte(i%10))
	}

	// Create interconnected trust relationships
	for i := 0; i < numNodes; i++ {
		node.TrustRegistry[ids[i]] = make(map[string]float64)
		for j := 0; j < 3 && i+j+1 < numNodes; j++ {
			node.TrustRegistry[ids[i]][ids[i+j+1]] = 0.8
		}
	}

	observer := ids[0]
	target := ids[numNodes-1]

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		node.ComputeRelationalTrust(observer, target, 5)
	}
}

func TestLoadConfigTrustCacheTTLFromEnv(t *testing.T) {
	clearConfigEnvVars()
	defer clearConfigEnvVars()

	os.Setenv("TRUST_CACHE_TTL", "120s")

	cfg := LoadConfig()

	if cfg.TrustCacheTTL != 120*time.Second {
		t.Errorf("Expected TrustCacheTTL 120s, got %v", cfg.TrustCacheTTL)
	}
}

func TestLoadConfigTrustCacheTTLDefault(t *testing.T) {
	clearConfigEnvVars()
	defer clearConfigEnvVars()

	cfg := LoadConfig()

	if cfg.TrustCacheTTL != DefaultTrustCacheTTL {
		t.Errorf("Expected default TrustCacheTTL %v, got %v", DefaultTrustCacheTTL, cfg.TrustCacheTTL)
	}
}
