# Threat Model: Federated Learning Attestation

## Assets

1. **Gradient-contribution integrity** — the record of what
   each participant submitted.
2. **Trained model quality** — prevented from being poisoned
   by Byzantine gradients.
3. **Credit allocation fairness** — revenue sharing based on
   contributions.
4. **Participant data privacy** — raw data stays local.

## Attackers

| Attacker               | Capability                              | Goal                          |
|------------------------|-----------------------------------------|-------------------------------|
| Compromised participant| Valid key of a participant              | Submit biased/adversarial gradients |
| Malicious coordinator  | Aggregation authority                   | Bias model, misreport         |
| Free-rider             | Legitimate participant, low effort      | Claim credit without training |
| External              | Network observer                        | Infer data from gradients     |

## Threats

### T1. Byzantine gradients
**Attack.** Participant submits crafted gradient to degrade
global model.
**Mitigation.** Outlier detection on coordinator side;
`gradient.flagged` events documented in stream. Repeated
flagging lowers participant's trust and their effective
weight in future rounds.

### T2. Malicious coordinator bias
**Attack.** Coordinator publishes aggregation that favors
certain participants.
**Mitigation.** Every participant can independently re-run
aggregation and emit `round.disputed` if results differ.
Next round, participants may vote to replace coordinator.

### T3. Free-rider
**Attack.** Participant submits minimal or zero-effort
gradient to claim credit.
**Mitigation.** Attested training metadata (data size,
duration) + coordinator validation. Gradients of abnormally
small effect detected as outliers.

### T4. Participant replay
**Attack.** Participant re-submits a prior round's gradient.
**Mitigation.** Event is bound to the round's quid; replay
in a different round has wrong `subjectId`. Anchor nonce +
event dedup.

### T5. Gradient inversion attack
**Attack.** Observer with gradient access reconstructs
original training data.
**Mitigation.** **Protocol doesn't prevent this**;
participants should apply differential privacy noise or
secure aggregation at the application layer. Quidnug records
metadata including DP noise scale as attestation.

### T6. Coordinator compromise
**Attack.** Coordinator's signing key stolen.
**Mitigation.** Guardian recovery. Post-rotation, old
coordinator signatures rejected.

### T7. Round-hijack
**Attack.** Attacker publishes fake `round.opened` event
impersonating coordinator.
**Mitigation.** Event signed by coordinator's key;
participants verify before engaging. Forged signatures fail.

## Not defended against

- **Privacy of raw data** — that's local to each participant.
  Quidnug doesn't see it.
- **Correctness of participant's local training** — if
  participant claims to train on X but trains on Y, we only
  catch it if gradient behavior is anomalous.
- **Deep adversarial ML attacks** (hidden triggers,
  backdoors) — detection is an active research area.

## References
- [QDP-0005 Push Gossip](../../docs/design/0005-push-based-gossip.md)
- [`../ai-model-provenance/threat-model.md`](../ai-model-provenance/threat-model.md)
