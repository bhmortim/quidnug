// Package core. domain.go — trust-domain registration and subdomain authority.
package core

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
)

func GetParentDomain(domain string) string {
	idx := strings.Index(domain, ".")
	if idx == -1 {
		return ""
	}
	return domain[idx+1:]
}

// IsRootDomain checks if a domain is a root domain (has no dots).
func IsRootDomain(domain string) bool {
	return !strings.Contains(domain, ".")
}

// ValidateSubdomainAuthority checks if validators of a child domain are authorized by parent domain validators.
// Returns true if:
// - The parent domain is not registered (no authority to check against), OR
// - At least one parent validator trusts at least one child validator
// Returns false if parent exists but no trust relationship is found.
// For domains not yet registered, use validateProposedSubdomainAuthority during registration.
func (node *QuidnugNode) ValidateSubdomainAuthority(childDomain, parentDomain string) bool {
	node.TrustDomainsMutex.RLock()
	parent, parentExists := node.TrustDomains[parentDomain]
	child, childExists := node.TrustDomains[childDomain]
	node.TrustDomainsMutex.RUnlock()

	if !parentExists {
		// Parent not registered, no authority to check against
		return true
	}

	if !childExists {
		// Child not registered, can't validate with this function
		return false
	}

	return node.checkValidatorTrust(parent.ValidatorNodes, child.ValidatorNodes)
}

// checkValidatorTrust checks if any validator from parentValidators trusts any validator from childValidators.
// Returns true if at least one parent validator has non-zero trust in at least one child validator.
func (node *QuidnugNode) checkValidatorTrust(parentValidators, childValidators []string) bool {
	for _, parentValidator := range parentValidators {
		for _, childValidator := range childValidators {
			trustLevel, _, _ := node.ComputeRelationalTrust(parentValidator, childValidator, DefaultTrustMaxDepth)
			if trustLevel > 0 {
				return true
			}
		}
	}
	return false
}

// validateProposedSubdomainAuthority checks if proposed validators for a new domain
// are authorized by an existing parent domain's validators.
func (node *QuidnugNode) validateProposedSubdomainAuthority(parentDomain string, childValidators []string) bool {
	node.TrustDomainsMutex.RLock()
	parent, parentExists := node.TrustDomains[parentDomain]
	node.TrustDomainsMutex.RUnlock()

	if !parentExists {
		// Parent not registered, no authority to check against - allow
		return true
	}

	return node.checkValidatorTrust(parent.ValidatorNodes, childValidators)
}

// IsDomainSupported checks if a domain is supported by this node.
// If SupportedDomains is empty, all domains are allowed.
// Supports wildcard patterns like "*.example.com" for subdomain matching.
func (node *QuidnugNode) IsDomainSupported(domain string) bool {
	// Empty list means all domains are supported
	if len(node.SupportedDomains) == 0 {
		return true
	}

	// Special case: "default" domain is always supported if no restrictions
	// or if explicitly listed
	for _, pattern := range node.SupportedDomains {
		if MatchDomainPattern(domain, pattern) {
			return true
		}
	}

	return false
}

// RegisterTrustDomain registers a new trust domain with this node.
// For subdomains (domains containing dots), validates that at least one parent
// domain validator trusts the new domain's validators (if RequireParentDomainAuth is enabled).
//
// Lock ordering: the subdomain-authority check calls into
// validateProposedSubdomainAuthority which, via ComputeRelationalTrust,
// takes TrustRegistryMutex. To avoid lock-order inversion and the
// previous defer-plus-manual-unlock double-unlock bug, the check runs
// BEFORE TrustDomainsMutex is acquired. Everything inside the lock is
// purely local map mutation.
func (node *QuidnugNode) RegisterTrustDomain(domain TrustDomain) error {
	// Policy gates, no lock required.
	if !node.AllowDomainRegistration {
		return fmt.Errorf("dynamic domain registration is not allowed on this node")
	}
	if !node.IsDomainSupported(domain.Name) {
		return fmt.Errorf("trust domain %s is not supported by this node", domain.Name)
	}

	// Ensure this node is included as a validator before the
	// subdomain-authority check inspects the validator set.
	validatorFound := false
	for _, validatorID := range domain.ValidatorNodes {
		if validatorID == node.NodeID {
			validatorFound = true
			break
		}
	}
	if !validatorFound {
		domain.ValidatorNodes = append(domain.ValidatorNodes, node.NodeID)
	}

	// Subdomain authority check — uses its own locks internally.
	if node.RequireParentDomainAuth && !IsRootDomain(domain.Name) {
		parentDomain := GetParentDomain(domain.Name)
		if !node.validateProposedSubdomainAuthority(parentDomain, domain.ValidatorNodes) {
			return fmt.Errorf("subdomain %s requires authorization from parent domain %s validators", domain.Name, parentDomain)
		}
	}

	// Initialize validators map if empty
	if domain.Validators == nil {
		domain.Validators = make(map[string]float64)
	}
	// Add this node as a validator with full participation weight
	domain.Validators[node.NodeID] = 1.0

	// Initialize ValidatorPublicKeys map if empty
	if domain.ValidatorPublicKeys == nil {
		domain.ValidatorPublicKeys = make(map[string]string)
	}
	// Add this node's public key for signature verification
	domain.ValidatorPublicKeys[node.NodeID] = node.GetPublicKeyHex()

	// QDP-0012 Phase 1 — apply the single-registrant governance
	// fallback. If the registrant didn't supply an explicit
	// Governors set we install the registering node as the sole
	// governor with unanimous quorum (1.0), which preserves pre-
	// QDP-0012 behavior: the registrant is the only party who
	// can mutate their own domain.
	//
	// Callers that want a multi-governor consortium pass a
	// populated `domain.Governors` + `domain.GovernorPublicKeys`
	// + `domain.GovernanceQuorum`. Phase 1 doesn't enforce any
	// of this — Phase 3 wires enforcement behind the QDP-0009
	// fork-activation flag.
	if len(domain.Governors) == 0 {
		domain.Governors = map[string]float64{node.NodeID: 1.0}
	}
	if domain.GovernorPublicKeys == nil {
		domain.GovernorPublicKeys = make(map[string]string)
	}
	// Make sure every declared governor has a public key on file.
	// For the registering node we can fill it from our own key;
	// for other governors the registrant is expected to have
	// supplied the keys. Missing keys leave the entry empty (log
	// a warning so Phase 2 validation knows what to flag).
	if _, ok := domain.GovernorPublicKeys[node.NodeID]; !ok {
		if _, selfIsGovernor := domain.Governors[node.NodeID]; selfIsGovernor {
			domain.GovernorPublicKeys[node.NodeID] = node.GetPublicKeyHex()
		}
	}
	for govQuid := range domain.Governors {
		if _, ok := domain.GovernorPublicKeys[govQuid]; !ok {
			logger.Warn("Governor declared without accompanying public key",
				"domain", domain.Name, "governorQuid", govQuid)
		}
	}
	if domain.GovernanceQuorum == 0 {
		domain.GovernanceQuorum = 1.0 // unanimous, matches the single-registrant default
	}
	if domain.ParentDelegationMode == "" {
		domain.ParentDelegationMode = DelegationModeSelf
	}

	// Single exclusive section for the registry mutation.
	node.TrustDomainsMutex.Lock()
	defer node.TrustDomainsMutex.Unlock()
	if _, exists := node.TrustDomains[domain.Name]; exists {
		return fmt.Errorf("trust domain %s already exists", domain.Name)
	}
	node.TrustDomains[domain.Name] = domain

	logger.Info("Registered new trust domain",
		"domain", domain.Name,
		"validators", len(domain.ValidatorNodes),
		"governors", len(domain.Governors),
		"governanceQuorum", domain.GovernanceQuorum,
		"delegationMode", domain.ParentDelegationMode)
	return nil
}

// BootstrapDomainFromBlock registers a TrustDomain locally based
// solely on a block's TrustProof. This is the lazy-bootstrap path
// for ENG-80: a node that joined an existing mesh has no record of
// dynamically-registered domains until it sees a block for one. The
// block-sync receive path calls this helper before cryptographic
// validation so the freshly-bootstrapped domain entry is in place
// when ValidateBlockTiered's per-domain validator-set lookup runs.
//
// Bootstrap policy:
//
//   - The proof MUST carry both ValidatorID and ValidatorPublicKey
//     (modern block format). Older blocks without an embedded
//     pubkey can't be safely bootstrapped from — we don't know
//     who signed them — and are rejected upstream.
//   - The validator's id is recomputed from the embedded public
//     key (sha256[:16]); a mismatch is treated as a forged claim
//     and the bootstrap is refused. This is the same self-
//     consistency check ValidateBlockCryptographic enforces, so
//     an attacker cannot use a fake validator id to seed an
//     unauthorized domain entry.
//   - The bootstrapped domain has a single validator (the block
//     signer) with full participation weight, the registrant as
//     sole governor with quorum 1.0, and a default 0.75 trust
//     threshold. Any of these can be widened later by gossip
//     fingerprints, TRUST_DOMAIN transactions, or
//     DOMAIN_GOVERNANCE updates without re-bootstrapping.
//   - This function is idempotent: a domain that already exists
//     locally is left untouched and a nil error is returned. The
//     caller does not need to pre-check existence.
//
// Trust-anchor invariant: this function never bypasses signature
// verification. ReceiveBlock still runs ValidateBlockCryptographic
// after the bootstrap; if the signature doesn't match the embedded
// key, the block is rejected and the bootstrapped entry is the
// only durable side effect (and even that is harmless: a stub
// domain with one validator pubkey is no different from a domain
// the operator could have configured directly).
func (node *QuidnugNode) BootstrapDomainFromBlock(block Block) error {
	proof := block.TrustProof
	if proof.TrustDomain == "" {
		return fmt.Errorf("bootstrap: empty trust domain in block proof")
	}
	if proof.TrustDomain == "genesis" {
		// Genesis is a per-node anchor, never a real domain to
		// bootstrap from a peer.
		return fmt.Errorf("bootstrap: refusing to bootstrap from genesis-domain block")
	}
	if proof.ValidatorID == "" {
		return fmt.Errorf("bootstrap: missing validator id in block proof")
	}
	if proof.ValidatorPublicKey == "" {
		return fmt.Errorf("bootstrap: missing validator public key in block proof " +
			"(block predates ENG-80 bootstrap support)")
	}

	// Recompute validator id from the embedded public key. This is
	// the same self-consistency check ValidateBlockCryptographic
	// runs at line 627; we duplicate it here so a forged ID/pubkey
	// pair cannot poison the local TrustDomains map even
	// transiently before crypto validation runs.
	pubBytes, err := hex.DecodeString(proof.ValidatorPublicKey)
	if err != nil {
		return fmt.Errorf("bootstrap: validator public key not valid hex: %w", err)
	}
	computedID := fmt.Sprintf("%x", sha256.Sum256(pubBytes))[:16]
	if computedID != proof.ValidatorID {
		return fmt.Errorf("bootstrap: validator public key does not match validator id "+
			"(claimed=%s, computed=%s)", proof.ValidatorID, computedID)
	}

	// Idempotent insertion under the registry lock.
	node.TrustDomainsMutex.Lock()
	defer node.TrustDomainsMutex.Unlock()
	if _, exists := node.TrustDomains[proof.TrustDomain]; exists {
		return nil
	}

	bootstrapped := TrustDomain{
		Name:                proof.TrustDomain,
		ValidatorNodes:      []string{proof.ValidatorID},
		Validators:          map[string]float64{proof.ValidatorID: 1.0},
		ValidatorPublicKeys: map[string]string{proof.ValidatorID: proof.ValidatorPublicKey},
		TrustThreshold:      0.75,
		BlockchainHead:      "",
		// QDP-0012: single-governor fallback matches what
		// RegisterTrustDomain installs for self-registered
		// domains. The block signer is the registrant from this
		// node's perspective.
		Governors:            map[string]float64{proof.ValidatorID: 1.0},
		GovernorPublicKeys:   map[string]string{proof.ValidatorID: proof.ValidatorPublicKey},
		GovernanceQuorum:     1.0,
		ParentDelegationMode: DelegationModeSelf,
	}
	node.TrustDomains[proof.TrustDomain] = bootstrapped

	logger.Info("Bootstrapped trust domain from peer-served block",
		"domain", proof.TrustDomain,
		"validator", proof.ValidatorID,
		"blockIndex", block.Index)
	return nil
}

// MatchDomainPattern checks if a domain matches a pattern.
// Patterns can be exact matches or wildcard patterns like "*.example.com".
// Wildcard patterns match any subdomain but not the base domain itself.
func MatchDomainPattern(domain, pattern string) bool {
	if domain == "" || pattern == "" {
		return false
	}
	if domain == pattern {
		return true
	}
	if strings.HasPrefix(pattern, "*.") {
		suffix := pattern[1:]
		if strings.HasSuffix(domain, suffix) && len(domain) > len(suffix) {
			return true
		}
	}
	return false
}
