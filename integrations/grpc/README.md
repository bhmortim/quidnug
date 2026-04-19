# Quidnug gRPC gateway (scaffold)

Status: **SCAFFOLD.** The Go reference node currently speaks HTTP/JSON
only. This scaffold captures the planned gRPC surface so protobuf
consumers (Envoy, Linkerd, grpc-gateway, Buf generators) have a
stable target.

## Proto files

```
integrations/grpc/
├── proto/quidnug/v2/
│   ├── identity.proto       # RegisterIdentity, GetIdentity
│   ├── trust.proto          # GrantTrust, GetTrust, GetTrustEdges
│   ├── title.proto          # RegisterTitle, GetTitle
│   ├── events.proto         # EmitEvent, GetEventStream, GetStreamEvents
│   ├── guardian.proto       # GuardianSetUpdate, Recovery*, GetGuardianSet
│   ├── gossip.proto         # SubmitAnchorGossip, SubmitDomainFingerprint
│   ├── bootstrap.proto      # SubmitNonceSnapshot, BootstrapStatus
│   ├── fork_block.proto     # SubmitForkBlock, ForkBlockStatus
│   └── merkle.proto         # VerifyInclusionProof (client-side helper)
└── README.md
```

## Roadmap

1. Author proto schemas mirroring `schemas/json/*.schema.json`.
2. Expose the gateway as a sidecar process using grpc-gateway so the
   node keeps its HTTP/JSON surface while the gateway translates
   gRPC ↔ HTTP under the hood.
3. Generated clients: Go, Python, Java, Kotlin, Swift, Rust, C#.
4. Buf registry publishing: `buf.build/quidnug/quidnug/v2`.

## License

Apache-2.0.
