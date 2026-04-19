import XCTest
import CryptoKit
@testable import Quidnug

final class MerkleTests: XCTestCase {
    private func sh(_ d: Data) -> Data { Data(SHA256.hash(data: d)) }
    private func concat(_ a: Data, _ b: Data) -> Data { var r = a; r.append(b); return r }
    private func toHex(_ d: Data) -> String { d.map { String(format: "%02x", $0) }.joined() }

    func testSingleSiblingRight() throws {
        let tx = Data("tx-1".utf8)
        let sib = sh(Data("tx-2".utf8))
        let leaf = sh(tx)
        let root = sh(concat(leaf, sib))
        let frames = [MerkleProofFrame(hash: toHex(sib), side: "right")]
        XCTAssertTrue(try Merkle.verifyInclusionProof(
            txBytes: tx, frames: frames, expectedRootHex: toHex(root)))
    }

    func testFourLeafTree() throws {
        let leaves = (0..<4).map { sh(Data("tx-\($0)".utf8)) }
        let p0 = sh(concat(leaves[0], leaves[1]))
        let p1 = sh(concat(leaves[2], leaves[3]))
        let root = sh(concat(p0, p1))
        let frames = [
            MerkleProofFrame(hash: toHex(leaves[3]), side: "right"),
            MerkleProofFrame(hash: toHex(p0), side: "left")
        ]
        XCTAssertTrue(try Merkle.verifyInclusionProof(
            txBytes: Data("tx-2".utf8), frames: frames, expectedRootHex: toHex(root)))
    }

    func testTamperedTxRejected() throws {
        let sib = sh(Data("tx-2".utf8))
        let leaf = sh(Data("tx-1".utf8))
        let root = sh(concat(leaf, sib))
        let frames = [MerkleProofFrame(hash: toHex(sib), side: "right")]
        XCTAssertFalse(try Merkle.verifyInclusionProof(
            txBytes: Data("tampered".utf8), frames: frames, expectedRootHex: toHex(root)))
    }

    func testMalformedFrameThrows() {
        let frames = [MerkleProofFrame(hash: "nothex", side: "right")]
        XCTAssertThrowsError(try Merkle.verifyInclusionProof(
            txBytes: Data("x".utf8), frames: frames,
            expectedRootHex: String(repeating: "a", count: 64)))
    }

    func testEmptyTxThrows() {
        XCTAssertThrowsError(try Merkle.verifyInclusionProof(
            txBytes: Data(), frames: [], expectedRootHex: String(repeating: "a", count: 64)))
    }
}
