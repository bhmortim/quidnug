package schemaorg

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/quidnug/quidnug/pkg/client"
)

func TestConvert(t *testing.T) {
	ev := client.Event{
		SubjectID:   "prod-123",
		SubjectType: "TITLE",
		EventType:   "REVIEW",
		Payload: map[string]any{
			"rating":       4.5,
			"maxRating":    5.0,
			"title":        "Solid laptop",
			"bodyMarkdown": "Works great but keyboard is mushy.",
			"locale":       "en-US",
		},
		Timestamp: 1700000000,
		Sequence:  1,
		Creator:   "abc1234abc1234ab",
	}
	r := Convert(ev, "Example Laptop", "https://example.com/p/123", "Alice")

	if r.Context != "https://schema.org" {
		t.Errorf("context: %q", r.Context)
	}
	if r.Type != "Review" {
		t.Errorf("type: %q", r.Type)
	}
	if r.ReviewRating.RatingValue != 4.5 {
		t.Errorf("rating: %v", r.ReviewRating.RatingValue)
	}
	if r.Author.Identifier != "did:quidnug:abc1234abc1234ab" {
		t.Errorf("author.identifier: %q", r.Author.Identifier)
	}

	j, err := ConvertJSON(r)
	if err != nil {
		t.Fatalf("ConvertJSON: %v", err)
	}
	if !strings.Contains(string(j), `"@type": "Review"`) {
		t.Errorf("JSON missing type: %s", j)
	}
}

func TestAggregate(t *testing.T) {
	events := []client.Event{
		fakeReview(4.5, 5.0),
		fakeReview(3.0, 5.0),
		fakeReview(8.0, 10.0), // different scale → 4.0 on 5-scale
		{
			SubjectID:   "prod",
			EventType:   "HELPFUL_VOTE", // should be ignored
			Payload:     map[string]any{},
			Timestamp:   1,
		},
	}

	agg, err := Aggregate(events, "Example Laptop", "https://ex.com/p")
	if err != nil {
		t.Fatalf("Aggregate: %v", err)
	}
	if agg.ReviewCount != 3 {
		t.Errorf("review count: %d (expected 3, HELPFUL_VOTE should be excluded)", agg.ReviewCount)
	}
	// (4.5 + 3.0 + 4.0) / 3 = 3.833
	if agg.RatingValue < 3.7 || agg.RatingValue > 3.9 {
		t.Errorf("rating: %v (expected ~3.83)", agg.RatingValue)
	}

	// JSON conformance
	j, _ := AggregateJSON(agg)
	var parsed map[string]any
	if err := json.Unmarshal(j, &parsed); err != nil {
		t.Fatalf("json roundtrip: %v", err)
	}
	if parsed["@type"] != "AggregateRating" {
		t.Errorf("@type: %v", parsed["@type"])
	}
}

func TestAggregateWithNoReviews(t *testing.T) {
	_, err := Aggregate(nil, "x", "y")
	if err == nil {
		t.Fatal("expected error on empty set")
	}
}

func fakeReview(rating, maxRating float64) client.Event {
	return client.Event{
		SubjectID: "prod",
		EventType: "REVIEW",
		Payload: map[string]any{
			"rating":    rating,
			"maxRating": maxRating,
		},
		Timestamp: 1700000000,
	}
}
