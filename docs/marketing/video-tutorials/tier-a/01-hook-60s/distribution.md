# Video 1 — distribution plan

## Publish sequence

### Day 0
- [ ] Upload to YouTube with SEO metadata (below).
- [ ] Post horizontal link to Twitter / X with caption (below).
- [ ] Post vertical clip to LinkedIn personal + Quidnug org page.
- [ ] Schedule Bluesky post for Day 1 at 10am PT (avoid clash with Twitter).
- [ ] Add a "Videos" section to the repo root README with embed.
- [ ] Add a "Watch the 60-second overview" CTA to `docs/integration-guide.md`.

### Day 1
- [ ] Post Bluesky + Mastodon.
- [ ] Reply to every comment on Day-0 posts.

### Day 5
- [ ] Repost vertical cut on LinkedIn with a DIFFERENT hook (test which converts better — LinkedIn does not penalize this if the post copy differs).

### Day 14
- [ ] If retention + CTR are positive, write a blog post expanding the "per-observer trust" concept, embedding the video. Target dev.to or Hashnode.
- [ ] Cross-post the vertical cut on r/cryptography (not the main video — that's for Video 2).

## YouTube metadata

**Title:** `Why your "trust score" is one-size-fits-all (and shouldn't be)`
**Description:**
```
60-second intro to Quidnug — a decentralized protocol for
relational, per-observer trust.

Today, "trust" in software is a single number per entity:
your vendor's rating, your credit score, a verified badge. But
Alice and Bob can reasonably trust the same vendor
differently — and we've been pretending they can't.

Quidnug is a P2P protocol where every person, organization,
AI agent, and device has its own cryptographic identity (a
"quid") and issues signed trust edges to others. Queries are
per-observer and transitive.

7 SDKs (Python, Go, JavaScript, Rust, Java, .NET, Swift).
Apache-2.0. Production-ready.

Repo: https://github.com/bhmortim/quidnug
Full 3-minute version coming soon.

Chapters:
0:00 — The problem
0:10 — Trust is personal
0:32 — What Quidnug is
0:46 — What Quidnug can do
0:55 — Try it

#cryptography #decentralized #identity #opensource
```

**Tags:** decentralized, identity, cryptography, trust, open
source, protocol, peer to peer, quidnug, p2p, reputation

**Category:** Science & Technology

**Visibility:** Public. Unlisted while thumbnail A/B testing
is running (first 4 hours), then Public.

## Twitter / X post

```
most software "trust systems" give you ONE score. globally.

but Alice and Bob can reasonably have different trust in the
same vendor. we've been pretending they can't.

60-sec explainer of what changes when you build trust as a
graph: [YouTube link]

🧵 1/2
```

**Reply tweet:**
```
2/2 — full 3-min version on the channel. 7 SDKs (Python, Go,
JS, Rust, Java, .NET, Swift) all at full protocol parity.
Apache-2.0.

github.com/bhmortim/quidnug
```

## LinkedIn (personal)

```
Most trust systems today give you ONE score per entity.
Globally. But Alice and Bob can reasonably have different
trust in the same vendor. Credit bureaus pretend they can't.

Built an open-source protocol (Apache-2.0) for relational
trust that's personal, cryptographic, and transitive — here's
the 60-second version.

[VERTICAL VIDEO EMBED]

More: github.com/bhmortim/quidnug

7 SDKs (Python, Go, JS, Rust, Java, .NET, Swift).
#DecentralizedIdentity #OpenSource #Cryptography #Quidnug
```

## LinkedIn (Quidnug org page)

```
Quidnug — relational trust, per-observer, cryptographic.
Apache-2.0, 7 SDKs, production-ready.

60-second explainer ↓

[VERTICAL VIDEO]

github.com/bhmortim/quidnug
```

## Bluesky

```
what if "trust" in software wasn't one-number-fits-all?

we built it. 60 sec: [LINK]

per-observer, transitive, cryptographic. Apache-2.0.
github.com/bhmortim/quidnug
```

## Mastodon (@social.coop or fosstodon.org)

```
60-second intro to Quidnug — a decentralized protocol for
relational, per-observer trust.

Every person, org, AI agent, device = cryptographic identity.
Trust edges are signed. Trust queries are per-observer and
transitive.

Apache-2.0. 7 SDKs.

Video: [YouTube link]
Repo: https://github.com/bhmortim/quidnug

#Decentralized #OpenSource #Identity
```

## Communities (day 14+)

For the hook video specifically, do NOT post to HN / Reddit
beyond r/cryptography — save those for Video 2 (the 3-min
overview). A 60-second video is a poor HN fit; a 3-min
technical overview is a strong HN fit.

## Cross-linking

- Repo README: embed at top of the README after the TL;DR, before
  the demo code block.
- `docs/integration-guide.md`: embed at top with caption "60-second
  intro."
- GitHub Social Preview: generate a still from 0:40 and set as
  the repo's social preview image (boosts GitHub link previews
  on Twitter / LinkedIn).

## License

Apache-2.0.
