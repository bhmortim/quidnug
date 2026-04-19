import Foundation

/// Structured error taxonomy matching the other Quidnug SDKs.
public enum QuidnugError: Error, CustomStringConvertible, Sendable {
    /// Local precondition failed before any network call.
    case validation(String)

    /// Node logically rejected — nonce replay, quorum not met, etc.
    case conflict(code: String, message: String)

    /// HTTP 503 or feature-not-active.
    case unavailable(code: String, message: String)

    /// Transport / unexpected 5xx / malformed response.
    case node(status: Int, message: String)

    /// Signature / key / crypto failure.
    case crypto(String)

    /// Read-only quid asked to sign.
    case readOnly

    /// Invalid hex encoding — used during CanonicalBytes / Merkle paths.
    case badHex

    public var description: String {
        switch self {
        case .validation(let m): return "validation: \(m)"
        case .conflict(let c, let m): return "conflict [\(c)]: \(m)"
        case .unavailable(let c, let m): return "unavailable [\(c)]: \(m)"
        case .node(let s, let m): return "node error (HTTP \(s)): \(m)"
        case .crypto(let m): return "crypto: \(m)"
        case .readOnly: return "quid is read-only"
        case .badHex: return "invalid hex string"
        }
    }
}
