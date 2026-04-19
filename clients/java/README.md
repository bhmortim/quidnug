# Quidnug Java / Kotlin SDK (scaffold)

Status: **SCAFFOLD — not yet on Maven Central.**

This directory scaffolds the public-API shape of a JVM SDK for
Quidnug — targeting Java 17+ (runs on Kotlin / Scala / Clojure
callers as well). It currently ships:

- `Quid.generate() / fromPrivateHex() / fromPublicHex()` — P-256
  keypair with the same SHA-256→16-hex-char ID derivation used by
  every other Quidnug SDK.
- `Quid.sign(byte[])` / `Quid.verify(byte[], sigHex)` — ECDSA P-256
  SHA-256 with DER-hex output, byte-compatible with the Go reference.
- Gradle `build.gradle.kts` with Jackson + BouncyCastle wiring.

## Roadmap to 2.0.0 release

1. `CanonicalBytes` — port of the round-trip-through-generic-object
   rule in `schemas/types/canonicalization.md`.
2. `QuidnugClient` — HTTP surface via `java.net.http.HttpClient`,
   mirroring the Python + Go APIs.
3. `MerkleProofFrame` + `verifyInclusionProof(...)`.
4. Wire types under `com.quidnug.client.types`.
5. JUnit 5 tests + integration tests against the mock server pattern
   used by the Rust crate.
6. Publish to Maven Central under `com.quidnug:quidnug-client:2.0.0`.

## Why the scaffold lands now

Enterprise Java shops need to know the **API shape** before they
commit to an adoption timeline. Shipping this skeleton together with
the reference protocol gives procurement teams a stable coordinate
even before the first complete binding release.

Every other Quidnug SDK (Python, Go, JS, Rust) is already shipping at
full protocol parity — the Java port is the last mile for JVM shops
that cannot consume Go / Rust directly.

## Contributing

Pull requests that complete the missing pieces above are welcome. See
the Python SDK as the reference implementation — the Java API should
mirror its method names and return types wherever feasible.

## License

Apache-2.0.
