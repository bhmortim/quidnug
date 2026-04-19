package sigstore

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/quidnug/quidnug/pkg/client"
)

func TestRecordBundleSubmitsEvent(t *testing.T) {
	var hitPath string
	var gotTx map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hitPath = r.URL.Path
		// The emit-event path auto-fetches the stream first. Return 404
		// then the actual POST.
		if r.Method == http.MethodGet {
			w.WriteHeader(404)
			_ = json.NewEncoder(w).Encode(map[string]any{
				"success": false,
				"error":   map[string]any{"code": "NOT_FOUND"},
			})
			return
		}
		_ = json.NewDecoder(r.Body).Decode(&gotTx)
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"success": true, "data": map[string]any{"id": "evt-1", "sequence": 1},
		})
	}))
	defer srv.Close()

	c, _ := client.New(srv.URL, client.WithMaxRetries(0))
	r, err := New(Options{Client: c, Domain: "supplychain.example.com"})
	if err != nil {
		t.Fatal(err)
	}
	signer, _ := client.GenerateQuid()

	res, err := r.RecordBundle(context.Background(), signer, Bundle{
		ArtifactID:     "artifact-1",
		ArtifactDigest: "sha256:abc",
		SignatureB64:   "MEUCIQDfake",
		CertificatePEM: "-----BEGIN CERTIFICATE-----\nFAKE\n-----END CERTIFICATE-----",
		Signer:         "alice@example.com",
		SignedAt:       time.Unix(1700000000, 0),
		BundleURI:      "https://rekor.sigstore.dev/api/v1/log/entries/123",
	})
	if err != nil {
		t.Fatalf("RecordBundle: %v", err)
	}
	if res["id"] != "evt-1" {
		t.Fatalf("unexpected result: %v", res)
	}
	if hitPath != "/api/events" {
		t.Fatalf("wrong path: %s", hitPath)
	}
	if gotTx["type"] != "EVENT" || gotTx["eventType"] != "SIGSTORE_SIGNATURE" {
		t.Fatalf("tx shape: %+v", gotTx)
	}
	payload, ok := gotTx["payload"].(map[string]any)
	if !ok || payload["schema"] != "sigstore-bundle/v0.2" {
		t.Fatalf("payload shape: %+v", gotTx["payload"])
	}
	if payload["bundleUri"] != "https://rekor.sigstore.dev/api/v1/log/entries/123" {
		t.Fatalf("missing bundle URI: %v", payload["bundleUri"])
	}
}

func TestRecordBundleValidatesFields(t *testing.T) {
	c, _ := client.New("http://x")
	r, _ := New(Options{Client: c})
	_, err := r.RecordBundle(context.Background(), nil, Bundle{})
	if err == nil {
		t.Fatal("expected error on missing ArtifactID")
	}
}
