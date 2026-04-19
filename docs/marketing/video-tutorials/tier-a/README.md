# Tier A production plan — the first 5 videos

A detailed, doubled-budget plan for the 5 launch videos. Covers
tool stack, team, timeline, per-video budget allocation, and a
production bible for each video.

**Total budget:** $800–$900 across the 5 videos + tooling
subscriptions for one quarter.
**Total production time:** ~80 hours spread over 6 weeks.
**Audience:** developers, architects, and technical leaders
evaluating Quidnug.

---

## 1. Tool stack (3-month subscriptions)

| Tool | Tier | Monthly | 3-month | Why this tier |
| --- | --- | --- | --- | --- |
| **HeyGen** | Creator ($39) | $39 | $117 | Custom avatar trained on your face (not stock). 4K export. Background removal. |
| **ElevenLabs** | Creator ($22) | $22 | $66 | Voice clone with Professional Voice Clone quality (vs. Instant clone on free). Commercial license. |
| **Descript** | Creator ($24) | $24 | $72 | 30h/mo transcription + overdub. Pro voice + better filler-word removal. |
| **Runway ML** | Standard ($15) | $15 | $45 | Gen-4 text-to-video for B-roll. 625 credits/mo. |
| **Epidemic Sound** | Personal ($15) | $15 | $45 | 40 000 tracks + commercial license. Avoids copyright strikes on YouTube. |
| **Figma** | Pro ($15) | $15 | $45 | Motion boards + Dev Mode for precise architecture diagrams. |
| **Storyblocks** | Video All Access ($20) | $20 | $60 | Unlimited stock B-roll + motion graphics templates. |
| **Canva** | Pro ($15) | $15 | $45 | Thumbnail + social-cut templates. |
| **TubeBuddy** | Pro ($8) | $8 | $24 | Thumbnail A/B testing + SEO. |

**Tooling subtotal:** $173/month × 3 = **$519**.

## 2. Freelance line items

Hire once, use across all 5 videos or specific ones.

| Role | Where | Cost | Applied to |
| --- | --- | --- | --- |
| Motion-graphics designer (architecture animations, trust-graph viz) | Upwork / Fiverr Pro | $250 flat for a "Quidnug visual kit" = 6 reusable animated diagrams | Videos 2, 4 |
| Custom thumbnail design (5 thumbnails) | 99designs / Dribbble freelancer | $40 × 5 = $200 | All 5 |
| Logo + brand intro/outro card (5 sec each) | Fiverr | $80 one-time | All 5 |
| Color grading / audio mastering pass | Fiverr | $30 × 3 hero videos = $90 | Videos 1, 2, 4 |

**Freelance subtotal:** **$620**.

## 3. Per-video budget allocation

| # | Video | Length | Budget | Run-time |
| --- | --- | --- | --- | --- |
| 1 | **60-second hook** | 0:60 | $150 | 10h |
| 2 | **3-minute overview** | 3:00 | $220 | 16h |
| 3 | **Python SDK quickstart** | 5:00 | $80 | 8h |
| 4 | **AI agent identity** | 8:00 | $260 | 24h |
| 5 | **Quidnug vs DIDs + VCs** | 6:00 | $120 | 14h |
| **Totals** | | **23:00** | **$830** | **72h** |

(The monthly tooling subscriptions above overlap with and
partially fund the per-video budgets — the "$830" is the
incremental project budget assuming tool subs are already paid.)

## 4. Timeline (6 weeks)

### Week 1 — Brand kit + voice clone
- Hire intro/outro freelancer (1 day).
- Clone your voice in ElevenLabs (30 min — record 1 min
  high-quality reference in your final studio setup).
- Train custom HeyGen avatar (30 min — record 2 min of you
  looking at the camera in Good Outfit™, no jewelry glints,
  even lighting).
- Hire motion-graphics designer for the "Quidnug visual kit"
  — 6 reusable animations:
  1. Trust graph filling out (3 variants: sparse, dense, POV shift)
  2. Multi-layer agent stack (foundation → lab → agent → action)
  3. Before/after: chaos vs signed graph
  4. Sign → verify data-flow animation
  5. Cross-SDK interop "same signature everywhere"
  6. Merkle tree expand/collapse for QDP-0010 content

Output of Week 1: brand kit ready, voice clone ready, avatar
ready, visual kit delivered (these unlock video production).

### Week 2 — Video 1 (hook) + Video 3 (Python quickstart)
- Day 1: Finalize scripts with any last tweaks.
- Day 2: Record Video 3 screen content (2 takes, ~45 min).
- Day 3: Generate Video 1 B-roll in Runway (iterate on prompts;
  plan for 3 rounds of regeneration).
- Day 4: Edit both in Descript.
- Day 5: Freelance thumbnail designs for both (send briefs).
- Day 6: Final export, color grade Video 1, publish both.

### Week 3 — Video 2 (overview)
- Day 1: Finalize script with architecture-diagram callouts.
- Day 2: Pair with motion designer on any diagram tweaks.
- Day 3: Record HeyGen avatar narration (use final script).
- Day 4: Assemble in Descript with PIP + diagrams.
- Day 5: Color grade + audio master.
- Day 6: Publish.

### Week 4 — Video 4 (AI agents)
- Day 1–2: Re-verify the runnable Python example works.
- Day 3: Record code walkthrough + HeyGen avatar segments.
- Day 4: Integrate motion-kit animations.
- Day 5–6: Edit, color grade, subtitle.
- Day 7: Publish.

### Week 5 — Video 5 (comparison)
- Day 1: Finalize slide deck (8 slides).
- Day 2: Record HeyGen avatar narration over slides.
- Day 3: Edit + transitions.
- Day 4: Publish.

### Week 6 — Distribution + iteration
- Republish top-performing videos as vertical cuts.
- Review YouTube analytics: retention curves, CTR, audience
  drop-off.
- Repair thumbnails on anything under 3% CTR (A/B test with
  TubeBuddy).
- Post the series to HackerNews ONLY once Video 2 is out.
- Start the Tier B script writing.

## 5. Team model

Three viable models depending on your situation:

### Model A — Solo founder with doubled budget (you + AI + freelance)

Hours breakdown:
- 72h production time across 6 weeks = 12h/week.
- Freelance: motion designer, thumbnails, brand card.
- Tools do the rest.

Total: **$830 + your 72 hours**.

### Model B — Founder + editor (hire an editor for the whole series)

- You record + write; an editor finalizes everything in
  Descript / After Effects.
- Editor cost: ~$500–1200 for the series (Upwork mid-tier).
- Your time drops to ~25 hours across 6 weeks.

Total: **$1400–2000 + your 25 hours**. Add this if your time
is the bottleneck.

### Model C — Fractional DevRel agency

Several small agencies (e.g. Draft.dev, LogRocket, DevTell)
produce technical video content as a service at $2500–5000
per video. Overkill for 5 videos but worth knowing if you scale
to 20+ videos.

---

## 6. Per-video production bibles

Deep plans for each. Go to the numbered directories:

- [`01-hook-60s/`](01-hook-60s/) — the 60-second opener
- [`02-overview-3min/`](02-overview-3min/) — 3-minute concept
  video
- [`03-python-quickstart/`](03-python-quickstart/) — first
  developer video
- [`04-ai-agents/`](04-ai-agents/) — longest, richest, the
  flagship technical video
- [`05-vs-did-vc/`](05-vs-did-vc/) — comparison video

Each directory contains:

- `shotlist.md` — second-by-second shot list with what's on
  screen, what's said, what asset to use
- `assets.md` — checklist of every asset needed, where it
  comes from, specs (resolution, fps, duration)
- `distribution.md` — where each cut goes, with ready-to-paste
  captions
- `measurement.md` — what to measure after publish, decision
  criteria for re-thumbnailing / re-cutting
- Updated `script.md` (the earlier scripts moved inline)

---

## 7. Brand kit — lock in first

Before any video is produced, lock the brand. This is cheap to
do now and expensive to retrofit across a series.

### Colors
- **Primary**: deep navy `#0B1D3A` (cryptographic / trustworthy)
- **Accent**: signal-amber `#F9A825` (high-value, call-to-action)
- **Success / trust-high**: `#2E8B57`
- **Warning / trust-low**: `#C0392B`
- **Neutral / code bg**: `#1E1E1E` (VS Code dark matches)

### Typography
- **Headings**: Inter Bold / Space Grotesk Bold
- **Body**: Inter Regular / Söhne
- **Code**: JetBrains Mono 20px+ for every screen

### Intro / outro
- 3-second intro: Quidnug logo fades in over navy + amber
  gradient sweep
- 5-second outro: repo URL `github.com/bhmortim/quidnug` + all
  7 language logos + "Apache-2.0" badge

### Music
- Select 3–4 reusable beds from Epidemic Sound (search
  "tech documentary," "cinematic minimal," "ambient focus")
- Normalize every video to -16 LUFS (YouTube / Spotify
  standard)
- Music bed always at -18 to -24 dB relative to narration

### Caption style
- All captions: white text, 2px black outline, bottom-third
  of screen, 24px
- Burn-in for all social cuts
- Soft captions (SRT sidecar) for YouTube main cuts

---

## 8. Success criteria (per video)

Each video has explicit measurement targets 30 days post-publish.
Failure to hit these triggers a specific remediation (all in
each video's `measurement.md`).

| Video | Primary metric | Target | Fallback action |
| --- | --- | --- | --- |
| 1 (Hook) | Thumbnail CTR on YouTube | ≥ 4% | Re-thumbnail via TubeBuddy A/B |
| 1 (Hook) | Impressions via LinkedIn | ≥ 5000 | Rewrite caption, repost in 2 weeks |
| 2 (Overview) | 50%+ retention through 2:00 | ≥ 40% | Re-edit Acts 1→2 pacing |
| 3 (Python quickstart) | Time-to-first-successful-run | Viewers reporting "it worked" in comments | If negative comments > positive, debug the example |
| 4 (AI agents) | Watch-time to minute 5 | ≥ 45% | Trim architecture sections, re-upload |
| 5 (vs DIDs+VCs) | Shares among identity-architect audience | ≥ 20 shares | Re-post on LinkedIn with DID-community tags |

---

## 9. Distribution plan

Every video ships in three formats minimum:

1. **Full horizontal** (1920×1080) — YouTube main channel,
   embed in repo README.
2. **Vertical short** (1080×1920, ≤ 60 sec) — the most arresting
   60 seconds of the video, for LinkedIn / YouTube Shorts /
   Twitter / Bluesky.
3. **Square** (1080×1080) — Instagram feed (optional), LinkedIn
   feed secondary cut.

Distribution sequence per video:

- **Day 0** (publish):
  - YouTube main channel
  - Post to LinkedIn personal + company page with vertical cut
  - Twitter/X thread with vertical cut
- **Day 1**:
  - Bluesky + Mastodon posts with vertical cut
  - Update repo README with embed
- **Day 2**:
  - Hacker News submission (only for Videos 2 and 4 — reserve
    HN for your strongest content)
- **Day 5**:
  - Repost vertical cut on LinkedIn with different hook
- **Day 14**:
  - Cross-post to r/Python (V3), r/rust (V? — not in tier A),
    r/MachineLearning (V4), r/devops (V17 — later), etc.

---

## 10. What to skip even at doubled budget

Tempting but not worth it for the first 5:

- **Custom 3D animations.** Real 3D is $500+/shot and the
  payoff is marginal vs. motion-graphics 2D at our audience.
- **Actor-based footage.** Stock actors feel stocky. Your own
  voice + authentic HeyGen avatar > "corporate marketing video."
- **Live event recording.** Scripting beats ad-libbing at this
  stage. Once the series is proven, record live for conferences.
- **Multi-language dubs.** Do English + captions first. Add
  Spanish / Mandarin / German after you've proven traction in
  English. HeyGen dubbing is great but wait for signal.
- **Paid YouTube ads** to boost launch videos. Earned reach
  first; if week 4 metrics are weak, consider $100 TrueView test.

---

## 11. Quick-start checklist

Copy this, do it, and you'll be producing video by end of
week 1:

- [ ] Subscribe to HeyGen Creator, ElevenLabs Creator,
      Descript Creator, Runway Standard
- [ ] Record 2 minutes of reference video for HeyGen custom
      avatar (studio lighting, plain background, steady eye line)
- [ ] Record 1 minute of clean voice for ElevenLabs clone
      (quiet room, reading arbitrary tech text in your normal
      presenting cadence)
- [ ] Post Upwork / Fiverr brief for motion-graphics designer
      (include the 6 deliverables from Week 1 above)
- [ ] Set up a private YouTube channel for rough cuts
- [ ] Lock the brand kit (hex colors, fonts, music bed
      selections)
- [ ] Order intro/outro stinger on Fiverr (brief: "navy +
      amber 3-second Quidnug fade-in, 5-second outro with URL")

## License

Apache-2.0.
