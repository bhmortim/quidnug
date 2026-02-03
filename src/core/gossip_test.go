package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestCreateDomainGossip(t *testing.T) {
	node := newTestNode()
	node.SupportedDomains = []string{"example.com", "test.org"}
	node.GossipTTL = 3

	gossip := node.createDomainGossip()

	if gossip == nil {
		t.Fatal("Expected gossip to be created")
	}

	if gossip.NodeID != node.NodeID {
		t.Errorf("Expected nodeId %s, got %s", node.NodeID, gossip.NodeID)
	}

	if len(gossip.Domains) != 2 {
		t.Errorf("Expected 2 domains, got %d", len(gossip.Domains))
	}

	if gossip.TTL != 3 {
		t.Errorf("Expected TTL 3, got %d", gossip.TTL)
	}

	if gossip.HopCount != 0 {
		t.Errorf("Expected HopCount 0, got %d", gossip.HopCount)
	}

	if gossip.MessageID == "" {
		t.Error("Expected MessageID to be set")
	}

	if gossip.Timestamp == 0 {
		t.Error("Expected Timestamp to be set")
	}
}

func TestCreateDomainGossip_NoDomainsReturnsNil(t *testing.T) {
	node := newTestNode()
	node.SupportedDomains = []string{}
	// Clear default trust domain
	node.TrustDomainsMutex.Lock()
	node.TrustDomains = make(map[string]TrustDomain)
	node.TrustDomainsMutex.Unlock()

	gossip := node.createDomainGossip()

	if gossip != nil {
		t.Error("Expected nil gossip when no domains to gossip")
	}
}

func TestCreateDomainGossip_FallsBackToTrustDomains(t *testing.T) {
	node := newTestNode()
	node.SupportedDomains = []string{}

	// Default trust domain should be included
	gossip := node.createDomainGossip()

	if gossip == nil {
		t.Fatal("Expected gossip to be created from trust domains")
	}

	found := false
	for _, domain := range gossip.Domains {
		if domain == "default" {
			found = true
			break
		}
	}
	if !found {
		t.Error("Expected 'default' trust domain in gossip")
	}
}

func TestReceiveDomainGossip_Valid(t *testing.T) {
	node := newTestNode()

	gossip := DomainGossip{
		NodeID:    "othernode12345",
		Domains:   []string{"example.com", "test.org"},
		Timestamp: time.Now().Unix(),
		TTL:       2,
		HopCount:  1,
		MessageID: "test-message-1",
	}

	err := node.ReceiveDomainGossip(gossip)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	// Verify domain registry was updated
	node.DomainRegistryMutex.RLock()
	nodes := node.DomainRegistry["example.com"]
	node.DomainRegistryMutex.RUnlock()

	if len(nodes) != 1 || nodes[0] != "othernode12345" {
		t.Errorf("Expected domain registry to contain othernode12345, got %v", nodes)
	}

	// Verify known nodes was updated
	node.KnownNodesMutex.RLock()
	knownNode, exists := node.KnownNodes["othernode12345"]
	node.KnownNodesMutex.RUnlock()

	if !exists {
		t.Fatal("Expected node to be added to known nodes")
	}

	if len(knownNode.TrustDomains) != 2 {
		t.Errorf("Expected 2 trust domains, got %d", len(knownNode.TrustDomains))
	}
}

func TestReceiveDomainGossip_DuplicateIgnored(t *testing.T) {
	node := newTestNode()

	gossip := DomainGossip{
		NodeID:    "othernode12345",
		Domains:   []string{"example.com"},
		Timestamp: time.Now().Unix(),
		TTL:       2,
		HopCount:  1,
		MessageID: "test-message-dup",
	}

	// First receive
	err := node.ReceiveDomainGossip(gossip)
	if err != nil {
		t.Fatalf("First receive failed: %v", err)
	}

	// Update domains for second receive
	gossip.Domains = []string{"changed.com"}

	// Second receive should be ignored (same messageId)
	err = node.ReceiveDomainGossip(gossip)
	if err != nil {
		t.Fatalf("Second receive failed: %v", err)
	}

	// Verify original domains are still set (not updated)
	node.DomainRegistryMutex.RLock()
	nodes := node.DomainRegistry["example.com"]
	changedNodes := node.DomainRegistry["changed.com"]
	node.DomainRegistryMutex.RUnlock()

	if len(nodes) != 1 {
		t.Error("Original domain should still be registered")
	}
	if len(changedNodes) != 0 {
		t.Error("Changed domain should not be registered (duplicate ignored)")
	}
}

func TestReceiveDomainGossip_SelfIgnored(t *testing.T) {
	node := newTestNode()

	gossip := DomainGossip{
		NodeID:    node.NodeID, // Same as receiving node
		Domains:   []string{"example.com"},
		Timestamp: time.Now().Unix(),
		TTL:       2,
		HopCount:  0,
		MessageID: "test-message-self",
	}

	err := node.ReceiveDomainGossip(gossip)
	if err == nil {
		t.Error("Expected error for self-gossip")
	}
}

func TestReceiveDomainGossip_EmptyNodeIdRejected(t *testing.T) {
	node := newTestNode()

	gossip := DomainGossip{
		NodeID:    "",
		Domains:   []string{"example.com"},
		Timestamp: time.Now().Unix(),
		TTL:       2,
		HopCount:  0,
		MessageID: "test-message-empty",
	}

	err := node.ReceiveDomainGossip(gossip)
	if err == nil {
		t.Error("Expected error for empty nodeId")
	}
}

func TestReceiveDomainGossip_NegativeTTLRejected(t *testing.T) {
	node := newTestNode()

	gossip := DomainGossip{
		NodeID:    "othernode12345",
		Domains:   []string{"example.com"},
		Timestamp: time.Now().Unix(),
		TTL:       -1,
		HopCount:  0,
		MessageID: "test-message-neg-ttl",
	}

	err := node.ReceiveDomainGossip(gossip)
	if err == nil {
		t.Error("Expected error for negative TTL")
	}
}

func TestReceiveDomainGossipHandler(t *testing.T) {
	node := newTestNode()
	router := setupTestRouter(node)

	gossip := DomainGossip{
		NodeID:    "othernode12345",
		Domains:   []string{"example.com", "test.org"},
		Timestamp: time.Now().Unix(),
		TTL:       2,
		HopCount:  1,
		MessageID: "test-message-handler",
	}

	gossipJSON, _ := json.Marshal(gossip)

	req, err := http.NewRequest("POST", "/api/v1/gossip/domains", strings.NewReader(string(gossipJSON)))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")

	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d: %s", rr.Code, rr.Body.String())
	}

	var response map[string]interface{}
	if err := json.Unmarshal(rr.Body.Bytes(), &response); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	if !response["success"].(bool) {
		t.Error("Expected success to be true")
	}
}

func TestReceiveDomainGossipHandler_InvalidGossip(t *testing.T) {
	node := newTestNode()
	router := setupTestRouter(node)

	gossip := DomainGossip{
		NodeID:    "", // Invalid
		Domains:   []string{"example.com"},
		Timestamp: time.Now().Unix(),
		TTL:       2,
		HopCount:  0,
		MessageID: "test-invalid",
	}

	gossipJSON, _ := json.Marshal(gossip)

	req, err := http.NewRequest("POST", "/api/v1/gossip/domains", strings.NewReader(string(gossipJSON)))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")

	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", rr.Code)
	}
}

func TestGossipForwarding_TTLDecremented(t *testing.T) {
	var receivedGossip DomainGossip
	var receivedCount int32

	// Create test server to receive forwarded gossip
	testServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/v1/gossip/domains" {
			atomic.AddInt32(&receivedCount, 1)
			json.NewDecoder(r.Body).Decode(&receivedGossip)
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]interface{}{
				"success": true,
				"data":    map[string]interface{}{"status": "accepted"},
			})
		}
	}))
	defer testServer.Close()

	node := newTestNode()
	address := strings.TrimPrefix(testServer.URL, "http://")

	// Add a known node to forward to
	node.KnownNodesMutex.Lock()
	node.KnownNodes["forwardTarget"] = Node{
		ID:      "forwardTarget",
		Address: address,
	}
	node.KnownNodesMutex.Unlock()

	// Receive gossip with TTL > 0
	gossip := DomainGossip{
		NodeID:    "originalSender",
		Domains:   []string{"example.com"},
		Timestamp: time.Now().Unix(),
		TTL:       2,
		HopCount:  1,
		MessageID: "test-forward",
	}

	err := node.ReceiveDomainGossip(gossip)
	if err != nil {
		t.Fatalf("Failed to receive gossip: %v", err)
	}

	// Wait for forwarding goroutine
	time.Sleep(100 * time.Millisecond)

	if atomic.LoadInt32(&receivedCount) != 1 {
		t.Error("Expected gossip to be forwarded")
	}

	if receivedGossip.TTL != 1 {
		t.Errorf("Expected TTL to be decremented to 1, got %d", receivedGossip.TTL)
	}

	if receivedGossip.HopCount != 2 {
		t.Errorf("Expected HopCount to be incremented to 2, got %d", receivedGossip.HopCount)
	}

	if receivedGossip.MessageID != gossip.MessageID {
		t.Error("MessageID should be preserved during forwarding")
	}
}

func TestGossipForwarding_ZeroTTLNotForwarded(t *testing.T) {
	var receivedCount int32

	testServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/v1/gossip/domains" {
			atomic.AddInt32(&receivedCount, 1)
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]interface{}{
				"success": true,
				"data":    map[string]interface{}{"status": "accepted"},
			})
		}
	}))
	defer testServer.Close()

	node := newTestNode()
	address := strings.TrimPrefix(testServer.URL, "http://")

	node.KnownNodesMutex.Lock()
	node.KnownNodes["forwardTarget"] = Node{
		ID:      "forwardTarget",
		Address: address,
	}
	node.KnownNodesMutex.Unlock()

	// Receive gossip with TTL = 0
	gossip := DomainGossip{
		NodeID:    "originalSender",
		Domains:   []string{"example.com"},
		Timestamp: time.Now().Unix(),
		TTL:       0, // Should not forward
		HopCount:  3,
		MessageID: "test-no-forward",
	}

	err := node.ReceiveDomainGossip(gossip)
	if err != nil {
		t.Fatalf("Failed to receive gossip: %v", err)
	}

	time.Sleep(100 * time.Millisecond)

	if atomic.LoadInt32(&receivedCount) != 0 {
		t.Error("Gossip with TTL=0 should not be forwarded")
	}
}

func TestGossipForwarding_NotForwardedBackToOriginator(t *testing.T) {
	var receivedCount int32
	var receivedFrom string

	testServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/v1/gossip/domains" {
			atomic.AddInt32(&receivedCount, 1)
			var g DomainGossip
			json.NewDecoder(r.Body).Decode(&g)
			receivedFrom = g.NodeID
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]interface{}{
				"success": true,
				"data":    map[string]interface{}{"status": "accepted"},
			})
		}
	}))
	defer testServer.Close()

	node := newTestNode()
	address := strings.TrimPrefix(testServer.URL, "http://")

	// Add original sender as known node
	node.KnownNodesMutex.Lock()
	node.KnownNodes["originalSender"] = Node{
		ID:      "originalSender",
		Address: address,
	}
	node.KnownNodesMutex.Unlock()

	gossip := DomainGossip{
		NodeID:    "originalSender",
		Domains:   []string{"example.com"},
		Timestamp: time.Now().Unix(),
		TTL:       2,
		HopCount:  0,
		MessageID: "test-no-echo",
	}

	err := node.ReceiveDomainGossip(gossip)
	if err != nil {
		t.Fatalf("Failed to receive gossip: %v", err)
	}

	time.Sleep(100 * time.Millisecond)

	// Should not forward back to original sender
	if atomic.LoadInt32(&receivedCount) != 0 {
		t.Errorf("Gossip should not be forwarded back to originator, received from: %s", receivedFrom)
	}
}

func TestBroadcastDomainInfo(t *testing.T) {
	var receivedCount int32

	testServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/v1/gossip/domains" {
			atomic.AddInt32(&receivedCount, 1)
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]interface{}{
				"success": true,
				"data":    map[string]interface{}{"status": "accepted"},
			})
		}
	}))
	defer testServer.Close()

	node := newTestNode()
	node.SupportedDomains = []string{"example.com"}
	node.GossipTTL = 3
	address := strings.TrimPrefix(testServer.URL, "http://")

	// Add multiple known nodes
	node.KnownNodesMutex.Lock()
	node.KnownNodes["node1"] = Node{ID: "node1", Address: address}
	node.KnownNodes["node2"] = Node{ID: "node2", Address: address}
	node.KnownNodesMutex.Unlock()

	node.BroadcastDomainInfo()

	time.Sleep(200 * time.Millisecond)

	if atomic.LoadInt32(&receivedCount) != 2 {
		t.Errorf("Expected gossip to be sent to 2 nodes, got %d", receivedCount)
	}
}

func TestBroadcastDomainInfo_NoSelfSend(t *testing.T) {
	var receivedCount int32

	testServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/v1/gossip/domains" {
			atomic.AddInt32(&receivedCount, 1)
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]interface{}{
				"success": true,
				"data":    map[string]interface{}{"status": "accepted"},
			})
		}
	}))
	defer testServer.Close()

	node := newTestNode()
	node.SupportedDomains = []string{"example.com"}
	address := strings.TrimPrefix(testServer.URL, "http://")

	// Add self and one other node
	node.KnownNodesMutex.Lock()
	node.KnownNodes[node.NodeID] = Node{ID: node.NodeID, Address: address}
	node.KnownNodes["other"] = Node{ID: "other", Address: address}
	node.KnownNodesMutex.Unlock()

	node.BroadcastDomainInfo()

	time.Sleep(200 * time.Millisecond)

	// Should only send to other node, not self
	if atomic.LoadInt32(&receivedCount) != 1 {
		t.Errorf("Expected gossip to be sent to 1 node (not self), got %d", receivedCount)
	}
}

func TestCleanupGossipSeen(t *testing.T) {
	node := newTestNode()

	// Add old and new entries
	node.GossipSeenMutex.Lock()
	node.GossipSeen["old-message"] = time.Now().Add(-1 * time.Hour).Unix()
	node.GossipSeen["new-message"] = time.Now().Unix()
	node.GossipSeenMutex.Unlock()

	node.cleanupGossipSeen()

	node.GossipSeenMutex.RLock()
	_, oldExists := node.GossipSeen["old-message"]
	_, newExists := node.GossipSeen["new-message"]
	node.GossipSeenMutex.RUnlock()

	if oldExists {
		t.Error("Old message should have been cleaned up")
	}

	if !newExists {
		t.Error("New message should not have been cleaned up")
	}
}

func TestUpdateNodeDomainsHandler_TriggersGossip(t *testing.T) {
	var gossipReceived int32

	testServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/v1/gossip/domains" {
			atomic.AddInt32(&gossipReceived, 1)
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]interface{}{
				"success": true,
				"data":    map[string]interface{}{"status": "accepted"},
			})
		}
	}))
	defer testServer.Close()

	node := newTestNode()
	node.AllowDomainRegistration = true
	address := strings.TrimPrefix(testServer.URL, "http://")

	// Add known node to receive gossip
	node.KnownNodesMutex.Lock()
	node.KnownNodes["othernode"] = Node{ID: "othernode", Address: address}
	node.KnownNodesMutex.Unlock()

	router := setupTestRouter(node)

	body := `{"domains": ["new.example.com"]}`
	req, _ := http.NewRequest("POST", "/api/v1/node/domains", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", rr.Code)
	}

	// Wait for gossip goroutine
	time.Sleep(200 * time.Millisecond)

	if atomic.LoadInt32(&gossipReceived) < 1 {
		t.Error("Expected gossip to be triggered when domains are updated")
	}
}

func TestLoadConfigDomainGossipIntervalFromEnv(t *testing.T) {
	clearConfigEnvVars()
	defer clearConfigEnvVars()

	cfg := LoadConfig()

	if cfg.DomainGossipInterval != DefaultDomainGossipInterval {
		t.Errorf("Expected default DomainGossipInterval %v, got %v", DefaultDomainGossipInterval, cfg.DomainGossipInterval)
	}

	if cfg.DomainGossipTTL != DefaultDomainGossipTTL {
		t.Errorf("Expected default DomainGossipTTL %d, got %d", DefaultDomainGossipTTL, cfg.DomainGossipTTL)
	}
}

func TestLoadConfigDomainGossipFromEnvOverride(t *testing.T) {
	clearConfigEnvVars()
	defer clearConfigEnvVars()

	t.Setenv("DOMAIN_GOSSIP_INTERVAL", "5m")
	t.Setenv("DOMAIN_GOSSIP_TTL", "5")

	cfg := LoadConfig()

	if cfg.DomainGossipInterval != 5*time.Minute {
		t.Errorf("Expected DomainGossipInterval 5m, got %v", cfg.DomainGossipInterval)
	}

	if cfg.DomainGossipTTL != 5 {
		t.Errorf("Expected DomainGossipTTL 5, got %d", cfg.DomainGossipTTL)
	}
}

func TestGossipIntegration_MultiNode(t *testing.T) {
	// Create three nodes that will gossip with each other
	node1 := newTestNode()
	node2 := newTestNode()
	node3 := newTestNode()

	node1.SupportedDomains = []string{"domain1.example.com"}
	node2.SupportedDomains = []string{"domain2.example.com"}
	node3.SupportedDomains = []string{"domain3.example.com"}

	node1.GossipTTL = 2
	node2.GossipTTL = 2
	node3.GossipTTL = 2

	// Start test servers for each node
	router1 := setupTestRouter(node1)
	router2 := setupTestRouter(node2)
	router3 := setupTestRouter(node3)

	server1 := httptest.NewServer(router1)
	server2 := httptest.NewServer(router2)
	server3 := httptest.NewServer(router3)
	defer server1.Close()
	defer server2.Close()
	defer server3.Close()

	addr1 := strings.TrimPrefix(server1.URL, "http://")
	addr2 := strings.TrimPrefix(server2.URL, "http://")
	addr3 := strings.TrimPrefix(server3.URL, "http://")

	// Set up network topology: node1 -> node2 -> node3
	node1.KnownNodesMutex.Lock()
	node1.KnownNodes[node2.NodeID] = Node{ID: node2.NodeID, Address: addr2}
	node1.KnownNodesMutex.Unlock()

	node2.KnownNodesMutex.Lock()
	node2.KnownNodes[node1.NodeID] = Node{ID: node1.NodeID, Address: addr1}
	node2.KnownNodes[node3.NodeID] = Node{ID: node3.NodeID, Address: addr3}
	node2.KnownNodesMutex.Unlock()

	node3.KnownNodesMutex.Lock()
	node3.KnownNodes[node2.NodeID] = Node{ID: node2.NodeID, Address: addr2}
	node3.KnownNodesMutex.Unlock()

	// Node1 broadcasts its domain info
	node1.BroadcastDomainInfo()

	// Wait for gossip to propagate
	time.Sleep(500 * time.Millisecond)

	// Verify node2 learned about node1's domain
	node2.DomainRegistryMutex.RLock()
	node2HasDomain1 := len(node2.DomainRegistry["domain1.example.com"]) > 0
	node2.DomainRegistryMutex.RUnlock()

	if !node2HasDomain1 {
		t.Error("Node2 should have learned about node1's domain via direct gossip")
	}

	// Verify node3 learned about node1's domain via forwarding
	node3.DomainRegistryMutex.RLock()
	node3HasDomain1 := len(node3.DomainRegistry["domain1.example.com"]) > 0
	node3.DomainRegistryMutex.RUnlock()

	if !node3HasDomain1 {
		t.Error("Node3 should have learned about node1's domain via gossip forwarding")
	}
}

func TestRunDomainGossip_ContextCancellation(t *testing.T) {
	node := newTestNode()
	node.SupportedDomains = []string{"example.com"}

	ctx, cancel := context.WithCancel(context.Background())

	done := make(chan struct{})
	go func() {
		node.runDomainGossip(ctx, 1*time.Hour) // Long interval, should not trigger
		close(done)
	}()

	// Cancel immediately
	cancel()

	// Should exit quickly
	select {
	case <-done:
		// Success
	case <-time.After(1 * time.Second):
		t.Error("runDomainGossip did not exit on context cancellation")
	}
}
