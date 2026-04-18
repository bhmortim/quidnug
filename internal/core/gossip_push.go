// Package core — gossip_push.go
//
// Push-based gossip for cross-domain anchors and fingerprints
// (QDP-0005 / Phase H1). Extends the existing pull-based flow
// (POST /api/v2/anchor-gossip + GET /api/v2/domain-fingerprints)
// with fire-and-forget HTTP POSTs that propagate through the
// network over a bounded number of hops.
//
// Invariants enforced here:
//
//   - Dedup runs BEFORE signature verification. Duplicate floods
//     are cheap to reject.
//   - Subscription check runs before validation. A node that
//     has no state about a signer does not pay ECDSA cost to
//     validate a message it will then ignore.
//   - TTL is clamped on receipt, not trusted from the sender.
//     Prevents amplification via a forged-TTL attack.
//   - Envelope fields (TTL, HopCount, ForwardedBy) are NOT
//     covered by the producer's signature — mutating them per
//     hop is safe; mutating the payload breaks the signature.
//
// The fan-out path is producer-triggered: on a fresh anchor
// or fingerprint, the node fans out via PushAnchor /
// PushFingerprint. Receivers continue the fan-out based on
// remaining TTL.
package core

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"time"
)

// ---------------------------------------------------------------------------
// Wire types
// ---------------------------------------------------------------------------

// GossipPushSchemaVersion is the envelope schema version. Bumped
// only on breaking changes to the envelope shape — not the
// wrapped payload, which has its own version field.
const GossipPushSchemaVersion = 1

// AnchorPushMessage wraps an AnchorGossipMessage with routing
// metadata. The producer signs only the Payload (via
// GetAnchorGossipSignableBytes from the QDP-0003 flow); envelope
// fields are mutable per hop.
type AnchorPushMessage struct {
	SchemaVersion int                 `json:"schemaVersion"`
	Payload       AnchorGossipMessage `json:"payload"`

	// TTL is the remaining hop budget. Clamped on receipt to
	// [0, DomainGossipTTL] so a forged large value can't amplify.
	TTL int `json:"ttl"`

	// HopCount is the number of hops this message has already
	// traversed. Advisory for metrics; never used for validation.
	HopCount int `json:"hopCount"`

	// ForwardedBy is the immediate sender's node ID. Used to
	// avoid forwarding back to the sender; not covered by the
	// signature, so it's advisory.
	ForwardedBy string `json:"forwardedBy"`
}

// FingerprintPushMessage wraps a DomainFingerprint with routing
// metadata. Producer signs only the Payload (via
// GetDomainFingerprintSignableBytes); envelope fields are mutable.
type FingerprintPushMessage struct {
	SchemaVersion int               `json:"schemaVersion"`
	Payload       DomainFingerprint `json:"payload"`
	TTL           int               `json:"ttl"`
	HopCount      int               `json:"hopCount"`
	ForwardedBy   string            `json:"forwardedBy"`
}

// ---------------------------------------------------------------------------
// Drop reason constants (used by metrics + logs)
// ---------------------------------------------------------------------------

const (
	DropReasonSchema         = "schema"
	DropReasonTTL            = "ttl"
	DropReasonUnknownProd    = "unknown_producer"
	DropReasonSignature      = "sig"
	DropReasonMonotonicity   = "monotonicity"
	DropReasonDuplicate      = "dup"
	DropReasonRateLimit      = "rate_limit"
	DropReasonNotSubscribed  = "not_subscribed"
	DropReasonApply          = "apply"
	DropReasonInvalidPayload = "invalid_payload"
)

// ---------------------------------------------------------------------------
// Error values
// ---------------------------------------------------------------------------

var (
	ErrPushBadSchema      = errors.New("gossip-push: unknown envelope schemaVersion")
	ErrPushTTLExpired     = errors.New("gossip-push: TTL already zero on receipt (should not forward)")
	ErrPushNotSubscribed  = errors.New("gossip-push: receiver not subscribed to producer/domain")
	ErrPushRateLimited    = errors.New("gossip-push: producer rate-limited")
	ErrPushEmptyPayload   = errors.New("gossip-push: empty payload")
	ErrPushEnvelopeTooBig = errors.New("gossip-push: envelope exceeds size cap")
)

// GossipPushMaxEnvelopeBytes caps the encoded envelope size to
// guard against a memory-exhaustion attack via an enormous
// forged payload. A typical AnchorGossipMessage is under 8 KB
// (one block); we give 10x headroom.
const GossipPushMaxEnvelopeBytes = 128 * 1024

// ---------------------------------------------------------------------------
// Subscription check
// ---------------------------------------------------------------------------

// isSubscribedToAnchor reports whether this node has any state
// about the anchor's gossip producer. If signerKeys[producer] is
// empty, we cannot validate the signature anyway, so there's no
// point forwarding — this serves both as a performance filter
// and as an anti-amplification defense.
//
// Under QDP-0005 §6.3 this is the implicit subscription signal:
// "the receiver's ledger contains signerKeys[producerQuid]".
func (node *QuidnugNode) isSubscribedToAnchor(m AnchorGossipMessage) bool {
	if node.NonceLedger == nil {
		return false
	}
	// We care about the gossip producer (whoever pushed the
	// message) AND the subject of the payload (whose state we
	// may hold). Either match is enough.
	if hasAnySignerKey(node.NonceLedger, m.GossipProducerQuid) {
		return true
	}
	// The anchor's tx quid (the signer whose state changes)
	// also counts — we need to know we care about updates to
	// that signer.
	if q := anchorSubjectQuid(m); q != "" && hasAnySignerKey(node.NonceLedger, q) {
		return true
	}
	return false
}

// isSubscribedToFingerprint reports whether this node is
// interested in a fingerprint for the given domain. True if we
// have any block for that domain OR have a previously-stored
// fingerprint for it.
func (node *QuidnugNode) isSubscribedToFingerprint(fp DomainFingerprint) bool {
	if node.NonceLedger == nil {
		return false
	}
	if _, ok := node.NonceLedger.GetDomainFingerprint(fp.Domain); ok {
		return true
	}
	node.BlockchainMutex.RLock()
	defer node.BlockchainMutex.RUnlock()
	for i := range node.Blockchain {
		if node.Blockchain[i].TrustProof.TrustDomain == fp.Domain {
			return true
		}
	}
	return false
}

// hasAnySignerKey returns true iff the ledger has at least one
// epoch key recorded for the quid.
func hasAnySignerKey(l *NonceLedger, quid string) bool {
	if quid == "" {
		return false
	}
	l.mu.RLock()
	defer l.mu.RUnlock()
	return len(l.signerKeys[quid]) > 0
}

// anchorSubjectQuid extracts the subject quid from the anchor
// transaction inside the payload's OriginBlock. Returns ""
// if the index is out of range or the tx doesn't have a quid
// field we recognize. Best-effort — validation proper happens
// later.
func anchorSubjectQuid(m AnchorGossipMessage) string {
	if m.AnchorTxIndex < 0 || m.AnchorTxIndex >= len(m.OriginBlock.Transactions) {
		return ""
	}
	raw := m.OriginBlock.Transactions[m.AnchorTxIndex]
	// Fast path: typed AnchorTransaction
	if tx, ok := raw.(AnchorTransaction); ok {
		return tx.Anchor.SignerQuid
	}
	if tx, ok := raw.(GuardianSetUpdateTransaction); ok {
		return tx.Update.SubjectQuid
	}
	if tx, ok := raw.(GuardianRecoveryInitTransaction); ok {
		return tx.Init.SubjectQuid
	}
	if tx, ok := raw.(GuardianResignationTransaction); ok {
		return tx.Resignation.SubjectQuid
	}
	// Slow path: decode via JSON probe.
	b, err := json.Marshal(raw)
	if err != nil {
		return ""
	}
	var probe struct {
		Anchor struct {
			SignerQuid string `json:"signerQuid"`
		} `json:"anchor"`
		Update struct {
			SubjectQuid string `json:"subjectQuid"`
		} `json:"update"`
		Init struct {
			SubjectQuid string `json:"subjectQuid"`
		} `json:"init"`
		Resignation struct {
			SubjectQuid string `json:"subjectQuid"`
		} `json:"resignation"`
	}
	if err := json.Unmarshal(b, &probe); err != nil {
		return ""
	}
	switch {
	case probe.Anchor.SignerQuid != "":
		return probe.Anchor.SignerQuid
	case probe.Update.SubjectQuid != "":
		return probe.Update.SubjectQuid
	case probe.Init.SubjectQuid != "":
		return probe.Init.SubjectQuid
	case probe.Resignation.SubjectQuid != "":
		return probe.Resignation.SubjectQuid
	}
	return ""
}

// ---------------------------------------------------------------------------
// Receive path
// ---------------------------------------------------------------------------

// ReceiveAnchorPush handles an incoming AnchorPushMessage. The
// ordering is dedup → subscription → validate → apply → forward,
// exactly as specified in QDP-0005 §6.4.
//
// Returns (duplicate, error). duplicate=true means the message
// was already seen (idempotent 200). error!=nil means the
// message was rejected and should NOT be forwarded.
func (node *QuidnugNode) ReceiveAnchorPush(m AnchorPushMessage) (bool, error) {
	// 1. Schema.
	if m.SchemaVersion != GossipPushSchemaVersion {
		recordGossipDrop("anchor", DropReasonSchema)
		return false, fmt.Errorf("%w: %d", ErrPushBadSchema, m.SchemaVersion)
	}

	// 2. Clamp TTL to the node's local cap.
	if m.TTL < 0 {
		m.TTL = 0
	}
	if cap := node.GossipTTL; cap > 0 && m.TTL > cap {
		m.TTL = cap
	}

	// 3. Dedup FIRST. Cheap map lookup; kills floods.
	if node.NonceLedger != nil && node.NonceLedger.seenGossip(m.Payload.MessageID) {
		recordGossipDrop("anchor", DropReasonDuplicate)
		return true, nil
	}

	// 4. Subscription filter. Unknown producer / subject → drop.
	if !node.isSubscribedToAnchor(m.Payload) {
		recordGossipDrop("anchor", DropReasonNotSubscribed)
		return false, ErrPushNotSubscribed
	}

	// 5. Full QDP-0003 validation chain.
	if err := ValidateAnchorGossip(node.NonceLedger, m.Payload, time.Now()); err != nil {
		reason := DropReasonInvalidPayload
		switch {
		case errors.Is(err, ErrGossipNoProducerKey):
			reason = DropReasonUnknownProd
		case errors.Is(err, ErrGossipBadGossipSig):
			reason = DropReasonSignature
		case errors.Is(err, ErrGossipDuplicate):
			// Race: dedup saw it between our check and validate.
			recordGossipDrop("anchor", DropReasonDuplicate)
			return true, nil
		}
		recordGossipDrop("anchor", reason)
		return false, err
	}

	// 6. Rate limit BEFORE apply. If we're over the cap we still
	//    apply (the payload is genuinely new and valid), but we
	//    stop forwarding. That way the receiver gets the truth
	//    while forwarding is choked.
	producer := m.Payload.GossipProducerQuid
	forward := node.gossipRateAllow(producer)

	// 7. Apply.
	if err := node.ApplyAnchorGossip(m.Payload); err != nil {
		// Should not happen after successful Validate, but guard.
		recordGossipDrop("anchor", DropReasonApply)
		return false, err
	}
	recordGossipApplied("anchor")
	observeGossipLatency("anchor", m.Payload.Timestamp)

	// 8. Forward if TTL allows and rate-limit permits.
	if !forward {
		recordGossipRateLimited(producer)
		return false, nil
	}
	if m.TTL > 0 {
		forward := AnchorPushMessage{
			SchemaVersion: GossipPushSchemaVersion,
			Payload:       m.Payload,
			TTL:           m.TTL - 1,
			HopCount:      m.HopCount + 1,
			ForwardedBy:   node.NodeID,
		}
		node.fanOutAnchorPush(forward, m.ForwardedBy)
	}
	return false, nil
}

// ReceiveFingerprintPush handles an incoming FingerprintPushMessage.
// Same ordering as ReceiveAnchorPush.
func (node *QuidnugNode) ReceiveFingerprintPush(m FingerprintPushMessage) (bool, error) {
	if m.SchemaVersion != GossipPushSchemaVersion {
		recordGossipDrop("fingerprint", DropReasonSchema)
		return false, fmt.Errorf("%w: %d", ErrPushBadSchema, m.SchemaVersion)
	}
	if m.TTL < 0 {
		m.TTL = 0
	}
	if cap := node.GossipTTL; cap > 0 && m.TTL > cap {
		m.TTL = cap
	}

	// Fingerprint dedup: key is (domain, blockHeight, blockHash,
	// producer) — a re-signed fingerprint of the same block by
	// the same producer is a duplicate.
	fpID := fingerprintDedupID(m.Payload)
	if node.NonceLedger != nil && node.NonceLedger.seenGossip(fpID) {
		recordGossipDrop("fingerprint", DropReasonDuplicate)
		return true, nil
	}

	// Monotonicity short-circuit: if we already have a newer
	// fingerprint for this domain, drop without validating
	// (signature verify is more expensive than a map lookup).
	if node.NonceLedger != nil {
		if cur, ok := node.NonceLedger.GetDomainFingerprint(m.Payload.Domain); ok {
			if cur.BlockHeight >= m.Payload.BlockHeight {
				recordGossipDrop("fingerprint", DropReasonMonotonicity)
				return true, nil
			}
		}
	}

	if !node.isSubscribedToFingerprint(m.Payload) {
		recordGossipDrop("fingerprint", DropReasonNotSubscribed)
		return false, ErrPushNotSubscribed
	}

	if err := VerifyDomainFingerprint(node.NonceLedger, m.Payload, time.Now()); err != nil {
		reason := DropReasonInvalidPayload
		switch {
		case errors.Is(err, ErrFingerprintNoProdKey):
			reason = DropReasonUnknownProd
		case errors.Is(err, ErrFingerprintBadSignature):
			reason = DropReasonSignature
		}
		recordGossipDrop("fingerprint", reason)
		return false, err
	}

	producer := m.Payload.ProducerQuid
	forward := node.gossipRateAllow(producer)

	node.NonceLedger.StoreDomainFingerprint(m.Payload)
	node.NonceLedger.markSeenGossip(fpID, time.Now())
	recordGossipApplied("fingerprint")
	observeGossipLatency("fingerprint", m.Payload.Timestamp)

	if !forward {
		recordGossipRateLimited(producer)
		return false, nil
	}
	if m.TTL > 0 {
		forward := FingerprintPushMessage{
			SchemaVersion: GossipPushSchemaVersion,
			Payload:       m.Payload,
			TTL:           m.TTL - 1,
			HopCount:      m.HopCount + 1,
			ForwardedBy:   node.NodeID,
		}
		node.fanOutFingerprintPush(forward, m.ForwardedBy)
	}
	return false, nil
}

// fingerprintDedupID is the stable key we use for fingerprint
// push dedup. The fingerprint itself has no MessageID, so we
// synthesize one from its identifying fields.
func fingerprintDedupID(fp DomainFingerprint) string {
	return "fp:" + fp.Domain + ":" + strconv.FormatInt(fp.BlockHeight, 10) + ":" + fp.BlockHash + ":" + fp.ProducerQuid
}

// ---------------------------------------------------------------------------
// Fan-out (produce + forward)
// ---------------------------------------------------------------------------

// PushAnchor originates a new anchor push to all known peers.
// Called by the producing node AFTER the anchor has been sealed
// into a block and gossip-signed via SignAnchorGossip.
//
// This is the "producer" path. Forwarding hops use
// fanOutAnchorPush directly to exclude the sender.
func (node *QuidnugNode) PushAnchor(payload AnchorGossipMessage) {
	if !node.PushGossipEnabled {
		return
	}
	msg := AnchorPushMessage{
		SchemaVersion: GossipPushSchemaVersion,
		Payload:       payload,
		TTL:           node.GossipTTL,
		HopCount:      0,
		ForwardedBy:   node.NodeID,
	}
	// Mark our own messageID as seen so a forwarded copy
	// doesn't loop back and validate (and potentially fail
	// against post-apply state; see the QDP-0003 §8.3 lesson).
	if node.NonceLedger != nil {
		node.NonceLedger.markSeenGossip(payload.MessageID, time.Now())
	}
	node.fanOutAnchorPush(msg, "")
}

// PushFingerprint originates a new fingerprint push to all known
// peers.
func (node *QuidnugNode) PushFingerprint(payload DomainFingerprint) {
	if !node.PushGossipEnabled {
		return
	}
	msg := FingerprintPushMessage{
		SchemaVersion: GossipPushSchemaVersion,
		Payload:       payload,
		TTL:           node.GossipTTL,
		HopCount:      0,
		ForwardedBy:   node.NodeID,
	}
	if node.NonceLedger != nil {
		node.NonceLedger.markSeenGossip(fingerprintDedupID(payload), time.Now())
	}
	node.fanOutFingerprintPush(msg, "")
}

// maybePushAnchorFromBlock is the producer-side hook called from
// processBlockTransactions when an anchor-kind transaction lands
// in a Trusted block. Only the validator that sealed the block
// originates the push — receivers of the block via normal block
// propagation do not duplicate the push, because the forwarding
// will reach them through the push path itself.
//
// No-op when PushGossipEnabled is false. No-op when the block
// is not ours (another validator sealed it).
//
// Canonicalization note: the gossip message signs over
// OriginBlock.Hash rather than the full block (QDP-0003 §8.3).
// We pass the block through by value; the receiver re-computes
// the hash and checks it matches. If our local block state
// differs from what we emit, the signature will verify against
// our view — which is the correct behavior: the signer attests
// to their observation.
func (node *QuidnugNode) maybePushAnchorFromBlock(block Block, txIdx int) {
	if !node.PushGossipEnabled {
		return
	}
	if block.TrustProof.ValidatorID != node.NodeID {
		return
	}
	if txIdx < 0 || txIdx >= len(block.Transactions) {
		return
	}
	// Build the inline fingerprint. Producing is cheap: it's
	// just signing our current head for the block's domain.
	fp := DomainFingerprint{
		SchemaVersion: DomainFingerprintSchemaVersion,
		Domain:        block.TrustProof.TrustDomain,
		BlockHeight:   block.Index,
		BlockHash:     block.Hash,
		Timestamp:     time.Now().Unix(),
		ProducerQuid:  node.NodeID,
	}
	signedFp, err := node.SignDomainFingerprint(fp)
	if err != nil {
		logger.Error("gossip-push: sign fingerprint failed", "error", err)
		return
	}

	// Message ID: stable over (domain, block-hash, index).
	msgID := "anchor:" + block.TrustProof.TrustDomain + ":" + block.Hash + ":" + strconv.Itoa(txIdx)

	payload := AnchorGossipMessage{
		SchemaVersion:      AnchorGossipSchemaVersion,
		MessageID:          msgID,
		OriginDomain:       block.TrustProof.TrustDomain,
		OriginBlockHeight:  block.Index,
		OriginBlock:        block,
		AnchorTxIndex:      txIdx,
		DomainFingerprint:  signedFp,
		Timestamp:          time.Now().Unix(),
		GossipProducerQuid: node.NodeID,
	}
	// QDP-0010 / H2: attach a compact inclusion proof when the
	// block has a TransactionsRoot. Receivers prefer proof-based
	// verification (skips re-marshal of other txs) and fall back
	// to full-block otherwise.
	if block.TransactionsRoot != "" {
		if proof, err := MerkleProof(block.Transactions, txIdx); err == nil {
			payload.MerkleProof = proof
		}
	}
	signed, err := node.SignAnchorGossip(payload)
	if err != nil {
		logger.Error("gossip-push: sign anchor failed", "error", err)
		return
	}
	node.PushAnchor(signed)
}

// fanOutAnchorPush sends to every known peer except self and the
// node we received from. Each send is a separate goroutine —
// fire-and-forget, like BroadcastDomainInfo.
func (node *QuidnugNode) fanOutAnchorPush(msg AnchorPushMessage, exclude string) {
	peers := node.sortedForwardPeers(exclude)
	if len(peers) == 0 {
		return
	}
	body, err := json.Marshal(msg)
	if err != nil {
		logger.Error("gossip-push: marshal anchor failed", "error", err)
		return
	}
	for _, p := range peers {
		go node.postGossipPush(p, "/api/v2/gossip/push-anchor", body, "anchor")
	}
}

// fanOutFingerprintPush sends to every known peer except self
// and the node we received from.
func (node *QuidnugNode) fanOutFingerprintPush(msg FingerprintPushMessage, exclude string) {
	peers := node.sortedForwardPeers(exclude)
	if len(peers) == 0 {
		return
	}
	body, err := json.Marshal(msg)
	if err != nil {
		logger.Error("gossip-push: marshal fingerprint failed", "error", err)
		return
	}
	for _, p := range peers {
		go node.postGossipPush(p, "/api/v2/gossip/push-fingerprint", body, "fingerprint")
	}
}

// sortedForwardPeers returns the set of peers to forward to,
// excluding self and the immediate sender. Deterministic order
// (by ID) makes tests repeatable.
func (node *QuidnugNode) sortedForwardPeers(exclude string) []Node {
	node.KnownNodesMutex.RLock()
	peers := make([]Node, 0, len(node.KnownNodes))
	for _, n := range node.KnownNodes {
		if n.ID == node.NodeID || n.ID == exclude || n.Address == "" {
			continue
		}
		peers = append(peers, n)
	}
	node.KnownNodesMutex.RUnlock()
	sort.Slice(peers, func(i, j int) bool { return peers[i].ID < peers[j].ID })
	return peers
}

// postGossipPush performs the HTTP POST. Fire-and-forget; errors
// are logged at debug level only because an unreachable peer is
// not a protocol violation.
func (node *QuidnugNode) postGossipPush(target Node, path string, body []byte, kind string) {
	if len(body) > GossipPushMaxEnvelopeBytes {
		logger.Warn("gossip-push: refusing to send oversized envelope",
			"kind", kind, "size", len(body), "cap", GossipPushMaxEnvelopeBytes)
		return
	}
	endpoint := "http://" + target.Address + path
	req, err := http.NewRequest("POST", endpoint, bytes.NewReader(body))
	if err != nil {
		logger.Debug("gossip-push: NewRequest failed", "error", err)
		return
	}
	req.Header.Set("Content-Type", "application/json")

	// Node-to-node auth, identical pattern to sendDomainGossip.
	if secret := GetNodeAuthSecret(); secret != "" {
		ts := time.Now().Unix()
		sig := SignRequest("POST", path, body, secret, ts)
		req.Header.Set(NodeSignatureHeader, sig)
		req.Header.Set(NodeTimestampHeader, strconv.FormatInt(ts, 10))
	}

	resp, err := node.httpClient.Do(req)
	if err != nil {
		logger.Debug("gossip-push: send failed", "target", target.ID, "error", err)
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		logger.Debug("gossip-push: non-2xx",
			"target", target.ID, "path", path, "status", resp.StatusCode)
	}
}
