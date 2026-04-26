# Design brief: visualizing trust-weighted reviews

> A package for a human designer. Describes everything a
> Quidnug review surface might need to communicate, the
> tensions involved, and six different design directions to
> react to or innovate beyond. The goal is not to lock you
> into our existing visual primitives (the aurora /
> constellation / trace family in
> [`rating-visualization.md`](rating-visualization.md)) but
> to surface the underlying problem so you can propose better.

This brief is structured for a 90-minute first read followed
by repeated reference. Skim parts 1-3 for context, study 4-6
for opinions to react to, return to 7-8 when designing
specific surfaces.

---

## Part 1: What is being communicated, in one paragraph

A Quidnug review carries more information than a five-star
rating ever has: the same product can show different ratings
to different observers (because they trust different people),
ratings are scoped to topics (Bob is trusted on cameras, not
restaurants), reviews carry credentials (the reviewer's
DNS-validated identity, helpfulness history, sponsorship
disclosure), trust flows transitively (you trust Bob; Bob
trusts Alice; her review counts at a decay), and confidence
is graph-shaped (a rating built from 50 paths through your
trust network is more solid than one from 3). The design
question is: how do we show all of that without making
people's eyes glaze, while keeping the familiar five-star
glanceability when that's all the user wants.

## Part 2: The complete data inventory

Every data point that might appear on some Quidnug review
surface. Designers pick subsets per surface; not every
surface shows everything. Group by category:

### 2A. The rating itself

| Field                          | Type              | Notes                                                                                         |
|--------------------------------|-------------------|-----------------------------------------------------------------------------------------------|
| Per-observer rating            | float, 0 to 5     | The headline number for this observer. Differs from anonymous baseline.                       |
| Anonymous baseline rating      | float, 0 to 5     | The operator-rooted weighting any visitor sees without a wallet. Used as comparison anchor.   |
| Personalization delta          | float, signed     | Per-observer minus baseline. "0.4 higher than crowd," "0.7 lower than crowd."                 |
| Confidence                     | 0-100% or label   | How many distinct trust paths fed this number. A function of contributor count and weight.    |
| Polarization                   | low / mid / high  | Standard deviation of contributor ratings, weighted. "Trusted sources agree" vs "split."      |
| Distribution histogram         | array of floats   | The 1,2,3,4,5-star buckets, weighted by observer trust.                                       |
| Recency profile                | timeline data     | When the underlying reviews were written; how much weight the recent vs. old reviews carry.   |

### 2B. Topical context

| Field                          | Type              | Notes                                                                                         |
|--------------------------------|-------------------|-----------------------------------------------------------------------------------------------|
| Topic domain                   | string            | "reviews.public.technology.cameras" — the technical name.                                      |
| Topic display name             | string            | "Cameras" or "Restaurants in NYC" — human-readable.                                           |
| Topic ancestors                | array of strings  | "Technology > Cameras > DSLR" breadcrumb.                                                     |
| Topic age                      | duration          | When this topic tree was first registered.                                                    |
| Sibling topics                 | array             | What other topics this product could plausibly fit under.                                      |

### 2C. Reviewer credentials

| Field                          | Type              | Notes                                                                                         |
|--------------------------------|-------------------|-----------------------------------------------------------------------------------------------|
| Reviewer display name          | string            | "Alice (alice-eats.com)" or pseudonym.                                                        |
| Reviewer quid                  | hex 16            | Compact form for power users; usually folded behind display name.                             |
| Validated domain               | string or null    | "alice-eats.com" if Pro/Business validated, null otherwise.                                   |
| Validation tier                | enum              | none / Free / Pro / Business / Partner.                                                       |
| KYB status                     | bool              | Legal entity verified via Stripe Identity etc. Business+ only.                                 |
| OIDC provider                  | string or null    | "Google" / "GitHub" if onboarded via OIDC bridge.                                              |
| Reviewer tenure                | date              | First review on the network.                                                                  |
| Reviews in this topic          | integer           | How many reviews they've published under this topic root.                                     |
| Helpfulness ratio              | 0-1               | (helpful votes received from trusted observers) / (total received).                            |
| Last activity                  | duration ago      | "active 4 days ago" / "dormant 2 years."                                                       |
| Reputation tier                | enum              | Computed from above. "Top 1% in cameras" / "established" / "new."                              |
| Cross-imported history         | array             | "247 reviews imported from Amazon (verified by ...)" — past life.                              |

### 2D. Trust path explanation

| Field                          | Type              | Notes                                                                                         |
|--------------------------------|-------------------|-----------------------------------------------------------------------------------------------|
| Trust source                   | enum              | direct / 2-hop / 3-hop / operator-baseline / OIDC-baseline.                                   |
| Path summary                   | string            | "You trust Bob, Bob trusts Alice on cameras."                                                 |
| Path length                    | integer           | 1, 2, 3+.                                                                                     |
| Path weight                    | 0-1               | Effective trust weight after decay.                                                           |
| Alternate paths                | integer           | "and 4 other paths reach Alice from your graph."                                              |
| Highest-weight intermediary    | reviewer          | The most-trusted-by-you person who vouches for them.                                          |

### 2E. Single review attributes

| Field                          | Type              | Notes                                                                                         |
|--------------------------------|-------------------|-----------------------------------------------------------------------------------------------|
| Review timestamp               | ISO datetime      | When written.                                                                                 |
| Review rating                  | float 0-5         | The reviewer's score.                                                                         |
| Review title                   | string            | Optional headline.                                                                            |
| Review body                    | markdown          | The text.                                                                                     |
| Review media                   | array             | Photos, video, audio, with provenance signatures.                                             |
| Verified purchase              | bool              | Backed by a PURCHASE attestation from a validated site.                                        |
| Sponsorship disclosure         | object or null    | Sponsor, payment, terms. Mandatory for sponsored reviews.                                     |
| Edit history                   | array             | Append-only edits with timestamps.                                                            |
| Helpfulness votes              | object            | { trustedYes, trustedNo, anonymousYes, anonymousNo }                                          |
| Flag state                     | object or null    | { flagger, reason, weight } if any trusted moderator flagged.                                  |
| Reply thread                   | nested            | Comments, debates, reviewer's responses to questions.                                         |

### 2F. Network and operational state

| Field                          | Type              | Notes                                                                                         |
|--------------------------------|-------------------|-----------------------------------------------------------------------------------------------|
| Event published timestamp      | ISO datetime      | On-network publish time (may differ from review timestamp).                                   |
| Event finality                 | enum              | tentative / trusted (per QDP block tier).                                                     |
| Propagation                    | "N of M nodes"    | How widely gossiped; useful for very fresh events.                                            |
| Last refresh                   | duration ago      | When the rendering UI last polled.                                                            |
| Source seed                    | string            | Which seed served the data (for transparency, optional).                                      |

### 2G. Product Title metadata

| Field                          | Type              | Notes                                                                                         |
|--------------------------------|-------------------|-----------------------------------------------------------------------------------------------|
| Canonical name                 | string            | "Sony A7 IV"                                                                                  |
| Identifiers                    | object            | { asin, ean, upc, isbn, schemaOrgUrl }                                                        |
| Product Title quid             | hex 16            | Deterministic from identifiers.                                                               |
| Manufacturer / publisher       | quid + name       | If known.                                                                                     |
| Image                          | URL               | Schema.org-style hero image.                                                                  |
| Locale                         | string            | "en-US"                                                                                       |

### 2H. Action affordances

| Action                         | Required state          | Notes                                                                                  |
|--------------------------------|-------------------------|----------------------------------------------------------------------------------------|
| Vote helpful / not helpful     | observer is signed in   | Signed event. Should feel weightless to perform.                                       |
| Write a review                 | observer is signed in   | Opens compose UI.                                                                      |
| Trust this reviewer (manually) | observer is signed in   | Adds an explicit TRUST edge.                                                           |
| Mute this reviewer             | observer is signed in   | Local-only; doesn't publish on-chain.                                                  |
| Flag for moderation            | observer is signed in   | Publishes FLAG event; effects scoped to people who trust the flagger as moderator.     |
| Trust drill-down               | always                  | "Why this rating?" Shows the path explanation.                                         |
| Compare to anonymous           | always                  | "What does the crowd think?" Toggle.                                                   |
| Subscribe to reviewer          | observer is signed in   | Notifies on new reviews from this reviewer.                                            |
| Share / quote                  | always                  | Permanent link to a specific review event.                                             |
| Report to support              | always                  | Out-of-band: contact for legal takedown.                                               |

## Part 3: Audiences and surfaces

The same data renders very differently across surfaces and
audiences. Cross-reference matrix:

| Surface                                  | Anonymous reader     | Signed-in reader      | Reviewer       | Site operator     |
|------------------------------------------|----------------------|-----------------------|----------------|-------------------|
| S1. Inline rating on product page        | baseline only        | per-observer + delta  | own visible    | aggregate metrics |
| S2. Reviewer card (in any review)        | name + tier          | + your trust path     | own card       | flagging tools    |
| S3. Single review card (full)            | full                 | + your relevance      | + edit/respond | + moderation      |
| S4. Trust drilldown (why this rating?)   | n/a (sign in to use) | full graph view       | own incoming   | n/a               |
| S5. Browser-extension overlay            | baseline             | per-observer          | n/a            | n/a               |
| S6. Review compose / submit form         | n/a                  | n/a                   | full           | n/a               |
| S7. Reviewer profile page                | public history       | + your trust          | own profile    | + moderation      |
| S8. Email / push notification            | n/a                  | new from you-trust    | new responses  | site activity     |
| S9. Search-engine rich result            | aggregate static     | n/a                   | n/a            | n/a               |
| S10. Comparison view (multi-product)     | side-by-side         | per-observer columns  | own across     | n/a               |
| S11. Site operator dashboard             | n/a                  | n/a                   | n/a            | KPIs, moderation  |

This matrix tells the designer where to invest. S1, S3, S5
are the highest-volume surfaces (every page view). S2 is on
every review. S4 is the surface where the novelty has to land.
S6 is where the writer experience differentiates Quidnug from
"just leave a star." S11 monetizes; treat as a secondary
project.

## Part 4: Six design tensions

These are the trade-offs the designer must resolve. Some
combinations of choices conflict; the designer should pick a
coherent stance.

### Tension 1: Richness vs. simplicity

The data inventory above is huge. Showing all of it is
overwhelming. Hiding all of it loses Quidnug's whole value
proposition (the user might as well see Amazon's average).

**Two extremes:**
- **Minimal:** Show one number, no novelty, hide everything else
  behind a long-press / expander.
- **Maximal:** Show personalization delta, confidence,
  topical context, and reviewer credential at a glance.

**Healthy resolution:** progressive disclosure. The headline
is glanceable in 1 second; one tap reveals trust path; a
second tap reveals the full constellation; the data dashboard
is for power users.

### Tension 2: Novelty vs. familiarity

Per-observer ratings are genuinely new. People expect "5
stars on Amazon means something universal." Quidnug is saying
no, your 5 stars and my 5 stars are different. This is true
but cognitively expensive.

**The risk:** "Why does this say 4.1 for me but my friend
sees 4.6? This site is broken."

**Mitigation paths:**
- Always render the anonymous baseline alongside the
  personalized rating, with explicit labels ("for you" vs.
  "everyone").
- One-tap "what does the crowd think?" toggle.
- An onboarding moment the first time a logged-in user sees a
  rating that differs significantly from the baseline:
  inline tooltip explaining "we computed this from your trust
  graph. Tap to see why."

### Tension 3: Trust signal vs. crypto-anxiety

We can show "validated by Quidnug, KYB by Stripe, signed at
block 1234567." That's accurate. It's also off-putting to
non-technical users who may pattern-match to "crypto thing,
suspicious."

**Resolution:** translate cryptographic facts into trust
language. "alice-eats.com (Pro tier, since 2026)" is the
same fact as "TRUST edge from operators.alice-eats.com.network.quidnug.com
at level 0.95 issued 2026-04-12." Hide the second; show the
first; let power users opt into the technical view.

### Tension 4: Universality across verticals

The same component must work for cameras, restaurants,
healthcare professionals, and books. Each vertical has
different review mores: restaurants emphasize photos, books
emphasize spoilers, healthcare emphasizes credentialing.

**Resolution:** vertical-specific theming layered on a
universal core. The core component contracts (data shape,
interaction pattern) is the same; the chrome (which fields
are surfaced, accent colors, microcopy) varies.

### Tension 5: Embedded in hostile contexts

The browser extension overlay sits on top of Amazon's UI in a
sliver of viewport space. The WordPress widget sits on a page
the site owner styled. The widget cannot demand 800px or a
specific font. It must be opinionated enough to be glanceable
but pliable enough to coexist.

**Resolution:** strict CSS isolation (Shadow DOM); design at
multiple sizes (`nano`, `compact`, `standard`, `large`); make
typography intrinsically robust (system font stack with
explicit fallbacks); avoid color reliance for critical signal.

### Tension 6: SEO and accessibility floor

A search-engine crawler needs to see a number, not an SVG-
animated aurora. A screen-reader user needs to navigate the
review without understanding visual constellation graphs. A
color-blind user needs the rating signal in shape, not just
hue.

**Resolution:** every visual primitive emits a Schema.org
JSON-LD aggregate underneath; every visual signal has a
text/shape redundant signal; use ARIA roles and live regions
appropriately; render meaningful content even with CSS
disabled.

## Part 5: Six design directions

Each direction is a coherent point of view. The designer can
pick one wholesale, mix two, or invent a seventh. Each comes
with an exemplar visual metaphor, what it elevates, what it
suppresses, and where it shines.

### Direction A: "Five stars, plus one number"

**Metaphor:** A star rating, but with a small chip beside it
showing how it's been adjusted for you.

**Elevates:** familiarity, glanceability, universality with
existing review UIs.

**Suppresses:** the trust graph structure, the topical
scoping, the credentialing.

**Concept:**

```
      ★★★★☆   4.3
              for you  (+0.4)
              [why?]
```

Tap "why?" expands a small panel showing the most influential
reviewers and their paths.

**Strengths:**
- Zero learning curve. Existing users immediately get it.
- Drops cleanly into Amazon-shaped product pages.
- Schema.org compatible by default.

**Weaknesses:**
- Doesn't communicate Quidnug's full value at a glance.
- "for you" might be glossed over as a minor adjustment.
- Hard to express confidence or polarization in this idiom.

**Where it shines:** the browser-extension overlay (S5),
where space is tight and the user already understands stars.
The first-page-render in the WordPress widget for sites whose
audiences are not crypto-curious.

### Direction B: "Graph native"

**Metaphor:** show the trust network as a literal small graph.
The reviewer is at the center; their trust paths to you are
visible lines.

**Elevates:** the network structure, the topical relevance,
the "why" of personalization.

**Suppresses:** simple legibility, fast scanning, crowd
context.

**Concept:**

```
          you ●
           │
        Bob ● ────► trusts (cameras: 0.85)
           │
        Alice ●      <-- this reviewer
        ★★★★☆ 4.5 stars
        "Crisp colors, slow autofocus..."
```

In a more compact form, this becomes a small inline diagram:

```
 you → Bob → Alice (0.85 × 0.8 = 0.68 effective)
```

**Strengths:**
- Communicates the genuinely-novel mechanic directly.
- Educational the first time a user sees it.
- Aligns with the "constellation" primitive in
  rating-visualization.md.

**Weaknesses:**
- Cognitively heavy on every glance, not just the first.
- Doesn't scale well on mobile or in tight space.
- Anonymous users see no graph; need a fallback rendering.

**Where it shines:** the trust drilldown surface (S4), the
reviewer card on hover/focus (S2 expanded).

### Direction C: "Bloomberg terminal"

**Metaphor:** dense grid of small numerical and graphical
indicators, like a financial trading dashboard. Each cell is
a fact.

**Elevates:** total information density. Power users, data-
oriented audiences (B2B, financial reviews, professional
research).

**Suppresses:** approachability, brand warmth, anything that
feels human.

**Concept:**

```
┌───────────────────────────────────────────────────────┐
│ Sony A7 IV                                             │
│ ───────────────────────────────────────                │
│ For you   4.32  Δ +0.31  conf 78%   pol low           │
│ Crowd     4.01  ─       conf 92%   pol mid            │
│ ───────────────────────────────────────                │
│ 247 reviews · 14 trusted · 3.1y oldest · 4d newest     │
│ ▂▃▄▆█  histogram (weighted for you)                    │
│ ───────────────────────────────────────                │
│ Top voices    weight   trust path                       │
│ alice-eats     0.68    you→Bob→Alice                    │
│ camerareview   0.51    you→guild→camerareview           │
│ jdoe.eth       0.34    operator-baseline                │
│ ───────────────────────────────────────                │
│ Source: api.quidnug.com  refreshed 8s ago              │
└───────────────────────────────────────────────────────┘
```

**Strengths:**
- Power users will love it.
- Surfaces every fact above without hiding anything.
- Differentiates Quidnug as serious infrastructure.

**Weaknesses:**
- Hostile to casual readers.
- Fails on mobile.
- Reads as cold.

**Where it shines:** the site operator dashboard (S11), the
expert-witness reviewer profile (S7 power-user mode), the
desktop browser extension's expanded panel.

### Direction D: "Narrative voice"

**Metaphor:** sentences instead of numbers. The interface
reads like a friend explaining the rating.

**Elevates:** approachability, low number-anxiety, accessible
to non-numerate users.

**Suppresses:** at-a-glance comparison, terseness for power
users, scanability of long lists.

**Concept:**

```
   Five reviewers you trust have rated this
   camera. Most of them (Alice, Bob, Camille)
   give it 4 to 5 stars; one is more cautious
   (Dan: 3 stars). For you, the camera averages
   to a 4.3. The crowd at large gives it 4.0.

   See the reviews →
```

A shorter form for inline use:

```
   For you: 4.3 ★ (5 trusted reviewers, mostly positive)
```

**Strengths:**
- Communicates personalization without making the user think
  about it.
- Inclusive of users who don't parse charts.
- Plays well with screen readers natively.

**Weaknesses:**
- Verbose. Slow to scan a page of products.
- Localization-heavy: every string has to be carefully
  translated.
- Hard to convey nuanced facts like polarization compactly.

**Where it shines:** review summaries on email digests (S8),
mobile (smaller screen rewards tighter prose), the
"why this rating?" first-time-user moment (S4 onboarding).

### Direction E: "Trust paths as the headline"

**Metaphor:** the chain of who-trusts-whom is the primary
visual element; the number is secondary.

**Elevates:** the social proof structure that distinguishes
Quidnug from anonymous-crowd platforms.

**Suppresses:** the rating value itself.

**Concept:**

```
   ┌──────────────────────────────────────────┐
   │  via Bob (cameras 0.85) and 3 others      │
   │  ★★★★☆  4.3 for you                       │
   └──────────────────────────────────────────┘
```

Or, in expanded form:

```
   You trust 7 of these reviewers (4 directly,
   3 via Bob, 1 via the Photographers Guild).
   They give this camera 4.3 stars on average.

     directly: ●●●●         (Alice, Bob, ...)
     via Bob:  ●●●           (Camille, ...)
     via guild:●              (Frank)
```

**Strengths:**
- Most explicit about the "why you can trust this number"
  question.
- Differentiates Quidnug visually from any other review
  system.
- The path summary is intrinsically compelling: "your friend's
  friend recommends this."

**Weaknesses:**
- Requires the user already has a trust graph; degrades on
  cold start.
- Privacy considerations: showing "via Bob" reveals Bob is in
  the user's trust graph to anyone watching their screen.
- Names take space; needs careful handling for reviewers with
  long display names.

**Where it shines:** the reviewer card (S2), the trust
drilldown (S4), the comparison view (S10) where path
divergence is the interesting story.

### Direction F: "Ambient signal" (the existing aurora family)

**Metaphor:** abstract visual primitives that encode multiple
dimensions in shape, color, and motion without any explicit
text.

**Elevates:** distinctive visual identity, multi-dimensional
encoding in a small space, emotional/affective signal.

**Suppresses:** explicit numerical comparison, text-search
crawlability, immediate legibility for new users.

**Concept** (matches the existing rating-visualization.md):

```
   ◉                aurora: dot color = rating,
  ◌◌◌               ring thickness = confidence,
                    ring dash = directness,
                    chip = personalization delta

  ●  ◐  ◑           constellation: bullseye of trust tiers,
  ◌  ◐               one dot per contributor,
                    color = their rating,
                    size = their weight

  you → Bob → Alice  trace: the path itself
```

**Strengths:**
- Aesthetic, distinctive, brand-defining.
- Encodes multi-dimensional data in a small footprint.
- Already specified and partially implemented.

**Weaknesses:**
- Steep learning curve (what does the ring dash mean?).
- Requires legend or hover tooltips for first-time users.
- Reads as "abstract crypto thing" if not done carefully;
  brand risk.

**Where it shines:** the brand mark of Quidnug-powered
reviews, hero ratings on product detail pages, marketing
material. Less effective in dense lists or unfamiliar
contexts.

### Mixing directions

The strongest design probably mixes 2-3 directions across
surfaces:

- Direction A on the browser-extension overlay (familiar
  context).
- Direction F as the brand mark in product detail hero areas.
- Direction B/E in the trust drilldown (where we have permission
  to be educational).
- Direction D in email digests and mobile-compact.
- Direction C only on the operator dashboard.

A coherent system designer might build one underlying token /
layout system that can render all five and let the consumer
pick the mode per context.

## Part 6: Surface-by-surface treatment

Wireframes for the eight high-priority surfaces. ASCII-only;
they describe layout and information hierarchy, not visual
style. The designer's job is to translate.

### S1. Inline product-page rating

Placed in product card on listing pages, hero area on detail
pages. Three sizes: nano (grid), compact (list), large (hero).

Mixed Direction A + F:

```
NANO (≤ 80px wide; product grid card)
─────────────────────
 ◉ 4.3
 ─────────────────────
 (one aurora dot, one number, no text)

COMPACT (≤ 240px wide; list view)
─────────────────────
 ◉ 4.3 ★  for you  +0.4
 (247 reviews · 14 you-trust)
 ─────────────────────

LARGE (full-width hero on detail page)
────────────────────────────────────────────
 ◉◌                 4.3
                    For you (Sony A7 IV)
                    +0.4 vs. crowd · 78% confidence

 247 reviews · 14 from people you trust · 3.1y of history
 ▂▃▄▆█  weighted-for-you distribution
 [why this rating?] [see crowd view] [write a review]
────────────────────────────────────────────
```

### S2. Reviewer card

Appears within every review and in reviewer-profile callouts.

Direction E (trust path as headline) + A (familiar bottom):

```
COMPACT (inside a review)
──────────────────────────────────────
 alice-eats.com ✓Pro · KYB
 Restaurants in NYC · 247 reviews · 4d ago
 You trust her: via Bob (0.68 effective)
──────────────────────────────────────

EXPANDED (on hover or in profile preview)
─────────────────────────────────────────────────────
 alice-eats.com  ✓ Pro-tier  ✓ KYB verified
 ─────────────────────────────────────────
 247 reviews · helpful 87% · active 4d ago · since 2026
 Topics: Restaurants (NYC, JP), Books (food writing)
 ─────────────────────────────────────────
 Trust paths from you:
   direct: not yet (you can [trust] her)
   via:   Bob (you 0.9 → Bob 0.85 → Alice = 0.68)
          Photographers Guild (you 0.7 → guild 0.6 → Alice = 0.42)
 ─────────────────────────────────────────
 [trust]  [follow]  [her profile]  [mute]
─────────────────────────────────────────────────────
```

### S3. Single review card (full)

The atomic unit of content. Most-viewed surface in the system.

Direction A + small chips for context:

```
─────────────────────────────────────────────────────
 alice-eats.com  ✓Pro KYB  ★★★★½  4.5
 Restaurants in NYC · 4 days ago · verified meal · sponsored: no
 ─────────────────────────────────────────
 # Crisp acidity, careful service
 
 The chef's tasting menu is built around seasonal
 brassicas, and the wine pairings show genuine wit
 (a 2018 Soave with the cabbage course made me smile).
 
 Service was attentive without hovering; we were 
 there 2.5 hours and never felt rushed.
 ─────────────────────────────────────────
 [📷 4 photos] [🎧 1 audio note]
 ─────────────────────────────────────────
 87 found this helpful (12 from people you trust)
 [helpful] [not helpful] [reply] [share]
 ─────────────────────────────────────────
 You trust this reviewer via Bob and 2 others.
 [why this rating?]
─────────────────────────────────────────────────────
```

Note the explicit "sponsored: no" line: the disclosure status
is always present, never absent. A sponsored review reads
"sponsored: yes (acmebrand.com paid $200 · disclosed)".

### S4. Trust drilldown ("why this rating?")

The surface where the novelty has to land. Modal or full-page.

Direction B + E + interactive graph:

```
─────────────────────────────────────────────────────
 Why your Sony A7 IV rating is 4.3 (vs crowd's 4.0)
 ─────────────────────────────────────────
 14 reviewers contribute weight from your view:
 
 [interactive graph here]
 
   you ●─────────► Bob ●────────► Alice ●  4.5★
                       └────────► Camille ●  5.0★
                       
        ●──────► Photographers Guild ●─────► Dan ●  3.0★
                                       └───► Eve ●  4.5★
        
        ●──────► (operator-baseline) ───► 8 more ●●●●●●●●  avg 4.1★
 ─────────────────────────────────────────
 What moves the number for you:
   + 4 of your direct trust voices average 4.4
   + 3 reviewers via Bob average 4.6
   - 1 reviewer via Guild gave it 3.0 (Dan; he hates the autofocus)
 ─────────────────────────────────────────
 Compare to the crowd's 4.0 (everyone, equal weight)
 [show crowd view]   [adjust your trust]
─────────────────────────────────────────────────────
```

### S5. Browser-extension overlay

Lives on top of Amazon, Yelp, Google Maps. Tight space. Must
be glanceable in <2 seconds. Direction A core.

```
COLLAPSED (~60×24px next to existing site rating)
──────────────────────────
 ◉ 4.3 for you  ▾
──────────────────────────

EXPANDED (~280×360px popover when clicked)
────────────────────────────────────
 Sony A7 IV
 ─────────────────────────
 Quidnug for you:  4.3 ★
 Quidnug crowd:    4.0 ★
 Amazon shows:     4.5 ★ (247 reviews)
                   ↑ 30% of these flagged suspicious
                     by Quidnug auditors

 14 reviewers in your trust graph contribute.
 Top: alice-eats (0.68w), Bob (0.55w)

 [see full breakdown on quidnug.com]
 [write a review here]
────────────────────────────────────
```

The "30% flagged suspicious" is the wedge: Quidnug overlays
audit data on top of Amazon's existing reviews, exposing the
fake-review tax that consumers pay implicitly.

### S6. Review compose form

Where reviewers do the work. Friction here equals fewer
reviews. Direction D narrative for the topic-picker; A for
star input.

```
─────────────────────────────────────────────────────
 Reviewing: Sony A7 IV
 Camera, made by Sony · ASIN B0BJZ123 · Schema.org match ✓
 ─────────────────────────────────────────
 Topic: Cameras (suggested)
 [cameras  ▾]  not this? [restaurants] [audio] [other]
 ─────────────────────────────────────────
 Your rating:
 ★★★★☆ ☆           4 of 5
 ─────────────────────────────────────────
 Title: [______________________________________]
 ─────────────────────────────────────────
 Review:
 ┌──────────────────────────────────────────┐
 │                                            │
 │                                            │
 │  Markdown supported                        │
 └──────────────────────────────────────────┘
 [📷 add photo] [🎧 add audio]
 ─────────────────────────────────────────
 Disclosure: 
 ( ) Not sponsored
 ( ) Sponsored or comped (you'll disclose details)
 ( ) I work for the maker / sell this product
 ─────────────────────────────────────────
 Sign with: alice-eats.com (Pro tier, signed-in via wallet)
 [post review]   [save draft]
─────────────────────────────────────────────────────
```

The disclosure radio is required (no default), forcing an
explicit answer. The "sign with" line shows reviewer who
they're publishing as; if multiple identities, dropdown.

### S7. Reviewer profile page

The professional reviewer's home. Direction A + C hybrid;
public power-user view.

```
─────────────────────────────────────────────────────
 alice-eats.com  ✓Pro  ✓KYB  since 2026-04-12
 ─────────────────────────────────────────
 247 reviews · helpfulness 87% · active 4 days ago
 ─────────────────────────────────────────
 Topics:
   Restaurants (NYC, JP)         182 reviews   avg 4.1
   Books (food writing)           41 reviews   avg 4.4
   Travel (food destinations)     24 reviews   avg 3.8
 ─────────────────────────────────────────
 Trust from you:
   You don't directly trust Alice.
   You reach her via Bob (cameras: 0.85) at 0.68 weight.
   [trust directly]
 ─────────────────────────────────────────
 Recent reviews
   ★★★★☆  Le Bernardin — 2026-04-21
   ★★★★½  Joji Wine Bar — 2026-04-15
   ★★★☆☆  Pinch Chinese — 2026-04-08
   [more →]
 ─────────────────────────────────────────
 Cross-imported history:
   247 reviews from Yelp (verified by alice-eats.com 2026-03-15)
 ─────────────────────────────────────────
 [follow]  [trust on a topic]  [send tip]  [contact]
─────────────────────────────────────────────────────
```

The "send tip" reflects the new economic role: pay
professional reviewers directly.

### S8. Email digest

Direction D. Once a week or per-event.

```
Subject: This week in your trust graph

   Bob (someone you trust on cameras) wrote a 5-star
   review of the Sony A7 IV. He notes the autofocus
   has improved noticeably in firmware 1.20.
   → Read

   Three reviewers you trust on restaurants visited
   spots in Tokyo last week. Average rating: 4.4.
   Top spot: a 9-seat counter in Yoyogi-Uehara.
   → See all three

   The Photographers Guild added Camille (camille.photo)
   as a member; you transitively trust her now (0.42
   effective). 47 of her past reviews are now visible
   in your feed.
   → Welcome Camille

[manage subscriptions]  [trust settings]  [unsubscribe]
```

## Part 7: Component primitives the designer should produce

Regardless of which direction(s) the designer picks, these
atomic components recur across surfaces. Build them once,
reuse everywhere.

| Component                         | Purpose                                                                                | Variants                          |
|-----------------------------------|----------------------------------------------------------------------------------------|-----------------------------------|
| **Rating mark**                   | Single visual representation of one rating value                                       | nano, compact, standard, large    |
| **Personalization chip**          | "+0.4 for you" / "for you" / "everyone" indicator                                       | positive, negative, neutral       |
| **Confidence indicator**          | How solid the underlying graph is                                                       | low, mid, high; or % bar          |
| **Polarization indicator**        | Whether the contributors agree                                                          | low spread, high spread           |
| **Trust path summary**            | "via Bob (0.68)" or "directly" or "operator baseline"                                   | direct, transitive, baseline      |
| **Reviewer credential badge**     | "Pro" / "Business" / "KYB" / "OIDC" tier with tooltip explaining                       | tier-specific colors              |
| **Disclosure ribbon**             | "sponsored: yes/no" prominently placed on every review                                  | mandatory presence                |
| **Verified-purchase mark**        | Distinct from validation tier; about a specific PURCHASE attestation                    | validated-source / unvalidated    |
| **Topic breadcrumb**              | "Technology > Cameras > DSLR" or "Restaurants in NYC"                                  | full, abbreviated                 |
| **Helpfulness counter**           | Trusted-by-you / total split                                                            | with optional "(N from your trust)" |
| **Flag indicator**                | When a moderator you trust has flagged                                                  | warning, hidden-by-default         |
| **Reviewer name lockup**          | Display name + validated domain + tier badge in canonical layout                       | full, compact                     |

## Part 8: Open questions for the designer

We don't yet have answers; your perspective is welcome.

1. **First-encounter onboarding.** When a logged-in user sees
   a rating that differs noticeably from the anonymous
   baseline for the first time, do we surface that
   actively (modal, banner) or wait for them to ask? Both
   options have failure modes.

2. **Trust-path privacy.** Showing "via Bob" reveals that Bob
   is in the user's trust graph to anyone with shoulder-view
   access. Acceptable trade-off, or a privacy issue we should
   degrade gracefully (show "via 3 trusted contacts" without
   names by default)?

3. **Polarization signal weight.** A 4.3 rating where
   contributors range 1-5 is meaningfully different from a
   4.3 where they range 4-5. Should polarization be a
   first-class glanceable signal, or a power-user detail?

4. **Cross-vertical reviewer identity.** Alice writes
   restaurant reviews and tech reviews. Should her reviewer
   card render the same in both contexts, or specialize per-
   topic ("Alice on restaurants" vs "Alice on cameras")?

5. **Sponsored-review treatment.** Visually fold sponsored
   reviews into the main flow with a clear ribbon? Show in a
   separate "sponsored" tab? Hide unless the user opts in?

6. **Cold-start anonymous experience.** A first-time visitor
   with no wallet, no trust graph, no logged-in session sees
   only the anonymous baseline. How do we communicate
   "personalize this" without nagging?

7. **Mobile constraints.** A trust-path graph on a 320px
   screen is nearly unusable. Should mobile collapse to
   Direction D narrative? Or show a sparser variant?

8. **Aesthetic.** Quidnug's brand is currently "serious
   crypto-protocol vibes" (dark mode, monospace, technical).
   Does the consumer-facing review UI inherit that, or
   present a warmer, more approachable face?

9. **Density on operator dashboard.** Direction C is the most
   information-dense, but is it the right paradigm? Should
   the dashboard inherit from financial trading UIs, from
   developer tools (Datadog, Grafana), from CRM dashboards
   (Salesforce, HubSpot), or invent its own?

10. **Negative space.** Where do we leave room for things we
    haven't designed yet? Future-proof slots for: video
    reviews, debate threads, expert-witness CV exports, brand
    response letters.

## Part 9: References for the designer

Real-world precedents to draw from. Not for direct copying,
but to anchor expectations.

| Reference                          | What's relevant                                                                                |
|------------------------------------|------------------------------------------------------------------------------------------------|
| Substack post header               | Author + tier + paywall status in compact lockup. Influences reviewer card design.             |
| Stripe dashboard                   | Dense data without coldness. Color use restrained. Charts as supporting material.              |
| Linear's issue cards               | Information hierarchy in tight space. Hover-to-expand patterns.                                |
| Goodreads review                   | Long-form reading with reviewer credibility (followers, prior reviews).                        |
| Bloomberg terminal                 | Direction C exemplar.                                                                          |
| Apple App Store reviews            | Five stars + "verified purchase" + helpfulness. The shape consumers expect.                    |
| Wirecutter article                 | Editorial trust at the publication level; how to signal "this person is the real deal."        |
| Reddit comment trees               | Threaded reply patterns; voting that visibly affects ordering.                                 |
| Bandcamp purchase page             | Verified-purchase shown without crypto-aesthetic; warm.                                        |
| Tripadvisor sort/filter UI         | What we explicitly don't want (filter games, opinion lock-in).                                 |
| Google reviews                     | The brigading failure mode rendered at scale; what we're competing against.                    |
| Mastodon profile cards             | Federated identity rendered approachably; KYB-shaped verification badges done well.            |
| GitHub commit graph                | Visualizing a person's activity over time without overwhelming.                                |

## Part 10: Deliverables we'd love from the designer

Not a fixed list. Anything in this set, in this priority order,
moves us forward:

1. A coherent visual language proposal (1-2 directions or a
   hybrid). High-fidelity mocks of the headline rating
   primitive at all four sizes (nano, compact, standard,
   large).
2. Mocks of S1, S2, S3, and S4 in the chosen language.
3. Animated/interactive prototype of the trust drilldown
   (S4) showing the why-this-rating moment.
4. A component spec sheet covering the primitives in Part 7
   with tokens (color, type, spacing) and accessibility notes.
5. Mobile variants of S1 and S3.
6. Browser-extension overlay (S5) at expanded and collapsed.
7. Email digest template (S8) with copywriting hints.
8. Provocative reactions to Part 8's open questions: where
   you'd push back on our framing.

Innovation beyond this brief is welcome. If you see the
problem fundamentally differently, the brief is wrong, not
you.

---

## Appendix: glossary

For the designer's reference.

| Term                  | Meaning in this brief                                                            |
|-----------------------|----------------------------------------------------------------------------------|
| Quid                  | A reviewer's cryptographic identity (16 hex chars). User never types it.         |
| Trust edge            | A signed assertion that one quid trusts another at level 0-1 in a topic.         |
| Topic / domain        | Hierarchical category like `reviews.public.technology.cameras`.                   |
| Per-observer rating   | The rating a specific user sees, weighted by their trust graph.                  |
| Anonymous baseline    | The rating any visitor without a wallet sees; operator-rooted weighting.         |
| Personalization delta | Per-observer minus baseline. The "for you" adjustment.                           |
| Confidence            | A function of how many distinct trust paths fed the rating.                      |
| Polarization          | Spread of contributor ratings; "trusted sources agree vs split."                 |
| Trust path            | Chain of edges: you → Bob → Alice → review.                                       |
| Validation tier       | Quidnug LLC's commercial product. Free / Pro / Business / Partner.               |
| KYB                   | Know-your-business: legal-entity verification (Stripe Identity etc.).            |
| OIDC bridge           | "Sign in with Google" path that mints a custodial quid.                          |
| Validated domain      | A domain (alice-eats.com) the reviewer or site has cryptographically tied to.    |
| Disclosure            | First-class event type marking a review as sponsored, comped, or affiliated.     |
| PURCHASE attestation  | Signed event by a validated seller confirming an actual transaction occurred.    |
| Aurora / constellation / trace | Existing visual primitives (in `rating-visualization.md`); one option of many. |

End of brief.
