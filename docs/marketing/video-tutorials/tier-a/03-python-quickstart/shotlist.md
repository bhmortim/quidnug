# Video 3 — Python SDK quickstart — shot list

**Final length:** 5:00
**Budget:** $80 (thumbnail $40, music $15, buffer $25 — cheap because no motion graphics needed)
**Production time:** 8 hours over 3 days
**Aspect ratios delivered:** 1920×1080 horizontal (main), 1080×1920 60s vertical "Python in 60 sec" cut

## What this video does

The first developer-facing video; convinces a Python developer
(likely FastAPI / ML / data-engineering background) to `pip
install quidnug` and run the example. This is where the
series earns its first adopter.

## Key upgrade vs. base scripts

The doubled budget lets us:
- Hire a thumbnail designer (critical for YouTube discovery).
- Add a proper "next steps" end card animation.
- Record in a quiet treated room OR rent a studio for one day.

Motion graphics are NOT needed — screen + voice is the right
format for this audience.

## Pre-flight (30 min before recording)

- [ ] Fresh Python 3.11+ virtualenv.
- [ ] Local node up: `cd deploy/compose && docker compose up -d`.
- [ ] VS Code with JetBrains Mono 20px, Dark Modern theme,
      zoom level 150%.
- [ ] Terminal: Windows Terminal / iTerm2 with 18pt font,
      dark transparent background.
- [ ] OBS Studio configured for 1920×1080 60fps capture at 8
      Mbps.
- [ ] Notifications off; all non-essential tabs closed.
- [ ] Demo file pre-written at `/tmp/demo.py` — you'll copy
      lines live, not type live (typing live is slow and
      error-prone).

## Second-by-second shot list

### 0:00–0:20 — Intro

| Time | On-screen | Voice-over | Notes |
| --- | --- | --- | --- |
| 0:00–0:03 | Brand stinger → fade to your face (webcam PIP bottom-right) over IDE | *(music: light tech upbeat)* | Stinger |
| 0:03–0:08 | PIP you speaking + background VS Code open to blank file | "In this video we'll set up the Python SDK for Quidnug from scratch, register two identities, create a trust relationship, and query relational trust — all in under five minutes." | Webcam shot by you |
| 0:08–0:15 | PIP you + on-screen text: "5-minute quickstart," "Python 3.9+" | "If you want to skip straight to the code, the finished example is at `examples/ai-agents/` in the repo — I'll link it below." | |
| 0:15–0:20 | Cut to full-screen terminal | *(short music pause)* | Transition into demo |

### 0:20–1:00 — Install

| Time | On-screen | Voice-over | Notes |
| --- | --- | --- | --- |
| 0:20–0:30 | Terminal: `python -m venv venv` + activation | "First, fresh Python environment." | Pre-typed, you hit enter |
| 0:30–0:50 | Terminal: `pip install quidnug` with output | "Pip pulls in the Python SDK, built on cryptography and requests. No other runtime dependencies." | |
| 0:50–1:00 | Terminal: `python -c "import quidnug; print(quidnug.__version__)"` → `2.0.0` | "Version 2.0 — we're ready." | |

### 1:00–1:40 — Start a local node

| Time | On-screen | Voice-over | Notes |
| --- | --- | --- | --- |
| 1:00–1:10 | Terminal: `cd quidnug/deploy/compose` | "If you already have a Quidnug node somewhere, skip this. I'll use the Docker Compose dev setup — three nodes, Prometheus, Grafana all wired up." | |
| 1:10–1:30 | Terminal: `docker compose up -d` followed by `docker compose ps` | *(narrates over output)* | Containers spin up |
| 1:30–1:40 | Terminal: `curl http://localhost:8081/api/health` returning `{"success":true,"data":{"status":"ok"}}` | "All three nodes up. We'll talk to the first one on port 8081." | |

### 1:40–3:20 — Write the code

| Time | On-screen | Voice-over | Notes |
| --- | --- | --- | --- |
| 1:40–1:50 | Switch to VS Code, new file `demo.py` | "Open a new file: `demo.py`." | |
| 1:50–2:15 | Paste imports and client init (line by line, with brief pauses) | "Pull in Quid and QuidnugClient. Point the client at our local node." | Use VS Code live-share-style paste |
| 2:15–2:35 | Paste `alice = Quid.generate()` and `bob = Quid.generate()` and `print` | "`Quid.generate` creates a fresh ECDSA P-256 keypair. The quid ID is the first 16 hex characters of the public key's SHA-256 — the same ID every other Quidnug SDK produces for the same keypair." | |
| 2:35–2:55 | Paste `register_identity` calls | "`register_identity` submits a signed IDENTITY transaction. The SDK canonicalizes and signs internally — you don't see any of the wire-format crypto." | |
| 2:55–3:10 | Paste `grant_trust` call | "`grant_trust` issues a trust edge from Alice to Bob with level 0.9 in the 'demo' domain. Domains scope the trust — Alice trusting Bob in 'demo' doesn't mean anything in 'contractors.home.'" | |
| 3:10–3:20 | Paste `get_trust` query and print statements | "And `get_trust` runs a relational-trust query from Alice's perspective to Bob. Since there's a direct edge, the result is 0.9 along a one-hop path." | |

### 3:20–3:50 — Run it

| Time | On-screen | Voice-over | Notes |
| --- | --- | --- | --- |
| 3:20–3:25 | Cut to terminal | "Let's run it." | Transition |
| 3:25–3:40 | Terminal: `python demo.py` → output | *(narrates output as it appears)* | |
| 3:40–3:50 | Freeze-frame on output, zoom in on `trust = 0.900` | "We have a Quidnug network with two identities, a signed trust relationship, and a queryable score." | |

### 3:50–4:30 — Transitive trust

| Time | On-screen | Voice-over | Notes |
| --- | --- | --- | --- |
| 3:50–4:00 | Cut back to editor; add `carol = Quid.generate()` and her register/trust lines | "Now let's show why relational trust is the interesting part. Add Carol." | |
| 4:00–4:15 | Paste Carol registration + Bob granting Carol trust at 0.8 | "Bob trusts Carol at 0.8. Alice has never met Carol — no direct edge." | |
| 4:15–4:30 | Add a second `get_trust` for Alice→Carol, run it | "What's Alice's transitive trust in Carol?" *(re-run, output appears)* "0.72. Because Alice trusts Bob at 0.9, Bob trusts Carol at 0.8 — transitively Alice trusts Carol at 0.9 × 0.8 = 0.72. This is what makes Quidnug different from a certificate authority or blockchain reputation: trust composes along the graph, per-observer." | |

### 4:30–5:00 — Where to go next

| Time | On-screen | Voice-over | Notes |
| --- | --- | --- | --- |
| 4:30–4:40 | PIP you + end-card showing 4 follow-up items | "That's the core. From here:" | |
| 4:40–4:50 | Each follow-up item lifts up in sequence: "Async client (FastAPI/asyncio)," "Full protocol surface (guardians, events, gossip)," "AI-agent example," "Comparison docs" | "The async client if you're in FastAPI or asyncio. The full protocol surface — guardians, event streams, cross-domain gossip. The AI-agent example if you're working with LLM attribution. And the comparison docs if you're weighing Quidnug against DIDs plus Verifiable Credentials or blockchain reputation." | |
| 4:50–5:00 | Outro stinger, URL, "Apache-2.0" | "All linked in the description. Apache-2.0. See you in the next video." | Brand outro |

## Timing check

5-minute script covers ~800 words at 160 wpm — realistic.
Terminal/editor moments are your breathing room; use them.

## Music cues

| Time | Action |
| --- | --- |
| 0:00 | Light tech upbeat bed enters |
| 0:20 | Drop volume by 6 dB under narration |
| 3:20 | Brief swell at the "run it" cut |
| 4:30 | Sustained bed through end card |
| 4:55 | Fade out |

## Pre-written demo script

Save this file at `/tmp/demo.py` BEFORE recording so you're
pasting, not typing live:

```python
from quidnug import Quid, QuidnugClient

client = QuidnugClient("http://localhost:8081")

alice = Quid.generate()
bob = Quid.generate()
print(f"alice = {alice.id}")
print(f"bob   = {bob.id}")

client.register_identity(alice, name="Alice", home_domain="demo")
client.register_identity(bob,   name="Bob",   home_domain="demo")

client.grant_trust(alice, trustee=bob.id, level=0.9, domain="demo")

tr = client.get_trust(alice.id, bob.id, domain="demo")
print(f"trust = {tr.trust_level:.3f}")
print(f"path  = {' -> '.join(tr.path)}")

# Transitive section (appears at 3:50)
carol = Quid.generate()
client.register_identity(carol, name="Carol", home_domain="demo")
client.grant_trust(bob, trustee=carol.id, level=0.8, domain="demo")
tr = client.get_trust(alice.id, carol.id, domain="demo")
print(f"alice->carol trust = {tr.trust_level:.3f}")
print(f"path = {' -> '.join(tr.path)}")
```

## Export spec

| Deliverable | Dimensions | Duration | Target |
| --- | --- | --- | --- |
| Horizontal master | 1920×1080 60fps | 5:00 | YouTube, Twitch re-use |
| Vertical "60-sec quickstart" cut | 1080×1920 60fps | 0:60 | 0:20–1:00 (install + pip) for maximum hook |
| Square | 1080×1080 60fps | 5:00 | Skip for this video |

## Thumbnail brief

> Design a YouTube thumbnail for "Quidnug Python SDK in 5
> Minutes (Quickstart)."
>
> Left half: large Python logo + "5:00" clock icon + VS Code
> screenshot fragment. Right half: huge bold text "QUIDNUG
> PYTHON QUICKSTART" in white with navy background. Accent:
> amber arrow from the "Python" logo to a "trust = 0.900"
> fake output snippet.
>
> Brand colors: navy `#0B1D3A`, amber `#F9A825`.

## Distribution captions

### LinkedIn
> `pip install quidnug` — 5-minute quickstart for relational
> trust in Python.
>
> What you get: ECDSA P-256 keypair, signed identities, signed
> trust edges, transitive trust queries. Apache-2.0.
>
> Full 5-minute video: [YouTube link]
> 60-second highlights: [vertical clip]
>
> github.com/bhmortim/quidnug

### Twitter
> Python devs — built an SDK for relational trust over P2P.
>
> Works like this:
>
> ```
> alice = Quid.generate()
> client.grant_trust(alice, trustee=bob.id, level=0.9)
> tr = client.get_trust(alice.id, carol.id)  # transitive
> ```
>
> 5-minute quickstart: [link]

### Reddit r/Python
Title: "Built a Python SDK for per-observer relational trust
(decentralized, ECDSA P-256). 5-min quickstart"
Body: Link to the video + description of what problem it
solves. Ask for SDK feedback.

## License

Apache-2.0.
