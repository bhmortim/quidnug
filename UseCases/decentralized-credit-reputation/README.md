# Decentralized Credit & Reputation

**FinTech · Consumer rights · Anti-social-credit · Data sovereignty**

> A replacement for credit bureaus and social-credit systems.
> No central scorer. No single database that can leak. No
> universal number that follows you forever. Instead: a
> cryptographically-signed, borrower-owned history where every
> potential lender computes their own view and the borrower
> controls who sees what.

---

## The two problems — one shape

### Credit bureaus (Equifax / Experian / TransUnion, and their global peers)

Three private companies hold the financial-reputation destinies
of hundreds of millions of people. The status-quo is well-
documented to be broken:

- **Errors are rampant.** FTC studies find 20–25% of credit
  reports contain errors; ~5% contain errors significant
  enough to cause loan denial. Correcting them takes 30–180
  days of disputes — and bureaus often fail to investigate at
  all, merely forwarding the dispute to the lender who made
  the claim.
- **Breaches are catastrophic.** The 2017 Equifax breach
  exposed SSNs, birthdates, addresses, and credit details of
  147 million Americans. A centralized database is a
  centralized target.
- **Opacity.** The FICO scoring formula is proprietary.
  VantageScore is proprietary. Consumers can't replicate the
  calculation. "Why was I denied?" is answered with a
  boilerplate reason code, not the actual math.
- **Thin-file exclusion.** Young adults, immigrants, the
  underbanked, and anyone who avoids credit cards has no
  score — and no access to mainstream credit — because the
  bureau data model doesn't recognize alternative indicators
  of creditworthiness.
- **You don't own the data.** Credit bureaus gather information
  about you without consent, sell it, and you have no
  mechanism to opt out of the system.
- **Universal score.** The same 720 follows you to the
  mortgage lender, the car dealer, the apartment landlord,
  the business-loan underwriter. Each has very different risk
  models, but the score doesn't know that.
- **Gaming.** "Authorized user" scams, credit-repair
  manipulators, and thin-to-thick-file tricks exist because
  the scoring is formulaic and gameable.

### Social-credit systems (state-operated reputation)

Some jurisdictions have experimented with state-run reputation
systems that aggregate financial, legal, behavioral, and social
data into a single score that gates access to travel, loans,
jobs, school admissions, and more.

Even where critics of these systems and proponents disagree on
whether they "work," both sides agree on the structural risks:

- **Concentration of judgment.** A single authority decides what
  is "good citizenship" and what is not. Dissent becomes a score
  penalty.
- **Irreversible effects.** A low score can lock someone out
  of public services with no clear path to rehabilitate.
- **Behavioral drift.** Citizens self-censor because anything
  might affect the score.
- **Due process vacuum.** Appeals and dispute mechanisms are
  ad hoc; the algorithm is the judge.
- **Discriminatory aggregation.** Political activity, social
  associations, religious practice — whatever the authority
  considers "bad" — can accumulate into economic exclusion.

### Why they share a shape

Both systems concentrate the **power to judge** in one entity
and produce a **universal number** that affects access to
opportunity. The judge has no skin in the game for your
outcomes. The number follows you without your consent. Fixing
errors means petitioning the judge.

**The root problem is architectural:** centralized judgment
applied to an entire population.

---

## The Quidnug alternative: relational trust + signed history

In Quidnug, there is **no score**. There is:

1. **A history of signed events** attached to the subject (the
   borrower/citizen): loans taken, payments made, defaults,
   renegotiations, alternative-data attestations.
2. **Trust edges from lenders/attesters to the subject** in
   specific domains: `credit.mortgage.us`, `credit.auto-loan.us`,
   `credit.small-business.consumer-goods`, etc.
3. **Each potential counterparty computes their own evaluation**
   using their own trust graph. Lender A's view of a borrower
   might be 0.85; Lender B's view of the same borrower might be
   0.60; neither is "the score" — both are correct from each
   lender's perspective.

The subject **owns their history**. They control who sees it.
Lenders build reputation by their own accuracy; a lender who
files false defaults finds other lenders devaluing their
attestations. Government observers can watch public events but
have no protocol authority to impose a universal score.

### Mapping the problems to Quidnug primitives

| Problem                                  | Quidnug primitive                                 |
|------------------------------------------|---------------------------------------------------|
| "Bureau is a central point of failure"   | No bureau. Subject's quid is the canonical id.   |
| "Errors take months to fix"              | Dispute event on-chain; visible to all counterparties instantly. |
| "I don't own my data"                    | Subject's quid + borrower-controlled encryption keys |
| "One score everywhere"                   | Per-domain relational trust; different views per lender |
| "Thin-file exclusion"                    | Alternative data sources (utilities, rent) as first-class signers |
| "Opaque scoring formula"                 | Each lender's algorithm is their own; consumers can run their own. |
| "Social-credit concentration"            | No protocol authority to issue a universal score |
| "Data breach exposure"                   | Distributed + encrypted off-chain payloads       |
| "Lender defamation"                      | Dispute + lender reputation via consumer trust   |
| "Lender collusion (fake credit)"         | Cross-lender trust means colluders need to trick everyone |

---

## Core design

### Quid types

| Quid type            | Owned by                                    | Example                              |
|----------------------|---------------------------------------------|--------------------------------------|
| **Subject**          | The individual / business (BYOQ)            | `subject-alice-chen-xyz`             |
| **Lender**           | A financial institution                     | `lender-chase-bank`                  |
| **Alt-data source**  | Utility, landlord, employer                 | `utility-con-edison-nyc`             |
| **Identity verifier**| KYC provider / government                   | `verifier-dmv-texas`                 |
| **Dispute arbiter**  | Voluntary mediator (CFPB-style, private)    | `arbiter-consumer-financial-watch`   |
| **Guarantor**        | A party that co-signs / underwrites         | `guarantor-parent-bob-chen`          |

All are regular Quidnug quids. What distinguishes them is the
domains they operate in and the trust edges others have issued
to them.

### Domain hierarchy

```
credit                                            (top-level)
├── credit.reports                                 (generic credit-event stream)
│
├── credit.mortgage.<country>
├── credit.auto-loan.<country>
├── credit.personal-loan.<country>
├── credit.small-business.<country>
├── credit.credit-card.<country>
├── credit.student-loan.<country>
│
├── credit.alternative-data.utilities
├── credit.alternative-data.rent
├── credit.alternative-data.employment
│
├── credit.disputes
│
└── credit.identity-verification
```

Each lender / attester / verifier operates in the domains where
they have legitimate authority. A utility company signs events
in `credit.alternative-data.utilities` but not in
`credit.mortgage.us`.

### The three things that replace a credit bureau

#### 1. Credit events (the "history")

Every financial event between a subject and a counterparty is a
signed event on the subject's event stream. The on-chain record
carries only metadata; the sensitive details are encrypted
off-chain.

```json
{
  "subjectId": "subject-alice-chen-xyz",
  "subjectType": "QUID",
  "eventType": "credit.loan.originated",
  "payload": {
    "counterparty": "lender-chase-bank",
    "category": "auto-loan",
    "principalBand": "20k-30k",          // coarse public bucket
    "termMonths": 60,
    "originationDate": 1713400000,
    "detailCID": "bafy...",              // encrypted blob, IPFS
    "detailHash": "<sha256>",            // binds encrypted blob
    "accessGrantPolicy": "subject-approved-only"
  },
  "signer": "lender-chase-bank"
}
```

Subsequent events on the same subject's stream:

```
credit.loan.payment-received      (signed by lender)
credit.loan.payment-late           (signed by lender, within dispute window)
credit.loan.default-declared       (signed by lender, with dispute window)
credit.loan.restructured          (signed by lender + subject)
credit.loan.paid-off              (signed by lender)
credit.dispute.opened             (signed by subject)
credit.dispute.resolved           (signed by subject + lender)
credit.guarantor.cosigned         (signed by guarantor)
```

Each event is append-only, signed by its actor, and
time-ordered. Over years, a subject accumulates a rich history.

#### 2. Trust edges (the "score inputs")

After each completed credit relationship, the lender issues a
trust edge to the subject in the relevant domain:

```json
{
  "truster": "lender-chase-bank",
  "trustee": "subject-alice-chen-xyz",
  "trustLevel": 0.92,
  "domain": "credit.auto-loan.us",
  "description": "60-month auto loan, paid as agreed, zero late payments",
  "validUntil": 1776844800,
  "nonce": 47
}
```

- `trustLevel` reflects the lender's view. A lender who was
  paid on time + early would issue a higher level than one
  who was paid late-but-eventually.
- `validUntil` is typically 2–5 years; after that the edge
  expires unless reaffirmed. Forces fresh data over ancient.
- `domain` scopes the endorsement. A mortgage lender issuing
  trust in `credit.auto-loan.us` doesn't automatically endorse
  the borrower for `credit.mortgage.us`.
- `nonce` provides replay protection and lets the lender
  update (e.g., downgrade if they later reopen a claim).

Alternative-data sources issue the same kinds of edges in
their own domains:

```json
{
  "truster": "utility-con-edison-nyc",
  "trustee": "subject-alice-chen-xyz",
  "trustLevel": 0.88,
  "domain": "credit.alternative-data.utilities",
  "description": "36 months on-time payment history",
  "validUntil": 1776844800
}
```

#### 3. Per-lender relational trust evaluation (the "score")

When a prospective lender evaluates an applicant, they run
their own relational-trust computation:

```
Chase-Auto-Lending: evaluating Alice for a new auto loan.

Direct history:
  Chase ──0.92──► Alice   (prior auto loan, paid as agreed)
  → direct trust: 0.92 (strongest signal)

Transitive history:
  Wells-Fargo ──0.85──► Alice (mortgage, paid)
  Capital-One ──0.80──► Alice (credit card, paid)
  Chase already trusts Wells-Fargo and Capital-One as credible
  lenders (the industry-peer trust graph).

  Chase ──0.9──► Wells-Fargo
  Chase ──0.9──► Capital-One

  Transitive trust via Wells-Fargo: 0.9 × 0.85 = 0.765
  Transitive trust via Capital-One: 0.9 × 0.80 = 0.72

Alternative-data:
  Con-Edison ──0.88──► Alice
  Chase trusts Con-Edison in utilities-payment domain at 0.7.
  Transitive: 0.7 × 0.88 = 0.616

Aggregation (Chase's own formula):
  max of direct/transitive = 0.92
  sum of alt-data reinforcement = +0.05
  Final: 0.97

  Chase's decision: approve at standard rate.
```

Another lender, say a new fintech startup, runs a different
formula and gets a different number. That's fine. There's no
"right" number — there's each lender's judgment, made with
their own trust graph and their own risk tolerance.

---

## Privacy model: what's on-chain, what's off-chain

Credit history is sensitive. Quidnug's public chain is, well,
public. The design balances these:

### On-chain (public, auditable)

- Subject's quid (public key only)
- Event type (`credit.loan.originated`, etc.)
- Counterparty identity
- Coarse metadata: category, principal-band (e.g., "20k-30k"
  not "$23,451"), term length, origination date
- Hash of the encrypted detail blob
- IPFS CID of the encrypted blob
- Trust edges (truster → trustee + level + domain)
- Dispute events

This level of public visibility is **deliberately similar to
what lenders already learn from credit reports**: "this person
has an auto loan with Chase from 2024." What's NOT public:
the exact principal, rate, payment amounts, etc.

### Off-chain (encrypted, subject-controlled)

- Exact principal amount
- Interest rate and terms
- Monthly payment amount
- Specific payment dates and amounts
- Notes / memos from either party
- Correspondence / dispute details

Stored in IPFS (or S3, or any blob store) as JSON/CBOR
encrypted with a symmetric key. The decryption key lives on
the subject's device.

### How a lender accesses the encrypted details

```
1. Subject grants access to a specific lender:
   - Generates an ephemeral encryption key for the lender
   - Uses ECIES (elliptic-curve integrated encryption scheme)
     to encrypt the symmetric key with the lender's public key
   - Publishes the encrypted key as an access-grant event:

   eventType: "credit.access-grant"
   subjectId: subject-alice-chen-xyz
   payload:
     grantedTo: lender-chase-bank
     scope: ["credit.loan.originated:*", "credit.loan.payment-received:*"]
     encryptedKey: <ECIES(key, chase.pubkey)>
     validUntil: <now + 30 days>
   signer: subject-alice-chen-xyz

2. Lender decrypts the key with their own private key.

3. Lender fetches the encrypted blobs from IPFS.

4. Lender decrypts with the subject-provided key.

5. Now lender has full detail. They verify hashes match.

6. After validUntil, lender re-requests access.
```

**Key property:** The subject grants access to specific lenders
for specific scopes and time windows. A lender can't aggregate
data on non-customers. A "shadow bureau" (scraping the public
chain) sees only coarse metadata — not enough to reconstruct a
person's financial life.

### The social-credit defense

For social-credit prevention specifically, this privacy model
is critical:

- A government observer can see "Alice took out loans" but not
  the amounts, terms, or personal details.
- The government can't unilaterally aggregate. They'd have to
  get Alice's explicit access grants — which she controls.
- Even if a government compels disclosure from individual
  lenders, each lender only has the sliver of history involving
  them — no single entity has the full picture.

---

## Replacing credit reporting — the flows

### Subject onboarding (bring-your-own-quid)

```
Step 1. Individual generates their quid
────────────────────────────────────────
  On their phone / device (no central registration required)

    $ quidnug-credit generate
    Your Subject Quid: subject-alice-chen-xyz
    Your public key: 0x04abcd...
    Your private key: saved locally (keep secure)

  No SSN required. No forced enrollment.

Step 2. Identity verification (optional but useful)
────────────────────────────────────────────────────
  To participate in regulated credit (mortgages, etc.), subject
  needs identity verification. Multiple verifiers can endorse:

  A DMV / KYC provider signs a trust edge:
    TRUST:
      truster: verifier-dmv-texas
      trustee: subject-alice-chen-xyz
      trustLevel: 1.0
      domain: credit.identity-verification.us
      attributes:
        verifiedName: HASH:<name+DOB+SSN>  // hash, not raw
        verificationMethod: "in-person-DMV"
        verificationDate: ...
      validUntil: <5 years>

  Multiple verifiers can endorse the same subject. Lenders
  accept whichever verifier they trust. A lender who doesn't
  trust the Texas DMV can require a different verifier.

Step 3. Alternative-data source linking
────────────────────────────────────────
  Subject links their utility / rent / employer accounts.

  Utility company: "Alice authorized me to attest."
    EVENT: credit.alt-data.linked
    subjectId: subject-alice-chen-xyz
    payload:
      dataSource: utility-con-edison-nyc
      scope: "monthly-payment-history"
      linkedAt: ...
    signer: utility-con-edison-nyc + subject-alice-chen-xyz (co-signed)

  Utility then publishes monthly:
    EVENT: credit.alt-data.payment-record
    payload:
      month: "2026-04"
      onTime: true
      amount: "120-130" (coarse band)
      detailCID: <encrypted for subject + verified>
```

### Taking out a loan

```
Step 1. Subject applies at a lender
─────────────────────────────────────
  Application is off-chain (lender's existing workflow).
  Subject provides: their Subject Quid ID + grant-of-access to
  relevant history.

Step 2. Lender evaluates
─────────────────────────
  Pulls subject's quid, fetches accessible history + trust edges.
  Runs own credit model (as described in §Core design #3).
  Makes approve/decline/adjust-rate decision per their own
  underwriting standards.

Step 3. Loan origination
─────────────────────────
  Lender emits:
    EVENT: credit.loan.originated
    subjectId: subject-alice-chen-xyz
    payload:
      counterparty: lender-chase-bank
      category: "auto-loan"
      principalBand: "20k-30k"
      termMonths: 60
      detailCID: <encrypted blob>
      detailHash: <hash>
      annualRateBand: "4-6%"
    signer: lender-chase-bank

  Subject countersigns via an acknowledgment event:
    EVENT: credit.loan.acknowledged
    payload:
      originationEventID: <id>
      termsAccepted: true
    signer: subject-alice-chen-xyz

  Lender's funds disburse off-chain (bank transfer).
```

### Ongoing payments

```
Monthly, lender emits:
  EVENT: credit.loan.payment-received
  subjectId: subject-alice-chen-xyz
  payload:
    loanRef: <origination event id>
    paymentDate: 1713400000
    onTime: true
    detailCID: <encrypted>
  signer: lender-chase-bank

If late:
  EVENT: credit.loan.payment-late
  payload:
    daysLate: 7
    noticeIssued: true

  Subject can emit a counter-event if they dispute the "late"
  claim (e.g., payment was sent on time, bank delay):
    EVENT: credit.dispute.opened
    payload:
      contestsEventID: <the payment-late event>
      evidence: <hash of bank transfer receipt>
```

### Loan payoff and trust edge

```
At payoff:
  EVENT: credit.loan.paid-off
  payload:
    loanRef: ...
    finalPaymentDate: ...
    summary: {
      totalOnTimePayments: 58,
      totalLatePayments: 2,
      daysOfLatenessMax: 9,
      renegotiations: 0
    }
  signer: lender-chase-bank

And simultaneously, lender issues the reputation trust edge:
  TRUST:
    truster: lender-chase-bank
    trustee: subject-alice-chen-xyz
    trustLevel: 0.88             // reflecting the 2 late payments
    domain: credit.auto-loan.us
    description: "60-month auto loan, 2 late payments (≤9 days),
                  no renegotiation, paid in full"
    validUntil: <now + 3 years>
    nonce: ...
```

This trust edge becomes a persistent positive reference for
Alice, visible to any future auto-loan lender she authorizes.

### Disputes

```
Scenario: Chase reports a "default" on Alice that Alice claims
is wrong (say, identity theft, or an accounting error).

Alice files a dispute event:
  EVENT: credit.dispute.opened
  subjectId: subject-alice-chen-xyz
  payload:
    contestsEventID: <the disputed event's id>
    contestsLender: lender-chase-bank
    contestType: "identity-theft" | "error" | "misclassification" | "other"
    evidence: <IPFS CID of evidence documents>
    requestedRemedy: "withdraw claim" | "correct to ..." | "other"
  signer: subject-alice-chen-xyz

Chase has 30 days (per policy, or per jurisdiction regulation)
to respond:
  EVENT: credit.dispute.responded
  payload:
    disputeRef: <id>
    response: "accepted" | "denied" | "partial"
    resolution: { ... }
  signer: lender-chase-bank

If accepted:
  Chase emits a correction event:
    EVENT: credit.loan.correction
    payload:
      corrects: <original event id>
      newSummary: { /* corrected facts */ }
    signer: lender-chase-bank

If denied:
  Alice can:
    (a) escalate to an arbiter (a neutral third party signs
        their opinion as an arbitration event)
    (b) accept the outcome
    (c) let the dispute stand publicly — future lenders see
        both the original claim and Alice's rebuttal

  Key property: the DISPUTE is always on record. An unresolved
  lender-vs-subject disagreement is visible to every future
  counterparty, who can weigh both sides.

Arbiter involvement (optional):
  EVENT: credit.dispute.arbitration-opinion
  payload:
    disputeRef: <id>
    arbiterQuid: arbiter-consumer-financial-watch
    opinion: "lender-supported" | "subject-supported" | "inconclusive"
    reasoning: <IPFS CID of arbitration report>
  signer: arbiter-consumer-financial-watch

Subject retains full control: they chose to engage this arbiter.
Lenders may honor the opinion or not, weighted by their own
trust in the arbiter.
```

### Lender reputation feedback loop

A lender that files a false default or wrongfully denies a
dispute faces consequences:

- **Subject's future lenders see the dispute + unresolved
  status.** Sophisticated lenders factor this in as noise,
  but they can also see that the lender filing false claims
  has a pattern.
- **Cross-subject pattern analysis** — a lender that has many
  open, subject-contested disputes has a reputation signal
  visible to every other lender. Other lenders may
  downgrade their trust in this lender in the inter-lender
  trust graph, reducing the weight of their attestations.
- **Subject can revoke trust edges to the lender** (e.g.,
  if the subject later becomes a lender or validator in
  another domain).
- **Arbitration records aggregate.** A lender whose claims
  are consistently rejected by respected arbiters has a
  visible pattern.
- **Regulatory action.** Fair-credit regulation (FCRA-equivalents)
  continues to apply. The difference is that enforcement
  has cryptographic evidence to work with.

---

## Replacing the credit score — per-lender evaluation

Every lender runs their own algorithm. Examples:

### Conservative mortgage lender
```
def evaluate_mortgage(subject_quid, requested_amount):
    # Only direct mortgage history counts heavily
    direct_mortgage_trust = compute_direct_trust(
        self=my_quid,
        subject=subject_quid,
        domain="credit.mortgage.us"
    )
    if direct_mortgage_trust < 0.7:
        return DECLINE

    # Verify recent on-time payment history
    events = get_events(subject_quid, domain="credit.*")
    recent_lates = count_events(events, "credit.loan.payment-late",
                                since=now-2years)
    if recent_lates > 3:
        return DECLINE

    # Verify identity via trusted verifier
    if not has_trust_edge(verifier=my_trusted_verifiers,
                         to=subject_quid,
                         domain="credit.identity-verification.us"):
        return DECLINE

    # Approve with standard rate
    return APPROVE, standard_rate
```

### Progressive fintech lender (uses alternative data heavily)
```
def evaluate_personal_loan(subject_quid, requested_amount):
    # Alternative data carries significant weight
    alt_data_trust = 0
    for domain in ["credit.alternative-data.utilities",
                   "credit.alternative-data.rent",
                   "credit.alternative-data.employment"]:
        alt_data_trust += compute_trust(self, subject_quid, domain)

    # Traditional credit is optional but additive
    traditional_trust = compute_trust(self, subject_quid, "credit.*")

    composite = 0.6 * alt_data_trust / 3 + 0.4 * traditional_trust

    # Decline only if composite very low
    if composite < 0.4:
        return DECLINE

    rate = map_composite_to_rate(composite)
    return APPROVE, rate
```

Same subject quid → two lenders → two different decisions.
Both are correct per their own risk models. No central score
overruled or constrained either.

### A subject compares their offers

```
$ quidnug-credit evaluate-offers
Subject: subject-alice-chen-xyz

Pulling offers from 5 lenders...

  Chase (traditional):           APPROVED at 6.5%  ($23,451 / 60 mo)
  Wells Fargo (traditional):     APPROVED at 7.1%
  Capital One (progressive):     APPROVED at 5.8%
  LendFin (alt-data-heavy):      APPROVED at 5.2%
  TraditionalOnly:               DECLINED (thin file)

Best offer: LendFin at 5.2%.
```

Different lenders value different signals. The subject
benefits from competition and from bringing their own
alternative data.

---

## Specific social-credit prevention measures

Beyond the structural "no central scorer," the design
includes specific protections:

### 1. No "total citizen" domain

The domain hierarchy is intentionally bounded to `credit.*`.
There is no `citizenship.*` or `trustworthiness.*` top-level
domain. The protocol doesn't provide a natural home for a
"citizen trust score." A government attempting to create one
would have to define a new domain, and individual lenders
would have to voluntarily accept endorsements from it — which
most wouldn't if it carries political weight.

### 2. Per-domain scoping enforced

A lender's trust edge in `credit.auto-loan.us` does NOT
automatically count in `credit.mortgage.us` or any other
domain. Each evaluator chooses which domains' edges are
relevant to their decision. There's no aggregated "overall
score."

### 3. Subject-owned data access

If a government tries to mandate that all lenders publish every
customer's full history in the clear, the subject's access-grant
mechanism breaks that: encrypted details require the subject's
key. A mandate would have to compel subjects individually,
which is a much harder political project than mandating
centralized bureaus.

### 4. Multiple independent verifiers

A government requiring its own verifier to be used is met
with subject choice: subjects can use multiple verifiers,
and lenders can accept whichever ones they trust. No monopoly
on identity.

### 5. Political speech does not touch `credit.*`

The design domain is financial-creditworthiness. A speech act
on social media is not a `credit.*` event. Even if a state
tried to shoehorn political behavior into credit domains (via
a new "social-behavior" attester), lenders (and subjects) can
simply refuse to honor that attester's signatures.

### 6. Voluntary participation

Nothing in the protocol forces a subject to register. A person
can go through life without a Subject Quid. They simply can't
get credit from lenders that require one. Compare to current
social-credit systems where participation is mandatory.

### 7. No chain-level consequence enforcement

The protocol records signed events and trust edges. It does not
enforce consequences (e.g., "this person can't buy train
tickets"). Any consequence is enforced by individual
counterparties, who choose whether to honor a given endorser's
judgment. A coordinated exclusion attempt requires many
independent actors to cooperate — which gives dissenters room
to find refuge in non-cooperating lenders / markets.

---

## Data sovereignty and portability

### The subject's quid is theirs

A subject migrating to a different country takes their quid
with them. Their signed history is portable; foreign lenders
can evaluate (weighting their trust in the attesters, which
may be lower for foreign ones, but the data exists).

### Guardian recovery

A subject who loses their device / private key uses guardian
recovery (QDP-0002) to re-establish. Their history remains
intact; only the key changes.

### Right to be forgotten

A subject can:
- Rotate their quid (old history now associated with a
  retired identity).
- Revoke future access grants (existing cached data is out
  of their control but no new queries).
- Require lenders to emit a `credit.right-to-delete`
  acknowledgment in jurisdictions where GDPR / CCPA apply.

The design doesn't prevent regulations from being
enforceable; it makes them enforceable cryptographically
(lenders' compliance is visible on-chain).

---

## Key Quidnug features used

- **Bring-your-own-quid** — subject owns identity.
- **Event streams** — credit history on subject's stream.
- **Trust edges with `validUntil`** — lender endorsements expire.
- **Per-domain trust** — credit type separation.
- **Relational trust computation** — per-lender evaluation.
- **Push gossip (QDP-0005)** — fast propagation of new events /
  edges / disputes.
- **Guardian recovery (QDP-0002)** — subject key-loss recovery.
- **GuardianResignation (QDP-0006)** — attesters can resign
  roles.
- **Encrypted payloads + IPFS** — privacy on-chain metadata,
  off-chain details.
- **Selective disclosure via access-grant events** — subject
  controls who reads their full history.

---

## Value delivered

| Dimension                            | Credit bureaus today                             | Quidnug                                                 |
|--------------------------------------|--------------------------------------------------|---------------------------------------------------------|
| Data ownership                       | Bureau                                           | Subject                                                  |
| Error correction time                | 30–180 days                                      | Immediate (dispute event)                                |
| Score customization per lender        | All use same bureau                              | Each lender runs own formula                             |
| Alternative-data inclusion            | Limited, opaque                                  | Native first-class                                       |
| Breach blast radius                  | Catastrophic (147M at Equifax)                   | Distributed + encrypted                                   |
| Portability across borders           | None                                             | Quid travels with you                                     |
| Transparency of "score"              | Proprietary                                      | Every evaluator's algorithm is their own, reproducible    |
| Dispute leverage                     | Bureau mediates (slowly)                         | On-chain dispute visible to all future lenders           |
| Thin-file problem                    | Excluded                                         | Alt-data + guarantors accessible                          |
| Consent to being scored              | Implicit / forced                                | Explicit via BYOQ                                        |
| Social-credit risk                   | Low but concerning                               | Structurally blocked by design                            |
| Historical completeness              | Gaps common                                      | Subject-verified, lender-signed                           |

---

## What's in this folder

- [`README.md`](README.md) — this document (comprehensive design)
- [`architecture.md`](architecture.md) — data model, domain hierarchy, privacy mechanism
- [`implementation.md`](implementation.md) — concrete flows per role
- [`threat-model.md`](threat-model.md) — attackers, threats, mitigations

## Related

- [`../elections/`](../elections/) — similar BYOQ + cryptographic-privacy pattern
- [`../interbank-wire-authorization/`](../interbank-wire-authorization/) — institutional-lender threat model
- [`../merchant-fraud-consortium/`](../merchant-fraud-consortium/) — similar consortium-trust pattern
- [QDP-0002 Guardian Recovery](../../docs/design/0002-guardian-based-recovery.md)
- [QDP-0005 Push Gossip](../../docs/design/0005-push-based-gossip.md)

## Comparison with prior decentralized-credit attempts

| Attempt                        | Design differences                                            |
|--------------------------------|---------------------------------------------------------------|
| Blockchain credit scoring (various) | Usually produce a single number (just moving the bureau on-chain). Quidnug refuses to produce a universal score. |
| Self-sovereign identity (SSI)  | Credential-based ("you have a degree"); doesn't model credit history and payment events as first-class primitives. Quidnug does. |
| DeFi credit (Aave, Compound)   | Over-collateralized — doesn't solve the uncollateralized trust problem. Quidnug models actual repayment history. |
| Credit unions                  | Closer in spirit (member-owned) but still centrally scored. Quidnug removes even the member-owned central scorer. |

Quidnug differs from all of these by combining: (a) relational
trust (no universal score), (b) rich signed event streams
(detailed payment history), (c) per-domain scoping, (d)
subject-controlled privacy, (e) guardian-recoverable identity.
No existing system has all five.
