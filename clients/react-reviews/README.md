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

## Examples

See [`examples/`](examples/) for full pages:

- `product-page.jsx` — typical e-commerce product-page layout
- `review-dashboard.jsx` — "reviewer reputation" dashboard
- `moderation-ui.jsx` — trust-filtered moderation queue

## License

Apache-2.0.
