// Multi-layer rate limiting for QDP-0016 Phase 1.
//
// The existing IPRateLimiter in this package covers the
// network-perimeter layer — it's keyed on client IP and
// applied at HTTP ingress. QDP-0016 §3 describes four additional
// scales that fire at the mempool-admission layer (after
// signature verification, before block inclusion):
//
//   - per-quid — the signer's identity-scoped budget
//   - per-epoch-key — rotation-aware counter (Phase 4)
//   - per-operator — aggregated across an operator's managed quids
//   - per-domain — target-scoped protection
//
// This file implements those layers as a single MultiLayerLimiter.
// The IP layer stays in ratelimit.go because it's used by HTTP
// middleware; callers that want the full stack pass the IP as a
// separate layer to AdmitWrite.
package ratelimit

import (
	"sync"
	"time"

	"golang.org/x/time/rate"
)

// Layer is a string tag identifying which scale a rate-limit
// decision comes from. Used for telemetry labels and for
// explaining rejections to callers.
type Layer string

const (
	LayerIP       Layer = "ip"
	LayerQuid     Layer = "quid"
	LayerOperator Layer = "operator"
	LayerDomain   Layer = "domain"
)

// LayerConfig holds one layer's token-bucket parameters.
type LayerConfig struct {
	// Enabled toggles this layer in AdmitWrite. Disabled
	// layers are always-allowed.
	Enabled bool
	// RequestsPerMinute is the steady-state refill rate.
	RequestsPerMinute int
	// Burst is the maximum the bucket can hold; also the
	// initial capacity for new actors.
	Burst int
	// MaxActors caps the per-layer actor map. 0 = uses
	// DefaultMaxIPs.
	MaxActors int
	// IdleTTL evicts actors with no recent activity. 0 =
	// uses DefaultIdleTTL.
	IdleTTL time.Duration
}

// MultiLayerConfig bundles the four layer configs.
type MultiLayerConfig struct {
	IP       LayerConfig
	Quid     LayerConfig
	Operator LayerConfig
	Domain   LayerConfig
}

// DefaultWriteLimits returns the QDP-0016 §3 recommended
// defaults for the write-admission path. Operators override
// via config YAML (Phase 6 work).
func DefaultWriteLimits() MultiLayerConfig {
	return MultiLayerConfig{
		IP: LayerConfig{
			Enabled:           true,
			RequestsPerMinute: 60,
			Burst:             15,
		},
		Quid: LayerConfig{
			Enabled:           true,
			RequestsPerMinute: 10,
			Burst:             10,
		},
		Operator: LayerConfig{
			Enabled:           true,
			RequestsPerMinute: 2000,
			Burst:             500,
		},
		Domain: LayerConfig{
			Enabled:           true,
			RequestsPerMinute: 1000,
			Burst:             200,
		},
	}
}

// KeyedLimiter is the generic token-bucket map that underlies
// every layer except the pre-existing IP one (which uses
// IPRateLimiter; kept separate for backward-compat of the HTTP
// middleware API).
//
// Design mirrors IPRateLimiter: bounded map, idle eviction,
// amortized cleanup on grow.
type KeyedLimiter struct {
	mu       sync.Mutex
	limiters map[string]*limiterEntry
	rate     rate.Limit
	burst    int
	max      int
	idleTTL  time.Duration
}

// NewKeyedLimiter constructs a limiter with the given
// refill-rate-per-minute and burst size.
func NewKeyedLimiter(requestsPerMinute, burst int) *KeyedLimiter {
	return NewKeyedLimiterWithEviction(requestsPerMinute, burst, DefaultMaxIPs, DefaultIdleTTL)
}

// NewKeyedLimiterWithEviction is the fully-parameterized form
// for tests and operators that want non-default bounds.
func NewKeyedLimiterWithEviction(requestsPerMinute, burst, max int, idleTTL time.Duration) *KeyedLimiter {
	if burst <= 0 {
		burst = requestsPerMinute
	}
	return &KeyedLimiter{
		limiters: make(map[string]*limiterEntry),
		rate:     rate.Limit(float64(requestsPerMinute) / 60.0),
		burst:    burst,
		max:      max,
		idleTTL:  idleTTL,
	}
}

// Allow returns whether a single request for this key would be
// admitted right now. Spends a token on success.
func (k *KeyedLimiter) Allow(key string) bool {
	if k == nil || key == "" {
		return true
	}
	l := k.getLimiter(key)
	return l.Allow()
}

// Size returns the current actor count; used in tests.
func (k *KeyedLimiter) Size() int {
	k.mu.Lock()
	defer k.mu.Unlock()
	return len(k.limiters)
}

func (k *KeyedLimiter) getLimiter(key string) *rate.Limiter {
	k.mu.Lock()
	defer k.mu.Unlock()
	now := time.Now()
	if e, ok := k.limiters[key]; ok {
		e.lastSeen = now
		return e.limiter
	}
	if k.max > 0 && len(k.limiters) >= k.max {
		k.evictLocked(now)
	}
	l := rate.NewLimiter(k.rate, k.burst)
	k.limiters[key] = &limiterEntry{limiter: l, lastSeen: now}
	return l
}

func (k *KeyedLimiter) evictLocked(now time.Time) {
	if k.idleTTL > 0 {
		cutoff := now.Add(-k.idleTTL)
		for key, e := range k.limiters {
			if e.lastSeen.Before(cutoff) {
				delete(k.limiters, key)
			}
		}
	}
	if k.max > 0 && len(k.limiters) >= k.max {
		var oldestKey string
		var oldestSeen time.Time
		first := true
		for key, e := range k.limiters {
			if first || e.lastSeen.Before(oldestSeen) {
				oldestKey = key
				oldestSeen = e.lastSeen
				first = false
			}
		}
		if oldestKey != "" {
			delete(k.limiters, oldestKey)
		}
	}
}

// ActorKeys bundles the actor identifiers for one write.
// Any field may be empty; empty means "skip this layer."
type ActorKeys struct {
	IP       string
	Quid     string
	Operator string
	Domain   string
}

// Decision is the result of an AdmitWrite call.
type Decision struct {
	Allowed bool
	// Layer identifies which layer denied the write. Empty
	// when Allowed is true.
	Layer Layer
}

// Denied constructs a rejection decision attributing the
// rejection to the given layer.
func Denied(layer Layer) Decision {
	return Decision{Allowed: false, Layer: layer}
}

// Allowed returns an accept decision.
func AllowedDecision() Decision {
	return Decision{Allowed: true}
}

// MultiLayerLimiter composes the four post-HTTP layers. Each
// layer is independent; a write is admitted only if *every*
// enabled layer allows it.
type MultiLayerLimiter struct {
	cfg MultiLayerConfig
	ip  *KeyedLimiter
	qd  *KeyedLimiter
	op  *KeyedLimiter
	dm  *KeyedLimiter
}

// NewMultiLayerLimiter constructs a new multi-layer limiter
// wired to the provided config.
func NewMultiLayerLimiter(cfg MultiLayerConfig) *MultiLayerLimiter {
	mll := &MultiLayerLimiter{cfg: cfg}
	if cfg.IP.Enabled {
		mll.ip = NewKeyedLimiterWithEviction(cfg.IP.RequestsPerMinute, cfg.IP.Burst,
			pickMax(cfg.IP.MaxActors), pickTTL(cfg.IP.IdleTTL))
	}
	if cfg.Quid.Enabled {
		mll.qd = NewKeyedLimiterWithEviction(cfg.Quid.RequestsPerMinute, cfg.Quid.Burst,
			pickMax(cfg.Quid.MaxActors), pickTTL(cfg.Quid.IdleTTL))
	}
	if cfg.Operator.Enabled {
		mll.op = NewKeyedLimiterWithEviction(cfg.Operator.RequestsPerMinute, cfg.Operator.Burst,
			pickMax(cfg.Operator.MaxActors), pickTTL(cfg.Operator.IdleTTL))
	}
	if cfg.Domain.Enabled {
		mll.dm = NewKeyedLimiterWithEviction(cfg.Domain.RequestsPerMinute, cfg.Domain.Burst,
			pickMax(cfg.Domain.MaxActors), pickTTL(cfg.Domain.IdleTTL))
	}
	return mll
}

func pickMax(v int) int {
	if v <= 0 {
		return DefaultMaxIPs
	}
	return v
}

func pickTTL(v time.Duration) time.Duration {
	if v <= 0 {
		return DefaultIdleTTL
	}
	return v
}

// AdmitWrite consults every enabled layer and returns an
// aggregate decision. Layers are evaluated in this order:
// IP → quid → operator → domain, matching design-doc §3.6.
//
// A layer is skipped when:
//   - its config is disabled, OR
//   - the ActorKeys field for that layer is empty (no actor
//     to charge against).
//
// Each allowed layer spends one token; on rejection, no tokens
// are spent on later layers.
func (m *MultiLayerLimiter) AdmitWrite(keys ActorKeys) Decision {
	if m == nil {
		return AllowedDecision()
	}
	if m.ip != nil && keys.IP != "" {
		if !m.ip.Allow(keys.IP) {
			return Denied(LayerIP)
		}
	}
	if m.qd != nil && keys.Quid != "" {
		if !m.qd.Allow(keys.Quid) {
			return Denied(LayerQuid)
		}
	}
	if m.op != nil && keys.Operator != "" {
		if !m.op.Allow(keys.Operator) {
			return Denied(LayerOperator)
		}
	}
	if m.dm != nil && keys.Domain != "" {
		if !m.dm.Allow(keys.Domain) {
			return Denied(LayerDomain)
		}
	}
	return AllowedDecision()
}

// LayerSize returns the current actor count for the named
// layer; used for introspection endpoints and tests. Returns
// zero for disabled layers.
func (m *MultiLayerLimiter) LayerSize(layer Layer) int {
	if m == nil {
		return 0
	}
	switch layer {
	case LayerIP:
		if m.ip != nil {
			return m.ip.Size()
		}
	case LayerQuid:
		if m.qd != nil {
			return m.qd.Size()
		}
	case LayerOperator:
		if m.op != nil {
			return m.op.Size()
		}
	case LayerDomain:
		if m.dm != nil {
			return m.dm.Size()
		}
	}
	return 0
}

// Config returns the active configuration; operators can use
// this for introspection endpoints without exposing internal
// state.
func (m *MultiLayerLimiter) Config() MultiLayerConfig {
	if m == nil {
		return MultiLayerConfig{}
	}
	return m.cfg
}
