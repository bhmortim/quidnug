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
	node.PullBlocksFromPeerForTest(context.Background(), "peer-1", addr)

	if srvCalls != 1 {
		t.Fatalf("expected 1 server call, got %d", srvCalls)
	}
	// ENG-82: offset is now slice-based (pagination position),
	// not (deprecated) localTipIdx+1. Initial pull starts at 0.
	// The mock server returns 2 < blockSyncBatchLimit blocks,
	// so the pagination loop terminates on the partial page
	// without a second call.
	if gotOffset != "0" {
		t.Fatalf("expected offset=0 in query, got %q", gotOffset)
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
	node.PullBlocksFromPeerForTest(context.Background(), "peer-1", addr)
}

// TestENG82_MultiDomain_NoSilentSkip pins the ENG-82 regression.
//
// Bug: pullBlocksFromPeer used (offset = localTipIdx + 1) and
// filtered returned blocks by (b.Index <= localTipIdx). After
// ENG-80 made block.Index per-domain, those three values
// (slice offset, per-domain tail Index, per-domain block Index)
// stopped being comparable. In a multi-domain mesh a peer's
// chain can hold a domain-Y block whose per-domain Index is
// smaller than the local tail's per-domain Index, sitting at a
// slice offset past localTipIdx+1 — slipping through the offset
// window and getting silently skipped by the index filter.
//
// Construction:
//
//   - producer chain: [genesis, X1(idx=1, dom=X), X2(idx=2, dom=X),
//     Y1(idx=1, dom=Y)]
//   - consumer locally already has [genesis, X1, X2] applied.
//   - consumer pulls. With the bug, Y1 was skipped because
//     Y1.Index (1) <= localTipIdx (X2's Index, 2). With the
//     hash-dedup fix, Y1 is recognized as a new hash and
//     applied.
//
// We stand up a real httptest server hosting the producer's
// /api/v1/blocks (real handler, real envelope shape) so the
// pagination + dedup wiring is exercised end-to-end.
func TestENG82_MultiDomain_NoSilentSkip(t *testing.T) {
	producer := newTestNode()
	producer.AllowDomainRegistration = true
	const domainX = "x.example.com"
	const domainY = "y.example.com"
	for _, d := range []string{domainX, domainY} {
		if err := producer.RegisterTrustDomain(TrustDomain{
			Name:           d,
			ValidatorNodes: []string{producer.NodeID},
			TrustThreshold: 0.75,
		}); err != nil {
			t.Fatalf("producer register %s: %v", d, err)
		}
	}

	// Build [genesis, X1, X2, Y1] on producer.
	x1 := Block{
		Index:     1,
		Timestamp: 1700000000,
		PrevHash:  producer.Blockchain[0].Hash,
		TrustProof: TrustProof{
			TrustDomain:        domainX,
			ValidatorID:        producer.NodeID,
			ValidatorPublicKey: producer.GetPublicKeyHex(),
			ValidationTime:     1700000000,
		},
	}
	signBlock(producer, &x1)
	if acc, err := producer.ReceiveBlock(x1); err != nil || acc != BlockTrusted {
		t.Fatalf("producer x1: err=%v acc=%d", err, acc)
	}
	x2 := Block{
		Index:     2,
		Timestamp: 1700000060,
		PrevHash:  x1.Hash,
		TrustProof: TrustProof{
			TrustDomain:        domainX,
			ValidatorID:        producer.NodeID,
			ValidatorPublicKey: producer.GetPublicKeyHex(),
			ValidationTime:     1700000060,
		},
	}
	signBlock(producer, &x2)
	if acc, err := producer.ReceiveBlock(x2); err != nil || acc != BlockTrusted {
		t.Fatalf("producer x2: err=%v acc=%d", err, acc)
	}
	y1 := Block{
		Index:     1, // per-domain index for Y, lower than X's tail
		Timestamp: 1700000120,
		PrevHash:  producer.Blockchain[0].Hash,
		TrustProof: TrustProof{
			TrustDomain:        domainY,
			ValidatorID:        producer.NodeID,
			ValidatorPublicKey: producer.GetPublicKeyHex(),
			ValidationTime:     1700000120,
		},
	}
	signBlock(producer, &y1)
	if acc, err := producer.ReceiveBlock(y1); err != nil || acc != BlockTrusted {
		t.Fatalf("producer y1: err=%v acc=%d", err, acc)
	}

	// Sanity: producer's chain is exactly what we built above.
	producer.BlockchainMutex.RLock()
	producerLen := len(producer.Blockchain)
	producerTail := producer.Blockchain[producerLen-1]
	producer.BlockchainMutex.RUnlock()
	if producerLen != 4 {
		t.Fatalf("producer chain length: want 4, got %d", producerLen)
	}
	if producerTail.Hash != y1.Hash {
		t.Fatalf("producer tail hash mismatch: want %s, got %s", y1.Hash, producerTail.Hash)
	}

	// Stand up an HTTP server backed by the producer's real
	// blocks handler so the pagination + envelope shape match
	// production. Mounting GetBlocksHandler directly works
	// because it doesn't pull path vars off the request.
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/blocks", producer.GetBlocksHandler)
	srv := httptest.NewServer(mux)
	defer srv.Close()
	addr := strings.TrimPrefix(srv.URL, "http://")

	// Consumer: trust the producer, manually pre-apply X1, X2
	// so the local tail is X2 (Index=2) at the time of pull.
	// Y1 has Index=1, which under the buggy filter would be
	// skipped (1 <= 2).
	consumer := newTestNode()
	consumer.TrustRegistry[consumer.NodeID] = map[string]float64{producer.NodeID: 0.9}
	if acc, err := consumer.ReceiveBlock(x1); err != nil || acc != BlockTrusted {
		t.Fatalf("consumer pre-apply x1: err=%v acc=%d", err, acc)
	}
	if acc, err := consumer.ReceiveBlock(x2); err != nil || acc != BlockTrusted {
		t.Fatalf("consumer pre-apply x2: err=%v acc=%d", err, acc)
	}

	// Pull from the producer. After ENG-82 fix, Y1 should be
	// pulled, dedup-checked (new hash → apply), and accepted.
	consumer.PullBlocksFromPeerForTest(context.Background(), producer.NodeID, addr)

	// Verify Y1 landed.
	consumer.BlockchainMutex.RLock()
	defer consumer.BlockchainMutex.RUnlock()
	hasY1 := false
	for _, b := range consumer.Blockchain {
		if b.Hash == y1.Hash {
			hasY1 = true
			break
		}
	}
	if !hasY1 {
		t.Fatalf("Y1 was not synced (silent-skip regression). Consumer chain has %d blocks; expected to include Y1=%s",
			len(consumer.Blockchain), y1.Hash)
	}
}

// TestENG82_HashDedup_SkipsDuplicates: when the peer serves
// blocks the consumer already has, each duplicate is a no-op
// rather than re-applied. Pinned because the new hash-set is
// the only dedup mechanism after the index-filter removal —
// regressions here would manifest as ReceiveBlock churn.
func TestENG82_HashDedup_SkipsDuplicates(t *testing.T) {
	producer := newTestNode()
	producer.AllowDomainRegistration = true
	const dom = "dedup.example.com"
	if err := producer.RegisterTrustDomain(TrustDomain{
		Name:           dom,
		ValidatorNodes: []string{producer.NodeID},
		TrustThreshold: 0.75,
	}); err != nil {
		t.Fatalf("register: %v", err)
	}

	b1 := Block{
		Index:     1,
		Timestamp: 1700000000,
		PrevHash:  producer.Blockchain[0].Hash,
		TrustProof: TrustProof{
			TrustDomain:        dom,
			ValidatorID:        producer.NodeID,
			ValidatorPublicKey: producer.GetPublicKeyHex(),
		},
	}
	signBlock(producer, &b1)
	if _, err := producer.ReceiveBlock(b1); err != nil {
		t.Fatalf("producer b1: %v", err)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/blocks", producer.GetBlocksHandler)
	srv := httptest.NewServer(mux)
	defer srv.Close()
	addr := strings.TrimPrefix(srv.URL, "http://")

	// Consumer has the SAME b1 already (we apply it manually
	// before pulling — same hash because deterministic).
	consumer := newTestNode()
	consumer.TrustRegistry[consumer.NodeID] = map[string]float64{producer.NodeID: 0.9}
	if _, err := consumer.ReceiveBlock(b1); err != nil {
		t.Fatalf("consumer pre-apply b1: %v", err)
	}
	consumer.BlockchainMutex.RLock()
	beforeLen := len(consumer.Blockchain)
	consumer.BlockchainMutex.RUnlock()

	// Pull. b1's hash matches the consumer's existing entry,
	// so dedup short-circuits before ReceiveBlock.
	consumer.PullBlocksFromPeerForTest(context.Background(), producer.NodeID, addr)

	consumer.BlockchainMutex.RLock()
	afterLen := len(consumer.Blockchain)
	consumer.BlockchainMutex.RUnlock()
	if afterLen != beforeLen {
		t.Fatalf("dedup: chain length changed from %d to %d on a duplicate-only pull", beforeLen, afterLen)
	}
}
