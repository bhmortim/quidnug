# Federated Learning Gradient Attestation

**AI · Federated learning · Multi-party cryptographic attestation**

## The problem

Multiple organizations want to train a shared machine-learning
model without pooling their raw data. Federated learning (FL)
is the pattern:

- Each participant trains on their own local data.
- Participants share model updates (gradients) with a
  coordinator.
- Coordinator aggregates gradients, updates global model.
- Process repeats until convergence.

Example: 20 banks collaborate on a fraud-detection model.
Each bank's transaction data is sensitive; FL lets them
contribute without exposing individual records.

The trust problems:

1. **Who actually contributed what?** The coordinator claims
   bank A submitted gradient update `g_A`. Bank A can deny it.
   Bank B can claim they submitted when they didn't. Credit
   allocation for training contribution is contested.

2. **Was the contribution real?** A bank claiming it trained
   on 10M transactions but actually only used 10k could be
   freeriding.

3. **Coordinator trust.** Participants trust the coordinator
   to aggregate fairly. But who watches the coordinator?
   A compromised coordinator could bias the global model.

4. **Adversarial gradients.** A participant could submit
   manipulated gradients designed to degrade or bias the
   model. Detection requires comparison against normal
   distributions.

5. **Privacy vs. auditability.** Stronger crypto (homomorphic
   encryption, secure aggregation) hides individual gradients
   — but then there's no evidence of individual contribution.

## Why Quidnug fits

Each gradient update is a **signed event** on the training
round's stream. Each participant signs their own gradient;
the signature is the proof-of-contribution. The coordinator
aggregates and signs the result. All observable, all
auditable.

| Problem                                      | Quidnug primitive                             |
|---------------------------------------------|-----------------------------------------------|
| "Did participant X really contribute?"      | Signed `gradient.submitted` event             |
| "How much did they contribute?"             | Payload metadata + attestation metrics         |
| "Was the aggregation fair?"                 | Coordinator's signed `round.aggregated` event |
| "Credit allocation for trained model"       | Event stream is the ledger of contributions    |
| "Compromised coordinator?"                  | Push gossip → participants see same state      |
| "Re-create the round after dispute"         | Replay the event stream                        |

## High-level architecture

```
                  Training round as a quid
               ("fl-round-2026-04-18-fraud")
                           │
                           ▼
              ┌──────────────────────────┐
              │    Round's event stream   │
              │                            │
              │ 1. round.opened            │
              │ 2. participant.registered  │
              │ 3. model.state.published   │
              │ 4. gradient.submitted (×N) │
              │ 5. round.aggregated        │
              │ 6. model.state.updated     │
              │ 7. round.closed            │
              └──────────────────────────┘
                           │
          ┌────────────────┼────────────────┬────────────────┐
          │                │                │                │
          ▼                ▼                ▼                ▼
   bank-A quid      bank-B quid      bank-C quid      coordinator quid
   (participant)    (participant)    (participant)    (coordinator)
```

## Data model

### Quids
- **Coordinator** — runs the aggregation; has a quid.
- **Participant** — each participating organization; has a
  quid with its own guardian set for key recovery.
- **Training round** — each round of FL has its own quid.
  The round's event stream is the canonical record.

### Domain
```
ai.federated-learning.fraud-detection
ai.federated-learning.drug-discovery
ai.federated-learning.industrial-iot
```

### Round lifecycle events

```
Event 1: round.opened
  subjectId: fl-round-2026-04-18-fraud
  payload:
    roundNumber: 47
    modelStateHash: <hash of starting weights>
    deadline: 2026-04-18T18:00:00Z
    eligibleParticipants: [bank-A, bank-B, bank-C, ...]
    minParticipants: 5
  signer: coordinator

Event 2: participant.registered (each participant)
  payload:
    participant: bank-A
    attestedDataSize: 10000000
    attestedDataHash: <hash of training set schema>
  signer: bank-A

Event 3: model.state.published (coordinator)
  payload:
    modelWeightsCID: <IPFS CID of current weights>
    schema: <hash>
  signer: coordinator

Event 4a: gradient.submitted (bank-A)
  payload:
    participant: bank-A
    gradientCID: <IPFS CID of encrypted gradient>
    gradientHash: <sha256 of plaintext gradient for later reveal>
    trainingDataSize: 10000000
    trainingDuration: 3600
    signatureOfLocalModel: <sig>
  signer: bank-A

  (Repeat for each participant)

Event 5: round.aggregated
  payload:
    participatingRoundMembers: [bank-A, bank-B, ...]
    aggregateGradientCID: <IPFS CID>
    aggregationMethod: "fedavg"
    weightedByContribution: true
    weightingSchemeHash: <hash>
  signer: coordinator

Event 6: model.state.updated
  payload:
    newModelWeightsCID: <CID>
    newStateHash: <hash>
    roundNumber: 47
    improvementMetric: 0.023
  signer: coordinator

Event 7: round.closed
  signer: coordinator
```

## Per-participant credit tracking

At the round's close, each participant's contribution is
provably recorded:

```go
func (p *Participant) MyContributions(modelRef string) []Contribution {
    rounds := p.GetRoundsForModel(modelRef)
    contributions := []Contribution{}
    for _, round := range rounds {
        events := p.GetEvents(round.Quid, "QUID")
        for _, ev := range events {
            if ev.EventType == "gradient.submitted" &&
               ev.Payload["participant"] == p.quid {
                contributions = append(contributions, Contribution{
                    RoundID:           round.Quid,
                    DataSize:          ev.Payload["trainingDataSize"],
                    TrainingDuration:  ev.Payload["trainingDuration"],
                    GradientHash:      ev.Payload["gradientHash"],
                })
            }
        }
    }
    return contributions
}
```

Commercial arrangement: revenue from the trained model is
distributed pro-rata based on contribution metrics. The
event stream is the arbitrable record.

## Byzantine-robust aggregation

Coordinator must defend against participants submitting
malicious gradients.

Strategies (application-layer, supported by Quidnug's
record-keeping):

1. **Outlier detection.** Compare each participant's gradient
   against the distribution. Extreme outliers flagged in
   `gradient.flagged` events.
2. **Trimmed aggregation.** Coordinator discards top-K%
   outliers; records what was discarded in
   `round.aggregated` event.
3. **Reputation weighting.** Participants with history of
   flagged gradients get lower weight in future rounds.
   Quidnug's trust edges encode this.

```
coordinator ──0.9──► bank-A  (consistently good gradients)
coordinator ──0.3──► bank-outlier-y  (often flagged; deprioritized)
```

## Coordinator accountability

Participants verify the coordinator is playing fair:

1. All participants see the same event stream (push gossip).
2. Each participant can re-run the aggregation locally
   using the submitted gradients (if decrypted under the
   consortium's agreed protocol) and verify the
   coordinator's claimed result.
3. If the coordinator's aggregate doesn't match
   participants' independent re-aggregation:
   `round.disputed` event. Other participants can vote
   to switch coordinators in the next round.

## Privacy-preserving variant

For stronger privacy (gradients never revealed to peers or
coordinator), layer homomorphic encryption or secure
aggregation on top. Quidnug still records:
- Who participated (signed registration).
- Each participant's ciphertext submission hash.
- Coordinator's aggregate result.

The contribution **metadata** (size, duration) stays on-chain;
the raw gradient stays encrypted. Best-of-both: accountability
+ privacy.

## Key Quidnug features

- **Event streams** — round = subject, contributions = events.
- **Signed events** — cryptographic proof of contribution.
- **Push gossip (QDP-0005)** — all participants see the same
  round state within seconds.
- **Relational trust** — weighting bad-gradient contributors
  lower.
- **Guardian recovery** — participant HSM loss doesn't orphan
  their history.
- **Domain hierarchy** — scope federated-learning rounds to
  specific topics.
- **K-of-K bootstrap** — new participants onboard by fetching
  consensus state from existing participants.

## Value delivered

| Dimension                         | Before                                     | With Quidnug                                       |
|-----------------------------------|--------------------------------------------|----------------------------------------------------|
| Contribution proof                | Coordinator's word                         | Signed events, cryptographic                        |
| Revenue sharing disputes          | Contractual + audits                       | Deterministic replay of chain                       |
| Byzantine gradient detection      | Coordinator-internal                       | Flagged events visible to all                       |
| Coordinator accountability        | Trust the operator                         | Peers re-verify aggregation                         |
| Cross-round reputation            | Not tracked                                | Trust edges evolve over time                        |
| Privacy                          | Either all-or-nothing                      | Metadata on-chain, gradients encrypted              |
| Dispute resolution                | Arbitration ± logs                         | Cryptographic event chain replay                    |

## What's in this folder

- [`README.md`](README.md) — this document
- [`implementation.md`](implementation.md) — API calls
- [`threat-model.md`](threat-model.md) — security analysis

## Runnable POC

Full end-to-end demo at
[`examples/federated-learning-attestation/`](../../examples/federated-learning-attestation/):

- `fl_audit.py` — pure audit logic: registration / submission
  matching, suspicious-gradient flagging by median-norm ratio,
  fair-weights computation for coordinator-bias detection.
- `fl_audit_test.py` — 11 pytest cases.
- `demo.py` — four rounds showing valid / insufficient /
  suspicious-gradient-flagged / strict-registration-violation.

```bash
cd examples/federated-learning-attestation
python demo.py
```

## Related

- [`../ai-model-provenance/`](../ai-model-provenance/) — the resulting model's lineage
- [`../merchant-fraud-consortium/`](../merchant-fraud-consortium/) — similar many-parties-sharing-signals pattern
- [QDP-0005 Push Gossip](../../docs/design/0005-push-based-gossip.md)
