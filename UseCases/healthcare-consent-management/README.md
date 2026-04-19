# Healthcare Consent Management

**Cross-industry · Patient-controlled access · Emergency override**

## The problem

A patient's medical records span 10+ systems: primary care,
specialists, hospitals, labs, insurers, pharmacies, mental
health. Every access requires consent, but the current reality:

- **Paper consents** at each provider office. Unscannable,
  uncorrelated.
- **Platform-locked portals** (Epic MyChart, Cerner HealtheLife,
  etc.) — each siloed.
- **HIPAA retroactively punishes** wrongful access but doesn't
  prevent it at time-of-access. Breaches happen regularly.
- **Emergency access** is ad-hoc: an ER doctor needing records
  for an unconscious patient either bypasses controls ("glass
  break" override) or waits hours for phone-based consent.
- **Revocation is fictional.** A patient "revokes" consent with
  a practice; the practice keeps local copies; other providers
  don't learn of the revocation.
- **Referral chains** are opaque. Primary → specialist → lab is
  a chain of trust the patient implicitly grants but can't
  easily model.

The right model: **the patient is the principal; access is
time-bounded, explicit consent from the patient; emergency
override exists via a patient-selected guardian quorum; every
access is auditable on a tamper-evident log**.

## Why Quidnug fits

Consent management is a trust graph: patient trusts doctor A
for a specific purpose for a specific period. Guardians can
override for emergencies with time-lock. Revocation propagates.

| Problem                                     | Quidnug primitive                             |
|---------------------------------------------|-----------------------------------------------|
| "Patient X consents to Dr. Y accessing..."  | Signed TRUST edge with `validUntil`           |
| "Emergency access when patient unconscious" | GuardianRecovery with short delay              |
| "Revoke access across all providers"        | Trust-edge update, push-gossiped               |
| "Audit trail of who saw what"               | Event stream per access                        |
| "Specialist referral trust"                 | Transitive trust through referring doctor      |
| "Patient's signing key lost"                | Guardian recovery (spouse, chosen POA)         |

## High-level architecture

```
                     Patient ("patient-alice-123")
                  as quid with Guardian Set:
              {spouse, adult-child, primary-care-doc,
               chosen-healthcare-proxy}
              threshold: 2  (2 of 4 for emergency access)
              requireGuardianRotation: false  (allow normal
                updates by patient)
                               │
                               │
       ┌───────────────────────┼──────────────────────┐
       │                       │                      │
       ▼                       ▼                      ▼
 trust edges for         trust edges for        Event stream:
 specific providers      domain inheritance     [consent granted,
                                                 record accessed,
                                                 consent revoked,
                                                 emergency override]
```

## Data model

### Patient as quid
```json
{
  "quidId":"patient-alice-123",
  "homeDomain":"healthcare.consent.us",
  "attributes":{
    "patientID":"<internal hash>",
    "dateOfBirth":"HASH:<sha256 of DOB+nonce>",
    "healthcareProxy":"adult-child-bob-456"
  }
}
```

Patient PII is kept minimal on-chain. Rich data stays in
provider systems; Quidnug tracks consent and access.

### Patient's guardian set
```json
{
  "guardians":[
    {"quid":"spouse-carol","weight":1},
    {"quid":"adult-child-bob","weight":1},
    {"quid":"primary-care-doc-dr-smith","weight":1},
    {"quid":"healthcare-proxy-advance-directive","weight":2}
  ],
  "threshold":2,
  "recoveryDelay":900000000000,    /* 15 minutes — short for emergencies */
  "requireGuardianRotation":false
}
```

Short recovery delay (15 min) balances emergency need with
basic veto window. Patient can increase (e.g., 24h) for
non-life-threatening scenarios.

### Consent as trust edge
```bash
# Patient grants Dr. Jones access to their records for 90 days
curl -X POST $NODE/api/trust -d '{
  "truster":"patient-alice-123",
  "trustee":"dr-jones-cardiology",
  "trustLevel":0.9,
  "domain":"healthcare.records.access",
  "nonce":47,
  "validUntil":<now + 90d>,
  "description":"Cardiac consultation; full record access"
}'
```

Provider's system queries Quidnug for consent before each
access. A query returns trust level + validity; provider
proceeds only if both are acceptable.

### Access as event
```json
{
  "type":"EVENT",
  "subjectId":"patient-alice-123",
  "subjectType":"QUID",
  "eventType":"record.accessed",
  "payload":{
    "accessor":"dr-jones-cardiology",
    "consentTxId":"<ID of trust transaction>",
    "accessType":"clinical-notes",
    "accessedAt":1713400000,
    "purpose":"follow-up appointment prep"
  },
  "signature":"<Dr. Jones's sig>"
}
```

Every access logged. Patient (or regulator) can query their
own stream to see who accessed what.

## Emergency override flow

Patient arrives unconscious at ER. ER doctor needs records.
No direct consent available.

```
1. ER doctor (dr-er-cooper) requests emergency access.

2. Dr. Cooper submits a guardian-recovery-init:
   subjectQuid: patient-alice-123
   fromEpoch: current
   toEpoch: current + 1  (new "emergency-access" epoch)
   newPublicKey: <ephemeral key for this emergency session>
   guardianSigs: [primary-care-doc signature + healthcare-proxy]
                 (2 signatures from guardians met threshold)

3. Recovery delay: 15 minutes. Patient (or spouse) could veto
   if this is a hostile takeover. In practice, patient is
   unconscious → no veto → delay elapses.

4. Commit: ER gets access with an "emergency" trust edge
   scoped to the hospital for 24 hours.

5. Event stream records the emergency access with full
   context. Patient sees it upon recovery.
```

For a genuine life-threatening situation requiring access in
seconds not minutes, providers have "break-glass" procedures
separately — Quidnug records the override event for post-hoc
audit, not gatekeeping time-critical care.

## Referral chain (transitive trust)

Primary care (Dr. Smith) refers patient to specialist
(Dr. Jones). Specialist refers to lab (Lab Corp).

```
Patient ──0.9──► Dr. Smith (primary care, longstanding)
Dr. Smith ──0.9──► Dr. Jones (trusted cardiologist)
Dr. Jones ──0.8──► Lab Corp (usual lab)

Patient's transitive trust in Lab Corp = 0.9 × 0.9 × 0.8 = 0.648
```

Patient's app can preconfigure: "accept transitive access
through my PCP up to 3 hops, max trust decay 0.6". The app
automatically grants access when the chain meets threshold.

## Consent revocation

Patient revokes access:

```bash
# Update trust edge to 0 (or expire it)
curl -X POST $NODE/api/trust -d '{
  "truster":"patient-alice-123",
  "trustee":"dr-jones-cardiology",
  "trustLevel":0.0,
  "domain":"healthcare.records.access",
  "nonce":48,
  "description":"Ending care; revoking access"
}'
```

Push gossip propagates revocation within seconds. Every
provider node that queries for consent sees 0 trust → denies
future access.

## Sub-domain granularity

Different record types → different access rules:

```
healthcare.records.access.full            (everything)
healthcare.records.access.prescriptions    (only Rx)
healthcare.records.access.imaging          (only imaging)
healthcare.records.access.mental-health    (highly restricted)
```

Patient can grant Dr. Jones `full` access but grant the
pharmacy only `prescriptions`.

## Key Quidnug features

- **Signed trust edges with `validUntil`** — consent as
  time-bounded grant.
- **Guardian recovery with short delay** — emergency override
  with audit trail.
- **Event streams** — every access logged.
- **Push gossip** — revocation propagates fast.
- **Domain hierarchy** — sub-scope consents.
- **Transitive trust** — referral chains modeled cleanly.
- **GuardianResignation (QDP-0006)** — guardian lifecycle
  (spouse divorce, proxy decline).

## Value delivered

| Dimension                        | Before                                  | With Quidnug                                     |
|----------------------------------|-----------------------------------------|--------------------------------------------------|
| Consent portability              | Per-platform, siloed                    | Single source of truth across all providers       |
| Time-bounded consent             | Rarely enforced                         | `validUntil` enforced cryptographically           |
| Revocation speed                 | Days to weeks                           | Seconds                                           |
| Emergency access                 | "Break glass" uncontrolled              | Guardian-recovery with 15-min time-lock + audit   |
| Audit trail                      | Per-provider, fragmented                | Single event stream per patient                   |
| Granularity                      | Whole-record                            | Sub-domain (prescriptions vs. imaging vs. etc.)   |
| Patient control                  | Limited by platform UI                  | Own quid + own guardian set                       |
| Compliance reporting             | Manual extraction                       | Deterministic chain replay                        |

## What's in this folder

- [`README.md`](README.md)
- [`implementation.md`](implementation.md) — Quidnug API calls
- [`threat-model.md`](threat-model.md) — security analysis

## Related

- [`../credential-verification-network/`](../credential-verification-network/)
  — for verifying provider credentials
- [QDP-0002 Guardian Recovery](../../docs/design/0002-guardian-based-recovery.md)
