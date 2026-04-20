# Enterprise domain authority operations

> Deployment topology, daily operations, runbooks for
> employee + partner + record-change flows, incident
> response. Written for a national-bank-scale enterprise
> running ~3,000 employees + 150 partners + split-horizon
> DNS under one `AUTHORITY_DELEGATE`.

## 1. Deployment scale

| Scale | Example | Employees | Partners | Records | Nodes | Monthly cost |
|---|---|---|---|---|---|---|
| SMB | Regional law firm | 50-200 | 10-30 | ~500 | 2 validators + 1 cache | $300-800 |
| Mid-market | Regional bank, mid-sized healthcare | 500-2000 | 30-100 | ~5,000 | 3 validators + 3 cache | $2,000-5,000 |
| Enterprise | National bank, global SaaS | 2000-10000 | 100-500 | ~50,000 | 5 validators + 10 cache | $10,000-30,000 |
| Global | Fortune 100 | 10000+ | 500+ | ~500,000 | 10+ validators + 40+ cache | $50,000-200,000 |

Scale drivers: employee count (group-key rotation frequency),
partner count (trust-gated record query volume), record count
(storage + query volume).

## 2. Deployment architecture

### 2.1 Mid-market reference topology

```
         ┌──────────────────────────────────────┐
         │  BigCorp governance quorum           │
         │  (CEO + CFO + CTO + CISO + HR head)  │
         └─────────────────┬────────────────────┘
                           │
                           │ signs governance events
                           ▼
  ┌────────────────────────────────────────────────┐
  │ Validator 1 (US-East: NYC data center)        │
  │ Validator 2 (US-West: Silicon Valley)          │
  │ Validator 3 (Europe: Frankfurt)                │
  │   roles: validator + archive                   │
  │   keys: HSM-backed per validator               │
  └────────┬─────────┬─────────┬───────────────────┘
           │         │         │ gossip
           ▼         ▼         ▼
     ┌──────┐   ┌──────┐   ┌──────┐
     │Cache │   │Cache │   │Cache │
     │NYC   │   │SV    │   │Fra   │
     └──────┘   └──────┘   └──────┘
           │         │         │
           ▼         ▼         ▼
     Public   Employees   Partners
     resolvers  (client    (client
                apps)      apps)
```

3 validators (2-of-3 consortium quorum). 3 caches (one per
region). Authoritative serves from validators; caches serve
most queries.

### 2.2 Validator hardware

| Component | Spec |
|---|---|
| CPU | 8-16 cores |
| RAM | 32 GB minimum |
| Disk | 2 TB NVMe (IOPS 50k+) |
| Network | 1 Gbps minimum; 10 Gbps preferred |
| HSM | Thales Luna / Entrust nShield / AWS CloudHSM / Azure Dedicated HSM |

### 2.3 Cache hardware

| Component | Spec |
|---|---|
| CPU | 4 cores |
| RAM | 16 GB |
| Disk | 500 GB SSD |
| Network | 1 Gbps |

Cache nodes can be spot instances; they're stateless
(replicate from validators).

## 3. Runbooks

### 3.1 Employee onboarding

**Trigger:** HR adds employee to IAM system.

**Time to complete:** ~5 minutes automated; ~30 minutes manual.

**Steps:**

1. IAM webhook fires to `onboarding-sync` service.
2. Sync service:
   a. Generates a new Quidnug quid for the employee (or
      uses their existing personal quid if they have one +
      policy allows).
   b. Publishes `IDENTITY` transaction registering the quid
      under `bigcorp.com.employees` with HR governor as
      creator.
   c. Publishes `MEMBER_KEY_PACKAGE` event with the
      employee's X25519 public key (from their device
      keyring).
   d. Publishes TRUST edge from `bigcorp.com.employees` to
      the new quid at level 1.0, `validUntil` =
      (employment end date if fixed-term).
3. Group admin service (can be automated):
   a. Detects new member via dynamic group membership.
   b. Signs `EPOCH_ADVANCE` event with path update + welcome
      package for new employee.
4. Employee's device client (SDK):
   a. Polls for welcome package.
   b. Decrypts welcome with their X25519 private key.
   c. Derives current epoch key.
   d. Can now decrypt `private:bigcorp.com.employees`
      records.

**Observability:** Prometheus metric
`quidnug_group_additions_total{group_id="bigcorp.com.employees"}`
increments.

**Failure modes:**

- Welcome package doesn't arrive: check that
  `MEMBER_KEY_PACKAGE` was published before
  `EPOCH_ADVANCE`. Retry epoch-advance with correct
  packaging.
- New employee can't decrypt: verify their X25519 private
  key is correctly provisioned on their device. Common
  cause: wrong keyring path in SDK configuration.

### 3.2 Employee offboarding

**Trigger:** HR removes employee from IAM system.

**Time to complete:** ~2 minutes automated.

**Steps:**

1. IAM webhook fires to `offboarding-sync` service.
2. Sync service:
   a. Revokes TRUST edge from `bigcorp.com.employees` to the
      departing employee.
   b. If immediate cut-off is required (terminations for
      cause): signs emergency REMOVE_MEMBER with 1-hour
      notice.
3. Group admin:
   a. Detects removed member on next dynamic check.
   b. Signs `EPOCH_ADVANCE` with `reasonCode="member-removed"`.
4. Departing employee cannot derive the new epoch key;
   loses access to all records published from this block
   forward.

**Observability:** Metric
`quidnug_group_removals_total{group_id}` increments.

**Failure modes:**

- Epoch advance not honored by cache tier: stale caches
  might serve old records to removed employee's cached
  queries briefly. Mitigation: max cache TTL of 60 seconds
  for sensitive records.
- Key material persistence on employee's device: employee
  could retain cached past-epoch keys and read records they
  previously accessed. By design; past access is preserved.
  For stricter requirements, rotate to a new group entirely.

### 3.3 Partner onboarding

**Trigger:** Business partnership agreement signed.

**Time to complete:** ~1 day (primarily agreement + trust-
edge establishment workflow).

**Steps:**

1. Partner generates (or provides existing) quid.
2. BigCorp's legal + partnership team reviews the
   partnership terms.
3. Governance quorum signs TRUST edge:
   ```
   quidnug-cli trust grant \
       --truster bigcorp.com.partners \
       --trustee <partner-quid> \
       --level 0.8 \    # or 0.5 for loose integration
       --domain bigcorp.com.partners \
       --valid-until <contract-end-date> \
       --sign-with <governance-key>
   ```
4. Partner receives confirmation + documentation on how to
   query trust-gated records.
5. Partner integrates via BigCorp's client library.
6. Partner tests: queries `_api.partners/API/v2` record.
   Response indicates successful integration.

**Observability:** Dashboard shows per-partner query rate
+ success/failure.

**Failure modes:**

- Partner can't access records: verify trust edge exists +
  has not expired. Check `min_trust` threshold in
  delegation policy.
- Partner's trust edge expires mid-contract: monitoring
  should alert 30 days before expiry. Auto-renew if
  contract is auto-renewal.

### 3.4 Publishing a public record

**Trigger:** Marketing team needs to update website IP /
Mail team rotates MX / etc.

**Time to complete:** seconds.

**Steps:**

```
quidnug-cli records publish \
    --domain bigcorp.com \
    --record-type A \
    --name "@" \
    --value "203.0.113.100" \
    --ttl 300 \
    --visibility public \
    --sign-with <ops-key>
```

**Audit trail:** Event is signed + timestamped; visible to
anyone.

### 3.5 Publishing a private record

**Trigger:** HR updates employee directory / engineering
rotates internal service endpoint.

**Time to complete:** seconds.

**Steps:**

```
quidnug-cli records publish \
    --domain bigcorp.com \
    --record-type "DIRECTORY/employees" \
    --name "_directory" \
    --value-file latest-directory.json \
    --ttl 1800 \
    --visibility "private:bigcorp.com.employees" \
    --sign-with <ops-key>
```

Client-side: encrypt payload with current epoch key,
publish `ENCRYPTED_RECORD` event.

**Audit trail:** Event publication visible (timestamp +
publisher quid + group); content visible only to group
members.

### 3.6 Group epoch rotation (scheduled)

**Trigger:** Policy `rotationIntervalSeconds` elapsed
(default 90 days) OR ad-hoc security hygiene.

**Time to complete:** ~10 seconds to execute; members
receive updated state within 1 block (~3s).

**Steps:**

```
quidnug-cli groups advance-epoch \
    --group-id bigcorp.com.employees \
    --reason scheduled \
    --sign-with <group-admin-key>
```

### 3.7 Compromise response (emergency epoch rotation)

**Trigger:** Security team detects employee key
compromise OR suspicion.

**Time to complete:** ~2 minutes.

**Steps:**

```
# 1. Immediately revoke the compromised quid's trust edge
quidnug-cli trust revoke \
    --truster bigcorp.com.employees \
    --trustee <compromised-quid> \
    --sign-with <ciso-emergency-key>

# 2. Emergency epoch advance (all groups the compromised
# member was in)
quidnug-cli groups advance-epoch \
    --group-id bigcorp.com.employees \
    --reason compromise-response \
    --removed <compromised-quid> \
    --sign-with <ciso-emergency-key>

# 3. Audit: what did the compromised quid read recently?
quidnug-cli audit queries \
    --client-quid <compromised-quid> \
    --since "7d"

# 4. Forensic analysis + reporting per bigcorp policy.
```

Within 2 minutes the compromised quid has no access to
future records. Past records they accessed remain a
confidentiality risk (can't be un-accessed); risk is
assessed in the forensic analysis step.

### 3.8 Attestation renewal

**Trigger:** Attestation expiring within 60 days.

**Time to complete:** ~10 minutes.

**Steps:**

1. Automated monitor fires.
2. CLI publishes renewal request:
   ```
   quidnug-cli dns renew \
       --domain bigcorp.com \
       --root quidnug.com \
       --owner-key <key>
   ```
3. Root reruns verification (DNS TXT still present, well-
   known file still present, TLS fingerprint valid).
4. Root publishes new `DNS_ATTESTATION` with updated
   `validUntil`.
5. If TLS cert has rotated in the meantime: verify CT-log
   chain proof is included in the renewal.

## 4. Daily operations playbook

### 4.1 Morning (06:00 local)

- Validator health check: all 3 validators producing blocks.
- Cache health check: all cache replicas responsive.
- Overnight metrics review:
  - Query volume per visibility class
  - Any anomalous rejection rates (trust-gated failures)
  - Any failed publishes
  - Epoch-rotation activity
- Attestation status: all paid roots still attesting.
- On-call rotation: who's responsible today.

### 4.2 Midday (12:00 local)

- Peak-traffic monitoring: query latency p99 holding SLA.
- Partner integration health: per-partner query success
  rate.
- Any incident tickets opened since morning.

### 4.3 End-of-day (18:00 local)

- Day's publish count per record type.
- Day's onboarding / offboarding activity.
- Overnight backup job kicks off.

### 4.4 Overnight (18:00 - 06:00)

- Archive nodes sync.
- Scheduled group rotations (if due).
- Non-urgent maintenance (node upgrades, etc.).

## 5. Incident response

### 5.1 Validator node down

**Detection:** Health check alert.

**Response:**

1. Confirm it's a real failure (not transient network).
2. Consortium continues with remaining validators (2-of-3
   quorum preserved).
3. Spare validator brought online; `NODE_ADVERTISEMENT`
   published; consortium re-balances.
4. Post-incident RCA within 48h.

### 5.2 Cache tier outage in one region

**Detection:** Regional latency alert.

**Response:**

1. Regional queries fail over to nearest healthy region
   (clients automatically try next-best cache per QDP-0014).
2. Latency increases for affected region (from ~10ms local
   to ~50ms cross-region).
3. Recovery: bring cache nodes back or stand up spare
   capacity.
4. Cost: minor UX impact; no data loss.

### 5.3 Attestation root goes offline

**Detection:** Monitoring detects root unresponsive;
attestation can't be renewed.

**Response:**

1. If single root: roll over to backup root (enterprise
   should always have ≥2 roots attesting).
2. If all roots: critical incident. Contact root
   operators. Fall back to public DNS during outage.
3. Long-term: review root-diversification policy.

### 5.4 Suspected attestation fraud

**Detection:** A competing attestation appears for
`bigcorp.com` pointing to a different quid.

**Response:**

1. Log the event; file incident ticket.
2. Verify BigCorp's owner-quid attestations are still
   valid at all known roots.
3. If competing attestation is via a known root: contact
   the root's governor quorum to revoke.
4. If via an unknown/sybil root: confirm the sybil root has
   near-zero trust weight (it will). Monitor for any
   amplification attempts.
5. Publish a clarifying statement via BigCorp's website +
   attested press release (via content authenticity chain).

### 5.5 Group-admin key compromise

**Detection:** Unusual epoch-advance activity, or direct
security-team alert.

**Response:**

1. Guardian quorum on the group admin's key initiates
   recovery per QDP-0002.
2. New group admin key replaces old.
3. All groups administered by old key: emergency epoch
   advance to rotate current epoch key (defense in depth).
4. Forensic analysis of recent epoch advances for any
   unauthorized changes.

### 5.6 Legal subpoena for employee records

**Detection:** Legal notice arrives.

**Response:**

1. Legal team reviews scope + validity.
2. If valid: operations generates scoped decryption access.
   Either:
   a. Temporarily add the requesting party (e.g., regulator
      investigator) to the relevant group.
   b. Decrypt the specific records and hand over plaintext
      (outside the protocol).
3. Audit trail: all access logged via QDP-0018.
4. Notify affected employees per legal requirements.

## 6. Security discipline

### 6.1 Key custody

| Role | Custody |
|---|---|
| Owner-quid (BigCorp root) | HSM-backed + 5-of-7 guardian quorum |
| Governor quids (execs) | HSM + personal 3-of-5 guardians |
| Validator node keys | Dedicated HSM per validator |
| Group admin keys | HSM, dual control required for production |
| Employee quids | Device-secured (TPM / Secure Enclave / YubiKey) |

### 6.2 Separation of duties

- Owner-quid ≠ group-admin-quid ≠ validator-node-quid.
- No single key can compromise the whole system.
- Emergency action (REMOVE_VALIDATOR, compromise response)
  requires a separate emergency quorum from normal
  governance.

### 6.3 Audit requirements

- All governance actions audited per QDP-0018.
- Quarterly third-party audit of key custody + operational
  discipline.
- Annual penetration testing.

## 7. Cost breakdown

### 7.1 Mid-market annual cost

| Item | Cost/year |
|---|---|
| 3 validators (cloud or on-prem) | $10k-20k |
| 3 cache nodes | $3k-5k |
| HSM infrastructure | $10k-20k (amortized) |
| Attestation fees (3 roots × 1 domain × $5) | $15 |
| Operations staff (1 FTE allocation) | $100k-150k |
| Security + audit | $20k |
| **Total** | **~$140k-215k** |

### 7.2 Compared to legacy stack

Legacy enterprise equivalents (BIND + AD DNS + Vault +
Consul + audit tooling + IAM sync infrastructure):

- Typical mid-market annual TCO: $500k-1M+ across all the
  legacy systems.

Quidnug-based consolidation: ~1/3 to 1/5 the legacy cost +
dramatically better audit + compliance primitives.

## 8. References

- [`README.md`](README.md) — use-case overview.
- [`architecture.md`](architecture.md) — data model.
- [`integration.md`](integration.md) — integration with
  existing Quidnug deployments.
- [`threat-model.md`](threat-model.md) — attack analysis.
- [QDP-0023: DNS-Anchored Identity Attestation](../../docs/design/0023-dns-anchored-attestation.md)
- [QDP-0024: Private Communications & Group-Keyed Encryption](../../docs/design/0024-private-communications.md)
- [`UseCases/interbank-wire-authorization/operations.md`](../interbank-wire-authorization/operations.md)
  — companion operations doc for banks.
