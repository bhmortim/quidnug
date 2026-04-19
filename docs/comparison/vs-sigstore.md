# Quidnug vs. Sigstore + Fulcio

Sigstore is the "sign every artifact, no long-lived keys"
movement in the software-supply-chain space. cosign is the CLI.
Fulcio is the certificate authority. Rekor is the transparency
log. They're excellent, and Quidnug integrates with them rather
than competing.

## What Sigstore does well

- **Keyless signing.** Use an OIDC identity (GitHub,
  workload identity) to obtain a short-lived signing cert from
  Fulcio.
- **Transparency log.** Every signature is appended to Rekor, a
  public log.
- **Universal ergonomics.** `cosign sign image.tar` "just works."
- **Maturity.** Widely deployed; CNCF-graduated.
- **SLSA integration.** Native support for SLSA provenance
  attestations.

## What Sigstore doesn't do

- **Relational trust.** A cosign signature is "Fulcio says this
  certificate was issued to `alice@github`." Whether YOU trust
  `alice@github` at all is out of scope.
- **Per-observer scoring.** Sigstore verification is binary. You
  either trust the Fulcio root (everyone does) and the claimed
  OIDC subject (which is "them, at that moment"), or you don't.
- **Revocation composition.** cosign has no native way to say
  "accept signatures from X only if they're also vouched for by
  Y."
- **Non-artifact signing.** cosign is specifically for container
  images, blobs, attestations. Signing a "trust edge" or an
  "identity update" requires building your own wire format.

## What Quidnug adds

- **Score cosign signers.** Quidnug's [sigstore
  integration](../../integrations/sigstore/) records each
  verified cosign bundle as a signed EVENT on the artifact's
  Title. Downstream consumers can query relational trust to
  each signer.
- **Unified revocation.** Emit `SIGSTORE_REVOKED` events on the
  artifact's stream to instantly invalidate a specific signature
  in your audit.
- **Non-artifact signing.** Identity updates, trust edges, event
  streams — things Sigstore doesn't cover.

## The recommended architecture

**Use both.** Cosign signs your artifacts; Quidnug records the
signatures and layers per-viewer trust scoring.

```
 ┌──────────────────────────────────────────────────────────┐
 │ Build pipeline                                           │
 │   cosign sign image.tar --identity=alice@github          │
 │     └── Rekor entry (public log)                         │
 │     └── Fulcio short-lived cert                          │
 └──────────────────────────────────────────────────────────┘
                           │
                           ▼  record on Quidnug
 ┌──────────────────────────────────────────────────────────┐
 │ integrations/sigstore/ → EVENT on the artifact's Title   │
 │   event_type = SIGSTORE_SIGNATURE                        │
 │   payload = { signer, cert, signedAt, bundleUri }        │
 └──────────────────────────────────────────────────────────┘
                           │
                           ▼  downstream query
 ┌──────────────────────────────────────────────────────────┐
 │ Consumer verifies AND scores:                            │
 │   1. cosign verify (Rekor + Fulcio)                      │
 │   2. Quidnug trust query: my_quid -> signer_quid         │
 │   3. Accept iff both pass                                │
 └──────────────────────────────────────────────────────────┘
```

See [`integrations/sigstore/`](../../integrations/sigstore/) for
the working Go integration with end-to-end tests.

## When to pick which

### Use Sigstore alone when

- You just need to sign container images / build artifacts.
- "Signed by @alice-github" is enough — you don't need
  transitive / relational scoring.
- You don't need non-artifact signing.

### Use Sigstore + Quidnug when

- You want transitive trust scoring: "Accept this attestation
  only if its signer is transitively trusted by my org at ≥
  0.7."
- You have multiple issuers and want a unified revocation /
  audit log across them.
- You're building software-supply-chain dashboards and want to
  show trust confidence per artifact.

### Use Quidnug alone when

- Your signing concerns are outside Sigstore's scope (trust
  edges, identity updates, event streams).
- Your artifacts live outside containerized workflows.

## License

Apache-2.0.
