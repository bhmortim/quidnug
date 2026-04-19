import Foundation
import CryptoKit

/// A cryptographic identity (ECDSA P-256). The quid ID is
/// `sha256(publicKey)[0..8]` in hex — 16 hex chars — matching
/// every other Quidnug SDK byte-for-byte.
public struct Quid {
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
        let der = try bytes(hex: hex)
        let sk = try P256.Signing.PrivateKey(derRepresentation: der)
        return from(sk: sk)
    }

    /// Read-only Quid from a SEC1 uncompressed hex public key.
    public static func fromPublicHex(_ hex: String) throws -> Quid {
        let raw = try bytes(hex: hex)
        let pk = try P256.Signing.PublicKey(x963Representation: raw)
        let id = quidIdOf(raw)
        return Quid(id: id, publicKeyHex: hex, privateKeyHex: nil,
                    privateKey: nil, publicKey: pk)
    }

    public var hasPrivateKey: Bool { privateKey != nil }

    /// Sign data. Returns hex-encoded DER signature.
    public func sign(_ data: Data) throws -> String {
        guard let sk = privateKey else {
            throw QuidError.readOnly
        }
        let sig = try sk.signature(for: data)
        return sig.derRepresentation.hexString
    }

    /// Verify a hex-encoded DER signature.
    public func verify(_ data: Data, sigHex: String) -> Bool {
        guard let sigBytes = try? Quid.bytes(hex: sigHex) else { return false }
        guard let sig = try? P256.Signing.ECDSASignature(derRepresentation: sigBytes)
        else { return false }
        return publicKey.isValidSignature(sig, for: data)
    }

    // --- Helpers ----------------------------------------------------

    private static func from(sk: P256.Signing.PrivateKey) -> Quid {
        let pk = sk.publicKey
        let pub = pk.x963Representation
        let priv = sk.derRepresentation
        let id = quidIdOf(Data(pub))
        return Quid(id: id,
                    publicKeyHex: Data(pub).hexString,
                    privateKeyHex: priv.hexString,
                    privateKey: sk,
                    publicKey: pk)
    }

    private static func quidIdOf(_ publicKey: Data) -> String {
        let digest = SHA256.hash(data: publicKey)
        return Data(digest.prefix(8)).hexString
    }

    private static func bytes(hex: String) throws -> Data {
        guard hex.count % 2 == 0 else { throw QuidError.badHex }
        var data = Data(capacity: hex.count / 2)
        var index = hex.startIndex
        while index < hex.endIndex {
            let next = hex.index(index, offsetBy: 2)
            guard let b = UInt8(hex[index..<next], radix: 16) else {
                throw QuidError.badHex
            }
            data.append(b)
            index = next
        }
        return data
    }
}

public enum QuidError: Error {
    case readOnly
    case badHex
}

extension Data {
    var hexString: String {
        self.map { String(format: "%02x", $0) }.joined()
    }
}
