# Quidnug v1.0 Cross-SDK Test Vectors

Canonical test cases that every v1.0-conformant SDK MUST
reproduce byte-identically. If two independent
implementations consume this directory and both pass, their
canonical serialization and signing behavior is compatible.

## Scope of this directory

```
docs/test-vectors/v1.0/
├── README.md                         (this file)
├── trust-tx.json                     (TRUST: 4 cases)
├── identity-tx.json                  (IDENTITY: 3 cases)
├── event-tx.json                     (EVENT: 3 cases)
├── title-tx.json                     (TITLE: 3 cases)
├── node-advertisement-tx.json        (NODE_ADVERTISEMENT: 2 cases)
├── moderation-action-tx.json         (MODERATION_ACTION: 3 cases)
├── dsr-tx.json                       (DSR family: 6 cases across 5 tx types)
└── test-keys/
    ├── key-alice.json
    ├── key-bob.json
    └── key-carol.json                (deterministic test keys; NEVER used in production)
```

Total: 24 conformance cases across 7 files covering all v1.0
Required transaction types.

## Authoritative canonical form (v1.0)

This section is the single source of truth for v1.0 canonical
byte form. The reference node's implementation in
`internal/core/validation.go` and the reviews integration
test in `internal/core/reviews_integration_test.go` both
follow this convention.

### Signable byte form

A transaction's **signable bytes** are computed by:

1. Starting from a fully-populated transaction struct
   (typed, e.g., `TrustTransaction`).
2. Setting the `Signature` field to the empty string.
3. Leaving all other fields at their submitted values
   (including the ID, which is bound by the signature).
4. Serializing with Go's `encoding/json` default settings:
   - Struct fields are emitted in **declaration order**
     (not alphabetical).
   - `json:` tag names are used where present.
   - `omitempty` fields are omitted when zero-valued.
   - No whitespace beyond JSON-required separators.

This produces UTF-8 bytes that are the authoritative
signable payload.

### Signature algorithm

1. `H = SHA-256(signable_bytes)`.
2. Sign `H` with ECDSA over NIST P-256 (current reference
   node uses non-deterministic K; RFC 6979 deterministic K
   is a proposed v1.0 requirement flagged as `[OPEN]` in
   `docs/protocol-v1.0.md` §2.3).
3. Encode the resulting `(r, s)` pair as **IEEE-1363**:
   `r_bytes || s_bytes`, each component zero-padded to
   exactly 32 bytes. Total: 64 bytes.
4. Represent on-wire as lowercase hex: 128 characters.

DER-encoded ECDSA signatures (as produced by
`ecdsa.SignASN1`) MUST NOT be accepted by v1.0 verifiers.
This is a divergence from the current `pkg/client` Go SDK
which produces DER; see **Known divergences** below.

### Transaction ID derivation

Each transaction type defines its own ID input set. IDs are
SHA-256 hashes of a type-specific subset of fields,
serialized with `encoding/json` on an anonymous struct in
the field order matching `internal/core/transactions.go`.
Specific derivations are documented inline in each vector
file.

### Public-key encoding

Public keys are SEC1 uncompressed-point encoding:
`0x04 || X || Y`, lowercase hex, 130 characters.

### Quid ID derivation

`quidID = hex(SHA-256(sec1_uncompressed_pubkey)[0:8])`,
16 lowercase hex characters.

## Vector file schema

Each vector file is a JSON array of test cases:

```json
{
  "schema_version": "1.0",
  "tx_type": "TRUST",
  "generated_at": "2026-04-20T00:00:00Z",
  "generator_commit": "<git-sha>",
  "canonical_form_notes": "...",
  "cases": [
    {
      "name": "<descriptive>",
      "comments": "<what this case proves>",
      "signer_key_ref": "alice",
      "input": { /* full tx struct */ },
      "expected": {
        "canonical_signable_bytes_hex": "<hex>",
        "canonical_signable_bytes_utf8": "<string>",
        "sha256_of_canonical_hex": "<hex>",
        "expected_id": "<hex>",
        "reference_signature_hex": "<hex>",
        "signature_length_bytes": 64
      }
    }
  ]
}
```

### Field notes

- `signer_key_ref`: a string referring to a key in
  `test-keys/key-<name>.json`. Keys are deterministic and
  checked in; they are NOT production keys and MUST NOT be
  trusted in any real network.
- `canonical_signable_bytes_hex`: hex encoding of the
  exact UTF-8 bytes produced by serializing the input per
  the authoritative canonical form above.
- `canonical_signable_bytes_utf8`: the same bytes rendered
  as a UTF-8 string, provided for human inspection. SDKs
  SHOULD verify their output matches the UTF-8 form OR the
  hex form; matching either is sufficient.
- `sha256_of_canonical_hex`: SHA-256 of the canonical
  bytes. Provided so SDKs can verify the hash computation
  independently.
- `expected_id`: the transaction ID derived per §2.4 of the
  v1.0 spec.
- `reference_signature_hex`: a reference signature over
  the SHA-256 of the canonical bytes, produced by the
  generator. Because ECDSA is currently non-deterministic,
  SDKs cannot reproduce this exact signature. They MUST
  verify that the reference signature VALIDATES, and they
  MAY produce their own valid signatures over the same
  bytes.

## SDK conformance contract

A v1.0-conformant SDK consuming this directory MUST, for
every case in every file:

1. **Reproduce canonical bytes.** Serialize `input` through
   the SDK's canonical-bytes routine; hex-encode the result;
   compare against `canonical_signable_bytes_hex`. MUST
   match byte-identically.

2. **Reproduce the ID.** Compute the transaction ID from
   `input` using the SDK's ID-derivation routine; compare
   against `expected_id`. MUST match.

3. **Verify the reference signature.** Given the test key's
   public key, the canonical bytes, and
   `reference_signature_hex`, the SDK's verification routine
   MUST return `true`.

4. **Reject a tampered signature.** Flip any bit in
   `reference_signature_hex` and re-submit to the SDK's
   verifier. MUST return `false`.

5. **Produce a valid independent signature.** Sign the
   canonical bytes with the test key. The result need not
   match `reference_signature_hex` (non-deterministic K)
   but MUST verify when passed back to the SDK's verifier.

All five properties together establish cross-SDK
compatibility. An SDK that passes 1-4 but fails 5 is broken.
An SDK that passes 3-5 but fails 1-2 has divergent
canonical form.

## Conformance status

All eight crypto-bearing SDKs have been migrated to v1.0
canonical form in lockstep with the vector harness.

| SDK | Signing path | Vector tests | Verified locally |
|---|---|---|---|
| `internal/core` (Go reference) | authoritative | 24/24 PASS | yes |
| `pkg/client` (Go SDK) | typed-mirror + IEEE-1363 | 24/24 PASS | yes |
| `clients/python` | `quidnug.wire` typed dataclasses + IEEE-1363 | 24/24 PASS + 4 wire tests | yes |
| `clients/rust` | typed `wire.rs` + IEEE-1363 + Go-compat float serializer | 3 tests (TRUST + IDENTITY + smoke) PASS | yes |
| `clients/js` | `v1-wire.js` + WebCrypto IEEE-1363 | 5 tests PASS | yes |
| `clients/java` | `Quid.sign` DER→IEEE-1363 + `VectorsTest` | 11 vector tests + 21 existing = 32/32 PASS | yes |
| `clients/dotnet` | `Quid.Sign` now returns IEEE-1363 natively + `VectorsTests.cs` | pending CI (runtime broken locally) | compile-verified only |
| `clients/swift` | `sig.rawRepresentation` + `VectorsTests.swift` | pending CI (Swift not on Windows) | code-review only |
| `clients/android` | `AndroidKeystoreSigner.sign` uses `Quid.derToIeee1363Sig` | follows Java SDK | Java tests pass |
| `clients/browser-extension` | `signWithQuid` returns IEEE-1363 directly + static regression probe | 3/3 PASS | yes |

### Remaining divergence probes (regression detectors)

Two tests remain in `pkg/client/vectors_test.go` to catch
accidental rollback:

- `TestPkgClientCanonicalBytesDivergesFromAuthoritative`:
  `pkg/client.CanonicalBytes()` retains alphabetical
  ordering for backward compat but is no longer on any v1.0
  signing path (submission uses `types_wire.go` mirrors).
  The probe documents the retained legacy behavior.
- `TestPkgClientSignNowMatchesAuthoritative`: asserts that
  `(*Quid).Sign` produces valid 64-byte IEEE-1363
  signatures that verify via an independent verifier.

Equivalent probes in `clients/python/tests/test_vectors.py`
(`test_clients_python_sdk_sign_diverges_from_authoritative`,
`test_clients_python_canonical_bytes_diverges_from_authoritative`)
self-heal now that the SDK converged: they log
"Divergence resolved" on the sign path and
"retained legacy behavior" on the canonical-bytes path.

### Per-SDK notes

**Go-compat float formatting.** Go's `encoding/json`
serializes `float64(1.0)` as `"1"`; most other languages
emit `"1.0"`. This matters for integer-valued floats on
fields like TRUST's `trust_level` (e.g. the
`trust_bob_to_alice_level_1.0_minimal_description` case).

Per-language handling:

- **Go** (reference): native. No special case.
- **Rust**: `wire.rs` installs `serialize_with =
  "serialize_go_compat_f64"` on `trust_level` + seed
  `TrustLevel`. Without it, serde_json emits `"1.0"`.
- **Python**: `wire.py` applies `_go_compat_value()` in
  both `_emit_signable` and `_seed_id`. Integer-valued
  finite floats cast to int before `json.dumps`.
- **JavaScript**: `JSON.stringify(1.0)` emits `"1"` natively
  (JS has a single Number type). No special case needed.
- **Java**: `VectorsTest.putGoCompatFloat()` and the
  per-test-type seed builders cast integer-valued floats
  to long before `JsonGenerator.writeNumberField`.
- **.NET**: `VectorsTests.WriteGoCompatFloat()` does the
  same cast before `Utf8JsonWriter.WriteNumber`.
- **Swift**: `VectorsTests.goCompatFloat()` returns `Int`
  for integer-valued `Double` so the hand-rolled JSON
  writer emits the integer form.

If you add test vectors with non-integer floats (e.g.
`trust_level: 0.123456789012345`), audit whether every
SDK round-trips the value bit-for-bit. The current vector
set uses values that round-trip cleanly under all reasonable
float encoders.

**JS SDK dual-path.** The legacy `clients/js/quidnug-client.js`
uses base64 keys + SPKI-derived quid IDs, incompatible with
the v1.0 server. The conformant path is the new
`clients/js/v1-wire.js` module exported at
`@quidnug/client/v1-wire`. The legacy module is in
quarantine pending full deprecation; new code should import
`v1-wire`.

**.NET verification pending.** The local dev environment at
commit time had a broken .NET 9 runtime config, so
`VectorsTests.cs` could not be executed locally. The file
compiles in isolation (System.Text.Json + Xunit only);
verification in CI is the next step.

**Swift verification pending.** Swift isn't available on
the author's dev box (Windows only), so `VectorsTests.swift`
could not be executed locally. The file targets Swift 5.9+
+ CryptoKit + Foundation; verification in macOS/Linux CI is
the next step. The `derRepresentation`→`rawRepresentation`
swap is a well-understood one-line change per Apple's
CryptoKit documentation.

**Android verification via Java.** The Android SDK's
`AndroidKeystoreSigner` wraps Java's `Signature.getInstance`
which returns DER, then converts via the shared
`Quid.derToIeee1363Sig` helper from the Java SDK. Tests in
the Java module exercise the helper; Android-specific tests
require an Android emulator or Instrumentation test harness.

- Consumer pending. Known to use `p256` crate; signature
  encoding configuration requires verification.

## Regeneration

Vectors are regenerated by:

```bash
go run ./cmd/quidnug-test-vectors generate \
    --out docs/test-vectors/v1.0
```

The generator uses the authoritative reference
implementation in `internal/core` and deterministic test
keys checked into `docs/test-vectors/v1.0/test-keys/`. Any
change to canonical form, signing, or ID derivation in
`internal/core` should be followed by regeneration +
review of the resulting diff.

A CI job on the reference repository SHOULD run vector
generation and diff against the checked-in version; any
drift is a bug.

## Using the vectors in a non-Go SDK

Pseudocode for a consumer in any language:

```
for each file in v1.0/*.json (skip README.md, test-keys/):
    for each case in file.cases:
        key = load_key(case.signer_key_ref)
        
        # Property 1: canonical bytes
        actual_canonical = my_sdk.canonical_bytes(case.input, exclude="signature")
        assert hex(actual_canonical) == case.expected.canonical_signable_bytes_hex
        
        # Property 2: ID
        actual_id = my_sdk.derive_id(case.input)
        assert actual_id == case.expected.expected_id
        
        # Property 3: reference signature verifies
        assert my_sdk.verify(
            pubkey = key.public_hex,
            data = actual_canonical,
            signature_hex = case.expected.reference_signature_hex
        )
        
        # Property 4: tampered signature rejects
        tampered = flip_bit(case.expected.reference_signature_hex, 5)
        assert not my_sdk.verify(key.public_hex, actual_canonical, tampered)
        
        # Property 5: independent sign-then-verify round-trip
        my_sig = my_sdk.sign(actual_canonical, key.private_hex)
        assert my_sdk.verify(key.public_hex, actual_canonical, my_sig)
```

## Versioning of this directory

When v1.0 freezes, this directory becomes immutable. Any
change to canonical form that breaks existing vectors is
a protocol-level breaking change and requires either:

- A new `docs/test-vectors/v1.1/` directory (additive
  capability, no existing vector breakage), OR
- A fork-block migration per QDP-0009 (actual breaking
  change).

The vector-schema version (`schema_version` field) may
increment to v1.1 even within a v1.0 protocol if additive
metadata is added (e.g., new optional fields in cases).

## Test keys

The test keys in `test-keys/` are deterministic keypairs
generated from fixed seeds. They are NEVER used in
production. Any signature produced by them is only valid
for test vectors.

| Name  | Seed | Quid ID | Notes |
|---|---|---|---|
| alice | `sha256("quidnug-test-key-alice-v1")` | computed | Used as primary signer |
| bob   | `sha256("quidnug-test-key-bob-v1")` | computed | Used as counterparty |
| carol | `sha256("quidnug-test-key-carol-v1")` | computed | Used as third party |

Keys are checked in as PKCS8 DER hex (matching the
`pkg/client` on-disk format). Regeneration of the keys
changes every downstream vector.

## References

- `docs/protocol-v1.0.md` §2.2 (canonical byte form)
- `docs/protocol-v1.0.md` §2.3 (signature algorithm)
- `docs/protocol-v1.0.md` §2.4 (transaction ID derivation)
- `docs/protocol-v1.0.md` §13 (test vector appendix spec)
- `internal/core/validation.go` (authoritative verify logic)
- `internal/core/reviews_integration_test.go` (authoritative
  sign-then-verify pattern)
- `cmd/quidnug-test-vectors/main.go` (the generator)
- `pkg/client/vectors_test.go` (Go SDK consumer)
