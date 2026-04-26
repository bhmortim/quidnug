# Threat model: trust-weighted reviews

> Catalog of attacks on trust-weighted reviews: the seven
> classic gaming patterns from existing systems plus
> Quidnug-specific adversaries. For each: mechanism, mitigation,
> residual risk. Companion to [`README.md`](README.md),
> [`architecture.md`](architecture.md), and
> [`implementation.md`](implementation.md).

## 1. Threat-model framing

Three adversary classes:

- **A1: Sybil-class spammer.** Cheap volume, no real-world
  identity. Buys aged accounts, rotates IPs, generates
  reviews with LLMs.
- **A2: Insider-class actor.** Legitimate participant who
  abuses position: a seller running review-trade rings, a
  validated site issuing fake PURCHASE attestations, a
  reviewer accepting payment without disclosure.
- **A3: Targeted attacker.** Capable, motivated. Goal-directed:
  destroy a specific competitor, suppress a specific reviewer,
  capture a specific topic's governance, compromise the
  network root.

Defenses must be evaluated against all three. A defense that
only stops A1 is not enough.

## 2. Attacks on the existing review market and how Quidnug differs

### T-1: Click-farm sybils (A1)

**Existing-system mechanism.** Buy aged accounts at $0.50-$3
each. Rotate residential proxies. Post 5-star reviews via
review-as-a-service shop.

**Quidnug-specific landing.** Same accounts can mint quids on
Quidnug for free (no entry cost on the protocol; the OIDC
bridge requires a Google/GitHub account but those are also
cheap on the gray market).

**Mitigation.**
- Per-observer weighting demotes them: a sybil quid has no
  trust edges from anyone the observer trusts, so its review
  weight is multiplied by ~zero in the algorithm.
- Anonymous-observer baseline (operator-root weighted) is also
  near-zero for new quids without OIDC verification, so even
  unauthenticated readers don't see meaningful uplift from
  sybils.
- DNS-anchored validation gives real reviewers a costly,
  persistent identity that sybils cannot afford to replicate
  at volume.

**Residual.** Sybils can still create noise visible to the
"raw event count" indicator. Observers must rely on weighted
ratings, not raw counts. UI design problem; addressed by
showing weighted-by-default with raw-count behind a click.

### T-2: Brushing (A2)

**Existing-system mechanism.** Seller ships empty packages to
fake addresses (or scraped real addresses); receiving "buyer"
posts verified-purchase 5-star review.

**Quidnug-specific.** PURCHASE events must be signed by the
seller's quid. A validated seller signing fraudulent
PURCHASEs at scale becomes detectable: the seller's
PURCHASE-to-review-conversion ratio is anomalously high; the
volume of single-buyer-single-purchase patterns is
detectable.

**Mitigation.**
- Validated sellers (Pro/Business) put their site domain at
  risk. Loss of validation = loss of seller's TRUST edge =
  PURCHASE attestations from them carry near-zero weight.
- Auditor / fraud-hunter role (T-15 below) can publish FLAG
  events against suspect seller quids; observers who trust
  the auditor inherit the flag.
- Statistical detection: brushing produces signature patterns
  (impossibly fast purchase-to-review conversion, repeated
  buyer addresses, single-purchase-per-buyer-quid). The
  network's append-only history makes these patterns
  impossible to hide.

**Residual.** Small-scale brushing by validated sellers who
discipline their volume can still slip through. Reduces the
ROI of brushing significantly without eliminating it.

### T-3: Review-trade networks (A2)

**Existing-system mechanism.** Closed groups (Facebook,
Telegram) where sellers swap 5-star reviews. Real humans, real
purchases, very hard to detect by content.

**Quidnug-specific.** Trade rings appear in the trust graph
as densely-mutually-connected clusters with little outside
connection. Graph analysis exposes them.

**Mitigation.**
- Per-observer weighting: trade-ring members have trust edges
  among themselves but no edges from observers outside the
  ring. Weights collapse for outside readers.
- Auditor role: clustering analysis publishes FLAG events
  against ring members.
- Topical scoping: a trade ring covering "tech.laptops" doesn't
  pollute "restaurants" or "books." Vertical isolation limits
  blast radius.

**Residual.** Cohorts of real-purchase-real-human reciprocal
reviewers are inherently legitimate-looking from inside; only
graph topology and behavioral patterns differentiate.
Effective defense requires active auditors; no purely
algorithmic defense suffices.

### T-4: Incentivized review schemes (A2)

**Existing-system mechanism.** Insert-card schemes ("review
us for a $20 gift card") inducing reviews outside the
platform's audit pipeline.

**Quidnug-specific.** Inducement is invisible to the protocol
unless disclosed. QRP-0001 includes a DISCLOSURE event for
voluntary disclosure ("this review was sponsored by ...").

**Mitigation.**
- Make undisclosed inducement a violation of QRP norms;
  observers can filter or down-weight reviews from quids that
  the auditor has flagged for undisclosed inducement.
- Brand-disclosure marketplace (see README §6) offers an
  on-protocol path for sponsored reviews that's better-priced
  and more credible than off-protocol bribery, undercutting
  the gray market.

**Residual.** Off-protocol inducement is undetectable from the
protocol alone; relies on whistleblower flagging and pattern
analysis.

### T-5: Brigading (A3)

**Existing-system mechanism.** Coordinated negative-review
attacks driven by politics, competition, viral grievance.
Weekend brigade can move 4.6 → 3.2.

**Quidnug-specific.** Brigaders are typically new or low-rep
quids with no trust edges from existing observer graphs.

**Mitigation.**
- Per-observer weighting: brigader weights collapse to near-
  zero from any observer with an established trust graph.
- "Weighted from people you trust" UI replacing "average
  rating" UI. The brigading attack is invisible to a
  weighted-rating reader.
- Anonymous-observer baseline uses operator-root weighting,
  so brigaders need a TRUST edge from operators to count;
  Quidnug LLC issues those only via OIDC bridge (level 0.2)
  or paid validation. Brigading via OIDC requires creating
  many Google/GitHub accounts (still possible but more
  expensive than today's brigading).

**Residual.** A determined attacker with budget for thousands
of OIDC-bridged identities can still produce visible noise in
raw counts; weighted ratings remain robust.

### T-6: Suppression (A3)

**Existing-system mechanism.** Brand sues reviewer for
defamation, files DMCA strike, or pressures platform to
de-rank negative review. Reviewer has no permanent record
because platform owns the only copy.

**Quidnug-specific.** Reviews are append-only, signed, and
hosted on the public network. No platform can delete a
review. Brand cannot pay anyone to bury it.

**Mitigation.**
- Operator-level non-gossip per QDP-0015: an operator who
  receives a valid takedown request (defamation ruling, GDPR
  erasure, DMCA) can choose to stop gossiping a specific
  event ID through their nodes. Other operators may continue
  serving.
- Federation discipline: an operator who systematically
  removes negative reviews of their advertisers loses
  validator trust from peer operators; their network
  influence shrinks.
- Reviewer migration: if Quidnug LLC is captured (e.g., bought
  by an entity that suppresses), reviewers can re-anchor at
  another federated operator without losing reputation.

**Residual.** Legal regimes (GDPR, defamation) require some
takedown capability; Quidnug provides operator-level
non-gossip but cannot prevent another operator from serving.
Publish a clear moderation policy at quidnug.com/policy
before launch.

### T-7: Generative-AI flooding (A1)

**Existing-system mechanism.** GPT-class models produce
reviews indistinguishable from human prose. Defense based on
"this reads like real text" is dead.

**Quidnug-specific.** AI-generated reviews require an
identity that has cost. Without DNS-anchored validation or
costly OIDC accounts, AI-generated reviews live in the
sybil-weight bucket (T-1).

**Mitigation.**
- Identity is the defense, not text-detection. Reviews from
  unvalidated, no-trust-edge quids carry near-zero weight
  regardless of how good the prose is.
- Validated reviewers (alice-eats.com Pro tier) staking their
  domain can still write AI-assisted reviews, but they put
  their identity at risk if caught.

**Residual.** Validated reviewers with poor judgment can use
AI to produce dishonest reviews under their real identity.
This is a reputation-management problem, addressed by the
auditor role and observer-driven trust adjustments, not by
the protocol.

## 3. Quidnug-specific attacks

### T-8: Validation-operator key compromise (A3)

**Mechanism.** Attacker steals the
`VALIDATION_OPERATOR_KEY` Wrangler secret. Can issue arbitrary
TRUST edges in `operators.<anything>.network.quidnug.com`.

**Mitigation.**
- The validation-operator quid is subordinate to the seed
  governor root: the seed quid issued the validation-operator
  its authority.
- On detection: governor root publishes
  `AnchorInvalidation` against the compromised
  validation-operator key, freezing its edges. New
  validation-operator quid generated and authorized.
- Customer-side impact is limited: existing TRUST edges from
  the compromised operator are also invalidated;
  re-validation runs automatically.

**Residual.** Window between compromise and detection.
Mitigated by short rotation cadence and key-usage anomaly
detection (rate of edge-issuance vs. customer signup rate).

### T-9: Seed-validator quorum capture (A3)

**Mechanism.** Attacker compromises both seed nodes
simultaneously, achieves 2-of-2 validator quorum, can produce
arbitrary blocks.

**Mitigation.**
- Two physically separate operators (home + VPS); compromise
  of one doesn't compromise the other.
- Guardian quorum on each seed key (3-of-5, 24h time-lock)
  prevents long-term capture: even if attacker gets both
  keys, seeds can be recovered through guardian process.
- Federation (QDP-0013): once 3+ operators federate, no
  single operator's compromise destroys the network.

**Residual.** Pre-federation single-operator phase has higher
risk. Mitigation: faster path to federation, conservative
governance, status-page disclosure if compromise suspected.

### T-10: Customer key theft (A3)

**Mechanism.** Reviewer's wallet key stolen. Attacker can
publish reviews and TRUST edges in reviewer's name.

**Mitigation.**
- Guardian recovery (QDP-0002): reviewer initiates
  GuardianRecoveryInit; guardians approve; key rotated;
  attacker's window is the time-lock period.
- Reviewer publishes `AnchorInvalidation` from the new key
  retroactively invalidating attacker-issued events.
- Observers who notice the attack can publish FLAG events
  immediately; the rating algorithm should de-weight events
  near a flagged time window.

**Residual.** Reviewer's reputation may take a hit during the
recovery window. Reviews issued by the attacker that match
the reviewer's normal pattern are hard to retroactively
distinguish.

### T-11: Domain hijack of validated site (A3)

**Mechanism.** Attacker compromises DNS for acmestore.com,
publishes their own challenge, gets validated as the "owner."

**Mitigation.**
- Validation Worker requires the DNS challenge to be at
  `_quidnug-challenge.<fqdn>` and the resolver quorum must
  match across 4 DoH providers. A DNS hijack via a single
  resolver doesn't pass quorum.
- Recheck cadence (15min Business, 1h Pro): if the legitimate
  owner regains DNS within the recheck window, the attacker's
  TRUST edge is auto-revoked.
- Domain-level anomaly: a domain with an existing TRUST edge
  that suddenly fails verification multiple times triggers
  manual review by Quidnug LLC ops.
- Customer can publish a manual revocation request via signed
  request from a backup channel (email + KYB-verified
  identity proof).

**Residual.** A registrar-level hijack (attacker controls
the registrar, redirects authoritative nameservers) can pass
DoH quorum. Mitigation requires monitoring DNS via additional
tooling (e.g., DNSSEC validation in the Worker) and customer
backup channels.

### T-12: Governance capture (A2/A3)

**Mechanism.** Quidnug LLC (or its corporate successor) is
acquired by a hostile entity that uses governor authority to
reshape the public review tree, suppress validators, or
issue self-serving TRUST edges.

**Mitigation.**
- Federation (QDP-0013): once multiple operators federate,
  any single operator's governance authority is
  geographically/legally distributed.
- Foundation transition: long-term, governance authority over
  the public review tree migrates to a 501(c)(6) trade
  association or comparable non-profit structure (year 2-3
  per the entity plan).
- Observer disintermediation: observers who don't trust the
  current governor set can ignore the operator root and rely
  purely on their direct trust graph; the algorithm degrades
  gracefully.

**Residual.** Acceptably low for a federated/foundation-
governed network; concerning for a single-operator network.
The pre-federation phase carries this risk.

### T-13: Brand-disclosure marketplace abuse (A2)

**Mechanism.** Brands offer payment for reviews via the
on-protocol marketplace; reviewers accept payment but submit
biased reviews disguised as honest. Or: disclosure is technically
present but buried in metadata.

**Mitigation.**
- DISCLOSURE events are first-class; widgets surface them
  prominently in the reviewer card and review body.
- Observers can filter or apply different weights to
  disclosed-sponsored reviews. Default rendering shows
  sponsored reviews separately from unsponsored.
- Reviewers caught accepting undisclosed payment lose their
  validation (PR/legal pressure on the validation operator
  to revoke); on-chain DISCLOSURE history is auditable.

**Residual.** Sophisticated bias (technically disclosed but
written to favor the sponsor anyway) is an editorial-judgment
problem, not a protocol problem. Auditor role + observer
judgment.

### T-14: Cross-network spoofing under federation (A3)

**Mechanism.** Under QDP-0013 federation, attacker stands up
a malicious "federated network" claiming compatibility with
the public Quidnug network, copies real reviews, and adds
malicious edges that propagate cross-network at the federation
discount.

**Mitigation.**
- Federation requires bilateral signed peering edges (QDP-0013).
  Quidnug LLC reviews each peering request manually; rejection
  reasons are documented in
  `deploy/public-network/rejection-reasons.md`.
- Cross-network discount (default 0.8) limits the damage from
  a malicious peer.
- A peer caught issuing malicious edges is defederated
  (TRUST level=0.0 published bilaterally); their cross-network
  edges collapse.

**Residual.** Window between malicious peer activity and
defederation; mitigated by active auditing of peer behavior.

### T-15: Auditor / fraud-hunter capture (A3)

**Mechanism.** Auditor role becomes powerful (their FLAGs
move ratings); attacker captures a prominent auditor either
by buying them or by compromising their key.

**Mitigation.**
- Multiple auditors competing in the same topic; observers
  trust a basket of auditors, not a single one. No auditor's
  compromise alone moves observer ratings.
- Auditors must validate their own domain (Pro/Business);
  their own credibility is at stake when they publish FLAGs.
- Auditor flags themselves are subject to FLAG events (meta-
  flagging); a captured auditor publishing bad-faith FLAGs
  can be flagged by other auditors, demoting their weight.

**Residual.** Single dominant auditor (Wirecutter-of-flagging)
is a single point of failure; market design should encourage
plurality. Foundations can fund multiple competing auditors to
prevent monopoly.

## 4. Defense-in-depth summary

The trust-weighted-review system has no single defense.
Layered properties combine:

| Property                   | Stops which attacks                |
|----------------------------|-----------------------------------|
| Per-observer weighting     | T-1, T-3, T-5, T-7                |
| Topical trust scoping      | T-3, T-13                         |
| Append-only signed events  | T-6 (suppression)                 |
| DNS-anchored validation    | T-1, T-2, T-7 (raises floor)      |
| Guardian recovery          | T-8, T-9, T-10                    |
| Subordinate operator quids | T-8                               |
| Resolver-quorum DoH probes | T-11                              |
| Federation                 | T-9, T-12                         |
| Auditor / FLAG mechanism   | T-2, T-3, T-13, T-15              |
| KYB attestation            | T-4, T-13                         |
| Operator non-gossip policy | T-6 (legal compliance)            |

Each layer alone is incomplete. The layered combination is
what makes the floor of credible fraud rise sharply against
the existing market.

## 5. Open questions and residual risk

Items not solved by this design, called out for future QDP
work:

1. **Anonymous reviews.** Some review categories (medical
   experiences, employer reviews) require pseudonymity for
   legitimate safety reasons. QDP-0021 (blind signatures) and
   QDP-0024 (group encryption) are paths but not yet active.

2. **Cross-jurisdictional content moderation.** A review legal
   in Texas may be defamatory in Germany. Operators in
   different legal regimes will disagree on what to gossip.
   QDP-0015 partially addresses but the policy boundaries are
   per-operator.

3. **Long-term reputation portability across federation
   schisms.** If two operator clusters defederate after a
   governance dispute, what happens to reviewer reputations
   that span both? Open per QDP-0013 §5.

4. **Quantum threat to ECDSA P-256.** Long-term: post-quantum
   crypto migration. Not specific to reviews; protocol-level
   concern.

5. **Network effects of single-operator phase.** The pre-
   federation phase concentrates governance risk. Mitigation
   is faster federation; risk is operational drag of getting
   second operator on-line.

These are called out so reviewers know what they're trusting
and observers know how much weight to assign.

## 6. Threat-model maintenance

This document is a living catalog. Add a new entry when:
- A new attack pattern is observed in the wild.
- A new defense is shipped that changes the residual risk
  on an existing attack.
- A new actor class emerges (e.g., regulator-as-adversary
  if a hostile government targets the network).

Updates are PR'd against this file; significant changes to
the threat model that affect protocol design should be
captured as their own QDP.
