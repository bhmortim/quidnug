# AI Agent Capability Authorization

**AI · Agent safety · Time-locked grants · Emergency revocation**

## The problem

Autonomous AI agents are stepping into real-world responsibility:
- Writing code that gets merged and deployed
- Executing financial transactions
- Reading/writing sensitive data (medical, financial, personal)
- Operating on behalf of users in ways that affect third parties

The current approach is OAuth-style scopes and API keys. Problems:

1. **Binary grants.** "Agent can read email" or not. No
   "Agent can read emails matching pattern X but not Y, and
   only for the next 4 hours."
2. **No oversight.** Once a scope is granted, the agent uses it
   autonomously. No "someone must approve anything above $500."
3. **Revocation is slow.** If an agent misbehaves, revoking its
   token across many systems is a scramble.
4. **No accountability trail.** "Why did the agent do that?
   Who authorized this?" — only as good as each system's
   audit log.
5. **Multi-party authorization is ad hoc.** A contract-signing
   agent needs authorization from the user + legal review;
   today that's email ping-pong.

This gets much worse as agents become more autonomous. An
AI agent representing a small business might need to:
- Spend $X/month autonomously
- Defer to the business owner for anything above $X
- Defer to a legal committee for contracts
- Be immediately killable if it goes rogue

The protocol need is **cryptographically verifiable,
time-bounded, multi-party-approvable, revocable capability
grants**.

## Why Quidnug fits

The agent is a quid. Its capabilities are encoded in the
quid's guardian set + trust edges. Revocation and amendment
use guardian-set updates.

| Problem                                     | Quidnug primitive                              |
|---------------------------------------------|------------------------------------------------|
| "What can this agent do?"                   | Guardian set structure + trust edges           |
| "Emergency kill the agent"                  | AnchorInvalidation on agent's key epoch        |
| "Approve this specific high-value action"   | M-of-N cosigning via agent's stakeholders       |
| "Time-bound the grant"                      | `EffectiveAt` / `validUntil` on trust edges    |
| "Revoke without agent cooperation"          | `GuardianResignation` (QDP-0006)               |
| "Agent's key compromised"                   | Guardian-recovery rotates to new key           |
| "Prove the grant was authorized"            | Signed on-chain guardian-set installation      |

## High-level architecture

```
                   Agent quid ("agent-acme-finance-001")
                              │
                              │ has GuardianSet:
                              │   guardians: [owner, safety-committee, audit-bot]
                              │   threshold: 2
                              │   requireGuardianRotation: true
                              │
                              ▼
              ┌───────────────────────────────────────┐
              │   Capability grants as trust edges   │
              │                                        │
              │  agent → money.company-spending.acme  │
              │    trustLevel: limited                 │
              │    validUntil: <now + 30d>            │
              │                                        │
              │  agent → code.repo.acme-backend        │
              │    trustLevel: write                   │
              │    validUntil: <now + 7d>             │
              └───────────────────────────────────────┘
                              │
                              │ each action is a signed tx
                              │ + events in action's stream
                              ▼
              ┌───────────────────────────────────────┐
              │   Agent operates within scope;        │
              │   high-value actions cosigned via     │
              │   guardian quorum                      │
              └───────────────────────────────────────┘
```

## Data model

### Quids
- **Agent**: one quid per AI agent instance. HSM-backed or
  software-key (depending on sensitivity).
- **Principal**: the user or organization the agent represents.
- **Safety committee**: human quorum for high-value actions.
- **Audit bot**: automated oversight (monitors event streams
  for anomalies, cosigns if conditions met).

### Domain
```
ai.agents                              (root)
├── ai.agents.finance-trading
├── ai.agents.code-writing
├── ai.agents.customer-support
└── ai.agents.research
```

### Agent's guardian set = its capability committee

Unlike interbank wire (where the guardian set is "who can
authorize on behalf of the bank"), here the agent's guardian
set is "who must cosign anything the agent does above
threshold T."

```
GuardianSet for "agent-acme-finance-001":
  guardians:
    - principal-acme-business-owner  (weight: 1, epoch: 0)
    - safety-committee-acme          (weight: 2, epoch: 0)
    - audit-bot-acme                 (weight: 1, epoch: 0)
  threshold: 2
  recoveryDelay: 1h
  requireGuardianRotation: true
```

- Owner (w=1) alone → insufficient for high-value actions.
- Owner + audit bot (w=2) → meets threshold.
- Safety committee (w=2) → can approve alone
  (for emergency/auditor-only actions).

### Action events as authorization records

Every action the agent takes is an event. Low-value actions
the agent signs alone. High-value actions require cosignatures.

```
eventType: "agent.action.proposed"
subjectId: <agent quid>
payload:
  actionType: "wire.send"
  target: "contractor-x"
  amount: "15000"
  currency: "USD"
  reason: "monthly retainer per contract"
  riskClass: "low-routine"
signer: agent (self-signed for routine)

eventType: "agent.action.proposed"
payload:
  actionType: "contract.sign"
  target: "new-vendor-ltd"
  amount: "150000"
  terms: "annual service agreement"
  riskClass: "high"
signer: agent

eventType: "agent.action.cosigned"
payload:
  signerQuid: "principal-acme-business-owner"
  cosigns: "<event ID of proposal>"
signer: principal-acme-business-owner

eventType: "agent.action.cosigned"
payload:
  signerQuid: "safety-committee-acme"
  cosigns: "<event ID of proposal>"
signer: safety-committee-acme
```

Once threshold is met (weight ≥ 2), system emits:

```
eventType: "agent.action.authorized"
payload:
  originalProposal: "<event ID>"
  totalWeight: 3
  threshold: 2
```

Agent observes `agent.action.authorized` on its own stream
and proceeds to execute the action externally (send the wire,
sign the contract, commit the code).

### Risk classes → routing

Agent's risk-classifier (part of the agent's logic) assigns
each proposed action to a class:

| Risk class         | Authorization                       |
|--------------------|-------------------------------------|
| `trivial`          | Agent self-signs; no cosigners      |
| `low-routine`      | Agent + 1 cosigner (audit bot auto) |
| `medium`           | Agent + threshold-weight cosigners  |
| `high`             | Full quorum + safety-committee veto |
| `emergency`        | Safety committee alone (weight 2)   |

## Time-locked grants

A capability grant can be time-bounded via the trust edge's
`validUntil`:

```bash
curl -X POST $NODE/api/trust -d '{
  "truster":"principal-acme-business-owner",
  "trustee":"agent-acme-finance-001",
  "trustLevel":0.8,
  "domain":"money.company-spending.acme",
  "nonce":1,
  "validUntil":<now + 30d>,
  "description":"Monthly spending up to $10k/mo"
}'
```

After 30 days, the edge expires; the agent's trust drops. If
the edge isn't renewed, the agent can no longer authorize
spending in that domain.

## Emergency revocation

Agent has gone off the rails. Immediate response:

### Option 1: Invalidate the agent's epoch
```bash
curl -X POST $NODE/api/anchors -d '{
  "kind":"invalidation",
  "signerQuid":"agent-acme-finance-001",
  "epochToInvalidate":0,
  "anchorNonce":<next>,
  "validFrom":<now>,
  "signature":"<from principal or safety committee>"
}'
```

All future actions from the agent fail signature verification.
The agent is effectively frozen.

### Option 2: Guardian-set update that removes authority
```bash
curl -X POST $NODE/api/v2/guardian/set-update -d '{
  "subjectQuid":"agent-acme-finance-001",
  "newSet":{
    "guardians":[],  /* or single quorum */
    "threshold":1
  },
  ...
}'
```

Effectively orphans the agent — no cosigners available for
high-value actions.

### Option 3: Safety-committee override (via guardian veto)

If the agent proposes a high-value action, safety committee
can submit an `agent.action.vetoed` event:

```
eventType: "agent.action.vetoed"
payload:
  vetoes: "<event ID of proposal>"
  reason: "anomalous-behavior"
signer: safety-committee-acme
```

Agent listens for vetoes on its own stream and aborts.

## Principle of least privilege

Agent's capabilities are tightly scoped:
- **Per-domain trust edges**: `money.company-spending.acme`
  ≠ `code.acme-backend`. Agent trusted in one doesn't
  cross-grant the other.
- **Per-action risk classification**: trivial actions
  self-executed; high-risk fully cosigned.
- **Time-bounded**: edges expire.

## Key Quidnug features

- **Guardian set as capability committee** — cosigning
  quorum for high-value actions.
- **Time-locked veto** (QDP-0002) — safety committee has
  a delay window to revoke before an action commits.
- **`requireGuardianRotation=true`** — agent can't
  self-rotate its own authorization structure.
- **GuardianResignation (QDP-0006)** — a cosigner who no
  longer wants to be on the hook can resign.
- **Event streams** — full audit trail.
- **AnchorInvalidation** — emergency kill-switch.
- **Trust edge `validUntil`** — time-bounded grants.

## Value delivered

| Dimension                                | Before                                    | With Quidnug                                            |
|------------------------------------------|-------------------------------------------|---------------------------------------------------------|
| Grant specificity                        | OAuth scope booleans                       | Typed trust edges per domain                              |
| Time-bounding                            | Token expiry only                          | Trust-edge `validUntil`, renewal required                 |
| Multi-party authorization for high-value | Custom app code or nothing                | Guardian-set quorum (cryptographic)                       |
| Emergency revocation                     | Revoke tokens across N systems             | Single AnchorInvalidation, entire network observes        |
| Accountability trail                     | Per-service audit log (fragmented)         | Single signed event stream                                |
| Safety committee oversight               | Rarely implemented                         | First-class: time-locked veto window                      |
| Capability delegation                    | Agent → sub-agent is a mess                | Trust edge from parent agent to sub-agent                 |
| Compromise recovery                      | Revoke + re-provision (downtime)           | Guardian recovery to new key                              |

## What's in this folder

- [`README.md`](README.md) — this document
- [`implementation.md`](implementation.md) — API calls
- [`threat-model.md`](threat-model.md) — security analysis

## Runnable POC

Full end-to-end demo at
[`examples/ai-agent-authorization/`](../../examples/ai-agent-authorization/):

- `agent_authz.py` — pure decision logic (risk class → required
  cosigner weight, grant coverage, veto override).
- `agent_authz_test.py` — 17 pytest cases covering risk-class
  routing, vetoes, grant scoping, expiry, duplicates, and
  non-guardian cosignatures.
- `demo.py` — seven-step end-to-end flow against a live node:
  register actors, grant a time-bounded capability, propose
  trivial / medium / high-with-veto / out-of-domain / expired
  actions, and observe per-class verdicts.

```bash
cd examples/ai-agent-authorization
python demo.py
```

## Related

- [`../ai-model-provenance/`](../ai-model-provenance/) — authorization
  presumes a trusted model
- [`../institutional-custody/`](../institutional-custody/) — same
  M-of-N quorum pattern for high-value human actions
- [QDP-0002 Guardian Recovery](../../docs/design/0002-guardian-based-recovery.md)
- [QDP-0006 Guardian Resignation](../../docs/design/0006-guardian-resignation.md)
