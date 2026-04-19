package client

import (
	"net/http"
	"time"
)

// Option configures a Client at construction.
type Option func(*Client)

// WithHTTPClient injects a custom http.Client (connection pooling,
// TLS client certs, custom RoundTripper, proxy config).
func WithHTTPClient(h *http.Client) Option {
	return func(c *Client) { c.http = h }
}

// WithTimeout sets the per-request timeout. Default 30s.
func WithTimeout(d time.Duration) Option {
	return func(c *Client) { c.timeout = d }
}

// WithMaxRetries controls retry count for transient GET failures.
// Default 3. Set to 0 to disable retries.
func WithMaxRetries(n int) Option {
	return func(c *Client) { c.maxRetries = n }
}

// WithRetryBaseDelay sets the starting backoff. Doubles each retry
// with ±100ms jitter. Default 1s.
func WithRetryBaseDelay(d time.Duration) Option {
	return func(c *Client) { c.retryBase = d }
}

// WithAuthToken adds Authorization: Bearer <token> to every request.
func WithAuthToken(token string) Option {
	return func(c *Client) { c.authToken = token }
}

// WithUserAgent overrides the User-Agent header. Default is
// "quidnug-go-sdk/2.x".
func WithUserAgent(ua string) Option {
	return func(c *Client) { c.userAgent = ua }
}
