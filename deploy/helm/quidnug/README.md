# Quidnug Helm Chart

Deploy a Quidnug node (or a multi-replica StatefulSet) on Kubernetes.

## TL;DR

```bash
helm repo add quidnug https://bhmortim.github.io/quidnug/charts   # planned
helm install my-node quidnug/quidnug -n quidnug --create-namespace

# or from a source checkout:
helm install my-node deploy/helm/quidnug -n quidnug --create-namespace
```

## Prerequisites

- Kubernetes 1.24+
- Helm 3.12+
- For `serviceMonitor.enabled: true` and `prometheusRule.enabled: true`:
  kube-prometheus-stack (or the Prometheus Operator CRDs) must be
  installed in the cluster.

## What it ships

| Resource | Purpose |
| --- | --- |
| `StatefulSet` (default) or `Deployment` | Runs the Quidnug node. |
| `Service` (ClusterIP) | Stable in-cluster endpoint on port 8080. |
| `Service` (headless) | Required for StatefulSet stable DNS. |
| `ConfigMap` | Mounted at `/etc/quidnug/config.yaml`. |
| `Ingress` (opt-in) | External HTTP access. |
| `PersistentVolumeClaim` | 50Gi default for `/var/lib/quidnug`. |
| `ServiceAccount`, `PodDisruptionBudget` | Operational plumbing. |
| `ServiceMonitor` (opt-in) | Prometheus scrape config. |
| `PrometheusRule` (opt-in) | The bundled alert rules. |

## Key values

| Value | Default | Description |
| --- | --- | --- |
| `image.repository` / `image.tag` | `ghcr.io/bhmortim/quidnug` / chart appVersion | Container image. |
| `replicaCount` | `3` | Node count. 3+ recommended for Proof-of-Trust quorum. |
| `persistence.enabled` | `true` | PVC for node state. |
| `persistence.size` | `50Gi` | Per-replica storage. |
| `service.type` | `ClusterIP` | `LoadBalancer` / `NodePort` also supported. |
| `ingress.enabled` | `false` | Enable for external HTTP access. |
| `serviceMonitor.enabled` | `false` | Scrape `/metrics` into Prometheus. |
| `prometheusRule.enabled` | `false` | Ship bundled alerts as CRD. |
| `config.*` | see values.yaml | Any key under `config:` is rendered into node config.yaml verbatim. |
| `existingQuidSecret` | empty | Reference a pre-created Secret holding the bootstrap quid key. |
| `generateQuidOnInstall` | `false` | Auto-generate a quid into a Secret on first install. |

## Custom configuration

Everything under `config:` in `values.yaml` is rendered into the
node's `config.yaml` one-to-one. Override any key with `-f` or `--set`:

```bash
helm install my-node deploy/helm/quidnug \
  --set config.node.homeDomain=contractors.example.com \
  --set config.gossipPush.maxPerSecond=200 \
  --set service.type=LoadBalancer
```

## Production guidelines

- **Multi-zone**: default affinity spreads replicas across zones. Set
  `replicaCount: 5` + `podDisruptionBudget.minAvailable: 3` for an
  HA Proof-of-Trust quorum.
- **TLS termination**: terminate at Ingress; the node speaks plain HTTP
  internally.
- **Bootstrap quid**: for production, pre-provision the Quid keypair
  and reference via `existingQuidSecret`. The secret must have a key
  `quid-private-hex` containing the PKCS8 DER hex-encoded private key.
- **Resource tuning**: the default 500m / 1Gi request / 2CPU 4Gi limit
  fits a small/medium workload. Scale up for high-throughput domains.
- **Monitoring**: enable `serviceMonitor` and `prometheusRule`, then
  import the Grafana dashboard from `deploy/observability/`.

## Uninstall

```bash
helm uninstall my-node -n quidnug
# PVCs are NOT deleted by default. To purge state:
kubectl delete pvc -n quidnug -l app.kubernetes.io/instance=my-node
```

## Verifying the rendered manifests

```bash
helm template deploy/helm/quidnug > /tmp/rendered.yaml
kubectl --dry-run=client apply -f /tmp/rendered.yaml
```

## License

Apache-2.0.
