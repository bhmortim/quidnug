# Interbank wire authorization, POC demo

Runnable proof-of-concept for the
[`UseCases/interbank-wire-authorization/`](../../UseCases/interbank-wire-authorization/)
use case. Demonstrates tier-based, weighted, role-gated wire
approval with per-signer nonce replay protection and
counterparty re-verification on the same signed event stream.

## What this POC proves

Sender bank, receiving bank, three officers, one compliance
officer on a shared `wires.federal.us` domain. Key claims the
demo verifies:

1. **Amount-tiered policy works.** A $500 wire needs one signer
   (tier 1); $5M needs two (tier 2); $50M needs three weight
   plus the compliance role (tier 3). The policy is a single
   declarative structure evaluated per wire.
2. **Weighted signers change the math.** The compliance officer
   has weight 2, so they can single-handedly satisfy a tier 2
   weight threshold. But tier 3 explicitly requires the
   compliance role, not just weight.
3. **Receiver bank reaches the same verdict without a phone call.**
   The same decision function run against the same stream gives
   both sender and receiver identical verdicts.
4. **Per-signer nonces catch replays.** An attacker reposting a
   used nonce against a new wire has their cosignature rejected
   at the evaluator, with a clear "replay" reason in the audit.
5. **The stream is the audit.** Every proposal, cosignature, and
   policy-installation is a signed event on the bank's or wire's
   stream, queryable by regulators and counterparties.

## What's in this folder

| File | Purpose |
|---|---|
| `wire_approval.py` | Pure decision logic: `WirePolicy`, `ApprovalTier`, `WireSigner`, `WireApproval`, `evaluate_wire`, `receiver_verify`, in-memory `NonceLedger`, stream extractors. |
| `wire_approval_test.py` | 14 pytest cases across tier routing, weighted signers, required-role gating, nonce replay, non-policy rejection, dedup, and receiver parity. |
| `demo.py` | End-to-end runnable against a live node. Seven steps across the three tiers, receiver re-verification, and a replay defense. |

## Running

```bash
# 1. Start a local node.
cd deploy/compose && docker compose up -d

# 2. Install Python SDK.
cd clients/python && pip install -e .

# 3. Run the demo.
cd examples/interbank-wire-authorization
python demo.py
```

## Testing without a live node

```bash
cd examples/interbank-wire-authorization
python -m pytest wire_approval_test.py -v
```

## QDP catalog audit

| Feature | Purpose | Status |
|---|---|---|
| `IDENTITY` tx | Register bank, officers, compliance | v1.0 |
| `TITLE` tx | Wire as an asset owned by the sender bank | v1.0 |
| `EVENT` tx streams | Proposals, cosignatures, policy installations | v1.0 |
| QDP-0001 nonce ledger | Per-signer monotonic nonces catch replays | v1.0 |
| QDP-0002 guardian recovery | Officer HSM failure -> new HSM via guardian quorum | v1.0 (not exercised) |
| QDP-0005 push gossip | Fast propagation to counterparty banks | v1.0 |
| QDP-0006 guardian resignation | Officer leaves the bank | v1.0 (not exercised) |
| QDP-0007 lazy epoch probe | Correspondent bank probes after 30 days of silence | v1.0 (not exercised) |
| QDP-0009 fork-block | Consortium-wide threshold change ($10M -> $25M) | v1.0 (not exercised) |

No protocol gaps. The policy engine is thin application-layer
logic.

## What a production deployment would add

- **Actual on-chain GuardianSet install** for the bank's
  approval set, so the policy is enforced at the node layer
  (not only in the application). The POC keeps the policy
  structure client-side for hermetic demo runs.
- **Fedwire / CHIPS / SWIFT bridges.** Once the verdict flips
  to approved, a bridge service emits the actual settlement
  instruction to the appropriate rail.
- **Per-counterparty TRUST_IMPORT** (QDP-0013 federation).
  Each correspondent bank runs its own Quidnug network and
  imports the sender's by trust edge, so the audit trail
  crosses network boundaries cleanly.
- **HSM integration.** Each officer's key lives in their own
  YubiHSM / AWS CloudHSM / nCipher. The Python SDK's signing
  call becomes a thin HSM client.
- **Automatic policy activation via fork-block.** When the
  bank consortium decides to raise the high-value threshold,
  a QDP-0009 fork-block transaction activates the new tier
  structure at a scheduled height across all participants.

## Related

- Use case: [`UseCases/interbank-wire-authorization/`](../../UseCases/interbank-wire-authorization/)
- Related POC: [`examples/institutional-custody/`](../institutional-custody/)
  is the same M-of-N cosigning pattern for crypto custody;
  differs in wallet vs. wire framing
- Related POC: [`examples/ai-agent-authorization/`](../ai-agent-authorization/)
  is the same pattern for agent spending; differs in
  risk-class-based routing vs. amount-tier routing
- Protocol: [QDP-0001 Nonce Ledger](../../docs/design/0001-global-nonce-ledger.md)
- Protocol: [QDP-0002 Guardian Recovery](../../docs/design/0002-guardian-based-recovery.md)
