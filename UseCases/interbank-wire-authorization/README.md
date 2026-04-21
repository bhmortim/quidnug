# Interbank Wire Authorization

**FinTech · High-value · M-of-N signing · Guardian recovery**

## The problem

A mid-sized bank settles ~$2B/day across Fedwire, CHIPS, SWIFT, and a
regional faster-payments rail. For wires above a policy threshold
(e.g., $1M), internal controls require **two separately-credentialed
officers** to cosign, with a compliance officer review for anything
above $10M.

Current production reality at most banks:

- Cosigning is a **spreadsheet workflow** wrapped around a hardware
  security module (HSM). The "second signature" is often a scanned
  approval PDF attached in a ticket.
- When an HSM dies (firmware bug, cert expiry, a staff member
  departs), remediation is an emergency vendor ticket. Wires pile up.
- Replay protection is **transaction-ID uniqueness in the core banking
  system**, which is a single point of contention. If the core is
  unreachable (e.g., DR failover), two branches can process the same
  wire twice.
- There's no cryptographic audit trail of "which officer's key
  approved this wire" — only a database row with a user ID.

When an auditor or regulator asks "prove this $50M wire was authorized
by two distinct, active officers on that date," the answer is a
screenshot of an SSO login event plus an emailed PDF. Nobody is happy
about that.

## Why Quidnug fits

The capabilities Quidnug offers line up directly with the primitives a
multi-party wire-approval system needs:

| Problem                                           | Quidnug primitive                               |
|--------------------------------------------------|-------------------------------------------------|
| "Which officers can approve this class of wire?" | `GuardianSetUpdate` — on-chain M-of-N quorum   |
| "Is this approval replay-safe?"                  | Per-signer monotonic anchor nonces              |
| "Officer's HSM died — now what?"                 | `GuardianRecoveryInit` + time-locked rotation   |
| "How do we audit this?"                          | Signed anchor history in the blockchain         |
| "How do we coordinate policy changes?"           | `ForkBlockTransaction` at scheduled height      |
| "Compliance needs to veto in-flight wires"       | Guardian recovery veto (within delay window)    |

A wire approval becomes: **a `TITLE` transaction over the wire
instruction, authorized by the bank's M-of-N guardian set, with each
cosigner's signature at their own monotonically-advancing nonce.**

## High-level architecture

```
           ┌─────────────────────────────────────────────┐
           │  Bank Core Banking System (existing)        │
           │  Generates wire instructions                 │
           └──────────────────┬──────────────────────────┘
                              │
                              │  "Submit wire for signing"
                              ▼
    ┌──────────────────────────────────────────────────────┐
    │  Quidnug Bank Node (consortium node #1)               │
    │  ├─ Bank root quid: "bank-us-wire"                    │
    │  ├─ Officer quids: alice-op, bob-op, carol-compliance │
    │  ├─ Guardian set for "bank-us-wire":                  │
    │  │   threshold=2,                                     │
    │  │   members={alice-op(w=1), bob-op(w=1),             │
    │  │            carol-compliance(w=2)}                  │
    │  └─ Domain: "wires.federal.us"                        │
    └──────────────────────────────────────────────────────┘
              │         │         │            │
              │         │         │            │
              ▼         ▼         ▼            ▼
         ┌────────┐ ┌──────┐ ┌──────────┐ ┌─────────┐
         │Alice's │ │Bob's │ │Carol's   │ │ Fed/    │
         │HSM     │ │HSM   │ │HSM       │ │ CHIPS   │
         │(EAST)  │ │(WEST)│ │(Compl.)  │ │ Gateway │
         └────────┘ └──────┘ └──────────┘ └─────────┘
```

Each officer holds their own HSM-backed signing key. The bank's node
tracks the on-chain guardian set; wire instructions route to officers
for signing; signed titles are published to a **consortium** of peer
banks (clearing counterparties, correspondent banks, regulators) that
all run Quidnug nodes.

## Key Quidnug features used

- **`GuardianSetUpdate` (QDP-0002)** — declares which officers have
  approval authority, with weighted thresholds so the compliance
  officer (weight=2) counts as two regular officers. Every officer
  in the set must have on-chain consented.
- **Per-signer anchor nonces (QDP-0001)** — each wire approval
  includes an officer's anchor-nonce that must strictly advance. A
  replayed approval from a captured session is rejected at the
  ledger.
- **Guardian-based recovery (QDP-0002 §6.4.4)** — when Alice's HSM
  fails, her coworkers (her personal guardian set, separate from the
  bank's approval set) initiate a rotation to a new HSM. The default
  1-hour time-lock window is plenty for Alice to veto if she's at a
  coffee shop and not actually dead.
- **`GuardianResignation` (QDP-0006)** — when Alice leaves the bank,
  she resigns from the approval set. Doesn't invalidate wires
  already in flight.
- **Lazy epoch probe (QDP-0007)** — correspondent banks that haven't
  seen Alice sign a wire in 30 days probe her home domain before
  accepting a new approval. Catches the "Alice's old key was
  compromised after she rotated but before the counterparty learned
  of the rotation" attack.
- **Fork-block trigger (QDP-0009)** — when the consortium decides
  to raise the high-value threshold from $10M to $25M, a fork-block
  transaction signed by a quorum of member banks activates the new
  rule at a future height across all participants simultaneously.

## Value delivered

| Dimension                              | Before                                     | With Quidnug                                                  |
|----------------------------------------|--------------------------------------------|--------------------------------------------------------------|
| Multi-sig latency                      | Hours (PDF workflow)                       | Seconds (HSM → node → gossip)                                |
| Key recovery time (HSM failure)        | Hours to days (vendor ticket)              | 1h time-lock window + new-HSM provisioning                    |
| Audit trail for a specific wire        | 4-system join (SSO, ticket, core, email)   | Single blockchain query                                       |
| Replay risk under DR failover          | Possible (cross-branch race)               | None — per-signer nonces are monotonic                        |
| Adding a new officer                   | Ticket to IAM + compliance paperwork       | On-chain guardian update, consent signatures                  |
| Regulatory reporting cost              | Manual reconciliation                      | Deterministic chain replay                                    |
| Cross-bank counterparty validation     | Call their ops desk                        | Verify guardian set signature cryptographically               |

## Runnable POC

Full end-to-end demo at
[`examples/interbank-wire-authorization/`](../../examples/interbank-wire-authorization/):

- `wire_approval.py` — pure decision logic: tier selection,
  weighted signers, role gating, per-signer NonceLedger,
  counterparty verification.
- `wire_approval_test.py` — 14 pytest cases across the full
  decision surface.
- `demo.py` — seven-step end-to-end flow: install tiered
  policy, run three wires across the tier boundaries,
  receiver re-verification, replay-defense demonstration.

```bash
cd examples/interbank-wire-authorization
python demo.py
```

## What's in this folder

- [`README.md`](README.md) — this document (high-level)
- [`architecture.md`](architecture.md) — detailed data model,
  sequence diagrams, per-component breakdown
- [`integration.md`](integration.md) — how the use case composes
  on top of the three architectural pillars (QDP-0012 governance,
  QDP-0013 federation, QDP-0014 discovery + sharding); why each
  bank is its own network
- [`operations.md`](operations.md) — deployment topology by
  bank scale (community / regional / national / global),
  capacity planning, daily ops playbook, incident response,
  SWIFT/Fedwire/SEPA bridge patterns
- [`launch-checklist.md`](launch-checklist.md) — T-180 through
  T+30 bank-onboarding sequence: legal + regulatory foundation,
  HSM procurement, governance setup, peer federation,
  staged rollout to production
- [`implementation.md`](implementation.md) — concrete Quidnug API
  calls with Go and shell examples
- [`threat-model.md`](threat-model.md) — attacker profiles, what the
  design defends against, explicit limits

## Related

- [QDP-0001: Nonce Ledger](../../docs/design/0001-global-nonce-ledger.md)
- [QDP-0002: Guardian Recovery](../../docs/design/0002-guardian-based-recovery.md)
- [QDP-0007: Lazy Epoch Propagation](../../docs/design/0007-lazy-epoch-propagation.md)
- [QDP-0009: Fork-Block Trigger](../../docs/design/0009-fork-block-trigger.md)
- [QDP-0012: Domain Governance](../../docs/design/0012-domain-governance.md)
  — governor / consortium / cache separation; the mechanism
  behind per-bank governance quorums
- [QDP-0013: Network Federation](../../docs/design/0013-network-federation.md)
  — one-protocol-many-networks; every bank is its own network
  linked to peers via `TRUST_IMPORT`
- [QDP-0014: Node Discovery + Sharding](../../docs/design/0014-node-discovery-and-sharding.md)
  — `NODE_ADVERTISEMENT` + `.well-known/quidnug-network.json` for
  cross-bank counterparty discovery
- [`UseCases/elections/`](../elections/) — companion coordination-
  archetype use case (same governance + federation + discovery
  story, different deployment profile)

## Out of scope for this design

- **Actual settlement finality.** Quidnug authorizes; the Fedwire /
  CHIPS / SWIFT gateway still settles. This design is the approval
  and audit layer, not replacement for the rails.
- **AML/KYC on the payee.** Standard compliance tooling still runs;
  the Quidnug layer attests that the bank's internal controls were
  followed.
- **Encryption of wire contents.** All signed data is in the open on
  the consortium chain. A production system would encrypt the wire
  payload and store a hash on-chain, decrypting via a separate
  key-management flow. That's a design axis orthogonal to trust.
