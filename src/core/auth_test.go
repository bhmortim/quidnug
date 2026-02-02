package main

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"testing"
	"time"
)

func TestSignRequest(t *testing.T) {
	secret := "test-secret-key"
	timestamp := int64(1700000000)

	// Test POST with body
	sig1 := SignRequest("POST", "/api/transactions/trust", []byte(`{"test":"data"}`), secret, timestamp)
	if sig1 == "" {
		t.Error("SignRequest returned empty signature")
	}

	// Same inputs should produce same signature
	sig2 := SignRequest("POST", "/api/transactions/trust", []byte(`{"test":"data"}`), secret, timestamp)
	if sig1 != sig2 {
		t.Error("Same inputs produced different signatures")
	}

	// Different body should produce different signature
	sig3 := SignRequest("POST", "/api/transactions/trust", []byte(`{"test":"other"}`), secret, timestamp)
	if sig1 == sig3 {
		t.Error("Different body produced same signature")
	}

	// Different method should produce different signature
	sig4 := SignRequest("GET", "/api/transactions/trust", []byte(`{"test":"data"}`), secret, timestamp)
	if sig1 == sig4 {
		t.Error("Different method produced same signature")
	}

	// Different path should produce different signature
	sig5 := SignRequest("POST", "/api/transactions/identity", []byte(`{"test":"data"}`), secret, timestamp)
	if sig1 == sig5 {
		t.Error("Different path produced same signature")
	}

	// Different timestamp should produce different signature
	sig6 := SignRequest("POST", "/api/transactions/trust", []byte(`{"test":"data"}`), secret, timestamp+1)
	if sig1 == sig6 {
		t.Error("Different timestamp produced same signature")
	}

	// Different secret should produce different signature
	sig7 := SignRequest("POST", "/api/transactions/trust", []byte(`{"test":"data"}`), "other-secret", timestamp)
	if sig1 == sig7 {
		t.Error("Different secret produced same signature")
	}

	// GET with empty body
	sig8 := SignRequest("GET", "/api/domains/test/query", nil, secret, timestamp)
	if sig8 == "" {
		t.Error("SignRequest with nil body returned empty signature")
	}
}

func TestVerifyRequest(t *testing.T) {
	secret := "test-secret-key"
	now := time.Now().Unix()
	body := []byte(`{"test":"data"}`)
	method := "POST"
	path := "/api/transactions/trust"

	// Valid signature should verify
	signature := SignRequest(method, path, body, secret, now)
	if !VerifyRequest(method, path, body, secret, now, signature) {
		t.Error("Valid signature failed verification")
	}

	// Wrong signature should fail
	if VerifyRequest(method, path, body, secret, now, "invalid-signature") {
		t.Error("Invalid signature passed verification")
	}

	// Wrong secret should fail
	wrongSig := SignRequest(method, path, body, "wrong-secret", now)
	if VerifyRequest(method, path, body, secret, now, wrongSig) {
		t.Error("Signature with wrong secret passed verification")
	}

	// Modified body should fail
	if VerifyRequest(method, path, []byte(`{"modified":"body"}`), secret, now, signature) {
		t.Error("Modified body passed verification")
	}

	// Modified path should fail
	if VerifyRequest(method, "/different/path", body, secret, now, signature) {
		t.Error("Modified path passed verification")
	}
}

func TestVerifyRequestTimestamp(t *testing.T) {
	secret := "test-secret-key"
	body := []byte(`{"test":"data"}`)
	method := "POST"
	path := "/api/transactions/trust"

	// Current timestamp should work
	now := time.Now().Unix()
	sig := SignRequest(method, path, body, secret, now)
	if !VerifyRequest(method, path, body, secret, now, sig) {
		t.Error("Current timestamp failed verification")
	}

	// Timestamp 4 minutes ago should work (within 5 minute tolerance)
	fourMinAgo := now - 240
	sig2 := SignRequest(method, path, body, secret, fourMinAgo)
	if !VerifyRequest(method, path, body, secret, fourMinAgo, sig2) {
		t.Error("Timestamp 4 minutes ago failed verification")
	}

	// Timestamp 6 minutes ago should fail (outside 5 minute tolerance)
	sixMinAgo := now - 360
	sig3 := SignRequest(method, path, body, secret, sixMinAgo)
	if VerifyRequest(method, path, body, secret, sixMinAgo, sig3) {
		t.Error("Stale timestamp (6 min ago) passed verification")
	}

	// Timestamp 6 minutes in the future should fail
	sixMinFuture := now + 360
	sig4 := SignRequest(method, path, body, secret, sixMinFuture)
	if VerifyRequest(method, path, body, secret, sixMinFuture, sig4) {
		t.Error("Future timestamp (6 min ahead) passed verification")
	}
}

func TestNodeAuthMiddleware_AuthDisabled(t *testing.T) {
	// Reset and configure auth as disabled
	ResetNodeAuthConfigForTesting()
	os.Unsetenv("NODE_AUTH_SECRET")
	os.Unsetenv("REQUIRE_NODE_AUTH")

	handler := NodeAuthMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	// Request without auth headers should pass when auth is disabled
	req := httptest.NewRequest("POST", "/api/transactions/trust", bytes.NewReader([]byte(`{"test":"data"}`)))
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("Expected 200 when auth disabled, got %d", rr.Code)
	}
}

func TestNodeAuthMiddleware_AuthRequired_ValidSignature(t *testing.T) {
	// Reset and configure auth as required
	ResetNodeAuthConfigForTesting()
	os.Setenv("NODE_AUTH_SECRET", "test-secret")
	os.Setenv("REQUIRE_NODE_AUTH", "true")
	defer func() {
		os.Unsetenv("NODE_AUTH_SECRET")
		os.Unsetenv("REQUIRE_NODE_AUTH")
		ResetNodeAuthConfigForTesting()
	}()

	handler := NodeAuthMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify body is still readable
		body, err := io.ReadAll(r.Body)
		if err != nil || string(body) != `{"test":"data"}` {
			t.Error("Body was not properly restored after auth verification")
		}
		w.WriteHeader(http.StatusOK)
	}))

	body := []byte(`{"test":"data"}`)
	timestamp := time.Now().Unix()
	signature := SignRequest("POST", "/api/transactions/trust", body, "test-secret", timestamp)

	req := httptest.NewRequest("POST", "/api/transactions/trust", bytes.NewReader(body))
	req.Header.Set(NodeSignatureHeader, signature)
	req.Header.Set(NodeTimestampHeader, strconv.FormatInt(timestamp, 10))
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("Expected 200 with valid signature, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestNodeAuthMiddleware_AuthRequired_MissingHeaders(t *testing.T) {
	// Reset and configure auth as required
	ResetNodeAuthConfigForTesting()
	os.Setenv("NODE_AUTH_SECRET", "test-secret")
	os.Setenv("REQUIRE_NODE_AUTH", "true")
	defer func() {
		os.Unsetenv("NODE_AUTH_SECRET")
		os.Unsetenv("REQUIRE_NODE_AUTH")
		ResetNodeAuthConfigForTesting()
	}()

	handler := NodeAuthMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	// Request without auth headers should be rejected
	req := httptest.NewRequest("POST", "/api/transactions/trust", bytes.NewReader([]byte(`{"test":"data"}`)))
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("Expected 401 without auth headers, got %d", rr.Code)
	}
}

func TestNodeAuthMiddleware_AuthRequired_InvalidSignature(t *testing.T) {
	// Reset and configure auth as required
	ResetNodeAuthConfigForTesting()
	os.Setenv("NODE_AUTH_SECRET", "test-secret")
	os.Setenv("REQUIRE_NODE_AUTH", "true")
	defer func() {
		os.Unsetenv("NODE_AUTH_SECRET")
		os.Unsetenv("REQUIRE_NODE_AUTH")
		ResetNodeAuthConfigForTesting()
	}()

	handler := NodeAuthMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("POST", "/api/transactions/trust", bytes.NewReader([]byte(`{"test":"data"}`)))
	req.Header.Set(NodeSignatureHeader, "invalid-signature")
	req.Header.Set(NodeTimestampHeader, strconv.FormatInt(time.Now().Unix(), 10))
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("Expected 401 with invalid signature, got %d", rr.Code)
	}
}

func TestNodeAuthMiddleware_AuthRequired_StaleTimestamp(t *testing.T) {
	// Reset and configure auth as required
	ResetNodeAuthConfigForTesting()
	os.Setenv("NODE_AUTH_SECRET", "test-secret")
	os.Setenv("REQUIRE_NODE_AUTH", "true")
	defer func() {
		os.Unsetenv("NODE_AUTH_SECRET")
		os.Unsetenv("REQUIRE_NODE_AUTH")
		ResetNodeAuthConfigForTesting()
	}()

	handler := NodeAuthMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	body := []byte(`{"test":"data"}`)
	// Use timestamp 10 minutes ago (outside 5 minute tolerance)
	staleTimestamp := time.Now().Unix() - 600
	signature := SignRequest("POST", "/api/transactions/trust", body, "test-secret", staleTimestamp)

	req := httptest.NewRequest("POST", "/api/transactions/trust", bytes.NewReader(body))
	req.Header.Set(NodeSignatureHeader, signature)
	req.Header.Set(NodeTimestampHeader, strconv.FormatInt(staleTimestamp, 10))
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("Expected 401 with stale timestamp, got %d", rr.Code)
	}
}

func TestNodeAuthMiddleware_NonTransactionEndpoints(t *testing.T) {
	// Reset and configure auth as required
	ResetNodeAuthConfigForTesting()
	os.Setenv("NODE_AUTH_SECRET", "test-secret")
	os.Setenv("REQUIRE_NODE_AUTH", "true")
	defer func() {
		os.Unsetenv("NODE_AUTH_SECRET")
		os.Unsetenv("REQUIRE_NODE_AUTH")
		ResetNodeAuthConfigForTesting()
	}()

	handler := NodeAuthMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	// GET request to non-transaction endpoint should pass without auth
	req := httptest.NewRequest("GET", "/api/health", nil)
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("Expected 200 for non-transaction endpoint, got %d", rr.Code)
	}

	// POST to non-transaction endpoint should also pass
	req2 := httptest.NewRequest("POST", "/api/quids", bytes.NewReader([]byte(`{}`)))
	rr2 := httptest.NewRecorder()

	handler.ServeHTTP(rr2, req2)

	if rr2.Code != http.StatusOK {
		t.Errorf("Expected 200 for non-transaction POST endpoint, got %d", rr2.Code)
	}
}

func TestIsNodeToNodeEndpoint(t *testing.T) {
	tests := []struct {
		path     string
		expected bool
	}{
		{"/api/transactions/trust", true},
		{"/api/transactions/identity", true},
		{"/api/transactions/title", true},
		{"/api/v1/transactions/trust", true},
		{"/api/health", false},
		{"/api/blocks", false},
		{"/api/quids", false},
		{"/api/nodes", false},
		{"/api/transactions", false}, // GET all transactions endpoint
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			result := isNodeToNodeEndpoint(tt.path)
			if result != tt.expected {
				t.Errorf("isNodeToNodeEndpoint(%q) = %v, want %v", tt.path, result, tt.expected)
			}
		})
	}
}
