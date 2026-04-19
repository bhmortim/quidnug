// Package otel wires the Quidnug Go SDK to OpenTelemetry tracing.
//
// Wrap an existing http.Client with InstrumentedHTTPClient, pass
// it to client.New via WithHTTPClient, and every request the SDK
// makes creates a span with the right attributes.
//
// Example:
//
//	import (
//	    "github.com/quidnug/quidnug/pkg/client"
//	    quidnugotel "github.com/quidnug/quidnug/pkg/client/otel"
//	)
//
//	c, _ := client.New("http://node.local:8080",
//	    client.WithHTTPClient(quidnugotel.InstrumentedHTTPClient()),
//	)
//
// Span attributes emitted for every SDK call:
//
//   - http.method
//   - http.url
//   - http.status_code
//   - net.peer.name (host)
//   - quidnug.sdk.version = "2.0.0"
//
// This package depends on go.opentelemetry.io/otel and its HTTP
// instrumentation. Callers who don't want the OTel dependency
// should not import this package — the core SDK has zero OTel
// dependency.
//
// # Why it's opt-in
//
// OpenTelemetry has a fair dependency footprint (~3 MB when fully
// loaded with exporters). Many Quidnug consumers run in tight
// environments (embedded, CLI tools, serverless-functions) where
// that's a non-starter. Keeping OTel in a subpackage lets adopters
// pull it in only where they need it.
package otel

import (
	"net/http"
	"time"

	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"
)

// SDKVersion is tagged on every SDK-emitted span.
const SDKVersion = "2.0.0"

// Options tunes the instrumented transport.
type Options struct {
	// Tracer lets callers inject a named tracer. Defaults to
	// otel.Tracer("github.com/quidnug/quidnug/pkg/client").
	Tracer trace.Tracer
	// Propagators let callers inject/extract context headers.
	// Defaults to the global propagator set.
	Propagators propagation.TextMapPropagator
	// Transport is the wrapped HTTP transport. Defaults to
	// http.DefaultTransport.
	Transport http.RoundTripper
	// Timeout is applied to the returned client. Defaults to 30s.
	Timeout time.Duration
}

// InstrumentedHTTPClient returns an http.Client whose transport is
// wrapped by otelhttp, so every request the Quidnug SDK makes
// creates a properly-annotated OTel span.
func InstrumentedHTTPClient(opts ...func(*Options)) *http.Client {
	o := Options{
		Transport: http.DefaultTransport,
		Timeout:   30 * time.Second,
	}
	for _, apply := range opts {
		apply(&o)
	}

	tracerOpts := []otelhttp.Option{
		otelhttp.WithSpanNameFormatter(func(op string, r *http.Request) string {
			return "Quidnug." + r.Method + " " + r.URL.Path
		}),
		otelhttp.WithSpanOptions(trace.WithAttributes(
			attribute.String("quidnug.sdk.version", SDKVersion),
		)),
	}
	if o.Propagators != nil {
		tracerOpts = append(tracerOpts, otelhttp.WithPropagators(o.Propagators))
	}

	return &http.Client{
		Timeout:   o.Timeout,
		Transport: otelhttp.NewTransport(o.Transport, tracerOpts...),
	}
}

// WithTracer injects a specific OTel tracer.
func WithTracer(t trace.Tracer) func(*Options) {
	return func(o *Options) { o.Tracer = t }
}

// WithTransport replaces the wrapped transport (for custom TLS,
// proxy, etc).
func WithTransport(t http.RoundTripper) func(*Options) {
	return func(o *Options) { o.Transport = t }
}

// WithTimeout sets the request timeout.
func WithTimeout(d time.Duration) func(*Options) {
	return func(o *Options) { o.Timeout = d }
}

// WithPropagators sets OTel context propagators.
func WithPropagators(p propagation.TextMapPropagator) func(*Options) {
	return func(o *Options) { o.Propagators = p }
}
