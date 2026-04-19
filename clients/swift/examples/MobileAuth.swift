import Foundation
import Quidnug

/// Mobile-authentication example.
///
/// In a real iOS app you'd store the private key in the Keychain or
/// behind Secure Enclave (via SecureEnclave.P256.Signing.PrivateKey).
/// Here we use an in-memory Quid for brevity.
///
/// Pattern: the app registers the user's quid on first launch, then
/// emits a signed LOGIN event on every session. Downstream audit
/// dashboards query the user's event stream for tamper-evident logs
/// of authentication activity.
enum MobileAuth {
    static func registerOnFirstLaunch(
        _ client: QuidnugClient,
        persistPrivateHex: (String) -> Void
    ) async throws -> Quid {
        let quid = Quid.generate()
        persistPrivateHex(quid.privateKeyHex!)
        _ = try await client.registerIdentity(
            signer: quid,
            name: "mobile-user-\(quid.id.prefix(4))",
            homeDomain: "myapp.users")
        return quid
    }

    static func restorePersisted(
        _ client: QuidnugClient,
        privateHex: String
    ) throws -> Quid {
        try Quid.fromPrivateHex(privateHex)
    }

    static func recordLogin(
        _ client: QuidnugClient,
        _ user: Quid,
        deviceModel: String,
        ip: String
    ) async throws {
        _ = try await client.emitEvent(
            signer: user,
            subjectId: user.id,
            subjectType: "QUID",
            eventType: "LOGIN",
            domain: "myapp.users",
            payload: [
                "deviceModel": deviceModel,
                "ip": ip,
                "timestamp": Int(Date().timeIntervalSince1970)
            ])
    }

    static func fetchAuditLog(
        _ client: QuidnugClient, _ user: Quid, limit: Int = 20
    ) async throws -> [Event] {
        try await client.getStreamEvents(
            subjectId: user.id, domain: "myapp.users", limit: limit)
    }
}
