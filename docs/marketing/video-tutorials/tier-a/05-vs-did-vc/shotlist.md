# Video 5 — Quidnug vs W3C DIDs + Verifiable Credentials — shot list

**Final length:** 6:00
**Budget:** $120 (thumbnail $40, music $15, slide design polish $40, buffer $25)
**Production time:** 14 hours over 4 days
**Aspect ratios delivered:** 1920×1080 horizontal, 1080×1920 vertical of the decision table

## What this video does

The identity-architect audience lands here from search —
"Quidnug vs DIDs," "relational trust vs VCs." The video's job
is to give them a clean "when to pick which" framework that
they can show their team in their next architecture review.

## Key upgrade vs. base scripts

Doubled budget funds:
- A properly-designed 8-slide deck (Pitch.com or Figma Slides
  with a pro template).
- HeyGen avatar in slightly more formal attire (for the
  identity-architect audience).
- A decision table that's well-designed.
- Animated transitions between slides.

## Second-by-second shot list

### Slide 1 — Framing (0:00–0:30)

| Time | On-screen | Voice-over | Notes |
| --- | --- | --- | --- |
| 0:00–0:03 | Brand stinger | *(music: ambient thoughtful)* | Stinger |
| 0:03–0:30 | HeyGen avatar PIP + title slide "Quidnug vs W3C DIDs + Verifiable Credentials — composes or competes?" | "If you're in the identity space in 2026, you've heard of W3C Decentralized Identifiers and Verifiable Credentials — DIDs and VCs. They're great standards, well-maintained, widely supported. Today I want to answer a question a lot of folks have been asking: does Quidnug compete with DIDs + VCs, or does it compose with them? Short answer: composes. Longer answer — let's spend six minutes on it." | HeyGen + slide |

### Slide 2 — What DIDs + VCs do well (0:30–1:30)

| Time | On-screen | Voice-over | Notes |
| --- | --- | --- | --- |
| 0:30–0:40 | Slide title: "What DIDs + VCs do well" | "Let me be clear — this is a love letter, not a takedown. DIDs and VCs are excellent for what they do." | |
| 0:40–0:55 | Four bullet points animate in: "Persistent, self-sovereign identity," "Signed credentials," "Cryptographic verification," "Mature ecosystem" | "DIDs give you a persistent, self-sovereign identifier — not bound to any registrar. Your DID survives Google, Microsoft, or Auth0 turning you off." | |
| 0:55–1:15 | Show examples: "did:key," "did:web," "did:ion," "did:ethr," and credential formats "JSON-LD + Ed25519," "JWT-VC," "SD-JWT," "mDL" | "The ecosystem is mature — multiple DID methods, multiple credential formats, EU rolling out eIDAS 2.0 on this stack, US states piloting it." | Logo soup from W3C docs |
| 1:15–1:30 | Slide summarizes with 4 check-marks | "If you need to interop with existing VC verifiers — European wallets, SIOP-V2 flows — VCs are the right format. Full stop." | Transition in |

### Slide 3 — What's missing (1:30–2:45)

| Time | On-screen | Voice-over | Notes |
| --- | --- | --- | --- |
| 1:30–1:45 | Slide title: "What's NOT in the VC data model" | "Four gaps I want to talk about." | |
| 1:45–2:00 | Gap 1 highlighted: "Trust composition" + a small diagram showing binary accept/reject | "**One — trust composition.** VC verification is binary. You trust the issuer's DID, or you don't. There's no 'I trust this issuer at 0.7,' no transitive chain." | |
| 2:00–2:15 | Gap 2: "Per-observer scoring" — show two verifier avatars seeing the same "valid" outcome | "**Two — per-observer scoring.** Two different verifiers looking at the same VC see the same outcome. No notion that Alice trusts University X more than Bob does. Identity is subject-sovereign; trust is whatever each verifier hard-codes." | |
| 2:15–2:30 | Gap 3: "Unified revocation" — show separate status lists per issuer | "**Three — unified revocation.** Each issuer maintains their own revocation list. There's no cross-issuer revocation feed." | |
| 2:30–2:45 | Gap 4: "Audit log" — timeline with gaps | "**Four — audit log.** VCs are point-in-time documents. There's no built-in append-only 'issued-revoked-reinstated' history you can query." | |

### Slide 4 — How Quidnug fills those gaps (2:45–4:00)

| Time | On-screen | Voice-over | Notes |
| --- | --- | --- | --- |
| 2:45–3:00 | Slide title: "The recommended architecture: use both" | "The recommended pattern: use DIDs + VCs AND Quidnug together." | |
| 3:00–3:15 | Stack diagram: VC layer on top (standards-compliant), Quidnug layer below (trust graph) | "Your credential subject's DID maps to a Quidnug quid. `did:quidnug:<quid-id>` is a valid DID method by construction." | Custom stack diagram |
| 3:15–3:35 | Animated: VC is signed (standard Ed25519), AND posted as a signed event on Quidnug stream | "Your issuer signs a standard VC — Ed25519, ES256. Compatible with every standards-compliant verifier. Separately, the issuer posts the VC as a signed event on the subject's Quidnug stream. Event type VC_ISSUED, payload is the full VC JSON-LD." | Animation |
| 3:35–3:50 | Revocation: shown as a simple `VC_REVOKED` event later in the same stream | "For revocation, just emit a VC_REVOKED event." | |
| 3:50–4:00 | Three-check verifier flow: 1) VC signature 2) Quidnug event 3) relational trust | "Now every verifier has three checks instead of one. Standards-compliant verifiers that don't know about Quidnug still see a valid VC. Quidnug-aware verifiers get transitive trust scoring for free." | |

### Slide 5 — Concrete example (4:00–4:45)

| Time | On-screen | Voice-over | Notes |
| --- | --- | --- | --- |
| 4:00–4:15 | Visual scenario: University of Example → Alice's diploma VC → HigherEd Accreditation → Acme HR | "A university issues a degree. Your employer has never heard of the university. But your employer trusts the regional accreditation body at 0.9. The accreditation body trusts the university at 0.95." | |
| 4:15–4:30 | Math overlay: 0.9 × 0.95 = 0.855 | "Your employer's transitive trust in the university is 0.9 × 0.95 = 0.855." | |
| 4:30–4:45 | Threshold slider: 0.7 → green check; "credential accepted" | "If your employer's acceptance threshold is 0.7, the credential passes. No per-employer whitelist of universities to maintain. The graph does the scoring." | |

### Slide 6 — Decision table (4:45–5:30)

| Time | On-screen | Voice-over | Notes |
| --- | --- | --- | --- |
| 4:45–5:30 | Full-screen decision table — 7 rows, animates in 1 row per 6 seconds | *(narrates each row)* | Table animation |

Table content (design it like a real decision grid):

| If you need... | Use |
| --- | --- |
| Interop with eIDAS / EU wallet / US state pilots | VCs (Quidnug optional) |
| Binary "issuer-is-whitelisted" verification | VCs alone |
| Transitive trust chains | VCs + Quidnug |
| Per-verifier trust scoring | VCs + Quidnug |
| Unified revocation across issuers | VCs + Quidnug |
| Pure backend / no VC-wallet needed | Quidnug alone |
| Need both standards interop AND graph trust | VCs + Quidnug |

### Slide 7 — Outro (5:30–6:00)

| Time | On-screen | Voice-over | Notes |
| --- | --- | --- | --- |
| 5:30–5:45 | Slide: "Use both" + runnable example pointer at `examples/verifiable-credentials/` | "Bottom line — DIDs and VCs are the right credential format. Use them. Quidnug isn't trying to replace them. But credential verification today is a whitelist problem that doesn't scale. Quidnug adds the trust-graph layer on top. Same credentials, richer verification." | |
| 5:45–6:00 | Brand outro: URL + Apache-2.0 badge + "Full example: examples/verifiable-credentials/ (JavaScript)" | "Full runnable example — university issues a degree, employer verifies via transitive trust — at `examples/verifiable-credentials/`. JavaScript, runs in about five minutes. Apache-2.0. github.com/bhmortim/quidnug." | |

## Slide design (8 slides total)

Use Pitch.com or Figma Slides. Spend the $40 polish budget
hiring a freelance designer to make these 8 look **crisp**.
Brief:

> 8-slide deck, 16:9. Dark navy background `#0B1D3A`.
> Brand font Inter Bold for headings, Inter Regular for
> body. Accent amber `#F9A825` for emphasis. Each slide
> has a ~5-word title top-left, body content centered, a
> subtle animated element on slide enter. No corporate
> clip-art; use real iconography (Feather, Lucide, or custom
> geometric shapes).
>
> Slides:
> 1. Title — "Quidnug vs W3C DIDs + VCs — composes or competes?"
> 2. "What DIDs + VCs do well" — 4 check bullets
> 3. "What's NOT in the VC data model" — 4 gap highlights
> 4. "The recommended architecture" — stack diagram (VC on top, Quidnug below)
> 5. "The 3-check verifier flow" — three checkmark columns
> 6. "Concrete example" — University diagram with math overlay
> 7. "Decision table" — 7-row "If you need X, use Y" table
> 8. "Outro" — URL + logo + example pointer

## Music cues

| Time | Action |
| --- | --- |
| 0:00 | Ambient thoughtful bed enters soft |
| 1:30 | Tension rises into the "what's missing" segment |
| 2:45 | Release — lighter, collaborative mood |
| 4:45 | Sustain under the decision table |
| 5:45 | Fade out under outro |

## Avatar direction

HeyGen avatar in slightly business-casual framing. Direction:

- **Tone**: consultative, not evangelical. The identity-architect
  audience is already skeptical of yet-another-identity-thing.
- **Pace**: 150 wpm (slower than overview video, denser content).
- **Emphasis**: "composes," "both," "richer verification."
- **Eye line**: direct camera except during the 3-check
  animation at 3:50 — look off-camera briefly to give the
  animation space.

## Export spec

| Deliverable | Dimensions | Duration | Target |
| --- | --- | --- | --- |
| YouTube master | 1920×1080 60fps | 6:00 | Primary |
| Vertical cut (decision table) | 1080×1920 60fps | 0:45 | 4:45–5:30 isolated + outro |
| Square | 1080×1080 60fps | 6:00 | LinkedIn feed |

## Thumbnail brief

> Design a YouTube thumbnail for "Quidnug vs DIDs + VCs:
> compose, don't compete."
>
> Split-screen: left side shows "DID + VC" logo stack in
> a blue-gray tone; right side shows "QUIDNUG" in amber
> with a graph underneath. Between them, a large white
> "+" sign (not "vs") emphasizing composition.
>
> Top text: "DIDs + VCs + QUIDNUG" in bold white.
> Bottom kicker: "the 3-check verification pattern"
>
> Brand colors: navy, amber, cooler VC-community blue
> `#3B82F6` for the DID side for contrast.

## Distribution captions

### LinkedIn
> DIDs + VCs or Quidnug? Neither — both.
>
> 6-minute walk-through of the 3-check verification pattern:
> standards-compliant VC verification PLUS relational trust
> scoring via Quidnug.
>
> Decision table at 4:45. [YouTube link]
>
> Runnable example: `examples/verifiable-credentials/` in
> github.com/bhmortim/quidnug

### Twitter
> DIDs + VCs or Quidnug?
>
> Both. They compose cleanly:
>   1. VC signed the standard way (Ed25519)
>   2. VC issuance recorded as signed event on Quidnug
>   3. Verifier scores the issuer via relational trust
>
> 6 min: [link]

### Reddit r/identity (or identoo.ai community)
Title: "Quidnug + DIDs + VCs: a compose-don't-compete pattern
for per-verifier trust scoring"
Body: Link + explain the three-check flow. Ask for feedback
from DID community folks.

## License

Apache-2.0.
