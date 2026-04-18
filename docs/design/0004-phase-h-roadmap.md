# QDP-0004: Phase H Roadmap — Residual Protocol Work

| Field         | Value                                      |
|---------------|--------------------------------------------|
| Status        | Draft                                      |
| Track         | Protocol + infrastructure                  |
| Author        | The Quidnug Authors                        |
| Created       | 2026-04-18                                 |
| Supersedes    | —                                          |
| Requires      | QDP-0001, QDP-0002, QDP-0003 (all landed)  |
| Implements in | Staged across v2.3 – v2.6                  |

## 1. Summary

QDP-0001 / -0002 / -0003 landed their foundations. Six items were
deliberately deferred because each has enough surface area to warrant
careful design, none fits cleanly as an incremental patch, and all
are independent enough that parallel work is feasible.

This document is the roadmap for those six items. It is **not** a
full design for each — it frames the problem, sketches the design
direction, calls out the hard constraints and rejection paths, and
proposes concrete sub-phases H1–H6. Each sub-phase will get its own
dedicated design doc (QDP-0005 through QDP-0010) before implementation.

The six items:

| Sub-phase | Title                              | Depends on   | Target   |
|-----------|------------------------------------|--------------|----------|
| H1        | Push-based gossip                  | QDP-0003     | v2.3     |
| H2        | Compact Merkle proofs              | QDP-0003     | v2.4 (*) |
| H3        | Snapshot K-of-K bootstrap protocol | QDP-0001 §7  | v2.3     |
| H4        | Lazy epoch propagation             | QDP-0003     | v2.3     |
| H5        | Fork-block migration trigger       | QDP-0001 §10 | v2.5 (†) |
| H6        | Guardian-consent revocation        | QDP-0002     | v2.4     |

(*) Hard fork — block structure changes. Schedule carefully.
(†) One-shot deployment, not ongoing protocol. Timed with the
QDP-0001 v1→v2 hard fork window.

## 2. Cross-cutting constraints

All six items must honor the same ground rules the earlier work
established:

- **No on-wire ambiguity.** Every new message carries a schema
  version. Signable canonical forms are deterministic across JSON
  round-trips (see the QDP-0003 canonicalization fix for why this
  matters).
- **Dedup-first validation.** Any idempotent message uses
  message-ID dedup before signature verification.
- **Anchor monotonicity preserved.** Any new anchor-like message
  strictly advances `lastAnchorNonce[signer]` so replay protection
  holds across all message types.
- **No silent protocol changes.** Hard forks get an explicit
  transition block and a shadow-mode observation period (QDP-0001
  Phase-0 pattern).
- **Test methodology documented.** Each sub-phase ships unit
  tests, integration tests, and — where applicable — adversarial
  rejection-path tests, with file-header doc comments explaining
  the invariants being guarded.

## 3. Sub-phases

### 3.1 H1 — Push-based gossip

**Problem.** QDP-0003 fingerprint / anchor-gossip is currently
pull-based: peers query `GET /api/v2/domain-fingerprints/{domain}/latest`
and operators submit via `POST /api/v2/anchor-gossip`. This is
simple and adequate for small deployments, but it doesn't scale and
can't deliver a rotation to a peer that isn't polling. Push-based
gossip lets a producing node fan out updates over the existing
domain-gossip channel in seconds instead of minutes.

**Design direction.**

- Extend the existing `DomainGossip` message-passing infrastructure
  (`runDomainGossip` already fans out per-domain availability
  messages) with two new message types:
  - `fingerprintGossip` — carries a signed `DomainFingerprint`,
    propagates to any node that subscribes to the domain.
  - `anchorGossip` — carries a signed `AnchorGossipMessage`,
    propagates to any node that has accepted at least one
    transaction from the signer (so the gossip reaches every
    domain where the signer has state).
- Propagation uses the existing TTL + seen-message dedup in
  `GossipSeen` — no new deduplication structure required.
- Subscription signals: when a node first seeds a signer's key
  into its ledger (e.g., via an identity transaction in a block
  it accepts), it becomes a "subscriber" for that signer's
  anchor gossip. Periodic sweep removes signers whose state has
  aged out.

**Rejection paths.**

- A gossip with unknown producer is dropped, not forwarded (avoids
  the network helping an attacker spread forged messages).
- A gossip whose origin-block fingerprint doesn't self-verify is
  dropped.
- Fan-out is rate-limited per (domain, signer) so a compromised
  validator can't flood.

**Success criteria.** A rotation sealed in domain A reaches every
node with state about the signer within `O(domain_gossip_interval
× propagation_hops)` — for the default 2 min interval and a
three-hop network, that's under 10 minutes. Measured by a
multi-node integration test with simulated propagation latency.

**Deferred within H1.**

- QUIC-based push (requires a transport overhaul). Current design
  stays on the existing HTTP+TTL gossip.
- Subscription-by-predicate ("all rotations with weight ≥ X"): a
  scaling optimization but not needed for the immediate problem.

---

### 3.2 H2 — Compact Merkle proofs

**Problem.** `AnchorGossipMessage` currently ships the entire
origin block. Anchors are rare, so the bandwidth cost is
acceptable, but the block-contents shape means receivers must also
hold the full block in memory to verify. Compact inclusion proofs
reduce both bandwidth and memory-pin cost and — more importantly —
give us a basis for light-client verification in Phase I.

**Design direction.**

- Add `TransactionsRoot string` to `Block`. Computed at seal time
  as the root of a Merkle tree over the canonical transaction
  bytes. SHA-256.
- Change `calculateBlockHash` to include `TransactionsRoot`
  rather than the full transaction list. Stage-1 canonicalization
  (round-trip-through-map) is preserved for the rest of the block.
- Extend `AnchorGossipMessage` with an optional `MerkleProof
  []string` field. When populated, the receiver verifies the
  anchor-at-index against `OriginBlock.TransactionsRoot` via the
  proof path and ignores the rest of `OriginBlock.Transactions`.
- Wire format stays backward-compatible during a shadow period:
  producers emit both the full-block and Merkle-proof forms;
  receivers prefer the proof when present.

**Hard-fork considerations.** Adding `TransactionsRoot` changes
the signable bytes of every new block. Scheduled as the v2.4
hard-fork at block `H_v2_4`. Nodes on v2.3 cannot verify v2.4
blocks; coordinated upgrade required.

**Rejection paths.**

- A proof path that doesn't verify against the claimed
  TransactionsRoot is rejected.
- A proof whose length exceeds `ceil(log2(MaxTxsPerBlock))` is
  rejected (prevents amplification).

**Success criteria.** Average `AnchorGossipMessage` size drops by
~70% for typical block-transaction counts. Round-trip integration
test on the v2.4 fork confirms rotations propagate across domains
using only proofs.

**Open questions.**

- Binary Merkle vs sparse Merkle? Binary is simpler, sparse
  enables efficient absence proofs. Leaning binary for H2;
  revisit for future tx-absence proofs.
- Leaf canonicalization: we already have the gossip
  canonicalization lesson. Each leaf is `canonicalizeTx(tx)` —
  map-round-trip-then-marshal. Tested against the same property
  test as the block hash.

---

### 3.3 H3 — Snapshot K-of-K bootstrap protocol

**Problem.** QDP-0001 §7 specified snapshots but left the
consumer-side bootstrap protocol as a deferred item. A fresh node
joining the network currently has no authoritative way to seed
its nonce ledger from a consistent snapshot — it must either start
blank (replay entire chain) or trust a single peer blindly.

**Design direction.**

- Define a `BootstrapSession` client state:
  1. Client queries `GET /api/v2/nonce-snapshots/{domain}/latest`
     from at least `K` peers (target K=3).
  2. If all K returned snapshots agree on `BlockHash` and
     `Timestamp` within a tolerance window, client accepts the
     consensus and seeds its ledger.
  3. If any peer disagrees, client retries with a larger peer
     set (fall back to 2K, then 3K) and escalates to a warning
     log entry. An operator override is required to proceed with
     a disputed bootstrap.
- Peer discovery: reuse the existing `KnownNodes` + `SeedNodes`
  mechanism. A `/peers` endpoint exposes the node's current
  known-peer set for bootstrapping clients.
- Shadow mode: new nodes that have bootstrapped run in
  shadow-verify mode for their first `N` blocks, recording any
  divergence between snapshot-seeded state and block-replayed
  state. Used to catch snapshot bugs before they bite.

**Rejection paths.**

- K-of-K quorum not met → don't bootstrap; require explicit
  operator action (empty-ledger start or trusted-peer override).
- Snapshot schema version unknown → fallback to block replay.
- Any snapshot signature invalid → exclude that peer from the
  quorum count.

**Success criteria.** A fresh node bootstrapping from 3 peers
against a domain with 10k signers seeds its ledger in under 2
seconds and correctly rejects all pre-snapshot-height replayed
transactions.

**Deferred.**

- Byzantine fault tolerance under M malicious peers of N — the
  K-of-K model assumes the operator chose reasonable seed peers.
  A PBFT-style consensus for snapshots is a v3 concern.

---

### 3.4 H4 — Lazy epoch propagation

**Problem.** QDP-0003 §15.4: a signer who transacts in domain B
only quarterly may have rotated their key in domain A months ago,
and domain B's local ledger still records the old epoch because
push-gossip never reached it. First post-rotation transaction
should not silently use the new key against the stale old-epoch
state.

**Design direction.**

- Add a `RecencyCheck` step to transaction admission: when a
  transaction from signer `Q` arrives in domain B and
  `CurrentEpoch(Q)` in domain B is older than `t_recency` (proposal:
  7 days), the node fires off a fingerprint-pull to the signer's
  declared home domain asynchronously. The transaction is
  admitted into a "quarantine" pending pool until the recency
  probe completes.
- "Home domain" is declared in the signer's identity record. If
  unset, the node defaults to its own domain set.
- Probe outcome:
  - Home reports a higher epoch → apply the rotation, let the
    quarantined transaction through if it uses the updated epoch.
  - Home reports the same epoch → release from quarantine
    unchanged.
  - Probe timeout → operator-configurable: either admit with a
    warning or reject.
- Quarantine aging: transactions sitting in quarantine longer
  than `t_quarantine` (proposal: 1 hour) are dropped with a
  metric + log.

**Rejection paths.** Signer's home-domain fingerprint is not
obtainable (down, missing, mutually distrusted) → falls back to
operator policy.

**Success criteria.** A test signer rotates in domain A, is
silent for 8 days, then submits a transaction in domain B. Node
quarantines, probes home, learns of the rotation, applies it, and
admits the transaction — all within the quarantine window.
Without the probe, the transaction would silently commit under the
stale epoch and bypass the rotation's intended compromise
recovery.

**Deferred.**

- Pre-warming of hot-signer state (proactive epoch fetches for
  signers expected to transact soon). Optimization only.

---

### 3.5 H5 — Fork-block migration trigger

**Problem.** QDP-0001 §10 described a shadow → enforce rollout:
`EnableNonceLedger` is off by default; operators turn it on
manually. For a production network, the flag flip must be
coordinated across all nodes at a specific block height so
consensus is preserved. Without a trigger mechanism, there's no
way to say "at block H_v2_5, every node starts enforcing."

**Design direction.**

- Add a new `ForkBlockTransaction` type with `TxTypeForkBlock =
  "FORK_BLOCK"`. Carries:
  - `ForkHeight int64` — the block height at which the fork
    takes effect.
  - `ForkBehavior []string` — a list of feature flags to flip
    (e.g. `"enable_nonce_ledger"`, `"require_tx_tree_root"`).
  - Signed by a quorum of validators for the domain (or by a
    configured "governance set" of quids — leaning validator
    quorum).
- When a node processes a Trusted block containing a
  `ForkBlockTransaction`, it schedules the feature flip for
  `ForkHeight`. At that block height, the ledger's
  `enforcementMode[behavior]` transitions from shadow to enforce.
- Pre-fork blocks are validated under the old rules. Post-fork
  blocks (index > ForkHeight) are validated under the new rules.
- A second `ForkBlockTransaction` with the same feature but
  different ForkHeight supersedes the first only if it arrives
  before the earlier ForkHeight.

**Rejection paths.**

- Fork tx signed by fewer than quorum validators → rejected.
- ForkHeight in the past or within `t_min_notice` of current
  height (proposal: 24h of blocks) → rejected as too soon for
  coordination.
- Conflicting ForkBlockTransactions (same ForkHeight, different
  ForkBehavior) → reject the second as ambiguous.

**Success criteria.** A simulated multi-node network agrees on a
future fork height via a single `ForkBlockTransaction`, all
nodes transition behavior simultaneously at that height, no node
forks.

**Hard considerations.**

- What if only a minority of validators propose the fork? Per
  Proof-of-Trust the minority can seal its own tentative chain;
  the majority ignores. Fork tx validation requires
  validator-quorum SIGNATURES, not just validator-quorum
  agreement — a single validator with enough combined trust can
  still unilaterally seal its minority fork, but only honest
  nodes that trust the signers will honor it.

---

### 3.6 H6 — Guardian-consent revocation mechanics

**Problem.** QDP-0002 §12.1 called out guardian revocation as
unresolved: how does a guardian who has consented to be in
subject `S`'s set withdraw that consent later? Reasons range from
the mundane (guardian stepped back from their role) to the urgent
(guardian's key was compromised and they don't want to be
reachable for recovery any more).

**Design direction.**

- New anchor kind: `AnchorGuardianResign`.
  - Signed by the resigning guardian (`GuardianQuid`).
  - Reference: `SubjectQuid`, `GuardianSetHash` (hash of the set
    they're resigning from — prevents replay across sets).
  - `AnchorNonce` — strictly monotonic per-guardian across
    resignations (a guardian resigning from multiple subjects
    uses separate nonce streams keyed by subject).
  - `EffectiveAt int64` — Unix timestamp from which the
    resignation is effective; must be `>=` now but operators can
    set a delay to give the subject time to reshape their set.
- Effect on the subject's `GuardianSet`:
  - From `EffectiveAt`, the resigning guardian's signature no
    longer counts toward the threshold.
  - The subject's set is otherwise untouched — threshold doesn't
    auto-adjust, so if M=3 and a guardian resigns from an N=5
    set, the subject is now operating at effective M-of-N-1. The
    subject must publish a `GuardianSetUpdate` to recover.
  - If the resignation drops effective weight below the
    threshold, the set is considered "weakened" — a metric
    fires but operation continues. Recovery via the normal path
    still works, just with higher coordination cost.
- HTTP endpoint: `POST /api/v2/guardian/resign`.
- Subject notification: when a resignation is accepted into a
  Trusted block that affects the subject, the subject's node
  (if observing) emits a `guardian_resignation` log + metric.

**Rejection paths.**

- Resignation signed by a non-member of the referenced set →
  rejected.
- `EffectiveAt` in the past or far in the future → rejected.
- Duplicate resignation (same guardian, same subject, already
  resigned) → dedup, return 200.

**Success criteria.** Guardian resigns during a pending recovery
— does the resignation affect the pending recovery's quorum?
Proposal: no, the resignation takes effect going forward; it does
not retroactively invalidate in-flight signatures. Test covers
both the happy path and this "mid-flight" case.

**Deferred.**

- Forced removal of a guardian by the subject without consent
  (e.g., the subject doesn't trust the guardian any more but the
  guardian won't resign). This is a separate mechanism — effectively
  a `GuardianSetUpdate` with the old guardian removed — and the
  authorization model already handles it (primary + current-threshold
  signatures, with the resigning guardian possibly declining to
  sign). Documented in QDP-0002 §6.4.4; no new QDP needed.

## 4. Dependency and sequencing

```
                    [H1 push gossip]
                         │
                         ▼
    [H4 lazy epoch propagation] ◀──── QDP-0003
                         │
                         ▼
    [H3 K-of-K bootstrap]  ◀─── QDP-0001
                         │
                         ▼
                    [H5 fork trigger]
                         │
                         ▼
              [H2 compact Merkle proofs]  (hard fork)
                         ▲
                         │
             [H6 guardian resignation] ◀─── QDP-0002
```

H2 depends on the fork-trigger because it's the cleanest way to
coordinate the `TransactionsRoot` wire change. H1/H3/H4/H6 can
proceed independently; any can ship first.

## 5. Scope cap

This document is a **roadmap**, not a design doc per item. Before
implementation, each sub-phase gets its own numbered QDP document
with the detail QDP-0001/0002/0003 provided (background, problems,
goals, threat model, data model, validation rules, migration, test
plan, alternatives, open questions).

| Sub-phase | Dedicated design doc |
|-----------|----------------------|
| H1        | QDP-0005 (to be written) |
| H2        | QDP-0006 |
| H3        | QDP-0007 |
| H4        | QDP-0008 |
| H5        | QDP-0009 |
| H6        | QDP-0010 |

## 6. Execution strategy

- **No sub-phase starts implementation before its dedicated QDP
  exists.** Preserves the project's discipline around thinking-
  before-typing.
- **H-prefix branches for parallel work.** Each sub-phase is
  independent enough that multiple can be in flight, though
  integration tests will need to run with feature flags combined.
- **Shadow → enforce for every protocol change.** The QDP-0001
  Phase-0 pattern (warning-only for a window, then enforce)
  applies to every item here that changes validation.
- **Continuous coverage floor.** Overall coverage stays above
  70%; every new sub-phase contributes methodology-documented
  tests.

## 7. Open questions for this roadmap itself

1. **H2 fork timing vs. H5 prerequisite.** H5 (the trigger) is
   listed before H2 (which uses the trigger). Should H5 land and
   be proven in a shadow flip first, to reduce the blast radius
   of H2's hard fork?
2. **H3 Byzantine tolerance.** K-of-K is simple but brittle. Is
   the v2.3 target acceptable, or should we wait for a BFT
   snapshot design?
3. **H4 quarantine vs. pre-warm.** The design proposes
   quarantine-on-demand. A pre-warm alternative (fetch home-domain
   fingerprints on schedule for known-home signers) may be less
   latency-sensitive. Revisit in QDP-0008.
4. **H6 interaction with RequireGuardianRotation.** If a subject
   has `RequireGuardianRotation=true` and a guardian resigns,
   dropping below threshold, should the subject's existing
   rotation-capable state be affected? Proposal: no — the
   flag stays on; the subject's threshold is weakened but
   guardian-only-rotation remains the rule, operationally
   worse but safer. Revisit in QDP-0010.

## 8. References

- [QDP-0001: Global Nonce Ledger](0001-global-nonce-ledger.md)
- [QDP-0002: Guardian-Based Recovery](0002-guardian-based-recovery.md)
- [QDP-0003: Cross-Domain Nonce Scoping](0003-cross-domain-nonce-scoping.md)
- [CHANGELOG.md](../../CHANGELOG.md) — rolling record of what has
  already landed vs. what's pending.

---

**Review status.** Draft. Sign-off required from maintainers on
the sequencing in §4 (particularly the H5-before-H2 ordering) and
the H1-subscription semantics in §3.1 before the first dedicated
QDP is written.
