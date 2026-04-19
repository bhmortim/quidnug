# End-to-end reviews demo (live Quidnug node)

A working demonstration of **trust-weighted per-observer review
ratings** against a running reference Quidnug node. Same five
reviews, different observers, genuinely different effective
ratings.

## What it proves

| Invariant | How it's proven |
|---|---|
| All four tx types (IDENTITY / TITLE / TRUST / EVENT) flow end-to-end through HTTP | `demo.py` posts each one and asserts success |
| Reviews, helpfulness votes, and activity all land in the correct event streams | `_wait_for_stream_seq` blocks until commit, then reads back |
| Different observers compute different effective ratings from the same raw reviews | `effective_rating` called per-observer; `index.html` renders the divergence |
| Signatures are byte-compatible with the Go node's `json.Marshal` + IEEE-1363 + SEC1 conventions | The Go helper at `sign_helper/main.go` signs every tx, exercised 30+ times in one run |
| The same flow works without HTTP or Python | `internal/core/reviews_integration_test.go` runs it all in-process |

## Prerequisites

- **Go 1.21+** — builds both the node and the signing helper.
- **Python 3.10+** with `requests` + `cryptography` (from
  `clients/python/pyproject.toml`).
- The Quidnug node binary. Build from repo root:

  ```bash
  go build -o /tmp/quidnug ./cmd/quidnug
  ```

- The signing helper (byte-compatible ECDSA P-256 signer). Build:

  ```bash
  go build \
      -o examples/reviews-and-comments/demo/sign_helper/sign-helper \
      ./examples/reviews-and-comments/demo/sign_helper
  ```

  On Windows, output `sign-helper.exe` instead. The demo auto-selects
  the right extension.

## Running it

```bash
# 1. Start the node with the demo config (fast blocks, open domain registration)
CONFIG_FILE=examples/reviews-and-comments/demo-config.yaml /tmp/quidnug &

# 2. Run the scenario
PYTHONIOENCODING=utf-8 python examples/reviews-and-comments/demo/demo.py

# 3. View the results — open in any browser
open examples/reviews-and-comments/demo/index.html
```

Expected output (abridged):

```
[0] registering domain tree...            ok
[1] 16 identities...                      ok
[2] product quid + title...               ok
[3] trust graphs...                       ok
[4] 5 reviews posted...                   ok
[5] 5 HELPFUL → veteran, 3 UNHELPFUL → suspicious 5-star
[6] 10 filler reviews for veteran activity
[7] per-observer ratings:

  Observer                           Rating        Contributing
  ----------------------------------------------------------
  Classic unweighted average         3.96          5
  Alice (techie)                     4.53          2/5
  Bob (skeptic)                      4.34          2/5
  Carol (restaurant reviewer)        4.50          1/5
```

## What to look for in the UI

Open `index.html` after the scenario runs. Click each observer
box to see:

- **Why Alice sees 4.53**: she trusts `veteran` at 0.9 directly,
  so veteran's 4.5 carries nearly all the weight. `rev_random`'s
  5 stars are filtered out (she doesn't trust rev_random) and
  `newbie`'s 2 stars are filtered out (same reason).
- **Why Bob sees 4.34**: he trusts `veteran` at 0.6 (less than
  Alice), but also trusts `rev_bob_trusts` at 0.8, so his score
  is a weighted average of 4.5 and 3.5.
- **Why Carol sees 4.50**: her only edge is in
  `reviews.public.restaurants` — not a laptop domain. Topic
  inheritance from `reviews.public` kicks in with the 0.8-per-hop
  decay, so her effective trust in `veteran` collapses to about
  0.9 × 0.8² ≈ 0.576. Only veteran's review clears the
  weight threshold.

Classic unweighted average is 3.96 — exactly what every review
site on the web shows today. That single number lumps together
the trustworthy `veteran`'s 4.5, the hostile outlier `newbie`'s
2.0, and the obviously-inflated `rev_random`'s 5.0. The three
per-observer views extract signal from the noise in
qualitatively different ways.

## How signing works

The Python `cryptography` library emits DER-encoded ECDSA
signatures. The reference Go node validates IEEE-1363 `r||s`
(64-byte concatenation, zero-padded). These are not
interchangeable.

Rather than reimplement ECDSA in Python, the demo spawns
`sign_helper` (Go binary) as a persistent subprocess and sends
it unsigned transactions over stdin. It returns the fully signed
transaction verbatim — including a SEC1 uncompressed public key
(`04 || X || Y`, 65 bytes) — which Python POSTs to the node.

```
 ┌──────────────┐  stdin JSON lines  ┌────────────────────┐
 │  demo.py     │ ──────────────────▶│  sign-helper.exe   │
 │  (orchestr.) │◀──────────────────│  (crypto/ecdsa)    │
 └──────────────┘  stdout JSON lines └────────────────────┘
        │
        │ HTTP POST /api/transactions/...
        ▼
 ┌──────────────────────────────────────────────────────┐
 │  quidnug node (single-node, 500ms blocks)            │
 │  validates + gossips + generates blocks + commits    │
 └──────────────────────────────────────────────────────┘
```

This keeps a single source of truth for signature bytes (the Go
node), while letting the demo logic live in Python where it's
short and readable.

## Why each event goes on the signer's own stream

The Go node enforces "only the quid owner can append to its own
stream." This is the right default for trust accounting, but it
means you can't write a REVIEW directly to a product's stream —
you'd have to be the product's owner to do that.

So the demo writes each REVIEW to the *reviewer's* stream with
`productAssetQuid` in the payload. To aggregate reviews for a
given product the consumer scans across reviewer streams and
filters by payload. `effective_rating` does exactly this.

This is consistent with how `HELPFUL_VOTE` already worked in the
protocol spec (those go on the voter's stream, pointing at the
reviewer), so it's a natural extension.

## Config used

See `demo-config.yaml`. Key knobs:

- `block_interval: "500ms"` — so identity → title → event chains
  don't stall waiting for blocks.
- `rate_limit_per_minute: 100000` — the demo fires ~60-80 txs in
  rapid succession.
- `allow_domain_registration: true` — `reviews.public.*` topics
  are created on the fly.
- `require_parent_domain_auth: false` — no cross-validator
  handshake needed for the demo tree.

## Node-side bugs fixed along the way

The demo surfaced three real bugs in the reference Go node that
are fixed in this branch:

1. `GenerateBlock` didn't include `EventTransaction` in its
   domain-extraction switch, so events stayed in the pending
   pool forever.
2. `ValidateBlockTiered` didn't include `EventTransaction` in
   its per-tx validation switch, so a block containing events
   would be rejected as `BlockInvalid`.
3. `cmd/quidnug/main.go`'s startup called `StartServer(port)`
   which hardcoded `DefaultRateLimitPerMinute` — the value in
   `cfg.RateLimitPerMinute` from the config file was ignored.

The in-process test at `internal/core/reviews_integration_test.go`
exists to prevent (1) and (2) from silently regressing again.

## See also

- `PROTOCOL.md` — full QRP-0001 event-type spec
- `algorithm.md` / `algorithm.py` — standalone rating math
- `bootstrap-trust.md` — how new users get into the trust graph
- `../../../clients/reviews-widget/` — drop-in HTML embed
- `../../../clients/wordpress-plugin/` — WooCommerce integration
- `../../../integrations/schema-org/reviews.go` — Google rich-results
