package config

import "os"

// ClearConfigEnvVarsForTesting clears every environment variable
// LoadConfig consults. Tests that mutate any of these MUST call this
// before and after (usually as defer) to prevent state leakage into
// the rest of the suite. Missing an entry here has bitten us before —
// a TrustCacheTTL leak caused TestLoadConfigTrustCacheTTLDefault to
// fail when run after TestLoadConfigTrustCacheTTLFromEnv.
//
// Exported (despite the "ForTesting" suffix signalling intent) so
// test code in other packages can call it. Keep the list aligned
// with every os.Getenv call in config.go.
func ClearConfigEnvVarsForTesting() {
	for _, k := range []string{
		"CONFIG_FILE",
		"PORT",
		"SEED_NODES",
		"LOG_LEVEL",
		"BLOCK_INTERVAL",
		"RATE_LIMIT_PER_MINUTE",
		"MAX_BODY_SIZE_BYTES",
		"DATA_DIR",
		"SHUTDOWN_TIMEOUT",
		"HTTP_CLIENT_TIMEOUT",
		"NODE_AUTH_SECRET",
		"REQUIRE_NODE_AUTH",
		"QUIDNUG_IPFS_ENABLED",
		"QUIDNUG_IPFS_GATEWAY_URL",
		"QUIDNUG_IPFS_TIMEOUT",
		"SUPPORTED_DOMAINS",
		"ALLOW_DOMAIN_REGISTRATION",
		"REQUIRE_PARENT_DOMAIN_AUTH",
		"TRUST_CACHE_TTL",
		"DOMAIN_GOSSIP_INTERVAL",
		"DOMAIN_GOSSIP_TTL",
	} {
		os.Unsetenv(k)
	}
}
