// Package core — handlers_guardian_test.go
//
// Methodology
// -----------
// Covers the v2 guardian HTTP surface. Each submit-endpoint test
// exercises:
//
//   * The happy path: valid payload is accepted (202 Accepted),
//     the anchor lands in PendingTxs.
//   * The validation-rejection path: the same handler's
//     ValidateX call rejects malformed input with 400.
//
// Query endpoints exercise 404-when-absent plus a 200-with-body
// after the relevant state has been installed directly.
//
// We use setupTestRouter (which now mounts /api/v2/guardian/*) so
// these tests exercise the exact mux configuration that
// StartServerWithConfig produces.
package core

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// buildSignedSetUpdate creates a fully-signed first-install
// GuardianSetUpdate ready for submission. Shared across the submit
// test and the query-after-accept test.
func buildSignedSetUpdate(t *testing.T, node *QuidnugNode, gs []guardian) GuardianSetUpdate {
	t.Helper()

	for _, g := range gs {
		node.NonceLedger.SetSignerKey(g.quid, 0, g.pub)
	}
	set := buildSet(gs, 1, 2*time.Hour)

	u := GuardianSetUpdate{
		Kind:        AnchorGuardianSetUpdate,
		SubjectQuid: node.NodeID,
		NewSet:      *set,
		AnchorNonce: 1,
		ValidFrom:   time.Now().Unix(),
	}
	signable, _ := GuardianSetUpdateSignableBytes(u)
	sig, err := node.SignData(signable)
	if err != nil {
		t.Fatalf("sign: %v", err)
	}
	u.PrimarySignature = &PrimarySignature{
		KeyEpoch:  0,
		Signature: hexEncode(sig),
	}
	u.NewGuardianConsents = consentsFromAll(t, gs, signable)
	return u
}

// ----- SetUpdate submission -----------------------------------------------

func TestSubmitGuardianSetUpdateHandler_HappyPath(t *testing.T) {
	node := newTestNode()
	router := setupTestRouter(node)

	gs := []guardian{newGuardian(t, "gA"), newGuardian(t, "gB")}
	update := buildSignedSetUpdate(t, node, gs)

	body, _ := json.Marshal(update)
	req := httptest.NewRequest("POST", "/api/v2/guardian/set-update", bytes.NewReader(body))
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusAccepted {
		t.Fatalf("status: want 202, got %d, body=%s", rr.Code, rr.Body.String())
	}

	// The anchor must be enqueued for inclusion in a future block.
	node.PendingTxsMutex.RLock()
	defer node.PendingTxsMutex.RUnlock()
	if len(node.PendingTxs) != 1 {
		t.Fatalf("want 1 pending tx, got %d", len(node.PendingTxs))
	}
	if _, ok := node.PendingTxs[0].(GuardianSetUpdateTransaction); !ok {
		t.Fatalf("pending tx has wrong type %T", node.PendingTxs[0])
	}
}

func TestSubmitGuardianSetUpdateHandler_Rejects400OnMissingConsent(t *testing.T) {
	node := newTestNode()
	router := setupTestRouter(node)

	gs := []guardian{newGuardian(t, "gA"), newGuardian(t, "gB")}
	update := buildSignedSetUpdate(t, node, gs)
	update.NewGuardianConsents = nil // strip consent

	body, _ := json.Marshal(update)
	req := httptest.NewRequest("POST", "/api/v2/guardian/set-update", bytes.NewReader(body))
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status: want 400, got %d, body=%s", rr.Code, rr.Body.String())
	}
	// Nothing enqueued.
	node.PendingTxsMutex.RLock()
	defer node.PendingTxsMutex.RUnlock()
	if len(node.PendingTxs) != 0 {
		t.Fatalf("rejected submission must not enqueue: got %d pending", len(node.PendingTxs))
	}
}

func TestSubmitGuardianSetUpdateHandler_RejectsMalformedJSON(t *testing.T) {
	node := newTestNode()
	router := setupTestRouter(node)

	req := httptest.NewRequest("POST", "/api/v2/guardian/set-update", bytes.NewReader([]byte("{not-json")))
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("want 400 on malformed JSON, got %d", rr.Code)
	}
}

// ----- Recovery flow submission --------------------------------------------

func TestSubmitGuardianRecoveryInitHandler_HappyPath(t *testing.T) {
	node := newTestNode()
	router := setupTestRouter(node)

	// Subject has a set installed via direct ledger manipulation (we're
	// testing the init handler here, not set install).
	gs := []guardian{newGuardian(t, "gA"), newGuardian(t, "gB")}
	for _, g := range gs {
		node.NonceLedger.SetSignerKey(g.quid, 0, g.pub)
	}
	node.NonceLedger.setGuardianSet(node.NodeID, buildSet(gs, 1, 2*time.Hour))

	_, newPub := keypairHex(t)
	init := GuardianRecoveryInit{
		Kind:         AnchorGuardianRecoveryInit,
		SubjectQuid:  node.NodeID,
		FromEpoch:    0,
		ToEpoch:      1,
		NewPublicKey: newPub,
		MinNextNonce: 1,
		AnchorNonce:  1,
		ValidFrom:    time.Now().Unix(),
	}
	signable, _ := GuardianRecoveryInitSignableBytes(init)
	init.GuardianSigs = []GuardianSignature{
		{GuardianQuid: gs[0].quid, KeyEpoch: 0, Signature: signWithGuardianKey(t, gs[0].priv, signable)},
	}

	body, _ := json.Marshal(init)
	req := httptest.NewRequest("POST", "/api/v2/guardian/recovery/init", bytes.NewReader(body))
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusAccepted {
		t.Fatalf("status: want 202, got %d, body=%s", rr.Code, rr.Body.String())
	}
	node.PendingTxsMutex.RLock()
	defer node.PendingTxsMutex.RUnlock()
	if len(node.PendingTxs) != 1 {
		t.Fatalf("want 1 pending tx, got %d", len(node.PendingTxs))
	}
}

// ----- Query handlers ------------------------------------------------------

func TestGetGuardianSetHandler_NotFound(t *testing.T) {
	node := newTestNode()
	router := setupTestRouter(node)

	req := httptest.NewRequest("GET", "/api/v2/guardian/set/nonexistent", nil)
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)
	if rr.Code != http.StatusNotFound {
		t.Fatalf("want 404 on missing set, got %d, body=%s", rr.Code, rr.Body.String())
	}
}

func TestGetGuardianSetHandler_ReturnsInstalledSet(t *testing.T) {
	node := newTestNode()
	router := setupTestRouter(node)

	gs := []guardian{newGuardian(t, "gA"), newGuardian(t, "gB")}
	node.NonceLedger.setGuardianSet(node.NodeID, buildSet(gs, 1, 3*time.Hour))

	req := httptest.NewRequest("GET", "/api/v2/guardian/set/"+node.NodeID, nil)
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("want 200 on installed set, got %d, body=%s", rr.Code, rr.Body.String())
	}

	var resp struct {
		Success bool `json:"success"`
		Data    struct {
			Guardians []GuardianRef `json:"guardians"`
			Threshold uint16        `json:"threshold"`
		} `json:"data"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v (body=%s)", err, rr.Body.String())
	}
	if !resp.Success {
		t.Fatal("response.success is false")
	}
	if len(resp.Data.Guardians) != 2 {
		t.Fatalf("guardians: want 2, got %d", len(resp.Data.Guardians))
	}
	if resp.Data.Threshold != 1 {
		t.Fatalf("threshold: want 1, got %d", resp.Data.Threshold)
	}
}

func TestGetPendingRecoveryHandler_NotFound(t *testing.T) {
	node := newTestNode()
	router := setupTestRouter(node)

	req := httptest.NewRequest("GET", "/api/v2/guardian/pending-recovery/nonexistent", nil)
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)
	if rr.Code != http.StatusNotFound {
		t.Fatalf("want 404, got %d", rr.Code)
	}
}

func TestGetPendingRecoveryHandler_ReturnsPending(t *testing.T) {
	node := newTestNode()
	router := setupTestRouter(node)

	node.NonceLedger.beginPendingRecovery(node.NodeID, &PendingRecovery{
		InitHash:        "deadbeef",
		InitBlockHeight: 42,
		MaturityUnix:    time.Now().Add(1 * time.Hour).Unix(),
		State:           RecoveryPending,
		Init: GuardianRecoveryInit{
			SubjectQuid: node.NodeID, FromEpoch: 0, ToEpoch: 1,
		},
	})

	req := httptest.NewRequest("GET", "/api/v2/guardian/pending-recovery/"+node.NodeID, nil)
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("want 200, got %d", rr.Code)
	}

	var resp struct {
		Success bool `json:"success"`
		Data    struct {
			InitHash        string `json:"initHash"`
			InitBlockHeight int64  `json:"initBlockHeight"`
			State           string `json:"state"`
		} `json:"data"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.Data.InitHash != "deadbeef" {
		t.Fatalf("initHash: want 'deadbeef', got %q", resp.Data.InitHash)
	}
	if resp.Data.State != "pending" {
		t.Fatalf("state: want 'pending', got %q", resp.Data.State)
	}
}
