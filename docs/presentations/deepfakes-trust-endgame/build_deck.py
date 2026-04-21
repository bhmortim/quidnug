"""Deepfakes deck (~75 slides)."""
import pathlib, sys
HERE = pathlib.Path(__file__).parent
ASSETS = HERE / "assets"
OUTPUT = HERE / "deepfakes-trust-endgame.pptx"
sys.path.insert(0, str(HERE.parent))
from _deck_helpers import (  # noqa: E402
    make_presentation, title_slide, section_divider, content_slide,
    two_col_slide, stat_slide, quote_slide, table_slide, image_slide,
    code_slide, icon_grid_slide, closing_slide, add_notes, add_footer,
    TEAL, CORAL, EMERALD, AMBER, TEXT_MUTED,
)
from pptx.util import Inches  # noqa: E402

BRAND = "Quidnug  \u00B7  Deepfakes Are Trust's Endgame"
TOTAL = 75


def _im(prs, title, image, image_h=None, **kw):
    if image_h is not None and not hasattr(image_h, 'emu'):
        image_h = Inches(image_h)
    elif image_h is None:
        image_h = Inches(4.6)
    return image_slide(prs, title, image, image_h=image_h,
                       assets_dir=ASSETS, **kw)


def build():
    prs = make_presentation()

    s = title_slide(prs,
        "Deepfakes Are Trust's Endgame",
        "Why C2PA is necessary but insufficient, and what the full "
        "defense architecture actually looks like.",
        eyebrow="QUIDNUG  \u00B7  INFORMATION INTEGRITY")
    add_notes(s, [
        "75 slides, ~60 min with Q&A.",
        "Audience: journalism leaders, platform trust and safety, "
        "policy staff, technology decision-makers in media.",
        "Central thesis: detection cannot scale; provenance must. "
        "C2PA is good but only Layer 1 of a three-layer stack."
    ])
    add_footer(s, 1, TOTAL, BRAND)

    s = stat_slide(prs, "5,600x",
        "growth in detected deepfake media in circulation, "
        "2019 to 2025.",
        context="Detection is losing the arms race. The defenders "
                "must shift from detection to provenance.",
        title="The 2024 inflection")
    add_footer(s, 2, TOTAL, BRAND)

    s = stat_slide(prs, "11",
        "documented deepfake election incidents across 9 countries "
        "in 2024 alone.",
        context="From Slovakia to Indonesia to Romania (where the "
                "first round of presidential elections was annulled "
                "in November 2024 over deepfake-driven manipulation).",
        title="Operational political tool, worldwide", stat_color=CORAL)
    add_footer(s, 3, TOTAL, BRAND)

    s = content_slide(prs, "Agenda", bullets=[
        ("1. The 2024 elections.",
         "Documented incidents, scale measurements, beyond politics."),
        ("2. Why detection is losing.",
         "Asymmetry, evasion dynamic, what detection IS good for."),
        ("3. What C2PA gets right.",
         "Capture-to-display chain. Adoption status. Where it stops."),
        ("4. Layer 2: DNS-anchored publisher identity.",
         "Bind existing trusted brands to crypto identity."),
        ("5. Layer 3: relational trust.",
         "Per-observer publisher weighting. Composes with C2PA + DNS."),
        ("6. Protecting sources without exposing them.",
         "Selective disclosure for investigative journalism."),
        ("7. End-to-end architecture.",
         "Full pipeline from witness to reader."),
        ("8. Adoption strategy and tradeoffs.",
         "Who moves first. What the next 5 years look like."),
    ])
    add_footer(s, 4, TOTAL, BRAND)

    s = content_slide(prs, "Four claims this talk defends",
        bullets=[
            ("Detection is necessary but losing.",
             "Generator quality is improving faster than classifier "
             "accuracy."),
            ("Provenance-first architecture is the only durable answer.",
             "Sign at capture; verify at display; trust per-observer."),
            ("C2PA is correct but insufficient.",
             "It is Layer 1 of three. We need Layers 2 and 3 too."),
            ("Source protection composes with provenance.",
             "Selective disclosure preserves credibility without "
             "identity exposure."),
        ])
    add_footer(s, 5, TOTAL, BRAND)

    # Section 1
    s = section_divider(prs, 1, "The 2024 Elections",
        "When deepfakes became operational political tools.")
    add_footer(s, 6, TOTAL, BRAND)

    s = _im(prs, "Documented deepfake election incidents 2024",
        "chart_elections.png", image_h=4.8)
    add_footer(s, 7, TOTAL, BRAND)

    s = content_slide(prs, "Slovakia September 2023: the first proof point",
        bullets=[
            ("Days before the September 30 parliamentary election.",
             "An audio deepfake of opposition leader Michal Simecka "
             "circulated on social media."),
            ("Content.",
             "Fake conversation between Simecka and a journalist "
             "about rigging the election."),
            ("Detection happened.",
             "But not before significant viral spread, days before "
             "voting."),
            ("Outcome contested.",
             "His party narrowly lost; impossible to say if the "
             "deepfake was decisive."),
            ("The first time a deepfake was credibly tied to an "
             "election outcome.",
             "Not the last."),
        ])
    add_footer(s, 8, TOTAL, BRAND)

    s = content_slide(prs, "US NH primary January 2024: AI robocalls",
        bullets=[
            ("Two days before the New Hampshire Democratic primary.",
             "A robocall went out impersonating President Biden's "
             "voice, urging voters to skip the primary."),
            ("Reach: estimated 5,000 to 25,000 calls.",
             "FCC fined the source $6M in May 2024."),
            ("This was a US federal election.",
             "Not a hypothetical. Not a foreign country. Not a fringe "
             "platform."),
            ("The technical bar.",
             "Eleven Labs voice cloning. Public Biden recordings. "
             "Roughly $1 of compute."),
            ("FCC banned AI-generated robocalls in February 2024.",
             "After the fact."),
        ])
    add_footer(s, 9, TOTAL, BRAND)

    s = content_slide(prs, "Romania November 2024: election annulled",
        bullets=[
            ("First round of Romanian presidential elections held "
             "November 24, 2024.",
             "Far-right candidate Calin Georgescu won surprisingly "
             "with significant margin."),
            ("Days later: Romania's intelligence services declassified "
             "evidence of coordinated TikTok campaign.",
             "Estimated 25,000 accounts. Massive use of "
             "AI-generated content."),
            ("December 6, 2024: Constitutional Court annulled the "
             "election.",
             "First time in EU history."),
            ("Re-vote held in May 2025.",
             "Different outcome, but the precedent is set."),
            ("The lesson.",
             "Even when caught, deepfake-driven elections can pass "
             "the threshold of legitimacy. Annulment is the extreme "
             "remedy."),
        ])
    add_footer(s, 10, TOTAL, BRAND)

    s = _im(prs, "Deepfake media in circulation: log-scale growth",
        "chart_deepfake_growth.png",
        caption="Sumsub Identity Fraud Report 2024; DeepMedia; "
                "Reality Defender 2024 industry estimates.",
        image_h=4.4)
    add_footer(s, 11, TOTAL, BRAND)

    s = content_slide(prs, "Beyond politics: financial fraud, NCII, scams",
        bullets=[
            ("Financial fraud via deepfake video calls.",
             "February 2024: Hong Kong CFO transferred $25M after "
             "video conference call where multiple 'colleagues' "
             "were deepfakes."),
            ("Non-consensual intimate imagery.",
             "Internet Watch Foundation: 11x increase in AI-generated "
             "CSAM 2023-2024. Catastrophic individual harm."),
            ("Romance scams.",
             "Pig-butchering schemes now use AI video to impersonate "
             "love interests."),
            ("Corporate impersonation.",
             "Fake CEO video issued to investors during M&A "
             "negotiations."),
            ("This is not a future problem.",
             "It is a present one. The trust crisis touches every "
             "domain that relies on visual / audio authenticity."),
        ])
    add_footer(s, 12, TOTAL, BRAND)

    # Section 2
    s = section_divider(prs, 2, "Why Detection Is a Losing Arms Race",
        "Asymmetry, evasion, the substrate problem.")
    add_footer(s, 13, TOTAL, BRAND)

    s = _im(prs, "Detection accuracy declining as generation improves",
        "chart_detection_race.png",
        caption="Composite: Reality Defender, Pindrop, DeepMedia "
                "industry reports.",
        image_h=4.2)
    add_footer(s, 14, TOTAL, BRAND)

    s = content_slide(prs, "The asymmetry problem", bullets=[
        ("Detection requires near-100% accuracy at scale.",
         "False positives erode trust in real content. False "
         "negatives erode trust in detection."),
        ("Generation requires only ONE successful piece.",
         "Slovakia: one audio file. NH primary: one robocall script. "
         "The attacker only needs one."),
        ("Generation cost approaches zero.",
         "Voice cloning: minutes. Video: hours. Cost: low single "
         "digits of dollars."),
        ("Detection cost is increasing.",
         "Each generator improvement requires retraining classifiers "
         "on new examples."),
        ("Compounding effect.",
         "Defender investment scales linearly; attacker capability "
         "scales with model frontier (rapidly)."),
    ])
    add_footer(s, 15, TOTAL, BRAND)

    s = content_slide(prs, "The evasion dynamic", bullets=[
        ("Adversarial robustness is largely an unsolved problem.",
         "Goodfellow et al 2014 explained adversarial examples; "
         "10 years later, no general defense."),
        ("Anti-detection techniques.",
         "Adversarial perturbations, model fingerprint laundering, "
         "post-processing filters."),
        ("Defender publishes detection method.",
         "Attacker now knows what to evade. Publishing accelerates "
         "the arms race."),
        ("Defender doesn't publish.",
         "Then independent verification of detection accuracy is "
         "impossible. Public can't audit."),
        ("Either way, defenders are at a structural disadvantage.",
         "This is why detection alone cannot win."),
    ])
    add_footer(s, 16, TOTAL, BRAND)

    s = content_slide(prs, "What detection IS good for", bullets=[
        ("Triage at scale.",
         "Platforms processing billions of uploads need a fast filter. "
         "Detection is fine here, even if imperfect."),
        ("Bulk content review.",
         "Pre-screening for human reviewers in trust and safety teams."),
        ("Forensic post-hoc analysis.",
         "After incidents, detection helps identify what was AI-"
         "generated for legal/journalistic purposes."),
        ("Generator-specific defense.",
         "When you know the attacker is using a specific model, "
         "targeted detection can work."),
        ("What detection cannot do.",
         "Provide individual users with reliable trust signals at "
         "consumption time. That requires provenance."),
    ])
    add_footer(s, 17, TOTAL, BRAND)

    s = quote_slide(prs,
        "We need provenance-first architecture, not detection-first. "
        "Sign at capture, verify at display, trust per-observer.",
        "The strategic pivot",
        title="The pivot")
    add_footer(s, 18, TOTAL, BRAND)

    # Section 3
    s = section_divider(prs, 3, "What C2PA Gets Right",
        "Coalition for Content Provenance and Authenticity. Layer 1 "
        "of the defense.")
    add_footer(s, 19, TOTAL, BRAND)

    s = content_slide(prs, "C2PA core model", bullets=[
        ("Coalition for Content Provenance and Authenticity.",
         "Founded 2021 by Adobe, Microsoft, Truepic, BBC, Sony, "
         "Nikon, others. Open spec at c2pa.org."),
        ("Manifest attached to media.",
         "JSON document signed by the camera or editor. Contains "
         "claims about the content."),
        ("Claims include.",
         "Capture device, location (optional), editing actions, "
         "creator identity, timestamp."),
        ("Each transformation adds a signed assertion.",
         "Camera signs at capture. Editor signs after edits. "
         "Publisher signs at publication."),
        ("Reader / viewer verifies the chain at display.",
         "If signatures verify, reader sees a 'verified' indicator. "
         "If broken, sees 'unable to verify'."),
    ])
    add_footer(s, 20, TOTAL, BRAND)

    s = _im(prs, "C2PA capture-to-display chain",
        "chart_c2pa.png", image_h=3.8)
    add_footer(s, 21, TOTAL, BRAND)

    s = content_slide(prs, "What's attested in a C2PA manifest",
        bullets=[
            ("Recording device fingerprint.",
             "If using a C2PA-enabled camera (Sony Alpha 9 III, "
             "Leica M11-P, Nikon Z9 firmware), device signs at "
             "capture."),
            ("Edit operations.",
             "Crop, color correction, resize: each appears as a "
             "signed assertion in the chain."),
            ("Creator identity.",
             "Optional, but most newsroom workflows include "
             "photographer ID."),
            ("Timestamp.",
             "When the media was captured and edited."),
            ("AI generation flags.",
             "If any AI tool was used, the type and extent are "
             "logged."),
        ])
    add_footer(s, 22, TOTAL, BRAND)

    s = content_slide(prs, "Why C2PA is well-designed", bullets=[
        ("Cryptographic, not heuristic.",
         "Verification is deterministic. No 'detection probability' "
         "to interpret."),
        ("Open standard.",
         "Spec at c2pa.org. Anyone can implement. No vendor lock-in."),
        ("Backwards compatible.",
         "Media without C2PA still displays. Just lacks the "
         "verification badge."),
        ("Privacy aware.",
         "Creator can choose what to include. Location optional. "
         "Identity optional."),
        ("Multi-vendor traction.",
         "Adobe Content Credentials, Truepic, Microsoft, BBC, AP, "
         "Reuters all engaged."),
    ])
    add_footer(s, 23, TOTAL, BRAND)

    s = content_slide(prs, "C2PA deployment status (early 2026)",
        bullets=[
            ("C2PA-enabled cameras shipping.",
             "Sony Alpha 9 III, Leica M11-P, Nikon Z9 firmware update."),
            ("Adobe Content Credentials.",
             "Photoshop, Lightroom, Premiere Pro all attach C2PA "
             "manifests for AI-generated content."),
            ("AP and Reuters.",
             "All wire-service photos signed at point of capture or "
             "import."),
            ("Browser display.",
             "Chrome, Edge, Firefox have varying levels of C2PA "
             "manifest UI."),
            ("Adoption gap.",
             "Maybe 5% of news media bears valid C2PA today. Most "
             "social-media-shared content does not."),
        ])
    add_footer(s, 24, TOTAL, BRAND)

    s = content_slide(prs, "Where C2PA stops", bullets=[
        ("C2PA tells you who SIGNED the content.",
         "Adobe says 'this was edited in Photoshop.' Sony says "
         "'this came from a Sony camera.'"),
        ("It does NOT tell you whether to trust the signer.",
         "Adobe is signing the editor; the editor's brand is "
         "separate."),
        ("It does NOT tell you the publisher's reputation.",
         "Adobe doesn't know if Reuters or InfoWars is publishing "
         "the result."),
        ("It does NOT give you per-observer trust.",
         "Everyone sees the same C2PA badge regardless of their "
         "trust in the publisher."),
        ("These are Layers 2 and 3.",
         "C2PA is necessary; it is not sufficient for a complete "
         "defense."),
    ])
    add_footer(s, 25, TOTAL, BRAND)

    # Section 4
    s = section_divider(prs, 4, "Layer 2: DNS-Anchored Publisher Identity",
        "Bind existing trusted brands to cryptographic identity.")
    add_footer(s, 26, TOTAL, BRAND)

    s = content_slide(prs, "The existing infrastructure we can leverage",
        bullets=[
            ("DNS is the most-deployed identity system in human history.",
             "Every news organization has a domain. Every domain has "
             "a verifiable owner via DNS records and TLS certificates."),
            ("Reuters.com is Reuters.",
             "BBC.co.uk is the BBC. Fox.com is Fox News. The brand "
             "lives in the domain name."),
            ("DNSSEC + Certificate Transparency.",
             "Two cryptographic layers below most people's awareness "
             "already authenticate domain ownership."),
            ("Quidnug DNS-anchored attestation (QDP-0023).",
             "Lets a Quidnug quid prove DNS ownership, then carry "
             "that brand identity into Quidnug's trust graph."),
            ("Bootstrap is essentially free.",
             "Existing news organizations don't need new identity. "
             "They have one."),
        ])
    add_footer(s, 27, TOTAL, BRAND)

    s = content_slide(prs, "How the binding works", bullets=[
        ("Reuters publishes a TXT record at _quidnug.reuters.com.",
         "Containing their Quidnug quid (16-char hex)."),
        ("Quidnug attestation root checks the record.",
         "Must match the Quidnug quid that signs articles."),
        ("Attestation root signs an attestation.",
         "'Reuters.com is owned by quid abcdef1234567890 as of "
         "2026-04-22.'"),
        ("Renewable.",
         "Annual or quarterly renewal. Revocation by domain owner "
         "or attestation root if compromised."),
        ("End result.",
         "Reader's UI can display 'Verified Reuters' alongside the "
         "C2PA badge with cryptographic certainty."),
    ])
    add_footer(s, 28, TOTAL, BRAND)

    s = content_slide(prs, "Why DNS-anchored identity works", bullets=[
        ("Existing trust transfer.",
         "Decades of brand recognition for major publishers. The "
         "binding makes that brand machine-verifiable."),
        ("No new authority required.",
         "Reuters does not need to convince anyone they ARE Reuters. "
         "The DNS record proves it."),
        ("Adversary cost.",
         "To impersonate Reuters, an attacker would have to "
         "compromise reuters.com (DNSSEC + cert validation). Far "
         "harder than 'create a website.'"),
        ("Compatible with takedown.",
         "If a publisher loses the domain, attestation auto-expires. "
         "No vendor lock-in."),
        ("Federation-friendly.",
         "Works equally for major outlets and small bloggers. "
         "DNS scales to all."),
    ])
    add_footer(s, 29, TOTAL, BRAND)

    s = content_slide(prs, "Composition with C2PA", bullets=[
        ("C2PA proves: 'This image came from a Sony camera and was "
         "edited in Photoshop.'",
         ""),
        ("DNS-anchored adds: 'And it was published by reuters.com.'",
         ""),
        ("Both layers use cryptographic verification.",
         "No conflict between them. They compose."),
        ("Reader UI shows both.",
         "C2PA chain badge plus 'Verified Reuters' badge."),
        ("Attack surface dramatically reduced.",
         "Faking C2PA + DNS together requires compromising both "
         "the camera vendor's signing infrastructure AND the "
         "publisher's domain."),
    ])
    add_footer(s, 30, TOTAL, BRAND)

    s = content_slide(prs, "Institutional coverage", bullets=[
        ("News organizations.",
         "Every reuters.com, ap.org, nytimes.com, bbc.co.uk could "
         "be DNS-anchored within months."),
        ("Government.",
         ".gov, whitehouse.gov, treasury.gov etc. official "
         "communications cryptographically anchored to domain."),
        ("Universities.",
         ".edu domains for academic publications. mit.edu = MIT "
         "verified."),
        ("Major brands.",
         "Apple, Google, Microsoft press releases verifiably theirs."),
        ("Citizen journalists and small publishers.",
         "Anyone with a domain can DNS-anchor. Lower the bar to "
         "verified-publisher status."),
    ])
    add_footer(s, 31, TOTAL, BRAND)

    # Section 5
    s = section_divider(prs, 5, "Layer 3: Relational Trust",
        "Per-observer publisher weighting. Composes with C2PA + DNS.")
    add_footer(s, 32, TOTAL, BRAND)

    s = content_slide(prs, "The per-observer problem", bullets=[
        ("C2PA + DNS gives you 'this is genuinely from publisher X.'",
         "It does NOT tell you whether to trust publisher X."),
        ("Different readers reasonably trust different publishers.",
         "A US conservative may trust Fox News and distrust the New "
         "York Times. The opposite may also be true."),
        ("A platform that imposes a global trust hierarchy is "
         "censoring.",
         "But a platform that ignores trust signals entirely is "
         "useless."),
        ("Relational trust solves this.",
         "Each reader has their own trust graph. Quidnug PoT computes "
         "publisher trust per-observer."),
        ("Compatible with both ends of the political spectrum.",
         "Trust is the user's choice; the platform provides the "
         "computation, not the verdict."),
    ])
    add_footer(s, 33, TOTAL, BRAND)

    s = _im(prs, "One observer's publisher trust weighting",
        "chart_publisher_trust.png",
        caption="Each viewer's weighting is their own. The platform "
                "shows them what their trusted sources think.",
        image_h=4.0)
    add_footer(s, 34, TOTAL, BRAND)

    s = content_slide(prs, "Operationalizing on content", bullets=[
        ("Each piece of media has a publisher quid (Layer 2).",
         "Reader's trust in that publisher determines weight."),
        ("Aggregate trust across multiple publishers covering the "
         "same event.",
         "If Reuters, AP, and BBC all publish similar accounts, "
         "trust is high. If only InfoWars covers it, trust is much "
         "lower for most observers."),
        ("Same four-factor formula as relativistic ratings.",
         "Trust × event coverage × recency × independence."),
        ("Per-observer aggregation.",
         "Each reader sees the trust score of an event computed "
         "from THEIR trusted publishers."),
        ("Filter bubble concern addressed.",
         "Platform can show 'how would your view differ if you "
         "weighted publishers differently?' (Optional control)."),
    ])
    add_footer(s, 35, TOTAL, BRAND)

    s = content_slide(prs, "The display model", bullets=[
        ("Verification badge: cryptographic.",
         "C2PA verified, DNS verified, no question of reality."),
        ("Trust score: per-observer, calibrated.",
         "'Your trusted sources rate this story 0.85.'"),
        ("Provenance trace: drilldown.",
         "Click to see the C2PA chain, the publisher attestation, "
         "and your trust path to the publisher."),
        ("Crowd score: optional comparison.",
         "'The general public's average is 0.62.' Always available "
         "as a sanity check."),
        ("UX similar to existing news platforms.",
         "Layered detail. Most users see the badge; curious users "
         "drill down."),
    ])
    add_footer(s, 36, TOTAL, BRAND)

    s = content_slide(prs, "User opt-in calibration", bullets=[
        ("Onboarding: ask the user about their trust priors.",
         "'Which of these publishers do you trust?' List of major "
         "outlets."),
        ("From a few seed selections.",
         "Quidnug trust graph propagates: who do those publishers "
         "trust? Builds a starting graph."),
        ("Ongoing adjustment.",
         "User can up-weight or down-weight any publisher at any "
         "time. Visible UI."),
        ("Diversity nudge.",
         "Optional: 'Your trust graph is concentrated in conservative "
         "outlets. Consider weighting some others?' Opt-in."),
        ("Always reversible.",
         "User remains in control. Platform never imposes."),
    ])
    add_footer(s, 37, TOTAL, BRAND)

    # Section 6
    s = section_divider(prs, 6, "Protecting Sources Without Exposing Them",
        "Selective disclosure for investigative journalism.")
    add_footer(s, 38, TOTAL, BRAND)

    s = content_slide(prs, "The tension", bullets=[
        ("Investigative journalism depends on confidential sources.",
         "Snowden, Watergate, Panama Papers, every major exposure."),
        ("A signed-content world raises a question.",
         "How do you sign source content without identifying the "
         "source?"),
        ("Naive answer: don't sign source material.",
         "But then we lose verification. Adversaries claim 'this "
         "leak is fake.'"),
        ("Better answer: signed by the JOURNALIST, with selective "
         "disclosure of source metadata.",
         "Cryptographic credibility without identity exposure."),
        ("Quidnug primitives that enable this.",
         "Selective disclosure (QDP-0024 partial), private "
         "communications (QDP-0024 full), and pseudonymous quids."),
    ])
    add_footer(s, 39, TOTAL, BRAND)

    s = content_slide(prs, "What Quidnug offers: selective disclosure",
        bullets=[
            ("Source has a Quidnug identity (could be pseudonymous).",
             "Journalist receives content from this identity over an "
             "encrypted channel."),
            ("Journalist signs an attestation.",
             "'I have verified this content originates from a source "
             "with these credentials [doctor, hospital insider, "
             "verified by my newspaper] without disclosing identity.'"),
            ("Reader's trust calculation.",
             "Trust in the journalist + trust in the journalist's "
             "selective disclosure verification."),
            ("Not blind trust.",
             "Reader knows the journalist's track record. Bad sources "
             "tank the journalist's reputation; honest sources "
             "build it."),
            ("Source identity remains protected.",
             "Cryptographic separation between source identity and "
             "verification of source credentials."),
        ])
    add_footer(s, 40, TOTAL, BRAND)

    s = content_slide(prs, "QDP-0024: private communications", bullets=[
        ("Group-keyed encryption built on MLS protocol.",
         "RFC 9420. Industrial-grade end-to-end encryption."),
        ("Source-journalist channel.",
         "Encrypted, with forward secrecy. Past messages remain "
         "private even if current keys are compromised."),
        ("Selective metadata disclosure.",
         "Journalist can prove 'I received this from someone with "
         "X credentials' without revealing who or how."),
        ("Compatible with publishing.",
         "When ready to publish, journalist signs the article with "
         "embedded source-credibility attestations."),
        ("Compatible with legal protection.",
         "Cryptographic separation gives stronger source protection "
         "than current 'trust the journalist's notes' model."),
        ])
    add_footer(s, 41, TOTAL, BRAND)

    s = content_slide(prs, "The full stack for investigative journalism",
        bullets=[
            ("Source obtains evidence (documents, video, audio).",
             ""),
            ("Source contacts journalist via encrypted channel.",
             "Quidnug private communications (QDP-0024)."),
            ("Journalist verifies authenticity in their newsroom.",
             "Internal review process, fact-checking, legal review."),
            ("Journalist publishes article.",
             "C2PA-signed manuscript. Selective-disclosure attestation "
             "of source credibility. DNS-anchored to publisher."),
            ("Reader's trust calculation.",
             "Layer 1 (C2PA verified) + Layer 2 (verified publisher) "
             "+ Layer 3 (per-observer trust in publisher)."),
            ("Source remains protected throughout.",
             "End-to-end."),
        ])
    add_footer(s, 42, TOTAL, BRAND)

    s = content_slide(prs, "The credibility-without-identity tradeoff",
        bullets=[
            ("Stronger than current 'trust the journalist's word.'",
             "Cryptographic verification of journalist's attestation; "
             "source verification process explicit."),
            ("Weaker than 'fully named source.'",
             "Reader cannot independently verify the source. Must "
             "trust the journalist's verification."),
            ("Better fit for the modern threat model.",
             "Sources are increasingly at risk. Source protection "
             "is increasingly important."),
            ("Newsroom incentives align.",
             "Reputation tracks accurate sourcing. Burning sources "
             "or accepting bad ones costs reputation."),
            ("This is the architecture for the next decade of "
             "investigative work.",
             "Same primitives that enable healthcare consent and "
             "elections."),
        ])
    add_footer(s, 43, TOTAL, BRAND)

    # Section 7
    s = section_divider(prs, 7, "End-to-End Architecture",
        "The full pipeline from witness to reader.")
    add_footer(s, 44, TOTAL, BRAND)

    s = _im(prs, "End-to-end trust pipeline",
        "chart_full_stack.png",
        caption="Source to reader, every node attestable, every "
                "edge verifiable.",
        image_h=4.4)
    add_footer(s, 45, TOTAL, BRAND)

    s = _im(prs, "The three-layer defense, explicit",
        "chart_three_layers.png", image_h=4.4)
    add_footer(s, 46, TOTAL, BRAND)

    s = content_slide(prs, "Worked example: Reuters publishes a war photo",
        bullets=[
            ("Step 1.",
             "Photographer Sarah uses Sony Alpha 9 III in the field. "
             "Camera signs C2PA at capture."),
            ("Step 2.",
             "Sarah uploads to Reuters editing pipeline. Editor "
             "crops; edit signed."),
            ("Step 3.",
             "Reuters editorial sign-off. Article published with "
             "Reuters publisher signature."),
            ("Step 4.",
             "Distribution to subscriber platforms (NYTimes, "
             "Washington Post, etc). Each receives signed article."),
            ("Step 5.",
             "Reader on Twitter sees the photo. Twitter UI displays:"
             " 'Verified by Reuters via Sony Alpha 9.'"),
            ("Step 6.",
             "Reader's Quidnug trust graph: 0.85 trust in Reuters. "
             "Composite trust: very high."),
        ])
    add_footer(s, 47, TOTAL, BRAND)

    s = content_slide(prs, "What fails under this architecture",
        bullets=[
            ("Adversary scrapes the photo, strips C2PA.",
             "Reposts on social media. Loses verification badge. "
             "Readers see no trust signal."),
            ("Adversary creates a fake publisher.",
             "deepfake-news.com. No DNS attestation, no major "
             "publisher trust. Most readers' trust graphs weight "
             "this near zero."),
            ("Adversary uses a cracked C2PA signing key.",
             "Camera vendor revokes; chain breaks. Visible to "
             "readers."),
            ("Adversary impersonates Reuters.",
             "Has to compromise reuters.com domain or DNS. Hard."),
            ("Adversary publishes a real photo with false caption.",
             "Image is verified. Caption is the journalist's claim. "
             "Journalist's reputation absorbs the cost."),
        ])
    add_footer(s, 48, TOTAL, BRAND)

    s = _im(prs, "Attack surface: with vs without three-layer defense",
        "chart_attack_surface.png", image_h=4.4)
    add_footer(s, 49, TOTAL, BRAND)

    # Section 8
    s = section_divider(prs, 8, "Adoption Strategy and Tradeoffs",
        "Who moves first. What the next 5 years look like.")
    add_footer(s, 50, TOTAL, BRAND)

    s = _im(prs, "Projected adoption curves",
        "chart_adoption.png",
        caption="Layer 1 (C2PA) leads; Layers 2-3 follow over 5-7 years.",
        image_h=4.0)
    add_footer(s, 51, TOTAL, BRAND)

    s = content_slide(prs, "Who moves first", bullets=[
        ("AP and Reuters.",
         "Already C2PA-engaged. Could DNS-anchor in months. "
         "Industry moves with them."),
        ("Major broadcasters: BBC, NHK, ARD, France TV.",
         "Public-service broadcasters have institutional incentives "
         "for credibility infrastructure."),
        ("Investigative outlets: ProPublica, ICIJ, OCCRP.",
         "Source-protection use case is most acute for them."),
        ("Camera and software vendors.",
         "Sony, Nikon, Adobe, Truepic already in C2PA. They're the "
         "Layer 1 ecosystem."),
        ("Browser vendors.",
         "Chrome, Edge, Firefox already adding C2PA UI. Layer 2-3 "
         "extensions follow."),
    ])
    add_footer(s, 52, TOTAL, BRAND)

    s = content_slide(prs, "What the next 5 years look like", bullets=[
        ("Year 1.",
         "More C2PA-enabled cameras and editors. Major news "
         "organizations DNS-anchor. Pilot platforms display "
         "trust badges."),
        ("Year 2.",
         "Critical mass of mainstream news C2PA-signed. "
         "Browser-native verification UI ships."),
        ("Year 3.",
         "Per-observer trust calibration UI in major news platforms. "
         "Citizen journalists routinely DNS-anchor."),
        ("Year 4.",
         "Detection-only fact-checking augmented by provenance "
         "analysis. Election misinformation reduced."),
        ("Year 5.",
         "Three-layer architecture is standard. Unverified content "
         "is flagged by default. The trust ecosystem rebuilds."),
    ])
    add_footer(s, 53, TOTAL, BRAND)

    s = content_slide(prs, "Honest tradeoff 1: legacy content",
        bullets=[
            ("Decades of pre-C2PA media exists.",
             "Cannot be retroactively signed. Legacy content stays "
             "unverified."),
            ("Not a fatal problem.",
             "Future content gets signed; legacy content remains in "
             "legacy mode. Coexistence."),
            ("Some institutions can sign retroactively with "
             "attestations.",
             "'We the BBC verify this 2010 footage from our "
             "archives.' Different signature; still verifiable."),
            ("Legacy will be a smaller fraction of viewed content "
             "over time.",
             "New content shipped daily; legacy fades naturally."),
            ("This is also true for HTTPS.",
             "Decade-old http:// pages still exist. New ones are "
             "https://. We adapted."),
        ])
    add_footer(s, 54, TOTAL, BRAND)

    s = content_slide(prs, "Honest tradeoff 2: privacy concerns",
        bullets=[
            ("Photographer identity in C2PA manifest can leak source "
             "info.",
             "Especially in conflict zones or authoritarian regimes."),
            ("Mitigation: pseudonymous credentials.",
             "Reuters can attest 'this is from a verified Reuters "
             "stringer' without naming the photographer."),
            ("Location stripping.",
             "C2PA allows excluding location metadata. Newsrooms "
             "should default to strip in sensitive contexts."),
            ("Selective disclosure.",
             "Same primitive that protects sources protects "
             "photographers in dangerous situations."),
            ("Privacy is a feature, not a bug.",
             "The architecture explicitly supports it."),
        ])
    add_footer(s, 55, TOTAL, BRAND)

    s = content_slide(prs, "Honest tradeoff 3: institutional trust",
        bullets=[
            ("DNS-anchored identity assumes major publishers are "
             "trustworthy.",
             "Many readers (left, right, otherwise) increasingly "
             "doubt that."),
            ("The architecture does not force trust.",
             "It makes trust user-controllable. Reader chooses "
             "weights; doesn't have to trust everyone."),
            ("Trust must be earned over time.",
             "Publishers caught publishing falsehoods see their "
             "trust decline in user calibration."),
            ("Smaller, specialized publishers.",
             "DNS-anchored identity equally available. Substack "
             "writers, citizen journalists, etc."),
            ("Decentralization possible.",
             "Federation rather than monoculture. Many different "
             "trust hierarchies coexist."),
        ])
    add_footer(s, 56, TOTAL, BRAND)

    s = content_slide(prs, "Honest tradeoff 4: technical complexity",
        bullets=[
            ("C2PA + DNS attestation + Quidnug trust graph is more "
             "complex than 'view the picture.'",
             ""),
            ("Most users don't need to understand the layers.",
             "UI abstracts: 'Verified Reuters - Trust 0.85'. Three "
             "layers reduced to one badge."),
            ("Curious users can drill down.",
             "Click to see C2PA chain, DNS attestation, trust path. "
             "Optional."),
            ("Implementation engineers need to know the layers.",
             "Same as TLS, OAuth, DNSSEC. Not user-visible "
             "complexity."),
            ("Standards mature over time.",
             "C2PA spec stabilizing. DNS attestation following the "
             "same path. Quidnug trust graph similarly."),
        ])
    add_footer(s, 57, TOTAL, BRAND)

    s = content_slide(prs, "What this protocol does NOT solve",
        bullets=[
            ("Real photos with false captions.",
             "Image verifies; caption is journalist's claim. Trust "
             "in journalist absorbs the cost."),
            ("Honest reporting on uncertain events.",
             "Provenance verifies what was reported, not whether "
             "it's true."),
            ("State-actor disinformation with full publisher "
             "infrastructure.",
             "If a state-controlled publisher is DNS-anchored, "
             "they're verified; trust is in the user's hands."),
            ("Confirmation bias.",
             "User who only trusts publishers that align with their "
             "priors will continue to do so."),
            ("This is one defense layer.",
             "Critical thinking and media literacy still matter."),
        ])
    add_footer(s, 58, TOTAL, BRAND)

    s = content_slide(prs, "What to do this year (newsroom)", bullets=[
        ("Adopt C2PA at scale.",
         "Camera firmware updates, editing workflow integration."),
        ("DNS-anchor your publication.",
         "Quidnug attestation root + TXT record. Few days of work."),
        ("Train staff on selective disclosure for source protection.",
         "Internal workflows for sensitive sources."),
        ("Update publication standards.",
         "Sign articles. Attestation as part of editorial "
         "completion."),
        ("Lobby browser vendors.",
         "Push for C2PA UI to be enabled by default."),
    ])
    add_footer(s, 59, TOTAL, BRAND)

    s = content_slide(prs, "What to do this year (platform)", bullets=[
        ("Display verification badges.",
         "C2PA + DNS attestation visible to users."),
        ("Build trust calibration UI.",
         "Let users adjust their trust weights. Make it visible "
         "and reversible."),
        ("Surface provenance on demand.",
         "Drill-down for curious users. C2PA chain, publisher "
         "attestation, trust path."),
        ("Don't impose a global trust hierarchy.",
         "Let users decide. Platform provides the math, not the "
         "verdict."),
        ("Combat unverified content.",
         "Don't ban; flag. 'This content has no provenance "
         "verification' lets users decide."),
    ])
    add_footer(s, 60, TOTAL, BRAND)

    s = content_slide(prs, "What to do this year (regulator / policy)",
        bullets=[
            ("Mandate C2PA for synthetic media disclosure.",
             "EU AI Act + similar laws can require provenance "
             "metadata for AI-generated content."),
            ("Fund the standards.",
             "C2PA spec, browser implementation, library development. "
             "Public-good investment."),
            ("Recognize cryptographic provenance in legal "
             "frameworks.",
             "Court admissibility of signed evidence."),
            ("Avoid centralized authority.",
             "DNS + open standards, not government certification "
             "regime."),
            ("Coordinate internationally.",
             "EU + US + Asia alignment on C2PA + similar standards. "
             "Avoid fragmentation."),
        ])
    add_footer(s, 61, TOTAL, BRAND)

    s = content_slide(prs, "What to do this year (citizen)", bullets=[
        ("Look for verification badges.",
         "Train yourself to expect them. Suspect content without "
         "them."),
        ("Calibrate your trust graph thoughtfully.",
         "Diverse sources. Not just one outlet."),
        ("Push platforms to display provenance.",
         "Demand the verification UI."),
        ("Support C2PA-enabled creators.",
         "Photographers, journalists, news organizations who sign "
         "their work."),
        ("Be skeptical of unverified viral content.",
         "Especially political, especially before elections."),
    ])
    add_footer(s, 62, TOTAL, BRAND)

    s = quote_slide(prs,
        "C2PA + DNS-anchored identity + relational trust. "
        "Three layers, none sufficient alone, all necessary together.",
        "The deepfake defense in one sentence",
        title="The architecture in one sentence")
    add_footer(s, 63, TOTAL, BRAND)

    s = content_slide(prs, "Summary: the four claims revisited",
        bullets=[
            ("1. Detection is necessary but losing.",
             "Generator quality outpaces classifier accuracy. Defenders "
             "must shift strategy."),
            ("2. Provenance-first architecture is the durable answer.",
             "Sign at capture, verify at display, trust per-observer."),
            ("3. C2PA is correct but insufficient.",
             "Layer 1 of three. Layers 2 (DNS) and 3 (relational "
             "trust) close the gap."),
            ("4. Source protection composes with provenance.",
             "Selective disclosure preserves credibility without "
             "identity exposure."),
        ])
    add_footer(s, 64, TOTAL, BRAND)

    s = content_slide(prs, "References", bullets=[
        ("Coalition for Content Provenance and Authenticity (C2PA).",
         "c2pa.org. Spec v1.3 (2025)."),
        ("Adobe Content Credentials.",
         "contentcredentials.org."),
        ("Sumsub Identity Fraud Report 2024.",
         "Documented deepfake growth metrics."),
        ("Reality Defender 2024 industry report.",
         "Detection accuracy trends."),
        ("EU AI Act 2024.",
         "Article 50: synthetic content disclosure obligations."),
        ("Romania Constitutional Court Decision (Dec 6, 2024).",
         "First election annulment over deepfake-driven manipulation."),
        ("Quidnug QDP-0023 (DNS-anchored attestation).",
         "github.com/quidnug/quidnug/docs/design/0023-*."),
        ("Quidnug QDP-0024 (private communications).",
         "github.com/quidnug/quidnug/docs/design/0024-*."),
    ])
    add_footer(s, 65, TOTAL, BRAND)

    s = content_slide(prs, "More references", bullets=[
        ("Goodfellow, Shlens, Szegedy (2014). Adversarial examples.",
         "ICLR 2015."),
        ("Internet Watch Foundation (2024). AI CSAM report.",
         "iwf.org.uk."),
        ("FCC ruling on AI-generated robocalls (Feb 2024).",
         "fcc.gov."),
        ("Microsoft Reuters AP collaboration on C2PA newsroom workflows.",
         ""),
        ("Companion blog post.",
         "blogs/2026-04-24-deepfakes-trust-endgame.md."),
        ("DNSSEC and Certificate Transparency.",
         "Existing infrastructure C2PA + DNS attestation builds on."),
        ("WEF Global Risks Report 2024.",
         "Misinformation ranked #1 global risk."),
    ])
    add_footer(s, 66, TOTAL, BRAND)

    s = content_slide(prs, "Common objections, briefly", bullets=[
        ("'C2PA can be stripped.'",
         "True. The badge then shows 'unable to verify.' Users "
         "learn to distrust unverified content."),
        ("'DNS can be hijacked.'",
         "Rarely, briefly. DNSSEC + Certificate Transparency raise "
         "the bar substantially."),
        ("'Per-observer trust = filter bubble.'",
         "Constellation view shows the crowd alongside personalized. "
         "User-controlled, transparent."),
        ("'This will hurt small publishers.'",
         "Opposite. DNS-anchored identity equally available. Lowers "
         "the bar to verified-publisher status."),
        ("'Detection still catches some attacks.'",
         "Yes. Detection complements provenance. We don't choose; "
         "we use both."),
    ])
    add_footer(s, 67, TOTAL, BRAND)

    s = content_slide(prs, "What success looks like in 2030", bullets=[
        ("Mainstream news media routinely C2PA-signed.",
         "Unsigned content is the suspicious exception, not the rule."),
        ("Major publishers DNS-anchored.",
         "Verified Reuters, Verified BBC, Verified Le Monde "
         "displayed natively in browsers."),
        ("User-side trust calibration ubiquitous.",
         "Every news platform has a trust UI. Personalized, "
         "transparent, reversible."),
        ("Source protection workflow standardized.",
         "Selective disclosure attestations a normal part of "
         "investigative journalism."),
        ("Election deepfake incidents drop sharply.",
         "Not zero, but no longer effective at scale because "
         "verification is the default."),
    ])
    add_footer(s, 68, TOTAL, BRAND)

    s = quote_slide(prs,
        "Detection is the rear-guard. Provenance is the future. "
        "Build for what comes next, not for what we wish hadn't "
        "happened.",
        "The strategic outlook",
        title="The outlook")
    add_footer(s, 69, TOTAL, BRAND)

    s = content_slide(prs, "What this conversation enables", bullets=[
        ("Healthier elections.",
         "Verified candidate communication. Lower deepfake "
         "effectiveness."),
        ("Better investigative journalism.",
         "Source protection + cryptographic credibility."),
        ("Restored public trust in media.",
         "Visible chain of evidence."),
        ("Reduced financial fraud.",
         "Verified business communication."),
        ("Reduced individual harm.",
         "Lower NCII spread, lower romance scam success."),
        ("This is foundational infrastructure.",
         "Not a feature; not a product. A substrate for the next "
         "twenty years of trust."),
    ])
    add_footer(s, 70, TOTAL, BRAND)

    s = content_slide(prs, "Things we owe ourselves", bullets=[
        ("Coordinated international standards.",
         "EU + US + Asia alignment on C2PA + DNS attestation + "
         "relational trust."),
        ("Open implementations.",
         "Anyone should be able to verify; not gated by vendor."),
        ("Public-interest funding.",
         "Foundation grants, government infrastructure investment. "
         "Not VC-only."),
        ("Researcher independence.",
         "Adversarial robustness testing by neutral parties, not "
         "just vendors."),
        ("Civil liberties oversight.",
         "Privacy and free speech advocacy in standards bodies. "
         "EFF, ACLU, Article 19."),
    ])
    add_footer(s, 71, TOTAL, BRAND)

    s = content_slide(prs, "What this protocol explicitly avoids",
        bullets=[
            ("No central truth authority.",
             "DNS is decentralized infrastructure; no Ministry of "
             "Verified Content."),
            ("No mandatory tracking.",
             "Photographers can stay pseudonymous. Sources stay "
             "protected."),
            ("No global trust hierarchy.",
             "Each user's trust graph is their own."),
            ("No platform censorship via verification.",
             "Unverified content is flagged, not blocked."),
            ("This is infrastructure, not editorial.",
             "It tells you WHO published; not WHETHER to believe."),
        ])
    add_footer(s, 72, TOTAL, BRAND)

    s = quote_slide(prs,
        "We do not need to detect every deepfake. We need to "
        "make signed authenticity the default. The asymmetry "
        "shifts back to defenders.",
        "The strategic insight",
        title="The strategic insight")
    add_footer(s, 73, TOTAL, BRAND)

    s = content_slide(prs, "Next steps", bullets=[
        ("Newsrooms.",
         "C2PA at scale. DNS-anchor publication. Sign articles. "
         "Push browser vendors for verification UI."),
        ("Platforms.",
         "Display badges. Build trust calibration UI. Don't "
         "impose hierarchy."),
        ("Vendors.",
         "Camera and software vendors: complete C2PA integration. "
         "Continue investment."),
        ("Regulators.",
         "Mandate disclosure for synthetic content. Fund standards. "
         "Recognize cryptographic provenance legally."),
        ("Citizens.",
         "Look for badges. Calibrate trust thoughtfully. Demand "
         "verification UI from the platforms you use."),
    ])
    add_footer(s, 74, TOTAL, BRAND)

    s = closing_slide(prs,
        "Questions",
        subtitle="Thank you. The hard work begins now.",
        cta="Where does the three-layer architecture fail in your "
            "context?\n\nWhich layer is your stakeholder bottleneck?\n\n"
            "What's the next concrete step for your org?",
        resources=[
            "github.com/quidnug/quidnug",
            "blogs/2026-04-24-deepfakes-trust-endgame.md",
            "C2PA spec: c2pa.org",
            "Adobe Content Credentials: contentcredentials.org",
            "Quidnug QDP-0023 (DNS-anchored attestation)",
            "Quidnug QDP-0024 (private communications)",
            "Sumsub Identity Fraud Report 2024",
            "WEF Global Risks Report 2024",
        ])
    add_footer(s, 75, TOTAL, BRAND)

    return prs


if __name__ == "__main__":
    prs = build()
    prs.save(str(OUTPUT))
    print(f"wrote {OUTPUT} ({len(prs.slides)} slides)")
