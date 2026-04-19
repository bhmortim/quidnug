package sigstore

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/quidnug/quidnug/pkg/client"
)

// Bundle is the minimal subset of a sigstore bundle this package
// needs. Structurally compatible with cosign v0.2 bundles: populate
// these fields from the output of `cosign sign ... --output=bundle`.
type Bundle struct {
	// ArtifactID is the Quidnug Title's asset id that this signature
	// is being recorded against. Typically a SHA-256 of the artifact
	// or a Quidnug Title quid ID.
	ArtifactID string `json:"artifactId"`
	// ArtifactDigest is the raw digest sigstore signed over.
	// Convention: "sha256:<hex>".
	ArtifactDigest string `json:"artifactDigest"`
	// SignatureB64 is the base64-encoded signature from the bundle.
	SignatureB64 string `json:"signature"`
	// CertificatePEM is the PEM-encoded Fulcio-issued signing cert.
	CertificatePEM string `json:"certificate"`
	// BundleURI optionally points at a Rekor transparency-log entry.
	BundleURI string `json:"bundleUri,omitempty"`
	// Signer is the extracted Fulcio SAN (email / SPIFFE / OIDC sub).
	Signer string `json:"signer"`
	// SignedAt is when sigstore issued the signature.
	SignedAt time.Time `json:"signedAt"`
	// Extra holds any cosign-specific envelope fields verbatim.
	Extra map[string]any `json:"extra,omitempty"`
}

// Recorder posts SIGSTORE_SIGNATURE events to a Quidnug node.
type Recorder struct {
	client   *client.Client
	domain   string
	eventType string
}

// Options configures the Recorder.
type Options struct {
	// Client is the Quidnug HTTP client to use. Required.
	Client *client.Client
	// Domain is the trust domain to record events in. Defaults to
	// "default".
	Domain string
	// EventType overrides the event type string. Defaults to
	// "SIGSTORE_SIGNATURE".
	EventType string
}

// New returns a configured Recorder.
func New(opts Options) (*Recorder, error) {
	if opts.Client == nil {
		return nil, errors.New("Client is required")
	}
	r := &Recorder{
		client:    opts.Client,
		domain:    opts.Domain,
		eventType: opts.EventType,
	}
	if r.domain == "" {
		r.domain = "default"
	}
	if r.eventType == "" {
		r.eventType = "SIGSTORE_SIGNATURE"
	}
	return r, nil
}

// RecordBundle signs the bundle payload with `signer` (owner of the
// ArtifactID title) and submits it as an EVENT.
//
// The caller is responsible for sigstore-side verification (use
// github.com/sigstore/sigstore-go or cosign's library). This function
// simply records the signed bundle in the Quidnug stream.
func (r *Recorder) RecordBundle(ctx context.Context, signer *client.Quid, b Bundle) (map[string]any, error) {
	if b.ArtifactID == "" {
		return nil, errors.New("ArtifactID is required")
	}
	if b.SignatureB64 == "" {
		return nil, errors.New("SignatureB64 is required")
	}
	payload := map[string]any{
		"schema":         "sigstore-bundle/v0.2",
		"artifactDigest": b.ArtifactDigest,
		"signature":      b.SignatureB64,
		"certificate":    b.CertificatePEM,
		"signer":         b.Signer,
		"signedAt":       b.SignedAt.Unix(),
	}
	if b.BundleURI != "" {
		payload["bundleUri"] = b.BundleURI
	}
	if len(b.Extra) > 0 {
		payload["extra"] = b.Extra
	}
	return r.client.EmitEvent(ctx, signer, client.EventParams{
		SubjectID:   b.ArtifactID,
		SubjectType: "TITLE",
		EventType:   r.eventType,
		Domain:      r.domain,
		Payload:     payload,
	})
}

// BundleFromCosignJSON parses the JSON produced by `cosign sign ...
// --output=bundle` into a Bundle. The structure matches the sigstore
// bundle spec v0.2.
func BundleFromCosignJSON(artifactID string, raw []byte) (Bundle, error) {
	var env cosignEnvelope
	if err := json.Unmarshal(raw, &env); err != nil {
		return Bundle{}, fmt.Errorf("parse cosign bundle: %w", err)
	}
	b := Bundle{
		ArtifactID:     artifactID,
		ArtifactDigest: env.Base64Signature, // not quite right — placeholder
		SignatureB64:   env.Base64Signature,
		CertificatePEM: env.Cert.RawBody,
		BundleURI:      env.RekorBundle.RekorBundleURL,
		Signer:         env.Cert.SubjectEmail,
	}
	if env.SignedAt > 0 {
		b.SignedAt = time.Unix(env.SignedAt, 0)
	}
	return b, nil
}

// cosignEnvelope is the minimal cosign-bundle shape we care about.
type cosignEnvelope struct {
	Base64Signature string `json:"base64Signature"`
	SignedAt        int64  `json:"signedAt,omitempty"`
	Cert            struct {
		RawBody      string `json:"rawBody"`
		SubjectEmail string `json:"subjectEmail"`
	} `json:"cert"`
	RekorBundle struct {
		RekorBundleURL string `json:"rekorBundleUrl"`
	} `json:"rekorBundle"`
}
