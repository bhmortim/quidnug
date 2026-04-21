# AI content authenticity, POC demo

Runnable proof-of-concept for the
[`UseCases/ai-content-authenticity/`](../../UseCases/ai-content-authenticity/)
use case. Demonstrates C2PA-plus provenance chains: every step
from camera capture through edit through publication through
fact-check is a signed event on the asset's event stream, and
consumers evaluate authenticity with per-consumer relational
trust.

## What this POC proves

Eight actors (camera, photographer, editor, publisher,
fact-checker, AI generator, aggregator, consumer newsroom) on a
shared `media.provenance.news` domain. Key claims the demo
verifies:

1. **Full capture-to-publication chain is verifiable.** An
   authentic news photo walks through camera -> photographer
   (crop) -> editor (grade) -> publisher -> fact-checker on a
   single event stream. Consumer aggregates the signed events
   and reaches an `accept` verdict.
2. **Hash continuity catches tampering.** An edit event whose
   input_hash doesn't match the prior output_hash triggers a
   hard reject regardless of trust scores.
3. **The weakest editor taints the chain.** Trust is the
   minimum across capture, edits, and publisher. A single
   low-trust editor pulls the overall score below threshold
   even if everyone else is top-tier.
4. **AI-generated content is first-class but policy-gated.**
   A `media.generated` event replaces `media.captured`. The
   consumer policy decides whether AI content gets `accept`,
   `warn`, or `reject` without changing the underlying stream.
5. **Fact-check bonus is optional.** A trusted fact-checker's
   endorsement event adds a configurable bonus to the overall
   score.

## What's in this folder

| File | Purpose |
|---|---|
| `content_authenticity.py` | Pure decision logic: `MediaAssetV1`, `ProvenanceEvent`, `AuthenticityPolicy`, `evaluate_authenticity`, hash-chain check, stream extractor. |
| `content_authenticity_test.py` | 12 pytest cases: happy path, fact-check bonus, weak-link taint, hash-chain tamper, AI warn/accept/reject, publisher required, trust range. |
| `demo.py` | End-to-end runnable against a live node. Three scenarios (authentic, tampered, AI-generated) with three policy variants for the AI case. |

## Running

```bash
# 1. Start a local node.
cd deploy/compose && docker compose up -d

# 2. Install Python SDK.
cd clients/python && pip install -e .

# 3. Run the demo.
cd examples/ai-content-authenticity
python demo.py
```

## Testing without a live node

```bash
cd examples/ai-content-authenticity
python -m pytest content_authenticity_test.py -v
```

## QDP catalog audit

| Feature | Purpose | Status |
|---|---|---|
| `IDENTITY` tx | Register camera / photographer / editor / publisher / fact-checker / model | v1.0 |
| `TITLE` tx | The asset itself, owned by its creator | v1.0 |
| `TRUST` tx | Per-consumer relational trust in each provenance party | v1.0 |
| `EVENT` tx streams | The edit chain: captured/generated, cropped, graded, published, fact-checked | v1.0 |
| QDP-0002 guardian recovery | Camera key fails -> manufacturer rotates device's quid | v1.0 (not exercised) |
| QDP-0005 push gossip | Rapid propagation of revocation | v1.0 |
| QDP-0019 decay | Old published assets fade naturally | Phase 1 landed; optional |
| AI-model-provenance composition | The generator quid has its own provenance chain | composes with POC #9 |

No protocol gaps.

## What a production deployment would add

- **C2PA manifest bridge.** Extract C2PA metadata from the
  media file itself and cross-reference it to the on-chain
  chain. A producer-side tool would write both the C2PA
  manifest and the Quidnug events in one ceremony.
- **Camera hardware keys.** Each DSLR / smartphone with C2PA
  hardware gets a per-device quid, with the manufacturer as
  the guardian. The POC simulates this with a software keypair.
- **Automated transform detection.** A consumer-side verifier
  that actually runs the claimed edit transforms (crop rect,
  color adjustments) on the pre-image and confirms the
  output_hash matches. Rejects any event whose claimed
  transform doesn't match the bits.
- **Integration with [`ai-model-provenance`](../ai-model-provenance/)**.
  When an AI-generated asset is evaluated, also pull the
  model's own provenance chain (training data, weights
  version, safety evals) and factor into the score.

## Related

- Use case: [`UseCases/ai-content-authenticity/`](../../UseCases/ai-content-authenticity/)
- Related POC: [`examples/ai-model-provenance/`](../ai-model-provenance/)
  (upcoming) covers the generator model's own provenance
- Related POC: [`examples/credential-verification-network/`](../credential-verification-network/)
  is the same min-of-chain + trust pattern in a different domain
- Protocol: C2PA standard (https://c2pa.org/) for the industry
  context this POC aims to interoperate with
