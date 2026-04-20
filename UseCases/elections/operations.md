# Elections operations

> Deployment topology, capacity planning, election-day
> operations, and incident response for running a Quidnug-
> based election at any scale from precinct-only pilot to
> federated presidential general.
>
> Read
> [`README.md`](README.md) for the election semantics,
> [`architecture.md`](architecture.md) for the data model,
> [`integration.md`](integration.md) for the architectural-
> pillar integration. This document assumes you've internalized
> those.
>
> Operationally this parallels
> [`deploy/public-network/home-operator-plan.md`](../../deploy/public-network/home-operator-plan.md)
> but for elections instead of reviews — same protocol,
> different deployment profile.

## 1. The deployment-scale matrix

Pick the row that matches your election:

| Scale | Example | Voters | Nodes | Monthly cost | Complexity |
|---|---|---|---|---|---|
| Pilot | Student body election, 1 precinct | < 1k | 2 (validator + backup) | < $50 | Single-day launch |
| Small municipal | Town council, 5 precincts | 1-10k | 4 (consortium + observer + 2 caches) | $100-300 | 1-2 week launch |
| County | Williamson County TX example | 100k-1M | 10-20 (5 consortium + cache + precinct devices) | $500-2k | 3-6 month launch |
| State | Texas SoS statewide | 10M+ | 30-100 (state consortium + federated to counties) | $5k-20k | 1-2 year launch |
| Federal | US presidential | 150M+ | 200+ (federal consortium + federated to 50 states + 3k counties) | $50k-250k | 2-4 year launch |

The protocol is the same at every scale; only topology and
operational rigor change. This document gives you a
capacity-planning and deployment recipe for each.

## 2. Deployment architecture per scale

### 2.1 Pilot (student body, HOA, office election)

Simplest possible deployment:

```
┌─────────────────────┐
│  Your laptop (WSL)  │
│  ┌───────────────┐  │
│  │ quidnug node  │  │  port :8087
│  │ (validator +  │  │
│  │  archive +    │  │
│  │  cache)       │  │
│  └───────────────┘  │
└──────────┬──────────┘
           │ cloudflared tunnel
           ▼
    https://vote.your-org.example
           │
           │ voters hit this URL
           │ from their phones
           ▼
    [voter phones / browsers]
```

One node. One operator quid (you). Maybe one observer key on
a friend's laptop for two-person control. No physical
infrastructure beyond a laptop and a Cloudflare tunnel.

Governance: 1-of-1 governor (you). Consortium: your node.
Cache: your node. Everyone connects direct to your URL.

**Launch time:** an afternoon. Follows the home-operator-plan
exactly, with "elections" swapped for "reviews."

### 2.2 Small municipal (town council, school board)

Three nodes, two physical sites:

```
┌──────────────────────┐   ┌──────────────────────┐
│  Town clerk machine  │   │  Observer laptop     │
│  (data center or     │   │  (town hall WAN      │
│   home broadband     │   │   or LTE)            │
│   via CF tunnel)     │   │                      │
│  validator+archive   │   │  validator           │
└──────────┬───────────┘   └──────────┬───────────┘
           │                          │
           │       gossip             │
           └──────────────────────────┘
                       │
                       │
           ┌───────────▼───────────┐
           │  Cache-only VPS       │
           │  ($5/mo on Oracle     │
           │   Free Tier)           │
           │  cache+bootstrap      │
           └───────────────────────┘
                       │
                       │ discovery api
                       ▼
                 voters + poll workers
```

Three-node consortium, 2-of-3 quorum (so either the clerk or
an observer + the cache VPS can produce a block; the observer
alone can't, preserving check-and-balance).

Per-precinct check-in: each precinct's polling place has a
laptop or iPad running a cache-only profile that syncs from
the VPS. Operates in polling-place LAN mode during the day,
syncs to the VPS at regular intervals. At polls close,
precinct device publishes `POLLS_CLOSED` events.

Launch time: 1-2 weeks to register domains, publish
governance, brief poll workers, test.

### 2.3 County (100k-1M voters)

This is where it starts to look like infrastructure:

```
       ┌──────────────────────────────────────────────┐
       │ Authority + Observer validator consortium    │
       │ (5 nodes, 3-of-5 quorum)                     │
       │ - authority-primary  (gov data center 1)     │
       │ - authority-backup   (gov data center 2)     │
       │ - observer-r         (R party HQ)            │
       │ - observer-d         (D party HQ)            │
       │ - observer-lwv       (LWV office)            │
       └──────────────────────┬───────────────────────┘
                              │ gossip
       ┌──────────────────────┴───────────────────────┐
       │ Dedicated cache tier (10 nodes in 3 regions) │
       │ - cache-east-1..3   (east county data)       │
       │ - cache-west-1..3   (west county data)       │
       │ - cache-central-1..4                          │
       └──────────────────────┬───────────────────────┘
                              │ api gateway
                              ▼
                    api.elections.<county>.gov
                              │
       ┌──────────────────────┼───────────────────────┐
       ▼                      ▼                       ▼
precinct-001 device    precinct-002 device ... precinct-423
  (cache, scoped         (cache, scoped         (cache, scoped
   to precinct-001)       to precinct-002)       to precinct-423)
```

~15 dedicated nodes + ~423 precinct devices. The precinct
devices are cheap (iPad or small laptop); the dedicated
tier uses proper server hardware.

Dedicated cache tier is a new pattern for this scale. They
serve nearly all read traffic (poll-book lookups,
ballot-issuance verification, observer queries). The
consortium validators only handle writes (registrations,
ballots cast, tally events). This separation prevents
election-day read traffic from overwhelming the
block-producing nodes.

**Launch time:** 3-6 months of planning + deploy + test +
training + dry-run + legal sign-off + poll-worker cert.

### 2.4 State (10M+ voters)

```
       ┌─────────────────────────────────────────────┐
       │ State consortium (9 nodes, 5-of-9 quorum)   │
       │ - state SoS primary + backup + 2 DRs        │
       │ - state legislature observer (2 seats)      │
       │ - federal monitor observer (1 seat)         │
       │ - civil-society observer (2 seats: LWV + similar)│
       └──────────────────────┬──────────────────────┘
                              │
                              │ federates to
                              ▼
       ┌─────────────────────────────────────────────┐
       │ 254 county networks (one per TX county)    │
       │ each with its own consortium                │
       └─────────────────────────────────────────────┘
                              │
                              │ state cache-gateway
                              ▼
       ┌─────────────────────────────────────────────┐
       │ State-level discovery gateway + cache tier  │
       │ (20-40 cache nodes in 4-5 regions of TX)    │
       └─────────────────────────────────────────────┘
                              │
                              │ api.elections.tx.gov
                              ▼
       observers / journalists / candidates statewide
```

State consortium produces aggregation events. County networks
produce per-county tallies. State federates down to
counties via QDP-0013. State-level cache tier serves state-
wide queries (state-level candidate totals, statewide prop
counts).

**Launch time:** 1-2 years. State-wide rollout is a
multi-year project involving every county's adoption + new
legal framework + vendor transition.

### 2.5 Federal (US presidential general)

Three-tier federated network of networks (see
[`integration.md`](integration.md) §3.2 for the diagram).
Federal consortium is deliberately minimal — its only job is
aggregation, so 3-5 nodes with 2-of-3 quorum is enough. Most
of the federal network's operational weight is in the
federation configuration: pinning 50 state networks' operator
pubkeys, maintaining well-known file mirrors, running
aggregation jobs.

## 3. Hardware specification

By role.

### 3.1 Validator node (consortium member)

Produces blocks. Modest load but critical uptime.

| Requirement | Minimum | Recommended |
|---|---|---|
| CPU | 2 cores | 4 cores |
| RAM | 4 GB | 8 GB |
| Disk | 50 GB SSD | 200 GB NVMe |
| Disk IOPS | 2k | 10k |
| Network | 100 Mbps | 1 Gbps |
| Uptime SLO | 99.5% | 99.9% |

At county scale: a single Hetzner CX41 ($10/mo) comfortably
handles the validator role. For state scale: a dedicated VM
with burstable CPU. For federal: a small physical server in a
certified data center.

### 3.2 Cache node

Scales with read volume. Easy to horizontally add more.

| Requirement | Small county | Large county |
|---|---|---|
| CPU | 2 cores | 8 cores |
| RAM | 4 GB | 16 GB |
| Disk | 100 GB SSD | 500 GB SSD |
| Network | 100 Mbps | 1 Gbps |
| Uptime SLO | 99% (redundant; one down is fine) | 99.5% |

Target: 5k-20k read QPS per cache node. If election-day
peak exceeds that, add more cache nodes (QDP-0014
discovery auto-routes).

### 3.3 Archive node

Stores full history. Rarely queried in real-time; used for
post-election audit + forensics.

| Requirement | Per year's elections |
|---|---|
| CPU | 2 cores |
| RAM | 8 GB |
| Disk | 1-4 TB (post-election data can be moved to cold storage) |
| Network | 10 Mbps (low real-time; high for audit days) |
| Uptime SLO | 99% |

At least 2 archive nodes per jurisdiction, geographically
diverse, ideally hosted by different governance entities
(one by the state, one by an observer org).

### 3.4 Precinct device (cache-only)

The thing a poll worker hands a voter.

| Requirement | Value |
|---|---|
| CPU | Any reasonable phone / tablet / laptop CPU |
| RAM | 2 GB |
| Disk | 8 GB (of which ~1 GB is the precinct's local cache) |
| Battery | 8+ hours (polling-place day) |
| Network | Primary: LTE or precinct wifi; fallback: none (offline operation for 4h) |
| Supported hardware | iPad Pro (recommended), Surface Go, Chromebook, Linux laptop |
| Security | Secure enclave for storing the precinct device's node key |

Critical: must degrade gracefully when network is out.
Precinct device runs as a cache node with a large local cache
so offline queries (check-in verification) still work. Votes
cast offline queue locally + sync on reconnection with full
cryptographic integrity.

## 4. Capacity planning

### 4.1 Transaction volume estimates

For a county-scale election with 500k registered voters:

| Phase | Events | Duration | Rate |
|---|---|---|---|
| Pre-election (registration period, 90 days) | 500k | 90 days | ~6k/day = ~0.07/s |
| Early voting (14 days) | 100k-150k | 14 days | ~10k/day = ~0.12/s |
| Election day | 350k | 12 hours | ~30k/hour = ~8/s |
| Peak hour (noon lunch rush) | 75k in 1 hour | 1 hour | 21/s |
| Tally + certification | ~100 | 1-7 days | negligible |

8/s sustained, 21/s peak. The node's default rate limit at
100k/min (which is what we configured for the reviews demo)
is way more than enough. The real constraint is storage
(500k registrations + 350k ballots + vote events = ~2M events,
~500MB at ~250 bytes/event) and validator throughput for
block production.

At a 60-second block interval, a county election never
generates more than 50 transactions per block during peak
hours — comfortable margin.

### 4.2 Read load estimates

Much higher than write load, because every interaction
generates reads.

| Query | Per voter | Per day (election day) |
|---|---|---|
| Voter checks their own registration | 2-5 | 500k × 3 = 1.5M |
| Poll worker checks in voter | 1 | 350k |
| Voter checks their own vote (individual verifiability) | 1-3 | 350k × 2 = 700k |
| Observer queries | 10k-100k per observer | varies |
| Journalist / candidate queries | 100k-1M total | varies |
| Total | | 3M-5M |

3-5 million reads per day on election day. Distributed
across 10 cache nodes: 300-500k per node per day, or 3-6 QPS
per node. Trivial. CDN edge caching of idempotent GETs
further reduces origin load.

### 4.3 Spike handling

Traffic isn't uniform. Expect:

- **Opening of polls (6-7am):** ~5x normal read load for 30
  minutes as poll workers come online.
- **Lunch rush (11am-1pm):** ~3x normal voter traffic.
- **Pre-close (6-7pm):** ~4x normal traffic as commuters
  arrive after work.
- **Post-close (7-10pm):** ~10x normal observer / journalist
  traffic as tallies publish.

Cloudflare edge cache handles most of this. Origin load
stays reasonable. Rate-limiting on the node side is set to
allow 10x normal volume so bursts don't cause legitimate
requests to fail.

### 4.4 Storage growth

Per voter per election:
- 1 registration event: ~500 bytes
- 1 ballot issuance: ~500 bytes (includes blind signature)
- 5-20 vote edges (depending on contests): ~500 bytes each = 2.5k-10k
- 1-3 verification events: ~300 bytes each = ~1k

Per voter per election: ~4-12 KB on-chain.

County of 500k voters, full election: ~2-6 GB of on-chain
data. Over 10 years of elections: ~20-60 GB per county.
Archive nodes handle this easily. Cache nodes keep only
hot state (current + recent elections) which is much
smaller.

## 5. Election-day operations playbook

### 5.1 T-30 minutes: final check

```bash
# Health check every consortium member
for node in authority-primary authority-backup observer-r observer-d observer-lwv; do
    curl -fsS "https://$node.elections.<county>.gov/api/health" \
        | jq '{uptime, status, blockTip}'
done

# Confirm consortium in-sync
curl -fsS "https://api.elections.<county>.gov/api/v2/discovery/domain/elections.<county>.2026-nov" \
    | jq '.blockTip'

# Verify precinct devices registered as cache replicas
curl -fsS "https://api.elections.<county>.gov/api/v2/discovery/operator/<county-clerk-quid>" \
    | jq '[.[] | select(.capabilities.cache == true)] | length'
# Expected: ~423 (number of precincts)

# Check audit pipeline
curl -fsS "https://api.elections.<county>.gov/api/streams/<county-quid>/events?eventType=AUDIT_READY" \
    | jq '.data | length'
```

If any of these fail: delay poll opening if the failure is
consortium-level; handle locally if it's a single precinct.

### 5.2 Polls open (typically 7am local)

1. Each precinct's chief judge signs + publishes a
   `POLLS_OPENED` event to their precinct's poll-book domain.
2. Precinct devices start accepting check-ins.
3. Consortium starts producing blocks containing check-ins +
   ballot issuances + cast votes.
4. Observers begin scheduled monitoring queries.
5. Monitoring dashboard shows traffic per precinct, with
   alerts for any precinct that's processing 0 check-ins
   more than 30 minutes after open.

### 5.3 During voting day

Continuous monitoring for:

- **Check-in rate** per precinct. Anomalies (e.g., 10x
  surge) may indicate attempted ballot-stuffing.
- **Ballot-issuance rate** — should equal check-in rate.
- **Vote-cast rate** — should equal ballot-issuance rate
  within an hour (voters finish marking).
- **Block-tier distribution** — every validator's block
  count should be roughly equal. Asymmetry may indicate
  one validator falling behind or failing.
- **Consortium agreement rate** — percentage of blocks
  accepted as `Trusted` by all consortium members. Should
  be 100%; below 99% is a warning.
- **Failed tx rate** — should be near-zero. Spike
  indicates either attack or bug.

Dashboard at `status.elections.<county>.gov` publishes:
- Total registered voters who've voted (turnout live).
- Per-precinct turnout rates.
- Any precinct with alerts.

### 5.4 Polls close (typically 7pm local)

1. Each precinct's chief judge signs a `POLLS_CLOSED` event
   with final local totals + cryptographic hash of paper
   ballots (confirming paper = digital).
2. Consortium waits 30 minutes for straggler tx from
   precincts with poor connectivity to gossip through.
3. Tally computation runs:
   - Sum all vote edges per contest.
   - Emit `CONTEST_TALLY_PRELIMINARY` event per contest,
     signed by the consortium.
4. Publish preliminary results to the status dashboard +
   the official website.

### 5.5 Post-close (7-10pm)

1. Observer teams run their own independent recounts
   (anyone can; see §5.6).
2. Any discrepancies trigger paper-ballot comparison
   (procedure in
   [`implementation.md`](implementation.md) §7).
3. Candidates who dispute results receive fine-grained
   per-ballot drill-down access (still cryptographically
   tied to paper).

### 5.6 T+24 hours: certification

After a 24-hour observer-review window:
1. County canvassing board meets + certifies.
2. Authority publishes `CERTIFIED` event to the tally
   domain with full quorum signatures.
3. Results federate to the state network via `TRUST_IMPORT`.
4. Certified totals become official.

## 6. Incident response

Scenarios + response protocols.

### 6.1 A consortium node goes offline during voting

**Detection:** health check fails, or validator's block
participation rate drops.

**Response:**
1. Remaining consortium continues (3-of-5 quorum preserved
   as long as 4 are up).
2. IR team investigates the failing node; bring back up if
   possible.
3. If recovery > 2 hours, consider emergency
   `REMOVE_VALIDATOR` (via the 1h-emergency-notice clause
   from `integration.md` §7.2).
4. Publish `NODE_DEGRADED` event to the audit domain
   explaining what happened; public transparency reduces
   legitimacy risk.

### 6.2 Precinct device compromised

**Detection:** precinct device publishing invalid events,
duplicate check-ins, or just stopped responding.

**Response:**
1. Chief judge at the precinct switches to paper-only
   check-in (manual poll book).
2. Compromised device is removed; paper check-ins get
   cryptographically bridged at a later point.
3. IR team investigates the compromised device:
   - What was its node key's scope? (Just the precinct's
     poll-book domain, per QDP-0014.)
   - Any tx published under the key are flagged for extra
     verification.
4. Revoke the device's node-quid via an operator-attestation
   TRUST edge update.

Because the precinct device only holds a cache-only key
scoped to one precinct, the blast radius is bounded.

### 6.3 Suspected registration fraud

**Detection:** an observer notices a voter registration that
seems suspicious (address doesn't exist, voter is deceased, etc.).

**Response:**
1. Observer publishes a `REGISTRATION_CHALLENGE` event.
2. Authority investigates within 24h.
3. If fraudulent: publish `REGISTRATION_REVOKED` event
   (stops future ballot issuance).
4. If voter already cast a ballot: flag for paper-ballot
   verification at tally time.
5. Legal referral to prosecutor for follow-up.

The challenge/revocation mechanism is cryptographic +
auditable: no silent changes.

### 6.4 Network partition / DDoS / outage

**Detection:** ISP connectivity fails, DDoS floods the
gateway, Cloudflare incident.

**Response:**
1. Gateway failover to backup DNS entries.
2. Precinct devices operate in local-cache mode; sync
   deferred.
3. If outage exceeds 30 minutes, chief election officer
   extends voting hours per statutory authority; publishes
   `VOTING_HOURS_EXTENDED` event.
4. Post-incident, publish full timeline on audit domain.

## 7. Security discipline

### 7.1 Key custody per role

| Role | Key custody |
|---|---|
| Governor (human) | HSM or YubiKey. Paper backup in a physical vault. QDP-0002 guardian quorum for recovery. |
| Consortium member node | HSM-backed signing on dedicated hardware. Daily health check. |
| Precinct device | Secure-enclave-generated key, locked to the precinct domain. Device returns to sealed storage between elections. |
| Voter VRQ | Voter's own custody (phone secure enclave, YubiKey, paper). |
| Voter BQ | Ephemeral; generated per ballot, discarded after cast. |

### 7.2 Pre-election hardening

- All software audited by an independent firm (publish the
  audit report).
- All nodes on isolated networks during voting hours; no
  outbound traffic except to the consortium + CDN.
- Admin access logs on the audit domain (every console
  login is an on-chain event).
- Disaster-recovery drill ~30 days before election.

### 7.3 Election-day rules

- No governance changes during active voting (except
  emergency `REMOVE_VALIDATOR`).
- No node key rotation during voting.
- No software updates between polls-open and polls-close.
- All governors + consortium operators on-call until
  certification.

## 8. Cost

By scale.

### 8.1 Small municipal (under $500 total)

- 1 VPS @ $6/mo × 12 = $72
- Backup storage @ $1/mo × 12 = $12
- Domain rental = $10/yr
- Total: ~$100/yr

Plus one-time: legal sign-off (~$3k if you're buying it
commercial; $0 if municipal legal already does it).

### 8.2 County (mid-size)

- 5 validators @ $20/mo each = $100/mo = $1200/yr
- 10 cache @ $10/mo = $100/mo = $1200/yr
- 2 archive @ $50/mo = $100/mo = $1200/yr
- Domain + TLS + CDN free tier = $0
- Monitoring (Grafana Cloud free tier) = $0
- Total: ~$3600/yr

Plus one-time: precinct devices (~$500 each × 423 = $211k)
— but these already exist for traditional electronic
poll books, so usually not incremental.

### 8.3 State

- 9 state validators @ $100/mo = $900/mo = $11k/yr
- 40 state caches @ $50/mo = $2000/mo = $24k/yr
- Monitoring paid tier = $5k/yr
- Legal + audit = $100k/yr
- Operations staff (2 FTEs) = $250k/yr
- Total: ~$400k/yr

### 8.4 Federal

- Federal consortium (minimal) = $50k/yr
- Federation gateway + observation = $100k/yr
- Audit + compliance = $500k/yr
- Staff (10 FTEs across oversight) = $2M/yr
- Total: ~$2.5M/yr

Compare to current US election spending: ~$5B / year across
all jurisdictions. The Quidnug deployment replaces
proprietary voting-machine contracts ($500M-$1B / year) and
registration-database licenses ($100M+/year). Net savings
are substantial at any scale.

## 9. Running drills before the real thing

Pre-real-election checklist:

- **T-90 days:** full dry-run with 1000 fake voters across
  10 test precincts. Include a suspected-fraud scenario and
  a consortium-node-failure scenario.
- **T-60 days:** security audit final report + fixes
  deployed.
- **T-30 days:** second dry-run with 10,000 fake voters;
  observer teams run recounts.
- **T-14 days:** final integration test + all poll workers
  certified on the precinct device.
- **T-7 days:** freeze code. No deploys until post-election.
- **T-1 day:** polling-place hardware verified.
- **Day 0:** operate the election.

## 10. What this doc doesn't cover

- **Specific local legal requirements.** These vary by state
  and by country. Work with election counsel.
- **Voter accessibility (ADA, multilingual).** The protocol
  handles signing; the UX/client apps handle accessibility.
  That's a substantial separate design track.
- **Long-term archival policy.** Events live forever on
  chain; but storing + serving 30 years of election data
  is an evolving operations question.
- **Integration with existing SoS workflows.** Every state
  has its own admin software. Bridges are use-case-specific.

For all of these, adapt the pattern. The protocol layer is
the same; the policy and UX layers wrap it.

## 11. References

- [`README.md`](README.md) — election semantics.
- [`architecture.md`](architecture.md) — data model + flows.
- [`integration.md`](integration.md) — QDPs 0012/0013/0014
  integration.
- [`launch-checklist.md`](launch-checklist.md) — T-90 through
  T+30 sequence.
- [`threat-model.md`](threat-model.md) — attack analysis.
- [`deploy/public-network/home-operator-plan.md`](../../deploy/public-network/home-operator-plan.md)
  — parallel operator playbook for the reviews system.
- [`deploy/public-network/reviews-launch-checklist.md`](../../deploy/public-network/reviews-launch-checklist.md)
  — launch-sequence template this doc cribs from.
