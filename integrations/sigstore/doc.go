// Package sigstore mirrors cosign / sigstore artifact signatures into
// Quidnug event streams, making supply-chain provenance queryable via
// the relational-trust graph.
//
// # The Problem
//
// Sigstore (cosign, rekor, fulcio) gives you cryptographically-signed
// attestations of software artifacts — but trust in those signatures
// is binary: either you trust the signer's Fulcio identity, or you
// don't. Quidnug's per-observer relational trust gives you a richer
// answer: "I trust cosign signers A and B at 1.0, and transitively
// trust anything they countersigned at decaying levels, up to depth
// N".
//
// This package bridges the two:
//
//  1. Take a cosign signature bundle (artifact digest + sig + cert).
//  2. Extract the signer identity (Fulcio-issued cert subject).
//  3. Record an EVENT on the artifact's Quidnug Title with
//     subject_type=TITLE, event_type=SIGSTORE_SIGNATURE, payload=
//     {digest, signature, cert, bundleUri, signer, signedAt}.
//
// Downstream consumers can then query:
//
//   - "What signatures exist on this artifact?" → stream events on the
//     title.
//   - "How trusted are those signers from my perspective?" → relational
//     trust queries per signer.
//   - "Show me the first countersigned artifact produced by a team my
//     vendor trusts at > 0.7" → join of events + trust graph.
//
// # Scope
//
// This is a scaffold implementation. It parses cosign-style JSON
// bundles (matching the sigstore bundle spec v0.2) and records them
// as events. It does NOT re-verify the cosign signature — use the
// official sigstore-go library for verification before passing
// bundles into RecordBundle().
package sigstore
