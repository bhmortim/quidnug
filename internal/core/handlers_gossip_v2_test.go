// Package core — handlers_gossip_v2_test.go
//
// Methodology
// -----------
// HTTP-layer tests for QDP-0003 endpoints (fingerprint + anchor
// gossip). Each submit endpoint has a happy-path + bad-input test;
// the fingerprint GET endpoint has 404-absent + 200-present pairs.
//
// We route through setupTestRouter (which now mounts the v2
// subrouter), so these tests verify the production mux configuration
// and handler wiring, not just the handler function bodies.
package core

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// ----- fingerprint submit/query -------------------------------------------

func TestSubmitDomainFingerprintHandler_HappyPath(t *testing.T) {
	node := newTestNode()
	router := setupTestRouter(node)

	// Seed the producer's key so verification passes.
	node.NonceLedger.SetSignerKey(node.NodeID, 0, node.GetPublicKeyHex())

	fp := DomainFingerprint{
		SchemaVersion: DomainFingerprintSchemaVersion,
		Domain:        "payloads.example",
		BlockHeight:   10,
		BlockHash:     "block-hash-under-test",
		Timestamp:     time.Now().Unix(),
		ProducerQuid:  node.NodeID,
	}
	signed, _ := node.SignDomainFingerprint(fp)

	body, _ := json.Marshal(signed)
	req := httptest.NewRequest("POST", "/api/v2/domain-fingerprints", bytes.NewReader(body))
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusAccepted {
		t.Fatalf("status: want 202, got %d, body=%s", rr.Code, rr.Body.String())
	}
	stored, ok := node.NonceLedger.GetDomainFingerprint("payloads.example")
	if !ok {
		t.Fatal("handler did not store the fingerprint")
	}
	if stored.BlockHash != fp.BlockHash {
		t.Fatalf("stored hash mismatch: want %q, got %q", fp.BlockHash, stored.BlockHash)
	}
}

func TestSubmitDomainFingerprintHandler_Rejects400OnUnknownProducer(t *testing.T) {
	node := newTestNode()
	router := setupTestRouter(node)
	// newTestNode / NewQuidnugNode already seeds the node's own key.
	// To actually test the unknown-producer path we need a separate
	// party whose key the node doesn't know.
	stranger := newTestNode() // fresh node, fresh keys

	fp := DomainFingerprint{
		SchemaVersion: DomainFingerprintSchemaVersion,
		Domain:        "payloads.example",
		BlockHeight:   10,
		BlockHash:     "hash",
		Timestamp:     time.Now().Unix(),
		ProducerQuid:  stranger.NodeID, // `node` has never seen this quid
	}
	signed, _ := stranger.SignDomainFingerprint(fp)

	body, _ := json.Marshal(signed)
	req := httptest.NewRequest("POST", "/api/v2/domain-fingerprints", bytes.NewReader(body))
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("want 400, got %d (%s)", rr.Code, rr.Body.String())
	}
}

func TestGetLatestDomainFingerprintHandler_NotFound(t *testing.T) {
	node := newTestNode()
	router := setupTestRouter(node)

	req := httptest.NewRequest("GET", "/api/v2/domain-fingerprints/absent.example/latest", nil)
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Fatalf("want 404, got %d", rr.Code)
	}
}

func TestGetLatestDomainFingerprintHandler_ReturnsStored(t *testing.T) {
	node := newTestNode()
	router := setupTestRouter(node)

	node.NonceLedger.StoreDomainFingerprint(DomainFingerprint{
		SchemaVersion: DomainFingerprintSchemaVersion,
		Domain:        "present.example",
		BlockHeight:   42,
		BlockHash:     "block-42",
		Timestamp:     time.Now().Unix(),
		ProducerQuid:  "some-quid",
		Signature:     "aa",
	})

	req := httptest.NewRequest("GET", "/api/v2/domain-fingerprints/present.example/latest", nil)
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("want 200, got %d", rr.Code)
	}
	var resp struct {
		Success bool              `json:"success"`
		Data    DomainFingerprint `json:"data"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if !resp.Success {
		t.Fatal("response.success false")
	}
	if resp.Data.BlockHeight != 42 || resp.Data.BlockHash != "block-42" {
		t.Fatalf("wrong body: got %+v", resp.Data)
	}
}

// ----- anchor-gossip submit -----------------------------------------------

func TestSubmitAnchorGossipHandler_HappyPath(t *testing.T) {
	s := newOriginSetup(t, "origin.example.com")
	router := setupTestRouter(s.receiver)

	msg, _ := buildRotationGossip(t, s)

	body, _ := json.Marshal(msg)
	req := httptest.NewRequest("POST", "/api/v2/anchor-gossip", bytes.NewReader(body))
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusAccepted {
		t.Fatalf("want 202, got %d, body=%s", rr.Code, rr.Body.String())
	}

	// Receiver's ledger reflects the rotation — the apply ran.
	if got := s.receiver.NonceLedger.CurrentEpoch(s.origin.NodeID); got != 1 {
		t.Fatalf("ledger not updated: currentEpoch = %d", got)
	}
}

func TestSubmitAnchorGossipHandler_DuplicateReturns200(t *testing.T) {
	s := newOriginSetup(t, "origin.example.com")
	router := setupTestRouter(s.receiver)
	msg, _ := buildRotationGossip(t, s)

	// First submission — 202.
	body, _ := json.Marshal(msg)
	req := httptest.NewRequest("POST", "/api/v2/anchor-gossip", bytes.NewReader(body))
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)
	if rr.Code != http.StatusAccepted {
		t.Fatalf("first submit: want 202, got %d, body=%s", rr.Code, rr.Body.String())
	}

	// Replay — 200 with a duplicate indicator. We deliberately do NOT
	// return 400 here so relays with retry logic can be idempotent.
	req = httptest.NewRequest("POST", "/api/v2/anchor-gossip", bytes.NewReader(body))
	rr = httptest.NewRecorder()
	router.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("duplicate submit: want 200, got %d, body=%s", rr.Code, rr.Body.String())
	}

	var resp struct {
		Data struct {
			Duplicate bool `json:"duplicate"`
		} `json:"data"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if !resp.Data.Duplicate {
		t.Fatal("duplicate indicator missing from response body")
	}
}

func TestSubmitAnchorGossipHandler_RejectsMalformedJSON(t *testing.T) {
	node := newTestNode()
	router := setupTestRouter(node)

	req := httptest.NewRequest("POST", "/api/v2/anchor-gossip", bytes.NewReader([]byte("{not-json")))
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("want 400 on malformed JSON, got %d", rr.Code)
	}
}
