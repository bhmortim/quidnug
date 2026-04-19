# Quidnug .NET SDK (scaffold)

Status: **SCAFFOLD — not yet on NuGet.**

This directory scaffolds a C#/.NET SDK for Quidnug, targeting
.NET 8 and .NET Standard 2.1 (runs under .NET Framework 4.8,
.NET Core 3.1+, .NET 5/6/7/8, Unity, Xamarin).

Currently ships:

- `Quid.Generate()` / `FromPrivateHex()` / `FromPublicHex()` — P-256
  keypair with the same SHA-256→16-hex-char ID derivation as every
  other Quidnug SDK.
- `Quid.Sign(...)` / `Quid.Verify(...)` — ECDSA P-256 SHA-256 with
  DER-hex output, byte-compatible with the Go reference.
- `Quidnug.Client.csproj` packaging stub.

## Roadmap to 2.0.0 release

1. `CanonicalBytes(object, string[] excludeFields)` — port of the
   round-trip-through-generic-object rule.
2. `QuidnugClient` — HTTP surface via `HttpClient`, mirroring the
   Python + Go APIs.
3. `MerkleProofFrame` + `VerifyInclusionProof(...)`.
4. Wire DTOs under `Quidnug.Client.Types`.
5. xUnit test suite + integration tests against a mock server.
6. Publish to NuGet.org as `Quidnug.Client`.

## Build

```bash
cd clients/dotnet
dotnet build
dotnet test
```

(Tests currently empty — see roadmap above.)

## Contributing

See the Python SDK under `clients/python/` for the reference
implementation. The C# API should mirror its method names wherever
feasible.

## License

Apache-2.0.
