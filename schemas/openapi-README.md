# Quidnug node API — OpenAPI 3.1 spec

[`openapi.yaml`](openapi.yaml) is the machine-readable specification
of the Quidnug node HTTP API. Every SDK we ship — Python, Go, JS, Rust,
Java, .NET, Swift — wraps this exact surface.

## What it enables

- **Swagger UI / Stoplight / Redoc rendering.** Point any OpenAPI
  viewer at this file for an interactive browser.
- **Generated clients.** Feed `openapi.yaml` to
  [openapi-generator](https://github.com/OpenAPITools/openapi-generator)
  or [oapi-codegen](https://github.com/deepmap/oapi-codegen) to
  produce stub clients for any language we don't yet support
  first-class (PHP, Ruby, Elixir, Haskell, …).
- **Automated compatibility checks.** CI can diff this spec against
  the live node to catch handler-level drift.
- **Postman / Insomnia / Bruno import.** One-click to a populated
  request collection.
- **Mock servers.** `prism mock openapi.yaml` spins up a fake node
  for frontend dev without a real backend.

## Viewing locally

```bash
# Swagger UI via Docker
docker run --rm -v "${PWD}:/spec" -p 8000:8080 \
    -e SWAGGER_JSON=/spec/schemas/openapi.yaml \
    swaggerapi/swagger-ui
# → http://localhost:8000
```

or

```bash
npx @redocly/cli preview-docs schemas/openapi.yaml
```

## Generating a client

```bash
# Ruby client (for instance)
openapi-generator-cli generate \
    -i schemas/openapi.yaml \
    -g ruby \
    -o /tmp/quidnug-ruby

# PHP
openapi-generator-cli generate -i schemas/openapi.yaml -g php -o /tmp/quidnug-php

# Elixir
openapi-generator-cli generate -i schemas/openapi.yaml -g elixir -o /tmp/quidnug-elixir
```

The first-class SDKs (Python / Go / JS / Rust / Java / .NET / Swift)
are **not** machine-generated — they're hand-written for idiomatic
API shapes — but the canonical-bytes + wire-types contract they honor
is defined by this spec. A generated client sits alongside them as a
fallback for languages we haven't ported yet.

## Coverage

The spec covers every documented endpoint: identity, trust, titles,
events, guardians, gossip, bootstrap, fork-block, blocks, IPFS, and
registry. Edge-case request shapes for `guardian/recovery/*` and
`fork-block` are marked as `type: object` — populate from the
corresponding struct in the relevant SDK when generating clients.

## Validation

```bash
# Lint the spec
npx @stoplight/spectral-cli lint schemas/openapi.yaml

# Validate against the actual node
npx dredd schemas/openapi.yaml http://localhost:8080
```

## License

Apache-2.0.
