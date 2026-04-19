# Quidnug Swift SDK

iOS 15+ / macOS 12+ client SDK for
[Quidnug](https://github.com/bhmortim/quidnug), a decentralized
protocol for relational, per-observer trust. Covers the **full v2
protocol surface** (QDPs 0001–0010).

Uses Apple's `CryptoKit` for ECDSA P-256 (no C dependencies, no
App Store review friction) and `URLSession` for HTTP.

## Install

**Swift Package Manager:**

```swift
// Package.swift
dependencies: [
    .package(url: "https://github.com/bhmortim/quidnug.git", from: "2.0.0")
]
```

Or in Xcode: **File → Add Packages** → paste the repo URL.

Requires **iOS 15+** / **macOS 12+** (for Swift concurrency + CryptoKit).

## Thirty-second example

```swift
import Quidnug

let client = try QuidnugClient(baseURL: "http://localhost:8080")

let alice = Quid.generate()
let bob = Quid.generate()

_ = try await client.registerIdentity(signer: alice, name: "Alice", homeDomain: "contractors.home")
_ = try await client.registerIdentity(signer: bob,   name: "Bob",   homeDomain: "contractors.home")
_ = try await client.grantTrust(signer: alice, trustee: bob.id, level: 0.9, domain: "contractors.home")

let tr = try await client.getTrust(observer: alice.id, target: bob.id, domain: "contractors.home")
print(String(format: "%.3f via %@", tr.trustLevel, tr.path.joined(separator: " -> ")))
```

Runnable examples under [`examples/`](examples/):

| File | Shows |
| --- | --- |
| `Quickstart.swift` | Two-party trust + relational trust query. |
| `MobileAuth.swift` | Register on first launch, emit LOGIN events, fetch audit log. |
| `TrustMatrix.swift` | SwiftUI view model rendering an N×N relational-trust grid. |

## What ships

### `Quid` — ECDSA P-256 identity

```swift
let alice = Quid.generate()                         // fresh keypair
let bob   = try Quid.fromPrivateHex(storedHex)      // reconstruct
let carol = try Quid.fromPublicHex(networkPubHex)   // read-only

let sig = try alice.sign(data)
let ok = alice.verify(data, sigHex: sig)
```

P-256 + SHA-256, DER-hex signatures — byte-compatible with Go,
Python, Java, .NET, Rust, and JavaScript SDKs. Quid ID is
`sha256(publicKey)[0..8]`.

### `QuidnugClient` — async HTTP surface

`QuidnugClient` is an `actor` so all mutation is isolated. Every
method uses `async throws`.

| Area | Methods |
| --- | --- |
| Health | `health`, `info`, `nodes` |
| Identity | `registerIdentity`, `getIdentity` |
| Trust | `grantTrust`, `getTrust`, `getTrustEdges` |
| Title | `registerTitle`, `getTitle` |
| Events | `emitEvent`, `getEventStream`, `getStreamEvents` |
| Guardians (QDP-0002) | `submitGuardianSetUpdate`, `getGuardianSet` |
| Gossip (QDP-0003) | `getLatestDomainFingerprint` |
| Bootstrap (QDP-0008) | `bootstrapStatus` |
| Fork-block (QDP-0009) | `forkBlockStatus` |

### `CanonicalBytes` / `Merkle`

```swift
let signable = try CanonicalBytes.of(tx, excludeFields: ["signature", "txId"])
let sig = try signer.sign(signable)

let ok = try Merkle.verifyInclusionProof(
    txBytes: signable,
    frames: gossipMsg.merkleProof,
    expectedRootHex: originBlock.transactionsRoot)
```

Canonicalization matches every Quidnug SDK byte-for-byte. See
[`schemas/types/canonicalization.md`](../../schemas/types/canonicalization.md).

## Error handling

```swift
do {
    _ = try await client.grantTrust(signer: alice, trustee: bob.id,
                                     level: 0.9, domain: "contractors.home")
} catch QuidnugError.conflict(let code, let message) {
    // Nonce replay, quorum not met, etc.
    print("node rejected: \(code) / \(message)")
} catch QuidnugError.unavailable {
    // 503 / feature not yet active
} catch QuidnugError.node(let status, let message) {
    // Transport / unexpected 5xx
    print("HTTP \(status): \(message)")
} catch QuidnugError.validation(let m) {
    print("local validation: \(m)")
} catch {
    print("other: \(error)")
}
```

## Retry policy

- **GETs** retry up to `maxRetries` times (default 3) on 5xx and 429
  with exponential backoff + jitter.
- **POSTs** are **not** retried — reconcile via a GET before
  replaying a write.

```swift
let client = try QuidnugClient(
    baseURL: "https://node.example.com",
    timeout: 60,
    maxRetries: 5,
    retryBaseDelay: 0.5,
    authToken: ProcessInfo.processInfo.environment["QUIDNUG_TOKEN"],
    userAgent: "my-app/1.0")
```

## iOS / macOS-specific patterns

### Storing keys in Keychain

```swift
import Security

func persist(_ quid: Quid) {
    let q: [String: Any] = [
        kSecClass as String: kSecClassKey,
        kSecAttrApplicationTag as String: "com.myapp.quid".data(using: .utf8)!,
        kSecValueData as String: Data(quid.privateKeyHex!.utf8)
    ]
    SecItemDelete(q as CFDictionary)
    SecItemAdd(q as CFDictionary, nil)
}
```

### SwiftUI wiring

```swift
@MainActor final class QuidnugEnv: ObservableObject {
    let client: QuidnugClient
    @Published var user: Quid?

    init() throws {
        self.client = try QuidnugClient(baseURL: "http://localhost:8080")
    }
}

@main struct MyApp: App {
    @StateObject var env = try! QuidnugEnv()

    var body: some Scene {
        WindowGroup { ContentView().environmentObject(env) }
    }
}
```

### Secure Enclave signing

For hardware-backed keys, substitute
`SecureEnclave.P256.Signing.PrivateKey` inside the `Quid` type — a
port is on the roadmap.

## Build + test

```bash
cd clients/swift
swift build
swift test
```

Tests ship under `Tests/QuidnugTests/`:

- `QuidTests` — 5 tests
- `CanonicalBytesTests` — 3 tests
- `MerkleTests` — 5 tests

## Protocol version compatibility

| SDK | Node | QDPs |
| --- | --- | --- |
| 2.x | 2.x | 0001–0010 |

## License

Apache-2.0.
