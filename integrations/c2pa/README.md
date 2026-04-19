# Quidnug × C2PA integration

Record C2PA manifests (Content Provenance & Authenticity) in Quidnug
event streams so per-observer trust applies to creator/editor chains.

```go
rec, _ := c2pa.New(c2pa.Options{ Client: c, Domain: "media.example.com" })
_, err := rec.RecordManifest(ctx, assetOwner, c2pa.Manifest{
    AssetID:        "asset-title-id",
    Format:         "image/jpeg",
    Title:          "drone-shot-2024-06.jpg",
    ClaimGenerator: "Adobe Photoshop 25.2",
    Signer:         "creator@news.example.com",
    SignedAt:       time.Now().Unix(),
    Assertions: []c2pa.Assertion{
        {Label: "c2pa.actions", Data: map[string]any{"actions": []any{
            map[string]any{"action": "c2pa.created"},
            map[string]any{"action": "c2pa.cropped"},
        }}},
        {Label: "c2pa.training-mining", Data: map[string]any{"mining": "notAllowed"}},
    },
})
```

Verification is NOT done here — run `c2pa-rs` or `c2pa-js` upstream
before recording.

## Why record C2PA in Quidnug

- **Per-observer trust**: "do I trust this claim generator at least
  0.7?" is a Quidnug trust query.
- **Cross-org lineage**: if asset A lists asset B as an ingredient,
  and B's manifest was signed by someone I transitively trust, that
  trust applies transitively in the Quidnug graph.
- **Tamper-evident audit**: Quidnug's Merkle-rooted block structure
  makes the recorded manifest itself immutable and inclusion-provable.

## License

Apache-2.0.
