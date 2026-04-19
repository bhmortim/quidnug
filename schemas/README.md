# Quidnug Schemas

Language-agnostic schemas for Quidnug wire types. Every client
library in [`../clients/`](../clients/) and every integration in
[`../integrations/`](../integrations/) should reference these
schemas rather than hand-roll their own.

## Layout

```
schemas/
├── json/                   JSON Schema (draft 2020-12) for every
│                           HTTP request / response body
├── types/                  Canonical type reference per protocol
│                           surface — pure data, no encoding
└── README.md               this file
```

## Source of truth ordering

1. Go reference node (`internal/core/`) — ultimate source of truth.
2. `docs/openapi.yaml` — authoritative REST surface.
3. `schemas/` (this folder) — language-agnostic restatement.
4. Client SDKs — type-generated or hand-written mirrors.

When a protocol change lands, the order to propagate is
(1) → (2) → (3) → (4). Do not skip 3; language-specific
hand-translation produces drift.

## Versioning

Schemas are labeled with the QDP that introduced them:

- `json/txn_trust.schema.json` — pre-Phase-H
- `json/txn_identity.schema.json` — pre-Phase-H
- `json/txn_title.schema.json` — pre-Phase-H
- `json/event.schema.json` — pre-Phase-H
- `json/anchor.schema.json` — QDP-0001
- `json/guardian_set.schema.json` — QDP-0002
- `json/guardian_recovery.schema.json` — QDP-0002
- `json/guardian_resign.schema.json` — QDP-0006
- `json/anchor_gossip.schema.json` — QDP-0003
- `json/push_envelope.schema.json` — QDP-0005
- `json/snapshot.schema.json` — QDP-0008
- `json/fork_block.schema.json` — QDP-0009
- `json/merkle_proof.schema.json` — QDP-0010

When a new QDP adds a wire type, the schema goes here first
and the PR description links to the QDP.

## Why hand-authored, not generated

OpenAPI is expressive but not lossless for signature-bearing
types. The schemas here document **signable-byte canonicalization
rules** — which fields are excluded when computing the signable
bytes — that OpenAPI can't express natively. See
`schemas/types/canonicalization.md`.
