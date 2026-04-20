# QDP-0013: Network Federation Model

| Field      | Value                                                            |
|------------|------------------------------------------------------------------|
| Status     | Draft — design only, some changes are clarifications of existing behavior |
| Track      | Protocol + architecture                                          |
| Author     | The Quidnug Authors                                              |
| Created    | 2026-04-20                                                       |
| Requires   | QDP-0001, QDP-0003, QDP-0008, QDP-0012                           |
| Implements | Cross-network reputation fungibility + explicit statement of protocol uniformity |

## 1. Summary

The Quidnug protocol does not distinguish between "the public
network" and "a private network." Every node speaks the same
wire format, can register any domain, can run its own
consortium (QDP-0012), and can peer with any other node. The
"public network" operated under `quidnug.com` is one
configuration of the protocol — a specific consortium, a
specific set of reserved domains, a specific set of gossip
peers. Nothing at the protocol level elevates it.

This has two consequences the existing documentation didn't
make explicit:

1. **Anyone can spin up a parallel "little public network."**
   Pick a root domain, register a consortium, invite peers,
   ship. It's the same protocol; the only difference is social
   (which operators do other observers choose to trust?).

2. **Reputation is fungible across networks.** A quid
   (cryptographic identity) is global — the same public key
   yields the same quid ID on every network. Trust edges
   targeting that quid on the public network are signed
   statements anyone can verify. A private network operator can
   **import those edges as reputation inputs** to their own
   relational-trust computations without joining the public
   network's consortium.

QDP-0013 specifies:

- The architectural invariant that ensures uniformity (no
  hardcoded distinctions).
- The three mechanisms by which cross-network reputation
  fungibility works (shared-peer gossip, explicit external
  trust sources, signed re-attestation).
- A small new primitive — the `TRUST_IMPORT` transaction — that
  makes cross-network reputation explicit, auditable, and safe.
- The attack surface that opens up and how each vector is
  mitigated.

Most of this is clarification of existing behavior. The only
new protocol surface is the `TRUST_IMPORT` transaction type
and an optional `external_trust_sources` config knob.

## 2. Goals and non-goals

**Goals:**

- Ensure no code path special-cases "the public network."
- Make the protocol uniformity invariant explicit and testable.
- Define how an operator runs a private network that optionally
  benefits from public-network reputation.
- Provide a clean primitive for importing external attestations
  into a local chain without copying the entire external chain.
- Preserve the relativistic trust model: importing doesn't mean
  accepting.

**Non-goals:**

- Cross-chain atomic operations. Networks don't synchronize
  state; the protocol is gossip-over-HTTP, not cross-chain
  consensus.
- Global namespace authority. `reviews.public` on my network
  and `reviews.public` on your network are distinct domain
  objects; distinguishing them is the consumer's job.
- Currency-like transferability. "Reputation" here means
  "weight in relational-trust computations," not a stake or
  token.
- Privacy between federated networks. If you gossip, you leak.
  Private data stays private-domain and is never imported.

## 3. The uniformity invariant

The reference node SHOULD NOT hardcode any behavior that
depends on a specific domain name, operator identity, or peer
URL. Specifically:

- **Domain names are opaque strings** to the protocol. The node
  cares only about:
  - Whether it's registered in `TrustDomains`.
  - Whether the local policy (`SupportedDomains`) admits it.
  - What its consortium + governor state looks like (post-QDP-0012).
- **Operator identities are ordinary quids.** The protocol's
  notion of "the seed operator" is whoever registered a given
  domain and signed the initial governor attestation.
- **Peer URLs are config.** `seed_nodes` is a list the operator
  sets. The protocol has no preferred peers, no root-of-trust,
  no blessed authority.

This is already true in the current codebase (confirmed by a
grep for `"public"`, `"quidnug.com"`, and similar strings — no
matches in `internal/core/`). QDP-0013 makes it normative: any
future change that violates uniformity is a protocol bug.

### Test for the invariant

A CI check can grep the code for hardcoded domain names /
operator quids and fail the build if any are introduced.
Proposed addition to `ci.yml`:

```yaml
- name: Uniformity invariant
  run: |
      set -euo pipefail
      # Fail if any string-constant look like a public-network
      # reserved name made it into the implementation.
      if grep -rE '"(reviews\.public|network\.quidnug|operators\.network)' \
              internal/ cmd/ pkg/; then
          echo "uniformity invariant violated: hardcoded public-network name"
          exit 1
      fi
```

Test files + docs + examples are exempt (they legitimately need
example names).

## 4. The three federation mechanisms

A node operator participates in the public network in one or
more of three modes.

### 4.1 Shared-peer gossip (the default)

The operator points their node at public seed nodes via
`seed_nodes`. They also peer with their own private-consortium
nodes. Gossip flows between all configured peers. Trust edges
from the public network naturally land in the operator's chain
state. Any relational-trust query walks both public and private
edges transparently.

**Pros:** Zero new code. Works today. Full fidelity — the
node sees every trust edge the public gossip network sees for
domains it cares about.

**Cons:** Bandwidth + storage costs scale with public-network
activity. Hard to filter — you get everything the peer gossips
under your `SupportedDomains`.

**Best for:** Operators who want the full public-network
experience plus their own domains on the side.

### 4.2 External trust sources (new config)

The operator's node doesn't peer with the public network at
all for gossip purposes, but configures one or more external
HTTP trust sources:

```yaml
external_trust_sources:
  - url: "https://api.quidnug.com"
    domains:
      - "operators.network.quidnug.com"
      - "validators.network.quidnug.com"
    cache_ttl: "5m"
    require_tls: true
    sigverify_operator_pubkey: "<public-network-operator-pubkey-hex>"
```

When computing relational trust and the path traverses one of
the listed domains, the node **queries the external source's
HTTP API** instead of (or in addition to) its own local edges.
Results are cached for `cache_ttl`. Each response must be
accompanied by a signature from the declared operator pubkey
over the response body — unsigned or badly-signed responses
are discarded.

**Pros:** Low bandwidth. Small storage footprint. Precise
scoping (only the listed domains are consulted externally).

**Cons:** Query latency. External source is a dependency; if
it's down, trust lookups on those domains return stale cache
or fail. Requires the external source to sign responses,
which means the external operator has to opt in to being a
"reputation API" for others.

**Best for:** Private networks that want a reputation boost
from public-operator attestations but don't want to store the
full public chain.

**Implementation outline:** `ComputeRelationalTrust` already
walks TRUST edges via local registry lookups. Wrap the lookup
in a resolver interface that falls back to
`ExternalTrustSource` when the local registry misses in an
enumerated domain. Cache with a small LRU keyed on
`(observer, target, domain)`.

### 4.3 Signed re-attestation (the explicit path)

The operator explicitly re-publishes a public-network trust
edge on their own network via a new `TRUST_IMPORT` transaction.
This is distinct from gossip replication: `TRUST_IMPORT` commits
a specific external attestation to the local chain as a
first-class event that downstream tooling can audit.

```go
type TrustImportTransaction struct {
    BaseTransaction

    // The external network's URL where the original TRUST
    // edge was published. MUST be one declared in the node's
    // external_trust_sources config at tx-submission time.
    SourceNetwork string `json:"sourceNetwork"`

    // The external network's operator attestation — the public
    // key the SourceNetwork operator signs its API responses
    // with. Verified at tx acceptance.
    SourceOperatorPubKey string `json:"sourceOperatorPubKey"`

    // The original TRUST edge being imported, verbatim. The
    // BaseTransaction inside is the foreign-chain tx id +
    // signature + creator; nothing is rewritten.
    ImportedTrustEdge TrustTransaction `json:"importedTrustEdge"`

    // Your node's attestation over the import. The importer
    // signs (sourceNetwork || sourceOperatorPubKey ||
    // canonicalBytes(importedTrustEdge)) and this field
    // proves the importer did the work of fetching + verifying.
    ImporterSignature string `json:"importerSignature"`

    // Reason / memo, optional. For audit.
    Memo string `json:"memo,omitempty"`
}
```

On acceptance, the local node:

1. Verifies `ImportedTrustEdge`'s signature under its own
   embedded `PublicKey` (the original signer's key).
2. Verifies the `SourceNetwork` API claims responsibility for
   this edge (by probing
   `SourceNetwork/api/trust/import-verify?tx=<id>` and checking
   an operator signature over the response).
3. Verifies `ImporterSignature`.
4. Adds the imported edge to the local `TrustRegistry` — but
   scoped to a **domain derived from the source**:
   `imports.<sha256-of-SourceNetwork>.<original-domain>`. This
   prevents name-collisions with local domains.
5. Stores a `TrustImportRecord` for auditability.

Relational-trust computations can then traverse the imported
edge under the derived domain. Observers who want to credit
imported edges configure the `derive-from-imports-at: 0.5`
inheritance-decay factor (or similar) to control how much of
the original trust weight survives the import.

**Pros:** Explicit, auditable, cryptographically verifiable.
The imported edge has a clear provenance record. Downstream
observers can choose whether to trust the importer's judgment
in addition to the original signer.

**Cons:** Each import is a tx. Doesn't scale to bulk imports.
Best for selectively boosting specific operators' reputations
rather than mirroring a whole network.

**Best for:** "I want the public-network attestation that X is
a valid operator to count on my network, where I'm the
governor. I'll import it explicitly and sign it."

### 4.4 Combination is allowed

All three modes can run simultaneously. A node might:

- Peer with public nodes for `reviews.public.*` (mode 1).
- Use an external trust source for `operators.network.quidnug.com`
  specifically (mode 2) because it doesn't want the gossip
  overhead of the whole public chain.
- Manually import specific attestations via `TRUST_IMPORT`
  when promoting a user into a private consortium (mode 3).

The three modes are independent; conflicts are resolved by the
normal relational-trust aggregation rules (max across paths).

## 5. Global quid identity

The foundational reason federation works: quids are universal
by construction. Quid ID = `sha256(publicKey)[:16]`. The same
key material produces the same quid ID on any network. An
operator does not need a different identity for each network
they participate in.

This means:

- If operator X's quid is `a1b2c3...` on the public network, it
  is also `a1b2c3...` on every other network X participates in.
- A TRUST edge targeting `a1b2c3...` signed on one network is
  verifiable on any other (verification only needs the public
  key, which is embedded in the edge).
- Importing the edge into another network's chain is a matter
  of committing a signed record, not creating a new identity.

No additional primitive is needed for cross-network identity.
The quid-ID-from-public-key derivation has been in the protocol
since day one.

## 6. The private network lifecycle

An operator running their own network goes through:

### 6.1 Choose a root domain

Any string. No registration. Typical conventions:

- `your-org.example.com` — matches an organization's DNS.
- `private.consortium-N` — for informal networks.
- `app.domain.provider` — for private app data.

The protocol doesn't enforce DNS-like hierarchy. `a.b.c` and
`c.b.a` are unrelated domain names unless the operator
explicitly configures them.

### 6.2 Register the domain + consortium + governors

Same CLI as the public network:

```bash
quidnug-cli domain register \
    --name "my-org.example.com" \
    --validators "my-node-1:1.0" \
    --governors "my-personal-quid:1.0" \
    --governance-quorum 1.0 \
    --threshold 0.5
```

Nothing about this registration differs from the public-network
registration. It's the same transaction, same validation, same
on-chain record.

### 6.3 Optionally peer with the public network

Add public seeds to `seed_nodes`. The node gossips with them.
Blocks under your private domain are NOT gossiped to them
(their `SupportedDomains` would reject them anyway), and their
public-chain blocks are NOT gossiped to your private-only peers.

### 6.4 Optionally federate reputation

Configure `external_trust_sources` or start issuing
`TRUST_IMPORT` transactions for specific operators you want
public reputation to carry weight for.

### 6.5 Operate independently

Your network has its own governance, its own chain history, its
own validator rotation cadence. The public network can go down
and yours keeps working. Your network can fork and the public
network doesn't care. This is the point.

## 7. Attack vectors and mitigations

### 7.1 Fake public-network impersonation

**Attack:** Attacker runs a network that claims to be "the
public network" by registering the same domain names
(`reviews.public`, etc.) and pointing victims at
`api.attacker.example.com` for lookups.

**Mitigation:** Network identity is the **operator pubkey + the
URL the operator publishes at**. Victims don't look up "the
public network"; they look up "the network run by operator X
at URL Y." `external_trust_sources` requires
`sigverify_operator_pubkey` — a mis-configured victim who
points at an attacker gets attacker responses, but correctly
configured victims (using the real operator's published
pubkey) verify signatures and reject forgeries.

Additionally, `seeds.json` on the real operator's site is
signed by the real operator's quid. Anyone federating with the
public network should first fetch + verify that file.

### 7.2 Import poisoning via compromised external source

**Attack:** The public network's API is compromised and serves
false responses signed by a compromised operator key.

**Mitigation:** The importer's node verifies the original
TRUST edge's signature under the ORIGINAL signer's embedded
pubkey (step 1 of §4.3). An attacker with only the external
operator's key cannot forge a signed TRUST edge from an
unrelated quid — they'd need that quid's private key too. The
`ImportedTrustEdge.Signature` field is verified against
`ImportedTrustEdge.PublicKey`, not against the external
operator's key.

Additionally, `TRUST_IMPORT` transactions are on-chain and
human-reviewable; a burst of suspicious imports gets flagged
by operational monitoring.

### 7.3 Private-chain exfiltration via federation

**Attack:** An operator peers with the public network,
intending to participate only in `reviews.public.*`, but their
node accidentally gossips private-domain blocks to public
peers.

**Mitigation:** Gossip is filtered per-domain at send time.
The node only gossips blocks for domains its peer's
`SupportedDomains` admits. Private blocks stay private as long
as the operator's `SupportedDomains` on the private side
doesn't include the public network's pattern.

This is already implemented. A belt-and-braces test in CI
verifies the gossip filter correctness.

### 7.4 Reputation-laundering

**Attack:** Operator X has bad reputation on the public
network. They run a private network, register themselves as a
consortium member there, issue glowing trust edges from their
private alter-ego quids, then try to import those edges back
into the public network to launder reputation.

**Mitigation:** Imports flow ONE DIRECTION — from the external
network into the local chain. The public network doesn't
auto-import from arbitrary private networks. An operator who
wants to use private-network attestations needs to explicitly
configure that private network as a trust source, which
requires trusting its operator. For public-network consumers,
the default is to trust only the public network's own
attestations.

Additionally, imported edges are namespaced under
`imports.<source-hash>.<domain>` — they never collide with the
native domain, so downstream relational-trust computations can
distinguish them and apply different inheritance-decay factors.

### 7.5 Sybil identity across networks

**Attack:** Attacker generates many quids on a private
network, self-trusts them all, and exports them to the public
network.

**Mitigation:** The public network doesn't auto-import. For
any quid to gain reputation on the public network, some
public-network operator has to `TRUST` them (directly or
transitively). Self-attestation on a private network is
worthless to the public network unless a public-network
operator independently vouches.

### 7.6 Gossip-storm amplification

**Attack:** A malicious node peers with the public network and
replays old trust edges at high volume, hoping to cause
bandwidth burn or confuse block inclusion.

**Mitigation:** Existing rate-limits + nonce-based replay
prevention (QDP-0001) discard the replays before inclusion.
Per-peer gossip rate-limits throttle the attacker's
connection; at threshold, the public network operators revoke
the peering edge per the peering protocol.

### 7.7 Domain-name squatting

**Attack:** An attacker runs a network and registers
`reviews.public` on it, producing a parallel chain. Honest
operators who federate by name might import attacker data.

**Mitigation:** Federation is not name-based; it's URL +
operator pubkey based. The attacker's `reviews.public` on
their node is distinct from the public network's
`reviews.public` because federation goes through a specific
URL + verified operator signature. Anyone federating by name
alone is self-harming — the protocol can't protect against
misconfiguration, only document the right way to configure.

## 8. Compatibility with existing QDPs

| QDP | Interaction |
| --- | --- |
| 0001 (Nonce ledger) | `TRUST_IMPORT` gets a new tx type slot; nonce scoping per importer quid. |
| 0002 (Guardian recovery) | Unaffected. Quids rotate the same way regardless of which network they're attested on. |
| 0003 (Cross-domain nonce scoping) | Imported edges live in `imports.<source-hash>.<domain>` which is a separate domain for nonce purposes. |
| 0007 (Lazy epoch propagation) | The original TRUST edge's signer epoch is verified at import; later epoch rotations are the original network's responsibility to gossip. Importing network may see stale epoch state — fine, it's a per-observer concern. |
| 0008 (K-of-K bootstrap) | New nodes bootstrap from their own network's peers; they do NOT bootstrap across network boundaries. An operator federating two networks runs two bootstraps. |
| 0009 (Fork-block) | Federation is a per-node config concern; activating a fork on network A doesn't affect network B. |
| 0010 (Merkle proofs) | Imports are ordinary txs; merkle-proof inclusion works the same way. |
| 0012 (Domain governance) | Imported edges do NOT grant consortium membership. They're only inputs to relational trust. Joining a consortium is strictly a local governance action. |

## 9. Implementation plan

Four small pieces, all low-risk.

### Phase 1: CI uniformity invariant

Add the grep-based check to `ci.yml` (see §3). Five-minute
task, prevents future regressions.

### Phase 2: External trust source config

Add `external_trust_sources: []` to the YAML config schema.
Implement the HTTP resolver with signature verification + LRU
cache. Wrap `ComputeRelationalTrust`'s edge-lookup call in a
resolver chain (local first, then configured external
sources).

Effort: ~1 person-week plus tests.

### Phase 3: `TRUST_IMPORT` transaction

Add `TxTypeTrustImport` and the new struct. Implement:

- `ValidateTrustImportTransaction` — signature checks (inner +
  outer + external-source verification).
- Registry handler that adds imported edges to the
  `imports.<hash>.<domain>` namespace.
- CLI command `quidnug-cli trust import --source <URL> --tx <foreign-tx-id>`.

Effort: ~1.5 person-weeks plus tests.

### Phase 4: Documentation + tutorials

Operator-facing how-to for each of the three federation modes.
Already underway via `deploy/public-network/federation-model.md`
(companion to this QDP).

## 10. Worked example — someone runs their own little public network

**Scenario:** A group of hardware reviewers decides to run their
own public reviews network. They call it `reviews.hardware.coop`.
They want it to look and feel like the main `reviews.public.*`
network, but run by and for themselves, with optional
reputation crossover.

### Step 1: Register their root

```bash
# On reviews-coop-seed-1, the group's primary node:
quidnug-cli domain register \
    --name "reviews.hardware.coop" \
    --validators "coop-seed-1:1.0,coop-seed-2:1.0,coop-seed-3:1.0" \
    --governors "coop-chair:1.0,coop-treasurer:1.0,coop-tech-lead:1.0" \
    --governance-quorum 0.67

# Plus child topics:
for child in \
    reviews.hardware.coop.laptops \
    reviews.hardware.coop.cameras \
    reviews.hardware.coop.keyboards; do
    quidnug-cli domain register --name "$child" ...
done
```

Structurally identical to what the main operator did.

### Step 2: Peer with the main public network

```yaml
# coop-seed-1 config
seed_nodes:
    - "api.quidnug.com"             # the main public network
    - "coop-seed-2.reviews.coop"
    - "coop-seed-3.reviews.coop"
supported_domains:
    - "reviews.hardware.coop"
    - "reviews.public"               # read-only cache the main public tree
    - "operators.network.quidnug.com"  # read for federation
    - "network.quidnug.com"
```

Now the coop's node is a cache replica for the main public
tree AND runs its own consortium for `reviews.hardware.coop.*`.

### Step 3: Federate main-network operator reputation

```yaml
# coop-seed-1 config — reputation-boost pathway
external_trust_sources:
    - url: "https://api.quidnug.com"
      domains:
          - "operators.network.quidnug.com"
      sigverify_operator_pubkey: "<main-operator-pubkey-hex>"
      cache_ttl: "15m"
```

Now when someone registers as a hardware reviewer on the coop
network, the coop's relational-trust computation can factor in
their `operators.network.quidnug.com` standing on the main
public network (at a configurable inheritance-decay factor, say
0.6).

### Step 4: Explicit cross-network trust boost for a specific reviewer

A respected laptop reviewer, "jo," has been running on the
main public network for a year and earned direct trust edges
from multiple public-network operators. The coop's chair wants
to give them instant credibility on the coop network:

```bash
quidnug-cli trust import \
    --source https://api.quidnug.com \
    --foreign-tx <id-of-a-public-trust-edge-targeting-jo> \
    --memo "welcoming jo to the hardware coop with full standing" \
    --sign-with coop-chair.key.json
```

This posts a `TRUST_IMPORT` transaction on the coop's chain.
The imported edge lives at
`imports.<sha256-of-api.quidnug.com>.operators.network.quidnug.com`,
visible in the coop's local trust graph. The coop's
`ComputeRelationalTrust` walks it when computing trust in jo
from the chair's perspective.

### Step 5: Operate autonomously

The coop continues running their own network. They can take
their own governance decisions, rotate their own keys, admit
new consortium members, revoke misbehavers. The main public
network is a reputation source they've federated with — not a
parent, not a boss. If the coop decides the main network has
gone off the rails, they can remove the federation with a
config change and continue operating independently.

## 11. Open questions

1. **Should `external_trust_sources` support discovery via
   DNS?** Currently the config is a flat URL list. A DNS-TXT
   discovery mechanism could let operators announce their
   trust-API endpoints under their own domains. Deferred —
   simpler is better at launch.

2. **Rate-limit for `TRUST_IMPORT` transactions.** A node
   operator could spam imports to bloat their chain. Default
   rate-limit of 60/minute (same as other mutation types) is
   probably adequate; revisit after operational data.

3. **Should imported edges expire?** Imports are signed records
   of an external attestation at a point in time; the external
   attestation itself may later be revoked or superseded on its
   home network. We currently do not track revocation. Option
   for a future QDP: a `TRUST_IMPORT_REVOKE` counter-tx that
   marks imports as stale.

4. **Federation graph topology.** Networks that federate with
   each other that federate with each other could form complex
   reputation webs. Should there be a depth limit on federation
   traversal? Tentative answer: the default inheritance-decay
   factor already bounds this; an edge federated through three
   hops of networks has weight 0.6³ = 0.216 at most. Revisit if
   operational experience shows this isn't enough.

5. **Signed peering attestations between federated networks.**
   Currently, the coop's federation of the main network is a
   one-way config choice. Could there be a bilateral signed
   `NETWORK_PEERING` record showing mutual awareness? Useful
   for building a discoverable graph of federated networks.
   Probably worth a small future addition.

## 12. Review status

Draft. This QDP is primarily documentation of existing
behavior (most of §3 and §4.1 work today). The new code
surface is:

- CI invariant check (trivial).
- `external_trust_sources` config + resolver (small).
- `TRUST_IMPORT` transaction + CLI (moderate).

Each is independently shippable. All three together bring
reputation fungibility to a shipping state.

## 13. References

- [QDP-0012 (Domain Governance)](0012-domain-governance.md) — role separation that federation builds on.
- [`../../deploy/public-network/federation-model.md`](../../deploy/public-network/federation-model.md) — the operator-facing summary.
- [`../../deploy/public-network/governance-model.md`](../../deploy/public-network/governance-model.md) — the companion doc for QDP-0012.
- [QDP-0001 (Nonce Ledger)](0001-global-nonce-ledger.md) — replay protection for all tx types.
- [QDP-0003 (Cross-Domain Nonce Scoping)](0003-cross-domain-nonce-scoping.md) — nonce isolation across domains, applied to imported edges.
