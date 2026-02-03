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
}
