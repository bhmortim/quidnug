package main

// Core transaction types
type TransactionType string

const (
	TxTypeTrust    TransactionType = "TRUST"
	TxTypeIdentity TransactionType = "IDENTITY"
	TxTypeTitle    TransactionType = "TITLE"
	TxTypeGeneric  TransactionType = "GENERIC"
)

// Trust computation resource limits
const (
	DefaultTrustMaxDepth  = 5
	MaxTrustQueueSize     = 10000
	MaxTrustVisitedSize   = 10000
)

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

// Block represents a block in the blockchain
type Block struct {
	Index        int64         `json:"index"`
	Timestamp    int64         `json:"timestamp"`
	Transactions []interface{} `json:"transactions"`
	TrustProof   TrustProof    `json:"trustProof"`
	PrevHash     string        `json:"prevHash"`
	Hash         string        `json:"hash"`
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
	VerificationGaps []VerificationGap `json:"verificationGaps,omitempty"`
}

// VerificationGap describes an unverified hop in the trust path
type VerificationGap struct {
	From           string  `json:"from"`
	To             string  `json:"to"`
	ValidatorQuid  string  `json:"validatorQuid"`
	ValidatorTrust float64 `json:"validatorTrust"`
}
