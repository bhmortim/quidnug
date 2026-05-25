# @quidnug/vue-reviews

Vue 3 composables + components for Quidnug's trust-weighted
review system. Wraps `@quidnug/web-components` and the
`@quidnug/client` SDK with idiomatic Vue APIs that mirror
`@quidnug/react-reviews` one-for-one — so a user that knows the
React surface can switch frameworks without relearning.

## Install

```bash
npm install @quidnug/vue-reviews @quidnug/web-components @quidnug/client
```

## 30-second example

```vue
<script setup>
import QuidnugClient from "@quidnug/client";
import {
    provideQuidnug,
    useTrustWeightedRating,
    QuidnugStars,
} from "@quidnug/vue-reviews";

const client = new QuidnugClient({ defaultNode: "https://node.example" });
provideQuidnug({ client });

const { data, loading } = useTrustWeightedRating("title:movie:dune2", "movies");
</script>

<template>
    <p v-if="loading">…</p>
    <p v-else-if="data">
        Your weighted rating: {{ data.rating?.toFixed(1) }}
        ({{ data.contributingReviews }} trusted reviewers)
    </p>

    <QuidnugStars product="title:movie:dune2" topic="movies" show-count />
</template>
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

Composables and components need a Quidnug client (and optionally
a signed-in observer Quid) in their context. Call
`provideQuidnug` once in a root component:

```vue
<script setup>
import { ref } from "vue";
import QuidnugClient from "@quidnug/client";
import { provideQuidnug } from "@quidnug/vue-reviews";

const client = new QuidnugClient({ defaultNode: "https://node.example" });
const quid = ref(null); // or a Quid object loaded from storage
provideQuidnug({ client, quid });
</script>
```

Both `client` and `quid` may be plain values or Vue refs. The
composables unwrap both forms and re-run when the underlying
ref changes.

## Composables

| Composable | Signature | Returns |
| --- | --- | --- |
| `useTrustWeightedRating` | `(product, topic, options?)` | `{ data, loading, error, refetch }` — refs. `data` is a `WeightedRatingResult` or `null`. |
| `useReviews` | `(product, topic, { limit?, offset? }?)` | `{ data, loading, error, refetch }` — refs. `data` is an array of `REVIEW` events. |
| `useWriteReview` | `()` | `{ mutate, loading, error, data }`. Call `mutate({ product, topic, rating, title, body, ... })` to post a review; throws if no signed-in Quid is provided. |

`product` and `topic` may be plain strings or refs. Passing refs
makes the composable re-fetch automatically when they change.

## Components

| Component | Props | Notes |
| --- | --- | --- |
| `<QuidnugStars>` | `product`, `topic`, `max?` (default 5), `showCount?`, emits `@rating` | Compact star widget. Fetches its own data via `useTrustWeightedRating`. |
| `<QuidnugReviewPanel>` | `product`, `topic`, `showWrite?` | Full review experience: headline rating + sorted review list + (optional) inline write form. |
| `<QuidnugReviewList>` | `product`, `topic`, `contributions?`, `sort?` (`weight` \| `recent` \| `rating-high` \| `rating-low`), `limit?` | Sorted review list. Highlights reviews that contributed to your weighted rating. |
| `<QuidnugWriteReview>` | `product`, `topic`, emits `@success` | Inline form for posting a new review. Requires `quid.has_private_key`. |

### Primitives

The low-level primitives are still exported for cases where you
already have rating data and just want to render:

| Component | Purpose |
| --- | --- |
| `<QnAurora>` | Headline rating glyph. Sentiment dot + confidence ring + optional delta chip. Three sizes: `nano`, `standard`, `large`. |
| `<QnConstellation>` | Bullseye drilldown. Concentric tiers of trust, one dot per contributor. |
| `<QnTrace>` | Horizontal stacked weight bar. One segment per contributor. |

See [../web-components/stories/index.html](../web-components/stories/index.html) for every visual state.

## License

Apache-2.0.
