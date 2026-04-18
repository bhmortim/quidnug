package core

import (
	"errors"
	"reflect"
	"testing"
	"time"
)

func TestProduceSnapshot_EmptyLedgerYieldsEmptyEntries(t *testing.T) {
	ledger := NewNonceLedger()
	snap := ledger.ProduceSnapshot(1, "hash", "d", "producer", time.Now())

	if len(snap.Entries) != 0 {
		t.Fatalf("empty ledger should produce no entries, got %d", len(snap.Entries))
	}
	if snap.TrustDomain != "d" {
		t.Fatalf("domain: want 'd', got %q", snap.TrustDomain)
	}
	if snap.SchemaVersion != SnapshotSchemaVersion {
		t.Fatalf("schema version: want %d, got %d", SnapshotSchemaVersion, snap.SchemaVersion)
	}
}

func TestProduceSnapshot_FiltersByDomain(t *testing.T) {
	ledger := NewNonceLedger()
	ledger.CommitAccepted(NonceKey{Quid: "a", Domain: "d1"}, 5)
	ledger.CommitAccepted(NonceKey{Quid: "b", Domain: "d1"}, 3)
	ledger.CommitAccepted(NonceKey{Quid: "c", Domain: "d2"}, 10)

	snap := ledger.ProduceSnapshot(2, "hash", "d1", "producer", time.Now())
	if len(snap.Entries) != 2 {
		t.Fatalf("want 2 entries for d1, got %d: %+v", len(snap.Entries), snap.Entries)
	}
	for _, e := range snap.Entries {
		if e.Quid == "c" {
			t.Fatalf("d2 entry leaked into d1 snapshot: %+v", e)
		}
	}
}

func TestProduceSnapshot_DeterministicOrdering(t *testing.T) {
	ledger := NewNonceLedger()
	// Add in non-sorted order across both Quid and Epoch dimensions.
	ledger.CommitAccepted(NonceKey{Quid: "bbb", Domain: "d"}, 5)
	ledger.CommitAccepted(NonceKey{Quid: "aaa", Domain: "d", Epoch: 2}, 1)
	ledger.CommitAccepted(NonceKey{Quid: "aaa", Domain: "d", Epoch: 1}, 10)

	s1 := ledger.ProduceSnapshot(1, "h", "d", "producer", time.Now())
	s2 := ledger.ProduceSnapshot(1, "h", "d", "producer", time.Now())
	if !reflect.DeepEqual(s1.Entries, s2.Entries) {
		t.Fatalf("non-deterministic snapshot ordering:\n  s1=%+v\n  s2=%+v", s1.Entries, s2.Entries)
	}
	// Expected order: aaa@1, aaa@2, bbb@0.
	want := []NonceSnapshotEntry{
		{Quid: "aaa", Epoch: 1, MaxNonce: 10},
		{Quid: "aaa", Epoch: 2, MaxNonce: 1},
		{Quid: "bbb", Epoch: 0, MaxNonce: 5},
	}
	if !reflect.DeepEqual(s1.Entries, want) {
		t.Fatalf("unexpected order:\n  got  %+v\n  want %+v", s1.Entries, want)
	}
}

func TestSignAndVerifySnapshot_RoundTrip(t *testing.T) {
	ledger := NewNonceLedger()
	ledger.CommitAccepted(NonceKey{Quid: "a", Domain: "d"}, 5)

	node := newTestNode()
	ledger.SetSignerKey(node.NodeID, 0, node.GetPublicKeyHex())

	unsigned := ledger.ProduceSnapshot(1, "hash", "d", node.NodeID, time.Now())
	signed, err := node.SignSnapshot(unsigned)
	if err != nil {
		t.Fatalf("SignSnapshot: %v", err)
	}
	if signed.Signature == "" {
		t.Fatal("SignSnapshot left Signature empty")
	}
	if err := VerifySnapshot(ledger, signed); err != nil {
		t.Fatalf("VerifySnapshot: %v", err)
	}
}

func TestVerifySnapshot_RejectsBadSignature(t *testing.T) {
	ledger := NewNonceLedger()
	node := newTestNode()
	ledger.SetSignerKey(node.NodeID, 0, node.GetPublicKeyHex())

	snap := ledger.ProduceSnapshot(1, "hash", "d", node.NodeID, time.Now())
	signed, _ := node.SignSnapshot(snap)

	// Tamper with the Entries after signing.
	signed.Entries = append(signed.Entries, NonceSnapshotEntry{Quid: "attacker", Epoch: 0, MaxNonce: 9999})
	if err := VerifySnapshot(ledger, signed); !errors.Is(err, ErrSnapshotBadSignature) {
		t.Fatalf("want ErrSnapshotBadSignature, got %v", err)
	}
}

func TestVerifySnapshot_RejectsUnknownProducer(t *testing.T) {
	ledger := NewNonceLedger()
	node := newTestNode()
	// Deliberately do NOT register node's key.

	snap := ledger.ProduceSnapshot(1, "hash", "d", node.NodeID, time.Now())
	signed, _ := node.SignSnapshot(snap)
	if err := VerifySnapshot(ledger, signed); !errors.Is(err, ErrSnapshotNoProducerKey) {
		t.Fatalf("want ErrSnapshotNoProducerKey, got %v", err)
	}
}

func TestVerifySnapshot_RejectsWrongSchema(t *testing.T) {
	ledger := NewNonceLedger()
	node := newTestNode()
	ledger.SetSignerKey(node.NodeID, 0, node.GetPublicKeyHex())

	snap := ledger.ProduceSnapshot(1, "hash", "d", node.NodeID, time.Now())
	snap.SchemaVersion = 999
	signed, _ := node.SignSnapshot(snap)
	if err := VerifySnapshot(ledger, signed); !errors.Is(err, ErrSnapshotBadSchema) {
		t.Fatalf("want ErrSnapshotBadSchema, got %v", err)
	}
}
