import Foundation

/// Canonical signable-bytes encoder — byte-for-byte compatible with
/// the Go, Python, Java, .NET, Rust, and JavaScript Quidnug SDKs.
///
/// Rule (see `schemas/types/canonicalization.md`):
///  1. Serialize the object to JSON.
///  2. Parse back into a generic tree.
///  3. Serialize again with **alphabetized keys** (matches Go's
///     `encoding/json` output for `map[string]interface{}`).
///  4. Exclude named top-level fields.
public enum CanonicalBytes {

    /// Return the canonical signable bytes for a dictionary.
    public static func of(_ obj: [String: Any], excludeFields: [String] = []) throws -> Data {
        var filtered = obj
        for f in excludeFields { filtered.removeValue(forKey: f) }

        let sortedValue = sortKeysDeep(filtered)

        // JSONSerialization on its own does not sort keys, even though
        // we've done recursive sort; this keeps order when we build an
        // OrderedJSON encoder-style output. Since all nested values are
        // already dictionary-ordered in our sortKeysDeep, we serialize
        // manually to preserve the order.
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
