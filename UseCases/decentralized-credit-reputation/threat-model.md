# Threat Model: Decentralized Credit & Reputation

## Assets

1. **Subject's credit history integrity** — the cryptographic
   record of their financial behavior.
2. **Subject's private key + encryption key** — controls
   identity and data access.
3. **Lender's signing authority** — attestations carry their
   reputation.
4. **Consumer financial autonomy** — ability to participate
   without coercion.
5. **Absence of centralized scoring** — the architectural
   property that prevents social-credit concentration.

## Attacker inventory

| Attacker                         | Capability                                       | Primary goal                                    |
|----------------------------------|--------------------------------------------------|-------------------------------------------------|
| Compromised subject key          | Subject's signing key                            | Open fraudulent credit in subject's name        |
| Identity thief                   | SSN / ID + ability to create fake quid           | Open credit accounts impersonating subject      |
| Collusive lender ring            | Real lenders, cooperative fraud                  | Fake credit histories                           |
| Defamatory lender                | Real lender, wants to harm borrower              | File false defaults                             |
| Shadow-bureau aggregator         | Public chain observer + ML                        | Rebuild a central score from public metadata    |
| State actor — social credit      | Legal/political authority                         | Impose universal citizen score                  |
| Employer                         | Has leverage over subject                         | Compel subject to reveal full history           |
| Coercer / abuser                 | Personal relationship with subject                | Compel subject to share access grants           |
| Compromised verifier             | Valid identity-verifier key                      | Enable identity theft at scale                  |
| Breach of lender's DB            | Off-chain data                                   | Disclose details despite encryption             |
| Lender employees                 | Legitimate read access to customer data          | Exfiltrate for profit                           |

## Threats and mitigations

### Category A: Identity and impersonation

#### A1. Stolen subject quid private key

**Attack.** Attacker obtains the subject's private key and
opens credit in their name.

**Mitigations.**

- **Lender identity verification** — lenders accept new
  applications only for subjects with trust edges from
  verifiers the lender trusts (DMV, bank-KYC). Attacker with
  stolen key but no matching verifier endorsement fails.
- **Multi-factor at origination** — lender's off-chain flow
  (call, email confirm, biometric) bounds the damage.
- **Subject monitoring** — subject sees any new credit events
  on their stream. Push gossip propagates within seconds; a
  watching wallet app alerts immediately. Subject can
  rotate via guardian recovery within hours.
- **Guardian recovery** — subject rotates their quid to a
  new key; old key's future signatures are invalid. Lender
  accepts the new epoch for new applications.

**Residual risk.** Small window between key theft and subject
detection. Bounded by monitoring diligence; mitigated by
standard fraud-detection overlays (same as current systems).

#### A2. Synthetic identity — attacker creates fake quid + impersonates real person

**Attack.** Attacker generates a fresh quid and tries to
establish an identity as if they were some real person.

**Mitigations.**

- Verifier trust edges are the gate. Identity verifiers
  (DMV, KYC providers) do their existing real-world
  verification. Attacker can't synthesize a legit verifier
  endorsement without passing their checks.
- Cross-verifier consistency: multiple verifiers should have
  consistent views. A synthetic identity verified by one
  verifier but not any other is suspicious.
- Existing ID-verification industry continues to operate;
  Quidnug makes their endorsements cryptographically
  portable.

### Category B: Lender misconduct

#### B1. False-default defamation

**Attack.** Lender files a late-payment or default event that
the subject disputes as false.

**Mitigations.**

- **Dispute mechanism** — subject's counter-event is visible
  alongside the lender's claim. Future evaluators see both.
- **Arbiter opinions** — independent arbiter can weigh in.
  Lenders with multiple arbiter-rejected claims look bad.
- **Cross-lender trust degradation** — lenders who observe
  another lender consistently over-reporting can lower their
  inter-lender trust edge, reducing that lender's attestation
  weight.
- **Regulatory enforcement** — FCRA-equivalent laws still
  apply, now with cryptographic evidence trail.
- **Subject narrative control** — dispute events let subject
  document the context; future evaluators judge fairly.

**Residual risk.** Sophisticated lenders can file well-crafted
false claims that look legitimate. Mitigated by: aggregate
pattern analysis, arbiter ecosystems, regulator oversight.

#### B2. Hostage-taking — lender refuses to emit payoff event

**Attack.** Subject has paid off their loan but lender delays
or refuses to emit the `paid-off` event, leaving the loan
appearing open.

**Mitigations.**

- **Subject-initiated correction dispute** — subject emits
  `credit.dispute.opened` with evidence of final payment.
  Future evaluators see the dispute.
- **Arbiter engagement** — independent arbiter can verify and
  issue an opinion.
- **Bank-regulator involvement** — regulatory systems still
  exist and can mandate lender compliance.
- **Switching lenders** — subject can route future business
  away from the hostile lender, and lenders that see this
  pattern downgrade their trust in the original lender.

#### B3. Lender collusion ring — fake endorsements

**Attack.** Lender A and Lender B (controlled by same fraudster)
issue fake trust edges back and forth endorsing fraudulent
subjects. The goal: manufacture creditworthiness so those
subjects can defraud legitimate Lender C.

**Mitigations.**

- **Inter-lender trust** — Lender C's evaluation of a
  subject's endorsers is based on C's own trust in the
  endorsers. Fake lenders with no legitimate banking activity
  have minimal inter-lender trust. Their endorsements carry
  little weight.
- **Regulatory registration** — real lenders have OCC / FDIC
  / regulatory numbers. Endorsements from unregistered "lenders"
  are filterable.
- **Pattern detection** — a ring of mutually-endorsing new
  entities without any external activity is a statistical
  anomaly. Research / consumer-protection orgs can publish
  warning events about such rings.
- **Regulator observation** — regulators have free public read
  access to the chain. Regulatory action possible.

**Residual risk.** Well-disguised collusion at scale is
possible but harder than gaming today's bureau-based system.

#### B4. Hidden rate discrimination

**Attack.** Lenders use trust evaluations to discriminate on
prohibited categories (race, age, etc.).

**Mitigations.**

- **Protected categories are not on-chain.** Quidnug chain
  doesn't store name, race, address, DOB. Discrimination based
  on these requires the lender accessing off-chain data — same
  as today.
- **Public pattern analysis** — regulators can observe
  approval patterns per-lender (approve/decline rates by
  inferable demographics). Same existing regulatory oversight
  mechanism.
- **Lender's algorithm is their own** — but the inputs are on
  the chain. Regulators can demand the algorithm and replay
  decisions to audit discrimination.

### Category C: Privacy and data-broker threats

#### C1. Shadow-bureau aggregator

**Attack.** Entity scrapes the public chain, builds a
database of subjects' coarse credit histories, sells it as a
"traditional credit report" service.

**Mitigations.**

- **Public chain contains only coarse metadata.** Exact
  amounts, rates, payment details are encrypted. Even with
  perfect scraping, the scraper gets "subject had a loan,
  type, approx size, on-time or not" — already comparable to
  what bureaus publish publicly.
- **Subject can use multiple quids** — a privacy-conscious
  subject can maintain separate quids for different credit
  relationships, making cross-relationship correlation
  harder.
- **Legal regime** — shadow bureaus compiling records without
  consumer consent fall under existing FCRA / GDPR
  frameworks.

**Residual risk.** A competent scraper can produce a "summary
report" service. Less detailed than current bureaus, but some
reconstruction is possible. Fundamental to any public-history
system; solution is stronger cryptographic privacy if needed
(ZK proofs; future QDP).

#### C2. Coerced access grants

**Attack.** Employer / abuser / landlord requires subject to
share full credit history as a condition.

**Mitigations.**

- **Partial access grants** — subject can share only specific
  scopes. Employer asking for "alt-data.rent" history doesn't
  get all loan data.
- **Time-bounded access** — 30-day access grant expires;
  coercer has to keep re-coercing.
- **Multiple quids** — subject can maintain a "professional
  quid" with limited history and a "full quid" kept private.
- **Legal protections** — jurisdictions that prohibit
  employer credit checks continue to do so; the protocol
  doesn't enable new coercion pathways that weren't possible
  before.

#### C3. Detail-blob leak at lender

**Attack.** Lender stores detail blob in plaintext on their
server; server is breached.

**Mitigations.**

- **Design requires lender keeps blob encrypted at rest.**
  Access grant provides a key; lender's servers should store
  the encrypted blob plus the key (or decrypt on-demand and
  discard).
- **Lender security practices continue to apply.** This is
  the same problem as any lender holding customer data.
- **Scope-limited grants** — lender only receives the detail
  for specific scopes they need. Leak exposes only granted
  scope.

### Category D: Social-credit concentration attempts

#### D1. State mandates a "national citizen trust score"

**Attack.** Government declares that all lenders must accept
endorsements from a state-operated scoring agency and weight
them heavily.

**Mitigations (structural / operational).**

- **Protocol refuses to produce a universal score.** State
  can create a quid and issue endorsements, but can't force
  private lenders to weight those endorsements.
- **Domain separation** — a "citizen-trust" domain created by
  the state is a new domain. Lenders opt in per jurisdiction
  and legal requirement. Voluntary adherence is the check.
- **Parallel markets emerge** — if state-score-accepting
  lenders become the only option, non-state-score lenders
  (in friendlier jurisdictions, or informal / alternative
  markets) become a pressure valve.
- **Civil-society resistance** — consumer-advocacy orgs publish
  trust edges warning subjects about state-score lenders.

**Residual risk.** In fully-authoritarian regimes, the
government can pass laws forcing participation. Quidnug
can't override law; it provides architectural defense for
everywhere the law doesn't reach.

#### D2. Political-behavior attestations

**Attack.** State or private actor publishes
`political-loyalty-score` trust edges and pressures lenders
to use them.

**Mitigations.**

- **Lenders can refuse** — there's no protocol requirement to
  honor any particular attester's edges.
- **Regulatory environment** — anti-discrimination law
  typically prohibits lending decisions based on political
  affiliation. Publishing an edge doesn't mean lenders can
  lawfully use it.
- **Subject awareness** — subjects can see what attesters are
  issuing about them and challenge via disputes.

#### D3. Cross-domain score aggregation

**Attack.** An aggregator takes trust edges from
`credit.mortgage.us`, `credit.auto-loan.us`, etc., and
publishes a composite "general creditworthiness" score.

**Mitigations.**

- **Lenders evaluate independently.** An aggregator's
  composite is meaningful only if lenders choose to use it —
  same pattern as the social-credit defense.
- **Competition in aggregation** — no monopoly; many
  possible aggregators. Subjects can choose which they want
  to be summarized by.
- **Transparency** — aggregator's formula is public (on-chain
  event schemas); lenders can audit it.

### Category E: Infrastructure attacks

#### E1. Network-level denial of service

**Attack.** DDoS on Quidnug nodes to prevent lenders or
subjects from querying.

**Mitigations.** Standard DDoS protections; distributed node
operation means multiple alternative nodes available.

#### E2. IPFS-dependency attacks

**Attack.** IPFS pinning service that holds detail blobs
goes offline or is compromised.

**Mitigations.**

- **Subject should pin their own detail blobs** (via local
  IPFS client or paid pinning).
- **Lender can also pin** (for their own access durability).
- **Alternative storage** — design is IPFS-friendly but
  not IPFS-required. S3, Filecoin, Arweave, or any content-
  addressable store works.

### Category F: Adversarial economics

#### F1. Lender refuses to operate at a loss

**Attack.** A bad actor (sovereign or private) gains
influence over many lenders; pressures them to refuse credit
to specific demographic or political class.

**Mitigations.**

- **Market diversity** — no single entity controls all
  lenders. Marginalized borrowers find alternative lenders.
- **Alternative-data-heavy lenders** — fintech lenders
  serving underbanked populations exist; relational trust
  gives them first-class visibility.
- **Community lending** — informal peer-to-peer lending has
  always existed; Quidnug gives it cryptographic tools.

#### F2. Predatory inclusion

**Attack.** Lender uses relational-trust data to identify
vulnerable subjects for predatory loans.

**Mitigations.**

- Regulatory frameworks against predatory lending continue
  to operate.
- Subject's dispute mechanism + visible pattern of lenders
  with predatory behavior.
- Consumer-advocate trust edges warning about specific
  lenders.

## Attack scenarios — end-to-end

### Scenario 1: "Attacker wants to sabotage subject's credit"

Required actions:
1. File a false-default event against subject.
2. Ensure subject can't dispute (?).
3. Cause future lenders to weigh the event heavily.

Defenses that defeat this:
- Subject sees the event via monitoring → files dispute.
- Dispute is visible to future lenders.
- Attacker-lender's own reputation degrades.
- Regulator/arbiter oversight.

Compared to current bureaus, the attacker needs to be an
endorsed lender to file events (regulatory barrier) AND
survive subject's dispute + arbiter scrutiny.

### Scenario 2: "Government wants to build a social-credit system"

Required actions:
1. Create a state-operated "citizen trust" issuer.
2. Compel private lenders to weight the state's endorsements.
3. Compel subjects to share full history.

Defenses:
- Private lenders' algorithms are their own; legal compulsion
  required and constrained by jurisdiction constitutions.
- Subject access grants are discretionary; compelling every
  subject is politically costly.
- International lenders operate in different jurisdictions.
- Civil society can publish counter-attestations.

Compared to a centralized government-operated database, the
protocol makes the authoritarian move much harder (requires
coercing many independent actors).

### Scenario 3: "Thieves steal subject's key via phishing"

Required actions:
1. Phish private key.
2. Apply for credit quickly before subject notices.

Defenses:
- Lender-side identity verification (second channel).
- Subject monitoring + push notifications.
- Guardian recovery within hours.
- Temporary suspension of new applications while subject
  investigates.

Comparable to current system's defenses against identity
theft; cryptographic guardians make recovery faster than
traditional bureau freeze / unfreeze cycles.

## Not defended against (explicit limits)

1. **Subject voluntarily shares all history under duress.**
   Legal regime remains the primary defense.
2. **Regulator collusion.** If all regulators coordinate to
   mandate state-score usage, protocol doesn't override.
3. **Fundamental privacy vs. verifiability trade-off.** The
   public metadata (event type, counterparty, coarse band) is
   inherent. Stronger privacy (ZK proofs) is future work.
4. **Lender's off-chain decision.** Lender can deny credit
   for any reason that isn't specifically illegal. Protocol
   supports the decision; doesn't mandate it.
5. **Full anonymization.** The protocol preserves
   pseudonymity (quids), not full anonymity. State-level
   adversaries with enough data can correlate.
6. **Irrational lenders.** A lender willing to lose money can
   ignore any trust signal and lend anyway. Market
   competition is the discipline.

## Monitoring

Public observability / metrics:

| Metric                                                  | Alert condition / purpose           |
|---------------------------------------------------------|-------------------------------------|
| Subject dispute rate per lender                         | > baseline: pattern of defamation   |
| Lender approval-vs-denial rate by inferable demographic | > threshold: possible discrimination|
| Trust edges issued between new quids                    | > threshold: possible collusion     |
| Arbiter opinions issued per lender per month            | > threshold: lender misconduct      |
| Subject-lender disputes unresolved past 90 days         | > threshold: lender compliance issue|
| Cross-lender trust degradation events                   | any                                 |

## Incident response

Playbooks (excerpts):

1. **Subject reports identity theft.**
   - Emit `subject.identity-compromised` event on subject's stream.
   - Guardian-recovery rotation of subject quid.
   - All pending applications halted at new-epoch.
   - Lender notifications via push gossip.

2. **Lender has pattern of disputed claims.**
   - Consumer-advocate org publishes public warning event.
   - Other lenders downgrade inter-lender trust edges.
   - Regulator can issue a formal enforcement event.

3. **Attempted state-credit-score rollout.**
   - Civil-society orgs publish counter-attestations.
   - Lenders in jurisdictions with legal protections refuse
     to weight it.
   - Subjects advised to use multi-jurisdictional quid
     strategies.

## References

- [QDP-0002 Guardian Recovery](../../docs/design/0002-guardian-based-recovery.md)
- [QDP-0005 Push Gossip](../../docs/design/0005-push-based-gossip.md)
- [QDP-0006 Guardian Resignation](../../docs/design/0006-guardian-resignation.md)
- [`../elections/threat-model.md`](../elections/threat-model.md)
  — related voter-coercion / state-capture analyses
- [`../merchant-fraud-consortium/`](../merchant-fraud-consortium/)
  — related cross-lender trust threat model
- FCRA (Fair Credit Reporting Act) — current regulatory
  baseline
- GDPR / CCPA — data-portability + deletion rights regime
