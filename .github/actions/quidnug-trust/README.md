# Quidnug Trust Check — GitHub Action

Queries Quidnug relational trust in a GitHub workflow and fails
the step if the computed trust is below a threshold.

Use cases:

- **Branch protection**: only allow PRs whose author is
  transitively trusted by the repo's maintainer quid at ≥ 0.7.
- **Release gating**: require approval from multiple reviewer
  quids before pushing a production tag.
- **Deployment gates**: block prod deploys whose originator
  doesn't pass your org's trust floor for "deploy.prod".
- **Downstream integrity**: before pulling a dependency's
  release artifact, verify the publisher's quid has sufficient
  trust from your org.

## Usage

```yaml
name: trust-gated-release
on:
  push:
    tags: ["v*"]

jobs:
  trust-gate:
    runs-on: ubuntu-latest
    steps:
      - name: Check release author's trust
        uses: quidnug/quidnug/.github/actions/quidnug-trust@main
        with:
          node: ${{ secrets.QUIDNUG_NODE_URL }}
          observer: ${{ vars.COMPANY_TRUST_ROOT_QUID }}
          target: ${{ vars.RELEASE_AUTHOR_QUID }}
          domain: "company.releases"
          threshold: "0.8"
          auth-token: ${{ secrets.QUIDNUG_TOKEN }}

      - name: Ship it
        run: make release
```

## Inputs

| Input | Required | Default | Description |
| --- | --- | --- | --- |
| `node` | yes | — | Node URL (e.g. `https://quidnug.example.com`) |
| `observer` | yes | — | Observer quid ID |
| `target` | yes | — | Target quid ID |
| `domain` | yes | — | Trust domain |
| `threshold` | no | `0.7` | Minimum acceptable trust in [0, 1] |
| `max-depth` | no | `5` | Maximum hops to search |
| `auth-token` | no | — | Bearer token if node requires auth |
| `fail-on-no-path` | no | `true` | Treat "no path" as failure |

## Outputs

| Output | Description |
| --- | --- |
| `trust-level` | Computed trust in [0, 1] |
| `path` | Comma-separated quid IDs along the best path |
| `path-depth` | Number of hops |
| `passed` | `"true"` iff trust ≥ threshold |

Use downstream:

```yaml
- uses: quidnug/quidnug/.github/actions/quidnug-trust@main
  id: trust
  with: { ... }

- name: Annotate
  if: always()
  run: |
    echo "Trust level: ${{ steps.trust.outputs.trust-level }}"
    echo "Path:        ${{ steps.trust.outputs.path }}"
```

## Exit codes

- **0** — trust ≥ threshold.
- **1** — trust < threshold, or no path found (unless
  `fail-on-no-path: false`).

## Requirements

- `curl` and `jq` available on the runner (both are
  pre-installed on GitHub-hosted runners).
- Network access to your Quidnug node from the runner. For
  self-hosted runners behind a VPN, this just works; for
  GitHub-hosted runners against a private node, use a public
  endpoint or a proxy.

## Security notes

- Pass the auth token via `secrets.QUIDNUG_TOKEN`, never hardcode.
- The trust query is read-only; this action never submits
  transactions to the node. Worst-case impact of a compromised
  action config is incorrect pass/fail decisions.

## License

Apache-2.0.
