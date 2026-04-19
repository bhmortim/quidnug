/**
 * Quidnug Client SDK v2 — TypeScript type definitions.
 *
 * These definitions extend QuidnugClient (from quidnug-client.d.ts)
 * via module augmentation. Importing "./quidnug-client-v2.js" as a
 * side-effect installs the methods described here on the prototype.
 */

// ============================================================================
// Guardian types (QDP-0002, QDP-0006)
// ============================================================================

export interface GuardianRef {
  quid: string;
  weight?: number;
  epoch?: number;
  addedAtBlock?: number;
}

export interface GuardianSet {
  subjectQuid: string;
  guardians: GuardianRef[];
  threshold: number;
  recoveryDelaySeconds: number;
  requireGuardianRotation?: boolean;
  updatedAtBlock?: number;
}

export interface PrimarySignature {
  keyEpoch: number;
  signature: string;
}

export interface GuardianSignature {
  guardianQuid: string;
  keyEpoch: number;
  signature: string;
}

export interface GuardianSetUpdate {
  subjectQuid: string;
  newSet: GuardianSet;
  anchorNonce: number;
  validFrom: number;
  primarySignature?: PrimarySignature;
  newGuardianConsents?: GuardianSignature[];
  currentGuardianSigs?: GuardianSignature[];
}

export interface GuardianRecoveryInit {
  subjectQuid: string;
  fromEpoch: number;
  toEpoch: number;
  newPublicKey: string;
  minNextNonce: number;
  maxAcceptedOldNonce: number;
  anchorNonce: number;
  validFrom: number;
  guardianSigs?: GuardianSignature[];
  expiresAt?: number;
}

export interface GuardianRecoveryVeto {
  subjectQuid: string;
  recoveryAnchorHash: string;
  anchorNonce: number;
  validFrom: number;
  primarySignature?: PrimarySignature;
  guardianSigs?: GuardianSignature[];
}

export interface GuardianRecoveryCommit {
  subjectQuid: string;
  recoveryAnchorHash: string;
  anchorNonce: number;
  validFrom: number;
  committerQuid: string;
  committerSig: string;
}

export interface GuardianResignation {
  guardianQuid: string;
  subjectQuid: string;
  guardianSetHash: string;
  resignationNonce: number;
  effectiveAt: number;
  signature?: string;
}

// ============================================================================
// Gossip types (QDP-0003, QDP-0005)
// ============================================================================

export interface DomainFingerprint {
  domain: string;
  blockHeight: number;
  blockHash: string;
  producerQuid: string;
  timestamp: number;
  signature?: string;
  schemaVersion?: number;
}

export interface MerkleProofFrame {
  hash: string;
  side: "left" | "right";
}

export interface AnchorGossipMessage {
  messageId: string;
  originDomain: string;
  originBlockHeight: number;
  originBlock: unknown;
  anchorTxIndex: number;
  domainFingerprint: DomainFingerprint;
  timestamp: number;
  gossipProducerQuid: string;
  gossipSignature?: string;
  schemaVersion?: number;
  merkleProof?: MerkleProofFrame[];
}

// ============================================================================
// Snapshot + fork-block types
// ============================================================================

export interface NonceSnapshotEntry {
  quid: string;
  epoch: number;
  maxNonce: number;
}

export interface NonceSnapshot {
  blockHeight: number;
  blockHash: string;
  timestamp: number;
  trustDomain: string;
  entries: NonceSnapshotEntry[];
  producerQuid: string;
  signature?: string;
  schemaVersion?: number;
}

export interface ForkSig {
  validatorQuid: string;
  keyEpoch: number;
  signature: string;
}

export interface ForkBlock {
  trustDomain: string;
  feature: string;
  forkHeight: number;
  forkNonce: number;
  proposedAt: number;
  signatures?: ForkSig[];
  expiresAt?: number;
}

// ============================================================================
// Client method augmentations
// ============================================================================

declare module "./quidnug-client.js" {
  interface QuidnugClient {
    // Guardians
    submitGuardianSetUpdate(update: GuardianSetUpdate): Promise<unknown>;
    submitRecoveryInit(init: GuardianRecoveryInit): Promise<unknown>;
    submitRecoveryVeto(veto: GuardianRecoveryVeto): Promise<unknown>;
    submitRecoveryCommit(commit: GuardianRecoveryCommit): Promise<unknown>;
    submitGuardianResignation(r: GuardianResignation): Promise<unknown>;
    getGuardianSet(quidId: string): Promise<GuardianSet | null>;
    getPendingRecovery(quidId: string): Promise<unknown | null>;
    getGuardianResignations(quidId: string): Promise<GuardianResignation[]>;

    // Gossip
    submitDomainFingerprint(fp: DomainFingerprint): Promise<unknown>;
    getLatestDomainFingerprint(domain: string): Promise<DomainFingerprint | null>;
    submitAnchorGossip(msg: AnchorGossipMessage): Promise<unknown>;
    pushAnchor(msg: AnchorGossipMessage): Promise<unknown>;
    pushFingerprint(fp: DomainFingerprint): Promise<unknown>;

    // Bootstrap
    submitNonceSnapshot(s: NonceSnapshot): Promise<unknown>;
    getLatestNonceSnapshot(domain: string): Promise<NonceSnapshot | null>;
    getBootstrapStatus(): Promise<unknown>;

    // Fork-block
    submitForkBlock(fb: ForkBlock): Promise<unknown>;
    getForkBlockStatus(): Promise<unknown>;
  }

  namespace QuidnugClient {
    // Static helpers installed by v2
    function verifyInclusionProof(
      txBytes: Uint8Array | ArrayBuffer | string,
      frames: MerkleProofFrame[],
      expectedRootHex: string
    ): Promise<boolean>;

    function canonicalBytes(obj: object, excludeFields?: string[]): Uint8Array;

    function bytesToHex(buf: Uint8Array | ArrayBuffer): string;
    function hexToBytes(hex: string): Uint8Array;
  }
}
