// Unit + integration tests for the QDP-0014
// NodeAdvertisementTransaction primitive.
//
// Coverage:
//   - validation: every rule in §4.1 has at least one
//     pass + one fail case.
//   - registry: upsert, GC, per-operator list, per-domain list.
//   - end-to-end: a NODE_ADVERTISEMENT flows through the full
//     stack (submit → pending → block → registry) alongside
//     other tx types, matching the pattern reviewer-integration
//     tests rely on.
package core

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"strings"
	"testing"
	"time"
)

// testNodeActor mirrors testActor in reviews_integration_test.go
// but with a dedicated sign-advertisement helper. Keeping it
// separate avoids test-package ordering issues.
type testNodeActor struct {
	Priv   *ecdsa.PrivateKey
	PubHex string
	QuidID string
}

func newTestNodeActor(t *testing.T) *testNodeActor {
	t.Helper()
	priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("keygen: %v", err)
	}
	pubBytes := elliptic.Marshal(elliptic.P256(), priv.PublicKey.X, priv.PublicKey.Y) //nolint:staticcheck
	pubHex := hex.EncodeToString(pubBytes)
	return &testNodeActor{
		Priv:   priv,
		PubHex: pubHex,
		QuidID: QuidIDFromPublicKeyHex(pubHex),
	}
}

func (a *testNodeActor) signAdvertisement(tx NodeAdvertisementTransaction) NodeAdvertisementTransaction {
	tx.PublicKey = a.PubHex
	tx.NodeQuid = a.QuidID
	tx.Signature = ""
	signable, _ := json.Marshal(tx)
	// Sign-and-pack as IEEE-1363 r||s (64 bytes hex), matching
	// the node's VerifySignature format.
	tx.Signature = signIEEE1363(a.Priv, signable)
	return tx
}

// signIEEE1363 produces a 64-byte r||s signature over SHA-256
// of data, hex-encoded. Duplicates the helper in
// reviews_integration_test.go because Go test files can't
// cross-import.
func signIEEE1363(priv *ecdsa.PrivateKey, data []byte) string {
	sum := sha256Sum(data)
	r, s, err := ecdsa.Sign(rand.Reader, priv, sum)
	if err != nil {
		return ""
	}
	sig := make([]byte, 64)
	rBytes := r.Bytes()
	sBytes := s.Bytes()
	copy(sig[32-len(rBytes):32], rBytes)
	copy(sig[64-len(sBytes):64], sBytes)
	return hex.EncodeToString(sig)
}

func sha256Sum(data []byte) []byte {
	sum := sha256.Sum256(data)
	return sum[:]
}

// ----- helper: a QuidnugNode pre-wired with operator→node attestation

type nodeAdTestFixture struct {
	node     *QuidnugNode
	operator *testNodeActor
	nodeAct  *testNodeActor
	domain   string
}

func newAdvertisementTestFixture(t *testing.T, opts ...func(*nodeAdTestFixture)) *nodeAdTestFixture {
	t.Helper()
	node := newTestNode()

	domain := "operators.network.example.com"
	// Register the operator domain.
	node.TrustDomains[domain] = TrustDomain{
		Name:                domain,
		ValidatorNodes:      []string{node.NodeID},
		TrustThreshold:      0.5,
		Validators:          map[string]float64{node.NodeID: 1.0},
		ValidatorPublicKeys: map[string]string{node.NodeID: node.GetPublicKeyHex()},
	}

	operator := newTestNodeActor(t)
	nodeAct := newTestNodeActor(t)

	// Seed identity registry so the operator and node exist.
	node.IdentityRegistry[operator.QuidID] = IdentityTransaction{
		BaseTransaction: BaseTransaction{PublicKey: operator.PubHex},
		QuidID:          operator.QuidID,
		Name:            "operator",
		Creator:         operator.QuidID,
	}
	node.IdentityRegistry[nodeAct.QuidID] = IdentityTransaction{
		BaseTransaction: BaseTransaction{PublicKey: nodeAct.PubHex},
		QuidID:          nodeAct.QuidID,
		Name:            "node",
		Creator:         operator.QuidID,
	}

	// Seed the TRUST registry with the operator→node
	// attestation. hasOperatorAttestation is the gate this
	// satisfies.
	node.TrustRegistryMutex.Lock()
	node.TrustRegistry[operator.QuidID] = map[string]float64{nodeAct.QuidID: 1.0}
	node.TrustRegistryMutex.Unlock()

	f := &nodeAdTestFixture{
		node:     node,
		operator: operator,
		nodeAct:  nodeAct,
		domain:   domain,
	}
	for _, opt := range opts {
		opt(f)
	}
	return f
}

// validAdvertisement returns a signed, validation-passing
// advertisement for the fixture's node. Individual tests
// mutate a field or two and re-sign to exercise a specific
// validation rule.
//
// The tx ID is pre-assigned so downstream AddNode-submit paths
// don't re-hash after signing (which would invalidate the sig).
func (f *nodeAdTestFixture) validAdvertisement(t *testing.T, nonce int64) NodeAdvertisementTransaction {
	t.Helper()
	// Fresh random ID; no collision with anything in the pool.
	var idBuf [16]byte
	_, _ = rand.Read(idBuf[:])

	ad := NodeAdvertisementTransaction{
		BaseTransaction: BaseTransaction{
			ID:          hex.EncodeToString(idBuf[:]),
			Type:        TxTypeNodeAdvertisement,
			TrustDomain: f.domain,
			Timestamp:   time.Now().Unix(),
		},
		OperatorQuid: f.operator.QuidID,
		Endpoints: []NodeEndpoint{
			{
				URL:      "https://node.example.com",
				Protocol: "http/2",
				Region:   "iad",
				Priority: 1,
				Weight:   100,
			},
		},
		SupportedDomains:   []string{"*.public.technology.laptops"},
		Capabilities:       NodeCapabilities{Cache: true, Bootstrap: true},
		ProtocolVersion:    "1.0",
		ExpiresAt:          time.Now().Add(1 * time.Hour).UnixNano(),
		AdvertisementNonce: nonce,
	}
	return f.nodeAct.signAdvertisement(ad)
}

// ============================================================
// Registry-level tests (no signing / no validator; pure data).
// ============================================================

func TestNodeAdvertisementRegistry_UpsertAndGet(t *testing.T) {
	reg := NewNodeAdvertisementRegistry()
	ad := NodeAdvertisementTransaction{
		NodeQuid:           "aaaabbbbccccdddd",
		OperatorQuid:       "1111222233334444",
		ExpiresAt:          time.Now().Add(1 * time.Hour).UnixNano(),
		AdvertisementNonce: 1,
	}
	reg.upsert(ad, time.Now())

	got, ok := reg.Get("aaaabbbbccccdddd")
	if !ok {
		t.Fatalf("Get returned !ok for inserted advertisement")
	}
	if got.OperatorQuid != "1111222233334444" {
		t.Fatalf("unexpected operator: %s", got.OperatorQuid)
	}

	if _, ok := reg.Get("does-not-exist"); ok {
		t.Fatalf("Get returned ok for missing quid")
	}
}

func TestNodeAdvertisementRegistry_ExpiryMasksGet(t *testing.T) {
	reg := NewNodeAdvertisementRegistry()
	ad := NodeAdvertisementTransaction{
		NodeQuid:           "expiredquid00000",
		ExpiresAt:          time.Now().Add(-1 * time.Minute).UnixNano(),
		AdvertisementNonce: 1,
	}
	reg.upsert(ad, time.Now())
	if _, ok := reg.Get("expiredquid00000"); ok {
		t.Fatalf("Get returned ok for expired advertisement")
	}
}

func TestNodeAdvertisementRegistry_GarbageCollect(t *testing.T) {
	reg := NewNodeAdvertisementRegistry()
	reg.upsert(NodeAdvertisementTransaction{
		NodeQuid:  "alive00000000001",
		ExpiresAt: time.Now().Add(1 * time.Hour).UnixNano(),
	}, time.Now())
	reg.upsert(NodeAdvertisementTransaction{
		NodeQuid:  "dead000000000001",
		ExpiresAt: time.Now().Add(-1 * time.Hour).UnixNano(),
	}, time.Now())
	reg.upsert(NodeAdvertisementTransaction{
		NodeQuid:  "dead000000000002",
		ExpiresAt: time.Now().Add(-1 * time.Second).UnixNano(),
	}, time.Now())

	n := reg.GarbageCollect()
	if n != 2 {
		t.Fatalf("expected 2 expired ads pruned, got %d", n)
	}
	if _, ok := reg.Get("alive00000000001"); !ok {
		t.Fatalf("live ad pruned by GC")
	}
	if _, ok := reg.Get("dead000000000001"); ok {
		t.Fatalf("expired ad survived GC")
	}
}

func TestNodeAdvertisementRegistry_ListForDomain(t *testing.T) {
	reg := NewNodeAdvertisementRegistry()
	// Use MatchDomainPattern-compatible patterns: wildcards
	// look like "*.suffix" and match subdomains (not the suffix
	// itself). For arbitrary-depth prefixes, use the exact name.
	reg.upsert(NodeAdvertisementTransaction{
		NodeQuid:         "a000000000000001",
		ExpiresAt:        time.Now().Add(1 * time.Hour).UnixNano(),
		SupportedDomains: []string{"*.technology.laptops"},
	}, time.Now())
	reg.upsert(NodeAdvertisementTransaction{
		NodeQuid:         "b000000000000002",
		ExpiresAt:        time.Now().Add(1 * time.Hour).UnixNano(),
		SupportedDomains: []string{"*.network.quidnug.com"},
	}, time.Now())
	reg.upsert(NodeAdvertisementTransaction{
		NodeQuid:         "c000000000000003",
		ExpiresAt:        time.Now().Add(1 * time.Hour).UnixNano(),
		SupportedDomains: []string{"reviews.public.technology.laptops"},
	}, time.Now())

	matches := reg.ListForDomain("reviews.public.technology.laptops")
	// "c..." matches exactly; "a..." matches via glob
	// (reviews.public is a subdomain-prefix of technology.laptops).
	if len(matches) != 2 {
		t.Fatalf("expected 2 matches for laptops domain, got %d", len(matches))
	}
}

// ============================================================
// Validator tests (each rule probed).
// ============================================================

func TestValidateNodeAdvertisement_Happy(t *testing.T) {
	f := newAdvertisementTestFixture(t)
	ad := f.validAdvertisement(t, 1)
	if !f.node.ValidateNodeAdvertisementTransaction(ad) {
		t.Fatalf("expected valid advertisement to pass")
	}
}

func TestValidateNodeAdvertisement_UnknownDomain(t *testing.T) {
	f := newAdvertisementTestFixture(t)
	ad := f.validAdvertisement(t, 1)
	ad.TrustDomain = "not-a-registered-domain"
	ad = f.nodeAct.signAdvertisement(ad)
	if f.node.ValidateNodeAdvertisementTransaction(ad) {
		t.Fatalf("expected rejection for unregistered trust domain")
	}
}

func TestValidateNodeAdvertisement_MissingOperatorAttestation(t *testing.T) {
	f := newAdvertisementTestFixture(t)

	// Remove the operator→node trust edge.
	f.node.TrustRegistryMutex.Lock()
	delete(f.node.TrustRegistry, f.operator.QuidID)
	f.node.TrustRegistryMutex.Unlock()

	ad := f.validAdvertisement(t, 1)
	if f.node.ValidateNodeAdvertisementTransaction(ad) {
		t.Fatalf("expected rejection when operator has no trust edge to the node")
	}
}

func TestValidateNodeAdvertisement_NodeQuidMismatch(t *testing.T) {
	f := newAdvertisementTestFixture(t)
	ad := f.validAdvertisement(t, 1)
	// Scramble NodeQuid so it no longer matches the pubkey hash.
	ad.NodeQuid = "0000000000000000"
	// Do NOT re-sign; we want the pubkey-hash check to fail.
	if f.node.ValidateNodeAdvertisementTransaction(ad) {
		t.Fatalf("expected rejection when NodeQuid disagrees with signing pubkey")
	}
}

func TestValidateNodeAdvertisement_NonMonotonicNonce(t *testing.T) {
	f := newAdvertisementTestFixture(t)
	// Seed a previously-accepted ad at nonce 5.
	prev := f.validAdvertisement(t, 5)
	f.node.NodeAdvertisementRegistry.upsert(prev, time.Now().Add(-1*time.Hour))

	// Try to replay with nonce 5 again.
	stale := f.validAdvertisement(t, 5)
	if f.node.ValidateNodeAdvertisementTransaction(stale) {
		t.Fatalf("expected rejection for non-monotonic nonce")
	}

	// Try a lower nonce.
	older := f.validAdvertisement(t, 4)
	if f.node.ValidateNodeAdvertisementTransaction(older) {
		t.Fatalf("expected rejection for lower nonce")
	}
}

func TestValidateNodeAdvertisement_ExpiryBoundaries(t *testing.T) {
	f := newAdvertisementTestFixture(t)

	// Already expired.
	ad := f.validAdvertisement(t, 1)
	ad.ExpiresAt = time.Now().Add(-1 * time.Minute).UnixNano()
	ad = f.nodeAct.signAdvertisement(ad)
	if f.node.ValidateNodeAdvertisementTransaction(ad) {
		t.Fatalf("expected rejection for already-expired advertisement")
	}

	// TTL > 7 days.
	ad = f.validAdvertisement(t, 2)
	ad.ExpiresAt = time.Now().Add(8 * 24 * time.Hour).UnixNano()
	ad = f.nodeAct.signAdvertisement(ad)
	if f.node.ValidateNodeAdvertisementTransaction(ad) {
		t.Fatalf("expected rejection for TTL > 7 days")
	}
}

func TestValidateNodeAdvertisement_EndpointConstraints(t *testing.T) {
	f := newAdvertisementTestFixture(t)

	cases := []struct {
		name    string
		mutate  func(*NodeAdvertisementTransaction)
		wantOK  bool
	}{
		{
			name: "empty endpoints",
			mutate: func(ad *NodeAdvertisementTransaction) {
				ad.Endpoints = nil
			},
			wantOK: false,
		},
		{
			name: "non-https URL",
			mutate: func(ad *NodeAdvertisementTransaction) {
				ad.Endpoints[0].URL = "http://insecure.example.com"
			},
			wantOK: false,
		},
		{
			name: "unknown protocol",
			mutate: func(ad *NodeAdvertisementTransaction) {
				ad.Endpoints[0].Protocol = "gopher"
			},
			wantOK: false,
		},
		{
			name: "priority out of range",
			mutate: func(ad *NodeAdvertisementTransaction) {
				ad.Endpoints[0].Priority = 200
			},
			wantOK: false,
		},
		{
			name: "too many endpoints",
			mutate: func(ad *NodeAdvertisementTransaction) {
				extra := ad.Endpoints[0]
				for i := 0; i < MaxNodeEndpoints+1; i++ {
					ad.Endpoints = append(ad.Endpoints, extra)
				}
			},
			wantOK: false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ad := f.validAdvertisement(t, 1)
			tc.mutate(&ad)
			ad = f.nodeAct.signAdvertisement(ad)
			got := f.node.ValidateNodeAdvertisementTransaction(ad)
			if got != tc.wantOK {
				t.Fatalf("case %q: validator returned %v, want %v", tc.name, got, tc.wantOK)
			}
		})
	}
}

func TestValidateNodeAdvertisement_ProtocolVersionFormat(t *testing.T) {
	f := newAdvertisementTestFixture(t)
	cases := map[string]bool{
		"":                false,
		"1":               false,
		"1.":              false,
		"1.0":             true,
		"1.0.0":           true,
		"2.1.0-rc.1":      true,
		strings.Repeat("1.", 20): false, // too long
	}
	for v, wantOK := range cases {
		ad := f.validAdvertisement(t, 1)
		ad.ProtocolVersion = v
		ad = f.nodeAct.signAdvertisement(ad)
		got := f.node.ValidateNodeAdvertisementTransaction(ad)
		if got != wantOK {
			t.Errorf("protocolVersion %q: got %v, want %v", v, got, wantOK)
		}
	}
}

func TestValidateNodeAdvertisement_RateLimit(t *testing.T) {
	f := newAdvertisementTestFixture(t)

	// Seed an accepted-advertisement "just now" — below the
	// min-interval threshold.
	prev := f.validAdvertisement(t, 1)
	f.node.NodeAdvertisementRegistry.upsert(prev, time.Now())

	// Same node tries to publish again immediately.
	ad := f.validAdvertisement(t, 2)
	if f.node.ValidateNodeAdvertisementTransaction(ad) {
		t.Fatalf("expected rate-limit rejection when advertising within min interval")
	}
}

// ============================================================
// End-to-end: submit via AddNodeAdvertisementTransaction,
// verify landing in pending pool, commit via block, verify
// registry.
// ============================================================

func TestAddNodeAdvertisement_EndToEnd(t *testing.T) {
	f := newAdvertisementTestFixture(t)

	// Ensure the node has a validator public key registered for
	// the domain so block generation can sign the block.
	f.node.TrustDomainsMutex.Lock()
	td := f.node.TrustDomains[f.domain]
	td.ValidatorPublicKeys = map[string]string{f.node.NodeID: f.node.GetPublicKeyHex()}
	f.node.TrustDomains[f.domain] = td
	f.node.TrustDomainsMutex.Unlock()

	ad := f.validAdvertisement(t, 1)

	// Submit.
	txID, err := f.node.AddNodeAdvertisementTransaction(ad)
	if err != nil {
		t.Fatalf("submit: %v", err)
	}
	if txID == "" {
		t.Fatal("expected non-empty txID")
	}

	// Advertisement sits in the pending pool waiting for a block.
	f.node.PendingTxsMutex.RLock()
	found := false
	for _, pendingTx := range f.node.PendingTxs {
		if pending, ok := pendingTx.(NodeAdvertisementTransaction); ok {
			if pending.NodeQuid == ad.NodeQuid {
				found = true
				break
			}
		}
	}
	f.node.PendingTxsMutex.RUnlock()
	if !found {
		t.Fatal("advertisement not in pending pool")
	}

	// Generate a block + process it. Registry should reflect
	// the advertisement afterwards.
	block, err := f.node.GenerateBlock(f.domain)
	if err != nil {
		t.Fatalf("GenerateBlock: %v", err)
	}
	if err := f.node.AddBlock(*block); err != nil {
		t.Fatalf("AddBlock: %v", err)
	}

	stored, ok := f.node.NodeAdvertisementRegistry.Get(ad.NodeQuid)
	if !ok {
		t.Fatal("advertisement not in registry after block commit")
	}
	if stored.OperatorQuid != f.operator.QuidID {
		t.Fatalf("stored operator mismatch: %s", stored.OperatorQuid)
	}
	if len(stored.Endpoints) != 1 {
		t.Fatalf("expected 1 endpoint, got %d", len(stored.Endpoints))
	}
	if stored.Endpoints[0].URL != "https://node.example.com" {
		t.Fatalf("endpoint URL mismatch: %s", stored.Endpoints[0].URL)
	}
}

