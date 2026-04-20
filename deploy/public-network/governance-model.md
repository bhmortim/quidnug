# Public-network governance model

> The plain-English version of [QDP-0012](../../docs/design/0012-domain-governance.md).
> Read this first if you want to understand how nodes join the
> public Quidnug network and what roles they can hold. Read the
> QDP if you want the protocol details and the attack-vector
> analysis.

## The core idea in three sentences

A public domain (like `reviews.public`) has a **consortium** of
nodes that collectively produce its blockchain. Any other node
can become a **cache replica** for that domain by trusting the
consortium — the replica mirrors the agreed chain locally,
serves read queries, and gossips transactions to the
consortium. Joining the consortium itself — so your own blocks
are accepted as part of the agreed chain — requires a signed
on-chain promotion by the domain's governors.

Three roles, one domain:

```
                       ┌───────────────────────────┐
                       │     Governor quorum       │
                       │  (M-of-N signed votes)    │
                       │                           │
                       │  mutates the consortium   │
                       │  roster via signed txs    │
                       └─────────────┬─────────────┘
                                     │
                                     ▼
      ┌──────────────────────────────────────────────────┐
      │              Consortium members                  │
      │  (produce blocks; their blocks are "the chain")  │
      │                                                  │
      │     N1 ◄──► N2 ◄──► N3 ◄──► ... ◄──► Nk          │
      └──────────┬────────────────────────────┬──────────┘
                 │                            │
                 │  gossip blocks, rotations  │
                 ▼                            ▼
      ┌──────────────────────────────────────────────────┐
      │                 Cache replicas                   │
      │       (mirror the chain; serve reads;            │
      │        relay txs back into the consortium)       │
      │                                                  │
      │   R1     R2     R3     R4    ...     Rn          │
      └──────────────────────────────────────────────────┘
```

Everything on the public network — reviews, identities, trust
edges, everything — is defined within a domain that has this
structure.

## What each role actually does

### Cache replica

The default role for any node that chooses to participate. To
become a cache replica for domain `D`:

1. Boot a `quidnug` node.
2. Point it at the public seed nodes via `seed_nodes` config.
3. Bootstrap the chain state using QDP-0008 (k-of-k snapshot).
4. Publish a `TRUST` edge from the operator quid toward the
   current consortium members in the relevant validator domain
   (typically `validators.<domain>.network.quidnug.com`). This
   tells your node "accept blocks from these members as
   `Trusted` for this domain."

Your node now:

- Has a local, continuously-updated mirror of `D`'s chain.
- Can answer any read query (`GET /api/trust/...`,
  `GET /api/streams/...`, etc.) from its local state.
- Forwards locally-received transactions (reviews, votes, trust
  edges, whatever) toward the consortium via gossip; consortium
  members decide whether to include them in future blocks.
- Does NOT produce blocks for `D`. Attempting to do so is a
  protocol error post-QDP-0012 enforcement.

Think of a cache replica as a CDN edge for the agreed chain.
It makes the network faster + more resilient without changing
the agreement.

### Consortium member (validator)

A cache replica that's been promoted to produce blocks. To
become a consortium member for domain `D`, your quid must be
listed in `D.Validators`. The only way to get listed is by an
on-chain `DOMAIN_GOVERNANCE` transaction signed by a quorum of
`D`'s current governors.

Once you're a consortium member, your node:

- Produces blocks for `D` on the regular block-generation
  interval.
- Gossips those blocks to other consortium members + cache
  replicas.
- Continues to accept blocks produced by OTHER consortium
  members. Membership doesn't make you the sole authority —
  you're one of a group.

Because the consortium is a group, the chain for `D` is
"agreed" in the sense that any consortium member's valid block
advances it. Disagreements among consortium members (e.g.,
network partitions) are handled by the existing tiered-block-
acceptance machinery — observers resolve via their own trust
graphs when the partitions heal.

### Governor

A quid authorized to vote on changes to `D`'s consortium and
parameters. Governors are set at domain registration time and
can be changed only by unanimous agreement of the current
governors.

Governors do NOT have to run nodes or produce blocks. They can
be human operators, multi-sig keys, or an M-of-N quorum of
stakeholders. They're the "policy layer" above the operational
consortium.

Typical governance actions:

| Action | What it does |
|---|---|
| `ADD_VALIDATOR` | Admit a new node to the consortium |
| `REMOVE_VALIDATOR` | Expel a node from the consortium |
| `SET_TRUST_THRESHOLD` | Adjust the trust threshold for blocks in this domain |
| `DELEGATE_CHILD` | Transfer governance of a child domain to a different quorum |
| `REVOKE_DELEGATION` | Take back governance of a previously-delegated child |
| `UPDATE_GOVERNORS` | Replace the governor set (requires unanimity) |

Every governance action takes effect at a future block height
(default 24-hour notice period), giving other governors +
operators time to react if something's wrong.

## How the public network grows, step by step

This is the narrative you'd explain to a potential third-party
operator asking "how do I join?"

### Day 0: You register the public tree

You (the seed operator) register `reviews.public` and its
children, declaring:

- **Consortium:** your three seed nodes (`seed-1`, `seed-2`,
  `seed-3` quids).
- **Governors:** your personal quid + co-founder's quid, 2-of-2
  quorum.

Now anyone in the world can trust your consortium and establish
a cache replica. Nobody else can produce blocks for
`reviews.public.*` yet.

### Day 30: Third-party operator X joins as a cache replica

X runs a node pointing at your seeds, bootstraps via k-of-k,
publishes a TRUST edge toward your consortium. Their node
mirrors the public reviews chain. They can serve reviews to
their users instantly (local reads, no round-trip to your
seeds). They can accept new reviews from their users and relay
them into the consortium.

X is not yet producing any blocks. That's fine. Cache replicas
are the backbone of network resilience — more cache replicas
means faster reads, more DDoS surface, more fault-tolerant
coverage.

### Day 120: You promote X to the consortium for one sub-tree

X has been running a cache replica for three months, clean
history, no spam. You decide they've earned consortium
membership for a specific sub-tree — say
`reviews.public.technology.laptops`:

```bash
quidnug-cli governance propose \
    --target-domain reviews.public.technology.laptops \
    --action ADD_VALIDATOR \
    --subject <X-quid> \
    --target-weight 1.0 \
    --effective-height +1500 \
    --memo "promoting X after 4 months clean cache operation" \
    --sign-with personal.key.json
# co-founder co-signs:
quidnug-cli governance co-sign --pending-tx <id> --sign-with cofounder.key.json
```

24 hours later X is a consortium member for laptops. Their
blocks are part of the agreed chain. They're still only a cache
replica for all the other `reviews.public.*` sub-trees.

### Day 365: X earns delegation of a sub-tree

X has been an exemplary consortium member for laptops. They ask
for governance delegation of the enthusiast sub-tree so they
can operate it autonomously. You issue:

```bash
quidnug-cli governance propose \
    --target-domain reviews.public.technology.laptops \
    --action DELEGATE_CHILD \
    --child-domain reviews.public.technology.laptops.enthusiast \
    --proposed-governors X:1.0,X-co:1.0 \
    --effective-height +1500 \
    --memo "delegating enthusiast sub-tree to X"
```

X now fully governs that one sub-domain. They can admit or
remove their own consortium members, set their own parameters,
delegate further children. You still govern everything outside
the delegated sub-tree. If X misbehaves, you `REVOKE_DELEGATION`
and governance reverts to you.

This is the "grow the network by delegation" pattern. You
retain authority over the root; trusted operators grow branches.

## How this plays against attack scenarios

The QDP has the formal list. Here's the operator's version.

### "A stranger claims to be a validator for reviews.public"

They can't. Unless their quid is in `D.Validators` — which only
happens via a governance tx signed by you and your co-founder —
their blocks fail the block-production gate. Nodes running the
current software reject them.

Their TRUST edges toward their own node are their own business,
but those edges don't promote them. The per-observer trust
graph and the on-chain consortium roster are separate concerns.

### "Someone stole a governor's key and is trying to expel validators"

Three layers of defense:

1. **Quorum requirement:** a single governor can't act alone.
   The attacker needs both governor keys.
2. **Notice period:** a governance action takes effect 24h
   after acceptance. Honest governors have a day to issue a
   `SUPERSEDE` action invalidating the malicious one.
3. **Guardian recovery:** each governor has a QDP-0002 guardian
   quorum that can rotate their key if compromised. Post-recovery,
   the new key can re-sign an `UPDATE_GOVERNORS` action refreshing
   the `GovernorPublicKeys` entry to match.

The attacker has to steal more keys than the quorum requires,
AND act within 24 hours, AND suppress the honest governors'
ability to counteract. Hard.

### "Someone is publishing abusive reviews"

This isn't a governance-layer attack; it's a content moderation
issue. The consortium doesn't gate individual transactions
beyond basic cryptographic + structural validation. Abuse
filtering happens at the rating-algorithm layer — reviews from
untrusted reviewers get low weight automatically. Explicit
moderation is via the `FLAG` event type.

Governance comes in only when you want to remove a repeat
offender from ever producing blocks — that's an operator-level
decision, not a network-level one.

### "A forked chain appears claiming to be reviews.public"

Anyone can register a domain with the same name if they start a
new network. The protocol's relativistic — there is no global
namespace authority. What matters is which chain you (and your
cache replicas and consortium members) agree is "the" one.

In practice, this is solved by social attestation:

- Seeds publish their quid IDs in `seeds.json` and in operator
  attestations.
- Cache replicas explicitly choose which consortium to trust.
- Anyone who forks and claims to be the same domain needs
  operators to believe them. Without trust edges toward the
  forked consortium, their chain has zero observers.

Governance doesn't prevent forks. It just makes the agreed-on
lineage auditable and cryptographically bound.

### "All my governors are dead / lost their keys simultaneously"

Use the guardian quorum. Each governor key has a QDP-0002
guardian set (people / orgs you chose at setup time) who can
rotate the key after a time-lock. Once any single governor's
key is recovered, they can issue a quorum-exemption
`UPDATE_GOVERNORS` action using the guardian-recovery flag,
bootstrapping the governor set back to operational.

If ALL governors are lost AND their guardians are all lost, the
network forks. The existing chain is frozen; operators who want
to continue governance have to start a new domain, migrate
data by re-anchoring, and persuade cache replicas to follow the
new lineage. This is a network-rebirth event; the point of
having multiple governors + guardians is to make it
vanishingly unlikely.

## What you (the seed operator) need to decide before QDP-0012 ships

Before the protocol change lands and you register
`reviews.public` under the new regime, pick:

| Decision | Default suggestion | Why |
|---|---|---|
| Initial consortium | Your three seed nodes | Matches your infrastructure. |
| Initial governors | You + co-founder, 2-of-2 | Small enough for fast action, dual enough to prevent single-point accidents. |
| Governance quorum | 2/3 for routine actions; all for `UPDATE_GOVERNORS` | Industry-standard balance. |
| Notice period | 24 hours (1440 blocks at 60s intervals) | Enough for operators to react; short enough for real business. |
| Child-domain delegation mode at registration | `inherit` | Children default to parent governance until explicitly delegated. |
| `MinValidatorsPerDomain` | 1 | Permits sparsely-populated sub-trees; raise for critical ones. |

These all become defaults; individual domains can override at
registration or via `UPDATE_GOVERNORS`.

## How this interacts with the rest of the home-operator plan

The [home-operator plan](home-operator-plan.md) gets a few
tweaks to land cleanly under QDP-0012:

1. **Phase 1** (home node setup) is unchanged — you're
   registering domains as the sole operator, so the legacy
   single-registrant governance works.
2. **Phase 2** (failover VPS) is unchanged — the VPS key
   becomes a second consortium member, not a governor.
3. **Phase 6** (reviews launch) adds one step: when
   registering `reviews.public.*`, explicitly supply the
   initial governor set and quorum. Without QDP-0012
   enforcement active yet, this is forward-compatible
   documentation; with enforcement, it's required.
4. **Long-term growth** happens through the delegation pattern
   in §"How the public network grows" above.

The [reviews-launch checklist](reviews-launch-checklist.md)
gets an entry right after topic registration: "publish the
initial governor attestation and link it from
`seeds.json`." That's the operator's public commitment to who
can act as governor for the public tree.

## Further reading

- [QDP-0012 formal spec](../../docs/design/0012-domain-governance.md)
  — attack vectors, validation rules, rollout plan, worked
  example.
- [QDP-0002 guardian recovery](../../docs/design/0002-guardian-based-recovery.md)
  — the mechanism governors use to rotate compromised keys.
- [QDP-0008 k-of-k bootstrap](../../docs/design/0008-kofk-bootstrap.md)
  — how new cache replicas safely ingest the agreed chain from
  multiple peers.
- [QDP-0009 fork-block activation](../../docs/design/0009-fork-block-trigger.md)
  — how the QDP-0012 enforcement flag flips on network-wide.
- [Peering protocol](peering-protocol.md) — the bilateral trust
  convention new operators use to request cache + consortium
  membership.
