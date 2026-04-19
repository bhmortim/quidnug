# Elastic / OpenSearch integration (scaffold)

Status: **SCAFFOLD.**

Bulk-index Quidnug event streams into Elastic/OpenSearch for
full-text search, Kibana dashboards, and cross-stream correlation.

## Planned pipeline

```
Quidnug node ──websocket stream── ingester ──bulk API──► Elastic
  (live events)                       │
                                       └─ enriches with trust scores
                                         from live trust query
```

## Index mapping excerpt

```json
{
  "mappings": {
    "properties": {
      "subjectId":    { "type": "keyword" },
      "subjectType":  { "type": "keyword" },
      "eventType":    { "type": "keyword" },
      "timestamp":    { "type": "date", "format": "epoch_second" },
      "payload":      { "type": "object", "dynamic": true },
      "signer":       { "type": "keyword" },
      "trust": {
        "properties": {
          "observer":   { "type": "keyword" },
          "score":      { "type": "half_float" },
          "pathDepth":  { "type": "byte" }
        }
      }
    }
  }
}
```

The trust sub-object is computed at ingest per event, for a
configured set of "interesting" observers. Downstream Kibana
queries filter by `trust.observer` + `trust.score > 0.7`.

## Roadmap

1. Ingester service under `integrations/elastic/ingester/`.
2. Kibana dashboard export mirroring `deploy/observability/`.
3. Elastic Common Schema (ECS) field mapping.

## License

Apache-2.0.
