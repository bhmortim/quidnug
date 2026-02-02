/**
 * Quidnug Client SDK - TypeScript Type Definitions
 * 
 * @packageDocumentation
 * @module @quidnug/client
 */

// ============================================================================
// Core Types
// ============================================================================

/**
 * A Quid represents an identity in the Quidnug network.
 * The ID is a 16-character hex string derived from SHA-256 of the public key.
 */
export interface Quid {
  /** 16-character hex ID derived from public key hash */
  id: string;
  /** Base64-encoded SPKI format public key */
  publicKey: string;
  /** Base64-encoded PKCS8 format private key */
  privateKey: string;
  /** Unix timestamp when the quid was created (only for generated quids) */
  created?: number;
  /** Optional metadata associated with the quid */
  metadata?: Record<string, unknown>;
  /** True if the quid was imported rather than generated */
  imported?: boolean;
}

/**
 * Ownership stake in a title transaction.
 * Percentages in a transaction must sum to exactly 100.0.
 */
export interface OwnershipStake {
  /** Quid ID of the owner */
  ownerId: string;
  /** Ownership percentage (0.0 to 100.0) */
  percentage: number;
  /** Optional stake type descriptor */
  stakeType?: string;
}

// ============================================================================
// Transaction Types
// ============================================================================

/** Base fields common to all transactions */
export interface BaseTransaction {
  /** Transaction type discriminator */
  type: TransactionType;
  /** Unix timestamp when transaction was created */
  timestamp: number;
  /** Trust domain for this transaction */
  trustDomain: string;
  /** Quid ID of the signer */
  signerQuid: string;
  /** Base64-encoded ECDSA signature */
  signature: string;
  /** Hex-encoded transaction ID (SHA-256 hash) */
  txId: string;
}

/** Transaction type discriminator values */
export type TransactionType = 'TRUST' | 'IDENTITY' | 'TITLE';

/**
 * Trust transaction establishing direct trust from truster to trustee.
 * Creates an edge in the trust graph that can be traversed for transitive trust.
 */
export interface TrustTransaction extends BaseTransaction {
  type: 'TRUST';
  /** Quid ID of the entity giving trust (same as signerQuid) */
  truster: string;
  /** Quid ID of the entity receiving trust */
  trustee: string;
  /** Trust level from 0.0 (distrust) to 1.0 (full trust) */
  trustLevel: number;
  /** Monotonic nonce for replay protection */
  nonce: number;
  /** Optional Unix timestamp when trust expires */
  validUntil?: number;
  /** Optional human-readable description */
  description?: string;
}

/**
 * Identity transaction declaring or updating identity attributes.
 */
export interface IdentityTransaction extends BaseTransaction {
  type: 'IDENTITY';
  /** Quid ID of the entity defining the identity (same as signerQuid) */
  definerQuid: string;
  /** Quid ID of the subject being defined */
  subjectQuid: string;
  /** Schema version for identity format */
  schemaVersion: string;
  /** Monotonic nonce for updates (must exceed previous value) */
  updateNonce: number;
  /** Optional human-readable name */
  name?: string;
  /** Optional description */
  description?: string;
  /** Custom identity attributes */
  attributes: Record<string, unknown>;
}

/**
 * Title transaction defining ownership of an asset.
 */
export interface TitleTransaction extends BaseTransaction {
  type: 'TITLE';
  /** Quid ID of the entity issuing the title (same as signerQuid) */
  issuerQuid: string;
  /** Quid ID of the asset being titled */
  assetQuid: string;
  /** Array of ownership stakes (must sum to 100%) */
  ownershipMap: OwnershipStake[];
  /** Signatures from transferring parties */
  transferSigs: Record<string, string>;
  /** Previous title transaction ID for transfers */
  prevTitleTxID?: string;
  /** Type of title (e.g., 'ownership', 'license') */
  titleType?: string;
}

/** Union of all transaction types */
export type Transaction = TrustTransaction | IdentityTransaction | TitleTransaction;

// ============================================================================
// Blockchain Types
// ============================================================================

/**
 * Trust proof for block validation (Proof of Trust consensus).
 */
export interface TrustProof {
  /** Trust domain this proof applies to */
  trustDomain: string;
  /** Quid ID of the validator */
  validatorId: string;
  /** Hex-encoded public key of validator */
  validatorPublicKey?: string;
  /** Validator's trust level in the block creator */
  validatorTrustInCreator: number;
  /** Array of validator signatures */
  validatorSigs: string[];
  /** Additional consensus metadata */
  consensusData?: Record<string, unknown>;
  /** Unix timestamp of validation */
  validationTime: number;
}

/**
 * Block in the Quidnug blockchain.
 */
export interface Block {
  /** Block index (height) */
  index: number;
  /** Unix timestamp of block creation */
  timestamp: number;
  /** Transactions included in this block */
  transactions: Transaction[];
  /** Trust proof for this block */
  trustProof: TrustProof;
  /** Hash of the previous block */
  prevHash: string;
  /** Hash of this block */
  hash: string;
}

// ============================================================================
// Network Types
// ============================================================================

/**
 * A node in the Quidnug network.
 */
export interface Node {
  /** Node's quid ID */
  id: string;
  /** Network address (URL) */
  address: string;
  /** Trust domains this node participates in */
  trustDomains: string[];
  /** Domains this node manages/validates */
  managedDomains: string[];
  /** Whether this node is a validator */
  isValidator: boolean;
  /** Unix timestamp of last contact */
  lastSeen: number;
  /** Current connection status */
  connectionStatus: string;
}

/**
 * Trust domain configuration.
 */
export interface TrustDomain {
  /** Domain name (dot-notation, e.g., 'sub.example.com') */
  name: string;
  /** List of validator node IDs */
  validatorNodes: string[];
  /** Minimum trust threshold for validation */
  trustThreshold: number;
  /** Hash of the latest block */
  blockchainHead: string;
  /** Map of validator IDs to their voting weights */
  validators: Record<string, number>;
  /** Map of validator IDs to their hex-encoded public keys */
  validatorPublicKeys: Record<string, string>;
}

// ============================================================================
// API Response Types
// ============================================================================

/**
 * Standard API response wrapper.
 */
export interface APIResponse<T> {
  /** Response data */
  data: T;
  /** Optional status message */
  message?: string;
}

/**
 * Pagination metadata.
 */
export interface PaginationMeta {
  /** Total number of items */
  total: number;
  /** Number of items returned */
  limit: number;
  /** Number of items skipped */
  offset: number;
  /** Whether there are more items */
  hasMore: boolean;
}

/**
 * Paginated API response.
 */
export interface PaginatedResponse<T> {
  /** Array of items */
  data: T[];
  /** Pagination metadata */
  pagination: PaginationMeta;
}

/**
 * Pagination options for list queries.
 */
export interface PaginationOptions {
  /** Maximum number of items to return (default: 50) */
  limit?: number;
  /** Number of items to skip (default: 0) */
  offset?: number;
}

// ============================================================================
// Query & Result Types
// ============================================================================

/**
 * Result of a relational trust query.
 */
export interface TrustResult {
  /** Quid ID of the observer (source) */
  observer: string;
  /** Quid ID of the target */
  target: string;
  /** Computed relational trust level (0.0 to 1.0) */
  trustLevel: number;
  /** Array of quid IDs forming the best trust path */
  trustPath: string[];
  /** Number of hops in the path */
  pathDepth: number;
  /** Trust domain queried */
  domain: string;
  /** Optional message (e.g., 'No trust relationship found') */
  message?: string;
}

/**
 * Result of a trust path search.
 */
export interface TrustPathResult {
  /** Whether a valid path was found */
  found: boolean;
  /** Computed trust level along the path */
  trustLevel: number;
  /** Array of quid IDs from observer to target */
  path: string[];
  /** Number of hops in the path */
  pathDepth: number;
}

/**
 * Client-side trust graph for offline computation.
 * Maps truster quid ID -> trustee quid ID -> trust level.
 */
export type TrustGraph = Record<string, Record<string, number>>;

// ============================================================================
// Method Parameter Types
// ============================================================================

/**
 * Configuration options for QuidnugClient constructor.
 */
export interface QuidnugClientOptions {
  /** URL of the default Quidnug node */
  defaultNode?: string;
  /** Enable debug logging */
  debug?: boolean;
  /** Maximum number of retry attempts for failed requests (default: 3) */
  maxRetries?: number;
  /** Base delay in milliseconds for exponential backoff (default: 1000) */
  retryBaseDelayMs?: number;
}

/**
 * Parameters for creating a trust transaction.
 */
export interface CreateTrustTransactionParams {
  /** Quid ID of the entity being trusted */
  trustee: string;
  /** Trust domain */
  domain: string;
  /** Trust level (0.0 to 1.0) */
  trustLevel: number;
  /** Monotonic nonce for replay protection (defaults to 1) */
  nonce?: number;
  /** Optional expiration timestamp (Unix seconds) */
  validUntil?: number;
  /** Optional description */
  description?: string;
}

/**
 * Parameters for creating an identity transaction.
 */
export interface CreateIdentityTransactionParams {
  /** Quid ID being defined */
  subjectQuid: string;
  /** Identity domain */
  domain: string;
  /** Optional human-readable name */
  name?: string;
  /** Optional description */
  description?: string;
  /** Custom attributes */
  attributes?: Record<string, unknown>;
}

/**
 * Parameters for creating a title transaction.
 */
export interface CreateTitleTransactionParams {
  /** Quid ID of the asset */
  assetQuid: string;
  /** Title domain */
  domain: string;
  /** Array of ownership stakes (must sum to 100%) */
  ownershipMap: OwnershipStake[];
  /** Previous title transaction ID for transfers */
  prevTitleTxID?: string;
  /** Type of title */
  titleType?: string;
}

/**
 * Parameters for importing an existing quid.
 */
export interface ImportQuidParams {
  /** Base64-encoded PKCS8 format private key */
  privateKey: string;
  /** Base64-encoded SPKI format public key */
  publicKey: string;
}

/**
 * Options for trust level queries.
 */
export interface TrustQueryOptions {
  /** Maximum trust path depth (hops), default 5 */
  maxDepth?: number;
}

/**
 * Options for trust path queries.
 */
export interface TrustPathOptions extends TrustQueryOptions {
  /** Minimum trust level threshold */
  minTrustLevel?: number;
}

/**
 * Parameters for relational trust query.
 */
export interface RelationalTrustQuery {
  /** Quid ID of the observer */
  observer: string;
  /** Quid ID of the target */
  target: string;
  /** Trust domain (defaults to 'default') */
  domain?: string;
  /** Maximum path depth */
  maxDepth?: number;
}

/**
 * Options for trust registry queries.
 */
export interface TrustRegistryQueryOptions extends PaginationOptions {
  /** Filter by truster quid ID */
  truster?: string;
  /** Filter by trustee quid ID */
  trustee?: string;
}

/**
 * Options for identity registry queries.
 */
export interface IdentityRegistryQueryOptions extends PaginationOptions {
  /** Filter by specific quid ID */
  quidId?: string;
}

/**
 * Options for title registry queries.
 */
export interface TitleRegistryQueryOptions extends PaginationOptions {
  /** Filter by specific asset ID */
  assetId?: string;
  /** Filter by owner ID */
  ownerId?: string;
}

// ============================================================================
// Internal Types (for reference)
// ============================================================================

/**
 * Internal node pool entry tracking node health.
 */
export interface NodePoolEntry {
  /** Node URL */
  url: string;
  /** Health status */
  status: 'unknown' | 'healthy' | 'unhealthy' | 'unreachable';
  /** Unix timestamp of last health check */
  lastChecked: number;
  /** Node's quid ID (if known) */
  quidId?: string;
}

// ============================================================================
// QuidnugClient Class
// ============================================================================

/**
 * Quidnug Client SDK for interacting with Quidnug nodes.
 * 
 * Provides functionality for identity, trust, and ownership management
 * using the relational trust model where trust is computed from an
 * observer's perspective through the trust graph.
 * 
 * @example
 * ```typescript
 * import QuidnugClient from '@quidnug/client';
 * 
 * const client = new QuidnugClient({ defaultNode: 'http://localhost:8080' });
 * 
 * // Generate a new identity
 * const alice = await client.generateQuid({ name: 'Alice' });
 * console.log('Created quid:', alice.id);
 * 
 * // Create and submit a trust transaction
 * const trustTx = await client.createTrustTransaction({
 *   trustee: 'bob123456789abc',
 *   domain: 'example.com',
 *   trustLevel: 0.8
 * }, alice);
 * 
 * await client.submitTransaction(trustTx);
 * 
 * // Query relational trust
 * const result = await client.getTrustLevel(alice.id, 'charlie789abcdef', 'example.com');
 * console.log('Trust level:', result.trustLevel);
 * console.log('Trust path:', result.trustPath);
 * ```
 */
declare class QuidnugClient {
  /** Array of nodes in the pool */
  nodes: NodePoolEntry[];
  /** Default node URL */
  defaultNode: string | undefined;
  /** Debug mode flag */
  debug: boolean;
  /** Maximum number of retry attempts for failed requests */
  maxRetries: number;
  /** Base delay in milliseconds for exponential backoff */
  retryBaseDelayMs: number;

  /**
   * Initialize the Quidnug client.
   * @param options - Configuration options
   */
  constructor(options?: QuidnugClientOptions);

  /**
   * Add a node to the client's node pool.
   * Automatically initiates a health check for the node.
   * @param nodeUrl - URL of the Quidnug node
   */
  addNode(nodeUrl: string): void;

  /**
   * Generate a new quid with a fresh ECDSA P-256 key pair.
   * Attempts to register with a healthy node but falls back to local-only mode.
   * @param metadata - Optional metadata to associate with the quid
   * @returns The generated quid object
   */
  generateQuid(metadata?: Record<string, unknown>): Promise<Quid>;

  /**
   * Import an existing quid from a private/public key pair.
   * Both keys are required because Web Crypto API cannot derive public from private.
   * @param keys - Object containing privateKey and publicKey in Base64 format
   * @returns The imported quid object
   * @throws Error if privateKey is missing
   * @throws Error if publicKey is missing
   * @throws Error if key format is invalid
   */
  importQuid(keys: ImportQuidParams): Promise<Quid>;

  /**
   * Create a trust transaction to establish direct trust from the signer to a trustee.
   * The signing quid becomes the truster.
   * @param params - Transaction parameters
   * @param quid - Quid object with private key for signing (becomes the truster)
   * @returns Signed trust transaction ready for submission
   * @throws Error if quid lacks private key
   * @throws Error if required parameters are missing
   * @throws Error if trustLevel is not between 0.0 and 1.0
   */
  createTrustTransaction(params: CreateTrustTransactionParams, quid: Quid): Promise<TrustTransaction>;

  /**
   * Create an identity transaction to declare or update identity attributes.
   * @param params - Transaction parameters
   * @param quid - Quid object with private key for signing
   * @returns Signed identity transaction ready for submission
   * @throws Error if quid lacks private key
   * @throws Error if required parameters are missing
   */
  createIdentityTransaction(params: CreateIdentityTransactionParams, quid: Quid): Promise<IdentityTransaction>;

  /**
   * Create a title transaction to define asset ownership.
   * @param params - Transaction parameters
   * @param quid - Quid object with private key for signing
   * @returns Signed title transaction ready for submission
   * @throws Error if quid lacks private key
   * @throws Error if required parameters are missing
   * @throws Error if ownership percentages don't sum to 100%
   */
  createTitleTransaction(params: CreateTitleTransactionParams, quid: Quid): Promise<TitleTransaction>;

  /**
   * Submit a signed transaction to the Quidnug network.
   * @param transaction - Signed transaction to submit
   * @returns Transaction submission result
   * @throws Error if transaction is invalid
   * @throws Error if no healthy nodes are available
   */
  submitTransaction(transaction: Transaction): Promise<APIResponse<unknown>>;

  /**
   * Get relational trust level between quids.
   * Computes trust from the observer's perspective through the trust graph.
   * @param observer - Quid ID of the observer (source of trust query)
   * @param target - Quid ID of the target (destination of trust query)
   * @param domain - Trust domain
   * @param options - Additional query options
   * @returns Trust result with computed trust level and path
   */
  getTrustLevel(observer: string, target: string, domain: string, options?: TrustQueryOptions): Promise<TrustResult>;

  /**
   * Get quid identity information.
   * @param quidId - Quid ID to look up
   * @param domain - Optional specific domain
   * @returns Identity information or null if not found
   */
  getIdentity(quidId: string, domain?: string): Promise<IdentityTransaction | null>;

  /**
   * Get asset ownership information.
   * @param assetId - Asset quid ID
   * @param domain - Optional specific domain
   * @returns Ownership information or null if not found
   */
  getAssetOwnership(assetId: string, domain?: string): Promise<TitleTransaction | null>;

  /**
   * Find trust path between quids.
   * Convenience wrapper around getTrustLevel() with path-focused results.
   * @param observer - Observer quid ID
   * @param target - Target quid ID
   * @param domain - Trust domain
   * @param options - Query options including optional minTrustLevel threshold
   * @returns Trust path result
   */
  findTrustPath(observer: string, target: string, domain: string, options?: TrustPathOptions): Promise<TrustPathResult>;

  /**
   * Query relational trust using a structured query object.
   * Useful for programmatic queries with dynamic parameters.
   * @param query - Relational trust query parameters
   * @returns Relational trust result
   */
  queryRelationalTrust(query: RelationalTrustQuery): Promise<TrustResult>;

  /**
   * Compute transitive trust client-side from a cached trust graph.
   * Uses BFS with multiplicative decay, matching the server-side algorithm.
   * @param trustGraph - Map of truster -> trustee -> trustLevel
   * @param observer - Observer quid ID
   * @param target - Target quid ID
   * @param maxDepth - Maximum path depth (default: 5)
   * @returns Trust result with trustLevel and trustPath
   */
  computeTransitiveTrust(
    trustGraph: TrustGraph,
    observer: string,
    target: string,
    maxDepth?: number
  ): TrustResult;

  /**
   * Get blocks from the blockchain with pagination.
   * @param options - Pagination options
   * @returns Paginated blocks response
   */
  getBlocks(options?: PaginationOptions): Promise<PaginatedResponse<Block>>;

  /**
   * Get known nodes with pagination.
   * @param options - Pagination options
   * @returns Paginated nodes response
   */
  getNodes(options?: PaginationOptions): Promise<PaginatedResponse<Node>>;

  /**
   * Get pending transactions with pagination.
   * @param options - Pagination options
   * @returns Paginated transactions response
   */
  getPendingTransactions(options?: PaginationOptions): Promise<PaginatedResponse<Transaction>>;

  /**
   * Query the trust registry with optional filters and pagination.
   * @param options - Query options with optional truster/trustee filters
   * @returns Trust registry data (paginated when no filters applied)
   */
  queryTrustRegistry(options?: TrustRegistryQueryOptions): Promise<PaginatedResponse<TrustTransaction> | APIResponse<unknown>>;

  /**
   * Query the identity registry with optional filters and pagination.
   * @param options - Query options with optional quidId filter
   * @returns Identity registry data (paginated when no quidId filter applied)
   */
  queryIdentityRegistry(options?: IdentityRegistryQueryOptions): Promise<PaginatedResponse<IdentityTransaction> | APIResponse<unknown>>;

  /**
   * Query the title registry with optional filters and pagination.
   * @param options - Query options with optional assetId/ownerId filters
   * @returns Title registry data (paginated when no filters applied)
   */
  queryTitleRegistry(options?: TitleRegistryQueryOptions): Promise<PaginatedResponse<TitleTransaction> | APIResponse<unknown>>;

  /**
   * Query a specific trust domain.
   * @param domain - Domain to query
   * @param type - Query type ('trust', 'identity', 'title')
   * @param param - Query parameter
   * @returns Query results
   */
  queryDomain(domain: string, type: 'trust' | 'identity' | 'title', param: string): Promise<APIResponse<unknown>>;

  /**
   * Find nodes that manage a specific domain.
   * Implements hierarchical domain walking to find parent authorities.
   * @param domain - Domain to find nodes for
   * @returns List of nodes managing the domain
   */
  findNodesForDomain(domain: string): Promise<Node[]>;
}

// ============================================================================
// Module Exports
// ============================================================================

export default QuidnugClient;
export { QuidnugClient };
