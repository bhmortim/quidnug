package client

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// --- Envelope parsing ---------------------------------------------------

func TestSuccessEnvelopeUnwraps(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"success": true,
			"data":    map[string]any{"status": "ok", "quidId": "abc"},
		})
	}))
	defer srv.Close()

	c, err := New(srv.URL, WithMaxRetries(0))
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	data, err := c.Health(context.Background())
	if err != nil {
		t.Fatalf("Health: %v", err)
	}
	if data["status"] != "ok" {
		t.Fatalf("data: %v", data)
	}
}

func TestErrorEnvelope409Conflict(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(409)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"success": false,
			"error": map[string]any{
				"code":    "NONCE_REPLAY",
				"message": "replay detected",
			},
		})
	}))
	defer srv.Close()

	c, _ := New(srv.URL, WithMaxRetries(0))
	_, err := c.Health(context.Background())
	var ce *ConflictError
	if !errors.As(err, &ce) {
		t.Fatalf("want ConflictError, got %T: %v", err, err)
	}
	if ce.Code() != "NONCE_REPLAY" {
		t.Fatalf("code: got %q", ce.Code())
	}
}

func TestErrorEnvelope503Unavailable(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(503)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"success": false,
			"error":   map[string]any{"code": "BOOTSTRAPPING", "message": "not ready"},
		})
	}))
	defer srv.Close()

	c, _ := New(srv.URL, WithMaxRetries(0))
	_, err := c.Health(context.Background())
	var ue *UnavailableError
	if !errors.As(err, &ue) {
		t.Fatalf("want UnavailableError, got %T: %v", err, err)
	}
}

func TestNetworkFailureIsNodeError(t *testing.T) {
	// Point at a closed port.
	c, _ := New("http://127.0.0.1:1", WithMaxRetries(0), WithTimeout(500*time.Millisecond))
	_, err := c.Health(context.Background())
	var ne *NodeError
	if !errors.As(err, &ne) {
		t.Fatalf("want NodeError, got %T: %v", err, err)
	}
}

func TestErrSDKSentinelMatchesAll(t *testing.T) {
	errs := []error{
		&ValidationError{baseError{msg: "m"}},
		&ConflictError{baseError{msg: "m"}},
		&UnavailableError{baseError{msg: "m"}},
		&NodeError{baseError: baseError{msg: "m"}},
		&CryptoError{baseError{msg: "m"}},
	}
	for _, e := range errs {
		if !errors.Is(e, ErrSDK) {
			t.Fatalf("errors.Is(%T, ErrSDK) should be true", e)
		}
	}
}

// --- Retry behaviour ----------------------------------------------------

func TestRetryOn5xxThenSuccess(t *testing.T) {
	var n int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n++
		if n < 3 {
			w.WriteHeader(500)
			_ = json.NewEncoder(w).Encode(map[string]any{
				"success": false,
				"error":   map[string]any{"code": "INTERNAL"},
			})
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"success": true,
			"data":    map[string]any{"ok": true},
		})
	}))
	defer srv.Close()

	c, _ := New(srv.URL, WithMaxRetries(3), WithRetryBaseDelay(5*time.Millisecond))
	data, err := c.Health(context.Background())
	if err != nil {
		t.Fatalf("Health: %v", err)
	}
	if data["ok"] != true {
		t.Fatalf("data: %v", data)
	}
	if n != 3 {
		t.Fatalf("expected 3 attempts, got %d", n)
	}
}

func TestDoesNotRetryPOST(t *testing.T) {
	var n int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n++
		w.WriteHeader(500)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"success": false, "error": map[string]any{"code": "INTERNAL"},
		})
	}))
	defer srv.Close()

	c, _ := New(srv.URL, WithMaxRetries(3), WithRetryBaseDelay(5*time.Millisecond))
	q, _ := GenerateQuid()
	_, err := c.GrantTrust(context.Background(), q, TrustParams{Trustee: "bob", Level: 0.5})
	if err == nil {
		t.Fatal("expected error")
	}
	if n != 1 {
		t.Fatalf("POST should not retry; got %d attempts", n)
	}
}

// --- Local validation ---------------------------------------------------

func TestGrantTrustLevelValidation(t *testing.T) {
	c, _ := New("http://x")
	q, _ := GenerateQuid()
	_, err := c.GrantTrust(context.Background(), q, TrustParams{Trustee: "b", Level: 1.5})
	if err == nil || !strings.Contains(err.Error(), "level") {
		t.Fatalf("expected level validation error, got %v", err)
	}
}

func TestRegisterTitlePercentageValidation(t *testing.T) {
	c, _ := New("http://x")
	q, _ := GenerateQuid()
	_, err := c.RegisterTitle(context.Background(), q, TitleParams{
		AssetID: "a",
		Owners: []OwnershipStake{
			{OwnerID: "x", Percentage: 60.0},
			{OwnerID: "y", Percentage: 30.0},
		},
	})
	if err == nil || !strings.Contains(err.Error(), "100") {
		t.Fatalf("expected 100%% validation error, got %v", err)
	}
}

func TestEmitEventRequiresPayloadXorCID(t *testing.T) {
	c, _ := New("http://x")
	q, _ := GenerateQuid()
	_, err := c.EmitEvent(context.Background(), q, EventParams{
		SubjectID: "s", SubjectType: "QUID", EventType: "LOGIN",
	})
	if err == nil {
		t.Fatal("expected error when neither Payload nor PayloadCID is set")
	}
	_, err = c.EmitEvent(context.Background(), q, EventParams{
		SubjectID: "s", SubjectType: "QUID", EventType: "LOGIN",
		Payload: map[string]any{"x": 1}, PayloadCID: "Qm...",
	})
	if err == nil {
		t.Fatal("expected error when both Payload and PayloadCID are set")
	}
}

// --- Endpoint routing ---------------------------------------------------

func TestGrantTrustPostsToCorrectEndpoint(t *testing.T) {
	var hitPath string
	var body map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hitPath = r.URL.Path
		_ = json.NewDecoder(r.Body).Decode(&body)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"success": true, "data": map[string]any{"txId": "abc"},
		})
	}))
	defer srv.Close()

	c, _ := New(srv.URL, WithMaxRetries(0))
	q, _ := GenerateQuid()
	_, err := c.GrantTrust(context.Background(), q, TrustParams{
		Trustee: "bob", Level: 0.9, Domain: "contractors.home",
	})
	if err != nil {
		t.Fatalf("GrantTrust: %v", err)
	}
	if hitPath != "/api/transactions/trust" {
		t.Fatalf("hit path: %s", hitPath)
	}
	if body["type"] != "TRUST" {
		t.Fatalf("type: %v", body["type"])
	}
	if body["trustee"] != "bob" {
		t.Fatalf("trustee: %v", body["trustee"])
	}
	if _, ok := body["signature"]; !ok {
		t.Fatal("expected signature field in body")
	}
}

func TestGetIdentityReturnsNilOn404(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(404)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"success": false,
			"error":   map[string]any{"code": "NOT_FOUND", "message": "absent"},
		})
	}))
	defer srv.Close()

	c, _ := New(srv.URL, WithMaxRetries(0))
	rec, err := c.GetIdentity(context.Background(), "missing", "")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if rec != nil {
		t.Fatalf("want nil, got %+v", rec)
	}
}
