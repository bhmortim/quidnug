# Relativistic Ratings: The End of Review Spam and the Social Science of Personal Reputation

*Why the five-star average is a fiction, what six decades of
social-psychology research say about how trust actually
propagates, and how relativistic ratings solve review spam as a
side effect of getting the math right.*

| Metadata | Value |
|----------|-------|
| Date     | 2026-04-20 |
| Authors  | The Quidnug Authors |
| Category | Reviews, Reputation Systems, Social Science |
| Length   | ~8,900 words |
| Audience | Product leaders, trust and safety engineers, reputation system designers, anyone who has ever wondered why every Amazon product is rated 4.4 |

---

## TL;DR

The average Amazon product is rated 4.4 out of 5. The average
Yelp business is 3.8. The average Google Maps restaurant is 4.3.
These numbers are not describing reality. They are describing a
broken measurement instrument.

Three things are going on at once:

1. **Distributional bias.** Reviewers self-select into a
   J-shaped distribution (Hu, Pavlou, & Zhang, 2006; Dellarocas,
   2003). The median of the underlying quality distribution is
   not 4.4. The median of *who bothers to leave a review* is.
2. **Industrial-scale spam.** The fake review economy is worth
   roughly $152 million per year across Amazon alone (He,
   Hollenbeck, & Proserpio, 2022, NBER W29855). The US Federal
   Trade Commission finalized its "Trade Regulation Rule on the
   Use of Consumer Reviews and Testimonials" in 2024, making
   fake reviews an explicit federal violation. Enforcement is
   ongoing and insufficient.
3. **The global-average assumption.** Collapsing a heterogeneous
   population's opinions into one number throws away the fact
   that you, the specific observer, may legitimately weight
   some reviewers more than others.

The first two problems have been studied for two decades by the
reputation-systems research community (Resnick, Zeckhauser,
Friedman, & Kuwabara, 2000; Jøsang, Ismail, & Boyd, 2007). The
third is the one almost everyone gets wrong.

This post argues that **relativistic ratings**, which compute a
personalized score for each observer from their own trust graph,
solve review spam as a structural side effect of a better
mathematical model. The case is backed by six decades of social
psychology on how humans actually form trust (Asch 1951,
Granovetter 1973, Fukuyama 1995, Mayer/Davis/Schoorman 1995), by
peer-reviewed reputation-system theory, and by the mathematics
of Quidnug's four-factor rating formula (QRP-0001).

The second half of the post covers visualization: how do you
surface a personalized, multi-dimensional score in a UI without
overwhelming users? Quidnug ships three SVG primitives (`<qn-aurora>`,
`<qn-constellation>`, `<qn-trace>`) that display the same
underlying data at three different information densities, plus
composition patterns for assembling them into review interfaces
that work at every fidelity from "4.5 next to a product card" to
"full provenance audit for an expert buyer."

**Key claims this post defends:**

1. The global-average star rating is a statistically malformed
   summary of a process that produces reviews.
2. Review spam is economically rational under global averages
   and economically irrational under relativistic ratings. The
   math changes the attacker's cost function.
3. Six decades of social psychology agree with the relativistic
   model and disagree with the global-average model.
4. You can visualize relativistic ratings in a UI as
   efficiently as stars once you stop fighting the extra
   dimensions and start using them.

---

## Table of Contents

1. [The Five Stars Are Lying](#1-the-five-stars-are-lying)
2. [What Social Science Says About Trust](#2-what-social-science-says-about-trust)
3. [The Review Spam Economy](#3-the-review-spam-economy)
4. [Failure Modes of Global Ratings](#4-failure-modes-of-global-ratings)
5. [What is a Relativistic Rating?](#5-what-is-a-relativistic-rating)
6. [The Math: Four-Factor Rating](#6-the-math-four-factor-rating)
7. [Anti-Spam Properties](#7-anti-spam-properties)
8. [Visualization at Three Depths](#8-visualization-at-three-depths)
9. [Real-World Examples](#9-real-world-examples)
10. [Honest Tradeoffs](#10-honest-tradeoffs)
11. [Implementation Notes](#11-implementation-notes)
12. [Conclusion](#12-conclusion)
13. [References](#13-references)

---

## 1. The Five Stars Are Lying

### 1.1 The J-shaped distribution

Here is the first empirical fact you need to internalize. Online
review distributions are not bell-curved. They are J-shaped.

Hu, Pavlou, and Zhang's widely-cited 2006 paper "Can Online
Reviews Reveal a Product's True Quality?" [^hu2006] analyzed
Amazon review data and showed that the distribution of star
ratings is bimodal, dominated by 5-star and (to a lesser degree)
1-star reviews, with very few 2/3/4-star reviews. They called
it the "self-selection bias in online reviews."

```
Typical Amazon / Yelp / Google review star distribution

5 stars │████████████████████████████████████████  58%
4 stars │██████████                                 15%
3 stars │████                                        7%
2 stars │████                                        6%
1 star  │█████████                                  14%

Source: Hu, Pavlou, and Zhang (2006); Dellarocas (2003);
recent Luca Yelp dataset. Values vary by platform but the
J-shape is universal.
```

The math that follows from this distribution:

- **Mean rating:** ~4.0-4.5, depending on weight of the 1-star tail.
- **Median rating:** 5.
- **Mode:** 5.
- **Standard deviation:** large (~1.4), making ratings
  statistically indistinguishable for products within 0.3 stars
  of each other.

The mean rating is thus a statistic computed over a
non-representative sample, reported as if it described the
underlying population. This is equivalent to computing "the
average temperature in this room" by asking only the people
sitting next to the heater and the people sitting next to the
air conditioner. The number you get has a clear mathematical
definition; it has no epistemic value.

### 1.2 The academic consensus on this is twenty years old

Dellarocas (2003) [^dellarocas2003] framed online reviews as "a
radical new form of word of mouth" and predicted the self-
selection problem. Hu, Pavlou, and Zhang (2006) measured it.
Subsequent work has only sharpened the conclusions:

- **Dellarocas, Zhang, and Awad (2007)** [^dellarocas2007] showed
  the J-shape is stable across product categories and platforms.
- **Anderson and Magruder (2012)** [^anderson2012] found that a
  half-star increase in Yelp rating causes a 19% increase in the
  likelihood of a restaurant selling out during peak hours. The
  measurement is noisy; the consequences are real.
- **Luca (2016)** [^luca2016] showed a one-star Yelp increase
  causes a 5-9% increase in revenue for independent restaurants,
  and that the effect is strongest for restaurants near
  threshold levels (e.g., 3.5 vs 4.0).
- **Luca and Zervas (2016)** [^lucazervas2016] documented review
  fraud as a deliberate competitive strategy, not an isolated
  aberration.

Industry has known this for longer than some of the SDKs being
written to display star averages. The question is why star
averages persist.

### 1.3 Why star averages persist anyway

Three reasons:

1. **Legibility.** A star average fits in 30 characters. A
   distribution takes a chart. A relativistic rating takes
   deliberate design work.
2. **Schema.org compatibility.** Google rich results accept
   `AggregateRating.ratingValue`. Deviating from the global
   average forfeits SEO visibility.
3. **Vendor lock-in.** Platforms (Amazon, Yelp, Google, TripAdvisor)
   control both the distribution of reviews and the display of
   aggregates. They benefit from the opacity.

Quidnug's position: you can keep the SEO-friendly single number
(we do, via Schema.org JSON-LD with `AggregateRating`) while
showing humans a more useful picture. The two layers are not in
conflict once you accept that the single number is a simplified
artifact rather than a ground truth.

---

## 2. What Social Science Says About Trust

This section is the load-bearing one. I will walk through eight
well-trusted pieces of social-science research and show what
each implies for rating-system design. The short version: the
relativistic model is what social psychology has been telling us
to do since 1951.

### 2.1 Asch (1951, 1956): conformity and social proof

Solomon Asch ran the famous line-judgment experiments [^asch1951]
[^asch1956]. Participants were asked to match a target line to
one of three comparison lines. The correct answer was obvious.
But when seven confederates all gave the same wrong answer
before the participant, roughly 75% of participants conformed at
least once, and 32% conformed on the critical trials overall.

**Implications for ratings:**

- A 4.8-star aggregate is not just information; it is social
  pressure. Showing it to a new visitor *before* they form their
  own view biases them toward conformity with the existing
  majority, whether or not that majority is representative.
- Relativistic ratings mitigate this by framing the aggregate as
  "what your trusted reviewers say" rather than "what everyone
  says." The social proof moves from an anonymous crowd to
  people the observer has specifically chosen to weight.

Cialdini's "social proof" principle from *Influence* (1984)
[^cialdini1984] is effectively a packaging of Asch for marketers.
The takeaway is the same: social proof is powerful, and whose
social you accept as proof matters.

### 2.2 Festinger (1954): social comparison theory

Leon Festinger's social comparison theory [^festinger1954]
posits that humans evaluate themselves and their opinions by
comparing to similar others. Not anonymous crowds. **Similar
others.**

**Implications:**

- A global average is, from Festinger's framing, the wrong
  reference class. You want to compare opinions against people
  *similar to you*, not against a random sample of the internet.
- The relativistic model encodes "similar to you" operationally:
  a trust graph is precisely a formalization of whose opinions
  you weight.

### 2.3 Granovetter (1973): the strength of weak ties

Mark Granovetter's "The Strength of Weak Ties" [^granovetter1973]
is arguably the most influential paper in modern sociology. It
showed that weak social ties (acquaintances, loose professional
connections) carry more novel information than strong ties
(family, close friends), because they connect disjoint parts of
the social graph.

For job search specifically, Granovetter found 56% of jobs were
found through weak ties, 27% through intermediate, only 17%
through strong ties.

**Implications for ratings:**

- Trust decay should be graceful, not binary. The friend-of-a-
  friend you barely know might give you the most useful review,
  even if their weight is lower than your sister's.
- This is *exactly* what multiplicative decay on a trust graph
  encodes. A path of length 3 with weights 0.9, 0.7, 0.8 yields
  composite trust 0.504, non-zero but meaningfully lower than a
  direct edge at 0.9. Granovetter's insight becomes a formula.

```
Why Granovetter matters for trust propagation

Strong ties          Weak ties             Composite
(direct, high-weight) (multi-hop, decayed)  (weighted blend)

   0.9                   0.504                 0.72
    ●                     ●                     ●
    │                     │                     │
    │  most accurate      │  most novel        Multi-source
    │  for known goods    │  info              aggregation
    │                     │                     │
```

Granovetter is the reason Quidnug lets trust propagate through
5 hops with multiplicative decay rather than treating only
direct edges as valid. Social science says weak ties matter.

### 2.4 Milgram (1963): authority and small-world networks

Stanley Milgram's obedience studies [^milgram1963] are what most
people remember, but his "small world" experiment (1967) is what
matters here. Milgram showed that the average American was
connected to a randomly-chosen target by a path of length ~6
(the famous "six degrees"). Modern measurements on Facebook
data put the number at 3.57 (Bhagat et al., 2016 [^facebook2016]).

**Implications:**

- The distance from any observer to any reviewer is small. A
  trust-graph walk of depth 5 is not theoretical; it is
  empirically sufficient for almost any pair of observer and
  reviewer to have a path.
- Sybil attacks that require inserting a node *between* two
  honest parties become harder the denser the real social graph
  is. Six degrees of separation is a structural defense.

### 2.5 Mayer, Davis, and Schoorman (1995): integrative model of trust

The Mayer-Davis-Schoorman (MDS) integrative model [^mayer1995]
is the most-cited framework in organizational trust literature.
It decomposes trustworthiness into three components:

1. **Ability:** does the trustee have the competence to deliver?
2. **Benevolence:** does the trustee intend well toward the
   trustor?
3. **Integrity:** does the trustee adhere to principles the
   trustor finds acceptable?

Plus a personality variable: **propensity to trust** (how
willing the trustor is to trust at all).

**Implications for ratings:**

- "Trustworthy reviewer" is not a single dimension. The MDS
  model predicts that a reviewer who is competent (has rated
  many cameras) but has unknown intentions (new account) will
  be weighted differently than one who is less competent but
  has a strong history of honest behavior.
- This maps directly onto Quidnug's four-factor formula: `T`
  encodes ability-plus-domain, `H` encodes a signal that
  combines benevolence and integrity (helpfulness votes), `A`
  encodes demonstrated competence, `R` encodes recency as a
  proxy for ongoing commitment.

The four factors are not arbitrary; they operationalize a
30-year-old theoretical model.

### 2.6 Fukuyama (1995) and Putnam (2000): trust as social capital

Francis Fukuyama's *Trust* [^fukuyama1995] and Robert Putnam's
*Bowling Alone* [^putnam2000] argued that generalized trust is
an economic and civic good. Fukuyama: "Societies with high
levels of social trust organize themselves in a wider variety of
ways and have lower transaction costs." Putnam: declining social
trust tracks declining civic engagement.

Both works also argued that trust is **bounded by community.**
The default generalized-trust level varies enormously across
cultures (Fukuyama's canonical examples: Japan and the US high;
Italy and Latin America low). No universal rating system can
assume a flat trust baseline.

**Implications:**

- A rating system that assumes global trust homogeneity will
  under-weight reviews for populations with higher propensity
  to trust and over-weight reviews for populations with lower
  propensity. Relativistic ratings avoid this by making trust
  explicit and per-observer.

### 2.7 Resnick, Zeckhauser, Friedman, and Kuwabara (2000): reputation systems

The foundational CACM article "Reputation Systems" [^resnick2000]
laid out the engineering requirements. The authors (Paul Resnick,
Richard Zeckhauser, and colleagues at MIT and Harvard) argued
that reputation systems must:

1. Provide information for decision-making.
2. Give feedback to improve reputation.
3. Deter bad behavior.

They also identified the core failure modes:

- **Entry/exit attacks:** reputation is lost on account
  creation, creating incentives to burn and re-create accounts.
- **Sybil attacks:** one attacker, many pseudo-identities.
- **Whitewashing:** starting fresh after bad behavior.
- **Collusion:** coordinated positive or negative ratings.

**Implications:**

- Any rating system that does not structurally resist Sybil
  attacks is broken. Structural resistance means either (a)
  making identities expensive to create (Bitcoin, PoW), (b)
  anchoring identities to rare signals (OIDC, KYC), or (c)
  making Sybils *irrelevant* because nobody in the trust graph
  weights them. Relativistic ratings do (c).

### 2.8 Jøsang, Ismail, and Boyd (2007): survey of trust and reputation

Audun Jøsang's comprehensive survey [^josang2007] reviewed 40+
trust and reputation systems across the preceding decade. Their
conclusion: **transitive trust with graded decay is the most
general and expressive model.** Binary trust (trust/don't-trust)
loses information; numerical trust without decay does not handle
uncertainty well.

They also conclude: **context matters.** A reputation system
must be scoped to the context of the judgment. A good landscape
photographer is not necessarily a good portrait photographer.

**Implications:**

- Quidnug's topic-scoped trust domains (e.g.,
  `reviews.public.technology.laptops`) operationalize Jøsang's
  context requirement.
- The four-factor formula's topical-trust component `T` uses
  domain-scoped path computation with fallback decay when no
  direct topical edge exists.

### 2.9 Summary

| Finding | Paper | What it implies for ratings |
|---------|-------|------------------------------|
| Social proof biases judgment (32% conformity rate) | Asch 1951, 1956 | Show *trusted* social proof, not anonymous crowd proof |
| Humans evaluate by comparison to similar others | Festinger 1954 | Global averages pick the wrong reference class |
| Weak ties carry novel information | Granovetter 1973 | Multi-hop trust propagation with decay |
| Average network distance is small (~3-6) | Milgram 1967, Facebook 2016 | Depth-5 trust walks reach everyone |
| Trust decomposes into ability, benevolence, integrity | Mayer/Davis/Schoorman 1995 | Multi-factor rating with independent signals |
| Trust is bounded by community | Fukuyama 1995, Putnam 2000 | Global flat trust assumption is wrong |
| Reputation systems must resist Sybils | Resnick et al. 2000 | Structural Sybil resistance is mandatory |
| Transitive trust with context is the right model | Jøsang et al. 2007 | Domain scoping and graded decay |

Every one of these findings points to the same answer: **ratings
should be computed from the observer's own context-aware
weighted view of the people whose opinions they trust.** That is
the relativistic rating.

The five-star average is doing the exact opposite of everything
social science has learned about how trust actually works.

---

## 3. The Review Spam Economy

We need to talk about the empirical scale of review fraud. The
numbers are not small, and they are growing.

### 3.1 The measured market

He, Hollenbeck, and Proserpio (2022) [^hehollenbeck2022]
identified a functioning market for fake reviews operating via
private Facebook groups, WeChat channels, and dedicated broker
websites. Their findings, measured across Amazon product
categories:

- Sellers paid an average of $0-30 per fake review, with the
  product often reimbursed in full as part of the deal.
- Products that purchased fake reviews experienced a 12.5%
  short-term rating increase and a modest sales uplift.
- However, over the longer term (90+ days), the spike was
  followed by a rating decline as genuine buyers received lower-
  quality products than the fake reviews promised.
- Amazon's automated detection caught some but not all. Detected
  sellers faced higher review-fraud costs going forward.

The authors estimated the annualized expenditure on fake reviews
across Amazon alone at $152 million as of study date. More
recent surveys (Trustpilot's 2023 transparency report) put the
total cross-platform fake-review economy well over $500 million
annually.

```
Fake review ecosystem (schematic)

     ┌─────────────────────────────────────────────┐
     │   Review Broker / Matchmaker Services       │
     │   (Facebook groups, WeChat, websites)       │
     └─────────────────────────────────────────────┘
                  │                      │
                  ▼                      ▼
     ┌──────────────────────┐    ┌─────────────────────┐
     │  Sellers requesting  │    │  Reviewers offering │
     │  positive reviews on │    │  to post for free   │
     │  their products      │    │  product + cash     │
     └──────────────────────┘    └─────────────────────┘
                                      │
                                      ▼
                          ┌──────────────────────────┐
                          │  Amazon / Yelp / Google  │
                          │  (detection vs evasion   │
                          │  arms race)              │
                          └──────────────────────────┘
```

### 3.2 The academic literature on detection

Ott, Choi, Cardie, and Hancock (2011) [^ott2011] built
supervised classifiers for deceptive hotel reviews on TripAdvisor
data. Best classifier accuracy: ~89%. Sounds impressive; consider
what 89% means at scale.

If a platform hosts 100 million reviews and 10% are fake, that is
10 million fake reviews. A 89%-accurate classifier catches 8.9
million. That still leaves **1.1 million undetected fake reviews**
polluting the corpus. And the false-positive rate means ~9
million genuine reviews are incorrectly flagged.

Jindal and Liu (2008) [^jindal2008] found similar numbers on
Amazon. Their classifier achieved 78% accuracy. Better classifiers
have been built since, but the detect-vs-evade game is adversarial:
each round of defender improvement is followed by attacker adaptation.

**Meta-point:** supervised detection is a losing arms race. The
attackers iterate faster than the defenders' label-collection
pipelines. Structural defenses (making fake reviews useless to
the observer regardless of detection) scale much better than
probabilistic defenses (trying to filter fake reviews out after
they exist).

### 3.3 Astroturfing and review bombing

Mayzlin, Dover, and Chevalier (2014) [^mayzlin2014] compared
reviews on Expedia (where only verified-purchasing guests could
review) versus TripAdvisor (open to anyone). They found:

- Competitor-proximate listings on TripAdvisor had systematically
  lower ratings than on Expedia.
- The gap was largest for small independent hotels near large
  chain hotels (suggesting targeted negative astroturfing from
  the chains).
- Statistical significance: p < 0.001.

This is not a theoretical concern. Negative astroturfing is a
documented competitive tactic.

Review bombing (coordinated campaigns to tank a product's
rating for cultural or political reasons) gained prominence in
2018-2024 with campaigns targeting video games, books, and
films. Because review-bombing campaigns are often organized in
public spaces (Twitter, Reddit), their coordination is visible,
but the resulting rating damage is persistent.

### 3.4 The FTC rule and its limits

The US Federal Trade Commission finalized the "Trade Regulation
Rule on the Use of Consumer Reviews and Testimonials" in August
2024 (16 CFR Part 465). The rule prohibits:

- Buying fake reviews.
- Selling fake reviews.
- Suppressing negative reviews (platforms cannot edit or remove
  reviews based on sentiment alone).
- Misleading consumer testimonial use.

Penalties: up to $51,744 per violation.

**Why this is not enough:**

- The rule targets US jurisdiction; most fake-review brokers
  operate from China, Southeast Asia, and Eastern Europe.
- Per-violation fines assume platforms or sellers can be
  identified and sued, which is often infeasible.
- The rule addresses fraud but does not fix the statistical
  problem: even without fraud, the J-shape and global-average
  issues remain.

Regulation is a necessary hygiene layer, not a solution.

---

## 4. Failure Modes of Global Ratings

Let me enumerate the specific ways global rating systems fail.
Each one is documented in the literature; each one is structurally
impossible under a relativistic model.

### 4.1 Selection bias

Reviewers are not representative. Extremely happy and extremely
unhappy customers review; the indifferent majority does not.
Result: the J-shape.

### 4.2 Threshold effects

Anderson and Magruder (2012) showed that crossing the 4.0 Yelp
threshold has outsized revenue consequences. This creates
disproportionate incentives for fraud at threshold boundaries.
Relativistic ratings do not have a single threshold; they have a
personal threshold per observer. Fraud around a single numeric
target becomes ineffective.

### 4.3 Astroturfing

As discussed above (Mayzlin et al. 2014). Relativistic ratings
defeat astroturfing because a new fake reviewer has zero trust
from any observer's graph until socially validated.

### 4.4 Review bombing

Coordinated negative campaigns. Same defense: bombers are
strangers to the observer's trust graph and contribute near-zero
weight.

### 4.5 Filter bubbles, except backwards

Eli Pariser's *The Filter Bubble* (2011) [^pariser2011] and Cass
Sunstein's *Republic.com* (2001) [^sunstein2001] worry that
personalization isolates people in information silos.

These concerns are legitimate and deserve engagement. Two
responses:

1. **Global ratings already produce filter bubbles, just worse
   ones.** The 4.4-star average filter bubble includes reviewers
   whose opinions have no bearing on yours. Relativistic ratings
   at least let you *see* the reviewers whose opinions you are
   weighting.
2. **Quidnug's visualization primitives show opt-in pathways for
   broadening the trust graph.** The `<qn-constellation>`
   display includes a "crowd" tier, so observers can see what
   the global crowd thinks *alongside* their personalized
   rating. Users can consciously choose to weight the crowd
   tier up or down. This is fundamentally different from
   algorithmic recommendation systems that hide their scoring.

Filter bubbles become a problem when the filter is opaque.
Relativistic ratings make the filter transparent.

### 4.6 The "everyone is 4.4" equivalence class

When every product is rated between 4.0 and 4.7, the
discrimination power of the rating is zero. A fair analogy: if a
thermometer only reports temperatures between 70°F and 72°F, you
cannot use it to choose when to wear a coat. Ratings with this
compression are not lies *per se*; they are just useless.

### 4.7 Gaming via review solicitation

Many platforms let sellers send review requests to purchasers.
Since satisfied purchasers are more likely to respond to a
request than dissatisfied ones, requested reviews skew positive.
This is not fraud (no payment changes hands), but it is a
source of bias that relativistic ratings handle better by
exposing each reviewer's history to inspection.

---

## 5. What is a Relativistic Rating?

Now the positive construction. I will define the relativistic
rating precisely and walk through a worked example.

### 5.1 Definition

A **relativistic rating** `RR(observer, product, topic)` is a
scalar in [0, 5] (or whatever rating range is in use) computed
as the weighted average of individual reviews, where the weight
of each review is a function of:

- The observer's trust in the reviewer, scoped to the topic
  domain.
- The reviewer's historical helpfulness, as judged by the
  observer's own trust graph.
- The reviewer's activity level (how much history they have in
  the topic).
- The recency of the review.

Formally:

```
w(r) = T(observer, r.reviewer, topic)
     × H(observer, r.reviewer, topic)
     × A(r.reviewer, topic)
     × R(r.timestamp)

RR(observer, product, topic) = Σ_r r.rating · w(r)  /  Σ_r w(r)
```

Where the sum is over all reviews `r` with positive weight.

### 5.2 Worked example: same product, two observers

Consider a laptop product with four reviews:

| Reviewer | Rating | History | Helpful votes |
|----------|--------|---------|---------------|
| Alice    | 5.0    | 80 laptop reviews | widely upvoted |
| Bob      | 2.0    | 3 laptop reviews | few votes |
| Carol    | 4.5    | 200 tech reviews | widely upvoted |
| Dave     | 5.0    | 1 review (brand new) | none |

Observer 1 (Jamie, a software engineer): directly trusts Alice,
Carol. Has never interacted with Bob or Dave.

Observer 2 (Pat, a first-time laptop buyer): directly trusts
Bob (Pat's brother). No prior graph.

Observer 1's weighted computation:

```
w_Alice  = T(Jamie, Alice, laptops) = 0.9
         × H(Jamie, Alice, laptops) = 0.85
         × A(Alice, laptops) = 1.0     (ceiling)
         × R(Alice's review time) = 0.95
         = 0.727

w_Bob    = T(Jamie, Bob, laptops) = 0.05     (very weak transitive)
         × H(Jamie, Bob, laptops) = 0.5       (neutral)
         × A(Bob, laptops) = 0.28
         × R(...) = 0.95
         = 0.0067

w_Carol  = 0.8 × 0.82 × 1.0 × 0.95
         = 0.623

w_Dave   = T(Jamie, Dave) = 0 (no path) ⇒ weight 0

Effective rating = (5.0 · 0.727 + 2.0 · 0.0067 + 4.5 · 0.623 + 5.0 · 0)
                 / (0.727 + 0.0067 + 0.623)
                 = (3.635 + 0.013 + 2.803) / 1.357
                 = 4.75
```

Observer 2's computation is different:

```
w_Alice  = T(Pat, Alice, laptops) = 0 (no path)  ⇒ weight 0
w_Bob    = T(Pat, Bob, laptops) = 0.95 (brother)
         × H(Pat, Bob, laptops) = 0.5  (neutral, few votes)
         × A(Bob, laptops) = 0.28
         × R(...) = 0.95
         = 0.126

w_Carol  = 0 (no path)
w_Dave   = 0 (no path)

Effective rating = (2.0 · 0.126) / 0.126
                 = 2.0
```

Same product. Same four reviews. Observer 1 sees 4.75.
Observer 2 sees 2.0.

Both are correct. Observer 1's view reflects their trust in two
domain experts. Observer 2's view reflects their trust in their
brother (who happens to disagree with the experts).

**This is not a bug. This is exactly what a rating should do.**
If Observer 2 wants to broaden their view, they can add trust
edges to other reviewers and the number updates accordingly.
The rating is explicit about whose opinions it represents.

### 5.3 The structural property this enables

In a relativistic rating, **an attacker who wants to move the
rating needs to enter the observer's trust graph.** That is not
a matter of posting more reviews; it is a matter of getting the
observer (or someone the observer trusts) to vouch for the
attacker.

In a global-average rating, an attacker who wants to move the
rating only needs to post more reviews faster than detection.
The observer has no say.

This difference changes the attacker's cost function fundamentally.

---

## 6. The Math: Four-Factor Rating

The four-factor formula is the operationalization of the
relativistic rating. Let me formalize each factor and prove a
few properties.

### 6.1 Factor T: topical transitive trust

Defined as the maximum-path trust from observer to reviewer in
the specified topic domain:

```
T(observer, reviewer, topic) = RT(observer, reviewer | domain=topic, max_depth=5)
```

Where `RT` is the relational trust function proved to have
monotonic decay and bounded complexity in the previous blog
post in this series. If no path exists in the exact topic, the
formula falls back to parent domains with a 0.8 decay per step:

```
T(o, r, "reviews.public.tech.laptops") =
    RT(o, r, "reviews.public.tech.laptops")
    if > 0
    else 0.8 · RT(o, r, "reviews.public.tech")
    if > 0
    else 0.64 · RT(o, r, "reviews.public")
    else 0
```

This fallback is informed by Jøsang et al. 2007: topical
specificity matters, but a competent cross-topic reviewer should
not be discarded entirely.

### 6.2 Factor H: helpfulness reputation

Reviewers accumulate `HELPFUL_VOTE` and `UNHELPFUL_VOTE` events
from other users on their prior reviews. The observer weights
each voter by their own trust graph, then computes the ratio:

```
H(observer, reviewer, topic) =
    Σ_v (T(observer, v.voter, topic) if v is HELPFUL)
  / Σ_v  T(observer, v.voter, topic)
```

If no votes, `H = 0.5` (neutral).

**Recursive cap:** to prevent infinite recursion in trust
computation, the `max_depth` for the trust lookup *inside* H is
capped at 3 (vs 5 for the outer `T`). This keeps worst-case
complexity bounded.

### 6.3 Factor A: activity

```
A(reviewer, topic) = clip( log(reviews_in_topic) / log(50), 0, 1 )
```

Reviews counted over the last 24 months. Chosen empirically:
first review contributes little, reviews scale logarithmically,
caps at 50. Rationale:

- First-time reviewers are not penalized to zero; they can still
  contribute via T and H.
- A reviewer with consistent history gets a ~0.6x boost at 10
  reviews, ~1.0 at 50.
- Past 50 reviews, additional reviews do not further increase
  weight (prevents "log everything to farm weight").

This mapping is consistent with Resnick et al. 2000's requirement
that reputation reward demonstrated behavior.

### 6.4 Factor R: recency

```
R(timestamp) = max(0.3, exp(-age_days / 730))
```

Exponential decay with 2-year half-life, clamped at 0.3 (an
older review never goes to zero). Rationale:

- Products age; tastes change. A 10-year-old review is not
  irrelevant but is less current.
- The 0.3 floor means "this was good once" still carries signal.
- 730-day half-life is an editable default; operators can tune
  per domain.

### 6.5 Why multiplicative, not additive?

We combine the four factors by multiplication, not addition.
This is deliberate:

- **Any factor near zero should near-zero the whole weight.** A
  reviewer the observer has no trust path to (T=0) should be
  excluded entirely, regardless of how active they are.
- **All factors must be present.** A reviewer with no history
  (A=0) should not get full credit for having strong T+H+R.
- **Multiplication is commutative and associative.** Order of
  evaluation does not matter.

Additive combinations would let one factor mask another. A
reviewer who is 99%-trusted but never active would still get
heavy weight under an additive scheme, which is wrong.

### 6.6 Formal property: bounded influence per reviewer

**Claim.** For any fixed observer and topic, no single reviewer's
contribution to the rating exceeds `w(r) / Σ w(r)`.

**Proof.** The aggregate rating is a weighted average:

```
RR = Σ r.rating · w(r) / Σ w(r)
```

The partial derivative of RR with respect to r.rating is:

```
∂RR / ∂r.rating = w(r) / Σ w(r) ≤ 1
```

This is bounded by the reviewer's *relative* weight, which for
any single reviewer is capped at the reviewer's share of the
total weight. An attacker who compromises one reviewer account
can move the rating by at most that share. ∎

Contrast this with global averages where one fake review with a
viral helpfulness campaign can disproportionately move a small
product's rating. Relativistic ratings naturally cap the damage.

---

## 7. Anti-Spam Properties

This section synthesizes the anti-spam argument explicitly. The
attacker's goal is to move the rating in some direction. How
much does it cost under relativistic ratings?

### 7.1 Cost to create a fake reviewer account

Zero. Anyone can create a Quidnug quid (ECDSA keypair) in
microseconds. This is important: we are *not* trying to make
identity creation expensive.

### 7.2 Cost to have that fake reviewer influence anyone

Dramatically higher. The attacker must either:

- **(a) Directly compromise the observer's trust edges.** Get
  the observer to publish a TRUST edge toward the fake reviewer.
  This is a social attack requiring that the observer either be
  fooled into thinking the fake is legitimate, or be compromised
  in their identity management.
- **(b) Compromise someone the observer trusts.** Walk up the
  observer's trust graph and compromise a node whose trust
  propagates to the target. The cost multiplies: if each honest
  party has a compromise cost C, and trust decays by factor d
  per hop, then compromising a hop-k node influences the
  observer by d^k × signal_strength, making the attack's return
  exponentially worse per hop.

### 7.3 Economic comparison: fake reviews under global vs relativistic

```
Cost to move a product's rating by 0.5 stars

Global rating system:
    Number of fake reviews needed: ~50 (assuming 200 existing, flat weight)
    Cost per fake review: $5-30 (He/Hollenbeck/Proserpio 2022)
    Total: $250 to $1500
    Detection probability: 10-30%
    Expected cost: $280-$1900

Relativistic rating system (typical observer):
    Fake reviews posted by strangers: weight ≈ 0
    Fake reviews needed to move rating: ∞ (they contribute no weight)
    Alternative: compromise observer's trusted contacts
    Cost per trusted-contact compromise: varies, typically $100-$10,000
    Number needed: depends on decay, typically 2-3 trusted sources
    Detection probability: high (observer notices strange activity
        from trusted contacts)
    Expected cost: $200 to $30,000+ and high risk of detection
```

The cost asymmetry is the whole argument. Relativistic ratings
do not make fake reviews impossible to create; they make fake
reviews **economically useless** to the attacker.

### 7.4 Why bots naturally exclude themselves

Under relativistic ratings, a bot that posts 1000 fake reviews
contributes the same total weight to the observer's computation
as a bot that posts 0 reviews: zero. The bot's existence does
not affect the observer's view.

This is a structural property. The attacker cannot "scale up"
their way out of it by generating more Sybils. Quantity does
not help when quality (trust edges to the observer) is the
binding constraint.

Compare to the detect-filter arms race: every defense against
bots requires improving detection, which requires more training
data, which requires more bot samples. Relativistic ratings skip
this loop.

### 7.5 Comparison to Sybil-resistance approaches

| Approach | Mechanism | Cost to attacker | Failure mode |
|----------|-----------|------------------|--------------|
| CAPTCHA | Human-verification gate | ~$0.001 per solve (captcha farms) | Farms are cheap and legal |
| PoW identity creation | Burn electricity per quid | Low at scale (parallelizable) | Rich adversaries win |
| PoS identity stake | Lock economic stake per quid | Proportional to stake | Wealthy attackers win |
| Global reputation score | Build history over time | 6+ months of clean activity | Aged accounts sold in gray markets |
| Relativistic trust graph | Get observer to trust you | Social engineering cost | High per-observer; no scaling |

The last row is the only one where the attacker cost scales
with the attack's reach. Attacking 10 observers requires 10
independent compromises, not one.

---

## 8. Visualization at Three Depths

A relativistic rating has more dimensions than a five-star
scalar. Surfacing those dimensions without overwhelming users
is a UX problem, not a math problem. Quidnug ships three SVG
primitives that render the same underlying contributor data at
three different information densities.

### 8.1 The three primitives

```
Quidnug visualization primitives: a comparative summary

┌─────────────────┬──────────────┬───────────────┬────────────────┐
│  Primitive      │  Use when    │  Bits shown   │  Target size   │
├─────────────────┼──────────────┼───────────────┼────────────────┤
│ <qn-aurora>     │  Glance      │  ~4           │  24px - 120px  │
│                 │  (list view) │  (rating,     │                │
│                 │              │   confidence, │                │
│                 │              │   directness, │                │
│                 │              │   delta)      │                │
├─────────────────┼──────────────┼───────────────┼────────────────┤
│ <qn-constellat- │  Drilldown   │  ~N*3         │  200px - 500px │
│ ion>            │  (detail pg) │  (per-reviewer│                │
│                 │              │   rating,     │                │
│                 │              │   weight,     │                │
│                 │              │   proximity)  │                │
├─────────────────┼──────────────┼───────────────┼────────────────┤
│ <qn-trace>      │  Composition │  ~N*2         │  20px - 80px   │
│                 │  (side-by-   │  (weight      │  (horizontal)  │
│                 │   side list) │   share,      │                │
│                 │              │   rating      │                │
│                 │              │   color)      │                │
└─────────────────┴──────────────┴───────────────┴────────────────┘
```

The key design decision: **all three primitives consume the same
input data shape** (a list of contributor objects with rating,
weight, trust-distance, and directness flags). A host page
computes the rating state once and passes it into whichever
primitives it needs.

### 8.2 The aurora: single glance

The aurora encodes four dimensions in a single composite glyph:

```
Aurora layout (schematic)

           ┌──────────────┐
           │    ● 4.5     │     center dot: rating color
           │   ○ ring     │     ring thickness: confidence
           │   ─ ─ ─      │     ring pattern: directness
           │   ↑0.4       │     chip: personalization delta
           └──────────────┘
```

Color (red/amber/green) encodes the rating value. Ring thickness
encodes confidence (how many trusted reviewers contributed).
Ring pattern (solid/dashed) encodes whether the trust is direct
or transitive. The chip (↑0.4) encodes the delta from the
unweighted crowd average (so the observer knows if their
personalized view diverges).

Accessibility: screen readers get a computed plain-language
label, e.g., "4.5 stars from your trusted reviewers, high
confidence, 0.4 higher than the crowd average."

### 8.3 The constellation: drilldown

The constellation shows the actual trust graph that fed the
rating:

```
Constellation layout (schematic, observer at center)

                    crowd tier
                     ○    ○    ○
                   ○    ○    ○  ○
                3-hop tier
              ○        ●        ○
                 ●           ○
                2-hop tier
              ◉          ●
                  ◉     ●
                direct tier (1-hop)
                     ●
                   ● ◉
                       you at center
```

Each dot is a reviewer. Position encodes trust proximity (center
= observer, outer rings = more distant). Color encodes their
rating (red/amber/green). Size encodes their weight. Dot outline
encodes direct (solid) vs transitive (dashed) trust.

Clicking a dot opens the trust path (e.g., "you → Alice → Bob →
this reviewer, path trust 0.63") for full provenance.

This primitive answers the question "why is my rating what it
is?" A global rating system cannot answer this; the reviewer
population is anonymous. Relativistic ratings make it concrete.

### 8.4 The trace: side-by-side composition

When comparing multiple products in a list, the trace compresses
the contributor composition into a horizontal stacked bar:

```
Trace primitive examples (horizontal stacked bars)

Product A     ▓▓▓▓▓░░░▒▒  (wide solid green = strong trusted agreement)
Product B     ▓▓░░░░▒▒▒▒  (narrow solid, wide dashed = weaker signal)
Product C     ████░░██░░  (split: mixed ratings from trusted sources)
Product D     ░░░░░░░░░░  (all transitive, no direct: low confidence)
Product E     ▓▒        (very few reviewers: low volume)
```

Width of each segment is the reviewer's weight share. Color is
their rating. Outline (solid/dashed) is directness. Eyeballing a
list of products with traces lets you spot "this one has good
ratings from people you actually trust" without reading any
numbers.

### 8.5 Composition pattern

```
   nano list view           detail page hero            drilldown
   ┌──────────┐           ┌───────────────┐           ┌───────────────┐
   │ ●4.5 ↑0.4│           │    ┌─────┐    │           │   ┌─────┐     │
   └──────────┘           │    │ 4.5 │    │           │   │ 4.5 │     │
                          │    └─────┘    │           │   └─────┘     │
                          │  qn-aurora    │           │  qn-aurora    │
                          └───────────────┘           │               │
                                                      │  ┌─────────┐  │
                                                      │  │ ◎  ●  ● │  │
                                                      │  │ ◉  ●  ○ │  │
                                                      │  └─────────┘  │
                                                      │qn-constellat. │
                                                      │               │
                                                      │ ▓▓▓▓░░░▒▒     │
                                                      │  qn-trace     │
                                                      └───────────────┘
```

The progression matches the user's attention budget at each
point:

- **List / grid:** aurora nano alongside product title and price.
  Milliseconds of attention per product.
- **Detail / hero:** aurora standard large, optional trace for
  a simple composition view. 2-5 seconds of attention.
- **Drilldown / "why":** full constellation + trace + sortable
  reviewer table. 30+ seconds of deliberate evaluation.
- **Expert / audit:** full constellation + trace + reviewer
  table + trust-path viewer + Quidnug trust-query explorer.
  Minutes of investigation.

### 8.6 Fallback to stars for SEO

Quidnug still emits Schema.org JSON-LD with
`AggregateRating.ratingValue` in the crowd (unweighted) form, so
Google rich results render stars. The five-star summary is the
*most compressed* form of the rating, not the authoritative one.
Humans see the aurora next to it. Search engines see the stars.
Both are served.

```html
<script type="application/ld+json">
{
  "@context": "https://schema.org",
  "@type": "Product",
  "aggregateRating": {
    "@type": "AggregateRating",
    "ratingValue": "4.3",
    "reviewCount": "487"
  }
}
</script>

<qn-aurora rating="4.5" size="standard"
           observer="did:quidnug:..."
           product="did:quidnug:..."
           topic="reviews.public.tech.laptops">
  <!-- fallback content for clients without JS -->
  <span aria-label="4.5 stars personalized from your trusted reviewers">
    ★★★★☆ 4.5 (your view)
  </span>
</qn-aurora>
```

### 8.7 Accessibility guarantees

All three primitives are designed to WCAG 2.1 AA:

- **Color redundancy.** Every color distinction is backed by
  shape or stroke (good = dot, mixed = square, bad = triangle;
  direct = solid, transitive = dashed).
- **Screen reader support.** Every primitive exposes a computed
  `aria-label` in plain language.
- **Keyboard navigation.** Every interactive element is
  tab-reachable; constellation dots respond to Enter/Space.
- **Reduced motion.** Animations respect
  `prefers-reduced-motion: reduce`.
- **Minimum contrast.** 4.5:1 for text, 3:1 for UI components.

This is not optional. A rating UI that fails AA is broken.

### 8.8 Progressive disclosure: a worked user journey

User opens a product detail page:

```
Time t=0 (immediate)
    ┌─────────────────────────────────────────────┐
    │  Apple MacBook Pro 16"                      │
    │  $3,199                                     │
    │  ● 4.5    [ Buy ]                           │
    │  (aurora standard, shown immediately        │
    │   from cached rating snapshot)              │
    └─────────────────────────────────────────────┘

Time t=2s (rating resolved from local node)
    ┌─────────────────────────────────────────────┐
    │  Apple MacBook Pro 16"                      │
    │  $3,199                                     │
    │  ● 4.5 ↑0.4  [ Buy ]   See why this →       │
    │  (aurora shows personalization delta)       │
    └─────────────────────────────────────────────┘

Time t=8s (user clicks "See why")
    ┌─────────────────────────────────────────────┐
    │  Personalized rating: 4.5                   │
    │  Crowd average: 4.1                         │
    │                                             │
    │  ┌───────────────────┐                      │
    │  │  ○    ○      ○    │ crowd               │
    │  │     ○   ○         │                      │
    │  │  ○        ●       │ 3-hop               │
    │  │     ●    ○        │                      │
    │  │      ●            │ 2-hop               │
    │  │  ◉         ●      │ direct              │
    │  │       ●          │                       │
    │  │       *           │ ← you                │
    │  └───────────────────┘                      │
    │   (constellation: click any dot to         │
    │    see the trust path)                      │
    │                                             │
    │  Top contributors (by weight):              │
    │   Alice (0.73) • laptops expert, 5★        │
    │   Carol (0.62) • reviewer since 2021, 4.5★ │
    │   ...                                       │
    └─────────────────────────────────────────────┘
```

The progressive disclosure respects the user's attention
budget at every step while making the full picture available on
demand. Global ratings cannot do this because the detail does
not exist.

---

## 9. Real-World Examples

### 9.1 Restaurant reviews: Yelp

A Yelp restaurant has a single 3.8-star average. Who is that
average over?

- 60% from first-time visitors
- 25% from occasional diners
- 10% from local regulars
- 5% from professional food critics

Each of these groups has systematically different preferences.
A business traveler wants reliability; a local foodie wants
adventure; a critic wants technical skill. Collapsing them into
one number serves no group well.

**Relativistic model:** a foodie with a trust graph that includes
other local foodies and several professional critics sees the
restaurant at (say) 4.2 because their trusted cohort rated it
higher than the average tourist. A business traveler who trusts
Michelin guides and no one else sees 3.5 (pro-critic rating was
harsh). A tourist with no graph sees the crowd default (3.8).

All three views are honest. Any one of them alone is misleading.

### 9.2 Movie reviews: Letterboxd vs IMDb

IMDb shows a global aggregate rating. Letterboxd emphasizes
social graph: whose ratings do you follow? The Letterboxd model
is empirically more predictive of personal preference because it
selects reviewers who share your taste.

Quidnug generalizes the Letterboxd model: rather than requiring
you to manually follow reviewers, it computes the weighted score
from your existing trust graph. If you have zero graph, you get
the crowd default; otherwise you get a personalized score.

### 9.3 Academic papers: Google Scholar vs Scite

Google Scholar shows citation count. Scite [^scite] shows
citations weighted by whether they were supporting or
contradicting, and allows drill-down into which papers cited
which.

A relativistic academic rating would go further: weight citing
papers by their own reputation in your subfield. A positive
citation from a seminal paper in your area is worth more than
100 citations from tangential papers in unrelated subfields.

### 9.4 Medical practitioners: Healthgrades

A specialist rated 4.2 on Healthgrades. Who reviewed? Other
specialists? General practitioners? Patients with one visit?
Caregivers of long-term patients? Each group has different
information.

The relativistic model: a patient choosing a cardiologist
weights reviews from cardiologists (peer assessment), primary
care physicians (referring-doctor assessment), and patients
with condition similar to theirs (outcome experience). Each
category contributes with its own trust weight.

### 9.5 Reddit's weighted voting

Reddit's default score (ups - downs) is a global sum. Reddit's
actual ranking algorithm weights by vote quality (users with
higher "karma" have higher-weight votes, roughly). This is a
crude relativistic scheme; it uses one global graph (karma)
rather than per-user graphs, but the principle is identical.

Reddit's r/askhistorians applies this even more aggressively:
answers are curated by moderators with domain expertise. The
community has implicitly adopted a relativistic trust model
despite the platform's global-score defaults.

### 9.6 Stack Overflow

Stack Overflow's reputation system (Resnick et al.'s work made
flesh) weights votes by reputation and makes reputation
topic-specific via tags. It is closer to relativistic than
Amazon is. It still has a single global reputation number per
user, which is the next step down from a per-observer graph,
but the direction is correct.

---

## 10. Honest Tradeoffs

I argued in the previous blog that PoT has real weaknesses.
Relativistic ratings inherit some of those and add their own.

### 10.1 Bootstrap problem

A new user with no trust graph has no personalization. They see
the crowd default, which is (as we established) misleading.

**Mitigation:** the crowd default itself can be an opinionated
aggregate (e.g., the seed operator's curated reviewer roster).
That is less biased than a raw average because the operator
applied editorial judgment to the baseline. Users then
customize from there by adding trust edges to people they
discover.

This is comparable to first-time use of Spotify or Netflix: the
recommendation is generic until the user interacts; personalization
emerges with use.

### 10.2 Filter bubble concerns (reprise)

The constellation view explicitly shows the crowd tier so users
can see "what everyone thinks" alongside "what my graph thinks."
Hiding the crowd would be the bubble; showing it alongside the
personalized view is the anti-bubble.

There is also an active research direction on "adversarial
recommenders" that deliberately push users toward diverse views
(Sunstein 2001's original framing). Quidnug supports this: a
user can consciously weight an "intellectual diversity" sub-
graph that pulls from reviewers they disagree with, precisely
to challenge their baseline.

Relativistic ratings do not solve filter bubbles automatically.
They make the bubble visible and manipulable, which is a
necessary first step.

### 10.3 When global ratings are correct

Honestly: some contexts want global ratings and should have them.

- **Legal / regulatory standards.** "Is this restaurant in
  compliance?" should be a single objective rating, not
  personalized.
- **Safety ratings.** Car crash test scores, medical device
  approval: one truth.
- **Certifications.** "Does this product meet ISO 27001?" is
  binary, not graded.

The relativistic model is for opinion-shaped judgments
(quality, fit, reliability in subjective use), not for
objective-shaped judgments (compliance, certification,
safety). Use the right tool.

### 10.4 Computational cost

A per-observer rating is not free. For each (observer, product,
topic), the system computes trust paths for each reviewer, which
is O(b^d) per path with b ~ 10 and d ~ 5 (so ~100k node visits
worst case). With a reasonable cache (Quidnug's TrustCache)
this amortizes to <1ms for typical queries.

Compared to a global average (O(1) after precomputation), this
is a real cost. The cost is order-of-microseconds on modern
hardware, which is still below HTTP latency, but engineers
should know. Caching strategies and batch computation of
page-level rating summaries are standard implementation
techniques.

### 10.5 "Objective" users expect objective answers

Some users emphatically do not want their view of reviews
personalized. They want "the" number.

**Mitigation:** every Quidnug-integrated review display can be
toggled to "crowd" mode (unweighted global average). The
default should be personalized; the toggle should be clearly
accessible. Users who disable personalization still benefit
from the underlying anti-spam properties because the crowd
itself is weighted-filtered (a known-bot's reviews are
downweighted via the standard mechanism; no observer's graph
is involved).

---

## 11. Implementation Notes

For readers evaluating adoption, here is the practical landing
path.

### 11.1 Quidnug Reviews Protocol (QRP-0001)

Full protocol specification:
`examples/reviews-and-comments/PROTOCOL.md`. Reference
implementations in Python and Go at
`examples/reviews-and-comments/algorithm.py` and
`integrations/schema-org/`.

Event types:

- `REVIEW`: the actual review with rating + text + topic
- `HELPFUL_VOTE`: upvote for an existing review
- `UNHELPFUL_VOTE`: downvote
- `FLAG`: report for moderation (hooks into QDP-0015)

### 11.2 Framework adapters

Drop-in components for every major web framework:

| Framework | Package | Component |
|-----------|---------|-----------|
| Web components | `@quidnug/reviews-web-components` | `<qn-aurora>`, `<qn-constellation>`, `<qn-trace>` |
| React | `@quidnug/reviews-react` | `<Aurora>`, `<Constellation>`, `<Trace>` |
| Vue | `@quidnug/reviews-vue` | same |
| Astro | `@quidnug/reviews-astro` | same, with SSR |
| WordPress | `quidnug-reviews-wp` | shortcode `[qn-aurora product=...]` |
| Shopify | scaffold in `clients/shopify/` | theme snippet |

All adapters produce the same visual output and accept the same
contributor-array input shape.

### 11.3 Integration checklist for a new platform

1. Integrate Quidnug client SDK (Python, Go, JS/TS, Rust, or
   via REST API).
2. Issue a TRUST domain for your review scope, e.g.,
   `reviews.yourplatform.com`.
3. Publish seed trust edges for your baseline reviewers (or
   let them self-publish and signal-boost).
4. Display `<qn-aurora>` in your UI wherever you currently
   show a star rating.
5. Optionally: integrate `<qn-constellation>` on detail pages
   for the "why this rating" drilldown.
6. Add Schema.org JSON-LD for SEO preservation.
7. Hook `HELPFUL_VOTE` / `UNHELPFUL_VOTE` events for user
   feedback on reviews.

### 11.4 Observability

Every relativistic rating query emits metrics:

- `quidnug_rating_computation_duration_seconds`
- `quidnug_rating_cache_hits_total`
- `quidnug_rating_contributors_count_histogram`
- `quidnug_rating_personalization_delta_histogram`

The last is particularly interesting. If the distribution of
`delta` is wide, it indicates high information content (the
personalization is doing real work). If it is narrow, users are
seeing near-crowd ratings and the trust graph is sparse.

---

## 12. Conclusion

The five-star average is not a statistic; it is a habit. It
survived into the 2020s because it is legible and because every
platform shared the same habit. That is not a good reason.

Social science, reputation theory, and direct mathematical
analysis all say the same thing: **trust is relational, context-
dependent, and transitive with decay.** Ratings should be the
same.

Review spam is not a detection problem; it is an incentive
problem. Global averages create an incentive to post fake
reviews. Relativistic ratings eliminate that incentive by
making fake reviews contribute zero weight to observers who do
not trust the fake reviewer. The detection arms race
terminates.

The visualization primitives, the four-factor formula, the
trust graph, and the social-science foundation all compose into
a coherent alternative. It is not theoretical. Quidnug ships it
today in production-ready Python, Go, JS/TS, and Rust SDKs, with
drop-in web components for every major web framework and a
demonstrated working end-to-end flow against a live node.

If you are building a review system, a reputation feature, a
marketplace, or anything where "who should I trust?" is the
question your users are asking, the answer is not a
five-star-average with a fraud-detection pipeline. The answer
is a relativistic rating with a transparent trust graph.

The sixty years of social psychology backing this position are
not an accident. They are the empirical record of what
trust-formation looks like in humans. A rating system that
works *with* that record produces more useful signals than one
that works against it.

That is worth building.

---

## 13. References

### Social psychology and sociology

[^asch1951]: Asch, S. E. (1951). *Effects of group pressure upon
the modification and distortion of judgments.* In H. Guetzkow
(Ed.), Groups, Leadership and Men (pp. 177-190). Carnegie Press.

[^asch1956]: Asch, S. E. (1956). *Studies of independence and
conformity: I. A minority of one against a unanimous majority.*
Psychological Monographs: General and Applied, 70(9), 1-70.

[^festinger1954]: Festinger, L. (1954). *A theory of social
comparison processes.* Human Relations, 7(2), 117-140.
https://doi.org/10.1177/001872675400700202

[^granovetter1973]: Granovetter, M. S. (1973). *The Strength of
Weak Ties.* American Journal of Sociology, 78(6), 1360-1380.
https://www.jstor.org/stable/2776392

[^milgram1963]: Milgram, S. (1963). *Behavioral Study of
Obedience.* Journal of Abnormal and Social Psychology, 67(4),
371-378.

[^facebook2016]: Bhagat, S., Burke, M., Diuk, C., Filiz, I. O.,
& Edunov, S. (2016). *Three and a half degrees of separation.*
Facebook Research.
https://research.facebook.com/blog/2016/2/three-and-a-half-degrees-of-separation/

[^fukuyama1995]: Fukuyama, F. (1995). *Trust: The Social Virtues
and the Creation of Prosperity.* Free Press, New York.

[^putnam2000]: Putnam, R. D. (2000). *Bowling Alone: The
Collapse and Revival of American Community.* Simon & Schuster.

[^cialdini1984]: Cialdini, R. B. (1984). *Influence: The
Psychology of Persuasion.* William Morrow.

### Organizational trust

[^mayer1995]: Mayer, R. C., Davis, J. H., & Schoorman, F. D.
(1995). *An Integrative Model of Organizational Trust.* Academy
of Management Review, 20(3), 709-734.
https://www.jstor.org/stable/258792

### Reputation systems

[^resnick2000]: Resnick, P., Kuwabara, K., Zeckhauser, R., &
Friedman, E. (2000). *Reputation Systems.* Communications of
the ACM, 43(12), 45-48.
https://dl.acm.org/doi/10.1145/355112.355122

[^dellarocas2003]: Dellarocas, C. (2003). *The Digitization of
Word of Mouth: Promise and Challenges of Online Feedback
Mechanisms.* Management Science, 49(10), 1407-1424.
https://doi.org/10.1287/mnsc.49.10.1407.17308

[^dellarocas2007]: Dellarocas, C., Zhang, X. M., & Awad, N. F.
(2007). *Exploring the value of online product reviews in
forecasting sales: The case of motion pictures.* Journal of
Interactive Marketing, 21(4), 23-45.

[^josang2007]: Jøsang, A., Ismail, R., & Boyd, C. (2007). *A
Survey of Trust and Reputation Systems for Online Service
Provision.* Decision Support Systems, 43(2), 618-644.
https://doi.org/10.1016/j.dss.2005.05.019

[^mui2002]: Mui, L., Mohtashemi, M., & Halberstadt, A. (2002).
*A Computational Model of Trust and Reputation.* Proceedings of
the 35th Hawaii International Conference on System Sciences.

### Review manipulation / fraud

[^hu2006]: Hu, N., Pavlou, P. A., & Zhang, J. (2006). *Can online
reviews reveal a product's true quality? Empirical findings and
analytical modeling of Online word-of-mouth communication.*
Proceedings of the 7th ACM Conference on Electronic Commerce,
324-330.

[^luca2016]: Luca, M. (2016). *Reviews, Reputation, and Revenue:
The Case of Yelp.com.* Harvard Business School Working Paper
12-016.
https://www.hbs.edu/faculty/Pages/item.aspx?num=41233

[^lucazervas2016]: Luca, M., & Zervas, G. (2016). *Fake It Till
You Make It: Reputation, Competition, and Yelp Review Fraud.*
Management Science, 62(12), 3412-3427.
https://doi.org/10.1287/mnsc.2015.2304

[^anderson2012]: Anderson, M., & Magruder, J. (2012). *Learning
from the crowd: Regression discontinuity estimates of the effects
of an online review database.* Economic Journal, 122(563),
957-989.

[^mayzlin2014]: Mayzlin, D., Dover, Y., & Chevalier, J. (2014).
*Promotional Reviews: An Empirical Investigation of Online Review
Manipulation.* American Economic Review, 104(8), 2421-2455.
https://doi.org/10.1257/aer.104.8.2421

[^hehollenbeck2022]: He, S., Hollenbeck, B., & Proserpio, D.
(2022). *The Market for Fake Reviews.* NBER Working Paper 29855.
https://www.nber.org/papers/w29855

[^ott2011]: Ott, M., Choi, Y., Cardie, C., & Hancock, J. T.
(2011). *Finding Deceptive Opinion Spam by Any Stretch of the
Imagination.* Proceedings of the 49th Annual Meeting of the
Association for Computational Linguistics.

[^jindal2008]: Jindal, N., & Liu, B. (2008). *Opinion Spam and
Analysis.* Proceedings of the International Conference on Web
Search and Data Mining (WSDM), 219-230.

### Filter bubbles and personalization

[^pariser2011]: Pariser, E. (2011). *The Filter Bubble: What the
Internet Is Hiding from You.* Penguin Press.

[^sunstein2001]: Sunstein, C. R. (2001). *Republic.com.*
Princeton University Press.

### Online platforms and industry data

[^scite]: Scite.ai. *Smart Citations Dashboard.*
https://scite.ai/

### Judgment under uncertainty

[^tversky1974]: Tversky, A., & Kahneman, D. (1974). *Judgment
under Uncertainty: Heuristics and Biases.* Science, 185(4157),
1124-1131.
https://doi.org/10.1126/science.185.4157.1124

### Game theory of cooperation

[^axelrod1984]: Axelrod, R. (1984). *The Evolution of
Cooperation.* Basic Books.

[^schelling1960]: Schelling, T. C. (1960). *The Strategy of
Conflict.* Harvard University Press.

### Quidnug specification documents

- **QRP-0001:** Quidnug Reviews Protocol.
  `examples/reviews-and-comments/PROTOCOL.md`
- **QRP-0001 rating algorithm:**
  `examples/reviews-and-comments/algorithm.md`
- **Rating visualization system:**
  `docs/reviews/rating-visualization.md`
- **QDP-0015:** Content Moderation & Takedowns.
  `docs/design/0015-content-moderation.md`

---

## Further reading

- *Reputation Systems* (Paul Resnick's ongoing work,
  especially the CACM foundational piece). The canonical starting
  point for engineering-grade reputation design.
- *Trust: The Social Virtues and the Creation of Prosperity*
  (Francis Fukuyama, 1995). The broader civic context for why
  trust infrastructure matters beyond any single platform.
- *The Evolution of Cooperation* (Robert Axelrod, 1984). Game-
  theoretic foundation of how cooperation emerges without
  enforcement. Relativistic ratings are an operationalization
  of tit-for-tat at internet scale.
- *Thinking, Fast and Slow* (Daniel Kahneman, 2011). How humans
  actually make judgments under uncertainty, which is what
  ratings inform.
- *The Digitization of Word of Mouth* (Chrysanthos Dellarocas,
  2003). Landmark paper on how online reputation mechanisms
  differ from offline ones.

---

*The Quidnug reviews protocol, reference implementations, and
framework adapters are at
[github.com/quidnug/quidnug](https://github.com/quidnug/quidnug)
under `examples/reviews-and-comments/` and `clients/`.
Contributions, critiques, and competing designs are welcome via
the Discussions tab or as pull requests.*
