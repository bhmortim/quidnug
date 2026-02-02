package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

func TestBroadcastTransaction(t *testing.T) {
	node := newTestNode()

	t.Run("broadcasts trust transaction to known nodes", func(t *testing.T) {
		var receivedCount int32
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == "/api/transactions/trust" && r.Method == "POST" {
				atomic.AddInt32(&receivedCount, 1)
				w.WriteHeader(http.StatusCreated)
				json.NewEncoder(w).Encode(map[string]string{"status": "success"})
			} else {
				w.WriteHeader(http.StatusNotFound)
			}
		}))
		defer server.Close()

		node.KnownNodesMutex.Lock()
		node.KnownNodes["test_node_1"] = Node{
			ID:           "test_node_1",
			Address:      server.Listener.Addr().String(),
			TrustDomains: []string{"test.domain.com"},
		}
		node.KnownNodesMutex.Unlock()

		node.TrustDomainsMutex.Lock()
		node.TrustDomains["test.domain.com"] = TrustDomain{
			Name:           "test.domain.com",
			ValidatorNodes: []string{"test_node_1"},
		}
		node.TrustDomainsMutex.Unlock()

		tx := TrustTransaction{
			BaseTransaction: BaseTransaction{
				ID:          "tx_broadcast_test",
				Type:        TxTypeTrust,
				TrustDomain: "test.domain.com",
				Timestamp:   time.Now().Unix(),
			},
			Truster:    "quid_truster_001",
			Trustee:    "quid_trustee_001",
			TrustLevel: 0.8,
		}

		node.BroadcastTransaction(tx)

		time.Sleep(100 * time.Millisecond)

		if atomic.LoadInt32(&receivedCount) != 1 {
			t.Errorf("Expected 1 broadcast, got %d", receivedCount)
		}
	})

	t.Run("broadcasts identity transaction", func(t *testing.T) {
		var receivedPath string
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			receivedPath = r.URL.Path
			w.WriteHeader(http.StatusCreated)
			json.NewEncoder(w).Encode(map[string]string{"status": "success"})
		}))
		defer server.Close()

		node.KnownNodesMutex.Lock()
		node.KnownNodes["test_node_2"] = Node{
			ID:           "test_node_2",
			Address:      server.Listener.Addr().String(),
			TrustDomains: []string{"identity.domain.com"},
		}
		node.KnownNodesMutex.Unlock()

		node.TrustDomainsMutex.Lock()
		node.TrustDomains["identity.domain.com"] = TrustDomain{
			Name:           "identity.domain.com",
			ValidatorNodes: []string{"test_node_2"},
		}
		node.TrustDomainsMutex.Unlock()

		tx := IdentityTransaction{
			BaseTransaction: BaseTransaction{
				ID:          "tx_identity_broadcast",
				Type:        TxTypeIdentity,
				TrustDomain: "identity.domain.com",
				Timestamp:   time.Now().Unix(),
			},
			QuidID: "quid_new",
			Name:   "Test Identity",
		}

		node.BroadcastTransaction(tx)

		time.Sleep(100 * time.Millisecond)

		if receivedPath != "/api/transactions/identity" {
			t.Errorf("Expected path '/api/transactions/identity', got '%s'", receivedPath)
		}
	})

	t.Run("broadcasts title transaction", func(t *testing.T) {
		var receivedPath string
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			receivedPath = r.URL.Path
			w.WriteHeader(http.StatusCreated)
			json.NewEncoder(w).Encode(map[string]string{"status": "success"})
		}))
		defer server.Close()

		node.KnownNodesMutex.Lock()
		node.KnownNodes["test_node_3"] = Node{
			ID:           "test_node_3",
			Address:      server.Listener.Addr().String(),
			TrustDomains: []string{"title.domain.com"},
		}
		node.KnownNodesMutex.Unlock()

		node.TrustDomainsMutex.Lock()
		node.TrustDomains["title.domain.com"] = TrustDomain{
			Name:           "title.domain.com",
			ValidatorNodes: []string{"test_node_3"},
		}
		node.TrustDomainsMutex.Unlock()

		tx := TitleTransaction{
			BaseTransaction: BaseTransaction{
				ID:          "tx_title_broadcast",
				Type:        TxTypeTitle,
				TrustDomain: "title.domain.com",
				Timestamp:   time.Now().Unix(),
			},
			AssetID: "asset_001",
			Owners:  []OwnershipStake{{OwnerID: "owner1", Percentage: 100.0}},
		}

		node.BroadcastTransaction(tx)

		time.Sleep(100 * time.Millisecond)

		if receivedPath != "/api/transactions/title" {
			t.Errorf("Expected path '/api/transactions/title', got '%s'", receivedPath)
		}
	})

	t.Run("skips self when broadcasting", func(t *testing.T) {
		var receivedCount int32
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			atomic.AddInt32(&receivedCount, 1)
			w.WriteHeader(http.StatusCreated)
		}))
		defer server.Close()

		node.KnownNodesMutex.Lock()
		node.KnownNodes[node.NodeID] = Node{
			ID:           node.NodeID,
			Address:      server.Listener.Addr().String(),
			TrustDomains: []string{"self.domain.com"},
		}
		node.KnownNodesMutex.Unlock()

		node.TrustDomainsMutex.Lock()
		node.TrustDomains["self.domain.com"] = TrustDomain{
			Name:           "self.domain.com",
			ValidatorNodes: []string{node.NodeID},
		}
		node.TrustDomainsMutex.Unlock()

		tx := TrustTransaction{
			BaseTransaction: BaseTransaction{
				TrustDomain: "self.domain.com",
			},
		}

		node.BroadcastTransaction(tx)

		time.Sleep(100 * time.Millisecond)

		if atomic.LoadInt32(&receivedCount) != 0 {
			t.Errorf("Expected 0 broadcasts to self, got %d", receivedCount)
		}
	})

	t.Run("handles unreachable node gracefully", func(t *testing.T) {
		node.KnownNodesMutex.Lock()
		node.KnownNodes["unreachable_node"] = Node{
			ID:           "unreachable_node",
			Address:      "127.0.0.1:59999",
			TrustDomains: []string{"unreachable.domain.com"},
		}
		node.KnownNodesMutex.Unlock()

		node.TrustDomainsMutex.Lock()
		node.TrustDomains["unreachable.domain.com"] = TrustDomain{
			Name:           "unreachable.domain.com",
			ValidatorNodes: []string{"unreachable_node"},
		}
		node.TrustDomainsMutex.Unlock()

		tx := TrustTransaction{
			BaseTransaction: BaseTransaction{
				TrustDomain: "unreachable.domain.com",
			},
		}

		node.BroadcastTransaction(tx)

		time.Sleep(100 * time.Millisecond)
	})
}

func TestQueryOtherDomain(t *testing.T) {
	node := newTestNode()

	t.Run("queries remote node successfully", func(t *testing.T) {
		expectedResponse := map[string]interface{}{
			"quidId": "test_quid",
			"name":   "Test Name",
		}

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == "/api/domains/remote.domain.com/query" {
				queryType := r.URL.Query().Get("type")
				param := r.URL.Query().Get("param")
				if queryType == "identity" && param == "test_quid" {
					w.Header().Set("Content-Type", "application/json")
					json.NewEncoder(w).Encode(expectedResponse)
					return
				}
			}
			w.WriteHeader(http.StatusNotFound)
		}))
		defer server.Close()

		node.KnownNodesMutex.Lock()
		node.KnownNodes["remote_node"] = Node{
			ID:           "remote_node",
			Address:      server.Listener.Addr().String(),
			TrustDomains: []string{"remote.domain.com"},
		}
		node.KnownNodesMutex.Unlock()

		result, err := node.QueryOtherDomain("remote.domain.com", "identity", "test_quid")
		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}

		resultMap, ok := result.(map[string]interface{})
		if !ok {
			t.Fatalf("Expected map result, got %T", result)
		}

		if resultMap["quidId"] != "test_quid" {
			t.Errorf("Expected quidId 'test_quid', got '%v'", resultMap["quidId"])
		}
	})

	t.Run("returns error when no nodes manage domain", func(t *testing.T) {
		_, err := node.QueryOtherDomain("nonexistent.domain.com", "identity", "test")
		if err == nil {
			t.Error("Expected error for unknown domain, got nil")
		}
	})

	t.Run("tries next node on failure", func(t *testing.T) {
		var callCount int32

		failingServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			atomic.AddInt32(&callCount, 1)
			w.WriteHeader(http.StatusInternalServerError)
		}))
		defer failingServer.Close()

		workingServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			atomic.AddInt32(&callCount, 1)
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
		}))
		defer workingServer.Close()

		node.KnownNodesMutex.Lock()
		node.KnownNodes["failing_node"] = Node{
			ID:           "failing_node",
			Address:      failingServer.Listener.Addr().String(),
			TrustDomains: []string{"failover.domain.com"},
		}
		node.KnownNodes["working_node"] = Node{
			ID:           "working_node",
			Address:      workingServer.Listener.Addr().String(),
			TrustDomains: []string{"failover.domain.com"},
		}
		node.KnownNodesMutex.Unlock()

		result, err := node.QueryOtherDomain("failover.domain.com", "identity", "test")
		if err != nil {
			t.Fatalf("Expected success after failover, got error: %v", err)
		}

		if result == nil {
			t.Error("Expected non-nil result")
		}

		if atomic.LoadInt32(&callCount) < 2 {
			t.Errorf("Expected at least 2 calls (failover), got %d", callCount)
		}
	})

	t.Run("returns error when all nodes fail", func(t *testing.T) {
		failingServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
		}))
		defer failingServer.Close()

		node.KnownNodesMutex.Lock()
		node.KnownNodes["all_fail_node"] = Node{
			ID:           "all_fail_node",
			Address:      failingServer.Listener.Addr().String(),
			TrustDomains: []string{"allfail.domain.com"},
		}
		node.KnownNodesMutex.Unlock()

		_, err := node.QueryOtherDomain("allfail.domain.com", "identity", "test")
		if err == nil {
			t.Error("Expected error when all nodes fail, got nil")
		}
	})
}

func TestDomainHierarchyWalking(t *testing.T) {
	node := newTestNode()

	t.Run("walks up domain hierarchy to find parent", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]string{"found": "via_parent"})
		}))
		defer server.Close()

		node.KnownNodesMutex.Lock()
		node.KnownNodes["parent_domain_node"] = Node{
			ID:           "parent_domain_node",
			Address:      server.Listener.Addr().String(),
			TrustDomains: []string{"domain.com"},
		}
		node.KnownNodesMutex.Unlock()

		result, err := node.QueryOtherDomain("sub.domain.com", "identity", "test")
		if err != nil {
			t.Fatalf("Expected to find parent domain node, got error: %v", err)
		}

		resultMap, ok := result.(map[string]interface{})
		if !ok {
			t.Fatalf("Expected map result, got %T", result)
		}

		if resultMap["found"] != "via_parent" {
			t.Errorf("Expected 'via_parent', got '%v'", resultMap["found"])
		}
	})

	t.Run("walks multiple levels up hierarchy", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]string{"found": "root"})
		}))
		defer server.Close()

		node.KnownNodesMutex.Lock()
		node.KnownNodes["root_node"] = Node{
			ID:           "root_node",
			Address:      server.Listener.Addr().String(),
			TrustDomains: []string{"com"},
		}
		node.KnownNodesMutex.Unlock()

		result, err := node.QueryOtherDomain("deep.sub.domain.com", "identity", "test")
		if err != nil {
			t.Fatalf("Expected to find root domain node, got error: %v", err)
		}

		resultMap, ok := result.(map[string]interface{})
		if !ok {
			t.Fatalf("Expected map result, got %T", result)
		}

		if resultMap["found"] != "root" {
			t.Errorf("Expected 'root', got '%v'", resultMap["found"])
		}
	})

	t.Run("prefers exact match over parent", func(t *testing.T) {
		parentServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]string{"found": "parent"})
		}))
		defer parentServer.Close()

		exactServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]string{"found": "exact"})
		}))
		defer exactServer.Close()

		node.KnownNodesMutex.Lock()
		node.KnownNodes["parent_node_pref"] = Node{
			ID:           "parent_node_pref",
			Address:      parentServer.Listener.Addr().String(),
			TrustDomains: []string{"prefer.com"},
		}
		node.KnownNodes["exact_node_pref"] = Node{
			ID:           "exact_node_pref",
			Address:      exactServer.Listener.Addr().String(),
			TrustDomains: []string{"sub.prefer.com"},
		}
		node.KnownNodesMutex.Unlock()

		result, err := node.QueryOtherDomain("sub.prefer.com", "identity", "test")
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}

		resultMap, ok := result.(map[string]interface{})
		if !ok {
			t.Fatalf("Expected map result, got %T", result)
		}

		if resultMap["found"] != "exact" {
			t.Errorf("Expected exact match 'exact', got '%v'", resultMap["found"])
		}
	})

	t.Run("returns error when no nodes found at any level", func(t *testing.T) {
		_, err := node.QueryOtherDomain("totally.unknown.tld", "identity", "test")
		if err == nil {
			t.Error("Expected error for completely unknown domain hierarchy")
		}
	})
}

func TestFindNodesForDomainWithHierarchy(t *testing.T) {
	node := newTestNode()

	node.KnownNodesMutex.Lock()
	node.KnownNodes["level1_node"] = Node{
		ID:           "level1_node",
		Address:      "level1.example:8080",
		TrustDomains: []string{"example.org"},
	}
	node.KnownNodes["level2_node"] = Node{
		ID:           "level2_node",
		Address:      "level2.example:8080",
		TrustDomains: []string{"sub.example.org"},
	}
	node.KnownNodesMutex.Unlock()

	t.Run("finds exact domain match", func(t *testing.T) {
		nodes := node.findNodesForDomainWithHierarchy("sub.example.org")
		if len(nodes) != 1 {
			t.Fatalf("Expected 1 node, got %d", len(nodes))
		}
		if nodes[0].ID != "level2_node" {
			t.Errorf("Expected 'level2_node', got '%s'", nodes[0].ID)
		}
	})

	t.Run("walks up to parent domain", func(t *testing.T) {
		nodes := node.findNodesForDomainWithHierarchy("deep.sub.example.org")
		if len(nodes) != 1 {
			t.Fatalf("Expected 1 node, got %d", len(nodes))
		}
		if nodes[0].ID != "level2_node" {
			t.Errorf("Expected 'level2_node', got '%s'", nodes[0].ID)
		}
	})

	t.Run("walks up multiple levels", func(t *testing.T) {
		nodes := node.findNodesForDomainWithHierarchy("very.deep.nested.example.org")
		if len(nodes) != 1 {
			t.Fatalf("Expected 1 node, got %d", len(nodes))
		}
		if nodes[0].ID != "level1_node" {
			t.Errorf("Expected 'level1_node', got '%s'", nodes[0].ID)
		}
	})

	t.Run("returns nil for unknown hierarchy", func(t *testing.T) {
		nodes := node.findNodesForDomainWithHierarchy("unknown.tld")
		if len(nodes) != 0 {
			t.Errorf("Expected 0 nodes, got %d", len(nodes))
		}
	})
}

func TestGetTrustDomainNodes(t *testing.T) {
	node := newTestNode()

	node.KnownNodesMutex.Lock()
	node.KnownNodes["domain_validator"] = Node{
		ID:           "domain_validator",
		Address:      "validator.example:8080",
		TrustDomains: []string{"validators.domain.com"},
	}
	node.KnownNodesMutex.Unlock()

	node.TrustDomainsMutex.Lock()
	node.TrustDomains["validators.domain.com"] = TrustDomain{
		Name:           "validators.domain.com",
		ValidatorNodes: []string{"domain_validator"},
	}
	node.TrustDomainsMutex.Unlock()

	t.Run("returns validators for known domain", func(t *testing.T) {
		nodes := node.GetTrustDomainNodes("validators.domain.com")
		if len(nodes) != 1 {
			t.Fatalf("Expected 1 node, got %d", len(nodes))
		}
		if nodes[0].ID != "domain_validator" {
			t.Errorf("Expected 'domain_validator', got '%s'", nodes[0].ID)
		}
	})

	t.Run("returns empty for unknown domain", func(t *testing.T) {
		nodes := node.GetTrustDomainNodes("unknown.domain.com")
		if len(nodes) != 0 {
			t.Errorf("Expected 0 nodes, got %d", len(nodes))
		}
	})
}
