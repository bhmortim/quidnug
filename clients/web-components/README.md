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

## Visualization primitives

Three zero-dependency SVG custom elements that live under
`@quidnug/web-components/src/primitives/`. They're the building
blocks used by `<quidnug-review>` internally, and available
directly when you want to render a rating glyph from state you
already have (without the networking layer).

### `<qn-aurora>`

The headline rating glyph: sentiment dot + confidence ring +
optional delta chip + optional radial histogram of contributor
ratings. Three sizes share one visual vocabulary.

```html
<qn-aurora
    size="standard"
    rating="4.5"
    crowd="4.1"
    contributors="7"
    direct="5"
    observer-name="alice"
    show-delta
    show-histogram
    contributor-ratings="[4.5, 4.8, 4.2, 5, 4, 4.3, 4.7]"></qn-aurora>
```

Key attributes: `size` (`nano` / `standard` / `large`),
`rating`, `max` (default 5), `crowd`, `contributors`, `direct`,
`observer-name`, `show-value`, `show-delta`, `show-histogram`,
`contributor-ratings` (JSON array). Fires `qn-aurora-click` on
interaction.

### `<qn-constellation>`

Bullseye drilldown. Concentric tiers encode trust-hop distance
from the observer; each dot is one contributing reviewer.

```html
<qn-constellation
    size="standard"
    observer-name="alice"
    title-text="Your trust map for this rating"
    contributors='[
        {"id":"v","name":"veteran","rating":4.8,"weight":0.33,"hops":1},
        {"id":"s","name":"sam","rating":4.5,"weight":0.22,"hops":2}
    ]'></qn-constellation>
```

Fires `qn-constellation-select` with the contributor object when
any dot is clicked.

### `<qn-trace>`

Horizontal stacked weight bar. One segment per contributor.
Ideal for comparing multiple products side-by-side.

```html
<qn-trace
    show-labels
    contributors='[
        {"id":"v","name":"veteran","rating":4.8,"weight":0.6,"direct":true},
        {"id":"s","name":"sam","rating":4.5,"weight":0.2,"direct":true}
    ]'></qn-trace>
```

### Design tokens

All three primitives draw from a single
[`design-tokens.js`](src/design-tokens.js) module that's
overridable via CSS custom properties
(`--qn-sentiment-*`, `--qn-delta-*`, `--qn-font`, …).
Every color distinction is paired with a shape distinction for
color-blind accessibility.

### SSR

Each primitive exposes a pure renderer (`renderAuroraSVG`,
`renderConstellationSVG`, `renderTraceSVG`) used directly by
the [`@quidnug/astro-reviews`](../astro-reviews/) adapter for
server-side rendering. Search engines and feed readers see the
exact same SVG the interactive element eventually renders.

### Storybook

[`stories/index.html`](stories/index.html) renders every state
variant side-by-side (rich / sparse / crowd-only / polarized /
no-basis at all three sizes, plus a full drilldown and a
product-list example). Open it directly in a browser — no
build step required.

### Design doc

See [`docs/reviews/rating-visualization.md`](../../docs/reviews/rating-visualization.md)
for the full design rationale, framework-adapter roadmap, and
40-platform integration target list.

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
