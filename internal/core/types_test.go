package core

import (
	"encoding/json"
	"testing"
)

func TestTransactionType_ConstantsMatchWireFormat(t *testing.T) {
	cases := map[TransactionType]string{
		TxTypeTrust:    "TRUST",
		TxTypeIdentity: "IDENTITY",
		TxTypeTitle:    "TITLE",
		TxTypeEvent:    "EVENT",
		TxTypeGeneric:  "GENERIC",
	}
	for got, want := range cases {
		if string(got) != want {
			t.Errorf("TransactionType %q: want wire form %q, got %q", want, want, got)
		}
	}
}

func TestBlockAcceptance_Ordering(t *testing.T) {
	// The code assumes Trusted < Tentative < Untrusted < Invalid as
	// iota-declared BlockAcceptance values. Regression-guard that so a
	// re-ordering can't silently flip tier-comparison logic.
	if BlockTrusted != 0 {
		t.Fatalf("BlockTrusted expected to be zero-valued iota, got %d", BlockTrusted)
	}
	if !(BlockTrusted < BlockTentative && BlockTentative < BlockUntrusted && BlockUntrusted < BlockInvalid) {
		t.Fatal("BlockAcceptance iota ordering changed; tier-comparison code may break")
	}
}

func TestBaseTransaction_RoundTripsJSON(t *testing.T) {
	tx := TrustTransaction{
		BaseTransaction: BaseTransaction{
			ID:          "tx-1",
			Type:        TxTypeTrust,
			TrustDomain: "test.domain.com",
			Timestamp:   1234567890,
			Signature:   "aa",
			PublicKey:   "bb",
		},
		Truster:    "aaaaaaaaaaaaaaaa",
		Trustee:    "bbbbbbbbbbbbbbbb",
		TrustLevel: 0.8,
		Nonce:      1,
	}
	data, err := json.Marshal(tx)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var back TrustTransaction
	if err := json.Unmarshal(data, &back); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if back.TrustLevel != tx.TrustLevel ||
		back.Truster != tx.Truster ||
		back.Trustee != tx.Trustee ||
		back.Type != tx.Type ||
		back.ID != tx.ID {
		t.Fatalf("round-trip mismatch: got %+v, want %+v", back, tx)
	}
}

func TestOwnershipStake_TotalPercentageIsCallerResponsibility(t *testing.T) {
	// OwnershipStake is a value type; percentages are arbitrary floats.
	// This test documents the invariant that types.go intentionally does
	// NOT enforce totals - callers (TitleTransaction validation) are
	// responsible. Guardrail: the struct must not silently normalize.
	owners := []OwnershipStake{
		{OwnerID: "a", Percentage: 0.6, StakeType: "equity"},
		{OwnerID: "b", Percentage: 0.6, StakeType: "equity"}, // deliberately > 1.0 total
	}
	if owners[0].Percentage != 0.6 || owners[1].Percentage != 0.6 {
		t.Fatal("OwnershipStake unexpectedly normalized its Percentage")
	}
}

func TestTrustCacheEntry_Fields(t *testing.T) {
	entry := TrustCacheEntry{TrustLevel: 0.5, TrustPath: []string{"a", "b"}, ExpiresAt: 1}
	if entry.TrustLevel != 0.5 || entry.TrustPath[1] != "b" || entry.ExpiresAt != 1 {
		t.Fatal("TrustCacheEntry field wiring broken")
	}
}
