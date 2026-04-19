# Implementation: Institutional Custody

Concrete Quidnug setup for a multi-jurisdiction custody firm.

## 0. Topology

Three Quidnug nodes minimum:
- `node-us.custody.example` (US subsidiary)
- `node-eu.custody.example` (EU subsidiary)
- `node-root.custody.example` (firm-level coordination)

Each runs a Quidnug node with:
```
ENABLE_NONCE_LEDGER=true
ENABLE_PUSH_GOSSIP=true
ENABLE_LAZY_EPOCH_PROBE=true
ENABLE_KOFK_BOOTSTRAP=true
SEED_NODES=[...]
NODE_AUTH_SECRET=...
```

## 1. Hierarchical identity setup

```bash
# 1a. Root identity (rarely used; kept cold)
curl -X POST $NODE_ROOT/api/identities -d '{
  "quidId":"custody-root",
  "name":"Custody Firm Root",
  "creator":"custody-root",
  "updateNonce":1,
  "attributes":{"jurisdictions":["US","EU"]}
}'

# 1b. Root guardian set — 3-of-4 executive quorum
curl -X POST $NODE_ROOT/api/v2/guardian/set-update -d '{
  "subjectQuid":"custody-root",
  "newSet":{
    "guardians":[
      {"quid":"ceo-quid","weight":1,"epoch":0},
      {"quid":"ciso-quid","weight":1,"epoch":0},
      {"quid":"coo-quid","weight":1,"epoch":0},
      {"quid":"ext-auditor-quid","weight":1,"epoch":0}
    ],
    "threshold":3,
    "recoveryDelay":604800000000000,   /* 7 days in ns */
    "requireGuardianRotation":true
  },
  ...
}'

# 1c. US subsidiary quid
curl -X POST $NODE_US/api/identities -d '{
  "quidId":"custody-us",
  "name":"US Custody Subsidiary",
  "creator":"custody-root",
  "updateNonce":1
}'

# 1d. US subsidiary guardian set
curl -X POST $NODE_US/api/v2/guardian/set-update -d '{
  "subjectQuid":"custody-us",
  "newSet":{
    "guardians":[
      {"quid":"us-signer-1","weight":1,"epoch":0},
      {"quid":"us-signer-2","weight":1,"epoch":0},
      {"quid":"us-signer-3","weight":1,"epoch":0},
      {"quid":"us-signer-4","weight":1,"epoch":0},
      {"quid":"us-signer-5","weight":1,"epoch":0}
    ],
    "threshold":3,
    "recoveryDelay":86400000000000,    /* 1 day */
    "requireGuardianRotation":false    /* subsidiary can have fast-path */
  },
  ...
}'

# Repeat for EU.
```

## 2. Wallet setup

Each wallet is its own quid:

```bash
curl -X POST $NODE_US/api/identities -d '{
  "quidId":"wallet-cold-btc-001",
  "name":"US Cold Storage BTC Wallet 001",
  "creator":"custody-us",
  "updateNonce":1,
  "attributes":{
    "chain":"bitcoin",
    "type":"cold-storage",
    "onChainAddress":"bc1q...",
    "limit":"100"  /* BTC */
  }
}'

# Cold wallet: 5-of-7 quorum, 7-day delay, required guardian rotation
curl -X POST $NODE_US/api/v2/guardian/set-update -d '{
  "subjectQuid":"wallet-cold-btc-001",
  "newSet":{
    "guardians":[
      {"quid":"us-signer-1","weight":1,"epoch":0},
      /* ... 7 signers ... */
    ],
    "threshold":5,
    "recoveryDelay":604800000000000,
    "requireGuardianRotation":true
  },
  ...
}'

# Hot wallet: 2-of-3 quorum, 1-hour delay
# (configure separately per wallet)
```

## 3. Transfer authorization flow

Same as interbank wires — see
[`../interbank-wire-authorization/implementation.md`](../interbank-wire-authorization/implementation.md)
§3 for the detailed sequence. Key differences:

- Title includes `targetChain`, `targetAddress`, `amount`.
- Quorum check runs against the wallet's specific guardian set.
- On approval, subsidiary's bridge extracts signatures and
  submits to the on-chain multi-sig contract.

## 4. Quarterly rotation automation

```go
package custody

func (c *Custodian) AutoRotateSigners(ctx context.Context, maxAgeDays int) error {
    signers := c.ListSigners()
    for _, signer := range signers {
        lastRotation := c.GetLastRotationAnchor(signer.Quid)
        age := time.Since(time.Unix(lastRotation.Timestamp, 0))
        if age.Hours() > float64(maxAgeDays)*24 {
            // Overdue — notify the signer and begin rotation.
            c.NotifySignerRotationDue(signer)
            continue
        }
        if age.Hours() > float64(maxAgeDays-14)*24 {
            // 14-day warning
            c.NotifyUpcomingRotation(signer)
        }
    }
    return nil
}

func (c *Custodian) InitiateRotation(ctx context.Context, signerQuid string, newPubKey string) error {
    current := c.ledger.CurrentEpoch(signerQuid)
    nextNonce := c.ledger.LastAnchorNonce(signerQuid) + 1

    rotation := core.NonceAnchor{
        Kind:                core.AnchorRotation,
        SignerQuid:          signerQuid,
        FromEpoch:           current,
        ToEpoch:             current + 1,
        NewPublicKey:        newPubKey,
        MinNextNonce:        1,
        MaxAcceptedOldNonce: 100,      /* grace for in-flight approvals */
        AnchorNonce:         nextNonce,
        ValidFrom:           time.Now().Unix(),
    }
    signable, _ := core.GetAnchorSignableData(rotation)
    rotation.Signature = signWithCurrentEpochHSM(signerQuid, signable)

    return c.submitAnchor(rotation)
}
```

## 5. Emergency invalidation

A signer's key is confirmed compromised. Immediate action:

```bash
# Freeze the epoch
curl -X POST $NODE_US/api/anchors -d '{
  "kind":"invalidation",
  "signerQuid":"us-signer-3",
  "epochToInvalidate":2,
  "anchorNonce":<next>,
  "validFrom":<now>,
  "signature":"<signed by a currently-trusted signer or via guardian quorum>"
}'
```

Now no further transactions at that epoch are admitted
anywhere in the network (even cross-subsidiary).

## 6. Cross-subsidiary transfer

```
US wallet → EU wallet transfer:

1. US subsidiary creates an "outbound transfer" title with their
   5-of-7 quorum.
2. Their on-chain multi-sig executes: funds move from US BTC
   address to EU BTC address.
3. EU subsidiary creates an "inbound attestation" title
   confirming receipt, cosigned by EU's quorum.
4. Both titles link to each other via a shared correlation ID.

Compliance sees a full chain: US approvers + EU approvers both
on-chain.
```

## 7. Lazy epoch probe in action

The US subsidiary has signer-EU-3 in its trust graph (for
cross-subsidiary transfer verification). EU-3 hasn't signed
anything the US node has seen in 35 days.

When a new cross-subsidiary transfer arrives with EU-3's
signature:

```
US node:
  1. Check recency of EU-3: > 7-day window → stale.
  2. Quarantine the transaction.
  3. Probe EU's domain: GET /api/v2/domain-fingerprints/eu.custody/latest
  4. EU node responds with its latest fingerprint; EU-3's epoch
     is 4 (they rotated 3 times we didn't see).
  5. US ledger updates EU-3's epoch.
  6. Re-validate the transaction: signed at epoch 4? OK, accept.
                                   signed at stale epoch 2? Reject.
```

Catches the scenario where an attacker has EU-3's old (rotated-
out) key and tries to use it in the US.

## 8. Subsidiary opening (bootstrap)

Opening an APAC subsidiary:

```bash
# 8a. APAC node is fresh. Bootstrap trust list seeded by firm ops:
# (3 known-good peers: US, EU, external auditor)
```
```go
apacNode.SeedBootstrapTrustList([]core.BootstrapTrustEntry{
    {Quid:"custody-us",PublicKey:"<hex>"},
    {Quid:"custody-eu",PublicKey:"<hex>"},
    {Quid:"ext-auditor-quid",PublicKey:"<hex>"},
}, 3)

sess, _ := apacNode.BootstrapFromPeers(ctx, "custody.firm.global",
    core.DefaultBootstrapConfig())
if sess.State == core.BootstrapQuorumMet {
    apacNode.ApplyBootstrapSnapshot(64)
}
```

## 9. Policy change: raise threshold globally

Firm decides all wallets now need 6-of-9 instead of 5-of-7.
Fork-block coordination:

```bash
curl -X POST $NODE_ROOT/api/v2/fork-block -d '{
  "trustDomain":"custody.firm.global",
  "feature":"require_tx_tree_root",   /* placeholder for the actual policy update */
  "forkHeight":<future>,
  "forkNonce":1,
  "signatures":[ /* executive quorum */ ]
}'
```

At the fork height, every subsidiary's nodes apply the new
policy simultaneously.

## 10. Audit report generation

```go
func (c *Custodian) GenerateQuarterlyAudit(ctx context.Context, q Quarter) AuditReport {
    report := AuditReport{}

    // 1. For each wallet, list all transfers that completed in
    //    the quarter.
    for _, wallet := range c.ListWallets() {
        transfers := c.ListApprovedTransfers(wallet.Quid, q.Start, q.End)
        for _, t := range transfers {
            entry := AuditEntry{
                WalletID:      wallet.Quid,
                TransferID:    t.TitleID,
                Amount:        t.Amount,
                Signers:       []SignerInfo{},
            }
            for _, sig := range t.Signatures {
                // Look up the signer's epoch at the time of signing
                epoch := c.ledger.EpochAt(sig.SignerQuid, t.BlockHeight)
                entry.Signers = append(entry.Signers, SignerInfo{
                    Quid:  sig.SignerQuid,
                    Epoch: epoch,
                    LastRotated: c.LastRotationTime(sig.SignerQuid, epoch),
                })
            }
            report.Entries = append(report.Entries, entry)
        }
    }

    // 2. List all guardian-set changes in the quarter.
    report.SetChanges = c.GuardianSetChangesInRange(q.Start, q.End)

    // 3. List all rotations and invalidations.
    report.Rotations = c.RotationsInRange(q.Start, q.End)
    report.Invalidations = c.InvalidationsInRange(q.Start, q.End)

    return report
}
```

## 11. Testing

```go
func TestCustody_ColdWalletRequiresFullQuorum(t *testing.T) {
    // 5-of-7 cold wallet, only 4 signatures → not approved
}

func TestCustody_EpochRotationDoesNotAffectInFlight(t *testing.T) {
    // Signer starts approving a transfer with epoch 0
    // Signer rotates to epoch 1
    // Transfer completes with remaining signers' epoch-1 sigs
    // Original epoch-0 signature still valid (within MaxAcceptedOldNonce)
}

func TestCustody_CrossSubsidiaryStaleKeyDetected(t *testing.T) {
    // US node hasn't seen EU-3 in 35d
    // EU-3 has rotated 3 times
    // Attack with old-epoch EU-3 sig detected via lazy probe
}
```

## Where to go next

- [`threat-model.md`](threat-model.md)
- Similar patterns: [`../interbank-wire-authorization/`](../interbank-wire-authorization/),
  [`../developer-artifact-signing/`](../developer-artifact-signing/)
