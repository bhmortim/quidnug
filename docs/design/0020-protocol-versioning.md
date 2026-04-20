# QDP-0020: Protocol Versioning & Deprecation

| Field      | Value                                                            |
|------------|------------------------------------------------------------------|
| Status     | Draft — design only                                              |
| Track      | Protocol                                                         |
| Author     | The Quidnug Authors                                              |
| Created    | 2026-04-20                                                       |
| Requires   | QDP-0009 (fork-block activation), QDP-0014 (discovery)           |
| Implements | Semantic versioning + negotiated features + deprecation timelines |

## 1. Summary

Twenty QDPs in, the protocol has accumulated real surface area.
Block format, transaction types, HTTP API, JSON-LD schemas,
governance primitives, federation, discovery. Every addition
has shipped on its own activation plan (fork-block activation
per QDP-0009, shadow phases, optional config flags, etc.). That
works for additions; it doesn't give a good answer for:

- **How does a new operator know which protocol version a peer
  speaks?** Node A expects QDP-0014 discovery endpoints; Node B
  was last updated before QDP-0014 shipped.
- **How do we deprecate something?** If QDP-0025 obsoletes
  QDP-0005's push-gossip format, how do operators coordinate
  retirement?
- **How do clients negotiate capabilities?** The JS client uses
  the v2 mixin for guardians + gossip + merkle. What about a
  future v3?
- **How do we retire a QDP cleanly?** QDPs 0001-0014 were
  additive. Some future QDP is going to require saying "this
  message format is no longer valid, transition by date X."

QDP-0020 introduces three mechanisms:

1. **Semantic protocol versioning** — every release gets a
   version number, every API response carries it, every
   `NODE_ADVERTISEMENT` declares it.
2. **Capability negotiation** — peers discover which QDPs each
   other implement via structured capability lists.
3. **Deprecation timelines** — a well-defined process for
   retiring old features that gives operators warning +
   observability before anything breaks.

## 2. Goals and non-goals

**Goals:**

- A coherent story for "this node speaks these versions + these
  features."
- Clean separation between SemVer-major (breaking), SemVer-minor
  (additive), and SemVer-patch (bugfixes).
- Graceful deprecation with multi-month runway for operators.
- Cross-version compatibility matrix for the SDK ecosystem.
- No new transaction types — versioning is metadata + config.

**Non-goals:**

- Retroactive version retrofitting. Current QDPs 0001-0019 are
  grandfathered into the 2.x series; pre-2.0 isn't discussed.
- Marketing-driven versioning. Version bumps happen on technical
  grounds, not "we're launching a shiny new thing."
- Forced upgrades. Operators decide when to upgrade; the
  protocol's grace periods make sure decisions have long lead
  times.

## 3. Semantic versioning

### 3.1 Format

Protocol version follows SemVer:

```
major.minor.patch
```

with optional pre-release / build metadata:

```
2.1.0
2.1.0-rc.1
2.1.0+build.42
```

### 3.2 What each bump means

**Major (X.0.0):** breaking wire-format change that old nodes
can't handle. Examples would include:

- Rewriting the block header format
- Replacing ECDSA P-256 with a different signing scheme
- Removing a transaction type entirely
- Changing the canonical-bytes derivation rules

None of QDPs 0001-0019 is major. Quidnug currently targets
major = 2 (the post-Phase-H consolidation); a future 3.0 would
be substantial, multi-year planning.

**Minor (2.X.0):** additive feature that older nodes ignore
cleanly. New transaction types, new HTTP endpoints, new
optional config keys, new capabilities. QDPs 0011-0019 would
each bump minor.

**Patch (2.1.X):** bugfix or clarification that doesn't change
behavior. Wire-format identical to prior patch.

### 3.3 Current version

At QDP-0020 time:

- Major: 2
- Minor: 14 (QDPs 0011-0014 all shipped as minor bumps from 2.10)
- Patch: 0

Pre-1.0 versions were the experimental drafts before QDP-0001
landed. 1.x was "phase H" (QDPs 0001-0010 iteratively). 2.0
was the consolidation. Going forward: 2.15.0 = QDP-0015
(moderation) shipped; 2.16.0 = QDP-0016; etc.

## 4. Version declaration surface

### 4.1 In HTTP responses

Every node response includes:

```
X-Quidnug-Protocol-Version: 2.14.0
X-Quidnug-Features: 0001,0002,0005,0007,0008,0009,0010,0014
```

Non-upgraded clients ignore; newer clients use it for
compatibility checks.

### 4.2 In `NODE_ADVERTISEMENT`

QDP-0014's advertisement already has `ProtocolVersion`. Per
QDP-0020, this is formalized:

```json
{
    "protocolVersion": "2.14.0",
    "supportedFeatures": ["0001", "0002", "0005", ..., "0014"],
    "deprecated": ["0005-gossip-v1"]
}
```

Clients picking nodes via discovery can filter by version or
by feature.

### 4.3 In `.well-known/quidnug-network.json`

Each operator declares their minimum-supported version:

```json
{
    "protocolMinVersion": "2.14.0",
    "protocolRecommendedVersion": "2.16.0",
    "supportedFeatures": ["0001..0014", "0015-shadow", "0016"]
}
```

Operators who lag behind the recommended version get a warning
in the operator dashboard; they're not forced to upgrade but
clients can detect stale operators.

### 4.4 Feature codes

Each QDP gets a four-digit feature code (its QDP number) for
compact listing. Special codes:

- `NNNN-shadow` — feature is shipped but enforcement is
  behind a fork-block activation
- `NNNN-deprecated` — feature is scheduled for removal (§6)
- `NNNN-removed` — no longer supported; operators running the
  feature will see errors

## 5. Capability negotiation

### 5.1 The handshake

When two nodes begin gossiping, they exchange capability lists
in the first message:

```
POST /api/v2/gossip/handshake
{
    "protocolVersion": "2.14.0",
    "supportedFeatures": ["0001","0002","0005","0007","0008","0009","0010","0014"],
    "quidnugImpl": "official-go/2.14.0"
}
```

Response:

```
{
    "protocolVersion": "2.13.0",
    "supportedFeatures": ["0001","0002","0005","0007","0008","0009","0010"],
    "negotiatedCommon": ["0001","0002","0005","0007","0008","0009","0010"],
    "negotiatedVersion": "2.13.0"
}
```

The older node says "I can't do 0014 (discovery), but I can do
everything up to 0013." The pair settles on the common subset.

### 5.2 Graceful fallback

For a feature A's request to a peer that doesn't support A,
the requester either:

- Falls back to a pre-A mechanism, or
- Returns a clear error to the caller ("peer doesn't support
  feature A")

Current QDPs are additive, so fallbacks are usually possible.
When a fallback isn't available (rare), the error message
includes:

```
X-Quidnug-Feature-Missing: 0014
Upgrade recommended to protocol 2.14.0+
```

### 5.3 Client library version matrix

Each first-party SDK publishes a compatibility matrix:

| SDK version | Min protocol | Max tested | Features |
|---|---|---|---|
| `@quidnug/client 2.0.x` | 2.0.0 | 2.10.0 | 0001-0010 |
| `@quidnug/client 2.1.x` | 2.0.0 | 2.14.0 | 0001-0014 (via v2 mixin for 0011-0014) |

Clients query the target node's version first, then choose
compatible code paths.

## 6. Deprecation lifecycle

### 6.1 The four-stage timeline

When a feature is slated for removal:

**Stage 1: Marked deprecated.** Feature continues to work. Every
call emits a warning metric and header:

```
X-Quidnug-Deprecated-Feature: 0005-gossip-v1
X-Quidnug-Sunset-Date: 2027-04-01T00:00:00Z
```

Operators see `quidnug_deprecated_calls_total{feature="0005"}`
in metrics + a Grafana dashboard panel.

**Stage 2: Warn-on-use.** Next minor version. Feature still
works but emits loud warnings on every invocation. Log entries
+ alert-ready metric.

**Stage 3: Default-disabled.** Feature disabled in the default
config. Operators who need it explicitly re-enable via
`legacy_features.<feature-code>: true`. A fork-block activation
migrates the default.

**Stage 4: Removed.** Feature deleted from the codebase. Nodes
claiming to support it advertise it as `NNNN-removed`.

### 6.2 Minimum timeline

Total: **minimum 18 months from stage 1 to stage 4**:

- 6 months warning at Stage 1
- 6 months vocal warning at Stage 2
- 3 months default-off at Stage 3
- 3 months finalization at Stage 4

Shorter timelines allowed only for:

- Security-forced deprecations (if a feature has a
  cryptographic flaw, it can be deprecated on a 30-day timeline)
- Legally-forced deprecations (regulator mandate with specific
  deadlines)

In both accelerated cases, the accelerated timeline is itself
a QDP amendment.

### 6.3 Community review

Every deprecation goes through a specific QDP — the amendment
to the original QDP that introduced the feature. The amendment
documents:

- Why the feature is being removed
- What replaces it (required)
- Migration instructions for operators
- Sample code changes for SDK users

Operators can object to a deprecation timeline via the same
channel any QDP is debated. Bias toward "extend the timeline"
— breaking changes are socially expensive.

## 7. Backward compatibility promises

### 7.1 Data at rest

Blocks produced under older protocol versions remain valid
forever. A node running 3.0 in 2030 still accepts blocks from
2.0 in 2026 as part of its chain history.

### 7.2 SDKs

Client libraries maintain backward compatibility across one
major version line. `@quidnug/client` 2.20 still works with
a 2.0 node for any feature the 2.0 node supported; new
features gracefully degrade.

Breaking changes at SDK level happen only when the protocol
itself does (new major version).

### 7.3 HTTP API

Endpoints added in minor versions stay available. Endpoints
renamed or reshaped follow the deprecation lifecycle (§6).

### 7.4 JSON schemas

Schema changes that add optional fields are minor; changes
that add required fields or change types are major. Legacy
clients can always serialize and deserialize old-shape
transactions.

## 8. Fork-block vs protocol-version

QDP-0009 (fork-block activation) is orthogonal to this QDP.
Relationship:

- **Protocol version** = what a node advertises its code can
  do.
- **Fork-block activation** = when specific rules are enforced
  across the network.

A node might run protocol 2.14.0 (supports feature 0014)
without having activated feature 0014's enforcement fork. The
feature is known; its rules aren't yet mandatory.

Deprecation via fork-block: Stage 3 of the deprecation timeline
(§6.1) can use QDP-0009 to coordinate the default-off
activation across operators.

## 9. Release workflow

### 9.1 Release cadence

- **Minor releases:** quarterly. Each ships 1-3 QDPs worth of
  features.
- **Patch releases:** as-needed. Bugfixes, clarifications,
  security patches.
- **Major releases:** every 2-3 years, only when breaking
  changes are truly necessary.

### 9.2 Release checklist

Per `deploy/release-playbook.md` (to be written):

1. All QDPs targeting this release are `Landed` status.
2. Backward-compat tests pass against the previous minor version.
3. SDK changelogs updated.
4. `@quidnug/client` released with matching version.
5. `quidnug-network.json` schema updated if `protocolMinVersion` changed.
6. `quidnug.com/protocol/versions` page updated.
7. Docker image tagged with the new version.
8. Helm chart version bumped.
9. Deprecation warnings advanced (stage 1 → stage 2 for any
   scheduled items).

### 9.3 Pre-release testing

- Internal dogfooding on the main public network's
  infrastructure.
- One-week public pre-release period where operators can
  install the release candidate and run it against test
  domains.
- At least one federated peer must successfully handshake on
  the new version before GA.

## 10. Interaction with existing QDPs

| QDP | Versioning interaction |
|---|---|
| 0001 | Nonce ledger is 2.x core; no changes expected |
| 0002 | Guardian recovery is 2.x core |
| 0009 | Fork-block handles per-feature activation within a minor-release |
| 0011 | Client SDKs track protocol version via their own SDK versioning |
| 0014 | `NODE_ADVERTISEMENT` declares version + features |
| 0015-0019 | Each adds to the feature code list |

## 11. Attack vectors

### 11.1 Version-spoofing attack

**Attack:** Peer claims to support feature X but doesn't, then
errors out on real use.

**Mitigation:** Handshake failures emit metrics. Repeat
offenders are revoked from peer lists via the normal
peering-revocation flow. No protocol-level enforcement of
"claims match reality" — it's a social signal.

### 11.2 Downgrade attack

**Attack:** MITM forces two nodes to negotiate an older
protocol version to exploit a known vulnerability.

**Mitigation:**
- Handshake messages are signed, so the version claims aren't
  forgeable in transit.
- Node operators can set a minimum acceptable peer version:

  ```yaml
  peering:
      minimum_peer_version: "2.12.0"
  ```

  Refuses to peer with anything older.

### 11.3 Deprecation-race abuse

**Attack:** A feature is scheduled for deprecation in 18
months. Adversary accumulates reliance on it during that
window; legitimate operators migrate, but the adversary times
their exploit for the last week when enforcement is weak.

**Mitigation:** The 18-month timeline explicitly ends with
feature removal (§6.1 stage 4). The adversary gets the same
hard wall as everyone else.

### 11.4 Feature-matrix sybil

**Attack:** Adversary creates many nodes claiming to support
many features to skew discovery rankings.

**Mitigation:** Feature claims are verifiable (just try to use
the feature). The discovery API's quid-index shows which nodes
actually handle specific request types vs which just claim to.

## 12. Open questions

1. **Who decides on version bumps?** Currently the protocol's
   authors. For an open governance model, deprecation could
   require consent from a quorum of active public-network
   operators. Probably worth formalizing in a follow-up.

2. **Long-term SDK support.** How many SDK versions do we
   maintain simultaneously? Probably last two minor lines.
   Shipped as maintenance releases with CVEs only.

3. **Multi-protocol mode.** Should a single node be able to
   speak two major versions simultaneously during a transition?
   Probably yes for the transition window (which is part of
   why major versions span years). Complexity cost is high
   but it's the only way to avoid hard fork chaos.

4. **Per-domain version pinning.** Could an operator say "this
   domain only uses features up to 2.12 because some
   community member's client is stuck"? Probably yes as an
   opt-in, overrides-default-negotiation mode.

5. **Published compatibility matrix.** Should there be an
   official page listing which SDK version pairs with which
   node version? Yes, at `quidnug.com/protocol/compatibility`.
   Auto-generated from release metadata.

## 13. Review status

Draft. Needs:

- Operator buy-in on the 18-month deprecation default. Some
  will want shorter (impatient to remove tech debt); some
  longer (enterprise constraints).
- SDK maintainer consensus on the backward-compat window.
- Infrastructure decisions on where to host
  `quidnug.com/protocol/versions` as a live service.

Once shipped, this QDP itself becomes the template for all
subsequent QDPs that want to introduce deprecated-and-replaced
primitives.

## 14. References

- [Semantic Versioning 2.0.0](https://semver.org/)
- [QDP-0009 (Fork-block activation)](0009-fork-block-trigger.md) —
  complement for enforcement coordination
- [QDP-0014 (Node Discovery)](0014-node-discovery-and-sharding.md) —
  NODE_ADVERTISEMENT carries the version declaration
- [Rust RFC deprecation process](https://rust-lang.github.io/rfcs/0507-release-channels.html) —
  precedent for stability guarantees in a fast-moving project
