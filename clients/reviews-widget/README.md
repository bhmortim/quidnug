# @quidnug/reviews-widget

The simplest possible Quidnug reviews integration: a **single
line of HTML** that works on any website, no JS build step
required.

## Install

Paste into your page:

```html
<quidnug-review
    product="your-product-id"
    topic="reviews.public.technology.laptops"
    src="https://widget.quidnug.dev/v2/"
></quidnug-review>
<script src="https://widget.quidnug.dev/v2/loader.js" defer></script>
```

That's it. The loader script registers the custom element and
an iframe-based fallback for browsers without Custom Elements
V1.

## What you get

- Per-observer, trust-weighted star rating.
- Full review list with per-review weight display.
- Optional inline write-review form.
- Schema.org JSON-LD for SEO.
- Works in React, Vue, Angular, Svelte, plain HTML, Wix,
  Squarespace, Webflow — anywhere that lets you paste HTML.

## Iframe fallback

For environments that disallow custom elements or block
`<script>` tags (some CMSes, email newsletters), the same
widget works as an iframe:

```html
<iframe
    src="https://widget.quidnug.dev/v2/?product=your-product-id&topic=reviews.public.technology.laptops"
    width="100%"
    height="600"
    style="border: 0;"
    loading="lazy"
    allow="clipboard-read; clipboard-write"
></iframe>
```

## Sizing attributes

| Attribute | Default | Description |
| --- | --- | --- |
| `product` | (required) | Canonical product asset id |
| `topic` | (required) | Topic domain |
| `compact` | off | Stars-only mode |
| `show-write` | off | Include write-review form |
| `show-schema` | on | Emit Schema.org JSON-LD |
| `theme` | `light` | `light` / `dark` / `auto` |
| `node-url` | public node | Override node URL |

## CSP notes

If your site uses Content Security Policy, allow:

```
script-src https://widget.quidnug.dev;
connect-src https://public.quidnug.dev;
frame-src https://widget.quidnug.dev;
```

## Self-hosting

The widget is Apache-2.0. If you prefer to self-host:

1. Clone the repo.
2. Build:
   ```bash
   cd clients/reviews-widget
   npm install
   npm run build
   ```
3. Deploy `dist/` to your CDN.

## License

Apache-2.0.
