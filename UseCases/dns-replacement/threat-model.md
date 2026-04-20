# Threat model — DNS on Quidnug

What this design defends against, what it inherits from the
protocol layer, what's out of scope, and where the limits are.

## 1. Threat categories

Four kinds of adversary to consider:

1. **Impersonator** — wants responses for a domain they don't
   own (phishing, mail hijacking, CA mis-issuance bypass).
2. **Censor** — wants to prevent a domain from resolving
   (nation-state, ISP, court order).
3. **Thief** — wants to take ownership of a domain from its
   legitimate owner (theft, extortion, seizure).
4. **Disruptor** — wants to degrade service (DDoS, cache
   poisoning, resource exhaustion).

For each, we trace how DNS fails today, what Quidnug does
differently, and where residual risk lives.

## 2. Impersonator attacks

### 2.1 DNS cache poisoning (Kaminsky-style)

**DNS today:** By predicting the 16-bit query ID + source
port, an attacker can race legitimate responses and poison a
resolver's cache. DNSSEC defeats this but adoption is low.

**Quidnug DNS:** Every response is cryptographically signed
by the domain's current governor. A forged response that
doesn't verify is rejected at the resolver. The attacker
can't forge because they don't have the governor's key.

**Residual risk:** none. The signature makes forgery
infeasible regardless of transport.

### 2.2 Typosquatting (`example.quidnug` vs `examp1e.quidnug`)

**DNS today:** Endemic. Registrars happily register lookalikes.

**Quidnug DNS:** Same problem — anyone can register unclaimed
names. The TLD operator can implement trademark-dispute
processes via governance, but the underlying issue is social,
not technical.

**Mitigation:**

- Browser + resolver UI should surface "is this a known
  deceptive lookalike?" based on cross-referencing a
  curated list of trusted domains.
- TLDs could require a mild proof-of-work or fee for
  registration to increase attacker cost.
- For high-value names, a TLD operator can maintain a
  "verified brands" trust list signed into the chain, and
  resolvers / browsers can surface "this is the
  brand-verified example.quidnug" vs "this is a lookalike."

**Residual risk:** same as DNS. Partially mitigated at the UI
layer, not at the protocol.

### 2.3 Response spoofing in transit

**DNS today:** Plain DNS over UDP is trivially spoofable at
any point between resolver and client.

**Quidnug DNS:** All Quidnug API traffic is over HTTPS with
CA-validated TLS; inside the response, the DNS event is
signed by the domain's governor. Two layers of cryptographic
protection.

The gateway → legacy client link still speaks plain DNS, so
the last-mile is the same as DNSSEC: clients doing
DNSSEC-validation get integrity; clients not doing
DNSSEC-validation don't.

**Residual risk:** for legacy clients, same as today's DNS.
For Quidnug-native clients, none.

### 2.4 BGP hijacking the resolver path

**DNS today:** An adversary announcing a victim's prefix
redirects resolver → authoritative-server traffic to their
own server.

**Quidnug DNS:** The gateway / resolver path is HTTPS, which
prevents modification. But an attacker who hijacks the
route to `api.quidnug.com` can return signed-by-attacker
responses — except the attacker's signatures don't verify
against the domain's governor pubkey, so resolvers reject
them.

**Residual risk:** BGP hijacking can cause denial of
service (resolver can't reach the real api gateway) but
not forgery. Mitigate with anycast, multiple federated
endpoints, and DoH-style fallback.

### 2.5 Certificate authority mis-issuance

**TLS today:** A compromised or coerced CA can issue a valid
certificate for any domain. Certificate Transparency helps
detect post-hoc but doesn't prevent.

**Quidnug DNS:** Applications using DANE / TLSA validation
don't rely on CAs. The domain publishes its own TLS key
hash; clients verify against that. A compromised CA issuing a
valid-but-fake cert won't match the TLSA record and the
client rejects.

**Residual risk:** only for applications still in the CA
trust model. Migration to DANE is the mitigation.

## 3. Censor attacks

### 3.1 Nation-state blocking a domain at ISP resolvers

**DNS today:** ISPs return NXDOMAIN or redirect to a block
page. Users bypass via DoH/DoT, but if the blocked domain is
served by a CDN the state can block the CDN's IPs too.

**Quidnug DNS:** Multiple public networks (federation)
operate roots. A government blocking the main
`quidnug.com`-operated root doesn't affect a federated
alternative root run by, say, a consortium in another
jurisdiction. Users who care switch their resolver's
well-known file to an alternative.

**Residual risk:** ISP-level blocking of Quidnug API
traffic is still possible, but the attack surface is much
wider than blocking a few DNS resolvers — you have to block
a large set of HTTPS endpoints.

### 3.2 Court-ordered domain seizure

**DNS today:** A judge issues an order; the registrar or
registry flips the NS records or blackholes the domain. The
owner has no recourse without reversing the order.

**Quidnug DNS:** Domain ownership is cryptographic. A judge
can order the owner to surrender their key, or order the
governors of the TLD to delegate the domain elsewhere. The
first requires the owner's cooperation (they can refuse and
accept the contempt charge). The second requires a quorum
of TLD governors — if the governors are in a jurisdiction
out of the court's reach, they can't be compelled.

**Residual risk:**

- If the owner is in-jurisdiction: key surrender is possible
  via compulsion (though they can also invoke
  guardian-recovery to rotate the key post-surrender).
- If the TLD governors are in-jurisdiction: delegation
  takeover is possible. Mitigation: diversify TLD governors
  across jurisdictions (QDP-0012 supports this).
- Federation (QDP-0013) lets users migrate their trust to
  an alternative TLD operated by out-of-jurisdiction
  governors.

Quidnug makes seizure harder, not impossible. The key
improvement is requiring global coordination of multiple
parties vs. today's one-registrar-one-order model.

### 3.3 DNS root-server DDoS

**DNS today:** The 13 root servers have been targeted
multiple times (2002, 2015, 2022). They've mostly held up
due to anycast, but a sufficiently-resourced adversary
could take the root offline.

**Quidnug DNS:** There is no root server in the DNS sense.
The discovery API is served by many cache replicas, fronted
by CDN (Cloudflare, etc.), distributed globally. Taking it
offline requires attacking each consortium member + many
cache replicas simultaneously. Much harder.

**Residual risk:** low but non-zero. A massive coordinated
attack could degrade service; the system falls back to
stale cached responses gracefully.

### 3.4 "Kill switch" at the registrar

**DNS today:** GoDaddy, Namecheap, or any registrar can
unilaterally cancel a domain by policy (for ToS violations,
content complaints, sanctions lists). No cryptographic
appeal.

**Quidnug DNS:** There's no registrar-as-chokepoint. The TLD
governance can delete a delegation, but that requires a
quorum of governors. Registering a `.quidnug` domain
doesn't give any single party unilateral removal power.

**Residual risk:** low. If the TLD governance is captured,
that TLD can delete domains. Users who care about censorship
resistance should choose TLDs whose governance includes
independent non-captured governors.

## 4. Thief attacks

### 4.1 Stolen private key

**DNS today:** If your registrar account credentials are
phished, the attacker can change NS records, transfer the
domain, lock you out. Recovery depends on registrar support.

**Quidnug DNS:** If the domain's governor key is stolen, the
attacker can publish records as the legitimate owner until
the owner triggers guardian recovery. During the 24-hour
notice period, the attacker can publish whatever they want.

**Mitigations:**

- **Guardian recovery** (QDP-0002) — legitimate owner's
  guardian quorum signs a key-rotation with time-lock. The
  attacker can't stop this without compromising the
  guardian keys.
- **Multi-governor quorum** — a domain with 2-of-3 or 3-of-5
  governors requires the attacker to compromise multiple
  keys simultaneously.
- **Monitoring** — operators should have alerts on every
  record change for their high-value domains. An attacker
  change triggers immediate response.

**Residual risk:** 24-hour window between compromise
detection and recovery activation, during which the attacker
can publish malicious records. Shorten by running shorter
notice periods for high-value domains (configurable per-
governor).

### 4.2 Extortion-induced transfer

**DNS today:** Attacker threatens victim, demands domain
transfer. Victim clicks "transfer" at the registrar.
Irreversible.

**Quidnug DNS:** Same human problem — a victim under
coercion can execute `UPDATE_GOVERNORS` to the attacker's
quid. 24-hour notice gives time for external intervention
but doesn't help a victim under physical threat.

**Residual risk:** identical to DNS. This is a coercion
problem, not a cryptographic one.

### 4.3 Forgotten key

**DNS today:** Not applicable (credentials reset via email).

**Quidnug DNS:** If the owner loses the private key, they
lose the domain — unless they pre-configured guardian
recovery.

**Mitigation:** guardian recovery is mandatory for domains
you care about. Pick 3-5 guardians independent of you
(family, lawyer, bank, etc.). Document the recovery
procedure.

**Residual risk:** owners who don't set up guardians lose
domains permanently. User-education problem, not a
cryptographic one.

## 5. Disruptor attacks

### 5.1 DDoS against the API gateway

**DNS today:** UDP amplification is a known issue; mitigated
by rate limiting and EDNS cookies.

**Quidnug DNS:** The api gateway sits behind Cloudflare /
other CDN. DDoS is absorbed at the CDN edge. The origin
nodes see only CDN-ratelimited traffic.

**Residual risk:** small. A massive L7 attack could exhaust
Cloudflare bot mitigation, but the protocol layer isn't the
weak point.

### 5.2 Chain-state bloat via spam registrations

**Attack:** Attacker registers millions of meaningless
domains to bloat the chain.

**Mitigation:**

- Registration is a governance action at the TLD level.
  TLDs enforce rate limits, proof-of-work, or fees.
- Nodes can prune very-old unused domains from hot state
  (keep only `latest-per-key` summaries; archive the rest).

**Residual risk:** economic. TLD operators set their own
anti-spam policies.

### 5.3 Record-update spam

**Attack:** Legitimate owner publishes 1000 updates/second
to degrade the chain.

**Mitigation:** per-signer rate limits (existing at the
node level). A single quid can't publish faster than its
quota.

**Residual risk:** low. An attacker-controlled quid with
many sub-delegations could amplify, but each sub-domain has
its own rate limit.

### 5.4 Unavailable consortium members

**Attack:** All consortium members for a domain go offline
(failure, DDoS, coordinated).

**Mitigation:** cache replicas continue serving stale-but-
valid records up to their TTL. Consumers see degraded
freshness, not outright failure. Governance can promote
additional consortium members to replace downed ones.

**Residual risk:** prolonged outage (> TTL) means resolvers
return stale / NXDOMAIN. Bounded by the TTL, which is
operator-settable.

## 6. Attacks on delegation

### 6.1 Parent captures a child's records

**Attack:** The TLD consortium decides to publish records on
behalf of a delegated sub-domain, overwriting the owner's.

**Defense:** Once `DELEGATE_CHILD` activates, the parent
governors lose record-publication authority for that child.
Only the child's governors can publish records. If the parent
tries to publish anyway, the resolver sees the signer isn't
in the child's governor set and rejects.

**Residual risk:** the parent can `REVOKE_DELEGATION`
during the 24-hour notice period. Owners who suspect
revocation should watch for it + dispute during notice.

### 6.2 Child delegates to a fake parent

**Attack:** Attacker sets up a fake `.quidnug`-like TLD with
similar-looking branding, tricks users into delegating sub-
domains there, then takes over.

**Defense:** Same problem as DNS: `.quidnug-alt-scam`
isn't the real `.quidnug`. Users verify the TLD operator's
pubkey from the well-known file and pin it.

**Residual risk:** social engineering. Protocol can't
defend against users pointing their resolver at a scam TLD.

### 6.3 Circular delegation

**Attack:** `example.quidnug` delegates to `sub.example.quidnug`
which delegates back to `example.quidnug`.

**Defense:** `DELEGATE_CHILD` validation rejects this —
child must be structurally a descendant, and the delegation
graph is cycle-checked.

**Residual risk:** none. Detected at tx validation.

## 7. Attacks specific to the DNS gateway

### 7.1 Gateway returns fake responses

**Attack:** A compromised DNS gateway returns valid-DNSSEC-
signed responses with attacker-chosen IPs.

**Defense:** The gateway's DNSSEC key is separate from the
Quidnug governor key. A compromised gateway can sign bad
responses, and DNSSEC-aware clients will accept them (because
the gateway legitimately owns the DNSSEC key).

**Mitigation:** operators who care should use Quidnug-native
resolvers (not the gateway) for high-value names.
Alternatively, run your own gateway whose DNSSEC key you
control.

**Residual risk:** the gateway is a trust concentration
point. It's a legacy-compat layer, not a protocol
component. In the long run, eliminate the gateway by
upgrading clients to native Quidnug-DNS.

### 7.2 Gateway cache poisoning by the backend

**Attack:** A fake Quidnug node serves fake records to the
gateway, which then serves DNSSEC-signed wrong answers.

**Defense:** The gateway verifies Quidnug responses'
signatures before caching. It only trusts records signed by
governors known from the well-known file.

**Residual risk:** if the well-known file is compromised,
the gateway trusts fake governors. The gateway should pin
its well-known file at startup + refresh from a pinned URL
with TLS pin.

## 8. Attacks on TLS / DANE integration

### 8.1 Attacker controls the IP but not the domain

Say the attacker BGP-hijacks the IP for `example.quidnug`
and presents their own TLS cert.

**Defense:** Client looks up the TLSA record, hashes the
presented cert's pubkey, compares to the TLSA record. No
match → TLS handshake fails.

**Residual risk:** only for applications not using DANE.
For DANE-validating clients, BGP hijacking doesn't help the
attacker (they can deny service but not impersonate).

### 8.2 Attacker gets a CA to mis-issue a cert AND hijacks the IP

**Defense:** CA trust path is defeated by the TLSA check.
The presented cert has to match the domain's published
key; a CA-issued cert for a different public key doesn't.

**Residual risk:** none. This is DANE's whole purpose.

### 8.3 Attacker compromises the TLS server's private key

**Defense:** The published TLSA record still points at the
old key. The attacker can serve content using the stolen
key until the victim notices + rotates.

**Mitigation:** monitor TLS key usage. When suspicion hits,
publish a new TLSA record pointing at a fresh key, and
rotate the server's key simultaneously. The 24-hour delay
between the TLSA update and cache-replica visibility bounds
damage.

**Residual risk:** 24h window of stale TLSA info. Operators
can shorten TTL for ultra-paranoid deployments.

## 9. Residual risks summary

Things this design does NOT defend against:

- **Social engineering.** Users delegating to a lookalike TLD,
  giving away keys under coercion, not setting up guardians.
  The protocol limits what the attacker can do once they have
  the key; it can't prevent users from handing over keys.
- **Whole-network outages.** If every consortium member for a
  domain is offline AND the cache TTL expires, users can't
  resolve. Bound by the TTL, not by the protocol.
- **TLD governance capture.** If the TLD operators collude,
  they can revoke any delegation. Users should diversify by
  running multiple federated TLDs (QDP-0013).
- **Physical-layer denial.** A nation-state blocking HTTPS
  traffic to `api.quidnug.com` can deny service to its
  citizens. The protocol doesn't defeat transport-layer
  censorship (that's Tor/VPN/I2P's job).
- **Economic censorship.** A TLD charging for registrations
  excludes users without money. This is a policy problem.
  Choose a TLD with policies you like.
- **Fundamental cryptographic breaks.** If ECDSA P-256 is
  broken (a quantum computer, a novel attack), the whole
  system falls. Post-quantum migration is a future QDP.

## 10. Compared to DNS + DNSSEC failure modes

Side-by-side for the main threats:

| Threat | DNS without DNSSEC | DNS with DNSSEC | Quidnug DNS |
|---|---|---|---|
| Cache poisoning | ❌ vulnerable | ✅ defended | ✅ defended |
| BGP hijacking for resolver-authoritative path | ❌ allows forgery | ✅ signatures catch forgery | ✅ signatures catch forgery |
| Response spoofing in transit | ❌ vulnerable | ✅ defended at validator | ✅ defended + HTTPS in transit |
| Registrar seizure | ❌ possible | ❌ possible | ✅ requires governor-key compromise |
| Root authority capture | ❌ possible (ICANN) | ❌ possible (KSK) | Mitigated via federation |
| Key rotation | Fragile | Complex (DS records) | Simple (AnchorRotation) |
| Lost key recovery | Registrar support | None | Guardian recovery |
| CA mis-issuance for TLS | ❌ | Partial (via CAA) | ✅ (via DANE) |
| DDoS against root | Has happened | Has happened | No root to target |
| Cross-jurisdiction resilience | ❌ | ❌ | ✅ (federation) |

Quidnug-DNS at minimum matches DNSSEC on every security
property AND adds seizure resistance AND simplifies key
management AND makes DANE-style CA elimination practical.

## 11. Limits

Things to be realistic about:

1. **Adoption is the real enemy.** Any alternative DNS has to
   cross the chasm of "my browser doesn't know how to resolve
   this." The gateway bridges that, but gateways are also
   centralization points. Pure-Quidnug-native resolution
   requires changes to browsers / curl / systemd / etc.
2. **Governance attacks are social, not technical.** A
   well-funded adversary can buy-up TLD governors or infiltrate
   consortiums. The protocol helps detect + slow takeovers but
   can't prevent a majority of governors from colluding.
3. **IPv4 scarcity isn't solved by DNS.** Quidnug can point
   names at IPs; it can't make more IPs.
4. **Not a privacy tool.** Domain ownership and record updates
   are public on-chain. Private domains exist (separate
   networks) but the authoritative data is observable to
   anyone with the chain. Private naming use-cases require
   encrypted record payloads (future QDP).
5. **Not a content-addressing tool.** DNS names resolve to IPs
   or hostnames. IPFS-style content addressing is a separate
   problem (though Quidnug's `ipfsEnabled` + content-hash
   events give primitives for it).

## 12. Response playbooks

Operator-facing playbooks for the common scenarios:

### 12.1 "My domain's key was stolen"

1. Immediately notify guardians (out-of-band).
2. Initiate guardian recovery: `GuardianRecoveryInit` signed
   by M-of-N guardians.
3. During the 24h notice, monitor the domain for attacker
   actions. Record and publish any malicious records the
   attacker publishes so consumers can see them.
4. After notice: guardians commit the recovery, installing a
   new governor key.
5. Publish corrective records from the new key.
6. Optionally invalidate the old epoch via `AnchorInvalidation`
   so any old-key-signed records are frozen.
7. File a post-mortem. Update guardian setup if needed.

### 12.2 "A TLD governor account is compromised"

1. Notify the other governors.
2. Issue a `SUPERSEDE` governance tx to halt any pending
   malicious actions.
3. Issue `UPDATE_GOVERNORS` to remove the compromised key
   (requires unanimity of remaining governors).
4. Use guardian recovery to rotate the compromised
   governor's key to a new one.
5. Issue another `UPDATE_GOVERNORS` to add the new key
   back.
6. Publish a post-mortem.

### 12.3 "Nation-state pressure on a TLD operator"

1. Scenario: operator is ordered to delete a specific
   domain's delegation.
2. Short-term: operator complies (to avoid personal
   jeopardy).
3. Medium-term: affected users migrate their domain to a
   federated TLD operated out-of-jurisdiction.
4. Long-term: TLD governance distributes across more
   jurisdictions to prevent single-point pressure.

### 12.4 "A cached record is stale / wrong"

1. Identify the domain and record type.
2. Publish a corrective event.
3. Push-gossip propagates within ~5 seconds.
4. Cache replicas refresh their hot state on next push.
5. Legacy DNS gateway caches refresh on next TTL expiry.

Normal operation. No emergency workflow needed.

## 13. Known-unknowns

Open questions that need operational data to resolve:

1. **TTL-economics balance.** Short TTLs = faster updates but
   more query load. Long TTLs = stale-longer during incidents.
   Pick a default?
2. **Default guardian-quorum size for TLDs.** 3-of-5? 5-of-9?
   More = safer, also more operational overhead.
3. **Registration anti-squat policies.** Fee-based, proof-of-
   work, application-gated, or open? Tradeoffs depend on the
   TLD's goals.
4. **How aggressively to push legacy-compat.** DNS gateway is
   easy; browser-native integration is years of work across
   many vendors. Realistically, Quidnug-DNS lives alongside
   classical DNS for a long time.

None of these are blockers. All become clear with 6-12 months
of real deployment data.
