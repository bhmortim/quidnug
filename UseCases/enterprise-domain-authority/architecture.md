# Enterprise domain authority architecture

> Detailed data model, `AUTHORITY_DELEGATE` payload structure,
> and group-key management for enterprise split-horizon
> domains. Companion to [`README.md`](README.md).

## 1. Data model overview

Every enterprise domain after attestation + delegation has
five moving parts:

1. **Root DNS attestation** (per QDP-0023) — the signed
   binding from DNS name to quid. Lives on the federated
   attestation root's ledger.
2. **Authority delegation** (per QDP-0023 `AUTHORITY_DELEGATE`)
   — the signed "who serves records" pointer. Lives on the
   owner's control domain.
3. **Record events** (per standard Quidnug event schema) —
   the actual record data. Lives on the delegated domain's
   event stream.
4. **Trust-gate edges** (per standard TRUST transactions) —
   what controls access to `trust-gated:*` records. Lives on
   the access-control domains (`<domain>.partners`, etc.).
5. **Group state** (per QDP-0024) — the TreeKEM groups that
   back `private:*` records. Lives on per-group governance
   domains.

## 2. Domain tree

A typical BigCorp deployment post-launch:

```
bigcorp.com                             (root; attested by root)
├── public                              (public records subdomain)
├── partners                            (trust-gate domain)
│   ├── [partner-1-quid] ← trust edge
│   ├── [partner-2-quid] ← trust edge
│   └── ...
├── employees                           (employee group root)
├── contractors                         (contractor group root)
├── executives                          (executive subgroup)
├── board                               (board group)
├── governance                          (governor quorum domain)
│   ├── dns-policy                      (governance over delegation)
│   ├── group-policy                    (governance over groups)
│   └── fee-policy                      (if BigCorp runs internal attestation tier)
└── consortium                          (validators' attestation domain)
```

Most of this tree is already present if BigCorp is running
any prior Quidnug use case (interbank wires, reviews, agent
authorization). Adding DNS authority adds mostly just the
`.public`, `.partners`, `.employees`, `.board` subdomains
plus the `AUTHORITY_DELEGATE` event.

## 3. The `AUTHORITY_DELEGATE` event (canonical form)

Generic form defined in QDP-0023 §3.3. Full enterprise
example:

```json
{
  "id": "<event-id>",
  "type": "AUTHORITY_DELEGATE",
  "trustDomain": "bigcorp.com.governance.dns-policy",
  "timestamp": 1729468800000000000,
  "subjectId": "bigcorp.com",
  "subjectType": "DOMAIN",
  "sequence": 1,
  "eventType": "AUTHORITY_DELEGATE",
  "payload": {
    "attestation_ref": "<dns-attestation-event-id>",
    "attestation_kind": "dns",
    "subject": "bigcorp.com",
    "delegate_nodes": [
      "a1b2c3d4e5f67890",
      "1234567890abcdef",
      "fedcba0987654321"
    ],
    "delegate_domain": "bigcorp.com",
    "visibility": {
      "record_types": {
        "A":           {"class": "public"},
        "AAAA":        {"class": "public"},
        "MX":          {"class": "public"},
        "NS":          {"class": "public"},
        "CNAME":       {"class": "public"},
        "CAA":         {"class": "public"},
        "TXT/public":  {"class": "public"},
        "TXT/SPF":     {"class": "public"},
        "TXT/DKIM*":   {"class": "public"},
        "TXT/DMARC*":  {"class": "public"},
        "API/*":       {
          "class": "trust-gated",
          "gate_domain": "bigcorp.com.partners",
          "min_trust": 0.5
        },
        "CREDENTIAL/*": {
          "class": "trust-gated",
          "gate_domain": "bigcorp.com.partners",
          "min_trust": 0.8
        },
        "INTERNAL/*":  {
          "class": "private",
          "group_id": "bigcorp.com.employees",
          "encryption": "mls-x25519-aes256gcm"
        },
        "DIRECTORY/*": {
          "class": "private",
          "group_id": "bigcorp.com.employees"
        },
        "BOARD/*":     {
          "class": "private",
          "group_id": "bigcorp.com.board"
        }
      },
      "default": {
        "class": "trust-gated",
        "gate_domain": "bigcorp.com.partners",
        "min_trust": 0.5
      }
    },
    "fallback_public": false,
    "effective_at": 1729468800000000000,
    "valid_until": 1792540800000000000
  },
  "signature": "...",
  "publicKey": "<bigcorp-owner-pubkey>"
}
```

Interpretation:

- **`delegate_nodes`**: the three validator quids that serve
  records authoritatively. Clients prefer these for fresh
  reads. Cache replicas everywhere store ciphertext/trust-
  gated data and serve per policy.
- **`delegate_domain`**: all record events for `bigcorp.com`
  flow through this Quidnug domain. BigCorp's existing
  consortium (from the wire-authorization deployment) hosts
  the domain.
- **`visibility.record_types`**: per-record-type policy.
  Granular down to "record type + subtype pattern." Wildcard
  matches apply.
- **`default`**: fallback when a record's type doesn't match
  any explicit rule. Safe default (trust-gated) prevents
  accidental public leakage of new record types.
- **`fallback_public: false`**: if `delegate_nodes` are
  unreachable, don't fall back to serving public records
  from cache tier. Stricter availability but prevents stale-
  cache leakage during delegation issues.

## 4. Record event schema

Standard Quidnug `EVENT` transaction with a record-specific
payload. Example public record:

```json
{
  "type": "EVENT",
  "trustDomain": "bigcorp.com",
  "timestamp": 1729468800000000000,
  "subjectId": "bigcorp.com",
  "subjectType": "DOMAIN",
  "sequence": 42,
  "eventType": "DNS_RECORD",
  "payload": {
    "recordType": "A",
    "name": "@",
    "value": "203.0.113.42",
    "ttl": 300,
    "visibility_class": "public"
  }
}
```

Trust-gated record:

```json
{
  "type": "EVENT",
  "trustDomain": "bigcorp.com",
  "sequence": 43,
  "eventType": "DNS_RECORD",
  "payload": {
    "recordType": "API/v2",
    "name": "_api.partners",
    "value": "https://api.bigcorp.com/v2/",
    "ttl": 3600,
    "visibility_class": "trust-gated",
    "gate_domain": "bigcorp.com.partners",
    "min_trust": 0.5
  }
}
```

Private record (encrypted payload):

```json
{
  "type": "EVENT",
  "trustDomain": "bigcorp.com",
  "sequence": 44,
  "eventType": "ENCRYPTED_RECORD",
  "payload": {
    "groupId": "bigcorp.com.employees",
    "epoch": 7,
    "contentType": "dns-record",
    "nonce": "a1b2c3d4e5f6...",
    "ciphertext": "...",
    "aad": ""
  }
}
```

The `ENCRYPTED_RECORD` wraps the inner DNS-record payload.
Plaintext (after decryption):

```json
{
  "recordType": "DIRECTORY/employees",
  "name": "_directory",
  "value": "[full JSON directory...]",
  "ttl": 1800
}
```

## 5. Resolver flow for each visibility class

### 5.1 Public record query

```
1. Client queries: resolve "bigcorp.com/A"
2. Resolver consults attestation roots; gets DNS_ATTESTATION
   for bigcorp.com → <bigcorp-owner-quid>.
3. Resolver consults authority delegation; gets 
   AUTHORITY_DELEGATE pointing to bigcorp.com domain +
   3 validator nodes.
4. Resolver queries any of 3 validators OR any cache replica
   (since record is public).
5. Cache/validator returns: EVENT with visibility_class=public,
   A=203.0.113.42, signed by bigcorp-owner-quid.
6. Resolver verifies signature, returns record to client.
```

Cost: 1-3 network hops (can short-circuit via cache). Same
latency budget as DNS + DNSSEC.

### 5.2 Trust-gated record query

```
1. Client queries: resolve "bigcorp.com/API/v2" with
   client_quid=<partner-quid>.
2. Resolver gets attestation + delegation.
3. Cache replica receives query; sees visibility_class=trust-gated,
   gate_domain="bigcorp.com.partners", min_trust=0.5.
4. Cache replica computes:
     weight = GetTrustLevel(
         client_quid = <partner-quid>,
         subject_domain = "bigcorp.com.partners",
         max_depth = 5
     )
5. If weight >= 0.5: return record.
   Else: return NXDOMAIN (indistinguishable from no such record).
6. Partner receives API endpoint; uses it.
```

Non-partners get NXDOMAIN, not "access denied" (preventing
probing for existence of trust-gated records).

### 5.3 Private record query

```
1. Employee queries: resolve "bigcorp.com/DIRECTORY/employees".
2. Resolver gets attestation + delegation.
3. Cache replica sees visibility_class=private, group_id=
   "bigcorp.com.employees".
4. Cache replica doesn't need to verify membership — ciphertext
   is returned regardless, but only group members can decrypt.
5. Client checks: am I a member of bigcorp.com.employees at
   epoch 7? (Consults group state events.)
6. If yes: derive K_epoch_7 via TreeKEM tree walk, decrypt the
   record, return plaintext.
7. If no: decryption fails. Client treats as NXDOMAIN.
```

Cache serves encrypted data freely (no crypto on the hot
path). Decryption happens client-side. Membership checks are
also client-side (TreeKEM state is stored per member).

## 6. Group management for private records

### 6.1 Groups backing DNS visibility

Typical BigCorp groups:

| Group ID | Type | Members | Records encrypted |
|---|---|---|---|
| `bigcorp.com.employees` | dynamic | All quids with trust edge into `bigcorp.com.employees` ≥ 0.3 | DIRECTORY/*, INTERNAL/* |
| `bigcorp.com.contractors` | dynamic | Trust edges in `bigcorp.com.contractors` | INTERNAL/* (subset) |
| `bigcorp.com.executives` | hybrid | CEO + CFO + CTO + ... (static) ∪ dynamic (officer role) | EXEC/* |
| `bigcorp.com.board` | static | 7 board-member quids | BOARD/* |
| `bigcorp.com.security-team` | static | CISO + 5 deputies | SEC_INCIDENT/* |

### 6.2 Onboarding a new employee

See [`operations.md`](operations.md) §3 for the full runbook.
Short version:

1. New employee generates a quid (via corporate onboarding
   portal or directly on their device).
2. Employee publishes their `MEMBER_KEY_PACKAGE` event
   (QDP-0024 §6.3).
3. HR governance signs a TRUST edge from `bigcorp.com.employees`
   to the new employee at level 1.0.
4. Since the `bigcorp.com.employees` group is dynamic with
   threshold 0.3, the employee is auto-added on next epoch.
5. A group member (typically IT ops) signs the
   `EPOCH_ADVANCE` event adding the new employee to the tree.
6. New employee receives welcome package; decrypts to derive
   current epoch key.
7. Employee can now decrypt all records published from this
   epoch forward.
8. Optional: group members choose to re-wrap historical epoch
   keys for the new employee so they can read past records
   (useful for role-based onboarding that needs history
   access).

### 6.3 Offboarding an employee

1. HR governance signs TRUST edge revocation (or lets the
   edge expire via QDP-0022 `ValidUntil`).
2. Next `EPOCH_ADVANCE` blanks the departing employee's
   leaf; path updates.
3. Departing employee cannot derive new epoch keys.
4. Past-epoch keys they hold remain valid for reading
   records written while they were a member (by design).
5. Optional (compromise response): force immediate epoch
   advance if the employee is suspected of exfiltration.

Offboarding takes < 1 block interval to take effect on new
records. No coordination with 6 separate systems.

### 6.4 Partner onboarding

Partners don't join groups; they receive a TRUST edge:

1. Partner registers their org quid (via their own Quidnug
   deployment or as a sub-identity under a partnership
   registration).
2. BigCorp governance quorum signs a TRUST edge from
   `bigcorp.com.partners` to the partner at a negotiated
   level (e.g., 0.8 for deeply-integrated partners, 0.5 for
   standard integration).
3. Partner can now query trust-gated records with their
   quid.
4. Partnership agreements carry a `validUntil` per QDP-0022,
   so the trust edge auto-expires at contract end.

## 7. Multi-region considerations

Large enterprises have multi-region deployments. QDP-0014
sharding handles this naturally:

### 7.1 Cache tier spread

Caches replicas in each region (US-East, US-West, Europe,
Asia-Pac). Public records served from nearest cache.
Trust-gated records require trust-graph walk; caches can
short-circuit via local graph state or defer to validators.
Private records served from any cache (ciphertext; client-
side decrypt).

### 7.2 Validator placement

BigCorp's 3 validators (from the wire-authorization
deployment) might be in US-East / US-West / Europe.
Authoritative writes happen at validators; gossip replicates
to cache tier within a block interval.

### 7.3 Regional policy variance

Some records may have region-specific visibility (e.g., EU
employees can read EU-only records but not US HR records).
Implemented via sub-groups:

- `bigcorp.com.employees.eu` (EU staff only)
- `bigcorp.com.employees.us` (US staff only)
- `bigcorp.com.employees` (all staff; superset group)

Records encrypted with the appropriate subgroup's key.

## 8. Interaction with TLS / certificate infrastructure

BigCorp's TLS certs are issued by standard CAs. The DNS
attestation captures the TLS fingerprint at verification
time (QDP-0023 §4.3). On cert rotation:

1. BigCorp gets new cert (normal CA flow).
2. On next DNS attestation renewal, new TLS fingerprint is
   captured.
3. The renewal event's `fingerprintRotationProof` field
   includes the CT-log entry chain connecting old to new
   cert (proves legitimate rotation vs. adversary swap).
4. Attestation renews successfully.

For TLSA (DANE-style) records, BigCorp can also publish
their TLS public-key hash as a `TXT/TLSA` public record.
Relying parties that support DANE validate the TLS cert
against this published key, bypassing the CA entirely.

## 9. Interaction with identity management

Enterprise identity lives in HR systems + IAM (Okta,
AzureAD, etc.). Integration pattern:

1. IAM remains source of truth for "who works here."
2. HR onboarding provisions a Quidnug quid for each new
   hire + publishes trust edge in `bigcorp.com.employees`.
3. IAM offboarding revokes the trust edge.
4. Quidnug group membership derives automatically from the
   trust edges (dynamic groups).

For smaller orgs without IAM integration, everything can be
manual (CLI-based).

## 10. Observability + audit

Every operation on the enterprise domain is a signed event.
BigCorp can run standard Quidnug observability (QDP-0018):

- Per-operator hash-chained audit log.
- Periodic on-chain anchoring.
- Five verification endpoints.
- Standardized metric label set.

Compliance officers get a single query target for "who
accessed what when." Regulatory queries resolve in seconds
rather than weeks.

## 11. References

- [QDP-0023: DNS-Anchored Identity Attestation](../../docs/design/0023-dns-anchored-attestation.md)
- [QDP-0024: Private Communications & Group-Keyed Encryption](../../docs/design/0024-private-communications.md)
- [QDP-0012: Domain Governance](../../docs/design/0012-domain-governance.md)
- [QDP-0014: Node Discovery + Domain Sharding](../../docs/design/0014-node-discovery-and-sharding.md)
- [QDP-0018: Observability + Tamper-Evident Operator Log](../../docs/design/0018-observability-and-audit.md)
- [`README.md`](README.md) — use-case overview.
- [`operations.md`](operations.md) — deployment + runbooks.
- [`threat-model.md`](threat-model.md) — attack analysis.
