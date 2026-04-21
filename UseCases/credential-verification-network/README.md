# Credential Verification Network

**Cross-industry · Education · Licensing · Certifications**

## The problem

Credentials — university degrees, professional licenses,
industry certifications — form the fabric of who can do what
in society. Yet their verification infrastructure is
embarrassing:

- **Employer calls registrar:** days of phone tag.
- **PDF verification:** a PDF diploma is trivially forged.
- **Third-party verifiers** (Parchment, NSC): each has
  coverage gaps and proprietary APIs.
- **Revocation barely works.** A doctor's license revoked
  in State A may not be visible to employers / patients in
  State B for months.
- **Cross-jurisdiction reciprocity** is manual. "Yes, a
  PharmD from India is equivalent to..." → bureaucratic
  process taking months.
- **Sub-attributes** are lost. "Bachelor's in CS with
  specialization in security from University X, GPA 3.8, in
  2023" becomes a binary did-they-or-didn't-they.

## Why Quidnug fits

Credentials are signed claims from identified issuers about
subjects. That's identity transactions + trust edges.
Revocation is an anchor invalidation. Cross-jurisdiction
trust is relational trust between credential issuers.

| Problem                                   | Quidnug primitive                             |
|-------------------------------------------|-----------------------------------------------|
| "Did University X issue this degree?"     | Signed IDENTITY/TITLE by University X         |
| "Is University X a real university?"      | Accreditor's trust edge in University X       |
| "Did State A revoke Dr. Y's license?"     | Invalidation anchor from State A              |
| "Is State A's license valid in State B?"  | Reciprocity trust edges                       |
| "Employer trusts foreign degrees"         | Relational trust across jurisdictions         |
| "Recovery if registrar's key is lost"     | Guardian set for the registrar                |

## High-level architecture

```
                 credentials.education (domain)
                           │
     ┌─────────────────────┼─────────────────────┐
     │                     │                     │
     ▼                     ▼                     ▼
Accreditor-US        Accreditor-EU         Accreditor-APAC
(SACSCOC, etc.)     (ENQA, etc.)         (regional orgs)
     │                     │                     │
     │ trust edges to     │ trust edges to      │ trust edges to
     │ accredited         │ accredited          │ accredited
     │ universities        │ universities         │ universities
     ▼                     ▼                     ▼
 Universities        Universities          Universities
     │                     │                     │
     │ issue credentials   │                     │
     ▼                     ▼                     ▼
 Students              Students              Students
  (credential holders' quids)
```

## Data model

### Quids
- **Accreditor** — SACSCOC, ABET, NASAA, etc. Top of trust hierarchy.
- **University / issuer** — accredited by one or more accreditors.
- **Student / professional** — credential holder.
- **State board** (for professional licenses) — medical boards,
  bar associations, CPA boards, etc.

### Domain
```
credentials.education
├── credentials.education.undergraduate
├── credentials.education.graduate
├── credentials.education.certifications

credentials.licensing.medicine.texas
credentials.licensing.medicine.california
credentials.licensing.bar.ny

credentials.certifications.aws
credentials.certifications.cncf
credentials.certifications.iso
```

### Credential as identity + title

```json
{
  "type":"TITLE",
  "assetId":"degree-uoftexas-austin-alice-2023-cs-bs",
  "domain":"credentials.education.undergraduate",
  "titleType":"academic-degree",
  "owners":[{"ownerId":"student-alice-chen","percentage":100.0}],
  "attributes":{
    "issuer":"university-of-texas-austin",
    "degreeType":"Bachelor of Science",
    "field":"Computer Science",
    "specialization":"Cybersecurity",
    "gpa":"3.84",
    "graduationDate":"2023-05-15",
    "classRank":"summa-cum-laude",
    "transcriptHash":"<sha256 of full transcript>",
    "registrarSignature":"<separate sig by registrar quid>"
  },
  "signatures":{
    "university-of-texas-austin":"<sig>",
    "registrar-ut-austin":"<sig>"
  }
}
```

### Professional license

```json
{
  "type":"TITLE",
  "assetId":"license-medical-texas-dr-jones-12345",
  "domain":"credentials.licensing.medicine.texas",
  "titleType":"medical-license",
  "owners":[{"ownerId":"dr-jones-cardiology","percentage":100.0}],
  "attributes":{
    "issuer":"texas-medical-board",
    "licenseNumber":"12345",
    "issuedDate":"2015-08-20",
    "renewalDate":"2024-08-20",
    "specialty":"Cardiology",
    "boardCertifications":["ABIM"],
    "status":"active"
  },
  "signatures":{"texas-medical-board":"<sig>"}
}
```

### Verification events

```
eventType: "credential.verified"
subjectId: "degree-uoftexas-austin-alice-2023-cs-bs"
payload:
  verifier: "employer-acme-corp"
  verifiedAt: 1713400000
  verificationPurpose: "pre-employment background check"
signer: employer-acme-corp

eventType: "credential.revoked"
subjectId: "license-medical-texas-dr-smith-99999"
payload:
  revoker: "texas-medical-board"
  reason: "malpractice-ruling"
  effectiveAt: 1713400000
  caseReference: "TMB-2026-0123"
signer: texas-medical-board
```

### Reciprocity / cross-jurisdiction

Trust edges model reciprocity:

```bash
# Texas recognizes California medical licenses
curl -X POST $NODE/api/trust -d '{
  "truster":"texas-medical-board",
  "trustee":"california-medical-board",
  "trustLevel":0.9,
  "domain":"credentials.licensing.medicine",
  "description":"Reciprocity agreement per TX Rule 163.3(b)"
}'

# Indirectly: employer in Texas trusts CA doctor's license
# at trust(tx-board → ca-board) × trust(ca-board → ca-doctor) = 0.81
```

## Employer verification flow

```go
func (employer *Employer) VerifyCredential(ctx context.Context, candidateID string, credentialID string) (*VerifyResult, error) {
    // Get the credential title
    title, err := employer.client.GetTitle(ctx, credentialID)
    if err != nil { return nil, err }

    // Check if holder matches
    if !hasOwner(title, candidateID) {
        return &VerifyResult{Valid: false, Reason: "Credential not issued to this candidate"}, nil
    }

    // Check issuer's trust
    issuer := title.Attributes["issuer"].(string)
    domain := title.Domain
    issuerTrust, _ := employer.client.GetTrust(ctx, employer.quid, issuer, domain, &GetTrustOptions{MaxDepth: 3})

    if issuerTrust.TrustLevel < 0.5 {
        return &VerifyResult{
            Valid:  false,
            Reason: fmt.Sprintf("Issuer trust too low: %.2f (need 0.5+)", issuerTrust.TrustLevel),
            TrustPath: issuerTrust.TrustPath,
        }, nil
    }

    // Check for revocation events
    events, _ := employer.client.GetSubjectEvents(ctx, credentialID, "TITLE")
    for _, ev := range events {
        if ev.EventType == "credential.revoked" {
            revoker := ev.Payload["revoker"].(string)
            // Only the original issuer (or their current authorized
            // successor) can revoke
            if revoker == issuer || canRevoke(revoker, issuer) {
                return &VerifyResult{
                    Valid:  false,
                    Reason: fmt.Sprintf("Credential revoked by %s: %s",
                        revoker, ev.Payload["reason"]),
                }, nil
            }
        }
    }

    // Check expiration
    if renewalDate, ok := title.Attributes["renewalDate"]; ok {
        if parseDate(renewalDate.(string)).Before(time.Now()) {
            return &VerifyResult{Valid: false, Reason: "Credential expired"}, nil
        }
    }

    // Log verification
    employer.LogVerification(ctx, credentialID)

    return &VerifyResult{
        Valid:       true,
        IssuerTrust: issuerTrust.TrustLevel,
        TrustPath:   issuerTrust.TrustPath,
    }, nil
}
```

## Revocation propagation

State medical board revokes Dr. Smith's license:

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

Push gossip propagates to all hospital / insurer / pharmacy
nodes within minutes. Every verification query from that
point forward returns "revoked."

## Issuer key rotation

University loses their registrar HSM. Guardian recovery:

```bash
curl -X POST $NODE/api/v2/guardian/recovery/init -d '{
  "subjectQuid":"university-of-texas-austin",
  "fromEpoch":0,
  "toEpoch":1,
  "newPublicKey":"<hex>",
  "guardianSigs":[
    {"guardianQuid":"ut-system-president","keyEpoch":0,"signature":"<sig>"},
    {"guardianQuid":"ut-provost","keyEpoch":0,"signature":"<sig>"},
    {"guardianQuid":"sacscoc-accreditor","keyEpoch":0,"signature":"<sig>"}
  ],
  ...
}'
```

Post-rotation, historical degrees (signed by old-epoch key)
remain valid — old epochs are preserved, not invalidated.
New degrees use the new key.

## Key Quidnug features

- **Title transactions** for credentials with structured
  attributes.
- **Event streams** for verification + revocation history.
- **Trust edges** for accreditation hierarchy + reciprocity.
- **Guardian recovery** for issuer key management.
- **Push gossip** for fast revocation propagation.
- **Domain hierarchy** for jurisdiction + credential type.
- **GuardianResignation (QDP-0006)** — a university losing
  accreditation can be handled via accreditor's revoking
  trust edges + issuer's self-rotation.

## Value delivered

| Dimension                    | Before                                 | With Quidnug                                           |
|------------------------------|----------------------------------------|--------------------------------------------------------|
| Employer verification        | Phone calls, days                      | API query, seconds                                     |
| Revocation propagation       | Months cross-jurisdiction              | Minutes                                                |
| Forgery resistance           | Paper/PDF trivial to fake              | Cryptographically signed; forgery = compromise         |
| Cross-jurisdiction credit    | Manual reciprocity paperwork           | Trust edges; transitive                                |
| Sub-attribute preservation   | Lost in PDF                             | Structured on-chain                                    |
| Audit of who verified        | None                                   | Signed event per verification                          |
| Employer customization       | Binary                                 | Relational trust; tune accreditor trust per-employer   |

## What's in this folder

- [`README.md`](README.md)
- [`implementation.md`](implementation.md)
- [`threat-model.md`](threat-model.md)

## Runnable POC

Full end-to-end demo at
[`examples/credential-verification-network/`](../../examples/credential-verification-network/):

- `credential_verify.py` — standalone verifier logic.
- `credential_verify_test.py` — 10 pytest cases covering
  direct + transitive trust, revocation, threshold tuning,
  cross-jurisdiction observer-relative verdicts.
- `demo.py` — seven-step end-to-end flow against a live
  Quidnug node: register actors, establish accreditation,
  issue degree, verify from US + APAC employer perspectives,
  revoke, re-verify, cross-jurisdiction credential.

```bash
cd examples/credential-verification-network
python demo.py
```

## Related

- [`../healthcare-consent-management/`](../healthcare-consent-management/)
  — verifying provider credentials
- [`../developer-artifact-signing/`](../developer-artifact-signing/)
  — issuer key recovery pattern
- [QDP-0002 Guardian Recovery](../../docs/design/0002-guardian-based-recovery.md)
