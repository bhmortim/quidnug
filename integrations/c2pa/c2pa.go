package c2pa

import (
	"context"
	"errors"

	"github.com/quidnug/quidnug/pkg/client"
)

// Manifest is the minimal subset of a C2PA manifest this package
// needs. Populate from the verified output of c2pa-rs or the
// c2pa-js library.
type Manifest struct {
	AssetID        string         `json:"assetId"`
	Format         string         `json:"format"`         // "image/jpeg", "video/mp4", …
	Title          string         `json:"title"`
	ClaimGenerator string         `json:"claimGenerator"` // "Adobe Photoshop 25.0", …
	Signer         string         `json:"signer"`         // signer name / email / SAN
	SignedAt       int64          `json:"signedAt"`       // Unix seconds
	Ingredients    []Ingredient   `json:"ingredients,omitempty"`
	Assertions     []Assertion    `json:"assertions,omitempty"`
	Extra          map[string]any `json:"extra,omitempty"`
}

// Ingredient is an upstream asset this manifest references.
type Ingredient struct {
	URI       string `json:"uri"`
	Hash      string `json:"hash"`
	Format    string `json:"format"`
	Relationship string `json:"relationship"` // "parentOf", "componentOf", …
}

// Assertion is one C2PA assertion label+value pair. Labels are
// registered URIs like `c2pa.actions`, `c2pa.training-mining`, etc.
type Assertion struct {
	Label string         `json:"label"`
	Data  map[string]any `json:"data"`
}

// Recorder posts C2PA_MANIFEST events to a Quidnug node.
type Recorder struct {
	client    *client.Client
	domain    string
	eventType string
}

// Options configures the Recorder.
type Options struct {
	Client    *client.Client
	Domain    string
	EventType string
}

// New returns a configured Recorder.
func New(opts Options) (*Recorder, error) {
	if opts.Client == nil {
		return nil, errors.New("Client is required")
	}
	if opts.Domain == "" {
		opts.Domain = "default"
	}
	if opts.EventType == "" {
		opts.EventType = "C2PA_MANIFEST"
	}
	return &Recorder{client: opts.Client, domain: opts.Domain, eventType: opts.EventType}, nil
}

// RecordManifest submits a C2PA_MANIFEST event for the asset Title.
func (r *Recorder) RecordManifest(ctx context.Context, signer *client.Quid, m Manifest) (map[string]any, error) {
	if m.AssetID == "" {
		return nil, errors.New("AssetID is required")
	}
	payload := map[string]any{
		"schema":         "c2pa-manifest/v1.3",
		"format":         m.Format,
		"title":          m.Title,
		"claimGenerator": m.ClaimGenerator,
		"signer":         m.Signer,
		"signedAt":       m.SignedAt,
	}
	if len(m.Ingredients) > 0 {
		payload["ingredients"] = m.Ingredients
	}
	if len(m.Assertions) > 0 {
		payload["assertions"] = m.Assertions
	}
	if len(m.Extra) > 0 {
		payload["extra"] = m.Extra
	}
	return r.client.EmitEvent(ctx, signer, client.EventParams{
		SubjectID:   m.AssetID,
		SubjectType: "TITLE",
		EventType:   r.eventType,
		Domain:      r.domain,
		Payload:     payload,
	})
}
