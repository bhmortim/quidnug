# Developer Artifact Signing

**Open source · Supply-chain security · GPG replacement**

## The problem

Every week: another popular npm / PyPI / crate package's
maintainer reports "I lost my signing key" or "my key was
compromised — invalidate everything I've published."

The status quo for open-source artifact signing:

- **GPG keys** still dominate. A single key, stored God-knows-where
  by the maintainer. Lost key = downstream chaos. Compromised
  key = everyone downstream has to re-verify against a new
  key.
- **Sigstore / cosign** is a major improvement (short-lived
  certificates tied to OIDC identity) but centralizes trust in
  Fulcio / Rekor. Also doesn't address the "what if the
  maintainer leaves the project" or "what if there are
  multiple maintainers" case.
- **Package-registry-level signatures** (npm signed registry)
  tie signing to the registry; the registry becomes a single
  point.

What's actually needed:

1. **Guardian-recoverable signing keys.** Lose your HSM? Your
   team's chosen guardians can rotate you to a new key without
   nuking all your downstream consumers.
2. **Multi-maintainer signing.** A package maintained by 3
   people should be able to publish from any of them, without
   each consumer having to manage 3 separate GPG keys.
3. **Cryptographic chain of releases.** The v2.0.1 release
   was signed by a key that was a rotation from v2.0.0's key.
   Downstream consumers can follow the chain.
4. **Revocation that actually propagates.** Compromised key?
   Revocation visible to every consumer within seconds, not
   "they need to notice the Twitter post."
5. **Ecosystem-wide upgrades.** "As of January 2027, all
   packages in this ecosystem must attest their build via
   sigstore" — coordinated activation across millions of
   downstream consumers.

## Why Quidnug fits

A package maintainer is a quid. Their project is a quid (owned
by the maintainer's team). Each release is a title. Guardian
recovery handles key loss. Events track the release lifecycle.

| Problem                                       | Quidnug primitive                              |
|-----------------------------------------------|------------------------------------------------|
| "Lost signing key"                            | Guardian recovery (co-maintainers as guardians)|
| "Multi-maintainer signing"                    | Guardian set of maintainers, threshold 1+       |
| "Release chain continuity"                    | Anchor rotation chain                           |
| "Revoke compromised release"                  | `release.revoked` event on the title            |
| "Coordinate ecosystem upgrade"                | Fork-block transaction                          |
| "Consumer trust in maintainer"                | Relational trust edges                         |
| "Cross-registry signing"                      | One quid identity, multiple registry mappings   |

## High-level architecture

```
      ┌───────────────────────────────────────────┐
      │     Developer / Org Quids                 │
      │                                            │
      │  Alice's quid (maintainer of "webapp-js")  │
      │    GuardianSet: {Bob, Carol, backup-HSM}   │
      │    Threshold: 1    (for routine release)   │
      │    Recovery: {Bob+Carol req'd,  delay=24h}│
      └───────────────────────────────────────────┘
                            │
                            │ publishes signed TITLE
                            ▼
      ┌───────────────────────────────────────────┐
      │ Release title:                             │
      │ "webapp-js@2.3.1"                          │
      │   attributes:                              │
      │     - artifactHash: <sha256 of tarball>    │
      │     - version: "2.3.1"                     │
      │     - repository: github.com/acme/webapp   │
      │     - commitHash: <git sha>                │
      └───────────────────────────────────────────┘
                            │
      ┌─────────────────────┴───────────────────┐
      │            Event stream:                 │
      │   - release.published                    │
      │   - release.sbom-attested                │
      │   - release.vulnerability-reported       │
      │   - release.revoked                     │
      └─────────────────────────────────────────┘
                            │
                            ▼
         Downstream consumers verify via Quidnug
```

## Data model

### Quids
- **Developer** — individual maintainer. HSM/hardware key for
  signing; co-maintainers as recovery guardians.
- **Project** — team-owned artifact namespace.
- **Organization** — for corporate OSS (e.g., `apache-software-
  foundation`).
- **Release registry** — npm, PyPI, crates, Maven Central. Each
  has a quid for its own signing role.

### Domain

```
developer.signing.npm
developer.signing.pypi
developer.signing.crates
developer.signing.maven-central
developer.signing.github-releases
```

### Release title

```json
{
  "type":"TITLE",
  "assetId":"webapp-js-2.3.1",
  "domain":"developer.signing.npm",
  "titleType":"software-release",
  "owners":[{"ownerId":"maintainer-alice","percentage":100.0}],
  "attributes":{
    "packageName":"webapp-js",
    "version":"2.3.1",
    "artifactHash":"<sha256 of tarball>",
    "repository":"github.com/acme/webapp-js",
    "commitHash":"abc123...",
    "buildEnvironment":"github-actions-ubuntu-22.04",
    "buildProvenance":"<sigstore reference>",
    "publishedAt":1713400000,
    "previousReleaseRef":"webapp-js-2.3.0"  /* chain link */
  },
  "signatures":{"maintainer-alice":"<sig>"}
}
```

### Release lifecycle events

```
1. release.published
   payload: { version, artifactHash, buildLogHash }
   signer: maintainer

2. release.sbom-attested
   payload: { sbomHash, componentAnalysisHash }
   signer: maintainer (or CI system)

3. release.vulnerability-reported (by security researcher)
   payload: { cveId, severity, affectedVersions }
   signer: reporter-quid

4. release.vulnerability-patched
   payload: { cveId, patchCommit, patchedInVersion }
   signer: maintainer

5. release.revoked
   payload: { reason: "key-compromise", revokedAt }
   signer: maintainer (or post-key-recovery, successor)
```

## Multi-maintainer project flow

Project maintained by Alice, Bob, Carol. Each is a quid.
The project quid has guardian set:

```json
{
  "subjectQuid":"project-webapp-js",
  "newSet":{
    "guardians":[
      {"quid":"alice","weight":1},
      {"quid":"bob","weight":1},
      {"quid":"carol","weight":1}
    ],
    "threshold":1,  /* any one maintainer can publish */
    "recoveryDelay":86400000000000,  /* 24h */
    "requireGuardianRotation":true
  }
}
```

Any of Alice, Bob, or Carol can publish a release (threshold 1).
But recovery (changing the maintainer set, rotating to new
keys) requires a quorum among the remaining maintainers.

## Key loss: Alice loses her HSM

Alice is the lead maintainer; her HSM died. Her co-maintainers
initiate guardian recovery to give her a new key:

```bash
curl -X POST $NODE/api/v2/guardian/recovery/init -d '{
  "subjectQuid":"maintainer-alice",
  "fromEpoch":0,
  "toEpoch":1,
  "newPublicKey":"<Alice'\''s new HSM pub key>",
  "minNextNonce":1,
  "maxAcceptedOldNonce":0,  /* revoke all old-epoch sigs */
  "guardianSigs":[
    {"guardianQuid":"bob","keyEpoch":0,"signature":"<sig>"},
    {"guardianQuid":"carol","keyEpoch":0,"signature":"<sig>"}
  ],
  ...
}'
```

24-hour delay (since this is a high-stakes key). If Alice's
HSM is genuinely dead, no one vetoes. Post-commit, Alice's
epoch advances.

Downstream consumers querying for "alice's current signing
key" see the new one automatically via Quidnug. No downstream
config change needed.

## Consumer verification

```go
type ArtifactVerifier struct {
    client   QuidnugClient
    selfQuid string
}

func (v *ArtifactVerifier) Verify(ctx context.Context, packageName, version string, artifactBytes []byte) (*VerifyResult, error) {
    // Query for the release title
    releaseID := packageName + "-" + version
    title, err := v.client.GetTitle(ctx, releaseID)
    if err != nil {
        return nil, err
    }

    // Verify artifact hash matches
    expectedHash := title.Attributes["artifactHash"].(string)
    actualHash := sha256sum(artifactBytes)
    if expectedHash != actualHash {
        return &VerifyResult{Valid: false, Reason: "Artifact hash mismatch"}, nil
    }

    // Check maintainer trust
    maintainer := title.Owners[0].OwnerID
    trust, err := v.client.GetTrust(ctx, v.selfQuid, maintainer,
        title.Domain, nil)
    if err != nil || trust.TrustLevel < 0.5 {
        return &VerifyResult{Valid: false, Reason: "Maintainer trust too low"}, nil
    }

    // Check for revocation
    events, _ := v.client.GetSubjectEvents(ctx, releaseID, "TITLE")
    for _, ev := range events {
        if ev.EventType == "release.revoked" {
            return &VerifyResult{Valid: false, Reason: fmt.Sprintf("Release revoked: %s",
                ev.Payload["reason"])}, nil
        }
    }

    // Check for unpatched high-severity vulnerabilities
    hasUnpatchedHighSev := false
    for _, ev := range events {
        if ev.EventType == "release.vulnerability-reported" {
            severity := ev.Payload["severity"].(string)
            cveID := ev.Payload["cveId"].(string)
            // Was it patched?
            patched := hasPatchEvent(events, cveID)
            if severity == "HIGH" && !patched {
                hasUnpatchedHighSev = true
            }
        }
    }

    return &VerifyResult{
        Valid:               true,
        MaintainerTrust:     trust.TrustLevel,
        HasUnpatchedIssues:  hasUnpatchedHighSev,
    }, nil
}
```

## Ecosystem-wide upgrade

NPM ecosystem decides "effective block H, all packages must
include a sigstore attestation." Fork-block:

```bash
curl -X POST $NODE/api/v2/fork-block -d '{
  "trustDomain":"developer.signing.npm",
  "feature":"require_tx_tree_root",   /* or similar app-specific feature */
  "forkHeight":<future>,
  "signatures":[
    {"validatorQuid":"npm-foundation","keyEpoch":0,"signature":"<sig>"},
    {"validatorQuid":"github","keyEpoch":0,"signature":"<sig>"},
    {"validatorQuid":"eslint-maintainer-council","keyEpoch":0,"signature":"<sig>"}
  ]
}'
```

At the fork height, every downstream consumer's verifier
automatically enforces the new requirement.

## Key Quidnug features

- **Guardian recovery (QDP-0002)** — maintainer's co-maintainers
  are their recovery guardians.
- **Anchor rotation** — clean chain from old to new signing
  key.
- **Event streams** — release history + security lifecycle.
- **Relational trust** — consumers trust specific maintainers.
- **Push gossip (QDP-0005)** — revocation propagates in
  seconds.
- **Fork-block (QDP-0009)** — ecosystem-wide upgrades.
- **Domain hierarchy** — per-registry scoping.

## Value delivered

| Dimension                          | GPG / sigstore                       | With Quidnug                                          |
|------------------------------------|--------------------------------------|-------------------------------------------------------|
| Key loss recovery                  | Full re-keying by all consumers      | Guardian recovery; consumers auto-resolve new key      |
| Multi-maintainer signing           | Ad-hoc workarounds                   | First-class guardian set                               |
| Revocation propagation             | Twitter + GitHub issue               | Seconds via push gossip                                |
| Release chain continuity           | Manual "this key replaces X"         | Anchor rotation chain                                  |
| Consumer customization             | All-or-nothing                       | Relational trust per maintainer                        |
| Ecosystem coordination             | None                                 | Fork-block activations                                 |
| SBOM / vulnerability tracking      | Tool-specific                        | Native events on release stream                        |
| Cross-registry identity            | Separate keys per registry           | One quid, multiple registry mappings                    |

## What's in this folder

- [`README.md`](README.md)
- [`implementation.md`](implementation.md)
- [`threat-model.md`](threat-model.md)

## Related

- [`../ai-model-provenance/`](../ai-model-provenance/) — same supply-chain pattern
- [`../institutional-custody/`](../institutional-custody/) — full key lifecycle
- [QDP-0002 Guardian Recovery](../../docs/design/0002-guardian-based-recovery.md)
- [QDP-0009 Fork-Block Trigger](../../docs/design/0009-fork-block-trigger.md)
