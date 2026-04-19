# Script 02 — "Quidnug in 3 minutes"

**Length:** 3:00
**Audience:** Developers / architects evaluating whether to
spend an hour on our docs.
**Goal:** Leave them with a clear mental model and one concrete
reason to try it.
**Recommended stack:** HeyGen avatar (or real you on camera) +
Descript + Figma for architecture frames.

---

## Act 1 — The problem (0:00–0:45)

> "I'm going to explain Quidnug in under three minutes. Here's
> the problem:"
>
> "Imagine you're building a system where trust matters.
> Medical records. Supply-chain provenance. AI-agent
> attribution. Election integrity. Vendor management."
>
> "You need to answer questions like: 'Should I trust this
> party's claim?' 'Who vouches for them?' 'If someone in the
> chain gets compromised, how do I recover?'"
>
> "Today, there are basically three answers."
>
> "**One** — centralized reputation systems. Credit bureaus.
> Yelp. App-store ratings. One score per entity, globally.
> Fine if you trust the scorer."
>
> "**Two** — blockchains. Global consensus, but every
> transaction is public forever, fees per write, and trust is
> still one-number-fits-all."
>
> "**Three** — federation / Web-of-Trust systems like PGP. No
> central authority, but no recovery, no domain scoping, no
> typed relationships."
>
> "We wanted something different."

*Architecture frame: three labeled boxes fading out, one by
one, as the narrator dismisses each.*

---

## Act 2 — What Quidnug actually is (0:45–1:45)

> "Quidnug is a decentralized protocol for **relational trust**."
>
> "Every person, organization, AI agent, and device is a
> **quid** — a cryptographic identity with an ECDSA P-256
> keypair. You own your key. No central registrar."
>
> "Each quid issues signed **trust edges** to other quids.
> 'I trust Alice at 0.9 in the "vendor" domain.' The edge is
> scoped to a domain — so trusting someone as a *vendor*
> doesn't leak into trusting them as an *election observer*."
>
> "Quidnug then answers relational-trust queries: from
> **observer A's perspective**, through the graph, what is the
> effective trust in **target B**? It finds the best path,
> multiplies the edge weights along it, and returns the
> decayed trust."
>
> "Critically, every observer gets their *own* answer. There's
> no global score. My trust in you is different from my
> neighbor's trust in you, and Quidnug makes that explicit."

*Architecture frame: a graph of ~8 quids appears. Edges fade
in one by one. Then the "camera" moves to each observer and
shows the computed path + score from their POV.*

---

## Act 3 — Why you'd actually use it (1:45–2:30)

> "Why does this matter?"
>
> "Because today, when your app asks 'should I accept this
> claim?', the answer has to be a human policy decision —
> 'accept if issued by one of my 12 whitelisted CAs.' That
> list never scales, never adapts, and every edge case becomes
> a pull request."
>
> "With relational trust, the policy becomes declarative: 'accept
> if my transitive trust is above 0.7.' The graph encodes the
> nuance."
>
> "On top of that, Quidnug gives you:"
>
> "- **Typed event streams** — every quid has an append-only
>   log of signed events."
>
> "- **Guardian-based recovery** — M-of-N signatures to rotate
>   a lost key, with a time-locked veto to catch social
>   engineering attacks."
>
> "- **Cross-domain gossip** — domains talk via compact Merkle
>   proofs so no domain has to trust another domain's full
>   history."

*Architecture frame: three icons (event log, guardian shield,
gossip arrows) animate in as the narrator lists them.*

---

## Act 4 — How to try it (2:30–3:00)

> "We ship seven SDKs — Python, Go, JavaScript, Rust, Java,
> .NET, and Swift — all at full protocol parity. Plus a
> Docker Compose dev network, a Helm chart for production, and
> integrations with Sigstore, FHIR, Chainlink, and ISO 20022."
>
> "One command gets you a local consortium:"
>
> *[screen cut — terminal showing]*
>
> ```
> cd deploy/compose && docker compose up -d
> ```
>
> "Then follow a quickstart in your language of choice. The
> Python one is 10 lines."
>
> "Apache-2.0. Open source. **github.com/bhmortim/quidnug**."
>
> "If you're building anything where trust is more than a
> one-number-fits-all answer — take a look."

*Outro: GitHub URL + language logos + "made with ❤️ for the
internet that was."*

---

## Production notes

- **Architecture frames**: Build 4 animated Figma / Excalidraw
  frames, each 3–5 seconds, exported as MP4. Use HeyGen's
  picture-in-picture overlay so the avatar stays on screen
  while the diagram animates.
- **Visual pacing**: architecture frame every ~45 seconds;
  camera moves between POVs in Act 2 are the anchor moment.
- **Tone**: Technical, confident, not overselling. Assume
  the viewer is a senior dev or architect.
- **Subtitles**: non-negotiable. Include them burned-in for
  social cuts and optional for YouTube (YouTube auto-captions
  are fine if you correct them).

## License

Apache-2.0.
