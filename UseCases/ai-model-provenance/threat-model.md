# Threat Model: AI Model Provenance

## Assets

1. **Provenance integrity** — the cryptographic record of
   dataset, training, and fine-tune relationships.
2. **Safety attestations** — signed claims from evaluators.
3. **License-dispute evidence** — the full chain of claims
   and counter-claims.
4. **Model producer reputations** — a producer who has
   shipped many well-attested safe models builds trust.

## Attackers

| Attacker           | Capability                                          | Goal                          |
|--------------------|-----------------------------------------------------|-------------------------------|
| Rogue model producer| Their own signing key                              | False safety claims, hide IP issues |
| Competitor         | No access to producer's keys                       | Smear via false license claims |
| Fake "evaluator"   | Spins up a new quid claiming to be a safety org    | Bogus safety endorsements     |
| Data subject       | Has their own content in training data             | Force takedown via false claims|
| End user           | Consumes inferences                                | Verify authenticity           |

## Threats and mitigations

### T1. Producer falsely claims safety

**Attack.** Acme self-publishes a `safety.evaluated` event
claiming an independent evaluator endorsed safety, but
actually signed it with their own key.

**Mitigation.**
- **Signer verification.** The event is signed by whoever
  submitted it. If Acme submits, only Acme's key matches.
  Consumers looking for "evaluator's own attestation"
  check `creator` on the event, not just the content.
- **Relational trust in evaluator.** A consumer's own trust
  in the evaluator determines weight. Acme self-vouching
  counts as... Acme self-vouching, weighted only by
  consumer's trust in Acme.

**Residual risk.** None structural. Consumer needs to
understand who signs what.

### T2. Fake evaluator quid

**Attack.** Attacker creates a quid named "Anthropic-Evals-
Official" and publishes endorsements of a malicious model.

**Mitigation.**
- **Trust edges must be issued from trusted parties to the
  quid** — consumers don't trust a quid just because of its
  name. They trust it because other entities they trust
  have declared trust in it.
- **Domain ownership** (if configured) — the
  `ai.provenance.safety` domain's validators can prevent
  random quids from claiming to be safety evaluators.

**Residual risk.** Social engineering (naming tricks) can
confuse uninformed consumers. Mitigated by tooling that
shows "trust path" prominently in UI.

### T3. Competitor smear via false license claim

**Attack.** Competitor publishes a `license.claimed` event
claiming Acme violated their copyright (fabricated).

**Mitigation.**
- Acme can contest with `license.contested` event.
- Both claim and contest are visible; consumers weigh both
  by their trust in each party.
- Frivolous claims from low-trust entities are
  de-prioritized.

**Residual risk.** Reputational. A false claim visible on-
chain may chill adoption even if contested. Market dynamics.

### T4. Model producer's key compromise

**Attack.** Acme's signing key is stolen. Attacker publishes
fake derivation events or fake benchmark results.

**Mitigation.**
- **Guardian recovery** rotates Acme's key. Post-rotation,
  attacker's signatures at old epoch become invalid.
- **Anchor nonces** — even with the old key, attacker can't
  replay or re-use a signature.
- **Quick invalidation path** — immediate epoch freeze
  via invalidation anchor.

**Residual risk.** Window between compromise and rotation.
During window, attacker can publish events with old-epoch
sig. Mitigated by monitoring (event-rate anomalies).

### T5. Dataset hash forgery

**Attack.** Acme registers a dataset title with a fake
`datasetHash`. Claims to have trained on a clean dataset
when in fact they trained on something else.

**Mitigation.**
- **Dataset hash is deterministic.** Independent auditors
  can replicate the hash given the dataset. A fake hash
  can be caught in audit.
- **Safety evaluator's report** (if thorough) audits the
  claimed training data. A `safety.evaluated` event from a
  high-trust evaluator includes this.

**Residual risk.** If the dataset is proprietary and no one
can independently re-hash, the claim is on Acme's word
alone. Mitigated by audit norms.

### T6. Fine-tune without authorization

**Attack.** Someone fine-tunes Acme's model without
authorization, then registers the fine-tune as their own.

**Mitigation.**
- Missing `derivation.authorized` event is detectable.
  Consumers' pre-flight checks flag it.
- Licensing enforcement is ultimately legal; Quidnug
  provides the evidence trail.

### T7. Inference forgery

**Attack.** Someone serves inferences from model X and
claims they're from model Y.

**Mitigation.**
- **Inference events signed by the model producer**. Fake
  inferences would need the producer's signing key.
- **Response hash** binds the inference content to the
  attestation — altering the response breaks the hash
  match.

**Residual risk.** If the producer sincerely offers
inferences that then get repackaged by middlemen, middlemen
can misattribute. Consumer needs to verify inference event
signatures, not trust middlemen.

### T8. Privacy: what can be inferred from on-chain data?

**Concern.** The event stream reveals:
- Which models exist
- When they were trained
- Who evaluated them
- Which datasets they used
- Inference counts

**Mitigation / reality.**
- Metadata is public by design — that's the point of
  provenance.
- Sensitive training data stays OFF-chain; only hashes
  are published.
- For inference privacy, emit batch events ("10,000
  inferences this hour") rather than per-inference if
  volume is sensitive.

### T9. Replay

**Attack.** Attacker replays old events.

**Mitigation.** Anchor nonce + dedup. Same as every other
use case.

### T10. Fork-block abuse

**Attack.** Consortium fork-block changes provenance rules
in a way that hides producer accountability.

**Mitigation.** 2/3 quorum + notice period. See
[`../institutional-custody/threat-model.md`](../institutional-custody/threat-model.md).

## Not defended against

1. **Physical-world copyright.** Whether a model's output
   "substantially resembles" copyrighted training data is a
   legal question, not a cryptographic one.

2. **Model weight theft.** If attacker steals Acme's model
   weights, Quidnug can't prevent it. But if they try to
   publish the weights under a new quid, there's no
   `derivation.authorized` from Acme — their claim is
   trivially traceable.

3. **Fine-grained safety.** "Safe for adults but not kids" —
   that's attribute-level granularity beyond a single event.
   Multiple signed evaluations with distinct contexts
   handle this; protocol supports it, just needs
   consumer-side aggregation logic.

4. **Regulatory-mandated provenance** — if the EU AI Act
   requires specific fields we don't yet have, add them.
   Extensible by design.

## References

- [QDP-0002 Guardian Recovery](../../docs/design/0002-guardian-based-recovery.md)
- [QDP-0005 Push Gossip](../../docs/design/0005-push-based-gossip.md)
- [`../ai-agent-authorization/`](../ai-agent-authorization/)
