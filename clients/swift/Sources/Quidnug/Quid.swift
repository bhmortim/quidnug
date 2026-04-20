import Foundation
import CryptoKit

/// A cryptographic identity (ECDSA P-256). The quid ID is
/// `sha256(publicKey)[0..8]` in hex — 16 hex chars — matching
/// every other Quidnug SDK byte-for-byte.
public struct Quid: Sendable {
    public let id: String
    public let publicKeyHex: String
    public let privateKeyHex: String?
    private let privateKey: P256.Signing.PrivateKey?
    private let publicKey: P256.Signing.PublicKey

    /// Generate a fresh P-256 keypair.
    public static func generate() -> Quid {
        let sk = P256.Signing.PrivateKey()
        return from(sk: sk)
    }

    /// Rebuild from a PKCS#8 DER hex-encoded private key.
    public static func fromPrivateHex(_ hex: String) throws -> Quid {
        let der = try Hex.decode(hex)
        let sk = try P256.Signing.PrivateKey(derRepresentation: der)
        return from(sk: sk)
    }

    /// Read-only Quid from a SEC1 uncompressed hex public key.
    public static func fromPublicHex(_ hex: String) throws -> Quid {
        let raw = try Hex.decode(hex)
        let pk = try P256.Signing.PublicKey(x963Representation: raw)
        let id = quidIdOf(raw)
        return Quid(id: id, publicKeyHex: hex, privateKeyHex: nil,
                    privateKey: nil, publicKey: pk)
    }

    public var hasPrivateKey: Bool { privateKey != nil }

    /// Sign data.
    ///
    /// v1.0 canonical form: returns hex-encoded 64-byte IEEE-1363
    /// raw signature (`r||s`, each zero-padded to 32 bytes). This
    /// matches the reference node's `VerifySignature` expectation.
    ///
    /// CryptoKit's `ECDSASignature.rawRepresentation` is the
    /// IEEE-1363 form; `derRepresentation` would be DER-encoded.
    public func sign(_ data: Data) throws -> String {
        guard let sk = privateKey else { throw QuidnugError.readOnly }
        let sig = try sk.signature(for: data)
        return Hex.encode(sig.rawRepresentation)
    }

    /// Verify a hex-encoded IEEE-1363 raw 64-byte signature.
    ///
    /// v1.0 canonical form: expects exactly 64 bytes.
    /// Anything else is rejected.
    public func verify(_ data: Data, sigHex: String) -> Bool {
        guard let sigBytes = try? Hex.decode(sigHex) else { return false }
        guard sigBytes.count == 64 else { return false }
        guard let sig = try? P256.Signing.ECDSASignature(rawRepresentation: sigBytes) else { return false }
        return publicKey.isValidSignature(sig, for: data)
    }

    /// Reconstruct a Quid from a raw private scalar in hex.
    ///
    /// Used by v1.0 test vectors, which check in deterministic
    /// keys as raw scalars. For production keys, use
    /// ``fromPrivateHex`` with the PKCS8-encoded form.
    public static func fromPrivateScalarHex(_ hex: String) throws -> Quid {
        let scalar = try Hex.decode(hex)
        guard scalar.count <= 32 else {
            throw QuidnugError.validation("private scalar must be <= 32 bytes")
        }
        // Left-pad to 32 bytes.
        var padded = Data(count: 32)
        padded.replaceSubrange((32 - scalar.count)..<32, with: scalar)
        let sk = try P256.Signing.PrivateKey(rawRepresentation: padded)
        return from(sk: sk)
    }

    // --- Helpers -----------------------------------------------------------

    private static func from(sk: P256.Signing.PrivateKey) -> Quid {
        let pk = sk.publicKey
        let pub = pk.x963Representation
        let priv = sk.derRepresentation
        let id = quidIdOf(Data(pub))
        return Quid(id: id,
                    publicKeyHex: Hex.encode(Data(pub)),
                    privateKeyHex: Hex.encode(priv),
                    privateKey: sk,
                    publicKey: pk)
    }

    private static func quidIdOf(_ publicKey: Data) -> String {
        let digest = SHA256.hash(data: publicKey)
        return Hex.encode(Data(digest.prefix(8)))
    }
}
