package core

import (
	"testing"
)

// TestRouting_SortedForwardPeersByScore: when the scoreboard
// has different composites for two peers, sortedForwardPeers
// returns the higher-scored peer first.
func TestRouting_SortedForwardPeersByScore(t *testing.T) {
	node := newTestNode()
	node.PeerScoreboard = NewPeerScoreboard(DefaultPeerScoreWeights(), "", 0)

	// Both peers in KnownNodes.
	node.KnownNodes["good"] = Node{ID: "good", Address: "10.0.0.1:1"}
	node.KnownNodes["bad"] = Node{ID: "bad", Address: "10.0.0.2:1"}

	// Make "good" actually good and "bad" actually bad.
	for i := 0; i < 30; i++ {
		node.PeerScoreboard.Record("good", EventClassValidation, true, "")
	}
	node.PeerScoreboard.RecordSevere("bad", SevereForkClaim, "")
	node.PeerScoreboard.RecordSevere("bad", SevereForkClaim, "")

	out := node.sortedForwardPeers()
	if len(out) != 2 {
		t.Fatalf("want 2 peers, got %d", len(out))
	}
	if out[0].ID != "good" || out[1].ID != "bad" {
		t.Fatalf("expected [good, bad], got [%s, %s]", out[0].ID, out[1].ID)
	}
}

// TestRouting_QuarantinedExcluded: quarantined peers are
// filtered out of the forward set entirely.
func TestRouting_QuarantinedExcluded(t *testing.T) {
	node := newTestNode()
	node.PeerScoreboard = NewPeerScoreboard(DefaultPeerScoreWeights(), "", 0)

	node.KnownNodes["alice"] = Node{ID: "alice", Address: "10.0.0.1:1"}
	node.KnownNodes["bob"] = Node{ID: "bob", Address: "10.0.0.2:1"}
	node.KnownNodes["sketchy"] = Node{ID: "sketchy", Address: "10.0.0.3:1"}

	node.PeerScoreboard.SetQuarantined("sketchy", true, "test")

	out := node.sortedForwardPeers()
	for _, p := range out {
		if p.ID == "sketchy" {
			t.Fatal("quarantined peer should be excluded from forward set")
		}
	}
	if len(out) != 2 {
		t.Fatalf("want 2 non-quarantined peers, got %d", len(out))
	}
}

// TestRouting_PreferByScoreFiltersQuarantine: preferByScore
// (used by query candidate ordering) drops quarantined peers
// from the candidate list.
func TestRouting_PreferByScoreFiltersQuarantine(t *testing.T) {
	node := newTestNode()
	node.PeerScoreboard = NewPeerScoreboard(DefaultPeerScoreWeights(), "", 0)

	in := []Node{
		{ID: "a", Address: "10.0.0.1:1"},
		{ID: "b", Address: "10.0.0.2:1"},
		{ID: "q", Address: "10.0.0.3:1"},
	}
	node.PeerScoreboard.SetQuarantined("q", true, "test")

	out := node.preferByScore(in)
	if len(out) != 2 {
		t.Fatalf("want 2 (q excluded), got %d", len(out))
	}
	for _, n := range out {
		if n.ID == "q" {
			t.Fatal("preferByScore did not filter quarantined")
		}
	}
}

// TestRouting_NilScoreboardFallback: when the scoreboard is nil
// (typical in older tests), sortedForwardPeers falls back to
// ID-sort instead of panicking.
func TestRouting_NilScoreboardFallback(t *testing.T) {
	node := newTestNode()
	node.PeerScoreboard = nil

	node.KnownNodes["zeta"] = Node{ID: "zeta", Address: "10.0.0.1:1"}
	node.KnownNodes["alpha"] = Node{ID: "alpha", Address: "10.0.0.2:1"}

	out := node.sortedForwardPeers()
	if len(out) != 2 {
		t.Fatalf("want 2 peers, got %d", len(out))
	}
	if out[0].ID != "alpha" {
		t.Fatalf("expected ID-sort [alpha, zeta], got [%s, %s]", out[0].ID, out[1].ID)
	}
}
