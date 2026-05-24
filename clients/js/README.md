# Quidnug JavaScript SDK

`@quidnug/client` — the official JavaScript/TypeScript client for
[Quidnug](https://github.com/bhmortim/quidnug), a decentralized
protocol for relational, per-observer trust.

Runs in browsers (via Web Crypto) and Node 18+.

## Install

```bash
npm install @quidnug/client
```

## v1 surface

The default import provides the v1 surface — identities, trust,
titles, event streams, IPFS, peers, domains, moderation (QDP-0015),
audit (QDP-0018), privacy/DSR (QDP-0017), and the client-side
relational trust BFS. These are covered by the existing test suites
(`quidnug-client.test.js`, `quidnug-client.retry.test.js`,
`quidnug-client.coverage.test.js`).

```js
import QuidnugClient from "@quidnug/client";

const client = new QuidnugClient({
  defaultNode: "http://localhost:8080",
  debug: false,
});

const alice = await client.generateQuid({ name: "Alice" });
const bob   = await client.generateQuid({ name: "Bob" });

const tx = await client.createTrustTransaction(
  { trustee: bob.id, domain: "contractors.home", trustLevel: 0.9 },
  alice,
);
await client.submitTransaction(tx);

const result = await client.getTrustLevel(alice.id, bob.id, "contractors.home");
console.log(result.trustLevel, result.trustPath);
```

### v1 method list

| Area | Methods |
| --- | --- |
| Health / info | `getHealth`, `getInfo`, `getNodes`, `getPeers`, `getPeer` |
| Quids | `generateQuid`, `importQuid` |
| Transactions | `createTrustTransaction`, `createIdentityTransaction`, `createTitleTransaction`, `createEventTransaction`, `submitTransaction`, `getPendingTransactions` |
| Trust | `getTrustLevel`, `queryRelationalTrust`, `getTrustEdges`, `findTrustPath`, `computeTransitiveTrust` |
| Identity | `getIdentity`, `queryIdentityRegistry` |
| Titles | `getAssetOwnership`, `queryTitleRegistry` |
| Events | `getEventStream`, `getStreamEvents` |
| Storage | `pinToIPFS`, `getFromIPFS` |
| Blocks | `getBlocks`, `getTentativeBlocks` |
| Domains | `listDomains`, `registerDomain`, `getTopDomains`, `queryDomain`, `findNodesForDomain`, `getNodeDomains`, `updateNodeDomains` |
| Gossip / advertisements | `sendDomainGossip`, `createNodeAdvertisement` |
| Trust registry | `queryTrustRegistry` |
| Moderation (QDP-0015) | `createModerationAction`, `getModerationActions` |
| Audit (QDP-0018) | `getAuditHead`, `getAuditEntries`, `getAuditEntry` |
| Privacy (QDP-0017) | `createDSR`, `getDSRStatus`, `createConsentGrant`, `createConsentWithdraw`, `getConsentHistory`, `createProcessingRestriction`, `getRestrictionsForSubject`, `createDSRCompliance` |

## v2 extensions (QDPs 0002–0010, 0014, 0023)

Importing the v2 module installs guardian / gossip / bootstrap /
fork-block / Merkle methods on the `QuidnugClient` prototype. Keep
your v1 import for the transaction-signing surface, and add v2 when
you need the newer protocol features.

```js
import QuidnugClient from "@quidnug/client";
import "@quidnug/client/v2";

// Guardian set
const gs = await client.getGuardianSet("abcd1234abcd1234");

// Cross-domain gossip
const fp = await client.getLatestDomainFingerprint("contractors.home");
await client.submitAnchorGossip(msg);

// Compact Merkle proof verification (QDP-0010)
const ok = await QuidnugClient.verifyInclusionProof(
  canonicalTxBytes,           // Uint8Array or UTF-8 string
  gossipMsg.merkleProof,      // frames: [{ hash, side }]
  originBlock.transactionsRoot,
);
```

### v2 method list

| Area | Methods |
| --- | --- |
| Guardians (QDP-0002 / 0006) | `submitGuardianSetUpdate`, `submitRecoveryInit`, `submitRecoveryVeto`, `submitRecoveryCommit`, `submitGuardianResignation`, `getGuardianSet`, `getPendingRecovery`, `getGuardianResignations` |
| Gossip (QDP-0003 / 0005) | `submitDomainFingerprint`, `getLatestDomainFingerprint`, `submitAnchorGossip`, `pushAnchor`, `pushFingerprint` |
| Bootstrap (QDP-0008) | `submitNonceSnapshot`, `getLatestNonceSnapshot`, `getBootstrapStatus` |
| Fork-block (QDP-0009) | `submitForkBlock`, `getForkBlockStatus` |
| Discovery (QDP-0014) | `discoverDomain`, `discoverNode`, `discoverOperator`, `discoverQuids`, `discoverTrustedQuids` |
| DNS attestation (QDP-0023) | `submitDNSClaim`, `submitDNSChallenge`, `submitDNSAttestation`, `submitDNSRenewal`, `submitDNSRevocation`, `submitAuthorityDelegate`, `submitAuthorityDelegateRevocation`, `getDNSAttestations`, `getDNSAttestationsWeighted`, `resolveDNSRecord` |
| Static helpers | `QuidnugClient.verifyInclusionProof`, `QuidnugClient.canonicalBytes`, `QuidnugClient.bytesToHex`, `QuidnugClient.hexToBytes` |

### Canonicalization

`QuidnugClient.canonicalBytes(obj, excludeFields)` produces the
signable bytes used across all Quidnug SDKs — sorted keys,
UTF-8 JSON, excluding named fields like `signature`, `txId`,
`publicKey`. This matches the Go reference and the Python SDK
byte-for-byte, so a signature produced by one SDK verifies against
any other.

See [`schemas/types/canonicalization.md`](../../schemas/types/canonicalization.md)
for the full specification.

## TypeScript

Both v1 and v2 ship PR-quality `.d.ts` files. v2 uses module
augmentation, so importing the v2 side-effect module also expands
the TypeScript type surface automatically.

## Running the tests

```bash
npm test           # runs v1 + retry + v2 suites
npm run test:v2    # v2 only
```

## License

Apache-2.0.
