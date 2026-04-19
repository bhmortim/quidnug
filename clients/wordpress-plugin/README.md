# Quidnug Reviews — WordPress plugin

Drop-in trust-weighted reviews for WordPress and WooCommerce. Per-
observer ratings, cross-site reviewer reputation, no proprietary
database.

## Install

**Option A — install from ZIP (pre-release):**

Download the latest release from
[GitHub releases](https://github.com/bhmortim/quidnug/releases),
then **Plugins → Add New → Upload Plugin** in WP admin.

**Option B — Composer / Git checkout:**

```bash
cd wp-content/plugins
git clone https://github.com/bhmortim/quidnug.git quidnug
ln -s quidnug/clients/wordpress-plugin quidnug-reviews
```

Then activate in WP admin.

## What it does

1. On every WooCommerce product page, replaces the default
   "Reviews" tab with a Quidnug review panel. Ratings are
   computed **per-observer** — a signed-in user sees stars
   weighted against their Quidnug trust graph; an anonymous
   visitor sees an unweighted average with a "sign in to see
   your personal rating" hint.
2. Adds two shortcodes:
   - `[quidnug-reviews product="..." topic="..."]` — full panel
   - `[quidnug-stars product="..." topic="..."]` — compact
3. Emits Schema.org `AggregateRating` JSON-LD for each product,
   compatible with Google / Bing / DuckDuckGo rich results.
4. Configurable category-to-topic mapping so
   `Category: Electronics` maps to
   `reviews.public.technology`, etc.

## Settings

**Settings → Quidnug Reviews** in WP admin:

| Setting | Description |
| --- | --- |
| Quidnug node URL | The Quidnug node to query. Default: the public node at `https://public.quidnug.dev` (planned). |
| Default topic domain | Fallback topic for posts without category mapping. |
| Replace WooCommerce reviews | When on, hides the built-in WP review form on product pages. |
| Emit Schema.org JSON-LD | SEO — emit AggregateRating for search crawlers. |

## Shortcode

Full panel, anywhere in a post:

```
[quidnug-reviews product="my-product-sku" topic="reviews.public.technology.laptops" show-write="1"]
```

Compact stars:

```
[quidnug-stars product="my-product-sku" topic="reviews.public.books"]
```

## Category → topic mapping

Edit the default mapping in `class-product-page.php` or filter it:

```php
add_filter('quidnug_reviews_category_topic_map', function ($map) {
    $map['wine']      = 'reviews.public.food.wine';
    $map['camping']   = 'reviews.public.outdoors';
    return $map;
});
```

## User sign-in

Reviewers need a Quidnug identity to post. Two paths:

1. **Browser extension** (`clients/browser-extension/`). User
   installs the extension once; the extension holds their quid
   and signs on-demand. Preferred for power users.
2. **OIDC bridge** (`cmd/quidnug-oidc/`). User signs in with
   Google / Facebook / Apple; the bridge mints a quid bound to
   their OIDC subject. Preferred for casual sites where asking
   users to install an extension is a blocker.

Neither requires any WordPress user-account integration — the
signed reviews are posted directly from the reviewer's browser
to the Quidnug network, bypassing WP entirely. WP is just
rendering the global data.

## Architecture

```
 ┌──────────────────────────────────────────────────────────────┐
 │ WordPress (this plugin)                                      │
 │  - Settings page                                             │
 │  - Shortcode handler                                         │
 │  - Schema.org JSON-LD for SEO                                │
 │  - Enqueues @quidnug/web-components bundle                   │
 └────────┬─────────────────────────────────────────────────────┘
          │ (no DB writes; just renders custom-element tags)
          ▼
 ┌──────────────────────────────────────────────────────────────┐
 │ Browser                                                      │
 │  - <quidnug-review> web component                            │
 │  - @quidnug/client v2                                        │
 │  - @quidnug/web-components Rater                             │
 │  - User's Quidnug extension / wallet                         │
 └────────┬─────────────────────────────────────────────────────┘
          │ HTTP to the public Quidnug network
          ▼
 ┌──────────────────────────────────────────────────────────────┐
 │ PUBLIC QUIDNUG NETWORK (reviews.public.*)                    │
 │  - Review events                                             │
 │  - Helpful votes                                             │
 │  - Trust edges                                               │
 └──────────────────────────────────────────────────────────────┘
```

## Privacy

- The plugin stores no review data on your WordPress database.
  Everything is public on the Quidnug network.
- Reviewers' quids are pseudonymous 16-char hex IDs. Users
  choosing to use the OIDC bridge associate their quid with
  an IdP subject (email), but the association lives on the
  bridge server, not on-chain.
- Cookie-free rendering: the plugin doesn't set any cookies or
  track visitors. The Quidnug client may make HTTP requests to
  the public node, but these aren't fingerprinting calls.

## Performance

- Schema.org JSON-LD computation is cached for 5 minutes
  (`set_transient`).
- The web-components bundle is loaded lazily on product pages
  only (not on every page).
- Per-observer rating computation runs in the visitor's browser;
  your WP server doesn't shoulder the load.

## Development

```bash
# Package for release
cd clients/wordpress-plugin
zip -r quidnug-reviews.zip . -x '*.DS_Store' '*.git*'

# Or symlink into your local WP
cd wp-content/plugins
ln -s /path/to/quidnug/clients/wordpress-plugin quidnug-reviews
```

Minimum requirements: **WordPress 6.2+, PHP 7.4+, WooCommerce
optional (for product-page integration).**

## License

Apache-2.0.
