// QDP-0012 Phase 1 state-extension tests.
//
// Phase 1 is intentionally a no-behavior-change addition: new
// fields on TrustDomain with omitempty tags, populated at
// registration with a single-registrant fallback. These tests
// verify:
//
//   - Role helpers (IsGovernor / IsConsortiumMember / Role)
//   - Quorum-weight computation
//   - JSON round-trip preserves / omits new fields per spec
//   - RegisterTrustDomain installs the fallback governance set
//   - Explicit multi-governor declarations pass through
//
// Phase 2 (DOMAIN_GOVERNANCE transaction) and Phase 3 (fork-
// activated enforcement) build on this state without further
// struct changes.
package core

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestTrustDomain_Role_CacheReplicaIsDefault(t *testing.T) {
	td := TrustDomain{Name: "x.example"}
	if got := td.Role("random-quid"); got != DomainRoleCacheReplica {
		t.Errorf("expected cache-replica for unknown quid, got %q", got)
	}
}

func TestTrustDomain_Role_ConsortiumMember(t *testing.T) {
	td := TrustDomain{
		Name:       "x.example",
		Validators: map[string]float64{"node-1": 1.0, "node-2": 0.0},
	}
	if got := td.Role("node-1"); got != DomainRoleConsortiumMember {
		t.Errorf("weighted validator should be consortium-member, got %q", got)
	}
	// Zero-weight entry should demote to cache-replica.
	if got := td.Role("node-2"); got != DomainRoleCacheReplica {
		t.Errorf("zero-weight validator should be cache-replica, got %q", got)
	}
}

func TestTrustDomain_Role_GovernorWinsOverValidator(t *testing.T) {
	td := TrustDomain{
		Name:       "x.example",
		Validators: map[string]float64{"both": 1.0},
		Governors:  map[string]float64{"both": 1.0},
	}
	if got := td.Role("both"); got != DomainRoleGovernor {
		t.Errorf("governor should take precedence, got %q", got)
	}
}

func TestTrustDomain_IsGovernor_ZeroWeight(t *testing.T) {
	td := TrustDomain{Governors: map[string]float64{"x": 0}}
	if td.IsGovernor("x") {
		t.Error("zero-weight governor should not count")
	}
}

func TestTrustDomain_GovernorQuorumWeight_Default(t *testing.T) {
	td := TrustDomain{
		Governors: map[string]float64{"a": 1.0, "b": 1.0},
		// GovernanceQuorum zero — treated as 1.0 (unanimous).
	}
	if got := td.GovernorQuorumWeight(); got != 2.0 {
		t.Errorf("unanimous 2-governor domain should need weight 2.0, got %f", got)
	}
}

func TestTrustDomain_GovernorQuorumWeight_TwoThirds(t *testing.T) {
	td := TrustDomain{
		Governors:        map[string]float64{"a": 1.0, "b": 1.0, "c": 1.0},
		GovernanceQuorum: 0.67,
	}
	got := td.GovernorQuorumWeight()
	// 3.0 * 0.67 == 2.01
	if got < 2.009 || got > 2.011 {
		t.Errorf("2/3 of 3 governors should be ~2.01, got %f", got)
	}
}

func TestTrustDomain_GovernorQuorumWeight_NoGovernorsReturnsZero(t *testing.T) {
	td := TrustDomain{Name: "legacy.example"}
	if got := td.GovernorQuorumWeight(); got != 0 {
		t.Errorf("domain with no governors should need weight 0, got %f", got)
	}
}

// TestTrustDomain_JSON_OmitsEmpty confirms that a pre-QDP-0012
// domain (zero governance fields) serializes without the new
// keys — essential for round-trip compatibility with nodes that
// predate Phase 1.
func TestTrustDomain_JSON_OmitsEmpty(t *testing.T) {
	td := TrustDomain{
		Name:           "legacy.example",
		ValidatorNodes: []string{"n1"},
		TrustThreshold: 0.5,
		Validators:     map[string]float64{"n1": 1.0},
	}
	data, err := json.Marshal(td)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	out := string(data)
	for _, key := range []string{
		`"governors"`, `"governorPublicKeys"`, `"governanceQuorum"`,
		`"governanceNonce"`, `"parentDelegationMode"`, `"delegatedFrom"`,
	} {
		if strings.Contains(out, key) {
			t.Errorf("empty governance field %s should be omitted from JSON, got %s", key, out)
		}
	}
}

// TestTrustDomain_JSON_RoundTripWithGovernance confirms that a
// Phase 1 domain survives a full marshal/unmarshal with every
// new field intact.
func TestTrustDomain_JSON_RoundTripWithGovernance(t *testing.T) {
	td := TrustDomain{
		Name:             "reviews.public",
		ValidatorNodes:   []string{"s1", "s2"},
		TrustThreshold:   0.5,
		Validators:       map[string]float64{"s1": 1.0, "s2": 1.0},
		Governors:        map[string]float64{"p1": 1.0, "p2": 1.0},
		GovernorPublicKeys: map[string]string{
			"p1": "deadbeef",
			"p2": "cafef00d",
		},
		GovernanceQuorum:     0.67,
		GovernanceNonce:      42,
		ParentDelegationMode: DelegationModeSelf,
	}

	data, err := json.Marshal(td)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var round TrustDomain
	if err := json.Unmarshal(data, &round); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if round.Name != td.Name {
		t.Errorf("name mismatch: %q vs %q", round.Name, td.Name)
	}
	if round.GovernanceQuorum != td.GovernanceQuorum {
		t.Errorf("quorum mismatch: %f vs %f", round.GovernanceQuorum, td.GovernanceQuorum)
	}
	if round.GovernanceNonce != td.GovernanceNonce {
		t.Errorf("nonce mismatch: %d vs %d", round.GovernanceNonce, td.GovernanceNonce)
	}
	if round.Governors["p1"] != 1.0 || round.Governors["p2"] != 1.0 {
		t.Errorf("governors mismatch: %+v", round.Governors)
	}
	if round.GovernorPublicKeys["p1"] != "deadbeef" {
		t.Errorf("governor pubkey lost: %+v", round.GovernorPublicKeys)
	}
	if round.ParentDelegationMode != DelegationModeSelf {
		t.Errorf("delegation mode lost: %q", round.ParentDelegationMode)
	}
}

// TestTrustDomain_JSON_UnmarshalsOldFormatCleanly verifies that
// pre-Phase-1 JSON (without the governance keys) unmarshals
// without error and sees zero values on the new fields. This
// is the critical backward-compat guarantee.
func TestTrustDomain_JSON_UnmarshalsOldFormatCleanly(t *testing.T) {
	legacy := `{
		"name": "legacy.example",
		"validatorNodes": ["n1"],
		"trustThreshold": 0.5,
		"blockchainHead": "",
		"validators": {"n1": 1.0},
		"validatorPublicKeys": {"n1": "deadbeef"}
	}`
	var td TrustDomain
	if err := json.Unmarshal([]byte(legacy), &td); err != nil {
		t.Fatalf("legacy JSON should unmarshal without error: %v", err)
	}
	if td.Governors != nil {
		t.Errorf("Governors should be nil, got %+v", td.Governors)
	}
	if td.GovernanceNonce != 0 {
		t.Errorf("GovernanceNonce should default to 0, got %d", td.GovernanceNonce)
	}
	if td.ParentDelegationMode != "" {
		t.Errorf("ParentDelegationMode should default to empty, got %q", td.ParentDelegationMode)
	}
	// Role helpers must still behave sensibly on a domain with no
	// governors declared.
	if td.IsGovernor("n1") {
		t.Error("legacy domain has no governors; IsGovernor should be false")
	}
	if td.Role("n1") != DomainRoleConsortiumMember {
		t.Errorf("n1 is a validator; Role should be consortium-member, got %q", td.Role("n1"))
	}
}

// TestRegisterTrustDomain_AppliesGovernanceFallback confirms
// the Phase 1 backward-compat default: a registration without
// explicit governors installs the registering node as the sole
// governor with unanimous quorum.
func TestRegisterTrustDomain_AppliesGovernanceFallback(t *testing.T) {
	node := newTestNode()
	node.AllowDomainRegistration = true
	node.SupportedDomains = nil // empty list => all domains supported

	err := node.RegisterTrustDomain(TrustDomain{
		Name:           "new.example.com",
		TrustThreshold: 0.5,
		ValidatorNodes: []string{node.NodeID},
	})
	if err != nil {
		t.Fatalf("register: %v", err)
	}

	stored := node.TrustDomains["new.example.com"]
	if len(stored.Governors) != 1 {
		t.Fatalf("expected single-governor fallback, got %+v", stored.Governors)
	}
	if w := stored.Governors[node.NodeID]; w != 1.0 {
		t.Errorf("self-governor weight should be 1.0, got %f", w)
	}
	if stored.GovernanceQuorum != 1.0 {
		t.Errorf("default quorum should be 1.0 (unanimous), got %f", stored.GovernanceQuorum)
	}
	if stored.ParentDelegationMode != DelegationModeSelf {
		t.Errorf("default delegation mode should be self, got %q", stored.ParentDelegationMode)
	}
	if stored.GovernorPublicKeys[node.NodeID] == "" {
		t.Error("self-governor should have a public key installed")
	}
	if !stored.IsGovernor(node.NodeID) {
		t.Error("registrant should be recognized as a governor")
	}
}

// TestRegisterTrustDomain_HonorsExplicitGovernors confirms that
// callers supplying an explicit governor set don't get
// overwritten by the fallback.
func TestRegisterTrustDomain_HonorsExplicitGovernors(t *testing.T) {
	node := newTestNode()
	node.AllowDomainRegistration = true
	node.SupportedDomains = nil // empty list => all domains supported

	err := node.RegisterTrustDomain(TrustDomain{
		Name:           "multi-gov.example.com",
		TrustThreshold: 0.5,
		ValidatorNodes: []string{node.NodeID},
		Governors: map[string]float64{
			node.NodeID: 1.0,
			"cofounder": 1.0,
		},
		GovernorPublicKeys: map[string]string{
			"cofounder": "cafef00d",
		},
		GovernanceQuorum: 0.5,
	})
	if err != nil {
		t.Fatalf("register: %v", err)
	}

	stored := node.TrustDomains["multi-gov.example.com"]
	if len(stored.Governors) != 2 {
		t.Errorf("expected 2 governors, got %d", len(stored.Governors))
	}
	if stored.GovernanceQuorum != 0.5 {
		t.Errorf("caller's quorum should be preserved, got %f", stored.GovernanceQuorum)
	}
	// The registrant's own public key must have been filled in even
	// when the caller supplied GovernorPublicKeys for other quids.
	if stored.GovernorPublicKeys[node.NodeID] == "" {
		t.Error("registrant self-pubkey should be auto-filled alongside explicit set")
	}
	if stored.GovernorPublicKeys["cofounder"] != "cafef00d" {
		t.Error("explicit co-founder pubkey should be preserved")
	}
}
