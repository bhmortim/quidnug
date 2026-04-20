# Starter artifacts

Concrete templates referenced from
[`../home-operator-plan.md`](../home-operator-plan.md). Each is
a minimal, production-safe starting point — review before
deploying, replace any `<PLACEHOLDER>` markers, and add
deployment-specific hardening.

| File | Where it runs | What it does |
|---|---|---|
| [`cloudflare-tunnel/config.yml.example`](cloudflare-tunnel/config.yml.example) | Home PC (WSL) | Adds `node1.quidnug.com` to an existing Cloudflare Tunnel config so the node is reachable without opening any ports on the home router. |
| [`systemd/quidnug-node.service`](systemd/quidnug-node.service) | Home (WSL-systemd) + VPS | Runs the node as a hardened systemd service. Usable on both deployments. |
| [`wsl-startup/start-quidnug-home.sh`](wsl-startup/start-quidnug-home.sh) | Home PC (WSL) | Fallback boot-time launcher for WSL instances without systemd — starts the node + cloudflared with supervision. |
| [`backup/r2-snapshot.sh`](backup/r2-snapshot.sh) | Both nodes | Nightly tar + zstd + rclone snapshot of `data_dir` to a Cloudflare R2 bucket. |

## Intentionally not included here

The following are described in the plan but deliberately left
for a separate build-out pass, so the user can decide scope
before committing code:

- **API gateway Cloudflare Worker** (`api.quidnug.com` router)
  — lives in the *site* repo rather than this protocol repo,
  alongside the other Workers. The plan describes the routing
  logic; pair-program the Worker when ready.
- **Uptime Kuma monitor import file** — Uptime Kuma's own UI
  is easy enough to configure once; generating a JSON export
  isn't worth the maintenance.
- **Grafana Cloud dashboards** — `deploy/observability/` already
  has the importable JSON. The plan directs you there.
- **`peer-seeds.sh` command-line variant** — the existing Fly.io
  version in this directory already supports arbitrary URLs via
  `--node1 / --node2` flags. No template needed.
- **TOR / I2P exposure** — scope creep for launch; evaluate
  after traffic patterns become clear.

## Updating these artifacts

When the node's behavior changes (e.g. new config keys, new
required env vars, new metrics endpoints), update these files
in the same PR. The plan document in the parent directory
references them by relative path — keep the relative paths
stable.
