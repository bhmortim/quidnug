# Video tutorial plan

A production plan for the Quidnug video-tutorial library, covering:

1. **AI video service landscape** — which tool for which kind of
   video, with realistic 2026 pricing and capability notes.
2. **Prioritized video roadmap** — 18 videos ranked by adoption
   impact, with target length and audience.
3. **Production recipes** — specific tool combinations for each
   video style, including scripts + assets to feed the tools.

Ready-to-record scripts live in [`scripts/`](scripts/). Start
with the hook video (`script-01`) and the Python quickstart
(`script-03`) — those are the highest-leverage first two.

---

## 1. AI video service landscape (2026)

### Talking-head AI avatars (for explainers + conference talks)

| Service | Best for | ~ Price | Notes |
| --- | --- | --- | --- |
| **HeyGen** | Conference-style talks, bilingual | $30–90 / mo | 100+ avatars, voice cloning, multi-language lip sync. Output quality is industry-leading. |
| **Synthesia** | Corporate training | $30–90 / mo | 140+ avatars, 120+ languages. Great for SDK training content. |
| **D-ID** | Quick explainers | $5–108 / mo | Fastest to ship; slightly lower fidelity than HeyGen. |
| **Colossyan** | "AI actors" with scenes | $27+ / mo | Scene-based — lets you swap backgrounds, ideal for mixed presenter + diagram videos. |
| **Hour One** | Sales & onboarding | $30+ / mo | B2B-focused, good for "here's how Quidnug helps CTOs" angle. |

### Pure AI video generation (B-roll, concept shots, intros)

| Service | Best for | ~ Price | Notes |
| --- | --- | --- | --- |
| **Runway Gen-4** | Cinematic B-roll | $15–95 / mo | Text-to-video + video-to-video. Ideal for abstract "trust graph" visual intros. |
| **OpenAI Sora** | Best fidelity at longer duration | varies | 1080p, 20s+ clips. Great for anything more than 3 seconds. |
| **Google Veo 3** | Cinematic, with lip sync | in Vertex AI | Native audio + lip sync in same shot. |
| **Pika Labs** | Short social clips | $10–70 / mo | Fast iteration, ideal for Twitter/LinkedIn announcement clips. |
| **Luma Dream Machine** | Motion graphics | $10–95 / mo | Strong on motion; weak on detailed code/UI. |
| **Kling AI** | Realism on faces | $10+ / mo | Strongest at human subjects. |

### Screen recording + AI editing (for code walkthroughs)

| Service | Best for | ~ Price | Notes |
| --- | --- | --- | --- |
| **Descript** | Podcast-style editing via transcript | $16–40 / mo | **Highest leverage for dev tutorials.** Edit video by editing the transcript; voice cloning; auto-remove filler words; green-screen. |
| **Loom AI** | Async team demos | $12–24 / mo | Auto-generated chapters + summaries from screen recordings. Fastest way to ship "how our SDK works" videos. |
| **Riverside.fm** | Interview-quality recording | $15–29 / mo | High-quality 4K per-participant recording; AI transcription + editing. |
| **Tella** | Polished screen + camera | $15–29 / mo | Branded, easy-on-the-eyes layouts — good for YouTube. |

### Tutorial-specific platforms (turn clicks into step-by-step)

| Service | Best for | ~ Price | Notes |
| --- | --- | --- | --- |
| **Guidde** | SaaS walkthroughs with voiceover | $16–40 / mo | AI captures the click-stream + auto-narrates. Turns "install Helm chart" into polished video in 20 minutes. |
| **Scribe** | Step-by-step visual guides | free + $29 | Converts screen activity into illustrated docs *and* video. Great for onboarding runbooks. |
| **Arcade** | Interactive product demos | $32–80 / mo | Clickable demos that also export as video. Ideal for the Quidnug dashboard tour. |
| **Supademo** | Same category | $27–50 / mo | Alternative to Arcade; similar pricing. |

### AI voiceover (when you want a specific voice)

| Service | Best for | ~ Price | Notes |
| --- | --- | --- | --- |
| **ElevenLabs** | Highest-quality voice clone | $5–330 / mo | Clone from 30s of audio. **Best voice quality on the market.** |
| **Murf AI** | Library of pro voices (no cloning needed) | $19–79 / mo | 120+ pro voices, no own-voice clone required. |
| **Play.ht** | Multilingual voice clones | $39–99 / mo | Good multilingual. |
| **WellSaid Labs** | Enterprise-grade | $49–99 / mo | Locked-down voice catalog; used in corporate training. |

### Full-stack "make me a video from a script" services

| Service | Best for | ~ Price | Notes |
| --- | --- | --- | --- |
| **InVideo AI** | Quick turnarounds with stock footage | $25–60 / mo | Text-to-video with AI voiceover + stock B-roll + music. Good for marketing clips. |
| **Pictory** | Turn blog posts into videos | $19–99 / mo | Point it at a URL or doc, get a video. Ideal for turning our existing READMEs into videos. |
| **Lumen5** | Social video from articles | $29–149 / mo | Social-sized (YouTube Shorts, TikTok, Instagram). |

---

## 2. Prioritized video roadmap

Ranked by adoption impact ÷ production difficulty.

### Tier A — ship first (highest leverage)

| # | Title | Length | Audience | Script |
| --- | --- | --- | --- | --- |
| 1 | **"What is Quidnug?"** — 60-second hook | 0:60 | Cold traffic | [`script-01`](scripts/script-01-hook-60s.md) |
| 2 | **"Quidnug in 3 minutes"** — concept video | 3:00 | Developers evaluating | [`script-02`](scripts/script-02-overview-3min.md) |
| 3 | **Python SDK quickstart** | 5:00 | Python devs | [`script-03`](scripts/script-03-python-quickstart.md) |
| 4 | **AI agent identity & provenance** | 8:00 | AI platform teams | [`script-04`](scripts/script-04-ai-agents.md) |
| 5 | **Quidnug vs DIDs + Verifiable Credentials** | 6:00 | Identity architects | [`script-05`](scripts/script-05-vs-did-vc.md) |

### Tier B — second wave

| # | Title | Length | Audience |
| --- | --- | --- | --- |
| 6 | Go SDK quickstart | 5:00 | Go devs |
| 7 | JavaScript + React quickstart | 5:00 | Frontend devs |
| 8 | Rust SDK quickstart | 5:00 | Systems devs |
| 9 | The trust graph, visualized | 4:00 | Non-technical |
| 10 | Guardian-based key recovery (QDP-0002) | 7:00 | Security architects |
| 11 | Cross-domain gossip + federation | 6:00 | Distributed-systems devs |
| 12 | Integrating with Sigstore | 5:00 | Supply-chain-security teams |

### Tier C — deep dives

| # | Title | Length | Audience |
| --- | --- | --- | --- |
| 13 | W3C Verifiable Credentials on Quidnug | 8:00 | VC/DID community |
| 14 | Elections on Quidnug | 10:00 | Civic-tech orgs |
| 15 | HSM-backed signing in production | 7:00 | Enterprise security |
| 16 | OIDC bridge: Okta/Auth0/Azure → Quidnug | 6:00 | Enterprise IAM |
| 17 | Operations: Helm deploy + observability | 8:00 | Platform engineers |
| 18 | 30-minute conference talk (from the 77-slide deck) | 30:00 | Conference attendees |

---

## 3. Production recipes

Concrete tool stacks for each video style we'll produce.

### Recipe A — 60-second hook

**Goal:** Get someone to remember Quidnug and want more.
**Tools:** ElevenLabs (voice) + Runway Gen-4 (B-roll) + Descript (edit).
**Cost:** ~$30 one-time (plan allocations).
**Production time:** 4 hours.

Flow:
1. Write the script (already done — `script-01`).
2. Generate voiceover in ElevenLabs with your own cloned voice
   (30-second voice sample needed).
3. Generate 3–5 second B-roll clips in Runway Gen-4 from the
   visual-cue lines in the script ("a constellation of
   interconnected nodes," "a handshake between two
   cryptographic keys").
4. Drop everything into Descript, trim filler, add captions,
   export at 1080p vertical (for LinkedIn/YouTube Shorts) and
   horizontal (for YouTube/Twitter).

### Recipe B — 3-minute overview

**Goal:** Developer-focused. Convince them this is worth their
next hour.
**Tools:** HeyGen avatar + screen recording of the compose
quickstart + Descript.
**Cost:** ~$50.
**Production time:** 8 hours.

Flow:
1. Feed `script-02` into HeyGen with your avatar (or use an
   off-the-shelf avatar matched to your audience).
2. Record the code demo separately (OBS or Screen Studio).
3. Mix avatar + demo in Descript, using picture-in-picture for
   the avatar during code shots.
4. Export at 1080p; shorter 30s cuts for social.

### Recipe C — SDK quickstart (5 min)

**Goal:** Developer runs the code and it works.
**Tools:** Loom AI or Guidde + ElevenLabs voiceover (or your
own).
**Cost:** ~$20.
**Production time:** 3 hours per SDK.

Flow:
1. Open a fresh terminal + VS Code.
2. Start recording with Loom AI.
3. Walk through the quickstart from the SDK README verbatim
   (don't improvise — read the script).
4. Loom AI auto-generates chapters + summary.
5. Optional: replace your voice with ElevenLabs voice-over
   (useful if you want consistent voices across a series).

### Recipe D — Use-case deep dive (8–15 min)

**Goal:** Concrete example of a use case. Architecture +
code + audit walk-through.
**Tools:** HeyGen (presenter segments) + Descript (code demo
segments) + Figma/Excalidraw for architecture diagrams.
**Cost:** ~$70.
**Production time:** 12 hours per video.

Flow:
1. Script in 3 acts: *problem → solution → demo*.
2. Record architecture explanation via HeyGen avatar.
3. Record terminal/IDE walkthrough via Descript.
4. Splice with architecture diagrams (2–3 beats of "here's the
   data flowing through" animated in Figma → exported as MP4).

### Recipe E — Comparison video (5–10 min)

**Goal:** "Quidnug vs X" decision framework.
**Tools:** HeyGen + Google Slides / Pitch / Figma Slides.
**Cost:** ~$40.
**Production time:** 8 hours.

Flow:
1. Use the comparison docs in `docs/comparison/` as source
   material — the arguments are already written.
2. Build ~6 slides (hook, feature table, when-to-X, when-to-Y,
   when-both, conclusion).
3. HeyGen avatar narrates over slides.
4. Export with captions.

### Recipe F — Conference talk (30 min)

**Goal:** YouTube-findable "Introduction to Quidnug" keynote-
style video.
**Tools:** Record yourself on a real camera (if possible) OR
HeyGen for fully AI version + the 77-slide deck as-is.
**Cost:** ~$100.
**Production time:** 20 hours.

Flow:
1. Take the 77-slide deck (already in `docs/presentations/`).
   Speaker notes are your script — they're already written.
2. Option A: Record yourself, splice with the deck.
3. Option B: HeyGen avatar over the deck (cheaper, faster, less
   personal).
4. Upload to YouTube with timestamps matching each section of
   the deck.

---

## 4. What NOT to do

Some common AI-video failure modes to avoid:

- **Don't use an AI avatar for the 60-second hook** unless
  you've tested it rigorously. First-impression videos live
  and die on authenticity; the uncanny valley is real. Prefer
  a real human (even just a voice-over) for the hero video.
- **Don't use pure text-to-video** for code tutorials. Runway /
  Sora / Pika can't render code correctly. Use actual screen
  recordings.
- **Don't skip captions**. 85% of social video is watched
  without sound. Descript auto-generates them; use that.
- **Don't use stock AI voices** for your voice-over if you're
  doing >3 videos. Clone your actual voice in ElevenLabs — the
  consistency builds recognition.
- **Don't try to do all 18 in one month**. Ship Tier A first,
  get feedback, then Tier B. The cadence matters more than
  volume.

---

## 5. Distribution

Every video should be cut into at least two formats:

- **Long-form**: YouTube, your docs site, embed in GitHub README.
- **Social short**: 30–60 second vertical cut for LinkedIn,
  Twitter, Bluesky, YouTube Shorts. Descript does this with one
  click.

Recommended cadence:
- Month 1: ship Tier A (5 videos).
- Month 2: ship Tier B (7 videos).
- Month 3: ship Tier C (6 videos).

That's 18 videos in 90 days — realistic with the AI stack above
at ~$100–150 / month of tool budget.

---

## 6. What's in this directory

- [`scripts/script-01-hook-60s.md`](scripts/script-01-hook-60s.md) — ready to record
- [`scripts/script-02-overview-3min.md`](scripts/script-02-overview-3min.md)
- [`scripts/script-03-python-quickstart.md`](scripts/script-03-python-quickstart.md)
- [`scripts/script-04-ai-agents.md`](scripts/script-04-ai-agents.md)
- [`scripts/script-05-vs-did-vc.md`](scripts/script-05-vs-did-vc.md)

Templates for the remaining 13 videos are in [`templates.md`](templates.md) —
take the corresponding README / existing doc and paste the key
paragraphs into the template structure.

## License

Apache-2.0.
