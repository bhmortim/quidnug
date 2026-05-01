package core

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/quidnug/quidnug/internal/audit"
	"github.com/quidnug/quidnug/internal/config"
	"github.com/quidnug/quidnug/internal/ipfsclient"
	"github.com/quidnug/quidnug/internal/ratelimit"
	"github.com/quidnug/quidnug/internal/safeio"
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

	// OperatorQuid is the long-lived identity of the human or
	// organization that runs this node. It is intentionally
	// separate from NodeID/PrivateKey above: NodeID identifies
	// THIS process for gossip dedup and fork detection, while
	// the operator quid is the identity that accumulates
	// transferable trust. One operator quid can run on N nodes
	// simultaneously — each of those nodes still has its own
	// NodeID, but they all advertise the same OperatorQuid.
	//
	// Trust granted to OperatorQuid in some trust domain (via
	// TRUST transactions) is what authorizes this node to
	// process transactions in that domain. The same TRUST grant
	// applies regardless of which of the operator's nodes a
	// counterparty interacts with — that's the QDP-0001 +
	// QDP-0014 transferable-trust property.
	//
	// Loaded from cfg.OperatorQuidFile (a .quid.json file as
	// emitted by `quidnug-cli quid generate`). Empty when no
	// operator file is configured (typical in tests and
	// short-lived demos); the node still works but
	// counterparties have no trust anchor that survives
	// restart, and an operator running multiple nodes cannot
	// pool trust across them.
	OperatorQuidID           string
	OperatorQuidPublicKeyHex string
	OperatorQuidPrivateKey   *ecdsa.PrivateKey // nil when only public-key file was loaded
	OperatorQuidFile         string            // source path, for landing-page diagnostics

	// PrivateAddrAllowList is the per-peer override set consulted
	// by safeDialContext. Operators populate it implicitly by
	// listing peers in peers_file with `allow_private: true`, or
	// by enabling LAN discovery (mDNS-found peers are added
	// automatically). The allow-list lets a node dial specific
	// 192.168.x.x peers without opening the dial filter for
	// every private range globally.
	PrivateAddrAllowList *PrivateAddrAllowList

	// PeerAdmit is the snapshot of admit-pipeline thresholds
	// captured at boot. Used by every peer source (gossip,
	// static, mDNS) so the gating policy is uniform.
	PeerAdmit PeerAdmitConfig

	// PeerScoreboard is the per-peer quality-scoring system
	// (Phase 4). Every interaction with a peer (handshake,
	// gossip post, query, broadcast, validation outcome)
	// nudges the score; quarantine + eviction policies and
	// routing preference consult it. Persists across restarts
	// via the snapshot loop in Run().
	PeerScoreboard *PeerScoreboard

	// State registries
	TrustRegistry      map[string]map[string]float64
	TrustNonceRegistry map[string]map[string]int64
	// TrustExpiryRegistry tracks the TRUST edge's ValidUntil
	// timestamp (Unix seconds). Zero means no expiry.
	// Guarded by the same TrustRegistryMutex as TrustRegistry.
	// See QDP-0022.
	TrustExpiryRegistry map[string]map[string]int64
	// TrustEdgeTimestampRegistry tracks the last-refreshed
	// timestamp (Unix seconds) for each trust edge. Populated
	// from the TRUST transaction's Timestamp at admission.
	// Used by QDP-0019 decay computation. Guarded by
	// TrustRegistryMutex. Zero = no timestamp known (treat as
	// "decay disabled" for that edge).
	TrustEdgeTimestampRegistry map[string]map[string]int64
	IdentityRegistry   map[string]IdentityTransaction
	TitleRegistry      map[string]TitleTransaction

	// Event registries
	EventStreamRegistry map[string]*EventStream
	EventRegistry       map[string][]EventTransaction

	// QDP-0014: per-node discovery registry. Owns its own
	// internal lock; no QuidnugNode-level mutex needed.
	NodeAdvertisementRegistry *NodeAdvertisementRegistry

	// QDP-0015: moderation registry. Owns its own internal
	// lock; no QuidnugNode-level mutex needed.
	ModerationRegistry *ModerationRegistry

	// QDP-0017: privacy registry (consent / restriction / DSR).
	// Owns its own internal lock.
	PrivacyRegistry *PrivacyRegistry

	// QDP-0023: DNS attestation registry. Owns its own
	// internal lock.
	DNSAttestationRegistry *DNSAttestationRegistry

	// QDP-0021: blind-key attestation registry. Owns its own
	// internal lock.
	BlindKeyRegistry *BlindKeyRegistry

	// QDP-0024: group encryption registry. Owns its own
	// internal lock.
	GroupRegistry *GroupRegistry

	// QDP-0016: multi-layer write-admission rate limiter. Fires
	// at the mempool-admission layer, after signature verification.
	// The existing HTTP-ingress IP limiter (configured via
	// middleware.go) is independent and complementary.
	WriteLimiter *ratelimit.MultiLayerLimiter

	// QDP-0018 Phase 1: tamper-evident operator audit log. Owns
	// its own internal lock. Nil if the operator has disabled
	// audit logging via config.
	AuditLog *audit.Log

	// QDP-0014: per-(domain, quid) activity index, populated
	// incrementally as blocks commit.
	QuidDomainIndex *QuidDomainIndex

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

	// Discover other nodes via seeds (with context)
	wg.Add(1)
	go func() {
		defer wg.Done()
		quidnugNode.DiscoverNodes(ctx, cfg.SeedNodes)
	}()

	// Static peers from operator-managed peers_file. Idempotent
	// no-op when cfg.PeersFile is empty.
	wg.Add(1)
	go func() {
		defer wg.Done()
		quidnugNode.runStaticPeerLoop(ctx, cfg.PeersFile, quidnugNode.PeerAdmit)
	}()

	// LAN discovery via mDNS. Off by default; opt-in for home,
	// office, and lab deployments. Idempotent no-op when
	// cfg.LANDiscovery is false.
	wg.Add(1)
	go func() {
		defer wg.Done()
		port, _ := strconv.Atoi(cfg.Port)
		quidnugNode.runLANPeerLoop(
			ctx,
			cfg.LANDiscovery,
			cfg.LANServiceName,
			port,
			quidnugNode.PeerAdmit,
		)
	}()

	// Peer-scoreboard persistence: snapshot every
	// cfg.PeerScorePersistInterval (default 5m) so reputation
	// survives restart. No-op when peerScoreboardPath returns
	// empty (DataDir unset, typical in tests).
	wg.Add(1)
	go func() {
		defer wg.Done()
		quidnugNode.runPeerScorePersistLoop(ctx, peerScoreboardPath(cfg), cfg.PeerScorePersistInterval)
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
	// Generate a fresh ECDSA keypair for this node process. The
	// node's identity (NodeID) is intentionally distinct from
	// the operator's quid (loaded below if configured): each
	// running node must have its own NodeID so peer discovery,
	// gossip dedup, and fork detection work correctly even when
	// one operator runs N nodes under a single quid. Two nodes
	// sharing a NodeID would collide on those mechanisms.
	privateKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, fmt.Errorf("failed to generate key pair: %v", err)
	}

	// Generate a node ID based on the public key
	publicKeyBytes := elliptic.Marshal(privateKey.PublicKey.Curve, privateKey.PublicKey.X, privateKey.PublicKey.Y)
	nodeID := fmt.Sprintf("%x", sha256.Sum256(publicKeyBytes))[:16]

	// Load the operator quid if one is configured. The operator
	// quid is the long-lived identity this node operates under;
	// see the OperatorQuid* fields on QuidnugNode for the
	// rationale. A load failure here is fatal: misconfigured
	// operator identity is something operators want to know
	// about at boot, not silently treated as ephemeral.
	var (
		operatorQuidID           string
		operatorQuidPublicHex    string
		operatorQuidPrivateKey   *ecdsa.PrivateKey
	)
	if cfg.OperatorQuidFile != "" {
		op, err := loadOperatorQuid(cfg.OperatorQuidFile)
		if err != nil {
			return nil, fmt.Errorf("load operator quid from %q: %w", cfg.OperatorQuidFile, err)
		}
		operatorQuidID = op.ID
		operatorQuidPublicHex = op.PublicKeyHex
		operatorQuidPrivateKey = op.PrivateKey
	}

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
		OperatorQuidID:            operatorQuidID,
		OperatorQuidPublicKeyHex:  operatorQuidPublicHex,
		OperatorQuidPrivateKey:    operatorQuidPrivateKey,
		OperatorQuidFile:          cfg.OperatorQuidFile,
		PrivateKey:                privateKey,
		PublicKey:                 &privateKey.PublicKey,
		Blockchain:                []Block{genesisBlock},
		PendingTxs:                []interface{}{},
		TrustDomains:              make(map[string]TrustDomain),
		KnownNodes:                make(map[string]Node),
		TrustRegistry:                 make(map[string]map[string]float64),
		TrustNonceRegistry:            make(map[string]map[string]int64),
		TrustExpiryRegistry:           make(map[string]map[string]int64),
		TrustEdgeTimestampRegistry:    make(map[string]map[string]int64),
		IdentityRegistry:          make(map[string]IdentityTransaction),
		TitleRegistry:             make(map[string]TitleTransaction),
		EventStreamRegistry:       make(map[string]*EventStream),
		EventRegistry:             make(map[string][]EventTransaction),
		NodeAdvertisementRegistry: NewNodeAdvertisementRegistry(),
		ModerationRegistry:        NewModerationRegistry(),
		PrivacyRegistry:           NewPrivacyRegistry(),
		DNSAttestationRegistry:    NewDNSAttestationRegistry(),
		BlindKeyRegistry:          NewBlindKeyRegistry(),
		GroupRegistry:             NewGroupRegistry(),
		WriteLimiter:              ratelimit.NewMultiLayerLimiter(ratelimit.DefaultWriteLimits()),
		QuidDomainIndex:           NewQuidDomainIndex(),
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
		PrivateAddrAllowList: NewPrivateAddrAllowList(),
		PeerAdmit: PeerAdmitConfig{
			RequireAdvertisement:  cfg.RequireAdvertisement,
			MinOperatorTrust:      cfg.PeerMinOperatorTrust,
			MinOperatorReputation: cfg.PeerMinOperatorReputation,
			HandshakeTimeout:      5 * time.Second,
		},
		PeerScoreboard: NewPeerScoreboard(
			DefaultPeerScoreWeights(),
			peerScoreboardPath(cfg),
			cfg.PeerScorePersistInterval,
		),
	}
	// SSRF defense: reject loopback, private, link-local, and
	// metadata-IP destinations at dial time so peer-advertised
	// addresses can't trick the node into querying its own
	// infrastructure. The closure captures the per-node allow-list
	// so peers explicitly listed in peers_file (or learned via
	// mDNS) can still be dialed even if they sit in an otherwise-
	// blocked range. See safedial.go.
	node.httpClient = &http.Client{
		Timeout: 5 * time.Second,
		Transport: &http.Transport{
			DialContext:           NewSafeDialContext(node.PrivateAddrAllowList),
			MaxIdleConns:          100,
			IdleConnTimeout:       90 * time.Second,
			TLSHandshakeTimeout:   10 * time.Second,
			ExpectContinueTimeout: 1 * time.Second,
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

	// QDP-0018 Phase 1: initialize the operator audit log.
	// If the config supplies a path, open a disk-backed store;
	// otherwise fall back to an in-memory-only log. Failure to
	// open the disk store is not fatal — we log and continue
	// with an in-memory log so a misconfigured audit path can't
	// prevent the node from starting.
	if cfg.AuditLogPath != "" {
		resolvedPath := cfg.AuditLogPath
		if !filepath.IsAbs(resolvedPath) && cfg.DataDir != "" {
			resolvedPath = filepath.Join(cfg.DataDir, resolvedPath)
		}
		store, err := audit.NewFileStore(resolvedPath)
		if err != nil {
			logger.Warn("Failed to open audit log file, falling back to in-memory log",
				"path", resolvedPath, "err", err)
			node.AuditLog = audit.NewLog(nodeID)
		} else if log, err := audit.NewLogWithStore(nodeID, store); err != nil {
			logger.Warn("Failed to replay audit log file, falling back to in-memory log",
				"path", resolvedPath, "err", err)
			_ = store.Close()
			node.AuditLog = audit.NewLog(nodeID)
		} else {
			node.AuditLog = log
			logger.Info("Opened operator audit log",
				"path", resolvedPath, "height", log.Height())
		}
	} else {
		node.AuditLog = audit.NewLog(nodeID)
	}

	// Record the node-lifecycle event so external auditors can
	// see every startup in the log.
	node.emitAudit(audit.CategoryNodeLifecycle, map[string]interface{}{
		"event":   "node_start",
		"node_id": nodeID,
	}, "node initialized")

	// Rehydrate peer scores from disk if a previous run wrote
	// peer_scores.json. Missing file is fine.
	if node.PeerScoreboard != nil {
		if err := node.PeerScoreboard.LoadFrom(peerScoreboardPath(cfg)); err != nil {
			logger.Warn("Failed to load peer scoreboard", "error", err)
		}
	}

	if logger != nil {
		fields := []any{"nodeId", nodeID}
		if operatorQuidID != "" {
			fields = append(fields,
				"operatorQuid", operatorQuidID,
				"operatorQuidHasPrivateKey", operatorQuidPrivateKey != nil,
				"operatorQuidFile", cfg.OperatorQuidFile)
		} else {
			fields = append(fields, "operatorQuid", "(none — ephemeral node)")
		}
		logger.Info("Initialized quidnug node", fields...)
	}
	return node, nil
}

// peerScoreboardPath resolves the on-disk location for the peer
// scoreboard JSON snapshot. Empty when DataDir is empty (in-
// memory-only operation, fine for tests).
func peerScoreboardPath(cfg *config.Config) string {
	if cfg == nil || cfg.DataDir == "" {
		return ""
	}
	return filepath.Join(cfg.DataDir, "peer_scores.json")
}

// operatorQuidFile is the on-disk format produced by
// `quidnug-cli quid generate`. Mirrored here so the node can
// load the same file without depending on the CLI package.
type operatorQuidFile struct {
	ID            string `json:"id"`
	PublicKeyHex  string `json:"publicKeyHex"`
	PrivateKeyHex string `json:"privateKeyHex,omitempty"`
}

// loadedOperatorQuid is the in-memory result of loading an
// operator quid file. PrivateKey is nil when the file contained
// only a public key (a node operating under a quid the operator
// keeps offline; the node can display the identity but cannot
// sign on the operator's behalf).
type loadedOperatorQuid struct {
	ID           string
	PublicKeyHex string
	PrivateKey   *ecdsa.PrivateKey
}

// loadOperatorQuid reads and validates a .quid.json file at the
// given path. The path is treated as untrusted-input and goes
// through safeio.ReadFile (rejects path traversal, NUL bytes,
// symlinks, non-regular files). The decoded ID is cross-checked
// against the public key so a tampered ID can't impersonate
// another quid.
func loadOperatorQuid(path string) (*loadedOperatorQuid, error) {
	raw, err := safeio.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var qf operatorQuidFile
	if err := json.Unmarshal(raw, &qf); err != nil {
		return nil, fmt.Errorf("parse: %w", err)
	}
	if qf.PublicKeyHex == "" {
		return nil, fmt.Errorf("missing publicKeyHex")
	}
	pubKey, err := decodeOperatorPublicKey(qf.PublicKeyHex)
	if err != nil {
		return nil, fmt.Errorf("publicKeyHex: %w", err)
	}
	// Cross-check ID = sha256(publicKey)[:16] using the same
	// derivation NodeID uses (so the on-wire identifier shape
	// is consistent).
	pubBytes, err := hex.DecodeString(qf.PublicKeyHex)
	if err != nil {
		return nil, fmt.Errorf("publicKeyHex not hex: %w", err)
	}
	wantID := fmt.Sprintf("%x", sha256.Sum256(pubBytes))[:16]
	if qf.ID != "" && qf.ID != wantID {
		return nil, fmt.Errorf("id %q does not match sha256(publicKey)[:16] = %q", qf.ID, wantID)
	}
	out := &loadedOperatorQuid{
		ID:           wantID,
		PublicKeyHex: qf.PublicKeyHex,
	}
	if qf.PrivateKeyHex != "" {
		priv, err := decodeOperatorPrivateKey(qf.PrivateKeyHex, pubKey)
		if err != nil {
			return nil, fmt.Errorf("privateKeyHex: %w", err)
		}
		out.PrivateKey = priv
	}
	// Permissions sanity check on POSIX. Skip on Windows where
	// the perm bits don't reflect actual ACL state.
	if runtime.GOOS != "windows" {
		if st, err := os.Stat(path); err == nil {
			if mode := st.Mode().Perm(); mode&0o077 != 0 {
				logger.Warn("Operator quid file is group/world-readable; recommend chmod 600",
					"path", path, "mode", fmt.Sprintf("%o", mode))
			}
		}
	}
	return out, nil
}

// decodeOperatorPublicKey decodes a hex-encoded SEC1 uncompressed
// P-256 public key into an *ecdsa.PublicKey. Format matches what
// `pkg/client.Quid.PublicKeyHex` produces.
func decodeOperatorPublicKey(hexStr string) (*ecdsa.PublicKey, error) {
	raw, err := hex.DecodeString(hexStr)
	if err != nil {
		return nil, fmt.Errorf("not hex: %w", err)
	}
	curve := elliptic.P256()
	x, y := elliptic.Unmarshal(curve, raw)
	if x == nil || y == nil {
		return nil, fmt.Errorf("invalid SEC1 uncompressed P-256 point")
	}
	return &ecdsa.PublicKey{Curve: curve, X: x, Y: y}, nil
}

// decodeOperatorPrivateKey decodes a hex-encoded PKCS8 DER
// (the format pkg/client.Quid emits) into an *ecdsa.PrivateKey
// and verifies it produces the supplied public key. Refusing to
// load a key whose public half doesn't match closes a common
// config-mistake hole (operator paired the wrong files together).
func decodeOperatorPrivateKey(hexStr string, expectedPub *ecdsa.PublicKey) (*ecdsa.PrivateKey, error) {
	derBytes, err := hex.DecodeString(hexStr)
	if err != nil {
		return nil, fmt.Errorf("not hex: %w", err)
	}
	keyAny, err := x509.ParsePKCS8PrivateKey(derBytes)
	if err != nil {
		return nil, fmt.Errorf("PKCS8 parse: %w", err)
	}
	priv, ok := keyAny.(*ecdsa.PrivateKey)
	if !ok {
		return nil, fmt.Errorf("not an ECDSA private key (got %T)", keyAny)
	}
	if priv.Curve != elliptic.P256() {
		return nil, fmt.Errorf("not P-256")
	}
	if priv.PublicKey.X.Cmp(expectedPub.X) != 0 || priv.PublicKey.Y.Cmp(expectedPub.Y) != 0 {
		return nil, fmt.Errorf("private scalar does not match publicKeyHex")
	}
	return priv, nil
}

// SetHTTPClientTimeout configures the HTTP client timeout
func (node *QuidnugNode) SetHTTPClientTimeout(timeout time.Duration) {
	node.httpClient.Timeout = timeout
}

// AddTrustTransaction adds a trust transaction to the pending pool
// Transactions are included if trustLevel >= node.TransactionTrustThreshold.
