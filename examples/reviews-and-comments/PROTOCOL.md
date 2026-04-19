# QRP-0001 — The Quidnug Reviews Protocol

> **Status**: Draft v1
> **Date**: 2026-04-19
> **Editor**: reviews-protocol@quidnug.dev
> **Depends on**: QDP-0001 (nonces), QDP-0002 (guardians),
>                QDP-0003 (gossip), QDP-0010 (Merkle proofs)

## 1. Overview

QRP-0001 defines the wire-level protocol for reviews, comments,
helpfulness voting, flagging, and verified-purchase attestation
on top of the base Quidnug protocol. Every review system or
website that speaks QRP-0001 interoperates with every other,
using the same global reviewer identities and the same trust
graph.

The protocol is intentionally thin — QRP-0001 does **not**
define presentation, UI, or the exact trust-weighting formula.
That's where implementations differentiate. QRP-0001 defines the
**data** so every implementation sees the same facts on the wire.

## 2. Actors

| Actor | Role |
| --- | --- |
| **Reviewer** | A Quidnug quid that publishes reviews and votes on others |
| **Product** | A Quidnug Title representing what's reviewed (a laptop, restaurant, book, service) |
| **Site** | A platform that hosts reviews (may optionally issue its own trust edges) |
| **Observer** | Anyone reading reviews; computes per-observer weighted scores |
| **Moderator** | A reviewer quid that also publishes FLAG events; observers choose to respect their flags |
| **Retailer** | A quid that attests "this reviewer actually purchased this product" via PURCHASE events |

## 3. Domain scheme

Reviews live in the public `reviews.public` domain tree:

```
reviews.public                              (root)
├── reviews.public.technology
│   ├── reviews.public.technology.laptops
│   ├── reviews.public.technology.cameras
│   ├── reviews.public.technology.software
│   └── reviews.public.technology.phones
├── reviews.public.restaurants
│   ├── reviews.public.restaurants.us.ny.nyc
│   ├── reviews.public.restaurants.jp.tokyo
│   └── ... (ISO country / city hierarchy)
├── reviews.public.books
├── reviews.public.movies
├── reviews.public.services
├── reviews.public.places              (hotels, AirBnBs, attractions)
└── reviews.public.other
```

**Topical trust is scoped to these domains.** A quid that Alice
trusts at 0.9 in `reviews.public.restaurants.us.ny.nyc` does
**not** automatically get 0.9 in `reviews.public.technology`.

**Inheritance rule:** trust in a child domain inherits from
ancestors at a decay factor of 0.8 per hop. If you trust Bob at
0.9 in `reviews.public.technology`, he gets 0.9 × 0.8 = 0.72
in `reviews.public.technology.laptops` by default. Observers
can override with an explicit child-domain edge.

Sites can optionally run private sibling trees
(`reviews.privatestore.mycompany.com`) for internal reviews
while still consuming the global tree.

## 4. Product titles

Every reviewable entity is registered as a Quidnug Title.

Title registration for a product looks like:

```json
{
  "type": "TITLE",
  "trustDomain": "reviews.public.technology.laptops",
  "assetQuid": "<deterministic-product-id>",
  "issuerQuid": "<registrar-quid>",
  "titleType": "REVIEWABLE_PRODUCT",
  "ownershipMap": [
    { "ownerId": "<manufacturer-quid-or-registrar>", "percentage": 100 }
  ],
  "attributes": {
    "identifiers": {
      "asin": "B0C1234ABC",
      "ean": "0123456789012",
      "upc": "012345678905",
      "isbn": "9780123456789",
      "schemaOrgUrl": "https://example.com/p/123"
    },
    "canonicalName": "Example Brand XPS 15 9530",
    "locale": "en-US"
  }
}
```

**The `assetQuid` MUST be deterministic** — computed from the
canonical identifiers so the same product gets the same Title
regardless of who registers it first. Recommended computation:

```
assetQuid = sha256("REVIEW-TITLE:" || canonical_identifiers_json)[:16]
          (16 hex chars, same shape as a quid ID)
```

Where `canonical_identifiers_json` is the canonical JSON of the
identifiers block, alphabetized.

**Duplicate registration** is harmless — subsequent TITLE
transactions for the same `assetQuid` are idempotent-rejected
with `ALREADY_EXISTS`, and the first registration wins.

## 5. Event types

All events are emitted on the **product Title's** event stream
(QDP-0001 event semantics) unless noted otherwise.

### 5.1 REVIEW

Published by: any reviewer.
Stream: the product's Title stream.
Domain: the product's most-specific topic domain.

```json
{
  "type": "EVENT",
  "subjectId": "<product-assetQuid>",
  "subjectType": "TITLE",
  "eventType": "REVIEW",
  "payload": {
    "qrpVersion": 1,
    "rating": 4.5,
    "maxRating": 5.0,
    "title": "Solid laptop with one caveat",
    "bodyMarkdown": "...",
    "bodyHtml": null,
    "locale": "en-US",
    "mediaAttachments": [
      { "cid": "bafy...", "contentType": "image/jpeg", "caption": "..." }
    ],
    "contextTags": ["business-use", "travel"],
    "purchaseAttestationCid": null,
    "supersedes": null
  }
}
```

Notes:
- `rating` is a float in [0, `maxRating`]. 5-star scales use
  `maxRating: 5.0`; pass/fail reviews use `maxRating: 1.0`.
- `bodyMarkdown` is preferred; `bodyHtml` is allowed but will
  be sanitized by consumers.
- `supersedes` MAY reference a prior REVIEW's tx id (same
  reviewer only). Observers SHOULD display the newest and fade
  the superseded.
- `contextTags` is free-form — lets reviewers declare "I'm
  reviewing for X use case." Observers can filter.

### 5.2 HELPFUL_VOTE / UNHELPFUL_VOTE

Published by: any quid other than the review's author.
Stream: the **reviewer's** stream (NOT the product's).
Domain: same as the referenced review.

```json
{
  "type": "EVENT",
  "subjectId": "<reviewer-quid>",
  "subjectType": "QUID",
  "eventType": "HELPFUL_VOTE",
  "payload": {
    "qrpVersion": 1,
    "reviewTxId": "<tx-id-of-review>",
    "productAssetQuid": "<product-id>",
    "reasonCode": null,
    "reasonText": null
  }
}
```

The reverse `UNHELPFUL_VOTE` has the identical shape but
different `eventType`.

Why on the **reviewer's** stream and not the product's? Because
helpfulness votes accrue to the **reviewer's reputation**, not
the product's. Observers fetch a reviewer's helpfulness history
by streaming their quid, not by scanning every product they've
ever reviewed.

Rate limits: an observer SHOULD see at most **one helpful/
unhelpful vote per review per voter**. Duplicates SHOULD be
treated as idempotent-replace.

### 5.3 REPLY

Published by: any reviewer.
Stream: the product's Title stream.
Threading: replies reference the parent via `inReplyTo`.

```json
{
  "type": "EVENT",
  "subjectId": "<product-assetQuid>",
  "subjectType": "TITLE",
  "eventType": "REPLY",
  "payload": {
    "qrpVersion": 1,
    "inReplyTo": "<parent-tx-id>",
    "bodyMarkdown": "I disagree — I've had the opposite experience with the keyboard.",
    "locale": "en-US"
  }
}
```

Replies to reviews, replies to replies, and author responses
are all the same `REPLY` event type. Threading is fully
determined by `inReplyTo`.

### 5.4 FLAG

Published by: any quid.
Stream: the product's Title stream.

```json
{
  "type": "EVENT",
  "subjectId": "<product-assetQuid>",
  "subjectType": "TITLE",
  "eventType": "FLAG",
  "payload": {
    "qrpVersion": 1,
    "targetTxId": "<review-or-reply-tx-id>",
    "reasonCode": "SPAM|FAKE|INAPPROPRIATE|OFF_TOPIC|OTHER",
    "reasonText": "Looks like it was copy-pasted from another listing.",
    "severity": "LOW|MEDIUM|HIGH"
  }
}
```

**Critical:** FLAG events are advisory. An observer decides
whether to respect a flag based on **their trust** in the
flagger. A flag from an untrusted spammer is weighted zero; a
flag from a moderator the observer has granted `moderate`
trust to is weighted fully. No global "deletion" — the
underlying review remains on the stream; just hidden from
observers whose trusted moderators flagged it.

### 5.5 PURCHASE

Published by: a retailer quid.
Stream: the **reviewer's** stream.

```json
{
  "type": "EVENT",
  "subjectId": "<reviewer-quid>",
  "subjectType": "QUID",
  "eventType": "PURCHASE",
  "payload": {
    "qrpVersion": 1,
    "productAssetQuid": "<product-id>",
    "purchasedAt": 1700000000,
    "retailerAttestation": {
      "retailerName": "Example Retailer",
      "orderIdHash": "sha256:...",
      "amountUsd": null
    }
  }
}
```

A PURCHASE event lets a retailer attest that a reviewer did in
fact purchase the product. The review UI can display a
"verified purchase" badge iff there's a PURCHASE event from a
retailer the observer trusts, referencing the same product as
the review.

Privacy: retailers SHOULD hash order IDs, not publish them
plaintext. Amounts are optional and SHOULD be omitted unless
the reviewer opts in.

### 5.6 TRUST_TOPIC (shorthand)

This is a normal Quidnug TRUST transaction, but QRP-0001 profiles
the usage:

- `trustDomain` MUST be a `reviews.public.*` domain.
- `description` SHOULD include a short note explaining why:
  "highly respected for DSLR camera reviews."
- `validUntil` is useful: trust may be time-limited (e.g., "I
  trust this reviewer for 1 year then it needs renewal").

No new wire format needed — reuse the base protocol.

## 6. Supersession & deletion

- **Edit:** emit a new REVIEW with `supersedes` pointing at the
  old. Both remain on-stream; clients display the newest.
- **Delete:** emit a REVIEW with `supersedes` set, empty
  `rating` and empty `bodyMarkdown`. Clients interpret this
  as "retracted by author" and hide.
- **True deletion:** not possible in an append-only log.
  GDPR-right-to-erasure is handled at the presentation layer
  by suppressing display of any content from a quid that has
  published a valid ERASE_REQUEST event (spec TBD in QRP-0002).

## 7. Governance of the public tree

Adding a new top-level topic under `reviews.public.*` requires:

1. A proposal post on the QRP working group list (email or forum).
2. 30-day public comment period.
3. Rough consensus among the top 10 node operators by traffic.
4. A signed `TOPIC_ADD` transaction posted to a governance
   domain (`reviews.public.governance`) by a majority.

Node operators are **volunteers**. A public node operator
commits to:

- 99% uptime (measured by periodic ping).
- Mirror the full `reviews.public.*` tree (storage budget ~200 GB
  projected at launch, growing ~20 GB/year at expected volumes).
- Publish signed domain fingerprints every 5 minutes so peers
  detect divergence.

## 8. Identity bootstrap

New reviewers have no history. Three bootstrap mechanisms:

### 8.1 OIDC bridge (easiest)

Sign in with Google / Apple / GitHub. The Quidnug OIDC bridge
(`cmd/quidnug-oidc/`) provisions a quid bound to your OIDC
subject. Your reputation starts at zero, but your identity is
durable (future logins resolve to the same quid).

### 8.2 Manual keypair (for power users)

Generate a quid in the browser extension or CLI. Import across
devices. Zero-dependency on an IdP.

### 8.3 Cross-site import

For existing reviewers on Amazon / Yelp / Google Maps: a
signed import flow. The old platform signs a bridge attestation
("this Amazon account `username` is now bound to Quidnug quid
X"). A Quidnug quid that imports an Amazon account with
10,000 helpful votes starts with a bootstrap helpfulness
reputation. Reviewers opt in; platforms can choose to
participate (nothing stops them from ignoring, but
participating helps their users retain trust).

## 9. Anti-spam & sybil resistance

Because reviews are **per-observer-weighted**, sybils are
mostly self-defeating:

- A fresh quid has zero trust from anyone. Its reviews carry
  zero weight from every observer until someone extends trust.
- HELPFUL_VOTE events from fresh quids are similarly weighted
  zero.
- Running 10,000 bot quids and having them all upvote each
  other produces a clique in the graph that has zero trust
  from legitimate observers — the clique cannot bootstrap
  itself without a legitimate trust edge into it.

Observers can further filter by:
- "Show only reviews from quids with ≥ 10 total helpful votes
  from observers I trust"
- "Show only reviews from quids with a PURCHASE event from a
  trusted retailer"
- "Hide reviews flagged by any moderator I trust at ≥ 0.7"

The protocol doesn't enforce these — it just provides the data.
Presentation layers decide the UX.

## 10. Storage & growth projections

Rough numbers at scale:

- 1 M active reviewers
- 100 reviews/reviewer/year average → 100 M reviews/year
- ~1 KB per review after canonicalization
- ~5 helpfulness votes per review → 500 M vote events/year
- ~100 bytes per vote event

Total wire volume: ~150 GB/year of review data, ~50 GB/year of
vote data. Storage with compression: ~100 GB/year per public
node. This is comfortably within a single volunteer-operated
node's capacity.

## 11. Compatibility with Schema.org

Every Quidnug review SHOULD be exportable as a Schema.org
`Review` JSON-LD block for SEO and interop with legacy
consumers. The mapping:

| Schema.org field | QRP-0001 source |
| --- | --- |
| `@type` | Fixed: `"Review"` |
| `itemReviewed` | The product Title's attributes |
| `reviewRating.ratingValue` | `rating` |
| `reviewRating.bestRating` | `maxRating` |
| `author.name` | Reviewer's identity record `name` |
| `author.identifier` | `did:quidnug:<reviewer-quid>` |
| `reviewBody` | `bodyMarkdown` (rendered) |
| `datePublished` | Event timestamp |

A reference converter ships at
[`integrations/schema-org/`](../../integrations/schema-org/).

## 12. Reference implementations

- **Algorithm**: [`algorithm.py`](algorithm.py) (Python),
  [`algorithm.go`](algorithm.go) (Go).
- **Simulation**: [`simulation.py`](simulation.py).
- **Web component**: [`clients/web-components/`](../../clients/web-components/).
- **React lib**: [`clients/react-reviews/`](../../clients/react-reviews/).
- **WordPress plugin**: [`clients/wordpress-plugin/`](../../clients/wordpress-plugin/).
- **Shopify app**: [`clients/shopify-app/`](../../clients/shopify-app/).

## 13. Versioning

`qrpVersion` in every payload allows future revisions. A
client receiving a `qrpVersion` higher than it recognizes
SHOULD render what it understands and fall back gracefully
on unknown fields.

## 14. Security considerations

- Reviewer quids with compromised keys: use QDP-0002 guardian
  recovery to rotate. Past reviews remain signed under the
  compromised key but new reviews get the new key's signature.
- Retailer quid compromise: a compromised retailer can forge
  PURCHASE events. Mitigation: observers track retailer
  credibility via relational trust (don't accept a PURCHASE
  from an unknown retailer).
- Mass-brigade attacks: handled by the per-observer weighting
  — a mass of uncredentialed accounts doesn't shift a
  credentialed observer's view.

## License

Apache-2.0.
