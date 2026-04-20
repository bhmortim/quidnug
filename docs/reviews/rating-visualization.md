# Rating visualization system

The Quidnug rating UI is built from three zero-dependency SVG
primitives that share a common visual language and render the
same data at three different information densities.

This document covers what each primitive is for, how they
compose, how they handle accessibility, and the roadmap of
framework adapters.

## Why not stars

Stars encode exactly one dimension: the rating value. Quidnug
ratings carry more signal than that:

- The **personalization delta** (your trust-weighted rating vs.
  the crowd's unweighted average).
- The **confidence** (how many trusted sources contributed).
- The **trust proximity** (direct circle vs. friends-of-friends
  vs. extended network).
- The **polarization** (whether trusted sources agree).
- The **freshness** (recency-decayed weight).
- The **factor decomposition** (topic trust T, helpfulness H,
  activity A, recency R).

Collapsing all of that into five stars throws away 80% of what
makes Quidnug's ratings useful. We keep stars around for SEO
(Schema.org JSON-LD carries the raw number for rich results)
but use a richer graphic family for humans.

## The three primitives

### `<qn-aurora>`

A sentiment dot surrounded by a confidence ring, with an
optional delta chip and numeric rating. Same visual vocabulary
at three sizes: `nano` (product grid), `standard` (detail
page), `large` (hero).

**Encodes:** rating value (dot color + numeric), confidence
(ring thickness), trust directness (ring dash pattern),
personalization delta (chip).

**Use when:** you need a single glanceable rating indicator.

### `<qn-constellation>`

A bullseye of concentric tiers (you → direct → 2-hop → 3+ →
crowd) with one dot per contributing reviewer. Dot color =
their rating; dot size = their weight; dot position = their
trust tier; dot outline = direct vs. transitive.

**Encodes:** the entire trust graph relevant to this rating,
at a glance. Clicking a dot reveals the trust path.

**Use when:** the user wants to understand *why* they see the
number they see. The "why this rating" drilldown.

### `<qn-trace>`

A horizontal stacked bar. One segment per contributor; width =
relative weight, color = their rating, outline = direct vs.
transitive.

**Encodes:** weight composition across contributors.

**Use when:** comparing multiple items side by side, because
you can eyeball "mostly wide solid green" versus "narrow mixed
dashed" across a list of products.

## Composition pattern

```
 nano list view           detail page hero            drilldown
 ┌──────────┐           ┌───────────────┐           ┌───────────────┐
 │ ●4.5 ↑0.4│           │    ┌─────┐    │           │   ┌─────┐     │
 └──────────┘           │    │ 4.5 │    │           │   │ 4.5 │     │
                        │    └─────┘    │           │   └─────┘     │
                        │  qn-aurora    │           │  qn-aurora    │
                        └───────────────┘           │               │
                                                    │  ┌─────────┐  │
                                                    │  │ ◎  ●  ● │  │
                                                    │  │ ◉  ●  ○ │  │
                                                    │  └─────────┘  │
                                                    │qn-constellation│
                                                    │               │
                                                    │ ▓▓▓▓░░░▒▒     │
                                                    │  qn-trace     │
                                                    └───────────────┘
```

All three primitives consume the same contributor-array shape,
so a host page computes rating state once and passes it into
whichever primitives it needs.

## Accessibility guarantees

**WCAG 2.1 AA target.** The primitives are designed and tested
to meet AA out of the box; hosts can drop them in without
additional work.

### Color redundancy

Every color distinction is backed by shape or stroke
redundancy:

- **Sentiment colors are paired with shapes.** Good ratings
  render as a dot, mixed as a square, bad as a triangle. A
  color-blind user sees a different shape even if the colors
  merge.
- **Direct vs. transitive trust.** Solid vs. dashed outline on
  every contributor dot. Screen-reader text names the trust
  distance explicitly.
- **Delta direction.** `↑` and `↓` symbols plus sign prefix;
  not color-only.

### Screen readers

Every primitive exposes a plain-language `aria-label` computed
from `describeState()`. Example aurora output:

> 4.5 out of 5, great, 7 trusted sources contributed, higher
> than public average, from people you trust directly.

Every contributor dot in the constellation has a `<title>`
with the same attributes a sighted user sees on hover:

> veteran · 4.8 out of 5 · 1 hop · weight 0.330

### Keyboard navigation

- `<qn-aurora>` is a single focus target (`tabindex=0`), fires
  `qn-aurora-click` on Enter/Space.
- `<qn-constellation>` contributor dots are each focusable;
  Enter/Space fires `qn-constellation-select`.
- `<qn-trace>` uses WAI-ARIA `list` and `listitem` roles.

Focus is always visible via a 2px outline with 3px offset.

### Motion

All transitions respect `prefers-reduced-motion`. No auto-play
animations anywhere in the default render.

### Text sizing

Numeric rating is rendered as proper SVG text, not a raster,
so browser zoom and text scaling work correctly. The digit
size scales with the overall primitive size (11px at nano,
22px at standard, 44px at large).

### RTL

The aurora is circularly symmetric. Delta chips and trace bars
flip orientation correctly under RTL scripts; we test with
`<html dir="rtl">` during the accessibility review pass.

## SEO / SSR

All three primitives expose pure renderer functions
(`renderAuroraSVG`, `renderConstellationSVG`, `renderTraceSVG`)
that produce identical markup to the client-hydrated version.
The Astro adapter uses these for server-side rendering.

For Schema.org rich results, the higher-level widget still
emits a standard `<script type="application/ld+json">` block
with `AggregateRating` / `Review` carrying the public
unweighted average. Crawlers see a familiar 4.1-star rating;
humans see the personalized 4.5 with its confidence ring.

## Framework adapters

| Package | Status |
| --- | --- |
| `@quidnug/web-components` — plain HTML, any framework | shipped |
| `@quidnug/react-reviews` — React wrappers | shipped |
| `@quidnug/vue-reviews` — Vue 3 wrappers | shipped |
| `@quidnug/astro-reviews` — SSR-first Astro | shipped |
| `@quidnug/wordpress-plugin` — WooCommerce | existing, to be updated to use new primitives |
| `@quidnug/shopify-app` — Shopify product pages | existing scaffold, to be wired to primitives |
| Svelte adapter | planned |
| SolidJS adapter | planned |
| Ember add-on | planned |
| Angular directive | planned |

All adapters are thin wrappers over `@quidnug/web-components`.
When you write a new adapter, copy the React wrappers as a
template (40 lines per primitive, most of which is prop
marshaling).

## Integration targets

The primitives plug into most rating contexts on the web.
Priority integrations for the reviews system:

### E-commerce
- **WooCommerce** (WordPress plugin) — product pages, catalog grids, checkout social proof
- **Shopify** — product detail pages, collection pages
- **Magento / Adobe Commerce** — product pages, category listings
- **BigCommerce** — storefront rating widgets
- **PrestaShop** — rating displays on product cards
- **Wix Stores** / **Squarespace Commerce** — via embed script
- **Webflow Ecommerce** — via `<script>` drop-in

### Content management
- **WordPress** — post ratings, comment weighting (beyond WooCommerce)
- **Joomla** — article ratings, K2 items
- **Drupal** — custom rating fields
- **Ghost** — post + author credibility
- **Contentful / Strapi / Sanity** — headless CMS product pages via React/Vue/Astro adapters

### Review-specific platforms
- **Yotpo** / **Trustpilot** / **Bazaarvoice** — trust-weighted layer overlaid on imported reviews
- **Judge.me** — WooCommerce review enrichment
- **Product Reviews Shopify app** — enrichment

### Video & media
- **YouTube / Vimeo** channel ratings (creator reputation)
- **Podcast directory** rating pages
- **IMDb-style movie / show rating** (community plus personal network)

### People / services
- **LinkedIn-like endorsements** (expert credibility per skill)
- **GitHub contributor / repo quality** indicators
- **Yelp / TripAdvisor** — place ratings filtered by your network
- **Healthgrades / Zocdoc** — practitioner ratings
- **TaskRabbit / Thumbtack** — service-provider quality
- **Upwork / Fiverr** — freelancer reputation

### Social + discussion
- **Reddit / Lemmy / Mastodon** — comment quality weighting
- **Hacker News / Lobsters** — user comment reputation overlay
- **Discord / Slack** — message / contributor credibility bots
- **Stack Overflow** — answer quality beyond upvotes

### News & info
- **News article credibility** (publisher + author weighted)
- **Wikipedia edit trust**
- **Fact-checking sites**

### Everything else
- **App store ratings** (Play Store / App Store bridges)
- **Hardware review aggregators** (Tom's Hardware style)
- **Academic paper credibility** (Semantic Scholar overlay)
- **Restaurant menus** — per-dish ratings
- **Real estate listings** — property + agent ratings

Most of these can be a one-day integration using the
appropriate framework adapter; the primitives do the heavy
visual lifting, and the platform-specific code is just
"where does the data live and where does the glyph go."

## Adding a new integration

1. Pick the adapter closest to your target platform's
   rendering model (React for SPAs, Astro for SSR content
   sites, plain web-components for CMSes that only accept
   HTML).
2. Fetch or compute the personal + crowd rating and the
   contributor array using `@quidnug/client`.
3. Render `<QnAurora>` everywhere stars currently appear; use
   `<QnConstellation>` + `<QnTrace>` in drilldown panels.
4. Ship Schema.org JSON-LD alongside the visual for SEO.
5. Ship a self-hostable fallback CDN path so the integration
   works without requiring the host to run a Quidnug node
   directly.

See `clients/wordpress-plugin/` and `examples/reviews-and-comments/demo/`
for reference integrations.

## Theming

Hosts override any token via CSS custom property on an
ancestor of the widget. Common customizations:

```css
:root {
    --qn-sentiment-great: #0f7b0f;       /* brand green */
    --qn-sentiment-good: #2aa52a;
    --qn-sentiment-mixed: #e8ac00;
    --qn-sentiment-poor: #d96c00;
    --qn-sentiment-bad: #cc2929;
    --qn-sentiment-no-basis: #999;
    --qn-delta-positive: #0f7b0f;
    --qn-delta-negative: #1a6ac9;
    --qn-font: "Inter", system-ui, sans-serif;
}
```

Every token declared in `design-tokens.js` maps to a matching
CSS variable; host pages can override any of them.

## Roadmap

- Figma component library mirroring the primitives so designers
  can compose custom layouts without us shipping a hundred variants.
- A formal accessibility audit with real screen readers + a
  keyboard-only walkthrough; publish findings in `docs/reviews/a11y-report.md`.
- Polarization glyph variant: when your network is split, the
  ring renders as two half-arcs of different colors.
- Temporal variant: when all contributing reviews are ancient,
  the aurora renders with a clock overlay.
- `<QuidnugRatingBadge>` higher-level React/Vue component that
  fetches + computes + renders in one drop-in.
- `@quidnug/figma-plugin` lets designers drop the primitive in
  Figma and have it render real data from a Quidnug node.
