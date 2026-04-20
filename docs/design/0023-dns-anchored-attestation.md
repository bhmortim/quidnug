# QDP-0023: DNS-Anchored Identity Attestation

| Field      | Value                                                            |
|------------|------------------------------------------------------------------|
| Status     | Draft — design only, no code landed                              |
| Track      | Protocol + ecosystem                                             |
| Author     | The Quidnug Authors                                              |
| Created    | 2026-04-20                                                       |
| Requires   | QDP-0012 (domain governance), QDP-0013 (network federation), QDP-0014 (node discovery + sharding), QDP-0019 (reputation decay), QDP-0022 (timed trust) |
| Enables    | QDP-0024 (private communications), every existing use case that wants a "verified by DNS" trust signal |

## 1. Summary

Existing DNS has a large installed base, a well-known
ownership model (registrar + registrant), and nearly universal
client reach. It is also brittle in the ways catalogued in
`UseCases/dns-replacement/README.md`: centralized roots,
registrar seizure, weak identity binding, DNSSEC adoption
stuck under 20%, cache poisoning, BGP hijacking, censorship
at every layer, bolt-on TLS PKI.

The original `dns-replacement` use case proposed a parallel
`.quidnug` namespace: convince people to migrate names over
time. That pitch works for crypto-native projects and
seizure-resistance use cases but will never reach mass
adoption. Nobody wakes up wanting a second DNS.

QDP-0023 specifies the **DNS anchor**: a standardized,
automated, fee-paid mechanism for binding an existing DNS
domain (`example.com`) to a Quidnug quid. The domain owner
keeps their current DNS exactly as it is today. In parallel
they publish a verifiable attestation on the Quidnug network
saying "this quid controls this DNS name." Attestation roots
(federated, per QDP-0013) verify the claim by checking DNS
records + TLS fingerprints + WHOIS + blocklist data, then
sign a `DNS_ATTESTATION` event.

Downstream every use case in the protocol can now reference
the DNS-anchored quid as "the operator of `example.com`." The
`reviews.public.*` domain knows which quid legitimately
represents `joespizza.com`. The interbank-wire federation
knows which quid represents `chase.com`. The agent-authorization
use case knows which quid represents the operator of `openai.com`.
All of this comes free to the domain owner after one
verification.

This QDP also specifies a **generic `AUTHORITY_DELEGATE`
event type** not tied to DNS. Once a subject has an attestation
(of any kind), they can delegate "who answers queries against
this attestation" back to their own nodes or a corporate-
controlled domain, with per-record-type visibility policy.
DNS is the first consumer; reviews, credential verification,
and any future use case that wants split-horizon semantics
will consume the same primitive.

The design is **markets-all-the-way-down**: no canonical root,
no monopoly pricing, no single point of control. Trust
weighting uses the existing graph primitives and reputation
decay to sort legitimate attestations from noise.

## 2. Goals and non-goals

**Goals:**

- Let any DNS domain owner bind their name to a Quidnug quid
  via a standardized, automated verification flow.
- Let multiple independent attestation roots coexist, compete,
  and federate. No canonical root.
- Use the existing trust-graph machinery to sort attestations
  by credibility (weight of transitive trust into the
  attesting root).
- Provide a generic `AUTHORITY_DELEGATE` primitive so the
  attestation owner can hand resolution authority back to
  nodes they control, with per-record-type visibility policy.
- Make revocation first-class: attestations can be revoked by
  the attesting root, by governor quorum on the root's own
  network, or by the domain owner.
- Compose cleanly with the rest of the protocol. No new consensus
  rules; everything is a `TRUST` or `EVENT` transaction.

**Non-goals:**

- Replacing DNS itself. This QDP layers on top of DNS; the
  existing dns-replacement use case documents the separate
  path toward a post-DNS parallel namespace.
- Certificate authority replacement. TLS fingerprint recording
  is for identity continuity, not certificate issuance.
- Payment processing on-chain. Fee settlement is off-chain
  (each root picks its payment rails); the on-chain record
  includes only a payment-proof reference.
- Dictating a single default trust-root list. The reference
  SDK ships an opinionated bootstrap, but users and
  downstream SDK consumers can override.

## 3. The three-layer model

### 3.1 Layer 1: Federated attestation roots

An **attestation root** is any Quidnug operator that:

1. Runs a consortium per QDP-0012.
2. Publishes governance for its `attestation.root.dns` domain
   (governor quorum, fee schedule, blocklists, verification
   rigor).
3. Runs at least the minimum verifier discipline (§5).
4. Emits `DNS_ATTESTATION` events against verified claims.

Roots are federated per QDP-0013. Any operator can stand one
up tomorrow without permission. quidnug.com plans to run the
first one; others can run competing roots with different fee
schedules, blocklists, and verification rigor.

Each root publishes its own `.well-known/quidnug-network.json`
(QDP-0014) declaring its:

- Root quid + pubkey
- Governor quorum
- Attestation-API endpoints
- Fee schedule reference
- Supported DNS TLDs + free-tier rules

### 3.2 Layer 2: Transitive trust weighting (client convention)

This is the critical departure from traditional PKI. An
attestation is **not** a boolean "this root said yes." It
carries a weight equal to the cumulative trust flowing into
the attesting root through the `roots.dns.attestation` trust
domain.

Clients compute weight with the existing primitive:

```
weight = GetTrustLevel(
    client_quid,
    attesting_root_quid,
    domain = "roots.dns.attestation",
    max_depth = <client-configurable; default 5>
)
```

Weight is in `[0.0, 1.0]`. Conceptually:

- `1.0`: the client (or its trust graph) has maximum confidence
  in the attesting root.
- `0.0`: the client has no evidence whatsoever supporting the
  attesting root.
- Anything in between: a soft signal.

When a client resolves `example.com`, it fetches *all*
`DNS_ATTESTATION` events for that domain from every federated
root it knows about, computes the weight of each, and picks
the highest-weighted. Alternatively it can surface them all
(e.g., "3 roots attest this, combined weight 0.97; 1 disagrees,
weight 0.02").

Per QDP-0019, trust decays over time without reinforcement.
A root that was trusted five years ago but has gone dark
loses weight. An attestation issued ten years ago by a still-
trusted root remains valid as long as the attestation itself
hasn't expired (attestations carry their own `validUntil`
per QDP-0022).

This approach is a **client convention** not a protocol
primitive. The reference SDKs implement the weighting; other
implementations may choose different aggregation functions
(max, sum, median, etc.). The published methodology in §6
standardizes the recommended default.

### 3.3 Layer 3: Generic authority delegation

Once a subject owns an attestation, they can delegate
"who answers queries against this attestation" via a new
generic event type:

```
type: AUTHORITY_DELEGATE
payload:
  attestation_ref:  <event-id of the parent attestation>
  attestation_kind: "dns" | "review" | "credential" | ...
  subject:          "chase.com"                            # the attested thing
  delegate_nodes:   ["<quid>", "<quid>", ...]              # who serves the data
  delegate_domain:  "bank.chase"                           # domain whose stream carries records
  visibility:
    record_types:
      A:           "public"
      MX:          "public"
      TXT/public:  "public"
      API/*:       "trust-gated:partners.bank.chase"
      INTERNAL/*:  "private:bank.chase.employees"
    default:       "trust-gated:partners.bank.chase"
  fallback_public: false
  effective_at:    <unix-ns, optional>   # honor from this block onward
  valid_until:     <unix-ns, optional>   # per QDP-0022
signed_by:         <quid that owns the attestation>
```

Visibility classes:

- **public**: record is served from any cache replica to any
  querier. No auth required. Standard DNS behavior.
- **trust-gated:<domain>**: record served only to clients
  whose trust graph reaches into `<domain>` above a threshold
  (threshold specified in the visibility policy or root-
  defaulted). Non-qualifying querier gets NXDOMAIN
  indistinguishable from a nonexistent record.
- **private:<domain>**: same as trust-gated but with payload
  encryption per QDP-0024. Cache replicas store only
  ciphertext; only qualifying queriers can decrypt.

The generic design means:

- DNS uses it to split public records (website IP) from
  trust-gated records (partner APIs) from private records
  (internal directory).
- Reviews might use it for a merchant to serve "verified
  reviews" from their own nodes while letting cache tier
  serve summaries.
- Credential verification might use it for a university to
  serve transcripts only to authorized verifiers.
- Any future use case needing split-horizon gets this for
  free.

## 4. Transaction types

### 4.1 `DNS_CLAIM` (event)

Publishes intent to attest. Sent from domain owner to a
chosen attestation root.

```go
type DNSClaimPayload struct {
    Domain               string `json:"domain"`           // example.com
    OwnerQuid            string `json:"ownerQuid"`        // 16-hex
    RootQuid             string `json:"rootQuid"`         // the root being asked
    RequestedValidUntil  int64  `json:"requestedValidUntil"` // Unix ns; bounded by TLD rules
    PaymentMethod        string `json:"paymentMethod"`    // "stripe" | "crypto" | "waiver"
    PaymentReference     string `json:"paymentReference"` // off-chain receipt id
    ContactEmail         string `json:"contactEmail,omitempty"` // optional; for challenge-delivery fallback
}
```

Emitted on the root's `attestation.claims.<root-quid>` domain.
The root reads this and issues a challenge.

### 4.2 `DNS_CHALLENGE` (event)

Emitted by the root against the claim. Carries the nonce the
owner must publish.

```go
type DNSChallengePayload struct {
    ClaimRef             string `json:"claimRef"`         // parent DNS_CLAIM event id
    Nonce                string `json:"nonce"`            // 32 random bytes hex
    ChallengeExpiresAt   int64  `json:"challengeExpiresAt"`  // Unix ns; default +7d
    TXTRecordName        string `json:"txtRecordName"`    // "_quidnug-attest.example.com"
    WellKnownURL         string `json:"wellKnownURL"`     // "https://example.com/.well-known/quidnug-domain-attestation.txt"
    RequiredContentTemplate string `json:"requiredContentTemplate"` // see §5.1
}
```

### 4.3 `DNS_ATTESTATION` (event)

The binding. Emitted by the root after successful verification.

```go
type DNSAttestationPayload struct {
    ClaimRef             string  `json:"claimRef"`
    Domain               string  `json:"domain"`
    OwnerQuid            string  `json:"ownerQuid"`
    RootQuid             string  `json:"rootQuid"`
    TLD                  string  `json:"tld"`                // "com" | "gov" | "edu" | ...
    TLDTier              string  `json:"tldTier"`            // "free-public" | "standard" | "premium" | "luxury"
    VerifiedAt           int64   `json:"verifiedAt"`         // Unix ns
    ValidUntil           int64   `json:"validUntil"`         // Unix ns; bounded by TLD rules
    TLSFingerprintSHA256 string  `json:"tlsFingerprintSHA256"` // captured at verification
    WHOISRegisteredSince int64   `json:"whoisRegisteredSince"` // Unix seconds from WHOIS
    BlocklistCheckedAt   int64   `json:"blocklistCheckedAt"`
    BlocklistsChecked    []string `json:"blocklistsChecked"`  // ["ofac", "phishtank", ...]
    VerifierNodes        []string `json:"verifierNodes"`      // 3+ verifier quids
    ResolverConsensus    []ResolverResult `json:"resolverConsensus"`
    FeePaidUSD           float64 `json:"feePaidUSD"`
    PaymentMethod        string  `json:"paymentMethod"`
    PaymentReference     string  `json:"paymentReference"`
}

type ResolverResult struct {
    ResolverLabel  string `json:"resolverLabel"`  // "google-8.8.8.8", "cloudflare-1.1.1.1", etc.
    TXTValue       string `json:"txtValue"`
    ObservedAt     int64  `json:"observedAt"`
}
```

Emitted on the root's `attestation.dns.<root-quid>` domain.
Propagated to every federation peer.

### 4.4 `DNS_RENEWAL` (event)

Re-verifies before expiry. Must be submitted within the
renewal window (default: 30 days before `validUntil`).

```go
type DNSRenewalPayload struct {
    PriorAttestationRef  string  `json:"priorAttestationRef"`
    NewValidUntil        int64   `json:"newValidUntil"`
    NewTLSFingerprintSHA256 string `json:"newTLSFingerprintSHA256"`  // may differ if cert rotated
    FingerprintRotationProof string `json:"fingerprintRotationProof,omitempty"` // chain of CT-log entries connecting old to new
    FeePaidUSD           float64 `json:"feePaidUSD"`
    PaymentReference     string  `json:"paymentReference"`
}
```

### 4.5 `DNS_REVOCATION` (event)

Revokes an attestation. Signable by (in order of precedence):

```go
type DNSRevocationPayload struct {
    AttestationRef       string `json:"attestationRef"`
    RevokerQuid          string `json:"revokerQuid"`
    RevokerRole          string `json:"revokerRole"`   // "root" | "governor-quorum" | "owner"
    Reason               string `json:"reason"`        // "fraud-detected" | "owner-request" | "transfer" | "malfeasance" | ...
    RevokedAt            int64  `json:"revokedAt"`
    GovernorSignatures   []string `json:"governorSignatures,omitempty"` // required if role="governor-quorum"
}
```

Revocation does not delete the attestation from history. It
sets `validUntil` to the revocation timestamp and emits a
revocation event the client-side weight calculation honors
(revoked attestations contribute 0.0 weight regardless of
trust into the root).

### 4.6 `AUTHORITY_DELEGATE` (event, generic)

Defined in §3.3. Independent of DNS; consumable by any
attestation use case.

### 4.7 `AUTHORITY_DELEGATE_REVOCATION` (event, generic)

Mirror of `DNS_REVOCATION` for delegation. Signable by the
attestation owner (always) or the attesting root (for abuse
cases where the delegated nodes are serving malicious content
under the root's attestation).

## 5. Verification protocol

### 5.1 Owner-side: publishing the challenge response

After receiving a `DNS_CHALLENGE`, the owner publishes both:

**DNS TXT record** at `_quidnug-attest.<domain>`:

```
v=1; quid=<16-hex>; nonce=<32-hex>; sig=<hex-ecdsa-sig-over-canonical-bytes>
```

where the signed canonical bytes are:

```
"quidnug-dns-attest-v1\n" +
domain + "\n" +
quid + "\n" +
nonce + "\n" +
root_quid + "\n" +
challenge_event_id
```

**Well-known file** at `https://<domain>/.well-known/quidnug-domain-attestation.txt`:

```
Identical byte-for-byte content as the TXT record value.
```

Both must be in place. The `_quidnug-attest.*` subdomain
scope keeps the TXT record out of bare-domain TXT noise
(DKIM, SPF, etc.). The well-known file proves HTTPS control,
not just DNS control, which defeats nameserver-only
hijacking.

### 5.2 Root-side: verification pass

A verifier node performs, in parallel, from at least three
geographically-separated vantage points:

1. **Multi-resolver DNS TXT lookup.** Query `_quidnug-attest.<domain>`
   via (at minimum) Google Public DNS (8.8.8.8), Cloudflare
   (1.1.1.1), Quad9 (9.9.9.9), and one non-US resolver (e.g.,
   Yandex or a European public resolver). All must return the
   same value. Any discrepancy flags for manual review.
2. **HTTPS well-known fetch.** Retrieve the well-known file
   over HTTPS. Record the TLS certificate fingerprint (SHA-256
   of the leaf cert in DER) observed during the fetch.
3. **Content equality check.** TXT value and well-known body
   must match byte-for-byte.
4. **Signature validation.** Verify the embedded ECDSA
   signature with the declared quid's public key (fetched
   from the quid's current identity state on the Quidnug
   network).
5. **WHOIS age check.** Fetch WHOIS for `<domain>`. The
   `Creation Date` must be at least **30 days** before the
   claim time. Younger domains are rejected (anti-typosquat,
   anti-drive-by).
6. **Blocklist intersection.** Query at minimum:
   - OFAC sanctioned domains list
   - Google Safe Browsing
   - PhishTank
   - Spamhaus DBL
   - National CSAM registries (where applicable)
   Intersection with the claim domain is a rejection.
7. **TLD-tier policy check.** Enforce the root's fee schedule:
   free tiers must match the TLD (and registry cross-check if
   the root policy requires it); paid tiers must have payment
   cleared.
8. **Rate-limit check.** The claimant quid must not have
   submitted more than N claims in the last hour across this
   root (prevents spam).

All eight must pass. Any failure aborts the verification and
publishes a `DNS_CLAIM_REJECTED` event with the reason. The
claimant can address the issue and resubmit.

### 5.3 Verification rigor levels

Roots may publish their own verification-rigor level in their
`.well-known/quidnug-network.json`. The reference SDK
recognizes three levels and consumes them in the trust-weight
computation:

| Level | Minimum checks | Appropriate for |
|---|---|---|
| `basic` | TXT + well-known + signature + blocklist | Hobbyist / FOSS projects / low-stakes |
| `standard` | Basic + 3-resolver consensus + WHOIS age + TLS fingerprint | Commercial sites / SaaS / e-commerce |
| `rigorous` | Standard + trademark check + KYC + 30-day public-notice window | Financial / healthcare / government |

Higher-rigor attestations are weighted slightly higher in the
default client aggregation (convention, not protocol rule):

```
effective_weight = transitive_trust_weight × rigor_multiplier
  where rigor_multiplier = { basic: 0.8, standard: 1.0, rigorous: 1.1 }
```

Roots self-declare their rigor level; verifier peers check
that emitted attestations carry the claimed checks in their
payload (§4.3) and will demote the root in their local trust
graph if claims don't match evidence.

## 6. Trust-weighting methodology (published standard)

This section is the convention §3.2 references. Reference
SDKs implement this algorithm; alternate clients may deviate
but should document their deviation.

### 6.1 Per-attestation weight

For a client querying `example.com` and finding `N` attestations:

```
for each attestation a_i:
    t_i = GetTrustLevel(client_quid, a_i.root_quid,
                        domain="roots.dns.attestation",
                        max_depth=5)
    r_i = rigor_multiplier(a_i.rigor_level)
    age_factor = 1.0 if not revoked, else 0.0
    freshness = decay_curve(now - a_i.verified_at)
    w_i = t_i × r_i × age_factor × freshness
```

Where `decay_curve` is per QDP-0019: attestations don't decay
for the first 6 months, then half-life 18 months. A fresh
attestation carries full weight; a 3-year-old attestation
carries about 1/4.

### 6.2 Aggregation across attestations

Default aggregation is **max** of per-attestation weights
for each claimed owner, then pick the highest-weighted owner:

```
by_owner = group(attestations, by=attestation.owner_quid)
for owner, atts in by_owner:
    owner_weight[owner] = max(w_i for a_i in atts)
winner = argmax(owner_weight)
```

Alternative aggregations (available as client config):

- **max-with-confirmation**: pick max only if at least 2 roots
  agree on the owner.
- **sum-capped**: sum weights up to 1.0 (multi-root reinforces).
- **median**: median of per-attestation weights (robust to
  outliers; appropriate for high-stakes decisions).

### 6.3 Disagreement handling

If two roots attest the same domain to different owners, the
client:

1. Computes weight for each claimed owner.
2. If one weight is >= 3x the other, the high-weight owner
   wins cleanly.
3. Otherwise surfaces the disagreement to the application
   (e.g., "this domain has contested ownership; proceed with
   caution").

Browsers and UI tooling should render contested attestations
visibly differently from clean attestations. The protocol
doesn't force a resolution; the user or application decides.

### 6.4 No-attestation fallback

For domains with zero Quidnug attestations, the client
returns a null trust result. Applications can:

- Fall back to traditional DNS (the domain isn't claimed in
  Quidnug; that's fine).
- Display an "unverified" badge.
- Refuse to process (high-security applications).

The protocol doesn't force a policy.

## 7. Fee model

Fees are policy, set by each root, updated via governor
quorum on the root's governance domain. The reference
schedule that quidnug.com commits to at launch:

### 7.1 Free tiers

| TLD / pattern | Rationale |
|---|---|
| `.gov` | US federal; registry cross-checked against GSA |
| `.edu` | US higher-ed; registry cross-checked against EDUCAUSE |
| `.mil` | US military |
| `.int` | International treaty orgs |
| `.onion` | Tor hidden services; already cryptographic |
| `.<cc>.gov.<cc>`, `.gov.uk`, `.edu.au`, `.ac.jp`, etc. | National public-sector zones |

Free tiers require domain-owner-quid + signature; no payment.
Root may still charge for premium support.

### 7.2 Standard tier ($5 / year reference)

| TLDs |
|---|
| `.com`, `.net`, `.org`, `.biz`, `.info` |
| Most country-code TLDs (`.us`, `.ca`, `.uk`, `.de`, `.fr`, `.jp`, ...) |
| Most generic new gTLDs (`.store`, `.online`, `.tech`, `.site`, ...) |

### 7.3 Premium tier ($25 / year reference)

| TLDs |
|---|
| `.ai`, `.io`, `.app`, `.dev`, `.ly`, `.xyz`, `.co` |
| Short / memorable / high-demand gTLDs |

### 7.4 Luxury tier ($100 / year reference)

| Pattern | Examples |
|---|---|
| Single-letter domains | `x.com`, `m.co` |
| Two-letter `.com` | `ai.com`, `ok.com` |
| Common dictionary words under `.com` | `car.com`, `home.com` |
| Celebrity / trademark high-value | per-case |

Luxury tier requires a **30-day public-notice window**: the
claim is posted for dispute before the attestation is
issued. Any pre-existing attestation holder or trademark
holder can file a challenge during this window.

### 7.5 Fee governance

The fee schedule lives as a document under the root's
`governance.attestation.fees` domain. Updating requires a
governor quorum signature + 24-hour notice period (emergency
changes use the standard QDP-0012 emergency clause with 1-hour
notice).

Fee proceeds fund the root's verifier operation. Each root
publishes a quarterly transparency report (transaction count,
gross revenue, operating cost, remainder) on its governance
domain.

### 7.6 Competition dynamics

Federated roots compete on:

- **Fee level**: cheaper roots attract volume but carry less
  trust weight if rigor slips.
- **Verification rigor**: higher-rigor roots justify higher
  prices (rigor level declared in network.json).
- **Free-tier generosity**: a root may offer free `.com`
  attestations to seed adoption.
- **Jurisdictional neutrality**: a root outside a given
  jurisdiction may be more attractive to dissidents + activist
  domains.
- **Payment rails**: crypto-only roots, USD-only roots, EUR-
  only roots, etc.

"Attested by multiple roots" becomes a legitimate compounded
trust signal. A domain with three independent root attestations
stacks trust; phishing operations won't get all three because
each root enforces independent checks.

## 8. Client root-preference bootstrap

### 8.1 Default list

The reference SDK ships with a default list in
`clients/<lang>/src/quidnug/defaults/attestation_roots.json`:

```json
{
  "version": 1,
  "roots": [
    {
      "quid": "<quidnug-com-root-quid>",
      "well_known": "https://quidnug.com/.well-known/quidnug-network.json",
      "default_enabled": true,
      "editorial_notes": "Operated by quidnug.com; funds protocol development. Rigor: standard+."
    }
  ],
  "transparency_report": "https://quidnug.com/transparency/attestation-roots"
}
```

At launch, only quidnug.com's root ships in the default list.
As additional reputable roots come online, the SDK ships
updates. Inclusion criteria:

- Root must be operational for 6+ months.
- Root must publish transparency reports.
- Root must commit publicly to the reference verification
  rigor (§5.3 standard tier minimum).
- No conflicting governance with quidnug.com's operations.

The transparency report documents every inclusion decision
and the reasoning. Users who disagree with quidnug.com's
editorial choices can override.

### 8.2 User customization

Users can:

- **Add** additional roots (e.g., their employer's internal
  root, a region-specific root they prefer).
- **Remove** any root including quidnug.com's.
- **Weight** roots manually (override default).
- **Sign** their preference list with their quid; preferences
  sync across devices.

Preference config schema:

```json
{
  "version": 1,
  "signed_by": "<user-quid>",
  "timestamp": <unix-ns>,
  "roots": [
    {"quid": "<root-a>", "enabled": true, "weight_override": null},
    {"quid": "<root-b>", "enabled": true, "weight_override": 0.5},
    {"quid": "<root-c>", "enabled": false}
  ]
}
```

### 8.3 Transparency report

Every default-list decision is logged on the Quidnug network
itself (on a `roots.dns.attestation.policy.quidnug-com`
domain). Anyone can query the history: when was root X
added, when was root Y removed, what was the stated reason.

## 9. Security analysis

### 9.1 Attack vectors

**A1. Adversary takes temporary control of DNS nameservers.**

- Attack: adversary hijacks nameserver (via registrar social
  engineering) and posts their own TXT record.
- Mitigation: multi-resolver consistency catches short-lived
  hijacks (attack must persist across all resolvers
  simultaneously). TLS fingerprint check catches hijacks
  that don't also compromise the TLS cert. Well-known-file
  check requires HTTPS control, not just DNS. WHOIS age
  check is a stability signal.
- Residual risk: a sophisticated long-term nameserver-plus-
  TLS compromise could pass verification. Defense-in-depth
  relies on CT-log monitoring catching the cert issuance
  and on the owner's guardian quorum recovering the
  attestation.

**A2. Adversary compromises registrar account.**

- Attack: registrar account social-engineered; adversary
  transfers the domain + reissues certs.
- Mitigation: same as A1 (multi-resolver, TLS fingerprint
  continuity via CT logs). New TLS fingerprint without a
  plausible CT-log rotation chain is a rejection signal on
  renewal.
- Residual risk: if a registrar is fully compromised and
  can also coordinate a clean TLS reissuance from a colluding
  CA, the adversary can pass verification. Users mitigate
  this by paying multiple independent roots; adversary needs
  to compromise each root's view separately.

**A3. Adversary stands up their own attestation root.**

- Attack: adversary runs a "root" that claims to attest
  `chase.com` to an adversary quid.
- Mitigation: transitive trust weighting. Adversary's root
  has no trust flowing into it; its attestations carry
  weight near 0. Clients either ignore or surface the
  disagreement (weighted comparison sorts it).
- Residual risk: none as long as the weighting methodology
  is honestly computed.

**A4. Adversary takes over the domain owner's Quidnug quid.**

- Attack: adversary compromises the domain owner's private
  key.
- Mitigation: guardian recovery per QDP-0002. Owner's
  guardian quorum can re-sign a new attestation binding.
  Adversary's transactions during the compromise window get
  flagged once recovery publishes.
- Residual risk: during the window between compromise and
  recovery, adversary can act as the owner. Time-bounded to
  the owner's detection + recovery latency.

**A5. Attestation root colludes with adversary.**

- Attack: a root deliberately issues attestations to
  adversary quids for domains the adversary doesn't own.
- Mitigation: governance quorum on the root's network can
  revoke individual attestations + the root's trust graph
  will suffer as other verifier-peer nodes demote it. Other
  roots + independent monitors surface the contradiction.
- Residual risk: a colluding root with a small user base
  can harm its own users until reputation collapses.
  Widespread collusion is detectable via cross-root monitoring.

**A6. Owner's guardian quorum compromised while attestation
     active.**

- Attack: adversary fully compromises both owner quid and
  their guardian set.
- Mitigation: out of scope; this is equivalent to total
  identity compromise. Domain-level revocation by the root
  (on evidence of fraud) stops the bleeding.

**A7. Revocation abuse.**

- Attack: a root wrongfully revokes an attestation.
- Mitigation: multi-root redundancy — if other roots still
  attest, domain still has valid binding. Domain owner can
  appeal to the governor quorum on the misbehaving root's
  network.
- Residual risk: a monopoly single-root user suffers if
  "their" root misbehaves. Mitigation: we advise paying ≥2
  roots for any domain where availability matters.

**A8. Sybil farming of free-tier attestations.**

- Attack: adversary registers many `.gov` subdomains (via
  compromised `.gov` zones) and attests each.
- Mitigation: `.gov` / `.edu` verification checks the
  parent registry (GSA / EDUCAUSE). Subdomain attestations
  require a fresh claim per subdomain. Rate limits on claim
  submission. Higher rigor-level free tiers (governor approval
  per attestation) available.

**A9. Fee-model adversary stands up a free-for-all root.**

- Attack: "DirtCheapRoot" charges $0 for any domain with
  zero verification and floods the network with fake
  attestations.
- Mitigation: trust weighting naturally assigns near-zero
  weight to this root. Other roots' attestations dominate.
  Default-list curation keeps DirtCheapRoot out of the
  reference SDK default.
- Residual risk: user who manually adds DirtCheapRoot to their
  trust graph gets the attestation weight they asked for.
  Caveat emptor.

**A10. Attestation replay / rollback.**

- Attack: adversary replays a revoked attestation.
- Mitigation: revocation events are durable on-chain. Client
  weight computation consults revocation state. Per QDP-0001,
  transaction nonces prevent simple replay.

### 9.2 Threats explicitly out of scope

- **Full state-level root compromise** where an adversary controls
  DNS, TLS PKI, CT logs, and a majority of federated roots
  simultaneously. No design resists a universal adversary.
- **Side-channel identification** of domain owners via
  verification metadata (timing, IP). Mitigations in QDP-0024
  (private communications) apply to the signal channel; the
  attestation itself is public by design.

## 10. Integration with existing use cases

### 10.1 Reviews (QRP-0001)

A review's subject domain today is a free-text label:
`reviews.public.<sha256-of-domain>`. With DNS-anchored
attestations, `sha256-of-domain` maps to an attested quid.
The reviews ecosystem can:

- Display "verified merchant" on reviews for attested domains.
- Reject reviews for the same domain under conflicting
  attestations (defeats review-farm domain spoofing).
- Weight a merchant's response to reviews higher when their
  quid matches an attested binding.

### 10.2 Interbank wire authorization

`UseCases/interbank-wire-authorization/` already has the
`bank.<id>` domain structure. DNS-anchored attestation adds
a cryptographic binding from the bank's real DNS name to
this domain. Counterparty banks can verify a wire's
originator by resolving `chase.com` through Quidnug
attestation rather than consulting SWIFT BIC directories.

Authority delegation (§3.3) is natural here: Chase delegates
`chase.com` resolution to their `bank.chase` domain, which
already runs their consortium per operations.md.

### 10.3 AI agent authorization

`UseCases/ai-agent-authorization/` has agents acting on
behalf of organizations. An agent claiming to represent
`openai.com` can be cryptographically verified via:

1. DNS attestation binding `openai.com` to a quid.
2. Sub-identity delegation from that quid to the agent quid.
3. Scoped capability attestation from the org to the agent.

### 10.4 Content authenticity (C2PA integration)

An article published on `nytimes.com` can carry a C2PA
manifest signed by a key bound via DNS attestation to the
domain. The C2PA verifier queries Quidnug for the current
`nytimes.com` attestation + verifies the signing key chains
back to it.

### 10.5 Credential verification

University issuing a diploma signs with a key whose quid is
attested to `<university>.edu` (free tier). Verifier queries
the attestation, confirms the signing key, accepts the
credential. No separate university-key-discovery problem.

## 11. Implementation plan

### 11.1 Phase 1: Core protocol (~3 person-weeks)

- Implement the five event types (`DNS_CLAIM`, `DNS_CHALLENGE`,
  `DNS_ATTESTATION`, `DNS_RENEWAL`, `DNS_REVOCATION`) +
  generic `AUTHORITY_DELEGATE` + `AUTHORITY_DELEGATE_REVOCATION`.
- Add event-type validation to the node's registry.
- Add query endpoints:
  - `GET /api/v2/dns/attestations/{domain}` returns all
    attestations for a domain across federation.
  - `GET /api/v2/dns/attestations/{domain}/weighted?observer={quid}`
    returns attestations sorted by computed weight for the
    given observer quid.
  - `GET /api/v2/dns/resolve/{domain}/{record_type}` returns
    record data honoring authority delegation.
- Ship the reference trust-weighting algorithm in the Go SDK
  and Python SDK.

### 11.2 Phase 2: Reference verifier (~2 person-weeks)

- `cmd/quidnug-dns-verifier` Go binary that performs the full
  verification pass (§5.2).
- Multi-resolver DNS client.
- HTTPS fetcher with TLS fingerprint capture.
- WHOIS client (with fallback registries per TLD).
- Blocklist query integration (OFAC, PhishTank, Spamhaus).
- Signature validator.
- Emits `DNS_ATTESTATION` event on success / `DNS_CLAIM_REJECTED`
  on failure.

### 11.3 Phase 3: Owner CLI + SDK helpers (~1 person-week)

- `quidnug-cli dns claim` subcommand: starts a claim,
  generates the TXT record + well-known content, prompts
  the user to publish.
- `quidnug-cli dns verify` subcommand: submits the
  challenge-response and waits for attestation.
- `quidnug-cli dns renew` subcommand.
- Python + Go SDK helpers matching the same flow.

### 11.4 Phase 4: Public verification UI (~2 person-weeks)

- Web app at `verify.quidnug.com` for non-CLI users.
- Step-by-step: paste domain, generate quid (or use existing),
  Stripe-out fee, watch verification progress in real time.
- Transparency dashboard showing recent attestations + root
  operational statistics.

### 11.5 Phase 5: Federation hardening (~2 person-weeks)

- Federation between roots (TRUST_IMPORT for cross-root
  attestation visibility).
- Cross-root monitoring: detect contradictory attestations
  across roots + automatic escalation.
- Client SDK weighting aggregation in all reference SDKs.

Total: ~10 person-weeks to full deployment.

## 12. Alternatives considered

### 12.1 DNSSEC-based anchor

Publish the attestation binding in a DNSSEC-signed DNS record
and rely on DNS's existing delegation chain. Rejected:

- DNSSEC adoption is <20% on `.com`.
- Trust chain terminates at ICANN root, reintroducing
  centralization.
- Key rotation is painful.
- No guardian recovery.

### 12.2 X.509 / EV cert extension

Extend TLS certificate EV attributes to carry the Quidnug
quid. Rejected:

- CA-dependent; adds another centralized trust chain.
- Requires CA cooperation per certificate issuance.
- EV is declining in browser UI prominence.

### 12.3 Blockchain-on-Ethereum (ENS-style)

Host attestations on Ethereum via an ENS-like registry.
Rejected:

- Gas fees per update.
- Dependency on Ethereum availability.
- No key recovery (matches ENS limitation, one of our key
  differentiators).

### 12.4 Centralized registrar

quidnug.com as sole registrar + attester. Rejected:

- Single point of failure + control.
- Antithetical to protocol design goals.
- Limits pricing competition.

## 13. Open questions

**Q1. Who holds the payment processor relationship?**

Each root manages its own. quidnug.com's root uses Stripe
initially (merchant of record); other rails can be added
later.

**Q2. What happens if a root's governor quorum loses
cohesion (founders leave, organizational collapse)?**

The root's attestations remain valid until their natural
expiry. New attestations cannot be issued. Downstream clients
see expiry events naturally per QDP-0022 and fall back to
other roots. Domain owners can re-verify at another root.

**Q3. How are jurisdictional conflicts handled?**

A US-run root may be subject to US court orders (OFAC
sanctions, DOJ takedowns). A root outside US jurisdiction is
not. Users pick which roots they trust based on their risk
tolerance. The default list is opinionated; users can
override.

**Q4. Can an attestation root itself be attested?**

Yes. quidnug.com's root quid can itself be DNS-anchored to
`quidnug.com` (via a peer root). This creates a clean
self-referential story: "verify the root by verifying it
owns its own DNS name."

**Q5. How does this interact with QDP-0014 `.well-known`
files?**

Domain's `.well-known/quidnug-network.json` can include a
reference to its current attestations. QDP-0014 is extended
in Phase 5 to parse this field and surface attestation status
in discovery responses.

**Q6. Minimum attestation lifetime?**

Recommended: 1 year (matches typical DNS registration).
Shorter lifetimes possible but increase renewal overhead.

**Q7. Data-retention policy for expired / revoked
attestations?**

On-chain: append-only; never deleted (audit requirement).
Default display: expired/revoked not surfaced after 90 days.
Full history available via direct query.

## 14. References

- [QDP-0002: Guardian-Based Recovery](0002-guardian-based-recovery.md)
- [QDP-0012: Domain Governance](0012-domain-governance.md)
- [QDP-0013: Network Federation Model](0013-network-federation.md)
- [QDP-0014: Node Discovery + Domain Sharding](0014-node-discovery-and-sharding.md)
- [QDP-0019: Reputation Decay & Time-Weighted Trust](0019-reputation-decay.md)
- [QDP-0022: Timed Trust & TTL Semantics](0022-timed-trust-and-ttl.md)
- [QDP-0024: Private Communications & Group-Keyed Encryption](0024-private-communications.md) (companion)
- [`UseCases/dns-replacement/`](../../UseCases/dns-replacement/) — parallel namespace use case
- [`UseCases/enterprise-domain-authority/`](../../UseCases/enterprise-domain-authority/) — split-horizon example
- RFC 2616 (HTTP/1.1) — well-known URI discovery pattern
- RFC 5785 — well-known URIs convention
- RFC 8615 — well-known URIs update
- RFC 6844 (CAA) — analogous DNS-record-based policy
- RFC 7858 / 8310 — DoT / DoH (resolver-side comparison)
