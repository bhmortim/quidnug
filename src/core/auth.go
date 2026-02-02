package main

import (
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"fmt"
	"os"
	"sync"
	"time"
)

// Node authentication header names
const (
	NodeSignatureHeader = "X-Node-Signature"
	NodeTimestampHeader = "X-Node-Timestamp"
)

// NodeAuthTimestampTolerance is the maximum age of a signed request (5 minutes)
const NodeAuthTimestampTolerance = 5 * time.Minute

// Package-level auth configuration loaded once from environment
var (
	nodeAuthConfig struct {
		secret   string
		required bool
	}
	nodeAuthConfigOnce sync.Once
)

// loadNodeAuthConfig loads auth configuration from environment variables
func loadNodeAuthConfig() {
	nodeAuthConfigOnce.Do(func() {
		nodeAuthConfig.secret = os.Getenv("NODE_AUTH_SECRET")
		nodeAuthConfig.required = os.Getenv("REQUIRE_NODE_AUTH") == "true"
	})
}

// GetNodeAuthSecret returns the node authentication secret
func GetNodeAuthSecret() string {
	loadNodeAuthConfig()
	return nodeAuthConfig.secret
}

// IsNodeAuthRequired returns whether node authentication is required
func IsNodeAuthRequired() bool {
	loadNodeAuthConfig()
	return nodeAuthConfig.required
}

// SignRequest creates an HMAC-SHA256 signature for a request.
// The signature covers: method + path + body + timestamp
func SignRequest(method, path string, body []byte, secret string, timestamp int64) string {
	message := fmt.Sprintf("%s\n%s\n%s\n%d", method, path, string(body), timestamp)
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(message))
	return base64.StdEncoding.EncodeToString(mac.Sum(nil))
}

// VerifyRequest verifies the HMAC-SHA256 signature of a request.
// Returns false if the timestamp is stale or the signature doesn't match.
func VerifyRequest(method, path string, body []byte, secret string, timestamp int64, signature string) bool {
	// Verify timestamp is within acceptable window
	now := time.Now().Unix()
	toleranceSec := int64(NodeAuthTimestampTolerance.Seconds())
	if timestamp < now-toleranceSec || timestamp > now+toleranceSec {
		return false
	}

	// Compute expected signature
	expectedSig := SignRequest(method, path, body, secret, timestamp)

	// Use constant-time comparison to prevent timing attacks
	return subtle.ConstantTimeCompare([]byte(signature), []byte(expectedSig)) == 1
}

// ResetNodeAuthConfigForTesting resets the auth config for testing purposes.
// This should only be used in tests.
func ResetNodeAuthConfigForTesting() {
	nodeAuthConfigOnce = sync.Once{}
}
