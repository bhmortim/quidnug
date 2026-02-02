package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gorilla/mux"
)

func setupTestRouter(node *QuidnugNode) *mux.Router {
	router := mux.NewRouter()
	router.HandleFunc("/api/health", node.HealthCheckHandler).Methods("GET")
	router.HandleFunc("/api/info", node.GetInfoHandler).Methods("GET")
	router.HandleFunc("/api/quids", node.CreateQuidHandler).Methods("POST")
	router.HandleFunc("/api/trust/query", node.RelationalTrustQueryHandler).Methods("POST")
	router.HandleFunc("/api/trust/edges/{quidId}", node.GetTrustEdgesHandler).Methods("GET")
	router.HandleFunc("/api/trust/{observer}/{target}", node.GetTrustHandler).Methods("GET")
	router.HandleFunc("/api/identity/{quidId}", node.GetIdentityHandler).Methods("GET")
	router.HandleFunc("/api/title/{assetId}", node.GetTitleHandler).Methods("GET")
	router.HandleFunc("/api/nodes", node.GetNodesHandler).Methods("GET")
	router.HandleFunc("/api/blocks", node.GetBlocksHandler).Methods("GET")
	router.HandleFunc("/api/blocks/tentative/{domain}", node.GetTentativeBlocksHandler).Methods("GET")
	router.HandleFunc("/api/domains", node.GetDomainsHandler).Methods("GET")
	return router
}

func TestHealthCheckHandler(t *testing.T) {
	node := newTestNode()
	router := setupTestRouter(node)

	req := httptest.NewRequest("GET", "/api/health", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	var response map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	if response["status"] != "ok" {
		t.Errorf("Expected status 'ok', got '%v'", response["status"])
	}

	if response["node_id"] != node.NodeID {
		t.Errorf("Expected node_id '%s', got '%v'", node.NodeID, response["node_id"])
	}
}

func TestGetInfoHandler(t *testing.T) {
	node := newTestNode()
	router := setupTestRouter(node)

	req := httptest.NewRequest("GET", "/api/info", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	var response map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	if response["nodeQuid"] != node.NodeID {
		t.Errorf("Expected nodeQuid '%s', got '%v'", node.NodeID, response["nodeQuid"])
	}

	if response["version"] != "1.0.0" {
		t.Errorf("Expected version '1.0.0', got '%v'", response["version"])
	}

	if _, ok := response["managedDomains"].([]interface{}); !ok {
		t.Error("Expected managedDomains to be an array")
	}

	if _, ok := response["blockHeight"].(float64); !ok {
		t.Error("Expected blockHeight to be a number")
	}
}

func TestCreateQuidHandler(t *testing.T) {
	node := newTestNode()
	router := setupTestRouter(node)

	t.Run("create quid without metadata", func(t *testing.T) {
		req := httptest.NewRequest("POST", "/api/quids", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		if w.Code != http.StatusCreated {
			t.Errorf("Expected status 201, got %d", w.Code)
		}

		var response map[string]interface{}
		if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
			t.Fatalf("Failed to parse response: %v", err)
		}

		quidID, ok := response["quidId"].(string)
		if !ok || len(quidID) != 16 {
			t.Errorf("Expected 16-character quidId, got '%v'", response["quidId"])
		}

		if _, ok := response["publicKey"].(string); !ok {
			t.Error("Expected publicKey to be a string")
		}

		if _, ok := response["created"].(float64); !ok {
			t.Error("Expected created to be a number")
		}
	})

	t.Run("create quid with metadata", func(t *testing.T) {
		body := bytes.NewBufferString(`{"metadata":{"name":"Test Quid"}}`)
		req := httptest.NewRequest("POST", "/api/quids", body)
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		if w.Code != http.StatusCreated {
			t.Errorf("Expected status 201, got %d", w.Code)
		}

		var response map[string]interface{}
		if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
			t.Fatalf("Failed to parse response: %v", err)
		}

		if _, ok := response["quidId"].(string); !ok {
			t.Error("Expected quidId to be a string")
		}
	})
}

func TestGetTrustHandler(t *testing.T) {
	node := newTestNode()
	router := setupTestRouter(node)

	node.TrustRegistryMutex.Lock()
	if node.TrustRegistry["quid_truster_001"] == nil {
		node.TrustRegistry["quid_truster_001"] = make(map[string]float64)
	}
	node.TrustRegistry["quid_truster_001"]["quid_trustee_001"] = 0.85
	node.TrustRegistryMutex.Unlock()

	t.Run("existing trust relationship", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/trust/quid_truster_001/quid_trustee_001?domain=test.domain.com", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("Expected status 200, got %d", w.Code)
		}

		var response map[string]interface{}
		if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
			t.Fatalf("Failed to parse response: %v", err)
		}

		if response["observer"] != "quid_truster_001" {
			t.Errorf("Expected observer 'quid_truster_001', got '%v'", response["observer"])
		}

		if response["target"] != "quid_trustee_001" {
			t.Errorf("Expected target 'quid_trustee_001', got '%v'", response["target"])
		}

		if response["domain"] != "test.domain.com" {
			t.Errorf("Expected domain 'test.domain.com', got '%v'", response["domain"])
		}

		if response["trustLevel"] != 0.85 {
			t.Errorf("Expected trustLevel 0.85, got '%v'", response["trustLevel"])
		}

		trustPath, ok := response["trustPath"].([]interface{})
		if !ok {
			t.Error("Expected trustPath to be an array")
		} else if len(trustPath) != 2 {
			t.Errorf("Expected trustPath length 2, got %d", len(trustPath))
		}

		pathDepth, ok := response["pathDepth"].(float64)
		if !ok {
			t.Error("Expected pathDepth to be a number")
		} else if int(pathDepth) != 1 {
			t.Errorf("Expected pathDepth 1, got %v", pathDepth)
		}
	})

	t.Run("non-existing trust relationship", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/trust/unknown_truster/unknown_trustee?domain=default", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("Expected status 200, got %d", w.Code)
		}

		var response map[string]interface{}
		if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
			t.Fatalf("Failed to parse response: %v", err)
		}

		if response["trustLevel"] != 0.0 {
			t.Errorf("Expected trustLevel 0.0 for non-existing relationship, got '%v'", response["trustLevel"])
		}

		trustPath, ok := response["trustPath"].([]interface{})
		if ok && len(trustPath) != 0 {
			t.Errorf("Expected empty trustPath for non-existing relationship, got %v", trustPath)
		}
	})

	t.Run("default domain when not specified", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/trust/quid_truster_001/quid_trustee_001", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("Expected status 200, got %d", w.Code)
		}

		var response map[string]interface{}
		json.Unmarshal(w.Body.Bytes(), &response)

		if response["domain"] != "default" {
			t.Errorf("Expected domain 'default', got '%v'", response["domain"])
		}
	})

	t.Run("maxDepth query parameter", func(t *testing.T) {
		node.TrustRegistryMutex.Lock()
		node.TrustRegistry["quid_truster_001"]["intermediate_001"] = 0.9
		if node.TrustRegistry["intermediate_001"] == nil {
			node.TrustRegistry["intermediate_001"] = make(map[string]float64)
		}
		node.TrustRegistry["intermediate_001"]["distant_target"] = 0.9
		node.TrustRegistryMutex.Unlock()

		req := httptest.NewRequest("GET", "/api/trust/quid_truster_001/distant_target?maxDepth=1", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("Expected status 200, got %d", w.Code)
		}

		var response map[string]interface{}
		json.Unmarshal(w.Body.Bytes(), &response)

		if response["trustLevel"] != 0.0 {
			t.Errorf("Expected trustLevel 0.0 with maxDepth=1 (target is 2 hops away), got '%v'", response["trustLevel"])
		}

		req2 := httptest.NewRequest("GET", "/api/trust/quid_truster_001/distant_target?maxDepth=3", nil)
		w2 := httptest.NewRecorder()
		router.ServeHTTP(w2, req2)

		var response2 map[string]interface{}
		json.Unmarshal(w2.Body.Bytes(), &response2)

		if response2["trustLevel"] == 0.0 {
			t.Error("Expected non-zero trustLevel with maxDepth=3")
		}
	})
}

func TestGetIdentityHandler(t *testing.T) {
	node := newTestNode()
	router := setupTestRouter(node)

	t.Run("existing identity", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/identity/quid_truster_001", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("Expected status 200, got %d", w.Code)
		}

		var response map[string]interface{}
		if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
			t.Fatalf("Failed to parse response: %v", err)
		}

		if response["quidId"] != "quid_truster_001" {
			t.Errorf("Expected quidId 'quid_truster_001', got '%v'", response["quidId"])
		}

		if response["name"] != "Test Truster" {
			t.Errorf("Expected name 'Test Truster', got '%v'", response["name"])
		}
	})

	t.Run("non-existing identity", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/identity/unknown_quid", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		if w.Code != http.StatusNotFound {
			t.Errorf("Expected status 404, got %d", w.Code)
		}
	})

	t.Run("identity with domain query param", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/identity/quid_truster_001?domain=test.domain.com", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("Expected status 200, got %d", w.Code)
		}
	})
}

func TestGetTitleHandler(t *testing.T) {
	node := newTestNode()
	router := setupTestRouter(node)

	t.Run("existing title", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/title/quid_asset_001", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("Expected status 200, got %d", w.Code)
		}

		var response map[string]interface{}
		if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
			t.Fatalf("Failed to parse response: %v", err)
		}

		if response["assetId"] != "quid_asset_001" {
			t.Errorf("Expected assetId 'quid_asset_001', got '%v'", response["assetId"])
		}

		owners, ok := response["owners"].([]interface{})
		if !ok {
			t.Error("Expected owners to be an array")
		} else if len(owners) != 2 {
			t.Errorf("Expected 2 owners, got %d", len(owners))
		}
	})

	t.Run("non-existing title", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/title/unknown_asset", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		if w.Code != http.StatusNotFound {
			t.Errorf("Expected status 404, got %d", w.Code)
		}
	})

	t.Run("title with domain query param", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/title/quid_asset_001?domain=test.domain.com", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("Expected status 200, got %d", w.Code)
		}
	})
}

func TestGetNodesHandler(t *testing.T) {
	node := newTestNode()
	router := setupTestRouter(node)

	req := httptest.NewRequest("GET", "/api/nodes", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	var response map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	if _, ok := response["nodes"]; !ok {
		t.Error("Expected 'nodes' key in response")
	}
}

func TestGetBlocksHandler(t *testing.T) {
	node := newTestNode()
	router := setupTestRouter(node)

	req := httptest.NewRequest("GET", "/api/blocks", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	var response map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	blocks, ok := response["blocks"].([]interface{})
	if !ok {
		t.Error("Expected 'blocks' to be an array")
	}

	if len(blocks) < 1 {
		t.Error("Expected at least genesis block")
	}
}

func TestGetDomainsHandler(t *testing.T) {
	node := newTestNode()
	router := setupTestRouter(node)

	req := httptest.NewRequest("GET", "/api/domains", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	var response map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	if _, ok := response["domains"]; !ok {
		t.Error("Expected 'domains' key in response")
	}
}

func TestGetTrustHandler_IncludeUnverified(t *testing.T) {
	node := newTestNode()
	router := setupTestRouter(node)

	// Set up verified trust edges: A -> B
	node.AddVerifiedTrustEdge(TrustEdge{
		Truster:       "observer_enhanced",
		Trustee:       "target_enhanced1",
		TrustLevel:    0.8,
		SourceBlock:   "block123",
		ValidatorQuid: node.NodeID,
		Verified:      true,
		Timestamp:     1000000,
	})

	t.Run("includeUnverified=true returns enhanced result", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/trust/observer_enhanced/target_enhanced1?includeUnverified=true", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("Expected status 200, got %d", w.Code)
		}

		var response map[string]interface{}
		if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
			t.Fatalf("Failed to parse response: %v", err)
		}

		// Check for EnhancedTrustResult fields
		if _, ok := response["confidence"]; !ok {
			t.Error("Expected 'confidence' field in enhanced result")
		}

		if _, ok := response["unverifiedHops"]; !ok {
			t.Error("Expected 'unverifiedHops' field in enhanced result")
		}

		if _, ok := response["verificationGaps"]; !ok {
			t.Error("Expected 'verificationGaps' field in enhanced result")
		}

		if response["confidence"] != "high" {
			t.Errorf("Expected confidence 'high' for verified path, got '%v'", response["confidence"])
		}

		if response["trustLevel"] != 0.8 {
			t.Errorf("Expected trustLevel 0.8, got '%v'", response["trustLevel"])
		}
	})

	t.Run("includeUnverified=false returns standard result", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/trust/observer_enhanced/target_enhanced1?includeUnverified=false", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("Expected status 200, got %d", w.Code)
		}

		var response map[string]interface{}
		json.Unmarshal(w.Body.Bytes(), &response)

		// Standard result should NOT have confidence field
		if _, ok := response["confidence"]; ok {
			t.Error("Standard result should not have 'confidence' field")
		}
	})
}

func TestRelationalTrustQueryHandler_IncludeUnverified(t *testing.T) {
	node := newTestNode()
	router := setupTestRouter(node)

	// Set up verified trust edges
	node.AddVerifiedTrustEdge(TrustEdge{
		Truster:       "query_observer_01",
		Trustee:       "query_target_001",
		TrustLevel:    0.9,
		SourceBlock:   "block456",
		ValidatorQuid: node.NodeID,
		Verified:      true,
		Timestamp:     1000000,
	})

	t.Run("includeUnverified true in request body", func(t *testing.T) {
		body := bytes.NewBufferString(`{"observer":"query_observer_01","target":"query_target_001","includeUnverified":true}`)
		req := httptest.NewRequest("POST", "/api/trust/query", body)
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("Expected status 200, got %d", w.Code)
		}

		var response map[string]interface{}
		if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
			t.Fatalf("Failed to parse response: %v", err)
		}

		// Check for EnhancedTrustResult fields
		if _, ok := response["confidence"]; !ok {
			t.Error("Expected 'confidence' field in enhanced result")
		}

		if _, ok := response["unverifiedHops"]; !ok {
			t.Error("Expected 'unverifiedHops' field in enhanced result")
		}

		if response["trustLevel"] != 0.9 {
			t.Errorf("Expected trustLevel 0.9, got '%v'", response["trustLevel"])
		}
	})

	t.Run("includeUnverified false returns standard result", func(t *testing.T) {
		body := bytes.NewBufferString(`{"observer":"query_observer_01","target":"query_target_001","includeUnverified":false}`)
		req := httptest.NewRequest("POST", "/api/trust/query", body)
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("Expected status 200, got %d", w.Code)
		}

		var response map[string]interface{}
		json.Unmarshal(w.Body.Bytes(), &response)

		// Standard result should NOT have confidence field
		if _, ok := response["confidence"]; ok {
			t.Error("Standard result should not have 'confidence' field")
		}
	})
}

func TestGetTentativeBlocksHandler(t *testing.T) {
	node := newTestNode()
	router := setupTestRouter(node)

	t.Run("empty domain returns empty blocks", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/blocks/tentative/nonexistent", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("Expected status 200, got %d", w.Code)
		}

		var response map[string]interface{}
		if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
			t.Fatalf("Failed to parse response: %v", err)
		}

		if response["domain"] != "nonexistent" {
			t.Errorf("Expected domain 'nonexistent', got '%v'", response["domain"])
		}
	})

	t.Run("domain with tentative blocks returns them", func(t *testing.T) {
		// Store a tentative block
		block := Block{
			Index:        1,
			Timestamp:    1234567890,
			Transactions: []interface{}{},
			PrevHash:     node.Blockchain[0].Hash,
			TrustProof: TrustProof{
				TrustDomain:   "testdomain",
				ValidatorID:   "somevalidator123",
				ValidatorSigs: []string{"sig"},
			},
		}
		block.Hash = calculateBlockHash(block)

		err := node.StoreTentativeBlock(block)
		if err != nil {
			t.Fatalf("Failed to store tentative block: %v", err)
		}

		req := httptest.NewRequest("GET", "/api/blocks/tentative/testdomain", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("Expected status 200, got %d", w.Code)
		}

		var response map[string]interface{}
		if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
			t.Fatalf("Failed to parse response: %v", err)
		}

		if response["domain"] != "testdomain" {
			t.Errorf("Expected domain 'testdomain', got '%v'", response["domain"])
		}

		blocks, ok := response["blocks"].([]interface{})
		if !ok {
			t.Error("Expected 'blocks' to be an array")
		} else if len(blocks) != 1 {
			t.Errorf("Expected 1 tentative block, got %d", len(blocks))
		}
	})
}

func TestGetTrustEdgesHandler(t *testing.T) {
	node := newTestNode()
	router := setupTestRouter(node)

	// Add verified edges
	node.AddVerifiedTrustEdge(TrustEdge{
		Truster:       "edges_quid_0001",
		Trustee:       "edges_target_001",
		TrustLevel:    0.85,
		SourceBlock:   "block789",
		ValidatorQuid: node.NodeID,
		Verified:      true,
		Timestamp:     1000000,
	})

	node.AddVerifiedTrustEdge(TrustEdge{
		Truster:       "edges_quid_0001",
		Trustee:       "edges_target_002",
		TrustLevel:    0.7,
		SourceBlock:   "block790",
		ValidatorQuid: node.NodeID,
		Verified:      true,
		Timestamp:     1000001,
	})

	t.Run("returns verified edges with provenance", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/trust/edges/edges_quid_0001?includeUnverified=false", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("Expected status 200, got %d", w.Code)
		}

		var response map[string]interface{}
		if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
			t.Fatalf("Failed to parse response: %v", err)
		}

		if response["quidId"] != "edges_quid_0001" {
			t.Errorf("Expected quidId 'edges_quid_0001', got '%v'", response["quidId"])
		}

		if response["includeUnverified"] != false {
			t.Errorf("Expected includeUnverified false, got '%v'", response["includeUnverified"])
		}

		edges, ok := response["edges"].(map[string]interface{})
		if !ok {
			t.Error("Expected 'edges' to be a map")
		} else if len(edges) != 2 {
			t.Errorf("Expected 2 edges, got %d", len(edges))
		}

		// Check edge provenance fields
		if edge1, ok := edges["edges_target_001"].(map[string]interface{}); ok {
			if edge1["trustLevel"] != 0.85 {
				t.Errorf("Expected trustLevel 0.85, got '%v'", edge1["trustLevel"])
			}
			if edge1["sourceBlock"] != "block789" {
				t.Errorf("Expected sourceBlock 'block789', got '%v'", edge1["sourceBlock"])
			}
			if edge1["verified"] != true {
				t.Errorf("Expected verified true, got '%v'", edge1["verified"])
			}
		} else {
			t.Error("Expected edge to edges_target_001 to exist")
		}
	})

	t.Run("includeUnverified returns both verified and unverified", func(t *testing.T) {
		// Add an unverified edge
		node.AddUnverifiedTrustEdge(TrustEdge{
			Truster:       "edges_quid_0001",
			Trustee:       "unverified_tgt01",
			TrustLevel:    0.5,
			SourceBlock:   "block999",
			ValidatorQuid: "untrusted_val_01",
			Verified:      false,
			Timestamp:     1000002,
		})

		req := httptest.NewRequest("GET", "/api/trust/edges/edges_quid_0001?includeUnverified=true", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("Expected status 200, got %d", w.Code)
		}

		var response map[string]interface{}
		json.Unmarshal(w.Body.Bytes(), &response)

		if response["includeUnverified"] != true {
			t.Errorf("Expected includeUnverified true, got '%v'", response["includeUnverified"])
		}

		edges, ok := response["edges"].(map[string]interface{})
		if !ok {
			t.Error("Expected 'edges' to be a map")
		} else if len(edges) != 3 {
			t.Errorf("Expected 3 edges (2 verified + 1 unverified), got %d", len(edges))
		}

		// Check unverified edge
		if unverifiedEdge, ok := edges["unverified_tgt01"].(map[string]interface{}); ok {
			if unverifiedEdge["verified"] != false {
				t.Errorf("Expected verified false for unverified edge, got '%v'", unverifiedEdge["verified"])
			}
		} else {
			t.Error("Expected unverified edge to exist when includeUnverified=true")
		}
	})

	t.Run("quid with no edges returns empty map", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/trust/edges/nonexistent_quid", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("Expected status 200, got %d", w.Code)
		}

		var response map[string]interface{}
		json.Unmarshal(w.Body.Bytes(), &response)

		edges, ok := response["edges"].(map[string]interface{})
		if !ok {
			t.Error("Expected 'edges' to be a map")
		} else if len(edges) != 0 {
			t.Errorf("Expected 0 edges for nonexistent quid, got %d", len(edges))
		}
	})
}

func TestRelationalTrustQueryHandler(t *testing.T) {
	node := newTestNode()
	router := setupTestRouter(node)

	node.TrustRegistryMutex.Lock()
	if node.TrustRegistry["observer_quid_01"] == nil {
		node.TrustRegistry["observer_quid_01"] = make(map[string]float64)
	}
	node.TrustRegistry["observer_quid_01"]["target_quid_001"] = 0.75
	node.TrustRegistryMutex.Unlock()

	t.Run("valid query with observer and target", func(t *testing.T) {
		body := bytes.NewBufferString(`{"observer":"observer_quid_01","target":"target_quid_001","domain":"test.domain"}`)
		req := httptest.NewRequest("POST", "/api/trust/query", body)
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("Expected status 200, got %d", w.Code)
		}

		var response map[string]interface{}
		if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
			t.Fatalf("Failed to parse response: %v", err)
		}

		if response["observer"] != "observer_quid_01" {
			t.Errorf("Expected observer 'observer_quid_01', got '%v'", response["observer"])
		}

		if response["target"] != "target_quid_001" {
			t.Errorf("Expected target 'target_quid_001', got '%v'", response["target"])
		}

		if response["trustLevel"] != 0.75 {
			t.Errorf("Expected trustLevel 0.75, got '%v'", response["trustLevel"])
		}

		if response["domain"] != "test.domain" {
			t.Errorf("Expected domain 'test.domain', got '%v'", response["domain"])
		}

		trustPath, ok := response["trustPath"].([]interface{})
		if !ok {
			t.Error("Expected trustPath to be an array")
		} else if len(trustPath) != 2 {
			t.Errorf("Expected trustPath length 2, got %d", len(trustPath))
		}

		pathDepth, ok := response["pathDepth"].(float64)
		if !ok {
			t.Error("Expected pathDepth to be a number")
		} else if int(pathDepth) != 1 {
			t.Errorf("Expected pathDepth 1, got %v", pathDepth)
		}
	})

	t.Run("missing observer", func(t *testing.T) {
		body := bytes.NewBufferString(`{"target":"target_quid_001","domain":"test.domain"}`)
		req := httptest.NewRequest("POST", "/api/trust/query", body)
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		if w.Code != http.StatusBadRequest {
			t.Errorf("Expected status 400, got %d", w.Code)
		}
	})

	t.Run("missing target", func(t *testing.T) {
		body := bytes.NewBufferString(`{"observer":"observer_quid_01","domain":"test.domain"}`)
		req := httptest.NewRequest("POST", "/api/trust/query", body)
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		if w.Code != http.StatusBadRequest {
			t.Errorf("Expected status 400, got %d", w.Code)
		}
	})

	t.Run("query with maxDepth parameter", func(t *testing.T) {
		node.TrustRegistryMutex.Lock()
		if node.TrustRegistry["hop1_quid_00001"] == nil {
			node.TrustRegistry["hop1_quid_00001"] = make(map[string]float64)
		}
		node.TrustRegistry["hop1_quid_00001"]["hop2_quid_00001"] = 0.8
		if node.TrustRegistry["hop2_quid_00001"] == nil {
			node.TrustRegistry["hop2_quid_00001"] = make(map[string]float64)
		}
		node.TrustRegistry["hop2_quid_00001"]["hop3_quid_00001"] = 0.8
		node.TrustRegistryMutex.Unlock()

		body := bytes.NewBufferString(`{"observer":"hop1_quid_00001","target":"hop3_quid_00001","maxDepth":1}`)
		req := httptest.NewRequest("POST", "/api/trust/query", body)
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("Expected status 200, got %d", w.Code)
		}

		var response map[string]interface{}
		json.Unmarshal(w.Body.Bytes(), &response)

		if response["trustLevel"] != 0.0 {
			t.Errorf("Expected trustLevel 0.0 with maxDepth=1 (target is 2 hops away), got '%v'", response["trustLevel"])
		}

		body2 := bytes.NewBufferString(`{"observer":"hop1_quid_00001","target":"hop3_quid_00001","maxDepth":3}`)
		req2 := httptest.NewRequest("POST", "/api/trust/query", body2)
		req2.Header.Set("Content-Type", "application/json")
		w2 := httptest.NewRecorder()
		router.ServeHTTP(w2, req2)

		var response2 map[string]interface{}
		json.Unmarshal(w2.Body.Bytes(), &response2)

		expected := 0.8 * 0.8
		if response2["trustLevel"] != expected {
			t.Errorf("Expected trustLevel %f with maxDepth=3, got '%v'", expected, response2["trustLevel"])
		}
	})

	t.Run("query that returns no path", func(t *testing.T) {
		body := bytes.NewBufferString(`{"observer":"isolated_quid_1","target":"isolated_quid_2","domain":"test.domain"}`)
		req := httptest.NewRequest("POST", "/api/trust/query", body)
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("Expected status 200, got %d", w.Code)
		}

		var response map[string]interface{}
		json.Unmarshal(w.Body.Bytes(), &response)

		if response["trustLevel"] != 0.0 {
			t.Errorf("Expected trustLevel 0.0 for no path, got '%v'", response["trustLevel"])
		}

		pathDepth, ok := response["pathDepth"].(float64)
		if !ok || int(pathDepth) != 0 {
			t.Errorf("Expected pathDepth 0, got '%v'", response["pathDepth"])
		}
	})
}
