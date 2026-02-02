package main

// Core transaction types
type TransactionType string

const (
	TxTypeTrust    TransactionType = "TRUST"
	TxTypeIdentity TransactionType = "IDENTITY"
	TxTypeTitle    TransactionType = "TITLE"
	TxTypeGeneric  TransactionType = "GENERIC"
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
	TrustDomain    string                 `json:"trustDomain"`
	ValidatorID    string                 `json:"validatorId"`
	TrustScore     float64                `json:"trustScore"`
	ValidatorSigs  []string               `json:"validatorSigs"`
	ConsensusData  map[string]interface{} `json:"consensusData,omitempty"`
	ValidationTime int64                  `json:"validationTime"`
}

// Node represents a quidnug node in the network
type Node struct {
	ID               string   `json:"id"`
	Address          string   `json:"address"`
	TrustDomains     []string `json:"trustDomains"`
	IsValidator      bool     `json:"isValidator"`
	TrustScore       float64  `json:"trustScore"`
	LastSeen         int64    `json:"lastSeen"`
	ConnectionStatus string   `json:"connectionStatus"`
}

// TrustDomain represents a domain that this node manages or interacts with
type TrustDomain struct {
	Name           string             `json:"name"`
	ValidatorNodes []string           `json:"validatorNodes"`
	TrustThreshold float64            `json:"trustThreshold"`
	BlockchainHead string             `json:"blockchainHead"`
	Validators     map[string]float64 `json:"validators"`
}
