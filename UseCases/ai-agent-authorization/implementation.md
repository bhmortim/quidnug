# Implementation: AI Agent Authorization

## 1. Agent identity and capability committee

```bash
# Create the agent's quid
curl -X POST $NODE/api/identities -d '{
  "quidId":"agent-acme-finance-001",
  "name":"Acme Finance AI Agent v1",
  "homeDomain":"ai.agents.finance-trading",
  "creator":"principal-acme-business-owner",
  "updateNonce":1,
  "attributes":{
    "agentType":"finance-ops",
    "modelRef":"model-acme-fine-tune-v2",
    "deploymentEnv":"prod-us-east",
    "initialOwner":"principal-acme-business-owner"
  }
}'

# Install the capability committee as guardian set
curl -X POST $NODE/api/v2/guardian/set-update -d '{
  "subjectQuid":"agent-acme-finance-001",
  "newSet":{
    "guardians":[
      {"quid":"principal-acme-business-owner","weight":1,"epoch":0},
      {"quid":"safety-committee-acme","weight":2,"epoch":0},
      {"quid":"audit-bot-acme","weight":1,"epoch":0}
    ],
    "threshold":2,
    "recoveryDelay":3600000000000,
    "requireGuardianRotation":true
  },
  "anchorNonce":1,
  "validFrom":<now>,
  "primarySignature":{"keyEpoch":0,"signature":"<from agent>"},
  "newGuardianConsents":[ /* each guardian signs */ ]
}'
```

## 2. Grant capabilities via trust edges

```bash
# Owner grants agent spending capability in a specific domain
curl -X POST $NODE/api/trust -d '{
  "truster":"principal-acme-business-owner",
  "trustee":"agent-acme-finance-001",
  "trustLevel":0.8,
  "domain":"money.company-spending.acme",
  "nonce":1,
  "validUntil":<now + 30d>,
  "description":"Monthly operating expenses up to $10k/mo"
}'

# Code-repo capability in a DIFFERENT domain
curl -X POST $NODE/api/trust -d '{
  "truster":"principal-acme-business-owner",
  "trustee":"agent-acme-finance-001",
  "trustLevel":0.6,
  "domain":"code.repo.acme-backend",
  "nonce":2,
  "validUntil":<now + 7d>,
  "description":"Can open PRs; cannot merge without human review"
}'
```

Note different `trustLevel` per domain — allows the agent to
have higher confidence in some contexts than others.

## 3. Agent proposes an action (self-signs routine actions)

Agent's internal logic evaluates action risk class:

```go
type ActionProposal struct {
    Type       string                 // "wire.send", "contract.sign", ...
    Target     string
    Amount     string
    Reason     string
    RiskClass  string
}

func (a *Agent) PropseAction(ctx context.Context, p ActionProposal) (string, error) {
    proposalID := uuid.NewString()
    event := map[string]interface{}{
        "subjectId":   a.quid,
        "subjectType": "QUID",
        "eventType":   "agent.action.proposed",
        "payload": map[string]interface{}{
            "proposalID": proposalID,
            "actionType": p.Type,
            "target":     p.Target,
            "amount":     p.Amount,
            "reason":     p.Reason,
            "riskClass":  p.RiskClass,
            "proposedAt": time.Now().Unix(),
        },
        "creator":   a.quid,
        "signature": a.sign(/* canonical */),
    }
    return proposalID, a.submitEvent(ctx, event)
}

func (a *Agent) ExecuteIfAuthorized(ctx context.Context, proposalID string) error {
    // Poll for agent.action.authorized on our own stream
    events, _ := a.getEvents(ctx)
    for _, ev := range events {
        if ev.EventType == "agent.action.authorized" &&
           ev.Payload["originalProposal"] == proposalID {
            return a.executeExternalAction(ev)
        }
        if ev.EventType == "agent.action.vetoed" &&
           ev.Payload["vetoes"] == proposalID {
            return ErrActionVetoed
        }
    }
    return ErrActionPending
}

func (a *Agent) RoutineExecute(ctx context.Context, p ActionProposal) error {
    if p.RiskClass == "trivial" {
        // Agent's own signature is sufficient. Still record event
        // for audit trail.
        return a.submitEvent(ctx, buildAutoAuthorizedEvent(p))
    }
    _, err := a.PropseAction(ctx, p)
    return err
}
```

## 4. Cosigners approve or veto

A principal (human) reviews the proposal via a dashboard:

```bash
# Approve
curl -X POST $NODE/api/v1/events -d '{
  "subjectId":"agent-acme-finance-001",
  "subjectType":"QUID",
  "eventType":"agent.action.cosigned",
  "payload":{
    "signerQuid":"principal-acme-business-owner",
    "cosigns":"<proposal event ID>",
    "authorizedBy":"human-review",
    "timestamp":<now>
  },
  "creator":"principal-acme-business-owner",
  "signature":"<sig>"
}'

# Veto (if something's off)
curl -X POST $NODE/api/v1/events -d '{
  "subjectId":"agent-acme-finance-001",
  "subjectType":"QUID",
  "eventType":"agent.action.vetoed",
  "payload":{
    "vetoes":"<proposal event ID>",
    "reason":"amount exceeds monthly budget"
  },
  "creator":"safety-committee-acme",
  "signature":"<sig>"
}'
```

## 5. Quorum checker (system-side service)

A lightweight background service watches each agent's event
stream and, when quorum is met, emits the `authorized` event:

```go
func (q *QuorumWatcher) Tick(ctx context.Context) {
    agents := q.ListAgents(ctx)
    for _, agentQuid := range agents {
        pending := q.PendingProposals(ctx, agentQuid)
        set := q.ledger.EffectiveGuardianSet(agentQuid, time.Now())
        for _, p := range pending {
            if q.hasVeto(ctx, p.ID) {
                continue  // vetoed; skip
            }
            weight := q.weightOfCosigners(ctx, p.ID, set)
            if weight >= uint32(set.Threshold) {
                q.emitAuthorized(ctx, agentQuid, p.ID, weight, set.Threshold)
            }
        }
    }
}
```

## 6. Time-lock window for high-value actions

High-risk actions can enforce an additional time-lock:
proposal is visible for N hours before quorum can fire,
giving the safety committee time to veto.

```go
func (q *QuorumWatcher) TimeLockExpired(p Proposal, riskClass string) bool {
    windowSeconds := map[string]int64{
        "trivial":     0,
        "low-routine": 300,        /* 5 min */
        "medium":      3600,       /* 1h */
        "high":        86400,      /* 24h */
        "emergency":   0,
    }[riskClass]
    return time.Now().Unix()-p.ProposedAt >= windowSeconds
}
```

## 7. Emergency kill-switch

```bash
# Principal or safety committee invalidates the agent's epoch
curl -X POST $NODE/api/anchors -d '{
  "kind":"invalidation",
  "signerQuid":"agent-acme-finance-001",
  "epochToInvalidate":0,
  "anchorNonce":<next>,
  "validFrom":<now>,
  "signature":"<signed by principal via their own key>"
}'
```

Agent's signatures stop verifying. External systems (bank, repo,
email provider) that verify through Quidnug reject the agent's
requests.

For systems that don't integrate with Quidnug directly, a
monitoring watchdog can see the invalidation event and
revoke tokens at the API gateway.

## 8. Rotation: agent compromised → new key

Guardian recovery by the capability committee:

```bash
curl -X POST $NODE/api/v2/guardian/recovery/init -d '{
  "subjectQuid":"agent-acme-finance-001",
  "fromEpoch":0,
  "toEpoch":1,
  "newPublicKey":"<hex of fresh key>",
  "minNextNonce":1,
  "maxAcceptedOldNonce":0,
  "anchorNonce":<next>,
  "validFrom":<now>,
  "guardianSigs":[
    { "guardianQuid":"principal-acme-business-owner","keyEpoch":0,"signature":"..." },
    { "guardianQuid":"safety-committee-acme","keyEpoch":0,"signature":"..." }
  ]
}'
```

With `recoveryDelay=1h` and `requireGuardianRotation=true`, this
is the only way to rotate. Attacker with the agent's compromised
key cannot self-rotate.

## 9. Sub-agent delegation

Agent-A delegates a narrow capability to agent-B:

```bash
# Agent-A grants scoped trust to agent-B
curl -X POST $NODE/api/trust -d '{
  "truster":"agent-acme-finance-001",
  "trustee":"agent-sub-invoice-parser-001",
  "trustLevel":0.4,
  "domain":"data.invoices.acme",
  "validUntil":<now + 1h>,
  "description":"Parse incoming invoices; report back"
}'
```

Sub-agent operates within the scope; its own event stream is
auditable; if it misbehaves, Agent-A revokes.

## 10. Testing

```go
func TestAgent_LowRiskSelfApproves(t *testing.T) {
    // Agent proposes trivial action → authorized without cosigners
}

func TestAgent_HighRiskRequiresCommittee(t *testing.T) {
    // Agent alone → not authorized
    // Agent + safety committee → authorized (weight 1+2)
}

func TestAgent_VetoesWork(t *testing.T) {
    // Proposal + cosigns meeting threshold
    // Subsequent veto → agent detects and aborts
}

func TestAgent_EmergencyInvalidation(t *testing.T) {
    // After invalidation, agent's further signatures rejected
}

func TestAgent_TimeLockedHighRiskAction(t *testing.T) {
    // High-risk proposal with cosigners
    // Before time-lock expires, not yet authorized
    // After, authorized
}
```

## Where to go next

- [`threat-model.md`](threat-model.md)
- [`../ai-model-provenance/`](../ai-model-provenance/) — trust in
  the underlying model
