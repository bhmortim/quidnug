# Architecture: Decentralized Credit & Reputation

Data model, domain hierarchy, privacy mechanism, and component
breakdown for a Quidnug-based credit system.

## Node roles

| Node type               | Operator                        | Function                                        |
|-------------------------|---------------------------------|------------------------------------------------|
| Subject wallet          | Individual / business            | Generates quid, manages keys, grants access    |
| Lender node             | Bank, fintech, private lender    | Submits events, runs own trust computation     |
| Alt-data publisher      | Utility, landlord, employer      | Publishes monthly payment attestations         |
| Identity verifier node  | DMV, KYC provider                | Issues verification trust edges                |
| Dispute arbiter node    | Voluntary third party            | Publishes opinions on disputes                 |
| Observer node (r/o)     | Regulator, research, consumer advocacy | Reads public chain for analytics         |

Subject wallet is typically a mobile app backed by key
management (secure enclave on iOS/Android). Lender nodes are
full Quidnug nodes. Observer nodes are read-only Quidnug
nodes.

## Cryptographic primitives

| Primitive                            | Used for                                          |
|-------------------------------------|---------------------------------------------------|
| ECDSA P-256 (native)                | All identity, event, trust-edge signatures        |
| SHA-256                             | Quid IDs, event hashes, blob integrity            |
| **ECIES (EC P-256)**                | Encrypting access-grant keys to specific lenders  |
| **AES-256-GCM**                     | Encrypting detail blobs                           |
| IPFS CID (sha256 default)           | Off-chain payload addressing                      |
| HMAC-SHA256 (existing Quidnug)      | Inter-node authentication                         |

ECIES is a well-established standard and can be layered on
Quidnug's existing ECDSA keys. A subject's single keypair can
be used for both signing (Quidnug-native) and encryption (ECIES)
with standard key-derivation.

## Domain hierarchy (complete)

```
credit
├── credit.identity-verification.<country>
├── credit.reports                                    (global cross-reference, rarely used)
│
├── credit.mortgage.<country>
├── credit.auto-loan.<country>
├── credit.personal-loan.<country>
├── credit.credit-card.<country>
├── credit.small-business.<country>
├── credit.student-loan.<country>
├── credit.buy-now-pay-later.<country>
│
├── credit.alternative-data.utilities
├── credit.alternative-data.rent
├── credit.alternative-data.employment
├── credit.alternative-data.subscriptions    (Netflix-style recurring)
│
├── credit.disputes
│
├── credit.inter-lender-trust                 (lenders endorsing each other)
│
└── credit.arbiter-trust                      (subjects' trust in arbiters)
```

### Cross-domain trust

Some trust is meaningful across domains. A prospective mortgage
lender reviewing a subject typically cares about:

- Direct `credit.mortgage.*` history (highest weight)
- Indirect `credit.auto-loan.*` and `credit.personal-loan.*`
  as indicators of repayment behavior (lower weight)
- Alt-data (`credit.alternative-data.*`) as supporting signal

The cross-domain weighting is **the lender's choice**. Some
lenders cross-count heavily; others don't. There's no protocol
mandate.

### Jurisdiction scoping

Domains include country code because regulatory regimes differ.
A US lender's endorsement in `credit.auto-loan.us` has
different legal meaning than an endorsement in
`credit.auto-loan.eu`. Cross-border evaluation is possible
(trust edges from foreign lenders are first-class) but lenders
weight them per their own policies.

## Quid schemas

### Subject Quid

```go
type SubjectIdentity struct {
    QuidID       string    // sha256 of public key, 16 hex
    PublicKey    []byte    // ECDSA P-256
    Creator      string    // self (BYOQ)
    UpdateNonce  int64
    Attributes   struct {
        // Deliberately minimal on-chain.
        // Identity facts are represented by signed trust edges
        // from verifiers, not embedded here.
        CreatedAt     int64
    }
}
```

No SSN, no name, no address on-chain. All those are off-chain
in verifiers' systems and in encrypted blobs.

### Lender Quid

```go
type LenderIdentity struct {
    QuidID      string
    PublicKey   []byte
    Attributes  struct {
        LegalName         string     // "JPMorgan Chase Bank, N.A."
        Jurisdiction      string     // "US"
        RegulatoryID      string     // OCC number, FDIC cert, etc.
        LenderCategory    []string   // ["mortgage","auto-loan",...]
        LicenseVerified   bool       // endorsed by regulator trust edge
    }
}

type LenderGuardianSet struct {
    // Bank's own recovery/authority structure
    Guardians  []GuardianRef  // CEO, CRO, compliance, regulator
    Threshold  uint16
    RecoveryDelay time.Duration  // e.g. 72h
    RequireGuardianRotation: true
}
```

### Alternative-data source quid

```go
type AltDataSourceIdentity struct {
    QuidID      string
    Attributes  struct {
        LegalName        string
        DataType         string  // "utility-payments", "rent", "employment"
        Jurisdiction     string
        CustomerAgreedToOpt OptInTermsHash string  // how they get subject consent
    }
}
```

### Identity verifier quid

```go
type VerifierIdentity struct {
    QuidID     string
    Attributes struct {
        Type         string   // "DMV", "passport-office", "KYC-provider", "bank-KYC"
        Jurisdiction string
        Authority    string   // "US Department of State", "NY DMV", "Plaid-KYC"
    }
}
```

## Event schemas

### Credit event: loan origination

```go
type LoanOriginationEvent struct {
    BaseEvent                      // standard EVENT fields
    SubjectID    string             // borrower's quid
    SubjectType  string             // "QUID"
    EventType    string             // "credit.loan.originated"
    Payload      struct {
        Counterparty      string   // lender's quid
        Category          string   // "auto-loan" | "mortgage" | etc.
        PrincipalBand     string   // "5k-10k", "10k-25k", etc. (public coarse)
        TermMonths        int
        OriginationDate   int64
        AnnualRateBand    string   // "4-6%", "6-10%" (coarse)
        DetailCID         string   // IPFS CID of encrypted blob
        DetailHash        string   // sha256 of encrypted blob
        AccessGrantPolicy string   // "subject-approved-only"
    }
    Creator      string            // lender's quid
    Signature    string            // lender's signature
}
```

### Credit event: payment

```go
type PaymentEvent struct {
    EventType  string   // "credit.loan.payment-received" | "credit.loan.payment-late"
    Payload    struct {
        LoanRef        string    // reference to origination event ID
        PaymentDate    int64
        OnTime         bool
        DaysLate       int       // 0 if onTime
        AmountBand     string    // coarse
        DetailCID      string
        DetailHash     string
    }
    Creator    string            // lender
}
```

### Trust edge: lender endorses subject

```go
type CreditTrustEdge struct {
    BaseTrustEdge              // standard TRUST fields
    Truster     string          // lender's quid
    Trustee     string          // subject's quid
    TrustLevel  float64         // 0.0 - 1.0, lender's discretion
    Domain      string          // e.g., "credit.auto-loan.us"
    Nonce       int64
    ValidUntil  int64
    Attributes  struct {
        RelationshipSummary string  // human-readable
        LoansInvolved       []string  // origination event IDs
        AggregatedMetrics   struct {
            TotalPrincipalBand  string
            TotalPayments       int
            LatePayments        int
            MaxDaysLate         int
            Renegotiations      int
        }
    }
}
```

### Access grant event

```go
type CreditAccessGrantEvent struct {
    EventType  string   // "credit.access-grant"
    Payload    struct {
        GrantedTo         string      // lender/verifier quid
        Scope             []string    // event-type-globs to which this grants
        ValidUntil        int64
        EncryptedKey      []byte      // symmetric key encrypted to grantee's pubkey via ECIES
        GrantVersion      int         // for revocation/updates
    }
    Creator    string               // subject (self)
}
```

### Dispute event

```go
type CreditDisputeEvent struct {
    EventType  string   // "credit.dispute.opened"
    Payload    struct {
        ContestsEventID  string       // the disputed event's id
        ContestsLender   string
        ContestType      string       // "identity-theft" | "error" | "misclassification"
        EvidenceCID      string       // IPFS CID of evidence docs
        RequestedRemedy  string
    }
    Creator    string               // subject
}

type CreditDisputeResponseEvent struct {
    EventType  string   // "credit.dispute.responded"
    Payload    struct {
        DisputeRef     string
        Response       string       // "accepted" | "denied" | "partial"
        Resolution     interface{}  // varies
        CorrectionEventID string    // if applicable
    }
    Creator    string              // lender
}
```

## Privacy mechanism in depth

### Encrypted detail blob structure

Off-chain blob (stored in IPFS or equivalent, addressed by CID):

```json
{
  "blobVersion": 1,
  "encryptionAlgorithm": "AES-256-GCM",
  "detail": {
    "exactPrincipal": "23451.00",
    "annualRate": "5.74%",
    "monthlyPayment": "450.33",
    "paymentSchedule": [ ... ],
    "collateral": "...",
    "borrowerAddress": "...",
    "agreement": "<hash of full contract>",
    "internalNotes": "..."
  }
}
```

The whole blob is encrypted with a random symmetric key. The
CID addresses the encrypted bytes.

### Access grant mechanism

The symmetric key is distributed to authorized parties via
ECIES-encrypted access grants:

1. Subject knows the symmetric key (they approved the event,
   they have a copy).
2. When the subject wants to share with a lender:
   - Generate an ephemeral ECDH keypair.
   - Compute shared secret with the lender's public key.
   - Derive a key-encryption key (KEK).
   - Encrypt the symmetric key with the KEK.
   - Publish the encrypted key + ephemeral public point as an
     access-grant event.
3. Lender receives the access grant:
   - Compute the same shared secret using their private key +
     the ephemeral public point.
   - Derive the KEK.
   - Decrypt the symmetric key.
   - Fetch the blob from IPFS. Decrypt. Verify hash matches.

### What can a passive observer see?

The public chain reveals:
- "Subject X has a credit event of type `credit.loan.originated`
  with counterparty Chase on date D, in category `auto-loan`,
  principal band 20k-30k, term 60 months"
- "Subject X paid on time 58 of 60 months; 2 late payments of ≤10 days"
  (aggregated in the final loan.paid-off event's summary)
- Trust edge: Chase trusts Subject X at 0.88 for 3 years

What the public chain does NOT reveal:
- Exact principal
- Interest rate
- Exact payment amounts
- Personal details of subject
- Reason for any late payment
- Details of any dispute

Is this level of public info enough to cause concern? That's a
design choice. For stronger privacy, a jurisdiction can deploy
with:
- **Commitment-only events** — even the counterparty is hidden
  on-chain; only a commitment is public. Full data (including
  counterparty identity) is in the encrypted blob.
- **Zero-knowledge proofs** — for computed evaluations ("this
  subject's on-time payment ratio ≥ 90%") without revealing
  the underlying events.

These would be QDPs in their own right. The baseline design
gives privacy comparable to what current credit reports show
a lender, while enabling far more granular consumer control.

## Trust computation details

### Direct trust

A direct trust edge from a specific counterparty in the queried
domain:

```
query: evaluate Alice for an auto loan, I am lender Chase

edges: Chase --0.92--> Alice   in credit.auto-loan.us  (found)
direct trust = 0.92
```

### Transitive trust

Through other lenders Chase trusts:

```
edges:
  Chase ---0.9----> Wells-Fargo    in credit.inter-lender-trust
  Chase ---0.85---> Capital-One    in credit.inter-lender-trust
  Wells-Fargo ---0.85---> Alice    in credit.auto-loan.us
  Capital-One ---0.80---> Alice    in credit.auto-loan.us

transitive paths:
  Chase -> Wells-Fargo -> Alice  = 0.9 * 0.85 = 0.765
  Chase -> Capital-One -> Alice  = 0.85 * 0.80 = 0.68

max transitive = 0.765
```

### Alternative-data trust

Same shape but in alt-data domains:

```
edges:
  Chase ---0.7----> Con-Edison   in credit.inter-lender-trust (or a special alt-data trust domain)
  Con-Edison ---0.88--> Alice    in credit.alternative-data.utilities

transitive = 0.7 * 0.88 = 0.616
```

### Aggregation — lender's discretion

Simple formulas:

```
conservative_score = max(direct, transitive, alt_data)
progressive_score = 0.5 * direct + 0.3 * transitive + 0.2 * alt_data
weighted_by_recency = ... (weight recent edges more)
```

The lender writes this logic once. Queries Quidnug for all
edges + events. Computes. Decides. Independent from any bureau.

### Anti-collusion trust math

A subject cannot simply pay a friendly "fake lender" for a
glowing endorsement because the fake lender's edges are only
worth as much as other lenders trust the fake lender.

- Fake lender needs inter-lender trust to be useful
- To gain inter-lender trust, it needs real loan history with
  subjects other lenders trust
- Building this trust takes years and real-capital exposure
- At scale, fraud rings are detectable (same subjects
  endorsing each other with no external borrowing activity)

This is structurally much harder than the current system,
where a lender can simply file false data to a bureau.

## Protocol flows (concise)

### Subject onboarding
1. Subject generates quid locally
2. Subject visits identity verifier (in-person / online KYC)
3. Verifier issues trust edge in `credit.identity-verification.*`
4. Subject links alt-data sources (utility, rent, employer)
5. Alt-data publishers publish monthly attestation events

### Credit relationship
1. Subject applies at lender (off-chain UI)
2. Lender requests access grant from subject
3. Subject grants access for specific scope + duration
4. Lender fetches events + edges, runs own model
5. Decision made; if approved, lender issues origination event
6. Monthly payment events over loan lifetime
7. At payoff, lender issues trust edge endorsement

### Dispute
1. Subject emits `credit.dispute.opened` contesting a specific event
2. Lender responds within window
3. Optional arbiter opinion
4. Correction event if warranted, or dispute remains public record

## Scale estimates

A mid-size national deployment:
- 100M subjects
- 10,000 lender nodes (banks, credit unions, fintechs)
- 1,000 alt-data publisher nodes
- 50 identity verifier nodes
- 500 arbiter nodes

Per-subject activity:
- ~5-10 active credit relationships at any time
- ~1 credit event per active relationship per month
- Total: 60-120 events/subject/year
- 100M subjects × 100 events/year = 10B events/year ≈ 300M/day

Event volume is large but not unusual for a blockchain system.
Push gossip's producer rate limits cap per-lender spam.

Trust-graph size: 100M subjects × avg 10 inter-lender trust
edges = 1B edges. Handleable with good indexing. Most queries
are per-subject so they don't touch the full graph at once.

## Next

- [`implementation.md`](implementation.md) — concrete API calls
- [`threat-model.md`](threat-model.md) — attackers & mitigations
