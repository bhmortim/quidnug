/**
 * Quidnug Client SDK - Reference Implementation
 * 
 * This is a simplified client library for interacting with Quidnug nodes.
 * It provides the core functionality needed for applications to integrate with
 * the Quidnug platform for identity, trust, and ownership management.
 * 
 * Trust Model: Quidnug uses relational trust where trust is computed from an
 * observer's perspective through the trust graph. Trust is transitive with
 * multiplicative decay - if A trusts B at 0.8 and B trusts C at 0.9, then
 * A's relational trust in C is 0.8 * 0.9 = 0.72. The system finds the best
 * path (highest trust) when multiple paths exist.
 */

class QuidnugClient {
  /**
   * Initialize the Quidnug client
   * @param {Object} options - Configuration options
   * @param {string} options.defaultNode - URL of the default Quidnug node
   * @param {boolean} options.debug - Enable debug logging
   */
  constructor(options = {}) {
    this.nodes = [];
    this.defaultNode = options.defaultNode;
    this.debug = options.debug || false;
    this.maxRetries = options.maxRetries !== undefined ? options.maxRetries : 3;
    this.retryBaseDelayMs = options.retryBaseDelayMs !== undefined ? options.retryBaseDelayMs : 1000;
    
    if (this.defaultNode) {
      this.addNode(this.defaultNode);
    }
  }

  /**
   * Add a node to the client's node pool
   * @param {string} nodeUrl - URL of the Quidnug node
   */
  addNode(nodeUrl) {
    this.nodes.push({
      url: nodeUrl,
      status: 'unknown',
      lastChecked: 0
    });

    // Check node health
    this._checkNodeHealth(nodeUrl);
  }

  /**
   * Check if a node is healthy
   * @private
   * @param {string} nodeUrl - URL of the node to check
   */
  async _checkNodeHealth(nodeUrl) {
    try {
      const response = await fetch(`${nodeUrl}/api/health`);
      const data = await response.json();
      
      const nodeIndex = this.nodes.findIndex(node => node.url === nodeUrl);
      if (nodeIndex >= 0) {
        this.nodes[nodeIndex].status = data.status === 'ok' ? 'healthy' : 'unhealthy';
        this.nodes[nodeIndex].lastChecked = Date.now();
        this.nodes[nodeIndex].quidId = data.quidId;
      }
      
      if (this.debug) {
        console.log(`Node ${nodeUrl} status: ${this.nodes[nodeIndex].status}`);
      }
    } catch (error) {
      const nodeIndex = this.nodes.findIndex(node => node.url === nodeUrl);
      if (nodeIndex >= 0) {
        this.nodes[nodeIndex].status = 'unreachable';
        this.nodes[nodeIndex].lastChecked = Date.now();
      }
      
      if (this.debug) {
        console.error(`Error checking node ${nodeUrl}:`, error);
      }
    }
  }

  /**
   * Fetch with retry and exponential backoff
   * @private
   * @param {string} url - URL to fetch
   * @param {Object} options - Fetch options
   * @param {number} [maxRetries] - Maximum number of retries (uses instance default if not specified)
   * @param {number} [baseDelayMs] - Base delay in milliseconds (uses instance default if not specified)
   * @returns {Promise<Response>} Fetch response
   */
  async _fetchWithRetry(url, options = {}, maxRetries, baseDelayMs) {
    const retries = maxRetries !== undefined ? maxRetries : this.maxRetries;
    const baseDelay = baseDelayMs !== undefined ? baseDelayMs : this.retryBaseDelayMs;
    
    let lastError;
    for (let attempt = 0; attempt <= retries; attempt++) {
      try {
        const response = await fetch(url, options);
        // Don't retry on 4xx client errors (except 429 rate limit)
        if (response.ok || (response.status >= 400 && response.status < 500 && response.status !== 429)) {
          return response;
        }
        // Retry on 5xx server errors and 429 rate limit
        if (attempt < retries) {
          const delay = baseDelay * Math.pow(2, attempt) + Math.random() * 100;
          if (this.debug) {
            console.log(`Retry ${attempt + 1}/${retries} for ${url} after ${Math.round(delay)}ms (HTTP ${response.status})`);
          }
          await new Promise(resolve => setTimeout(resolve, delay));
        }
        lastError = new Error(`HTTP ${response.status}`);
      } catch (error) {
        lastError = error;
        if (attempt < retries) {
          const delay = baseDelay * Math.pow(2, attempt) + Math.random() * 100;
          if (this.debug) {
            console.log(`Retry ${attempt + 1}/${retries} for ${url} after ${Math.round(delay)}ms (${error.message})`);
          }
          await new Promise(resolve => setTimeout(resolve, delay));
        }
      }
    }
    throw lastError;
  }

  /**
   * Get a healthy node from the pool
   * @private
   * @returns {string} URL of a healthy node
   * @throws {Error} If no healthy nodes are available
   */
  _getHealthyNode() {
    const healthyNodes = this.nodes.filter(node => node.status === 'healthy');
    
    if (healthyNodes.length === 0) {
      throw new Error('No healthy nodes available');
    }
    
    // Return a random healthy node
    return healthyNodes[Math.floor(Math.random() * healthyNodes.length)].url;
  }

  /**
   * Generate cryptographic key pair for a new quid
   * @private
   * @returns {Promise<Object>} Key pair { privateKey, publicKey }
   */
  async _generateKeyPair() {
    // Use the Web Crypto API for secure key generation
    const keyPair = await window.crypto.subtle.generateKey(
      {
        name: 'ECDSA',
        namedCurve: 'P-256'
      },
      true,
      ['sign', 'verify']
    );
    
    // Export keys
    const privateKeyBuffer = await window.crypto.subtle.exportKey('pkcs8', keyPair.privateKey);
    const publicKeyBuffer = await window.crypto.subtle.exportKey('spki', keyPair.publicKey);
    
    // Convert to base64
    const privateKey = this._arrayBufferToBase64(privateKeyBuffer);
    const publicKey = this._arrayBufferToBase64(publicKeyBuffer);
    
    return { privateKey, publicKey, keyPair };
  }
  
  /**
   * Convert ArrayBuffer to Base64 string
   * @private
   * @param {ArrayBuffer} buffer - Array buffer to convert
   * @returns {string} Base64 encoded string
   */
  _arrayBufferToBase64(buffer) {
    const bytes = new Uint8Array(buffer);
    let binary = '';
    for (let i = 0; i < bytes.byteLength; i++) {
      binary += String.fromCharCode(bytes[i]);
    }
    return window.btoa(binary);
  }
  
  /**
   * Convert Base64 to ArrayBuffer
   * @private
   * @param {string} base64 - Base64 encoded string
   * @returns {ArrayBuffer} Decoded array buffer
   */
  _base64ToArrayBuffer(base64) {
    const binaryString = window.atob(base64);
    const bytes = new Uint8Array(binaryString.length);
    for (let i = 0; i < binaryString.length; i++) {
      bytes[i] = binaryString.charCodeAt(i);
    }
    return bytes.buffer;
  }
  
  /**
   * Generate a quid ID from a public key
   * @private
   * @param {string} publicKeyBase64 - Base64 encoded public key
   * @returns {Promise<string>} Quid ID
   */
  async _generateQuidId(publicKeyBase64) {
    const publicKeyBuffer = this._base64ToArrayBuffer(publicKeyBase64);
    const hashBuffer = await window.crypto.subtle.digest('SHA-256', publicKeyBuffer);
    const hashArray = Array.from(new Uint8Array(hashBuffer));
    const hashHex = hashArray.map(b => b.toString(16).padStart(2, '0')).join('');
    return hashHex.substring(0, 16);
  }
  
  /**
   * Sign data with a quid's private key
   * @private
   * @param {Object} keyPair - Key pair object
   * @param {ArrayBuffer} data - Data to sign
   * @returns {Promise<string>} Base64 encoded signature
   */
  async _signData(keyPair, data) {
    const signature = await window.crypto.subtle.sign(
      {
        name: 'ECDSA',
        hash: { name: 'SHA-256' }
      },
      keyPair.privateKey,
      data
    );
    
    return this._arrayBufferToBase64(signature);
  }
  
  /**
   * Generate a new quid
   * @param {Object} metadata - Optional metadata for the quid
   * @returns {Promise<Object>} Quid object
   */
  async generateQuid(metadata = {}) {
    // Generate key pair
    const { privateKey, publicKey, keyPair } = await this._generateKeyPair();
    
    // Generate quid ID
    const id = await this._generateQuidId(publicKey);
    
    // Create quid object
    const quid = {
      id,
      publicKey,
      privateKey,
      created: Math.floor(Date.now() / 1000),
      metadata
    };
    
    // Register with node if available
    try {
      const nodeUrl = this._getHealthyNode();
      const response = await this._fetchWithRetry(`${nodeUrl}/api/quids`, {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json'
        },
        body: JSON.stringify({
          publicKey,
          metadata
        })
      });
      
      if (!response.ok) {
        if (this.debug) {
          console.warn('Failed to register quid with node, but quid was created locally');
        }
      }
    } catch (error) {
      if (this.debug) {
        console.warn('Failed to register quid with node, but quid was created locally:', error);
      }
    }
    
    return quid;
  }
  
  /**
   * Import an existing quid from a private/public key pair.
   * 
   * Both keys are required because the Web Crypto API cannot derive a public key
   * from a private key. The old signature `importQuid(privateKeyBase64)` is no
   * longer supported.
   * 
   * @param {Object} keys - Key pair object
   * @param {string} keys.privateKey - Base64-encoded PKCS8 format private key
   * @param {string} keys.publicKey - Base64-encoded SPKI format public key
   * @returns {Promise<Object>} Quid object with id, publicKey, privateKey, and imported flag
   * @throws {Error} If privateKey is missing: "privateKey is required (Base64-encoded PKCS8 format)"
   * @throws {Error} If publicKey is missing: "publicKey is required (Base64-encoded SPKI format). Web Crypto API cannot derive public key from private key."
   * @throws {Error} If key format is invalid: "Failed to import quid: ..."
   */
  async importQuid({ privateKey, publicKey } = {}) {
    if (!privateKey) {
      throw new Error('privateKey is required (Base64-encoded PKCS8 format)');
    }
    if (!publicKey) {
      throw new Error('publicKey is required (Base64-encoded SPKI format). Web Crypto API cannot derive public key from private key.');
    }
    
    try {
      const privateKeyBuffer = this._base64ToArrayBuffer(privateKey);
      await window.crypto.subtle.importKey(
        'pkcs8',
        privateKeyBuffer,
        {
          name: 'ECDSA',
          namedCurve: 'P-256'
        },
        true,
        ['sign']
      );
      
      const publicKeyBuffer = this._base64ToArrayBuffer(publicKey);
      await window.crypto.subtle.importKey(
        'spki',
        publicKeyBuffer,
        {
          name: 'ECDSA',
          namedCurve: 'P-256'
        },
        true,
        ['verify']
      );
      
      const id = await this._generateQuidId(publicKey);
      
      return {
        id,
        publicKey,
        privateKey,
        imported: true
      };
    } catch (error) {
      throw new Error(`Failed to import quid: ${error.message}`);
    }
  }
  
  /**
   * Create a trust transaction to establish direct trust from the signer to a trustee.
   * 
   * This creates a direct trust edge in the trust graph. The signing quid becomes
   * the truster. When others query relational trust, this edge may be traversed
   * as part of transitive trust computation.
   * 
   * @param {Object} params - Transaction parameters
   * @param {string} params.trustee - Quid ID of the entity being trusted
   * @param {string} params.domain - Trust domain
   * @param {number} params.trustLevel - Trust level (0.0 to 1.0)
   *   - 0.0: complete distrust
   *   - 0.5: neutral
   *   - 1.0: complete trust
   * @param {number} [params.nonce] - Monotonic nonce for replay protection (defaults to 1 for new trust relationships)
   * @param {number} [params.validUntil] - Optional expiration timestamp (Unix seconds)
   * @param {string} [params.description] - Optional description
   * @param {Object} quid - Quid object with private key for signing (becomes the truster)
   * @returns {Promise<Object>} Signed trust transaction
   */
  async createTrustTransaction(params, quid) {
    if (!quid || !quid.privateKey) {
      throw new Error('Valid quid with private key is required for signing');
    }
    
    // Validate parameters
    if (!params.trustee || !params.domain || params.trustLevel === undefined) {
      throw new Error('Missing required parameters: trustee, domain, trustLevel');
    }
    
    if (params.trustLevel < 0 || params.trustLevel > 1) {
      throw new Error('Trust level must be between 0.0 and 1.0');
    }
    
    // Create transaction object
    const transaction = {
      type: 'TRUST',
      timestamp: Math.floor(Date.now() / 1000),
      trustDomain: params.domain,
      signerQuid: quid.id,
      truster: quid.id,
      trustee: params.trustee,
      trustLevel: params.trustLevel,
      nonce: params.nonce || 1
    };
    
    if (params.validUntil) {
      transaction.validUntil = params.validUntil;
    }
    
    if (params.description) {
      transaction.description = params.description;
    }
    
    // Sign transaction
    const txData = new TextEncoder().encode(JSON.stringify(transaction));
    
    // Import private key for signing
    const privateKeyBuffer = this._base64ToArrayBuffer(quid.privateKey);
    const privateKey = await window.crypto.subtle.importKey(
      'pkcs8',
      privateKeyBuffer,
      {
        name: 'ECDSA',
        namedCurve: 'P-256'
      },
      false,
      ['sign']
    );
    
    const signatureBuffer = await window.crypto.subtle.sign(
      {
        name: 'ECDSA',
        hash: { name: 'SHA-256' }
      },
      privateKey,
      txData
    );
    
    transaction.signature = this._arrayBufferToBase64(signatureBuffer);
    
    // Generate transaction ID
    const txIdBuffer = await window.crypto.subtle.digest(
      'SHA-256',
      new TextEncoder().encode(JSON.stringify(transaction))
    );
    
    const txIdArray = Array.from(new Uint8Array(txIdBuffer));
    transaction.txId = txIdArray.map(b => b.toString(16).padStart(2, '0')).join('');
    
    return transaction;
  }
  
  /**
   * Create an identity transaction
   * @param {Object} params - Transaction parameters
   * @param {string} params.subjectQuid - Quid ID being defined
   * @param {string} params.domain - Identity domain
   * @param {string} [params.name] - Human-readable name
   * @param {string} [params.description] - Optional description
   * @param {Object} params.attributes - Custom attributes
   * @param {Object} quid - Quid object with private key for signing
   * @returns {Promise<Object>} Signed identity transaction
   */
  async createIdentityTransaction(params, quid) {
    if (!quid || !quid.privateKey) {
      throw new Error('Valid quid with private key is required for signing');
    }
    
    // Validate parameters
    if (!params.subjectQuid || !params.domain) {
      throw new Error('Missing required parameters: subjectQuid, domain');
    }
    
    // Create transaction object
    const transaction = {
      type: 'IDENTITY',
      timestamp: Math.floor(Date.now() / 1000),
      trustDomain: params.domain,
      signerQuid: quid.id,
      definerQuid: quid.id,
      subjectQuid: params.subjectQuid,
      schemaVersion: '1.0',
      updateNonce: 1 // Would be incremented for updates
    };
    
    if (params.name) {
      transaction.name = params.name;
    }
    
    if (params.description) {
      transaction.description = params.description;
    }
    
    if (params.attributes) {
      transaction.attributes = params.attributes;
    } else {
      transaction.attributes = {};
    }
    
    // Sign transaction
    const txData = new TextEncoder().encode(JSON.stringify(transaction));
    
    // Import private key for signing
    const privateKeyBuffer = this._base64ToArrayBuffer(quid.privateKey);
    const privateKey = await window.crypto.subtle.importKey(
      'pkcs8',
      privateKeyBuffer,
      {
        name: 'ECDSA',
        namedCurve: 'P-256'
      },
      false,
      ['sign']
    );
    
    const signatureBuffer = await window.crypto.subtle.sign(
      {
        name: 'ECDSA',
        hash: { name: 'SHA-256' }
      },
      privateKey,
      txData
    );
    
    transaction.signature = this._arrayBufferToBase64(signatureBuffer);
    
    // Generate transaction ID
    const txIdBuffer = await window.crypto.subtle.digest(
      'SHA-256',
      new TextEncoder().encode(JSON.stringify(transaction))
    );
    
    const txIdArray = Array.from(new Uint8Array(txIdBuffer));
    transaction.txId = txIdArray.map(b => b.toString(16).padStart(2, '0')).join('');
    
    return transaction;
  }
  
  /**
   * Create a title transaction
   * @param {Object} params - Transaction parameters
   * @param {string} params.assetQuid - Quid ID of the asset
   * @param {string} params.domain - Title domain
   * @param {Array} params.ownershipMap - Array of ownership stakes
   * @param {string} [params.prevTitleTxID] - Previous title transaction ID
   * @param {string} [params.titleType] - Type of title
   * @param {Object} quid - Quid object with private key for signing
   * @returns {Promise<Object>} Signed title transaction
   */
  async createTitleTransaction(params, quid) {
    if (!quid || !quid.privateKey) {
      throw new Error('Valid quid with private key is required for signing');
    }
    
    // Validate parameters
    if (!params.assetQuid || !params.domain || !params.ownershipMap) {
      throw new Error('Missing required parameters: assetQuid, domain, ownershipMap');
    }
    
    // Validate ownership map
    if (!Array.isArray(params.ownershipMap) || params.ownershipMap.length === 0) {
      throw new Error('OwnershipMap must be a non-empty array');
    }
    
    // Check total percentage
    const totalPercentage = params.ownershipMap.reduce(
      (sum, stake) => sum + (stake.percentage || 0), 0
    );
    
    if (Math.abs(totalPercentage - 100.0) > 0.001) {
      throw new Error(`Total ownership percentage must equal 100% (got ${totalPercentage}%)`);
    }
    
    // Create transaction object
    const transaction = {
      type: 'TITLE',
      timestamp: Math.floor(Date.now() / 1000),
      trustDomain: params.domain,
      signerQuid: quid.id,
      issuerQuid: quid.id,
      assetQuid: params.assetQuid,
      ownershipMap: params.ownershipMap,
      transferSigs: {}
    };
    
    if (params.prevTitleTxID) {
      transaction.prevTitleTxID = params.prevTitleTxID;
    }
    
    if (params.titleType) {
      transaction.titleType = params.titleType;
    }
    
    // Sign transaction
    const txData = new TextEncoder().encode(JSON.stringify(transaction));
    
    // Import private key for signing
    const privateKeyBuffer = this._base64ToArrayBuffer(quid.privateKey);
    const privateKey = await window.crypto.subtle.importKey(
      'pkcs8',
      privateKeyBuffer,
      {
        name: 'ECDSA',
        namedCurve: 'P-256'
      },
      false,
      ['sign']
    );
    
    const signatureBuffer = await window.crypto.subtle.sign(
      {
        name: 'ECDSA',
        hash: { name: 'SHA-256' }
      },
      privateKey,
      txData
    );
    
    transaction.signature = this._arrayBufferToBase64(signatureBuffer);
    
    // Generate transaction ID
    const txIdBuffer = await window.crypto.subtle.digest(
      'SHA-256',
      new TextEncoder().encode(JSON.stringify(transaction))
    );
    
    const txIdArray = Array.from(new Uint8Array(txIdBuffer));
    transaction.txId = txIdArray.map(b => b.toString(16).padStart(2, '0')).join('');
    
    return transaction;
  }
  
  /**
   * Submit a transaction to the Quidnug network
   * @param {Object} transaction - Signed transaction
   * @returns {Promise<Object>} Transaction result
   */
  async submitTransaction(transaction) {
    if (!transaction || !transaction.type || !transaction.txId) {
      throw new Error('Invalid transaction');
    }
    
    try {
      const nodeUrl = this._getHealthyNode();
      let endpoint;
      
      switch (transaction.type) {
        case 'TRUST':
          endpoint = 'transactions/trust';
          break;
        case 'IDENTITY':
          endpoint = 'transactions/identity';
          break;
        case 'TITLE':
          endpoint = 'transactions/title';
          break;
        default:
          throw new Error(`Unknown transaction type: ${transaction.type}`);
      }
      
      const response = await this._fetchWithRetry(`${nodeUrl}/api/${endpoint}`, {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json'
        },
        body: JSON.stringify(transaction)
      });
      
      return await this._parseResponse(response);
    } catch (error) {
      if (error.code) throw error;
      throw new Error(`Transaction submission failed: ${error.message}`);
    }
  }
  
  /**
   * Get relational trust level between quids.
   * 
   * Computes trust from the observer's perspective to the target through the
   * trust graph. Uses BFS to find the best path with multiplicative decay.
   * 
   * @param {string} observer - Quid ID of the observer (source of trust query)
   * @param {string} target - Quid ID of the target (destination of trust query)
   * @param {string} domain - Trust domain
   * @param {Object} [options] - Additional options
   * @param {number} [options.maxDepth=5] - Maximum trust path depth (hops)
   * @returns {Promise<Object>} Trust result with:
   *   - observer: the observer quid ID
   *   - target: the target quid ID
   *   - trustLevel: computed relational trust (0.0 to 1.0)
   *   - trustPath: array of quid IDs forming the best trust path
   *   - pathDepth: number of hops in the path
   *   - domain: the trust domain
   */
  async getTrustLevel(observer, target, domain, options = {}) {
    if (!observer || !target || !domain) {
      throw new Error('Missing required parameters: observer, target, domain');
    }
    
    try {
      const nodeUrl = this._getHealthyNode();
      let url = `${nodeUrl}/api/trust/${observer}/${target}?domain=${encodeURIComponent(domain)}`;
      
      if (options.maxDepth) {
        url += `&maxDepth=${options.maxDepth}`;
      }
      
      const response = await this._fetchWithRetry(url);
      return await this._parseResponse(response);
    } catch (error) {
      if (error.code === 'NOT_FOUND') {
        return { 
          observer,
          target,
          trustLevel: 0, 
          trustPath: [],
          pathDepth: 0,
          domain,
          message: 'No trust relationship found' 
        };
      }
      if (error.code) throw error;
      throw new Error(`Trust level query failed: ${error.message}`);
    }
  }
  
  /**
   * Get quid identity
   * @param {string} quidId - Quid ID
   * @param {string} [domain] - Optional specific domain
   * @returns {Promise<Object>} Identity information
   */
  async getIdentity(quidId, domain) {
    if (!quidId) {
      throw new Error('Missing required parameter: quidId');
    }
    
    try {
      const nodeUrl = this._getHealthyNode();
      let url = `${nodeUrl}/api/identity/${quidId}`;
      
      if (domain) {
        url += `?domain=${encodeURIComponent(domain)}`;
      }
      
      const response = await this._fetchWithRetry(url);
      return await this._parseResponse(response);
    } catch (error) {
      if (error.code === 'NOT_FOUND') {
        return null;
      }
      if (error.code) throw error;
      throw new Error(`Identity query failed: ${error.message}`);
    }
  }
  
  /**
   * Get asset ownership
   * @param {string} assetId - Asset quid ID
   * @param {string} [domain] - Optional specific domain
   * @returns {Promise<Object>} Ownership information
   */
  async getAssetOwnership(assetId, domain) {
    if (!assetId) {
      throw new Error('Missing required parameter: assetId');
    }
    
    try {
      const nodeUrl = this._getHealthyNode();
      let url = `${nodeUrl}/api/title/${assetId}`;
      
      if (domain) {
        url += `?domain=${encodeURIComponent(domain)}`;
      }
      
      const response = await this._fetchWithRetry(url);
      return await this._parseResponse(response);
    } catch (error) {
      if (error.code === 'NOT_FOUND') {
        return null;
      }
      if (error.code) throw error;
      throw new Error(`Asset ownership query failed: ${error.message}`);
    }
  }
  
  /**
   * Find trust path between quids.
   * 
   * This is a convenience wrapper around getTrustLevel() that returns
   * path-focused results. The relational trust API always computes and
   * returns the best path.
   * 
   * @param {string} observer - Observer quid ID (who is asking about trust)
   * @param {string} target - Target quid ID (who we want to know about)
   * @param {string} domain - Trust domain
   * @param {Object} [options] - Additional options
   * @param {number} [options.maxDepth=5] - Maximum path depth (hops)
   * @param {number} [options.minTrustLevel] - Minimum trust level threshold
   * @returns {Promise<Object>} Trust path result with:
   *   - found: boolean indicating if a path exists
   *   - trustLevel: computed relational trust along the path
   *   - path: array of quid IDs from observer to target
   *   - pathDepth: number of hops
   */
  async findTrustPath(observer, target, domain, options = {}) {
    if (!observer || !target || !domain) {
      throw new Error('Missing required parameters: observer, target, domain');
    }
    
    try {
      const result = await this.getTrustLevel(observer, target, domain, {
        maxDepth: options.maxDepth
      });
      
      const found = result.trustPath && result.trustPath.length > 0;
      const meetsThreshold = options.minTrustLevel === undefined || 
                             result.trustLevel >= options.minTrustLevel;
      
      return {
        found: found && meetsThreshold,
        trustLevel: result.trustLevel,
        path: result.trustPath || [],
        pathDepth: result.pathDepth || 0
      };
    } catch (error) {
      throw new Error(`Trust path query failed: ${error.message}`);
    }
  }
  
  /**
   * Query relational trust using a structured query object.
   * 
   * This method POSTs to /api/trust/query and is useful for programmatic
   * trust queries where parameters are built dynamically.
   * 
   * @param {Object} query - Relational trust query
   * @param {string} query.observer - Quid ID of the observer
   * @param {string} query.target - Quid ID of the target
   * @param {string} [query.domain] - Trust domain (defaults to 'default')
   * @param {number} [query.maxDepth=5] - Maximum path depth
   * @returns {Promise<Object>} Relational trust result
   */
  async queryRelationalTrust({ observer, target, domain, maxDepth }) {
    if (!observer || !target) {
      throw new Error('Missing required parameters: observer, target');
    }
    
    try {
      const nodeUrl = this._getHealthyNode();
      
      const response = await this._fetchWithRetry(`${nodeUrl}/api/trust/query`, {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json'
        },
        body: JSON.stringify({
          observer,
          target,
          domain: domain || 'default',
          maxDepth: maxDepth || 5
        })
      });
      
      return await this._parseResponse(response);
    } catch (error) {
      if (error.code) throw error;
      throw new Error(`Relational trust query failed: ${error.message}`);
    }
  }
  
  /**
   * Compute transitive trust client-side from a cached trust graph.
   * 
   * This is useful for offline computation when the full trust graph
   * (or a relevant subset) has been cached locally. Uses BFS with
   * multiplicative decay, matching the server-side algorithm.
   * 
   * @param {Object} trustGraph - Map of truster -> { trustee -> trustLevel }
   * @param {string} observer - Observer quid ID
   * @param {string} target - Target quid ID
   * @param {number} [maxDepth=5] - Maximum path depth
   * @returns {Object} Trust result with trustLevel and trustPath
   */
  computeTransitiveTrust(trustGraph, observer, target, maxDepth = 5) {
    if (observer === target) {
      return { trustLevel: 1.0, trustPath: [observer] };
    }
    
    const queue = [{
      quid: observer,
      path: [observer],
      trust: 1.0
    }];
    
    let bestTrust = 0;
    let bestPath = [];
    
    while (queue.length > 0) {
      const current = queue.shift();
      const trustees = trustGraph[current.quid] || {};
      
      for (const [trustee, edgeTrust] of Object.entries(trustees)) {
        if (current.path.includes(trustee)) {
          continue;
        }
        
        const pathTrust = current.trust * edgeTrust;
        const newPath = [...current.path, trustee];
        
        if (trustee === target) {
          if (pathTrust > bestTrust) {
            bestTrust = pathTrust;
            bestPath = newPath;
          }
          continue;
        }
        
        if (current.path.length < maxDepth) {
          queue.push({
            quid: trustee,
            path: newPath,
            trust: pathTrust
          });
        }
      }
    }
    
    return {
      trustLevel: bestTrust,
      trustPath: bestPath,
      pathDepth: bestPath.length > 1 ? bestPath.length - 1 : 0
    };
  }
  
  /**
   * Get blocks from the blockchain with pagination
   * @param {Object} [options] - Query options
   * @param {number} [options.limit=50] - Maximum number of items to return
   * @param {number} [options.offset=0] - Number of items to skip
   * @returns {Promise<Object>} Paginated blocks response with data and pagination metadata
   */
  async getBlocks(options = {}) {
    try {
      const nodeUrl = this._getHealthyNode();
      let url = `${nodeUrl}/api/blocks`;

      const params = new URLSearchParams();
      if (options.limit !== undefined) params.append('limit', options.limit);
      if (options.offset !== undefined) params.append('offset', options.offset);

      if (params.toString()) {
        url += `?${params.toString()}`;
      }

      const response = await this._fetchWithRetry(url);
      return await this._parseResponse(response);
    } catch (error) {
      if (error.code) throw error;
      throw new Error(`Blocks query failed: ${error.message}`);
    }
  }

  /**
   * Get known nodes with pagination
   * @param {Object} [options] - Query options
   * @param {number} [options.limit=50] - Maximum number of items to return
   * @param {number} [options.offset=0] - Number of items to skip
   * @returns {Promise<Object>} Paginated nodes response with data and pagination metadata
   */
  async getNodes(options = {}) {
    try {
      const nodeUrl = this._getHealthyNode();
      let url = `${nodeUrl}/api/nodes`;

      const params = new URLSearchParams();
      if (options.limit !== undefined) params.append('limit', options.limit);
      if (options.offset !== undefined) params.append('offset', options.offset);

      if (params.toString()) {
        url += `?${params.toString()}`;
      }

      const response = await this._fetchWithRetry(url);
      return await this._parseResponse(response);
    } catch (error) {
      if (error.code) throw error;
      throw new Error(`Nodes query failed: ${error.message}`);
    }
  }

  /**
   * Get pending transactions with pagination
   * @param {Object} [options] - Query options
   * @param {number} [options.limit=50] - Maximum number of items to return
   * @param {number} [options.offset=0] - Number of items to skip
   * @returns {Promise<Object>} Paginated transactions response with data and pagination metadata
   */
  async getPendingTransactions(options = {}) {
    try {
      const nodeUrl = this._getHealthyNode();
      let url = `${nodeUrl}/api/transactions`;

      const params = new URLSearchParams();
      if (options.limit !== undefined) params.append('limit', options.limit);
      if (options.offset !== undefined) params.append('offset', options.offset);

      if (params.toString()) {
        url += `?${params.toString()}`;
      }

      const response = await this._fetchWithRetry(url);
      return await this._parseResponse(response);
    } catch (error) {
      if (error.code) throw error;
      throw new Error(`Transactions query failed: ${error.message}`);
    }
  }

  /**
   * Query the trust registry with optional pagination
   * @param {Object} [options] - Query options
   * @param {string} [options.truster] - Filter by truster quid ID
   * @param {string} [options.trustee] - Filter by trustee quid ID
   * @param {number} [options.limit=50] - Maximum number of items to return (for full registry query)
   * @param {number} [options.offset=0] - Number of items to skip (for full registry query)
   * @returns {Promise<Object>} Trust registry data (paginated when no filters applied)
   */
  async queryTrustRegistry(options = {}) {
    try {
      const nodeUrl = this._getHealthyNode();
      let url = `${nodeUrl}/api/registry/trust`;

      const params = new URLSearchParams();
      if (options.truster) params.append('truster', options.truster);
      if (options.trustee) params.append('trustee', options.trustee);
      if (options.limit !== undefined) params.append('limit', options.limit);
      if (options.offset !== undefined) params.append('offset', options.offset);

      if (params.toString()) {
        url += `?${params.toString()}`;
      }

      const response = await this._fetchWithRetry(url);
      return await this._parseResponse(response);
    } catch (error) {
      if (error.code) throw error;
      throw new Error(`Trust registry query failed: ${error.message}`);
    }
  }

  /**
   * Query the identity registry with optional pagination
   * @param {Object} [options] - Query options
   * @param {string} [options.quidId] - Filter by specific quid ID
   * @param {number} [options.limit=50] - Maximum number of items to return (for full registry query)
   * @param {number} [options.offset=0] - Number of items to skip (for full registry query)
   * @returns {Promise<Object>} Identity registry data (paginated when no quidId filter applied)
   */
  async queryIdentityRegistry(options = {}) {
    try {
      const nodeUrl = this._getHealthyNode();
      let url = `${nodeUrl}/api/registry/identity`;

      const params = new URLSearchParams();
      if (options.quidId) params.append('quid_id', options.quidId);
      if (options.limit !== undefined) params.append('limit', options.limit);
      if (options.offset !== undefined) params.append('offset', options.offset);

      if (params.toString()) {
        url += `?${params.toString()}`;
      }

      const response = await this._fetchWithRetry(url);
      return await this._parseResponse(response);
    } catch (error) {
      if (error.code) throw error;
      throw new Error(`Identity registry query failed: ${error.message}`);
    }
  }

  /**
   * Query the title registry with optional pagination
   * @param {Object} [options] - Query options
   * @param {string} [options.assetId] - Filter by specific asset ID
   * @param {string} [options.ownerId] - Filter by owner ID
   * @param {number} [options.limit=50] - Maximum number of items to return (for full registry query)
   * @param {number} [options.offset=0] - Number of items to skip (for full registry query)
   * @returns {Promise<Object>} Title registry data (paginated when no filters applied)
   */
  async queryTitleRegistry(options = {}) {
    try {
      const nodeUrl = this._getHealthyNode();
      let url = `${nodeUrl}/api/registry/title`;

      const params = new URLSearchParams();
      if (options.assetId) params.append('asset_id', options.assetId);
      if (options.ownerId) params.append('owner_id', options.ownerId);
      if (options.limit !== undefined) params.append('limit', options.limit);
      if (options.offset !== undefined) params.append('offset', options.offset);

      if (params.toString()) {
        url += `?${params.toString()}`;
      }

      const response = await this._fetchWithRetry(url);
      return await this._parseResponse(response);
    } catch (error) {
      if (error.code) throw error;
      throw new Error(`Title registry query failed: ${error.message}`);
    }
  }

  /**
   * Query a specific trust domain
   * @param {string} domain - Domain to query
   * @param {string} type - Query type (trust, identity, title)
   * @param {string} param - Query parameter
   * @returns {Promise<Object>} Query results
   */
  async queryDomain(domain, type, param) {
    if (!domain || !type || !param) {
      throw new Error('Missing required parameters: domain, type, param');
    }
    
    try {
      const nodeUrl = this._getHealthyNode();
      const url = `${nodeUrl}/api/domains/${domain}/query?type=${type}&param=${encodeURIComponent(param)}`;
      
      const response = await this._fetchWithRetry(url);
      return await this._parseResponse(response);
    } catch (error) {
      if (error.code) throw error;
      throw new Error(`Domain query failed: ${error.message}`);
    }
  }
  
  /**
   * Find nodes that manage a specific domain
   * @param {string} domain - Domain to find nodes for
   * @returns {Promise<Array>} List of nodes managing the domain
   */
  async findNodesForDomain(domain) {
    if (!domain) {
      throw new Error('Missing required parameter: domain');
    }
    
    try {
      // Get list of all known nodes
      const nodeUrl = this._getHealthyNode();
      const response = await this._fetchWithRetry(`${nodeUrl}/api/nodes`);
      const result = await this._parseResponse(response);
      
      const nodes = result.data || [];
      
      // Filter nodes that manage this domain
      return nodes.filter(node => {
        if (node.managedDomains.includes(domain)) {
          return true;
        }
        
        // Check for parent domains
        const domainParts = domain.split('.');
        while (domainParts.length > 1) {
          domainParts.shift();
          const parentDomain = domainParts.join('.');
          if (node.managedDomains.includes(parentDomain)) {
            return true;
          }
        }
        
        return false;
      });
    } catch (error) {
      throw new Error(`Node discovery failed: ${error.message}`);
    }
  }
  
  /**
   * Parse API response and extract data or throw error
   * @private
   * @param {Response} response - Fetch response object
   * @returns {Promise<Object>} Parsed data from response
   * @throws {Error} With code property on API error
   */
  async _parseResponse(response) {
    const json = await response.json();
    
    if (json.success) {
      return json.data;
    }
    
    const error = new Error(json.error?.message || 'API request failed');
    error.code = json.error?.code || 'UNKNOWN_ERROR';
    throw error;
  }
  
  /**
   * Build canonical data for event transaction signing
   * @private
   * @param {Object} tx - Transaction object (without signature)
   * @returns {Uint8Array} Encoded transaction data for signing
   */
  _buildEventSignatureData(tx) {
    const txCopy = { ...tx };
    delete txCopy.signature;
    delete txCopy.txId;
    return new TextEncoder().encode(JSON.stringify(txCopy));
  }
  
  /**
   * Create an event transaction for event streaming.
   * 
   * Events are append-only records associated with a subject (QUID or TITLE).
   * The signing quid must be the owner of the subject.
   * 
   * @param {Object} params - Transaction parameters
   * @param {string} params.subjectId - Quid ID of the subject (entity the event is about)
   * @param {string} params.subjectType - Type of subject ('QUID' or 'TITLE')
   * @param {string} params.eventType - Type of event (max 64 chars)
   * @param {Object} [params.payload] - Event payload data (required if no payloadCID)
   * @param {string} [params.payloadCID] - IPFS CID of payload (required if no payload)
   * @param {string} params.domain - Trust domain
   * @param {number} [params.sequence] - Sequence number (auto-generated if not provided)
   * @param {Object} quid - Quid object with private key for signing (must be subject owner)
   * @returns {Promise<Object>} Result with txId and sequence
   */
  async createEventTransaction(params, quid) {
    if (!quid || !quid.privateKey) {
      throw new Error('Valid quid with private key is required for signing');
    }
    
    if (!params.subjectId || !params.subjectType || !params.eventType || !params.domain) {
      throw new Error('Missing required parameters: subjectId, subjectType, eventType, domain');
    }
    
    if (!params.payload && !params.payloadCID) {
      throw new Error('Either payload or payloadCID must be provided');
    }
    
    if (params.subjectType !== 'QUID' && params.subjectType !== 'TITLE') {
      throw new Error('subjectType must be "QUID" or "TITLE"');
    }
    
    // Auto-generate sequence if not provided
    let sequence = params.sequence;
    if (sequence === undefined || sequence === null) {
      try {
        const stream = await this.getEventStream(params.subjectId, params.domain);
        if (stream) {
          sequence = stream.latestSequence + 1;
        } else {
          sequence = 1;
        }
      } catch (error) {
        sequence = 1;
      }
    }
    
    const transaction = {
      type: 'EVENT',
      timestamp: Math.floor(Date.now() / 1000),
      trustDomain: params.domain,
      subjectId: params.subjectId,
      subjectType: params.subjectType,
      eventType: params.eventType,
      sequence: sequence
    };
    
    if (params.payload) {
      transaction.payload = params.payload;
    }
    
    if (params.payloadCID) {
      transaction.payloadCid = params.payloadCID;
    }
    
    // Sign transaction
    const txData = this._buildEventSignatureData(transaction);
    
    const privateKeyBuffer = this._base64ToArrayBuffer(quid.privateKey);
    const privateKey = await window.crypto.subtle.importKey(
      'pkcs8',
      privateKeyBuffer,
      {
        name: 'ECDSA',
        namedCurve: 'P-256'
      },
      false,
      ['sign']
    );
    
    const signatureBuffer = await window.crypto.subtle.sign(
      {
        name: 'ECDSA',
        hash: { name: 'SHA-256' }
      },
      privateKey,
      txData
    );
    
    transaction.signature = this._arrayBufferToBase64(signatureBuffer);
    
    if (quid.publicKey) {
      transaction.publicKey = quid.publicKey;
    }
    
    // Submit via POST /api/v1/events
    try {
      const nodeUrl = this._getHealthyNode();
      const response = await this._fetchWithRetry(`${nodeUrl}/api/v1/events`, {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json'
        },
        body: JSON.stringify(transaction)
      });
      
      const result = await this._parseResponse(response);
      return {
        txId: result.id,
        sequence: result.sequence
      };
    } catch (error) {
      if (error.code) throw error;
      throw new Error(`Event transaction submission failed: ${error.message}`);
    }
  }
  
  /**
   * Get event stream metadata for a subject
   * @param {string} subjectId - Subject quid ID
   * @param {string} [domain] - Trust domain (optional)
   * @returns {Promise<Object|null>} Stream metadata or null if not found
   */
  async getEventStream(subjectId, domain) {
    if (!subjectId) {
      throw new Error('Missing required parameter: subjectId');
    }
    
    try {
      const nodeUrl = this._getHealthyNode();
      let url = `${nodeUrl}/api/v1/streams/${subjectId}`;
      
      if (domain) {
        url += `?domain=${encodeURIComponent(domain)}`;
      }
      
      const response = await this._fetchWithRetry(url);
      return await this._parseResponse(response);
    } catch (error) {
      if (error.code === 'NOT_FOUND') {
        return null;
      }
      if (error.code) throw error;
      throw new Error(`Event stream query failed: ${error.message}`);
    }
  }
  
  /**
   * Get paginated events for a stream
   * @param {string} subjectId - Subject quid ID
   * @param {Object} [options] - Query options
   * @param {number} [options.limit] - Maximum events to return
   * @param {number} [options.offset] - Number of events to skip
   * @param {string} [options.domain] - Trust domain
   * @returns {Promise<Object>} Object with events array and pagination metadata
   */
  async getStreamEvents(subjectId, options = {}) {
    if (!subjectId) {
      throw new Error('Missing required parameter: subjectId');
    }
    
    try {
      const nodeUrl = this._getHealthyNode();
      let url = `${nodeUrl}/api/v1/streams/${subjectId}/events`;
      
      const params = new URLSearchParams();
      if (options.limit !== undefined) params.append('limit', options.limit);
      if (options.offset !== undefined) params.append('offset', options.offset);
      if (options.domain) params.append('domain', options.domain);
      
      if (params.toString()) {
        url += `?${params.toString()}`;
      }
      
      const response = await this._fetchWithRetry(url);
      const result = await this._parseResponse(response);
      
      return {
        events: result.data || [],
        pagination: result.pagination || {}
      };
    } catch (error) {
      if (error.code) throw error;
      throw new Error(`Stream events query failed: ${error.message}`);
    }
  }
  
  /**
   * Pin content to IPFS
   * @param {string|ArrayBuffer} content - Content to pin
   * @returns {Promise<string>} CID of pinned content
   */
  async pinToIPFS(content) {
    if (!content) {
      throw new Error('Content is required');
    }
    
    try {
      const nodeUrl = this._getHealthyNode();
      
      let body;
      const headers = {
        'Content-Type': 'application/octet-stream'
      };
      
      if (content instanceof ArrayBuffer) {
        body = this._arrayBufferToBase64(content);
        headers['Content-Transfer-Encoding'] = 'base64';
      } else if (typeof content === 'string') {
        body = content;
      } else {
        throw new Error('Content must be a string or ArrayBuffer');
      }
      
      const response = await this._fetchWithRetry(`${nodeUrl}/api/v1/ipfs/pin`, {
        method: 'POST',
        headers: headers,
        body: body
      });
      
      const result = await this._parseResponse(response);
      return result.cid;
    } catch (error) {
      if (error.code) throw error;
      throw new Error(`IPFS pin failed: ${error.message}`);
    }
  }
  
  /**
   * Get content from IPFS by CID
   * @param {string} cid - Content identifier
   * @returns {Promise<ArrayBuffer>} Content as ArrayBuffer
   */
  async getFromIPFS(cid) {
    if (!cid) {
      throw new Error('CID is required');
    }
    
    try {
      const nodeUrl = this._getHealthyNode();
      const response = await this._fetchWithRetry(`${nodeUrl}/api/v1/ipfs/${cid}`);
      
      if (!response.ok) {
        const errorData = await response.json();
        const error = new Error(errorData.error?.message || 'IPFS retrieval failed');
        error.code = errorData.error?.code || 'IPFS_ERROR';
        throw error;
      }
      
      return await response.arrayBuffer();
    } catch (error) {
      if (error.code) throw error;
      throw new Error(`IPFS retrieval failed: ${error.message}`);
    }
  }
}

// Example usage in a browser environment
if (typeof window !== 'undefined') {
  window.QuidnugClient = QuidnugClient;
}

// CommonJS export
if (typeof module !== 'undefined' && module.exports) {
  module.exports = QuidnugClient;
}

// ES module export
export default QuidnugClient;
