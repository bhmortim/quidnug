package main

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
)

func TestIPRateLimiter(t *testing.T) {
	t.Run("creates new limiter for unknown IP", func(t *testing.T) {
		limiter := NewIPRateLimiter(100)
		l1 := limiter.GetLimiter("192.168.1.1")
		if l1 == nil {
			t.Error("Expected non-nil limiter")
		}
	})

	t.Run("returns same limiter for same IP", func(t *testing.T) {
		limiter := NewIPRateLimiter(100)
		l1 := limiter.GetLimiter("192.168.1.1")
		l2 := limiter.GetLimiter("192.168.1.1")
		if l1 != l2 {
			t.Error("Expected same limiter for same IP")
		}
	})

	t.Run("returns different limiters for different IPs", func(t *testing.T) {
		limiter := NewIPRateLimiter(100)
		l1 := limiter.GetLimiter("192.168.1.1")
		l2 := limiter.GetLimiter("192.168.1.2")
		if l1 == l2 {
			t.Error("Expected different limiters for different IPs")
		}
	})
}

func TestRateLimitMiddleware(t *testing.T) {
	t.Run("allows requests under limit", func(t *testing.T) {
		limiter := NewIPRateLimiter(100)
		handler := RateLimitMiddleware(limiter)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		}))

		req := httptest.NewRequest("GET", "/test", nil)
		req.RemoteAddr = "192.168.1.1:12345"
		w := httptest.NewRecorder()

		handler.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("Expected status 200, got %d", w.Code)
		}
	})

	t.Run("returns 429 when rate limit exceeded", func(t *testing.T) {
		limiter := NewIPRateLimiter(10)
		handler := RateLimitMiddleware(limiter)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		}))

		var exceeded bool
		for i := 0; i < 20; i++ {
			req := httptest.NewRequest("GET", "/test", nil)
			req.RemoteAddr = "192.168.1.100:12345"
			w := httptest.NewRecorder()

			handler.ServeHTTP(w, req)

			if w.Code == http.StatusTooManyRequests {
				exceeded = true
				break
			}
		}

		if !exceeded {
			t.Error("Expected rate limit to be exceeded")
		}
	})

	t.Run("sets X-RateLimit-Remaining header", func(t *testing.T) {
		limiter := NewIPRateLimiter(100)
		handler := RateLimitMiddleware(limiter)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		}))

		req := httptest.NewRequest("GET", "/test", nil)
		req.RemoteAddr = "192.168.1.2:12345"
		w := httptest.NewRecorder()

		handler.ServeHTTP(w, req)

		remaining := w.Header().Get("X-RateLimit-Remaining")
		if remaining == "" {
			t.Error("Expected X-RateLimit-Remaining header to be set")
		}
	})

	t.Run("rate limits per IP independently", func(t *testing.T) {
		limiter := NewIPRateLimiter(5)
		handler := RateLimitMiddleware(limiter)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		}))

		for i := 0; i < 10; i++ {
			req := httptest.NewRequest("GET", "/test", nil)
			req.RemoteAddr = "192.168.1.200:12345"
			w := httptest.NewRecorder()
			handler.ServeHTTP(w, req)
		}

		req := httptest.NewRequest("GET", "/test", nil)
		req.RemoteAddr = "192.168.1.201:12345"
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("Expected new IP to not be rate limited, got %d", w.Code)
		}
	})
}

func TestRateLimitMiddlewareConcurrent(t *testing.T) {
	limiter := NewIPRateLimiter(1000)
	handler := RateLimitMiddleware(limiter)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			req := httptest.NewRequest("GET", "/test", nil)
			req.RemoteAddr = "192.168.1.50:12345"
			w := httptest.NewRecorder()
			handler.ServeHTTP(w, req)
		}()
	}
	wg.Wait()
}

func TestGetClientIP(t *testing.T) {
	t.Run("extracts IP from X-Forwarded-For", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/test", nil)
		req.Header.Set("X-Forwarded-For", "203.0.113.195, 70.41.3.18, 150.172.238.178")
		req.RemoteAddr = "127.0.0.1:12345"

		ip := getClientIP(req)
		if ip != "203.0.113.195" {
			t.Errorf("Expected '203.0.113.195', got '%s'", ip)
		}
	})

	t.Run("extracts IP from X-Real-IP", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/test", nil)
		req.Header.Set("X-Real-IP", "203.0.113.195")
		req.RemoteAddr = "127.0.0.1:12345"

		ip := getClientIP(req)
		if ip != "203.0.113.195" {
			t.Errorf("Expected '203.0.113.195', got '%s'", ip)
		}
	})

	t.Run("falls back to RemoteAddr", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/test", nil)
		req.RemoteAddr = "192.168.1.1:12345"

		ip := getClientIP(req)
		if ip != "192.168.1.1" {
			t.Errorf("Expected '192.168.1.1', got '%s'", ip)
		}
	})

	t.Run("handles single X-Forwarded-For value", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/test", nil)
		req.Header.Set("X-Forwarded-For", "203.0.113.195")
		req.RemoteAddr = "127.0.0.1:12345"

		ip := getClientIP(req)
		if ip != "203.0.113.195" {
			t.Errorf("Expected '203.0.113.195', got '%s'", ip)
		}
	})
}

func TestBodySizeLimitMiddleware(t *testing.T) {
	t.Run("allows small POST body", func(t *testing.T) {
		handler := BodySizeLimitMiddleware(1024)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			buf := make([]byte, 100)
			_, err := r.Body.Read(buf)
			if err != nil && err.Error() != "EOF" {
				http.Error(w, "Read error", http.StatusInternalServerError)
				return
			}
			w.WriteHeader(http.StatusOK)
		}))

		body := bytes.NewReader([]byte(`{"test": "data"}`))
		req := httptest.NewRequest("POST", "/test", body)
		w := httptest.NewRecorder()

		handler.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("Expected status 200, got %d", w.Code)
		}
	})

	t.Run("rejects oversized POST body", func(t *testing.T) {
		handler := BodySizeLimitMiddleware(100)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			buf := make([]byte, 200)
			_, err := r.Body.Read(buf)
			if err != nil {
				var maxBytesErr *http.MaxBytesError
				if err.Error() == "http: request body too large" {
					http.Error(w, "Payload Too Large", http.StatusRequestEntityTooLarge)
					return
				}
			}
			w.WriteHeader(http.StatusOK)
		}))

		largeBody := bytes.NewReader(make([]byte, 200))
		req := httptest.NewRequest("POST", "/test", largeBody)
		w := httptest.NewRecorder()

		handler.ServeHTTP(w, req)

		if w.Code != http.StatusRequestEntityTooLarge {
			t.Errorf("Expected status 413, got %d", w.Code)
		}
	})

	t.Run("does not limit GET requests", func(t *testing.T) {
		handler := BodySizeLimitMiddleware(10)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		}))

		req := httptest.NewRequest("GET", "/test", nil)
		w := httptest.NewRecorder()

		handler.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("Expected status 200, got %d", w.Code)
		}
	})
}

func TestDecodeJSONBody(t *testing.T) {
	t.Run("decodes valid JSON", func(t *testing.T) {
		body := strings.NewReader(`{"name": "test"}`)
		req := httptest.NewRequest("POST", "/test", body)
		w := httptest.NewRecorder()

		var result struct {
			Name string `json:"name"`
		}

		err := DecodeJSONBody(w, req, &result)
		if err != nil {
			t.Errorf("Expected no error, got %v", err)
		}

		if result.Name != "test" {
			t.Errorf("Expected name 'test', got '%s'", result.Name)
		}
	})

	t.Run("returns error for invalid JSON", func(t *testing.T) {
		body := strings.NewReader(`{invalid json}`)
		req := httptest.NewRequest("POST", "/test", body)
		w := httptest.NewRecorder()

		var result struct{}
		err := DecodeJSONBody(w, req, &result)

		if err == nil {
			t.Error("Expected error for invalid JSON")
		}

		if w.Code != http.StatusBadRequest {
			t.Errorf("Expected status 400, got %d", w.Code)
		}
	})

	t.Run("returns 413 for oversized body", func(t *testing.T) {
		largeBody := strings.NewReader(strings.Repeat("x", 200))
		req := httptest.NewRequest("POST", "/test", largeBody)
		req.Body = http.MaxBytesReader(httptest.NewRecorder(), req.Body, 100)
		w := httptest.NewRecorder()

		var result struct{}
		err := DecodeJSONBody(w, req, &result)

		if err == nil {
			t.Error("Expected error for oversized body")
		}

		if w.Code != http.StatusRequestEntityTooLarge {
			t.Errorf("Expected status 413, got %d", w.Code)
		}
	})
}

func TestIsValidQuidID(t *testing.T) {
	tests := []struct {
		input    string
		expected bool
	}{
		{"a1b2c3d4e5f6a7b8", true},
		{"0000000000000000", true},
		{"ffffffffffffffff", true},
		{"abcdef1234567890", true},
		{"ABCDEF1234567890", false},
		{"a1b2c3d4e5f6a7b", false},
		{"a1b2c3d4e5f6a7b89", false},
		{"g1b2c3d4e5f6a7b8", false},
		{"a1b2c3d4-5f6a7b8", false},
		{"", false},
		{"abc", false},
	}

	for _, tc := range tests {
		t.Run(tc.input, func(t *testing.T) {
			result := IsValidQuidID(tc.input)
			if result != tc.expected {
				t.Errorf("IsValidQuidID(%q) = %v, expected %v", tc.input, result, tc.expected)
			}
		})
	}
}

func TestValidateStringField(t *testing.T) {
	t.Run("accepts valid string within limit", func(t *testing.T) {
		if !ValidateStringField("hello world", 100) {
			t.Error("Expected valid string to pass")
		}
	})

	t.Run("rejects string exceeding max length", func(t *testing.T) {
		longString := strings.Repeat("a", 101)
		if ValidateStringField(longString, 100) {
			t.Error("Expected long string to fail")
		}
	})

	t.Run("accepts allowed whitespace characters", func(t *testing.T) {
		if !ValidateStringField("hello\nworld\twith\rspaces", 100) {
			t.Error("Expected string with allowed whitespace to pass")
		}
	})

	t.Run("rejects control characters", func(t *testing.T) {
		if ValidateStringField("hello\x00world", 100) {
			t.Error("Expected string with null byte to fail")
		}

		if ValidateStringField("hello\x07world", 100) {
			t.Error("Expected string with bell character to fail")
		}
	})

	t.Run("accepts empty string", func(t *testing.T) {
		if !ValidateStringField("", 100) {
			t.Error("Expected empty string to pass")
		}
	})

	t.Run("handles exact max length", func(t *testing.T) {
		exactString := strings.Repeat("a", 100)
		if !ValidateStringField(exactString, 100) {
			t.Error("Expected string at exact max length to pass")
		}
	})
}

func TestContainsControlCharacters(t *testing.T) {
	tests := []struct {
		input    string
		expected bool
	}{
		{"normal text", false},
		{"with\nnewline", false},
		{"with\ttab", false},
		{"with\rcarriage", false},
		{"with\x00null", true},
		{"with\x07bell", true},
		{"with\x1bescape", true},
		{"", false},
	}

	for _, tc := range tests {
		t.Run(tc.input, func(t *testing.T) {
			result := ContainsControlCharacters(tc.input)
			if result != tc.expected {
				t.Errorf("ContainsControlCharacters(%q) = %v, expected %v", tc.input, result, tc.expected)
			}
		})
	}
}
