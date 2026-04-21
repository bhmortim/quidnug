# DNS replacement, POC demo (Phase 0)

Runnable proof-of-concept for the
[`UseCases/dns-replacement/`](../../UseCases/dns-replacement/)
use case. A minimal but end-to-end demonstration that DNS-style
resolution works over Quidnug's primitives: zones as quids,
records as signed events, governors as the authoritative
publisher set, and relational trust gating the resolver.

## What this POC proves

A zone, two governors, a resolver, a cache-poisoning attacker,
and a weak-trust observer on a shared DNS domain. Key claims:

1. **Records are signed events.** Every A / AAAA / MX / TXT /
   CNAME / SRV / TLSA record is a `dns.record-published` event
   on the zone's stream. Rotation is a `dns.record-revoked`
   followed by a new publish.
2. **Cache poisoning is structurally prevented.** A non-governor
   trying to publish a record on the zone's stream has their
   event visible, but the resolver filters out anything not
   from the governor set. No DNSSEC retrofit needed; the check
   is inherent.
3. **Rotation via revoke + publish.** The resolver always
   returns only records that haven't been superseded by a
   revocation.
4. **TTL is a policy knob.** By default the resolver trusts
   records regardless of age; with `enforce_ttl=True` it treats
   TTL-expired records as NXDOMAIN.
5. **Relational trust gates resolution.** Two resolvers with
   different trust in the same governor reach different
   verdicts: one gets `ok`, the other `indeterminate`.

## What's in this folder

| File | Purpose |
|---|---|
| `dns_resolve.py` | Pure resolver logic: `DNSRecord`, `ResolvePolicy`, `resolve`, stream extractor. |
| `dns_resolve_test.py` | 14 pytest cases: basic resolution, NXDOMAIN, record types, cache poisoning, revocation, trust gating, TTL, invalid input, round-robin. |
| `demo.py` | End-to-end runnable against a live node. Seven steps walking registration, publication, cache poisoning attempt, rotation, and weak-trust observer behavior. |

## Running

```bash
# 1. Start a local node.
cd deploy/compose && docker compose up -d

# 2. Install Python SDK.
cd clients/python && pip install -e .

# 3. Run the demo.
cd examples/dns-replacement
python demo.py
```

## Testing without a live node

```bash
cd examples/dns-replacement
python -m pytest dns_resolve_test.py -v
```

## QDP catalog audit

| Feature | Purpose | Status |
|---|---|---|
| `IDENTITY` tx | Register zone, governors, resolvers | v1.0 |
| `TRUST` tx | Resolver's trust in each governor | v1.0 |
| `EVENT` tx streams | dns.record-published / dns.record-revoked | v1.0 |
| QDP-0002 guardian recovery | Governor key rotation after compromise | v1.0 (not exercised) |
| QDP-0005 push gossip | Record propagation to caching replicas | v1.0 |
| QDP-0009 fork-block | Raise minimum-signature requirement network-wide | v1.0 (not exercised) |
| QDP-0012 domain governance | Formal governor/consortium model | v1.0 (approximated with explicit governor set) |
| QDP-0013 federation | Cross-root resolution | v1.0 (not exercised) |
| QDP-0014 node discovery | Finding a zone's authoritative node | v1.0 (not exercised) |
| QDP-0023 DNS attestation | Interoperate with legacy DNS during migration | Phase 1 landed |

No protocol gaps for Phase 0 scope.

## What Phase 1+ would add

- **Hierarchical delegation.** Phase 0 has a single zone;
  Phase 1 wires up `mail.example.quidnug` as a child zone
  delegated by `example.quidnug`'s governors, with its own
  governor set.
- **Cross-root federation.** Phase 1 exercises QDP-0013: two
  separate root consortiums each running their own
  `.quidnug` TLD, with users choosing (or combining) roots.
- **Legacy bridge.** A gateway publishes Quidnug DNS events
  in standard-DNS wire format so legacy resolvers can still
  reach a Quidnug-hosted zone.
- **TLSA + cert pinning.** A web server publishes its TLS
  public key as a TLSA record; clients pin to the record
  instead of chaining to a public CA.
- **Full governance via QDP-0012.** Governor additions,
  removals, and weighted thresholds via the Domain Governance
  primitives rather than a client-held set.

## Related

- Use case: [`UseCases/dns-replacement/`](../../UseCases/dns-replacement/)
- Related POC: [`examples/enterprise-domain-authority/`](../enterprise-domain-authority/)
  (upcoming) covers the enterprise side of the same name-
  ownership model
- Related POC: [`examples/credential-verification-network/`](../credential-verification-network/)
  has the same transitive-trust property applied to credentials
- Protocol: [QDP-0012 Domain Governance](../../docs/design/0012-domain-governance.md)
- Protocol: [QDP-0023 DNS Attestation](../../docs/design/0023-dns-attestation.md)
