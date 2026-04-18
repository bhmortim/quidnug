// Package core — fork_block_test.go
//
// Methodology
// -----------
// Fork-block migration triggers (QDP-0009 / H5) coordinate
// feature activation across the network via a single signed
// on-chain transaction. These tests guard:
//
//   - Unknown features are rejected. Operators can't smuggle
//     in arbitrary flag names.
//
//   - ForkHeight in the past is rejected (idempotent replay
//     safety + "nothing to activate now" semantics).
//
//   - ForkHeight too soon is rejected (operators need at
//     least MinForkNoticeBlocks of lead time).
//
//   - Validator quorum is enforced per (domain). Fewer valid
//     signatures than required = rejection.
//
//   - Feature activation is idempotent. Applying the same
//     fork twice does not flip the flag back.
//
//   - Activation fires at the block index matching
//     ForkHeight; pre-fork blocks leave the flag unchanged.
//
//   - Supersede logic: a later fork with higher nonce
//     arriving BEFORE the earlier ForkHeight overrides.
//     After the earlier ForkHeight passed, new forks for
//     the same (domain, feature) are rejected.
package core

import (
	"encoding/hex"
	"errors"
	"testing"
	"time"
)

// ----- Helpers -------------------------------------------------------------

// buildForkBlock constructs a fork-block, signs it with the
// given list of validators, and returns the result. Caller is
// responsible for registering the validators' keys in the
// node's ledger before validation.
func buildForkBlock(t *testing.T, signers []*QuidnugNode, domain, feature string, height, nonce int64) ForkBlock {
	t.Helper()
	f := ForkBlock{
		Kind:        AnchorForkBlock,
		TrustDomain: domain,
		Feature:     feature,
		ForkHeight:  height,
		ForkNonce:   nonce,
		ProposedAt:  time.Now().Unix(),
	}
	signable, err := GetForkBlockSignableBytes(f)
	if err != nil {
		t.Fatalf("canonicalize: %v", err)
	}
	for _, s := range signers {
		sig, err := s.SignData(signable)
		if err != nil {
			t.Fatalf("sign: %v", err)
		}
		f.Signatures = append(f.Signatures, ForkSig{
			ValidatorQuid: s.NodeID,
			KeyEpoch:      0,
			Signature:     hex.EncodeToString(sig),
		})
	}
	return f
}

// newForkTestSetup creates a node with a domain and N
// validator nodes whose keys are registered in the ledger.
// Returns the node, the N validator signers, and the domain
// name.
func newForkTestSetup(t *testing.T, domain string, n int) (*QuidnugNode, []*QuidnugNode) {
	t.Helper()
	node := newTestNode()
	signers := make([]*QuidnugNode, 0, n)
	validatorIDs := make([]string, 0, n)
	for i := 0; i < n; i++ {
		v := newTestNode()
		node.NonceLedger.SetSignerKey(v.NodeID, 0, v.GetPublicKeyHex())
		signers = append(signers, v)
		validatorIDs = append(validatorIDs, v.NodeID)
	}
	node.TrustDomainsMutex.Lock()
	node.TrustDomains[domain] = TrustDomain{
		Name:           domain,
		ValidatorNodes: validatorIDs,
	}
	node.TrustDomainsMutex.Unlock()
	return node, signers
}

// ----- Validation: happy path ----------------------------------------------

func TestForkBlock_HappyPathValidates(t *testing.T) {
	node, signers := newForkTestSetup(t, "d.example", 3)
	// 2/3 quorum of 3 = 2. Sign with 2 of 3.
	f := buildForkBlock(t, signers[:2], "d.example", "enable_nonce_ledger", 2000, 1)

	if err := node.ValidateForkBlock(f, 100, time.Now()); err != nil {
		t.Fatalf("want valid, got %v", err)
	}
}

// ----- Validation: rejection paths -----------------------------------------

func TestForkBlock_UnknownFeature(t *testing.T) {
	node, signers := newForkTestSetup(t, "d.example", 3)
	f := buildForkBlock(t, signers[:2], "d.example", "not_a_real_feature", 2000, 1)

	err := node.ValidateForkBlock(f, 100, time.Now())
	if !errors.Is(err, ErrForkUnknownFeature) {
		t.Fatalf("want ErrForkUnknownFeature, got %v", err)
	}
}

func TestForkBlock_HeightInPast(t *testing.T) {
	node, signers := newForkTestSetup(t, "d.example", 3)
	// Height 100 but current head is 500.
	f := buildForkBlock(t, signers[:2], "d.example", "enable_nonce_ledger", 100, 1)

	err := node.ValidateForkBlock(f, 500, time.Now())
	if !errors.Is(err, ErrForkHeightInPast) {
		t.Fatalf("want ErrForkHeightInPast, got %v", err)
	}
}

func TestForkBlock_HeightTooSoon(t *testing.T) {
	node, signers := newForkTestSetup(t, "d.example", 3)
	// Height 1000 but current head is 500; gap is 500 < MinForkNoticeBlocks=1440.
	f := buildForkBlock(t, signers[:2], "d.example", "enable_nonce_ledger", 1000, 1)

	err := node.ValidateForkBlock(f, 500, time.Now())
	if !errors.Is(err, ErrForkHeightTooSoon) {
		t.Fatalf("want ErrForkHeightTooSoon, got %v", err)
	}
}

func TestForkBlock_NonceReplay(t *testing.T) {
	node, signers := newForkTestSetup(t, "d.example", 3)
	f := buildForkBlock(t, signers[:2], "d.example", "enable_nonce_ledger", 2000, 5)

	// First time passes.
	if err := node.ValidateForkBlock(f, 100, time.Now()); err != nil {
		t.Fatalf("first validate: %v", err)
	}
	// Store so nonce is recorded.
	node.forks.storePending(f)

	// Same nonce → replay.
	err := node.ValidateForkBlock(f, 100, time.Now())
	if !errors.Is(err, ErrForkNonceReplay) {
		t.Fatalf("want ErrForkNonceReplay, got %v", err)
	}
	// Lower nonce → also replay.
	f2 := buildForkBlock(t, signers[:2], "d.example", "enable_nonce_ledger", 2500, 3)
	f2.Feature = "enable_nonce_ledger"
	err = node.ValidateForkBlock(f2, 100, time.Now())
	if !errors.Is(err, ErrForkNonceReplay) {
		t.Fatalf("want ErrForkNonceReplay on lower nonce, got %v", err)
	}
}

func TestForkBlock_BelowQuorum(t *testing.T) {
	node, signers := newForkTestSetup(t, "d.example", 5)
	// 5 validators, quorum ceiling(2/3 * 5) = 4. Sign with 3.
	f := buildForkBlock(t, signers[:3], "d.example", "enable_nonce_ledger", 2000, 1)

	err := node.ValidateForkBlock(f, 100, time.Now())
	if !errors.Is(err, ErrForkBelowQuorum) {
		t.Fatalf("want ErrForkBelowQuorum, got %v", err)
	}
}

func TestForkBlock_DuplicateSigner(t *testing.T) {
	node, signers := newForkTestSetup(t, "d.example", 3)
	f := buildForkBlock(t, signers[:2], "d.example", "enable_nonce_ledger", 2000, 1)
	// Duplicate the first signer's signature.
	f.Signatures = append(f.Signatures, f.Signatures[0])

	err := node.ValidateForkBlock(f, 100, time.Now())
	if !errors.Is(err, ErrForkDuplicateSigner) {
		t.Fatalf("want ErrForkDuplicateSigner, got %v", err)
	}
}

// ----- Supersede rules -----------------------------------------------------

// A later fork with higher nonce arriving BEFORE the earlier
// ForkHeight replaces it.
func TestForkBlock_Supersedes(t *testing.T) {
	node, signers := newForkTestSetup(t, "d.example", 3)

	f1 := buildForkBlock(t, signers[:2], "d.example", "enable_nonce_ledger", 3000, 1)
	node.forks.storePending(f1)

	// Supersede with higher nonce, earlier-but-still-valid height.
	f2 := buildForkBlock(t, signers[:2], "d.example", "enable_nonce_ledger", 2500, 2)
	f2.Feature = "enable_nonce_ledger"
	if err := node.ValidateForkBlock(f2, 100, time.Now()); err != nil {
		t.Fatalf("supersede validate: %v", err)
	}
	node.forks.storePending(f2)

	// Only one pending per (domain, feature).
	pending, _ := node.forks.snapshot()
	if pending["d.example"]["enable_nonce_ledger"].ForkHeight != 2500 {
		t.Fatalf("supersede did not replace; got %+v", pending["d.example"]["enable_nonce_ledger"])
	}
}

// After the earlier fork has been activated (ForkHeight
// reached), new forks for the same (domain, feature) are
// rejected.
func TestForkBlock_LateSupersedeRejected(t *testing.T) {
	node, signers := newForkTestSetup(t, "d.example", 3)

	f1 := buildForkBlock(t, signers[:2], "d.example", "enable_nonce_ledger", 3000, 1)
	node.forks.storePending(f1)
	// Activate it.
	node.forks.moveToActive("d.example", "enable_nonce_ledger")

	f2 := buildForkBlock(t, signers[:2], "d.example", "enable_nonce_ledger", 5000, 2)
	f2.Feature = "enable_nonce_ledger"
	err := node.ValidateForkBlock(f2, 3500, time.Now())
	if !errors.Is(err, ErrForkSupersedeTooLate) {
		t.Fatalf("want ErrForkSupersedeTooLate, got %v", err)
	}
}

// ----- Activation ----------------------------------------------------------

// A pending fork whose ForkHeight is reached activates and
// flips the corresponding node flag.
func TestForkBlock_ActivationAtForkHeight(t *testing.T) {
	node, _ := newForkTestSetup(t, "d.example", 3)
	node.NonceLedgerEnforce = false

	f := ForkBlock{
		Kind:        AnchorForkBlock,
		TrustDomain: "d.example",
		Feature:     "enable_nonce_ledger",
		ForkHeight:  100,
		ForkNonce:   1,
	}
	node.forks.storePending(f)

	// Block below height → no activation.
	node.maybeActivateForks("d.example", 99)
	if node.NonceLedgerEnforce {
		t.Fatal("fork should not activate before ForkHeight")
	}

	// Block at/past height → activation.
	node.maybeActivateForks("d.example", 100)
	if !node.NonceLedgerEnforce {
		t.Fatal("fork should activate at ForkHeight")
	}

	// Activation is idempotent — second call doesn't toggle
	// back.
	node.maybeActivateForks("d.example", 101)
	if !node.NonceLedgerEnforce {
		t.Fatal("second activation should not toggle back")
	}
}

// Catch-up scenario: a fork was legitimately committed in the
// past (ForkHeight had proper notice then). When the replaying
// node's chain head has since advanced past ForkHeight,
// subsequent blocks in the same domain trigger
// maybeActivateForks, which flips the flag.
//
// We simulate by directly storePending (the fork was valid at
// its time of submission), then advance past ForkHeight via
// maybeActivateForks.
func TestForkBlock_CatchUpActivationOnReplay(t *testing.T) {
	node, _ := newForkTestSetup(t, "d.example", 3)
	node.NonceLedgerEnforce = false

	// Pre-populate a pending fork as if it had been accepted
	// earlier (replay from storage).
	node.forks.storePending(ForkBlock{
		Kind:        AnchorForkBlock,
		TrustDomain: "d.example",
		Feature:     "enable_nonce_ledger",
		ForkHeight:  2000,
		ForkNonce:   1,
	})

	// Replaying blocks up to 1999 — flag stays off.
	node.maybeActivateForks("d.example", 1999)
	if node.NonceLedgerEnforce {
		t.Fatal("flag should not flip before ForkHeight during replay")
	}
	// First block at/past ForkHeight triggers activation.
	node.maybeActivateForks("d.example", 2000)
	if !node.NonceLedgerEnforce {
		t.Fatal("catch-up should activate at ForkHeight")
	}
}

// ----- Feature activation helpers ------------------------------------------

// activateFeature flips the right flag per feature name.
func TestActivateFeature_EachFlag(t *testing.T) {
	node := newTestNode()

	cases := []struct {
		feature string
		check   func() bool
	}{
		{"enable_nonce_ledger", func() bool { return node.NonceLedgerEnforce }},
		{"enable_push_gossip", func() bool { return node.PushGossipEnabled }},
		{"enable_lazy_epoch_probe", func() bool { return node.LazyEpochProbeEnabled }},
	}
	for _, c := range cases {
		t.Run(c.feature, func(t *testing.T) {
			node.NonceLedgerEnforce = false
			node.PushGossipEnabled = false
			node.LazyEpochProbeEnabled = false
			node.activateFeature(c.feature)
			if !c.check() {
				t.Fatalf("feature %q did not flip its flag", c.feature)
			}
		})
	}
}

// ----- Validator quorum ----------------------------------------------------

func TestValidatorQuorum_TwoThirdsCeiling(t *testing.T) {
	cases := []struct {
		n    int
		want int
	}{
		{1, 1}, // 2/3 of 1 = 0 → floor 1
		{2, 2}, // 2/3 of 2 = 1.33 → 2
		{3, 2}, // 2/3 of 3 = 2
		{4, 3}, // 2/3 of 4 = 2.67 → 3
		{5, 4}, // 2/3 of 5 = 3.33 → 4
		{6, 4}, // 2/3 of 6 = 4
		{7, 5}, // 2/3 of 7 = 4.67 → 5
		{10, 7},
	}
	for _, c := range cases {
		node := newTestNode()
		vids := make([]string, c.n)
		for i := range vids {
			vids[i] = "v" + string(rune('a'+i))
		}
		node.TrustDomains["d"] = TrustDomain{
			Name:           "d",
			ValidatorNodes: vids,
		}
		got := node.validatorQuorumForDomain("d")
		if got != c.want {
			t.Fatalf("n=%d: want quorum %d, got %d", c.n, c.want, got)
		}
	}
}
