# AI Model Provenance and Supply Chain

**AI · Provenance · Audit trails · Copyright attestation**

## The problem

The modern AI pipeline touches dozens of parties and artifacts:
- **Training datasets** from multiple sources (Common Crawl,
  licensed content, user-contributed, synthetic).
- **Base models** developed and licensed (Llama, Mistral, GPT,
  Claude).
- **Fine-tuned variants** building on the base.
- **Distilled / quantized / modified** derivatives.
- **Inference endpoints** serving the models.
- **Applications** consuming the inferences.

Claims across this chain are constantly contested:

- **Copyright.** "This model was trained on copyrighted content
  we didn't license." Class actions and regulatory investigations
  are piling up. The model developer's internal logs aren't a
  sufficient answer.
- **Attribution.** "This fine-tune is ours, don't claim it's
  yours." Derivative work disputes.
- **Safety.** "This model was fine-tuned for a purpose different
  from what's disclosed." (A model fine-tuned on malicious data,
  distributed under a benign name.)
- **Licensing.** "This model's license forbids commercial use,
  but the downstream app is commercial." The downstream
  application may not even know where the model came from.
- **Benchmarks / capabilities.** "Model X achieves Y on
  benchmark Z." Self-reported; hard to independently verify.

Today: internal CSVs, training-run tags, and README files.
Not cryptographic. Not verifiable by anyone other than the
model's producer.

## Why Quidnug fits

AI artifacts have natural identities and relationships:
- Datasets **are** identifiable things.
- Models **are derived from** datasets + parent models.
- Fine-tunes **are derived from** a specific base model
  and training data.
- Inferences **come from** specific model versions.

This is a **directed graph of signed claims**. Quidnug's
trust + title + event model fits directly.

| Problem                                        | Quidnug primitive                                  |
|------------------------------------------------|----------------------------------------------------|
| "What's this model trained on?"                | Title of model + events linking to dataset quids   |
| "Was this training authorized by data owner?"  | Signed event by data owner on the model title      |
| "What benchmarks has this model been tested on?"| Events from independent benchmarkers             |
| "Who claims this model is safe?"                | Trust edges from safety attesters                 |
| "Is this inference from the claimed model?"     | Inference output bound to model quid via signature |
| "Has this model been fine-tuned since release?" | Model's event stream lists all descendants         |

## High-level architecture

```
     ┌─────────────────────────────────────────────────┐
     │        ai.provenance.models (domain)             │
     └─────────────────────────────────────────────────┘
                         │
      ┌──────────────────┼──────────────────┐
      │                  │                  │
      ▼                  ▼                  ▼
 Dataset quids     Base-model quids    Fine-tune quids
      │                  │                  │
      │                  │                  │
      │ TITLE:           │ TITLE:            │ TITLE:
      │ "is-training-    │ "is-derived-     │ "is-derived-
      │  data-for"       │  from-dataset"    │  from-model"
      │                  │                  │
      ▼                  ▼                  ▼
 Event streams:    Event streams:     Event streams:
 - licensed        - training-run     - fine-tune-start
 - access-granted  - benchmarks       - benchmark
 - scraped         - safety-review    - deployed
                                     - inference-count
```

## Data model

### Quids

- **Dataset** — each training dataset has a quid. The dataset's
  owner (data curator) signs its metadata.
- **Model** — each model + version is a quid. Base models,
  fine-tunes, and quantized versions are distinct quids.
- **Model producer** — the organization that trains the model;
  has its own guardian set for key recovery.
- **Benchmark org** — MLCommons, HELM, etc.; publishes signed
  benchmark results.
- **Safety attester** — independent safety auditors.
- **Rights holder** — publisher, artist, code author with
  licensing claims.

### Domain

```
ai.provenance                       (top)
├── ai.provenance.datasets
├── ai.provenance.models
│   ├── ai.provenance.models.foundation
│   └── ai.provenance.models.fine-tunes
├── ai.provenance.licensing
└── ai.provenance.benchmarks
```

### Dataset title

```json
{
  "type": "TITLE",
  "assetId": "dataset-common-crawl-2024-cc-main-en",
  "domain": "ai.provenance.datasets",
  "titleType": "training-dataset",
  "owners": [{"ownerId": "common-crawl-foundation", "percentage": 100.0}],
  "attributes": {
    "datasetHash": "<sha256 of canonicalized dataset>",
    "sizeBytes": "3.1T",
    "language": "en",
    "contentType": "web-text",
    "collectionDate": "2024-12",
    "license": "CC0",
    "licenseURL": "https://commoncrawl.org/terms-of-use/",
    "exclusions": ["copyrighted-ebook-shadow-library",
                   "known-malicious-sites"]
  },
  "signatures": {"common-crawl-foundation": "<sig>"}
}
```

### Model title

```json
{
  "type": "TITLE",
  "assetId": "model-acme-foundation-7b-v2",
  "domain": "ai.provenance.models.foundation",
  "titleType": "ai-model",
  "owners": [{"ownerId": "acme-ai", "percentage": 100.0}],
  "attributes": {
    "modelArchitecture": "decoder-transformer",
    "parameters": 7000000000,
    "modelHash": "<sha256 of model weights>",
    "framework": "PyTorch",
    "trainingDataRef": [
      "dataset-common-crawl-2024-cc-main-en",
      "dataset-acme-proprietary-licensed-books"
    ],
    "license": "Apache-2.0",
    "releaseDate": "2026-04-01",
    "trainingCompute": "1.2e23 FLOPs"
  },
  "signatures": {"acme-ai": "<sig>"}
}
```

### Training run events

On the model's stream:

```
1. training.started
   payload: { trainingDataRefs: [...], config: <hash>,
              startedAt: ... }
   signer: acme-ai

2. training.completed
   payload: { finalLossHash: ..., checkpointsHash: ...,
              totalFLOPs: ..., endedAt: ... }
   signer: acme-ai

3. safety.evaluated
   signer: safety-org-anthropic-evals
   payload: { evaluatorOrg: ..., evaluationHash: ...,
              redTeamReportHash: ..., overallRating: "acceptable" }

4. benchmark.submitted
   signer: mlcommons
   payload: { benchmark: "MMLU", score: 0.78, runDate: ... }

5. license.claimed
   signer: rights-holder-publisher-X
   payload: { claim: "model trained on copyrighted books ...",
              counterclaimID: null, evidenceHash: ... }
```

### Derivative model (fine-tune)

```json
{
  "type": "TITLE",
  "assetId": "model-widgetco-finetune-for-support",
  "domain": "ai.provenance.models.fine-tunes",
  "owners": [{"ownerId": "widget-corp", "percentage": 100.0}],
  "attributes": {
    "baseModelRef": "model-acme-foundation-7b-v2",
    "fineTuneData": "dataset-widget-support-tickets-private",
    "modelHash": "<sha256>",
    "license": "inherited + proprietary additions",
    "intendedUse": "customer support chatbot"
  },
  "signatures": {"widget-corp": "<sig>"}
}
```

On this title's stream, `derivation.authorized` events from
the base model's owner:

```
derivation.authorized
  signer: acme-ai   (the base model owner)
  payload: { derivativeModelID: "model-widgetco-finetune-for-support",
             authorizedUses: ["commercial", "non-commercial"],
             termsHash: "<sha256 of license>" }
```

## Inference attestation

When a model serves an inference, it can emit a signed
inference-ran event:

```
eventType: "inference.ran"
subjectId: <model quid>
payload: {
  inferenceID: "inf-abc-123",
  requestHash: "<sha256 of prompt>",
  responseHash: "<sha256 of response>",
  timestamp: ...,
  computeEnv: "acme-gpu-cluster-us-east"
}
signer: model producer
```

A downstream consumer can verify: "The inference I received was
produced by model X, running at time T." No one can forge an
inference claim from a model without that model's producer's
key.

## Consumer trust

A downstream application (e.g., an LLM-powered customer support
product):

```go
func (app *App) EvaluateModel(modelID string) ModelAssessment {
    title := app.quidnug.GetTitle(modelID)
    events := app.quidnug.GetSubjectEvents(modelID, "TITLE")

    // Check each attestation's source via relational trust
    var safetyOK bool
    for _, ev := range events {
        if ev.EventType == "safety.evaluated" {
            trust, _ := app.quidnug.GetTrust(app.quid,
                ev.Payload["evaluatorOrg"].(string),
                "ai.provenance.safety", nil)
            if trust.TrustLevel >= 0.8 {
                safetyOK = true
                break
            }
        }
    }

    // Check for license claims
    hasUnresolvedLicenseClaims := false
    for _, ev := range events {
        if ev.EventType == "license.claimed" && ev.Payload["counterclaimID"] == nil {
            // Unresolved copyright claim — risky
            claimantTrust, _ := app.quidnug.GetTrust(app.quid,
                ev.Payload["signer"].(string),
                "ai.provenance.licensing", nil)
            if claimantTrust.TrustLevel >= 0.5 {
                hasUnresolvedLicenseClaims = true
            }
        }
    }

    return ModelAssessment{
        SafetyVerified:          safetyOK,
        UnresolvedLicenseIssues: hasUnresolvedLicenseClaims,
        ReadyForProduction:      safetyOK && !hasUnresolvedLicenseClaims,
    }
}
```

## Counter-attestations

Disputes happen. A rights holder files a license-claim event.
The model producer can file a `license.contested`:

```
eventType: "license.contested"
payload: {
  contestsClaimID: <earlier event ID>,
  evidence: <hash>,
  arguments: "Model was trained on publicly available
              summaries, not full text. Summaries are
              transformative under fair use..."
}
```

Both claim and contest live in the record. Consumers weigh
their trust in both parties. Courts (if it gets there) have
a full signed evidence chain.

## Key Quidnug features

- **Title-of-title hierarchy** — dataset, base model, fine-tune
  all have titles; event links model them into a DAG.
- **Event streams per artifact** — training runs, safety evals,
  benchmarks, license claims.
- **Domain hierarchy** — scope trust by dataset provenance vs.
  model safety vs. licensing.
- **Relational trust** — different consumers trust different
  safety orgs / benchmarkers.
- **Guardian sets** — model producer's signing keys
  recoverable (a lab's HSM loss shouldn't orphan all their
  published models).
- **Push gossip** — new claims (especially safety and
  license) propagate immediately.

## Value delivered

| Dimension                                  | Before                                      | With Quidnug                                             |
|--------------------------------------------|---------------------------------------------|----------------------------------------------------------|
| Dataset provenance                         | README files                                | Signed title + hash; verifiable                           |
| Model-to-dataset linkage                   | Blog posts                                  | Signed derivation relationship                            |
| Safety attestation                         | Internal labs / private audits              | On-chain claims from independent attesters                |
| License dispute evidence                   | Emails, depositions                         | Signed claim/counterclaim chain                           |
| Benchmark result verification              | Self-reported                               | Benchmark org's signed event                              |
| Fine-tune authorization                    | Contract + trust                            | `derivation.authorized` event                             |
| Inference authenticity                     | Rely on endpoint                            | Signed inference event                                    |
| Consumer evaluation                        | Vendor's marketing                          | Algorithmic: trust × attestations                         |

## What's in this folder

- [`README.md`](README.md) — this document
- [`implementation.md`](implementation.md) — Quidnug API calls
- [`threat-model.md`](threat-model.md) — security analysis

## Related

- [`../ai-agent-authorization/`](../ai-agent-authorization/) — authorizing the agent built on top of a model
- [`../ai-content-authenticity/`](../ai-content-authenticity/) — provenance for AI-generated content
- [QDP-0005 Push Gossip](../../docs/design/0005-push-based-gossip.md)
