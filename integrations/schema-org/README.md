# Quidnug × Schema.org bridge

Two-way mapping between Quidnug `REVIEW` events (QRP-0001) and
Schema.org `Review` / `AggregateRating` JSON-LD.

## Why

Search engines (Google, Bing, DuckDuckGo) use Schema.org
structured data to show review rich results in SERPs. Quidnug
is the source of truth for reviews, but search crawlers don't
speak Quidnug — they speak Schema.org.

This bridge lets a site render its normal interactive Quidnug
reviews to humans, while ALSO emitting standards-compliant
Schema.org JSON-LD for search crawlers.

## Usage (Go)

```go
import (
    "context"
    "fmt"
    "github.com/quidnug/quidnug/integrations/schema-org"
    "github.com/quidnug/quidnug/pkg/client"
)

c, _ := client.New("https://public.quidnug.dev")
events, _, _ := c.GetStreamEvents(ctx, "prod-id", "reviews.public.technology", 100, 0)

// Aggregate (one blob per product, for the product page head)
agg, _ := schemaorg.Aggregate(events, "Example Laptop", "https://example.com/p/123")
aggJSON, _ := schemaorg.AggregateJSON(agg)
fmt.Printf(`<script type="application/ld+json">%s</script>`, aggJSON)

// Per-review (list on review index pages)
for _, ev := range events {
    r := schemaorg.Convert(ev, "Example Laptop", "https://example.com/p/123", "Alice")
    blob, _ := schemaorg.ConvertJSON(r)
    fmt.Printf(`<script type="application/ld+json">%s</script>`, blob)
}
```

## Mapping

| Schema.org field | Quidnug source |
| --- | --- |
| `@type`               | Fixed: `"Review"` or `"AggregateRating"` |
| `itemReviewed.@type`  | Fixed: `"Product"` (can be specialized) |
| `itemReviewed.name`   | Product title attribute |
| `itemReviewed.url`    | Product landing page URL |
| `reviewRating.ratingValue`  | `rating` from event payload |
| `reviewRating.bestRating`   | `maxRating` from event payload |
| `reviewRating.worstRating`  | 0 |
| `author.@type`              | Fixed: `"Person"` |
| `author.name`               | Reviewer's identity record name |
| `author.identifier`         | `did:quidnug:<reviewer-quid>` |
| `reviewBody`          | `bodyMarkdown` (rendered to plain text) |
| `datePublished`       | Event timestamp (RFC 3339 UTC) |
| `inLanguage`          | `locale` from event payload |
| `aggregateRating.ratingValue` | Simple average, normalized to 5.0 |
| `aggregateRating.reviewCount` | Count of REVIEW events |

## Testing

```bash
cd integrations/schema-org
go test -v
```

3 tests covering conversion + aggregate + error cases.

## SEO best practices

- Emit **AggregateRating** on product pages.
- Emit **Review** on review-detail pages (one per review).
- Keep `reviewCount` honest — Google rejects fake inflation.
- Update the cached aggregate at least every 5–15 minutes for
  active products; never cache longer than 24 hours.
- The aggregate is the "anonymous observer" view — search
  crawlers don't have a Quidnug identity, so they see the
  un-weighted simple average. Per-observer weighting only
  runs in actual users' browsers.

## License

Apache-2.0.
