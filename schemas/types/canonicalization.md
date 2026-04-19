# Signable-Bytes Canonicalization

Every signature in Quidnug is over the canonical bytes of a
specific subset of the transaction's fields. Clients MUST
compute signable bytes identically or signatures will not
verify across implementations.

## Universal rule

The signer's own signature field is **cleared** (set to empty
string / omitted) before computing signable bytes.

## Round-trip canonicalization

For any transaction containing typed-struct fields that may
traverse a JSON round-trip (gossip, IPFS), canonicalization is
**round-trip through a generic map**:

```
signable_bytes(tx) =
  JSON.stringify(
    JSON.parse(
      JSON.stringify(cloneWithSignatureCleared(tx))
    )
  )
```

This matches Go's `json.Marshal` → `json.Unmarshal` into
`interface{}` → `json.Marshal` pattern in
`internal/core/crypto.go:calculateBlockHash` and friends. The
reason: typed structs and maps produce different field
orderings after deserialization; rount-tripping normalizes to
the map-based ordering.

### UTF-8 encoding rule (**critical cross-language gotcha**)

The serialized JSON bytes MUST contain raw UTF-8 for all non-ASCII
characters — **not** `\uXXXX` JSON escapes.

- Go's `encoding/json` does this by default.
- Rust's `serde_json` does this by default.
- JavaScript's `JSON.stringify` does this by default.
- **Python's `json.dumps` does NOT** — you must pass
  `ensure_ascii=False`.
- **Java's Jackson does NOT when using its default string-writer path**
  — the SDK uses `writeValueAsString` which emits raw UTF-8 directly.
- **.NET's `System.Text.Json` does NOT** by default — but the SDK
  configures `JsonSerializerOptions` so it emits UTF-8.

The Quidnug SDKs for every language handle this correctly. If you
hand-roll a canonicalizer, the cross-SDK interop harness at
`tests/interop/` will catch any UTF-8 divergence in the first
transaction that contains non-ASCII input.

### Key sorting rule

The second serialization MUST alphabetize map keys at every depth.
Go's `encoding/json` alphabetizes `map[string]interface{}` keys by
default, which is why the round-trip-through-map pattern produces
sorted output "for free" in Go. Every other language must do this
explicitly — recursively sort keys before the second marshal.

## Per-type signable bytes

### TRUST transaction

All fields except `signature`. The signature field is cleared.

### IDENTITY transaction

All fields except `signature`.

### TITLE transaction

All fields except `signatures` map. The map is cleared; each
owner's signature is computed separately over the canonical
bytes with the map cleared.

### EVENT transaction

All fields except `signature`.

### ANCHOR (and all anchor variants)

All fields except `signature`. Nested structures (e.g.
`Guardians` in `AnchorGuardianSetUpdate`) are included fully.

### GUARDIAN_SET_UPDATE

Signable bytes clear three signature fields at once:
`primarySignature`, `newGuardianConsents`, `currentGuardianSigs`.
Each signer signs over the **same** canonical bytes (with all
three cleared). This is critical: nobody's signature alters
what the others are signing.

### GUARDIAN_RECOVERY_INIT

`guardianSigs` slice is cleared.

### GUARDIAN_RECOVERY_VETO

`primarySignature` AND `guardianSigs` both cleared. Veto can
be authorized by either path; the signable bytes are the same
regardless.

### GUARDIAN_RECOVERY_COMMIT

`committerSig` cleared.

### GUARDIAN_RESIGN

`signature` cleared.

### FORK_BLOCK

`signatures` slice cleared.

### ANCHOR_GOSSIP

`gossipSignature` cleared. **Sign over `OriginBlock.Hash`**, not
the full `OriginBlock` — see QDP-0003 §8.3 for why. This is
the only transaction that signs over a hash rather than the
full structure.

## Cross-language correctness

Every client SDK MUST include a test that:

1. Constructs a known-value transaction (hardcoded).
2. Computes signable bytes.
3. Compares byte-for-byte to the Go reference.

Test vectors live at `schemas/test_vectors/` (one JSON file per
transaction type, with expected hex of the signable bytes).

## Signature algorithm

- Curve: NIST P-256 (`secp256r1`)
- Hash: SHA-256
- Encoding: DER for interop; hex-encoded at the wire layer.
- No RFC 6979 deterministic nonces — standard ECDSA with
  cryptographic RNG is sufficient (we don't have a secret-
  derivation path that requires determinism).
