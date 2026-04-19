# Quidnug × Chainlink External Adapter

Expose Quidnug relational-trust queries to on-chain smart contracts.

## Build + run

```bash
QUIDNUG_NODE=http://quidnug.local:8080 go run ./integrations/chainlink/cmd
# → quidnug-chainlink-adapter: listening on :8090, node=...
```

## Protocol

The adapter implements the standard Chainlink External Adapter
request/response envelope.

```
POST /
Content-Type: application/json

{
  "id": "job-run-id",
  "data": {
    "observer": "<observer quid id>",
    "target":   "<target quid id>",
    "domain":   "supplychain.home",
    "maxDepth": 5
  }
}
```

Response:

```json
{
  "jobRunID":   "job-run-id",
  "data": {
    "trustLevel": 0.72,
    "pathDepth":  2,
    "path":       ["a", "c", "b"],
    "observer":   "a",
    "target":     "b",
    "domain":     "supplychain.home"
  },
  "result":     0.72,
  "statusCode": 200
}
```

The `result` field is the primary scalar Chainlink's job spec
normally consumes (see ChainlinkJobSpec TOML, task `copy`/`multiply`).

## Solidity consumer sketch

```solidity
// Kick off the trust query.
bytes32 public requestId = sendChainlinkRequestTo(oracle, req, fee);

// Callback: Chainlink decodes `result` as fixed-point in your
// spec. For a 0-1 float, multiply by 1e18 off-chain then decode as
// uint256.
function fulfill(bytes32 _requestId, uint256 _trustScoreWei) external {
    require(_trustScoreWei >= 0.7e18, "vendor trust too low");
    // … release funds / mint allowance / etc.
}
```

## Env vars

| Var | Default | Purpose |
| --- | --- | --- |
| `QUIDNUG_NODE` | `http://localhost:8080` | Node to query. |
| `QUIDNUG_TOKEN` | _unset_ | Optional bearer auth token. |
| `LISTEN` | `:8090` | HTTP listen address. |

## Security

- The adapter is **read-only** on the Quidnug side — no writes.
- The adapter itself is trust-neutral; the relational-trust result it
  returns is a function of the observer you pass in. Any on-chain
  consumer that accepts an adapter reply for an arbitrary observer
  must also validate that observer is the contract's expected
  authority.

## License

Apache-2.0.
