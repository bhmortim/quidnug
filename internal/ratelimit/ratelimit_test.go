// Package ratelimit — ratelimit_test.go
//
// Methodology
// -----------
// IPRateLimiter's correctness rests on two properties: the token-
// bucket algorithm (inherited from golang.org/x/time/rate, which we
// trust as a dependency) and the memory-bound eviction policy
// (original to this package). These tests focus on the eviction:
//
//   - basic lifecycle: creating, reusing, separating IPs
//   - idle-TTL eviction triggers when new entries push past capacity
//     AND idle entries exist
//   - cap-only eviction drops the oldest-seen entry when idle TTL
//     doesn't trim enough
//   - Size() reports accurately for off-by-one regression
//
// Eviction tests use NewWithEviction to construct small synthetic
// caches (maxIPs=2 or 3) so the scenarios are deterministic without
// manufacturing 10 000 fake IPs.
package ratelimit

import (
	"testing"
	"time"
)

func TestIPRateLimiter_Basics(t *testing.T) {
	t.Run("creates new limiter for unknown IP", func(t *testing.T) {
		limiter := New(100)
		if limiter.GetLimiter("192.168.1.1") == nil {
			t.Fatal("expected non-nil limiter")
		}
	})

	t.Run("returns same limiter for same IP", func(t *testing.T) {
		limiter := New(100)
		a := limiter.GetLimiter("192.168.1.1")
		b := limiter.GetLimiter("192.168.1.1")
		if a != b {
			t.Fatal("expected identical limiter for repeated IP")
		}
	})

	t.Run("returns different limiters for different IPs", func(t *testing.T) {
		limiter := New(100)
		a := limiter.GetLimiter("192.168.1.1")
		b := limiter.GetLimiter("192.168.1.2")
		if a == b {
			t.Fatal("expected distinct limiters for distinct IPs")
		}
	})
}

func TestIPRateLimiter_EvictsByIdleTTL(t *testing.T) {
	limiter := NewWithEviction(100, 3, 50*time.Millisecond)
	limiter.GetLimiter("1.1.1.1")
	limiter.GetLimiter("2.2.2.2")
	limiter.GetLimiter("3.3.3.3")
	if got := limiter.Size(); got != 3 {
		t.Fatalf("pre-sleep size: want 3, got %d", got)
	}

	time.Sleep(60 * time.Millisecond)
	limiter.GetLimiter("4.4.4.4")
	if got := limiter.Size(); got != 1 {
		t.Fatalf("post-sleep size: want 1 (only 4.4.4.4 survives), got %d", got)
	}
}

func TestIPRateLimiter_EvictsByMaxIPs(t *testing.T) {
	limiter := NewWithEviction(100, 2, 10*time.Second)
	limiter.GetLimiter("1.1.1.1")
	time.Sleep(2 * time.Millisecond)
	limiter.GetLimiter("2.2.2.2")
	limiter.GetLimiter("3.3.3.3")
	if got := limiter.Size(); got != 2 {
		t.Fatalf("size at cap: want 2, got %d", got)
	}
}

func TestIPRateLimiter_SizeReportsAccurately(t *testing.T) {
	limiter := NewWithEviction(100, 0, 0) // no eviction
	for i := 1; i <= 10; i++ {
		ip := "10.0.0." + string(rune('0'+i))
		limiter.GetLimiter(ip)
		if got := limiter.Size(); got != i {
			t.Fatalf("after %d inserts, Size() = %d", i, got)
		}
	}
}
