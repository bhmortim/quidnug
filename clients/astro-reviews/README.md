# @quidnug/astro-reviews

SSR-first Astro components for the Quidnug trust-weighted
rating system. The SVG is rendered statically at build time so
search engines, feed readers, and no-JS clients see the exact
same visualization the browser shows, plus the custom element
hydrates for interactivity (hover tooltips, click events).

## Install

```bash
npm install @quidnug/astro-reviews @quidnug/web-components
```

## Use

```astro
---
// product.astro
import { QnAurora, QnTrace } from "@quidnug/astro-reviews";
import { computePersonalRating } from "./lib/rating.js";

const rating = await computePersonalRating({
    productId: Astro.props.productId,
    observerId: Astro.locals.user?.quidId,
    topic: "reviews.public.technology.laptops",
});
---

<section>
    <h1>{rating.productName}</h1>

    <QnAurora size="standard"
              rating={rating.personal}
              crowd={rating.crowd}
              contributors={rating.contributors.length}
              direct={rating.contributors.filter(c => c.direct).length}
              observerName={Astro.locals.user?.displayName}
              showDelta
              showHistogram
              contributorRatings={rating.contributors.map(c => c.rating)} />

    <QnTrace contributors={rating.contributors} showLabels />
</section>
```

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
