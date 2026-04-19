# Implementation: AI Model Provenance

## 1. Register a dataset

```bash
curl -X POST $NODE/api/identities -d '{
  "quidId":"common-crawl-foundation",
  "name":"Common Crawl Foundation",
  "homeDomain":"ai.provenance.datasets",
  "creator":"common-crawl-foundation","updateNonce":1
}'

# The dataset itself is a TITLE owned by the curator
curl -X POST $NODE/api/v1/titles -d '{
  "assetId":"dataset-common-crawl-2024-cc-main-en",
  "domain":"ai.provenance.datasets",
  "titleType":"training-dataset",
  "owners":[{"ownerId":"common-crawl-foundation","percentage":100.0}],
  "attributes":{
    "datasetHash":"<sha256>",
    "sizeBytes":"3.1T",
    "language":"en",
    "license":"CC0",
    "exclusions":["copyrighted-shadow-libraries"]
  },
  "signatures":{"common-crawl-foundation":"<sig>"}
}'
```

## 2. Register a base model

```bash
# Identity for the lab
curl -X POST $NODE/api/identities -d '{
  "quidId":"acme-ai-labs",
  "name":"Acme AI Labs",
  "homeDomain":"ai.provenance.models.foundation",
  "creator":"acme-ai-labs","updateNonce":1
}'

# Install a guardian set for the lab (HSM failures happen)
curl -X POST $NODE/api/v2/guardian/set-update -d '{ /* ... */ }'

# The model itself
curl -X POST $NODE/api/v1/titles -d '{
  "assetId":"model-acme-foundation-7b-v2",
  "domain":"ai.provenance.models.foundation",
  "titleType":"ai-model",
  "owners":[{"ownerId":"acme-ai-labs","percentage":100.0}],
  "attributes":{
    "modelArchitecture":"decoder-transformer",
    "parameters":7000000000,
    "modelHash":"<sha256 of weights>",
    "license":"Apache-2.0",
    "trainingDataRefs":[
      "dataset-common-crawl-2024-cc-main-en",
      "dataset-acme-proprietary-licensed-books"
    ],
    "releaseDate":"2026-04-01"
  },
  "signatures":{"acme-ai-labs":"<sig>"}
}'
```

## 3. Training run events

```bash
# When training begins
curl -X POST $NODE/api/v1/events -d '{
  "subjectId":"model-acme-foundation-7b-v2",
  "subjectType":"TITLE",
  "eventType":"training.started",
  "payload":{
    "trainingDataRefs":["dataset-common-crawl-2024-cc-main-en"],
    "configHash":"<sha256 of training config>",
    "startedAt":1713400000,
    "expectedCompletion":1716000000,
    "computeEnv":"acme-gpu-cluster-us-east"
  },
  "creator":"acme-ai-labs","signature":"<sig>"
}'

# When training completes
curl -X POST $NODE/api/v1/events -d '{
  "subjectId":"model-acme-foundation-7b-v2",
  "subjectType":"TITLE",
  "eventType":"training.completed",
  "payload":{
    "finalLossHash":"<sha256>",
    "checkpointsHash":"<sha256>",
    "totalFLOPs":"1.2e23",
    "endedAt":1716000000
  },
  "creator":"acme-ai-labs","signature":"<sig>"
}'
```

## 4. Safety evaluation

An independent safety org (e.g., `anthropic-evals-team`) runs
tests and publishes:

```bash
curl -X POST $NODE/api/v1/events -d '{
  "subjectId":"model-acme-foundation-7b-v2",
  "subjectType":"TITLE",
  "eventType":"safety.evaluated",
  "payload":{
    "evaluatorOrg":"anthropic-evals-team",
    "evaluationHash":"<sha256 of full report>",
    "redTeamReportHash":"<sha256>",
    "overallRating":"acceptable",
    "knownIssues":["occasional-hallucination-on-math-problems"],
    "evaluationDate":1716100000
  },
  "creator":"anthropic-evals-team","signature":"<sig>"
}'
```

Anthropic Evals publishes their trust from whoever views them
as authoritative. Consumers doing their own trust eval weigh
Anthropic's signature by their own trust in Anthropic.

## 5. Benchmark submissions

MLCommons, HELM, or other benchmark orgs run tests:

```bash
curl -X POST $NODE/api/v1/events -d '{
  "subjectId":"model-acme-foundation-7b-v2",
  "subjectType":"TITLE",
  "eventType":"benchmark.submitted",
  "payload":{
    "benchmark":"MMLU",
    "score":0.78,
    "benchmarkVersion":"2024.04",
    "runDate":1716200000,
    "fullResultsHash":"<sha256>"
  },
  "creator":"mlcommons","signature":"<sig>"
}'
```

## 6. Derivative (fine-tune) authorization

Widget Corp fine-tunes Acme's model:

```bash
# First register the fine-tune as a title
curl -X POST $NODE/api/v1/titles -d '{
  "assetId":"model-widgetco-finetune-v1",
  "domain":"ai.provenance.models.fine-tunes",
  "titleType":"ai-model",
  "owners":[{"ownerId":"widget-corp","percentage":100.0}],
  "attributes":{
    "baseModelRef":"model-acme-foundation-7b-v2",
    "fineTuneDataRef":"dataset-widget-support-tickets",
    "intendedUse":"customer support",
    "license":"proprietary",
    "modelHash":"<sha256>"
  },
  "signatures":{"widget-corp":"<sig>"}
}'

# Acme signs an authorization event on the fine-tune title
curl -X POST $NODE/api/v1/events -d '{
  "subjectId":"model-widgetco-finetune-v1",
  "subjectType":"TITLE",
  "eventType":"derivation.authorized",
  "payload":{
    "baseModelRef":"model-acme-foundation-7b-v2",
    "derivativeModelRef":"model-widgetco-finetune-v1",
    "authorizedUses":["commercial-internal","non-commercial-research"],
    "forbiddenUses":["generative-content-for-resale"],
    "licenseTermsHash":"<sha256 of full license terms doc>"
  },
  "creator":"acme-ai-labs","signature":"<sig>"
}'
```

Without Acme's signed authorization, Widget Corp's fine-tune's
event stream lacks the `derivation.authorized` event. Downstream
consumers relying on that authorization can detect it.

## 7. Inference attestation

When the production service runs an inference:

```go
type InferenceAttestation struct {
    InferenceID   string
    ModelRef      string
    RequestHash   string
    ResponseHash  string
    Timestamp     int64
    ComputeEnv    string
}

func (s *InferenceServer) AttestInference(req InferenceRequest, resp InferenceResponse) error {
    event := map[string]interface{}{
        "subjectId":   s.modelQuid,
        "subjectType": "TITLE",
        "eventType":   "inference.ran",
        "payload": map[string]interface{}{
            "inferenceID":   req.ID,
            "requestHash":   sha256sum(req),
            "responseHash":  sha256sum(resp),
            "timestamp":     time.Now().Unix(),
            "computeEnv":    s.computeEnv,
        },
        "creator":   s.operatorQuid,
        "signature": s.sign(/* canonical bytes */),
    }
    return s.submitEvent(event)
}
```

Inference consumers can later verify: "This response I claim
came from model X at time T really did." Useful for:
- AI-generated content attribution
- Regulatory compliance ("which model produced this
  recommendation?")
- Debugging: "Did the right model handle this request?"

## 8. License claim and contest

A publisher detects content from their books in the model's
outputs:

```bash
curl -X POST $NODE/api/v1/events -d '{
  "subjectId":"model-acme-foundation-7b-v2",
  "subjectType":"TITLE",
  "eventType":"license.claimed",
  "payload":{
    "claimType":"copyright-violation",
    "claimantJurisdiction":"US",
    "evidenceHash":"<sha256>",
    "affectedWorks":["isbn-1234567890","isbn-1234567891"],
    "demandedRemedy":"cease + statutory damages"
  },
  "creator":"publisher-x","signature":"<sig>"
}'
```

Acme contests:

```bash
curl -X POST $NODE/api/v1/events -d '{
  "subjectId":"model-acme-foundation-7b-v2",
  "subjectType":"TITLE",
  "eventType":"license.contested",
  "payload":{
    "contestsClaimID":"<event ID of claim>",
    "argumentsHash":"<sha256 of response brief>",
    "evidenceHash":"<sha256 of training-data audit>"
  },
  "creator":"acme-ai-labs","signature":"<sig>"
}'
```

## 9. Consumer-side evaluation

```go
func (c *Consumer) PreflightModel(modelID string) PreflightReport {
    events := c.GetEvents(modelID, "TITLE")

    report := PreflightReport{ModelID: modelID}

    for _, ev := range events {
        switch ev.EventType {
        case "safety.evaluated":
            evaluator := ev.Payload["evaluatorOrg"].(string)
            trust := c.GetTrust(c.selfQuid, evaluator, "ai.provenance.safety")
            report.SafetyAttestations = append(report.SafetyAttestations,
                SafetyRecord{Evaluator: evaluator, Rating: ev.Payload["overallRating"].(string), Trust: trust.TrustLevel})

        case "benchmark.submitted":
            bench := ev.Payload["benchmark"].(string)
            score := ev.Payload["score"].(float64)
            signerTrust := c.GetTrust(c.selfQuid, ev.Creator, "ai.provenance.benchmarks")
            report.Benchmarks = append(report.Benchmarks,
                BenchmarkResult{Benchmark: bench, Score: score, ReporterTrust: signerTrust.TrustLevel})

        case "license.claimed":
            // Check if contested
            contested := c.hasContestEvent(events, ev.ID)
            if !contested {
                report.OpenLicenseClaims = append(report.OpenLicenseClaims, ev)
            }
        }
    }

    return report
}
```

## 10. Model key rotation (producer lost HSM)

Acme's signing HSM fails. Initiate guardian recovery:

```bash
curl -X POST $NODE/api/v2/guardian/recovery/init -d '{
  "subjectQuid":"acme-ai-labs",
  "fromEpoch":0,
  "toEpoch":1,
  "newPublicKey":"<hex>",
  "minNextNonce":1,
  "maxAcceptedOldNonce":0,
  "anchorNonce":<next>,
  "validFrom":<now>,
  "guardianSigs":[ /* Acme's CEO, CTO, CISO */ ]
}'
```

Post-rotation, downstream consumers still verify their
historical events (those used the old-epoch key, which is
still known in the ledger). New events use the new-epoch key.

## 11. Testing

```go
func TestModelProvenance_DerivationChainVerification(t *testing.T) {
    // Register dataset, base model, fine-tune, auth event
    // Verify: consumer traversing from fine-tune can reach
    //   original dataset + all safety attestations
}

func TestModelProvenance_UnauthorizedFineTuneDetectable(t *testing.T) {
    // Fine-tune title created without derivation.authorized event
    // Consumer's preflight: flags missing authorization
}

func TestModelProvenance_LicenseClaimContest(t *testing.T) {
    // Publisher files claim; Acme contests
    // Consumer sees both; can decide
}
```

## Where to go next

- [`threat-model.md`](threat-model.md)
- [`../ai-agent-authorization/`](../ai-agent-authorization/)
