# Credential verification network — POC demo

Runnable proof-of-concept for the
[`UseCases/credential-verification-network/`](../../UseCases/credential-verification-network/)
use case. Demonstrates transitive-trust credential
verification, revocation, and cross-jurisdiction
reciprocity end-to-end.

## What this POC proves

Four roles (accreditor, university, student, employer) on a
shared credential domain. Key claims the demo verifies:

1. **An employer who has never heard of the issuing university
   can still verify a credential** by walking the trust
   graph: employer → accreditor → university.
2. **Revocation works** as a signed event on the student's
   stream, visible to every verifier at decision time.
3. **Cross-jurisdiction verdicts differ by observer.** A US
   employer with a weak trust edge to an APAC accreditor
   reaches "indeterminate" on an APAC-only degree. An APAC
   employer with a strong edge reaches "accept". No central
   authority resolves this — each employer owns their policy.
4. **Credentials are signed events**, not centrally held
   records. The student holds her degree via the stream of
   events on her own quid.

## What's in this folder

| File | Purpose |
|---|---|
| `credential_verify.py` | Standalone verifier. No SDK dep. Defines `CredentialV1`, `verify_credential`, `verify_batch`. |
| `credential_verify_test.py` | 10 pytest cases covering direct trust, transitive trust, revocation, custom thresholds, observer-relative verdicts. |
| `demo.py` | End-to-end runnable against a live node. Seven steps through the full flow. |

## Running

```bash
# 1. Start a local node.
cd deploy/compose && docker compose up -d

# 2. Install Python SDK.
cd clients/python && pip install -e .

# 3. Run the demo.
cd examples/credential-verification-network
python demo.py
```

## Testing without a live node

```bash
cd examples/credential-verification-network
python -m pytest credential_verify_test.py -v
```

## QDP catalog audit

| Feature | Purpose | Status |
|---|---|---|
| `IDENTITY` tx | Register actors | v1.0 |
| `TRUST` tx | Accreditor → university, employer → accreditor edges | v1.0 |
| Transitive trust query | Employer walks accreditor → university chain | v1.0 |
| `EVENT` tx streams | `credential.issued` / `credential.revoked` events on student's stream | v1.0 |
| QDP-0002 guardian recovery | Registrar's key recovery on loss | v1.0 (not exercised in demo but available) |
| QDP-0007 epoch probe | Catch "registrar rotated key before telling employer" attack | Landed |
| QDP-0019 decay | Old credentials fade if issuer's trust graph stops reinforcing | Phase 1 landed |

No protocol gaps. Every required primitive is available.

## What a production deployment would add

- **QDP-0021 ballot-proof pattern** for anonymous credentials
  (e.g., "I am a licensed MD without revealing which state").
  The RSA-FDH primitive exists in `pkg/crypto/blindrsa`; the
  use case would need a credential-specific blind-signature
  flow built on top.
- **QDP-0024 private credentials** for transcripts containing
  sensitive details (GPA, course list). Use the group-keyed
  encryption primitive in `pkg/crypto/groupenc` with the
  student as the sole default member, who can then add
  verifiers temporarily.
- **Integration with W3C Verifiable Credentials format** as a
  wrapper so existing VC-consuming ecosystems interoperate.
  The node treats VCs as opaque payloads on event streams;
  the Quidnug signature + trust-path machinery adds the
  transitive-trust dimension VCs lack natively.
- **National registry cross-signing** (GSA for .gov, EDUCAUSE
  for .edu) so the root accreditors themselves have anchored
  identities via QDP-0023 DNS attestation.

## Related

- Protocol: [QDP-0002 Guardian Recovery](../../docs/design/0002-guardian-based-recovery.md) for registrar key recovery
- Related POC: [`examples/merchant-fraud-consortium/`](../merchant-fraud-consortium/) — same relational-trust property in a different domain
- Legacy demo (pre-v1.0 SDK): [`examples/verifiable-credentials/vc_issue_verify.js`](../verifiable-credentials/vc_issue_verify.js) — uses the older JS SDK path; superseded by this Python demo which uses the v1.0-converged SDK.
