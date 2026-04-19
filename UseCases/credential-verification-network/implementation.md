# Implementation: Credential Verification Network

## 1. Accreditor + issuer setup

```bash
# Accreditor (top-level trust root)
curl -X POST $NODE/api/identities -d '{
  "quidId":"sacscoc",
  "name":"Southern Association of Colleges and Schools",
  "homeDomain":"credentials.education.accreditation",
  "creator":"sacscoc","updateNonce":1
}'

# Accreditor's guardian set (board of directors quorum)
curl -X POST $NODE/api/v2/guardian/set-update -d '{ /* ... */ }'

# University quid
curl -X POST $NODE/api/identities -d '{
  "quidId":"university-of-texas-austin",
  "name":"University of Texas at Austin",
  "homeDomain":"credentials.education.undergraduate",
  "creator":"university-of-texas-austin","updateNonce":1
}'

# Accreditor endorses the university
curl -X POST $NODE/api/trust -d '{
  "truster":"sacscoc",
  "trustee":"university-of-texas-austin",
  "trustLevel":0.95,
  "domain":"credentials.education",
  "nonce":1,
  "validUntil":<now + 10y>,  /* accreditation typically 10 years */
  "description":"Regional accreditation valid through 2036"
}'
```

## 2. Issue a credential

```bash
# University issues a degree to a student
curl -X POST $NODE/api/v1/titles -d '{
  "type":"TITLE",
  "assetId":"degree-uoftexas-alice-2023-cs-bs",
  "domain":"credentials.education.undergraduate",
  "titleType":"academic-degree",
  "owners":[{"ownerId":"student-alice-chen","percentage":100.0}],
  "attributes":{
    "issuer":"university-of-texas-austin",
    "degreeType":"Bachelor of Science",
    "field":"Computer Science",
    "specialization":"Cybersecurity",
    "graduationDate":"2023-05-15",
    "gpa":"3.84",
    "transcriptHash":"<sha256>",
    "honorDesignation":"summa-cum-laude"
  },
  "creator":"university-of-texas-austin",
  "signatures":{
    "university-of-texas-austin":"<sig>",
    "registrar-ut-austin":"<sig>"
  }
}'
```

## 3. Professional license example

```bash
# State medical board issues license
curl -X POST $NODE/api/v1/titles -d '{
  "assetId":"license-medical-texas-dr-jones-12345",
  "domain":"credentials.licensing.medicine.texas",
  "titleType":"medical-license",
  "owners":[{"ownerId":"dr-jones-cardiology","percentage":100.0}],
  "attributes":{
    "issuer":"texas-medical-board",
    "licenseNumber":"12345",
    "issuedDate":"2015-08-20",
    "renewalDate":"2025-08-20",
    "specialty":"Cardiology",
    "status":"active"
  },
  "signatures":{"texas-medical-board":"<sig>"}
}'
```

## 4. Reciprocity edges

```bash
# Texas board recognizes California licenses
curl -X POST $NODE/api/trust -d '{
  "truster":"texas-medical-board",
  "trustee":"california-medical-board",
  "trustLevel":0.9,
  "domain":"credentials.licensing.medicine",
  "description":"Reciprocity: TX Admin Code 163.3(b)"
}'

# Similarly reverse
curl -X POST $NODE/api/trust -d '{
  "truster":"california-medical-board",
  "trustee":"texas-medical-board",
  "trustLevel":0.9,
  "domain":"credentials.licensing.medicine",
  "description":"Mutual recognition"
}'
```

A California hospital verifying a Texas-licensed doctor gets
trust: ca-board → tx-board → dr-jones = 0.9 × (direct trust of
board in the doctor, typically 0.95 if active) = 0.855.
Above threshold → accepted.

## 5. Employer verification

```go
type CredentialVerifier struct {
    quid   string
    client QuidnugClient
}

func (v *CredentialVerifier) Verify(ctx context.Context, credentialID string, expectedHolder string) (*VerificationResult, error) {
    title, err := v.client.GetTitle(ctx, credentialID)
    if err != nil {
        return nil, err
    }

    // Check holder matches
    hasHolder := false
    for _, owner := range title.Owners {
        if owner.OwnerID == expectedHolder {
            hasHolder = true
            break
        }
    }
    if !hasHolder {
        return &VerificationResult{Valid: false, Reason: "Credential not issued to this holder"}, nil
    }

    // Check issuer trust
    issuer := title.Attributes["issuer"].(string)
    issuerTrust, err := v.client.GetTrust(ctx, v.quid, issuer,
        title.Domain, &GetTrustOptions{MaxDepth: 3})
    if err != nil || issuerTrust.TrustLevel < 0.5 {
        return &VerificationResult{
            Valid:  false,
            Reason: fmt.Sprintf("Issuer trust %.2f below threshold", issuerTrust.TrustLevel),
        }, nil
    }

    // Check for revocation
    events, _ := v.client.GetSubjectEvents(ctx, credentialID, "TITLE")
    for _, ev := range events {
        if ev.EventType == "credential.revoked" {
            revoker := ev.Payload["revoker"].(string)
            if revoker == issuer {
                return &VerificationResult{
                    Valid:  false,
                    Reason: fmt.Sprintf("Revoked by %s: %s",
                        revoker, ev.Payload["reason"]),
                }, nil
            }
        }
    }

    // Check renewal date
    if renewal, ok := title.Attributes["renewalDate"]; ok {
        if parseDate(renewal.(string)).Before(time.Now()) {
            return &VerificationResult{Valid: false, Reason: "Expired"}, nil
        }
    }

    // Log verification
    v.emitVerificationEvent(ctx, credentialID)

    return &VerificationResult{
        Valid:       true,
        IssuerTrust: issuerTrust.TrustLevel,
        TrustPath:   issuerTrust.TrustPath,
    }, nil
}
```

## 6. Revocation

```bash
curl -X POST $NODE/api/v1/events -d '{
  "subjectId":"license-medical-texas-dr-smith-99999",
  "subjectType":"TITLE",
  "eventType":"credential.revoked",
  "payload":{
    "revoker":"texas-medical-board",
    "reason":"malpractice-conviction",
    "effectiveAt":1713400000,
    "caseReference":"TMB-2026-0123"
  },
  "creator":"texas-medical-board","signature":"<sig>"
}'
```

## 7. Testing

```go
func TestCredential_BasicVerification(t *testing.T) {
    // Accreditor → University → Student chain
    // Verify returns valid with trust path
}

func TestCredential_RevocationPropagates(t *testing.T) {
    // License issued
    // Revocation event
    // Subsequent verification returns invalid
}

func TestCredential_CrossJurisdictionReciprocity(t *testing.T) {
    // TX board recognizes CA board
    // TX hospital verifies CA-licensed doctor
    // Trust path: TX employer → TX board → CA board → doctor
    // Verification succeeds above threshold
}

func TestCredential_IssuerKeyRotation(t *testing.T) {
    // University rotates key via guardian recovery
    // Historical degrees still verify (old epoch preserved)
    // New degrees signed with new epoch key
}
```

## Where to go next

- [`threat-model.md`](threat-model.md)
