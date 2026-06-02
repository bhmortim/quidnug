import Foundation

/// Relational trust query result.
public struct TrustResult: Codable, Sendable {
    public let observer: String
    public let target: String
    public let trustLevel: Double
    public let trustPath: [String]?
    public let pathDepth: Int
    public let domain: String

    public var path: [String] { trustPath ?? [] }

    enum CodingKeys: String, CodingKey {
        case observer, target, trustLevel, trustPath, pathDepth, domain
    }
}

/// Direct outbound trust edge.
public struct TrustEdge: Codable, Sendable {
    public let truster: String
    public let trustee: String
    public let trustLevel: Double
    public let domain: String
    public let nonce: Int64
}

/// Identity record snapshot.
public struct IdentityRecord: Codable, Sendable {
    public let quidId: String
    public let name: String?
    public let homeDomain: String?
    public let publicKey: String?
    public let updateNonce: Int64
}

/// Ownership stake.
public struct OwnershipStake: Codable, Sendable {
    public let ownerId: String
    public let percentage: Double
    public let stakeType: String?

    public init(ownerId: String, percentage: Double, stakeType: String? = nil) {
        self.ownerId = ownerId
        self.percentage = percentage
        self.stakeType = stakeType
    }
}

/// Title record.
public struct Title: Codable, Sendable {
    public let assetId: String
    public let domain: String?
    public let titleType: String?
    public let ownershipMap: [OwnershipStake]?
}

/// Event-stream row.
public struct Event: Codable, Sendable {
    public let subjectId: String
    public let subjectType: String
    public let eventType: String
    public let payloadCid: String?
    public let timestamp: Int64
    public let sequence: Int64
}

/// Guardian entry.
public struct GuardianRef: Codable, Sendable {
    public let quid: String
    public let weight: Int
    public let epoch: Int
}

/// Guardian set.
public struct GuardianSet: Codable, Sendable {
    public let subjectQuid: String
    public let guardians: [GuardianRef]?
    public let threshold: Int
    public let recoveryDelaySeconds: Int64
}

/// Domain fingerprint.
public struct DomainFingerprint: Codable, Sendable {
    public let domain: String
    public let blockHeight: Int64
    public let blockHash: String
    public let producerQuid: String
    public let timestamp: Int64
}

// MARK: - QDP-0014 Discovery

/// Parameters for `discoverQuids`. All fields optional except `domain`.
public struct DiscoverQuidsParams: Sendable {
    public var domain: String
    /// UnixNano lower bound on event time.
    public var since: Int64?
    /// One of "activity" | "last-seen" | "first-seen" | "trust-weight".
    public var sort: String?
    /// Enables trust-weight sort and populates trustWeight in results.
    public var observer: String?
    public var eventType: String?
    public var minTrustWeight: Double?
    public var excludeQuids: [String]
    /// Default 50, max 500 (server-enforced).
    public var limit: Int?
    public var offset: Int?

    public init(
        domain: String,
        since: Int64? = nil,
        sort: String? = nil,
        observer: String? = nil,
        eventType: String? = nil,
        minTrustWeight: Double? = nil,
        excludeQuids: [String] = [],
        limit: Int? = nil,
        offset: Int? = nil
    ) {
        self.domain = domain
        self.since = since
        self.sort = sort
        self.observer = observer
        self.eventType = eventType
        self.minTrustWeight = minTrustWeight
        self.excludeQuids = excludeQuids
        self.limit = limit
        self.offset = offset
    }
}

/// A single node-advertisement endpoint (QDP-0014).
public struct NodeAdvertEndpoint: Codable, Sendable {
    public var url: String
    public var transport: String?
    public var priority: Int?

    public init(url: String, transport: String? = nil, priority: Int? = nil) {
        self.url = url
        self.transport = transport
        self.priority = priority
    }
}

/// Capabilities advertised by a node (QDP-0014).
public struct NodeAdvertCapabilities: Codable, Sendable {
    public var features: [String]?
    public var maxRequestBytes: Int64?
    public var maxConcurrentStreams: Int?

    public init(
        features: [String]? = nil,
        maxRequestBytes: Int64? = nil,
        maxConcurrentStreams: Int? = nil
    ) {
        self.features = features
        self.maxRequestBytes = maxRequestBytes
        self.maxConcurrentStreams = maxConcurrentStreams
    }
}

/// Parameters for `publishNodeAdvertisement` (QDP-0014).
public struct NodeAdvertisementParams: Sendable {
    public var operatorQuid: String
    public var endpoints: [NodeAdvertEndpoint]
    public var supportedDomains: [String]?
    public var capabilities: NodeAdvertCapabilities
    public var domain: String
    public var protocolVersion: String?
    /// Time-to-live in seconds. Default 6h, max 7 days.
    public var ttl: TimeInterval?
    public var advertisementNonce: Int64

    public init(
        operatorQuid: String,
        endpoints: [NodeAdvertEndpoint],
        domain: String,
        capabilities: NodeAdvertCapabilities = NodeAdvertCapabilities(),
        supportedDomains: [String]? = nil,
        protocolVersion: String? = nil,
        ttl: TimeInterval? = nil,
        advertisementNonce: Int64
    ) {
        self.operatorQuid = operatorQuid
        self.endpoints = endpoints
        self.supportedDomains = supportedDomains
        self.capabilities = capabilities
        self.domain = domain
        self.protocolVersion = protocolVersion
        self.ttl = ttl
        self.advertisementNonce = advertisementNonce
    }
}
