package core

import (
	"testing"
	"time"
)

// TestReattest_MissingAdRecordsRevocation: when a peer is in
// KnownNodes but has no current advertisement and the admit
// pipeline requires ads, runReattestOnce records an
// AdRevocation severe event.
func TestReattest_MissingAdRecordsRevocation(t *testing.T) {
	node := newTestNode()
	node.PeerScoreboard = NewPeerScoreboard(DefaultPeerScoreWeights(), "", 0)
	node.PeerAdmit = PeerAdmitConfig{RequireAdvertisement: true}

	node.KnownNodes["lonely"] = Node{
		ID: "lonely", Address: "10.0.0.1:1", ConnectionStatus: "gossip",
	}

	node.runReattestOnce()

	snap := node.PeerScoreboard.SnapshotOne("lonely")
	if snap == nil {
		t.Fatal("expected score record")
	}
	if snap.AdRevocations != 1 {
		t.Fatalf("AdRevocations=%d want 1", snap.AdRevocations)
	}
}

// TestReattest_NoAdsRequiredNoEvent: when the admit pipeline
// doesn't require ads, missing-ad is silent (no severe event).
func TestReattest_NoAdsRequiredNoEvent(t *testing.T) {
	node := newTestNode()
	node.PeerScoreboard = NewPeerScoreboard(DefaultPeerScoreWeights(), "", 0)
	// RequireAdvertisement default zero (false).

	node.KnownNodes["chill"] = Node{
		ID: "chill", Address: "10.0.0.2:1", ConnectionStatus: "gossip",
	}

	node.runReattestOnce()

	snap := node.PeerScoreboard.SnapshotOne("chill")
	if snap != nil && snap.AdRevocations > 0 {
		t.Fatalf("did not expect AdRevocation in non-strict mode: %+v", snap)
	}
}

// TestReportPeerForkClaim_Quarantine: 2 fork claims under the
// "quarantine" action move the peer into the quarantine pool.
func TestReportPeerForkClaim_Quarantine(t *testing.T) {
	node := newTestNode()
	node.PeerScoreboard = NewPeerScoreboard(DefaultPeerScoreWeights(), "", 0)
	node.KnownNodes["forky"] = Node{
		ID: "forky", Address: "10.0.0.3:1", ConnectionStatus: "gossip",
	}

	// First fork claim: should NOT auto-quarantine (need ≥2).
	node.ReportPeerForkClaim("forky", "quarantine", time.Hour, "block 5 mismatch")
	if node.PeerScoreboard.IsQuarantined("forky") {
		t.Fatal("first fork claim should not auto-quarantine")
	}

	// Second fork claim: should trigger quarantine.
	node.ReportPeerForkClaim("forky", "quarantine", time.Hour, "block 7 mismatch")
	if !node.PeerScoreboard.IsQuarantined("forky") {
		t.Fatal("second fork claim should auto-quarantine")
	}
}

// TestReportPeerForkClaim_Evict: under the "evict" action, the
// FIRST fork claim immediately removes the peer from KnownNodes
// — even if the peer is static-source.
func TestReportPeerForkClaim_Evict(t *testing.T) {
	node := newTestNode()
	node.PeerScoreboard = NewPeerScoreboard(DefaultPeerScoreWeights(), "", 0)
	node.KnownNodes["villain"] = Node{
		ID: "villain", Address: "10.0.0.4:1", ConnectionStatus: "static",
	}

	node.ReportPeerForkClaim("villain", "evict", time.Hour, "block 5 mismatch")

	if _, ok := node.KnownNodes["villain"]; ok {
		t.Fatal("evict action should remove peer from KnownNodes immediately, even under static-immunity")
	}
}

// TestReportPeerForkClaim_LogOnly: under the "log" action, a
// fork claim records the severe event but takes no automatic
// action.
func TestReportPeerForkClaim_LogOnly(t *testing.T) {
	node := newTestNode()
	node.PeerScoreboard = NewPeerScoreboard(DefaultPeerScoreWeights(), "", 0)
	node.KnownNodes["bystander"] = Node{
		ID: "bystander", Address: "10.0.0.5:1", ConnectionStatus: "gossip",
	}

	for i := 0; i < 5; i++ {
		node.ReportPeerForkClaim("bystander", "log", time.Hour, "block X mismatch")
	}
	if node.PeerScoreboard.IsQuarantined("bystander") {
		t.Fatal("log-only action should not quarantine")
	}
	if _, ok := node.KnownNodes["bystander"]; !ok {
		t.Fatal("log-only action should not evict")
	}
	snap := node.PeerScoreboard.SnapshotOne("bystander")
	if snap == nil || snap.ForkClaims != 5 {
		t.Fatalf("expected 5 fork claims recorded, got %+v", snap)
	}
}
