import XCTest
import CryptoKit
@testable import Quidnug

/// Swift SDK consumer for the v1.0 cross-SDK test vectors at
/// `docs/test-vectors/v1.0/`.
///
/// Asserts the five conformance properties via the Swift SDK's
/// public API (`Quid.sign`, `Quid.verify`, `Quid.fromPublicHex`,
/// `Quid.fromPrivateScalarHex`) against the checked-in reference
/// vectors.
///
/// NOTE: Swift isn't available on the author's dev box (Windows
/// only), so these tests could not be executed locally. The file
/// compiles against Swift 5.9+ + CryptoKit + Foundation as a
/// logical continuation of the existing test suite. Verification
/// in macOS / Linux CI is pending.
final class VectorsTests: XCTestCase {

    private static func vectorsRoot() throws -> URL {
        // Walk up from the test bundle / executable path until we
        // find docs/test-vectors/v1.0.
        let fm = FileManager.default
        var url = URL(fileURLWithPath: fm.currentDirectoryPath)
        for _ in 0..<10 {
            let candidate = url.appendingPathComponent("docs")
                .appendingPathComponent("test-vectors")
                .appendingPathComponent("v1.0")
            if fm.fileExists(atPath: candidate.path) {
                return candidate
            }
            url = url.deletingLastPathComponent()
        }
        // Fall back to relative path from the test source file.
        let thisFile = URL(fileURLWithPath: #filePath)
        return thisFile
            .deletingLastPathComponent()
            .deletingLastPathComponent()
            .deletingLastPathComponent()
            .deletingLastPathComponent()
            .appendingPathComponent("docs")
            .appendingPathComponent("test-vectors")
            .appendingPathComponent("v1.0")
    }

    struct TestKey: Codable {
        let name: String
        let seed: String
        let privateScalarHex: String
        let publicKeySEC1Hex: String
        let quidId: String

        enum CodingKeys: String, CodingKey {
            case name, seed
            case privateScalarHex = "private_scalar_hex"
            case publicKeySEC1Hex = "public_key_sec1_hex"
            case quidId = "quid_id"
        }
    }

    private static func loadKeys() throws -> [String: TestKey] {
        let dir = try vectorsRoot().appendingPathComponent("test-keys")
        let contents = try FileManager.default.contentsOfDirectory(atPath: dir.path)
        var out: [String: TestKey] = [:]
        for name in contents where name.hasSuffix(".json") {
            let data = try Data(contentsOf: dir.appendingPathComponent(name))
            let k = try JSONDecoder().decode(TestKey.self, from: data)
            out[k.name] = k
        }
        return out
    }

    private static func loadVectorFile(_ name: String) throws -> [String: Any] {
        let url = try vectorsRoot().appendingPathComponent(name)
        let data = try Data(contentsOf: url)
        let obj = try JSONSerialization.jsonObject(with: data)
        guard let dict = obj as? [String: Any] else {
            throw QuidnugError.validation("\(name): not a JSON object")
        }
        return dict
    }

    private static func bytesToHex(_ b: Data) -> String {
        return b.map { String(format: "%02x", $0) }.joined()
    }

    private static func hexToBytes(_ s: String) -> Data {
        var out = Data()
        var i = s.startIndex
        while i < s.endIndex {
            let j = s.index(i, offsetBy: 2)
            if let byte = UInt8(s[i..<j], radix: 16) {
                out.append(byte)
            }
            i = j
        }
        return out
    }

    /// Run all five conformance properties for a single case.
    private func runCase(
        caseObj: [String: Any],
        key: TestKey,
        signable: Data,
        derivedId: String
    ) throws {
        let name = caseObj["name"] as? String ?? "<no-name>"
        guard let expected = caseObj["expected"] as? [String: Any] else {
            XCTFail("\(name): missing expected"); return
        }

        // Property 1: SHA-256 matches.
        let digest = SHA256.hash(data: signable)
        let shaHex = Self.bytesToHex(Data(digest))
        XCTAssertEqual(
            shaHex,
            expected["sha256_of_canonical_hex"] as? String,
            "\(name): SHA-256 mismatch"
        )

        // Property 2: hex + utf8 canonical forms match.
        XCTAssertEqual(
            Self.bytesToHex(signable),
            expected["canonical_signable_bytes_hex"] as? String,
            "\(name): canonical hex diverges"
        )
        XCTAssertEqual(
            String(data: signable, encoding: .utf8),
            expected["canonical_signable_bytes_utf8"] as? String,
            "\(name): canonical utf8 diverges"
        )

        // Property 3: ID derivation.
        XCTAssertEqual(
            derivedId,
            expected["expected_id"] as? String,
            "\(name): ID mismatch"
        )

        // Property 4: reference signature verifies via SDK.
        let qRO = try Quid.fromPublicHex(key.publicKeySEC1Hex)
        let refSigHex = expected["reference_signature_hex"] as! String
        XCTAssertTrue(
            qRO.verify(signable, sigHex: refSigHex),
            "\(name): SDK Verify rejected reference signature"
        )
        XCTAssertEqual(
            (expected["signature_length_bytes"] as? Int) ?? -1,
            64,
            "\(name): signature_length_bytes != 64"
        )

        // Property 5: tampered signature rejects.
        var tampered = Self.hexToBytes(refSigHex)
        tampered[5] ^= 0x01
        XCTAssertFalse(
            qRO.verify(signable, sigHex: Self.bytesToHex(tampered)),
            "\(name): tampered signature accepted"
        )

        // Property 6: SDK sign-verify round-trip.
        let qSign = try Quid.fromPrivateScalarHex(key.privateScalarHex)
        let sdkSig = try qSign.sign(signable)
        XCTAssertEqual(sdkSig.count, 128, "\(name): SDK signature not 64 bytes")
        XCTAssertTrue(qSign.verify(signable, sigHex: sdkSig),
            "\(name): SDK sign-verify round-trip failed")
    }

    // --- Per-type signable-bytes builders (Go struct order) ---

    private static func trustSignable(_ inp: [String: Any]) -> Data {
        var pairs: [(String, Any)] = [
            ("id", inp["id"] as? String ?? ""),
            ("type", "TRUST"),
            ("trustDomain", inp["trustDomain"] as? String ?? ""),
            ("timestamp", inp["timestamp"] as? Int ?? 0),
            ("signature", ""),
            ("publicKey", inp["publicKey"] as? String ?? ""),
            ("truster", inp["truster"] as? String ?? ""),
            ("trustee", inp["trustee"] as? String ?? ""),
            ("trustLevel", goCompatFloat(inp["trustLevel"] as? Double ?? 0)),
            ("nonce", inp["nonce"] as? Int ?? 0),
        ]
        if let desc = inp["description"] as? String, !desc.isEmpty {
            pairs.append(("description", desc))
        }
        if let vu = inp["validUntil"] as? Int, vu != 0 {
            pairs.append(("validUntil", vu))
        }
        return encodePairs(pairs)
    }

    private static func trustId(_ inp: [String: Any]) -> String {
        let pairs: [(String, Any)] = [
            ("Truster", inp["truster"] as? String ?? ""),
            ("Trustee", inp["trustee"] as? String ?? ""),
            ("TrustLevel", goCompatFloat(inp["trustLevel"] as? Double ?? 0)),
            ("TrustDomain", inp["trustDomain"] as? String ?? ""),
            ("Timestamp", inp["timestamp"] as? Int ?? 0),
        ]
        let data = encodePairs(pairs)
        let sha = SHA256.hash(data: data)
        return bytesToHex(Data(sha))
    }

    private static func identitySignable(_ inp: [String: Any]) -> Data {
        var pairs: [(String, Any)] = [
            ("id", inp["id"] as? String ?? ""),
            ("type", "IDENTITY"),
            ("trustDomain", inp["trustDomain"] as? String ?? ""),
            ("timestamp", inp["timestamp"] as? Int ?? 0),
            ("signature", ""),
            ("publicKey", inp["publicKey"] as? String ?? ""),
            ("quidId", inp["quidId"] as? String ?? ""),
            ("name", inp["name"] as? String ?? ""),
        ]
        if let desc = inp["description"] as? String, !desc.isEmpty {
            pairs.append(("description", desc))
        }
        if let attrs = inp["attributes"] as? [String: Any], !attrs.isEmpty {
            pairs.append(("attributes", attrs))
        }
        pairs.append(("creator", inp["creator"] as? String ?? ""))
        pairs.append(("updateNonce", inp["updateNonce"] as? Int ?? 0))
        if let hd = inp["homeDomain"] as? String, !hd.isEmpty {
            pairs.append(("homeDomain", hd))
        }
        return encodePairs(pairs)
    }

    private static func identityId(_ inp: [String: Any]) -> String {
        let pairs: [(String, Any)] = [
            ("QuidID", inp["quidId"] as? String ?? ""),
            ("Name", inp["name"] as? String ?? ""),
            ("Creator", inp["creator"] as? String ?? ""),
            ("TrustDomain", inp["trustDomain"] as? String ?? ""),
            ("UpdateNonce", inp["updateNonce"] as? Int ?? 0),
            ("Timestamp", inp["timestamp"] as? Int ?? 0),
        ]
        let data = encodePairs(pairs)
        return bytesToHex(Data(SHA256.hash(data: data)))
    }

    /// Match Go's `encoding/json` behavior of eliding ".0" on
    /// integer-valued floats.
    private static func goCompatFloat(_ v: Double) -> Any {
        if v.isFinite, v.rounded() == v, abs(v) < 1e15 {
            return Int(v)
        }
        return v
    }

    /// Serialize ordered key-value pairs into struct-declaration-order
    /// JSON, matching Go's `json.Marshal` on a typed struct.
    ///
    /// We use JSONSerialization on an ordered NSDictionary-like
    /// structure via manual JSON assembly to guarantee order.
    /// (Swift's JSONSerialization sorts keys when .sortedKeys is
    /// set but otherwise iterates an unordered dict — so we build
    /// the JSON string by hand.)
    private static func encodePairs(_ pairs: [(String, Any)]) -> Data {
        var json = "{"
        for (i, (k, v)) in pairs.enumerated() {
            if i > 0 { json += "," }
            json += "\"\(k)\":\(encodeValue(v))"
        }
        json += "}"
        return Data(json.utf8)
    }

    private static func encodeValue(_ v: Any) -> String {
        if let s = v as? String {
            let encoded = s
                .replacingOccurrences(of: "\\", with: "\\\\")
                .replacingOccurrences(of: "\"", with: "\\\"")
                .replacingOccurrences(of: "\n", with: "\\n")
                .replacingOccurrences(of: "\r", with: "\\r")
                .replacingOccurrences(of: "\t", with: "\\t")
            return "\"\(encoded)\""
        }
        if let i = v as? Int {
            return String(i)
        }
        if let d = v as? Double {
            // Non-integer-valued float path; rare in v1.0 vectors.
            return String(d)
        }
        if let dict = v as? [String: Any] {
            // Objects within payload/attributes: use JSONSerialization
            // with .sortedKeys for stability of nested keys (matches
            // Go's map[string]interface{} alphabetization).
            if let data = try? JSONSerialization.data(
                withJSONObject: dict,
                options: [.sortedKeys, .withoutEscapingSlashes]),
               let s = String(data: data, encoding: .utf8) {
                return s
            }
            return "{}"
        }
        if let arr = v as? [Any] {
            let parts = arr.map { encodeValue($0) }
            return "[\(parts.joined(separator: ","))]"
        }
        if let b = v as? Bool {
            return b ? "true" : "false"
        }
        return "null"
    }

    // --- Tests ---

    func testTrustVectors() throws {
        let keys = try Self.loadKeys()
        let vf = try Self.loadVectorFile("trust-tx.json")
        XCTAssertEqual(vf["tx_type"] as? String, "TRUST")
        let cases = vf["cases"] as! [[String: Any]]
        XCTAssertFalse(cases.isEmpty)

        for c in cases {
            let ref = c["signer_key_ref"] as! String
            let key = keys[ref]!
            let inp = c["input"] as! [String: Any]
            let signable = Self.trustSignable(inp)
            let id = Self.trustId(inp)
            try runCase(caseObj: c, key: key, signable: signable, derivedId: id)
        }
    }

    func testIdentityVectors() throws {
        let keys = try Self.loadKeys()
        let vf = try Self.loadVectorFile("identity-tx.json")
        XCTAssertEqual(vf["tx_type"] as? String, "IDENTITY")
        let cases = vf["cases"] as! [[String: Any]]
        XCTAssertFalse(cases.isEmpty)

        for c in cases {
            let ref = c["signer_key_ref"] as! String
            let key = keys[ref]!
            let inp = c["input"] as! [String: Any]
            let signable = Self.identitySignable(inp)
            let id = Self.identityId(inp)
            try runCase(caseObj: c, key: key, signable: signable, derivedId: id)
        }
    }

    func testSdkSignProducesIEEE1363() throws {
        let q = Quid.generate()
        let data = Data("test-data".utf8)
        let sigHex = try q.sign(data)
        XCTAssertEqual(sigHex.count, 128,
            "v1.0 mandates 64-byte IEEE-1363 (128 hex chars)")
        XCTAssertTrue(q.verify(data, sigHex: sigHex))
    }
}
