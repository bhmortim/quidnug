# Quidnug Swift SDK (scaffold)

Status: **SCAFFOLD — not yet on Swift Package Registry.**

iOS / macOS SDK for Quidnug, targeting iOS 15+ / macOS 12+. Uses
CryptoKit for ECDSA P-256 so there are no C dependencies, no Apple
Store review friction.

## Currently shipping

- `Quid.generate() / fromPrivateHex() / fromPublicHex()`
- `Quid.sign(Data) / Quid.verify(Data, sigHex:)`
- Swift Package Manager layout + XCTest suite.

## Roadmap

1. `canonicalBytes` port.
2. `QuidnugClient` async/await API via `URLSession`.
3. `verifyInclusionProof`.
4. Wire types as `Codable` structs.

## Install

```swift
// Package.swift
.package(url: "https://github.com/bhmortim/quidnug.git", from: "2.0.0")
```

## License

Apache-2.0.
