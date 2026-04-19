# quidnug-cli

Operator-facing command-line interface for Quidnug nodes.

Wraps `github.com/quidnug/quidnug/pkg/client` so every capability of
the Go SDK is reachable from a shell: health checks, identity /
trust / title writes, event emission, guardian queries, gossip,
Merkle proof verification.

## Build / install

```bash
go install github.com/quidnug/quidnug/cmd/quidnug-cli@latest
# or from source
go build -o quidnug-cli ./cmd/quidnug-cli
```

## Common workflows

```bash
# Sanity-check a node
quidnug-cli health
quidnug-cli info --node http://localhost:8080

# Generate a keypair and register an identity
quidnug-cli quid generate --out alice.json
ALICE=$(jq -r .id alice.json)
quidnug-cli identity register --quid alice.json --name Alice --home-domain demo.home

# Grant trust and query relational trust
quidnug-cli trust grant --signer alice.json --trustee $BOB --level 0.9 --domain demo.home
quidnug-cli trust get $ALICE $BOB --domain demo.home

# Emit an event
quidnug-cli event emit --signer alice.json \
    --subject-id $ALICE --subject-type QUID \
    --type LOGIN --payload '{"ip":"198.51.100.7"}'

# Inspect a guardian set
quidnug-cli guardian get $ALICE

# Verify a compact Merkle proof offline (no node required)
quidnug-cli merkle verify --tx tx.bin --proof proof.json --root $ROOT_HEX
```

## Global flags

| Flag | Default | Description |
| --- | --- | --- |
| `--node` | `$QUIDNUG_NODE` or `http://localhost:8080` | node base URL |
| `--timeout` | `30s` | per-request timeout |
| `--token` | `$QUIDNUG_TOKEN` | bearer auth token |
| `--json` | off | JSON output instead of key=value |
| `-v`, `--verbose` | off | verbose output |

## Exit codes

| Code | Meaning |
| --- | --- |
| `0` | Success. |
| `1` | Generic failure (bad args, unreachable file, etc.). |
| `2` | Validation error (bad flags, missing required input). |
| `3` | Server logically rejected (nonce replay, quorum not met, etc.). |
| `4` | Service unavailable / feature not yet activated. |
| `5` | Transport / 5xx after retries. |
| `6` | `merkle verify` reported mismatch (valid inputs, bad proof). |

These map to the Go SDK's error taxonomy, so scripts can branch on them.

## Output formats

The default format is `key=value` lines for scalar-heavy responses and
pretty-printed JSON for everything else. Pass `--json` on any command
to force JSON output (useful for piping into `jq`).

Example:

```bash
$ quidnug-cli trust get $A $B --domain demo.home
trust_level=0.810000
path=alice -> carol -> bob
depth=2
domain=demo.home

$ quidnug-cli trust get $A $B --domain demo.home --json
{
  "observer": "alice...",
  "target":   "bob...",
  "trustLevel": 0.81,
  "trustPath": ["alice...", "carol...", "bob..."],
  "pathDepth": 2,
  "domain":    "demo.home"
}
```

## Security notes

- `.quid.json` files hold private keys. The CLI writes them with mode
  `0600`. Back them up offline — lose the file, lose the identity.
  Guardian recovery (QDP-0002) can replace the key if you pre-configured
  guardians; otherwise the quid is unrecoverable.
- `--token` is read from the `$QUIDNUG_TOKEN` env var by default so it
  does not appear in shell history.

## License

Apache-2.0.
