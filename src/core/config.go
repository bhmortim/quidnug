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
	Port               string        `json:"port" yaml:"port"`
	SeedNodes          []string      `json:"seedNodes" yaml:"seed_nodes"`
	LogLevel           string        `json:"logLevel" yaml:"log_level"`
	BlockInterval      time.Duration `json:"blockInterval" yaml:"-"`
	RateLimitPerMinute int           `json:"rateLimitPerMinute" yaml:"rate_limit_per_minute"`
	MaxBodySizeBytes   int64         `json:"maxBodySizeBytes" yaml:"max_body_size_bytes"`
	DataDir            string        `json:"dataDir" yaml:"data_dir"`
	ShutdownTimeout    time.Duration `json:"shutdownTimeout" yaml:"-"`
	HTTPClientTimeout  time.Duration `json:"httpClientTimeout" yaml:"-"`
	NodeAuthSecret     string        `json:"nodeAuthSecret" yaml:"node_auth_secret"`
	RequireNodeAuth    bool          `json:"requireNodeAuth" yaml:"require_node_auth"`
	IPFSEnabled        bool          `json:"ipfsEnabled" yaml:"ipfs_enabled"`
	IPFSGatewayURL     string        `json:"ipfsGatewayUrl" yaml:"ipfs_gateway_url"`
	IPFSTimeout        time.Duration `json:"ipfsTimeout" yaml:"-"`
}

// fileConfig is used for parsing config files with string durations
type fileConfig struct {
	Port               string   `json:"port" yaml:"port"`
	SeedNodes          []string `json:"seedNodes" yaml:"seed_nodes"`
	LogLevel           string   `json:"logLevel" yaml:"log_level"`
	BlockInterval      string   `json:"blockInterval" yaml:"block_interval"`
	RateLimitPerMinute int      `json:"rateLimitPerMinute" yaml:"rate_limit_per_minute"`
	MaxBodySizeBytes   int64    `json:"maxBodySizeBytes" yaml:"max_body_size_bytes"`
	DataDir            string   `json:"dataDir" yaml:"data_dir"`
	ShutdownTimeout    string   `json:"shutdownTimeout" yaml:"shutdown_timeout"`
	HTTPClientTimeout  string   `json:"httpClientTimeout" yaml:"http_client_timeout"`
	NodeAuthSecret     string   `json:"nodeAuthSecret" yaml:"node_auth_secret"`
	RequireNodeAuth    bool     `json:"requireNodeAuth" yaml:"require_node_auth"`
	IPFSEnabled        bool     `json:"ipfsEnabled" yaml:"ipfs_enabled"`
	IPFSGatewayURL     string   `json:"ipfsGatewayUrl" yaml:"ipfs_gateway_url"`
	IPFSTimeout        string   `json:"ipfsTimeout" yaml:"ipfs_timeout"`
}

// Default values
const (
	DefaultRateLimitPerMinute = 100
	DefaultMaxBodySizeBytes   = 1 << 20 // 1MB
	DefaultDataDir            = "./data"
	DefaultShutdownTimeout    = 30 * time.Second
	DefaultHTTPClientTimeout  = 5 * time.Second
	DefaultIPFSEnabled        = false
	DefaultIPFSGatewayURL     = "http://localhost:5001"
	DefaultIPFSTimeout        = 30 * time.Second
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
		Port:               "8080",
		SeedNodes:          []string{"seed1.quidnug.net:8080", "seed2.quidnug.net:8080"},
		LogLevel:           "info",
		BlockInterval:      60 * time.Second,
		RateLimitPerMinute: DefaultRateLimitPerMinute,
		MaxBodySizeBytes:   DefaultMaxBodySizeBytes,
		DataDir:            DefaultDataDir,
		ShutdownTimeout:    DefaultShutdownTimeout,
		HTTPClientTimeout:  DefaultHTTPClientTimeout,
		IPFSEnabled:        DefaultIPFSEnabled,
		IPFSGatewayURL:     DefaultIPFSGatewayURL,
		IPFSTimeout:        DefaultIPFSTimeout,
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

	return cfg
}
