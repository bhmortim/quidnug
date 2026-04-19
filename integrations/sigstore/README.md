# Quidnug × Sigstore integration

`github.com/quidnug/quidnug/integrations/sigstore` mirrors cosign /
sigstore artifact signatures into Quidnug event streams, turning
supply-chain signatures into queries over the Quidnug trust graph.

## Why

Classic cosign verification asks a binary question: "did a trusted
Fulcio identity sign this?" Quidnug's relational trust asks a richer
one: "from my perspective, how many hops away is the signer, and at
what decayed trust level?" This integration records every cosign
signature as a Quidnug EVENT on the artifact's Title, so you can:

- Enumerate every signer of an artifact (stream events).
- Score them via relational trust (SDK `get_trust` calls).
- Filter attestations to those signed by parties your org transitively
  trusts at ≥ N.

## Usage

```go
import (
    "context"
    "os"

    "github.com/quidnug/quidnug/integrations/sigstore"
    "github.com/quidnug/quidnug/pkg/client"
)

func main() {
    ctx := context.Background()
    c, _ := client.New("http://quidnug.local:8080")

    rec, _ := sigstore.New(sigstore.Options{
        Client: c,
        Domain: "supplychain.example.com",
    })

    owner, _ := client.QuidFromPrivateHex(os.Getenv("ARTIFACT_OWNER_KEY"))
    raw, _ := os.ReadFile("bundle.json")
    bundle, _ := sigstore.BundleFromCosignJSON("artifact-title-id", raw)

    _, _ = rec.RecordBundle(ctx, owner, bundle)
}
```

## What is / isn't verified

This package does **not** re-verify the cosign signature. Do that
first with the official sigstore library, e.g.:

```go
import "github.com/sigstore/sigstore-go/pkg/verify"
// ... verify.Verify(sigBundle, ...)
```

Only record the bundle after verification passes. Recording an
unverified bundle would let a malicious publisher poison the trust
graph with fake attestations.

## Event shape

Every recorded bundle becomes one EVENT:

```json
{
  "type": "EVENT",
  "subjectId":   "<artifact title id>",
  "subjectType": "TITLE",
  "eventType":   "SIGSTORE_SIGNATURE",
  "payload": {
    "schema":         "sigstore-bundle/v0.2",
    "artifactDigest": "sha256:…",
    "signature":      "<base64>",
    "certificate":    "-----BEGIN CERTIFICATE-----\n…",
    "signer":         "alice@example.com",
    "signedAt":       1700000000,
    "bundleUri":      "https://rekor.sigstore.dev/..."
  }
}
```

Consumers fetch the stream via `GET /api/streams/{artifactTitleId}/events`
and filter by `eventType=SIGSTORE_SIGNATURE`.

## Query recipes

```go
// 1. All attestations on an artifact
events, _, _ := c.GetStreamEvents(ctx, "artifact-title-id", "supplychain.example.com", 100, 0)

// 2. Signers with relational trust >= 0.5 from us
for _, ev := range events {
    signer := ev.Payload["signer"].(string)
    tr, _ := c.GetTrust(ctx, myQuid.ID, signer, "supplychain.example.com", 5)
    if tr.TrustLevel >= 0.5 {
        // accept this signature
    }
}
```

## License

Apache-2.0.
