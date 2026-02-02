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

Trust is established between quids:
- Explicit trust levels (0.0 to 1.0)
- Domain-specific (e.g., `medical.credentials`, `property.texas`)
- Can have expiration dates
- Propagates transitively through the network

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
// Get trust level between quids
const trustLevel = await quidnugClient.getTrustLevel(
  'truster_quid_id', 
  'trustee_quid_id',
  'example.com'
);
console.log("Trust level:", trustLevel);

// Get quid identity
const identity = await quidnugClient.getIdentity('quid_id', 'example.com');
console.log("Identity:", identity);

// Get asset ownership
const ownership = await quidnugClient.getAssetOwnership('asset_quid_id', 'example.com');
console.log("Owners:", ownership.ownershipMap);

// Find path of trust between quids
const trustPath = await quidnugClient.findTrustPath(
  'source_quid_id',
  'target_quid_id',
  'example.com',
  { maxDepth: 4, minTrustLevel: 0.5 }
);
```

## Building Applications with Quidnug

### Authentication & Authorization

Quidnug can serve as an identity and authorization system:

```javascript
// Authenticate a user with their quid
async function authenticateUser(quidId, challenge, signature) {
  // Verify the user signed the challenge with their quid's private key
  const isValid = await quidnugClient.verifySignature(quidId, challenge, signature);
  
  if (isValid) {
    // Check if the quid is trusted in the relevant domain
    const trustLevel = await quidnugClient.getTrustLevel(
      'your_service_quid_id',
      quidId,
      'your-service.com'
    );
    
    if (trustLevel >= 0.7) {
      return { authenticated: true, trustLevel };
    }
  }
  
  return { authenticated: false };
}
```

### Credential Verification

For applications that need to verify credentials:

```javascript
// Verify a credential
async function verifyCredential(credentialQuidId, issuerQuidId, domain) {
  // Check if the credential exists and is defined by the expected issuer
  const credential = await quidnugClient.getIdentity(credentialQuidId, domain);
  
  if (credential && credential.definerQuid === issuerQuidId) {
    // Check if we trust the issuer
    const trustLevel = await quidnugClient.getTrustLevel(
      'your_service_quid_id',
      issuerQuidId,
      domain
    );
    
    if (trustLevel >= 0.8) {
      return { verified: true, credential };
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

## Additional Resources

- [API Reference](https://docs.quidnug.org/api)
- [Client Libraries](https://github.com/quidnug/client-libraries)
- [Example Applications](https://github.com/quidnug/examples)
- [Security Best Practices](https://docs.quidnug.org/security)
- [Community Forum](https://community.quidnug.org)
