# Threat Model: Credential Verification Network

## Assets

1. **Credential authenticity** — degrees / licenses / certs
   represent something real.
2. **Issuer reputation** — accreditor + issuer trust chain.
3. **Revocation integrity** — a revoked credential must be
   unverifiable going forward.

## Attackers

| Attacker                   | Capability                                  | Goal                                 |
|----------------------------|---------------------------------------------|--------------------------------------|
| Diploma mill               | Registers as "issuer"                       | Issue fraudulent credentials          |
| Credential holder          | Has legit credential                        | Use credential past revocation        |
| Compromised issuer         | Valid issuer signing key                    | Issue unauthorized credentials        |
| Employer                   | Read-only access                            | Data-mine beyond verification purpose |
| External                   | No access                                   | Exploration / forgery                 |

## Threats and mitigations

### T1. Diploma mill
**Attack.** Registers quid as "Prestigious Online University."
Issues fake degrees.
**Mitigation.** Accreditor trust edges. Diploma mill isn't
accredited; its trust from any legitimate accreditor is zero.
Employers query through accreditor hierarchy; diploma mill's
credentials fail verification.

### T2. Compromised issuer
**Attack.** University registrar's HSM stolen. Attacker issues
forged degrees.
**Mitigation.**
- Guardian recovery rotates issuer key.
- Anchor nonces prevent simple replay.
- Forged credentials may be retracted via revocation events.

**Residual risk.** Between compromise and rotation, forged
credentials can be issued. Mitigation: monitoring of issuance
rate anomalies.

### T3. Credential holder claims valid past revocation
**Attack.** License revoked; holder presents old verification
to employer.
**Mitigation.** Employer verifies via Quidnug at time-of-check,
not from cached record. Revocation propagates within minutes.

### T4. Cross-jurisdiction forgery
**Attack.** Attacker forges credentials from a distant-
jurisdiction issuer that local verifiers don't check.
**Mitigation.** Trust chain must exist from verifier to
issuer. No chain → trust = 0 → verification fails.

### T5. Attribute tampering
**Attack.** Genuine degree, but claimed GPA higher than
actual.
**Mitigation.** Title attributes are signed. Tampering
breaks the signature. Signature verified at each query.

### T6. Revocation spam / abuse
**Attack.** Malicious "issuer" emits bogus `credential.revoked`
events against legitimate credentials.
**Mitigation.** Events signed by revoker. Only the original
issuer (or their authorized successor) has valid authority.
Verifier checks revoker = issuer; bogus revokers are ignored.

### T7. Privacy (degree history → life history)
**Concern.** Verification events reveal "employer X did a
background check on Alice on date Y."
**Mitigation.**
- Verification events can be stored on a privacy-scoped
  subdomain accessible only to the subject.
- Pseudonymous quids are an option.
- Regulatory constraints (GDPR, CCPA) apply at application
  layer.

## Not defended against

1. **Credential lookup via third-parties.** Attacker sees
   what employers verify — pattern inference is possible.
2. **Holder impersonation at application layer.** If an
   attacker convincingly claims to be Alice and uses Alice's
   credential ID, verification confirms the credential exists
   but can't prevent impersonation downstream.
3. **Issuer collusion with holder.** University fraudulently
   issues a real-looking degree. Protocol verifies the
   signature; fraud is out-of-band.

## References

- [`../healthcare-consent-management/`](../healthcare-consent-management/)
- [QDP-0002 Guardian Recovery](../../docs/design/0002-guardian-based-recovery.md)
