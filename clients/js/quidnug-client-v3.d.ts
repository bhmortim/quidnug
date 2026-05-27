/**
 * Quidnug Client SDK v3 — TypeScript type definitions.
 *
 * Adds module augmentations for the v3 protocol surface
 * (peers, audit, moderation, privacy, discovery, DNS
 * attestation, node advertisements). Bodies and responses are
 * intentionally loose — pass any JSON-serializable object as
 * the request and accept any JSON response. Strongly-typed
 * wrappers can be layered on top by callers that need them.
 */

export type JsonValue =
  | string
  | number
  | boolean
  | null
  | JsonValue[]
  | { [key: string]: JsonValue };

declare module "./quidnug-client.js" {
  interface QuidnugClient {
    // Peers
    getPeers(params?: Record<string, unknown>): Promise<unknown>;
    getPeer(nodeQuid: string): Promise<unknown | null>;

    // Node advertisements
    submitNodeAdvertisement(ad: object): Promise<unknown>;

    // Domain registry extras
    getTopDomains(params?: Record<string, unknown>): Promise<unknown>;
    getTentativeBlocks(domain: string): Promise<unknown>;
    submitDomainGossip(msg: object): Promise<unknown>;

    // Moderation (QDP-0015)
    submitModerationAction(action: object): Promise<unknown>;
    getModerationActions(targetType: string, targetId: string): Promise<unknown>;

    // Audit (QDP-0018)
    getAuditHead(): Promise<unknown>;
    getAuditEntries(params?: { since?: number; limit?: number }): Promise<unknown>;
    getAuditEntry(sequence: number | string): Promise<unknown | null>;

    // Privacy (QDP-0017)
    submitDSR(request: object): Promise<unknown>;
    getDSRStatus(requestTxId: string): Promise<unknown | null>;
    grantConsent(grant: object): Promise<unknown>;
    withdrawConsent(withdraw: object): Promise<unknown>;
    getConsentHistory(params?: Record<string, unknown>): Promise<unknown>;
    createProcessingRestriction(restriction: object): Promise<unknown>;
    getProcessingRestrictions(subjectQuid: string): Promise<unknown>;
    submitDSRCompliance(compliance: object): Promise<unknown>;

    // Discovery (QDP-0014) — under /api/v2/discovery/*
    getDiscoveryDomain(name: string): Promise<unknown | null>;
    getDiscoveryNode(quid: string): Promise<unknown | null>;
    getDiscoveryOperator(quid: string): Promise<unknown | null>;
    discoveryQuids(params?: Record<string, unknown>): Promise<unknown>;
    discoveryTrustedQuids(params?: Record<string, unknown>): Promise<unknown>;

    // DNS attestation (QDP-0023) — under /api/v2/dns/*
    submitDNSClaim(claim: object): Promise<unknown>;
    submitDNSChallenge(challenge: object): Promise<unknown>;
    submitDNSAttestation(attestation: object): Promise<unknown>;
    submitDNSRenewal(renewal: object): Promise<unknown>;
    submitDNSRevocation(revocation: object): Promise<unknown>;
    submitDNSDelegate(delegate: object): Promise<unknown>;
    submitDNSDelegateRevocation(revocation: object): Promise<unknown>;
    getDNSAttestations(domain: string): Promise<unknown>;
    getDNSAttestationsWeighted(domain: string): Promise<unknown>;
    resolveDNS(domain: string, recordType: string): Promise<unknown>;
  }
}
