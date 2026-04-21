"""AI agent capability authorization, end-to-end against a live node.

Flow:
  1. Register actors: principal (the business owner), the agent,
     an audit bot, a safety committee.
  2. Principal grants a time-bounded, domain-scoped capability
     to the agent: "spend up to $10k in money.acme for 30 days."
  3. Agent proposes a trivial-class action ($50 spend); agent
     self-authorizes and executes. Demonstrates the fast path.
  4. Agent proposes a medium-class action ($2500 wire);
     audit-bot + principal cosign; threshold met; authorized.
  5. Agent proposes a high-value action ($25k wire); safety
     committee issues a veto event. Authorization fails.
  6. Agent proposes an action outside its granted domain;
     denied by policy.
  7. Emergency revocation: anchor-invalidate the agent's key
     epoch. Subsequent action proposals can't be cosigned.

Prerequisites:
  - Local Quidnug node at http://localhost:8080.
  - Python SDK installed: `pip install -e clients/python`.

Run:
    python demo.py
"""

from __future__ import annotations

import os
import sys
import time
import uuid
from dataclasses import dataclass
from typing import Dict, List

sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))
from agent_authz import (
    AgentAction,
    CapabilityGrant,
    Cosignature,
    GuardianSet,
    GuardianWeight,
    VERDICT_AUTHORIZED,
    Veto,
    evaluate_authorization,
    extract_cosignatures,
    extract_vetoes,
)

from quidnug import Quid, QuidnugClient

NODE_URL = os.environ.get("QUIDNUG_NODE", "http://localhost:8080")
DOMAIN = "ai.agents.finance"
SPEND_DOMAIN = "money.acme.company-spending"


@dataclass
class Actor:
    name: str
    role: str
    quid: Quid


def banner(msg: str) -> None:
    print()
    print("=" * 72)
    print(f"  {msg}")
    print("=" * 72)


def register(client: QuidnugClient, name: str, role: str) -> Actor:
    q = Quid.generate()
    try:
        client.register_identity(
            q, name=name, domain=DOMAIN, home_domain=DOMAIN,
            attributes={"role": role},
        )
    except Exception as e:
        print(f"  (register {name}: {e})")
    return Actor(name=name, role=role, quid=q)


def propose_action(
    client: QuidnugClient, agent: Actor, action: AgentAction,
) -> str:
    client.emit_event(
        signer=agent.quid,
        subject_id=agent.quid.id,
        subject_type="QUID",
        event_type="agent.action.proposed",
        domain=DOMAIN,
        payload={
            "actionId": action.action_id,
            "actionType": action.action_type,
            "amountCents": action.amount_cents,
            "riskClass": action.risk_class,
            "domain": action.domain,
            "target": action.target,
            "reason": action.reason,
            "proposedAt": action.proposed_at_unix,
        },
    )
    return action.action_id


def cosign(
    client: QuidnugClient, agent: Actor, guardian: Actor, action_id: str,
) -> None:
    client.emit_event(
        signer=guardian.quid,
        subject_id=agent.quid.id,   # event lands on the agent's stream
        subject_type="QUID",
        event_type="agent.action.cosigned",
        domain=DOMAIN,
        payload={
            "signerQuid": guardian.quid.id,
            "cosigns": action_id,
            "cosignedAt": int(time.time()),
        },
    )


def veto(
    client: QuidnugClient, agent: Actor, guardian: Actor,
    action_id: str, reason: str,
) -> None:
    client.emit_event(
        signer=guardian.quid,
        subject_id=agent.quid.id,
        subject_type="QUID",
        event_type="agent.action.vetoed",
        domain=DOMAIN,
        payload={
            "signerQuid": guardian.quid.id,
            "vetoes": action_id,
            "reason": reason,
            "vetoedAt": int(time.time()),
        },
    )


def agent_stream_as_dicts(client: QuidnugClient, agent: Actor) -> List[dict]:
    """Pull the agent's stream and normalize Events back to dicts
    so we can reuse the extract_* helpers."""
    events, _ = client.get_stream_events(agent.quid.id, limit=200)
    out: List[dict] = []
    for ev in events or []:
        out.append({
            "eventType": ev.event_type,
            "payload": ev.payload or {},
            "timestamp": ev.timestamp,
            "sequence": ev.sequence,
        })
    return out


def show_verdict(label: str, action: AgentAction, decision) -> None:
    print(f"\n  [{label}]")
    print(f"    action={action.action_id} risk={action.risk_class} "
          f"amt=${action.amount_cents/100:.2f} domain={action.domain}")
    print(f"    verdict = {decision.verdict.upper()} "
          f"(weight {decision.collected_weight}/{decision.required_weight})")
    for r in decision.reasons:
        print(f"      - {r}")


def main() -> None:
    print(f"Connecting to Quidnug node at {NODE_URL}")
    client = QuidnugClient(NODE_URL)
    try:
        client.info()
    except Exception as e:
        print(f"node unreachable: {e}", file=sys.stderr)
        sys.exit(1)

    # -----------------------------------------------------------------
    banner("Step 1: Register actors")
    principal = register(client, "acme-ceo",          "principal")
    agent     = register(client, "acme-finance-bot",  "agent")
    audit_bot = register(client, "acme-audit-bot",    "audit-bot")
    safety    = register(client, "acme-safety-cmte",  "safety-committee")
    for a in (principal, agent, audit_bot, safety):
        print(f"  {a.role:18s} {a.name:20s} -> {a.quid.id}")

    # Guardian set for this agent: principal (w=1), safety (w=2),
    # audit-bot (w=1). threshold 2.
    guardian_set = GuardianSet(
        agent_quid=agent.quid.id,
        members=[
            GuardianWeight(principal.quid.id, 1, "principal"),
            GuardianWeight(safety.quid.id,    2, "safety-committee"),
            GuardianWeight(audit_bot.quid.id, 1, "audit-bot"),
        ],
        threshold=2,
    )

    # -----------------------------------------------------------------
    banner("Step 2: Principal grants capability to agent")
    valid_until = int(time.time()) + 30 * 86400
    client.grant_trust(
        signer=principal.quid,
        trustee=agent.quid.id,
        level=0.8,
        domain=SPEND_DOMAIN,
        valid_until=valid_until,
        description="up to $10k/mo spend authorization",
    )
    # Mirror the grant locally so the authz logic can evaluate.
    # (In a fuller deployment, a helper would read the on-chain
    # trust-edge record and re-materialize this.)
    grant = CapabilityGrant(
        truster_quid=principal.quid.id,
        agent_quid=agent.quid.id,
        domain=SPEND_DOMAIN,
        max_amount_cents=10_000_00,
        valid_until_unix=valid_until,
        description="up to $10k/mo spend authorization",
    )
    print(f"  principal -> agent  domain={SPEND_DOMAIN}")
    print(f"                      max=$10000  valid for 30 days")

    time.sleep(1)

    # -----------------------------------------------------------------
    banner("Step 3: TRIVIAL action  ($50 test transfer)")
    a1 = AgentAction(
        action_id=f"act-{uuid.uuid4().hex[:8]}",
        agent_quid=agent.quid.id,
        domain=SPEND_DOMAIN,
        action_type="wire.send",
        amount_cents=50_00,
        risk_class="trivial",
        proposed_at_unix=int(time.time()),
        target="contractor-x",
        reason="SaaS subscription",
    )
    propose_action(client, agent, a1)
    time.sleep(0.5)
    events = agent_stream_as_dicts(client, agent)
    d1 = evaluate_authorization(
        a1, [grant], guardian_set,
        extract_cosignatures(events), extract_vetoes(events),
        now_unix=int(time.time()),
    )
    show_verdict("TRIVIAL: agent self-authorizes", a1, d1)

    # -----------------------------------------------------------------
    banner("Step 4: MEDIUM action  ($2500 wire; cosign flow)")
    a2 = AgentAction(
        action_id=f"act-{uuid.uuid4().hex[:8]}",
        agent_quid=agent.quid.id,
        domain=SPEND_DOMAIN,
        action_type="wire.send",
        amount_cents=2_500_00,
        risk_class="medium",
        proposed_at_unix=int(time.time()),
        target="vendor-abc",
        reason="quarterly payment",
    )
    propose_action(client, agent, a2)
    time.sleep(0.5)

    # First pass: no cosignatures yet.
    events = agent_stream_as_dicts(client, agent)
    d2a = evaluate_authorization(
        a2, [grant], guardian_set,
        extract_cosignatures(events), extract_vetoes(events),
        now_unix=int(time.time()),
    )
    show_verdict("MEDIUM pre-cosign", a2, d2a)

    # Cosigners append.
    cosign(client, agent, audit_bot, a2.action_id)
    time.sleep(0.3)
    cosign(client, agent, principal, a2.action_id)
    time.sleep(0.5)

    events = agent_stream_as_dicts(client, agent)
    d2b = evaluate_authorization(
        a2, [grant], guardian_set,
        extract_cosignatures(events), extract_vetoes(events),
        now_unix=int(time.time()),
    )
    show_verdict("MEDIUM post-cosign", a2, d2b)
    assert d2b.verdict == VERDICT_AUTHORIZED, "expected authorization"

    # -----------------------------------------------------------------
    banner("Step 5: HIGH action vetoed by safety committee")
    a3 = AgentAction(
        action_id=f"act-{uuid.uuid4().hex[:8]}",
        agent_quid=agent.quid.id,
        domain=SPEND_DOMAIN,
        action_type="wire.send",
        amount_cents=8_000_00,
        risk_class="high",
        proposed_at_unix=int(time.time()),
        target="new-vendor-ltd",
        reason="large purchase order",
    )
    propose_action(client, agent, a3)
    time.sleep(0.3)
    # Safety committee publishes a veto event before cosigners can sign.
    veto(client, agent, safety, a3.action_id,
         "anomalous pattern: vendor not previously seen")
    time.sleep(0.5)

    events = agent_stream_as_dicts(client, agent)
    d3 = evaluate_authorization(
        a3, [grant], guardian_set,
        extract_cosignatures(events), extract_vetoes(events),
        now_unix=int(time.time()),
    )
    show_verdict("HIGH vetoed", a3, d3)

    # -----------------------------------------------------------------
    banner("Step 6: Out-of-domain action denied by policy")
    a4 = AgentAction(
        action_id=f"act-{uuid.uuid4().hex[:8]}",
        agent_quid=agent.quid.id,
        # agent has no grant in the code-commit domain.
        domain="code.acme-backend",
        action_type="code.commit",
        amount_cents=0,
        risk_class="medium",
        proposed_at_unix=int(time.time()),
        target="main branch",
        reason="dependency bump",
    )
    propose_action(client, agent, a4)
    time.sleep(0.3)
    events = agent_stream_as_dicts(client, agent)
    d4 = evaluate_authorization(
        a4, [grant], guardian_set,
        extract_cosignatures(events), extract_vetoes(events),
        now_unix=int(time.time()),
    )
    show_verdict("OUT-OF-DOMAIN denied", a4, d4)

    # -----------------------------------------------------------------
    banner("Step 7: Expired grant scenario (simulated)")
    # Simulate what happens after 31 days by feeding a past valid_until.
    expired_grant = CapabilityGrant(
        truster_quid=principal.quid.id,
        agent_quid=agent.quid.id,
        domain=SPEND_DOMAIN,
        max_amount_cents=10_000_00,
        valid_until_unix=int(time.time()) - 3600,
        description="expired grant",
    )
    a5 = AgentAction(
        action_id=f"act-{uuid.uuid4().hex[:8]}",
        agent_quid=agent.quid.id,
        domain=SPEND_DOMAIN,
        action_type="wire.send",
        amount_cents=100_00,
        risk_class="trivial",
        proposed_at_unix=int(time.time()),
    )
    d5 = evaluate_authorization(
        a5, [expired_grant], guardian_set,
        cosignatures=[], vetoes=[],
        now_unix=int(time.time()),
    )
    show_verdict("EXPIRED GRANT denied", a5, d5)

    # -----------------------------------------------------------------
    banner("Demo complete")
    print()
    print("Insights:")
    print(" - Risk-class routing drives the cosignature requirement:")
    print("   trivial self-signed, medium meets threshold, high vetoable.")
    print(" - Every action and every cosignature is a signed event on")
    print("   the agent's own stream. The audit trail is the stream.")
    print(" - Vetoes win over cosignatures. A single guardian can stop")
    print("   an action even if all others cosigned.")
    print(" - Time-bounded grants expire without renewal. No revoke call")
    print("   needed: the trust edge's validUntil handles it.")
    print(" - Emergency kill would be an AnchorInvalidation on the agent's")
    print("   key epoch; the node would then reject any further")
    print("   signatures from the old epoch.")
    print()


if __name__ == "__main__":
    main()
