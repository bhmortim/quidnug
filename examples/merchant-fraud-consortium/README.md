# Merchant fraud consortium — POC demo

Runnable proof-of-concept for the
[`UseCases/merchant-fraud-consortium/`](../../UseCases/merchant-fraud-consortium/)
use case. Demonstrates the "relational trust" core value
prop end-to-end.

## What this POC proves

Four merchants (acme-retail, bigbox-corp, startup-inc,
noisy-reporter) observe a suspicious card and emit fraud
signals as signed EVENT transactions on a shared trust
domain `fraud.signals.us-retail`. Each consortium member
has their own opinion about every other member's signal
quality, expressed as signed TRUST edges.

When a consumer (observer) wants to know "is this card
fraudulent?" they query the signal stream and weight each
signal by their own relational trust in the reporter. The
aggregate score is observer-specific: two observers with
different trust graphs see different confidence scores for
the same underlying signals.

This is exactly the shape centralized fraud-consortium
services cannot deliver. There's no "is this reporter
trusted" toggle, no central operator holding a global
reputation score. Each consumer owns their own view.

## What's in this folder

| File | Purpose |
|---|---|
| `fraud_weighting.py` | Standalone module: weighted-aggregation math. No SDK dep; pure Python. |
| `fraud_weighting_test.py` | pytest suite exercising the math + observer-relative correctness. 9 tests pass. |
| `demo.py` | End-to-end runnable against a live Quidnug node. |

## Running

```bash
# 1. Start a local node.
cd deploy/compose && docker compose up -d

# 2. Install the Python SDK if not already installed.
cd clients/python && pip install -e .

# 3. Run the demo.
cd examples/merchant-fraud-consortium
python demo.py
```

Expected output: a five-step walk showing merchants being
registered, trust edges published, fraud signals emitted,
and then the same signals producing different aggregate
scores when viewed from different observer merchants.

## Testing without a live node

```bash
cd examples/merchant-fraud-consortium
python -m pytest fraud_weighting_test.py -v
```

Exercises the weighted-aggregation math in-process. No
Quidnug node needed.

## QDP catalog audit (how this use case composes with v1.0)

The POC exercises these protocol features end-to-end:

| Feature | Purpose | Status |
|---|---|---|
| `IDENTITY` tx (QDP-0001) | Register merchants | v1.0 |
| `TRUST` tx (QDP-0001/0002) | Asymmetric trust edges between merchants | v1.0 |
| `EVENT` tx streams | Fraud signals as append-only events | v1.0 |
| Relational trust query (`GET /api/trust/{o}/{t}`) | Weight each signal | v1.0 |
| QDP-0019 decay (opt-in) | Time-weighted trust for fading merchants | Phase 1 landed |
| QDP-0022 ValidUntil | Force periodic re-attestation of trust edges | v1.0 |
| QDP-0016 rate limits | Prevent a single compromised account from flooding | Phase 1 landed |
| QDP-0015 moderation | Takedowns of provably-false signals | Phase 1 landed |

No protocol primitives are missing for this use case. It's
fully covered by the landed v1.0 surface.

## What a production deployment would add

Beyond this POC, a real fraud consortium deployment would
integrate:

- **QDP-0017 DSR support** when personal payment data is
  embedded in signals (card holder identity, etc.). Signals
  should reference off-chain evidence by hash, not by
  plaintext.
- **QDP-0018 audit log** for compliance reporting to
  regulators.
- **Federation (QDP-0013)** between consortium
  sub-networks (e.g., US retail + EU retail as separate
  networks with import-edges where policy allows).
- **Integration with existing risk-scoring engines** (Sift,
  Signifyd, in-house) so the Quidnug-weighted signal feeds
  alongside proprietary features rather than replacing them.

## Related

- Protocol: [QDP-0019 Reputation Decay](../../docs/design/0019-reputation-decay.md)
- Design: [`UseCases/merchant-fraud-consortium/`](../../UseCases/merchant-fraud-consortium/)
- Sibling POC with similar "relational trust" core: [`UseCases/decentralized-credit-reputation/`](../../UseCases/decentralized-credit-reputation/)
