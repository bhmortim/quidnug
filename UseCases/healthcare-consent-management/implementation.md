# Implementation: Healthcare Consent Management

## 1. Patient onboarding

```bash
# Patient creates quid
curl -X POST $NODE/api/identities -d '{
  "quidId":"patient-alice-123",
  "name":"Alice (pseudonymous ID)",
  "homeDomain":"healthcare.consent.us",
  "creator":"patient-alice-123","updateNonce":1
}'

# Patient installs guardian set for emergency override
curl -X POST $NODE/api/v2/guardian/set-update -d '{
  "subjectQuid":"patient-alice-123",
  "newSet":{
    "guardians":[
      {"quid":"spouse-carol","weight":1,"epoch":0},
      {"quid":"adult-child-bob","weight":1,"epoch":0},
      {"quid":"primary-care-dr-smith","weight":1,"epoch":0},
      {"quid":"healthcare-proxy-legal-doc","weight":2,"epoch":0}
    ],
    "threshold":2,
    "recoveryDelay":900000000000,    /* 15 min */
    "requireGuardianRotation":false
  },
  "anchorNonce":1,"validFrom":<now>,
  "primarySignature":{"keyEpoch":0,"signature":"<patient sig>"},
  "newGuardianConsents":[
    {"guardianQuid":"spouse-carol","keyEpoch":0,"signature":"<sig>"},
    {"guardianQuid":"adult-child-bob","keyEpoch":0,"signature":"<sig>"},
    {"guardianQuid":"primary-care-dr-smith","keyEpoch":0,"signature":"<sig>"},
    {"guardianQuid":"healthcare-proxy-legal-doc","keyEpoch":0,"signature":"<sig>"}
  ]
}'
```

## 2. Grant consent to a provider

```bash
# Patient grants Dr. Jones cardiology full-record access for 90 days
curl -X POST $NODE/api/trust -d '{
  "truster":"patient-alice-123",
  "trustee":"dr-jones-cardiology",
  "trustLevel":0.9,
  "domain":"healthcare.records.access",
  "nonce":47,
  "validUntil":<now + 90d>,
  "description":"Cardiac consultation series Q2 2026"
}'

# Sub-domain granular consent for pharmacy (Rx only)
curl -X POST $NODE/api/trust -d '{
  "truster":"patient-alice-123",
  "trustee":"cvs-pharmacy-lincoln-park",
  "trustLevel":0.9,
  "domain":"healthcare.records.access.prescriptions",
  "nonce":48,
  "validUntil":<now + 365d>
}'
```

## 3. Provider access check

Before accessing records, provider's system queries:

```go
func (p *Provider) CanAccess(ctx context.Context, patientID string, recordType string) (bool, error) {
    domain := "healthcare.records.access"
    if recordType != "" {
        domain = fmt.Sprintf("healthcare.records.access.%s", recordType)
    }

    trust, err := p.client.GetTrust(ctx, patientID, p.quid, domain, &GetTrustOptions{
        MaxDepth: 3,  // Allow referral chain up to 3 hops
    })
    if err != nil {
        return false, err
    }

    if trust.TrustLevel < 0.5 {
        return false, nil  // Insufficient trust
    }

    // Check validity — GetTrust returns active edges only, but
    // verify the most recent edge hasn't expired.
    return trust.TrustLevel > 0, nil
}

func (p *Provider) LogAccess(ctx context.Context, patientID string, access AccessDetails) error {
    event := map[string]interface{}{
        "subjectId":   patientID,
        "subjectType": "QUID",
        "eventType":   "record.accessed",
        "payload": map[string]interface{}{
            "accessor":     p.quid,
            "consentTxId":  access.ConsentRef,
            "accessType":   access.RecordType,
            "accessedAt":   time.Now().Unix(),
            "purpose":      access.Purpose,
        },
        "creator":   p.quid,
        "signature": p.sign(/* canonical */),
    }
    return p.submitEvent(ctx, event)
}
```

## 4. Emergency override (unconscious patient)

```bash
# ER submits emergency guardian recovery
curl -X POST $NODE/api/v2/guardian/recovery/init -d '{
  "kind":"guardian_recovery_init",
  "subjectQuid":"patient-alice-123",
  "fromEpoch":0,
  "toEpoch":1,
  "newPublicKey":"<ephemeral ER key>",
  "minNextNonce":1,
  "maxAcceptedOldNonce":100,   /* don'\''t invalidate prior consent */
  "anchorNonce":<next>,
  "validFrom":<now>,
  "guardianSigs":[
    {"guardianQuid":"primary-care-dr-smith","keyEpoch":0,"signature":"<sig>"},
    {"guardianQuid":"healthcare-proxy-legal-doc","keyEpoch":0,"signature":"<sig>"}
  ]
}'
```

With guardian threshold 2 met, recovery is pending. 15-minute
delay. If no veto, auto-commit by any party or explicit
commit by ER:

```bash
curl -X POST $NODE/api/v2/guardian/recovery/commit -d '{
  "kind":"guardian_recovery_commit",
  "subjectQuid":"patient-alice-123",
  "recoveryAnchorHash":"<hash>",
  "anchorNonce":<next>,
  "validFrom":<now>,
  "committerQuid":"er-system-dr-cooper",
  "committerSig":"<sig>"
}'
```

After commit, an emergency trust edge is auto-created for the
ER with a 24-hour window:

```bash
curl -X POST $NODE/api/trust -d '{
  "truster":"patient-alice-123",  /* now signed by new emergency key */
  "trustee":"hospital-er-central",
  "trustLevel":0.9,
  "domain":"healthcare.records.access",
  "nonce":99,
  "validUntil":<now + 24h>,
  "description":"Emergency access via guardian quorum; unconscious patient"
}'
```

## 5. Patient reviews emergency access afterward

After recovery, patient queries their stream:

```bash
curl "$NODE/api/v1/events/QUID/patient-alice-123" | jq '.data[] | select(.eventType == "record.accessed")'
```

Shows exactly who accessed records during their unconscious
period. Patient sees the guardian-recovery event, understands
it as emergency override, and either retroactively approves
or files a dispute.

## 6. Revoke consent

```bash
# Update trust edge to 0
curl -X POST $NODE/api/trust -d '{
  "truster":"patient-alice-123",
  "trustee":"dr-jones-cardiology",
  "trustLevel":0.0,
  "domain":"healthcare.records.access",
  "nonce":50,
  "description":"Discontinued care"
}'
```

Push gossip within seconds; all providers querying for consent
see trust=0 and deny.

## 7. Referral (transitive access)

```
Patient ──0.9──► Dr. Smith (primary care)
Dr. Smith ──0.85──► Dr. Jones (cardiology specialist, referred)
Dr. Jones ──0.8──► Lab-Corp (ordering labs)
```

Lab-Corp queries Quidnug for trust from patient: traverses
Patient → Smith → Jones → Lab-Corp.  Computed trust: 0.9 × 0.85
× 0.8 = 0.612.  Above 0.5 threshold → access granted.

Patient preconfigures their app: "auto-accept through primary
care up to 3 hops with trust ≥ 0.5". The app stays out of
the clinical flow except for exceptions.

## 8. Testing

```go
func TestHealthcare_DirectConsent(t *testing.T) {
    // Patient grants Dr. Jones
    // Dr. Jones can access; Dr. Lee (no consent) cannot
}

func TestHealthcare_ConsentExpiresAfterValidUntil(t *testing.T) {
    // Consent issued for 1 hour
    // After expiry, provider access denied
}

func TestHealthcare_EmergencyOverride(t *testing.T) {
    // 2-of-4 guardians initiate
    // 15-min delay elapses with no veto
    // ER system gets emergency trust edge
    // Access logged in patient's event stream
}

func TestHealthcare_PatientVetoesEmergencyOverride(t *testing.T) {
    // Guardian recovery initiated
    // Patient (or another guardian) vetoes before time-lock
    // No ER access granted
}

func TestHealthcare_RevocationPropagates(t *testing.T) {
    // Set trust; gossip to 3-node network
    // Revoke; verify all nodes see revocation
    // Provider access denied across network
}
```

## Where to go next

- [`threat-model.md`](threat-model.md)
- [`../credential-verification-network/`](../credential-verification-network/)
