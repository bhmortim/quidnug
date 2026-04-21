# AI agent authorization, POC demo

Runnable proof-of-concept for the
[`UseCases/ai-agent-authorization/`](../../UseCases/ai-agent-authorization/)
use case. Exercises cryptographically-verifiable, time-bounded,
multi-party-approvable capability grants for AI agents.

## What this POC proves

Four actors (principal, agent, audit bot, safety committee) over
two domains (`money.acme.company-spending`,
`code.acme-backend`). Key claims the demo verifies:

1. **Risk-class routing works.** A trivial action self-authorizes
   instantly; a medium action requires cosigner weight meeting the
   guardian-set threshold; a high action can be vetoed by any
   single guardian.
2. **Vetoes win over cosignatures.** A safety-committee veto
   event published on the agent's stream blocks an action even
   when cosigners would otherwise authorize it.
3. **Capability grants are scoped and time-bounded.** An action
   outside the granted domain is denied by local policy; an
   expired grant is ignored.
4. **The audit trail is the stream.** Proposals, cosignatures,
   and vetoes are all signed events on the agent's event
   stream, queryable by any observer with read access.

## What's in this folder

| File | Purpose |
|---|---|
| `agent_authz.py` | Pure decision logic. No SDK dep. Exports `AgentAction`, `CapabilityGrant`, `GuardianSet`, `evaluate_authorization`, etc. |
| `agent_authz_test.py` | 17 pytest cases covering risk-class routing, vetoes, grant scoping, expiry, duplicates, non-guardian ignores. |
| `demo.py` | End-to-end runnable against a live node. Seven steps exercising trivial / medium / high-with-veto / out-of-domain / expired scenarios. |

## Running

```bash
# 1. Start a local node.
cd deploy/compose && docker compose up -d

# 2. Install Python SDK.
cd clients/python && pip install -e .

# 3. Run the demo.
cd examples/ai-agent-authorization
python demo.py
```

## Testing without a live node

```bash
cd examples/ai-agent-authorization
python -m pytest agent_authz_test.py -v
```

## QDP catalog audit

| Feature | Purpose | Status |
|---|---|---|
| `IDENTITY` tx | Register principal, agent, audit bot, safety cmte | v1.0 |
| `TRUST` tx with `validUntil` | Time-bounded capability grant | v1.0 |
| `EVENT` tx streams | Proposals, cosignatures, vetoes on agent's stream | v1.0 |
| QDP-0002 guardian recovery | Agent key rotation after compromise | v1.0 (not exercised in demo but available) |
| QDP-0006 guardian resignation | Cosigner opts out of the committee | v1.0 (not exercised) |
| AnchorInvalidation | Emergency kill-switch on the agent's epoch | v1.0 (described in demo Step 7; not executed) |
| QDP-0019 decay | Stale grants fade without explicit expiry | Phase 1 landed; optional |

No protocol gaps. The risk-class policy is application-layer; the
node enforces signatures, epoch validity, and trust-edge freshness.

## What a production deployment would add

- **Automated risk classification.** The demo has a hardcoded
  `risk_class` on each action. A real deployment would have a
  classifier service (rule-based or model-based) that scores
  each proposal and chooses the class.
- **Guardian-side cosign daemon.** In the demo, cosignatures are
  scripted. Real guardians (humans or bots) would subscribe to
  the agent's stream and cosign proposals that match their
  policy, reject/veto those that don't.
- **Anchor-invalidation driven kill-switch.** Tie the
  principal's web UI's "kill agent now" button to a
  `POST /api/anchors` call that invalidates the agent's key
  epoch. After that point, signatures from the agent fail
  verification across the entire network within gossip
  propagation time.
- **Grant renewal UX.** Time-bounded grants are a feature, not
  a bug: they force periodic review. A production system would
  remind the principal a week before expiry, let them extend or
  terminate.
- **QDP-0024 private action logs** for confidentiality when
  the action stream contains sensitive data (trade positions,
  medical orders, etc.). The agent's stream would be
  group-encrypted with the principal + auditors as members.

## Related

- Use case: [`UseCases/ai-agent-authorization/`](../../UseCases/ai-agent-authorization/)
- Protocol: [QDP-0002 Guardian Recovery](../../docs/design/0002-guardian-based-recovery.md)
  for agent key rotation
- Related demo: [`examples/ai-agents/agent_identity.py`](../ai-agents/agent_identity.py)
  handles the attribution chain (lab -> model -> agent -> consumer)
  that complements the capability-authorization story here
- Related POC: [`examples/institutional-custody/`](../institutional-custody/)
  uses the same cosigning pattern for human-driven high-value actions
