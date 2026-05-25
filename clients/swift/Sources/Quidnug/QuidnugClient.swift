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

    /// GET /api/registry/identity — paginated identity registry dump.
    public func queryIdentityRegistry(
        domain: String? = nil, limit: Int? = nil, offset: Int? = nil
    ) async throws -> [String: Any] {
        var path = "registry/identity"
        let q = buildQuery([
            "domain": domain,
            "limit": limit.map { String($0) },
            "offset": offset.map { String($0) },
        ])
        if !q.isEmpty { path += "?" + q }
        return try await requestJSON(method: "GET", path: path, body: nil)
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

    /// POST /api/trust/query — structured relational trust query.
    public func queryRelationalTrust(
        observer: String,
        target: String,
        domain: String? = nil,
        maxDepth: Int? = nil,
        includeUnverified: Bool = false
    ) async throws -> TrustResult {
        guard !observer.isEmpty, !target.isEmpty else {
            throw QuidnugError.validation("observer and target are required")
        }
        var body: [String: Any] = [
            "observer": observer,
            "target": target,
            "includeUnverified": includeUnverified,
        ]
        if let d = domain { body["domain"] = d }
        if let m = maxDepth { body["maxDepth"] = m }
        let raw = try await requestJSON(method: "POST", path: "trust/query", body: body)
        return try decode(TrustResult.self, from: raw)
    }

    /// GET /api/registry/trust — paginated trust-edge registry.
    public func queryTrustRegistry(
        domain: String? = nil, limit: Int? = nil, offset: Int? = nil
    ) async throws -> [String: Any] {
        var path = "registry/trust"
        let q = buildQuery([
            "domain": domain,
            "limit": limit.map { String($0) },
            "offset": offset.map { String($0) },
        ])
        if !q.isEmpty { path += "?" + q }
        return try await requestJSON(method: "GET", path: path, body: nil)
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

    /// GET /api/registry/title — paginated title registry.
    public func queryTitleRegistry(
        domain: String? = nil, limit: Int? = nil, offset: Int? = nil
    ) async throws -> [String: Any] {
        var path = "registry/title"
        let q = buildQuery([
            "domain": domain,
            "limit": limit.map { String($0) },
            "offset": offset.map { String($0) },
        ])
        if !q.isEmpty { path += "?" + q }
        return try await requestJSON(method: "GET", path: path, body: nil)
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
    // IPFS / large-payload storage
    // =========================================================================

    /// POST /api/ipfs/pin — pin raw bytes and return the resulting CID.
    public func ipfsPin(content: Data) async throws -> String {
        let (raw, status) = try await requestRaw(
            method: "POST",
            path: "ipfs/pin",
            body: content,
            contentType: "application/octet-stream",
            envelope: true
        )
        let env = try parseEnvelope(data: raw, statusCode: status)
        if let cid = env["cid"] as? String { return cid }
        if let v = env["value"] as? String { return v }
        throw QuidnugError.node(status: status, message: "IPFS pin response missing cid")
    }

    /// GET /api/ipfs/{cid} — fetch raw pinned bytes.
    public func ipfsGet(cid: String) async throws -> Data {
        let (data, _) = try await requestRaw(
            method: "GET",
            path: "ipfs/\(urlEscape(cid))",
            body: nil,
            contentType: nil,
            envelope: false
        )
        return data
    }

    // =========================================================================
    // Guardians / gossip / fork-block / bootstrap
    // =========================================================================

    public func submitGuardianSetUpdate(_ update: [String: Any]) async throws -> [String: Any] {
        try await requestJSON(method: "POST", path: "guardian/set-update", body: update)
    }

    /// POST /api/guardian/recovery/init — start the M-of-N recovery delay.
    public func submitRecoveryInit(_ `init`: [String: Any]) async throws -> [String: Any] {
        try await requestJSON(method: "POST", path: "guardian/recovery/init", body: `init`)
    }

    /// POST /api/guardian/recovery/veto — abort a pending recovery.
    public func submitRecoveryVeto(_ veto: [String: Any]) async throws -> [String: Any] {
        try await requestJSON(method: "POST", path: "guardian/recovery/veto", body: veto)
    }

    /// POST /api/guardian/recovery/commit — finalize a delayed recovery.
    public func submitRecoveryCommit(_ commit: [String: Any]) async throws -> [String: Any] {
        try await requestJSON(method: "POST", path: "guardian/recovery/commit", body: commit)
    }

    /// POST /api/guardian/resign — guardian leaves the set.
    public func submitGuardianResignation(_ r: [String: Any]) async throws -> [String: Any] {
        try await requestJSON(method: "POST", path: "guardian/resign", body: r)
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

    /// GET /api/guardian/pending-recovery/{quidId} — pending recovery for an account.
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

    /// GET /api/guardian/resignations/{quidId} — guardian resignations for an account.
    public func getGuardianResignations(quidId: String) async throws -> [[String: Any]] {
        let raw = try await requestJSON(
            method: "GET",
            path: "guardian/resignations/\(urlEscape(quidId))",
            body: nil
        )
        if let arr = raw["data"] as? [[String: Any]] { return arr }
        if let arr = raw["resignations"] as? [[String: Any]] { return arr }
        if let arr = raw["value"] as? [[String: Any]] { return arr }
        return []
    }

    /// POST /api/domain-fingerprints — publish a signed fingerprint.
    public func submitDomainFingerprint(_ fp: [String: Any]) async throws -> [String: Any] {
        try await requestJSON(method: "POST", path: "domain-fingerprints", body: fp)
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

    /// POST /api/anchor-gossip — deliver a cross-domain anchor message.
    /// Idempotent on the server: re-delivery of an already-accepted
    /// `messageId` returns 200 with `data.duplicate=true`.
    public func submitAnchorGossip(_ msg: [String: Any]) async throws -> [String: Any] {
        try await requestJSON(method: "POST", path: "anchor-gossip", body: msg)
    }

    /// POST /api/gossip/push-anchor — push-gossip variant (QDP-0005).
    public func pushAnchor(_ msg: [String: Any]) async throws -> [String: Any] {
        try await requestJSON(method: "POST", path: "gossip/push-anchor", body: msg)
    }

    /// POST /api/gossip/push-fingerprint — push-gossip variant (QDP-0005).
    public func pushFingerprint(_ fp: [String: Any]) async throws -> [String: Any] {
        try await requestJSON(method: "POST", path: "gossip/push-fingerprint", body: fp)
    }

    /// POST /api/nonce-snapshots — publish a K-of-K bootstrap snapshot (QDP-0008).
    public func submitNonceSnapshot(_ s: [String: Any]) async throws -> [String: Any] {
        try await requestJSON(method: "POST", path: "nonce-snapshots", body: s)
    }

    /// GET /api/nonce-snapshots/{domain}/latest — latest signed snapshot for a domain.
    public func getLatestNonceSnapshot(domain: String) async throws -> NonceSnapshot? {
        do {
            return try decode(
                NonceSnapshot.self,
                from: await requestJSON(
                    method: "GET",
                    path: "nonce-snapshots/\(urlEscape(domain))/latest",
                    body: nil
                )
            )
        } catch QuidnugError.validation(let m) where m.contains("NOT_FOUND") {
            return nil
        }
    }

    public func bootstrapStatus() async throws -> [String: Any] {
        try await requestJSON(method: "GET", path: "bootstrap/status", body: nil)
    }

    /// POST /api/fork-block — submit a signed fork-activation block (QDP-0009).
    public func submitForkBlock(_ fb: [String: Any]) async throws -> [String: Any] {
        try await requestJSON(method: "POST", path: "fork-block", body: fb)
    }

    public func forkBlockStatus() async throws -> [String: Any] {
        try await requestJSON(method: "GET", path: "fork-block/status", body: nil)
    }

    // =========================================================================
    // Blocks + pending transactions
    // =========================================================================

    /// GET /api/blocks — recent committed blocks (paginated).
    public func getBlocks(limit: Int? = nil, offset: Int? = nil) async throws -> [String: Any] {
        var path = "blocks"
        let q = buildQuery([
            "limit": limit.map { String($0) },
            "offset": offset.map { String($0) },
        ])
        if !q.isEmpty { path += "?" + q }
        return try await requestJSON(method: "GET", path: path, body: nil)
    }

    /// GET /api/blocks/tentative/{domain} — uncommitted blocks for a domain.
    public func getTentativeBlocks(domain: String) async throws -> [String: Any] {
        try await requestJSON(
            method: "GET",
            path: "blocks/tentative/\(urlEscape(domain))",
            body: nil
        )
    }

    /// GET /api/transactions — pending mempool transactions.
    public func getPendingTransactions(
        domain: String? = nil, limit: Int? = nil, offset: Int? = nil
    ) async throws -> [String: Any] {
        var path = "transactions"
        let q = buildQuery([
            "domain": domain,
            "limit": limit.map { String($0) },
            "offset": offset.map { String($0) },
        ])
        if !q.isEmpty { path += "?" + q }
        return try await requestJSON(method: "GET", path: path, body: nil)
    }

    // =========================================================================
    // Domains
    // =========================================================================

    /// GET /api/domains — list all trust domains known to the node.
    public func listDomains() async throws -> [String: Any] {
        try await requestJSON(method: "GET", path: "domains", body: nil)
    }

    /// POST /api/domains — register a new trust domain.
    public func registerDomain(_ domain: String) async throws -> [String: Any] {
        guard !domain.isEmpty else { throw QuidnugError.validation("domain is required") }
        return try await requestJSON(
            method: "POST",
            path: "domains",
            body: ["name": domain]
        )
    }

    /// Idempotent wrapper around `registerDomain` — returns success even when
    /// the domain already exists. Useful in demo / bootstrap scripts.
    public func ensureDomain(_ domain: String) async throws -> [String: Any] {
        do {
            return try await registerDomain(domain)
        } catch QuidnugError.validation(let m) where m.lowercased().contains("already exists") {
            return [
                "status": "success",
                "domain": domain,
                "message": "trust domain already exists",
            ]
        } catch QuidnugError.conflict(_, let m) where m.lowercased().contains("already exists") {
            return [
                "status": "success",
                "domain": domain,
                "message": "trust domain already exists",
            ]
        }
    }

    /// GET /api/node/domains — domains the node is configured to manage.
    public func getNodeDomains() async throws -> [String: Any] {
        try await requestJSON(method: "GET", path: "node/domains", body: nil)
    }

    /// POST /api/node/domains — update the set of domains this node manages.
    public func updateNodeDomains(_ domains: [String]) async throws -> [String: Any] {
        try await requestJSON(
            method: "POST",
            path: "node/domains",
            body: ["managedDomains": domains]
        )
    }

    // =========================================================================
    // Commit-wait helpers
    // =========================================================================

    /// Poll `getIdentity` until the record commits or the timeout elapses.
    ///
    /// A just-submitted IDENTITY transaction lives in the pending pool
    /// until the next block is sealed. Code that immediately emits
    /// events or title transactions referencing the new quid must
    /// wait for commit first.
    @discardableResult
    public func waitForIdentity(
        quidId: String,
        domain: String? = nil,
        timeout: TimeInterval = 30,
        pollInterval: TimeInterval = 0.5
    ) async throws -> IdentityRecord {
        let deadline = Date().addingTimeInterval(timeout)
        while Date() < deadline {
            if let rec = try await getIdentity(quidId: quidId, domain: domain) {
                return rec
            }
            try await Task.sleep(nanoseconds: UInt64(pollInterval * 1_000_000_000))
        }
        throw QuidnugError.unavailable(
            code: "TIMEOUT",
            message: "identity \(quidId) did not commit within \(timeout)s"
        )
    }

    /// Block until every quid id in `quidIds` is committed, sharing a
    /// single deadline across all waits.
    @discardableResult
    public func waitForIdentities(
        quidIds: [String],
        domain: String? = nil,
        timeout: TimeInterval = 30
    ) async throws -> [IdentityRecord] {
        let deadline = Date().addingTimeInterval(timeout)
        var results: [IdentityRecord] = []
        results.reserveCapacity(quidIds.count)
        for qid in quidIds {
            let remaining = max(0, deadline.timeIntervalSinceNow)
            if remaining <= 0 {
                throw QuidnugError.unavailable(
                    code: "TIMEOUT",
                    message: "identities not all committed within \(timeout)s (blocked on \(qid))"
                )
            }
            let rec = try await waitForIdentity(
                quidId: qid,
                domain: domain,
                timeout: remaining
            )
            results.append(rec)
        }
        return results
    }

    /// Poll `getTitle` until the record commits or the timeout elapses.
    @discardableResult
    public func waitForTitle(
        assetId: String,
        domain: String? = nil,
        timeout: TimeInterval = 30,
        pollInterval: TimeInterval = 0.5
    ) async throws -> Title {
        let deadline = Date().addingTimeInterval(timeout)
        while Date() < deadline {
            if let rec = try await getTitle(assetId: assetId, domain: domain) {
                return rec
            }
            try await Task.sleep(nanoseconds: UInt64(pollInterval * 1_000_000_000))
        }
        throw QuidnugError.unavailable(
            code: "TIMEOUT",
            message: "title \(assetId) did not commit within \(timeout)s"
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

    /// Build a URL-encoded query string from a `[name: value?]` map. Nil
    /// values are dropped (matches Go's omitempty / Python's `_strip_none`).
    private func buildQuery(_ params: [String: String?]) -> String {
        var pairs: [String] = []
        // Sort for stable ordering — easier on tests + caches.
        for key in params.keys.sorted() {
            guard let value = params[key], let v = value else { continue }
            let ek = key.addingPercentEncoding(withAllowedCharacters: .urlQueryAllowed) ?? key
            let ev = v.addingPercentEncoding(withAllowedCharacters: .urlQueryAllowed) ?? v
            pairs.append("\(ek)=\(ev)")
        }
        return pairs.joined(separator: "&")
    }

    /// Issue a request that bypasses JSON body encoding — used for raw
    /// payloads (IPFS pin) and raw response bodies (IPFS fetch).
    ///
    /// - Parameter envelope: if `true` the caller will parse the returned
    ///   bytes as the standard `{success,data,error}` JSON envelope; if
    ///   `false`, the raw response body is returned unchanged.
    private func requestRaw(
        method: String,
        path: String,
        body: Data?,
        contentType: String?,
        envelope: Bool = true
    ) async throws -> (Data, Int) {
        let url = apiBase.appendingPathComponent(path.hasPrefix("/") ? String(path.dropFirst()) : path)
        let u = URLComponents(url: url, resolvingAgainstBaseURL: false)
        let finalURL = u?.url ?? url

        var req = URLRequest(url: finalURL, timeoutInterval: timeout)
        req.httpMethod = method
        req.setValue(userAgent, forHTTPHeaderField: "User-Agent")
        if let token = authToken {
            req.setValue("Bearer \(token)", forHTTPHeaderField: "Authorization")
        }
        if envelope {
            req.setValue("application/json", forHTTPHeaderField: "Accept")
        }
        if let b = body {
            req.httpBody = b
            if let ct = contentType {
                req.setValue(ct, forHTTPHeaderField: "Content-Type")
            }
        }

        let data: Data
        let response: URLResponse
        do {
            (data, response) = try await session.data(for: req)
        } catch {
            throw QuidnugError.node(status: 0, message: "network: \(error)")
        }
        guard let http = response as? HTTPURLResponse else {
            throw QuidnugError.node(status: 0, message: "non-HTTP response")
        }
        if !envelope && http.statusCode >= 400 {
            throw QuidnugError.node(
                status: http.statusCode,
                message: "raw fetch failed (HTTP \(http.statusCode))"
            )
        }
        return (data, http.statusCode)
    }
}
