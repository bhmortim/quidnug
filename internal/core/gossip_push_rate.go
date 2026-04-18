// Package core — gossip_push_rate.go
//
// Per-producer token-bucket rate limiter for push gossip. Bounds
// how many messages a single producer can cause us to forward in
// a rolling window. Separate structure from the IP rate limiter
// because the blame key is the producer QUID, not an IP, and the
// buckets need to survive connection churn.
//
// Semantics:
//
//   - On each push we check gossipRateAllow(producer). If it
//     returns false we apply the message locally (truth still
//     propagates to us) but do NOT forward. This is the
//     "apply-then-choke" rule from QDP-0005 §7.
//
//   - LRU-evict when we hit the bucket cap. Bucket state is not
//     persisted; a restart forgives all producers. Acceptable
//     because the worst case of a forgiven flood is one window
//     of full traffic before the bucket fills again.
package core

import (
	"container/list"
	"sync"
	"time"
)

// GossipProducerRateMax is the default maximum number of pushes
// from a single producer per window that this node will forward.
// Applied locally only; producers are unaware of it.
const GossipProducerRateMax = 30

// GossipProducerRateWindow is the rolling-window size for the
// rate limiter.
const GossipProducerRateWindow = 60 * time.Second

// gossipBucketCap is the maximum number of distinct producers
// tracked concurrently. LRU evicts beyond this. Sized for
// typical deployments (~100 active producers × 10x headroom).
const gossipBucketCap = 1024

// gossipBucket is a simple token bucket with refill-on-demand.
// Not goroutine-safe on its own; callers hold gossipRateMutex.
type gossipBucket struct {
	tokens     float64
	lastRefill time.Time
	elem       *list.Element // position in LRU list
}

// gossipRateState is the rate-limiter's in-memory state. One
// instance per QuidnugNode. The limiter is allocated lazily on
// first use so tests that don't exercise push gossip don't
// allocate anything.
type gossipRateState struct {
	mu      sync.Mutex
	buckets map[string]*gossipBucket
	lru     *list.List // front = most recently used
	max     float64
	window  time.Duration
	cap     int
}

// newGossipRateState returns a configured limiter.
func newGossipRateState() *gossipRateState {
	return &gossipRateState{
		buckets: make(map[string]*gossipBucket),
		lru:     list.New(),
		max:     float64(GossipProducerRateMax),
		window:  GossipProducerRateWindow,
		cap:     gossipBucketCap,
	}
}

// allow reports whether the producer is within their quota. As
// a side effect it consumes one token (if available) and
// updates LRU position.
func (s *gossipRateState) allow(producer string, now time.Time) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	b, ok := s.buckets[producer]
	if !ok {
		// First message from this producer; bucket starts full.
		b = &gossipBucket{
			tokens:     s.max - 1, // this message consumes one
			lastRefill: now,
		}
		b.elem = s.lru.PushFront(producer)
		s.buckets[producer] = b
		s.evictIfOver()
		return true
	}

	// Refill based on elapsed time, capped at max.
	elapsed := now.Sub(b.lastRefill)
	if elapsed > 0 {
		refill := s.max * (float64(elapsed) / float64(s.window))
		b.tokens += refill
		if b.tokens > s.max {
			b.tokens = s.max
		}
		b.lastRefill = now
	}

	// Move to front on any access (LRU touch).
	s.lru.MoveToFront(b.elem)

	if b.tokens < 1.0 {
		return false
	}
	b.tokens--
	return true
}

// evictIfOver drops the least-recently-used bucket if we've
// exceeded the cap. Called under mu.
func (s *gossipRateState) evictIfOver() {
	for s.lru.Len() > s.cap {
		back := s.lru.Back()
		if back == nil {
			return
		}
		key := back.Value.(string)
		s.lru.Remove(back)
		delete(s.buckets, key)
	}
}

// size returns the current number of tracked producers. Testing
// hook.
func (s *gossipRateState) size() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.buckets)
}

// ---------------------------------------------------------------------------
// Node accessor — lazy allocation
// ---------------------------------------------------------------------------

// gossipRateAllow is the public entry point used by the receive
// path. The first call on a given node allocates the limiter.
// producer is the gossip producer QUID.
func (node *QuidnugNode) gossipRateAllow(producer string) bool {
	if producer == "" {
		// Can't rate-limit without an identity. Allow (the message
		// will fail validation anyway).
		return true
	}
	node.gossipRateMutex.Lock()
	if node.gossipRate == nil {
		node.gossipRate = newGossipRateState()
	}
	state := node.gossipRate
	node.gossipRateMutex.Unlock()
	return state.allow(producer, time.Now())
}
