# Quidnug vs. W3C DIDs + Verifiable Credentials

W3C Decentralized Identifiers (DIDs) and Verifiable Credentials (VCs)
are the standards-track answer to "decentralized identity." They're
excellent at what they do. Quidnug overlaps in some areas and is
complementary in others.

## What DIDs + VCs do well

- **Persistent, self-sovereign identity.** A DID is a URI you
  control without asking a registrar.
- **Signed credential documents** in a well-defined JSON-LD
  format.
- **Cryptographic verification** of issuer claims about a subject.
- **Mature ecosystem**: several DID methods (did:key, did:web,
  did:ion, did:ethr), several credential formats (JSON-LD with
  Ed25519Signature2020, JWT-based VCs, SD-JWT, mDL).

## What DIDs + VCs don't do

- **Trust composition.** A VC is binary: either you trust the
  issuer's DID, or you don't. There's no "I trust this issuer at
  0.7" — you either accept or reject every credential they sign.
- **Transitive trust.** If you don't trust the issuer directly but
  do trust a third party that trusts them, you have to build your
  own trust chain; there's no protocol-level primitive.
- **Per-observer scoring.** Two different verifiers evaluating
  the same VC see the same "valid"/"invalid" outcome. There's no
  notion that Alice and Bob have different trust levels in the
  same issuer.
- **Revocation composition.** Each issuer maintains their own
  revocation list (StatusList2021, etc.). There's no unified
  revocation feed across issuers.
- **Audit log.** VCs are point-in-time artifacts; there's no
  built-in append-only event stream for "credential issued,
  revoked, reinstated."

## What Quidnug adds

| Capability | DID + VC alone | DID + VC on Quidnug |
| --- | --- | --- |
| Cryptographic verification | ✓ | ✓ |
| Subject-sovereign identity | ✓ (DID) | ✓ (Quid) |
| Credential issuance | ✓ | ✓ |
| Revocation | per-issuer lists | unified event stream |
| Per-verifier trust | binary | transitive relational trust |
| Cross-issuer trust chains | ad-hoc federation | native trust graph |
| Audit log (issuance → revocation history) | out of scope | native event stream |
| M-of-N recovery of issuer keys | out of scope | QDP-0002 guardians |
| Cross-domain gossip | out of scope | QDP-0003 |

## The recommended architecture

**Use both.** Represent DIDs as Quidnug quids (the quid ID maps
1:1 to a `did:quidnug:<id>` method). Issue VCs using standard
VC tooling (Ed25519Signature2020, SD-JWT, etc.) and record the
issuance as an EVENT on the subject's Quidnug stream.

- The VC itself conforms to VC Data Model 1.1, verifiable by any
  standards-compliant VC verifier.
- The EVENT gives you a Quidnug-native audit record: signed by
  the issuer, time-stamped by the block, revocable via a
  `VC_REVOKED` event.
- When a verifier evaluates the VC, they check:
  1. The VC signature itself (standard VC verification).
  2. The Quidnug EVENT containing it (unified revocation).
  3. Their relational trust to the issuer's quid (per-verifier
     scoring).

See [`examples/verifiable-credentials/`](../../examples/verifiable-credentials/)
for a runnable end-to-end example.

## When to pick one or the other

### Use DIDs + VCs (without Quidnug) when

- You need to interop with existing VC verifiers (eIDAS 2.0 wallet,
  EU digital identity, SIOP-V2 flows).
- Your trust model is flat: "my app accepts credentials from
  issuers X, Y, Z — full stop."
- You're using a DID method that's already widely adopted in
  your ecosystem (did:web for universities, did:jwk for machine
  identities).

### Use Quidnug on top of DIDs + VCs when

- Different verifiers have different trust in different issuers,
  and you need per-observer scoring.
- You need transitive trust chains — "I trust issuer A because
  they're vouched for by accreditor B whom I trust directly."
- You need a unified audit + revocation ledger across issuers.
- You need key recovery (guardian-based) on issuer keys.
- You want the relational-trust graph to inform UI decisions
  ("show a badge; green for high trust, red for low").

### Use Quidnug alone (no VCs) when

- Your domain doesn't require VC compatibility.
- You control both issuer and verifier within your system.
- You want to ship faster without coordinating with VC Data
  Model standards bodies.

## Migration paths

**Starting with VCs, adding Quidnug:**

Keep every VC flow. Whenever a VC is issued or revoked, also
post an EVENT on the subject's Quidnug stream. Verifiers that
know about Quidnug get richer scoring; verifiers that don't see
no change.

**Starting with Quidnug, adding VCs:**

When you need interop with an external VC consumer (government
eID, accreditor database, wallet app), issue a signed VC using
standard tools and attach it as the `payload.vc` of a
`VC_ISSUED` event on Quidnug.

## License

Apache-2.0.
