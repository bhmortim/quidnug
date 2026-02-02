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
	"os"
	"sync"
	"time"
)

// QuidnugNode is the main server structure
type QuidnugNode struct {
	NodeID     string
	PrivateKey *ecdsa.PrivateKey
	PublicKey  *ecdsa.PublicKey
	Blockchain []Block
	PendingTxs []interface{}
	TrustDomains map[string]TrustDomain
	KnownNodes   map[string]Node

	// State registries
	TrustRegistry    map[string]map[string]float64
	IdentityRegistry map[string]IdentityTransaction
	TitleRegistry    map[string]TitleTransaction

	// Mutexes for thread safety
	BlockchainMutex       sync.RWMutex
	PendingTxsMutex       sync.RWMutex
	KnownNodesMutex       sync.RWMutex
	TrustDomainsMutex     sync.RWMutex
	TrustRegistryMutex    sync.RWMutex
	IdentityRegistryMutex sync.RWMutex
	TitleRegistryMutex    sync.RWMutex
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

// NewQuidnugNode initializes a new quidnug node
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

// AddTrustTransaction adds a trust transaction to the pending pool
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

// AddIdentityTransaction adds an identity transaction to the pending pool
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

// AddTitleTransaction adds a title transaction to the pending pool
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

// GenerateBlock generates a new block with pending transactions
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

// AddBlock adds a block to the blockchain after validation
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

// RegisterTrustDomain registers a new trust domain
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

// GetTrustDomainScore returns the node's trust score for a domain
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
