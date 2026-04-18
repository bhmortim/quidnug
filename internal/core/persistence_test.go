// Package core — persistence_test.go
//
// Methodology
// -----------
// Tests cover Save/Load/Clear for pending transactions, the on-disk
// state the node persists across restarts.
//
//   - Round-trip: save with one transaction, load into a fresh node,
//     verify PendingTxs has the same count.
//   - No-op on empty: saving an empty PendingTxs does NOT create a
//     file (documented behavior; prevents empty-file clutter).
//   - Missing file on Load is not an error: a fresh node should
//     start cleanly without a pre-existing state file.
//   - Malformed JSON on Load is a hard error.
//   - Clear is idempotent: clearing twice must not error.
//
// All tests use t.TempDir() so file I/O is automatically cleaned up
// and cannot interfere with other tests or persist between runs.
package core

import (
	"os"
	"path/filepath"
	"testing"
)

func TestPersistence_RoundTrip(t *testing.T) {
	dir := t.TempDir()

	saver := newTestNode()
	saver.PendingTxs = []interface{}{
		TrustTransaction{
			BaseTransaction: BaseTransaction{
				ID:          "tx-1",
				Type:        TxTypeTrust,
				TrustDomain: "test.domain.com",
				Timestamp:   1000,
			},
			Truster:    "aaaaaaaaaaaaaaaa",
			Trustee:    "bbbbbbbbbbbbbbbb",
			TrustLevel: 0.5,
			Nonce:      1,
		},
	}

	if err := saver.SavePendingTransactions(dir); err != nil {
		t.Fatalf("SavePendingTransactions: %v", err)
	}

	path := filepath.Join(dir, "pending_transactions.json")
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("expected persistence file at %s, got %v", path, err)
	}

	loader := newTestNode()
	loader.PendingTxs = nil
	if err := loader.LoadPendingTransactions(dir); err != nil {
		t.Fatalf("LoadPendingTransactions: %v", err)
	}
	if got := len(loader.PendingTxs); got != 1 {
		t.Fatalf("expected 1 transaction loaded, got %d", got)
	}
}

func TestSavePendingTransactions_NoopOnEmpty(t *testing.T) {
	dir := t.TempDir()
	node := newTestNode()
	node.PendingTxs = nil

	if err := node.SavePendingTransactions(dir); err != nil {
		t.Fatalf("SavePendingTransactions on empty: %v", err)
	}

	path := filepath.Join(dir, "pending_transactions.json")
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("expected no file to exist; got err=%v", err)
	}
}

func TestLoadPendingTransactions_MissingFileIsNoOp(t *testing.T) {
	node := newTestNode()
	node.PendingTxs = []interface{}{"should remain"}
	if err := node.LoadPendingTransactions(t.TempDir()); err != nil {
		t.Fatalf("LoadPendingTransactions of missing file: %v", err)
	}
	if len(node.PendingTxs) != 1 {
		t.Fatalf("missing file should not modify PendingTxs; got %v", node.PendingTxs)
	}
}

func TestLoadPendingTransactions_RejectsMalformedJSON(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "pending_transactions.json")
	if err := os.WriteFile(path, []byte("{not valid json}"), 0o600); err != nil {
		t.Fatalf("seed malformed file: %v", err)
	}

	node := newTestNode()
	err := node.LoadPendingTransactions(dir)
	if err == nil {
		t.Fatal("expected error on malformed JSON")
	}
}

func TestClearPendingTransactionsFile_RemovesFile(t *testing.T) {
	dir := t.TempDir()

	node := newTestNode()
	node.PendingTxs = []interface{}{
		TrustTransaction{BaseTransaction: BaseTransaction{ID: "x"}},
	}
	if err := node.SavePendingTransactions(dir); err != nil {
		t.Fatalf("save: %v", err)
	}

	if err := node.ClearPendingTransactionsFile(dir); err != nil {
		t.Fatalf("clear: %v", err)
	}
	path := filepath.Join(dir, "pending_transactions.json")
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("expected file to be removed; got err=%v", err)
	}

	// Removing twice must not error.
	if err := node.ClearPendingTransactionsFile(dir); err != nil {
		t.Fatalf("clear twice: %v", err)
	}
}
