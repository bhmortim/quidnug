package core

import (
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"time"
)

// AnchorGossipMessage carries a single anchor from the domain where
// it was sealed into a block, to any other domain a participating
// signer also operates in. QDP-0003 §6.4: "anchor gossip is rare
// (humans rotate keys infrequently). It does not scale with
// transaction volume."
//
// Every gossip bundle is self-verifying: the full origin block is
// included alongside an inline DomainFingerprint that proves the
// block was sealed in the origin domain. This costs more bandwidth
// than a Merkle path but requires no block-structure changes and
// matches QDP-0003's pragmatic trade-off for the foundation phase.
// Future optimization (QDP-0003 §15.4) can switch to compact
// inclusion proofs once blocks carry a transaction-tree root.
type AnchorGossipMessage struct {
	SchemaVersion int `json:"schemaVersion"`

	// MessageID is a stable identifier for deduplication. Any
	// deterministic-over-contents id is fine; producers typically use
	// sha256(OriginDomain || OriginBlockHash || AnchorTxIndex).
	MessageID string `json:"messageId"`

	// The block, the position of the anchor inside its Transactions
	// slice, and the domain that sealed it.
	OriginDomain      string `json:"originDomain"`
	OriginBlockHeight int64  `json:"originBlockHeight"`
	OriginBlock       Block  `json:"originBlock"`
	AnchorTxIndex     int    `json:"anchorTxIndex"`

	// Inline DomainFingerprint proving the origin block was sealed in
	// origin domain. The receiver verifies this before trusting the
	// block.
	DomainFingerprint DomainFingerprint `json:"domainFingerprint"`

	// The gossip message itself is signed so a receiver can rate-limit
	// / blame specific producers. This is distinct from the block's
	// own signatures (which validate the block) and the fingerprint's
	// signature (which validates the block-hash claim).
	Timestamp          int64  `json:"timestamp"`
	GossipProducerQuid string `json:"gossipProducerQuid"`
	GossipSignature    string `json:"gossipSignature"`
}

const AnchorGossipSchemaVersion = 1

// Errors from AnchorGossipMessage handling.
var (
	ErrGossipBadSchema           = errors.New("anchor-gossip: unknown schemaVersion")
	ErrGossipMissingMessageID    = errors.New("anchor-gossip: missing messageId")
	ErrGossipMissingProducer     = errors.New("anchor-gossip: missing gossipProducerQuid")
	ErrGossipStaleTimestamp      = errors.New("anchor-gossip: timestamp outside accepted window")
	ErrGossipNoProducerKey       = errors.New("anchor-gossip: no public key for gossipProducerQuid")
	ErrGossipBadGossipSig        = errors.New("anchor-gossip: gossip signature verification failed")
	ErrGossipBadBlockHash        = errors.New("anchor-gossip: originBlock.Hash does not match calculateBlockHash(originBlock)")
	ErrGossipBlockDomainMismatch = errors.New("anchor-gossip: originBlock.TrustProof.TrustDomain does not match originDomain")
	ErrGossipFingerprintMismatch = errors.New("anchor-gossip: fingerprint does not cover the claimed origin block")
	ErrGossipTxIndexOutOfRange   = errors.New("anchor-gossip: anchorTxIndex is out of bounds for originBlock.Transactions")
	ErrGossipTxNotAnchor         = errors.New("anchor-gossip: tx at anchorTxIndex is not an anchor kind")
	ErrGossipDuplicate           = errors.New("anchor-gossip: message has already been processed (duplicate messageId)")
)

// AnchorGossipMaxAge bounds how old the gossip producer's signature
// can be. Separate from AnchorMaxAge because a gossip message is
// "live" — unlike an anchor whose ValidFrom is a protocol-level
// timestamp, the gossip's Timestamp is about when the bundle was
// transmitted.
const AnchorGossipMaxAge = 24 * time.Hour

// GetAnchorGossipSignableBytes is the canonical form the producer
// signs. GossipSignature is cleared.
//
// NB: we sign over OriginBlock.Hash rather than the full OriginBlock
// value. The block's Transactions slice is typed as []interface{},
// so Go's json.Marshal produces declaration-order output for typed
// wrapper structs (AnchorTransaction, GuardianSetUpdateTransaction,
// ...) but alphabetical order for the map[string]interface{} values
// that result from a JSON-over-HTTP round trip. Signing the content
// directly would make signatures non-verifiable on the receiving
// side. Signing the hash sidesteps this entirely: block integrity
// is verified separately via calculateBlockHash(OriginBlock), and
// the gossip signature binds a specific block-hash claim.
func GetAnchorGossipSignableBytes(m AnchorGossipMessage) ([]byte, error) {
	return json.Marshal(struct {
		SchemaVersion      int               `json:"schemaVersion"`
		MessageID          string            `json:"messageId"`
		OriginDomain       string            `json:"originDomain"`
		OriginBlockHeight  int64             `json:"originBlockHeight"`
		OriginBlockHash    string            `json:"originBlockHash"`
		AnchorTxIndex      int               `json:"anchorTxIndex"`
		DomainFingerprint  DomainFingerprint `json:"domainFingerprint"`
		Timestamp          int64             `json:"timestamp"`
		GossipProducerQuid string            `json:"gossipProducerQuid"`
	}{
		SchemaVersion:      m.SchemaVersion,
		MessageID:          m.MessageID,
		OriginDomain:       m.OriginDomain,
		OriginBlockHeight:  m.OriginBlockHeight,
		OriginBlockHash:    m.OriginBlock.Hash,
		AnchorTxIndex:      m.AnchorTxIndex,
		DomainFingerprint:  m.DomainFingerprint,
		Timestamp:          m.Timestamp,
		GossipProducerQuid: m.GossipProducerQuid,
	})
}

// ValidateAnchorGossip runs the full cross-domain verification chain
// (QDP-0003 §6.4). Successful return means the message is safe to
// apply; a failing return carries the reason for metrics and logs.
func ValidateAnchorGossip(l *NonceLedger, m AnchorGossipMessage, now time.Time) error {
	if m.SchemaVersion != AnchorGossipSchemaVersion {
		return fmt.Errorf("%w: %d", ErrGossipBadSchema, m.SchemaVersion)
	}
	if m.MessageID == "" {
		return ErrGossipMissingMessageID
	}
	if m.GossipProducerQuid == "" {
		return ErrGossipMissingProducer
	}
	if m.Timestamp <= 0 {
		return ErrGossipStaleTimestamp
	}
	ts := time.Unix(m.Timestamp, 0)
	if now.Sub(ts) > AnchorGossipMaxAge {
		return ErrGossipStaleTimestamp
	}
	if ts.Sub(now) > AnchorMaxFutureSkew {
		return ErrGossipStaleTimestamp
	}

	if l == nil {
		return ErrGossipNoProducerKey
	}

	// Dedup FIRST. It's a cheap map lookup and avoids expensive ECDSA
	// work on replays. Equally important: if this gossip carries a
	// self-rotation for the producer, applying it advances the
	// producer's currentEpoch — which would cause a naive re-
	// validation of the same bytes to fail against the new key.
	// Rejecting duplicates up front sidesteps that entirely.
	if l.seenGossip(m.MessageID) {
		return ErrGossipDuplicate
	}

	// 1. Gossip producer signature.
	producerEpoch := l.CurrentEpoch(m.GossipProducerQuid)
	producerKey, ok := l.GetSignerKey(m.GossipProducerQuid, producerEpoch)
	if !ok {
		return ErrGossipNoProducerKey
	}
	signable, err := GetAnchorGossipSignableBytes(m)
	if err != nil {
		return fmt.Errorf("anchor-gossip: canonicalization: %w", err)
	}
	if _, err := hex.DecodeString(m.GossipSignature); err != nil {
		return fmt.Errorf("%w: %v", ErrGossipBadGossipSig, err)
	}
	if !VerifySignature(producerKey, signable, m.GossipSignature) {
		return ErrGossipBadGossipSig
	}

	// 2. Inline fingerprint: cryptographically valid and covers this
	//    origin block.
	if err := VerifyDomainFingerprint(l, m.DomainFingerprint, now); err != nil {
		return fmt.Errorf("anchor-gossip: fingerprint: %w", err)
	}
	if m.DomainFingerprint.Domain != m.OriginDomain {
		return ErrGossipFingerprintMismatch
	}
	if m.DomainFingerprint.BlockHeight != m.OriginBlockHeight {
		return ErrGossipFingerprintMismatch
	}
	if m.DomainFingerprint.BlockHash != m.OriginBlock.Hash {
		return ErrGossipFingerprintMismatch
	}

	// 3. Origin block structure: self-consistent.
	recomputedHash := calculateBlockHash(m.OriginBlock)
	if recomputedHash != m.OriginBlock.Hash {
		return ErrGossipBadBlockHash
	}
	if m.OriginBlock.TrustProof.TrustDomain != m.OriginDomain {
		return ErrGossipBlockDomainMismatch
	}
	if m.OriginBlock.Index != m.OriginBlockHeight {
		return ErrGossipBadBlockHash
	}

	// 4. Anchor index is valid and points to an anchor-kind tx.
	if m.AnchorTxIndex < 0 || m.AnchorTxIndex >= len(m.OriginBlock.Transactions) {
		return ErrGossipTxIndexOutOfRange
	}
	kind, err := anchorKindOf(m.OriginBlock.Transactions[m.AnchorTxIndex])
	if err != nil {
		return err
	}
	if kind == "" {
		return ErrGossipTxNotAnchor
	}

	return nil
}

// anchorKindOf returns the TransactionType string if the given
// interface value is one of the anchor transaction wrappers
// (NonceAnchor variants + guardian kinds, including H6 resign).
// Returns "" + nil if it's not an anchor, or an error if
// unmarshaling fails.
//
// We accept both the concrete typed structs (the common case from
// processBlockTransactions) and the generic interface{} path used
// when a block has been serialized-and-deserialized at a cross-domain
// hop (JSON unmarshal produces map[string]interface{} for interface{}
// fields, so we need a tolerant decoder).
func anchorKindOf(rawTx interface{}) (TransactionType, error) {
	// Concrete-typed transaction — fastest path.
	switch tx := rawTx.(type) {
	case AnchorTransaction:
		return tx.Type, nil
	case GuardianSetUpdateTransaction:
		return tx.Type, nil
	case GuardianRecoveryInitTransaction:
		return tx.Type, nil
	case GuardianRecoveryVetoTransaction:
		return tx.Type, nil
	case GuardianRecoveryCommitTransaction:
		return tx.Type, nil
	case GuardianResignationTransaction:
		return tx.Type, nil
	}

	// Interface path: the tx arrived as a generic map (JSON decode
	// into []interface{}). Marshal then inspect the Type tag.
	b, err := json.Marshal(rawTx)
	if err != nil {
		return "", err
	}
	var probe struct {
		Type TransactionType `json:"type"`
	}
	if err := json.Unmarshal(b, &probe); err != nil {
		return "", err
	}
	switch probe.Type {
	case TxTypeAnchor,
		TxTypeGuardianSetUpdate,
		TxTypeGuardianRecoveryInit,
		TxTypeGuardianRecoveryVeto,
		TxTypeGuardianRecoveryCommit,
		TxTypeGuardianResign:
		return probe.Type, nil
	}
	return "", nil
}

// ApplyAnchorGossip re-validates the message and, on success,
// dispatches the referenced transaction through the same pathway
// processBlockTransactions uses for in-domain anchors. Idempotent:
// duplicate messageIDs are a no-op.
func (node *QuidnugNode) ApplyAnchorGossip(m AnchorGossipMessage) error {
	if node.NonceLedger == nil {
		return errors.New("anchor-gossip: nonce ledger not initialized")
	}
	if err := ValidateAnchorGossip(node.NonceLedger, m, time.Now()); err != nil {
		return err
	}

	// Store the inline fingerprint so future gossip referencing the
	// same (or older) block can also verify. StoreDomainFingerprint is
	// monotonic on block height.
	node.NonceLedger.StoreDomainFingerprint(m.DomainFingerprint)

	// Dispatch the single tx through the per-kind apply path. We
	// re-marshal then unmarshal to handle the interface{}-map case
	// cleanly.
	raw := m.OriginBlock.Transactions[m.AnchorTxIndex]
	txJSON, err := json.Marshal(raw)
	if err != nil {
		return fmt.Errorf("anchor-gossip: re-marshal: %w", err)
	}
	var base BaseTransaction
	if err := json.Unmarshal(txJSON, &base); err != nil {
		return fmt.Errorf("anchor-gossip: base decode: %w", err)
	}

	switch base.Type {
	case TxTypeAnchor:
		var tx AnchorTransaction
		if err := json.Unmarshal(txJSON, &tx); err != nil {
			return fmt.Errorf("anchor-gossip: AnchorTransaction decode: %w", err)
		}
		node.applyAnchorFromBlock(tx.Anchor, m.OriginBlock)

	case TxTypeGuardianSetUpdate:
		var tx GuardianSetUpdateTransaction
		if err := json.Unmarshal(txJSON, &tx); err != nil {
			return fmt.Errorf("anchor-gossip: GuardianSetUpdateTransaction decode: %w", err)
		}
		node.applyGuardianSetUpdate(tx.Update, m.OriginBlock)

	case TxTypeGuardianRecoveryInit:
		var tx GuardianRecoveryInitTransaction
		if err := json.Unmarshal(txJSON, &tx); err != nil {
			return fmt.Errorf("anchor-gossip: GuardianRecoveryInitTransaction decode: %w", err)
		}
		node.applyGuardianRecoveryInit(tx.Init, m.OriginBlock)

	case TxTypeGuardianRecoveryVeto:
		var tx GuardianRecoveryVetoTransaction
		if err := json.Unmarshal(txJSON, &tx); err != nil {
			return fmt.Errorf("anchor-gossip: GuardianRecoveryVetoTransaction decode: %w", err)
		}
		node.applyGuardianRecoveryVeto(tx.Veto, m.OriginBlock)

	case TxTypeGuardianRecoveryCommit:
		var tx GuardianRecoveryCommitTransaction
		if err := json.Unmarshal(txJSON, &tx); err != nil {
			return fmt.Errorf("anchor-gossip: GuardianRecoveryCommitTransaction decode: %w", err)
		}
		node.applyGuardianRecoveryCommit(tx.Commit, m.OriginBlock)

	case TxTypeGuardianResign:
		var tx GuardianResignationTransaction
		if err := json.Unmarshal(txJSON, &tx); err != nil {
			return fmt.Errorf("anchor-gossip: GuardianResignationTransaction decode: %w", err)
		}
		node.applyGuardianResignation(tx.Resignation, m.OriginBlock)

	default:
		return ErrGossipTxNotAnchor
	}

	node.NonceLedger.markSeenGossip(m.MessageID, time.Now())
	// QDP-0007 (H4): push / pull gossip counts as recency evidence
	// for both the gossip producer and the anchor's subject.
	node.NonceLedger.MarkEpochRefresh(m.GossipProducerQuid, time.Now())
	if subject := anchorSubjectQuid(m); subject != "" {
		node.NonceLedger.MarkEpochRefresh(subject, time.Now())
		if node.quarantine != nil {
			node.releaseQuarantinedForSigner(subject, "gossip")
		}
	}
	logger.Info("Applied cross-domain anchor gossip",
		"messageId", m.MessageID,
		"originDomain", m.OriginDomain,
		"originBlockHeight", m.OriginBlockHeight,
		"kind", string(base.Type))
	return nil
}

// SignAnchorGossip signs a gossip message. Convenience method;
// producers may construct messages wherever and sign at the edge.
func (node *QuidnugNode) SignAnchorGossip(m AnchorGossipMessage) (AnchorGossipMessage, error) {
	signable, err := GetAnchorGossipSignableBytes(m)
	if err != nil {
		return m, fmt.Errorf("anchor-gossip: canonicalization: %w", err)
	}
	sig, err := node.SignData(signable)
	if err != nil {
		return m, fmt.Errorf("anchor-gossip: signing: %w", err)
	}
	m.GossipSignature = hex.EncodeToString(sig)
	return m, nil
}
