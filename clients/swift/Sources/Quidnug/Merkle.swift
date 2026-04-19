import Foundation
import CryptoKit

/// One frame in a QDP-0010 Merkle inclusion proof.
public struct MerkleProofFrame: Codable, Sendable {
    public let hash: String
    public let side: String // "left" or "right"

    public init(hash: String, side: String) {
        self.hash = hash
        self.side = side
    }
}

public enum Merkle {
    /// Verify an inclusion proof reconstructs the expected root.
    ///
    /// Returns `true` on success, `false` on a clean mismatch, and
    /// throws `QuidnugError` for malformed input.
    public static func verifyInclusionProof(
        txBytes: Data,
        frames: [MerkleProofFrame],
        expectedRootHex: String
    ) throws -> Bool {
        guard !txBytes.isEmpty else { throw QuidnugError.crypto("txBytes is empty") }

        let root: Data
        do {
            root = try Hex.decode(expectedRootHex)
        } catch {
            throw QuidnugError.validation("expectedRoot is not valid hex")
        }
        guard root.count == 32 else {
            throw QuidnugError.validation("expectedRoot must be 32 bytes (got \(root.count))")
        }

        var current = Data(SHA256.hash(data: txBytes))
        for (i, f) in frames.enumerated() {
            let sib: Data
            do {
                sib = try Hex.decode(f.hash)
            } catch {
                throw QuidnugError.validation("frame \(i) hash is not valid hex")
            }
            guard sib.count == 32 else {
                throw QuidnugError.validation("frame \(i) hash must be 32 bytes (got \(sib.count))")
            }
            var concat = Data(capacity: 64)
            switch f.side {
            case "left":
                concat.append(sib)
                concat.append(current)
            case "right":
                concat.append(current)
                concat.append(sib)
            default:
                throw QuidnugError.validation(
                    "frame \(i) side must be 'left' or 'right', got '\(f.side)'")
            }
            current = Data(SHA256.hash(data: concat))
        }

        // Constant-time compare
        guard current.count == root.count else { return false }
        var diff: UInt8 = 0
        for i in 0..<current.count {
            diff |= current[i] ^ root[i]
        }
        return diff == 0
    }
}

enum Hex {
    static func encode(_ data: Data) -> String {
        data.map { String(format: "%02x", $0) }.joined()
    }

    static func decode(_ hex: String) throws -> Data {
        guard hex.count.isMultiple(of: 2) else { throw QuidnugError.validation("odd hex length") }
        var out = Data(capacity: hex.count / 2)
        var index = hex.startIndex
        while index < hex.endIndex {
            let next = hex.index(index, offsetBy: 2)
            guard let b = UInt8(hex[index..<next], radix: 16) else {
                throw QuidnugError.validation("invalid hex character")
            }
            out.append(b)
            index = next
        }
        return out
    }
}
