// Tests for the QDP-0014 discovery HTTP endpoints.
// Complements node_advertisement_test.go (the core primitive)
// with end-to-end HTTP assertions against each discovery endpoint.
package core

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gorilla/mux"
)

// newDiscoveryTestServer wires the minimum surface required to
// exercise the discovery handlers: a node with a registered
// domain, a consortium member, an advertisement in the registry,
// and a stub quid-domain index populated with some activity.
func newDiscoveryTestServer(t *testing.T) (*QuidnugNode, *mux.Router, *testNodeActor, *testNodeActor) {
	t.Helper()
	f := newAdvertisementTestFixture(t)

	// Ensure the validator-pubkey map is populated so the
	// domain handler's consortium response looks right.
	f.node.TrustDomainsMutex.Lock()
	td := f.node.TrustDomains[f.domain]
	td.ValidatorPublicKeys = map[string]string{f.node.NodeID: f.node.GetPublicKeyHex()}
	f.node.TrustDomains[f.domain] = td
	f.node.TrustDomainsMutex.Unlock()

	// Publish a valid advertisement so the domain handler's
	// hints include something.
	ad := f.validAdvertisement(t, 1)
	if _, err := f.node.AddNodeAdvertisementTransaction(ad); err != nil {
		t.Fatalf("seed ad: %v", err)
	}
	block, err := f.node.GenerateBlock(f.domain)
	if err != nil {
		t.Fatalf("GenerateBlock: %v", err)
	}
	if err := f.node.AddBlock(*block); err != nil {
		t.Fatalf("AddBlock: %v", err)
	}

	// Seed some activity in the per-domain index so /quids has
	// something to return.
	nowSec := time.Now().Unix()
	f.node.QuidDomainIndex.observeTrustEdge(f.domain, "truster123456789", "trustee12345678a", nowSec-60)
	f.node.QuidDomainIndex.observeEvent(f.domain, "truster123456789", "REVIEW", nowSec-10)

	// Build a router scoped to /api/v2 for the tests.
	router := mux.NewRouter()
	v2 := router.PathPrefix("/api/v2").Subrouter()
	f.node.RegisterDiscoveryRoutes(v2)
	return f.node, router, f.operator, f.nodeAct
}

func doGET(t *testing.T, router *mux.Router, target string) (*httptest.ResponseRecorder, map[string]interface{}) {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, target, nil)
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)
	var body map[string]interface{}
	if rr.Code == 200 {
		if err := json.NewDecoder(rr.Body).Decode(&body); err != nil {
			t.Fatalf("decode: %v", err)
		}
	}
	return rr, body
}

// ============================================================

func TestDiscoveryDomainHandler(t *testing.T) {
	_, router, _, nodeAct := newDiscoveryTestServer(t)

	rr, body := doGET(t, router, "/api/v2/discovery/domain/operators.network.example.com")
	if rr.Code != http.StatusOK {
		t.Fatalf("status: got %d", rr.Code)
	}
	data, ok := body["data"].(map[string]interface{})
	if !ok {
		t.Fatalf("missing data object")
	}
	if data["domain"] != "operators.network.example.com" {
		t.Errorf("domain field mismatch: %v", data["domain"])
	}
	if _, ok := data["blockTip"]; !ok {
		t.Error("missing blockTip")
	}
	if _, ok := data["consortium"]; !ok {
		t.Error("missing consortium")
	}
	endpoints, ok := data["endpoints"].([]interface{})
	if !ok {
		t.Fatalf("endpoints should be an array; got %T", data["endpoints"])
	}

	// The advertisement we seeded lives for the node actor's
	// quid; it's not a consortium member for this domain, but
	// ListForDomain should still surface it via SupportedDomains
	// glob match. Since we used "*.public.technology.laptops"
	// the hint won't match operators.network.example.com.
	// The consortium member path (node.NodeID) has no
	// advertisement of its own, so the endpoints list is empty
	// in this specific fixture. That's fine — we're verifying
	// the shape, not the population.
	_ = endpoints
	_ = nodeAct
}

func TestDiscoveryDomainHandler_Unknown(t *testing.T) {
	_, router, _, _ := newDiscoveryTestServer(t)
	rr, _ := doGET(t, router, "/api/v2/discovery/domain/does-not-exist")
	if rr.Code != http.StatusNotFound {
		t.Fatalf("expected 404; got %d", rr.Code)
	}
}

func TestDiscoveryNodeHandler(t *testing.T) {
	_, router, _, nodeAct := newDiscoveryTestServer(t)

	rr, body := doGET(t, router, "/api/v2/discovery/node/"+nodeAct.QuidID)
	if rr.Code != http.StatusOK {
		t.Fatalf("status: got %d, body=%s", rr.Code, rr.Body.String())
	}
	data, _ := body["data"].(map[string]interface{})
	if data["nodeQuid"] != nodeAct.QuidID {
		t.Errorf("nodeQuid mismatch: %v", data["nodeQuid"])
	}
}

func TestDiscoveryNodeHandler_BadQuid(t *testing.T) {
	_, router, _, _ := newDiscoveryTestServer(t)
	rr, _ := doGET(t, router, "/api/v2/discovery/node/not-a-quid")
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400; got %d", rr.Code)
	}
}

func TestDiscoveryNodeHandler_NotFound(t *testing.T) {
	_, router, _, _ := newDiscoveryTestServer(t)
	rr, _ := doGET(t, router, "/api/v2/discovery/node/aaaabbbbccccdddd")
	if rr.Code != http.StatusNotFound {
		t.Fatalf("expected 404; got %d", rr.Code)
	}
}

func TestDiscoveryOperatorHandler(t *testing.T) {
	_, router, operator, _ := newDiscoveryTestServer(t)

	rr, body := doGET(t, router, "/api/v2/discovery/operator/"+operator.QuidID)
	if rr.Code != http.StatusOK {
		t.Fatalf("status: got %d", rr.Code)
	}
	data, _ := body["data"].(map[string]interface{})
	nodes, ok := data["nodes"].([]interface{})
	if !ok {
		t.Fatalf("nodes should be an array")
	}
	if len(nodes) != 1 {
		t.Fatalf("expected 1 advertisement for operator; got %d", len(nodes))
	}
}

func TestDiscoveryQuidsHandler(t *testing.T) {
	_, router, _, _ := newDiscoveryTestServer(t)

	rr, body := doGET(t, router, "/api/v2/discovery/quids?domain=operators.network.example.com&sort=activity")
	if rr.Code != http.StatusOK {
		t.Fatalf("status: got %d, body=%s", rr.Code, rr.Body.String())
	}
	data, _ := body["data"].(map[string]interface{})
	quids, _ := data["quids"].([]interface{})
	if len(quids) < 2 {
		t.Fatalf("expected at least 2 active quids, got %d", len(quids))
	}
}

func TestDiscoveryQuidsHandler_RequiresDomain(t *testing.T) {
	_, router, _, _ := newDiscoveryTestServer(t)
	rr, _ := doGET(t, router, "/api/v2/discovery/quids")
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400; got %d", rr.Code)
	}
}

func TestDiscoveryQuidsHandler_EventTypeFilter(t *testing.T) {
	_, router, _, _ := newDiscoveryTestServer(t)
	// Only "truster123456789" has a REVIEW event in our seed.
	rr, body := doGET(t, router, "/api/v2/discovery/quids?domain=operators.network.example.com&eventType=REVIEW")
	if rr.Code != http.StatusOK {
		t.Fatalf("status: got %d", rr.Code)
	}
	data, _ := body["data"].(map[string]interface{})
	quids, _ := data["quids"].([]interface{})
	if len(quids) != 1 {
		t.Fatalf("expected 1 quid with REVIEW events, got %d", len(quids))
	}
	first, _ := quids[0].(map[string]interface{})
	if first["quidId"] != "truster123456789" {
		t.Errorf("unexpected quid: %v", first["quidId"])
	}
}

func TestDiscoveryTrustedQuidsHandler(t *testing.T) {
	node, router, _, nodeAct := newDiscoveryTestServer(t)

	// Seed a TRUST edge from the consortium member (node.NodeID)
	// directly into a target quid at level 0.8 so the trusted-
	// quids endpoint has data to return.
	node.TrustRegistryMutex.Lock()
	if node.TrustRegistry[node.NodeID] == nil {
		node.TrustRegistry[node.NodeID] = map[string]float64{}
	}
	node.TrustRegistry[node.NodeID][nodeAct.QuidID] = 0.8
	node.TrustRegistryMutex.Unlock()

	rr, body := doGET(t, router, "/api/v2/discovery/trusted-quids?domain=operators.network.example.com&min-trust=0.5")
	if rr.Code != http.StatusOK {
		t.Fatalf("status: got %d, body=%s", rr.Code, rr.Body.String())
	}
	data, _ := body["data"].(map[string]interface{})
	trusted, ok := data["trusted"].([]interface{})
	if !ok {
		t.Fatalf("trusted should be an array, got %T", data["trusted"])
	}
	found := false
	for _, row := range trusted {
		m := row.(map[string]interface{})
		if m["quidId"] == nodeAct.QuidID {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected nodeAct quid in trusted list")
	}
}
