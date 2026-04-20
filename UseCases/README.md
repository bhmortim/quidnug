# Quidnug Use Cases

Real-world, production-grade implementation designs for building
on Quidnug. Each folder is self-contained: problem statement,
architecture, implementation plan with concrete API calls, and
threat model.

## Start here

Two cross-cutting documents anyone building on Quidnug should
read first:

- **[`ARCHITECTURE.md`](ARCHITECTURE.md)** — the single-document
  tour of the protocol substrate, the three architectural
  pillars (QDPs 0012 / 0013 / 0014), and how every use case
  below fits the same underlying model. If you read one thing,
  read this.
- **[`BUILDING-A-USE-CASE.md`](BUILDING-A-USE-CASE.md)** — a
  concrete six-phase recipe for taking an idea to a shippable
  design document. Use this when you're ready to add your own.

## How to read the use-case folders

Each use case has four files:

| File | What it covers |
| --- | --- |
| `README.md` | Problem, audience, why-Quidnug mapping, high-level architecture |
| `architecture.md` | Data model, sequence diagrams, API usage patterns |
| `implementation.md` | Concrete code, integration steps |
| `threat-model.md` | What it defends against, known limits |

Read `README.md` first for the high-level design. Then
`architecture.md` + `implementation.md` if you want to build.
`threat-model.md` documents what the design defends against and
where its limits are.

Each use case is opinionated — it picks a specific way to wire
Quidnug's primitives together. You don't have to follow it
exactly; use it as a starting point and adapt to your
constraints.

---

## FinTech

Financial systems where multi-party approval, replay safety, key recovery,
and on-chain audit trails are load-bearing requirements.

### [`interbank-wire-authorization/`](interbank-wire-authorization/)

**Problem:** Wire transfers above a threshold need multi-party cosigning
across geographically-separated signers, with strong replay protection and
the ability to recover if a signer's HSM fails.

**Why Quidnug:** `GuardianSetUpdate` declares the M-of-N signer quorum
on-chain. Per-signer monotonic nonces prevent replay. Guardian recovery
handles HSM loss without emergency vendor tickets.

---

### [`merchant-fraud-consortium/`](merchant-fraud-consortium/)

**Problem:** Merchants want to share fraud signals (bad cards, mule
accounts, patterns) without trusting every other merchant equally. A
small merchant sharing a false positive shouldn't poison the whole
consortium.

**Why Quidnug:** Relational trust means merchant A's view of a fraud
signal is weighted by A's trust in the reporter, not a global score.
Domain-scoped trust lets banks and merchants participate under
different rules.

---

### [`defi-oracle-network/`](defi-oracle-network/)

**Problem:** DeFi protocols need off-chain data (prices, weather, events)
from independent providers without centralizing trust in a single oracle
operator or being vulnerable to a handful of colluders.

**Why Quidnug:** Oracle reporters publish signed feeds; consumers
compute relational trust in each reporter from their own perspective.
Aggregation is weighted per-consumer. K-of-K snapshot bootstrap seeds
fresh consumers with trusted signer sets.

---

### [`institutional-custody/`](institutional-custody/)

**Problem:** Crypto custody holding billion-dollar positions needs better
than "multi-sig spreadsheet." Keys get rotated quarterly, compromised,
inherited by successors, distributed across geographies and HSMs.

**Why Quidnug:** Full key lifecycle on-chain: Anchor-based rotation with
nonce caps, guardian-based recovery with time-lock, epoch-scoped validity
for audit, `EnableLazyEpochProbe` catches stale-key attacks across
subsidiaries.

---

### [`b2b-invoice-financing/`](b2b-invoice-financing/)

**Problem:** Invoice financing ("factoring") requires multiple parties
validating the same invoice: buyer acknowledges receipt, seller confirms
shipment, financier validates both. Current systems are email chains
plus database records.

**Why Quidnug:** Each invoice is a title with fractional ownership. Each
validation is a signed event in the invoice's stream. Trust domain
`factoring.supply-chain` holds the industry's validators (rating
agencies, insurers).

---

## AI

AI systems where provenance, authorization, and accountability must be
cryptographic rather than platform-owned.

### [`ai-model-provenance/`](ai-model-provenance/)

**Problem:** "Is this really GPT-4o? Is this model trained on copyrighted
data? Did someone fine-tune it on malware?" The AI supply chain has no
universal cryptographic provenance.

**Why Quidnug:** Quids represent training datasets, base models, fine-
tune outputs, inference agents. Title transactions track "derived from"
relationships with signed authorizations. Event streams log each training
run. Trust domains scope attestations (copyright, safety, benchmarks).

---

### [`ai-agent-authorization/`](ai-agent-authorization/)

**Problem:** Autonomous agents spend money, sign contracts, write to
databases. "Grant this agent access to $10K of credits" isn't a single
OAuth scope — it's a time-bounded, revocable, multi-party-approved
capability.

**Why Quidnug:** Agent is a quid. Its authorization is a guardian-set
update installing a quorum (principal + safety committee + audit bot).
Emergency revocation is a GuardianSetUpdate replacing the set. Time-lock
veto gives humans a window to stop a going-rogue agent.

---

### [`federated-learning-attestation/`](federated-learning-attestation/)

**Problem:** Banks collaborating on a fraud model want to contribute
gradients without exposing their raw data, and want credit for their
contribution. They don't trust each other enough to accept "central
coordinator says this participant contributed X."

**Why Quidnug:** Each gradient update is an event in the round's stream,
signed by the contributor's quid. Relational trust in the coordinator
is personalized per participant. Push gossip ensures everyone sees the
same signed log of contributions.

---

### [`ai-content-authenticity/`](ai-content-authenticity/)

**Problem:** "Is this photo AI-generated? Has this video been edited?
Who published it first?" C2PA + watermarking is a partial answer;
bridging C2PA identities to a decentralized trust graph is the gap.

**Why Quidnug:** Camera manufacturer's quid signs a capture event. Each
edit (crop, grade, filter) is a subsequent event in the asset's stream,
signed by the editor's quid. Consumers evaluate trust in the chain from
their own view — a news organization trusts Reuters editors, a meme
site trusts different editors.

---

## Consumer rights / Anti-centralization

### [`decentralized-credit-reputation/`](decentralized-credit-reputation/)

**Problem:** Credit bureaus (Equifax / Experian / TransUnion)
are opaque, error-prone, breach-prone, and concentrate judgment
in three private companies. Social-credit systems concentrate
judgment even further — in a state. Both produce a universal
score that follows you without consent.

**Why Quidnug:** No score. Signed credit events on the
borrower's own quid (BYOQ); trust edges from each lender in
domain-specific scope; each prospective lender runs their own
relational trust evaluation. No central scorer. Subject controls
who sees what via encrypted access grants. Structurally prevents
social-credit concentration because there is no protocol
authority for a "total-citizen score."

---

## Government / Elections

### [`elections/`](elections/)

**Problem:** Voter registration, poll books, ballot secrecy,
verifiability, and instant recount — today's systems trade one
against another and none are publicly verifiable.

**Why Quidnug:** Bring-your-own voter quid + authority-signed
registration trust edge + blind-signature ballot issuance +
per-BQ vote trust edges = every design requirement for a
cryptographically-sound election, with paper-ballot parity for
fail-safe auditability. Anyone can recount by running a query.

The elections folder is the most detailed use case in the
library — seven files covering the full lifecycle:

- `README.md` + `architecture.md` + `implementation.md` +
  `threat-model.md` (the standard four-file use-case pattern)
- `integration.md` — how the design composes on top of
  QDPs 0012 / 0013 / 0014
- `operations.md` — deployment at five scales (pilot to
  federal), capacity planning, cost analysis
- `launch-checklist.md` — sequential T-180 through T+30 go-
  live steps

~5000 lines total. Use it as the reference when designing any
complex multi-party / multi-jurisdictional use case.

---

## Cross-industry

High-stakes domains where the trust model, key lifecycle, or audit
story is what matters.

### [`healthcare-consent-management/`](healthcare-consent-management/)

**Problem:** Patient medical records span ERs, primary care, specialists,
labs, and insurers. Consent is currently faxed signatures or platform-
locked portals. HIPAA doesn't prevent wrong-provider access so much as
punish it after the fact.

**Why Quidnug:** Patient quid with guardian-based override (for
emergencies when the patient is unconscious). Per-provider access
consents are event-stream entries. Specialist referral trust chains
give a new doctor transitive access through referral chains the patient
approved.

---

### [`credential-verification-network/`](credential-verification-network/)

**Problem:** Diplomas, professional licenses, certifications. Current
flow: employer requests paper copy → phones the registrar → maybe gets
a response in 5 business days. Revocation is effectively impossible.

**Why Quidnug:** Issuers (universities, state medical boards, cert
orgs) are quids with their own guardian sets. Credentials are signed
identity transactions. Trust domains scope jurisdiction/industry.
Revocation = guardian-approved invalidation anchor. Cross-jurisdiction
trust flows through reciprocity agreements recorded as trust edges.

---

### [`developer-artifact-signing/`](developer-artifact-signing/)

**Problem:** Open-source maintainer signs releases with GPG. Loses the
key — or it gets compromised. Every downstream consumer has to re-verify
against a new key, and there's no recovery path. "Reflections on Trusting
Trust" meets "I forgot my password."

**Why Quidnug:** Maintainer's quid has a guardian set of co-maintainers.
Lost the signing key? Guardians rotate to a new key with time-lock veto.
Downstream consumers track the maintainer quid, not a specific key —
rotation is cryptographically auditable. Fork-block transactions
coordinate ecosystem-wide changes (e.g., "after block H, enforce sigstore-
style transparency log for this package").

---

## Infrastructure / internet plumbing

Use cases that replace or augment foundational internet
services. Everything in this category load-bears heavily on
all three architectural pillars (QDP-0012 governance,
QDP-0013 federation, QDP-0014 discovery).

### [`dns-replacement/`](dns-replacement/)

**Problem:** DNS has nine structural flaws — centralized root authority,
registrar-mediated rent-seeking ownership, DNSSEC complexity, cache
poisoning, BGP-hijack vulnerability, CA-dependent TLS, fragile key
rotation, opaque censorship at every layer, and no cryptographic owner
binding. Each one would be a design blocker if DNS were being proposed
today.

**Why Quidnug:** A domain is a `TrustDomain` with cryptographic
governors (QDP-0012). DNS record types map one-to-one to signed
`DNS_RECORD` events on the domain's stream. Federation (QDP-0013)
gives users alternative roots. Discovery (QDP-0014) finds the right
nodes to query. Guardian recovery replaces "forgot password." DANE
over Quidnug-signed TLSA records removes CAs from the TLS trust path.
First deployable as a parallel `.quidnug` TLD, then as a DNS gateway
for legacy clients, eventually as an alternative root for existing
TLDs.

---

## Index summary

| #  | Use case                               | Domain        | Key Quidnug features                                                   |
|----|----------------------------------------|---------------|-----------------------------------------------------------------------|
| 1  | interbank-wire-authorization           | FinTech       | Guardian M-of-N, nonce replay protection, recovery                    |
| 2  | merchant-fraud-consortium              | FinTech       | Relational trust, domain scoping, push gossip                         |
| 3  | defi-oracle-network                    | FinTech       | Signed feeds, K-of-K bootstrap, per-consumer aggregation              |
| 4  | institutional-custody                  | FinTech       | Full key lifecycle, epoch audit, lazy probe                           |
| 5  | b2b-invoice-financing                  | FinTech       | Titles, event streams, domain-scoped validators                       |
| 6  | ai-model-provenance                    | AI            | Titles for models, event streams for training, trust in attesters     |
| 7  | ai-agent-authorization                 | AI            | Guardian set as capability scope, time-lock veto, emergency revoke    |
| 8  | federated-learning-attestation         | AI            | Event streams for gradients, push gossip for coordination             |
| 9  | ai-content-authenticity                | AI            | Event streams per asset, transitive edit trust                        |
| 10 | **elections**                          | **Government**| **BYO voter quid, blind-sig ballot issuance, public recount, paper parity** |
| 11 | **decentralized-credit-reputation**    | **Consumer/FinTech** | **BYO subject quid, per-lender relational trust, alt-data, anti-social-credit** |
| 12 | healthcare-consent-management          | Healthcare    | Guardians for emergency override, consent event streams               |
| 13 | credential-verification-network        | Cross-industry| Issuer quids + guardians, revocable anchors, domain hierarchy          |
| 14 | developer-artifact-signing             | Open source   | Guardian recovery, fork-block for ecosystem upgrades                  |
| 15 | **dns-replacement**                    | **Infrastructure** | **Domain governance, federation (alt roots), DANE-integrated TLS, guardian recovery for names** |

---

## Contributing a use case

See [`BUILDING-A-USE-CASE.md`](BUILDING-A-USE-CASE.md) for the
full recipe: six phases in six hours that produce the four-file
use-case folder structure. The short version:

1. Read [`ARCHITECTURE.md`](ARCHITECTURE.md) first so you know
   the protocol substrate, the three architectural pillars
   (governance / federation / discovery), and the shape every
   use case takes.
2. Pick an archetype (reputation / attestation / coordination
   / infrastructure) and crib heavily from the closest
   existing use case's folder.
3. Follow the six-phase recipe in
   [`BUILDING-A-USE-CASE.md`](BUILDING-A-USE-CASE.md) to
   produce your README.md + architecture.md + implementation.md
   + threat-model.md.
4. Link your folder from this index and from the top-level
   README. Open a PR.

Don't bundle protocol changes into a use-case PR. If your
design needs new on-chain primitives, propose those as a QDP
under `docs/design/` first, then come back and write the
use case.
