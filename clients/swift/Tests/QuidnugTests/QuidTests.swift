import XCTest
@testable import Quidnug

final class QuidTests: XCTestCase {
    func testGenerateHasExpectedIdFormat() throws {
        let q = Quid.generate()
        XCTAssertEqual(q.id.count, 16)
        XCTAssertTrue(q.hasPrivateKey)
        XCTAssertNotNil(q.publicKeyHex)
        XCTAssertNotNil(q.privateKeyHex)
    }

    func testSignVerifyRoundtrip() throws {
        let q = Quid.generate()
        let sig = try q.sign(Data("hello".utf8))
        XCTAssertTrue(q.verify(Data("hello".utf8), sigHex: sig))
        XCTAssertFalse(q.verify(Data("tampered".utf8), sigHex: sig))
    }

    func testPrivateHexRoundtrip() throws {
        let q = Quid.generate()
        let r = try Quid.fromPrivateHex(q.privateKeyHex!)
        XCTAssertEqual(q.id, r.id)
        XCTAssertEqual(q.publicKeyHex, r.publicKeyHex)
        XCTAssertTrue(r.hasPrivateKey)
    }

    func testReadOnlyCannotSign() throws {
        let q = Quid.generate()
        let ro = try Quid.fromPublicHex(q.publicKeyHex)
        XCTAssertFalse(ro.hasPrivateKey)
        XCTAssertEqual(q.id, ro.id)
        XCTAssertThrowsError(try ro.sign(Data("x".utf8)))
    }

    func testCrossVerifyFails() throws {
        let a = Quid.generate()
        let b = Quid.generate()
        let sig = try a.sign(Data("shared".utf8))
        XCTAssertFalse(b.verify(Data("shared".utf8), sigHex: sig))
    }
}
