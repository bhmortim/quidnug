# Video 2 — distribution plan

## Publish sequence

### Day 0 (Tuesday is ideal — best HN day)
- [ ] Upload to YouTube (metadata below).
- [ ] Post 60s vertical cut on LinkedIn + Twitter + Bluesky.
- [ ] Add embedded video to repo root README (replacing or
      supplementing the hook video).
- [ ] Update `docs/integration-guide.md` with "Start here: the
      3-minute overview" as the top item.

### Day 0 — 9am PT
- [ ] Submit to Hacker News with title: `Quidnug – a
      decentralized protocol for relational, per-observer
      trust`. URL = YouTube video.
- [ ] First comment on HN: link to repo + point out the
      comparison docs at `docs/comparison/` so folks don't
      ask the "why not X" questions.

### Day 1
- [ ] Reply to every HN comment.
- [ ] Cross-post the YouTube link to r/programming (Reddit HN-
      adjacent).

### Day 2
- [ ] LinkedIn in-feed native video (not just link).
- [ ] Submit to lobste.rs (different audience; respect the rule
      about self-promotion once per week).

### Day 7
- [ ] Write a companion blog post on dev.to or Hashnode
      expanding the 3-minute video into long-form reading.
      Embed video. Link to Python quickstart in post.

### Day 14
- [ ] Pitch to a podcast — CoRecursive, Software Unscripted,
      FOSSnet — for a 30-min interview.
- [ ] Email reach-out to select devrel contacts at adjacent
      OSS projects (DID Community, Sigstore, OpenTelemetry)
      for cross-promotion.

## YouTube metadata

**Title:** `Quidnug in 3 Minutes — Relational Trust Explained`
**Description:**
```
Three minutes on Quidnug — a decentralized protocol for
relational trust. Covers what it is, why you'd use it, and
how to try it locally.

Most "trust" in software today is one-number-fits-all.
Quidnug is different: every person, organization, AI agent,
and device is a cryptographic identity (a "quid") that issues
signed trust edges to others. Queries are per-observer and
transitive.

Chapters:
0:00 — The problem
0:45 — What Quidnug is
1:45 — Why you'd use it
2:30 — How to try it

Concretely:
• 7 SDKs (Python, Go, JavaScript, Rust, Java, .NET, Swift)
• Helm chart + Docker Compose for deploy
• Integrations with Sigstore, HL7 FHIR, Chainlink, Kafka,
  W3C Verifiable Credentials
• Apache-2.0, zero gatekeeping

Repo: https://github.com/bhmortim/quidnug
60-second intro: [Video 1 link]
Python quickstart: [Video 3 link]

#DecentralizedIdentity #OpenSource #Cryptography #Trust
#P2P
```

**Tags:** quidnug, decentralized identity, relational trust,
per-observer, cryptography, p2p, open source, did, vc, web3,
blockchain alternative

## Hacker News

**Submission title:** `Quidnug – a decentralized protocol for
relational, per-observer trust`
**Submission URL:** YouTube video URL (not the repo — video is
more clickable for HN audience at intro-level content).

**First comment (post immediately after submission):**
```
Author here. Happy to answer questions.

Quick context: this is a working protocol (not a paper or
roadmap) with 7 language SDKs, integrations, and a
production-oriented Helm chart. Apache-2.0.

The video is a 3-minute conceptual overview; deeper technical
content:
- Protocol design (QDPs 0001-0010): [link to docs/qdps/]
- Cross-SDK interop harness: [link to tests/interop/]
- How it compares to DIDs + VCs / PGP WoT / OAuth / blockchain:
  [link to docs/comparison/]

Genuinely interested in blunt feedback on the protocol design
— especially from folks who've shipped similar systems.
```

## LinkedIn (personal + org)

**Personal:**
```
3 minutes on why "trust" in software has been one-size-fits-
all for too long.

We built an Apache-2.0 protocol for relational trust that's
personal, transitive, and cryptographic.

• Every person, org, AI agent, device = cryptographic identity
• Trust edges are signed
• Queries are per-observer and transitive
• Domain scoping means "vendor trust" doesn't leak into
  "election trust"

Full video (3 min on YouTube, 60s vertical here):

[VERTICAL VIDEO]

Detailed comparison to DIDs + VCs, PGP WoT, OAuth, and
blockchain reputation in the repo.

github.com/bhmortim/quidnug
```

## Twitter / X

```
"trust" in software today = one number per entity. globally.

in Quidnug: per-observer transitive trust. alice and bob can
have genuinely different trust in the same vendor.

3-minute explainer ↓

(YouTube link)

🧵 1/3
```

**Reply 2/3:**
```
2/3 — 7 SDKs (Python, Go, JavaScript, Rust, Java, .NET,
Swift) at full protocol parity. Byte-identical signatures
across all of them (caught a UTF-8 canonicalization bug the
hard way during interop testing).

Apache-2.0.
```

**Reply 3/3:**
```
3/3 — repo has runnable examples for AI agent identity,
election integrity, W3C Verifiable Credentials integration,
Helm-deployable production node, and a CI GitHub Action for
trust-gated workflows.

github.com/bhmortim/quidnug
```

## Bluesky

```
3 minutes on a decentralized protocol we built for relational,
per-observer trust.

per-observer means alice and bob can reasonably trust the same
entity differently. current systems pretend they can't.

apache-2.0, production-ready. 7 SDKs.

📺 [youtube link]
🔗 github.com/bhmortim/quidnug
```

## Reddit

**r/programming:**
Title: "Quidnug — a decentralized protocol for relational,
per-observer trust (Apache-2.0, 7 SDKs)"
Body: "3-minute explainer video: [YouTube]. Repo:
github.com/bhmortim/quidnug. Comparison with DIDs / blockchain /
PGP WoT in the `docs/comparison/` directory."

**r/cryptography** (after 24 hours, to stagger):
Title: "Canonical-bytes + signature interop across 7 SDKs
(plus a UTF-8 gotcha we caught)"
Body: Link to the cross-SDK interop harness at `tests/interop/`;
tease the Unicode escape bug the harness caught in Python's
default `json.dumps`. This one targets the algorithm audience.

## Companion blog post (day 7)

**Title:** "Relational Trust: Why One Number Per Entity
Isn't Enough"
**Where:** dev.to or Hashnode.
**Length:** ~1500 words.
**Structure:**
1. The problem — our UIs have been lying (5-star reviews,
   trust scores)
2. What "relational" means mathematically
3. The Quidnug data model in one paragraph
4. Embedded 3-minute video
5. Concrete example: vendor onboarding
6. Comparison with DIDs / blockchain in 2-3 sentences each
7. "Try it yourself" section with Python quickstart
8. Link to Video 3 and the AI-agent deep dive

## Podcast pitch template

```
Subject: Quidnug — relational trust protocol — podcast guest?

Hi [Host],

I've been a listener — [specific episode] was particularly
useful for my work on [thing].

I built a protocol called Quidnug for per-observer relational
trust. 3-minute explainer: [video]. Repo, Apache-2.0:
github.com/bhmortim/quidnug.

I'd love to come on for a 30-minute conversation about the
protocol design, the trade-offs vs DIDs / blockchain, and what
I've learned shipping 7 parallel SDKs. Happy to dive into
cryptographic canonical-bytes gotchas, relational-trust
algorithms, or guardian-based key recovery — whichever fits
your audience best.

Thanks,
[You]
```

## License

Apache-2.0.
