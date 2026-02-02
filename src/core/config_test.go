package main

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func clearConfigEnvVars() {
	os.Unsetenv("CONFIG_FILE")
	os.Unsetenv("PORT")
	os.Unsetenv("SEED_NODES")
	os.Unsetenv("LOG_LEVEL")
	os.Unsetenv("BLOCK_INTERVAL")
	os.Unsetenv("RATE_LIMIT_PER_MINUTE")
	os.Unsetenv("MAX_BODY_SIZE_BYTES")
	os.Unsetenv("DATA_DIR")
	os.Unsetenv("SHUTDOWN_TIMEOUT")
	os.Unsetenv("HTTP_CLIENT_TIMEOUT")
	os.Unsetenv("NODE_AUTH_SECRET")
	os.Unsetenv("REQUIRE_NODE_AUTH")
}

func TestLoadConfigDefaults(t *testing.T) {
	// Clear any existing env vars
	os.Unsetenv("PORT")
	os.Unsetenv("SEED_NODES")
	os.Unsetenv("LOG_LEVEL")
	os.Unsetenv("BLOCK_INTERVAL")
	os.Unsetenv("RATE_LIMIT_PER_MINUTE")
	os.Unsetenv("MAX_BODY_SIZE_BYTES")
	os.Unsetenv("DATA_DIR")
	os.Unsetenv("SHUTDOWN_TIMEOUT")

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

	if cfg.RateLimitPerMinute != DefaultRateLimitPerMinute {
		t.Errorf("Expected default rate limit %d, got %d", DefaultRateLimitPerMinute, cfg.RateLimitPerMinute)
	}

	if cfg.MaxBodySizeBytes != DefaultMaxBodySizeBytes {
		t.Errorf("Expected default max body size %d, got %d", DefaultMaxBodySizeBytes, cfg.MaxBodySizeBytes)
	}

	if cfg.DataDir != DefaultDataDir {
		t.Errorf("Expected default data dir '%s', got '%s'", DefaultDataDir, cfg.DataDir)
	}

	if cfg.ShutdownTimeout != DefaultShutdownTimeout {
		t.Errorf("Expected default shutdown timeout %v, got %v", DefaultShutdownTimeout, cfg.ShutdownTimeout)
	}
}

func TestLoadConfigFromEnv(t *testing.T) {
	// Set environment variables
	os.Setenv("PORT", "9090")
	os.Setenv("SEED_NODES", `["node1.example.com:8080","node2.example.com:8080","node3.example.com:8080"]`)
	os.Setenv("LOG_LEVEL", "debug")
	os.Setenv("BLOCK_INTERVAL", "30s")
	os.Setenv("RATE_LIMIT_PER_MINUTE", "200")
	os.Setenv("MAX_BODY_SIZE_BYTES", "2097152")
	os.Setenv("DATA_DIR", "/custom/data")
	os.Setenv("SHUTDOWN_TIMEOUT", "45s")

	defer func() {
		os.Unsetenv("PORT")
		os.Unsetenv("SEED_NODES")
		os.Unsetenv("LOG_LEVEL")
		os.Unsetenv("BLOCK_INTERVAL")
		os.Unsetenv("RATE_LIMIT_PER_MINUTE")
		os.Unsetenv("MAX_BODY_SIZE_BYTES")
		os.Unsetenv("DATA_DIR")
		os.Unsetenv("SHUTDOWN_TIMEOUT")
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

	if cfg.RateLimitPerMinute != 200 {
		t.Errorf("Expected rate limit 200, got %d", cfg.RateLimitPerMinute)
	}

	if cfg.MaxBodySizeBytes != 2097152 {
		t.Errorf("Expected max body size 2097152, got %d", cfg.MaxBodySizeBytes)
	}

	if cfg.DataDir != "/custom/data" {
		t.Errorf("Expected data dir '/custom/data', got '%s'", cfg.DataDir)
	}

	if cfg.ShutdownTimeout != 45*time.Second {
		t.Errorf("Expected shutdown timeout 45s, got %v", cfg.ShutdownTimeout)
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
	os.Unsetenv("RATE_LIMIT_PER_MINUTE")
	os.Unsetenv("MAX_BODY_SIZE_BYTES")
	os.Unsetenv("DATA_DIR")
	os.Unsetenv("SHUTDOWN_TIMEOUT")

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

	if cfg.RateLimitPerMinute != DefaultRateLimitPerMinute {
		t.Errorf("Expected default rate limit, got %d", cfg.RateLimitPerMinute)
	}

	if cfg.DataDir != DefaultDataDir {
		t.Errorf("Expected default data dir, got '%s'", cfg.DataDir)
	}

	if cfg.ShutdownTimeout != DefaultShutdownTimeout {
		t.Errorf("Expected default shutdown timeout, got %v", cfg.ShutdownTimeout)
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

func TestLoadConfigInvalidRateLimit(t *testing.T) {
	tests := []struct {
		input    string
		expected int
	}{
		{"not-a-number", DefaultRateLimitPerMinute},
		{"-50", DefaultRateLimitPerMinute},
		{"0", DefaultRateLimitPerMinute},
		{"50", 50},
	}

	for _, tc := range tests {
		os.Setenv("RATE_LIMIT_PER_MINUTE", tc.input)
		cfg := LoadConfig()
		if cfg.RateLimitPerMinute != tc.expected {
			t.Errorf("For input '%s', expected %d, got %d", tc.input, tc.expected, cfg.RateLimitPerMinute)
		}
	}

	os.Unsetenv("RATE_LIMIT_PER_MINUTE")
}

func TestLoadConfigInvalidMaxBodySize(t *testing.T) {
	tests := []struct {
		input    string
		expected int64
	}{
		{"not-a-number", DefaultMaxBodySizeBytes},
		{"-1000", DefaultMaxBodySizeBytes},
		{"0", DefaultMaxBodySizeBytes},
		{"524288", 524288},
	}

	for _, tc := range tests {
		os.Setenv("MAX_BODY_SIZE_BYTES", tc.input)
		cfg := LoadConfig()
		if cfg.MaxBodySizeBytes != tc.expected {
			t.Errorf("For input '%s', expected %d, got %d", tc.input, tc.expected, cfg.MaxBodySizeBytes)
		}
	}

	os.Unsetenv("MAX_BODY_SIZE_BYTES")
}

func TestLoadConfigFromYAMLFile(t *testing.T) {
	clearConfigEnvVars()

	// Create a temporary YAML config file
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")

	yamlContent := `
port: "9090"
seed_nodes:
  - "node1.example.com:8080"
  - "node2.example.com:8080"
log_level: "debug"
block_interval: "45s"
rate_limit_per_minute: 150
max_body_size_bytes: 2097152
data_dir: "/custom/data"
shutdown_timeout: "60s"
http_client_timeout: "10s"
node_auth_secret: "mysecret"
require_node_auth: true
`
	if err := os.WriteFile(configPath, []byte(yamlContent), 0644); err != nil {
		t.Fatalf("Failed to write test config file: %v", err)
	}

	cfg, err := LoadConfigFromFile(configPath)
	if err != nil {
		t.Fatalf("LoadConfigFromFile failed: %v", err)
	}

	if cfg.Port != "9090" {
		t.Errorf("Expected port '9090', got '%s'", cfg.Port)
	}

	if len(cfg.SeedNodes) != 2 || cfg.SeedNodes[0] != "node1.example.com:8080" {
		t.Errorf("Expected seed nodes from file, got %v", cfg.SeedNodes)
	}

	if cfg.LogLevel != "debug" {
		t.Errorf("Expected log level 'debug', got '%s'", cfg.LogLevel)
	}

	if cfg.BlockInterval != 45*time.Second {
		t.Errorf("Expected block interval 45s, got %v", cfg.BlockInterval)
	}

	if cfg.RateLimitPerMinute != 150 {
		t.Errorf("Expected rate limit 150, got %d", cfg.RateLimitPerMinute)
	}

	if cfg.MaxBodySizeBytes != 2097152 {
		t.Errorf("Expected max body size 2097152, got %d", cfg.MaxBodySizeBytes)
	}

	if cfg.DataDir != "/custom/data" {
		t.Errorf("Expected data dir '/custom/data', got '%s'", cfg.DataDir)
	}

	if cfg.ShutdownTimeout != 60*time.Second {
		t.Errorf("Expected shutdown timeout 60s, got %v", cfg.ShutdownTimeout)
	}

	if cfg.HTTPClientTimeout != 10*time.Second {
		t.Errorf("Expected HTTP client timeout 10s, got %v", cfg.HTTPClientTimeout)
	}

	if cfg.NodeAuthSecret != "mysecret" {
		t.Errorf("Expected node auth secret 'mysecret', got '%s'", cfg.NodeAuthSecret)
	}

	if !cfg.RequireNodeAuth {
		t.Error("Expected require_node_auth to be true")
	}
}

func TestLoadConfigFromJSONFile(t *testing.T) {
	clearConfigEnvVars()

	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.json")

	jsonContent := `{
		"port": "7070",
		"seedNodes": ["jsonnode1:8080", "jsonnode2:8080"],
		"logLevel": "warn",
		"blockInterval": "90s",
		"rateLimitPerMinute": 200,
		"maxBodySizeBytes": 524288,
		"dataDir": "/json/data",
		"shutdownTimeout": "15s",
		"httpClientTimeout": "3s"
	}`
	if err := os.WriteFile(configPath, []byte(jsonContent), 0644); err != nil {
		t.Fatalf("Failed to write test config file: %v", err)
	}

	cfg, err := LoadConfigFromFile(configPath)
	if err != nil {
		t.Fatalf("LoadConfigFromFile failed: %v", err)
	}

	if cfg.Port != "7070" {
		t.Errorf("Expected port '7070', got '%s'", cfg.Port)
	}

	if len(cfg.SeedNodes) != 2 || cfg.SeedNodes[0] != "jsonnode1:8080" {
		t.Errorf("Expected seed nodes from JSON file, got %v", cfg.SeedNodes)
	}

	if cfg.LogLevel != "warn" {
		t.Errorf("Expected log level 'warn', got '%s'", cfg.LogLevel)
	}

	if cfg.BlockInterval != 90*time.Second {
		t.Errorf("Expected block interval 90s, got %v", cfg.BlockInterval)
	}
}

func TestLoadConfigEnvOverridesFile(t *testing.T) {
	clearConfigEnvVars()

	// Create a config file
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")

	yamlContent := `
port: "9090"
log_level: "debug"
block_interval: "45s"
data_dir: "/file/data"
`
	if err := os.WriteFile(configPath, []byte(yamlContent), 0644); err != nil {
		t.Fatalf("Failed to write test config file: %v", err)
	}

	// Set CONFIG_FILE and some env overrides
	os.Setenv("CONFIG_FILE", configPath)
	os.Setenv("PORT", "3000")
	os.Setenv("LOG_LEVEL", "error")

	defer clearConfigEnvVars()

	cfg := LoadConfig()

	// Port should be from env, not file
	if cfg.Port != "3000" {
		t.Errorf("Expected port '3000' from env, got '%s'", cfg.Port)
	}

	// Log level should be from env, not file
	if cfg.LogLevel != "error" {
		t.Errorf("Expected log level 'error' from env, got '%s'", cfg.LogLevel)
	}

	// Block interval should be from file (no env override)
	if cfg.BlockInterval != 45*time.Second {
		t.Errorf("Expected block interval 45s from file, got %v", cfg.BlockInterval)
	}

	// Data dir should be from file (no env override)
	if cfg.DataDir != "/file/data" {
		t.Errorf("Expected data dir '/file/data' from file, got '%s'", cfg.DataDir)
	}
}

func TestLoadConfigMissingFileUsesDefaults(t *testing.T) {
	clearConfigEnvVars()

	// Point to a non-existent config file
	os.Setenv("CONFIG_FILE", "/nonexistent/config.yaml")
	defer clearConfigEnvVars()

	cfg := LoadConfig()

	// Should fall back to defaults
	if cfg.Port != "8080" {
		t.Errorf("Expected default port '8080', got '%s'", cfg.Port)
	}

	if cfg.LogLevel != "info" {
		t.Errorf("Expected default log level 'info', got '%s'", cfg.LogLevel)
	}

	if cfg.BlockInterval != 60*time.Second {
		t.Errorf("Expected default block interval 60s, got %v", cfg.BlockInterval)
	}
}

func TestLoadConfigMalformedYAML(t *testing.T) {
	clearConfigEnvVars()

	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")

	malformedContent := `
port: "9090"
  invalid_indentation: true
seed_nodes:
- missing_quotes
`
	if err := os.WriteFile(configPath, []byte(malformedContent), 0644); err != nil {
		t.Fatalf("Failed to write test config file: %v", err)
	}

	_, err := LoadConfigFromFile(configPath)
	if err == nil {
		t.Error("Expected error for malformed YAML, got nil")
	}
}

func TestLoadConfigMalformedJSON(t *testing.T) {
	clearConfigEnvVars()

	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.json")

	malformedContent := `{"port": "9090", invalid json}`
	if err := os.WriteFile(configPath, []byte(malformedContent), 0644); err != nil {
		t.Fatalf("Failed to write test config file: %v", err)
	}

	_, err := LoadConfigFromFile(configPath)
	if err == nil {
		t.Error("Expected error for malformed JSON, got nil")
	}
}

func TestLoadConfigInvalidDurationInFile(t *testing.T) {
	clearConfigEnvVars()

	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")

	yamlContent := `
port: "9090"
block_interval: "not-a-duration"
`
	if err := os.WriteFile(configPath, []byte(yamlContent), 0644); err != nil {
		t.Fatalf("Failed to write test config file: %v", err)
	}

	_, err := LoadConfigFromFile(configPath)
	if err == nil {
		t.Error("Expected error for invalid duration, got nil")
	}
}

func TestLoadConfigPartialFile(t *testing.T) {
	clearConfigEnvVars()

	// Create a config file with only some values set
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")

	yamlContent := `
port: "5000"
log_level: "warn"
`
	if err := os.WriteFile(configPath, []byte(yamlContent), 0644); err != nil {
		t.Fatalf("Failed to write test config file: %v", err)
	}

	os.Setenv("CONFIG_FILE", configPath)
	defer clearConfigEnvVars()

	cfg := LoadConfig()

	// Values from file
	if cfg.Port != "5000" {
		t.Errorf("Expected port '5000' from file, got '%s'", cfg.Port)
	}

	if cfg.LogLevel != "warn" {
		t.Errorf("Expected log level 'warn' from file, got '%s'", cfg.LogLevel)
	}

	// Default values for unset fields
	if len(cfg.SeedNodes) != 2 {
		t.Errorf("Expected 2 default seed nodes, got %d", len(cfg.SeedNodes))
	}

	if cfg.BlockInterval != 60*time.Second {
		t.Errorf("Expected default block interval 60s, got %v", cfg.BlockInterval)
	}

	if cfg.RateLimitPerMinute != DefaultRateLimitPerMinute {
		t.Errorf("Expected default rate limit %d, got %d", DefaultRateLimitPerMinute, cfg.RateLimitPerMinute)
	}
}

func TestLoadConfigFileWithConfigFileEnv(t *testing.T) {
	clearConfigEnvVars()

	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "custom-config.yaml")

	yamlContent := `
port: "6060"
log_level: "debug"
`
	if err := os.WriteFile(configPath, []byte(yamlContent), 0644); err != nil {
		t.Fatalf("Failed to write test config file: %v", err)
	}

	os.Setenv("CONFIG_FILE", configPath)
	defer clearConfigEnvVars()

	cfg := LoadConfig()

	if cfg.Port != "6060" {
		t.Errorf("Expected port '6060' from CONFIG_FILE, got '%s'", cfg.Port)
	}

	if cfg.LogLevel != "debug" {
		t.Errorf("Expected log level 'debug' from CONFIG_FILE, got '%s'", cfg.LogLevel)
	}
}
