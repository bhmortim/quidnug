package core

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

// TestPeerScore_NeutralStart confirms a peer with no events
// reports a 0.5 composite (neutral). Important for cold-start
// behavior — a newly-admitted peer with no track record
// shouldn't be punished.
func TestPeerScore_NeutralStart(t *testing.T) {
	sb := NewPeerScoreboard(DefaultPeerScoreWeights(), "", 0)
	sb.SetAdmittedAt("alice", time.Now())
	got := sb.Composite("alice")
	if got != 0.5 {
		t.Fatalf("composite=%v want 0.5 (neutral)", got)
	}
}

// TestPeerScore_AllSuccessTendsToOne verifies that a peer with
// only successes converges to 1.0 across enough events.
func TestPeerScore_AllSuccessTendsToOne(t *testing.T) {
	sb := NewPeerScoreboard(DefaultPeerScoreWeights(), "", 0)
	for i := 0; i < 50; i++ {
		sb.Record("alice", EventClassValidation, true, "")
		sb.Record("alice", EventClassHandshake, true, "")
		sb.Record("alice", EventClassQuery, true, "")
		sb.Record("alice", EventClassGossip, true, "")
		sb.Record("alice", EventClassBroadcast, true, "")
	}
	got := sb.Composite("alice")
	if got < 0.95 {
		t.Fatalf("composite=%v want ≥0.95 after 50 wins per class", got)
	}
}

// TestPeerScore_ForkClaimDragsScore verifies that severe events
// pull the composite down below the eviction threshold even
// when per-class rates are perfect.
func TestPeerScore_ForkClaimDragsScore(t *testing.T) {
	sb := NewPeerScoreboard(DefaultPeerScoreWeights(), "", 0)
	for i := 0; i < 20; i++ {
		sb.Record("evil", EventClassValidation, true, "")
		sb.Record("evil", EventClassHandshake, true, "")
	}
	pre := sb.Composite("evil")
	if pre < 0.6 {
		t.Fatalf("pre-fork score %v should be high", pre)
	}
	// One fork claim subtracts 0.20 — still above eviction.
	sb.RecordSevere("evil", SevereForkClaim, "block 5 mismatch")
	mid := sb.Composite("evil")
	// A second fork claim should land us in quarantine territory.
	sb.RecordSevere("evil", SevereForkClaim, "block 7 mismatch")
	post := sb.Composite("evil")
	if mid <= post {
		t.Fatalf("expected score to keep dropping with more forks (mid=%v post=%v)", mid, post)
	}
	if post >= mid-0.1 {
		t.Fatalf("expected ≥0.1 drop per fork claim (mid=%v post=%v)", mid, post)
	}
}

// TestPeerScore_DecaysOverTime verifies that successes decay so
// a peer that was good once and is now silent doesn't keep its
// halo forever. We force decay by manipulating LastTick.
func TestPeerScore_DecaysOverTime(t *testing.T) {
	sb := NewPeerScoreboard(DefaultPeerScoreWeights(), "", 0)
	for i := 0; i < 10; i++ {
		sb.Record("alice", EventClassValidation, true, "")
	}
	// Force the lastTick back to simulate hours of silence.
	sb.mu.Lock()
	if p, ok := sb.scores["alice"]; ok {
		p.mu.Lock()
		p.Validation.LastTick = time.Now().Add(-2 * time.Hour)
		p.mu.Unlock()
	}
	sb.mu.Unlock()
	c := sb.Composite("alice")
	// 2 hours / 15-min half-life = 8 half-lives → ~0.4% remaining.
	// Validation rate effectively 0.5 (neutral) again. Composite
	// approaches 0.5.
	if c < 0.45 || c > 0.55 {
		t.Fatalf("composite after 2h decay = %v want ~0.5", c)
	}
}

// TestPeerScore_Quarantine sets and reads quarantine state.
func TestPeerScore_Quarantine(t *testing.T) {
	sb := NewPeerScoreboard(DefaultPeerScoreWeights(), "", 0)
	if sb.IsQuarantined("alice") {
		t.Fatal("new peer should not be quarantined")
	}
	prior := sb.SetQuarantined("alice", true, "score below threshold")
	if prior {
		t.Fatal("first quarantine call should report prior=false")
	}
	if !sb.IsQuarantined("alice") {
		t.Fatal("expected quarantined")
	}
	snap := sb.SnapshotOne("alice")
	if snap == nil || !snap.Quarantined || snap.QuarantineReason == "" {
		t.Fatalf("snapshot lost quarantine fields: %+v", snap)
	}
	sb.SetQuarantined("alice", false, "")
	if sb.IsQuarantined("alice") {
		t.Fatal("expected un-quarantined")
	}
}

// TestPeerScore_PersistRoundTrip writes the scoreboard to disk,
// loads it into a fresh scoreboard, and confirms key state
// survives.
func TestPeerScore_PersistRoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "peer_scores.json")
	a := NewPeerScoreboard(DefaultPeerScoreWeights(), path, 0)
	a.SetAdmittedAt("alice", time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC))
	for i := 0; i < 5; i++ {
		a.Record("alice", EventClassQuery, true, "")
	}
	a.RecordSevere("alice", SevereSignatureFail, "ECDSA verify failed")
	a.SetQuarantined("alice", true, "manual operator action")
	if err := a.persistOnce(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("persistOnce: %v", err)
	}
	b := NewPeerScoreboard(DefaultPeerScoreWeights(), path, 0)
	if err := b.LoadFrom(path); err != nil {
		t.Fatalf("load: %v", err)
	}
	snap := b.SnapshotOne("alice")
	if snap == nil {
		t.Fatal("alice missing after reload")
	}
	if snap.SignatureFails != 1 {
		t.Fatalf("severe count lost: %d", snap.SignatureFails)
	}
	if !snap.Quarantined || snap.QuarantineReason != "manual operator action" {
		t.Fatalf("quarantine state lost: %+v", snap)
	}
	if snap.Query.Successes < 4 {
		t.Fatalf("query successes lost: %v", snap.Query.Successes)
	}
}

// TestPeerScore_RingBuffer caps event history at the documented
// size and rolls oldest-out when full.
func TestPeerScore_RingBuffer(t *testing.T) {
	sb := NewPeerScoreboard(DefaultPeerScoreWeights(), "", 0)
	for i := 0; i < PeerScoreEventRingSize+10; i++ {
		sb.Record("alice", EventClassQuery, true, "")
	}
	snap := sb.SnapshotOne("alice")
	if snap == nil {
		t.Fatal("missing alice")
	}
	if len(snap.RecentEvents) != PeerScoreEventRingSize {
		t.Fatalf("ring size %d want %d", len(snap.RecentEvents), PeerScoreEventRingSize)
	}
}

// TestPeerScore_NilToleranceOnHelpers confirms node-side
// helpers tolerate a nil scoreboard, so existing tests that
// don't initialize one don't crash on telemetry calls.
func TestPeerScore_NilToleranceOnHelpers(t *testing.T) {
	var n *QuidnugNode
	// Should not panic.
	n.recordPeerScore("alice", EventClassQuery, true, "")
	n2 := &QuidnugNode{} // zero PeerScoreboard
	n2.recordPeerScore("alice", EventClassQuery, true, "")
	n2.recordPeerSevere("alice", SevereForkClaim, "")
}

// TestPeerScore_SnapshotSortsWorstFirst is the operator-facing
// guarantee: scoreboard snapshots are sorted ascending by
// composite so worst peers surface first.
func TestPeerScore_SnapshotSortsWorstFirst(t *testing.T) {
	sb := NewPeerScoreboard(DefaultPeerScoreWeights(), "", 0)
	for i := 0; i < 10; i++ {
		sb.Record("good", EventClassValidation, true, "")
	}
	for i := 0; i < 10; i++ {
		sb.Record("bad", EventClassValidation, false, "")
	}
	snaps := sb.Snapshot()
	if len(snaps) < 2 {
		t.Fatalf("want 2 snapshots, got %d", len(snaps))
	}
	if snaps[0].Composite >= snaps[1].Composite {
		t.Fatalf("not sorted ascending: %v vs %v", snaps[0].Composite, snaps[1].Composite)
	}
	if snaps[0].NodeQuid != "bad" || snaps[len(snaps)-1].NodeQuid != "good" {
		t.Fatalf("worst-first order broken: %+v", snaps)
	}
}
