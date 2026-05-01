// CORS middleware regression coverage. QDP-0025 §10.2.
package core

import (
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
)

// TestCORS_DenyByDefault: with no EXPLORER_CORS_ORIGINS set,
// cross-origin requests get no Access-Control-Allow-Origin
// header (browser rejects them as expected) and OPTIONS still
// short-circuits with 204.
func TestCORS_DenyByDefault(t *testing.T) {
	t.Setenv("EXPLORER_CORS_ORIGINS", "")
	ResetCORSConfigForTesting()

	called := false
	inner := http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {
		called = true
	})
	mw := CORSMiddleware(inner)

	req := httptest.NewRequest("GET", "/api/v1/info", nil)
	req.Header.Set("Origin", "https://explorer.example.com")
	rec := httptest.NewRecorder()
	mw.ServeHTTP(rec, req)

	if !called {
		t.Fatal("inner handler should still be called for non-OPTIONS requests")
	}
	if v := rec.Header().Get("Access-Control-Allow-Origin"); v != "" {
		t.Fatalf("ACAO should be empty by default; got %q", v)
	}
}

// TestCORS_AllowsListedOrigin: the explicit origin in the env
// var gets echoed back; an unrelated origin does not.
func TestCORS_AllowsListedOrigin(t *testing.T) {
	t.Setenv("EXPLORER_CORS_ORIGINS",
		"http://localhost:5173,https://explorer.quidnug.com")
	ResetCORSConfigForTesting()

	mw := CORSMiddleware(http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {}))

	cases := []struct {
		origin string
		want   string
	}{
		{"http://localhost:5173", "http://localhost:5173"},
		{"https://explorer.quidnug.com", "https://explorer.quidnug.com"},
		{"https://attacker.example.com", ""},
	}
	for _, tc := range cases {
		t.Run(tc.origin, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/api/v1/info", nil)
			req.Header.Set("Origin", tc.origin)
			rec := httptest.NewRecorder()
			mw.ServeHTTP(rec, req)
			if got := rec.Header().Get("Access-Control-Allow-Origin"); got != tc.want {
				t.Fatalf("origin %q: ACAO want %q got %q", tc.origin, tc.want, got)
			}
		})
	}
}

// TestCORS_Wildcard: "*" allows any origin (dev convenience).
func TestCORS_Wildcard(t *testing.T) {
	t.Setenv("EXPLORER_CORS_ORIGINS", "*")
	ResetCORSConfigForTesting()

	mw := CORSMiddleware(http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {}))

	req := httptest.NewRequest("GET", "/api/v1/info", nil)
	req.Header.Set("Origin", "https://anything.example")
	rec := httptest.NewRecorder()
	mw.ServeHTTP(rec, req)

	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "https://anything.example" {
		t.Fatalf("wildcard should echo origin; got %q", got)
	}
}

// TestCORS_PreflightShortCircuits: OPTIONS gets 204 and does
// not reach the inner handler. Critical for POST endpoints
// where NodeAuth would reject a body-less request.
func TestCORS_PreflightShortCircuits(t *testing.T) {
	t.Setenv("EXPLORER_CORS_ORIGINS", "*")
	ResetCORSConfigForTesting()

	called := false
	inner := http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {
		called = true
	})
	mw := CORSMiddleware(inner)

	req := httptest.NewRequest("OPTIONS", "/api/transactions/trust", nil)
	req.Header.Set("Origin", "http://localhost:5173")
	req.Header.Set("Access-Control-Request-Method", "POST")
	req.Header.Set("Access-Control-Request-Headers", "Content-Type, Authorization")
	rec := httptest.NewRecorder()
	mw.ServeHTTP(rec, req)

	if called {
		t.Fatal("preflight should short-circuit, not reach inner handler")
	}
	if rec.Code != http.StatusNoContent {
		t.Fatalf("preflight status: want 204, got %d", rec.Code)
	}
	if got := rec.Header().Get("Access-Control-Allow-Methods"); got == "" {
		t.Fatalf("preflight should advertise allowed methods, got empty")
	}
	if got := rec.Header().Get("Access-Control-Allow-Headers"); got == "" {
		t.Fatalf("preflight should advertise allowed headers, got empty")
	}
}

// TestCORS_VaryOrigin: Vary: Origin must always be set so
// caches don't merge responses across origins.
func TestCORS_VaryOrigin(t *testing.T) {
	t.Setenv("EXPLORER_CORS_ORIGINS", "*")
	ResetCORSConfigForTesting()

	mw := CORSMiddleware(http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {}))
	req := httptest.NewRequest("GET", "/api/v1/info", nil)
	req.Header.Set("Origin", "http://localhost:5173")
	rec := httptest.NewRecorder()
	mw.ServeHTTP(rec, req)

	vary := rec.Header().Values("Vary")
	found := false
	for _, v := range vary {
		if v == "Origin" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("Vary should include Origin; got %v", vary)
	}
}

// TestCORS_NoOriginHeader: a same-origin or curl-style request
// with no Origin header is unaffected.
func TestCORS_NoOriginHeader(t *testing.T) {
	t.Setenv("EXPLORER_CORS_ORIGINS", "*")
	ResetCORSConfigForTesting()

	called := false
	mw := CORSMiddleware(http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {
		called = true
	}))
	req := httptest.NewRequest("GET", "/api/v1/info", nil)
	rec := httptest.NewRecorder()
	mw.ServeHTTP(rec, req)

	if !called {
		t.Fatal("origin-less request should pass through to the handler")
	}
	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "" {
		t.Fatalf("origin-less request should not get an ACAO header, got %q", got)
	}
}

// Ensure os import is used (the t.Setenv API is Go 1.17+ but
// we lean on os indirectly in case future tests need it).
var _ = os.Getenv
