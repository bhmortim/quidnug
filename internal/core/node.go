package core

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/quidnug/quidnug/internal/config"
	"github.com/quidnug/quidnug/internal/ipfsclient"
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
//  11. DomainRegistryMutex   - Protects DomainRegistry map (reverse index of domain->nodes)
//  12. GossipSeenMutex       - Protects GossipSeen map (deduplication of gossip messages)
//  13. TrustCache (internal) - Has its own mutex, can be accessed independently
//
// Guidelines:
//   - Prefer acquiring a single lock when possible
//   - Release locks as soon as the protected data is no longer needed
//   - Use RLock for read-only access to enable concurrent readers
//   - Never call external code (HTTP requests, etc.) while holding a lock
//   - When computing trust (ComputeRelationalTrust), only TrustRegistryMutex is held briefly for reads

// Package-level logger. Initialized to a safe default so that code called
// outside main() (unit tests, library use) never panics on a nil logger.
// initLogger replaces it with the configured handler during startup.
var logger = slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
	Level: slog.LevelInfo,
}))

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
	NodeID       string
	PrivateKey   *ecdsa.PrivateKey
	PublicKey    *ecdsa.PublicKey
	Blockchain   []Block
	PendingTxs   []interface{}
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

	// QDP-0014: per-node discovery registry. Owns its own
	// internal lock; no QuidnugNode-level mutex needed.
	NodeAdvertisementRegistry *NodeAdvertisementRegistry

	// IPFS client
	IPFSClient ipfsclient.IPFSClient

	// HTTP server for graceful shutdown
	Server *http.Server

	// HTTP client for network communication
	httpClient *http.Client

	// Domain registry - reverse index from domain to node IDs for efficient lookup
	DomainRegistry map[string][]string

	// Mutexes for thread safety
	BlockchainMutex       sync.RWMutex
	PendingTxsMutex       sync.RWMutex
	KnownNodesMutex       sync.RWMutex
	TrustDomainsMutex     sync.RWMutex
	TrustRegistryMutex    sync.RWMutex
	IdentityRegistryMutex sync.RWMutex
	TitleRegistryMutex    sync.RWMutex
	EventStreamMutex      sync.RWMutex
	DomainRegistryMutex   sync.RWMutex

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

	// Domain restriction configuration
	SupportedDomains        []string // Empty = all domains allowed
	AllowDomainRegistration bool     // Whether dynamic domain registration is permitted
	RequireParentDomainAuth bool     // Whether subdomains require parent validator authorization

	// Trust computation cache
	TrustCache *TrustCache

	// Gossip protocol state
	GossipSeen      map[string]int64 // messageId -> timestamp of when seen
	GossipSeenMutex sync.RWMutex
	GossipTTL       int // Default TTL for outgoing gossip messages

	// QDP-0001 global nonce ledger (see ledger.go). Always allocated;
	// enforcement is gated on config.Config.EnableNonceLedger. In shadow mode
	// the ledger is still updated from block checkpoints so an operator
	// can flip the flag without a full ledger rebuild.
	NonceLedger        *NonceLedger
	NonceLedgerEnforce bool

	// QDP-0005 push-based gossip (H1). When PushGossipEnabled is
	// true the node both emits push messages on fresh anchors /
	// fingerprints and accepts them on the /api/v2/gossip/push-*
	// endpoints. Default false; rolls out as a shadow flag.
	// gossipRate is the per-producer token-bucket limiter,
	// lazily allocated on first use. gossipRateMutex protects
	// the lazy initialization itself; the state has its own
	// internal mutex for the hot path.
	PushGossipEnabled bool
	gossipRate        *gossipRateState
	gossipRateMutex   sync.Mutex

	// QDP-0007 lazy epoch propagation (H4). LazyEpochProbeEnabled
	// gates the quarantine + probe behavior. Recency window,
	// probe timeout, and timeout policy are deliberately settable
	// per-node (not just config-loaded) so tests can override
	// without LoadConfig round-trips.
	LazyEpochProbeEnabled bool
	EpochRecencyWindow    time.Duration
	EpochProbeTimeout     time.Duration
	ProbeTimeoutPolicy    string
	quarantine            *quarantineState

	// QDP-0008 snapshot K-of-K bootstrap (H3). Registry of
	// in-flight and completed bootstrap sessions. Lazily
	// allocated on first bootstrap call.
	bootstrap *bootstrapRegistry

	// QDP-0009 fork-block migration trigger (H5). Tracks
	// pending and active fork-block transactions. Feature
	// activation at ForkHeight flips the corresponding node
	// flag deterministically across the network.
	forks *forkRegistry

	// QDP-0010 / H2: when true (activated via QDP-0009 fork
	// for `require_tx_tree_root`), incoming blocks with empty
	// TransactionsRoot are rejected. Before activation the
	// field is optional and producers emit it in shadow mode.
	RequireTxTreeRoot bool
}

// Run starts the Quidnug node's main loop: loads configuration, initializes
// the logger, constructs the node, kicks off background goroutines, and
// blocks until the process receives a shutdown signal or the server fails.
// It is the single public entry point called from cmd/quidnug/main.go.
func Run() {
	// Load configuration
	cfg := config.LoadConfig()

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

	// Start domain gossip loop (with context)
	wg.Add(1)
	go func() {
		defer wg.Done()
		quidnugNode.runDomainGossip(ctx, cfg.DomainGossipInterval)
	}()

	// Start tentative-block GC loop (QDP-0001 §6.4)
	wg.Add(1)
	go func() {
		defer wg.Done()
		quidnugNode.runTentativeBlockGC(ctx, DefaultTentativeGCInterval, DefaultTentativeBlockMaxAge)
	}()

	// Start HTTP server (non-blocking)
	serverErr := make(chan error, 1)
	wg.Add(1)
	go func() {
		defer wg.Done()
		if err := quidnugNode.StartServerWithConfig(cfg.Port, cfg.RateLimitPerMinute, cfg.MaxBodySizeBytes); err != nil && err != http.ErrServerClosed {
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
func (node *QuidnugNode) Shutdown(ctx context.Context, cfg *config.Config) {
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

// NewQuidnugNode initializes a new quidnug node.
//
// When `cfg` is nil (typical in tests), the defaults applied here must
// match config.LoadConfig's defaults — otherwise tests behave differently
// from production. A previous version only set a handful of fields,
// which left AllowDomainRegistration=false and silently broke every
// test that relied on RegisterTrustDomain.
func NewQuidnugNode(cfg *config.Config) (*QuidnugNode, error) {
	if cfg == nil {
		cfg = &config.Config{
			IPFSEnabled:             config.DefaultIPFSEnabled,
			IPFSGatewayURL:          config.DefaultIPFSGatewayURL,
			IPFSTimeout:             config.DefaultIPFSTimeout,
			TrustCacheTTL:           config.DefaultTrustCacheTTL,
			AllowDomainRegistration: config.DefaultAllowDomainRegistration,
			RequireParentDomainAuth: config.DefaultRequireParentDomainAuth,
			DomainGossipInterval:    config.DefaultDomainGossipInterval,
			DomainGossipTTL:         config.DefaultDomainGossipTTL,
		}
	}
	// Ensure TrustCacheTTL has a valid value
	if cfg.TrustCacheTTL <= 0 {
		cfg.TrustCacheTTL = config.DefaultTrustCacheTTL
	}
	// Ensure DomainGossipTTL has a valid value
	if cfg.DomainGossipTTL <= 0 {
		cfg.DomainGossipTTL = config.DefaultDomainGossipTTL
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
	var ipfsClient ipfsclient.IPFSClient
	if cfg.IPFSEnabled {
		ipfsClient = ipfsclient.NewHTTPIPFSClient(cfg.IPFSGatewayURL, &http.Client{Timeout: cfg.IPFSTimeout})
	} else {
		ipfsClient = &ipfsclient.NoOpIPFSClient{}
	}

	node := &QuidnugNode{
		NodeID:                    nodeID,
		PrivateKey:                privateKey,
		PublicKey:                 &privateKey.PublicKey,
		Blockchain:                []Block{genesisBlock},
		PendingTxs:                []interface{}{},
		TrustDomains:              make(map[string]TrustDomain),
		KnownNodes:                make(map[string]Node),
		TrustRegistry:             make(map[string]map[string]float64),
		TrustNonceRegistry:        make(map[string]map[string]int64),
		IdentityRegistry:          make(map[string]IdentityTransaction),
		TitleRegistry:             make(map[string]TitleTransaction),
		EventStreamRegistry:       make(map[string]*EventStream),
		EventRegistry:             make(map[string][]EventTransaction),
		NodeAdvertisementRegistry: NewNodeAdvertisementRegistry(),
		IPFSClient:                ipfsClient,
		TentativeBlocks:           make(map[string][]Block),
		VerifiedTrustEdges:        make(map[string]map[string]TrustEdge),
		UnverifiedTrustRegistry:   make(map[string]map[string]TrustEdge),
		DomainRegistry:            make(map[string][]string),
		DistrustThreshold:         0.0,
		TransactionTrustThreshold: 0.0,
		SupportedDomains:          cfg.SupportedDomains,
		AllowDomainRegistration:   cfg.AllowDomainRegistration,
		RequireParentDomainAuth:   cfg.RequireParentDomainAuth,
		TrustCache:                NewTrustCache(cfg.TrustCacheTTL),
		GossipSeen:                make(map[string]int64),
		GossipTTL:                 cfg.DomainGossipTTL,
		NonceLedger:               NewNonceLedger(),
		NonceLedgerEnforce:        cfg.EnableNonceLedger,
		PushGossipEnabled:         cfg.EnablePushGossip,
		LazyEpochProbeEnabled:     cfg.EnableLazyEpochProbe,
		EpochRecencyWindow:        cfg.EpochRecencyWindow,
		EpochProbeTimeout:         cfg.EpochProbeTimeout,
		ProbeTimeoutPolicy:        cfg.ProbeTimeoutPolicy,
		quarantine:                newQuarantineState(),
		forks:                     newForkRegistry(),
		httpClient: &http.Client{
			Timeout: 5 * time.Second,
		},
	}

	// Seed the node's own epoch-0 signing key into the ledger so that a
	// self-rotation anchor can be verified. Other signers' keys are
	// seeded as their identity transactions land (see
	// updateIdentityRegistry) or via explicit SetSignerKey calls during
	// migration from a snapshot.
	node.NonceLedger.SetSignerKey(nodeID, 0, publicKeyHex)

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
// Transactions are included if trustLevel >= node.TransactionTrustThreshold.
