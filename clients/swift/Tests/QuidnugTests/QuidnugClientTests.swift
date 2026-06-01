import XCTest
@testable import Quidnug

/// HTTP integration tests for ``QuidnugClient``.
///
/// Uses a `URLProtocol` stub registered on a private `URLSession`
/// so we never touch the network. Each test installs a handler
/// that maps an incoming request to a canned `(statusCode, body)`
/// response.
final class QuidnugClientTests: XCTestCase {

    // MARK: - URLProtocol stub

    /// Handler signature: receives the URLRequest, returns a tuple
    /// `(HTTP status, response Data)`. The protocol synthesizes
    /// the `HTTPURLResponse` from those.
    typealias StubHandler = (URLRequest) -> (Int, Data)

    final class StubProtocol: URLProtocol {
        static var handler: StubHandler?
        static var lastRequest: URLRequest?
        static var lastBody: Data?

        override class func canInit(with request: URLRequest) -> Bool { true }
        override class func canonicalRequest(for r: URLRequest) -> URLRequest { r }

        override func startLoading() {
            // Capture the request for assertions. URLProtocol strips
            // httpBody during the protocol pipeline so we surface it
            // via `httpBodyStream` if needed.
            var captured = request
            if captured.httpBody == nil, let stream = captured.httpBodyStream {
                stream.open()
                var data = Data()
                let buf = UnsafeMutablePointer<UInt8>.allocate(capacity: 4096)
                defer { buf.deallocate() }
                while stream.hasBytesAvailable {
                    let n = stream.read(buf, maxLength: 4096)
                    if n <= 0 { break }
                    data.append(buf, count: n)
                }
                stream.close()
                captured.httpBody = data
                Self.lastBody = data
            } else {
                Self.lastBody = captured.httpBody
            }
            Self.lastRequest = captured

            guard let h = Self.handler else {
                client?.urlProtocol(self, didFailWithError:
                    NSError(domain: "stub", code: -1))
                return
            }
            let (status, body) = h(request)
            let response = HTTPURLResponse(
                url: request.url!,
                statusCode: status,
                httpVersion: "HTTP/1.1",
                headerFields: ["Content-Type": "application/json"])!
            client?.urlProtocol(self, didReceive: response, cacheStoragePolicy: .notAllowed)
            client?.urlProtocol(self, didLoad: body)
            client?.urlProtocolDidFinishLoading(self)
        }

        override func stopLoading() {}
    }

    // MARK: - helpers

    private func makeClient(handler: @escaping StubHandler) throws -> QuidnugClient {
        StubProtocol.handler = handler
        StubProtocol.lastRequest = nil
        StubProtocol.lastBody = nil
        let cfg = URLSessionConfiguration.ephemeral
        cfg.protocolClasses = [StubProtocol.self]
        let session = URLSession(configuration: cfg)
        return try QuidnugClient(baseURL: "http://stub.local",
                                 session: session,
                                 timeout: 5,
                                 maxRetries: 0,
                                 retryBaseDelay: 0)
    }

    private func envelope(_ data: Any) -> Data {
        let wrapped: [String: Any] = ["success": true, "data": data]
        return try! JSONSerialization.data(withJSONObject: wrapped, options: [])
    }

    private func errorEnvelope(code: String, message: String = "boom") -> Data {
        let wrapped: [String: Any] = [
            "success": false,
            "error": ["code": code, "message": message],
        ]
        return try! JSONSerialization.data(withJSONObject: wrapped, options: [])
    }

    // MARK: - Node-level reads

    func testBlocksAppendsPagination() async throws {
        let client = try makeClient { _ in
            (200, self.envelope(["blocks": []]))
        }
        _ = try await client.blocks(limit: 25, offset: 10)
        let q = StubProtocol.lastRequest?.url?.query ?? ""
        XCTAssertTrue(q.contains("limit=25"))
        XCTAssertTrue(q.contains("offset=10"))
        XCTAssertEqual(StubProtocol.lastRequest?.url?.path, "/api/blocks")
    }

    func testTentativeBlocksUsesDomainPath() async throws {
        let client = try makeClient { _ in (200, self.envelope([:])) }
        _ = try await client.tentativeBlocks(domain: "contractors.home")
        XCTAssertEqual(StubProtocol.lastRequest?.url?.path,
                       "/api/blocks/tentative/contractors.home")
    }

    func testPendingTransactionsHasLimit() async throws {
        let client = try makeClient { _ in (200, self.envelope([:])) }
        _ = try await client.pendingTransactions(limit: 5)
        XCTAssertEqual(StubProtocol.lastRequest?.url?.path, "/api/transactions")
        XCTAssertEqual(StubProtocol.lastRequest?.url?.query, "limit=5")
    }

    func testListDomainsGetsRoot() async throws {
        let client = try makeClient { _ in (200, self.envelope(["domains": []])) }
        _ = try await client.listDomains()
        XCTAssertEqual(StubProtocol.lastRequest?.url?.path, "/api/domains")
        XCTAssertEqual(StubProtocol.lastRequest?.httpMethod, "GET")
    }

    func testGetNodeDomains() async throws {
        let client = try makeClient { _ in
            (200, self.envelope(["managedDomains": ["a", "b"]]))
        }
        let out = try await client.getNodeDomains()
        XCTAssertEqual(StubProtocol.lastRequest?.url?.path, "/api/node/domains")
        XCTAssertEqual(out["managedDomains"] as? [String], ["a", "b"])
    }

    func testUpdateNodeDomainsPostsBody() async throws {
        let client = try makeClient { _ in (200, self.envelope([:])) }
        _ = try await client.updateNodeDomains(["finance.home", "ops.home"])
        XCTAssertEqual(StubProtocol.lastRequest?.httpMethod, "POST")
        XCTAssertEqual(StubProtocol.lastRequest?.url?.path, "/api/node/domains")
        let body = try JSONSerialization.jsonObject(with: StubProtocol.lastBody ?? Data())
            as? [String: Any]
        XCTAssertEqual(body?["managedDomains"] as? [String],
                       ["finance.home", "ops.home"])
    }

    // MARK: - Registry queries

    func testQueryIdentityRegistryPaginates() async throws {
        let client = try makeClient { _ in (200, self.envelope([:])) }
        _ = try await client.queryIdentityRegistry(limit: 50, offset: 100, domain: "x")
        let q = StubProtocol.lastRequest?.url?.query ?? ""
        XCTAssertTrue(q.contains("limit=50"))
        XCTAssertTrue(q.contains("offset=100"))
        XCTAssertTrue(q.contains("domain=x"))
        XCTAssertEqual(StubProtocol.lastRequest?.url?.path, "/api/registry/identity")
    }

    func testQueryTrustRegistry() async throws {
        let client = try makeClient { _ in (200, self.envelope([:])) }
        _ = try await client.queryTrustRegistry()
        XCTAssertEqual(StubProtocol.lastRequest?.url?.path, "/api/registry/trust")
    }

    func testQueryTitleRegistry() async throws {
        let client = try makeClient { _ in (200, self.envelope([:])) }
        _ = try await client.queryTitleRegistry(limit: 1)
        XCTAssertEqual(StubProtocol.lastRequest?.url?.path, "/api/registry/title")
        XCTAssertEqual(StubProtocol.lastRequest?.url?.query, "limit=1")
    }

    // MARK: - Relational trust + domain query

    func testQueryRelationalTrustPostsBody() async throws {
        let client = try makeClient { _ in
            (200, self.envelope([
                "observer": "A", "target": "B",
                "trustLevel": 0.75, "trustPath": ["A", "X", "B"],
                "pathDepth": 2, "domain": "d",
            ]))
        }
        let r = try await client.queryRelationalTrust(
            observer: "A", target: "B", domain: "d", maxDepth: 3)
        XCTAssertEqual(r.observer, "A")
        XCTAssertEqual(r.target, "B")
        XCTAssertEqual(r.trustLevel, 0.75, accuracy: 1e-9)
        XCTAssertEqual(r.pathDepth, 2)
        XCTAssertEqual(StubProtocol.lastRequest?.httpMethod, "POST")
        XCTAssertEqual(StubProtocol.lastRequest?.url?.path, "/api/trust/query")
        let body = try JSONSerialization.jsonObject(with: StubProtocol.lastBody ?? Data())
            as? [String: Any]
        XCTAssertEqual(body?["maxDepth"] as? Int, 3)
    }

    func testQueryDomain() async throws {
        let client = try makeClient { _ in (200, self.envelope([:])) }
        _ = try await client.queryDomain(domain: "ops.home", type: "list", param: "*")
        XCTAssertEqual(StubProtocol.lastRequest?.url?.path,
                       "/api/domains/ops.home/query")
        let q = StubProtocol.lastRequest?.url?.query ?? ""
        XCTAssertTrue(q.contains("type=list"))
        XCTAssertTrue(q.contains("param=") )
    }

    // MARK: - IPFS

    func testIpfsPinSendsRawBytes() async throws {
        let client = try makeClient { _ in
            (200, self.envelope(["cid": "Qmabc"]))
        }
        let cid = try await client.ipfsPin(Data([0xDE, 0xAD, 0xBE, 0xEF]))
        XCTAssertEqual(cid, "Qmabc")
        XCTAssertEqual(StubProtocol.lastRequest?.value(forHTTPHeaderField: "Content-Type"),
                       "application/octet-stream")
        XCTAssertEqual(StubProtocol.lastRequest?.url?.path, "/api/ipfs/pin")
        XCTAssertEqual(StubProtocol.lastBody, Data([0xDE, 0xAD, 0xBE, 0xEF]))
    }

    func testIpfsGetReturnsRawBytes() async throws {
        let payload = Data("hello-from-ipfs".utf8)
        let client = try makeClient { _ in (200, payload) }
        let got = try await client.ipfsGet(cid: "QmXYZ")
        XCTAssertEqual(got, payload)
        XCTAssertEqual(StubProtocol.lastRequest?.url?.path, "/api/ipfs/QmXYZ")
    }

    func testIpfsGet404Throws() async throws {
        let client = try makeClient { _ in (404, Data()) }
        do {
            _ = try await client.ipfsGet(cid: "nope")
            XCTFail("expected error")
        } catch QuidnugError.node(let status, _) {
            XCTAssertEqual(status, 404)
        }
    }

    // MARK: - Guardians

    func testSubmitRecoveryInit() async throws {
        let client = try makeClient { _ in (200, self.envelope([:])) }
        _ = try await client.submitRecoveryInit(["subjectQuid": "abc"])
        XCTAssertEqual(StubProtocol.lastRequest?.httpMethod, "POST")
        XCTAssertEqual(StubProtocol.lastRequest?.url?.path,
                       "/api/guardian/recovery/init")
    }

    func testSubmitGuardianResignationUsesResignPath() async throws {
        // The Go server route is /api/guardian/resign — match the
        // Python SDK rather than the descriptive "resignation" name.
        let client = try makeClient { _ in (200, self.envelope([:])) }
        _ = try await client.submitGuardianResignation(["guardian": "g1"])
        XCTAssertEqual(StubProtocol.lastRequest?.url?.path, "/api/guardian/resign")
    }

    func testGetPendingRecoveryReturnsNilOn404() async throws {
        let client = try makeClient { _ in
            (404, self.errorEnvelope(code: "NOT_FOUND", message: "no pending"))
        }
        let out = try await client.getPendingRecovery(quidId: "abc")
        XCTAssertNil(out)
    }

    func testGetPendingRecoveryReturnsData() async throws {
        let client = try makeClient { _ in
            (200, self.envelope(["state": "pending"]))
        }
        let out = try await client.getPendingRecovery(quidId: "abc")
        XCTAssertEqual(out?["state"] as? String, "pending")
    }

    func testGetGuardianResignationsExtractsList() async throws {
        let client = try makeClient { _ in
            (200, self.envelope([
                "data": [["guardian": "g1"], ["guardian": "g2"]],
            ]))
        }
        let out = try await client.getGuardianResignations(quidId: "abc")
        XCTAssertEqual(out.count, 2)
        XCTAssertEqual(out.first?["guardian"] as? String, "g1")
    }

    // MARK: - Gossip

    func testSubmitDomainFingerprint() async throws {
        let client = try makeClient { _ in (200, self.envelope([:])) }
        _ = try await client.submitDomainFingerprint(["domain": "x"])
        XCTAssertEqual(StubProtocol.lastRequest?.url?.path,
                       "/api/domain-fingerprints")
    }

    func testSubmitAnchorGossip() async throws {
        let client = try makeClient { _ in (200, self.envelope([:])) }
        _ = try await client.submitAnchorGossip(["messageId": "m"])
        XCTAssertEqual(StubProtocol.lastRequest?.url?.path, "/api/anchor-gossip")
    }

    func testPushAnchor() async throws {
        let client = try makeClient { _ in (200, self.envelope([:])) }
        _ = try await client.pushAnchor(["messageId": "m"])
        XCTAssertEqual(StubProtocol.lastRequest?.url?.path,
                       "/api/gossip/push-anchor")
    }

    func testPushFingerprint() async throws {
        let client = try makeClient { _ in (200, self.envelope([:])) }
        _ = try await client.pushFingerprint(["domain": "d"])
        XCTAssertEqual(StubProtocol.lastRequest?.url?.path,
                       "/api/gossip/push-fingerprint")
    }

    // MARK: - Bootstrap

    func testSubmitNonceSnapshot() async throws {
        let client = try makeClient { _ in (200, self.envelope([:])) }
        _ = try await client.submitNonceSnapshot(["trustDomain": "d"])
        XCTAssertEqual(StubProtocol.lastRequest?.url?.path,
                       "/api/nonce-snapshots")
    }

    func testGetLatestNonceSnapshotDecodes() async throws {
        let client = try makeClient { _ in
            (200, self.envelope([
                "blockHeight": 42,
                "blockHash": "abcd",
                "timestamp": 1_700_000_000,
                "trustDomain": "default",
                "entries": [
                    ["quid": "q1", "epoch": 1, "maxNonce": 7],
                ],
                "producerQuid": "p",
                "signature": "sig",
                "schemaVersion": 1,
            ]))
        }
        let snap = try await client.getLatestNonceSnapshot(domain: "default")
        XCTAssertNotNil(snap)
        XCTAssertEqual(snap?.blockHeight, 42)
        XCTAssertEqual(snap?.entries.count, 1)
        XCTAssertEqual(snap?.entries.first?.quid, "q1")
        XCTAssertEqual(snap?.entries.first?.maxNonce, 7)
    }

    func testGetLatestNonceSnapshot404IsNil() async throws {
        let client = try makeClient { _ in
            (404, self.errorEnvelope(code: "NOT_FOUND"))
        }
        let snap = try await client.getLatestNonceSnapshot(domain: "x")
        XCTAssertNil(snap)
    }

    // MARK: - Fork-block

    func testSubmitForkBlock() async throws {
        let client = try makeClient { _ in (200, self.envelope([:])) }
        _ = try await client.submitForkBlock(["feature": "f"])
        XCTAssertEqual(StubProtocol.lastRequest?.url?.path, "/api/fork-block")
    }

    // MARK: - Domain register / ensure

    func testRegisterDomainAddsName() async throws {
        let client = try makeClient { _ in (200, self.envelope([:])) }
        _ = try await client.registerDomain("ops.home", attrs: ["public": true])
        XCTAssertEqual(StubProtocol.lastRequest?.url?.path, "/api/domains")
        let body = try JSONSerialization.jsonObject(with: StubProtocol.lastBody ?? Data())
            as? [String: Any]
        XCTAssertEqual(body?["name"] as? String, "ops.home")
        XCTAssertEqual(body?["public"] as? Bool, true)
    }

    func testEnsureDomainSwallowsAlreadyExists() async throws {
        let client = try makeClient { _ in
            (400, self.errorEnvelope(code: "ALREADY_EXISTS",
                                     message: "trust domain already exists"))
        }
        let out = try await client.ensureDomain("ops.home")
        XCTAssertEqual(out["status"] as? String, "success")
        XCTAssertEqual(out["domain"] as? String, "ops.home")
    }

    // MARK: - Wait helpers

    func testWaitForIdentityReturnsImmediatelyWhenCommitted() async throws {
        let client = try makeClient { _ in
            (200, self.envelope([
                "quidId": "abc",
                "updateNonce": 1,
            ]))
        }
        let rec = try await client.waitForIdentity(
            quidId: "abc", timeout: 1.0, pollInterval: 0.05)
        XCTAssertEqual(rec.quidId, "abc")
    }

    func testWaitForIdentityTimesOut() async throws {
        let client = try makeClient { _ in
            (404, self.errorEnvelope(code: "NOT_FOUND"))
        }
        do {
            _ = try await client.waitForIdentity(
                quidId: "missing", timeout: 0.25, pollInterval: 0.05)
            XCTFail("expected timeout")
        } catch QuidnugError.node(let status, let message) {
            XCTAssertEqual(status, 408)
            XCTAssertTrue(message.contains("missing"))
        }
    }

    func testWaitForTitleReturnsImmediatelyWhenCommitted() async throws {
        let client = try makeClient { _ in
            (200, self.envelope(["assetId": "asset-1"]))
        }
        let t = try await client.waitForTitle(
            assetId: "asset-1", timeout: 1.0, pollInterval: 0.05)
        XCTAssertEqual(t.assetId, "asset-1")
    }

    // MARK: - Error mapping sanity

    func testConflictMaps409() async throws {
        let client = try makeClient { _ in
            (409, self.errorEnvelope(code: "DUPLICATE", message: "dup"))
        }
        do {
            _ = try await client.queryTitleRegistry()
            XCTFail("expected conflict")
        } catch QuidnugError.conflict(let code, _) {
            XCTAssertEqual(code, "DUPLICATE")
        }
    }

    func testUnavailableMaps503() async throws {
        let client = try makeClient { _ in
            (503, self.errorEnvelope(code: "NOT_READY", message: "warming"))
        }
        do {
            _ = try await client.listDomains()
            XCTFail("expected unavailable")
        } catch QuidnugError.unavailable(let code, _) {
            XCTAssertEqual(code, "NOT_READY")
        }
    }
}
