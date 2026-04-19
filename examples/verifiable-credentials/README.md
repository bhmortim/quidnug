# W3C Verifiable Credentials on Quidnug

How to use Quidnug as the **issuance + revocation + verification
ledger** for [W3C Verifiable Credentials (VCs)](https://www.w3.org/TR/vc-data-model/).

## The bridge

Each VC is represented in Quidnug as:

- **Credential subject** → Quidnug quid (the person / org / thing
  the credential is about).
- **Issuer** → Quidnug quid (the institution that issued).
- **Credential itself** → EVENT on the subject's stream, signed by
  the issuer. Payload is the VC JSON-LD document (or its hash +
  IPFS CID).
- **Revocation** → another EVENT with `eventType=VC_REVOKED` and
  the credential ID.

The Quidnug trust graph lets verifiers ask the question VCs alone
can't answer: **"from my perspective, how much do I trust the
issuer of this credential?"**

## Why

Plain VC verification is binary: either the issuer's DID is in your
trust list, or it isn't. Quidnug's relational trust lets a verifier
accept credentials from issuers they've never directly whitelisted,
as long as a transitive chain reaches them at sufficient level.

Example:

- Issuer: "University of Example"
- Verifier: "ACME HR" (never heard of University of Example)
- But: ACME HR trusts "HigherEd Accreditation Authority" at 0.9,
  which trusts University of Example at 0.95 → ACME HR's
  transitive trust in a University of Example credential is 0.855.

## Runnable example

See [`vc_issue_verify.js`](vc_issue_verify.js) — end-to-end:

1. University issues a degree credential to a student.
2. University publishes the credential as an EVENT on the student's
   quid stream.
3. Employer verifies: fetches the credential from the stream,
   checks the signature, scores the issuer via relational trust,
   and displays an acceptance decision.

```bash
cd examples/verifiable-credentials
node vc_issue_verify.js
```

## VC JSON-LD → Quidnug event payload

Quidnug stores the VC verbatim in the event `payload`:

```json
{
  "@context": ["https://www.w3.org/2018/credentials/v1"],
  "id": "urn:uuid:...",
  "type": ["VerifiableCredential", "UniversityDegreeCredential"],
  "issuer": "quidnug://university-of-example",
  "issuanceDate": "2026-05-15T00:00:00Z",
  "credentialSubject": {
    "id": "quidnug://<student-quid>",
    "degree": {
      "type": "BachelorDegree",
      "name": "Bachelor of Science in Computer Science"
    }
  },
  "proof": {
    "type": "Ed25519Signature2020",
    "created": "2026-05-15T00:00:01Z",
    "verificationMethod": "quidnug://university-of-example#keys-1",
    "proofPurpose": "assertionMethod",
    "proofValue": "..."
  }
}
```

Note: the VC's `proof` block uses the standard VC signature suite
(Ed25519, ES256, etc). Quidnug additionally signs the ENTIRE event
containing this VC with the issuer's ECDSA P-256 key via the
normal Quidnug signature path. So the VC gets **two** signatures:

1. The VC's own `proof` (for interop with existing VC verifiers).
2. The Quidnug event signature (for interop with the trust graph).

## Revocation

```json
{
  "type": "EVENT",
  "subjectId": "<student-quid>",
  "eventType": "VC_REVOKED",
  "payload": {
    "credentialId": "urn:uuid:...",
    "revokedAt": 1700000000,
    "reason": "credential_reissued_with_corrections"
  }
}
```

Verifiers scan the stream for `VC_REVOKED` events before accepting
a `VC_ISSUED` event. Revocation is append-only — to re-instate, emit
a `VC_REINSTATED` event.

## Verifier flow

```js
// 1. Fetch the student's stream
const events = await client.getStreamEvents(studentQuid, ...);

// 2. Find the credential
const issuances = events.events.filter(e => e.eventType === "VC_ISSUED");
const revocations = events.events.filter(e => e.eventType === "VC_REVOKED");

const active = issuances.filter(ev => {
    return !revocations.some(rv =>
        rv.payload.credentialId === ev.payload.id);
});

// 3. Score the issuer via relational trust
for (const vc of active) {
    const trust = await client.getTrustLevel(
        verifierQuid, vc.payload.issuer, "vc.education");
    if (trust.trustLevel >= 0.7) {
        // Accept
    }
}
```

## Compared to pure DID + VC

| Capability | DID + VC alone | DID + VC on Quidnug |
| --- | --- | --- |
| Cryptographic verification | ✓ | ✓ |
| Revocation | via revocation list / status list | via Quidnug event stream |
| Per-verifier trust | binary (whitelisted issuer?) | transitive relational trust |
| Cross-ecosystem issuer trust chains | bilateral federation lists | transitive through Quidnug graph |
| Audit log | out of scope | native event stream |

## Compatibility

- The VC document payload conforms to VC Data Model 1.1.
- The VC's own signature (`proof`) is preserved so existing VC
  verifiers can validate it without knowing about Quidnug.
- Adding Quidnug doesn't prevent you from also distributing the
  VC through standard channels (holder-mediated presentation,
  CHAPI, OpenID4VP).

## License

Apache-2.0.
