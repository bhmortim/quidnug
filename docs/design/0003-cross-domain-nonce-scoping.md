# QDP-0003: Cross-Domain Nonce Scoping

| Field         | Value                                       |
|---------------|---------------------------------------------|
| Status        | Draft                                       |
| Track         | Protocol (hard fork bundled with QDP-0001)  |
| Author        | The Quidnug Authors                         |
| Created       | 2026-04-18                                  |
| Supersedes    | —                                           |
| Requires      | QDP-0001 (Global Nonce Ledger)              |
| Implements in | v2.0 (target; co-ships with QDP-0001)       |

## 1. Summary

[QDP-0001](0001-global-nonce-ledger.md) proposed a **global** per-signer
nonce counter: one monotonic counter per `(signer, keyEpoch)`, shared
across all trust domains the signer operates in. That proposal flagged
the choice as "simple but possibly wrong" (§15.5) and deferred the
decision. This document makes the decision: **nonces are scoped per
`(signer, trustDomain, keyEpoch)`**, not global.

The central insight is that cross-domain **replay** is already
prevented by the existing signature scheme, because `TrustDomain` is
part of every transaction's signable data
([src/core/validation.go:110](../../src/core/validation.go)). A
transaction signed for `domain-A` cannot be submitted as a valid
transaction in `domain-B`, because the signature would not verify — the
bytes being signed differ. Nonces exist to prevent replay *within* a
domain's sequence; having them global buys no security and imposes
real costs.

Per-domain scoping, in contrast, delivers:

- **Isolation.** Each domain's validator set operates independently on
  nonce state, so a high-throughput domain doesn't contend with a
  low-throughput one.
- **Reduced memory for specialized nodes.** A node that participates
  only in `dns.example.com` doesn't need to track the nonce state of
  `healthcare.example.com` or any other domain.
- **Independent block cadences.** A domain producing one block every
  10 seconds and one producing blocks every hour can evolve their nonce
  state on entirely different timelines without coordination.
- **Cleaner audit and reasoning.** "Has signer `Q` done anything
  unusual in domain `D`?" is answered by looking at `D` alone.

The one subtlety — **key rotation is still global** — is addressed in
§6.4. Rotating a quid's key rotates it in every domain that quid
participates in, because a quid's identity and therefore its key is an
identity-level property, not a domain-level property.

## 2. Background

### 2.1 How `TrustDomain` scopes signatures today

Every transaction type embeds a `TrustDomain` string. Signature
verification in the current code is:

```go
txCopy := tx
txCopy.Signature = ""
signableData, _ := json.Marshal(txCopy)
if !VerifySignature(tx.PublicKey, signableData, tx.Signature) {
    return false
}
```

`TrustDomain` is inside `signableData`. Therefore the same signer
producing two transactions — identical except for `TrustDomain` —
produces two completely different signatures over two completely
different byte sequences. A signature captured from `TrustDomain =
"a.example"` fails verification when re-submitted as a `TrustDomain =
"b.example"` transaction.

This property is **pre-existing**, not something this QDP introduces.
It is the foundation of the argument for per-domain nonces.

### 2.2 What QDP-0001 initially proposed

QDP-0001 §6.1.2 proposed `NonceKey := (Quid, Epoch)`. At §15.5 it
discussed but did not resolve whether to scope by domain as well. This
document resolves it by changing the key to `(Quid, Domain, Epoch)`.

### 2.3 Why this changes now

Between the drafting of QDP-0001 and this document, two facts became
decisive:

1. The signature-scoping observation (§2.1) removes the main security
   argument *for* global nonces.
2. Early performance profiling on a test network showed that global
   per-signer map contention was meaningful when a handful of busy
   signers (institutional quids) transacted heavily across many
   domains: the single-writer lock held during nonce update became a
   serialization point even when the underlying domains were
   independent.

## 3. Problems with global nonces

### 3.1 Unnecessary contention (MEDIUM)

A global counter means every transaction from a heavy signer touches
the same map row regardless of which domain the transaction belongs
to. Under QDP-0001 §8.1's shard-by-`hash(quid)` layout, all operations
on that signer's counter hit the same shard, and within that shard, the
same counter. This is fine for low-activity signers and lethal for
signers active in many domains concurrently.

### 3.2 Bloated state on specialized nodes (MEDIUM)

A specialized node — say, a validator for only `dns.example.com` —
doesn't care about any other domain's nonce state. Under global
scoping, the node must still track every transacting signer in the
network, because any signer might enter `dns.example.com` at any time
and their global nonce is relevant. This bloats memory proportional to
the whole network, not to the domain served.

### 3.3 Coupled block cadences (LOW–MEDIUM)

When Domain A seals a block that advances signer `Q`'s nonce, Domain B
(where `Q` also operates) must observe that advance to remain
consistent. This forces cross-domain gossip of nonce checkpoints even
when the two domains are otherwise logically independent. It is not a
correctness problem — gossip catches up — but it means a slow or
offline Domain A briefly stalls `Q`'s activity in Domain B.

### 3.4 Audit and debugging complexity (LOW)

Answering "what has `Q` done in `D` recently?" requires either
querying `D` (fast, correct) or querying the global ledger and filtering
by domain (slower, same answer). Under global scoping, clients are
tempted to use the global ledger because it's "one source of truth,"
then pay the cost of filtering. Per-domain makes the natural path the
efficient path.

### 3.5 Blast radius of operational errors (MEDIUM)

A buggy validator that advances signer `Q`'s nonce incorrectly in
Domain A (because of a software bug, not a security issue) corrupts
`Q`'s counter network-wide under global scoping. Under per-domain
scoping, the bug's effect is confined to Domain A, and `Q` can
continue transacting elsewhere while Domain A is being fixed.

## 4. Goals and non-goals

### 4.1 Goals

- **G1.** Each domain maintains an independent per-signer nonce
  counter, unaware of and unaffected by other domains' counters.
- **G2.** Security properties of QDP-0001 (fresh-join replay
  resistance, partition-tolerance, anchor-based recovery) hold
  independently in each domain.
- **G3.** Key rotation and invalidation remain identity-level
  (global) concerns — a single anchor re-keys the quid in every
  domain.
- **G4.** Memory use on a specialized node scales with the set of
  domains the node participates in, not with the total network.
- **G5.** Zero additional signature-verification cost relative to
  global nonces.
- **G6.** Migration from pre-v2.0 state is straightforward and fits
  inside QDP-0001's one-shot migration window.

### 4.2 Non-goals

- **N1.** Per-domain *keys* (i.e., distinct key material per
  domain). That is a separate, larger design; see §11.2.
- **N2.** Hiding cross-domain signer identity. A signer active in
  multiple domains is publicly linkable across them. Operators who
  need unlinkability should use distinct quids per domain — which is
  already supported.
- **N3.** Domain-specific key epochs. Key-epoch advances apply
  globally. See §6.4 for why.
- **N4.** Hierarchical nonces for subdomains. See §7 for why.

## 5. Threat model

Inherited from QDP-0001 §5, with these additions:

| Capability                                                     | In scope? |
|----------------------------------------------------------------|-----------|
| Adversary operates in one domain and attempts to influence another | Yes, prevented |
| Adversary replays a Domain-A tx as a Domain-B tx               | Yes, prevented by signature scoping (§2.1) |
| Adversary replays a Domain-A tx into Domain A at a later time  | Yes, prevented by per-domain nonce |
| Adversary forks state in Domain A                              | Yes, handled per QDP-0001 §6.4 |
| Adversary forks state globally                                 | Yes, handled per-domain; no cross-domain consensus to attack |

The key addition: per-domain scoping removes the need for a global
consensus object for nonces, simplifying the threat surface.

## 6. Design

### 6.1 Data model change

Replace QDP-0001's `NonceKey`:

```go
// Before (QDP-0001 §6.1.2)
type NonceKey struct {
    Quid  string
    Epoch uint32
}

// After (this QDP)
type NonceKey struct {
    Quid   string
    Domain string   // new
    Epoch  uint32
}
```

The `NonceLedger` data structure is otherwise unchanged: two maps
(`accepted` and `tentative`), keyed by the new `NonceKey`. Anchor-
related maps (`currentEpoch`, last-anchor-nonce) remain keyed by
`(Quid, ...)` without domain — see §6.4.

### 6.2 Validation algorithm

Only one line of QDP-0001 §6.2 changes:

```
key = (tx.SignerQuid, tx.TrustDomain, tx.KeyEpoch)   // was (tx.SignerQuid, tx.KeyEpoch)
```

Every other validation step — strict monotonicity, tentative
reservation, max-gap check — is identical and retains the same
semantics.

### 6.3 Block-header checkpoints

Block checkpoints in QDP-0001 §6.1.3 gain a domain field. Note that
each block already belongs to exactly one `TrustDomain` (via
`TrustProof.TrustDomain`), so the domain is redundant to name
explicitly but good discipline:

```go
type NonceCheckpoint struct {
    Quid     string
    Domain   string   // = the block's TrustProof.TrustDomain
    Epoch    uint32
    MaxNonce int64
}
```

Validators MUST check `Checkpoint.Domain == Block.TrustProof.TrustDomain`
for every entry in `NonceCheckpoints` to prevent a misbehaving producer
from sneaking in cross-domain checkpoints.

### 6.4 Global key epochs

Key epochs remain global. Anchors (`AnchorRotation`,
`AnchorInvalidation`, `AnchorEpochCap` from QDP-0001, plus the
guardian anchors from QDP-0002) are **identity-level** events and
propagate to every domain the quid operates in.

The mechanism:

1. An anchor is submitted to one domain — typically the signer's
   "home" domain, but any domain works. The anchor is ordinary
   transaction traffic from the perspective of that domain's
   validators.
2. Once the anchor is in a Trusted block in that domain, a dedicated
   **anchor gossip** message propagates it to every other domain the
   signer has ever transacted in. The gossip payload is the original
   signed anchor plus a proof-of-inclusion Merkle path back to the
   Trusted block.
3. Each recipient domain, on receiving a gossiped anchor with a valid
   inclusion proof from a peer domain, records the new `currentEpoch`
   for the signer in its local ledger. Subsequent transactions in the
   recipient domain must use the new epoch.

This introduces a coordination requirement across domains — the one
thing per-domain scoping is trying to avoid. Mitigated by:

- Inclusion proofs are self-verifying: a domain doesn't need to *trust*
  the originating domain's validators to accept the anchor, only to
  verify the inclusion proof against the originating domain's chain
  hash (which the signer's quid includes in the gossip). See §7.3 for
  how domains learn each other's chain hashes.
- Anchor gossip is rare (humans rotate keys infrequently). It does
  not scale with transaction volume.
- A signer who wants the strongest isolation can voluntarily forego
  cross-domain gossip: transacting in multiple domains with
  deliberately not-rotated keys and absorbing the compromise blast
  radius if any.

### 6.5 Per-domain tentative and accepted state

The tier rules from QDP-0001 §6.4 apply per-domain. A block's
tentative acceptance in Domain A advances Domain A's tentative map for
the block's contributors; it does not affect Domain B's view. This
is the natural consequence of per-domain ledgers and is a feature: a
long-partitioned Domain A cannot stall Domain B's nonce advancement.

### 6.6 Per-domain snapshots

QDP-0001 §7 already proposed per-domain snapshots (the `TrustDomain`
field was present in `NonceSnapshot`). This QDP reifies that choice:
snapshots are authoritative within a single domain, produced and
consumed by nodes that participate in that domain.

A specialized node joining Domain `D` requests Domain `D`'s latest
snapshot; it does not need any other domain's snapshot. Memory and
bandwidth costs scale with `O(active signers in D)` regardless of
network size.

## 7. Subdomains and domain topology

### 7.1 The question

Quidnug supports wildcard domain patterns
([config.example.yaml](../../config.example.yaml)) like
`*.example.com`. Does a transaction in `sub.example.com` advance any
nonce in `example.com`?

### 7.2 The answer: no

Each fully qualified domain name is its own nonce scope. A transaction
in `sub.example.com` advances the signer's `(Q, "sub.example.com", E)`
counter and no other. The parent domain `example.com` has its own
counter and is unaffected.

This matches the existing `MatchDomainPattern` semantics ([config.go:420-440](../../src/core/config.go)),
which treats wildcards as *configuration-time* matchers for which
domains a node *accepts*, not as transitive scoping for state.

### 7.3 Cross-domain state exchange (for anchor gossip)

Per §6.4, anchors must propagate across domains. Each domain publishes
a periodic `DomainFingerprint` message on the existing gossip channel
([domain gossip, node.go:runDomainGossip](../../src/core/node.go)):

```go
type DomainFingerprint struct {
    Domain         string
    LatestBlock    int64
    LatestBlockHash string
    Timestamp      int64
    SignerQuid     string
    Signature      string
}
```

Consumers (other domains' validators) use `DomainFingerprint` entries
to verify inclusion proofs on anchor gossip: "this anchor is claimed to
be in block `H` of domain `D`, whose chain hash is `X` — I can verify
the Merkle path against `X`."

This adds modest traffic to the existing gossip layer. `DomainGossip`
messages today carry domain availability ([types.go:216-224](../../src/core/types.go));
`DomainFingerprint` is a peer gossip type with the same propagation
model.

## 8. Storage and performance

### 8.1 Per-domain memory

For a signer active in `K` domains, `accepted` has `K` entries for that
signer (one per domain). For a network with `S` signers and an average
of `D` domains per signer, total entries are `S × D`. Under global
scoping, it would be `S`. So per-domain scoping costs `D×` more memory
in aggregate.

This looks bad until you notice: the entries are partitioned across
domains. A node hosting `k` of the `total_domains` sees at most
`S × k / total_domains × D` entries. At the typical case of
`k << total_domains`, the per-node memory is **less** than global
scoping because the node no longer has to track all `S` global
signers; it only tracks signers present in its `k` domains.

Rough numbers with illustrative ratios:

| Signers | Avg domains/signer | Per-node footprint (global) | Per-node footprint (this QDP, node hosts 2% of domains) |
|---------|---------------------|------------------------------|---------------------------------------------------------|
| 1M      | 3                   | 64 MB                        | ~4 MB                                                   |
| 10M     | 5                   | 640 MB                       | ~64 MB                                                  |

Heavy signers (those active in many domains) are the exception — their
per-node footprint under this QDP is `D × 64 B` where `D` is the
number of domains the heavy signer operates in that the *local node*
hosts. For institutional signers active in hundreds of domains, this
is still a few kilobytes per signer.

### 8.2 Validation cost

Per-transaction validation cost is identical to QDP-0001 (two map
lookups + comparison). The key includes a string domain, which pushes
the hash a few bytes longer; this is negligible.

### 8.3 Anchor gossip cost

Cross-domain anchor gossip is the one new cost. An anchor is a few
hundred bytes; it propagates to domains where the signer has state.
For a typical quid active in 3 domains, a single rotation produces 3
anchor messages total across the network — trivial.

## 9. Wire formats

Changes from QDP-0001:

- `NonceKey` gains `Domain` field (in-memory only; not serialized
  independently).
- `NonceCheckpoint` gains `Domain` field (serialized; see §6.3).
- `NonceSnapshot.Entries[].Domain` is no longer needed — already
  scoped by the outer `NonceSnapshot.TrustDomain` — but we keep the
  field for explicitness and future-proofing.

New message type:

```json
{
  "kind": "DomainFingerprint",
  "domain": "example.com",
  "latestBlock": 12345,
  "latestBlockHash": "...",
  "timestamp": 1760000000,
  "signerQuid": "...",
  "signature": "..."
}
```

New HTTP endpoints:

- `GET /api/v2/domains/{domain}/fingerprint` — latest fingerprint
  produced by this node.
- `GET /api/v2/anchors/{signerQuid}/gossip?fromBlock=N` — anchor-
  gossip stream the node has observed from peer domains, useful for a
  fresh-joining node to catch up on pending global epoch changes.

## 10. Migration

Bundled with QDP-0001's v2.0 migration (§10.2 of that document). No
separate migration step is required; the one-shot ledger construction
simply groups pre-fork transactions by `(signer, domain)` instead of
by signer alone:

```
For each pre-H block B:
    for each tx in B.Transactions:
        key = (tx.SignerQuid, B.TrustProof.TrustDomain, 0)   // epoch 0
        ledger.accepted[key] = max(ledger.accepted[key], tx.Nonce)
```

(Plus the identity-update and event-sequence max-folds described in
QDP-0001 §10.2.1, equivalently scoped by domain.)

Migration produces a per-domain ledger; nodes that only participate in
a subset of domains can discard the rest.

## 11. Alternatives considered

### 11.1 Global nonces (the QDP-0001 default)

Already evaluated in this document. The signature-scope observation
(§2.1) removes the main security argument for globality, and the
contention and memory costs make it worse for specialized nodes.
Rejected in favor of per-domain.

### 11.2 Per-domain keys (full isolation)

Each `(signer, domain)` pair has its own public key. Compromise in one
domain is surgically isolated.

- **Pro.** Strongest possible isolation.
- **Con.** A quid is no longer a single identity; it becomes a table
  of keys. Every client must track which key is valid in which
  context. Cross-domain trust relationships become ambiguous.
- **Con.** Signing ergonomics collapse: the user must select the
  right key per transaction, mediated by tooling that knows the
  domain → key mapping.
- **Con.** Key rotation becomes a per-domain event; an anchor in
  Domain A has no effect on Domain B.

Rejected. A user wanting this level of isolation can already achieve
it by creating multiple quids — one per domain — which is a clean
pattern that composes well with trust relationships (the multiple
quids can trust each other with high weight).

### 11.3 Hybrid: primary key + per-domain operational keys

Each signer has a root key (rarely used, cold-stored) that signs a set
of domain-scoped operational keys. Transactions in Domain A are signed
by Operational-Key-A, which was attested by the root.

- **Pro.** Cleanest separation of duties.
- **Pro.** Matches conventional PKI (CA + intermediates) and modern
  wallet architecture (hardware root + software operational keys).
- **Con.** Introduces a two-tier key management discipline for
  every quid. Most users want one key.
- **Con.** Doubles protocol surface: every signature-related
  operation now supports both root and operational keys with
  different rules.

Not rejected — **deferred to a future QDP-0004** ("Operational Keys
and Attestations"). The operational-keys design is large enough to
need its own document and should build on both QDP-0001 and this one.

### 11.4 Global nonce with explicit domain field inside

A hybrid where nonces are global but each entry remembers which domain
most recently advanced it. Intended as an audit-friendly global
counter.

- **Con.** Doesn't reduce contention (§3.1) and doesn't reduce memory
  on specialized nodes (§3.2). It only restores a property per-domain
  scoping already has (per-domain audit).
- Rejected.

## 12. Security analysis

### 12.1 Invariants

- **I1.** For every `(signer, domain, epoch)`, accepted nonces are
  strictly monotonic.
- **I2.** A cross-domain replay is impossible because the signature
  domain field is part of signable data (pre-existing).
- **I3.** Anchor gossip propagates within a bounded time (proposal:
  `K` gossip rounds, each of `T` seconds; for typical `K=4, T=10s`
  that's 40 seconds worst case) such that a rotated key becomes invalid
  in every domain within the propagation bound.
- **I4.** No domain's nonce state depends on another domain's
  liveness. A partitioned or offline domain does not stall any other.

### 12.2 New attack considerations

**Anchor-gossip suppression.** An attacker who can prevent anchor
gossip from reaching Domain B can delay Domain B's recognition of a
key rotation. Meanwhile, in Domain B, the attacker's (old, compromised)
key is still accepted. Mitigations:

- Anchor gossip is redundant: multiple peers in Domain B will receive
  and relay the anchor. The attacker must suppress *all* such peers.
- Each `AnchorRotation` carries a `ValidFrom` timestamp; Domain B can
  refuse to accept any transaction signed by the old key *after* a
  `ValidFrom` it has observed via any channel, even before the anchor
  itself is Trusted in Domain B.
- Operators of security-critical quids should monitor anchor gossip
  latency; a lag above a threshold is an alerting signal.

**Domain-isolated replay.** An attacker captures a transaction in
Domain A, fails to replay it in A (nonce too low), and attempts the
replay in B. Prevented because the signature's `TrustDomain` field
wouldn't match B. This is the core reason per-domain scoping is safe.

**Domain-squatting by attacker.** An attacker spins up a malicious
Domain X in which the attacker controls the validator set, then
accepts replayed transactions from the victim signer in X. This works
against the attacker only — the victim signer's legitimate domains
are unaffected. Reducing this attack's impact further requires
application-level care: verifiers should check not just that a
transaction is valid, but that it came from a domain with an expected
validator set.

## 13. Open questions

1. **Default domain for anchors.** When a user rotates a key, to
   which domain do they submit the anchor first? Proposal: the
   "home domain" as declared in the quid's identity record; default to
   the first domain the quid ever transacted in.

2. **Anchor gossip trust.** Today's domain gossip (`types.go`
   `DomainGossip`) is advisory. Anchor gossip carries consequences
   (key rotation). Should it require inclusion-proof verification in
   all cases, or can a peer short-circuit with a signed attestation?
   Proposal: always inclusion-proof verified; no short-circuit, because
   the attestation layer would become a new trusted-party problem.

3. **Domain fingerprint retention.** How long does a node keep old
   `DomainFingerprint` records? Proposal: `min(14 days, since last
   anchor for any signer)`. Anchors older than the fingerprint window
   may fail to verify; fallback is full block sync from the peer
   domain.

4. **Lazy epoch propagation.** A signer who transacts in Domain B only
   quarterly may not propagate epoch changes to B in time. Should
   Domain B's validators proactively query the signer's home domain
   on first transaction after a long gap? Proposal: yes; add a
   `RecencyCheck` step in validation that rejects transactions from
   signers whose epoch is older than the home-domain-reported epoch.

5. **Mixed migration.** What if one domain upgrades to v2.0 but an
   adjacent domain is still on v1? Anchor propagation between them is
   not defined. Proposal: v1 domains are opaque; anchors do not
   propagate into them and the signer accepts that their old key
   remains valid in laggard domains until those domains upgrade.

## 14. Test plan

### 14.1 Unit tests

- `NonceKey` round-trip with domain; collision with same
  `(quid, epoch)` but different domain is not a collision.
- `NonceCheckpoint.Domain` mismatch vs. block's `TrustProof.TrustDomain`
  is rejected.
- Migration function (§10) on a synthetic multi-domain pre-fork
  history: per-domain ledgers match an independently-computed reference.

### 14.2 Integration tests

- **Cross-domain replay.** Signer `Q` publishes a TrustTransaction in
  Domain A with nonce 5. Attempt to re-submit the exact same bytes
  in Domain B. Verify rejection at the signature-verification step.
- **Independent advancement.** `Q` publishes transactions in Domain A
  (nonces 1..10) and Domain B (nonces 1..3) concurrently. Verify
  both domains' ledgers advance independently; a future Domain-A
  transaction at nonce 11 is accepted and Domain-B's counter is
  unaffected.
- **Anchor gossip.** `Q` rotates key in Domain A. Verify that Domain
  B, with gossip enabled, transitions to the new epoch within the
  propagation bound. Verify that Domain B rejects old-epoch
  transactions after the transition.
- **Gossip suppression.** Drop all anchor-gossip traffic to Domain B
  for 30 seconds. Verify Domain B accepts old-epoch transactions
  during the outage, then rejects them after gossip resumes.
  (Document this behavior rather than trying to prevent it — it's
  the inherent CAP trade-off.)

### 14.3 Performance benchmarks

- `BenchmarkPerDomainLedgerWrite` vs. `BenchmarkGlobalLedgerWrite`
  under 8-way concurrent writers each targeting a distinct domain.
  Per-domain should scale roughly linearly with writer count; global
  should saturate on lock contention.
- `BenchmarkNodeMemory` for a specialized node hosting 1-of-N domains.
  Verify `O(signers in hosted domains)` memory, not `O(total
  signers)`.

## 15. Rollout

Co-ships with QDP-0001 at v2.0. No separate rollout window; the two
QDPs are a single hard-fork package.

Observability additions:

- `quidnug_nonce_ledger_entries{domain="..."}` — per-domain gauge.
- `quidnug_anchor_gossip_latency_seconds{from_domain, to_domain}` —
  histogram.
- `quidnug_anchor_gossip_suppressed_total` — counter; alerts on
  sustained non-zero.
- `quidnug_domain_fingerprint_stale_total` — counter of inclusion-proof
  verifications that failed because the peer fingerprint was too old.

## 16. References

- [QDP-0001: Global Nonce Ledger](0001-global-nonce-ledger.md) (§15.5
  in particular)
- [QDP-0002: Guardian-Based Recovery](0002-guardian-based-recovery.md)
- [src/core/validation.go](../../src/core/validation.go) — current
  per-transaction validation (including signature-scope property)
- [src/core/config.go:420-440](../../src/core/config.go) —
  `MatchDomainPattern`
- [src/core/node.go](../../src/core/node.go) — existing domain gossip
  infrastructure (`runDomainGossip`)
- [docs/architecture.md](../architecture.md) — trust domain model

---

**Review status.** Draft. Required sign-off before merge: (a) §6.4
anchor gossip mechanism (cross-domain coordination is the highest-risk
part of this design), (b) §7.2 subdomain semantics (do we really want
`sub.example.com` and `example.com` to be fully independent?), (c)
§12.2 anchor-gossip suppression acceptable-risk framing.
