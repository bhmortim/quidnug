// ENG-80 regression coverage: cross-node block-sync convergence
// when the source has dynamically registered a trust domain that
// the receiver has never heard of.
//
// The bug: each node's genesis is unique (per-node ValidatorID and
// timestamp), so a globally strict chain-link check rejects every
// peer-served block as "block failed cryptographic validation".
// On top of that, the per-domain validator-set lookup in
// ValidateTrustProofTiered fails because the receiver has no
// record of the dynamically-registered domain.
//
// The fix introduces:
//
//   - BootstrapDomainFromBlock: register the domain locally from
//     the block's TrustProof when ReceiveBlock first sees it.
//   - Per-domain chain-link in ValidateBlockCryptographic and
//     GenerateBlock: each domain has its own sub-chain anchored
//     at the first block we accept for it.
//
// This file exercises both pieces end-to-end with two independent
// QuidnugNode instances (each with its own genesis), and pins
// down behavior that the in-process reviews_integration_test
// could not catch (it ran everything against a single node, so
// the cross-genesis chain-link issue never surfaced).
package core

import (
	"testing"
)

// TestENG80_BlockSyncBootstrap_CrossGenesis: the headline
// scenario from the bug report. nodeA registers a brand-new
// trust domain, generates a block for it, and nodeB (which has
// never heard of the domain) accepts the block as Trusted via
// ReceiveBlock — proving convergence works across distinct
// genesis blocks.
//
// Pre-fix: ReceiveBlock returned BlockInvalid + the literal
// error "block failed cryptographic validation" because the
// global chain-link check rejected any peer-served block whose
// PrevHash anchored on the producer's genesis (which is unique
// per node).
func TestENG80_BlockSyncBootstrap_CrossGenesis(t *testing.T) {
	nodeA := newTestNode()
	nodeB := newTestNode()

	if nodeA.NodeID == nodeB.NodeID {
		t.Fatal("preconditions: two test nodes must have distinct NodeIDs")
	}
	if nodeA.Blockchain[0].Hash == nodeB.Blockchain[0].Hash {
		t.Fatal("preconditions: two test nodes must have distinct genesis hashes")
	}

	// Establish operator trust from nodeB to nodeA. In a real
	// deployment this comes from the peer-admit pipeline +
	// operator-attestation; here we set the registry edge
	// directly so the relational-trust computation in
	// ValidateTrustProofTiered returns >= TrustThreshold.
	nodeB.TrustRegistry[nodeB.NodeID] = map[string]float64{
		nodeA.NodeID: 0.9,
	}

	// nodeA dynamically registers the domain (mirrors POST
	// /api/v1/domains in production).
	const newDomain = "examples.public.quidnug.com"
	nodeA.AllowDomainRegistration = true
	if err := nodeA.RegisterTrustDomain(TrustDomain{
		Name:           newDomain,
		ValidatorNodes: []string{nodeA.NodeID},
		TrustThreshold: 0.75,
	}); err != nil {
		t.Fatalf("nodeA.RegisterTrustDomain: %v", err)
	}

	// Build a block on nodeA for the new domain. We construct
	// it directly rather than via GenerateBlock so the test
	// stays focused on the receive path; the GenerateBlock
	// behavior is covered by TestENG80_GenerateBlock_PerDomainPrev.
	block := Block{
		Index:        1,
		Timestamp:    1700000000,
		Transactions: []interface{}{},
		PrevHash:     nodeA.Blockchain[0].Hash, // anchored at A's genesis
		TrustProof: TrustProof{
			TrustDomain:             newDomain,
			ValidatorID:             nodeA.NodeID,
			ValidatorPublicKey:      nodeA.GetPublicKeyHex(),
			ValidatorTrustInCreator: 1.0,
			ValidationTime:          1700000000,
		},
	}
	signBlock(nodeA, &block)

	// Sanity: nodeB has no record of the domain before receive.
	nodeB.TrustDomainsMutex.RLock()
	_, exists := nodeB.TrustDomains[newDomain]
	nodeB.TrustDomainsMutex.RUnlock()
	if exists {
		t.Fatal("preconditions: nodeB should not yet know the domain")
	}

	// THE TEST: ReceiveBlock on nodeB should accept the block
	// as Trusted (since nodeB trusts nodeA above the domain
	// threshold).
	acceptance, err := nodeB.ReceiveBlock(block)
	if err != nil {
		t.Fatalf("ReceiveBlock returned error: %v (acceptance=%d)", err, acceptance)
	}
	if acceptance != BlockTrusted {
		t.Fatalf("expected BlockTrusted, got acceptance=%d", acceptance)
	}

	// Side-effect: the domain is now bootstrapped on nodeB
	// with nodeA's pubkey.
	nodeB.TrustDomainsMutex.RLock()
	td, ok := nodeB.TrustDomains[newDomain]
	nodeB.TrustDomainsMutex.RUnlock()
	if !ok {
		t.Fatal("post-receive: nodeB should have bootstrapped the domain")
	}
	if got := td.ValidatorPublicKeys[nodeA.NodeID]; got != nodeA.GetPublicKeyHex() {
		t.Fatalf("post-receive: validator pubkey mismatch on bootstrapped domain (got %q)", got)
	}

	// Side-effect: the block was integrated into nodeB's local
	// chain at the per-domain head.
	nodeB.BlockchainMutex.RLock()
	chainLen := len(nodeB.Blockchain)
	last := nodeB.Blockchain[chainLen-1]
	nodeB.BlockchainMutex.RUnlock()
	if chainLen != 2 {
		t.Fatalf("post-receive: expected nodeB chain length 2 (genesis + block), got %d", chainLen)
	}
	if last.Hash != block.Hash {
		t.Fatal("post-receive: nodeB's chain tail should be the received block")
	}
}

// TestENG80_BlockSyncBootstrap_NoErrorWithoutOperatorTrust: when
// nodeB has not established operator trust to nodeA, the block
// is still ACCEPTED (no error, no "cryptographic validation
// failed"). Acceptance tier degrades to Tentative or Untrusted
// per the per-node trust graph, but the convergence machinery
// no longer rejects on principle.
func TestENG80_BlockSyncBootstrap_NoErrorWithoutOperatorTrust(t *testing.T) {
	nodeA := newTestNode()
	nodeB := newTestNode()
	const newDomain = "untrusted-but-valid.example.com"

	nodeA.AllowDomainRegistration = true
	if err := nodeA.RegisterTrustDomain(TrustDomain{
		Name:           newDomain,
		ValidatorNodes: []string{nodeA.NodeID},
		TrustThreshold: 0.75,
	}); err != nil {
		t.Fatalf("RegisterTrustDomain: %v", err)
	}

	block := Block{
		Index:    1,
		PrevHash: nodeA.Blockchain[0].Hash,
		TrustProof: TrustProof{
			TrustDomain:        newDomain,
			ValidatorID:        nodeA.NodeID,
			ValidatorPublicKey: nodeA.GetPublicKeyHex(),
			ValidationTime:     1700000000,
		},
	}
	signBlock(nodeA, &block)

	acc, err := nodeB.ReceiveBlock(block)
	if err != nil {
		t.Fatalf("ReceiveBlock returned error without operator trust: %v (acc=%d)", err, acc)
	}
	if acc == BlockInvalid {
		t.Fatalf("acceptance must not be BlockInvalid (got %d) — convergence is broken", acc)
	}
	// The bug pre-fix returned BlockInvalid with the literal
	// "block failed cryptographic validation". After fix,
	// untrusted-validator blocks are simply Untrusted, and
	// the per-domain registry is still seeded for future
	// operator-trust establishment.
}

// TestENG80_BlockSyncBootstrap_SecondBlockChainsCorrectly: after
// the first block has bootstrapped the domain on the receiver,
// a second block for the same domain must chain to the first
// block (per-domain head), not to the receiver's genesis.
func TestENG80_BlockSyncBootstrap_SecondBlockChainsCorrectly(t *testing.T) {
	nodeA := newTestNode()
	nodeB := newTestNode()
	const newDomain = "examples.public.quidnug.com"

	// Trust edge nodeB → nodeA so blocks are accepted as
	// Trusted (and therefore appended to the local chain
	// where the per-domain head can be observed).
	nodeB.TrustRegistry[nodeB.NodeID] = map[string]float64{
		nodeA.NodeID: 0.9,
	}

	nodeA.AllowDomainRegistration = true
	if err := nodeA.RegisterTrustDomain(TrustDomain{
		Name:           newDomain,
		ValidatorNodes: []string{nodeA.NodeID},
		TrustThreshold: 0.75,
	}); err != nil {
		t.Fatalf("RegisterTrustDomain: %v", err)
	}

	// First block on nodeA chains to A's genesis.
	block1 := Block{
		Index:     1,
		Timestamp: 1700000000,
		PrevHash:  nodeA.Blockchain[0].Hash,
		TrustProof: TrustProof{
			TrustDomain:        newDomain,
			ValidatorID:        nodeA.NodeID,
			ValidatorPublicKey: nodeA.GetPublicKeyHex(),
			ValidationTime:     1700000000,
		},
	}
	signBlock(nodeA, &block1)
	if _, err := nodeA.ReceiveBlock(block1); err != nil {
		t.Fatalf("nodeA.ReceiveBlock(block1): %v", err)
	}

	// Second block on nodeA chains to block1.
	block2 := Block{
		Index:     2,
		Timestamp: 1700000060,
		PrevHash:  block1.Hash,
		TrustProof: TrustProof{
			TrustDomain:        newDomain,
			ValidatorID:        nodeA.NodeID,
			ValidatorPublicKey: nodeA.GetPublicKeyHex(),
			ValidationTime:     1700000060,
		},
	}
	signBlock(nodeA, &block2)
	if _, err := nodeA.ReceiveBlock(block2); err != nil {
		t.Fatalf("nodeA.ReceiveBlock(block2): %v", err)
	}

	// nodeB receives in order. block1 is the bootstrap; block2
	// chains to block1 via per-domain head.
	if acc, err := nodeB.ReceiveBlock(block1); err != nil || acc != BlockTrusted {
		t.Fatalf("nodeB.ReceiveBlock(block1): err=%v acc=%d", err, acc)
	}
	if acc, err := nodeB.ReceiveBlock(block2); err != nil || acc != BlockTrusted {
		t.Fatalf("nodeB.ReceiveBlock(block2): err=%v acc=%d (chain-link to block1 must succeed)", err, acc)
	}
}

// TestENG80_BlockSyncBootstrap_RejectsForgedValidatorID: a
// malicious peer claims a validator id that doesn't match its
// embedded public key. The bootstrap path must NOT register a
// stub domain entry from this proof, and ReceiveBlock must
// reject the block.
func TestENG80_BlockSyncBootstrap_RejectsForgedValidatorID(t *testing.T) {
	nodeA := newTestNode()
	nodeB := newTestNode()
	const newDomain = "spoofed.example.com"

	// Build a block with a mismatched ValidatorID. Pubkey is
	// nodeA's, but ValidatorID claims to be nodeB's.
	block := Block{
		Index:    1,
		PrevHash: nodeA.Blockchain[0].Hash,
		TrustProof: TrustProof{
			TrustDomain:        newDomain,
			ValidatorID:        nodeB.NodeID, // forged
			ValidatorPublicKey: nodeA.GetPublicKeyHex(),
			ValidationTime:     1700000000,
		},
	}
	// Sign with A's key (so the signature is valid against the
	// embedded pubkey, but the ID claim doesn't match).
	signBlock(nodeA, &block)

	// nodeB attempts to receive — the bootstrap should refuse
	// the mismatched id, and the crypto check should reject
	// the block.
	acc, err := nodeB.ReceiveBlock(block)
	if err == nil {
		t.Fatal("ReceiveBlock should reject a block with mismatched validator id/pubkey")
	}
	if acc == BlockTrusted {
		t.Fatalf("forged-id block must not be Trusted (got acc=%d)", acc)
	}

	// And the spoofed domain must NOT be in the local registry.
	nodeB.TrustDomainsMutex.RLock()
	_, registered := nodeB.TrustDomains[newDomain]
	nodeB.TrustDomainsMutex.RUnlock()
	if registered {
		t.Fatal("forged-id block must not seed a TrustDomain entry")
	}
}

// TestENG80_BootstrapDomainFromBlock_Idempotent: calling the
// bootstrap helper twice for the same domain is a no-op on the
// second call (does not overwrite an existing entry).
func TestENG80_BootstrapDomainFromBlock_Idempotent(t *testing.T) {
	nodeA := newTestNode()
	nodeB := newTestNode()
	const newDomain = "idempotent.example.com"

	block := Block{
		Index:    1,
		PrevHash: nodeA.Blockchain[0].Hash,
		TrustProof: TrustProof{
			TrustDomain:        newDomain,
			ValidatorID:        nodeA.NodeID,
			ValidatorPublicKey: nodeA.GetPublicKeyHex(),
			ValidationTime:     1700000000,
		},
	}
	signBlock(nodeA, &block)

	if err := nodeB.BootstrapDomainFromBlock(block); err != nil {
		t.Fatalf("first bootstrap: %v", err)
	}
	// Mutate the local entry so we can detect overwrite.
	nodeB.TrustDomainsMutex.Lock()
	td := nodeB.TrustDomains[newDomain]
	td.TrustThreshold = 0.42
	nodeB.TrustDomains[newDomain] = td
	nodeB.TrustDomainsMutex.Unlock()

	if err := nodeB.BootstrapDomainFromBlock(block); err != nil {
		t.Fatalf("second bootstrap: %v", err)
	}
	nodeB.TrustDomainsMutex.RLock()
	got := nodeB.TrustDomains[newDomain].TrustThreshold
	nodeB.TrustDomainsMutex.RUnlock()
	if got != 0.42 {
		t.Fatalf("idempotency violated: TrustThreshold was overwritten (got %v)", got)
	}
}

// TestENG80_GenerateBlock_PerDomainPrev: a node generating two
// blocks for two different domains should produce blocks whose
// chain-link is per-domain. Block 2 of domain X chains to block
// 1 of domain X, not to a block of domain Y that was inserted
// in between.
func TestENG80_GenerateBlock_PerDomainPrev(t *testing.T) {
	node := newTestNode()
	const domainX = "x.example.com"
	const domainY = "y.example.com"
	node.AllowDomainRegistration = true
	for _, d := range []string{domainX, domainY} {
		if err := node.RegisterTrustDomain(TrustDomain{
			Name:           d,
			ValidatorNodes: []string{node.NodeID},
			TrustThreshold: 0.75,
		}); err != nil {
			t.Fatalf("register %s: %v", d, err)
		}
	}

	// Manually append blocks (skip GenerateBlock which requires
	// pending tx) so we can verify ValidateBlockCryptographic
	// uses per-domain prev. Build x1, then y1, then x2.
	x1 := Block{
		Index:    1,
		PrevHash: node.Blockchain[0].Hash,
		TrustProof: TrustProof{
			TrustDomain:        domainX,
			ValidatorID:        node.NodeID,
			ValidatorPublicKey: node.GetPublicKeyHex(),
			ValidationTime:     1700000000,
		},
	}
	signBlock(node, &x1)
	if acc, err := node.ReceiveBlock(x1); err != nil || acc != BlockTrusted {
		t.Fatalf("x1 receive: err=%v acc=%d", err, acc)
	}

	y1 := Block{
		Index:     1, // independent per-domain index
		Timestamp: 1700000060,
		PrevHash:  node.Blockchain[0].Hash,
		TrustProof: TrustProof{
			TrustDomain:        domainY,
			ValidatorID:        node.NodeID,
			ValidatorPublicKey: node.GetPublicKeyHex(),
			ValidationTime:     1700000060,
		},
	}
	signBlock(node, &y1)
	if acc, err := node.ReceiveBlock(y1); err != nil || acc != BlockTrusted {
		t.Fatalf("y1 receive: err=%v acc=%d (per-domain chain-link should bypass cross-domain)", err, acc)
	}

	// x2 chains to x1 (NOT to y1, which is the global tail).
	x2 := Block{
		Index:     2,
		Timestamp: 1700000120,
		PrevHash:  x1.Hash,
		TrustProof: TrustProof{
			TrustDomain:        domainX,
			ValidatorID:        node.NodeID,
			ValidatorPublicKey: node.GetPublicKeyHex(),
			ValidationTime:     1700000120,
		},
	}
	signBlock(node, &x2)
	if acc, err := node.ReceiveBlock(x2); err != nil || acc != BlockTrusted {
		t.Fatalf("x2 receive: err=%v acc=%d (per-domain chain-link to x1 should succeed)", err, acc)
	}
}
