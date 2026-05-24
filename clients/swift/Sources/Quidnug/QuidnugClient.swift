import Foundation

/// Strongly-typed async HTTP client for a Quidnug node.
///
/// Covers the full v2 protocol surface. Thread-safe by construction
/// (immutable configuration + Foundation's thread-safe `URLSession`).
public actor QuidnugClient {
    private let apiBase: URL
    private let session: URLSession
    private let timeout: TimeInterval
    private let maxRetries: Int
    private let retryBaseDelay: TimeInterval
    private let authToken: String?
    private let userAgent: String

    private static let conflictCodes: Set<String> = [
        "NONCE_REPLAY", "GUARDIAN_SET_MISMATCH", "QUORUM_NOT_MET", "VETOED",
        "INVALID_SIGNATURE", "FORK_ALREADY_ACTIVE", "DUPLICATE",
        "ALREADY_EXISTS", "INVALID_STATE_TRANSITION"
    ]
    private static let unavailableCodes: Set<String> = [
        "FEATURE_NOT_ACTIVE", "NOT_READY", "BOOTSTRAPPING"
    ]

    public init(
        baseURL: String,
        session: URLSession = .shared,
        timeout: TimeInterval = 30,
        maxRetries: Int = 3,
        retryBaseDelay: TimeInterval = 1.0,
        authToken: String? = nil,
        userAgent: String = "quidnug-swift-sdk/2.0.0"
    ) throws {
        guard let base = URL(string: baseURL.hasSuffix("/")
                              ? String(baseURL.dropLast()) + "/api"
                              : baseURL + "/api") else {
            throw QuidnugError.validation("baseURL is required")
        }
        self.apiBase = base
        self.session = session
        self.timeout = timeout
        self.maxRetries = maxRetries
        self.retryBaseDelay = retryBaseDelay
        self.authToken = authToken
        self.userAgent = userAgent
    }

    // =========================================================================
    // Health / info / peers
    // =========================================================================

    public func health() async throws -> [String: Any] {
        try await requestJSON(method: "GET", path: "health", body: nil)
    }

    public func info() async throws -> [String: Any] {
        try await requestJSON(method: "GET", path: "info", body: nil)
    }

    public func nodes() async throws -> [String: Any] {
        try await requestJSON(method: "GET", path: "nodes", body: nil)
    }

    // =========================================================================
    // Identity
    // =========================================================================

    public func registerIdentity(
        signer: Quid,
        name: String? = nil,
        homeDomain: String? = nil,
        domain: String = "default",
        description: String? = nil,
        attributes: [String: Any]? = nil,
        updateNonce: Int64 = 1
    ) async throws -> [String: Any] {
        try requireSigner(signer)
        var tx: [String: Any] = [
            "type": "IDENTITY",
            "timestamp": Int(Date().timeIntervalSince1970),
            "trustDomain": domain,
            "signerQuid": signer.id,
            "definerQuid": signer.id,
            "subjectQuid": signer.id,
            "updateNonce": updateNonce,
            "schemaVersion": "1.0",
            "attributes": attributes ?? [:]
        ]
        if let n = name { tx["name"] = n }
        if let d = description { tx["description"] = d }
        if let h = homeDomain { tx["homeDomain"] = h }
        try signIntoTx(signer: signer, tx: &tx)
        return try await requestJSON(method: "POST", path: "transactions/identity", body: tx)
    }

    public func getIdentity(quidId: String, domain: String? = nil) async throws -> IdentityRecord? {
        var path = "identity/\(urlEscape(quidId))"
        if let d = domain { path += "?domain=\(urlEscape(d))" }
        do {
            let raw = try await requestJSON(method: "GET", path: path, body: nil)
            return try decode(IdentityRecord.self, from: raw)
        } catch QuidnugError.validation(let m) where m.contains("NOT_FOUND") {
            return nil
        }
    }

    // =========================================================================
    // Trust
    // =========================================================================

    public func grantTrust(
        signer: Quid,
        trustee: String,
        level: Double,
        domain: String = "default",
        nonce: Int64 = 1,
        validUntil: Int64? = nil,
        description: String? = nil
    ) async throws -> [String: Any] {
        try requireSigner(signer)
        guard !trustee.isEmpty else { throw QuidnugError.validation("trustee is required") }
        guard (0.0...1.0).contains(level) else {
            throw QuidnugError.validation("level must be in [0, 1]")
        }
        var tx: [String: Any] = [
            "type": "TRUST",
            "timestamp": Int(Date().timeIntervalSince1970),
            "trustDomain": domain,
            "signerQuid": signer.id,
            "truster": signer.id,
            "trustee": trustee,
            "trustLevel": level,
            "nonce": nonce,
        ]
        if let v = validUntil { tx["validUntil"] = v }
        if let d = description { tx["description"] = d }
        try signIntoTx(signer: signer, tx: &tx)
        return try await requestJSON(method: "POST", path: "transactions/trust", body: tx)
    }

    public func getTrust(
        observer: String, target: String, domain: String, maxDepth: Int = 5
    ) async throws -> TrustResult {
        let path = "trust/\(urlEscape(observer))/\(urlEscape(target))"
                 + "?domain=\(urlEscape(domain))&maxDepth=\(maxDepth)"
        let raw = try await requestJSON(method: "GET", path: path, body: nil)
        return try decode(TrustResult.self, from: raw)
    }

    public func getTrustEdges(quidId: String) async throws -> [TrustEdge] {
        let raw = try await requestJSON(method: "GET", path: "trust/edges/\(urlEscape(quidId))", body: nil)
        let listRaw = (raw["edges"] as? [[String: Any]]) ?? (raw["data"] as? [[String: Any]]) ?? []
        let data = try JSONSerialization.data(withJSONObject: listRaw, options: [])
        return try JSONDecoder().decode([TrustEdge].self, from: data)
    }

    // =========================================================================
    // Title
    // =========================================================================

    public func registerTitle(
        signer: Quid,
        assetId: String,
        owners: [OwnershipStake],
        domain: String = "default",
        titleType: String? = nil,
        prevTitleTxId: String? = nil
    ) async throws -> [String: Any] {
        try requireSigner(signer)
        guard !assetId.isEmpty else { throw QuidnugError.validation("assetId is required") }
        guard !owners.isEmpty else { throw QuidnugError.validation("owners is required") }
        let total = owners.reduce(0.0) { $0 + $1.percentage }
        guard abs(total - 100.0) <= 0.001 else {
            throw QuidnugError.validation("owner percentages must sum to 100 (got \(total))")
        }
        var tx: [String: Any] = [
            "type": "TITLE",
            "timestamp": Int(Date().timeIntervalSince1970),
            "trustDomain": domain,
            "signerQuid": signer.id,
            "issuerQuid": signer.id,
            "assetQuid": assetId,
            "ownershipMap": owners.map { ["ownerId": $0.ownerId, "percentage": $0.percentage] },
            "transferSigs": [String: String]()
        ]
        if let t = titleType { tx["titleType"] = t }
        if let p = prevTitleTxId { tx["prevTitleTxID"] = p }
        try signIntoTx(signer: signer, tx: &tx)
        return try await requestJSON(method: "POST", path: "transactions/title", body: tx)
    }

    public func getTitle(assetId: String, domain: String? = nil) async throws -> Title? {
        var path = "title/\(urlEscape(assetId))"
        if let d = domain { path += "?domain=\(urlEscape(d))" }
        do {
            return try decode(Title.self, from: await requestJSON(method: "GET", path: path, body: nil))
        } catch QuidnugError.validation(let m) where m.contains("NOT_FOUND") {
            return nil
        }
    }

    // =========================================================================
    // Events + streams
    // =========================================================================

    public func emitEvent(
        signer: Quid,
        subjectId: String,
        subjectType: String,
        eventType: String,
        domain: String = "default",
        payload: [String: Any]? = nil,
        payloadCid: String? = nil,
        sequence: Int64 = 0
    ) async throws -> [String: Any] {
        try requireSigner(signer)
        guard subjectType == "QUID" || subjectType == "TITLE" else {
            throw QuidnugError.validation("subjectType must be 'QUID' or 'TITLE'")
        }
        guard !eventType.isEmpty else { throw QuidnugError.validation("eventType is required") }
        guard (payload == nil) != (payloadCid == nil) else {
            throw QuidnugError.validation("exactly one of payload or payloadCid is required")
        }

        var seq = sequence
        if seq == 0 {
            do {
                let s = try await getEventStream(subjectId: subjectId, domain: domain)
                if let latest = (s?["latestSequence"] as? NSNumber)?.int64Value { seq = latest + 1 }
                else { seq = 1 }
            } catch {
                seq = 1
            }
        }

        var tx: [String: Any] = [
            "type": "EVENT",
            "timestamp": Int(Date().timeIntervalSince1970),
            "trustDomain": domain,
            "subjectId": subjectId,
            "subjectType": subjectType,
            "eventType": eventType,
            "sequence": seq,
        ]
        if let p = payload { tx["payload"] = p }
        if let c = payloadCid { tx["payloadCid"] = c }

        let signable = try CanonicalBytes.of(tx, excludeFields: ["signature", "txId", "publicKey"])
        tx["signature"] = try signer.sign(signable)
        tx["publicKey"] = signer.publicKeyHex
        return try await requestJSON(method: "POST", path: "events", body: tx)
    }

    public func getEventStream(subjectId: String, domain: String? = nil) async throws -> [String: Any]? {
        do {
            var path = "streams/\(urlEscape(subjectId))"
            if let d = domain { path += "?domain=\(urlEscape(d))" }
            return try await requestJSON(method: "GET", path: path, body: nil)
        } catch QuidnugError.validation(let m) where m.contains("NOT_FOUND") {
            return nil
        }
    }

    public func getStreamEvents(
        subjectId: String, domain: String? = nil, limit: Int = 50, offset: Int = 0
    ) async throws -> [Event] {
        var path = "streams/\(urlEscape(subjectId))/events?"
        if let d = domain { path += "domain=\(urlEscape(d))&" }
        if limit > 0 { path += "limit=\(limit)&" }
        if offset > 0 { path += "offset=\(offset)" }
        let raw = try await requestJSON(method: "GET", path: path, body: nil)
        let listRaw = (raw["data"] as? [[String: Any]]) ?? (raw["events"] as? [[String: Any]]) ?? []
        let data = try JSONSerialization.data(withJSONObject: listRaw, options: [])
        return try JSONDecoder().decode([Event].self, from: data)
    }

    // =========================================================================
    // Guardians / gossip / fork-block / bootstrap
    // =========================================================================

    public func submitGuardianSetUpdate(_ update: [String: Any]) async throws -> [String: Any] {
        try await requestJSON(method: "POST", path: "guardian/set-update", body: update)
    }

    public func getGuardianSet(quidId: String) async throws -> GuardianSet? {
        do {
            return try decode(GuardianSet.self,
                              from: await requestJSON(method: "GET",
                                                      path: "guardian/set/\(urlEscape(quidId))",
                                                      body: nil))
        } catch QuidnugError.validation(let m) where m.contains("NOT_FOUND") {
            return nil
        }
    }

    public func getLatestDomainFingerprint(domain: String) async throws -> DomainFingerprint? {
        do {
            return try decode(DomainFingerprint.self,
                              from: await requestJSON(
                                method: "GET",
                                path: "domain-fingerprints/\(urlEscape(domain))/latest",
                                body: nil))
        } catch QuidnugError.validation(let m) where m.contains("NOT_FOUND") {
            return nil
        }
    }

    public func bootstrapStatus() async throws -> [String: Any] {
        try await requestJSON(method: "GET", path: "bootstrap/status", body: nil)
    }

    public func forkBlockStatus() async throws -> [String: Any] {
        try await requestJSON(method: "GET", path: "fork-block/status", body: nil)
    }

    /// POST /api/guardian/recovery/init.
    public func submitRecoveryInit(_ body: [String: Any]) async throws -> [String: Any] {
        try await requestJSON(method: "POST", path: "guardian/recovery/init", body: body)
    }

    /// POST /api/guardian/recovery/veto.
    public func submitRecoveryVeto(_ body: [String: Any]) async throws -> [String: Any] {
        try await requestJSON(method: "POST", path: "guardian/recovery/veto", body: body)
    }

    /// POST /api/guardian/recovery/commit.
    public func submitRecoveryCommit(_ body: [String: Any]) async throws -> [String: Any] {
        try await requestJSON(method: "POST", path: "guardian/recovery/commit", body: body)
    }

    /// POST /api/guardian/resign.
    public func submitGuardianResignation(_ body: [String: Any]) async throws -> [String: Any] {
        try await requestJSON(method: "POST", path: "guardian/resign", body: body)
    }

    /// GET /api/guardian/pending-recovery/{quid}.
    public func getPendingRecovery(quidId: String) async throws -> [String: Any]? {
        do {
            return try await requestJSON(
                method: "GET",
                path: "guardian/pending-recovery/\(urlEscape(quidId))",
                body: nil
            )
        } catch QuidnugError.validation(let m) where m.contains("NOT_FOUND") {
            return nil
        }
    }

    /// GET /api/guardian/resignations/{quid}.
    public func getGuardianResignations(quidId: String) async throws -> [String: Any] {
        try await requestJSON(
            method: "GET",
            path: "guardian/resignations/\(urlEscape(quidId))",
            body: nil
        )
    }

    /// POST /api/domain-fingerprints.
    public func submitDomainFingerprint(_ fp: [String: Any]) async throws -> [String: Any] {
        try await requestJSON(method: "POST", path: "domain-fingerprints", body: fp)
    }

    /// POST /api/anchor-gossip.
    public func submitAnchorGossip(_ msg: [String: Any]) async throws -> [String: Any] {
        try await requestJSON(method: "POST", path: "anchor-gossip", body: msg)
    }

    /// POST /api/gossip/push-anchor.
    public func pushAnchor(_ msg: [String: Any]) async throws -> [String: Any] {
        try await requestJSON(method: "POST", path: "gossip/push-anchor", body: msg)
    }

    /// POST /api/gossip/push-fingerprint.
    public func pushFingerprint(_ fp: [String: Any]) async throws -> [String: Any] {
        try await requestJSON(method: "POST", path: "gossip/push-fingerprint", body: fp)
    }

    /// POST /api/nonce-snapshots.
    public func submitNonceSnapshot(_ snap: [String: Any]) async throws -> [String: Any] {
        try await requestJSON(method: "POST", path: "nonce-snapshots", body: snap)
    }

    /// GET /api/nonce-snapshots/{domain}/latest.
    public func getLatestNonceSnapshot(domain: String) async throws -> [String: Any]? {
        do {
            return try await requestJSON(
                method: "GET",
                path: "nonce-snapshots/\(urlEscape(domain))/latest",
                body: nil
            )
        } catch QuidnugError.validation(let m) where m.contains("NOT_FOUND") {
            return nil
        }
    }

    /// POST /api/fork-block.
    public func submitForkBlock(_ fb: [String: Any]) async throws -> [String: Any] {
        try await requestJSON(method: "POST", path: "fork-block", body: fb)
    }

    // =========================================================================
    // Peers
    // =========================================================================

    /// GET /api/peers — peer scoreboard snapshot.
    public func peers() async throws -> [String: Any] {
        try await requestJSON(method: "GET", path: "peers", body: nil)
    }

    /// GET /api/peers/{nodeQuid} — one peer's full record, or nil.
    public func getPeer(nodeQuid: String) async throws -> [String: Any]? {
        do {
            return try await requestJSON(
                method: "GET",
                path: "peers/\(urlEscape(nodeQuid))",
                body: nil
            )
        } catch QuidnugError.validation(let m) where m.contains("NOT_FOUND") || m.contains("PEER_NOT_FOUND") {
            return nil
        }
    }

    // =========================================================================
    // Blocks / domains / quids
    // =========================================================================

    /// GET /api/blocks.
    public func blocks() async throws -> [String: Any] {
        try await requestJSON(method: "GET", path: "blocks", body: nil)
    }

    /// GET /api/blocks/tentative/{domain}.
    public func getTentativeBlocks(domain: String) async throws -> [String: Any] {
        try await requestJSON(method: "GET", path: "blocks/tentative/\(urlEscape(domain))", body: nil)
    }

    /// GET /api/transactions.
    public func pendingTransactions() async throws -> [String: Any] {
        try await requestJSON(method: "GET", path: "transactions", body: nil)
    }

    /// GET /api/domains.
    public func listDomains() async throws -> [String: Any] {
        try await requestJSON(method: "GET", path: "domains", body: nil)
    }

    /// POST /api/domains — register a trust domain.
    public func registerDomain(_ domain: String, attrs: [String: Any]? = nil) async throws -> [String: Any] {
        var body: [String: Any] = attrs ?? [:]
        body["name"] = domain
        return try await requestJSON(method: "POST", path: "domains", body: body)
    }

    /// GET /api/domains/top — top-N most-active domains.
    public func topDomains() async throws -> [String: Any] {
        try await requestJSON(method: "GET", path: "domains/top", body: nil)
    }

    /// GET /api/domains/{name}/query.
    public func queryDomain(_ domain: String, queryType: String, param: String) async throws -> [String: Any] {
        guard ["identity", "trust", "title"].contains(queryType) else {
            throw QuidnugError.validation("queryType must be 'identity', 'trust', or 'title'")
        }
        let path = "domains/\(urlEscape(domain))/query?type=\(urlEscape(queryType))&param=\(urlEscape(param))"
        return try await requestJSON(method: "GET", path: path, body: nil)
    }

    /// GET /api/node/domains.
    public func getNodeDomains() async throws -> [String: Any] {
        try await requestJSON(method: "GET", path: "node/domains", body: nil)
    }

    /// POST /api/node/domains.
    public func updateNodeDomains(_ domains: [String]) async throws -> [String: Any] {
        try await requestJSON(
            method: "POST", path: "node/domains",
            body: ["managedDomains": domains]
        )
    }

    /// POST /api/gossip/domains.
    public func sendDomainGossip(_ gossip: [String: Any]) async throws -> [String: Any] {
        try await requestJSON(method: "POST", path: "gossip/domains", body: gossip)
    }

    /// POST /api/quids — server-side keygen (trusted mode only).
    public func generateQuid(metadata: [String: Any]? = nil) async throws -> [String: Any] {
        try await requestJSON(method: "POST", path: "quids",
                              body: ["metadata": metadata ?? [:]])
    }

    /// POST /api/node-advertisements.
    public func createNodeAdvertisement(_ advertisement: [String: Any]) async throws -> [String: Any] {
        try await requestJSON(method: "POST", path: "node-advertisements", body: advertisement)
    }

    // =========================================================================
    // Registry queries
    // =========================================================================

    public func queryTrustRegistry() async throws -> [String: Any] {
        try await requestJSON(method: "GET", path: "registry/trust", body: nil)
    }

    public func queryIdentityRegistry() async throws -> [String: Any] {
        try await requestJSON(method: "GET", path: "registry/identity", body: nil)
    }

    public func queryTitleRegistry() async throws -> [String: Any] {
        try await requestJSON(method: "GET", path: "registry/title", body: nil)
    }

    public func queryRelationalTrust(_ query: [String: Any]) async throws -> [String: Any] {
        try await requestJSON(method: "POST", path: "trust/query", body: query)
    }

    // =========================================================================
    // IPFS
    // =========================================================================

    /// POST /api/ipfs/pin — pin raw bytes; returns the CID.
    public func ipfsPin(content: Data) async throws -> String {
        let url = apiBase.appendingPathComponent("ipfs/pin")
        var req = URLRequest(url: url, timeoutInterval: timeout)
        req.httpMethod = "POST"
        req.setValue("application/octet-stream", forHTTPHeaderField: "Content-Type")
        req.setValue(userAgent, forHTTPHeaderField: "User-Agent")
        if let token = authToken {
            req.setValue("Bearer \(token)", forHTTPHeaderField: "Authorization")
        }
        req.httpBody = content
        let (data, response) = try await session.data(for: req)
        guard let http = response as? HTTPURLResponse else {
            throw QuidnugError.node(status: 0, message: "non-HTTP response")
        }
        let env = try parseEnvelope(data: data, statusCode: http.statusCode)
        if let cid = env["cid"] as? String { return cid }
        if let cid = env["value"] as? String { return cid }
        throw QuidnugError.node(status: http.statusCode, message: "ipfs pin response missing cid")
    }

    /// GET /api/ipfs/{cid} — fetch the pinned bytes.
    public func ipfsGet(cid: String) async throws -> Data {
        let url = apiBase.appendingPathComponent("ipfs/\(urlEscape(cid))")
        var req = URLRequest(url: url, timeoutInterval: timeout)
        req.httpMethod = "GET"
        req.setValue(userAgent, forHTTPHeaderField: "User-Agent")
        if let token = authToken {
            req.setValue("Bearer \(token)", forHTTPHeaderField: "Authorization")
        }
        let (data, response) = try await session.data(for: req)
        guard let http = response as? HTTPURLResponse, http.statusCode < 400 else {
            let status = (response as? HTTPURLResponse)?.statusCode ?? 0
            throw QuidnugError.node(status: status, message: "ipfs get failed")
        }
        return data
    }

    // =========================================================================
    // Moderation (QDP-0015)
    // =========================================================================

    public func createModerationAction(_ action: [String: Any]) async throws -> [String: Any] {
        try await requestJSON(method: "POST", path: "moderation/actions", body: action)
    }

    public func getModerationActions(targetType: String, targetId: String) async throws -> [String: Any] {
        try await requestJSON(
            method: "GET",
            path: "moderation/actions/\(urlEscape(targetType))/\(urlEscape(targetId))",
            body: nil
        )
    }

    // =========================================================================
    // Audit (QDP-0018)
    // =========================================================================

    public func auditHead() async throws -> [String: Any] {
        try await requestJSON(method: "GET", path: "audit/head", body: nil)
    }

    public func auditEntries(since: Int64? = nil, limit: Int? = nil) async throws -> [String: Any] {
        var path = "audit/entries"
        var qs: [String] = []
        if let s = since { qs.append("since=\(s)") }
        if let l = limit { qs.append("limit=\(l)") }
        if !qs.isEmpty { path += "?" + qs.joined(separator: "&") }
        return try await requestJSON(method: "GET", path: path, body: nil)
    }

    public func auditEntry(sequence: Int64) async throws -> [String: Any]? {
        do {
            return try await requestJSON(
                method: "GET",
                path: "audit/entry/\(sequence)",
                body: nil
            )
        } catch QuidnugError.validation(let m) where m.contains("NOT_FOUND") {
            return nil
        }
    }

    // =========================================================================
    // Privacy (QDP-0017)
    // =========================================================================

    public func createDSR(_ request: [String: Any]) async throws -> [String: Any] {
        try await requestJSON(method: "POST", path: "privacy/dsr", body: request)
    }

    public func getDSRStatus(requestTxId: String) async throws -> [String: Any]? {
        do {
            return try await requestJSON(
                method: "GET",
                path: "privacy/dsr/\(urlEscape(requestTxId))",
                body: nil
            )
        } catch QuidnugError.validation(let m) where m.contains("NOT_FOUND") {
            return nil
        }
    }

    public func createConsentGrant(_ grant: [String: Any]) async throws -> [String: Any] {
        try await requestJSON(method: "POST", path: "privacy/consent/grants", body: grant)
    }

    public func createConsentWithdraw(_ withdraw: [String: Any]) async throws -> [String: Any] {
        try await requestJSON(method: "POST", path: "privacy/consent/withdraws", body: withdraw)
    }

    public func getConsentHistory(subjectQuid: String) async throws -> [String: Any] {
        try await requestJSON(
            method: "GET",
            path: "privacy/consent/history?subject=\(urlEscape(subjectQuid))",
            body: nil
        )
    }

    public func createProcessingRestriction(_ restriction: [String: Any]) async throws -> [String: Any] {
        try await requestJSON(method: "POST", path: "privacy/restrictions", body: restriction)
    }

    public func getRestrictionsForSubject(subjectQuid: String) async throws -> [String: Any] {
        try await requestJSON(
            method: "GET",
            path: "privacy/restrictions/\(urlEscape(subjectQuid))",
            body: nil
        )
    }

    public func createDSRCompliance(_ compliance: [String: Any]) async throws -> [String: Any] {
        try await requestJSON(method: "POST", path: "privacy/compliance", body: compliance)
    }

    // =========================================================================
    // Discovery (QDP-0014)
    // =========================================================================

    public func discoverDomain(name: String) async throws -> [String: Any] {
        try await requestJSON(method: "GET", path: "v2/discovery/domain/\(urlEscape(name))", body: nil)
    }

    public func discoverNode(quid: String) async throws -> [String: Any] {
        try await requestJSON(method: "GET", path: "v2/discovery/node/\(urlEscape(quid))", body: nil)
    }

    public func discoverOperator(quid: String) async throws -> [String: Any] {
        try await requestJSON(method: "GET", path: "v2/discovery/operator/\(urlEscape(quid))", body: nil)
    }

    public func discoverQuids() async throws -> [String: Any] {
        try await requestJSON(method: "GET", path: "v2/discovery/quids", body: nil)
    }

    public func discoverTrustedQuids() async throws -> [String: Any] {
        try await requestJSON(method: "GET", path: "v2/discovery/trusted-quids", body: nil)
    }

    // =========================================================================
    // DNS attestation (QDP-0023)
    // =========================================================================

    public func submitDNSClaim(_ claim: [String: Any]) async throws -> [String: Any] {
        try await requestJSON(method: "POST", path: "v2/dns/claim", body: claim)
    }

    public func submitDNSChallenge(_ challenge: [String: Any]) async throws -> [String: Any] {
        try await requestJSON(method: "POST", path: "v2/dns/challenge", body: challenge)
    }

    public func submitDNSAttestation(_ attestation: [String: Any]) async throws -> [String: Any] {
        try await requestJSON(method: "POST", path: "v2/dns/attestation", body: attestation)
    }

    public func submitDNSRenewal(_ renewal: [String: Any]) async throws -> [String: Any] {
        try await requestJSON(method: "POST", path: "v2/dns/renewal", body: renewal)
    }

    public func submitDNSRevocation(_ revocation: [String: Any]) async throws -> [String: Any] {
        try await requestJSON(method: "POST", path: "v2/dns/revocation", body: revocation)
    }

    public func submitAuthorityDelegate(_ delegate: [String: Any]) async throws -> [String: Any] {
        try await requestJSON(method: "POST", path: "v2/dns/delegate", body: delegate)
    }

    public func submitAuthorityDelegateRevocation(_ revocation: [String: Any]) async throws -> [String: Any] {
        try await requestJSON(method: "POST", path: "v2/dns/delegate-revocation", body: revocation)
    }

    public func getDNSAttestations(domain: String) async throws -> [String: Any] {
        try await requestJSON(method: "GET", path: "v2/dns/attestations/\(urlEscape(domain))", body: nil)
    }

    public func getDNSAttestationsWeighted(domain: String) async throws -> [String: Any] {
        try await requestJSON(method: "GET", path: "v2/dns/attestations/\(urlEscape(domain))/weighted", body: nil)
    }

    public func resolveDNSRecord(domain: String, recordType: String) async throws -> [String: Any] {
        try await requestJSON(
            method: "GET",
            path: "v2/dns/resolve/\(urlEscape(domain))/\(urlEscape(recordType))",
            body: nil
        )
    }

    // =========================================================================
    // HTTP plumbing
    // =========================================================================

    private func requestJSON(
        method: String, path: String, body: Any?
    ) async throws -> [String: Any] {
        let retry = method == "GET"
        let attempts = retry ? maxRetries + 1 : 1
        var lastError: Error? = nil

        for attempt in 0..<attempts {
            let url = apiBase.appendingPathComponent(path.hasPrefix("/") ? String(path.dropFirst()) : path)
            let u = URLComponents(url: url, resolvingAgainstBaseURL: false)
            let finalURL = u?.url ?? url

            var req = URLRequest(url: finalURL, timeoutInterval: timeout)
            req.httpMethod = method
            req.setValue("application/json", forHTTPHeaderField: "Accept")
            req.setValue(userAgent, forHTTPHeaderField: "User-Agent")
            if let token = authToken {
                req.setValue("Bearer \(token)", forHTTPHeaderField: "Authorization")
            }
            if let b = body {
                req.setValue("application/json", forHTTPHeaderField: "Content-Type")
                req.httpBody = try JSONSerialization.data(withJSONObject: b, options: [])
            }

            let data: Data
            let response: URLResponse
            do {
                (data, response) = try await session.data(for: req)
            } catch {
                lastError = error
                if attempt < attempts - 1 { await sleepBackoff(attempt: attempt); continue }
                throw QuidnugError.node(status: 0, message: "network: \(error)")
            }
            guard let http = response as? HTTPURLResponse else {
                throw QuidnugError.node(status: 0, message: "non-HTTP response")
            }
            let sc = http.statusCode
            if (sc >= 500 || sc == 429) && attempt < attempts - 1 {
                await sleepBackoff(attempt: attempt)
                continue
            }
            return try parseEnvelope(data: data, statusCode: sc)
        }
        throw (lastError.map { QuidnugError.node(status: 0, message: "\($0)") }
               ?? QuidnugError.node(status: 0, message: "retries exhausted"))
    }

    private func parseEnvelope(data: Data, statusCode: Int) throws -> [String: Any] {
        let raw: Any
        do {
            raw = try JSONSerialization.jsonObject(with: data, options: [])
        } catch {
            throw QuidnugError.node(status: statusCode, message: "non-JSON response")
        }
        guard let env = raw as? [String: Any] else {
            throw QuidnugError.node(status: statusCode, message: "envelope is not an object")
        }
        if (env["success"] as? Bool) == true {
            if let dict = env["data"] as? [String: Any] { return dict }
            if env["data"] is NSNull { return [:] }
            // data might be scalar / array — wrap.
            return ["value": env["data"] as Any]
        }
        let err = env["error"] as? [String: Any] ?? [:]
        let code = (err["code"] as? String) ?? "UNKNOWN_ERROR"
        let message = (err["message"] as? String) ?? "HTTP \(statusCode)"
        if statusCode == 503 || Self.unavailableCodes.contains(code) {
            throw QuidnugError.unavailable(code: code, message: message)
        }
        if statusCode == 409 || Self.conflictCodes.contains(code) {
            throw QuidnugError.conflict(code: code, message: message)
        }
        if (400..<500).contains(statusCode) {
            // Encode code in message so callers can detect NOT_FOUND via the text.
            throw QuidnugError.validation("\(code): \(message)")
        }
        throw QuidnugError.node(status: statusCode, message: message)
    }

    private func sleepBackoff(attempt: Int) async {
        let delay = min(retryBaseDelay * pow(2.0, Double(attempt)) + Double.random(in: 0...0.1), 60)
        try? await Task.sleep(nanoseconds: UInt64(delay * 1_000_000_000))
    }

    private func signIntoTx(signer: Quid, tx: inout [String: Any]) throws {
        let bytes = try CanonicalBytes.of(tx, excludeFields: ["signature", "txId"])
        tx["signature"] = try signer.sign(bytes)
    }

    private func requireSigner(_ signer: Quid) throws {
        if !signer.hasPrivateKey {
            throw QuidnugError.validation("signer must have a private key")
        }
    }

    private func decode<T: Decodable>(_ type: T.Type, from obj: [String: Any]) throws -> T {
        let data = try JSONSerialization.data(withJSONObject: obj, options: [])
        return try JSONDecoder().decode(T.self, from: data)
    }

    private func urlEscape(_ s: String) -> String {
        s.addingPercentEncoding(withAllowedCharacters: .urlPathAllowed)
            ?? s
    }
}
