package core

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode"

	"github.com/google/uuid"
	"github.com/quidnug/quidnug/internal/ratelimit"
)

// RateLimitMiddleware creates rate limiting middleware. The rate-
// limiter type itself lives in internal/ratelimit (self-contained,
// no core-type dependencies); this middleware wires it into the HTTP
// request path and pulls the client IP through the trusted-proxy
// gate in getClientIP.
func RateLimitMiddleware(limiter *ratelimit.IPRateLimiter) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ip := getClientIP(r)
			l := limiter.GetLimiter(ip)

			remaining := int(l.Tokens())
			w.Header().Set("X-RateLimit-Remaining", fmt.Sprintf("%d", remaining))

			if !l.Allow() {
				http.Error(w, "Too Many Requests", http.StatusTooManyRequests)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// trustedProxyNets is the parsed CIDR list from the TRUSTED_PROXIES env var.
// When empty, getClientIP falls back to "loopback or RFC1918 private" as the
// default definition of "trusted immediate peer" — which matches a typical
// localhost / container-sidecar reverse-proxy deployment and keeps unit tests
// (which use 127.0.0.1 RemoteAddr) passing.
//
// Operators exposing the node directly to the public internet should set
// TRUSTED_PROXIES="" explicitly-empty behaviour is NOT the same as unset:
// with an explicit empty list callers should also set TRUST_CLIENT_IP_HEADERS=false
// to fully ignore XFF / X-Real-IP. See docs/architecture.md.
var (
	trustedProxyNets     []*net.IPNet
	trustedProxyOnce     sync.Once
	clientIPHeadersTrust bool = true // flipped off via TRUST_CLIENT_IP_HEADERS=false
)

func loadTrustedProxyConfig() {
	trustedProxyOnce.Do(func() {
		if v := os.Getenv("TRUST_CLIENT_IP_HEADERS"); strings.EqualFold(v, "false") {
			clientIPHeadersTrust = false
		}
		raw := os.Getenv("TRUSTED_PROXIES")
		if raw == "" {
			return
		}
		for _, part := range strings.Split(raw, ",") {
			part = strings.TrimSpace(part)
			if part == "" {
				continue
			}
			// Allow bare IPs (treated as /32 or /128).
			if !strings.Contains(part, "/") {
				if ip := net.ParseIP(part); ip != nil {
					if ip.To4() != nil {
						part += "/32"
					} else {
						part += "/128"
					}
				}
			}
			if _, network, err := net.ParseCIDR(part); err == nil {
				trustedProxyNets = append(trustedProxyNets, network)
			}
		}
	})
}

// ResetTrustedProxyConfigForTesting allows tests to re-read env vars. It is
// safe to call from test code only.
func ResetTrustedProxyConfigForTesting() {
	trustedProxyOnce = sync.Once{}
	trustedProxyNets = nil
	clientIPHeadersTrust = true
}

// isTrustedPeer reports whether an immediate-peer IP is allowed to set
// forwarding headers.
func isTrustedPeer(ip net.IP) bool {
	if ip == nil {
		return false
	}
	if len(trustedProxyNets) > 0 {
		for _, n := range trustedProxyNets {
			if n.Contains(ip) {
				return true
			}
		}
		return false
	}
	// No explicit list: trust loopback and private ranges.
	if ip.IsLoopback() {
		return true
	}
	if ip.To4() != nil {
		// RFC1918 + link-local.
		if ip4 := ip.To4(); ip4[0] == 10 ||
			(ip4[0] == 172 && ip4[1] >= 16 && ip4[1] <= 31) ||
			(ip4[0] == 192 && ip4[1] == 168) ||
			(ip4[0] == 169 && ip4[1] == 254) {
			return true
		}
		return false
	}
	// IPv6 private / link-local.
	return ip.IsPrivate() || ip.IsLinkLocalUnicast()
}

// getClientIP extracts the client IP from the request, honoring XFF and
// X-Real-IP only when the request's immediate peer is itself trusted.
// This prevents attackers from bypassing per-IP rate limits by spoofing
// these headers.
func getClientIP(r *http.Request) string {
	loadTrustedProxyConfig()

	peerHost, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		peerHost = r.RemoteAddr
	}
	peerIP := net.ParseIP(peerHost)

	if clientIPHeadersTrust && isTrustedPeer(peerIP) {
		if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
			// Left-most entry is the original client.
			if idx := strings.Index(xff, ","); idx != -1 {
				return strings.TrimSpace(xff[:idx])
			}
			return strings.TrimSpace(xff)
		}
		if xri := r.Header.Get("X-Real-IP"); xri != "" {
			return strings.TrimSpace(xri)
		}
	}

	if peerIP != nil {
		return peerIP.String()
	}
	return peerHost
}

// BodySizeLimitMiddleware limits the request body size for POST/PUT/PATCH requests
func BodySizeLimitMiddleware(maxBytes int64) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method == "POST" || r.Method == "PUT" || r.Method == "PATCH" {
				r.Body = http.MaxBytesReader(w, r.Body, maxBytes)
			}
			next.ServeHTTP(w, r)
		})
	}
}

// DecodeJSONBody decodes the request body as JSON. It rejects unknown
// fields to defend against attackers smuggling extra data through
// permissive deserialization.
func DecodeJSONBody(w http.ResponseWriter, r *http.Request, dst interface{}) error {
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	err := dec.Decode(dst)
	if err != nil {
		var maxBytesErr *http.MaxBytesError
		if errors.As(err, &maxBytesErr) {
			http.Error(w, "Payload Too Large", http.StatusRequestEntityTooLarge)
			return err
		}
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return err
	}
	return nil
}

// CORSMiddleware applies cross-origin headers based on the
// EXPLORER_CORS_ORIGINS env var: a comma-separated list of
// allowed Origin values, or "*" to allow any origin. Default
// is empty (deny). Preflight OPTIONS requests are answered
// with 204 + the appropriate headers and short-circuit the
// rest of the chain.
//
// QDP-0025 §10.2: the Quidnug Explorer SPA hosted on a
// different origin than the node (e.g., explorer.quidnug.com
// connecting to node1.quidnug.com) needs the operator to
// enable this explicitly. The embedded /explorer mode does
// not need CORS because origins match. Wildcard "*" should
// be used only in dev; in production, list explicit explorer
// origins.
//
// Example: EXPLORER_CORS_ORIGINS="http://localhost:5173,http://localhost:5174,https://explorer.quidnug.com"
//
// Wired into the middleware chain outer than NodeAuth so
// preflights for POST endpoints (which would otherwise fail
// the signature check on a body-less OPTIONS request) are
// answered cleanly.
func CORSMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		allowed := loadCORSOrigins()
		origin := r.Header.Get("Origin")
		// Always advertise that we may serve different bodies
		// based on Origin so caches don't merge entries across
		// origins.
		w.Header().Add("Vary", "Origin")
		if origin != "" && len(allowed) > 0 && originAllowed(origin, allowed) {
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
			// Headers the explorer's API client emits:
			// Authorization (for the workspace API token),
			// Content-Type (for POST bodies), X-Request-ID
			// (request tracing).
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-Request-ID")
			// Cache preflight result for 5 minutes so a busy
			// explorer session doesn't preflight every request.
			w.Header().Set("Access-Control-Max-Age", "300")
		}
		if r.Method == http.MethodOptions {
			// Preflight; short-circuit the rest of the chain.
			// Even when the origin is not allowed we return 204
			// without CORS headers, which makes the browser
			// reject the request cleanly rather than letting
			// it through.
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

var (
	corsOriginsOnce sync.Once
	corsOrigins     []string
)

func loadCORSOrigins() []string {
	corsOriginsOnce.Do(func() {
		raw := os.Getenv("EXPLORER_CORS_ORIGINS")
		if raw == "" {
			return
		}
		for _, p := range strings.Split(raw, ",") {
			if v := strings.TrimSpace(p); v != "" {
				corsOrigins = append(corsOrigins, v)
			}
		}
	})
	return corsOrigins
}

// ResetCORSConfigForTesting allows tests to re-read the env
// var. Tests only.
func ResetCORSConfigForTesting() {
	corsOriginsOnce = sync.Once{}
	corsOrigins = nil
}

func originAllowed(origin string, allowed []string) bool {
	for _, a := range allowed {
		if a == "*" || strings.EqualFold(a, origin) {
			return true
		}
	}
	return false
}

// SecurityHeadersMiddleware adds a conservative set of HTTP security headers
// to every response. HSTS is only emitted when the connection is TLS so it
// cannot be set accidentally over plaintext HTTP.
func SecurityHeadersMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := w.Header()
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("X-Frame-Options", "DENY")
		h.Set("Referrer-Policy", "no-referrer")
		h.Set("Cross-Origin-Opener-Policy", "same-origin")
		h.Set("Cross-Origin-Resource-Policy", "same-origin")
		h.Set("Permissions-Policy", "geolocation=(), microphone=(), camera=()")
		if r.TLS != nil {
			h.Set("Strict-Transport-Security", "max-age=31536000; includeSubDomains")
		}
		next.ServeHTTP(w, r)
	})
}

// RequestIDKey is the context key for request IDs
type contextKey string

const RequestIDContextKey contextKey = "requestID"

// RequestIDMiddleware generates a UUID for each request and adds it to context and response header
func RequestIDMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestID := r.Header.Get("X-Request-ID")
		if requestID == "" {
			requestID = uuid.New().String()
		}

		w.Header().Set("X-Request-ID", requestID)

		ctx := context.WithValue(r.Context(), RequestIDContextKey, requestID)

		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// GetRequestID retrieves the request ID from context
func GetRequestID(ctx context.Context) string {
	if id, ok := ctx.Value(RequestIDContextKey).(string); ok {
		return id
	}
	return ""
}

// MetricsMiddleware records HTTP request metrics
func MetricsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		wrapped := &statusResponseWriter{ResponseWriter: w, statusCode: http.StatusOK}

		next.ServeHTTP(wrapped, r)

		duration := time.Since(start).Seconds()
		path := r.URL.Path
		method := r.Method
		status := strconv.Itoa(wrapped.statusCode)

		httpRequestsTotal.WithLabelValues(method, path, status).Inc()
		httpRequestDuration.WithLabelValues(method, path).Observe(duration)
	})
}

// statusResponseWriter wraps http.ResponseWriter to capture status code
type statusResponseWriter struct {
	http.ResponseWriter
	statusCode int
}

func (w *statusResponseWriter) WriteHeader(code int) {
	w.statusCode = code
	w.ResponseWriter.WriteHeader(code)
}

// Validation helpers

var quidIDRegex = regexp.MustCompile(`^[a-f0-9]{16}$`)

// IsValidQuidID checks if a quid ID has valid format (16 lowercase hex characters)
func IsValidQuidID(id string) bool {
	return quidIDRegex.MatchString(id)
}

// Field length limits
const (
	MaxNameLength        = 256
	MaxDescriptionLength = 4096
	MaxDomainLength      = 253
)

// ValidateStringField checks for max length and control characters
func ValidateStringField(s string, maxLength int) bool {
	if len(s) > maxLength {
		return false
	}

	for _, r := range s {
		if unicode.IsControl(r) && r != '\n' && r != '\r' && r != '\t' {
			return false
		}
	}

	return true
}

// ContainsControlCharacters checks if a string contains invalid control characters
func ContainsControlCharacters(s string) bool {
	for _, r := range s {
		if unicode.IsControl(r) && r != '\n' && r != '\r' && r != '\t' {
			return true
		}
	}
	return false
}

// NodeAuthMiddleware creates middleware that verifies node-to-node request signatures.
// It checks POST requests to transaction endpoints when RequireNodeAuth is enabled.
func NodeAuthMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Only verify POST requests to transaction endpoints
		if r.Method == "POST" && isNodeToNodeEndpoint(r.URL.Path) {
			if !verifyNodeAuth(w, r) {
				return
			}
		}
		next.ServeHTTP(w, r)
	})
}

// isNodeToNodeEndpoint checks if a path is a node-to-node transaction endpoint
func isNodeToNodeEndpoint(path string) bool {
	return strings.Contains(path, "/transactions/trust") ||
		strings.Contains(path, "/transactions/identity") ||
		strings.Contains(path, "/transactions/title")
}

// verifyNodeAuth verifies the authentication of a node-to-node request.
// Returns true if verification passes or auth is not required.
func verifyNodeAuth(w http.ResponseWriter, r *http.Request) bool {
	if !IsNodeAuthRequired() {
		return true
	}

	secret := GetNodeAuthSecret()
	if secret == "" {
		logger.Error("Node authentication required but no secret configured")
		http.Error(w, "Server authentication not configured", http.StatusInternalServerError)
		return false
	}

	signature := r.Header.Get(NodeSignatureHeader)
	timestampStr := r.Header.Get(NodeTimestampHeader)

	if signature == "" || timestampStr == "" {
		http.Error(w, "Missing authentication headers", http.StatusUnauthorized)
		return false
	}

	timestamp, err := strconv.ParseInt(timestampStr, 10, 64)
	if err != nil {
		http.Error(w, "Invalid timestamp", http.StatusUnauthorized)
		return false
	}

	// Read body for verification
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "Failed to read request body", http.StatusBadRequest)
		return false
	}
	// Restore body for downstream handlers
	r.Body = io.NopCloser(bytes.NewReader(body))

	if !VerifyRequest(r.Method, r.URL.Path, body, secret, timestamp, signature) {
		http.Error(w, "Invalid signature or stale timestamp", http.StatusUnauthorized)
		return false
	}

	return true
}
