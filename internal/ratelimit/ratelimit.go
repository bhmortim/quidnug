// Package ratelimit provides an IP-keyed token-bucket rate limiter
// with bounded memory.
//
// Why bounded memory matters: an attacker that rotates source IPs
// (directly or by spoofing X-Forwarded-For) can add a fresh entry to
// a naïve per-IP map with every request. Without an eviction policy
// the map grows without bound and the process eventually dies. The
// limiter here caps the map at MaxIPs and evicts entries that have
// been idle longer than IdleTTL, so the steady-state memory is
// O(active attackers × entry-size), not O(everyone who ever hit us).
//
// The package is standalone: it depends only on the standard library
// and golang.org/x/time/rate. HTTP middleware that uses it lives in
// internal/core.
package ratelimit

import (
	"sync"
	"time"

	"golang.org/x/time/rate"
)

// Defaults for IPRateLimiter eviction. Operators may override via
// environment variables in the core package; the ratelimit package
// itself knows nothing about env vars.
const (
	DefaultMaxIPs  = 10_000
	DefaultIdleTTL = 10 * time.Minute
)

type limiterEntry struct {
	limiter  *rate.Limiter
	lastSeen time.Time
}

// IPRateLimiter manages per-IP token-bucket rate limiters.
//
// The limiter map is bounded by maxIPs and entries unused for more
// than idleTTL are evicted on access. Without these bounds an
// attacker that rotates source IPs (or spoofs them via
// X-Forwarded-For) can trivially exhaust server memory.
type IPRateLimiter struct {
	mu       sync.Mutex
	limiters map[string]*limiterEntry
	rate     rate.Limit
	burst    int
	maxIPs   int
	idleTTL  time.Duration
}

// New creates a new IP-based rate limiter with default eviction policy.
func New(requestsPerMinute int) *IPRateLimiter {
	return NewWithEviction(requestsPerMinute, DefaultMaxIPs, DefaultIdleTTL)
}

// NewWithEviction creates a new IP-based rate limiter with an explicit
// eviction policy. maxIPs <= 0 disables the cap; idleTTL <= 0
// disables idle eviction.
func NewWithEviction(requestsPerMinute, maxIPs int, idleTTL time.Duration) *IPRateLimiter {
	r := rate.Limit(float64(requestsPerMinute) / 60.0)
	return &IPRateLimiter{
		limiters: make(map[string]*limiterEntry),
		rate:     r,
		burst:    requestsPerMinute,
		maxIPs:   maxIPs,
		idleTTL:  idleTTL,
	}
}

// GetLimiter returns the rate limiter for a given IP, creating one if
// needed. This also performs opportunistic eviction of idle and
// over-capacity entries.
func (ipl *IPRateLimiter) GetLimiter(ip string) *rate.Limiter {
	ipl.mu.Lock()
	defer ipl.mu.Unlock()

	now := time.Now()

	if entry, exists := ipl.limiters[ip]; exists {
		entry.lastSeen = now
		return entry.limiter
	}

	// Amortized eviction: whenever we'd grow beyond maxIPs, walk the
	// map once and drop idle entries. If still over-budget, evict the
	// single oldest entry.
	if ipl.maxIPs > 0 && len(ipl.limiters) >= ipl.maxIPs {
		ipl.evictLocked(now)
	}

	limiter := rate.NewLimiter(ipl.rate, ipl.burst)
	ipl.limiters[ip] = &limiterEntry{limiter: limiter, lastSeen: now}
	return limiter
}

// evictLocked drops idle entries, then (if still at capacity) the
// oldest one. Caller must hold ipl.mu.
func (ipl *IPRateLimiter) evictLocked(now time.Time) {
	if ipl.idleTTL > 0 {
		cutoff := now.Add(-ipl.idleTTL)
		for ip, entry := range ipl.limiters {
			if entry.lastSeen.Before(cutoff) {
				delete(ipl.limiters, ip)
			}
		}
	}
	if ipl.maxIPs > 0 && len(ipl.limiters) >= ipl.maxIPs {
		var oldestIP string
		var oldestSeen time.Time
		first := true
		for ip, entry := range ipl.limiters {
			if first || entry.lastSeen.Before(oldestSeen) {
				oldestIP = ip
				oldestSeen = entry.lastSeen
				first = false
			}
		}
		if oldestIP != "" {
			delete(ipl.limiters, oldestIP)
		}
	}
}

// Size returns the current number of tracked IPs. Used in tests.
func (ipl *IPRateLimiter) Size() int {
	ipl.mu.Lock()
	defer ipl.mu.Unlock()
	return len(ipl.limiters)
}
