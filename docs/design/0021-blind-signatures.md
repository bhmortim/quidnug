# QDP-0021: Blind Signatures for Anonymous Ballot Issuance

| Field      | Value                                                            |
|------------|------------------------------------------------------------------|
| Status     | Draft — design only, no code landed                              |
| Track      | Protocol (auxiliary crypto)                                      |
| Author     | The Quidnug Authors                                              |
| Created    | 2026-04-21                                                       |
| Requires   | QDP-0001 (nonce ledger), QDP-0002 (guardian recovery), QDP-0012 (domain governance) |
| Implements | The blind-signature ballot-issuance primitive flagged as an open research question in `UseCases/elections/` |

## 1. Summary

The elections use case (`UseCases/elections/`) needs a way
for an authority to sign a ballot **without learning which
voter the ballot was issued to**. The authority must verify
eligibility (check the voter's VRQ, mark them as "issued a
ballot," prevent double-issuance) while preserving the secret
ballot (no later correlation between the issuance event and
the ballot cast).

Classical answer: blind signatures. Voter blinds a token,
authority signs the blinded token, voter unblinds to reveal a
valid signature on the original token. Authority sees only
the blinded version; the signature-of-record is on a token
the authority has never seen.

Quidnug's native signing primitive is ECDSA P-256 over SHA-256.
**ECDSA does not have a mainstream blind-signature variant.**
(There are research proposals — Boldyreva's, various EC-Schnorr
adaptations — but none are widely deployed or have stable
standard implementations.) So we need an auxiliary cryptographic
protocol alongside the existing ECDSA machinery.

QDP-0018 specifies:

- Which blind-signature scheme the reference implementation
  uses (**RSA-FDH with 3072-bit keys**).
- How the authority's RSA blind-signing key relates to its
  existing ECDSA governance key.
- Three new transaction types that carry blind-signed ballots
  through the protocol.
- How verifiers reconstruct and check the blinded-then-
  unblinded signature end-to-end.
- Migration path + alternative schemes we deliberately don't
  adopt and why.

The design is scoped to the elections use case but the
primitive is general — any use case needing "authority signs
X without learning which user X belongs to" can use it.

## 2. Goals and non-goals

**Goals:**

- Let an election authority prove eligibility for a ballot
  without the authority being able to correlate the ballot
  with the voter who requested it.
- Use well-standardized cryptography (no bespoke schemes).
- Integrate cleanly with existing Quidnug primitives (TRUST
  edges, events, nonces, guardian recovery).
- Support authority key rotation with forward compatibility
  for issued ballots.
- Provide a clear verification path auditable by any observer
  without special trust in the authority.

**Non-goals:**

- **Post-quantum resistance.** RSA-FDH is classical-secure; PQ
  blind signatures are a future QDP. For elections with a
  ~6-month ballot-validity window, classical security is fine
  through at least 2035.
- **General EC-Schnorr blind signatures** across the protocol.
  This QDP keeps blind signing in a clearly-scoped auxiliary
  role; a broader EC-based scheme is its own larger design
  exercise.
- **Receipt-freeness** (voters unable to even voluntarily
  prove how they voted). Current design preserves
  coercion-resistance through the BQ ephemerality but not
  full receipt-freeness; that would require a later QDP with
  re-randomizable commitments or homomorphic tallying.
- **Anonymous credentials more generally.** Use cases wanting
  "prove-you're-in-set-without-revealing-which-member" should
  use a dedicated anonymous-credential primitive (zk-SNARKs,
  BBS+ signatures) — QDP-0018 is specifically the
  sign-this-thing-without-knowing-whose-it-is primitive,
  which is narrower.

## 3. Why RSA-FDH with 3072-bit keys

Four candidate schemes considered:

1. **RSA-FDH** (Full Domain Hash) blind signatures. Chaum's
   original scheme with a hash-to-full-domain construction.
   Widely deployed (Privacy Pass, Signal, various voting
   systems). Standardized in RFC 9474.
2. **RSA-BSSA** (Blind Signature with Message Augmentation),
   RFC 9474 §3. Stronger security analysis; added cost is a
   one-extra-round protocol.
3. **EC-Schnorr blind signatures.** Smaller signatures, same
   curve as our existing ECDSA. But no current IETF-track
   standard, proofs are subtler, implementations vary.
4. **Pairing-based BLS** with a blind variant. Best
   theoretical aggregation properties but requires a pairing-
   friendly curve we don't otherwise need.

We pick **RSA-FDH at 3072-bit keys** for the following
reasons:

- **Mature standard.** RFC 9474 gives a clean reference
  implementation; test vectors exist; production
  deployments exist (Signal, Apple Privacy Pass).
- **Already-deployable in every SDK.** Python's
  `cryptography`, Go's `crypto/rsa`, Rust's `rsa` crate, JS's
  WebCrypto all support the underlying RSA operations.
  Blinding on top is ~30 lines of code each.
- **No new curve.** We don't need to add RSA to general
  protocol signing — only ballot issuance. Keeping it
  scoped means the main protocol stays ECDSA-P-256 as
  before.
- **3072-bit strength = ~128 bits of security** against
  classical attackers. Matches ECDSA P-256 strength.
  4096-bit is conservative-overkill for sub-decade ballots;
  2048-bit is below the 2030+ recommendation of most
  national crypto bodies.

We deliberately reject RSA-BSSA for complexity reasons (its
security gain is against a threat model that doesn't apply
in elections) and the EC variants on standardization grounds.
If a future use case needs smaller signatures or higher
throughput, it can adopt an EC scheme as a parallel QDP.

## 4. Key hierarchy

Each election authority has **two distinct key pairs**:

```
election-authority-<election>.governor-key      ECDSA P-256
    Used for: everything QDP-0012 (governance transactions,
              block signing, etc.), managed per existing
              QDP-0002 guardian recovery + QDP-0001 epoch
              rotation.

election-authority-<election>.blind-issuance-key   RSA-3072
    Used for: signing blinded ballot tokens ONLY.
    Life: one key per election cycle. Retired at election close.
    Recovery: via guardian quorum (same guardians as governor
              key, but independent key material).
```

The two keys are **cryptographically bound**: the authority
publishes a `BLIND_KEY_ATTESTATION` event signed with the
governor (ECDSA) key, declaring the RSA public key's fingerprint
as the valid blind-issuance key for this election. Voters and
observers verify the RSA key by first checking the governor-
signed attestation.

```json
{
    "type": "EVENT",
    "subjectId": "<election-authority-quid>",
    "subjectType": "QUID",
    "eventType": "BLIND_KEY_ATTESTATION",
    "sequence": <next>,
    "payload": {
        "electionId": "williamson-county-tx.2026-nov",
        "rsaPublicKey": "<SubjectPublicKeyInfo PEM or DER>",
        "rsaKeyFingerprint": "<sha256 of DER, hex>",
        "rsaBits": 3072,
        "algorithm": "RSASSA-PSS-FDH-SHA256",
        "validFrom": <unix-seconds>,
        "validUntil": <unix-seconds>,
        "previousKeyFingerprint": "<hex or null>"
    },
    "publicKey": "<authority's ECDSA pubkey>",
    "signature": "<ECDSA sig over canonical bytes>"
}
```

The attestation is itself a standard Quidnug event, so it
lives on-chain and is independently verifiable. A rogue
actor trying to substitute a fake RSA public key would need
to forge the ECDSA signature, which is infeasible.

**Rotation mid-election** is allowed but strongly discouraged:
a subsequent `BLIND_KEY_ATTESTATION` with
`previousKeyFingerprint` pointing at the prior key
supersedes it for signatures issued afterwards. Signatures
issued with the superseded key remain valid until
`validUntil` expires. In practice, blind-issuance keys are
ephemeral (per-election) and rotation is rare.

## 5. The ballot-issuance flow (end-to-end)

Sequence diagram showing how a voter gets a blind-signed
ballot, annotated with what each party does and doesn't see.

```
Voter's device                 Poll worker device           Authority node (consortium)
(holds VRQ priv key)           (holds precinct cache)       (runs blind-issuance service)
─────────────────────          ───────────────────────      ────────────────────────────

1. Voter arrives at precinct.
   Poll worker reads VRQ public ID from voter's phone.

         ─── VRQ.publicID ───►
                                  2. Poll worker queries poll-book
                                     cache for VRQ.publicID. Confirms
                                     (a) registered (b) not yet issued
                                     ballot for this election.
                                  3. Poll worker signs CHECK_IN event
                                     with their precinct-device key.
                                     Publishes to poll-book domain.

4. Voter's device generates:
   - A fresh per-ballot nonce N.
   - A fresh BQ ephemeral keypair (ECDSA P-256).
   - Ballot token T = SHA256(electionId || BQ.pubkey || N)

5. Voter's device blinds T:
   - Fetches the authority's RSA public key from the
     latest BLIND_KEY_ATTESTATION event (with ECDSA
     verification against the authority's governor key).
   - Generates random r in [2, n-1] where n is the RSA modulus.
   - Computes blinded = (T * r^e) mod n.
     (RSA blinding: r^e hides T but is multiplicatively
     reversible once signed.)

         ─── VRQ.sig(blinded, electionId, checkin_id) ───►
                                                            6. Authority node validates:
                                                               (a) VRQ.sig is valid for the blinded
                                                                   msg (proves voter owns the VRQ).
                                                               (b) A valid CHECK_IN event exists
                                                                   for VRQ in this election.
                                                               (c) No prior BALLOT_ISSUED event
                                                                   for this VRQ in this election.
                                                            7. Authority signs:
                                                               signed_blinded = blinded^d mod n
                                                               (where d is authority's RSA priv key;
                                                                this is RSA-FDH-standard signing.)
                                                            8. Authority publishes BALLOT_ISSUED event:
                                                               - payload: {VRQ.publicID, electionId}
                                                               - does NOT publish the signed_blinded
                                                                 (it's returned in the RPC response only)
                                                               - the event marks "issued" without revealing
                                                                 which blinded token was signed.
         ◄─── signed_blinded ────────────────────────────
9. Voter's device unblinds:
   - Computes signature S = signed_blinded * r^(-1) mod n
   - Verifies: S^e mod n == T (confirms valid signature on
     the original, unblinded ballot token T).
   - Discards r (the blinding factor). Once r is gone, no
     link between the authority-visible blinded msg and
     the signature S on T can be recovered, even with the
     authority's full internal logs.

10. Voter now holds:
    - BQ private key (for signing votes)
    - BQ public key (identifies the BQ)
    - T (the ballot token — an opaque random-looking hash)
    - S (authority's signature on T)

11. Voter goes into booth. Casts votes by publishing TRUST
    edges from BQ to candidates, each edge's payload includes:
    - T (proves this BQ is associated with a real ballot)
    - S (authority's blind-signature on T)
    - electionId
    - ephemeral BQ pubkey

12. Tally engine, for each vote edge from any BQ:
    - Verify ECDSA signature on the edge (standard).
    - Verify S is a valid RSA signature on T using the
      authority's published blind-issuance-key for this election.
    - If valid: count the vote.
    - If invalid: reject; publish REJECT event with reason.

    The tally has NO way to link T or S back to the VRQ that
    requested the ballot. The blinding factor r was discarded
    in step 9.
```

The key insight: **the authority sees `blinded = T * r^e mod n`
in step 6, but never sees T itself**. After voter discards `r`,
no one — including the authority with full log access — can
recover `blinded` from `T + S` or vice versa. The anonymity
comes from information-theoretically discarding r.

## 6. Transaction types

Three new or extended event types.

### 6.1 `BLIND_KEY_ATTESTATION`

Shown above in §4. Published by the authority at election
setup. Establishes the RSA-3072 blind-issuance key for the
election. Signed by the authority's governor ECDSA key.

### 6.2 `BALLOT_ISSUANCE_REQUEST` (voter → authority RPC)

This is not a traditional on-chain tx; it's a signed HTTP
request. Spec:

```
POST https://<authority>/api/v2/elections/<election>/ballot-request
Content-Type: application/json

{
    "electionId": "williamson-county-tx.2026-nov",
    "vrqPublicId": "<VRQ quid ID>",
    "checkinEventId": "<tx id of the CHECK_IN event>",
    "blindedBallotToken": "<hex>",
    "blindingKeyFingerprint": "<hex>",
    "vrqSignature": "<ECDSA sig over the rest of the body>",
    "timestamp": <unix-nano>
}
```

The authority's response:

```
HTTP/1.1 200 OK
Content-Type: application/json

{
    "status": "signed",
    "signedBlindedToken": "<hex>",
    "ballotIssuanceTxId": "<tx id of the BALLOT_ISSUED event>",
    "rsaKeyFingerprint": "<hex>",
    "timestamp": <unix-nano>
}
```

The `signedBlindedToken` is the authority's RSA signature on
the blinded token. Returned directly in the HTTP response; not
persisted anywhere the authority can later mine.

Important: the request itself is **not stored on-chain**. Only
the `BALLOT_ISSUED` event (§6.3) with minimal information is
stored. The blinded token is ephemeral — once the authority
signs it + the voter unblinds, no artifacts remain.

### 6.3 `BALLOT_ISSUED` event

On-chain record that a ballot was issued. Prevents double-
issuance + provides audit trail. Payload:

```json
{
    "eventType": "BALLOT_ISSUED",
    "payload": {
        "electionId": "williamson-county-tx.2026-nov",
        "vrqPublicId": "<VRQ quid ID>",
        "checkinEventId": "<tx id>",
        "rsaKeyFingerprint": "<fingerprint of the authority's RSA key used>",
        "issuedAt": <unix-nano>
    }
}
```

Note what's **not** in the payload: the blinded token, the
signed blinded token, the resulting unblinded signature. Only
"this VRQ was issued a ballot at this time using key X." The
voter is identifiable as having voted; how they voted remains
unlinkable.

### 6.4 Modified `VOTE_EDGE` (extension of TRUST_EDGE)

The vote itself. The existing TRUST transaction is
augmented with optional `ballotProof` fields:

```json
{
    "type": "TRUST",
    "truster": "<BQ quid ID (ephemeral)>",
    "trustee": "<candidate quid ID>",
    "trustLevel": 1.0,
    "domain": "elections.williamson-county-tx.2026-nov.contests.us-senate",
    "nonce": 1,

    "ballotProof": {
        "electionId": "williamson-county-tx.2026-nov",
        "ballotToken": "<T, hex>",
        "blindSignature": "<S, hex>",
        "rsaKeyFingerprint": "<fingerprint>",
        "bqEphemeralPubkey": "<hex>"
    },

    "publicKey": "<BQ pubkey>",
    "signature": "<BQ ECDSA sig over canonical bytes>"
}
```

The `ballotProof` block is what makes the vote verifiable.
At tally time, the engine:

1. Verifies the outer ECDSA signature (standard).
2. Verifies `ballotToken == SHA256(electionId || bqEphemeralPubkey || <something only the voter knows>)`.
   (Or, more precisely: `ballotToken` is any 32-byte value; its
   specific construction is voter-side.)
3. Verifies `blindSignature^e mod n == ballotToken` (the
   authority's RSA signature on the token).
4. Verifies `rsaKeyFingerprint` matches the
   `BLIND_KEY_ATTESTATION` published for this election.
5. If all four pass: count the vote.

## 7. Validation rules

Every `BALLOT_ISSUANCE_REQUEST` is rejected unless:

1. **VRQ signature valid.** ECDSA-P-256 verify of the
   request body (minus the signature field) against the
   VRQ's public key.
2. **Electoral roll present.** VRQ has a
   `VOTER_REGISTERED` event in the election's
   `registration` domain.
3. **Check-in present.** A `CHECK_IN` event exists for this
   VRQ referencing the claimed `checkinEventId`, within the
   last 2 hours (prevents replay of old check-ins).
4. **Not yet issued.** No prior `BALLOT_ISSUED` event for
   this VRQ in this election.
5. **Blinding key current.** `blindingKeyFingerprint` matches
   the authority's current `BLIND_KEY_ATTESTATION` event.
6. **Request freshness.** `timestamp` within 5 minutes of
   server clock.

Every `VOTE_EDGE` with `ballotProof` is rejected at tally
unless:

1. **Standard TRUST validation** (nonce, domain, etc.).
2. **Ballot token format.** 32 bytes, hex-encoded.
3. **Blind-signature verification.** `S^e mod n == T`
   against the authority's RSA public key identified by
   `rsaKeyFingerprint`.
4. **Key fingerprint known.** `rsaKeyFingerprint` matches
   a published `BLIND_KEY_ATTESTATION` for the election.
5. **Token not double-used.** No prior vote in this
   contest with the same `ballotToken`. (One ballot = one
   vote per contest; enforcement per-contest because a
   voter may vote on many contests with the same ballot.)

## 8. Double-vote prevention

Two layers:

**Layer 1: one BALLOT_ISSUED per VRQ.** Validation rule §7.4
above. An authority issuing a second ballot to the same VRQ
is a protocol violation; any observer can detect two
`BALLOT_ISSUED` events and flag it.

**Layer 2: one VOTE_EDGE per (ballotToken, contest).** At
tally, if the same `ballotToken` appears twice in votes for
the same contest, both are rejected and a
`DOUBLE_VOTE_DETECTED` event is published on the audit
domain for manual review.

Observe the interaction: a voter with one ballot can submit
votes for all contests on that ballot (governor, senator,
etc.) — each vote has the same `ballotToken` but a
different `contest` domain, so it's permitted. Two votes in
the same contest with the same ballot = forgery or replay;
blocked.

An attacker with a compromised VRQ private key can request
one ballot. They can't request two (validation rule §7.4).
A compromised authority key could issue multiple ballots
but observers would see multiple `BALLOT_ISSUED` events for
the same VRQ and flag immediately.

## 9. RSA key protection

The authority's RSA blind-issuance key is the single most
sensitive cryptographic asset of the election. Compromise =
attacker can issue unbounded fake ballots.

Controls:

1. **Generated offline.** On an air-gapped machine during
   pre-election setup. Never transmitted over a network.
2. **Stored in HSM.** The authority's signing service calls
   into a hardware security module (SoftHSM for pilot, real
   HSM for production). The private key never leaves the
   HSM.
3. **Signing service hardened.** Runs on isolated hardware,
   accepts signing requests only from the authority's
   consortium validators. Audited logs of every signing
   operation.
4. **One signature per valid request.** Rate-limited at the
   HSM level to prevent batch extraction.
5. **Guardian recovery** (QDP-0002). Like all keys; requires
   guardian quorum + time-lock. For an in-flight election,
   mid-election recovery is catastrophic (would invalidate
   all issued ballots); paper-ballot fallback is activated
   instead.
6. **Key retired at election close.** Post-election, the key
   is destroyed (literally: the HSM's container is deleted +
   backups are cryptographically shredded). Prevents
   retroactive ballot forgery.

## 10. Verification path

Any observer can verify blind-signature correctness without
trust in the authority. The verification chain:

```
Published on-chain:
  1. BLIND_KEY_ATTESTATION event, signed by authority's
     ECDSA governor key. Contains RSA public key.
  2. BALLOT_ISSUED events, one per VRQ that was issued a
     ballot.
  3. VOTE_EDGE transactions with ballotProof blocks.

Verification:
  A. Fetch BLIND_KEY_ATTESTATION(s) for the election.
     Verify ECDSA signature against the authority's
     known governor pubkey (published in the well-known
     file, QDP-0014).
  B. For each VOTE_EDGE with ballotProof:
     - Fetch the RSA public key referenced by
       rsaKeyFingerprint from the BLIND_KEY_ATTESTATION.
     - Compute S^e mod n. Compare to ballotToken.
     - If equal: the signature is valid. Count the vote.
     - If unequal: the signature is invalid. Reject +
       log for audit.
  C. Compare the number of VOTE_EDGEs per contest to the
     number of BALLOT_ISSUED events. These should match
     within one "unreturned ballots" margin. Any large
     discrepancy indicates an issue.

All verification uses only standard RSA + ECDSA primitives.
No novel crypto. Any observer with a laptop can run the
full check.
```

Crucially, this verification is **universal** — anyone, not
just the authority, can independently confirm every vote is
tied to a valid ballot and every ballot has one vote per
contest. The blinding operation preserves secrecy; the
verification operation preserves integrity. Both properties
come from the same underlying RSA math.

## 11. Security analysis

### 11.1 What the design protects

- **Vote secrecy.** Information-theoretic. After r is
  discarded, the authority's log of blinded tokens is
  mathematically decorrelated from the issued signatures.
  No amount of authority logs / coercion / subpoena can
  recover the linkage.
- **One-voter-one-vote.** Cryptographic. Double issuance
  requires compromising the authority key (detectable via
  duplicate `BALLOT_ISSUED`); double voting requires forging
  RSA signatures (infeasible).
- **Universal verifiability.** Any observer can verify
  every step with standard crypto libraries.
- **Auditability.** All events on-chain, signed, time-
  stamped, immutable.

### 11.2 What the design doesn't protect

- **Receipt-freeness.** A voter CAN voluntarily reveal
  `T` + `S` to a coercer, proving how they voted (since
  the coercer can verify via the public RSA key). Countered
  partially by: BQ is ephemeral, so the coercer has no
  way to force the voter to use a specific BQ; voter can
  generate multiple BQs and discard all but one. Full
  receipt-freeness requires homomorphic tallying or ZK
  proofs — future QDPs.
- **Authority-guardian-coalition compromise.** If the
  authority's RSA key is stolen AND the governor quorum
  is bypassed, attackers can issue unlimited fake ballots.
  Defeated only by the policy layer (multi-party
  governance, paper-ballot cross-verification).
- **Voter-device compromise.** Malware on a voter's phone
  could substitute a different candidate at vote-cast
  time. Countered by voter-side verification (after
  casting, fetch the published vote, display to voter;
  paper ballot serves as source of truth).

### 11.3 Threat: authority correlates checkin timing to ballot cast

The authority publishes `CHECK_IN` at time T₁ and receives the
corresponding `BALLOT_ISSUANCE_REQUEST` at T₂. Could
timing-correlation attack: "voter X checked in at 10:14:02;
blind-issuance request arrived at 10:14:04 with specific
blinded token; voter X's vote cast at 10:14:29 used matching
unblinded token" — does this leak voter-to-vote linkage?

**Mitigation 1: batching.** The authority's signing service
buffers requests for 30-60 seconds and processes them in a
random order. Timing entropy destroyed.

**Mitigation 2: voter-side delay.** The voter app introduces
a random 30-300 second delay between receiving the signed
blinded token and casting votes. Further decorrelates timing.

**Mitigation 3: group casting.** Votes are cast from the
voter's device but only committed to the chain in batches
(every ~60 seconds, via push gossip to the consortium). The
specific timing of any single vote cast is blurred.

These mitigations together make timing correlation
statistically ineffective at reasonable election volumes
(>1000 voters/hour). For very-low-volume elections
(pilot/small municipal), strong timing correlation is
possible and a mitigation note should appear in the user
docs.

## 12. Implementation plan

Three phases, each landable independently.

### Phase 1: crypto primitives

- Add RSA-FDH blind-signature primitives to each SDK:
  - Go: `pkg/crypto/blindrsa/`
  - Python: `quidnug.crypto.blindrsa`
  - JS: `@quidnug/client/crypto/blindrsa`
  - Rust: `quidnug::crypto::blindrsa`
- Test vectors: take RFC 9474 test vectors verbatim.
- HSM integration: PKCS#11 call sequence for batched blind
  signing.

Effort: ~2 person-weeks including tests + HSM drivers.

### Phase 2: transaction type + validation

- Add `BLIND_KEY_ATTESTATION` event type.
- Extend `TRUST` transaction with optional `ballotProof`.
- Add server-side validation for both.
- Add `/api/v2/elections/<election>/ballot-request` endpoint.

Effort: ~1.5 person-weeks.

### Phase 3: integration + docs

- Wire into the elections reference implementation
  (`examples/elections/`).
- Document in `UseCases/elections/` integration sections.
- Add threat-model sections for blind-specific attacks.
- Publish an operator guide for key generation + HSM setup.

Effort: ~1 person-week.

Total: ~5 person-weeks from proposal to shipped.

## 13. Deliberately not adopting

- **EC-Schnorr blind signatures.** Smaller signatures would
  be nice but no standardized variant exists. Re-evaluate
  once an IETF RFC or equivalent lands.
- **BSSA (Blind Signature with Augmentation).** RFC 9474
  covers both FDH and BSSA; BSSA is stronger against some
  theoretical attacks but adds a round-trip and is harder
  to implement in HSMs. For elections, FDH is sufficient.
- **Full anonymous credentials (BBS+ / CL-signatures).**
  These let users prove *arbitrary* statements about
  credentials. For elections, we only need "prove I have
  a valid ballot" — a blind signature is sufficient. Full
  anonymous credentials are a future-QDP with richer use
  cases.
- **Homomorphic tallying.** Full receipt-freeness would
  require ciphertext-space tally. Paillier / ElGamal-based
  schemes exist; massive scope. Future QDP.

## 14. Open questions

1. **Key-rotation during a long election.** State-wide
   elections with multi-week early voting may want to
   rotate the RSA key mid-cycle for defense-in-depth.
   Current design allows it via `BLIND_KEY_ATTESTATION`
   chaining but doesn't mandate it. Should we?

2. **Cross-election key reuse.** For repeated elections
   with the same authority, is it acceptable to reuse a
   single RSA key pair across cycles? Crypto is fine
   (each election has independent blinding factors); audit
   simplicity argues for per-election keys.

3. **Standardization.** Does it make sense to ship this as
   a Quidnug-specific thing or try to get the scheme into
   an IETF BoF? RFC 9474 does the hard work; we'd just
   need a small profile document.

4. **HSM cost at scale.** Federal-level elections need
   blind-signing throughput of potentially 10k ops/sec
   during peak. Commercial HSMs capable of this cost
   $50k-$200k. Budget implications for federal adoption.

5. **Interaction with `TRUST_IMPORT` (QDP-0013).** Can a
   blind-signed ballot be federated across networks? In
   principle yes (it's just a signature verifiable
   independently); in practice the semantics ("this is a
   ballot valid on our network but not yours") need care.

## 15. References

- [RFC 9474: RSA Blind Signatures](https://www.rfc-editor.org/rfc/rfc9474.html)
  — the authoritative wire-format spec for RSA-FDH blind
  signatures.
- [D. Chaum, "Blind Signatures for Untraceable Payments" (1983)](https://www.chaum.com/publications/Chaum-blind-signatures.PDF)
  — the foundational paper.
- [Privacy Pass (Cloudflare, Apple)](https://privacypass.github.io/)
  — a large-scale production deployment of RSA blind signatures.
- [QDP-0001](0001-global-nonce-ledger.md) — nonce ledger.
- [QDP-0002](0002-guardian-based-recovery.md) — guardian
  recovery (applies to the RSA blind-issuance key).
- [QDP-0012](0012-domain-governance.md) — governance for
  the authority's domain + governor quorum.
- [`UseCases/elections/README.md`](../../UseCases/elections/README.md)
  — the use case that drives this QDP.
- [`UseCases/elections/integration.md`](../../UseCases/elections/integration.md)
  — the elections integration that references blind signatures.

## 16. Review status

Draft. Primary reviewer bandwidth needed:

- A cryptographer familiar with blind-signature schemes
  (specifically to sanity-check the RFC 9474 profile
  choices + HSM integration).
- An elections-operations practitioner (to validate the
  key-lifecycle + ceremony design is practical at real
  jurisdictional scale).
- An auditor (to validate the universal-verifiability chain
  stands up to legal scrutiny).

Once those three sign off, Phase 1 implementation is safe
to start. The elections reference implementation in
`examples/elections/` can be adapted to use the blind-
signature primitives as soon as Phase 1 lands in the SDKs.
