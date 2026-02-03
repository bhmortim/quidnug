package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

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
}

// fileConfig is used for parsing config files with string durations
type fileConfig struct {
	Port                    string   `json:"port" yaml:"port"`
	SeedNodes               []string `json:"seedNodes" yaml:"seed_nodes"`
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
)

// DefaultConfigSearchPaths defines the default locations to search for config files
var DefaultConfigSearchPaths = []string{
	"./config.yaml",
	"./config.json",
	"/etc/quidnug/config.yaml",
}

// LoadConfigFromFile loads configuration from a file (YAML or JSON)
func LoadConfigFromFile(path string) (*Config, error) {
	data, err := os.ReadFile(path)
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
		SeedNodes:          fc.SeedNodes,
		LogLevel:           fc.LogLevel,
		RateLimitPerMinute: fc.RateLimitPerMinute,
		MaxBodySizeBytes:   fc.MaxBodySizeBytes,
		DataDir:            fc.DataDir,
		NodeAuthSecret:     fc.NodeAuthSecret,
		RequireNodeAuth:    fc.RequireNodeAuth,
		SupportedDomains:   fc.SupportedDomains,
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
			if len(fileCfg.SeedNodes) > 0 {
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
		}
	}

	// Environment variables override everything
	if port := os.Getenv("PORT"); port != "" {
		cfg.Port = port
	}

	if seedNodesEnv := os.Getenv("SEED_NODES"); seedNodesEnv != "" {
		var seedNodes []string
		if err := json.Unmarshal([]byte(seedNodesEnv), &seedNodes); err == nil && len(seedNodes) > 0 {
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

	return cfg
}

// MatchDomainPattern checks if a domain matches a pattern.
// Patterns can be exact matches or wildcard patterns like "*.example.com".
// Wildcard patterns match any subdomain but not the base domain itself.
func MatchDomainPattern(domain, pattern string) bool {
	if domain == "" || pattern == "" {
		return false
	}

	// Exact match
	if domain == pattern {
		return true
	}

	// Wildcard pattern: *.example.com
	if strings.HasPrefix(pattern, "*.") {
		suffix := pattern[1:] // ".example.com"
		// Domain must end with the suffix and have something before it
		if strings.HasSuffix(domain, suffix) && len(domain) > len(suffix) {
			return true
		}
	}

	return false
}
