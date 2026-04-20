// dns_attestation.go — QDP-0023 Phase 1: DNS-anchored
// identity attestation core protocol.
//
// Phase 1 scope (docs/design/0023-dns-anchored-attestation.md
// §11.1):
//
//   - The seven event types (DNS_CLAIM, DNS_CHALLENGE,
//     DNS_ATTESTATION, DNS_RENEWAL, DNS_REVOCATION,
//     AUTHORITY_DELEGATE, AUTHORITY_DELEGATE_REVOCATION) are
//     defined in types.go.
//   - This module adds the DNSAttestationRegistry + its
//     admission + query helpers.
//   - Validation + signature-check per event type.
//   - Query endpoints live in handlers_dns_attestation.go.
//
// Out of scope for Phase 1 (comes in Phase 2-5 per the QDP):
//   - The actual multi-resolver DNS verification flow (ran by
//     `cmd/quidnug-dns-verifier`, not the node).
//   - Payment settlement (off-chain).
//   - Federation-import of attestations from peer roots.
//   - Public verify.quidnug.com web UI.
//
// The Phase 1 node provides the protocol bedrock so the
// off-chain verifier + UI can land in subsequent sprints.

package core

import (
	"sync"
	"time"
)

// DNSAttestationRegistry tracks the lifecycle of every DNS
// attestation the node has accepted. Indexed several ways
// so resolution queries are O(1) in the common path.
type DNSAttestationRegistry struct {
	mu sync.RWMutex

	// attestations stores DNS_ATTESTATION by its tx ID.
	attestations map[string]DNSAttestationTransaction

	// byDomain indexes attestation tx IDs by the domain being
	// attested. One domain may have attestations from multiple
	// roots; we keep them all.
	byDomain map[string][]string

	// byRoot indexes attestation tx IDs by the attesting root.
	byRoot map[string][]string

	// revocations stores revocation tx IDs keyed by the
	// attestation they revoke. Presence of a key means the
	// attestation has at least one revocation; use
	// IsAttestationRevoked to check effective state.
	revocations map[string][]DNSRevocationTransaction

	// renewals stores renewal tx IDs keyed by the prior
	// attestation they extend.
	renewals map[string][]DNSRenewalTransaction

	// claims + challenges are lighter-state but we still keep
	// them so the verifier + UI can reconstruct the full
	// admission history.
	claims     map[string]DNSClaimTransaction
	challenges map[string]DNSChallengeTransaction

	// nonceRegistry tracks the highest DNS_* tx nonce seen per
	// signer, for replay protection across the DNS family.
	nonceRegistry map[string]int64

	// delegations stores AUTHORITY_DELEGATE by tx ID + indexed
	// by attestation ref.
	delegations         map[string]AuthorityDelegateTransaction
	delegationsByAttRef map[string][]string
	delegationRevocations map[string][]AuthorityDelegateRevocationTransaction
}

// NewDNSAttestationRegistry returns an empty registry.
func NewDNSAttestationRegistry() *DNSAttestationRegistry {
	return &DNSAttestationRegistry{
		attestations:          make(map[string]DNSAttestationTransaction),
		byDomain:              make(map[string][]string),
		byRoot:                make(map[string][]string),
		revocations:           make(map[string][]DNSRevocationTransaction),
		renewals:              make(map[string][]DNSRenewalTransaction),
		claims:                make(map[string]DNSClaimTransaction),
		challenges:            make(map[string]DNSChallengeTransaction),
		nonceRegistry:         make(map[string]int64),
		delegations:           make(map[string]AuthorityDelegateTransaction),
		delegationsByAttRef:   make(map[string][]string),
		delegationRevocations: make(map[string][]AuthorityDelegateRevocationTransaction),
	}
}

// currentNonce returns the highest nonce observed for the
// given signer across any DNS_* tx.
func (r *DNSAttestationRegistry) currentNonce(signer string) int64 {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.nonceRegistry[signer]
}

// recordNonce bumps the signer's nonce marker if the supplied
// nonce is strictly greater. Safe for concurrent use.
func (r *DNSAttestationRegistry) recordNonce(signer string, nonce int64) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if cur, ok := r.nonceRegistry[signer]; !ok || nonce > cur {
		r.nonceRegistry[signer] = nonce
	}
}

// --- Admission paths ---

// admitClaim stores a DNSClaimTransaction.
func (r *DNSAttestationRegistry) admitClaim(tx DNSClaimTransaction) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.claims[tx.ID] = tx
	if cur, ok := r.nonceRegistry[tx.OwnerQuid]; !ok || tx.Nonce > cur {
		r.nonceRegistry[tx.OwnerQuid] = tx.Nonce
	}
}

// admitChallenge stores a DNSChallengeTransaction.
func (r *DNSAttestationRegistry) admitChallenge(tx DNSChallengeTransaction) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.challenges[tx.ID] = tx
	signer := ""
	if len(tx.PublicKey) >= 16 {
		// Root-published: the tx signer derives from PublicKey
		// via the standard quid ID derivation. For index
		// purposes we use the PublicKey hex prefix as a
		// lightweight handle. A future cleanup can plumb the
		// real quid ID.
		signer = tx.PublicKey[:16]
	}
	if signer != "" {
		if cur, ok := r.nonceRegistry[signer]; !ok || tx.TxNonce > cur {
			r.nonceRegistry[signer] = tx.TxNonce
		}
	}
}

// admitAttestation stores a DNSAttestationTransaction.
func (r *DNSAttestationRegistry) admitAttestation(tx DNSAttestationTransaction) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.attestations[tx.ID] = tx
	r.byDomain[tx.Domain] = append(r.byDomain[tx.Domain], tx.ID)
	r.byRoot[tx.RootQuid] = append(r.byRoot[tx.RootQuid], tx.ID)
	if cur, ok := r.nonceRegistry[tx.RootQuid]; !ok || tx.Nonce > cur {
		r.nonceRegistry[tx.RootQuid] = tx.Nonce
	}
}

// admitRenewal stores a DNSRenewalTransaction.
func (r *DNSAttestationRegistry) admitRenewal(tx DNSRenewalTransaction) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.renewals[tx.PriorAttestationRef] = append(r.renewals[tx.PriorAttestationRef], tx)
	// A renewal typically bumps the prior attestation's
	// effective ValidUntil. Update the stored attestation in
	// place to keep reads fast.
	if prior, ok := r.attestations[tx.PriorAttestationRef]; ok {
		if tx.NewValidUntil > prior.ValidUntil {
			prior.ValidUntil = tx.NewValidUntil
			if tx.NewTLSFingerprintSHA256 != "" {
				prior.TLSFingerprintSHA256 = tx.NewTLSFingerprintSHA256
			}
			r.attestations[tx.PriorAttestationRef] = prior
		}
	}
}

// admitRevocation stores a DNSRevocationTransaction.
func (r *DNSAttestationRegistry) admitRevocation(tx DNSRevocationTransaction) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.revocations[tx.AttestationRef] = append(r.revocations[tx.AttestationRef], tx)
	// Zero out the attestation's ValidUntil so weighted
	// queries naturally exclude it.
	if prior, ok := r.attestations[tx.AttestationRef]; ok {
		prior.ValidUntil = tx.RevokedAt
		r.attestations[tx.AttestationRef] = prior
	}
}

// admitDelegate stores an AuthorityDelegateTransaction.
func (r *DNSAttestationRegistry) admitDelegate(tx AuthorityDelegateTransaction) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.delegations[tx.ID] = tx
	r.delegationsByAttRef[tx.AttestationRef] = append(
		r.delegationsByAttRef[tx.AttestationRef], tx.ID)
}

// admitDelegateRevocation stores an AuthorityDelegateRevocationTransaction.
func (r *DNSAttestationRegistry) admitDelegateRevocation(tx AuthorityDelegateRevocationTransaction) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.delegationRevocations[tx.DelegationRef] = append(
		r.delegationRevocations[tx.DelegationRef], tx)
}

// --- Read helpers ---

// GetAttestationsForDomain returns all attestations for a
// domain, oldest-first. Revoked attestations are included;
// callers that want "live" results should filter by
// IsAttestationRevoked and ValidUntil.
func (r *DNSAttestationRegistry) GetAttestationsForDomain(domain string) []DNSAttestationTransaction {
	r.mu.RLock()
	defer r.mu.RUnlock()
	ids := r.byDomain[domain]
	out := make([]DNSAttestationTransaction, 0, len(ids))
	for _, id := range ids {
		if att, ok := r.attestations[id]; ok {
			out = append(out, att)
		}
	}
	return out
}

// GetActiveAttestationsForDomain returns only attestations
// that (a) are not revoked and (b) have ValidUntil > now.
func (r *DNSAttestationRegistry) GetActiveAttestationsForDomain(domain string, nowNs int64) []DNSAttestationTransaction {
	all := r.GetAttestationsForDomain(domain)
	out := make([]DNSAttestationTransaction, 0, len(all))
	for _, a := range all {
		if r.IsAttestationRevoked(a.ID) {
			continue
		}
		if a.ValidUntil > 0 && a.ValidUntil <= nowNs {
			continue
		}
		out = append(out, a)
	}
	return out
}

// IsAttestationRevoked returns true if any revocation has
// been recorded against the given attestation tx ID.
func (r *DNSAttestationRegistry) IsAttestationRevoked(attestationRef string) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.revocations[attestationRef]) > 0
}

// GetAttestation returns the attestation by tx ID or false.
func (r *DNSAttestationRegistry) GetAttestation(txID string) (DNSAttestationTransaction, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	a, ok := r.attestations[txID]
	return a, ok
}

// GetActiveDelegations returns all non-revoked delegations
// for a given attestation.
func (r *DNSAttestationRegistry) GetActiveDelegations(attestationRef string) []AuthorityDelegateTransaction {
	r.mu.RLock()
	defer r.mu.RUnlock()
	ids := r.delegationsByAttRef[attestationRef]
	out := make([]AuthorityDelegateTransaction, 0, len(ids))
	for _, id := range ids {
		d, ok := r.delegations[id]
		if !ok {
			continue
		}
		if len(r.delegationRevocations[id]) > 0 {
			continue
		}
		out = append(out, d)
	}
	return out
}

// --- Weighted attestation query (QDP-0023 §3.2 + §6) ---

// WeightedAttestation pairs an attestation with the trust
// weight the observer assigns to the attesting root. Higher
// weight = higher credibility.
type WeightedAttestation struct {
	Attestation DNSAttestationTransaction
	Weight      float64
}

// GetWeightedAttestationsForDomain returns live attestations
// for the given domain, each annotated with the trust weight
// the observer assigns to the attesting root. The weight is
// computed via ComputeRelationalTrustWithDecay against the
// "roots.dns.attestation" trust domain walk.
//
// The returned slice is sorted by weight descending — the
// highest-confidence attestation first.
//
// nowNs is the reference time for filtering expired or
// revoked records. Callers in production pass
// time.Now().UnixNano().
func (node *QuidnugNode) GetWeightedAttestationsForDomain(
	domain, observer string,
	maxDepth int,
	decay DecayConfig,
	nowNs int64,
) []WeightedAttestation {
	if node.DNSAttestationRegistry == nil {
		return nil
	}
	live := node.DNSAttestationRegistry.GetActiveAttestationsForDomain(domain, nowNs)
	out := make([]WeightedAttestation, 0, len(live))
	for _, a := range live {
		weight := 0.0
		if observer != "" && a.RootQuid != "" {
			// If observer == root, weight is trivially 1.0.
			if observer == a.RootQuid {
				weight = 1.0
			} else {
				w, _, err := node.ComputeRelationalTrustWithDecay(
					observer, a.RootQuid, maxDepth, decay)
				if err == nil {
					weight = w
				}
			}
		}
		out = append(out, WeightedAttestation{Attestation: a, Weight: weight})
	}
	// Sort descending by weight.
	for i := 0; i < len(out); i++ {
		for j := i + 1; j < len(out); j++ {
			if out[j].Weight > out[i].Weight {
				out[i], out[j] = out[j], out[i]
			}
		}
	}
	return out
}

// --- Admission wrappers on the node ---

// AddDNSClaimTransaction validates + stores a DNS_CLAIM tx.
func (node *QuidnugNode) AddDNSClaimTransaction(tx DNSClaimTransaction) (string, error) {
	if node.DNSAttestationRegistry == nil {
		return "", ErrTxTypeUnsupported("DNS_CLAIM: registry not initialized")
	}
	signed := tx.Signature != ""
	if !signed && tx.Timestamp == 0 {
		tx.Timestamp = time.Now().Unix()
	}
	if !signed {
		tx.Type = TxTypeDNSClaim
	}
	if !signed && tx.Nonce == 0 {
		tx.Nonce = node.DNSAttestationRegistry.currentNonce(tx.OwnerQuid) + 1
	}
	if !signed && tx.ID == "" {
		tx.ID = seedID(struct {
			Domain    string
			OwnerQuid string
			RootQuid  string
			Nonce     int64
			Timestamp int64
		}{tx.Domain, tx.OwnerQuid, tx.RootQuid, tx.Nonce, tx.Timestamp})
	}
	if !node.ValidateDNSClaim(tx) {
		return "", ErrInvalidTx("DNS_CLAIM")
	}
	node.DNSAttestationRegistry.admitClaim(tx)
	return tx.ID, nil
}

// AddDNSChallengeTransaction validates + stores a DNS_CHALLENGE tx.
func (node *QuidnugNode) AddDNSChallengeTransaction(tx DNSChallengeTransaction) (string, error) {
	if node.DNSAttestationRegistry == nil {
		return "", ErrTxTypeUnsupported("DNS_CHALLENGE: registry not initialized")
	}
	signed := tx.Signature != ""
	if !signed && tx.Timestamp == 0 {
		tx.Timestamp = time.Now().Unix()
	}
	if !signed {
		tx.Type = TxTypeDNSChallenge
	}
	if !signed && tx.ID == "" {
		tx.ID = seedID(struct {
			ClaimRef           string
			Nonce              string
			ChallengeExpiresAt int64
			TxNonce            int64
			Timestamp          int64
		}{tx.ClaimRef, tx.Nonce, tx.ChallengeExpiresAt, tx.TxNonce, tx.Timestamp})
	}
	if !node.ValidateDNSChallenge(tx) {
		return "", ErrInvalidTx("DNS_CHALLENGE")
	}
	node.DNSAttestationRegistry.admitChallenge(tx)
	return tx.ID, nil
}

// AddDNSAttestationTransaction validates + stores a DNS_ATTESTATION tx.
func (node *QuidnugNode) AddDNSAttestationTransaction(tx DNSAttestationTransaction) (string, error) {
	if node.DNSAttestationRegistry == nil {
		return "", ErrTxTypeUnsupported("DNS_ATTESTATION: registry not initialized")
	}
	signed := tx.Signature != ""
	if !signed && tx.Timestamp == 0 {
		tx.Timestamp = time.Now().Unix()
	}
	if !signed {
		tx.Type = TxTypeDNSAttestation
	}
	if !signed && tx.Nonce == 0 {
		tx.Nonce = node.DNSAttestationRegistry.currentNonce(tx.RootQuid) + 1
	}
	if !signed && tx.ID == "" {
		tx.ID = seedID(struct {
			Domain    string
			OwnerQuid string
			RootQuid  string
			Nonce     int64
			Timestamp int64
		}{tx.Domain, tx.OwnerQuid, tx.RootQuid, tx.Nonce, tx.Timestamp})
	}
	if !node.ValidateDNSAttestation(tx) {
		return "", ErrInvalidTx("DNS_ATTESTATION")
	}
	node.DNSAttestationRegistry.admitAttestation(tx)
	return tx.ID, nil
}

// AddDNSRenewalTransaction validates + stores a DNS_RENEWAL tx.
func (node *QuidnugNode) AddDNSRenewalTransaction(tx DNSRenewalTransaction) (string, error) {
	if node.DNSAttestationRegistry == nil {
		return "", ErrTxTypeUnsupported("DNS_RENEWAL: registry not initialized")
	}
	signed := tx.Signature != ""
	if !signed && tx.Timestamp == 0 {
		tx.Timestamp = time.Now().Unix()
	}
	if !signed {
		tx.Type = TxTypeDNSRenewal
	}
	if !signed && tx.ID == "" {
		tx.ID = seedID(struct {
			PriorRef      string
			NewValidUntil int64
			Nonce         int64
			Timestamp     int64
		}{tx.PriorAttestationRef, tx.NewValidUntil, tx.Nonce, tx.Timestamp})
	}
	if !node.ValidateDNSRenewal(tx) {
		return "", ErrInvalidTx("DNS_RENEWAL")
	}
	node.DNSAttestationRegistry.admitRenewal(tx)
	return tx.ID, nil
}

// AddDNSRevocationTransaction validates + stores a DNS_REVOCATION tx.
func (node *QuidnugNode) AddDNSRevocationTransaction(tx DNSRevocationTransaction) (string, error) {
	if node.DNSAttestationRegistry == nil {
		return "", ErrTxTypeUnsupported("DNS_REVOCATION: registry not initialized")
	}
	signed := tx.Signature != ""
	if !signed && tx.Timestamp == 0 {
		tx.Timestamp = time.Now().Unix()
	}
	if !signed {
		tx.Type = TxTypeDNSRevocation
	}
	if !signed && tx.ID == "" {
		tx.ID = seedID(struct {
			AttRef      string
			RevokerQuid string
			Nonce       int64
			Timestamp   int64
		}{tx.AttestationRef, tx.RevokerQuid, tx.Nonce, tx.Timestamp})
	}
	if !node.ValidateDNSRevocation(tx) {
		return "", ErrInvalidTx("DNS_REVOCATION")
	}
	node.DNSAttestationRegistry.admitRevocation(tx)
	return tx.ID, nil
}

// AddAuthorityDelegateTransaction validates + stores an
// AUTHORITY_DELEGATE tx.
func (node *QuidnugNode) AddAuthorityDelegateTransaction(tx AuthorityDelegateTransaction) (string, error) {
	if node.DNSAttestationRegistry == nil {
		return "", ErrTxTypeUnsupported("AUTHORITY_DELEGATE: registry not initialized")
	}
	signed := tx.Signature != ""
	if !signed && tx.Timestamp == 0 {
		tx.Timestamp = time.Now().Unix()
	}
	if !signed {
		tx.Type = TxTypeAuthorityDelegate
	}
	if !signed && tx.ID == "" {
		tx.ID = seedID(struct {
			AttestationRef string
			Subject        string
			DelegateDomain string
			Nonce          int64
			Timestamp      int64
		}{tx.AttestationRef, tx.Subject, tx.DelegateDomain, tx.Nonce, tx.Timestamp})
	}
	if !node.ValidateAuthorityDelegate(tx) {
		return "", ErrInvalidTx("AUTHORITY_DELEGATE")
	}
	node.DNSAttestationRegistry.admitDelegate(tx)
	return tx.ID, nil
}

// AddAuthorityDelegateRevocationTransaction validates +
// stores an AUTHORITY_DELEGATE_REVOCATION tx.
func (node *QuidnugNode) AddAuthorityDelegateRevocationTransaction(
	tx AuthorityDelegateRevocationTransaction,
) (string, error) {
	if node.DNSAttestationRegistry == nil {
		return "", ErrTxTypeUnsupported("AUTHORITY_DELEGATE_REVOCATION: registry not initialized")
	}
	signed := tx.Signature != ""
	if !signed && tx.Timestamp == 0 {
		tx.Timestamp = time.Now().Unix()
	}
	if !signed {
		tx.Type = TxTypeAuthorityDelegateRevocation
	}
	if !signed && tx.ID == "" {
		tx.ID = seedID(struct {
			DelegationRef string
			RevokerQuid   string
			Nonce         int64
			Timestamp     int64
		}{tx.DelegationRef, tx.RevokerQuid, tx.Nonce, tx.Timestamp})
	}
	node.DNSAttestationRegistry.admitDelegateRevocation(tx)
	return tx.ID, nil
}
