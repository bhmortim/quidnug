// Package core — QDP-0022 TTL / ValidUntil support.
//
// This file centralizes the "is this trust edge / record still
// valid at time T" checks. TrustTransactions have had a
// ValidUntil field since day one but it was never enforced;
// QDP-0022 wires it up end-to-end:
//
//   - Block-level validator rejects txs whose ValidUntil is
//     already in the past.
//   - Trust registry parallel-tracks expiry per (truster, trustee).
//   - ComputeRelationalTrust (and friends) skip expired edges.
//   - EventTransaction payloads may set `expiresAt`
//     (UnixNano) for self-expiring events (e.g.,
//     short-lived consents, session tokens).
//
// All checks use a single clock source (time.Now) so test
// harnesses can freeze time with an override helper.
package core

import (
	"sync/atomic"
	"time"
)

// clockOverrideNano lets tests fix "now" to a specific
// instant. Zero means use real wall-clock time.
var clockOverrideNano atomic.Int64

// setTestClockNano freezes the TTL clock for tests. Pass 0
// to revert to real-time. Test-only helper; avoid in
// production code paths.
func setTestClockNano(n int64) {
	clockOverrideNano.Store(n)
}

// nowNano returns the current time in Unix nanoseconds. Tests
// can freeze this via setTestClockNano.
func nowNano() int64 {
	if v := clockOverrideNano.Load(); v != 0 {
		return v
	}
	return time.Now().UnixNano()
}

// nowUnix returns the current time in Unix seconds. Tests can
// freeze this via setTestClockNano.
func nowUnix() int64 {
	return nowNano() / int64(time.Second)
}

// IsTrustEdgeValid returns true if the edge (truster, trustee)
// has either no expiry set OR an expiry in the future.
// Returns true for untracked edges (an edge not yet in the
// registry is "not expired" — that's the caller's problem).
//
// Safe for concurrent use.
func (node *QuidnugNode) IsTrustEdgeValid(truster, trustee string) bool {
	if node.TrustExpiryRegistry == nil {
		return true
	}
	node.TrustRegistryMutex.RLock()
	defer node.TrustRegistryMutex.RUnlock()
	return node.isTrustEdgeValidLocked(truster, trustee)
}

// isTrustEdgeValidLocked is the lock-free variant used by
// callers that already hold TrustRegistryMutex (for reading or
// writing). Re-acquiring an RLock in that situation can deadlock
// against a pending writer, so this split exists purely to avoid
// that hazard.
func (node *QuidnugNode) isTrustEdgeValidLocked(truster, trustee string) bool {
	if node.TrustExpiryRegistry == nil {
		return true
	}
	trusteeMap, ok := node.TrustExpiryRegistry[truster]
	if !ok {
		return true
	}
	validUntil, ok := trusteeMap[trustee]
	if !ok {
		return true
	}
	if validUntil == 0 {
		return true // explicitly "no expiry"
	}
	return validUntil > nowUnix()
}

// GetTrustEdgeExpiry returns (validUntil, hasExpiry). Zero
// second return means "no explicit expiry set."
func (node *QuidnugNode) GetTrustEdgeExpiry(truster, trustee string) (int64, bool) {
	if node.TrustExpiryRegistry == nil {
		return 0, false
	}
	node.TrustRegistryMutex.RLock()
	defer node.TrustRegistryMutex.RUnlock()
	trusteeMap, ok := node.TrustExpiryRegistry[truster]
	if !ok {
		return 0, false
	}
	validUntil, ok := trusteeMap[trustee]
	if !ok {
		return 0, false
	}
	return validUntil, validUntil != 0
}

// IsEventPayloadExpired inspects an EventTransaction's payload
// for an `expiresAt` field (Unix nanoseconds, int64/float64)
// and returns true if it's set and in the past. False means
// "no expiry set" OR "expiry is in the future."
//
// Events with expired payloads are not removed from the chain
// (append-only). Serving layers use this to filter them from
// API responses.
func IsEventPayloadExpired(payload map[string]interface{}) bool {
	if payload == nil {
		return false
	}
	raw, ok := payload["expiresAt"]
	if !ok {
		return false
	}
	var expiresAt int64
	switch v := raw.(type) {
	case int64:
		expiresAt = v
	case int:
		expiresAt = int64(v)
	case float64:
		// JSON unmarshaling yields float64 for numbers
		expiresAt = int64(v)
	default:
		return false
	}
	if expiresAt == 0 {
		return false
	}
	return expiresAt < nowNano()
}
