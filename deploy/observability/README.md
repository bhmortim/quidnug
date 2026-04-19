# Observability — Grafana + Prometheus

Turnkey dashboards and alert rules for running Quidnug in production.

```
deploy/observability/
├── grafana-dashboard.json     # importable Grafana dashboard
├── prometheus-alerts.yml      # Prometheus alerting rules
└── README.md                  # this file
```

## What's covered

Quidnug nodes expose the metric family `quidnug_*` at
`GET /metrics` (Prometheus text format). The dashboard and alerts
reference these families directly — no renaming or relabeling needed.

| Family | Purpose |
| --- | --- |
| `quidnug_blocks_total` | Block production rate |
| `quidnug_transactions_total` | Tx ingestion rate (labeled by type) |
| `quidnug_pending_transactions` | Mempool depth |
| `quidnug_connected_nodes` | Peer count |
| `quidnug_http_requests_total` / `_duration_seconds` | API surface |
| `quidnug_trust_computation_duration_seconds` | Relational-trust BFS latency |
| `quidnug_nonce_replay_rejections_total` | QDP-0001 replay defense |
| `quidnug_nonce_ledger_entries` | Nonce-ledger sizing |
| `quidnug_guardian_resignations_total` / `_rejected_total` / `_set_weakened_total` | QDP-0002/0006 lifecycle |
| `quidnug_fork_block_accepted_total` / `_activated_total` / `_rejected_total` | QDP-0009 activation |
| `quidnug_gossip_push_received_total` / `_forward_dropped_total` / `_rate_limited_total` / `_propagation_latency_seconds` | QDP-0005 gossip health |
| `quidnug_merkle_proof_used_total` / `_fallback_total` | QDP-0010 proof usage |
| `quidnug_probe_attempts_total` / `_success_total` / `_failure_total` | QDP-0007 home-domain probes |
| `quidnug_quarantine_*` | Stale-epoch quarantine state |
| `quidnug_block_missing_tx_root_rejected_total` | QDP-0010 sanity check |

## Importing the dashboard

1. In Grafana, go to **Dashboards → Import**.
2. Upload `grafana-dashboard.json` (or paste its contents).
3. Select your Prometheus data source when prompted. The dashboard
   uses the template variable `${DS_PROMETHEUS}`, which Grafana
   resolves at import time.

The dashboard has four rows:

- **Top stats** — blocks/hour, txs/hour, peers, mempool, quarantine,
  nonce ledger. Intended for a Loki-style status wall.
- **HTTP** — requests/sec by path, p95 latency by path.
- **Consensus + gossip** — trust computation latency, gossip
  propagation, security rejections, guardian/fork-block lifecycle.
- **Merkle + probes** — QDP-0010 proof usage, QDP-0007 probe outcomes.

Template variables:

- `$job` — the Prometheus job name scraping Quidnug nodes (default
  `quidnug`).
- `$instance` — one or many instance labels.

## Wiring the alert rules

Mount `prometheus-alerts.yml` into your Prometheus pod and reference
it from `prometheus.yml`:

```yaml
rule_files:
  - "/etc/prometheus/rules/quidnug-alerts.yml"
```

Or if you use Prometheus Operator, wrap it in a `PrometheusRule`
CRD — the top-level `groups:` block is already in the right shape.

### Alert groups

| Group | Goal |
| --- | --- |
| `quidnug.availability` | Node up, 5xx rate, HTTP latency. |
| `quidnug.consensus` | Block production stall, pending backlog, peer floor. |
| `quidnug.security` | Nonce replay, missing tx-root, guardian weakening, fork rejections, quarantine overflow. |
| `quidnug.gossip` | Propagation latency, rate-limiting. |
| `quidnug.probes` | QDP-0007 home-domain probe failure rate. |

Severity conventions:

- `critical` — page on-call. Protocol-level safety impact.
- `warning` — address within one business day.
- `info` — observability signal, no pager.

Tune the `for:` windows and numeric thresholds to your environment.
The defaults target a reasonably busy protocol node (≥ 1 block/min,
≥ 2 peers). Small dev deployments will want longer windows.

## Quick start on a dev host

```bash
# Scrape target
cat <<EOF > /etc/prometheus/prometheus.yml
global:
  scrape_interval: 15s
scrape_configs:
  - job_name: quidnug
    static_configs:
      - targets: ["localhost:8080"]
rule_files:
  - "/etc/prometheus/rules/quidnug-alerts.yml"
EOF

mkdir -p /etc/prometheus/rules
cp deploy/observability/prometheus-alerts.yml /etc/prometheus/rules/
systemctl reload prometheus

# Grafana → Import → upload deploy/observability/grafana-dashboard.json
```

## Versioning

Dashboard + alerts are versioned with the node build. Treat
`grafana-dashboard.json` as a source-of-truth artifact: regenerate or
hand-edit, but always keep it in lockstep with which metric families
are live. Retired metrics should be removed from the dashboard in the
same PR that removes the emission.

## License

Apache-2.0.
