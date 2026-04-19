# Threat Model: Institutional Custody

## Assets

1. **Client crypto holdings** — the primary asset. Theft is
   the catastrophic outcome.
2. **Signer keys** — HSM-protected; compromise of enough keys
   approves theft.
3. **Guardian keys** — recovery authority; compromise enables
   malicious rotation to attacker-controlled keys.
4. **Root quid key** — structural authority. Catastrophic if
   compromised (can rewrite guardian sets).

## Attackers

| Attacker                 | Capability                                        | Goal                       |
|--------------------------|---------------------------------------------------|----------------------------|
| External                 | No access to any firm infrastructure              | Steal holdings             |
| Insider: signer          | Has one signing HSM                               | Participate in theft       |
| Insider: sysadmin        | Can reboot nodes, modify config, not keys        | Subtle sabotage            |
| Insider: executive       | Guardian quorum member                           | Collusion                  |
| State adversary          | Network observer, legal compulsion               | Surveillance / seizure     |
| Supply-chain             | Compromised dependency, HSM firmware             | Persistent backdoor        |
| Social engineer          | Access to signers via email / phone              | Trick into signing         |

## Threats

### T1. Theft via compromised signer quorum

**Attack.** Attacker compromises 5 of 7 cold-wallet signers'
HSMs and triggers a transfer to their own address.

**Mitigations.**
- Cold-wallet quorum is high (5-of-7 minimum). Compromising
  5 independent HSMs is a major operation.
- Signers spread geographically and across HSM vendors —
  correlated-HSM-compromise is much harder than a single
  vendor bug.
- `requireGuardianRotation=true` on cold wallets means any
  signer-set change goes through time-locked guardian
  recovery, giving compliance / ops a window to intervene.

**Residual risk.** Determined state-level adversary with
multiple independent compromises. Mitigated by insurance +
monitoring + air-gapped cold signers.

### T2. Insider collusion

**Attack.** Three signers + compliance officer collude to move
funds to an attacker-controlled address.

**Mitigations.**
- Quorum math should be set so any single insider group can't
  meet threshold (N-1 if possible). Cold wallet with
  5-of-7 means 3 colluders can't succeed.
- Periodic rotation (every 90 days) limits the window of
  stable collusion.
- Executive-level oversight: guardian-set updates require
  root quid's guardians (CEO, CISO, COO, auditor) — a
  signer-level collusion can't amend the quorum structure.

**Residual risk.** Sufficient collusion (5+ people) defeats
any quorum. Mitigated by culture, rotation, compensation
design.

### T3. Guardian-quorum takeover

**Attack.** Attacker compromises a signer's personal guardians
(spouse, manager, backup HSM) and initiates a recovery to an
attacker-controlled key.

**Mitigations.**
- `recoveryDelay` of 7 days for cold-wallet signers. During
  this window, the legit signer (if they're fine) can veto.
- Veto mechanism: any of the personal guardians can veto a
  pending recovery if the signer has an out-of-band way to
  contact them.
- `EnableLazyEpochProbe` (QDP-0007) means unexpected rotations
  fire visible cross-subsidiary events that compliance sees.

**Residual risk.** Attacker with access to both the signer's
HSM AND sufficient personal guardians can complete a
recovery. Mitigated by physical security of backup HSMs,
guardian rotation, and periodic re-attestation.

### T4. Root quid compromise

**Attack.** CEO's phone is stolen + phished. Attacker
initiates a guardian-set update modifying the root's
structure.

**Mitigations.**
- Root guardians are {CEO, CISO, COO, external auditor} —
  2 executives + external auditor required for quorum.
- External auditor is organizationally independent — phishing
  them separately is unlikely.
- `requireGuardianRotation=true` on root means no fast-path.
- 7-day delay at root level → lots of time for anomaly
  detection.

**Residual risk.** Low; all top-level keys would have to be
simultaneously compromised.

### T5. Stale-key attack across subsidiaries

**Attack.** Attacker gets hold of a signer's OLD key (pre-
rotation) and tries to use it in a subsidiary that hasn't
observed the rotation.

**Mitigations.**
- **Push gossip (QDP-0005)** propagates rotations across
  subsidiaries in seconds to minutes.
- **Lazy epoch probe (QDP-0007)** catches the case where
  push missed a subsidiary: the receiving node probes the
  signer's home domain before accepting a stale-looking
  signature.
- **MaxAcceptedOldNonce** cap (default 100) limits the
  grace window.

**Residual risk.** Window between rotation and propagation
is short and monitorable.

### T6. Replay of a signed approval

**Attack.** Intercepted approval for transfer A replayed as
transfer B.

**Mitigations.**
- Signatures bind to the specific title's canonical content.
  Different content = different signable bytes = signature
  mismatch.
- Anchor-nonce monotonicity: same nonce can't appear twice.

**Residual risk.** None.

### T7. On-chain multi-sig contract bug

**Attack.** The Bitcoin / Ethereum multi-sig contract that
ultimately executes the transfer has a vulnerability.

**Mitigations.**
- This is the on-chain side, not Quidnug's responsibility.
  Quidnug authorizes; the on-chain contract enforces.
- Multi-sig contracts are extensively audited and battle-
  tested (Gnosis Safe, etc.).

**Residual risk.** Outside this use case's scope.

### T8. Signer ransomware

**Attack.** Attacker encrypts a signer's HSM interface and
demands ransom to unlock.

**Mitigations.**
- Signers have backup HSMs in separate locations.
- Guardian recovery lets the firm rotate the ransomware'd
  signer to a fresh HSM in under 7 days (or minutes for
  hot wallets).
- Temporary quorum reduction is possible via guardian-
  set update (4-of-6 until the 7th signer is replaced).

**Residual risk.** Minor operational disruption; no fund loss.

### T9. Fork-block abuse

**Attack.** Executive quorum pushes a fork-block that lowers
quorum thresholds, enabling a later insider attack.

**Mitigations.**
- Fork-block requires 2/3 of domain validators — this is
  the consortium governance layer, not an internal-only
  decision.
- `MinForkNoticeBlocks = 1440` (~24h) notice.
- External auditor in the executive quorum provides
  independent oversight.

**Residual risk.** Majority-executive collusion. Mitigated
by governance, insurance, and audit.

### T10. Regulatory compulsion

**Attack.** State actor subpoenas the firm's root signing keys.

**Mitigations.**
- Keys are HSM-protected; the firm cannot "hand over" keys
  directly — they can be used but not extracted.
- Transfers still require quorum; a compelled executive
  signature alone doesn't meet threshold.
- Multi-jurisdictional signers: US subsidiary's signers
  aren't subject to EU court orders and vice versa.

**Residual risk.** If enough jurisdictions' signers are
compelled simultaneously, compulsion can force transfers.
This is the deliberate "compliance with subpoenas" path;
no protocol avoids it.

## Not defended against

1. **Off-chain custody (cash, paper certificates).** Quidnug
   is for cryptographic assets with signed transactions.
2. **User-level social engineering.** If an attacker convinces
   a signer to actually sign a malicious transfer, the system
   works as designed.
3. **Multi-subsidiary collusion.** EU + US executives both
   corrupt = bigger than protocol-level concern.
4. **Privacy of holdings.** On-chain balances are visible;
   this is a blockchain property, not something Quidnug
   changes.

## Monitoring

Critical Prometheus metrics + alerts:

| Metric                                                  | Threshold          | Action                  |
|---------------------------------------------------------|--------------------|-------------------------|
| `quidnug_guardian_resignations_rejected_total`          | > 0 in 24h         | Investigate             |
| `quidnug_guardian_set_weakened_total{subject=wallet}`   | any                | Pager                   |
| Rotation age > policy (custom)                          | > 100 days         | Alert signer            |
| Un-rotated epoch age (custom)                           | > 365 days         | Pager + freeze          |
| `quidnug_probe_failure_total{reason="all_failed"}`      | > 5/min            | Check subsidiary link   |
| `quidnug_nonce_replay_rejections_total{enforced=true}`  | any                | Pager (attack attempt) |

## Incident response playbook

1. **Signer HSM compromise suspected.**
   - Invalidate epoch immediately: `POST /api/anchors {kind:"invalidation"}`
   - Initiate guardian recovery for a replacement HSM.
   - Audit all transactions from that signer in the last 30 days.

2. **Root guardian concern (CEO phone stolen).**
   - Immediately invalidate CEO's epoch.
   - All other root guardians (CISO, COO, auditor) are alerted.
   - Pending root-guardian-set updates blocked; they require
     CEO's key, which is now frozen.
   - Rotate CEO's key through the root guardian quorum.

3. **Unexpected guardian-set update detected.**
   - Operator halts nodes.
   - Review the update's authorizing signatures.
   - If malicious: file emergency halt with consortium; no
     further transactions signed while investigation runs.

## References

- [QDP-0001 Nonce Ledger](../../docs/design/0001-global-nonce-ledger.md)
- [QDP-0002 Guardian Recovery §13 Threat Model](../../docs/design/0002-guardian-based-recovery.md)
- [QDP-0006 Guardian Resignation](../../docs/design/0006-guardian-resignation.md)
- [`../interbank-wire-authorization/threat-model.md`](../interbank-wire-authorization/threat-model.md) — complementary design
