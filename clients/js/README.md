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
titles, event streams, IPFS, and the client-side relational trust BFS.
These are covered by the existing test suite
(`quidnug-client.test.js`, `quidnug-client.retry.test.js`).

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

## v2 extensions (QDPs 0002–0010)

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
| Guardians | `submitGuardianSetUpdate`, `submitRecoveryInit`, `submitRecoveryVeto`, `submitRecoveryCommit`, `submitGuardianResignation`, `getGuardianSet`, `getPendingRecovery`, `getGuardianResignations` |
| Gossip | `submitDomainFingerprint`, `getLatestDomainFingerprint`, `submitAnchorGossip`, `pushAnchor`, `pushFingerprint` |
| Bootstrap | `submitNonceSnapshot`, `getLatestNonceSnapshot`, `getBootstrapStatus` |
| Fork-block | `submitForkBlock`, `getForkBlockStatus` |
| Static helpers | `QuidnugClient.verifyInclusionProof`, `QuidnugClient.canonicalBytes`, `QuidnugClient.bytesToHex`, `QuidnugClient.hexToBytes` |

All v2 methods route to `/api/v2/<path>` on the node, where the
v2-only handlers are mounted. (Earlier `2.x` releases routed to
`/api/<path>`, which returns 404 against a stock node.)

## v3 extensions (QDPs 0014, 0015, 0017, 0018, 0023)

Importing the v3 module installs peer / audit / moderation /
privacy / discovery / DNS-attestation / node-advertisement methods
on the `QuidnugClient` prototype.

```js
import QuidnugClient from "@quidnug/client";
import "@quidnug/client/v2";
import "@quidnug/client/v3";

// Operator audit log
const head = await client.getAuditHead();
const page = await client.getAuditEntries({ since: head.sequence - 100, limit: 100 });

// Network discovery
const info = await client.getDiscoveryDomain("contractors.home");

// DNS attestation lookup
const records = await client.resolveDNS("example.org", "A");
```

### v3 method list

| Area | Methods | Server path |
| --- | --- | --- |
| Peers | `getPeers`, `getPeer` | `/api/peers`, `/api/peers/{nodeQuid}` |
| Node advertisements | `submitNodeAdvertisement` | `/api/node-advertisements` |
| Domain registry | `getTopDomains`, `getTentativeBlocks`, `submitDomainGossip` | `/api/domains/top`, `/api/blocks/tentative/{domain}`, `/api/gossip/domains` |
| Moderation (QDP-0015) | `submitModerationAction`, `getModerationActions` | `/api/moderation/actions[...]` |
| Audit (QDP-0018) | `getAuditHead`, `getAuditEntries`, `getAuditEntry` | `/api/audit/*` |
| Privacy (QDP-0017) | `submitDSR`, `getDSRStatus`, `grantConsent`, `withdrawConsent`, `getConsentHistory`, `createProcessingRestriction`, `getProcessingRestrictions`, `submitDSRCompliance` | `/api/privacy/*` |
| Discovery (QDP-0014) | `getDiscoveryDomain`, `getDiscoveryNode`, `getDiscoveryOperator`, `discoveryQuids`, `discoveryTrustedQuids` | `/api/v2/discovery/*` |
| DNS attestation (QDP-0023) | `submitDNSClaim`, `submitDNSChallenge`, `submitDNSAttestation`, `submitDNSRenewal`, `submitDNSRevocation`, `submitDNSDelegate`, `submitDNSDelegateRevocation`, `getDNSAttestations`, `getDNSAttestationsWeighted`, `resolveDNS` | `/api/v2/dns/*` |

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

v1, v2, and v3 all ship `.d.ts` files. v2 and v3 use module
augmentation, so importing each side-effect module also expands
the TypeScript type surface automatically.

## Running the tests

```bash
npm test           # runs v1 + retry + v2 + v3 + vector suites
npm run test:v2    # v2 only
npm run test:v3    # v3 only
```

## License

Apache-2.0.
