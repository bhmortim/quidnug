# Quidnug WebSocket stream subscriber (scaffold)

Status: **SCAFFOLD.**

Long-lived WebSocket push interface for real-time Quidnug event
streams, block updates, and guardian-lifecycle notifications.

## Planned protocol

```
Client → Server:
  { "type": "subscribe", "topic": "stream:<subjectId>", "since": <sequence> }
  { "type": "subscribe", "topic": "blocks:<domain>", "fromHeight": N }
  { "type": "subscribe", "topic": "guardian:<quidId>" }

Server → Client:
  { "type": "event",  "topic": "stream:…",    "payload": { event } }
  { "type": "block",  "topic": "blocks:…",    "payload": { block } }
  { "type": "ping" }   # every 30s
  { "type": "error",  "reason": "…" }
```

Transport: JSON over WS or MessagePack over WS, negotiated via the
`Sec-WebSocket-Protocol` header.

## Use cases

- React hook `useLiveStream(subjectId)` — real-time event feed
  without polling.
- Guardian dashboards that reflect set-updates and pending recoveries
  the instant they're accepted.
- Cross-domain bridges that react to anchor-gossip arrivals.

## Roadmap

1. Implement server-side WS endpoint at `/api/ws/stream` inside the
   Go node. Reuse existing event-stream + block-ledger internals.
2. Client ports to JS, Python, Go, Rust.
3. Backpressure + reconnection semantics documented in the protocol
   guide.

## License

Apache-2.0.
