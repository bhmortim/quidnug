package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode"

	"github.com/google/uuid"
	"golang.org/x/time/rate"
)

// IPRateLimiter manages per-IP rate limiters using token bucket algorithm
type IPRateLimiter struct {
	limiters map[string]*rate.Limiter
	mu       sync.RWMutex
	rate     rate.Limit
	burst    int
}

// NewIPRateLimiter creates a new IP-based rate limiter
func NewIPRateLimiter(requestsPerMinute int) *IPRateLimiter {
	r := rate.Limit(float64(requestsPerMinute) / 60.0)
	return &IPRateLimiter{
		limiters: make(map[string]*rate.Limiter),
		rate:     r,
		burst:    requestsPerMinute,
	}
}

// GetLimiter returns the rate limiter for a given IP
func (ipl *IPRateLimiter) GetLimiter(ip string) *rate.Limiter {
	ipl.mu.Lock()
	defer ipl.mu.Unlock()

	limiter, exists := ipl.limiters[ip]
	if !exists {
		limiter = rate.NewLimiter(ipl.rate, ipl.burst)
		ipl.limiters[ip] = limiter
	}

	return limiter
}

// RateLimitMiddleware creates rate limiting middleware
func RateLimitMiddleware(limiter *IPRateLimiter) func(http.Handler) http.Handler {
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

// getClientIP extracts the client IP from the request
func getClientIP(r *http.Request) string {
	xff := r.Header.Get("X-Forwarded-For")
	if xff != "" {
		if idx := strings.Index(xff, ","); idx != -1 {
			return strings.TrimSpace(xff[:idx])
		}
		return strings.TrimSpace(xff)
	}

	xri := r.Header.Get("X-Real-IP")
	if xri != "" {
		return xri
	}

	ip, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return ip
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

// DecodeJSONBody decodes JSON body and handles max bytes errors appropriately
func DecodeJSONBody(w http.ResponseWriter, r *http.Request, dst interface{}) error {
	err := json.NewDecoder(r.Body).Decode(dst)
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
