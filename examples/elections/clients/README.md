# Reference client implementations for elections

Five runnable Python clients demonstrating the end-to-end
election flow from
[`UseCases/elections/implementation.md`](../../../UseCases/elections/implementation.md).
One per actor role; shared crypto / HTTP / type helpers
under [`common/`](common/).

| Client | Role | What it demonstrates |
|---|---|---|
| [`voter.py`](voter.py) | Voter | Generate VRQ, register, request + unblind ballot, cast votes, verify own vote |
| [`poll_worker.py`](poll_worker.py) | Poll worker | Look up registration, check in voter, publish check-in event |
| [`observer.py`](observer.py) | Observer | Stream events, publish procedural attestations, flag anomalies |
| [`tally.py`](tally.py) | Tally engine | Walk vote stream, verify signatures + ballot proofs, emit contest totals |
| [`audit.py`](audit.py) | Auditor / recount | Pull full chain, compare with paper ballots, produce public recount report |

## Prerequisites

Python 3.10+. Dependencies:

```bash
pip install cryptography requests pyyaml
```

All five clients use Python's standard `cryptography` library
for ECDSA P-256 signing, RSA-3072 blind-signature primitives
(per QDP-0021), and SHA-256 hashing. No bespoke crypto.

## Running against a local test election

The clients are designed to run against a live Quidnug node
with the election authority's domain tree registered. For a
smoke test:

```bash
# 1. Start a local Quidnug node (from the repo root):
CONFIG_FILE=examples/elections/clients/test-config.yaml \
  quidnug > /tmp/quidnug.log 2>&1 &

# 2. Authority sets up the election + publishes blind-key
#    attestation:
python setup_authority.py \
  --election williamson-2026-nov \
  --out-dir ./authority-state/

# 3. A few voters register:
for i in 1 2 3; do
  python voter.py register \
    --name "Voter$i" \
    --precinct "precinct-042" \
    --election williamson-2026-nov \
    --out voter-$i.keys
done

# 4. Poll workers check voters in on election day:
python poll_worker.py check-in \
  --vrq voter-1.keys \
  --precinct precinct-042 \
  --election williamson-2026-nov

# 5. Voters request ballots + cast votes:
python voter.py vote \
  --keys voter-1.keys \
  --election williamson-2026-nov \
  --choices governor=nicosia senate=hartwell

# 6. Tally engine runs the count:
python tally.py \
  --election williamson-2026-nov \
  --contest governor

# 7. Any observer can re-run tally for themselves:
python observer.py recount \
  --election williamson-2026-nov \
  --contest governor

# 8. Post-election audit verifies paper-vs-digital:
python audit.py \
  --election williamson-2026-nov \
  --paper-ballots ./paper-sample.csv
```

## What these clients mock (for now)

QDP-0012 / QDP-0014 / QDP-0021 implementations are partially
in-flight. For the parts that aren't shipped yet, the clients
include stub implementations with clear `# MOCK:` comments
flagging what will change when the underlying protocol
support lands.

Specifically:

- **Blind-signature flow** (QDP-0021, Draft) — the voter app
  and authority's signing endpoint use Python's `cryptography`
  to do real RSA-FDH blind signing. This works end-to-end
  against a local mock of the authority's signing service.
  Once QDP-0021 lands in the node, swap the mock endpoint for
  the real `/api/v2/elections/<id>/ballot-request` endpoint.
- **Governance transactions** (QDP-0012, Draft) — the setup
  script mocks governor quorum signing with a single-key
  fallback. Once QDP-0012 lands, replace the stub with the
  real quorum-collection flow.
- **Well-known file signing** (QDP-0014, **Landed**) — the
  setup script uses the landed `NODE_ADVERTISEMENT` +
  `.well-known/quidnug-network.json` primitives directly.

## Design intent

These are not production-hardened. They're **reference
implementations** — the minimum-viable code that exercises
every protocol flow described in the use case. Production
deployments would:

- Wrap the voter app in a mobile/web UI with proper UX.
- Run the poll-worker logic inside a dedicated iPad/Surface
  app with secure-enclave key storage.
- Run the tally engine as a signed, audited service.
- Run the audit CLI as a standalone artifact distributed to
  campaigns, observer orgs, and journalists.

But the *protocol flows* themselves are exactly what these
scripts do. Treat them as executable specifications of the
election use case.

## Shared library

All five clients depend on [`common/`](common/):

- [`common/crypto.py`](common/crypto.py) — ECDSA P-256
  key gen + signing + verify; RSA-3072 blind-signature
  blinding + unblinding per QDP-0021.
- [`common/http_client.py`](common/http_client.py) — HTTP
  client wrapping the Quidnug node's API (discovery,
  streams, transaction submission).
- [`common/types.py`](common/types.py) — data classes for
  events, transactions, quids, ballot proofs.
- [`common/config.py`](common/config.py) — YAML config
  loader with schema validation.

Tests for the shared library live under `tests/` and run
with `pytest`.

## Next steps

When QDP-0021 lands in the node:

1. Remove the `# MOCK:` authority-blind-signing service in
   `setup_authority.py` + the voter's mock path.
2. Replace with real calls to the node's blind-issuance
   endpoint.
3. Regenerate test vectors against the real flow.

When QDP-0012 implementation lands:

1. Replace single-key governance-action stubs in
   `setup_authority.py` with the real quorum-collection
   flow.
2. Add mid-cycle governance-change demos to
   `observer.py`.

These changes are scoped and small — the bulk of the code
(crypto primitives, HTTP client, event shapes, flow
orchestration) doesn't change.
