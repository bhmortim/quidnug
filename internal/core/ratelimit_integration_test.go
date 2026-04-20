// QDP-0016 Phase 1 integration test — the multi-layer write
// limiter is wired into every mempool-admission path. This
// file exercises the Trust, Moderation, and Privacy admission
// surfaces to confirm that the limiter catches floods at the
// layer QDP-0016 §3 describes.
package core

import (
	"testing"
	"time"

	"github.com/quidnug/quidnug/internal/ratelimit"
)

func TestWriteLimiter_DefaultsInstalled(t *testing.T) {
	node := newTestNode()
	if node.WriteLimiter == nil {
		t.Fatal("expected WriteLimiter to be initialized by NewQuidnugNode")
	}
	cfg := node.WriteLimiter.Config()
	if !cfg.Quid.Enabled {
		t.Error("default config should enable quid layer")
	}
}

func TestAddTrustTransaction_RateLimitedPerQuid(t *testing.T) {
	node := newTestNode()
	// Override the write limiter with a tight per-quid cap so
	// the test doesn't depend on the default burst.
	node.WriteLimiter = ratelimit.NewMultiLayerLimiter(ratelimit.MultiLayerConfig{
		Quid:   ratelimit.LayerConfig{Enabled: true, RequestsPerMinute: 60, Burst: 2},
		Domain: ratelimit.LayerConfig{Enabled: true, RequestsPerMinute: 60, Burst: 1000},
	})

	base := TrustTransaction{
		BaseTransaction: BaseTransaction{
			Type:        TxTypeTrust,
			TrustDomain: "test.domain.com",
			Timestamp:   time.Now().Unix(),
			Signature:   "aabb", // make sure AddTrust doesn't auto-fill / mutate
			PublicKey:   node.GetPublicKeyHex(),
		},
		Truster:    "alice",
		Trustee:    "bob",
		TrustLevel: 0.5,
	}

	// Two through the rate limiter should proceed to validation.
	// Validation will fail (unsigned / unknown quid), but only
	// after the limiter admits — so we expect the "invalid" error
	// string for the first two attempts.
	for i := 0; i < 2; i++ {
		base.Nonce = int64(i + 1)
		_, err := node.AddTrustTransaction(base)
		if err == nil {
			t.Fatalf("iteration %d should fail validation (not a real signed tx)", i)
		}
		if got := err.Error(); got == "rate limit exceeded at quid layer" {
			t.Fatalf("iteration %d: limiter fired too early: %s", i, got)
		}
	}
	// Third attempt should hit the per-quid limit.
	base.Nonce = 99
	_, err := node.AddTrustTransaction(base)
	if err == nil {
		t.Fatal("third attempt should hit rate limit")
	}
	if err.Error() != "rate limit exceeded at quid layer" {
		t.Errorf("expected quid-layer denial, got %q", err.Error())
	}
}

func TestAddModerationAction_RateLimitedPerQuid(t *testing.T) {
	node, actor := newModerationTestFixture(t)
	node.WriteLimiter = ratelimit.NewMultiLayerLimiter(ratelimit.MultiLayerConfig{
		Quid:   ratelimit.LayerConfig{Enabled: true, RequestsPerMinute: 60, Burst: 1},
		Domain: ratelimit.LayerConfig{Enabled: true, RequestsPerMinute: 60, Burst: 1000},
	})

	first := actor.signModeration(baselineAction(actor, 1))
	if _, err := node.AddModerationActionTransaction(first); err != nil {
		t.Fatalf("first moderation action should succeed, got %v", err)
	}

	second := actor.signModeration(baselineAction(actor, 2))
	_, err := node.AddModerationActionTransaction(second)
	if err == nil || err.Error() != "rate limit exceeded at quid layer" {
		t.Errorf("second moderation action should hit quid limit, got %v", err)
	}
}

func TestAddDSR_RateLimitedPerQuid(t *testing.T) {
	node, actor := newPrivacyTestFixture(t)
	node.WriteLimiter = ratelimit.NewMultiLayerLimiter(ratelimit.MultiLayerConfig{
		Quid:   ratelimit.LayerConfig{Enabled: true, RequestsPerMinute: 60, Burst: 1},
		Domain: ratelimit.LayerConfig{Enabled: true, RequestsPerMinute: 60, Burst: 1000},
	})

	first := DataSubjectRequestTransaction{
		BaseTransaction: BaseTransaction{
			Type:        TxTypeDataSubjectRequest,
			TrustDomain: "test.domain.com",
			Timestamp:   time.Now().Unix(),
		},
		RequestType: DSRTypeAccess,
		Nonce:       1,
	}
	first = actor.signDSR(first)
	if _, err := node.AddDataSubjectRequestTransaction(first); err != nil {
		t.Fatalf("first DSR should succeed, got %v", err)
	}

	second := DataSubjectRequestTransaction{
		BaseTransaction: BaseTransaction{
			Type:        TxTypeDataSubjectRequest,
			TrustDomain: "test.domain.com",
			Timestamp:   time.Now().Unix(),
		},
		RequestType: DSRTypeErasure,
		Nonce:       2,
	}
	second = actor.signDSR(second)
	_, err := node.AddDataSubjectRequestTransaction(second)
	if err == nil || err.Error() != "rate limit exceeded at quid layer" {
		t.Errorf("second DSR should hit quid limit, got %v", err)
	}
}

func TestWriteLimiter_NilIsSafeInAdmission(t *testing.T) {
	node := newTestNode()
	node.WriteLimiter = nil // simulate operator-disabled rate limit

	// With no limiter the admission path must not panic or
	// reject; validation errors still surface normally.
	base := TrustTransaction{
		BaseTransaction: BaseTransaction{
			Type:        TxTypeTrust,
			TrustDomain: "test.domain.com",
			Timestamp:   time.Now().Unix(),
		},
		Truster:    "alice",
		Trustee:    "bob",
		TrustLevel: 0.5,
		Nonce:      1,
	}
	_, err := node.AddTrustTransaction(base)
	if err == nil {
		t.Fatal("expected validation failure, got nil")
	}
	// Key assertion: the error is not the rate-limit one.
	if err.Error() == "rate limit exceeded at quid layer" {
		t.Errorf("nil limiter should not produce rate-limit rejections, got %q", err.Error())
	}
}
