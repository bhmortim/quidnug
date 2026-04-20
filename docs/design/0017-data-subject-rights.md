# QDP-0017: Data Subject Rights & Privacy

| Field      | Value                                                            |
|------------|------------------------------------------------------------------|
| Status     | Draft — design only                                              |
| Track      | Protocol + ops + legal                                           |
| Author     | The Quidnug Authors                                              |
| Created    | 2026-04-20                                                       |
| Requires   | QDP-0001, QDP-0002, QDP-0013, QDP-0014, QDP-0015                 |
| Implements | GDPR / CCPA / LGPD data-subject-rights workflow                  |

## 1. Summary

Privacy law obligates an operator to honor certain rights on
behalf of users whose personal data the operator processes:

- **Right to access** — give me everything you hold about me
- **Right to rectification** — correct inaccurate data
- **Right to erasure** — delete my data (with carve-outs)
- **Right to restrict processing** — don't use my data for X
- **Right to data portability** — export my data in a
  machine-readable format
- **Right to object** — stop processing on specific grounds

Under GDPR, CCPA, LGPD, PIPEDA, and similar regimes the
operator has 30-45 days to respond. Failing to respond carries
penalties up to 4% of global annual revenue under GDPR.

The tension is familiar: Quidnug is append-only; the chain
can't unilaterally delete records. QDP-0015 gave operators a
"suppress from serving" primitive; QDP-0017 extends that with:

1. A clear conceptual model of who the "data subject" is and
   what data "about them" means.
2. Protocol-level primitives for each right:
   - `ACCESS_REQUEST` + operator-generated export
   - `RECTIFICATION` via anchor rotation + pointer update
   - `ERASURE` built on QDP-0015 suppress + cryptographic
     shredding via CID
   - `CONSENT` transactions for opt-in data processing
   - `DATA_PORTABILITY_MANIFEST` signed export format
3. Operator obligations + documented workflow for each right.
4. A taxonomy of "what is personal data" in a Quidnug context —
   non-obvious in a system where the primary data unit is a
   cryptographic identity.

## 2. Goals and non-goals

**Goals:**

- A defensible compliance posture for operators in EU / CA / BR
  / CA(nada) / similar jurisdictions.
- Clear operator-to-user workflow for each right.
- On-chain signed records of every data-rights action
  (for audit + dispute).
- Cryptographic guarantees where possible (signed consent
  records; cryptographic shredding where achievable).
- Privacy-by-default recommendations in operator config.

**Non-goals:**

- Full legal indemnification. This QDP documents the protocol
  affordances; operators still need legal counsel.
- Mathematically-complete anonymization. Full anonymity on an
  append-only signed chain is impossible by construction; we
  aim for pseudonymity-by-default plus pragmatic erasure.
- Zero-knowledge-proof primitives for privacy-preserving
  queries. That's a separate future QDP (research-grade).
- Prevention of correlation attacks on the trust graph.
  Operators warn users; users who need stronger protection use
  multiple quids.

## 3. What counts as "personal data" in Quidnug

GDPR defines personal data as "any information relating to an
identified or identifiable natural person." In a Quidnug
network that includes:

### 3.1 Per-quid

| Data | Personal? | Notes |
|---|---|---|
| `QuidID` (sha256(pubkey)[:16]) | **Yes, if linkable** | Pseudonymous alone; personal data if linked to a real identity via OIDC / email / phone |
| `PublicKey` | Yes, same reasoning | |
| `IdentityTransaction.Name` | Yes (usually) | Free-form; often the user's real name |
| `IdentityTransaction.Attributes` | Yes (depends on content) | User-defined; may contain PII |
| `IdentityTransaction.HomeDomain` | Typically not | Infrastructure metadata |

### 3.2 Per-event

| Data | Personal? | Notes |
|---|---|---|
| `EventTransaction.Creator` | Yes (via quid link) | |
| `EventTransaction.Payload` | Depends | Often has user-generated text |
| `EventTransaction.Timestamp` | Usually not alone | Can be correlated for profiling |

### 3.3 Derived / inferred

| Data | Personal? | Notes |
|---|---|---|
| Trust graph position | Yes | Reveals social relationships |
| Reputation scores | Yes | Behavioral profile |
| Review history | Yes | Often reveals location, preferences, beliefs |
| Reading patterns | Yes | If tracked server-side |

### 3.4 Operator-held logs

| Data | Personal? | Notes |
|---|---|---|
| IP addresses in server logs | Yes under GDPR | Retention limits apply |
| Session tokens | Yes if linkable | |
| Moderation audit trails referencing a user | Yes | |

### 3.5 What the protocol treats as "fully anonymous"

- An unlinked quid (no OIDC binding, no known-identity trust
  edges from named operators, no free-form PII in attributes)
  is **pseudonymous**, not anonymous. A determined adversary
  with access to logs might still deanonymize via traffic
  analysis.
- For true anonymity, users should:
  - Generate a fresh quid per context where deanonymization
    would harm them
  - Use a VPN / Tor when publishing
  - Avoid including PII in payloads
  - Not use the OIDC bridge

The protocol documents this; it doesn't enforce it.

## 4. The six rights in protocol terms

### 4.1 Right to access

**What the user asks:** "Give me everything you hold about me."

**Protocol flow:**

1. User sends a signed `DATA_SUBJECT_REQUEST` transaction with
   `RequestType: "ACCESS"` and their quid id.
2. The operator's node verifies the signature (proving the
   requester controls the quid).
3. Within 30 days, operator generates a
   `DATA_ACCESS_MANIFEST` — a structured JSON document
   containing:
   - All signed transactions by the subject quid
   - All events referencing the subject quid as
     `SubjectID`
   - All trust edges where subject is truster or trustee
   - All moderation actions targeting the subject
   - Server-side logs (IP history, session metadata) under the
     operator's retention policy
4. Operator signs the manifest with their operator key and
   sends to subject (directly, not on-chain).

The manifest is itself kept signed on-chain as proof of
compliance (redacted if necessary).

### 4.2 Right to rectification

**What the user asks:** "Correct this inaccurate data."

In an append-only chain, corrections are new records, not
mutations. The protocol flow:

1. User identifies the incorrect IdentityTransaction or
   IdentityAttribute.
2. User publishes a new `IdentityTransaction` with updated
   fields + strictly-higher `UpdateNonce` + a signed link back
   to the prior tx:
   ```
   supersedesTxID: <old-tx-id>
   ```
3. Serving-layer recognizes the supersede link and:
   - Returns the new tx when queries ask for "current identity"
   - Returns the old tx + a `X-Superseded-By: <new-tx-id>`
     header when queries ask for specific historical records
   - Updates identity-index views to reflect the new state

For rectification of an *event*'s content (e.g., a factual error
in a review), the user supersedes via a new event with a
`RectifiesTxID` field and an `EFFECTIVE_FROM: <new-tx-timestamp>`
marker. Read queries prefer the rectification.

### 4.3 Right to erasure (GDPR Art. 17)

**What the user asks:** "Forget me."

The strictest. Most relevant protocol flow:

1. User sends `DATA_SUBJECT_REQUEST` with `RequestType: "ERASURE"`.
2. Operator evaluates whether carve-outs apply (see §4.3.1).
3. If granted, operator issues:
   - A `MODERATION_ACTION` with scope `suppress` and reason
     `GDPR_ERASURE` targeting the subject's quid.
   - If subject's events are stored inline (small payloads),
     all subject-authored events are included in the suppress
     list.
   - If payloads are stored via CID, operator unpins the CIDs.
     Federated operators get a signed request to unpin too.
4. A signed `ERASURE_CONFIRMATION` tx is published, attesting
   to what was suppressed + unpinned.

**Post-erasure state:**

- Chain still contains the signed transactions (including
  suppressed ones) — required for chain consistency.
- HTTP API returns 451 Unavailable For Legal Reasons when
  queried for the subject's data.
- Gossip layer does not propagate the subject's data.
- Trust graph computations treat the subject's edges as
  nonexistent.
- Payload content (for CID-stored events) is unreachable from
  any cooperating operator's IPFS.

#### 4.3.1 Carve-outs from erasure

Art. 17(3) exempts processing necessary for:

- Compliance with a legal obligation
- Performance of a contract with the subject
- Establishment, exercise, or defense of legal claims
- Public interest / official authority
- Archiving, research, statistical purposes

For a review site, relevant carve-outs include:

- Retaining dispute-resolution evidence when a reviewer's
  content is being contested
- Retaining CSAM reports (legal obligation)
- Retaining payment/transaction records (tax compliance)

The protocol makes the carve-out explicit: a
`MODERATION_ACTION` with reason `GDPR_ERASURE` can be paired
with an additional `erasureCarveOutReason` field explaining
why certain records are retained.

### 4.4 Right to restrict processing

**What the user asks:** "Don't use my data for X, but don't
delete it either."

Protocol flow:

1. User publishes a `PROCESSING_RESTRICTION` transaction:
   ```json
   {
       "type": "PROCESSING_RESTRICTION",
       "subjectQuid": "<my-quid>",
       "restrictedUses": ["reputation-computation", "recommendation-aggregation"],
       "nonce": 1
   }
   ```
2. The operator's node honors the restriction in the relevant
   computation paths:
   - Trust calculations exclude this quid from transitive paths
     if `reputation-computation` is restricted.
   - Recommendation / discovery APIs skip the quid if
     `recommendation-aggregation` is restricted.
3. The chain still contains the user's transactions for audit
   purposes, but operator-level processing is gated.

### 4.5 Right to data portability

**What the user asks:** "Give me my data in a format I can take
elsewhere."

Flow:

1. User sends `DATA_SUBJECT_REQUEST` with
   `RequestType: "PORTABILITY"`.
2. Operator generates a `DATA_PORTABILITY_MANIFEST` — a signed
   tarball/zip containing:
   - All signed transactions by subject (same as access)
   - `schema.json` describing the structure
   - Instructions for importing into any Quidnug node
3. Signed with operator key. Delivered to subject via secure
   channel.

Because signatures verify independently, a Quidnug export is
inherently portable — the user can import these transactions
into any other network's node and they'll validate.

### 4.6 Right to object

**What the user asks:** "Stop the specific processing I find
objectionable."

Covered by §4.4 (restrict processing). The `restrictedUses`
enum covers the objection grounds; the operator honors them.

## 5. Consent as a first-class primitive

### 5.1 `CONSENT_GRANT` transaction

For operators that process data on the basis of consent (GDPR
Art. 6(1)(a)), consent must be:

- Freely given
- Specific
- Informed
- Unambiguous
- Revocable

Protocol support:

```go
type ConsentGrantTransaction struct {
    BaseTransaction
    SubjectQuid     string   `json:"subjectQuid"`    // the data subject
    ControllerQuid  string   `json:"controllerQuid"` // the operator getting consent
    Scope           []string `json:"scope"`          // enum list (see §5.2)
    PolicyURL       string   `json:"policyUrl"`      // link to the policy being consented to
    PolicyHash      string   `json:"policyHash"`     // sha256 of policy at grant time
    EffectiveUntil  int64    `json:"effectiveUntil,omitempty"` // optional expiry
    Nonce           int64    `json:"nonce"`
}
```

Signed by the subject. Creates an on-chain record of consent at
a specific point in time against a specific policy version.
`PolicyHash` prevents later policy modifications from changing
what the user agreed to without their knowledge.

### 5.2 Consent scope enum

Stable enum of processing categories:

- `PROFILE_BUILDING` — operator builds a behavioral profile
- `RECOMMENDATION_COMPUTATION` — operator aggregates for recs
- `THIRD_PARTY_ANALYTICS` — share with external analytics
- `MARKETING_EMAIL` — direct marketing
- `FEDERATION_EXPORT` — export to federated networks
- `AI_TRAINING` — use subject's content for model training
- `CUSTOM:<operator-specific>` — reserved prefix for operator-specific scopes

### 5.3 `CONSENT_WITHDRAW` transaction

Revokes a prior consent. Mandatory per GDPR.

```go
type ConsentWithdrawTransaction struct {
    BaseTransaction
    SubjectQuid        string `json:"subjectQuid"`
    WithdrawsGrantTxID string `json:"withdrawsGrantTxId"`
    Reason             string `json:"reason,omitempty"`
    Nonce              int64  `json:"nonce"`
}
```

Serving-layer honors withdrawal immediately: new data is not
processed under the withdrawn consent; past data may or may
not be retroactively deleted depending on the carve-outs
(§4.3.1).

### 5.4 Audit

Any user can query their on-chain consent history:

```
GET /api/v2/consent/history?subject=<quid>
```

Returns a signed list of all CONSENT_GRANT / CONSENT_WITHDRAW
transactions for that subject. Users can verify what they've
agreed to; regulators can verify operator compliance.

## 6. Pseudonymity-by-default configuration

A new configuration section for operators running
privacy-focused nodes:

```yaml
privacy_defaults:
    require_pii_by_cid: true           # inline PII payloads rejected
    default_event_retention: "3y"      # after which, auto-suppression unless retained
    auto_rotate_keys: "180d"           # encourage key rotation
    warn_cross_quid_correlation: true  # detect attempts at identity linking
    deanonymization_risk_indicators:
        - "email-pattern-in-payload"
        - "phone-pattern-in-payload"
        - "geolocation-in-payload"
    ip_log_retention: "30d"            # shortest defensible retention
    require_tls_client_cert: false
```

Public-network operators pick sensible defaults; private
networks tune to their compliance posture.

## 7. The ten-minute data subject request workflow

For operators to handle an inbound DSR quickly:

### 7.1 Inbound

User sends an email / form submission or a signed
`DATA_SUBJECT_REQUEST` transaction to the operator's intake
endpoint:

```
POST /api/v2/privacy/dsr
```

with body:

```json
{
    "subjectQuid": "<quid>",
    "requestType": "ACCESS | RECTIFICATION | ERASURE | RESTRICTION | PORTABILITY | OBJECTION",
    "requestDetails": "free-form text",
    "contactEmail": "user@example.com"
}
```

Signed by the subject.

### 7.2 Verification

Node auto-verifies the signature proves quid control. No
further identity verification is typically needed — the
cryptographic proof is strong. Operators may optionally add
an email confirmation step.

### 7.3 Operator action

For each request type, the operator runs a CLI command:

```bash
quidnug-cli privacy fulfill \
    --request <request-id> \
    --action ACCESS \
    --out user-data.json
```

Or the auto-fulfill handler:

```bash
quidnug-cli privacy auto-fulfill \
    --request <request-id> \
    --policy-check
```

`auto-fulfill` runs the operator's pre-configured policy: for
simple cases (subject asks for their own data; no complications)
it generates the manifest and sends the email automatically.
For edge cases (rectification affecting other users; erasure
with potential carve-outs) it flags for operator review.

### 7.4 Publish compliance record

Every DSR fulfillment emits a signed `DSR_COMPLIANCE` event:

```json
{
    "requestTxId": "<original-request-hash>",
    "requestType": "ACCESS",
    "completedAt": 1776710000,
    "actionsCategory": "manifest-generated",
    "carveOutsApplied": [],
    "operatorQuid": "<operator-quid>",
    "signature": "..."
}
```

This is public-by-default (user-identifying fields omitted).
Operators publish an aggregate transparency report quarterly
(matches QDP-0015 reporting).

## 8. Attack vectors and mitigations

### 8.1 Forged DSR

**Attack:** Attacker submits a DSR in victim's name to get
their data.

**Mitigation:** DSR must be signed by the subject's quid. An
attacker without the private key can't produce a valid
request. Social-engineering attacks (impersonation via email)
fail the signature check automatically.

### 8.2 Erasure abuse

**Attack:** User asks for erasure, then later reappears with
fresh identity to publish the "same" content with a
different legitimacy claim.

**Mitigation:** Nothing stops a user from creating a new quid
and re-publishing. That's how the real-world right to erasure
works too — you can delete your data and change your mind. The
operator's policy may refuse to service a second erasure request
from a new quid that's clearly linked to a previously-erased one.

### 8.3 Bulk DSR flood

**Attack:** Attacker generates fake DSRs to overwhelm
operator.

**Mitigation:** Per QDP-0016 rate limits apply. DSRs require
signed quid identity; rate-limited per-quid. Further: the
auto-fulfill workflow is cheap (most DSRs are standard ACCESS
requests), so even a real flood is manageable.

### 8.4 Consent-withdrawal timing

**Attack:** User withdraws consent immediately after receiving
benefit, trying to retain benefit while revoking permission.

**Mitigation:** Consent is revocable; past processing under
valid consent remains lawful. The operator's terms of service
should make this clear. For irreversible benefits (pay-per-use),
the contract basis for processing applies, not consent.

### 8.5 Retention-policy circumvention

**Attack:** A federation partner network retains data past the
original operator's retention window.

**Mitigation:** Federation is by explicit trust. If the partner
doesn't honor data-subject rights, the original operator
shouldn't federate with them. Federated networks are expected
to have their own DSR compliance; the protocol doesn't enforce
this technically but documents it operationally.

### 8.6 Correlation attacks

**Attack:** An analyst correlates "anonymous" quid activity
with external data (timing, writing style, payment records) to
re-identify a user.

**Mitigation:** The protocol can't prevent this. Users who need
strong privacy should:

- Use multiple quids, one per context
- Avoid including PII in payloads
- Use Tor/VPN to avoid IP correlation
- Generate fresh quids periodically

The pseudonymity-by-default config (§6) surfaces these
recommendations to users who ask.

## 9. Jurisdiction-specific notes

### 9.1 GDPR (EU + UK)

Full coverage of Arts. 15-22. 30-day response window
(extensible to 90 days in complex cases). Operator must
appoint a Data Protection Officer if processing high-risk data.
QDP-0017's DSR workflow satisfies the procedural requirements;
operator must still provide substantive responses.

### 9.2 CCPA / CPRA (California)

Subject is "consumer." Rights are similar to GDPR but with
shorter timelines (15 days to confirm receipt, 45 days to
respond). "Right to know categories" is trivially satisfied by
the protocol's inherent auditability. "Right to opt-out of
sale" — the protocol has no concept of selling data directly;
operators must document their federation-export practices
separately.

### 9.3 LGPD (Brazil)

Similar to GDPR. Portuguese-language intake advisable for
Brazilian users.

### 9.4 PIPEDA (Canada)

Organization-level consent can be implicit for reasonable
processing. Explicit consent (§5) is best practice anyway.

### 9.5 Children's data

COPPA (US, under 13), GDPR (under 16, with member-state
variation), and similar laws impose stricter requirements for
minors. Operators accepting child users must:

- Verify age at quid creation (OIDC bridge with age-verified
  sources)
- Require parental consent
- Apply enhanced retention limits
- Not share across federation without specific consent

The protocol doesn't encode this; operator policy enforces.

## 10. Implementation plan

### Phase 1: Transaction types + validation

- `DATA_SUBJECT_REQUEST`, `CONSENT_GRANT`, `CONSENT_WITHDRAW`,
  `PROCESSING_RESTRICTION`, `DSR_COMPLIANCE` transaction types.
- Validators for each.
- Registry updates (consent ledger, restriction ledger,
  DSR-status registry).

Effort: ~1.5 person-weeks.

### Phase 2: Operator handlers + CLI

- `POST /api/v2/privacy/dsr` intake.
- `GET /api/v2/privacy/dsr/{id}` status lookup.
- `GET /api/v2/consent/history?subject=...` — user-accessible.
- `quidnug-cli privacy fulfill` / `auto-fulfill` commands.

Effort: ~1 person-week.

### Phase 3: Data-generation for rights

- Access / portability manifest generators.
- Erasure helpers (interacts with QDP-0015 moderation +
  IPFS unpin).
- Restriction enforcement in trust / discovery / event paths.

Effort: ~1.5 person-weeks.

### Phase 4: Operator documentation + playbook

- `docs/privacy/` directory:
  - `privacy-operator-playbook.md`
  - `gdpr-compliance-checklist.md`
  - `dpo-checklist.md` (for operators who need a Data Protection Officer)
  - `retention-policy-template.md`

Effort: ~5 days (operational docs, mostly writing).

### Phase 5: Transparency reporting

- Auto-generated quarterly DSR statistics.
- Public `quidnug.com/network/transparency/privacy` page.
- Machine-readable JSON feed for auditors.

Effort: ~3 days.

## 11. Open questions

1. **Should DSR fulfillments be publicly-auditable?** Aggregate
   yes; individual no. Current design publishes aggregated
   stats + a hash of each fulfillment; full records are
   operator-local.

2. **Federated cross-operator DSR.** If a user has data across
   multiple operators, must they DSR each one separately? Yes
   for the 2026-stage release; a future "federated DSR
   broker" could consolidate.

3. **Encrypted-consent private log.** Some regulated operators
   want their consent ledger private (only regulators can
   decrypt). Out of scope for this QDP; requires a separate
   encryption primitive.

4. **Automated PII detection.** The `require_pii_by_cid`
   config needs a reasonable PII classifier (email + phone
   + SSN + credit-card regex). Phase 1 ships with basic
   patterns; a follow-up can integrate a proper library.

5. **Age-gated access to the protocol itself.** Should new
   nodes enforce minimum-age checks? Probably no — that's a
   centralized constraint. Operators who serve minors wrap
   the protocol with their own age checks.

## 12. Review status

Draft. Like QDP-0015, the hardest questions are legal not
protocol:

- Counsel review of the erasure-via-suppress-and-unpin model
  vs GDPR Art. 17 actual-deletion requirements. Most EU
  regulators have accepted the "reasonable technical measures"
  interpretation for distributed systems.
- DPIA (Data Protection Impact Assessment) template for
  operators considering launching in the EU.
- CCPA-style "do not sell" language — protocol doesn't sell,
  but federation-export could be characterized as sale if
  money changes hands. Operators should document federation
  terms explicitly.

Implementation sequencing: Phases 1-3 are mandatory for any
EU-facing launch. Phases 4-5 are operational maturity.

## 13. References

- [GDPR (Regulation 2016/679)](https://gdpr-info.eu/)
- [CCPA / CPRA](https://oag.ca.gov/privacy/ccpa)
- [LGPD (Brazil)](https://lgpd-brazil.info/)
- [PIPEDA (Canada)](https://www.priv.gc.ca/en/privacy-topics/privacy-laws-in-canada/the-personal-information-protection-and-electronic-documents-act-pipeda/)
- [COPPA](https://www.ftc.gov/legal-library/browse/rules/childrens-online-privacy-protection-rule-coppa)
- [QDP-0015 (Content moderation)](0015-content-moderation.md) —
  the complement for adversarial content
- [QDP-0016 (Abuse prevention)](0016-abuse-prevention.md) —
  protects the DSR intake from flooding
- [QDP-0002 (Guardian recovery)](0002-guardian-based-recovery.md)
  — key-lifecycle support that matters for right-to-rectification
