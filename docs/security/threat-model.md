# Quidnug — Threat Model

This document enumerates the adversary capabilities Quidnug's
protocol and SDKs are designed to defend against, the invariants
that must hold for security, and the residual risks that are
out-of-scope.

It is intentionally a **starting point**, not a completed audit.
A third-party review by Trail of Bits / NCC / Kudelski is on the
roadmap. This document's primary purpose is to give integrators
enough context to make informed deployment choices today.

**Last revised:** 2026-04-19.
**Protocol version:** 2.x (QDPs 0001–0010).
**Intended audience:** integrators, platform teams, security
reviewers.

---

## Assets we protect

| Asset | Why it matters |
| --- | --- |
| **Quid private keys** | Identity spoofing is the root harm |
| **Nonce ledger integrity** | Replay = forged action authorization |
| **Guardian set integrity** | Wrong set → wrong recovery beneficiary |
| **Block contents** | Tampered history breaks every downstream trust query |
| **Trust edges** | Corrupting edges distorts relational trust |
| **Event streams** | Revisionism / history-rewriting |
| **Gossip fingerprints** | Cross-domain trust depends on fingerprint integrity |
| **Fork-block activation state** | Controls which features every node believes active |

## Actors

| Actor | Trust | Capabilities |
| --- | --- | --- |
| Honest node operator | trusted | local read/write to their node, signs blocks |
| Normal user | partially trusted | submits signed transactions, can only sign for their own quid |
| Malicious user | untrusted | can craft arbitrary payloads, replay, withhold, DoS |
| Network adversary | untrusted | passive eavesdropping, active injection, partition |
| Compromised node | malicious | full access to one node's keys, can sign arbitrary blocks |
| Compromised guardian | partially malicious | can deny or forge recovery attempts |
| Quantum adversary (future) | out of scope | current crypto is P-256 ECDSA; post-quantum migration is QDP-0012 work |

## Invariants the protocol maintains

### I1 — Signature binding
A signed transaction is bound to the exact canonical bytes of its
content. Altering any field after signing invalidates the
signature.

**Enforcement:**
- Cross-SDK canonical-bytes interop (see `tests/interop/`).
- Every SDK's Merkle proof verifier rechecks the binding.
- Node's signature verification is exercised on every incoming
  transaction.

### I2 — Nonce monotonicity
For a given (signer, domain) pair, each submitted nonce strictly
increases. A replayed old-nonce transaction is rejected with
`NONCE_REPLAY`.

**Enforcement:**
- `internal/core/nonce_ledger.go` records the last-seen nonce per
  signer.
- Metrics `quidnug_nonce_replay_rejections_total` fires on replay
  attempts for observability.

**Residual risk:** nonce space is `int64`, so a malicious submitter
can burn the nonce budget by submitting `2^63` transactions. QDP-0001
caps effective nonce growth via periodic epoch rotation.

### I3 — Guardian quorum
Guardian-set changes and recovery commits only succeed when the
weighted signatures meet or exceed `threshold`.

**Enforcement:**
- `internal/core/guardian_apply.go` counts weighted signatures
  before accepting any update.
- Each guardian signs with their current `key_epoch`; stale
  signatures fail verification.

**Residual risk:** if a threshold's worth of guardian keys are
compromised simultaneously, the adversary can install themselves
as the new owner. Mitigation: `recovery_delay_seconds` provides a
window for the original owner to `VETO`. Recommend ≥ 72 hours in
production.

### I4 — Fork-block activation
A feature only activates after a signed fork-block with quorum
approval is accepted by the domain's validator set.

**Enforcement:** QDP-0009. Nodes gate feature code paths on
`FeatureActive(feature)` checks.

**Residual risk:** a malicious validator majority can force-activate
a feature. Mitigation: validator roster changes themselves require
quorum signatures, bootstrapping the trust.

### I5 — Block transaction-root binding (QDP-0010)
When `require_tx_tree_root` is active, every block has a
`transactions_root` = Merkle root of canonical tx hashes. Accepting
a block without this field (post-activation) is rejected.

**Enforcement:** `internal/core/block_operations.go` validates
before persisting. Metric
`quidnug_block_missing_tx_root_rejected_total` fires on rejection.

### I6 — Gossip authenticity
Every cross-domain gossip message carries a signed
`DomainFingerprint`. Receiving nodes verify before trusting.

**Enforcement:** see `internal/core/anchor_gossip.go` and the
push-gossip flow. Rate-limits on inbound gossip prevent flooding.

**Residual risk:** if a producer's key is compromised, they can
forge fingerprints in their own domain. This is detectable via the
Merkle proof mismatch and flagged to operators.

---

## Adversary models

### A1 — Dishonest node operator

An operator who runs a node and has access to its private keys
can:
- Sign arbitrary blocks as themselves.
- Refuse to serve queries.
- Present inconsistent state to different peers.

**They CANNOT:**
- Forge signatures from other quids they don't control.
- Force other nodes to accept their blocks if those nodes
  disagree (Proof-of-Trust consensus: each node independently
  decides which blocks to trust based on its relational view of
  the signer).
- Break quid-level nonce monotonicity for others.

Mitigation: other nodes in the domain continue to serve correct
state; relational trust from the dishonest operator decays via
downstream reputational effects.

### A2 — Network adversary (active MITM)

Can intercept / modify / inject / partition traffic between
nodes.

**They CANNOT:**
- Forge signatures (every transaction is signed end-to-end).
- Make a node accept a block it would otherwise reject.
- Cause permanent state divergence (gossip eventually re-sync).

Mitigation: deploy nodes behind TLS. The protocol assumes nodes
speak HTTPS; plaintext deployments are for dev/test only.

### A3 — Compromised user quid

An adversary who obtains a user's private key can sign
transactions as that user until the user rotates or recovers via
guardians.

**Detection:** the user (or their guardians) notice anomalous
transactions and initiate recovery (QDP-0002).

**Recovery window:** bounded by the `recovery_delay_seconds`
configured at guardian-set-install time.

**Residual risk:** damage done during the recovery window (e.g.
trust edges granted to a malicious party) is not automatically
undone. Operators should treat the event stream as a ledger and
manually revoke post-recovery.

### A4 — Compromised guardian subset < threshold

A compromised minority of guardians cannot force a recovery
(QDP-0002 weighted quorum), but can:
- Collude to deny recovery attempts by abstaining.
- Leak confidential guardian information (who the guardians are).

Mitigation: guardian sets should have `threshold` <
`total_weight`, leaving margin for one or two unreachable
guardians. Recommend `threshold = ceil(2/3 * total_weight)`.

### A5 — Key-exfiltration via compromised SDK

A malicious build of the SDK could exfiltrate keys on generation.

Mitigation:
- Use HSM-backed signing (`pkg/signer/hsm/`) in production so
  keys never enter process memory.
- Verify SDK releases against published checksums (planned:
  cosign signatures on releases).
- Build from source for security-critical deployments.

---

## Out of scope

These are real risks but not addressed by Quidnug's protocol
layer. Integrators must handle them at the application layer.

- **Denial-of-service via flooding.** The protocol has rate limits
  but a sufficiently large botnet can saturate any node's CPU.
  Mitigation: deploy behind a DDoS-absorbing edge (Cloudflare,
  AWS Shield).
- **Replay of signed EVENTs across domains.** A signed EVENT in
  domain A is structurally valid in domain B. Every signed tx
  explicitly carries its `trustDomain` field; applications must
  check the domain matches their expectation.
- **Key theft from endpoint devices.** A stolen laptop → stolen
  Quid. Use a HSM or hardware-backed Keystore (see
  `clients/android/AndroidKeystoreSigner`).
- **Social engineering to install a malicious guardian set.**
  Treat guardian-set-update UIs with the same scrutiny you'd give
  a password reset. Out-of-band confirmation is recommended.
- **Post-quantum cryptography.** P-256 is classical ECDSA.
  Migration to NIST PQC (ML-DSA / Dilithium) is QDP-0012 work.
- **Side-channel attacks (power analysis, EM, cache timing) on
  signing operations.** Mitigation: HSM-backed signing.
- **Physical security** of nodes, HSMs, guardian devices.

---

## Secure deployment checklist

A minimum-security deployment should:

- [ ] Run every node behind TLS with a publicly-trusted certificate.
- [ ] Use HSM-backed signing (`pkg/signer/hsm/`) for node and
      operator keys in production.
- [ ] Configure guardian sets with `recovery_delay_seconds` ≥
      72 hours for any high-value identity.
- [ ] Monitor `quidnug_nonce_replay_rejections_total` and
      `quidnug_block_missing_tx_root_rejected_total` — alerts
      at the thresholds in `deploy/observability/prometheus-alerts.yml`.
- [ ] Pin SDK dependencies by exact version; audit SDK releases
      against hash-of-signed releases.
- [ ] Run fork-block activation votes deliberately — treat
      feature-gate changes as governance events.
- [ ] Back up node state + quid private keys to encrypted offline
      storage.
- [ ] Enable OpenTelemetry tracing so anomalous call patterns are
      observable.

---

## Reporting vulnerabilities

See [`SECURITY.md`](../../SECURITY.md) for the disclosure process.
90-day embargo; 30 days early access for node operators once a
CVE is cut.

## Next steps on this document

1. Third-party security audit (target: Q3 2026).
2. Formal verification of canonical-bytes determinism and
   nonce-ledger liveness (TLA+ or similar — on the roadmap).
3. Fuzzing harnesses for the Merkle proof and DER-signature
   parsers.
4. Published SBOM for every release.
5. Cosign signatures on all release artifacts.

## License

Apache-2.0.
