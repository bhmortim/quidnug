// Package core — ledger_test.go
//
// Methodology
// -----------
// Exercises the NonceLedger type (QDP-0001 §6.1.2). Tests are grouped
// into four stanzas reflecting the ledger's public surface:
//
//   1. Admit happy paths     — fresh signer, monotonic sequence,
//                              independence across (quid, domain).
//   2. Admit rejection paths — each error class surfaced exactly once
//                              (replay, reservation, gap, invalid).
//   3. Tentative lifecycle   — Reserve/Release/CommitAccepted state
//                              transitions + checkpoint application.
//   4. Epoch semantics       — ensures Admit honors rotation state.
//
// Concurrency is exercised by a single race test that fans 100
// goroutines through Admit on the same key. Go's race detector (run
// in CI via `go test -race`) is the primary line of defense against
// ledger data races; the test here just exercises the code path.
//
// The ledger's authoritative state is reconstructible from the
// blockchain, so tests construct state by direct method calls rather
// than replaying blocks — block-level flow is covered by the
// anchor_integration_test and by the upstream block_operations tests.
package core

import (
	"errors"
	"testing"
)

// ----- Admit: happy paths ---------------------------------------------------

func TestLedger_Admit_FreshSigner(t *testing.T) {
	l := NewNonceLedger()
	err := l.Admit(NonceKey{Quid: "aaa"}, 1)
	if err != nil {
		t.Fatalf("first nonce for fresh signer should be accepted, got %v", err)
	}
}

func TestLedger_Admit_StrictMonotonicPerKey(t *testing.T) {
	l := NewNonceLedger()
	key := NonceKey{Quid: "aaa"}

	if err := l.Admit(key, 1); err != nil {
		t.Fatalf("admit 1: %v", err)
	}
	l.CommitAccepted(key, 1)

	if err := l.Admit(key, 2); err != nil {
		t.Fatalf("admit 2 after accepting 1: %v", err)
	}
}

func TestLedger_Admit_DifferentKeysIndependent(t *testing.T) {
	l := NewNonceLedger()
	a := NonceKey{Quid: "aaa"}
	b := NonceKey{Quid: "bbb"}
	l.CommitAccepted(a, 5)
	if err := l.Admit(b, 1); err != nil {
		t.Fatalf("b's first nonce independent of a's: %v", err)
	}
}

func TestLedger_Admit_DifferentDomainsIndependent(t *testing.T) {
	l := NewNonceLedger()
	a := NonceKey{Quid: "aaa", Domain: "foo.example"}
	b := NonceKey{Quid: "aaa", Domain: "bar.example"}
	l.CommitAccepted(a, 5)
	if err := l.Admit(b, 1); err != nil {
		t.Fatalf("same signer in different domain should not be blocked: %v", err)
	}
}

// ----- Admit: rejection paths ----------------------------------------------

func TestLedger_Admit_RejectsReplay(t *testing.T) {
	l := NewNonceLedger()
	key := NonceKey{Quid: "aaa"}
	l.CommitAccepted(key, 5)

	if err := l.Admit(key, 5); !errors.Is(err, ErrNonceReplay) {
		t.Fatalf("want ErrNonceReplay, got %v", err)
	}
	if err := l.Admit(key, 4); !errors.Is(err, ErrNonceReplay) {
		t.Fatalf("want ErrNonceReplay on strictly-lower nonce, got %v", err)
	}
}

func TestLedger_Admit_RejectsReservedByTentative(t *testing.T) {
	l := NewNonceLedger()
	key := NonceKey{Quid: "aaa"}
	l.ReserveTentative(key, 3)

	if err := l.Admit(key, 3); !errors.Is(err, ErrNonceReserved) {
		t.Fatalf("want ErrNonceReserved, got %v", err)
	}
	if err := l.Admit(key, 4); err != nil {
		t.Fatalf("nonce above reservation should be admissible: %v", err)
	}
}

func TestLedger_Admit_RejectsGapTooLarge(t *testing.T) {
	l := NewNonceLedger()
	l.SetMaxNonceGap(10)
	key := NonceKey{Quid: "aaa"}
	l.CommitAccepted(key, 5)

	if err := l.Admit(key, 16); !errors.Is(err, ErrNonceGapTooLarge) {
		t.Fatalf("want ErrNonceGapTooLarge at gap=11, got %v", err)
	}
	if err := l.Admit(key, 15); err != nil {
		t.Fatalf("gap exactly at the cap should pass: %v", err)
	}
}

func TestLedger_Admit_RejectsInvalidInput(t *testing.T) {
	l := NewNonceLedger()
	if err := l.Admit(NonceKey{Quid: ""}, 1); !errors.Is(err, ErrNonceInvalidInput) {
		t.Fatalf("want ErrNonceInvalidInput for empty quid, got %v", err)
	}
	if err := l.Admit(NonceKey{Quid: "aaa"}, 0); !errors.Is(err, ErrNonceInvalidInput) {
		t.Fatalf("want ErrNonceInvalidInput for zero nonce, got %v", err)
	}
	if err := l.Admit(NonceKey{Quid: "aaa"}, -1); !errors.Is(err, ErrNonceInvalidInput) {
		t.Fatalf("want ErrNonceInvalidInput for negative nonce, got %v", err)
	}
}

// ----- Tentative lifecycle --------------------------------------------------

func TestLedger_ReserveAndRelease(t *testing.T) {
	l := NewNonceLedger()
	key := NonceKey{Quid: "aaa"}

	l.ReserveTentative(key, 5)
	if got := l.Tentative(key); got != 5 {
		t.Fatalf("Tentative after reserve: want 5, got %d", got)
	}

	// Lower ReserveTentative is a no-op.
	l.ReserveTentative(key, 3)
	if got := l.Tentative(key); got != 5 {
		t.Fatalf("Tentative should not decrease on lower Reserve: got %d", got)
	}

	// Commit raises accepted AND keeps tentative at least as high.
	l.CommitAccepted(key, 4)
	if got := l.Accepted(key); got != 4 {
		t.Fatalf("Accepted after commit: want 4, got %d", got)
	}
	if got := l.Tentative(key); got != 5 {
		t.Fatalf("Tentative should remain 5 after commit(4): got %d", got)
	}

	// Release back to accepted floor (4); the 5 reservation is dropped.
	l.ReleaseTentative(key, 4)
	if got := l.Tentative(key); got != 4 {
		t.Fatalf("Tentative after release to floor: want 4, got %d", got)
	}

	// Release attempt below accepted is clamped.
	l.ReleaseTentative(key, 0)
	if got := l.Tentative(key); got != 4 {
		t.Fatalf("Tentative must never fall below Accepted: got %d", got)
	}
}

func TestLedger_ApplyCheckpoints_TrustedAdvancesAccepted(t *testing.T) {
	l := NewNonceLedger()
	cps := []NonceCheckpoint{
		{Quid: "a", Epoch: 0, MaxNonce: 10},
		{Quid: "b", Epoch: 0, MaxNonce: 3},
	}
	l.ApplyCheckpoints(cps, true)

	if got := l.Accepted(NonceKey{Quid: "a"}); got != 10 {
		t.Fatalf("Accepted(a): want 10, got %d", got)
	}
	if got := l.Accepted(NonceKey{Quid: "b"}); got != 3 {
		t.Fatalf("Accepted(b): want 3, got %d", got)
	}
	if got := l.Tentative(NonceKey{Quid: "a"}); got != 10 {
		t.Fatalf("Tentative(a) should track Accepted: got %d", got)
	}
}

func TestLedger_ApplyCheckpoints_TentativeOnlyDoesNotCommit(t *testing.T) {
	l := NewNonceLedger()
	cps := []NonceCheckpoint{{Quid: "a", Epoch: 0, MaxNonce: 10}}
	l.ApplyCheckpoints(cps, false)

	if got := l.Accepted(NonceKey{Quid: "a"}); got != 0 {
		t.Fatalf("Accepted should remain 0 for tentative-only checkpoint: got %d", got)
	}
	if got := l.Tentative(NonceKey{Quid: "a"}); got != 10 {
		t.Fatalf("Tentative should be 10: got %d", got)
	}
}

// ----- Epoch semantics (anchor-future) --------------------------------------

func TestLedger_Admit_EpochZeroAcceptedForFreshSigner(t *testing.T) {
	l := NewNonceLedger()
	if err := l.Admit(NonceKey{Quid: "aaa", Epoch: 0}, 1); err != nil {
		t.Fatalf("epoch 0 on fresh signer should be fine: %v", err)
	}
}

func TestLedger_Admit_EpochStaleAfterRotation(t *testing.T) {
	l := NewNonceLedger()
	// Simulate a rotation having landed (anchor implementation is future work,
	// but the ledger must already honor the epoch rule).
	l.mu.Lock()
	l.currentEpoch["aaa"] = 1
	l.mu.Unlock()

	if err := l.Admit(NonceKey{Quid: "aaa", Epoch: 0}, 1); !errors.Is(err, ErrNonceEpochStale) {
		t.Fatalf("want ErrNonceEpochStale for epoch=0 when current=1, got %v", err)
	}
	if err := l.Admit(NonceKey{Quid: "aaa", Epoch: 2}, 1); !errors.Is(err, ErrNonceEpochUnknown) {
		t.Fatalf("want ErrNonceEpochUnknown for epoch=2 when current=1, got %v", err)
	}
	if err := l.Admit(NonceKey{Quid: "aaa", Epoch: 1}, 1); err != nil {
		t.Fatalf("epoch=1 matching current should be fine: %v", err)
	}
}

// ----- Concurrency ---------------------------------------------------------

func TestLedger_Admit_Concurrent(t *testing.T) {
	l := NewNonceLedger()
	key := NonceKey{Quid: "aaa"}

	// 100 goroutines race to admit different nonces. Admit itself is
	// read-only, so all should pass (none reserves). This stresses the
	// RWMutex path.
	done := make(chan struct{})
	for i := 1; i <= 100; i++ {
		n := int64(i)
		go func() {
			if err := l.Admit(key, n); err != nil {
				t.Errorf("admit %d: %v", n, err)
			}
			done <- struct{}{}
		}()
	}
	for i := 0; i < 100; i++ {
		<-done
	}
}
