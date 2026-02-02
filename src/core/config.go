package main

import (
	"encoding/json"
	"os"
	"strconv"
	"time"
)

// Config holds the application configuration
type Config struct {
	Port               string        `json:"port"`
	SeedNodes          []string      `json:"seedNodes"`
	LogLevel           string        `json:"logLevel"`
	BlockInterval      time.Duration `json:"blockInterval"`
	RateLimitPerMinute int           `json:"rateLimitPerMinute"`
	MaxBodySizeBytes   int64         `json:"maxBodySizeBytes"`
	DataDir            string        `json:"dataDir"`
	ShutdownTimeout    time.Duration `json:"shutdownTimeout"`
}

// Default values
const (
	DefaultRateLimitPerMinute = 100
	DefaultMaxBodySizeBytes   = 1 << 20 // 1MB
	DefaultDataDir            = "./data"
	DefaultShutdownTimeout    = 30 * time.Second
)

// LoadConfig reads configuration from environment variables with defaults
func LoadConfig() *Config {
	cfg := &Config{
		Port:               "8080",
		SeedNodes:          []string{"seed1.quidnug.net:8080", "seed2.quidnug.net:8080"},
		LogLevel:           "info",
		BlockInterval:      60 * time.Second,
		RateLimitPerMinute: DefaultRateLimitPerMinute,
		MaxBodySizeBytes:   DefaultMaxBodySizeBytes,
		DataDir:            DefaultDataDir,
		ShutdownTimeout:    DefaultShutdownTimeout,
	}

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

	return cfg
}
