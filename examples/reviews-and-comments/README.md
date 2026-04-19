# Trust-weighted reviews & comments on Quidnug

> A global, cross-site review protocol where every rating is
> weighted by **your** trust in the reviewer, their topical
> expertise, and their historical helpfulness — not by an
> average that treats every reviewer as identical.

## Want to run it?

A **full end-to-end working demo** against a live reference
node is in [`demo/`](./demo/). It posts 16 identities, 14 trust
edges, 15 reviews, and 8 helpfulness votes to an actual running
Quidnug node and renders three divergent per-observer ratings
in a browser. Start there if you want to see this working.

## The problem with today's reviews

Every review system on the web uses the same broken model:

- **One rating per entity.** A restaurant is "4.3 stars,"
  globally, for everyone.
- **All reviewers weighted equally.** A bot-net farm's review
  counts the same as a credentialed food critic's.
- **Helpfulness votes don't propagate.** Upvoting a helpful
  review does nothing for that reviewer's next review on a
  different site.
- **No topical specialization.** Someone who's a brilliant
  software reviewer has no baseline credibility when they
  write about restaurants.
- **Trust resets at every site.** Your Amazon reputation
  doesn't carry to Walmart, your Yelp history doesn't help on
  Google Maps, your TripAdvisor status vanishes at Hotels.com.

These aren't bugs — they're the direct consequence of "one
average score fits all," baked into every review UI since the
1990s.

## What Quidnug changes

Quidnug reviews are **relational**: the effective rating
**you** see for a product depends on:

1. **Your direct trust** in each reviewer (if you know them).
2. **Transitive trust** — if you don't know Alice but you trust
   Bob, and Bob trusts Alice on this topic, her review counts
   at a decayed weight.
3. **Topical expertise** — trust is scoped to domains like
   `reviews.public.technology`, `reviews.public.restaurants`.
   Someone brilliant on kitchen knives doesn't get undue
   weight on DSLR cameras.
4. **Historical helpfulness** — when prior reviews by this
   reviewer were upvoted by people **you** trust, their new
   review carries more weight.
5. **Recency + activity** — active reviewers over time get more
   credibility; one-off reviews decay.

And crucially — this is a **public global scheme**. Your
reviewer identity is the same cryptographic quid everywhere
you review, on any site, forever. A website adds trust-weighted
reviews by dropping in a web component. A reviewer builds
cross-site reputation automatically.

## What's in this directory

| File | What |
| --- | --- |
| [`PROTOCOL.md`](PROTOCOL.md) | The full Quidnug Reviews Protocol (QRP-0001) spec: event types, payloads, domain scheme, interaction rules |
| [`algorithm.md`](algorithm.md) | The trust-weighted rating algorithm in detail, including how helpfulness, topical trust, and recency combine |
| [`algorithm.py`](algorithm.py) | Reference Python implementation (importable from the Python SDK) |
| [`algorithm_test.py`](algorithm_test.py) | Unit tests covering the algorithm's corner cases |
| [`simulation.py`](simulation.py) | Multi-actor simulation showing how per-observer trust actually diverges |
| [`bootstrap-trust.md`](bootstrap-trust.md) | How new reviewers build initial trust (OIDC bridge, cross-site import, social bootstrap) |

## Libraries & frameworks for adoption

### For any website — drop-in widgets

| Package | Language/Platform | Effort | Status |
| --- | --- | --- | --- |
| [`@quidnug/reviews-widget`](../../clients/reviews-widget/) | Pure-JS iframe widget | 1 line of HTML | shipping |
| [`@quidnug/web-components`](../../clients/web-components/) | Custom elements (`<quidnug-review>`, `<quidnug-stars>`, `<quidnug-write-review>`) | Framework-agnostic | shipping |
| [`@quidnug/react-reviews`](../../clients/react-reviews/) | React component library | `npm install` | shipping |

### For major e-commerce / CMS platforms

| Package | Platform | Status |
| --- | --- | --- |
| [`quidnug-reviews` WordPress plugin](../../clients/wordpress-plugin/) | WooCommerce + vanilla WP | shipping |
| [`quidnug-reviews` Shopify app](../../clients/shopify-app/) | Shopify | scaffold |
| `quidnug-reviews` Squarespace / Wix / Webflow extensions | Various no-code | roadmap |
| `quidnug-reviews` Magento 2 / PrestaShop / BigCommerce modules | E-commerce | roadmap |

### For SEO + existing review interop

| Integration | What it does | Status |
| --- | --- | --- |
| [`integrations/schema-org/`](../../integrations/schema-org/) | Two-way mapping between Schema.org Review and Quidnug events | shipping |
| Browser-extension overlay | Add trust-weighted scores to Amazon / Yelp / Google Maps / TripAdvisor pages | scaffold in `clients/browser-extension/` |

## Architecture in one picture

```
      ┌────────────────────────────────────────────────────────────────┐
      │  PUBLIC QUIDNUG NETWORK — reviews.public.* domain tree         │
      │                                                                │
      │  ┌───────────────┐   ┌───────────────┐   ┌───────────────┐     │
      │  │ node-us-east  │◄─►│ node-eu-west  │◄─►│ node-apac     │...  │
      │  │ (Title stream │   │ (trust edges, │   │ (any volunteer│     │
      │  │  + events)    │   │ helpful votes)│   │  node)        │     │
      │  └───────┬───────┘   └───────┬───────┘   └───────┬───────┘     │
      │          └───────────────────┴───────────────────┘             │
      │                              │                                 │
      └──────────────────────────────┼─────────────────────────────────┘
                                     │ HTTP/JSON + gossip
            ┌────────────────────────┼────────────────────────┐
            │                        │                        │
   ┌────────▼────────┐    ┌──────────▼─────────┐    ┌────────▼────────┐
   │  amazon.com     │    │  some-indie-store  │    │  corporate site │
   │  product pages  │    │  .com product pages│    │  review section │
   │                 │    │                    │    │                 │
   │  <quidnug-review│    │  <quidnug-review/> │    │  WordPress      │
   │    product=ASIN │    │    web component   │    │  plugin         │
   │  />             │    │                    │    │                 │
   └────────┬────────┘    └──────────┬─────────┘    └────────┬────────┘
            │                        │                        │
            └────────────────────────┴────────────────────────┘
                                     │
                                     ▼
                      The reviewer's browser extension or
                      embedded wallet signs reviews and
                      helpful votes with THEIR quid. Same
                      quid on every site. Reputation
                      accrues globally.
```

A reviewer reviews a product on Amazon. The website calls
into `<quidnug-review>`, which signs the review with the
reviewer's quid (held in their browser extension). The event
goes onto the **global** Quidnug network under
`reviews.public.<category>`. Weeks later, the same reviewer
reviews the same product on Target. Their reputation (built
from helpfulness votes on the Amazon review) **follows them**.
An observer in Berlin loading the product page sees per-observer
weighted ratings that reflect **their** trust graph.

## The "public global scheme" in concrete terms

One Quidnug domain tree, maintained by volunteer node operators
(the same model as DNS root servers):

```
reviews.public
├── reviews.public.technology
│   ├── reviews.public.technology.laptops
│   ├── reviews.public.technology.cameras
│   ├── reviews.public.technology.software
│   └── ...
├── reviews.public.restaurants
│   ├── reviews.public.restaurants.us.ny.nyc
│   ├── reviews.public.restaurants.jp.tokyo
│   └── ...
├── reviews.public.books
├── reviews.public.movies
├── reviews.public.services
└── ...
```

Any site can query any public node for any topic. Any site can
run its own node that mirrors the public tree and adds private
extensions (e.g., `reviews.privatestore.myshop.com` for stuff
they want to keep private).

The domain tree is governed by a lightweight QRP (Quidnug
Reviews Protocol) working group; adding new top-level topics
requires rough consensus among node operators (see
[`PROTOCOL.md`](PROTOCOL.md) §7).

## Why this matters

- **For consumers**: ratings that reflect your actual trust
  relationships, not a global average corrupted by fake reviews.
- **For reviewers**: your reputation is portable and persistent.
  Build it once, it follows you everywhere.
- **For sites**: drop in a widget. Get better reviews than you
  could build in-house. No database, no moderation overhead,
  no sybil detection pipeline.
- **For the internet**: a shared commons of trust that isn't
  owned by Amazon, Yelp, or Google.

## Quick demo

The easiest way to see this working:

```bash
# Spin up a local node
cd deploy/compose && docker compose up -d

# Run the full multi-actor simulation
cd examples/reviews-and-comments
python simulation.py
```

The simulation runs through:
1. Alice, Bob, Carol, Dave, Eve all register quids.
2. A product gets 5 reviews (some helpful, some not).
3. Helpfulness votes accrue in a plausible pattern.
4. The same product gets its per-observer rating computed
   from Alice's, Bob's, and Carol's viewpoints — and the
   numbers come out meaningfully different.

## License

Apache-2.0.
