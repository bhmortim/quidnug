# Trust-weighted review rating — algorithm

The per-observer weighted rating is computed from four
independent signals, combined multiplicatively. No single
factor dominates; a reviewer missing any one factor simply
loses that contribution.

## The four factors

For a given `(observer, product, topic)` triplet, each review
`r` on the product gets a weight `w(r)` computed as:

```
w(r) = T(observer, r.reviewer, topic)
     × H(observer, r.reviewer, topic)
     × A(r.reviewer, topic)
     × R(r.timestamp)
```

Then the effective rating is:

```
effective_rating = Σ r.rating × w(r)  /  Σ w(r)
```

Over all reviews with non-zero weight.

## Factor 1 — Topical transitive trust: `T`

From the observer's perspective through the Quidnug trust
graph, the best-path transitive trust to the reviewer in the
given topic domain, capped at 1.0.

Implementation: `client.get_trust(observer, reviewer, domain=topic, max_depth=5)`.

Topic inheritance: if no direct edge exists in the given
domain, fall back to the parent domain with a decay of 0.8
per step up. E.g., for `reviews.public.technology.laptops`:

1. Try `reviews.public.technology.laptops` first.
2. If no path, try `reviews.public.technology` with
   `× 0.8` decay applied.
3. If no path, try `reviews.public` with `× 0.64`.
4. If still no path, `T = 0` (stranger).

## Factor 2 — Helpfulness reputation: `H`

How helpful has this reviewer been historically, as judged by
the observer's own trust graph?

For each of the reviewer's prior reviews, fetch
HELPFUL_VOTE and UNHELPFUL_VOTE events. Weight each voter's
signal by the observer's trust in that voter.

```
def helpfulness_score(observer, reviewer, topic, client):
    helpful_weight = 0
    unhelpful_weight = 0

    prior_votes = fetch_votes_on_reviewer(reviewer, topic)

    for vote in prior_votes:
        voter_trust = T(observer, vote.voter, topic)
        if vote.type == "HELPFUL_VOTE":
            helpful_weight += voter_trust
        elif vote.type == "UNHELPFUL_VOTE":
            unhelpful_weight += voter_trust

    total = helpful_weight + unhelpful_weight
    if total == 0:
        return 0.5  # no signal, neutral

    return helpful_weight / total
```

The base value when no votes exist is 0.5 (pure neutral).
A reviewer with many helpful votes (weighted by observer
trust) approaches 1.0; a reviewer with many unhelpful votes
approaches 0.0.

**Recursive cap:** to prevent infinite depth, limit the
transitive-trust lookup inside helpfulness scoring to
`max_depth=3`. The base-case transitive trust (factor `T`)
uses `max_depth=5`.

## Factor 3 — Activity: `A`

Rewards consistent reviewers over one-time accounts. Purely
a function of the reviewer, not the observer.

```
A(reviewer, topic) = clip(log(reviews_in_topic) / log(50), 0, 1.0)
```

Where `reviews_in_topic` is the count of the reviewer's REVIEW
events in the given topic domain over the last 24 months.

- 0 reviews → 0 (but weight still applies via other factors)
- 1 review → ~0.18
- 10 reviews → ~0.59
- 50 reviews → 1.0
- 500 reviews → capped at 1.0

Rationale: first-time reviewers aren't penalized hard (they
still get weight from `T` and `H`), but a reviewer with
history gets a multiplicative boost.

## Factor 4 — Recency: `R`

Older reviews decay. Reviews are timestamped at event publish
time.

```
R(timestamp) = max(0.3, exp(-age_days / halflife))
```

Where:
- `age_days` = days since review was published.
- `halflife` = 730 days (2 years) by default.
- The `max(0.3, ...)` clamp ensures an old review never goes
  below 30% weight — history matters, even for year-old reviews.

Implementations MAY override `halflife` per topic (restaurants
= 180 days, books = 1825 days / 5 years).

## Worked example

Alice is looking at product `laptop-xps15`. The product has
5 reviews:

| # | Reviewer | Rating | Age (d) | Prior helpful/unhelpful votes |
| --- | --- | --- | --- | --- |
| 1 | Bob (Alice's friend, trust=1.0) | 4.5 | 30 | 40 helpful / 5 unhelpful |
| 2 | Carol (no direct trust) | 2.0 | 90 | 2 helpful / 0 unhelpful |
| 3 | Dave (transitively trusted via Bob at 0.72) | 4.8 | 5 | 100 helpful / 10 unhelpful |
| 4 | Eve (anonymous, no trust from anyone Alice trusts) | 5.0 | 60 | 1 helpful / 0 unhelpful |
| 5 | Frank (well-known laptop reviewer, Alice trusts at 0.9) | 4.2 | 540 | 500 helpful / 30 unhelpful |

Computing for Alice:

| # | T | H | A | R | w(r) | rating × w |
| --- | ---: | ---: | ---: | ---: | ---: | ---: |
| 1 Bob | 1.00 | 0.89 | 0.36 | 0.96 | 0.307 | 1.382 |
| 2 Carol | 0 | — | — | — | 0 | 0 |
| 3 Dave | 0.72 | 0.91 | 0.56 | 0.99 | 0.363 | 1.744 |
| 4 Eve | 0 | — | — | — | 0 | 0 |
| 5 Frank | 0.90 | 0.94 | 1.00 | 0.56 | 0.474 | 1.991 |

Sum of weights: `0.307 + 0.363 + 0.474 = 1.144`
Sum of rating × w: `1.382 + 1.744 + 1.991 = 5.117`

**Alice's effective rating: `5.117 / 1.144 = 4.47 / 5.0`**

Note how:
- Eve (5.0 stars!) contributes zero because Alice has no
  transitive trust to her.
- Frank's older review (540 days) loses weight via `R` but
  still dominates because of Alice's direct trust + his
  activity.
- Bob's neutral helpfulness doesn't hurt him much (trust is
  directly established).

Bob's simple unweighted average: `(4.5+2.0+4.8+5.0+4.2)/5 = 4.10`.

The trust-weighted view for Alice is **higher** because the
5.0-star reviewer she can't verify is excluded, and the
opinionated 2.0-star from an unknown reviewer is also
excluded.

For another observer — say, Carol herself — the computation
would look completely different. Same facts on the wire;
different rating UI per observer. That's the whole point.

## Implementation notes

See [`algorithm.py`](algorithm.py) for the reference
implementation. Key design points:

- **Caching:** helpfulness scores for a reviewer in a topic
  can be cached per observer for ~5 minutes. The underlying
  data is append-only so eventual consistency is fine.
- **Batching:** fetching trust to N reviewers in a topic can
  be parallelized. The Go + Rust clients support this natively;
  the Python client needs async.
- **Threshold filtering:** an implementation MAY drop reviews
  below a minimum weight threshold (e.g., `w(r) < 0.01`) to
  avoid noise from near-zero contributions. This is a
  presentation choice; the protocol doesn't mandate.
- **Confidence intervals:** when `Σ w(r)` is small, the
  effective rating has high variance. UIs SHOULD display
  confidence — e.g., show "4.5 ± 0.4 based on 3 trusted
  reviews" rather than a lone "4.5."

## Tunable parameters

These are implementation-level, not protocol-level, so sites
can tune them:

| Parameter | Default | Range |
| --- | --- | --- |
| Recency half-life | 730 days | 30–3650 |
| Activity saturation point | 50 reviews | 10–1000 |
| Topic inheritance decay | 0.8 per step | 0.5–1.0 |
| Min weight threshold | 0.01 | 0–0.2 |
| Max transitive trust depth | 5 for T, 3 for H | 1–10 |
| Helpfulness neutral base | 0.5 | 0.3–0.7 |

Sites should publish their tuning so reviewers understand what
they're optimizing for.

## License

Apache-2.0.
