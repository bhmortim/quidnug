package core

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
)

// stubInfoServer is a tiny httptest server that serves
// /api/v1/info with the supplied nodeQuid + operatorQuid. Used
// by admit-pipeline tests to simulate a peer's handshake
// endpoint without standing up a full QuidnugNode.
func stubInfoServer(t *testing.T, nodeQuid, operatorQuid string) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/info" {
			http.NotFound(w, r)
			return
		}
		body := map[string]any{
			"success": true,
			"data": map[string]any{
				"nodeQuid": nodeQuid,
			},
		}
		if operatorQuid != "" {
			body["data"].(map[string]any)["operatorQuid"] = map[string]any{"id": operatorQuid}
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(body)
	}))
	return srv
}

// TestAdmitPeer_NoGates_PermissiveMode confirms that with all
// gates off, the admit pipeline doesn't even contact the peer's
// /api/v1/info — it accepts the candidate's self-asserted
// NodeQuid. This is the back-compat path for existing tests.
func TestAdmitPeer_NoGates_PermissiveMode(t *testing.T) {
	node := newTestNode()
	v, err := node.AdmitPeer(context.Background(), PeerCandidate{
		Address:  "127.0.0.1:1",
		NodeQuid: "selfclaimed12345",
		Source:   PeerSourceGossip,
	}, PeerAdmitConfig{
		// All gates off.
	})
	if err != nil {
		t.Fatalf("admit: %v", err)
	}
	if v.NodeQuid != "selfclaimed12345" {
		t.Fatalf("nodeQuid: %q", v.NodeQuid)
	}
}

// TestAdmitPeer_GatesActive_RequiresHandshake confirms that any
// gate flips on the handshake step — handshake failures reject.
func TestAdmitPeer_GatesActive_RequiresHandshake(t *testing.T) {
	node := newTestNode()
	// Point at a port that isn't listening, so handshake fails.
	_, err := node.AdmitPeer(context.Background(), PeerCandidate{
		Address: "127.0.0.1:1",
		Source:  PeerSourceGossip,
	}, PeerAdmitConfig{
		MinOperatorTrust: 0.5,
		HandshakeTimeout: 200 * time.Millisecond,
	})
	if err == nil || !strings.Contains(err.Error(), "handshake") {
		t.Fatalf("expected handshake failure, got %v", err)
	}
}

// TestAdmitPeer_RequireAdvertisement_RejectsUnattested confirms
// that a peer whose NodeQuid is absent from the
// NodeAdvertisementRegistry is rejected when
// RequireAdvertisement is true.
func TestAdmitPeer_RequireAdvertisement_RejectsUnattested(t *testing.T) {
	srv := stubInfoServer(t, "peerquid12345678", "")
	defer srv.Close()
	addr := strings.TrimPrefix(srv.URL, "http://")

	node := newTestNode()
	_, err := node.AdmitPeer(context.Background(), PeerCandidate{
		Address: addr,
		Source:  PeerSourceGossip,
	}, PeerAdmitConfig{
		RequireAdvertisement: true,
		HandshakeTimeout:     2 * time.Second,
	})
	if err == nil || !strings.Contains(err.Error(), "no current NodeAdvertisement") {
		t.Fatalf("expected ad-required rejection, got %v", err)
	}
}

// TestAdmitPeer_NodeQuidMismatch_Rejects confirms the handshake
// cross-check fails when a peer claims one NodeQuid but the
// candidate pinned another.
func TestAdmitPeer_NodeQuidMismatch_Rejects(t *testing.T) {
	srv := stubInfoServer(t, "actualpeerquidx1", "")
	defer srv.Close()
	addr := strings.TrimPrefix(srv.URL, "http://")

	node := newTestNode()
	_, err := node.AdmitPeer(context.Background(), PeerCandidate{
		Address:  addr,
		NodeQuid: "expectedquidxxxx",
		Source:   PeerSourceStatic,
	}, PeerAdmitConfig{
		RequireAdvertisement: true,
		HandshakeTimeout:     2 * time.Second,
	})
	if err == nil || !strings.Contains(err.Error(), "NodeQuid mismatch") {
		t.Fatalf("expected NodeQuid mismatch, got %v", err)
	}
}

// TestPrivateAddrAllowList_HostAndHostPort confirms allow-list
// matches both "host:port" and bare "host" tokens.
func TestPrivateAddrAllowList_HostAndHostPort(t *testing.T) {
	a := NewPrivateAddrAllowList()
	a.Set([]string{"192.168.1.50:8080", "10.0.0.1"})
	if !a.Has("192.168.1.50:8080") {
		t.Fatal("exact host:port should match")
	}
	// 10.0.0.1 listed bare; should match any port for that host.
	if !a.Has("10.0.0.1:9090") {
		t.Fatal("bare host should match any port")
	}
	if a.Has("192.168.1.50:9090") {
		t.Fatal("specific port allow should not match other ports")
	}
	if a.Has("172.16.0.1:8080") {
		t.Fatal("unrelated address should not match")
	}
}

// TestPrivateAddrAllowList_Concurrent confirms thread safety
// under concurrent Set + Has from multiple goroutines.
func TestPrivateAddrAllowList_Concurrent(t *testing.T) {
	a := NewPrivateAddrAllowList()
	var wg sync.WaitGroup
	for i := 0; i < 4; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			for j := 0; j < 200; j++ {
				a.Set([]string{"a:1", "b:2", "c:3"})
				_ = a.Has("a:1")
				_ = a.Has("nonexistent:9")
			}
		}(i)
	}
	wg.Wait()
}
