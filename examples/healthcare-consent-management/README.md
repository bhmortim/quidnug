# Healthcare consent management, POC demo

Runnable proof-of-concept for the
[`UseCases/healthcare-consent-management/`](../../UseCases/healthcare-consent-management/)
use case. Demonstrates patient-controlled consent, transitive
referrals, revocation, and guardian-quorum emergency override.

## What this POC proves

A patient, PCP, specialist, ER doc, and two guardians (spouse
+ healthcare proxy) on a healthcare consent domain. Key claims:

1. **Direct consent is a signed trust edge** plus a
   `consent.granted` event for auditability. Provider's access
   request against the patient's stream returns `allow` when
   the grant is fresh, valid, and above trust threshold.
2. **Revocation is an instant-effective event.** Any
   subsequent access request after `consent.revoked` is
   denied, regardless of the original grant still being within
   its expiry.
3. **Transitive consent works for referrals.** If the patient
   trusts their PCP and the PCP trusts a specialist, the
   specialist is allowed access (with composed trust clearing
   the policy threshold).
4. **Emergency override requires a guardian quorum.** A single
   captured guardian key is not enough; the threshold (2-of-N
   in the demo) must be met with signatures from quids in the
   actual guardian set. Non-guardians' signatures are ignored.
5. **Every access is a signed event on the patient's stream.**
   The stream is the audit log; patient (or regulator) can
   replay it to see who accessed what and under which consent.

## What's in this folder

| File | Purpose |
|---|---|
| `consent_evaluate.py` | Pure decision logic: `AccessRequest`, `ConsentGrant`, `AccessPolicy`, `evaluate_access`, stream extractors, transitive-chain walker. |
| `consent_evaluate_test.py` | 16 pytest cases: direct / expired / low-trust / category-mismatch / revocation / transitive / emergency quorum variants. |
| `demo.py` | End-to-end runnable against a live node. Seven steps: register actors, grant consent, PCP direct access, cardiologist transitive access, revocation, ER emergency override with quorum, emergency denial without quorum. |

## Running

```bash
# 1. Start a local node.
cd deploy/compose && docker compose up -d

# 2. Install Python SDK.
cd clients/python && pip install -e .

# 3. Run the demo.
cd examples/healthcare-consent-management
python demo.py
```

## Testing without a live node

```bash
cd examples/healthcare-consent-management
python -m pytest consent_evaluate_test.py -v
```

## QDP catalog audit

| Feature | Purpose | Status |
|---|---|---|
| `IDENTITY` tx | Register patient, providers, guardians | v1.0 |
| `TRUST` tx with `validUntil` | Consent as a time-bounded trust edge | v1.0 |
| `EVENT` tx streams | consent.granted, consent.revoked, record.accessed, consent.emergency-override | v1.0 |
| QDP-0002 guardian recovery | Patient key recovery | v1.0 (not exercised directly; referenced by the emergency-override pattern) |
| QDP-0005 push gossip | Fast propagation of revocations across provider nodes | v1.0 |
| QDP-0024 group encryption | Encrypt sensitive event payloads | Phase 1 landed; optional for this POC |

No protocol gaps. The emergency override is modeled as a
multi-guardian-signed event (2+ guardians each emit a signed
override event); in a stricter deployment this would be a
GuardianRecoveryInit with a short delay before commit.

## What a production deployment would add

- **Actual encryption of record payloads.** The POC's
  `record.accessed` events carry no PHI, so the demo runs
  without encryption. A production system would encrypt PHI
  payloads and gate decryption on the verdict this logic
  produces.
- **Guardian recovery delay.** The POC treats emergency
  override as immediately effective once threshold is met.
  Production typically inserts a short (5-15 min) time-lock
  during which the patient or a non-signing guardian can veto,
  to defend against kidnap / coercion scenarios.
- **HIPAA-compliant break-glass pathway.** For life-threatening
  scenarios requiring access in seconds, providers have
  break-glass procedures that bypass the pre-authorization
  check. The Quidnug record becomes the post-hoc audit log,
  not the real-time gate.
- **Provider-side verification enforcement.** The POC assumes
  providers will run the `evaluate_access` check. A production
  system would put it inside the FHIR API gateway or the EHR
  access layer so no clinician workflow can bypass it.
- **Domain-scoped sub-consents.** Patient consents to PCP for
  `clinical-notes` but not `mental-health`; specialist for
  `labs` only; etc. The POC's category field already supports
  this.

## Related

- Use case: [`UseCases/healthcare-consent-management/`](../../UseCases/healthcare-consent-management/)
- Related POC: [`examples/credential-verification-network/`](../credential-verification-network/)
  is the same verifiable-identity pattern for licensing
- Related POC: [`examples/ai-agent-authorization/`](../ai-agent-authorization/)
  uses the same guardian-quorum override pattern for high-value
  agent actions
- Related POC: [`examples/enterprise-domain-authority/`](../enterprise-domain-authority/)
  uses the same group-membership check for private-tier
  visibility
