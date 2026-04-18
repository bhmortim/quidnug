# QDP-0006: Guardian-Consent Revocation (H6)

| Field      | Value                                                    |
|------------|----------------------------------------------------------|
| Status     | Draft                                                    |
| Track      | Protocol                                                 |
| Author     | The Quidnug Authors                                      |
| Created    | 2026-04-18                                               |
| Supersedes | —                                                        |
| Requires   | QDP-0002 (guardian-based recovery, landed)               |
| Implements | Phase H6 of QDP-0004 roadmap                             |
| Target     | v2.4                                                     |

## 1. Summary

QDP-0002 §12.1 left guardian revocation unresolved: a guardian
who has consented to be in subject `S`'s set has no on-chain way
to withdraw that consent later. Reasons range from the mundane
(guardian stepped back from the role) to the urgent (guardian's
key was compromised and they don't want to be reachable for
recovery anymore).

This document specifies `AnchorGuardianResign` — a new anchor
kind, signed by the resigning guardian, that withdraws their
consent to participate in a named subject's recovery quorum.
From the effective time, the guardian's signature no longer
counts toward the threshold; the subject's set is otherwise
preserved, and a subsequent `GuardianSetUpdate` can install a
replacement.

## 2. Problem statement

A guardian's relationship to a subject is long-lived and
consent-bound. Today that consent is expressed once (at
`GuardianSetUpdate` time via `NewGuardianConsents`) and never
revocable. Three concrete failure modes:

1. **Compromised guardian.** Alice's personal device is stolen.
   Alice is a guardian for Bob. Alice wants to tell the network
   "my signatures on Bob's recovery no longer count" without
   having to ask Bob to do a `GuardianSetUpdate` (which requires
   Bob's primary key plus current-threshold consent — moves the
   liability to Bob).
2. **Coerced guardian.** Carol is pressured to help an attacker
   recover to a key they control. Carol wants an escape hatch
   that withdraws her authority before the recovery completes.
3. **Role change.** Dave is an organizational guardian who leaves
   the organization. The organization's successor doesn't yet
   hold a guardian slot; Dave wants his slot to be inert until
   the subject explicitly reshapes the set.

Without revocation, all three paths require the subject's
cooperation. In (1) and (2) the subject may be unreachable or
uninterested. In (3) the subject has no signal that the change
has happened.

## 3. Goals and non-goals

**Goals.**

- **G1.** Any guardian can revoke their participation in a
  named subject's set without the subject's cooperation.
- **G2.** Revocation is strictly future-looking: it does NOT
  invalidate signatures already provided to a pending
  recovery. In-flight recoveries proceed on the set as it was
  when the recovery initiated.
- **G3.** Revocation is self-contained per `(guardian, subject)`
  pair: a guardian who is in multiple subjects' sets revokes
  each independently. No cascading revocation.
- **G4.** Set threshold is NOT auto-adjusted. If resignation
  drops effective weight below threshold, the set enters a
  "weakened" state observable via metrics; recovery still
  works, just requires greater coordination from the remaining
  guardians.
- **G5.** Duplicate revocations are idempotent — same
  `(guardian, subject, set-hash)` pair accepted more than once
  returns a success response without a second state change.

**Non-goals.**

- **NG1.** Forced removal of a guardian by the subject. That
  path already exists: a `GuardianSetUpdate` removing the old
  guardian, authorized by primary + current-threshold. The
  current guardian may decline to sign; authorization requires
  only threshold, not unanimity.
- **NG2.** Delegated revocation ("I authorize my lawyer to
  resign on my behalf"). Out of scope for the protocol; an
  operator workflow.
- **NG3.** Emergency instantaneous effect. Resignations take
  effect at `EffectiveAt`, which the guardian chooses; a
  malicious or distressed guardian can pick `now` for
  immediate effect, but cannot retroactively unwind
  signatures already given.

## 4. Threat model

| Threat                                                            | Mitigation                                                                                                                  |
|-------------------------------------------------------------------|-----------------------------------------------------------------------------------------------------------------------------|
| Attacker who doesn't control the guardian key resigns falsely     | Resignation requires the guardian's signature at the guardian's current epoch — same trust as any other guardian signature. |
| Guardian resigns after signing an Init, hoping to unwind it       | Resignation is prospective only (G2). Init + delay + Commit proceeds on set-at-Init-time.                                   |
| Replay of an old resignation across set versions                  | Resignation carries `GuardianSetHash` — it is valid only against the exact set it references.                               |
| Distressed guardian resigns from all subjects simultaneously      | No protocol prevention. Subjects observing the `guardian_resigned` metric can re-shape their sets.                          |
| Timestamp manipulation (`EffectiveAt` in past / far future)       | Validation: must be `>= now - 5 min` and `<= now + 365 days`.                                                               |
| Two competing resignations from the same guardian                 | Per-guardian, per-subject monotonic `ResignationNonce`. A later resignation with the same nonce is rejected.                |

## 5. Data model

### 5.1 Wire format

```go
// AnchorGuardianResign: seventh anchor kind. A guardian
// withdraws their consent to participate in a named subject's
// recovery quorum. Signed by the guardian at their current
// epoch.
type GuardianResignation struct {
    Kind              AnchorKind `json:"kind"`
    GuardianQuid      string     `json:"guardianQuid"`
    SubjectQuid       string     `json:"subjectQuid"`

    // GuardianSetHash pins the exact set this resignation
    // applies to. If the subject updates the set after this
    // resignation is signed, the resignation no longer matches
    // and is rejected with ErrResignationSetHashMismatch.
    GuardianSetHash   string     `json:"guardianSetHash"`

    // ResignationNonce is a per-(guardian, subject) monotonic
    // counter, keyed separately from other anchor streams.
    // Prevents replay across resignations.
    ResignationNonce  int64      `json:"resignationNonce"`

    // EffectiveAt is the unix timestamp from which the
    // resignation takes effect. MUST be >= now - 5 min
    // (small past tolerance for clock skew) and <= now + 1
    // year. Until EffectiveAt the resignation is stored but
    // the guardian's authority is unchanged.
    EffectiveAt       int64      `json:"effectiveAt"`

    Signature         string     `json:"signature"`
}
```

A new `GuardianResignationTransaction` wraps the anchor for
block inclusion, matching the existing guardian-anchor
wrapper pattern.

### 5.2 Ledger state

A new map on `NonceLedger`:

```go
// guardianResignations[subject] is the list of revoked
// (guardian, setHash, effectiveAt) entries. Consulted during
// threshold calculation to zero out resigned guardians'
// weights. Retained after effect so a later query can answer
// "was X a guardian at time T?" for audit.
guardianResignations map[string][]GuardianResignation

// guardianResignationNonces[guardian][subject] is the highest
// ResignationNonce seen, for replay protection.
guardianResignationNonces map[string]map[string]int64
```

No changes to `GuardianSet` itself — resignations are tracked
as a parallel overlay. This preserves the "set as installed"
view for audit while giving the threshold-computation path
access to the effective set.

### 5.3 Effective guardian set

A new accessor computes the effective set at query time:

```go
// EffectiveGuardianSet returns the subject's set with
// resigned guardians' weights zeroed. If at least one
// resignation has taken effect, the returned set's
// TotalWeight is below the original, and if that drops
// below Threshold the set is "weakened" (still usable but
// flagged via metric).
func (l *NonceLedger) EffectiveGuardianSet(subject string, now time.Time) *GuardianSet
```

All downstream threshold checks (`ValidateGuardianRecoveryInit`,
`ValidateGuardianRecoveryVeto`) use this accessor rather than
the raw `GuardianSetOf`.

## 6. Validation rules

`ValidateGuardianResignation` checks in order:

1. **Kind** is `AnchorGuardianResign`.
2. **Non-empty fields**: `GuardianQuid`, `SubjectQuid`,
   `GuardianSetHash`, `Signature` must all be present.
3. **Subject has a set**: `guardianSets[SubjectQuid]` exists.
   Resigning from a non-existent set is a semantic error.
4. **Guardian is a member**: the resigning guardian appears
   in the current set. Resignations from non-members are
   rejected (no-op).
5. **Set hash matches**: `sha256(canonicalize(currentSet))` ==
   `GuardianSetHash`. If the subject has updated the set since
   the resignation was signed, the resignation is stale and
   rejected with a specific error so the guardian can re-sign
   against the new set.
6. **Nonce monotonicity**: `ResignationNonce >
   guardianResignationNonces[GuardianQuid][SubjectQuid]`.
7. **Effective-at in window**:
   `now - 5min <= EffectiveAt <= now + 365 days`.
8. **Signature valid**: verifies against
   `signerKeys[GuardianQuid][currentEpoch(GuardianQuid)]`.

### 6.1 Rejection paths (explicit list)

| Condition                                           | Error                                  |
|-----------------------------------------------------|----------------------------------------|
| Unknown subject                                     | `ErrResignationSubjectUnknown`          |
| Guardian not in current set                         | `ErrResignationNotMember`               |
| Set hash doesn't match installed set                | `ErrResignationSetHashMismatch`         |
| Nonce ≤ previously stored for this pair             | `ErrResignationReplay`                  |
| EffectiveAt < now − 5min                            | `ErrResignationEffectiveAtPast`         |
| EffectiveAt > now + 365d                            | `ErrResignationEffectiveAtTooFar`       |
| Signature invalid                                   | `ErrResignationBadSignature`            |
| Guardian has no current public key in ledger        | `ErrResignationNoGuardianKey`           |

### 6.2 Duplicate resignation behavior

Resignation with identical `(GuardianQuid, SubjectQuid,
GuardianSetHash, ResignationNonce)` to a previously-accepted
one: rejected as replay (rule 6). The HTTP layer turns this
into a 200 OK with `{duplicate: true}` for idempotent retries.

Resignation with the same `(guardian, subject, setHash)` but
a strictly-higher nonce: accepted and appended. The overlay
list can contain multiple entries for the same pair across
different set versions; the `EffectiveGuardianSet` accessor
considers only entries whose `GuardianSetHash` matches the
current set.

## 7. Mid-flight recovery semantics

The most delicate case is "a guardian resigns while a recovery
is pending." Per G2 the resignation is prospective only:

- If `PendingRecovery.InitBlockHeight < block containing the
  resignation`, the recovery's authorization was computed
  against the set as it was when Init was accepted. The
  resignation does NOT retroactively invalidate Init.
- If `PendingRecovery.InitBlockHeight >= block containing the
  resignation`, the Init was not yet in flight when the
  resignation took effect; the Init is validated against the
  effective set (resignations applied).
- `Commit` validation does not re-check guardian signatures —
  authorization was the Init itself. So a Commit for an
  in-flight recovery always succeeds on the delay elapsing,
  regardless of later resignations.
- `Veto` validation uses the effective set at veto time. A
  resigned guardian cannot use their signature to veto.

This asymmetry is deliberate: we want recovery to complete
(forward progress) but allow a distressed guardian to stop
FUTURE recoveries.

## 8. HTTP surface

New endpoint:

```
POST /api/v2/guardian/resign
```

Body: `GuardianResignation` (pre-signed).

Responses:
- `202 Accepted` — accepted into the pending-transaction pool
  for inclusion in a block.
- `200 OK` with `{duplicate: true}` — already accepted.
- `400 Bad Request` — validation failed; body carries the
  specific error code.
- `503 Service Unavailable` — nonce ledger not initialized.

## 9. Migration

Additive, no hard fork:

1. **Phase 0 (v2.4.0-alpha).** Code lands with the new anchor
   kind + validation path behind no feature flag. Nodes that
   don't recognize the anchor kind reject the transaction as
   invalid; older nodes in a mixed network will simply not
   process any `GuardianResignation` transactions.
2. **Phase 1 (v2.4.0).** Default on. Operators who want to
   block resignations at their node can submit
   validation-time overrides (not currently supported; a
   future config hook if needed).

Because the effect is tracked as an overlay and the
`EffectiveGuardianSet` accessor is what downstream validation
calls, nodes that don't apply the overlay still have correct
`GuardianSet` state — they just don't honor resignations.
During the mixed-version window, recovery at a non-upgraded
node may accept a signature from a resigned guardian; this is
indistinguishable from "guardian was compromised and re-signed"
from the non-upgraded node's perspective and does not fork
consensus.

## 10. Test plan

### 10.1 Unit tests

- **HappyPath** — resignation reduces effective threshold weight.
- **SubjectUnknown** → `ErrResignationSubjectUnknown`.
- **NotMember** → `ErrResignationNotMember`.
- **SetHashMismatch** — subject updates set after resignation
  signed → rejected.
- **NonceReplay** — identical resignation replayed → rejected.
- **EffectiveAtFuture** — resignation with future `EffectiveAt`
  stored but effective set unchanged until time advances.
- **BadSignature** — tampered signature → rejected.

### 10.2 Mid-flight tests

- **ResignationAfterInit** — Init accepted, guardian resigns,
  Commit succeeds (Init's authorization preserved).
- **ResignationBeforeInit** — resignation effective, Init
  must come in with remaining-threshold — if the weakened set
  can't reach threshold, Init is rejected.
- **ResignationVeto** — a resigned guardian tries to veto a
  pending recovery → rejected as not-a-member-at-veto-time.

### 10.3 Integration test

- **MultiResignationWeakening** — 3-of-5 set, two guardians
  resign, `guardian_set_weakened_total` metric increments,
  recovery still succeeds with the remaining 3.

### 10.4 HTTP test

- **Endpoint returns 202 on happy path, 200 on replay, 400
  on validation failure.**

## 11. Metrics

```
quidnug_guardian_resignations_total{subject}
quidnug_guardian_set_weakened_total{subject}
quidnug_guardian_resignations_rejected_total{reason}
```

The `weakened` metric fires once per observation window when a
set's effective weight drops below its threshold. Operator
dashboards alert on this.

## 12. Alternatives considered

### 12.1 Threshold auto-reduction (rejected)

When a guardian resigns, auto-reduce the subject's threshold
so the remaining guardians can still satisfy it. **Rejected**
because threshold is the security parameter the subject chose
deliberately; reducing it without the subject's consent is a
confused-deputy pattern. If three of five guardians resign and
the threshold was four, the set should be weakened — forcing
the subject to reshape rather than silently making the set
weaker.

### 12.2 Resignation with delay (rejected for MVP)

Require a delay (like recovery delay) between a resignation
being submitted and taking effect. **Rejected** because the
primary use case is "guardian's key compromised, guardian
wants out NOW." A delay helps attackers more than it helps
honest guardians.

### 12.3 Resignation revocation (rejected)

Allow a guardian to un-resign. **Rejected** because the state
machine is simpler with one-way transitions, and a guardian
who wants to re-join should be explicitly re-added by the
subject via `GuardianSetUpdate` (with all the consent checks
that implies).

## 13. Open questions

1. **Should we notify the subject?** A subject whose set has
   been weakened should want to know. Current design relies on
   the subject observing their own node's metrics. A push
   notification (via existing gossip) is tempting but would
   add wire surface; defer to ops runbook for now.
2. **Interaction with `RequireGuardianRotation`.** Per QDP-0004
   §7, a subject with the flag on who drops below threshold:
   is rotation still forbidden? Proposal: yes. The flag is the
   subject's deliberate stance; the weakened set just makes
   recovery more coordination-intensive, not impossible. Any
   change would require the subject to first set
   `RequireGuardianRotation = false` via a GuardianSetUpdate.
3. **Guardian can resign from ALL subjects at once?** Not in
   this QDP — each resignation is per-(guardian, subject).
   Bulk resignation is operator scripting, not protocol.

## 14. References

- [QDP-0002: Guardian-Based Recovery](0002-guardian-based-recovery.md)
- [QDP-0004: Phase H Roadmap](0004-phase-h-roadmap.md) §3.6
- [`internal/core/guardian.go`](../../internal/core/guardian.go)

---

**Review status.** Draft. Mid-flight semantics (§7) is the
principal sign-off requirement.
