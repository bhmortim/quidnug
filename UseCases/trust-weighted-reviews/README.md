# Trust-weighted reviews on Quidnug

> A global, cross-site review substrate where every rating is
> weighted by **the observer's** trust graph, not by an
> average that treats every reviewer as identical. Reviewer
> identity is portable; reputation is per-vertical; sites and
> reviewers stake real-world skin in the game through
> DNS-anchored validation.

This dossier is the strategic and market-facing case for
trust-weighted reviews. The QRP-0001 wire protocol, the
reference rating algorithm, the multi-actor simulation, and
the running demo all live in
[`examples/reviews-and-comments/`](../../examples/reviews-and-comments/).

## 1. The problem

Every review system on the open web in 2026 uses a model that
the underlying market structure makes impossible to fix:

- **One rating per entity.** "4.3 stars, globally, for everyone"
  is the only output shape the platform sells.
- **All reviewers weighted equally.** A bot-net farm's review
  counts the same as a credentialed food critic's.
- **Helpfulness votes don't propagate.** Upvoting a helpful
  review does nothing for that reviewer's next review on a
  different site.
- **No topical specialization.** A brilliant software reviewer
  has no baseline credibility on restaurants.
- **Reputation resets at every site.** Your Amazon history
  doesn't carry to Walmart, Yelp doesn't help on Google Maps,
  TripAdvisor vanishes at Hotels.com.
- **The platform owns the only copy.** Reviews can be removed,
  re-ranked, or quietly de-weighted to please advertisers, and
  the reviewer has no enforcement.

These are not bugs. They are the direct consequence of
"one average score fits all," baked into every review UI since
the 1990s, layered on a market structure where the platform's
incentive to police fraud is weaker than the seller's incentive
to commit it.

### Sizing the harm

- The US FTC's 2024 review-fraud rule estimates fake reviews
  influence roughly $152B in annual US purchasing. Per-violation
  penalties run up to $51,744.
- Amazon disclosed in court filings that ~30% of its review
  reservoir is suspect at any given moment; Amazon sued >10,000
  fake-review-broker groups in 2022-2023 alone.
- Independent audits of Google Maps reviews on competitive
  verticals (lawyers, dentists, tourist-zone restaurants)
  show 15-40% paid or coerced reviews.
- "Brushing" (sending empty boxes to fake addresses to forge
  verified-purchase labels) is a multi-billion-dollar global
  industry per FTC and Reuters reporting.
- Yelp has been sued repeatedly by small businesses alleging
  filter-driven extortion of advertising spend.

The fraud is a tax that consumers, honest sellers, and
platform employees all pay, but only the platforms are in a
position to fix it, and their gradient of profit points
toward the broken state.

## 2. The seven attack patterns

Stable, well-documented, all priced into the current market:

1. **Click-farm sybils.** Aged accounts at $0.50-$3 each on
   the gray market, residential proxies for IP rotation,
   industrial-scale review-as-a-service.

2. **Brushing.** Seller ships empty packages to fake addresses
   so the receiving "buyer" can post a "verified purchase"
   five-star review. Verified-purchase is the primary
   credibility signal on Amazon; gaming it is the highest-ROI
   attack.

3. **Review-trade networks.** Closed groups (Facebook, Telegram,
   Discord) where sellers swap five-star reviews on each
   other's products. Real human reviewers, real low-cost
   purchases, very hard to detect.

4. **Incentivized review schemes.** Insert-card schemes
   ("review us on Amazon for a $20 gift card") inducing
   reviews outside the platform's audit pipeline.

5. **Brigading.** Coordinated negative-review attacks driven by
   politics, competition, or viral grievance. The averaging
   algorithm offers no defense; one weekend can permanently
   move a 4.6 to a 3.2.

6. **Suppression.** Defamation suits and DMCA strikes against
   negative reviewers; documented patterns of platforms
   de-ranking negatives of paying advertisers. The reviewer
   has no permanent record because the platform owns the
   only copy.

7. **Generative-AI flooding.** GPT-class models produce reviews
   indistinguishable from human prose. The economic floor of
   fake reviews is now near zero. Any defense based on "this
   reads like real text" is dead.

## 3. What Quidnug changes mechanically

Five structural changes, not one new feature:

1. **Per-observer weighting.** No "global rating." Two readers
   loading the same product see two different numbers, both
   honest from their own viewpoints. The platform cannot
   publish a single average because no single average exists.

2. **Topical trust scoping.** Trust edges are scoped to domains
   like `reviews.public.technology.cameras`. Trust on cameras
   does not bleed into trust on restaurants. Reputation is
   built per-vertical, the way real-world expertise is.

3. **Decayed transitive trust.** If you don't know Alice but
   you trust Bob, and Bob trusts Alice on this topic, her
   review counts at a per-hop decayed weight (default 0.8 per
   hop). Delegate vetting; don't repeat it.

4. **Helpfulness votes that propagate.** A `HELPFUL_VOTE` is
   itself a signed event, scoped to a topic, by an identity
   with its own trust weight. Ten votes from observers you
   transitively trust mean something; ten votes from new
   accounts don't.

5. **Append-only signed events.** No platform can edit, delete,
   or quietly de-rank your review or your helpful-vote
   retroactively. The platform is a viewport; events live on
   the public Quidnug network.

The combined effect is not "fraud goes to zero." It is
**the economic floor for credible fraud rises sharply**. Faking
a review that affects a knowledgeable observer requires a
real cryptographic identity, real trust edges from real
reviewers in the target topic, and consistent quality not to
be flagged out. That cost structure is not what the
fake-review market is built around today.

## 4. How the public network actually works without a central authority

Three nested layers. Each is independent.

### Layer 1: protocol substrate

The public Quidnug network of seed nodes (initially a home +
VPS pair operated by Quidnug LLC, expandable through bilateral
peering per QDP-0013) gossips signed events under the
`reviews.public.*` domain tree. Defined by QDP-0001 through
QDP-0024. Anyone can run a node, anyone can read, only domain
validators can produce blocks for that domain.

### Layer 2: QRP-0001, the reviews protocol

Defined in
[`../../examples/reviews-and-comments/PROTOCOL.md`](../../examples/reviews-and-comments/PROTOCOL.md).
Specifies:

- Event types: `REVIEW`, `HELPFUL_VOTE`, `FLAG`, `PURCHASE`,
  `DISCLOSURE`.
- The domain hierarchy under `reviews.public.*`.
- Deterministic computation of product Title quids so the same
  physical product gets the same Quidnug Title regardless of
  which site registers it first.
- Inheritance and decay rules for trust across the topic tree.

### Layer 3: per-observer rating algorithms

Multiple are possible. The reference algorithm is in
[`../../examples/reviews-and-comments/algorithm.py`](../../examples/reviews-and-comments/algorithm.py).
Sites differentiate at this layer. Two sites loading the same
product can show subtly different ratings to the same observer
if they parameterize the algorithm differently (recency
weighting, hop decay, helpfulness coefficient). The algorithm
is the editorial voice of the site, layered on shared raw
data.

### Governance without a central authority

Per QDP-0012 (Domain Governance), each domain has:
- A consortium of validators (initially Quidnug LLC's two seed
  nodes; expandable through QDP-0013 federation).
- A governor set (the operator plus delegated co-governors).
- A governance quorum threshold for adding new top-level
  topics or rotating validators.

This is the same model DNS uses: ICANN-governed roots delegate
to TLD operators who delegate to registrars. Quidnug's review
tree has the same structure but is enforceable by signature
rather than by central registry. If another operator runs a
parallel `reviews.public.*` tree (federation per QDP-0013),
the two trees can cross-sign; reviewers don't pick a network,
their quid spans all of them.

## 5. DNS-anchored validation as the credibility floor

This is what binds cryptographic identity to legal
accountability. It is also the commercial product Quidnug LLC
sells (see the service-api and pricing docs).

### Sites that host reviews

A site validates `acmestore.com` through Quidnug Pro
($19/month) or Business ($99/month with KYB). The validation
Worker (per the service API spec) publishes a TRUST edge from
`operators.acmestore.com.network.quidnug.com` to the site's
quid. Now:

- Review embeds and PURCHASE attestations from acmestore.com
  carry verifiable accountability: observers know the events
  came from the legally accountable owner of acmestore.com.
- Sites with Business-tier KYB carry stronger attestations:
  Quidnug verified the legal entity. Their review embeds and
  TRUST edges they issue to their own customers carry more
  weight in observer rating computations.
- Sites that don't validate can still host reviews; observers
  may discount them.

### Reviewers who go pro

Alice the food critic registers `alice-eats.com`, validates
through Quidnug Pro, and the validation Worker publishes a
TRUST edge from the operator root to her reviewer quid under
`operators.alice-eats.com.network.quidnug.com`. Now:

- Anyone reading Alice's reviews can verify her quid is owned
  by the legal owner of alice-eats.com.
- Alice's reviewer card in the widget shows "alice-eats.com,
  Pro tier, validated YYYY-MM-DD, KYB verified, 247 reviews
  in `reviews.public.restaurants`, helpfulness ratio 0.87."
- If Alice lies about credentials, she can be sued under her
  real name. Her domain is a costly, persistent identity
  bond.
- Lose the domain (failed renewal, lost dispute, failed KYB
  rerun), and the validation edge auto-revokes (TRUST level
  drops to 0). Old quid's accumulated rep no longer points to
  a verifiable identity.

The compounding effect: dense, signed reputation graphs of
validated reviewers become navigable by observers. Subscribe
to one curator (a Wirecutter-equivalent), inherit transitive
trust in hundreds of reviewers. None of this requires the
curator to own the platform; the platform layer is the public
network.

## 6. New economic roles

The current market has two roles: platforms that monetize the
substrate, and unpaid reviewers. Trust-weighted reviews on a
public substrate enables roles that don't exist now.

### Professional trusted reviewer

A real job. The mechanics:
- Validate a personal domain through Quidnug ($19-$99/month).
- Build vertical reputation through consistent work in
  `reviews.public.<vertical>`.
- Monetize through paid newsletters, Patreon, disclosed
  sponsorships, consulting, expert testimony, paywalled
  detailed reports.

The reviewer doesn't depend on any single platform. Their quid
signs reviews everywhere. The compound: reputation accrues
across all sites that read the same network.

Comparable analogues to estimate market size: Substack's paid
newsletters cleared >$1B/yr in writer payouts within five
years of launch. YouTube's creator economy is >$20B/yr in ads
and sponsorships. Reviewers as a category are at least as
large; they just don't have the substrate today.

### Trusted-reviewer aggregator

Wirecutter/Consumer Reports as a Quidnug-native business.
Curate 50 vetted reviewers, validate the curating site, issue
TRUST edges to the reviewers. Sell subscriptions to consumers
who inherit the curation. The aggregator is selling editorial
trust as a service with cryptographic provenance.

### Reviewer guild or co-op

Members of `members.foodcritics.coop` validate the guild
domain and issue TRUST edges to dues-paying members. This is a
credentialing body. Members who lose the guild's trust have
their edge revoked publicly. This is the bar association
pattern, applied to review work.

### Brand-disclosure marketplace

A brand posts: "We will pay $X to N reviewers with rep > 0.7
in `reviews.public.tech.cameras` who will write an honest
review of product Y with mandatory DISCLOSURE event."
Reviewers bid; brand picks; disclosure is on-chain. Observers
can filter or weight sponsored reviews. Both sides get
something the current market can't deliver: real reviewers
who can't lie about being unsponsored, and brands who get
exposure to skeptical audiences without paying for ads they
know are filtered.

### Auditor / fraud hunter

Specializes in detecting review-trade networks, brushing
patterns, AI-generated review clusters. Publishes signed FLAG
events. Sites' Trust & Safety teams subscribe; foundations
fund. The role exists today inside platforms with no
externalization path; trust-weighted reviews makes it a
market.

### Expert testimony

A reviewer with 5 years of consistently helpful reviews in
`reviews.public.medical-devices` is now hireable as an expert
witness or consultant. Their public history is auditable;
their identity is verifiable. This use case doesn't exist
because today's reputations are not portable, persistent, or
verifiable enough.

## 7. Why this is now possible

Three preconditions just lined up:

1. **Cryptographic identity at scale is solved.** Modern
   browsers ship WebCrypto; key management UX (passkeys,
   browser extensions, mobile keychains) is mainstream as of
   2024-2025.

2. **DNS-anchored validation is cheap and fast.** Cloudflare,
   Google, Quad9, OpenDNS resolvers via DoH let any HTTP
   service do a four-resolver quorum probe in <500ms. The
   pattern is proven by ACME (Let's Encrypt) at internet
   scale.

3. **Generative AI broke the old defense.** Pre-2023, "this
   reads like real prose" was a credibility signal. Post-2023,
   it isn't. Identity-bound reputation is the only defense
   that scales, and that requires substrate the platform
   doesn't own.

## 8. Adoption path

Three audiences in priority order:

### Phase 1, weeks 1-8 from network launch

- **First reviewers** (10-50 known professionals across two or
  three verticals: tech, food, books). Validate them at
  Quidnug Pro free for the first year as seed-trust. Each one
  gets a TRUST edge from the operator root and seeds their
  vertical.
- **Aggregator partners** (1-3 publications). Wirecutter
  competitors, niche-vertical bloggers, podcast review shows.
  Same free year. Their endorsement of reviewers seeds the
  graph.

### Phase 2, weeks 8-24

- **Indie e-commerce sites and SaaS marketplaces** install the
  WordPress / Shopify / web-component widgets. Validate at Pro
  tier. Their PURCHASE attestations become the verified-purchase
  signal that doesn't depend on any platform.
- **Browser extension overlay** ships, allowing observers to
  see Quidnug-weighted ratings on Amazon, Yelp, Google Maps,
  TripAdvisor product pages. The overlay reads from
  `reviews.public.*` and re-ranks the existing site's reviews
  by observer trust. This is the wedge: consumers see the
  difference without sites needing to integrate.

### Phase 3, months 6-18

- **Major retailers** integrate at the platform level. By this
  point the network is dense enough that the marginal
  retailer who refuses looks like the holdout, not the
  default.
- **Regulatory tailwind.** FTC and EU consumer-protection
  rules increasingly demand verifiable review provenance;
  Quidnug's signed-event substrate is the lowest-friction
  compliance path.

### Phase 4, year 2+

- **Cross-vertical expansion** into healthcare reviews
  (paired with FHIR integration per `integrations/fhir/`),
  professional services, education credentials.
- **International federation.** Operators in EU, APAC run
  parallel networks under QDP-0013 federation; reputations
  cross at controlled discount.

## 9. Files in this dossier

- [`README.md`](README.md) — this strategic case (you are here).
- [`architecture.md`](architecture.md) — system design, data
  flow, governance, federation, the rating algorithm at a
  conceptual level.
- [`implementation.md`](implementation.md) — concrete code paths
  for sites, reviewers, and observers; widget integration;
  validation Worker plumbing; OIDC bootstrap; Schema.org
  interop.
- [`threat-model.md`](threat-model.md) — attack catalog mapping
  the seven gaming patterns above plus Quidnug-specific
  adversaries to mitigations and residual risk.

## 10. Related material

- [`../../examples/reviews-and-comments/PROTOCOL.md`](../../examples/reviews-and-comments/PROTOCOL.md)
  QRP-0001 wire spec.
- [`../../examples/reviews-and-comments/algorithm.md`](../../examples/reviews-and-comments/algorithm.md)
  Reference rating algorithm.
- [`../../examples/reviews-and-comments/bootstrap-trust.md`](../../examples/reviews-and-comments/bootstrap-trust.md)
  How new reviewers build initial trust.
- [`../../examples/reviews-and-comments/demo/`](../../examples/reviews-and-comments/demo/)
  Working end-to-end demo against a live node.
- [`../../docs/design/0023-dns-anchored-attestation.md`](../../docs/design/0023-dns-anchored-attestation.md)
  QDP-0023 DNS-anchored identity attestation.
- [`../../docs/design/0012-domain-governance.md`](../../docs/design/0012-domain-governance.md)
  QDP-0012 domain governance.
- [`../../docs/design/0013-network-federation.md`](../../docs/design/0013-network-federation.md)
  QDP-0013 network federation model.
