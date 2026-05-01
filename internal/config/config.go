package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/quidnug/quidnug/internal/safeio"
	"gopkg.in/yaml.v3"
)

// Config holds the application configuration
type Config struct {
	Port                    string        `json:"port" yaml:"port"`
	SeedNodes               []string      `json:"seedNodes" yaml:"seed_nodes"`
	LogLevel                string        `json:"logLevel" yaml:"log_level"`
	BlockInterval           time.Duration `json:"blockInterval" yaml:"-"`
	RateLimitPerMinute      int           `json:"rateLimitPerMinute" yaml:"rate_limit_per_minute"`
	MaxBodySizeBytes        int64         `json:"maxBodySizeBytes" yaml:"max_body_size_bytes"`
	DataDir                 string        `json:"dataDir" yaml:"data_dir"`
	ShutdownTimeout         time.Duration `json:"shutdownTimeout" yaml:"-"`
	HTTPClientTimeout       time.Duration `json:"httpClientTimeout" yaml:"-"`
	NodeAuthSecret          string        `json:"nodeAuthSecret" yaml:"node_auth_secret"`
	RequireNodeAuth         bool          `json:"requireNodeAuth" yaml:"require_node_auth"`
	IPFSEnabled             bool          `json:"ipfsEnabled" yaml:"ipfs_enabled"`
	IPFSGatewayURL          string        `json:"ipfsGatewayUrl" yaml:"ipfs_gateway_url"`
	IPFSTimeout             time.Duration `json:"ipfsTimeout" yaml:"-"`
	SupportedDomains        []string      `json:"supportedDomains" yaml:"supported_domains"`
	AllowDomainRegistration bool          `json:"allowDomainRegistration" yaml:"allow_domain_registration"`
	RequireParentDomainAuth bool          `json:"requireParentDomainAuth" yaml:"require_parent_domain_auth"`
	TrustCacheTTL           time.Duration `json:"trustCacheTTL" yaml:"-"`
	DomainGossipInterval    time.Duration `json:"domainGossipInterval" yaml:"-"`
	DomainGossipTTL         int           `json:"domainGossipTTL" yaml:"domain_gossip_ttl"`

	// QDP-0001 nonce-ledger feature flag. When false (default), the
	// ledger is populated and observed in shadow mode but does not
	// reject transactions. When true, ledger-level replay/reservation/
	// gap violations cause transaction admission to fail. Phase-0 →
	// Phase-1 switch per QDP-0001 §10.
	EnableNonceLedger bool `json:"enableNonceLedger" yaml:"enable_nonce_ledger"`

	// QDP-0005 push-gossip feature flag (H1). When false (default),
	// the node operates on pull-only gossip. When true, fresh
	// anchors and fingerprints are fanned out on the
	// /api/v2/gossip/push-* endpoints and the node accepts
	// inbound pushes. Can be flipped independently of
	// EnableNonceLedger; push gossip operates in shadow mode for
	// the first two weeks of v2.3 before defaulting on.
	EnablePushGossip bool `json:"enablePushGossip" yaml:"enable_push_gossip"`

	// QDP-0007 lazy epoch propagation (H4). When true, any
	// transaction from a signer whose local epoch state is older
	// than EpochRecencyWindow is quarantined pending a probe
	// against the signer's home domain. Default false — new
	// behavior that operators opt into. Complements EnablePushGossip
	// rather than replacing it.
	EnableLazyEpochProbe bool          `json:"enableLazyEpochProbe" yaml:"enable_lazy_epoch_probe"`
	EpochRecencyWindow   time.Duration `json:"epochRecencyWindow" yaml:"-"`
	EpochProbeTimeout    time.Duration `json:"epochProbeTimeout" yaml:"-"`
	ProbeTimeoutPolicy   string        `json:"probeTimeoutPolicy" yaml:"probe_timeout_policy"`

	// QDP-0018 tamper-evident operator audit log.
	//
	// AuditLogPath, if set, points at a file the node opens in
	// append-only mode to persist audit entries. Empty means the
	// log is in-memory only (fine for dev + tests, not recommended
	// for production). Relative paths resolve against DataDir.
	AuditLogPath string `json:"auditLogPath" yaml:"audit_log_path"`

	// OperatorQuidFile is the path to a `.quid.json` file (the
	// format `quidnug-cli quid generate` emits) whose private key
	// the node uses as its signing identity. When set, the node
	// loads this file on startup instead of generating a fresh
	// ephemeral keypair, which makes the node's quid stable
	// across restarts AND lets one operator run multiple nodes
	// under a single shared identity. Trust granted to an
	// operator's quid then accumulates against the operator
	// regardless of which physical node a counterparty interacts
	// with — that's the QDP-0001 "transferable trust" property.
	//
	// Empty means generate ephemeral (existing behavior; fine
	// for tests and short-lived demos, not recommended for any
	// node intended to accumulate reputation).
	//
	// The file MUST contain a privateKeyHex; loading a public-
	// only quid file is rejected because the node needs the key
	// to sign. Permissions on the file should be 0600; the node
	// warns if it's world-readable.
	//
	// Environment variable: OPERATOR_QUID_FILE
	OperatorQuidFile string `json:"operatorQuidFile" yaml:"operator_quid_file"`

	// --- Peering ---------------------------------------------------------
	//
	// PeersFile is a path to a YAML/JSON file enumerating peers this
	// node should consider known at boot, in addition to anything
	// learned from seed_nodes gossip discovery. Reload on file change
	// (fsnotify) so operators can add/remove peers without restart.
	//
	// File schema (YAML):
	//
	//   peers:
	//     - address: "node2.example.com:8080"
	//       operator_quid: "034bc467852ffa94"   # optional: pin operator
	//       allow_private: false                # default false
	//     - address: "192.168.1.50:8080"
	//       operator_quid: "feedfacedeadbeef"
	//       allow_private: true                 # bypass safedial for this peer
	//
	// `allow_private: true` is the explicit escape hatch for LAN
	// peers. It is honored ONLY for entries the operator wrote into
	// peers_file (or that arrive via mDNS discovery, see LANDiscovery
	// below). It is NOT honored for peers learned from gossip.
	//
	// Environment variable: PEERS_FILE
	PeersFile string `json:"peersFile" yaml:"peers_file"`

	// LANDiscovery enables mDNS / DNS-SD service-type
	// `_quidnug._tcp.local.` so nodes on the same broadcast domain
	// can find each other without configuration. Off by default;
	// opt in for home/office/lab deployments. Peers found via mDNS
	// are admitted with allow_private semantics so the dial
	// filter doesn't refuse 192.168.x.x destinations.
	//
	// Environment variable: LAN_DISCOVERY
	LANDiscovery bool `json:"lanDiscovery" yaml:"lan_discovery"`

	// LANServiceName is the mDNS service type advertised + browsed.
	// Defaults to "_quidnug._tcp" when empty.
	//
	// Environment variable: LAN_SERVICE_NAME
	LANServiceName string `json:"lanServiceName" yaml:"lan_service_name"`

	// RequireAdvertisement gates whether a peer learned via gossip
	// must have a current NodeAdvertisementTransaction (QDP-0014)
	// in this node's registry to be admitted. Default true in
	// production deployments. Set false in dev when peers are
	// running ephemeral identities without yet publishing an ad.
	//
	// Environment variable: REQUIRE_ADVERTISEMENT
	RequireAdvertisement bool `json:"requireAdvertisement" yaml:"require_advertisement"`

	// PeerMinOperatorTrust is the minimum TRUST-edge weight from
	// OperatorQuid → NodeQuid required to admit a peer. Floors at
	// 0; defaults to 0.5 (matches QDP-0014's MinOperatorTrustWeight).
	// Tighten to 0.7+ for production peerings; loosen to 0 to
	// disable the operator-attestation gate entirely (not
	// recommended).
	//
	// Environment variable: PEER_MIN_OPERATOR_TRUST
	PeerMinOperatorTrust float64 `json:"peerMinOperatorTrust" yaml:"peer_min_operator_trust"`

	// PeerMinOperatorReputation is an OPTIONAL second gate: if > 0,
	// the candidate peer's OperatorQuid must have an incoming
	// TRUST edge from at least one quid this node already trusts,
	// at the named weight or higher. Default 0 (gate off). Set to
	// e.g. 0.3 to enforce "I only peer with operators my friends
	// trust."
	//
	// Environment variable: PEER_MIN_OPERATOR_REPUTATION
	PeerMinOperatorReputation float64 `json:"peerMinOperatorReputation" yaml:"peer_min_operator_reputation"`

	// PeerReattestationInterval is how often the admit pipeline
	// re-checks the operator-attestation TRUST edge for an already-
	// admitted peer. Default 30m. The check is cheap (in-memory
	// trust graph read), but tightening below 5m on a node with
	// hundreds of peers will visibly burn CPU.
	//
	// Environment variable: PEER_REATTESTATION_INTERVAL
	PeerReattestationInterval time.Duration `json:"peerReattestationInterval" yaml:"-"`

	// --- Peer scoring (Phase 4) ----------------------------------------

	// PeerScorePersistInterval is how often the scoreboard is
	// snapshot to data_dir/peer_scores.json. Default 5m.
	// Reputation accumulated since the last snapshot is lost
	// on hard kill; honoring SIGTERM triggers a final flush.
	//
	// Environment variable: PEER_SCORE_PERSIST_INTERVAL
	PeerScorePersistInterval time.Duration `json:"peerScorePersistInterval" yaml:"-"`

	// PeerQuarantineThreshold: peers whose composite score
	// drops below this are quarantined — kept in KnownNodes
	// but excluded from active routing. Default 0.4. Hysteresis
	// requires the score to rise above threshold+0.1 before
	// the peer is un-quarantined. Phase 4b.
	//
	// Environment variable: PEER_QUARANTINE_THRESHOLD
	PeerQuarantineThreshold float64 `json:"peerQuarantineThreshold" yaml:"peer_quarantine_threshold"`

	// PeerEvictionThreshold: peers whose composite score stays
	// below this for PeerEvictionGrace are evicted from
	// KnownNodes entirely. Default 0.2. Static-source peers
	// are NOT subject to automatic eviction (operator intent
	// wins) but still get a stern warning logged. Phase 4b.
	//
	// Environment variable: PEER_EVICTION_THRESHOLD
	PeerEvictionThreshold float64 `json:"peerEvictionThreshold" yaml:"peer_eviction_threshold"`

	// PeerEvictionGrace: how long a peer's composite must stay
	// below PeerEvictionThreshold before eviction fires.
	// Default 5m. Prevents transient outages from auto-evicting
	// a peer that recovers within the window.
	//
	// Environment variable: PEER_EVICTION_GRACE
	PeerEvictionGrace time.Duration `json:"peerEvictionGrace" yaml:"-"`

	// PeerForkAction: what the fork-detection feedback does
	// when a peer is implicated in 2+ fork claims within
	// PeerForkWindow. One of: "log", "quarantine", "evict".
	// Default "quarantine". Phase 4c.
	//
	// Environment variable: PEER_FORK_ACTION
	PeerForkAction string `json:"peerForkAction" yaml:"peer_fork_action"`

	// PeerForkWindow: rolling window for the 2-fork-claims
	// trigger. Default 1h. Older claims still count toward
	// the cumulative ForkClaims counter; this window only
	// gates the quarantine/eviction action.
	//
	// Environment variable: PEER_FORK_WINDOW
	PeerForkWindow time.Duration `json:"peerForkWindow" yaml:"-"`
}

// fileConfig is used for parsing config files with string durations
type fileConfig struct {
	Port                    string   `json:"port" yaml:"port"`
	SeedNodes               *[]string `json:"seedNodes" yaml:"seed_nodes"` // ENG-16: pointer distinguishes "omitted" from "explicitly empty"
	LogLevel                string   `json:"logLevel" yaml:"log_level"`
	BlockInterval           string   `json:"blockInterval" yaml:"block_interval"`
	RateLimitPerMinute      int      `json:"rateLimitPerMinute" yaml:"rate_limit_per_minute"`
	MaxBodySizeBytes        int64    `json:"maxBodySizeBytes" yaml:"max_body_size_bytes"`
	DataDir                 string   `json:"dataDir" yaml:"data_dir"`
	ShutdownTimeout         string   `json:"shutdownTimeout" yaml:"shutdown_timeout"`
	HTTPClientTimeout       string   `json:"httpClientTimeout" yaml:"http_client_timeout"`
	NodeAuthSecret          string   `json:"nodeAuthSecret" yaml:"node_auth_secret"`
	RequireNodeAuth         bool     `json:"requireNodeAuth" yaml:"require_node_auth"`
	IPFSEnabled             bool     `json:"ipfsEnabled" yaml:"ipfs_enabled"`
	IPFSGatewayURL          string   `json:"ipfsGatewayUrl" yaml:"ipfs_gateway_url"`
	IPFSTimeout             string   `json:"ipfsTimeout" yaml:"ipfs_timeout"`
	SupportedDomains        []string `json:"supportedDomains" yaml:"supported_domains"`
	AllowDomainRegistration *bool    `json:"allowDomainRegistration" yaml:"allow_domain_registration"`
	RequireParentDomainAuth *bool    `json:"requireParentDomainAuth" yaml:"require_parent_domain_auth"`
	TrustCacheTTL           string   `json:"trustCacheTTL" yaml:"trust_cache_ttl"`
	DomainGossipInterval    string   `json:"domainGossipInterval" yaml:"domain_gossip_interval"`
	DomainGossipTTL         int      `json:"domainGossipTTL" yaml:"domain_gossip_ttl"`
	AuditLogPath            string   `json:"auditLogPath" yaml:"audit_log_path"`
	OperatorQuidFile        string   `json:"operatorQuidFile" yaml:"operator_quid_file"`

	// Peering knobs (file-loaded; envs override below)
	PeersFile                 string  `json:"peersFile" yaml:"peers_file"`
	LANDiscovery              *bool   `json:"lanDiscovery" yaml:"lan_discovery"`
	LANServiceName            string  `json:"lanServiceName" yaml:"lan_service_name"`
	RequireAdvertisement      *bool   `json:"requireAdvertisement" yaml:"require_advertisement"`
	PeerMinOperatorTrust      *float64 `json:"peerMinOperatorTrust" yaml:"peer_min_operator_trust"`
	PeerMinOperatorReputation *float64 `json:"peerMinOperatorReputation" yaml:"peer_min_operator_reputation"`
	PeerReattestationInterval string  `json:"peerReattestationInterval" yaml:"peer_reattestation_interval"`
}

// Default values
const (
	DefaultRateLimitPerMinute      = 100
	DefaultMaxBodySizeBytes        = 1 << 20 // 1MB
	DefaultDataDir                 = "./data"
	DefaultShutdownTimeout         = 30 * time.Second
	DefaultHTTPClientTimeout       = 5 * time.Second
	DefaultIPFSEnabled             = false
	DefaultIPFSGatewayURL          = "http://localhost:5001"
	DefaultIPFSTimeout             = 30 * time.Second
	DefaultAllowDomainRegistration = true
	DefaultRequireParentDomainAuth = true
	DefaultTrustCacheTTL           = 60 * time.Second
	DefaultDomainGossipInterval    = 2 * time.Minute
	DefaultDomainGossipTTL         = 3 // Default hop count before gossip is dropped

	// Peering defaults
	DefaultLANServiceName            = "_quidnug._tcp"
	DefaultRequireAdvertisement      = true
	DefaultPeerMinOperatorTrust      = 0.5 // matches QDP-0014 MinOperatorTrustWeight
	DefaultPeerMinOperatorReputation = 0.0 // off by default
	DefaultPeerReattestationInterval = 30 * time.Minute

	// Peer-scoring defaults (Phase 4)
	DefaultPeerScorePersistInterval = 5 * time.Minute
	DefaultPeerQuarantineThreshold  = 0.4
	DefaultPeerEvictionThreshold    = 0.2
	DefaultPeerEvictionGrace        = 5 * time.Minute
	DefaultPeerForkAction           = "quarantine"
	DefaultPeerForkWindow           = 1 * time.Hour
)

// DefaultConfigSearchPaths defines the default locations to search for config files
var DefaultConfigSearchPaths = []string{
	"./config.yaml",
	"./config.json",
	"/etc/quidnug/config.yaml",
}

// LoadConfigFromFile loads configuration from a file (YAML or JSON).
// The path is sourced from operator input (CLI flag, env var, or
// search-path discovery) and is treated as untrusted: safeio.ReadFile
// rejects path-traversal attempts, NUL injection, symlinks, and
// non-regular files before the read is issued.
func LoadConfigFromFile(path string) (*Config, error) {
	data, err := safeio.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var fc fileConfig
	ext := strings.ToLower(filepath.Ext(path))

	switch ext {
	case ".yaml", ".yml":
		if err := yaml.Unmarshal(data, &fc); err != nil {
			return nil, fmt.Errorf("failed to parse YAML config: %w", err)
		}
	case ".json":
		if err := json.Unmarshal(data, &fc); err != nil {
			return nil, fmt.Errorf("failed to parse JSON config: %w", err)
		}
	default:
		// Try YAML first, then JSON
		if err := yaml.Unmarshal(data, &fc); err != nil {
			if err := json.Unmarshal(data, &fc); err != nil {
				return nil, fmt.Errorf("failed to parse config file (tried YAML and JSON): %w", err)
			}
		}
	}

	return fileConfigToConfig(&fc)
}

// fileConfigToConfig converts a fileConfig to a Config, parsing duration strings
func fileConfigToConfig(fc *fileConfig) (*Config, error) {
	cfg := &Config{
		Port:               fc.Port,
		LogLevel:           fc.LogLevel,
		RateLimitPerMinute: fc.RateLimitPerMinute,
		MaxBodySizeBytes:   fc.MaxBodySizeBytes,
		DataDir:            fc.DataDir,
		NodeAuthSecret:     fc.NodeAuthSecret,
		RequireNodeAuth:    fc.RequireNodeAuth,
		SupportedDomains:   fc.SupportedDomains,
	}

	// ENG-16: propagate explicit empty seed_nodes from file config.
	if fc.SeedNodes != nil {
		cfg.SeedNodes = *fc.SeedNodes
	}

	if fc.AllowDomainRegistration != nil {
		cfg.AllowDomainRegistration = *fc.AllowDomainRegistration
	}

	if fc.RequireParentDomainAuth != nil {
		cfg.RequireParentDomainAuth = *fc.RequireParentDomainAuth
	}

	if fc.BlockInterval != "" {
		duration, err := time.ParseDuration(fc.BlockInterval)
		if err != nil {
			return nil, fmt.Errorf("invalid block_interval: %w", err)
		}
		cfg.BlockInterval = duration
	}

	if fc.ShutdownTimeout != "" {
		duration, err := time.ParseDuration(fc.ShutdownTimeout)
		if err != nil {
			return nil, fmt.Errorf("invalid shutdown_timeout: %w", err)
		}
		cfg.ShutdownTimeout = duration
	}

	if fc.HTTPClientTimeout != "" {
		duration, err := time.ParseDuration(fc.HTTPClientTimeout)
		if err != nil {
			return nil, fmt.Errorf("invalid http_client_timeout: %w", err)
		}
		cfg.HTTPClientTimeout = duration
	}

	cfg.IPFSEnabled = fc.IPFSEnabled
	cfg.IPFSGatewayURL = fc.IPFSGatewayURL

	if fc.IPFSTimeout != "" {
		duration, err := time.ParseDuration(fc.IPFSTimeout)
		if err != nil {
			return nil, fmt.Errorf("invalid ipfs_timeout: %w", err)
		}
		cfg.IPFSTimeout = duration
	}

	if fc.TrustCacheTTL != "" {
		duration, err := time.ParseDuration(fc.TrustCacheTTL)
		if err != nil {
			return nil, fmt.Errorf("invalid trust_cache_ttl: %w", err)
		}
		cfg.TrustCacheTTL = duration
	}

	if fc.DomainGossipInterval != "" {
		duration, err := time.ParseDuration(fc.DomainGossipInterval)
		if err != nil {
			return nil, fmt.Errorf("invalid domain_gossip_interval: %w", err)
		}
		cfg.DomainGossipInterval = duration
	}

	if fc.DomainGossipTTL > 0 {
		cfg.DomainGossipTTL = fc.DomainGossipTTL
	}

	if fc.AuditLogPath != "" {
		cfg.AuditLogPath = fc.AuditLogPath
	}
	if fc.OperatorQuidFile != "" {
		cfg.OperatorQuidFile = fc.OperatorQuidFile
	}

	// Peering knobs
	if fc.PeersFile != "" {
		cfg.PeersFile = fc.PeersFile
	}
	if fc.LANDiscovery != nil {
		cfg.LANDiscovery = *fc.LANDiscovery
	}
	if fc.LANServiceName != "" {
		cfg.LANServiceName = fc.LANServiceName
	}
	if fc.RequireAdvertisement != nil {
		cfg.RequireAdvertisement = *fc.RequireAdvertisement
	}
	if fc.PeerMinOperatorTrust != nil {
		cfg.PeerMinOperatorTrust = *fc.PeerMinOperatorTrust
	}
	if fc.PeerMinOperatorReputation != nil {
		cfg.PeerMinOperatorReputation = *fc.PeerMinOperatorReputation
	}
	if fc.PeerReattestationInterval != "" {
		d, err := time.ParseDuration(fc.PeerReattestationInterval)
		if err != nil {
			return nil, fmt.Errorf("invalid peer_reattestation_interval: %w", err)
		}
		cfg.PeerReattestationInterval = d
	}

	return cfg, nil
}

// findConfigFile searches for a config file in the default paths
func findConfigFile() string {
	for _, path := range DefaultConfigSearchPaths {
		if _, err := os.Stat(path); err == nil {
			return path
		}
	}
	return ""
}

// LoadConfig reads configuration with the following precedence (highest to lowest):
// 1. Environment variables
// 2. Config file (specified by CONFIG_FILE env var or found in default paths)
// 3. Default values
func LoadConfig() *Config {
	// Start with defaults
	cfg := &Config{
		Port:                    "8080",
		SeedNodes:               []string{"seed1.quidnug.net:8080", "seed2.quidnug.net:8080"},
		LogLevel:                "info",
		BlockInterval:           60 * time.Second,
		RateLimitPerMinute:      DefaultRateLimitPerMinute,
		MaxBodySizeBytes:        DefaultMaxBodySizeBytes,
		DataDir:                 DefaultDataDir,
		ShutdownTimeout:         DefaultShutdownTimeout,
		HTTPClientTimeout:       DefaultHTTPClientTimeout,
		IPFSEnabled:             DefaultIPFSEnabled,
		IPFSGatewayURL:          DefaultIPFSGatewayURL,
		IPFSTimeout:             DefaultIPFSTimeout,
		SupportedDomains:        []string{},
		AllowDomainRegistration: DefaultAllowDomainRegistration,
		RequireParentDomainAuth: DefaultRequireParentDomainAuth,
		TrustCacheTTL:           DefaultTrustCacheTTL,
		DomainGossipInterval:    DefaultDomainGossipInterval,
		DomainGossipTTL:         DefaultDomainGossipTTL,

		// Peering defaults
		LANServiceName:            DefaultLANServiceName,
		RequireAdvertisement:      DefaultRequireAdvertisement,
		PeerMinOperatorTrust:      DefaultPeerMinOperatorTrust,
		PeerMinOperatorReputation: DefaultPeerMinOperatorReputation,
		PeerReattestationInterval: DefaultPeerReattestationInterval,

		// Peer-scoring defaults (Phase 4)
		PeerScorePersistInterval: DefaultPeerScorePersistInterval,
		PeerQuarantineThreshold:  DefaultPeerQuarantineThreshold,
		PeerEvictionThreshold:    DefaultPeerEvictionThreshold,
		PeerEvictionGrace:        DefaultPeerEvictionGrace,
		PeerForkAction:           DefaultPeerForkAction,
		PeerForkWindow:           DefaultPeerForkWindow,
	}

	// Try to load from config file
	configPath := os.Getenv("CONFIG_FILE")
	if configPath == "" {
		configPath = findConfigFile()
	}

	if configPath != "" {
		if fileCfg, err := LoadConfigFromFile(configPath); err == nil {
			// Apply file config values (only non-zero values)
			if fileCfg.Port != "" {
				cfg.Port = fileCfg.Port
			}
			if fileCfg.SeedNodes != nil { // ENG-16: nil means omitted; empty means explicit override
				cfg.SeedNodes = fileCfg.SeedNodes
			}
			if fileCfg.LogLevel != "" {
				cfg.LogLevel = fileCfg.LogLevel
			}
			if fileCfg.BlockInterval > 0 {
				cfg.BlockInterval = fileCfg.BlockInterval
			}
			if fileCfg.RateLimitPerMinute > 0 {
				cfg.RateLimitPerMinute = fileCfg.RateLimitPerMinute
			}
			if fileCfg.MaxBodySizeBytes > 0 {
				cfg.MaxBodySizeBytes = fileCfg.MaxBodySizeBytes
			}
			if fileCfg.DataDir != "" {
				cfg.DataDir = fileCfg.DataDir
			}
			if fileCfg.ShutdownTimeout > 0 {
				cfg.ShutdownTimeout = fileCfg.ShutdownTimeout
			}
			if fileCfg.HTTPClientTimeout > 0 {
				cfg.HTTPClientTimeout = fileCfg.HTTPClientTimeout
			}
			if fileCfg.NodeAuthSecret != "" {
				cfg.NodeAuthSecret = fileCfg.NodeAuthSecret
			}
			if fileCfg.RequireNodeAuth {
				cfg.RequireNodeAuth = fileCfg.RequireNodeAuth
			}
			if fileCfg.IPFSEnabled {
				cfg.IPFSEnabled = fileCfg.IPFSEnabled
			}
			if fileCfg.IPFSGatewayURL != "" {
				cfg.IPFSGatewayURL = fileCfg.IPFSGatewayURL
			}
			if fileCfg.IPFSTimeout > 0 {
				cfg.IPFSTimeout = fileCfg.IPFSTimeout
			}
			if len(fileCfg.SupportedDomains) > 0 {
				cfg.SupportedDomains = fileCfg.SupportedDomains
			}
			if !fileCfg.AllowDomainRegistration {
				cfg.AllowDomainRegistration = fileCfg.AllowDomainRegistration
			}
			if !fileCfg.RequireParentDomainAuth {
				cfg.RequireParentDomainAuth = fileCfg.RequireParentDomainAuth
			}
			if fileCfg.TrustCacheTTL > 0 {
				cfg.TrustCacheTTL = fileCfg.TrustCacheTTL
			}
			if fileCfg.DomainGossipInterval > 0 {
				cfg.DomainGossipInterval = fileCfg.DomainGossipInterval
			}
			if fileCfg.DomainGossipTTL > 0 {
				cfg.DomainGossipTTL = fileCfg.DomainGossipTTL
			}
			if fileCfg.AuditLogPath != "" {
				cfg.AuditLogPath = fileCfg.AuditLogPath
			}
			if fileCfg.OperatorQuidFile != "" {
				cfg.OperatorQuidFile = fileCfg.OperatorQuidFile
			}
			if fileCfg.PeersFile != "" {
				cfg.PeersFile = fileCfg.PeersFile
			}
			// Default false; truthy overrides.
			if fileCfg.LANDiscovery {
				cfg.LANDiscovery = true
			}
			if fileCfg.LANServiceName != "" {
				cfg.LANServiceName = fileCfg.LANServiceName
			}
			// Default true; only an explicit-false in file overrides.
			// We cannot distinguish "explicit false" from "absent"
			// at this layer, so a config that omits this key keeps
			// the default. Operators who want false set the env var.
			if !fileCfg.RequireAdvertisement {
				cfg.RequireAdvertisement = false
			}
			if fileCfg.PeerMinOperatorTrust > 0 {
				cfg.PeerMinOperatorTrust = fileCfg.PeerMinOperatorTrust
			}
			if fileCfg.PeerMinOperatorReputation > 0 {
				cfg.PeerMinOperatorReputation = fileCfg.PeerMinOperatorReputation
			}
			if fileCfg.PeerReattestationInterval > 0 {
				cfg.PeerReattestationInterval = fileCfg.PeerReattestationInterval
			}
		}
	}

	// Environment variables override everything
	if port := os.Getenv("PORT"); port != "" {
		cfg.Port = port
	}

	if seedNodesEnv := os.Getenv("SEED_NODES"); seedNodesEnv != "" {
		var seedNodes []string
		if err := json.Unmarshal([]byte(seedNodesEnv), &seedNodes); err == nil { // ENG-16: accept explicit empty list
			cfg.SeedNodes = seedNodes
		}
	}

	if logLevel := os.Getenv("LOG_LEVEL"); logLevel != "" {
		cfg.LogLevel = logLevel
	}

	if blockInterval := os.Getenv("BLOCK_INTERVAL"); blockInterval != "" {
		if duration, err := time.ParseDuration(blockInterval); err == nil {
			cfg.BlockInterval = duration
		}
	}

	if rateLimitEnv := os.Getenv("RATE_LIMIT_PER_MINUTE"); rateLimitEnv != "" {
		if rateLimit, err := strconv.Atoi(rateLimitEnv); err == nil && rateLimit > 0 {
			cfg.RateLimitPerMinute = rateLimit
		}
	}

	if maxBodyEnv := os.Getenv("MAX_BODY_SIZE_BYTES"); maxBodyEnv != "" {
		if maxBody, err := strconv.ParseInt(maxBodyEnv, 10, 64); err == nil && maxBody > 0 {
			cfg.MaxBodySizeBytes = maxBody
		}
	}

	if dataDir := os.Getenv("DATA_DIR"); dataDir != "" {
		cfg.DataDir = dataDir
	}

	if shutdownTimeout := os.Getenv("SHUTDOWN_TIMEOUT"); shutdownTimeout != "" {
		if duration, err := time.ParseDuration(shutdownTimeout); err == nil {
			cfg.ShutdownTimeout = duration
		}
	}

	if httpTimeout := os.Getenv("HTTP_CLIENT_TIMEOUT"); httpTimeout != "" {
		if duration, err := time.ParseDuration(httpTimeout); err == nil {
			cfg.HTTPClientTimeout = duration
		}
	}

	if nodeAuthSecret := os.Getenv("NODE_AUTH_SECRET"); nodeAuthSecret != "" {
		cfg.NodeAuthSecret = nodeAuthSecret
	}

	if requireNodeAuth := os.Getenv("REQUIRE_NODE_AUTH"); requireNodeAuth == "true" {
		cfg.RequireNodeAuth = true
	}

	if ipfsEnabled := os.Getenv("QUIDNUG_IPFS_ENABLED"); ipfsEnabled == "true" {
		cfg.IPFSEnabled = true
	} else if ipfsEnabled == "false" {
		cfg.IPFSEnabled = false
	}

	if ipfsGatewayURL := os.Getenv("QUIDNUG_IPFS_GATEWAY_URL"); ipfsGatewayURL != "" {
		cfg.IPFSGatewayURL = ipfsGatewayURL
	}

	if ipfsTimeout := os.Getenv("QUIDNUG_IPFS_TIMEOUT"); ipfsTimeout != "" {
		if duration, err := time.ParseDuration(ipfsTimeout); err == nil {
			cfg.IPFSTimeout = duration
		}
	}

	if supportedDomainsEnv := os.Getenv("SUPPORTED_DOMAINS"); supportedDomainsEnv != "" {
		var supportedDomains []string
		if err := json.Unmarshal([]byte(supportedDomainsEnv), &supportedDomains); err == nil {
			cfg.SupportedDomains = supportedDomains
		}
	}

	if allowDomainReg := os.Getenv("ALLOW_DOMAIN_REGISTRATION"); allowDomainReg != "" {
		cfg.AllowDomainRegistration = allowDomainReg == "true"
	}

	if requireParentAuth := os.Getenv("REQUIRE_PARENT_DOMAIN_AUTH"); requireParentAuth != "" {
		cfg.RequireParentDomainAuth = requireParentAuth == "true"
	}

	if trustCacheTTL := os.Getenv("TRUST_CACHE_TTL"); trustCacheTTL != "" {
		if duration, err := time.ParseDuration(trustCacheTTL); err == nil {
			cfg.TrustCacheTTL = duration
		}
	}

	if domainGossipInterval := os.Getenv("DOMAIN_GOSSIP_INTERVAL"); domainGossipInterval != "" {
		if duration, err := time.ParseDuration(domainGossipInterval); err == nil {
			cfg.DomainGossipInterval = duration
		}
	}

	if domainGossipTTL := os.Getenv("DOMAIN_GOSSIP_TTL"); domainGossipTTL != "" {
		if ttl, err := strconv.Atoi(domainGossipTTL); err == nil && ttl > 0 {
			cfg.DomainGossipTTL = ttl
		}
	}

	if auditLogPath := os.Getenv("AUDIT_LOG_PATH"); auditLogPath != "" {
		cfg.AuditLogPath = auditLogPath
	}

	if operatorQuid := os.Getenv("OPERATOR_QUID_FILE"); operatorQuid != "" {
		cfg.OperatorQuidFile = operatorQuid
	}

	// Peering env vars
	if peersFile := os.Getenv("PEERS_FILE"); peersFile != "" {
		cfg.PeersFile = peersFile
	}
	if lan := os.Getenv("LAN_DISCOVERY"); lan != "" {
		cfg.LANDiscovery = lan == "true" || lan == "1" || strings.EqualFold(lan, "yes")
	}
	if name := os.Getenv("LAN_SERVICE_NAME"); name != "" {
		cfg.LANServiceName = name
	}
	if reqAd := os.Getenv("REQUIRE_ADVERTISEMENT"); reqAd != "" {
		cfg.RequireAdvertisement = reqAd == "true" || reqAd == "1" || strings.EqualFold(reqAd, "yes")
	}
	if v := os.Getenv("PEER_MIN_OPERATOR_TRUST"); v != "" {
		if f, err := strconv.ParseFloat(v, 64); err == nil && f >= 0 && f <= 1 {
			cfg.PeerMinOperatorTrust = f
		}
	}
	if v := os.Getenv("PEER_MIN_OPERATOR_REPUTATION"); v != "" {
		if f, err := strconv.ParseFloat(v, 64); err == nil && f >= 0 && f <= 1 {
			cfg.PeerMinOperatorReputation = f
		}
	}
	if v := os.Getenv("PEER_REATTESTATION_INTERVAL"); v != "" {
		if d, err := time.ParseDuration(v); err == nil && d > 0 {
			cfg.PeerReattestationInterval = d
		}
	}
	if v := os.Getenv("PEER_SCORE_PERSIST_INTERVAL"); v != "" {
		if d, err := time.ParseDuration(v); err == nil && d > 0 {
			cfg.PeerScorePersistInterval = d
		}
	}
	if v := os.Getenv("PEER_QUARANTINE_THRESHOLD"); v != "" {
		if f, err := strconv.ParseFloat(v, 64); err == nil && f >= 0 && f <= 1 {
			cfg.PeerQuarantineThreshold = f
		}
	}
	if v := os.Getenv("PEER_EVICTION_THRESHOLD"); v != "" {
		if f, err := strconv.ParseFloat(v, 64); err == nil && f >= 0 && f <= 1 {
			cfg.PeerEvictionThreshold = f
		}
	}
	if v := os.Getenv("PEER_EVICTION_GRACE"); v != "" {
		if d, err := time.ParseDuration(v); err == nil && d > 0 {
			cfg.PeerEvictionGrace = d
		}
	}
	if v := os.Getenv("PEER_FORK_ACTION"); v != "" {
		switch strings.ToLower(v) {
		case "log", "quarantine", "evict":
			cfg.PeerForkAction = strings.ToLower(v)
		}
	}
	if v := os.Getenv("PEER_FORK_WINDOW"); v != "" {
		if d, err := time.ParseDuration(v); err == nil && d > 0 {
			cfg.PeerForkWindow = d
		}
	}

	if enablePushGossip := os.Getenv("ENABLE_PUSH_GOSSIP"); enablePushGossip != "" {
		cfg.EnablePushGossip = enablePushGossip == "true"
	}

	if enableLazyProbe := os.Getenv("ENABLE_LAZY_EPOCH_PROBE"); enableLazyProbe != "" {
		cfg.EnableLazyEpochProbe = enableLazyProbe == "true"
	}
	if recency := os.Getenv("EPOCH_RECENCY_WINDOW"); recency != "" {
		if d, err := time.ParseDuration(recency); err == nil {
			cfg.EpochRecencyWindow = d
		}
	}
	if probeTimeout := os.Getenv("EPOCH_PROBE_TIMEOUT"); probeTimeout != "" {
		if d, err := time.ParseDuration(probeTimeout); err == nil {
			cfg.EpochProbeTimeout = d
		}
	}
	if policy := os.Getenv("PROBE_TIMEOUT_POLICY"); policy != "" {
		cfg.ProbeTimeoutPolicy = policy
	}

	return cfg
}
