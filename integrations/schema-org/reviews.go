// Package schemaorg provides a converter between Quidnug REVIEW
// events (QRP-0001) and Schema.org Review / AggregateRating
// JSON-LD.
//
// Use this when a site needs to emit SEO-friendly structured data
// for Google / Bing / DuckDuckGo rich results, while using Quidnug
// as the source of truth for reviews.
//
// Direction: Quidnug → Schema.org (for emitting JSON-LD).
// For the reverse (ingesting old Schema.org reviews into Quidnug),
// see the `Import` function.
package schemaorg

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/quidnug/quidnug/pkg/client"
)

// Review is the shape of a single Schema.org Review.
type Review struct {
	Context       string           `json:"@context"`
	Type          string           `json:"@type"`
	ItemReviewed  *Product         `json:"itemReviewed,omitempty"`
	ReviewRating  *Rating          `json:"reviewRating,omitempty"`
	Author        *Person          `json:"author,omitempty"`
	ReviewBody    string           `json:"reviewBody,omitempty"`
	DatePublished string           `json:"datePublished,omitempty"`
	Name          string           `json:"name,omitempty"`
	InLanguage    string           `json:"inLanguage,omitempty"`
}

// AggregateRating is the summary across all reviews.
type AggregateRating struct {
	Context      string   `json:"@context"`
	Type         string   `json:"@type"`
	ItemReviewed *Product `json:"itemReviewed,omitempty"`
	RatingValue  float64  `json:"ratingValue"`
	BestRating   float64  `json:"bestRating,omitempty"`
	WorstRating  float64  `json:"worstRating,omitempty"`
	ReviewCount  int      `json:"reviewCount"`
}

type Product struct {
	Type string `json:"@type"`
	Name string `json:"name,omitempty"`
	URL  string `json:"url,omitempty"`
}

type Rating struct {
	Type        string  `json:"@type"`
	RatingValue float64 `json:"ratingValue"`
	BestRating  float64 `json:"bestRating,omitempty"`
	WorstRating float64 `json:"worstRating,omitempty"`
}

type Person struct {
	Type       string `json:"@type"`
	Name       string `json:"name,omitempty"`
	Identifier string `json:"identifier,omitempty"`
}

// Convert transforms a Quidnug REVIEW event + product name/URL into a
// Schema.org Review JSON-LD blob.
func Convert(ev client.Event, productName, productURL, reviewerName string) Review {
	payload := ev.Payload
	rating, _ := payload["rating"].(float64)
	maxRating, _ := payload["maxRating"].(float64)
	if maxRating == 0 {
		maxRating = 5.0
	}
	body, _ := payload["bodyMarkdown"].(string)
	title, _ := payload["title"].(string)
	locale, _ := payload["locale"].(string)
	if locale == "" {
		locale = "en"
	}

	return Review{
		Context: "https://schema.org",
		Type:    "Review",
		Name:    title,
		ItemReviewed: &Product{
			Type: "Product",
			Name: productName,
			URL:  productURL,
		},
		ReviewRating: &Rating{
			Type:        "Rating",
			RatingValue: rating,
			BestRating:  maxRating,
			WorstRating: 0,
		},
		Author: &Person{
			Type:       "Person",
			Name:       reviewerName,
			Identifier: "did:quidnug:" + ev.Creator,
		},
		ReviewBody:    body,
		DatePublished: time.Unix(ev.Timestamp, 0).UTC().Format(time.RFC3339),
		InLanguage:    locale,
	}
}

// Aggregate computes an AggregateRating JSON-LD from a set of REVIEW
// events. This is the "anonymous observer" view — no per-observer
// weighting, which is appropriate for search-engine crawlers.
func Aggregate(
	events []client.Event,
	productName, productURL string,
) (AggregateRating, error) {
	count := 0
	sumNormalized := 0.0
	for _, ev := range events {
		if ev.EventType != "REVIEW" {
			continue
		}
		rating, ok := ev.Payload["rating"].(float64)
		if !ok {
			continue
		}
		maxRating, _ := ev.Payload["maxRating"].(float64)
		if maxRating == 0 {
			maxRating = 5.0
		}
		sumNormalized += rating / maxRating
		count++
	}
	if count == 0 {
		return AggregateRating{}, fmt.Errorf("no reviews to aggregate")
	}
	return AggregateRating{
		Context: "https://schema.org",
		Type:    "AggregateRating",
		ItemReviewed: &Product{
			Type: "Product",
			Name: productName,
			URL:  productURL,
		},
		RatingValue: (sumNormalized / float64(count)) * 5.0,
		BestRating:  5.0,
		WorstRating: 0,
		ReviewCount: count,
	}, nil
}

// ConvertJSON is a convenience: returns the JSON-LD string ready to
// drop inside a <script type="application/ld+json">...</script> tag.
func ConvertJSON(r Review) ([]byte, error) {
	return json.MarshalIndent(r, "", "  ")
}

// AggregateJSON is the same for AggregateRating.
func AggregateJSON(a AggregateRating) ([]byte, error) {
	return json.MarshalIndent(a, "", "  ")
}
