"""AI agent identity + attribution on Quidnug.

Full runnable walkthrough. Assumes a local node at localhost:8080
— start one via `cd deploy/compose && docker compose up -d`.

    python examples/ai-agents/agent_identity.py
"""

from __future__ import annotations

import hashlib
import time
from pathlib import Path

from quidnug import Quid, QuidnugClient


def sha256_hex(data: str) -> str:
    return "sha256:" + hashlib.sha256(data.encode("utf-8")).hexdigest()


def main() -> None:
    client = QuidnugClient("http://localhost:8080")
    domain = "ai.agents"

    # --- Actors -----------------------------------------------------------
    # In production, each of these quids lives in a different place:
    # - The lab quid's private key lives in the lab's HSM.
    # - The model quid's private key lives in the lab's signing service.
    # - The agent quid's private key is generated per-session and
    #   discarded when the session ends.
    lab = Quid.generate()
    model = Quid.generate()
    agent = Quid.generate()
    consumer = Quid.generate()
    print(f"lab      = {lab.id}")
    print(f"model    = {model.id}")
    print(f"agent    = {agent.id}")
    print(f"consumer = {consumer.id}\n")

    # --- Register identities ---------------------------------------------
    client.register_identity(lab, name="Example AI Lab",
                             home_domain=domain,
                             attributes={"org_type": "research_lab"})
    client.register_identity(model, name="Claude 3.5 Sonnet",
                             home_domain=domain,
                             attributes={
                                 "model_family": "claude",
                                 "version": "3.5-sonnet",
                                 "training_checkpoint": "2024-10",
                             })
    client.register_identity(agent, name="assistant-session-42",
                             home_domain=domain,
                             attributes={"session_started": int(time.time())})
    client.register_identity(consumer, name="Skeptical Consumer",
                             home_domain=domain)

    # --- Trust chain ------------------------------------------------------
    # Lab vouches for the specific model weights checkpoint.
    client.grant_trust(lab, trustee=model.id, level=1.0, domain=domain,
                       description="lab-deployed model checkpoint")

    # Model "vouches" for the agent session (equivalent to: "this
    # agent session was started with the model's API key").
    client.grant_trust(model, trustee=agent.id, level=0.95, domain=domain,
                       description="agent session using this model")

    # Consumer trusts the lab at 0.9 (they've evaluated it directly).
    client.grant_trust(consumer, trustee=lab.id, level=0.9, domain=domain,
                       description="evaluated lab's safety practices")

    print("Trust chain installed.\n")

    # --- Fake LLM call, recorded as a signed EVENT -----------------------
    system_prompt = "You are a helpful assistant that answers concisely."
    user_input = "What's the capital of France?"
    model_output = "Paris."

    receipt = client.emit_event(
        agent,
        subject_id=agent.id,
        subject_type="QUID",
        event_type="LLM_INFERENCE",
        domain=domain,
        payload={
            "model": "claude-3.5-sonnet",
            "systemPromptDigest": sha256_hex(system_prompt),
            "inputDigest": sha256_hex(user_input),
            "outputDigest": sha256_hex(model_output),
            "temperature": 0.7,
            "maxTokens": 2048,
            "startedAt": int(time.time()),
            "tokensIn": 147,
            "tokensOut": 12,
        },
    )
    print(f"Emitted EVENT: tx_id={receipt.get('id')} seq={receipt.get('sequence')}\n")

    # --- The consumer computes its trust in the agent's output -----------
    tr = client.get_trust(consumer.id, agent.id, domain=domain, max_depth=5)
    print(f"Consumer trust in agent:")
    print(f"  level = {tr.trust_level:.3f}")
    print(f"  path  = {' -> '.join(tr.path) or 'no path'}")
    print(f"  depth = {tr.path_depth}")

    threshold = 0.7
    if tr.trust_level >= threshold:
        print(f"\n✓ agent output acceptable at threshold {threshold}")
    else:
        print(f"\n✗ agent output below threshold {threshold}; reject")

    # --- Audit: pull the full history of this agent's activity -----------
    print("\nAgent stream:")
    events, _ = client.get_stream_events(agent.id, domain=domain, limit=10)
    for e in events:
        print(f"  #{e.sequence} {e.event_type} @ {e.timestamp}: "
              f"model={e.payload.get('model', '?')}")


if __name__ == "__main__":
    main()
