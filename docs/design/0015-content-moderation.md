# QDP-0015: Content Moderation & Takedowns

| Field      | Value                                                            |
|------------|------------------------------------------------------------------|
| Status     | Draft — design only                                              |
| Track      | Protocol + ops + legal                                           |
| Author     | The Quidnug Authors                                              |
| Created    | 2026-04-20                                                       |
| Requires   | QDP-0001 (nonce ledger), QDP-0012 (governance), QDP-0013 (federation), QDP-0014 (discovery) |
| Implements | Legally-compliant content moderation in an append-only system    |

## 1. Summary

Quidnug is append-only by design — blocks are immutable, trust
edges are permanent, events are signed cryptographic artifacts.
This is good for auditability and bad for every real-world
obligation a running operator faces:

- DMCA takedowns (copyright)
- GDPR right-to-erasure requests
- Court-ordered defamation content removal
- CSAM and other illegal content
- Hate speech under country-specific laws
- Legal holds and subpoena responses
- Spam and obvious abuse

The existing `FLAG` event type in QRP-0001 (the reviews
protocol) gives users a way to mark content as objectionable,
and the trust-weighted rating algorithm de-weights heavily-flagged
reviewers. But that's a community signal, not a legal compliance
mechanism. A running operator needs the ability to **refuse to
serve** specific content from its local node, with a clear audit
trail, without violating the protocol's immutability invariants.

QDP-0015 introduces three primitives that give operators a
complete moderation workflow:

1. **`MODERATION_ACTION` transaction** — a signed on-chain record
   declaring "this operator has applied this action to this
   content." Append-only (naturally) but visible, auditable,
   and independently verifiable.
2. **Operator-level content policy** — declared per-node via a
   signed policy document, consumed by the node's serving layer
   to decide what to gossip, serve, and index.
3. **Cross-federation moderation gossip** — allows operators to
   share moderation signals without requiring agreement. Network A
   can flag content as CSAM; Network B decides whether to honor
   that flag.

The key insight: **immutability of the chain ≠ serving of the
content**. A node's chain can contain an event it refuses to
serve via its API. Other nodes can still choose to serve it. The
chain remains correct; each operator's policy remains local.

## 2. Goals and non-goals

**Goals:**

- Give operators a legal-compliance-ready workflow for every
  common takedown scenario (DMCA, GDPR, court order, CSAM).
- Preserve the append-only chain invariant — moderation actions
  are additions, never deletions.
- Transparent audit trail — every moderation action is signed
  and visible to anyone who wants to audit an operator's
  decisions.
- Federation-aware — moderation is operator-local unless
  explicitly propagated.
- Scalable — moderation actions don't require modifying
  existing blocks or re-hashing history.
- Reversible — a moderation action can be undone via a later
  counter-action with transparent provenance.

**Non-goals:**

- Guaranteed global content removal. In a federated network,
  different operators can disagree about what constitutes a
  valid takedown. QDP-0015 doesn't force agreement.
- Prevention of illegal content at submission time. The chain
  accepts any well-formed signed transaction; moderation is
  after-the-fact.
- Content encryption at rest. Protocol-level encryption is a
  separate concern (see roadmap). Moderation works on the
  assumption that content is readable by the operator.
- Automatic AI-based moderation. Moderation decisions remain
  human (or operator-automated) and on-chain; the protocol
  doesn't encode AI classifiers.

## 3. Concept model

### 3.1 The immutability vs takedown tension

Append-only + content obligations is a hard tension everyone hits.
Existing approaches:

- **Selective gossip** (BitTorrent trackers refusing to serve
  certain infohashes) — good for operator-local compliance, poor
  for visibility.
- **Encrypted-until-unlocked** (some archival systems) — good for
  "right to be forgotten" but doesn't work post-facto.
- **Cryptographic shredding** (deleting the encryption key) —
  works for pre-encrypted content.
- **Pure suppression** (operator-local blocklists) — simple but
  opaque.

Quidnug uses a hybrid: **transparent suppression**. The content
stays in the chain forever (correct by protocol), but each node
applies a signed, on-chain policy that controls what it will
gossip, serve via HTTP, or index. The policy itself is a first-
class artifact, so third parties can audit an operator's
moderation decisions.

### 3.2 Three scopes of action

Per QDP-0015, each moderation action has one of three scopes:

- **`suppress`** — node refuses to include the target in
  HTTP responses, gossip, or index outputs. The most
  aggressive scope; used for illegal content (CSAM, court-
  ordered removals).
- **`hide`** — node returns the target only to authenticated
  queries with a `?includeHidden=true` parameter. Used for
  DMCA-style content that might be restored or for content
  flagged as misleading but not illegal.
- **`annotate`** — node serves the target as normal but
  attaches a visible warning or contextual note. Used for
  disputed reviews, unverified claims, or content that
  requires user awareness.

These three scopes are orthogonal to whether the target is a
single event, an entire event stream, a quid, a trust edge, or
any other signed artifact.

### 3.3 Target types

A moderation action targets one of:

- `TxID` — a specific transaction by its hash
- `EventID` — shorthand for a `TxID` that happens to be an EVENT
- `QuidID` — all transactions by a quid
- `Domain` — all transactions in a domain
- `ReviewOfProduct` — all reviews of a specific product quid

Targets compose with scope: a `suppress` action on a `QuidID`
blocks every transaction by that quid; an `annotate` action on a
`ReviewOfProduct` adds a banner to every review of that product.

### 3.4 Evidence and rationale

Every moderation action includes two mandatory fields:

- **`reasonCode`** — an enumerated reason matching a well-known
  taxonomy (DMCA, COURT_ORDER, CSAM, TOS_VIOLATION,
  SPAM, MISINFORMATION, GDPR_ERASURE, VOLUNTARY, OTHER).
- **`evidenceURL`** — a URL to the supporting documentation
  (DMCA notice, court order PDF, internal ticket). The URL
  may be internal-only; external auditors can request access.

`reasonCode` is public and auditable. `evidenceURL` is visible
but may return 403 to non-authenticated requests. The point is
that an operator cannot claim "we didn't know" — every takedown
has a committed reason and a link to evidence.

## 4. The `MODERATION_ACTION` transaction

### 4.1 Shape

```go
type ModerationActionTransaction struct {
    BaseTransaction

    // Who's issuing this action. Typically the node operator's
    // own quid, but a delegated moderator quid is also valid
    // (see §4.4 for delegation semantics).
    ModeratorQuid string `json:"moderatorQuid"`

    // What's being moderated.
    TargetType string `json:"targetType"` // "TX" | "QUID" | "DOMAIN" | "REVIEW_OF_PRODUCT"
    TargetID   string `json:"targetId"`   // matches TargetType's identifier format

    // The action's severity.
    Scope string `json:"scope"` // "suppress" | "hide" | "annotate"

    // Legally-required metadata.
    ReasonCode  string `json:"reasonCode"`  // enum, see §4.5
    EvidenceURL string `json:"evidenceUrl,omitempty"` // optional in VOLUNTARY

    // For "annotate" scope — the text to display alongside
    // the content. Capped at 2048 chars. Optional for other
    // scopes.
    AnnotationText string `json:"annotationText,omitempty"`

    // An optional reverse link to a prior MODERATION_ACTION that
    // this one supersedes. Used for un-suppress, reason-code
    // correction, or escalation.
    SupersedesTxID string `json:"supersedesTxId,omitempty"`

    // Effective-from / effective-until bounds. Zero = no bound.
    EffectiveFrom  int64 `json:"effectiveFrom,omitempty"`
    EffectiveUntil int64 `json:"effectiveUntil,omitempty"`

    // Monotonic nonce per moderator.
    Nonce int64 `json:"nonce"`
}
```

### 4.2 Validation rules

A `MODERATION_ACTION` is rejected unless:

1. **Domain exists + is supported.** Standard Base-level check.
2. **Moderator is authorized.** Either:
   - The moderator is the operator's own quid (directly trusted
     as a validator on this node, or in `SeedsJSON.operator.quid`),
     OR
   - A delegation TRUST edge from the operator to the moderator
     exists at weight ≥ 0.7 in the reserved domain
     `moderators.*` (per operator's home tree).
3. **TargetType is one of the enums.**
4. **TargetID format matches TargetType:**
   - `TX`: 64-char hex
   - `QUID`: 16-char hex
   - `DOMAIN`: valid domain name
   - `REVIEW_OF_PRODUCT`: 16-char product quid
5. **Scope is one of** `suppress` / `hide` / `annotate`.
6. **ReasonCode is in the taxonomy** (§4.5).
7. **EvidenceURL is required** for all reason codes except
   `VOLUNTARY` and `SPAM`. For `DMCA` / `COURT_ORDER` / `CSAM` /
   `GDPR_ERASURE`, `EvidenceURL` MUST be non-empty.
8. **AnnotationText ≤ 2048 chars; no control characters.**
9. **SupersedesTxID** (if present) must reference an existing
   `MODERATION_ACTION` from the same moderator. Supersede
   chains are single-parent; multiple-branch supersession is
   rejected.
10. **Effective bounds are sane** — if both are set,
    `EffectiveFrom < EffectiveUntil`.
11. **Nonce monotonic per moderator.**
12. **Signature valid.**

### 4.3 Serving-time enforcement

The node's HTTP handlers consult a `ModerationState` view
(indexed by target) before returning content:

```go
func (node *QuidnugNode) moderationFilter(target TargetRef, req *http.Request) FilterDecision {
    actions := node.ModerationState.ActiveFor(target)
    scope := computeEffectiveScope(actions) // highest-severity active action
    switch scope {
    case "suppress":
        return FilterDecision{HTTPStatus: 451, Reason: actions[0].ReasonCode}
    case "hide":
        if req.URL.Query().Get("includeHidden") != "true" {
            return FilterDecision{HTTPStatus: 404, Reason: "hidden"}
        }
        return FilterDecision{Passthrough: true}
    case "annotate":
        return FilterDecision{Passthrough: true, AttachAnnotation: actions[0].AnnotationText}
    }
    return FilterDecision{Passthrough: true}
}
```

Filter is applied in:

- Event streams (`GET /api/streams/{subject}/events`)
- Tx-level lookups (`GET /api/transactions/{txId}`)
- Trust queries (if a target quid is fully suppressed,
  trust queries return level 0)
- Discovery API (`/api/v2/discovery/quids` and friends)
- Gossip outbound (suppressed targets don't propagate)

### 4.4 Moderator delegation

An operator can delegate moderation authority to specific
moderator quids without giving them full node operator authority.
This is done via an ordinary TRUST transaction:

```bash
quidnug-cli trust grant \
    --signer operator.key.json \
    --trustee <moderator-quid> \
    --domain moderators.my-op.example \
    --level 0.9 \
    --nonce <next>
```

The moderator can then issue `MODERATION_ACTION` transactions
for content within the operator's supported domains. The trust
edge can be revoked at any time via a counter-TRUST at level 0.

### 4.5 Reason code taxonomy

Stable enum. New codes added via QDP amendment.

| Code | Description | Evidence URL required | Typical scope |
|---|---|---|---|
| `DMCA` | Copyright takedown | Yes | suppress |
| `COURT_ORDER` | Court-ordered removal | Yes | suppress |
| `CSAM` | Child sexual abuse material | Yes | suppress |
| `GDPR_ERASURE` | Right-to-be-forgotten (GDPR Art. 17) | Yes | hide (see §5) |
| `DATA_SUBJECT_REQUEST` | Other privacy-law erasure request | Yes | hide |
| `HATE_SPEECH` | Violates operator TOS on hate speech | Yes (internal ticket) | hide or annotate |
| `DEFAMATION` | Adjudicated defamation | Yes (court order or settlement) | suppress |
| `TOS_VIOLATION` | General TOS violation | No (internal only) | hide |
| `SPAM` | Automated / coordinated low-quality | No | hide |
| `MISINFORMATION` | Factually false, disputed | Yes (internal ticket) | annotate |
| `VOLUNTARY` | User-requested removal | No | hide |
| `OTHER` | Catch-all; evidence recommended | Optional | varies |

### 4.6 Composition: multiple actions on the same target

Over time a target may accumulate multiple `MODERATION_ACTION`
records: an initial `DMCA` suppress, a later `COURT_ORDER`
supersede that un-suppresses after the copyright dispute was
resolved, a still later `GDPR_ERASURE` hide if the author
requests it.

The effective scope at any given time is computed as:

1. Walk supersede chains to find the "tip" (most recent non-
   superseded action per moderator-target pair).
2. Union the tips across all moderators acting on the target.
3. Choose the highest-severity scope active at the current time
   (per EffectiveFrom/EffectiveUntil bounds):
   - `suppress` beats `hide` beats `annotate` beats `passthrough`.

## 5. Interaction with append-only: the cryptographic shredding path

For GDPR erasure requests where the data subject demands actual
data destruction (not just suppression), the protocol offers
an optional primitive built on top of payload-CID indirection.

### 5.1 Inline vs CID payloads

Most `EventTransaction` payloads are inline (`Payload` map).
These cannot be erased without corrupting the chain. For events
with large payloads, the protocol supports payload-by-CID:
the chain stores only the IPFS CID; the actual payload lives
in IPFS.

### 5.2 The erasure path

For a GDPR erasure request on an event:

1. Operator issues a `MODERATION_ACTION` with scope `suppress`
   and reason `GDPR_ERASURE` (standard QDP-0015 flow).
2. If the event's payload was stored by CID, the operator can
   additionally:
   - Remove the IPFS pin from their node
   - Encourage federated operators to unpin
   - Make the payload unreachable via any federation node

The CID itself remains in the chain forever. The payload
content is gone from every honest operator's IPFS pinset. An
auditor can confirm a signed `EventTransaction` existed at
that CID; they cannot recover its content.

For inline payloads, the protocol has no erasure primitive. The
operational advice is: **store personally-identifiable data
via CID**, not inline. A new `require_pii_by_cid` config
flag (default false, recommended true for EU-facing deployments)
causes the node to reject inline payloads that appear to
contain PII (heuristic match on emails, phone numbers, etc.).

### 5.3 What this does and doesn't satisfy

GDPR Art. 17 allows controllers to rely on "technical
impossibility" when complete erasure isn't feasible, provided
they've taken reasonable steps. The suppress + unpin pattern
constitutes reasonable steps. It doesn't guarantee that a
non-cooperating third party didn't retain a copy of the data.
Operators should get legal review before relying on this path
for high-risk content.

## 6. Federation-aware moderation

Moderation actions are **operator-local by default**. Network A
refusing to serve content X does not bind Network B.

### 6.1 Propagation via federation

A moderation action is a transaction; it flows through the
normal gossip layer. But:

- Recipient nodes store the action in their chain (correct by
  protocol).
- Whether they apply the action depends on whether they
  respect the moderator's authority.
- Only moderators the recipient has chosen to trust (via
  TRUST edges) have their actions applied.

Concretely: Alice's node sees Bob's moderation action. Alice's
node applies it if Alice trusts Bob's moderation authority
(which requires Alice to publish a TRUST edge from her own
quid to Bob in the `moderators.*` tree).

### 6.2 Cross-network moderation gossip

For networks that want to share moderation signals without full
federation, a lightweight HTTP endpoint:

```
POST /api/v2/moderation/import
```

Accepts a signed `MODERATION_ACTION` transaction from an
external network. If the importing node trusts the external
moderator (via configured federation trust source, see
QDP-0013), it imports the action. If not, it's silently
ignored.

This lets a CSAM-reporting network (e.g., an IWF-like operator)
publish signed CSAM hashes, and any Quidnug operator can opt in
to applying IWF's takedowns.

### 6.3 "Do not federate" flag

Moderation actions carry an optional `DoNotFederate: true` flag
for cases where the takedown itself is confidential (e.g.,
ongoing legal matter). If set, the action is applied locally
but not gossiped. Trust in this flag is social — a malicious
node could still leak the action. For strict confidentiality,
suppress actions should be kept entirely off-chain.

## 7. Attack vectors and mitigations

### 7.1 Censorship via false takedown

**Attack:** An adversary floods takedown requests to force
suppression of content they don't like.

**Mitigation:**
- Moderation authority is scoped per operator. The adversary
  would need to compromise the operator's key or compel them
  legally.
- Reasons codes + evidence URLs + signed transactions make
  every action auditable. "Wrong takedown" is discoverable
  and reversible.
- Federation means other operators can still serve. The attack
  succeeds only within one operator's sphere.

### 7.2 Moderator key compromise

**Attack:** An attacker steals a moderator's key and issues
mass-suppression actions.

**Mitigation:**
- Operator revokes the moderator's delegation TRUST edge
  (level=0). All future actions from that moderator become
  invalid.
- Prior actions from before revocation remain in effect until
  explicitly superseded by a trusted moderator.
- Guardian recovery (QDP-0002) rotates the compromised key.

### 7.3 Stream flooding via `annotate`

**Attack:** A moderator issues thousands of annotate actions
with long annotation text to bloat the chain.

**Mitigation:**
- 2048-char cap on annotation text.
- Standard per-moderator rate limit (QDP-0016).
- Domain governors (QDP-0012) can remove moderators whose
  actions violate policy.

### 7.4 Takedown evasion via key rotation

**Attack:** A user generates a new quid for each review to
avoid prior suppressions tied to their original quid.

**Mitigation:**
- Content-level suppressions (scope on `TX` or
  `REVIEW_OF_PRODUCT`) still apply to the new review.
- OIDC bridge + operator-attested identity binding makes
  cross-quid identity linking possible for operators who
  choose to require verification.
- Reputation bootstrapping slows — each new quid starts with
  zero trust.

### 7.5 Back-channel deletion

**Attack:** Operator says they've "suppressed" content but
actually still serves it via a private back-channel to a
specific requester.

**Mitigation:** This is a social + legal concern outside the
protocol's guarantees. The on-chain `MODERATION_ACTION`
documents the operator's public commitment; honesty testing
requires either a trusted auditor probing the API or a
whistleblower.

### 7.6 Malicious federation inclusion

**Attack:** An untrusted federation source sends crafted
moderation actions that slip through an operator's filters.

**Mitigation:**
- Moderation actions must be signed by quids trusted by the
  operator. Federation alone does not grant authority.
- Each imported action goes through the same validation pipeline
  as locally-issued ones.

### 7.7 Reverse-engineered evidence URLs

**Attack:** Evidence URLs might leak sensitive information
(a court case's docket number reveals the plaintiff's name).

**Mitigation:**
- Evidence URLs can point to auth-required internal resources.
  The protocol doesn't require them to be publicly accessible.
- For extremely sensitive takedowns, use `reasonCode=OTHER`
  with a sealed URL.

## 8. Implementation plan

### Phase 1: State + validation

- Add `TxTypeModerationAction` constant.
- Add `ModerationActionTransaction` struct.
- Add `ValidateModerationActionTransaction` with all §4.2 rules.
- Add `ModerationRegistry` indexed by target type + target ID.
- Wire into block-processing and registry commits.

Effort: ~1 person-week.

### Phase 2: Serving-layer filter

- Hook `moderationFilter` into HTTP handlers:
  - `GetEventHandler`
  - `GetEventStreamHandler`
  - `GetStreamEventsHandler`
  - Discovery handlers (suppress hidden quids)
  - Trust query handlers (quid-level suppressions return 0)
- Add `?includeHidden=true` query parameter handling.
- Add `X-Quidnug-Moderated: <reasonCode>` response header for
  hidden/suppressed items.

Effort: ~1 person-week.

### Phase 3: CLI + moderation UI

- `quidnug-cli moderate suppress --target-type TX --target-id X --reason-code DMCA --evidence-url https://...`
- `quidnug-cli moderate history --target X` to audit action log
- `quidnug-cli moderate supersede --tx-id X --new-scope hide`

Effort: ~3 person-days.

### Phase 4: Federation import surface

- `POST /api/v2/moderation/import` endpoint.
- External-source configuration + signature verification.
- Pull-model: node periodically polls trusted external sources.

Effort: ~3-5 person-days.

### Phase 5: Operator tooling

- Moderation dashboard at `quidnug.com/network/moderation` showing
  recent actions, their authors, and provenance.
- DMCA intake web form that auto-generates draft moderation
  transactions for operator review.
- CSAM reporting-compatibility mode (IWF list consumption).

Effort: ~2-3 person-weeks.

## 9. Transparency reporting

Every operator that honors QDP-0015 should publish a quarterly
transparency report, generated by the node itself:

```bash
quidnug-cli moderate transparency-report \
    --operator-key operator.key.json \
    --period 2026-Q1 \
    --out transparency-2026-q1.json
```

The report contains:
- Total actions in period, by reason code
- Actions by scope (suppress / hide / annotate)
- Actions that were later superseded
- Aggregate affected-targets count
- Signed by the operator for verifiability

This shows up at `quidnug.com/network/transparency/` as a
public commitment to accountability.

## 10. Operator playbook excerpt

A short doc at
`deploy/public-network/moderation-playbook.md` (to be written
alongside Phase 1 implementation) will cover:

- Setting up a moderator quid
- Handling a DMCA notice (step-by-step)
- Responding to a GDPR erasure request
- When to use suppress vs hide vs annotate
- Incident-response flow for CSAM reports
- How to delegate moderation to a team
- Quarterly transparency reporting cadence

## 11. Open questions

1. **Should operators be allowed to moderate on other operators'
   behalf?** Currently an operator's moderation applies only to
   its own node. A "reciprocal mod" primitive where Operator B
   preemptively accepts Operator A's CSAM suppressions might
   reduce per-operator workload. Probably worth a follow-up QDP.

2. **Immutable annotations on inline payloads.** If a payload's
   text is defamatory, annotating it doesn't undo the harm of
   the content still being visible. Should `annotate` scope
   suppress the original and add a replacement? Lean toward no:
   that's a slippery-slope toward centralized rewriting.

3. **Statute-of-limitations expiry.** Should `suppress` actions
   expire automatically after, say, 10 years for non-criminal
   content? Argues against consistent archival. Probably leave
   to per-operator policy.

4. **Automated moderation classifiers.** The protocol doesn't
   encode them, but operators will use them internally. Should
   classifier output be linked from `evidenceURL`? Probably yes
   as a best practice, no as a requirement.

5. **Bulk operations.** Moderating 10,000 spam reviews one-by-one
   is operationally painful. Consider a `BulkModerationAction`
   that targets a list of TxIDs. Save for a follow-up QDP if
   demand materializes.

## 12. Review status

Draft. The hardest questions are legal, not protocol:

- Legal review of the "suppress = not-served vs actually-erased"
  distinction under GDPR Art. 17. Probably defensible, needs
  counsel confirmation.
- DMCA safe-harbor applicability when an operator only
  suppresses (doesn't delete). DMCA §512(c) assumes the
  service provider can remove content; "make unavailable" is
  usually close enough.
- CSAM reporting workflow alignment with NCMEC / IWF / Canadian
  Center for Child Protection reporting requirements.

Implementation is straightforward once legal signs off.

## 13. References

- [DMCA Section 512(c)](https://www.law.cornell.edu/uscode/text/17/512) —
  notice-and-takedown procedure
- [GDPR Article 17](https://gdpr-info.eu/art-17-gdpr/) —
  right to erasure
- [QRP-0001 §REVIEW](../../examples/reviews-and-comments/PROTOCOL.md)
  — the FLAG event type this QDP complements
- [QDP-0012 (Domain Governance)](0012-domain-governance.md) —
  governor approval paths for moderator delegation
- [QDP-0013 (Network Federation)](0013-network-federation.md) —
  cross-network moderation import mechanism
- [QDP-0016 (Abuse Prevention, planned)](0016-abuse-prevention.md) —
  the complement to moderation: preventing abuse at submission
  time
