# Video 3 — distribution plan

## Publish sequence

### Day 0
- [ ] Upload to YouTube.
- [ ] Pin to the Python SDK README as the canonical quickstart.
- [ ] Post to LinkedIn + Twitter with Python-specific hooks.
- [ ] Submit to r/Python with "Show-r/Python" flair.

### Day 1
- [ ] Reply to comments.
- [ ] Post the 60-second vertical cut (the `pip install` → run
      segment) to Twitter/X with "1 minute to first Python
      quickstart" framing.

### Day 7
- [ ] Include in a Python Weekly / PyCoder's Weekly submission
      (these newsletters publish links they've curated).

### Day 14
- [ ] If metrics hit target, write a dev.to blog post titled
      "Cryptographic identity for Python apps in 5 minutes"
      referencing the video.

## YouTube metadata

**Title:** `Quidnug Python SDK — 5-minute quickstart (ECDSA P-256, relational trust)`
**Description:**
```
Install the Quidnug Python SDK, register two identities,
grant signed trust, and query transitive trust — in under
five minutes.

What you'll build: two cryptographic identities with P-256
keypairs, a signed trust edge from Alice to Bob, and a
third actor (Carol) that shows off transitive trust
(Alice → Bob → Carol at 0.9 × 0.8 = 0.72).

Works with Python 3.9+, built on `cryptography` and `requests`.
Apache-2.0.

Chapters:
0:00 — Intro
0:20 — Install
1:00 — Start a local node
1:40 — Write the code
3:20 — Run it
3:50 — Transitive trust
4:30 — Where to go next

Runnable example: examples/ai-agents/agent_identity.py
Async variant: `quidnug.async_client.AsyncQuidnugClient`
Repo: https://github.com/bhmortim/quidnug

#Python #OpenSource #Cryptography #Identity #Quidnug
```

## LinkedIn

```
`pip install quidnug` — 5-minute Python quickstart for
relational, cryptographically-signed trust.

What you get out of the box:
• ECDSA P-256 keypairs
• Signed identities
• Signed trust edges with numeric levels
• Transitive trust queries (0.9 × 0.8 = 0.72 as a 2-hop path)
• Async variant for FastAPI / aiohttp / asyncio

Full video: [YouTube link]
60-second install + first transaction: [vertical]

Apache-2.0, production-ready, byte-compatible with Go / JS /
Rust / Java / .NET / Swift SDKs.

github.com/bhmortim/quidnug
```

## Twitter / X

```
Python devs — 5 minute quickstart for relational trust:

```
alice = Quid.generate()
client.grant_trust(alice, trustee=bob.id, level=0.9)
tr = client.get_trust(alice.id, carol.id)  # 0.72 transitively
```

full video: [link]
```

## Reddit r/Python

**Title:** "Relational trust for Python apps — signed identities
+ transitive trust queries (Apache-2.0)"
**Body:**
```
Built a Python SDK for Quidnug — an open protocol for
per-observer relational trust. Think "what PGP's Web of
Trust should have been, with domain scoping, replay
protection, and guardian-based key recovery."

5-minute quickstart video: [YouTube link]
Async (httpx) variant also shipped for FastAPI / aiohttp folks.

Examples covering AI agent identity and W3C Verifiable
Credentials mapping in the repo.

Apache-2.0. https://github.com/bhmortim/quidnug

Particularly curious for feedback from data-engineering /
ML-platform folks — does the per-observer trust model
solve real problems in your pipelines?
```

## r/Python flair: `Showcase`

## Success criteria (abbreviated)

| Metric | Target | At |
| --- | --- | --- |
| YouTube views | ≥ 2000 | 30d |
| YouTube retention (4:30 mark) | ≥ 45% | 30d |
| r/Python upvotes | ≥ 40 | 48h |
| Python Weekly inclusion | Listed | 14d |
| Python SDK PyPI installs (once published) | +100/week | 30d |

## License

Apache-2.0.
