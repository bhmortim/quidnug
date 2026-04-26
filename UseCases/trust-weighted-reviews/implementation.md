# Implementation: trust-weighted reviews

> Concrete code paths and integration recipes. Companion to
> [`README.md`](README.md) (market case) and
> [`architecture.md`](architecture.md) (system design).
> The protocol-level reference is QRP-0001 in
> [`../../examples/reviews-and-comments/PROTOCOL.md`](../../examples/reviews-and-comments/PROTOCOL.md);
> the rating algorithm is in
> [`../../examples/reviews-and-comments/algorithm.py`](../../examples/reviews-and-comments/algorithm.py).

## 1. Five integration patterns

| Pattern                     | Audience                                  | Effort        | Library                              |
|-----------------------------|-------------------------------------------|---------------|--------------------------------------|
| Drop-in iframe widget       | Any site, no build step                   | 1 line of HTML| `@quidnug/reviews-widget`            |
| Web Components              | Any framework, modern site                | npm install   | `@quidnug/web-components`            |
| React / Vue / Astro adapter | App-framework users                       | npm install   | `@quidnug/react-reviews` etc.        |
| WordPress plugin            | WP / WooCommerce sites                    | upload zip    | `clients/wordpress-plugin/`          |
| Browser-extension overlay   | Observers reading existing review sites   | install ext.  | `clients/browser-extension/`         |

All five share the same underlying client and the same
event/trust model.

## 2. Pattern 1: drop-in iframe widget (zero-build)

For a static product page on any site:

```html
<!-- inside the page <head> -->
<script
  src="https://cdn.quidnug.com/widget/v1/reviews-widget.js"
  defer></script>

<!-- where you want reviews to appear -->
<quidnug-reviews
  asin="B0C1234ABC"
  ean="0123456789012"
  category="reviews.public.technology.laptops"
  api="https://api.quidnug.com">
</quidnug-reviews>
```

What happens:
- The widget computes the deterministic product Title quid
  from the canonical identifiers.
- It fetches REVIEW + HELPFUL_VOTE + FLAG events from the
  Quidnug network.
- It detects whether a Quidnug wallet extension is installed.
  - If yes: pulls the observer's trust graph, computes
    per-observer rating, renders.
  - If no: renders the anonymous-baseline rating (operator
    root weighting only) with a "Personalize my reviews"
    prompt linking to wallet install.

Five-minute integration; no backend changes.

## 3. Pattern 2: web components (more control)

For sites that want native UI, SEO, server-side rendering:

```html
<script type="module"
  src="https://cdn.quidnug.com/wc/v1/quidnug-reviews.js"></script>

<qn-aurora
  product="<product-title-quid>"
  category="reviews.public.technology.laptops"
  size="large"></qn-aurora>

<qn-constellation
  product="<product-title-quid>"
  category="reviews.public.technology.laptops"
  observer-mode="auto"></qn-constellation>

<qn-trace
  product="<product-title-quid>"
  category="reviews.public.technology.laptops"></qn-trace>

<quidnug-write-review
  product="<product-title-quid>"
  category="reviews.public.technology.laptops"
  on-submit="..."></quidnug-write-review>
```

Three visualization primitives (aurora = single rating,
constellation = drilldown of contributors, trace = audit path
showing which trust edges matter most) plus a write-review
form.

Documentation:
[`../../docs/reviews/rating-visualization.md`](../../docs/reviews/rating-visualization.md).

## 4. Pattern 3: React/Vue/Astro adapter

```jsx
// React
import { Aurora, Constellation, WriteReview }
  from '@quidnug/react-reviews';

export default function ProductPage({ product }) {
  return (
    <>
      <Aurora product={product.titleQuid}
              category="reviews.public.technology.laptops" />
      <Constellation product={product.titleQuid}
                     category="reviews.public.technology.laptops" />
      <WriteReview product={product.titleQuid}
                   category="reviews.public.technology.laptops"
                   onSubmit={handleSubmit} />
    </>
  );
}
```

For Astro (SSR, SEO-friendly):

```astro
---
// src/pages/products/[slug].astro
import { Aurora, Constellation } from '@quidnug/astro-reviews';
import { client } from '../../lib/quidnug.js';

const { slug } = Astro.params;
const product = await getProduct(slug);
const initialRating = await client.getRating(
  product.titleQuid,
  'reviews.public.technology.laptops',
  /* anonymous observer */ null
);
---
<Aurora product={product.titleQuid}
        category="reviews.public.technology.laptops"
        initialRating={initialRating} />
<Constellation product={product.titleQuid}
               category="reviews.public.technology.laptops" />
```

The Astro version renders real SVG at build/SSR time so search
engines see populated ratings, not empty placeholders.

## 5. Pattern 4: WordPress / WooCommerce plugin

```bash
# In WP admin → Plugins → Add New → Upload
# Upload clients/wordpress-plugin/quidnug-reviews.zip
# Activate.
# Then in Settings → Quidnug Reviews:
#   - API endpoint: https://api.quidnug.com
#   - Default category: reviews.public.products  (or set per-product)
#   - Site quid: (paste from Quidnug console after validating site domain)
#   - Site key: (paste from Quidnug console)
```

Per-product overrides via a WooCommerce custom field. The
plugin renders the reviews block automatically on product
pages, replacing or augmenting the built-in WooCommerce
reviews UI.

For a site that's already Quidnug-validated (Pro/Business
tier through `service.quidnug.com`), the plugin can also
issue PURCHASE attestations on completed orders so reviews
from buyers carry verified-purchase status.

## 6. Pattern 5: browser-extension overlay

The observer-side wedge. Doesn't require any site to
integrate; runs entirely in the consumer's browser.

```bash
# Build:
cd clients/browser-extension
npm install
npm run build:chromium  # or :firefox

# Install: chrome://extensions → Load unpacked → dist/
```

What it does:
- Detects product pages on Amazon, Yelp, Google Maps,
  TripAdvisor, Booking, etc.
- Extracts canonical identifiers (ASIN from URL, business
  name + address from page).
- Computes the Quidnug product Title.
- Fetches REVIEW events from `api.quidnug.com`.
- Renders a side panel showing the per-observer trust-weighted
  rating, plus diff against the host site's average.
- Optionally lets the observer write a review (signed by their
  wallet quid) without leaving the page.

This is the consumer-facing wedge: people see the difference
without sites needing to integrate.

## 7. Reviewer wallet (signing identity)

The wallet holds the reviewer's quid and signs events.
Implementations:

- **Browser extension** (`clients/wallet-extension/`): primary
  for desktop reviewers. Manages keys, prompts for
  signature on each review.
- **Mobile app** (`clients/mobile-wallet/` planned): for
  in-store / restaurant review writing on phones.
- **Hosted wallet via OIDC bridge**: lowest friction. Reviewer
  signs in with Google/GitHub at `auth.quidnug.com`; the
  bridge holds a custodial key. Trade-off: convenience vs.
  custody. Pro reviewers should self-custody; casual reviewers
  use the bridge.

Key recovery for self-custody: guardians (QDP-0002), 3-of-5
default with a 24-hour time-lock. Wallet prompts the user
through guardian setup at first launch.

## 8. Site validation: the Quidnug Pro/Business signup flow

This is what your service-api Worker provides (see service-api
spec). For a site operator like acmestore.com:

```
1. Visit quidnug.com/pricing → click "Pro $19/month".
2. Stripe Checkout. On completion:
   - Stripe webhook fires service.quidnug.com/v1/webhooks/stripe.
   - Worker creates Customer + customer-quid (deterministic
     from email+salt) + initial API key.
   - Email sent with key + console link.
3. In console (app.quidnug.com), customer adds domain
   acmestore.com → POST /v1/domains.
   - Worker returns DNS challenge: TXT record at
     _quidnug-challenge.acmestore.com.
4. Customer publishes the TXT (typically via their DNS provider
   in <5 min).
5. Customer clicks "Verify" → POST /v1/domains/:id/verify.
6. Worker queries Cloudflare/Google/Quad9/OpenDNS via DoH;
   requires 3-of-4 quorum match.
7. On pass: Worker submits signed TRUST tx to api.quidnug.com:
   - truster:    validation-operator-quid
   - trustee:    acmestore.com's customer-quid
   - domain:     operators.acmestore.com.network.quidnug.com
   - level:      0.8 (Pro) or 0.95 (Business with KYB)
   - attributes: { tier, signedAt, challengeId, checkId,
                   resolversUsed }
8. On 2xx from api.quidnug.com:
   - Worker stores trustEdgeTxId in D1.
   - State → active.
   - R2 attestation bundle (signed JSON + raw evidence + PDF)
     written; downloadUrl emailed.
   - domain.validated webhook fires.
9. Cron re-probes every 1 hour (Pro) / 15 min (Business).
   On 4-hour grace fail: TRUST level=0.0 superseding edge.
   State → revoked. domain.revoked webhook.
```

Pseudocode for the verification step inside the Worker:

```typescript
async function verifyDomain(domainId: string, env: Env) {
  const domain = await env.DB.prepare(
    "SELECT * FROM domain WHERE id=?"
  ).bind(domainId).first<DomainRow>();

  const challenge = await env.DB.prepare(
    "SELECT * FROM challenge WHERE domain_id=? " +
    "AND consumed_at IS NULL ORDER BY id DESC LIMIT 1"
  ).bind(domainId).first<ChallengeRow>();

  if (!challenge || Date.now() > challenge.expires_at) {
    throw new ApiError("challenge_expired", 400);
  }

  const expected = `qn-chl=v1;nonce=${challenge.nonce}`;
  const resolvers = [
    { name: "cloudflare", url: "https://cloudflare-dns.com/dns-query" },
    { name: "google",     url: "https://dns.google/resolve" },
    { name: "quad9",      url: "https://dns.quad9.net:5053/dns-query" },
    { name: "opendns",    url: "https://doh.opendns.com/dns-query" },
  ];

  const probes = await Promise.all(
    resolvers.map(r => probeDoh(r, challenge.record_name, expected))
  );

  const passes = probes.filter(p => p.ok).length;
  const checkId = ulid();

  await env.DB.prepare(
    "INSERT INTO check_log VALUES (?,?,?,?,?,?)"
  ).bind(
    checkId, domainId, "initial",
    passes >= 3 ? "pass" : "fail_mismatch",
    JSON.stringify({ resolvers: probes, quorum: { needed: 3, got: passes }}),
    Date.now()
  ).run();

  if (passes < 3) {
    throw new ApiError("verification_failed", 422,
      { resolvers: probes, quorumNeeded: 3, quorumGot: passes });
  }

  // Quorum met; publish TRUST edge.
  const tx = await buildTrustEdgeTx({
    truster:  env.VALIDATION_OPERATOR_QUID,
    trustee:  domain.customer_quid,
    domain:   `operators.${domain.fqdn}.network.quidnug.com`,
    level:    levelForTier(domain.tier),
    nonce:    await nextSignerNonce(env, env.VALIDATION_OPERATOR_QUID),
    signKey:  await loadKey(env.VALIDATION_OPERATOR_KEY),
    attributes: {
      tier: domain.tier,
      signedAt: new Date().toISOString(),
      challengeId: challenge.id,
      checkId,
      resolversUsed: probes.map(p => p.name),
    },
  });

  const txResult = await fetch(
    "https://api.quidnug.com/api/transactions",
    { method: "POST", body: JSON.stringify(tx),
      headers: { "Content-Type": "application/json" } }
  );

  if (!txResult.ok) {
    throw new ApiError("protocol_unavailable", 502);
  }

  const { id: txId } = await txResult.json();

  await env.DB.batch([
    env.DB.prepare(
      "UPDATE domain SET state='active', trust_edge_tx_id=?, " +
      "last_verified_at=?, last_check_id=? WHERE id=?"
    ).bind(txId, Date.now(), checkId, domainId),
    env.DB.prepare(
      "UPDATE challenge SET consumed_at=? WHERE id=?"
    ).bind(Date.now(), challenge.id),
  ]);

  await enqueueAttestationGeneration(env, domainId, txId);
  await emitWebhook(env, domain.customer_id, "domain.validated",
    { domainId, fqdn: domain.fqdn, trustEdgeTxId: txId });

  return { state: "active", trustEdgeTxId: txId, checkId };
}
```

## 9. Reviewer validation: the pro-reviewer signup

Same Worker, slightly different messaging on the front end.
For Alice the food critic:

```
1. Visit quidnug.com/professional-reviewer (or just /pricing).
2. Stripe Checkout, Pro tier.
3. In console, Alice adds her domain alice-eats.com.
4. Same DNS challenge → resolver quorum → TRUST edge.
5. After validation, Alice's reviewer card in widgets shows:
   "alice-eats.com ✓ Pro-validated YYYY-MM-DD"
6. She can now claim ownership of her existing reviewer quid:
   POST /v1/reviewer/claim with a signature from her quid
   over a challenge from the Worker. The Worker publishes a
   second TRUST edge from her validated alice-eats.com customer
   quid → her existing reviewer quid in
   reviews.public.restaurants. This binds the two together
   on-chain.
7. Optional: Alice upgrades to Business ($99/mo) and goes
   through KYB via Stripe Identity. Once approved, the Worker
   re-signs her edge at level 0.95 and her reviewer card adds
   "KYB verified."
```

This is the same product, sold to a different buyer.

## 10. OIDC bridge for low-friction reviewer onboarding

For consumers who don't want to install a wallet extension or
buy a domain. Lives at `auth.quidnug.com`, separate Worker /
service. Implementation:

- Standard OIDC OP supporting Google, GitHub, Apple,
  Microsoft.
- On first sign-in: derive a deterministic quid from the
  hash of `provider_subject_id + email + AUTH_BRIDGE_SALT`.
  Generate a custodial signing key, hold it server-side
  encrypted-at-rest.
- Issue a TRUST edge from `operators.network.quidnug.com` to
  the new quid at level 0.2 (configurable) in
  `reviews.public.*` (or a list of subdomains the user
  selects on first sign-in).
- Provide a JSON-RPC signing API the widgets can hit when the
  user is logged in: "sign this REVIEW event for me."
- Support graduation: a custodial user can upgrade to
  self-custody by providing a public key from a wallet
  extension; the bridge issues a TRUST edge from the
  custodial quid → the new self-custody quid at level 1.0
  and stops signing on behalf of the user.

The trade-off is custody-vs-friction, made visible to users.

## 11. Schema.org JSON-LD interop for SEO

Search engines (Google, Bing, DuckDuckGo) reward structured
review data with rich-result cards. Quidnug's `integrations/
schema-org/` package generates valid JSON-LD from QRP events:

```html
<!-- emitted server-side by the Astro adapter or WP plugin -->
<script type="application/ld+json">
{
  "@context": "https://schema.org",
  "@type": "Product",
  "name": "Example Brand XPS 15 9530",
  "aggregateRating": {
    "@type": "AggregateRating",
    "ratingValue": "4.3",
    "reviewCount": 247,
    "bestRating": "5",
    "worstRating": "1"
  },
  "review": [
    {
      "@type": "Review",
      "author": { "@type": "Person",
                  "name": "alice-eats.com",
                  "url": "https://alice-eats.com" },
      "datePublished": "2026-04-12",
      "reviewBody": "...",
      "reviewRating": {
        "@type": "Rating",
        "ratingValue": "4.5",
        "bestRating": "5"
      }
    }
  ]
}
</script>
```

The aggregateRating uses the **anonymous-observer** rating
(operator-rooted, no observer trust graph), so the search-
engine rich-result is consistent across observers. The
per-observer rating is rendered client-side over the top
when the observer's wallet is detected.

The package also goes the other way: importing existing
Schema.org Review JSON-LD from sites that have it (e.g.,
Schema.org-marked WordPress sites) into Quidnug events,
preserving the original timestamps and authorship where
identifiable. This supports cross-site reputation import for
established reviewers.

## 12. PURCHASE attestations (verified-purchase replacement)

The Amazon "verified purchase" badge is the highest-value
signal in current review systems and the most-attacked. QRP-
0001 specifies a PURCHASE event signed by the seller's quid:

```json
{
  "type": "PURCHASE",
  "subject_id": "<reviewer-quid>",
  "subject_type": "QUID",
  "domain": "reviews.public.technology.laptops",
  "payload": {
    "qrpVersion": 1,
    "productAssetQuid": "<product-title>",
    "purchaseTimestamp": "2026-04-15T10:23:00Z",
    "orderId": "...",
    "amount": { "value": 1899.00, "currency": "USD" },
    "verificationMethod": "stripe-charge-succeeded"
  },
  "signature": "...by acmestore.com customer quid..."
}
```

Sites that are Pro/Business validated through Quidnug carry
non-trivial weight when issuing PURCHASE attestations. Sites
that aren't validated can still issue them but observers may
discount them. Brushing (the empty-box attack) is harder
because it requires either a Pro/Business validated seller
willing to sign fraudulent PURCHASE events (and risk losing
their validation) or an unvalidated seller whose attestations
carry near-zero weight.

## 13. End-to-end test checklist

For an integrator validating their setup:

- [ ] Widget loads and renders without console errors.
- [ ] Anonymous observer sees a baseline rating that matches
      the operator-rooted computation in algorithm.py.
- [ ] Logged-in observer (with wallet) sees a different rating
      that matches their trust-graph-weighted computation.
- [ ] Posting a review from the widget produces a signed event
      visible at `api.quidnug.com/api/streams/<reviewer>/events`.
- [ ] Helpfulness vote on an existing review produces a
      HELPFUL_VOTE event visible in the same stream.
- [ ] PURCHASE attestation flow works for validated sites.
- [ ] Schema.org JSON-LD is present in HTML source and
      validates against Google Rich Results Test.
- [ ] Browser extension overlay displays correctly on
      Amazon/Yelp test pages.
- [ ] Domain validation flow completes end-to-end (DNS
      challenge → quorum → TRUST edge visible at
      `api.quidnug.com/api/trust/<truster>/<trustee>?domain=...`).

## 14. Migration from existing review systems

For a site with an existing review database:

1. Map the site's product IDs to canonical Schema.org
   identifiers (ASIN/UPC/EAN/ISBN). The deterministic
   product Title quid follows.
2. For each reviewer email, run the OIDC bridge offline (or
   ask reviewers to sign in once) to mint quids.
3. Bulk-import reviews via a one-time CLI: each review becomes
   a REVIEW event signed by the imported reviewer's quid,
   with the original timestamp preserved (sequence numbers
   reordered by timestamp). Mark them with an
   `imported_from: "<site-domain>"` attribute so observers
   can choose to weight imports differently.
4. Publish TRUST edges from the importing site's validated
   domain to the imported reviewers at a baseline level
   (e.g., 0.4) reflecting partial known-good status.
5. Run the existing system in parallel for 30-90 days; cut
   over once new reviews are flowing primarily through the
   Quidnug widget.

This pattern lets a Yelp or Amazon-class site preserve their
existing reviewer corpus while migrating to the new substrate
and giving reviewers portable identity going forward.
