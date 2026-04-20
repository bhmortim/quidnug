# QDP-0024: Private Communications & Group-Keyed Encryption

| Field      | Value                                                            |
|------------|------------------------------------------------------------------|
| Status     | Draft — design only, no code landed                              |
| Track      | Protocol (cryptographic payload layer)                           |
| Author     | The Quidnug Authors                                              |
| Created    | 2026-04-20                                                       |
| Requires   | QDP-0002 (guardian recovery), QDP-0012 (domain governance), QDP-0014 (discovery), QDP-0019 (reputation decay), QDP-0022 (timed trust) |
| Enables    | `AUTHORITY_DELEGATE` visibility class `private:*` (from QDP-0023), end-to-end encrypted messaging use cases, enterprise split-horizon records, confidential attestations |

## 1. Summary

Quidnug events are public by default. Every transaction is
stored on-chain, gossiped across validators, replicated to
caches, and queryable by anyone with network access to the
appropriate domain's data. This is correct for most use
cases (review text, wire authorization metadata, ballot
events, domain attestations), and the protocol leans into
it aggressively for the auditability guarantee.

But a growing class of use cases needs the opposite:
stored records that are **durably persisted on-chain,
cached by validators, but cryptographically unreadable
except to a defined set of quids**. Examples:

- **Enterprise split-horizon DNS.** Chase attests `chase.com`
  via QDP-0023 and delegates resolution to its own nodes; it
  wants to publish some records (partner API endpoints,
  employee directory) that are encrypted at rest and readable
  only by Chase employees with the right trust edges.
- **Confidential credential disclosure.** A diploma issued
  by a university is attested on-chain; the detailed grade
  transcript is encrypted and disclosable only to verifiers
  the student authorizes.
- **Encrypted messaging.** A "message to quid X" is an event
  visible to the network, but only quid X can read the content.
- **Sensitive consent records.** GDPR/HIPAA-type consent
  events may carry PII that must be encrypted at rest per
  regulation (QDP-0017 intersection).
- **Board / privileged communications.** A corporate board
  communicates via on-chain events; auditable that communication
  occurred, but content private to board members.

QDP-0024 specifies how this works:

1. **Group-keyed encryption** using TreeKEM (MLS-style) for
   efficient membership management in groups of any size.
2. **Epoch-based key rotation** where each group advances
   through epochs; past members retain access to past-epoch
   records but not future-epoch records.
3. **On-chain key distribution** where wrapped keys are
   published as events so any qualified member can
   bootstrap without prior contact.
4. **Integration with `AUTHORITY_DELEGATE`** so the `private:*`
   visibility class in QDP-0023 has a concrete cryptographic
   backing.
5. **Guardian recovery compatibility** so a member who loses
   their key can recover access to past records they were
   entitled to.

Non-confidentiality guarantees: the *existence* of a private
record, its size, timing, and the group it belongs to remain
public by design (any observer sees "a 4 KB encrypted record
was published to group G at time T"). This is a deliberate
tradeoff for auditability; applications needing traffic-
analysis resistance layer other primitives on top.

## 2. Goals and non-goals

**Goals:**

- Encrypt record payloads such that only members of a named
  group (set of quids) can decrypt.
- Support groups from 2 members (private 1:1 conversation)
  to 10,000+ members (enterprise employee directory) with
  logarithmic membership-update cost.
- Rotate group keys on membership change (post-compromise
  security): a removed member cannot read records written
  after their removal.
- Preserve historical access: a member who was present in
  epoch N retains the ability to decrypt epoch-N records
  even after leaving.
- Compose with `AUTHORITY_DELEGATE` so enterprises can run
  split-horizon DNS / services trivially.
- Work alongside guardian recovery (QDP-0002) so lost-key
  members regain access.
- Use well-reviewed cryptography (MLS per RFC 9420, X25519,
  AES-GCM-256, HKDF-SHA256).

**Non-goals:**

- Hide that encrypted records exist. Metadata (group, size,
  timing) is public.
- Provide anonymity for the writer. Writers are identified
  by their quid signature; the *content* is encrypted but
  the authorship is not.
- Provide forward secrecy in the traditional messaging sense.
  Past members retain past-epoch keys (by design — they were
  authorized at the time). Applications wanting forward
  secrecy can additionally rotate per-record keys (see §9).
- Replace end-to-end messaging protocols (Signal, Matrix) for
  real-time chat. This is for durable on-chain records, not
  ephemeral messages.
- Resist traffic analysis. Observers can correlate publication
  patterns.

## 3. Threat model

### 3.1 Adversaries

**T1. Network observer.**
- Capability: can see every event flowing across Quidnug
  gossip, every stored record in every cache, every query
  response.
- Cannot: decrypt records for groups they're not a member of.

**T2. Cache-replica operator.**
- Capability: operates a cache replica; holds persistent
  copies of all records in the domains it caches.
- Cannot: decrypt records for groups they're not a member of,
  even with full cache access.

**T3. Removed group member.**
- Capability: was a group member until epoch N; has all
  epoch keys up to N.
- Cannot: decrypt records published in epoch N+1 or later.

**T4. Compromised group-member key.**
- Capability: adversary has stolen member M's private key
  during epoch N.
- Can read: all records in epochs M was a member of, up to N.
- Cannot read: records in epochs after the group rotates in
  response to the compromise (post-compromise security).

**T5. Colluding subset of group members.**
- Capability: some members agree to share their keys with an
  outside party.
- This is equivalent to adding the outsider to the group;
  the protocol does not resist it. Membership discipline is
  a governance concern, not a cryptographic one.

### 3.2 What the design defends and doesn't

| Property | Defended | Mechanism |
|---|---|---|
| Confidentiality vs non-member | ✅ | Group key unknown to non-members |
| Cache-at-rest encryption | ✅ | Records stored ciphertext |
| Post-compromise security | ✅ | Epoch rotation on suspected compromise |
| Forward secrecy within epoch | ⚠️ (optional) | Per-record keys; opt-in |
| Removed-member past access | ✅ (deliberate) | Past epoch keys retained |
| Removed-member future access | ✅ (defended) | New epoch keys not delivered |
| Metadata (size, timing, group) | ❌ (by design) | Publicly visible for auditability |
| Writer anonymity | ❌ (by design) | Signatures identify writer |
| Member collusion | ❌ (governance problem) | Out of scope |

## 4. Cryptographic building blocks

### 4.1 Primitives

| Purpose | Primitive |
|---|---|
| Key agreement | X25519 (Curve25519 ECDH) |
| Symmetric authenticated encryption | AES-GCM-256 |
| Key derivation | HKDF-SHA256 |
| Tree-based group keying | TreeKEM (from MLS, RFC 9420) |
| Signature (for member-owned events) | Ed25519 (optional; ECDSA-P256 default remains for identity signing) |

X25519 is preferred over P-256 for ECDH because MLS specifies
X25519 (RFC 9420 §5.1) and performance is better for leaf-key
ephemeral operations. ECDSA-P256 remains the identity signing
algorithm.

### 4.2 Group structure

A **group** is a set of member quids with a shared key derived
via TreeKEM. Types:

| Type | Membership determined by | Use |
|---|---|---|
| Static | Explicit `GroupMembershipSet` event (list of quids) | Small, hand-managed (a board) |
| Dynamic | Trust edges into a domain (e.g., everyone trusted in `bank.chase.employees`) | Large, graph-defined |
| Hybrid | Static set ∪ dynamic set | Mix (named employees + anyone in a trust domain) |

For dynamic groups, the "current membership" is computed by
the relying node at query time (trust-graph walk). Key
distribution is pre-computed: whenever a new quid gains the
relevant trust edge, the group's current-epoch key is wrapped
for their public key and published.

### 4.3 Epochs

Each group has a monotonic `epoch` counter. An epoch is
defined by its group key `K_epoch` plus the membership
snapshot at the moment of epoch advance.

Epoch transitions happen on:

1. **Membership change** (add, remove, role change).
2. **Scheduled rotation** (per group policy, e.g., every 90
   days).
3. **Compromise response** (explicit `EPOCH_ADVANCE` event
   signed by group governor).

Records are tagged with the epoch they were encrypted
under. Decryption requires `K_epoch` for the matching epoch.

## 5. TreeKEM-based key derivation

This is a brief specification; normative reference is
RFC 9420 §7-8.

### 5.1 Tree shape

- Binary tree with `n` leaves for `n` members.
- Leaves hold member-specific leaf keys (X25519 keypairs).
- Parent nodes hold secrets derived from their children.
- Root secret = group key `K_epoch`.

### 5.2 Key derivation

For each internal node, its secret is:

```
parent_secret = HKDF-Expand(
    HKDF-Extract(
        salt = "MLS 1.0 path secret",
        ikm  = HKDF-Expand(left_child_secret, "tree", 32) ||
               HKDF-Expand(right_child_secret, "tree", 32)
    ),
    info = "internal_secret",
    L    = 32
)
```

(Simplified; see RFC 9420 §8 for full derivation.)

Root secret is the group epoch key:

```
K_epoch = HKDF-Expand(root_secret, "group_key", 32)
```

### 5.3 Path updates

When member M changes (add, remove, key rotation), only the
path from M's leaf to the root must be recomputed. This is
`O(log n)` rather than `O(n)`.

The member triggering the change publishes:

- New ciphertexts for each node on the path, encrypted to
  the *resolution* of each node's co-path (the sibling subtree's
  union of leaves).
- These are packaged as an `EPOCH_ADVANCE` event (§6.2).

Every other member can derive the new path secrets from
their own leaf key + the published ciphertexts along the
co-path.

### 5.4 Member additions

When adding member M:

1. Find the leftmost empty leaf (or blank node) in the tree.
2. M's leaf key is set to their X25519 pubkey (published as
   a `MEMBER_KEY_PACKAGE` event).
3. Path from M's leaf to root is recomputed.
4. `EPOCH_ADVANCE` publishes the new path secrets (encrypted
   to co-path resolutions so existing members can update).
5. M's "welcome" is a separate ciphertext, encrypted to M's
   leaf key, that contains the initial path secrets needed
   for M to compute `K_epoch`.

### 5.5 Member removals

When removing member M:

1. M's leaf is **blanked** (set to unknown).
2. Path from M's leaf to root is recomputed by a remaining
   member (typically the group owner / designated remover).
3. `EPOCH_ADVANCE` published; M does not receive the update.

Blanked leaves can be reused on next addition. Tree can be
compacted periodically.

## 6. On-chain event types

### 6.1 `GROUP_CREATE`

Establishes a new group.

```go
type GroupCreatePayload struct {
    GroupId             string   `json:"groupId"`          // canonical id
    GroupName           string   `json:"groupName"`        // "bank.chase.employees"
    GroupType           string   `json:"groupType"`        // "static" | "dynamic" | "hybrid"
    StaticMembers       []string `json:"staticMembers"`    // quids for static/hybrid
    DynamicTrustDomain  string   `json:"dynamicTrustDomain,omitempty"` // for dynamic/hybrid
    DynamicMinTrust     float64  `json:"dynamicMinTrust,omitempty"`    // threshold
    InitialEpochKey     string   `json:"initialEpochKey"`  // ciphertext of K_epoch_0
    TreeShape           string   `json:"treeShape"`        // serialized initial tree
    Policy              GroupPolicy `json:"policy"`
}

type GroupPolicy struct {
    RotationIntervalSeconds int64 `json:"rotationIntervalSeconds"` // 0 = no scheduled rotation
    GovernorQuids           []string `json:"governorQuids"`        // who can force advance
    GovernanceQuorum        float64  `json:"governanceQuorum"`
    MaxMembers              int      `json:"maxMembers,omitempty"` // hard cap; 0 = unbounded
    HistoryRetention        string   `json:"historyRetention"`     // "forever" | "90d" | "1y"
}
```

Signed by the initial group creator. Emitted on the group's
control domain (e.g., `groups.private.<owner-domain>.<group-name>`).

### 6.2 `EPOCH_ADVANCE`

Advances group to a new epoch.

```go
type EpochAdvancePayload struct {
    GroupId           string `json:"groupId"`
    PreviousEpoch     int    `json:"previousEpoch"`
    NewEpoch          int    `json:"newEpoch"`
    ReasonCode        string `json:"reasonCode"` // "member-added" | "member-removed" | "scheduled" | "compromise-response"
    AddedMembers      []string `json:"addedMembers,omitempty"`
    RemovedMembers    []string `json:"removedMembers,omitempty"`
    PathUpdate        string `json:"pathUpdate"`        // base64 TreeKEM update
    WelcomePackages   map[string]string `json:"welcomePackages,omitempty"` // quid -> ciphertext for new members
    EffectiveAt       int64  `json:"effectiveAt"`
}
```

Signed by a member authorized to advance (by group policy:
usually any member, or governor quorum for compromise-response).

### 6.3 `MEMBER_KEY_PACKAGE`

Publishes a member's long-lived X25519 public key for group
participation. Separate from the member's identity-signing
ECDSA key.

```go
type MemberKeyPackagePayload struct {
    MemberQuid       string `json:"memberQuid"`
    X25519PublicKey  string `json:"x25519PublicKey"`  // hex
    ValidUntil       int64  `json:"validUntil"`       // Unix ns
    Ciphersuite      string `json:"ciphersuite"`      // "MLS_128_DHKEMX25519_AES128GCM_SHA256_Ed25519"
    Credential       string `json:"credential"`       // link to identity
}
```

Signed by the member's identity key.

### 6.4 `ENCRYPTED_RECORD`

Standard event type with an encrypted payload.

```go
type EncryptedRecordPayload struct {
    GroupId         string `json:"groupId"`
    Epoch           int    `json:"epoch"`
    ContentType     string `json:"contentType"`    // "dns-record" | "message" | "credential" | ...
    Nonce           string `json:"nonce"`          // 96-bit GCM nonce, hex
    Ciphertext      string `json:"ciphertext"`     // AES-GCM(K_epoch, nonce, plaintext, aad)
    AAD             string `json:"aad,omitempty"`  // associated data, hex
    SenderLeafIndex int    `json:"senderLeafIndex"` // for audit; optional
}
```

Plaintext shape depends on content-type. For DNS records used
under `AUTHORITY_DELEGATE` visibility class `private:<group>`:

```
plaintext = {
  "recordType": "A",
  "value": "10.0.0.5",
  "ttl": 300,
  "extensions": {...}
}
```

### 6.5 `MEMBER_INVITE`

Sends a "welcome package" to a newly-added member.

```go
type MemberInvitePayload struct {
    GroupId              string `json:"groupId"`
    InvitedMemberQuid    string `json:"invitedMemberQuid"`
    InvitingMemberQuid   string `json:"invitingMemberQuid"`
    WelcomeCiphertext    string `json:"welcomeCiphertext"` // encrypted to invited member's leaf
    EpochSnapshot        int    `json:"epochSnapshot"`     // epoch this welcome corresponds to
    EffectiveAt          int64  `json:"effectiveAt"`
}
```

Invited member decrypts the welcome with their X25519 private
key (matched to their published `MEMBER_KEY_PACKAGE`) to
extract their leaf secret and derive `K_epoch`.

### 6.6 `MEMBER_KEY_RECOVERY`

Integrates with guardian recovery (QDP-0002). When a member
loses their X25519 key, their guardian quorum emits a
recovery that:

1. Publishes a new `MEMBER_KEY_PACKAGE` with a new X25519
   pubkey.
2. Requests a "welcome re-issue" from current group members
   so the recovered member regains access to past-epoch keys
   they were entitled to.

```go
type MemberKeyRecoveryPayload struct {
    MemberQuid         string   `json:"memberQuid"`
    GroupIds           []string `json:"groupIds"`
    NewX25519PublicKey string   `json:"newX25519PublicKey"`
    GuardianSignatures []string `json:"guardianSignatures"` // per QDP-0002
    PriorKeyRevoked    bool     `json:"priorKeyRevoked"`
}
```

After recovery, existing members re-wrap past-epoch keys for
the recovered member's new X25519 key. Member regains full
history access up to the recovery point. Their prior key is
invalidated.

## 7. Record lifecycle

### 7.1 Writing an encrypted record

```
1. Writer holds leaf secret for current epoch.
2. Writer derives K_epoch via TreeKEM tree walk.
3. Writer generates fresh 96-bit nonce.
4. plaintext = canonical form of payload
5. ciphertext = AES-GCM-256(K_epoch, nonce, plaintext, aad)
6. Writer publishes ENCRYPTED_RECORD event with
   groupId + epoch + nonce + ciphertext.
7. Event signed by writer's identity key.
```

### 7.2 Reading an encrypted record

```
1. Reader fetches ENCRYPTED_RECORD event + associated group
   metadata.
2. Reader checks membership: am I in group G at epoch E?
   - If yes: continue.
   - If no: abort (can't decrypt).
3. Reader derives K_epoch for epoch E from their stored
   path secrets.
4. plaintext = AES-GCM-256-Decrypt(K_epoch, nonce, ciphertext, aad)
5. Reader returns plaintext to application.
```

### 7.3 Membership change

```
Adding member M:
1. Governor/authorized member signs ADD request.
2. M publishes MEMBER_KEY_PACKAGE (if not already present).
3. Group tree updated: M assigned a leaf.
4. EPOCH_ADVANCE event published with path update.
5. MEMBER_INVITE published with welcome package for M.
6. M decrypts welcome, derives current epoch secrets.
7. M can now read all records in the new epoch + any
   welcome-included past-epoch keys.

Removing member M:
1. Governor/authorized member signs REMOVE request.
2. M's leaf is blanked.
3. EPOCH_ADVANCE event published with path update.
4. Remaining members update their secrets from co-path.
5. M does NOT receive the update, so cannot derive new epoch.
6. M retains access to past epochs they were in (they have
   those K_epoch values cached).
```

### 7.4 Epoch rotation (no membership change)

Scheduled rotation advances the epoch without adding or
removing members. Any member can trigger:

```
1. Member selects a new leaf secret (replaces own leaf).
2. Member recomputes path.
3. EPOCH_ADVANCE published with "scheduled" reason + path
   update.
4. Every member updates secrets.
```

### 7.5 Compromise response

A group can declare a member's key compromised:

```
1. Governor quorum signs COMPROMISE_REPORTED for member M.
2. M's leaf immediately blanked (same as removal).
3. EPOCH_ADVANCE with reason="compromise-response".
4. Out-of-band investigation determines whether M's key was
   truly compromised or recovered.
5. If recovered (M found their key): regular re-add flow
   brings M back.
6. If compromised: M stays out; any records the attacker
   created are challenged via normal means (signature
   validation still requires the identity key, not just
   group key, so attacker can't have published events in
   M's name unless identity key was also lost).
```

## 8. Integration with `AUTHORITY_DELEGATE`

Per QDP-0023 §3.3, the `AUTHORITY_DELEGATE` event carries a
visibility policy. For records marked `private:<group-id>`:

1. Writer locates the group with `<group-id>`.
2. Writer encrypts the record payload per §7.1.
3. Writer publishes `ENCRYPTED_RECORD` event, tagged with
   `contentType` matching the record type (e.g.,
   `dns-record`).
4. Cache replicas store the ciphertext. Non-member queriers
   get NXDOMAIN (they can't decrypt anyway).
5. Member queriers decrypt per §7.2 and return the plaintext
   record to their application.

For records marked `trust-gated:<domain>`, no encryption is
needed. Cache replicas serve only to clients whose trust
graph reaches into `<domain>` above threshold. This is
cheaper (no crypto overhead) but weaker (cache replicas see
plaintext). Applications choose between `trust-gated:*` (faster)
and `private:*` (stronger).

## 9. Optional: per-record keys for enhanced forward secrecy

For especially sensitive records (e.g., medical consent), an
additional layer can be added:

1. Writer generates a fresh record key `K_record`.
2. Writer encrypts the plaintext with `K_record`.
3. Writer encrypts `K_record` with the group `K_epoch`.
4. Published ciphertext = group-encrypted(K_record) ||
   K_record-encrypted(plaintext).

If `K_record` is zeroized from memory after publication and
never persisted, then even full disclosure of `K_epoch` by a
compromised member doesn't recover the plaintext unless the
record key is captured separately. (Trade-off: writer can no
longer re-read their own past records without re-storing
`K_record`. Applications that need to re-read should skip
this layer.)

## 10. Group discovery

A member needs to find:

- Which groups they belong to.
- Current epoch of each group.
- Current `MEMBER_KEY_PACKAGE` for every peer member.

Discovery endpoints (extends QDP-0014):

```
GET /api/v2/groups/{member_quid}
  returns: list of groups this quid is a member of.

GET /api/v2/groups/{group_id}/members
  returns: current members + their X25519 public keys.

GET /api/v2/groups/{group_id}/epochs
  returns: epoch history (ids + timestamps; not keys).

GET /api/v2/groups/{group_id}/records?since={epoch}
  returns: encrypted records since the given epoch.
```

All queries answerable by any cache replica. No decryption
performed server-side.

## 11. Performance

### 11.1 Key derivation cost

Per RFC 9420 benchmarks, TreeKEM operations on X25519:

| Operation | Cost for n=1000 members |
|---|---|
| Add member (path update) | ~10 ms |
| Remove member (path update) | ~10 ms |
| Derive epoch key (startup) | ~50 ms |
| Encrypt record | ~0.1 ms |
| Decrypt record | ~0.1 ms |

Scales as `O(log n)` for membership operations, `O(1)` for
encryption/decryption.

### 11.2 Storage overhead

Each `ENCRYPTED_RECORD` is approximately:

```
base_event_overhead (signatures, ids, nonces) = ~400 bytes
aes_gcm_nonce = 12 bytes
aes_gcm_tag = 16 bytes
ciphertext = |plaintext|
```

Ciphertext size is essentially equal to plaintext (AES-GCM
is not length-expanding beyond the 16-byte tag). Total
overhead per encrypted record: ~430 bytes beyond the plaintext
payload.

For a 50-byte DNS record, overhead is ~8x. Amortizes
favorably for larger payloads.

### 11.3 Epoch-advance bandwidth

Per `EPOCH_ADVANCE` event, the path update is approximately:

```
path_size = ceil(log2(n_members)) × 96 bytes
```

For n=1000 members: ~10 × 96 = ~960 bytes.
For n=10000 members: ~14 × 96 = ~1344 bytes.

Each welcome package for a newly-added member is ~2 KB.
Gossip bandwidth impact is minimal.

## 12. Security analysis

### 12.1 Confidentiality

- Non-members cannot decrypt records because they lack the
  path secrets needed to derive `K_epoch`.
- TreeKEM's security reduces to the hardness of computing
  ECDH over X25519 (discrete log in the Curve25519 group).
- AES-GCM-256 provides authenticated encryption; tampering
  is detected.

### 12.2 Post-compromise security

- When member M is compromised and removed, `EPOCH_ADVANCE`
  excludes them.
- New epoch secrets are derived from the new tree state; M
  has no path secrets in the new tree.
- Therefore M cannot decrypt records from the new epoch
  onward.
- Compromise-response assumes the group detects the compromise.
  Undetected compromises remain a concern; mitigate with
  scheduled rotation (`rotationIntervalSeconds` in group
  policy).

### 12.3 Member-key-package freshness

- `MEMBER_KEY_PACKAGE` events have `ValidUntil` per QDP-0022.
- Members should rotate their X25519 key periodically (90d
  recommended).
- Stale key packages are rejected by the protocol.

### 12.4 Identity vs group key separation

Member's identity-signing ECDSA-P256 key is **separate** from
their X25519 group-participation key. Compromise of one does
not imply compromise of the other. Compromise of the identity
key is more serious (attacker can forge events in the
member's name); compromise of the X25519 key allows reading
group records but not forging them.

### 12.5 Metadata leakage

Metadata (group, size, timing, writer identity) is public.
Observers can:

- Count encrypted records per group.
- Correlate record publication with external events.
- Identify which members are active writers.

Applications needing stronger metadata privacy should:

- Pad records to fixed sizes (cover-traffic).
- Publish at fixed intervals regardless of content
  (rate-padding).
- Consider off-chain communication channels for high-
  sensitivity data.

These are application-layer concerns outside QDP-0024 scope.

## 13. Integration with existing QDPs

### 13.1 QDP-0002 (guardian recovery)

Member key recovery described in §6.6. Guardians authorize
new X25519 pubkey; existing members re-wrap past-epoch
secrets for the new key.

### 13.2 QDP-0012 (domain governance)

Group governance domain per group. Governor quorum can:
- Force `EPOCH_ADVANCE` for compromise response
- Update group policy (rotation interval, max members)
- Dissolve the group (revoke all member access)

### 13.3 QDP-0015 (content moderation)

Encrypted records can still be subject to moderation:
- Moderation targets the *event* (takedown), not the
  plaintext.
- Suppress/hide/annotate actions apply at the event layer
  before any attempt at decryption.
- Key escrow is **not** supported; moderators cannot decrypt.

### 13.4 QDP-0017 (data subject rights)

GDPR erasure of encrypted records proceeds via cryptographic
shredding:
1. Owner-of-group destroys all copies of relevant
   `K_epoch` values.
2. Records become mathematically undecryptable (AES-GCM-256
   is indistinguishable from random to anyone without the key).
3. QDP-0017 `DATA_SUBJECT_REQUEST` events can trigger this
   workflow.

Compatible with append-only ledger: records remain in place
but are permanently unreadable.

### 13.5 QDP-0022 (timed trust)

Group membership can be time-bounded: a member's trust edge
can have `ValidUntil` causing automatic removal at expiry
(for dynamic groups).

### 13.6 QDP-0023 (DNS-anchored attestation)

Primary consumer: `AUTHORITY_DELEGATE` with
`visibility.record_types.*.private:<group-id>` relies on
QDP-0024 groups.

## 14. Use-case walk-throughs

### 14.1 Enterprise split-horizon DNS (Chase)

1. Chase attests `chase.com` via QDP-0023.
2. Chase creates group `bank.chase.employees`
   (`dynamic`, trust-edges into `bank.chase.employees`
   domain).
3. Chase creates group `bank.chase.partners` (hybrid: static
   list of partner-company quids + dynamic).
4. Chase emits `AUTHORITY_DELEGATE` for `chase.com`:
   - `A` records: public
   - Partner-API records: `trust-gated:bank.chase.partners`
   - Employee-directory records: `private:bank.chase.employees`
5. Chase publishes `ENCRYPTED_RECORD` events for
   employee-directory records, encrypted with the employees
   group's current epoch key.
6. Employee querying directory: cache replica returns
   ciphertext, client decrypts locally.
7. External observer querying directory: cache replica
   returns NXDOMAIN (trust-gate check fails before
   ciphertext is even offered).

### 14.2 Confidential credential disclosure (University)

1. University attests `<uni>.edu` via QDP-0023 (free tier).
2. Student earns credential; university publishes a
   `CREDENTIAL_ISSUED` event referencing a private group
   `credentials.<uni>.edu.<student-quid>`.
3. Student is sole member of their credential group.
4. Student "shares" with a verifier by adding the verifier
   to the group (temporary membership).
5. Verifier joins, reads credential detail, exits.
6. Group continues with just the student; additional shares
   can be made similarly.

### 14.3 Board-level communications

1. Board creates a static group `board.bigcorp` with the
   seven board member quids.
2. Group policy: 90-day scheduled rotation, governor quorum
   required for member changes.
3. Board meetings / communications posted as
   `ENCRYPTED_RECORD` events on `board.bigcorp`.
4. Audit trail: "a 2 KB encrypted record was posted on
   date X" is public, content private.
5. Rotation on member change: when a board member leaves,
   group governor quorum removes them; `EPOCH_ADVANCE`
   published; departing member can't read future records.

### 14.4 Encrypted 1:1 messaging

1. Alice and Bob each publish `MEMBER_KEY_PACKAGE` events.
2. Alice creates group `im.<alice-quid>.<bob-quid>` with just
   Alice + Bob as static members.
3. Messages are `ENCRYPTED_RECORD` events on that group.
4. Either party can rotate the epoch (e.g., daily) for
   incremental forward secrecy.
5. Group dissolution removes all future access for both
   parties (useful for "delete this conversation" UX).

## 15. Implementation plan

### 15.1 Phase 1: Core crypto (~2 person-weeks)

- Implement X25519, HKDF-SHA256, AES-GCM-256 primitives
  (already available in Go `crypto/*` and Python
  `cryptography`).
- Implement TreeKEM per RFC 9420 §7-8 (there are reference
  implementations; Adams, Mulmuley, Barnes et al. published
  Rust + Go reference code).
- Unit tests against RFC 9420 test vectors.

### 15.2 Phase 2: Event types (~1 person-week)

- Add `GROUP_CREATE`, `EPOCH_ADVANCE`, `MEMBER_KEY_PACKAGE`,
  `ENCRYPTED_RECORD`, `MEMBER_INVITE`, `MEMBER_KEY_RECOVERY`
  to the node's event-type registry.
- Validation rules per event type.
- Storage integration.

### 15.3 Phase 3: SDK helpers (~2 person-weeks)

- Python + Go SDK client APIs:
  - `group.create(name, type, members, policy)`
  - `group.add_member(quid)`
  - `group.remove_member(quid)`
  - `group.rotate_epoch()`
  - `group.encrypt_record(content_type, plaintext)`
  - `group.decrypt_record(event)`
- Guardian-recovery integration.

### 15.4 Phase 4: Query endpoints (~1 person-week)

- Discovery endpoints per §10.
- Cache-replica honoring (no decryption server-side).

### 15.5 Phase 5: Documentation + examples (~1 person-week)

- Cookbook entries for each use-case walk-through (§14).
- Migration guidance for existing `trust-gated` records to
  `private`.

Total: ~7 person-weeks to full deployment.

## 16. Open questions

**Q1. Should TreeKEM be the only supported scheme?**

For very small groups (2-10 members), a simple
"encrypt-to-each-member" approach is simpler with negligible
overhead. Proposal: support both, tagged in `GROUP_CREATE`:
`keyScheme: "treekem"` vs `keyScheme: "direct-wrap"`.

**Q2. How do we prevent group-membership metadata from leaking
     sensitive organizational structure?**

Employee-directory groups reveal org structure to any
observer. For truly sensitive organizations, groups should
be named with opaque identifiers (`group-a3f2b9`) rather than
descriptive names (`bank.chase.executive-team`).

**Q3. How does cross-network private communication work?**

If Alice is on network N1 and Bob is on network N2, they can
create a group whose events are cross-federated per QDP-0013.
`MEMBER_KEY_PACKAGE` events import across federation;
`ENCRYPTED_RECORD` events appear in both networks' caches.
No special handling needed.

**Q4. Should we support asymmetric group membership?**

Some use cases want "readers but not writers" or "writers
but not readers." Proposal: group policy includes
`readerQuids` vs `writerQuids`; writers have both
identity-signing authority and K_epoch access, readers have
only K_epoch access. Implementable as a layer on top of
core TreeKEM.

**Q5. Post-quantum readiness?**

X25519 is not post-quantum secure. A future QDP should
migrate to a post-quantum KEM (ML-KEM per FIPS 203) for the
key-agreement layer. Signatures still rely on ECDSA; a
broader post-quantum transition is coordinated protocol-wide
via fork-block (QDP-0009).

**Q6. Key-package rotation UX?**

If a member forgets to rotate their `MEMBER_KEY_PACKAGE`
before `validUntil`, they're effectively locked out. SDK
should warn 14 days before expiry and offer one-click
rotation. Enterprise integrations should automate rotation
via cron-like scheduling.

## 17. References

- RFC 9420: The Messaging Layer Security (MLS) Protocol
- RFC 9180: Hybrid Public Key Encryption (HPKE)
- RFC 7748: Elliptic Curves for Security (X25519)
- RFC 5869: HKDF
- FIPS 197: AES
- [QDP-0002: Guardian-Based Recovery](0002-guardian-based-recovery.md)
- [QDP-0012: Domain Governance](0012-domain-governance.md)
- [QDP-0015: Content Moderation](0015-content-moderation.md)
- [QDP-0017: Data Subject Rights](0017-data-subject-rights.md)
- [QDP-0022: Timed Trust](0022-timed-trust-and-ttl.md)
- [QDP-0023: DNS-Anchored Identity Attestation](0023-dns-anchored-attestation.md) (primary consumer)
- Signal Protocol documentation: "The Double Ratchet Algorithm"
- Wire security white paper (MLS deployment at scale)
- [`UseCases/enterprise-domain-authority/`](../../UseCases/enterprise-domain-authority/) — demonstration consumer
