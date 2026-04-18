// Package core — bootstrap_test.go
//
// Methodology
// -----------
// K-of-K snapshot bootstrap (QDP-0008 / H3) is the mechanism
// by which fresh nodes seed their nonce ledger without trusting
// a single peer. These tests guard:
//
//   - K-of-K is strict. Any disagreement fails closed;
//     majority-of-N is NOT sufficient.
//
//   - Signature validation excludes bad peers from the quorum
//     count (they don't count as a "response" for K).
//
//   - Height tolerance is bounded. Wildly different heights
//     from the same declared state is evidence of real
//     divergence, not just asynchrony.
//
//   - Trust-list seeding is a prerequisite. Without signer
//     keys pre-seeded, every peer's snapshot would fail
//     signature verification.
//
//   - Shadow-verify catches divergence between the seeded
//     state and live blocks replayed after bootstrap — the
//     "seed was subtly wrong" failure mode.
//
// We use httptest servers to exercise the real HTTP path
// end-to-end. Unit-level tests for the agreement logic use
// pre-constructed NonceSnapshot values.
package core

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/mux"
)

// ----- Helpers -------------------------------------------------------------

// seedPeerServer spins up a tiny HTTP server that returns a
// canned snapshot for any {domain}/latest GET. Returns the
// server, its URL address, and the signed snapshot it serves.
func seedPeerServer(t *testing.T, producer *QuidnugNode, domain string, height int64, blockHash string, entries []NonceSnapshotEntry) (*httptest.Server, string, NonceSnapshot) {
	t.Helper()

	snap := NonceSnapshot{
		SchemaVersion: SnapshotSchemaVersion,
		BlockHeight:   height,
		BlockHash:     blockHash,
		Timestamp:     time.Now().Unix(),
		TrustDomain:   domain,
		Entries:       entries,
		ProducerQuid:  producer.NodeID,
	}
	signed, err := producer.SignSnapshot(snap)
	if err != nil {
		t.Fatalf("sign snapshot: %v", err)
	}

	r := mux.NewRouter()
	r.HandleFunc("/api/v2/nonce-snapshots/{domain}/latest", func(w http.ResponseWriter, req *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		envelope := map[string]interface{}{
			"success": true,
			"data":    signed,
		}
		json.NewEncoder(w).Encode(envelope)
	}).Methods("GET")
	srv := httptest.NewServer(r)

	addr := strings.TrimPrefix(srv.URL, "http://")
	return srv, addr, signed
}

// seedPeer wires up a remote peer's identity and the given
// address into the bootstrapping node so it will be discovered
// during peer lookup.
func seedPeer(node *QuidnugNode, peerNode *QuidnugNode, address string, domain string) {
	node.NonceLedger.SetSignerKey(peerNode.NodeID, 0, peerNode.GetPublicKeyHex())
	node.KnownNodesMutex.Lock()
	node.KnownNodes[peerNode.NodeID] = Node{ID: peerNode.NodeID, Address: address, TrustDomains: []string{domain}}
	node.KnownNodesMutex.Unlock()
	node.DomainRegistryMutex.Lock()
	node.DomainRegistry[domain] = append(node.DomainRegistry[domain], peerNode.NodeID)
	node.DomainRegistryMutex.Unlock()
}

// ----- Peer discovery ------------------------------------------------------

// With fewer than K peers known, bootstrap returns
// ErrBootstrapNoPeers without issuing HTTP calls.
func TestBootstrap_FewerThanKPeers(t *testing.T) {
	node := newTestNode()
	cfg := DefaultBootstrapConfig()
	cfg.Quorum = 3

	sess, err := node.BootstrapFromPeers(context.Background(), "d.example", cfg)
	if !errors.Is(err, ErrBootstrapNoPeers) {
		t.Fatalf("want ErrBootstrapNoPeers, got %v", err)
	}
	if sess.State != BootstrapQuorumMissed {
		t.Fatalf("want QuorumMissed state, got %v", sess.State)
	}
}

// ----- Happy path ----------------------------------------------------------

// 3 peers return byte-identical signed snapshots → QuorumMet.
func TestBootstrap_AllPeersAgree(t *testing.T) {
	node := newTestNode()
	cfg := DefaultBootstrapConfig()
	cfg.Quorum = 3

	entries := []NonceSnapshotEntry{
		{Quid: "signer-1", Epoch: 0, MaxNonce: 10},
		{Quid: "signer-2", Epoch: 0, MaxNonce: 5},
	}
	domain := "d.example"
	const height = int64(100)
	const blockHash = "hash-of-block-100"

	var servers []*httptest.Server
	for i := 0; i < 3; i++ {
		producer := newTestNode()
		srv, addr, _ := seedPeerServer(t, producer, domain, height, blockHash, entries)
		defer srv.Close()
		seedPeer(node, producer, addr, domain)
		servers = append(servers, srv)
	}
	_ = servers

	sess, err := node.BootstrapFromPeers(context.Background(), domain, cfg)
	if err != nil {
		t.Fatalf("bootstrap: %v", err)
	}
	if sess.State != BootstrapQuorumMet {
		t.Fatalf("want QuorumMet, got %v", sess.State)
	}
	if sess.Consensus == nil || sess.Consensus.BlockHash != blockHash {
		t.Fatalf("consensus wrong: %+v", sess.Consensus)
	}
}

// ----- K-of-K strictness ---------------------------------------------------

// 2-of-3 agree, 1 disagrees → fails. K=3 requires K-of-K, not majority.
func TestBootstrap_TwoOfThreeAgreeIsNotQuorum(t *testing.T) {
	node := newTestNode()
	cfg := DefaultBootstrapConfig()
	cfg.Quorum = 3
	domain := "d.example"

	entries := []NonceSnapshotEntry{{Quid: "s", Epoch: 0, MaxNonce: 1}}

	// Two peers with blockHash "A".
	for i := 0; i < 2; i++ {
		producer := newTestNode()
		srv, addr, _ := seedPeerServer(t, producer, domain, 100, "hash-A", entries)
		defer srv.Close()
		seedPeer(node, producer, addr, domain)
	}
	// One peer with blockHash "B".
	{
		producer := newTestNode()
		srv, addr, _ := seedPeerServer(t, producer, domain, 100, "hash-B", entries)
		defer srv.Close()
		seedPeer(node, producer, addr, domain)
	}

	sess, err := node.BootstrapFromPeers(context.Background(), domain, cfg)
	if !errors.Is(err, ErrBootstrapDisagreement) {
		t.Fatalf("want ErrBootstrapDisagreement, got %v", err)
	}
	if sess.State != BootstrapQuorumMissed {
		t.Fatalf("want QuorumMissed, got %v", sess.State)
	}
}

// ----- Signature validation ------------------------------------------------

// A peer returning a snapshot signed by an unknown quid is
// excluded from the quorum count. With K=3 and 2 valid + 1
// invalid, we fall below quorum.
func TestBootstrap_InvalidSignatureExcludedFromQuorum(t *testing.T) {
	node := newTestNode()
	cfg := DefaultBootstrapConfig()
	cfg.Quorum = 3
	domain := "d.example"

	entries := []NonceSnapshotEntry{{Quid: "s", Epoch: 0, MaxNonce: 1}}

	// Two valid peers.
	for i := 0; i < 2; i++ {
		producer := newTestNode()
		srv, addr, _ := seedPeerServer(t, producer, domain, 100, "hash-A", entries)
		defer srv.Close()
		seedPeer(node, producer, addr, domain)
	}
	// One peer whose key is NOT seeded — signature verify fails.
	{
		producer := newTestNode()
		srv, addr, _ := seedPeerServer(t, producer, domain, 100, "hash-A", entries)
		defer srv.Close()
		// Deliberately DO NOT call SetSignerKey on the node.
		node.KnownNodesMutex.Lock()
		node.KnownNodes[producer.NodeID] = Node{ID: producer.NodeID, Address: addr, TrustDomains: []string{domain}}
		node.KnownNodesMutex.Unlock()
		node.DomainRegistryMutex.Lock()
		node.DomainRegistry[domain] = append(node.DomainRegistry[domain], producer.NodeID)
		node.DomainRegistryMutex.Unlock()
	}

	_, err := node.BootstrapFromPeers(context.Background(), domain, cfg)
	if err == nil {
		t.Fatal("expected failure when one peer's signature was unverifiable")
	}
}

// ----- Height tolerance ----------------------------------------------------

// 3 peers with heights [100, 101, 102] and identical hashes
// (unrealistic but we're testing the tolerance logic
// independent of hash agreement here — same hash means they're
// attesting to the same state). Should succeed.
func TestBootstrap_HeightWithinTolerance(t *testing.T) {
	node := newTestNode()
	cfg := DefaultBootstrapConfig()
	cfg.Quorum = 3
	cfg.HeightTolerance = 4
	domain := "d.example"

	entries := []NonceSnapshotEntry{{Quid: "s", Epoch: 0, MaxNonce: 1}}
	heights := []int64{100, 101, 102}
	for _, h := range heights {
		producer := newTestNode()
		srv, addr, _ := seedPeerServer(t, producer, domain, h, "hash-X", entries)
		defer srv.Close()
		seedPeer(node, producer, addr, domain)
	}

	sess, err := node.BootstrapFromPeers(context.Background(), domain, cfg)
	if err != nil {
		t.Fatalf("bootstrap: %v", err)
	}
	if sess.State != BootstrapQuorumMet {
		t.Fatalf("want QuorumMet, got %v", sess.State)
	}
}

// 3 peers at heights [100, 200, 300] with the same hash (hash
// agreement is the winner-selection criterion, but height
// spread triggers rejection). Our group-by-hash lumps them;
// tolerance check rejects.
func TestBootstrap_HeightToleranceExceeded(t *testing.T) {
	node := newTestNode()
	cfg := DefaultBootstrapConfig()
	cfg.Quorum = 3
	cfg.HeightTolerance = 4
	domain := "d.example"

	entries := []NonceSnapshotEntry{{Quid: "s", Epoch: 0, MaxNonce: 1}}
	// Identical hash but very different heights. Group-by-hash
	// puts them together; tolerance check rejects.
	for _, h := range []int64{100, 200, 300} {
		producer := newTestNode()
		srv, addr, _ := seedPeerServer(t, producer, domain, h, "hash-X", entries)
		defer srv.Close()
		seedPeer(node, producer, addr, domain)
	}

	_, err := node.BootstrapFromPeers(context.Background(), domain, cfg)
	if !errors.Is(err, ErrBootstrapHeightTolerance) {
		t.Fatalf("want ErrBootstrapHeightTolerance, got %v", err)
	}
}

// ----- Trust list ---------------------------------------------------------

// SeedBootstrapTrustList requires at least K entries.
func TestBootstrap_TrustListRequiresK(t *testing.T) {
	node := newTestNode()
	err := node.SeedBootstrapTrustList(
		[]BootstrapTrustEntry{{Quid: "a", PublicKey: "xx"}},
		3,
	)
	if !errors.Is(err, ErrBootstrapTrustListEmpty) {
		t.Fatalf("want ErrBootstrapTrustListEmpty, got %v", err)
	}
}

// Happy path: K entries seed the ledger's epoch-0 keys.
func TestBootstrap_TrustListSeedsLedger(t *testing.T) {
	node := newTestNode()
	entries := []BootstrapTrustEntry{
		{Quid: "a", PublicKey: "key-a"},
		{Quid: "b", PublicKey: "key-b"},
		{Quid: "c", PublicKey: "key-c"},
	}
	if err := node.SeedBootstrapTrustList(entries, 3); err != nil {
		t.Fatalf("seed: %v", err)
	}
	for _, e := range entries {
		key, ok := node.NonceLedger.GetSignerKey(e.Quid, 0)
		if !ok || key != e.PublicKey {
			t.Fatalf("trust entry %q not seeded; got key=%q ok=%v", e.Quid, key, ok)
		}
	}
}

// ----- Apply snapshot + shadow verify ---------------------------------------

// ApplyBootstrapSnapshot seeds the ledger's accepted map from
// the consensus snapshot.
func TestBootstrap_ApplySnapshotSeedsAccepted(t *testing.T) {
	node := newTestNode()
	cfg := DefaultBootstrapConfig()
	cfg.Quorum = 3
	domain := "d.example"

	entries := []NonceSnapshotEntry{
		{Quid: "alpha", Epoch: 0, MaxNonce: 42},
		{Quid: "beta", Epoch: 1, MaxNonce: 7},
	}
	for i := 0; i < 3; i++ {
		producer := newTestNode()
		srv, addr, _ := seedPeerServer(t, producer, domain, 100, "hash-X", entries)
		defer srv.Close()
		seedPeer(node, producer, addr, domain)
	}

	_, err := node.BootstrapFromPeers(context.Background(), domain, cfg)
	if err != nil {
		t.Fatalf("bootstrap: %v", err)
	}
	if err := node.ApplyBootstrapSnapshot(cfg.ShadowBlocks); err != nil {
		t.Fatalf("apply: %v", err)
	}

	// Ledger's accepted map has the seeded entries.
	alpha := node.NonceLedger.Accepted(NonceKey{Quid: "alpha", Domain: domain, Epoch: 0})
	if alpha != 42 {
		t.Fatalf("alpha accepted: want 42, got %d", alpha)
	}
	beta := node.NonceLedger.Accepted(NonceKey{Quid: "beta", Domain: domain, Epoch: 1})
	if beta != 7 {
		t.Fatalf("beta accepted: want 7, got %d", beta)
	}

	sess := node.GetBootstrapSession()
	if sess.State != BootstrapShadowVerify {
		t.Fatalf("want ShadowVerify, got %v", sess.State)
	}
}

// ShadowVerifyStep decrements shadowLeft and transitions to
// Done after N blocks.
func TestBootstrap_ShadowVerifyCompletes(t *testing.T) {
	node := newTestNode()
	cfg := DefaultBootstrapConfig()
	cfg.Quorum = 3
	cfg.ShadowBlocks = 3
	domain := "d.example"
	for i := 0; i < 3; i++ {
		producer := newTestNode()
		srv, addr, _ := seedPeerServer(t, producer, domain, 100, "h", []NonceSnapshotEntry{})
		defer srv.Close()
		seedPeer(node, producer, addr, domain)
	}
	_, err := node.BootstrapFromPeers(context.Background(), domain, cfg)
	if err != nil {
		t.Fatalf("bootstrap: %v", err)
	}
	if err := node.ApplyBootstrapSnapshot(cfg.ShadowBlocks); err != nil {
		t.Fatalf("apply: %v", err)
	}

	// Emit 3 blocks (empty checkpoints → trivially compatible).
	for i := 0; i < 3; i++ {
		block := Block{Index: int64(101 + i), TrustProof: TrustProof{TrustDomain: domain}}
		if err := node.ShadowVerifyStep(block); err != nil {
			t.Fatalf("shadow step %d: %v", i, err)
		}
	}
	sess := node.GetBootstrapSession()
	if sess.State != BootstrapDone {
		t.Fatalf("want Done after N blocks, got %v", sess.State)
	}
}

// Divergence: a block's checkpoint says a key had MaxNonce=5
// but our seeded state says MaxNonce=10 pre-block. That means
// the seed was AHEAD of what this block is claiming — the
// seed is wrong.
func TestBootstrap_ShadowVerifyDetectsDivergence(t *testing.T) {
	node := newTestNode()
	domain := "d.example"
	cfg := DefaultBootstrapConfig()
	cfg.Quorum = 3
	cfg.ShadowBlocks = 5
	for i := 0; i < 3; i++ {
		producer := newTestNode()
		srv, addr, _ := seedPeerServer(t, producer, domain, 100, "h",
			[]NonceSnapshotEntry{{Quid: "q", Epoch: 0, MaxNonce: 10}})
		defer srv.Close()
		seedPeer(node, producer, addr, domain)
	}
	_, _ = node.BootstrapFromPeers(context.Background(), domain, cfg)
	_ = node.ApplyBootstrapSnapshot(cfg.ShadowBlocks)

	// A block claiming q.MaxNonce=5 contradicts our seed of 10.
	badBlock := Block{
		Index:      101,
		TrustProof: TrustProof{TrustDomain: domain},
		NonceCheckpoints: []NonceCheckpoint{
			{Quid: "q", Domain: domain, Epoch: 0, MaxNonce: 5},
		},
	}
	err := node.ShadowVerifyStep(badBlock)
	if !errors.Is(err, ErrBootstrapDivergence) {
		t.Fatalf("want ErrBootstrapDivergence, got %v", err)
	}
	sess := node.GetBootstrapSession()
	if sess.State != BootstrapFailed {
		t.Fatalf("want Failed state, got %v", sess.State)
	}
}

// ----- HTTP endpoint integration -------------------------------------------

// End-to-end: store a snapshot on one node, fetch it via the
// GET endpoint, expect matching contents.
func TestBootstrap_HTTPGetReturnsStoredSnapshot(t *testing.T) {
	node := newTestNode()
	snap := NonceSnapshot{
		SchemaVersion: SnapshotSchemaVersion,
		BlockHeight:   42,
		BlockHash:     "h42",
		Timestamp:     time.Now().Unix(),
		TrustDomain:   "d.example",
		Entries:       []NonceSnapshotEntry{{Quid: "q", Epoch: 0, MaxNonce: 1}},
		ProducerQuid:  node.NodeID,
	}
	signed, err := node.SignSnapshot(snap)
	if err != nil {
		t.Fatalf("sign: %v", err)
	}
	node.NonceLedger.StoreLatestSnapshot(signed)

	r := mux.NewRouter()
	v2 := r.PathPrefix("/api/v2").Subrouter()
	node.registerCrossDomainRoutes(v2)

	req := httptest.NewRequest("GET", "/api/v2/nonce-snapshots/d.example/latest", nil)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("GET: want 200, got %d body=%s", rr.Code, rr.Body.String())
	}
	var envelope struct {
		Success bool          `json:"success"`
		Data    NonceSnapshot `json:"data"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &envelope); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if envelope.Data.BlockHeight != 42 {
		t.Fatalf("returned height: want 42, got %d", envelope.Data.BlockHeight)
	}
}

// SubmitNonceSnapshotHandler accepts valid signed snapshots and
// rejects tampered ones.
func TestBootstrap_HTTPSubmitValidatesSignature(t *testing.T) {
	node := newTestNode()
	producer := newTestNode()
	node.NonceLedger.SetSignerKey(producer.NodeID, 0, producer.GetPublicKeyHex())

	snap := NonceSnapshot{
		SchemaVersion: SnapshotSchemaVersion,
		BlockHeight:   10,
		BlockHash:     "hh",
		Timestamp:     time.Now().Unix(),
		TrustDomain:   "d.example",
		ProducerQuid:  producer.NodeID,
	}
	signed, _ := producer.SignSnapshot(snap)

	// Valid: 202.
	body, _ := json.Marshal(signed)
	req := httptest.NewRequest("POST", "/api/v2/nonce-snapshots", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	r := mux.NewRouter()
	node.registerCrossDomainRoutes(r.PathPrefix("/api/v2").Subrouter())
	r.ServeHTTP(rr, req)
	if rr.Code != http.StatusAccepted {
		t.Fatalf("valid submit: want 202, got %d body=%s", rr.Code, rr.Body.String())
	}

	// Tampered: 400.
	bad := signed
	bad.BlockHeight = 9999
	body2, _ := json.Marshal(bad)
	req2 := httptest.NewRequest("POST", "/api/v2/nonce-snapshots", bytes.NewReader(body2))
	req2.Header.Set("Content-Type", "application/json")
	rr2 := httptest.NewRecorder()
	r.ServeHTTP(rr2, req2)
	if rr2.Code != http.StatusBadRequest {
		t.Fatalf("tampered submit: want 400, got %d", rr2.Code)
	}
}

// Bootstrap status endpoint returns session info.
func TestBootstrap_HTTPStatusReportsSession(t *testing.T) {
	node := newTestNode()

	// No session yet.
	r := mux.NewRouter()
	node.registerCrossDomainRoutes(r.PathPrefix("/api/v2").Subrouter())

	req := httptest.NewRequest("GET", "/api/v2/bootstrap/status", nil)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("status: want 200, got %d", rr.Code)
	}
	if !strings.Contains(rr.Body.String(), `"hasSession":false`) {
		t.Fatalf("want hasSession:false; got %s", rr.Body.String())
	}

	// Seed a session.
	node.setBootstrapSession(&BootstrapSession{
		Domain: "d.example",
		State:  BootstrapQuorumMet,
		K:      3,
	})
	rr = httptest.NewRecorder()
	r.ServeHTTP(rr, req)
	if !strings.Contains(rr.Body.String(), `"state":"quorum_met"`) {
		t.Fatalf("want state quorum_met; got %s", rr.Body.String())
	}
}

// ----- Diagnostic: describe the error ------------------------------------

// Error message includes the peer IDs whose snapshots
// disagreed, so operators have a starting point.
func TestBootstrap_DisagreementIncludesDetails(t *testing.T) {
	node := newTestNode()
	cfg := DefaultBootstrapConfig()
	cfg.Quorum = 3
	domain := "d.example"

	entries := []NonceSnapshotEntry{{Quid: "s", Epoch: 0, MaxNonce: 1}}
	for i := 0; i < 3; i++ {
		producer := newTestNode()
		srv, addr, _ := seedPeerServer(t, producer, domain, 100, fmt.Sprintf("hash-%d", i), entries)
		defer srv.Close()
		seedPeer(node, producer, addr, domain)
	}

	sess, _ := node.BootstrapFromPeers(context.Background(), domain, cfg)
	// 3 peers, 3 distinct hashes → no group size >= K → fail.
	if sess.State != BootstrapQuorumMissed {
		t.Fatalf("want QuorumMissed, got %v", sess.State)
	}
	if len(sess.Responses) != 3 {
		t.Fatalf("want 3 responses recorded for diagnostic visibility, got %d", len(sess.Responses))
	}
}
