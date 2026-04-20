# Interbank wire authorization integration with the three architectural pillars

> How the interbank-wire use case composes on top of the three
> architectural QDPs (0012 Domain Governance, 0013 Network
> Federation, 0014 Node Discovery + Sharding). Companion to
> [`README.md`](README.md) + [`architecture.md`](architecture.md),
> which predate the pillars and describe the wire-authorization
> mechanics without addressing the larger protocol architecture.
>
> Read the pillar operator-facing summaries first:
> [`deploy/public-network/governance-model.md`](../../deploy/public-network/governance-model.md),
> [`deploy/public-network/federation-model.md`](../../deploy/public-network/federation-model.md),
> [`deploy/public-network/sharding-model.md`](../../deploy/public-network/sharding-model.md).

## 1. What's new here vs. elections

The interbank-wire-authorization use case is structurally
similar to elections: coordination-archetype, multi-party
governance, guardian-recoverable keys, auditable event streams.
But the deployment profile is fundamentally different:

| | Elections | Interbank wires |
|---|---|---|
| Number of participating orgs | ~5-50 (authority + observers) | ~2-20 per wire (originator + correspondents + beneficiary bank) |
| Geography | Jurisdiction-bounded | Cross-border and cross-jurisdiction by definition |
| Transaction volume | ~10M per major election | ~150M SWIFT wires per DAY globally |
| Latency requirement | Minutes (tally; daily cycle) | Seconds-to-minutes (SWIFT is already ~minutes; real-time is the goal) |
| Audit timeline | Weeks-to-months post-election | Hours to 7 days (regulatory inquiry windows) |
| Regulatory framework | State election law per-jurisdiction | FATF + Basel + local central-bank rules — every bank in every country |
| Failure cost | Election integrity + political crisis | Tens of millions per missed wire, trust erosion, regulatory fines |

So the integration needs emphasize: **federation** (every bank
is its own network), **discovery** (finding the right node for
a counterparty's transaction at wire time), and **governance**
(changes to signatory rosters need explicit on-chain action,
not ad-hoc spreadsheets).

## 2. QDP-0012 governance — every bank is its own governing body

### 2.1 The domain + governance shape

Each bank is an operator with its own domain tree:

```
bank.<id>                              (the bank's root domain)
    ├── wires.outbound                 (wires originated by this bank)
    ├── wires.inbound                  (wires received by this bank)
    ├── signatories                    (who's authorized to cosign)
    ├── audit                          (compliance log)
    └── peering.counterparty-banks     (trust edges to other banks)
```

For `bank.chase`, the governance structure:

```
Domain: bank.chase.wires.outbound
  governors:
    chase-chief-executive-quid         weight=3
    chase-chief-compliance-quid        weight=2
    chase-chief-risk-quid              weight=2
    fed-reserve-monitor-quid           weight=2   # regulator observer
  governanceQuorum: 0.7   # 7 of 9 weighted
  notice period: 24 hours normally; 1h emergency clause for REMOVE_VALIDATOR

  consortium (validators):
    node-chase-ny-primary              weight=1
    node-chase-london-primary          weight=1
    node-chase-sg-primary              weight=1
  validatorTrustThreshold: 0.67  # 2-of-3 blocks accepted

  cache replicas:
    node-chase-ny-cache-1..5
    node-chase-london-cache-1..3
    node-chase-sg-cache-1..2
    (plus any read-only observer node the regulator runs)
```

Crucially, the regulator (`fed-reserve-monitor-quid`) is a
**governor, not a validator**. They can vote on governance
changes + observe every transaction, but they don't produce
blocks. This distinction matters: regulators need visibility +
authority-to-block-bad-changes but shouldn't hold block-
production keys (that's the bank's operational concern).

### 2.2 Signatory quorum as a domain property

A wire transfer needs M-of-N cosigning from authorized bank
employees. Instead of hardcoding this in application logic,
the signatory quorum is a **governance property of the
`bank.chase.wires.outbound` domain**:

```json
{
  "domain": "bank.chase.wires.outbound",
  "signatoryQuorum": {
    "minSigners": 2,
    "minTotalWeight": 3,
    "requiredRoles": ["wire-officer", "compliance-officer"],
    "maxSigners": 5
  },
  "signatoryPool": {
    "wire-officer-alice-quid": { "weight": 1, "role": "wire-officer" },
    "wire-officer-bob-quid":   { "weight": 1, "role": "wire-officer" },
    "compliance-carol-quid":   { "weight": 2, "role": "compliance-officer" },
    "vp-david-quid":           { "weight": 2, "role": "vp-operations" }
  }
}
```

Any addition / removal / weight change to the signatory pool
is a `DOMAIN_GOVERNANCE` action (QDP-0012) — signed by the
bank's governor quorum, subject to the 24-hour notice period.
This is the critical protection against insider attack: a
compromised wire-officer can't silently add a co-conspirator;
that change takes 24 hours + signatures from multiple
governors.

### 2.3 Wire-specific governance actions

Three wire-specific actions extend the standard QDP-0012 set:

| Action | Effect | Required signers |
|---|---|---|
| `ADD_SIGNATORY` | Add a quid to the wire-signatory pool | governor quorum |
| `REMOVE_SIGNATORY` | Remove a quid | governor quorum |
| `UPDATE_SIGNATORY_POLICY` | Change `minSigners`, `requiredRoles`, thresholds | governor quorum + 7-day notice (longer than ADD/REMOVE) |

Each action takes effect after the notice period. During the
notice window, the old policy remains active; the old
signatories can continue cosigning pending wires. This
ensures no in-flight wires are orphaned by a signatory-pool
change.

## 3. QDP-0013 federation — every bank is its own network

### 3.1 The federation topology

Every bank runs its own Quidnug network. Cross-bank wires
federate bilaterally:

```
            ┌─────────────────┐      federation       ┌─────────────────┐
            │  Bank A network │◄────────────────────►│  Bank B network │
            │  bank.chase.*   │   TRUST_IMPORT for    │ bank.deutsche.* │
            │                 │   counterparty        │                 │
            │  own consortium │   attestations        │  own consortium │
            │  own governors  │                        │  own governors  │
            └─────────────────┘                        └─────────────────┘
                     ▲                                          ▲
                     │                                          │
                     │ federation                   federation │
                     │                                          │
                     ▼                                          ▼
            ┌──────────────────────────────────────────────────────────┐
            │    Correspondent bank network (or SWIFTnet bridge)      │
            │    federates between pairs of origination banks         │
            └──────────────────────────────────────────────────────────┘
```

Each bank operator independently runs their consortium, holds
their own keys, and manages their own domain tree. What
federation adds is the ability to cross-reference attestations
between networks.

### 3.2 Three federation use cases

**Use case 1: a wire originated at Bank A, destined to
Bank B.** Bank A's network publishes a `WIRE_AUTHORIZED`
event. Bank B's network federates with Bank A's to verify:

- Wire came from a signatory in good standing on Bank A's
  chain.
- Quorum of signatures met Bank A's policy.
- Wire amount + currency + beneficiary match what's being
  remitted.

Bank B then publishes its own `WIRE_RECEIVED` event on its
chain referencing Bank A's originating event. Two signed
records of the same wire, one per bank. Reconciliation
becomes trivial.

**Use case 2: regulator audits across banks.** A federal
regulator (FinCEN, FSA, FCA, etc.) runs a read-only node
that federates with every bank under their jurisdiction. They
can query "show me every wire over $1M to sanctioned
countries in Q4 2026" across dozens of banks without having
to request data from each one.

All responses are cryptographically signed by the
originating bank's governor quorum; the regulator has no
access to any bank's private keys but can verify every
response is authentic.

**Use case 3: correspondent-bank reputation.** A bank choosing
whether to open a correspondent relationship with another
bank can query that bank's public reputation edges — e.g.,
"has FinCEN flagged this bank for sanctions violations in the
last 24 months?" Trust edges from regulators (or from other
banks) are public + signed + verifiable.

### 3.3 Privacy boundaries

Some wire details are public on both banks' chains (from
+ to quids, timestamp, wire amount in aggregate). Others
are encrypted payloads with on-chain hash commitments (the
memo line, internal routing details, KYC documentation).

Only authorized parties (originator bank, receiver bank, the
two beneficiaries' banks if different, plus regulators with
subpoena-backed access) can decrypt the encrypted parts.
The hashes on-chain are enough for auditors to verify later
that "what was actually sent" matches the public record.

This pattern is generalizable to any federation with
privacy concerns — see QDP-0017 for the data-subject-rights
framework that formalizes it.

## 4. QDP-0014 node discovery + sharding

### 4.1 Operator-to-nodes hierarchy for a global bank

A major bank runs nodes in every major banking center
(NY, London, Singapore, Tokyo, Hong Kong, Frankfurt, etc.).
Each node is attested by the bank's operator quid:

```
chase-operator-quid
    ──TRUST 1.0──►  node-chase-ny-primary    (validator)
    ──TRUST 1.0──►  node-chase-ny-backup     (validator)
    ──TRUST 1.0──►  node-chase-london-primary (validator)
    ──TRUST 1.0──►  node-chase-london-backup (validator)
    ──TRUST 1.0──►  node-chase-sg-primary    (validator)
    ──TRUST 1.0──►  node-chase-ny-cache-1..5 (cache, NY region)
    ──TRUST 1.0──►  node-chase-london-cache-1..3
    ──TRUST 1.0──►  node-chase-sg-cache-1..2
    ──TRUST 1.0──►  node-chase-archive-1     (archive)
    ──TRUST 1.0──►  node-chase-archive-2     (archive, different region)
    in operators.bank.chase
```

### 4.2 Node advertisements per role

Each node publishes a signed `NODE_ADVERTISEMENT`:

```json
{
    "nodeQuid": "node-chase-ny-primary-quid",
    "operatorQuid": "chase-operator-quid",
    "endpoints": [
        { "url": "https://wires-ny.chase.com", "region": "us-ny", "priority": 1, "weight": 100 }
    ],
    "supportedDomains": ["bank.chase.wires.outbound", "bank.chase.wires.inbound"],
    "capabilities": {
        "validator": true,
        "cache": true,
        "archive": false,
        "gossipSink": true
    },
    "protocolVersion": "2.4.0",
    "expiresAt": 1748030400,
    "advertisementNonce": 847
}
```

Clients (a corporate treasury team originating a wire, a
compliance officer auditing) use the discovery API:

```
GET /api/v2/discovery/domain/bank.chase.wires.outbound
```

Returns the three NY/London/SG validators plus the 10
regional caches. The client's HTTP library picks the
closest (by geoip, or by a pre-configured region preference)
and submits the wire authorization there.

### 4.3 Wire-processing topology

Daily volume at a major bank is ~500k wires / day. Peak is
~10k/minute during end-of-day cutoffs. The topology needs to
handle this without dropping wires:

```
┌──────────────────────────────────────────────────────┐
│ Validators (3 regions, 2-of-3 consortium)            │
│  - Produce blocks for wires.outbound + wires.inbound │
│  - Handle ~1000 authorized wires/s at peak            │
└────────────────────┬─────────────────────────────────┘
                     │ gossip
┌──────────────────────────────────────────────────────┐
│ Cache tier (10+ nodes, 3 regions)                    │
│  - Handle read queries from corporate treasury        │
│    teams + compliance officers                        │
│  - Target: 50k+ QPS per region                        │
└────────────────────┬─────────────────────────────────┘
                     │ api gateway
                     ▼
┌──────────────────────────────────────────────────────┐
│ api-wires-<region>.chase.com                         │
│  - Cloudflare Worker routing                         │
│  - Corporate treasury + compliance access             │
│  - Rate-limited per corporate account                │
└──────────────────────────────────────────────────────┘
```

Shard by:

- **Geography:** each region has its own validator pool and
  cache tier. US wires hit NY; Europe hits London; Asia
  hits Singapore. Cross-region wires gossip across
  consortiums.
- **Capability:** validators don't serve reads; caches
  don't produce blocks; archives are rarely touched except
  during regulatory queries.
- **Domain-tree:** `wires.outbound` and `wires.inbound` are
  separate domains, each with its own consortium. Some
  smaller banks run them on the same consortium; giant
  banks run them independently for isolation.

## 5. No-node participation for corporate clients

A mid-size corporation (say, a supply-chain company needing
to pay international suppliers) doesn't run a Quidnug node.
They:

1. Hold a cryptographic quid for their treasury department.
2. Have trust edges from their bank(s) attesting to their
   account authorization.
3. Use client-side SDK to sign wire-authorization events.
4. POST to the bank's api gateway.

The bank's validator consortium verifies:
- The corporate's quid is in good standing.
- Signatory quorum for THE CORPORATE entity's policy is met
  (e.g., CFO + controller for wires > $1M).
- The wire is properly formed.

If all pass, the wire is authorized + transmitted to the
correspondent/receiving bank via existing SWIFT or a federated
Quidnug link.

The corporate gets real-time verifiability (they know when
the wire hit each stage) without running any infrastructure.
Same pattern as voters in elections — full cryptographic
participation, zero infrastructure. This is QDP-0014's
"lightweight participation" mode.

## 6. Cross-network flow: end-to-end wire

Full sequence, touching all three pillars:

```
1. Corporate treasurer at MegaCorp prepares wire instruction:
      $2.5M USD from MegaCorp's USD account at Chase
      to beneficiary's EUR account at Deutsche Bank.

2. MegaCorp's treasurer (Alice) and CFO (Bob) both sign an
   instruction (each with their own quid; quorum 2-of-2
   required per MegaCorp's internal policy for $1M+).
   Published as WIRE_REQUEST event on MegaCorp's own domain
   bank.chase.corporate.megacorp.wire-requests (delegated
   sub-tree).

3. Chase's validator consortium receives the request via
   gossip. Validates:
   - MegaCorp corporate quid is authorized on Chase's books.
   - Quorum met per MegaCorp's policy.
   - Wire format + currency valid.
   - AML screening passes.

4. Chase publishes WIRE_AUTHORIZED event on
   bank.chase.wires.outbound domain. Signed by 2-of-3
   consortium validators. This is the cryptographic
   "Chase has approved this wire."

5. Chase's network federates with Deutsche's network via
   QDP-0013. Deutsche's consortium picks up the event,
   validates:
   - Event signed by Chase's validator consortium (verify
     against Chase's .well-known file pinned pubkey).
   - Beneficiary exists at Deutsche.
   - Wire format compatible with Deutsche's standards.

6. Deutsche publishes WIRE_RECEIVED event on
   bank.deutsche.wires.inbound domain. References Chase's
   WIRE_AUTHORIZED event by its tx ID.

7. Deutsche credits the beneficiary's account. Publishes
   SETTLEMENT event referencing the WIRE_RECEIVED.

8. Chase's inbound federation picks up the SETTLEMENT event,
   publishes SETTLEMENT_CONFIRMED on its own chain, completing
   the loop.

9. Both banks' regulators (FinCEN, BaFin) see the complete
   chain of events via their read-only federation nodes.

10. MegaCorp's treasurer can verify every step by fetching
    the events from either bank's api gateway.
```

Every step is cryptographically signed + publicly verifiable.
The whole wire takes ~30 seconds vs. the current ~1 hour
minimum for SWIFT + correspondent banking. Regulatory access
is real-time instead of quarterly after-action reviews.

## 7. Governance changes mid-wire-day

Unlike elections (which happen on a fixed schedule),
interbank wires happen 24/7. Governance changes have to be
non-disruptive.

### 7.1 Adding a new signatory mid-day

Routine — happens whenever a new person is hired or
promoted to a wire-authorization role.

1. Governor quorum signs `ADD_SIGNATORY` transaction.
2. 24-hour notice period: the new signatory can't sign
   wires yet.
3. At activation, the new signatory is added to the pool
   with the specified role + weight.

Banks routinely plan these 24h+ in advance (employee
onboarding takes that long anyway).

### 7.2 Emergency removal of a compromised signatory

If a signatory's key is suspected compromised:

1. Any governor can initiate `REMOVE_SIGNATORY` with the
   1-hour emergency clause.
2. Immediate audit event published noting the removal +
   reason.
3. 1 hour later: signatory is removed from the pool.
   Any pending wires they signed in the last hour are
   flagged for manual review.
4. Post-incident: guardian recovery on their individual
   quid if desired (separate from the pool-removal action).

### 7.3 Cross-bank coordinated changes

Occasionally two or more banks need to coordinate changes
(e.g., standard cutoff times for a new currency pair).
Each bank independently publishes the change on their own
network; federation ensures the other bank sees it. No
central coordinator is required; the federation topology
is the coordination layer.

## 8. Operational readings

- [`operations.md`](operations.md) — deployment topology,
  capacity planning, daily operations playbook, incident
  response.
- [`launch-checklist.md`](launch-checklist.md) — 3-6 month
  bank-onboarding-to-live-wire-traffic sequence.

For the architectural pillars themselves:

- [`deploy/public-network/governance-model.md`](../../deploy/public-network/governance-model.md)
- [`deploy/public-network/federation-model.md`](../../deploy/public-network/federation-model.md)
- [`deploy/public-network/sharding-model.md`](../../deploy/public-network/sharding-model.md)

## 9. Not covered here

Two substantial topics this integration doc deliberately
defers:

- **SWIFT/Fedwire/SEPA bridging.** Each jurisdiction has
  existing wire-transfer infrastructure. The Quidnug
  design replaces the *authorization* layer, not the
  *transport* layer — a wire authorized on Quidnug still
  typically moves via SWIFT. Bridge design is its own
  substantial project; see `operations.md` §5 for the
  integration pattern.

- **Regulatory reporting (AML, CTR, SAR).** Each country's
  reporting requirements differ. Quidnug makes the data
  available in a standardized format; the per-jurisdiction
  reporting code lives in bank-specific applications on
  top.

Both are legitimate production concerns; both are
application-layer, not protocol-layer. A deploying bank
builds them atop the Quidnug substrate the same way they
build SWIFT message generation atop internal databases
today.
