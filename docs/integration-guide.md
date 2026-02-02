# Quidnug Integration Guide

This guide serves as a comprehensive resource for developers looking to build applications on top of the Quidnug platform. Quidnug provides a cryptographic identity, trust, and ownership infrastructure that can be leveraged by various applications.

## Connecting to a Node

Before using the client library, ensure you have a running Quidnug node:

```bash
# Local development
SEED_NODES='[]' ./quidnug-node

# The node will be available at http://localhost:8080
```

### Node Health Check

Always verify node connectivity before operations:

```javascript
const response = await fetch('http://localhost:8080/api/health');
const health = await response.json();
// { status: "ok", node_id: "...", uptime: 123, version: "1.0.0" }
```

## Core Concepts

### Quids

A **quid** is a cryptographic identity (similar to a Bitcoin wallet) with:
- A public/private key pair
- A unique identifier derived from the public key
- The ability to sign transactions and messages
- A reputation based on trust from other quids

Every entity in the Quidnug system (people, organizations, assets, documents) is represented as a quid.

### Trust Relationships

Trust is established between quids with these key characteristics:

- **Relational, not absolute**: Trust is always computed from an observer's perspective to a target. There is no global "trust score" for any quid—the same quid may have different trust levels when viewed by different observers.
- **Explicit trust levels** (0.0 to 1.0) define direct relationships
- **Domain-specific** (e.g., `medical.credentials`, `property.texas`)
- **Can have expiration dates**
- **Transitive with multiplicative decay**: Trust propagates through the network. If A trusts B at 0.8 and B trusts C at 0.7, then A's transitive trust in C is 0.8 × 0.7 = 0.56.

### Trust Domains

Domains organize trust hierarchically:
- Similar to DNS (e.g., `real-estate.travis-county.texas.us`)
- Each domain has its own validation rules
- Independent authority at each level
- Specialized functionality for different applications

### Transactions

Quidnug supports three core transaction types:

1. **Trust Transactions**: Establish trust between quids
2. **Identity Transactions**: Define attributes for quids
3. **Title Transactions**: Establish ownership relationships between quids

## Getting Started with Client Development

### 1. Creating a Quid

To interact with Quidnug, your application needs to create or import quids:

```javascript
// Example in JavaScript
const quidnugClient = new QuidnugClient();

// Generate a new quid
const userQuid = await quidnugClient.generateQuid();

// Save the quid securely (NEVER share the private key)
localStorage.setItem('userQuidPublicKey', userQuid.publicKey);
localStorage.setItem('userQuidPrivateKey', userQuid.privateKey);
console.log("Your Quid ID:", userQuid.id);

// Import an existing quid
const importedQuid = await quidnugClient.importQuid(privateKey);
```

### 2. Connecting to Quidnug Nodes

Your application needs to connect to one or more Quidnug nodes:

```javascript
// Connect to a node
await quidnugClient.connectToNode('https://quidnug-node.example.com');

// Add backup nodes
await quidnugClient.addBackupNode('https://quidnug-node2.example.com');
await quidnugClient.addBackupNode('https://quidnug-node3.example.com');

// Find nodes for specific domains
const propertyNodes = await quidnugClient.findNodesForDomain('real-estate.travis-county.texas.us');
```

### 3. Submitting Transactions

Creating and submitting transactions:

```javascript
// Create a trust transaction
const trustTx = await quidnugClient.createTrustTransaction({
  truster: userQuid.id,          // Your quid
  trustee: 'quid_id_to_trust',   // Target quid to trust
  domain: 'example.com',         // Trust domain
  trustLevel: 0.9,               // Trust level (0.0 to 1.0)
  validUntil: 1672531200000      // Optional expiration (milliseconds since epoch)
});

// Sign and submit the transaction
const txResult = await quidnugClient.submitTransaction(trustTx, userQuid);
console.log("Transaction ID:", txResult.txId);

// Create an identity transaction
const identityTx = await quidnugClient.createIdentityTransaction({
  subjectQuid: 'target_quid_id',
  domain: 'example.com',
  name: 'Example Entity',
  attributes: {
    type: 'organization',
    location: 'Austin, TX',
    website: 'https://example.com'
  }
});

// Create a title transaction
const titleTx = await quidnugClient.createTitleTransaction({
  assetQuid: 'asset_quid_id',
  domain: 'real-estate.travis-county.texas.us',
  ownershipMap: [
    { ownerId: 'owner1_quid_id', percentage: 75.0 },
    { ownerId: 'owner2_quid_id', percentage: 25.0 }
  ]
});
```

### 4. Querying the Quidnug Network

Retrieving information:

```javascript
// Get relational trust from observer to target
// Trust is always computed from YOUR perspective (the observer)
const result = await quidnugClient.getTrustLevel(
  observerQuidId,  // Your quid (who is asking)
  targetQuidId,    // The quid you want to assess
  'example.com',
  { maxDepth: 5 }
);
console.log("Trust level:", result.trustLevel);
console.log("Trust path:", result.trustPath);
console.log("Path depth:", result.pathDepth);

// Get quid identity
const identity = await quidnugClient.getIdentity('quid_id', 'example.com');
console.log("Identity:", identity);

// Get asset ownership
const ownership = await quidnugClient.getAssetOwnership('asset_quid_id', 'example.com');
console.log("Owners:", ownership.ownershipMap);
```

### Understanding Relational Trust

Trust in Quidnug is **relational**, not absolute. This is a fundamental design principle:

#### Key Concepts

1. **Observer Perspective**: Every trust query requires an observer (who is asking) and a target (who is being assessed). The same target quid may have different trust levels when queried by different observers.

2. **Multiplicative Decay**: Trust decays as it propagates through intermediaries:
   ```
   A → B (0.8) → C (0.7) → D (0.9)
   A's trust in D = 0.8 × 0.7 × 0.9 = 0.504
   ```

3. **Best Path Selection**: When multiple paths exist between observer and target, the algorithm returns the path with the **maximum** trust level.

4. **Depth Limiting**: The `maxDepth` parameter (default 5) limits how far the algorithm searches. Deeper paths have more decay and are less likely to yield high trust.

5. **Special Cases**:
   - Same entity: An observer has full trust (1.0) in itself
   - No path: Returns trust level 0.0

#### Example: Computing Relational Trust

```javascript
// Your application's quid is the observer
const myQuidId = 'a1b2c3d4e5f6g7h8';

// Query trust in a potential business partner
const partnerTrust = await quidnugClient.getTrustLevel(
  myQuidId,           // Observer: your perspective
  'partner_quid_id',  // Target: who you're assessing
  'business.example.com',
  { maxDepth: 4 }
);

if (partnerTrust.trustLevel >= 0.7) {
  console.log("Partner is trusted via:", partnerTrust.trustPath);
  // e.g., ["a1b2c3d4", "colleague_quid", "partner_quid_id"]
} else if (partnerTrust.trustLevel > 0) {
  console.log("Partner has low trust:", partnerTrust.trustLevel);
} else {
  console.log("No trust path found to partner");
}

// Alternative: POST query for more complex scenarios
const result = await quidnugClient.queryRelationalTrust({
  observer: myQuidId,
  target: 'partner_quid_id',
  domain: 'business.example.com',
  maxDepth: 5
});
```

## Building Applications with Quidnug

### Authentication & Authorization

Quidnug can serve as an identity and authorization system. Note that trust is always relational—your service assesses trust from its own perspective:

```javascript
// Authenticate a user with their quid
async function authenticateUser(quidId, challenge, signature) {
  // Verify the user signed the challenge with their quid's private key
  const isValid = await quidnugClient.verifySignature(quidId, challenge, signature);
  
  if (isValid) {
    // Check if YOUR SERVICE trusts the user's quid
    // The observer is your service, the target is the user
    const trustResult = await quidnugClient.getTrustLevel(
      'your_service_quid_id',  // Observer: your service's perspective
      quidId,                   // Target: the user being authenticated
      'your-service.com',
      { maxDepth: 5 }
    );
    
    if (trustResult.trustLevel >= 0.7) {
      return { 
        authenticated: true, 
        trustLevel: trustResult.trustLevel,
        trustPath: trustResult.trustPath  // Shows how trust was established
      };
    }
  }
  
  return { authenticated: false };
}
```

### Credential Verification

For applications that need to verify credentials. Trust in the issuer is computed relationally from your service's perspective:

```javascript
// Verify a credential
async function verifyCredential(credentialQuidId, issuerQuidId, domain) {
  // Check if the credential exists and is defined by the expected issuer
  const credential = await quidnugClient.getIdentity(credentialQuidId, domain);
  
  if (credential && credential.definerQuid === issuerQuidId) {
    // Check if YOUR SERVICE trusts the issuer
    // Different services may have different trust levels for the same issuer
    const trustResult = await quidnugClient.getTrustLevel(
      'your_service_quid_id',  // Observer: your service
      issuerQuidId,            // Target: the credential issuer
      domain,
      { maxDepth: 4 }
    );
    
    if (trustResult.trustLevel >= 0.8) {
      return { 
        verified: true, 
        credential,
        issuerTrust: trustResult.trustLevel,
        trustPath: trustResult.trustPath
      };
    }
  }
  
  return { verified: false };
}
```

### Asset Tracking

For applications that track asset ownership:

```javascript
// Transfer asset ownership
async function transferAssetOwnership(assetQuidId, newOwners, domain, userQuid) {
  // Get current ownership
  const currentTitle = await quidnugClient.getAssetOwnership(assetQuidId, domain);
  
  // Create a new title transaction
  const titleTx = await quidnugClient.createTitleTransaction({
    assetQuid: assetQuidId,
    domain: domain,
    ownershipMap: newOwners,
    prevTitleTxID: currentTitle.txId
  });
  
  // Sign and submit
  return await quidnugClient.submitTransaction(titleTx, userQuid);
}
```

## Best Practices

### Security Considerations

1. **Private Key Management**: Never expose private keys. Use secure storage solutions.
2. **Trust Verification**: Always verify trust paths before relying on them.
3. **Multiple Node Connections**: Connect to multiple nodes for redundancy.
4. **Signature Verification**: Always verify signatures on received data.

### Performance Optimization

1. **Caching**: Cache frequently accessed trust relationships and identities.
2. **Batching**: Combine multiple related transactions where possible.
3. **Trust Path Optimization**: Limit trust path depth for time-sensitive operations.

### Integration Patterns

1. **Trust Bridge Pattern**: Create bridge quids between different trust domains.
2. **Delegate Pattern**: Allow users to delegate trust to specialized quids.
3. **Trust Threshold Pattern**: Require multiple trusted quids to validate important actions.

## Advanced Features

### Multi-signature Capabilities

For high-security applications:

```javascript
// Create a multi-signature requirement
const multiSigTitle = await quidnugClient.createTitleTransaction({
  assetQuid: 'high_value_asset',
  domain: 'security.example.com',
  ownershipMap: [{ ownerId: 'owner_quid', percentage: 100.0 }],
  requireSignatures: ['trustee1', 'trustee2', 'trustee3'],
  requiredSignatureCount: 2  // At least 2 of 3 must sign
});
```

### Trust Domain Governance

For managing domain rules:

```javascript
// Create a governance proposal
const proposal = await quidnugClient.createGovernanceProposal({
  domain: 'example.com',
  proposalType: 'UPDATE_TRUST_THRESHOLD',
  changes: { trustThreshold: 0.8 },
  votingDeadline: Date.now() + (7 * 24 * 60 * 60 * 1000) // 1 week from now
});

// Vote on a proposal
await quidnugClient.voteOnProposal(proposal.id, true, userQuid);
```

## Examples of Quidnug Applications

1. **Decentralized Identity Systems**: Self-sovereign identity with verifiable credentials
2. **Supply Chain Tracking**: Trace asset provenance through multiple owners
3. **Professional Credential Verification**: Verify licenses and certifications
4. **Decentralized Governance**: Voting systems based on trust relationships
5. **Access Control Systems**: Permission management based on trust levels
6. **Resource Allocation**: Distribute resources based on trust scores
7. **Reputation Systems**: Build context-specific reputation metrics

## Working with Proof of Trust

Quidnug uses **Proof of Trust** consensus, where block validation is subjective—each node validates blocks based on its own relational trust in the validator. This has important implications for application design.

### Understanding Confidence Levels

When querying trust with `includeUnverified=true`, the response includes a confidence level:

```javascript
const result = await quidnugClient.queryRelationalTrust({
  observer: myQuidId,
  target: partnerQuidId,
  domain: 'business.example.com',
  includeUnverified: true  // Get enhanced result with confidence
});

console.log("Trust level:", result.trustLevel);
console.log("Confidence:", result.confidence);
console.log("Unverified hops:", result.unverifiedHops);

// Handle different confidence levels
switch (result.confidence) {
  case 'high':
    // All edges verified - safe for high-stakes decisions
    allowFullAccess();
    break;
  case 'medium':
    // Some unverified edges - use caution
    allowLimitedAccess();
    break;
  case 'low':
    // Significant unverified data - require additional verification
    requestAdditionalVerification();
    break;
}
```

### When to Use includeUnverified

| Scenario | includeUnverified | Rationale |
|----------|-------------------|-----------|
| Authentication | `false` | High-stakes decision needs verified edges only |
| Large transaction approval | `false` | Financial risk requires high assurance |
| Discovery / exploration | `true` | Finding potential connections is lower risk |
| Displaying trust context | `true` | Users benefit from seeing all connections |
| Initial onboarding | `true` | New users have few verified connections |

```javascript
// High-stakes: authentication - verified edges only
async function authenticateUser(userQuidId) {
  const trust = await quidnugClient.getTrustLevel(
    serviceQuidId,
    userQuidId,
    'auth.example.com',
    { includeUnverified: false }  // Strict verification
  );
  return trust.trustLevel >= 0.8;
}

// Discovery: finding potential partners - include unverified
async function discoverPartners(category) {
  const candidates = await searchByCategory(category);
  
  for (const candidate of candidates) {
    const trust = await quidnugClient.queryRelationalTrust({
      observer: myQuidId,
      target: candidate.quidId,
      includeUnverified: true  // Cast a wider net
    });
    
    candidate.trustLevel = trust.trustLevel;
    candidate.confidence = trust.confidence;
    candidate.verificationGaps = trust.verificationGaps;
  }
  
  return candidates;
}
```

### Working with Tentative Blocks

Tentative blocks are cryptographically valid blocks from validators your node doesn't fully trust. They're stored separately and may be promoted later.

```javascript
// Get tentative blocks for a domain
const response = await fetch(
  'http://localhost:8080/api/blocks/tentative/example.com'
);
const { blocks, count } = await response.json();

console.log(`${count} tentative blocks for example.com`);

for (const block of blocks) {
  console.log(`Block ${block.index} from validator ${block.trustProof.validatorId}`);
  console.log(`  Validator trust: ${block.trustProof.validatorTrustInCreator}`);
  console.log(`  Transactions: ${block.transactions.length}`);
}
```

#### Promoting Tentative Blocks

When you establish trust in a new validator, tentative blocks from that validator may be promoted:

```javascript
// 1. Establish trust in a validator
await quidnugClient.submitTransaction({
  type: 'TRUST',
  truster: myQuidId,
  trustee: newValidatorQuid,
  trustLevel: 0.85,
  domain: 'example.com'
});

// 2. Node will automatically re-evaluate tentative blocks
// Or explicitly trigger re-evaluation via node admin API
```

### Handling Subjective Validation

Different nodes may have different views of the blockchain based on their trust relationships. This is **by design**, not a bug.

#### Application Design Implications

1. **Don't assume global consensus**: Your view of "valid" blocks depends on your trust relationships

2. **Design for eventual consistency**: Transactions may appear at different times on different nodes

3. **Use trust paths for verification**: When displaying data, show users the trust path so they understand the provenance

```javascript
// Display trust context to users
function displayPartnerInfo(partner, trustResult) {
  const trustPath = trustResult.trustPath;
  
  if (trustPath.length === 2) {
    // Direct trust
    return `You directly trust ${partner.name}`;
  } else {
    // Transitive trust - show the chain
    const intermediaries = trustPath.slice(1, -1);
    const names = await Promise.all(
      intermediaries.map(quid => getQuidName(quid))
    );
    return `You trust ${partner.name} through: ${names.join(' → ')}`;
  }
}
```

4. **Handle missing trust paths gracefully**: If no trust path exists, the entity isn't necessarily bad—you just don't have a connection to them

```javascript
const trust = await quidnugClient.getTrustLevel(myQuid, unknownQuid, domain);

if (trust.trustLevel === 0 && trust.trustPath.length === 0) {
  // No trust path exists
  showMessage(
    "No trust connection found. This entity may be legitimate, " +
    "but no one in your trust network has vouched for them."
  );
  
  // Offer options
  offerDirectVerification();  // Let user establish direct trust
  offerReferralRequest();     // Ask someone trusted to vouch
}
```

### Trust Edge Provenance

For audit purposes, you can retrieve full provenance for trust edges:

```javascript
// Get all trust edges for a quid with provenance
const response = await fetch(
  'http://localhost:8080/api/trust/edges/a1b2c3d4e5f6g7h8?includeUnverified=true'
);
const { edges, verifiedCount, unverifiedCount } = await response.json();

console.log(`${verifiedCount} verified edges, ${unverifiedCount} unverified`);

for (const edge of edges) {
  console.log(`${edge.truster} → ${edge.trustee}: ${edge.trustLevel}`);
  console.log(`  Source block: ${edge.sourceBlock}`);
  console.log(`  Validator: ${edge.validatorQuid}`);
  console.log(`  Verified: ${edge.verified}`);
  console.log(`  Recorded: ${new Date(edge.timestamp * 1000).toISOString()}`);
}
```

### Best Practices for Proof of Trust

1. **Start with verified-only queries**, expand to unverified when needed
2. **Always show users the trust path**, not just the trust score
3. **Design UX for "no path found"** scenarios
4. **Cache trust results** but with short TTLs (trust relationships change)
5. **Monitor tentative blocks** for domains you care about
6. **Establish trust proactively** with validators in your ecosystem

## Additional Resources

- [API Reference](https://docs.quidnug.org/api)
- [Client Libraries](https://github.com/quidnug/client-libraries)
- [Example Applications](https://github.com/quidnug/examples)
- [Security Best Practices](https://docs.quidnug.org/security)
- [Community Forum](https://community.quidnug.org)
