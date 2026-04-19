# Implementation: Developer Artifact Signing

## 1. Maintainer + project identity

```bash
# Each maintainer is a quid
curl -X POST $NODE/api/identities -d '{
  "quidId":"maintainer-alice",
  "name":"Alice (maintainer)",
  "homeDomain":"developer.signing.npm",
  "creator":"maintainer-alice","updateNonce":1
}'

# Alice's personal guardian set (for key recovery)
curl -X POST $NODE/api/v2/guardian/set-update -d '{
  "subjectQuid":"maintainer-alice",
  "newSet":{
    "guardians":[
      {"quid":"bob","weight":1,"epoch":0},
      {"quid":"carol","weight":1,"epoch":0},
      {"quid":"alice-backup-hsm","weight":1,"epoch":0}
    ],
    "threshold":2,
    "recoveryDelay":86400000000000,  /* 24h */
    "requireGuardianRotation":true
  },
  ...
}'

# The project itself
curl -X POST $NODE/api/identities -d '{
  "quidId":"project-webapp-js",
  "name":"webapp-js (npm package)",
  "homeDomain":"developer.signing.npm",
  "creator":"maintainer-alice","updateNonce":1,
  "attributes":{
    "packageName":"webapp-js",
    "registry":"npm",
    "repository":"github.com/acme/webapp-js"
  }
}'

# Project's guardian set = maintainer team
curl -X POST $NODE/api/v2/guardian/set-update -d '{
  "subjectQuid":"project-webapp-js",
  "newSet":{
    "guardians":[
      {"quid":"maintainer-alice","weight":1},
      {"quid":"maintainer-bob","weight":1},
      {"quid":"maintainer-carol","weight":1}
    ],
    "threshold":1,
    "recoveryDelay":86400000000000,
    "requireGuardianRotation":true
  },
  ...
}'
```

## 2. Publish a release

```bash
# Alice publishes v2.3.1
curl -X POST $NODE/api/v1/titles -d '{
  "assetId":"webapp-js-2.3.1",
  "domain":"developer.signing.npm",
  "titleType":"software-release",
  "owners":[{"ownerId":"project-webapp-js","percentage":100.0}],
  "attributes":{
    "packageName":"webapp-js",
    "version":"2.3.1",
    "artifactHash":"<sha256 of .tgz>",
    "repository":"github.com/acme/webapp-js",
    "commitHash":"abc123...",
    "buildEnvironment":"github-actions-ubuntu-22.04",
    "buildLogHash":"<sha256>",
    "previousReleaseRef":"webapp-js-2.3.0",
    "sbomCID":"bafy..."
  },
  "creator":"maintainer-alice",
  "signatures":{"maintainer-alice":"<sig>"}
}'

# Published event
curl -X POST $NODE/api/v1/events -d '{
  "subjectId":"webapp-js-2.3.1",
  "subjectType":"TITLE",
  "eventType":"release.published",
  "payload":{
    "publisher":"maintainer-alice",
    "timestamp":1713400000,
    "changelog":"<url>",
    "npmTag":"latest"
  },
  "creator":"maintainer-alice","signature":"<sig>"
}'
```

## 3. SBOM attestation

```bash
curl -X POST $NODE/api/v1/events -d '{
  "subjectId":"webapp-js-2.3.1",
  "subjectType":"TITLE",
  "eventType":"release.sbom-attested",
  "payload":{
    "sbomFormat":"CycloneDX-1.5",
    "sbomHash":"<sha256>",
    "sbomCID":"bafy...",
    "dependencyCount":247
  },
  "creator":"maintainer-alice","signature":"<sig>"
}'
```

## 4. Vulnerability reporting

Security researcher files a CVE:

```bash
curl -X POST $NODE/api/v1/events -d '{
  "subjectId":"webapp-js-2.3.1",
  "subjectType":"TITLE",
  "eventType":"release.vulnerability-reported",
  "payload":{
    "reporter":"security-researcher-x",
    "cveId":"CVE-2026-1234",
    "severity":"HIGH",
    "affectedVersions":"^2.3.0",
    "proofOfConceptCID":"bafy..."
  },
  "creator":"security-researcher-x","signature":"<sig>"
}'
```

Maintainer patches:

```bash
curl -X POST $NODE/api/v1/events -d '{
  "subjectId":"webapp-js-2.3.1",
  "subjectType":"TITLE",
  "eventType":"release.vulnerability-patched",
  "payload":{
    "cveId":"CVE-2026-1234",
    "patchCommit":"def456",
    "patchedInVersion":"2.3.2"
  },
  "creator":"maintainer-alice","signature":"<sig>"
}'
```

## 5. Key rotation (scheduled)

Alice rotates her signing key every 6 months as policy:

```bash
curl -X POST $NODE/api/anchors -d '{
  "kind":"rotation",
  "signerQuid":"maintainer-alice",
  "fromEpoch":0,"toEpoch":1,
  "newPublicKey":"<hex>",
  "minNextNonce":1,
  "maxAcceptedOldNonce":1000,  /* grace for in-flight */
  "anchorNonce":<next>,
  "signature":"<signed with current epoch key>"
}'
```

## 6. Key loss + recovery

Alice's HSM failed. Bob and Carol initiate recovery:

```bash
curl -X POST $NODE/api/v2/guardian/recovery/init -d '{
  "subjectQuid":"maintainer-alice",
  "fromEpoch":1,
  "toEpoch":2,
  "newPublicKey":"<Alice'\''s new HSM>",
  "minNextNonce":1,
  "maxAcceptedOldNonce":0,
  "guardianSigs":[
    {"guardianQuid":"bob","keyEpoch":0,"signature":"<sig>"},
    {"guardianQuid":"carol","keyEpoch":0,"signature":"<sig>"}
  ],
  ...
}'
```

24h delay; if no veto, commit. Post-commit, Alice signs new
releases with her epoch-2 key.

## 7. Consumer verification

```go
type PackageVerifier struct {
    client   QuidnugClient
    selfQuid string
}

func (v *PackageVerifier) VerifyInstall(ctx context.Context, packageName, version string, artifactBytes []byte) (*VerifyResult, error) {
    releaseID := fmt.Sprintf("%s-%s", packageName, version)
    title, err := v.client.GetTitle(ctx, releaseID)
    if err != nil {
        return nil, err
    }

    // Hash check
    expected := title.Attributes["artifactHash"].(string)
    if sha256sum(artifactBytes) != expected {
        return &VerifyResult{Valid: false, Reason: "Artifact hash mismatch"}, nil
    }

    // Trust in project
    projectID := title.Owners[0].OwnerID
    trust, _ := v.client.GetTrust(ctx, v.selfQuid, projectID,
        title.Domain, &GetTrustOptions{MaxDepth: 3})
    if trust.TrustLevel < 0.5 {
        return &VerifyResult{Valid: false, Reason: "Project trust too low"}, nil
    }

    // Revocation check
    events, _ := v.client.GetSubjectEvents(ctx, releaseID, "TITLE")
    for _, ev := range events {
        if ev.EventType == "release.revoked" {
            return &VerifyResult{Valid: false, Reason: "Release revoked"}, nil
        }
    }

    return &VerifyResult{Valid: true, Trust: trust.TrustLevel}, nil
}
```

## 8. npm install integration (sketch)

```bash
# Hypothetical npm-quidnug plugin
npm install --with-quidnug-verify webapp-js
# ... plugin queries Quidnug for the release title, verifies
#     artifact hash, checks trust, proceeds with install
```

## 9. Testing

```go
func TestArtifact_VerifyHashMatch(t *testing.T) {
    // Publish release with hash H
    // Verify with tampered bytes → fails
    // Verify with correct bytes → passes
}

func TestArtifact_GuardianKeyRecovery(t *testing.T) {
    // Alice publishes v1.0 with epoch-0 key
    // Guardian recovery to epoch-1
    // Alice publishes v1.1 with epoch-1 key
    // Consumer verifies both (historical v1.0 under old epoch;
    //                         current v1.1 under new epoch)
}

func TestArtifact_RevocationPropagation(t *testing.T) {
    // Publish, then revoke
    // Consumer sees revoked state within gossip window
}

func TestArtifact_MultiMaintainer(t *testing.T) {
    // Alice, Bob, Carol all in guardian set (threshold 1)
    // Any can publish a valid release
    // Consumer verifies regardless of which maintainer signed
}
```

## Where to go next

- [`threat-model.md`](threat-model.md)
- [`../ai-model-provenance/`](../ai-model-provenance/) — same supply-chain pattern for AI
