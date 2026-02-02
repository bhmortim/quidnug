package main

import (
	"encoding/json"
	"os"
	"time"
)

// Config holds the application configuration
type Config struct {
	Port          string        `json:"port"`
	SeedNodes     []string      `json:"seedNodes"`
	LogLevel      string        `json:"logLevel"`
	BlockInterval time.Duration `json:"blockInterval"`
}

// LoadConfig reads configuration from environment variables with defaults
func LoadConfig() *Config {
	cfg := &Config{
		Port:          "8080",
		SeedNodes:     []string{"seed1.quidnug.net:8080", "seed2.quidnug.net:8080"},
		LogLevel:      "info",
		BlockInterval: 60 * time.Second,
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

	return cfg
}
