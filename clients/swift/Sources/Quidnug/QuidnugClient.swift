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

    /// Lists known peers. Pagination applies.
    public func nodes(limit: Int? = nil, offset: Int? = nil) async throws -> [String: Any] {
        try await requestJSON(method: "GET",
                              path: appendPagination("nodes", limit: limit, offset: offset),
                              body: nil)
    }

    /// Performs a GET to `path` (without the `/api` prefix; client adds it)
    /// and returns the raw response body. Useful for ad-hoc CLI commands
    /// that don't yet have a typed wrapper. The returned bytes are the
    /// full JSON envelope — callers parse it themselves.
    public func rawGet(_ path: String) async throws -> Data {
        let trimmed = path.hasPrefix("/") ? String(path.dropFirst()) : path
        let url = apiBase.appendingPathComponent(trimmed)
        var req = URLRequest(url: url, timeoutInterval: timeout)
        req.httpMethod = "GET"
        req.setValue("application/json", forHTTPHeaderField: "Accept")
        req.setValue(userAgent, forHTTPHeaderField: "User-Agent")
        if let token = authToken {
            req.setValue("Bearer \(token)", forHTTPHeaderField: "Authorization")
        }
        let (data, response): (Data, URLResponse)
        do {
            (data, response) = try await session.data(for: req)
        } catch {
            throw QuidnugError.node(status: 0, message: "network: \(error)")
        }
        guard let http = response as? HTTPURLResponse else {
            throw QuidnugError.node(status: 0, message: "non-HTTP response")
        }
        if !(200..<300).contains(http.statusCode) {
            throw QuidnugError.node(status: http.statusCode, message: "status \(http.statusCode)")
        }
        return data
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

    // =========================================================================
    // QDP-0014 Discovery
    // =========================================================================

    /// Returns the current consortium, endpoint hints, and block tip for a domain.
    public func discoverDomain(_ domain: String) async throws -> [String: Any] {
        guard !domain.isEmpty else { throw QuidnugError.validation("domain is required") }
        return try await requestJSON(method: "GET",
                                     path: "v2/discovery/domain/\(urlEscape(domain))",
                                     body: nil)
    }

    /// Returns the raw signed advertisement for a node quid.
    public func discoverNode(_ quid: String) async throws -> [String: Any] {
        guard !quid.isEmpty else { throw QuidnugError.validation("quid is required") }
        return try await requestJSON(method: "GET",
                                     path: "v2/discovery/node/\(urlEscape(quid))",
                                     body: nil)
    }

    /// Lists all advertisements for a given operator quid.
    public func discoverOperator(_ operatorQuid: String) async throws -> [String: Any] {
        guard !operatorQuid.isEmpty else {
            throw QuidnugError.validation("operatorQuid is required")
        }
        return try await requestJSON(method: "GET",
                                     path: "v2/discovery/operator/\(urlEscape(operatorQuid))",
                                     body: nil)
    }

    /// Queries the per-domain quid index.
    public func discoverQuids(_ p: DiscoverQuidsParams) async throws -> [String: Any] {
        guard !p.domain.isEmpty else { throw QuidnugError.validation("domain is required") }
        var items: [(String, String)] = [("domain", p.domain)]
        if let v = p.since, v > 0 { items.append(("since", String(v))) }
        if let s = p.sort, !s.isEmpty { items.append(("sort", s)) }
        if let o = p.observer, !o.isEmpty { items.append(("observer", o)) }
        if let e = p.eventType, !e.isEmpty { items.append(("eventType", e)) }
        if let m = p.minTrustWeight, m > 0 {
            items.append(("min-trust-weight", formatFloat(m)))
        }
        if !p.excludeQuids.isEmpty {
            items.append(("excludeQuid", p.excludeQuids.joined(separator: ",")))
        }
        if let l = p.limit, l > 0 { items.append(("limit", String(l))) }
        if let o = p.offset, o > 0 { items.append(("offset", String(o))) }
        let path = "v2/discovery/quids" + encodeQuery(items)
        return try await requestJSON(method: "GET", path: path, body: nil)
    }

    /// Returns quids the consortium members have directly TRUSTed above
    /// the given threshold.
    public func discoverTrustedQuids(
        domain: String, minTrust: Double? = nil, limit: Int? = nil
    ) async throws -> [String: Any] {
        guard !domain.isEmpty else { throw QuidnugError.validation("domain is required") }
        var items: [(String, String)] = [("domain", domain)]
        if let m = minTrust, m > 0 { items.append(("min-trust", formatFloat(m))) }
        if let l = limit, l > 0 { items.append(("limit", String(l))) }
        return try await requestJSON(method: "GET",
                                     path: "v2/discovery/trusted-quids" + encodeQuery(items),
                                     body: nil)
    }

    // =========================================================================
    // QDP-0014: Node advertisement (signed)
    // =========================================================================

    /// Builds, signs, and submits a QDP-0014 NodeAdvertisementTransaction.
    ///
    /// NOTE: This method is currently a stub — the Swift SDK does not yet
    /// implement the field-order-sensitive `json.Marshal` of a typed
    /// struct that the Go SDK uses to derive byte-identical signable
    /// bytes. Reproducing it here requires either a custom encoder that
    /// preserves declaration order or a hand-rolled JSON writer. Both
    /// are tracked under a follow-up SDK ticket. Until then this method
    /// throws `QuidnugError.unsupported` so callers fail loudly rather
    /// than producing an invalid signature.
    public func publishNodeAdvertisement(
        signer: Quid, params: NodeAdvertisementParams
    ) async throws -> [String: Any] {
        // TODO(QDP-0014 swift): implement field-order-deterministic
        // canonicalization for NodeAdvertisementTransaction. Server
        // verifies the signature against json.Marshal of a Go struct,
        // so this SDK must produce byte-identical JSON.
        _ = signer; _ = params
        throw QuidnugError.unsupported(
            "publishNodeAdvertisement: pending field-order-deterministic encoder")
    }

    // =========================================================================
    // Guardian recovery (QDP-0006)
    // =========================================================================

    /// Starts the delayed recovery flow.
    public func submitRecoveryInit(_ initBody: [String: Any]) async throws -> [String: Any] {
        try await requestJSON(method: "POST", path: "guardian/recovery/init", body: initBody)
    }

    /// Aborts an in-flight recovery.
    public func submitRecoveryVeto(_ veto: [String: Any]) async throws -> [String: Any] {
        try await requestJSON(method: "POST", path: "guardian/recovery/veto", body: veto)
    }

    /// Finalizes a recovery after the delay.
    public func submitRecoveryCommit(_ commit: [String: Any]) async throws -> [String: Any] {
        try await requestJSON(method: "POST", path: "guardian/recovery/commit", body: commit)
    }

    /// Removes a guardian from the set.
    public func submitGuardianResignation(_ resignation: [String: Any]) async throws -> [String: Any] {
        try await requestJSON(method: "POST", path: "guardian/resign", body: resignation)
    }

    /// Returns the pending recovery record or nil if none is active.
    public func getPendingRecovery(_ quid: String) async throws -> [String: Any]? {
        do {
            return try await requestJSON(method: "GET",
                                         path: "guardian/pending-recovery/\(urlEscape(quid))",
                                         body: nil)
        } catch QuidnugError.validation(let m) where m.contains("NOT_FOUND") {
            return nil
        }
    }

    // =========================================================================
    // Gossip / fingerprints (QDP-0003, QDP-0005)
    // =========================================================================

    /// Publishes a signed domain fingerprint.
    public func submitDomainFingerprint(_ fp: [String: Any]) async throws -> [String: Any] {
        try await requestJSON(method: "POST", path: "domain-fingerprints", body: fp)
    }

    /// Delivers a cross-domain anchor gossip message.
    public func submitAnchorGossip(_ message: [String: Any]) async throws -> [String: Any] {
        try await requestJSON(method: "POST", path: "anchor-gossip", body: message)
    }

    /// Push-mode variant of anchor gossip (QDP-0005).
    public func pushAnchor(_ message: [String: Any]) async throws -> [String: Any] {
        try await requestJSON(method: "POST", path: "gossip/push-anchor", body: message)
    }

    /// Push-mode variant of fingerprint gossip.
    public func pushFingerprint(_ fingerprint: [String: Any]) async throws -> [String: Any] {
        try await requestJSON(method: "POST", path: "gossip/push-fingerprint", body: fingerprint)
    }

    // =========================================================================
    // Bootstrap (QDP-0008)
    // =========================================================================

    /// Publishes a K-of-K bootstrap nonce snapshot.
    public func submitNonceSnapshot(_ snapshot: [String: Any]) async throws -> [String: Any] {
        try await requestJSON(method: "POST", path: "nonce-snapshots", body: snapshot)
    }

    /// Returns the most recent nonce snapshot for a domain or nil on 404.
    public func getLatestNonceSnapshot(_ domain: String) async throws -> [String: Any]? {
        do {
            return try await requestJSON(
                method: "GET",
                path: "nonce-snapshots/\(urlEscape(domain))/latest",
                body: nil)
        } catch QuidnugError.validation(let m) where m.contains("NOT_FOUND") {
            return nil
        }
    }

    // =========================================================================
    // Fork-block (QDP-0009)
    // =========================================================================

    /// Submits a signed fork-activation block.
    public func submitForkBlock(_ forkBlock: [String: Any]) async throws -> [String: Any] {
        try await requestJSON(method: "POST", path: "fork-block", body: forkBlock)
    }

    // =========================================================================
    // Blocks, transactions, domains
    // =========================================================================

    /// Returns paginated blocks.
    public func getBlocks(limit: Int? = nil, offset: Int? = nil) async throws -> [String: Any] {
        try await requestJSON(method: "GET",
                              path: appendPagination("blocks", limit: limit, offset: offset),
                              body: nil)
    }

    /// Returns paginated pending transactions.
    public func getPendingTransactions(limit: Int? = nil, offset: Int? = nil) async throws -> [String: Any] {
        try await requestJSON(method: "GET",
                              path: appendPagination("transactions", limit: limit, offset: offset),
                              body: nil)
    }

    /// Returns all known domains.
    public func listDomains() async throws -> [String: Any] {
        try await requestJSON(method: "GET", path: "domains", body: nil)
    }

    /// Registers a new trust domain. Fails with an "already exists" error
    /// if the domain is already known; see `ensureDomain` for an idempotent
    /// variant. Extra attributes are merged into the POST body alongside
    /// `{"name": domain}`.
    public func registerDomain(_ domain: String, attrs: [String: Any] = [:]) async throws -> [String: Any] {
        var body: [String: Any] = ["name": domain]
        for (k, v) in attrs { body[k] = v }
        return try await requestJSON(method: "POST", path: "domains", body: body)
    }

    /// Registers a trust domain if it does not already exist. Idempotent —
    /// calling it twice is cheap and does not error.
    public func ensureDomain(_ domain: String, attrs: [String: Any] = [:]) async throws -> [String: Any] {
        do {
            return try await registerDomain(domain, attrs: attrs)
        } catch let err {
            let msg: String
            switch err {
            case QuidnugError.validation(let m): msg = m
            case QuidnugError.conflict(_, let m): msg = m
            default: throw err
            }
            if msg.lowercased().contains("already exists") {
                return [
                    "status": "success",
                    "domain": domain,
                    "message": "trust domain already exists",
                ]
            }
            throw err
        }
    }

    // =========================================================================
    // Wait helpers
    // =========================================================================

    /// Blocks until the identity with the given quid ID is visible in the
    /// committed registry. Respects task cancellation via `Task.sleep`.
    public func waitForIdentity(
        _ quidId: String,
        domain: String? = nil,
        pollInterval: TimeInterval = 0.5
    ) async throws -> IdentityRecord {
        let interval = pollInterval > 0 ? pollInterval : 0.5
        while true {
            try Task.checkCancellation()
            if let rec = try await getIdentity(quidId: quidId, domain: domain) {
                return rec
            }
            try await Task.sleep(nanoseconds: UInt64(interval * 1_000_000_000))
        }
    }

    /// Blocks until every listed quid ID is committed.
    public func waitForIdentities(
        _ quidIds: [String],
        domain: String? = nil,
        pollInterval: TimeInterval = 0.5
    ) async throws {
        for id in quidIds {
            do {
                _ = try await waitForIdentity(id, domain: domain, pollInterval: pollInterval)
            } catch {
                throw QuidnugError.validation("wait for identity \(id): \(error)")
            }
        }
    }

    /// Blocks until the title with the given asset ID is visible in the
    /// committed registry. Respects task cancellation via `Task.sleep`.
    public func waitForTitle(
        _ assetId: String,
        domain: String? = nil,
        pollInterval: TimeInterval = 0.5
    ) async throws -> Title {
        let interval = pollInterval > 0 ? pollInterval : 0.5
        while true {
            try Task.checkCancellation()
            if let t = try await getTitle(assetId: assetId, domain: domain) {
                return t
            }
            try await Task.sleep(nanoseconds: UInt64(interval * 1_000_000_000))
        }
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

    private func urlQueryEscape(_ s: String) -> String {
        s.addingPercentEncoding(withAllowedCharacters: .urlQueryAllowed) ?? s
    }

    /// Builds a `?limit=…&offset=…` suffix; returns `path` unchanged if both
    /// are nil/non-positive.
    private func appendPagination(_ path: String, limit: Int?, offset: Int?) -> String {
        var items: [(String, String)] = []
        if let l = limit, l > 0 { items.append(("limit", String(l))) }
        if let o = offset, o > 0 { items.append(("offset", String(o))) }
        return path + encodeQuery(items)
    }

    /// Encodes an ordered list of query items into `?k=v&…` form, returning
    /// the empty string when there are no items. Order is preserved so this
    /// matches the Go SDK's url.Values.Encode (lexicographic) only when the
    /// caller supplies items in the desired order — present call sites do.
    private func encodeQuery(_ items: [(String, String)]) -> String {
        guard !items.isEmpty else { return "" }
        let parts = items.map { "\(urlQueryEscape($0.0))=\(urlQueryEscape($0.1))" }
        return "?" + parts.joined(separator: "&")
    }

    /// Formats a Double using Go's strconv.FormatFloat(f, 'f', -1, 64)
    /// shape — shortest decimal that round-trips. Swift's default
    /// `String(_:)` for Double already produces this form.
    private func formatFloat(_ v: Double) -> String {
        String(v)
    }
}
