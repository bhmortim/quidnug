# Video 5 — distribution plan

Video 5 targets a specific technical audience — W3C DID / VC
community members, identity architects, and enterprise
identity teams. Less mass-market than Videos 1–4, but
higher-quality-per-viewer.

## Publish sequence

### Day 0
- [ ] YouTube upload.
- [ ] Post to LinkedIn with identity-architect framing.
- [ ] Post to Twitter with tags to DID community accounts.

### Day 1
- [ ] Post to /r/identity (if active) or /r/decentralized.
- [ ] Post to identoo.ai community (DID/VC focused Discord).

### Day 2
- [ ] Post to the W3C Credentials CG mailing list — this
      audience is the exact target. Frame as "we built a
      complementary trust layer, curious for feedback."
- [ ] Cross-link to /r/selfsovereign if active.

### Day 7
- [ ] Pitch to Identity Unlocked / Identity, Unlocked! podcast
      (Auth0's DID community podcast).
- [ ] Reach out to known VC ecosystem builders (Sovrin
      Foundation, Trinsic, Animo, Mattr, Spruce) for
      cross-promotion.

### Day 14
- [ ] Write a companion blog post titled "DIDs + VCs +
      Quidnug: The Three-Check Verification Pattern" with
      embedded video.

## YouTube metadata

**Title:** `Quidnug vs W3C DIDs + Verifiable Credentials — compose, don't compete`
**Description:**
```
W3C DIDs and Verifiable Credentials are excellent standards.
Quidnug doesn't compete with them — it composes cleanly to
add transitive, per-verifier trust scoring on top.

This video walks through:
• What DIDs and VCs do well (and which EU + US pilots they
  enable)
• Four things missing from the VC data model today
• The "three-check verification" pattern: VC signature +
  Quidnug event + relational trust
• A concrete example: university-issued degree → unknown
  verifier accepts via accreditation chain
• When to pick which (decision table at 4:45)

Chapters:
0:00 — Framing: composes or competes?
0:30 — What DIDs + VCs do well
1:30 — Four gaps
2:45 — The composable architecture
4:00 — University-degree example
4:45 — Decision table
5:30 — Outro

Runnable example (JavaScript): examples/verifiable-credentials/
in the repo.
Detailed comparison doc: docs/comparison/vs-did-vc.md

Repo: https://github.com/bhmortim/quidnug

#DID #VerifiableCredentials #Decentralized #Identity
#W3C #OpenSource
```

## LinkedIn

```
DIDs + VCs or Quidnug? Neither.

Both.

They compose cleanly via the 3-check verification pattern:
1. VC signature (standard — Ed25519)
2. Quidnug event (unified audit + revocation)
3. Relational trust (per-verifier scoring)

Result: eIDAS / US-pilot compliance AND transitive trust
chains. Same credentials, richer verification.

6-minute walk-through + decision table at 4:45:
[YouTube link]

Runnable JS example (university-degree flow with an
unknown verifier accepting via transitive accreditation
trust): [repo link]

github.com/bhmortim/quidnug
docs/comparison/vs-did-vc.md

#DID #VerifiableCredentials #Identity #OpenSource
```

## Twitter / X

```
DIDs + VCs or Quidnug?

neither. both.

they compose via the 3-check pattern:
1. standard VC signature (Ed25519)
2. Quidnug event (unified revocation + audit)
3. relational trust (per-verifier scoring)

6 min:
```

## W3C Credentials CG mailing list

**Subject:** `[FYI] Quidnug — complementary trust layer for VCs`
**Body:**
```
Hi folks,

We've been building Quidnug (Apache-2.0, 7 SDKs) — a
decentralized protocol for per-observer relational trust.
It's designed to compose with W3C DIDs + VCs rather than
replace them.

Short 6-minute video walk-through: [YouTube link]
Comparison doc with decision table: [repo link]

The pattern: every VC is issued as a standard VC (keeps
eIDAS / wallet interop), AND posted as a signed event on
the subject's Quidnug stream (gives unified audit +
revocation), AND evaluated via Quidnug's relational-trust
query (per-verifier scoring).

I'd love feedback from this community on:
- The did:quidnug method mapping (it's trivially a DID
  method; worth formalizing?).
- Whether the 3-check pattern aligns with how you think
  verifiers will evolve.
- The boundary between VC revocation lists (StatusList2021
  etc.) and Quidnug-event-based revocation.

Thanks for the wonderful work on the standards — truly.
We've built on them extensively.

[You]
```

## Success criteria (abbreviated)

| Metric | Target | At |
| --- | --- | --- |
| YouTube views | ≥ 1500 | 30d |
| LinkedIn reactions from identity-architect demographics | ≥ 30 | 7d |
| W3C Credentials CG thread responses | ≥ 3 substantive | 7d |
| Inbound from DID ecosystem (Trinsic, Mattr, etc.) | ≥ 1 | 30d |
| Companion blog post views | ≥ 500 | 30d |

## License

Apache-2.0.
