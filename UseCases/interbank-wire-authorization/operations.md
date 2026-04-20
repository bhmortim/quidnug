# Interbank wire authorization operations

> Deployment topology, capacity planning, daily operations, and
> incident response for a bank running Quidnug-based wire
> authorization. Adapted from
> [`UseCases/elections/operations.md`](../elections/operations.md)
> for banking deployment profile.

## 1. Deployment scale

| Scale | Example | Wires / day | Nodes | Monthly cost | Staffing |
|---|---|---|---|---|---|
| Small community bank | Local credit union | < 100 | 2 validators + 1 cache | $200-500 | 1 part-time admin |
| Regional bank | State-level commercial bank | 1k-10k | 5 nodes, 2 regions | $2k-5k | 2 FTEs ops |
| National bank | Top 20 US bank | 10k-100k | 15-30 nodes, 3 regions | $20k-60k | 8-15 FTEs ops |
| Global | Chase, HSBC, Deutsche | 100k-1M | 50-150 nodes, 8+ regions | $100k-500k | 40+ FTEs ops + compliance |

Different from elections: wire volume per day is higher and
more continuous. 24/7 operation. Lower tolerance for outages
(minutes of downtime = tens of millions in delayed settlements).

## 2. Deployment architecture

### 2.1 Small community bank

```
  ┌──────────────────────────┐
  │ Primary validator        │  (in-branch data center or
  │ (validator + cache +     │   cloud + private connection)
  │  archive, behind CF)     │
  └──────────┬───────────────┘
             │ gossip
  ┌──────────────────────────┐
  │ Backup validator         │  (disaster-recovery facility)
  │ (validator + cache)      │
  └──────────┬───────────────┘
             │
  ┌──────────────────────────┐
  │ Regional cache           │  (bank's HQ, wide-area redundancy)
  └──────────────────────────┘
             │
             ▼
  corporate treasury clients (via API)
```

Three nodes. 2-of-3 validator consortium. Governors: bank CEO,
head of ops, head of compliance, state banking commissioner.

### 2.2 Regional bank

```
East-coast region               West-coast region
┌─────────────────────┐         ┌─────────────────────┐
│ val-east-1          │◄───────►│ val-west-1          │
│ val-east-2          │         │ val-west-2          │
└──────┬──────────────┘         └──────┬──────────────┘
       │                               │
       │ gossip                        │ gossip
       │                               │
┌──────────────────────────────────────────────────────┐
│ Central consortium member (audit + reconciliation)   │
│ (validator — tiebreaker)                             │
└──────────────────────────────────────────────────────┘

Per region, cache tier:
east: 3 cache nodes
west: 2 cache nodes

Archive: 2 nodes, geographically distributed.
```

5 validators, 2-of-5 for normal operation, geo-redundant.

### 2.3 National bank

```
┌───────────────────────────────────────────────────────────┐
│ Consortium (11 nodes, 6-of-11 quorum)                    │
│ - 3 east-coast (primary, backup, audit-focused)          │
│ - 3 west-coast                                            │
│ - 2 Texas (central)                                       │
│ - 1 London (for EUR wires)                               │
│ - 1 Singapore (for Asian wires)                          │
│ - 1 Compliance observer (Fed/OCC monitor)                │
└───────────────────────────────────────────────────────────┘

Cache tier:
  5 caches per major region (NY, LA, Chicago, DFW, Atlanta,
  London, Singapore) = ~35 cache nodes total.

Archive:
  4 nodes: 2 in separate regulator-approved data centers,
  2 in cold-storage backup.
```

### 2.4 Global bank

Per major banking center, a full regional consortium:
- 3-5 validators
- 5-10 cache nodes
- 1-2 archive
- Regulator observer nodes per jurisdiction

Cross-region federation per QDP-0013. Each jurisdiction
may have its own regulator + its own governance variation
(e.g., EU wires governed under EU rules; US wires under
FFIEC/OCC).

Total: 50-150 nodes spread across 8-15 countries.

## 3. Hardware

| Role | CPU | RAM | Disk | Network |
|---|---|---|---|---|
| Validator | 8 cores | 32 GB | 2 TB NVMe (IOPS 50k) | 10 Gbps |
| Cache | 4 cores | 16 GB | 500 GB SSD | 1 Gbps |
| Archive | 4 cores | 16 GB | 20 TB HDD + 1 TB SSD cache | 1 Gbps |
| Observer | 2 cores | 8 GB | 1 TB SSD | 100 Mbps |

Validators are expensive — they need high-IOPS SSD for
replay + crash recovery. Caches are cheap; can be spot
instances. Archives are slow but huge.

Per QDP-0014's sharding model, validator-cache-archive
separation is enforced via node advertisement capabilities
— a cache node cannot produce blocks even if configured
incorrectly.

## 4. Capacity planning

### 4.1 Wire volume

For a national bank with 50k wires/day = ~0.58/s sustained
= ~10/s peak (end-of-day cutoff). Per-wire transaction size:

| Stage | Events | Size/event | Sub-total |
|---|---|---|---|
| Authorization | 1-5 signatures | 500B | 0.5-2.5 KB |
| Routing decision | 1 | 200B | 0.2 KB |
| Send | 1 | 500B | 0.5 KB |
| Receipt confirmation | 1 | 300B | 0.3 KB |
| Settlement | 1 | 400B | 0.4 KB |
| Per-wire total | 5-9 | | ~2-4 KB |

50k wires/day × 3 KB = 150 MB/day on-chain data. 55 GB/year.
Archive nodes handle this; cache tier keeps only recent
months.

### 4.2 Query load

Much higher than write load. Corporate treasury + compliance
teams + regulator access + reconciliation systems all query
frequently:

- Wire status queries: ~200k/day (4x wire count)
- Compliance report generation: ~1k/day
- Reconciliation batch jobs: continuous
- Regulator queries: variable, occasionally high

Rate limit: 50k/hour per corporate client, 1M/hour aggregate.
Cache tier should handle 50-100k QPS per region.

### 4.3 Latency targets

- Wire authorization (originator side): <3s from signing to
  consortium-accepted block
- Cross-bank handoff: <30s from authorization on Bank A to
  WIRE_RECEIVED event on Bank B
- End-to-end settlement: <2 minutes
- Regulator query response: <5 seconds for live data,
  <30 seconds for archive queries

Block interval: 3 seconds (shorter than elections' 60s)
because wire latency matters; consortium members produce
blocks more aggressively.

## 5. Integration with existing wire infrastructure

Quidnug handles authorization + audit; existing systems
handle transport (SWIFT MT, Fedwire, SEPA, CHIPS).

### 5.1 SWIFT bridge

```
Corporate → Quidnug authorization (cryptographic quorum) →
  WIRE_AUTHORIZED event →
  Quidnug-to-SWIFT bridge service generates MT103 message →
  SWIFT network delivers →
  Receiving bank's SWIFT MT103 → Quidnug-to-SWIFT bridge
  generates WIRE_RECEIVED event on Quidnug.
```

The bridge is a trusted service operated by the bank's IT
team. Not a Quidnug primitive — just application code
tying existing SWIFT connectivity to Quidnug's
authorization + audit layer.

For banks wanting to skip SWIFT entirely (interbank
settlement via Quidnug alone), QDP-0013 federation makes
this possible but requires bilateral agreement with every
counterparty bank. Realistic only for closed-consortium
scenarios (e.g., a central-bank settlement network).

### 5.2 Fedwire

Similar bridge pattern. Fedwire's ISO 20022 message format
is structurally similar to SWIFT MT; one bridge handles
both with different message-generation modules.

The bank's Fedwire credentials (the one-per-bank account
with the Fed) stay in the bank's existing infrastructure;
Quidnug events trigger Fedwire message generation.

### 5.3 SEPA

SEPA credit transfers (SCT) and instant payments (SCT Inst)
both have ISO 20022 message formats. Same bridge pattern.

SEPA is the closest to "real-time" in existing
infrastructure and benefits the most from Quidnug's
authorization + audit speed. A Quidnug-accelerated SEPA
flow can complete in seconds end-to-end.

## 6. Daily operations playbook

### 6.1 Pre-market (03:00-06:00 local)

- Operations team runs overnight reconciliation between
  consortium state and core banking system.
- Batch settlement jobs for prior-day wires complete.
- Archive backup runs.
- Morning briefing: on-call rotation, known incidents.

### 6.2 Market open (06:00-18:00)

- Wire traffic ramps throughout the morning.
- Peak hours: 09:00-11:00 (Europe open), 15:00-16:00 (US
  close + Europe late).
- End-of-day cutoff: typically 17:00 or 18:00 local.
- Real-time monitoring: per-region validator participation,
  cross-bank federation latency, cache hit rates.

### 6.3 End-of-day cutoff

- Hard cutoff published as a `MARKET_CLOSE` event per region.
- Wires submitted after cutoff queue for next-day.
- Consortium waits for all in-flight wires to settle before
  sealing the final block of the day.
- Daily reconciliation automated: every wire matched
  against every settlement event; any unmatched flagged
  for manual review.

### 6.4 Overnight (18:00-03:00)

- Backup + archive sync.
- Regulator reports generated + submitted per jurisdiction
  requirements (varies; US: CTR for >$10k wires, SAR for
  suspicious activity).
- System maintenance window for non-urgent updates.

## 7. Incident response

### 7.1 Validator node down during market hours

**Detection:** health-check alert on the validator.

**Response:**
1. Immediately confirm: is it just network, process, or
   hardware?
2. Remaining validators continue (6-of-11 quorum preserved
   if one is down).
3. IR team investigates in parallel with continued operation.
4. If recovery time > 1 hour during peak: trigger emergency
   `REMOVE_VALIDATOR` to unblock any 6-required operation.
5. Post-incident: audit event published; root cause analysis
   within 48h.

### 7.2 Cross-bank federation partition

**Detection:** federation latency alert — e.g., Chase-to-
Deutsche federation hasn't echoed a WIRE_RECEIVED event in
more than 5 minutes.

**Response:**
1. Check both banks' status pages for correlated outages.
2. Verify network connectivity (BGP, DNS, Cloudflare status).
3. If transient: automated retry with exponential backoff.
4. If sustained: bridge to SWIFT as fallback; notify
   affected corporates.
5. Post-healing: reconcile any wires that went via SWIFT
   fallback against the federation record.

### 7.3 Suspected signatory key compromise

**Detection:** anomalous wire pattern (volume, destination,
time-of-day) flagged by AML engine; or direct security-team
alert.

**Response:**
1. Freeze the signatory's quid immediately (via governor
   quorum emergency REMOVE_SIGNATORY action — 1 hour
   notice).
2. Audit all wires signed by that signatory in the last
   24h; flag for additional manual review before
   settlement.
3. Notify affected corporates.
4. Law enforcement referral per bank policy.
5. Guardian recovery on the compromised quid; new key
   installed only if the original holder is cleared.

### 7.4 Regulator subpoena

**Detection:** legal notice arrives.

**Response:**
1. Legal team reviews scope.
2. Operations generates read-only access for the regulator's
   designated investigator — a federated read-only node
   connecting to the bank's network with scoped domain
   access.
3. All queries the regulator runs are logged on-chain (the
   regulator's node publishes its queries as events on an
   audit domain, so the subpoena scope is provable).
4. Regulator's queries return cryptographically-verifiable
   data; no edits possible; the chain IS the record.
5. If regulator also wants decrypted payload data (KYC docs,
   wire memos), legal team evaluates scope; decryption keys
   are released per the legal requirement.

This is a significant UX upgrade over current state —
regulatory queries today require months of IT work
extracting data from multiple systems + internal review.
With Quidnug, it's hours.

## 8. Security discipline

### 8.1 Key custody

| Role | Custody |
|---|---|
| Governor (CEO, compliance chief) | HSM-backed + paper backup + 3-of-5 guardian quorum |
| Validator node | Dedicated HSM per validator; keys never leave HSM |
| Signatory (wire officer) | YubiKey or similar; 2FA required; daily-use key |
| Corporate treasurer | Their own custody; bank provides optional guardian-recovery-as-a-service for small corporates |

### 8.2 Separation of duties

- Block-production (validator) keys ≠ wire-signing (signatory) keys.
- Governance keys ≠ operational keys.
- A compromised wire officer cannot alone produce wires;
  quorum + separation-of-duty enforced on-chain.
- A compromised validator cannot alone change signatory
  pools; that requires governance quorum.

### 8.3 Annual audit

- External security audit of the bank's Quidnug
  infrastructure annually.
- Quarterly penetration testing.
- Daily automated security scans.

## 9. Cost at scale

| Scale | One-time | Ongoing/year | Main cost drivers |
|---|---|---|---|
| Small bank | $50k | $5k | HSM, one staff member's time |
| Regional | $500k | $150k | Multiple HSMs, 2 FTEs |
| National | $5M | $3M | Enterprise HSMs, 10-15 FTEs, regulator-certified data centers |
| Global | $50M | $30M | Dozens of regional HSMs, 40+ FTEs, global compliance framework |

Compare to current wire infrastructure: global banks spend
~$100M+/year on wire systems alone (SWIFT connectivity,
Fedwire licensing, FX infrastructure). Quidnug's deployment
is a fraction.

## 10. References

- [`README.md`](README.md) — wire-authorization semantics.
- [`architecture.md`](architecture.md) — data model + flows.
- [`integration.md`](integration.md) — architectural-pillar
  integration.
- [`launch-checklist.md`](launch-checklist.md) — bank-
  onboarding sequence.
- [`threat-model.md`](threat-model.md) — attack analysis.
- [`UseCases/elections/operations.md`](../elections/operations.md)
  — companion operations doc; same-pattern at election scale
  rather than daily-banking scale.
