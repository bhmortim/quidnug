// Package core — domain_fingerprint_test.go
//
// Methodology
// -----------
// DomainFingerprint is the trust anchor for cross-domain gossip
// (QDP-0003 §7.3). If fingerprint signature semantics are wrong,
// every layer built on top is compromised. These tests exhaustively
// cover the signer side (Produce + Sign), the verifier side, and the
// monotonic-store semantics.
//
//   * Produce: fingerprint for a domain this node has sealed a block
//     in succeeds; for a domain with no blocks, returns an explicit
//     error rather than a zero-value fingerprint.
//   * Sign+verify: round-trip through the ledger's SetSignerKey →
//     GetSignerKey path, so a canonicalization bug between signer
//     and verifier shows up here.
//   * Verify rejections: wrong schema, missing producer key,
//     tampered block hash, stale timestamp.
//   * Store: monotonic on block height — older fingerprints don't
//     overwrite newer ones, regardless of arrival order.
package core

import (
	"errors"
	"testing"
	"time"
)

func TestProduceDomainFingerprint_ErrorsWhenDomainAbsent(t *testing.T) {
	node := newTestNode()
	_, err := node.ProduceDomainFingerprint("never-seen.example", time.Now())
	if err == nil {
		t.Fatal("expected error for domain with no blocks")
	}
}

func TestProduceDomainFingerprint_UsesLatestBlockForDomain(t *testing.T) {
	node := newTestNode()
	domain := "test.domain.com"

	// Inject two blocks for the domain. ProduceDomainFingerprint
	// must pick the later one.
	node.BlockchainMutex.Lock()
	node.Blockchain = append(node.Blockchain,
		Block{
			Index:      1,
			Timestamp:  time.Now().Unix(),
			Hash:       "hashA",
			TrustProof: TrustProof{TrustDomain: domain},
		},
		Block{
			Index:      2,
			Timestamp:  time.Now().Unix(),
			Hash:       "hashB",
			TrustProof: TrustProof{TrustDomain: domain},
		},
	)
	node.BlockchainMutex.Unlock()

	fp, err := node.ProduceDomainFingerprint(domain, time.Now())
	if err != nil {
		t.Fatalf("produce: %v", err)
	}
	if fp.BlockHeight != 2 || fp.BlockHash != "hashB" {
		t.Fatalf("want latest block (2/hashB), got %d/%s", fp.BlockHeight, fp.BlockHash)
	}
	if fp.Domain != domain {
		t.Fatalf("domain: want %q, got %q", domain, fp.Domain)
	}
	if fp.ProducerQuid != node.NodeID {
		t.Fatalf("producer: want %s, got %s", node.NodeID, fp.ProducerQuid)
	}
}

func TestSignAndVerifyDomainFingerprint_RoundTrip(t *testing.T) {
	node := newTestNode()
	node.NonceLedger.SetSignerKey(node.NodeID, 0, node.GetPublicKeyHex())

	fp := DomainFingerprint{
		SchemaVersion: DomainFingerprintSchemaVersion,
		Domain:        "d1",
		BlockHeight:   7,
		BlockHash:     "hash-under-test",
		Timestamp:     time.Now().Unix(),
		ProducerQuid:  node.NodeID,
	}
	signed, err := node.SignDomainFingerprint(fp)
	if err != nil {
		t.Fatalf("sign: %v", err)
	}
	if signed.Signature == "" {
		t.Fatal("signature field is empty after SignDomainFingerprint")
	}
	if err := VerifyDomainFingerprint(node.NonceLedger, signed, time.Now()); err != nil {
		t.Fatalf("verify: %v", err)
	}
}

func TestVerifyDomainFingerprint_RejectsTamperedBlockHash(t *testing.T) {
	node := newTestNode()
	node.NonceLedger.SetSignerKey(node.NodeID, 0, node.GetPublicKeyHex())

	fp := DomainFingerprint{
		SchemaVersion: DomainFingerprintSchemaVersion,
		Domain:        "d1",
		BlockHeight:   7,
		BlockHash:     "real-hash",
		Timestamp:     time.Now().Unix(),
		ProducerQuid:  node.NodeID,
	}
	signed, _ := node.SignDomainFingerprint(fp)
	signed.BlockHash = "attacker-substituted-hash"

	if err := VerifyDomainFingerprint(node.NonceLedger, signed, time.Now()); !errors.Is(err, ErrFingerprintBadSignature) {
		t.Fatalf("want ErrFingerprintBadSignature, got %v", err)
	}
}

func TestVerifyDomainFingerprint_RejectsUnknownProducer(t *testing.T) {
	node := newTestNode()
	// NewQuidnugNode seeds the node's own epoch-0 key automatically,
	// so we need a genuinely unknown party here. Using a second
	// test node gives us one whose private key we can still sign
	// with but whose public key never landed in `node`'s ledger.
	stranger := newTestNode()

	fp := DomainFingerprint{
		SchemaVersion: DomainFingerprintSchemaVersion,
		Domain:        "d1",
		BlockHeight:   1,
		BlockHash:     "hash",
		Timestamp:     time.Now().Unix(),
		ProducerQuid:  stranger.NodeID,
	}
	signed, _ := stranger.SignDomainFingerprint(fp)

	// Even with a valid signature, `node` can't find the key so
	// this must return NoProducerKey rather than "bad signature".
	if err := VerifyDomainFingerprint(node.NonceLedger, signed, time.Now()); !errors.Is(err, ErrFingerprintNoProdKey) {
		t.Fatalf("want ErrFingerprintNoProdKey, got %v", err)
	}
}

func TestVerifyDomainFingerprint_RejectsStaleTimestamp(t *testing.T) {
	node := newTestNode()
	node.NonceLedger.SetSignerKey(node.NodeID, 0, node.GetPublicKeyHex())

	fp := DomainFingerprint{
		SchemaVersion: DomainFingerprintSchemaVersion,
		Domain:        "d1",
		BlockHeight:   1,
		BlockHash:     "hash",
		Timestamp:     time.Now().Add(-60 * 24 * time.Hour).Unix(), // 60 days old
		ProducerQuid:  node.NodeID,
	}
	signed, _ := node.SignDomainFingerprint(fp)
	if err := VerifyDomainFingerprint(node.NonceLedger, signed, time.Now()); !errors.Is(err, ErrFingerprintStale) {
		t.Fatalf("want ErrFingerprintStale, got %v", err)
	}
}

func TestVerifyDomainFingerprint_RejectsWrongSchema(t *testing.T) {
	node := newTestNode()
	node.NonceLedger.SetSignerKey(node.NodeID, 0, node.GetPublicKeyHex())

	fp := DomainFingerprint{
		SchemaVersion: 999,
		Domain:        "d1",
		BlockHeight:   1,
		BlockHash:     "hash",
		Timestamp:     time.Now().Unix(),
		ProducerQuid:  node.NodeID,
	}
	signed, _ := node.SignDomainFingerprint(fp)
	if err := VerifyDomainFingerprint(node.NonceLedger, signed, time.Now()); !errors.Is(err, ErrFingerprintBadSchema) {
		t.Fatalf("want ErrFingerprintBadSchema, got %v", err)
	}
}

func TestStoreDomainFingerprint_MonotonicOnBlockHeight(t *testing.T) {
	l := NewNonceLedger()

	a := DomainFingerprint{Domain: "d", BlockHeight: 1, BlockHash: "a"}
	b := DomainFingerprint{Domain: "d", BlockHeight: 3, BlockHash: "b"}
	c := DomainFingerprint{Domain: "d", BlockHeight: 2, BlockHash: "c"} // regression

	l.StoreDomainFingerprint(a)
	l.StoreDomainFingerprint(b)
	l.StoreDomainFingerprint(c)

	got, ok := l.GetDomainFingerprint("d")
	if !ok {
		t.Fatal("no fingerprint stored")
	}
	if got.BlockHeight != 3 || got.BlockHash != "b" {
		t.Fatalf("want latest (3/b) preserved, got %d/%s", got.BlockHeight, got.BlockHash)
	}
}
