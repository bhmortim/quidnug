# Peering-request rejection reasons

Stable enum reviewers use when rejecting a request. Each rejection
comment in the GitHub issue MUST be one of these strings (or a new one
added to this file in the same PR).

| Code                         | When to use                                                                    |
| ---------------------------- | ------------------------------------------------------------------------------ |
| `BAD_SIGNATURE`              | Signature doesn't verify, or quid ≠ first 16 hex of `sha256(publicKey)`.       |
| `ENDPOINT_UNREACHABLE`       | `nodeEndpoint /api/info` did not respond within 10s from the reviewing seed.   |
| `PROTOCOL_VERSION_TOO_OLD`   | Requesting node advertises a protocol version older than the minimum we serve. |
| `DOMAIN_OUT_OF_POLICY`       | Requested domain is outside `network.quidnug.com` or within a scoped subtree that requires prior attestation the requester lacks. |
| `DUPLICATE_OPERATOR`         | Another active peer claims the same operator name/URL with a different key.     |
| `RATE_LIMITED`               | Same operator made another request in the last 7 days.                          |
| `SUSPECTED_ABUSE`            | The requester's node is currently tier-Untrusted at the reviewing seed due to prior behavior. |
| `INCOMPLETE_METADATA`        | `operatorName`, `operatorUrl`, or `contact` is missing, obviously placeholder, or fails reachability. |
| `GUARDIAN_QUORUM_REQUIRED`   | Request targets `validators.*` but the requester has not installed a guardian quorum. |
| `NO_RECIPROCAL_CAPACITY`     | Requester's node is unable to reciprocate because its own epoch is invalidated or it's offline. |
| `DEFERRED_REVIEW`            | The request needs more than a desk review; reviewer is deferring to an out-of-band discussion. |
| `POLICY_DECISION`            | Reviewer declines on policy grounds not captured above. MUST be followed by a free-text paragraph explaining why; prompts a new enum candidate. |

## Appeal

A rejection is **prospective only**; the request can be re-submitted
after the underlying issue is addressed. Rate-limited rejections aside,
there is no N-strikes rule — operators can re-apply as many times as
they want.

Ongoing-policy rejections (`POLICY_DECISION`) should be discussed in
the public `#network` channel so the community can scrutinize and, over
time, codify the decision into a new enum.
