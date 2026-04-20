# Enterprise domain authority threat model

> Attack vectors, comparison with legacy split-horizon
> approaches, what the design defends against, explicit
> limits. Companion to [`README.md`](README.md),
> [`architecture.md`](architecture.md),
> [`integration.md`](integration.md), and
> [`operations.md`](operations.md).

## 1. Adversary taxonomy

### 1.1 External

**A1. Internet-at-large observer.**
- Motivated by: network reconnaissance, competitive
  intelligence.
- Capability: query any public record; observe encrypted-
  record sizes and publication timing.

**A2. Targeted external attacker.**
- Motivated by: exfiltrating BigCorp data, disrupting
  operations, extortion.
- Capability: full social-engineering toolkit; infrastructure
  resources; long-term persistence.

**A3. Nation-state adversary.**
- Motivated by: geopolitical advantage.
- Capability: CA compromise, BGP hijack, registrar pressure,
  zero-days in validator stack.

### 1.2 Partner ecosystem

**A4. Compromised partner.**
- Motivated by: partner's own adversary has taken over
  partner's infrastructure and is exploiting the partnership.
- Capability: any query a legitimate partner could make
  with their trust edge + their published authentication
  material.

**A5. Ex-partner.**
- Motivated by: competitive intelligence; vindictive; or
  now working with a competitor.
- Capability: retained access to records from their active
  partnership period; expired trust edge for future access.

### 1.3 Insider

**A6. Current malicious employee.**
- Motivated by: exfiltration, insider trading, personal
  grievance.
- Capability: full access to whatever group membership
  grants them; can write records they're authorized to
  publish.

**A7. Departing / departed employee.**
- Motivated by: retention of sensitive knowledge; future
  competitive advantage; blackmail material.
- Capability: past-epoch records they had access to (by
  design, this access cannot be revoked retroactively).

**A8. Compromised employee key.**
- Motivated by: external actor has captured employee key
  material.
- Capability: employee's legitimate access rights until
  rotation.

### 1.4 Operational / structural

**A9. Compromised validator node.**
- Motivated by: varies.
- Capability: delay/censor transactions; cannot forge
  without HSM compromise; can be detected via block-
  production monitoring.

**A10. Compromised governance quorum.**
- Motivated by: nation-state-level.
- Capability: reshape groups, revoke attestations,
  reassign authority delegation.

**A11. Rogue attestation root.**
- Motivated by: disruption; sybil attacks on the trust
  ecosystem.
- Capability: issue counterfeit attestations.

## 2. Attack vectors + defenses

### 2.1 Attestation-layer attacks

**V1. Fake attestation via rogue root.**

- Attack: A11 stands up a root, attests `bigcorp.com` to an
  attacker-controlled quid.
- Defense: trust weighting. Rogue root has near-zero trust
  weight; its attestation loses in weighted comparison to
  BigCorp's legitimate multi-root attestations.
- Residual: clients that manually trust the rogue root get
  the rogue attestation. Client default-list curation keeps
  rogues out of the reference SDK defaults.

**V2. Attestation takeover via DNS/registrar compromise.**

- Attack: A2/A3 compromises BigCorp's registrar, changes
  nameservers, publishes their own `_quidnug-attest.*` TXT,
  claims a new attestation.
- Defense:
  - Multi-resolver TXT consistency check (A3 must coordinate
    globally simultaneously).
  - TLS fingerprint continuity: new attestation's TLS
    fingerprint differs from BigCorp's known rotation chain
    → rejection.
  - Multiple roots: attacker must compromise each root's
    verification path.
  - Revocation: legitimate BigCorp retains root quorum
    signatures to revoke the fraudulent attestation.
- Residual: A3-level attacker with CA cooperation + global
  resolver manipulation is a real risk. Mitigation: RLA-style
  continuous monitoring for attestation changes.

**V3. TLS cert rotation as cover for takeover.**

- Attack: attacker coordinates fresh TLS cert issuance via
  CA compromise and uses it for the well-known endpoint
  during fake attestation.
- Defense: CT-log verification. All legitimate TLS certs
  must appear in CT logs; attestation renewal proof must
  include the CT-log chain. A fresh cert without
  plausible-continuity CT-log entries is rejected.
- Residual: CT-log compromise is a separate but high-bar
  attack.

### 2.2 Delegation-layer attacks

**V4. Delegation authority hijack.**

- Attack: attacker publishes a fraudulent
  `AUTHORITY_DELEGATE` event claiming to be BigCorp.
- Defense: event signature validation. `AUTHORITY_DELEGATE`
  must be signed by the quid named in the attestation's
  `owner_quid` field. Attacker doesn't have the owner-quid's
  private key.
- Residual: if A8/A10 compromises the owner-quid key
  directly, they can redirect delegation. Mitigation:
  guardian quorum can sign revocation + new delegation.

**V5. Malicious delegate node.**

- Attack: BigCorp delegates to node X; node X's operator
  serves malicious or altered records.
- Defense: every record event is signed by the authorized
  publisher (BigCorp's record-ops quid). Delegate cannot
  forge records; can only serve or refuse to serve.
  Consortium membership is visible; BigCorp sees which node
  is misbehaving.
- Residual: delegate can delay or censor (temporary denial).
  Mitigation: multiple delegate nodes (quorum); clients
  prefer fresh data from validators over cache.

### 2.3 Trust-gated record attacks

**V6. Unauthorized access to trust-gated records.**

- Attack: A1 tries to query trust-gated partner API records
  without a trust edge.
- Defense: cache replica checks `GetTrustLevel(client_quid,
  gate_domain, min_trust)`. No edge → NXDOMAIN (not "access
  denied"; indistinguishable from nonexistent record).
- Residual: A1 can still probe for existence via indirect
  means (timing attacks, error-message differentiation);
  mitigation is constant-time responses + no error-message
  differentiation on query-time.

**V7. Partner collusion.**

- Attack: A4 partner shares their trust edge / quid with a
  third party.
- Defense: not defended. Trust edges are transferable key
  material; if partner misbehaves, their trust edge is
  revoked + partnership agreement invoked.
- Detection: monitoring for unusual query patterns from
  a partner quid.

**V8. Expired-edge exploitation.**

- Attack: A5 ex-partner continues querying records using
  an expired trust edge; cache tier fails to enforce TTL.
- Defense: QDP-0022 `ValidUntil` enforcement. Cache replicas
  check `ValidUntil` against local clock; expired edges are
  skipped.
- Residual: brief window of cache staleness (< TTL). Mitigation:
  short TTL on sensitive trust edges.

### 2.4 Private record attacks

**V9. Group membership analysis.**

- Attack: A1 observes which quids are in which groups by
  analyzing epoch-advance events + member-key-package
  publications.
- Defense: partial. Group membership is public metadata by
  design; encrypted-record *content* is private.
- Residual: org-chart inference is possible. Mitigation:
  opaque group naming (`group-a3f2b9`), periodic reshuffling,
  or off-chain communication for truly sensitive structure.

**V10. Post-membership access retention.**

- Attack: A7 departing employee retains past-epoch keys;
  reads records from epochs they were a member of.
- Defense: by design, past access is preserved. Mitigation
  for sensitive data: rotate groups per-sensitivity-level,
  shorter history retention per-group policy, cryptographic
  shredding (destroy past-epoch keys when retention expires).

**V11. Compromised employee key.**

- Attack: A8 adversary has employee's X25519 private key;
  decrypts ongoing encrypted records.
- Defense: compromise-response epoch rotation. Once detected,
  the compromised key's future access is revoked within
  one epoch advance.
- Residual: records published between compromise and detection
  are exposed. Mitigation: scheduled epoch rotation (default
  90d); short rotation interval for high-sensitivity groups.

**V12. Forward-compromise attack.**

- Attack: A2 captures current epoch key; reads future
  records until they're detected.
- Defense: compromise-response rotation; post-compromise
  security ensures future keys are not derivable.
- Residual: same window issue as V11.

### 2.5 Governance attacks

**V13. Rogue group admin.**

- Attack: A6 employee with group-admin privileges forcibly
  advances epochs or excludes legitimate members.
- Defense: every `EPOCH_ADVANCE` is audited; affected
  members detect exclusion quickly.
- Mitigation: governance quorum required for certain
  sensitive actions (remove member from board group, etc.).
- Residual: temporary disruption possible; full recovery
  within hours.

**V14. Governance quorum compromise.**

- Attack: A10 takes over a majority of governors.
- Defense: no defense against majority compromise by
  design. Mitigation: guardian quorum on each governor
  key (QDP-0002) means majority-compromise requires
  compromising majority of (governor AND their guardians).
- Residual: catastrophic-compromise scenario; full system
  re-initialization required.

### 2.6 Cache + consortium attacks

**V15. Cache-stored ciphertext harvest.**

- Attack: A1 cache-replica operator (or their compromised
  counterpart) harvests all encrypted records + attempts
  offline decryption.
- Defense: AES-GCM-256 is computationally infeasible to
  break offline without the group key.
- Residual: "harvest now, decrypt later" remains a concern
  if quantum computers break X25519 key agreement
  retrospectively. Mitigation: QDP-0024 roadmap includes
  post-quantum migration.

**V16. Validator censorship.**

- Attack: A9 compromised validator refuses to include
  BigCorp's record-update transactions.
- Defense: consortium quorum (2-of-3) means one bad
  validator cannot fully censor; transactions go via the
  remaining validators.
- Residual: brief latency increase during censorship.

## 3. Comparison with legacy split-horizon approaches

| Property | BIND + views | AD DNS + VPN | Consul + Vault | Quidnug |
|---|---|---|---|---|
| Public record integrity | Unsigned; DNSSEC optional + rare | Unsigned | Internal to org | Signed by construction |
| Access-control granularity | Source IP / ACL | AD group | Service-mesh policy | Trust-graph edge |
| Audit trail | System logs (fragile) | AD event logs + SIEM | Consul audit + Vault audit + SIEM | Signed events (tamper-evident) |
| Key rotation | Manual per key | Manual | Manual | Guardian recovery primitive |
| Compromise response | Manual | Manual | Manual | Epoch rotation automated |
| Employee offboarding | ~8 systems touched | ~8 systems | ~6 systems | 1 trust revocation + 1 epoch advance |
| Partner revocation | Manual per system | Manual | Manual | 1 trust revocation |
| Private record encryption | External tool required | External tool | External tool | Native QDP-0024 |
| Multi-region | Per-system config | Per-system config | Per-system config | Native QDP-0014 |
| Cryptographic DNS ownership proof | None | None | None | QDP-0023 attestation |
| Key recovery on loss | Varies; often catastrophic | Domain admin reset | Vault recovery keys | Guardian quorum |
| Cost | Low ops, high integration | High | High | Medium (one system) |

## 4. What this design deliberately does NOT defend against

1. **Full compromise of the owner quid.** If A2 obtains the
   BigCorp owner-quid's private key + all its guardians,
   they own the domain. Mitigation relies on key-custody
   discipline (HSMs, dual control, 5-of-7 guardian quorums).

2. **Nation-state infrastructure control.** Simultaneous
   compromise of DNS, CT logs, multiple attestation roots,
   and BigCorp's governance is not defended. Design assumption:
   such actor has achieved catastrophic capability and
   cryptographic measures alone cannot help.

3. **Traffic analysis / metadata observation.** Observers
   can see that encrypted records exist, their sizes, timing,
   and which group they belong to. Applications needing
   metadata privacy layer additional primitives (cover
   traffic, rate padding, off-chain channels) on top.

4. **Physical access to employee devices.** If an attacker
   has physical access to an employee's device + extracts
   their X25519 key, they have the employee's access. Use
   device-level keystore (Secure Enclave / TPM / YubiKey)
   to raise the bar.

5. **Coercion.** An employee compelled to share group keys
   cannot be defended against by cryptography. Operational
   discipline (group admin reviews access patterns) and
   legal process apply.

6. **Supply-chain compromise.** If the Quidnug node binary
   is backdoored upstream, the threat model breaks down.
   Reproducible builds + audit discipline help; doesn't
   eliminate.

7. **Long-term cryptographic advances.** X25519 is not
   post-quantum. Future migration required. QDP-0024 §16
   flags this as open work.

## 5. Risk prioritization

For a typical enterprise deployment, risk likelihood + impact:

| Risk | Likelihood | Impact | Priority |
|---|---|---|---|
| V11 compromised employee key | Medium | Medium | **High** |
| V2 registrar/DNS compromise | Low | High | **High** |
| V9 group membership analysis | High | Low | Medium |
| V16 validator censorship | Low | Medium | Medium |
| V14 governance compromise | Very Low | Catastrophic | **High** |
| V1 rogue root | Medium | Low (low weight) | Low |
| V10 post-membership retention | Low (by design) | Low | Low |
| V5 malicious delegate | Low | Medium | Medium |

## 6. Recommended security discipline

Enterprise deployment should implement all of:

1. **HSM-backed owner + governor keys.** Non-negotiable.
2. **5-of-7 guardian quorums** on governance keys.
3. **Multi-root attestation** (≥2 independent roots).
4. **Automated CT-log monitoring** for TLS cert changes.
5. **Scheduled epoch rotation** on all private groups (90d
   default, 30d for high-sensitivity).
6. **Compromise-response runbook** rehearsed quarterly.
7. **Independent penetration testing** annually.
8. **Key-ceremony room** for key generation (air-gapped +
   paper backup).
9. **Dual-control** on sensitive operations (group admin +
   governor required for board-group changes).
10. **Third-party audit** of operational discipline annually.

## 7. References

- [`README.md`](README.md) — use-case overview.
- [`architecture.md`](architecture.md) — data model.
- [`integration.md`](integration.md) — deployment
  integration.
- [`operations.md`](operations.md) — runbooks + incident
  response.
- [QDP-0023: DNS-Anchored Identity Attestation](../../docs/design/0023-dns-anchored-attestation.md) §9 for attestation-layer attacks.
- [QDP-0024: Private Communications & Group-Keyed Encryption](../../docs/design/0024-private-communications.md) §12 for encryption-layer attacks.
