# Quidnug Reviews — Shopify app (scaffold)

Shopify-native app scaffold for Quidnug trust-weighted reviews.

Status: **Scaffold with concrete design**; full app submission to
the Shopify App Store is on the roadmap once we have a published
marketplace-ready node at `public.quidnug.dev`.

## Architecture

Shopify apps integrate three ways:

1. **Theme App Extensions** — the modern way to drop UI onto
   a merchant's storefront. Uses Liquid blocks + JS.
2. **Checkout UI extensions** — for post-purchase "leave a
   review" prompts.
3. **Admin UI** — the merchant's settings page (node URL,
   topic mapping per collection).

This scaffold sketches all three.

## Theme App Extension

Theme extensions live in `extensions/quidnug-reviews-theme/`:

```liquid
{%- comment -%}
    Quidnug Reviews — product-page block.
    Shopify merchants drag this into product templates via
    the theme editor.
{%- endcomment -%}

<script src="https://cdn.jsdelivr.net/npm/@quidnug/client@2/quidnug-client.js"></script>
<script src="https://cdn.jsdelivr.net/npm/@quidnug/client@2/quidnug-client-v2.js"></script>
<script type="module"
        src="https://cdn.jsdelivr.net/npm/@quidnug/web-components@2/src/index.js"></script>

<quidnug-review
    product="shopify-{{ product.id }}"
    topic="{{ block.settings.topic | default: 'reviews.public.other' }}"
    show-write
></quidnug-review>

<script>
    // Hook up the client + observer
    const QC = window.QuidnugClient;
    import("https://cdn.jsdelivr.net/npm/@quidnug/web-components@2/src/context.js")
        .then(({ setClient, setObserverQuid }) => {
            const c = new QC({ defaultNode: "{{ app.metafields.quidnug.node_url }}" });
            setClient(c);
            if (window.quidnug) {
                window.quidnug.listQuids().then((qs) => {
                    if (qs?.length > 0) setObserverQuid(qs[0]);
                }).catch(() => {});
            }
        });
</script>

{% schema %}
{
  "name": "Quidnug Trust-Weighted Reviews",
  "target": "section",
  "settings": [
    {
      "type": "text",
      "id": "topic",
      "label": "Topic domain",
      "default": "reviews.public.other",
      "info": "One of the reviews.public.* domains. Set per-collection for better topical trust."
    }
  ]
}
{% endschema %}
```

## Admin UI

A minimal Polaris-based app for the merchant's admin:

```
extensions/quidnug-admin/
├── package.json
├── shopify.app.toml
└── web/
    ├── index.js           # Shopify app backend (Node/Remix)
    ├── app/
    │   └── routes/
    │       └── _index.jsx  # Settings form — node URL, topic mappings
    └── ...
```

## Checkout Review Prompt

A post-purchase extension that nudges buyers to review:

```javascript
import {
  extension,
  BlockStack,
  Text,
  Button,
  useApi,
} from "@shopify/ui-extensions/checkout";

extension("purchase.thank-you.block.render", (root, api) => {
  const { order } = api;
  root.appendChild(root.createComponent(BlockStack, null, [
    root.createComponent(Text, { size: "medium" },
      "Help other shoppers — leave a trust-weighted review."),
    root.createComponent(Button, {
      onPress: () => {
        // Open the merchant's Quidnug review page for each item
        const lineItems = order.lineItems;
        // ...
      },
    }, "Write a review"),
  ]));
});
```

## Verified purchase attestation

For each purchase, the merchant's quid emits a `PURCHASE` event
per QRP-0001 §5.5:

```json
{
  "type": "EVENT",
  "subjectId": "<buyer-quid>",
  "subjectType": "QUID",
  "eventType": "PURCHASE",
  "payload": {
    "qrpVersion": 1,
    "productAssetQuid": "shopify-<product-id>",
    "purchasedAt": 1700000000,
    "retailerAttestation": {
      "retailerName": "Example Shop",
      "orderIdHash": "sha256:..."
    }
  }
}
```

This lets Quidnug clients display a "verified purchase" badge
whenever the reviewer's quid has a PURCHASE event from a
retailer the observer trusts.

## Merchant signing

The Shopify app holds a retailer-level quid keypair (per store)
in the Shopify app database. The app signs PURCHASE events
automatically on every order fulfillment.

**Security:** because the retailer quid has write access to the
`reviews.public.*` network, its key compromise is high-severity.
Production deployments should:

1. Store the private key in a KMS (AWS KMS / GCP KMS / Shopify
   Private App tokens).
2. Use QDP-0002 guardian recovery (the store owner + Shopify
   support + a recovery service serve as guardians).

## Distribution

To list in the Shopify App Store, the app needs:

- GDPR webhooks (data request + data erasure).
- Billing API integration.
- Compliance review.

These are standard Shopify app requirements. The Quidnug-
specific bits are what's above.

## Roadmap

1. **Phase 1 (this scaffold)**: full theme extension working
   against the public node.
2. **Phase 2**: admin UI for topic mapping + analytics.
3. **Phase 3**: checkout extension with auto-PURCHASE attestation.
4. **Phase 4**: Shopify App Store listing.

## License

Apache-2.0.
