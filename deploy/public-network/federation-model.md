# Federation model — one protocol, many networks

> The plain-English companion to
> [QDP-0013](../../docs/design/0013-network-federation.md).
> Reads in about 10 minutes. Start here if you want to
> understand how Quidnug's uniformity works, how to run your
> own network, and how to borrow reputation from another one.

## The principle in one sentence

**There is no "the public network" at the protocol level** —
the main public network at `quidnug.com` is one configuration
of Quidnug, and your own network can look structurally
identical with a different set of operators, domains, and
peers.

This is not a feature we "added"; it's a property of how the
protocol was built. The reference node has no hardcoded
preference for any operator, any peer URL, or any domain name.
A grep for `"public"` or `"quidnug.com"` in `internal/core/`
returns zero matches in production code paths.

## What the main public network actually is

Four things, all of which you can reproduce:

1. **A set of operator-held quids** — the governor quorum for
   the public domain tree.
2. **A set of node-held quids** — the consortium that produces
   blocks for the public domain tree.
3. **A reserved naming convention** — `reviews.public`,
   `network.quidnug.com`, `operators.*`, etc. Names that
   observers widely agree refer to the `quidnug.com` operator's
   setup.
4. **A social commitment** — the operator published
   `seeds.json` saying "these are my nodes, these are my
   governors, here's my public API at `api.quidnug.com`."

Nothing in the protocol stops you from picking a different
name, running different nodes, declaring different governors,
and publishing your own `seeds.json`. The only reason
`quidnug.com`'s network has users is that some users chose to
trust it; your network can get users the same way.

## Why this matters

Three consequences worth internalizing before you start
building.

### You can run a parallel public network

Pick a root domain — literally any string. Something like
`reviews.hardware.coop` or `opinions.myclub.example` or
`attest.ourcompany.internal`. Register it with your own
consortium and governor set. Invite peers who want to
participate. You now have a network structurally identical to
the main public one, operating independently.

Users of your network get all the same primitives: trust-
weighted reviews, identity verification, guardian recovery,
key rotation, consortium membership. They're just playing on
your network's trust graph instead of the main public one.

### You can run a fully private network

Same as above, but don't peer with anyone outside your trusted
circle. Configure `supported_domains` to include only your
internal domain names. Configure `seed_nodes` to include only
your own nodes. The protocol works identically; you just
aren't connected to anyone else's gossip.

### You can borrow reputation from any network to any network

A quid (cryptographic identity) is the same on every network —
quid ID is derived from the public key and nothing else. If
operator Alice has earned trust edges on the main public
network targeting her quid, those edges are signed data anyone
can verify. Your network can credit her trust without
requiring her to re-earn it.

The mechanism for this is the focus of §3 below.

## Three ways to participate in someone else's network

Concrete recipes for each.

### 3.1 Peer directly (full gossip)

**What it means:** Your node talks to the other network's
nodes. Blocks under domains you both support flow between you.
Trust edges flow. The full experience.

**Config:**

```yaml
seed_nodes:
    - "api.quidnug.com"                # the main public network
    - "your-internal-node-1.local"
    - "your-internal-node-2.local"

supported_domains:
    - "reviews.public.*"               # main network's tree
    - "network.quidnug.com"            # main network's meta-domains
    - "your-private.*"                 # your own tree

allow_domain_registration: true
```

**Pros:** Zero new code. Full fidelity.

**Cons:** Bandwidth scales with the main network's activity.
You ingest every TRUST edge and EVENT the main gossip layer
pushes under the domains you've subscribed to.

**Best for:** You want to BE part of the main public network,
just with your own domains on the side.

### 3.2 Configure an external trust source (read-only lookup)

**What it means:** Your node does NOT peer with the main
network for gossip. Instead, when your relational-trust query
reaches a domain you've marked external, it fetches the
answer over HTTPS from the other network's API.

**Config (planned; see QDP-0013 for implementation status):**

```yaml
external_trust_sources:
    - url: "https://api.quidnug.com"
      domains:
          - "operators.network.quidnug.com"
          - "validators.network.quidnug.com"
      # The operator's pubkey used to sign API responses.
      # Fail hard if a response isn't signed by this key.
      sigverify_operator_pubkey: "<main-operator-pubkey-hex>"
      cache_ttl: "15m"
      require_tls: true
```

**Pros:** Low bandwidth + storage. Precise scoping — only the
listed domains are queried externally. Leaves your network
autonomous.

**Cons:** Query latency hits the remote API. Dependency — if
the external source goes down, external lookups fail (local
trust computations in your own domains still work fine).

**Best for:** Your private network needs the reputation signal
from a few specific public domains without the gossip overhead.

### 3.3 Import specific trust attestations (on-chain record)

**What it means:** You explicitly commit a `TRUST_IMPORT`
transaction to your own chain that references a specific
foreign trust edge. Your chain now carries a signed record
saying "this attestation from network X counts here."

**Command (planned; see QDP-0013 for implementation status):**

```bash
quidnug-cli trust import \
    --source https://api.quidnug.com \
    --foreign-tx <id-of-foreign-trust-edge> \
    --memo "respected laptop reviewer, importing their main-network standing" \
    --sign-with coop-chair.key.json
```

The imported edge lands in a derived namespace like
`imports.<hash>.operators.network.quidnug.com` on your chain.
Your relational-trust computations can walk it, typically at a
decay factor like 0.6 (configurable).

**Pros:** Explicit. Auditable. Cryptographically verifiable.
Doesn't require a gossip link to the other network.

**Cons:** One import = one transaction. Doesn't scale to bulk.
Best for selective reputation boosts, not wholesale migration.

**Best for:** You've identified a specific external operator
whose track record you want to credit on your network. You're
making a deliberate governance decision.

### The three are composable

Nothing stops you from:

- Peering with the main public network for everything under
  `reviews.public.*` (mode 1)
- Using an external trust source for
  `operators.network.quidnug.com` specifically (mode 2)
- And explicitly importing the occasional high-value
  attestation via `TRUST_IMPORT` (mode 3)

All at once. The modes are orthogonal; the protocol's
relational-trust computation naturally combines them via the
usual max-across-paths rule.

## How reputation actually becomes fungible

The simplest concrete example.

### The setup

- **Main public network** at `quidnug.com`. Operator `O1`.
- **Your coop network** at `reviews.hardware.coop`. You're the
  operator.
- **Reviewer Jo** has a quid `j7k4...`. Jo is a well-regarded
  laptop reviewer on the main public network. There are
  multiple TRUST edges targeting Jo in the
  `operators.network.quidnug.com` domain on the main network's
  chain.

### Without federation

Jo shows up on your coop network and wants to post reviews
under `reviews.hardware.coop.laptops`. Jo's quid is the same
as on the main network (`j7k4...`), but nobody on your network
knows them. Your users see their reviews with very low weight
— they have no local trust edges.

### With federation (any of the three modes)

Your coop network's relational-trust computation traverses the
imported / gossiped / fetched edges targeting `j7k4...` in the
`operators.*` domain. Even at a discounted inheritance factor
(say 0.6), those edges produce meaningful weight — maybe 0.5
or 0.4 on a 0-1 scale. Your users see Jo's reviews with
substantial weight from the first day Jo shows up.

Jo did not have to rebuild their reputation on your network.
They brought their public-network reputation with them,
quantifiable, cryptographic, and verifiable.

That's what "fungible reputation" means in practice.

## Attack scenarios at the federation layer

Quick mental model of what can and can't happen.

### "Someone spins up a fake main public network"

They can't affect your correctly-configured node. Federation
is URL + pubkey based, not name based. Your
`external_trust_sources` entry says "the operator at
`api.quidnug.com` signs responses with this specific pubkey" —
a fake network at a different URL gets ignored; a fake
response at the right URL fails signature verification.

Mitigation depends on correct configuration. Document the
pubkey clearly on `quidnug.com/network/seeds.json` and tell
federating operators to pin it.

### "My private trust edges leak to the public network"

They don't — unless you configured your `supported_domains` to
include your private domains AND peered with public nodes.
Gossip is filtered per-domain at send time. A default config
that lists only your private domains in `supported_domains`
never gossips them outside your consortium.

### "Someone launders bad reputation through a fake network"

They'd need to:
1. Spin up a fake network.
2. Earn trust there.
3. Convince your network to federate with their fake network.

Step 3 is the defense: federation is a per-operator choice.
You don't auto-federate with random networks. An attacker who
convinces you to point `external_trust_sources` at their
network is a social engineering win, not a protocol failure.

### "The main public network API gets compromised"

An attacker with the operator's key could sign malicious
responses. But the TRUST edges inside those responses are
signed by the ORIGINAL signer's key, not the operator's —
which the attacker doesn't have. So the attacker can't forge
a "jo is trusted by everyone" response; at best they can
replay old responses or withhold answers. Replay gets caught
by nonce / timestamp checks; withholding degrades the API's
usefulness but doesn't allow forgery.

Plus, large federated networks should notice the main network
behaving oddly (monitoring, operator attestations).

### "Federation depth explosion"

Network A federates with B; B federates with C; C federates
with A. Trust edges loop forever.

Prevented by two layers:

1. Inheritance-decay — weight multiplies by a fraction every
   federation hop, so cycles converge.
2. Depth limits in the relational-trust BFS — default max
   depth of 5 hops, including federation traversals.

## What you (the original public-network operator) should commit to

If you run a network that other networks federate with, you
take on a soft obligation to be a stable, honest reputation
source. Concretely:

1. **Publish your operator pubkey unambiguously.** In
   `seeds.json`, on the website, in documentation. Pinning is
   the federating operator's responsibility, but you make it
   easy for them.
2. **Sign API responses with that pubkey.** The
   `external_trust_sources` mechanism requires it.
3. **Rotate keys carefully.** When you rotate (guardian
   recovery or scheduled anchor rotation), federating
   operators need to see the new key before they can verify
   responses signed by it. Publish rotation announcements well
   ahead of activation.
4. **Don't suddenly revoke large populations of attestations.**
   Federating networks have built reputation on your edges.
   Mass revocation destabilizes them. Use individual revocations,
   announced in advance.
5. **Run a transparency log.** Your `TRUST` and
   `DOMAIN_GOVERNANCE` transactions are already on-chain;
   surface them on your website as a queryable browse
   interface. See `quidnug.com/network/transactions` (planned).

These aren't protocol-enforced; they're operator etiquette.
But getting them wrong destroys federating operators' trust in
your network as a reputation source.

## Quickstart — running your own network

The five commands from zero to running your own public-style
network:

```bash
# 1. Generate your root-operator key offline, save it to paper.
quidnug-cli keygen --out ~/.quidnug/my-operator.key.json

# 2. Generate your seed-node keys (one per node).
quidnug-cli keygen --out ~/.quidnug/seed-1.key.json
quidnug-cli keygen --out ~/.quidnug/seed-2.key.json

# 3. Start nodes (use the home-operator-plan.md for details).
# Configure each node's YAML to include your chosen supported_domains.

# 4. Register your root domain from seed-1.
SEED_QUIDS="$(./seed-1-quid),$(./seed-2-quid)"
GOV_QUID="$(./my-operator-quid)"
quidnug-cli domain register \
    --name "my-network.example.com" \
    --validators "${SEED_QUIDS}" \
    --governors "${GOV_QUID}:1.0" \
    --governance-quorum 1.0 \
    --threshold 0.5

# 5. (Optional) Federate with the main public network for reputation.
# Edit seed-1's config to add external_trust_sources,
# OR add api.quidnug.com to seed_nodes for full gossip.
```

Publish your own `seeds.json` attestation at a URL on your
domain. Invite peers. You're running a public Quidnug network.

## Further reading

- [QDP-0013 formal spec](../../docs/design/0013-network-federation.md)
  — protocol details, attack-vector analysis, implementation
  plan.
- [QDP-0012 (Domain Governance)](../../docs/design/0012-domain-governance.md)
  — the cache / consortium / governor role separation that
  applies to every network.
- [`governance-model.md`](governance-model.md) — operator-facing
  version of QDP-0012.
- [`home-operator-plan.md`](home-operator-plan.md) — specific
  deployment plan for the main `quidnug.com` public network.
  Structurally identical for your own network, different
  names / keys / seeds.
- [`peering-protocol.md`](peering-protocol.md) — the convention
  for bilateral trust agreements between networks.
