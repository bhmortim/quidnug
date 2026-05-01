# Sharding + discovery model

> The plain-English companion to
> [QDP-0014](../../docs/design/0014-node-discovery-and-sharding.md).
> How you run many nodes under a single operator identity,
> how clients find the right node for a query, and how you
> shard workload across regions / domain trees / node roles.
>
> Read [`governance-model.md`](governance-model.md) and
> [`federation-model.md`](federation-model.md) first — this
> doc assumes you know what a consortium, a cache replica,
> and a federated network are.

## The two-layer identity

Running a serious public network splits into two classes of
cryptographic identity:

**Operator quid.** One (maybe two or three) per organization.
It's "you" — the human/legal entity running the infrastructure.
Reputation targets it. Governance authority lives with it.
Cross-network federation attestations bind to it. You guard
this key like it's the keys to your house, because it kind
of is.

**Node quid.** One per running server. It's "this box" — a
specific Linux process holding blocks. Blocks it produces are
signed with its key; peers authenticate it by this key. Losing
a node key is bad but recoverable (spin up a replacement);
losing an operator key is a network-rebirth event.

The binding between them is a TRUST edge:

```
operator_quid ──TRUST 1.0──► node_quid_1
              ──TRUST 1.0──► node_quid_2
              ──TRUST 1.0──► node_quid_N
         in operators.network.<your-domain>
```

These edges are on-chain, signed by the operator. They're the
authoritative answer to "does this node belong to this
operator?" Any query that walks the operator's reputation
graph naturally traverses them.

## How clients find the right node

Three layers of discovery, each more specific than the last:

### Layer 0: Well-known file (cold-start)

A client knows nothing about your network except the domain
name. They fetch:

```
https://<your-domain>/.well-known/quidnug-network.json
```

That file lists:

- Your operator quid + pubkey (for verification).
- Your api-gateway URL (`api.quidnug.com`).
- Your seed nodes (a few node-quid+URL entries).
- Your primary served domain trees.
- Your governance documentation URL.

All signed by your operator key. Clients pin your pubkey and
verify every response that claims to come from "you" from that
point forward.

This file is what makes your network discoverable. Publish it
at a stable HTTPS URL and it works forever.

### Layer 1: Discovery API (domain → endpoint hints)

Once a client has your api-gateway URL, they query:

```
GET https://api.quidnug.com/api/v2/discovery/domain/<name>
```

Response (signed):

```json
{
    "domain": "reviews.public.technology.laptops",
    "blockTip": {"index": 42817, ...},
    "consortium": {"validators": {...}, "threshold": 0.5},
    "endpoints": [
        {"nodeQuid": "...", "endpoints": [...], "capabilities": {...}},
        ...
    ]
}
```

The client now knows:

- Which nodes are the current consortium for this domain.
- Which URLs those nodes are reachable at.
- What each node can do (validator? cache? archive? IPFS?).
- Where the chain tip is.

### Layer 2: Signed node advertisements (authoritative)

If the client doesn't trust the discovery response (hint: they
shouldn't, blindly), they fetch each node's signed
advertisement:

```
GET https://api.quidnug.com/api/v2/discovery/node/<quid>
```

Returns the raw `NODE_ADVERTISEMENT` transaction, signed by
the node itself. Cross-check:

1. The advertisement is signed by the node's quid.
2. The node's quid has an operator-attestation TRUST edge
   from a pubkey the client pinned from the well-known file.
3. The advertisement hasn't expired.

If all three pass, the client uses the endpoints from the
advertisement directly, bypassing any potentially-adversarial
intermediary.

### The flow in one picture

```
┌────────────────┐
│  Client has    │  1. GET /.well-known/quidnug-network.json
│  only the      │─────────────────────────────────────►
│  domain name   │  2. Pin operator pubkey, cache api URL
└────────┬───────┘
         │ 3. GET /api/v2/discovery/domain/<name>
         ▼
┌────────────────┐
│  api.quidnug   │─────────────────────────────────────►
│   .com Worker  │  4. Returns signed endpoints + hints
└────────┬───────┘
         │ 5. Pick best endpoint by
         │    (region, capability, priority)
         │ 6. GET /api/streams/...
         ▼
┌────────────────┐
│  Specific node │  7. Fulfills the actual query
│  serving this  │
│  domain        │
└────────────────┘
```

Two HTTP hops in steady state (the discovery response caches
at the CDN edge for 30 seconds, so many queries skip step 3).

## Sharding strategies

You have four ways to split workload across multiple nodes.
Pick any combination.

### Geographic sharding

Nodes in different regions. Clients pick the closest.

```
Operator: you
Nodes:
  node-iad-1..3  region=iad, roles=[validator, cache, archive]
  node-lhr-1..2  region=lhr, roles=[validator, cache]
  node-sin-1     region=sin, roles=[cache]
```

Good for reducing latency globally. Each region has its own
validators so write-side operations (POSTs) are local too.

### Domain-tree sharding

Different nodes handle different parts of your domain tree.
Each advertisement's `supportedDomains` declares which.

```
Operator: you
Nodes:
  node-reviews-1..3    supportedDomains=[reviews.public.*]
  node-meta-1..2       supportedDomains=[operators.*, peering.*]
  node-archive-1       supportedDomains=[*]
```

Good when different domains have very different access patterns
(e.g., high-traffic reviews vs rarely-touched meta-domains).

### Capability sharding

Separate the work each node does. Validators are expensive
(they produce blocks, they need full chain history); cache
replicas are cheap (they just serve reads).

```
Operator: you
Nodes:
  validator-1..3     capabilities={validator:true, cache:false, archive:false}
  cache-1..10        capabilities={validator:false, cache:true}
  archive-1          capabilities={validator:false, cache:true, archive:true}
  ipfs-1..2          capabilities={ipfsGateway:true, cache:true}
  bootstrap-1..2     capabilities={bootstrap:true, archive:true}
```

Good at scale. You have few validators (they're the trust
anchors), many caches (throughput), a couple archives (audit),
specialized IPFS and bootstrap nodes (specific workload).

Clients hitting a write endpoint (`POST /api/events`) are
routed by the api-gateway to a validator. Clients hitting a
read endpoint (`GET /api/streams/*`) get routed to a cache.
They never touch an overloaded validator for a plain read.

### Network-federation sharding

Some nodes bridge multiple networks (via QDP-0013 federation).
Their advertisements declare both networks' domains.

```
Operator: you
Nodes:
  public-only-1..5         supportedDomains=[reviews.public.*]
  public-bridge-1..2       supportedDomains=[reviews.public.*, consortium-x.private.*]
  private-only-1..3        supportedDomains=[consortium-x.private.*]
```

Good for operators who run both public and private networks.
Bridge nodes see both; private-only nodes stay private.

## Mixing strategies in practice

Nothing constrains you to a single strategy. A realistic
steady-state deployment might be:

```
Operator: you (quidnug.com operator)
  Geographic: 3 regions (iad, lhr, sin)
  Capabilities: validator-only, cache-only, archive, bootstrap, ipfs
  Domain-tree: public-reviews nodes vs meta-domain nodes

Per region (iad example):
  quidnug-iad-val-1: validator, archive
  quidnug-iad-val-2: validator, archive
  quidnug-iad-cache-1..5: cache
  quidnug-iad-ipfs-1: ipfs+cache

Per network-wide:
  quidnug-bootstrap-1: bootstrap+archive (anywhere)
  quidnug-bootstrap-2: bootstrap+archive (elsewhere)
  quidnug-meta-domain-1: supportedDomains=[*.network.quidnug.com]
```

Total: ~20 nodes, geographically distributed, role-specialized,
single operator identity, everything discoverable by any
client with the `quidnug.com` well-known file.

## Running your first extra node

When you graduate from one node to two, here's the minimum:

### 1. Bring up the new node

The new node will auto-generate its per-process NodeID on first boot
(persisted to `data_dir/node_key.json`). Configure it with the same
`operator_quid_file:` pointing at your shared operator quid — that's
how this node identifies itself as one of yours. After first boot,
note the NodeID:

```bash
curl http://node-iad-cache-1.example/api/v1/info | jq -r .data.nodeQuid
# e.g. 91d0f4b88f44c7a2  ← use this in step 2 below
```

### 2. Publish the operator-attestation TRUST edge

```bash
./bin/quidnug-cli trust grant \
    --truster <operator-quid> \
    --trustee <new-node-quid> \
    --domain "operators.network.quidnug.com" \
    --level 1.0 \
    --nonce <next-nonce> \
    --sign-with operator.key.json
```

### 3. Deploy the node

Standard home-operator-plan flow with the new key.

### 4. Have the node publish its advertisement

```bash
# Runs on the new node itself, signed with its own key
./bin/quidnug-cli node advertise \
    --operator-quid <operator-quid> \
    --endpoints "https://cache-iad-1.quidnug.com:443,http/2,iad,1,100" \
    --capabilities "cache" \
    --supported-domains "reviews.public.*" \
    --expires-in "6h" \
    --sign-with ~/.quidnug/node-iad-cache-1.key.json
```

### 5. Update the well-known file

Add the new node to `.well-known/quidnug-network.json`, re-sign
with the operator key, redeploy.

### 6. Clients discover it automatically

Within ~30 seconds (CDN TTL) clients start routing eligible
queries to the new node. The discovery API picks it up
immediately from the advertisement registry.

Total operator effort per new node: ~10 minutes once you have
the CLI automation set up.

## Cost analysis at various scales

| Scale | Nodes | Config | Typical cost |
|---|---|---|---|
| Launch | 2 | 1 home + 1 cheap VPS | $0-6/month |
| 1k active users | 3-4 | +1 cache VPS, +1 archive | $10-20/month |
| 10k active users | 6-8 | 2 regions, separate validator/cache | $50-100/month |
| 100k active users | 15-20 | 3 regions, role specialization, CDN | $200-500/month |
| 1M active users | 30+ | Heavy CDN, multiple bootstrap nodes, dedicated IPFS | $1k-3k/month |

Discovery + sharding design is what lets you scale up without
re-architecting — you add nodes, publish advertisements, done.
The client discovery flow doesn't change.

## Standards alignment (the short version)

Why this isn't just something we made up:

- **RFC 8615 well-known URIs** — the `.well-known/` path is a
  standard HTTP pattern used by ACME, OIDC, and dozens of
  other protocols. Our `/.well-known/quidnug-network.json`
  follows the same convention.
- **W3C DID Core 1.0** — node identities are expressible as
  `did:quidnug:<quid-id>` and resolve to W3C-compliant DID
  Documents. Third-party DID tooling works out of the box.
- **OpenID Connect Discovery** — our well-known file format
  mirrors `/.well-known/openid-configuration`, so developers
  familiar with OIDC recognize the pattern.
- **DNS SRV/TXT records** — optional DNS-based entry point for
  clients that start from a DNS name. Uses existing DNS
  primitives, no new record type.

Adopting these means:

- Existing DID resolvers understand your nodes.
- Existing HTTP client libraries handle `.well-known` paths
  automatically.
- Tooling from the OIDC ecosystem can read your configuration.
- Operators familiar with DNS can set up SRV records if they
  prefer DNS-based discovery.

We're not inventing a parallel universe; we're fitting the
existing web-standards patterns to a decentralized-trust
protocol.

## Participating without running a node

The discovery model makes something important possible that
wasn't obvious before: **you don't have to run a node at all**.

A third-party app — a review UI, a rating widget, a mobile
app — can own a quid, sign transactions, submit them to
`api.quidnug.com`, and get all the protocol benefits (cryptographic
identity, reputation fungibility, on-chain audit trail, the
whole visualization primitives stack) without running any
infrastructure.

### The path

1. **Generate a quid.** Once, offline. Save the key securely
   (password manager, paper backup, whatever your operator
   policy is). This is your app's cryptographic identity.

2. **Register an app-specific domain.** Under the public
   tree, via the public API. Declare your quid as the sole
   governor:

   ```bash
   quidnug-cli domain register \
       --node https://api.quidnug.com \
       --name "reviews.public.apps.your-app" \
       --governors "<your-app-quid>:1.0" \
       --governance-quorum 1.0 \
       --threshold 0.5 \
       --sign-with app.key.json
   ```

3. **Sign and POST transactions.** Your app's backend signs
   with the quid key and submits to `api.quidnug.com`. The
   public network's consortium validates, includes in a
   block, gossips.

4. **Read via discovery API.** Aggregate ratings, review
   lists, user reputation — all via `api.quidnug.com`.

You now have a fully-functional app built on the public
Quidnug network without any infrastructure of your own. The
cost is exactly zero (API Gateway absorbs the hits; rate
limits apply but are generous).

### When to graduate to a node

You outgrow no-node mode when:

- You hit the public API's rate limits consistently.
- Read latency (~10-50ms over the API) is too slow for your UX.
- You want offline operation (e.g., a mobile app that syncs
  later).
- You want cryptographic custody of your own chain for
  compliance / audit.

The transition is smooth: start a cache replica (follow the
home-operator plan from scratch), peer with the public
network, and progressively move reads to your local cache.
Writes still go through the public consortium until you earn
consortium membership.

### Use cases that fit no-node mode

- **Review UI vendors.** A SaaS product that offers
  trust-weighted reviews as a feature. Each customer is a
  tenant; the SaaS provider owns a quid per tenant.
- **CMS plugins.** WordPress, Shopify, Drupal — ship a plugin
  that holds the site owner's quid key and submits
  transactions server-side.
- **Mobile apps.** User installs the app; app generates a
  per-install quid stored in the OS keystore; app submits
  reviews on the user's behalf.
- **Browser extensions.** Wallet-style UI where the extension
  holds the user's quid and signs transactions on their
  behalf across any site that supports Quidnug reviews.
- **Indie review sites.** Someone builds a specialty review
  site for an obscure domain. No node, no infrastructure —
  just a frontend that talks to `api.quidnug.com`.

## Finding quids by domain

Once there are many participants, you need ways to discover
which quids are active in a given domain. A review UI needs
to find "trusted reviewers in this topic." A moderation tool
needs to find "spammy accounts in this domain." A federation
tool needs to find "reputable operators to consider peering
with."

The discovery API answers these queries via two new endpoints:

### `/api/v2/discovery/quids?domain=...`

Returns the index of quids active in a domain, with filters
and sorts. Sample query — "top 20 reviewers in laptops by
Alice's trust weight":

```
GET /api/v2/discovery/quids?
    domain=reviews.public.technology.laptops
    &sort=trust-weight
    &observer=<alice-quid>
    &limit=20
```

Response is a list of quids with their activity stats + Alice's
trust weight in each. The UI can render "top trusted reviewers
for Alice" immediately.

### `/api/v2/discovery/trusted-quids?domain=...&min-trust=0.5`

The stricter question: "who does the consortium itself trust
in this domain?" Useful when you want a consortium-blessed
list (verified reviewers, attested service providers,
pre-vetted operators).

Smaller result set, highly cacheable, safe to display
directly as "verified" badges.

### Use cases the quid index unlocks

- **"Show me trusted reviewers"** — review UI discovers
  reviewers to feature.
- **"Who's new?"** — moderation surface for recently-active
  quids that haven't yet earned trust edges.
- **"Who should I federate with?"** — a private network
  scouting for reputable public-network operators.
- **"Who reviews which categories most?"** — reviewer-profile
  pages show the domains a reviewer is most active in.
- **"Is this quid legitimate?"** — a third-party caller
  fetching a quid's per-domain stats to decide whether to
  trust them.

### Privacy implications

Because all transactions are public, the quid index is always
available for public domains — it's just a view of data that
already existed. For private domains, the index is served only
to authenticated consortium members (per local operator
policy).

If your app's users expect privacy, pair this primitive with
topical quid segregation — use a different quid per domain if
you need to prevent cross-domain correlation.

## What about the "I want this to be a standard" question

Three things are needed for QDP-0014 to become a genuine
standard:

1. **Stable on-chain behavior.** Ship the
   `NODE_ADVERTISEMENT` tx type, run it for 6+ months, shake
   out edge cases.
2. **Reference implementations in multiple languages.** Go
   (the reference node) is done; Python, JS, Rust clients
   need discovery-API client libraries.
3. **Formal registration of the well-known URI with IANA.**
   Per RFC 8615 §3.1: submit
   `quidnug-network` to the IANA registry. Takes a few weeks.

Once those three are done, you can submit it as an informational
Internet-Draft to the IETF — doesn't require standardization
ratification, just publication. Someone writing a standards-
conformant client library later can cite the RFC as
authoritative.

The peering-protocol and the reviews-protocol (QRP-0001) could
follow the same path: ship them, iterate on real usage,
publish as Internet-Drafts, encourage independent
implementations.

## Further reading

- [QDP-0014 formal spec](../../docs/design/0014-node-discovery-and-sharding.md)
  — protocol details, attack-vector analysis, schemas.
- [`node-advertisement.schema.json`](../../schemas/json/node-advertisement.schema.json)
  — wire-format JSON schema for the tx.
- [QDP-0013 network federation](../../docs/design/0013-network-federation.md)
  — how discovery interacts with cross-network queries.
- [QDP-0012 domain governance](../../docs/design/0012-domain-governance.md)
  — consortium membership, which the discovery layer reflects.
- [`home-operator-plan.md`](home-operator-plan.md) — starts
  with a 2-node deployment; this doc shows the path beyond
  that.
