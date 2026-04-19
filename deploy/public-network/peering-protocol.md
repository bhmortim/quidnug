# Public network peering protocol

> A lightweight bilateral-trust convention layered on top of standard
> Quidnug `TRUST` transactions — letting any operator join the public
> network without any change to the protocol.

## Overview

The public Quidnug network is a set of cooperating nodes that mutually
trust each other to produce and validate blocks in specific domains. It
is **not** a permissionless chain. It is seeded by the nodes operated
under the `quidnug.com` administrative control, and grows by
**tit-for-tat mutual trust edges** signed bilaterally by existing
operators and new joiners.

This document specifies:

1. The reserved domain hierarchy used for peering.
2. The signed peering-request format operators send to join.
3. The review and publish flow executed by the inviting operator.
4. The reciprocal publish the joiner must perform to complete peering.
5. The revocation path.

Nothing here is novel at the protocol layer — the mechanism is ordinary
domain-scoped `TRUST` transactions. The spec exists so operators agree
on naming and workflow.

## 1. Reserved domains

All peering-related trust edges are scoped under the reserved root
`network.quidnug.com`.

| Domain                                     | Semantic                                                              |
| ------------------------------------------ | --------------------------------------------------------------------- |
| `peering.network.quidnug.com`              | "I will gossip with you" — bare minimum: exchange blocks + rotations. |
| `validators.network.quidnug.com`           | "I will tier your blocks as Trusted in my chain."                     |
| `validators.<app-domain>.network.quidnug.com` | As above, scoped to an application domain.                          |
| `operators.network.quidnug.com`            | "I attest this identity corresponds to the named human/org operator." |
| `bootstrap.network.quidnug.com`            | "I will serve you as a K-of-K snapshot source" (QDP-0008).            |

These are conventions, not protocol. A node won't refuse a gossip
connection just because `peering.*` trust is missing — the node's own
tiering rules determine what it actually accepts. But the conventions
let operators answer the question "**can I tell whether this peer
belongs?**" by looking at what domains the root seeds vouch for it in.

### Domain inheritance

A trust edge in `validators.network.quidnug.com` **does not imply**
trust in specific app-domain validators like
`validators.oracles.ethereum.network.quidnug.com`. Operators opt into
each app-domain explicitly. This fits the protocol's deliberate anti-
composition stance on cross-domain trust.

## 2. Peering request

A joiner submits a **signed peering request** describing the edges
they'd like to receive (and are willing to reciprocate).

### Wire format

```json
{
  "type": "QUIDNUG_PEERING_REQUEST_V1",
  "requester": {
    "quidId":       "<16-hex-char subject id>",
    "publicKey":    "<hex-encoded ECDSA P-256 public key>",
    "operatorName": "Alice's Oracle LLC",
    "operatorUrl":  "https://alice-oracle.example",
    "nodeEndpoint": "https://node.alice-oracle.example:8080",
    "nodeVersion":  "v1.4.2",
    "contact":      "ops@alice-oracle.example"
  },
  "target": {
    "quidId":       "<seed node's quid to peer with>"
  },
  "domains": [
    "peering.network.quidnug.com",
    "validators.oracles.ethereum.mainnet.network.quidnug.com"
  ],
  "trustLevel":     0.85,
  "proposedExpiry": 1766150400,
  "nonce":          1,
  "timestamp":      1740096000,
  "signature":      "<base64 ECDSA signature over the canonical bytes>"
}
```

- **Canonical bytes**: the same byte-layout every SDK produces — remove
  the `signature` field, canonically serialize the rest, sign.
- **Nonce** is this signer's monotonic nonce in the standard sense. If
  a request is rejected, the next attempt must use `nonce + 1`.
- **`trustLevel`** is a proposal. The reviewer may publish a different
  level; the joiner is free to walk away or accept the revised edge.
- **`proposedExpiry`** is a Unix seconds value beyond which the joiner
  expects the edge to be refreshed. Omit for no expiry.
- **`domains`** must all be subtrees of `network.quidnug.com`.

### How to submit

Three acceptable channels:

1. **GitHub issue** on `quidnug/quidnug` with the label
   `peering-request`, using the issue template at
   [.github/ISSUE_TEMPLATE/peering_request.md](../../.github/ISSUE_TEMPLATE/peering_request.md)
   (the request JSON pasted in a fenced block). Public, auditable,
   simplest.
2. **HTTP POST** to `https://api.quidnug.com/v1/peering/requests` once
   the automation lands — returns a short-lived receipt UUID.
3. **Email** to `peering@quidnug.com` with the request attached. Used
   only if GitHub/HTTP is unavailable.

Channel (1) is canonical. (2) and (3) echo into (1) automatically.

## 3. Review and publish (seed operator)

The seed operator ("reviewer") performs the following checks:

1. **Signature verifies** against the claimed public key.
2. **`quidId` matches** `sha256(publicKey)[:16]`.
3. **`nodeEndpoint` resolves** and returns a valid
   `GET /api/info` advertising a compatible protocol version.
4. **No prior active trust** with a different key for this operator name
   (prevent impersonation of an existing peer).
5. **Domains are within policy** — for example, the root may refuse to
   vouch for new validators in `oracles.*` unless the requester has an
   existing attestation from a trusted oracle community.
6. **Rate limit** — one successful peering request per operator per 7
   days, unless explicitly waived.

On approval the reviewer posts a `TRUST` transaction from the seed
node's quid to the requester's quid, in each requested domain, at the
accepted trust level. These land on the seed node's chain immediately;
gossip propagates them to any peer that trusts the seed enough to
accept the blocks as Trusted.

On rejection the reviewer posts a public decision in the same GitHub
issue with a reason from a stable enum (see
[rejection-reasons.md](rejection-reasons.md)).

## 4. Reciprocation (joiner)

Within **72 hours** of approval, the joiner must publish reciprocal
`TRUST` transactions from their node's quid to the seed node's quid,
in **at least** the `peering.network.quidnug.com` domain, at any
non-zero level.

If reciprocation is not observed within 72 hours, the seed's edges
become eligible for revocation. (The seed operator is not obligated
to revoke — some operators will choose to sustain asymmetric trust —
but "ghost peering" is a signal of an operator who has lost their keys.)

## 5. Revocation

Either party can revoke by:

- Publishing a `TRUST` transaction in the same domain with
  `trustLevel: 0.0`, **or**
- Publishing an `AnchorInvalidation` against the epoch they used to
  sign the original peering edge.

A revocation is **prospective only**. Blocks already accepted remain in
the chain; new transactions signed after the revocation block will tier
as `Untrusted` from the revoker's viewpoint.

### Mass revocation

If the seed operator detects a widespread compromise of a peer (e.g. a
coordinated abuse campaign), they publish a revocation edge in
`validators.network.quidnug.com` and announce in the public `#network`
Slack / GitHub Discussions channel. Other operators decide independently
whether to follow the seed's revocation — by protocol design, the seed
does not have authority over anyone else's trust graph.

## 6. Ergonomic notes

- The protocol doesn't know about "the public network" as a concept —
  only about signed trust edges. If you want to belong to *a different
  public network*, fork these conventions under a different reserved
  root (`network.<yourdomain>`) and seed separately.
- Peering does not imply any service-level agreement. Operators may
  drop peers at any time, with or without notice.
- Running a node is not the same as peering. Anyone can run a node that
  simply consumes the public network's gossip; peering is only
  necessary to have *your own* blocks tiered as Trusted by peers.

## 7. Open questions

- **Automation threshold**: the current flow is user-in-the-loop at
  step 3. Partial automation (e.g. auto-approve low-stakes peering
  edges from requesters with an existing `operators.*` attestation)
  is a future QDP candidate.
- **Peering-request bundling**: should a single request allow multiple
  target seed nodes? Current spec: one target per request.
- **Gossip-health attestation**: should peers periodically publish
  signed "I successfully gossiped with you in the last hour"
  heartbeats? Fits `peering.*` semantics but doubles traffic.

These questions should mature into a QDP once the protocol has been
running in public for a few months and we have operator feedback.
