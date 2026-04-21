# POC runbook

Reproducible end-to-end verification of every POC in this
repository against a live Quidnug node. Last executed
2026-04-20 on Windows 11 + Go 1.25 + Python 3.14.

## Status: 16/16 POCs pass against a live node

Every POC in `examples/` has been verified end-to-end: unit
tests pass under pytest, and each `demo.py` runs cleanly
against a freshly-built single-node deployment with a 2-second
block interval. The fixes that surfaced during this sweep are
listed at the end of this document; they are already applied
to the SDK, node, and demos on `main`.

| Layer | Status |
|---|---|
| Go node (`./cmd/quidnug`) | builds clean; full core test sweep green |
| Python SDK (`clients/python`) | 79/79 unit tests green; new `ensure_domain`, `wait_for_identity`, `wait_for_identities`, `wait_for_title` helpers |
| 16 POC unit-test suites | 196/196 tests green |
| 16 POC live demos | 16/16 pass against running node |

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

## Run everything

Smoke test: unit tests for all 16 POCs in one shot.

```bash
cd examples
for d in merchant-fraud-consortium credential-verification-network \
          ai-agent-authorization developer-artifact-signing \
          institutional-custody b2b-invoice-financing \
          interbank-wire-authorization ai-content-authenticity \
          ai-model-provenance defi-oracle-network \
          federated-learning-attestation dns-replacement \
          enterprise-domain-authority decentralized-credit-reputation \
          healthcare-consent-management ; do
  echo "=== $d ==="
  (cd "$d" && python -m pytest -q) || break
done
```

Full end-to-end (requires a running node per the setup above):

```bash
for d in merchant-fraud-consortium credential-verification-network \
          ai-agent-authorization developer-artifact-signing \
          institutional-custody b2b-invoice-financing \
          interbank-wire-authorization ai-content-authenticity \
          ai-model-provenance defi-oracle-network \
          federated-learning-attestation dns-replacement \
          enterprise-domain-authority decentralized-credit-reputation \
          healthcare-consent-management ; do
  echo "=== $d ==="
  (cd examples/$d && python demo.py) || break
done
# Plus the standalone Go POC:
go run ./examples/elections/blind-flow/
```

End-to-end runtime is roughly 8–12 minutes at `BLOCK_INTERVAL=2s`
(dominated by commit waits and the 10-tx-per-minute per-quid rate
limit). Individual demos take 30–120 seconds each.

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
| 9 | ai-model-provenance | 14/14 | pass | model titles jointly owned by producer + safety evaluators + benchmark orgs; derivative base-model trust check relaxed (base isn't a quid) |
| 10 | defi-oracle-network | 10/10 | pass | reporters publish price reports on their own streams with feedQuid in payload; consumers merge across all reporter streams |
| 11 | federated-learning-attestation | 11/11 | pass | each round is a jointly-owned TITLE (coordinator + all participant banks) |
| 12 | elections (blind-flow) | n/a (Go demo) | pass | self-contained Go POC, runs out of the box; no node, no fix needed |
| 13 | dns-replacement | 14/14 | pass | zone is a TITLE jointly owned by governors; rogue cache-poisoning attempt is rejected at node layer (stronger than original resolver-only filter) |
| 14 | enterprise-domain-authority | 13/13 | pass | zone + employees group as governor-owned TITLEs; governor is sole owner so only governor can emit |
| 15 | decentralized-credit-reputation | 11/11 | pass | attesters publish credit events on own streams; reduced event counts to fit per-quid rate limit |
| 16 | healthcare-consent-management | 16/16 | pass | provider access-logs + guardian emergency-overrides route to each actor's own quid stream with patient pointer; resolver merges across ambient actors |

## Demo-authoring recipe (applies to every new POC)

After writing 16 of these, the same pattern works everywhere.
Every new demo that uses the live SDK should follow these
rules:

### 1. Each actor is a QUID. Each shared object is a TITLE.

Quidnug's security model: any event on a QUID stream must be
signed by the quid itself; any event on a TITLE stream must be
signed by one of the title's owners. If your POC has multiple
parties who need to write to a shared log, model that log as a
TITLE with all writers as joint owners. If you tried modelling
a shared object as a plain QUID, the first non-owner write will
fail with `"Signer is not the subject owner"` at server
validation.

Examples of "shared object as TITLE":
- `examples/ai-agent-authorization/` — each action is a TITLE
  jointly owned by agent + all guardians.
- `examples/institutional-custody/` — each transfer is a TITLE
  jointly owned by wallet + all signers.
- `examples/federated-learning-attestation/` — each round is a
  TITLE jointly owned by coordinator + all participants.
- `examples/dns-replacement/` — zone is a TITLE jointly owned
  by all governors.

### 2. Attestations from external parties go on the attester's own stream

When a non-owner of some subject wants to attest something
about the subject (a researcher reporting a CVE, a lender
reporting a payment, a utility reporting an on-time payment),
emit the event on the attester's OWN quid stream with the
target subject in the payload. Verifiers subscribe to a
curated list of attester streams and cross-reference.

Examples:
- `examples/developer-artifact-signing/` — researcher publishes
  CVE reports on the researcher's own stream, with
  `targetReleaseId` in the payload.
- `examples/decentralized-credit-reputation/` — every lender
  and utility publishes credit events on their own stream, with
  `subject` in the payload.
- `examples/defi-oracle-network/` — every reporter publishes
  prices on its own stream, with `feedQuid` in the payload.
- `examples/healthcare-consent-management/` — providers log
  accesses, guardians log emergency overrides, both on their
  own streams with `patient` in the payload.

### 3. Template for the top of `main()`

Every demo follows this opening sequence:

```python
def main() -> None:
    client = QuidnugClient(NODE_URL)
    try:
        client.info()
    except Exception as e:
        print(f"node unreachable: {e}", file=sys.stderr)
        sys.exit(1)

    # A. Register the trust domain(s) the demo operates in.
    client.ensure_domain(DOMAIN)

    # B. Register every actor identity, then block until they
    #    commit so follow-on tx can reference them.
    alice = register(client, "alice", "role")
    bob   = register(client, "bob",   "role")
    # ...
    client.wait_for_identities([alice.quid.id, bob.quid.id, ...])

    # C. Register any shared TITLEs, then block until commit.
    client.register_title(
        signer=alice.quid, asset_id="shared-id",
        owners=[
            OwnershipStake(alice.quid.id, 0.7, "primary"),
            OwnershipStake(bob.quid.id,   0.3, "secondary"),
        ],
        domain=DOMAIN, title_type="some-type",
    )
    client.wait_for_title("shared-id")

    # D. Now emit events. Sleep ~3s between rapid bursts to stay
    #    under the 10-tx-per-minute per-quid rate limit and to
    #    allow batched events to commit before the next step
    #    reads the stream.
    ...
```

### 4. Common gotchas (and their symptoms)

| Symptom | Cause | Fix |
|---|---|---|
| `invalid identity transaction` / `unknown trust domain` | domain not registered | `client.ensure_domain(DOMAIN)` at top of main |
| `invalid trust transaction` / `subject QUID not found` | referenced quid not yet committed | `client.wait_for_identities([...])` after bootstrap |
| `invalid event transaction` / `Subject TITLE not found` | title not yet committed | `client.wait_for_title(asset_id)` after register_title |
| `invalid event transaction` / `Invalid signature` | non-alphabetical nested-dict keys in payload | Python SDK now sorts automatically; use latest SDK |
| `invalid event transaction` / `Signer is not the subject owner` | wrong authorship model | model shared object as TITLE with signer as owner, or route event to signer's own stream |
| `rate limit exceeded at quid layer` | >10 tx in 60s from one quid | sleep between bursts, or reduce loop counts |
| empty stream reads, missing events | read races commit | `time.sleep(3)` between emit and read, or use wait helpers |

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
