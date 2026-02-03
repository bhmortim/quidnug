package main

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"
)

// Lock ordering to prevent deadlocks:
//
// When acquiring multiple locks, always acquire them in this order:
//   1. BlockchainMutex       - Protects Blockchain slice
//   2. TrustDomainsMutex     - Protects TrustDomains map
//   3. TrustRegistryMutex    - Protects TrustRegistry, TrustNonceRegistry, VerifiedTrustEdges
//   4. IdentityRegistryMutex - Protects IdentityRegistry map
//   5. TitleRegistryMutex    - Protects TitleRegistry map
//   6. EventStreamMutex      - Protects EventStreamRegistry, EventRegistry maps
//   7. PendingTxsMutex       - Protects PendingTxs slice
//   8. TentativeBlocksMutex  - Protects TentativeBlocks map
//   9. UnverifiedRegistryMutex - Protects UnverifiedTrustRegistry map
//  10. KnownNodesMutex       - Protects KnownNodes map
//
// Guidelines:
//   - Prefer acquiring a single lock when possible
//   - Release locks as soon as the protected data is no longer needed
//   - Use RLock for read-only access to enable concurrent readers
//   - Never call external code (HTTP requests, etc.) while holding a lock
//   - When computing trust (ComputeRelationalTrust), only TrustRegistryMutex is held briefly for reads

// Package-level logger
var logger *slog.Logger

// initLogger initializes the structured logger based on the log level
func initLogger(logLevel string) {
	var level slog.Level
	switch strings.ToLower(logLevel) {
	case "debug":
		level = slog.LevelDebug
	case "info":
		level = slog.LevelInfo
	case "warn", "warning":
		level = slog.LevelWarn
	case "error":
		level = slog.LevelError
	default:
		level = slog.LevelInfo
	}

	handler := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: level,
	})
	logger = slog.New(handler)
}

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
	TrustRegistry      map[string]map[string]float64
	TrustNonceRegistry map[string]map[string]int64
	IdentityRegistry   map[string]IdentityTransaction
	TitleRegistry      map[string]TitleTransaction

	// Event registries
	EventStreamRegistry map[string]*EventStream
	EventRegistry       map[string][]EventTransaction

	// IPFS client
	IPFSClient IPFSClient

	// HTTP server for graceful shutdown
	Server *http.Server

	// HTTP client for network communication
	httpClient *http.Client

	// Mutexes for thread safety
	BlockchainMutex       sync.RWMutex
	PendingTxsMutex       sync.RWMutex
	KnownNodesMutex       sync.RWMutex
	TrustDomainsMutex     sync.RWMutex
	TrustRegistryMutex    sync.RWMutex
	IdentityRegistryMutex sync.RWMutex
	TitleRegistryMutex    sync.RWMutex
	EventStreamMutex      sync.RWMutex

	// Node identity for signing blocks
	NodeQuidID string

	// Tentative blocks storage (blocks from partially-trusted validators)
	TentativeBlocks      map[string][]Block // keyed by trust domain
	TentativeBlocksMutex sync.RWMutex

	// Dual-layer trust registry
	VerifiedTrustEdges      map[string]map[string]TrustEdge
	UnverifiedTrustRegistry map[string]map[string]TrustEdge
	UnverifiedRegistryMutex sync.RWMutex

	// Threshold configuration
	DistrustThreshold         float64 // Below this, block is 'untrusted' (default 0.0)
	TransactionTrustThreshold float64 // Minimum trust to include tx in block (default 0.0)
}

func main() {
	// Load configuration
	cfg := LoadConfig()

	// Initialize structured logger
	initLogger(cfg.LogLevel)

	// Create cancellable context for graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Initialize node
	quidnugNode, err := NewQuidnugNode(cfg)
	if err != nil {
		logger.Error("Failed to initialize quidnug node", "error", err)
		os.Exit(1)
	}

	// Configure HTTP client timeout from config
	quidnugNode.SetHTTPClientTimeout(cfg.HTTPClientTimeout)

	// Load persisted pending transactions
	if err := quidnugNode.LoadPendingTransactions(cfg.DataDir); err != nil {
		logger.Warn("Failed to load pending transactions", "error", err)
	}

	// WaitGroup for background goroutines
	var wg sync.WaitGroup

	// Set up signal handling for graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		sig := <-sigChan
		logger.Info("Received shutdown signal", "signal", sig.String())
		cancel()
	}()

	// Discover other nodes (with context)
	wg.Add(1)
	go func() {
		defer wg.Done()
		quidnugNode.DiscoverNodes(ctx, cfg.SeedNodes)
	}()

	// Start block generation for managed trust domains (with context)
	wg.Add(1)
	go func() {
		defer wg.Done()
		quidnugNode.runBlockGeneration(ctx, cfg.BlockInterval)
	}()

	// Start HTTP server (non-blocking)
	serverErr := make(chan error, 1)
	wg.Add(1)
	go func() {
		defer wg.Done()
		if err := quidnugNode.StartServer(cfg.Port); err != nil && err != http.ErrServerClosed {
			serverErr <- err
		}
	}()

	// Wait for shutdown signal or server error
	select {
	case <-ctx.Done():
		logger.Info("Initiating graceful shutdown...")
	case err := <-serverErr:
		logger.Error("Server failed", "error", err)
		cancel()
	}

	// Graceful shutdown sequence
	quidnugNode.Shutdown(ctx, cfg)

	// Wait for all goroutines to finish
	logger.Info("Waiting for background goroutines to finish...")
	wg.Wait()

	logger.Info("Shutdown complete")
}

// runBlockGeneration runs the block generation loop with context cancellation support
func (node *QuidnugNode) runBlockGeneration(ctx context.Context, interval time.Duration) {
	logger.Info("Starting block generation loop", "interval", interval)
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			logger.Info("Block generation loop stopped")
			return
		case <-ticker.C:
			node.TrustDomainsMutex.RLock()
			managedDomains := make([]string, 0, len(node.TrustDomains))
			for domain := range node.TrustDomains {
				managedDomains = append(managedDomains, domain)
			}
			node.TrustDomainsMutex.RUnlock()

			for _, domain := range managedDomains {
				select {
				case <-ctx.Done():
					return
				default:
				}

				block, err := node.GenerateBlock(domain)
				if err != nil {
					logger.Debug("Failed to generate block", "domain", domain, "error", err)
					continue
				}

				if err := node.AddBlock(*block); err != nil {
					logger.Error("Failed to add generated block", "domain", domain, "error", err)
				}
			}
		}
	}
}

// Shutdown performs graceful shutdown of the node
func (node *QuidnugNode) Shutdown(ctx context.Context, cfg *Config) {
	// Create timeout context for shutdown
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), cfg.ShutdownTimeout)
	defer shutdownCancel()

	// Shutdown HTTP server gracefully
	if node.Server != nil {
		logger.Info("Shutting down HTTP server...", "timeout", cfg.ShutdownTimeout)
		if err := node.Server.Shutdown(shutdownCtx); err != nil {
			logger.Error("HTTP server shutdown error", "error", err)
		} else {
			logger.Info("HTTP server shutdown complete")
		}
	}

	// Save pending transactions
	logger.Info("Saving pending transactions...")
	if err := node.SavePendingTransactions(cfg.DataDir); err != nil {
		logger.Error("Failed to save pending transactions", "error", err)
	} else {
		node.PendingTxsMutex.RLock()
		txCount := len(node.PendingTxs)
		node.PendingTxsMutex.RUnlock()
		logger.Info("Pending transactions saved", "count", txCount)
	}
}

// NewQuidnugNode initializes a new quidnug node
func NewQuidnugNode(cfg *Config) (*QuidnugNode, error) {
	// Handle nil config with defaults for testing
	if cfg == nil {
		cfg = &Config{
			IPFSEnabled:    DefaultIPFSEnabled,
			IPFSGatewayURL: DefaultIPFSGatewayURL,
			IPFSTimeout:    DefaultIPFSTimeout,
		}
	}
	// Generate a new ECDSA key pair
	privateKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, fmt.Errorf("failed to generate key pair: %v", err)
	}

	// Generate a node ID based on the public key
	publicKeyBytes := elliptic.Marshal(privateKey.PublicKey.Curve, privateKey.PublicKey.X, privateKey.PublicKey.Y)
	nodeID := fmt.Sprintf("%x", sha256.Sum256(publicKeyBytes))[:16]

	// Compute public key hex for genesis block
	publicKeyHex := hex.EncodeToString(publicKeyBytes)

	// Initialize the node with genesis block
	genesisBlock := Block{
		Index:        0,
		Timestamp:    time.Now().Unix(),
		Transactions: []interface{}{},
		TrustProof: TrustProof{
			TrustDomain:             "genesis",
			ValidatorID:             nodeID,
			ValidatorPublicKey:      publicKeyHex,
			ValidatorTrustInCreator: 1.0,
			ValidatorSigs:           []string{},
			ValidationTime:          time.Now().Unix(),
		},
		PrevHash: "0",
	}
	genesisBlock.Hash = calculateBlockHash(genesisBlock)

	// Initialize IPFS client based on config
	var ipfsClient IPFSClient
	if cfg.IPFSEnabled {
		ipfsClient = NewHTTPIPFSClient(cfg.IPFSGatewayURL, &http.Client{Timeout: cfg.IPFSTimeout})
	} else {
		ipfsClient = &NoOpIPFSClient{}
	}

	node := &QuidnugNode{
		NodeID:                    nodeID,
		PrivateKey:               privateKey,
		PublicKey:                &privateKey.PublicKey,
		Blockchain:               []Block{genesisBlock},
		PendingTxs:               []interface{}{},
		TrustDomains:             make(map[string]TrustDomain),
		KnownNodes:               make(map[string]Node),
		TrustRegistry:            make(map[string]map[string]float64),
		TrustNonceRegistry:       make(map[string]map[string]int64),
		IdentityRegistry:         make(map[string]IdentityTransaction),
		TitleRegistry:            make(map[string]TitleTransaction),
		EventStreamRegistry:      make(map[string]*EventStream),
		EventRegistry:            make(map[string][]EventTransaction),
		IPFSClient:               ipfsClient,
		TentativeBlocks:          make(map[string][]Block),
		VerifiedTrustEdges:       make(map[string]map[string]TrustEdge),
		UnverifiedTrustRegistry:  make(map[string]map[string]TrustEdge),
		DistrustThreshold:        0.0,
		TransactionTrustThreshold: 0.0,
		httpClient: &http.Client{
			Timeout: 5 * time.Second,
		},
	}

	// Initialize default trust domain with this node's public key
	node.TrustDomains["default"] = TrustDomain{
		Name:                "default",
		ValidatorNodes:      []string{nodeID},
		TrustThreshold:      0.75,
		BlockchainHead:      genesisBlock.Hash,
		Validators:          map[string]float64{nodeID: 1.0},
		ValidatorPublicKeys: map[string]string{nodeID: publicKeyHex},
	}

	// Set node's quid identity from its public key
	node.NodeQuidID = node.GetPublicKeyHex()

	if logger != nil {
		logger.Info("Initialized quidnug node", "nodeId", nodeID)
	}
	return node, nil
}

// SetHTTPClientTimeout configures the HTTP client timeout
func (node *QuidnugNode) SetHTTPClientTimeout(timeout time.Duration) {
	node.httpClient.Timeout = timeout
}

// AddTrustTransaction adds a trust transaction to the pending pool
func (node *QuidnugNode) AddTrustTransaction(tx TrustTransaction) (string, error) {
	// Set timestamp if not set
	if tx.Timestamp == 0 {
		tx.Timestamp = time.Now().Unix()
	}
	
	// Set type
	tx.Type = TxTypeTrust
	
	// Set nonce if not provided (use current highest nonce + 1 for this truster-trustee pair)
	if tx.Nonce == 0 {
		node.TrustRegistryMutex.RLock()
		currentNonce := int64(0)
		if trusterNonces, exists := node.TrustNonceRegistry[tx.Truster]; exists {
			if nonce, found := trusterNonces[tx.Trustee]; found {
				currentNonce = nonce
			}
		}
		node.TrustRegistryMutex.RUnlock()
		tx.Nonce = currentNonce + 1
	}
	
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
		RecordTransactionProcessed("trust", false)
		return "", fmt.Errorf("invalid trust transaction")
	}

	node.PendingTxsMutex.Lock()
	defer node.PendingTxsMutex.Unlock()

	// Add transaction to pending pool
	node.PendingTxs = append(node.PendingTxs, tx)

	// Record metrics
	RecordTransactionProcessed("trust", true)
	UpdatePendingTransactionsGauge(len(node.PendingTxs))

	// Broadcast to other nodes in the same trust domain
	go node.BroadcastTransaction(tx)

	logger.Info("Added trust transaction to pending pool", "txId", tx.ID, "domain", tx.TrustDomain)
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
		RecordTransactionProcessed("identity", false)
		return "", fmt.Errorf("invalid identity transaction")
	}

	node.PendingTxsMutex.Lock()
	defer node.PendingTxsMutex.Unlock()

	// Add transaction to pending pool
	node.PendingTxs = append(node.PendingTxs, tx)

	// Record metrics
	RecordTransactionProcessed("identity", true)
	UpdatePendingTransactionsGauge(len(node.PendingTxs))

	// Broadcast to other nodes in the same trust domain
	go node.BroadcastTransaction(tx)

	logger.Info("Added identity transaction to pending pool", "txId", tx.ID, "quidId", tx.QuidID, "domain", tx.TrustDomain)
	return tx.ID, nil
}

// AddEventTransaction adds an event transaction to the pending pool
func (node *QuidnugNode) AddEventTransaction(tx EventTransaction) (string, error) {
	// Set timestamp if not set
	if tx.Timestamp == 0 {
		tx.Timestamp = time.Now().Unix()
	}

	// Set type
	tx.Type = TxTypeEvent

	// Auto-assign sequence if not provided (latest + 1)
	if tx.Sequence == 0 {
		node.EventStreamMutex.RLock()
		events, exists := node.EventRegistry[tx.SubjectID]
		if exists && len(events) > 0 {
			tx.Sequence = events[len(events)-1].Sequence + 1
		} else {
			tx.Sequence = 1
		}
		node.EventStreamMutex.RUnlock()
	}

	// Generate transaction ID if not present
	if tx.ID == "" {
		txData, _ := json.Marshal(struct {
			SubjectID   string
			EventType   string
			Sequence    int64
			TrustDomain string
			Timestamp   int64
		}{
			SubjectID:   tx.SubjectID,
			EventType:   tx.EventType,
			Sequence:    tx.Sequence,
			TrustDomain: tx.TrustDomain,
			Timestamp:   tx.Timestamp,
		})

		hash := sha256.Sum256(txData)
		tx.ID = hex.EncodeToString(hash[:])
	}

	// Validate the transaction
	if !node.ValidateEventTransaction(tx) {
		RecordTransactionProcessed("event", false)
		return "", fmt.Errorf("invalid event transaction")
	}

	node.PendingTxsMutex.Lock()
	defer node.PendingTxsMutex.Unlock()

	// Add transaction to pending pool
	node.PendingTxs = append(node.PendingTxs, tx)

	// Record metrics
	RecordTransactionProcessed("event", true)
	UpdatePendingTransactionsGauge(len(node.PendingTxs))

	// Broadcast to other nodes in the same trust domain
	go node.BroadcastTransaction(tx)

	logger.Info("Added event transaction to pending pool", "txId", tx.ID, "subjectId", tx.SubjectID, "domain", tx.TrustDomain)
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
		RecordTransactionProcessed("title", false)
		return "", fmt.Errorf("invalid title transaction")
	}

	node.PendingTxsMutex.Lock()
	defer node.PendingTxsMutex.Unlock()

	// Add transaction to pending pool
	node.PendingTxs = append(node.PendingTxs, tx)

	// Record metrics
	RecordTransactionProcessed("title", true)
	UpdatePendingTransactionsGauge(len(node.PendingTxs))

	// Broadcast to other nodes in the same trust domain
	go node.BroadcastTransaction(tx)

	logger.Info("Added title transaction to pending pool", "txId", tx.ID, "assetId", tx.AssetID, "domain", tx.TrustDomain)
	return tx.ID, nil
}

// FilterTransactionsForBlock filters pending transactions based on trust.
// Only includes transactions from sources the node trusts (or sponsored transactions).
// For each transaction, extracts the creator quid and computes relational trust.
// Transactions are included if trustLevel >= node.TransactionTrustThreshold.
func (node *QuidnugNode) FilterTransactionsForBlock(txs []interface{}, domain string) []interface{} {
	var filtered []interface{}

	for _, tx := range txs {
		var creatorQuid string
		var txID string

		// Extract creator quid based on transaction type
		switch t := tx.(type) {
		case TrustTransaction:
			creatorQuid = t.Truster
			txID = t.ID
		case IdentityTransaction:
			creatorQuid = t.Creator
			txID = t.ID
		case TitleTransaction:
			// For title transactions, use first owner as creator
			if len(t.Owners) > 0 {
				creatorQuid = t.Owners[0].OwnerID
			}
			txID = t.ID
		default:
			// Unknown transaction type, skip
			logger.Debug("Skipping unknown transaction type in trust filter")
			continue
		}

		// If no creator quid, skip this transaction
		if creatorQuid == "" {
			logger.Debug("Skipping transaction with empty creator",
				"txId", txID,
				"domain", domain)
			continue
		}

		// Compute relational trust from this node to the creator
		trustLevel, _, err := node.ComputeRelationalTrust(node.NodeQuidID, creatorQuid, DefaultTrustMaxDepth)
		if err != nil {
			logger.Warn("Trust computation exceeded resource limits",
				"txId", txID,
				"creator", creatorQuid,
				"error", err,
				"domain", domain)
			// Use partial result (trustLevel contains best found so far)
		}

		// Include if trust meets threshold
		if trustLevel >= node.TransactionTrustThreshold {
			filtered = append(filtered, tx)
		} else {
			logger.Debug("Filtered out transaction due to insufficient trust",
				"txId", txID,
				"creator", creatorQuid,
				"trustLevel", trustLevel,
				"threshold", node.TransactionTrustThreshold,
				"domain", domain)
		}
	}

	return filtered
}

// GenerateBlock generates a new block with pending transactions
func (node *QuidnugNode) GenerateBlock(trustDomain string) (*Block, error) {
	node.BlockchainMutex.RLock()
	prevBlock := node.Blockchain[len(node.Blockchain)-1]
	node.BlockchainMutex.RUnlock()

	// Get validator's participation weight for this domain
	node.TrustDomainsMutex.RLock()
	validatorWeight := 1.0 // default for self-owned domains
	if domain, exists := node.TrustDomains[trustDomain]; exists {
		if weight, found := domain.Validators[node.NodeID]; found {
			validatorWeight = weight
		}
	}
	node.TrustDomainsMutex.RUnlock()

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

	// Apply trust-based filtering to domain transactions
	domainTxs = node.FilterTransactionsForBlock(domainTxs, trustDomain)

	if len(domainTxs) == 0 {
		return nil, fmt.Errorf("no pending transactions for trust domain: %s", trustDomain)
	}

	// Create a new block
	newBlock := Block{
		Index:        prevBlock.Index + 1,
		Timestamp:    time.Now().Unix(),
		Transactions: domainTxs,
		TrustProof: TrustProof{
			TrustDomain:             trustDomain,
			ValidatorID:             node.NodeID,
			ValidatorPublicKey:      node.GetPublicKeyHex(),
			ValidatorTrustInCreator: validatorWeight,
			ValidatorSigs:           []string{},
			ValidationTime:          time.Now().Unix(),
		},
		PrevHash: prevBlock.Hash,
	}

	// Sign the full block content (not just PrevHash) to prevent transaction tampering
	signableData := GetBlockSignableData(newBlock)
	signature, err := node.SignData(signableData)
	if err == nil {
		newBlock.TrustProof.ValidatorSigs = append(newBlock.TrustProof.ValidatorSigs, hex.EncodeToString(signature))
	}

	// Calculate the hash of the new block
	newBlock.Hash = calculateBlockHash(newBlock)

	// Update pending transactions (remove the ones included in this block)
	node.PendingTxs = remainingTxs

	logger.Info("Generated new block",
		"blockIndex", newBlock.Index,
		"domain", trustDomain,
		"txCount", len(domainTxs),
		"hash", newBlock.Hash)

	RecordBlockGenerated(trustDomain)

	return &newBlock, nil
}

// AddBlock adds a block to the blockchain after validation.
// Only accepts fully trusted blocks. For tiered processing, use ReceiveBlock.
func (node *QuidnugNode) AddBlock(block Block) error {
	acceptance, err := node.ReceiveBlock(block)
	if err != nil {
		return err
	}
	if acceptance != BlockTrusted {
		return fmt.Errorf("block not trusted (acceptance tier: %d)", acceptance)
	}
	return nil
}

// ReceiveBlock processes an incoming block with tiered acceptance.
// Extracts trust graph data from all cryptographically valid blocks.
func (node *QuidnugNode) ReceiveBlock(block Block) (BlockAcceptance, error) {
	// 1. Cryptographic validation
	if !node.ValidateBlockCryptographic(block) {
		return BlockInvalid, fmt.Errorf("block failed cryptographic validation")
	}

	// 2. Extract trust edges from ALL cryptographically valid blocks
	edges := node.ExtractTrustEdgesFromBlock(block, false)
	for _, edge := range edges {
		node.AddUnverifiedTrustEdge(edge)
	}

	// 3. Get tiered acceptance
	acceptance := node.ValidateBlockTiered(block)

	// Record block reception metrics
	RecordBlockReceived(block.TrustProof.TrustDomain, acceptance)

	// 4. Process based on tier
	switch acceptance {
	case BlockTrusted:
		// Add to main chain
		node.BlockchainMutex.Lock()
		node.Blockchain = append(node.Blockchain, block)
		node.BlockchainMutex.Unlock()

		// Process transactions
		node.processBlockTransactions(block)

		// Update domain head
		node.TrustDomainsMutex.Lock()
		if domain, exists := node.TrustDomains[block.TrustProof.TrustDomain]; exists {
			domain.BlockchainHead = block.Hash
			node.TrustDomains[block.TrustProof.TrustDomain] = domain
		}
		node.TrustDomainsMutex.Unlock()

		// Promote extracted edges to verified
		for _, edge := range edges {
			node.PromoteTrustEdge(edge.Truster, edge.Trustee)
		}

		if logger != nil {
			logger.Info("Received trusted block",
				"blockIndex", block.Index,
				"hash", block.Hash,
				"domain", block.TrustProof.TrustDomain)
		}

	case BlockTentative:
		if err := node.StoreTentativeBlock(block); err != nil {
			return acceptance, err
		}
		if logger != nil {
			logger.Info("Received tentative block",
				"blockIndex", block.Index,
				"hash", block.Hash,
				"domain", block.TrustProof.TrustDomain)
		}

	case BlockUntrusted:
		// Edges already added as unverified, don't store block
		if logger != nil {
			logger.Info("Received untrusted block - extracted edges only",
				"blockIndex", block.Index,
				"hash", block.Hash,
				"domain", block.TrustProof.TrustDomain)
		}

	case BlockInvalid:
		return BlockInvalid, fmt.Errorf("block validation failed")
	}

	return acceptance, nil
}

// StoreTentativeBlock stores a block in the tentative storage.
func (node *QuidnugNode) StoreTentativeBlock(block Block) error {
	node.TentativeBlocksMutex.Lock()
	defer node.TentativeBlocksMutex.Unlock()

	domain := block.TrustProof.TrustDomain
	if _, exists := node.TentativeBlocks[domain]; !exists {
		node.TentativeBlocks[domain] = make([]Block, 0)
	}

	// Check if block already exists (by hash)
	for _, existing := range node.TentativeBlocks[domain] {
		if existing.Hash == block.Hash {
			return fmt.Errorf("block %s already in tentative storage", block.Hash)
		}
	}

	node.TentativeBlocks[domain] = append(node.TentativeBlocks[domain], block)
	return nil
}

// GetTentativeBlocks returns tentative blocks for a domain.
func (node *QuidnugNode) GetTentativeBlocks(domain string) []Block {
	node.TentativeBlocksMutex.RLock()
	defer node.TentativeBlocksMutex.RUnlock()

	if blocks, exists := node.TentativeBlocks[domain]; exists {
		result := make([]Block, len(blocks))
		copy(result, blocks)
		return result
	}
	return nil
}

// ReEvaluateTentativeBlocks checks if any tentative blocks can now be promoted.
func (node *QuidnugNode) ReEvaluateTentativeBlocks(domain string) error {
	node.TentativeBlocksMutex.Lock()
	blocks, exists := node.TentativeBlocks[domain]
	if !exists || len(blocks) == 0 {
		node.TentativeBlocksMutex.Unlock()
		return nil
	}
	// Take ownership and clear
	node.TentativeBlocks[domain] = nil
	node.TentativeBlocksMutex.Unlock()

	var remaining []Block
	for _, block := range blocks {
		acceptance := node.ValidateBlockTiered(block)
		switch acceptance {
		case BlockTrusted:
			node.BlockchainMutex.Lock()
			node.Blockchain = append(node.Blockchain, block)
			node.BlockchainMutex.Unlock()

			node.processBlockTransactions(block)

			node.TrustDomainsMutex.Lock()
			if d, exists := node.TrustDomains[block.TrustProof.TrustDomain]; exists {
				d.BlockchainHead = block.Hash
				node.TrustDomains[block.TrustProof.TrustDomain] = d
			}
			node.TrustDomainsMutex.Unlock()

			edges := node.ExtractTrustEdgesFromBlock(block, true)
			for _, edge := range edges {
				node.PromoteTrustEdge(edge.Truster, edge.Trustee)
			}

			if logger != nil {
				logger.Info("Promoted tentative block to trusted",
					"blockIndex", block.Index,
					"hash", block.Hash,
					"domain", domain)
			}

		case BlockTentative:
			remaining = append(remaining, block)

		case BlockUntrusted, BlockInvalid:
			if logger != nil {
				logger.Info("Removed tentative block",
					"blockIndex", block.Index,
					"hash", block.Hash,
					"domain", domain,
					"newStatus", acceptance)
			}
		}
	}

	if len(remaining) > 0 {
		node.TentativeBlocksMutex.Lock()
		node.TentativeBlocks[domain] = remaining
		node.TentativeBlocksMutex.Unlock()
	}

	return nil
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

