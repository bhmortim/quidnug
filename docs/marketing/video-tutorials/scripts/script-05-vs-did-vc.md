# Script 05 — Quidnug vs W3C DIDs + Verifiable Credentials

**Length:** 6:00
**Audience:** Identity architects already evaluating W3C DIDs +
VCs for their project. The questions they're asking: "is
Quidnug a competitor? A replacement? A complement?"
**Goal:** Clear "when to pick which" framework.
**Recommended stack:** HeyGen avatar + Google Slides / Pitch +
Descript.

---

## Script

### [0:00–0:30] Framing

> "If you're in the identity space in 2026, you've heard of
> W3C Decentralized Identifiers and Verifiable Credentials —
> DIDs and VCs. They're great standards, well-maintained,
> widely supported."
>
> "Today I want to answer a question a lot of folks have been
> asking: does Quidnug compete with DIDs + VCs, or does it
> compose with them? Short answer: it composes. Longer answer
> — let's spend six minutes on it."

### [0:30–1:30] What DIDs + VCs do well

*[slide: "What DIDs + VCs solve"]*

> "DIDs give you a persistent, self-sovereign identifier that's
> not bound to any particular registrar. Your DID survives
> Google, Microsoft, or Auth0 turning you off."
>
> "VCs give you cryptographically-signed credential documents.
> 'University of Example issues this credential asserting that
> Alice has a Bachelor of Science.' Every standards-compliant
> verifier can check the signature."
>
> "The ecosystem is mature: several DID methods — `did:key`,
> `did:web`, `did:ion`, `did:ethr`. Multiple credential
> formats — JSON-LD with Ed25519Signature, JWT-VC, SD-JWT,
> mDL for mobile driver's licenses."
>
> "The EU is rolling out eIDAS 2.0 on this stack. US states
> are piloting it. If you need interop with existing wallets,
> VCs are the right format."

*[slide: checkmarks next to: persistent identity, signed
credentials, mature ecosystem, eIDAS 2.0 interop]*

### [1:30–2:45] What DIDs + VCs don't do

*[slide: "What's missing"]*

> "Four gaps."
>
> "**One — trust composition.** Verifying a VC is binary. You
> trust the issuer's DID, or you don't. There's no 'I trust
> this issuer at 0.7,' no transitive chain."
>
> "**Two — per-observer scoring.** Two different verifiers
> looking at the same VC see the same outcome. There's no
> notion that Alice trusts University X more than Bob does.
> Identity in DIDs is subject-sovereign; *trust* is whatever
> each verifier hard-codes."
>
> "**Three — unified revocation.** Each issuer maintains their
> own revocation list — StatusList2021 or similar. There's no
> cross-issuer revocation feed."
>
> "**Four — audit log.** VCs are point-in-time documents.
> There's no built-in append-only 'issued-revoked-reinstated'
> history you can query."

*[slide: red X next to: transitive trust, per-observer scoring,
unified revocation, audit log]*

### [2:45–4:00] How Quidnug fills those gaps

*[slide: architecture diagram — VC layer on top, Quidnug layer
underneath]*

> "The recommended pattern is: use DIDs + VCs AND Quidnug
> together."
>
> "- Your credential subject's DID maps to a Quidnug quid.
>   Literally `did:quidnug:<quid-id>` is a valid DID method."
>
> "- Your issuer signs a standard VC with their existing key
>   — Ed25519, ES256, whatever. That's compatible with every
>   standards-compliant verifier."
>
> "- Separately, the issuer posts the VC as a **signed event**
>   on the subject's Quidnug stream. Event type
>   `VC_ISSUED`, payload is the full VC JSON-LD."
>
> "- For revocation, post a `VC_REVOKED` event."
>
> "Now every verifier has three checks instead of one:"
>
> "1. The VC's internal signature (standard VC verification)."
>
> "2. The Quidnug event — is it in the stream, signed, not
>   revoked?"
>
> "3. **Relational trust** from the verifier's quid to the
>   issuer's quid."
>
> "Standards-compliant verifiers that don't know about
> Quidnug still see a valid VC. Quidnug-aware verifiers get
> transitive trust scoring for free."

### [4:00–4:45] The concrete example

*[slide: a specific flow]*

> "A university issues a degree. Your employer has never
> heard of the university. But your employer trusts the
> regional accreditation body at 0.9. The accreditation body
> trusts the university at 0.95."
>
> "Your employer's transitive trust in the university is
> 0.9 × 0.95 = 0.855."
>
> "If your employer's acceptance threshold is 0.7, the
> credential passes. No per-employer whitelist of universities
> to maintain. The graph does the scoring."

### [4:45–5:30] When to pick which

*[slide: decision table]*

| If you need... | Use |
| --- | --- |
| Interop with eIDAS / EU wallet / US state pilots | VCs, with Quidnug optional |
| Binary "issuer-is-whitelisted" verification | VCs alone |
| Transitive trust chains | VCs + Quidnug |
| Per-verifier trust scoring | VCs + Quidnug |
| Unified revocation across issuers | VCs + Quidnug |
| Pure backend / no VC-wallet needed | Quidnug alone |
| Need both standards interop AND graph trust | VCs + Quidnug |

### [5:30–6:00] Outro

> "Bottom line — DIDs and VCs are the right credential format.
> Use them. Quidnug isn't trying to replace them."
>
> "But credential *verification* today is a whitelist problem
> that doesn't scale. Quidnug adds the trust-graph layer on
> top. Same credentials, richer verification."
>
> "The full runnable example — university issues a degree,
> employer verifies via transitive trust — is at
> `examples/verifiable-credentials/` in the repo. JavaScript,
> runs in about five minutes."
>
> "Apache-2.0. github.com/bhmortim/quidnug."

---

## Production notes

- **Slides**: 8 total. Problem (1), DIDs strengths (1), gaps
  (1), composition architecture (1), concrete example (1),
  decision table (1), outro (1), plus title card.
- **Avatar positioning**: HeyGen picture-in-picture, bottom-
  right, 20% screen. Slide content full-screen.
- **Pace**: This audience rewards density. Don't pad.

## License

Apache-2.0.
