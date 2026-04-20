# Enterprise Domain Authority

**Infrastructure · Split-horizon · Private records · Federated-by-default**

## The problem

A large enterprise has a DNS name (`bigcorp.com`, `chase.com`,
`kp.org`, `<university>.edu`). They want:

1. **Cryptographic proof** that they control that name,
   referenceable by partners, regulators, customers, and
   other Quidnug-native services.
2. **Authoritative resolution** for their own records. Not
   "my DNS resolves to an IP address from a cache somewhere."
   Rather: "queries for records on my domain are served by
   nodes I control, and I decide what's publicly queryable
   versus restricted versus private."
3. **Split-horizon visibility.** Some records should be
   public globally (the main website IP, the MX record).
   Some should be visible only to trusted partners (partner
   API endpoints, B2B integration URLs). Some should be
   private and cryptographically unreadable to anyone outside
   the organization (employee directory, internal service
   discovery, board-level communications).
4. **Auditable change trail.** Every record modification is
   signed, stamped, attributable. No more "we don't know who
   updated that DNS record last Tuesday."
5. **Legacy compatibility.** Existing DNS stays intact. Nobody
   has to change their resolver. The Quidnug layer is additive.

Current state of the art: traditional DNS + Active Directory
DNS + corporate DNS vendors + internal secret-sharing tools +
VPN-gated service discovery. Six to eight separate systems
per enterprise, all with different ownership, different
access rules, different audit capabilities, different failure
modes.

## Why Quidnug fits

All the pieces exist in the protocol now (post-QDP-0023 +
QDP-0024) to collapse these six-to-eight systems into one:

| Need | Quidnug primitive |
|---|---|
| Cryptographic DNS-to-quid binding | QDP-0023 `DNS_ATTESTATION` |
| Authoritative-resolution handoff | QDP-0023 `AUTHORITY_DELEGATE` |
| Public records | Standard event stream on the delegated domain |
| Trust-gated records | `AUTHORITY_DELEGATE.visibility.*.trust-gated:<partners-domain>` |
| Private records | `AUTHORITY_DELEGATE.visibility.*.private:<employees-group>` + QDP-0024 encrypted events |
| Key rotation + recovery | QDP-0002 guardian recovery |
| Change audit | Signed events by construction |
| Multi-site / multi-region | QDP-0014 discovery + sharding |

The enterprise pays the one-time attestation fee (typically
$5-25 depending on TLD), delegates resolution back to their
own consortium (already operational per their Quidnug
deployment), and gets the whole stack for free.

This use case documents how to do it end-to-end.

## The reference scenario

Throughout this folder we use **BigCorp** as a running
example:

- BigCorp owns `bigcorp.com` (registered with standard
  registrar, 20 years old).
- BigCorp operates ~3,000 employees across 12 offices
  globally.
- BigCorp has ~150 partner organizations integrating with its
  APIs.
- BigCorp has an existing Quidnug consortium running the
  interbank-wire-authorization use case (three validators, 8
  cache replicas, ~50k wires/day). They want to extend that
  consortium to serve as their DNS authority rather than
  standing up a separate one.

## High-level architecture

```
                        Internet at large
                               │
                               ▼
       ┌────────────────────────────────────────┐
       │  Federated Attestation Roots           │
       │  (quidnug.com / EFF / Cloudflare)      │
       │                                        │
       │  DNS_ATTESTATION:                      │
       │    bigcorp.com → <bigcorp-owner-quid>  │
       └─────────────────┬──────────────────────┘
                         │
                         │  "for bigcorp.com, delegate to..."
                         ▼
       ┌────────────────────────────────────────┐
       │  AUTHORITY_DELEGATE:                   │
       │    bigcorp.com →                       │
       │      delegate_domain: "bank.bigcorp"   │
       │      delegate_nodes: [3 validators]    │
       │      visibility:                       │
       │        A:      public                  │
       │        MX:     public                  │
       │        TXT/public: public              │
       │        API/*:  trust-gated:partners    │
       │        INTERNAL/*: private:employees   │
       └─────────────────┬──────────────────────┘
                         │
                         ▼
    ┌─────────────────────────────────────────────────┐
    │  BigCorp's own consortium (3 validators)        │
    │  Serves records per visibility policy           │
    └─────┬─────────────┬─────────────┬───────────────┘
          │             │             │
      ┌───▼───┐     ┌───▼───┐     ┌───▼───┐
      │Public │     │Partner│     │Emp.   │
      │ cache │     │cache  │     │cache  │
      │ tier  │     │(trust-│     │(priv. │
      │       │     │ gated)│     │enc.)  │
      └───────┘     └───────┘     └───────┘
          │             │             │
          ▼             ▼             ▼
      Public        Partners      Employees
      clients       (with trust   (with group
                    edge)         membership)
```

Three tiers:

1. **Public tier**: A / MX / bare TXT records. Served from
   any cache replica globally. Same UX as today's DNS.
2. **Trust-gated tier**: Partner APIs, B2B URLs. Served to
   clients whose trust graph reaches `bigcorp.com.partners`
   above threshold. Non-qualifying clients get NXDOMAIN.
3. **Private tier**: Employee directory, internal service
   discovery, board comms. Encrypted via QDP-0024 group keys.
   Only members of `bigcorp.com.employees` (or sub-groups)
   can decrypt.

## Concrete ergonomics

### Public record (website IP)

```bash
quidnug-cli records publish \
    --domain bigcorp.com \
    --record-type A \
    --name "@" \
    --value 203.0.113.42 \
    --ttl 300 \
    --visibility public \
    --sign-with <bigcorp-owner-key>
```

Any resolver globally can query this. Cache replicas
everywhere serve it. Cryptographically verifiable.

### Trust-gated record (partner API)

```bash
quidnug-cli records publish \
    --domain bigcorp.com \
    --record-type "TXT/API" \
    --name "_api.partners" \
    --value "https://api.bigcorp.com/v2/" \
    --ttl 3600 \
    --visibility "trust-gated:bigcorp.com.partners" \
    --min-trust 0.5 \
    --sign-with <bigcorp-owner-key>
```

To query successfully, client must have a trust edge chain
reaching `bigcorp.com.partners` with weight ≥ 0.5. Every
approved partner organization has this edge (established at
partnership-agreement signing). Non-partners get NXDOMAIN
indistinguishable from a non-existent record.

### Private record (employee directory)

```bash
quidnug-cli records publish \
    --domain bigcorp.com \
    --record-type "TXT/EMP" \
    --name "_directory.employees" \
    --value-file directory.json \
    --ttl 1800 \
    --visibility "private:bigcorp.com.employees" \
    --sign-with <bigcorp-owner-key>
```

Payload is encrypted with the current epoch key of the
`bigcorp.com.employees` group (QDP-0024). Cache replicas
store ciphertext; only employees with their group leaf key
can decrypt. When an employee leaves, group rotates epoch
and the departing employee loses future access (past
records they saw remain readable — by design).

## The value proposition

| Capability | Traditional stack | Quidnug |
|---|---|---|
| Public DNS | BIND / Cloud DNS | Quidnug public records |
| Split-horizon internal DNS | BIND views / AD DNS | Quidnug trust-gated |
| Private service discovery | Consul / etcd / internal VPN | Quidnug private + QDP-0024 |
| Secret management | Vault / K8s secrets / 1Password Teams | Quidnug encrypted events |
| Change audit | Cloud provider audit logs + SIEM | Quidnug signed events (always-on) |
| Key rotation | Manual per-system | QDP-0002 guardian recovery |
| Key recovery | Varies per system | QDP-0002 guardian recovery |
| Employee offboarding | Touch ~8 systems | Remove from group; epoch rotates |
| Partner onboarding | Touch ~5 systems | Grant trust edge to partners domain |
| Multi-region availability | Per-system setup | Inherent (QDP-0014 cache tier) |
| TLS cert continuity tracking | Manual / CT-log monitoring | Attestation carries fingerprint |
| Compliance / audit export | Custom per-system | Single chain query |

**Single system to operate.** Single audit trail. Single key
management surface. Compliance teams get a single query
target. Ops teams get one on-call rotation rather than six.

## What's in this folder

- [`README.md`](README.md) — this document (high-level).
- [`architecture.md`](architecture.md) — detailed data model
  for each tier, `AUTHORITY_DELEGATE` payload structure,
  group-key management per QDP-0024.
- [`integration.md`](integration.md) — how this layers onto
  an existing Quidnug deployment (like the interbank wire
  consortium), what to reuse vs. what to add fresh.
- [`operations.md`](operations.md) — deployment topology,
  employee onboarding/offboarding runbook, partner
  onboarding runbook, incident response, multi-region ops.
- [`threat-model.md`](threat-model.md) — attack vectors,
  comparison with legacy split-horizon approaches, what the
  design defends against and explicit limits.

## Related

- [QDP-0023: DNS-Anchored Identity Attestation](../../docs/design/0023-dns-anchored-attestation.md)
- [QDP-0024: Private Communications & Group-Keyed Encryption](../../docs/design/0024-private-communications.md)
- [QDP-0012: Domain Governance](../../docs/design/0012-domain-governance.md)
- [QDP-0013: Network Federation Model](../../docs/design/0013-network-federation.md)
- [QDP-0014: Node Discovery + Domain Sharding](../../docs/design/0014-node-discovery-and-sharding.md)
- [QDP-0002: Guardian-Based Recovery](../../docs/design/0002-guardian-based-recovery.md)
- [`UseCases/dns-replacement/`](../dns-replacement/) — the parallel-namespace long-term vision this use case bypasses by piggybacking on existing DNS
- [`UseCases/interbank-wire-authorization/`](../interbank-wire-authorization/) — enterprise deployment pattern this use case extends
- RFC 9420 (MLS) — group encryption mechanics
- RFC 8615 (`.well-known` URIs)

## Who's going to build this

Realistic adoption order:

1. **Banks** already running Quidnug for interbank wires
   (per the existing use case). DNS authority is a natural
   extension with very high value (compliance + partner
   auth + internal service discovery all collapse).
2. **Universities** with `.edu` (free attestation tier).
   Transcripts + credentials + research-computing service
   discovery all fit the same model.
3. **Healthcare systems** with `.org` and HIPAA
   requirements. Private tier is the critical feature —
   PHI-bearing directory services encrypted at rest.
4. **Large SaaS companies** running multi-region
   infrastructure + partner ecosystems. Split-horizon DNS +
   partner API discovery in one system.
5. **Government agencies** with `.gov` (free tier). Public
   records + private internal coordination + inter-agency
   trust edges.

## Status

Design. Builds on QDP-0023 + QDP-0024 (both Draft). Reference
implementation scheduled for 2026-Q3 alongside the QDPs.
