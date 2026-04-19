# Implementation: Federated Learning Attestation

## 1. Setup

Each participant runs a Quidnug node.  Coordinator runs one
too.  All are on the same consortium domain, e.g.
`ai.federated-learning.fraud-detection`.

```bash
# Each participant creates identity + guardian set
curl -X POST $NODE/api/identities -d '{
  "quidId":"bank-a",
  "name":"Bank A",
  "homeDomain":"ai.federated-learning.fraud-detection",
  "creator":"bank-a","updateNonce":1
}'
# ... + guardian set for key recovery

# Coordinator likewise
curl -X POST $NODE/api/identities -d '{
  "quidId":"fl-coordinator",
  "name":"FL Coordinator Acme",
  "homeDomain":"ai.federated-learning.fraud-detection",
  "creator":"fl-coordinator","updateNonce":1
}'
```

## 2. Open a round

Each training round is a new quid.

```bash
curl -X POST $NODE/api/identities -d '{
  "quidId":"fl-round-2026-04-18-fraud-47",
  "name":"FL Fraud Detection Round 47",
  "creator":"fl-coordinator","updateNonce":1,
  "attributes":{
    "modelRef":"model-consortium-fraud-v1",
    "roundNumber":47
  }
}'

# Coordinator opens the round
curl -X POST $NODE/api/v1/events -d '{
  "subjectId":"fl-round-2026-04-18-fraud-47",
  "subjectType":"QUID",
  "eventType":"round.opened",
  "payload":{
    "roundNumber":47,
    "modelStateHash":"<sha256>",
    "deadline":1713484800,
    "eligibleParticipants":["bank-a","bank-b","bank-c","bank-d","bank-e"],
    "minParticipants":4,
    "startingWeightsCID":"bafy..."
  },
  "creator":"fl-coordinator","signature":"<sig>"
}'
```

## 3. Participants register + submit

```bash
# Bank A registers
curl -X POST $NODE/api/v1/events -d '{
  "subjectId":"fl-round-2026-04-18-fraud-47",
  "subjectType":"QUID",
  "eventType":"participant.registered",
  "payload":{
    "participant":"bank-a",
    "attestedDataSize":10000000,
    "attestedDataSchemaHash":"<sha256>"
  },
  "creator":"bank-a","signature":"<sig>"
}'

# Bank A does local training (off-chain), then submits gradient
curl -X POST $NODE/api/v1/events -d '{
  "subjectId":"fl-round-2026-04-18-fraud-47",
  "subjectType":"QUID",
  "eventType":"gradient.submitted",
  "payload":{
    "participant":"bank-a",
    "gradientCID":"bafy... (encrypted gradient in IPFS)",
    "gradientHash":"<sha256 of plaintext>",
    "trainingDataSize":10000000,
    "trainingDurationSec":3600,
    "framework":"PyTorch",
    "localModelHash":"<sha256 post-training>",
    "dpNoiseScale":0.001
  },
  "creator":"bank-a","signature":"<sig>"
}'
```

## 4. Coordinator aggregates

```go
func (c *Coordinator) RunRound(ctx context.Context, roundID string) error {
    events, _ := c.GetEvents(ctx, roundID)

    // Gather submitted gradients
    submissions := []Submission{}
    for _, ev := range events {
        if ev.EventType == "gradient.submitted" {
            submissions = append(submissions, parseSubmission(ev))
        }
    }
    if len(submissions) < c.minParticipants {
        return fmt.Errorf("round aborted: below min participants")
    }

    // Detect outliers (Byzantine robust)
    flagged := c.DetectOutliers(submissions)
    for _, f := range flagged {
        c.SubmitEvent(ctx, roundID, "gradient.flagged", map[string]interface{}{
            "participant": f.Participant,
            "reason":      f.Reason,
            "deviation":   f.Deviation,
        })
    }

    // Aggregate non-flagged gradients
    accepted := filterOut(submissions, flagged)
    aggregate := c.FedAvg(accepted)
    aggregateCID := c.UploadIPFS(aggregate)

    // Emit aggregation result
    return c.SubmitEvent(ctx, roundID, "round.aggregated", map[string]interface{}{
        "participatingMembers": participantIDs(accepted),
        "aggregateGradientCID": aggregateCID,
        "aggregationMethod":    "fedavg",
        "weightingScheme":      c.WeightingSchemeHash(),
        "flaggedMembers":       participantIDs(flagged),
    })
}
```

## 5. Participants verify

```go
func (p *Participant) VerifyRound(ctx context.Context, roundID string) error {
    events, _ := p.GetEvents(ctx, roundID)

    var aggEvent *Event
    submissions := []Submission{}
    for _, ev := range events {
        switch ev.EventType {
        case "round.aggregated":
            aggEvent = ev
        case "gradient.submitted":
            submissions = append(submissions, parseSubmission(ev))
        }
    }
    if aggEvent == nil {
        return fmt.Errorf("no aggregation event")
    }

    // Participants that can decrypt (consortium key) can
    // independently re-run the aggregation.
    recomputed := p.FedAvg(filterAccepted(submissions, aggEvent))
    claimedCID := aggEvent.Payload["aggregateGradientCID"].(string)
    claimed := p.DownloadIPFS(claimedCID)
    if !bytes.Equal(p.Hash(recomputed), p.Hash(claimed)) {
        // Coordinator's aggregation differs from our computation.
        // Emit a dispute event.
        return p.SubmitEvent(ctx, roundID, "round.disputed", map[string]interface{}{
            "participant":     p.quid,
            "claimedHash":     sha256sum(claimed),
            "recomputedHash":  sha256sum(recomputed),
            "reason":          "aggregation-mismatch",
        })
    }
    return nil
}
```

## 6. Trust-based reputation weighting

Over many rounds, coordinator builds trust edges:

```bash
# After Round 50, coordinator updates trust in bank-a
curl -X POST $NODE/api/trust -d '{
  "truster":"fl-coordinator",
  "trustee":"bank-a",
  "trustLevel":0.92,
  "domain":"ai.federated-learning.fraud-detection",
  "description":"50 rounds, 2 flagged, high-quality gradients",
  "validUntil":<now + 90d>
}'

# A participant with many flagged rounds gets lower trust
curl -X POST $NODE/api/trust -d '{
  "truster":"fl-coordinator",
  "trustee":"bank-outlier-y",
  "trustLevel":0.3,
  "description":"Frequent outliers; quality concerns"
}'
```

In the next round, coordinator's aggregation weights
gradients by trust.

## 7. Credit allocation report

```go
func (c *Coordinator) CreditReport(modelRef string) CreditAllocation {
    rounds := c.GetRoundsForModel(modelRef)
    contributions := map[string]Contribution{}

    for _, round := range rounds {
        events, _ := c.GetEvents(round.Quid, "QUID")
        for _, ev := range events {
            if ev.EventType == "gradient.submitted" {
                p := ev.Payload["participant"].(string)
                ds := int64(ev.Payload["trainingDataSize"].(float64))
                contributions[p] = contributions[p].Add(Contribution{
                    Rounds:    1,
                    DataSize:  ds,
                    Duration:  int64(ev.Payload["trainingDurationSec"].(float64)),
                })
            }
        }
    }
    return normalizeAllocation(contributions)
}
```

Credit allocation drives revenue sharing when the final
model is commercialized.

## 8. Testing

```go
func TestFL_GradientAttestationSigned(t *testing.T) {
    // Bank submits gradient; event stored
    // Verify signature matches bank's current epoch key
}

func TestFL_ByzantineOutlierFlagged(t *testing.T) {
    // 4 normal gradients, 1 adversarial
    // Verify `gradient.flagged` emitted for the outlier
    // Verify `round.aggregated` excludes the outlier
}

func TestFL_CoordinatorDisputeDetectable(t *testing.T) {
    // Coordinator publishes bogus aggregation
    // Participant verifies, emits round.disputed
}
```

## Where to go next

- [`threat-model.md`](threat-model.md)
- [`../ai-model-provenance/`](../ai-model-provenance/)
