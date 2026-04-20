# QDP-0014: Node Discovery and Domain Sharding

| Field      | Value                                                            |
|------------|------------------------------------------------------------------|
| Status     | Draft — design only                                              |
| Track      | Protocol + ops                                                   |
| Author     | The Quidnug Authors                                              |
| Created    | 2026-04-20                                                       |
| Requires   | QDP-0001, QDP-0012, QDP-0013                                     |
| Implements | How clients find the node that actually holds the chain for a domain, at operator-scale. |

## 1. Summary

At operator scale the current flat `seed_nodes` model breaks
down. A single operator will eventually run tens of servers,
each handling different domains with different roles
(validator / cache replica / archive), possibly in different
regions, possibly for different networks (per QDP-0013).
Clients today have no way to discover which specific node
holds the blocks for a given domain — they just hit a known
seed and hope.

QDP-0014 introduces three primitives that make discovery
explicit, verifiable, and cache-friendly:

1. **Operator-to-nodes attestation** — a signed on-chain
   record binding individual node quids to an operator quid.
   Establishes "these N servers are all me."

2. **`NODE_ADVERTISEMENT` transaction** — a signed record
   each node publishes declaring its endpoints, supported
   domains, capabilities, and expiration. Client-reachable,
   updatable, expires on its own if a node goes offline.

3. **`.well-known/quidnug-network.json`** — a signed static
   JSON document an operator publishes at a stable HTTPS URL.
   Entry point for discovery when the client has never heard
   of the network before. Follows RFC 8615 well-known URIs.

The design aligns with existing web standards: nodes are
addressable as `did:quidnug:<quid>` per W3C DID spec;
endpoints resemble DNS SRV records; the well-known document
mirrors OpenID Connect's `/.well-known/openid-configuration`
pattern.

## 2. Goals and non-goals

**Goals:**

- A client with nothing but an operator's root URL can bootstrap
  to the right node for any domain.
- Operators can shard by domain tree, by region, by
  capability (validator / cache / archive / IPFS gateway), or
  any combination.
- Clients can pick the "best" endpoint for their query
  (closest region, right capability, live-and-healthy).
- Advertisements are signed — clients don't trust random
  peers' claims about where data lives.
- Stale advertisements age out without operator intervention.
- Works across federated networks (QDP-0013) — a client on
  network A can discover nodes on network B if A federates
  with B.

**Non-goals:**

- Global DHT routing. Each network discovers within itself
  (plus federation hops). No Chord / Kademlia layer.
- Automatic load balancing or failover at the protocol
  layer. Clients pick endpoints using published hints; load
  balancing happens at the HTTP layer (Cloudflare Workers,
  Envoy, nginx, whatever).
- Service-mesh-level features (mTLS, circuit breakers).
  Bring your own service mesh if you need those.
- Privacy protection. Public-network node endpoints are
  inherently public. Private networks can still use this
  primitive for their own discovery; the protocol doesn't
  enforce any privacy boundary — that's config.

## 3. The operator-to-nodes hierarchy

At operator scale there are two distinct classes of quid:

**Operator quid.** One or a small handful per operator. Used
for:

- Signing `seeds.json` / `quidnug-network.json` attestations.
- Receiving reputation (trust edges target the operator quid,
  not individual nodes).
- Issuing governance decisions (as a governor per QDP-0012).
- Cross-network federation (QDP-0013 attestations bind to the
  operator quid).

**Node quid.** One per running server. Used for:

- Signing blocks in its consortium-member capacity.
- Authenticating gossip (HMAC with a shared secret + the
  node's pubkey).
- Publishing its own `NODE_ADVERTISEMENT`.

The binding between operator and nodes is a reserved TRUST
edge structure:

```
operator_quid
    ──TRUST──► node_quid_1  in  operators.network.<operator-domain>
    ──TRUST──► node_quid_2  in  operators.network.<operator-domain>
    ...
    ──TRUST──► node_quid_N  in  operators.network.<operator-domain>
```

These trust edges are the authoritative "this node is mine"
signal. Any client following an operator-reputation path walks
these edges; any node claiming to belong to the operator
without the edge is a forgery.

Why a trust edge and not a custom tx type? The existing
`TRUST` transaction already covers it — at weight 1.0 in the
reserved `operators.network.<operator-domain>` domain, the
edge is unambiguous "this is my node, operated by me." Reusing
`TRUST` keeps the on-chain state model uniform and lets
existing query tooling ("show me all of operator X's nodes")
work without new code.

## 4. The `NODE_ADVERTISEMENT` transaction

A new tx type that each node publishes on its own behalf.

```go
type NodeAdvertisementTransaction struct {
    BaseTransaction  // signed by the node's quid

    // Self-reference — must equal the tx's PublicKey's quidID.
    NodeQuid string `json:"nodeQuid"`

    // The operator's quid. There MUST be a current TRUST edge
    // from OperatorQuid → NodeQuid in the operator's
    // operators.network.<operator-domain> for this advertisement
    // to be honored.
    OperatorQuid string `json:"operatorQuid"`

    // One or more endpoints for this node. Clients prefer the
    // endpoint with the lowest priority and highest weight
    // matching their network location + protocol preference.
    Endpoints []NodeEndpoint `json:"endpoints"`

    // Domains this node serves. Patterns are glob-style.
    // An advertisement for "*.quidnug.com" means "I will accept
    // queries for any domain under quidnug.com." If empty,
    // the node advertises for the domains in its on-chain
    // validator records only.
    SupportedDomains []string `json:"supportedDomains,omitempty"`

    // What the node can actually do.
    Capabilities NodeCapabilities `json:"capabilities"`

    // Protocol version this node speaks.
    ProtocolVersion string `json:"protocolVersion"`

    // UnixNano. The advertisement becomes ignored after this;
    // clients MUST NOT use endpoints from expired advertisements.
    // Recommended refresh cadence: 6 hours. Max allowed: 7 days.
    ExpiresAt int64 `json:"expiresAt"`

    // Per-node monotonic nonce prevents replay of old
    // advertisements. Required.
    AdvertisementNonce int64 `json:"advertisementNonce"`
}

type NodeEndpoint struct {
    URL      string `json:"url"`                // https://node.example.com
    Protocol string `json:"protocol"`           // "http/1.1" | "http/2" | "grpc"
    Region   string `json:"region,omitempty"`   // free-form; suggested: "iad" | "lhr" | "sin"
    Priority int    `json:"priority"`           // lower = preferred
    Weight   int    `json:"weight"`             // for equal-priority round-robin
}

type NodeCapabilities struct {
    // Consortium membership for at least one domain. Block-
    // producing role. MUST be backed by on-chain presence in
    // some domain's Validators map.
    Validator bool `json:"validator"`

    // Serves reads + caches the agreed chain. Default true.
    Cache bool `json:"cache"`

    // Holds the full block history. Required for audit +
    // late-joining k-of-k bootstrap sources.
    Archive bool `json:"archive"`

    // Serves K-of-K bootstrap snapshots (QDP-0008).
    Bootstrap bool `json:"bootstrap"`

    // Accepts gossip from any trusted peer.
    GossipSink bool `json:"gossipSink"`

    // Proxies IPFS payload retrieval.
    IPFSGateway bool `json:"ipfsGateway"`

    // Optional. If set, max tx body this node will accept.
    MaxBodyBytes int `json:"maxBodyBytes,omitempty"`

    // Optional. Floors the peer protocol version.
    MinPeerProtocol string `json:"minPeerProtocol,omitempty"`
}
```

### 4.1 Validation rules

1. **Self-sign consistency.** `NodeQuid == sha256(PublicKey)[:16]`.
   The tx must be signed with `PublicKey`.
2. **Operator attestation.** A current TRUST edge from
   `OperatorQuid → NodeQuid` at level ≥ 0.5 must exist in the
   `operators.network.<operator-domain>` domain. Domain name is
   derived from the operator quid's home domain (QDP-0007).
3. **Nonce monotonicity.** `AdvertisementNonce >
   last_advertisement_nonce_for_this_quid`. Replays rejected.
4. **Expiry sanity.** `ExpiresAt > now` and
   `ExpiresAt - now <= 7 days`. Short-TTL advertisements are
   fine, long-TTL advertisements are rejected as a defense
   against stale-forever endpoints.
5. **Endpoint list.** 1 ≤ `len(Endpoints)` ≤ 10. URLs must be
   `https://` (reject plain HTTP). Region strings limited to
   64 chars.
6. **Supported-domain glob limit.** `len(SupportedDomains)` ≤ 50.
   Individual patterns ≤ 253 chars (DNS name limit).
7. **Capability consistency.** `Validator: true` only honored
   if the node is in some domain's `Validators`. Other
   capabilities are self-reported, not cross-checked.
8. **Rate-limit.** Per-`NodeQuid`, max 1 advertisement per
   15 minutes accepted. Bursts queued or rejected.

### 4.2 On-chain storage

Each node's latest valid advertisement is kept in a
`NodeAdvertisementRegistry` map keyed by `NodeQuid`. Old
advertisements are garbage-collected after they expire. Light
clients can fetch the current advertisement via the discovery
API (§6) without replaying chain history.

## 5. Domain endpoint hints

For fast client-side discovery without walking the full
advertisement registry, each domain carries a denormalized
hint list derived automatically from its consortium members'
advertisements.

Extends the `TrustDomain` struct (post-QDP-0012):

```go
type TrustDomain struct {
    // ... all existing fields ...

    // Automatically maintained by the node. Not stored
    // directly in blocks (would be redundant + non-deterministic).
    // Rebuilt on node restart from the advertisement registry.
    EndpointHints []DomainEndpointHint `json:"-"`
}

type DomainEndpointHint struct {
    NodeQuid     string           `json:"nodeQuid"`
    Endpoints    []NodeEndpoint   `json:"endpoints"`
    Capabilities NodeCapabilities `json:"capabilities"`
    ExpiresAt    int64            `json:"expiresAt"`
}
```

Hints are a read-optimization, not authoritative. A client
who doesn't trust the hint fetches the underlying signed
advertisement and verifies.

## 6. Discovery API

Three new endpoints on every node:

### 6.1 `GET /api/v2/discovery/domain/{name}`

Returns the current consortium + endpoint hints + block tip
for the domain.

```json
{
    "domain": "reviews.public.technology.laptops",
    "blockTip": {
        "index": 42817,
        "hash": "0xabc...",
        "timestamp": 1745178293
    },
    "consortium": {
        "validators": {
            "5f8a9b...": 1.0,
            "c7e2d1...": 1.0,
            "9a4b6f...": 1.0
        },
        "threshold": 0.5
    },
    "governance": {
        "governors": { "8e1f3a...": 1.0 },
        "quorum": 1.0,
        "currentNonce": 7
    },
    "endpoints": [
        {
            "nodeQuid": "5f8a9b...",
            "endpoints": [
                {"url": "https://iad.node.quidnug.com", "priority": 1, "weight": 100, "region": "iad"},
                {"url": "https://lhr.node.quidnug.com", "priority": 2, "weight": 100, "region": "lhr"}
            ],
            "capabilities": {"validator": true, "cache": true, "archive": true, "bootstrap": true}
        },
        {
            "nodeQuid": "c7e2d1...",
            "endpoints": [{"url": "https://cache1.node.quidnug.com", "priority": 1, "weight": 100}],
            "capabilities": {"cache": true}
        }
    ]
}
```

Response signed by the serving node's quid in the
`X-Quidnug-Response-Signature` header. Client verifies against
the node's own advertisement.

### 6.2 `GET /api/v2/discovery/node/{quid}`

Returns a specific node's current advertisement — the raw
signed transaction. Useful for verifying hints.

### 6.3 `GET /api/v2/discovery/operator/{quid}`

Returns all nodes attested by this operator. Walks the
operator's `operators.network.<operator-domain>` TRUST edges
and resolves each node's current advertisement.

Response is an array of `NodeAdvertisementTransaction` records.
Ordered by the operator's TRUST edge weight, then by node quid
for stability.

## 7. Well-known entry points

Cold-start discovery: a client with no prior context needs
one URL to bootstrap. Two complementary mechanisms:

### 7.1 `/.well-known/quidnug-network.json` (RFC 8615)

Published at a stable operator-controlled URL (e.g.,
`https://quidnug.com/.well-known/quidnug-network.json`).
Format:

```json
{
    "version": 1,
    "operator": {
        "quid": "8e1f3a...",
        "name": "quidnug.com",
        "publicKey": "<SEC1 uncompressed hex>"
    },
    "apiGateway": "https://api.quidnug.com",
    "seeds": [
        {
            "nodeQuid": "5f8a9b...",
            "url": "https://node1.quidnug.com",
            "region": "home-wsl",
            "capabilities": ["validator", "archive", "bootstrap"]
        },
        {
            "nodeQuid": "c7e2d1...",
            "url": "https://node2.quidnug.com",
            "region": "vps-hetzner-fsn1",
            "capabilities": ["validator", "cache"]
        }
    ],
    "domains": [
        {
            "name": "reviews.public",
            "description": "Trust-weighted product reviews (QRP-0001)",
            "tree": "reviews.public.*"
        },
        {
            "name": "network.quidnug.com",
            "description": "Peering + governance meta-domains",
            "tree": "*.network.quidnug.com"
        }
    ],
    "governance": {
        "documented_at": "https://quidnug.com/network/governance"
    },
    "lastUpdated": 1745178293,
    "signature": "<operator signature over the canonical body>"
}
```

This is the single file any client needs to bootstrap into
your network. Clients verify the `signature` field against the
pinned `operator.publicKey`; everything else cascades from
there.

Format mirrors OpenID Connect Discovery
(`/.well-known/openid-configuration`) for familiarity and
tooling reuse.

### 7.2 DNS TXT records (optional)

For clients that only know the operator's DNS name:

```
_quidnug.quidnug.com. IN TXT "v=1; well-known=https://quidnug.com/.well-known/quidnug-network.json"
```

Resolvers fetch the TXT record, then fetch the well-known
file, then proceed with discovery.

## 8. Client-side discovery algorithm

Given: a domain name and a target query.

```
1. If the client has no network context:
     - Resolve the operator's well-known file
     - Verify its signature
     - Cache (operator pubkey, api gateway, seed list) locally

2. Query the domain:
     a. Hit the API gateway (or any seed) at:
            GET /api/v2/discovery/domain/<name>
     b. Verify the response signature against the serving
        node's advertisement
     c. Inspect the endpoints list

3. Pick an endpoint for the actual query:
     - Filter by required capability (e.g., need validator for
       submitting a block? need bootstrap for k-of-k?)
     - Filter by region preference (geoip or static config)
     - Sort by (Priority asc, Weight desc, randomized tiebreak)
     - Hit the top endpoint

4. On endpoint failure:
     - Try the next endpoint in the sorted list
     - If all fail: refresh the discovery answer (§2a),
       the endpoint set may have changed
     - If still all fail: fall back to api gateway
```

The whole flow takes 0-2 HTTP round trips in steady state (the
discovery response caches at the edge for 30 seconds; the
real query follows).

## 9. Sharding patterns

The combination of operator attestation + node advertisements
+ domain hints enables four sharding strategies, mixable.

### 9.1 Geographic sharding

Multiple advertisements per node's region. Clients pick
closest region.

```
Operator: quidnug.com
  node-iad-1: validator+cache+archive, region=iad
  node-lhr-1: validator+cache,          region=lhr
  node-sin-1: cache,                    region=sin
```

### 9.2 Domain-tree sharding

Different nodes serve different sub-trees. `SupportedDomains`
in each advertisement scopes which domains it answers for.

```
Operator: quidnug.com
  node-reviews-1: {SupportedDomains: [reviews.public.*]}
  node-meta-1:    {SupportedDomains: [operators.network.*, peering.network.*]}
  node-archive-1: {SupportedDomains: [*]}
```

### 9.3 Capability sharding

Separate block-producing, cache-serving, archive, and IPFS
roles.

```
Operator: quidnug.com
  validator-1, validator-2, validator-3: {validator:true, cache:false, archive:false}
  cache-1..10:                           {validator:false, cache:true}
  archive-1:                             {validator:false, cache:true, archive:true}
  ipfs-1, ipfs-2:                        {ipfsGateway:true, cache:true}
  bootstrap-1, bootstrap-2:              {bootstrap:true, archive:true}
```

Validators are precious (block production), cache nodes are
cheap (scale horizontally), archives are few (long-term
storage).

### 9.4 Network-federation sharding

Nodes that bridge two networks advertise for both.

```
Operator: acme-corp (running on quidnug.com public + own private)
  public-bridge-1:  {SupportedDomains: [reviews.public.*, acme-corp.private.*]}
  private-only-1:   {SupportedDomains: [acme-corp.private.*]}
```

Client on the public network hits `public-bridge-1` for
cross-network trust lookups; `private-only-1` stays hidden.

## 10. Attack vectors

### 10.1 Fake node advertisement

**Attack:** Attacker publishes an advertisement claiming to be
a public-network node, with endpoint URL pointing to
attacker-controlled infrastructure.

**Mitigation:** Validation rule §4.1(2): the operator
attestation TRUST edge must exist. An attacker without the
operator's private key can't produce that edge, so their
advertisement is rejected.

Also, the advertisement itself is signed by the node's quid.
Forging an advertisement with a node quid the operator
attested to would require stealing that node's key.

### 10.2 Stale endpoint DoS

**Attack:** Attacker finds an expired-but-stale advertisement,
re-broadcasts it, causes clients to hit a dead endpoint.

**Mitigation:** `ExpiresAt` enforced strictly. Validation
rejects expired advertisements at ingress. Clients also check
`ExpiresAt` before using any endpoint.

### 10.3 DDoS amplification via discovery

**Attack:** Attacker induces many clients to query discovery
simultaneously, overloading the discovery endpoint.

**Mitigation:** Discovery responses cache at the CDN edge for
30 seconds (GETs are idempotent). Attack traffic hits
Cloudflare, not the node.

### 10.4 Endpoint poisoning via compromised node key

**Attack:** A single node key is compromised; attacker
republishes advertisements redirecting queries to attacker
infrastructure.

**Mitigation:**
1. Nodes rotate keys via guardian recovery (QDP-0002).
2. The advertisement nonce is monotonic — a rotated key
   signs a higher nonce, invalidating any attacker
   advertisements.
3. Operator can revoke the operator-TRUST-edge for the
   compromised node quid, severing it from the operator's
   attested node set.

### 10.5 Operator-attestation flooding

**Attack:** Operator publishes thousands of node-attestation
TRUST edges to bloat chain state.

**Mitigation:** Standard tx rate-limit per operator quid.
Practically, an operator has no incentive to do this to
themselves; the defense is really against a compromised
operator key.

### 10.6 Cross-network endpoint confusion

**Attack:** Client on network A looks up a domain name that
exists on both A and B; serves B's endpoints.

**Mitigation:** Discovery is network-scoped. The client's
entry point (the well-known file or configured seed) tells
which network to query. Cross-network queries go through the
QDP-0013 federation mechanism, which explicitly scopes
external lookups. A domain name on network A is a separate
object from the same name on network B.

### 10.7 Private-network discovery leak

**Attack:** A node that bridges public + private networks
leaks private-domain advertisements to public clients.

**Mitigation:** Each advertisement's `SupportedDomains` acts
as a visibility filter. Public clients don't see
private-domain endpoints because the node's public
advertisement doesn't list them. The node serves
private-domain queries only over its private-side
connection (set up by the operator's `supported_domains`
config).

### 10.8 DID Document injection

**Attack:** A client uses DID-resolution libraries that
expect standard verification; an attacker crafts a malicious
DID Document response.

**Mitigation:** DID-compatible output is a bonus, not the
authoritative path. The authoritative source is the
advertisement tx on-chain, signature-verified. Our own client
libraries query the discovery API directly, not via third-
party DID resolvers.

## 11. Standards alignment

### 11.1 DIDs (W3C Decentralized Identifiers)

Every quid is expressible as:

```
did:quidnug:<quid-id>
```

Resolving via:

```
GET /api/v2/discovery/node/<quid-id>?format=did
```

Returns a DID Document conforming to
[W3C DID Core 1.0](https://www.w3.org/TR/did-core/):

```json
{
    "@context": ["https://www.w3.org/ns/did/v1"],
    "id": "did:quidnug:5f8a9b...",
    "verificationMethod": [
        {
            "id": "did:quidnug:5f8a9b...#keys-1",
            "type": "EcdsaSecp256r1VerificationKey2019",
            "controller": "did:quidnug:5f8a9b...",
            "publicKeyHex": "<SEC1 uncompressed>"
        }
    ],
    "service": [
        {
            "id": "did:quidnug:5f8a9b...#node-endpoint",
            "type": "QuidnugNodeEndpoint",
            "serviceEndpoint": "https://node1.quidnug.com"
        },
        {
            "id": "did:quidnug:5f8a9b...#discovery",
            "type": "QuidnugDiscoveryAPI",
            "serviceEndpoint": "https://node1.quidnug.com/api/v2/discovery"
        }
    ]
}
```

Third-party DID resolvers plus our own resolver both work.

### 11.2 DNS SRV / TXT records

Optional entry point via DNS. Mirrors familiar SRV semantics
without inventing a new record type.

### 11.3 RFC 8615 well-known URIs

`.well-known/quidnug-network.json` is a well-known URI per
[RFC 8615](https://www.rfc-editor.org/rfc/rfc8615). We should
register the URI suffix with IANA once the spec is stable —
formally declaring `quidnug-network` as the URI registrant.

### 11.4 OpenID Connect Discovery parallels

Format mirrors `/.well-known/openid-configuration`. This is
intentional — OIDC tooling can parse it with minor adaptation,
and developers familiar with OIDC get the pattern immediately.

### 11.5 W3C Web Packaging (future)

Signed HTTP exchanges would let a CDN serve signed discovery
responses without the serving node proving custody of the
operator key. Deferred; adds complexity for marginal benefit
until the scale justifies.

## 12. Implementation plan

Four phases, parallel where possible.

### Phase 1 — Operator attestation (trivial; already works)

No code changes needed. Document the convention:

- Operator quid publishes TRUST edges to each node quid in
  `operators.network.<operator-domain>` at weight 1.0.
- Seeds.json format includes both operator and node quids.

This is already the intended usage per the existing peering
protocol. Phase 1 is just nomenclature + docs.

### Phase 2 — Node advertisement transaction

- Add `TxTypeNodeAdvertisement` and the struct.
- Implement `ValidateNodeAdvertisementTransaction` per §4.1.
- Registry (`NodeAdvertisementRegistry`) indexed by NodeQuid.
- Expiry-driven GC goroutine.

Effort: ~1 person-week.

### Phase 3 — Discovery API + CDN edge cache rules

- Three new HTTP endpoints (§6).
- Edge-cacheable response headers + signed body.
- CF Worker cache rules (TTL=30s for GETs, bypass for POSTs).

Effort: ~1 person-week.

### Phase 4 — Well-known + CLI tooling

- `.well-known/quidnug-network.json` static-file generator
  + signature wrapper.
- `quidnug-cli node advertise` command (build + sign + post).
- `quidnug-cli discover <domain>` command (walk the discovery
  flow for debugging).
- Optional: DID Document output mode in the discovery API.

Effort: ~1 person-week plus docs.

Total: ~3-4 person-weeks across four independently-landable
pieces.

## 13. Worked example — sharding the quidnug.com public network at scale

Hypothetical steady-state deployment after a year of adoption.

```
Operator quid: 8e1f3a... (your personal quid)
  governs: reviews.public.*, network.quidnug.com.*
  trusts as node: 12 node quids

Node quids, attested:
  quidnug-val-iad-1     validator+archive, endpoint: https://iad1.quidnug.com
  quidnug-val-iad-2     validator+archive, endpoint: https://iad2.quidnug.com
  quidnug-val-lhr-1     validator+archive, endpoint: https://lhr1.quidnug.com
  quidnug-cache-iad-1..4 cache,            endpoint: https://iad{1..4}.cache.quidnug.com
  quidnug-cache-lhr-1..2 cache,            endpoint: https://lhr{1,2}.cache.quidnug.com
  quidnug-ipfs-1        ipfs+cache,        endpoint: https://ipfs1.quidnug.com

Each publishes a NODE_ADVERTISEMENT every 6 hours.

Domain: reviews.public.technology.laptops
  Validators: {quidnug-val-iad-1, quidnug-val-iad-2, quidnug-val-lhr-1}
  EndpointHints (auto-derived from advertisements):
    validators: iad1, iad2, lhr1
    cache: iad1-cache..iad4-cache, lhr1-cache, lhr2-cache
    ipfs: ipfs1
```

Client in New York asks `api.quidnug.com` for reviews on a
laptop. The api-gateway Worker:

1. Hits `discovery/domain/reviews.public.technology.laptops`.
2. Sees three validators + six caches. Client geoip is east
   coast; prefers region=iad.
3. Routes the actual review query (`GET /api/streams/...`) to
   `iad1.cache.quidnug.com` (lowest priority + weight, cache
   capability).

A user from London gets the same flow but ends up on
`lhr1.cache.quidnug.com`.

A user submitting a review (`POST /api/events`) is routed to
`iad1.quidnug.com` (validator capability). The Worker picks
IAD over LHR because the east-coast validators are priority 1
for POSTs in the Worker's routing policy.

If `iad1.quidnug.com` is down, the Worker falls back to
`iad2.quidnug.com`, then `lhr1.quidnug.com`, then returns 503
only if all three validators are down simultaneously.

Everyone's a few HTTP hops from the right box for their query,
nothing is manually configured at the client, and the operator
can add or remove nodes by publishing new advertisements.

## 14. Lightweight participation — using the network without running a node

Not every participant needs infrastructure. The protocol has
always supported "just sign transactions and submit them,"
but it hasn't been documented as a first-class mode. Under
QDP-0014 it becomes explicit.

### 14.1 The model

An application (or operator, or end user) holds a quid but
runs no node. They use the public network's API gateway as
their sole backend. They get:

- Full cryptographic participation — their transactions are
  signed by their own key, committed to the chain, available
  to every observer.
- Reputation fungibility — same quid identity everywhere,
  same trust edges visible to everyone.
- Zero infrastructure cost — they're just an HTTP client.

They give up:

- The ability to produce blocks (they're not in any consortium).
- Offline operation (every write requires the API gateway).
- Custody of the chain (they trust the public network
  operators to host it durably).

This is the right tradeoff for 90% of use cases. An
independent reviewer building a reviews UI, a rating-system
vendor plugging into `reviews.public.*`, a CMS plugin, a
mobile app — none of them need to run a node. They need a key
and a library.

### 14.2 The flow

Minimum viable path for an app to participate:

1. **Generate a quid locally** (or let the user generate one
   via a wallet / browser extension).
2. **Register an app-specific domain** on the public network
   via the API gateway, declaring the app's quid as a governor.
   The public network's consortium validates and commits the
   registration.
3. **Sign and submit transactions** to the API gateway via
   `POST /api/events`, `POST /api/transactions/*`, etc. The
   app signs with its key; the public network validates and
   gossips.
4. **Read back via the discovery API** — per-observer trust
   queries, stream listings, aggregate ratings, whatever the
   app needs.

No node involved. The app is a pure API consumer.

### 14.3 What the app operator has to commit to

Practically identical to a node operator, just scaled down:

- **Key custody.** The app's quid key is still sensitive.
  Same principles as operator-quid custody (offline paper
  backup, guardian quorum, rotation plan).
- **Registering the domain properly.** `reviews.yourapp.com`
  or similar under the public network's top-level tree.
  Declare the app's quid as the sole governor, configure a
  `GovernanceQuorum = 1.0`, and you own the sub-tree the same
  way a large operator owns `reviews.public`.
- **Transaction rate-limits.** The public network's rate
  limits apply. For high-volume apps, this is why you
  eventually want your own node.

### 14.4 When to graduate to a node

Signals that you've outgrown lightweight-only mode:

- Your tx volume hits the public network's rate-limit ceiling.
- You want sub-100ms reads and the API gateway's RTT is too
  slow.
- You want offline operation (a mobile app that sync later).
- You want cryptographic custody of your own chain for legal /
  compliance reasons.
- You're willing to run the `home-operator-plan.md` flow.

All of these scale organically. You go from "just use the API"
→ "run a cache replica of the domains you care about" →
"become a consortium member for those domains" by following
the existing QDP-0012 promotion path.

## 15. Per-domain quid index

The inverse discovery question: "given a domain, who's active
in it?" QDP-0014 adds a per-domain quid index maintained by
every node and served via the discovery API.

### 15.1 The index structure

Each node maintains an in-memory + disk-backed map:

```
domain → {
    quid → {
        firstSeen      int64  // unix of first tx in this domain
        lastSeen       int64  // unix of most recent tx
        txCount        int64
        eventTypeCounts map[string]int64  // REVIEW: 10, HELPFUL_VOTE: 32, ...
        trustEdgesOut  int64  // count of TRUST edges they've issued
        trustEdgesIn   int64  // count of TRUST edges targeting them
    }
}
```

Populated as a side-effect of the existing block-processing
path. Storage cost is linear in (domains × active quids),
bounded by the volume of unique quids the network has seen.

### 15.2 Discovery API

```
GET /api/v2/discovery/quids?domain=<name>
    &since=<unix>                 (optional; filter to recently-active)
    &sort=activity|last-seen|first-seen|trust-weight
    &limit=<int>                  (default 50, max 500)
    &offset=<int>                 (pagination)
    &observer=<quid>              (optional; enables trust-weight sort)
    &eventType=<REVIEW|...>       (optional; filter by event type signed)
    &min-trust-weight=<0..1>      (optional; requires observer)
```

Response:

```json
{
    "domain": "reviews.public.technology.laptops",
    "pagination": { "total": 1500, "limit": 50, "offset": 0 },
    "quids": [
        {
            "quidId": "5f8a9b0000000100",
            "firstSeen": 1745100000,
            "lastSeen":  1745178293,
            "txCount":   42,
            "eventTypeCounts": { "REVIEW": 10, "HELPFUL_VOTE": 32 },
            "trustEdgesOut": 3,
            "trustEdgesIn":  17,
            "trustWeight":   0.87
        },
        ...
    ],
    "generatedAt": 1745178300
}
```

`trustWeight` appears only when `observer` is supplied — it's
the observer's relational-trust level in the listed quid in
this domain.

### 15.3 Use cases

- **App onboarding:** a new review UI wants to list the top
  trusted reviewers in a topic. Query with
  `sort=trust-weight&observer=<current-user-quid>&limit=20`.
- **Anti-spam discovery:** identify quids with suspicious
  activity patterns (high tx count, zero trust edges in).
- **Reviewer reputation pages:** show "people who've reviewed
  the most laptops."
- **Cross-network scouting (QDP-0013):** a private network
  looking for operators to federate with can query the public
  network's quid index in the relevant domain.

### 15.4 Freshness + caching

The index is a materialized view; it's eventually consistent
with block state. Acceptable staleness is domain-dependent:

- Real-time-ish (seconds): high-activity domains. Node
  rebuilds on each block commit.
- Minutes-stale: low-activity domains. Node recomputes on a
  timer.

API responses carry `generatedAt` so clients can reason about
staleness. Edge cache TTL of 60s is the default for sorted
queries.

### 15.5 Security considerations

- **Enumeration risks:** the index exposes which quids are
  active in a domain. This is public data by design — all
  transactions are public. But an app operator choosing to use
  a private domain should know their activity is visible via
  this endpoint.
- **PII:** quid IDs are not personal identifiers by themselves.
  But a quid that's publicly linked to a real person (via
  OIDC bridge binding) becomes personally identifying when
  listed in a quid index. For GDPR-sensitive deployments,
  private domains + consortium-only access to the index are
  the right answer.
- **Result-ordering bias:** sort by activity / trust-weight
  creates a feedback loop where the already-popular get more
  exposure. Applications should mix "top trusted" with
  "newest" / "random" when showing reviewer lists.
- **Operator filtering:** operators may want to exclude
  specific quids from index responses (per local moderation
  policy). The API supports `excludeQuid` param for this.

### 15.6 Alternate interface — TRUST-only index

For discovery scenarios where the relevant signal is "who does
the domain's consortium trust?" rather than "who's active?",
a second endpoint:

```
GET /api/v2/discovery/trusted-quids?domain=<name>&min-trust=0.5
```

Returns quids that the consortium has published TRUST edges
toward in the domain. Small result set; stable; highly
cacheable.

This is the answer to "show me the verified reviewers on the
public network" — a reviewer who's been explicitly blessed by
a consortium member shows up here.

## 16. Open questions

1. **Should cached responses bypass signature verification?**
   30-second edge cache means the ~1-second cost of
   signature verification amortizes across many cache hits.
   Keeping verification mandatory is the safer default;
   revisit if perf numbers demand it.

2. **DID-based identifier scheme portability.** Multiple DID
   methods could resolve the same quid. Should we register
   `did:quidnug` formally? Tentatively yes; worth waiting
   until the method is stable.

3. **Advertisement-refresh cadence default.** 6 hours is
   conservative. Too-frequent refresh bloats chain storage;
   too-infrequent means longer failure-detection delays.
   6 hours feels right for launch; adjust based on real data.

4. **Multi-operator nodes.** A single physical server could
   be attested by multiple operators (consortium
   co-operation). Currently we assume one operator per node.
   Supporting multi-operator would mean multiple
   `OperatorQuid` fields and per-operator trust checks.
   Defer until a real use case emerges.

5. **Geographic-proximity discovery.** Clients picking
   endpoints currently rely on a static `region` hint.
   A richer model could use RTT probes or
   Anycast-DNS-derived geolocation. Probably premature; the
   CDN already handles most of this for unsigned-in users.

6. **Advertisement gossip priority.** Advertisements are
   ordinary transactions at the gossip layer. Are they
   time-sensitive enough to warrant priority lanes? Probably
   not for v1; check with operational data.

## 17. Review status

Draft. Design only, no code. Needs operator review against
real operational scenarios. Specifically would appreciate
feedback on:

- The operator-to-nodes hierarchy — does the single-operator
  TRUST-edge-attestation model stand up, or do we need an
  explicit `OPERATOR_ATTESTATION` transaction?
- The capability enumeration — is `validator / cache /
  archive / bootstrap / ipfs / gossip-sink` the right set?
  What am I missing?
- The well-known file format — does it capture everything a
  new operator would want to publish, without bloating?
- Standard alignments — am I getting DID / DNS / OIDC
  patterns right?

## 18. References

- [RFC 8615: Well-Known URIs](https://www.rfc-editor.org/rfc/rfc8615)
- [W3C DID Core 1.0](https://www.w3.org/TR/did-core/)
- [OpenID Connect Discovery 1.0](https://openid.net/specs/openid-connect-discovery-1_0.html)
- [DNS SRV (RFC 2782)](https://www.rfc-editor.org/rfc/rfc2782)
- [QDP-0012 (Domain Governance)](0012-domain-governance.md)
- [QDP-0013 (Network Federation)](0013-network-federation.md)
- [`../../deploy/public-network/sharding-model.md`](../../deploy/public-network/sharding-model.md) — operator-facing version
- [`../../schemas/json/node-advertisement.schema.json`](../../schemas/json/node-advertisement.schema.json) — wire-format schema
