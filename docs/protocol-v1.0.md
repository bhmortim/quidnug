# Quidnug Protocol Specification, v1.0

| Field         | Value                                    |
|---------------|------------------------------------------|
| Status        | Draft in progress                        |
| Version       | 1.0.0-draft                              |
| Authors       | The Quidnug Authors                      |
| Created       | 2026-04-20                               |
| Supersedes    | (nothing; this is the initial freeze)    |
| Companion     | `docs/protocol-invariants.md` (pending)  |

> **Purpose.** This is the single authoritative reference for
> protocol version 1.0. Every transaction type, every canonical
> byte layout, every validation rule, every endpoint that
> launches in v1.0 is enumerated here. Anything not specified
> in this document is not v1.0 and does not launch.
>
> Once v1.0 is formally frozen (tagged in `CHANGELOG.md` +
> network bootstrap per `docs/launch/genesis.md`) no changes
> to this document are permitted except via the QDP-0009
> fork-block migration mechanism or the QDP-0020 capability-
> negotiation path.
>
> This document is derived from QDPs 0001 through 0024 plus
> the current reference-node implementation. Where a QDP's
> design diverges from the implementation, the authoritative
> answer is in this document; conflicts are called out as
> **[OPEN]** items requiring resolution before freeze.

## 1. Introduction + scope

### 1.1 What Quidnug is

Quidnug is a relational-trust protocol. Participants ("quids")
express cryptographically-signed trust relationships with
other quids at a per-domain granularity. Events, titles, and
attestations are signed and appended to per-subject streams.
Any participant can compute the aggregate trust between any
two quids in a domain by walking the graph of signed edges.

The protocol has no global reputation score. Every trust
computation is observer-relative: what is "trusted" depends
on who's asking and which domain they're asking about. This
is the core design stance and is what distinguishes Quidnug
from centralized trust systems, from blockchain smart
contracts, and from flat PKI.

### 1.2 Architectural pillars

Three QDPs provide the top-level architectural model:

- **QDP-0012 Domain Governance.** Trust domains have
  governors (authorize policy changes), consortium members
  (produce blocks), and cache replicas (serve reads but
  cannot write). Roles are separable.
- **QDP-0013 Network Federation.** Multiple independent
  networks can run the same protocol with full interop.
  Reputation flows across networks via `TRUST_IMPORT`.
- **QDP-0014 Node Discovery + Domain Sharding.**
  `NODE_ADVERTISEMENT` transactions + five discovery
  endpoints + `.well-known/quidnug-network.json` let clients
  find the right nodes to query for any given domain.

All v1.0 behavior composes on top of these three pillars.

### 1.3 What v1.0 includes

Inclusion criteria:

1. The capability is specified in a QDP whose status is
   "Landed" or "Phase 1 Landed" at time of freeze.
2. The reference node implements the capability.
3. The reference SDKs (Go, Python, JS, Rust) expose the
   capability with byte-compatible signatures.
4. Cross-SDK test vectors validate round-trip correctness.

Section 12 enumerates the complete v1.0 scope. Sections 2-10
specify the wire format, primitives, transactions, events,
endpoints, registry state, and network protocols that
compose v1.0.

### 1.4 Normative language

Keywords MUST, MUST NOT, REQUIRED, SHALL, SHALL NOT, SHOULD,
SHOULD NOT, RECOMMENDED, MAY, and OPTIONAL follow RFC 2119
conventions.

## 2. Wire format

### 2.1 Field serialization

All on-wire payloads use **UTF-8 JSON** per RFC 8259. Numbers
are serialized as JSON numbers; 64-bit integers that exceed
`2^53 - 1` are serialized as JSON strings (applies only to
some timestamp fields; noted per-type in §4).

### 2.2 Canonical byte form (signable payload)

A transaction's **signable bytes** are computed by:

1. Construct a copy of the transaction struct with the
   `Signature` field set to the empty string.
2. Leave `PublicKey` as-is (it is part of the signable
   content).
3. Leave `ID` as-is. For freshly-generated transactions the
   signer computes the ID first (§2.4), populates it on the
   struct, then clears `Signature` and marshals for
   signing. This means `ID` is bound by the signature.
4. Serialize the struct with Go `encoding/json` default
   settings:
   - Top-level struct fields emitted in **declaration
     order** (NOT alphabetical).
   - Keys use the `json:` tag names where present.
   - `omitempty` fields are omitted when zero-valued.
   - No whitespace beyond JSON-required separators (Go
     default).
5. Any nested `map[string]interface{}` field (payload,
   attributes, and any other generic map the schema accepts)
   MUST be serialized with its keys in **Unicode codepoint
   order** (ascending). This matches Go's default
   `encoding/json` map-marshal behavior, which sorts
   map keys alphabetically. Arrays preserve their element
   order; dict elements nested inside arrays recursively
   follow this rule.
6. The resulting UTF-8 byte sequence is the signable
   payload.

#### 2.2.1 Why the two-rule split

Go's `encoding/json` treats structs and maps differently:

- `struct` values are serialized in field declaration order
  (the order fields appear in the Go source). This is stable
  and the v1.0 spec's transaction shapes pin specific orders
  per tx type in §4.
- `map[K]V` values are serialized with keys sorted
  alphabetically; this is the default because Go map
  iteration order is random.

Transaction shapes are fixed up-front (they are Go structs),
so the top-level serialization is determined by the schema
tables in §4. Nested user-supplied data (a payload JSON
object, an attributes map) is not tied to any Go struct — it
is marshaled as a generic map and therefore sorted.

**SDKs MUST implement both halves:** preserve caller-supplied
top-level field order (mirroring the Go struct declaration
listed per tx type), AND recursively sort keys of any nested
generic map. An SDK that sorts the top-level struct fields
alphabetically, or that preserves insertion order for nested
maps, will produce canonical bytes that diverge from the
reference node's re-marshal output — and server-side
signature verification will fail.

Reference implementations and their specific handling:

| SDK | Top level | Nested maps |
|---|---|---|
| Go (`pkg/client`) | preserved by `json.Marshal` on typed struct | sorted by `encoding/json` default |
| Python (`clients/python`) | explicit `(key, value, omitempty)` tuple list per tx type | `_go_compat_value` sorts nested dicts |
| Rust (`clients/rust`) | preserved by `serde::Serialize` on typed `Tx` struct | `serde_json`'s default `BTreeMap` is alphabetical |
| JS (`clients/js/v1-wire.js`) | explicit field tuple list | `goCompatValue` recursively sorts nested objects |
| Java (`CanonicalBytes.v1Of`) | preserved via Jackson `LinkedHashMap` tree | `sortNestedKeysOnly` recurses into nested objects |
| Swift (`CanonicalBytes.v1OfOrdered`) | caller supplies an array of `(String, Any)` pairs | `sortKeysDeep` recurses into nested dictionaries |

A regression-guard test vector
(`event_payload_key_sort_regression_guard` in
[`docs/test-vectors/v1.0/event-tx.json`](./test-vectors/v1.0/event-tx.json))
locks in this rule. The vector's payload is constructed with
keys in deliberately non-alphabetical insertion order at
both the top and one nested level; any SDK that reproduces
the expected canonical bytes for that case is correctly
implementing the rule.

### 2.3 Signature algorithm

All signatures in v1.0 use **ECDSA over NIST P-256** with
**SHA-256** as the hash function.

Signing process:

1. Compute canonical signable bytes per §2.2.
2. `H = SHA-256(signable_bytes)`.
3. Sign `H` with the signer's ECDSA-P256 private key.
   - The reference node MUST use RFC 6979 deterministic-k
     (see §2.3.1).
   - Third-party SDKs SHOULD use RFC 6979 deterministic-k.
   - Third-party SDKs MAY use non-deterministic-k backed
     by a cryptographically secure RNG.
4. Encode the resulting `(r, s)` pair as **IEEE-1363**:
   `r_bytes || s_bytes`, each component zero-padded to
   exactly 32 bytes. Total signature length: 64 bytes.
5. Represent on-wire as lowercase hex: 128 characters.

**DER-encoded ECDSA signatures MUST NOT be accepted.** The
reference node rejects non-64-byte signatures at the
verification layer (`internal/core/crypto.go:VerifySignature`).

#### 2.3.1 RFC 6979 deterministic-k

The reference node's `SignRFC6979` function in
`internal/core/crypto.go` implements RFC 6979 §3.2 for
P-256 + SHA-256 directly (no third-party dependencies).
Key properties:

- The nonce `k` is derived as a deterministic function of
  the private key `x` and the message digest `h1` via
  HMAC-DRBG with SHA-256. The standalone test at
  `internal/core/rfc6979_test.go` verifies the
  implementation against RFC 6979 Appendix A.2.5's
  published test vector (P-256 + SHA-256, message
  `"sample"`).

- Signatures are bit-stable across runs for the same
  `(key, message)` pair. The test vector harness in
  `docs/test-vectors/v1.0/` exploits this: the
  `reference_signature_hex` field in every case is
  reproduced byte-identically every time vectors are
  regenerated, so signature drift is detectable via `git
  diff` after regeneration.

- Signatures automatically have low `s` (see §2.3.2), so
  RFC 6979 + low-s normalization compose naturally.

SDK migration guidance:

- **Go** (`pkg/client`): migrate `(*Quid).Sign` to use
  `core.SignRFC6979` in a future minor release. Current
  implementation uses non-deterministic `crypto/ecdsa.Sign`
  but produces valid signatures that still verify.
- **Rust** (`clients/rust`): the `p256` crate's
  `ecdsa::SigningKey::sign` already uses RFC 6979
  deterministic-k by default. No migration required.
- **Python** (`clients/python`): `cryptography` library
  default is non-deterministic. Migration path: implement
  RFC 6979 in `quidnug/crypto.py` or vendor a library
  like `ecdsa` (pure-Python).
- **JS** (`clients/js`): WebCrypto's ECDSA is
  non-deterministic. Migration path: use `@noble/curves/p256`
  which supports deterministic signing.
- **Java** (`clients/java`): BouncyCastle's
  `ECDSASigner` with `HMacDSAKCalculator` supports RFC 6979.
- **.NET** (`clients/dotnet`): requires BouncyCastle.NET
  or a custom implementation; `ECDsa.SignHash` is
  non-deterministic.
- **Swift** (`clients/swift`): CryptoKit is
  non-deterministic with no current deterministic option;
  would need a different library.

Until SDKs migrate, cross-SDK test vectors remain
verification-stable (the reference signature verifies via
any SDK's verifier) but SDK sign-then-verify round-trips
produce different bytes from the reference.

### 2.3.2 Low-s normalization

Reference-node signatures always have `s <= n/2` (BIP-62
§low-s). When RFC 6979 produces an initial `s > n/2`, the
signer substitutes `n - s`. Both forms are mathematically
valid ECDSA signatures; the low-s form is canonical.

**Verification accepts both low-s and high-s signatures in
v1.0.** Enforcement of low-s at the verification layer is
deferred to v1.1+ via a fork-block migration per QDP-0009,
giving SDKs a coordinated window to adopt low-s
normalization.

### 2.4 Transaction ID derivation

Every transaction carries a deterministic `ID` that is:

1. A lowercase-hex encoding of a 32-byte SHA-256 hash.
2. Stable across re-serialization: given identical
   underlying transaction data, the ID is always the same.
3. Bound by the signature: the ID is included in the
   signable bytes, so tampering with the ID after signing
   invalidates the signature.

The bytes hashed to produce the ID are a subset of the
transaction's fields, varying per transaction type. The
subsets are chosen so that semantically-equivalent
transactions produce the same ID.

Per-type ID derivation schemas are specified in §4.

**[OPEN: ID derivation consistency]**
The Go implementation derives IDs from type-specific field
subsets (e.g., TrustTransaction ID from Truster+Trustee+
TrustLevel+TrustDomain+Timestamp). The Python SDK replicates
the shape but it's worth a line-by-line check. Test vectors
in §13 MUST cover this end to end.

### 2.5 Event ID derivation

Events (within `EventTransaction`) also carry an `ID` field.
Event ID equals the containing `EventTransaction.ID` for
v1.0. Future versions may distinguish these; clients
treating them as equivalent today are correct.

### 2.6 Block hash derivation

Block hashes are computed by:

1. Produce canonical bytes of the block over these fields:
   `Index`, `Timestamp`, `Transactions`,
   `TrustProof` (with `ValidatorSigs` excluded), `PrevHash`.
2. Canonicalize by a round-trip through
   `interface{}` / `map[string]interface{}` so sub-fields
   normalize to alphabetical JSON key ordering (reference:
   `calculateBlockHash` in `internal/core/crypto.go`).
3. SHA-256 over the resulting bytes; lowercase-hex encode.

The `NonceCheckpoints` and `TransactionsRoot` fields are
NOT included in block signable data in v1.0 (pre-activation
per QDP-0001 §10.2 and QDP-0010 respectively). Activation
happens via fork-block post-v1.0; transitioning is out of
scope for this document.

### 2.7 Timestamps

Two timestamp conventions are used:

- **Unix seconds** (int64): used on `BaseTransaction.Timestamp`
  for IDENTITY, TRUST, TITLE, EVENT, and derivatives. These
  fields hold seconds-since-epoch.
- **Unix nanoseconds** (int64): used on newer fields like
  `ValidUntil` on TRUST, `ExpiresAt` on NODE_ADVERTISEMENT,
  `EffectiveAt` / `ValidUntil` on event payloads, and
  attestation-class expiry.

**[OPEN: timestamp unit unification]**
Mixing seconds and nanoseconds in the same protocol is a
bug farm. The reference implementation has evolved to
nanoseconds for new fields but kept seconds for
historical `Timestamp`. Before freeze: decide whether to
migrate `Timestamp` to nanoseconds (breaking change, fork-
block required) or accept the two-convention reality and
document it clearly with per-field units in §4.

### 2.8 String field conventions

- Quid identifiers: lowercase-hex, always exactly 16
  characters, derived from first 8 bytes of
  `SHA-256(uncompressed-public-key)` per §3.2.
- Public keys: lowercase-hex uncompressed SEC1 encoding,
  always 130 characters (65 bytes: `0x04 || X || Y`).
- Signatures: lowercase-hex, always 128 characters
  (64 bytes: `r || s`).
- Transaction IDs: lowercase-hex, always 64 characters
  (32-byte SHA-256 output).
- Trust domains: dot-separated labels, each label a-z, 0-9,
  dash; max 253 characters total; max 63 characters per
  label (DNS-compatible).

## 3. Cryptographic primitives

### 3.1 Primitive catalog

| Purpose | Primitive | Reference |
|---|---|---|
| Identity signing | ECDSA P-256 over SHA-256 | FIPS 186-4, RFC 6979 |
| Key derivation (from pubkey to quid) | SHA-256 truncated to 8 bytes | FIPS 180-4 |
| Block + tx hash | SHA-256 | FIPS 180-4 |
| Block Merkle tree (QDP-0010) | SHA-256 | QDP-0010 §3 |
| Blind-signature ballot issuance (QDP-0021) | RSA-FDH-3072 | RFC 9474 |
| Group encryption key agreement (QDP-0024) | X25519 | RFC 7748 |
| Group encryption symmetric cipher (QDP-0024) | AES-GCM-256 | FIPS 197, SP 800-38D |
| Group encryption KDF (QDP-0024) | HKDF-SHA256 | RFC 5869 |
| Group encryption tree (QDP-0024) | TreeKEM per MLS | RFC 9420 |

### 3.2 Quid identifier derivation

Given an ECDSA-P256 public key:

1. Marshal the pubkey in SEC1 uncompressed form:
   `0x04 || X || Y`, totaling 65 bytes. Leading zero bytes
   on X and Y are preserved.
2. `H = SHA-256(marshaled_pubkey_bytes)`.
3. Quid ID = lowercase-hex of `H[0:8]` (16 hex characters).

Collisions in the 16-hex space are practically impossible
at launch scale; the 64-bit prefix gives `~1.8 × 10^19`
namespace. If collisions ever become relevant, a future
protocol version can extend to full 64-hex (32-byte) IDs
via fork-block.

### 3.3 Public-key encoding

On the wire, public keys appear as `publicKey` in JSON
objects. Format: hex encoding of the SEC1 uncompressed
form, 130 characters. Compressed (`0x02 || X` or
`0x03 || X`) forms MUST NOT be used in v1.0 transactions.

### 3.4 Signature verification

Verifier:

1. Decode hex public key; unmarshal SEC1 to `ecdsa.PublicKey`
   on the P-256 curve.
2. Decode hex signature to 64 bytes; split to 32-byte `r`
   and `s`.
3. Compute signable bytes per §2.2 from the received
   transaction.
4. `H = SHA-256(signable_bytes)`.
5. Verify `(r, s)` against `H` using the public key.

Implementation: `internal/core/crypto.go:VerifySignature`.

### 3.5 Low-s normalization and enforcement

Resolved. See §2.3.2 for the full specification:

- Reference-node signatures always have `s <= n/2`
  (automatic via `SignRFC6979` + normalization).
- Verification layer accepts both low-s and high-s
  signatures in v1.0. This lets SDKs using
  non-deterministic ECDSA (which roughly half the time
  produces high-s without explicit normalization) continue
  to work.
- Tightening verification to reject high-s is scheduled
  for v1.1+ via fork-block migration (QDP-0009), once SDKs
  have adopted low-s normalization on the signing side.

Rationale for the soft stance in v1.0: Quidnug transaction
IDs do not include signatures in their hash derivation
(see §2.4), and replay protection relies on per-signer
monotonic nonces (QDP-0001), not signature uniqueness.
Malleability is therefore a hardening item rather than a
correctness-blocking defect.

## 4. Transaction type catalog

Every transaction in v1.0 descends from `BaseTransaction`:

```go
type BaseTransaction struct {
    ID          string          `json:"id"`
    Type        TransactionType `json:"type"`
    TrustDomain string          `json:"trustDomain"`
    Timestamp   int64           `json:"timestamp"`   // Unix seconds
    Signature   string          `json:"signature"`
    PublicKey   string          `json:"publicKey"`
}
```

All transaction types REQUIRE these fields. Type-specific
fields are declared after the embedded base.

### 4.0 v1.0 size + length constants

The reference-node validators apply these limits uniformly
across every applicable transaction type. All values are in
bytes unless noted. Source of truth in each case is a Go
constant; changes require a fork-block migration (QDP-0009)
after launch.

| Constant | Value | Applies to | Source |
|---|---|---|---|
| `MaxDomainLength` | 253 | `trustDomain` on every tx | `internal/core/middleware.go:289` |
| `MaxNameLength` | 256 | `IdentityTransaction.Name`, `TitleTransaction.TitleType` | `internal/core/middleware.go:287` |
| `MaxDescriptionLength` | 4096 | `TrustTransaction.Description`, `IdentityTransaction.Description` | `internal/core/middleware.go:288` |
| `MaxEventTypeLength` | 64 | `EventTransaction.EventType` | `internal/core/validation.go:232` |
| `MaxPayloadSize` | 65536 (64 KB) | `EventTransaction.Payload` JSON-serialized | `internal/core/validation.go:235` |
| `MaxAnnotationTextLength` | 2048 | `ModerationActionTransaction.AnnotationText` | `internal/core/moderation.go:74` |
| `MaxRequestDetailsLength` | 4096 | `DataSubjectRequestTransaction.RequestDetails` | `internal/core/privacy.go:84` |
| `MaxPolicyHashLength` | 128 | `ConsentGrantTransaction.PolicyHash` | `internal/core/privacy.go:88` |
| `GossipPushMaxEnvelopeBytes` | 131072 (128 KB) | push-gossip messages | `internal/core/gossip_push.go:115` |
| `MerkleMaxTxsPerBlock` | 4096 | block transaction count | `internal/core/merkle.go:41` |
| `DefaultMaxNonceGap` | 1024 | per-signer nonce gap tolerance | `internal/core/ledger.go:52` |
| `AnchorMaxFutureSkew` | 5 min | `AnchorTransaction.AnchorTimestamp` future bound | `internal/core/anchor.go:70` |
| `AnchorMaxAge` | 30 days | `AnchorTransaction.AnchorTimestamp` past bound | `internal/core/anchor.go:76` |
| `AnchorGossipMaxAge` | 24 hours | anchor gossip message age | `internal/core/anchor_gossip.go:87` |
| `ResignationEffectiveAtMaxFuture` | 365 days | `GuardianResignation.EffectiveAt` lead | `internal/core/guardian_resignation.go:59` |

**Control character handling.** Every string field that
flows through `ValidateStringField` rejects ASCII control
characters except tab (`\t`, 0x09), newline (`\n`, 0x0A), and
carriage return (`\r`, 0x0D). See `middleware.go:293`.

### 4.1 Transaction type enumeration

| Type constant | On-wire `type` | QDP origin | v1.0 status |
|---|---|---|---|
| `TxTypeTrust` | `TRUST` | 0001, 0002 | Required |
| `TxTypeIdentity` | `IDENTITY` | 0001 | Required |
| `TxTypeTitle` | `TITLE` | 0002 | Required |
| `TxTypeEvent` | `EVENT` | 0003 | Required |
| `TxTypeGeneric` | `GENERIC` | (legacy) | **[OPEN]** remove or keep? |
| `TxTypeAnchor` | `ANCHOR` | 0001 | Required |
| `TxTypeNodeAdvertisement` | `NODE_ADVERTISEMENT` | 0014 | Required |
| `TxTypeModerationAction` | `MODERATION_ACTION` | 0015 | Required (Phase 1 landed) |
| `TxTypeDataSubjectRequest` | `DATA_SUBJECT_REQUEST` | 0017 | Required (Phase 1 landed) |
| `TxTypeConsentGrant` | `CONSENT_GRANT` | 0017 | Required (Phase 1 landed) |
| `TxTypeConsentWithdraw` | `CONSENT_WITHDRAW` | 0017 | Required (Phase 1 landed) |
| `TxTypeProcessingRestriction` | `PROCESSING_RESTRICTION` | 0017 | Required (Phase 1 landed) |
| `TxTypeDSRCompliance` | `DSR_COMPLIANCE` | 0017 | Required (Phase 1 landed) |

**Deferred to post-v1.0** (Draft QDP, not required for launch):

| Type constant | On-wire `type` | QDP | Phase |
|---|---|---|---|
| `DNS_CLAIM`, `DNS_CHALLENGE`, `DNS_ATTESTATION`, `DNS_RENEWAL`, `DNS_REVOCATION` | per-name | 0023 | Planned Phase 1 pre-launch |
| `AUTHORITY_DELEGATE`, `AUTHORITY_DELEGATE_REVOCATION` | per-name | 0023 | Planned Phase 1 pre-launch |
| `GROUP_CREATE`, `EPOCH_ADVANCE`, `MEMBER_KEY_PACKAGE`, `ENCRYPTED_RECORD`, `MEMBER_INVITE`, `MEMBER_KEY_RECOVERY` | per-name | 0024 | Planned Phase 1 pre-launch |
| `BLIND_KEY_ATTESTATION` | per-name | 0021 | Post-launch |
| `AUDIT_ANCHOR` | per-name | 0018 | Planned Phase 3 pre-launch |
| `TRUST_IMPORT` | per-name | 0013 | **[OPEN]** Phase 1 scope |
| `DOMAIN_GOVERNANCE` | per-name | 0012 | **[OPEN]** Phase 2 scope |
| `MODERATION_ACTION` federation import | per-name | 0015 | Phase 2 deferred |

**[OPEN: launch-gate transaction list]**
Decide which of the "planned Phase 1 pre-launch" items
actually ship in v1.0. The DNS attestation primitives
(QDP-0023) are strategically important. Group encryption
(QDP-0024) backs `private:*` but could be deferred to v1.1
if adoption scenarios don't need it day one. Decision
affects Phase 2 of the execution plan.

### 4.2 `TRUST` (TrustTransaction)

**Struct:**

```go
type TrustTransaction struct {
    BaseTransaction
    Truster     string  `json:"truster"`
    Trustee     string  `json:"trustee"`
    TrustLevel  float64 `json:"trustLevel"`
    Nonce       int64   `json:"nonce"`
    Description string  `json:"description,omitempty"`
    ValidUntil  int64   `json:"validUntil,omitempty"`   // Unix seconds
}
```

**Semantics:** declares that `Truster` trusts `Trustee` at
level `TrustLevel` in domain `TrustDomain`, optionally until
`ValidUntil`.

**Validation rules (v1.0):**

1. `TrustDomain` MUST refer to an existing `TrustDomain` in
   the receiver's registry.
2. `Nonce` MUST be positive and strictly greater than the
   current highest accepted nonce for the `(Truster,
   Trustee)` pair.
3. `TrustLevel` MUST satisfy `0.0 <= TrustLevel <= 1.0`
   and be neither `NaN` nor `±Inf`.
4. `ValidUntil` when non-zero MUST be strictly greater than
   `Timestamp`.
5. `Truster` and `Trustee` MUST be valid quid IDs (16 hex
   lowercase).
6. `Description` MUST NOT exceed `MaxDescriptionLength`
   (**4096 bytes** per `internal/core/middleware.go`) and
   MUST NOT contain ASCII control characters except tab,
   newline, or carriage return.
7. Signature MUST verify per §3.4 using the canonical bytes
   from §2.2.
8. Nonce ledger admission MUST succeed (QDP-0001) when
   enforcement is active.
9. QDP-0016 multi-layer rate limiter MUST admit.

**ID derivation:** hash the Go-struct-ordered JSON of
`{Truster, Trustee, TrustLevel, TrustDomain, Timestamp}`.

### 4.3 `IDENTITY` (IdentityTransaction)

**Struct:**

```go
type IdentityTransaction struct {
    BaseTransaction
    QuidID      string                 `json:"quidId"`
    Name        string                 `json:"name"`
    Description string                 `json:"description,omitempty"`
    Attributes  map[string]interface{} `json:"attributes,omitempty"`
    Creator     string                 `json:"creator"`
    UpdateNonce int64                  `json:"updateNonce"`
    HomeDomain  string                 `json:"homeDomain,omitempty"`
}
```

**Semantics:** registers or updates a quid's identity
record in `TrustDomain`. Creator may be the quid itself
(self-registration) or a parent quid acting as creator
(e.g., HR onboarding an employee).

**Validation rules (v1.0):**

1. `QuidID` MUST be valid quid ID format (§3.2).
2. Public key's derived quid (§3.2) MUST match the signer
   role: for self-registration, equals `QuidID`; for
   parent-creator registration, equals `Creator`.
3. `UpdateNonce` MUST be strictly greater than the
   registry's current value for `QuidID` (replay protection;
   `1` for first registration).
4. `Attributes` MUST be valid JSON and MUST NOT contain
   reserved keys (`[OPEN: enumerate reserved attribute keys]`).
5. `HomeDomain` when non-empty MUST be a valid trust domain
   name (used by QDP-0007 epoch-probe).
6. Signature MUST verify per §3.4.
7. QDP-0016 rate-limiter admission.

**ID derivation:** hash of
`{QuidID, Name, Creator, TrustDomain, UpdateNonce, Timestamp}`.

### 4.4 `TITLE` (TitleTransaction)

**Struct:**

```go
type TitleTransaction struct {
    BaseTransaction
    AssetID        string            `json:"assetId"`
    Owners         []OwnershipStake  `json:"owners"`
    PreviousOwners []OwnershipStake  `json:"previousOwners,omitempty"`
    Signatures     map[string]string `json:"signatures"`
    ExpiryDate     int64             `json:"expiryDate,omitempty"`  // Unix seconds
    TitleType      string            `json:"titleType,omitempty"`
}

type OwnershipStake struct {
    OwnerID    string  `json:"ownerId"`
    Percentage float64 `json:"percentage"`
    StakeType  string  `json:"stakeType,omitempty"`
}
```

**Semantics:** declares ownership of `AssetID` by the set
of `Owners`. Transfers reference `PreviousOwners`. Multi-
party titles require signatures from enough prior owners
per domain policy; per-owner signatures stored in
`Signatures[ownerId]`.

**Validation rules (v1.0):**

1. `AssetID` MUST be non-empty and MUST match format
   `[a-zA-Z0-9._:-]{1,128}` `[OPEN: canonical format?]`.
2. Sum of `Owners[].Percentage` MUST equal exactly `1.0`
   (floating-point with epsilon `1e-9`).
3. On transfer (non-empty `PreviousOwners`): the
   `Signatures` map MUST contain signatures from prior
   owners holding at least the transfer-threshold
   percentage (default 1.0 unless `TitleType` specifies a
   lower threshold; **[OPEN: specify per-title-type threshold
   table]**).
4. `ExpiryDate` when non-zero MUST be strictly greater
   than `Timestamp`.
5. Signature (in `BaseTransaction.Signature`) MUST verify
   from the title creator's key.

**ID derivation:** hash of
`{AssetID, Owners, TrustDomain, Timestamp}`.

### 4.5 `EVENT` (EventTransaction)

**Struct:**

```go
type EventTransaction struct {
    BaseTransaction
    SubjectID       string                 `json:"subjectId"`
    SubjectType     string                 `json:"subjectType"`  // "QUID" | "TITLE" | "DOMAIN"
    Sequence        int64                  `json:"sequence"`
    EventType       string                 `json:"eventType"`
    Payload         map[string]interface{} `json:"payload,omitempty"`
    PayloadCID      string                 `json:"payloadCid,omitempty"`   // IPFS CID for off-chain payload
    PreviousEventID string                 `json:"previousEventId,omitempty"`
}
```

**Semantics:** appends an event of kind `EventType` to the
stream for `SubjectID`. Streams are ordered by monotonically-
increasing `Sequence`.

**Validation rules (v1.0):**

1. `SubjectID` MUST be a valid quid ID (`SubjectType ==
   "QUID"`) or title ID (`SubjectType == "TITLE"`) or
   domain name (`SubjectType == "DOMAIN"`).
2. `Sequence` MUST be strictly greater than the stream's
   current highest sequence, OR equal to `1` if the stream
   is new.
3. `EventType` MUST be present and non-empty. Enumerated
   per-use-case (§5).
4. `PreviousEventID` when non-empty MUST equal the ID of
   the prior event in the stream (enforces linked-list
   integrity).
5. Exactly one of `Payload` or `PayloadCID` MUST be present
   (not both, not neither). SDK-side precondition checks
   enforce this (pkg/client, quidnug/client.py,
   clients/js/v1-wire.js); the reference node currently
   accepts both-populated and neither-populated submissions
   with warnings logged. **Enforcement at the node layer is
   scheduled for v1.1+** via fork-block; until then, SDK
   callers are responsible.
6. Payload size limits: inline `Payload` MUST NOT serialize
   to more than `MaxPayloadSize` (**64 KB per
   `internal/core/validation.go:235`**); larger payloads
   MUST use `PayloadCID` + IPFS.
7. `EventType` MUST NOT exceed `MaxEventTypeLength`
   (**64 bytes per `internal/core/validation.go:232`**).
8. Signature MUST verify; public key MUST resolve to a
   quid authorized to write to the subject's stream
   (subject-quid writes own stream; authorized delegates
   per subject's policy; creator quid for titles).

**ID derivation:** hash of
`{SubjectID, EventType, Sequence, TrustDomain, Timestamp}`.

#### 4.5.1 Authorship: who can write to a stream

Every `EVENT` transaction must be signed by a quid that is
authorized to write to the subject's stream. The v1.0 rules:

1. **`SubjectType == "QUID"`.** The signer's public key MUST
   match the subject quid's registered public key. In effect:
   a QUID stream is writable only by the quid itself.
2. **`SubjectType == "TITLE"`.** The signer MUST be one of
   the title's current owners (any listed
   `OwnershipStake.OwnerID` whose registered public key
   matches `tx.PublicKey`).

These are hard rules. The node rejects events that violate
them at validation time — there is no policy knob to relax
them.

#### 4.5.2 Convention: shared logs as jointly-owned titles

Many use cases need a **shared event stream** that multiple
parties can write to — a cosign ledger, a multi-party
authorization, a consortium audit log. The v1.0 rules above
mean a shared log CANNOT be a plain QUID (only the quid
itself could write to it).

The idiomatic pattern: model the shared log as a **`TITLE`
jointly owned by every party that needs to write to it.**
Each writer is listed as an `OwnershipStake` (share can be
small); events on the title's stream then satisfy rule (2).

Worked examples:
- An AI agent action: title jointly owned by the agent
  (primary share) and every guardian (small shares each).
- A custody wallet transfer: title jointly owned by the
  wallet, the proposer, and every authorized signer.
- A federated-learning round: title jointly owned by the
  coordinator and every participant.
- A DNS zone: title jointly owned by every governor.

#### 4.5.3 Convention: external attestations on the attester's own stream

The complementary pattern for **third-party attestations
about a subject** (a researcher reporting a CVE about a
release, a credit bureau attesting to a borrower, a
fact-checker endorsing media, an oracle reporting a price).
Such attestations cannot ride on the subject's stream
because the attester is not the subject's owner.

The idiomatic pattern: **the attester emits the event on
their OWN quid stream with a subject pointer in the
payload.** Downstream consumers subscribe to a curated list
of attester streams, filter by payload subject, and merge.

Worked examples:
- `release.vulnerability-reported` lives on the security
  researcher's own stream with `targetReleaseId` in the
  payload.
- `credit.payment.on-time` lives on the lender's own stream
  with `subject` (borrower quid) in the payload.
- `oracle.price-report` lives on each reporter's stream with
  `feedQuid` in the payload.
- `consent.emergency-override` lives on each guardian's
  stream with `patient` (subject quid) in the payload.

These two conventions (4.5.2 and 4.5.3) cover every
multi-party workflow the 16 reference use cases exercise.

### 4.6 `ANCHOR` (AnchorTransaction)

**Struct:**

```go
type AnchorTransaction struct {
    BaseTransaction
    Anchor NonceAnchor `json:"anchor"`
}

type NonceAnchor struct {
    // [VERIFY: check internal/core/types.go for full shape]
    SignerQuid      string `json:"signerQuid"`
    Domain          string `json:"domain"`
    Epoch           int64  `json:"epoch"`
    HighestNonce    int64  `json:"highestNonce"`
    AnchorTimestamp int64  `json:"anchorTimestamp"`
    Signature       string `json:"signature"`     // Anchor's own signature
    PublicKey       string `json:"publicKey"`
}
```

**Semantics:** QDP-0001 cross-domain nonce-anchor record.
Declares the highest-nonce value the signer has used in a
`(Domain, Epoch)` tuple. Propagated via gossip so other
domains can cross-reference when validating trust edges
across epochs.

The envelope `BaseTransaction.Signature` is empty; the
anchor's inner signature is the authoritative one.

**Validation rules (v1.0):** per QDP-0001 §6.5. Summarizing:

1. Inner anchor signature MUST verify with the embedded
   `Signature` + `PublicKey` over the anchor-specific
   canonical bytes.
2. `HighestNonce` MUST be >= any previously-seen anchor for
   the same `(SignerQuid, Domain, Epoch)`.
3. Epoch MUST match the domain's current epoch or a prior
   epoch; future-epoch anchors MUST be rejected.

### 4.7 `NODE_ADVERTISEMENT` (QDP-0014)

**Struct:**

```go
type NodeAdvertisementTransaction struct {
    BaseTransaction
    NodeQuid           string           `json:"nodeQuid"`
    OperatorQuid       string           `json:"operatorQuid"`
    Endpoints          []NodeEndpoint   `json:"endpoints"`
    SupportedDomains   []string         `json:"supportedDomains,omitempty"`
    Capabilities       NodeCapabilities `json:"capabilities"`
    ProtocolVersion    string           `json:"protocolVersion"`
    ExpiresAt          int64            `json:"expiresAt"`           // Unix nanoseconds
    AdvertisementNonce int64            `json:"advertisementNonce"`  // monotonic per NodeQuid
}

type NodeEndpoint struct {
    URL      string `json:"url"`                // MUST be https://
    Protocol string `json:"protocol,omitempty"` // "http/1.1" | "http/2" | "http/3" | "grpc"
    Region   string `json:"region,omitempty"`
    Priority int    `json:"priority"`           // 0..100; lower = preferred
    Weight   int    `json:"weight"`             // 0..10000
}

type NodeCapabilities struct {
    Validator       bool   `json:"validator,omitempty"`
    Cache           bool   `json:"cache,omitempty"`
    Archive         bool   `json:"archive,omitempty"`
    Bootstrap       bool   `json:"bootstrap,omitempty"`
    GossipSink      bool   `json:"gossipSink,omitempty"`
    IPFSGateway     bool   `json:"ipfsGateway,omitempty"`
    MaxBodyBytes    int    `json:"maxBodyBytes,omitempty"`
    MinPeerProtocol string `json:"minPeerProtocol,omitempty"`
}
```

**Validation rules (v1.0):** per QDP-0014 §4. Summarizing:

1. `NodeQuid` MUST equal the quid derived from
   `BaseTransaction.PublicKey` (node signs its own ad).
2. There MUST be a current TRUST edge from `OperatorQuid`
   to `NodeQuid` at weight ≥ 0.5 in a domain of form
   `operators.network.<operator-domain>`.
3. `ExpiresAt` MUST be in the future AND no more than 7
   days (604800 seconds) out.
4. `AdvertisementNonce` MUST be strictly monotonic per
   `NodeQuid`.
5. Each `Endpoints[].URL` MUST start with `https://`.
6. `Capabilities.Validator == true` is honored only if the
   node is ALSO in some domain's `Validators` map.
7. `ProtocolVersion` MUST be parseable as semver `[OPEN:
   tighten to regex?]`.

### 4.8 `MODERATION_ACTION` (QDP-0015 Phase 1)

**Struct:**

**[VERIFY: pull from internal/core/moderation.go]**

Fields per QDP-0015 §4:

- `ModeratorQuid` — the governor / operator quid issuing.
- `TargetType` — one of `QUID`, `EVENT`, `DOMAIN`,
  `TRANSACTION`.
- `TargetID` — the subject of moderation.
- `Scope` — one of `suppress`, `hide`, `annotate` (severity
  decreases left to right).
- `ReasonCode` — from the standardized taxonomy (DMCA,
  CSAM, GDPR-erasure, court-order, policy-violation,
  duplicate, spam, ...).
- `Annotation` — optional freeform (required if
  `Scope == annotate`).
- `Nonce` — monotonic per moderator.
- `Duration` — optional seconds; zero = indefinite.

**Validation rules (v1.0):** per QDP-0015 §5 (12-rule
validator). Summarizing:

1. Moderator authority verification: `ModeratorQuid` MUST
   be a governor on `TrustDomain` OR on a parent domain
   that delegates moderation authority.
2. Target existence check: `TargetID` MUST resolve.
3. Enum validation on `TargetType`, `Scope`, `ReasonCode`.
4. Supersede-chain resolution: a new action on the same
   target supersedes the prior if signed by equal-or-
   greater authority.
5. Nonce monotonicity.
6. Signature.

### 4.9 `DATA_SUBJECT_REQUEST` (QDP-0017 Phase 1)

**Struct:**

**[VERIFY: pull from internal/core/privacy.go]**

Fields per QDP-0017 §4:

- `SubjectQuid` — the data subject requesting.
- `RequestType` — `access`, `erasure`, `rectification`,
  `portability`, `restrict-processing`, `object`.
- `Scope` — subset of the subject's data the request
  applies to.
- `LegalBasis` — `gdpr-art-15`, `ccpa-right-to-know`,
  `lgpd-art-18`, etc.
- `Deadline` — Unix seconds; statutory deadline for
  operator response.

**Validation rules (v1.0):** per QDP-0017 §5. Summarizing:

1. Self-signed: signer equals `SubjectQuid`.
2. Enum validation on `RequestType` and `LegalBasis`.
3. `Deadline` MUST be in the future.

### 4.10 `CONSENT_GRANT` (QDP-0017 Phase 1)

**Struct:**

Fields per QDP-0017 §4:

- `GranterQuid` — the subject granting consent.
- `ProcessorQuid` — the operator / third party receiving
  consent.
- `Scopes` — list of specific processing uses (e.g.,
  `reputation-computation`, `marketing-analytics`,
  `research-use`).
- `PolicyVersion` — hash or version string of the policy
  consented to.
- `EffectiveFrom` — Unix seconds.
- `ExpiresAt` — Unix seconds; per QDP-0022 `ValidUntil`
  semantics.

**Validation rules:** self-signed by `GranterQuid`;
`EffectiveFrom` MUST be present; enum validation on scopes;
`PolicyVersion` MUST be non-empty.

### 4.11 `CONSENT_WITHDRAW` (QDP-0017 Phase 1)

**Struct:**

Fields:

- `GranterQuid` — same as original grant.
- `OriginalConsentRef` — ID of the `CONSENT_GRANT` being
  withdrawn.
- `EffectiveFrom` — Unix seconds; withdrawal takes effect
  at this timestamp.

**Validation:** self-signed; `OriginalConsentRef` MUST
exist in the privacy registry; `EffectiveFrom` MUST be
after the grant's `EffectiveFrom`.

### 4.12 `PROCESSING_RESTRICTION` (QDP-0017 Phase 1)

**Struct:**

Fields:

- `SubjectQuid`.
- `ProcessorQuid`.
- `RestrictedUses` — list of uses to restrict (subset of
  consent-scope enum).
- `EffectiveFrom`, `EffectiveUntil` — Unix seconds.

**Validation:** self-signed by `SubjectQuid`.

### 4.13 `DSR_COMPLIANCE` (QDP-0017 Phase 1)

**Struct:**

Fields:

- `OperatorQuid` — the operator proving compliance.
- `OriginalRequestRef` — ID of the `DATA_SUBJECT_REQUEST`.
- `Outcome` — `fulfilled`, `partially-fulfilled`,
  `denied-with-rationale`, `in-progress`.
- `Rationale` — freeform; required for all outcomes except
  `fulfilled`.
- `CompletedAt` — Unix seconds.

**Validation rules (v1.0):**

1. **Operator-only:** signer MUST be a validator on
   `TrustDomain` (not any quid can publish these).
2. `OriginalRequestRef` MUST exist in the privacy registry.
3. `Outcome` enum.
4. `Rationale` required unless `Outcome == fulfilled`.

## 5. Event type catalog

Events live inside `EventTransaction`. The `EventType`
field is a free string at the protocol level, but v1.0
reserves certain names with standardized payload schemas.
Implementations MAY use unreserved event types for
application-specific purposes.

### 5.1 Reserved event-type namespaces

- `quidnug.*` — protocol-internal event types.
- `reviews.*` — QRP-0001 reviews use case.
- `dns.*` — QDP-0023 DNS attestation use case.
- `wires.*` — interbank-wire-authorization use case.
- `elections.*` — elections use case.
- `health.*` — healthcare-consent use case.
- `credential.*` — credential-verification use case.

Application-defined event types SHOULD use a reverse-DNS
namespace (e.g., `com.example.orderplaced`) to avoid
collisions.

### 5.2 Protocol-internal event types

**[OPEN: enumerate the exhaustive list]**

Known protocol-internal types:

- `quidnug.domain.registered`
- `quidnug.domain.governance.updated` (QDP-0012 Phase 2)
- `quidnug.node.retired`
- `quidnug.fork-block.activated`

**[OPEN: do these have formal payload schemas or freeform
payloads? Document both shapes.]**

### 5.3 Reviews use-case event types

Per QRP-0001:

- `reviews.review.created` — initial review posting.
- `reviews.review.updated` — edit.
- `reviews.review.retracted` — withdrawal.
- `reviews.response.posted` — merchant response.
- `reviews.endorsement` — per-observer endorsement.

Payload schemas enumerated in `docs/reviews/QRP-0001.md`
**[VERIFY: path]**.

### 5.4 Interbank wires event types

Per `UseCases/interbank-wire-authorization/`:

- `wires.authorized`
- `wires.sent`
- `wires.received`
- `wires.settled`
- `wires.failed`

### 5.5 Elections event types

Per `UseCases/elections/`:

- `elections.voter.registered`
- `elections.checkin`
- `elections.ballot.issued`
- `elections.polls.opened`
- `elections.polls.closed`
- `elections.incident.reported`

Ballot casts are TRUST transactions with a `ballotProof`
field per QDP-0021, not EVENT transactions.

### 5.6 DNS attestation event types (QDP-0023; not in v1.0 unless launch-gated)

- `dns.claim`
- `dns.challenge`
- `dns.attestation`
- `dns.renewal`
- `dns.revocation`

### 5.7 Group encryption event types (QDP-0024; not in v1.0 unless launch-gated)

- `groups.create`
- `groups.epoch.advance`
- `groups.member.keypackage`
- `groups.record.encrypted`
- `groups.member.invite`
- `groups.member.recovery`

### 5.8 Payload size + encoding

- Inline payloads (`EventTransaction.Payload`) MUST NOT
  exceed **`MaxPayloadSize`**, set to **64 KB** per
  `internal/core/validation.go:235`. The constant is
  bytes of the serialized JSON payload.
- Larger payloads MUST use `PayloadCID` with content stored
  in IPFS. The CID MUST resolve to a valid multihash.
- Payload JSON MUST NOT contain object keys that collide
  with reserved top-level transaction fields: `id`, `type`,
  `trustDomain`, `timestamp`, `signature`, `publicKey`,
  plus the tx-type-specific reserved fields enumerated in
  §4 per transaction (e.g., TRUST's `truster` / `trustee` /
  `trustLevel` / `nonce` / `description` / `validUntil`).
  In practice the payload is nested inside the tx struct's
  `payload` field so collisions are only visible if a
  caller attempts to flatten the payload into the enclosing
  object. Don't do that.

## 6. Endpoint surface

All REST endpoints are served over HTTPS (TLS 1.3 required;
TLS 1.2 MAY be offered for legacy clients). Request /
response bodies are JSON.

### 6.1 Response envelope

All v1.0 responses (both success and error) use:

```json
{
  "success": true | false,
  "data": <response data on success>,
  "error": {
    "code": "<machine-readable enum>",
    "message": "<human-readable>",
    "details": <optional, per-error>
  }
}
```

HTTP status codes follow REST conventions: 2xx on
`success: true`, 4xx for client errors, 5xx for server
errors.

### 6.2 Authentication

v1.0 endpoints support three authentication modes:

1. **None** (public reads): anonymous queries against
   public / trust-gated records.
2. **Bearer token** (`Authorization: Bearer <token>`):
   operator-managed access for node-admin endpoints.
3. **Quid-signed request** (`X-Quidnug-Signature` header +
   `X-Quidnug-Quid`): client proves a quid identity for
   trust-gated record access. Signed data: canonical form
   of the request URL + body + timestamp.

**[OPEN: formalize signature-auth header format]**
Used by QDP-0023 trust-gated DNS records and by certain
private-records APIs. Needs a fully-specified signature
envelope (what's signed, what the nonce space is, how
replay is prevented). Candidate: HTTP Message Signatures
(RFC 9421).

### 6.3 Endpoint catalog

Routes are mounted by `internal/core/handlers.go:
StartServerWithConfig` under three path prefixes:

- `/api/v1/...` — versioned alias of the core surface.
- `/api/...` — unversioned alias of the core surface
  (identical route set; kept for backward compatibility).
- `/api/v2/...` — post-v1 additions: QDP-0002 guardians,
  QDP-0003 cross-domain gossip, QDP-0014 discovery.

Every core route is reachable at both `/api/v1/<route>` and
`/api/<route>`. The table below uses the unversioned form
for brevity; replace with `/api/v1/` to pin to v1 when
authoring integrations that should survive future default-
prefix churn.

Static paths (not mounted in the Go router):

| Method | Path | Purpose |
|---|---|---|
| GET | `/.well-known/quidnug-network.json` | Operator network descriptor (QDP-0014). Emitted by `quidnug-cli well-known generate`; served by any reverse proxy in front of the node. |
| GET | `/metrics` | Prometheus metrics (`promhttp.Handler`). Mounted directly at root, NOT under `/api/`. |

#### 6.3.1 Core transaction submission

| Method | Path | Handler | Purpose |
|---|---|---|---|
| POST | `/api/transactions/trust` | `CreateTrustTransactionHandler` | Submit TRUST tx |
| POST | `/api/transactions/identity` | `CreateIdentityTransactionHandler` | Submit IDENTITY tx |
| POST | `/api/transactions/title` | `CreateTitleTransactionHandler` | Submit TITLE tx |
| POST | `/api/events` | `CreateEventTransactionHandler` | Submit EVENT tx |
| POST | `/api/node-advertisements` | `CreateNodeAdvertisementHandler` | Submit NODE_ADVERTISEMENT (QDP-0014) |
| POST | `/api/quids` | `CreateQuidHandler` | Server-side quid generation (test utility) |

#### 6.3.2 Read-side queries

| Method | Path | Handler | Purpose |
|---|---|---|---|
| GET | `/api/health` | `HealthCheckHandler` | Liveness probe |
| GET | `/api/info` | `GetInfoHandler` | Node metadata + supported features |
| GET | `/api/nodes` | `GetNodesHandler` | Peer list |
| GET | `/api/transactions` | `GetTransactionsHandler` | Paginated tx list |
| GET | `/api/blocks` | `GetBlocksHandler` | Block tip + paginated list |
| GET | `/api/blocks/tentative/{domain}` | `GetTentativeBlocksHandler` | Tiered-acceptance pending blocks (QDP-0010) |
| GET | `/api/identity/{quidId}` | `GetIdentityHandler` | Identity record or 404 |
| GET | `/api/title/{assetId}` | `GetTitleHandler` | Title record or 404 |
| GET | `/api/trust/{observer}/{target}` | `GetTrustHandler` | Relational trust query |
| GET | `/api/trust/edges/{quidId}` | `GetTrustEdgesHandler` | Outbound edges |
| POST | `/api/trust/query` | `RelationalTrustQueryHandler` | Batch trust query (structured) |
| GET | `/api/streams/{subjectId}` | `GetEventStreamHandler` | Stream metadata |
| GET | `/api/streams/{subjectId}/events` | `GetStreamEventsHandler` | Paginated event list |

#### 6.3.3 Domain governance (QDP-0012)

| Method | Path | Handler | Purpose |
|---|---|---|---|
| GET | `/api/domains` | `GetDomainsHandler` | List known domains |
| POST | `/api/domains` | `RegisterDomainHandler` | Register a new domain |
| GET | `/api/domains/{name}/query` | `QueryDomainHandler` | Domain metadata |
| GET | `/api/registry/identity` | `QueryIdentityRegistryHandler` | Identity registry dump (paginated) |
| GET | `/api/registry/title` | `QueryTitleRegistryHandler` | Title registry dump |
| GET | `/api/registry/trust` | `QueryTrustRegistryHandler` | Trust-edge registry dump |
| GET | `/api/node/domains` | `GetNodeDomainsHandler` | Domains this node serves |
| POST | `/api/node/domains` | `UpdateNodeDomainsHandler` | Update served-domain list |
| POST | `/api/gossip/domains` | `ReceiveDomainGossipHandler` | Peer gossip: domain-registration sync |

#### 6.3.4 Discovery + sharding (QDP-0014)

v2-only. Each route is at `/api/v2/discovery/...`.

| Method | Path | Handler | Purpose |
|---|---|---|---|
| GET | `/api/v2/discovery/domain/{name}` | `DiscoveryDomainHandler` | Consortium + endpoint hints for a domain |
| GET | `/api/v2/discovery/node/{quid}` | `DiscoveryNodeHandler` | Current advertisement for a node |
| GET | `/api/v2/discovery/operator/{quid}` | `DiscoveryOperatorHandler` | All advertisements attested by an operator |
| GET | `/api/v2/discovery/quids` | `DiscoveryQuidsHandler` | Per-domain quid index (sortable by activity, etc.) |
| GET | `/api/v2/discovery/trusted-quids` | `DiscoveryTrustedQuidsHandler` | Consortium-blessed quid subset |

#### 6.3.5 Guardian recovery (QDP-0002 + QDP-0006)

v2-only. Each route is at `/api/v2/guardian/...`.

| Method | Path | Handler | Purpose |
|---|---|---|---|
| POST | `/api/v2/guardian/set-update` | `SubmitGuardianSetUpdateHandler` | Publish / rotate a guardian set |
| POST | `/api/v2/guardian/recovery/init` | `SubmitGuardianRecoveryInitHandler` | Start time-locked recovery |
| POST | `/api/v2/guardian/recovery/veto` | `SubmitGuardianRecoveryVetoHandler` | Veto in-flight recovery |
| POST | `/api/v2/guardian/recovery/commit` | `SubmitGuardianRecoveryCommitHandler` | Finalize after time-lock |
| POST | `/api/v2/guardian/resign` | `SubmitGuardianResignationHandler` | Resign a guardian (QDP-0006) |
| GET | `/api/v2/guardian/set/{quid}` | `GetGuardianSetHandler` | Current guardian set for a quid |
| GET | `/api/v2/guardian/pending-recovery/{quid}` | `GetPendingRecoveryHandler` | In-flight recovery state |
| GET | `/api/v2/guardian/resignations/{quid}` | `GetGuardianResignationsHandler` | Resignation history |

#### 6.3.6 Cross-domain gossip + fingerprints (QDP-0003 + QDP-0005)

v2-only. Each route is at `/api/v2/...`.

| Method | Path | Handler | Purpose |
|---|---|---|---|
| POST | `/api/v2/anchor-gossip` | `SubmitAnchorGossipHandler` | Publish cross-domain nonce anchor |
| POST | `/api/v2/domain-fingerprints` | `SubmitDomainFingerprintHandler` | Publish domain fingerprint |
| GET | `/api/v2/domain-fingerprints/{domain}/latest` | `GetLatestDomainFingerprintHandler` | Latest fingerprint for a domain |
| POST | `/api/v2/gossip/push-anchor` | `ReceiveAnchorPushHandler` | Receive pushed anchor |
| POST | `/api/v2/gossip/push-fingerprint` | `ReceiveFingerprintPushHandler` | Receive pushed fingerprint |

#### 6.3.7 Fork-block activation (QDP-0009)

| Method | Path | Handler | Purpose |
|---|---|---|---|
| POST | `/api/fork-block` | `SubmitForkBlockHandler` | Submit a signed fork activation |
| GET | `/api/fork-block/status` | `GetForkBlockStatusHandler` | Current fork-block state |

#### 6.3.8 Bootstrap snapshots (QDP-0008)

| Method | Path | Handler | Purpose |
|---|---|---|---|
| POST | `/api/nonce-snapshots` | `SubmitNonceSnapshotHandler` | Publish K-of-K snapshot |
| GET | `/api/nonce-snapshots/{domain}/latest` | `GetLatestNonceSnapshotHandler` | Latest snapshot for a domain |
| GET | `/api/bootstrap/status` | `GetBootstrapStatusHandler` | Bootstrap progress |

#### 6.3.9 Moderation (QDP-0015 Phase 1)

| Method | Path | Handler | Purpose |
|---|---|---|---|
| POST | `/api/moderation/actions` | `CreateModerationActionHandler` | Submit a moderation action |
| GET | `/api/moderation/actions/{targetType}/{targetId}` | `GetModerationActionsHandler` | Effective action for a target |

#### 6.3.10 Privacy + data-subject rights (QDP-0017 Phase 1)

| Method | Path | Handler | Purpose |
|---|---|---|---|
| POST | `/api/privacy/dsr` | `CreateDSRHandler` | Submit a data-subject request |
| GET | `/api/privacy/dsr/{requestTxId}` | `GetDSRStatusHandler` | DSR status + compliance record |
| POST | `/api/privacy/consent/grants` | `CreateConsentGrantHandler` | Submit CONSENT_GRANT |
| POST | `/api/privacy/consent/withdraws` | `CreateConsentWithdrawHandler` | Submit CONSENT_WITHDRAW |
| GET | `/api/privacy/consent/history` | `GetConsentHistoryHandler` | Per-granter grant + withdraw history |
| POST | `/api/privacy/restrictions` | `CreateProcessingRestrictionHandler` | Submit PROCESSING_RESTRICTION |
| GET | `/api/privacy/restrictions/{subjectQuid}` | `GetRestrictionsForSubjectHandler` | Active restrictions for a subject |
| POST | `/api/privacy/compliance` | `CreateDSRComplianceHandler` | Submit DSR_COMPLIANCE (validator-only) |

#### 6.3.11 Audit log (QDP-0018 Phase 1)

| Method | Path | Handler | Purpose |
|---|---|---|---|
| GET | `/api/audit/head` | `AuditHeadHandler` | Hash-chain head |
| GET | `/api/audit/entries` | `AuditEntriesHandler` | Paginated entries |
| GET | `/api/audit/entry/{sequence}` | `AuditEntryHandler` | Single entry by sequence |

#### 6.3.12 IPFS bridge

| Method | Path | Handler | Purpose |
|---|---|---|---|
| POST | `/api/ipfs/pin` | `PinToIPFSHandler` | Pin content to IPFS |
| GET | `/api/ipfs/{cid}` | `GetFromIPFSHandler` | Fetch content by CID |

#### 6.3.13 Authentication

v1.0 authentication is uniform: no built-in auth on most
routes, with node-configurable middleware for bearer
tokens applied in front of the mux.

- **Public routes** (no auth required): all read-side
  queries + all transaction submissions. Transactions
  authenticate via their embedded `signature` field.
- **Operator-scoped routes** (bearer token via
  `Authorization: Bearer <token>`, configured at node
  startup): `/api/audit/*`, `/api/node/domains` POST,
  `/api/privacy/compliance` POST (validator-only).
- **Rate-limited routes**: all POST routes pass through
  the `MultiLayerLimiter` (QDP-0016 Phase 1) per
  quid / operator / domain.

### 6.4 Endpoints deferred (not in v1.0 unless launch-gated)

- QDP-0023 DNS attestation endpoints (`/api/v2/dns/*`)
- QDP-0024 group encryption endpoints (`/api/v2/groups/*`)
- QDP-0021 blind-signature ballot issuance endpoint
- QDP-0018 Phase 3+ verification endpoints
- QDP-0015 Phase 2 federation-import endpoint
- QDP-0017 CLI-auto-fulfill integration endpoint

### 6.5 Versioning

Endpoint paths of form `/api/v2/*` indicate QDP-0014-era
additions. v1.0 keeps both `/api/*` (original) and
`/api/v2/*` paths. Post-v1.0, new endpoint families SHOULD
use `/api/v3/*` or a capability-negotiated path per
QDP-0020.

## 7. Registry state shapes

A v1.0 node maintains the following in-memory registries
with disk-backed persistence.

### 7.1 Trust-domain registry

Map `name` → `TrustDomain` struct (§3.1 of types.go).

### 7.2 Identity registry

Map `quid-id` → `IdentityState`:

- Current public key
- Current `UpdateNonce` (for replay protection)
- Creator reference
- Home domain
- Attributes snapshot
- Revocation flag
- Guardian quorum reference (QDP-0002)

### 7.3 Trust edge registry

Map `(truster, trustee, domain)` → `TrustEdgeState`:

- Current trust level
- Current nonce
- Description
- `ValidUntil` (QDP-0022)
- Origin event ID

Secondary indices:

- By truster (outbound edges).
- By trustee (inbound edges).
- By domain (domain-scoped walk).

### 7.4 Event stream registry

Per-`subjectID` `EventStream` state (types.go):

- Latest sequence
- Event count
- Created / updated timestamps
- Latest event ID

Plus the event-log itself (append-only, indexed by
sequence).

### 7.5 Nonce ledger (QDP-0001)

Map `(quid, domain, epoch)` → nonce-state:

- Highest accepted
- Tentative reservations (for pending transactions)
- Anchored value (cross-domain anchor reference)

### 7.6 Node-advertisement registry (QDP-0014)

Map `nodeQuid` → most-recent `NodeAdvertisementTransaction`.

Secondary indices:

- By operator quid.
- By supported domain pattern.
- By capability (validator / cache / archive / bootstrap).

Expiry GC runs periodically; expired advertisements removed.

### 7.7 Moderation registry (QDP-0015)

Map `(targetType, targetId)` → list of applicable actions,
resolved via supersede-chain to a single effective action.

### 7.8 Privacy registry (QDP-0017)

Five indices:

- Consent grants by granter quid.
- Consent withdrawals by granter quid + original-ref.
- Processing restrictions by subject quid.
- DSR requests by subject quid.
- DSR compliance records by original-request-ref.

Consent expiry GC per QDP-0022 hides withdrawn /
lapsed records from active-consent queries (but record
remains in history).

### 7.9 TTL registry (QDP-0022)

Map of `(txRef)` → `ValidUntil` for fast expiry filtering
at serving time. Shared between trust-edge registry and
privacy registry.

### 7.10 Audit log (QDP-0018)

Per-operator hash-chained audit log with disk persistence:

- `Entry` struct with sequence, timestamp, category,
  actor quid, reference, prev-entry hash, self-hash.
- Append-only; replayed on node startup.
- Categories: `moderation`, `dsr`, `consent`, `withdrawal`,
  `restriction`, `compliance`, `node-lifecycle`, and eight
  others (**[VERIFY: enumerate]**).

### 7.11 Block ledger

Append-only sequence of `Block` records with:

- Block hash index.
- Transaction-to-block mapping.
- Validator-signatures index.
- Block Merkle tree (QDP-0010; post-activation).

## 8. Gossip + block production

### 8.1 Push-based gossip (QDP-0005)

Implemented in `internal/core/gossip_push.go`. Operators
configure peer sets; transactions + blocks propagate on a
push schedule (QDP-0005 §4) with the parameters:

- Push interval: `[OPEN: specify — 5s? 10s?]`
- Peer fan-out: `[OPEN: specify]`
- Max message size: `[OPEN: specify]`

### 8.2 Block production

Block-producer quorum per QDP-0012:

- Consortium members authorized via `Validators` on
  `TrustDomain`.
- Block interval default: `[OPEN: 60s? per-domain?]`
- Block acceptance: `ValidatorTrustInCreator * weight >=
  domain's TrustThreshold`.
- Tiered acceptance per QDP-0010: tentative → trusted →
  archival.

### 8.3 Fork-block migration (QDP-0009)

Implemented in `internal/core/fork_block.go`. Used to
migrate protocol behavior at a scheduled block height.
Migration activation: `ForkBlock` tx type carrying the
activation scheduled-height and the new-rule payload.

Untested in production at v1.0 freeze time. **[OPEN:
exercise once on staging before launch.]**

## 9. Federation semantics (QDP-0013)

### 9.1 What v1.0 federation actually does

Every Quidnug network runs the same protocol. Two networks
can federate by configuring `external_trust_sources` to
reference each other.

`TRUST_IMPORT` transaction **[VERIFY: implementation
status]** imports a foreign network's TRUST edges into the
importing network's state. Imported edges are tagged by
origin and can be weighted differently from local edges.

### 9.2 Imported reputation weighting

Default policy: imported edges at 80% of original weight
(configurable per-external-source). **[OPEN: specify
reference default + tuning knobs.]**

### 9.3 Cross-network identity

A quid in network A is a distinct identity from a quid in
network B even if it uses the same public key. Applications
that treat cross-network identity as equivalent do so
explicitly by publishing `TRUST_IMPORT` bindings.

**[OPEN: this is an important point and needs a much
fuller treatment in §9. Expand.]**

## 10. Versioning + forward compatibility (QDP-0020)

### 10.1 Protocol version

v1.0 advertises `protocolVersion: "1.0.0"` in
`NodeAdvertisement.ProtocolVersion` and
`.well-known/quidnug-network.json`.

### 10.2 Capability negotiation

Clients connecting to a node consult the well-known
descriptor for capability flags:

- `"blind-signature-issuance"` (QDP-0021)
- `"dns-attestation"` (QDP-0023)
- `"group-encryption"` (QDP-0024)
- `"domain-governance-v2"` (QDP-0012 Phase 2)
- etc.

Missing capability in the descriptor means the node does
not implement that feature.

**[OPEN: enumerate full capability-flag vocabulary.]**

### 10.3 Deprecation + migration

Per QDP-0020:

- Deprecated features receive 18 months of notice before
  removal.
- Breaking protocol changes activate via fork-block
  (QDP-0009) at a scheduled block height.
- v1.1 features can be introduced without fork-block if
  they are backward-compatible (additive only).

## 11. Invariants + guarantees

### 11.1 Safety

**S1 (no equivocal trust edges).** For any
`(truster, trustee, domain)` tuple, at most one trust edge
is active at any block height. Prior edges are superseded
by nonce.

**S2 (no replay).** A TRUST, IDENTITY, TITLE, or EVENT
transaction with a nonce already seen in its nonce-ledger
key is rejected (per QDP-0001 enforcement mode).

**S3 (no expired edges in active queries).** `GetTrustLevel`
does not traverse edges whose `ValidUntil <= now` (per
QDP-0022).

**S4 (signature authenticity).** Any transaction whose
signature does not verify against its declared public key
is rejected at submission.

**S5 (linked-list integrity on event streams).** An
`EventTransaction` whose `PreviousEventID` does not match
the stream's most recent event is rejected.

**S6 (append-only ledger).** No block or transaction is
removed from the ledger after acceptance. Moderation
actions suppress at serving time but do not mutate the
chain.

### 11.2 Liveness

**L1 (consortium progress under f-of-n).** A trust domain
with `n` consortium members tolerates `f = n - ceil(n *
TrustThreshold)` non-responsive members while still
producing blocks.

**L2 (bounded staleness under network partition).** Upon
healing, gossip delivers lagging state within `[OPEN:
specify bound]` blocks.

### 11.3 Integrity

**I1 (operator log tamper-evidence).** QDP-0018 hash-chained
audit log: modifying any entry post-write invalidates every
subsequent entry's hash.

**I2 (block Merkle proofs).** Post-QDP-0010 activation,
inclusion of a transaction in a block is provable with
`O(log(n))` hash operations.

### 11.4 Privacy

**P1 (consent honoring at serving time).** Queries for
data that requires consent are filtered at serving time by
the privacy registry (QDP-0017).

**P2 (processing restrictions honored).** Operators MUST
NOT perform processing uses that a subject has restricted
via `PROCESSING_RESTRICTION`.

### 11.5 Explicit non-guarantees

- **Global consistency.** Different observers may see
  different trust weights for the same pair, due to
  domain-scoping + relational walking. By design.
- **Irrevocable anonymity.** Events are signed;
  pseudonymity is the default but not anonymity.
- **Byzantine fault tolerance beyond consortium quorum.**
  A compromised quorum can produce invalid blocks; defense
  relies on governor quorum + guardian recovery.

**[OPEN: this section needs a companion `protocol-
invariants.md` with precise formal statements.]**

## 12. Launch scope

### 12.1 v1.0 definition

The v1.0 protocol comprises:

1. All transaction types enumerated in §4.1 rows marked
   "Required" in the "v1.0 status" column.
2. The endpoint catalog in §6.3 rows marked "Required".
3. The cryptographic primitives in §3.1 rows 1-3 (ECDSA,
   SHA-256).
4. The canonical byte form in §2.2 and the signing protocol
   in §3.4.
5. The gossip + block-production protocol in §8.
6. The federation semantics in §9.
7. The invariants in §11.

### 12.2 Launch-blocking items before freeze

Items that MUST land before v1.0 freeze:

- **QDP-0019 Phase 1** (reputation decay): required for
  QDP-0023's transitive weighting to behave correctly.
  Without this, DNS attestation weighting math doesn't
  work.
- **QDP-0020 Phase 1** (protocol versioning): required to
  freeze the version-negotiation story.
- **QDP-0013 Phase 1** (federation): the TRUST_IMPORT
  transaction + `external_trust_sources` config. Critical
  for multi-root DNS attestation.
- **Cross-SDK byte-compatibility test matrix** covering all
  Required transaction types.
- **Genesis state + bootstrap ceremony** (see
  `docs/launch/genesis.md`, to be written).

### 12.3 Deferred to v1.1+ (post-launch)

- QDP-0015/0016/0017/0018 Phases 2-5 (operational tooling).
- QDP-0021 blind-signature ballot issuance (elections
  use case).
- Mobile SDKs, additional-language SDK parity.
- Additional framework adapters for reviews.
- Additional vertical integrations.
- Post-quantum migration.

### 12.4 Maybe in v1.0, maybe in v1.1 (design decision pending)

- **QDP-0023 DNS-anchored attestation.** Strategic
  adoption flywheel. Target: included in v1.0.
- **QDP-0024 private communications.** Backs `private:*`
  in QDP-0023. Target: included in v1.0.

**Decision gate:** If QDP-0023 + QDP-0024 are included,
v1.0 freeze happens only after both land Phase 1 and pass
cross-SDK tests. If deferred, v1.0 can freeze sooner but
the adoption story is weaker.

## 13. Test vector appendix

Reference test vectors live in
`docs/test-vectors/v1.0/` **[OPEN: build this directory]**
as JSON files, one per transaction type:

- `test-vectors/v1.0/trust-tx.json`
- `test-vectors/v1.0/identity-tx.json`
- `test-vectors/v1.0/title-tx.json`
- `test-vectors/v1.0/event-tx.json`
- `test-vectors/v1.0/node-advertisement-tx.json`
- `test-vectors/v1.0/moderation-action-tx.json`
- `test-vectors/v1.0/dsr-tx.json` (covers DSR + consent +
  restriction + compliance)

Each file contains an array of test cases:

```json
[
  {
    "name": "trust: basic trust edge",
    "input": { /* struct fields */ },
    "canonical_signable_bytes_hex": "...",
    "signature_hex": "...",
    "expected_id": "...",
    "signing_private_key_hex": "...",  // test key only
    "signing_public_key_hex": "...",
    "comments": "..."
  },
  ...
]
```

Every SDK's test suite MUST consume this file and verify:

1. Canonical byte computation matches `canonical_signable_bytes_hex`.
2. Signing with `signing_private_key_hex` produces
   `signature_hex` (assuming deterministic RFC 6979; see
   §2.3 open item).
3. ID computation matches `expected_id`.
4. Verification succeeds with `signing_public_key_hex`.

A CI job MUST run all SDKs' vector tests on every commit
to prevent divergence.

## 14. Conformance + compliance statement

An implementation is **v1.0 conformant** if it:

1. Accepts every Required transaction type per §4 with
   correct validation per the stated rules.
2. Emits only v1.0 transaction types (no proprietary
   additions) unless they match the format constraints of
   §2 for forward compatibility.
3. Serves every Required endpoint per §6 with the response
   envelope of §6.1.
4. Verifies signatures per §3.4 exactly (including
   canonical byte form per §2.2).
5. Passes the §13 test vector matrix for every transaction
   type used.
6. Honors the invariants of §11 at every boundary.
7. Advertises `protocolVersion: "1.0.0"` in its
   `NodeAdvertisement` and `.well-known/quidnug-network.json`.

Conformance testing harness: **[OPEN: spec a conformance
test suite similar to W3C's browser-WPT model.]**

## 15. Open questions consolidated

Status of each `[OPEN]` marker, most-recently-updated
first. Resolved items kept in the list for audit trail.

### Resolved

- **§2.2** — Canonical byte form cross-SDK determinism.
  **RESOLVED.** Eight SDKs (Go reference, pkg/client,
  Python, Rust, JS, Java, .NET, Swift, browser extension)
  have conforming test-vector consumers at
  `docs/test-vectors/v1.0/`. See the Conformance Status
  section there.
- **§2.3** — RFC 6979 deterministic signing mandate.
  **RESOLVED.** Reference node MUST use RFC 6979 + low-s.
  SDKs SHOULD migrate. Verified against RFC 6979 Appendix
  A.2.5 in `internal/core/rfc6979_test.go`.
- **§2.4** — Per-type ID derivation verification.
  **RESOLVED.** Every SDK's test suite derives IDs locally
  and compares against the vector `expected_id`.
- **§3.5** — Low-s signature enforcement.
  **RESOLVED.** Reference node signs low-s automatically.
  Verification accepts both forms in v1.0; strict
  rejection deferred to v1.1 via fork-block.
- **§4.0** — Size + length constants.
  **RESOLVED.** Constants table at §4.0 pulls actual
  values from `internal/core` source.
- **§4.2** — `MaxDescriptionLength` exact value. 4096
  bytes (see §4.0 table).
- **§4.5** — `MaxPayloadSize` (inline). 64 KB
  (see §4.0 table; renamed from `MaxInlinePayloadBytes`).
- **§6.3** — Endpoint catalog cross-check. **RESOLVED.**
  Catalog in §6.3 now enumerates all 66 routes across
  `/api/v1/`, `/api/`, and `/api/v2/` prefixes, grouped
  by functional area.
- **§13** — Test vector directory build. **RESOLVED.**
  Directory exists with 24 cases across 7 files covering
  every Required transaction type.

### Still open

1. **§2.7** — Timestamp unit unification (seconds vs ns).
2. **§4.1** — `TxTypeGeneric` — keep or remove?
3. **§4.1** — Which Draft QDPs actually land in v1.0?
4. **§4.3** — Reserved attribute keys on IDENTITY.
5. **§4.4** — TITLE asset ID canonical format.
6. **§4.4** — TITLE per-type transfer threshold table.
7. **§4.5** — `Payload` + `PayloadCID` mutual exclusion
   enforcement at the node layer (currently SDK-enforced;
   v1.1 tightening).
8. **§5.2** — Protocol-internal event-type exhaustive
   list + schemas.
9. **§5.6/5.7** — Whether DNS attestation + group
   encryption event types are in v1.0.
10. **§6.2** — Quid-signed request header format (for
    trust-gated reads of private records).
11. **§7.10** — Audit log category enumeration.
12. **§8.1** — Gossip interval + fan-out parameters.
13. **§8.2** — Block interval + acceptance thresholds.
14. **§8.3** — Fork-block exercised on staging once before
    launch.
15. **§9** — Federation semantics require fuller §9
    treatment.
16. **§9.2** — Import-weight reference default.
17. **§10.2** — Full capability-flag vocabulary.
18. **§11.2** — Bounded-staleness numeric bound.
19. **§11.5** — Companion `protocol-invariants.md`.
20. **§12.4** — QDP-0023 + QDP-0024 in-or-out decision.
21. **§14** — Conformance test harness (external
    implementation runner).

Each open question gates some portion of the freeze.
Resolution produces an amendment commit that edits this
document inline, adds to the "Resolved" list, and removes
any `[OPEN]` marker in the body.

## 16. Document maintenance

- Changes pre-freeze: standard commits with descriptive
  messages.
- Changes post-freeze: require either an amendment QDP
  OR a fork-block migration depending on whether the
  change is additive (QDP-0020 capability) or breaking
  (QDP-0009 fork-block).
- Version history in `CHANGELOG.md` under
  `## protocol-v1.x`.

## 17. References

- [QDP-0001: Global Nonce Ledger](design/0001-global-nonce-ledger.md)
- [QDP-0002: Guardian-Based Recovery](design/0002-guardian-based-recovery.md)
- [QDP-0009: Fork-Block Migration Trigger](design/0009-fork-block-trigger.md)
- [QDP-0010: Compact Merkle Proofs](design/0010-compact-merkle-proofs.md)
- [QDP-0012: Domain Governance](design/0012-domain-governance.md)
- [QDP-0013: Network Federation Model](design/0013-network-federation.md)
- [QDP-0014: Node Discovery + Domain Sharding](design/0014-node-discovery-and-sharding.md)
- [QDP-0015: Content Moderation & Takedowns](design/0015-content-moderation.md)
- [QDP-0016: Abuse Prevention & Resource Limits](design/0016-abuse-prevention.md)
- [QDP-0017: Data Subject Rights & Privacy](design/0017-data-subject-rights.md)
- [QDP-0018: Observability + Tamper-Evident Operator Log](design/0018-observability-and-audit.md)
- [QDP-0019: Reputation Decay](design/0019-reputation-decay.md)
- [QDP-0020: Protocol Versioning](design/0020-protocol-versioning.md)
- [QDP-0021: Blind Signatures](design/0021-blind-signatures.md)
- [QDP-0022: Timed Trust & TTL](design/0022-timed-trust-and-ttl.md)
- [QDP-0023: DNS-Anchored Attestation](design/0023-dns-anchored-attestation.md)
- [QDP-0024: Private Communications](design/0024-private-communications.md)
- RFC 2119, RFC 6979, RFC 7748, RFC 8259, RFC 9420, RFC 9421, RFC 9474
- FIPS 180-4, FIPS 186-4, FIPS 197
- SEC 1 v2.0 (Certicom): elliptic-curve encoding
