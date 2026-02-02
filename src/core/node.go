package main

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"math/big"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/mux"
)

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
	ID          string         `json:"id"`
	Type        TransactionType `json:"type"`
	TrustDomain string         `json:"trustDomain"`
	Timestamp   int64          `json:"timestamp"`
	Signature   string         `json:"signature"`
	PublicKey   string         `json:"publicKey"`
}

func main() {
	// Initialize node
	quidnugNode, err := NewQuidnugNode()
	if err != nil {
		log.Fatalf("Failed to initialize quidnug node: %v", err)
	}

	// Get port from environment or use default
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	// Get seed nodes from environment or use defaults
	var seedNodes []string
	seedNodesEnv := os.Getenv("SEED_NODES")
	if seedNodesEnv != "" {
		json.Unmarshal([]byte(seedNodesEnv), &seedNodes)
	} else {
		// Default seed nodes
		seedNodes = []string{"seed1.quidnug.net:8080", "seed2.quidnug.net:8080"}
	}

	// Discover other nodes
	go quidnugNode.DiscoverNodes(seedNodes)

	// Start block generation for managed trust domains
	go func() {
		for {
			time.Sleep(60 * time.Second) // Generate block every minute
			
			quidnugNode.TrustDomainsMutex.RLock()
			managedDomains := make([]string, 0, len(quidnugNode.TrustDomains))
			for domain := range quidnugNode.TrustDomains {
				managedDomains = append(managedDomains, domain)
			}
			quidnugNode.TrustDomainsMutex.RUnlock()
			
			for _, domain := range managedDomains {
				block, err := quidnugNode.GenerateBlock(domain)
				if err != nil {
					log.Printf("Failed to generate block for domain %s: %v", domain, err)
					continue
				}
				
				if err := quidnugNode.AddBlock(*block); err != nil {
					log.Printf("Failed to add generated block: %v", err)
				}
			}
		}
	}()

	// Start HTTP server
	log.Fatal(quidnugNode.StartServer(port))
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
	AssetID        string           `json:"assetId"`
	Owners         []OwnershipStake `json:"owners"`
	PreviousOwners []OwnershipStake `json:"previousOwners,omitempty"`
	Signatures     map[string]string `json:"signatures"` // Signatures from previous owners
	ExpiryDate     int64            `json:"expiryDate,omitempty"`
	TitleType      string           `json:"titleType,omitempty"`
}

// Block represents a block in the blockchain
type Block struct {
	Index        int64           `json:"index"`
	Timestamp    int64           `json:"timestamp"`
	Transactions []interface{}   `json:"transactions"` // Can contain any transaction type
	TrustProof   TrustProof      `json:"trustProof"`
	PrevHash     string          `json:"prevHash"`
	Hash         string          `json:"hash"`
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

// QuidnugNode is the main server structure
type QuidnugNode struct {
	NodeID            string
	PrivateKey        *ecdsa.PrivateKey
	PublicKey         *ecdsa.PublicKey
	Blockchain        []Block
	PendingTxs        []interface{} // Can contain any transaction type
	TrustDomains      map[string]TrustDomain
	KnownNodes        map[string]Node
	
	// State registries
	TrustRegistry     map[string]map[string]float64 // Maps truster -> (trustee -> trust level)
	IdentityRegistry  map[string]IdentityTransaction // Maps quidID -> identity
	TitleRegistry     map[string]TitleTransaction // Maps assetID -> current title
	
	// Mutexes for thread safety
	BlockchainMutex   sync.RWMutex
	PendingTxsMutex   sync.RWMutex
	KnownNodesMutex   sync.RWMutex
	TrustDomainsMutex sync.RWMutex
	TrustRegistryMutex sync.RWMutex
	IdentityRegistryMutex sync.RWMutex
	TitleRegistryMutex sync.RWMutex
}

// TrustDomain represents a domain that this node manages or interacts with
type TrustDomain struct {
	Name           string             `json:"name"`
	ValidatorNodes []string           `json:"validatorNodes"`
	TrustThreshold float64            `json:"trustThreshold"`
	BlockchainHead string             `json:"blockchainHead"`
	Validators     map[string]float64 `json:"validators"`
}

// Initialize a new quidnug node
func NewQuidnugNode() (*QuidnugNode, error) {
	// Generate a new ECDSA key pair
	privateKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, fmt.Errorf("failed to generate key pair: %v", err)
	}

	// Generate a node ID based on the public key
	publicKeyBytes := elliptic.Marshal(privateKey.PublicKey.Curve, privateKey.PublicKey.X, privateKey.PublicKey.Y)
	nodeID := fmt.Sprintf("%x", sha256.Sum256(publicKeyBytes))[:16]

	// Initialize the node with genesis block
	genesisBlock := Block{
		Index:        0,
		Timestamp:    time.Now().Unix(),
		Transactions: []interface{}{},
		TrustProof: TrustProof{
			TrustDomain:    "genesis",
			ValidatorID:    nodeID,
			TrustScore:     1.0,
			ValidationTime: time.Now().Unix(),
		},
		PrevHash: "0",
	}
	genesisBlock.Hash = calculateBlockHash(genesisBlock)

	node := &QuidnugNode{
		NodeID:            nodeID,
		PrivateKey:        privateKey,
		PublicKey:         &privateKey.PublicKey,
		Blockchain:        []Block{genesisBlock},
		PendingTxs:        []interface{}{},
		TrustDomains:      make(map[string]TrustDomain),
		KnownNodes:        make(map[string]Node),
		TrustRegistry:     make(map[string]map[string]float64),
		IdentityRegistry:  make(map[string]IdentityTransaction),
		TitleRegistry:     make(map[string]TitleTransaction),
	}

	// Initialize default trust domain
	node.TrustDomains["default"] = TrustDomain{
		Name:           "default",
		ValidatorNodes: []string{nodeID},
		TrustThreshold: 0.75,
		BlockchainHead: genesisBlock.Hash,
		Validators:     map[string]float64{nodeID: 1.0},
	}

	log.Printf("Initialized quidnug node with ID: %s", nodeID)
	return node, nil
}

// Calculate hash for a block
func calculateBlockHash(block Block) string {
	blockData, _ := json.Marshal(struct {
		Index        int64
		Timestamp    int64
		Transactions []interface{}
		TrustProof   TrustProof
		PrevHash     string
	}{
		Index:        block.Index,
		Timestamp:    block.Timestamp,
		Transactions: block.Transactions,
		TrustProof:   block.TrustProof,
		PrevHash:     block.PrevHash,
	})

	hash := sha256.Sum256(blockData)
	return hex.EncodeToString(hash[:])
}

// Add a trust transaction to the pending pool
func (node *QuidnugNode) AddTrustTransaction(tx TrustTransaction) (string, error) {
	// Set timestamp if not set
	if tx.Timestamp == 0 {
		tx.Timestamp = time.Now().Unix()
	}
	
	// Set type
	tx.Type = TxTypeTrust
	
	// Generate transaction ID if not present
	if tx.ID == "" {
		txData, _ := json.Marshal(struct {
			Truster     string
			Trustee     string
			TrustLevel  float64
			TrustDomain string
			Timestamp   int64
		}{
			Truster:     tx.Truster,
			Trustee:     tx.Trustee,
			TrustLevel:  tx.TrustLevel,
			TrustDomain: tx.TrustDomain,
			Timestamp:   tx.Timestamp,
		})

		hash := sha256.Sum256(txData)
		tx.ID = hex.EncodeToString(hash[:])
	}
	
	// Validate the transaction
	if !node.ValidateTrustTransaction(tx) {
		return "", fmt.Errorf("invalid trust transaction")
	}

	node.PendingTxsMutex.Lock()
	defer node.PendingTxsMutex.Unlock()

	// Add transaction to pending pool
	node.PendingTxs = append(node.PendingTxs, tx)

	// Broadcast to other nodes in the same trust domain
	go node.BroadcastTransaction(tx)

	log.Printf("Added trust transaction %s to pending pool", tx.ID)
	return tx.ID, nil
}

// Add an identity transaction to the pending pool
func (node *QuidnugNode) AddIdentityTransaction(tx IdentityTransaction) (string, error) {
	// Set timestamp if not set
	if tx.Timestamp == 0 {
		tx.Timestamp = time.Now().Unix()
	}
	
	// Set type
	tx.Type = TxTypeIdentity
	
	// Generate transaction ID if not present
	if tx.ID == "" {
		txData, _ := json.Marshal(struct {
			QuidID      string
			Name        string
			Creator     string
			TrustDomain string
			UpdateNonce int64
			Timestamp   int64
		}{
			QuidID:      tx.QuidID,
			Name:        tx.Name,
			Creator:     tx.Creator,
			TrustDomain: tx.TrustDomain,
			UpdateNonce: tx.UpdateNonce,
			Timestamp:   tx.Timestamp,
		})

		hash := sha256.Sum256(txData)
		tx.ID = hex.EncodeToString(hash[:])
	}
	
	// Validate the transaction
	if !node.ValidateIdentityTransaction(tx) {
		return "", fmt.Errorf("invalid identity transaction")
	}

	node.PendingTxsMutex.Lock()
	defer node.PendingTxsMutex.Unlock()

	// Add transaction to pending pool
	node.PendingTxs = append(node.PendingTxs, tx)

	// Broadcast to other nodes in the same trust domain
	go node.BroadcastTransaction(tx)

	log.Printf("Added identity transaction %s to pending pool", tx.ID)
	return tx.ID, nil
}

// Add a title transaction to the pending pool
func (node *QuidnugNode) AddTitleTransaction(tx TitleTransaction) (string, error) {
	// Set timestamp if not set
	if tx.Timestamp == 0 {
		tx.Timestamp = time.Now().Unix()
	}
	
	// Set type
	tx.Type = TxTypeTitle
	
	// Generate transaction ID if not present
	if tx.ID == "" {
		txData, _ := json.Marshal(struct {
			AssetID     string
			Owners      []OwnershipStake
			TrustDomain string
			Timestamp   int64
		}{
			AssetID:     tx.AssetID,
			Owners:      tx.Owners,
			TrustDomain: tx.TrustDomain,
			Timestamp:   tx.Timestamp,
		})

		hash := sha256.Sum256(txData)
		tx.ID = hex.EncodeToString(hash[:])
	}
	
	// Validate the transaction
	if !node.ValidateTitleTransaction(tx) {
		return "", fmt.Errorf("invalid title transaction")
	}

	node.PendingTxsMutex.Lock()
	defer node.PendingTxsMutex.Unlock()

	// Add transaction to pending pool
	node.PendingTxs = append(node.PendingTxs, tx)

	// Broadcast to other nodes in the same trust domain
	go node.BroadcastTransaction(tx)

	log.Printf("Added title transaction %s to pending pool", tx.ID)
	return tx.ID, nil
}

// Validate a trust transaction
func (node *QuidnugNode) ValidateTrustTransaction(tx TrustTransaction) bool {
	// Check if transaction belongs to a known trust domain
	node.TrustDomainsMutex.RLock()
	_, domainExists := node.TrustDomains[tx.TrustDomain]
	node.TrustDomainsMutex.RUnlock()

	if !domainExists && tx.TrustDomain != "" {
		log.Printf("Trust transaction from unknown trust domain: %s", tx.TrustDomain)
		return false
	}

	// Verify trust level is in valid range (0.0 to 1.0)
	if tx.TrustLevel < 0.0 || tx.TrustLevel > 1.0 {
		log.Printf("Invalid trust level: %f", tx.TrustLevel)
		return false
	}

	// Check if truster exists in identity registry
	node.IdentityRegistryMutex.RLock()
	_, trusterExists := node.IdentityRegistry[tx.Truster]
	node.IdentityRegistryMutex.RUnlock()

	if !trusterExists {
		log.Printf("Truster %s not found in identity registry", tx.Truster)
	}

	// Check if trustee exists in identity registry
	node.IdentityRegistryMutex.RLock()
	_, trusteeExists := node.IdentityRegistry[tx.Trustee]
	node.IdentityRegistryMutex.RUnlock()

	if !trusteeExists {
		log.Printf("Trustee %s not found in identity registry", tx.Trustee)
	}

	// Verify signature
	if tx.Signature == "" || tx.PublicKey == "" {
		log.Printf("Missing signature or public key in trust transaction")
		return false
	}

	// Get signable data (transaction with signature field cleared)
	txCopy := tx
	txCopy.Signature = ""
	signableData, err := json.Marshal(txCopy)
	if err != nil {
		log.Printf("Failed to marshal transaction for signature verification: %v", err)
		return false
	}

	if !VerifySignature(tx.PublicKey, signableData, tx.Signature) {
		log.Printf("Invalid signature in trust transaction")
		return false
	}

	return true
}

// Validate an identity transaction
func (node *QuidnugNode) ValidateIdentityTransaction(tx IdentityTransaction) bool {
	// Check if transaction belongs to a known trust domain
	node.TrustDomainsMutex.RLock()
	_, domainExists := node.TrustDomains[tx.TrustDomain]
	node.TrustDomainsMutex.RUnlock()

	if !domainExists && tx.TrustDomain != "" {
		log.Printf("Identity transaction from unknown trust domain: %s", tx.TrustDomain)
		return false
	}

	// Check if this is an update to an existing identity
	node.IdentityRegistryMutex.RLock()
	existingIdentity, exists := node.IdentityRegistry[tx.QuidID]
	node.IdentityRegistryMutex.RUnlock()

	if exists {
		// If it's an update, check that update nonce is higher than current
		if tx.UpdateNonce <= existingIdentity.UpdateNonce {
			log.Printf("Invalid update nonce for identity %s: %d <= %d", 
				tx.QuidID, tx.UpdateNonce, existingIdentity.UpdateNonce)
			return false
		}
		
		// Also verify that the creator is the same as original creator
		if tx.Creator != existingIdentity.Creator {
			log.Printf("Identity update creator mismatch: %s != %s", 
				tx.Creator, existingIdentity.Creator)
			return false
		}
	}

	// Verify signature
	if tx.Signature == "" || tx.PublicKey == "" {
		log.Printf("Missing signature or public key in identity transaction")
		return false
	}

	// Get signable data (transaction with signature field cleared)
	txCopy := tx
	txCopy.Signature = ""
	signableData, err := json.Marshal(txCopy)
	if err != nil {
		log.Printf("Failed to marshal transaction for signature verification: %v", err)
		return false
	}

	if !VerifySignature(tx.PublicKey, signableData, tx.Signature) {
		log.Printf("Invalid signature in identity transaction")
		return false
	}

	return true
}

// Validate a title transaction
func (node *QuidnugNode) ValidateTitleTransaction(tx TitleTransaction) bool {
	// Check if transaction belongs to a known trust domain
	node.TrustDomainsMutex.RLock()
	_, domainExists := node.TrustDomains[tx.TrustDomain]
	node.TrustDomainsMutex.RUnlock()

	if !domainExists && tx.TrustDomain != "" {
		log.Printf("Title transaction from unknown trust domain: %s", tx.TrustDomain)
		return false
	}

	// Check if asset exists in identity registry
	node.IdentityRegistryMutex.RLock()
	_, assetExists := node.IdentityRegistry[tx.AssetID]
	node.IdentityRegistryMutex.RUnlock()

	if !assetExists {
		log.Printf("Asset %s not found in identity registry", tx.AssetID)
		return false
	}

	// Verify total ownership percentage adds up to 100%
	var totalPercentage float64
	for _, stake := range tx.Owners {
		totalPercentage += stake.Percentage
	}
	
	if totalPercentage != 100.0 {
		log.Printf("Total ownership percentage doesn't equal 100%%: %.2f", totalPercentage)
		return false
	}

	// Verify main signature from issuer
	if tx.Signature == "" || tx.PublicKey == "" {
		log.Printf("Missing signature or public key in title transaction")
		return false
	}

	// Get signable data for issuer (transaction with main signature cleared)
	txCopyForIssuer := tx
	txCopyForIssuer.Signature = ""
	issuerSignableData, err := json.Marshal(txCopyForIssuer)
	if err != nil {
		log.Printf("Failed to marshal transaction for issuer signature verification: %v", err)
		return false
	}

	if !VerifySignature(tx.PublicKey, issuerSignableData, tx.Signature) {
		log.Printf("Invalid issuer signature in title transaction")
		return false
	}

	// If this is a transfer (has previous owners), verify previous owners' signatures
	if len(tx.PreviousOwners) > 0 {
		// Verify previous ownership matches current title in registry
		node.TitleRegistryMutex.RLock()
		currentTitle, exists := node.TitleRegistry[tx.AssetID]
		node.TitleRegistryMutex.RUnlock()
		
		if exists {
			if !areOwnershipStakesEqual(tx.PreviousOwners, currentTitle.Owners) {
				log.Printf("Previous owners don't match current title")
				return false
			}
		}

		// Get signable data for owners (transaction with all signatures cleared)
		txCopyForOwners := tx
		txCopyForOwners.Signature = ""
		txCopyForOwners.Signatures = nil
		ownerSignableData, err := json.Marshal(txCopyForOwners)
		if err != nil {
			log.Printf("Failed to marshal transaction for owner signature verification: %v", err)
			return false
		}

		// Verify each previous owner's signature
		for _, stake := range tx.PreviousOwners {
			// Get previous owner's public key from identity registry
			node.IdentityRegistryMutex.RLock()
			ownerIdentity, ownerExists := node.IdentityRegistry[stake.OwnerID]
			node.IdentityRegistryMutex.RUnlock()

			if !ownerExists {
				log.Printf("Previous owner %s not found in identity registry", stake.OwnerID)
				return false
			}

			if ownerIdentity.PublicKey == "" {
				log.Printf("Previous owner %s has no public key", stake.OwnerID)
				return false
			}

			// Get the signature for this owner
			ownerSig, hasSig := tx.Signatures[stake.OwnerID]
			if !hasSig || ownerSig == "" {
				log.Printf("Missing signature from previous owner %s", stake.OwnerID)
				return false
			}

			// Verify the owner's signature
			if !VerifySignature(ownerIdentity.PublicKey, ownerSignableData, ownerSig) {
				log.Printf("Invalid signature from previous owner %s", stake.OwnerID)
				return false
			}
		}
	}

	return true
}

// Helper function to compare ownership stakes
func areOwnershipStakesEqual(a, b []OwnershipStake) bool {
	if len(a) != len(b) {
		return false
	}
	
	// Create maps for easy comparison
	mapA := make(map[string]float64)
	mapB := make(map[string]float64)
	
	for _, stake := range a {
		mapA[stake.OwnerID] = stake.Percentage
	}
	
	for _, stake := range b {
		mapB[stake.OwnerID] = stake.Percentage
	}
	
	// Compare maps
	for owner, percentage := range mapA {
		if mapB[owner] != percentage {
			return false
		}
	}
	
	return true
}

// Generate a new block with pending transactions
func (node *QuidnugNode) GenerateBlock(trustDomain string) (*Block, error) {
	node.BlockchainMutex.RLock()
	prevBlock := node.Blockchain[len(node.Blockchain)-1]
	node.BlockchainMutex.RUnlock()

	node.PendingTxsMutex.Lock()
	defer node.PendingTxsMutex.Unlock()

	if len(node.PendingTxs) == 0 {
		return nil, fmt.Errorf("no pending transactions to include in block")
	}

	// Filter transactions for this trust domain
	var domainTxs []interface{}
	var remainingTxs []interface{}

	for _, tx := range node.PendingTxs {
		var txDomain string
		
		// Extract trust domain based on transaction type
		switch t := tx.(type) {
		case TrustTransaction:
			txDomain = t.TrustDomain
		case IdentityTransaction:
			txDomain = t.TrustDomain
		case TitleTransaction:
			txDomain = t.TrustDomain
		default:
			// Unknown transaction type, skip
			continue
		}
		
		if txDomain == trustDomain || (trustDomain == "default" && txDomain == "") {
			domainTxs = append(domainTxs, tx)
		} else {
			remainingTxs = append(remainingTxs, tx)
		}
	}

	if len(domainTxs) == 0 {
		return nil, fmt.Errorf("no pending transactions for trust domain: %s", trustDomain)
	}

	// Create a new block
	newBlock := Block{
		Index:        prevBlock.Index + 1,
		Timestamp:    time.Now().Unix(),
		Transactions: domainTxs,
		TrustProof: TrustProof{
			TrustDomain:    trustDomain,
			ValidatorID:    node.NodeID,
			TrustScore:     node.GetTrustDomainScore(trustDomain),
			ValidatorSigs:  []string{},
			ValidationTime: time.Now().Unix(),
		},
		PrevHash: prevBlock.Hash,
	}

	// Sign the block with our validator signature
	signature, err := node.SignData([]byte(newBlock.PrevHash))
	if err == nil {
		newBlock.TrustProof.ValidatorSigs = append(newBlock.TrustProof.ValidatorSigs, hex.EncodeToString(signature))
	}

	// Calculate the hash of the new block
	newBlock.Hash = calculateBlockHash(newBlock)

	// Update pending transactions (remove the ones included in this block)
	node.PendingTxs = remainingTxs

	log.Printf("Generated new block %d for trust domain %s with %d transactions", 
		newBlock.Index, trustDomain, len(domainTxs))

	return &newBlock, nil
}

// Add a block to the blockchain after validation
func (node *QuidnugNode) AddBlock(block Block) error {
	// Validate the block
	if !node.ValidateBlock(block) {
		return fmt.Errorf("invalid block")
	}

	node.BlockchainMutex.Lock()
	defer node.BlockchainMutex.Unlock()

	// Add block to blockchain
	node.Blockchain = append(node.Blockchain, block)

	// Process transactions in the block to update registries
	node.processBlockTransactions(block)

	// Update the trust domain's blockchain head
	node.TrustDomainsMutex.Lock()
	if domain, exists := node.TrustDomains[block.TrustProof.TrustDomain]; exists {
		domain.BlockchainHead = block.Hash
		node.TrustDomains[block.TrustProof.TrustDomain] = domain
	}
	node.TrustDomainsMutex.Unlock()

	log.Printf("Added block %d with hash %s to blockchain", block.Index, block.Hash)
	return nil
}

// Process transactions in a block to update registries
func (node *QuidnugNode) processBlockTransactions(block Block) {
	for _, txInterface := range block.Transactions {
		txJson, _ := json.Marshal(txInterface)
		
		// Determine transaction type
		var baseTx BaseTransaction
		json.Unmarshal(txJson, &baseTx)
		
		switch baseTx.Type {
		case TxTypeTrust:
			var tx TrustTransaction
			json.Unmarshal(txJson, &tx)
			node.updateTrustRegistry(tx)
			
		case TxTypeIdentity:
			var tx IdentityTransaction
			json.Unmarshal(txJson, &tx)
			node.updateIdentityRegistry(tx)
			
		case TxTypeTitle:
			var tx TitleTransaction
			json.Unmarshal(txJson, &tx)
			node.updateTitleRegistry(tx)
		}
	}
}

// Start the HTTP server for API endpoints
func (node *QuidnugNode) StartServer(port string) error {
	router := mux.NewRouter()

	// API endpoints
	router.HandleFunc("/api/health", node.HealthCheckHandler).Methods("GET")
	router.HandleFunc("/api/nodes", node.GetNodesHandler).Methods("GET")
	
	// Transaction endpoints
	router.HandleFunc("/api/transactions", node.GetTransactionsHandler).Methods("GET")
	router.HandleFunc("/api/transactions/trust", node.CreateTrustTransactionHandler).Methods("POST")
	router.HandleFunc("/api/transactions/identity", node.CreateIdentityTransactionHandler).Methods("POST")
	router.HandleFunc("/api/transactions/title", node.CreateTitleTransactionHandler).Methods("POST")
	
	// Blockchain endpoints
	router.HandleFunc("/api/blocks", node.GetBlocksHandler).Methods("GET")
	
	// Trust domain endpoints
	router.HandleFunc("/api/domains", node.GetDomainsHandler).Methods("GET")
	router.HandleFunc("/api/domains", node.RegisterDomainHandler).Methods("POST")
	router.HandleFunc("/api/domains/{name}/query", node.QueryDomainHandler).Methods("GET")
	
	// Registry query endpoints
	router.HandleFunc("/api/registry/trust", node.QueryTrustRegistryHandler).Methods("GET")
	router.HandleFunc("/api/registry/identity", node.QueryIdentityRegistryHandler).Methods("GET")
	router.HandleFunc("/api/registry/title", node.QueryTitleRegistryHandler).Methods("GET")

	// Start HTTP server
	log.Printf("Starting quidnug node server on port %s", port)
	return http.ListenAndServe(":"+port, router)
}

// --- HTTP Handlers ---

func (node *QuidnugNode) HealthCheckHandler(w http.ResponseWriter, r *http.Request) {
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":   "ok",
		"node_id":  node.NodeID,
		"uptime":   time.Now().Unix() - node.Blockchain[0].Timestamp,
		"version":  "1.0.0",
	})
}

func (node *QuidnugNode) GetNodesHandler(w http.ResponseWriter, r *http.Request) {
	node.KnownNodesMutex.RLock()
	defer node.KnownNodesMutex.RUnlock()

	// Convert map to slice for response
	var nodesList []Node
	for _, n := range node.KnownNodes {
		nodesList = append(nodesList, n)
	}

	json.NewEncoder(w).Encode(map[string]interface{}{
		"nodes": nodesList,
	})
}

func (node *QuidnugNode) GetTransactionsHandler(w http.ResponseWriter, r *http.Request) {
	node.PendingTxsMutex.RLock()
	defer node.PendingTxsMutex.RUnlock()

	json.NewEncoder(w).Encode(map[string]interface{}{
		"pending_transactions": node.PendingTxs,
	})
}

func (node *QuidnugNode) CreateTrustTransactionHandler(w http.ResponseWriter, r *http.Request) {
	var tx TrustTransaction
	if err := json.NewDecoder(r.Body).Decode(&tx); err != nil {
		http.Error(w, "Invalid transaction data", http.StatusBadRequest)
		return
	}

	// Set timestamp if not provided
	if tx.Timestamp == 0 {
		tx.Timestamp = time.Now().Unix()
	}

	txID, err := node.AddTrustTransaction(tx)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":          "success",
		"transaction_id":  txID,
		"message":         "Trust transaction added to pending pool",
	})
}

func (node *QuidnugNode) CreateIdentityTransactionHandler(w http.ResponseWriter, r *http.Request) {
	var tx IdentityTransaction
	if err := json.NewDecoder(r.Body).Decode(&tx); err != nil {
		http.Error(w, "Invalid transaction data", http.StatusBadRequest)
		return
	}

	// Set timestamp if not provided
	if tx.Timestamp == 0 {
		tx.Timestamp = time.Now().Unix()
	}

	txID, err := node.AddIdentityTransaction(tx)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":          "success",
		"transaction_id":  txID,
		"message":         "Identity transaction added to pending pool",
	})
}

func (node *QuidnugNode) CreateTitleTransactionHandler(w http.ResponseWriter, r *http.Request) {
	var tx TitleTransaction
	if err := json.NewDecoder(r.Body).Decode(&tx); err != nil {
		http.Error(w, "Invalid transaction data", http.StatusBadRequest)
		return
	}

	// Set timestamp if not provided
	if tx.Timestamp == 0 {
		tx.Timestamp = time.Now().Unix()
	}

	txID, err := node.AddTitleTransaction(tx)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":          "success",
		"transaction_id":  txID,
		"message":         "Title transaction added to pending pool",
	})
}

func (node *QuidnugNode) GetBlocksHandler(w http.ResponseWriter, r *http.Request) {
	node.BlockchainMutex.RLock()
	defer node.BlockchainMutex.RUnlock()

	json.NewEncoder(w).Encode(map[string]interface{}{
		"blocks": node.Blockchain,
	})
}

func (node *QuidnugNode) GetDomainsHandler(w http.ResponseWriter, r *http.Request) {
	node.TrustDomainsMutex.RLock()
	defer node.TrustDomainsMutex.RUnlock()

	// Convert map to slice for response
	var domainsList []TrustDomain
	for _, domain := range node.TrustDomains {
		domainsList = append(domainsList, domain)
	}

	json.NewEncoder(w).Encode(map[string]interface{}{
		"domains": domainsList,
	})
}

func (node *QuidnugNode) RegisterDomainHandler(w http.ResponseWriter, r *http.Request) {
	var domain TrustDomain
	if err := json.NewDecoder(r.Body).Decode(&domain); err != nil {
		http.Error(w, "Invalid domain data", http.StatusBadRequest)
		return
	}

	if err := node.RegisterTrustDomain(domain); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":  "success",
		"message": "Trust domain registered successfully",
		"domain":  domain.Name,
	})
}

func (node *QuidnugNode) QueryDomainHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	domainName := vars["name"]
	queryType := r.URL.Query().Get("type")
	queryParam := r.URL.Query().Get("param")

	// Check if this node manages the requested domain
	node.TrustDomainsMutex.RLock()
	_, localDomain := node.TrustDomains[domainName]
	node.TrustDomainsMutex.RUnlock()

	if localDomain {
		// Handle local domain query based on query type
		var result interface{}
		var err error
		
		switch queryType {
		case "identity":
			// Query local identity registry
			identity, exists := node.GetQuidIdentity(queryParam)
			if !exists {
				http.Error(w, "Identity not found", http.StatusNotFound)
				return
			}
			result = identity
			
		case "trust":
			// Parse truster and trustee from param (format: "truster:trustee")
			parts := strings.Split(queryParam, ":")
			if len(parts) != 2 {
				http.Error(w, "Invalid trust query format", http.StatusBadRequest)
				return
			}
			trustLevel := node.GetTrustLevel(parts[0], parts[1])
			result = map[string]interface{}{
				"truster":     parts[0],
				"trustee":     parts[1],
				"trust_level": trustLevel,
				"domain":      domainName,
			}
			
		case "title":
			// Query local title registry
			title, exists := node.GetAssetOwnership(queryParam)
			if !exists {
				http.Error(w, "Title not found", http.StatusNotFound)
				return
			}
			result = title
			
		default:
			http.Error(w, "Unknown query type", http.StatusBadRequest)
			return
		}
		
		json.NewEncoder(w).Encode(result)
	} else {
		// Forward query to other domains
		result, err := node.QueryOtherDomain(domainName, queryType, queryParam)
		if err != nil {
			http.Error(w, err.Error(), http.StatusNotFound)
			return
		}
		json.NewEncoder(w).Encode(result)
	}
}

func (node *QuidnugNode) QueryTrustRegistryHandler(w http.ResponseWriter, r *http.Request) {
	truster := r.URL.Query().Get("truster")
	trustee := r.URL.Query().Get("trustee")
	
	if truster != "" && trustee != "" {
		// Query specific trust relationship
		trustLevel := node.GetTrustLevel(truster, trustee)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"truster":     truster,
			"trustee":     trustee,
			"trust_level": trustLevel,
		})
	} else if truster != "" {
		// Query all relationships for a truster
		node.TrustRegistryMutex.RLock()
		relationships := node.TrustRegistry[truster]
		node.TrustRegistryMutex.RUnlock()
		
		json.NewEncoder(w).Encode(map[string]interface{}{
			"truster":        truster,
			"relationships":  relationships,
		})
	} else {
		// Return overview of trust registry
		node.TrustRegistryMutex.RLock()
		defer node.TrustRegistryMutex.RUnlock()
		
		json.NewEncoder(w).Encode(map[string]interface{}{
			"trust_registry": node.TrustRegistry,
		})
	}
}

func (node *QuidnugNode) QueryIdentityRegistryHandler(w http.ResponseWriter, r *http.Request) {
	quidID := r.URL.Query().Get("quid_id")
	
	if quidID != "" {
		// Query specific identity
		identity, exists := node.GetQuidIdentity(quidID)
		if !exists {
			http.Error(w, "Identity not found", http.StatusNotFound)
			return
		}
		
		json.NewEncoder(w).Encode(identity)
	} else {
		// Return all identities (could be paginated in a real implementation)
		node.IdentityRegistryMutex.RLock()
		defer node.IdentityRegistryMutex.RUnlock()
		
		json.NewEncoder(w).Encode(map[string]interface{}{
			"identities": node.IdentityRegistry,
		})
	}
}

func (node *QuidnugNode) QueryTitleRegistryHandler(w http.ResponseWriter, r *http.Request) {
	assetID := r.URL.Query().Get("asset_id")
	ownerID := r.URL.Query().Get("owner_id")
	
	if assetID != "" {
		// Query specific asset title
		title, exists := node.GetAssetOwnership(assetID)
		if !exists {
			http.Error(w, "Title not found", http.StatusNotFound)
			return
		}
		
		json.NewEncoder(w).Encode(title)
	} else if ownerID != "" {
		// Query all assets owned by a specific owner
		node.TitleRegistryMutex.RLock()
		defer node.TitleRegistryMutex.RUnlock()
		
		var ownedAssets []map[string]interface{}
		
		for assetID, title := range node.TitleRegistry {
			for _, stake := range title.Owners {
				if stake.OwnerID == ownerID {
					ownedAssets = append(ownedAssets, map[string]interface{}{
						"asset_id":   assetID,
						"percentage": stake.Percentage,
						"stake_type": stake.StakeType,
					})
					break
				}
			}
		}
		
		json.NewEncoder(w).Encode(map[string]interface{}{
			"owner_id":     ownerID,
			"owned_assets": ownedAssets,
		})
	} else {
		// Return overview of title registry
		node.TitleRegistryMutex.RLock()
		defer node.TitleRegistryMutex.RUnlock()
		
		json.NewEncoder(w).Encode(map[string]interface{}{
			"title_registry": node.TitleRegistry,
		})
	}
}

// Update trust registry with a trust transaction
func (node *QuidnugNode) updateTrustRegistry(tx TrustTransaction) {
	node.TrustRegistryMutex.Lock()
	defer node.TrustRegistryMutex.Unlock()
	
	// Initialize map for truster if it doesn't exist
	if _, exists := node.TrustRegistry[tx.Truster]; !exists {
		node.TrustRegistry[tx.Truster] = make(map[string]float64)
	}
	
	// Update trust level
	node.TrustRegistry[tx.Truster][tx.Trustee] = tx.TrustLevel
	
	log.Printf("Updated trust registry: %s trusts %s at level %.2f", 
		tx.Truster, tx.Trustee, tx.TrustLevel)
}

// Update identity registry with an identity transaction
func (node *QuidnugNode) updateIdentityRegistry(tx IdentityTransaction) {
	node.IdentityRegistryMutex.Lock()
	defer node.IdentityRegistryMutex.Unlock()
	
	// Add or update identity
	node.IdentityRegistry[tx.QuidID] = tx
	
	log.Printf("Updated identity registry: %s (%s)", tx.QuidID, tx.Name)
}

// Update title registry with a title transaction
func (node *QuidnugNode) updateTitleRegistry(tx TitleTransaction) {
	node.TitleRegistryMutex.Lock()
	defer node.TitleRegistryMutex.Unlock()
	
	// Add or update title
	node.TitleRegistry[tx.AssetID] = tx
	
	log.Printf("Updated title registry: Asset %s now has %d owners", 
		tx.AssetID, len(tx.Owners))
}

// Validate a block
func (node *QuidnugNode) ValidateBlock(block Block) bool {
	node.BlockchainMutex.RLock()
	defer node.BlockchainMutex.RUnlock()

	// Check if blockchain is empty
	if len(node.Blockchain) == 0 {
		return false
	}

	prevBlock := node.Blockchain[len(node.Blockchain)-1]

	// Check block index and previous hash
	if block.Index != prevBlock.Index+1 || block.PrevHash != prevBlock.Hash {
		return false
	}

	// Verify the block hash
	if calculateBlockHash(block) != block.Hash {
		return false
	}

	// Verify trust proof (implement proof of trust validation)
	if !node.ValidateTrustProof(block.TrustProof) {
		return false
	}

	// Validate all transactions in the block
	for _, txInterface := range block.Transactions {
		txJson, _ := json.Marshal(txInterface)
		
		// Determine transaction type
		var baseTx BaseTransaction
		json.Unmarshal(txJson, &baseTx)
		
		var isValid bool
		
		switch baseTx.Type {
		case TxTypeTrust:
			var tx TrustTransaction
			json.Unmarshal(txJson, &tx)
			isValid = node.ValidateTrustTransaction(tx)
			
		case TxTypeIdentity:
			var tx IdentityTransaction
			json.Unmarshal(txJson, &tx)
			isValid = node.ValidateIdentityTransaction(tx)
			
		case TxTypeTitle:
			var tx TitleTransaction
			json.Unmarshal(txJson, &tx)
			isValid = node.ValidateTitleTransaction(tx)
			
		default:
			isValid = false
		}
		
		if !isValid {
			return false
		}
	}

	return true
}

// Validate a trust proof
func (node *QuidnugNode) ValidateTrustProof(proof TrustProof) bool {
	node.TrustDomainsMutex.RLock()
	defer node.TrustDomainsMutex.RUnlock()

	// Check if the trust domain exists
	domain, exists := node.TrustDomains[proof.TrustDomain]
	if !exists {
		return false
	}

	// Verify validator is part of the domain
	validatorFound := false
	for _, validatorID := range domain.ValidatorNodes {
		if validatorID == proof.ValidatorID {
			validatorFound = true
			break
		}
	}

	if !validatorFound {
		return false
	}

	// Validate signatures (in a real implementation, this would verify cryptographic signatures)
	// For simplicity, we're just checking that signatures exist
	if len(proof.ValidatorSigs) == 0 {
		return false
	}

	// Check trust score threshold
	if proof.TrustScore < domain.TrustThreshold {
		return false
	}

	return true
}

// Get node's trust score for a domain
func (node *QuidnugNode) GetTrustDomainScore(domain string) float64 {
	node.TrustDomainsMutex.RLock()
	defer node.TrustDomainsMutex.RUnlock()

	if trustDomain, exists := node.TrustDomains[domain]; exists {
		if score, found := trustDomain.Validators[node.NodeID]; found {
			return score
		}
	}

	// Default score for unknown domains or validators
	return 0.5
}

// Get trust level between two quids
func (node *QuidnugNode) GetTrustLevel(truster, trustee string) float64 {
	node.TrustRegistryMutex.RLock()
	defer node.TrustRegistryMutex.RUnlock()
	
	// Check if truster has a trust relationship with trustee
	if trustMap, exists := node.TrustRegistry[truster]; exists {
		if trustLevel, found := trustMap[trustee]; found {
			return trustLevel
		}
	}
	
	// Default trust level if no explicit relationship exists
	return 0.0
}

// Get quid identity information
func (node *QuidnugNode) GetQuidIdentity(quidID string) (IdentityTransaction, bool) {
	node.IdentityRegistryMutex.RLock()
	defer node.IdentityRegistryMutex.RUnlock()
	
	identity, exists := node.IdentityRegistry[quidID]
	return identity, exists
}

// Get asset ownership information
func (node *QuidnugNode) GetAssetOwnership(assetID string) (TitleTransaction, bool) {
	node.TitleRegistryMutex.RLock()
	defer node.TitleRegistryMutex.RUnlock()
	
	title, exists := node.TitleRegistry[assetID]
	return title, exists
}

// Discover other nodes in the network
func (node *QuidnugNode) DiscoverNodes(seedNodes []string) {
	for _, seedAddress := range seedNodes {
		// Make HTTP request to the seed node's discovery endpoint
		resp, err := http.Get(fmt.Sprintf("http://%s/api/nodes", seedAddress))
		if err != nil {
			log.Printf("Failed to connect to seed node %s: %v", seedAddress, err)
			continue
		}
		defer resp.Body.Close()

		var nodesResponse struct {
			Nodes []Node `json:"nodes"`
		}

		if err := json.NewDecoder(resp.Body).Decode(&nodesResponse); err != nil {
			log.Printf("Failed to decode node list from %s: %v", seedAddress, err)
			continue
		}

		// Add discovered nodes to our known nodes list
		node.KnownNodesMutex.Lock()
		for _, discoveredNode := range nodesResponse.Nodes {
			node.KnownNodes[discoveredNode.ID] = discoveredNode
			log.Printf("Discovered node: %s at %s", discoveredNode.ID, discoveredNode.Address)
		}
		node.KnownNodesMutex.Unlock()
	}
}

// Get nodes from a specific trust domain
func (node *QuidnugNode) GetTrustDomainNodes(domainName string) []Node {
	var domainNodes []Node

	node.TrustDomainsMutex.RLock()
	domain, exists := node.TrustDomains[domainName]
	node.TrustDomainsMutex.RUnlock()

	if !exists {
		return domainNodes
	}

	node.KnownNodesMutex.RLock()
	defer node.KnownNodesMutex.RUnlock()

	for _, validatorID := range domain.ValidatorNodes {
		if knownNode, exists := node.KnownNodes[validatorID]; exists {
			domainNodes = append(domainNodes, knownNode)
		}
	}

	return domainNodes
}

// Broadcast a transaction to other nodes in the trust domain
func (node *QuidnugNode) BroadcastTransaction(tx interface{}) {
	// Extract trust domain based on transaction type
	var domainName string
	
	switch t := tx.(type) {
	case TrustTransaction:
		domainName = t.TrustDomain
	case IdentityTransaction:
		domainName = t.TrustDomain
	case TitleTransaction:
		domainName = t.TrustDomain
	default:
		log.Printf("Cannot broadcast unknown transaction type")
		return
	}
	
	if domainName == "" {
		domainName = "default"
	}

	// Get nodes in this trust domain
	domainNodes := node.GetTrustDomainNodes(domainName)

	// Broadcast to each node
	for _, targetNode := range domainNodes {
		if targetNode.ID == node.NodeID {
			continue // Skip broadcasting to self
		}

		// Convert transaction to JSON
		txJSON, err := json.Marshal(tx)
		if err != nil {
			log.Printf("Failed to marshal transaction: %v", err)
			continue
		}

		// In a real implementation, this would make an HTTP POST request
		// to the target node's transaction endpoint
		log.Printf("Broadcasting transaction to node %s at %s",
			targetNode.ID, targetNode.Address)
	}
}

// Register a new trust domain
func (node *QuidnugNode) RegisterTrustDomain(domain TrustDomain) error {
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

	// Initialize validators map if empty
	if domain.Validators == nil {
		domain.Validators = make(map[string]float64)
	}

	// Add this node as a validator with default trust score
	domain.Validators[node.NodeID] = 1.0

	// Register the domain
	node.TrustDomains[domain.Name] = domain

	log.Printf("Registered new trust domain: %s", domain.Name)
	return nil
}

// Sign data with node's private key
func (node *QuidnugNode) SignData(data []byte) ([]byte, error) {
	hash := sha256.Sum256(data)
	
	r, s, err := ecdsa.Sign(rand.Reader, node.PrivateKey, hash[:])
	if err != nil {
		return nil, err
	}

	// Pad r and s to 32 bytes each for P-256 (64 bytes total)
	signature := make([]byte, 64)
	rBytes := r.Bytes()
	sBytes := s.Bytes()
	copy(signature[32-len(rBytes):32], rBytes)
	copy(signature[64-len(sBytes):64], sBytes)
	
	return signature, nil
}

// GetPublicKeyHex returns the hex-encoded public key in uncompressed format
func (node *QuidnugNode) GetPublicKeyHex() string {
	publicKeyBytes := elliptic.Marshal(node.PublicKey.Curve, node.PublicKey.X, node.PublicKey.Y)
	return hex.EncodeToString(publicKeyBytes)
}

// VerifySignature verifies an ECDSA P-256 signature
// publicKeyHex: hex-encoded public key in uncompressed format (65 bytes: 0x04 || X || Y)
// data: the data that was signed
// signatureHex: hex-encoded signature (64 bytes: r || s, each padded to 32 bytes)
func VerifySignature(publicKeyHex string, data []byte, signatureHex string) bool {
	if publicKeyHex == "" || signatureHex == "" {
		return false
	}

	// Decode public key from hex
	publicKeyBytes, err := hex.DecodeString(publicKeyHex)
	if err != nil {
		log.Printf("Failed to decode public key hex: %v", err)
		return false
	}

	// Unmarshal the public key
	x, y := elliptic.Unmarshal(elliptic.P256(), publicKeyBytes)
	if x == nil {
		log.Printf("Failed to unmarshal public key")
		return false
	}

	publicKey := &ecdsa.PublicKey{
		Curve: elliptic.P256(),
		X:     x,
		Y:     y,
	}

	// Decode signature from hex
	signatureBytes, err := hex.DecodeString(signatureHex)
	if err != nil {
		log.Printf("Failed to decode signature hex: %v", err)
		return false
	}

	// For P-256, signature should be 64 bytes (32 for r, 32 for s)
	if len(signatureBytes) != 64 {
		log.Printf("Invalid signature length: expected 64, got %d", len(signatureBytes))
		return false
	}

	r := new(big.Int).SetBytes(signatureBytes[:32])
	s := new(big.Int).SetBytes(signatureBytes[32:])

	// Hash the data
	hash := sha256.Sum256(data)

	// Verify the signature
	return ecdsa.Verify(publicKey, hash[:], r, s)
}

// Recursively query other trust domains
func (node *QuidnugNode) QueryOtherDomain(domainName, queryType, queryParam string) (interface{}, error) {
	// Find nodes that manage this trust domain
	var domainManagers []Node

	node.KnownNodesMutex.RLock()
	for _, knownNode := range node.KnownNodes {
		for _, domain := range knownNode.TrustDomains {
			if domain == domainName {
				domainManagers = append(domainManagers, knownNode)
				break
			}
		}
	}
	node.KnownNodesMutex.RUnlock()

	if len(domainManagers) == 0 {
		return nil, fmt.Errorf("no known nodes manage trust domain: %s", domainName)
	}

	// For simplicity, query the first node that manages this domain
	targetNode := domainManagers[0]

	// In a real implementation, this would make an HTTP request to the
	// target node's query endpoint with the appropriate query parameters
	log.Printf("Querying node %s at %s for domain %s with query type: %s, param: %s",
		targetNode.ID, targetNode.Address, domainName, queryType, queryParam)

	// Mock response for demonstration
	switch queryType {
	case "identity":
		// Mocked identity query response
		return map[string]interface{}{
			"quid_id":    queryParam,
			"name":       "Sample Quid Name",
			"domain":     domainName,
			"attributes": map[string]interface{}{"key": "value"},
		}, nil
		
	case "trust":
		// Mocked trust query response
		return map[string]interface{}{
			"truster":     "truster_id",
			"trustee":     queryParam,
			"trust_level": 0.85,
			"domain":      domainName,
		}, nil
		
	case "title":
		// Mocked title query response
		return map[string]interface{}{
			"asset_id": queryParam,
			"domain":   domainName,
			"owners": []map[string]interface{}{
				{"owner_id": "owner1", "percentage": 60.0},
				{"owner_id": "owner2", "percentage": 40.0},
			},
		}, nil
		
	default:
		return map[string]interface{}{
			"error": "Unknown query type",
		}, nil
	}
}
