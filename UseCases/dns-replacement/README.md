# DNS Replacement

**Cross-industry · Critical internet infrastructure · Anti-centralization**

## The problem

DNS is the internet's name-to-address map and also one of its
weakest load-bearing pieces. It has nine serious flaws, any
one of which would be a design blocker if the system were
being proposed today:

1. **Centralized root authority.** ICANN and the 13 root-
   server operators sit at the top of a single tree. A
   jurisdictional change, corporate capture, or political
   pressure on any of them is a single point of influence on
   the whole internet.

2. **Registrar seizure.** You don't own your domain — you
   rent it. A court order, contract dispute, billing
   mishap, or your registrar's bankruptcy can take your
   name away. See: every seized-by-FBI banner on a .com.

3. **No real identity binding.** A domain is not
   cryptographically bound to its owner. The owner is
   whoever has the registrar account credentials. "Did
   example.com really send this email?" is answered with a
   mix of SPF / DKIM / DMARC tacked on over the years, none
   of which is in the DNS protocol itself.

4. **DNSSEC is a bolted-on retrofit.** It solves some of the
   integrity problem (cache poisoning, response forgery) but
   at huge operational cost: key rotation ceremonies, KSK /
   ZSK splits, DS records that require registrar cooperation
   to update. Adoption is < 20% of .com domains two decades
   in.

5. **Cache poisoning and spoofing.** Kaminsky 2008 was a
   wake-up call, but the underlying model — trust whatever
   you receive that matches a query ID — is still there for
   any domain that hasn't deployed DNSSEC.

6. **BGP-level hijacking.** DNS resolution is vulnerable to
   routing-layer attacks: an adversary announcing a
   hijacked prefix can redirect queries to their own
   servers. Happens repeatedly in the wild (2018 MyEtherWallet
   attack, 2022 KlaySwap).

7. **Censorship at every layer.** Nation-states block domains
   by poisoning ISP resolvers, hijacking routes, or leaning on
   registrars. DoH and DoT only move the problem.

8. **TLS is another centralized dependency.** To do HTTPS on
   `example.com` you rent a certificate from a CA. The CA is
   another trusted third party. Certificate Transparency helps
   detect compromise after the fact; it doesn't prevent it.

9. **Key lifecycle is fragile.** Rotating a DNS signing key
   means coordinating with your registrar, their parent zone,
   the root, and every caching resolver worldwide. Lost keys
   are catastrophic. There's no "guardian-based recovery."

Individually these are engineering annoyances. Collectively
they mean the internet's naming layer is one political
shift, one root-server-DDoS, or one BGP screwup away from a
bad day.

## Why Quidnug fits

Everything DNS tries to do is already a Quidnug primitive:

- **A domain is a signed, owned object.** In Quidnug, a
  `TrustDomain` has governors (QDP-0012) who cryptographically
  own it. Nobody can take it without the governors' keys.
- **A name-to-address mapping is a signed event.** DNS record
  types (A, AAAA, MX, TXT, SRV, TLSA) map one-for-one to
  `DNS_RECORD` events on the domain's event stream.
- **Every response is cryptographically verifiable.** No
  DNSSEC retrofit — signatures are built into the protocol.
- **Key rotation is a supported primitive.** `AnchorRotation`
  + guardian recovery (QDP-0002) handle key loss, rotation,
  and compromise.
- **Hierarchy is governance.** The `.quidnug` "TLD" is
  governed by its consortium; `example.quidnug` by its
  owners; `mail.example.quidnug` by whoever got delegation
  from `example.quidnug`. Exactly the DNS delegation model,
  with cryptographic enforcement instead of registrar trust.
- **There's no single root.** Federation (QDP-0013) means
  multiple public networks can each serve as a "root." Users
  pick which they trust.
- **Discovery is explicit.** QDP-0014 node-advertisement +
  per-domain hints let clients find where a name's data
  actually lives, without guessing at a flat NS list.

The one-liner: **DNS is a trust graph pretending not to be,
with cryptography bolted on after the fact. Quidnug is a
trust graph that knows it is, with cryptography built in
from the start.**

| Problem | Quidnug primitive |
|---|---|
| "Who owns `example.quidnug`?" | `Governors` map on the TrustDomain |
| "What's `example.quidnug`'s A record?" | `DNS_RECORD` event on the domain's stream |
| "Is the response real?" | Every event is signed by a governor |
| "How do I rotate my key?" | `AnchorRotation` transaction |
| "I lost my key!" | Guardian recovery (QDP-0002) |
| "ICANN seized my domain" | Not a thing; governance is cryptographic |
| "My ISP blocks me" | Multiple cache replicas + federated roots |
| "How do I prove this email came from my domain?" | Sign with the domain's governor key |
| "How do I bind my TLS cert to the domain?" | Publish a `DNS_TLSA_RECORD` event |
| "A CA mis-issued a cert for my domain" | CAs aren't in the loop; the domain-key is the TLS trust anchor |

## High-level architecture

```
                    Root consortium (public network)
                    ┌──────────────────────────────────┐
                    │ governs: .quidnug  (reserved tld) │
                    │  - issues child-domain delegations│
                    │  - maintains TLD-level policies   │
                    └──────────────────┬────────────────┘
                                       │ DELEGATE_CHILD
                                       ▼
                    Owner's TrustDomain: example.quidnug
                    ┌──────────────────────────────────┐
                    │ governors: {owner-quid: 1.0}     │
                    │  - publishes records (A, MX, TXT)│
                    │  - rotates keys via AnchorRotation│
                    │  - delegates subs via DELEGATE_CHILD│
                    └──────────────┬───────────────────┘
                                   │
                                   │ DNS_RECORD events
                                   │   {type: A,     value: 1.2.3.4}
                                   │   {type: MX,    value: mail.example.quidnug}
                                   │   {type: TXT,   value: "v=DKIM1; p=..."}
                                   │   {type: TLSA,  value: <tls-pubkey-hash>}
                                   │   ...
                                   ▼
                    Cache replicas + consortium members
                    ┌──────────────────────────────────┐
                    │ serve signed records to clients  │
                    │ via QDP-0014 discovery API       │
                    └──────────────────────────────────┘
                                   ▲
                                   │
                    Resolver library (client side)
                    ┌──────────────────────────────────┐
                    │ 1. Fetch domain state + records  │
                    │ 2. Verify signatures             │
                    │ 3. Return verified answer        │
                    └──────────────────────────────────┘
```

Everything client-facing looks like DNS: "give me the A
record for `mail.example.quidnug`, get back an IP." What's
different is every step in that pipeline is cryptographically
verifiable, the domain is owned by the person holding the
signing key (not by a registrar), and there's no single
hierarchy anyone can seize.

## What "replaces DNS" actually means

Not literally — DNS isn't going anywhere overnight. Realistic
adoption in three phases:

### Phase 1: Parallel namespace (now)

A new TLD (e.g. `.quidnug`) lives on the Quidnug network.
Early adopters publish records there, resolvers that support
it handle both. Legacy DNS still works; Quidnug names are
cryptographically-secured, centralization-free alternatives
for anyone who wants them.

**Use cases that win here:**

- Activists and journalists who need
  seizure-resistant publication.
- Projects critical of host governments (Tor, leak sites,
  political orgs) that can't rely on ICANN.
- Crypto and decentralized-protocol projects that need
  self-sovereign naming (ENS tried this with Ethereum;
  Quidnug does it more cheaply and with richer primitives).
- Apps that need strong cryptographic domain-to-key binding
  (PKI for secure email, DANE-integrated HTTPS).

### Phase 2: Bridge + gateway (6-18 months in)

DNS gateways that answer queries for `.quidnug` names over
standard UDP/TCP DNS, so any resolver (curl, Chrome, iOS) can
reach them without a Quidnug client library. The gateway
fetches the Quidnug record, verifies the signature, and
returns a standard DNSSEC-signed response.

This unlocks:

- Browsers accessing `.quidnug` URLs without special plugins
- Mail servers delivering mail to `example.quidnug`
- TLS certificates that validate against the domain's own
  published key instead of a CA's

### Phase 3: Alternative roots for existing TLDs (long-term)

An operator could mirror `.com`, `.org`, etc. into the
Quidnug namespace as an alternative, cryptographically-
protected source of truth. Skeptical? That's fine — it's opt-
in per client.

Users who don't trust ICANN's `.com` root pick a federated
alternative (the same mechanism operators already use per
QDP-0013 to pick which network's trust edges to accept). The
names don't change; the authority chain behind them does.

This is where Quidnug goes from "new namespace" to "genuine
DNS replacement candidate." It requires consortium governance
strong enough that users trust it more than ICANN, which is a
social problem, not a technical one. The technical substrate
makes the social move possible.

## Why this is better than DNS / DNSSEC / DoH / ENS

The short comparison:

| Property | Legacy DNS | DNSSEC | DoH / DoT | ENS | **Quidnug DNS** |
|---|---|---|---|---|---|
| Response integrity | ❌ (cache poisoning) | ✅ | ❌ (at edge only) | ✅ | ✅ |
| Response confidentiality | ❌ | ❌ | ✅ | ❌ | Optional |
| Cryptographic ownership | ❌ | ❌ | ❌ | ✅ | ✅ |
| Key recovery | ❌ | ❌ | ❌ | ❌ (lost ENS = lost forever) | ✅ (guardian quorum) |
| Censorship resistance | ❌ | ❌ | partial | ✅ (Ethereum-gated) | ✅ (federated) |
| Seizure resistance | ❌ | ❌ | ❌ | ✅ | ✅ |
| No central authority | ❌ | ❌ | ❌ | ❌ (Ethereum required) | ✅ |
| Operational complexity for domain owner | Low | **High** | Low | Medium | Low |
| Requires new TLS PKI | — | — | — | Yes (self-signed TLS) | No (published TLSA = DANE) |
| Gas / transaction cost | $0 | $0 | $0 | **$$$ per update** | ~$0 |
| Works offline for queries | ✅ (cached) | ✅ | ✅ | ❌ | ✅ (cached) |
| Propagation time | minutes-hours | same | same | blocks (minutes) | blocks (seconds with fast-block-interval) |
| Protocol complexity | Low | High | Low | High | Medium |

Key differentiators vs ENS (the closest competitor):

- **No blockchain gas fees.** Quidnug's consortium model
  means record updates cost the same as any other signed
  transaction (essentially zero).
- **Key recovery.** ENS names lost to key loss are
  unrecoverable. Quidnug names can use guardian recovery.
- **No Ethereum dependency.** Quidnug runs without any
  blockchain underneath.
- **Cheaper to resolve.** ENS requires an Ethereum RPC node
  (or a trusted gateway). Quidnug resolution is an HTTPS call
  to any node with the domain cached.

## Who's going to build this

Five early cohorts, roughly in adoption order:

1. **Crypto / Web3 projects.** Already familiar with self-
   sovereign identity. Most are already paying ENS gas
   fees; a cheaper, better-recovered alternative is obvious.
2. **Journalism and human-rights orgs.** For seizure
   resistance. A `.quidnug` domain for Tor-hosted sites, leak
   portals, and political publications is a real demand.
3. **Developer-tool namespaces.** `cli.example.quidnug` for
   CLI tools, `api.yourapp.quidnug` for APIs. Any project
   currently paying for a `.dev` domain is a candidate.
4. **Enterprise internal PKI.** A private Quidnug network as
   enterprise DNS + PKI (today's AD DNS + internal CA model
   replaced by a single signed graph).
5. **Consumer services once gateways + Phase 2 land.** The
   mass market isn't ready for "install a quidnug resolver
   plugin"; a DNS gateway hides the complexity.

## How this ties to the rest of the protocol

DNS replacement is a stress test for most of what we've built:

- **QDP-0012 (governance)** — domain ownership, key rotation,
  delegation.
- **QDP-0013 (federation)** — alternative roots, cross-network
  name resolution, cross-root reputation for registrars.
- **QDP-0014 (discovery + sharding)** — finding where a name's
  records live across a globally distributed cache-replica
  network.
- **QDP-0001 (nonce ledger)** — replay protection for record
  updates.
- **QDP-0002 (guardian recovery)** — the "I lost my key"
  story that DNSSEC never solved.
- **QDP-0003 (cross-domain nonce scoping)** — DNS records
  scoped per subdomain independently.
- **QDP-0010 (Merkle proofs)** — light-client resolvers with
  compact inclusion proofs.

Building this use case end-to-end validates the whole stack.
It's also a tangible pitch to a mainstream audience: "you
know DNS? Imagine if it just worked, was cryptographically
owned by you, and nobody could take it away."

## What this document contains

- [`architecture.md`](architecture.md) — the full data
  model, DNS-record-to-Quidnug-event mapping table, resolver
  algorithm, delegation mechanics.
- [`implementation.md`](implementation.md) — concrete CLI
  commands, Go resolver library, DNS gateway design.
- [`threat-model.md`](threat-model.md) — attack vectors,
  comparison with DNS/DNSSEC failure modes, limits of the
  design.

## Status

Design. Nothing from this use case has shipped yet. It builds
on QDP-0012, 0013, 0014 (all Draft) and the existing
transaction types + discovery APIs. Implementation would be a
second-party project — a DNS-compatible client library
(`@quidnug/dns-resolver`), a DNS gateway service, a CLI
tooling layer for record management.

The foundational layer (domain governance + signed event
streams + per-domain discovery) is what we're already
building for the reviews system. DNS replacement reuses
effectively 100% of that machinery.
