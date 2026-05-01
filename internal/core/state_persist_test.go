package core

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
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

// TestENG81_LoadBlockchain_ReplaysTransactions: the headline
// regression for ENG-81. After a restart-style reload, the
// blockchain itself comes back AND the in-memory registries
// (TrustRegistry, IdentityRegistry, NodeAdvertisementRegistry)
// hydrate from the loaded blocks. Pre-fix the chain was
// restored but every registry came back empty, breaking
// /api/v1/registry/trust, /api/v2/discovery/operator/<op>, and
// static peer admission.
func TestENG81_LoadBlockchain_ReplaysTransactions(t *testing.T) {
	dir := t.TempDir()
	a := newTestNode()

	// Build a single block carrying one of each "registry-bearing"
	// transaction type. We construct them as plain Go values and
	// rely on processBlockTransactions's JSON round-trip
	// (json.Marshal + Unmarshal in the type switch) to reach
	// the per-type update*Registry calls — same path the live
	// commit takes.
	const trusterQuid = "1111111111111111"
	const trusteeQuid = "2222222222222222"
	const newIdentityQuid = "3333333333333333"
	const advertisedNodeQuid = "4444444444444444"
	const advertisedOperator = "5555555555555555"

	trustTx := TrustTransaction{
		BaseTransaction: BaseTransaction{
			ID:          "tx-trust-eng81",
			Type:        TxTypeTrust,
			TrustDomain: "test.domain.com",
			Timestamp:   1700000000,
		},
		Truster:    trusterQuid,
		Trustee:    trusteeQuid,
		TrustLevel: 0.85,
		Nonce:      1,
	}

	identityTx := IdentityTransaction{
		BaseTransaction: BaseTransaction{
			ID:          "tx-id-eng81",
			Type:        TxTypeIdentity,
			TrustDomain: "test.domain.com",
			Timestamp:   1700000001,
		},
		QuidID:      newIdentityQuid,
		Name:        "ENG-81 Reload Persona",
		Creator:     trusterQuid,
		UpdateNonce: 1,
	}

	advertisementTx := NodeAdvertisementTransaction{
		BaseTransaction: BaseTransaction{
			ID:          "tx-adv-eng81",
			Type:        TxTypeNodeAdvertisement,
			TrustDomain: "test.domain.com",
			Timestamp:   1700000002,
		},
		NodeQuid:           advertisedNodeQuid,
		OperatorQuid:       advertisedOperator,
		Endpoints:          []NodeEndpoint{{URL: "http://127.0.0.1:9999"}},
		ProtocolVersion:    "1",
		ExpiresAt:          time.Now().Add(24 * time.Hour).UnixNano(),
		AdvertisementNonce: 1,
	}

	a.BlockchainMutex.Lock()
	a.Blockchain = append(a.Blockchain, Block{
		Index:        1,
		Timestamp:    1700000000,
		PrevHash:     a.Blockchain[0].Hash,
		Hash:         "synthetic-block-hash-eng81",
		Transactions: []interface{}{trustTx, identityTx, advertisementTx},
		TrustProof: TrustProof{
			TrustDomain:        "test.domain.com",
			ValidatorID:        a.NodeID,
			ValidatorPublicKey: a.GetPublicKeyHex(),
			ValidationTime:     1700000000,
		},
	})
	a.BlockchainMutex.Unlock()

	if err := a.SaveBlockchain(dir); err != nil {
		t.Fatalf("save: %v", err)
	}

	// Fresh node, clean registries — simulates a process
	// restart where everything is rebuilt from disk.
	b := newTestNode()
	// newTestNode pre-seeds IdentityRegistry with five entries
	// (test fixtures). Drop them so we're checking the replay
	// hydration alone, not the test fixture.
	b.IdentityRegistryMutex.Lock()
	for k := range b.IdentityRegistry {
		delete(b.IdentityRegistry, k)
	}
	b.IdentityRegistryMutex.Unlock()

	if err := b.LoadBlockchain(dir); err != nil {
		t.Fatalf("load: %v", err)
	}

	// The chain itself round-trips (the existing
	// TestSaveLoadBlockchain_RoundTrip already pinned this; we
	// re-check the tail to anchor the rest of the assertions).
	b.BlockchainMutex.RLock()
	chainLen := len(b.Blockchain)
	b.BlockchainMutex.RUnlock()
	if chainLen != 2 {
		t.Fatalf("expected 2 blocks after reload, got %d", chainLen)
	}

	// THE ASSERTIONS: each registry contains the entry it would
	// have if the live commit had just applied this block.

	// TrustRegistry: edge (truster -> trustee) with weight 0.85.
	b.TrustRegistryMutex.RLock()
	trusterMap, hasTruster := b.TrustRegistry[trusterQuid]
	b.TrustRegistryMutex.RUnlock()
	if !hasTruster {
		t.Fatal("TrustRegistry: truster not present after reload (replay missed)")
	}
	if got := trusterMap[trusteeQuid]; got != 0.85 {
		t.Fatalf("TrustRegistry: trust level want 0.85, got %v", got)
	}

	// IdentityRegistry: the new persona quid.
	b.IdentityRegistryMutex.RLock()
	idTx, hasID := b.IdentityRegistry[newIdentityQuid]
	b.IdentityRegistryMutex.RUnlock()
	if !hasID {
		t.Fatal("IdentityRegistry: identity not present after reload")
	}
	if idTx.Name != "ENG-81 Reload Persona" {
		t.Fatalf("IdentityRegistry: name mismatch, got %q", idTx.Name)
	}

	// NodeAdvertisementRegistry: the static-peer entry whose
	// absence is what made admit-static fail with "no current
	// NodeAdvertisement for <peer>".
	if b.NodeAdvertisementRegistry == nil {
		t.Fatal("NodeAdvertisementRegistry: nil after reload (registry not initialized)")
	}
	gotAdv, hasAdv := b.NodeAdvertisementRegistry.Get(advertisedNodeQuid)
	if !hasAdv {
		t.Fatalf("NodeAdvertisementRegistry: advertisement for %s missing after reload", advertisedNodeQuid)
	}
	if gotAdv.OperatorQuid != advertisedOperator {
		t.Fatalf("NodeAdvertisementRegistry: operator quid mismatch, got %q", gotAdv.OperatorQuid)
	}
}

// TestENG81_LoadBlockchain_ReplayIsIdempotent: replaying the
// same blocks twice converges to the same registry state. The
// update*Registry helpers are upserts, so a double-replay (e.g.
// double-load on a misconfigured boot path) doesn't corrupt the
// indices. Per the ticket's "idempotent by construction" claim.
func TestENG81_LoadBlockchain_ReplayIsIdempotent(t *testing.T) {
	dir := t.TempDir()
	a := newTestNode()

	const trusterQuid = "aaaa111111111111"
	const trusteeQuid = "bbbb222222222222"
	trustTx := TrustTransaction{
		BaseTransaction: BaseTransaction{
			ID:          "tx-trust-idemp",
			Type:        TxTypeTrust,
			TrustDomain: "test.domain.com",
			Timestamp:   1700000000,
		},
		Truster:    trusterQuid,
		Trustee:    trusteeQuid,
		TrustLevel: 0.5,
		Nonce:      1,
	}

	a.BlockchainMutex.Lock()
	a.Blockchain = append(a.Blockchain, Block{
		Index:        1,
		Timestamp:    1700000000,
		PrevHash:     a.Blockchain[0].Hash,
		Hash:         "synthetic-block-hash-idemp",
		Transactions: []interface{}{trustTx},
		TrustProof: TrustProof{
			TrustDomain:        "test.domain.com",
			ValidatorID:        a.NodeID,
			ValidatorPublicKey: a.GetPublicKeyHex(),
		},
	})
	a.BlockchainMutex.Unlock()

	if err := a.SaveBlockchain(dir); err != nil {
		t.Fatalf("save: %v", err)
	}

	b := newTestNode()
	if err := b.LoadBlockchain(dir); err != nil {
		t.Fatalf("first load: %v", err)
	}
	if err := b.LoadBlockchain(dir); err != nil {
		t.Fatalf("second load: %v", err)
	}

	// After two loads, the trust edge is present exactly once
	// with the canonical weight.
	b.TrustRegistryMutex.RLock()
	weight := b.TrustRegistry[trusterQuid][trusteeQuid]
	innerLen := len(b.TrustRegistry[trusterQuid])
	b.TrustRegistryMutex.RUnlock()
	if weight != 0.5 {
		t.Fatalf("idempotency: trust weight drifted to %v", weight)
	}
	if innerLen != 1 {
		t.Fatalf("idempotency: expected 1 trustee under truster, got %d", innerLen)
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
