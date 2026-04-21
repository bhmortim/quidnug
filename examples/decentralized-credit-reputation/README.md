# Decentralized credit reputation, POC demo

Runnable proof-of-concept for the
[`UseCases/decentralized-credit-reputation/`](../../UseCases/decentralized-credit-reputation/)
use case. Demonstrates per-lender credit evaluation on the
subject's signed event stream, with no universal score, with
subject-owned dispute handling, and with alt-data as a
first-class signal.

## What this POC proves

A subject, multiple lenders, a utility attester, a fraudulent
lender, and an alt-bank all sharing a credit-reports domain.
Key claims:

1. **No universal score.** Each lender runs
   `evaluate_borrower` on the subject's event stream with its
   own trust graph and policy. Different lenders reach
   different verdicts on the same subject.
2. **Untrusted attesters are filtered out.** A default filed by
   a lender the verifier doesn't trust above the floor is
   ignored, so a rogue party can't tank someone's access to
   credit network-wide.
3. **Subject-filed disputes discount negative events.** The
   subject's signed `credit.dispute` event on their own stream
   neutralizes a malicious default (while leaving a record that
   the dispute was filed).
4. **Alt-data lifts thin-file subjects.** Utility on-time
   payments contribute to the score at a scaled weight, giving
   subjects with no conventional credit history a path to
   access.
5. **Category filter lets policy scope the evaluation.** A
   mortgage lender can opt to consider only mortgage events,
   or to blend auto + rent + mortgage history.

## What's in this folder

| File | Purpose |
|---|---|
| `credit_evaluate.py` | Pure decision logic: `CreditEvent`, `Dispute`, `LenderPolicy`, `evaluate_borrower`, stream extractors. |
| `credit_evaluate_test.py` | 11 pytest cases: clean history, default, disputed default, untrusted attester, observer-relative verdicts, alt-data lift, category filter, subject filter, extraction. |
| `demo.py` | End-to-end runnable against a live node. Nine steps: register actors, build trust graphs, record clean history + alt-data + false default, file dispute, three lenders evaluate independently. |

## Running

```bash
# 1. Start a local node.
cd deploy/compose && docker compose up -d

# 2. Install Python SDK.
cd clients/python && pip install -e .

# 3. Run the demo.
cd examples/decentralized-credit-reputation
python demo.py
```

## Testing without a live node

```bash
cd examples/decentralized-credit-reputation
python -m pytest credit_evaluate_test.py -v
```

## QDP catalog audit

| Feature | Purpose | Status |
|---|---|---|
| `IDENTITY` tx | Register subject, lenders, utility, arbiter | v1.0 |
| `TRUST` tx | Per-lender trust in each attester | v1.0 |
| `EVENT` tx streams | Credit events + disputes on the subject's stream | v1.0 |
| QDP-0019 decay | Old events fade; recent behavior weighs more | Phase 1 landed; optional for this POC |
| QDP-0024 group encryption | Selectively disclose stream to a specific lender | Phase 1 landed; POC leaves stream public for demo simplicity |

No protocol gaps.

## What a production deployment would add

- **Encrypted stream + selective disclosure.** The POC runs
  with public event payloads. A production deployment would
  encrypt the payloads with QDP-0024 and let the subject share
  the group key with specific lenders on application.
- **Time decay.** A default five years ago matters less than
  one last month. QDP-0019 provides the primitive; the policy
  would apply a half-life.
- **KYC verifier integration.** An identity verifier (DMV,
  passport authority, KYC provider) binds the subject quid to
  a real-world identity via a signed event.
- **Dispute arbitration.** A third-party arbiter (CFPB-style)
  co-signs the dispute event with a ruling. The POC treats
  a subject-signed dispute as authoritative; production would
  require an arbitration step.
- **Regulatory observability.** A regulator quid has a trust
  edge from the network root but cannot itself alter scores
  or aggregate a universal number. They can audit the public
  event chain.

## Related

- Use case: [`UseCases/decentralized-credit-reputation/`](../../UseCases/decentralized-credit-reputation/)
- Related POC: [`examples/merchant-fraud-consortium/`](../merchant-fraud-consortium/)
  applies the same relational-trust aggregation to merchant
  fraud signals
- Related POC: [`examples/credential-verification-network/`](../credential-verification-network/)
  has the same observer-relative verdict property for credentials
- Related POC: [`examples/b2b-invoice-financing/`](../b2b-invoice-financing/)
  uses relational trust for counterparty creditworthiness in
  the B2B context
