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

## Security Notice

<!-- SEC-77: CVE-2026-48934 — Node.js TLS session reuse host verification bypass -->

Node.js versions 22.0.0–22.23.0, 24.0.0–24.17.0, and 26.0.0–26.3.1
contain a TLS session reuse flaw ([CVE-2026-48934][cve-2026-48934])
that can bypass host identity verification. Because the Quidnug SDK
connects to multiple node endpoints in a single process, this
vulnerability could allow a compromised endpoint to impersonate a
different Quidnug service.

**Minimum safe Node.js versions:** 22.23.1, 24.17.1, or 26.3.2.
Node.js 18.x and 20.x are not affected.

[cve-2026-48934]: https://nvd.nist.gov/vuln/detail/CVE-2026-48934

## License

Apache-2.0.
