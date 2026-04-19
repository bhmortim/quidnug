---
name: Peering request
about: Request bilateral peering with the public Quidnug network
title: 'Peering request: <your operator name>'
labels: peering-request
assignees: bhmortim
---

<!--
Paste your signed peering request JSON in the fenced block below. See
https://quidnug.com/docs/running-a-node/#join-the-public-network for
the full protocol spec, or
https://github.com/bhmortim/quidnug/blob/main/deploy/public-network/peering-protocol.md

The JSON must pass these checks:
  1. sha256(publicKey)[:16] == requester.quidId
  2. The signature field verifies against the declared publicKey
  3. nodeEndpoint is reachable and returns /api/info
  4. domains are all subtrees of network.quidnug.com
-->

## Peering request

```json
{
  "type": "QUIDNUG_PEERING_REQUEST_V1",
  "requester": {
    "quidId":       "",
    "publicKey":    "",
    "operatorName": "",
    "operatorUrl":  "",
    "nodeEndpoint": "",
    "nodeVersion":  "",
    "contact":      ""
  },
  "target": {
    "quidId":       ""
  },
  "domains": [
    "peering.network.quidnug.com"
  ],
  "trustLevel":     0.75,
  "proposedExpiry": null,
  "nonce":          1,
  "timestamp":      0,
  "signature":      ""
}
```

## Operator notes

<!-- Anything you'd like the reviewer to know — prior attestations,
     intended workload, domains you plan to operate in beyond the
     reserved `network.*` tree, hours you can respond to a security
     incident, etc. -->

## Checklist

- [ ] I have generated and installed a guardian quorum for this node's
      quid (M-of-N, time-lock configured).
- [ ] My node passes `GET /api/health` externally.
- [ ] I have read and accept the [peering protocol](../../deploy/public-network/peering-protocol.md),
      including the 72-hour reciprocation window and the revocation rules.
- [ ] My `operatorUrl` and `contact` are real and attended.
