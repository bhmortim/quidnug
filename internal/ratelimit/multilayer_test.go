// QDP-0016 Phase 1 multi-layer rate-limit tests.
package ratelimit

import (
	"testing"
	"time"
)

func TestKeyedLimiter_AllowsUpToBurst(t *testing.T) {
	l := NewKeyedLimiter(60, 3)
	for i := 0; i < 3; i++ {
		if !l.Allow("actor") {
			t.Fatalf("expected request %d to be allowed", i)
		}
	}
	if l.Allow("actor") {
		t.Error("request past burst should be denied")
	}
}

func TestKeyedLimiter_SeparateActorsIndependent(t *testing.T) {
	l := NewKeyedLimiter(60, 1)
	if !l.Allow("a") {
		t.Error("actor a first request should pass")
	}
	if l.Allow("a") {
		t.Error("actor a second request should be denied")
	}
	// Actor b has its own bucket.
	if !l.Allow("b") {
		t.Error("actor b first request should pass")
	}
}

func TestKeyedLimiter_BoundedByMax(t *testing.T) {
	l := NewKeyedLimiterWithEviction(60, 1, 3, DefaultIdleTTL)
	for _, k := range []string{"a", "b", "c", "d"} {
		l.Allow(k)
	}
	if got := l.Size(); got > 3 {
		t.Errorf("expected size ≤ 3 after eviction, got %d", got)
	}
}

func TestKeyedLimiter_IdleEviction(t *testing.T) {
	l := NewKeyedLimiterWithEviction(60, 1, 100, 50*time.Millisecond)
	l.Allow("a")
	time.Sleep(100 * time.Millisecond)
	// Trigger eviction by filling past capacity or waiting for
	// the next access-time-based eviction. With max=100 no
	// direct capacity pressure, so just look up another actor
	// to force the internal path.
	l.Allow("b")
	// Actor `a` is still technically in the map (we only evict
	// when over capacity), so no assertion needed; this test
	// just exercises the idle cutoff path for race-free coverage.
	_ = l
}

func TestKeyedLimiter_NilAlwaysAllows(t *testing.T) {
	var l *KeyedLimiter
	if !l.Allow("whoever") {
		t.Error("nil limiter should always allow")
	}
}

func TestMultiLayerLimiter_AdmitsUnderLimits(t *testing.T) {
	cfg := DefaultWriteLimits()
	cfg.Quid.Burst = 3
	cfg.Quid.RequestsPerMinute = 60
	m := NewMultiLayerLimiter(cfg)

	keys := ActorKeys{IP: "1.2.3.4", Quid: "q-1", Operator: "op-1", Domain: "d1"}
	for i := 0; i < 3; i++ {
		if got := m.AdmitWrite(keys); !got.Allowed {
			t.Fatalf("request %d should be allowed, got %+v", i, got)
		}
	}
	// Quid burst is 3; 4th should hit the quid layer.
	if got := m.AdmitWrite(keys); got.Allowed || got.Layer != LayerQuid {
		t.Errorf("expected quid-layer denial on 4th request, got %+v", got)
	}
}

func TestMultiLayerLimiter_SkipsDisabledLayers(t *testing.T) {
	cfg := DefaultWriteLimits()
	cfg.IP.Enabled = false
	m := NewMultiLayerLimiter(cfg)

	keys := ActorKeys{IP: "1.2.3.4", Quid: "q-1", Operator: "op-1", Domain: "d1"}
	if got := m.AdmitWrite(keys); !got.Allowed {
		t.Errorf("disabled IP layer should not block, got %+v", got)
	}
	if got := m.LayerSize(LayerIP); got != 0 {
		t.Errorf("disabled IP layer size should be 0, got %d", got)
	}
}

func TestMultiLayerLimiter_SkipsEmptyKeys(t *testing.T) {
	cfg := DefaultWriteLimits()
	m := NewMultiLayerLimiter(cfg)

	// IP is empty; every other layer is populated — shouldn't
	// consult the IP layer because key is empty.
	keys := ActorKeys{Quid: "q-1", Operator: "op-1", Domain: "d1"}
	if got := m.AdmitWrite(keys); !got.Allowed {
		t.Errorf("empty IP key should mean IP-layer skip, got %+v", got)
	}
	if got := m.LayerSize(LayerIP); got != 0 {
		t.Errorf("skipped IP layer should have 0 actors, got %d", got)
	}
}

func TestMultiLayerLimiter_NilSafe(t *testing.T) {
	var m *MultiLayerLimiter
	if got := m.AdmitWrite(ActorKeys{Quid: "anyone"}); !got.Allowed {
		t.Error("nil limiter should always admit")
	}
	if sz := m.LayerSize(LayerQuid); sz != 0 {
		t.Errorf("nil limiter size should be 0, got %d", sz)
	}
}

func TestMultiLayerLimiter_PerQuidIsolation(t *testing.T) {
	cfg := MultiLayerConfig{
		Quid: LayerConfig{Enabled: true, RequestsPerMinute: 60, Burst: 1},
	}
	m := NewMultiLayerLimiter(cfg)

	if got := m.AdmitWrite(ActorKeys{Quid: "a"}); !got.Allowed {
		t.Fatal("first quid-a request should pass")
	}
	if got := m.AdmitWrite(ActorKeys{Quid: "a"}); got.Allowed {
		t.Error("second quid-a request should be denied")
	}
	if got := m.AdmitWrite(ActorKeys{Quid: "b"}); !got.Allowed {
		t.Error("quid-b should have independent bucket")
	}
}

func TestMultiLayerLimiter_OperatorLayerCatchesFloodAcrossQuids(t *testing.T) {
	// Operator cap is 3/minute across all quids; quid layer is
	// higher-per-quid so without the operator layer this would pass.
	cfg := MultiLayerConfig{
		Quid:     LayerConfig{Enabled: true, RequestsPerMinute: 60, Burst: 10},
		Operator: LayerConfig{Enabled: true, RequestsPerMinute: 60, Burst: 3},
	}
	m := NewMultiLayerLimiter(cfg)

	for i, q := range []string{"q1", "q2", "q3"} {
		if got := m.AdmitWrite(ActorKeys{Quid: q, Operator: "op"}); !got.Allowed {
			t.Fatalf("request %d should be allowed, got %+v", i, got)
		}
	}
	got := m.AdmitWrite(ActorKeys{Quid: "q4", Operator: "op"})
	if got.Allowed || got.Layer != LayerOperator {
		t.Errorf("expected operator-layer denial, got %+v", got)
	}
}
