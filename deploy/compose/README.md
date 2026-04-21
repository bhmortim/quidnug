# Local consortium — Docker Compose

Spin up a three-node Quidnug consortium with IPFS pinning,
Prometheus, and Grafana pre-wired to the bundled dashboard. Useful
for local development, demos, and integration tests.

## Prerequisites

- Docker Engine 20.10+
- `docker compose` v2+ (bundled with Docker Desktop)

## Up / down

```bash
cd deploy/compose
docker compose up -d
docker compose logs -f n1

# once you're done
docker compose down -v     # -v wipes named volumes too
```

## Block interval

The compose file sets `BLOCK_INTERVAL=2s` on every node by
default. This is the **dev-friendly** setting: transactions
commit within a couple of seconds rather than the protocol
default of 60 seconds. Demos, tutorials, and integration
tests rely on it to finish in reasonable wall-clock time.

For **production**, override to the protocol default:

```bash
BLOCK_INTERVAL=60s docker compose up -d
```

or remove the env var entirely in your own compose overlay.
Long block intervals reduce validator load and align with
QDP-0001's expected timing for nonce-ledger gossip.

## What it runs

| Container | Purpose | Host port |
| --- | --- | --- |
| `quidnug-n1` | Node 1 | 8081 |
| `quidnug-n2` | Node 2 | 8082 |
| `quidnug-n3` | Node 3 | 8083 |
| `quidnug-ipfs` | IPFS pinning for payloads | 5001 |
| `quidnug-prometheus` | Metrics store + alert rules | 9090 |
| `quidnug-grafana` | Dashboard (auto-provisioned) | 3000 |

Grafana default credentials: `admin / admin`.

## Quick tour

```bash
# 1. Are all three nodes up?
for p in 8081 8082 8083; do curl -s http://localhost:$p/api/health; done

# 2. Generate two quids with the CLI and wire trust
./quidnug-cli --node http://localhost:8081 quid generate --out alice.json
./quidnug-cli --node http://localhost:8081 quid generate --out bob.json
ALICE=$(jq -r .id alice.json); BOB=$(jq -r .id bob.json)

./quidnug-cli --node http://localhost:8081 identity register --quid alice.json --name Alice
./quidnug-cli --node http://localhost:8081 identity register --quid bob.json   --name Bob

./quidnug-cli --node http://localhost:8081 trust grant \
    --signer alice.json --trustee $BOB --level 0.9 --domain dev.local

# 3. Query the same relationship from node 2 — should converge
./quidnug-cli --node http://localhost:8082 trust get $ALICE $BOB --domain dev.local
```

## Known limitations

- The compose setup uses the latest `ghcr.io/bhmortim/quidnug:2.0.0`
  image — if you've just built a local change, either `docker tag`
  your build as that image or add a `build:` stanza to the YAML.
- Three nodes run on one Docker network with host-port mappings.
  For a multi-host consortium, use the Helm chart under
  `deploy/helm/quidnug/`.

## License

Apache-2.0.
