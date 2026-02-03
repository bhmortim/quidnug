package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"
)

func TestMatchDomainPattern_ExactMatch(t *testing.T) {
	tests := []struct {
		domain   string
		pattern  string
		expected bool
	}{
		{"example.com", "example.com", true},
		{"sub.example.com", "sub.example.com", true},
		{"example.com", "other.com", false},
		{"", "example.com", false},
		{"example.com", "", false},
	}

	for _, tt := range tests {
		result := MatchDomainPattern(tt.domain, tt.pattern)
		if result != tt.expected {
			t.Errorf("MatchDomainPattern(%q, %q) = %v, want %v",
				tt.domain, tt.pattern, result, tt.expected)
		}
	}
}

func TestMatchDomainPattern_WildcardSubdomain(t *testing.T) {
	tests := []struct {
		domain   string
		pattern  string
		expected bool
	}{
		{"sub.example.com", "*.example.com", true},
		{"deep.sub.example.com", "*.example.com", true},
		{"a.b.c.example.com", "*.example.com", true},
		{"example.com", "*.example.com", false},
		{"notexample.com", "*.example.com", false},
		{"subexample.com", "*.example.com", false},
		{"sub.other.com", "*.example.com", false},
	}

	for _, tt := range tests {
		result := MatchDomainPattern(tt.domain, tt.pattern)
		if result != tt.expected {
			t.Errorf("MatchDomainPattern(%q, %q) = %v, want %v",
				tt.domain, tt.pattern, result, tt.expected)
		}
	}
}

func TestIsDomainSupported_EmptyList(t *testing.T) {
	node := &QuidnugNode{
		SupportedDomains: []string{},
	}

	if !node.IsDomainSupported("any.domain.com") {
		t.Error("Empty SupportedDomains should allow all domains")
	}

	if !node.IsDomainSupported("default") {
		t.Error("Empty SupportedDomains should allow 'default' domain")
	}
}

func TestIsDomainSupported_ExactMatches(t *testing.T) {
	node := &QuidnugNode{
		SupportedDomains: []string{"example.com", "other.org", "default"},
	}

	tests := []struct {
		domain   string
		expected bool
	}{
		{"example.com", true},
		{"other.org", true},
		{"default", true},
		{"notlisted.com", false},
		{"sub.example.com", false},
	}

	for _, tt := range tests {
		result := node.IsDomainSupported(tt.domain)
		if result != tt.expected {
			t.Errorf("IsDomainSupported(%q) = %v, want %v",
				tt.domain, result, tt.expected)
		}
	}
}

func TestIsDomainSupported_WildcardPatterns(t *testing.T) {
	node := &QuidnugNode{
		SupportedDomains: []string{"*.example.com", "specific.org"},
	}

	tests := []struct {
		domain   string
		expected bool
	}{
		{"sub.example.com", true},
		{"deep.sub.example.com", true},
		{"example.com", false},
		{"specific.org", true},
		{"sub.specific.org", false},
		{"other.com", false},
	}

	for _, tt := range tests {
		result := node.IsDomainSupported(tt.domain)
		if result != tt.expected {
			t.Errorf("IsDomainSupported(%q) = %v, want %v",
				tt.domain, result, tt.expected)
		}
	}
}

func TestIsDomainSupported_MixedPatterns(t *testing.T) {
	node := &QuidnugNode{
		SupportedDomains: []string{"example.com", "*.example.com", "*.test.org"},
	}

	tests := []struct {
		domain   string
		expected bool
	}{
		{"example.com", true},
		{"sub.example.com", true},
		{"deep.sub.example.com", true},
		{"sub.test.org", true},
		{"test.org", false},
		{"other.com", false},
	}

	for _, tt := range tests {
		result := node.IsDomainSupported(tt.domain)
		if result != tt.expected {
			t.Errorf("IsDomainSupported(%q) = %v, want %v",
				tt.domain, result, tt.expected)
		}
	}
}

func TestLoadConfigSupportedDomainsFromEnv(t *testing.T) {
	clearConfigEnvVars()
	defer clearConfigEnvVars()

	os.Setenv("SUPPORTED_DOMAINS", `["example.com", "*.test.org"]`)

	cfg := LoadConfig()

	if len(cfg.SupportedDomains) != 2 {
		t.Fatalf("Expected 2 supported domains, got %d", len(cfg.SupportedDomains))
	}

	if cfg.SupportedDomains[0] != "example.com" {
		t.Errorf("Expected first domain to be 'example.com', got %q", cfg.SupportedDomains[0])
	}

	if cfg.SupportedDomains[1] != "*.test.org" {
		t.Errorf("Expected second domain to be '*.test.org', got %q", cfg.SupportedDomains[1])
	}
}

func TestLoadConfigAllowDomainRegistrationFromEnv(t *testing.T) {
	clearConfigEnvVars()
	defer clearConfigEnvVars()

	cfg := LoadConfig()
	if !cfg.AllowDomainRegistration {
		t.Error("Default AllowDomainRegistration should be true")
	}

	os.Setenv("ALLOW_DOMAIN_REGISTRATION", "false")
	cfg = LoadConfig()
	if cfg.AllowDomainRegistration {
		t.Error("AllowDomainRegistration should be false when env var is 'false'")
	}

	os.Setenv("ALLOW_DOMAIN_REGISTRATION", "true")
	cfg = LoadConfig()
	if !cfg.AllowDomainRegistration {
		t.Error("AllowDomainRegistration should be true when env var is 'true'")
	}
}

func TestRegisterTrustDomain_DomainNotSupported(t *testing.T) {
	node := &QuidnugNode{
		SupportedDomains:        []string{"allowed.com"},
		AllowDomainRegistration: true,
		TrustDomains:            make(map[string]TrustDomain),
		NodeID:                  "testnode123456",
	}

	err := node.RegisterTrustDomain(TrustDomain{Name: "notallowed.com"})
	if err == nil {
		t.Error("Expected error when registering unsupported domain")
	}
}

func TestRegisterTrustDomain_RegistrationNotAllowed(t *testing.T) {
	node := &QuidnugNode{
		SupportedDomains:        []string{},
		AllowDomainRegistration: false,
		TrustDomains:            make(map[string]TrustDomain),
		NodeID:                  "testnode123456",
	}

	err := node.RegisterTrustDomain(TrustDomain{Name: "any.com"})
	if err == nil {
		t.Error("Expected error when domain registration is not allowed")
	}
}

func TestRegisterTrustDomain_SupportedDomain(t *testing.T) {
	node := &QuidnugNode{
		SupportedDomains:        []string{"*.example.com"},
		AllowDomainRegistration: true,
		TrustDomains:            make(map[string]TrustDomain),
		NodeID:                  "testnode123456",
	}
	node.PublicKey = nil

	err := node.RegisterTrustDomain(TrustDomain{Name: "sub.example.com"})
	if err != nil {
		t.Errorf("Unexpected error registering supported domain: %v", err)
	}
}

func TestAddTrustTransaction_UnsupportedDomain(t *testing.T) {
	node := newTestNode()
	node.SupportedDomains = []string{"allowed.com"}

	tx := TrustTransaction{
		BaseTransaction: BaseTransaction{
			TrustDomain: "notallowed.com",
		},
		Truster:    "1234567890abcdef",
		Trustee:    "abcdef1234567890",
		TrustLevel: 0.8,
	}

	_, err := node.AddTrustTransaction(tx)
	if err == nil {
		t.Error("Expected error for unsupported domain")
	}
}

func TestAddIdentityTransaction_UnsupportedDomain(t *testing.T) {
	node := newTestNode()
	node.SupportedDomains = []string{"allowed.com"}

	tx := IdentityTransaction{
		BaseTransaction: BaseTransaction{
			TrustDomain: "notallowed.com",
		},
		QuidID:  "1234567890abcdef",
		Name:    "Test",
		Creator: "abcdef1234567890",
	}

	_, err := node.AddIdentityTransaction(tx)
	if err == nil {
		t.Error("Expected error for unsupported domain")
	}
}

func TestAddTitleTransaction_UnsupportedDomain(t *testing.T) {
	node := newTestNode()
	node.SupportedDomains = []string{"allowed.com"}

	tx := TitleTransaction{
		BaseTransaction: BaseTransaction{
			TrustDomain: "notallowed.com",
		},
		AssetID: "1234567890abcdef",
		Owners: []OwnershipStake{
			{OwnerID: "abcdef1234567890", Percentage: 100.0},
		},
	}

	_, err := node.AddTitleTransaction(tx)
	if err == nil {
		t.Error("Expected error for unsupported domain")
	}
}

func TestAddEventTransaction_UnsupportedDomain(t *testing.T) {
	node := newTestNode()
	node.SupportedDomains = []string{"allowed.com"}

	tx := EventTransaction{
		BaseTransaction: BaseTransaction{
			TrustDomain: "notallowed.com",
		},
		SubjectID:   "1234567890abcdef",
		SubjectType: "QUID",
		EventType:   "test",
		Payload:     map[string]interface{}{"key": "value"},
	}

	_, err := node.AddEventTransaction(tx)
	if err == nil {
		t.Error("Expected error for unsupported domain")
	}
}

func TestGenerateBlock_UnsupportedDomain(t *testing.T) {
	node := newTestNode()
	node.SupportedDomains = []string{"allowed.com"}

	_, err := node.GenerateBlock("notallowed.com")
	if err == nil {
		t.Error("Expected error for unsupported domain")
	}
}

func TestReceiveBlock_UnsupportedDomain(t *testing.T) {
	node := newTestNode()
	node.SupportedDomains = []string{"allowed.com"}

	block := Block{
		Index:     1,
		Timestamp: 1234567890,
		TrustProof: TrustProof{
			TrustDomain: "notallowed.com",
			ValidatorID: node.NodeID,
		},
		PrevHash: node.Blockchain[0].Hash,
	}
	block.Hash = calculateBlockHash(block)

	acceptance, err := node.ReceiveBlock(block)
	if err == nil {
		t.Error("Expected error for unsupported domain")
	}
	if acceptance != BlockUntrusted {
		t.Errorf("Expected BlockUntrusted, got %v", acceptance)
	}
}

func TestAddTransaction_DefaultDomainSupported(t *testing.T) {
	node := newTestNode()
	node.SupportedDomains = []string{"default"}

	tx := signTrustTx(node, TrustTransaction{
		BaseTransaction: BaseTransaction{
			TrustDomain: "",
		},
		Truster:    node.NodeID,
		Trustee:    "abcdef1234567890",
		TrustLevel: 0.8,
	})

	_, err := node.AddTrustTransaction(tx)
	if err != nil {
		t.Errorf("Expected default domain to be supported when listed, got error: %v", err)
	}
}

func TestAddTransaction_EmptyDomainList_AllAllowed(t *testing.T) {
	node := newTestNode()
	node.SupportedDomains = []string{}

	tx := signTrustTx(node, TrustTransaction{
		BaseTransaction: BaseTransaction{
			TrustDomain: "any.domain.com",
		},
		Truster:    node.NodeID,
		Trustee:    "abcdef1234567890",
		TrustLevel: 0.8,
	})

	_, err := node.AddTrustTransaction(tx)
	if err != nil {
		t.Errorf("Expected all domains allowed with empty list, got error: %v", err)
	}
}

func TestGetNodeDomainsHandler(t *testing.T) {
	node := newTestNode()
	node.SupportedDomains = []string{"example.com", "*.test.org"}

	router := setupTestRouter(node)

	req, err := http.NewRequest("GET", "/api/v1/node/domains", nil)
	if err != nil {
		t.Fatal(err)
	}

	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", rr.Code)
	}

	var response map[string]interface{}
	if err := json.Unmarshal(rr.Body.Bytes(), &response); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	if !response["success"].(bool) {
		t.Error("Expected success to be true")
	}

	data := response["data"].(map[string]interface{})
	if data["nodeId"] != node.NodeID {
		t.Errorf("Expected nodeId %s, got %s", node.NodeID, data["nodeId"])
	}

	domains := data["domains"].([]interface{})
	if len(domains) != 2 {
		t.Errorf("Expected 2 domains, got %d", len(domains))
	}
}

func TestGetNodeDomainsHandler_EmptyDomains(t *testing.T) {
	node := newTestNode()
	node.SupportedDomains = []string{}

	router := setupTestRouter(node)

	req, err := http.NewRequest("GET", "/api/v1/node/domains", nil)
	if err != nil {
		t.Fatal(err)
	}

	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", rr.Code)
	}

	var response map[string]interface{}
	if err := json.Unmarshal(rr.Body.Bytes(), &response); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	data := response["data"].(map[string]interface{})
	domains := data["domains"]
	if domains != nil {
		domainsSlice, ok := domains.([]interface{})
		if ok && len(domainsSlice) != 0 {
			t.Errorf("Expected empty domains, got %v", domains)
		}
	}
}

func TestUpdateNodeDomainsHandler_Success(t *testing.T) {
	node := newTestNode()
	node.AllowDomainRegistration = true
	node.SupportedDomains = []string{}

	router := setupTestRouter(node)

	body := `{"domains": ["new.example.com", "*.updated.org"]}`
	req, err := http.NewRequest("POST", "/api/v1/node/domains", strings.NewReader(body))
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

	// Verify domains were updated
	if len(node.SupportedDomains) != 2 {
		t.Errorf("Expected 2 domains, got %d", len(node.SupportedDomains))
	}
	if node.SupportedDomains[0] != "new.example.com" {
		t.Errorf("Expected first domain 'new.example.com', got %s", node.SupportedDomains[0])
	}
}

func TestUpdateNodeDomainsHandler_NotAllowed(t *testing.T) {
	node := newTestNode()
	node.AllowDomainRegistration = false
	node.SupportedDomains = []string{"original.com"}

	router := setupTestRouter(node)

	body := `{"domains": ["new.example.com"]}`
	req, err := http.NewRequest("POST", "/api/v1/node/domains", strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")

	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusForbidden {
		t.Errorf("Expected status 403, got %d", rr.Code)
	}

	// Verify domains were not changed
	if len(node.SupportedDomains) != 1 || node.SupportedDomains[0] != "original.com" {
		t.Error("Domains should not have been modified")
	}
}

func TestUpdateNodeDomainsHandler_EmptyDomain(t *testing.T) {
	node := newTestNode()
	node.AllowDomainRegistration = true

	router := setupTestRouter(node)

	body := `{"domains": ["valid.com", ""]}`
	req, err := http.NewRequest("POST", "/api/v1/node/domains", strings.NewReader(body))
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

func TestFetchNodeDomains_Success(t *testing.T) {
	// Create a test server that returns domain info
	testServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/v1/node/domains" {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]interface{}{
				"success": true,
				"data": map[string]interface{}{
					"nodeId":  "testnode123456",
					"domains": []string{"example.com", "test.org"},
				},
			})
		}
	}))
	defer testServer.Close()

	node := newTestNode()
	ctx := context.Background()

	// Extract host:port from test server URL
	address := strings.TrimPrefix(testServer.URL, "http://")

	domains, err := node.fetchNodeDomains(ctx, address)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if len(domains) != 2 {
		t.Errorf("Expected 2 domains, got %d", len(domains))
	}
	if domains[0] != "example.com" {
		t.Errorf("Expected first domain 'example.com', got %s", domains[0])
	}
}

func TestFetchNodeDomains_ServerError(t *testing.T) {
	testServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer testServer.Close()

	node := newTestNode()
	ctx := context.Background()

	address := strings.TrimPrefix(testServer.URL, "http://")

	_, err := node.fetchNodeDomains(ctx, address)
	if err == nil {
		t.Error("Expected error for server error response")
	}
}

func TestFetchNodeDomains_UnsuccessfulResponse(t *testing.T) {
	testServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"error": map[string]interface{}{
				"code":    "ERROR",
				"message": "Something went wrong",
			},
		})
	}))
	defer testServer.Close()

	node := newTestNode()
	ctx := context.Background()

	address := strings.TrimPrefix(testServer.URL, "http://")

	_, err := node.fetchNodeDomains(ctx, address)
	if err == nil {
		t.Error("Expected error for unsuccessful response")
	}
}

func TestFindNodesForSubdomains_NoMatches(t *testing.T) {
	node := newTestNode()
	node.DomainRegistry = map[string][]string{
		"other.com":     {"node1"},
		"different.org": {"node2"},
	}
	node.KnownNodes = map[string]Node{
		"node1": {ID: "node1", Address: "localhost:8001"},
		"node2": {ID: "node2", Address: "localhost:8002"},
	}

	nodes := node.findNodesForSubdomains("example.com")
	if len(nodes) != 0 {
		t.Errorf("Expected 0 nodes, got %d", len(nodes))
	}
}

func TestFindNodesForSubdomains_SingleMatch(t *testing.T) {
	node := newTestNode()
	node.DomainRegistry = map[string][]string{
		"api.example.com": {"node1"},
		"other.com":       {"node2"},
	}
	node.KnownNodes = map[string]Node{
		"node1": {ID: "node1", Address: "localhost:8001"},
		"node2": {ID: "node2", Address: "localhost:8002"},
	}

	nodes := node.findNodesForSubdomains("example.com")
	if len(nodes) != 1 {
		t.Fatalf("Expected 1 node, got %d", len(nodes))
	}
	if nodes[0].ID != "node1" {
		t.Errorf("Expected node1, got %s", nodes[0].ID)
	}
}

func TestFindNodesForSubdomains_MultipleMatches(t *testing.T) {
	node := newTestNode()
	node.DomainRegistry = map[string][]string{
		"api.example.com":      {"node1"},
		"auth.example.com":     {"node2"},
		"deep.sub.example.com": {"node3"},
		"other.com":            {"node4"},
	}
	node.KnownNodes = map[string]Node{
		"node1": {ID: "node1", Address: "localhost:8001"},
		"node2": {ID: "node2", Address: "localhost:8002"},
		"node3": {ID: "node3", Address: "localhost:8003"},
		"node4": {ID: "node4", Address: "localhost:8004"},
	}

	nodes := node.findNodesForSubdomains("example.com")
	if len(nodes) != 3 {
		t.Errorf("Expected 3 nodes, got %d", len(nodes))
	}

	// Verify all expected nodes are present
	nodeIDs := make(map[string]bool)
	for _, n := range nodes {
		nodeIDs[n.ID] = true
	}
	for _, expected := range []string{"node1", "node2", "node3"} {
		if !nodeIDs[expected] {
			t.Errorf("Expected %s in results", expected)
		}
	}
	if nodeIDs["node4"] {
		t.Error("node4 should not be in results (manages other.com)")
	}
}

func TestFindNodesForSubdomains_MultipleNodesPerDomain(t *testing.T) {
	node := newTestNode()
	node.DomainRegistry = map[string][]string{
		"api.example.com": {"node1", "node2"},
	}
	node.KnownNodes = map[string]Node{
		"node1": {ID: "node1", Address: "localhost:8001"},
		"node2": {ID: "node2", Address: "localhost:8002"},
	}

	nodes := node.findNodesForSubdomains("example.com")
	if len(nodes) != 2 {
		t.Errorf("Expected 2 nodes, got %d", len(nodes))
	}
}

func TestFindNodesForSubdomains_DoesNotMatchExactDomain(t *testing.T) {
	node := newTestNode()
	node.DomainRegistry = map[string][]string{
		"example.com": {"node1"},
	}
	node.KnownNodes = map[string]Node{
		"node1": {ID: "node1", Address: "localhost:8001"},
	}

	// Looking for subdomains of example.com should NOT return example.com itself
	nodes := node.findNodesForSubdomains("example.com")
	if len(nodes) != 0 {
		t.Errorf("Expected 0 nodes (exact match should not be included), got %d", len(nodes))
	}
}

func TestFindNodesForSubdomains_DoesNotMatchSimilarDomain(t *testing.T) {
	node := newTestNode()
	node.DomainRegistry = map[string][]string{
		"notexample.com":    {"node1"},
		"myexample.com":     {"node2"},
		"example.com.other": {"node3"},
	}
	node.KnownNodes = map[string]Node{
		"node1": {ID: "node1", Address: "localhost:8001"},
		"node2": {ID: "node2", Address: "localhost:8002"},
		"node3": {ID: "node3", Address: "localhost:8003"},
	}

	nodes := node.findNodesForSubdomains("example.com")
	if len(nodes) != 0 {
		t.Errorf("Expected 0 nodes (similar but not subdomain), got %d", len(nodes))
	}
}

func TestUpdateDomainRegistry_AddDomains(t *testing.T) {
	node := newTestNode()

	node.updateDomainRegistry("node1", []string{"example.com", "test.org"})

	node.DomainRegistryMutex.RLock()
	defer node.DomainRegistryMutex.RUnlock()

	if len(node.DomainRegistry["example.com"]) != 1 || node.DomainRegistry["example.com"][0] != "node1" {
		t.Errorf("Expected node1 in example.com, got %v", node.DomainRegistry["example.com"])
	}
	if len(node.DomainRegistry["test.org"]) != 1 || node.DomainRegistry["test.org"][0] != "node1" {
		t.Errorf("Expected node1 in test.org, got %v", node.DomainRegistry["test.org"])
	}
}

func TestUpdateDomainRegistry_UpdateDomains(t *testing.T) {
	node := newTestNode()

	// Initial registration
	node.updateDomainRegistry("node1", []string{"old.com", "shared.org"})

	// Update to new domains
	node.updateDomainRegistry("node1", []string{"new.com", "shared.org"})

	node.DomainRegistryMutex.RLock()
	defer node.DomainRegistryMutex.RUnlock()

	// old.com should be removed
	if _, exists := node.DomainRegistry["old.com"]; exists {
		t.Error("old.com should have been removed")
	}

	// new.com should be added
	if len(node.DomainRegistry["new.com"]) != 1 || node.DomainRegistry["new.com"][0] != "node1" {
		t.Errorf("Expected node1 in new.com, got %v", node.DomainRegistry["new.com"])
	}

	// shared.org should still have node1
	if len(node.DomainRegistry["shared.org"]) != 1 || node.DomainRegistry["shared.org"][0] != "node1" {
		t.Errorf("Expected node1 in shared.org, got %v", node.DomainRegistry["shared.org"])
	}
}

func TestUpdateDomainRegistry_MultipleNodes(t *testing.T) {
	node := newTestNode()

	node.updateDomainRegistry("node1", []string{"shared.com"})
	node.updateDomainRegistry("node2", []string{"shared.com"})

	node.DomainRegistryMutex.RLock()
	defer node.DomainRegistryMutex.RUnlock()

	if len(node.DomainRegistry["shared.com"]) != 2 {
		t.Errorf("Expected 2 nodes in shared.com, got %d", len(node.DomainRegistry["shared.com"]))
	}
}

func TestUpdateDomainRegistry_RemoveAllDomains(t *testing.T) {
	node := newTestNode()

	node.updateDomainRegistry("node1", []string{"example.com"})
	node.updateDomainRegistry("node1", []string{})

	node.DomainRegistryMutex.RLock()
	defer node.DomainRegistryMutex.RUnlock()

	if _, exists := node.DomainRegistry["example.com"]; exists {
		t.Error("example.com should have been removed when node1 has no domains")
	}
}

func TestQueryOtherDomain_FallsBackToSubdomainNodes(t *testing.T) {
	// Create a test server that responds to queries
	testServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": true,
			"data": map[string]interface{}{
				"result": "found via subdomain node",
			},
		})
	}))
	defer testServer.Close()

	node := newTestNode()
	address := strings.TrimPrefix(testServer.URL, "http://")

	// Register a node that manages a subdomain
	node.KnownNodes["subnode1"] = Node{
		ID:           "subnode1",
		Address:      address,
		TrustDomains: []string{"api.example.com"},
	}
	node.updateDomainRegistry("subnode1", []string{"api.example.com"})

	// Query for parent domain (example.com) - no node manages it directly
	// but subnode1 manages api.example.com
	result, err := node.QueryOtherDomain("example.com", "identity", "test")
	if err != nil {
		t.Fatalf("Expected success via subdomain fallback, got error: %v", err)
	}

	if result == nil {
		t.Error("Expected non-nil result")
	}
}

func TestQueryOtherDomain_PrefersParentOverSubdomain(t *testing.T) {
	parentCalled := false
	subdomainCalled := false

	// Create parent domain server
	parentServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		parentCalled = true
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": true,
			"data":    map[string]interface{}{"source": "parent"},
		})
	}))
	defer parentServer.Close()

	// Create subdomain server
	subdomainServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		subdomainCalled = true
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": true,
			"data":    map[string]interface{}{"source": "subdomain"},
		})
	}))
	defer subdomainServer.Close()

	node := newTestNode()
	parentAddr := strings.TrimPrefix(parentServer.URL, "http://")
	subdomainAddr := strings.TrimPrefix(subdomainServer.URL, "http://")

	// Register both parent and subdomain nodes
	node.KnownNodes["parentnode"] = Node{
		ID:           "parentnode",
		Address:      parentAddr,
		TrustDomains: []string{"example.com"},
	}
	node.KnownNodes["subnode"] = Node{
		ID:           "subnode",
		Address:      subdomainAddr,
		TrustDomains: []string{"api.example.com"},
	}
	node.updateDomainRegistry("parentnode", []string{"example.com"})
	node.updateDomainRegistry("subnode", []string{"api.example.com"})

	// Query for sub.example.com - should try example.com first (parent walking)
	_, err := node.QueryOtherDomain("sub.example.com", "identity", "test")
	if err != nil {
		t.Fatalf("Expected success, got error: %v", err)
	}

	if !parentCalled {
		t.Error("Expected parent domain node to be called")
	}
	if subdomainCalled {
		t.Error("Subdomain node should not be called when parent succeeds")
	}
}

func TestDiscoverFromSeeds_PopulatesDomains(t *testing.T) {
	// Create a test node that will be "discovered"
	discoveredNodeID := "discovered12345"

	// Create a test server simulating a seed node
	testServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/nodes":
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]interface{}{
				"nodes": []map[string]interface{}{
					{
						"id":               discoveredNodeID,
						"address":          strings.TrimPrefix(r.Host, "http://"),
						"trustDomains":     []string{},
						"isValidator":      true,
						"lastSeen":         time.Now().Unix(),
						"connectionStatus": "connected",
					},
				},
			})
		case "/api/v1/node/domains":
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]interface{}{
				"success": true,
				"data": map[string]interface{}{
					"nodeId":  discoveredNodeID,
					"domains": []string{"discovered.example.com", "*.discovered.org"},
				},
			})
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer testServer.Close()

	node := newTestNode()
	ctx := context.Background()

	seedAddress := strings.TrimPrefix(testServer.URL, "http://")

	node.discoverFromSeeds(ctx, []string{seedAddress})

	// Verify the discovered node has domain info populated
	node.KnownNodesMutex.RLock()
	discoveredNode, exists := node.KnownNodes[discoveredNodeID]
	node.KnownNodesMutex.RUnlock()

	if !exists {
		t.Fatal("Expected discovered node to be in KnownNodes")
	}

	if len(discoveredNode.TrustDomains) != 2 {
		t.Errorf("Expected 2 domains, got %d", len(discoveredNode.TrustDomains))
	}

	if discoveredNode.TrustDomains[0] != "discovered.example.com" {
		t.Errorf("Expected first domain 'discovered.example.com', got %s", discoveredNode.TrustDomains[0])
	}

	// Verify domain registry was also populated
	node.DomainRegistryMutex.RLock()
	nodesForDomain := node.DomainRegistry["discovered.example.com"]
	node.DomainRegistryMutex.RUnlock()

	if len(nodesForDomain) != 1 || nodesForDomain[0] != discoveredNodeID {
		t.Errorf("Expected domain registry to contain discovered node, got %v", nodesForDomain)
	}
}

func TestDiscoverFromSeeds_UpdatesDomainRegistry(t *testing.T) {
	discoveredNodeID := "discovered12345"

	testServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/nodes":
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]interface{}{
				"nodes": []map[string]interface{}{
					{
						"id":               discoveredNodeID,
						"address":          strings.TrimPrefix(r.Host, "http://"),
						"trustDomains":     []string{},
						"isValidator":      true,
						"lastSeen":         time.Now().Unix(),
						"connectionStatus": "connected",
					},
				},
			})
		case "/api/v1/node/domains":
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]interface{}{
				"success": true,
				"data": map[string]interface{}{
					"nodeId":  discoveredNodeID,
					"domains": []string{"api.example.com", "auth.example.com"},
				},
			})
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer testServer.Close()

	node := newTestNode()
	ctx := context.Background()
	seedAddress := strings.TrimPrefix(testServer.URL, "http://")

	node.discoverFromSeeds(ctx, []string{seedAddress})

	// Verify subdomain lookup works via domain registry
	subdomainNodes := node.findNodesForSubdomains("example.com")
	if len(subdomainNodes) != 1 {
		t.Fatalf("Expected 1 subdomain node, got %d", len(subdomainNodes))
	}
	if subdomainNodes[0].ID != discoveredNodeID {
		t.Errorf("Expected %s, got %s", discoveredNodeID, subdomainNodes[0].ID)
	}
}

func TestGetParentDomain(t *testing.T) {
	tests := []struct {
		domain   string
		expected string
	}{
		{"sub.example.com", "example.com"},
		{"deep.sub.example.com", "sub.example.com"},
		{"example.com", "com"},
		{"default", ""},
		{"mynetwork", ""},
		{"", ""},
		{"a.b.c.d.e", "b.c.d.e"},
	}

	for _, tt := range tests {
		result := GetParentDomain(tt.domain)
		if result != tt.expected {
			t.Errorf("GetParentDomain(%q) = %q, want %q", tt.domain, result, tt.expected)
		}
	}
}

func TestIsRootDomain(t *testing.T) {
	tests := []struct {
		domain   string
		expected bool
	}{
		{"default", true},
		{"mynetwork", true},
		{"example.com", false},
		{"sub.example.com", false},
		{"", true},
	}

	for _, tt := range tests {
		result := IsRootDomain(tt.domain)
		if result != tt.expected {
			t.Errorf("IsRootDomain(%q) = %v, want %v", tt.domain, result, tt.expected)
		}
	}
}

func TestRegisterTrustDomain_RootDomain_NoParentAuthRequired(t *testing.T) {
	node := newTestNode()
	node.AllowDomainRegistration = true
	node.RequireParentDomainAuth = true
	node.SupportedDomains = []string{} // allow all

	// Root domain (no dots) should succeed without parent auth
	err := node.RegisterTrustDomain(TrustDomain{Name: "mynetwork"})
	if err != nil {
		t.Errorf("Root domain registration should succeed, got error: %v", err)
	}

	// Verify domain was registered
	node.TrustDomainsMutex.RLock()
	_, exists := node.TrustDomains["mynetwork"]
	node.TrustDomainsMutex.RUnlock()

	if !exists {
		t.Error("Root domain should have been registered")
	}
}

func TestRegisterTrustDomain_Subdomain_ParentNotRegistered_Succeeds(t *testing.T) {
	node := newTestNode()
	node.AllowDomainRegistration = true
	node.RequireParentDomainAuth = true
	node.SupportedDomains = []string{} // allow all

	// Subdomain with unregistered parent should succeed (no authority to check against)
	err := node.RegisterTrustDomain(TrustDomain{Name: "sub.example.com"})
	if err != nil {
		t.Errorf("Subdomain with unregistered parent should succeed, got error: %v", err)
	}
}

func TestRegisterTrustDomain_Subdomain_ParentRegistered_NoTrust_Fails(t *testing.T) {
	node := newTestNode()
	node.AllowDomainRegistration = true
	node.RequireParentDomainAuth = true
	node.SupportedDomains = []string{} // allow all

	// Create a different node to be the parent validator
	parentNode, err := NewQuidnugNode(nil)
	if err != nil {
		t.Fatalf("Failed to create parent node: %v", err)
	}

	// Register parent domain with the other node as validator
	node.TrustDomainsMutex.Lock()
	node.TrustDomains["example.com"] = TrustDomain{
		Name:           "example.com",
		ValidatorNodes: []string{parentNode.NodeID},
		Validators:     map[string]float64{parentNode.NodeID: 1.0},
	}
	node.TrustDomainsMutex.Unlock()

	// Create a third node as the proposed subdomain validator (not trusted by parent)
	childNode, err := NewQuidnugNode(nil)
	if err != nil {
		t.Fatalf("Failed to create child node: %v", err)
	}

	// Try to register subdomain with untrusted validator - should fail
	err = node.RegisterTrustDomain(TrustDomain{
		Name:           "sub.example.com",
		ValidatorNodes: []string{childNode.NodeID},
	})

	if err == nil {
		t.Error("Subdomain registration with untrusted validator should fail")
	}
}

func TestRegisterTrustDomain_Subdomain_ParentRegistered_WithTrust_Succeeds(t *testing.T) {
	node := newTestNode()
	node.AllowDomainRegistration = true
	node.RequireParentDomainAuth = true
	node.SupportedDomains = []string{} // allow all

	// Create a different node to be the parent validator
	parentNode, err := NewQuidnugNode(nil)
	if err != nil {
		t.Fatalf("Failed to create parent node: %v", err)
	}

	// Register parent domain with the other node as validator
	node.TrustDomainsMutex.Lock()
	node.TrustDomains["example.com"] = TrustDomain{
		Name:           "example.com",
		ValidatorNodes: []string{parentNode.NodeID},
		Validators:     map[string]float64{parentNode.NodeID: 1.0},
	}
	node.TrustDomainsMutex.Unlock()

	// Create a third node as the proposed subdomain validator
	childNode, err := NewQuidnugNode(nil)
	if err != nil {
		t.Fatalf("Failed to create child node: %v", err)
	}

	// Establish trust from parent validator to child validator
	node.TrustRegistryMutex.Lock()
	if _, exists := node.TrustRegistry[parentNode.NodeID]; !exists {
		node.TrustRegistry[parentNode.NodeID] = make(map[string]float64)
	}
	node.TrustRegistry[parentNode.NodeID][childNode.NodeID] = 0.8
	node.TrustRegistryMutex.Unlock()

	// Now register subdomain - should succeed
	err = node.RegisterTrustDomain(TrustDomain{
		Name:           "sub.example.com",
		ValidatorNodes: []string{childNode.NodeID},
	})

	if err != nil {
		t.Errorf("Subdomain registration with trusted validator should succeed, got error: %v", err)
	}
}

func TestRegisterTrustDomain_Subdomain_RequireParentAuthDisabled_Succeeds(t *testing.T) {
	node := newTestNode()
	node.AllowDomainRegistration = true
	node.RequireParentDomainAuth = false // Disabled
	node.SupportedDomains = []string{}   // allow all

	// Create a different node to be the parent validator
	parentNode, err := NewQuidnugNode(nil)
	if err != nil {
		t.Fatalf("Failed to create parent node: %v", err)
	}

	// Register parent domain with the other node as validator
	node.TrustDomainsMutex.Lock()
	node.TrustDomains["example.com"] = TrustDomain{
		Name:           "example.com",
		ValidatorNodes: []string{parentNode.NodeID},
		Validators:     map[string]float64{parentNode.NodeID: 1.0},
	}
	node.TrustDomainsMutex.Unlock()

	// Create a third node as the proposed subdomain validator (not trusted by parent)
	childNode, err := NewQuidnugNode(nil)
	if err != nil {
		t.Fatalf("Failed to create child node: %v", err)
	}

	// Register subdomain with untrusted validator - should succeed because check is disabled
	err = node.RegisterTrustDomain(TrustDomain{
		Name:           "sub.example.com",
		ValidatorNodes: []string{childNode.NodeID},
	})

	if err != nil {
		t.Errorf("Subdomain registration should succeed when RequireParentDomainAuth is false, got error: %v", err)
	}
}

func TestValidateSubdomainAuthority_BothDomainsRegistered(t *testing.T) {
	node := newTestNode()

	// Create validators
	parentValidatorNode, _ := NewQuidnugNode(nil)
	childValidatorNode, _ := NewQuidnugNode(nil)

	// Register parent domain
	node.TrustDomainsMutex.Lock()
	node.TrustDomains["example.com"] = TrustDomain{
		Name:           "example.com",
		ValidatorNodes: []string{parentValidatorNode.NodeID},
	}
	node.TrustDomains["sub.example.com"] = TrustDomain{
		Name:           "sub.example.com",
		ValidatorNodes: []string{childValidatorNode.NodeID},
	}
	node.TrustDomainsMutex.Unlock()

	// No trust established - should return false
	result := node.ValidateSubdomainAuthority("sub.example.com", "example.com")
	if result {
		t.Error("Expected false when no trust exists between validators")
	}

	// Establish trust
	node.TrustRegistryMutex.Lock()
	node.TrustRegistry[parentValidatorNode.NodeID] = map[string]float64{
		childValidatorNode.NodeID: 0.9,
	}
	node.TrustRegistryMutex.Unlock()

	// Now should return true
	result = node.ValidateSubdomainAuthority("sub.example.com", "example.com")
	if !result {
		t.Error("Expected true when trust exists between validators")
	}
}

func TestValidateSubdomainAuthority_ParentNotRegistered(t *testing.T) {
	node := newTestNode()

	childValidatorNode, _ := NewQuidnugNode(nil)

	// Register only child domain
	node.TrustDomainsMutex.Lock()
	node.TrustDomains["sub.example.com"] = TrustDomain{
		Name:           "sub.example.com",
		ValidatorNodes: []string{childValidatorNode.NodeID},
	}
	node.TrustDomainsMutex.Unlock()

	// Should return true (no parent to check against)
	result := node.ValidateSubdomainAuthority("sub.example.com", "example.com")
	if !result {
		t.Error("Expected true when parent domain is not registered")
	}
}

func TestValidateSubdomainAuthority_ChildNotRegistered(t *testing.T) {
	node := newTestNode()

	parentValidatorNode, _ := NewQuidnugNode(nil)

	// Register only parent domain
	node.TrustDomainsMutex.Lock()
	node.TrustDomains["example.com"] = TrustDomain{
		Name:           "example.com",
		ValidatorNodes: []string{parentValidatorNode.NodeID},
	}
	node.TrustDomainsMutex.Unlock()

	// Should return false (child not registered)
	result := node.ValidateSubdomainAuthority("sub.example.com", "example.com")
	if result {
		t.Error("Expected false when child domain is not registered")
	}
}

func TestLoadConfigRequireParentDomainAuthDefault(t *testing.T) {
	clearConfigEnvVars()
	defer clearConfigEnvVars()

	cfg := LoadConfig()

	if !cfg.RequireParentDomainAuth {
		t.Error("Default RequireParentDomainAuth should be true")
	}
}

func TestLoadConfigRequireParentDomainAuthFromEnv(t *testing.T) {
	clearConfigEnvVars()
	defer clearConfigEnvVars()

	os.Setenv("REQUIRE_PARENT_DOMAIN_AUTH", "false")
	cfg := LoadConfig()
	if cfg.RequireParentDomainAuth {
		t.Error("RequireParentDomainAuth should be false when env var is 'false'")
	}

	os.Setenv("REQUIRE_PARENT_DOMAIN_AUTH", "true")
	cfg = LoadConfig()
	if !cfg.RequireParentDomainAuth {
		t.Error("RequireParentDomainAuth should be true when env var is 'true'")
	}
}

func TestRegisterTrustDomain_DeepSubdomain_ChecksImmediateParent(t *testing.T) {
	node := newTestNode()
	node.AllowDomainRegistration = true
	node.RequireParentDomainAuth = true
	node.SupportedDomains = []string{} // allow all

	// Create validators
	midValidatorNode, _ := NewQuidnugNode(nil)
	childValidatorNode, _ := NewQuidnugNode(nil)

	// Register mid-level domain (example.com is NOT registered)
	node.TrustDomainsMutex.Lock()
	node.TrustDomains["sub.example.com"] = TrustDomain{
		Name:           "sub.example.com",
		ValidatorNodes: []string{midValidatorNode.NodeID},
		Validators:     map[string]float64{midValidatorNode.NodeID: 1.0},
	}
	node.TrustDomainsMutex.Unlock()

	// Establish trust from mid validator to child validator
	node.TrustRegistryMutex.Lock()
	node.TrustRegistry[midValidatorNode.NodeID] = map[string]float64{
		childValidatorNode.NodeID: 0.7,
	}
	node.TrustRegistryMutex.Unlock()

	// Register deep subdomain - should succeed because immediate parent (sub.example.com) trusts child
	err := node.RegisterTrustDomain(TrustDomain{
		Name:           "deep.sub.example.com",
		ValidatorNodes: []string{childValidatorNode.NodeID},
	})

	if err != nil {
		t.Errorf("Deep subdomain registration should succeed when immediate parent trusts validator, got error: %v", err)
	}
}

func TestCrossDomainQueryDelegation_ViaSubdomain(t *testing.T) {
	// This integration test verifies that queries can be delegated to subdomain nodes
	// when no exact or parent domain node exists

	queryReceived := make(chan string, 1)

	// Create a server for the subdomain node
	subdomainServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "/query") {
			queryReceived <- r.URL.Query().Get("param")
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]interface{}{
				"success": true,
				"data": map[string]interface{}{
					"quidId": "testquid1234567",
					"name":   "Test Identity",
				},
			})
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer subdomainServer.Close()

	node := newTestNode()
	subAddr := strings.TrimPrefix(subdomainServer.URL, "http://")

	// Register a node that manages a subdomain of example.com
	node.KnownNodes["subnode1"] = Node{
		ID:           "subnode1",
		Address:      subAddr,
		TrustDomains: []string{"api.example.com"},
	}
	node.updateDomainRegistry("subnode1", []string{"api.example.com"})

	// Query for example.com (no direct or parent node exists)
	// Should fall back to subdomain node
	result, err := node.QueryOtherDomain("example.com", "identity", "testquid1234567")
	if err != nil {
		t.Fatalf("Expected query to succeed via subdomain delegation, got error: %v", err)
	}

	// Verify the query was received by the subdomain node
	select {
	case param := <-queryReceived:
		if param != "testquid1234567" {
			t.Errorf("Expected param 'testquid1234567', got '%s'", param)
		}
	case <-time.After(time.Second):
		t.Fatal("Subdomain node did not receive the query")
	}

	if result == nil {
		t.Error("Expected non-nil result from subdomain delegation")
	}
}
