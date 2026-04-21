# POC runbook

Reproducible end-to-end verification of every POC in this
repository against a live Quidnug node. Last executed
2026-04-20 on Windows 11 + Go 1.25 + Python 3.14.

## One-time environment setup

### Option A: Build and run the node from source (recommended)

```bash
# From the repo root.
go build -o bin/quidnug ./cmd/quidnug

# Short block interval so demos see their own transactions
# commit within seconds rather than the production default of
# 60 seconds.
BLOCK_INTERVAL=2s ./bin/quidnug &

# Wait a second, then sanity-check:
curl http://localhost:8080/api/health
```

Expected:

```json
{"data":{"node_id":"...","status":"ok","uptime":1,"version":"1.0.0"},"success":true}
```

The node listens on `:8080` by default.

**Why the short block interval?** A brand-new identity
transaction lives in the pending pool until the next block is
sealed. Any follow-on event or title transaction that
references the new quid requires the quid to be in the
committed registry. The demos call
`client.wait_for_identities([...])` after bootstrapping actors
so they block until commit; with the default 60-second
interval that wait would dominate the demo's runtime.

### Option B: Docker compose (three-node consortium)

```bash
cd deploy/compose && docker compose up -d
curl http://localhost:8081/api/health   # n1
```

If you use compose, set `QUIDNUG_NODE=http://localhost:8081`
before running the demos below.

### Install the Python SDK

```bash
cd clients/python && pip install -e .
```

Verify:

```bash
pip show quidnug
# should print: Name: quidnug  Version: 2.0.0
```

## How each POC is structured

Every POC follows the same three-file layout:

| File | Role |
|---|---|
| `<name>.py` | Pure decision logic. No SDK dep. Exercisable in pytest. |
| `<name>_test.py` | Unit tests for the decision logic. `pytest` only. |
| `demo.py` | End-to-end flow against a live node. SDK calls. |

There are two tiers of reproduction:

1. **No node required** -- just the unit tests:
   ```bash
   cd examples/<poc-name>
   python -m pytest -v
   ```
   This verifies that the decision logic is correct
   independently of the network.

2. **Full end-to-end** -- unit tests plus live demo against
   a running node:
   ```bash
   cd examples/<poc-name>
   python -m pytest -v
   python demo.py
   ```
   This verifies the complete flow: SDK signing, node
   acceptance, event-stream retrieval, decision-layer
   interpretation.

## Execution log

Each POC below has a result row filled in after running. The
format:

- **status**: `pass` / `fail` / `skipped`
- **unit tests**: `<n>/<n>` pass count
- **demo**: `pass` / `fail` with a link to the fix commit if
  any issues were found during execution.

| # | Use case | Unit tests | Demo | Notes |
|---|---|---|---|---|
| 1 | merchant-fraud-consortium | 9/9 | pass | required 4 fixes: ensure_domain + wait_for_identities + Go-compat payload-key sort + short BLOCK_INTERVAL |
| 2 | credential-verification-network | 10/10 | pass | required node fixes: ownership sum 1.0 (not 100.0), free-form asset IDs, TITLE subject IDs free-form; demo refactored to issue credentials as jointly-owned TITLEs |
| 3 | ai-agent-authorization | 17/17 | pass | demo refactored: each action is a jointly-owned TITLE (agent primary + guardians); cosign / veto events on the action's title stream |
| 4 | developer-artifact-signing | 16/16 | pass | CVE reports moved to researcher's own QUID stream with targetReleaseId in payload; verifier merges release's title stream with trusted researchers' streams |
| 5 | institutional-custody | 14/14 | pass | transfers as jointly-owned TITLEs (wallet + proposer + all 7 signers); wallet policy / freeze events on ops officer's own stream with target-wallet pointer |
| 6 | b2b-invoice-financing | 15/15 | pass | invoice jointly owned by supplier + buyer + carrier + financiers; longer 3s inter-step sleeps wait for block commit |
| 7 | interbank-wire-authorization | 14/14 | pass | wire titles jointly owned by sender bank + all cosigning officers |
| 8 | ai-content-authenticity | 12/12 | pass | media titles jointly owned by camera + photographer + editors + publishers + fact-checkers |
| 9 | ai-model-provenance | _pending_ | _pending_ | |
| 10 | defi-oracle-network | _pending_ | _pending_ | |
| 11 | federated-learning-attestation | _pending_ | _pending_ | |
| 12 | elections (blind-flow) | n/a (Go demo) | _pending_ | |
| 13 | dns-replacement | _pending_ | _pending_ | |
| 14 | enterprise-domain-authority | _pending_ | _pending_ | |
| 15 | decentralized-credit-reputation | _pending_ | _pending_ | |
| 16 | healthcare-consent-management | _pending_ | _pending_ | |

## Fixes applied during this execution sweep

Three issues surfaced while running POCs against a live node
for the first time:

### 1. Trust domains must be pre-registered

Identity, trust, title, and event transactions all reject the
trust domain unless the domain has been registered with the
node first. Only `default` is pre-registered. The SDK gained
an idempotent `client.ensure_domain(domain)` helper and every
demo now calls it once after the node health check.

### 2. Identity registration is asynchronous

`client.register_identity()` queues the transaction in the
pending pool; the identity becomes visible in the committed
registry only after the next block is sealed. Follow-on trust
/ event / title transactions that reference the new quid fail
with "subject QUID not found" until commit. The SDK gained
`client.wait_for_identity(quid_id)` and
`client.wait_for_identities([...])` helpers. Each demo now
waits for all its actors to commit before issuing trust edges
or events.

### 3. Nested payload keys need alphabetical ordering

A transaction's signature is computed over its canonical byte
representation. Struct-typed top-level fields are ordered by
Go declaration order (preserved via each wire-type's explicit
field-tuple list). Nested `map[string]interface{}` payloads,
however, are re-marshaled server-side via Go's
`encoding/json`, which sorts map keys alphabetically. The
Python SDK's `_go_compat_value` helper had to be updated to
alphabetically sort nested dict keys to match. Without this
fix, any event whose payload had more than one key in
non-alphabetical insertion order would fail signature
verification.

The same bug exists in the JS and Rust SDKs and will need the
same fix there before they can be used against a live node
with multi-key payloads.

### 4. Title ownership shares use 1.0-scale fractions

Server `ValidateTitleTransaction` required ownership
percentages to sum to `100.0`, but the v1.0 spec's test
vectors encode shares as `1` (fractions summing to 1.0), and
the Python SDK normalizes to fractions before signing.
Updated the server to accept either form (fractions summing
to 1.0 or legacy percentages summing to 100.0, within a 1e-6
tolerance).

### 5. Title AssetID is a free-form asset identifier

Server required AssetID to be a 16-hex quid and pre-existing
in the identity registry, but the spec's test vectors use
free-form identifiers like `asset-sku-000001`. Relaxed the
check so AssetID accepts any non-empty printable string up to
256 chars. The `every listed owner must exist in the identity
registry` invariant is retained (and enforced for every owner,
not just one asset).

### 6. Event SubjectID format depends on SubjectType

Server rejected event transactions whose `subjectId` wasn't a
16-hex quid. For `subjectType=QUID` that's right; for
`subjectType=TITLE` the subject is the title's AssetID, which
is free-form. Split the check by subject type.

### 7. Title transactions commit asynchronously

Same as identity: titles land in the pending pool until the
next block is sealed, and events on the title fail with
"Subject TITLE not found" until commit. Added
`client.wait_for_title(asset_id)` analogous to
`wait_for_identity`.
