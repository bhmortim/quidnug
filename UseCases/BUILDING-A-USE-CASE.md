# Building a use case on Quidnug

> A concrete recipe for taking an idea from "I want to build X
> on Quidnug" to "I have a shippable design document." Read
> this after [`ARCHITECTURE.md`](ARCHITECTURE.md).
>
> The goal of this doc: a six-hour exercise that produces a
> folder under `UseCases/` structured like the existing ones.

## The recipe

Six phases. Each answers a specific question. Work through
them in order. Output is a set of documents that become your
use case's README + architecture + implementation + threat-
model files.

### Phase 1 — Name the problem (30 minutes)

Before any design: what's the status quo, and why does it
hurt?

Write a page that answers:

1. Who has this problem? (Bankers? Reviewers? Voters?
   Developers? Patients? Be specific.)
2. What do they do today? (Spreadsheets? Email? A
   centralized SaaS? A patchwork of registrars?)
3. What goes wrong? (Key compromise? Seizure? Cache
   poisoning? Fraud? Political capture?)
4. What would "solved" look like if the constraints
   dissolved?

Output: a paragraph or two per question. This becomes the
"Problem" section of your README.md.

**Example — credentials use case:**
> Employers verify academic credentials by phoning
> registrars. Processing delay: 5 days. Verification is
> trivial to forge (PDF diplomas). Revocation happens
> silently and slowly.

### Phase 2 — Match to an archetype (15 minutes)

From [`ARCHITECTURE.md §5`](ARCHITECTURE.md#5-use-case-archetypes--how-to-choose):

| Output shape | Archetype | Reference use case to crib from |
|---|---|---|
| Per-observer rating | reputation | `reviews-and-comments/` |
| Binary yes/no + signed path | attestation | `credential-verification-network/` |
| Multi-party coordinated action | coordination | `interbank-wire-authorization/` or `elections/` |
| Name or record lookup | infrastructure | `dns-replacement/` |

Pick the closest. Your use case may blend two, but one is
usually primary. Cribbing saves days of reinventing.

Output: one-liner naming your archetype + which existing use
case folder you're cribbing from. Note deviations.

### Phase 3 — Design the domain hierarchy (45 minutes)

Names your use case will live under. Hierarchical by
convention.

1. What's the root? (Usually `.<use-case>.public` for public
   use cases, or `<org>.<use-case>` for private.)
2. What are the sub-trees? (Per-category, per-jurisdiction,
   per-time-period, etc.)
3. Which domains have their own consortium vs. inherit from
   a parent?

Output: a tree of domain names with notes on which are
governed independently.

**Example — credentials:**
```
credentials.education                  (root; governed by a
                                       cross-jurisdictional
                                       consortium)
├── credentials.education.us           (delegated to a US
│                                       accreditor consortium)
│   ├── credentials.education.us.sacs
│   ├── credentials.education.us.msche
│   └── ...
├── credentials.education.eu
│   └── ...
└── credentials.education.medical
    └── credentials.education.medical.us.state.nj
```

### Phase 4 — Design events + trust edges (90 minutes)

Two questions that together define your protocol surface.

#### 4.1 What events does the use case emit?

For each event type:

- Name (uppercase with underscores, e.g. `REVIEW`,
  `CREDENTIAL_ISSUED`, `VOTER_REGISTERED`).
- Who signs it (which role in the actor list)?
- What's the payload shape? (JSON schema.)
- What's the validation rule? (What must be true for the
  event to be accepted on-chain?)
- What does a downstream consumer do with it?

#### 4.2 What do TRUST edges mean?

TRUST edges always mean "truster's confidence in trustee for
a specific domain at a level 0-1." In your use case,
specifically:

- Who publishes them? (Individual users? Authoritative
  issuers? Both?)
- What level conventions does your use case use? (0.5 =
  baseline? 0.9 = fully trusted? Decay per hop?)
- What do they weight in the output? (Which events do
  observers aggregate, and with which weights?)

**Example — credentials:**

Events:
- `CREDENTIAL_ISSUED` — signed by an issuer (university, cert
  org). Payload: subject-quid, credential-type, details, date,
  revocation-status.
- `CREDENTIAL_REVOKED` — signed by the same issuer. Payload:
  reference to the original `CREDENTIAL_ISSUED`, reason.
- `ACCREDITATION_GRANTED` — signed by an accreditor. Payload:
  institution-quid, scope of accreditation, valid-through.

Trust edges:
- Accreditor → Institution at 1.0 in
  `credentials.education.us.<accreditor>` = "I accredit this
  institution."
- Verifier → Accreditor at 0.95 in the same = "I trust this
  accreditor's judgment."

Aggregation: a verifier checks (a) the CREDENTIAL_ISSUED
signer (b) the signer's ACCREDITATION chain (c) no
subsequent CREDENTIAL_REVOKED exists. Binary yes/no with
an explicit signature path.

Output: a table of event types + a table of trust-edge
conventions. These become the core of your architecture.md.

### Phase 5 — Design the decision / rating algorithm (60 minutes)

Given the events + trust edges, how does a consumer (reader,
verifier, client) produce the output?

Four questions:

1. **Inputs:** which events does the algorithm consider?
   (All events on a subject's stream? Events with a specific
   type? Events within a time window?)
2. **Trust walk:** which TRUST edges does it follow?
   (Direct only? Multi-hop with decay? Domain-scoped?)
3. **Aggregation:** how does it combine multiple events
   into one output? (Max, sum, weighted mean, specific
   rules like "most recent wins"?)
4. **Edge cases:** what happens with zero trust paths? One
   untrusted source shouting? Conflicting attestations?

Write pseudocode. Even better: write reference
implementations in Python + Go.

**Example — reviews rating:**

```
effective_rating(observer, product, topic):
    reviews = fetch all REVIEW events for product (filter: domain matches topic)
    for each review:
        t = topical_trust(observer, reviewer, topic)
        h = helpfulness_score(observer, reviewer, topic)
        a = activity_score(reviewer)
        r, _ = recency_score(event.timestamp)
        w = t * h * a * r
        if w < min_weight:
            skip
        contributions.append((normalized_rating * w, w))
    return weighted_mean(contributions) if contributions else None
```

The [`examples/reviews-and-comments/algorithm.md`](../examples/reviews-and-comments/algorithm.md)
is the longest worked-out algorithm in the repo. Study it
even if your algorithm is simpler.

Output: pseudocode + reference implementation. This is the
heart of your use case.

### Phase 6 — Plan the deployment (60 minutes)

How does this actually ship?

1. **Launch set:** who registers the root domain? Who runs
   the initial consortium? Who are the initial governors?
2. **Bootstrap trust:** how do the first users enter the
   trust graph? (See [`ARCHITECTURE.md §7.2`](ARCHITECTURE.md#72-bootstrap-trust)
   for the four mechanisms.)
3. **Clients:** what libraries + UIs do users / apps need?
4. **Monitoring:** what Prometheus metrics? What
   alert thresholds? What does the status page show?
5. **Moderation:** what's the takedown policy? Who handles
   abuse? What events are used for de-weighting?
6. **Economic model:** does anything cost money? Who pays?
   (Registration fees? Rate-limit bypasses? Cross-network
   reputation imports?)

Output: a deployment section of your README + a launch
checklist (adapt
[`deploy/public-network/reviews-launch-checklist.md`](../deploy/public-network/reviews-launch-checklist.md)).

### Phase 7 — Threat-model (45 minutes)

For every actor class, list what a malicious version of them
could do, and how the design defends.

Use the four-category framework from
[`UseCases/dns-replacement/threat-model.md`](dns-replacement/threat-model.md):

1. **Impersonator** attacks — forge identities, redirect
   queries, spoof responses.
2. **Censor** attacks — block access, seize content,
   degrade service.
3. **Thief** attacks — steal keys, coerce transfers, lock
   out legitimate owners.
4. **Disruptor** attacks — DDoS, resource exhaustion, gossip
   storms.

For each, note:

- What the attack does.
- What DNS / the status-quo system does about it.
- What Quidnug does about it.
- What residual risk remains.

Honest threat models always name residual risk. Design that
claims perfect protection is design you shouldn't trust.

Output: a threat-model.md in the format of the existing ones.

## The six-hour day

In aggregate, that's about 5 hours of active work with 1
hour for writing / cleanup. One focused day, you come out
with:

- `<your-use-case>/README.md` (1500-2500 lines)
- `<your-use-case>/architecture.md` (2000-3500 lines)
- `<your-use-case>/implementation.md` (1500-3000 lines)
- `<your-use-case>/threat-model.md` (1000-2500 lines)

Total: 6000-11500 lines. This sounds like a lot but it's
mostly well-structured prose; the per-phase outputs flow
naturally into the right sections.

## Common mistakes

Three failure modes we've seen or avoided in the existing
use cases:

### "I'll skip the threat model"

Every use case in `UseCases/` has one. The protocol gives
you strong cryptographic primitives, but applications built
on top can still have holes — social-engineering paths,
race conditions at ingestion, governance concentration,
misconfigured federation. Writing the threat model forces
you to think about them.

If you skip this phase, you end up with a "secure protocol
with an insecure application." The threat model is where
you distinguish "problem the protocol solves" from
"problem you now need to solve in your application layer."

### "I'll invent new transaction types"

Almost always a mistake in the first pass. The protocol's
existing tx types (TRUST, IDENTITY, TITLE, EVENT, ANCHOR,
GUARDIAN_*) are sufficient for every use case in this
directory. What varies is the event payload schemas.

If you find yourself wanting to add a new tx type, ask
first:

- Could this be an EVENT with a new `eventType` value?
- Could this be a TRUST edge in a domain-specific meaning?
- Could this be a TITLE with specific ownership semantics?

9 out of 10 times, yes. If you genuinely need a new tx
type, that's a protocol change — propose it as a QDP,
not in a use-case folder.

### "I'll design for the case where I run everything"

Seductive but short-sighted. Even if YOU run everything at
launch, the governance / federation / discovery pillars
assume multiple parties eventually.

Always design for the M-of-N case even if your initial M =
N = 1. Add governors you'd invite in later. Pick domain
names that could outlive your company. Document how a
successor operator would take over.

The existing use cases all assume this multi-party future.
Yours should too.

## When your use case is ready

Checklist before submitting:

- [ ] README.md explains the problem clearly to a
      non-protocol-expert reader.
- [ ] README.md has a table mapping problems to Quidnug
      primitives (every existing use case has one).
- [ ] architecture.md has a data model with JSON schemas
      for every event payload.
- [ ] architecture.md has at least one sequence diagram
      (ASCII OK).
- [ ] implementation.md has runnable code — CLI commands,
      API calls, or library usage — not just prose.
- [ ] threat-model.md enumerates attacks by category and
      honestly names residual risks.
- [ ] All four files cross-reference each other and the
      relevant QDPs.
- [ ] UseCases/README.md updated with an index entry.
- [ ] Top-level README.md's use-case list updated with a
      link.
- [ ] Your use case builds on the existing protocol
      primitives — no proposed protocol changes bundled
      in. If it needs a new primitive, that's a QDP in
      `docs/design/`, separate from the use-case folder.

Submit via PR. Existing maintainers can tell you in 30
minutes whether the design holds together.

## Questions this doc doesn't answer

If you're stuck on any of these, open a GitHub Discussion
rather than inventing an answer:

- How do I handle a governance decision that the quorum
  disagrees about?
- What's the right fee model for public-network use of my
  use case?
- Should my use case require consortium membership for end
  users, or is cache-replica + no-node mode enough?
- What happens if the TLD operator my use case lives under
  goes rogue?

These have answers — they just depend on context that a
template doc can't capture. Ask.

## References

- [`ARCHITECTURE.md`](ARCHITECTURE.md) — the architecture
  tour; read before this.
- [`README.md`](README.md) — the use-case index.
- [`docs/design/`](../docs/design/) — QDPs, especially
  0012 / 0013 / 0014 for the architectural pillars.
- [`deploy/public-network/`](../deploy/public-network/) —
  operator playbooks you'll reuse for deployment.
- [`examples/reviews-and-comments/`](../examples/reviews-and-comments/)
  — the most worked-out reference use case.
