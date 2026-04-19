# Quidnug × HL7 FHIR bridge

`integrations/fhir` records FHIR R4/R5 resources as events on
Quidnug titles so healthcare trust decisions (which provider's
observations to accept, which claim chain is authoritative) become
per-observer queries in the Quidnug graph.

## Usage

```go
rec, _ := fhir.New(fhir.Options{ Client: c, Domain: "healthcare.hie1" })

_, err := rec.RecordResource(ctx, providerQuid, patientTitleID, fhir.Resource{
    ResourceType: "Observation",
    ID:           "obs-12345",
    Subject:      "Patient/789",
    Issuer:       "Practitioner/dr-alice",
    Status:       "final",
    EffectiveAt:  time.Now().Unix(),
    Body:         parsedFHIRJSON,
})
```

Each resource becomes one EVENT with event_type
`FHIR_RESOURCE.{ResourceType}` — e.g. `FHIR_RESOURCE.Observation`,
`FHIR_RESOURCE.MedicationRequest`, `FHIR_RESOURCE.Claim`.

## Patterns

- **Title = Patient/Practitioner/Organization quid.** Register a
  Quidnug Title for each FHIR subject (typically one Quidnug quid per
  real-world principal). Use the title's asset ID as the SubjectID
  for all events about that subject.

- **Issuer = signing provider quid.** The `issuer` in `EmitEvent` is
  the provider quid that verified the FHIR resource and is asserting
  it onto the graph. Their trust is what downstream consumers score.

- **Trust-based filtering.** A querying app asks "given observer O
  and patient P's event stream, which events were signed by a provider
  O transitively trusts at ≥ 0.7?"

## Scope

- FHIR-side validation (profile conformance, value-set checks, digital
  signature on the FHIR resource itself) is the caller's job. Use an
  existing FHIR validator upstream.
- No SMART-on-FHIR or OAuth token handling — use the official
  smart-on-fhir libraries for those concerns.

## License

Apache-2.0.
