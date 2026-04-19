# Election integrity on Quidnug

The Quidnug overview deck markets elections as one of the 14 target
use cases. This directory provides a runnable end-to-end example
covering:

1. **Election registration** — the election authority creates a
   Quidnug Title for the election itself, establishing canonical
   metadata (election name, jurisdiction, dates).
2. **Candidate identity** — each candidate registers a quid and
   the authority issues a trust edge vouching that this quid
   represents this named candidate.
3. **Voter registration** — voters register quids (in practice,
   issued + bootstrapped by a civic agency). The registration
   authority issues trust edges.
4. **Ballot recording** — every cast ballot becomes an EVENT on
   the election's stream, signed by the voter's quid. Ballot
   secrecy is preserved via blind signatures / commitments (this
   example uses a simple hash-commit pattern).
5. **Observer attestation** — independent observers sign EVENTs
   on the election's stream attesting to procedural integrity
   ("I witnessed the polling station open at 07:00", etc.).
6. **Tabulation audit** — the tabulator publishes the final tally
   as an EVENT, with a Merkle-rooted list of accepted ballot
   digests. Anyone can replay the proof.

## Why Quidnug fits

Every election system must solve:

- **Cryptographic vote integrity** — Quidnug gives you append-only
  signed events with Merkle roots out of the box.
- **Multi-party trust** — Quidnug's relational trust graph encodes
  "which observers does the public trust?" without a central
  oracle.
- **Auditability** — the event stream is the audit log, and every
  Title / stream is queryable post-hoc.

What Quidnug does **not** provide:

- **Vote secrecy** beyond plain hash commitment. For a real
  election, pair Quidnug with a proper mix-net or ZKP voting
  protocol (Helios, STAR-Vote, Microsoft ElectionGuard). Quidnug
  records the outputs of those protocols; it doesn't implement
  them.

## Runnable example

See [`election_flow.go`](election_flow.go) for a Go end-to-end
simulation:

```bash
cd examples/elections
go run election_flow.go
```

It simulates a small election (3 candidates, 5 voters, 2
observers), runs through every phase, and prints the final audit
log with relational-trust scores.

## Stream schema

### Election Title

```json
{
  "type": "TITLE",
  "assetQuid": "election-2026-mayor-nyc",
  "titleType": "ELECTION",
  "ownershipMap": [
    { "ownerId": "nyc-board-of-elections-quid", "percentage": 100.0 }
  ]
}
```

### Ballot event

```json
{
  "type": "EVENT",
  "subjectId": "election-2026-mayor-nyc",
  "subjectType": "TITLE",
  "eventType": "BALLOT_CAST",
  "payload": {
    "ballotCommitment": "sha256:abc...",
    "ballotIndex": 42,
    "pollingStation": "NYC-BRK-07",
    "castAt": 1700000000
  }
}
```

Note the event is signed by the VOTER's quid, not the polling
station's. The polling station's quid could countersign via a
separate EVENT if you want dual-attestation.

### Observer attestation

```json
{
  "type": "EVENT",
  "subjectId": "election-2026-mayor-nyc",
  "subjectType": "TITLE",
  "eventType": "OBSERVER_ATTEST",
  "payload": {
    "observation": "polling_station_opened",
    "pollingStation": "NYC-BRK-07",
    "observedAt": 1700000000,
    "notes": "all machines responding; no issues at open"
  }
}
```

### Tabulation

```json
{
  "type": "EVENT",
  "subjectId": "election-2026-mayor-nyc",
  "subjectType": "TITLE",
  "eventType": "FINAL_TABULATION",
  "payload": {
    "candidateVotes": {
      "candidate-a-quid": 1234,
      "candidate-b-quid": 987,
      "candidate-c-quid": 234
    },
    "totalBallots": 2455,
    "merkleRoot": "sha256:...",
    "tallyMethod": "first-past-the-post"
  }
}
```

## Trust-gated audit

A public observer queries the event stream and computes
relational trust from their quid to:

1. The election authority (should be ≥ 0.9 for the citizen).
2. Each observer who attested (citizen accepts observers they
   transitively trust ≥ 0.5).

An observer chain the citizen doesn't trust doesn't invalidate
the election — it just means that particular attestation isn't
accepted into the citizen's audit report.

## Jurisdictional considerations

This is a **technical demonstration**, not a drop-in election
system. Real-world deployment requires:

- Compliance with jurisdiction-specific election law (NYS
  Election Law, US EAC VVSG, NIST SP 800-52, etc.).
- Independent security audit against the target threat model.
- Integration with existing voter-registration databases.
- Accessibility compliance (ADA, WCAG).

Quidnug provides the integrity layer. Everything above lives at
the application layer.

## License

Apache-2.0.
