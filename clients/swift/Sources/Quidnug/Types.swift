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
