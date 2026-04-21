# Enterprise domain authority, POC demo

Runnable proof-of-concept for the
[`UseCases/enterprise-domain-authority/`](../../UseCases/enterprise-domain-authority/)
use case. Demonstrates split-horizon records (public /
trust-gated / private) on a single zone stream.

## What this POC proves

BigCorp as a worked example: one zone, three visibility tiers,
three observer profiles (outsider / partner / employee). Key
claims:

1. **Three-tier visibility works on a single stream.** Every
   record event carries a `visibility` field. The resolver
   applies a per-tier decision without needing separate
   public / partner / internal DNS installations.
2. **Trust-gated records require a trust-path.** An outsider
   with no trust in BigCorp's partners-group gets NXDOMAIN on
   `api.bigcorp.com`. A partner with the trust edge gets the
   record.
3. **Private records are group-membership gated.** The
   `internal.bigcorp.com` record is visible to members of
   `bigcorp-employees` and invisible to everyone else.
4. **Membership is a signed event stream.** Adds and removes
   are both events on the group's own stream; the resolver
   replays them to compute the current set.
5. **Policy is consumer-configurable.** Trust thresholds are
   knobs; a stricter enterprise raises them, a looser one
   lowers.

## What's in this folder

| File | Purpose |
|---|---|
| `split_horizon.py` | Pure decision logic. `ERecord`, `VisibilityPolicy`, `query_record`, `query_zone`, stream extractors. |
| `split_horizon_test.py` | 13 pytest cases: public visibility, trust-gated accept/reject, threshold tuning, malformed visibility, private accept/reject, unknown scheme, zone-wide filter, extraction. |
| `demo.py` | End-to-end runnable against a live node. Five steps: register, publish three-tier records, build trust + membership graphs, run three observers. |

## Running

```bash
# 1. Start a local node.
cd deploy/compose && docker compose up -d

# 2. Install Python SDK.
cd clients/python && pip install -e .

# 3. Run the demo.
cd examples/enterprise-domain-authority
python demo.py
```

## Testing without a live node

```bash
cd examples/enterprise-domain-authority
python -m pytest split_horizon_test.py -v
```

## QDP catalog audit

| Feature | Purpose | Status |
|---|---|---|
| `IDENTITY` tx | Register governor, zone, trust-groups, observers | v1.0 |
| `TRUST` tx | Mutual trust between partner org and partners-group quid | v1.0 |
| `EVENT` tx streams | edns.record-published, group.member-added/removed | v1.0 |
| QDP-0002 guardian recovery | Governor key recovery | v1.0 (not exercised) |
| QDP-0023 DNS attestation | Anchor the enterprise's existing DNS name to the governor quid | Phase 1 landed |
| QDP-0024 group encryption | Encrypt private-tier records to the group | Phase 1 landed; POC simulates the decision layer, real deployment would wire in pkg/crypto/groupenc |

No protocol gaps.

## What a production deployment would add

- **Real group encryption.** The POC's `private:` tier is
  decision-only; a production deployment wires in
  `pkg/crypto/groupenc` so the record's value is encrypted to
  the group's current epoch secret at publish time. Non-member
  resolvers see ciphertext; members decrypt.
- **QDP-0023 DNS attestation.** Anchor `bigcorp.com` to the
  governor quid so legacy DNS clients can still verify that
  the right entity controls the name.
- **AUTHORITY_DELEGATE** event that tells resolvers which
  Quidnug nodes host this zone, with visibility-specific
  routing (public cache ring vs partner ring vs employee ring).
- **Governance quorums on the governor role.** The POC has a
  single governor; production uses QDP-0012 governance with
  threshold signing on record publication.
- **Regional sharding.** Different regions serve the zone
  from different cache tiers per QDP-0014.

## Related

- Use case: [`UseCases/enterprise-domain-authority/`](../../UseCases/enterprise-domain-authority/)
- Related POC: [`examples/dns-replacement/`](../dns-replacement/)
  is the public-tier baseline; this POC extends it with
  trust-gated and private tiers.
- Protocol: [QDP-0023 DNS Attestation](../../docs/design/0023-dns-attestation.md)
- Protocol: [QDP-0024 Group Encryption](../../docs/design/0024-group-encryption.md)
