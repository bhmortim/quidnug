# Use-case examples

Runnable end-to-end examples covering every high-impact use
case in the Quidnug overview deck. Each example walks a
complete workflow — actors, trust setup, transactions, and the
audit path downstream consumers would use.

**Start here:** [`RUNBOOK.md`](RUNBOOK.md) — environment setup,
how to run each POC, a "run everything" one-liner, the demo-
authoring recipe, and the consolidated list of fixes the sweep
surfaced.

## Use-case POCs (16)

Each POC below ships a three-file layout:
`<name>.py` (pure decision logic, no SDK), `<name>_test.py`
(unit tests, no node required), and `demo.py` (end-to-end
against a live node).

| # | Use case | Path |
|---|---|---|
| 1 | Merchant fraud consortium | [`merchant-fraud-consortium/`](merchant-fraud-consortium/) |
| 2 | Credential verification network | [`credential-verification-network/`](credential-verification-network/) |
| 3 | AI agent capability authorization | [`ai-agent-authorization/`](ai-agent-authorization/) |
| 4 | Developer artifact signing (GPG replacement) | [`developer-artifact-signing/`](developer-artifact-signing/) |
| 5 | Institutional crypto custody (5-of-7 signing) | [`institutional-custody/`](institutional-custody/) |
| 6 | B2B invoice financing | [`b2b-invoice-financing/`](b2b-invoice-financing/) |
| 7 | Interbank wire authorization (tiered M-of-N) | [`interbank-wire-authorization/`](interbank-wire-authorization/) |
| 8 | AI content authenticity (C2PA-plus) | [`ai-content-authenticity/`](ai-content-authenticity/) |
| 9 | AI model provenance + supply chain | [`ai-model-provenance/`](ai-model-provenance/) |
| 10 | DeFi oracle network (consumer-weighted) | [`defi-oracle-network/`](defi-oracle-network/) |
| 11 | Federated learning attestation | [`federated-learning-attestation/`](federated-learning-attestation/) |
| 12 | Elections + anonymous ballot blind-signature | [`elections/`](elections/) (see `blind-flow/` for QDP-0021) |
| 13 | DNS replacement (Phase 0) | [`dns-replacement/`](dns-replacement/) |
| 14 | Enterprise domain authority (split-horizon) | [`enterprise-domain-authority/`](enterprise-domain-authority/) |
| 15 | Decentralized credit reputation | [`decentralized-credit-reputation/`](decentralized-credit-reputation/) |
| 16 | Healthcare consent management | [`healthcare-consent-management/`](healthcare-consent-management/) |

Every row above has been verified end-to-end against a freshly-
built single-node deployment. See
[`RUNBOOK.md`](RUNBOOK.md) for the full execution matrix.

## Companion demos (earlier work)

Smaller-scope demos predating the 16-POC sweep; kept for their
per-SDK / per-pattern value:

| Use case | Language(s) | Path |
|---|---|---|
| AI agent identity + LLM attribution | Python | [`ai-agents/`](ai-agents/) |
| W3C Verifiable Credentials on Quidnug | JavaScript | [`verifiable-credentials/`](verifiable-credentials/) |
| Trust-weighted reviews & comments (QRP-0001) | Python + Go + HTML | [`reviews-and-comments/`](reviews-and-comments/) |

## Environment

All POCs assume a local node at `http://localhost:8080`.
Fastest way to bring one up — see
[`RUNBOOK.md`](RUNBOOK.md) for details:

```bash
# From the repo root:
go build -o bin/quidnug ./cmd/quidnug
BLOCK_INTERVAL=2s ./bin/quidnug &
cd clients/python && pip install -e .
```

## Per-SDK quickstarts

Smaller per-API examples live in each client's `examples/`
folder:

- [`clients/python/examples/`](../clients/python/examples/)
- [`pkg/client/examples/`](../pkg/client/examples/)
- [`clients/rust/examples/`](../clients/rust/examples/)
- [`clients/java/examples/`](../clients/java/examples/)
- [`clients/dotnet/examples/`](../clients/dotnet/examples/)
- [`clients/swift/examples/`](../clients/swift/examples/)

## License

Apache-2.0.
