# Elections integration with the three architectural pillars

> How the elections use case composes on top of the three
> architectural QDPs (0012 Domain Governance, 0013 Network
> Federation, 0014 Node Discovery + Sharding). Written as a
> companion to [`README.md`](README.md) +
> [`architecture.md`](architecture.md), which predate the
> pillars and describe the election semantics without
> addressing the larger protocol architecture.
>
> Read the three pillar documents first:
> [`docs/design/0012-domain-governance.md`](../../docs/design/0012-domain-governance.md),
> [`docs/design/0013-network-federation.md`](../../docs/design/0013-network-federation.md),
> [`docs/design/0014-node-discovery-and-sharding.md`](../../docs/design/0014-node-discovery-and-sharding.md),
> or their operator-facing summaries under
> [`deploy/public-network/`](../../deploy/public-network/).

## 1. Why this integration matters

The original elections design (`README.md`, `architecture.md`)
specifies the election semantics: quid types, trust edges as
votes, blind-signature ballot issuance, paper-ballot parity,
universal recount. It's complete at the "what events get
signed" layer.

What it doesn't specify — because the relevant QDPs landed
later — is *how the system is deployed, governed, and
discovered*:

- **Who exactly are the governors for the election's domains?**
  The original doc has "Guardian set" for the authority quid but
  uses pre-QDP-0012 vocabulary that conflates guardians with
  governance quorum.
- **What runs the blocks, and at what scale?** Precinct-level
  polling places, county-wide counting, state-wide
  certification: this is a sharding question the original doc
  hand-waves.
- **How do federal elections federate across state-run
  infrastructure?** Pre-QDP-0013, the original doc implicitly
  assumed one giant network; realistically every jurisdiction
  runs its own.
- **How do voters' clients find the right nodes for their
  precinct?** The original doc assumes a central `api.quidnug`
  endpoint; per QDP-0014 this should be explicit.

This document fills those gaps.

## 2. QDP-0012 Domain Governance — the election authority as a governor

### 2.1 The mental model update

In the original doc, the "Election Authority Quid" is
described as a quid with a guardian set. Under QDP-0012 we
separate three roles more precisely:

| Role | Who, for elections |
|---|---|
| **Governor** | The humans / institutions authorized to vote on changes to the election's domain consortium + parameters. Typically: chief election official, state SoS office, bipartisan oversight board, independent observers. |
| **Consortium member (validator)** | The physical nodes that produce blocks for the election's domains. Typically operated by the authority + observer organizations, in geographically diverse locations. |
| **Cache replica** | Every precinct polling-place device, every observer's laptop, every journalist's monitor — any node that mirrors the agreed chain for reading but doesn't produce blocks. |
| **Guardian** | (Orthogonal to governance.) The specific quorum that can recover a compromised governor's key via QDP-0002. |

Any one human might hold two or three of these hats. The
chief election official is typically a governor AND operates
one of the consortium-member nodes. But the roles are
independently defined.

### 2.2 Governance structure for a county election

For `elections.williamson-county-tx.2026-nov`:

```
Domain: elections.williamson-county-tx.2026-nov
  governors:
    county-clerk-quid                  weight=2
    state-sos-tx-quid                  weight=2
    r-party-observer-quid              weight=1
    d-party-observer-quid              weight=1
    lwv-observer-quid                  weight=1   # League of Women Voters
  governanceQuorum: 0.7      # 5 of 7 weighted votes
  notice period: 72 hours     # longer than reviews; election-critical

  consortium (validators):
    node-authority-primary             weight=1
    node-authority-backup              weight=1
    node-observer-r                    weight=1
    node-observer-d                    weight=1
    node-observer-lwv                  weight=1
  validatorTrustThreshold: 0.67  # 3-of-5 blocks accepted

  cache replicas:
    precinct-001-polling-place
    precinct-002-polling-place
    ...
    precinct-423-polling-place
    (~423 precincts, each running a precinct-local cache-only node)
    (plus any observer / journalist / candidate running a mirror)
```

Five block-producing consortium members, three of which must
agree (2/3 quorum) for a block to reach `Trusted` on any
observer. Seven governors with weighted votes; 5-of-7 needed
to mutate anything (add a validator, remove one, etc.).

Every governor has a separate guardian quorum (QDP-0002). The
chief election official's guardian quorum might be: spouse,
business partner, state ethics officer, federal monitor,
lawyer. Losing the chief's key doesn't lose the election's
authority — guardians can rotate.

### 2.3 Governance actions during an election cycle

With QDP-0012's transactions, these become first-class, on-chain
events:

| Action | Governance transaction | Notice |
|---|---|---|
| Install a new validator (e.g. add a third observer org mid-cycle) | `ADD_VALIDATOR` | 72h |
| Remove a compromised validator | `REMOVE_VALIDATOR` | 72h (but see emergency path below) |
| Change the blocking threshold | `SET_TRUST_THRESHOLD` | 72h |
| Add a new precinct domain | child domain registration + `DELEGATE_CHILD` | 72h |
| Change the oversight composition (replace an observer) | `UPDATE_GOVERNORS` | 7 days, unanimous |
| Emergency: suspected compromised node | `REMOVE_VALIDATOR` with expedited notice | 1 hour by pre-negotiated emergency clause |

**Emergency clause.** For the week of the election, a
pre-negotiated emergency governance track allows 1-hour notice
periods on `REMOVE_VALIDATOR` actions signed by a full quorum
(5-of-7 weighted). This lets the oversight board remove a
compromised node the same day a breach is detected, without
waiting three days. The emergency clause itself is installed
via a standard 72h `SET_EMERGENCY_WINDOW` governance action
before election day.

### 2.4 Domain tree as governance hierarchy

Each sub-domain of the election can have its own governance
scope or inherit from the parent:

```
elections.williamson-county-tx.2026-nov             (governed by
                                                    the root
                                                    county-level
                                                    quorum)
  ├── registration                                  (inherits)
  ├── poll-book.precinct-001                        (inherits, or
  │                                                  delegated to
  │                                                  the precinct
  │                                                  judges for
  │                                                  operational
  │                                                  signing?)
  ├── ballot-issuance                               (inherits)
  ├── contests.us-senate                            (inherits)
  ├── contests.governor                             (inherits)
  └── audit                                         (inherits,
                                                    possibly
                                                    delegated to
                                                    post-election
                                                    audit firm)
```

For most sub-domains, inherit-from-parent is correct — the
county authority governs everything uniformly. For audit, a
post-election delegation to an independent auditor (via
`DELEGATE_CHILD`) lets the auditor publish findings on-chain
with their own quorum, which is stronger proof than having the
county authority publish them.

## 3. QDP-0013 Network Federation — multi-jurisdiction elections

### 3.1 The problem without federation

In the US, elections are administered at the county level, but
contests span multiple levels:

- **County** races (e.g. county commissioner) — county authority
  runs the show.
- **State** races (governor, US senator) — state-wide tally is
  the sum of county-level results.
- **Federal** races (US president) — multi-state aggregation
  with Electoral College complications.

If each county runs its own Quidnug network (which is the
natural deployment), how does the state get a view across all
counties? How does a federal election observer verify totals
without trusting every county's infrastructure individually?

### 3.2 The federation topology

Each jurisdiction runs its own network. Parent jurisdictions
(state, federal) federate trust-lookups against the child
jurisdictions' networks:

```
                  ┌──────────────────────────┐
                  │  FEDERAL NETWORK         │
                  │  (elections.us.2028-pres)│
                  │  federates with:         │
                  │   - all 50 state nets    │
                  │   - reputation via       │
                  │     `TRUST_IMPORT`       │
                  └─────────────┬────────────┘
                                │ federates
                    ┌───────────┼───────────┐
                    ▼                       ▼
         ┌─────────────────┐      ┌─────────────────┐
         │ STATE NET (TX)  │      │ STATE NET (CA)  │
         │ state SoS       │      │ state SoS       │
         │ governs         │      │ governs         │
         │ federates with: │      │ federates with: │
         │  - 254 counties │      │  - 58 counties  │
         └────────┬────────┘      └────────┬────────┘
                  │ federates             │
            ┌─────┼─────┐            ┌────┼────┐
            ▼           ▼            ▼        ▼
     ┌──────────┐ ┌──────────┐  ┌─────────┐┌─────────┐
     │ COUNTY   │ │ COUNTY   │  │ COUNTY  ││ COUNTY  │
     │ Williams │ │ Travis   │  │ LA      ││ SF      │
     │ own net  │ │ own net  │  │ own net ││ own net │
     │ own      │ │ own      │  │ own     ││ own     │
     │ conso.   │ │ conso.   │  │ conso.  ││ conso.  │
     └──────────┘ └──────────┘  └─────────┘└─────────┘
```

Each level operates its own Quidnug network — own consortium,
own governors, own chain. Parent levels federate to their
children for cross-jurisdictional reads.

### 3.3 What federation actually means for elections

Three distinct federation scenarios:

**Scenario A: State publishes per-county totals at tally time.**
The state SoS network queries each county's network at
certification time, importing the signed tally events into the
state-level chain via `TRUST_IMPORT`. Any observer can verify:

1. The state-level tally event has M-of-N state governor
   signatures.
2. Each imported county tally has its originating county's
   consortium signatures.
3. The import chain is intact (no county's tally was dropped
   or altered at import time).

No observer has to trust the state's claim about a county's
total; they can fetch the county's own signed tally directly
and compare.

**Scenario B: Voter registered in one county, voting in another.**
A voter registers in County A, then moves to County B before
the election. County B needs to verify their registration.
Instead of a faxed form, County B's network configures County
A's network as an `external_trust_source` in the
`elections.us.<year>.registration` domain. When County B's
ballot-issuance flow needs to verify a voter's registration,
it can fetch the registration record directly from County A
with cryptographic proof.

This replaces a currently-fragile state DMV "motor voter"
interstate data-exchange system with a signed, auditable
cross-network lookup.

**Scenario C: Federal observer runs a national recount.**
An NGO or news organization wants to compile a national
tally. They run a single node that federates with all 50
state networks (or, via state → county transitivity, all
~3000 county networks). Their node's discovery API answers
"what is the total vote count for president, from each
state's chain, aggregated?" All cryptographic, all signed by
the originating jurisdictions, all runnable from anyone's
laptop.

### 3.4 Cross-jurisdictional reputation fungibility

Beyond tallies, federation enables reputation-carrying for
election officials + observer organizations:

- A certified election monitor (League of Women Voters, etc.)
  has trust edges from multiple state authorities in their
  respective `observers.elections.*` domains. Federal-election
  observation trusts the union of these.
- A precinct election judge with years of clean operation
  (no disputed close-outs, no irregularity reports) accumulates
  reputation in their county's domain. When they move to
  another county, their reputation travels via `TRUST_IMPORT`.
- Post-election auditing firms accumulate multi-jurisdictional
  reputation the same way. Hiring an auditor becomes a
  reputation-weighted decision, not just a procurement
  checklist.

None of this requires a new primitive — it's QDP-0013's
federation mechanisms applied to the election domain.

## 4. QDP-0014 Node Discovery + Sharding — operational topology

### 4.1 The operator-to-nodes hierarchy

Per QDP-0014, the election authority is an operator quid with
multiple node quids under it. For a county election:

```
county-clerk-quid (operator)
    ──TRUST 1.0──►  node-authority-primary
                    (validator, archive; runs in county data center)
    ──TRUST 1.0──►  node-authority-backup
                    (validator, archive; runs in a different facility)
    ──TRUST 1.0──►  node-certification
                    (cache, archive; post-election audit-focused)
    in operators.elections.williamson-county-tx.2026-nov

r-party-observer-quid (operator)
    ──TRUST 1.0──►  node-observer-r
    in operators.elections.williamson-county-tx.2026-nov

... (similarly for D, LWV, SoS)

county-clerk-quid (via precinct judges' sub-delegation)
    ──TRUST 1.0──►  precinct-001-polling-device
                    (cache only)
    ──TRUST 1.0──►  precinct-002-polling-device
                    ...
    in operators.elections.williamson-county-tx.2026-nov.precincts
```

At election-day scale for a medium county: ~5 validator nodes
(the consortium), ~10 dedicated cache nodes (for fast
check-in queries), ~500 precinct-device cache replicas, plus
an unknown number of observer / journalist nodes.

### 4.2 Node advertisements

Each node publishes a signed `NODE_ADVERTISEMENT` declaring:

- **Endpoints** — where it's reachable.
- **Supported domains** — which parts of the tree this node
  serves (`registration`, `poll-book.precinct-042`, etc.).
- **Capabilities** — validator, cache, archive, bootstrap,
  gossip-sink.
- **Expiration** — short TTL (6 hours at election launch; 1
  hour during the active voting day so dead nodes age out
  fast).

Precinct devices publish "cache-only" advertisements scoped
to their own precinct's poll-book + ballot-issuance domains.
The authority's primary validators publish "validator +
archive" advertisements scoped to the full tree.

### 4.3 Discovery for voters + poll workers + observers

The voter's phone app or the poll worker's device uses the
QDP-0014 discovery API:

```
# Poll worker checks in a voter at precinct 042:
GET /api/v2/discovery/domain/elections.williamson-county-tx.2026-nov.poll-book.precinct-042

Returns:
    - block tip for this precinct's domain
    - consortium members authorized to serve this domain
    - cache replicas currently reachable
    - endpoints, sorted by region (local precinct first)

Poll worker's device picks the local precinct cache (lowest
latency) for the actual check-in query.
```

Observers run their own nodes and hit `api.<authority>.gov`
(the authority's api gateway). The gateway routes to
appropriate backend nodes via the same discovery logic — they
don't need to know physical topology.

### 4.4 Sharding patterns for elections

Combinations:

- **Geographic** — nodes deployed in every precinct + county
  seat + state-level backup.
- **Domain-tree** — heavy-traffic domains (poll-book during
  voting hours, ballot-issuance, contest streams) get more
  cache nodes; lightly-touched domains (certification, audit)
  get fewer.
- **Capability** — validators are few (5-9 per jurisdiction);
  cache nodes scale horizontally; archive nodes are 2-3 per
  jurisdiction for post-election audit.
- **Network federation** — some state nodes also act as
  federation bridges to county networks; some federal nodes
  federate to all 50 states.

Example sharding for election day at a major county:

```
node-authority-primary      validator+archive,  region=data-center-1
node-authority-backup       validator+archive,  region=data-center-2
node-observer-r             validator,           region=party-r-office
node-observer-d             validator,           region=party-d-office
node-observer-lwv           validator,           region=neutral-facility
[above = the five-member consortium]

node-cache-east-1..3        cache,               region=east-county
node-cache-west-1..3        cache,               region=west-county
node-cache-central-1..3     cache,               region=central-county
[above = dedicated cache nodes for heavy read traffic]

node-archive-1              archive,             region=state-archive
node-archive-2              archive,             region=federal-archive
[above = post-election audit sources]

precinct-001..423-device    cache,               region=precinct-local
                            supportedDomains=[poll-book.<their-precinct>]
[above = hundreds of precinct-level cache replicas, each only
 serving its own precinct's poll-book lookups]
```

The QDP-0014 discovery API routes each client query to the
closest + most-capable-matching node automatically.

### 4.5 No-node participation for voters

A voter doesn't run a node. They hold:

- Their VRQ private key (on their phone, in a hardware token,
  or on paper).
- Their BQ private key (ephemeral; generated per ballot).

Their client app:

1. Fetches the well-known election file from the authority's
   HTTPS URL.
2. Uses the discovery API to find a reachable cache node.
3. Signs transactions (registration, ballot request, vote)
   locally.
4. Submits via the api gateway.
5. Reads back own records for verification (individual
   verifiability).

This is exactly QDP-0014's "lightweight participation"
pattern (§14 of QDP-0014). Voters get full cryptographic
participation with zero infrastructure. An election with 10
million voters doesn't require 10 million nodes — it requires
1 authority + ~500 precinct caches + 5-9 validator
consortium members.

### 4.6 Per-domain quid index for precinct lookups

The QDP-0014 per-domain quid index is load-bearing for one
specific election flow: "is this voter registered in this
precinct?"

```
GET /api/v2/discovery/quids
    ?domain=elections.williamson-county-tx.2026-nov.registration
    &since=<last-registration-deadline>
    &sort=first-seen
    &limit=500
    &offset=0
```

Returns: every quid with a `VOTER_REGISTERED` event in the
domain within the window. The poll-book device paginates
through to build its local precinct roll at polling-place
opening.

Alternatively, a precinct's poll book is scoped to a child
domain (`poll-book.precinct-042`), and the registration
authority issues `REGISTERED_IN_PRECINCT` events to that
domain, making per-precinct lookups trivial.

## 5. Well-known discovery for elections

Every election authority publishes a
`.well-known/quidnug-network.json` file (per QDP-0014 §7)
that serves as the cold-start entry point for all
participants.

**Example URL for a county election:**

```
https://elections.williamson-county-tx.gov/.well-known/quidnug-network.json
```

**Contents** (abbreviated):

```json
{
    "version": 1,
    "operator": {
        "quid": "<county-clerk-quid>",
        "name": "Williamson County Clerk",
        "publicKey": "04..."
    },
    "apiGateway": "https://api.elections.williamson-county-tx.gov",
    "seeds": [
        { "nodeQuid": "...", "url": "https://node1.elections.wilco.gov", "capabilities": ["validator", "archive"] },
        { "nodeQuid": "...", "url": "https://observer-r.elections.wilco.gov", "capabilities": ["validator"] },
        { "nodeQuid": "...", "url": "https://observer-d.elections.wilco.gov", "capabilities": ["validator"] },
        { "nodeQuid": "...", "url": "https://observer-lwv.elections.wilco.gov", "capabilities": ["validator"] }
    ],
    "domains": [
        { "name": "elections.williamson-county-tx.2026-nov", "description": "November 2026 general election", "tree": "elections.williamson-county-tx.2026-nov.*" }
    ],
    "governance": {
        "documentedAt": "https://elections.williamson-county-tx.gov/governance",
        "governors": [
            { "quid": "...", "name": "County Clerk", "publicKey": "04...", "weight": 2 },
            { "quid": "...", "name": "State SoS TX", "publicKey": "04...", "weight": 2 },
            { "quid": "...", "name": "R Party Observer", "publicKey": "04...", "weight": 1 },
            { "quid": "...", "name": "D Party Observer", "publicKey": "04...", "weight": 1 },
            { "quid": "...", "name": "LWV Observer", "publicKey": "04...", "weight": 1 }
        ],
        "quorum": 0.7
    },
    "federationAvailable": true,
    "lastUpdated": 1748000000,
    "signature": "..."
}
```

Voter apps, observer software, and federated state networks
all fetch this file first, pin the operator pubkey, and use
it to verify everything they see afterwards.

The file is also mirrored publicly on the
`elections.williamson-county-tx.gov` website's landing page
so anyone can verify the declared governors + consortium
before trusting the election output.

## 6. Federated-network scenario: US presidential election

Putting all three pillars together for the hardest case.

### 6.1 Network topology

- **Federal network** (`elections.us.2028-pres`) — governed
  by a narrow consortium (FEC? bipartisan federal electors
  board? — policy question, not technical). Light; mostly
  contains an `imports` sub-tree that pulls per-state
  certifications.
- **50 state networks** — each governed by its SoS +
  legislative oversight + federal monitor. Each state-level
  network contains aggregated per-county totals.
- **~3000 county networks** — each governed by the county
  election authority as described above.

### 6.2 Data flow, election night

```
23:00 EST  counties begin closing polls
23:30 EST  county authorities publish signed TALLY events
           on their own chains
00:00 EST  state-level networks run scheduled federated
           aggregation jobs:
            - fetch each county's latest TALLY event via
              the federation API
            - verify signatures against the county's
              governor pubkeys (pinned from each county's
              well-known file)
            - aggregate and sign a STATE_AGGREGATION event
              on the state's own chain
01:00 EST  federal-level network aggregates per-state:
            - federated fetch of each state's
              STATE_AGGREGATION event
            - verify + sum + emit FEDERAL_TALLY event
           (this is not "official certification" — that
           follows Electoral-College + canvassing rules.
           it's the auditable cryptographic aggregation.)
```

At every level, the aggregated output carries cryptographic
proof of every input it incorporates. Anyone with a laptop
can redo the federation walk and confirm the aggregation is
correct. **The chain IS the audit trail** — the post-election
audit becomes a replay of the chain, not a parallel manual
verification exercise.

### 6.3 Recount at federal level

"Candidate X contests the presidential result."

1. Candidate X's lawyer runs the aggregation from a cold
   laptop:
   ```
   quidnug-cli elections aggregate-federated \
       --root https://elections.us.gov/.well-known/quidnug-network.json \
       --year 2028 \
       --contest us-president \
       --verify-signatures
   ```
2. The CLI federates across all 50 state networks, fetches
   all county tallies, verifies every signature, and prints:
   - Total per candidate.
   - State-level breakdowns.
   - Any county whose tally signature fails verification
     (red-flagged for manual review).
3. If the candidate disputes a specific county's tally:
   ```
   quidnug-cli elections county-detail \
       --county williamson-county-tx \
       --year 2028-nov \
       --contest us-president
   ```
   Returns the full per-ballot vote edge list for the
   contest, cryptographically tied to ballot issuances,
   which are tied to voter registrations. Every vote is
   independently verifiable from primary data.
4. No waiting for the county to re-scan ballots. No vendor
   software rerunning on the same counts. The recount is
   an O(n) read of the public chain.

Compare to 2020 Arizona, where the Cyber Ninjas ballot audit
took months and cost millions. Under this design, the same
audit is a few hours of laptop time and costs the
electricity to run it.

## 7. Governance changes mid-cycle

Elections don't happen on schedule; unexpected events force
governance changes. Three scenarios the original design didn't
formalize:

### 7.1 A governor is unavailable on election day

A governor has a heart attack, loses their phone, gets
arrested — doesn't matter. Their key is unavailable.

**Quorum math matters:** the election's 5-of-7 quorum still
functions with 6 available governors. One missing governor
can't block actions. If TWO become unavailable simultaneously,
the remaining 5 can still act.

**Guardian recovery** (QDP-0002) for a lost key takes ~24h at
minimum. For election-day issues, this isn't fast enough —
but because the quorum is resilient, it doesn't need to be.

### 7.2 A consortium node is compromised election night

An observer's node is detected publishing inconsistent blocks.

1. Election-night IR team confirms compromise.
2. Governor quorum signs `REMOVE_VALIDATOR` with the
   pre-negotiated 1h emergency-notice clause.
3. In 1 hour, the validator is removed; the remaining 4 nodes
   continue at 3-of-4 threshold.
4. Post-incident: the `REMOVE_VALIDATOR` tx + incident-report
   event on the audit domain give a complete public record.

The 1-hour window isn't zero, but a compromised single node
out of 5 can't unilaterally produce accepted blocks anyway —
the 3-of-5 threshold means it needs collusion. The emergency
removal is about cleaning up, not stopping in-progress harm.

### 7.3 The whole consortium is compromised

Extremely unlikely (would require ~all 5 node operators
independently compromised) but design for it.

**The answer is federation (QDP-0013).** If the county's
consortium is untrustworthy, the state SoS network refuses to
federate with that county's results. The county's chain is
still there, still verifiable, but loses its legitimacy at
the aggregation layer. The state can run an emergency
paper-ballot audit at the county level using the county's own
paper ballots (QDP-0013 doesn't require the county's
electronic consortium to cooperate — the paper ballots are in
a physical lockbox the state has access to).

This is why paper-ballot parity is non-negotiable: it's the
fallback when the digital consortium is worst-case
compromised. The cryptographic design makes that fallback
almost never needed, but it's there.

## 8. Operational readings

For actually running an election on this architecture:

- [`operations.md`](operations.md) — deployment topology,
  capacity planning, election-day operations playbook,
  incident response. How to stand up the authority + observer
  node infrastructure at any scale.
- [`launch-checklist.md`](launch-checklist.md) — T-90 through
  T+30 sequential steps. The go-live equivalent for elections
  (parallels
  [`deploy/public-network/reviews-launch-checklist.md`](../../deploy/public-network/reviews-launch-checklist.md)).

For understanding the protocol layer referenced above:

- [`deploy/public-network/governance-model.md`](../../deploy/public-network/governance-model.md)
  — operator-facing QDP-0012 explainer.
- [`deploy/public-network/federation-model.md`](../../deploy/public-network/federation-model.md)
  — operator-facing QDP-0013 explainer.
- [`deploy/public-network/sharding-model.md`](../../deploy/public-network/sharding-model.md)
  — operator-facing QDP-0014 explainer.

## 9. What this integration changes in the existing design

A handful of specific corrections to the pre-QDP-0012 text in
[`README.md`](README.md) and [`architecture.md`](architecture.md):

1. **"Guardian set" vs "governor quorum."** The README §1
   "Election Authority Quid" section describes a
   `requireGuardianRotation: true` model. Under QDP-0012,
   guardians (QDP-0002 recovery) and governors (QDP-0012
   voting) are separate. Each governor has their own
   guardian quorum; the election authority as a whole has a
   governor quorum.

2. **Domain validators.** README §domain-hierarchy says "Each
   domain has its own validators (the election authority +
   observers)." Under QDP-0012, this means the consortium
   members for each domain — who must be added via
   `ADD_VALIDATOR` governance transactions, not just declared
   ad-hoc.

3. **No mention of cache replicas.** The existing design
   implicitly assumes every precinct device is a full-fledged
   node. With QDP-0012 + QDP-0014, precinct devices are
   cache replicas (read-only, much cheaper to deploy). This
   is a material cost reduction for large-county deployments.

4. **No federation story.** The existing design is
   single-jurisdiction. Real elections span jurisdictions.
   §3 above fills this gap.

5. **Discovery is implicit.** The existing design says "hit
   the api gateway." QDP-0014 makes this discoverable + signed
   + cache-friendly + shardable.

These corrections don't invalidate the existing design — they
layer on top, making the deployment story complete. Treat this
integration document as the authoritative ops-layer companion
to the semantics layer in README.md + architecture.md.
