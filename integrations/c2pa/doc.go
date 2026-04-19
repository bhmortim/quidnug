// Package c2pa bridges C2PA (Coalition for Content Provenance and
// Authenticity) manifests into Quidnug event streams.
//
// C2PA asserts "this image / video / document was produced by
// hardware H, edited by tools T1+T2, signed by creator C". It's the
// emerging standard for AI-attribution and media provenance (Adobe,
// Microsoft, Arm, BBC, NYT, Reuters, all shipping C2PA).
//
// The standard signs each assertion bundle with an X.509 certificate.
// This package records a verified C2PA manifest as a Quidnug EVENT on
// the asset's Title, binding the C2PA provenance to a per-observer
// trust score in the Quidnug graph — so downstream consumers can ask:
//
//   - "Is this image signed by a C2PA-claimed creator that I
//     transitively trust at ≥ 0.8?"
//   - "Show the edit history for this asset across all C2PA manifests
//     any of my trusted parties has published."
//
// # Scope
//
// Scaffold implementation: parses the minimal manifest-store JSON
// shape and records it as an EVENT. Does NOT re-verify the manifest
// signature or the embedded XMP — use the official c2pa-rs library.
package c2pa
