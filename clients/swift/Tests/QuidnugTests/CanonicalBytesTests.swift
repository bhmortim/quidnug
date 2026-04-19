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
}
