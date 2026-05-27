# Quidnug Python SDK

The official Python client for [Quidnug](https://github.com/bhmortim/quidnug), a
decentralized protocol for relational, per-observer trust.

Version 2.x of this SDK covers the full protocol surface: identity,
trust, titles, event streams, anchors, guardian sets, guardian
recovery, cross-domain gossip, K-of-K bootstrap, fork-block activation,
and compact Merkle inclusion proofs (QDPs 0001–0010).

## Install

```bash
pip install quidnug
```

Requires Python 3.9+. The package ships a `py.typed` marker for full
type-checker support under mypy / pyright.

## Thirty-second example

```python
from quidnug import Quid, QuidnugClient

client = QuidnugClient("http://localhost:8080")

alice = Quid.generate()
bob = Quid.generate()

client.register_identity(alice, name="Alice", home_domain="contractors.home")
client.register_identity(bob, name="Bob", home_domain="contractors.home")
client.grant_trust(alice, trustee=bob.id, level=0.9, domain="contractors.home")

tr = client.get_trust(alice.id, bob.id, domain="contractors.home")
print(f"{tr.trust_level:.3f} via {' → '.join(tr.path)}")
```

More runnable examples live in [`examples/`](./examples/):

| File | What it covers |
| --- | --- |
| `quickstart.py` | Two-party trust + relational trust query. |
| `event_stream.py` | Emit an event; read back the stream. |
| `guardian_recovery.py` | Install a guardian set with weighted quorum. |
| `merkle_proof.py` | Verify a compact QDP-0010 inclusion proof. |

## What's in the package

### `QuidnugClient`

HTTP client for a running Quidnug node. Every node endpoint has a
corresponding typed method.

| Area | Methods |
| --- | --- |
| Health / info | `health`, `info`, `nodes` |
| Identity | `register_identity`, `get_identity`, `query_identity_registry` |
| Trust | `grant_trust`, `get_trust`, `query_relational_trust`, `get_trust_edges`, `query_trust_registry` |
| Title | `register_title`, `get_title`, `query_title_registry` |
| Events | `emit_event`, `get_event_stream`, `get_stream_events` |
| Storage | `ipfs_pin`, `ipfs_get` |
| Guardians | `submit_guardian_set_update`, `submit_recovery_init`, `submit_recovery_veto`, `submit_recovery_commit`, `submit_guardian_resignation`, `get_guardian_set`, `get_pending_recovery`, `get_guardian_resignations` |
| Gossip | `submit_domain_fingerprint`, `get_latest_domain_fingerprint`, `submit_anchor_gossip`, `push_anchor`, `push_fingerprint` |
| Bootstrap | `submit_nonce_snapshot`, `get_latest_nonce_snapshot`, `bootstrap_status` |
| Fork-block | `submit_fork_block`, `fork_block_status` |
| Blocks | `get_blocks`, `get_tentative_blocks`, `get_pending_transactions` |
| Domains | `list_domains`, `register_domain`, `get_node_domains`, `update_node_domains` |

All guardian / gossip / bootstrap / fork-block methods route to
`/api/v2/<path>` on the node, where the v2-only handlers are mounted.
(Earlier `2.x` releases of this SDK routed to `/api/<path>`, which
returns 404 against a stock node.)

### v3 extensions (QDPs 0011, 0014, 0015, 0017, 0018, 0023)

The v3 surface adds protocol coverage for endpoints introduced after
the v2 release. The methods live on `QuidnugClient` (and
`AsyncQuidnugClient`) directly — no extra import is needed.

```python
# Operator audit log (QDP-0018)
head = client.get_audit_head()
page = client.get_audit_entries(since=head["sequence"] - 100, limit=100)

# Network discovery (QDP-0014)
info = client.get_discovery_domain("contractors.home")

# DNS attestation lookup (QDP-0023)
records = client.resolve_dns("example.org", "A")
```

| Area | Methods | Server path |
| --- | --- | --- |
| Peers (QDP-0011) | `get_peers`, `get_peer` | `/api/peers`, `/api/peers/{nodeQuid}` |
| Node advertisements | `submit_node_advertisement` | `/api/node-advertisements` |
| Domain registry | `get_top_domains`, `get_tentative_blocks`, `submit_domain_gossip` | `/api/domains/top`, `/api/blocks/tentative/{domain}`, `/api/gossip/domains` |
| Moderation (QDP-0015) | `submit_moderation_action`, `get_moderation_actions` | `/api/moderation/actions[...]` |
| Audit (QDP-0018) | `get_audit_head`, `get_audit_entries`, `get_audit_entry` | `/api/audit/*` |
| Privacy (QDP-0017) | `submit_dsr`, `get_dsr_status`, `grant_consent`, `withdraw_consent`, `get_consent_history`, `create_processing_restriction`, `get_processing_restrictions`, `submit_dsr_compliance` | `/api/privacy/*` |
| Discovery (QDP-0014) | `get_discovery_domain`, `get_discovery_node`, `get_discovery_operator`, `discovery_quids`, `discovery_trusted_quids` | `/api/v2/discovery/*` |
| DNS attestation (QDP-0023) | `submit_dns_claim`, `submit_dns_challenge`, `submit_dns_attestation`, `submit_dns_renewal`, `submit_dns_revocation`, `submit_dns_delegate`, `submit_dns_delegate_revocation`, `get_dns_attestations`, `get_dns_attestations_weighted`, `resolve_dns` | `/api/v2/dns/*` |

GET-by-id endpoints that can 404 (`get_peer`, `get_discovery_*`,
`get_audit_entry`, `get_dsr_status`) return `None` rather than raise,
matching the existing convention on `get_identity` / `get_title`.

### `Quid` — cryptographic identity

```python
alice = Quid.generate()                               # fresh keypair
bob = Quid.from_private_hex(stored_key_hex)           # reconstruct
carol = Quid.from_public_hex(pub_hex_from_network)    # read-only
sig = alice.sign(b"hello")
ok = alice.verify(b"hello", sig)
```

Every identity uses ECDSA P-256 with SHA-256, DER-encoded hex
signatures — matching the Go reference implementation byte-for-byte.

### `canonical_bytes` / `verify_signature`

Low-level primitives exposed for advanced use cases:

```python
from quidnug import canonical_bytes

signable = canonical_bytes(tx, exclude_fields=("signature", "txId"))
tx["signature"] = quid.sign(signable)
```

Canonicalization follows the **round-trip-through-a-generic-object**
rule: serialize → decode back to a plain map → serialize again with
alphabetized keys. This matches Go's `encoding/json` output for
`map[string]interface{}`, which is what the node verifies against. See
[`schemas/types/canonicalization.md`](../../schemas/types/canonicalization.md)
for the full specification.

### `verify_inclusion_proof` (QDP-0010)

```python
from quidnug import verify_inclusion_proof

ok = verify_inclusion_proof(
    tx_bytes=canonical_bytes(anchor_tx),
    proof_frames=gossip_msg.merkle_proof,
    expected_root=origin_block.transactions_root,
)
```

Reconstructs the Merkle root from leaf + sibling frames. Returns
`True` if the proof is correct and binds to the expected root.

### Wire dataclasses

Every protocol-level type is a plain Python dataclass in `quidnug.types`:
`TrustEdge`, `IdentityRecord`, `Title`, `OwnershipStake`, `Event`,
`Anchor`, `GuardianRef`, `GuardianSet`, `GuardianSetUpdate`,
`GuardianRecoveryInit/Veto/Commit`, `GuardianResignation`,
`DomainFingerprint`, `Block`, `AnchorGossipMessage`,
`MerkleProofFrame`, `NonceSnapshot`, `ForkBlock`, `TrustResult`.

All fields use snake_case; the client transparently rewrites to
camelCase on the wire.

## Error handling

```python
from quidnug import QuidnugClient, ConflictError, UnavailableError, NodeError

try:
    client.grant_trust(alice, trustee=bob.id, level=0.9, nonce=old_nonce)
except ConflictError as e:
    print(f"Node rejected: {e.message}")      # e.g. NONCE_REPLAY
except UnavailableError:
    print("Node still bootstrapping — retry later")
except NodeError as e:
    print(f"Transport failure: HTTP {e.status_code}")
```

| Exception | When |
| --- | --- |
| `ValidationError` | Local precondition failed before any network call. |
| `ConflictError` | Node logically rejected (nonce replay, quorum failure, …). |
| `UnavailableError` | HTTP 503 or feature-not-active. |
| `NodeError` | Transport, 5xx after retries, unexpected response shape. |
| `CryptoError` | Signature verify / key derivation failed. |

All inherit from `QuidnugError`, so a catch-all is safe.

## Retry policy

- GETs are retried up to `max_retries` times on 5xx and 429, with
  exponential backoff + ±100 ms jitter. Respects `Retry-After`.
- POSTs (writes) are **not** retried by default — repeat a write only
  once you've confirmed the server's view via a GET, to avoid double
  commits on non-idempotent transactions.

Tune with constructor arguments:

```python
client = QuidnugClient(
    "http://node.local",
    max_retries=5,
    retry_base_delay=0.5,
    timeout=60.0,
)
```

## Type hints

PEP 561–compliant. mypy and pyright will see the full type surface:

```bash
pip install mypy
mypy --strict your_module.py
```

## Running the tests

```bash
cd clients/python
pip install -e .[dev]
pytest -v
```

## Protocol version compatibility

| SDK version | Node version | QDPs |
| --- | --- | --- |
| 2.x | 2.x | 0001–0010 |
| 1.x | 1.x | identity, trust, title only |

v2 is **not** wire-compatible with v1. If you have a v1-era node,
upgrade the node first (all v1 data migrates automatically) and then
install SDK v2.

## Development

```bash
# Format + lint
pip install ruff
ruff format quidnug tests
ruff check quidnug tests

# Type-check
pip install mypy
mypy --strict quidnug
```

## License

Apache-2.0 — see [`LICENSE`](../../LICENSE) at the repo root.
