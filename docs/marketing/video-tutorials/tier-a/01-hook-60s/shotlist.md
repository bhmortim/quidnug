# Video 1 — "What is Quidnug?" — shot list

**Final length:** 0:60
**Budget:** $150 (thumbnail $40, motion-design polish $80, music $15 from subscription, buffer $15)
**Production time:** 10 hours over 6 days
**Aspect ratios delivered:** 1920×1080 (horizontal master), 1080×1920 (vertical), 1080×1080 (square)

## Opening premise

This is the one video in the series where a **real human voice**
(yours, un-cloned) matters most. Every other video can use
HeyGen + ElevenLabs clones. Record your own audio in a treated
room for this one.

## Second-by-second shot list

| Time | On-screen | Voice-over | Source / asset |
| --- | --- | --- | --- |
| **0:00–0:03** | Rapid-cut montage: a bank's "Approved" screen, a Yelp 5-star review, a "Verified" blue check, a 5-star Amazon rating | *(music in: low tech documentary bed, Epidemic Sound search "cinematic minimal")* "Your bank has 'approved' you..." | Storyblocks UI-screenshot stock (search: "banking approved UI," "verified badge," "5 star review") |
| **0:03–0:05** | "Your vendor is 'verified.' Your contractor has 5 stars." | | |
| **0:05–0:07** | Freeze-frame on the "Approved" card; a question mark icon materializes above it | "Do any of those actually mean anything to you?" | Motion kit asset #3 ("chaos vs signed graph") — frame 1 only |
| **0:07–0:10** | A single monolithic "TRUST SCORE: 87" card fills the screen | "Trust today is one-size-fits-all..." | Motion designer builds this — tight animation, 3 seconds |
| **0:10–0:13** | The "87" card shatters into hundreds of smaller nodes flying outward | "A single reputation score. A single approved flag." | Continuation of same motion-design clip |
| **0:13–0:18** | Nodes settle into a sparse graph with directed edges; one edge glows as "you → vendor at 0.85" | "But your bank trusts a vendor differently than you do..." | Motion kit asset #1, frame 1 (sparse trust graph) |
| **0:18–0:22** | Graph densifies; multiple POVs indicated by highlighted nodes | "Your company trusts a contractor differently than your neighbor does." | Motion kit asset #1, frame 2 (dense graph) |
| **0:22–0:27** | Camera "moves" from one POV to another; edge weights shift visibly | "Real trust is personal — and we've been pretending it isn't." | Motion kit asset #1, frame 3 (POV shift) |
| **0:27–0:32** | "QUIDNUG" wordmark fades in over the settled graph, slight scale-up | "Quidnug is a decentralized protocol for relational trust..." | Brand card |
| **0:32–0:40** | Split-screen: on the left, a person icon; on the right, a cryptographic keypair icon; they merge. Label: "each quid = cryptographic identity" | "...trust that's personal, transitive, and cryptographic." "Every person, organization, AI agent, and device is a quid — an identity you control." | Motion designer builds this beat |
| **0:40–0:46** | Trust edges animate drawing between quids, with numeric weights (0.9, 0.8, 0.7) appearing beside them | "You issue signed trust edges to people you actually trust. Quidnug computes transitive trust across the graph — Alice trusts Carol at 0.9, Carol trusts Bob at 0.8, so Alice's relational trust in Bob is 0.72." | Motion kit asset #1 + overlay text |
| **0:46–0:50** | Cutaway: three observer silhouettes, each seeing a different numeric score for the same target | "Every observer gets their own answer." | New motion designed beat |
| **0:50–0:55** | Language logos appear in a 7-across row: Python, Go, JS, Rust, Java, .NET, Swift. Below: "Apache-2.0. Production-ready." Quick terminal cut of the CLI echoing `quid generate` | "It works today. Apache-2.0. SDKs for every language. Drop-in for AI agents, elections, verifiable credentials, supply-chain attestation." | Language logos from Simple Icons; terminal recording by you |
| **0:55–0:60** | Final hold on URL `github.com/bhmortim/quidnug` in large navy-over-amber type, logo animates in | "Per-observer trust. No central authority. See how it works: github.com/bhmortim/quidnug." | Brand card outro |

## Shot-list total check

- **Script length**: approximately 120 spoken words → 55 seconds
  at 130 wpm. Leaves 5 seconds of breathing room (good).
- **Visual complexity**: 11 distinct beats at average 5.5 seconds
  each. **This is tight.** Expect to spend most editing time on
  pacing between beats, not on asset creation.

## Audio notes

- **Voice**: record yourself at 24-bit 48kHz in a treated room
  (or dry closet with blankets). USB mic minimum: Shure MV7, Rode
  NT-USB+, or similar. Read the full script 4–5 times; pick the
  takes with cleanest breath control.
- **Music bed**: Epidemic Sound — filter "Cinematic," "Tech
  Documentary," "Minimal." Duration 60s. Start narrow, build at
  0:27 with the wordmark reveal, soften under final URL.
- **Sound design**:
  - subtle "whoosh" transition between the rapid cuts at 0:00–0:05
  - soft "ding" at 0:07 when the question mark lands
  - "shatter" synth at 0:10 when the "87" card breaks
  - ambient presence through the graph animation
  - final "crisp" UI-click at 0:55 when the URL locks

## Export spec

| Deliverable | Dimensions | Duration | Usage |
| --- | --- | --- | --- |
| Horizontal master | 1920×1080 60fps | 60s | YouTube, embed in README |
| Vertical | 1080×1920 60fps | 60s | LinkedIn, Shorts, TikTok, Bluesky |
| Square | 1080×1080 60fps | 60s | Instagram feed (optional) |

All MP4 / H.264 / AAC 192kbps / 8 Mbps video target.

## Production steps (in order)

1. **Day 1 — assets**: Finalize motion-design brief; confirm
   music bed selected; record voice-over (multiple takes).
2. **Day 2 — Runway B-roll**: Generate the "cryptographic key
   materializes" shot at 0:32–0:35 (if motion designer can't
   build in time, fall back to Runway). Prompt:
   `"cryptographic key glyph materializing from particles,
   cinematic slow-motion, deep navy background, amber accent
   light, abstract, 3 seconds"`
3. **Day 3 — rough cut in Descript**: Paste voice-over,
   timeline out to 60 seconds, drop in each beat as placeholder.
4. **Day 4 — final assembly**: Replace placeholders with
   delivered motion-design assets. Add music. Add SFX.
5. **Day 5 — color & audio mastering**: Hand off to freelance
   colorist (optional but recommended for the hero video).
   Return audio normalized to -16 LUFS.
6. **Day 6 — thumbnail + publish**: Receive thumbnail from
   designer (brief below). Upload to YouTube with all three
   aspect ratios. Schedule LinkedIn + Twitter posts.

## Thumbnail brief (send to designer)

> Design a YouTube thumbnail for the video "What is Quidnug?"
> — 1280×720. The video is about a decentralized trust protocol.
>
> Visual concept: a stylized trust graph (nodes + edges) in
> navy blue with amber-orange accent lines. On the right,
> the large text "WHY YOUR TRUST SCORE IS WRONG" in bold
> white sans-serif, high contrast. On the left, a silhouette
> of a person looking at the graph. Small Quidnug wordmark
> in the bottom-right.
>
> Brand colors: navy `#0B1D3A`, amber `#F9A825`, white
> `#FFFFFF`.
>
> Test against 3 variants via TubeBuddy: one with the
> graph dominant, one with the question text dominant,
> one with a face close-up.

## Distribution captions (ready to paste)

### LinkedIn
> Most trust systems give you ONE score per entity. Globally.
>
> But Alice and Bob can reasonably have different trust in the
> same vendor. Credit bureaus pretend they can't.
>
> Built an open-source protocol (Apache-2.0) for relational
> trust that's personal, cryptographic, and transitive — here's
> the 60-second version. 🧵 1/2
>
> [Vertical video]
>
> More: github.com/bhmortim/quidnug

### Twitter / X
> per-observer relational trust is a different data structure
> than "ranking."
>
> 60 seconds on why:
>
> (link to YouTube video)

### Bluesky
> What if trust wasn't one-number-fits-all? Built it.
>
> 60 sec: [link]

## License

Apache-2.0.
