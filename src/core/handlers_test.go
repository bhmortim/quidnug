package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
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

	if response["success"] != true {
		t.Errorf("Expected success true, got '%v'", response["success"])
	}

	data, ok := response["data"].(map[string]interface{})
	if !ok {
		t.Fatal("Expected data to be a map")
	}

	if data["status"] != "ok" {
		t.Errorf("Expected status 'ok', got '%v'", data["status"])
	}

	if data["node_id"] != node.NodeID {
		t.Errorf("Expected node_id '%s', got '%v'", node.NodeID, data["node_id"])
	}

	// Check X-API-Version header
	if w.Header().Get("X-API-Version") != "1.0" {
		t.Errorf("Expected X-API-Version '1.0', got '%s'", w.Header().Get("X-API-Version"))
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

	if response["success"] != true {
		t.Errorf("Expected success true, got '%v'", response["success"])
	}

	data, ok := response["data"].(map[string]interface{})
	if !ok {
		t.Fatal("Expected data to be a map")
	}

	if data["nodeQuid"] != node.NodeID {
		t.Errorf("Expected nodeQuid '%s', got '%v'", node.NodeID, data["nodeQuid"])
	}

	if data["version"] != "1.0.0" {
		t.Errorf("Expected version '1.0.0', got '%v'", data["version"])
	}

	if _, ok := data["managedDomains"].([]interface{}); !ok {
		t.Error("Expected managedDomains to be an array")
	}

	if _, ok := data["blockHeight"].(float64); !ok {
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

		if response["success"] != true {
			t.Errorf("Expected success true, got '%v'", response["success"])
		}

		data, ok := response["data"].(map[string]interface{})
		if !ok {
			t.Fatal("Expected data to be a map")
		}

		quidID, ok := data["quidId"].(string)
		if !ok || len(quidID) != 16 {
			t.Errorf("Expected 16-character quidId, got '%v'", data["quidId"])
		}

		if _, ok := data["publicKey"].(string); !ok {
			t.Error("Expected publicKey to be a string")
		}

		if _, ok := data["created"].(float64); !ok {
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

		if response["success"] != true {
			t.Errorf("Expected success true, got '%v'", response["success"])
		}

		data, ok := response["data"].(map[string]interface{})
		if !ok {
			t.Fatal("Expected data to be a map")
		}

		if _, ok := data["quidId"].(string); !ok {
			t.Error("Expected quidId to be a string")
		}
	})
}

func TestGetTrustHandler(t *testing.T) {
	node := newTestNode()
	router := setupTestRouter(node)

	node.TrustRegistryMutex.Lock()
	if node.TrustRegistry["0000000000000001"] == nil {
		node.TrustRegistry["0000000000000001"] = make(map[string]float64)
	}
	node.TrustRegistry["0000000000000001"]["0000000000000002"] = 0.85
	node.TrustRegistryMutex.Unlock()

	t.Run("existing trust relationship", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/trust/0000000000000001/0000000000000002?domain=test.domain.com", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("Expected status 200, got %d", w.Code)
		}

		var response map[string]interface{}
		if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
			t.Fatalf("Failed to parse response: %v", err)
		}

		if response["success"] != true {
			t.Errorf("Expected success true, got '%v'", response["success"])
		}

		data, ok := response["data"].(map[string]interface{})
		if !ok {
			t.Fatal("Expected data to be a map")
		}

		if data["observer"] != "0000000000000001" {
			t.Errorf("Expected observer '0000000000000001', got '%v'", data["observer"])
		}

		if data["target"] != "0000000000000002" {
			t.Errorf("Expected target '0000000000000002', got '%v'", data["target"])
		}

		if data["domain"] != "test.domain.com" {
			t.Errorf("Expected domain 'test.domain.com', got '%v'", data["domain"])
		}

		if data["trustLevel"] != 0.85 {
			t.Errorf("Expected trustLevel 0.85, got '%v'", data["trustLevel"])
		}

		trustPath, ok := data["trustPath"].([]interface{})
		if !ok {
			t.Error("Expected trustPath to be an array")
		} else if len(trustPath) != 2 {
			t.Errorf("Expected trustPath length 2, got %d", len(trustPath))
		}

		pathDepth, ok := data["pathDepth"].(float64)
		if !ok {
			t.Error("Expected pathDepth to be a number")
		} else if int(pathDepth) != 1 {
			t.Errorf("Expected pathDepth 1, got %v", pathDepth)
		}
	})

	t.Run("non-existing trust relationship", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/trust/000000000000001d/000000000000001e?domain=default", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("Expected status 200, got %d", w.Code)
		}

		var response map[string]interface{}
		if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
			t.Fatalf("Failed to parse response: %v", err)
		}

		if response["success"] != true {
			t.Errorf("Expected success true, got '%v'", response["success"])
		}

		data, ok := response["data"].(map[string]interface{})
		if !ok {
			t.Fatal("Expected data to be a map")
		}

		if data["trustLevel"] != 0.0 {
			t.Errorf("Expected trustLevel 0.0 for non-existing relationship, got '%v'", data["trustLevel"])
		}

		trustPath, ok := data["trustPath"].([]interface{})
		if ok && len(trustPath) != 0 {
			t.Errorf("Expected empty trustPath for non-existing relationship, got %v", trustPath)
		}
	})

	t.Run("default domain when not specified", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/trust/0000000000000001/0000000000000002", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("Expected status 200, got %d", w.Code)
		}

		var response map[string]interface{}
		json.Unmarshal(w.Body.Bytes(), &response)

		data := response["data"].(map[string]interface{})
		if data["domain"] != "default" {
			t.Errorf("Expected domain 'default', got '%v'", data["domain"])
		}
	})

	t.Run("maxDepth query parameter", func(t *testing.T) {
		node.TrustRegistryMutex.Lock()
		node.TrustRegistry["0000000000000001"]["000000000000001b"] = 0.9
		if node.TrustRegistry["000000000000001b"] == nil {
			node.TrustRegistry["000000000000001b"] = make(map[string]float64)
		}
		node.TrustRegistry["000000000000001b"]["000000000000001c"] = 0.9
		node.TrustRegistryMutex.Unlock()

		req := httptest.NewRequest("GET", "/api/trust/0000000000000001/000000000000001c?maxDepth=1", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("Expected status 200, got %d", w.Code)
		}

		var response map[string]interface{}
		json.Unmarshal(w.Body.Bytes(), &response)

		data := response["data"].(map[string]interface{})
		if data["trustLevel"] != 0.0 {
			t.Errorf("Expected trustLevel 0.0 with maxDepth=1 (target is 2 hops away), got '%v'", data["trustLevel"])
		}

		req2 := httptest.NewRequest("GET", "/api/trust/0000000000000001/000000000000001c?maxDepth=3", nil)
		w2 := httptest.NewRecorder()
		router.ServeHTTP(w2, req2)

		var response2 map[string]interface{}
		json.Unmarshal(w2.Body.Bytes(), &response2)

		data2 := response2["data"].(map[string]interface{})
		if data2["trustLevel"] == 0.0 {
			t.Error("Expected non-zero trustLevel with maxDepth=3")
		}
	})
}

func TestGetIdentityHandler(t *testing.T) {
	node := newTestNode()
	router := setupTestRouter(node)

	t.Run("existing identity", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/identity/0000000000000001", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("Expected status 200, got %d", w.Code)
		}

		var response map[string]interface{}
		if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
			t.Fatalf("Failed to parse response: %v", err)
		}

		if response["success"] != true {
			t.Errorf("Expected success true, got '%v'", response["success"])
		}

		data, ok := response["data"].(map[string]interface{})
		if !ok {
			t.Fatal("Expected data to be a map")
		}

		if data["quidId"] != "0000000000000001" {
			t.Errorf("Expected quidId '0000000000000001', got '%v'", data["quidId"])
		}

		if data["name"] != "Test Truster" {
			t.Errorf("Expected name 'Test Truster', got '%v'", data["name"])
		}
	})

	t.Run("non-existing identity", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/identity/000000000000001f", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		if w.Code != http.StatusNotFound {
			t.Errorf("Expected status 404, got %d", w.Code)
		}

		var response map[string]interface{}
		json.Unmarshal(w.Body.Bytes(), &response)

		if response["success"] != false {
			t.Errorf("Expected success false, got '%v'", response["success"])
		}

		errData, ok := response["error"].(map[string]interface{})
		if !ok {
			t.Fatal("Expected error to be a map")
		}

		if errData["code"] != "NOT_FOUND" {
			t.Errorf("Expected error code 'NOT_FOUND', got '%v'", errData["code"])
		}
	})

	t.Run("identity with domain query param", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/identity/0000000000000001?domain=test.domain.com", nil)
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
		req := httptest.NewRequest("GET", "/api/title/0000000000000003", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("Expected status 200, got %d", w.Code)
		}

		var response map[string]interface{}
		if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
			t.Fatalf("Failed to parse response: %v", err)
		}

		if response["success"] != true {
			t.Errorf("Expected success true, got '%v'", response["success"])
		}

		data, ok := response["data"].(map[string]interface{})
		if !ok {
			t.Fatal("Expected data to be a map")
		}

		if data["assetId"] != "0000000000000003" {
			t.Errorf("Expected assetId '0000000000000003', got '%v'", data["assetId"])
		}

		owners, ok := data["owners"].([]interface{})
		if !ok {
			t.Error("Expected owners to be an array")
		} else if len(owners) != 2 {
			t.Errorf("Expected 2 owners, got %d", len(owners))
		}
	})

	t.Run("non-existing title", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/title/0000000000000020", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		if w.Code != http.StatusNotFound {
			t.Errorf("Expected status 404, got %d", w.Code)
		}

		var response map[string]interface{}
		json.Unmarshal(w.Body.Bytes(), &response)

		if response["success"] != false {
			t.Errorf("Expected success false, got '%v'", response["success"])
		}
	})

	t.Run("title with domain query param", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/title/0000000000000003?domain=test.domain.com", nil)
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

	if response["success"] != true {
		t.Errorf("Expected success true, got '%v'", response["success"])
	}

	data, ok := response["data"].(map[string]interface{})
	if !ok {
		t.Fatal("Expected data to be a map")
	}

	if _, ok := data["data"]; !ok {
		t.Error("Expected 'data' key in data")
	}

	if _, ok := data["pagination"]; !ok {
		t.Error("Expected 'pagination' key in data")
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

	if response["success"] != true {
		t.Errorf("Expected success true, got '%v'", response["success"])
	}

	responseData, ok := response["data"].(map[string]interface{})
	if !ok {
		t.Fatal("Expected response data to be a map")
	}

	data, ok := responseData["data"].([]interface{})
	if !ok {
		t.Error("Expected 'data' to be an array")
	}

	if len(data) < 1 {
		t.Error("Expected at least genesis block")
	}

	if _, ok := responseData["pagination"]; !ok {
		t.Error("Expected 'pagination' key in response data")
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

	if response["success"] != true {
		t.Errorf("Expected success true, got '%v'", response["success"])
	}

	data, ok := response["data"].(map[string]interface{})
	if !ok {
		t.Fatal("Expected data to be a map")
	}

	if _, ok := data["domains"]; !ok {
		t.Error("Expected 'domains' key in data")
	}
}

func TestGetTrustHandler_IncludeUnverified(t *testing.T) {
	node := newTestNode()
	router := setupTestRouter(node)

	// Set up verified trust edges: A -> B
	node.AddVerifiedTrustEdge(TrustEdge{
		Truster:       "000000000000000b",
		Trustee:       "000000000000000c",
		TrustLevel:    0.8,
		SourceBlock:   "block123",
		ValidatorQuid: node.NodeID,
		Verified:      true,
		Timestamp:     1000000,
	})

	t.Run("includeUnverified=true returns enhanced result", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/trust/000000000000000b/000000000000000c?includeUnverified=true", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("Expected status 200, got %d", w.Code)
		}

		var response map[string]interface{}
		if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
			t.Fatalf("Failed to parse response: %v", err)
		}

		if response["success"] != true {
			t.Errorf("Expected success true, got '%v'", response["success"])
		}

		data, ok := response["data"].(map[string]interface{})
		if !ok {
			t.Fatal("Expected data to be a map")
		}

		// Check for EnhancedTrustResult fields
		if _, ok := data["confidence"]; !ok {
			t.Error("Expected 'confidence' field in enhanced result")
		}

		if _, ok := data["unverifiedHops"]; !ok {
			t.Error("Expected 'unverifiedHops' field in enhanced result")
		}

		if _, ok := data["verificationGaps"]; !ok {
			t.Error("Expected 'verificationGaps' field in enhanced result")
		}

		if data["confidence"] != "high" {
			t.Errorf("Expected confidence 'high' for verified path, got '%v'", data["confidence"])
		}

		if data["trustLevel"] != 0.8 {
			t.Errorf("Expected trustLevel 0.8, got '%v'", data["trustLevel"])
		}
	})

	t.Run("includeUnverified=false returns standard result", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/trust/000000000000000b/000000000000000c?includeUnverified=false", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("Expected status 200, got %d", w.Code)
		}

		var response map[string]interface{}
		json.Unmarshal(w.Body.Bytes(), &response)

		data := response["data"].(map[string]interface{})

		// Standard result should NOT have confidence field
		if _, ok := data["confidence"]; ok {
			t.Error("Standard result should not have 'confidence' field")
		}
	})
}

func TestRelationalTrustQueryHandler_IncludeUnverified(t *testing.T) {
	node := newTestNode()
	router := setupTestRouter(node)

	// Set up verified trust edges
	node.AddVerifiedTrustEdge(TrustEdge{
		Truster:       "000000000000000d",
		Trustee:       "000000000000000e",
		TrustLevel:    0.9,
		SourceBlock:   "block456",
		ValidatorQuid: node.NodeID,
		Verified:      true,
		Timestamp:     1000000,
	})

	t.Run("includeUnverified true in request body", func(t *testing.T) {
		body := bytes.NewBufferString(`{"observer":"000000000000000d","target":"000000000000000e","includeUnverified":true}`)
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

		if response["success"] != true {
			t.Errorf("Expected success true, got '%v'", response["success"])
		}

		data, ok := response["data"].(map[string]interface{})
		if !ok {
			t.Fatal("Expected data to be a map")
		}

		// Check for EnhancedTrustResult fields
		if _, ok := data["confidence"]; !ok {
			t.Error("Expected 'confidence' field in enhanced result")
		}

		if _, ok := data["unverifiedHops"]; !ok {
			t.Error("Expected 'unverifiedHops' field in enhanced result")
		}

		if data["trustLevel"] != 0.9 {
			t.Errorf("Expected trustLevel 0.9, got '%v'", data["trustLevel"])
		}
	})

	t.Run("includeUnverified false returns standard result", func(t *testing.T) {
		body := bytes.NewBufferString(`{"observer":"000000000000000d","target":"000000000000000e","includeUnverified":false}`)
		req := httptest.NewRequest("POST", "/api/trust/query", body)
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("Expected status 200, got %d", w.Code)
		}

		var response map[string]interface{}
		json.Unmarshal(w.Body.Bytes(), &response)

		data := response["data"].(map[string]interface{})

		// Standard result should NOT have confidence field
		if _, ok := data["confidence"]; ok {
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

		if response["success"] != true {
			t.Errorf("Expected success true, got '%v'", response["success"])
		}

		data := response["data"].(map[string]interface{})
		if data["domain"] != "nonexistent" {
			t.Errorf("Expected domain 'nonexistent', got '%v'", data["domain"])
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

		if response["success"] != true {
			t.Errorf("Expected success true, got '%v'", response["success"])
		}

		data := response["data"].(map[string]interface{})
		if data["domain"] != "testdomain" {
			t.Errorf("Expected domain 'testdomain', got '%v'", data["domain"])
		}

		blocks, ok := data["blocks"].([]interface{})
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
		Truster:       "000000000000000f",
		Trustee:       "0000000000000010",
		TrustLevel:    0.85,
		SourceBlock:   "block789",
		ValidatorQuid: node.NodeID,
		Verified:      true,
		Timestamp:     1000000,
	})

	node.AddVerifiedTrustEdge(TrustEdge{
		Truster:       "000000000000000f",
		Trustee:       "0000000000000011",
		TrustLevel:    0.7,
		SourceBlock:   "block790",
		ValidatorQuid: node.NodeID,
		Verified:      true,
		Timestamp:     1000001,
	})

	t.Run("returns verified edges with provenance", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/trust/edges/000000000000000f?includeUnverified=false", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("Expected status 200, got %d", w.Code)
		}

		var response map[string]interface{}
		if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
			t.Fatalf("Failed to parse response: %v", err)
		}

		if response["success"] != true {
			t.Errorf("Expected success true, got '%v'", response["success"])
		}

		data := response["data"].(map[string]interface{})

		if data["quidId"] != "000000000000000f" {
			t.Errorf("Expected quidId '000000000000000f', got '%v'", data["quidId"])
		}

		if data["includeUnverified"] != false {
			t.Errorf("Expected includeUnverified false, got '%v'", data["includeUnverified"])
		}

		edges, ok := data["edges"].(map[string]interface{})
		if !ok {
			t.Error("Expected 'edges' to be a map")
		} else if len(edges) != 2 {
			t.Errorf("Expected 2 edges, got %d", len(edges))
		}

		// Check edge provenance fields
		if edge1, ok := edges["0000000000000010"].(map[string]interface{}); ok {
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
			t.Error("Expected edge to 0000000000000010 to exist")
		}
	})

	t.Run("includeUnverified returns both verified and unverified", func(t *testing.T) {
		// Add an unverified edge
		node.AddUnverifiedTrustEdge(TrustEdge{
			Truster:       "000000000000000f",
			Trustee:       "0000000000000012",
			TrustLevel:    0.5,
			SourceBlock:   "block999",
			ValidatorQuid: "0000000000000013",
			Verified:      false,
			Timestamp:     1000002,
		})

		req := httptest.NewRequest("GET", "/api/trust/edges/000000000000000f?includeUnverified=true", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("Expected status 200, got %d", w.Code)
		}

		var response map[string]interface{}
		json.Unmarshal(w.Body.Bytes(), &response)

		data := response["data"].(map[string]interface{})

		if data["includeUnverified"] != true {
			t.Errorf("Expected includeUnverified true, got '%v'", data["includeUnverified"])
		}

		edges, ok := data["edges"].(map[string]interface{})
		if !ok {
			t.Error("Expected 'edges' to be a map")
		} else if len(edges) != 3 {
			t.Errorf("Expected 3 edges (2 verified + 1 unverified), got %d", len(edges))
		}

		// Check unverified edge
		if unverifiedEdge, ok := edges["0000000000000012"].(map[string]interface{}); ok {
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

		data := response["data"].(map[string]interface{})
		edges, ok := data["edges"].(map[string]interface{})
		if !ok {
			t.Error("Expected 'edges' to be a map")
		} else if len(edges) != 0 {
			t.Errorf("Expected 0 edges for nonexistent quid, got %d", len(edges))
		}
	})
}

func TestParsePaginationParams(t *testing.T) {
	t.Run("default values when no params", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/test", nil)
		params := ParsePaginationParams(req, 50, 1000)

		if params.Limit != 50 {
			t.Errorf("Expected default limit 50, got %d", params.Limit)
		}
		if params.Offset != 0 {
			t.Errorf("Expected default offset 0, got %d", params.Offset)
		}
	})

	t.Run("custom limit and offset", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/test?limit=25&offset=100", nil)
		params := ParsePaginationParams(req, 50, 1000)

		if params.Limit != 25 {
			t.Errorf("Expected limit 25, got %d", params.Limit)
		}
		if params.Offset != 100 {
			t.Errorf("Expected offset 100, got %d", params.Offset)
		}
	})

	t.Run("limit capped to maxLimit", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/test?limit=5000", nil)
		params := ParsePaginationParams(req, 50, 1000)

		if params.Limit != 1000 {
			t.Errorf("Expected limit capped to 1000, got %d", params.Limit)
		}
	})

	t.Run("negative limit uses default", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/test?limit=-10", nil)
		params := ParsePaginationParams(req, 50, 1000)

		if params.Limit != 50 {
			t.Errorf("Expected default limit 50 for negative input, got %d", params.Limit)
		}
	})

	t.Run("negative offset uses default", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/test?offset=-5", nil)
		params := ParsePaginationParams(req, 50, 1000)

		if params.Offset != 0 {
			t.Errorf("Expected default offset 0 for negative input, got %d", params.Offset)
		}
	})

	t.Run("invalid limit string uses default", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/test?limit=abc", nil)
		params := ParsePaginationParams(req, 50, 1000)

		if params.Limit != 50 {
			t.Errorf("Expected default limit 50 for invalid string, got %d", params.Limit)
		}
	})

	t.Run("zero limit uses default", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/test?limit=0", nil)
		params := ParsePaginationParams(req, 50, 1000)

		if params.Limit != 50 {
			t.Errorf("Expected default limit 50 for zero, got %d", params.Limit)
		}
	})
}

func TestGetBlocksHandlerPagination(t *testing.T) {
	node := newTestNode()
	router := setupTestRouter(node)

	t.Run("default pagination", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/blocks", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("Expected status 200, got %d", w.Code)
		}

		var response map[string]interface{}
		json.Unmarshal(w.Body.Bytes(), &response)

		data := response["data"].(map[string]interface{})

		if _, ok := data["data"]; !ok {
			t.Error("Expected 'data' key in response data")
		}

		pagination, ok := data["pagination"].(map[string]interface{})
		if !ok {
			t.Error("Expected 'pagination' key in response data")
		} else {
			if pagination["limit"].(float64) != 50 {
				t.Errorf("Expected limit 50, got %v", pagination["limit"])
			}
			if pagination["offset"].(float64) != 0 {
				t.Errorf("Expected offset 0, got %v", pagination["offset"])
			}
		}
	})

	t.Run("custom pagination params", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/blocks?limit=10&offset=0", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		var response map[string]interface{}
		json.Unmarshal(w.Body.Bytes(), &response)

		data := response["data"].(map[string]interface{})
		pagination := data["pagination"].(map[string]interface{})
		if pagination["limit"].(float64) != 10 {
			t.Errorf("Expected limit 10, got %v", pagination["limit"])
		}
	})
}

func TestGetNodesHandlerPagination(t *testing.T) {
	node := newTestNode()
	router := setupTestRouter(node)

	req := httptest.NewRequest("GET", "/api/nodes?limit=5", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	var response map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &response)

	data := response["data"].(map[string]interface{})

	if _, ok := data["data"]; !ok {
		t.Error("Expected 'data' key in response data")
	}

	pagination, ok := data["pagination"].(map[string]interface{})
	if !ok {
		t.Error("Expected 'pagination' key in response data")
	} else {
		if pagination["limit"].(float64) != 5 {
			t.Errorf("Expected limit 5, got %v", pagination["limit"])
		}
	}
}

func TestGetTransactionsHandlerPagination(t *testing.T) {
	node := newTestNode()
	router := mux.NewRouter()
	router.HandleFunc("/api/transactions", node.GetTransactionsHandler).Methods("GET")

	req := httptest.NewRequest("GET", "/api/transactions?limit=20&offset=5", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	var response map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &response)

	data := response["data"].(map[string]interface{})

	if _, ok := data["data"]; !ok {
		t.Error("Expected 'data' key in response data")
	}

	pagination := data["pagination"].(map[string]interface{})
	if pagination["limit"].(float64) != 20 {
		t.Errorf("Expected limit 20, got %v", pagination["limit"])
	}
	if pagination["offset"].(float64) != 5 {
		t.Errorf("Expected offset 5, got %v", pagination["offset"])
	}
}

func TestQueryTrustRegistryHandlerPagination(t *testing.T) {
	node := newTestNode()
	router := mux.NewRouter()
	router.HandleFunc("/api/registry/trust", node.QueryTrustRegistryHandler).Methods("GET")

	node.TrustRegistryMutex.Lock()
	for i := 0; i < 100; i++ {
		truster := "truster_" + strconv.Itoa(i)
		if node.TrustRegistry[truster] == nil {
			node.TrustRegistry[truster] = make(map[string]float64)
		}
		node.TrustRegistry[truster]["trustee_"+strconv.Itoa(i)] = 0.5
	}
	node.TrustRegistryMutex.Unlock()

	req := httptest.NewRequest("GET", "/api/registry/trust?limit=10&offset=5", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	var response map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &response)

	responseData := response["data"].(map[string]interface{})
	data, ok := responseData["data"].([]interface{})
	if !ok {
		t.Error("Expected 'data' to be an array")
	} else if len(data) > 10 {
		t.Errorf("Expected at most 10 items, got %d", len(data))
	}

	pagination := responseData["pagination"].(map[string]interface{})
	if pagination["total"].(float64) < 100 {
		t.Errorf("Expected total >= 100, got %v", pagination["total"])
	}
}

func TestQueryIdentityRegistryHandlerPagination(t *testing.T) {
	node := newTestNode()
	router := mux.NewRouter()
	router.HandleFunc("/api/registry/identity", node.QueryIdentityRegistryHandler).Methods("GET")

	req := httptest.NewRequest("GET", "/api/registry/identity?limit=10", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	var response map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &response)

	data := response["data"].(map[string]interface{})

	if _, ok := data["data"]; !ok {
		t.Error("Expected 'data' key in response data")
	}

	if _, ok := data["pagination"]; !ok {
		t.Error("Expected 'pagination' key in response data")
	}
}

func TestQueryTitleRegistryHandlerPagination(t *testing.T) {
	node := newTestNode()
	router := mux.NewRouter()
	router.HandleFunc("/api/registry/title", node.QueryTitleRegistryHandler).Methods("GET")

	req := httptest.NewRequest("GET", "/api/registry/title?limit=10", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	var response map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &response)

	data := response["data"].(map[string]interface{})

	if _, ok := data["data"]; !ok {
		t.Error("Expected 'data' key in response data")
	}

	if _, ok := data["pagination"]; !ok {
		t.Error("Expected 'pagination' key in response data")
	}
}

func TestPaginationLimitCapped(t *testing.T) {
	node := newTestNode()
	router := setupTestRouter(node)

	req := httptest.NewRequest("GET", "/api/blocks?limit=5000", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	var response map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &response)

	data := response["data"].(map[string]interface{})
	pagination := data["pagination"].(map[string]interface{})
	if pagination["limit"].(float64) != 1000 {
		t.Errorf("Expected limit capped to 1000, got %v", pagination["limit"])
	}
}

func TestVersionedRoutes(t *testing.T) {
	node := newTestNode()
	router := mux.NewRouter()

	// Register versioned routes
	v1Router := router.PathPrefix("/api/v1").Subrouter()
	node.registerAPIRoutes(v1Router)

	// Register backward-compatible routes
	apiRouter := router.PathPrefix("/api").Subrouter()
	node.registerAPIRoutes(apiRouter)

	t.Run("v1 health endpoint works", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/v1/health", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("Expected status 200, got %d", w.Code)
		}

		var response map[string]interface{}
		json.Unmarshal(w.Body.Bytes(), &response)

		if response["success"] != true {
			t.Errorf("Expected success true, got '%v'", response["success"])
		}

		// Check X-API-Version header
		if w.Header().Get("X-API-Version") != "1.0" {
			t.Errorf("Expected X-API-Version '1.0', got '%s'", w.Header().Get("X-API-Version"))
		}
	})

	t.Run("backward compatible /api endpoint works", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/health", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("Expected status 200, got %d", w.Code)
		}

		var response map[string]interface{}
		json.Unmarshal(w.Body.Bytes(), &response)

		if response["success"] != true {
			t.Errorf("Expected success true, got '%v'", response["success"])
		}
	})
}

func TestRelationalTrustQueryHandler(t *testing.T) {
	node := newTestNode()
	router := setupTestRouter(node)

	node.TrustRegistryMutex.Lock()
	if node.TrustRegistry["0000000000000014"] == nil {
		node.TrustRegistry["0000000000000014"] = make(map[string]float64)
	}
	node.TrustRegistry["0000000000000014"]["0000000000000015"] = 0.75
	node.TrustRegistryMutex.Unlock()

	t.Run("valid query with observer and target", func(t *testing.T) {
		body := bytes.NewBufferString(`{"observer":"0000000000000014","target":"0000000000000015","domain":"test.domain"}`)
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

		if response["success"] != true {
			t.Errorf("Expected success true, got '%v'", response["success"])
		}

		data := response["data"].(map[string]interface{})

		if data["observer"] != "0000000000000014" {
			t.Errorf("Expected observer '0000000000000014', got '%v'", data["observer"])
		}

		if data["target"] != "0000000000000015" {
			t.Errorf("Expected target '0000000000000015', got '%v'", data["target"])
		}

		if data["trustLevel"] != 0.75 {
			t.Errorf("Expected trustLevel 0.75, got '%v'", data["trustLevel"])
		}

		if data["domain"] != "test.domain" {
			t.Errorf("Expected domain 'test.domain', got '%v'", data["domain"])
		}

		trustPath, ok := data["trustPath"].([]interface{})
		if !ok {
			t.Error("Expected trustPath to be an array")
		} else if len(trustPath) != 2 {
			t.Errorf("Expected trustPath length 2, got %d", len(trustPath))
		}

		pathDepth, ok := data["pathDepth"].(float64)
		if !ok {
			t.Error("Expected pathDepth to be a number")
		} else if int(pathDepth) != 1 {
			t.Errorf("Expected pathDepth 1, got %v", pathDepth)
		}
	})

	t.Run("missing observer", func(t *testing.T) {
		body := bytes.NewBufferString(`{"target":"0000000000000015","domain":"test.domain"}`)
		req := httptest.NewRequest("POST", "/api/trust/query", body)
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		if w.Code != http.StatusBadRequest {
			t.Errorf("Expected status 400, got %d", w.Code)
		}

		var response map[string]interface{}
		json.Unmarshal(w.Body.Bytes(), &response)

		if response["success"] != false {
			t.Errorf("Expected success false, got '%v'", response["success"])
		}

		errData := response["error"].(map[string]interface{})
		if errData["code"] != "MISSING_PARAMETERS" {
			t.Errorf("Expected error code 'MISSING_PARAMETERS', got '%v'", errData["code"])
		}
	})

	t.Run("missing target", func(t *testing.T) {
		body := bytes.NewBufferString(`{"observer":"0000000000000014","domain":"test.domain"}`)
		req := httptest.NewRequest("POST", "/api/trust/query", body)
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		if w.Code != http.StatusBadRequest {
			t.Errorf("Expected status 400, got %d", w.Code)
		}

		var response map[string]interface{}
		json.Unmarshal(w.Body.Bytes(), &response)

		if response["success"] != false {
			t.Errorf("Expected success false, got '%v'", response["success"])
		}
	})

	t.Run("query with maxDepth parameter", func(t *testing.T) {
		node.TrustRegistryMutex.Lock()
		if node.TrustRegistry["0000000000000016"] == nil {
			node.TrustRegistry["0000000000000016"] = make(map[string]float64)
		}
		node.TrustRegistry["0000000000000016"]["0000000000000017"] = 0.8
		if node.TrustRegistry["0000000000000017"] == nil {
			node.TrustRegistry["0000000000000017"] = make(map[string]float64)
		}
		node.TrustRegistry["0000000000000017"]["0000000000000018"] = 0.8
		node.TrustRegistryMutex.Unlock()

		body := bytes.NewBufferString(`{"observer":"0000000000000016","target":"0000000000000018","maxDepth":1}`)
		req := httptest.NewRequest("POST", "/api/trust/query", body)
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("Expected status 200, got %d", w.Code)
		}

		var response map[string]interface{}
		json.Unmarshal(w.Body.Bytes(), &response)

		data := response["data"].(map[string]interface{})
		if data["trustLevel"] != 0.0 {
			t.Errorf("Expected trustLevel 0.0 with maxDepth=1 (target is 2 hops away), got '%v'", data["trustLevel"])
		}

		body2 := bytes.NewBufferString(`{"observer":"0000000000000016","target":"0000000000000018","maxDepth":3}`)
		req2 := httptest.NewRequest("POST", "/api/trust/query", body2)
		req2.Header.Set("Content-Type", "application/json")
		w2 := httptest.NewRecorder()
		router.ServeHTTP(w2, req2)

		var response2 map[string]interface{}
		json.Unmarshal(w2.Body.Bytes(), &response2)

		data2 := response2["data"].(map[string]interface{})
		expected := 0.8 * 0.8
		if data2["trustLevel"] != expected {
			t.Errorf("Expected trustLevel %f with maxDepth=3, got '%v'", expected, data2["trustLevel"])
		}
	})

	t.Run("query that returns no path", func(t *testing.T) {
		body := bytes.NewBufferString(`{"observer":"0000000000000019","target":"000000000000001a","domain":"test.domain"}`)
		req := httptest.NewRequest("POST", "/api/trust/query", body)
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("Expected status 200, got %d", w.Code)
		}

		var response map[string]interface{}
		json.Unmarshal(w.Body.Bytes(), &response)

		data := response["data"].(map[string]interface{})

		if data["trustLevel"] != 0.0 {
			t.Errorf("Expected trustLevel 0.0 for no path, got '%v'", data["trustLevel"])
		}

		pathDepth, ok := data["pathDepth"].(float64)
		if !ok || int(pathDepth) != 0 {
			t.Errorf("Expected pathDepth 0, got '%v'", data["pathDepth"])
		}
	})
}
