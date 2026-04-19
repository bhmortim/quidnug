package oidc

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/quidnug/quidnug/pkg/client"
)

// Bridge is the OIDC → Quidnug adapter. It handles the "given a
// verified OIDC ID token, produce or resolve the bound quid" flow.
//
// Construction is intentionally decoupled from any particular OIDC
// library — callers pass in the already-validated token claims. Use
// github.com/coreos/go-oidc for ID-token signature + issuer + audience
// + exp + nonce checks.
type Bridge struct {
	store   BindingStore
	signer  QuidFactory
	defDom  string
}

// IDToken is the minimal subset of OIDC ID-token claims the bridge
// cares about. Populate by verifying the JWT with go-oidc's Verifier
// and extracting Claims.
type IDToken struct {
	Issuer  string
	Subject string
	Email   string
	Name    string
}

// QuidFactory produces Quid-shaped signer identities. Typically wraps
// hsm.Open or an in-process key generator for dev.
type QuidFactory interface {
	// CreateNew provisions a fresh quid for a just-bound subject.
	// The returned object must remain usable for the lifetime of the
	// binding — the bridge does NOT keep a reference beyond the
	// current request.
	CreateNew(ctx context.Context, issuer, subject, email string) (BoundQuid, error)

	// Get retrieves the signer for an already-bound quid.
	Get(ctx context.Context, quidID string) (BoundQuid, error)
}

// BoundQuid is the minimal interface the bridge needs. Both
// *client.Quid (in-process) and signer.Signer-backed proxies satisfy it.
//
// Exported so external callers (e.g. cmd/quidnug-oidc) can implement
// their own QuidFactory whose method signatures reference it.
type BoundQuid interface {
	ID() string
	PublicKeyHex() string
	Sign(data []byte) (string, error)
}

// Options configures a Bridge.
type Options struct {
	// Store persists bindings. Required.
	Store BindingStore
	// Signer produces/resolves quid keys. Required.
	Signer QuidFactory
	// DefaultDomain is the trust domain used when the caller doesn't
	// override it (typically per-tenant).
	DefaultDomain string
}

// New constructs a Bridge.
func New(opts Options) (*Bridge, error) {
	if opts.Store == nil {
		return nil, errors.New("Store is required")
	}
	if opts.Signer == nil {
		return nil, errors.New("Signer is required")
	}
	if opts.DefaultDomain == "" {
		opts.DefaultDomain = "default"
	}
	return &Bridge{
		store:  opts.Store,
		signer: opts.Signer,
		defDom: opts.DefaultDomain,
	}, nil
}

// Resolve returns the Binding + signer for an OIDC login. On a first
// login for (issuer, sub), it provisions a fresh quid and records the
// binding. Subsequent logins resolve to the existing quid.
//
// The caller is responsible for ID-token verification BEFORE calling
// Resolve — the bridge trusts the supplied claims implicitly.
func (b *Bridge) Resolve(ctx context.Context, tok IDToken) (*Binding, BoundQuid, error) {
	if tok.Issuer == "" || tok.Subject == "" {
		return nil, nil, errors.New("Issuer and Subject are required")
	}
	if existing, err := b.store.Get(tok.Issuer, tok.Subject); err != nil {
		return nil, nil, err
	} else if existing != nil {
		_ = b.store.Update(tok.Issuer, tok.Subject, tok.Email, tok.Name)
		signer, err := b.signer.Get(ctx, existing.QuidID)
		if err != nil {
			return nil, nil, fmt.Errorf("resolve signer: %w", err)
		}
		return existing, signer, nil
	}

	// First login: provision.
	newSigner, err := b.signer.CreateNew(ctx, tok.Issuer, tok.Subject, tok.Email)
	if err != nil {
		return nil, nil, fmt.Errorf("provision quid: %w", err)
	}
	bnd := Binding{
		Issuer:    tok.Issuer,
		Subject:   tok.Subject,
		QuidID:    newSigner.ID(),
		Email:     tok.Email,
		Name:      tok.Name,
		CreatedAt: time.Now(),
	}
	if err := b.store.Create(bnd); err != nil {
		return nil, nil, fmt.Errorf("persist binding: %w", err)
	}
	out := bnd
	return &out, newSigner, nil
}

// RegisterIdentityOnNode registers the bound quid as an identity in
// the Quidnug node. Idempotent — the node's NONCE_REPLAY / duplicate
// detection handles repeated calls. Returns the server's receipt.
func (b *Bridge) RegisterIdentityOnNode(
	ctx context.Context,
	c *client.Client,
	binding *Binding,
	signer BoundQuid,
) error {
	// Build an in-process *client.Quid shim the client package knows
	// how to use. The signer comes from the bridge, not from a
	// local keypair.
	proxy := clientQuidFromSigner(signer)
	_, err := c.RegisterIdentity(ctx, proxy, client.IdentityParams{
		Name:       binding.Name,
		HomeDomain: b.defDom,
		Attributes: map[string]any{
			"oidc": map[string]any{
				"issuer":  binding.Issuer,
				"subject": binding.Subject,
			},
			"email": binding.Email,
		},
	})
	return err
}

// clientQuidFromSigner adapts a BoundQuid to the *client.Quid the SDK
// accepts. The production implementation is in pkg/signer.SignableQuid;
// we inline a minimal adapter here to avoid a circular dependency.
type signingQuidAdapter struct {
	s BoundQuid
}

func clientQuidFromSigner(s BoundQuid) *client.Quid {
	// The pkg/client type is concrete; we fake one by constructing
	// from the public key and then overriding the signing interface
	// via an internal shim. This is stable as long as pkg/client
	// exposes QuidFromPublicHex + a way to inject a signer.
	//
	// Simplest reliable path: build a QuidFromPublicHex (read-only)
	// and rely on the caller to splice the signature manually. For
	// the scaffold, we return the read-only handle; a real bridge
	// should construct a more featureful adapter when
	// pkg/client grows a Signer hook.
	q, _ := client.QuidFromPublicHex(s.PublicKeyHex())
	_ = signingQuidAdapter{s: s}
	return q
}
