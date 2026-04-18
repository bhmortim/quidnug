package core

import "testing"

func TestComputeNonceCheckpoints_EmptyTxs(t *testing.T) {
	if got := computeNonceCheckpoints(nil, "d"); got != nil {
		t.Fatalf("empty txs should yield nil, got %v", got)
	}
	if got := computeNonceCheckpoints([]interface{}{}, "d"); got != nil {
		t.Fatalf("empty slice should yield nil, got %v", got)
	}
}

func TestComputeNonceCheckpoints_IgnoresNonTrustTxs(t *testing.T) {
	txs := []interface{}{
		IdentityTransaction{BaseTransaction: BaseTransaction{ID: "x"}, QuidID: "q"},
		TitleTransaction{BaseTransaction: BaseTransaction{ID: "y"}, AssetID: "a"},
	}
	if got := computeNonceCheckpoints(txs, "d"); got != nil {
		t.Fatalf("non-trust txs should be ignored, got %v", got)
	}
}

func TestComputeNonceCheckpoints_GroupsBySignerTakesMax(t *testing.T) {
	txs := []interface{}{
		TrustTransaction{Truster: "aaa", Trustee: "x", Nonce: 2, TrustLevel: 0.5},
		TrustTransaction{Truster: "aaa", Trustee: "y", Nonce: 7, TrustLevel: 0.5},
		TrustTransaction{Truster: "aaa", Trustee: "z", Nonce: 5, TrustLevel: 0.5},
		TrustTransaction{Truster: "bbb", Trustee: "x", Nonce: 3, TrustLevel: 0.5},
	}
	got := computeNonceCheckpoints(txs, "dom")
	if len(got) != 2 {
		t.Fatalf("want 2 checkpoints (one per signer), got %d: %+v", len(got), got)
	}

	want := map[string]int64{"aaa": 7, "bbb": 3}
	for _, cp := range got {
		w, ok := want[cp.Quid]
		if !ok {
			t.Fatalf("unexpected quid in checkpoints: %q", cp.Quid)
		}
		if cp.MaxNonce != w {
			t.Fatalf("checkpoint for %q: want MaxNonce=%d, got %d", cp.Quid, w, cp.MaxNonce)
		}
		if cp.Domain != "dom" {
			t.Fatalf("checkpoint for %q: want Domain='dom', got %q", cp.Quid, cp.Domain)
		}
		delete(want, cp.Quid)
	}
}

func TestComputeNonceCheckpoints_DeterministicOrder(t *testing.T) {
	// Two calls with the same logical input (but different insertion
	// order) must produce byte-identical slices so that two honest
	// producers compute the same block hash.
	txs1 := []interface{}{
		TrustTransaction{Truster: "ccc", Trustee: "x", Nonce: 1, TrustLevel: 0.5},
		TrustTransaction{Truster: "aaa", Trustee: "x", Nonce: 2, TrustLevel: 0.5},
		TrustTransaction{Truster: "bbb", Trustee: "x", Nonce: 3, TrustLevel: 0.5},
	}
	txs2 := []interface{}{
		TrustTransaction{Truster: "aaa", Trustee: "x", Nonce: 2, TrustLevel: 0.5},
		TrustTransaction{Truster: "bbb", Trustee: "x", Nonce: 3, TrustLevel: 0.5},
		TrustTransaction{Truster: "ccc", Trustee: "x", Nonce: 1, TrustLevel: 0.5},
	}

	a := computeNonceCheckpoints(txs1, "d")
	b := computeNonceCheckpoints(txs2, "d")

	if len(a) != len(b) {
		t.Fatalf("length mismatch: %d vs %d", len(a), len(b))
	}
	for i := range a {
		if a[i] != b[i] {
			t.Fatalf("position %d differs: %+v vs %+v", i, a[i], b[i])
		}
	}
	// Also assert the actual alphabetical ordering.
	if a[0].Quid != "aaa" || a[1].Quid != "bbb" || a[2].Quid != "ccc" {
		t.Fatalf("ordering wrong: %+v", a)
	}
}

func TestComputeNonceCheckpoints_SkipsEmptyTrusterOrZeroNonce(t *testing.T) {
	txs := []interface{}{
		TrustTransaction{Truster: "", Trustee: "x", Nonce: 5, TrustLevel: 0.5},    // empty truster
		TrustTransaction{Truster: "aaa", Trustee: "x", Nonce: 0, TrustLevel: 0.5}, // zero nonce
		TrustTransaction{Truster: "aaa", Trustee: "x", Nonce: -1, TrustLevel: 0.5}, // negative
		TrustTransaction{Truster: "aaa", Trustee: "y", Nonce: 1, TrustLevel: 0.5}, // valid
	}
	got := computeNonceCheckpoints(txs, "d")
	if len(got) != 1 || got[0].MaxNonce != 1 {
		t.Fatalf("want single checkpoint with MaxNonce=1, got %+v", got)
	}
}
