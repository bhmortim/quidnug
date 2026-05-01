# Home-operator launch plan

> A practical, $5-or-less-per-month playbook for running the public
> Quidnug network from a home machine (via Cloudflare Tunnel) with
> a small VPS for failover, then launching the reviews ecosystem
> on top.

This is the home-operator companion to [`README.md`](README.md),
which assumes you've got Fly.io credits to burn. This plan
instead assumes you have:

- A Windows PC at home already running WSL2 + Caddy + Cloudflare
  Tunnel serving `quidnug.com` (your current website setup).
- Owner of the `quidnug.com` domain.
- A small budget — we're targeting **$0 to $5 per month**
  at launch, scaling linearly as adoption grows.
- A willingness to operate the public network yourself, which
  means: signing blocks under your key, responding to incidents,
  and accepting that uptime is on you.

Read the whole doc once before you start executing. Phase 1 is
~half a day if nothing goes wrong; the reviews rollout is a
second half-day.

> **Read this first:** [`governance-model.md`](governance-model.md)
> explains how nodes join the public network (cache replicas /
> consortium members / governors). The Phase 6 reviews rollout
> assumes you've internalized that role separation. The formal
> protocol spec is [QDP-0012](../../docs/design/0012-domain-governance.md).

---

## Table of contents

1. [Architecture in one diagram](#1-architecture-in-one-diagram)
2. [Bill of materials and cost](#2-bill-of-materials-and-cost)
3. [Security model](#3-security-model)
4. [Phase 1: get one home node on the internet](#4-phase-1-get-one-home-node-on-the-internet)
5. [Phase 2: add a failover VPS node](#5-phase-2-add-a-failover-vps-node)
6. [Phase 3: front the pair with an API gateway Worker](#6-phase-3-front-the-pair-with-an-api-gateway-worker)
7. [Phase 4: monitoring + public status page](#7-phase-4-monitoring--public-status-page)
8. [Phase 5: nightly backup to R2](#8-phase-5-nightly-backup-to-r2)
9. [Phase 6: launch the reviews ecosystem](#9-phase-6-launch-the-reviews-ecosystem)
10. [Operational cadence](#10-operational-cadence)
11. [Scaling triggers](#11-scaling-triggers)
12. [Risks and mitigations](#12-risks-and-mitigations)
13. [Appendix: starter artifacts](#13-appendix-starter-artifacts)

---

## 1. Architecture in one diagram

```
                ┌─────────────────────────────────────────┐
                │         quidnug.com (Astro site)        │
                │   home WSL → Caddy → Cloudflare Tunnel  │
                │   /reviews/* pages use @quidnug/astro-  │
                │   reviews pointing at api.quidnug.com   │
                └──────────────────┬──────────────────────┘
                                   │
                                   ▼
                 ┌─────────────────────────────────────┐
                 │    api.quidnug.com (CF Worker)      │
                 │  health-routes GETs + POSTs between │
                 │  the two node origins; caches safe  │
                 │  GETs at the edge for 30s           │
                 └──────────┬──────────────┬───────────┘
                            │              │
                (primary)   │              │   (failover)
                            ▼              ▼
              ┌───────────────────┐  ┌───────────────────┐
              │ node1.quidnug.com │  │ node2.quidnug.com │
              │                   │  │                   │
              │  Cloudflare       │  │  Cloudflare-      │
              │  Tunnel →         │  │  proxied direct   │
              │  home PC (WSL)    │  │  VPS ($4-6/mo or  │
              │  127.0.0.1:8087   │  │  free tier)       │
              │                   │  │                   │
              │  holds the        │  │  peer validator   │
              │  "root" seed     │  │  (K-of-2 bootstrap │
              │  validator key    │  │  quorum with home) │
              └────────┬──────────┘  └──────────┬────────┘
                       │                        │
                       └───── gossip (P2P) ─────┘
                         signed txs + anchors +
                         epoch rotations over HTTPS
                       │                        │
                       ▼                        ▼
              ┌───────────────────┐  ┌───────────────────┐
              │ ./data (WSL fs)   │  │ /data (VPS disk)  │
              │ + nightly R2      │  │ + nightly R2      │
              │   snapshot        │  │   snapshot        │
              └───────────────────┘  └───────────────────┘
                        ▲                      ▲
                        │                      │
                        └──── cloudflare r2 ───┘
                         (cross-region cold backup,
                          ~$0 at typical data volumes)

          ┌─────────────────────────────────────────────────┐
          │                Monitoring                        │
          │                                                  │
          │  Both nodes remote-write metrics → Grafana      │
          │  Cloud free tier (10k series). Uptime Kuma on   │
          │  the VPS (NOT the home PC) pokes both nodes     │
          │  + the Worker every 60s. Status snapshot        │
          │  published to status.quidnug.com.               │
          └─────────────────────────────────────────────────┘
```

Two design invariants worth internalizing before you start:

- **Home is primary, VPS is failover.** The home PC has more
  resources and carries the "root" validator identity. The VPS
  exists so the network keeps answering queries when your house
  is offline (power, ISP, WSL reboot, whatever).
- **The website and the node network are separable failure
  domains.** If nodes go down, the website still shows cached
  reviews and a clear status banner. If the website goes down,
  nodes keep serving (just not via `api.quidnug.com`).

---

## 2. Bill of materials and cost

### One-time

| Item | Cost | Notes |
|---|---|---|
| Domain (`quidnug.com`) | already owned | no change |
| Home PC disk space | ~10 GB | for blocks + data directory |
| Paper vault for seed-key backup | ~$5 | fireproof envelope, one per year |

### Recurring

| Item | Monthly | Alternatives |
|---|---|---|
| Home PC electricity delta | ~$1-3 | depends on your machine |
| VPS for node2 | $0-6 | Oracle Cloud Free Tier (forever free, 1GB arm), Hetzner CX11 ($4/mo), or Fly free tier |
| Cloudflare Tunnel | $0 | free for unlimited usage |
| Cloudflare Workers (api.quidnug.com router) | $0 | first 100k requests/day free |
| Cloudflare R2 (backups) | $0-1 | first 10 GB storage free, $0 egress |
| Grafana Cloud free tier | $0 | 10k series, 50 GB logs/traces, 14-day retention |
| Uptime Kuma (self-hosted) | $0 | runs on the VPS |
| **Total at launch** | **$0 – $6** | |

Scaling-cost checkpoints (see §11):

- Past ~100k API requests/day at the Worker: $5/mo Workers Paid.
- Past 10k time series at Grafana: ~$19/mo for 100k series, or self-host Prometheus on the VPS.
- Past 10 GB R2: $0.015/GB/month.
- Past Oracle Free Tier egress limits: switch to Hetzner.

### What you're NOT paying for

- Load balancer — Cloudflare does it in the Worker layer.
- SSL — CF Tunnel + CF proxy terminate TLS for free.
- DDoS protection — CF absorbs.
- A status page SaaS — Uptime Kuma renders its own.

---

## 3. Security model

The threat model for this deployment.

### What we protect

1. **The validator keys** (one per node). Losing control of either
   key to an attacker lets them forge blocks that your network
   accepts as `Trusted`.
2. **Your home IP address.** Never reach the internet directly.
   Cloudflare Tunnel puts the home node behind CF's anycast network.
3. **The review corpus.** Reviews are public and immutable, but
   we want them durable against "my home PC's SSD died" and
   "CloudFlare terminates the account."

### What we explicitly don't try to hide

- **Who operates the network.** `seeds.json` is published and
  signed. Transparency is the trust story.
- **The content of events.** Reviews, trust edges, identity
  records are all public. Privacy features (ZK selective
  disclosure, etc.) are on the roadmap but not in this plan.

### Controls we apply

| Surface | Control |
|---|---|
| Home node | Runs as non-privileged WSL user. Binds only to `127.0.0.1`. Exposed to the internet exclusively via `cloudflared` tunnel. |
| VPS node | Runs as dedicated `quidnug` user. UFW firewall drops everything except SSH (from your IPs), 443 (from Cloudflare IPs only), and outbound HTTPS. |
| Inter-node gossip | HMAC-authenticated via the shared `NODE_AUTH_SECRET`. Rotate annually. |
| Validator keys | Generated offline, never touched `NODE_KEY` env var committed to any repo. Paper backup in a fireproof envelope, digital backup in an encrypted password manager entry. |
| Public API | Rate limited at the node (`rate_limit_per_minute` config) + at Cloudflare (WAF rule at 120 req/min per IP on write endpoints). |
| Write endpoints | Cloudflare WAF challenges (not blocks) suspicious UAs. Signed txs validate server-side regardless. |
| Backups | R2 bucket with a write-only IAM policy from the nodes; reads gated on a separate admin token. |

### Key-custody rule of thumb

- **Home key stays on home machine + paper.** Never on GitHub,
  never on the VPS, never in a shared password-manager vault.
- **VPS key is different and subordinate.** The VPS is a
  validator quorum member, not THE root. If the VPS is
  compromised you rotate its key via `AnchorRotation` from home,
  revoke its validator trust edges, and spin up a new VPS.
- **Guardians.** Install a 3-of-5 guardian quorum for the home
  key within 24 hours of first deploy. Use people + orgs you
  trust independently — close friends, an accountant, a lawyer,
  a bank safe-deposit box that holds a sealed envelope. See
  [QDP-0002](../../docs/design/0002-guardian-based-recovery.md).

---

## 4. Phase 1: get one home node on the internet

Goal: `curl https://node1.quidnug.com/api/health` returns 200
from anywhere, served by your home PC via Cloudflare Tunnel, and
the process survives a reboot.

### 4.1 Build + pin a node binary

In WSL inside the quidnug-repo checkout:

```bash
go build -trimpath -ldflags "-s -w" -o ~/bin/quidnug ./cmd/quidnug
~/bin/quidnug --version
```

Put `~/bin/quidnug` on PATH. Also tag the commit you built from
locally so you can reproduce:

```bash
git tag -s node1-$(date +%Y%m%d) -m "node1 build"
```

### 4.2 Generate the operator quid offline

The **operator quid** is your long-lived identity — it accumulates
trust across every node you run, now and in the future. It is
distinct from each node's per-process NodeID (which is auto-
persisted in `data_dir/node_key.json` and changes only when you
explicitly wipe that directory).

Generate the operator quid on your workstation, not the node:

```bash
quidnug-cli quid generate \
    --out ~/.quidnug/operator.quid.json
chmod 600 ~/.quidnug/operator.quid.json
```

Print the public key + quid ID and write them down on paper
alongside a hex dump of the private key. Yes, paper. Put it in
the fireproof envelope.

Copy the same file into an encrypted password manager entry
labeled "quidnug operator quid (long-lived)." This file:

- gets deployed to **every node you run** (same file, every node);
- is referenced from each node's config via `operator_quid_file:`
  or `OPERATOR_QUID_FILE`;
- should **never be regenerated** unless you accept losing every
  trust grant pointing at it. Treat it like an SSH host key.

### 4.3 Create the data directory and config

```bash
mkdir -p ~/.quidnug/data
cat > ~/.quidnug/node1.yaml <<'YAML'
port:                    "8087"
data_dir:                "/home/YOURUSER/.quidnug/data"
log_level:               "info"
seed_nodes:              []  # filled in when node2 exists
rate_limit_per_minute:   600
max_body_size_bytes:     1048576
block_interval:          "5s"
require_node_auth:       true
node_auth_secret_file:   "/home/YOURUSER/.quidnug/auth.secret"
supported_domains:
    - "network.quidnug.com"
    - "reviews.public"
    - "examples.public.quidnug.com"
    - "*"   # accept txs for any domain but gossip only the above
enable_nonce_ledger:     true
enable_push_gossip:      true
enable_kofk_bootstrap:   true
ipfs_enabled:            false
allow_domain_registration: true
require_parent_domain_auth: false

# Operator identity (long-lived; same file on every node)
operator_quid_file:      "/home/YOURUSER/.quidnug/operator.quid.json"

# Static peers (live-reloaded on edit; populated when node2 comes up)
peers_file:              "/home/YOURUSER/.quidnug/peers.yaml"

# Peer admit gates — production defaults
require_advertisement:    true
peer_min_operator_trust:  0.5

# Peer scoring (Phase 4) — defaults are conservative; tighten if
# you want stricter quarantine
# peer_quarantine_threshold: 0.4
# peer_eviction_threshold:   0.2
# peer_eviction_grace:       "5m"
# peer_fork_action:          "quarantine"
YAML

# Seed peers file (start empty; fill in when node2 exists)
cat > ~/.quidnug/peers.yaml <<'YAML'
peers: []
YAML
chmod 600 ~/.quidnug/peers.yaml

# 32-byte shared secret (copied to node2 later)
openssl rand -hex 32 > ~/.quidnug/auth.secret
chmod 600 ~/.quidnug/auth.secret
```

#### What ends up in `data_dir`

After first boot, `~/.quidnug/data/` contains:

```
node_key.json           # this node's per-process keypair (auto-generated, persisted)
blockchain.json         # block history snapshot, refreshed every 30s + on shutdown
trust_domains.json      # TrustDomains + DomainRegistry index
pending_transactions.json
peer_scores.json        # per-peer scoreboard (composite + recent events)
```

Back up the whole directory with the operator-quid file. Losing
`data_dir` is recoverable (you'll re-discover peers and re-receive
gossip); losing the operator-quid file isn't.

### 4.4 Bring up the node under systemd-in-WSL

WSL2 supports systemd since 2022. Verify with
`systemctl --version`; if that fails, add `systemd=true` under
`[boot]` in `/etc/wsl.conf` and `wsl --shutdown` once.

Install the unit:

```bash
sudo cp deploy/public-network/artifacts/systemd/quidnug-node.service \
    /etc/systemd/system/
sudo systemctl daemon-reload
sudo systemctl enable --now quidnug-node
systemctl status quidnug-node
```

Confirm local reachability:

```bash
curl http://127.0.0.1:8087/api/health
# {"success":true,"data":{"status":"ok",...}}
```

### 4.5 Expose via Cloudflare Tunnel

You already have `cloudflared` installed for the website. Add
a new hostname to the existing tunnel rather than creating a new
tunnel — it's one less thing to manage.

Edit `~/.cloudflared/config.yml` (or wherever your tunnel config
lives):

```yaml
tunnel: <your-existing-tunnel-uuid>
credentials-file: /home/YOURUSER/.cloudflared/<uuid>.json

ingress:
    # Existing website rule — leave in place.
    - hostname: quidnug.com
      service: http://127.0.0.1:80

    # NEW — the node's public HTTP surface.
    - hostname: node1.quidnug.com
      service: http://127.0.0.1:8087
      originRequest:
          connectTimeout: 10s
          tcpKeepAlive: 30s
          # Don't let CF reuse TCP connections across requests
          # for long (the node has per-request HMAC state).
          keepAliveTimeout: 60s

    # Catch-all must stay last.
    - service: http_status:404
```

Then in the Cloudflare dashboard → Zero Trust → Access →
Tunnels → [your tunnel] → Public Hostname → add
`node1.quidnug.com` routing to `http://localhost:8087`.

Restart `cloudflared` (or whatever wraps it under systemd):

```bash
sudo systemctl restart cloudflared
```

Verify from anywhere:

```bash
curl https://node1.quidnug.com/api/health
# 200 OK
```

### 4.6 Lock down CF

In the Cloudflare dashboard, under the `node1.quidnug.com`
hostname's rules:

1. **WAF → Security Level:** High.
2. **WAF → Custom Rule:**
   - Name: `throttle-write-endpoints`
   - Expression: `(http.request.method in {"POST" "PUT" "DELETE"}) and (http.request.uri.path matches "^/api/")`
   - Action: Managed Challenge
   - Rate: 120/min per IP
3. **Cache Rules → Rule:**
   - Match: `http.request.uri.path in {"/api/health" "/api/info" "/metrics"}`
   - Cache: bypass (these should always hit the node).
4. **Cache Rules → Rule:**
   - Match: `http.request.method eq "GET" and starts_with(http.request.uri.path, "/api/")`
   - Edge TTL: 30 seconds (safe — all payloads are
     append-only).
5. **Speed → Compression:** Brotli on.

### 4.7 Seed the network's bedrock data

Register the reserved domains. Each domain gets an explicit
initial consortium (your seed nodes) + governor set (you +
your co-founder's quid) — see
[`governance-model.md`](governance-model.md) for the background
on what these roles mean. Run from any machine with CLI access:

```bash
SEEDS="node1-quid:1.0,node2-quid:1.0"     # consortium members
GOVS="operator-quid:1.0,cofounder-quid:1.0" # governor quorum (2-of-2)

for d in network.quidnug.com \
         peering.network.quidnug.com \
         validators.network.quidnug.com \
         operators.network.quidnug.com \
         bootstrap.network.quidnug.com \
         reviews.public \
         reviews.public.technology \
         reviews.public.technology.laptops \
         reviews.public.restaurants \
         reviews.public.services; do
    quidnug-cli domain register \
        --node https://node1.quidnug.com \
        --name "$d" \
        --threshold 0.5 \
        --validators "${SEEDS}" \
        --governors "${GOVS}" \
        --governance-quorum 1.0
done
```

Pre-QDP-0012 activation this extra metadata is just stored; the
node still auto-adds itself as a single-governor validator for
backward compatibility. Post-activation it's the authoritative
source of truth for domain governance. Publishing it now is
forward-compatible and documents your intent.

**Phase 1 done.** You have a public node reachable at
`https://node1.quidnug.com`, serving the reviews reserved
domains, with its key safely custodied.

---

## 5. Phase 2: add a failover VPS node

Goal: `curl https://node2.quidnug.com/api/health` succeeds
independent of your home machine; both nodes have mutual trust
edges so either can serve as the primary validator.

### 5.1 Pick a VPS

- **Oracle Cloud Free Tier** — forever-free 1 GB ARM Ampere
  VM, 200 GB disk, 10 TB egress. This is the cheapest viable
  option. Set it up in a region near your user base
  (`iad` / `lhr` / `syd`).
- **Hetzner CX11** — €3.79/mo, 2 GB RAM, 20 GB disk. More
  predictable, closer EU support.
- **Fly.io free allowance** — 3 shared-cpu VMs included in the
  $5 billing threshold.

For this plan I'll write against a plain Ubuntu 22.04 VM,
which matches all three.

### 5.2 Baseline harden

```bash
adduser --gecos "" quidnug
usermod -aG sudo quidnug
ssh-copy-id quidnug@<ip>

# Now as root via ssh or console:
apt update && apt upgrade -y
apt install -y ufw fail2ban
ufw default deny incoming
ufw default allow outgoing
ufw allow ssh
ufw enable
# Allow 443 inbound only from Cloudflare — add their IP list:
for cidr in $(curl -s https://www.cloudflare.com/ips-v4); do
    ufw allow from "$cidr" to any port 443 proto tcp
done
systemctl enable --now fail2ban
```

SSH config:

```
# /etc/ssh/sshd_config.d/hardening.conf
PermitRootLogin no
PasswordAuthentication no
MaxAuthTries 3
```

`systemctl restart ssh`, then confirm you can still log in.

### 5.3 Install the node

```bash
sudo useradd -r -m -d /var/lib/quidnug -s /bin/false quidnug
sudo mkdir -p /etc/quidnug /var/lib/quidnug/data
sudo chown -R quidnug:quidnug /var/lib/quidnug

# Pull the signed release image:
docker pull ghcr.io/bhmortim/quidnug:latest

# Or build-and-copy the binary; up to you.
sudo cp ./quidnug /usr/local/bin/quidnug
sudo chmod +x /usr/local/bin/quidnug
```

Generate a separate key for node2 (do NOT copy the home key):

```bash
# Both nodes use the SAME operator quid file you generated in §4.2.
# Each node's per-process NodeID is auto-persisted to
# data_dir/node_key.json on first boot — you don't need to generate
# a separate key for node2 the way the original plan implied.
#
# Upload the shared operator quid file over SSH:
scp ~/.quidnug/operator.quid.json quidnug@<vps>:/etc/quidnug/operator.quid.json
ssh quidnug@<vps> "sudo chmod 600 /etc/quidnug/operator.quid.json"
```

Copy the shared secret (same on both nodes):

```bash
scp ~/.quidnug/auth.secret quidnug@<vps>:/etc/quidnug/auth.secret
ssh quidnug@<vps> "sudo chmod 600 /etc/quidnug/auth.secret"
```

Place config at `/etc/quidnug/node2.yaml`. Same as node1, but:

- `port: "8087"` (keep consistent)
- `data_dir: "/var/lib/quidnug/data"`
- `seed_nodes: ["node1.quidnug.com"]`
- `node_auth_secret_file: "/etc/quidnug/auth.secret"`

Install the systemd unit from
[`artifacts/systemd/quidnug-node.service`](artifacts/systemd/quidnug-node.service).

```bash
sudo systemctl daemon-reload
sudo systemctl enable --now quidnug-node
```

### 5.4 Put CF in front

In Cloudflare DNS, add an `A` record `node2 → <vps-ip>` with
proxy ON (orange cloud). Add the `node2.quidnug.com` hostname
to the same WAF rules you applied to node1.

Confirm reachability:

```bash
curl https://node2.quidnug.com/api/health
```

### 5.5 Peer the two nodes

From your workstation:

```bash
./deploy/public-network/peer-seeds.sh \
    --node1 https://node1.quidnug.com \
    --node2 https://node2.quidnug.com
```

(See `peer-seeds.sh` — the Fly version of this script is already
in the repo; it accepts any two URLs.)

The script posts bidirectional `TRUST` transactions at level
0.95 in the `peering.*` and `validators.*` domains. Verify each
side sees the other as `Trusted`:

```bash
curl "https://node1.quidnug.com/api/trust/node1_quid/node2_quid?domain=validators.network.quidnug.com"
curl "https://node2.quidnug.com/api/trust/node2_quid/node1_quid?domain=validators.network.quidnug.com"
```

Add node1 back to node2's seed_nodes config, and node2 to
node1's. Restart both.

**Phase 2 done.** Two nodes, two keys, one shared auth secret,
pairwise trust, and either can continue serving if the other
is offline.

---

## 6. Phase 3: front the pair with an API gateway Worker

Goal: `https://api.quidnug.com` is the public entry point for
any client. It routes to whichever of node1 / node2 is healthy
and caches idempotent GETs at the edge.

I strongly recommend not publishing `node1.quidnug.com` or
`node2.quidnug.com` URLs in client code. Use `api.quidnug.com`
so you can repoint without breaking integrations.

### 6.1 The Worker

Scaffold at `C:\Users\stream\quidnug\workers\api-gateway\`
(in the site repo, NOT this protocol repo). The logic is ~60
lines of TypeScript:

```typescript
// Minimal router. Full version lives in the site repo.
export default {
    async fetch(req, env, ctx) {
        const backends = env.BACKENDS.split(",");
        // 1. Health-check with short-lived KV cache.
        const healthy = await pickHealthy(backends, env);
        if (!healthy) return new Response("no healthy backend",
            { status: 503 });
        // 2. Forward, preserving method + headers + body.
        const url = new URL(req.url);
        url.host = new URL(healthy).host;
        const fwd = new Request(url.toString(), req);
        // 3. Cache safe GETs in CF's edge cache.
        const res = await fetch(fwd);
        if (req.method === "GET" && res.ok &&
            url.pathname.startsWith("/api/")) {
            const cache = caches.default;
            ctx.waitUntil(cache.put(req, res.clone()));
        }
        return res;
    },
};
```

Deploy with `npm run deploy` from the site repo. Point
`api.quidnug.com` DNS at the Worker.

### 6.2 Set BACKENDS

```bash
wrangler secret put BACKENDS
# paste: https://node1.quidnug.com,https://node2.quidnug.com
```

### 6.3 Health-check cache

Put a `NODE_HEALTH` KV namespace on the Worker. Cache health
status for 10 seconds. `pickHealthy` reads from the cache first,
falls through to a live probe, and writes the result back. This
means the 99th-percentile request path does zero probes.

**Phase 3 done.** `https://api.quidnug.com` now gives you:

- Health-aware routing with a 10-second stale-OK cache
- 30-second edge cache on safe GETs (nothing happens to event
  data in that window — the protocol is append-only)
- Automatic failover when one node is down

---

## 7. Phase 4: monitoring + public status page

Goal: you know before your users do when something is broken,
and users have a status page they can check.

### 7.1 Metrics → Grafana Cloud

Sign up for a free Grafana Cloud account. It gives you:

- Prometheus endpoint (remote-write)
- Grafana with dashboards
- 10k time series (enough for 2 nodes)

Set `GRAFANA_REMOTE_WRITE_URL` and `GRAFANA_REMOTE_WRITE_TOKEN`
as node env vars. The Helm chart and Docker image already
support remote-write; for the plain systemd deployment, run a
sidecar `grafana-agent` binary pointing at both nodes'
`/metrics`.

Import the repo dashboard:

```bash
# In Grafana Cloud UI:
Dashboards → Import → Upload JSON →
    deploy/observability/grafana-dashboard.json
```

Alert rules from
`deploy/observability/prometheus-alerts.yml` — upload via
Grafana Cloud's alert rules UI or the Terraform provider.

### 7.2 Liveness → Uptime Kuma

On the VPS (NOT the home machine — you want uptime monitoring
that keeps working when your home is the thing that's down):

```bash
docker run -d --restart=always --name uptime-kuma \
    -p 127.0.0.1:3001:3001 \
    -v uptime-kuma:/app/data \
    louislam/uptime-kuma:1
```

Expose via `status.quidnug.com` through the same Cloudflare
proxy pattern you used for node2. Add monitors:

- `https://node1.quidnug.com/api/health` — 60s interval
- `https://node2.quidnug.com/api/health` — 60s interval
- `https://api.quidnug.com/api/health` — 60s interval
- `https://quidnug.com` — 120s interval
- Ping `<home-ip>` — 120s interval (indirect health signal)
- Keyword check on `https://node1.quidnug.com/api/info` for
  `"protocolVersion"` — catches malformed responses

Wire up a public status page in Uptime Kuma at
`https://status.quidnug.com`. Configure the page to show the
last 24h of each monitor.

### 7.3 Notifications

In Uptime Kuma: email + webhook notifications. Route to:

1. Your personal email (immediate)
2. A Signal / Slack / Discord webhook
3. A fallback SMS via Twilio or Pushover (for genuine outages
   lasting >5 minutes)

Hard rule: do NOT put the notification endpoint on the home
machine. If home goes dark, the alert needs to fire from
elsewhere.

---

## 8. Phase 5: nightly backup to R2

Goal: losing either node's disk never loses the public review
corpus. Cold restore can reconstitute from R2.

### 8.1 Create an R2 bucket

```bash
wrangler r2 bucket create quidnug-node-snapshots
```

Generate an R2 API token with `Object:Write` scope for the
bucket (Cloudflare dashboard → R2 → Manage R2 API Tokens).
Store the token + account-id in the nodes' environment.

### 8.2 Install the backup script

From
[`artifacts/backup/r2-snapshot.sh`](artifacts/backup/r2-snapshot.sh),
copy to both nodes and schedule in `cron`:

```
# /etc/cron.d/quidnug-backup
30 3 * * * quidnug /usr/local/bin/r2-snapshot.sh node1 >>/var/log/quidnug-backup.log 2>&1
```

### 8.3 Retention policy

- Keep every nightly snapshot for 14 days.
- Keep one weekly snapshot for 12 weeks.
- Keep one monthly snapshot indefinitely.

The script does this via R2 object-lifecycle rules configured
once through the Cloudflare dashboard.

### 8.4 Periodic restore test

Once per quarter, spin up a throwaway VPS, pull the latest
snapshot, start a node pointing at it with
`seed_nodes: []`, and confirm it serves a reasonable portion of
the blockchain by comparing block index tip against the live
node. This verifies your backups actually work — don't skip it.

---

## 9. Phase 6: launch the reviews ecosystem

With the two-node network healthy and public, you're ready to
light up QRP-0001 end-to-end.

### 9.1 Register the topic tree

Do it once, from the CLI, against either node:

```bash
for topic in \
    reviews.public.technology.cameras \
    reviews.public.technology.phones \
    reviews.public.books \
    reviews.public.movies \
    reviews.public.restaurants.us \
    reviews.public.restaurants.eu \
    reviews.public.services; do
    quidnug-cli domain register \
        --node https://api.quidnug.com \
        --name "$topic" --threshold 0.5
done
```

### 9.2 Bootstrap trust — the hardest part

A brand-new review network has no trust graph yet, so every
review renders as "no basis yet." This isn't a failure; it's
honest. But it means adoption needs a trust seeding plan.

Three bootstrap paths, in priority order:

1. **OIDC → Quidnug binding.** Deploy the OIDC bridge
   (`cmd/quidnug-oidc/`) and set up Google / GitHub / Keycloak
   as identity providers. When a user signs in with Google, the
   bridge mints them a Quidnug quid tied to their verified
   email. All OIDC-bound identities start with an implicit trust
   edge from the `operators.*` root (tunable), which gives them
   baseline weight for other observers who trust OIDC-verified
   accounts.
2. **Cross-site import.** An early reviewer with existing
   reputation on another site (say Amazon or Yelp) can request
   an attestation from a domain operator you trust. That
   operator signs a one-time `TRUST` edge from the
   `operators.network.quidnug.com` root to their quid.
3. **Social bootstrap.** Invite a small group of known humans
   (friends, colleagues, beta testers) and have them trust each
   other directly. This is fine for the first dozen users; it
   becomes the seed that OIDC-bound users then trust indirectly.

All three are documented in
[`../../examples/reviews-and-comments/bootstrap-trust.md`](../../examples/reviews-and-comments/bootstrap-trust.md).

### 9.3 Light up the website's /reviews section

On the Astro site:

```bash
npm install @quidnug/astro-reviews @quidnug/web-components
```

In `astro.config.mjs`, nothing special — the primitives register
themselves.

Create `src/pages/reviews/index.astro` showing:

- The `<qn-aurora>` primitive at `size="large"` with a live
  rating on a demo product
- A `<qn-constellation>` drilldown explaining the trust-graph
  visualization
- Copy explaining what Quidnug-weighted reviews are, linked to
  the use-case README

Create `src/pages/reviews/[product].astro` as the generic
per-product template. Fetch reviews server-side via the
astro-reviews SSR primitives so search engines see real SVG
rendered from the live api.quidnug.com data.

### 9.4 Ship the integration docs

Add a page `quidnug.com/reviews/integrate/` listing the
framework-adapter packages and pointing at
`docs/reviews/rating-visualization.md`. Ship a "five-minute
integration" guide for the top three patterns:

1. Static HTML (script tag)
2. WordPress / WooCommerce (plugin upload)
3. Any React / Vue / Astro app

### 9.5 Synthetic baseline traffic (optional)

Use the existing `synthetic-traffic.sh` to post a baseline of
realistic review activity under
`examples.public.quidnug.com`. This makes charts look lively
during the quiet launch period. Don't write to the real
`reviews.public.*` tree — those should be authentic only.

---

## 10. Operational cadence

| Frequency | Task |
|---|---|
| Real-time | Uptime Kuma alerts on any monitor failing; acknowledge within 15 minutes |
| Weekly | Skim Grafana dashboard for anomalies; check disk free %; check Worker error rate |
| Monthly | Rotate Worker logs; review R2 storage growth; purge anything unexpected |
| Quarterly | Test R2 restore; review + rotate `NODE_AUTH_SECRET`; review peering requests |
| Annually | Rotate node validator keys (`AnchorRotation`); paper-key vault audit; security audit (external if possible); guardians re-confirmation |
| On any suspected compromise | Follow the emergency playbook in [`README.md#7`](README.md#7-emergency-playbook) |

---

## 11. Scaling triggers

When you see these metrics, make the corresponding change.

| Signal | Action |
|---|---|
| Worker at >80% of free tier (80k req/day) | Enable Workers Paid ($5/mo unlimited). |
| Either node CPU >60% p95 for a week | Double the VPS VM size, or add node3. |
| Uptime Kuma flags >5 home-node outages per month | Promote VPS to primary and home to secondary (swap the validator roles — this is a key-lifecycle event, NOT a config change). |
| R2 storage >10 GB | Move cold snapshots to R2 Infrequent Access (cheaper), or prune older-than-quarterly. |
| Grafana series >10k | Either trim cardinality (drop `tx_id` labels) or upgrade to paid ($19/mo for 100k). |
| Reviews traffic exceeds the 600/min node rate limit | Raise the node config limit + CF WAF rate. Consider edge-caching longer. |
| One topic gets >10k reviews/day | Shard into child topics — `reviews.public.technology.laptops.dell`, etc. |

---

## 12. Risks and mitigations

| Risk | Probability | Severity | Mitigation |
|---|---|---|---|
| Home PC power / ISP outage | High (monthly) | Low (VPS covers) | Two-node design; CF Worker auto-routes. |
| Home PC hardware failure | Medium (annual) | Medium (reconstruct from backup + paper key) | R2 backups; paper key; guardian quorum. |
| CF Tunnel account termination | Low (rare) | High (loss of public URL) | Use a stable email billing address; keep a secondary email verified; be prepared to switch to direct-exposed VPS-only for 24h. |
| Validator key compromised | Low | High (network rebirth risk) | Guardian recovery; publish `AnchorInvalidation`. |
| Node binary bug corrupts chain | Very low | High | Keep signed tagged releases; test each release on a scratch node; R2 backups before upgrading. |
| DDoS against api.quidnug.com | Medium (adoption-correlated) | Low (CF absorbs) | CF in front; WAF rate limits; Cloudflare DDoS protection is mostly transparent. |
| Abusive users posting garbage reviews | High at any adoption | Low (filtered by trust graph) | Trust-weighted rating algorithm naturally de-weights. Add a `FLAG` event flow to the moderation dashboard. |
| Review content becomes legally problematic (defamation, CSAM) | Low but non-trivial | High | See §9.6 below. |

### §9.6 Content moderation legal note (short version)

Quidnug is append-only and signed. Removing content is not a
protocol primitive. For legal-removal obligations (DMCA, GDPR
right-to-erasure, defamation rulings):

- You as the node operator CAN choose to not gossip specific
  event IDs under your node's operational policy, effectively
  making them unreachable via `api.quidnug.com`.
- You CANNOT prevent other node operators from serving the
  content, and you CANNOT rewrite history.
- For EU operations, consult a lawyer about whether your node
  operation meets the GDPR definition of a controller. The
  safe answer is "treat yourself as a hosting provider subject
  to take-down requests, act in good faith, document your
  decisions."
- Publish a clear content policy at `quidnug.com/policy` before
  you invite any third-party reviewers.

---

## 13. Appendix: starter artifacts

Living under `deploy/public-network/artifacts/`:

- [`systemd/quidnug-node.service`](artifacts/systemd/quidnug-node.service)
  — systemd unit usable on both WSL-systemd and VPS
- [`cloudflare-tunnel/config.yml.example`](artifacts/cloudflare-tunnel/config.yml.example)
  — CF Tunnel config snippet adding `node1.quidnug.com`
- [`wsl-startup/start-quidnug-home.sh`](artifacts/wsl-startup/start-quidnug-home.sh)
  — WSL boot hook so the node starts with the machine
- [`backup/r2-snapshot.sh`](artifacts/backup/r2-snapshot.sh)
  — nightly backup script (cron-scheduled)
- [`reviews-launch-checklist.md`](reviews-launch-checklist.md)
  — the go-live checklist for the reviews ecosystem

Each artifact is a template with `<PLACEHOLDER>` markers — fill
those in for your specific keys, paths, and secrets.

---

## What this plan deliberately skips

Things that are good ideas but not first-day launch work:

- **Multi-region failover.** Two nodes is enough to start. Adding
  a third node in a different region is a later phase when you
  have traffic that justifies it.
- **Full HSM-backed signing.** The root key on paper + encrypted
  password manager is fine for launch. HSM / YubiHSM is a
  reasonable Q3 upgrade when key value justifies the setup time.
- **Community validator onboarding.** The peering protocol
  supports it, but until adoption is real, there's no benefit
  to inviting more operators. Cross that bridge when review
  volume makes it interesting.
- **The visualization Figma plugin.** Nice to have; blocks
  nothing.

Ship what's here, monitor it, and the next phase's priorities
will become obvious from operational data.
