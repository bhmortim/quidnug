# Use-case examples

Runnable end-to-end examples covering the high-impact use cases
from the Quidnug overview deck. Each example walks a complete
workflow — actors, trust setup, transactions, and the audit path
downstream consumers would use.

| Use case | Language | Path |
| --- | --- | --- |
| AI agent identity + LLM attribution | Python | [`ai-agents/`](ai-agents/) |
| Election integrity (candidates + voters + observers + tabulation) | Go | [`elections/`](elections/) |
| W3C Verifiable Credentials on Quidnug | JavaScript | [`verifiable-credentials/`](verifiable-credentials/) |

All examples assume a local node at `http://localhost:8080`. The
fastest way to one:

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

## Why these three?

The [product audit](../docs/audit/product-adoption-audit.md)
identified AI agents, elections, and credentials as the three
largest use cases marketed in the overview deck with **zero
runnable example code**. A prospective integrator looking at our
website today for "how do I use Quidnug for X" would have found
nothing for these three. This directory closes that gap.

Additional use cases (decentralized credit, escrow, KYC workflows,
healthcare patient record signing) remain on the backlog.

## License

Apache-2.0.
