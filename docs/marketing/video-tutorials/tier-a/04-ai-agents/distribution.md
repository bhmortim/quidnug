# Video 4 — distribution plan

Video 4 targets the AI/ML platform audience — a fast-growing
technical segment. This video has the highest potential for
viral reach in this series.

## Publish sequence

### Day 0
- [ ] YouTube upload.
- [ ] Post to LinkedIn with AI-platform framing.
- [ ] Post to Twitter — thread, 3-4 tweets.

### Day 0 — 10am PT
- [ ] HN submission with strong framing — "Cryptographic
      identity for AI agents."
- [ ] First HN comment links to the running example + the
      QDP-0002 (guardian recovery) design doc.

### Day 1
- [ ] r/MachineLearning post (respect their self-promotion
      rules — your first post should have substantive
      technical content, not just a link).
- [ ] Reply to all comments.

### Day 2
- [ ] DevTo companion post: "Cryptographic Agent Identity
      and LLM Attribution: A Practical Pattern."
- [ ] Twitter thread follow-up with the 90-second vertical
      cut highlighting the cascade animation (5:50–6:00).

### Day 7
- [ ] Pitch to AI/ML-focused podcasts (Latent Space, MLOps
      Podcast, Practical AI). Offer a 30-min interview on
      "How we're thinking about agent identity."
- [ ] Cross-post to /r/LocalLLaMA if the self-hosted angle
      is relevant.
- [ ] Submit to Hugging Face community forum.

### Day 14
- [ ] Reach out to LangChain, LlamaIndex, CrewAI, AutoGen
      maintainer communities with "we built this — does it
      fit with your framework?" — asking about potential
      integrations.

## YouTube metadata

**Title:** `Cryptographic identity for AI agents — signed LLM calls, per-observer trust`
**Description:**
```
Eight minutes on how to give every AI agent a cryptographic
identity, record every LLM call as a signed event, and
score agents via per-observer relational trust.

Solves the "which model made this call?" problem that
chatbots, RAG pipelines, and multi-agent workflows face
today.

Chapters:
0:00 — The problem
0:40 — The 4-layer mental model
1:30 — Code part 1: trust chain setup
3:30 — Code part 2: emit an LLM event
5:00 — Downstream verification
6:00 — Multi-agent audit
7:00 — Use cases
7:40 — Outro

What you get:
• Every agent gets a Quid (ECDSA P-256 keypair).
• Every LLM call is a signed event on the agent's stream.
• Every source in a RAG pipeline gets a trust score.
• Multi-agent call graphs are cryptographically reconstructable.

Runnable example: examples/ai-agents/agent_identity.py
Full repo: https://github.com/bhmortim/quidnug

Related video: Quidnug in 3 minutes — [Video 2 link]

#AI #LLM #Cryptography #OpenSource #Agents #MLOps
```

**Tags:** AI agents, LLM, agent identity, cryptographic,
MLOps, LangChain, AutoGen, Claude, GPT-4, multi-agent,
RAG, open source, quidnug, trust

## Hacker News

**Submission title:** `Cryptographic identity for AI agents
— signed LLM calls and relational trust`
**URL:** YouTube video (or the repo + video link in first
comment, test both approaches).

**First comment:**
```
Author here. Happy to answer questions.

A big gap I keep running into: everyone building multi-agent
systems asks "which agent said this?" and "did this output
actually come from the model you think it did?" but there's
no cryptographic answer to those questions today.

This video shows a pattern where:
1. Each agent gets a P-256 keypair.
2. The lab/org cryptographically vouches for the model
   checkpoint.
3. The model vouches for each agent session.
4. Every LLM call is a signed event on the agent's stream.
5. Downstream consumers score via relational trust.

Full runnable Python example (no mock servers, actual
protocol implementation): https://github.com/bhmortim/quidnug/tree/main/examples/ai-agents

Apache-2.0.

Where I'm especially interested in feedback: the threat model
for compromised model keys and whether the M-of-N guardian
recovery (QDP-0002) is the right primitive for rotating an
inference endpoint's signing key when weights are repaired.
```

## LinkedIn

```
"Which AI agent made this call?"
"Did this come from your fine-tuned model, or a prompt-
injection impostor?"
"Can I audit which tools this agent invoked?"

Today: no cryptographic answers.

We built a pattern where every agent has a Quidnug quid
(ECDSA P-256 keypair), every LLM call becomes a signed
event on the agent's stream, and downstream consumers
score agents via per-observer relational trust.

Full 8-minute walk-through: [YouTube link]
90-second highlight (the cascade animation): [vertical clip]

Runnable example (Python, 10 minutes end-to-end):
github.com/bhmortim/quidnug/tree/main/examples/ai-agents

Apache-2.0.

#AI #MLOps #LLM #Cryptography #OpenSource
```

## Twitter / X (thread, 4 tweets)

**1/4:**
```
AI agent identity is an unsolved problem.

"which agent said this?" "did this LLM call come from the
model you think?" "can I audit which tools got invoked?"

no cryptographic answers today.

built one:
```

**2/4:**
```
2/4 pattern:

every agent = ECDSA P-256 keypair (a "quid")
every LLM call = signed event on that quid's stream
every RAG source = cited by its quid + inherits the source's
trust

consumers score agents via per-observer relational trust.

8-min video:
```

**3/4:**
```
3/4 the cascade is the fun part — if you lower trust in
one upstream (say the lab was compromised), every agent
downstream drops below threshold automatically.

no whitelist PRs. the graph does the work.

(demo at 5:50 in the video)
```

**4/4:**
```
4/4 runnable Python example, no mock servers — real protocol
working end-to-end:

github.com/bhmortim/quidnug/tree/main/examples/ai-agents

Apache-2.0. thoughts welcome.
```

## Reddit r/MachineLearning

**Title:** "[P] Cryptographic identity for AI agents — signed
tool calls + per-observer relational trust"
**Body:**
```
I've been working on an open-source protocol (Quidnug) for
per-observer relational trust and recently shipped the
AI-agent integration pattern. Published an 8-minute
technical walk-through: [YouTube].

Core idea: instead of trusting agent output because "the
platform says it's from Model X," cryptographically sign
each LLM call and score agents via transitive trust from
the consumer's own viewpoint.

Concrete fits:
- RAG pipelines where source citations can be filtered
  by relational trust
- Multi-agent coordination where calls need audit trails
- Model-checkpoint attribution for fine-tuned derivatives
- Agent marketplaces where buyers need per-observer
  scoring

Runnable Python example:
github.com/bhmortim/quidnug/tree/main/examples/ai-agents

Curious for feedback: is this the right layer for agent
identity, or does it belong further up the stack (e.g.
inside LangChain / AutoGen directly)?
```

## Success criteria (abbreviated)

| Metric | Target | At |
| --- | --- | --- |
| HN score | ≥ 100 | 24h |
| HN comments (substantive) | ≥ 30 | 48h |
| YouTube views | ≥ 5000 | 30d |
| Retention at 5:00 mark | ≥ 40% | 30d |
| GitHub stars delta post-HN | +100 | 48h |
| LangChain / AutoGen / CrewAI inbound | ≥ 1 contact | 14d |
| Podcast inbound | ≥ 1 serious inquiry | 30d |

## License

Apache-2.0.
