# Quidnug Design Proposals

This directory holds the versioned decision records for the
Quidnug protocol. Each numbered file specifies a change to the
protocol, an ecosystem convention, or the process itself. The
process is defined in [QDP-0000: The QDP Process](0000-qdp-process.md).

To start a new proposal, copy [TEMPLATE.md](TEMPLATE.md),
rename it to `NNNN-kebab-slug.md` using the next unused
integer, and open a PR.

**Next free QDP number:** 0025

## Index

| #    | Title                                                                                      | Status         | Track                                    |
|------|--------------------------------------------------------------------------------------------|----------------|------------------------------------------|
| 0000 | [The QDP Process](0000-qdp-process.md)                                                     | Landed         | Meta                                     |
| 0001 | [Global Nonce Ledger](0001-global-nonce-ledger.md)                                         | Draft          | Protocol (hard fork)                     |
| 0002 | [Guardian-Based Recovery](0002-guardian-based-recovery.md)                                 | Draft          | Protocol (hard fork)                     |
| 0003 | [Cross-Domain Nonce Scoping](0003-cross-domain-nonce-scoping.md)                           | Draft          | Protocol (hard fork, bundled with 0001)  |
| 0004 | [Phase H Roadmap](0004-phase-h-roadmap.md)                                                 | Draft          | Protocol + infrastructure                |
| 0005 | [Push-Based Gossip (H1)](0005-push-based-gossip.md)                                        | Draft          | Protocol                                 |
| 0006 | [Guardian-Consent Revocation (H6)](0006-guardian-resignation.md)                           | Draft          | Protocol                                 |
| 0007 | [Lazy Epoch Propagation (H4)](0007-lazy-epoch-propagation.md)                              | Draft          | Protocol                                 |
| 0008 | [Snapshot K-of-K Bootstrap (H3)](0008-kofk-bootstrap.md)                                   | Draft          | Protocol                                 |
| 0009 | [Fork-Block Migration Trigger (H5)](0009-fork-block-trigger.md)                            | Draft          | Protocol                                 |
| 0010 | [Compact Merkle Proofs (H2)](0010-compact-merkle-proofs.md)                                | Draft          | Protocol (soft fork)                     |
| 0011 | [Client Libraries and Integrations Roadmap](0011-client-libraries-and-integrations.md)     | Landed         | Ecosystem                                |
| 0012 | [Domain Governance](0012-domain-governance.md)                                             | Phase 1 landed | Protocol                                 |
| 0013 | [Network Federation Model](0013-network-federation.md)                                     | Draft          | Protocol + architecture                  |
| 0014 | [Node Discovery and Domain Sharding](0014-node-discovery-and-sharding.md)                  | Draft          | Protocol + ops                           |
| 0015 | [Content Moderation and Takedowns](0015-content-moderation.md)                             | Phase 1 landed | Protocol + ops + legal                   |
| 0016 | [Abuse Prevention and Resource Limits](0016-abuse-prevention.md)                           | Phase 1 landed | Protocol + ops                           |
| 0017 | [Data Subject Rights and Privacy](0017-data-subject-rights.md)                             | Phase 1 landed | Protocol + ops + legal                   |
| 0018 | [Observability, Audit, and Tamper-Evident Operator Log](0018-observability-and-audit.md)   | Phase 1 landed | Protocol + ops                           |
| 0019 | [Reputation Decay and Time-Weighted Trust](0019-reputation-decay.md)                       | Draft          | Protocol                                 |
| 0020 | [Protocol Versioning and Deprecation](0020-protocol-versioning.md)                         | Draft          | Protocol                                 |
| 0021 | [Blind Signatures for Anonymous Ballot Issuance](0021-blind-signatures.md)                 | Draft          | Protocol (auxiliary crypto)              |
| 0022 | [Timed Trust and TTL Semantics](0022-timed-trust-and-ttl.md)                               | Landed         | Protocol                                 |
| 0023 | [DNS-Anchored Identity Attestation](0023-dns-anchored-attestation.md)                      | Draft          | Protocol + ecosystem                     |
| 0024 | [Private Communications and Group-Keyed Encryption](0024-private-communications.md)        | Draft          | Protocol (cryptographic payload layer)   |

## Status summary

| Status           | Count | QDPs                                           |
|------------------|-------|------------------------------------------------|
| Landed           | 3     | 0000, 0011, 0022                               |
| Phase 1 landed   | 5     | 0012, 0015, 0016, 0017, 0018                   |
| Draft            | 17    | 0001-0010, 0013, 0014, 0019-0021, 0023, 0024   |

## Process

See [QDP-0000: The QDP Process](0000-qdp-process.md) for the
full lifecycle, status values, track taxonomy, and how to
take a proposal from idea to Landed.

## Related namespaces

- **QRP-NNNN** Quidnug Reviews Protocol, application-layer
  spec for the reviews ecosystem. See
  [`examples/reviews-and-comments/PROTOCOL.md`](../../examples/reviews-and-comments/PROTOCOL.md)
  for QRP-0001.
