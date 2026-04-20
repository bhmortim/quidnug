// QDP-0017 data subject rights + consent tests. Coverage:
//
//   - Per-tx-type validation: required-field, enum, nonce,
//     signature, self-sign consistency.
//   - Registry: grant → withdraw flow, restriction union,
//     DSR status lookup.
//   - Read helpers: HasActiveConsent, IsProcessingRestricted,
//     RestrictedUsesFor, ConsentHistoryFor.
package core

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

// --- shared helpers -----------------------------------------------------

func (a *testNodeActor) signDSR(tx DataSubjectRequestTransaction) DataSubjectRequestTransaction {
	tx.PublicKey = a.PubHex
	tx.SubjectQuid = a.QuidID
	tx.Signature = ""
	signable, _ := json.Marshal(tx)
	tx.Signature = signIEEE1363(a.Priv, signable)
	return tx
}

func (a *testNodeActor) signConsentGrant(tx ConsentGrantTransaction) ConsentGrantTransaction {
	tx.PublicKey = a.PubHex
	tx.SubjectQuid = a.QuidID
	tx.Signature = ""
	signable, _ := json.Marshal(tx)
	tx.Signature = signIEEE1363(a.Priv, signable)
	return tx
}

func (a *testNodeActor) signConsentWithdraw(tx ConsentWithdrawTransaction) ConsentWithdrawTransaction {
	tx.PublicKey = a.PubHex
	tx.SubjectQuid = a.QuidID
	tx.Signature = ""
	signable, _ := json.Marshal(tx)
	tx.Signature = signIEEE1363(a.Priv, signable)
	return tx
}

func (a *testNodeActor) signRestriction(tx ProcessingRestrictionTransaction) ProcessingRestrictionTransaction {
	tx.PublicKey = a.PubHex
	tx.SubjectQuid = a.QuidID
	tx.Signature = ""
	signable, _ := json.Marshal(tx)
	tx.Signature = signIEEE1363(a.Priv, signable)
	return tx
}

func (a *testNodeActor) signCompliance(tx DSRComplianceTransaction) DSRComplianceTransaction {
	tx.PublicKey = a.PubHex
	tx.OperatorQuid = a.QuidID
	tx.Signature = ""
	signable, _ := json.Marshal(tx)
	tx.Signature = signIEEE1363(a.Priv, signable)
	return tx
}

func newPrivacyTestFixture(t *testing.T) (*QuidnugNode, *testNodeActor) {
	t.Helper()
	node := newTestNode()
	actor := newTestNodeActor(t)
	return node, actor
}

// ---- DATA_SUBJECT_REQUEST ----------------------------------------------

func TestValidateDSR_Baseline(t *testing.T) {
	node, actor := newPrivacyTestFixture(t)
	tx := DataSubjectRequestTransaction{
		BaseTransaction: BaseTransaction{
			Type:        TxTypeDataSubjectRequest,
			TrustDomain: "test.domain.com",
			Timestamp:   time.Now().Unix(),
		},
		RequestType:  DSRTypeAccess,
		Nonce:        1,
		Jurisdiction: "EU",
	}
	tx = actor.signDSR(tx)
	if !node.ValidateDataSubjectRequestTransaction(tx) {
		t.Fatal("baseline DSR should validate")
	}
}

func TestValidateDSR_RejectsUnknownRequestType(t *testing.T) {
	node, actor := newPrivacyTestFixture(t)
	tx := DataSubjectRequestTransaction{
		BaseTransaction: BaseTransaction{
			Type:        TxTypeDataSubjectRequest,
			TrustDomain: "test.domain.com",
			Timestamp:   time.Now().Unix(),
		},
		RequestType: "OPTIMIZE", // not in the enum
		Nonce:       1,
	}
	tx = actor.signDSR(tx)
	if node.ValidateDataSubjectRequestTransaction(tx) {
		t.Error("unknown request type should be rejected")
	}
}

func TestValidateDSR_RejectsMissingDomain(t *testing.T) {
	node, actor := newPrivacyTestFixture(t)
	tx := DataSubjectRequestTransaction{
		BaseTransaction: BaseTransaction{Type: TxTypeDataSubjectRequest, Timestamp: time.Now().Unix()},
		RequestType:     DSRTypeAccess,
		Nonce:           1,
	}
	tx = actor.signDSR(tx)
	if node.ValidateDataSubjectRequestTransaction(tx) {
		t.Error("missing domain should be rejected")
	}
}

func TestValidateDSR_NonceMonotonic(t *testing.T) {
	node, actor := newPrivacyTestFixture(t)

	first := DataSubjectRequestTransaction{
		BaseTransaction: BaseTransaction{Type: TxTypeDataSubjectRequest, TrustDomain: "test.domain.com", Timestamp: time.Now().Unix()},
		RequestType:     DSRTypeAccess,
		Nonce:           3,
	}
	first = actor.signDSR(first)
	if !node.ValidateDataSubjectRequestTransaction(first) {
		t.Fatal("first should validate")
	}
	node.updatePrivacyRegistryDSR(first)

	replay := actor.signDSR(first)
	if node.ValidateDataSubjectRequestTransaction(replay) {
		t.Error("replay should be rejected")
	}

	lower := DataSubjectRequestTransaction{
		BaseTransaction: BaseTransaction{Type: TxTypeDataSubjectRequest, TrustDomain: "test.domain.com", Timestamp: time.Now().Unix()},
		RequestType:     DSRTypeAccess,
		Nonce:           2,
	}
	lower = actor.signDSR(lower)
	if node.ValidateDataSubjectRequestTransaction(lower) {
		t.Error("lower nonce should be rejected")
	}
}

func TestValidateDSR_PubkeyQuidMismatch(t *testing.T) {
	node, actor := newPrivacyTestFixture(t)
	other := newTestNodeActor(t)

	tx := DataSubjectRequestTransaction{
		BaseTransaction: BaseTransaction{Type: TxTypeDataSubjectRequest, TrustDomain: "test.domain.com", Timestamp: time.Now().Unix()},
		RequestType:     DSRTypeAccess,
		Nonce:           1,
	}
	tx = actor.signDSR(tx)
	tx.PublicKey = other.PubHex // mismatch
	if node.ValidateDataSubjectRequestTransaction(tx) {
		t.Error("mismatched pubkey should be rejected")
	}
}

// ---- CONSENT_GRANT -----------------------------------------------------

func baselineConsentGrant(actor *testNodeActor, nonce int64) ConsentGrantTransaction {
	return ConsentGrantTransaction{
		BaseTransaction: BaseTransaction{Type: TxTypeConsentGrant, TrustDomain: "test.domain.com", Timestamp: time.Now().Unix()},
		ControllerQuid:  "0000000000000009",
		Scope:           []string{ConsentScopeProfileBuilding, ConsentScopeAITraining},
		PolicyURL:       "https://operator.example/privacy",
		PolicyHash:      strings.Repeat("a", 64),
		Nonce:           nonce,
	}
}

func TestValidateConsentGrant_Baseline(t *testing.T) {
	node, actor := newPrivacyTestFixture(t)
	tx := actor.signConsentGrant(baselineConsentGrant(actor, 1))
	if !node.ValidateConsentGrantTransaction(tx) {
		t.Fatal("baseline consent grant should validate")
	}
}

func TestValidateConsentGrant_EmptyScopeRejected(t *testing.T) {
	node, actor := newPrivacyTestFixture(t)
	tx := baselineConsentGrant(actor, 1)
	tx.Scope = nil
	tx = actor.signConsentGrant(tx)
	if node.ValidateConsentGrantTransaction(tx) {
		t.Error("empty scope should be rejected")
	}
}

func TestValidateConsentGrant_UnknownScopeRejected(t *testing.T) {
	node, actor := newPrivacyTestFixture(t)
	tx := baselineConsentGrant(actor, 1)
	tx.Scope = []string{"TELEPATHY"}
	tx = actor.signConsentGrant(tx)
	if node.ValidateConsentGrantTransaction(tx) {
		t.Error("unknown scope should be rejected")
	}
}

func TestValidateConsentGrant_CustomScopeAccepted(t *testing.T) {
	node, actor := newPrivacyTestFixture(t)
	tx := baselineConsentGrant(actor, 1)
	tx.Scope = []string{"CUSTOM:recommend-me-books"}
	tx = actor.signConsentGrant(tx)
	if !node.ValidateConsentGrantTransaction(tx) {
		t.Error("CUSTOM:-prefixed scope should be accepted")
	}
}

func TestValidateConsentGrant_ExpiredEffectiveUntilRejected(t *testing.T) {
	node, actor := newPrivacyTestFixture(t)
	tx := baselineConsentGrant(actor, 1)
	tx.EffectiveUntil = tx.Timestamp - 10 // already expired
	tx = actor.signConsentGrant(tx)
	if node.ValidateConsentGrantTransaction(tx) {
		t.Error("already-expired consent should be rejected")
	}
}

// ---- CONSENT_WITHDRAW --------------------------------------------------

func TestValidateConsentWithdraw_RejectsWithoutPriorGrant(t *testing.T) {
	node, actor := newPrivacyTestFixture(t)
	tx := ConsentWithdrawTransaction{
		BaseTransaction:    BaseTransaction{Type: TxTypeConsentWithdraw, TrustDomain: "test.domain.com", Timestamp: time.Now().Unix()},
		WithdrawsGrantTxID: "does-not-exist",
		Nonce:              1,
	}
	tx = actor.signConsentWithdraw(tx)
	if node.ValidateConsentWithdrawTransaction(tx) {
		t.Error("withdraw without matching grant should be rejected")
	}
}

func TestConsentGrantThenWithdraw_Flow(t *testing.T) {
	node, actor := newPrivacyTestFixture(t)

	grant := baselineConsentGrant(actor, 1)
	grant.ID = "grant-1"
	grant = actor.signConsentGrant(grant)
	if !node.ValidateConsentGrantTransaction(grant) {
		t.Fatal("grant should validate")
	}
	node.updatePrivacyRegistryGrant(grant)

	if !node.HasActiveConsent(actor.QuidID, grant.ControllerQuid, ConsentScopeProfileBuilding) {
		t.Fatal("active consent should be reported")
	}

	withdraw := ConsentWithdrawTransaction{
		BaseTransaction:    BaseTransaction{Type: TxTypeConsentWithdraw, TrustDomain: "test.domain.com", Timestamp: time.Now().Unix()},
		WithdrawsGrantTxID: grant.ID,
		Nonce:              1,
	}
	withdraw = actor.signConsentWithdraw(withdraw)
	if !node.ValidateConsentWithdrawTransaction(withdraw) {
		t.Fatal("withdraw should validate")
	}
	node.updatePrivacyRegistryWithdraw(withdraw)

	if node.HasActiveConsent(actor.QuidID, grant.ControllerQuid, ConsentScopeProfileBuilding) {
		t.Error("consent should be inactive after withdraw")
	}
}

func TestConsentGrant_ExpiredEffectiveUntilNotActive(t *testing.T) {
	node, actor := newPrivacyTestFixture(t)

	grant := baselineConsentGrant(actor, 1)
	grant.ID = "grant-expire"
	// Set the EffectiveUntil at Timestamp+1 — validates now but
	// will be inactive once the clock passes it.
	grant.EffectiveUntil = grant.Timestamp + 1
	grant = actor.signConsentGrant(grant)
	if !node.ValidateConsentGrantTransaction(grant) {
		t.Fatal("grant should validate at submission")
	}
	node.updatePrivacyRegistryGrant(grant)

	// Freeze the clock past EffectiveUntil.
	defer resetTestClock()
	setTestClockNano((grant.EffectiveUntil + 60) * int64(time.Second))

	if node.HasActiveConsent(actor.QuidID, grant.ControllerQuid, ConsentScopeProfileBuilding) {
		t.Error("expired consent should not be active")
	}
}

// ---- PROCESSING_RESTRICTION -------------------------------------------

func TestValidateRestriction_Baseline(t *testing.T) {
	node, actor := newPrivacyTestFixture(t)
	tx := ProcessingRestrictionTransaction{
		BaseTransaction: BaseTransaction{Type: TxTypeProcessingRestriction, TrustDomain: "test.domain.com", Timestamp: time.Now().Unix()},
		RestrictedUses:  []string{"recommendation-aggregation", "marketing"},
		Nonce:           1,
	}
	tx = actor.signRestriction(tx)
	if !node.ValidateProcessingRestrictionTransaction(tx) {
		t.Fatal("baseline restriction should validate")
	}
}

func TestValidateRestriction_UnknownUseRejected(t *testing.T) {
	node, actor := newPrivacyTestFixture(t)
	tx := ProcessingRestrictionTransaction{
		BaseTransaction: BaseTransaction{Type: TxTypeProcessingRestriction, TrustDomain: "test.domain.com", Timestamp: time.Now().Unix()},
		RestrictedUses:  []string{"not-a-real-use"},
		Nonce:           1,
	}
	tx = actor.signRestriction(tx)
	if node.ValidateProcessingRestrictionTransaction(tx) {
		t.Error("unknown restricted use should be rejected")
	}
}

func TestIsProcessingRestricted_UnionAcrossMultiple(t *testing.T) {
	node, actor := newPrivacyTestFixture(t)

	for i, use := range [][]string{
		{"recommendation-aggregation"},
		{"marketing", "ai-training"},
	} {
		tx := ProcessingRestrictionTransaction{
			BaseTransaction: BaseTransaction{Type: TxTypeProcessingRestriction, TrustDomain: "test.domain.com", Timestamp: time.Now().Unix()},
			RestrictedUses:  use,
			Nonce:           int64(i + 1),
		}
		tx = actor.signRestriction(tx)
		if !node.ValidateProcessingRestrictionTransaction(tx) {
			t.Fatalf("restriction %d should validate", i)
		}
		node.updatePrivacyRegistryRestriction(tx)
	}

	for _, want := range []string{"recommendation-aggregation", "marketing", "ai-training"} {
		if !node.IsProcessingRestricted(actor.QuidID, want) {
			t.Errorf("expected %q to be restricted", want)
		}
	}
	if node.IsProcessingRestricted(actor.QuidID, "unrelated") {
		t.Error("unlisted use should not be restricted")
	}
}

func TestRestrictedUsesFor_ExpirationDrops(t *testing.T) {
	node, actor := newPrivacyTestFixture(t)

	active := ProcessingRestrictionTransaction{
		BaseTransaction: BaseTransaction{Type: TxTypeProcessingRestriction, TrustDomain: "test.domain.com", Timestamp: time.Now().Unix()},
		RestrictedUses:  []string{"marketing"},
		Nonce:           1,
	}
	active = actor.signRestriction(active)
	node.updatePrivacyRegistryRestriction(active)

	expiring := ProcessingRestrictionTransaction{
		BaseTransaction: BaseTransaction{Type: TxTypeProcessingRestriction, TrustDomain: "test.domain.com", Timestamp: time.Now().Unix()},
		RestrictedUses:  []string{"ai-training"},
		EffectiveUntil:  time.Now().Unix() + 5,
		Nonce:           2,
	}
	expiring = actor.signRestriction(expiring)
	node.updatePrivacyRegistryRestriction(expiring)

	defer resetTestClock()
	setTestClockNano((time.Now().Unix() + 60) * int64(time.Second))

	uses := node.RestrictedUsesFor(actor.QuidID)
	seen := map[string]bool{}
	for _, u := range uses {
		seen[u] = true
	}
	if !seen["marketing"] {
		t.Error("non-expiring restriction should remain")
	}
	if seen["ai-training"] {
		t.Error("expired restriction should have dropped out")
	}
}

// ---- DSR_COMPLIANCE ---------------------------------------------------

func TestValidateDSRCompliance_RequiresValidator(t *testing.T) {
	node := newTestNode()
	operator := newTestNodeActor(t)

	tx := DSRComplianceTransaction{
		BaseTransaction:  BaseTransaction{Type: TxTypeDSRCompliance, TrustDomain: "test.domain.com", Timestamp: time.Now().Unix()},
		RequestTxID:      "some-request",
		RequestType:      DSRTypeAccess,
		CompletedAt:      time.Now().Unix(),
		ActionsCategory:  "manifest-generated",
		Nonce:            1,
	}
	tx = operator.signCompliance(tx)
	// operator is NOT a validator on test.domain.com yet.
	if node.ValidateDSRComplianceTransaction(tx) {
		t.Fatal("non-validator operator should be rejected")
	}

	// Promote to validator and retry.
	td := node.TrustDomains["test.domain.com"]
	td.Validators[operator.QuidID] = 1.0
	td.ValidatorNodes = append(td.ValidatorNodes, operator.QuidID)
	node.TrustDomains["test.domain.com"] = td

	if !node.ValidateDSRComplianceTransaction(tx) {
		t.Error("validator operator should be accepted")
	}
}

func TestGetDSRStatus_HappyPath(t *testing.T) {
	node := newTestNode()
	subject := newTestNodeActor(t)
	operator := newTestNodeActor(t)

	// Promote operator to validator.
	td := node.TrustDomains["test.domain.com"]
	td.Validators[operator.QuidID] = 1.0
	td.ValidatorNodes = append(td.ValidatorNodes, operator.QuidID)
	node.TrustDomains["test.domain.com"] = td

	req := DataSubjectRequestTransaction{
		BaseTransaction: BaseTransaction{ID: "req-1", Type: TxTypeDataSubjectRequest, TrustDomain: "test.domain.com", Timestamp: time.Now().Unix()},
		RequestType:     DSRTypeAccess,
		Nonce:           1,
	}
	req = subject.signDSR(req)
	node.updatePrivacyRegistryDSR(req)

	// Not yet completed.
	_, _, reqOk, compOk := node.GetDSRStatus("req-1")
	if !reqOk || compOk {
		t.Errorf("expected request-only state, got reqOk=%v compOk=%v", reqOk, compOk)
	}

	comp := DSRComplianceTransaction{
		BaseTransaction:  BaseTransaction{ID: "comp-1", Type: TxTypeDSRCompliance, TrustDomain: "test.domain.com", Timestamp: time.Now().Unix()},
		RequestTxID:      "req-1",
		RequestType:      DSRTypeAccess,
		CompletedAt:      time.Now().Unix(),
		ActionsCategory:  "manifest-generated",
		Nonce:            1,
	}
	comp = operator.signCompliance(comp)
	if !node.ValidateDSRComplianceTransaction(comp) {
		t.Fatal("compliance should validate")
	}
	node.updatePrivacyRegistryCompliance(comp)

	gotReq, gotComp, reqOk, compOk := node.GetDSRStatus("req-1")
	if !reqOk || !compOk {
		t.Fatalf("expected both request + compliance, got reqOk=%v compOk=%v", reqOk, compOk)
	}
	if gotReq.ID != "req-1" || gotComp.RequestTxID != "req-1" {
		t.Errorf("status lookup returned wrong records: req=%+v comp=%+v", gotReq, gotComp)
	}
}

// ---- ConsentHistoryFor -------------------------------------------------

func TestConsentHistoryFor_ReturnsAllGrantsWithWithdrawnFlag(t *testing.T) {
	node, actor := newPrivacyTestFixture(t)

	grant1 := baselineConsentGrant(actor, 1)
	grant1.ID = "g-1"
	grant1 = actor.signConsentGrant(grant1)
	node.updatePrivacyRegistryGrant(grant1)

	grant2 := baselineConsentGrant(actor, 2)
	grant2.ID = "g-2"
	grant2.Scope = []string{ConsentScopeMarketingEmail}
	grant2 = actor.signConsentGrant(grant2)
	node.updatePrivacyRegistryGrant(grant2)

	// Withdraw grant2.
	w := ConsentWithdrawTransaction{
		BaseTransaction:    BaseTransaction{Type: TxTypeConsentWithdraw, TrustDomain: "test.domain.com", Timestamp: time.Now().Unix()},
		WithdrawsGrantTxID: "g-2",
		Nonce:              1,
	}
	w = actor.signConsentWithdraw(w)
	node.updatePrivacyRegistryWithdraw(w)

	entries := node.ConsentHistoryFor(actor.QuidID)
	if len(entries) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(entries))
	}
	for _, e := range entries {
		if e.Grant.ID == "g-1" && e.Withdrawn {
			t.Error("grant g-1 should not be withdrawn")
		}
		if e.Grant.ID == "g-2" && !e.Withdrawn {
			t.Error("grant g-2 should be withdrawn")
		}
	}
}
