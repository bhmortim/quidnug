# Quidnug × OpenTelemetry

Distributed-tracing instrumentation for the Go SDK. Every HTTP call
the SDK makes emits a properly-annotated OTel span with
`quidnug.sdk.version`, `http.method`, `http.url`, status code, and
peer host.

## Why opt-in

OpenTelemetry is a ~3 MB dependency with a large transitive graph
(exporters, propagators, SDK impl). Pulling it into every Quidnug
consumer — including CLI tools and serverless functions that don't
need it — would bloat the base SDK significantly. Keeping this
wiring in a subpackage means:

- Callers who want observability just `import .../otel` and pass
  its HTTP client to `client.New`.
- Callers who don't want OTel pay nothing — the base `pkg/client/`
  has zero OTel dependency.

## Install

```bash
go get github.com/quidnug/quidnug/pkg/client/otel@latest
# plus the OTel runtime
go get go.opentelemetry.io/otel@latest
go get go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp@latest
```

## Usage

```go
package main

import (
    "context"
    "log"

    "go.opentelemetry.io/otel"
    "go.opentelemetry.io/otel/sdk/trace"

    "github.com/quidnug/quidnug/pkg/client"
    quidnugotel "github.com/quidnug/quidnug/pkg/client/otel"
)

func main() {
    // Wire up a trace provider — export to stdout, Jaeger, OTLP, etc.
    tp := trace.NewTracerProvider( /* ... */ )
    otel.SetTracerProvider(tp)

    c, err := client.New("http://node.local:8080",
        client.WithHTTPClient(quidnugotel.InstrumentedHTTPClient()),
    )
    if err != nil {
        log.Fatal(err)
    }

    // Every call now produces a span.
    _, _ = c.Health(context.Background())
}
```

## Emitted spans

Each SDK HTTP call creates one span with name
`Quidnug.{METHOD} {PATH}` (e.g. `Quidnug.POST /api/transactions/trust`)
and attributes:

| Attribute | Value |
| --- | --- |
| `quidnug.sdk.version` | `2.0.0` |
| `http.method` | the HTTP method |
| `http.url` | the full target URL |
| `http.status_code` | set on response |
| `net.peer.name` | hostname of the node |

Context propagation follows the globally-configured
`propagation.TextMapPropagator`. If you set a parent span before
calling the SDK, it becomes the parent.

## Turnkey exporters

### stdout

```go
import stdout "go.opentelemetry.io/otel/exporters/stdout/stdouttrace"
exp, _ := stdout.New(stdout.WithPrettyPrint())
tp := trace.NewTracerProvider(trace.WithBatcher(exp))
```

### OTLP over gRPC (Jaeger / Tempo / Honeycomb / Grafana Cloud)

```go
import "go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
exp, _ := otlptracegrpc.New(ctx)
tp := trace.NewTracerProvider(trace.WithBatcher(exp))
```

## Python / Rust / JS / .NET

OpenTelemetry for the other SDKs follows the same opt-in pattern —
if the upstream HTTP library supports OTel middleware (aiohttp,
httpx, reqwest, fetch, HttpClient), plugging it in automatically
instruments Quidnug calls. Language-specific recipes land alongside
each SDK's README as they're requested.

## License

Apache-2.0.
