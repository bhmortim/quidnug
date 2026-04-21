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

What Quidnug does **not** provide in this demo set:

- **End-to-end mix-net integration.** The blind-flow demo
  covers voter unlinkability at the ballot-issuance layer
  (the authority cannot link blinded requests to cast
  ballots), which is the primary cryptographic property most
  voting schemes need. Compound protocols like Helios,
  STAR-Vote, or ElectionGuard layer additional properties on
  top (coercion resistance, end-to-end verifiable
  re-encryption mix-nets). Those are separable protocols that
  Quidnug records the outputs of.

## Runnable examples

Two companion demos live here:

### 1. Full election flow against a live node (`election_flow.go`)

```bash
cd examples/elections
go run election_flow.go
```

Simulates a small election (3 candidates, 5 voters, 2
observers), runs through every phase against a live Quidnug
node, and prints the audit log with relational-trust scores.
Uses a simple hash-commit pattern for ballots (ballot secrecy
is not the focus of this demo).

### 2. Anonymous ballot blind-signature flow (`blind-flow/main.go`)

```bash
cd examples/elections
go run ./blind-flow/
```

Self-contained demo of the QDP-0021 RSA-FDH blind-signature
flow (via `pkg/crypto/blindrsa`). The authority blindly signs
each voter's ballot token without learning the token. Voters
then cast signed ballots that are verifiable under the
authority's public key yet unlinkable to the signing session.
The demo also exercises two failure modes: a forged signature
from an attacker's key is rejected, and a replay of an
already-cast ballot is rejected. No live node required for
this one.

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
