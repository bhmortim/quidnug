import XCTest
@testable import Quidnug

final class QuidTests: XCTestCase {
    func testGenerateHasExpectedIdFormat() throws {
        let q = Quid.generate()
        XCTAssertEqual(q.id.count, 16)
        XCTAssertTrue(q.hasPrivateKey)
    }

    func testSignVerifyRoundtrip() throws {
        let q = Quid.generate()
        let sig = try q.sign(Data("hello".utf8))
        XCTAssertTrue(q.verify(Data("hello".utf8), sigHex: sig))
        XCTAssertFalse(q.verify(Data("tampered".utf8), sigHex: sig))
    }

    func testPrivateHexRoundtrip() throws {
        let q = Quid.generate()
        let q2 = try Quid.fromPrivateHex(q.privateKeyHex!)
        XCTAssertEqual(q.id, q2.id)
        XCTAssertEqual(q.publicKeyHex, q2.publicKeyHex)
    }
}
