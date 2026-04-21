# Federated learning attestation, POC demo

Runnable proof-of-concept for the
[`UseCases/federated-learning-attestation/`](../../UseCases/federated-learning-attestation/)
use case. Demonstrates a signed FL-round event stream that any
participant or auditor can replay to verify contribution,
aggregation, and anomaly flags.

## What this POC proves

A coordinator, five participant banks, and an auditor on a
shared FL domain. Each round is its own quid; the round's event
stream carries registration, gradient submission, and
aggregation events. Key claims:

1. **Proof-of-contribution is signed.** Each participant's
   `gradient.submitted` event is signed by the participant's
   quid. A bank cannot later deny having contributed; the
   coordinator cannot fabricate a contribution.
2. **Coordinator honesty is auditable.** `round.aggregated`
   carries the coordinator's declared participant weights. A
   separate `fair_weights_by_data_size` helper computes what
   the distribution *should* look like based on attested data
   sizes, letting auditors detect coordinator bias.
3. **Suspicious gradients are flagged but not disqualifying.**
   A gradient with norm more than 5x the round's median norm
   is surfaced in the audit without invalidating the round.
4. **Round policy is consumer-configurable.** Minimum
   participant count, strict-vs-loose registration, and the
   aggregation-required flag are all knobs.
5. **Insufficient participation or missing aggregation
   produce explicit verdicts** (`insufficient`, `incomplete`,
   `integrity-violation`).

## What's in this folder

| File | Purpose |
|---|---|
| `fl_audit.py` | Pure audit logic. `Registration`, `GradientSubmission`, `Aggregation`, `RoundPolicy`, `audit_round`, `fair_weights_by_data_size`. |
| `fl_audit_test.py` | 11 pytest cases: valid round, insufficient, strict/loose registration, suspicious gradient flag, missing aggregation, fair-weights math, extraction. |
| `demo.py` | End-to-end runnable against a live node. Five steps: register, run four rounds with different failure modes (valid / insufficient / suspicious / integrity-violation). |

## Running

```bash
# 1. Start a local node.
cd deploy/compose && docker compose up -d

# 2. Install Python SDK.
cd clients/python && pip install -e .

# 3. Run the demo.
cd examples/federated-learning-attestation
python demo.py
```

## Testing without a live node

```bash
cd examples/federated-learning-attestation
python -m pytest fl_audit_test.py -v
```

## QDP catalog audit

| Feature | Purpose | Status |
|---|---|---|
| `IDENTITY` tx | Register coordinator, participants, round quid, auditor | v1.0 |
| `EVENT` tx streams | round.opened, participant.registered, gradient.submitted, round.aggregated | v1.0 |
| QDP-0005 push gossip | Fast propagation of events across participant nodes | v1.0 |
| QDP-0002 guardian recovery | Participant's signing key recovery | v1.0 (not exercised) |
| QDP-0024 group encryption | Gradients encrypted to the aggregation committee | Phase 1 landed; optional for this POC |

No protocol gaps. The POC leaves gradient content as an opaque
CID reference (payload is a hash and a content-addressed pointer);
the actual gradient bytes live off-chain.

## What a production deployment would add

- **Secure aggregation integration.** Real FL deployments use
  secure-aggregation protocols to hide individual gradients
  from the coordinator while still allowing the sum. QDP-0024
  group-keyed encryption provides the building block; this POC
  leaves the encryption step to the caller.
- **TEE attestation.** A participant running the training in a
  trusted-execution environment can include the enclave
  attestation on the `gradient.submitted` event. Consumers
  then gate on "attestation present and verifying."
- **Poisoning defense.** Beyond norm-threshold flagging, a
  production auditor would run multi-round statistical
  analysis (Krum, trimmed mean) and compare against the
  reported participant weights.
- **Incentive / rewards.** A cross-round reputation stream per
  participant (never-missed-a-round, consistent data-size) can
  drive payout distributions from the round.

## Related

- Use case: [`UseCases/federated-learning-attestation/`](../../UseCases/federated-learning-attestation/)
- Related POC: [`examples/ai-model-provenance/`](../ai-model-provenance/)
  covers the supply chain for the final trained model; this
  POC covers how the training rounds that produced it are
  attested
- Related POC: [`examples/institutional-custody/`](../institutional-custody/)
  shares the pattern of per-round event streams with coordinator
  aggregation, in a financial context
