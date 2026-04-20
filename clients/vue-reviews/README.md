# @quidnug/vue-reviews

Vue 3 wrappers around Quidnug's trust-weighted review
components. Thin adapter layer over `@quidnug/web-components`
that lets you use `<QnAurora>`, `<QnConstellation>`, and
`<QnTrace>` as first-class Vue components with props + events.

## Install

```bash
npm install @quidnug/vue-reviews @quidnug/web-components
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

## Use

```vue
<script setup>
import { QnAurora, QnTrace } from "@quidnug/vue-reviews";

const contributors = [
    { id: "vet",  name: "veteran",   rating: 4.8, weight: 0.6, direct: true },
    { id: "sam",  name: "sam-tech",  rating: 4.5, weight: 0.2, direct: true },
    { id: "kai",  name: "kai",       rating: 4.2, weight: 0.1, direct: false },
];

function openDrilldown(detail) {
    // show a modal with the full breakdown
}
</script>

<template>
    <QnAurora :rating="4.7"
              :contributors="7" :direct="5" :crowd="4.1"
              observer-name="alice"
              show-delta show-histogram
              :contributor-ratings="[4.5, 4.8, 4.2, 5, 4, 4.3, 4.7]"
              @aurora-click="openDrilldown" />

    <QnTrace :contributors="contributors" show-labels />
</template>
```

## Primitives

| Component | Purpose |
| --- | --- |
| `<QnAurora>` | Headline rating glyph. Sentiment dot + confidence ring + optional delta chip. Three sizes: `nano`, `standard`, `large`. |
| `<QnConstellation>` | Bullseye drilldown. Concentric tiers of trust, one dot per contributor. |
| `<QnTrace>` | Horizontal stacked weight bar. One segment per contributor. |

See [../web-components/stories/index.html](../web-components/stories/index.html) for every visual state.

## Roadmap

- Vue composables mirroring the React hooks (`useTrustWeightedRating`,
  `useReviews`, `useWriteReview`).
- High-level `<QuidnugReviewPanel>`, `<QuidnugStars>` Vue components
  that fetch + compute + render automatically.
- Nuxt integration (SSR-safe).

## License

Apache-2.0.
