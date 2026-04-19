# Threat Model: Healthcare Consent Management

## Assets

1. **Patient medical data** — sensitive and regulated.
2. **Consent integrity** — the cryptographic record of who
   was granted what.
3. **Access audit trail** — tamper-evident log.
4. **Patient's signing key** — represents the patient;
   compromise enables unauthorized consent grants.

## Attackers

| Attacker                 | Capability                                  | Goal                            |
|--------------------------|---------------------------------------------|---------------------------------|
| Curious insider          | Provider employee with valid access         | Lookup records unauthorized     |
| Compromised provider     | Valid provider signing key                  | Access records without consent  |
| Malicious guardian       | Patient's named guardian, bad intent        | Force emergency access          |
| Patient compromise       | Patient's key stolen                        | Grant unauthorized access       |
| External                 | No credentials                             | Exploration / theft             |

## Threats

### T1. Unauthorized provider access

**Attack.** Provider tries to access records without patient
consent.

**Mitigation.**
- Provider's system checks trust before access; trust=0 or
  expired → deny.
- Every access attempt generates a signed event. False claims
  that "patient consented" are contradicted by the chain.

**Residual risk.** A provider with valid consent for record
type A accessing record type B (sub-domain scoping helps).

### T2. Guardian collusion for emergency access

**Attack.** Spouse + adult child collude to gain access to
records the patient doesn't want them to see.

**Mitigation.**
- 15-minute veto window. Patient or another guardian can
  intercede.
- Event stream records the access; patient sees it after.
- Guardian structure is patient-chosen; they can revoke
  problem guardians via `GuardianResignation` (QDP-0006).

**Residual risk.** If patient is truly incapacitated and
two guardians collude, access happens. Mitigation is choosing
guardians carefully and reviewing them periodically.

### T3. Compromised patient key

**Attack.** Patient's signing key stolen; attacker grants
themselves access.

**Mitigation.**
- Guardian recovery lets patient (via other channels) rotate.
- Providers observing recent rotations re-evaluate trust
  edges issued by the old-epoch key.
- Anchor nonces prevent replay.

### T4. Revocation delay exploit

**Attack.** Patient revokes consent; attacker accesses
records before revocation propagates.

**Mitigation.**
- **Push gossip (QDP-0005)** — revocation reaches all
  providers within seconds. Small window, but non-zero.
- Provider's system should query Quidnug at time-of-access,
  not rely on cached trust.

**Residual risk.** Very narrow window (seconds).

### T5. Emergency override abuse

**Attack.** Provider fakes emergency to trigger override.

**Mitigation.**
- 2 guardian signatures required. Provider alone can't
  initiate.
- Guardians who falsely sign emergency claims are
  accountable — their signatures are on-chain.
- Audit by regulators trivially surfaces "who requested
  emergency access in circumstances that weren't actually
  emergency".

### T6. HIPAA / regulatory compliance

**Concern.** Is Quidnug's event log HIPAA-compliant?

**Response.**
- PHI is NOT on Quidnug directly. Only consent and access
  events.
- Consent and access logs ARE PHI under some interpretations
  (they reveal care patterns).
- Deployment best practice: Quidnug events stored on
  consortium chain; patient-linked metadata encrypted.
- This is a regulatory implementation concern, not a
  protocol gap.

### T7. De-anonymization

**Attack.** Attacker correlates consent events to identify
patients.

**Mitigation.**
- Use pseudonymous quids. Patient's real identity is kept
  off-chain.
- Domain scoping + restrictive event visibility.
- Real-world identity resolution requires the patient or
  their provider's cooperation.

### T8. Replay of consent grant

**Attack.** Attacker replays a valid old consent grant after
patient has revoked.

**Mitigation.**
- Anchor nonces on trust transactions. Revocation is a
  higher-nonce transaction; replay of the old grant has
  a stale nonce and is rejected.

## Not defended against

1. **Physical record theft.** Paper records, USB drives with
   exports. Not a Quidnug concern.
2. **Provider's internal access controls.** Quidnug says
   "yes you can access". Which employee at the provider
   actually reads the file is the provider's access-control
   problem.
3. **Consent-under-duress.** Patient signs consent under
   pressure. The protocol records consent validly; the
   legality is a real-world matter.

## References

- [QDP-0002 Guardian Recovery](../../docs/design/0002-guardian-based-recovery.md)
- [QDP-0005 Push Gossip](../../docs/design/0005-push-based-gossip.md)
- [QDP-0006 Guardian Resignation](../../docs/design/0006-guardian-resignation.md)
