# Use-case examples

Runnable end-to-end examples covering the high-impact use cases
from the Quidnug overview deck. Each example walks a complete
workflow — actors, trust setup, transactions, and the audit path
downstream consumers would use.

| Use case | Language(s) | Path |
| --- | --- | --- |
| AI agent identity + LLM attribution | Python | [`ai-agents/`](ai-agents/) |
| Election integrity (candidates + voters + observers + tabulation) | Go | [`elections/`](elections/) |
| W3C Verifiable Credentials on Quidnug | JavaScript | [`verifiable-credentials/`](verifiable-credentials/) |
| Trust-weighted reviews & comments (QRP-0001) | Python orchestrator + Go signing helper + HTML render | [`reviews-and-comments/`](reviews-and-comments/) |

All examples assume a local node at `http://localhost:8080`
(the reviews demo runs on `:8087` via its own config). Fastest
way to a local node:

```bash
cd deploy/compose
docker compose up -d
```

Per-SDK quickstarts and smaller per-API examples live in each
client's `examples/` folder:

- [`clients/python/examples/`](../clients/python/examples/)
- [`pkg/client/examples/`](../pkg/client/examples/)
- [`clients/rust/examples/`](../clients/rust/examples/)
- [`clients/java/examples/`](../clients/java/examples/)
- [`clients/dotnet/examples/`](../clients/dotnet/examples/)
- [`clients/swift/examples/`](../clients/swift/examples/)

## Why these four?

The [product audit](../docs/audit/product-adoption-audit.md)
identified AI agents, elections, and credentials as the three
largest use cases marketed in the overview deck with **zero
runnable example code**. A prospective integrator looking at our
website today for "how do I use Quidnug for X" would have found
nothing for these three. This directory closes that gap.

The reviews-and-comments example is the fourth: a full
working end-to-end demo for the Quidnug Reviews Protocol
(QRP-0001). It shows the same 5 raw reviews producing three
divergent per-observer ratings and ships drop-in widgets for
every major web framework.

Additional use cases (decentralized credit, escrow, KYC workflows,
healthcare patient record signing) remain on the backlog.

## License

Apache-2.0.
