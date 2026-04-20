// QDP-0015 moderation tests. Coverage spans:
//
//   - Validation — every field rule from §4.2 has a pass + fail case.
//   - Registry — upsert, supersede, per-target actionsFor.
//   - Effective scope — composition rules: max-severity, supersede
//     walk, effective-range windows.
//   - Filter — the actual HTTP serving paths honor the scopes.
package core

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

// newModerationTestFixture wires up a QuidnugNode whose
// default test.domain.com has the given actor registered as a
// validator — i.e. the actor is an authorized moderator under
// phase-1 semantics.
func newModerationTestFixture(t *testing.T) (*QuidnugNode, *testNodeActor) {
	t.Helper()
	node := newTestNode()
	actor := newTestNodeActor(t)

	// Promote the actor to a validator on the default test
	// domain so ValidateModerationActionTransaction's §4.11
	// authority check passes.
	td := node.TrustDomains["test.domain.com"]
	td.Validators[actor.QuidID] = 1.0
	td.ValidatorNodes = append(td.ValidatorNodes, actor.QuidID)
	node.TrustDomains["test.domain.com"] = td

	return node, actor
}

// signModeration populates PublicKey / ModeratorQuid and a
// valid IEEE-1363 signature over the canonicalized tx.
func (a *testNodeActor) signModeration(tx ModerationActionTransaction) ModerationActionTransaction {
	tx.PublicKey = a.PubHex
	tx.ModeratorQuid = a.QuidID
	tx.Signature = ""
	signable, _ := json.Marshal(tx)
	tx.Signature = signIEEE1363(a.Priv, signable)
	return tx
}

// baselineAction returns a well-formed tx with the minimum
// fields needed to pass validation. Tests mutate one field at
// a time to target specific rules.
func baselineAction(actor *testNodeActor, nonce int64) ModerationActionTransaction {
	return ModerationActionTransaction{
		BaseTransaction: BaseTransaction{
			Type:        TxTypeModerationAction,
			TrustDomain: "test.domain.com",
			Timestamp:   time.Now().Unix(),
		},
		TargetType:  ModerationTargetTx,
		TargetID:    strings.Repeat("a", 64), // 64-char hex, trivially valid
		Scope:       ModerationScopeHide,
		ReasonCode:  ReasonCodeSpam, // evidence not required
		Nonce:       nonce,
	}
}

// --- validation (§4.2) --------------------------------------------------

func TestValidateModerationAction_Baseline(t *testing.T) {
	node, actor := newModerationTestFixture(t)
	tx := actor.signModeration(baselineAction(actor, 1))
	if !node.ValidateModerationActionTransaction(tx) {
		t.Fatal("baseline valid action unexpectedly rejected")
	}
}

func TestValidateModerationAction_RejectsUnknownDomain(t *testing.T) {
	node, actor := newModerationTestFixture(t)
	tx := baselineAction(actor, 1)
	tx.TrustDomain = "nonexistent.example.com"
	tx = actor.signModeration(tx)
	if node.ValidateModerationActionTransaction(tx) {
		t.Error("action in unknown domain should be rejected")
	}
}

func TestValidateModerationAction_RejectsMismatchedPubkey(t *testing.T) {
	node, actor := newModerationTestFixture(t)
	other := newTestNodeActor(t)
	tx := baselineAction(actor, 1)
	tx = actor.signModeration(tx)
	// Tamper: swap pubkey to a different actor's — quid/pubkey
	// mismatch now.
	tx.PublicKey = other.PubHex
	if node.ValidateModerationActionTransaction(tx) {
		t.Error("action with mismatched quid/pubkey should be rejected")
	}
}

func TestValidateModerationAction_RejectsBadScope(t *testing.T) {
	node, actor := newModerationTestFixture(t)
	tx := baselineAction(actor, 1)
	tx.Scope = "annihilate"
	tx = actor.signModeration(tx)
	if node.ValidateModerationActionTransaction(tx) {
		t.Error("action with bogus scope should be rejected")
	}
}

func TestValidateModerationAction_RejectsBadReason(t *testing.T) {
	node, actor := newModerationTestFixture(t)
	tx := baselineAction(actor, 1)
	tx.ReasonCode = "BECAUSE_I_SAID_SO"
	tx = actor.signModeration(tx)
	if node.ValidateModerationActionTransaction(tx) {
		t.Error("action with unknown reason code should be rejected")
	}
}

func TestValidateModerationAction_DMCARequiresEvidence(t *testing.T) {
	node, actor := newModerationTestFixture(t)
	tx := baselineAction(actor, 1)
	tx.ReasonCode = ReasonCodeDMCA
	tx.EvidenceURL = ""
	tx = actor.signModeration(tx)
	if node.ValidateModerationActionTransaction(tx) {
		t.Error("DMCA action without EvidenceURL should be rejected")
	}
}

func TestValidateModerationAction_DMCAWithEvidenceAccepted(t *testing.T) {
	node, actor := newModerationTestFixture(t)
	tx := baselineAction(actor, 1)
	tx.ReasonCode = ReasonCodeDMCA
	tx.EvidenceURL = "https://example.com/dmca/123"
	tx = actor.signModeration(tx)
	if !node.ValidateModerationActionTransaction(tx) {
		t.Error("DMCA action with EvidenceURL should be accepted")
	}
}

func TestValidateModerationAction_RejectsBadTxTargetID(t *testing.T) {
	node, actor := newModerationTestFixture(t)
	tx := baselineAction(actor, 1)
	tx.TargetID = "not-hex-and-too-short"
	tx = actor.signModeration(tx)
	if node.ValidateModerationActionTransaction(tx) {
		t.Error("action with non-hex TX target should be rejected")
	}
}

func TestValidateModerationAction_QuidTarget(t *testing.T) {
	node, actor := newModerationTestFixture(t)
	tx := baselineAction(actor, 1)
	tx.TargetType = ModerationTargetQuid
	tx.TargetID = "0000000000000001" // valid 16-char quid
	tx = actor.signModeration(tx)
	if !node.ValidateModerationActionTransaction(tx) {
		t.Error("QUID target with valid id should be accepted")
	}

	tx2 := baselineAction(actor, 2)
	tx2.TargetType = ModerationTargetQuid
	tx2.TargetID = "too-short"
	tx2 = actor.signModeration(tx2)
	if node.ValidateModerationActionTransaction(tx2) {
		t.Error("QUID target with invalid id should be rejected")
	}
}

func TestValidateModerationAction_AnnotationCap(t *testing.T) {
	node, actor := newModerationTestFixture(t)
	tx := baselineAction(actor, 1)
	tx.Scope = ModerationScopeAnnotate
	tx.AnnotationText = strings.Repeat("x", MaxAnnotationTextLength+1)
	tx = actor.signModeration(tx)
	if node.ValidateModerationActionTransaction(tx) {
		t.Error("over-long annotation should be rejected")
	}
}

func TestValidateModerationAction_AnnotationControlChars(t *testing.T) {
	node, actor := newModerationTestFixture(t)
	tx := baselineAction(actor, 1)
	tx.Scope = ModerationScopeAnnotate
	tx.AnnotationText = "hello\x00world" // NUL is forbidden
	tx = actor.signModeration(tx)
	if node.ValidateModerationActionTransaction(tx) {
		t.Error("annotation with control chars should be rejected")
	}
}

func TestValidateModerationAction_NonceMonotonic(t *testing.T) {
	node, actor := newModerationTestFixture(t)

	// Seed with nonce=5.
	first := actor.signModeration(baselineAction(actor, 5))
	if !node.ValidateModerationActionTransaction(first) {
		t.Fatal("first action (nonce=5) should validate")
	}
	node.updateModerationRegistry(first)

	// Replay: nonce=5 again must fail.
	replay := actor.signModeration(baselineAction(actor, 5))
	if node.ValidateModerationActionTransaction(replay) {
		t.Error("replay with same nonce should be rejected")
	}

	// Lower nonce must fail.
	lower := actor.signModeration(baselineAction(actor, 4))
	if node.ValidateModerationActionTransaction(lower) {
		t.Error("lower nonce should be rejected")
	}

	// Higher nonce accepted.
	higher := actor.signModeration(baselineAction(actor, 6))
	if !node.ValidateModerationActionTransaction(higher) {
		t.Error("higher nonce should be accepted")
	}
}

func TestValidateModerationAction_UnauthorizedModerator(t *testing.T) {
	node := newTestNode()
	actor := newTestNodeActor(t) // NOT a validator on test.domain.com
	tx := actor.signModeration(baselineAction(actor, 1))
	if node.ValidateModerationActionTransaction(tx) {
		t.Error("non-validator moderator should be rejected")
	}
}

func TestValidateModerationAction_DelegatedAuthorityAccepted(t *testing.T) {
	node, validator := newModerationTestFixture(t)
	delegate := newTestNodeActor(t)

	// Validator delegates to the new actor at weight 0.7.
	node.TrustRegistryMutex.Lock()
	if node.TrustRegistry[validator.QuidID] == nil {
		node.TrustRegistry[validator.QuidID] = make(map[string]float64)
	}
	node.TrustRegistry[validator.QuidID][delegate.QuidID] = 0.7
	node.TrustRegistryMutex.Unlock()

	tx := delegate.signModeration(baselineAction(delegate, 1))
	if !node.ValidateModerationActionTransaction(tx) {
		t.Error("delegated moderator at >=0.7 should be accepted")
	}
}

func TestValidateModerationAction_DelegatedAuthorityBelowThreshold(t *testing.T) {
	node, validator := newModerationTestFixture(t)
	delegate := newTestNodeActor(t)

	node.TrustRegistryMutex.Lock()
	if node.TrustRegistry[validator.QuidID] == nil {
		node.TrustRegistry[validator.QuidID] = make(map[string]float64)
	}
	// 0.5 is below the 0.7 delegation threshold.
	node.TrustRegistry[validator.QuidID][delegate.QuidID] = 0.5
	node.TrustRegistryMutex.Unlock()

	tx := delegate.signModeration(baselineAction(delegate, 1))
	if node.ValidateModerationActionTransaction(tx) {
		t.Error("delegated moderator below 0.7 should be rejected")
	}
}

func TestValidateModerationAction_SupersedeChainSameModerator(t *testing.T) {
	node, actor := newModerationTestFixture(t)

	first := actor.signModeration(baselineAction(actor, 1))
	if !node.ValidateModerationActionTransaction(first) {
		t.Fatal("first action should validate")
	}
	node.updateModerationRegistry(first)

	// Precompute an ID for the supersede link; validator doesn't
	// require it to match any particular format except "exists
	// in registry."
	first.ID = "first-action-id"
	node.updateModerationRegistry(first)

	second := baselineAction(actor, 2)
	second.SupersedesTxID = first.ID
	second = actor.signModeration(second)
	if !node.ValidateModerationActionTransaction(second) {
		t.Error("supersede to same moderator's prior action should validate")
	}
}

func TestValidateModerationAction_SupersedeForeignActionRejected(t *testing.T) {
	node, actor := newModerationTestFixture(t)
	other := newTestNodeActor(t)

	// Grant delegation to `other` so both are authorized.
	node.TrustRegistryMutex.Lock()
	if node.TrustRegistry[actor.QuidID] == nil {
		node.TrustRegistry[actor.QuidID] = make(map[string]float64)
	}
	node.TrustRegistry[actor.QuidID][other.QuidID] = 0.9
	node.TrustRegistryMutex.Unlock()

	foreign := other.signModeration(baselineAction(other, 1))
	foreign.ID = "foreign-id"
	node.updateModerationRegistry(foreign)

	attempt := baselineAction(actor, 2)
	attempt.SupersedesTxID = "foreign-id"
	attempt = actor.signModeration(attempt)
	if node.ValidateModerationActionTransaction(attempt) {
		t.Error("supersede across moderators should be rejected")
	}
}

// --- scope composition --------------------------------------------------

func TestComputeEffectiveScope_EmptyReturnsZero(t *testing.T) {
	if got := computeEffectiveScope(nil, time.Now().Unix()); got.Scope != "" {
		t.Errorf("empty should yield zero scope, got %+v", got)
	}
}

func TestComputeEffectiveScope_SuppressBeatsHide(t *testing.T) {
	now := time.Now().Unix()
	actions := []ModerationActionTransaction{
		{BaseTransaction: BaseTransaction{ID: "a"}, Scope: ModerationScopeHide, ReasonCode: ReasonCodeSpam},
		{BaseTransaction: BaseTransaction{ID: "b"}, Scope: ModerationScopeSuppress, ReasonCode: ReasonCodeDMCA},
	}
	scope := computeEffectiveScope(actions, now)
	if scope.Scope != ModerationScopeSuppress {
		t.Errorf("expected suppress, got %q", scope.Scope)
	}
	if scope.ReasonCode != ReasonCodeDMCA {
		t.Errorf("expected DMCA reason, got %q", scope.ReasonCode)
	}
}

func TestComputeEffectiveScope_SupersededNotCounted(t *testing.T) {
	now := time.Now().Unix()
	actions := []ModerationActionTransaction{
		{BaseTransaction: BaseTransaction{ID: "a"}, Scope: ModerationScopeSuppress, ReasonCode: ReasonCodeDMCA},
		{BaseTransaction: BaseTransaction{ID: "b"}, Scope: ModerationScopeAnnotate,
			ReasonCode: ReasonCodeMisinformation, SupersedesTxID: "a",
			AnnotationText: "resolved"},
	}
	scope := computeEffectiveScope(actions, now)
	if scope.Scope != ModerationScopeAnnotate {
		t.Errorf("expected annotate after supersede, got %q", scope.Scope)
	}
	if scope.AnnotationText != "resolved" {
		t.Errorf("expected annotation 'resolved', got %q", scope.AnnotationText)
	}
}

func TestComputeEffectiveScope_EffectiveRange(t *testing.T) {
	future := time.Now().Unix() + 3600
	past := time.Now().Unix() - 3600
	now := time.Now().Unix()

	actions := []ModerationActionTransaction{
		// Not yet active.
		{BaseTransaction: BaseTransaction{ID: "a"}, Scope: ModerationScopeSuppress,
			ReasonCode: ReasonCodeDMCA, EffectiveFrom: future},
		// Already expired.
		{BaseTransaction: BaseTransaction{ID: "b"}, Scope: ModerationScopeSuppress,
			ReasonCode: ReasonCodeDMCA, EffectiveUntil: past},
		// Currently active.
		{BaseTransaction: BaseTransaction{ID: "c"}, Scope: ModerationScopeHide,
			ReasonCode: ReasonCodeSpam},
	}
	scope := computeEffectiveScope(actions, now)
	if scope.Scope != ModerationScopeHide {
		t.Errorf("expected hide (only currently-active action), got %q", scope.Scope)
	}
}

// --- registry + serving -------------------------------------------------

func TestModerationRegistry_UpsertIdempotent(t *testing.T) {
	r := NewModerationRegistry()
	tx := ModerationActionTransaction{
		BaseTransaction: BaseTransaction{ID: "dup"},
		ModeratorQuid:   "mod-1",
		TargetType:      ModerationTargetTx,
		TargetID:        "t1",
		Nonce:           1,
	}
	r.upsert(tx)
	r.upsert(tx) // replay: should not duplicate
	acts := r.actionsFor(ModerationTargetTx, "t1")
	if len(acts) != 1 {
		t.Errorf("expected 1 action after replay, got %d", len(acts))
	}
}

func TestEffectiveScopeFor_UnmoderatedTarget(t *testing.T) {
	node, _ := newModerationTestFixture(t)
	scope := node.EffectiveScopeFor(ModerationTargetTx, "unknown")
	if scope.Scope != "" {
		t.Errorf("unmoderated target should have empty scope, got %+v", scope)
	}
}

func TestIsTargetSuppressed(t *testing.T) {
	node, actor := newModerationTestFixture(t)
	tx := baselineAction(actor, 1)
	tx.Scope = ModerationScopeSuppress
	tx.ReasonCode = ReasonCodeDMCA
	tx.EvidenceURL = "https://example/dmca"
	tx.TargetID = strings.Repeat("b", 64)
	tx = actor.signModeration(tx)
	node.updateModerationRegistry(tx)

	if !node.IsTargetSuppressed(ModerationTargetTx, tx.TargetID) {
		t.Error("expected target to be suppressed after suppress action committed")
	}
	if node.IsTargetSuppressed(ModerationTargetTx, "other-target") {
		t.Error("untargeted id should not be suppressed")
	}
}

// --- filterModeratedEvents ---------------------------------------------

func TestFilterModeratedEvents_SuppressDrops(t *testing.T) {
	node, actor := newModerationTestFixture(t)

	events := []EventTransaction{
		{BaseTransaction: BaseTransaction{ID: "keep-1"}, EventType: "x"},
		{BaseTransaction: BaseTransaction{ID: "drop-1"}, EventType: "x"},
		{BaseTransaction: BaseTransaction{ID: "keep-2"}, EventType: "x"},
	}

	tx := baselineAction(actor, 1)
	tx.Scope = ModerationScopeSuppress
	tx.ReasonCode = ReasonCodeCSAM
	tx.EvidenceURL = "https://ncmec/report/123"
	tx.TargetType = ModerationTargetTx
	tx.TargetID = strings.Repeat("d", 64) // id we'll set into event
	events[1].ID = tx.TargetID
	tx = actor.signModeration(tx)
	node.updateModerationRegistry(tx)

	out := node.filterModeratedEvents(events, false)
	if len(out) != 2 {
		t.Fatalf("expected 2 after suppress, got %d", len(out))
	}
	for _, ev := range out {
		if ev.ID == tx.TargetID {
			t.Error("suppressed event should have been dropped")
		}
	}
}

func TestFilterModeratedEvents_HideRespectsOverride(t *testing.T) {
	node, actor := newModerationTestFixture(t)

	hiddenID := strings.Repeat("e", 64)
	events := []EventTransaction{
		{BaseTransaction: BaseTransaction{ID: "visible"}, EventType: "x"},
		{BaseTransaction: BaseTransaction{ID: hiddenID}, EventType: "x"},
	}

	tx := baselineAction(actor, 1)
	tx.Scope = ModerationScopeHide
	tx.ReasonCode = ReasonCodeSpam
	tx.TargetID = hiddenID
	tx = actor.signModeration(tx)
	node.updateModerationRegistry(tx)

	defaultOut := node.filterModeratedEvents(events, false)
	if len(defaultOut) != 1 {
		t.Errorf("hidden event should be excluded by default, got %d", len(defaultOut))
	}
	adminOut := node.filterModeratedEvents(events, true)
	if len(adminOut) != 2 {
		t.Errorf("includeHidden=true should return all events, got %d", len(adminOut))
	}
}

func TestFilterModeratedEvents_AnnotateMergesNote(t *testing.T) {
	node, actor := newModerationTestFixture(t)

	annotatedID := strings.Repeat("f", 64)
	events := []EventTransaction{
		{
			BaseTransaction: BaseTransaction{ID: annotatedID},
			EventType:       "review",
			Payload:         map[string]interface{}{"stars": 5},
		},
	}

	tx := baselineAction(actor, 1)
	tx.Scope = ModerationScopeAnnotate
	tx.ReasonCode = ReasonCodeMisinformation
	tx.EvidenceURL = "https://internal/ticket/42"
	tx.AnnotationText = "Disputed: fact-check pending"
	tx.TargetID = annotatedID
	tx = actor.signModeration(tx)
	node.updateModerationRegistry(tx)

	out := node.filterModeratedEvents(events, false)
	if len(out) != 1 {
		t.Fatalf("annotate should not drop events, got %d", len(out))
	}
	note, ok := out[0].Payload["_moderationNote"]
	if !ok {
		t.Fatal("annotation note not merged into payload")
	}
	if note != "Disputed: fact-check pending" {
		t.Errorf("annotation mismatch: %v", note)
	}
	// Original payload slot must remain untouched.
	if out[0].Payload["stars"] != 5 {
		t.Error("original payload data was clobbered by annotation merge")
	}
	// The registry's stored event must NOT have been mutated.
	if _, mutated := events[0].Payload["_moderationNote"]; mutated {
		t.Error("annotation should not mutate the caller's slice (copy-on-write)")
	}
}
