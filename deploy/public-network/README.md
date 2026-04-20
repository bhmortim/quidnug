# Public network — seed operator playbook

> Operational guide for running the seed nodes behind `quidnug.com`.
> Not for joiners — if you want to peer with the public network, see
> [peering-protocol.md](peering-protocol.md).
>
> **The protocol is network-neutral.** The playbook below is
> specifically about the `quidnug.com` public network, but the
> same steps let anyone run their own parallel network with
> different names, keys, and peers. See
> [`federation-model.md`](federation-model.md) and
> [QDP-0013](../../docs/design/0013-network-federation.md) for
> the architectural statement and the primitives that let
> reputation flow between networks.
>
> **Read first:** [`governance-model.md`](governance-model.md)
> — how nodes actually join the network. Defines the cache-replica,
> consortium-member, and governor roles that everything below
> references. The formal protocol spec is
> [QDP-0012](../../docs/design/0012-domain-governance.md).
>
> **Cheap home-based deployment:** this README assumes a Fly.io
> budget. If you'd rather run the primary node on a home machine
> via Cloudflare Tunnel with a tiny VPS for failover (target cost
> $0–$6/month), follow
> [`home-operator-plan.md`](home-operator-plan.md) instead. The
> two playbooks describe the same architecture with different
> infrastructure choices.
>
> **Reviews rollout:** once your nodes are live, walk through
> [`reviews-launch-checklist.md`](reviews-launch-checklist.md) to
> bring the QRP-0001 ecosystem online on top of them.

## What lives here

| File                                | Purpose                                                          |
| ----------------------------------- | ---------------------------------------------------------------- |
| [peering-protocol.md](peering-protocol.md) | Wire protocol for peering requests (joiner-facing).       |
| [fly.toml.template](fly.toml.template)     | Fly.io config; copy per seed node.                        |
| [seeds.json](seeds.json)                   | Authoritative seed-node identity list (published publicly). |
| [synthetic-traffic.sh](synthetic-traffic.sh) | Cron script that generates realistic baseline traffic.  |
| [rejection-reasons.md](rejection-reasons.md) | Stable enum for peering-request rejections.             |

Seed nodes are named `quidnug-seed-<n>` where `n` in `{1, 2, 3}`. They
run on Fly.io in three regions (IAD, LHR, SIN) for latency diversity
and basic fault isolation.

## 0. One-time bootstrap

### Generate seed-node identities

Each seed node is a separate `Quid`. Generate three locally — **not**
inside Fly — so you retain the private keys offline:

```bash
cd $QUIDNUG_REPO
./bin/quidnug-cli keygen --out seed-1.key.json --name "quidnug-seed-1"
./bin/quidnug-cli keygen --out seed-2.key.json --name "quidnug-seed-2"
./bin/quidnug-cli keygen --out seed-3.key.json --name "quidnug-seed-3"
```

Store the private keys in a password manager **and** print paper copies
into your physical recovery vault. The quid IDs become the public
identity of the network; losing all three keys simultaneously is a
network-rebirth event.

### Install guardians

Immediately after generation, set up a guardian quorum for each seed
node's quid. Recommended: 3-of-5 with time-lock of 24 hours. Guardians
should be independent humans/orgs you trust for this specific role.
See [QDP-0002](../../docs/design/0002-guardian-based-recovery.md).

### Publish the identities

Commit `seeds.json` and the operator attestation in this directory.
The site will mirror it to `https://quidnug.com/network/seeds.json`
at build time.

## 1. Deploy a seed node

Pre-reqs: `flyctl auth login` (one-time), the image built and pushed
to GHCR (see [.github/workflows/publish-image.yml](../../.github/workflows/publish-image.yml)).

```bash
cd deploy/public-network
NODE_REGION=iad NODE_NAME=quidnug-seed-1 ./deploy-node.sh
```

Behind the scenes that script:

1. Copies `fly.toml.template` to `fly.<name>.toml` and fills the region,
   name, and volume mount.
2. Runs `fly launch --copy-config --no-deploy` to create the app.
3. Creates a 10 GB volume in the region.
4. Sets secrets:
   - `NODE_KEY` — base64 of the seed node's private key JSON.
   - `NODE_AUTH_SECRET` — shared 32-byte HMAC across all seed nodes.
   - `IPFS_GATEWAY_URL` if you want IPFS-backed events.
5. Runs `fly deploy` to push the image.
6. Prints the node's public URL. Add a CNAME at
   `node-<n>.quidnug.com` pointing at it.

Repeat for `seed-2` (region `lhr`) and `seed-3` (region `sin`).

## 2. Peer the seed nodes

Seed nodes must mutually trust each other first — otherwise their own
blocks tier as `Tentative` from each other's view.

```bash
./peer-seeds.sh
```

That script:

1. Fetches each seed's current quid and epoch.
2. Posts pairwise `TRUST` transactions at level 0.95 in the domains
   `peering.network.quidnug.com` and `validators.network.quidnug.com`.
3. Verifies the edges landed (tier: Trusted) on each node.

## 3. Publish the public endpoint

The developer-facing URL `api.quidnug.com` is a Cloudflare Worker that
health-routes to a healthy seed. The Worker lives at
`C:\Users\stream\quidnug\workers\api-gateway\` in the site repo.

```bash
cd C:\Users\stream\quidnug\workers\api-gateway
npm run deploy
```

Update `SEED_URLS` in `wrangler.jsonc` (the env var) if you add/remove
seeds.

## 4. Ongoing operations

### Reviewing peering requests

Open GitHub issues with the `peering-request` label; the issue body is
the signed request JSON (see
[peering-protocol.md §2](peering-protocol.md#2-peering-request)).

```bash
./review-peering.sh <issue-number>
```

The script:

1. Pulls the issue body, parses the JSON request.
2. Verifies the signature against the declared public key.
3. Checks `sha256(publicKey)[:16] == quidId`.
4. Probes `nodeEndpoint /api/info`, confirms protocol version.
5. Opens an interactive approval prompt showing everything it checked.
6. On approval, posts `TRUST` txns from the reviewing seed node.
7. Comments on the issue with the tx IDs and links to each edge.

Reject via:

```bash
./review-peering.sh <issue-number> --reject <reason>
```

`<reason>` must be one of the enums in
[rejection-reasons.md](rejection-reasons.md).

### Rotating a seed node's key

Do this at least every 12 months.

```bash
./rotate-seed-key.sh quidnug-seed-1
```

Workflow:

1. Generate new key locally.
2. Publish `AnchorRotation` from the current epoch's key.
3. Update Fly secret `NODE_KEY` to the new key JSON.
4. Restart the node: `fly deploy --strategy immediate -a quidnug-seed-1`.
5. Optionally cap old-epoch nonces to bound the tail.

Existing peer trust edges remain valid — they reference the quid, not
the epoch.

### Handling an outage

Fly auto-restarts crashed VMs. For deeper issues:

```bash
fly status -a quidnug-seed-1
fly logs  -a quidnug-seed-1 --region iad
fly ssh console -a quidnug-seed-1
```

The other two seeds continue serving. If the outage exceeds 30 minutes,
post a status note to `status.quidnug.com` (or update
`src/pages/network/index.astro#status` and redeploy the site).

### Adding a fourth seed

Only add seeds you're personally going to operate. Adding a seed
expands the "root of trust" for the network, and misconfiguring one
can poison the public namespace.

### When to rotate the shared `NODE_AUTH_SECRET`

- Immediately if you suspect any seed was compromised.
- Otherwise once per year.

Rotation is zero-downtime if you add the new secret as a second allowed
value before removing the old — the inter-node HMAC middleware accepts
either during the transition window (feature flag
`ACCEPT_SECONDARY_NODE_AUTH=true`).

## 5. Observability

Each seed exposes `/metrics`. Prometheus on each seed remote-writes to
Grafana Cloud (free tier). Dashboards live at:

- https://grafana.com/dashboards/quidnug-public-network (operator-only)
- https://quidnug.com/network/ (public summary — reads the same data
  via the metrics Worker)

Metrics you should watch:

| Metric                                    | What worries you                                   |
| ----------------------------------------- | -------------------------------------------------- |
| `quidnug_tx_rejected_total{code}`         | Sustained spike in a single code suggests a bug or abuse. |
| `quidnug_block_tier_total{tier="Untrusted"}` | A peer is pushing hostile blocks; investigate.  |
| `quidnug_gossip_inbound_total{src}`       | One source disproportionately loud; rate-limit.    |
| `quidnug_guardian_recovery_inflight`      | Non-zero means someone is recovering a key.        |
| HTTP p99                                  | >500ms sustained = node saturated; scale up.       |

## 6. Seed traffic (synthetic)

`synthetic-traffic.sh` runs as a cron on one of the seed nodes and
posts a realistic baseline of trust edges, identity updates, and
events — ~1 transaction per minute across a handful of example
domains.

Why: the landing page's charts are meant to show activity. Without
synthetic traffic they're mostly flat during developer quiet periods,
which undersells the network. The script writes transactions to
`examples.public.quidnug.com` — a domain reserved for throwaway demo
data.

Don't run it on more than one seed; the duplicate work is wasteful and
the charts will look busier than they really are.

## 7. Emergency playbook

### You suspect a seed key is compromised

1. Publish `AnchorInvalidation` for the affected epoch from the
   current live key (if you still have it).
2. Update `seeds.json` to mark the quid as `status: "frozen"`.
3. Revoke peering edges the compromised seed published using
   `./revoke-edges.sh --from <quid>` — this is a bulk revoke action
   operating from the other two seeds.
4. Post status.
5. If no live key remains: initiate guardian recovery
   (`GuardianRecoveryInit`) and wait the time-lock.

### All three seeds are down

The public network can't process transactions but already-anchored
data is fine. DNS fallback: point `api.quidnug.com` at the static
read-only mirror (`mirror.quidnug.com`) which serves the last snapshot
of the chain as a read-only API. See
[deploy/public-network/readonly-mirror.md](readonly-mirror.md) for the
pattern.

### A peer is abusing the network

1. Publish a `TRUST` with `trustLevel: 0.0` from all seeds against the
   peer's quid in `validators.*` and `peering.*`.
2. Announce in `#network`. Other operators decide whether to follow.
3. Rotate the seed's `NODE_AUTH_SECRET` if you suspect the peer had it.

## 8. Cost expectations

Baseline (three seeds, Grafana Cloud free tier, Cloudflare Workers):

| Line item                                  | Monthly USD          |
| ------------------------------------------ | -------------------- |
| Fly.io · 3× shared-cpu-1x · 512 MB · 10 GB | ~$15–24              |
| Grafana Cloud                              | $0 (free tier)       |
| Cloudflare Workers + KV                    | $0 (within free tier)|
| GitHub Actions (public repo)               | $0                   |
| Domain renewal                             | ~$10/year            |
| **Total**                                  | **~$20–25/mo**       |

This scales linearly with each additional seed. Traffic volume past the
free tiers will add Workers Paid ($5/mo) plus Fly per-GB egress.
