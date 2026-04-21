# Institutional crypto custody, POC demo

Runnable proof-of-concept for the
[`UseCases/institutional-custody/`](../../UseCases/institutional-custody/)
use case. Demonstrates M-of-N custody-wallet transfer approval,
per-signer rotation-epoch tracking, and emergency freeze -- with
every step producing a signed audit record.

## What this POC proves

Seven signers, an ops officer, a compliance auditor, and one
cold-storage wallet quid on a shared custody domain. Key claims
the demo verifies:

1. **M-of-N thresholds work.** A $5M transfer stays in `pending`
   state at four cosigners and flips to `authorized` once the
   fifth cosigner commits. Thresholds are declarative policy
   checked against signed stream events.
2. **Stale epochs are rejected.** A signer cosigning at an epoch
   older than their current epoch is not counted. Policy can
   optionally allow a grace window (matches AnchorRotation's
   `maxAcceptedOldNonce` pattern).
3. **Duplicate or non-policy signers are ignored.** An impostor's
   signature or a signer double-cosigning does not inflate the
   count. The audit trail records the rejection with reason.
4. **Freeze is atomic.** A single `wallet.frozen` event on the
   wallet's own stream denies every future transfer regardless
   of cosign count, with no per-wallet reconfiguration.
5. **The forensic audit is the stream.** Every proposal, cosign,
   and freeze event carries the signer's quid and epoch. A year
   later an auditor can replay the stream to reconstruct "who
   signed what, at what epoch, on what date."

## What's in this folder

| File | Purpose |
|---|---|
| `custody_policy.py` | Pure decision logic. `WalletPolicy`, `TransferAuthorization`, `SignerApproval`, `evaluate_transfer`, `audit_report`, stream extractors. No SDK dep. |
| `custody_policy_test.py` | 14 pytest cases covering threshold met / missed, duplicate / impostor / stale-epoch / future-epoch, frozen wallet, cross-transfer pollution, stale-rotation watch, stream extraction, audit rendering. |
| `demo.py` | End-to-end runnable against a live node. Eight steps from actor registration to post-freeze denial. |

## Running

```bash
# 1. Start a local node.
cd deploy/compose && docker compose up -d

# 2. Install Python SDK.
cd clients/python && pip install -e .

# 3. Run the demo.
cd examples/institutional-custody
python demo.py
```

## Testing without a live node

```bash
cd examples/institutional-custody
python -m pytest custody_policy_test.py -v
```

## QDP catalog audit

| Feature | Purpose | Status |
|---|---|---|
| `IDENTITY` tx | Register ops, auditor, wallet, signers | v1.0 |
| `TITLE` tx | Transfer authorization as an asset owned by the wallet | v1.0 |
| `EVENT` tx streams | Proposals, cosignatures, freezes, policy installations | v1.0 |
| QDP-0001 nonce ledger | Each signer's signing nonce tracked on-chain | v1.0 |
| QDP-0002 guardian recovery | Signer HSM failure -> recovery to new key | v1.0 (referenced in Step 7 monitoring commentary; not executed) |
| QDP-0005 push gossip | Rotations propagate across subsidiaries in seconds | v1.0 |
| QDP-0006 guardian resignation | Signer leaves the firm | v1.0 (not exercised) |
| QDP-0007 lazy epoch probe | Cross-subsidiary node probes for missed rotations | v1.0 (not exercised) |
| QDP-0008 K-of-K bootstrap | New subsidiary joins the federation | v1.0 (not exercised) |
| QDP-0009 fork-block | Raise network-wide minimum threshold | v1.0 (not exercised) |

No protocol gaps. The custody shop's policy engine is a thin
layer over these primitives.

## What a production deployment would add

- **Actual GuardianSet install** via `/api/v2/guardian/set-install`
  instead of policy-held-in-client. The POC keeps the policy
  structure local so the demo runs without a separate guardian
  setup call, but production would install the policy on-chain.
- **Per-subsidiary wallets.** The demo has one wallet; production
  would have per-subsidiary quids (US vault, EU vault, APAC
  vault) each with their own signer set, and cross-subsidiary
  transfers requiring quorum from both.
- **Automatic freeze on stale-epoch watch.** The demo's
  monitoring rule just prints overdue signers. Production would
  wire it to an auto-freeze via a compliance bot's signed event.
- **Hardware-security-module integration.** In the POC each
  signer is a Python-generated keypair. Production signers live
  in YubiHSMs, Ledger, or AWS CloudHSM.
- **On-chain bridge.** Once a transfer reaches `authorized`, a
  bridge service would extract the cosignatures and submit the
  on-chain multi-sig transaction to Bitcoin / Ethereum / Solana.

## Related

- Use case: [`UseCases/institutional-custody/`](../../UseCases/institutional-custody/)
- Related POC: [`examples/ai-agent-authorization/`](../ai-agent-authorization/)
  uses the same cosign-quorum pattern with risk-class routing
  for AI-agent spending
- Related POC: [`examples/interbank-wire-authorization/`](../interbank-wire-authorization/)
  (upcoming) applies the same pattern to correspondent banking
- Protocol: [QDP-0002 Guardian Recovery](../../docs/design/0002-guardian-based-recovery.md)
