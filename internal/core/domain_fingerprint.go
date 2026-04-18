package core

import (
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"time"
)

// DomainFingerprint is a signed claim by a domain validator that a
// specific block in that domain has been sealed with a given hash and
// height. Fingerprints are the trust anchor for cross-domain anchor
// gossip (QDP-0003 §7.3) — a receiver in domain B that has a trusted
// fingerprint for domain A can verify that an anchor allegedly from
// block H of domain A really belongs to that block, and that the
// block is the one A has committed (not a forged one).
type DomainFingerprint struct {
	SchemaVersion int    `json:"schemaVersion"`
	Domain        string `json:"domain"`
	BlockHeight   int64  `json:"blockHeight"`
	BlockHash     string `json:"blockHash"`
	Timestamp     int64  `json:"timestamp"`
	ProducerQuid  string `json:"producerQuid"`
	Signature     string `json:"signature"`
}

const DomainFingerprintSchemaVersion = 1

// DomainFingerprintRetention bounds how old a stored fingerprint can
// be before it's considered stale for gossip verification. Matches
// the 14-day proposal in QDP-0003 §15.3.
const DomainFingerprintRetention = 14 * 24 * time.Hour

// Errors from fingerprint handling.
var (
	ErrFingerprintBadSchema    = errors.New("domain-fingerprint: unknown schemaVersion")
	ErrFingerprintMissingProd  = errors.New("domain-fingerprint: missing producerQuid")
	ErrFingerprintBadSignature = errors.New("domain-fingerprint: signature verification failed")
	ErrFingerprintNoProdKey    = errors.New("domain-fingerprint: no public key recorded for producer quid")
	ErrFingerprintStale        = errors.New("domain-fingerprint: older than retention window")
)

// GetDomainFingerprintSignableBytes returns canonical bytes for
// signing / verifying a fingerprint. Same "clear signature, marshal"
// pattern used elsewhere in the codebase.
func GetDomainFingerprintSignableBytes(fp DomainFingerprint) ([]byte, error) {
	fp.Signature = ""
	return json.Marshal(fp)
}

// ProduceDomainFingerprint builds an unsigned fingerprint for the
// node's current head of the named domain. Caller signs via
// SignDomainFingerprint.
//
// Returns an error if the domain has no blocks on this node — you
// can't attest to a chain you don't have.
func (node *QuidnugNode) ProduceDomainFingerprint(domain string, now time.Time) (DomainFingerprint, error) {
	// Scan the main blockchain for the most recent block in `domain`.
	// Callers typically produce fingerprints at block-acceptance time,
	// so the scan cost is acceptable here. If this becomes a hot path
	// we can cache the per-domain head in the ledger.
	node.BlockchainMutex.RLock()
	defer node.BlockchainMutex.RUnlock()

	var head *Block
	for i := len(node.Blockchain) - 1; i >= 0; i-- {
		if node.Blockchain[i].TrustProof.TrustDomain == domain {
			head = &node.Blockchain[i]
			break
		}
	}
	if head == nil {
		return DomainFingerprint{}, fmt.Errorf("domain-fingerprint: no blocks for domain %q", domain)
	}

	return DomainFingerprint{
		SchemaVersion: DomainFingerprintSchemaVersion,
		Domain:        domain,
		BlockHeight:   head.Index,
		BlockHash:     head.Hash,
		Timestamp:     now.Unix(),
		ProducerQuid:  node.NodeID,
	}, nil
}

// SignDomainFingerprint signs the fingerprint with the node's private
// key. The node is responsible for ensuring its NodeID matches
// fp.ProducerQuid.
func (node *QuidnugNode) SignDomainFingerprint(fp DomainFingerprint) (DomainFingerprint, error) {
	data, err := GetDomainFingerprintSignableBytes(fp)
	if err != nil {
		return fp, fmt.Errorf("domain-fingerprint: canonicalization: %w", err)
	}
	sig, err := node.SignData(data)
	if err != nil {
		return fp, fmt.Errorf("domain-fingerprint: signing: %w", err)
	}
	fp.Signature = hex.EncodeToString(sig)
	return fp, nil
}

// VerifyDomainFingerprint validates a fingerprint's signature against
// the ledger's stored public key for the producer's current epoch.
// Does not judge whether the fingerprint's content is "right" — a
// caller wanting "is this block hash consistent with what I already
// know?" needs to cross-reference against their own state.
func VerifyDomainFingerprint(l *NonceLedger, fp DomainFingerprint, now time.Time) error {
	if fp.SchemaVersion != DomainFingerprintSchemaVersion {
		return fmt.Errorf("%w: %d", ErrFingerprintBadSchema, fp.SchemaVersion)
	}
	if fp.ProducerQuid == "" {
		return ErrFingerprintMissingProd
	}
	if fp.Timestamp > 0 && time.Since(time.Unix(fp.Timestamp, 0)) > DomainFingerprintRetention {
		return ErrFingerprintStale
	}
	if l == nil {
		return ErrFingerprintNoProdKey
	}

	epoch := l.CurrentEpoch(fp.ProducerQuid)
	producerKey, ok := l.GetSignerKey(fp.ProducerQuid, epoch)
	if !ok || producerKey == "" {
		return ErrFingerprintNoProdKey
	}

	signable, err := GetDomainFingerprintSignableBytes(fp)
	if err != nil {
		return fmt.Errorf("domain-fingerprint: canonicalization: %w", err)
	}
	if _, err := hex.DecodeString(fp.Signature); err != nil {
		return fmt.Errorf("%w: %v", ErrFingerprintBadSignature, err)
	}
	if !VerifySignature(producerKey, signable, fp.Signature) {
		return ErrFingerprintBadSignature
	}
	return nil
}

// ----- Ledger accessors for fingerprints ----------------------------------

// StoreDomainFingerprint records a fingerprint as the ledger's latest
// known for its domain. Monotonic on BlockHeight: an incoming
// fingerprint below the stored height is silently ignored (the
// caller pre-validates signatures; this is just a store).
func (l *NonceLedger) StoreDomainFingerprint(fp DomainFingerprint) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.latestFingerprints == nil {
		l.latestFingerprints = make(map[string]DomainFingerprint)
	}
	cur, ok := l.latestFingerprints[fp.Domain]
	if ok && cur.BlockHeight >= fp.BlockHeight {
		return
	}
	l.latestFingerprints[fp.Domain] = fp
}

// GetDomainFingerprint returns the latest known fingerprint for the
// domain, or the zero value + false if none is stored.
func (l *NonceLedger) GetDomainFingerprint(domain string) (DomainFingerprint, bool) {
	l.mu.RLock()
	defer l.mu.RUnlock()
	fp, ok := l.latestFingerprints[domain]
	return fp, ok
}

// ----- Gossip dedup accessors ---------------------------------------------

// seenGossip reports whether a message ID has already been observed.
// Mutating version (markSeenGossip) used by the apply path.
func (l *NonceLedger) seenGossip(messageID string) bool {
	l.mu.RLock()
	defer l.mu.RUnlock()
	_, ok := l.seenGossipMessages[messageID]
	return ok
}

// markSeenGossip records a messageID as observed, with opportunistic
// pruning of stale entries. Called exactly once per accepted gossip
// message.
func (l *NonceLedger) markSeenGossip(messageID string, at time.Time) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.seenGossipMessages == nil {
		l.seenGossipMessages = make(map[string]int64)
	}
	cutoff := at.Add(-DomainFingerprintRetention).Unix()
	for id, ts := range l.seenGossipMessages {
		if ts < cutoff {
			delete(l.seenGossipMessages, id)
		}
	}
	l.seenGossipMessages[messageID] = at.Unix()
}
