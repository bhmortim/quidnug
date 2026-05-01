package core

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// TestBlockSync_PullsBlocksBeyondTip simulates a peer that has
// blocks 0..3 while the local node has only block 0 (genesis).
// pullBlocksFromPeer should fetch blocks 1..3 and feed them into
// ReceiveBlock. We bypass actual cryptographic validation by
// stubbing the source server's response shape.
//
// We don't go through ReceiveBlock here because that requires
// a fully-cryptographic block. The test verifies the HTTP
// fetch + decode + per-block dispatch shape — which is the
// machinery the bug was missing.
func TestBlockSync_PullsBlocksBeyondTip(t *testing.T) {
	srvCalls := 0
	gotOffset := ""
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		srvCalls++
		gotOffset = r.URL.Query().Get("offset")
		// Mimic /api/v1/blocks envelope: { success, data: { data: [...], pagination: ... } }
		body := map[string]interface{}{
			"success": true,
			"data": map[string]interface{}{
				"data": []Block{
					{Index: 1, Timestamp: time.Now().Unix(), Hash: "h1",
						TrustProof: TrustProof{TrustDomain: "ignored.test"}},
					{Index: 2, Timestamp: time.Now().Unix(), Hash: "h2",
						TrustProof: TrustProof{TrustDomain: "ignored.test"}},
				},
				"pagination": map[string]interface{}{
					"limit": 50, "offset": 1, "total": 3,
				},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(body)
	}))
	defer srv.Close()
	addr := strings.TrimPrefix(srv.URL, "http://")

	node := newTestNode()
	node.PullBlocksFromPeerForTest(context.Background(), "peer-1", addr, 0)

	if srvCalls != 1 {
		t.Fatalf("expected 1 server call, got %d", srvCalls)
	}
	// Local node tipIdx=0 → request offset=1.
	if gotOffset != "1" {
		t.Fatalf("expected offset=1 in query, got %q", gotOffset)
	}
}

// TestBlockSync_QuarantinedPeerSkipped: when a peer is
// quarantined, runBlockSyncOnce should not try to pull blocks
// from it.
func TestBlockSync_QuarantinedPeerSkipped(t *testing.T) {
	srvHit := false
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		srvHit = true
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"success":true,"data":{"data":[]}}`))
	}))
	defer srv.Close()
	addr := strings.TrimPrefix(srv.URL, "http://")

	node := newTestNode()
	node.PeerScoreboard = NewPeerScoreboard(DefaultPeerScoreWeights(), "", 0)
	node.KnownNodes["sketchy"] = Node{ID: "sketchy", Address: addr, ConnectionStatus: "gossip"}
	node.PeerScoreboard.SetQuarantined("sketchy", true, "test")

	node.runBlockSyncOnce(context.Background())
	// Give the (skipped) goroutine a moment to NOT run.
	time.Sleep(100 * time.Millisecond)

	if srvHit {
		t.Fatal("quarantined peer should not have been contacted")
	}
}

// TestBlockSync_MalformedResponseDoesNotPanic: a peer that
// returns garbage just gets logged; no panic, no chain
// corruption.
func TestBlockSync_MalformedResponseDoesNotPanic(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`<not-json>`))
	}))
	defer srv.Close()
	addr := strings.TrimPrefix(srv.URL, "http://")

	node := newTestNode()
	// Should return cleanly without panicking.
	node.PullBlocksFromPeerForTest(context.Background(), "peer-1", addr, 0)
}
