package chainlink

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/quidnug/quidnug/pkg/client"
)

func TestAdapterHappyPath(t *testing.T) {
	// Mock Quidnug node.
	node := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"success": true,
			"data": map[string]any{
				"trustLevel": 0.72,
				"trustPath":  []string{"a", "c", "b"},
				"pathDepth":  2,
				"observer":   "a",
				"target":     "b",
				"domain":     "demo.home",
			},
		})
	}))
	defer node.Close()
	c, _ := client.New(node.URL, client.WithMaxRetries(0))

	handler := Handler(c)
	req := Request{
		JobRunID: "job-42",
		Data: RequestData{
			Observer: "a", Target: "b", Domain: "demo.home", MaxDepth: 3,
		},
	}
	body, _ := json.Marshal(req)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(body)))

	if rec.Code != http.StatusOK {
		t.Fatalf("status: %d, body: %s", rec.Code, rec.Body.String())
	}
	var resp Response
	_ = json.Unmarshal(rec.Body.Bytes(), &resp)
	if resp.JobRunID != "job-42" {
		t.Fatalf("job id: %s", resp.JobRunID)
	}
	if resp.Result != 0.72 {
		t.Fatalf("result: %v", resp.Result)
	}
	if resp.Data.PathDepth != 2 {
		t.Fatalf("depth: %d", resp.Data.PathDepth)
	}
}

func TestAdapterRejectsMissingFields(t *testing.T) {
	c, _ := client.New("http://x")
	handler := Handler(c)
	body, _ := json.Marshal(Request{JobRunID: "x", Data: RequestData{Observer: "a"}})
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(body)))
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
}

func TestAdapterRejectsGet(t *testing.T) {
	c, _ := client.New("http://x")
	handler := Handler(c)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405, got %d", rec.Code)
	}
}
