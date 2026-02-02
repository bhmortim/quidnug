package main

import (
	"os"
	"testing"
	"time"
)

func TestLoadConfigDefaults(t *testing.T) {
	// Clear any existing env vars
	os.Unsetenv("PORT")
	os.Unsetenv("SEED_NODES")
	os.Unsetenv("LOG_LEVEL")
	os.Unsetenv("BLOCK_INTERVAL")

	cfg := LoadConfig()

	if cfg.Port != "8080" {
		t.Errorf("Expected default port '8080', got '%s'", cfg.Port)
	}

	if len(cfg.SeedNodes) != 2 {
		t.Errorf("Expected 2 default seed nodes, got %d", len(cfg.SeedNodes))
	}

	if cfg.SeedNodes[0] != "seed1.quidnug.net:8080" {
		t.Errorf("Expected first seed node 'seed1.quidnug.net:8080', got '%s'", cfg.SeedNodes[0])
	}

	if cfg.LogLevel != "info" {
		t.Errorf("Expected default log level 'info', got '%s'", cfg.LogLevel)
	}

	if cfg.BlockInterval != 60*time.Second {
		t.Errorf("Expected default block interval 60s, got %v", cfg.BlockInterval)
	}
}

func TestLoadConfigFromEnv(t *testing.T) {
	// Set environment variables
	os.Setenv("PORT", "9090")
	os.Setenv("SEED_NODES", `["node1.example.com:8080","node2.example.com:8080","node3.example.com:8080"]`)
	os.Setenv("LOG_LEVEL", "debug")
	os.Setenv("BLOCK_INTERVAL", "30s")

	defer func() {
		os.Unsetenv("PORT")
		os.Unsetenv("SEED_NODES")
		os.Unsetenv("LOG_LEVEL")
		os.Unsetenv("BLOCK_INTERVAL")
	}()

	cfg := LoadConfig()

	if cfg.Port != "9090" {
		t.Errorf("Expected port '9090', got '%s'", cfg.Port)
	}

	if len(cfg.SeedNodes) != 3 {
		t.Errorf("Expected 3 seed nodes, got %d", len(cfg.SeedNodes))
	}

	if cfg.SeedNodes[0] != "node1.example.com:8080" {
		t.Errorf("Expected first seed node 'node1.example.com:8080', got '%s'", cfg.SeedNodes[0])
	}

	if cfg.LogLevel != "debug" {
		t.Errorf("Expected log level 'debug', got '%s'", cfg.LogLevel)
	}

	if cfg.BlockInterval != 30*time.Second {
		t.Errorf("Expected block interval 30s, got %v", cfg.BlockInterval)
	}
}

func TestLoadConfigInvalidSeedNodesJSON(t *testing.T) {
	os.Setenv("SEED_NODES", "invalid-json")
	defer os.Unsetenv("SEED_NODES")

	cfg := LoadConfig()

	// Should fall back to defaults
	if len(cfg.SeedNodes) != 2 {
		t.Errorf("Expected 2 default seed nodes on invalid JSON, got %d", len(cfg.SeedNodes))
	}
}

func TestLoadConfigEmptySeedNodesArray(t *testing.T) {
	os.Setenv("SEED_NODES", "[]")
	defer os.Unsetenv("SEED_NODES")

	cfg := LoadConfig()

	// Should fall back to defaults when array is empty
	if len(cfg.SeedNodes) != 2 {
		t.Errorf("Expected 2 default seed nodes on empty array, got %d", len(cfg.SeedNodes))
	}
}

func TestLoadConfigInvalidBlockInterval(t *testing.T) {
	os.Setenv("BLOCK_INTERVAL", "not-a-duration")
	defer os.Unsetenv("BLOCK_INTERVAL")

	cfg := LoadConfig()

	// Should fall back to default
	if cfg.BlockInterval != 60*time.Second {
		t.Errorf("Expected default block interval 60s on invalid duration, got %v", cfg.BlockInterval)
	}
}

func TestLoadConfigPartialEnv(t *testing.T) {
	os.Unsetenv("PORT")
	os.Unsetenv("SEED_NODES")
	os.Setenv("LOG_LEVEL", "warn")
	os.Unsetenv("BLOCK_INTERVAL")

	defer os.Unsetenv("LOG_LEVEL")

	cfg := LoadConfig()

	if cfg.Port != "8080" {
		t.Errorf("Expected default port '8080', got '%s'", cfg.Port)
	}

	if cfg.LogLevel != "warn" {
		t.Errorf("Expected log level 'warn', got '%s'", cfg.LogLevel)
	}

	if cfg.BlockInterval != 60*time.Second {
		t.Errorf("Expected default block interval 60s, got %v", cfg.BlockInterval)
	}
}

func TestLoadConfigVariousBlockIntervals(t *testing.T) {
	tests := []struct {
		input    string
		expected time.Duration
	}{
		{"1m", 1 * time.Minute},
		{"120s", 120 * time.Second},
		{"2m30s", 2*time.Minute + 30*time.Second},
		{"1h", 1 * time.Hour},
	}

	for _, tc := range tests {
		os.Setenv("BLOCK_INTERVAL", tc.input)
		cfg := LoadConfig()
		if cfg.BlockInterval != tc.expected {
			t.Errorf("For input '%s', expected %v, got %v", tc.input, tc.expected, cfg.BlockInterval)
		}
	}

	os.Unsetenv("BLOCK_INTERVAL")
}
