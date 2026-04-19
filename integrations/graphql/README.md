# Quidnug GraphQL gateway (scaffold)

Status: **SCAFFOLD.**

A GraphQL gateway in front of the Quidnug HTTP API, giving frontend
teams a single-round-trip interface to compose complex queries
(trust + identity + stream + title) into one request.

## Planned schema (excerpt)

```graphql
type Quid {
    id: ID!
    identity: IdentityRecord
    guardianSet: GuardianSet
    outboundTrust(domain: String!): [TrustEdge!]!
    trustTo(target: ID!, domain: String!, maxDepth: Int = 5): TrustResult!
    events(first: Int = 50, after: String): EventConnection!
}

type TrustResult {
    trustLevel: Float!
    path: [ID!]!
    pathDepth: Int!
}

type Query {
    quid(id: ID!): Quid
    title(assetId: ID!, domain: String): Title
}

type Mutation {
    grantTrust(input: GrantTrustInput!): TrustTxReceipt!
    registerIdentity(input: RegisterIdentityInput!): IdentityTxReceipt!
    emitEvent(input: EmitEventInput!): EventTxReceipt!
}
```

## Roadmap

1. Implement using the Go reference via gqlgen — consume `pkg/client`
   under the hood.
2. Implement DataLoader batching so a single GraphQL query fans out
   to one HTTP call per unique endpoint.
3. Ship Apollo Server + URQL + Relay codegen recipes.

## License

Apache-2.0.
