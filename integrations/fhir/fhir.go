package fhir

import (
	"context"
	"errors"
	"fmt"

	"github.com/quidnug/quidnug/pkg/client"
)

// Resource is the minimal envelope this package cares about. It
// matches the top-level shape of a FHIR R4/R5 resource after JSON
// parsing.
type Resource struct {
	ResourceType string         `json:"resourceType"` // "Observation", "Encounter", …
	ID           string         `json:"id"`
	Subject      string         `json:"subject"`      // Patient reference (e.g. "Patient/123")
	Issuer       string         `json:"issuer"`       // optional issuing provider
	Status       string         `json:"status"`
	EffectiveAt  int64          `json:"effectiveAt"`  // Unix seconds
	Body         map[string]any `json:"body"`         // full FHIR JSON
}

// Recorder records FHIR resources onto Quidnug event streams.
type Recorder struct {
	client    *client.Client
	domain    string
	eventType string
}

// Options configures the Recorder.
type Options struct {
	Client    *client.Client
	Domain    string
	EventType string // default "FHIR_RESOURCE"
}

// New returns a configured Recorder.
func New(opts Options) (*Recorder, error) {
	if opts.Client == nil {
		return nil, errors.New("Client is required")
	}
	if opts.Domain == "" {
		opts.Domain = "healthcare.default"
	}
	if opts.EventType == "" {
		opts.EventType = "FHIR_RESOURCE"
	}
	return &Recorder{client: opts.Client, domain: opts.Domain, eventType: opts.EventType}, nil
}

// RecordResource submits a FHIR_RESOURCE event on the subject's Title.
// The Title's asset ID should be the Quidnug quid representing the
// FHIR subject (Patient, Practitioner, Organization).
func (r *Recorder) RecordResource(
	ctx context.Context,
	issuer *client.Quid,
	assetTitleID string,
	res Resource,
) (map[string]any, error) {
	if assetTitleID == "" {
		return nil, errors.New("assetTitleID is required")
	}
	if res.ResourceType == "" || res.ID == "" {
		return nil, errors.New("Resource.ResourceType and Resource.ID are required")
	}
	payload := map[string]any{
		"schema":       "fhir-resource/v1",
		"resourceType": res.ResourceType,
		"resourceId":   res.ID,
		"subject":      res.Subject,
		"issuer":       res.Issuer,
		"status":       res.Status,
		"effectiveAt":  res.EffectiveAt,
		"body":         res.Body,
	}
	eventType := fmt.Sprintf("%s.%s", r.eventType, res.ResourceType)
	return r.client.EmitEvent(ctx, issuer, client.EventParams{
		SubjectID:   assetTitleID,
		SubjectType: "TITLE",
		EventType:   eventType,
		Domain:      r.domain,
		Payload:     payload,
	})
}
