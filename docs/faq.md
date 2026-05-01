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
- For ANCHOR transactions: nonces are global per domain, not per-signer.
  _(The `ANCHOR` transaction type does not appear in the current spec or SDK. This entry may reference a removed or renamed feature — human review needed before relying on this.)_

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

Everything that should survive a restart lives in `DATA_DIR` (default
`./data`, configured via `DATA_DIR` env / `data_dir:` YAML). Files:

- `node_key.json` — per-process ECDSA keypair. **Without this, NodeID
  changes on the next boot and any TRUST grants pointing at the old
  NodeID become orphaned.** This is the most important file in the
  directory.
- `blockchain.json` — block history snapshot.
- `trust_domains.json` — TrustDomains + DomainRegistry index.
- `pending_transactions.json` — pending tx queue.
- `peer_scores.json` — per-peer scoreboard with quarantine state and
  recent-event ring.

Cold backup:

```bash
systemctl stop quidnug
tar -C /var/lib/quidnug -czf backup-$(date +%F).tar.gz .
systemctl start quidnug
```

Hot backup: each file is written atomically (tmp + rename) so
copying `DATA_DIR/*.json` from a live node will give you a
point-in-time snapshot of each file individually. Inter-file
consistency isn't guaranteed — if you need a strictly-consistent
hot snapshot, take a filesystem-level snapshot (LVM, ZFS, EBS) or
stop the node briefly.

The `OPERATOR_QUID_FILE` (operator's long-lived signing identity)
is *not* in `DATA_DIR` by convention. Back it up separately to
somewhere that survives a host loss. **Losing it loses the
identity that accumulates trust across all your nodes.**

---

## "How do I configure my node's operator quid?"

The operator quid is your long-lived identity that accumulates trust
across every node you run. Generate once, deploy on every node:

```bash
# 1. Generate the operator quid (do this on a workstation,
#    not on a node — keeps the private key offline-capable).
quidnug-cli quid generate --out /etc/quidnug/operator.quid.json
chmod 600 /etc/quidnug/operator.quid.json

# 2. Deploy the file to every node you run. Same file, every node.
#    Each node still gets its own NodeID (from data_dir/node_key.json);
#    only the operator quid is shared.

# 3. Reference it from the node's config:
#    YAML:  operator_quid_file: /etc/quidnug/operator.quid.json
#    Env:   OPERATOR_QUID_FILE=/etc/quidnug/operator.quid.json

# 4. Confirm the node is running under it:
curl http://localhost:8080/api/v1/info | jq .data.operatorQuid
# { "id": "034bc467852ffa94", "publicKeyHex": "..." }
```

The landing page at `/` will show the operator quid in the "This node"
facts table when one is configured. Nodes without a configured
operator quid show "Ephemeral identity" with a link to this section.

---

## "How do I peer with another operator?"

Three peer sources feed the same admit pipeline:

1. **`seed_nodes:`** — bootstrap addresses. Every learned peer goes
   through admission (handshake + NodeAdvertisement lookup +
   operator-attestation TRUST check at weight ≥
   `peer_min_operator_trust`).
2. **`peers_file:`** — operator-managed YAML list, fsnotify-watched
   for live reload. Use this when you want to pin an operator quid
   or whitelist a LAN peer with `allow_private: true`. Edit:
   ```bash
   PEERS_FILE=/etc/quidnug/peers.yaml \
     quidnug-cli peer add 203.0.113.42:8080 --operator-quid Q
   ```
3. **`lan_discovery: true`** — mDNS / DNS-SD on `_quidnug._tcp.local.`.
   Opt-in. Useful for home/office/lab.

To check what your node sees:

```bash
quidnug-cli peer list                   # composite scores included
quidnug-cli peer show <nodeQuid>        # full per-peer breakdown
```

Or via API: `GET /api/v1/peers` (worst-first ordering — operators
want to see the bad ones first).

---

## "Why is my peer being quarantined / evicted?"

Phase 4 of the peering plan: every interaction with a peer
(handshake, gossip post, query, broadcast, validation outcome)
nudges that peer's composite score in `[0, 1]`. Defaults:

- `peer_quarantine_threshold: 0.4` — peers below this stay in
  `KnownNodes` but are excluded from routing.
- `peer_eviction_threshold: 0.2` — peers below this for
  `peer_eviction_grace` (5 min default) are dropped from
  `KnownNodes`. Static-source peers (from `peers_file`) are
  eviction-immune and just log a stern warning instead.
- `peer_fork_action: quarantine` — what happens when fork-claim
  detection fires. `log` records only, `quarantine` flips after 2+
  claims, `evict` is immediate (overrides static-immunity, since a
  fork claim is a Byzantine signal).

Inspect a quarantined peer:

```bash
quidnug-cli peer show <nodeQuid>
# Composite: 0.32
# Quarantined: yes — composite below quarantine threshold
# Per-class success/failure (decay-adjusted):
#   handshake:      4.1 / 0.0
#   validation:     5.0 / 12.3
#   ...
# Severe events:
#   signature fails: 1
# Recent events (newest last):
#   2026-05-01T...  validation FAIL  anchor: signature
#   ...
```

Fix paths: investigate why validation fails (peer's gossip
producer signed something that doesn't verify against their
on-file key — is their key file corrupt? clock skew?), or if
the peer is genuinely Byzantine, leave it quarantined.

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
