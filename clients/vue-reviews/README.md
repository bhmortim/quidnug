# @quidnug/vue-reviews

Vue 3 components, composables, and a `provideQuidnug()` context for
the Quidnug trust-weighted review system. Feature-parity with
[`@quidnug/react-reviews`](../react-reviews/) — every React hook
has a Vue composable, every React component has a Vue counterpart.

## Install

```bash
npm install @quidnug/vue-reviews @quidnug/web-components @quidnug/client
```

## Vite / compiler setup

Vue needs to be told that any tag starting with `qn-` or
`quidnug-` is a custom element, not a Vue component:

```js
// vite.config.js
import vue from "@vitejs/plugin-vue";

export default {
    plugins: [
        vue({
            template: {
                compilerOptions: {
                    isCustomElement: (tag) =>
                        tag.startsWith("qn-") || tag.startsWith("quidnug-"),
                },
            },
        }),
    ],
};
```

## Provider

Install the Quidnug context once in a top-level setup script. Every
composable below reads `client` (and optionally `quid`) from this
context — same shape as the React `<QuidnugProvider>`.

```vue
<!-- App.vue -->
<script setup>
import { provideQuidnug } from "@quidnug/vue-reviews";
import QuidnugClient from "@quidnug/client";
import "@quidnug/client/v2";

const client = new QuidnugClient({ defaultNode: "http://localhost:8080" });
provideQuidnug({ client, defaultDomain: "reviews.public.other" });
</script>
```

## Drop-in components

The high-level components fetch + render. Use them when you don't
want to wire the data flow yourself.

```vue
<script setup>
import {
    QuidnugStars,
    QuidnugReviewPanel,
} from "@quidnug/vue-reviews";
</script>

<template>
    <!-- Star summary -->
    <QuidnugStars product="laptop-x1"
                  topic="reviews.public.technology.laptops"
                  show-count />

    <!-- Full panel: headline + list + (optional) write form -->
    <QuidnugReviewPanel product="laptop-x1"
                        topic="reviews.public.technology.laptops"
                        :show-write="true" />
</template>
```

## Composables

When you want full control over the rendering:

```vue
<script setup>
import { useTrustWeightedRating, useReviews, useWriteReview }
    from "@quidnug/vue-reviews";

const product = "laptop-x1";
const topic = "reviews.public.technology.laptops";

const { data: rating, loading } = useTrustWeightedRating(product, topic);
const { data: reviews } = useReviews(product, topic, { limit: 50 });
const { mutate: postReview, loading: posting } = useWriteReview();
</script>
```

All composables accept `product` / `topic` as plain strings, refs,
or getter functions and re-run automatically when any dep — including
the active observer quid — changes.

## Low-level primitives

Pure-render building blocks — zero networking, identical SVG output
to `@quidnug/web-components`:

```vue
<script setup>
import { QnAurora, QnTrace } from "@quidnug/vue-reviews";

const contributors = [
    { id: "vet", name: "veteran",  rating: 4.8, weight: 0.6, direct: true },
    { id: "sam", name: "sam-tech", rating: 4.5, weight: 0.2, direct: true },
    { id: "kai", name: "kai",      rating: 4.2, weight: 0.1, direct: false },
];
</script>

<template>
    <QnAurora :rating="4.7" :contributors="7" :direct="5" :crowd="4.1"
              observer-name="alice" show-delta show-histogram
              :contributor-ratings="[4.5, 4.8, 4.2, 5, 4, 4.3, 4.7]"
              @aurora-click="(d) => console.log(d)" />
    <QnTrace :contributors="contributors" show-labels />
</template>
```

| Primitive | Purpose |
| --- | --- |
| `<QnAurora>` | Headline rating glyph. Sentiment dot + confidence ring + optional delta chip. Three sizes: `nano`, `standard`, `large`. |
| `<QnConstellation>` | Bullseye drilldown. Concentric tiers of trust, one dot per contributor. |
| `<QnTrace>` | Horizontal stacked weight bar. One segment per contributor. |

See [../web-components/stories/index.html](../web-components/stories/index.html)
for every visual state.

## Surface summary

| Symbol | Kind | Mirrors React |
| --- | --- | --- |
| `provideQuidnug`, `useQuidnug` | provider | `<QuidnugProvider>`, `useQuidnug()` |
| `useTrustWeightedRating(product, topic[, options])` | composable | `useTrustWeightedRating` |
| `useReviews(product, topic[, { limit, offset }])` | composable | `useReviews` |
| `useWriteReview()` | composable | `useWriteReview` |
| `<QuidnugStars>` | component | `<QuidnugStars>` |
| `<QuidnugReviewPanel>` | component | `<QuidnugReviewPanel>` |
| `<QuidnugReviewList>` | component | `<QuidnugReviewList>` |
| `<QuidnugWriteReview>` | component | `<QuidnugWriteReview>` |
| `<QnAurora>`, `<QnConstellation>`, `<QnTrace>` | primitive | identical |

## Roadmap

- Nuxt integration (SSR-safe `useFetch` wrapper around the composables).

## License

Apache-2.0.
