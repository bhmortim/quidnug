# Quidnug client API parity matrix

The Python SDK (`clients/python/quidnug/client.py`) is the reference
implementation: every node-level JSON endpoint defined in
`internal/core/handlers*.go` has a one-to-one Python method. This
file is the canonical "what each client covers" tracker.

Widget / framework / plugin clients (`react`, `react-reviews`,
`vue-reviews`, `astro-reviews`, `web-components`, `reviews-widget`,
`wordpress-plugin`, `shopify-app`, `browser-extension`, `iso20022`)
are deliberately narrower — they ship UI or platform glue on top of
a core SDK, and have their own scope-specific READMEs. Only the
six "core SDK" clients are tracked below.

Legend: ✓ implemented · – not implemented

## Node-level reads

| Method (Python name) | Py | JS | Java | .NET | Rust | Swift |
| --- | --- | --- | --- | --- | --- | --- |
| `health` | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ |
| `info` | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ |
| `nodes` | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ |
| `get_blocks` | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ |
| `get_tentative_blocks` | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ |
| `get_pending_transactions` | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ |
| `list_domains` | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ |
| `get_node_domains` | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ |
| `update_node_domains` | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ |

## Identity

| Method | Py | JS | Java | .NET | Rust | Swift |
| --- | --- | --- | --- | --- | --- | --- |
| `register_identity` | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ |
| `get_identity` | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ |
| `query_identity_registry` | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ |

## Trust

| Method | Py | JS | Java | .NET | Rust | Swift |
| --- | --- | --- | --- | --- | --- | --- |
| `grant_trust` | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ |
| `get_trust` | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ |
| `query_relational_trust` | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ |
| `get_trust_edges` | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ |
| `query_trust_registry` | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ |

## Title

| Method | Py | JS | Java | .NET | Rust | Swift |
| --- | --- | --- | --- | --- | --- | --- |
| `register_title` | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ |
| `get_title` | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ |
| `query_title_registry` | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ |

## Events / streams

| Method | Py | JS | Java | .NET | Rust | Swift |
| --- | --- | --- | --- | --- | --- | --- |
| `emit_event` | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ |
| `get_event_stream` | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ |
| `get_stream_events` | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ |

## IPFS

| Method | Py | JS | Java | .NET | Rust | Swift |
| --- | --- | --- | --- | --- | --- | --- |
| `ipfs_pin` | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ |
| `ipfs_get` | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ |

## Guardians (QDP-0002 / 0006)

| Method | Py | JS | Java | .NET | Rust | Swift |
| --- | --- | --- | --- | --- | --- | --- |
| `submit_guardian_set_update` | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ |
| `submit_recovery_init` | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ |
| `submit_recovery_veto` | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ |
| `submit_recovery_commit` | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ |
| `submit_guardian_resignation` | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ |
| `get_guardian_set` | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ |
| `get_pending_recovery` | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ |
| `get_guardian_resignations` | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ |

## Cross-domain gossip (QDP-0003 / 0005)

| Method | Py | JS | Java | .NET | Rust | Swift |
| --- | --- | --- | --- | --- | --- | --- |
| `submit_domain_fingerprint` | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ |
| `get_latest_domain_fingerprint` | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ |
| `submit_anchor_gossip` | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ |
| `push_anchor` | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ |
| `push_fingerprint` | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ |

## Bootstrap (QDP-0008)

| Method | Py | JS | Java | .NET | Rust | Swift |
| --- | --- | --- | --- | --- | --- | --- |
| `submit_nonce_snapshot` | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ |
| `get_latest_nonce_snapshot` | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ |
| `bootstrap_status` | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ |

## Fork-block (QDP-0009)

| Method | Py | JS | Java | .NET | Rust | Swift |
| --- | --- | --- | --- | --- | --- | --- |
| `submit_fork_block` | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ |
| `fork_block_status` | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ |

## Domain registration

| Method | Py | JS | Java | .NET | Rust | Swift |
| --- | --- | --- | --- | --- | --- | --- |
| `register_domain` | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ |
| `ensure_domain` | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ |
| `query_domain` | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ |

## Commit-wait helpers (client-side polling)

| Method | Py | JS | Java | .NET | Rust | Swift |
| --- | --- | --- | --- | --- | --- | --- |
| `wait_for_identity` | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ |
| `wait_for_identities` | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ |
| `wait_for_title` | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ |

## Static / offline helpers

| Helper | Py | JS | Java | .NET | Rust | Swift |
| --- | --- | --- | --- | --- | --- | --- |
| Canonical bytes | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ |
| Merkle proof verification (QDP-0010) | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ |
| Quid key generation / import | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ |

## Naming conventions

| Language | Style | Example |
| --- | --- | --- |
| Python | `snake_case` | `client.register_identity(signer, ...)` |
| JS / TS | `camelCase` | `client.createIdentityTransaction(params, quid)` |
| Java | `camelCase`, params via builder objects | `client.registerIdentity(signer, p)` |
| .NET / C# | `PascalCaseAsync` | `await client.RegisterIdentityAsync(...)` |
| Rust | `snake_case`, async + `Result<T, Error>` | `client.register_identity(&signer, ...).await?` |
| Swift | `camelCase`, async actor | `try await client.registerIdentity(signer: q, ...)` |

The bytes signed by every SDK are byte-identical (see
`schemas/types/canonicalization.md`), so a transaction signed by any
SDK is verifiable by any other.

## Endpoint paths

Every core SDK targets the same HTTP paths, matching the Go
reference at `pkg/client/client.go` and the OpenAPI spec at
`docs/openapi.yaml`. Notable canonical paths (relative to `/api/`):

- Identity: `transactions/identity`, `identity/{quid}`,
  `registry/identity`
- Trust: `transactions/trust`, `trust/{observer}/{target}`,
  `trust/query`, `trust/edges/{quid}`, `registry/trust`
- Title: `transactions/title`, `title/{asset}`, `registry/title`
- Events: `events`, `streams/{subject}`, `streams/{subject}/events`
- IPFS: `ipfs/pin`, `ipfs/{cid}`
- Guardians: `guardian/set-update`, `guardian/recovery/{init,veto,commit}`,
  `guardian/resign`, `guardian/set/{quid}`,
  `guardian/pending-recovery/{quid}`, `guardian/resignations/{quid}`
- Gossip: `domain-fingerprints`, `domain-fingerprints/{domain}/latest`,
  `anchor-gossip`, `gossip/push-anchor`, `gossip/push-fingerprint`
- Bootstrap: `nonce-snapshots`, `nonce-snapshots/{domain}/latest`,
  `bootstrap/status`
- Fork-block: `fork-block`, `fork-block/status`
- Domains: `domains`, `domains/{name}/query`, `node/domains`
- Blocks / mempool: `blocks`, `blocks/tentative/{domain}`,
  `transactions`

## Coverage summary

| Client | Public methods | Notes |
| --- | --- | --- |
| Python | 48 | reference implementation |
| JS (v1 + v2) | 45 | TypeScript `.d.ts` shipped |
| Java | 45 | builder objects for IdentityParams / TrustParams / TitleParams / EventParams |
| .NET | 47 | async + `CancellationToken` everywhere; nullable refs for 404→null |
| Rust | 49 | typed wire structs in `wire.rs`; envelope parser tolerates `edges` / `data` shapes |
| Swift | 49 | actor; built on Foundation `URLSession`; ephemeral test URLProtocol |

(Counts differ slightly across languages — e.g. Java exposes
`getIdentity(quidId)` and `getIdentity(quidId, domain)` as two
overloads; Python exposes one method with a keyword arg. The
underlying HTTP surface is identical.)

## Not covered here

- **Widget / framework clients** (`react`, `react-reviews`,
  `vue-reviews`, `astro-reviews`, `web-components`, `reviews-widget`)
  delegate API calls to a core SDK (typically JS). They expose
  domain-specific UI primitives, not API methods, and have their
  own READMEs.
- **Platform plugins** (`wordpress-plugin`, `shopify-app`) embed the
  JS SDK; their READMEs cover the plugin's own surface (shortcodes,
  app blocks).
- **Browser extension** (`browser-extension`) is a signing wallet
  that exposes `window.quidnug.{listQuids,sign,...}` — a deliberately
  small API for DApps, not the full node API.
- **Android** (`android`) is a Kotlin coroutines wrapper over the
  Java SDK plus Android-specific signers and storage; the API
  surface is whatever the Java SDK exposes.
- **ISO 20022** (`iso20022`) is a pointer to the Go integration in
  `integrations/iso20022/`. Companion SDKs for ISO 20022 wrap that
  Go package via the same HTTP/JSON event surface.
