# Quidnug — SDK Performance Benchmarks

Micro-benchmarks for the Quidnug Go SDK hot paths. These measure
the **local** work the SDK does per transaction (keygen,
canonicalization, signing, verification) — no HTTP / network.

## Running

```bash
cd tests/benchmarks
go test -bench=. -benchmem -run=^$ -benchtime=3s
```

## Reference numbers

Measured 2026-04 on a 13th-gen Intel Core i9-13900KS (32 threads).
Your mileage will vary with CPU. Lower is better.

| Benchmark | ns/op | B/op | allocs/op | Real-world meaning |
| --- | ---: | ---: | ---: | --- |
| `QuidGenerate` | 12,099 | 4,360 | 75 | fresh P-256 keypair |
| `CanonicalBytesSmall` | 3,981 | 2,443 | 73 | canonicalize an 8-field trust tx |
| `CanonicalBytesNested` | 6,431 | 4,535 | 111 | canonicalize a nested event payload |
| `Sign` | 21,199 | 6,384 | 62 | ECDSA P-256 + SHA-256 DER sign |
| `Verify` | 46,250 | 656 | 11 | ECDSA P-256 + SHA-256 DER verify |
| `SignWithCanonicalize` | 26,745 | 8,855 | 135 | full "sign a tx" path |

### Throughput implications

At these numbers, a single thread can:

- **Mint ~83 000 fresh quid identities per second.**
- **Sign ~37 000 transactions per second** (end-to-end canonicalize
  + sign).
- **Verify ~21 000 signatures per second.**

With 32 threads saturated (the measurement host), throughput scales
roughly linearly for signing and verification since ECDSA work has
no shared state. Expect ~1M sign/s and ~600k verify/s on this CPU.

For comparison, sub-millisecond HTTP round-trips to a local node
add ~500 µs each, so the network — not the SDK — is the dominant
cost above a few hundred writes per second.

## What this doesn't measure

- **Network round-trip latency.** A 10ms p50 HTTP round-trip
  adds ~100× to per-transaction cost vs. the numbers above.
- **Node-side verification.** The receiving node does its own
  signature check and nonce lookup; that's measured separately in
  `internal/core/*_test.go` benchmarks.
- **Storage I/O.** Block production writes to disk.
- **Merkle tree construction.** `BenchmarkMerkleRootBuild` is in
  `internal/core/block_merkle_test.go`.

## Cross-language parity (planned)

A companion harness will run equivalent sign/verify/canonicalize
benchmarks in every SDK (Python, Rust, JS, Java, .NET, Swift). The
goal is to publish a single table showing "SDK X does Y operations
per second" so adopters can plan capacity without asking us.

Some rough comparison points:

- **Go** uses stdlib `crypto/ecdsa` + `encoding/json`. Fastest
  path of all SDKs.
- **Rust** (p256 crate + serde_json): ~1.2–1.5× Go (zero-copy
  JSON helps; pure-Rust ECDSA is a hair slower than Go's
  assembly-optimized BoringSSL-derived code).
- **Python** (cryptography package, which uses OpenSSL): ~0.3×
  Go due to Python call overhead, though cryptography's OpenSSL
  bindings mean the ECDSA itself is fast.
- **JavaScript** (WebCrypto): similar to Rust when run in V8.

These estimates are rules of thumb; the cross-language harness
will replace them with measured numbers.

## CI

CI runs `go test -bench=.` on every PR touching `pkg/client/`,
posting the output as a workflow comment so regressions are
caught in review. A ≥10% regression on any benchmark fails the
job.

## License

Apache-2.0.
