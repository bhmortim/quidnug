# Threat Model: AI Agent Authorization

## Assets

1. **Agent's authority** — whatever the agent can do (spend
   money, write code, send email). Primary asset.
2. **Principal's funds/data** — what the agent has access to.
3. **Safety-committee veto authority** — the ability to stop
   a rogue agent quickly.
4. **Accountability trail** — the signed event history.

## Attackers

| Attacker                  | Capability                                     | Goal                             |
|---------------------------|------------------------------------------------|----------------------------------|
| Compromised agent key     | Signing key of the agent                       | Unauthorized actions             |
| Malicious model           | Influences agent's reasoning                  | Make agent request harmful things|
| Phisher                   | Gets principal's approval signature            | Trick human into cosigning       |
| Misaligned agent          | No external compromise; inherently unsafe      | Pursue unintended objectives     |
| External adversary        | Network, API access                            | Submit forged actions            |

## Threats and mitigations

### T1. Compromised agent key

**Attack.** Attacker steals the agent's signing key and
submits high-value action proposals.

**Mitigation.**
- High-risk actions require cosigners with weight ≥ threshold.
  Attacker with only agent's key = weight 0 in the quorum
  (the agent's own signature is on the proposal, but for
  authorization we sum **guardian** cosigner weights).
- Routine actions the agent can self-sign — but those are
  bounded by risk class and typically involve small amounts
  or narrow scopes.
- Agent is rotatable via guardian recovery.

**Residual risk.** Attacker can still cause low-value
nuisance until rotation completes.

### T2. Manipulated proposals (prompt injection)

**Attack.** Adversarial input to the agent's model causes
it to propose malicious actions (a "please-ignore-prior-
instructions" attack at scale).

**Mitigation.**
- **The model proposes; the committee authorizes.** Even
  if the agent proposes bad things, the committee can
  veto. This is exactly what the design protects against.
- Risk classification happens in the agent's code, not the
  model's output. A model influenced to ask for $1M wires
  gets classified as "high risk" regardless and requires
  full quorum.
- Safety committee's role is specifically to catch this
  class of adversarial input.

**Residual risk.** If the safety committee is not paying
attention (approves everything rubber-stamp), it's a
governance failure. Protocol can't force diligence.

### T3. Rubber-stamp cosigners

**Attack.** Principal defaults to approving everything
without review.

**Mitigation.**
- **Quidnug can't prevent human negligence.** What it can
  do: the entire history is on-chain, so a pattern of
  rubber-stamping is auditable after-the-fact.
- Time-lock windows for high-risk actions at least force
  a delay, giving automated monitors (anomaly detection,
  other humans) a chance to intervene.

### T4. Compromised cosigner

**Attack.** Attacker compromises the principal's key and
cosigns malicious proposals.

**Mitigation.**
- Safety committee still has veto power. Threshold=2 with
  principal(w=1) + safety-committee(w=2) means compromised
  principal alone = 1 weight, insufficient.
- Principal's own recovery guardians rotate them.

### T5. Time-lock bypass via fast-path

**Attack.** Emergency route (safety committee alone) is
supposed to be for genuine emergencies, but the safety
committee itself is compromised and uses the fast path.

**Mitigation.**
- Safety committee = organizational group, usually
  multi-person quorum internally.
- Committee members themselves have their own guardian
  sets.

### T6. Misaligned objectives

**Attack.** The agent isn't compromised — it's working
as designed but its design pursues unintended objectives
(classic AI alignment concern).

**Mitigation.**
- **Narrowly scoped trust edges** — agent is trusted only in
  specific domains, with time-bounded grants.
- **Risk classification + committee review** — safety
  committee is the last line.
- **Event stream forensics** — post-hoc analysis shows
  exactly what the agent proposed and why.

**Residual risk.** Fundamental alignment is beyond any
protocol. Quidnug provides the scaffolding for oversight;
oversight still has to be exercised.

### T7. Sub-agent escalation

**Attack.** Agent-A delegates capability to sub-agent-B.
Sub-agent-B either misbehaves or further delegates,
creating a tree of ambiguous authority.

**Mitigation.**
- Sub-agent-B's capability is bounded by Agent-A's grant
  (trust edge with `validUntil`).
- Sub-agent-B's own event stream is auditable from A's
  perspective.
- Sub-agent-B cannot grant more than it has.

### T8. Time-lock anti-veto race

**Attack.** Agent proposes a high-risk action. Attacker
compromises cosigners fast enough to meet quorum before
the safety committee's watchdog sees the proposal.

**Mitigation.**
- Time-lock window (24h for high-risk) is deliberately
  longer than typical attack-compromise + quorum windows.
- **Push gossip (QDP-0005)** propagates proposals within
  seconds. Watchdogs see them fast.

### T9. Event-stream flooding (DoS)

**Attack.** Attacker floods the agent's stream with junk
events to obscure real ones.

**Mitigation.**
- **Gossip rate limiting** (QDP-0005 §7) — per-producer
  throttle.
- **Event indexing** in the watcher service filters by
  expected types.

### T10. Agent replays own valid action

**Attack.** Compromised agent re-submits a valid old
`authorized` proposal to execute again.

**Mitigation.**
- Every proposal has a unique `proposalID`.
- External action executor checks for "already-executed"
  state before carrying out.
- Agent's anchor nonce is monotonic; can't reuse.

## Not defended against

1. **Misaligned model training.** The agent's model is
   provenance-attested (see
   [`../ai-model-provenance/`](../ai-model-provenance/)),
   but trust in that model's behavior is a separate
   evaluation.

2. **Committee attention fatigue.** If the safety committee
   reviews 1,000 proposals per day, diligence drops.
   Mitigations are organizational (risk-based tiering) not
   protocol.

3. **Deepfake signatures.** If an attacker compromises
   enough of a committee's members, protocol can't save
   you. Guardian recovery for those members helps.

4. **Principal compromise via social engineering.** If the
   principal genuinely approves under false pretense,
   everything after that is "authorized" cryptographically.

## References

- [QDP-0002 Guardian Recovery](../../docs/design/0002-guardian-based-recovery.md)
- [`../ai-model-provenance/`](../ai-model-provenance/)
- [`../institutional-custody/`](../institutional-custody/) — similar M-of-N for humans
