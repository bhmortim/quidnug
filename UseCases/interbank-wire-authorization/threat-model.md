# Threat Model: Interbank Wire Authorization

## Assets

1. **Wire authorization authority** — the ability to cause funds to
   move via the settlement rails.
2. **Approval audit trail** — the cryptographic record of who
   approved what, when.
3. **Officer signing keys** — HSM-protected keys whose compromise
   enables unauthorized wire approval.
4. **Bank's root quid** — the identity that controls the approval
   guardian set itself; compromising it enables swapping in
   attacker-controlled officers.

## Attackers

| Attacker              | Capability                                            | Motivation                         |
|-----------------------|-------------------------------------------------------|------------------------------------|
| External (untrusted)  | Can send HTTP requests, observe public gossip          | Theft                              |
| Insider: operator     | Has one officer's HSM; one valid signing key           | Fraud                              |
| Insider: compliance   | Has compliance HSM (weight 2); theoretically unilateral| Collusion with operator            |
| Insider: admin/IT     | Can modify node config, restart services              | Sabotage / deploy backdoor         |
| Insider: CEO/CFO      | Can authorize bank root quid updates                   | Catastrophic (mitigated separately)|
| State-level adversary | Can intercept network, subpoena key material           | Surveillance / control             |
| Supply-chain          | Compromised HSM firmware, dependency injection         | Persistent backdoor                |

## Threats and mitigations

### T1. External forgery of a wire approval

**Attack.** Attacker with no insider access tries to submit a wire
title claiming to be signed by Alice.

**Mitigation.**
- Every signature verifies against `signerKeys[alice-op][alice's
  current epoch]` — attacker doesn't have Alice's HSM.
- Monotonic nonce means even a replay of a captured valid signature
  is rejected (nonce already accepted).
- Even if the attacker has network access, the `NODE_AUTH_SECRET`
  HMAC gates inter-node traffic.

**Residual risk.** None structural. Reduces to "can the attacker
compromise an HSM" — which is the endpoint security problem, not
a protocol problem.

### T2. Captured session replay

**Attack.** Attacker intercepts Alice's valid approval for wire A
and replays it as approval for wire B.

**Mitigation.**
- The signature binds to the specific wire title's content. Replaying
  Alice's wire-A signature against wire-B produces a signature that
  doesn't match wire-B's canonical bytes.
- Anchor-nonce monotonicity: even if the attacker crafts wire B to
  have identical bytes (unlikely given amount/payee/timestamp), the
  nonce has already been accepted.

**Residual risk.** None.

### T3. Single compromised officer HSM

**Attack.** Alice's HSM is compromised. Attacker has one signing
key.

**Mitigation.**
- Wire still requires threshold 3. Alice alone = weight 1. Attacker
  also needs another compromise.
- Alice's anchor nonce is monotonic. If Alice notices unusual
  activity and rotates via her personal guardian recovery, the old
  key is immediately unable to sign new wires.
- `MaxAcceptedOldNonce` cap on rotation limits the window.

**Residual risk.** If attacker compromises Alice AND Bob before
anyone notices, they reach threshold 2 (w=1+1). Still under 3.
Need Alice+Bob+Carol or Alice+Carol or Bob+Carol to reach 3+ with
w=2 Carol. So: **two or three simultaneous compromises required
depending on who's compromised.** This is a significantly higher
bar than "one compromise."

### T4. Compliance officer collusion

**Attack.** Carol (weight=2) collaborates with Alice (weight=1) to
steal. Their combined weight (3) meets threshold.

**Mitigation (partial, organizational).**
- Weight distribution is a policy decision. The design doc's
  `threshold=3` setup specifically addresses this by requiring
  three parties (Carol + Alice + Bob) for any wire. If the bank
  wants the weaker property, it picks `threshold=2`.
- If threshold=3, Carol and Alice alone = 3 (w=2+1) *does* meet
  threshold. The fix is `threshold=4` with compliance staying at
  w=2 — requires two operators AND compliance.

**Residual risk.** Protocol cannot prevent quorum-of-allowed-parties
collusion by design. Standard corporate governance (separation of
duties, rotation, background checks) is the countermeasure.

### T5. Attacker compromises bank root quid

**Attack.** The root quid that controls the approval set is
compromised. Attacker adds their own key to the approval set and
drains the bank.

**Mitigation.**
- Root quid has its **own** guardian set (CEO, CFO, legal). A
  `GuardianSetUpdate` for `bank-us-wire` requires primary signature
  (root's current-epoch key) AND current-set consent quorum.
- Root-quid key compromise requires also compromising multiple
  executive guardians — far above the single-key-compromise bar.
- Even if achieved, the `GuardianSetUpdate` lands on the blockchain
  and the anomaly is auditable.

**Residual risk.** Low; this is the "CEO gets phished" scenario
where traditional controls (executive HSMs with in-person use,
quarterly attestation) apply.

### T6. Guardian takeover

**Attack.** Alice's personal guardians (spouse, manager, backup
HSM) are all compromised. Attacker initiates a recovery to their
own key.

**Mitigation.**
- Time-lock delay: default 1 hour, configurable up to 1 year. The
  legitimate Alice (if her key is fine) can veto during the
  window.
- `EnableLazyEpochProbe` (QDP-0007) means correspondent banks
  probe on stale signers. An unexpected recovery fires visible
  events.
- Guardian `RequireGuardianRotation` flag can be set on officer
  quids, forcing any rotation to go through the time-locked
  recovery path — so there's no "fast path" recovery for high-
  value officers.

**Residual risk.** An attacker with guardian quorum AND the ability
to prevent Alice from vetoing (e.g., Alice is on a plane for 12
hours) can complete a recovery. Mitigation: longer delay for
high-value officers; compensating monitoring.

### T7. Correspondent bank lies

**Attack.** Correspondent bank claims they received a valid
approval from us; actually they got half-signed state and are
trying to collect on an unapproved wire.

**Mitigation.**
- Wire isn't "approved" until the event stream has a quorum. The
  correspondent can re-verify our guardian set and weigh signatures
  independently — they have all the public state.
- Push gossip + fingerprints ensure both sides see the same state.

**Residual risk.** If correspondent also has their own internal
gaps, that's their problem; they can't forge our approval without
cryptographic material they don't have.

### T8. Fork-block hijack

**Attack.** A malicious quorum of consortium members passes a
fork-block lowering the compliance threshold, enabling easier theft.

**Mitigation.**
- Fork-block requires 2/3 (ceiling) of validators in the domain.
  Compromising a majority of banks' root keys is the prerequisite
  — orders of magnitude above compromising one officer.
- `MinForkNoticeBlocks = 1440` (~24h) means any change is visible
  to every participant for 24 hours before activation. Operators
  can observe a malicious fork in advance and halt their nodes.
- Post-activation, a future fork can roll back. Nothing is
  permanent unless the consortium says so.

**Residual risk.** Requires majority-validator compromise. At that
point the consortium has bigger problems.

### T9. Denial-of-service on the approval flow

**Attack.** Attacker floods the node with fake approval submissions
to keep legitimate ones from being processed.

**Mitigation.**
- IP-level rate limiting (per-IP token bucket).
- `NODE_AUTH_SECRET` gates inter-node traffic; external floods
  can't affect gossip.
- Dedup-first validation on gossip means replayed valid messages
  cost ~1 map lookup each.
- Push-gossip producer rate limiter (QDP-0005 §7) throttles any
  single party.

**Residual risk.** A well-provisioned attacker can cause resource
exhaustion against a single node, but not corrupt state.

### T10. Supply-chain compromise

**Attack.** A Quidnug dependency (Go stdlib, Gorilla mux,
Prometheus client) has a backdoor that exfiltrates signing keys.

**Mitigation.**
- Dependencies are minimal; see `go.mod`.
- Keys never leave HSMs in the designed flow. A dependency
  compromise can see signable bytes but not signing material.
- Apache-2.0 includes explicit patent grant; no custom crypto.
- Reproducible builds (TODO for the project) would help detect
  tampering.

**Residual risk.** Standard supply-chain risk level — comparable
to any Go binary running in production. Mitigated by standard
practices.

## Not defended against

Explicit limits of this design:

1. **Off-chain policy enforcement.** The protocol authorizes
   wires; it doesn't sanity-check the payee's sanctions status.
   That's OFAC / application-layer concern.

2. **Rail-level double-spends.** Quidnug says "this wire is
   approved." If your core banking system then tries to send the
   same approval to Fedwire twice, Fedwire's own idempotency keys
   should catch it. Quidnug doesn't try to control the rails.

3. **Insider with root-quid access.** If the bank's CEO actively
   participates in fraud, the protocol documents the crime
   beautifully but doesn't prevent it. Standard corporate
   governance is needed.

4. **HSM implementation bugs.** If the HSM's P-256 implementation
   has a known weakness (e.g., biased nonce during ECDSA sign),
   the protocol's correctness doesn't save you. Use good HSMs.

5. **Long-range chain rewrites.** Since Quidnug's consensus is
   relational, a node could theoretically accept a re-written
   history from a very-trusted peer. Mitigation: audit logs, the
   ledger's monotonic nonce invariants, and operator vigilance.
   Not a cryptographic guarantee the way Bitcoin's is.

6. **Confidentiality.** Wire contents are public on the consortium
   chain in this design. Production would encrypt payloads and
   store hashes on-chain. See §"Out of scope" in [`README.md`](README.md).

## Residual monitoring

Run Prometheus scrapes on:

- `quidnug_guardian_resignations_total` — spikes suggest an
  insider mass-exit.
- `quidnug_guardian_set_weakened_total` — structural vulnerability
  opening.
- `quidnug_nonce_replay_rejections_total{enforced="true"}` —
  attack attempts.
- `quidnug_gossip_rate_limited_total` — compromised peer flooding.
- `quidnug_probe_failure_total` — network partition isolating a
  correspondent (could be precursor to forged-approval attack).

## Incident response playbook

1. **Officer key suspected compromised.**
   - Page on-call HSM operator.
   - Run `POST /api/anchors` invalidation on the compromised epoch
     (freeze it). Zero tolerance for maxAcceptedOldNonce.
   - Guardian-set update removing the officer from approval quorum.
   - If no rotation possible: guardian recovery with shortest
     permitted delay.

2. **Bank root quid compromise suspected.**
   - Halt all nodes (operator switch, not protocol).
   - Re-verify guardian-set state.
   - Initiate root-quid rotation via executive guardian quorum.
   - Audit all recent guardian-set changes for unauthorized
     modifications.

3. **Unexpected fork-block transaction observed.**
   - Review signatures on the fork-block tx.
   - If quorum is legitimate: communicate with consortium.
   - If quorum involves suspect validator signatures: file an
     emergency-halt request with the consortium. Each node can
     independently refuse to activate the feature by operator
     override.

## References

- [QDP-0002 §13 Threat Model](../../docs/design/0002-guardian-based-recovery.md)
- [QDP-0001 §3.2 Attacker Model](../../docs/design/0001-global-nonce-ledger.md)
- [SECURITY.md](../../SECURITY.md)
