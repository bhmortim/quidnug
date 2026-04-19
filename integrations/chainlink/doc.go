// Package chainlink implements a Chainlink External Adapter that
// lets on-chain smart contracts query Quidnug relational trust.
//
// External Adapters are the standard Chainlink pattern for bringing
// off-chain data on-chain. Chainlink nodes POST a JSON job spec to
// the adapter; the adapter fetches data and returns a JSON envelope.
//
// This adapter exposes:
//
//	Query: { observer: "quid-a", target: "quid-b",
//	         domain: "supplychain.home", maxDepth: 5 }
//	Reply: { trustLevel: 0.72, pathDepth: 3, path: ["a","c","d","b"] }
//
// So a Solidity contract can (via a Chainlink oracle) ask "does
// my-procurement-quid relationally trust this vendor-quid above a
// threshold?" before releasing funds, minting an allowance, etc.
//
// # Deployment
//
// Ship as a stateless HTTP service. Chainlink DevOps bundles it into
// their external-adapter docker registry. Configure:
//
//   - QUIDNUG_NODE  — base URL of the Quidnug node to query.
//   - QUIDNUG_TOKEN — optional bearer token.
//
// The adapter does no caching — the Quidnug node has its own LRU,
// and Chainlink's response caching operates independently.
package chainlink
