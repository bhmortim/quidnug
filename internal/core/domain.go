// Package core. domain.go — trust-domain registration and subdomain authority.
package core

import (
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
func (node *QuidnugNode) RegisterTrustDomain(domain TrustDomain) error {
	// Check if dynamic domain registration is allowed
	if !node.AllowDomainRegistration {
		return fmt.Errorf("dynamic domain registration is not allowed on this node")
	}

	// Check if the domain is supported
	if !node.IsDomainSupported(domain.Name) {
		return fmt.Errorf("trust domain %s is not supported by this node", domain.Name)
	}

	node.TrustDomainsMutex.Lock()
	defer node.TrustDomainsMutex.Unlock()

	if _, exists := node.TrustDomains[domain.Name]; exists {
		return fmt.Errorf("trust domain %s already exists", domain.Name)
	}

	// Ensure this node is included as a validator
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

	// Validate subdomain authority if required and this is not a root domain
	if node.RequireParentDomainAuth && !IsRootDomain(domain.Name) {
		parentDomain := GetParentDomain(domain.Name)
		// Need to release lock before calling validateProposedSubdomainAuthority
		// which may acquire TrustRegistryMutex via ComputeRelationalTrust
		node.TrustDomainsMutex.Unlock()

		if !node.validateProposedSubdomainAuthority(parentDomain, domain.ValidatorNodes) {
			// Re-acquire lock is not needed as we're returning an error
			return fmt.Errorf("subdomain %s requires authorization from parent domain %s validators", domain.Name, parentDomain)
		}

		// Re-acquire lock to continue registration
		node.TrustDomainsMutex.Lock()
		// Re-check domain doesn't exist (could have been added while lock was released)
		if _, exists := node.TrustDomains[domain.Name]; exists {
			return fmt.Errorf("trust domain %s already exists", domain.Name)
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

	// Register the domain
	node.TrustDomains[domain.Name] = domain

	logger.Info("Registered new trust domain", "domain", domain.Name, "validators", len(domain.ValidatorNodes))
	return nil
}
