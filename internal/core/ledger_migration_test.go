package core

import (
	"reflect"
	"testing"
)

func TestMigrateLedgerFromBlocks_EmptyInput(t *testing.T) {
	got := MigrateLedgerFromBlocks(nil)
	if got == nil {
		t.Fatal("expected non-nil ledger even for empty input")
	}
	acc, _ := got.Size()
	if acc != 0 {
		t.Fatalf("empty input should yield empty accepted map, got %d", acc)
	}
}

func TestMigrateLedgerFromBlocks_TrustTxsMaxed(t *testing.T) {
	blocks := []Block{
		{
			TrustProof: TrustProof{TrustDomain: "d1"},
			Transactions: []interface{}{
				TrustTransaction{Truster: "aaa", Trustee: "x", Nonce: 1, TrustLevel: 0.5},
				TrustTransaction{Truster: "aaa", Trustee: "y", Nonce: 3, TrustLevel: 0.5},
			},
		},
		{
			TrustProof: TrustProof{TrustDomain: "d1"},
			Transactions: []interface{}{
				TrustTransaction{Truster: "aaa", Trustee: "z", Nonce: 2, TrustLevel: 0.5},
				TrustTransaction{Truster: "bbb", Trustee: "x", Nonce: 7, TrustLevel: 0.5},
			},
		},
	}

	ledger := MigrateLedgerFromBlocks(blocks)

	// aaa in d1: max is 3 (from block 1).
	if got := ledger.Accepted(NonceKey{Quid: "aaa", Domain: "d1"}); got != 3 {
		t.Fatalf("aaa/d1 accepted: want 3, got %d", got)
	}
	if got := ledger.Accepted(NonceKey{Quid: "bbb", Domain: "d1"}); got != 7 {
		t.Fatalf("bbb/d1 accepted: want 7, got %d", got)
	}
}

func TestMigrateLedgerFromBlocks_DomainScoping(t *testing.T) {
	blocks := []Block{
		{
			TrustProof: TrustProof{TrustDomain: "d1"},
			Transactions: []interface{}{
				TrustTransaction{Truster: "aaa", Trustee: "x", Nonce: 10, TrustLevel: 0.5},
			},
		},
		{
			TrustProof: TrustProof{TrustDomain: "d2"},
			Transactions: []interface{}{
				TrustTransaction{Truster: "aaa", Trustee: "y", Nonce: 3, TrustLevel: 0.5},
			},
		},
	}

	ledger := MigrateLedgerFromBlocks(blocks)

	if got := ledger.Accepted(NonceKey{Quid: "aaa", Domain: "d1"}); got != 10 {
		t.Fatalf("aaa/d1: want 10, got %d", got)
	}
	if got := ledger.Accepted(NonceKey{Quid: "aaa", Domain: "d2"}); got != 3 {
		t.Fatalf("aaa/d2: want 3, got %d", got)
	}
}

func TestMigrateLedgerFromBlocks_IdentityTxsUseCreator(t *testing.T) {
	blocks := []Block{
		{
			TrustProof: TrustProof{TrustDomain: "d1"},
			Transactions: []interface{}{
				IdentityTransaction{Creator: "aaa", QuidID: "target1", UpdateNonce: 5},
				IdentityTransaction{QuidID: "target2", UpdateNonce: 2}, // empty Creator → falls back to QuidID
			},
		},
	}

	ledger := MigrateLedgerFromBlocks(blocks)
	if got := ledger.Accepted(NonceKey{Quid: "aaa", Domain: "d1"}); got != 5 {
		t.Fatalf("identity with Creator=aaa: want 5, got %d", got)
	}
	if got := ledger.Accepted(NonceKey{Quid: "target2", Domain: "d1"}); got != 2 {
		t.Fatalf("identity fallback to QuidID: want 2, got %d", got)
	}
}

func TestMigrateLedgerFromBlocks_Deterministic(t *testing.T) {
	// Two runs over the same logical history produce byte-identical
	// accepted maps. This is the core consensus requirement of
	// QDP-0001 §10.2.1.
	blocks := []Block{
		{
			TrustProof: TrustProof{TrustDomain: "d1"},
			Transactions: []interface{}{
				TrustTransaction{Truster: "aaa", Trustee: "x", Nonce: 2, TrustLevel: 0.5},
				TrustTransaction{Truster: "bbb", Trustee: "y", Nonce: 1, TrustLevel: 0.5},
				IdentityTransaction{Creator: "aaa", QuidID: "q", UpdateNonce: 3},
				EventTransaction{SubjectID: "sub1", Sequence: 4, EventType: "e"},
			},
		},
		{
			TrustProof: TrustProof{TrustDomain: "d2"},
			Transactions: []interface{}{
				TrustTransaction{Truster: "ccc", Trustee: "z", Nonce: 10, TrustLevel: 0.5},
			},
		},
	}

	a := MigrateLedgerFromBlocks(blocks).Snapshot()
	b := MigrateLedgerFromBlocks(blocks).Snapshot()

	if !reflect.DeepEqual(a, b) {
		t.Fatalf("migration non-deterministic:\n  a=%v\n  b=%v", a, b)
	}
}

func TestMigrateLedgerFromBlocks_SkipsInvalidInputs(t *testing.T) {
	blocks := []Block{
		{
			TrustProof: TrustProof{TrustDomain: "d1"},
			Transactions: []interface{}{
				TrustTransaction{Truster: "", Trustee: "x", Nonce: 5, TrustLevel: 0.5},         // empty signer
				TrustTransaction{Truster: "aaa", Trustee: "x", Nonce: 0, TrustLevel: 0.5},      // zero nonce
				TrustTransaction{Truster: "aaa", Trustee: "x", Nonce: -1, TrustLevel: 0.5},     // negative
				TrustTransaction{Truster: "aaa", Trustee: "x", Nonce: 4, TrustLevel: 0.5},      // valid
			},
		},
	}

	ledger := MigrateLedgerFromBlocks(blocks)
	if got := ledger.Accepted(NonceKey{Quid: "aaa", Domain: "d1"}); got != 4 {
		t.Fatalf("should take only valid nonce: want 4, got %d", got)
	}
	if got := ledger.Accepted(NonceKey{Quid: "", Domain: "d1"}); got != 0 {
		t.Fatalf("empty-signer entry should not appear: got %d", got)
	}
}

func TestMigrateLedgerFromBlocks_MaxAcrossMultipleBlocks(t *testing.T) {
	blocks := []Block{
		{TrustProof: TrustProof{TrustDomain: "d1"}, Transactions: []interface{}{
			TrustTransaction{Truster: "aaa", Nonce: 5, TrustLevel: 0.5},
		}},
		{TrustProof: TrustProof{TrustDomain: "d1"}, Transactions: []interface{}{
			TrustTransaction{Truster: "aaa", Nonce: 100, TrustLevel: 0.5},
		}},
		{TrustProof: TrustProof{TrustDomain: "d1"}, Transactions: []interface{}{
			TrustTransaction{Truster: "aaa", Nonce: 50, TrustLevel: 0.5}, // lower — should not shrink
		}},
	}
	ledger := MigrateLedgerFromBlocks(blocks)
	if got := ledger.Accepted(NonceKey{Quid: "aaa", Domain: "d1"}); got != 100 {
		t.Fatalf("max across blocks: want 100, got %d", got)
	}
}

func TestMigrateLedgerFromBlocks_PointerVariants(t *testing.T) {
	tx := TrustTransaction{Truster: "aaa", Nonce: 9, TrustLevel: 0.5}
	ident := IdentityTransaction{Creator: "aaa", QuidID: "q", UpdateNonce: 3}
	blocks := []Block{
		{TrustProof: TrustProof{TrustDomain: "d1"}, Transactions: []interface{}{&tx, &ident}},
	}
	ledger := MigrateLedgerFromBlocks(blocks)
	if got := ledger.Accepted(NonceKey{Quid: "aaa", Domain: "d1"}); got != 9 {
		t.Fatalf("pointer-variant tx: want 9, got %d", got)
	}
}
