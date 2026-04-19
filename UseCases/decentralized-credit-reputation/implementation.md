# Implementation: Decentralized Credit & Reputation

Concrete Quidnug API calls and code snippets for each role.

## 0. Node configuration

### Lender node

```bash
ENABLE_NONCE_LEDGER=true
ENABLE_PUSH_GOSSIP=true
ENABLE_LAZY_EPOCH_PROBE=true
ENABLE_KOFK_BOOTSTRAP=true
SUPPORTED_DOMAINS=credit.*
NODE_AUTH_SECRET=<32-byte hex>
IPFS_ENABLED=true
IPFS_GATEWAY_URL=http://localhost:5001
```

### Subject wallet

The subject runs a thin client (mobile app) that talks to any
public read-only node + their own local IPFS client or a
trusted IPFS pinning service.

## 1. Subject onboarding

### 1a. Generate subject quid

```bash
# On the subject's device (phone app, CLI, hardware wallet)
$ quidnug-credit init
Generated new subject identity.
Subject Quid ID: subject-alice-chen-xyz
Public key: 0x04a1b2c3...
Private key: saved to local secure enclave

Encryption key (derived from subject key, same curve):
  - ECIES pubkey: 0x04a1b2c3...
  - ECIES privkey: used for decrypting access grants to you
```

### 1b. Link to identity verification

```bash
# After in-person / online KYC, the verifier emits trust edge
curl -X POST $VERIFIER_NODE/api/trust -d '{
  "type":"TRUST",
  "truster":"verifier-dmv-texas",
  "trustee":"subject-alice-chen-xyz",
  "trustLevel":1.0,
  "domain":"credit.identity-verification.us",
  "nonce":47283,
  "validUntil":<now + 5y>,
  "attributes":{
    "verificationType":"in-person-DMV",
    "verificationDate":"2026-04-18",
    "identityHash":"<sha256 of name+DOB+SSN+address>",
    "biometricVerified":true
  },
  "signature":"<DMV signs>"
}'
```

Subject can verify their verification:

```bash
curl "$ANY_NODE/api/v1/trust/edges?trustee=subject-alice-chen-xyz&domain=credit.identity-verification.us"
```

Multiple verifiers can endorse the same subject. Lenders accept
whichever ones they trust.

### 1c. Link alternative data sources

Subject authorizes their utility company to publish attestations:

```bash
# Subject signs consent event
curl -X POST $NODE/api/v1/events -d '{
  "type":"EVENT",
  "subjectId":"subject-alice-chen-xyz",
  "subjectType":"QUID",
  "eventType":"credit.alt-data.consent-granted",
  "payload":{
    "dataSource":"utility-con-edison-nyc",
    "scope":["monthly-payment-history"],
    "duration":"indefinite-until-revoked",
    "optInHash":"<sha256 of consent document>"
  },
  "creator":"subject-alice-chen-xyz",
  "signature":"<subject signs>"
}'

# Utility co-signs to acknowledge
curl -X POST $UTILITY_NODE/api/v1/events -d '{
  "type":"EVENT",
  "subjectId":"subject-alice-chen-xyz",
  "subjectType":"QUID",
  "eventType":"credit.alt-data.publisher-acknowledged",
  "payload":{
    "consentEventRef":"<the consent event id>",
    "dataSource":"utility-con-edison-nyc",
    "customerVerified":true
  },
  "creator":"utility-con-edison-nyc",
  "signature":"<utility signs>"
}'
```

Monthly, utility publishes attestation:

```bash
curl -X POST $UTILITY_NODE/api/v1/events -d '{
  "type":"EVENT",
  "subjectId":"subject-alice-chen-xyz",
  "subjectType":"QUID",
  "eventType":"credit.alt-data.payment-record",
  "payload":{
    "month":"2026-04",
    "onTime":true,
    "amountBand":"100-150",
    "detailCID":"bafy...",
    "detailHash":"<sha256>"
  },
  "creator":"utility-con-edison-nyc",
  "signature":"<utility signs>"
}'
```

After 12 months of consecutive on-time events, the utility may
issue a trust edge:

```bash
curl -X POST $UTILITY_NODE/api/trust -d '{
  "truster":"utility-con-edison-nyc",
  "trustee":"subject-alice-chen-xyz",
  "trustLevel":0.88,
  "domain":"credit.alternative-data.utilities",
  "description":"36 months on-time payment",
  "validUntil":<now + 2y>
}'
```

## 2. Lender origination flow

### 2a. Subject applies at lender

Off-chain: subject enters lender's website/app with their
Subject Quid ID. Lender asks for access to history.

### 2b. Subject grants access

```go
package subject

import (
    "crypto/aes"
    "crypto/cipher"
    "crypto/ecdsa"
    "crypto/rand"
)

// Subject generates a symmetric key for this access grant
func (s *Subject) GenerateAccessGrant(grantee string, scopes []string, duration time.Duration) (*AccessGrant, error) {
    // 1. Generate symmetric key
    symKey := make([]byte, 32)
    if _, err := rand.Read(symKey); err != nil {
        return nil, err
    }

    // 2. Fetch grantee's public key from Quidnug
    granteeIdentity, err := s.client.GetIdentity(ctx, grantee)
    if err != nil {
        return nil, err
    }

    // 3. Encrypt symKey with ECIES to grantee
    encryptedKey, err := ecies.Encrypt(granteeIdentity.PublicKey, symKey)
    if err != nil {
        return nil, err
    }

    // 4. Publish access-grant event on subject's stream
    grant := EventTransaction{
        SubjectID:   s.quid,
        SubjectType: "QUID",
        EventType:   "credit.access-grant",
        Payload: map[string]interface{}{
            "grantedTo":        grantee,
            "scope":            scopes,
            "validUntil":       time.Now().Add(duration).Unix(),
            "encryptedKey":     base64.StdEncoding.EncodeToString(encryptedKey),
            "grantVersion":     1,
        },
        Creator:   s.quid,
        Signature: s.sign(...),
    }
    if err := s.client.SubmitEvent(ctx, grant); err != nil {
        return nil, err
    }

    return &AccessGrant{
        Grantee:      grantee,
        SymKey:       symKey,
        Scopes:       scopes,
        ValidUntil:   time.Now().Add(duration),
    }, nil
}
```

### 2c. Lender runs credit evaluation

```go
package lender

type CreditEvaluator struct {
    client       QuidnugClient
    lenderQuid   string
    privateKey   *ecdsa.PrivateKey   // for decrypting access grants
    policy       UnderwritingPolicy
}

func (l *CreditEvaluator) Evaluate(ctx context.Context, subjectQuid string, loanRequest LoanRequest) (*Decision, error) {
    // 1. Retrieve public events
    events, err := l.client.GetSubjectEvents(ctx, subjectQuid, "QUID")
    if err != nil {
        return nil, err
    }

    // 2. Retrieve access grant (if any)
    grant, symKey, err := l.findAccessGrantForMe(events)
    if err != nil {
        return nil, err
    }

    // 3. Fetch and decrypt detail blobs for relevant events
    decryptedEvents := l.decryptEvents(events, symKey, grant.Scope)

    // 4. Retrieve relevant trust edges
    edges, err := l.client.GetTrustEdges(ctx, GetEdgesFilter{
        Trustee: subjectQuid,
        DomainPrefix: "credit.",
    })
    if err != nil {
        return nil, err
    }

    // 5. Compute trust per-domain relative to this lender
    directTrust := l.computeDirectTrust(subjectQuid, loanRequest.Category)
    transitiveTrust := l.computeTransitiveTrust(subjectQuid, loanRequest.Category, edges)
    altDataTrust := l.computeAltDataTrust(subjectQuid, edges)
    identityTrust := l.computeIdentityTrust(subjectQuid, edges)

    // 6. Check verification requirements
    if identityTrust < l.policy.MinIdentityTrust {
        return &Decision{Approved: false, Reason: "insufficient-identity-verification"}, nil
    }

    // 7. Apply policy
    composite := l.policy.CompositeFormula(directTrust, transitiveTrust, altDataTrust)

    if composite < l.policy.MinCompositeTrust {
        return &Decision{Approved: false, Reason: "insufficient-composite-trust"}, nil
    }

    // 8. Rate calculation
    rate := l.policy.RateForTrust(composite, loanRequest)

    return &Decision{
        Approved: true,
        Rate: rate,
        Principal: loanRequest.Amount,
        Term: loanRequest.Term,
        CompositeTrust: composite,
        DirectTrust: directTrust,
        TransitiveTrust: transitiveTrust,
        AltDataTrust: altDataTrust,
    }, nil
}

func (l *CreditEvaluator) computeDirectTrust(subject, category string) float64 {
    // Is there a trust edge from me directly to this subject in this domain?
    domain := fmt.Sprintf("credit.%s.us", category)
    edges, _ := l.client.GetTrustEdges(ctx, GetEdgesFilter{
        Truster: l.lenderQuid,
        Trustee: subject,
        Domain:  domain,
    })
    if len(edges) == 0 {
        return 0
    }
    return maxTrustLevel(edges)
}

func (l *CreditEvaluator) computeTransitiveTrust(subject, category string, allEdges []TrustEdge) float64 {
    // BFS through inter-lender trust
    domain := fmt.Sprintf("credit.%s.us", category)

    // Start: find all lenders who trust the subject in this domain
    subjectEndorsers := filterEdges(allEdges, trustee(subject), domain(domain))

    best := 0.0
    for _, edge := range subjectEndorsers {
        // How much does MY lender trust the endorsing lender?
        myTrustInEndorser := l.computeDirectInterLenderTrust(l.lenderQuid, edge.Truster)
        transitive := myTrustInEndorser * edge.TrustLevel
        if transitive > best {
            best = transitive
        }
    }
    return best
}
```

### 2d. Loan origination

After approval and subject's acceptance:

```go
func (l *Lender) OriginateLoan(ctx context.Context, sub string, terms LoanTerms) error {
    // 1. Build the detail blob
    detail := map[string]interface{}{
        "exactPrincipal":  terms.Principal,
        "annualRate":      terms.Rate,
        "monthlyPayment":  terms.MonthlyPayment,
        "paymentSchedule": terms.Schedule,
        "collateral":      terms.Collateral,
        "borrowerAddress": terms.Address,
        "agreementHash":   terms.AgreementHash,
    }
    blob, _ := json.Marshal(detail)

    // 2. Encrypt with symmetric key (shared with subject via access grant)
    symKey := l.getOrGenerateSymKey(ctx, sub)
    encrypted := encryptGCM(blob, symKey)

    // 3. Upload to IPFS, get CID
    cid, err := l.ipfs.Add(ctx, encrypted)
    if err != nil {
        return err
    }

    // 4. Emit origination event on subject's stream
    event := EventTransaction{
        SubjectID:   sub,
        SubjectType: "QUID",
        EventType:   "credit.loan.originated",
        Payload: map[string]interface{}{
            "counterparty":      l.lenderQuid,
            "category":          terms.Category,
            "principalBand":     principalBand(terms.Principal),
            "termMonths":        terms.TermMonths,
            "originationDate":   time.Now().Unix(),
            "annualRateBand":    rateBand(terms.Rate),
            "detailCID":         cid,
            "detailHash":        sha256sum(encrypted),
            "accessGrantPolicy": "subject-approved-only",
        },
        Creator:   l.lenderQuid,
        Signature: l.sign(/* canonical */),
    }
    return l.client.SubmitEvent(ctx, event)
}
```

### 2e. Subject acknowledges

```bash
curl -X POST $NODE/api/v1/events -d '{
  "subjectId":"subject-alice-chen-xyz",
  "subjectType":"QUID",
  "eventType":"credit.loan.acknowledged",
  "payload":{
    "originationEventID":"<event id from step 2d>",
    "termsAccepted":true,
    "signedAt":1713400000
  },
  "creator":"subject-alice-chen-xyz",
  "signature":"<subject signs>"
}'
```

## 3. Monthly payment events

Each month, lender emits a payment event:

```bash
# On-time
curl -X POST $NODE/api/v1/events -d '{
  "subjectId":"subject-alice-chen-xyz",
  "subjectType":"QUID",
  "eventType":"credit.loan.payment-received",
  "payload":{
    "loanRef":"<origination event id>",
    "paymentDate":1716048000,
    "onTime":true,
    "daysLate":0,
    "amountBand":"400-500",
    "detailCID":"bafy...",
    "detailHash":"<sha256>"
  },
  "creator":"lender-chase-bank",
  "signature":"<lender signs>"
}'

# Late
curl -X POST $NODE/api/v1/events -d '{
  ...
  "eventType":"credit.loan.payment-late",
  "payload":{
    "loanRef":"...",
    "daysLate":5,
    "noticeIssued":true
  },
  ...
}'
```

## 4. Dispute flow

### 4a. Subject files dispute

```bash
curl -X POST $NODE/api/v1/events -d '{
  "subjectId":"subject-alice-chen-xyz",
  "subjectType":"QUID",
  "eventType":"credit.dispute.opened",
  "payload":{
    "contestsEventID":"<id of disputed late-payment event>",
    "contestsLender":"lender-chase-bank",
    "contestType":"error",
    "evidenceCID":"bafy... (encrypted blob of bank transfer receipts)",
    "evidenceHash":"<sha256>",
    "requestedRemedy":"withdraw claim; payment was sent on time"
  },
  "creator":"subject-alice-chen-xyz",
  "signature":"<subject signs>"
}'
```

### 4b. Lender responds

```go
package lender

func (l *Lender) InvestigateDispute(ctx context.Context, disputeEventID string) error {
    dispute, err := l.client.GetEvent(ctx, disputeEventID)
    if err != nil {
        return err
    }

    // Access subject's evidence blob (they shared the key via access grant)
    evidence, err := l.fetchAndDecryptEvidence(ctx, dispute.Payload["evidenceCID"].(string))
    if err != nil {
        return err
    }

    // Investigate internally
    decision := l.reviewClaim(evidence, dispute)

    // Emit response
    response := EventTransaction{
        SubjectID:   dispute.SubjectID,
        SubjectType: "QUID",
        EventType:   "credit.dispute.responded",
        Payload: map[string]interface{}{
            "disputeRef":        disputeEventID,
            "response":          decision.Response, // "accepted" | "denied" | "partial"
            "resolution":        decision.Resolution,
            "correctionEventID": decision.CorrectionEventID,
        },
        Creator:   l.lenderQuid,
        Signature: l.sign(/* canonical */),
    }
    return l.client.SubmitEvent(ctx, response)
}
```

### 4c. Correction (if accepted)

```bash
curl -X POST $NODE/api/v1/events -d '{
  "subjectId":"subject-alice-chen-xyz",
  "subjectType":"QUID",
  "eventType":"credit.loan.correction",
  "payload":{
    "correctsEventID":"<original late-payment event>",
    "correctionReason":"Bank processing delay confirmed; payment was timely",
    "newFacts":{
      "onTime":true,
      "daysLate":0
    }
  },
  "creator":"lender-chase-bank",
  "signature":"<lender signs>"
}'
```

When future evaluators read the history, they see the late-
payment event + subject's dispute + lender's response + the
correction. The correction supersedes; evaluators counting the
history treat this payment as on-time.

### 4d. Escalation to arbiter

If subject disagrees with lender's denial:

```bash
# Subject engages an arbiter (arbiter's fee handled off-chain)
curl -X POST $NODE/api/v1/events -d '{
  "subjectId":"subject-alice-chen-xyz",
  "subjectType":"QUID",
  "eventType":"credit.dispute.arbitration-requested",
  "payload":{
    "disputeRef":"<original dispute event id>",
    "arbiterQuid":"arbiter-consumer-financial-watch"
  },
  "creator":"subject-alice-chen-xyz",
  "signature":"<subject signs>"
}'

# Arbiter reviews (off-chain process), then emits opinion
curl -X POST $ARBITER_NODE/api/v1/events -d '{
  "subjectId":"subject-alice-chen-xyz",
  "subjectType":"QUID",
  "eventType":"credit.dispute.arbitration-opinion",
  "payload":{
    "disputeRef":"<id>",
    "opinion":"subject-supported",
    "reasoningCID":"bafy... (full report)"
  },
  "creator":"arbiter-consumer-financial-watch",
  "signature":"<arbiter signs>"
}'
```

Lenders weigh the arbiter's opinion per their own trust.

## 5. Loan payoff and endorsement

```bash
# Final payment event
curl -X POST $NODE/api/v1/events -d '{
  "subjectId":"subject-alice-chen-xyz",
  "eventType":"credit.loan.paid-off",
  "payload":{
    "loanRef":"<origination id>",
    "finalPaymentDate":1744800000,
    "summary":{
      "totalPayments":60,
      "onTimePayments":58,
      "latePayments":2,
      "maxDaysLate":9,
      "renegotiations":0,
      "disputesRaised":1,
      "disputesResolved":1
    }
  },
  "creator":"lender-chase-bank","signature":"..."
}'

# Lender issues reputation trust edge
curl -X POST $NODE/api/trust -d '{
  "truster":"lender-chase-bank",
  "trustee":"subject-alice-chen-xyz",
  "trustLevel":0.88,
  "domain":"credit.auto-loan.us",
  "nonce":47,
  "validUntil":<now + 3y>,
  "description":"60mo auto loan; 58/60 on-time; dispute resolved; paid in full",
  "attributes":{
    "loanRef":"<origination id>",
    "principalBand":"20k-30k",
    "onTimePayments":58,
    "maxDaysLate":9
  }
}'
```

## 6. Subject-side: view and share credit history

### View own history

```bash
$ quidnug-credit history
Your history as of 2026-04-18:

CREDIT RELATIONSHIPS:
  1. Chase Bank — Auto Loan
     Status: Paid in full (2026-03-15)
     Principal band: 20k-30k
     Payment history: 58/60 on-time, 2 late (max 9 days)
     Trust level endorsed: 0.88 @ credit.auto-loan.us
     Dispute history: 1 filed, resolved

  2. Wells Fargo — Credit Card
     Status: Active
     Limit band: 5k-10k
     Current: Paid in full 36/36 months

ALTERNATIVE DATA:
  1. Con Edison Utility — 36 months on-time
     Trust level endorsed: 0.88 @ credit.alternative-data.utilities

IDENTITY VERIFICATIONS:
  1. TX DMV — verified 2024-03-15, valid until 2029-03-15

DISPUTES:
  1. Chase/late-payment/2025-07 — resolved in my favor
```

### Grant access to a new lender

```bash
$ quidnug-credit grant-access lender-newfintech-lending \
    --scope "credit.loan.*" \
    --duration 30d

Granted access to lender-newfintech-lending for 30 days.
Access-grant event ID: <id>
```

### Revoke access

```bash
$ quidnug-credit revoke-access lender-newfintech-lending
Access-grant revoked.
Note: lender may retain data they already cached.
```

## 7. Anyone: query public credit statistics (anonymized)

```bash
# Count how many loans originated in past month (public metadata)
curl "$ANY_NODE/api/v1/events/search?\
  eventType=credit.loan.originated&\
  since=2026-03-18&\
  until=2026-04-18"

# Returns event count; specific subjects not revealed unless
# querier has access grants
```

## 8. Key rotation / recovery

### Lost subject key → guardian recovery

```bash
# Subject's guardians (spouse, lawyer, backup-device) initiate recovery
curl -X POST $NODE/api/v2/guardian/recovery/init -d '{
  "subjectQuid":"subject-alice-chen-xyz",
  "fromEpoch":0,
  "toEpoch":1,
  "newPublicKey":"<hex of new key on new device>",
  "guardianSigs":[
    {"guardianQuid":"alice-spouse","keyEpoch":0,"signature":"..."},
    {"guardianQuid":"alice-lawyer","keyEpoch":0,"signature":"..."}
  ],
  ...
}'
```

After recovery, Alice's credit history is still linked to her
Subject Quid. Lenders re-query; they see the same history.
Access grants issued under the old epoch are invalidated
(lenders need new access grants after rotation). This is a
security property: stolen access grants don't survive a
compromise-rotation.

## 9. Testing

```go
func TestCredit_BasicEvaluation(t *testing.T) {
    // Subject has: 1 paid auto loan, 3 years utility history
    // Lender with direct trust in utility, no direct in subject
    // Expected: composite trust > 0.5 → approval at competitive rate
}

func TestCredit_AccessGrantExpiry(t *testing.T) {
    // Subject grants access for 30 days
    // Lender can fetch details within window
    // After expiry, new access grant required
}

func TestCredit_DisputeFlow(t *testing.T) {
    // Lender reports late payment
    // Subject files dispute
    // Lender accepts, emits correction
    // Subsequent evaluations see corrected history
}

func TestCredit_DisputeUnresolved(t *testing.T) {
    // Lender denies dispute
    // Arbiter opinion in subject's favor
    // Future lenders can weight the arbiter's view
    // Lender's cross-lender trust may degrade
}

func TestCredit_CollusionDetection(t *testing.T) {
    // Fake lender endorses subject at 0.99
    // Real lenders don't have trust in fake lender
    // Transitive trust through fake lender = 0
    // Evaluation relies on real signals only
}

func TestCredit_GuardianRecovery(t *testing.T) {
    // Subject's key compromised; rotated via guardians
    // Post-rotation: old access grants invalid
    // Subject issues new grants; lenders can re-fetch
    // History remains attached to subject quid
}
```

## Where to go next

- [`threat-model.md`](threat-model.md) — threats and mitigations
- [`../elections/`](../elections/) — related BYOQ + privacy
  design
- [`../merchant-fraud-consortium/`](../merchant-fraud-consortium/)
  — related cross-party trust graph
