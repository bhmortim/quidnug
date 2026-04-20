# Interbank wire authorization launch checklist

> Sequential T-180 through T+30 onboarding steps for bringing a
> bank onto Quidnug-based wire authorization. Work top to
> bottom; each item gates the next.
>
> Written for a **national bank joining its first correspondent
> consortium** (the most common starting profile). Small
> community banks can collapse steps 3-4 (fewer governors,
> single region); global banks should multiply the regional
> stand-up steps across jurisdictions.
>
> Parallels [`UseCases/elections/launch-checklist.md`](../elections/launch-checklist.md)
> but adapted for banking deployment profile. Read
> [`README.md`](README.md), [`integration.md`](integration.md),
> and [`operations.md`](operations.md) first.

---

## T-180 to T-120 days: Legal + regulatory foundation

Wire authorization is one of the most heavily regulated
activities a bank does. Nothing happens before legal sign-off.

### Regulatory strategy

- [ ] General counsel + compliance chief review + approve the
      overall architecture. Specifically: does running wire
      authorization on Quidnug satisfy:
      - Home-jurisdiction central-bank rules (Fed for US,
        BaFin for DE, BoE for UK, etc.)
      - FFIEC IT Examination Handbook (US federal banking)
      - OCC Heightened Standards (for national banks)
      - FATF Recommendation 16 (Wire Transfers)
      - Basel Committee Principles for Sound Management of
        Operational Risk
- [ ] File advance notice with primary regulator (US: OCC +
      Federal Reserve; EU: ECB + national regulator). Some
      jurisdictions require 30-90 day advance notice for
      operational-risk framework changes.
- [ ] Engage independent banking-software auditor for
      pre-certification. Must have prior experience with
      SWIFT/Fedwire/SEPA gateway integrations.
- [ ] Legal opinion letter on whether on-chain signatures
      satisfy the "two officer" requirement under applicable
      rules (varies; many jurisdictions treat cryptographic
      signatures as equivalent to written ones if audit
      trail is preserved).
- [ ] Privacy review: any PII in wire metadata that leaves the
      bank's controlled environment needs GDPR / CCPA / LGPD
      assessment. Plan: encrypt wire payloads, store only
      hashes on-chain for cross-bank audit.
- [ ] Cyber insurance review: policy coverage for
      cryptographic-key custody, HSM failure, blockchain
      infrastructure.

### Governance foundation

- [ ] Internal governance committee formed + briefed on the
      protocol architecture. Typical composition:
      - CEO or designated COO
      - Chief Compliance Officer
      - Chief Risk Officer
      - Head of Wire Operations
      - Head of IT / CISO
      - External regulator liaison (observer, non-voting)
- [ ] Governance structure defined + documented (see
      [`integration.md`](integration.md) §2):
      - Governor quorum (who, weights, threshold)
      - Consortium membership (which of the bank's own nodes
        produce blocks)
      - Peer-bank trust edges (counterparty banks)
      - Emergency-notice clause (normal 24h / emergency 1h)
      - Signatory roster + per-wire-class thresholds
      - Guardian quorums per governor (QDP-0002)
- [ ] Signatory policy published internally:
      - Wire-class thresholds ($1M, $10M, $100M, $1B)
      - Required signer count per class
      - Maker/checker separation rules
      - Dual-control for policy changes
- [ ] Board of directors briefed + resolution passed
      authorizing the new infrastructure.

---

## T-120 to T-90 days: Hardware + infrastructure procurement

### HSM procurement

Wire authorization is HSM-dependent. Every signing key lives
in hardware; never plaintext in RAM.

- [ ] HSM vendor selected. Acceptable options:
      - Thales Luna Network HSM (bank-grade)
      - Entrust nShield Connect
      - AWS CloudHSM (cloud-native equivalents)
      - Azure Dedicated HSM
      - Google Cloud HSM
- [ ] HSMs procured per region:
      - Primary data center: 2 HSMs (active + standby)
      - DR data center: 2 HSMs
      - Each additional region: 2 HSMs
- [ ] HSM firmware version locked + documented.
- [ ] Air-gapped key-ceremony room provisioned (offline laptop
      + paper backup printer + Faraday cage or
      shielded room).

### Node hardware

- [ ] Validator hardware procured per [`operations.md`](operations.md) §3:
      - 8 cores, 32 GB RAM, 2 TB NVMe (IOPS 50k+), 10 Gbps
      - Per-region: 2-3 validators (redundancy)
      - Total depending on bank scale (see operations.md §2)
- [ ] Cache tier hardware procured (1 per 10k wires/day of
      throughput, per region).
- [ ] Archive node hardware procured (20 TB disk, 2+ per
      bank minimum, geographically separated).
- [ ] Observer node hardware for regulators (lower-spec;
      regulators may supply their own).

### Network + connectivity

- [ ] Private cross-region connectivity (AWS Direct Connect,
      Azure ExpressRoute, MPLS) for validator gossip.
- [ ] Cloudflare (or equivalent) enterprise account for
      public-facing cache + well-known endpoints.
- [ ] Dedicated circuits for SWIFT (existing), Fedwire
      (existing), SEPA (existing). Quidnug doesn't replace
      these; it augments them.
- [ ] VPN + zero-trust network access for corporate treasury
      clients.

### Software + keys

- [ ] Quidnug node binary built from a specific tagged
      release (e.g., `v2.4.0`). SHA-256 hashes published.
      Reproducible build verified.
- [ ] All governor keys generated via air-gapped key
      ceremony. Paper backups printed + distributed to
      separate safe-deposit boxes (dual control).
- [ ] Guardian quorums installed for each governor key
      (following QDP-0002). Typical: 3-of-5 guardians per
      governor, drawn from executive + board.
- [ ] Validator node keys generated directly inside HSMs;
      never exportable.
- [ ] Signatory keys (wire officers) generated on YubiKey
      or equivalent, one per officer. Backup guardian
      quorum per signatory (typically 2-of-3 coworkers).
- [ ] CLI tooling (`quidnug-cli wires`) tested in staging.

---

## T-90 days: Domain + consortium setup

### Publish the network descriptor

- [ ] Publish `https://bank.<id>.com/.well-known/quidnug-network.json`
      per QDP-0014 §7. Signed by the bank's operator key.
      Includes:
      - Operator quid + pubkey (the bank's root identity)
      - API gateway URLs
      - Seed nodes + capabilities
      - Governors + quorum
      - Domain tree
      - Peer banks + federation links (QDP-0013)
- [ ] Publish backup mirror of the well-known file at a
      federated location (e.g., `https://quidnug.com/.well-known/banks/<id>.json`)
      for resilience if the bank's public site is down.

### Register the bank's domain tree

- [ ] Register the bank's root domain:
      ```bash
      quidnug-cli domain register \
          --name "bank.<id>" \
          --validators "<consortium-quids>" \
          --governors "<governor-quids>" \
          --governance-quorum 0.7 \
          --threshold 0.67 \
          --notice-period 24h \
          --emergency-notice 1h \
          --key <operator-key>
      ```
- [ ] Register sub-domains:
      ```bash
      for child in wires.outbound wires.inbound \
                   signatories audit \
                   peering.counterparty-banks; do
          quidnug-cli domain register \
              --name "bank.<id>.$child" \
              --parent-delegation-mode inherit \
              --key <operator-key>
      done
      ```
- [ ] Register wire-class policy domains (one per threshold
      tier):
      ```bash
      for tier in standard-under-1m high-value-1m-10m \
                  executive-10m-100m board-over-100m; do
          quidnug-cli domain register \
              --name "bank.<id>.wires.policy.$tier" \
              --parent-delegation-mode inherit \
              --signatory-threshold <N> \
              --key <operator-key>
      done
      ```
- [ ] Verify all domains visible in the bank's API at
      `api.bank.<id>.com/api/domains`.

### Stand up the consortium

- [ ] All consortium members (the bank's own validators)
      deploy Quidnug node binary from the audited release.
- [ ] Each publishes its own `NODE_ADVERTISEMENT` per
      QDP-0014:
      ```bash
      quidnug-cli node advertise \
          --operator-quid <bank-operator-quid> \
          --endpoints "https://node<i>.bank.<id>.com:443,http/2,<region>,1,100" \
          --capabilities "validator,archive" \
          --supported-domains "bank.<id>.*" \
          --expires-in "7d" \
          --sign-with <node-i-key>
      ```
- [ ] Bank operator publishes TRUST attestation edges to
      each validator:
      ```bash
      quidnug-cli trust grant \
          --truster <bank-operator-quid> \
          --trustee <node-i-quid> \
          --domain "operators.bank.<id>" \
          --level 1.0 \
          --sign-with <operator-key>
      ```
- [ ] Pairwise peering: all validators trust each other at
      0.95 in the `peering.*` domain.
- [ ] Block production verified: all validators see blocks
      from all others at `Trusted` tier.

### Register signatories

- [ ] Each wire officer registers an IDENTITY transaction
      under `bank.<id>.signatories`:
      ```bash
      quidnug-cli identity register \
          --domain "bank.<id>.signatories" \
          --name "alice.wireop" \
          --pubkey <alice-yubikey-pubkey> \
          --creator <hr-manager-quid> \
          --attributes '{"role":"wire-officer","region":"NY"}' \
          --sign-with <hr-manager-key>
      ```
- [ ] Governance quorum signs `ADD_SIGNATORY` event per
      officer, granting authority in specific policy
      domains:
      ```bash
      quidnug-cli wires add-signatory \
          --signatory-quid <alice-quid> \
          --policy-domain "bank.<id>.wires.policy.high-value-1m-10m" \
          --weight 1 \
          --governor-signatures "<sigs>" \
          --sign-with <governor-primary-key>
      ```
- [ ] Each signatory's guardian quorum published (for key
      recovery if YubiKey lost).

---

## T-60 days: Federation setup + peer onboarding

### Peer bank TRUST edges

Interbank wires cross networks (per QDP-0013). Before you
can send a wire to another bank, you need a federation
relationship.

- [ ] Bilateral federation agreement signed with each peer
      bank (legal document, not cryptographic). Covers:
      - Liability boundaries
      - Regulator observability terms
      - Emergency-response communication channels
      - Dispute-resolution procedure
- [ ] TRUST_IMPORT transactions configured per peer bank:
      ```bash
      quidnug-cli federation import \
          --source-network "bank.<peer-id>" \
          --source-root-quid <peer-operator-quid> \
          --trust-level 0.9 \
          --domain "peering.counterparty-banks" \
          --external-trust-source "https://bank.<peer-id>.com/.well-known/quidnug-network.json" \
          --sign-with <operator-key>
      ```
- [ ] Peer bank verifies the import from their side; signs
      reciprocal TRUST_IMPORT. Both networks now recognize
      each other's signatures.

### SWIFT/Fedwire/SEPA bridge configuration

- [ ] Quidnug-to-SWIFT bridge service deployed (see
      `integrations/iso20022/README.md`).
- [ ] Bridge signs events when sending/receiving wires:
      - `WIRE_AUTHORIZED` → SWIFT MT103 outbound
      - SWIFT MT103 inbound → `WIRE_RECEIVED` event
- [ ] Bridge service runs with its own dedicated quid
      (separate from signatory quids; separation-of-duty).
- [ ] Fedwire bridge (for US-domestic) + SEPA bridge (for
      EU) configured analogously.

### Regulator node

- [ ] Regulator observer node provisioned (either by the
      bank in the regulator's data center, or by the
      regulator themselves connecting to the bank's
      network).
- [ ] Observer node registered as capability=observer only
      (no validator, no archive — purely read).
- [ ] Regulator credentials issued for scoped-access
      queries (domain-level visibility per their mandate).
- [ ] Regulator training completed: how to run recounts,
      how to subpoena data, how to file an audit query.

---

## T-45 days: Audit + software review

### External audit

- [ ] External security-audit firm completes review of:
      - Quidnug node binary (reviewed at the QDP level)
      - Bank-specific integration code (bridges, policy
        enforcement, monitoring)
      - HSM integration correctness
      - Key-ceremony procedures
- [ ] Penetration-testing engagement on staging environment.
- [ ] Audit report published to regulators + governance
      committee.
- [ ] All high-severity findings addressed; medium-severity
      findings scheduled for post-launch; low-severity
      tracked.

### Internal code audit

- [ ] Wire-policy-enforcement code (what triggers additional
      signers for high-value wires) reviewed by compliance
      + risk.
- [ ] AML engine integration verified: every wire above
      threshold gets automated AML screen before
      authorization.
- [ ] Signatory-revocation flow tested: when Alice leaves
      the bank, her quid is removed from the signatory
      roster within 1 hour.

### Dry run #1 (1,000 fake wires)

- [ ] Staging environment reset to clean state.
- [ ] 1,000 fake wires generated across all policy tiers.
- [ ] Internal signatories authorize per policy rules.
- [ ] SWIFT bridge delivers simulated MT103 messages (to a
      test counterparty network).
- [ ] Reconciliation job runs; 1,000/1,000 match expected.
- [ ] "Suspected-fraud" scenario: 5 deliberately-malformed
      wires; AML engine flags them; compliance officer
      vetoes via governance quorum.
- [ ] "Validator-failure" scenario: kill 1 of 5 validators
      mid-batch; 4-of-5 continues.
- [ ] Document all issues in `dry-run-1-postmortem.md`.

---

## T-30 days: Final polish + training

### Dry run #2 (10,000 fake wires, realistic mix)

- [ ] Larger-scale test with 10,000 fake wires across all
      policy tiers + all regions.
- [ ] Peer bank (also in staging) runs receiving-side flow;
      cross-bank handoff latency measured.
- [ ] Regulator observer runs live-audit queries during the
      run.
- [ ] Performance: all 10,000 wires authorize within the 3s
      SLA; cross-bank handoff within the 30s SLA.
- [ ] Issues found + fixed.

### Staff training

- [ ] Wire officers complete certified training on:
      - YubiKey operation + daily key management
      - Wire-authorization workflow
      - Suspected-fraud escalation
      - Guardian-recovery procedure (if YubiKey lost)
- [ ] Operations team trained on:
      - Monitoring dashboards
      - Incident-response playbook (ops.md §7)
      - Validator + consortium health
      - Federation-partition response
- [ ] Compliance team trained on:
      - Live query tools
      - Governance-change workflow (REMOVE_SIGNATORY,
        emergency notice)
      - Regulator subpoena response
- [ ] Corporate treasury clients briefed:
      - New client API (for corporates wanting direct
        integration)
      - Status-page expectations

### Corporate client onboarding

- [ ] Top 10 corporate clients (by wire volume) invited to
      participate in closed beta.
- [ ] Each gets a client-side Quidnug quid (their
      treasurer's key) + guardian-recovery-as-a-service
      offering if desired.
- [ ] Test wires sent from beta clients in staging.
- [ ] Feedback incorporated.

### Infrastructure freeze

- [ ] Code freeze: no production deploys after T-14 except
      for critical security fixes.
- [ ] Final HSM firmware + node-binary versions locked.
- [ ] All consortium members confirm on-call rotation for
      go-live.

---

## T-14 days: Staged rollout begins

Wire authorization doesn't have an "election day" — it's
continuous. Ramp up gradually.

### Tier-1 rollout: internal test wires only

- [ ] Production network live but accepting only internal
      test wires (wires between the bank's own accounts).
- [ ] All internal reconciliation match.
- [ ] Compliance team monitors: no false positives, no
      missed authorizations.

### Daily checks during ramp-up

Each day from T-14 to T-1:

- [ ] Morning: all consortium nodes healthy; all
      HSMs responsive.
- [ ] Peak-hour traffic simulation: 10x expected production
      load injected to verify capacity.
- [ ] End-of-day reconciliation: 100% match.
- [ ] Any incidents: `INCIDENT_<type>` event published +
      fix.

---

## T-7 days: Beta corporate wires

### Tier-2 rollout: beta corporates

- [ ] 10 beta corporate clients begin sending real
      (low-value) wires via the new infrastructure.
- [ ] Parallel path: same wires also flow via legacy
      infrastructure as backup; reconciliation nightly to
      confirm both produce same result.
- [ ] Increase to 50 beta corporates over the week.

### Status page + monitoring

- [ ] Public status page live at `status.bank.<id>.com`.
- [ ] Regulator has live dashboard access.
- [ ] Internal SRE team 24/7 on-call.

---

## T-1 day: Pre-flight

- [ ] All consortium members verify blockchain head matches.
- [ ] Final smoke test: 100 synthetic wires across all
      tiers; all authorize within SLA.
- [ ] Publish `GO_LIVE_READY` event to audit domain.
- [ ] On-call roster confirmed; key personnel reachable.
- [ ] Communications lead briefed on launch-day message.

---

## T-0: Go-live day

### Market open

- [ ] First real production wire authorized via Quidnug.
      (Low-value, well-known corporate counterparty; CEO +
      CISO both watching in real time.)
- [ ] `GO_LIVE` event published to audit domain.
- [ ] Press release (if applicable) coordinated with
      regulator.

### Throughout day 1

- [ ] Wire volume ramps throughout the day as more
      corporates + counterparties transition.
- [ ] Every wire monitored manually for the first hour; then
      spot-checks throughout the day.
- [ ] Incident-response team on-site, not just on-call.

### End of day 1

- [ ] Day-1 wire count vs expected: report to governance
      committee.
- [ ] Reconciliation run against all counterparties.
- [ ] Incidents: none expected, but any found get
      postmortem within 24h.

---

## T+7 days: Week-1 review

- [ ] Operational postmortem:
      - Total wires authorized
      - Total volume ($)
      - Average authorization latency
      - Any incidents + resolutions
      - SLA attainment (target: 99.9% within 3s)
- [ ] Expand rollout to remaining corporates per schedule.
- [ ] Regulator week-1 report submitted per requirements.

---

## T+30 days: Month-1 review + full migration

- [ ] All corporate clients migrated to new infrastructure.
- [ ] Legacy wire-authorization system put in "receive
      only" mode (can receive inbound from correspondents,
      but no new outbound; prepares for decommission).
- [ ] Month-1 operational metrics report:
      - Uptime (target: 99.95%)
      - Wire volume processed
      - Cost savings vs legacy
      - Audit query response time (target: <5s live, <30s
        archive)
- [ ] Regulator month-1 briefing + sign-off for full
      production.
- [ ] Board of directors update.

---

## T+90 days: Legacy decommission

- [ ] Legacy wire-authorization system decommissioned.
- [ ] All paper-based cosigning workflows retired.
- [ ] Annual-audit cycle transitions from old system to
      Quidnug chain queries.
- [ ] First full quarter of operation closed with clean
      regulatory review.

---

## Metrics to track

For every operational day, track these against targets from
[`operations.md`](operations.md) §4:

| Metric | Target | How to measure |
|---|---|---|
| Wires authorized | (per bank baseline) | Count of `WIRE_AUTHORIZED` events |
| Authorization latency p99 | < 3s | Event timestamp delta |
| Cross-bank handoff p99 | < 30s | WIRE_AUTHORIZED → WIRE_RECEIVED delta |
| End-to-end settlement p99 | < 2 min | WIRE_AUTHORIZED → SETTLED delta |
| Consortium uptime | > 99.95% | Block-production rate + validator health |
| Cross-bank federation uptime | > 99.9% | Federation latency monitor |
| Reconciliation match rate | 100% | Daily cross-check with core banking |
| Failed-authorization rate | < 0.1% | Rejected events / total attempts |
| Regulator query response p99 | < 5s | Query logs |
| HSM operation error rate | < 0.001% | HSM logs |
| Audit events vs wires ratio | 1:1 | Every wire produces 5-9 audit events |

---

## What to do if...

### ...a signatory's YubiKey is reported lost

1. Signatory's guardian quorum (typically 2-of-3 coworkers)
   initiates `QuidRecoveryInit` per QDP-0002.
2. 1-hour time-lock window starts. Signatory cannot veto
   since the YubiKey is lost; if it turns out to be
   misplaced and they find it, they can reclaim within
   the window.
3. After time-lock, new YubiKey provisioned; signatory
   resumes wire-authorization authority.
4. Audit event published: `SIGNATORY_KEY_RECOVERED`.
5. All wires signed within the preceding 72 hours by the
   old key flagged for review (potential compromise
   window).

### ...a validator node fails during market hours

1. Confirm it's a real failure (not transient network).
2. Consortium continues with remaining validators
   (quorum preserved per [`operations.md`](operations.md) §7.1).
3. Spare validator brought online from DR tier; new node
   publishes `NODE_ADVERTISEMENT`; consortium re-balances.
4. Post-incident: root-cause analysis + postmortem within
   48h.

### ...cross-bank federation is partitioned

1. Wires to/from the partitioned counterparty queue.
2. Fallback to SWIFT / Fedwire / SEPA for urgent wires.
3. Monitor both networks' status pages.
4. Once partition heals, federation catches up
   automatically via gossip; reconciliation confirms no
   wires lost or duplicated.
5. If partition exceeds 1 hour during market hours:
   escalate to senior management + regulator notice.

### ...a wire is contested by a corporate client

1. Corporate submits `WIRE_DISPUTE` event via their client.
2. Operations team pulls full signature chain from chain
   queries: who signed, when, under which policy.
3. If dispute is clerical (wrong account, wrong amount
   but authorized): standard wire-recall procedure.
4. If dispute alleges unauthorized signing: security team
   investigates + potential signatory compromise handling.
5. Chain queries are cryptographically verifiable so
   dispute resolution is fast + unambiguous compared to
   legacy "search 4 systems" approach.

### ...a regulator subpoenas wire data

1. Legal team reviews scope.
2. Operations provisions read-only scoped access for the
   regulator's investigator (federated read-only node +
   scoped domain access).
3. All queries the regulator runs are logged on-chain
   (regulator's node publishes queries as events on an
   audit domain; subpoena scope is provable).
4. If decrypted payload data required (wire memo, KYC
   docs, beneficiary details): legal team releases
   decryption keys per subpoena.
5. Response time: hours to days, vs months under legacy.

### ...a peer bank exits the federation (e.g., acquired or failed)

1. Governance quorum signs `TRUST_IMPORT_REVOCATION` for
   the exiting peer.
2. In-flight wires to/from the peer are drained via SWIFT
   fallback.
3. Historical wires remain on-chain for audit; new wires
   to that peer are blocked.
4. If the peer is being acquired, reconfiguration of the
   federation to the acquirer's network takes 30-90 days
   (repeat the peer-onboarding flow from T-60).

### ...the home regulator revokes permission

1. Immediately halt all new wire authorizations (governor
   quorum emergency `FREEZE_NEW_WIRES` action).
2. In-flight wires complete via legacy path.
3. Legal team + senior management engage regulator.
4. If permanent revocation: decommission the network.
   Archive all data for retention period per
   jurisdiction (typically 5-7 years for banking).

---

## References

- [`README.md`](README.md) — wire-authorization semantics.
- [`architecture.md`](architecture.md) — data model + flows.
- [`integration.md`](integration.md) — architectural-pillar
  integration (QDP-0012/0013/0014).
- [`operations.md`](operations.md) — deployment topology,
  daily operations, incident response.
- [`threat-model.md`](threat-model.md) — attack analysis.
- [`implementation.md`](implementation.md) — concrete Quidnug
  API calls.
- [`UseCases/elections/launch-checklist.md`](../elections/launch-checklist.md)
  — companion launch checklist for the other coordination-
  archetype use case.
- [`deploy/public-network/reviews-launch-checklist.md`](../../deploy/public-network/reviews-launch-checklist.md)
  — public-network reviews launch template this doc cribs
  from.
