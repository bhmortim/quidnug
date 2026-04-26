# Architecture: trust-weighted reviews

> System design for trust-weighted reviews on Quidnug, with
> DNS-anchored validation as the credibility floor for both
> sites and reviewers. Companion to
> [`README.md`](README.md) (market case) and
> [`implementation.md`](implementation.md) (code paths).

## 1. The four-layer stack

```
┌─────────────────────────────────────────────────────────┐
│ 4. Site & widget layer                                  │
│    amazon.com, etsy.com, alice-eats.com, browser ext.   │
│    Reads events; renders per-observer ratings           │
└─────────────────────────────────────────────────────────┘
              ▲                          ▲
              │ HTTPS reads              │ HTTPS writes
              │                          │
┌─────────────────────────────────────────────────────────┐
│ 3. Per-observer rating algorithm                        │
│    examples/reviews-and-comments/algorithm.py           │
│    Per-vertical, decayed transitive, helpfulness-       │
│    weighted, recency-decayed                            │
└─────────────────────────────────────────────────────────┘
              ▲
              │ event queries
              │
┌─────────────────────────────────────────────────────────┐
│ 2. QRP-0001 Reviews Protocol                            │
│    REVIEW / HELPFUL_VOTE / FLAG / PURCHASE / DISCLOSURE │
│    Domain tree: reviews.public.<topic>                  │
│    Deterministic product Title computation              │
└─────────────────────────────────────────────────────────┘
              ▲
              │ wire protocol
              │
┌─────────────────────────────────────────────────────────┐
│ 1. Quidnug protocol substrate                           │
│    Seed nodes, gossip, signed blocks, append-only       │
│    QDP-0001..0024 + DNS validation via QDP-0023         │
└─────────────────────────────────────────────────────────┘
```

Each layer is independent. Layer 1 is run by Quidnug LLC and
federated peers (QDP-0013). Layer 2 is a versioned spec.
Layer 3 is where sites and aggregators differentiate. Layer 4
is anyone who reads or writes.

## 2. Identities

Three quid types interact in the reviews ecosystem:

| Quid type        | What it is                                      | Real-world bond                                     |
|------------------|-------------------------------------------------|-----------------------------------------------------|
| Reviewer quid    | A reviewer's signing identity                   | Optional: validated personal domain (Pro/Business)  |
| Site quid        | A platform that hosts review embeds             | Required to issue PURCHASE: validated site domain   |
| Product Title    | A reviewable thing (laptop, restaurant, book)   | Deterministic from canonical identifiers (ASIN etc.)|
| Operator quid    | A validated entity issuing TRUST edges to others| Required: validated domain plus optional KYB        |
| Validation-operator quid | Quidnug LLC's signing identity for validation TRUST edges | Subordinate to seed root        |

A reviewer can also be an operator (Alice issues TRUST edges
to other food critics). A site can also be a reviewer (the
official acmestore.com account writes reviews). All four
types use the same Quidnug quid primitive; the role is
contextual.

## 3. The domain tree

```
reviews.public                           (root governed by
│                                        Quidnug LLC + future
│                                        delegated governors)
├── reviews.public.technology
│   ├── reviews.public.technology.laptops
│   ├── reviews.public.technology.cameras
│   ├── reviews.public.technology.software
│   ├── reviews.public.technology.phones
│   └── reviews.public.technology.audio
├── reviews.public.restaurants
│   ├── reviews.public.restaurants.us.<state>.<city>
│   └── reviews.public.restaurants.<iso-country>.<city>
├── reviews.public.books
├── reviews.public.movies
├── reviews.public.tv
├── reviews.public.places
├── reviews.public.services
│   ├── reviews.public.services.professional
│   └── reviews.public.services.home
└── reviews.public.products
```

Topical scoping rules (QRP-0001 §3):

- Trust is scoped to the leaf domain. A 0.9 trust in
  `reviews.public.restaurants.us.ny.nyc` does not propagate
  to `reviews.public.technology`.
- **Inheritance**: a parent-domain trust edge inherits to
  children at decay 0.8 per hop. Trust at
  `reviews.public.technology` 0.9 → 0.72 at
  `reviews.public.technology.laptops` by default.
- **Override**: an explicit child-domain edge supersedes
  inheritance.
- **Sibling extensions**: sites can run private siblings
  (`reviews.privatestore.acmestore.com`) for internal review
  flows that don't pollute the public graph.

## 4. Event flow: writing a review

```
   Alice's browser                Quidnug network
   ───────────────                ────────────────
        │
        │ 1. Alice loads acmestore.com/p/12345
        │
        ▼
   ┌─────────┐
   │ <quidnug-│
   │  review> │
   │ widget   │
   └─────┬────┘
         │ 2. Widget asks browser ext./wallet for Alice's quid
         │
         ▼
   ┌──────────────┐
   │ Quidnug      │
   │ wallet       │ ──── 3. Looks up product Title:
   │ extension    │       sha256("REVIEW-TITLE:" || canonical_ids)[:16]
   └──────┬───────┘
          │
          │ 4. Constructs REVIEW event:
          │    - subject_id: <product-title-quid>
          │    - subject_type: TITLE
          │    - event_type: REVIEW
          │    - sequence: <next>
          │    - domain: reviews.public.technology.laptops
          │    - payload: { qrpVersion:1, rating:4.5, body:..., ... }
          │    - signed by Alice's quid
          │
          ▼
   ┌─────────────────────────────────────────────┐
   │ POST https://api.quidnug.com/api/transactions│
   └─────────┬───────────────────────────────────┘
             │
             ▼
   ┌────────────────────────────────────────┐
   │ api.quidnug.com (api-gateway Worker)   │
   │ → routes to healthy seed: node-1 or 2   │
   └────────────┬───────────────────────────┘
                │
                ▼
   ┌────────────────────────────────────────────┐
   │ seed node                                   │
   │ - validates signature                       │
   │ - validates domain authority                │
   │ - validates sequence (per-quid stream)      │
   │ - includes in next block                    │
   │ - gossips to peer seeds (QDP-0005)          │
   └────────────────────────────────────────────┘
```

Critical properties:
- The widget never talks directly to acmestore.com's database.
  The review goes to the public network, not the site.
- The site can later query the network and render Alice's
  review, but it cannot edit it.
- Helpfulness votes from any other site land in the same
  event stream and accrue to Alice's reviewer reputation.

## 5. Event flow: rendering a per-observer rating

```
   Bob's browser                 Quidnug network
   ─────────────                 ────────────────
        │
        │ 1. Bob loads acmestore.com/p/12345
        │
        ▼
   ┌─────────┐
   │ <quidnug-│
   │  review> │
   │ widget   │
   └─────┬────┘
         │ 2. Resolve product Title from canonical IDs
         │
         ▼
   ┌──────────────────────────────────────────┐
   │ GET api.quidnug.com/api/streams/         │
   │     <product-title>/events?type=REVIEW   │
   │     &type=HELPFUL_VOTE&type=FLAG&        │
   │     limit=N                              │
   └──────────────┬───────────────────────────┘
                  │
                  ▼
   ┌──────────────────────────────────────────┐
   │ Get Bob's outgoing TRUST edges in this   │
   │ topic (from his wallet local cache or    │
   │ from the network):                       │
   │ GET api.quidnug.com/api/trust/<bob>?     │
   │     domain=reviews.public.tech.laptops   │
   └──────────────┬───────────────────────────┘
                  │
                  ▼
   ┌──────────────────────────────────────────┐
   │ Run rating algorithm (algorithm.py):      │
   │ - For each REVIEW, compute reviewer's    │
   │   effective trust from Bob's view        │
   │   (direct edge, or transitive at 0.8/hop)│
   │ - Weight by HELPFUL_VOTE flow from       │
   │   trusted observers                       │
   │ - De-weight if FLAG events from trusted  │
   │   moderators                              │
   │ - Apply recency decay                    │
   │ - Sum, normalize                         │
   │ → Bob's effective rating: 4.1            │
   └──────────────┬───────────────────────────┘
                  │
                  ▼
              Renders 4.1 ⭐ to Bob;
              Carol on the same page sees 3.7;
              an anonymous visitor sees 3.9 (no observer trust);
              all three are honest from their viewpoint.
```

The same product Title with the same event stream produces
three different ratings for three observers. None is the
"correct" one; each is correct from its observer's frame.

## 6. DNS-anchored validation flow

The validation Worker (`service.quidnug.com`, see service-API
spec) is what binds quids to real-world domains. Two flows:

### 6.1 Site validation flow

```
   1. acmestore.com signs up at quidnug.com/pricing → Stripe
      Checkout (Pro tier, $19/month).
   2. Stripe webhook → service-api Worker creates Customer
      record, generates customer quid (or uses one supplied).
   3. Worker returns DNS challenge: TXT record at
      _quidnug-challenge.acmestore.com containing
      "qn-chl=v1;nonce=<32hex>;cus=<id>;iat=<ts>".
   4. Customer publishes the TXT record.
   5. Customer hits POST /v1/domains/:id/verify.
   6. Worker probes 4 DoH resolvers (Cloudflare, Google,
      Quad9, OpenDNS); requires 3-of-4 quorum.
   7. On pass, Worker constructs and submits TRUST tx via
      api.quidnug.com:
         truster: <validation-operator-quid>
         trustee: <acmestore.com customer quid>
         domain:  operators.acmestore.com.network.quidnug.com
         level:   0.8 (Pro)
      Tx published, edge live on the network.
   8. Worker writes Attestation bundle to R2; emails customer
      with API key + console link + attestation download.
   9. Cron re-probes every 1 hour (Pro). On 4-hour grace
      drop: TRUST level=0.0 superseding edge published.
```

### 6.2 Reviewer validation flow

Same mechanics, different intent. Alice the food critic
validates `alice-eats.com` at Pro tier. She gets the same
TRUST edge. But because she also signs reviews from
`alice-eats.com`'s quid (hopefully the same as her reviewer
quid for clarity), the rendering widget can show:

```
  Alice (alice-eats.com) ✓ Pro-validated, KYB verified
  247 reviews in reviews.public.restaurants
  helpfulness ratio 0.87  •  active 4 days ago
```

The validation edge composes with her reviewer trust graph.
Observers who don't directly know Alice but trust the
operator root inherit a low (0.2 default) transitive baseline
in her favor. Observers who follow her (direct edge at e.g.
0.85) override that baseline.

## 7. The rating algorithm at a conceptual level

Full reference in
[`../../examples/reviews-and-comments/algorithm.md`](../../examples/reviews-and-comments/algorithm.md).
At a conceptual level:

```
effective_rating(observer, product) =
    Σ_review rating(review)
        × reviewer_weight(observer, reviewer, topic)
        × helpfulness_weight(review, observer)
        × recency_decay(review.timestamp)
        × (1 - flag_penalty(review, observer))
    / Σ_review (the same multipliers, sum)

where:
  reviewer_weight = max over paths from observer to reviewer
                    in the trust graph, scoped to topic,
                    decayed at 0.8 per hop, capped at the
                    direct edge if one exists

  helpfulness_weight = 1 + Σ_helpful_vote
                        reviewer_weight(observer, voter, topic)
                        × time_decay(vote.timestamp)

  flag_penalty = max moderator flag weight where
                  observer has trust in moderator at
                  reviews.moderation.<topic>

  recency_decay = exp(-t/τ), τ tunable per-vertical
                  (electronics: 90 days; books: 5 years)
```

Critically, **no global score**. Two observers with
overlapping but non-identical trust graphs see different
numbers, both correct from their own frames.

## 8. Bootstrap problem and the OIDC bridge

The empty-graph problem: a brand-new reviewer has no trust
edges. Three bootstrap paths (full detail in
[`../../examples/reviews-and-comments/bootstrap-trust.md`](../../examples/reviews-and-comments/bootstrap-trust.md)):

1. **OIDC bridge** (lowest friction). New user signs in with
   Google/GitHub/Apple at `auth.quidnug.com`. The bridge
   mints them a quid bound to their verified email and issues
   a baseline TRUST edge from `operators.network.quidnug.com`
   at 0.2. They're now visible to anyone who trusts the
   operator root.

2. **Cross-site import**. An existing Amazon/Yelp reviewer
   requests an attestation from a domain operator (e.g., a
   reviewer guild) that vouches for their off-Quidnug history.
   The operator publishes a TRUST edge.

3. **Social bootstrap**. Friends-and-family seed: founders,
   beta testers, known reviewers cross-trust each other. This
   is enough for the first few dozen users before OIDC
   bootstrap takes over.

The OIDC bridge is the production engine; #2 and #3 are seed
material.

## 9. Federation (QDP-0013)

The single-operator network (Quidnug LLC running two seeds)
is a starting point. As soon as a second serious operator
wants to participate, federation kicks in:

- Operator B runs their own `reviews.public.*` parallel tree.
- A and B publish bilateral TRUST edges in
  `peering.network.quidnug.com` and
  `validators.network.quidnug.com`.
- Reviewer quids span both networks; reviews flow bidirectionally
  at a controlled discount (0.8 per cross-network hop, tunable).

This means there is no single-point-of-failure at the
governance layer. If Quidnug LLC misbehaves, operators
defederate; reviewer reputations survive on the operators they
remain peered with.

## 10. Sharding (QDP-0014)

For volume scaling: when one topic accrues > N events/day, it
shards into child topics. The sharding model and discovery
protocol are in QDP-0014. Practically, this means the
substrate scales horizontally; observers and sites need only
discover which seeds carry which shards.

## 11. What this architecture enables

- **A platform-independent review market.** Sites become
  viewports. Reviewers become portable professionals.
- **Verifiable provenance for SEO and compliance.** Schema.org
  Review JSON-LD is generated from signed events, so search
  engines (and regulators) get cryptographic provenance for
  free.
- **Audit trails for litigation.** A defamation suit can
  demonstrate (with cryptographic proof) the reviewer's
  signed event, timestamp, and any subsequent edits or
  retractions.
- **Cross-domain reputation flows.** Healthcare reviews,
  professional credentials, product reviews, expert testimony
  all live on the same substrate. A doctor's medical-device
  review reputation composes with their FHIR credential.

## 12. What this architecture does not solve

Honest about residual limits:

- **Observer initialization**. A user with no trust edges sees
  no opinionated scores; they see the "anonymous baseline"
  (operator-rooted weighting). This is honest but
  underwhelming on day one for that user. The OIDC bridge
  partially addresses this; full personalization requires the
  user to interact (follow some reviewers, vote helpful).
- **Privacy of reviews**. Reviews are public events. A
  reviewer's history is fully visible. This is a feature for
  professional reviewers, a bug for anonymous reviewers. QRP
  may add anonymous-ballot extensions later (QDP-0021 blind
  signatures, QDP-0024 group encryption); not in v1.
- **Subjective taste**. Trust-weighting helps with credibility,
  not with "is this restaurant's cuisine my style." That's
  matchmaking, not authentication; it's a separate problem.
- **Regulatory edge cases**. Defamation, GDPR right-to-erasure,
  DMCA: append-only protocols cannot delete. The mitigations
  are operator-level non-gossip per QDP-0015 (content
  moderation) and QDP-0017 (data subject rights). Operators
  who choose to honor takedown requests render the content
  unreachable through their nodes; other operators may continue
  serving. Publish your moderation policy before launching.
