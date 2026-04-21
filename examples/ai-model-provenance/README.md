# AI model provenance, POC demo

Runnable proof-of-concept for the
[`UseCases/ai-model-provenance/`](../../UseCases/ai-model-provenance/)
use case. Demonstrates a signed model + dataset + evaluation
graph that a downstream enterprise consumer can verify before
deploying a model.

## What this POC proves

Dataset curator, model producer, fine-tune producer, safety
evaluator, benchmark organization, and an enterprise consumer
on shared provenance domains. Key claims the demo verifies:

1. **Models are cryptographically tied to their training data.**
   The model's event stream references each dataset by ID; the
   dataset's own title declares its license and hash. A
   prohibited-license dataset in the training set is caught
   at verification time.
2. **Safety evaluations are independent, signed events.** A
   trusted safety-org's event carries the rating; a failing
   rating from a trusted evaluator is a hard reject under the
   default strict policy; absence of any acceptable eval is
   a warn.
3. **Derivatives inherit provenance through a base-model
   trust check.** A fine-tune producer's reputation alone is
   not enough -- the base model's trust must also clear the
   threshold, or the derivative is rejected.
4. **Consumers set policy.** Prohibited dataset-license list,
   safety strictness, benchmark requirement, trust thresholds
   are all local policy. The same on-chain chain can be
   accepted by one enterprise and rejected by another.

## What's in this folder

| File | Purpose |
|---|---|
| `model_provenance.py` | Pure decision logic: `ModelV1`, `DatasetV1`, `ModelPolicy`, `evaluate_model_provenance`, stream extractors. |
| `model_provenance_test.py` | 14 pytest cases covering foundation / derivative accept, producer-trust gating, base-model gating, dataset-license filter, missing-metadata, safety modes, benchmark-required variants. |
| `demo.py` | End-to-end runnable against a live node. Eight steps registering the full graph and walking three scenarios (accept foundation, accept derivative, reject prohibited dataset, warn on missing safety). |

## Running

```bash
# 1. Start a local node.
cd deploy/compose && docker compose up -d

# 2. Install Python SDK.
cd clients/python && pip install -e .

# 3. Run the demo.
cd examples/ai-model-provenance
python demo.py
```

## Testing without a live node

```bash
cd examples/ai-model-provenance
python -m pytest model_provenance_test.py -v
```

## QDP catalog audit

| Feature | Purpose | Status |
|---|---|---|
| `IDENTITY` tx | Register curator, producer, evaluator, benchmark org | v1.0 |
| `TITLE` tx | Datasets and models as on-chain assets | v1.0 |
| `TRUST` tx | Per-consumer relational trust in producer, evaluator, benchmarker | v1.0 |
| `EVENT` tx streams | training.started/completed, safety.evaluated, benchmark.reported | v1.0 |
| QDP-0002 guardian recovery | Producer key recovery | v1.0 (not exercised) |
| Domain hierarchy | `ai.provenance.*` per-sub-domain scoping | v1.0 |

No protocol gaps.

## What a production deployment would add

- **Weight-hash verification.** The consumer downloads the
  model weights and checks the sha256 matches the
  on-chain-declared `modelHash`. The POC skips this check for
  demo simplicity but a production verifier would never.
- **Training-run attestation via TEE.** A training run inside
  a trusted-execution environment emits an attestation that
  binds the claimed training-data refs to the actual data fed
  in. Without this, producers can still lie about their
  training set on-chain.
- **Rights-holder objection events.** A rights holder whose
  content was scraped can post a `data.license-objection`
  event on the relevant dataset's stream; the consumer policy
  can treat an unresolved objection as a reject.
- **Composition with [`ai-content-authenticity`](../ai-content-authenticity/)**.
  An AI-generated asset's `media.generated` event references
  the model quid; the model's provenance chain flows through
  as part of the asset's authenticity evaluation.
- **Lineage queries.** A "show me every fine-tune derived from
  model X" query drives governance on derivatives of a
  restricted base model.

## Related

- Use case: [`UseCases/ai-model-provenance/`](../../UseCases/ai-model-provenance/)
- Related POC: [`examples/ai-content-authenticity/`](../ai-content-authenticity/)
  consumes this chain when evaluating AI-generated media
- Related POC: [`examples/developer-artifact-signing/`](../developer-artifact-signing/)
  is the same supply-chain pattern for conventional software
- Related POC: [`examples/federated-learning-attestation/`](../federated-learning-attestation/)
  (upcoming) covers the distributed-training-round variant
