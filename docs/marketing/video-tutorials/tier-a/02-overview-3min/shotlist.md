# Video 2 — "Quidnug in 3 minutes" — shot list

**Final length:** 3:00
**Budget:** $220 (custom avatar $50 amortized, motion kit shared, thumbnail $40, music $15, color grade $30, buffer $85)
**Production time:** 16 hours over 5 days
**Aspect ratios delivered:** 1920×1080 (YouTube master), 1080×1920 (60s vertical cut of Act 2), 1080×1080 (square hook cut)

## Key upgrade vs. base scripts

With the doubled budget, this video becomes the **flagship**
for the technical audience. We're investing in:

- Custom HeyGen avatar (trained on your face — pays off over
  all of Tier A + Tier B).
- Four distinct animated architecture frames instead of static.
- Pro color grading pass.
- Music bed timed to scene transitions.

## Second-by-second shot list

### Act 1 — the problem (0:00–0:45)

| Time | On-screen | Voice-over | Source / asset |
| --- | --- | --- | --- |
| **0:00–0:03** | Stinger intro → fade to HeyGen avatar talking | *(music in: tech documentary, low)* | Brand stinger |
| **0:03–0:10** | Avatar speaks, clean background | "I'm going to explain Quidnug in under three minutes. Here's the problem." | HeyGen |
| **0:10–0:20** | Split-screen montage: medical records, supply-chain diagram, AI agent dashboard, election ballot, vendor KPIs | "Imagine you're building a system where trust matters. Medical records. Supply-chain provenance. AI-agent attribution. Election integrity. Vendor management." | Storyblocks stock + motion kit |
| **0:20–0:30** | Back to avatar PIP; three question marks float in | "You need to answer questions like: Should I trust this party's claim? Who vouches for them? And: if someone in the chain gets compromised, how do I recover?" | HeyGen + motion overlay |
| **0:30–0:45** | Three animated boxes fade in: "Centralized Reputation," "Blockchains," "Web of Trust" — each gets briefly described then fades out with a light-red ×. | "Today there are basically three answers. Centralized reputation — fine if you trust the scorer. Blockchains — global consensus but public forever, with fees per write. Federation like PGP — no central authority but no recovery, no domain scoping, no typed relationships. We wanted something different." | Motion kit asset: new "3 alternatives" animation |

### Act 2 — what Quidnug is (0:45–1:45)

| Time | On-screen | Voice-over | Source / asset |
| --- | --- | --- | --- |
| **0:45–0:55** | Quidnug wordmark reveal over dark navy background; music swell | "Quidnug is a decentralized protocol for **relational trust**." | Brand card |
| **0:55–1:10** | Animated reveal of a "quid" — person icon transforms into cryptographic key icon with a 16-char hex ID label | "Every person, organization, AI agent, and device is a quid — a cryptographic identity with an ECDSA P-256 keypair. You own your key. No central registrar." | Motion kit asset #2 (quid identity reveal) |
| **1:10–1:25** | Two quids appear side by side; a directed arrow labeled "trusts at 0.9" draws between them; then another arrow labeled "in the 'vendor' domain" | "Each quid issues signed trust edges to other quids. 'I trust Alice at 0.9 in the vendor domain.' The edge is scoped to a domain — so trusting someone as a vendor doesn't leak into trusting them as an election observer." | New motion beat |
| **1:25–1:40** | Full trust graph builds up; observer A highlighted; her POV shows scores to other nodes; camera moves to observer B; her POV shows different scores | "Quidnug answers relational-trust queries: from observer A's perspective through the graph, what's the effective trust in target B? It finds the best path, multiplies the edge weights, and returns the decayed trust." | Motion kit asset #1, all three frames sequenced |
| **1:40–1:45** | Three observer silhouettes, each seeing a different number | "Every observer gets their own answer." | End of motion kit asset #1 |

### Act 3 — why you'd use it (1:45–2:30)

| Time | On-screen | Voice-over | Source / asset |
| --- | --- | --- | --- |
| **1:45–1:55** | Back to avatar | "Why does this matter?" *(beat)* | HeyGen |
| **1:55–2:10** | Code screenshot: a typical "if issuer in WHITELIST" policy block; red strikethrough; replaced with "if trust(observer, issuer) >= 0.7" | "Today, when your app asks 'should I accept this claim?', the answer has to be a policy decision — 'accept if issued by one of my 12 whitelisted CAs.' That list never scales. With relational trust, the policy becomes declarative." | Custom code overlay from VS Code |
| **2:10–2:30** | Three icons animate in: event log (QDP-0001), guardian shield (QDP-0002), gossip arrows (QDP-0003) | "On top of that: typed event streams — every quid has an append-only signed log. Guardian-based recovery — M-of-N signatures to rotate a lost key with a time-locked veto. Cross-domain gossip via compact Merkle proofs so no domain has to trust another domain's full history." | Motion kit asset #5 (3 icons reveal) |

### Act 4 — how to try it (2:30–3:00)

| Time | On-screen | Voice-over | Source / asset |
| --- | --- | --- | --- |
| **2:30–2:40** | Language logos row: Python, Go, JavaScript, Rust, Java, .NET, Swift | "We ship seven SDKs, all at full protocol parity." | Simple Icons montage |
| **2:40–2:50** | Live terminal recording: `cd deploy/compose && docker compose up -d` → 3 nodes appear as Docker containers; `curl http://localhost:8081/api/health` returns `{"success": true}` | "Plus a Docker Compose dev network. One command:" | Screen recording by you |
| **2:50–2:55** | Back to avatar, smiling transition | "The Python quickstart is 10 lines. Followable in five minutes." | HeyGen |
| **2:55–3:00** | Outro: URL + logo + Apache-2.0 badge + language logos | "Apache-2.0. Open source. github.com/bhmortim/quidnug." | Brand outro |

## Timing check

Total words: ~480 → at 160 wpm = 3:00 exactly. **No slack.** Any
longer and you need to cut from Act 1 (which is the most
compressible).

## Music cues

| Time | Action |
| --- | --- |
| 0:00 | Soft in — tech documentary low |
| 0:45 | Swell at wordmark — intensity up |
| 1:45 | Sustain, slight build |
| 2:30 | Percussive lift for the demo + language logos |
| 2:55 | Fade under outro to let URL register |
| 3:00 | Full fade |

## Audio post

- Voice normalized to -16 LUFS (the HeyGen export will need
  adjustment — it defaults around -19 LUFS).
- Music bed consistently -22 dB under voice.
- Light de-ess on HeyGen output (avatar voices can be
  slightly sibilant).
- Final limiter at -1 dB ceiling.

## Export spec

| Deliverable | Dimensions | Duration | Target |
| --- | --- | --- | --- |
| YouTube master | 1920×1080 60fps | 3:00 | Primary channel upload |
| Vertical cut | 1080×1920 60fps | 0:60 | Clip of Act 2 + opening of Act 3 — social traction |
| Square | 1080×1080 60fps | 3:00 | Secondary LinkedIn feed |
| Thumbnail | 1280×720 PNG | n/a | YouTube thumbnail |

## Thumbnail brief

> Design a YouTube thumbnail for "Quidnug in 3 Minutes —
> Decentralized Trust Explained."
>
> Left third: the Quidnug custom HeyGen avatar headshot
> (export a frame at 1:12 — avatar looking toward the
> right), desaturated with navy tint.
>
> Right two-thirds: text layered in bold white sans-serif:
> Line 1 (huge): "3-MINUTE"
> Line 2 (medium): "relational trust explained"
> Small "WHY ONE NUMBER ISN'T ENOUGH" kicker above Line 1.
>
> Bottom-right: small Quidnug wordmark. Background: navy
> with subtle amber glow behind the avatar.

## Voice direction for HeyGen avatar

The avatar will read the script. Direction (paste into HeyGen's
notes field):

- **Tone**: engineer explaining to a peer; smart, direct,
  modestly enthusiastic. Not corporate.
- **Pace**: 160 wpm. This is the limit before it feels
  rushed.
- **Pausing**: half-second pause at every sentence break; full
  second at act transitions (0:45, 1:45, 2:30).
- **Emphasis**: lean on "relational," "quid," "per-observer,"
  "your own answer," "Apache-2.0."
- **Eye line**: camera-direct except during beats 0:55–1:10 and
  1:25–1:40 — during those, reduce avatar opacity to 60% so
  the animation reads cleanly underneath.

## Distribution captions (ready to paste)

### LinkedIn
> 3 minutes on why "trust" in software has been one-
> size-fits-all for too long.
>
> We built an Apache-2.0 protocol for relational trust
> — trust that's personal, transitive, and cryptographic.
>
> Full video (3:00 on YT, 60s vertical here):
>
> [Vertical clip]
>
> github.com/bhmortim/quidnug

### Twitter / X
> "trust" today = one number per entity, global.
>
> in Quidnug: per-observer transitive trust. Alice and Bob
> can have genuinely different trust in the same vendor.
>
> 3-minute explainer:
>
> (YouTube link)

### Hacker News (only post after Video 2 is live)
Title: `Quidnug — a decentralized protocol for relational trust (Apache-2.0)`
Body: (post Video 2's YouTube embed URL as the submission URL)
First comment: link to README, explicitly note the HN audience
is being asked for blunt protocol-design feedback.

## License

Apache-2.0.
