# Enterprise domain authority integration

> How this use case layers onto existing Quidnug
> deployments: what to reuse, what to add fresh, how to
> bootstrap from nothing. Companion to
> [`README.md`](README.md) + [`architecture.md`](architecture.md).

## 1. Three deployment starting points

### 1.1 Starting from an existing Quidnug consortium

If BigCorp already runs Quidnug for another use case (e.g.,
interbank-wire-authorization), they extend the existing
consortium:

- **Reuse**: validator nodes, cache tier, governor quorum,
  guardian quorums, HSM infrastructure, operations playbook.
- **Add**: DNS attestation claim (pay + verify at a
  federated root), `AUTHORITY_DELEGATE` event, record
  publishing flow, employee + board + partner groups.

Implementation effort: ~1 person-week. Most infrastructure
is already in place.

### 1.2 Starting greenfield

If BigCorp has no existing Quidnug deployment, they bootstrap
minimal infrastructure first:

1. Deploy ≥2 Quidnug validator nodes (preferably 3+ for
   resilience).
2. Configure governor quorum.
3. Stand up consortium per the standard deployment (see
   `deploy/public-network/` + `helm/`).
4. *Then* claim the DNS attestation + delegate authority.

Implementation effort: ~3 person-weeks including
infrastructure.

### 1.3 Starting as a consumer (no own consortium)

Smaller orgs can skip running their own nodes and delegate
authority to a provider's consortium (a "managed Quidnug
DNS" service run by a trusted third party):

- **Reuse**: provider's validator consortium + cache tier.
- **Add**: only the DNS attestation claim + an
  `AUTHORITY_DELEGATE` pointing to the provider's nodes.
- Owner still holds their own signing key for publishing
  records; provider just serves them.

Implementation effort: ~1 day. Trade-off: trust in the
provider's operational integrity (they can't forge records
because those are signed by owner's key, but they can refuse
to serve them or slow down publication).

## 2. Integration with QDP-0023 attestation

### 2.1 Claim flow

```
1. BigCorp generates (or reuses) a Quidnug quid:
     $ quidnug-cli identity create \
         --name "bigcorp-owner" \
         --save-to ~/.quidnug/bigcorp-owner.key

2. BigCorp initiates the claim at a federated root:
     $ quidnug-cli dns claim \
         --domain bigcorp.com \
         --root quidnug.com \
         --owner-key ~/.quidnug/bigcorp-owner.key \
         --payment-method stripe \
         --requested-valid-until 2027-04-20T00:00:00Z

3. CLI returns the TXT record + well-known file content.
   BigCorp publishes both:
     DNS TXT at _quidnug-attest.bigcorp.com
     File at https://bigcorp.com/.well-known/quidnug-domain-attestation.txt

4. BigCorp pays the $5 standard-tier fee via Stripe link.

5. Root's verifier runs the full verification pass
   (multi-resolver DNS + TLS fingerprint + WHOIS + blocklist).

6. Root publishes DNS_ATTESTATION event.

7. Attestation now visible across federation.
```

Time from step 1 to step 7: typically 10-30 minutes. Most of
it is Stripe + DNS propagation.

### 2.2 Multi-root stacking

For high-stakes domains (bank, government), BigCorp pays
multiple independent roots:

```
$ quidnug-cli dns claim --domain bigcorp.com --root quidnug.com
$ quidnug-cli dns claim --domain bigcorp.com --root eff.quidnug
$ quidnug-cli dns claim --domain bigcorp.com --root cloudflare.quidnug
```

Three independent attestations; combined trust weight near
saturation. Phishing operators can't get all three.

### 2.3 Renewal automation

Attestations expire (typically 1 year). Automate renewal in
CI/CD:

```yaml
# .github/workflows/quidnug-renew.yml
name: Renew Quidnug DNS attestation
on:
  schedule:
    - cron: '0 0 1 * *'  # monthly check
jobs:
  renew:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - name: Check attestation status
        run: |
          quidnug-cli dns status --domain bigcorp.com \
              --exit-zero-if-valid-for "90d"
      - name: Renew if needed
        if: steps.status.outcome == 'failure'
        run: |
          quidnug-cli dns renew --domain bigcorp.com \
              --owner-key-secret ${{ secrets.QUIDNUG_KEY }}
```

Automatic renewal 30+ days before expiry. No manual
coordination needed.

## 3. Integration with QDP-0024 groups

### 3.1 Group creation

```
$ quidnug-cli groups create \
    --group-id bigcorp.com.employees \
    --type dynamic \
    --dynamic-trust-domain bigcorp.com.employees \
    --dynamic-min-trust 0.3 \
    --policy-rotation-interval 90d \
    --policy-max-members 10000 \
    --governor-key ~/.quidnug/hr-governor.key
```

Creates `GROUP_CREATE` event + initial epoch key. Members
are everyone with trust edge into `bigcorp.com.employees`
at level ≥ 0.3 (dynamic).

### 3.2 Adding members

In the dynamic case, adding a member = granting the trust
edge:

```
$ quidnug-cli trust grant \
    --truster bigcorp.com.employees \
    --trustee <new-employee-quid> \
    --level 1.0 \
    --valid-until 2027-04-20T00:00:00Z \
    --sign-with ~/.quidnug/hr-governor.key

$ quidnug-cli groups advance-epoch \
    --group-id bigcorp.com.employees \
    --reason member-added \
    --added <new-employee-quid> \
    --sign-with ~/.quidnug/group-admin.key
```

The `advance-epoch` command publishes `EPOCH_ADVANCE` with
path update + welcome package for the new member.

### 3.3 Removing members

```
$ quidnug-cli trust revoke \
    --truster bigcorp.com.employees \
    --trustee <departing-employee-quid> \
    --sign-with ~/.quidnug/hr-governor.key

$ quidnug-cli groups advance-epoch \
    --group-id bigcorp.com.employees \
    --reason member-removed \
    --removed <departing-employee-quid> \
    --sign-with ~/.quidnug/group-admin.key
```

Trust edge revocation + epoch advance. Member loses future
access within 1 block.

### 3.4 Integration with existing IAM

Most enterprises have IAM providers (Okta, AzureAD, Google
Workspace). Pattern:

1. IAM stays source of truth.
2. IAM hooks trigger Quidnug sync:
   - Onboarding hook → generate quid + grant trust edge +
     advance epoch
   - Offboarding hook → revoke trust edge + advance epoch
3. Sync is idempotent; retries on failure.
4. Reference integration in
   `integrations/iam-sync/README.md` (to be built; current
   scope).

## 4. Integration with existing DNS

BigCorp keeps their existing DNS provider (Cloudflare DNS,
AWS Route 53, internal BIND). Quidnug layers on top:

### 4.1 Public records: DNS + Quidnug in parallel

Public records can be served from *both* systems:

- Traditional DNS (Cloudflare etc.) serves the record the
  way it does today.
- Quidnug serves the same record via cryptographic
  attestation.

Clients that support Quidnug resolver get the cryptographic
version; clients that don't fall back to traditional DNS. No
migration needed.

### 4.2 Trust-gated records: Quidnug-exclusive

Partner APIs + employee directories etc. cannot be
represented in traditional DNS (DNS has no access-control
mechanism). These records are Quidnug-exclusive. Clients
must use a Quidnug-aware resolver (SDK or CLI) to query
them.

For partner integrations: BigCorp provides client libraries
to each partner. Partner integrates via library; library
does Quidnug resolution under the hood.

### 4.3 Private records: Quidnug-exclusive, encrypted

Same as trust-gated but with encryption. Employees use
internal tooling that decrypts transparently.

## 5. Gateway pattern (optional)

For clients without native Quidnug support, BigCorp can run
a Quidnug-to-DNS gateway:

```
             External clients (legacy DNS resolvers)
                             │
                             ▼
           ┌──────────────────────────────────┐
           │ Quidnug-to-DNS Gateway Service   │
           │ (runs in BigCorp's infrastructure) │
           └────────────┬─────────────────────┘
                        │
                        │ Quidnug queries
                        ▼
           ┌──────────────────────────────────┐
           │ BigCorp's Quidnug validators     │
           └──────────────────────────────────┘
```

Gateway:

- Accepts legacy DNS queries (UDP/TCP port 53).
- For each query, does a Quidnug resolution with
  `client_quid=<gateway-quid>`.
- Returns whatever records the gateway's trust level grants
  it (typically only public tier for external resolvers;
  trust-gated if configured with appropriate trust edges).

Gateway is convenience; not required for mainstream adoption.

## 6. Integration with interbank-wire-authorization

BigCorp (hypothetically a bank) is already running the
interbank-wire consortium. Adding DNS authority:

- **Same validators** serve wire authorization + DNS.
- **Same governance** quorum adopts both.
- **Same guardian quorums** protect keys.
- **Same monitoring + alerting** covers both.

Specifically:

```
bigcorp.com                     (new: attested by root)
├── wires.outbound              (existing: wire authorization)
├── wires.inbound               (existing)
├── signatories                 (existing)
├── audit                       (existing)
├── peering.counterparty-banks  (existing)
├── public                      (new: public DNS records)
├── partners                    (new: trust-gated records)
├── employees                   (new: private group)
└── board                       (new: private group)
```

The tree grows; everything else stays.

## 7. Integration with content authenticity

If BigCorp publishes media (press releases, product
announcements), they can use C2PA + Quidnug:

1. BigCorp's quid (DNS-attested) signs a C2PA manifest.
2. Manifest published via `integrations/c2pa/`.
3. Consumer verifies: C2PA signature resolves to a quid
   attested to `bigcorp.com`. Signal authentic.

This is subtle but powerful: phishing press releases can't
fake the C2PA chain back to the attested domain.

## 8. Observability integration

Add to existing Quidnug observability:

### 8.1 Metrics to track

| Metric | Purpose |
|---|---|
| `quidnug_dns_resolutions_total{visibility,result}` | Query volume by class |
| `quidnug_dns_resolutions_p99_seconds{visibility}` | Latency per class |
| `quidnug_group_members{group_id}` | Per-group membership count |
| `quidnug_group_epoch_rotations_total{group_id,reason}` | Rotation frequency |
| `quidnug_record_publishes_total{visibility}` | Write volume |
| `quidnug_attestation_status{domain,root}` | Attestation health |

### 8.2 Alerts

| Alert | Threshold |
|---|---|
| Attestation expiring | < 60 days remaining |
| Attestation failing renewal | 3 failed attempts |
| Group rotation frequency | > 5/day (possible membership churn attack) |
| Private record decryption failure rate | > 0.1% |
| Trust-gated rejection rate | Sudden spike (possible partner misconfiguration) |

### 8.3 Audit queries

Compliance-friendly queries:

```bash
# Who accessed the employee directory in the last 30 days?
quidnug-cli audit queries \
    --domain bigcorp.com \
    --record-type "DIRECTORY/*" \
    --since "30d"

# When was the last board record published?
quidnug-cli events list \
    --domain bigcorp.com \
    --event-type ENCRYPTED_RECORD \
    --filter 'payload.groupId == "bigcorp.com.board"' \
    --limit 10

# All offboardings in the last quarter
quidnug-cli events list \
    --domain bigcorp.com.employees \
    --event-type EPOCH_ADVANCE \
    --filter 'payload.reasonCode == "member-removed"' \
    --since "90d"
```

## 9. Migration from legacy systems

### 9.1 From AD DNS (Microsoft Active Directory integrated DNS)

- AD DNS stays running during migration.
- Quidnug + AD DNS serve the same records in parallel.
- New Quidnug-aware clients automatically prefer Quidnug.
- Legacy clients continue using AD DNS via standard fallback.
- Migration cost: low; primarily CLI tooling + training.

### 9.2 From BIND + RBAC scripts

- BIND serves traditional DNS.
- RBAC on authoritative access moves to Quidnug trust edges.
- Split-horizon policy migrates to `AUTHORITY_DELEGATE`
  visibility rules.
- Scripts replaced by signed events.

### 9.3 From Consul / etcd / service mesh

- Service discovery moves to trust-gated records.
- Service secrets move to private encrypted records.
- mTLS cert distribution moves to signed TLSA records
  (DANE-style).

## 10. Testing + validation

### 10.1 Staging environment

Mirror production with a staging network:

```
$ quidnug-cli domain register \
    --name bigcorp-staging.test \
    --validators <staging-validators> \
    --governors <staging-governors>
```

Point a staging DNS name (`staging.bigcorp.com` or a
dedicated staging TLD) at the staging Quidnug network.
Exercise all visibility classes, group rotations, partner
onboarding flows.

### 10.2 Smoke tests

Automated daily:

```
#!/bin/bash
# Daily smoke test
set -e

# Public record exists + resolvable
quidnug-cli dns resolve bigcorp.com A --expect 203.0.113.42

# Trust-gated record exists for partner
quidnug-cli dns resolve bigcorp.com API/v2 \
    --client-quid <test-partner-quid> \
    --expect-non-null

# Private record decryptable by employee
quidnug-cli dns resolve bigcorp.com DIRECTORY/employees \
    --client-quid <test-employee-quid> \
    --expect-non-null

# Non-member gets NXDOMAIN for private record
quidnug-cli dns resolve bigcorp.com DIRECTORY/employees \
    --client-quid <random-quid> \
    --expect NXDOMAIN
```

### 10.3 Penetration testing

Annual penetration test covering:

- Can an attacker read trust-gated records without the edge?
- Can a departing employee read records from the new epoch?
- Can a gateway server be bypassed?
- Can TLS fingerprint continuity be spoofed?

## 11. References

- [QDP-0023: DNS-Anchored Identity Attestation](../../docs/design/0023-dns-anchored-attestation.md)
- [QDP-0024: Private Communications & Group-Keyed Encryption](../../docs/design/0024-private-communications.md)
- [QDP-0014: Node Discovery + Domain Sharding](../../docs/design/0014-node-discovery-and-sharding.md)
- [`README.md`](README.md) — use-case overview.
- [`architecture.md`](architecture.md) — data model details.
- [`operations.md`](operations.md) — deployment + runbooks.
- [`threat-model.md`](threat-model.md) — attack analysis.
- [`UseCases/interbank-wire-authorization/integration.md`](../interbank-wire-authorization/integration.md)
  — companion integration doc for banks.
- [`deploy/public-network/`](../../deploy/public-network/) — reference deployment.
