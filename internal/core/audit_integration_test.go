// QDP-0018 Phase 1 integration tests — audit log hooks on
// QuidnugNode write paths, plus the three HTTP query endpoints.
//
// The audit package has its own unit suite (see
// internal/audit/audit_test.go) covering chain, hash, and
// persistence invariants. This file exercises the integration
// surface:
//
//   - Node startup emits a NODE_LIFECYCLE entry.
//   - Moderation + DSR + consent admission emits the right
//     category entry.
//   - HTTP endpoints return consistent head / entries /
//     specific-entry views.
//   - Audit entries survive a disk round-trip via the file
//     store (smoke test — audit package has the deep coverage).
package core

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/mux"

	"github.com/quidnug/quidnug/internal/audit"
	"github.com/quidnug/quidnug/internal/config"
)

func TestAudit_NodeStartEmitsLifecycleEntry(t *testing.T) {
	node := newTestNode()
	head, ok := node.AuditLog.Head()
	if !ok {
		t.Fatal("audit log should have at least one entry after start")
	}
	if head.Category != audit.CategoryNodeLifecycle {
		t.Errorf("expected NODE_LIFECYCLE, got %q", head.Category)
	}
	if head.Payload["event"] != "node_start" {
		t.Errorf("expected event=node_start, got %+v", head.Payload)
	}
}

func TestAudit_ModerationAdmissionEmitsEntry(t *testing.T) {
	node, actor := newModerationTestFixture(t)
	startHeight := node.AuditLog.Height()

	tx := actor.signModeration(baselineAction(actor, 1))
	if _, err := node.AddModerationActionTransaction(tx); err != nil {
		t.Fatalf("moderation admission: %v", err)
	}

	got := node.AuditLog.EntriesSince(startHeight-1, 50)
	if len(got) == 0 {
		t.Fatal("expected at least one new audit entry")
	}
	// Find the moderation entry (may be the most recent).
	var found *audit.Entry
	for i := range got {
		if got[i].Category == audit.CategoryModerationAction {
			found = &got[i]
			break
		}
	}
	if found == nil {
		t.Fatalf("expected MODERATION_ACTION entry, got %+v", got)
	}
	if found.Payload["scope"] != "hide" {
		t.Errorf("expected scope=hide in payload, got %+v", found.Payload)
	}
	if found.Payload["reason_code"] != "SPAM" {
		t.Errorf("expected reason_code=SPAM, got %+v", found.Payload)
	}
}

func TestAudit_DSRAdmissionEmitsEntry(t *testing.T) {
	node, actor := newPrivacyTestFixture(t)
	startHeight := node.AuditLog.Height()

	tx := DataSubjectRequestTransaction{
		BaseTransaction: BaseTransaction{
			Type:        TxTypeDataSubjectRequest,
			TrustDomain: "test.domain.com",
			Timestamp:   time.Now().Unix(),
		},
		RequestType:  DSRTypeAccess,
		Jurisdiction: "EU",
		Nonce:        1,
	}
	tx = actor.signDSR(tx)
	if _, err := node.AddDataSubjectRequestTransaction(tx); err != nil {
		t.Fatalf("DSR admission: %v", err)
	}

	got := node.AuditLog.EntriesSince(startHeight-1, 50)
	var found *audit.Entry
	for i := range got {
		if got[i].Category == audit.CategoryDSRFulfillment {
			found = &got[i]
			break
		}
	}
	if found == nil {
		t.Fatalf("expected DSR_FULFILLMENT entry, got %+v", got)
	}
	if found.Payload["request_type"] != DSRTypeAccess {
		t.Errorf("expected request_type=ACCESS, got %+v", found.Payload)
	}
	if found.Payload["phase"] != "received" {
		t.Errorf("expected phase=received, got %+v", found.Payload)
	}
}

func TestAudit_HeadEndpoint(t *testing.T) {
	node := newTestNode()
	req := httptest.NewRequest(http.MethodGet, "/api/v2/audit/head", nil)
	w := httptest.NewRecorder()
	node.AuditHeadHandler(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var resp struct {
		Data struct {
			OperatorQuid string `json:"operatorQuid"`
			Height       int64  `json:"height"`
			HeadHash     string `json:"headHash"`
		} `json:"data"`
	}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.Data.Height < 1 {
		t.Errorf("expected positive height, got %d", resp.Data.Height)
	}
	if resp.Data.HeadHash == "" {
		t.Error("expected non-empty head hash")
	}
	if resp.Data.OperatorQuid == "" {
		t.Error("expected operator quid in response")
	}
}

func TestAudit_EntriesEndpointPaginates(t *testing.T) {
	node := newTestNode()
	// Add a few extra entries so pagination actually exercises.
	for i := 0; i < 3; i++ {
		node.emitAudit(audit.CategoryOperatorOther, map[string]interface{}{
			"i": i,
		}, "padding")
	}

	req := httptest.NewRequest(http.MethodGet, "/api/v2/audit/entries?since=-1&limit=2", nil)
	w := httptest.NewRecorder()
	node.AuditEntriesHandler(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var resp struct {
		Data struct {
			Entries []audit.Entry `json:"entries"`
			Height  int64         `json:"height"`
		} `json:"data"`
	}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(resp.Data.Entries) != 2 {
		t.Errorf("expected 2 entries (limit=2), got %d", len(resp.Data.Entries))
	}
	if resp.Data.Entries[0].Sequence != 0 {
		t.Errorf("expected first entry sequence=0, got %d", resp.Data.Entries[0].Sequence)
	}
}

func TestAudit_SpecificEntryEndpoint(t *testing.T) {
	node := newTestNode()
	router := mux.NewRouter()
	router.HandleFunc("/api/v2/audit/entry/{sequence}", node.AuditEntryHandler).Methods("GET")

	req := httptest.NewRequest(http.MethodGet, "/api/v2/audit/entry/0", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("entry/0 expected 200, got %d: %s", w.Code, w.Body.String())
	}

	badReq := httptest.NewRequest(http.MethodGet, "/api/v2/audit/entry/99999", nil)
	badW := httptest.NewRecorder()
	router.ServeHTTP(badW, badReq)
	if badW.Code != http.StatusNotFound {
		t.Errorf("entry/99999 expected 404, got %d", badW.Code)
	}
}

func TestAudit_ChainStaysIntactAcrossManyEntries(t *testing.T) {
	node := newTestNode()
	for i := 0; i < 20; i++ {
		node.emitAudit(audit.CategoryOperatorOther,
			map[string]interface{}{"i": i}, "bulk")
	}
	idx, err := node.AuditLog.VerifyChain()
	if err != nil {
		t.Fatalf("chain verify failed at %d: %v", idx, err)
	}
	if idx != -1 {
		t.Errorf("expected intact chain, first break at %d", idx)
	}
}

func TestAudit_NilLogIsSafe(t *testing.T) {
	node := newTestNode()
	node.AuditLog = nil // simulate operator-disabled audit

	// emitAudit must not panic with nil log.
	node.emitAudit(audit.CategoryOperatorOther, map[string]interface{}{"i": 1}, "")

	// HTTP endpoints return 404 when disabled.
	req := httptest.NewRequest(http.MethodGet, "/api/v2/audit/head", nil)
	w := httptest.NewRecorder()
	node.AuditHeadHandler(w, req)
	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404 when audit disabled, got %d", w.Code)
	}
}

func TestAudit_DiskStoreRoundTripViaConfig(t *testing.T) {
	dir := t.TempDir()
	logPath := filepath.Join(dir, "audit.jsonl")

	cfg := &config.Config{
		DataDir:      dir,
		AuditLogPath: logPath,
	}
	node1, err := NewQuidnugNode(cfg)
	if err != nil {
		t.Fatalf("first node: %v", err)
	}
	// Leave one extra entry.
	node1.emitAudit(audit.CategoryOperatorOther,
		map[string]interface{}{"k": "v"}, "round-trip marker")
	height1 := node1.AuditLog.Height()
	if err := node1.AuditLog.Close(); err != nil {
		t.Fatalf("close first: %v", err)
	}

	// Re-open. The second node should replay the log file.
	node2, err := NewQuidnugNode(cfg)
	if err != nil {
		t.Fatalf("second node: %v", err)
	}
	defer func() {
		if node2.AuditLog != nil {
			_ = node2.AuditLog.Close()
		}
	}()

	// Second node also appends its own NODE_LIFECYCLE entry on
	// startup, so height should be height1 + 1.
	want := height1 + 1
	if got := node2.AuditLog.Height(); got != want {
		t.Errorf("replay height = %d, want %d", got, want)
	}
	if idx, err := node2.AuditLog.VerifyChain(); idx != -1 || err != nil {
		t.Errorf("chain verify after reload: idx=%d err=%v", idx, err)
	}

	// Confirm the round-trip marker survived.
	ok := false
	for i := int64(0); i < node2.AuditLog.Height(); i++ {
		e, _ := node2.AuditLog.Get(i)
		if strings.Contains(e.Note, "round-trip marker") {
			ok = true
			break
		}
	}
	if !ok {
		t.Error("round-trip marker entry not found after reload")
	}
}
