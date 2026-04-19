# Threat Model: AI Content Authenticity

## Assets

1. **Attribution integrity** — the chain of who captured,
   edited, and published an asset.
2. **Consumer decision quality** — ability to correctly
   identify authentic vs. manipulated vs. generated media.
3. **Trust roots** — camera manufacturers, editing software,
   publishers.

## Attackers

| Attacker               | Capability                              | Goal                               |
|------------------------|-----------------------------------------|-----------------------------------|
| Deepfake producer      | Can generate synthetic media            | Pass off as captured              |
| Propaganda operator    | Resources to forge chains               | Undermine public trust            |
| Compromised editor key | Has valid editor signing key            | Forge edit events                 |
| Camera firmware exploit| Hack device to sign arbitrary captures  | Create fake "real" photos         |
| Publisher compromise   | Valid publisher signing key             | Publish manipulated content       |

## Threats

### T1. Deepfake passed as real photo
**Attack.** Attacker generates an image and submits a fake
`media.captured` event claiming a real camera captured it.
**Mitigation.** Camera's signing key is hardware-bound; an
attacker doesn't have the camera's private key. A capture
event signed by a quid that isn't actually a registered
camera fails consumer's trust evaluation.
**Residual risk.** If the attacker compromises an actual
camera's key (T4), they can generate signed captures. See T4.

### T2. Edit-chain forgery
**Attack.** Attacker creates a chain of `media.edited`
events impersonating legitimate editors.
**Mitigation.** Each event signed by editor's quid;
attacker lacks those keys. Relational-trust evaluation by
consumer checks each editor's quid against their trust
graph.

### T3. Compromised editor's key
**Attack.** A legitimate editor's key is stolen. Attacker
forges edit events.
**Mitigation.** Guardian recovery rotates to new key.
Post-rotation, old signatures invalid. `GuardianResignation`
(QDP-0006) lets someone who left remove themselves.

### T4. Camera firmware exploit
**Attack.** Firmware bug/exploit lets attacker use a real
camera's signing key to sign arbitrary images.
**Mitigation.** Camera manufacturer can revoke the specific
device's trust (set trust=0). Push gossip propagates the
revocation. Consumers then evaluate the device's trust as 0
→ captures from it are not trusted.
**Residual risk.** Window between exploit discovery and
revocation.

### T5. Publisher collusion with fake content
**Attack.** Publisher knowingly publishes manipulated
content, uses their own signing authority.
**Mitigation.** Publisher's reputation visible to all.
Independent fact-checkers can emit `media.authenticity.disputed`.
Consumers weigh the publisher's trust vs. the fact-checker's
in reaching conclusion.

### T6. AI-generated content passed as authentic
**Attack.** An AI tool is used to generate content; attacker
signs it with a trusted editor's key and claims
`media.edited` but not `media.generated`.
**Mitigation.** Without a preceding `media.captured` event
from a camera's key, the chain has no authentic origin. A
verifier looking for the capture event at the beginning of
the chain rejects the asset.
**Residual risk.** If the forger also forges a `media.captured`
event (requires camera key compromise T4), the defense
collapses to T4's mitigation.

### T7. Event reordering / replay
**Attack.** Attacker replays a captured edit event on a
different asset.
**Mitigation.** Events bound to subject (asset) ID +
anchor-nonce monotonicity. Replay to different asset has
wrong subject; replay on same asset has used-up nonce.

### T8. Cross-domain trust confusion
**Attack.** Asset with capture from `media.provenance.evidentiary`
is displayed as if it's from `media.provenance.news` where
different trust rules apply.
**Mitigation.** Domain scoping is explicit in each title.
Consumers should filter by expected domain.

### T9. Synthetic content denial (reverse of T1)
**Attack.** AI-generated content publisher signs it as
authentic; later claims it was labeled. Creates deniability.
**Mitigation.** Event stream's append-only history —
`media.generated` either exists or doesn't. Either the
publisher signed an attestation at generation time, or
they didn't. No "retroactively added" event.

## Not defended against

1. **Sufficiently skilled forgery without key compromise** —
   no system can detect content that was cryptographically
   generated with legitimate keys and a misleading narrative
   (a real camera photographing a deceptive scene).
2. **Context manipulation** — valid photo, false caption.
   Fact-checkers provide this counter-weight at the
   application layer.
3. **Trust system Sybils** — attacker spins up many "fake"
   fact-checker quids and endorses their own content.
   Mitigation: consumer's trust graph reflects only
   third-parties they independently trust.

## References

- [QDP-0002 Guardian Recovery](../../docs/design/0002-guardian-based-recovery.md)
- [QDP-0005 Push Gossip](../../docs/design/0005-push-based-gossip.md)
- [`../ai-model-provenance/`](../ai-model-provenance/)
