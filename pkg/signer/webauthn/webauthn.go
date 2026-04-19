// Package webauthn bridges a FIDO2 / WebAuthn authenticator to the
// Quidnug signer interface.
//
// Unlike an HSM signer that lives entirely server-side, WebAuthn is a
// two-party flow:
//
//  1. The server (this package) generates a challenge = canonical
//     signable bytes for the Quidnug transaction.
//  2. The browser or mobile client invokes navigator.credentials.get()
//     with that challenge and the user's registered credential ID.
//  3. The authenticator produces a WebAuthn assertion — authenticator
//     data + client data JSON + signature.
//  4. The server verifies the assertion and extracts the pure P-256
//     signature for inclusion in the Quidnug transaction envelope.
//
// This package provides:
//
//   - Registration(): bootstrap a user's WebAuthn credential as a
//     Quidnug quid. Stores the SEC1 public key and the WebAuthn
//     credential ID together.
//   - BeginSigning(): produce the challenge + credential allowList for
//     a navigator.credentials.get() call.
//   - FinishSigning(): validate the returned assertion, cross-check
//     the challenge, and return the raw DER signature to splice into
//     the Quidnug transaction.
//
// IMPORTANT: WebAuthn signs `authenticatorData || sha256(clientDataJSON)`,
// NOT the bare protocol bytes. This means the signature produced by a
// WebAuthn authenticator cannot be verified as a plain P-256
// signature over the canonical Quidnug bytes — it's a signature over
// a WebAuthn-specific envelope. Quidnug nodes therefore need a
// WebAuthn-aware verifier path for WebAuthn-signed transactions.
//
// This package emits the full assertion envelope alongside the
// signature so the node can perform WebAuthn verification when needed.
// The hybrid path — "extract a raw signature matching the Quidnug
// verifier" — is only possible in a platform authenticator that
// exposes raw ECDSA signing (e.g. on Android via CredentialManager
// with a dev-time dedicated credential), and is NOT the standard
// FIDO2 flow.
//
// # Status
//
// This package is a scaffold. The registration and authentication
// data models are wire-accurate and can be fed into a production
// verifier (e.g. github.com/go-webauthn/webauthn). A full
// verification chain (RP-ID hash check, origin validation, counter
// rollback) is out of scope for this initial drop — call out to the
// go-webauthn library for production use.
package webauthn

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"time"
)

// Credential is a registered WebAuthn credential as Quidnug sees it.
type Credential struct {
	// Raw WebAuthn credential ID (opaque to Quidnug; used in allowList).
	CredentialID []byte
	// SEC1 uncompressed public key (04||X||Y), same format as QuidID
	// derivation expects.
	PublicKeyHex string
	// Derived quid ID = sha256(publicKey)[:16] in hex.
	QuidID string
	// WebAuthn sign counter, for rollback detection.
	SignCount uint32
	// Created timestamp (unix seconds).
	CreatedAt int64
	// RP ID this credential is bound to.
	RPID string
	// Free-form label for UI display.
	Label string
}

// Challenge describes a server-issued challenge awaiting an assertion.
type Challenge struct {
	// Raw bytes passed to the authenticator (navigator.credentials.get
	// { publicKey: { challenge: ... }}).
	Bytes []byte
	// Canonical signable bytes of the Quidnug transaction. The
	// challenge is SHA-256(canonicalBytes) to keep the challenge
	// smaller than some authenticator limits, while still binding the
	// assertion to the specific transaction.
	CanonicalTxBytes []byte
	// TTL for challenge replay prevention.
	ExpiresAt time.Time
	// CredentialID the authenticator must use (from the user's prior
	// registration).
	CredentialID []byte
}

// AssertionRequest is what the server hands to the browser for
// navigator.credentials.get(). Encode to the WebAuthn-standard
// PublicKeyCredentialRequestOptions on the wire.
type AssertionRequest struct {
	ChallengeB64 string   `json:"challenge"`
	RPID         string   `json:"rpId"`
	Timeout      int      `json:"timeout"` // ms
	UserVerify   string   `json:"userVerification"`
	AllowList    []string `json:"allowCredentials"` // base64-encoded credential IDs
}

// AssertionResponse is the client's reply (PublicKeyCredential with
// response.clientDataJSON, response.authenticatorData, response.signature).
type AssertionResponse struct {
	CredentialIDB64       string `json:"id"`
	ClientDataJSONB64     string `json:"clientDataJSON"`
	AuthenticatorDataB64  string `json:"authenticatorData"`
	SignatureB64          string `json:"signature"`
	UserHandleB64         string `json:"userHandle,omitempty"`
}

// Config configures the WebAuthn server-side state.
type Config struct {
	RPID           string        // e.g. "quidnug.example.com"
	RPName         string        // human-readable relying-party name
	Origin         string        // e.g. "https://quidnug.example.com"
	ChallengeTTL   time.Duration // default 60s if zero
	Store          CredentialStore
	ChallengeStore ChallengeStore
}

// CredentialStore is the interface used to persist WebAuthn
// credentials. Implementations might be in-memory (for tests),
// backed by PostgreSQL, Redis, etc.
type CredentialStore interface {
	Save(c Credential) error
	Get(credentialID []byte) (*Credential, error)
	GetByQuidID(quidID string) (*Credential, error)
	UpdateSignCount(credentialID []byte, newCount uint32) error
}

// ChallengeStore persists one-shot challenges keyed by the challenge bytes.
type ChallengeStore interface {
	Put(challenge []byte, ch Challenge) error
	Consume(challenge []byte) (*Challenge, error) // removes on read
}

// Server is the server-side WebAuthn coordinator.
type Server struct {
	cfg Config
}

// New returns a configured WebAuthn server.
func New(cfg Config) (*Server, error) {
	if cfg.RPID == "" || cfg.Origin == "" {
		return nil, fmt.Errorf("RPID and Origin are required")
	}
	if cfg.Store == nil || cfg.ChallengeStore == nil {
		return nil, fmt.Errorf("Store and ChallengeStore are required")
	}
	if cfg.ChallengeTTL == 0 {
		cfg.ChallengeTTL = 60 * time.Second
	}
	return &Server{cfg: cfg}, nil
}

// BeginSigning issues a challenge bound to the given canonical
// transaction bytes. Returns the request body for the browser and the
// stored Challenge that FinishSigning will later consume.
//
// canonicalTxBytes is typically the output of client.CanonicalBytes
// on the pre-signature transaction. The challenge is
// SHA-256(canonicalTxBytes) — compact, binding, and within FIDO2
// length limits.
func (s *Server) BeginSigning(quidID string, canonicalTxBytes []byte) (*AssertionRequest, error) {
	cred, err := s.cfg.Store.GetByQuidID(quidID)
	if err != nil {
		return nil, fmt.Errorf("lookup credential: %w", err)
	}
	if cred == nil {
		return nil, fmt.Errorf("no credential registered for quid %s", quidID)
	}

	chSum := sha256.Sum256(canonicalTxBytes)
	// Append 16 bytes of fresh entropy so replay attempts with the
	// same tx cannot reuse a previous assertion.
	entropy := make([]byte, 16)
	if _, err := rand.Read(entropy); err != nil {
		return nil, err
	}
	challenge := append(chSum[:], entropy...)

	ch := Challenge{
		Bytes:            challenge,
		CanonicalTxBytes: canonicalTxBytes,
		ExpiresAt:        time.Now().Add(s.cfg.ChallengeTTL),
		CredentialID:     cred.CredentialID,
	}
	if err := s.cfg.ChallengeStore.Put(challenge, ch); err != nil {
		return nil, fmt.Errorf("persist challenge: %w", err)
	}
	return &AssertionRequest{
		ChallengeB64: base64.RawURLEncoding.EncodeToString(challenge),
		RPID:         s.cfg.RPID,
		Timeout:      int(s.cfg.ChallengeTTL / time.Millisecond),
		UserVerify:   "preferred",
		AllowList:    []string{base64.RawURLEncoding.EncodeToString(cred.CredentialID)},
	}, nil
}

// FinishSigning validates the assertion returned by the browser. On
// success it returns:
//
//   - the matching Credential (caller can extract quid id / public key)
//   - the raw WebAuthn-envelope-signed bytes (authData || SHA256(clientDataJSON))
//   - the raw DER signature as emitted by the authenticator
//
// NOTE: this package does NOT re-verify the ECDSA signature itself —
// plug in github.com/go-webauthn/webauthn for production verification.
// The simplified return surface is intentional: exposing the raw
// envelope lets callers build a WebAuthn-aware verifier into the
// Quidnug node without this package pretending to be a full WebAuthn
// stack.
func (s *Server) FinishSigning(resp AssertionResponse) (*Credential, []byte, []byte, error) {
	credID, err := base64.RawURLEncoding.DecodeString(resp.CredentialIDB64)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("bad credential id: %w", err)
	}
	clientDataJSON, err := base64.RawURLEncoding.DecodeString(resp.ClientDataJSONB64)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("bad clientDataJSON: %w", err)
	}
	authData, err := base64.RawURLEncoding.DecodeString(resp.AuthenticatorDataB64)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("bad authenticatorData: %w", err)
	}
	sig, err := base64.RawURLEncoding.DecodeString(resp.SignatureB64)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("bad signature: %w", err)
	}

	cred, err := s.cfg.Store.Get(credID)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("credential lookup: %w", err)
	}
	if cred == nil {
		return nil, nil, nil, fmt.Errorf("unknown credential")
	}

	// Extract challenge from clientDataJSON (type, challenge, origin).
	var cd clientData
	if err := decodeJSON(clientDataJSON, &cd); err != nil {
		return nil, nil, nil, fmt.Errorf("decode clientDataJSON: %w", err)
	}
	if cd.Type != "webauthn.get" {
		return nil, nil, nil, fmt.Errorf("clientData.type: %s", cd.Type)
	}
	if cd.Origin != s.cfg.Origin {
		return nil, nil, nil, fmt.Errorf("origin mismatch: %s != %s", cd.Origin, s.cfg.Origin)
	}
	challenge, err := base64.RawURLEncoding.DecodeString(cd.Challenge)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("decode challenge: %w", err)
	}
	stored, err := s.cfg.ChallengeStore.Consume(challenge)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("consume challenge: %w", err)
	}
	if stored == nil {
		return nil, nil, nil, fmt.Errorf("challenge expired or not issued")
	}
	if time.Now().After(stored.ExpiresAt) {
		return nil, nil, nil, fmt.Errorf("challenge expired")
	}

	// signableBytes = authData || SHA256(clientDataJSON)
	clientDataHash := sha256.Sum256(clientDataJSON)
	signableBytes := append(append([]byte{}, authData...), clientDataHash[:]...)

	return cred, signableBytes, sig, nil
}

// Registration bootstraps a WebAuthn credential as a Quidnug quid.
// Callers typically handle WebAuthn registration with a full library
// (go-webauthn) and then pass the resulting public-key bytes + cred id
// to this function.
func (s *Server) Registration(
	label string,
	credentialID []byte,
	publicKeyUncompressed []byte,
	signCount uint32,
) (*Credential, error) {
	if len(publicKeyUncompressed) != 65 || publicKeyUncompressed[0] != 0x04 {
		return nil, fmt.Errorf("public key must be 65-byte SEC1 uncompressed P-256 point")
	}
	sum := sha256.Sum256(publicKeyUncompressed)
	c := Credential{
		CredentialID: credentialID,
		PublicKeyHex: hex.EncodeToString(publicKeyUncompressed),
		QuidID:       hex.EncodeToString(sum[:8]),
		SignCount:    signCount,
		CreatedAt:    time.Now().Unix(),
		RPID:         s.cfg.RPID,
		Label:        label,
	}
	if err := s.cfg.Store.Save(c); err != nil {
		return nil, err
	}
	return &c, nil
}

type clientData struct {
	Type      string `json:"type"`
	Challenge string `json:"challenge"`
	Origin    string `json:"origin"`
}

func decodeJSON(raw []byte, v any) error {
	return jsonUnmarshal(raw, v)
}
