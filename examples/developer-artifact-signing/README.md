# Developer artifact signing, POC demo

Runnable proof-of-concept for the
[`UseCases/developer-artifact-signing/`](../../UseCases/developer-artifact-signing/)
use case. Demonstrates a GPG-replacement flow: maintainer-signed
releases as Quidnug TITLEs, release lifecycle (publish, CVE
report, patch, revoke) as events on the title's stream, and
relational-trust-gated consumer verification.

## What this POC proves

A software maintainer, a consumer, and a security researcher on
a shared `developer.signing.npm` domain. Key claims the demo
verifies:

1. **Release metadata is signed by the maintainer and
   cryptographically tied to the artifact bytes.** The release
   TITLE holds ownership; a `release.published` event on the
   title's stream carries the artifact hash, commit, repo, and
   publish time. A tampered tarball fails hash-match and is
   rejected.
2. **Vulnerability reports are first-class, not side-channel.**
   A researcher's CVE report is a signed event on the release's
   stream, visible to every consumer at verification time. An
   unpatched HIGH-severity CVE triggers a `warn` verdict.
3. **Patches resolve the warning.** Once the maintainer emits
   `release.vulnerability-patched`, the verdict returns to accept.
4. **Revocation propagates as an event.** A compromised release
   can be revoked; every verifier sees the revocation and
   rejects the artifact from that point forward.
5. **Consumers retain policy control.** The trust threshold,
   severity threshold, and strict/warn mode are all local policy
   knobs on the verifier side, not centrally enforced.

## What's in this folder

| File | Purpose |
|---|---|
| `artifact_verify.py` | Pure decision logic. No SDK dep. Exports `ReleaseV1`, `verify_artifact`, `verify_batch`, plus event-stream helpers. |
| `artifact_verify_test.py` | 16 pytest cases covering hash match/mismatch, revocation, trust threshold, CVE severity handling, batch verification. |
| `demo.py` | End-to-end runnable against a live node. Ten steps exercising publish, verify, CVE report, patch, revoke, and hash-mismatch scenarios. |

## Running

```bash
# 1. Start a local node.
cd deploy/compose && docker compose up -d

# 2. Install Python SDK.
cd clients/python && pip install -e .

# 3. Run the demo.
cd examples/developer-artifact-signing
python demo.py
```

## Testing without a live node

```bash
cd examples/developer-artifact-signing
python -m pytest artifact_verify_test.py -v
```

## QDP catalog audit

| Feature | Purpose | Status |
|---|---|---|
| `IDENTITY` tx | Register maintainer, consumer, researcher | v1.0 |
| `TITLE` tx | Establish release ownership | v1.0 |
| `TRUST` tx | Consumer's direct trust in the maintainer | v1.0 |
| `EVENT` tx streams | Lifecycle: published, vuln-reported, patched, revoked | v1.0 |
| QDP-0002 guardian recovery | Key rotation after compromise | v1.0 (described in Step 9 commentary; not exercised) |
| QDP-0005 push gossip | Fast revocation propagation | v1.0 |
| QDP-0009 fork-block | Ecosystem-wide policy upgrades | v1.0 (not exercised) |
| QDP-0019 decay | Long-abandoned releases fade | Phase 1 landed; optional for this flow |

No protocol gaps. Everything the use case needs is a v1.0
primitive. The verifier is application-layer policy over those
primitives.

## What a production deployment would add

- **Multi-maintainer release signing via guardian quorum.** The
  demo has single-maintainer TITLEs. A real team-owned project
  would use a project quid with the maintainers as guardians
  (threshold 1 for routine release, higher for recovery), and
  each release TITLE would be transferred from the project quid
  via M-of-N signatures.
- **Sigstore / in-toto attestation bridge.** The `release.published`
  event payload has a `buildEnvironment` slot; production would
  add `buildProvenance` with a sigstore reference or an in-toto
  SLSA attestation, and the verifier would chase that down.
- **SBOM event type.** `release.sbom-attested` would carry the
  hash of an SPDX / CycloneDX SBOM, letting downstream tooling
  audit the full dependency tree.
- **Per-ecosystem fork-block activations** (QDP-0009) for
  ecosystem-wide upgrades like "as of block H, all npm releases
  must attest a sigstore bundle."
- **Decay on maintainer trust** (QDP-0019) so a maintainer who
  stops publishing for years doesn't retain high trust forever.

## Related

- Use case: [`UseCases/developer-artifact-signing/`](../../UseCases/developer-artifact-signing/)
- Protocol: [QDP-0002 Guardian Recovery](../../docs/design/0002-guardian-based-recovery.md)
- Related POC: [`examples/credential-verification-network/`](../credential-verification-network/)
  is the same transitive-trust pattern in a different domain
  (credentials rather than artifacts)
- Related POC: [`examples/ai-model-provenance/`](../ai-model-provenance/)
  (upcoming) applies the same supply-chain pattern to AI model
  weights
