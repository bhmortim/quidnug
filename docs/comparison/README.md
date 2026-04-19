# Quidnug compared to adjacent technologies

One of the first questions developers ask when evaluating Quidnug
is "how does this compare to X?" — where X is one of a familiar
set of tools. This directory collects the honest, side-by-side
comparisons.

All comparisons assume you're evaluating for a real system
(not a toy). Each page answers:

1. What does X solve that Quidnug also solves?
2. What does X solve that Quidnug doesn't (and isn't trying to)?
3. What does Quidnug solve that X doesn't?
4. When to pick which.

| Comparison | File |
| --- | --- |
| Quidnug vs. PGP Web of Trust | [`vs-pgp-wot.md`](vs-pgp-wot.md) |
| Quidnug vs. OAuth / OIDC | [`vs-oauth.md`](vs-oauth.md) |
| Quidnug vs. W3C DIDs + Verifiable Credentials | [`vs-did-vc.md`](vs-did-vc.md) |
| Quidnug vs. Ethereum / on-chain reputation | [`vs-blockchain.md`](vs-blockchain.md) |
| Quidnug vs. Sigstore + Fulcio | [`vs-sigstore.md`](vs-sigstore.md) |

## TL;DR heuristic

- **Need per-viewer trust (transitive, personal)?** → Quidnug.
- **Need a universal-agreed global reputation?** → you probably
  don't, but if you really do, use a blockchain.
- **Need "this person is the holder of credential X"?** → W3C
  VCs; record their issuance/revocation on Quidnug.
- **Need "this artifact was built by CI in this repo"?** →
  Sigstore; record the attestation on Quidnug.
- **Need federated login across apps?** → OAuth / OIDC; the OIDC
  bridge (`cmd/quidnug-oidc/`) maps them to Quidnug quids.
- **Need to replace PGP's Web of Trust with something modern?** →
  Quidnug is exactly this, just with programmable types and
  proper nonce / recovery primitives.

## License

Apache-2.0.
