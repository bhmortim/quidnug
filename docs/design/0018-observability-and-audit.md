# QDP-0018: Observability, Audit, and Tamper-Evident Operator Log

| Field      | Value                                                            |
|------------|------------------------------------------------------------------|
| Status     | Phase 1 landed — audit log + query endpoints + automatic emission; Phases 3-6 pending |
| Track      | Protocol + ops                                                   |
| Author     | The Quidnug Authors                                              |
| Created    | 2026-04-20                                                       |
| Requires   | QDP-0010 (Merkle proofs), QDP-0012 (governance), QDP-0015 (moderation) |
| Implements | Operator-level accountability, tamper-evident admin log, structured observability |

## 1. Summary

The protocol is auditable by design at the transaction level —
every TRUST / IDENTITY / TITLE / EVENT / MODERATION_ACTION is
signed and committed on-chain. But there's a second layer of
activity that isn't in the transaction log:

- **Operator admin actions** — config changes, validator set
  edits, peer additions, key rotations.
- **Ops-layer signals** — health-check status changes, node
  restarts, middleware toggles, rate-limit threshold
  adjustments.
- **Cross-node operator coordination** — when multiple operators
  agree on something off-chain.

Right now these live in log files + Prometheus metrics +
operator memory. That's fine for small networks but inadequate
for the trust story at scale: "how do I know this operator is
behaving honestly?"

QDP-0018 adds a **tamper-evident operator audit log** — a
per-node, append-only, Merkle-committed log of every operator
action the node observes. The log is:

- **Chained** — each entry commits to the hash of the previous
  entry.
- **Signed** — the node's key signs the chain tip periodically.
- **Public-verifiable** — anyone can ask "give me the log since
  hash X" and verify the entire tail.
- **Independently queryable** — external auditors can scrape the
  tip without needing operator cooperation.

Plus it adds structured observability primitives:

- **Metrics standardization** — every major flow emits the same
  label set across every operator.
- **Structured event emission** for admin + ops events, using
  the existing EventTransaction shape (but on a dedicated
  `ops.internal.<operator>` meta-domain).
- **Alert-rule standardization** so any Prometheus deployment
  can import a canonical set of alerts.

This is primarily an operational reliability feature. It doesn't
change protocol semantics or add new cryptographic primitives;
it formalizes what "good operator behavior" looks like in a way
that's independently verifiable.

## 2. Goals and non-goals

**Goals:**

- A per-operator append-only audit log that third parties can
  verify is tamper-evident.
- Standardized metric label sets so cross-operator dashboards
  work consistently.
- Standardized structured events for all admin-level actions
  (config changes, validator-set edits, moderation actions,
  governance votes).
- Zero-dependency implementation: the existing block machinery
  + Merkle proofs + EventTransaction cover most of this.
- Integration with QDP-0015's moderation transparency reporting.

**Non-goals:**

- Centralized audit repository. Each operator owns their log.
- Protocol-layer enforcement of "honest operator behavior."
  The log catches dishonesty; remediation is social.
- Replacement for existing Prometheus metrics. This QDP adds
  to the set, doesn't replace.
- Zero-knowledge audit trail. Auditors see what operators
  publish; privacy-preserving variants are out of scope for
  the first version.

## 3. The operator audit log

### 3.1 Conceptual model

Each operator runs a continuous, append-only log of actions
taken by the operator's privileged processes. Every entry is:

- Numerically ordered (sequence number)
- Linked to the previous entry (parent hash)
- Timestamped
- Signed by the operator key at least once per day (log-head
  attestation)

Externally this looks like a Merkle log à la Certificate
Transparency: an auditor asks for the current head, then asks
for inclusion proofs for specific entries, then verifies the
chain back to a pinned-earlier head.

### 3.2 Entry schema

```go
type OperatorAuditEntry struct {
    // Monotonic per-operator
    Sequence int64 `json:"sequence"`

    // Hash of the previous entry; all-zeros for entry 0.
    PrevHash string `json:"prevHash"`

    // When this entry was recorded (nanoseconds since epoch).
    Timestamp int64 `json:"timestamp"`

    // Which operator quid generated this entry.
    OperatorQuid string `json:"operatorQuid"`

    // Category of action — see §3.3.
    Category string `json:"category"`

    // What was done. Structured but category-specific; see §3.4.
    Payload map[string]interface{} `json:"payload"`

    // Optional human-readable note.
    Note string `json:"note,omitempty"`

    // Self-hash: sha256 of all above fields serialized canonically.
    Hash string `json:"hash"`
}
```

Entries are **not** stored as transactions on the main chain.
They live in a separate operator-local append-only log file
(plus an in-memory index). Periodically the log's current head
is committed to the chain as a signed `AUDIT_ANCHOR` event
(§4), which gives the log its tamper-evidence — the chain
records that at time T the log head was at hash H.

### 3.3 Entry categories

Stable enum:

| Category | When emitted |
|---|---|
| `CONFIG_CHANGE` | Any mutation of `/etc/quidnug/node.yaml` or equivalent |
| `VALIDATOR_EDIT` | Changes to `TrustDomain.Validators` / `ValidatorPublicKeys` |
| `PEER_CHANGE` | Add / remove a peer from `seed_nodes` |
| `KEY_ROTATION` | Anchor-based key rotation event (QDP-0001) |
| `MODERATION_ACTION` | A MODERATION_ACTION tx was issued by this operator (§QDP-0015) |
| `GOVERNANCE_VOTE` | Operator's governor-role signature on a DOMAIN_GOVERNANCE tx (§QDP-0012) |
| `NODE_LIFECYCLE` | Node start / stop / restart / crash-recovery |
| `SIGNING_QUORUM` | Delegated signing to an HSM or WebAuthn key |
| `ABUSE_RESPONSE` | Rate-limit adjustments / challenge-issuance / ban |
| `DSR_FULFILLMENT` | QDP-0017 data subject request processed |
| `FORK_DECISION` | Operator's stance on an incoming fork-block proposal |
| `OPERATOR_OTHER` | Catch-all; free-form payload |

### 3.4 Example payloads

#### CONFIG_CHANGE

```json
{
    "sequence": 42,
    "category": "CONFIG_CHANGE",
    "payload": {
        "diff": "before:{...}\nafter:{...}",
        "actor": "alice@operator.example"
    }
}
```

#### VALIDATOR_EDIT

```json
{
    "sequence": 43,
    "category": "VALIDATOR_EDIT",
    "payload": {
        "domain": "reviews.public.technology.laptops",
        "action": "add",
        "target_quid": "c7e2d10000000001",
        "target_weight": 1.0,
        "rationale": "promoted after governance tx X ratified"
    }
}
```

#### MODERATION_ACTION

```json
{
    "sequence": 44,
    "category": "MODERATION_ACTION",
    "payload": {
        "moderation_tx_id": "<tx-hash>",
        "target_id": "<target>",
        "scope": "suppress",
        "reason_code": "DMCA",
        "evidence_url": "https://internal.example/dmca/1234"
    }
}
```

(Note: the moderation tx itself is on-chain per QDP-0015; this
audit-log entry is a local operator-side record with cross-
reference.)

#### GOVERNANCE_VOTE

```json
{
    "sequence": 45,
    "category": "GOVERNANCE_VOTE",
    "payload": {
        "governance_tx_id": "<tx-hash>",
        "action": "ADD_VALIDATOR",
        "target_subject": "<quid>",
        "vote_weight": 1.0,
        "rationale": "X has been running a clean cache replica for 120 days"
    }
}
```

### 3.5 Log head attestation

Periodically (default hourly, configurable), the node publishes
an `AUDIT_ANCHOR` event to its operator-meta-domain:

```go
type AuditAnchorPayload struct {
    LogHeadHash  string `json:"logHeadHash"`
    LogHeight    int64  `json:"logHeight"`
    GeneratedAt  int64  `json:"generatedAt"`
    Signature    string `json:"signature"` // operator-key over canonical bytes
}
```

The anchor is a regular `EventTransaction` on the
`ops.internal.<operator-domain>` domain with
`EventType: AUDIT_ANCHOR`. This means it flows through:

- Normal block inclusion (confirmed on-chain)
- Normal signature verification
- Normal gossip to federation peers
- Normal indexing in the per-domain quid index

Because the anchor is on the chain, anyone can retrieve a
cryptographic record saying "at time T the operator attests
their audit log was at head H." If the operator later tries to
rewrite log entries, the new head won't match an old attestation,
revealing the manipulation.

### 3.6 External audit query

Any client can fetch:

```
GET /api/v2/audit/head
GET /api/v2/audit/entries?since=<seq>&limit=100
GET /api/v2/audit/entry/{seq}
GET /api/v2/audit/proof/{seq}   ← Merkle proof against a committed anchor
```

Response for `/head`:

```json
{
    "operatorQuid": "...",
    "headHash": "...",
    "height": 12847,
    "lastAnchored": {
        "height": 12844,
        "anchorTxId": "<tx-hash>",
        "anchoredAt": 1776710000
    }
}
```

The client compares `headHash` to their last pinned head,
fetches only the new entries, and verifies the chain.

## 4. Structured observability events

### 4.1 Why a dedicated meta-domain

Admin events are themselves a kind of event stream. Rather
than inventing a new indexing mechanism, QDP-0018 reserves a
domain tree:

```
ops.internal.<operator-domain>
```

- `ops.internal.<operator-domain>.audit` — audit anchors (§3.5)
- `ops.internal.<operator-domain>.metrics` — periodic metric snapshots
- `ops.internal.<operator-domain>.alerts` — alert fires + resolutions
- `ops.internal.<operator-domain>.moderation` — moderation transparency

This domain is registered at operator-setup time. The operator
is the sole validator + governor. Events in this tree are
signed by the operator and indexed by the existing quid-domain
index (QDP-0014).

### 4.2 Standardized metric label set

All metrics exported via `/metrics` follow a consistent labeling
convention:

```
quidnug_<subsystem>_<metric>{
    operator="<operator-quid>",
    node="<node-quid>",
    domain="<trust-domain>",
    // subsystem-specific labels
}
```

Core subsystems:

| Subsystem | Example metrics |
|---|---|
| `block` | `_generated_total`, `_received_total{tier}`, `_size_bytes` |
| `tx` | `_processed_total{type, accepted}`, `_pending_count`, `_queue_depth` |
| `trust` | `_computation_duration_seconds`, `_cache_hits_total` |
| `gossip` | `_inbound_total{src}`, `_outbound_total{dst}`, `_dropped_total{reason}` |
| `audit` | `_entries_total{category}`, `_head_anchored_seconds_ago` |
| `moderation` | `_actions_total{scope, reason_code}`, `_transparency_report_generated_total` |
| `governance` | `_votes_total{action}`, `_activations_total`, `_superseded_total` |
| `ratelimit` | `_decisions_total{outcome, layer}`, `_bucket_depletion_ratio` |
| `dsr` | `_requests_total{type}`, `_fulfilled_total`, `_median_fulfillment_hours` |

Operators importing standard dashboards (`deploy/observability/
grafana-dashboard.json`) get consistent views.

### 4.3 Standardized alert rules

`deploy/observability/prometheus-alerts.yml` ships with ~25
canonical alert rules covering:

- Block-production stalls
- Gossip partition detection
- Audit log tampering (head doesn't match last anchor)
- Moderation-action spike (possible coordinated request)
- DSR-fulfillment SLA breaches
- Validator-set drift (unexpected edit)
- Rate-limit spike (possible attack)
- Node health flap

These serve as a sane default; operators customize per their
environment.

## 5. Third-party verification tooling

### 5.1 Audit verifier CLI

```bash
quidnug-cli audit verify \
    --operator https://api.quidnug.com \
    --since 1776000000
```

Walks the operator's audit log tail:

1. Fetches the current audit head.
2. Fetches all entries since the pinned baseline.
3. Verifies the hash chain (each entry's `prevHash` matches the
   prior entry's `Hash`).
4. Fetches all `AUDIT_ANCHOR` events published to the
   operator's meta-domain since the baseline.
5. Confirms each anchor's `LogHeadHash` matches the actual log
   state at that height.
6. Flags any anchor where the log doesn't match → tamper
   evidence.

### 5.2 Public-audit dashboard

At `quidnug.com/audit/<operator-domain>/`:

- Live graph of the audit log (entries over time by category)
- Latest head hash with verification button
- Anchor-timeline showing on-chain attestations
- Recent `MODERATION_ACTION` + `GOVERNANCE_VOTE` entries
  with links to the transaction audit pages
- DSR-fulfillment histogram

Each operator can deploy this dashboard; since the audit log
is independently queryable, third parties can also host their
own dashboards pointing at any operator they're curious about.

### 5.3 Cross-operator comparison

For federated networks, a client can query multiple operators'
audit logs:

```bash
quidnug-cli audit compare \
    --operators https://api.quidnug.com,https://api.other-op.example \
    --category MODERATION_ACTION \
    --since "24h ago"
```

Useful for:

- Detecting if one operator is moderating significantly more
  content than peers
- Checking whether two federated operators agree on moderation
  decisions
- Monitoring governance-voting patterns across a consortium

## 6. Attack vectors and mitigations

### 6.1 Log tampering

**Attack:** Operator rewrites historical log entries to hide
past misbehavior.

**Mitigation:** The log's hash chain means any modification
invalidates every subsequent entry's hash. The
`AUDIT_ANCHOR` events on the main chain commit the log's head
hash at specific points in time — if the operator rewrites
history, either:

- The current head doesn't match the most recent anchor
  (tampering visible from external audit)
- Or the anchor history itself has been rewritten, which would
  require forking the operator's own blockchain — which
  federated peers + cache replicas would notice.

The log isn't cryptographically impossible to forge; it's
tamper-evident, which is what matters for accountability.

### 6.2 Incomplete logging

**Attack:** Operator suppresses audit-log entries for actions
they want to hide.

**Mitigation:** This is harder to catch but still surfaces
circumstantially. An external auditor can:

- Compare log-entry timestamps against other evidence (the
  main chain, gossip records, even user complaints).
- Notice suspicious gaps or category-distribution anomalies.
- Correlate with federation peers who may have observed the
  action.

There's no cryptographic guarantee of completeness. Operators
who claim to honor QDP-0018 are committing to log completeness
as a social norm, and violations are reputation-damaging.

### 6.3 Misleading entries

**Attack:** Operator writes entries that are technically correct
but misleading (e.g., describing a DMCA suppression as
"voluntary").

**Mitigation:** The `category` enum is stable. Entries with
category `MODERATION_ACTION` reference a concrete on-chain
transaction whose `reasonCode` is independently verifiable.
Operators who repeatedly log one category while their on-chain
actions say another erode their own credibility.

### 6.4 Metric gaming

**Attack:** Operator manipulates metrics to look healthier than
they are.

**Mitigation:** Metrics are soft signals. The audit log + the
main chain are the ground truth. If metrics disagree with audit
+ chain data, the metrics are wrong.

### 6.5 DoS via audit queries

**Attack:** Attacker floods `/api/v2/audit/entries?since=0`
requests.

**Mitigation:** Standard rate limits (QDP-0016). Responses are
cacheable at the CDN edge (audit entries are append-only).

## 7. Implementation plan

### Phase 1: Audit log infrastructure

- `OperatorAuditLog` type with append + head-computation.
- Disk-backed store (bounded file with rotation + index).
- CLI commands:
  - `quidnug-cli audit log` — tail the local log
  - `quidnug-cli audit head` — show the current head
  - `quidnug-cli audit export --since N` — dump entries

Effort: ~1 person-week.

### Phase 2: Automatic entry emission

- Hook into existing action sites:
  - Config-change detection → `CONFIG_CHANGE`
  - `AddTrustTransaction` / `AddIdentity` / etc. when signed by
    operator key → corresponding category
  - `MODERATION_ACTION` submission → log entry
  - Governance transaction signing → `GOVERNANCE_VOTE`
- Configurable log levels (verbose / normal / minimal).

Effort: ~1 person-week.

### Phase 3: Audit anchor emission

- Periodic goroutine (default hourly) that signs + publishes
  an `AUDIT_ANCHOR` event.
- Configuration via `audit_anchor_interval: "1h"`.
- Graceful-shutdown flushes last entry + anchor.

Effort: ~3 days.

### Phase 4: External query endpoints

- `GET /api/v2/audit/*` handlers.
- Merkle-proof generation (reuses QDP-0010 primitive).
- CDN-cacheable response headers.

Effort: ~5 days.

### Phase 5: Verifier tooling

- `quidnug-cli audit verify` — full tail verification.
- `quidnug-cli audit compare` — cross-operator comparison.
- `docs/observability/audit-verifier-guide.md` for third parties.

Effort: ~1 week.

### Phase 6: Dashboards + alerts

- `deploy/observability/grafana-dashboards/audit.json`.
- Updated `deploy/observability/prometheus-alerts.yml`.
- Public dashboard page at `quidnug.com/audit/` showing
  live operator attestations.

Effort: ~1 week.

## 8. Disk + bandwidth cost analysis

Operators worry about overhead. Rough envelope:

Per entry: ~300 bytes serialized.
Entries per day for a typical operator: ~500-2000 (mostly
moderation + rate-limit + node lifecycle).

Daily: 150 KB - 600 KB.
Annual: ~100-250 MB.

Anchor emissions add ~24 EventTransactions per day, each ~2 KB.
Annual: ~20 MB of on-chain storage.

These are negligible at any scale that justifies running a
production node.

## 9. Interaction with existing QDPs

| QDP | Interaction |
|---|---|
| QDP-0001 | Audit log commits leverage nonce ledger for anchor tx replay protection |
| QDP-0010 | Merkle-proof subsystem reused for audit entry inclusion proofs |
| QDP-0012 | Governance votes are a first-class audit category |
| QDP-0013 | Federation peers can cross-verify each other's logs |
| QDP-0014 | Audit entries flow through existing EventTransaction + quid-index infrastructure |
| QDP-0015 | Moderation actions auto-emit audit entries; transparency reports build on audit data |
| QDP-0016 | Rate-limit adjustments emit audit entries; Prometheus metrics standardized |
| QDP-0017 | DSR fulfillments emit DSR_FULFILLMENT audit entries for compliance reporting |

## 10. Open questions

1. **Private audit entries.** For highly-sensitive events (e.g.,
   legal holds ongoing), the operator might not want to commit
   the full payload on-chain. Should the log support
   encrypted-body entries where the hash commits but the
   payload is hidden? Probably yes; defer to a Phase 6
   extension.

2. **Log retention**. Keeping the full log forever is cheap but
   generates compliance concerns of its own (GDPR). Should
   entries expire after a configurable window? Probably no —
   the audit trail needs to be long-term. Retention concerns
   are handled at the entry payload level (don't log PII).

3. **Cross-operator audit redundancy**. If federated operators
   retain mirror copies of each other's logs, it's harder for
   any one operator to tamper. Should this be an explicit
   peering feature? Probably yes as an optional mode for
   high-trust consortia.

4. **Audit log vs main chain redundancy**. Why not just put
   admin events on the main chain? Two reasons: main chain has
   higher overhead + rate limits apply; audit entries are far
   more frequent than transactions. Keeping them separate and
   anchoring periodically is the right tradeoff.

5. **Standardized SIEM integration**. The log should be
   consumable by standard SIEM tools (Splunk / Datadog / Elastic).
   Should QDP-0018 specify the transport (syslog / Kafka
   / direct API)? Probably leave it loose; document the JSON
   format and let operators choose.

## 11. Review status

Draft. Needs:

- Feedback on the entry schema — is `Payload` the right shape?
  Maybe strongly-typed per category is cleaner.
- Estimation of third-party verifier CLI UX. Is this something
  operators would actually run, or does it need a web UI?
- Threat-model review: what attacks does this genuinely deter
  vs what's feel-good auditability?

Implementation is parallel-independent from QDP-0015 / 0016 /
0017. Phase 1 could land before any of them.

## 12. References

- [Certificate Transparency (RFC 6962)](https://datatracker.ietf.org/doc/html/rfc6962)
  — the hash-chained log design this QDP mirrors
- [QDP-0010 (Compact Merkle Proofs)](0010-compact-merkle-proofs.md) —
  proof-generation infrastructure audit queries reuse
- [QDP-0012 (Domain Governance)](0012-domain-governance.md) —
  governance votes emit audit entries
- [QDP-0015 (Content Moderation)](0015-content-moderation.md) —
  moderation actions emit audit entries
- [QDP-0016 (Abuse Prevention)](0016-abuse-prevention.md) —
  rate-limit adjustments emit audit entries
- [QDP-0017 (Data Subject Rights)](0017-data-subject-rights.md) —
  DSR fulfillments emit audit entries
