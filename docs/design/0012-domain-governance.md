# QDP-0012: Domain Governance

| Field      | Value                                                            |
|------------|------------------------------------------------------------------|
| Status     | Phase 1 landed — state extension (no behavior change); Phases 2-6 pending |
| Track      | Protocol                                                         |
| Author     | The Quidnug Authors                                              |
| Created    | 2026-04-20                                                       |
| Requires   | QDPs 0001 (nonce ledger), 0002 (guardian recovery), 0008 (k-of-k bootstrap), 0009 (fork-block activation) |
| Implements | Formal governance primitives for the validator set of a domain   |

## 1. Summary

The public network grows by other operators choosing to trust
our seed nodes for a domain. A node that publishes `TRUST`
edges toward our seed validators is **establishing a cache
replica** — a local mirror of the agreed chain for that domain,
produced by the consortium of validators they've chosen to
follow. That part already works today.

What's missing is the primitive that lets a cache node graduate
into a **participant in the consortium** — a validator whose
own blocks other cache nodes would accept. Today that gap has
to be closed by manual direct mutation of the node's
`TrustDomain.Validators` map, which means:

- Every new operator is either a permanent cache replica
  (stuck as read-only),
- Or an unauthorized validator whose self-declared status is
  honored on honor (no protocol-level gate).

QDP-0012 formalizes the gate. It names three roles already
latent in the code but not distinguished:

- **Cache replica** (formerly "follower") — a node that
  receives, verifies, and tiers blocks for a domain using its
  own trust graph, maintaining a local mirror of the agreed
  chain. Default role for every node in every domain it hasn't
  been explicitly admitted to.
- **Consortium member** (aka **validator**) — a node whose
  blocks the consortium agrees are admissible for this domain's
  chain. Membership is a per-domain property, granted via
  governance action, not self-declared. Multiple consortium
  members per domain produce the agreed chain collectively.
- **Governor** — a quid authorized to mutate a domain's
  consortium roster and parameters. A domain has an M-of-N
  governor quorum fixed at registration and mutable only by
  the same quorum.

Promotion from cache replica to consortium member is a signed,
on-chain event co-signed by the domain's current governor
quorum, then activated after a notice period. The existing
`TRUST` transaction remains the mechanism by which each
observer decides whose consortium they mirror; `DOMAIN_GOVERNANCE`
decides who *can* belong to a consortium at all.

The relativistic core is preserved: you are still choosing
which consortium's chain to mirror **from your perspective**.
QDP-0012 standardizes how a consortium's membership evolves
over time so newcomers can join the agreed-on process rather
than impersonating it.

This lets the seed operator provide a durable "first-mover
trust" service — maintaining a public set of domains, running
the initial consortium, and selectively delegating child-domain
governance to vetted third-party operators.

## 2. Goals and non-goals

**Goals:**

- A clean follower / validator / governor separation, enforced
  by the reference node.
- A signed, auditable, replayable primitive for mutating a
  domain's validator set.
- Time-locked changes with a notice period long enough for
  other operators to react.
- Support for delegating child-domain authority without
  handing over the parent.
- Backward compatibility via QDP-0009 fork activation.

**Non-goals:**

- A global universal validator registry. Quidnug is relativistic
  and each observer still chooses which chain they follow;
  governance is about **who the network allows** to produce
  blocks, not about forcing anyone to accept those blocks.
- Economic incentives / staking. That would be a new QDP.
- Arbitrary on-chain parameter governance beyond validator set
  and thresholds. Specific scalar parameters (block interval,
  max tx size, etc.) stay out of `DOMAIN_GOVERNANCE` for now —
  add them incrementally as needed.
- Governance for non-public domains. Private-domain operators
  are free to run this same scheme, but can also just manage
  their validator set out-of-band.

## 3. Concept model

### 3.1 Roles per domain

For any domain `D`, each quid `Q` has a role derived from the
domain's current state:

```
role(Q, D) =
    "governor"         if Q ∈ D.Governors
    "consortium-member" if Q ∈ D.Validators (with nonzero weight)
    "cache-replica"    otherwise   (if the node trusts this domain at all)
```

Roles are non-exclusive. A quid can be a governor without being
a consortium member (useful for human operators who vote on
policy but don't run infrastructure), and a consortium member
without being a governor (useful for operational nodes that
produce blocks but don't decide who else gets to).

Every node that holds a local mirror of a domain's chain is a
cache replica for that domain, regardless of whether they're
also a consortium member or governor. Cache-replica-ness is the
baseline — it's what you get by pointing your node at a domain
and trusting its current consortium.

### 3.2 Existing state bag, extended

Today `TrustDomain` carries:

```go
type TrustDomain struct {
    Name                string
    ValidatorNodes      []string           // legacy list form
    TrustThreshold      float64
    BlockchainHead      string
    Validators          map[string]float64 // quid -> weight
    ValidatorPublicKeys map[string]string  // quid -> hex pubkey
}
```

Extended to:

```go
type TrustDomain struct {
    Name                string
    ValidatorNodes      []string
    TrustThreshold      float64
    BlockchainHead      string
    Validators          map[string]float64
    ValidatorPublicKeys map[string]string

    // QDP-0012 additions — all omitempty for backward compat.
    Governors            map[string]float64 `json:"governors,omitempty"`
    GovernorPublicKeys   map[string]string  `json:"governorPublicKeys,omitempty"`
    GovernanceQuorum     float64            `json:"governanceQuorum,omitempty"` // fraction, e.g. 0.67
    GovernanceNonce      int64              `json:"governanceNonce,omitempty"`  // last applied
    ParentDelegationMode string             `json:"parentDelegationMode,omitempty"` // "inherit" | "self" | "delegated"
    DelegatedFrom        string             `json:"delegatedFrom,omitempty"`    // parent domain name if delegated
}
```

`Governors` is a map of quid to vote weight so a quorum decision
can be weighted (operator A carries 2 votes, auditor B carries
1 vote, etc.). `GovernanceQuorum` is the fraction of total vote
weight required for a change to activate — typically `2/3`.

### 3.3 Block-production gate

The existing `GenerateBlock(domain)` path silently assumes the
node belongs to `D.Validators`. Under QDP-0012 that's made
explicit. `GenerateBlock` returns an error if:

```
node.NodeQuidID ∉ D.Validators OR D.Validators[node.NodeQuidID] == 0
```

Cache-replica nodes simply skip domains they're not consortium
members of when iterating their managed-domain list. This is a
one-line change in `runBlockGeneration`. Cache replicas still
gossip transactions toward consortium members, still serve
read queries, still tier incoming blocks — they just don't
produce.

### 3.4 Block-acceptance gate

The existing `ValidateBlockCryptographic` already requires the
producing validator's public key to be in
`D.ValidatorPublicKeys`. That's preserved. The new piece is
making sure `Validators` / `ValidatorPublicKeys` can only be
mutated through the governance transaction flow, not via
side-door code paths.

## 4. The `DOMAIN_GOVERNANCE` transaction

### 4.1 Shape

```go
type DomainGovernanceTransaction struct {
    BaseTransaction

    // Which domain this mutation applies to. Must be exactly the
    // domain being changed; governors of D cannot mutate D's
    // children unless they're also governors of the child.
    TargetDomain string `json:"targetDomain"`

    // What the mutation does. Enum; see §4.2.
    Action GovernanceAction `json:"action"`

    // Action-specific subject (validator quid being added,
    // governor being removed, child-domain name, etc.). Fields
    // unused by a given Action are empty.
    Subject        string  `json:"subject,omitempty"`
    TargetWeight   float64 `json:"targetWeight,omitempty"`
    TargetThreshold float64 `json:"targetThreshold,omitempty"`
    ChildDomain    string  `json:"childDomain,omitempty"`

    // For UPDATE_GOVERNORS: the full proposed new governor set
    // (replacing the existing one). Atomic swap to prevent
    // partial states.
    ProposedGovernors          map[string]float64 `json:"proposedGovernors,omitempty"`
    ProposedGovernorPublicKeys map[string]string  `json:"proposedGovernorPublicKeys,omitempty"`
    ProposedGovernanceQuorum   float64            `json:"proposedGovernanceQuorum,omitempty"`

    // Strictly-monotonic per-domain. Blocks replay.
    Nonce int64 `json:"nonce"`

    // The block-height at which this change activates. Must be
    // at least MinGovernanceNoticeBlocks in the future at tx
    // acceptance time.
    EffectiveHeight int64 `json:"effectiveHeight"`

    // The human-readable governor message committed on-chain.
    // Optional, capped at 1024 chars. Useful for audit trails.
    Memo string `json:"memo,omitempty"`

    // Map of governor-quid → hex signature over
    // canonicalBytes(tx with GovernorSigs cleared).
    GovernorSigs map[string]string `json:"governorSigs"`
}
```

### 4.2 Action enum

| Action | Effect | Required signers |
|---|---|---|
| `ADD_VALIDATOR` | Add `Subject` (a quid) to `Validators[Subject] = TargetWeight` and set `ValidatorPublicKeys[Subject] = (looked up from identity registry)` | ≥ quorum of current governors |
| `REMOVE_VALIDATOR` | Delete `Validators[Subject]` and `ValidatorPublicKeys[Subject]` | ≥ quorum of current governors |
| `UPDATE_VALIDATOR_WEIGHT` | Set `Validators[Subject] = TargetWeight` | ≥ quorum of current governors |
| `SET_TRUST_THRESHOLD` | Set `TrustThreshold = TargetThreshold` | ≥ quorum of current governors |
| `DELEGATE_CHILD` | Allow a specific third-party operator's quorum to govern `ChildDomain`. Sets child's `ParentDelegationMode = "delegated"` and `DelegatedFrom = TargetDomain`; child's `Governors` becomes `ProposedGovernors`. The child must exist and be an immediate child (one hop deeper). | ≥ quorum of **parent** governors (i.e. governors of `TargetDomain`, where `TargetDomain` is the parent) |
| `REVOKE_DELEGATION` | Reset child's governance to inherit from parent. Requires the same quorum as the original `DELEGATE_CHILD`. | ≥ quorum of **parent** governors |
| `UPDATE_GOVERNORS` | Atomically swap `Governors`, `GovernorPublicKeys`, and `GovernanceQuorum` to `ProposedGovernors` / `ProposedGovernorPublicKeys` / `ProposedGovernanceQuorum`. | **Unanimous** (every current governor's signature) OR if unanimity is impossible, the existing guardian-recovery primitive (QDP-0002) for each governor |

`UPDATE_GOVERNORS` uses unanimity because it's the primitive
most dangerous to mis-use: once done, the old governors can't
roll it back without the new governors' consent. If unanimity
is too hard in practice, operators should use guardian recovery
to restore a lost governor, not lower the quorum.

### 4.3 Validation rules

Every `DOMAIN_GOVERNANCE` transaction is rejected unless ALL
of the following hold:

1. **Domain exists.** `TargetDomain` (and `ChildDomain`, if set)
   are registered in the identity registry at the time of
   receipt.
2. **Nonce monotonicity.** `Nonce > D.GovernanceNonce`. The
   nonce jump to `D.GovernanceNonce + 1` is preferred but gaps
   are allowed to tolerate lossy pending pools.
3. **Notice period.** `EffectiveHeight >= current_block_index
   + MinGovernanceNoticeBlocks` (default 1440 blocks; at a
   typical 60-second block interval that's ~24 hours).
4. **Action valid.** `Action` is one of the enums in §4.2.
5. **Action-specific validation.**
   - `ADD_VALIDATOR`: `Subject` must be in the identity
     registry; `TargetWeight > 0`.
   - `REMOVE_VALIDATOR`: `Subject` must be currently in
     `D.Validators`; the removal must not reduce the validator
     count below `MinValidatorsPerDomain` (default 1).
   - `DELEGATE_CHILD`: `ChildDomain` must be an immediate child
     of `TargetDomain`; `ProposedGovernors` must be non-empty;
     child's current delegation mode must be `"inherit"`.
   - Others: analogous structural checks.
6. **Signer check.** Every quid in `GovernorSigs` must be in
   `D.Governors` (or for `DELEGATE_CHILD` / `REVOKE_DELEGATION`,
   in the parent's `Governors`).
7. **Signature verification.** For each signer `Q` in
   `GovernorSigs`, `VerifySignature(D.GovernorPublicKeys[Q],
   canonicalBytes(tx-without-GovernorSigs), GovernorSigs[Q])`
   must succeed. Missing, duplicate, or malformed signatures
   reject the whole transaction.
8. **Quorum.** The sum of `D.Governors[Q]` across valid
   signers must meet or exceed
   `D.GovernanceQuorum * sum(D.Governors.values())`.
   For `UPDATE_GOVERNORS`, every `D.Governors` quid must sign.
9. **No-conflict.** A later-nonce governance transaction for
   the same `(TargetDomain, Action, Subject)` tuple already in
   the pending or activated pool causes this one to be
   rejected — replay protection AND operator sanity (don't
   queue two contradictory changes).
10. **Signer keys live.** Each signing governor's epoch state
    must be live (not in a frozen epoch per QDP-0007). If a
    governor is mid-rotation, the signature must reference the
    new or old epoch consistent with QDP-0007's lazy probe.

### 4.4 Activation semantics

A validly-signed `DOMAIN_GOVERNANCE` transaction is included in
a block like any other transaction. The block's inclusion
records the intent but does not apply the change. When a block
at height `H >= tx.EffectiveHeight` is processed, the change is
applied idempotently to the domain state:

```
for tx in block.Transactions where tx.Type == DOMAIN_GOVERNANCE
    and tx.EffectiveHeight <= block.Index:
    applyGovernanceAction(tx)
    D.GovernanceNonce = max(D.GovernanceNonce, tx.Nonce)
```

The idempotence requirement means late-joining nodes can replay
the block history and converge on the same domain state.

### 4.5 Supersede and veto

During the notice period (between tx acceptance and
`EffectiveHeight`) any subsequent governance tx with a
**strictly higher** nonce replaces the pending one. This is
the supersede path: if a mistake is detected before activation,
the same quorum can issue a corrective tx with `Action =
SUPERSEDE` (or any other action) at a later nonce.

For emergency revocation during the notice period, the
`SUPERSEDE` action takes a quorum-signed transaction with no
other effect than bumping the nonce, thereby nullifying the
pending change. Noise level is low — issued only on emergencies.

After `EffectiveHeight` the change is historical fact. Reverting
it requires a fresh governance tx.

## 5. Bootstrap of a new domain

When a domain is registered today via `RegisterTrustDomain`, the
registering node becomes the sole validator. Under QDP-0012 the
registering actor explicitly declares the initial governance
state:

```json
{
    "type": "DOMAIN_REGISTRATION",
    "name": "reviews.public",
    "trustThreshold": 0.5,
    "validators": {
        "<seed-1-quid>": 1.0,
        "<seed-2-quid>": 1.0,
        "<seed-3-quid>": 1.0
    },
    "validatorPublicKeys": { "...": "..." },
    "governors": {
        "<operator-personal-quid>": 1.0,
        "<co-founder-quid>": 1.0
    },
    "governorPublicKeys": { "...": "..." },
    "governanceQuorum": 1.0
}
```

If `governors` is omitted, the registrant becomes the sole
governor with quorum 1.0 (pre-QDP-0012 behavior, for
backward compatibility with existing domain registrations).

**For the `reviews.public` tree specifically** the seed operator
registers:

- `reviews.public` with the three seed-node quids as validators,
  the operator personal + co-founder quids as 2-of-2 governors.
- Each child (`reviews.public.technology`, `...restaurants`, etc.)
  inherits the parent's governors by default
  (`ParentDelegationMode = "inherit"`).

Any change to the validator or governor set thereafter goes
through `DOMAIN_GOVERNANCE`.

## 6. Joining the network as a new operator

The journey for a third-party operator who wants to participate:

### 6.1 Starts as a cache replica

1. Build and run `quidnug` locally.
2. Configure `seed_nodes: ["node1.quidnug.com", "node2.quidnug.com"]`.
3. Use QDP-0008 K-of-K bootstrap to sync historical blocks from
   multiple seed nodes, establishing a local mirror of the
   agreed chain.
4. Post their node quid's identity transaction to the public
   network (reaches seed validators via gossip).
5. (Optional) publish a `TRUST` edge from their own operator
   quid toward the seed consortium members in the relevant
   validator domain — this tells their own node "accept blocks
   from this consortium for this domain as Trusted," which is
   what makes the cache replica actually useful.

At this point the new node is a **cache replica** for
`reviews.public.*`: it accepts blocks produced by the seed
consortium, mirrors the agreed chain, and serves read queries.
It **cannot produce** blocks for those domains — not yet. It
can still produce blocks in domains it itself registered (where
it's the sole / founding consortium member), and still gossip
transactions into the network for consortium inclusion.

### 6.2 Earns individual observer trust

Anyone who individually trusts the new operator can publish a
`TRUST` edge from themselves to the new node's quid in the
relevant validator domain. This affects **their own** tiering
of blocks — they'll accept blocks from the new node as Trusted
if the new node ever does become a consortium member. But this
per-observer trust does NOT promote the new node into the
consortium. The consortium is part of the domain's on-chain
state, not of any individual's trust graph.

### 6.3 Gets admitted to the consortium (on-chain)

The seed operator (or whoever governs the specific sub-tree)
decides the new operator has earned consortium membership.
They issue:

```json
{
    "type": "DOMAIN_GOVERNANCE",
    "targetDomain": "reviews.public.technology.laptops",
    "action": "ADD_VALIDATOR",
    "subject": "<new-operator-quid>",
    "targetWeight": 1.0,
    "nonce": 5,
    "effectiveHeight": <current + 1500>,
    "memo": "promoting third-party operator X after 6 months of follower-only operation",
    "governorSigs": {
        "<operator-personal-quid>": "<sig-over-canonical-bytes>",
        "<co-founder-quid>": "<sig>"
    }
}
```

24 hours after acceptance, the new operator is a consortium
member for that specific sub-domain. Their blocks are admissible;
cache replicas accept them via the same tiered acceptance as any
other consortium member's blocks.

### 6.4 Earns sub-tree delegation (optional)

If over time the new operator becomes the primary / responsive /
active validator for some sub-tree, the seed operator can
delegate governance of that sub-tree:

```json
{
    "type": "DOMAIN_GOVERNANCE",
    "targetDomain": "reviews.public.technology.laptops",
    "action": "DELEGATE_CHILD",
    "childDomain": "reviews.public.technology.laptops.enthusiast",
    "proposedGovernors": {
        "<new-operator-quid>": 1.0,
        "<new-operator-cofounder>": 1.0
    },
    "proposedGovernanceQuorum": 1.0,
    "nonce": 7,
    "effectiveHeight": <current + 1500>,
    "governorSigs": { ... seed operator's governor signatures ... }
}
```

The new operator now has governance authority over that one
sub-domain; they can add their own validators, set their own
thresholds, etc. The parent (seed operator) retains `REVOKE_DELEGATION`
if things go wrong.

## 7. Attack vectors and mitigations

Every new primitive opens new abuse surfaces. The ones we've
enumerated, with their mitigations:

### 7.1 Self-promotion attack

**Attack:** A new operator registers `reviews.public.foo` and
declares themselves a validator for the parent `reviews.public`.

**Mitigation:** `ADD_VALIDATOR` for `D` requires ≥ quorum of
`D.Governors`, not the candidate's own signature. A new
operator can only add themselves as a validator by social
means — convincing the governor quorum.

### 7.2 Governance nonce replay

**Attack:** An attacker intercepts a valid governance tx, stores
it, waits for the original effect to be superseded, then
re-broadcasts the old tx in a partition.

**Mitigation:** `GovernanceNonce` is strictly monotonic
per-domain. Once a later tx lands, the earlier one fails
`Nonce > D.GovernanceNonce`.

### 7.3 Sybil governors

**Attack:** Attacker registers N fake quid identities and
declares them governors, meeting a quorum of N/N.

**Mitigation:** At initial domain registration, the registrant
chooses the governors. An attacker who registers a domain with
fake governors only harms themselves — nothing they do with
that domain affects any other operator. For public domains, the
seed operator publishes the governor list in `seeds.json` so
anyone can audit.

### 7.4 Out-of-bounds child takeover

**Attack:** A child-domain governor signs a governance tx
targeting the parent domain.

**Mitigation:** `TargetDomain` field is authoritative. Governors
of `reviews.public.technology` cannot sign a change to
`reviews.public` because the quorum check evaluates `D.Governors`
for `D = TargetDomain`.

### 7.5 Fresh-node acceptance attack

**Attack:** A brand-new node starts with an empty `TrustDomains`
map and accepts any block claiming to be from any validator
(because `D.Validators` hasn't been populated yet).

**Mitigation:** This is QDP-0008's job already: new nodes
bootstrap from a K-of-K quorum of peer snapshots, not from
nothing. A node that skips bootstrap is a self-sabotage; its
"validation" applies only to itself.

### 7.6 Delegated-revoked race

**Attack:** Parent delegates sub-tree to operator X. Operator X
publishes malicious blocks. Parent revokes. But the malicious
blocks have already been accepted by followers during the
window.

**Mitigation:**
1. `DELEGATE_CHILD` requires a notice period (24h default) —
   during that period, other operators see the pending change
   and can veto by publishing trust edges against X.
2. Revocation also takes effect at a future block height, so
   operators can race to contain damage.
3. The existing tiered-acceptance model means followers who
   have tight local trust graphs already de-weight blocks from
   an operator they don't know; the public signal only matters
   to observers who explicitly trust whatever the seed
   operator blesses.

### 7.7 Governor key compromise

**Attack:** An attacker steals a governor's private key and
issues a `REMOVE_VALIDATOR` action against every honest
validator, or an `UPDATE_GOVERNORS` replacing the honest
quorum with their own.

**Mitigation:**
1. `UPDATE_GOVERNORS` requires unanimity. A compromised single
   governor can't replace the set; they'd need ALL of them.
2. For single-governor changes (`ADD_VALIDATOR`, etc.) a quorum
   is required. A compromised minority of governors can be
   outvoted by the honest majority.
3. The notice period gives honest governors a window to detect
   and `SUPERSEDE` or use the QDP-0002 guardian recovery to
   rotate the compromised key before `EffectiveHeight`.
4. Alerting: every governance tx is a rare event; the operator
   should have a monitor that fires on every
   `DOMAIN_GOVERNANCE` tx for domains they care about.

### 7.8 Governance-spam DoS

**Attack:** An attacker posts thousands of invalid governance
transactions to clog the pending pool or governors' inboxes.

**Mitigation:** Standard tx rate-limiting applies. Invalid
governance transactions are cheap to reject (signature check
is the hot path). They never reach a block because the
quorum condition fails.

### 7.9 Long-lived pending conflict

**Attack:** Two governor subsets queue contradictory actions
(one adds validator X, another removes X) before either
activates, causing confusion.

**Mitigation:** The no-conflict rule in §4.3.9 rejects a second
pending tx on the same `(TargetDomain, Action, Subject)` tuple.
Contradictory actions on different tuples (add X / remove Y)
can legitimately queue; their ordering at activation time is
deterministic by nonce.

### 7.10 Validator set reduction below quorum

**Attack:** Governors repeatedly remove validators until only
one remains, then that one is compromised.

**Mitigation:** `MinValidatorsPerDomain` enforced at validation
time. Default 1 for compatibility but operators should configure
`min_validators: 3` or higher for production public domains.

### 7.11 Privileged-tx injection

**Attack:** A node operator publishes a fake `DOMAIN_GOVERNANCE`
tx claiming to be signed by governors who never signed it.

**Mitigation:** Signature verification against
`D.GovernorPublicKeys` is mandatory per §4.3.7. Forgery is
computationally infeasible (ECDSA P-256 + SHA-256).

### 7.12 Time-shift via network partition

**Attack:** During a network partition, a faction produces
blocks fast enough to pass the `EffectiveHeight` of a pending
governance tx before the other faction sees it, locally
applying the change. At heal, the two factions disagree on
whether the change is applied.

**Mitigation:** Block-height is deterministic from the chain;
at partition heal the canonical chain wins by the existing
tiered-acceptance rules. Whichever faction's chain is accepted
as Trusted by an observer determines what state that observer
sees.

### 7.13 Validator epoch rotation race

**Attack:** A validator rotates keys (QDP-0001 anchor), then an
attacker uses the old key to sign blocks during the notice
window.

**Mitigation:** Per QDP-0007, epoch propagation is lazy but
bounded; signers of any tx (governance included) must have a
live epoch key. Old-epoch signatures are rejected as soon as
the rotation gossip reaches the receiver, which in practice is
seconds.

## 8. Compatibility with existing QDPs

### QDP-0001 (Nonce ledger)

`GovernanceNonce` is a new per-domain nonce dimension. It's
independent of QDP-0001's per-signer nonce ledger (which
protects TRUST / IDENTITY / EVENT / ANCHOR / etc. transactions
from replay). Governance tx nonces stack with signer nonces:
each governor's signature on a governance tx must have a valid
signer nonce too.

### QDP-0002 (Guardian recovery)

Governors are ordinary quids; if a governor's key is compromised
they can trigger `GuardianRecoveryInit` just like any other quid.
Post-recovery, their new key's signatures on governance tx
validate against the rotated `GovernorPublicKeys` entry — which
itself must have been refreshed via a `UPDATE_GOVERNORS` tx.
That's an important subtlety: recovery rotates the quid's key,
but `GovernorPublicKeys` is stored in the domain state and must
be updated to match. A scheduled `UPDATE_GOVERNORS` action
refreshing only the public keys is a safe post-recovery step.

### QDP-0003 (Cross-domain nonce scoping)

`DOMAIN_GOVERNANCE` is scoped to a single `TargetDomain`, so
cross-domain nonce scoping doesn't apply directly. Governors
who participate in multiple domains maintain independent
`GovernanceNonce` values per-domain.

### QDP-0008 (K-of-K bootstrap)

New nodes bootstrap their view of each domain's
`Validators` / `Governors` / `ValidatorPublicKeys` /
`GovernorPublicKeys` / `GovernanceNonce` from a K-of-K quorum
of peer snapshots. A node that bootstraps with < K peers has an
unverified domain state and should refuse to produce blocks
until it's verified.

### QDP-0009 (Fork-block activation)

The enforcement of `DOMAIN_GOVERNANCE` as a gate on validator
mutation is activated via a new `enforce_domain_governance`
feature flag. Pre-activation, existing domain registrations
continue to auto-register the registrant as a validator with a
single-governor fallback. Post-activation, non-governance
mutations of `Validators` / `ValidatorPublicKeys` are rejected.

This lets early public-network operators bootstrap without
QDP-0012 behavior, then flip the switch at a planned height
once everyone's upgraded.

### QDP-0010 (Compact Merkle proofs)

Governance transactions are regular transactions and hash into
the block's `TransactionsRoot` like any other. Light clients
can prove inclusion of a specific governance action using the
standard QDP-0010 path.

## 9. Worked example — bootstrapping `reviews.public`

The full sequence for launching the public reviews tree under
QDP-0012.

### Step 1: Registration (one-time, by seed operator)

The operator runs three seed nodes (quids `S1`, `S2`, `S3`).
They hold two personal quids `P1` (you) and `P2` (a
co-founder) for governance.

```bash
quidnug-cli domain register \
    --name "reviews.public" \
    --trust-threshold 0.5 \
    --validators "S1:1.0,S2:1.0,S3:1.0" \
    --governors "P1:1.0,P2:1.0" \
    --governance-quorum 1.0 \
    --key P1.key.json
```

Result: `D = reviews.public` with three validators, two
governors (unanimous quorum), `GovernanceNonce = 0`.

### Step 2: Children registered with inherited governance

```bash
for child in \
    reviews.public.technology \
    reviews.public.technology.laptops \
    reviews.public.restaurants \
    reviews.public.books; do
    quidnug-cli domain register \
        --name "$child" \
        --parent-delegation-mode inherit \
        --key P1.key.json
done
```

Result: each child references `reviews.public` as its governance
source (`DelegatedFrom = "reviews.public"`).

### Step 3: Operator X joins (6 months later)

X has been running a follower node, publishing their own
reviews, and has earned reputation. The seed operator promotes
them:

```bash
quidnug-cli governance propose \
    --target-domain "reviews.public.technology.laptops" \
    --action ADD_VALIDATOR \
    --subject "<X-node-quid>" \
    --target-weight 1.0 \
    --nonce 1 \
    --effective-height +1500 \
    --memo "promoting X after 6 months of clean follower-only operation" \
    --sign-with P1.key.json

# A human-readable URL is printed. Co-founder P2 signs the
# same tx off-line:
quidnug-cli governance co-sign \
    --pending-tx <id> \
    --sign-with P2.key.json
```

At the scheduled height, X becomes a validator for that sub-domain.
Their blocks under `reviews.public.technology.laptops` are now
admissible.

### Step 4: X gets their own sub-domain delegated

After another period of good operation:

```bash
quidnug-cli governance propose \
    --target-domain "reviews.public.technology.laptops" \
    --action DELEGATE_CHILD \
    --child-domain "reviews.public.technology.laptops.enthusiast" \
    --proposed-governors "X:1.0,X-co:1.0" \
    --proposed-governance-quorum 1.0 \
    --nonce 2 \
    --effective-height +1500 \
    --memo "delegating enthusiast sub-tree to X" \
    --sign-with P1.key.json
# ... P2 co-signs ...
```

X now fully governs `reviews.public.technology.laptops.enthusiast`
and can add their own validators for it.

### Step 5: X misbehaves, seed operator revokes

```bash
quidnug-cli governance propose \
    --target-domain "reviews.public.technology.laptops" \
    --action REVOKE_DELEGATION \
    --child-domain "reviews.public.technology.laptops.enthusiast" \
    --nonce 3 \
    --effective-height +1500 \
    --memo "revoking X delegation due to repeated validator spam" \
    --sign-with P1.key.json
# ... P2 co-signs ...
```

At the scheduled height, governance of the sub-tree falls back
to the parent (`reviews.public.technology.laptops`). X's past
blocks remain on-chain (the protocol is append-only) but they
can no longer issue new governance actions for that sub-tree.

## 10. Implementation plan

Phased so the protocol changes land in small, testable pieces.

### Phase 1 — State extension (no behavior change)

- Add the new fields to `TrustDomain` with `omitempty`.
- Populate `Governors` / `GovernorPublicKeys` at registration
  from the (optional) new CLI flags; fall back to `{registrant: 1.0}`
  with quorum 1.0.
- Add `GovernanceNonce: 0`.
- Existing nodes that read the new fields via JSON unmarshaling
  see zero-values; nothing breaks.

### Phase 2 — Transaction type (shadow mode)

- Add `TxTypeDomainGovernance` and the struct.
- Implement `ValidateDomainGovernanceTransaction` + registry
  handler.
- Accept governance transactions into the pending pool; include
  them in blocks; APPLY them at `EffectiveHeight`.
- Fork flag: `enforce_domain_governance_shadow` (bool, default
  true once Phase 2 ships). Shadow mode means pre-QDP-0012
  validator-set edits continue to work side-by-side.

### Phase 3 — Enforcement (fork-block gated)

- Add `enforce_domain_governance` to QDP-0009's
  `ForkSupportedFeatures`.
- Once activated, reject any validator-set mutation that
  doesn't come through a `DomainGovernanceTransaction`.
- Block production gate: `GenerateBlock(D)` errors out if
  `node.NodeQuidID ∉ D.Validators`.

### Phase 4 — CLI + SDK ergonomics

- `quidnug-cli governance propose / co-sign / status / list-pending`.
- SDK methods (Go, Python, JS, Rust) for building and signing
  governance transactions.
- Site: a public page at `quidnug.com/network/governance` showing
  all in-flight and historical governance actions for public
  domains, with verification badges ("✅ signed by ≥ quorum").

### Phase 5 — Monitoring

- Prometheus counters: `quidnug_governance_tx_total{action,domain}`,
  `quidnug_governance_pending_total{domain}`,
  `quidnug_governance_applied_total{action,domain}`.
- Alert rule: any `DOMAIN_GOVERNANCE` for a seed-operated domain
  triggers a human-routed page.

### Phase 6 — Deprecation of the legacy registration path

- Once QDP-0012 has been live for ~6 months and no domains
  remain on the single-registrant governance fallback, drop
  the fallback entirely.

Implementation effort: ~3-4 person-weeks for Phases 1-3; another
1-2 weeks for ergonomics; monitoring is cheap.

## 11. Open questions

1. **Should governor weights accumulate across parent and child?**
   Alternative: a parent governor has implicit veto power on
   any child even if delegated. Current design says no — delegation
   transfers authority fully until revoked. Revisit if
   operators ask for "read-only oversight."

2. **Governance voting history per quid.** Worth surfacing in
   the identity registry / website? Yes; defer to a Phase 5
   monitoring task.

3. **How to handle the initial genesis governors.**
   Bootstrap chicken-and-egg: when the very first public
   domain is registered, there are no prior governors to
   sign. Resolution: domain registration itself is signed by
   the registrant and carries the initial `Governors` set in a
   `DOMAIN_REGISTRATION` transaction. Ratified retroactively
   by whoever chooses to follow that chain.

4. **Should the `SUPERSEDE` action be its own type or an enum
   value?** Enum is simpler. Action: revisit if usage is
   frequent enough to warrant its own tx shape.

5. **Signature schemes for governors.** Current design assumes
   per-governor ECDSA sigs. Future: threshold-sig schemes
   (QDP-TBD) could consolidate to a single aggregated
   signature, reducing on-chain size.

## 12. Review status

Draft. Needs:

- Implementation of Phase 1 (no behavior change; safe to ship).
- Prototype CLI invocations for the worked example.
- External review of the attack-vector section; the list is
  what I could enumerate from first principles but more eyes
  on it would help.
- Operator review of the notice-period default. 24h / 1440
  blocks is reasonable for a first stab; real data from early
  promotion operations will inform adjustment.

## 13. References

- [`docs/architecture.md`](../architecture.md) — relational trust,
  tiered acceptance.
- [`docs/design/0001-global-nonce-ledger.md`](0001-global-nonce-ledger.md)
  — nonce monotonicity precedent.
- [`docs/design/0002-guardian-based-recovery.md`](0002-guardian-based-recovery.md)
  — key-rotation primitive used for compromised governors.
- [`docs/design/0008-kofk-bootstrap.md`](0008-kofk-bootstrap.md)
  — how new nodes ingest domain state including governance.
- [`docs/design/0009-fork-block-trigger.md`](0009-fork-block-trigger.md)
  — activation path for the enforcement flag.
- [`deploy/public-network/README.md`](../../deploy/public-network/README.md)
  — operator playbook.
- [`deploy/public-network/peering-protocol.md`](../../deploy/public-network/peering-protocol.md)
  — bilateral trust convention this proposal builds on.
- [`deploy/public-network/governance-model.md`](../../deploy/public-network/governance-model.md)
  — the operator-facing summary of this QDP.
