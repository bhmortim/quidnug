# Video 4 — AI agent identity & provenance — shot list

**Final length:** 8:00
**Budget:** $260 (thumbnail $40, extra motion-graphics scenes $100, color grade $30, music $15, buffer $75)
**Production time:** 24 hours over 5 days
**Aspect ratios delivered:** 1920×1080 (YouTube main), 1080×1920 (90-second vertical highlight), 1080×1080 (for LinkedIn feed)

## Why this is the flagship

AI/ML platform teams are the fastest-growing technical
audience. Their identity problems (agent attribution,
tool-call provenance, RAG source trust) map cleanly to
Quidnug. This video has the highest ROI for adoption —
invest here.

## Key upgrade vs. base scripts

Doubled budget funds:
- Two custom "AI-layer-stack" animations from motion designer.
- Color-graded final video.
- Slightly longer runtime (8 min vs the overview's 3) — more
  space to show the code and architecture.
- Pro thumbnail with an LLM-identity concept.

## Macro structure

- 0:00 – 0:40 — Hook
- 0:40 – 1:30 — The mental model (4-layer stack)
- 1:30 – 3:30 — Show the code, Part 1 (registration + trust chain)
- 3:30 – 5:00 — Show the code, Part 2 (emit an LLM event)
- 5:00 – 6:00 — Downstream verification
- 6:00 – 7:00 — Multi-agent audit
- 7:00 – 7:40 — Where to use this
- 7:40 – 8:00 — Outro

## Second-by-second shot list

### Hook (0:00–0:40)

| Time | On-screen | Voice-over | Notes |
| --- | --- | --- | --- |
| 0:00–0:03 | Brand stinger | *(music: "agent thriller" — tense, electronic)* | Stinger |
| 0:03–0:20 | Screen-cap cuts: LLM agent calling a tool; chatbot with "I am Claude 4.5"; RAG system citing 7 sources; autonomous agent spinning up sub-agents | "If you run AI agents in production, you have an identity problem. You might not know it yet." | Storyblocks + screen captures |
| 0:20–0:35 | Close-up of agent output: "Based on sources [1][2][3]..." — then those citations fade with a big "[unverified]" red tag | "Which agent made this tool call? Did the output come from your fine-tuned model, or a prompt-injection impostor? How do you attribute a claim to a specific agent session on a specific model version? Today there are no cryptographic answers to any of these." | |
| 0:35–0:40 | Logo / title card: "AI agent identity on Quidnug" | "I want to show you how to fix that — with Quidnug." | |

### Mental model: the 4-layer stack (0:40–1:30)

| Time | On-screen | Voice-over | Notes |
| --- | --- | --- | --- |
| 0:40–0:55 | HeyGen avatar PIP + text "Every principal is a quid" | "In Quidnug, every principal is a quid — a cryptographic identity with an ECDSA P-256 keypair." | |
| 0:55–1:30 | **Custom animated 4-layer stack** builds bottom-up, each layer labeled: Foundation Model → Deploying Org → Agent → Action | "For AI workflows we give quids to four levels. Level one — the foundation model. Anthropic's Claude checkpoint. OpenAI's GPT-4o. The weights get a quid. Level two — your lab, your startup, your team. They vouch for the specific model instances. Level three — the agent. A specific session. Could be minutes, could be hours. Gets its own keypair. Level four — every action the agent takes. Every LLM call, tool invocation, retrieved document, gets recorded as a signed event on the agent's stream." | Motion-graphics Asset A (new) |

### Code, Part 1 — setup + trust chain (1:30–3:30)

| Time | On-screen | Voice-over | Notes |
| --- | --- | --- | --- |
| 1:30–1:40 | Transition to VS Code with file `examples/ai-agents/agent_identity.py` open | "Here's the minimal end-to-end flow in Python — the runnable example from our `examples/ai-agents/` directory." | Pre-opened |
| 1:40–2:00 | Scroll through the imports + actor creation (lab, model, agent, consumer) | "Four actors: the lab, the model checkpoint, the agent session, and a downstream consumer. Each gets a fresh keypair." | Paste live |
| 2:00–2:20 | Register identities — walk through the `client.register_identity` calls | "Every actor is registered on the Quidnug node. The model gets attributes — model family, version, training checkpoint." | |
| 2:20–2:40 | Focus on the trust chain: lab→model at 1.0, model→agent at 0.95 | "Lab vouches for the specific model weights checkpoint at trust 1.0. The model vouches for this agent session at 0.95 — a slight decay because the session could go rogue even if the weights are solid." | |
| 2:40–2:55 | Focus on the consumer's direct trust in the lab at 0.9 | "Meanwhile our downstream consumer has trust 0.9 directly in the lab — they've evaluated the lab's safety practices." | |
| 2:55–3:30 | Zoom out: small animated overlay shows the trust graph assembling | "So we've built a chain: consumer → lab → model → agent." | Motion-graphics Asset B (reuse from Video 2) |

### Code, Part 2 — emit an LLM event (3:30–5:00)

| Time | On-screen | Voice-over | Notes |
| --- | --- | --- | --- |
| 3:30–3:40 | Switch to the `emit_event` code block | "Now the agent makes an actual LLM call. Let's record it as a signed event." | |
| 3:40–4:05 | Highlight payload fields: `model`, `systemPromptDigest`, `inputDigest`, `outputDigest`, `temperature`, `tokensIn`, `tokensOut` | "A couple of things to notice. We record digests, not raw text. Inputs and outputs are often sensitive; a SHA-256 is enough for tamper-evidence without leaking PII. And we capture the inference parameters — model name, temperature, token counts — everything an auditor needs to reproduce." | |
| 4:05–4:30 | Zoom on `signer=agent` in the function call; arrow points to the signing step | "This event is signed by the agent's key. The agent's key was issued by the model, which was issued by the lab. The signature chain is intact." | |
| 4:30–4:50 | Animation: an event flows into an append-only log with a lock icon | "And it goes onto the agent's stream — append-only, protected by the nonce ledger. You can't rewrite history." | Motion-graphics Asset C (new) |
| 4:50–5:00 | Run the script; output appears | *(runs)* "Event emitted — got an ID and a sequence number." | |

### Downstream verification (5:00–6:00)

| Time | On-screen | Voice-over | Notes |
| --- | --- | --- | --- |
| 5:00–5:15 | Highlight the consumer's `get_trust` call | "Now the consumer wants to check: should I trust the answer this agent gave?" | |
| 5:15–5:30 | Run it; output: "trust level: 0.855, path: consumer → lab → model → agent" | *(output appears, subtle zoom)* "Consumer trusts lab at 0.9. Lab trusts model at 1.0. Model trusts agent at 0.95. Transitive: 0.9 × 1.0 × 0.95 = 0.855." | |
| 5:30–5:50 | Below 0.7, show a red X; above 0.7, green check. Configure threshold slider animates from 0.5 → 0.9 | "Above our 0.7 threshold, so we accept. Below 0.7 — reject. The threshold is your policy, encoded as a simple numeric check." | |
| 5:50–6:00 | Quick cut: show how lowering the lab's trust from 0.9 to 0.5 cascades all the way to the agent being rejected | "If the lab got compromised tomorrow and you dropped their trust to 0.5, every agent in the chain drops below threshold automatically. No whitelist editing. No policy-file PR. The graph does the work." | Motion-graphics Asset D (new) |

### Multi-agent audit (6:00–7:00)

| Time | On-screen | Voice-over | Notes |
| --- | --- | --- | --- |
| 6:00–6:20 | Animated graph: agent A calls agent B calls agent C; each call is an EVENT with the callee's quid in payload | "In a multi-agent system, each agent-to-agent call is one more event on the caller's stream with the callee's quid ID in the payload." | Motion-graphics Asset E (new) |
| 6:20–6:45 | Walking through the call-graph; every edge has a signature | "An auditor walks every stream and reconstructs the full call graph. Every edge is cryptographically signed. Every signer is scorable via relational trust." | |
| 6:45–7:00 | Split-screen: LangSmith on the left ("logs"), Quidnug on the right ("signatures + trust graph") | "Fundamentally different from observability tools like LangSmith or Weights & Biases. Those are cooperative — the agent has to log honestly. Quidnug is cryptographic — the agent can't claim it did something it didn't, because the claim has to be signed." | Side-by-side screens |

### Where to use this (7:00–7:40)

| Time | On-screen | Voice-over | Notes |
| --- | --- | --- | --- |
| 7:00–7:40 | Four labeled cards fade in: "RAG pipelines," "Multi-agent coordination," "Model checkpoint attribution," "Agent marketplaces" | "Concrete fits today. RAG pipelines — each retrieved source cited as an event, filterable by relational trust. Multi-agent coordination — every tool call is an audit trail. Model-checkpoint attribution — prove your output came from a specific fine-tuned weights blob. Agent marketplaces — agents publish their Quidnug identity, users grant per-observer trust." | 4 info cards, sequenced |

### Outro (7:40–8:00)

| Time | On-screen | Voice-over | Notes |
| --- | --- | --- | --- |
| 7:40–8:00 | Brand outro + `examples/ai-agents/` URL + language logos | "Full working Python script at `examples/ai-agents/` in the repo. Apache-2.0. No gatekeeper. No token. github.com/bhmortim/quidnug." | |

## Motion-graphics asset list (new for this video)

| ID | Description | Duration | Used at | Cost |
| --- | --- | --- | --- | --- |
| A | 4-layer stack building bottom-up | 35s | 0:55–1:30 | Included in $250 kit |
| B | Trust graph with chain highlight (reuse) | 5s | 2:55–3:30 | Reuse |
| C | Event flowing into append-only log | 10s | 4:30–4:50 | Part of extra $100 scenes |
| D | Cascade animation (lab trust drops → agent drops below threshold) | 10s | 5:50–6:00 | Part of extra $100 scenes |
| E | Multi-agent call graph | 20s | 6:00–6:45 | Part of extra $100 scenes |

Motion-designer brief: "Three additional scenes at $100 total
extending the Quidnug visual kit. Style consistent with the
existing 6 scenes. Delivered as 1920×1080 MP4 + alpha PNG
sequence."

## Audio post

- Avatar segments: normalized to -16 LUFS, de-essed.
- Screen-recording segments (keyboard clicks): reduce keyboard
  sound to near-inaudible in post, some viewers find it
  distracting.
- Music bed: continuous through video except during key
  code-pasting moments (2:00, 2:20, 2:40, 3:30) where you
  drop to -30 dB for 2 seconds so narration is crystal clear.

## Export spec

| Deliverable | Dimensions | Duration | Target |
| --- | --- | --- | --- |
| YouTube master | 1920×1080 60fps | 8:00 | Primary |
| Vertical 90s highlight | 1080×1920 60fps | 1:30 | Take 5:00–6:30 (verification + cascade) |
| Square | 1080×1080 60fps | 8:00 | LinkedIn feed |
| Thumbnail | 1280×720 PNG | — | YouTube |

## Thumbnail brief

> Design a YouTube thumbnail for "AI Agent Identity on Quidnug
> — Cryptographic Provenance for LLM Calls."
>
> Left half: a stylized AI agent silhouette (could be a robot
> head or an abstract geometric "brain"); a faint cryptographic
> key glows behind it.
>
> Right half: bold white text "CRYPTOGRAPHIC AI IDENTITY"
> with a smaller kicker "every call signed, every source scored."
>
> Background: navy with subtle amber particles suggesting
> signal / data.
>
> Brand colors: navy `#0B1D3A`, amber `#F9A825`, cyan `#2FCFE0`
> for the agent glow.

## Distribution captions

### LinkedIn
> Which LLM made this call? Did the output come from the fine-
> tuned model or a prompt-injection impostor?
>
> Cryptographic answers: not standard yet.
>
> Built Quidnug — every AI agent gets a signed identity, every
> LLM call becomes a tamper-evident event, relational trust
> scores each signer per observer.
>
> Full 8-minute walk-through: [YouTube link]
> 90-second highlight: [vertical clip]
>
> github.com/bhmortim/quidnug

### Twitter
> "Which agent said this?" "Did this output come from the
> model you think it did?" "Did this tool call actually
> happen?"
>
> No cryptographic answers today.
>
> Built it: [YouTube link]

### Reddit r/MachineLearning
Title: "Cryptographic agent identity for multi-agent LLM
workflows — signed tool calls + per-observer relational
trust. Open source."
Body: Link + description of the signature chain. Ask for
feedback on the agent-marketplaces pattern.

### Hacker News (high priority — this is a strong HN fit)
Title: `Cryptographic identity for AI agents — signed LLM
calls and relational trust`
Body: Link the YouTube video + GitHub repo + runnable example.

## License

Apache-2.0.
