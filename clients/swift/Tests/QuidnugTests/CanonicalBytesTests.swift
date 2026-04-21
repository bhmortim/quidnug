import XCTest
@testable import Quidnug

final class CanonicalBytesTests: XCTestCase {
    func testStableAcrossInsertionOrder() throws {
        let a: [String: Any] = ["b": 1, "a": 2]
        let b: [String: Any] = ["a": 2, "b": 1]
        XCTAssertEqual(try CanonicalBytes.of(a), try CanonicalBytes.of(b))
    }

    func testExcludesNamedFields() throws {
        let tx: [String: Any] = [
            "type": "TRUST",
            "signature": "abc",
            "level": 0.9
        ]
        let out = try CanonicalBytes.of(tx, excludeFields: ["signature"])
        let s = String(data: out, encoding: .utf8)!
        XCTAssertFalse(s.contains("signature"))
        XCTAssertTrue(s.contains("level"))
        XCTAssertTrue(s.contains("TRUST"))
    }

    func testSortsNestedKeys() throws {
        let outer: [String: Any] = [
            "outer": "x",
            "nested": ["z": 1, "a": 2] as [String: Any]
        ]
        let out = try CanonicalBytes.of(outer)
        let s = String(data: out, encoding: .utf8)!
        XCTAssertEqual(s, "{\"nested\":{\"a\":2,\"z\":1},\"outer\":\"x\"}")
    }

    /// v1.0-conformant path: top-level keys preserve caller
    /// insertion order (Go struct declaration order for the tx
    /// type); nested keys are sorted alphabetically.
    ///
    /// This is the behavior v1.0 nodes require. The legacy `of`
    /// helper sorts everything and does NOT match v1.0;
    /// signatures built via that helper will not verify against a
    /// v1.0 server.
    func testV1OfOrderedPreservesTopLevelSortsNested() throws {
        // Struct-declaration-order field list (mirrors an EVENT
        // tx: id, type, trustDomain, timestamp, payload).
        let nested: [String: Any] = [
            "zebra": 1,
            "apple": 2,
            "mango": 3,
            "nested": [
                "yankee": "A",
                "alpha": "B",
                "mike": "C",
            ] as [String: Any],
        ]
        let fields: [(String, Any)] = [
            ("id", "abc"),
            ("type", "EVENT"),
            ("trustDomain", "test"),
            ("timestamp", 1729468800),
            ("payload", nested),
        ]
        let out = try CanonicalBytes.v1OfOrdered(fields)
        let s = String(data: out, encoding: .utf8)!

        // Top level is in caller-specified order (id, type,
        // trustDomain, timestamp, payload — NOT alphabetical).
        // Nested keys within payload are sorted alphabetically.
        let expected = "{\"id\":\"abc\","
            + "\"type\":\"EVENT\","
            + "\"trustDomain\":\"test\","
            + "\"timestamp\":1729468800,"
            + "\"payload\":{"
            +   "\"apple\":2,"
            +   "\"mango\":3,"
            +   "\"nested\":{\"alpha\":\"B\",\"mike\":\"C\",\"yankee\":\"A\"},"
            +   "\"zebra\":1"
            + "}}"
        XCTAssertEqual(s, expected,
            "v1OfOrdered must preserve top-level order and sort nested keys")
    }

    /// Regression guard: the legacy `of` helper sorts every level.
    /// This test locks in that behavior so the two helpers stay
    /// visibly distinct; v1.0 users must call `v1Of` or
    /// `v1OfOrdered`, not `of`.
    func testLegacyOfSortsTopLevelToo() throws {
        let tx: [String: Any] = ["type": "EVENT", "id": "abc"]
        let out = try CanonicalBytes.of(tx)
        let s = String(data: out, encoding: .utf8)!
        // Alphabetical: id before type.
        XCTAssertEqual(s, "{\"id\":\"abc\",\"type\":\"EVENT\"}")
    }
}
