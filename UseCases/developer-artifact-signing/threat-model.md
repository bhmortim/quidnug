# Threat Model: Developer Artifact Signing

## Assets

1. **Package authenticity** — downstream consumers installing
   the real artifact the maintainer intended.
2. **Maintainer identity continuity** — maintainer's public
   key isn't orphaned by one lost HSM.
3. **Supply-chain integrity** — cryptographic chain from code
   commit through release to consumer install.

## Attackers

| Attacker                     | Capability                               | Goal                            |
|------------------------------|------------------------------------------|---------------------------------|
| Malware injector             | Can publish to a registry if credentials | Distribute malware via package  |
| Compromised maintainer key   | Valid signing key                        | Publish malicious release       |
| Dependency-confusion attacker| Register similar-named package           | Trick consumers into wrong pkg  |
| Registry compromise          | Registry's signing key                   | Re-serve altered artifacts      |
| Nation-state                 | Compel maintainer / registry            | Backdoor via legitimate channel |

## Threats and mitigations

### T1. Compromised maintainer key
**Attack.** Attacker has Alice's HSM. Publishes malicious
`webapp-js@2.3.1`.
**Mitigation.**
- Guardian recovery rotates Alice's key. Post-rotation, the
  compromised epoch is invalid. Bob and Carol notice anomalous
  releases and initiate recovery.
- `release.revoked` event on the malicious release;
  downstream verifiers see it within seconds.
- `maxAcceptedOldNonce=0` at rotation time invalidates all
  old-epoch signatures.

**Residual risk.** Window between compromise and detection.
Malware may reach some consumers.

### T2. Dependency confusion
**Attack.** Attacker publishes `webapp-js` (no dash) with a
similar name; downstream `require("webapp-js")` is ambiguous.
**Mitigation.**
- **Trust in the project quid** — consumer trusts
  `project-webapp-js` (dash), not a random quid.
- Registry-level name uniqueness is still needed; Quidnug
  adds identity on top.

### T3. Rogue maintainer (insider)
**Attack.** One of Alice/Bob/Carol goes rogue and publishes
malware.
**Mitigation.**
- Project guardian set has threshold 1 (any maintainer can
  publish). Trade-off chosen for velocity.
- For high-security packages, raise threshold to 2 — every
  release needs cosigning.
- If rogue detected: `GuardianResignation` removes them, or
  guardian-set update replaces the set.

### T4. Registry compromise
**Attack.** NPM registry itself is compromised; attacker
replaces artifacts server-side.
**Mitigation.**
- `artifactHash` on the release title is the authoritative
  hash. Consumers compute the hash from downloaded bytes and
  compare.
- Registry-served artifact bytes tampering → hash mismatch →
  verifier rejects.
- Even with full registry compromise, consumers with Quidnug
  verification remain safe.

### T5. Build environment compromise
**Attack.** Attacker compromises CI/CD, injects backdoor
into built artifact; official build signature is applied.
**Mitigation.**
- `buildProvenance` attestation (sigstore style) is an
  additional field.
- Reproducible builds + attestation from independent
  rebuilders' quids.
- Protocol supports multiple attestations per release; any
  mismatch is visible.

**Residual risk.** Deep supply-chain attacks at build time
are a fundamental challenge beyond Quidnug's scope. Quidnug
helps with the "trust the build attestation authority"
question via relational trust.

### T6. Ecosystem fork-block abuse
**Attack.** Fork-block transaction passed requiring
attestations that attacker controls.
**Mitigation.** 2/3 validator quorum + 24h notice. Npm
ecosystem validators are the major players (npm, Github,
key maintainer councils); collusion hard.

### T7. Revocation propagation latency
**Attack.** Revocation emitted; attacker races to reach
consumers before they see it.
**Mitigation.**
- Push gossip: seconds propagation.
- Verifiers query at install time, not at package publish.
- Cached-verifier systems should periodically re-check.

## Not defended against

1. **Build-time injection at the maintainer's own infra.** If
   Alice's laptop is compromised, the build the laptop
   produces and the signature Alice applies are both
   malicious. Sigstore + independent builders help.
2. **Social engineering of maintainers into signing malicious
   builds.** Protocol can't prevent; monitoring helps.
3. **Vulnerability delay.** A vulnerability exists for some
   time before it's discovered and disclosed. Protocol can't
   help with unknown unknowns.
4. **Ecosystem governance.** Who decides who runs npm
   registry? That's not a protocol question.

## References

- [QDP-0002 Guardian Recovery](../../docs/design/0002-guardian-based-recovery.md)
- [QDP-0006 Guardian Resignation](../../docs/design/0006-guardian-resignation.md)
- [QDP-0009 Fork-Block Trigger](../../docs/design/0009-fork-block-trigger.md)
- [`../ai-model-provenance/`](../ai-model-provenance/)
