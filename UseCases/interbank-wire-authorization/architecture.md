# Architecture: Interbank Wire Authorization

## Participants

| Role                    | Represented as                     | Key material            |
|-------------------------|------------------------------------|-------------------------|
| Bank                    | Root quid `bank-us-wire`           | HSM-backed, rarely used |
| Approval officer        | Individual quid (e.g. `alice-op`)  | Personal HSM            |
| Compliance officer      | Individual quid                    | HSM, higher weight      |
| Correspondent bank      | Peer Quidnug node                  | Its own identity + set  |
| Regulator / observer    | Read-only Quidnug node              | Known public key        |

## Domain structure

```
wires.federal.us          (main domain — bank's approval flow)
├── wires.fed.chips.us    (CHIPS-specific quorum rules)
├── wires.fed.fedwire.us  (Fedwire-specific)
└── wires.international.swift  (SWIFT, international)
```

Each sub-domain can have its own validator threshold and its own
approval quorum — a CHIPS wire may require a different officer set
than a SWIFT wire.

## Identity hierarchy

```
                     ┌──────────────────────┐
                     │   bank-us-wire       │  (root bank quid)
                     │   GuardianSet:       │  Recovers ROOT.
                     │   {CEO, CFO, legal}  │  Threshold 2-of-3.
                     └──────────┬───────────┘
                                │
              ┌─────────────────┼─────────────────┐
              │                 │                 │
              ▼                 ▼                 ▼
        ┌──────────┐      ┌──────────┐     ┌────────────────┐
        │ alice-op │      │ bob-op   │     │carol-compliance│
        │          │      │          │     │                │
        │Guardian: │      │Guardian: │     │Guardian:       │
        │{spouse,  │      │{spouse,  │     │{spouse, mgr,   │
        │ mgr,     │      │ mgr,     │     │ legal}         │
        │ bkup HSM}│      │ bkup HSM}│     │                │
        └──────────┘      └──────────┘     └────────────────┘
```

Officers' personal guardian sets are **separate** from the bank's
approval set. Two distinct concerns:

- **Bank approval set** (`GuardianSet` for `bank-us-wire`): who can
  sign wires on the bank's behalf. Weighted: Alice(1), Bob(1),
  Carol(2). Threshold: 2.
- **Individual recovery sets** (one per officer quid): who can
  recover for Alice if her HSM fails. Personal — Alice picks her
  own guardians.

## Data flow: wire approval

```
┌─────────────────────────────────────────────────────────────────┐
│ Step 1. Core banking produces a wire instruction                 │
│   { payer, payee, amount, ref, date, rail }                      │
└─────────────────────────────────────────────────────────────────┘
                                 │
                                 ▼
┌─────────────────────────────────────────────────────────────────┐
│ Step 2. Pre-flight checks (amount → required quorum)             │
│                                                                   │
│   amount < 1M  → 1 signer required                               │
│   amount < 10M → 2 signers required                              │
│   amount ≥ 10M → compliance mandatory (carol-compliance)         │
└─────────────────────────────────────────────────────────────────┘
                                 │
                                 ▼
┌─────────────────────────────────────────────────────────────────┐
│ Step 3. Wire titled on-chain (first signer)                      │
│                                                                   │
│   POST /api/v1/titles                                            │
│   {                                                              │
│     assetId: "wire-2026-04-18-xyz",                              │
│     domain: "wires.fed.fedwire.us",                              │
│     owners: [{ ownerId: "bank-us-wire", percentage: 100 }],      │
│     attributes: {                                                │
│       payer: "...", payee: "...", amount: "50000000",            │
│       rail: "FEDWIRE", ref: "..."                                │
│     },                                                           │
│     signatures: { "alice-op": <sig_alice_nonce_5> }              │
│   }                                                              │
└─────────────────────────────────────────────────────────────────┘
                                 │
                                 ▼
┌─────────────────────────────────────────────────────────────────┐
│ Step 4. Second signer appends an event                           │
│                                                                   │
│   POST /api/v1/events                                            │
│   {                                                              │
│     subjectId: "wire-2026-04-18-xyz",                            │
│     subjectType: "TITLE",                                        │
│     eventType: "wire.cosign",                                    │
│     payload: { signerQuid: "bob-op", keyEpoch: 0 },              │
│     signature: <sig_bob_nonce_N>  (Bob's monotonically-advancing │
│                                   anchor nonce, not wire nonce)  │
│   }                                                              │
└─────────────────────────────────────────────────────────────────┘
                                 │
                                 ▼
┌─────────────────────────────────────────────────────────────────┐
│ Step 5. Compliance review (if required)                          │
│                                                                   │
│   Same shape as Step 4, but signerQuid = carol-compliance and    │
│   weight=2 counts toward threshold.                              │
└─────────────────────────────────────────────────────────────────┘
                                 │
                                 ▼
┌─────────────────────────────────────────────────────────────────┐
│ Step 6. Quorum check (node-side)                                 │
│                                                                   │
│   Sum of signer weights ≥ threshold AND                          │
│   every signer validly in the current GuardianSet AND            │
│   every signer's nonce strictly advances AND                     │
│   all signatures verify against current epoch keys.              │
│                                                                   │
│   If ✓: emit "wire.approved" event. The core banking system      │
│   polls for the approved event and instructs the rail.           │
└─────────────────────────────────────────────────────────────────┘
                                 │
                                 ▼
┌─────────────────────────────────────────────────────────────────┐
│ Step 7. Settlement confirmation (external rail)                  │
│                                                                   │
│   Core banking system calls back with settlement ID; another     │
│   event appended to the wire's stream:                           │
│                                                                   │
│   eventType: "wire.settled",                                     │
│   payload: { fedReference: "...", timestamp: ... }               │
└─────────────────────────────────────────────────────────────────┘
```

## Data flow: HSM failure recovery

Alice's HSM dies at 3am. Incident:

```
[t=0]   Alice's HSM enters fault state. All new signing fails.
        Node's lazy-epoch probe to peers may start flagging
        "Alice hasn't signed in N min" — operational signal only.

[t+10m] Ops confirms HSM is bricked. Initiates guardian recovery.

        POST /api/v2/guardian/recovery/init
        {
          kind: "guardian_recovery_init",
          subjectQuid: "alice-op",
          fromEpoch: 0, toEpoch: 1,
          newPublicKey: <hex from Alice's backup HSM or new token>,
          minNextNonce: <current+1>,
          maxAcceptedOldNonce: <current>,
          anchorNonce: <alice's anchor-nonce + 1>,
          validFrom: <now>,
          guardianSigs: [
            { guardianQuid: "alice-spouse", keyEpoch: 0, signature: ... },
            { guardianQuid: "alice-mgr",    keyEpoch: 0, signature: ... },
          ]
          // Alice's personal M-of-N recovery quorum met (2-of-3)
        }

[t+11m] Recovery accepted. Time-lock delay begins (default 1 hour).
        PendingRecovery stored; Alice can veto if she's actually fine.

[t+1h11m] No veto. Commit transaction lands automatically or is
        explicitly POSTed. Alice's currentEpoch advances to 1;
        new HSM's key is authoritative going forward.

[t+1h12m] Any in-flight wire approvals signed under the OLD epoch
        within the MaxAcceptedOldNonce window still settle. After
        that cap, old-epoch signatures are rejected.
```

Wires that Alice had partially approved before the HSM failure
complete via the original signatures — the rotation doesn't
retroactively invalidate her prior valid approvals.

## Data flow: officer departure

Alice leaves the bank. Three things happen:

1. **GuardianSetUpdate** on `bank-us-wire` — remove Alice from the
   approval set. Requires: bank's root-quid signature + current
   approval-set quorum (2 of {Alice, Bob, Carol}).
   
   Since Alice is the one leaving, the remaining signers (Bob +
   Carol) approve. Alice can opt to sign too or not — the threshold
   is already met without her if Carol(w=2) signs with Bob(w=1).

2. **GuardianResignation** — Alice can optionally withdraw from her
   own role proactively. Not strictly required since the set update
   removes her, but documents her exit in the chain.

3. **Alice's personal quid lives on**, in case any wires she signed
   before departure need post-hoc auditing. Her currentEpoch is
   frozen (invalidation anchor); no new approvals from her key work.

## Quorum math edge cases

### Weighted threshold with compliance

```
GuardianSet:
  { alice-op (w=1), bob-op (w=1), carol-compliance (w=2) }
  threshold: 2
```

- Alice + Bob → 2. ✓
- Alice + Carol → 3. ✓ (over-threshold is fine)
- Carol alone → 2. ✓ (compliance officer's weight covers the
  threshold alone — this is deliberate for $10M+ wires where the
  bank's policy lets compliance unilaterally authorize; check if
  that's actually what you want or if threshold should be 3)

To prevent the "Carol alone" case, raise threshold to 3:

```
threshold: 3
```

Now Carol alone (w=2) is insufficient; a line officer must also sign.

### Weighted threshold with distrust of Carol

If the bank policy is that compliance is a check-and-balance (not a
unilateral approver), set threshold=3 and keep Carol at w=2. Means
compliance can't act alone; always requires an operator.

## Consortium trust graph

Correspondent banks operate their own nodes. The trust setup:

```
bank-us-wire ──0.95──► bank-correspondent-london
    ▲
    │ 0.9
    │
bank-us-wire ──0.85──► bank-regulator-occ  (regulator observer)
    ▲
    │ 0.8
    │
bank-us-wire ──0.7──► small-counterparty-bank  (smaller party)
```

From the correspondent's perspective, when they receive a wire
title and event stream claiming the bank approved it, they verify:

1. Every signer is in `bank-us-wire`'s current `GuardianSet` as of
   the approval's block height.
2. Every signature verifies against the signer's current-epoch key
   (from the blockchain's identity registry).
3. Nonces are monotonic.
4. Threshold is met.

If the correspondent hasn't seen the bank's recent guardian-set
updates (partition, just rejoined the network), push gossip
(QDP-0005) delivers them on demand, and the lazy-epoch probe
(QDP-0007) catches edge cases.

## Scale estimates

Typical US mid-sized bank:
- 2,000 high-value wires/day
- 50 authorized approval officers across regions
- 5 compliance officers
- ~20 correspondent bank peers

Workload on a single Quidnug node:
- 2,000 titles/day = ~100 KB/day in titles
- ~6,000 events/day (avg 3 signatures per wire)
- ~100 KB/day peer gossip for inter-bank propagation

Quidnug's consortium-scale target (thousands of TPS with many
correspondents) is comfortable headroom above this workload.

## Next

See [`implementation.md`](implementation.md) for concrete code,
[`threat-model.md`](threat-model.md) for security analysis.
