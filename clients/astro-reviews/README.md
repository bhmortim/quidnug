# @quidnug/astro-reviews

SSR-first Astro components for the Quidnug trust-weighted
rating system. The SVG is rendered statically at build time so
search engines, feed readers, and no-JS clients see the exact
same visualization the browser shows, plus the custom element
hydrates for interactivity (hover tooltips, click events).

## Install

```bash
npm install @quidnug/astro-reviews @quidnug/web-components @quidnug/client
```

## Use

```astro
---
// product.astro
import { QnAurora, QnTrace } from "@quidnug/astro-reviews";
import { computePersonalRating } from "@quidnug/astro-reviews/lib/rating.js";

const rating = await computePersonalRating({
    node: import.meta.env.PUBLIC_QUIDNUG_NODE,
    productId: Astro.props.productId,
    observerId: Astro.locals.user?.quidId,
    topic: "reviews.public.technology.laptops",
});
---

<section>
    <h1>{rating.productName ?? Astro.props.productId}</h1>

    <QnAurora size="standard"
              rating={rating.personal}
              crowd={rating.crowd}
              contributors={rating.contributingReviews}
              direct={rating.contributors.filter(c => c.direct).length}
              observerName={Astro.locals.user?.displayName}
              showDelta
              showHistogram
              contributorRatings={rating.contributorRatings} />

    <QnTrace contributors={rating.contributors} showLabels />
</section>
```

## Server-side helper: `computePersonalRating`

```js
import { computePersonalRating } from "@quidnug/astro-reviews/lib/rating.js";

const r = await computePersonalRating({
    node: "http://localhost:8080",      // base URL of the Quidnug node
    productId: "laptop-x1",             // subject (title) id
    topic: "reviews.public.technology.laptops",
    observerId: "alice",                // or undefined for "anonymous"
    // raterOptions: { recencyHalflifeDays: 365, ... }
});
```

Returns:

```ts
{
    productId: string;
    observerId: string;
    topic: string;
    personal: number | null;        // observer-weighted rating
    crowd: number | null;           // anonymous-observer view (no trust edges)
    rating: number | null;          // alias for personal
    contributors: Array<{
        id: string; name?: string;
        rating: number; weight: number; direct: boolean;
    }>;
    contributorRatings: number[];   // ready for <QnAurora :contributor-ratings>
    contributingReviews: number;
    totalReviewsConsidered: number;
    confidenceRange: number;
}
```

Internally it runs the same `Rater` algorithm used by the
client-side custom element, against the live node — so the
build-time SVG, the hydrated client SVG, and the algorithm's
Python / Go reference implementations all agree byte-for-byte.

## Why SSR matters

The standard Quidnug widgets are client-rendered custom
elements, which is fine for most cases. For:

- **SEO.** Google's crawler indexes the static SVG; the
  Schema.org JSON-LD carries the underlying rating number for
  rich results.
- **Feed readers / newsletter previews.** The SVG renders in
  any HTML context, no JS required.
- **Performance.** First paint shows the aurora immediately,
  no FOUC while the web-component hydrates.

The Astro adapter calls the same pure `render*SVG()` functions
the custom elements use internally, so the server-rendered
markup and the client-hydrated markup are identical.

## Primitives

- `<QnAurora />` — headline rating glyph
- `<QnConstellation />` — bullseye drilldown
- `<QnTrace />` — horizontal stacked weight bar

See [../web-components/stories/index.html](../web-components/stories/index.html) for every visual state.

## License

Apache-2.0.
