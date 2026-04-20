# @quidnug/react-reviews

React hooks + components for Quidnug trust-weighted reviews.

## Install

```bash
npm install @quidnug/react-reviews @quidnug/react @quidnug/client
```

## Thirty-second example

```jsx
import { QuidnugProvider } from "@quidnug/react";
import { QuidnugReviewPanel, QuidnugStars } from "@quidnug/react-reviews";

function App() {
    return (
        <QuidnugProvider node="https://public.quidnug.dev" defaultDomain="reviews.public">
            <h1>Example Laptop</h1>
            <QuidnugStars product="laptop-xps15" topic="reviews.public.technology.laptops" />
            <QuidnugReviewPanel
                product="laptop-xps15"
                topic="reviews.public.technology.laptops"
                showWrite
            />
        </QuidnugProvider>
    );
}
```

## Components

### `<QuidnugStars product topic />`

Per-observer trust-weighted stars, compact. Ideal for product
cards.

| Prop | Required | Default | Description |
| --- | --- | --- | --- |
| `product` | yes | | Canonical product asset id |
| `topic` | yes | | e.g. `reviews.public.technology.laptops` |
| `max` | no | 5 | Display max |
| `showCount` | no | false | Append `(N trusted)` |
| `onRating` | no | | Callback invoked when rating computes |

### `<QuidnugReviewPanel />`

Full review panel — headline rating + list + optional inline
write form.

| Prop | Required | Default |
| --- | --- | --- |
| `product` | yes | — |
| `topic` | yes | — |
| `showWrite` | no | false |

### `<QuidnugReviewList />`

Just the list. Useful when you want stars and the list in
different regions of the page.

### `<QuidnugWriteReview />`

Inline write form. Requires the `QuidnugProvider` to have an
active Quid set.

## Hooks

### `useTrustWeightedRating(product, topic)`

```jsx
const { data, loading, error, refetch } = useTrustWeightedRating(product, topic);
```

`data` shape:
```js
{
    rating: 4.47,              // 0-5
    confidenceRange: 0.38,
    contributingReviews: 3,
    totalReviewsConsidered: 5,
    totalWeight: 1.144,
    contributions: [
        { reviewerQuid, rating, weight, t, h, a, r, ageDays },
        ...
    ]
}
```

### `useReviews(product, topic)`

```jsx
const { data: reviews, loading } = useReviews(product, topic, { limit: 50 });
```

### `useWriteReview()`

```jsx
const { mutate, loading, error } = useWriteReview();
await mutate({ product, topic, rating: 4.5, title, body });
```

## Primitives (from `@quidnug/react-reviews/primitives`)

Thin React wrappers over the zero-dependency SVG visualization
primitives in `@quidnug/web-components`. Import these when you
already have rating state computed (for example via
`useTrustWeightedRating` above) and just want to render.

### `<QnAurora />`

The headline rating glyph.

```jsx
import { QnAurora } from "@quidnug/react-reviews";

<QnAurora
    size="standard"
    rating={4.5}
    crowd={4.1}
    contributors={7}
    direct={5}
    observerName="alice"
    showDelta
    showHistogram
    contributorRatings={[4.5, 4.8, 4.2, 5, 4, 4.3, 4.7]}
    onAuroraClick={(detail) => openDrilldown(detail)}
/>
```

### `<QnConstellation />`

Bullseye drilldown — every contributor as a dot on a tier
keyed to their trust-hop distance.

```jsx
<QnConstellation
    size="standard"
    observerName="alice"
    titleText="Your trust map for this rating"
    contributors={[
        { id: "v", name: "veteran", rating: 4.8, weight: 0.33, hops: 1 },
        { id: "s", name: "sam",     rating: 4.5, weight: 0.22, hops: 2 },
    ]}
    onSelect={(c) => showTrustPath(c.id)}
/>
```

### `<QnTrace />`

Horizontal stacked weight bar — useful in product grids.

```jsx
<QnTrace
    contributors={[
        { id: "v", name: "veteran", rating: 4.8, weight: 0.6, direct: true },
        { id: "s", name: "sam",     rating: 4.5, weight: 0.2, direct: true },
    ]}
    showLabels
/>
```

See the design doc at [`docs/reviews/rating-visualization.md`](../../docs/reviews/rating-visualization.md)
for the visual vocabulary and a [storybook](../web-components/stories/index.html)
with every state variant.

## Examples

See [`examples/`](examples/) for full pages:

- `product-page.jsx` — typical e-commerce product-page layout
- `review-dashboard.jsx` — "reviewer reputation" dashboard
- `moderation-ui.jsx` — trust-filtered moderation queue

## License

Apache-2.0.
