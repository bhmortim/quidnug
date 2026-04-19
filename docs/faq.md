# FAQ + troubleshooting

Common questions and error codes, ordered by frequency.

If your issue isn't here, open an issue at
https://github.com/bhmortim/quidnug/issues with:

- The SDK name + version (`python 2.0.0`, `@quidnug/client 2.0.0`, …).
- The node version (from `GET /api/info`).
- A minimal reproduction.

---

## "Signature doesn't verify"

**Symptom:** You sign a transaction locally and the node rejects it
with `INVALID_SIGNATURE`.

**Most likely cause:** Canonical bytes differ between what you
signed and what the node is verifying against. Common culprits:

1. **You forgot to exclude a field before signing.** The
   `signature` field itself must be stripped from the transaction
   before computing canonical bytes. Every SDK does this for you
   via methods like `client.grant_trust(...)` — but if you're
   hand-crafting a transaction dict and calling
   `canonical_bytes(...)` directly, pass
   `exclude_fields=("signature", "txId")` (and `"publicKey"` for
   EVENTs).

2. **Your serializer is emitting `\uXXXX` escapes for non-ASCII.**
   Go's `encoding/json` emits raw UTF-8 by default; Python's
   `json.dumps` does NOT (requires `ensure_ascii=False`); .NET's
   `System.Text.Json` escapes all non-ASCII; Java's Jackson
   `writeValueAsString` is fine. Every first-class SDK handles this
   correctly; if you've subclassed or monkey-patched, check that
   raw UTF-8 survives.

3. **You're verifying with a different key than you signed with.**
   The `publicKey` attached to the transaction must match the
   private key that produced the signature. Use `Quid.from_private_hex`
   to reconstruct; do not hand-craft key pairs.

**Check:** Run the cross-SDK interop harness:

```bash
cd tests/interop
make produce verify
```

If your SDK produces bytes that match the harness's reference
vectors, you're aligned with every other Quidnug SDK.

---

## `NONCE_REPLAY`

**Cause:** You submitted a transaction with a nonce ≤ the highest
nonce the node has already seen for this signer.

**Fix:**

- For TRUST / IDENTITY transactions: the nonce is monotonic per
  signer-domain pair. Read the current nonce from the registry
  before submitting, then use `last + 1`.
- For ANCHOR transactions: nonces are global per domain, not
  per-signer.

The SDKs default to nonce `1` for convenience, which works for a
fresh signer but will fail if the same signer has already
submitted in this domain. Fetch the latest explicitly for
long-lived signers.

---

## `QUORUM_NOT_MET`

**Cause:** A guardian-set update / recovery required `threshold`
signatures but you supplied fewer (or some signatures were invalid).

**Fix:**

1. Count the weighted signatures on your update — remember each
   `GuardianRef.weight` applies; a 3-weight guardian signs for 3
   units, not 1.
2. Verify each signature individually using the guardian's known
   public key. Common issue: a stale `key_epoch` after a guardian
   rotated their key — the signer must sign with the epoch's
   current public key.

---

## `GUARDIAN_SET_MISMATCH`

**Cause:** Your guardian-related submission referenced a hash that
doesn't match the currently-installed guardian set for the subject.

**Fix:** Always fetch the current set (`get_guardian_set(quid)`) and
compute the hash at submission time. Don't cache the hash.

---

## `FEATURE_NOT_ACTIVE`

**Cause:** You called an endpoint for a protocol feature that
hasn't yet fork-activated in your domain (QDP-0009).

**Fix:** Check `GET /api/fork-block/status` to see which features
are live. Propose a fork-block to activate it, or wait for your
domain's next activation schedule.

---

## `BOOTSTRAPPING` / HTTP 503

**Cause:** The node is still catching up with peers before it can
accept writes.

**Fix:** Poll `GET /api/bootstrap/status` until it reports ready.
For automated pipelines, treat 503 responses as retriable with
exponential backoff — every SDK does this by default.

---

## `NOT_FOUND`

**Cause:** The queried quid / title / asset / domain / fingerprint
doesn't exist on this node.

**Fix:** Double-check the ID format (16 hex chars for quid IDs).
For cross-domain queries, confirm the receiving node has fingerprints
from the source domain — it might just not have received the
gossip yet.

---

## "connection refused" / "connect: connection refused"

**Cause:** No node listening on the address your SDK was
configured with.

**Fix:** Check the node is up (`curl http://localhost:8080/api/health`).
If you're using Docker Compose, ensure `docker compose up -d`
completed. If the node is on a remote host, check firewall /
security-group rules.

---

## `VALIDATION` (client-side)

**Cause:** Your SDK caught a precondition failure before any
network call. Common patterns:

- `percentages must sum to 100` — your TitleParams `owners` list
  doesn't sum precisely to 100.0 (±0.001).
- `exactly one of Payload or PayloadCID is required` — EVENT
  transactions need an inline payload OR an IPFS CID, not both,
  not neither.
- `level must be in [0, 1]` — trust level is bounded; clamp before
  submitting.
- `signer must have a private key` — you passed a read-only Quid
  (from `Quid.from_public_hex(...)`) where a signing Quid was
  required.

---

## "How do I enable verbose / debug logging?"

### Python

```python
import logging
logging.basicConfig(level=logging.DEBUG)
```

Enables structlog output for the SDK.

### Go

```go
import "log/slog"
slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelDebug})))
```

The Go SDK logs via `log/slog` when present.

### JavaScript

```js
const client = new QuidnugClient({ defaultNode, debug: true });
```

### Rust

```bash
RUST_LOG=quidnug=debug cargo run
```

### .NET

Enable the `Quidnug.Client` logger in `appsettings.json`:

```json
{
  "Logging": { "LogLevel": { "Quidnug.Client": "Debug" } }
}
```

---

## "How do I verify a Quidnug signature outside of the SDKs?"

You need ECDSA P-256 + SHA-256 verification against the
canonical-bytes output. The OpenSSL one-liner:

```bash
# Extract public key to PEM
echo "<hex-pubkey>" | xxd -r -p | openssl ec -inform DER -pubin -pubout > pubkey.pem

# Hash the canonical bytes
printf "<canonical-bytes>" | openssl dgst -sha256 -binary > tx.digest

# Verify
echo "<hex-sig>" | xxd -r -p > sig.der
openssl pkeyutl -verify -pubin -inkey pubkey.pem -in tx.digest -sigfile sig.der
```

The canonical-bytes format is documented in
[`schemas/types/canonicalization.md`](../schemas/types/canonicalization.md).

---

## "How do I back up a node?"

Cold backup:

1. Stop the node (`systemctl stop quidnug` / `docker stop`).
2. `tar cf - /var/lib/quidnug | gzip > backup-$(date +%F).tar.gz`.
3. Restart.

Hot backup: not yet supported. Planned via snapshot LSM-style
replication in a future release.

---

## "How do I audit a guardian recovery flow?"

Every step of QDP-0002 recovery emits an EVENT on the subject's
stream:

```
GET /api/streams/<subjectQuid>/events?eventType=GUARDIAN_SET_UPDATE
```

and

```
GET /api/streams/<subjectQuid>/events?eventType=GUARDIAN_RECOVERY_INIT
GET /api/streams/<subjectQuid>/events?eventType=GUARDIAN_RECOVERY_VETO
GET /api/streams/<subjectQuid>/events?eventType=GUARDIAN_RECOVERY_COMMIT
```

Every event is signed; concatenate them into an audit report by
timestamp.

---

## "How do I rate-limit gossip push?"

`config.yaml`:

```yaml
gossipPush:
  maxPerSecond: 100   # hard ceiling across all peers
  burst: 200
```

The settings apply to incoming pushes; outgoing pushes are not
rate-limited (but every peer you're pushing to has their own).

---

## Getting help

- Run the cross-SDK interop harness: `tests/interop/`.
- File an issue: https://github.com/bhmortim/quidnug/issues.
- Follow the maintainers for design-level discussion: see
  `CODE_OF_CONDUCT.md` and `CONTRIBUTING.md`.

## License

Apache-2.0.
