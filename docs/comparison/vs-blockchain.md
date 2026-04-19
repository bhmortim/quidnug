# Quidnug vs. Ethereum / on-chain reputation

"Blockchain identity" projects on Ethereum (and Cosmos, Solana,
Polkadot) solve overlapping problems to Quidnug. Here's the
honest comparison.

## What public blockchains do well

- **Global consensus.** Every participant has the same view.
- **Programmable smart contracts.** Token incentives and
  complex multi-party logic.
- **Censorship resistance.** Hard to suppress information once
  committed.
- **Permissionless participation.** Anyone can join.

## What public blockchains aren't great at

- **Personal trust.** On-chain reputation is necessarily a
  single public number per identity. You can't have "Alice
  trusts Bob at 0.7 from her perspective" on a public ledger
  without reducing it to a visible number that every observer
  sees.
- **Privacy.** Every transaction is public forever. ZK-SNARKs
  help but add massive complexity.
- **Throughput.** Ethereum L1 does ~15 tx/s. L2s help but the
  proof systems have their own constraints.
- **Cost.** Every transaction has a fee.
- **Key recovery.** The canonical answer is "your key IS your
  identity; lose it, lose everything." Social recovery
  contracts exist but aren't protocol-native.
- **Domain scoping.** No built-in notion of "trust in this
  domain is separate from trust in that domain."

## What Quidnug adds over a blockchain

| Capability | Blockchain | Quidnug |
| --- | --- | --- |
| Per-observer relational trust | global single number | native, per-viewer |
| Privacy by default | no (public ledger) | yes (domains are isolated; cross-domain only via signed gossip) |
| Cost per transaction | gas fee | zero |
| Throughput | L1: ~15 tx/s, L2: ~1000 tx/s | per-domain ~5k tx/s (bench) |
| M-of-N recovery | smart contract | protocol-native QDP-0002 |
| Event streams | call Event log; no structured semantics | native, signed, Merkle-rooted |
| Cross-domain trust gossip | bridges (risky) | QDP-0003 (designed for it) |

## What blockchains add over Quidnug

| Capability | Quidnug | Blockchain |
| --- | --- | --- |
| Global consensus (every observer sees same state) | no (Proof-of-Trust is per-observer) | yes |
| Programmable on-chain logic (smart contracts) | no | yes |
| Financial incentives (tokens, staking) | no | yes |
| Liquidity / DeFi integration | via Chainlink external adapter only | native |
| Decentralized trust in the root | per-deployment | protocol-native |

## The "single number" problem

The fundamental disagreement: blockchains assume there's a
single truth every participant should agree on. Quidnug assumes
trust is **personal**: my trust in Alice isn't yours.

If you need global consensus (token transfers, DeFi, governance
votes with worldwide binding), use a blockchain. If you need
personal trust relationships that scale to billions of pairs
without every pair being public, use Quidnug.

Many systems need both. A Quidnug consortium can feed relational
trust scores into an on-chain smart contract via the
[Chainlink external adapter](../../integrations/chainlink/). The
on-chain contract makes a policy decision ("release escrow if
the counterparty's relational trust from the buyer is ≥ 0.7"),
and the personal trust lives off-chain where it belongs.

## When to pick which

### Use a public blockchain when

- You need money / token transfer.
- You need every participant to see the same state.
- Censorship resistance is paramount.
- You're building something DeFi-adjacent.

### Use a private blockchain (Fabric, Besu, Corda) when

- You need global consensus within a known consortium.
- You need smart-contract logic executed by a quorum of
  participants.
- You're building B2B settlement where every party sees every
  transaction.

### Use Quidnug when

- Trust is personal, not universal.
- Privacy-by-domain matters.
- Recovery, audit, and per-observer scoring are core
  requirements.
- You don't want global consensus overhead.

### Use Quidnug + on-chain bridge when

- You have an on-chain smart contract that needs relational
  trust as an input to a decision.
- You want tokenized incentives layered on top of off-chain
  trust relationships.

## License

Apache-2.0.
