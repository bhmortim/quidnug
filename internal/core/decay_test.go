package core

import (
	"math"
	"testing"
)

// Helper: within rough tolerance for float comparisons.
func approxEq(got, want, tol float64) bool {
	return math.Abs(got-want) <= tol
}

func TestEdgeDecayFactor_DisabledWhenHalfLifeZero(t *testing.T) {
	if got := EdgeDecayFactor(1000, 0, 0.0); got != 1.0 {
		t.Errorf("halfLife=0 should disable decay, got %v", got)
	}
}

func TestEdgeDecayFactor_DisabledWhenAgeZero(t *testing.T) {
	if got := EdgeDecayFactor(0, 100, 0.0); got != 1.0 {
		t.Errorf("age=0 should be fresh (factor=1), got %v", got)
	}
}

func TestEdgeDecayFactor_HalfLife(t *testing.T) {
	// After exactly one half-life, factor should be 0.5.
	got := EdgeDecayFactor(100, 100, 0.0)
	if !approxEq(got, 0.5, 1e-9) {
		t.Errorf("at 1 half-life: want 0.5, got %v", got)
	}
}

func TestEdgeDecayFactor_TwoHalfLives(t *testing.T) {
	got := EdgeDecayFactor(200, 100, 0.0)
	if !approxEq(got, 0.25, 1e-9) {
		t.Errorf("at 2 half-lives: want 0.25, got %v", got)
	}
}

func TestEdgeDecayFactor_FloorClips(t *testing.T) {
	// 10 half-lives = 2^-10 ≈ 0.000976; floor=0.2 should clip to 0.2.
	got := EdgeDecayFactor(1000, 100, 0.2)
	if !approxEq(got, 0.2, 1e-9) {
		t.Errorf("floor clip failed: want 0.2, got %v", got)
	}
}

func TestDefaultDecayConfig_TwoYearHalfLife(t *testing.T) {
	cfg := DefaultDecayConfig()
	twoYears := int64(2 * 365 * 24 * 3600)
	if cfg.HalfLifeSeconds != twoYears {
		t.Errorf("default half-life: want %d, got %d", twoYears, cfg.HalfLifeSeconds)
	}
	if !approxEq(cfg.Floor, 0.2, 1e-9) {
		t.Errorf("default floor: want 0.2, got %v", cfg.Floor)
	}
}

func TestDecayConfig_PerDomainOverride(t *testing.T) {
	cfg := DecayConfig{
		HalfLifeSeconds: 100,
		Floor:           0.3,
		PerDomain: map[string]DecayOverride{
			"fast-fading.domain": {HalfLifeSeconds: 10, Floor: 0.05},
		},
	}
	hl, floor := cfg.effectiveForDomain("fast-fading.domain")
	if hl != 10 || floor != 0.05 {
		t.Errorf("per-domain override failed: got (%d, %v)", hl, floor)
	}
	hl, floor = cfg.effectiveForDomain("other.domain")
	if hl != 100 || floor != 0.3 {
		t.Errorf("default fallthrough failed: got (%d, %v)", hl, floor)
	}
}

// End-to-end test: build a small graph, advance time, verify
// decayed trust computation matches expected values.
func TestComputeRelationalTrustWithDecay_BasicPath(t *testing.T) {
	node := newTestNode()

	domain := "decay.test"
	node.TrustDomains[domain] = TrustDomain{
		Name:           domain,
		TrustThreshold: 0.5,
		Validators:     map[string]float64{node.NodeID: 1.0},
	}

	// Three quids: alice --trusts--> bob --trusts--> carol.
	// All edges timestamped at t=1000. Weights nominally 0.9.
	baseTS := int64(1000)
	node.TrustRegistry["alice"] = map[string]float64{"bob": 0.9}
	node.TrustRegistry["bob"] = map[string]float64{"carol": 0.9}
	node.TrustEdgeTimestampRegistry["alice"] = map[string]int64{"bob": baseTS}
	node.TrustEdgeTimestampRegistry["bob"] = map[string]int64{"carol": baseTS}

	// At t=1000, decay=1.0, trust = 0.9 * 0.9 = 0.81.
	cfg := DecayConfig{HalfLifeSeconds: 3600, Floor: 0.0}
	tr, _, err := node.computeRelationalTrustDecayAt("alice", "carol", 5, cfg, baseTS)
	if err != nil {
		t.Fatal(err)
	}
	if !approxEq(tr, 0.81, 1e-9) {
		t.Errorf("fresh: want 0.81, got %v", tr)
	}

	// At t=baseTS+3600 (1 half-life per edge), each edge decays
	// to 0.5*0.9 = 0.45. Path trust = 0.45*0.45 = 0.2025.
	tr, _, err = node.computeRelationalTrustDecayAt("alice", "carol", 5, cfg, baseTS+3600)
	if err != nil {
		t.Fatal(err)
	}
	if !approxEq(tr, 0.2025, 1e-9) {
		t.Errorf("one-half-life decay: want 0.2025, got %v", tr)
	}

	// At t=baseTS+1e9 (effectively infinite) with floor=0.1:
	// each edge decays to 0.1*0.9 = 0.09, path = 0.09*0.09 = 0.0081.
	cfgFloor := DecayConfig{HalfLifeSeconds: 3600, Floor: 0.1}
	tr, _, err = node.computeRelationalTrustDecayAt("alice", "carol", 5, cfgFloor, baseTS+1_000_000_000)
	if err != nil {
		t.Fatal(err)
	}
	want := 0.09 * 0.09
	if !approxEq(tr, want, 1e-9) {
		t.Errorf("floor clamp path: want %v, got %v", want, tr)
	}
}

func TestGetDirectTrusteesDecayed_NoTimestampPassesThrough(t *testing.T) {
	node := newTestNode()
	// Edge with no timestamp entry: should pass undecayed.
	node.TrustRegistry["a"] = map[string]float64{"b": 0.8}
	// Explicitly no TrustEdgeTimestampRegistry entry.
	cfg := DefaultDecayConfig()
	trustees := node.GetDirectTrusteesDecayed("a", int64(999_999_999), cfg)
	if v, ok := trustees["b"]; !ok || !approxEq(v, 0.8, 1e-9) {
		t.Errorf("expected 0.8 (no decay without ts), got %v (present=%v)", v, ok)
	}
}

func TestDecayConfig_ZeroValueIsNoDecay(t *testing.T) {
	// Zero DecayConfig{} has HalfLifeSeconds=0 which disables decay.
	cfg := DecayConfig{}
	got := EdgeDecayFactor(999_999_999, cfg.HalfLifeSeconds, cfg.Floor)
	if got != 1.0 {
		t.Errorf("zero-value config should not decay: got %v", got)
	}
}
