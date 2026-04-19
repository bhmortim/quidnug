# Quidnug Go Client SDK

`github.com/quidnug/quidnug/pkg/client` — the official Go SDK for
[Quidnug](https://github.com/bhmortim/quidnug), a decentralized
protocol for relational, per-observer trust.

This package covers the full protocol surface: identity, trust,
titles, event streams, anchors, guardian sets, guardian recovery,
cross-domain gossip, K-of-K bootstrap, fork-block activation, and
compact Merkle inclusion proofs (QDPs 0001–0010).

## Install

```bash
go get github.com/quidnug/quidnug/pkg/client@latest
```

Requires Go 1.22+.

## Thirty-second example

```go
package main

import (
    "context"
    "fmt"
    "log"

    "github.com/quidnug/quidnug/pkg/client"
)

func main() {
    c, err := client.New("http://localhost:8080")
    if err != nil {
        log.Fatal(err)
    }
    ctx := context.Background()

    alice, _ := client.GenerateQuid()
    bob,   _ := client.GenerateQuid()

    c.RegisterIdentity(ctx, alice, client.IdentityParams{Name: "Alice", HomeDomain: "contractors.home"})
    c.RegisterIdentity(ctx, bob,   client.IdentityParams{Name: "Bob",   HomeDomain: "contractors.home"})
    c.GrantTrust(ctx, alice, client.TrustParams{
        Trustee: bob.ID, Level: 0.9, Domain: "contractors.home",
    })

    tr, _ := c.GetTrust(ctx, alice.ID, bob.ID, "contractors.home", 5)
    fmt.Printf("%.3f via %v\n", tr.TrustLevel, tr.Path)
}
```

More runnable examples live in [`examples/`](./examples/).

## What's in the package

### `Client`

HTTP client — all methods take `context.Context` and return typed
results. Safe for concurrent use by multiple goroutines.

| Area | Methods |
| --- | --- |
| Health / info | `Health`, `Info`, `Nodes` |
| Identity | `RegisterIdentity`, `GetIdentity` |
| Trust | `GrantTrust`, `GetTrust`, `GetTrustEdges` |
| Title | `RegisterTitle`, `GetTitle` |
| Events | `EmitEvent`, `GetEventStream`, `GetStreamEvents` |
| Guardians | `SubmitGuardianSetUpdate`, `SubmitRecoveryInit/Veto/Commit`, `SubmitGuardianResignation`, `GetGuardianSet`, `GetPendingRecovery` |
| Gossip | `SubmitDomainFingerprint`, `GetLatestDomainFingerprint`, `SubmitAnchorGossip`, `PushAnchor`, `PushFingerprint` |
| Bootstrap | `SubmitNonceSnapshot`, `GetLatestNonceSnapshot`, `BootstrapStatus` |
| Fork-block | `SubmitForkBlock`, `ForkBlockStatus` |
| Blocks | `GetBlocks`, `GetPendingTransactions` |
| Domains | `ListDomains` |

### `Quid` — cryptographic identity

```go
alice, _ := client.GenerateQuid()                    // fresh keypair
bob,   _ := client.QuidFromPrivateHex(storedHex)     // reconstruct
carol, _ := client.QuidFromPublicHex(networkPubHex)  // read-only

sig, _ := alice.Sign([]byte("data"))
alice.Verify([]byte("data"), sig)
```

ECDSA P-256 + SHA-256, DER-encoded hex signatures — byte-compatible
with the Python SDK and the Go reference node.

### `CanonicalBytes` / `VerifyInclusionProof`

Primitives exposed for advanced workflows:

```go
signable, _ := client.CanonicalBytes(tx, "signature", "txId")
sig, _ := signer.Sign(signable)

ok, _ := client.VerifyInclusionProof(
    canonicalTxBytes,
    gossipMsg.MerkleProof,
    originBlock.TransactionsRoot,
)
```

Canonicalization = round-trip-through-a-generic-object with alphabetized
keys in the second marshal. Matches every other Quidnug SDK
byte-for-byte. See `schemas/types/canonicalization.md` in the repo
root.

## Error handling

```go
_, err := c.GrantTrust(ctx, alice, params)

var ce *client.ConflictError
var ue *client.UnavailableError
var ne *client.NodeError

switch {
case errors.As(err, &ce):
    log.Printf("conflict: %s (%s)", ce.Error(), ce.Code())
case errors.As(err, &ue):
    log.Printf("node unavailable, retry later")
case errors.As(err, &ne):
    log.Printf("transport: HTTP %d", ne.StatusCode)
case errors.Is(err, client.ErrSDK):
    // Any SDK error
}
```

| Error | When |
| --- | --- |
| `*ValidationError` | Local precondition failed. |
| `*ConflictError` | Server logical rejection (nonce replay, quorum not met, …). |
| `*UnavailableError` | 503 / feature-not-active. |
| `*NodeError` | Transport / 5xx / unexpected shape. Carries `StatusCode` + `ResponseBody`. |
| `*CryptoError` | Signature verify / key derivation failed. |

`errors.Is(err, client.ErrSDK)` matches any SDK-raised error.

## Retry policy

- GETs: retried up to `WithMaxRetries(n)` (default 3) on 5xx and 429.
  Exponential backoff with ±100 ms jitter, capped at 60s, honors
  `Retry-After`.
- POSTs: **not** retried by default. Reconcile with a follow-up GET
  before retrying a write.

## Options

```go
c, _ := client.New("https://node.example.com",
    client.WithTimeout(60*time.Second),
    client.WithMaxRetries(5),
    client.WithRetryBaseDelay(500*time.Millisecond),
    client.WithAuthToken(os.Getenv("QUIDNUG_TOKEN")),
    client.WithUserAgent("my-app/1.0"),
    client.WithHTTPClient(customClient),
)
```

## Protocol version compatibility

| SDK | Node | QDPs covered |
| --- | --- | --- |
| v2 | v2 | 0001–0010 |

## Running the tests

```bash
go test ./pkg/client/...
```

## License

Apache-2.0.
