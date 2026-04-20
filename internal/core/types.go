package core

// Core transaction types
type TransactionType string

const (
	TxTypeTrust    TransactionType = "TRUST"
	TxTypeIdentity TransactionType = "IDENTITY"
	TxTypeTitle    TransactionType = "TITLE"
	TxTypeEvent    TransactionType = "EVENT"
	TxTypeGeneric  TransactionType = "GENERIC"
	// TxTypeAnchor wraps a NonceAnchor inside the block's transaction
	// list (QDP-0001 §6.5). Anchors are not "transactions" in the
	// transfer-of-value sense — the wrapper just lets them share the
	// block-inclusion machinery.
	TxTypeAnchor TransactionType = "ANCHOR"
	// TxTypeNodeAdvertisement is a signed declaration by a node quid
	// of where it is reachable, which domains it serves, and what
	// capabilities it offers (QDP-0014). Clients use advertisements
	// for discovery + sharding.
	TxTypeNodeAdvertisement TransactionType = "NODE_ADVERTISEMENT"
	// TxTypeModerationAction is a signed on-chain takedown /
	// suppression record (QDP-0015). The chain remains append-
	// only; serving layers consult the moderation registry at
	// read time to honor DMCA / court orders / GDPR erasure /
	// CSAM takedowns / operator policy.
	TxTypeModerationAction TransactionType = "MODERATION_ACTION"
	// QDP-0017 data-subject-rights transactions.
	//
	// TxTypeDataSubjectRequest is a signed request from a
	// subject for access / erasure / rectification / etc. under
	// GDPR / CCPA / LGPD / PIPEDA.
	TxTypeDataSubjectRequest TransactionType = "DATA_SUBJECT_REQUEST"
	// TxTypeConsentGrant is a signed opt-in by a subject to
	// specific processing scopes against a specific policy
	// version.
	TxTypeConsentGrant TransactionType = "CONSENT_GRANT"
	// TxTypeConsentWithdraw revokes a prior CONSENT_GRANT.
	// Honored immediately by the serving layer.
	TxTypeConsentWithdraw TransactionType = "CONSENT_WITHDRAW"
	// TxTypeProcessingRestriction restricts specific processing
	// uses (reputation computation, recommendations, etc.)
	// without deleting underlying data.
	TxTypeProcessingRestriction TransactionType = "PROCESSING_RESTRICTION"
	// TxTypeDSRCompliance is an operator-signed proof that a
	// DSR request was fulfilled, including the carve-out
	// rationale and completion timestamp.
	TxTypeDSRCompliance TransactionType = "DSR_COMPLIANCE"
	// QDP-0023 DNS-anchored identity attestation types.
	//
	// TxTypeDNSClaim declares intent to attest a DNS domain to
	// a quid. Emitted by the domain owner to a chosen
	// attestation root.
	TxTypeDNSClaim TransactionType = "DNS_CLAIM"
	// TxTypeDNSChallenge is the root's response to a CLAIM
	// carrying a nonce the owner must publish in DNS + a
	// well-known file.
	TxTypeDNSChallenge TransactionType = "DNS_CHALLENGE"
	// TxTypeDNSAttestation is the root's signed binding from
	// DNS name to quid after successful verification.
	TxTypeDNSAttestation TransactionType = "DNS_ATTESTATION"
	// TxTypeDNSRenewal re-verifies + extends an existing
	// attestation before its ValidUntil.
	TxTypeDNSRenewal TransactionType = "DNS_RENEWAL"
	// TxTypeDNSRevocation revokes an attestation (by owner,
	// attesting root, or root governor quorum).
	TxTypeDNSRevocation TransactionType = "DNS_REVOCATION"
	// TxTypeAuthorityDelegate is the generic authority-
	// delegation primitive (not DNS-specific). Lets the
	// subject of any attestation delegate resolution to
	// specific nodes/domains with per-record-type visibility.
	TxTypeAuthorityDelegate TransactionType = "AUTHORITY_DELEGATE"
	// TxTypeAuthorityDelegateRevocation revokes a prior
	// AUTHORITY_DELEGATE.
	TxTypeAuthorityDelegateRevocation TransactionType = "AUTHORITY_DELEGATE_REVOCATION"
)

// Trust computation resource limits
const (
	DefaultTrustMaxDepth = 5
	MaxTrustQueueSize    = 10000
	MaxTrustVisitedSize  = 10000
)

// TrustCacheEntry holds a cached trust computation result
type TrustCacheEntry struct {
	TrustLevel float64
	TrustPath  []string
	ExpiresAt  int64 // UnixNano timestamp for expiration; nanosecond precision is required so sub-second TTLs work
}

// EnhancedTrustCacheEntry holds a cached enhanced trust computation result
type EnhancedTrustCacheEntry struct {
	Result    EnhancedTrustResult
	ExpiresAt int64 // UnixNano timestamp for expiration
}

// Base Transaction represents common fields for all transaction types
type BaseTransaction struct {
	ID          string          `json:"id"`
	Type        TransactionType `json:"type"`
	TrustDomain string          `json:"trustDomain"`
	Timestamp   int64           `json:"timestamp"`
	Signature   string          `json:"signature"`
	PublicKey   string          `json:"publicKey"`
}

// TrustTransaction establishes trust between entities with a specific trust level
type TrustTransaction struct {
	BaseTransaction
	Truster     string  `json:"truster"`
	Trustee     string  `json:"trustee"`
	TrustLevel  float64 `json:"trustLevel"`
	Nonce       int64   `json:"nonce"`
	Description string  `json:"description,omitempty"`
	ValidUntil  int64   `json:"validUntil,omitempty"`
}

// IdentityTransaction declares or defines a quid in the system
type IdentityTransaction struct {
	BaseTransaction
	QuidID      string                 `json:"quidId"`
	Name        string                 `json:"name"`
	Description string                 `json:"description,omitempty"`
	Attributes  map[string]interface{} `json:"attributes,omitempty"`
	Creator     string                 `json:"creator"`
	UpdateNonce int64                  `json:"updateNonce"`

	// HomeDomain (QDP-0007 / H4): the trust domain where this
	// signer authoritatively rotates their key. When set, other
	// domains probe this domain for epoch refreshes on stale
	// transactions. Empty falls back to the receiving node's
	// primary domain. Optional for backward compatibility with
	// identity records written before H4.
	HomeDomain string `json:"homeDomain,omitempty"`
}

// OwnershipStake represents a single ownership claim
type OwnershipStake struct {
	OwnerID    string  `json:"ownerId"`
	Percentage float64 `json:"percentage"`
	StakeType  string  `json:"stakeType,omitempty"`
}

// TitleTransaction defines ownership relationships between quids
type TitleTransaction struct {
	BaseTransaction
	AssetID        string            `json:"assetId"`
	Owners         []OwnershipStake  `json:"owners"`
	PreviousOwners []OwnershipStake  `json:"previousOwners,omitempty"`
	Signatures     map[string]string `json:"signatures"`
	ExpiryDate     int64             `json:"expiryDate,omitempty"`
	TitleType      string            `json:"titleType,omitempty"`
}

// EventTransaction represents an event in an append-only stream for a quid or title
type EventTransaction struct {
	BaseTransaction
	SubjectID       string                 `json:"subjectId"`
	SubjectType     string                 `json:"subjectType"`
	Sequence        int64                  `json:"sequence"`
	EventType       string                 `json:"eventType"`
	Payload         map[string]interface{} `json:"payload,omitempty"`
	PayloadCID      string                 `json:"payloadCid,omitempty"`
	PreviousEventID string                 `json:"previousEventId,omitempty"`
}

// NodeEndpoint is one reachable URL for a node, with routing hints.
// Clients prefer the lowest-priority endpoint whose capabilities match
// their query; ties broken by weight (higher weight = preferred).
type NodeEndpoint struct {
	URL      string `json:"url"`                // MUST be https://
	Protocol string `json:"protocol,omitempty"` // "http/1.1" | "http/2" | "http/3" | "grpc"
	Region   string `json:"region,omitempty"`   // free-form; suggested "iad" | "lhr" | "sin"
	Priority int    `json:"priority"`           // 0..100; lower = preferred
	Weight   int    `json:"weight"`             // 0..10000; equal-priority round-robin
}

// NodeCapabilities describes what a node is willing to do for clients.
// Validator == true is only honored if the node is also in some
// domain's Validators map (cross-check happens at advertisement-ingest).
type NodeCapabilities struct {
	Validator       bool   `json:"validator,omitempty"`
	Cache           bool   `json:"cache,omitempty"`
	Archive         bool   `json:"archive,omitempty"`
	Bootstrap       bool   `json:"bootstrap,omitempty"`
	GossipSink      bool   `json:"gossipSink,omitempty"`
	IPFSGateway     bool   `json:"ipfsGateway,omitempty"`
	MaxBodyBytes    int    `json:"maxBodyBytes,omitempty"`
	MinPeerProtocol string `json:"minPeerProtocol,omitempty"`
}

// NodeAdvertisementTransaction is the QDP-0014 signed record a node
// publishes to declare its endpoints, supported domains, capabilities,
// and expiration. The signer is the node itself (its BaseTransaction.
// PublicKey derives the node quid, which must equal NodeQuid).
//
// Operator attestation: there MUST be a current TRUST edge from
// OperatorQuid → NodeQuid at weight ≥ 0.5 in a domain of the form
// operators.network.<operator-domain> for an advertisement to be
// honored. See QDP-0014 §4.1.
type NodeAdvertisementTransaction struct {
	BaseTransaction
	NodeQuid           string           `json:"nodeQuid"`
	OperatorQuid       string           `json:"operatorQuid"`
	Endpoints          []NodeEndpoint   `json:"endpoints"`
	SupportedDomains   []string         `json:"supportedDomains,omitempty"`
	Capabilities       NodeCapabilities `json:"capabilities"`
	ProtocolVersion    string           `json:"protocolVersion"`
	ExpiresAt          int64            `json:"expiresAt"`          // UnixNano; rejected if >7d out
	AdvertisementNonce int64            `json:"advertisementNonce"` // monotonic per NodeQuid
}

// ---------------------------------------------------------------
// QDP-0023: DNS-anchored identity attestation (Phase 1)
// ---------------------------------------------------------------

// DNSClaimTransaction — owner declares intent to attest a DNS
// domain to their quid. Emitted on the root's
// attestation.claims.<root-quid> domain.
type DNSClaimTransaction struct {
	BaseTransaction
	Domain              string `json:"domain"`
	OwnerQuid           string `json:"ownerQuid"`
	RootQuid            string `json:"rootQuid"`
	RequestedValidUntil int64  `json:"requestedValidUntil,omitempty"` // Unix ns
	PaymentMethod       string `json:"paymentMethod,omitempty"`       // "stripe" | "crypto" | "waiver"
	PaymentReference    string `json:"paymentReference,omitempty"`    // off-chain receipt id
	ContactEmail        string `json:"contactEmail,omitempty"`
	Nonce               int64  `json:"nonce"`
}

// DNSChallengeTransaction — root's challenge back to the
// claimant. Carries the nonce the owner must publish in DNS
// + the well-known URL template.
type DNSChallengeTransaction struct {
	BaseTransaction
	ClaimRef                string `json:"claimRef"`
	Nonce                   string `json:"nonce"`            // 32-byte hex
	ChallengeExpiresAt      int64  `json:"challengeExpiresAt"` // Unix ns
	TXTRecordName           string `json:"txtRecordName"`
	WellKnownURL            string `json:"wellKnownURL"`
	RequiredContentTemplate string `json:"requiredContentTemplate,omitempty"`
	TxNonce                 int64  `json:"txNonce"`
}

// ResolverResult — per-resolver DNS TXT observation captured
// during verification. Embedded into DNS_ATTESTATION for
// audit trail.
type ResolverResult struct {
	ResolverLabel string `json:"resolverLabel"`
	TXTValue      string `json:"txtValue"`
	ObservedAt    int64  `json:"observedAt"`
}

// DNSAttestationTransaction — the signed binding. Emitted by
// the root after passing all verification checks.
type DNSAttestationTransaction struct {
	BaseTransaction
	ClaimRef             string           `json:"claimRef"`
	Domain               string           `json:"domain"`
	OwnerQuid            string           `json:"ownerQuid"`
	RootQuid             string           `json:"rootQuid"`
	TLD                  string           `json:"tld"`
	TLDTier              string           `json:"tldTier"`     // "free-public" | "standard" | "premium" | "luxury"
	VerifiedAt           int64            `json:"verifiedAt"`  // Unix ns
	ValidUntil           int64            `json:"validUntil"`  // Unix ns
	TLSFingerprintSHA256 string           `json:"tlsFingerprintSha256,omitempty"`
	WHOISRegisteredSince int64            `json:"whoisRegisteredSince,omitempty"` // Unix seconds
	BlocklistCheckedAt   int64            `json:"blocklistCheckedAt,omitempty"`
	BlocklistsChecked    []string         `json:"blocklistsChecked,omitempty"`
	VerifierNodes        []string         `json:"verifierNodes,omitempty"`
	ResolverConsensus    []ResolverResult `json:"resolverConsensus,omitempty"`
	FeePaidUSD           float64          `json:"feePaidUsd,omitempty"`
	PaymentMethod        string           `json:"paymentMethod,omitempty"`
	PaymentReference     string           `json:"paymentReference,omitempty"`
	RigorLevel           string           `json:"rigorLevel,omitempty"` // "basic" | "standard" | "rigorous"
	Nonce                int64            `json:"nonce"`
}

// DNSRenewalTransaction — re-verifies an existing attestation
// before expiry. Submitted within the renewal window
// (default: 30 days before ValidUntil).
type DNSRenewalTransaction struct {
	BaseTransaction
	PriorAttestationRef      string  `json:"priorAttestationRef"`
	NewValidUntil            int64   `json:"newValidUntil"`
	NewTLSFingerprintSHA256  string  `json:"newTlsFingerprintSha256,omitempty"`
	FingerprintRotationProof string  `json:"fingerprintRotationProof,omitempty"`
	FeePaidUSD               float64 `json:"feePaidUsd,omitempty"`
	PaymentReference         string  `json:"paymentReference,omitempty"`
	Nonce                    int64   `json:"nonce"`
}

// DNSRevocationTransaction — revokes an attestation. Signable
// by the attesting root, governor quorum on the root's
// governance domain, or the domain owner.
type DNSRevocationTransaction struct {
	BaseTransaction
	AttestationRef     string   `json:"attestationRef"`
	RevokerQuid        string   `json:"revokerQuid"`
	RevokerRole        string   `json:"revokerRole"` // "root" | "governor-quorum" | "owner"
	Reason             string   `json:"reason"`      // "fraud-detected" | "owner-request" | "transfer" | "malfeasance" | ...
	RevokedAt          int64    `json:"revokedAt"`   // Unix ns
	GovernorSignatures []string `json:"governorSignatures,omitempty"`
	Nonce              int64    `json:"nonce"`
}

// ---------------------------------------------------------------
// QDP-0023 generic authority delegation
// ---------------------------------------------------------------

// VisibilityPolicy describes how records of specific types
// should be served: public cache, trust-gated, or private
// (QDP-0024 encrypted).
type VisibilityPolicy struct {
	Class      string  `json:"class"` // "public" | "trust-gated" | "private"
	GateDomain string  `json:"gateDomain,omitempty"`
	MinTrust   float64 `json:"minTrust,omitempty"`
	GroupID    string  `json:"groupId,omitempty"`
	Encryption string  `json:"encryption,omitempty"` // e.g. "mls-x25519-aes256gcm"
}

// DelegationVisibility is the record-type -> policy map plus
// a fallback default.
type DelegationVisibility struct {
	RecordTypes map[string]VisibilityPolicy `json:"recordTypes"`
	Default     VisibilityPolicy            `json:"default"`
}

// AuthorityDelegateTransaction hands resolution authority for
// a previously-attested subject back to specific nodes/domain
// with per-record-type visibility policy.
type AuthorityDelegateTransaction struct {
	BaseTransaction
	AttestationRef  string               `json:"attestationRef"`
	AttestationKind string               `json:"attestationKind"` // "dns" | "review" | "credential" | ...
	Subject         string               `json:"subject"`         // e.g. "chase.com"
	DelegateNodes   []string             `json:"delegateNodes"`   // quids
	DelegateDomain  string               `json:"delegateDomain"`
	Visibility      DelegationVisibility `json:"visibility"`
	FallbackPublic  bool                 `json:"fallbackPublic,omitempty"`
	EffectiveAt     int64                `json:"effectiveAt,omitempty"` // Unix ns
	ValidUntil      int64                `json:"validUntil,omitempty"`  // Unix ns
	Nonce           int64                `json:"nonce"`
}

// AuthorityDelegateRevocationTransaction revokes a prior
// delegation. Signable by the attestation owner or (for
// abuse cases) the attesting root.
type AuthorityDelegateRevocationTransaction struct {
	BaseTransaction
	DelegationRef string `json:"delegationRef"`
	RevokerQuid   string `json:"revokerQuid"`
	RevokerRole   string `json:"revokerRole"` // "owner" | "root"
	Reason        string `json:"reason,omitempty"`
	RevokedAt     int64  `json:"revokedAt"`
	Nonce         int64  `json:"nonce"`
}

// AnchorTransaction carries a NonceAnchor through the block transaction
// envelope. Signature-wise the anchor is self-contained: the Signature
// inside the embedded NonceAnchor is what matters, not the envelope's
// BaseTransaction.Signature (which should remain empty for anchors).
type AnchorTransaction struct {
	BaseTransaction
	Anchor NonceAnchor `json:"anchor"`
}

// EventStream tracks the state of an event stream for a subject
type EventStream struct {
	SubjectID      string `json:"subjectId"`
	SubjectType    string `json:"subjectType"`
	LatestSequence int64  `json:"latestSequence"`
	EventCount     int64  `json:"eventCount"`
	CreatedAt      int64  `json:"createdAt"`
	UpdatedAt      int64  `json:"updatedAt"`
	LatestEventID  string `json:"latestEventId"`
}

// GlobalEventOrder provides a total ordering for events across the blockchain
type GlobalEventOrder struct {
	BlockIndex int64 `json:"blockIndex"`
	TxIndex    int   `json:"txIndex"`
	Sequence   int64 `json:"sequence"`
}

// Block represents a block in the blockchain.
//
// NonceCheckpoints is the per-(signer, domain, epoch) max-nonce
// summary introduced by QDP-0001 §6.1.3. The field is serialized with
// `omitempty` so pre-QDP-0001 blocks continue to round-trip. Block
// producers populate it at seal time via computeNonceCheckpoints; the
// signable data (see GetBlockSignableData) includes it only when
// non-empty, preserving signatures on legacy blocks.
type Block struct {
	Index            int64             `json:"index"`
	Timestamp        int64             `json:"timestamp"`
	Transactions     []interface{}     `json:"transactions"`
	TrustProof       TrustProof        `json:"trustProof"`
	PrevHash         string            `json:"prevHash"`
	Hash             string            `json:"hash"`
	NonceCheckpoints []NonceCheckpoint `json:"nonceCheckpoints,omitempty"`

	// TransactionsRoot (QDP-0010 / H2): root of a binary Merkle
	// tree over canonical transaction bytes. Empty when the
	// block was produced under pre-H2 code (omitempty keeps the
	// wire format backward-compatible). Receivers that observe
	// the `require_tx_tree_root` fork activation (QDP-0009)
	// reject blocks with empty root; pre-activation receivers
	// ignore the field.
	TransactionsRoot string `json:"transactionsRoot,omitempty"`
}

// TrustProof implements the proof of trust system
type TrustProof struct {
	TrustDomain             string                 `json:"trustDomain"`
	ValidatorID             string                 `json:"validatorId"`
	ValidatorPublicKey      string                 `json:"validatorPublicKey,omitempty"`
	ValidatorTrustInCreator float64                `json:"validatorTrustInCreator"`
	ValidatorSigs           []string               `json:"validatorSigs"`
	ConsensusData           map[string]interface{} `json:"consensusData,omitempty"`
	ValidationTime          int64                  `json:"validationTime"`
}

// Node represents a quidnug node in the network
type Node struct {
	ID               string   `json:"id"`
	Address          string   `json:"address"`
	TrustDomains     []string `json:"trustDomains"`
	IsValidator      bool     `json:"isValidator"`
	LastSeen         int64    `json:"lastSeen"`
	ConnectionStatus string   `json:"connectionStatus"`
}

// TrustDomain represents a domain that this node manages or interacts with
type TrustDomain struct {
	Name           string   `json:"name"`
	ValidatorNodes []string `json:"validatorNodes"`
	TrustThreshold float64  `json:"trustThreshold"`
	BlockchainHead string   `json:"blockchainHead"`
	// Validators maps validator node IDs to their participation weight (0.0-1.0).
	// This represents voting power in consensus, not an absolute trust score.
	Validators map[string]float64 `json:"validators"`
	// ValidatorPublicKeys maps validator node IDs to their hex-encoded public keys.
	// Used to cryptographically verify block signatures from domain validators.
	ValidatorPublicKeys map[string]string `json:"validatorPublicKeys"`

	// QDP-0012 Phase 1 additions — governance state. All fields
	// are `omitempty` so serialized TrustDomains from pre-QDP-0012
	// nodes round-trip unchanged and unmarshaling on older nodes
	// silently drops the extra keys.
	//
	// Phase 1 populates these at registration time with a single-
	// governor fallback (registrant becomes sole governor, quorum
	// 1.0). Phase 2 introduces the DOMAIN_GOVERNANCE transaction
	// that mutates them under governor-quorum signature. Phase 3
	// wires enforcement via the QDP-0009 fork-block activation
	// flag.
	//
	// See docs/design/0012-domain-governance.md §3.2.

	// Governors maps each governor's quid to their vote weight.
	// Nil / empty on domains that predate Phase 1.
	Governors map[string]float64 `json:"governors,omitempty"`
	// GovernorPublicKeys maps each governor's quid to their hex-
	// encoded public key, so DOMAIN_GOVERNANCE transaction
	// signatures can be verified without re-looking-up identity
	// registry entries.
	GovernorPublicKeys map[string]string `json:"governorPublicKeys,omitempty"`
	// GovernanceQuorum is the fraction of total governor vote
	// weight required for a governance action to activate, e.g.
	// 0.67 for 2/3. Zero means no quorum policy set (Phase 1
	// fallback — quorum is treated as "all governors unanimous"
	// which, with the single-registrant default, is trivially
	// met).
	GovernanceQuorum float64 `json:"governanceQuorum,omitempty"`
	// GovernanceNonce is the highest nonce of any applied
	// DOMAIN_GOVERNANCE transaction for this domain. Prevents
	// replay of earlier governance actions.
	GovernanceNonce int64 `json:"governanceNonce,omitempty"`
	// ParentDelegationMode describes how this domain's
	// governance relates to its parent domain:
	//   "self"      = this domain manages its own Governors
	//   "inherit"   = governance flows from the parent domain
	//   "delegated" = governance was explicitly handed to
	//                 another operator via DELEGATE_CHILD
	// Empty string is treated as "self" for backward compat.
	ParentDelegationMode string `json:"parentDelegationMode,omitempty"`
	// DelegatedFrom names the parent domain that delegated
	// governance authority here. Empty unless
	// ParentDelegationMode == "delegated" or "inherit".
	DelegatedFrom string `json:"delegatedFrom,omitempty"`
}

// Governance role constants for QDP-0012.
const (
	// DomainRoleGovernor is returned for quids in Governors.
	DomainRoleGovernor = "governor"
	// DomainRoleConsortiumMember is returned for quids in
	// Validators with non-zero weight.
	DomainRoleConsortiumMember = "consortium-member"
	// DomainRoleCacheReplica is the default for any quid the
	// domain has no record of. Matches the relativistic stance
	// of the protocol: a node can hold a mirror of a domain's
	// chain without being explicitly admitted to its consortium.
	DomainRoleCacheReplica = "cache-replica"
)

// Parent-delegation mode constants for QDP-0012.
const (
	DelegationModeSelf      = "self"
	DelegationModeInherit   = "inherit"
	DelegationModeDelegated = "delegated"
)

// IsGovernor reports whether quid carries governor authority
// on this domain. A quid with weight <= 0 is not a governor
// even if present in the map (defensive against future code
// that zero-weights rather than deletes).
func (td *TrustDomain) IsGovernor(quid string) bool {
	if td == nil || td.Governors == nil {
		return false
	}
	w, ok := td.Governors[quid]
	return ok && w > 0
}

// IsConsortiumMember reports whether quid produces admissible
// blocks for this domain. Mirrors the existing check that
// happened inline in block-acceptance paths; promoted to a
// named method so the role model is explicit.
func (td *TrustDomain) IsConsortiumMember(quid string) bool {
	if td == nil || td.Validators == nil {
		return false
	}
	w, ok := td.Validators[quid]
	return ok && w > 0
}

// Role returns one of DomainRoleGovernor /
// DomainRoleConsortiumMember / DomainRoleCacheReplica. A quid
// that is both a governor and a consortium member reports as
// DomainRoleGovernor (the higher-privilege role).
func (td *TrustDomain) Role(quid string) string {
	switch {
	case td.IsGovernor(quid):
		return DomainRoleGovernor
	case td.IsConsortiumMember(quid):
		return DomainRoleConsortiumMember
	default:
		return DomainRoleCacheReplica
	}
}

// GovernorQuorumWeight returns the minimum signed-weight sum
// required for a governance action to meet quorum on this
// domain. Zero quorum is treated as "require the full weight
// of all governors" (the pre-QDP-0012 sole-registrant default
// — trivially met, because the registrant IS the full weight).
func (td *TrustDomain) GovernorQuorumWeight() float64 {
	if td == nil || len(td.Governors) == 0 {
		return 0
	}
	var total float64
	for _, w := range td.Governors {
		total += w
	}
	q := td.GovernanceQuorum
	if q <= 0 {
		q = 1.0
	}
	return total * q
}

// RelationalTrustQuery represents a query for trust between two quids
type RelationalTrustQuery struct {
	Observer          string `json:"observer"`
	Target            string `json:"target"`
	Domain            string `json:"domain,omitempty"`
	MaxDepth          int    `json:"maxDepth,omitempty"`
	IncludeUnverified bool   `json:"includeUnverified,omitempty"`
}

// RelationalTrustResult represents the result of a relational trust query
type RelationalTrustResult struct {
	Observer   string   `json:"observer"`
	Target     string   `json:"target"`
	TrustLevel float64  `json:"trustLevel"`
	TrustPath  []string `json:"trustPath,omitempty"`
	PathDepth  int      `json:"pathDepth"`
	Domain     string   `json:"domain,omitempty"`
}

// BlockAcceptance represents the tiered acceptance level of a block
type BlockAcceptance int

const (
	BlockTrusted   BlockAcceptance = iota // Integrate into main chain
	BlockTentative                        // Store separately, don't build on
	BlockUntrusted                        // Extract data, relay, don't store block
	BlockInvalid                          // Reject entirely (cryptographically invalid)
)

// TrustEdge represents a trust relationship with provenance tracking
type TrustEdge struct {
	Truster       string  `json:"truster"`
	Trustee       string  `json:"trustee"`
	TrustLevel    float64 `json:"trustLevel"`
	SourceBlock   string  `json:"sourceBlock"`   // Block hash where this edge was recorded
	ValidatorQuid string  `json:"validatorQuid"` // Quid of validator who signed the block
	Verified      bool    `json:"verified"`      // True if from a trusted validator
	Timestamp     int64   `json:"timestamp"`
}

// EnhancedTrustResult extends RelationalTrustResult with provenance
type EnhancedTrustResult struct {
	RelationalTrustResult
	Confidence       string            `json:"confidence"` // "high", "medium", "low"
	UnverifiedHops   int               `json:"unverifiedHops"`
	VerificationGaps []VerificationGap `json:"verificationGaps"`
}

// VerificationGap describes an unverified hop in the trust path
type VerificationGap struct {
	From           string  `json:"from"`
	To             string  `json:"to"`
	ValidatorQuid  string  `json:"validatorQuid"`
	ValidatorTrust float64 `json:"validatorTrust"`
}

// DomainGossip represents a gossip message about domain availability
type DomainGossip struct {
	NodeID    string   `json:"nodeId"`
	Domains   []string `json:"domains"`
	Timestamp int64    `json:"timestamp"`
	TTL       int      `json:"ttl"`       // Remaining hops before this gossip is dropped
	HopCount  int      `json:"hopCount"`  // Number of hops this message has traveled
	MessageID string   `json:"messageId"` // Unique ID to prevent duplicate processing
}
