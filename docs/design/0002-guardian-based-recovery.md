# QDP-0002: Guardian-Based Recovery

| Field         | Value                                       |
|---------------|---------------------------------------------|
| Status        | Draft                                       |
| Track         | Protocol (hard fork)                        |
| Author        | The Quidnug Authors                         |
| Created       | 2026-04-18                                  |
| Supersedes    | —                                           |
| Requires      | QDP-0001 (Global Nonce Ledger)              |
| Implements in | v2.2 (target; strictly after v2.0 anchors)  |

## 1. Summary

[QDP-0001](0001-global-nonce-ledger.md) introduces single-signer anchors
for key rotation and compromise recovery but explicitly leaves one hole
open: the "anchor race." When an attacker holds a compromised key, they
can publish an `AnchorRotation` under that key *before* the legitimate
owner does, effectively stealing the quid's identity.

This document proposes **guardian-based recovery**: a signer designates
a set of guardian quids at identity creation (or later), declares a
threshold `M-of-N` for authorizing recovery, and sets a time-locked
recovery delay. A rotation initiated by guardians is not instant — it
waits out the delay during which the original key holder can veto,
giving legitimate owners a window to respond to alarm signals.

Motivating attack scenario solved by this proposal:

> An attacker exfiltrates quid `Q`'s private key through a phishing
> attack. Under QDP-0001 alone, the attacker immediately publishes an
> `AnchorRotation` to their own key and locks the legitimate owner out
> permanently. Under this proposal, the rotation requires either (a) the
> owner's key plus a `recoveryDelay` of zero — the normal self-rotation
> path, which the attacker can still abuse — *or* (b) signatures from
> `M` of `N` guardians. If the owner has declared guardians, the
> attacker's single-key rotation either doesn't work (if guardian
> signatures are required) or enters a `recoveryDelay` window during
> which the owner or guardians can publish a veto.

This proposal also addresses the related problem of "dead key" recovery:
a key whose owner has lost access (hardware failure, password loss,
death) but with no attacker involved. Today there is no recovery path.

## 2. Background

### 2.1 What QDP-0001 provides

- `AnchorRotation` signed by the old key, introducing a new key epoch.
- `AnchorInvalidation` to freeze an old epoch.
- `AnchorEpochCap` to watermark an old epoch.
- All anchors are single-signer: signed by the current primary key.

### 2.2 Why single-signer anchors are insufficient

- **Anchor race (primary concern).** See §1.
- **Dead-key paralysis.** If the primary key is lost (not compromised,
  just gone), the quid is frozen. No rotation is possible. Every trust
  relationship, identity record, and title held by that quid becomes
  inert; the network has no way to let the legitimate owner re-emerge
  under a new key.
- **No graduated trust.** A quid is either fully controlled by its
  primary key or not at all. There is no way to express "I want this
  operation to be authorized by me *and* two people I trust."

### 2.3 Prior art

- **Ethereum "social recovery" wallets** (Argent, Gnosis Safe): multi-
  signature gating on key rotation with a time-lock window. Closely
  matches this proposal's structure.
- **Signal Sealed Sender & safety numbers**: orthogonal — deals with
  message-level identity, not account recovery.
- **Shamir Secret Sharing** (SLIP-0039 et al.): key reconstruction
  from M-of-N shares. Operationally similar but cryptographically
  different; see §11.
- **Trusted Computing anchor attestation** (TPM, Apple Secure
  Enclave): hardware-backed attestation. Complementary; a guardian
  could use a hardware-protected key. Out of scope for the protocol
  itself.

## 3. Problems

### 3.1 Anchor race (CRITICAL)

An attacker with a compromised key can publish a rotation to their own
key, permanently capturing the quid. The legitimate owner has no
recourse — by the time they notice, the rotation is in the Trusted
chain.

### 3.2 Key loss is permanent (HIGH)

No recovery path if the primary key is lost. This is both a user
experience problem (users will burn identities when they lose devices)
and a concentration-of-risk problem (nobody will delegate important
state to a quid that can be bricked by a spilled coffee).

### 3.3 No separation of duties (MEDIUM)

A single key is a single point of both compromise and authorization. For
institutional quids (e.g., a trust-domain validator that represents a
standards body), this is unacceptable; standards bodies don't allow a
single individual to unilaterally rotate their identity.

### 3.4 No inheritance / succession (MEDIUM)

There is no protocol path for a quid to pass to a successor on its
owner's death or dissolution. Any solution today is out-of-band and
requires the network to trust a claimant's unverifiable assertion.

## 4. Goals and non-goals

### 4.1 Goals

- **G1.** A compromised primary key alone cannot capture a quid that
  has designated guardians, provided fewer than `M` guardians are
  simultaneously compromised.
- **G2.** A quid whose primary key is lost can be recovered to a new
  key, given `M` of `N` guardian authorizations.
- **G3.** The primary key holder retains full unilateral control for
  *normal* rotation — guardians exist to handle abnormal cases.
- **G4.** The legitimate owner has a veto window during guardian-
  initiated rotation, so a coerced or malicious guardian action can be
  halted.
- **G5.** Guardian sets can be updated: add, remove, or replace
  guardians; change the threshold; change the recovery delay.
- **G6.** Guardians themselves can be quids that use guardian-based
  recovery. The recursion is bounded (see §6.7).
- **G7.** No change to the underlying signature algorithm (still
  ECDSA-P256). Guardian consent is `M` independent signatures.

### 4.2 Non-goals

- **N1.** Preventing `M` guardians from colluding with each other or
  with an attacker. This is inherent to any `M-of-N` scheme.
- **N2.** Hiding guardian identities. Guardians are public in the
  quid's identity record; this is a feature, not a limitation (§6.2).
- **N3.** Threshold signatures (FROST / GG20). See §11 for rationale.
- **N4.** On-chain proof of liveness for guardians. A guardian who has
  disappeared looks the same as one who is merely quiet; §7.4 discusses
  the operational implications.
- **N5.** Recovering from a *majority* of guardians being compromised.
  That's outside the design's trust assumption.

## 5. Threat model

### 5.1 Adversary capabilities

| Capability                                         | In scope? |
|----------------------------------------------------|-----------|
| Compromise the primary key                         | Yes       |
| Compromise up to `M-1` of `N` guardians            | Yes       |
| Observe all public transactions and anchors        | Yes       |
| Delay or block messages from the legitimate owner  | Yes, bounded by `recoveryDelay` |
| Coerce 1 guardian via social/legal means           | Yes       |
| Coerce `M` guardians simultaneously                | No        |
| Break ECDSA-P256                                   | No        |
| Compromise the primary key *and* `M-1` guardians simultaneously | No — see §11.3 |

### 5.2 Owner capabilities

- The legitimate owner is online often enough to notice a guardian-
  initiated recovery within `recoveryDelay`.
- Or: if the owner is truly offline, the recovery is presumed
  legitimate after the delay. This is the "owner is dead or has lost
  the key" case; it cannot be distinguished from the "owner is simply
  on vacation" case, and the design treats them identically. Operators
  who need stronger liveness guarantees should choose longer delays.

## 6. Design

### 6.1 Guardian set data model

Each quid optionally carries a `GuardianSet` in its identity record:

```go
type GuardianSet struct {
    Guardians     []GuardianRef     // N ≥ 1 entries
    Threshold     uint8             // M; 1 ≤ M ≤ N
    RecoveryDelay time.Duration     // bounded by [MinRecoveryDelay, MaxRecoveryDelay]
    UpdatedAtBlock int64            // block at which this set was committed

    // Optional: cap on simultaneous recovery attempts to prevent DoS
    MaxConcurrentRecoveries uint8   // default 1
}

type GuardianRef struct {
    Quid          string     // guardian's quid ID
    Weight        uint16     // 1 by default; used for weighted voting
    Epoch         uint32     // which key epoch of the guardian
    AddedAtBlock  int64
}
```

Notes:

- `Threshold` is a weighted sum: a recovery is authorized when the sum
  of `Weight` across signing guardians ≥ `Threshold`. In the simplest
  case (all weights = 1), this reduces to M-of-N.
- `Epoch` pins the guardian's key version at the time of guardian
  designation. This prevents a rotated-away key from counting toward
  recovery unless the guardian set is explicitly updated.
- Network-wide constants `MinRecoveryDelay = 1 hour` and
  `MaxRecoveryDelay = 365 days`. Inside these bounds the owner picks.

### 6.2 Public visibility of guardians

Guardians are published in the identity record and therefore visible to
anyone who can query the identity. This is **intentional**:

- It lets relying parties (applications, other quids) see who can
  authorize recovery and reason about the trustworthiness of the
  arrangement.
- It lets guardians know they are guardians (required for consent —
  you cannot unwittingly be a guardian).
- It makes social-engineering attacks harder because the attacker
  must target a known, public set.

A future QDP could introduce **private guardians** via cryptographic
commitments (publish a hash of the guardian set; reveal on use), but
that is out of scope here.

### 6.3 Recovery state machine

```
                  init by owner (self-rotation, no delay)
     (Idle) ──────────────────────────────────────────────▶ (Done)
       │
       │ init by M-of-N guardians
       ▼
    (Pending)
     ├── veto by primary key within recoveryDelay ─────────▶ (Vetoed)
     ├── veto by M-of-N guardians within recoveryDelay ────▶ (Vetoed)
     ├── `recoveryDelay` elapses in Trusted chain ─────────▶ (Done)
     └── superseded by a later pending recovery ───────────▶ (Replaced)
```

Each transition is driven by a signed anchor. State is materialized in
the quid's identity record and advanced at block-acceptance time.

### 6.4 Anchor extensions

QDP-0001 defined three anchor kinds (`Rotation`, `Invalidation`,
`EpochCap`). This proposal adds four:

```go
type AnchorKind int
const (
    AnchorRotation          AnchorKind = iota + 1 // existing, QDP-0001
    AnchorInvalidation                            // existing, QDP-0001
    AnchorEpochCap                                // existing, QDP-0001
    AnchorGuardianRecoveryInit                    // new
    AnchorGuardianRecoveryVeto                    // new
    AnchorGuardianRecoveryCommit                  // new
    AnchorGuardianSetUpdate                       // new
)
```

#### 6.4.1 AnchorGuardianRecoveryInit

Initiates a time-locked recovery. Signed by ≥ `Threshold` guardians.

```go
type GuardianRecoveryInit struct {
    Kind                AnchorKind // AnchorGuardianRecoveryInit
    SubjectQuid         string     // whose recovery this is
    FromEpoch           uint32
    ToEpoch             uint32
    NewPublicKey        string     // SPKI hex of the target key
    MinNextNonce        int64
    MaxAcceptedOldNonce int64
    AnchorNonce         int64      // strictly monotonic per subject
    ValidFrom           int64
    ExpiresAt           int64      // ValidFrom + recoveryDelay + grace

    // M signatures over the above fields, one per participating guardian
    GuardianSigs []GuardianSignature
}

type GuardianSignature struct {
    GuardianQuid string
    KeyEpoch     uint32
    Signature    string // ECDSA-P256
}
```

Validation:
- Every `GuardianQuid` is a current member of `SubjectQuid`'s
  `GuardianSet`.
- Every `KeyEpoch` matches the guardian's pinned epoch at set time (or
  a later epoch authorized by `AnchorGuardianSetUpdate`).
- The sum of `Weight` across signing guardians ≥ `Threshold`.
- `AnchorNonce > lastRecoveryAnchorNonce[SubjectQuid]`.
- At most `MaxConcurrentRecoveries` are in `Pending` state at any time.

On Trusted inclusion, the subject quid's identity record gains a
`PendingRecovery` entry with expiration `ValidFrom + recoveryDelay`.
Normal transactions from the subject continue to validate against the
current (not yet rotated) key during the delay.

#### 6.4.2 AnchorGuardianRecoveryVeto

Cancels a pending recovery. Signed by either:

- The subject's current primary key (one signature, epoch =
  current-epoch), or
- ≥ `Threshold` guardians (same rules as Init).

```go
type GuardianRecoveryVeto struct {
    Kind              AnchorKind // AnchorGuardianRecoveryVeto
    SubjectQuid       string
    RecoveryAnchorHash string    // hash of the Init anchor being vetoed
    AnchorNonce       int64
    ValidFrom         int64

    // Exactly one of:
    PrimarySignature  *PrimarySignature     // owner veto
    GuardianSigs      []GuardianSignature   // threshold veto
}

type PrimarySignature struct {
    KeyEpoch  uint32
    Signature string
}
```

Importantly, the primary-key veto is the legitimate owner's fast path:
one signature, immediate effect (at block acceptance). The guardian-
threshold veto exists for the rare case where guardians themselves
detect that the recovery was fraudulently initiated (e.g., a guardian
who signed under duress later disavows).

#### 6.4.3 AnchorGuardianRecoveryCommit

Finalizes a recovery whose delay has elapsed. Any participant can
publish this anchor; it's a commitment, not an authorization — the
authorization was the Init anchor.

```go
type GuardianRecoveryCommit struct {
    Kind               AnchorKind // AnchorGuardianRecoveryCommit
    SubjectQuid        string
    RecoveryAnchorHash string     // hash of the Init anchor being committed
    AnchorNonce        int64
    ValidFrom          int64

    // Signed by the committer (any quid); its signature is not
    // load-bearing for authorization but makes the commit traceable.
    CommitterQuid string
    CommitterSig  string
}
```

Validation:
- The referenced Init anchor is in `Pending` state.
- Current block time ≥ Init's `ValidFrom + recoveryDelay`.
- No veto was accepted for this Init.

On Trusted inclusion, the subject quid rotates: `CurrentEpoch`
advances to `ToEpoch`, and the new key takes effect. From this moment
on, any transaction using an old-epoch key is rejected (per QDP-0001's
`MaxAcceptedOldNonce` logic).

#### 6.4.4 AnchorGuardianSetUpdate

Installs or modifies a guardian set. Authorization depends on whether a
guardian set currently exists:

- **No current set (first-time install):** signed by primary key only.
- **Current set exists:** signed by primary key **plus** ≥
  `Threshold` of current guardians. This is deliberately stricter
  than recovery — changing who can recover you is at least as
  consequential as recovering, and it happens far less often.

```go
type GuardianSetUpdate struct {
    Kind        AnchorKind // AnchorGuardianSetUpdate
    SubjectQuid string
    NewSet      GuardianSet
    AnchorNonce int64
    ValidFrom   int64

    PrimarySignature    *PrimarySignature
    NewGuardianConsents []GuardianSignature // required: consent from every guardian in NewSet
    CurrentGuardianSigs []GuardianSignature // required when replacing: threshold of CURRENT set
}
```

*Implementation note (finalized wire format):* the field previously
sketched as `GuardianSigs` was split into `NewGuardianConsents`
(required always, covering every guardian in the proposed new set)
and `CurrentGuardianSigs` (required only when replacing, carrying
≥ threshold of the *current* set). The split makes the two
authorization roles textually distinct and removes the ambiguity of
one field serving two different semantic purposes.

Additional validation:
- Every proposed `Guardian` quid must consent by either (a) signing the
  update themselves, or (b) having consented via an out-of-band
  `GuardianConsent` attestation stored in a preceding identity update.
  Proposal: require (a) — explicit on-chain consent — because a
  guardian who doesn't know they are one cannot perform the role.

### 6.5 Interaction with QDP-0001's plain AnchorRotation

The existing `AnchorRotation` (signed by the current primary key, no
delay) remains available and unchanged. It represents the "I want to
rotate my key right now and I still have my key" path. Operators may
choose to disable it in favor of guardian-only rotations by including a
`RequireGuardianRotation: true` flag in their `GuardianSet`. When set,
all non-guardian rotations are rejected even if the primary key signs
them. This option is for high-security accounts (validators for
regulated trust domains, for example) and trades convenience for
attack-resistance.

### 6.6 Ordering under Proof-of-Trust

Recovery anchors propagate through the existing Trusted / Tentative /
Untrusted tier structure. Per QDP-0001 §6.4, a recovery initiated in a
Tentative block reserves anchor-nonce space. If the Tentative block is
demoted, the reservation is released and a different Init anchor can
use the same nonce.

This matters because recovery initiation is a privileged, one-shot
operation: we don't want two concurrent Init anchors for the same
subject to both take effect. The anchor-nonce monotonicity rule
(QDP-0001 §6.5) guarantees that only one Init advances the counter;
the other is rejected as stale.

### 6.7 Guardian recursion

A guardian `G` that itself uses guardian-based recovery introduces a
question: what happens during subject `S`'s recovery if `G` is itself
in `Pending` recovery state?

Rule: `G`'s `Pending` state does not affect its ability to sign as a
guardian for `S`, because signing is performed with `G`'s current
(pre-recovery) key at its current epoch. After `G`'s own recovery
completes, `G`'s pinned epoch in `S`'s guardian set is stale —
`S` must perform an `AnchorGuardianSetUpdate` to refresh `G`'s pinned
epoch before `G` can participate in future recoveries for `S`. This
prevents a subtle attack where `G`'s old (compromised) key is used to
help recover `S` after `G`'s own key was rotated away.

### 6.8 Bootstrap: first-time quid creation

At identity creation, the owner may declare a `GuardianSet` in the
initial `IdentityTransaction`. If declared:

- Every listed guardian's `GuardianConsent` signature must be present
  in the same transaction (or in a companion transaction in the same
  block).
- `GuardianConsent` is a short signed attestation: "I, guardian `G`,
  consent to act as guardian for subject `S` under terms `{hash of
  GuardianSet}`. This consent applies until revoked."

If not declared at creation, the owner can install one later via
`AnchorGuardianSetUpdate` signed only by the primary key (§6.4.4).

## 7. Storage, performance, and ergonomics

### 7.1 Storage

Each quid's guardian set adds `~256 bytes × N` to the identity record,
plus the pending-recovery sub-record when a recovery is in flight.
For `N = 5` guardians (a reasonable operator default) and identities
without pending recoveries, the overhead is under 2 KB per quid.

Recovery anchors carry `M` signatures: `64 bytes × M` plus a
`GuardianRef` per signer. At `M = 3`, an Init anchor is ~400 bytes.

### 7.2 Performance

Recovery anchor verification is `O(M)` ECDSA verifications — the bulk
of the cost. For a network sealing blocks every 60 seconds with at
most a few recovery anchors per block, this is negligible.

### 7.3 Ergonomics

Publishing a recovery Init anchor requires `M` guardians to coordinate
signatures. In practice this means:

- Client software must support "partial signatures" that can be
  collected over email, Signal, or dedicated recovery coordinators.
- A reference client is out of scope for this QDP but is on the
  roadmap.

### 7.4 Guardian liveness

A guardian who has lost their key or has died cannot sign. This is
fine as long as at least `M` of `N` remain responsive. Operators
should choose `N - M ≥ 2` to tolerate at least two simultaneous
guardian failures.

## 8. Security analysis

### 8.1 Invariants

- **I1.** Only a current guardian's signature at its pinned epoch (or
  a later epoch authorized by a SetUpdate) counts toward the threshold.
- **I2.** A pending recovery is vetoable until its commit time; after
  commit, state advances irrevocably.
- **I3.** The primary-key veto is always available during the delay
  window, regardless of how many guardians signed the Init.
- **I4.** Guardian set updates during a pending recovery do not take
  effect until either the recovery completes or is vetoed. This
  prevents the attacker from initiating recovery and then updating the
  guardian set mid-flight to evade veto.
- **I5.** Anchor-nonce monotonicity from QDP-0001 applies to all
  guardian anchors.

### 8.2 Threat coverage

| Attack                                                          | Mitigation |
|-----------------------------------------------------------------|------------|
| Compromised primary key publishes rotation                      | Vetoable by the Init anchor → primary-key veto path, but only useful if guardians are declared AND owner notices. Operators can require guardian rotation (§6.5). |
| Compromised primary key + coerced guardian publishes rotation   | Still below `Threshold`. If `M-1` guardians compromise: prevented. |
| Compromised primary key + M guardians collude                   | Not prevented; outside threat model (§5.1). |
| Attacker publishes Init and races veto                          | `recoveryDelay` bounds the race. Primary-key veto is one signature, fast. |
| Attacker vetoes legitimate guardian recovery                    | The attacker is presumed to have the primary key (since veto is primary-signed). The legitimate owner must then rely on guardians and longer windows; limit case is §5.1's "attacker has everything" bound. |
| Attacker replays an old recovery Init                           | Anchor-nonce strictly increasing per subject; old Init rejected. |
| Attacker publishes guardian set update to swap in colluders     | Requires primary key + current-guardian threshold. Attacker who has both has already won. |
| Guardian key compromise undetected                              | One compromised guardian is below `M`; benign until `M` are compromised. Periodic `AnchorGuardianSetUpdate` to refresh pinned epochs mitigates. |

## 9. Migration and compatibility

This QDP is additive to QDP-0001. Nodes running v2.0 (QDP-0001 only)
that receive v2.2 anchors treat them as `AnchorKind = unknown` and
discard them silently. This is compatible as long as no v2.0 node is
consensus-critical for a v2.2 trust domain — which is the deployment
guidance from §14 of QDP-0001 (fork-block H).

Existing v2.0 quids that want to declare guardians use the normal
post-creation `AnchorGuardianSetUpdate` path.

## 10. Wire formats

Additions to `/api/v2` (partial):

- `POST /api/v2/anchors/guardian-recovery-init` — submit a recovery
  Init. Request body is the `GuardianRecoveryInit` struct.
- `POST /api/v2/anchors/guardian-recovery-veto` — submit a veto.
- `POST /api/v2/anchors/guardian-recovery-commit` — commit a mature
  pending recovery.
- `POST /api/v2/anchors/guardian-set-update` — update a guardian set.
- `GET /api/v2/identities/{quid}/recovery-state` — returns the current
  recovery state (`Idle | Pending | Vetoed | Replaced`) and, if
  pending, the maturation block height.

## 11. Alternatives considered

### 11.1 Shamir Secret Sharing (SSS)

Split the primary key into `N` shares such that any `M` can
reconstruct it. Rotation is simply reconstruction + re-encryption.

- **Pro.** No protocol change. Pure client-side.
- **Con.** At reconstruction, the full key is materialized somewhere.
  Any compromise during that moment is total. Also, SSS does not
  produce a verifiable on-chain record of *who* participated in the
  recovery, which is exactly the accountability property we want.
- **Con.** SSS doesn't support "veto by the owner" because the owner
  doesn't participate in the reconstruction — they hold a share or
  they don't.

Rejected: the on-chain accountability and veto window are worth the
protocol changes.

### 11.2 Threshold signatures (FROST / GG20 / ROAST)

`M-of-N` participants jointly produce a single ECDSA or Schnorr
signature. The resulting signature is indistinguishable from a normal
signature.

- **Pro.** Compact signatures; no verifier change.
- **Pro.** The same key works across all participants — no
  per-guardian `GuardianSig`.
- **Con.** Requires a new distributed key generation (DKG) protocol
  at quid creation and for every rotation. DKG is cryptographically
  involved and has a history of subtle security flaws in
  implementations.
- **Con.** Loses the public on-chain trail of "who authorized this
  recovery" — the signature is single, anonymous among the threshold
  set.
- **Con.** Guardian rotation requires re-running DKG with the new
  set, which is expensive and has its own attack surface.

Rejected for v2.2. A future QDP-0005 could introduce threshold
signatures as an *alternative* mechanism for quids that value
compactness and privacy over auditability.

### 11.3 Hardware-rooted recovery

Each quid has a hardware-backed recovery key (YubiKey, HSM, Apple
Secure Enclave). Recovery is "produce a signature with the hardware
recovery key."

- **Pro.** Strong against remote attackers.
- **Con.** Single point of failure; hardware loss = permanent lockout.
- **Con.** Composable with this proposal, not a substitute: a
  hardware-rooted key *is* a valid guardian.

Not rejected; complementary. Users are encouraged to make one
guardian a hardware-backed key.

### 11.4 External recovery services

Centralized custodian (Magic, Privy, Coinbase Recovery) holds recovery
authorization.

- **Pro.** Simple UX.
- **Con.** Centralization; incompatible with Quidnug's decentralization
  premise; regulatory vulnerability (subpoenas to the custodian).

Rejected at the protocol level. Users may still use such services by
designating the service's quid as a guardian, at their own risk.

### 11.5 Time-lock without guardians

"Any rotation to a new key takes effect in T days unless canceled by
the old key." Zero-trust alternative to guardian-based recovery.

- **Pro.** Simple; no guardian coordination required.
- **Con.** Does not solve the dead-key case (if the old key is gone,
  the rotation can never complete because it can't be canceled
  either). Inverse of what we want.

Rejected as a standalone solution. The veto window in this proposal
(§6.4.2) is the bounded-trust version of this alternative.

## 12. Open questions

1. **`GuardianConsent` revocation.** How does a guardian withdraw?
   Proposal: a `GuardianResign` anchor signed by the guardian; the
   subject must perform a `GuardianSetUpdate` to formally remove the
   guardian, but during the interim the resigned guardian's signature
   no longer counts toward `Threshold`. Open: how long is the interim?

2. **Dormant-guardian auto-pruning.** If a guardian hasn't signed
   anything in `T` time, should the subject be notified? Proposal:
   include a liveness beacon mechanism in the reference client
   (off-chain), but don't encode it in the protocol.

3. **Recovery audit trail retention.** Does the network keep a
   permanent record of past recoveries, or can they be pruned after
   some time? Proposal: permanent. Recoveries are important enough
   that historical auditability is worth the storage cost.

4. **Subject-initiated guardian proposal.** Currently only the
   subject (primary key) can propose a guardian set update. Should
   a threshold of current guardians be able to propose adding a new
   guardian to address a perceived weakness, *without* the subject's
   signature? Proposal: no — the subject defines their own trust
   boundaries, and guardians should not be able to expand their own
   set. Open to reconsideration for institutional quids.

5. **Interaction with `AnchorInvalidation`.** What happens if a
   recovery is pending and an invalidation of the current epoch lands?
   Proposal: the invalidation takes precedence; pending recovery is
   canceled; the subject must initiate a new recovery to establish a
   new epoch. This matches the "invalidation is an emergency button"
   framing from QDP-0001.

## 13. Test plan

### 13.1 Unit tests

- Guardian set validation: threshold bounds, weight sums, duplicate
  guardians, self-guardianship rejection.
- Anchor verification for each of the four new kinds.
- Recovery state-machine transitions (Idle → Pending → Done, and every
  veto/replace path).
- Anchor-nonce monotonicity including interaction with QDP-0001's
  anchor-nonce rules.

### 13.2 Integration tests

- **Happy-path recovery.** Subject declares 3-of-5 guardians, delay of
  1 hour. 3 guardians initiate. No veto. After delay + block, rotation
  commits. Subject transacts with new key.
- **Primary-key veto.** Guardians initiate, subject vetoes within the
  delay. Pending clears. Subject continues under old key.
- **Guardian-threshold veto.** Guardians initiate, then 3 guardians
  sign a veto. Pending clears.
- **Attacker race.** Attacker has primary key. Publishes
  `AnchorRotation` (normal, no delay). Also in the same block,
  guardians publish an `AnchorGuardianRecoveryInit`. The attacker's
  rotation takes effect immediately; guardians' Init is rejected as
  stale-epoch (because the primary key's rotation already advanced
  the epoch). Mitigation: enable `RequireGuardianRotation` (§6.5).

### 13.3 Adversarial property tests

Generate random guardian set configurations and random streams of
authorized / unauthorized anchors. Property: after any sequence of
anchor inclusions, the subject's effective key is a key authorized by
some accepted lineage of anchors.

## 14. Rollout

Follows the same template as QDP-0001 §14.

- **T-0** ship v2.1: accept guardian anchors in parse path, ignore
  semantically. Observe adoption.
- **T+30d** enable guardian semantics on a single test domain.
- **T+60d** enable on production domains.
- **T+90d** v2.2 is the minimum version for operators running
  high-value trust domains.

Observability:

- `quidnug_guardian_recovery_initiated_total`
- `quidnug_guardian_recovery_vetoed_total{by="primary"|"guardian"}`
- `quidnug_guardian_recovery_committed_total`
- `quidnug_guardian_set_updated_total`
- `quidnug_pending_recoveries` (gauge)

## 15. References

- [QDP-0001: Global Nonce Ledger](0001-global-nonce-ledger.md)
- [src/core/types.go](../../src/core/types.go) — `IdentityTransaction`
- [src/core/validation.go](../../src/core/validation.go) — current
  identity-update validation
- Argent Wallet technical documentation — social-recovery prior art
- SLIP-0039 — Shamir Secret Sharing for mnemonic backups
- FROST paper (Komlo & Goldberg, 2020) — threshold Schnorr
- EIP-4337 (Account Abstraction) — recovery-oriented account design
  patterns on Ethereum

---

**Review status.** Draft. Required sign-off before merge: (a) §6.4.4
guardian-set update authorization rules, (b) §6.7 recursion semantics,
(c) §11.2 threshold-signatures rejection rationale, (d) §12.4
subject-vs-guardian authority split.
