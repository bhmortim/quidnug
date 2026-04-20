// decay.go — QDP-0019 Phase 1 reputation decay.
//
// Phase 1 scope (from `docs/design/0019-reputation-decay.md`
// §7): observer-side decay computation. The wire format is
// unchanged — `TrustTransaction.Timestamp` is already part of
// the signed tx, so nodes already know when each edge was
// last refreshed. The timestamp registry populated in
// `updateTrustRegistry` makes this accessible at query time.
//
// This module provides:
//
//   1. `DecayConfig` — per-domain decay parameters.
//   2. `EdgeDecayFactor` — the decayed multiplier for an
//      edge given (age, config).
//   3. `GetDirectTrusteesDecayed` — parallel to
//      `GetDirectTrustees` but applies decay to every edge.
//   4. `ComputeRelationalTrustWithDecay` — trust walk with
//      decay applied edge-wise.
//
// The existing `ComputeRelationalTrust` / `GetDirectTrustees`
// keep current behavior (no decay). Callers opt in to decay
// by using the `*WithDecay` variants. This preserves
// backward compatibility while landing the primitive.
//
// Phases 2-5 (config surface, passive re-endorsement,
// dormancy, metrics) are deliberately out of scope here.

package core

import (
	"math"
	"time"
)

// DecayConfig parameters per QDP-0019 §3.2.
//
// Zero-value config (DecayConfig{}) disables decay (half-life
// of zero is treated as "no decay"). Callers who want the
// reference default should use DefaultDecayConfig().
type DecayConfig struct {
	// HalfLifeSeconds is the age at which an edge's effective
	// weight is half its nominal. Zero disables decay.
	HalfLifeSeconds int64

	// Floor is the minimum effective-weight fraction; even
	// very old edges never decay below NominalWeight*Floor.
	// Range [0.0, 1.0]. Zero means no floor (asymptotic
	// decay to zero).
	Floor float64

	// PerDomain overrides. Key is trust domain name; missing
	// keys fall through to the top-level HalfLifeSeconds /
	// Floor.
	PerDomain map[string]DecayOverride
}

// DecayOverride lets a specific trust domain use different
// decay parameters than the observer's default.
type DecayOverride struct {
	HalfLifeSeconds int64
	Floor           float64
}

// DefaultDecayConfig returns the reference default: 2-year
// half-life, 0.2 floor. Matches the default in QDP-0019 §3.2.
func DefaultDecayConfig() DecayConfig {
	return DecayConfig{
		HalfLifeSeconds: 2 * 365 * 24 * 3600, // 2 years
		Floor:           0.2,
	}
}

// effectiveForDomain returns the half-life and floor that
// apply to the given domain, preferring a PerDomain override
// if one exists.
func (c DecayConfig) effectiveForDomain(domain string) (halfLife int64, floor float64) {
	if override, ok := c.PerDomain[domain]; ok {
		return override.HalfLifeSeconds, override.Floor
	}
	return c.HalfLifeSeconds, c.Floor
}

// EdgeDecayFactor returns the decay multiplier in [floor, 1.0]
// for an edge with `ageSeconds` time since last refresh under
// the given domain's config.
//
// Returns 1.0 when decay is disabled (half-life <= 0 or age
// <= 0). Returns exactly `floor` when the decayed value would
// be below the floor.
func EdgeDecayFactor(ageSeconds, halfLifeSeconds int64, floor float64) float64 {
	if halfLifeSeconds <= 0 || ageSeconds <= 0 {
		return 1.0
	}
	// exp(-t/halfLife * ln(2)) == 0.5^(t/halfLife)
	ratio := float64(ageSeconds) / float64(halfLifeSeconds)
	factor := math.Exp(-ratio * math.Ln2)
	if factor < floor {
		return floor
	}
	if factor > 1.0 {
		return 1.0
	}
	return factor
}

// isValidFloor normalizes out-of-range floors to 0.0.
func isValidFloor(f float64) float64 {
	if f < 0.0 || f != f { // negative or NaN
		return 0.0
	}
	if f > 1.0 {
		return 1.0
	}
	return f
}

// GetDirectTrusteesDecayed returns the trustees map for the
// given quid, with every edge's trust level multiplied by its
// decay factor for the given observation time.
//
// Edges whose `ValidUntil` has expired (QDP-0022) are
// filtered out as in `GetDirectTrustees`.
//
// nowSec is the reference time in Unix seconds. Callers in
// production should pass `time.Now().Unix()`; tests can pass
// a fixed value.
func (node *QuidnugNode) GetDirectTrusteesDecayed(
	quidID string, nowSec int64, cfg DecayConfig,
) map[string]float64 {
	node.TrustRegistryMutex.RLock()
	defer node.TrustRegistryMutex.RUnlock()

	result := make(map[string]float64)
	trustMap, exists := node.TrustRegistry[quidID]
	if !exists {
		return result
	}
	for trustee, level := range trustMap {
		if !node.isTrustEdgeValidLocked(quidID, trustee) {
			continue
		}
		// Look up edge timestamp.
		var edgeTS int64
		if tsMap, ok := node.TrustEdgeTimestampRegistry[quidID]; ok {
			edgeTS = tsMap[trustee]
		}
		if edgeTS == 0 {
			// No timestamp recorded — pass through undecayed.
			result[trustee] = level
			continue
		}
		age := nowSec - edgeTS
		// We don't know the domain of this specific edge at
		// this layer (the edge lives in a single domain but
		// the registry is per-(truster,trustee) only). Phase 1
		// applies the top-level config; per-domain overrides
		// become available when we walk edges by domain
		// (Phase 2+ work).
		hl, floor := cfg.effectiveForDomain("")
		floor = isValidFloor(floor)
		factor := EdgeDecayFactor(age, hl, floor)
		result[trustee] = level * factor
	}
	return result
}

// ComputeRelationalTrustWithDecay performs the same BFS
// traversal as `ComputeRelationalTrust` but applies edge
// decay at each step. The result is (decayed best trust,
// path, error).
//
// Callers that want the classic un-decayed computation
// should keep calling `ComputeRelationalTrust`. This
// variant is opt-in.
func (node *QuidnugNode) ComputeRelationalTrustWithDecay(
	observer, target string,
	maxDepth int,
	cfg DecayConfig,
) (float64, []string, error) {
	return node.computeRelationalTrustDecayAt(observer, target, maxDepth, cfg, time.Now().Unix())
}

// computeRelationalTrustDecayAt is the testable entrypoint
// that accepts an explicit nowSec so unit tests can pin
// time.
func (node *QuidnugNode) computeRelationalTrustDecayAt(
	observer, target string,
	maxDepth int,
	cfg DecayConfig,
	nowSec int64,
) (float64, []string, error) {
	if maxDepth <= 0 {
		maxDepth = DefaultTrustMaxDepth
	}
	if observer == target {
		return 1.0, []string{observer}, nil
	}

	type searchState struct {
		quid  string
		path  []string
		trust float64
	}
	queue := []searchState{{
		quid:  observer,
		path:  []string{observer},
		trust: 1.0,
	}}
	visited := map[string]bool{observer: true}

	bestTrust := 0.0
	var bestPath []string

	for len(queue) > 0 {
		if len(queue) > MaxTrustQueueSize {
			return bestTrust, bestPath, ErrTrustGraphTooLarge
		}
		if len(visited) > MaxTrustVisitedSize {
			return bestTrust, bestPath, ErrTrustGraphTooLarge
		}
		current := queue[0]
		queue = queue[1:]

		trustees := node.GetDirectTrusteesDecayed(current.quid, nowSec, cfg)

		for trustee, edgeTrust := range trustees {
			inPath := false
			for _, p := range current.path {
				if p == trustee {
					inPath = true
					break
				}
			}
			if inPath {
				continue
			}
			pathTrust := current.trust * edgeTrust

			newPath := make([]string, len(current.path)+1)
			copy(newPath, current.path)
			newPath[len(current.path)] = trustee

			if trustee == target {
				if pathTrust > bestTrust {
					bestTrust = pathTrust
					bestPath = newPath
				}
				continue
			}
			if len(current.path) < maxDepth && !visited[trustee] {
				visited[trustee] = true
				queue = append(queue, searchState{
					quid:  trustee,
					path:  newPath,
					trust: pathTrust,
				})
			}
		}
	}
	return bestTrust, bestPath, nil
}
