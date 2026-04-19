# @quidnug/web-components

Drop-in web components for trust-weighted reviews. Works with
any framework (or no framework) — import once, use the tags
anywhere in HTML.

## Install

```bash
npm install @quidnug/web-components @quidnug/client
```

## Minimal page

```html
<!doctype html>
<html>
<head><meta charset="utf-8"><title>Example Product</title></head>
<body>
    <h1>Example Laptop</h1>

    <quidnug-stars product="laptop-xps15" topic="reviews.public.technology.laptops"></quidnug-stars>
    <quidnug-review product="laptop-xps15" topic="reviews.public.technology.laptops" show-write></quidnug-review>

    <script type="module">
        import QuidnugClient from "@quidnug/client";
        import "@quidnug/client/v2";
        import "@quidnug/web-components";
        import { setClient, setObserverQuid } from "@quidnug/web-components/context.js";

        const client = new QuidnugClient({ defaultNode: "https://public.quidnug.dev" });
        setClient(client);

        // Optional — sign in with the browser extension if present
        if (window.quidnug) {
            const quids = await window.quidnug.listQuids();
            if (quids.length > 0) {
                setObserverQuid(quids[0]);
            }
        }
    </script>
</body>
</html>
```

## Components

### `<quidnug-stars>`

The tiny per-observer weighted-rating widget. Ideal for
product-card-sized UI.

| Attribute | Required | Default | Description |
| --- | --- | --- | --- |
| `product` | yes | — | Canonical product asset id |
| `topic` | yes | — | Topic domain (e.g. `reviews.public.technology.laptops`) |
| `max` | no | `5` | Max rating to display |
| `show-count` | no | off | Show `(N trusted)` suffix |

Events:
- `quidnug-stars-ready` on compute completion.

### `<quidnug-review>`

Full review panel: aggregate rating + review list + optional
inline write form.

| Attribute | Required | Default | Description |
| --- | --- | --- | --- |
| `product` | yes | — | Canonical product asset id |
| `topic` | yes | — | Topic domain |
| `show-write` | no | off | Include the `<quidnug-write-review>` inline |

Events:
- `quidnug-review-rating` when the aggregate is computed.
- `quidnug-review-submitted` after a user posts a new review.

### `<quidnug-write-review>`

Inline review-writing form. Requires `setObserverQuid()` with
a signing-capable Quid (usually from the browser extension).

### `<quidnug-review-list>`

Standalone list with per-review weight display. Supports
sorting by weight, recent, rating-high, rating-low.

| Attribute | Required | Default | Description |
| --- | --- | --- | --- |
| `product` | yes | — | |
| `topic` | yes | — | |
| `limit` | no | `20` | Max reviews to render |
| `sort` | no | `weight` | `weight` / `recent` / `rating-high` / `rating-low` |

## Wiring in the observer's quid

For anonymous viewers, weights compute against the empty trust
graph (most reviews get zero weight — that's the honest
answer). For signed-in users, pass their Quid:

```js
import { setObserverQuid } from "@quidnug/web-components/context.js";
import { Quid } from "@quidnug/client";

// From browser extension
const quid = await window.quidnug.getActiveQuid();
setObserverQuid(quid);

// Or from app-managed key
const quid = Quid.fromPrivateHex(userPrivHex);
setObserverQuid(quid);
```

The same `setObserverQuid` affects every component instance
on the page.

## Styling

Every component uses Shadow DOM for isolation, so host-page
styles don't leak in. Customize via CSS custom properties:

```css
quidnug-stars, quidnug-review, quidnug-review-list {
    --quidnug-font: 'Inter', sans-serif;
}
```

Full CSS custom properties exposed across all components:

| Property | Default |
| --- | --- |
| `--quidnug-font` | `system-ui, sans-serif` |

(More tokens planned — file an issue with what you need.)

## Production deployment

For production, configure your `QuidnugClient` to point at a
reliable public node (or your own replica of the
`reviews.public.*` tree). Caching:

```js
const client = new QuidnugClient({
    defaultNode: "https://public.quidnug.dev",
    maxRetries: 3,
    retryBaseDelayMs: 500,
});
```

For high-traffic sites, consider wrapping the rater in a
server-side cache keyed by `(observer, product, topic)` and
TTL-expire after 5 minutes. The underlying data is append-only,
so stale reads are safe.

## License

Apache-2.0.
