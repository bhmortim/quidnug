# AI agent identity + attribution on Quidnug

A demonstration of how AI agents — LLM chains, autonomous workers,
RAG pipelines — can use Quidnug to establish cryptographic identity,
attribute outputs to specific models, and let downstream consumers
score outputs by **the relational trust they have in the agent's
creator**.

## The problem

AI outputs today have no verifiable provenance. A chatbot can
claim to be "ChatGPT-4o" or "Claude 3.5 Sonnet" but there's no
cryptographic proof. A RAG system can cite sources but the
citations are unsigned. Agents calling each other in multi-agent
workflows have no way to prove which agent said what, when.

## The shape of the solution

Every agent is a Quidnug Quid:

1. **Each agent gets a P-256 keypair** on first launch. The quid
   ID is the agent's cryptographic identity.
2. **The agent's *creator* issues a trust edge** to the agent in
   the "ai.agents" domain (or similar). This binds the agent's
   identity to a human / org who vouches for it.
3. **Every LLM call / tool invocation** becomes an EVENT on the
   agent's stream: input, output digest, model name, temperature,
   system prompt hash.
4. **Downstream consumers query relational trust** from their own
   quid to the agent's quid. If the agent's chain of vouchers
   transitively reaches them at ≥ threshold, they accept the
   output.

## Runnable example

See [`agent_identity.py`](agent_identity.py) for a full runnable
flow:

- Create the "lab" quid (your org).
- Create the "model" quid (the Claude instance).
- Create the "agent" quid (a specific agent session).
- Build the trust chain: `lab → model → agent`.
- Run a fake LLM call, emit the input+output as a signed EVENT.
- Query relational trust from a skeptical consumer.

```bash
cd examples/ai-agents
python agent_identity.py
```

## Event schema

```json
{
  "type": "EVENT",
  "subjectId": "<agent-quid-id>",
  "subjectType": "QUID",
  "eventType": "LLM_INFERENCE",
  "payload": {
    "model": "claude-3.5-sonnet",
    "inputDigest": "sha256:...",
    "outputDigest": "sha256:...",
    "systemPromptDigest": "sha256:...",
    "temperature": 0.7,
    "maxTokens": 2048,
    "startedAt": 1700000000,
    "endedAt": 1700000001,
    "tokensIn": 147,
    "tokensOut": 982
  }
}
```

Digests are preferred over raw input/output bodies (payload size
+ PII) — pin full inputs/outputs to IPFS and reference via
`payloadCid`.

## Trust model

Typical chain:

```
  Anthropic (foundation model provider)
     │ trust 1.0 (they made the model)
     ▼
  "Claude 3.5 Sonnet"  (model quid)
     │ trust 1.0 (Anthropic vouches for this weights checkpoint)
     ▼
  lab.example.com  (deploying org)
     │ trust 0.95 (lab configured + deployed)
     ▼
  assistant-session-42  (agent quid)
     │
     ▼
  individual LLM inference EVENT
```

A downstream consumer at trust 0.9 → lab.example.com gets
transitive trust 0.9 × 0.95 = 0.855 to the agent, and ultimately
to every event on the agent's stream. If they require ≥ 0.7 to
accept an output, the chain passes.

## Multi-agent workflows

For agent-to-agent calls, each caller-callee pair becomes its own
EVENT:

```
agent-A → tool call → agent-B
  → emit AGENT_CALL event on agent-A's stream
      with agent-B's quid ID in payload
```

An auditor walking the stream reconstructs the call graph and can
query relational trust at every hop.

## Attribution beyond LLMs

The same pattern applies to:

- **Classification models**: agent quid = model instance, EVENT
  per classification with confidence + input hash.
- **RAG pipelines**: each retrieved source cited in the EVENT
  payload as `(sourceUri, sourceSigner, confidence)`.
- **Fine-tuning datasets**: each training pair becomes a Quidnug
  Title, training metrics become EVENTs on the model quid.

## Why Quidnug vs. plain Sigstore?

Sigstore proves "this artifact was signed by identity X". Quidnug
adds the **per-observer trust relation to X**. A consumer who
trusts Anthropic at 1.0 but has never heard of `lab.example.com`
can still compute a trust score via the transitive path. Sigstore
alone would make them fall back to binary "do I trust this
Fulcio-issued cert?", which is frequently "no".

## License

Apache-2.0.
