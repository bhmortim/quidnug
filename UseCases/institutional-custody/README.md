# Institutional Crypto Custody

**FinTech · High-value · Full key lifecycle management**

## The problem

A custody firm holds $5B+ in crypto assets across dozens of chains
for pension funds, corporate treasuries, and ETF issuers. The key
management practice is:

- **Multi-sig wallets** with 3-of-5 or higher thresholds, co-
  signers spread across geographies and HSMs.
- **Quarterly key rotation** required by internal controls.
- **Emergency rotation** when a co-signer leaves, a device is
  compromised, or a jurisdiction-specific event requires it.
- **Post-incident forensics** is often "screenshot of the
  multi-sig dashboard at the time."

The pain points:

1. **Rotations are manual and fragile.** An errant key not
   rotated properly becomes a permanent liability — or a
   ticking time bomb when someone's HSM quietly fails.
2. **Recovery is an emergency.** Lost keys = lost funds. Wallet
   providers offer "social recovery" but with proprietary flows
   and questionable guarantees.
3. **Audit trails are shallow.** "This $100M transfer was signed
   by these three keys on this date" has to be reconstructed
   across multiple systems.
4. **Cross-jurisdiction coordination is hard.** EU and US
   subsidiaries each have their own HSMs and approval rules;
   moving funds between them requires reconciling two different
   workflows.

## Why Quidnug fits

Quidnug's guardian model (QDP-0002) and nonce ledger (QDP-0001)
are the exact primitives a custody shop needs, upgraded beyond
typical multi-sig:

| Problem                                     | Quidnug primitive                         |
|--------------------------------------------|--------------------------------------------|
| "Which keys can authorize a transfer?"     | GuardianSet for each wallet/account        |
| "Rotate a key every 90 days"                | AnchorRotation with MaxAcceptedOldNonce    |
| "Invalidate a compromised key immediately"  | AnchorInvalidation (freeze epoch)          |
| "Emergency recovery with oversight"         | GuardianRecovery with time-lock veto       |
| "Forensically audit last year's activity"   | Ledger snapshot + signed event stream      |
| "Coordinate policy across subsidiaries"     | Fork-block transaction at shared height    |
| "Detect stale-key signatures"               | Lazy epoch probe (QDP-0007)                |

## High-level architecture

```
                     Quidnug Custody Root
                  (company-level identity)
                             │
                             │ has guardians:
                             │ {CEO, CISO, COO, external-auditor}
                             │
                             ▼
              ┌──────────────────────────────┐
              │                              │
              ▼                              ▼
      ┌──────────────┐              ┌──────────────┐
      │  US Vault    │              │  EU Vault    │
      │ (subsidiary) │              │ (subsidiary) │
      └──────┬───────┘              └──────┬───────┘
             │                             │
             │ has approvers:              │
             │ {signer1, signer2,          │
             │  signer3, signer4,          │
             │  signer5}                   │
             ▼                             ▼
     ┌───────────────┐            ┌───────────────┐
     │ Wallet accounts│            │Wallet accounts│
     │ (one quid each)│            │(one quid each)│
     └───────────────┘            └───────────────┘
            │                             │
            ▼                             ▼
     On-chain (multi-sig        On-chain (multi-sig
      contract executes)         contract executes)
```

Each level has its own guardian set with distinct rules:
- **Root**: requires executive-level quorum for structural
  changes.
- **Subsidiary**: declares the pool of authorized approvers.
- **Individual wallet**: specifies the specific threshold for
  that wallet (e.g., cold-storage wallet might require 5-of-7
  vs. hot-wallet 2-of-3).

## Data model

### Wallet as a quid

Each custody wallet is a quid. Its guardian set is the set of
authorized signers. The wallet's on-chain (Ethereum/Bitcoin/
etc.) multi-sig contract is configured to accept signatures
that match the quorum declared on Quidnug.

```
wallet-cold-btc-001 (quid)
  GuardianSet:
    guardians: [signer1(w=1), signer2(w=1), signer3(w=1),
                signer4(w=1), signer5(w=1), signer6(w=1), signer7(w=1)]
    threshold: 5                       # 5-of-7
    recoveryDelay: 7 days              # long time-lock for cold storage
    requireGuardianRotation: true      # no primary-key fast path
```

### Approval as title + events

A transfer authorization is a title representing "the authorization
to transfer X from wallet Y". Approvers cosign via events
(same pattern as interbank wires).

```
title: "tx-auth-2026-04-18-abc"
  owner: wallet-cold-btc-001
  attributes:
    targetChain: "bitcoin"
    targetAddress: "bc1q..."
    amount: "10.5"
    currency: "BTC"
    proposedAt: <unix>

events on this title:
  - wire.cosign {signer: signer1, epoch: 0}
  - wire.cosign {signer: signer2, epoch: 0}
  - wire.cosign {signer: signer3, epoch: 0}
  - wire.cosign {signer: signer4, epoch: 0}
  - wire.cosign {signer: signer5, epoch: 0}  ← quorum met
  - wire.approved (system-emitted)
```

Once the `wire.approved` event fires, the subsidiary's bridge
system extracts each signer's signature and submits to the
on-chain multi-sig contract.

### Epoch-based audit

Every signer's signing key has an **epoch** — an integer that
monotonically advances on each rotation. A historical transfer
audit looks like:

```
"Transfer X was approved at block 123456:
 - signer1 signed at epoch=2 (rotated 2026-02-15)
 - signer2 signed at epoch=3 (rotated 2026-03-20)
 - signer3 signed at epoch=1 (rotated 2025-11-05)
 - signer4 signed at epoch=0 (never rotated — still original onboarding key!)
 - signer5 signed at epoch=2
 Quorum weight: 5 of 5. Threshold: 5. APPROVED."
```

An auditor immediately sees that signer4 is overdue for a
rotation. Actionable.

## Quarterly rotation workflow

Every 90 days, each signer rotates their key:

```
1. Generate new keypair in HSM (signer's local ceremony).
2. Submit AnchorRotation:
   - fromEpoch: current
   - toEpoch: current + 1
   - newPublicKey: <hex>
   - minNextNonce: 1
   - maxAcceptedOldNonce: 100  (grace for in-flight approvals)
3. Nodes across subsidiaries automatically learn of the rotation
   via push gossip (QDP-0005). In-flight transfer titles
   that the signer had started approving still complete
   under their old-epoch key up to the cap.
4. New transfers are signed with the new-epoch key.
```

If the rotation doesn't happen within the policy window, a
monitoring rule triggers an alert and (optionally) an
auto-invalidation of the stale epoch — locking the signer
out until they complete the rotation.

## Emergency key loss

Scenario: signer3's HSM catastrophically fails at 2am Sunday.
Several transfers are in flight. Recovery:

```
1. Ops confirms HSM is bricked (standard diagnostic).
2. signer3's personal guardians initiate recovery:
   - signer3's manager
   - signer3's backup HSM (stored in separate geo)
   - the subsidiary's compliance officer
   (3-of-5 recovery guardians; see signer3's own GuardianSet)

3. Recovery anchor submitted. Time-lock window begins
   (7 days for cold-wallet signer, 1h for hot).

4. During delay: each in-flight transfer can still complete
   with signer3's original signatures.

5. If signer3 is actually fine (just on vacation) and this is
   a social-engineering attempt, signer3 can veto from a
   different trusted device.

6. At delay expiration: commit fires. signer3's epoch advances.
   Any future signatures require the new HSM.
```

No emergency vendor calls. No protocol downtime. Just a
cryptographically-auditable flow.

## Subsidiary isolation

EU and US subsidiaries are separate quids with their own
approver sets. Moving funds between them is an explicit
transfer-out-of-one, transfer-in-to-other flow that requires
BOTH subsidiaries' approval quorums.

Compliance value: US compliance officer can't unilaterally
move funds sitting in the EU subsidiary. EU data-privacy rules
are enforceable because EU subsidiary's approvals don't
implicitly grant US access.

## Key Quidnug features

- **Guardian sets with `requireGuardianRotation`** — blocks
  primary-key-only rotations on sensitive quids. Every rotation
  goes through the time-locked guardian path.
- **AnchorRotation with `MaxAcceptedOldNonce`** — bounded grace
  window for rotation.
- **AnchorInvalidation** — immediate kill-switch for compromised
  keys.
- **GuardianResignation (QDP-0006)** — signer departs, formally
  resigns.
- **Push gossip (QDP-0005)** — rotations propagate across EU/US
  nodes within seconds.
- **Lazy epoch probe (QDP-0007)** — the US subsidiary's node
  detecting "hasn't seen signer-EU-3 sign in 35 days" probes
  the EU home domain before accepting a signature.
- **K-of-K bootstrap (QDP-0008)** — opening a new subsidiary in
  APAC bootstraps from US + EU + two other peers.
- **Fork-block trigger (QDP-0009)** — when the firm raises the
  minimum threshold to 6-of-9 globally, fork-block coordinates
  all subsidiaries.

## Value delivered

| Dimension                          | Before                                  | With Quidnug                                      |
|------------------------------------|-----------------------------------------|---------------------------------------------------|
| Quarterly key rotation             | Manual, ceremony + ticket               | Signer-initiated anchor; auditable chain          |
| Key-loss recovery                  | Emergency vendor call                   | Guardian recovery with time-lock                   |
| Audit "who signed what and when"   | Cross-system reconciliation             | Single chain query                                 |
| Cross-subsidiary transfer          | Separate approval chains + email        | Cross-subsidiary quorum natively supported         |
| Insider-removal                    | Revoke HSM cert + retrain multi-sig     | Guardian-set update (on-chain, verifiable)         |
| Compliance reporting               | Quarterly manual extract                | Query the blockchain                               |
| Forensic reconstruction of incident | Hope you have screenshots              | Replay the chain; every approval has signer+epoch  |

## What's in this folder

- [`README.md`](README.md) — this document
- [`implementation.md`](implementation.md) — Quidnug API calls
- [`threat-model.md`](threat-model.md) — attackers & mitigations

## Related

- [`../interbank-wire-authorization/`](../interbank-wire-authorization/) — similar M-of-N model
- [`../developer-artifact-signing/`](../developer-artifact-signing/) — guardian recovery pattern
- [QDP-0002 Guardian Recovery](../../docs/design/0002-guardian-based-recovery.md)
