import Foundation
import Quidnug

/// Quickstart: two-party trust.
///
/// Assumes a local node at http://localhost:8080. On iOS/macOS apps,
/// wrap this body in a Task { ... } inside a SwiftUI view or
/// @MainActor function.
@main
struct Quickstart {
    static func main() async throws {
        let client = try QuidnugClient(baseURL: "http://localhost:8080")
        _ = try await client.info()

        let alice = Quid.generate()
        let bob = Quid.generate()
        print("alice=\(alice.id) bob=\(bob.id)")

        _ = try await client.registerIdentity(
            signer: alice, name: "Alice", homeDomain: "demo.home")
        _ = try await client.registerIdentity(
            signer: bob, name: "Bob", homeDomain: "demo.home")
        _ = try await client.grantTrust(
            signer: alice, trustee: bob.id, level: 0.9, domain: "demo.home")

        let tr = try await client.getTrust(
            observer: alice.id, target: bob.id, domain: "demo.home")
        print(String(format: "trust %.3f via %@",
                     tr.trustLevel, tr.path.joined(separator: " -> ")))
    }
}
