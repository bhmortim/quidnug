# How use cases are built on Quidnug

> The single-document tour of Quidnug's architecture, read
> from the perspective of "I want to build X on top." Ties
> together the protocol substrate, the three architectural
> QDPs (0012, 0013, 0014), the operator playbooks, and every
> use case in this directory.
>
> **If you're new, read this first.** Every other doc in the
> repo assumes some of what's explained here. This is the map.

## Table of contents

1. [What Quidnug is, in one paragraph](#1-what-quidnug-is-in-one-paragraph)
2. [The protocol substrate](#2-the-protocol-substrate)
3. [The three architectural pillars](#3-the-three-architectural-pillars)
4. [The shape every use case takes](#4-the-shape-every-use-case-takes)
5. [Use-case archetypes + how to choose](#5-use-case-archetypes--how-to-choose)
6. [Mapping use cases to primitives](#6-mapping-use-cases-to-primitives)
7. [Cross-cutting concerns](#7-cross-cutting-concerns)
8. [Where to read next, by role](#8-where-to-read-next-by-role)
9. [The ground truth as of now](#9-the-ground-truth-as-of-now)

---

## 1. What Quidnug is, in one paragraph

Quidnug is a P2P protocol and Go reference node for systems
where trust is personal, cryptographic, and contextual. Every
identity is a quid (a public key + metadata); every assertion
between identities is a signed, replay-safe transaction; every
trust judgment is computed from the asking observer's
perspective, not from a universal score. A trust graph, a
tamper-evident event log, and key-lifecycle primitives
(rotation, guardian-based recovery, epoch scoping), all
in one protocol. Use cases are built by designing a domain
hierarchy, picking a consortium to produce its blocks, and
publishing signed events that carry domain-specific meaning.

Examples in this directory range from "replace credit
bureaus" to "replace DNS." The point is that the same
primitives, composed differently, give you all of them.

## 2. The protocol substrate

Six primitives, unchanged across every use case. Internalize
these and everything else is configuration.

### 2.1 Quid

A cryptographic identity. ECDSA P-256 keypair; quid ID =
`sha256(publicKey)[:16]` in hex. Universal — the same key
makes the same quid on every network. Quids can represent
people, organizations, devices, AI agents, documents,
contracts, or domains. Anything that signs things.

### 2.2 TRUST edge

A signed assertion "quid A trusts quid B at level L (0.0 to
1.0) in domain D." Replay-protected by a monotonic nonce per
`(truster, trustee)` pair. Relational by construction:
there's no "what is B's trust score," only "what is B's trust
score from my perspective, walking my edges."

### 2.3 Domain

A named namespace for trust judgments. Hierarchical by
convention (`contractors.home.services`), but the protocol
doesn't enforce hierarchy — domains are opaque strings. Each
domain has a consortium that produces its blocks and a
governor quorum that decides who's in the consortium (both
defined by QDP-0012).

### 2.4 Event stream

An append-only, monotonically-sequenced log bound to any quid
or title. Events are signed transactions with a type tag and
a payload. This is where use-case-specific semantics live: a
review, a medical record, a DNS record, a vote, a bridge
crossing. One protocol, many meanings.

### 2.5 Title

A first-class asset object with fractional ownership
(expressed as weighted quid shares). Titles can be transferred
between owners via co-signed transactions. Used for anything
that has "ownership" semantics: products being reviewed,
invoices being financed, medical records being consented to,
domains being held.

### 2.6 Anchor

A transaction that manages a quid's key lifecycle: rotation
(epoch N → N+1), invalidation (freeze an epoch), fork-block
(protocol-feature activation at a future block height).
Combined with guardian recovery (QDP-0002), anchors give
every quid a complete "what happens when the key goes wrong"
story.

These six compose. A domain is named. Identities publish
TRUST edges scoped to the domain. Events flow along those
trusted paths. Titles represent owned things; anchors keep
keys healthy. Every use case in this directory is some
combination of these six, nothing more.

## 3. The three architectural pillars

Starting from the substrate above, three QDPs layer on top to
make Quidnug usable at scale. They were designed together,
they work together, and every use case that goes beyond a
single-operator toy uses all three.

### 3.1 Domain Governance — [QDP-0012](../docs/design/0012-domain-governance.md)

**The question it answers:** who can act authoritatively on
behalf of a domain, and how does that change over time?

**The answer:** three roles per domain.

- **Cache replica** — any node that trusts the consortium for
  a domain. Mirrors the agreed chain locally, serves reads,
  relays transactions. Default role for every node. Zero
  protocol ceremony to become one.
- **Consortium member (validator)** — a node whose blocks are
  part of the agreed chain for the domain. Membership is
  per-domain, granted via on-chain governance action. Not
  self-declared.
- **Governor** — a quid authorized to vote on consortium
  roster changes. M-of-N quorum per domain, fixed at
  registration, mutable only by unanimous governor vote.

**The primitive:** `DOMAIN_GOVERNANCE` transaction. Seven
actions (`ADD_VALIDATOR`, `REMOVE_VALIDATOR`,
`UPDATE_VALIDATOR_WEIGHT`, `SET_TRUST_THRESHOLD`,
`DELEGATE_CHILD`, `REVOKE_DELEGATION`, `UPDATE_GOVERNORS`),
each gated on governor quorum + a notice period (24h default)
so operators can react before changes activate.

**Operator-facing version:** [`deploy/public-network/governance-model.md`](../deploy/public-network/governance-model.md)

### 3.2 Network Federation — [QDP-0013](../docs/design/0013-network-federation.md)

**The question it answers:** how do multiple independent
Quidnug networks relate to each other, and how does
reputation flow between them?

**The answer:** there is no "the public network" at the
protocol level. The quidnug.com network is one configuration;
anyone can run their own with different names, keys, and
peers. Three mechanisms let reputation flow between networks:

- **Shared-peer gossip** — point your node at both public and
  private peers; filter per-domain via `supported_domains`.
  Full fidelity.
- **External trust sources** — declare specific URLs + operator
  pubkeys for read-only trust lookups on specific domains.
  Bandwidth-efficient.
- **`TRUST_IMPORT` transaction** — explicitly commit a foreign
  TRUST edge to your local chain as an auditable on-chain
  record.

**The invariant:** a CI check enforces that no production code
hardcodes any public-network-specific identifier. The protocol
is provably network-neutral.

**Reputation fungibility works because quid IDs are universal**
— the same pubkey produces the same quid ID on every network,
so a signed attestation on one network is cryptographically
verifiable on any other.

**Operator-facing version:** [`deploy/public-network/federation-model.md`](../deploy/public-network/federation-model.md)

### 3.3 Node Discovery + Sharding — [QDP-0014](../docs/design/0014-node-discovery-and-sharding.md)

**The question it answers:** at operator-scale, how does a
client find the specific node that holds the data it needs,
without listing every node in every client?

**The answer:** a three-layer discovery model.

- **Operator-to-nodes attestation** — the operator quid
  publishes TRUST edges to each of its node quids in the
  reserved `operators.network.<operator-domain>` domain.
  These edges are the authoritative "this node is mine"
  signal.
- **`NODE_ADVERTISEMENT` transaction** — each node publishes
  its endpoints, supported domains, capabilities, and
  expiration. Signed by the node itself.
- **Well-known file (`/.well-known/quidnug-network.json`)** —
  the operator publishes a stable HTTPS URL as the cold-start
  entry point for discovery. Format mirrors OpenID Connect
  discovery for familiarity.

**Discovery API:** `/api/v2/discovery/domain/<name>` returns
the consortium + endpoint hints + block tip for any domain.
Signed responses; edge-cacheable.

**Sharding patterns enabled:** geographic (multi-region),
domain-tree (different nodes for different parts of the
tree), capability (validator / cache / archive / IPFS
gateway), and network-federation (bridge nodes spanning two
networks).

**No-node participation mode:** apps and end-users can
participate without running a node at all — just hold a quid,
sign transactions locally, and POST them to an api gateway.
They get reputation fungibility and full cryptographic
participation; they give up offline operation and block
production.

**Per-domain quid index:** the discovery API includes an
endpoint listing quids active in a given domain, filterable
by activity, trust weight, and event type. The "find
reviewers in this topic" / "find operators for this use case"
lookup.

**Operator-facing version:** [`deploy/public-network/sharding-model.md`](../deploy/public-network/sharding-model.md)

### 3.4 How the pillars compose

Everything below the horizontal line is one use case per
stack; everything above is shared substrate.

```
  ┌─────────────────────────────────────────────────────┐
  │ Reviews (QRP-0001)    DNS replacement    Elections  │
  │ Credentials           AI provenance       Credit    │
  │ Guardian recovery     Interbank wires     Oracle    │
  │ ... everything in UseCases/ ...                    │
  └──────────────────────────┬──────────────────────────┘
                             │
                             │  use-case-specific event
                             │  schemas + client libraries
                             │  + UI primitives
                             │
  ┌──────────────────────────▼──────────────────────────┐
  │                                                      │
  │   QDP-0014 Node Discovery + Sharding                │
  │   (find the right node; per-domain quid index;      │
  │    no-node participation mode)                      │
  │                                                      │
  │   QDP-0013 Network Federation                       │
  │   (one protocol, many networks;                     │
  │    reputation fungibility across them)              │
  │                                                      │
  │   QDP-0012 Domain Governance                        │
  │   (cache replica / consortium / governor roles;     │
  │    on-chain governance transactions with notice     │
  │    periods)                                          │
  │                                                      │
  ├──────────────────────────────────────────────────────┤
  │                                                      │
  │   Protocol substrate                                │
  │   QDP-0001-0011: quids, TRUST edges, domains,       │
  │   events, titles, anchors, nonce ledger,            │
  │   guardian recovery, cross-domain gossip,           │
  │   fork-block, Merkle proofs, K-of-K bootstrap       │
  │                                                      │
  └──────────────────────────────────────────────────────┘
```

## 4. The shape every use case takes

Every use case in this directory follows a recognizable
eight-step pattern. Internalize this and building a new use
case becomes mostly about filling in the blanks.

### Step 1: Identify the domain hierarchy

What names will the use case live under? Examples:

- Reviews: `reviews.public.technology.laptops`,
  `reviews.public.restaurants.us.ny`, etc.
- Credentials: `credentials.education.us.accredited`,
  `credentials.medical.us.state.nj`, etc.
- DNS: `quidnug` (TLD), `example.quidnug`, `mail.example.quidnug`.
- Elections: `elections.us.nj.2028.presidential.poll-book`,
  `elections.us.nj.2028.presidential.ballots`, etc.

Every use case starts with a naming scheme. Hierarchy is
by convention; the protocol doesn't enforce it.

### Step 2: Identify the actors (quids)

Who are the identities involved? Examples:

- Reviews: reviewers, product-sellers, voters, product-assets
  (products are quids too).
- Credentials: issuers (universities), subjects (students),
  verifiers (employers), accreditors.
- DNS: domain owners, TLD operators, resolvers, end-users.
- Elections: voters, candidates, election authorities, audit
  observers.

Each actor class gets a role in the use case, but at the
protocol level they're all just quids.

### Step 3: Design your governance

Per-domain, pick:

- Who are the **governors** for the domain? (The people/orgs
  authorized to change the consortium roster.)
- Who are the **consortium members**? (The nodes that
  produce blocks.)
- What's the **quorum** for governance changes? (2-of-3,
  unanimous, etc.)
- What's the **notice period** before changes activate?
  (24 hours default.)

For small use cases this might be: governor = you, consortium
= your one node, quorum = 1.0. For public networks: multiple
governors across jurisdictions, multiple nodes across regions,
2/3 or 3/4 quorum.

### Step 4: Design your event payloads

What are the transaction types the use case introduces?
Reviews has REVIEW, HELPFUL_VOTE, UNHELPFUL_VOTE, REPLY,
FLAG, PURCHASE. DNS has DNS_RECORD, DNS_RECORD_TOMBSTONE,
DNS_WILDCARD. Elections has VOTER_REGISTRATION,
BALLOT_ISSUED, BALLOT_CAST, AUDIT_OBSERVATION.

Each gets a JSON schema, validation rules, and a canonical
signable-bytes definition. Reuse the existing
`EventTransaction` type — no new protocol surface needed
most of the time.

### Step 5: Design your trust edges

What do TRUST edges mean in this use case's domain?
Examples:

- Reviews: "I trust this reviewer for this topic at level
  L."
- Credentials: "Accreditor A attests University X is
  accredited."
- DNS: Trust edges aren't prominent in DNS because the
  protocol verifies signatures directly.
- Elections: "Election authority attests this voter is
  eligible in this precinct."

Every use case has a specific meaning for its TRUST edges.
This is where use-case-specific logic lives.

### Step 6: Design your rating / decision algorithm

Given the edges and events, what output does the use case
produce? Examples:

- Reviews: a per-observer trust-weighted rating (the
  four-factor T×H×A×R algorithm).
- Credentials: binary "is this credential valid right now?"
  plus transitive-trust paths for cross-jurisdiction.
- DNS: simple resolution ("here's the A record").
- Elections: tallied votes, with verifiable recounts.

Most algorithms reduce to "walk the TRUST graph to find
relevant signers, aggregate their events in a domain-
specific way, handle edge cases." Reviews has the most
worked-out example.

### Step 7: Build client libraries + UI

Ship in the languages + frameworks your users actually
use. For most use cases:

- Reference implementation in one language (Go for
  infrastructure, Python for data-heavy, JS for web).
- SDK mirrors in others.
- Framework-specific adapters (React, Vue, Astro, WP for
  reviews; native DNS resolver + browser extension for
  DNS; etc.).
- UI primitives where the use case has a user-facing
  visualization (the `<qn-aurora>` / `<qn-constellation>` /
  `<qn-trace>` family for reviews).

### Step 8: Launch + operate

Register your domains (with governance metadata), publish
your seeds.json, deploy cache replicas + consortium members,
wire monitoring, launch. The [`deploy/public-network/`](../deploy/public-network/)
playbooks cover this generically.

Write a launch checklist specific to your use case (the
reviews system has
[`deploy/public-network/reviews-launch-checklist.md`](../deploy/public-network/reviews-launch-checklist.md)
as a reference).

## 5. Use-case archetypes + how to choose

Not every use case is the same shape. Three archetypes,
distinguished by what the trust graph does for you.

### 5.1 Reputation archetype

**Example use cases:** reviews, credit reputation, merchant
fraud consortium, freelancer reputation, oracle networks.

**What trust does:** weights individual contributions by the
observer's confidence in the contributor. The output is a
personalized per-observer rating.

**Characteristic:** every observer sees a different result,
and that's the feature. Unlike averaging reviews, you
compute the rating each person actually cares about.

**Shape:** TRUST edges carry weight. Events carry
contributions. The rating algorithm walks trust paths and
weights events by path strength.

### 5.2 Attestation archetype

**Example use cases:** credential verification, AI model
provenance, C2PA content authenticity, developer artifact
signing, healthcare consent.

**What trust does:** certifies that a claim about a subject
is made by an authorized issuer. The output is binary plus a
verification path.

**Characteristic:** there's usually an authoritative issuer
hierarchy (accreditors → universities → graduates;
manufacturers → software artifacts). Trust edges define the
hierarchy; events are the attestations.

**Shape:** TRUST edges define "who can attest what." Events
are attestations signed by issuers. Verification checks the
attestation's signer is in the authorized hierarchy.

### 5.3 Coordination archetype

**Example use cases:** elections, multi-party wire
transfers, AI agent authorization, institutional custody,
federated learning.

**What trust does:** gates multi-party actions. The output
is a coordinated decision that multiple parties have to
agree on.

**Characteristic:** signatures from M-of-N parties are
required, often with time-locked windows for veto or
confirmation. Guardian recovery is load-bearing.

**Shape:** Guardian sets define the M-of-N quorum. TRUST
edges define who can be a guardian. Events carry the
signed-off actions.

### 5.4 Infrastructure archetype

**Example use cases:** DNS replacement, registry services,
PKI-like services.

**What trust does:** binds names/records to cryptographic
owners. The output is a signed response to a lookup.

**Characteristic:** every name maps to a domain with
explicit governance. Records are signed events on that
domain's stream. No centralized root.

**Shape:** Domain hierarchy matches name hierarchy. Events
carry records. Governance carries delegation.

### 5.5 Decision tree

"What archetype am I?"

- My output is a **rating or score** that differs per
  observer → **reputation**
- My output is **yes/no** plus a signature chain → **attestation**
- My output is a **multi-party approval** of some action
  → **coordination**
- My output is a **name resolution** or a registry-style
  lookup → **infrastructure**

Most real use cases are one archetype with a seasoning of
the others. DNS replacement is infrastructure-primary with
a light attestation layer (TLSA records). Reviews is
reputation-primary with coordination aspects (helpful-vote
aggregation). Knowing the primary archetype tells you which
existing use case to crib from most heavily.

## 6. Mapping use cases to primitives

Which QDPs does each use case actually load-bear on?

| Use case | Archetype | Core primitives | Governance? | Federation? | Discovery? |
|---|---|---|---|---|---|
| reviews-and-comments (QRP-0001) | reputation | TRUST edges, events, titles | yes | yes (fungibility) | yes (scale) |
| credential-verification-network | attestation | TRUST edges, identities, anchors | yes | maybe | yes |
| merchant-fraud-consortium | reputation | TRUST edges, events | yes (consortium) | maybe | low |
| defi-oracle-network | reputation | TRUST edges, events, K-of-K bootstrap | yes | no | yes |
| institutional-custody | coordination | Guardian sets, anchors, events | yes | no | no |
| b2b-invoice-financing | attestation + coordination | Titles, events, guardian sets | yes | no | no |
| ai-model-provenance | attestation | Identities, titles, events | yes | maybe | yes |
| ai-agent-authorization | coordination | Guardian sets, fork-block | yes | no | no |
| federated-learning-attestation | coordination | Events, push gossip | yes (consortium) | no | yes |
| ai-content-authenticity | attestation | Events, titles | yes | maybe | no |
| decentralized-credit-reputation | reputation | TRUST edges, events, identities | yes | yes | yes |
| elections | coordination | TRUST edges, guardian sets, events | yes (heavy) | no | yes |
| healthcare-consent-management | coordination | Guardian sets, events, titles | yes | no | no |
| developer-artifact-signing | attestation | Identities, anchors, guardian recovery | yes | maybe | no |
| dns-replacement | infrastructure | Domains, events, titles, anchors, guardian recovery | yes (heavy) | yes (heavy) | yes (heavy) |
| interbank-wire-authorization | coordination | Guardian sets, nonces, anchors | yes | no | no |

Reading the table: **reviews** and **DNS** load-bear on all
three architectural pillars most heavily. **Institutional
custody** and **ai-agent-authorization** lean entirely on the
core substrate (coordination + guardians) with light
governance. Most use cases need governance; federation matters
most for cross-network reputation; discovery matters most at
scale.

This is useful when deciding which use case to study first for
your own project: pick the one whose row most closely matches
yours.

## 7. Cross-cutting concerns

Things that apply to every use case regardless of archetype.

### 7.1 Key custody

Every use case has at least one "root" key (operator key, TLD
key, election-authority key, whatever). The root key goes on
paper in a fireproof envelope + encrypted password-manager
backup + guardian quorum (QDP-0002) for recovery. Non-
negotiable for anything beyond a toy.

Node keys are operational-tier: stored on the machine,
rotated periodically via `AnchorRotation`, revocable via
guardian recovery if the machine is compromised.

### 7.2 Bootstrap trust

New users and new networks both face a chicken-and-egg
problem: with no trust edges, everything is untrusted. Four
bootstrapping mechanisms (documented at
[`examples/reviews-and-comments/bootstrap-trust.md`](../examples/reviews-and-comments/bootstrap-trust.md)):

1. **OIDC binding** — bind an authenticated Google/GitHub/etc.
   identity to a fresh quid, with a baseline trust edge from
   the operator's `operators.*` root.
2. **Cross-site import** — use QDP-0013's `TRUST_IMPORT` to
   carry reputation from one network to another.
3. **Social bootstrap** — invite known humans, have them
   trust each other directly. Seeds the graph.
4. **Domain validator opt-in** — being a consortium member
   for a specific domain grants implicit credibility there.

Every use case eventually needs at least one of these. Pick
based on your audience: OIDC for mass-consumer, cross-site
import for federation scenarios, social for early community
builds, validator opt-in for B2B consortium launches.

### 7.3 Moderation + takedown

Quidnug is append-only and signed. Removing content is not a
protocol primitive. Moderation happens at two layers:

- **Protocol layer** — a `FLAG` event from a trusted signer
  de-weights the target's contributions in the rating
  algorithm.
- **Operator layer** — a node can choose not to gossip
  specific events (operational policy). The content stays on
  the chain, but clients of that operator's api don't see it.

For legal obligations (DMCA, GDPR right-to-erasure,
defamation), operators act as hosting providers: act in good
faith, document decisions, publish policy ahead of time. The
protocol can't rewrite history; operators can refuse to
distribute.

### 7.4 Monitoring + incident response

Every deployed use case needs:

- **Prometheus metrics** on the `/metrics` endpoint
  (already exported by every node).
- **Grafana Cloud** (free tier) for dashboards + alerts.
- **Uptime Kuma** or similar for liveness monitoring,
  hosted on infrastructure independent of the primary
  node (QDP-0014 cross-reference).
- **Status page** at a stable public URL.
- **Incident-response playbooks** for key compromise,
  consortium partition, gossip storm, abuse detection.

The reviews-launch-checklist and home-operator-plan cover
this generically. Every use case's launch checklist
adapts these patterns.

### 7.5 SEO + search-engine discoverability

For public-facing use cases (reviews, credentials, DNS),
search engines care about structured metadata. Quidnug
emits Schema.org JSON-LD alongside its custom
visualizations so Google + Bing + DuckDuckGo see familiar
rich-result markup.

[`integrations/schema-org/`](../integrations/schema-org/) is
the reference; use it or adapt its pattern for your
domain-specific schema.

### 7.6 Accessibility

For user-facing UI primitives (the rating visualization
family, future DNS-management UIs, credential display
widgets, etc.), WCAG 2.1 AA is the floor. Color
distinctions are always backed by shape or pattern;
screen-reader text is generated from the same data the
visual uses; keyboard navigation works.

[`docs/reviews/rating-visualization.md`](../docs/reviews/rating-visualization.md)
covers the pattern for reviews; the same discipline applies
to every other user-facing visualization.

### 7.7 Privacy boundaries

Quidnug is public by default. Trust edges, events, domain
state — all readable by anyone who can talk to a cache
replica. Two levers for privacy:

1. **Private domains** — a domain whose consortium operates
   a closed network. Events stay inside the consortium's
   gossip. Works for enterprise + regulated use cases.
2. **Encrypted payloads** — events can carry encrypted
   payloads with on-chain hash commitments. Observers see
   that an event happened; content requires a decryption
   key. The healthcare-consent-management use case uses
   this pattern.

Privacy-first use cases (healthcare, personal finance,
certain consent flows) combine both.

## 8. Where to read next, by role

### 8.1 Developer / integrator

Your goal: understand enough to build a client, widget, or
adapter against an existing use case.

Reading order:

1. This document (you're here). Ground the architectural
   model.
2. The README of the use case closest to yours. Understand
   the specific shape.
3. [`docs/openapi.yaml`](../docs/openapi.yaml) for the HTTP
   API surface.
4. [`clients/python/README.md`](../clients/python/README.md)
   or the SDK closest to your language. Concrete examples.
5. If you're building a UI: [`docs/reviews/rating-visualization.md`](../docs/reviews/rating-visualization.md)
   for the primitive design, and browse the framework
   adapters under `clients/`.

After that you can build. The primitives compose naturally.

### 8.2 Operator / devops

Your goal: run a node, a consortium, or a federated network.

Reading order:

1. This document. Ground the architectural model.
2. [`deploy/public-network/governance-model.md`](../deploy/public-network/governance-model.md)
   — the role separation matters operationally.
3. [`deploy/public-network/federation-model.md`](../deploy/public-network/federation-model.md)
   — one protocol, many networks; decide which your
   network is.
4. [`deploy/public-network/sharding-model.md`](../deploy/public-network/sharding-model.md)
   — the operational topology of many nodes + one
   operator.
5. [`deploy/public-network/home-operator-plan.md`](../deploy/public-network/home-operator-plan.md)
   — the cheapest launch plan ($0-$6/month).
6. [`deploy/public-network/reviews-launch-checklist.md`](../deploy/public-network/reviews-launch-checklist.md)
   — concrete use-case launch pattern, even if you're
   launching something else.

Then pick your infrastructure, follow the plan, monitor +
iterate.

### 8.3 Business / product stakeholder

Your goal: understand what Quidnug unlocks for a specific
domain and evaluate whether to build on it.

Reading order:

1. [`../README.md`](../README.md) — the 30-second demo +
   use-case list.
2. This document. Ground the architectural model.
3. The use case closest to your problem. Don't skim; read
   all four files in the folder.
4. [`docs/comparison/`](../docs/comparison/) — comparisons
   with alternatives (DIDs, PGP web of trust, OAuth,
   blockchain-based approaches). Lets you position
   Quidnug vs. what you might already be considering.

Then talk to a developer to validate feasibility.

### 8.4 Protocol contributor

Your goal: extend Quidnug with a new primitive or a new
QDP.

Reading order:

1. This document.
2. [`docs/architecture.md`](../docs/architecture.md) — the
   authoritative architectural document for the node
   implementation.
3. Every QDP under [`docs/design/`](../docs/design/), at
   least skimmed. You need to know what's been decided.
4. [`CONTRIBUTING.md`](../CONTRIBUTING.md) — process.

Then draft your own QDP by copying an existing one as a
template. QDP-0012 / QDP-0013 / QDP-0014 are recent and
cover governance, federation, and discovery respectively —
they're good models for shape and depth.

## 9. The ground truth as of now

### 9.1 What's shipped

Code live on main:

- Full Go reference node (QDPs 0001 through 0011).
- SDKs: Python, Go package, JavaScript (v1 + v2), Rust.
  Scaffolds: Java, .NET, Swift, Android, browser extension,
  ISO 20022.
- Reviews system (QRP-0001): full protocol spec, four-factor
  algorithm in Python + Go, end-to-end demo against a live
  node.
- Rating visualization primitives (`<qn-aurora>`,
  `<qn-constellation>`, `<qn-trace>`) with React, Vue, Astro
  adapters.
- Domain integrations: Sigstore, C2PA, HL7 FHIR, Chainlink,
  Kafka, ISO 20022, Schema.org reviews, OIDC bridge.
- Deployment: production Helm chart, Docker Compose
  consortium, PKCS#11/HSM + WebAuthn signer backends,
  Grafana dashboards + Prometheus alerts.

### 9.2 What's designed but not yet implemented

- **QDP-0012 (Domain Governance)** — Draft, pending Phase-1
  state extensions after public-network launch.
- **QDP-0013 (Network Federation)** — Draft; mostly
  clarifies existing uniformity. New surface:
  `external_trust_sources` config + `TRUST_IMPORT`
  transaction. CI invariant check is active.
- **QDP-0014 (Node Discovery + Sharding)** — Draft;
  `NODE_ADVERTISEMENT` tx, discovery API, `.well-known/`
  schema, per-domain quid index, no-node participation
  mode. In-tree scaffolding has started.

### 9.3 What's designed as future use cases

Each file in [`UseCases/`](.) is a design, not a
deployment. Most reuse existing protocol primitives; a few
(like the DNS replacement) are gated on QDP-0012 / 0013 /
0014 landing.

### 9.4 What's planned but pre-design

- **ZK selective disclosure** — privacy-preserving trust
  paths.
- **Post-quantum migration** — NIST PQC finalists, via
  QDP-0009 fork-block.
- **Threshold signatures** — aggregate M-of-N signing.
- **Svelte / SolidJS / Angular / Ember / Qwik** visualization
  adapters — easy after the three existing primitives are
  stable.
- **Additional vertical integrations** — Yotpo, Judge.me,
  Shopify extensions, LinkedIn-style endorsements,
  reviewing-enrichment layers.

The full near-to-long-term plan lives in [`docs/roadmap.md`](../docs/roadmap.md).

---

## The short version

- **One protocol**, six primitives. Everything else is
  configuration.
- **Three architectural pillars** on top: governance,
  federation, discovery.
- **Every use case** fits one of four archetypes
  (reputation, attestation, coordination, infrastructure)
  and follows the same eight-step shape.
- **Build** by picking the closest existing use case as a
  template, filling in the domain hierarchy + event schemas
  + trust-edge semantics, and wiring the client / UI layer.
- **Deploy** using the public-network operator playbooks,
  adapted to your scale.

The rest is details. Pick a use case from the index and go.
