# AI Content Authenticity

**AI · Media provenance · C2PA+ · Editing chain of custody**

## The problem

Photos, videos, and audio are becoming indistinguishable from
synthetic generation. The C2PA (Coalition for Content
Provenance and Authenticity) standard embeds signed metadata
in media files claiming "this was captured by camera X at
time T", but it has gaps:

1. **Identity root of trust.** C2PA manifests are signed by
   a cert chain. Who validates the chain and says "yes, this
   cert is really Reuters'"? In practice, a handful of
   centralized issuers — reverting to a PKI trust model.
2. **Editing trust.** A news photo is captured, cropped,
   color-graded, and published. Each edit adds a C2PA
   signature, but there's no clear "did I trust this specific
   editor's cert" answer.
3. **Cross-platform reuse.** A photo captured on a Canon
   camera, edited in Adobe Lightroom, and published to a news
   site crosses three trust domains. C2PA handles the
   chain but consumers don't see the trust implications
   differently.
4. **Revocation is slow.** If a camera maker's cert is
   compromised, revoking it takes days; everything signed in
   the interim is ambiguous.
5. **Per-consumer trust.** A news organization trusts
   Reuters editors; a meme site trusts its own community.
   C2PA treats trust as binary (signature valid or not).

## Why Quidnug fits

Media assets have natural identities (the asset itself is a
thing), and provenance is a chain of signed operations on
that thing. That's title + events + relational trust.

| Problem                                     | Quidnug primitive                             |
|---------------------------------------------|-----------------------------------------------|
| "Is this the original capture?"             | TITLE with camera manufacturer as creator     |
| "Has it been edited?"                       | `capture`, `crop`, `grade`, `publish` events |
| "Who edited it and when?"                   | Event signed by editor's quid                 |
| "Does this consumer trust that editor?"     | Relational trust                             |
| "Compromised editor's key"                  | Guardian recovery rotates                     |
| "Revoke a whole issuer line"                | Trust edge set to 0; gossip propagates        |
| "Cross-platform trust federation"           | Domain hierarchy + cross-domain anchors       |

## High-level architecture

```
               ┌─────────────────────────────────────┐
               │   media.provenance.news (domain)     │
               │                                       │
               │   Trust roots: Reuters, AP, camera   │
               │   manufacturers, editing software    │
               │   providers                           │
               └─────────────────────────────────────┘

                       Asset-level flow:

  Camera ──────capture──────> Asset (TITLE)
     │                             │
     │                             │ Edit events:
     ▼                             ▼
  Camera's quid              ┌──────────┐
  signs the                  │ crop     │ ← photographer's quid
  initial capture            │ grade    │ ← editor's quid
                             │ overlay  │
                             │ caption  │
                             └──────────┘
                                   │
                                   ▼
                             ┌──────────┐
                             │ publish  │ ← news org's quid
                             └──────────┘
```

## Data model

### Quids
- **Camera manufacturer** (e.g., Canon, Sony, RED). Each signs
  a capture attestation event.
- **Photographer/videographer**: individual who captured.
- **Editor** (human or software): performs edits.
- **Publisher** (news org, agency, platform): final endorser.
- **Fact-checker** / **integrity-assessor**: independent
  validators that sign assessments.

### Domain
```
media.provenance
├── media.provenance.news
├── media.provenance.entertainment
├── media.provenance.evidentiary         (legal, police body cams)
└── media.provenance.social              (user-generated)
```

### Asset as title

```json
{
  "type":"TITLE",
  "assetId":"photo-canon-capture-a1b2c3",
  "domain":"media.provenance.news",
  "titleType":"media-asset",
  "owners":[{"ownerId":"photographer-jane-doe","percentage":100.0}],
  "attributes":{
    "assetType":"photo",
    "format":"JPEG",
    "capturedAt":1713400000,
    "captureDevice":"canon-5d-mark-iv-serial-123",
    "captureGeoHash":"9q8zn..."   /* or "hidden" */,
    "assetContentHash":"<sha256 of the original pixel data>",
    "captureLocation":"Austin, TX",
    "cameraSignatureAlgorithm":"ECDSA-P256"
  },
  "creator":"photographer-jane-doe",
  "signatures":{
    "photographer-jane-doe":"<sig>"
  }
}
```

### Capture event

Cameras with C2PA hardware sign a capture event as soon as the
shutter fires:

```
eventType: "media.captured"
subjectId: "photo-canon-capture-a1b2c3"
payload:
  captureDevice: "canon-5d-mark-iv-serial-123"
  originalHash: <sha256 of raw image>
  captureParams: { iso, aperture, shutter, lens }
  timestamp: 1713400000
signer: canon-5d-mark-iv-serial-123  (each camera has a quid,
                                       signed with per-device key)
```

The **camera itself is a quid** with its own signing key.
Canon operates guardian recovery infrastructure for when a
camera's key chips fail.

### Edit events

Each edit is a signed event:

```
eventType: "media.cropped"
payload:
  editor: "photographer-jane-doe"
  cropBox: { x1: 100, y1: 200, x2: 2000, y2: 1500 }
  inputHash: <prior state>
  outputHash: <new state>
  software: "Adobe Lightroom 13.0.1"
signer: photographer-jane-doe

eventType: "media.color-graded"
payload:
  editor: "editor-reuters-staff-mark"
  gradeParams: { exposure: -0.3, shadows: +0.5, ... }
  inputHash, outputHash
signer: editor-reuters-staff-mark

eventType: "media.captioned"
payload:
  caption: "Protesters gather outside capitol building..."
signer: editor-reuters-staff-mark

eventType: "media.published"
payload:
  publisher: "reuters"
  storyID: "reuters-story-12345"
  publishedAt: 1713500000
signer: reuters
```

### Fact-checker endorsement (optional)

```
eventType: "media.fact-checked"
payload:
  checker: "fact-check-org-maldita"
  assessment: "consistent-with-contextual-evidence"
  evidence: <hash of fact-check report>
signer: fact-check-org-maldita
```

## Per-consumer trust evaluation

A news organization viewing an asset:

```go
type AssetTrust struct {
    AssetID     string
    CaptureTrust float64   // trust in camera manufacturer + photographer
    EditTrust    float64   // trust in all editors in the chain
    PublisherTrust float64
    FactCheckBonus float64 // additional if trusted fact-checker endorses
    Overall      float64
}

func (org *NewsOrg) EvaluateAsset(ctx context.Context, assetID string) (AssetTrust, error) {
    title, _ := org.client.GetTitle(ctx, assetID)
    events, _ := org.client.GetSubjectEvents(ctx, assetID, "TITLE")

    result := AssetTrust{AssetID: assetID}

    // Capture step
    capturer := title.Creator
    captureTrust, _ := org.client.GetTrust(ctx, org.quid, capturer,
        "media.provenance.news", nil)
    result.CaptureTrust = captureTrust.TrustLevel

    // Editors — take MINIMUM trust across all editors in the chain
    // (a single low-trust editor taints the chain)
    minEditTrust := 1.0
    for _, ev := range events {
        if strings.HasPrefix(ev.EventType, "media.") &&
           ev.EventType != "media.captured" &&
           ev.EventType != "media.published" &&
           ev.EventType != "media.fact-checked" {
            editor := ev.Payload["editor"].(string)
            editorTrust, _ := org.client.GetTrust(ctx, org.quid, editor,
                "media.provenance.news", nil)
            if editorTrust.TrustLevel < minEditTrust {
                minEditTrust = editorTrust.TrustLevel
            }
        }
    }
    result.EditTrust = minEditTrust

    // Publisher
    for _, ev := range events {
        if ev.EventType == "media.published" {
            publisher := ev.Payload["publisher"].(string)
            pubTrust, _ := org.client.GetTrust(ctx, org.quid, publisher,
                "media.provenance.news", nil)
            result.PublisherTrust = pubTrust.TrustLevel
        }
    }

    // Fact-checker
    for _, ev := range events {
        if ev.EventType == "media.fact-checked" {
            checker := ev.Payload["checker"].(string)
            checkerTrust, _ := org.client.GetTrust(ctx, org.quid, checker,
                "media.provenance.news", nil)
            if checkerTrust.TrustLevel >= 0.8 {
                result.FactCheckBonus = 0.1
            }
        }
    }

    result.Overall = min(result.CaptureTrust, result.EditTrust, result.PublisherTrust) + result.FactCheckBonus
    return result, nil
}
```

Different consumers see different overall trust levels for the
same asset.

## AI-generated content

Synthetic content doesn't have a "camera" in the capture
step. Instead:

```
eventType: "media.generated"
payload:
  generator: "acme-image-model-v2"
  prompt: "Sunset over mountains"
  seed: 42
  guidanceScale: 7.5
  modelHash: <hash>
  outputHash: <hash>
signer: acme-image-model-v2  (the model itself is a quid)
```

Chain of editing can still proceed. A consumer can see
"this asset was originally generated by a model" and weigh
accordingly.

This composes with the [`ai-model-provenance`](../ai-model-provenance/)
use case: the model's own provenance chain is available.

## Revocation

Compromised camera key: Canon initiates guardian recovery
for the affected device's quid. Old-epoch signatures
stop validating.

Compromised publisher key: Reuters rotates via their own
guardian set. Old signatures are now from a stale epoch;
consumers that see them via gossip will probe and detect
the rotation.

Untrustworthy editor exposed: consumers lower trust edges
to that editor. Gossip propagates the change within
minutes.

## Key Quidnug features

- **Titles** for individual media assets.
- **Event streams** for the full edit chain.
- **Signed events** by each editor's quid.
- **Relational trust** for per-consumer trust evaluation.
- **Guardian recovery** for camera / editor / publisher key loss.
- **Push gossip** for rapid revocation propagation.
- **Domain hierarchy** for scoping (news vs. evidentiary vs.
  social).

## Value delivered

| Dimension                        | Before                                  | With Quidnug                                       |
|----------------------------------|-----------------------------------------|----------------------------------------------------|
| Capture attestation              | C2PA hardware certs (centralized PKI)   | Camera quid + guardian-recoverable key             |
| Editing chain                    | C2PA manifest (cert-chain based)        | Event stream with per-editor quids                 |
| Consumer-specific trust          | Binary (valid / invalid)                | Relational, per-domain                              |
| Revocation propagation            | Cert revocation lists (slow)            | Push gossip + trust-edge update                    |
| Cross-platform reuse             | Manual per-platform verification         | Single trust graph spans domains                    |
| Evidentiary-grade media          | Court-admissible chain of custody rare  | Protocol-level tamper-evident chain                |

## What's in this folder

- [`README.md`](README.md)
- [`implementation.md`](implementation.md) — concrete code
- [`threat-model.md`](threat-model.md) — security analysis

## Related

- [`../ai-model-provenance/`](../ai-model-provenance/) — for
  AI-generated content origins
- [QDP-0002 Guardian Recovery](../../docs/design/0002-guardian-based-recovery.md)
