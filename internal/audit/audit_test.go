// QDP-0018 Phase 1 audit log tests.
package audit

import (
	"os"
	"path/filepath"
	"testing"
)

func TestAppend_StampsSequenceAndChain(t *testing.T) {
	l := NewLog("op-quid")
	first, err := l.Append(Entry{
		Category: CategoryModerationAction,
		Payload:  map[string]interface{}{"scope": "suppress"},
	})
	if err != nil {
		t.Fatalf("first append: %v", err)
	}
	if first.Sequence != 0 {
		t.Errorf("expected sequence 0, got %d", first.Sequence)
	}
	if first.PrevHash != ZeroPrevHash {
		t.Errorf("entry 0 prev should be zero, got %q", first.PrevHash)
	}
	if first.Hash == "" {
		t.Error("self-hash should have been filled in")
	}
	if first.OperatorQuid != "op-quid" {
		t.Errorf("operator quid not stamped, got %q", first.OperatorQuid)
	}

	second, err := l.Append(Entry{
		Category: CategoryGovernanceVote,
		Payload:  map[string]interface{}{"action": "ADD_VALIDATOR"},
	})
	if err != nil {
		t.Fatalf("second append: %v", err)
	}
	if second.Sequence != 1 {
		t.Errorf("expected sequence 1, got %d", second.Sequence)
	}
	if second.PrevHash != first.Hash {
		t.Errorf("prev hash link broken: got %q want %q", second.PrevHash, first.Hash)
	}
}

func TestAppend_RejectsUnknownCategory(t *testing.T) {
	l := NewLog("op-quid")
	_, err := l.Append(Entry{
		Category: "TELEPATHIC_UPDATE",
		Payload:  map[string]interface{}{},
	})
	if err == nil {
		t.Error("unknown category should be rejected")
	}
}

func TestAppend_RejectsNilPayload(t *testing.T) {
	l := NewLog("op-quid")
	_, err := l.Append(Entry{Category: CategoryOperatorOther, Payload: nil})
	if err == nil {
		t.Error("nil payload should be rejected")
	}
}

func TestAppend_RejectsOverlongNote(t *testing.T) {
	l := NewLog("op-quid")
	big := make([]byte, MaxNoteLength+1)
	for i := range big {
		big[i] = 'x'
	}
	_, err := l.Append(Entry{
		Category: CategoryOperatorOther,
		Payload:  map[string]interface{}{},
		Note:     string(big),
	})
	if err == nil {
		t.Error("over-long note should be rejected")
	}
}

func TestHead_EmptyLog(t *testing.T) {
	l := NewLog("op-quid")
	_, ok := l.Head()
	if ok {
		t.Error("empty log should report Head() false")
	}
}

func TestHead_ReturnsLatestEntry(t *testing.T) {
	l := NewLog("op-quid")
	for i := 0; i < 3; i++ {
		if _, err := l.Append(Entry{
			Category: CategoryNodeLifecycle,
			Payload:  map[string]interface{}{"event": "tick"},
		}); err != nil {
			t.Fatalf("append %d: %v", i, err)
		}
	}
	head, ok := l.Head()
	if !ok {
		t.Fatal("expected head, got none")
	}
	if head.Sequence != 2 {
		t.Errorf("head sequence = %d, want 2", head.Sequence)
	}
}

func TestEntriesSince_ForwardCursor(t *testing.T) {
	l := NewLog("op-quid")
	for i := 0; i < 5; i++ {
		_, _ = l.Append(Entry{
			Category: CategoryOperatorOther,
			Payload:  map[string]interface{}{"i": i},
		})
	}

	got := l.EntriesSince(1, 10)
	if len(got) != 3 {
		t.Fatalf("expected 3 entries after seq=1, got %d", len(got))
	}
	if got[0].Sequence != 2 {
		t.Errorf("first entry should be sequence 2, got %d", got[0].Sequence)
	}
	if got[2].Sequence != 4 {
		t.Errorf("last entry should be sequence 4, got %d", got[2].Sequence)
	}
}

func TestEntriesSince_RespectsLimit(t *testing.T) {
	l := NewLog("op-quid")
	for i := 0; i < 10; i++ {
		_, _ = l.Append(Entry{
			Category: CategoryOperatorOther,
			Payload:  map[string]interface{}{"i": i},
		})
	}
	got := l.EntriesSince(-1, 4) // start from sequence 0
	if len(got) != 4 {
		t.Errorf("expected 4 entries, got %d", len(got))
	}
}

func TestEntriesSince_EmptyTail(t *testing.T) {
	l := NewLog("op-quid")
	_, _ = l.Append(Entry{Category: CategoryOperatorOther, Payload: map[string]interface{}{}})
	got := l.EntriesSince(0, 100)
	if got != nil {
		t.Errorf("expected nil for empty-tail query, got %+v", got)
	}
}

func TestGet_ByEntrySequence(t *testing.T) {
	l := NewLog("op-quid")
	for i := 0; i < 3; i++ {
		_, _ = l.Append(Entry{
			Category: CategoryOperatorOther,
			Payload:  map[string]interface{}{"i": i},
		})
	}

	e, ok := l.Get(1)
	if !ok {
		t.Fatal("expected entry 1 to exist")
	}
	if e.Sequence != 1 {
		t.Errorf("got wrong sequence: %d", e.Sequence)
	}

	_, ok = l.Get(99)
	if ok {
		t.Error("out-of-range get should return false")
	}
}

func TestVerifyChain_IntactChain(t *testing.T) {
	l := NewLog("op-quid")
	for i := 0; i < 5; i++ {
		_, _ = l.Append(Entry{
			Category: CategoryOperatorOther,
			Payload:  map[string]interface{}{"i": i},
		})
	}
	if idx, err := l.VerifyChain(); idx != -1 || err != nil {
		t.Errorf("healthy chain verify: idx=%d err=%v", idx, err)
	}
}

func TestVerifyChain_DetectsTamperedPayload(t *testing.T) {
	l := NewLog("op-quid")
	_, _ = l.Append(Entry{Category: CategoryOperatorOther, Payload: map[string]interface{}{"i": 1}})
	_, _ = l.Append(Entry{Category: CategoryOperatorOther, Payload: map[string]interface{}{"i": 2}})

	// Surgically tamper with entry 0's payload. The stored hash
	// won't match the recomputed hash any more.
	l.mu.Lock()
	l.entries[0].Payload["i"] = 999
	l.mu.Unlock()

	idx, err := l.VerifyChain()
	if err == nil {
		t.Fatal("tampered payload should fail verification")
	}
	if idx != 0 {
		t.Errorf("expected tamper at entry 0, got %d", idx)
	}
}

func TestVerifyChain_DetectsBrokenLink(t *testing.T) {
	l := NewLog("op-quid")
	_, _ = l.Append(Entry{Category: CategoryOperatorOther, Payload: map[string]interface{}{"i": 1}})
	_, _ = l.Append(Entry{Category: CategoryOperatorOther, Payload: map[string]interface{}{"i": 2}})

	// Break the PrevHash link on entry 1.
	l.mu.Lock()
	l.entries[1].PrevHash = "deadbeef"
	l.mu.Unlock()

	idx, err := l.VerifyChain()
	if err == nil {
		t.Fatal("broken link should fail verification")
	}
	if idx != 1 {
		t.Errorf("expected break at entry 1, got %d", idx)
	}
}

// ---- disk-backed store -----------------------------------------------

func TestFileStore_AppendAndLoadRoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "audit.jsonl")

	store, err := NewFileStore(path)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}

	l, err := NewLogWithStore("op-quid", store)
	if err != nil {
		t.Fatalf("wire log: %v", err)
	}
	for i := 0; i < 5; i++ {
		if _, err := l.Append(Entry{
			Category: CategoryModerationAction,
			Payload:  map[string]interface{}{"i": i},
		}); err != nil {
			t.Fatalf("append %d: %v", i, err)
		}
	}
	if err := l.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}

	// Re-open and confirm the same 5 entries replay.
	store2, err := NewFileStore(path)
	if err != nil {
		t.Fatalf("reopen store: %v", err)
	}
	l2, err := NewLogWithStore("op-quid", store2)
	if err != nil {
		t.Fatalf("rewire log: %v", err)
	}
	defer l2.Close() // close on test teardown so Windows can clean up TempDir

	if got := l2.Height(); got != 5 {
		t.Errorf("expected height 5 after replay, got %d", got)
	}
	if idx, err := l2.VerifyChain(); idx != -1 || err != nil {
		t.Errorf("chain should verify after reload: idx=%d err=%v", idx, err)
	}
}

func TestFileStore_FileOnDiskIsJSONLines(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "audit.jsonl")

	store, err := NewFileStore(path)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	l, err := NewLogWithStore("op-quid", store)
	if err != nil {
		t.Fatalf("wire: %v", err)
	}
	_, _ = l.Append(Entry{Category: CategoryOperatorOther, Payload: map[string]interface{}{}})
	_, _ = l.Append(Entry{Category: CategoryOperatorOther, Payload: map[string]interface{}{}})
	_ = l.Close()

	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read file: %v", err)
	}
	if n := countLines(raw); n != 2 {
		t.Errorf("expected 2 JSON lines on disk, got %d", n)
	}
}

// countLines counts how many LF-terminated lines are in data.
// Helper only used by the on-disk shape assertion.
func countLines(data []byte) int {
	n := 0
	for _, b := range data {
		if b == '\n' {
			n++
		}
	}
	return n
}

func TestHashIsStableAcrossPayloadKeyOrder(t *testing.T) {
	// Same payload, same timestamp, same operator, same sequence
	// must always hash to the same value even if the caller
	// built the payload with different key-insertion order.
	a := Entry{
		Sequence:     7,
		PrevHash:     ZeroPrevHash,
		Timestamp:    1700000000,
		OperatorQuid: "op",
		Category:     CategoryValidatorEdit,
		Payload:      map[string]interface{}{"b": 2, "a": 1, "c": 3},
	}
	b := Entry{
		Sequence:     7,
		PrevHash:     ZeroPrevHash,
		Timestamp:    1700000000,
		OperatorQuid: "op",
		Category:     CategoryValidatorEdit,
		Payload:      map[string]interface{}{"c": 3, "a": 1, "b": 2},
	}

	ha, err := computeEntryHash(a)
	if err != nil {
		t.Fatal(err)
	}
	hb, err := computeEntryHash(b)
	if err != nil {
		t.Fatal(err)
	}
	if ha != hb {
		t.Errorf("hashes diverged across payload order: %s vs %s", ha, hb)
	}
}
