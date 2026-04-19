# Implementation: Merchant Fraud Consortium

Concrete Quidnug API calls to build a cross-merchant fraud-sharing
network.

## 0. Configuration

Each member runs a Quidnug node with:

```bash
ENABLE_NONCE_LEDGER=true
ENABLE_PUSH_GOSSIP=true            # real-time signal propagation
ENABLE_LAZY_EPOCH_PROBE=true
SUPPORTED_DOMAINS=fraud.signals.*,fraud.counter-signals.*
SEED_NODES='["peer1.consortium.example:8080","peer2.consortium.example:8080"]'
```

## 1. Register as a consortium member

```bash
# Bootstrap the member's root identity with home domain
curl -X POST http://localhost:8080/api/identities -d '{
  "quidId": "acme-retail",
  "name": "Acme Retail",
  "creator": "acme-retail",
  "updateNonce": 1,
  "homeDomain": "fraud.signals.us-retail",
  "attributes": {
    "orgType": "retailer",
    "vertical": "apparel",
    "established": "2018",
    "contact": "security@acme-retail.example"
  }
}'
```

## 2. Install a guardian set for key recovery

```bash
# Build a GuardianSet for the member's own recovery path.
# (Use the same flow as the interbank case — see
#  ../interbank-wire-authorization/implementation.md §2).
# Members pick their own internal security team as guardians.
curl -X POST http://localhost:8080/api/v2/guardian/set-update -d '{...}'
```

## 3. Declare trust in peers

Each member issues trust edges to peers they've vetted. Trust
edges use the standard `TRUST` transaction:

```bash
curl -X POST http://localhost:8080/api/trust -d '{
  "type": "TRUST",
  "truster": "acme-retail",
  "trustee": "bigbox-inc",
  "trustLevel": 0.9,
  "domain": "fraud.signals.us-retail",
  "nonce": 1,
  "description": "Longstanding partner; strong fraud team",
  "validUntil": 1776844800
}'
```

Repeat for each peer. Trust decays naturally if `validUntil` is
set — forces periodic re-attestation.

## 4. Emit a fraud signal

When Acme's fraud system detects a card-testing pattern:

```bash
curl -X POST http://localhost:8080/api/v1/events -d '{
  "type": "EVENT",
  "subjectId": "card-fp-8f3a9b4e5d2c",  /* hash of card identifiers */
  "subjectType": "QUID",
  "eventType": "fraud.signal.card-testing",
  "payload": {
    "reporter": "acme-retail",
    "severity": 0.9,
    "patternType": "multiple-CVV-retries",
    "observedAt": 1713400000,
    "evidenceHash": "<sha256 of internal evidence blob>",
    "ipGeolocation": "US-CA",
    "actionTaken": "decline"
  },
  "signature": "<ECDSA sig over canonical event bytes>"
}'
```

Note: the raw evidence stays in Acme's internal systems for
privacy. Only a hash and structured metadata go on-chain.

## 5. Consume signals for decision-making

A merchant's fraud engine polls for new events in subscribed
subdomains:

```go
package fraud

import (
    "context"
    "net/http"
    "time"
)

// SignalDecision incorporates both the raw severity and the
// consumer's trust in the reporter.
type SignalDecision struct {
    SignalID        string
    Reporter        string
    RawSeverity     float64
    ConsumerTrust   float64
    EffectiveScore  float64
}

func (f *FraudEngine) EvaluateSignal(ctx context.Context, signal Event) SignalDecision {
    reporter := signal.Payload["reporter"].(string)
    domain := extractDomain(signal.EventType)

    // Query Quidnug for relational trust in the reporter.
    trust, err := f.quidnugClient.GetTrust(ctx, f.selfQuid, reporter, domain,
        &GetTrustOptions{MaxDepth: 4, IncludeUnverified: false})
    if err != nil {
        // Unknown reporter; treat as untrusted for safety.
        return SignalDecision{
            Reporter:       reporter,
            RawSeverity:    signal.Payload["severity"].(float64),
            ConsumerTrust:  0,
            EffectiveScore: 0,
        }
    }

    rawSev := signal.Payload["severity"].(float64)
    return SignalDecision{
        SignalID:       signal.ID,
        Reporter:       reporter,
        RawSeverity:    rawSev,
        ConsumerTrust:  trust.TrustLevel,
        EffectiveScore: rawSev * trust.TrustLevel,
    }
}

// Apply:
//   - effective >= 0.7 → block transaction
//   - 0.4 <= effective < 0.7 → step up (3DS, manual review)
//   - effective < 0.4 → allow but monitor
func (f *FraudEngine) Apply(d SignalDecision) Action {
    switch {
    case d.EffectiveScore >= 0.7:
        return ActionBlock
    case d.EffectiveScore >= 0.4:
        return ActionStepUp
    default:
        return ActionAllow
    }
}
```

## 6. Emit a counter-signal when a signal turns out to be wrong

BigBox finds that a card flagged by Newcomer was actually a
legitimate repeat customer:

```bash
curl -X POST http://localhost:8080/api/v1/events -d '{
  "type": "EVENT",
  "subjectId": "card-fp-8f3a9b4e5d2c",
  "subjectType": "QUID",
  "eventType": "fraud.signal.counter",
  "payload": {
    "reporter": "bigbox-inc",
    "countersEventId": "<event-id of the original>",
    "evidence": "Customer verified via 3DS; completed $2000 purchase",
    "severity": 0.9
  },
  "signature": "<BigBox sig>"
}'
```

Consumers processing signals should look for counter-signals
before acting on the original:

```go
func (f *FraudEngine) ActiveCounter(signalID string) bool {
    events, _ := f.quidnugClient.GetSubjectEvents(signalID, "QUID")
    for _, ev := range events {
        if ev.EventType != "fraud.signal.counter" {
            continue
        }
        if ev.Payload["countersEventId"] == signalID {
            // Trust this counter?
            counterTrust, _ := f.quidnugClient.GetTrust(
                ctx, f.selfQuid,
                ev.Payload["reporter"].(string),
                "fraud.signals.us-retail", nil)
            if counterTrust.TrustLevel >= 0.5 {
                return true
            }
        }
    }
    return false
}
```

## 7. Key compromise emergency

Acme's security team finds evidence their signing key is
compromised. Emergency rotation via guardians:

```bash
curl -X POST http://localhost:8080/api/v2/guardian/recovery/init -d '{
  "kind": "guardian_recovery_init",
  "subjectQuid": "acme-retail",
  "fromEpoch": 0,
  "toEpoch": 1,
  "newPublicKey": "<hex of new signing key>",
  "minNextNonce": 1,
  "maxAcceptedOldNonce": 0,
  "anchorNonce": <next>,
  "validFrom": <now>,
  "guardianSigs": [
    { "guardianQuid": "acme-ciso", "keyEpoch": 0, "signature": "..." },
    { "guardianQuid": "acme-security-lead", "keyEpoch": 0, "signature": "..." }
  ]
}'
```

During the time-lock window (1h default), consortium peers can
see the rotation is pending. Their push gossip catches the
eventual commit; within minutes of commit, Acme's old key is
no longer valid for new signals.

## 8. Joining as a new member

A new merchant wants to join. Four steps:

### 8a. Self-register identity (off-network)

Generate a keypair, build a Quidnug node, create root identity.

### 8b. K-of-K bootstrap from existing members

```go
cfg := core.DefaultBootstrapConfig()
cfg.Quorum = 3
cfg.TrustedPeer = ""  // no operator override — strict K-of-K

// Before bootstrap, seed the trust list with public keys of
// known-good existing members (obtained out-of-band — email,
// website, trade association).
node.SeedBootstrapTrustList([]BootstrapTrustEntry{
    {Quid: "acme-retail",  PublicKey: "<hex>"},
    {Quid: "bigbox-inc",   PublicKey: "<hex>"},
    {Quid: "fin-tech-1",   PublicKey: "<hex>"},
}, 3)

sess, err := node.BootstrapFromPeers(context.Background(),
    "fraud.signals.us-retail", cfg)
if err != nil || sess.State != core.BootstrapQuorumMet {
    // Bootstrap failed — operator investigates.
    panic(err)
}
node.ApplyBootstrapSnapshot(cfg.ShadowBlocks)
```

### 8c. Request inclusion from existing members

Send each member's security team a signed self-introduction
(off-band). Members review the newcomer's reputation,
certifications, and emit trust edges:

```bash
# Acme (having vetted the newcomer)
curl -X POST http://localhost:8080/api/trust -d '{
  "truster": "acme-retail",
  "trustee": "newcomer-ltd",
  "trustLevel": 0.3,     /* low starting trust; grows with performance */
  "domain": "fraud.signals.us-retail",
  "nonce": 47,
  "validUntil": <now + 90d>
}'
```

### 8d. Start emitting signals

After bootstrap and at least one inbound trust edge, the
newcomer can emit signals that will be weighted at whatever
level each peer has assigned.

## 9. Compliance reporting

At quarter-end, produce a report showing:
- Signals emitted by this member
- Signals countered against this member (FP rate)
- Signals received and acted on

```go
// Using the event stream API:
mySignals := client.GetEventsByReporter("acme-retail",
    "fraud.signals.us-retail", startDate, endDate)
countersAgainstUs := client.GetEventsByType(
    "fraud.signal.counter",
    startDate, endDate)

falsePositiveRate := float64(countersAgainstUs) / float64(len(mySignals))
```

## 10. Testing

```go
func TestFraudConsortium_SignalWithTrust(t *testing.T) {
    // Set up 3-node consortium.
    h := newConsortiumHarness(t, 3)
    defer h.close()

    acme := h.nodes[0]
    bigbox := h.nodes[1]

    // BigBox trusts Acme at 0.9.
    submitTrust(t, bigbox, "bigbox-inc", "acme-retail", 0.9, "fraud.signals.us-retail")

    // Acme emits a signal.
    submitSignal(t, acme, "card-fp-1", "acme-retail", 0.8)

    // BigBox evaluates.
    decision := evaluateFor(bigbox, "card-fp-1")
    assert.InDelta(t, 0.72, decision.EffectiveScore, 0.01) // 0.8 × 0.9
    assert.Equal(t, ActionBlock, apply(decision))
}

func TestFraudConsortium_CounterSignalDegrades(t *testing.T) {
    // Verify that a trusted counter-signal causes consumers to
    // de-act on the original.
}

func TestFraudConsortium_ReplayRejected(t *testing.T) {
    // Same signal re-submitted rejected by monotonic nonce.
}
```

## Where to go next

- [`threat-model.md`](threat-model.md)
- [`../defi-oracle-network/`](../defi-oracle-network/) — related
  pattern where signal → price, relational trust → weighted
  aggregation.
