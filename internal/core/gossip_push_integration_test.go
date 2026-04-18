// Package core — gossip_push_integration_test.go
//
// Methodology
// -----------
// These tests exercise push gossip across a simulated mini-
// network of 2–4 in-process nodes wired to one another via
// httptest servers. The goal is to prove end-to-end properties
// that unit tests on a single node can't:
//
//   - Multi-hop propagation. An anchor originating at node A
//     reaches node D via B and C without any of them pulling.
//
//   - Dedup across hops. A message that reaches D twice (once
//     via B, once via C) is applied once.
//
//   - Push + pull coexistence. A laggard node that missed the
//     push still gets the anchor via its periodic pull.
//
// We spin real httptest servers so we exercise the same marshal
// / unmarshal / handler stack that production traffic uses.
// Ensures the canonicalization lessons from QDP-0003 §8.3 still
// hold across the H1 envelope.
package core

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/mux"
)

// harness is a multi-node in-process simulator. Each node gets
// its own httptest server; KnownNodes is populated with the
// test servers' addresses so fan-out reaches them over real
// HTTP.
type harness struct {
	nodes   []*QuidnugNode
	servers []*httptest.Server
	t       *testing.T
}

func newHarness(t *testing.T, n int) *harness {
	t.Helper()
	h := &harness{t: t}

	for i := 0; i < n; i++ {
		node := newTestNode()
		node.PushGossipEnabled = true
		node.GossipTTL = 3

		// Mount only the cross-domain routes we need — keeps
		// the test surface tiny.
		r := mux.NewRouter()
		v2 := r.PathPrefix("/api/v2").Subrouter()
		node.registerCrossDomainRoutes(v2)

		srv := httptest.NewServer(r)
		h.nodes = append(h.nodes, node)
		h.servers = append(h.servers, srv)
	}

	// Wire every node to know every other node at its httptest
	// address. This gives a fully-connected topology; tests
	// that want a chain (A→B→C→D with no shortcuts) build
	// their own KnownNodes map instead.
	for i, node := range h.nodes {
		node.KnownNodesMutex.Lock()
		for j, other := range h.nodes {
			if i == j {
				continue
			}
			addr := strings.TrimPrefix(h.servers[j].URL, "http://")
			node.KnownNodes[other.NodeID] = Node{
				ID:      other.NodeID,
				Address: addr,
			}
		}
		node.KnownNodesMutex.Unlock()
	}

	// Every node must know every other node's epoch-0 key so
	// pushes can verify across the network.
	for i, receiver := range h.nodes {
		for j, producer := range h.nodes {
			if i == j {
				continue
			}
			receiver.NonceLedger.SetSignerKey(producer.NodeID, 0, producer.GetPublicKeyHex())
		}
	}

	return h
}

func (h *harness) close() {
	for _, s := range h.servers {
		s.Close()
	}
}

// wireChain rewires KnownNodes so node[i] only knows
// node[i+1]. Produces an A→B→C→D chain — useful for TTL-hop
// counting tests.
func (h *harness) wireChain() {
	for i, node := range h.nodes {
		node.KnownNodesMutex.Lock()
		node.KnownNodes = map[string]Node{}
		if i+1 < len(h.nodes) {
			next := h.nodes[i+1]
			addr := strings.TrimPrefix(h.servers[i+1].URL, "http://")
			node.KnownNodes[next.NodeID] = Node{ID: next.NodeID, Address: addr}
		}
		node.KnownNodesMutex.Unlock()
	}
}

// waitUntil polls fn at 10ms intervals until it returns true or
// deadline expires. Returns true if the condition was met in
// time. Tests use it rather than a flat time.Sleep to keep
// fast CI fast while still tolerating slow machines.
func waitUntil(deadline time.Duration, fn func() bool) bool {
	end := time.Now().Add(deadline)
	for time.Now().Before(end) {
		if fn() {
			return true
		}
		time.Sleep(10 * time.Millisecond)
	}
	return fn()
}

// ----- 2-node happy path ---------------------------------------------------

// A single push from A propagates to B and applies. Baseline
// end-to-end wire test.
func TestPushIntegration_SingleHop(t *testing.T) {
	h := newHarness(t, 2)
	defer h.close()
	a, b := h.nodes[0], h.nodes[1]

	env := buildPushEnvelope(t, a, b, "d.example")
	a.fanOutAnchorPush(env, "")

	if !waitUntil(2*time.Second, func() bool {
		return b.NonceLedger.CurrentEpoch(a.NodeID) == 1
	}) {
		t.Fatalf("B did not receive push; currentEpoch=%d", b.NonceLedger.CurrentEpoch(a.NodeID))
	}
}

// ----- Multi-hop propagation -----------------------------------------------

// A chain A→B→C→D with TTL=3. A pushes, D receives via B and C.
// Proves TTL-bounded forwarding works end-to-end.
func TestPushIntegration_MultiHopChain(t *testing.T) {
	h := newHarness(t, 4)
	defer h.close()
	h.wireChain()
	a, d := h.nodes[0], h.nodes[3]

	env := buildPushEnvelope(t, a, d, "d.example")
	a.fanOutAnchorPush(env, "")

	if !waitUntil(3*time.Second, func() bool {
		return d.NonceLedger.CurrentEpoch(a.NodeID) == 1
	}) {
		// Diagnose: check each hop.
		for i, n := range h.nodes {
			t.Logf("node %d (%s) currentEpoch(%s)=%d",
				i, n.NodeID, a.NodeID, n.NonceLedger.CurrentEpoch(a.NodeID))
		}
		t.Fatal("push did not reach D via A→B→C→D chain")
	}
}

// ----- Dedup across hops ---------------------------------------------------

// Fully-connected 3 nodes. A pushes. B forwards to C. A already
// pushed to C directly. C receives twice; dedup fires on the
// second. State is not corrupted by the double delivery.
func TestPushIntegration_DedupAcrossHops(t *testing.T) {
	h := newHarness(t, 3)
	defer h.close()
	a, _, c := h.nodes[0], h.nodes[1], h.nodes[2]

	env := buildPushEnvelope(t, a, c, "d.example")
	a.fanOutAnchorPush(env, "")

	if !waitUntil(2*time.Second, func() bool {
		return c.NonceLedger.CurrentEpoch(a.NodeID) == 1
	}) {
		t.Fatal("C did not receive push")
	}

	// After settling, epoch is still 1 (not double-applied).
	time.Sleep(200 * time.Millisecond)
	if got := c.NonceLedger.CurrentEpoch(a.NodeID); got != 1 {
		t.Fatalf("C epoch after dedup: want 1, got %d", got)
	}
}

// ----- Push + pull coexistence ---------------------------------------------

// Node X has PushGossipEnabled=false. It won't receive a push
// because no one pushes to it in its unreachable-peer topology.
// But a pull via POST /api/v2/anchor-gossip still works.
func TestPushIntegration_PullStillWorksWhenPushOff(t *testing.T) {
	h := newHarness(t, 2)
	defer h.close()
	a, b := h.nodes[0], h.nodes[1]

	b.PushGossipEnabled = false

	env := buildPushEnvelope(t, a, b, "d.example")
	// Pull-style submit instead of push.
	body, _ := json.Marshal(env.Payload)
	req := httptest.NewRequest("POST", "/api/v2/anchor-gossip", bytes.NewReader(body))
	rr := httptest.NewRecorder()
	r := mux.NewRouter()
	b.registerCrossDomainRoutes(r.PathPrefix("/api/v2").Subrouter())
	r.ServeHTTP(rr, req)

	if rr.Code != http.StatusAccepted {
		t.Fatalf("pull submit: want 202, got %d body=%s", rr.Code, rr.Body.String())
	}
	if got := b.NonceLedger.CurrentEpoch(a.NodeID); got != 1 {
		t.Fatalf("B epoch after pull: want 1, got %d", got)
	}
}

// ----- Fan-out excludes sender ---------------------------------------------

// Key anti-loop property. Node B forwards to everyone except
// the node that just sent to it — otherwise every push in a
// fully-connected 3-node network would ping-pong forever
// (relying purely on dedup to stop). This tests the
// ForwardedBy exclusion.
func TestPushIntegration_ForwardExcludesSender(t *testing.T) {
	h := newHarness(t, 3)
	defer h.close()
	a, b, c := h.nodes[0], h.nodes[1], h.nodes[2]

	// Count POSTs to A by wrapping A's httptest server's
	// handler. A receives the payload as part of the
	// fully-connected mesh when A originates. Then when B
	// forwards, B should EXCLUDE A from its fan-out.
	var aReceived int
	origServer := h.servers[0]
	origServer.Config.Handler = countingHandler(origServer.Config.Handler, &aReceived)

	env := buildPushEnvelope(t, a, b, "d.example")
	a.fanOutAnchorPush(env, "")

	// Wait for B and C to converge.
	if !waitUntil(2*time.Second, func() bool {
		return b.NonceLedger.CurrentEpoch(a.NodeID) == 1 &&
			c.NonceLedger.CurrentEpoch(a.NodeID) == 1
	}) {
		t.Fatal("B and C did not converge")
	}

	// A should not have received any pushes from B/C
	// forwarders — not with the exclude-sender rule. The
	// fan-out path from A→B and A→C are from A, not to A.
	time.Sleep(200 * time.Millisecond)
	// aReceived may legitimately be 0 or small (one from
	// C forwarding to A if the exclude rule failed). Assert
	// the exclusion — strict 0.
	if aReceived != 0 {
		t.Fatalf("A received %d forwarded pushes; expected 0 (exclude-sender rule)", aReceived)
	}
}

// countingHandler wraps h with a counter that increments on
// every POST to /api/v2/gossip/push-anchor.
func countingHandler(h http.Handler, counter *int) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "POST" && strings.HasPrefix(r.URL.Path, "/api/v2/gossip/push-anchor") {
			*counter++
		}
		h.ServeHTTP(w, r)
	})
}

// ----- Adversarial: forged producer ----------------------------------------

// A forged producer (random quid the receiver has never heard
// of) is dropped at the subscription filter. Ensures random
// nodes can't use a Quidnug cluster as a traffic amplifier.
func TestPushIntegration_RejectsForgedProducer(t *testing.T) {
	h := newHarness(t, 2)
	defer h.close()
	a, b := h.nodes[0], h.nodes[1]

	env := buildPushEnvelope(t, a, b, "d.example")
	// Swap producer quid to a stranger.
	stranger := newTestNode()
	env.Payload.GossipProducerQuid = stranger.NodeID
	// Also clear B's knowledge of the anchor subject so
	// subscription truly fails.
	b.NonceLedger = NewNonceLedger()

	body, _ := json.Marshal(env)
	resp, err := http.Post(h.servers[1].URL+"/api/v2/gossip/push-anchor",
		"application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("post: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("want 400, got %d", resp.StatusCode)
	}
}

// ----- Adversarial: schema drift -------------------------------------------

// An unknown envelope SchemaVersion is rejected with 400.
// Guards against silent acceptance of a future / malicious
// format during rollout.
func TestPushIntegration_RejectsUnknownSchema(t *testing.T) {
	h := newHarness(t, 2)
	defer h.close()
	a, b := h.nodes[0], h.nodes[1]

	env := buildPushEnvelope(t, a, b, "d.example")
	env.SchemaVersion = 99

	body, _ := json.Marshal(env)
	resp, err := http.Post(h.servers[1].URL+"/api/v2/gossip/push-anchor",
		"application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("post: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("want 400, got %d", resp.StatusCode)
	}
}

// ----- Adversarial: replay flood -------------------------------------------

// A compromised producer resending the same valid message
// 1000 times still only applies once; the 999 duplicates are
// cheap dedup hits. This is the QDP-0005 §5 dedup-first
// guarantee in action.
func TestPushIntegration_ReplayFloodHitsDedup(t *testing.T) {
	h := newHarness(t, 2)
	defer h.close()
	a, b := h.nodes[0], h.nodes[1]

	env := buildPushEnvelope(t, a, b, "d.example")
	body, _ := json.Marshal(env)

	// 50 replays (1000 is overkill for CI).
	for i := 0; i < 50; i++ {
		resp, err := http.Post(h.servers[1].URL+"/api/v2/gossip/push-anchor",
			"application/json", bytes.NewReader(body))
		if err != nil {
			t.Fatalf("post %d: %v", i, err)
		}
		resp.Body.Close()
	}
	if got := b.NonceLedger.CurrentEpoch(a.NodeID); got != 1 {
		t.Fatalf("replay flood applied %d times instead of 1", got)
	}
}

// ----- Fingerprint push integration ----------------------------------------

// End-to-end for the fingerprint path. Producer signs a
// fingerprint, pushes it, receiver stores it.
func TestPushIntegration_FingerprintSingleHop(t *testing.T) {
	h := newHarness(t, 2)
	defer h.close()
	a, b := h.nodes[0], h.nodes[1]

	// Seed B with a prior fingerprint so subscription matches.
	b.NonceLedger.StoreDomainFingerprint(DomainFingerprint{
		SchemaVersion: DomainFingerprintSchemaVersion,
		Domain:        "d.example",
		BlockHeight:   1,
		BlockHash:     "old-hash",
		ProducerQuid:  a.NodeID,
		Timestamp:     time.Now().Unix(),
	})

	fp := DomainFingerprint{
		SchemaVersion: DomainFingerprintSchemaVersion,
		Domain:        "d.example",
		BlockHeight:   2,
		BlockHash:     "new-hash",
		ProducerQuid:  a.NodeID,
		Timestamp:     time.Now().Unix(),
	}
	fp, err := a.SignDomainFingerprint(fp)
	if err != nil {
		t.Fatalf("sign: %v", err)
	}

	env := FingerprintPushMessage{
		SchemaVersion: GossipPushSchemaVersion,
		Payload:       fp,
		TTL:           3,
		ForwardedBy:   a.NodeID,
	}
	body, _ := json.Marshal(env)
	resp, err := http.Post(h.servers[1].URL+"/api/v2/gossip/push-fingerprint",
		"application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("post: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusAccepted {
		t.Fatalf("want 202, got %d body=%s", resp.StatusCode, bodyStr(resp))
	}

	stored, ok := b.NonceLedger.GetDomainFingerprint("d.example")
	if !ok || stored.BlockHeight != 2 {
		t.Fatalf("fingerprint not stored: ok=%v stored=%+v", ok, stored)
	}
}

// Monotonicity short-circuit: a fingerprint at an older height
// is dedup-dropped without signature verification. A later one
// supersedes.
func TestPushIntegration_FingerprintOlderHeightDropped(t *testing.T) {
	h := newHarness(t, 2)
	defer h.close()
	a, b := h.nodes[0], h.nodes[1]

	// Seed B with a newer fingerprint directly.
	newerFp, _ := a.SignDomainFingerprint(DomainFingerprint{
		SchemaVersion: DomainFingerprintSchemaVersion,
		Domain:        "d.example",
		BlockHeight:   10,
		BlockHash:     "newer-hash",
		ProducerQuid:  a.NodeID,
		Timestamp:     time.Now().Unix(),
	})
	b.NonceLedger.StoreDomainFingerprint(newerFp)

	// Now push an older fingerprint.
	olderFp, _ := a.SignDomainFingerprint(DomainFingerprint{
		SchemaVersion: DomainFingerprintSchemaVersion,
		Domain:        "d.example",
		BlockHeight:   5,
		BlockHash:     "older-hash",
		ProducerQuid:  a.NodeID,
		Timestamp:     time.Now().Unix(),
	})
	env := FingerprintPushMessage{
		SchemaVersion: GossipPushSchemaVersion,
		Payload:       olderFp,
		TTL:           3,
		ForwardedBy:   a.NodeID,
	}
	body, _ := json.Marshal(env)
	resp, err := http.Post(h.servers[1].URL+"/api/v2/gossip/push-fingerprint",
		"application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("post: %v", err)
	}
	defer resp.Body.Close()
	// Monotonicity hit is reported as 200 (idempotent-duplicate-like).
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("want 200 (monotonicity dup), got %d", resp.StatusCode)
	}
	stored, _ := b.NonceLedger.GetDomainFingerprint("d.example")
	if stored.BlockHeight != 10 {
		t.Fatalf("stored fingerprint regressed: height %d", stored.BlockHeight)
	}
}

// bodyStr drains a response body to string for diagnostics.
func bodyStr(r *http.Response) string {
	b := make([]byte, 512)
	n, _ := r.Body.Read(b)
	return fmt.Sprintf("%q", string(b[:n]))
}
