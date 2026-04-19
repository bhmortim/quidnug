// Package fhir bridges HL7 FHIR resources into Quidnug titles and
// event streams.
//
// Healthcare systems publish clinical events (observations, lab
// results, medication orders, claims) as FHIR resources. They are
// typically verified by digital signatures and routed across
// exchange hubs using TLS + OAuth2.
//
// This package lets a healthcare integrator:
//
//  1. Register a FHIR Patient (or Practitioner / Organization) as a
//     Quidnug Title — the Quidnug Title becomes the stable, portable
//     identity for that subject across FHIR servers.
//  2. Record FHIR resource events (Observation, Encounter, Claim, …)
//     on the Title's event stream, signed by the issuing provider's
//     Quidnug quid.
//  3. Use Quidnug relational trust to filter "which providers'
//     observations should I accept?" per-observer.
//
// The integration is intentionally narrow — it assumes the caller
// has a validated FHIR resource (R4 or R5) and a trusted mapping to
// the Quidnug Title. It emits events only.
package fhir
