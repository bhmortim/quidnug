package main

import (
	"testing"
)

func TestComputeRelationalTrustEnhanced_SameEntity(t *testing.T) {
	node := newTestNode()

	result, err := node.ComputeRelationalTrustEnhanced("1111111111111111", "1111111111111111", 5, true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.TrustLevel != 1.0 {
		t.Errorf("expected TrustLevel 1.0, got %f", result.TrustLevel)
	}
	if result.Confidence != "high" {
		t.Errorf("expected Confidence 'high', got %s", result.Confidence)
	}
	if result.UnverifiedHops != 0 {
		t.Errorf("expected UnverifiedHops 0, got %d", result.UnverifiedHops)
	}
}

func TestComputeRelationalTrustEnhanced_VerifiedOnlyPath(t *testing.T) {
	node := newTestNode()

	// Set up verified edges: A -> B -> C
	node.AddVerifiedTrustEdge(TrustEdge{
		Truster:    "aaaaaaaaaaaaaaaa",
		Trustee:    "bbbbbbbbbbbbbbbb",
		TrustLevel: 0.8,
	})
	node.AddVerifiedTrustEdge(TrustEdge{
		Truster:    "bbbbbbbbbbbbbbbb",
		Trustee:    "cccccccccccccccc",
		TrustLevel: 0.9,
	})

	result, err := node.ComputeRelationalTrustEnhanced("aaaaaaaaaaaaaaaa", "cccccccccccccccc", 5, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expectedTrust := 0.8 * 0.9 // 0.72
	if !floatEquals(result.TrustLevel, expectedTrust, 0.0001) {
		t.Errorf("expected TrustLevel %f, got %f", expectedTrust, result.TrustLevel)
	}
	if result.Confidence != "high" {
		t.Errorf("expected Confidence 'high', got %s", result.Confidence)
	}
	if result.UnverifiedHops != 0 {
		t.Errorf("expected UnverifiedHops 0, got %d", result.UnverifiedHops)
	}
	if len(result.VerificationGaps) != 0 {
		t.Errorf("expected no VerificationGaps, got %d", len(result.VerificationGaps))
	}
}

func TestComputeRelationalTrustEnhanced_MixedPathWithGaps(t *testing.T) {
	node := newTestNode()

	// Set up verified edge: A -> B (verified)
	node.AddVerifiedTrustEdge(TrustEdge{
		Truster:    "aaaaaaaaaaaaaaaa",
		Trustee:    "bbbbbbbbbbbbbbbb",
		TrustLevel: 0.8,
	})

	// Set up unverified edge: B -> C (from validator V)
	node.AddUnverifiedTrustEdge(TrustEdge{
		Truster:       "bbbbbbbbbbbbbbbb",
		Trustee:       "cccccccccccccccc",
		TrustLevel:    0.9,
		ValidatorQuid: "dddddddddddddddd",
	})

	// Set up A's trust in validator V
	node.AddVerifiedTrustEdge(TrustEdge{
		Truster:    "aaaaaaaaaaaaaaaa",
		Trustee:    "dddddddddddddddd",
		TrustLevel: 0.5,
	})

	result, err := node.ComputeRelationalTrustEnhanced("aaaaaaaaaaaaaaaa", "cccccccccccccccc", 5, true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Expected: 0.8 (A->B) * 0.9 (B->C) * 0.5 (validator discount) = 0.36
	expectedTrust := 0.8 * 0.9 * 0.5
	if !floatEquals(result.TrustLevel, expectedTrust, 0.0001) {
		t.Errorf("expected TrustLevel %f, got %f", expectedTrust, result.TrustLevel)
	}
	if result.Confidence != "medium" {
		t.Errorf("expected Confidence 'medium', got %s", result.Confidence)
	}
	if result.UnverifiedHops != 1 {
		t.Errorf("expected UnverifiedHops 1, got %d", result.UnverifiedHops)
	}
	if len(result.VerificationGaps) != 1 {
		t.Errorf("expected 1 VerificationGap, got %d", len(result.VerificationGaps))
	} else {
		gap := result.VerificationGaps[0]
		if gap.From != "bbbbbbbbbbbbbbbb" || gap.To != "cccccccccccccccc" {
			t.Errorf("expected gap From='bbbbbbbbbbbbbbbb' To='cccccccccccccccc', got From='%s' To='%s'", gap.From, gap.To)
		}
		if gap.ValidatorQuid != "dddddddddddddddd" {
			t.Errorf("expected ValidatorQuid 'dddddddddddddddd', got '%s'", gap.ValidatorQuid)
		}
		if gap.ValidatorTrust != 0.5 {
			t.Errorf("expected ValidatorTrust 0.5, got %f", gap.ValidatorTrust)
		}
	}
}

func TestComputeRelationalTrustEnhanced_UnverifiedDiscounting(t *testing.T) {
	node := newTestNode()

	// Set up unverified edge: A -> B (from validator V with no trust)
	node.AddUnverifiedTrustEdge(TrustEdge{
		Truster:       "aaaaaaaaaaaaaaaa",
		Trustee:       "bbbbbbbbbbbbbbbb",
		TrustLevel:    1.0,
		ValidatorQuid: "dddddddddddddddd",
	})

	// No trust path from A to V, so validator trust is 0
	result, err := node.ComputeRelationalTrustEnhanced("aaaaaaaaaaaaaaaa", "bbbbbbbbbbbbbbbb", 5, true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Expected: 1.0 * 0 (no trust in validator) = 0
	if result.TrustLevel != 0.0 {
		t.Errorf("expected TrustLevel 0.0, got %f", result.TrustLevel)
	}
}

func TestComputeRelationalTrustEnhanced_IncludeUnverifiedFalse(t *testing.T) {
	node := newTestNode()

	// Set up only unverified edges
	node.AddUnverifiedTrustEdge(TrustEdge{
		Truster:       "aaaaaaaaaaaaaaaa",
		Trustee:       "bbbbbbbbbbbbbbbb",
		TrustLevel:    0.8,
		ValidatorQuid: "dddddddddddddddd",
	})

	// With includeUnverified=false, should find no path
	result, err := node.ComputeRelationalTrustEnhanced("aaaaaaaaaaaaaaaa", "bbbbbbbbbbbbbbbb", 5, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.TrustLevel != 0.0 {
		t.Errorf("expected TrustLevel 0.0 (no path), got %f", result.TrustLevel)
	}
	if result.Confidence != "high" {
		t.Errorf("expected Confidence 'high' (no unverified hops), got %s", result.Confidence)
	}
}

func TestComputeRelationalTrustEnhanced_LowConfidenceMultipleGaps(t *testing.T) {
	node := newTestNode()

	// Set up two unverified edges: A -> B -> C (both from validator V)
	node.AddUnverifiedTrustEdge(TrustEdge{
		Truster:       "aaaaaaaaaaaaaaaa",
		Trustee:       "bbbbbbbbbbbbbbbb",
		TrustLevel:    0.8,
		ValidatorQuid: "dddddddddddddddd",
	})
	node.AddUnverifiedTrustEdge(TrustEdge{
		Truster:       "bbbbbbbbbbbbbbbb",
		Trustee:       "cccccccccccccccc",
		TrustLevel:    0.9,
		ValidatorQuid: "dddddddddddddddd",
	})

	// Set up A's trust in validator V
	node.AddVerifiedTrustEdge(TrustEdge{
		Truster:    "aaaaaaaaaaaaaaaa",
		Trustee:    "dddddddddddddddd",
		TrustLevel: 1.0,
	})

	result, err := node.ComputeRelationalTrustEnhanced("aaaaaaaaaaaaaaaa", "cccccccccccccccc", 5, true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Confidence != "low" {
		t.Errorf("expected Confidence 'low' (2+ unverified hops), got %s", result.Confidence)
	}
	if result.UnverifiedHops != 2 {
		t.Errorf("expected UnverifiedHops 2, got %d", result.UnverifiedHops)
	}
	if len(result.VerificationGaps) != 2 {
		t.Errorf("expected 2 VerificationGaps, got %d", len(result.VerificationGaps))
	}
}

func TestComputeRelationalTrustEnhanced_DirectVerifiedTrust(t *testing.T) {
	node := newTestNode()

	// Set up direct verified edge: A -> B
	node.AddVerifiedTrustEdge(TrustEdge{
		Truster:    "aaaaaaaaaaaaaaaa",
		Trustee:    "bbbbbbbbbbbbbbbb",
		TrustLevel: 0.75,
	})

	result, err := node.ComputeRelationalTrustEnhanced("aaaaaaaaaaaaaaaa", "bbbbbbbbbbbbbbbb", 5, true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.TrustLevel != 0.75 {
		t.Errorf("expected TrustLevel 0.75, got %f", result.TrustLevel)
	}
	if result.PathDepth != 1 {
		t.Errorf("expected PathDepth 1, got %d", result.PathDepth)
	}
	if len(result.TrustPath) != 2 {
		t.Errorf("expected TrustPath length 2, got %d", len(result.TrustPath))
	}
	if result.TrustPath[0] != "aaaaaaaaaaaaaaaa" || result.TrustPath[1] != "bbbbbbbbbbbbbbbb" {
		t.Errorf("expected TrustPath ['aaaaaaaaaaaaaaaa', 'bbbbbbbbbbbbbbbb'], got %v", result.TrustPath)
	}
}
