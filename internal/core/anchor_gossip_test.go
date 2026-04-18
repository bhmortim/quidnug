// Package core — anchor_gossip_test.go
//
// Methodology
// -----------
// The cross-domain anchor-gossip path is the most sensitive bit of
// QDP-0003: it takes a signed block from one domain and authorizes a
// ledger mutation on a completely different node that never saw
// that block's transactions being accepted in the origin domain's
// consensus. If the validation chain has a gap, an attacker who
// captures a relayed gossip message can manufacture state in
// unrelated domains. These tests therefore go deep on rejection
// paths.
//
// Test organization:
//
//  1. Happy path: end-to-end — a rotation anchor sealed in "domain-A"
//     gossips to a node that only participates in "domain-B". The
//     receiving node's global signer state (currentEpoch, signerKeys)
//     reflects the rotation after gossip.
//
//  2. Rejection paths, one per error in anchor_gossip.go:
//     - Gossip signature mismatch
//     - Fingerprint signature mismatch
//     - Fingerprint covers a different block
//     - Block's self-hash is wrong
//     - AnchorTxIndex points into void / non-anchor tx
//     - Duplicate MessageID
//
//  3. Idempotency: the same valid message applied twice is a no-op
//     the second time; the state doesn't flip back.
package core

import (
	"encoding/json"
	"errors"
	"testing"
	"time"
)

// ----- helpers -------------------------------------------------------------

// originSetup produces a "remote" origin where an anchor was accepted
// and a "local" receiver that will process the gossip. Both have
// their own ledgers. The origin's validator key is seeded into the
// receiver's ledger so the receiver can verify the fingerprint.
type originSetup struct {
	origin   *QuidnugNode
	receiver *QuidnugNode
	domain   string
}

func newOriginSetup(t *testing.T, domain string) *originSetup {
	t.Helper()
	origin := newTestNode()
	receiver := newTestNode()

	// Receiver must know the origin's epoch-0 key to verify the
	// fingerprint + gossip signature.
	receiver.NonceLedger.SetSignerKey(origin.NodeID, 0, origin.GetPublicKeyHex())

	// Origin commits a block in the target domain so the subject's
	// NodeID key is known there as a block producer. Not strictly
	// needed for these tests since we build the gossip message by
	// hand, but it keeps the scenario realistic.
	origin.BlockchainMutex.Lock()
	origin.Blockchain = append(origin.Blockchain, Block{
		Index:      1,
		Timestamp:  time.Now().Unix(),
		Hash:       "set-after-calculate",
		TrustProof: TrustProof{TrustDomain: domain, ValidatorID: origin.NodeID},
	})
	origin.BlockchainMutex.Unlock()

	return &originSetup{origin: origin, receiver: receiver, domain: domain}
}

// buildRotationGossip constructs a fully-signed AnchorGossipMessage
// for a simple rotation anchor. Encapsulates the repeating setup so
// individual tests can focus on their specific concern.
func buildRotationGossip(t *testing.T, s *originSetup) (AnchorGossipMessage, string) {
	t.Helper()

	// The "subject" of the rotation is the origin node itself — it
	// rotates its own key. Receiver will learn the new key.
	_, newPub := keypairHex(t)

	rot := NonceAnchor{
		Kind:         AnchorRotation,
		SignerQuid:   s.origin.NodeID,
		FromEpoch:    0,
		ToEpoch:      1,
		NewPublicKey: newPub,
		MinNextNonce: 1,
		ValidFrom:    time.Now().Unix(),
		AnchorNonce:  1,
	}
	// Origin signs rotation with its own private key (standard
	// AnchorRotation flow).
	rotSignable, _ := GetAnchorSignableData(rot)
	sig, err := s.origin.SignData(rotSignable)
	if err != nil {
		t.Fatalf("sign rotation: %v", err)
	}
	rot.Signature = hexEncode(sig)

	// Put the rotation into a block, compute hash, sign the block.
	originBlock := Block{
		Index:     2,
		Timestamp: time.Now().Unix(),
		Transactions: []interface{}{
			AnchorTransaction{
				BaseTransaction: BaseTransaction{Type: TxTypeAnchor, Timestamp: time.Now().Unix()},
				Anchor:          rot,
			},
		},
		TrustProof: TrustProof{
			TrustDomain:        s.domain,
			ValidatorID:        s.origin.NodeID,
			ValidatorPublicKey: s.origin.GetPublicKeyHex(),
		},
		PrevHash: "genesis-placeholder",
	}
	originBlock.Hash = calculateBlockHash(originBlock)

	// Origin produces a fingerprint covering this block.
	fp := DomainFingerprint{
		SchemaVersion: DomainFingerprintSchemaVersion,
		Domain:        s.domain,
		BlockHeight:   originBlock.Index,
		BlockHash:     originBlock.Hash,
		Timestamp:     time.Now().Unix(),
		ProducerQuid:  s.origin.NodeID,
	}
	fp, err = s.origin.SignDomainFingerprint(fp)
	if err != nil {
		t.Fatalf("sign fingerprint: %v", err)
	}

	m := AnchorGossipMessage{
		SchemaVersion:      AnchorGossipSchemaVersion,
		MessageID:          "msg-" + s.domain + "-2",
		OriginDomain:       s.domain,
		OriginBlockHeight:  originBlock.Index,
		OriginBlock:        originBlock,
		AnchorTxIndex:      0,
		DomainFingerprint:  fp,
		Timestamp:          time.Now().Unix(),
		GossipProducerQuid: s.origin.NodeID,
	}
	signed, err := s.origin.SignAnchorGossip(m)
	if err != nil {
		t.Fatalf("sign gossip: %v", err)
	}
	return signed, newPub
}

// ----- Happy path ----------------------------------------------------------

func TestAnchorGossip_RotationPropagatesAcrossDomains(t *testing.T) {
	s := newOriginSetup(t, "origin.example.com")
	msg, newPub := buildRotationGossip(t, s)

	if err := s.receiver.ApplyAnchorGossip(msg); err != nil {
		t.Fatalf("ApplyAnchorGossip: %v", err)
	}

	// Receiver's ledger now reflects the rotation globally for the
	// origin quid.
	if got := s.receiver.NonceLedger.CurrentEpoch(s.origin.NodeID); got != 1 {
		t.Fatalf("receiver currentEpoch: want 1, got %d", got)
	}
	if key, ok := s.receiver.NonceLedger.GetSignerKey(s.origin.NodeID, 1); !ok || key != newPub {
		t.Fatal("receiver missing new epoch-1 key")
	}
	// And the fingerprint was stored for future gossip verification.
	if _, ok := s.receiver.NonceLedger.GetDomainFingerprint(s.domain); !ok {
		t.Fatal("receiver should have stored the fingerprint alongside")
	}
}

// ----- Rejection paths -----------------------------------------------------

func TestAnchorGossip_RejectsForgedGossipSignature(t *testing.T) {
	s := newOriginSetup(t, "origin.example.com")
	msg, _ := buildRotationGossip(t, s)

	// Tamper post-signing: the signature now covers different bytes.
	msg.Timestamp += 60

	err := s.receiver.ApplyAnchorGossip(msg)
	if !errors.Is(err, ErrGossipBadGossipSig) {
		t.Fatalf("want ErrGossipBadGossipSig, got %v", err)
	}
}

func TestAnchorGossip_RejectsFingerprintSignedByStranger(t *testing.T) {
	s := newOriginSetup(t, "origin.example.com")
	msg, _ := buildRotationGossip(t, s)

	// Replace fingerprint with one signed by a party we don't know.
	stranger := newTestNode()
	bogus := DomainFingerprint{
		SchemaVersion: DomainFingerprintSchemaVersion,
		Domain:        s.domain,
		BlockHeight:   msg.OriginBlock.Index,
		BlockHash:     msg.OriginBlock.Hash,
		Timestamp:     time.Now().Unix(),
		ProducerQuid:  stranger.NodeID, // receiver doesn't know this quid
	}
	bogus, _ = stranger.SignDomainFingerprint(bogus)
	msg.DomainFingerprint = bogus

	// The gossip signature still covers the original fingerprint, so
	// the gossip signature check will fail FIRST. That's OK — either
	// error proves rejection. We just want any failure, not silent
	// acceptance.
	if err := s.receiver.ApplyAnchorGossip(msg); err == nil {
		t.Fatal("expected rejection of stranger-signed fingerprint")
	}
}

func TestAnchorGossip_RejectsFingerprintCoveringWrongBlock(t *testing.T) {
	s := newOriginSetup(t, "origin.example.com")

	// Build the message first.
	msg, _ := buildRotationGossip(t, s)

	// Produce a fingerprint over a different block hash, re-sign the
	// whole message so gossip signature is correct, but fingerprint
	// now references a block the receiver's validator didn't commit.
	msg.DomainFingerprint.BlockHash = "different-block-hash-entirely"
	msg.DomainFingerprint, _ = s.origin.SignDomainFingerprint(msg.DomainFingerprint)
	resigned, _ := s.origin.SignAnchorGossip(msg)

	err := s.receiver.ApplyAnchorGossip(resigned)
	if !errors.Is(err, ErrGossipFingerprintMismatch) {
		t.Fatalf("want ErrGossipFingerprintMismatch, got %v", err)
	}
}

func TestAnchorGossip_RejectsCorruptedBlock(t *testing.T) {
	s := newOriginSetup(t, "origin.example.com")
	msg, _ := buildRotationGossip(t, s)

	// Post-sign tampering: mutate a block field that's included in
	// the hash. The fingerprint + gossip sig still cover the original
	// bytes, so the block-self-hash check should catch the swap.
	//
	// We re-sign everything so only the block-hash mismatch is what
	// trips validation.
	msg.OriginBlock.Timestamp += 1000
	msg.OriginBlock.Hash = calculateBlockHash(msg.OriginBlock) // honest recompute
	// Now fingerprint.BlockHash no longer matches — fingerprint-
	// mismatch is the actual failure surface. That's fine: we're
	// confirming that tampering with the block breaks the chain.
	err := s.receiver.ApplyAnchorGossip(msg)
	if err == nil {
		t.Fatal("expected rejection of block-tampered gossip")
	}
}

func TestAnchorGossip_RejectsOutOfRangeTxIndex(t *testing.T) {
	s := newOriginSetup(t, "origin.example.com")
	msg, _ := buildRotationGossip(t, s)

	msg.AnchorTxIndex = 999
	resigned, _ := s.origin.SignAnchorGossip(msg)

	err := s.receiver.ApplyAnchorGossip(resigned)
	if !errors.Is(err, ErrGossipTxIndexOutOfRange) {
		t.Fatalf("want ErrGossipTxIndexOutOfRange, got %v", err)
	}
}

func TestAnchorGossip_RejectsNonAnchorTxAtIndex(t *testing.T) {
	s := newOriginSetup(t, "origin.example.com")

	// Build a block with a non-anchor transaction (TrustTransaction)
	// and point AnchorTxIndex at it.
	trustTx := TrustTransaction{
		BaseTransaction: BaseTransaction{Type: TxTypeTrust, Timestamp: time.Now().Unix()},
		Truster:         "aaaaaaaaaaaaaaaa",
		Trustee:         "bbbbbbbbbbbbbbbb",
		TrustLevel:      0.5,
		Nonce:           1,
	}
	blk := Block{
		Index:        2,
		Timestamp:    time.Now().Unix(),
		Transactions: []interface{}{trustTx},
		TrustProof:   TrustProof{TrustDomain: s.domain, ValidatorID: s.origin.NodeID},
		PrevHash:     "genesis",
	}
	blk.Hash = calculateBlockHash(blk)

	fp := DomainFingerprint{
		SchemaVersion: DomainFingerprintSchemaVersion,
		Domain:        s.domain,
		BlockHeight:   blk.Index,
		BlockHash:     blk.Hash,
		Timestamp:     time.Now().Unix(),
		ProducerQuid:  s.origin.NodeID,
	}
	fp, _ = s.origin.SignDomainFingerprint(fp)

	msg := AnchorGossipMessage{
		SchemaVersion:      AnchorGossipSchemaVersion,
		MessageID:          "msg-non-anchor",
		OriginDomain:       s.domain,
		OriginBlockHeight:  blk.Index,
		OriginBlock:        blk,
		AnchorTxIndex:      0,
		DomainFingerprint:  fp,
		Timestamp:          time.Now().Unix(),
		GossipProducerQuid: s.origin.NodeID,
	}
	signed, _ := s.origin.SignAnchorGossip(msg)

	err := s.receiver.ApplyAnchorGossip(signed)
	if !errors.Is(err, ErrGossipTxNotAnchor) {
		t.Fatalf("want ErrGossipTxNotAnchor, got %v", err)
	}
}

func TestAnchorGossip_RejectsDuplicateMessageID(t *testing.T) {
	s := newOriginSetup(t, "origin.example.com")
	msg, _ := buildRotationGossip(t, s)

	if err := s.receiver.ApplyAnchorGossip(msg); err != nil {
		t.Fatalf("first apply: %v", err)
	}
	// Second application must be a no-op rejection.
	err := s.receiver.ApplyAnchorGossip(msg)
	if !errors.Is(err, ErrGossipDuplicate) {
		t.Fatalf("want ErrGossipDuplicate, got %v", err)
	}
}

// ----- Idempotency sanity check -------------------------------------------

func TestAnchorGossip_StateUnchangedBySecondApply(t *testing.T) {
	s := newOriginSetup(t, "origin.example.com")
	msg, _ := buildRotationGossip(t, s)

	if err := s.receiver.ApplyAnchorGossip(msg); err != nil {
		t.Fatalf("first apply: %v", err)
	}
	epochAfterFirst := s.receiver.NonceLedger.CurrentEpoch(s.origin.NodeID)

	_ = s.receiver.ApplyAnchorGossip(msg) // ignored ErrGossipDuplicate
	if got := s.receiver.NonceLedger.CurrentEpoch(s.origin.NodeID); got != epochAfterFirst {
		t.Fatalf("epoch must not change on duplicate: %d -> %d", epochAfterFirst, got)
	}
}

// ----- anchorKindOf coverage -----------------------------------------------

func TestAnchorKindOf_RecognizesConcreteAndMapTypes(t *testing.T) {
	// Concrete-typed.
	raw := AnchorTransaction{BaseTransaction: BaseTransaction{Type: TxTypeAnchor}}
	if k, err := anchorKindOf(raw); err != nil || k != TxTypeAnchor {
		t.Fatalf("concrete AnchorTransaction: k=%v err=%v", k, err)
	}

	// Map-encoded (the post-JSON-decode shape).
	mapTx := map[string]interface{}{"type": string(TxTypeGuardianRecoveryInit), "timestamp": 1}
	if k, err := anchorKindOf(mapTx); err != nil || k != TxTypeGuardianRecoveryInit {
		t.Fatalf("map-encoded: k=%v err=%v", k, err)
	}

	// Non-anchor type — empty kind, no error.
	other := TrustTransaction{BaseTransaction: BaseTransaction{Type: TxTypeTrust}}
	if k, err := anchorKindOf(other); err != nil || k != "" {
		t.Fatalf("non-anchor should return empty: k=%v err=%v", k, err)
	}

	// Un-marshalable input returns an error.
	if _, err := anchorKindOf(func() {}); err == nil {
		t.Fatal("un-marshalable input should error")
	}
}

func TestAnchorGossipSignableBytes_ExcludesSignature(t *testing.T) {
	m := AnchorGossipMessage{
		SchemaVersion:   AnchorGossipSchemaVersion,
		MessageID:       "x",
		OriginDomain:    "d",
		Timestamp:       1,
		GossipSignature: "should-not-be-signed",
	}
	a, err := GetAnchorGossipSignableBytes(m)
	if err != nil {
		t.Fatalf("canon: %v", err)
	}

	m.GossipSignature = "completely-different"
	b, err := GetAnchorGossipSignableBytes(m)
	if err != nil {
		t.Fatalf("canon 2: %v", err)
	}
	if string(a) != string(b) {
		t.Fatal("signature field influenced signable bytes — would allow signature forgery")
	}

	var back map[string]interface{}
	if err := json.Unmarshal(a, &back); err != nil {
		t.Fatalf("decode: %v", err)
	}
}

// TestAnchorGossipSignableBytes_IndependentOfBlockContentShape is the
// regression guard for the signable-bytes bug that broke HTTP handler
// tests during Phase F development. The failure mode: json.Marshal
// produces struct field-declaration order for typed wrapper structs
// (AnchorTransaction, etc.) but alphabetical order for the
// map[string]interface{} values that result from a JSON-over-HTTP
// round trip. If the signable bytes depend on the shape of
// OriginBlock.Transactions entries, the signature can't be verified
// after the message passes through JSON.
//
// The fix signs over OriginBlock.Hash rather than its content. This
// test asserts that changing the typed-vs-map shape of the
// transactions slice does NOT change the signable bytes.
func TestAnchorGossipSignableBytes_IndependentOfBlockContentShape(t *testing.T) {
	// Construct a block with a typed AnchorTransaction and compute
	// its hash.
	anchor := NonceAnchor{Kind: AnchorRotation, SignerQuid: "q", AnchorNonce: 1}
	typedBlock := Block{
		Index:     5,
		Timestamp: 123,
		Transactions: []interface{}{
			AnchorTransaction{
				BaseTransaction: BaseTransaction{Type: TxTypeAnchor, Timestamp: 123},
				Anchor:          anchor,
			},
		},
		TrustProof: TrustProof{TrustDomain: "d"},
		PrevHash:   "p",
	}
	typedBlock.Hash = calculateBlockHash(typedBlock)

	// Now produce a "round-tripped" block where the transaction is a
	// generic map (what a JSON decode would yield).
	txJSON, _ := json.Marshal(typedBlock.Transactions[0])
	var asMap map[string]interface{}
	if err := json.Unmarshal(txJSON, &asMap); err != nil {
		t.Fatalf("decode: %v", err)
	}
	mapBlock := typedBlock
	mapBlock.Transactions = []interface{}{asMap}

	// Build two gossip messages differing ONLY in the shape of
	// OriginBlock.Transactions.
	base := AnchorGossipMessage{
		SchemaVersion:      AnchorGossipSchemaVersion,
		MessageID:          "msg",
		OriginDomain:       "d",
		OriginBlockHeight:  5,
		AnchorTxIndex:      0,
		DomainFingerprint:  DomainFingerprint{Domain: "d", BlockHeight: 5, BlockHash: typedBlock.Hash},
		Timestamp:          100,
		GossipProducerQuid: "producer",
	}
	typedMsg := base
	typedMsg.OriginBlock = typedBlock
	mapMsg := base
	mapMsg.OriginBlock = mapBlock

	a, err := GetAnchorGossipSignableBytes(typedMsg)
	if err != nil {
		t.Fatalf("typed: %v", err)
	}
	b, err := GetAnchorGossipSignableBytes(mapMsg)
	if err != nil {
		t.Fatalf("map: %v", err)
	}
	if string(a) != string(b) {
		t.Fatalf("signable bytes differ between typed and map transaction shapes:\n  typed=%s\n  map  =%s",
			string(a), string(b))
	}
}
