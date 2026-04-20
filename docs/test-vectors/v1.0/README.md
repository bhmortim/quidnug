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

### `internal/core` reference node — conformant

`internal/core/vectors_test.go` passes all 24 cases across 7
files. All conformance properties hold for every transaction
type using the reference node's own serialization +
signature-verification code paths.

### `pkg/client` Go SDK — conformant

`pkg/client/crypto.go` converged to IEEE-1363 signatures in
the same commit as the expanded vector harness. Submit paths
for TRUST, IDENTITY, TITLE, EVENT use typed mirror structs in
`pkg/client/types_wire.go` matching the server's field order
exactly. `pkg/client/vectors_test.go` exercises all 24 cases
via the SDK's public `(*Quid).Sign` / `(*Quid).Verify` /
`QuidFromPublicHex` entry points — all pass.

Two divergence-probe tests remain in place as regression
detectors:

- `TestPkgClientCanonicalBytesDivergesFromAuthoritative`:
  `CanonicalBytes()` retains alphabetical ordering for
  backward compat. It is no longer on the critical signing
  path for any v1.0 submission; the probe documents the
  retained legacy behavior.
- `TestPkgClientSignNowMatchesAuthoritative`: asserts that
  `(*Quid).Sign` produces valid 64-byte IEEE-1363
  signatures that verify via an independent authoritative
  verifier. Regression-detects any accidental rollback.

### `clients/python` Python vector consumer — conformant (via local helpers)

`clients/python/tests/test_vectors.py` passes all 24
conformance cases using local IEEE-1363 helpers that match
the authoritative form (see the `sign_ieee1363` /
`verify_ieee1363` functions in the test file).

### `clients/python/quidnug` Python SDK itself — still divergent

The SDK's public API (`quidnug.crypto.Quid.sign`,
`canonical_bytes`) still produces DER signatures and
alphabetical canonical bytes. Convergence of the Python SDK
itself is a pending launch blocker analogous to
`pkg/client`'s earlier state. The fix follows the same
pattern:

- Migrate `quidnug.crypto.Quid.sign` to return 64-byte
  IEEE-1363 (DER → raw r||s via `decode_dss_signature` then
  zero-padded concat, identical to the pattern already used
  in `examples/elections/clients/common/crypto.py`).
- Migrate `quidnug.crypto.Quid.verify` to accept the same
  format.
- Add typed mirror dataclasses per tx type, replacing
  map-based tx construction in `clients/python/quidnug/client.py`
  submit paths.

Two divergence probes in `test_vectors.py` document the SDK
gap:

- `test_clients_python_sdk_sign_diverges_from_authoritative`
- `test_clients_python_canonical_bytes_diverges_from_authoritative`

Both self-heal: they log "converged!" and pass without
assertion once the SDK migrates.

### `clients/js/` (JS/TS SDK) — consumer pending

Known to use a different signing library; cross-check
required before launch.

### `clients/rust/` (Rust SDK)

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
