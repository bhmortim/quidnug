# Architecture — DNS on Quidnug

Data model, record translations, resolution flow, delegation
mechanics, and gateway design.

## 1. The mental model mapping

Before any schemas: here's how every DNS concept translates.

| DNS concept | Quidnug equivalent |
|---|---|
| Root zone (`.`) | A public network's "root" consortium; more than one can exist |
| TLD (`.com`, `.quidnug`, etc.) | A registered `TrustDomain` whose governors are the TLD operators |
| Second-level domain (`example.quidnug`) | A child `TrustDomain` delegated from the TLD via `DELEGATE_CHILD` |
| Subdomain (`mail.example.quidnug`) | A child `TrustDomain` delegated from the second-level |
| Domain ownership | `Governors` map on the `TrustDomain` |
| NS records (nameservers) | Union of `NodeAdvertisement` entries for nodes serving the domain |
| Registrar | The consortium that operates the TLD (governors for `.com` equivalent) |
| Authoritative server | Any consortium member or cache replica with a fresh copy |
| Recursive resolver | A Quidnug client library that walks `TRUST` + discovery |
| Zone file | The set of `DNS_RECORD` events on the domain's stream |
| A, AAAA, MX, TXT, SRV, CNAME, PTR records | Event payloads with a `recordType` field |
| TLSA / DANE records | `DNS_TLSA_RECORD` events binding TLS keys directly |
| CAA records | `DNS_CAA_RECORD` events restricting CA issuance (mostly unused in Quidnug-native TLS) |
| DNSKEY / DS / RRSIG | Built in — every event is signed by a current governor |
| KSK / ZSK split | Not needed; governor keys rotate via `AnchorRotation` |
| DS record at parent | Implicit in the `DELEGATE_CHILD` governance tx |
| TTL | The event's `ttl` payload field + optional `validUntil` |
| SOA record | `DomainMetadataEvent` with serial, refresh, retry, expire, minimum |
| AXFR / IXFR zone transfer | `GET /api/streams/<domain-quid>/events` + k-of-k bootstrap |
| Glue records | Node-advertisement endpoints for consortium members |
| CNAME flattening | Resolver follows the target's records client-side |
| Wildcard records (`*.example.quidnug`) | `DNSWildcardEvent` with a glob pattern |

None of this requires a protocol fork. Every translation uses
existing transaction types with new, schema-locked event
payload types.

## 2. Domain lifecycle on Quidnug

Like DNS, except governance-native.

### 2.1 Register a TLD

Done by the TLD operator once. Lasts forever unless governors
explicitly retire it. Analogous to ICANN delegating `.com` to
Verisign — except the governance is cryptographic.

```
DomainRegistrationTransaction {
    name: "quidnug",
    validators: {tld-node-1: 1.0, tld-node-2: 1.0, tld-node-3: 1.0},
    governors: {tld-op-1: 0.34, tld-op-2: 0.33, tld-op-3: 0.33},
    governanceQuorum: 0.67,
    trustThreshold: 0.5,
    parentDelegationMode: "self"    // the TLD is the root of this tree
}
```

### 2.2 Register a second-level domain

An end-user wants `example.quidnug`. They submit:

```
DomainRegistrationTransaction {
    name: "example.quidnug",
    validators: {owner-node-1: 1.0},   // or empty — served by TLD consortium
    governors: {owner-quid: 1.0},
    governanceQuorum: 1.0,
    trustThreshold: 0.5,
    parentDelegationMode: "inherit"   // parent governance reaches here until delegated
}
```

The parent (`quidnug` TLD) must co-sign via a `DELEGATE_CHILD`
governance tx, transferring governance to the owner. After
the notice period (24h default), the owner is the sole
governor.

Open question: how does the parent decide to delegate? Three
policies, operator's choice:

1. **Open registration (ICANN-like)** — anyone can register
   any unclaimed name at a rate-limit. The parent's node
   auto-co-signs.
2. **Price-gated** — register a name by posting a fee (fiat
   or crypto) to the TLD operator.
3. **Application-gated** — submit a request; the TLD
   governors review and approve.

Real-world TLDs use (2) or (3); `.quidnug` could start with
(1) for low-stakes early adoption.

### 2.3 Add / update / remove records

Once a second-level is delegated, the owner publishes record
events on the domain's stream:

```
EventTransaction {
    subjectId: <example.quidnug's quid>,
    subjectType: "DOMAIN",
    sequence: N,
    eventType: "DNS_RECORD",
    payload: {
        recordType: "A",
        value: "192.0.2.1",
        ttl: 300
    }
}
```

Updates = new event at the next sequence with the same
`recordType` + name. Resolvers always return the latest.

Removal = a `DNS_RECORD_TOMBSTONE` event referencing the
earlier event's ID. Resolvers check tombstones before
returning answers.

### 2.4 Transfer a domain

Move `example.quidnug` from `owner-A-quid` to `owner-B-quid`:

```
DomainGovernanceTransaction {
    targetDomain: "example.quidnug",
    action: "UPDATE_GOVERNORS",
    proposedGovernors: {owner-B-quid: 1.0},
    proposedGovernanceQuorum: 1.0,
    governorSigs: {owner-A-quid: <sig>}    // unanimity = A alone
}
```

After 24h notice, B becomes sole governor. A can't revoke
during the notice period because only A can sign the
revocation (and A already consented). B and A both need to
be in-the-loop for the transfer — same as a DNS registrar
transfer, but cryptographic.

### 2.5 Expire / re-register

Domains don't expire in the Quidnug sense. An owner who stops
updating a domain still owns it. But a TLD's governance can
define a "lapsed" policy — e.g., after 2 years of zero
`DNS_RECORD` events, the TLD governors can republish the
domain as available.

Expiration is a policy, not a protocol feature. A TLD
operator who wants to mimic traditional DNS's yearly-
renewal model can do so. One that wants owned-forever
semantics can do that too.

## 3. DNS record payload schemas

Full catalog of `eventType: DNS_RECORD` payload shapes, one
per DNS record type we support. The design intent: any
competent DNS implementer should recognize the fields.

### 3.1 A / AAAA (IP addresses)

```json
{
    "recordType": "A",
    "name": "mail.example.quidnug",
    "value": "192.0.2.1",
    "ttl": 300
}
```

```json
{
    "recordType": "AAAA",
    "name": "mail.example.quidnug",
    "value": "2001:db8::1",
    "ttl": 300
}
```

### 3.2 MX (mail exchange)

```json
{
    "recordType": "MX",
    "name": "example.quidnug",
    "priority": 10,
    "value": "mail.example.quidnug",
    "ttl": 3600
}
```

### 3.3 TXT (free-form text)

```json
{
    "recordType": "TXT",
    "name": "example.quidnug",
    "value": "v=spf1 include:_spf.example.com ~all",
    "ttl": 3600
}
```

SPF / DKIM / DMARC all fit here.

### 3.4 CNAME

```json
{
    "recordType": "CNAME",
    "name": "www.example.quidnug",
    "target": "example.quidnug",
    "ttl": 3600
}
```

Resolver follows the target; may need to recurse.

### 3.5 SRV (service location)

```json
{
    "recordType": "SRV",
    "name": "_sip._tcp.example.quidnug",
    "priority": 10,
    "weight": 100,
    "port": 5060,
    "target": "sipserver.example.quidnug",
    "ttl": 3600
}
```

### 3.6 TLSA (DANE — the killer feature)

TLSA records in traditional DNS bind TLS keys to a domain,
letting clients verify TLS certificates against the domain's
DNSSEC-signed publication rather than (or in addition to)
CA-signed certificates. Adoption's been limited by DNSSEC
adoption.

In Quidnug, TLSA records work the same, but the signing is
built-in:

```json
{
    "recordType": "TLSA",
    "name": "_443._tcp.example.quidnug",
    "usage": 3,           // "DANE-EE" — domain's own cert, no CA required
    "selector": 1,        // public key (not full cert)
    "matchingType": 1,    // SHA-256
    "data": "<sha256-of-tls-pubkey-hex>",
    "ttl": 3600
}
```

This is the mechanism by which Quidnug-native HTTPS can
dispense with CAs entirely. Client fetches the TLSA record,
verifies the TLS server's certificate against the published
hash, and no external PKI is involved.

### 3.7 CAA (Certification Authority Authorization)

Mostly vestigial in a Quidnug world, but supported for legacy
compatibility:

```json
{
    "recordType": "CAA",
    "name": "example.quidnug",
    "flags": 0,
    "tag": "issue",
    "value": "letsencrypt.org",
    "ttl": 86400
}
```

### 3.8 PTR (reverse DNS)

```json
{
    "recordType": "PTR",
    "name": "1.2.0.192.in-addr.arpa.quidnug",
    "target": "mail.example.quidnug",
    "ttl": 86400
}
```

Reverse DNS is a weird case because the "zones" are IP-based
rather than name-based. In Quidnug, we treat each `/24` or
`/48` as its own domain, governed by whoever controls that
IP allocation. A hosting provider or ISP would govern their
reverse zones.

### 3.9 Wildcard records

```json
{
    "recordType": "DNS_WILDCARD",
    "name": "*.example.quidnug",
    "targetRecord": {
        "recordType": "A",
        "value": "192.0.2.1",
        "ttl": 300
    }
}
```

Resolver expands the wildcard at query time.

### 3.10 SOA (zone metadata)

Traditional SOA fields mapped:

```json
{
    "recordType": "SOA",
    "name": "example.quidnug",
    "primaryNS": "ns1.example.quidnug",
    "rname": "admin@example.quidnug",
    "serial": 2026042001,
    "refresh": 3600,
    "retry": 600,
    "expire": 604800,
    "minimum": 300
}
```

Most of these are only relevant for DNS-gateway integration;
Quidnug's own machinery doesn't need `refresh` / `retry`
(gossip-driven consistency handles it).

## 4. Resolution flow

Given a query like "what's the A record for
`mail.example.quidnug`?"

```
┌─────────────────────────────────────────────────────────────┐
│  Client (app, browser, DNS gateway)                         │
│                                                              │
│  1. query = { name: "mail.example.quidnug", type: "A" }     │
│                                                              │
└──────────────────────────┬──────────────────────────────────┘
                           │
                           ▼
┌─────────────────────────────────────────────────────────────┐
│  Quidnug resolver library                                   │
│                                                              │
│  2. Parse name → domain hierarchy                           │
│     [quidnug, example.quidnug, mail.example.quidnug]        │
│                                                              │
│  3. Check local cache (by (domain, recordType))             │
│     - If fresh: return immediately                          │
│     - If stale: proceed                                     │
│                                                              │
│  4. Discovery via QDP-0014:                                 │
│     GET /api/v2/discovery/domain/mail.example.quidnug       │
│     → returns:                                              │
│       - consortium: current governors + validators         │
│       - endpoints: nodes serving this domain               │
│       - blockTip: current chain head                        │
│                                                              │
│  5. Fetch most recent DNS_RECORD event for (name, type):    │
│     GET /api/v2/streams/<domain-quid>/events                │
│       ?eventType=DNS_RECORD&recordType=A&name=mail.example  │
│       .quidnug&latest=true                                  │
│     → returns a single signed event                         │
│                                                              │
│  6. Verify signature chain:                                 │
│     a. Event is signed by a current governor (from step 4) │
│     b. Governor's key is in live epoch (QDP-0007)          │
│     c. No tombstone supersedes this event                   │
│                                                              │
│  7. Return answer + TTL to client                           │
│                                                              │
└─────────────────────────────────────────────────────────────┘
```

Steady-state: one HTTPS round trip (step 4 caches at the
CDN edge per QDP-0014). Cold-start: two round trips
(discovery + stream fetch).

### 4.1 Delegation walk

When `mail.example.quidnug`'s answers live at a different
place than `example.quidnug`'s, the resolver walks the
delegation chain:

```
1. Discover `quidnug` (TLD) — find its governors + endpoints
2. Discover `example.quidnug` — verify it's delegated from `quidnug`
3. Discover `mail.example.quidnug` — verify it's delegated from `example.quidnug`
4. Fetch the DNS_RECORD event from `mail.example.quidnug`'s stream
5. Verify signature by `mail.example.quidnug`'s governor
```

Each delegation hop is a `DELEGATE_CHILD` transaction,
signed by the parent's governors. The resolver confirms
the delegation chain is intact.

In practice, cache replicas at each level serve the cached
delegation state, so the resolver rarely does a full walk
from scratch. TTLs on delegation edges are long (hours to
days).

### 4.2 Cache invalidation

Three signals invalidate a cache entry:

1. **TTL expiry** — the record's `ttl` field elapsed.
2. **Gossip push** — a new event for `(domain, recordType,
   name)` arrives via QDP-0005 push gossip.
3. **Block tip advance** — the discovery API reports a
   new block tip since the cache was populated. The cache
   can opportunistically refresh.

TTLs are an upper bound. Push gossip + block-tip-driven
refresh typically invalidate much faster.

## 5. Legacy DNS bridge

For clients that only speak UDP/TCP DNS (curl, macOS, Linux
`/etc/resolv.conf`, mail servers), a gateway translates:

```
┌──────────────────┐                ┌─────────────────────┐
│  legacy client   │─────DNS───────►│  Quidnug DNS gateway│
│  (Chrome, curl,  │    query       │                     │
│   mail server)   │                │  - receives UDP/TCP │
└──────────────────┘                │    DNS query        │
                                    │  - resolves via     │
                                    │    Quidnug API      │
                                    │  - builds DNSSEC-   │
                                    │    signed response  │
                                    │  - caches + returns │
                                    └──────────┬──────────┘
                                               │
                                               ▼ HTTPS
                                    ┌─────────────────────┐
                                    │  api.quidnug.com    │
                                    │  (Discovery +       │
                                    │   streams API)      │
                                    └─────────────────────┘
```

The gateway holds a DNSSEC signing key for the `.quidnug`
zone (administered via normal DNS tooling) and chains the
Quidnug-native signatures into DNSSEC records.

Technical note: since Quidnug domains can be updated
instantly but DNSSEC relies on periodic re-signing, the
gateway's cache TTL is bounded by the desired freshness.
Default 60 seconds is a reasonable balance.

**Multiple gateways:** geographically diverse gateways
(Cloudflare, private, regional) all serve the same
authoritative data. Unlike DNS, the "master zone" isn't held
by the gateway — it's in the Quidnug chain. The gateway is a
translation cache.

## 6. TLS / HTTPS integration

Three deployment modes, in increasing decentralization:

### 6.1 Mode A — CA + CAA (legacy-compatible)

Use Let's Encrypt or any ACME CA. The domain's CAA records
(published as `DNS_CAA_RECORD` events) restrict which CAs can
issue certificates. TLS clients verify via the existing CA PKI.

No change from today for TLS clients. The improvement is
CAA is signed at the protocol level, not DNSSEC-retrofitted.

### 6.2 Mode B — TLSA / DANE (PKI-optional)

The domain publishes a `DNS_TLSA_RECORD` binding the TLS
server's public key. DANE-aware clients (limited today:
mostly SMTP servers) verify against the published hash
instead of the CA chain.

This is where Quidnug's cryptographic binding really shines:
TLSA + Quidnug signing = self-sovereign TLS PKI. No CA
needed.

### 6.3 Mode C — Pure Quidnug-native (post-Phase-2)

A Quidnug-aware client (browser, curl) gets the TLS pubkey
directly from the resolver's response. TLS handshake
negotiates using the published key as the sole trust anchor.

This requires client-side integration beyond a DNS gateway —
the TLS library has to know to consult the TLSA record
authoritatively. A plausible path: curl / Chromium patches
(or a `HTTPS-over-quidnug` scheme).

## 7. Fees, rate limits, and economics

Unlike DNS (where ICANN sets wholesale fees and registrars
set retail fees) or ENS (where Ethereum gas is the price
floor), Quidnug-DNS fee structure is governance-defined
per-TLD.

Suggested policy for `.quidnug`:

- **First registration:** free for names ≥ 8 characters;
  rate-limited to prevent squatting (1/hour per IP at launch).
- **Premium names (< 8 chars):** auction-based via
  `DOMAIN_AUCTION` governance action (future primitive).
- **Renewal / record updates:** free (any signed tx is free
  at the protocol layer; operator rate-limits apply).
- **Transfer:** free.

Other TLDs define their own economics. A corporate TLD might
charge; a nonprofit TLD might not.

## 8. Operational data model

Quick reference for implementers.

### 8.1 The domain's quid

A domain's `TrustDomain.Name` is its FQDN. We also allocate
it a dedicated `quid` (standard 16-hex format) that's the
subject of its event stream:

```
Domain name: example.quidnug
Domain quid: sha256(governor_pubkey_concatenation)[:16]  // or any deterministic derivation
```

All `DNS_RECORD` events are emitted on this quid's stream.
The quid is not used for block production; it's just a
stream-subject handle.

### 8.2 Event sequence

Records accumulate on the stream as new events with
incrementing sequence numbers. The most recent event for a
given `(recordType, name)` tuple is authoritative. Resolvers
index client-side by `(recordType, name)` for efficient
lookup.

### 8.3 Storage cost

Per-domain storage is linear in `number of records` + `number
of updates`. A typical second-level domain with 5-10 records
updated monthly: ~120 events/year × ~200 bytes each = 24 KB.
A million domains: 24 GB. A hundred million: 2.4 TB. Cheap by
modern standards.

Archive nodes (QDP-0014 capability) hold the full history;
cache replicas hold only the latest-per-key state, which is
much smaller.

## 9. Sequence diagrams

### 9.1 Registering and populating a new domain

```
Owner          Owner's node          TLD consortium        Cache replicas
  │                 │                      │                     │
  │──register───────►                      │                     │
  │ example.quidnug │                      │                     │
  │                 │──DomainReg txn──────►                     │
  │                 │                      │                     │
  │                 │                      │◄─quorum sign────────
  │                 │                      │ DELEGATE_CHILD      │
  │                 │                      │                     │
  │                 │◄─block with delegation────────────────────┤
  │                 │                      │                     │
  │                 │───────────────────────block gossip───────►│
  │                 │                                            │
  │──publish A record───►                                        │
  │ 192.0.2.1       │                      │                     │
  │                 │──Event txn──────────►                     │
  │                 │ DNS_RECORD(A,..)     │                     │
  │                 │                                            │
  │                 │◄─block with event────────────────────────┤
  │                 │                                            │
  │                 │───block gossip──────────────────────────►│
  │                 │                                            │
  │◄─confirmation──│                                            │
  │                 │                                            │

24h later:

  Resolver
  │
  │──query────────►api.quidnug.com/api/v2/discovery/domain/example.quidnug
  │◄─endpoints─────
  │
  │──query────────►specific node /api/streams/<domain-quid>/events?type=DNS_RECORD&name=example.quidnug
  │◄─signed event──
  │
  │──verify sig────
  │   (against
  │    governor
  │    pubkey)
  │
  │ Return 192.0.2.1
```

### 9.2 Transferring ownership

```
Old owner                                 New owner
  │                                            │
  │ 1. Agree out-of-band on transfer           │
  │◄──────────────────────────────────────────►│
  │                                            │
  │ 2. New owner gives their quid ID           │
  │◄───────────────────────────────────────────│
  │                                            │
  │ 3. Old owner submits UPDATE_GOVERNORS      │
  │    (proposedGovernors = {new-owner: 1.0})  │
  │                                            │
  │                                            │
  │     Pending for 24h notice period...       │
  │                                            │
  │                                            │
  │ 4. Activation:                             │
  │    New owner now sole governor             │
  │    Old owner no longer has authority       │
  │                                            │
  │                                            │──publish records from here──►
```

### 9.3 Key compromise + recovery

```
Owner                     Guardians            Quidnug network
  │                            │                     │
  │  OH NO, key stolen!        │                     │
  │                            │                     │
  │──────contact guardians────►│                     │
  │                            │                     │
  │                            │──GuardianRecoveryInit─►
  │                            │  (time-locked)      │
  │                            │                     │
  │                            │   24h notice period │
  │                            │   ^                 │
  │                            │   │ attacker could  │
  │                            │   │ publish records │
  │                            │   │ during this     │
  │                            │   │ window          │
  │                            │                     │
  │                            │──GuardianRecoveryCommit─►
  │                            │  (new key installed)│
  │                            │                     │
  │                            │──AnchorInvalidation─►
  │                            │  (old key epoch     │
  │                            │   frozen)           │
  │                            │                     │
  │◄───new key delivered───────│                     │
  │                                                  │
  │ Publishes corrective records with new key        │
  │─────────────────────────────────────────────────►│
```

Contrast with traditional DNSSEC: key compromise requires
an emergency key rollover, registrar cooperation to update
DS records, and widespread resolver cache flushing. Often
operators just take the domain offline until it's resolved.

## 10. API surface

### 10.1 Discovery (from QDP-0014)

```
GET /api/v2/discovery/domain/{name}
GET /api/v2/discovery/node/{quid}
```

### 10.2 Stream query for records

```
GET /api/streams/{domain-quid}/events
    ?eventType=DNS_RECORD
    &recordType=A|AAAA|MX|TXT|...
    &name=<fqdn>
    &latest=true
```

Returns the latest signed event (or 404 if none).

### 10.3 CLI (for domain owners)

```bash
# Register
quidnug-cli dns register example.quidnug --owner-key <key>

# Set a record
quidnug-cli dns set example.quidnug --type A --value 192.0.2.1 --ttl 300 --key <key>

# Delete a record
quidnug-cli dns delete example.quidnug --type A --name example.quidnug --key <key>

# Query
quidnug-cli dns resolve example.quidnug --type A

# Transfer to new owner
quidnug-cli dns transfer example.quidnug --to <new-owner-quid> --key <key>

# Rotate the governor key
quidnug-cli dns rotate-key example.quidnug --new-key <new-key> --current-key <current-key>
```

## 11. Performance characteristics

Rough numbers for planning:

| Operation | Latency | Notes |
|---|---|---|
| Cold resolution | 100-200 ms | two HTTPS hops, signature verify |
| Warm resolution (cached) | 5-10 ms | local cache hit |
| CDN-cached discovery response | < 50 ms | Cloudflare edge |
| Record publish | block-interval (60s default) | signed tx on-chain |
| Record propagation to cache replicas | < 5 s | via push gossip (QDP-0005) |
| Delegation walk (3 hops) | 150-500 ms | three discovery + stream fetches |

These are comparable to DNSSEC-validating resolvers. For most
real users the perceived latency is indistinguishable from
classical DNS.

## 12. Referenced QDPs

- [QDP-0001](../../docs/design/0001-global-nonce-ledger.md)
  — replay protection for record updates.
- [QDP-0002](../../docs/design/0002-guardian-based-recovery.md)
  — the "I lost my key" story.
- [QDP-0003](../../docs/design/0003-cross-domain-nonce-scoping.md)
  — per-domain nonce scoping for independent subdomains.
- [QDP-0007](../../docs/design/0007-lazy-epoch-propagation.md)
  — epoch state for governor key rotation.
- [QDP-0012](../../docs/design/0012-domain-governance.md)
  — domain ownership and delegation.
- [QDP-0013](../../docs/design/0013-network-federation.md)
  — alternative roots, cross-root resolution.
- [QDP-0014](../../docs/design/0014-node-discovery-and-sharding.md)
  — finding the node that actually has the records.
