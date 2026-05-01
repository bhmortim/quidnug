package core

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestLoadOrCreateNodeKey_PersistsAcrossCalls: ENG-75. The
// per-process keypair must survive a "restart" (second call
// with the same data_dir).
func TestLoadOrCreateNodeKey_PersistsAcrossCalls(t *testing.T) {
	dir := t.TempDir()
	priv1, id1, err := loadOrCreateNodeKey(dir)
	if err != nil {
		t.Fatalf("first load: %v", err)
	}
	if id1 == "" || len(id1) != 16 {
		t.Fatalf("nodeId should be 16 hex chars, got %q", id1)
	}
	keyPath := filepath.Join(dir, "node_key.json")
	if _, err := os.Stat(keyPath); err != nil {
		t.Fatalf("expected node_key.json to exist after first load: %v", err)
	}

	// Second call should load the existing file, not create a new one.
	priv2, id2, err := loadOrCreateNodeKey(dir)
	if err != nil {
		t.Fatalf("second load: %v", err)
	}
	if id1 != id2 {
		t.Fatalf("nodeId changed across reload: %q vs %q", id1, id2)
	}
	if priv1.D.Cmp(priv2.D) != 0 {
		t.Fatal("private scalar changed across reload")
	}
}

// TestLoadOrCreateNodeKey_EphemeralOnEmptyDataDir: when
// DataDir is empty (test mode), fall back to ephemeral
// generation. Exercises the back-compat path.
func TestLoadOrCreateNodeKey_EphemeralOnEmptyDataDir(t *testing.T) {
	priv1, id1, err := loadOrCreateNodeKey("")
	if err != nil {
		t.Fatalf("first call: %v", err)
	}
	priv2, id2, err := loadOrCreateNodeKey("")
	if err != nil {
		t.Fatalf("second call: %v", err)
	}
	// Different keys each call (ephemeral semantics).
	if priv1.D.Cmp(priv2.D) == 0 {
		t.Fatal("ephemeral mode should produce different keys per call")
	}
	if id1 == id2 {
		t.Fatal("ephemeral mode should produce different node ids per call")
	}
}

// TestLoadOrCreateNodeKey_RejectsTamperedID: if the on-disk
// file's nodeId doesn't match the public key derived from
// privateKeyHex, loading must fail. Defends against an
// operator swapping IDs to impersonate.
func TestLoadOrCreateNodeKey_RejectsTamperedID(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := loadOrCreateNodeKey(dir); err != nil {
		t.Fatalf("first load: %v", err)
	}
	// Corrupt the file's nodeId.
	keyPath := filepath.Join(dir, "node_key.json")
	raw, err := os.ReadFile(keyPath)
	if err != nil {
		t.Fatal(err)
	}
	corrupted := strings.Replace(string(raw), `"nodeId":`, `"nodeId":"deadbeef00000000",_dummy:`, 1)
	if err := os.WriteFile(keyPath, []byte(corrupted), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, _, err := loadOrCreateNodeKey(dir); err == nil {
		t.Fatal("expected load to fail on tampered file")
	}
}

// TestSaveLoadBlockchain_RoundTrip: blockchain.json round-trip
// preserves the chain across simulated restart.
func TestSaveLoadBlockchain_RoundTrip(t *testing.T) {
	dir := t.TempDir()
	a := newTestNode()
	// newTestNode initializes a single-block genesis chain.
	// Add a synthetic block.
	a.BlockchainMutex.Lock()
	a.Blockchain = append(a.Blockchain, Block{
		Index:        1,
		Timestamp:    1234567890,
		Transactions: []interface{}{},
		PrevHash:     a.Blockchain[0].Hash,
		Hash:         "deadbeef",
	})
	a.BlockchainMutex.Unlock()

	if err := a.SaveBlockchain(dir); err != nil {
		t.Fatalf("save: %v", err)
	}

	b := newTestNode()
	if err := b.LoadBlockchain(dir); err != nil {
		t.Fatalf("load: %v", err)
	}
	b.BlockchainMutex.RLock()
	got := len(b.Blockchain)
	tip := b.Blockchain[len(b.Blockchain)-1].Index
	b.BlockchainMutex.RUnlock()
	if got != 2 {
		t.Fatalf("expected 2 blocks after reload, got %d", got)
	}
	if tip != 1 {
		t.Fatalf("expected tip index 1, got %d", tip)
	}
}

// TestSaveLoadTrustDomains_RoundTrip: trust_domains.json
// round-trip preserves dynamic domain registrations across
// simulated restart.
func TestSaveLoadTrustDomains_RoundTrip(t *testing.T) {
	dir := t.TempDir()
	a := newTestNode()
	a.TrustDomainsMutex.Lock()
	a.TrustDomains["example.com"] = TrustDomain{
		Name:           "example.com",
		ValidatorNodes: []string{"alice"},
		TrustThreshold: 0.5,
	}
	a.TrustDomainsMutex.Unlock()

	if err := a.SaveTrustDomains(dir); err != nil {
		t.Fatalf("save: %v", err)
	}

	b := newTestNode()
	if err := b.LoadTrustDomains(dir); err != nil {
		t.Fatalf("load: %v", err)
	}
	b.TrustDomainsMutex.RLock()
	_, ok := b.TrustDomains["example.com"]
	b.TrustDomainsMutex.RUnlock()
	if !ok {
		t.Fatal("expected example.com to be present after reload")
	}
}

// TestLoadBlockchain_MissingFileIsSilent: starting fresh with
// no on-disk snapshot is a clean boot, not an error.
func TestLoadBlockchain_MissingFileIsSilent(t *testing.T) {
	dir := t.TempDir()
	n := newTestNode()
	if err := n.LoadBlockchain(dir); err != nil {
		t.Fatalf("expected nil error on missing file, got %v", err)
	}
}

// TestClassifyDialError_DNSTag: ENG-76 classifier produces
// "dns" for the no-such-host case so cycle-summary logs are
// readable.
func TestClassifyDialError_DNSTag(t *testing.T) {
	got := classifyDialError(&dummyErr{"lookup quidnug-node-2 on 127.0.0.11:53: no such host"})
	if got != "dns" {
		t.Fatalf("want dns, got %q", got)
	}
}

func TestClassifyDialError_ConnRefused(t *testing.T) {
	got := classifyDialError(&dummyErr{"dial tcp 127.0.0.1:8080: connect: connection refused"})
	if got != "connection-refused" {
		t.Fatalf("want connection-refused, got %q", got)
	}
}

func TestClassifyDialError_Timeout(t *testing.T) {
	got := classifyDialError(&dummyErr{"context deadline exceeded"})
	if got != "timeout" {
		t.Fatalf("want timeout, got %q", got)
	}
}

func TestClassifyDialError_BlockedRange(t *testing.T) {
	got := classifyDialError(&dummyErr{`safedial: refused address 169.254.169.254 for "metadata" (blocked range)`})
	if got != "blocked-range" {
		t.Fatalf("want blocked-range, got %q", got)
	}
}

type dummyErr struct{ s string }

func (e *dummyErr) Error() string { return e.s }
