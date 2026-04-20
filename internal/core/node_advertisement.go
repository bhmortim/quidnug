// Package core — QDP-0014 node discovery + sharding.
//
// This file implements the NodeAdvertisementTransaction
// validation, registry, and expiry GC. Handler wiring is in
// handlers.go; registry commit on block-accept is in
// registry.go; block-inclusion switches are in
// block_operations.go and validation.go.
package core

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
	"sync"
	"time"
)

// QuidIDFromPublicKeyHex returns the 16-char hex quid ID
// derived from a hex-encoded SEC1 uncompressed public key, or
// the empty string on decode failure. Matches the derivation
// used elsewhere in the codebase (see e.g. validation.go).
func QuidIDFromPublicKeyHex(pubKeyHex string) string {
	raw, err := hex.DecodeString(pubKeyHex)
	if err != nil {
		return ""
	}
	return fmt.Sprintf("%x", sha256.Sum256(raw))[:16]
}

// Tuning constants for node-advertisement validation.
const (
	// MaxAdvertisementTTL caps how far in the future an
	// advertisement's ExpiresAt can sit. Advertisements with
	// longer TTLs are rejected so dead nodes age out.
	MaxAdvertisementTTL = 7 * 24 * time.Hour

	// MinAdvertisementInterval is the per-node rate limit on
	// accepted advertisements. Rapid-fire resubmissions are
	// rejected at validation time.
	MinAdvertisementInterval = 15 * time.Minute

	// MaxNodeEndpoints caps endpoints per advertisement. A node
	// that genuinely serves more regions should split into
	// multiple nodes with separate advertisements.
	MaxNodeEndpoints = 10

	// MaxSupportedDomainsPerAdvertisement caps the supported-
	// domain glob list length.
	MaxSupportedDomainsPerAdvertisement = 50

	// MaxSupportedDomainLength is the per-entry length limit
	// (matches DNS name length).
	MaxSupportedDomainLength = 253

	// MinOperatorTrustWeight is the minimum TRUST-edge weight
	// from operator → node that validates as a genuine
	// attestation.
	MinOperatorTrustWeight = 0.5
)

// validProtocols is the enumerated set of endpoint protocol
// strings accepted in advertisements. Anything else is rejected
// at validation.
var validProtocols = map[string]struct{}{
	"":         {}, // empty = unspecified; OK
	"http/1.1": {},
	"http/2":   {},
	"http/3":   {},
	"grpc":     {},
}

// NodeAdvertisementRegistry indexes the latest valid
// advertisement per node quid. Stale entries are pruned by the
// expiry GC goroutine.
type NodeAdvertisementRegistry struct {
	mu            sync.RWMutex
	ads           map[string]NodeAdvertisementTransaction // nodeQuid → advertisement
	lastPublished map[string]time.Time                    // nodeQuid → last-accepted time (for rate-limit)
}

// NewNodeAdvertisementRegistry constructs an empty registry.
func NewNodeAdvertisementRegistry() *NodeAdvertisementRegistry {
	return &NodeAdvertisementRegistry{
		ads:           make(map[string]NodeAdvertisementTransaction),
		lastPublished: make(map[string]time.Time),
	}
}

// Get returns the currently-valid advertisement for a node
// quid. The second return is false if there's no advertisement
// on file, OR if the advertisement has expired.
func (r *NodeAdvertisementRegistry) Get(nodeQuid string) (NodeAdvertisementTransaction, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	ad, ok := r.ads[nodeQuid]
	if !ok {
		return NodeAdvertisementTransaction{}, false
	}
	if ad.ExpiresAt < time.Now().UnixNano() {
		return NodeAdvertisementTransaction{}, false
	}
	return ad, true
}

// List returns every non-expired advertisement. Callers should
// not mutate the returned slice. Order is not guaranteed stable.
func (r *NodeAdvertisementRegistry) List() []NodeAdvertisementTransaction {
	r.mu.RLock()
	defer r.mu.RUnlock()
	nowNano := time.Now().UnixNano()
	out := make([]NodeAdvertisementTransaction, 0, len(r.ads))
	for _, ad := range r.ads {
		if ad.ExpiresAt >= nowNano {
			out = append(out, ad)
		}
	}
	return out
}

// ListByOperator returns every non-expired advertisement whose
// OperatorQuid field matches. Useful for serving the
// /api/v2/discovery/operator/{quid} endpoint.
func (r *NodeAdvertisementRegistry) ListByOperator(operatorQuid string) []NodeAdvertisementTransaction {
	all := r.List()
	out := make([]NodeAdvertisementTransaction, 0, len(all))
	for _, ad := range all {
		if ad.OperatorQuid == operatorQuid {
			out = append(out, ad)
		}
	}
	return out
}

// ListForDomain returns every non-expired advertisement whose
// SupportedDomains glob-matches the target domain. Used to
// build the per-domain endpoint-hint list for the discovery
// API's domain endpoint.
func (r *NodeAdvertisementRegistry) ListForDomain(domain string) []NodeAdvertisementTransaction {
	all := r.List()
	out := make([]NodeAdvertisementTransaction, 0, len(all))
	for _, ad := range all {
		if advertisementSupportsDomain(ad, domain) {
			out = append(out, ad)
		}
	}
	return out
}

func advertisementSupportsDomain(ad NodeAdvertisementTransaction, domain string) bool {
	// Empty SupportedDomains means the node serves only the
	// domains it's a consortium member for. That cross-check
	// happens in the discovery handler against the domain's
	// Validators map, not here.
	if len(ad.SupportedDomains) == 0 {
		return false
	}
	for _, pat := range ad.SupportedDomains {
		if MatchDomainPattern(domain, pat) {
			return true
		}
	}
	return false
}

// upsert replaces the current advertisement for a node quid
// unconditionally. The caller has validated nonce monotonicity.
// Also records the accept-time for rate-limit enforcement.
func (r *NodeAdvertisementRegistry) upsert(ad NodeAdvertisementTransaction, at time.Time) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.ads[ad.NodeQuid] = ad
	r.lastPublished[ad.NodeQuid] = at
}

// currentNonce returns the advertisement-nonce of the currently-
// held advertisement for nodeQuid, or 0 if none.
func (r *NodeAdvertisementRegistry) currentNonce(nodeQuid string) int64 {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if ad, ok := r.ads[nodeQuid]; ok {
		return ad.AdvertisementNonce
	}
	return 0
}

// lastPublishedAt returns when the most recent advertisement
// for nodeQuid was accepted (zero time if never).
func (r *NodeAdvertisementRegistry) lastPublishedAt(nodeQuid string) time.Time {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.lastPublished[nodeQuid]
}

// GarbageCollect removes expired advertisements. Returns the
// number removed so the caller can log.
func (r *NodeAdvertisementRegistry) GarbageCollect() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	now := time.Now().UnixNano()
	removed := 0
	for quid, ad := range r.ads {
		if ad.ExpiresAt < now {
			delete(r.ads, quid)
			delete(r.lastPublished, quid)
			removed++
		}
	}
	return removed
}

// updateNodeAdvertisementRegistry commits a validated
// NodeAdvertisementTransaction to the registry. Called from
// registry.go's processBlockTransactions when the transaction
// has been included in an accepted block.
//
// Idempotent — replaying a block with the same advertisement
// yields the same registry state (the nonce check in
// validation means only strictly-increasing advertisements
// reach this path).
func (node *QuidnugNode) updateNodeAdvertisementRegistry(tx NodeAdvertisementTransaction) {
	if node.NodeAdvertisementRegistry == nil {
		return
	}
	node.NodeAdvertisementRegistry.upsert(tx, time.Now())
	logger.Debug("Updated node advertisement registry",
		"nodeQuid", tx.NodeQuid,
		"operatorQuid", tx.OperatorQuid,
		"endpoints", len(tx.Endpoints),
		"expiresAt", tx.ExpiresAt,
		"nonce", tx.AdvertisementNonce)
}

// runAdvertisementGC runs a periodic GC sweep on the
// advertisement registry. Cancellable via ctx.
func (node *QuidnugNode) runAdvertisementGC(stop <-chan struct{}, interval time.Duration) {
	if node.NodeAdvertisementRegistry == nil {
		return
	}
	t := time.NewTicker(interval)
	defer t.Stop()
	for {
		select {
		case <-stop:
			return
		case <-t.C:
			n := node.NodeAdvertisementRegistry.GarbageCollect()
			if n > 0 {
				logger.Info("Pruned expired node advertisements", "count", n)
			}
		}
	}
}

// ValidateNodeAdvertisementTransaction enforces the QDP-0014 §4.1
// rules on an incoming advertisement:
//
//  1. Trust-domain must exist + be accepted by this node.
//  2. Self-sign consistency — the signing pubkey's quid must
//     equal NodeQuid.
//  3. Operator attestation — a direct TRUST edge from
//     OperatorQuid to NodeQuid at weight ≥ 0.5 must exist.
//  4. Advertisement-nonce monotonic per-node.
//  5. ExpiresAt > now AND ExpiresAt - now <= 7 days.
//  6. 1 <= len(Endpoints) <= 10; each URL starts with https://
//     and parses; protocol in allowed set; region <= 64 chars;
//     priority 0..100; weight 0..10000.
//  7. len(SupportedDomains) <= 50; each <= 253 chars.
//  8. Rate limit: last advertisement for this NodeQuid was >= 15min ago.
//  9. ProtocolVersion matches semver-ish pattern.
//  10. Signature verifies against the embedded PublicKey.
func (node *QuidnugNode) ValidateNodeAdvertisementTransaction(tx NodeAdvertisementTransaction) bool {
	// 1. Domain must be registered + supported.
	if tx.TrustDomain == "" {
		logger.Warn("Node advertisement missing trust domain", "txId", tx.ID)
		return false
	}
	node.TrustDomainsMutex.RLock()
	_, domainExists := node.TrustDomains[tx.TrustDomain]
	node.TrustDomainsMutex.RUnlock()
	if !domainExists {
		logger.Warn("Node advertisement from unknown trust domain",
			"domain", tx.TrustDomain, "txId", tx.ID)
		return false
	}
	if !node.IsDomainSupported(tx.TrustDomain) {
		logger.Warn("Node advertisement trust domain not supported by this node",
			"domain", tx.TrustDomain, "txId", tx.ID)
		return false
	}

	// 2. Required fields.
	if tx.NodeQuid == "" || !IsValidQuidID(tx.NodeQuid) {
		logger.Warn("Node advertisement has invalid NodeQuid",
			"nodeQuid", tx.NodeQuid, "txId", tx.ID)
		return false
	}
	if tx.OperatorQuid == "" || !IsValidQuidID(tx.OperatorQuid) {
		logger.Warn("Node advertisement has invalid OperatorQuid",
			"operatorQuid", tx.OperatorQuid, "txId", tx.ID)
		return false
	}
	if tx.Signature == "" || tx.PublicKey == "" {
		logger.Warn("Node advertisement missing signature or public key",
			"txId", tx.ID)
		return false
	}

	// Self-sign consistency — the signing pubkey must hash to NodeQuid.
	computedNodeQuid := QuidIDFromPublicKeyHex(tx.PublicKey)
	if computedNodeQuid == "" || computedNodeQuid != tx.NodeQuid {
		logger.Warn("Node advertisement NodeQuid does not match signing public key",
			"expected", tx.NodeQuid, "computed", computedNodeQuid, "txId", tx.ID)
		return false
	}

	// 3. Advertisement nonce monotonicity.
	if tx.AdvertisementNonce <= 0 {
		logger.Warn("Node advertisement has non-positive advertisementNonce",
			"nonce", tx.AdvertisementNonce, "txId", tx.ID)
		return false
	}
	if node.NodeAdvertisementRegistry != nil {
		prev := node.NodeAdvertisementRegistry.currentNonce(tx.NodeQuid)
		if tx.AdvertisementNonce <= prev {
			logger.Warn("Node advertisement nonce must be strictly greater than previous",
				"previous", prev, "provided", tx.AdvertisementNonce, "txId", tx.ID)
			return false
		}
	}

	// 4. Expiry sanity.
	nowNano := time.Now().UnixNano()
	if tx.ExpiresAt <= nowNano {
		logger.Warn("Node advertisement already expired",
			"expiresAt", tx.ExpiresAt, "now", nowNano, "txId", tx.ID)
		return false
	}
	maxExpiry := nowNano + MaxAdvertisementTTL.Nanoseconds()
	if tx.ExpiresAt > maxExpiry {
		logger.Warn("Node advertisement TTL exceeds maximum",
			"expiresAt", tx.ExpiresAt, "max", maxExpiry, "txId", tx.ID)
		return false
	}

	// 5. Endpoints.
	if len(tx.Endpoints) == 0 {
		logger.Warn("Node advertisement has no endpoints", "txId", tx.ID)
		return false
	}
	if len(tx.Endpoints) > MaxNodeEndpoints {
		logger.Warn("Node advertisement has too many endpoints",
			"count", len(tx.Endpoints), "max", MaxNodeEndpoints, "txId", tx.ID)
		return false
	}
	for i, ep := range tx.Endpoints {
		if err := validateEndpoint(ep); err != nil {
			logger.Warn("Node advertisement endpoint rejected",
				"index", i, "url", ep.URL, "err", err, "txId", tx.ID)
			return false
		}
	}

	// 6. SupportedDomains glob list.
	if len(tx.SupportedDomains) > MaxSupportedDomainsPerAdvertisement {
		logger.Warn("Node advertisement supportedDomains list too long",
			"count", len(tx.SupportedDomains), "max", MaxSupportedDomainsPerAdvertisement,
			"txId", tx.ID)
		return false
	}
	for _, d := range tx.SupportedDomains {
		if len(d) == 0 || len(d) > MaxSupportedDomainLength {
			logger.Warn("Node advertisement supportedDomains entry invalid",
				"entry", d, "txId", tx.ID)
			return false
		}
	}

	// 7. Protocol version (minimal format check).
	if !isValidProtocolVersion(tx.ProtocolVersion) {
		logger.Warn("Node advertisement protocolVersion invalid",
			"protocolVersion", tx.ProtocolVersion, "txId", tx.ID)
		return false
	}

	// 8. Rate limit.
	if node.NodeAdvertisementRegistry != nil {
		last := node.NodeAdvertisementRegistry.lastPublishedAt(tx.NodeQuid)
		if !last.IsZero() && time.Since(last) < MinAdvertisementInterval {
			logger.Warn("Node advertisement rate-limited",
				"nodeQuid", tx.NodeQuid, "sinceLast", time.Since(last).String(),
				"min", MinAdvertisementInterval.String(), "txId", tx.ID)
			return false
		}
	}

	// 9. Operator attestation — look for a TRUST edge from
	// OperatorQuid → NodeQuid in some operators.network.* domain
	// at level ≥ MinOperatorTrustWeight.
	if !node.hasOperatorAttestation(tx.OperatorQuid, tx.NodeQuid) {
		logger.Warn("Node advertisement missing operator attestation",
			"operator", tx.OperatorQuid, "node", tx.NodeQuid, "txId", tx.ID)
		return false
	}

	// 10. Signature — verify over canonical bytes of the tx with
	// Signature cleared.
	txCopy := tx
	txCopy.Signature = ""
	signableData, err := json.Marshal(txCopy)
	if err != nil {
		logger.Error("Node advertisement marshal for signature failed",
			"txId", tx.ID, "err", err)
		return false
	}
	if !VerifySignature(tx.PublicKey, signableData, tx.Signature) {
		logger.Warn("Node advertisement signature invalid", "txId", tx.ID)
		return false
	}

	return true
}

// validateEndpoint returns non-nil if the endpoint violates one
// of the QDP-0014 constraints.
func validateEndpoint(ep NodeEndpoint) error {
	if ep.URL == "" {
		return fmt.Errorf("empty url")
	}
	if !strings.HasPrefix(ep.URL, "https://") {
		return fmt.Errorf("url must be https://")
	}
	u, err := url.Parse(ep.URL)
	if err != nil {
		return fmt.Errorf("url parse: %w", err)
	}
	if u.Host == "" {
		return fmt.Errorf("url missing host")
	}
	if _, ok := validProtocols[ep.Protocol]; !ok {
		return fmt.Errorf("unknown protocol %q", ep.Protocol)
	}
	if len(ep.Region) > 64 {
		return fmt.Errorf("region too long")
	}
	if ep.Priority < 0 || ep.Priority > 100 {
		return fmt.Errorf("priority out of range (0..100)")
	}
	if ep.Weight < 0 || ep.Weight > 10000 {
		return fmt.Errorf("weight out of range (0..10000)")
	}
	return nil
}

// isValidProtocolVersion accepts any non-empty string up to 32
// chars matching a lenient major.minor[.patch] pattern. We
// don't want to enforce full semver since node operators may
// append build metadata.
func isValidProtocolVersion(v string) bool {
	if v == "" || len(v) > 32 {
		return false
	}
	// Minimum: "N.N" where each N is all-digits. Anything after
	// the second digit sequence is free-form.
	dots := 0
	sawDigit := false
	for _, c := range v {
		if c >= '0' && c <= '9' {
			sawDigit = true
			continue
		}
		if c == '.' {
			if !sawDigit {
				return false
			}
			dots++
			if dots > 1 {
				// Past the second digit group — free-form.
				return true
			}
			sawDigit = false
			continue
		}
		if dots < 1 {
			return false
		}
	}
	return sawDigit && dots >= 1
}

// hasOperatorAttestation checks for a direct TRUST edge from
// operatorQuid to nodeQuid at level >= MinOperatorTrustWeight.
// Returns true if such an edge exists.
//
// NOTE: the reference node's `TrustRegistry` is currently
// domain-agnostic (it collapses edges across domains to a
// single truster→trustee weight). The QDP-0014 design calls
// for a per-domain attestation under the operator's reserved
// meta-domain, but enforcing the domain constraint requires a
// richer registry than currently exists. For this phase we
// enforce the direct-edge + weight requirements only; a
// follow-up can add domain scoping once the trust registry
// tracks domains.
//
// We walk the TrustRegistry directly rather than via
// ComputeRelationalTrust because we want a strict 1-hop edge
// (transitive attestation isn't valid: you can only attest
// nodes you directly operate).
func (node *QuidnugNode) hasOperatorAttestation(operatorQuid, nodeQuid string) bool {
	node.TrustRegistryMutex.RLock()
	defer node.TrustRegistryMutex.RUnlock()

	trustees, ok := node.TrustRegistry[operatorQuid]
	if !ok {
		return false
	}
	level, found := trustees[nodeQuid]
	return found && level >= MinOperatorTrustWeight
}
