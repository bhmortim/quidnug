# Quidnug .NET SDK

`Quidnug.Client` — the official .NET client for
[Quidnug](https://github.com/bhmortim/quidnug), a decentralized
protocol for relational, per-observer trust. Covers the **full v2
protocol surface** (QDPs 0001–0010).

Targets .NET 8 (runs under .NET 8/9/10). Uses built-in
`System.Security.Cryptography.ECDsa`, `System.Net.Http`, and
`System.Text.Json` — **zero external runtime dependencies**.

## Install

```bash
dotnet add package Quidnug.Client
```

**PackageReference:**

```xml
<PackageReference Include="Quidnug.Client" Version="2.0.0" />
```

## Thirty-second example

```csharp
using Quidnug.Client;

using var client = new QuidnugClient("http://localhost:8080");
using var alice = Quid.Generate();
using var bob = Quid.Generate();

await client.RegisterIdentityAsync(alice, name: "Alice", homeDomain: "contractors.home");
await client.RegisterIdentityAsync(bob,   name: "Bob",   homeDomain: "contractors.home");
await client.GrantTrustAsync(alice, trustee: bob.Id, level: 0.9, domain: "contractors.home");

var tr = await client.GetTrustAsync(alice.Id, bob.Id, "contractors.home");
Console.WriteLine($"{tr.TrustLevel:F3} via {string.Join(" -> ", tr.PathOrEmpty)}");
```

Runnable examples under [`examples/`](examples/):

| Project | Shows |
| --- | --- |
| `Quickstart` | Two-party trust + relational trust query. |
| `AspNetAudit` | ASP.NET Core middleware writing every HTTP request to an event stream. |
| `GuardianRecovery` | 2-of-3 guardian set install (QDP-0002). |

## What ships

### `Quid` — ECDSA P-256 identity

```csharp
using var alice = Quid.Generate();                      // fresh keypair
using var bob   = Quid.FromPrivateHex(storedHex);       // reconstruct
using var carol = Quid.FromPublicHex(networkPubHex);    // read-only

string sig = alice.Sign(data);
bool ok = alice.Verify(data, sig);
```

P-256 + SHA-256, DER-hex signatures — byte-compatible with Go, Python,
Java, JavaScript, and Rust SDKs. The quid ID is `sha256(publicKey)[0..8]`.

### `QuidnugClient` — async HTTP surface

Every method returns a `Task`. Thread-safe — one instance may be
shared across request handlers.

| Area | Methods |
| --- | --- |
| Health | `HealthAsync`, `InfoAsync`, `NodesAsync`, `BlocksAsync` |
| Identity | `RegisterIdentityAsync`, `GetIdentityAsync` |
| Trust | `GrantTrustAsync`, `GetTrustAsync`, `GetTrustEdgesAsync`, `QueryRelationalTrustAsync` |
| Title | `RegisterTitleAsync`, `GetTitleAsync` |
| Events | `EmitEventAsync`, `GetEventStreamAsync`, `GetStreamEventsAsync` |
| Domains | `ListDomainsAsync`, `RegisterDomainAsync`, `EnsureDomainAsync`, `NodeDomainsAsync`, `UpdateNodeDomainsAsync` |
| Registry | `QueryIdentityRegistryAsync`, `QueryTrustRegistryAsync`, `QueryTitleRegistryAsync` |
| Blocks / mempool | `BlocksAsync`, `TentativeBlocksAsync`, `PendingTransactionsAsync` |
| IPFS | `IpfsPinAsync`, `IpfsGetAsync` |
| Guardians (QDP-0002/0006) | `SubmitGuardianSetUpdateAsync`, `SubmitRecoveryInit/Veto/CommitAsync`, `SubmitGuardianResignationAsync`, `GetGuardianSetAsync`, `PendingRecoveryAsync`, `GuardianResignationsAsync` |
| Gossip (QDP-0003/5) | `SubmitDomainFingerprintAsync`, `GetLatestDomainFingerprintAsync`, `SubmitAnchorGossipAsync`, `PushAnchorAsync`, `PushFingerprintAsync` |
| Bootstrap (QDP-0008) | `BootstrapStatusAsync`, `SubmitNonceSnapshotAsync`, `LatestNonceSnapshotAsync` |
| Fork-block (QDP-0009) | `SubmitForkBlockAsync`, `ForkBlockStatusAsync` |

### Full API surface

Every public method on `QuidnugClient`, with the endpoint it calls. Mirrors the Python SDK reference.
The cross-language matrix (`.NET` method ↔ JS / Java / Rust / Swift counterpart)
lives at [`docs/sdk-coverage.md`](../../docs/sdk-coverage.md).

| Method | Endpoint |
| --- | --- |
| `HealthAsync()` | `GET /api/health` |
| `InfoAsync()` | `GET /api/info` |
| `NodesAsync()` | `GET /api/nodes` |
| `BlocksAsync()` | `GET /api/blocks` |
| `TentativeBlocksAsync(domain)` | `GET /api/blocks/tentative/{domain}` |
| `PendingTransactionsAsync(limit?, offset?)` | `GET /api/transactions` |
| `ListDomainsAsync()` | `GET /api/domains` |
| `RegisterDomainAsync(domain)` | `POST /api/domains` |
| `EnsureDomainAsync(domain)` | `POST /api/domains` (idempotent) |
| `NodeDomainsAsync()` | `GET /api/node/domains` |
| `UpdateNodeDomainsAsync(domains)` | `POST /api/node/domains` |
| `RegisterIdentityAsync(signer, ...)` | `POST /api/transactions/identity` |
| `GetIdentityAsync(quidId, domain?)` | `GET /api/identity/{quidId}` |
| `QueryIdentityRegistryAsync(quidId?, limit?, offset?)` | `GET /api/registry/identity` |
| `GrantTrustAsync(signer, trustee, level, ...)` | `POST /api/transactions/trust` |
| `GetTrustAsync(observer, target, domain, maxDepth)` | `GET /api/trust/{observer}/{target}` |
| `GetTrustEdgesAsync(quidId)` | `GET /api/trust/edges/{quidId}` |
| `QueryRelationalTrustAsync(observer, target, domain, maxDepth)` | `POST /api/trust/query` |
| `QueryTrustRegistryAsync(truster?, trustee?, limit?, offset?)` | `GET /api/registry/trust` |
| `RegisterTitleAsync(signer, assetId, owners, ...)` | `POST /api/transactions/title` |
| `GetTitleAsync(assetId, domain?)` | `GET /api/title/{assetId}` |
| `QueryTitleRegistryAsync(assetId?, ownerId?, limit?, offset?)` | `GET /api/registry/title` |
| `EmitEventAsync(signer, subjectId, subjectType, eventType, ...)` | `POST /api/events` |
| `GetEventStreamAsync(subjectId, domain?)` | `GET /api/streams/{subjectId}` |
| `GetStreamEventsAsync(subjectId, domain?, limit, offset)` | `GET /api/streams/{subjectId}/events` |
| `IpfsPinAsync(content)` | `POST /api/ipfs/pin` (raw bytes) |
| `IpfsGetAsync(cid)` | `GET /api/ipfs/{cid}` (raw bytes) |
| `SubmitGuardianSetUpdateAsync(update)` | `POST /api/guardian/set-update` |
| `SubmitRecoveryInitAsync(init)` | `POST /api/guardian/recovery/init` |
| `SubmitRecoveryVetoAsync(veto)` | `POST /api/guardian/recovery/veto` |
| `SubmitRecoveryCommitAsync(commit)` | `POST /api/guardian/recovery/commit` |
| `SubmitGuardianResignationAsync(body)` | `POST /api/guardian/resign` |
| `GetGuardianSetAsync(quidId)` | `GET /api/guardian/set/{quidId}` |
| `PendingRecoveryAsync(quidId)` | `GET /api/guardian/pending-recovery/{quidId}` |
| `GuardianResignationsAsync(quidId)` | `GET /api/guardian/resignations/{quidId}` |
| `SubmitDomainFingerprintAsync(fp)` | `POST /api/domain-fingerprints` |
| `GetLatestDomainFingerprintAsync(domain)` | `GET /api/domain-fingerprints/{domain}/latest` |
| `SubmitAnchorGossipAsync(msg)` | `POST /api/anchor-gossip` |
| `PushAnchorAsync(body)` | `POST /api/gossip/push-anchor` |
| `PushFingerprintAsync(body)` | `POST /api/gossip/push-fingerprint` |
| `SubmitNonceSnapshotAsync(body)` | `POST /api/nonce-snapshots` |
| `LatestNonceSnapshotAsync(domain)` | `GET /api/nonce-snapshots/{domain}/latest` |
| `BootstrapStatusAsync()` | `GET /api/bootstrap/status` |
| `SubmitForkBlockAsync(fb)` | `POST /api/fork-block` |
| `ForkBlockStatusAsync()` | `GET /api/fork-block/status` |

### `CanonicalBytes` / `Merkle`

```csharp
byte[] signable = CanonicalBytes.Of(tx, "signature", "txId");
string sig = signer.Sign(signable);

bool ok = Merkle.VerifyInclusionProof(txBytes, frames, rootHex);
```

Canonicalization matches every other Quidnug SDK byte-for-byte. See
[`schemas/types/canonicalization.md`](../../schemas/types/canonicalization.md).

## Error handling

```csharp
try
{
    await client.GrantTrustAsync(alice, bob.Id, 0.9, "contractors.home");
}
catch (QuidnugConflictException ex)
{
    // Nonce replay, quorum not met, guardian-set-hash mismatch, ...
    Console.Error.WriteLine($"node rejected: {ex.Message} ({ex.Details["code"]})");
}
catch (QuidnugUnavailableException)
{
    // 503 / feature-not-active / bootstrapping
}
catch (QuidnugNodeException ex)
{
    // Transport / unexpected 5xx
    Console.Error.WriteLine($"HTTP {ex.StatusCode}: {ex.ResponseBody}");
}
catch (QuidnugValidationException ex)
{
    // Local precondition failed
}
```

All inherit from `QuidnugException`. `catch (QuidnugException)` handles any.

## Retry policy

- **GETs** retry up to `maxRetries` times (default 3) on 5xx and 429.
  Exponential backoff + ±100ms jitter. Honors `Retry-After`.
- **POSTs** are **not** retried — reconcile via a follow-up GET before
  replaying a write.

```csharp
using var client = new QuidnugClient(
    "https://node.example.com",
    timeout: TimeSpan.FromSeconds(60),
    maxRetries: 5,
    retryBaseDelay: TimeSpan.FromMilliseconds(500),
    authToken: Environment.GetEnvironmentVariable("QUIDNUG_TOKEN"),
    userAgent: "my-app/1.0");
```

For custom TLS / logging handlers, pass your own `HttpClient`:

```csharp
var handler = new SocketsHttpHandler { /* ... */ };
using var http = new HttpClient(handler);
using var client = new QuidnugClient("https://node.example.com", http: http);
```

## ASP.NET Core integration

```csharp
builder.Services.AddSingleton(sp =>
    new QuidnugClient(
        builder.Configuration["Quidnug:Node"]!,
        authToken: builder.Configuration["Quidnug:Token"]));

builder.Services.AddSingleton<Quid>(sp =>
{
    // Production: load from a secrets vault / HSM.
    // Dev: ephemeral keypair per process.
    return Quid.Generate();
});
```

Inject and use:

```csharp
app.MapPost("/transfer", async (TransferRequest req, QuidnugClient q, Quid me) =>
{
    var tr = await q.GetTrustAsync(me.Id, req.Recipient, "bank.treasury");
    return tr.TrustLevel >= 0.7
        ? Results.Ok("approved")
        : Results.StatusCode(403);
});
```

## Build

```bash
cd clients/dotnet
dotnet build
dotnet test           # in tests/
```

## Verifying tests

The SDK ships 24 unit tests:

```
Passed!  - Failed: 0, Passed: 24, Skipped: 0, Total: 24
  QuidTests:             5
  CanonicalBytesTests:   3
  MerkleTests:           5
  QuidnugClientTests:   11
```

Tests use a custom `HttpMessageHandler` stub — no MockHttp or
WireMock dependency.

## .NET-specific patterns

### Blazor WebAssembly

The SDK works inside Blazor WASM once you substitute a
`HttpClient` configured with the browser's `fetch` backend:

```csharp
var httpHandler = new HttpClient(new BrowserHttpHandler())
    { BaseAddress = new Uri(builder.HostEnvironment.BaseAddress) };
builder.Services.AddSingleton(new QuidnugClient("http://localhost:8080", http: httpHandler));
```

### Unity (via .NET Standard)

`Quid.Generate` and `CanonicalBytes.Of` work on Unity's Mono/IL2CPP
runtime. The `QuidnugClient` HTTP path needs `UnityWebRequest` — a
port of the interface is planned.

### Azure Functions

Register as a singleton in `Startup.cs` / `Program.cs`; the SDK is
designed for reuse across invocations. Never construct one per
invocation.

## Protocol version compatibility

| SDK | Node | QDPs |
| --- | --- | --- |
| 2.x | 2.x | 0001–0010 |

## License

Apache-2.0.
