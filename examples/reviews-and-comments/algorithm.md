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

## QRP-0002 algorithm extensions

QRP-0002 extends the algorithm output without changing the
core four-factor weighting. New fields populated on every
`effective_rating()` call:

| Field                              | Type           | Meaning                                                                      |
|------------------------------------|----------------|------------------------------------------------------------------------------|
| `anonymous_baseline_rating`        | float or None  | The rating an observer with operator-only trust would see. Same product, no observer trust graph. |
| `anonymous_baseline_total_weight`  | float          | Total weight contributing to the baseline.                                   |
| `personalization_delta`            | float or None  | `rating - anonymous_baseline_rating`. The "for you" adjustment.              |
| `confidence_pct`                   | float [0,100]  | Graph-density signal: how solid is this rating? Reaches 100% at high weight + many contributors. |
| `polarization`                     | float [0,1]    | Weighted spread of contributors. 0=tight agreement, 1=maximum spread.        |
| `top_intermediary_quid` (per contribution) | str or None | Best-known intermediary on the trust path. Populated when SDK exposes path. |

### Computing the anonymous baseline

To compute the baseline, the rater is configured with a
`baseline_observer_quid`: by convention, the recognized
validation-operator root for the network (initially Quidnug
LLC's `validation-operator-quid`, per QRP-0002 §5.3). The
baseline computation runs with this quid as the observer in
parallel with the per-observer computation.

For each review `r`, the baseline weight uses the same `H`,
`A`, and `R` factors but recomputes `T` from the operator
root:

```
T_baseline(r) = topical_trust(operator_root, r.reviewer, topic)
H_baseline(r) = helpfulness(operator_root, r.reviewer, topic)
A and R are observer-independent
w_baseline(r) = T_baseline × H_baseline × A × R
```

The baseline rating is then:

```
baseline_rating = Σ r.rating × w_baseline(r) / Σ w_baseline(r)
```

### Computing confidence percentage

```
confidence_pct = sqrt(
    min(1, total_weight / config.confidence_full_weight) ×
    min(1, contributing_reviews / config.confidence_full_contributors)
) × 100
```

Defaults: `confidence_full_weight=5.0`,
`confidence_full_contributors=10`. A rating from one tightly-
trusted reviewer (`total_weight=0.9, contributing_reviews=1`)
returns ~13%; from ten reviewers averaging weight 0.5 each
(`total_weight=5.0, contributing_reviews=10`) returns 100%.

The geometric mean ensures both factors matter: 100 reviewers
of trust 0.001 each don't get a confidence boost, and one
reviewer of trust 1.0 doesn't either.

### Computing polarization

```
polarization = stddev(contributions) / (display_max_rating / 2)
```

Where `stddev` is the weighted standard deviation of
contributor ratings. A polarization of 0.0 means all trusted
contributors agree (e.g., all rated 4.5); 1.0 means maximum
spread (e.g., half rated 0, half rated 5). UIs can render
this as "trusted sources agree" vs "trusted sources split."

### Top intermediary (path explanation)

The `top_intermediary_quid` field on each `ReviewContribution`
is populated when the SDK exposes path information. Today
the field is reserved (always None) until the SDK adds
`get_trust_path()`. UIs that render "via Bob" can prepare for
this by treating the field as optional and falling back to
the contribution weight alone.

### Standalone anonymous baseline

For SEO and Schema.org rendering, where the per-observer
rating is irrelevant (search engines index the baseline), use
`rater.anonymous_baseline_rating(product, topic)` directly:

```python
result = rater.anonymous_baseline_rating(
    product="laptop-xps15-asin-b0c1234",
    topic="reviews.public.technology.laptops",
)
schema_org_json = {
    "@type": "AggregateRating",
    "ratingValue": result.rating,
    "ratingCount": result.contributing_reviews,
}
```

The anonymous baseline is consistent across observers and is
the correct value for static, SEO-targeted rendering.

## License

Apache-2.0.
