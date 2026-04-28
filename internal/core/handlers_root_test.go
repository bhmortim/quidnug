package core

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gorilla/mux"
)

// setupTestRootRouter mirrors StartServerWithConfig's root-route
// registration so RootHandler and RobotsHandler can be exercised in
// isolation without spinning up the full server.
func setupTestRootRouter(node *QuidnugNode) *mux.Router {
	router := mux.NewRouter()
	router.HandleFunc("/", node.RootHandler).Methods("GET")
	router.HandleFunc("/robots.txt", node.RobotsHandler).Methods("GET")
	return router
}

func TestRootHandler_HTMLByDefault(t *testing.T) {
	node := newTestNode()
	router := setupTestRootRouter(node)

	req := httptest.NewRequest("GET", "/", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	ct := w.Header().Get("Content-Type")
	if !strings.HasPrefix(ct, "text/html") {
		t.Errorf("expected text/html content type, got %q", ct)
	}
	body := w.Body.String()
	for _, want := range []string{
		"<title>Quidnug node",
		"This node",
		"Version",
		QuidnugVersion,
		node.NodeID,
		"/api/v1/info",
		"/api/v1/domains",
		"github.com/bhmortim/quidnug",
		"QRP-0001",
		"home-operator-plan",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("rendered HTML missing substring %q", want)
		}
	}
}

func TestRootHandler_BrowserAcceptHeaderStaysHTML(t *testing.T) {
	node := newTestNode()
	router := setupTestRootRouter(node)

	// What a real browser sends: prefers HTML, fine with JSON as fallback.
	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("Accept",
		"text/html,application/xhtml+xml,application/xml;q=0.9,application/json;q=0.8,*/*;q=0.5")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	if ct := w.Header().Get("Content-Type"); !strings.HasPrefix(ct, "text/html") {
		t.Errorf("browser Accept should yield HTML, got %q", ct)
	}
}

func TestRootHandler_JSONWhenRequested(t *testing.T) {
	node := newTestNode()
	router := setupTestRootRouter(node)

	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("Accept", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	ct := w.Header().Get("Content-Type")
	if !strings.HasPrefix(ct, "application/json") {
		t.Errorf("expected application/json content type, got %q", ct)
	}

	var response map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if response["success"] != true {
		t.Errorf("expected success true, got %v", response["success"])
	}
	data, ok := response["data"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected data map, got %T", response["data"])
	}
	if data["version"] != QuidnugVersion {
		t.Errorf("expected version %q, got %v", QuidnugVersion, data["version"])
	}
	if data["nodeId"] != node.NodeID {
		t.Errorf("expected nodeId %q, got %v", node.NodeID, data["nodeId"])
	}
	endpoints, ok := data["endpoints"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected endpoints map, got %T", data["endpoints"])
	}
	for _, key := range []string{"health", "info", "domains", "nodes", "blocks", "metrics"} {
		if _, present := endpoints[key]; !present {
			t.Errorf("endpoints missing %q", key)
		}
	}
}

func TestRobotsHandler(t *testing.T) {
	node := newTestNode()
	router := setupTestRootRouter(node)

	req := httptest.NewRequest("GET", "/robots.txt", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	if ct := w.Header().Get("Content-Type"); !strings.HasPrefix(ct, "text/plain") {
		t.Errorf("expected text/plain, got %q", ct)
	}
	body := w.Body.String()
	if !strings.Contains(body, "User-agent: *") {
		t.Errorf("robots.txt missing User-agent directive: %q", body)
	}
	if !strings.Contains(body, "Allow: /") {
		t.Errorf("robots.txt missing Allow directive: %q", body)
	}
}

func TestWantsJSON(t *testing.T) {
	cases := []struct {
		name   string
		accept string
		want   bool
	}{
		{"empty header", "", false},
		{"explicit json", "application/json", true},
		{"browser default", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8", false},
		{"json before html", "application/json,text/html", false}, // html wins by presence
		{"wildcard only", "*/*", false},
		{"json with charset", "application/json; charset=utf-8", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/", nil)
			if tc.accept != "" {
				req.Header.Set("Accept", tc.accept)
			}
			got := wantsJSON(req)
			if got != tc.want {
				t.Errorf("Accept=%q: got %v, want %v", tc.accept, got, tc.want)
			}
		})
	}
}
