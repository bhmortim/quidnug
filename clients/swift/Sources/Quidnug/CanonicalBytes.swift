import Foundation

/// Canonical signable-bytes encoder — byte-for-byte compatible with
/// the Go, Python, Java, .NET, Rust, and JavaScript Quidnug SDKs.
///
/// Two modes ship here:
///
/// - `v1Of`: v1.0-spec-conformant. Top-level keys preserve caller
///   insertion order (which must mirror Go struct declaration order
///   for the tx type). Nested `[String: Any]` dictionaries are
///   recursively sorted alphabetically to match Go's `encoding/json`
///   default for `map[string]interface{}`. Use this for any
///   transaction bound for a v1.0 node.
///
/// - `of`: **legacy fully-sorted** mode. Sorts every level,
///   including the top. Does NOT match v1.0 canonical form.
///   Signatures built via this will not verify against a v1.0 node.
///   Kept for backward compatibility; deprecated.
///
/// Because Swift `[String: Any]` literals do not preserve insertion
/// order, callers targeting `v1Of` must pass an array of
/// `(String, Any)` pairs (via `v1OfOrdered`) OR construct their
/// dictionary and trust that the caller explicitly builds fields in
/// struct-declaration order via sequential assignment. The
/// `v1OfOrdered` form is the safer one for production.
public enum CanonicalBytes {

    /// v1.0-conformant: preserve top-level insertion order, sort nested.
    ///
    /// Pass fields as an array of `(key, value)` pairs so the
    /// top-level order survives — `[String: Any]` literals in Swift
    /// do not guarantee iteration order.
    public static func v1OfOrdered(
        _ fields: [(String, Any)], excludeFields: [String] = []
    ) throws -> Data {
        let exclude = Set(excludeFields)
        var buf = Data()
        buf.append(UInt8(ascii: "{"))
        var first = true
        for (k, v) in fields {
            if exclude.contains(k) { continue }
            if !first { buf.append(UInt8(ascii: ",")) }
            first = false
            appendString(k, to: &buf)
            buf.append(UInt8(ascii: ":"))
            // Nested values are recursively sorted.
            try appendJSON(sortKeysDeep(v), to: &buf)
        }
        buf.append(UInt8(ascii: "}"))
        return buf
    }

    /// Convenience overload: take a `[String: Any]` dictionary and
    /// an explicit field-order list. Fields listed in `fieldOrder`
    /// are emitted in that order; any additional keys are emitted
    /// after, alphabetically (rare — typically all fields are in
    /// the order list for v1.0 transactions).
    public static func v1Of(
        _ obj: [String: Any],
        fieldOrder: [String],
        excludeFields: [String] = []
    ) throws -> Data {
        let exclude = Set(excludeFields)
        var ordered: [(String, Any)] = []
        var seen = Set<String>()
        for k in fieldOrder {
            if exclude.contains(k) { continue }
            if let v = obj[k] {
                ordered.append((k, v))
                seen.insert(k)
            }
        }
        // Trailing fields the caller didn't explicitly order.
        for k in obj.keys.sorted() {
            if seen.contains(k) || exclude.contains(k) { continue }
            ordered.append((k, obj[k]!))
        }
        return try v1OfOrdered(ordered)
    }

    /// Legacy fully-sorted mode. Sorts every level, including top.
    /// Does NOT match v1.0 canonical form.
    @available(*, deprecated, message: "Use v1Of or v1OfOrdered for v1.0 nodes")
    public static func of(_ obj: [String: Any], excludeFields: [String] = []) throws -> Data {
        var filtered = obj
        for f in excludeFields { filtered.removeValue(forKey: f) }

        let sortedValue = sortKeysDeep(filtered)
        let data = try serializeSortedJSON(sortedValue)
        return data
    }

    private static func sortKeysDeep(_ v: Any) -> Any {
        if let dict = v as? [String: Any] {
            var sortedPairs: [(String, Any)] = []
            for k in dict.keys.sorted() {
                sortedPairs.append((k, sortKeysDeep(dict[k] ?? NSNull())))
            }
            // Preserve order via a simple marker type.
            return OrderedDict(pairs: sortedPairs)
        }
        if let arr = v as? [Any] {
            return arr.map { sortKeysDeep($0) }
        }
        return v
    }

    /// Internal marker for already-sorted dictionaries.
    private struct OrderedDict {
        let pairs: [(String, Any)]
    }

    private static func serializeSortedJSON(_ v: Any) throws -> Data {
        var buf = Data()
        try appendJSON(v, to: &buf)
        return buf
    }

    private static func appendJSON(_ v: Any, to buf: inout Data) throws {
        if let od = v as? OrderedDict {
            buf.append(UInt8(ascii: "{"))
            for (i, pair) in od.pairs.enumerated() {
                if i > 0 { buf.append(UInt8(ascii: ",")) }
                appendString(pair.0, to: &buf)
                buf.append(UInt8(ascii: ":"))
                try appendJSON(pair.1, to: &buf)
            }
            buf.append(UInt8(ascii: "}"))
            return
        }
        if let arr = v as? [Any] {
            buf.append(UInt8(ascii: "["))
            for (i, item) in arr.enumerated() {
                if i > 0 { buf.append(UInt8(ascii: ",")) }
                try appendJSON(item, to: &buf)
            }
            buf.append(UInt8(ascii: "]"))
            return
        }
        if v is NSNull {
            buf.append("null".data(using: .utf8)!); return
        }
        if let b = v as? Bool {
            buf.append((b ? "true" : "false").data(using: .utf8)!); return
        }
        if let n = v as? NSNumber {
            // NSNumber distinguishes integers and doubles by their objCType.
            let t = String(cString: n.objCType)
            if t == "c" || t == "C" { // Bool passes through here sometimes
                buf.append((n.boolValue ? "true" : "false").data(using: .utf8)!); return
            }
            if t == "q" || t == "Q" || t == "i" || t == "I" || t == "l" || t == "L" || t == "s" || t == "S" {
                buf.append(String(n.int64Value).data(using: .utf8)!); return
            }
            // Double / float
            let dbl = n.doubleValue
            if dbl == floor(dbl) && abs(dbl) < 1e15 {
                buf.append(String(Int64(dbl)).data(using: .utf8)!); return
            }
            buf.append(String(dbl).data(using: .utf8)!); return
        }
        if let s = v as? String {
            appendString(s, to: &buf); return
        }
        if let i = v as? Int {
            buf.append(String(i).data(using: .utf8)!); return
        }
        if let d = v as? Double {
            if d == floor(d) && abs(d) < 1e15 {
                buf.append(String(Int64(d)).data(using: .utf8)!); return
            }
            buf.append(String(d).data(using: .utf8)!); return
        }
        // Fallback: use JSONSerialization
        let json = try JSONSerialization.data(withJSONObject: v, options: [])
        buf.append(json)
    }

    private static func appendString(_ s: String, to buf: inout Data) {
        buf.append(UInt8(ascii: "\""))
        for scalar in s.unicodeScalars {
            switch scalar {
            case "\"": buf.append(contentsOf: [UInt8(ascii: "\\"), UInt8(ascii: "\"")])
            case "\\": buf.append(contentsOf: [UInt8(ascii: "\\"), UInt8(ascii: "\\")])
            case "\n": buf.append(contentsOf: [UInt8(ascii: "\\"), UInt8(ascii: "n")])
            case "\r": buf.append(contentsOf: [UInt8(ascii: "\\"), UInt8(ascii: "r")])
            case "\t": buf.append(contentsOf: [UInt8(ascii: "\\"), UInt8(ascii: "t")])
            case "\u{08}": buf.append(contentsOf: [UInt8(ascii: "\\"), UInt8(ascii: "b")])
            case "\u{0c}": buf.append(contentsOf: [UInt8(ascii: "\\"), UInt8(ascii: "f")])
            default:
                if scalar.value < 0x20 {
                    let hex = String(format: "\\u%04x", scalar.value)
                    buf.append(hex.data(using: .utf8)!)
                } else {
                    buf.append(String(scalar).data(using: .utf8)!)
                }
            }
        }
        buf.append(UInt8(ascii: "\""))
    }
}
