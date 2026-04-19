# Script 04 — AI agent identity & provenance

**Length:** 8:00
**Audience:** Platform/infra engineers at AI-first companies,
agent-framework builders (LangChain, LlamaIndex, AutoGen,
CrewAI, etc.), ML platform teams.
**Goal:** Convince them Quidnug is the right identity layer for
multi-agent workflows.
**Recommended stack:** HeyGen avatar for presenter segments +
Descript for terminal demos + Figma for architecture diagrams.

---

## Script

### [0:00–0:40] Hook

> "If you run AI agents in production, you have an identity
> problem. You just might not know it yet."
>
> "Which agent made this tool call? Did the output come from
> your fine-tuned model or a prompt-injection impostor? How do
> you attribute a claim to a specific agent session running on
> a specific model version?"
>
> "Today there are no cryptographic answers to any of these. I
> want to show you how to fix that with Quidnug."

*Architecture frame: a chaotic graph of agents calling each
other with question marks over the edges. Dissolves to a clean
signed-trust-graph at the end.*

### [0:40–1:30] The mental model

> "In Quidnug, every principal is a **quid** — a cryptographic
> identity with an ECDSA P-256 keypair."
>
> "For AI workflows, we give quids to four levels:"
>
> "**Level one — the foundation model**. Anthropic's Claude
> checkpoint. OpenAI's GPT-4o checkpoint. Google's Gemini.
> The **model weights** get a quid."
>
> "**Level two — the deploying org**. Your lab, your startup,
> your team. The org **vouches for the specific model
> instances** it runs."
>
> "**Level three — the agent**. A specific agent session. Short-
> lived — could be minutes, could be hours. Gets a fresh
> keypair."
>
> "**Level four — every action the agent takes**. Every LLM
> call, tool invocation, retrieved document. Gets recorded as
> a **signed event** on the agent's stream."

*Architecture frame: a 4-layer stack appearing in sequence,
bottom-up, with arrows labeled "trust" pointing up.*

### [1:30–3:30] Show the code

*[cut to editor]*

> "Here's the minimal end-to-end flow in Python — this is the
> runnable example at `examples/ai-agents/agent_identity.py`
> in the repo."

```python
from quidnug import Quid, QuidnugClient
import hashlib
import time

client = QuidnugClient("http://localhost:8080")
domain = "ai.agents"

lab = Quid.generate()        # your lab / company
model = Quid.generate()      # the Claude 3.5 checkpoint
agent = Quid.generate()      # this agent session
consumer = Quid.generate()   # downstream app

# Register
client.register_identity(lab, name="Acme AI Lab", home_domain=domain)
client.register_identity(model, name="Claude 3.5 Sonnet",
                         attributes={"checkpoint": "2024-10"})
client.register_identity(agent, name="assistant-session-42")
client.register_identity(consumer, name="Skeptical Consumer")

# Lab vouches for the model checkpoint
client.grant_trust(lab, trustee=model.id, level=1.0, domain=domain)

# Model "owns" the agent session
client.grant_trust(model, trustee=agent.id, level=0.95, domain=domain)

# Consumer has evaluated the lab directly
client.grant_trust(consumer, trustee=lab.id, level=0.9, domain=domain)
```

> "So far so good. The consumer has direct trust in the lab.
> The lab vouches for the model. The model vouches for this
> agent session. We've built a signed chain."

### [3:30–5:00] Record an LLM call

> "Now the agent makes an actual LLM call. Let's record it as
> a signed event."

```python
system_prompt = "You are a helpful assistant."
user_input = "What's the capital of France?"
model_output = "Paris."

receipt = client.emit_event(
    agent,
    subject_id=agent.id,
    subject_type="QUID",
    event_type="LLM_INFERENCE",
    payload={
        "model": "claude-3.5-sonnet",
        "systemPromptDigest": sha256_hex(system_prompt),
        "inputDigest": sha256_hex(user_input),
        "outputDigest": sha256_hex(model_output),
        "temperature": 0.7,
        "tokensIn": 147,
        "tokensOut": 12,
    },
)
```

> "A couple of things to notice."
>
> "One — we record **digests**, not raw text. Inputs and
> outputs are often sensitive; a SHA-256 is enough for
> tamper-evidence without leaking PII."
>
> "Two — this event is **signed by the agent's key**. The
> agent's key was issued by the model, which was issued by
> the lab. The signature chain is intact."
>
> "Three — the event goes onto the **agent's stream**. It's
> append-only and protected by the nonce ledger. You can't
> go back and rewrite history."

### [5:00–6:00] Downstream verification

> "Now the consumer wants to check: 'should I trust the
> answer this agent gave?'"

```python
tr = client.get_trust(consumer.id, agent.id, domain=domain, max_depth=5)
print(f"trust level: {tr.trust_level:.3f}")
print(f"path: {' -> '.join(tr.path)}")

if tr.trust_level >= 0.7:
    print("✓ accept the output")
else:
    print("✗ reject")
```

*[show output]*

```
trust level: 0.855
path: consumer -> lab -> model -> agent
```

> "Consumer trusts lab at 0.9. Lab trusts model at 1.0. Model
> trusts agent at 0.95. Transitive: 0.9 × 1.0 × 0.95 = 0.855.
> Above the 0.7 threshold, so we accept."
>
> "If the lab got compromised tomorrow and we downgraded our
> trust in it to 0.5, every agent in the chain would drop
> below threshold automatically. No whitelist editing, no
> policy-file PR. The graph does the work."

### [6:00–7:00] Multi-agent audit

> "In a multi-agent system — agent A calling agent B calling
> agent C — each call is one more event on the caller's
> stream with the callee's quid ID in the payload."
>
> "An auditor can walk every stream and reconstruct the full
> call graph. Every edge is cryptographically signed. Every
> signer is scorable via relational trust."

*Architecture frame: animated multi-agent call graph, edges
fading in with signatures.*

> "This is fundamentally different from observability-layer
> tools like LangSmith or Weights & Biases. Those are
> **cooperative** — the agent has to log honestly. Quidnug is
> **cryptographic** — the agent can't claim it did something it
> didn't, because the claim has to be signed, and the recipient
> verifies the signature."

### [7:00–7:40] Where to use this

> "Concrete fits today:"
>
> "- **RAG pipelines** — each retrieved source is a citation
>   attached to the event. Consumers filter by relational
>   trust."
>
> "- **Multi-agent coordination** — every tool call is an
>   audit trail."
>
> "- **Model-checkpoint attribution** — prove your output came
>   from a specific fine-tuned weights blob."
>
> "- **Agent marketplaces** — agents publish their Quidnug
>   identity and users grant per-observer trust."

### [7:40–8:00] Outro

> "Code is at `examples/ai-agents/` in the repo. Full working
> Python script, walks you through exactly what I just showed."
>
> "Apache-2.0. No gatekeeper. No token. No VC behind us.
> github.com/bhmortim/quidnug."

---

## Production notes

- **Architecture frames**: build 3 distinct animated diagrams
  — the 4-layer stack, the call-graph reconstruction, and the
  before/after "chaos graph vs signed graph."
- **Code pacing**: don't type — paste, then explain. Viewers
  will pause if they want to copy.
- **Terminal font**: 20px minimum. Zoom on key moments.
- **Include visible trust-level decay animation** around 5:30
  — the multiplicative effect is the "aha" for this audience.

## License

Apache-2.0.
