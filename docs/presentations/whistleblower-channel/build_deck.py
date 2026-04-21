"""Whistleblower Channel deck (~75 slides)."""
import pathlib, sys
HERE = pathlib.Path(__file__).parent
ASSETS = HERE / "assets"
OUTPUT = HERE / "whistleblower-channel.pptx"
sys.path.insert(0, str(HERE.parent))
from _deck_helpers import (  # noqa: E402
    make_presentation, title_slide, section_divider, content_slide,
    two_col_slide, stat_slide, quote_slide, table_slide, image_slide,
    code_slide, icon_grid_slide, closing_slide, add_notes, add_footer,
    TEAL, CORAL, EMERALD, AMBER, TEXT_MUTED,
)
from pptx.util import Inches  # noqa: E402

BRAND = "Quidnug  \u00B7  Whistleblower Channel"
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

    # ---- Opening -----------------------------------------------------
    s = title_slide(prs,
        "The Whistleblower Channel",
        "Selective disclosure plus cryptographic credibility plus "
        "trust-graph vouching is the architecture modern whistleblower "
        "platforms actually need.",
        eyebrow="QUIDNUG  \u00B7  JOURNALISM & CIVIL SOCIETY")
    add_footer(s, 1, TOTAL, BRAND)
    add_notes(s, [
        "Welcome. This deck covers how modern cryptography can give "
        "anonymous whistleblowers institutional credibility without "
        "ever revealing their identity.",
        "Our thesis: credibility-without-identity is a solved "
        "cryptographic problem. Blind signatures have been in "
        "production since 1988 and at billion-scale since 2017.",
        "What current whistleblower platforms are missing is not "
        "encryption or anonymity. Those are solved. The missing "
        "layer is attestation: proving 'I am who I claim to be' "
        "without saying who you are.",
        "This talk walks through the primitives, the architecture, "
        "the legal fit, and a worked example end-to-end."])

    s = stat_slide(prs, "3 \u2192 20",
        "US Espionage Act prosecutions of leakers went from 3 total "
        "(1917-2008) to roughly 20 in the 15 years since.",
        context="Source: ACLU National Security reporting. Leaking "
                "to a journalist is now radically riskier than in the "
                "Pentagon Papers era. The risk calculus for sources "
                "has materially worsened.",
        title="The chilling effect", stat_color=CORAL)
    add_footer(s, 2, TOTAL, BRAND)
    add_notes(s, [
        "From 1917 (passage of the Espionage Act) through 2008, three "
        "people were prosecuted under it for leaking to the press.",
        "Obama brought eight. Trump I brought eight more. Biden has "
        "brought four so far.",
        "The per-year rate is roughly 20x what it was historically. "
        "A source considering coming forward has to weigh materially "
        "greater legal exposure than any source in the pre-2009 era.",
        "This is the pressure that makes anonymity-preserving "
        "credibility infrastructure urgent rather than theoretical."])

    s = stat_slide(prs, "$0",
        "the cost for an audience to verify a credentialed "
        "whistleblower's institutional claims under this architecture, "
        "once it exists.",
        context="Under current practice the cost is borne by the "
                "journalist (weeks of private vetting) and the reader "
                "(trust the outlet). Selective disclosure flips that: "
                "any reader with the verification tool checks the "
                "chain in milliseconds.",
        title="What becomes cheap", stat_color=EMERALD)
    add_footer(s, 3, TOTAL, BRAND)
    add_notes(s, [
        "Credibility verification today is expensive because it "
        "happens in private inside the journalist's workflow.",
        "With cryptographic attestation, verification becomes "
        "mechanical. A reader's reader app checks the signature "
        "chain, the attester's DNS anchor, the pseudonym's track "
        "record. No journalist assertion required.",
        "The economics are the point. When verification is free, "
        "audiences can hold credibility claims to a higher standard."])

    s = content_slide(prs, "Agenda", bullets=[
        ("1. Why credibility is the hard part.",
         "The structural journalism problem and the chilling effect."),
        ("2. Current whistleblower infrastructure.",
         "SecureDrop, GlobaLeaks, Signal, Tor, Tails: what they "
         "cover and what they don't."),
        ("3. Selective disclosure primitive.",
         "Blind signatures, Privacy Pass, BBS+ credentials."),
        ("4. Stable pseudonyms with reputation.",
         "Accumulating credibility without identity."),
        ("5. Trust-graph vouching.",
         "Peer attestations and DNS-anchored institutional "
         "attestations."),
        ("6. End-to-end architecture.",
         "How the pieces compose source to reader."),
        ("7. Legal frameworks.",
         "US, EU, UK, non-democratic jurisdictions."),
        ("8. Worked example + honest limits.",
         "Financial fraud disclosure walkthrough and ethical tensions."),
    ])
    add_footer(s, 4, TOTAL, BRAND)
    add_notes(s, [
        "Eight sections. The first four establish why and what the "
        "primitives are. The middle two compose them into a concrete "
        "architecture. The last two address legal fit and honest "
        "limitations.",
        "If you have to leave early, the most important sections are "
        "3 (selective disclosure) and 6 (architecture). Sections 1-2 "
        "are motivation; sections 7-9 are implementation and caveats."])

    s = content_slide(prs, "Four claims this talk defends", bullets=[
        ("1. Credibility-without-identity is solved cryptography.",
         "Blind signatures 1988. Privacy Pass 2017. BBS+ 2020s. "
         "Billions of tokens per month already deployed."),
        ("2. Current platforms solve submission and encryption, "
         "not credibility.",
         "SecureDrop, Signal, Tor, Tails: every layer except the "
         "one that matters for reader trust."),
        ("3. Institutional attestation via DNS-anchored identity "
         "gives sources a credibility floor.",
         "Without ever revealing who they are."),
        ("4. The pattern generalizes beyond journalism.",
         "SEC tips, OSHA complaints, EU Whistleblower Directive, "
         "regulated-industry tip lines."),
    ])
    add_footer(s, 5, TOTAL, BRAND)
    add_notes(s, [
        "Four claims, each defended with specific evidence in the "
        "talk.",
        "Claim 1 is important to land early because engineers in the "
        "audience may suspect this is speculative cryptography. It "
        "isn't. Blind signatures run at billion-scale monthly in "
        "production today.",
        "Claim 4 matters because whistleblower channels are not just "
        "for marquee journalism. Every regulated industry runs a tip "
        "line with the same fundamental credibility problem."])

    # ---- Section 1: Why credibility ---------------------------------
    s = section_divider(prs, 1, "Why Credibility Is the Hard Part",
        "Journalism-ethics scholarship plus the chilling effect.")
    add_footer(s, 6, TOTAL, BRAND)

    s = content_slide(prs,
        "Successful disclosures succeed on credibility",
        bullets=[
        ("Snowden (2013).",
         "Worked because Greenwald and Poitras spent months "
         "verifying Snowden's credentials before publishing."),
        ("Haugen (2021).",
         "Worked because she provided thousands of signed documents "
         "and testified under oath to Congress."),
        ("Common thread.",
         "Both had massive pre-publication credibility work. The "
         "content wasn't enough on its own."),
        ("The pattern generalizes.",
         "Disclosures succeed when credibility is established. "
         "They fail when audiences cannot distinguish a real "
         "insider from a fabrication."),
    ])
    add_footer(s, 7, TOTAL, BRAND)
    add_notes(s, [
        "Both Snowden and Haugen are often invoked as 'the system "
        "works.' What that ignores is how expensive the credibility "
        "establishment was.",
        "Snowden's credibility took months of careful verification "
        "before The Guardian would publish. That's an enormous "
        "private cost borne by specific journalists.",
        "Haugen's credibility came partly from her willingness to "
        "testify publicly. That's a form of self-identification "
        "that removes the anonymity protection entirely.",
        "Neither is a repeatable pattern for sources without those "
        "resources or that willingness."])

    s = _im(prs, "The two case studies side-by-side",
        "chart_cases.png",
        caption="Both required credentials verification plus months "
                "of vetting. The anonymous-but-credible path didn't "
                "exist.",
        image_h=4.5)
    add_footer(s, 8, TOTAL, BRAND)
    add_notes(s, [
        "This chart shows why these two are textbook cases but not "
        "templates.",
        "Snowden: credentials verified, extensive vetting, eventually "
        "identified, cost was permanent exile.",
        "Haugen: signed documents, sworn testimony, protected under "
        "SEC whistleblower rules, cost was career impact.",
        "Both had exceptional resources. Most potential whistleblowers "
        "do not. The substrate we're describing makes credible "
        "disclosure available to sources who currently cannot reach "
        "credibility because they cannot expose themselves."])

    s = content_slide(prs,
        "The structural role of sources in journalism",
        bullets=[
        ("Evidence that the journalist has privileged information.",
         "The source's reality is part of the story's evidence."),
        ("Credibility assessment happens privately.",
         "Readers don't see the journalist's verification process."),
        ("Multiple-source corroboration is the gold standard.",
         "But each source is a private verification exercise."),
        ("Institutional affiliation carries weight.",
         "A 'senior engineer at Company X' outweighs an "
         "'anonymous source' in reader credibility assessments."),
        ("The chain is opaque.",
         "Readers rely on the journalist's reputation, not "
         "evidence about the source. Singer 2007; Robinson 2011."),
    ])
    add_footer(s, 9, TOTAL, BRAND)
    add_notes(s, [
        "This is journalism-ethics 101. Sources aren't just content "
        "providers; they're structural evidence for the story's "
        "validity.",
        "Singer and Robinson have both done extensive newsroom "
        "ethnography showing that credibility is established "
        "privately and then converted to assertion in the published "
        "piece.",
        "That conversion is the weak point. The reader sees the "
        "assertion and has no independent way to verify."])

    s = content_slide(prs,
        "The verification problem: what readers can't see",
        bullets=[
        ("What journalists do.",
         "Check employment records, cross-reference with public "
         "records, cultivate multiple sources, assess internal "
         "consistency."),
        ("What readers see.",
         "'A source familiar with the matter.' 'A senior employee.' "
         "'A person who requested anonymity.'"),
        ("The trust transfer.",
         "Reader trusts journalist \u2192 journalist trusts source \u2192 "
         "reader does not independently verify source."),
        ("Failure modes.",
         "Works for established outlets; fails for new outlets, "
         "adversarial contexts, cross-jurisdictional reporting, and "
         "'fake news' accusations. The liar's dividend: subjects of "
         "reporting can cast doubt on anonymous sources with no "
         "counter-evidence available."),
    ])
    add_footer(s, 10, TOTAL, BRAND)
    add_notes(s, [
        "The liar's dividend is important. When a story is attacked "
        "as 'fake news,' the outlet has no way to point at the "
        "credibility work they did without compromising the source.",
        "Cryptographic attestation gives outlets a way to point at "
        "verifiable attestation chains without ever revealing the "
        "source."])

    s = _im(prs, "Leak prosecutions under the Espionage Act",
        "chart_prosecutions.png",
        caption="Three pre-Obama, then roughly twenty since. The "
                "risk calculus for sources has materially worsened.",
        image_h=4.4)
    add_footer(s, 11, TOTAL, BRAND)
    add_notes(s, [
        "From 1917 through 2008 there were three Espionage Act "
        "prosecutions of leakers to journalists. Daniel Ellsberg "
        "(Pentagon Papers) was one. Samuel Morison (Navy photos) "
        "was another.",
        "Obama brought eight. Trump I brought eight more. Biden four "
        "so far.",
        "Roughly twenty in fifteen years vs three in ninety-one. "
        "That's a 20x increase in annualized rate.",
        "Every potential source is doing this math. The substrate "
        "doesn't protect against statutory liability, but it does "
        "mean that sources who can't come forward publicly can still "
        "contribute credibly."])

    s = content_slide(prs,
        "The chilling effect: what's at stake for sources",
        bullets=[
        ("Legal retaliation.",
         "Firing, civil litigation, criminal prosecution under "
         "statutes like the US Espionage Act."),
        ("Professional destruction.",
         "Industry blacklisting, loss of professional licenses, "
         "revocation of security clearances."),
        ("Social and family consequences.",
         "Documented in case studies from Snowden, Manning, "
         "Thomas Drake, and others."),
        ("Physical safety.",
         "Documented in authoritarian contexts. Journalists "
         "covering the Panama Papers were murdered."),
        ("The structural pressure.",
         "Pressure on journalists to protect anonymity is extreme. "
         "Anonymity erodes credibility. The tension is structural. "
         "Donaldson 2020; Radsch 2019."),
    ])
    add_footer(s, 12, TOTAL, BRAND)

    s = quote_slide(prs,
        "A source who is cryptographically verifiable as 'current "
        "senior engineer at Company X, with institutional attestation, "
        "not previously discredited' has substantially more credibility "
        "than 'an unnamed source' while maintaining identity protection. "
        "The source's specific identity remains private; their category "
        "(with specific attested claims) is public.",
        attrib="The core claim of this deck")
    add_footer(s, 13, TOTAL, BRAND)

    # ---- Section 2: Current infrastructure --------------------------
    s = section_divider(prs, 2, "Current Whistleblower Infrastructure",
        "Mapping what exists and what's missing.")
    add_footer(s, 14, TOTAL, BRAND)

    s = _im(prs, "Infrastructure coverage matrix",
        "chart_coverage.png",
        caption="Tor, Tails, SecureDrop, Signal cover four layers "
                "between them. Credibility attestation is uncovered.",
        image_h=4.8)
    add_footer(s, 15, TOTAL, BRAND)
    add_notes(s, [
        "This is the landscape today. Five layers matter: network "
        "anonymity, operational security, submission channel, "
        "encrypted comms, source credibility.",
        "The first four have mature tools. SecureDrop is the "
        "operational standard for AP, Washington Post, Guardian, "
        "NYT, ProPublica, The Intercept. Signal is used universally "
        "for source contact. Tor and Tails handle anonymity and "
        "endpoint security.",
        "Credibility is the one layer with no tool.",
        "The Quidnug substrate targets this specific gap. It does "
        "not replace SecureDrop or Signal; it composes with them."])

    s = content_slide(prs,
        "SecureDrop  \u2022  the submission channel",
        bullets=[
        ("Origin.",
         "Developed initially by Aaron Swartz and Kevin Poulsen, "
         "now maintained by Freedom of the Press Foundation."),
        ("Architecture.",
         "Source connects over Tor to a news organization's "
         "SecureDrop instance. Submission uses a code-name. "
         "Journalist retrieves via air-gapped system."),
        ("What SecureDrop does well.",
         "Source identity protection, document submission, "
         "journalist-side operational security."),
        ("What SecureDrop does not do.",
         "Cryptographic attestation of source's institutional claims. "
         "Persistent pseudonymous identity. Cross-publication "
         "credibility portability."),
        ("Takeaway.",
         "SecureDrop is the floor we build on, not the ceiling."),
    ])
    add_footer(s, 16, TOTAL, BRAND)
    add_notes(s, [
        "SecureDrop is genuinely excellent at what it does. The "
        "threat model is thoughtful, the track record is strong, "
        "and major news organizations have standardized on it.",
        "Our substrate adds a credibility attestation layer on top. "
        "A source uses SecureDrop to submit the package including "
        "attested credentials.",
        "The journalist's workflow stays largely the same; they "
        "just have cryptographic verification for claims they "
        "previously had to take on trust."])

    s = content_slide(prs,
        "GlobaLeaks  \u2022  SecureDrop for NGOs and tip lines",
        bullets=[
        ("Purpose.",
         "The SecureDrop pattern for NGOs and government "
         "anti-corruption tip lines."),
        ("Deployments.",
         "Transparency International chapters, anti-corruption "
         "agencies in multiple countries, investigative journalism "
         "initiatives worldwide."),
        ("Strengths.",
         "Same as SecureDrop. Good submission channel, thoughtful "
         "threat model, reasonable operational security guidance."),
        ("Gaps.",
         "Same as SecureDrop. No credibility attestation machinery."),
        ("Note.",
         "Both projects are mature and well-maintained. The "
         "credibility gap is not a criticism; it's a layer they "
         "don't claim to address."),
    ])
    add_footer(s, 17, TOTAL, BRAND)

    s = content_slide(prs,
        "Signal  \u2022  the communication layer",
        bullets=[
        ("Role.",
         "End-to-end encrypted messaging. The industry standard "
         "for source-journalist contact."),
        ("Protocol.",
         "Double Ratchet + X3DH. Well-studied, implemented in "
         "reference quality. Non-profit Signal Foundation."),
        ("Strengths.",
         "End-to-end encryption, forward secrecy, open source, "
         "extensively audited."),
        ("Limitations.",
         "No source credibility verification. Persistent pseudonym "
         "is only the phone number. Not anonymous against phone "
         "number association."),
        ("Substrate alignment.",
         "QDP-0024 MLS-based group encryption is a direct "
         "successor pattern with pseudonymous identity built in."),
    ])
    add_footer(s, 18, TOTAL, BRAND)
    add_notes(s, [
        "Signal is indispensable infrastructure. It's not going "
        "away.",
        "The limitation for whistleblowing is the phone number "
        "association. Signal requires a phone number to register. "
        "In adversarial environments, a phone number is a partial "
        "identification.",
        "QDP-0024 addresses this by using cryptographic identity "
        "instead of phone numbers. For our substrate, we use "
        "QDP-0024 MLS groups as an alternative to Signal when "
        "the phone number is itself a risk."])

    s = content_slide(prs,
        "Tor + Tails  \u2022  operational security",
        bullets=[
        ("Tor.",
         "Network-layer anonymity. Onion routing through volunteer "
         "relays. Protects source IP address and traffic analysis."),
        ("Tails.",
         "Linux live-USB OS designed for hostile-environment "
         "journalism and whistleblowing. Leaves no trace on "
         "host hardware."),
        ("Threat coverage.",
         "Network observation, endpoint compromise, forensic "
         "recovery of source activity."),
        ("What they provide.",
         "Identity protection at network and endpoint layers. "
         "Neither provides credibility machinery."),
        ("Integration.",
         "The substrate runs alongside these tools. Sources "
         "continue to use Tor and Tails; attestation workflows "
         "fit into that operational model."),
    ])
    add_footer(s, 19, TOTAL, BRAND)

    s = content_slide(prs,
        "The gap: every tool is one layer",
        bullets=[
        ("Network anonymity.",
         "Covered by Tor."),
        ("Operational security.",
         "Covered by Tails."),
        ("Submission channel.",
         "Covered by SecureDrop or GlobaLeaks."),
        ("Encrypted communication.",
         "Covered by Signal or QDP-0024."),
        ("Source credibility to the audience.",
         "Uncovered. This is the gap the Quidnug substrate fills."),
        ("Why it matters.",
         "A whistleblower using all of the above has done "
         "everything right operationally. The journalist still "
         "has to verify credibility privately; the reader still "
         "has to trust the journalist."),
    ])
    add_footer(s, 20, TOTAL, BRAND)

    # ---- Section 3: Selective disclosure ----------------------------
    s = section_divider(prs, 3, "The Selective Disclosure Primitive",
        "Prove specific attributes without revealing identity.")
    add_footer(s, 21, TOTAL, BRAND)

    s = content_slide(prs, "Origins of selective disclosure", bullets=[
        ("Chaum 1985.",
         "'Security without identification: transaction systems "
         "to make big brother obsolete.' Foundational paper."),
        ("Chaum 1988.",
         "'Blind signatures for untraceable payments.' Specific "
         "cryptographic primitive for unlinkable token issuance."),
        ("Brands 1993.",
         "Brands credentials: signatures on attribute sets with "
         "selective disclosure."),
        ("Camenisch-Lysyanskaya 2001, 2003.",
         "CL signatures: anonymous credentials with zero-knowledge "
         "attribute proofs."),
        ("Boneh-Boyen-Shacham and extensions.",
         "BBS+ signatures: compact signatures that support "
         "selective disclosure efficiently. Now the basis for "
         "W3C Verifiable Credentials."),
        ("Not a new idea.",
         "The research line is forty years deep. The deployment "
         "is the last decade."),
    ])
    add_footer(s, 22, TOTAL, BRAND)
    add_notes(s, [
        "This is forty years of cryptography research, not a "
        "speculative primitive.",
        "Chaum's 1988 paper is the starting point. His motivation "
        "was digital cash where the issuer cannot link issued coins "
        "to spending events. The same machinery applies to "
        "whistleblower credentials.",
        "BBS+ is what modern deployments use. It supports compact "
        "proofs, efficient verification, and selective disclosure "
        "over large attribute sets. It's the suite the W3C Verifiable "
        "Credentials standard endorses."])

    s = _im(prs, "Blind signatures deployed at massive scale",
        "chart_scale.png",
        caption="Billions of blind-signature tokens per month across "
                "Cloudflare Privacy Pass, Apple Private Relay, and "
                "others. The cryptography is proven.",
        image_h=4.4)
    add_footer(s, 23, TOTAL, BRAND)
    add_notes(s, [
        "If you doubt the primitive works at scale, look at the "
        "deployment numbers.",
        "Cloudflare Privacy Pass alone issues billions of tokens "
        "monthly. Apple Private Relay does roughly a billion "
        "monthly. Private Click Measurement, Firefox, and the W3C "
        "Verifiable Credentials ecosystem add more.",
        "The cryptography is not novel. The application to "
        "whistleblower credentials is."])

    s = content_slide(prs, "Privacy Pass as the concrete example",
        bullets=[
        ("The user experience.",
         "User solves a CAPTCHA once. Gets a batch of blinded "
         "tokens. Later, redeems tokens on other sites to prove "
         "'verified human' without being re-CAPTCHAed."),
        ("The cryptographic property.",
         "The CAPTCHA provider cannot link redeemed tokens to "
         "the specific CAPTCHA-solving session."),
        ("Standardization.",
         "IETF RFC 9576 (2024). Published standard with multiple "
         "interoperable implementations."),
        ("Why this matters for whistleblowing.",
         "Same primitive. Same cryptographic property. Apply it "
         "to 'verified insider at Company X' instead of 'verified "
         "human.'"),
    ])
    add_footer(s, 24, TOTAL, BRAND)

    s = _im(prs, "Blind signature issuance flow",
        "chart_blind_flow.png",
        caption="Attester signs a blinded message. Source unblinds "
                "locally. Verifier checks the unblinded signature.",
        image_h=4.6)
    add_footer(s, 25, TOTAL, BRAND)
    add_notes(s, [
        "The flow in five steps:",
        "One: source presents identity proof (HR records, license, "
        "cert) to the Attestation Authority and requests a blinded "
        "credential.",
        "Two: Attester signs the blinded message. Critically, the "
        "Attester does not see the message content; it sees only a "
        "cryptographic commitment.",
        "Three: source unblinds locally. Now holds a credential "
        "that the Attester signed but cannot recognize on later "
        "presentation.",
        "Four: source presents a zero-knowledge proof of the "
        "credential with selected attributes disclosed.",
        "Five: verifier checks that the proof chains to the "
        "Attester's public key (DNS-anchored). The verifier "
        "learns the attributes, not the source's identity."])

    s = code_slide(prs, "Concrete credential example",
        code_lines=[
            "Attested attributes (disclosed to publisher):",
            "  Industry:         Financial services",
            "  Employer cat:     Major US bank, top 5 by assets",
            "  Role:             Senior engineer, risk systems",
            "  Tenure range:     5+ years",
            "  Active employ:    Yes",
            "  Certifications:   CFA (held), CISSP (held)",
            "  Regulatory hist:  No sanctions or enforcement actions",
            "",
            "NOT disclosed (held back by source):",
            "  Specific name of bank",
            "  Specific name of employee",
            "  Specific team or manager",
            "  Specific geographic location",
            "",
            "Signature:  IBF_ATTESTER signed the above via blind",
            "            signature. DNS anchor: ibanking.example.",
        ])
    add_footer(s, 26, TOTAL, BRAND)
    add_notes(s, [
        "This is what a credential looks like in practice.",
        "The attested attributes are enough for a reader to "
        "calibrate their credibility assessment. Top-five US bank, "
        "senior engineer in risk systems, 5+ years, CFA and CISSP. "
        "That's a specific, positioned insider.",
        "The not-disclosed attributes preserve anonymity. A reader "
        "cannot identify which specific bank, which specific team, "
        "which specific employee.",
        "The Attester (Institute of Banking and Finance in this "
        "example) is a real professional body with a DNS-anchored "
        "identity. Readers trust the Attester to the extent their "
        "trust graph includes it."])

    s = content_slide(prs,
        "Why this is stronger than 'trusted journalist says'",
        bullets=[
        ("Current practice: trust transfer.",
         "Reader trusts journalist. Journalist trusts source. "
         "Reader does not independently verify."),
        ("Under selective disclosure: verifiable attestation.",
         "Reader trusts Attester. Attester verified attributes. "
         "Reader independently verifies attestation chain."),
        ("Journalist role shifts.",
         "From 'vouch for source credibility' to 'handle content "
         "and editorial judgment.' Both roles remain; attestation "
         "handles the credibility part cryptographically."),
        ("Cross-publication portability.",
         "The same attested source can credibly publish with any "
         "outlet. Credibility is not tied to a single outlet's "
         "reputation."),
    ])
    add_footer(s, 27, TOTAL, BRAND)

    # ---- Section 4: Stable pseudonyms -------------------------------
    s = section_divider(prs, 4, "Stable Pseudonyms with Reputation",
        "Accumulating track record without identity.")
    add_footer(s, 28, TOTAL, BRAND)

    s = content_slide(prs,
        "Pseudonym = keypair, consistent across disclosures",
        bullets=[
        ("The primitive.",
         "Source generates a Quidnug keypair. Uses it consistently "
         "across disclosures. The public key is the stable "
         "pseudonym."),
        ("Attribute layer is separate.",
         "Attested attributes from institutional attestation can "
         "vary across disclosures. The keypair stays constant."),
        ("Track record accumulates.",
         "Each disclosure produces outcomes (regulatory action, "
         "confirmatory filings, court records). The pseudonym's "
         "track record of accurate disclosures accumulates."),
        ("Not tied to publication.",
         "Same pseudonym can disclose to Guardian, NYT, "
         "Der Spiegel, ProPublica. Reputation is portable."),
    ])
    add_footer(s, 29, TOTAL, BRAND)

    s = _im(prs, "A pseudonym's reputation accumulating",
        "chart_pseudonym.png",
        caption="Composite credibility grows with each independently "
                "verified disclosure. Identity stays private throughout.",
        image_h=4.4)
    add_footer(s, 30, TOTAL, BRAND)
    add_notes(s, [
        "Three disclosures over two years, each independently "
        "verified by later evidence. The pseudonym's credibility "
        "moves from 0.45 (new, uncertain) to 0.91 (three-for-three "
        "verified).",
        "A fourth disclosure from this pseudonym carries the weight "
        "of three prior verified disclosures. The source is credible "
        "even though they remain anonymous.",
        "This is Donath's analysis of virtual-community identity "
        "applied to journalism: stable pseudonyms enable reputation "
        "accumulation; identity revelation is orthogonal to "
        "credibility."])

    s = content_slide(prs, "Verification without identification",
        bullets=[
        ("Accuracy signal: independent evidence.",
         "SEC filings, court records, company restatements, "
         "later institutional admissions."),
        ("The pseudonym's accuracy is assessed against these.",
         "Not against the journalist's assertion. Not against "
         "the source's own claim."),
        ("Donath 1999 on virtual community identity.",
         "Stable pseudonyms enable reputation accumulation. "
         "Identity revelation is orthogonal to credibility."),
        ("Axelrod 1984 on repeated games.",
         "Repeated-game dynamics produce cooperation among "
         "strangers when reputation is persistent."),
        ("Pseudonyms create the repeated game.",
         "Anonymous sources play a one-shot game with no "
         "accountability. Pseudonymous sources are in a "
         "repeated game with reputation consequences."),
    ])
    add_footer(s, 31, TOTAL, BRAND)

    s = content_slide(prs, "What prevents impersonation",
        bullets=[
        ("The keypair is the authority.",
         "Only the holder of the private key can sign under "
         "that pseudonym."),
        ("Compromise is discoverable.",
         "Inconsistent disclosure patterns, violation of prior "
         "commitments, or contradictory claims signal compromise."),
        ("Recovery model.",
         "For adversarial contexts, simpler model: if key is "
         "compromised, retire that pseudonym and start new. "
         "Accumulated credibility is lost; attacker cannot "
         "continue to impersonate."),
        ("QDP-0002 guardian recovery.",
         "Exists in protocol but for high-risk whistleblower "
         "contexts the retirement model is simpler and more "
         "robust."),
        ("Key storage.",
         "Air-gapped, offline, with backup on similarly protected "
         "hardware."),
    ])
    add_footer(s, 32, TOTAL, BRAND)

    s = content_slide(prs, "Anonymous-but-accountable",
        bullets=[
        ("Pure anonymity has no accountability.",
         "A one-shot anonymous source can lie once with full "
         "impact and face zero cost."),
        ("Pseudonyms create real cost.",
         "A pseudonym caught in a falsehood loses credibility. "
         "If they want to make future disclosures, they bear a "
         "consequence."),
        ("Falsehood is detectable.",
         "Independent verification (or non-verification) of "
         "claims is public. The signal is hard to suppress."),
        ("The consequence is real.",
         "Even without legal identity. Reputation pressure "
         "constrains behavior in the repeated game."),
        ("Partial restoration of accountability.",
         "Anonymous sources shift all consequence to the publication. "
         "Pseudonymous sources share it."),
    ])
    add_footer(s, 33, TOTAL, BRAND)

    # ---- Section 5: Trust-graph vouching ----------------------------
    s = section_divider(prs, 5, "Trust-Graph Vouching",
        "Peer and institutional attestation, composed.")
    add_footer(s, 34, TOTAL, BRAND)

    s = content_slide(prs, "Two types of attestation, composed",
        bullets=[
        ("Peer attestation.",
         "Other employees or professionals vouching for the source. "
         "'I, a colleague at the same institution, can confirm "
         "this person has the access they claim.'"),
        ("Institutional attestation.",
         "Organizations vouching for the source's category. "
         "'This person is a credentialed member of our professional "
         "body.'"),
        ("Both types compose.",
         "A source with 3 peer attestations + institutional "
         "attestation from a professional body has stronger "
         "evidence than either alone."),
        ("Each is independently pseudonymizable.",
         "Peers don't have to reveal themselves either. Pseudonymous "
         "peer attestations work using the same machinery."),
    ])
    add_footer(s, 35, TOTAL, BRAND)

    s = _im(prs, "Trust graph around the source",
        "chart_graph.png",
        caption="Institutional attester above, pseudonymous peers "
                "surrounding, journalist at the edge of the graph.",
        image_h=4.8)
    add_footer(s, 36, TOTAL, BRAND)
    add_notes(s, [
        "This is what the trust graph looks like for a well-"
        "constructed disclosure.",
        "Center: the source pseudonym. Above: an institutional "
        "attester (DNS-anchored). Sides: pseudonymous peers "
        "(each with their own track record). Far edge: the "
        "journalist who verifies the whole structure.",
        "The journalist's role is important but subordinate. They "
        "verify the chain, they don't manufacture credibility. "
        "Credibility is verifiable by anyone with the tooling."])

    s = content_slide(prs,
        "Peer attestation without identity exposure",
        bullets=[
        ("Peers have pseudonyms too.",
         "Any peer attestation can come from a pseudonym, not a "
         "named individual."),
        ("Pseudonymous peer attestation.",
         "'I am a current employee at Company X, pseudonymously "
         "known as pseudo-peer-7f, and I attest that source "
         "pseudo-banker-a2c7 has the access they claim.'"),
        ("Journalist sees structured evidence.",
         "Multiple pseudonymous attestations from people who claim "
         "specific institutional roles, each with their own track "
         "record."),
        ("Composition of weak signals.",
         "Three pseudonymous attestations from new pseudonyms are "
         "weaker than one from an established pseudonym. But three "
         "of them together is still information."),
    ])
    add_footer(s, 37, TOTAL, BRAND)

    s = content_slide(prs,
        "DNS-anchored institutional attestation (QDP-0023)",
        bullets=[
        ("Identity rooted in DNS.",
         "Institutions anchor Quidnug identity to their DNS "
         "domain. SPE.example publishes their public key via "
         "DNSSEC-signed TXT record."),
        ("Pre-existing reputation transfers.",
         "SPE's credibility in the real world is the credibility "
         "of their DNS-anchored identity."),
        ("Members request credentials.",
         "Via blind signature flow. SPE signs attestations of "
         "membership, tenure, certification status, sanctions "
         "history."),
        ("Readers verify.",
         "The attestation on the disclosure chains to SPE's DNS "
         "identity. Readers weight by their own trust graph's "
         "opinion of SPE."),
    ])
    add_footer(s, 38, TOTAL, BRAND)

    s = _im(prs, "The layered credibility stack",
        "chart_stack.png",
        caption="Each layer adds independent evidence. The reader "
                "evaluates the stack as a whole.",
        image_h=4.9)
    add_footer(s, 39, TOTAL, BRAND)
    add_notes(s, [
        "Five layers. Each is independently verifiable. Each adds "
        "to composite credibility.",
        "Layer 1: source pseudonym signature. Who's making the "
        "claim? A specific keypair with a history.",
        "Layer 2: institutional attestation. What category of "
        "person is the source? Attested by a DNS-anchored institution.",
        "Layer 3: peer attestations. Who else confirms the source "
        "is positioned to know? Pseudonymous colleagues with their "
        "own track records.",
        "Layer 4: document signatures. Are the documents authentic? "
        "Corporate PKI signatures prove document provenance.",
        "Layer 5: journalist editorial vetting. Does the whole "
        "package cohere with public reality? Traditional editorial "
        "judgment.",
        "Five independent sources of evidence, not one."])

    s = content_slide(prs, "Absence of attestation as a signal",
        bullets=[
        ("The stack is informative whether full or empty.",
         "A source with no institutional attestation and no "
         "peer vouches is signaling something."),
        ("Three possibilities.",
         "They cannot obtain attestation (unusual for genuine "
         "insiders). They are not positioned as they claim. They "
         "are a first-time source with no accumulated credibility."),
        ("Readers factor the absence.",
         "Composite credibility is drawn down when attestation is "
         "missing. This is principled, not arbitrary."),
        ("Not a disqualification.",
         "First-time anonymous sources can still provide "
         "information. But they start from a lower floor and "
         "build credibility through accurate disclosures."),
    ])
    add_footer(s, 40, TOTAL, BRAND)

    s = content_slide(prs, "Defense against fake attestations",
        bullets=[
        ("DNS-anchored identity requires real domain control.",
         "Creating 'Institute of Banking and Finance' as a fake "
         "requires registering a domain and building a presence "
         "readers would trust."),
        ("Trust graph weights institutions by reader's trust.",
         "A new unfamiliar 'institution' has low weight even if "
         "technically authentic."),
        ("Cross-institution corroboration.",
         "A source attested by one real institution is weaker "
         "than one attested by multiple from different domains."),
        ("Same defenses as review-spam prevention.",
         "The structure carries over from the relational ratings "
         "context. Trust graphs are self-defending when they're "
         "relational."),
    ])
    add_footer(s, 41, TOTAL, BRAND)

    # ---- Section 6: End-to-end architecture -------------------------
    s = section_divider(prs, 6, "End-to-End Architecture",
        "Source infrastructure to reader audit, assembled.")
    add_footer(s, 42, TOTAL, BRAND)

    s = _im(prs, "The complete architecture",
        "chart_arch.png",
        caption="Five stages: source infrastructure \u2192 communication "
                "\u2192 credibility chain \u2192 journalist workflow \u2192 reader "
                "experience.",
        image_h=4.8)
    add_footer(s, 43, TOTAL, BRAND)
    add_notes(s, [
        "End-to-end. This is what the full architecture looks like.",
        "Source infrastructure: Tails on a dedicated laptop, Tor "
        "routing, airgapped keypair, credential store.",
        "Communication: SecureDrop or QDP-0024 MLS. Encrypted "
        "contact channel.",
        "Credibility chain: pseudonym history, blind credential, "
        "peer attestations, document signatures.",
        "Journalist workflow: verify chain, apply editorial "
        "judgment, publish with metadata.",
        "Reader experience: see the trust calculation, drill into "
        "the audit trail.",
        "Every layer has known technology. The substrate composes "
        "known pieces with the credibility-attestation addition."])

    s = content_slide(prs, "Source infrastructure detail", bullets=[
        ("Tails USB.",
         "Dedicated hardware. Boots from USB. Leaves no trace. "
         "Freedom of the Press Foundation guidance applies."),
        ("Tor Browser.",
         "Network anonymity. Hidden service for destination."),
        ("Air-gapped keypair.",
         "Generated on offline hardware. Never touches internet-"
         "connected machines. Backups on similarly protected hardware."),
        ("Credential store.",
         "Obtained from Attesters via blind-signature flow. "
         "Bound to source's Quidnug quid."),
        ("Document preparation.",
         "Metadata scrubbed. Provenance chain preserved where "
         "possible (corporate PKI signatures stay intact)."),
    ])
    add_footer(s, 44, TOTAL, BRAND)

    s = content_slide(prs, "Communication channel options",
        bullets=[
        ("SecureDrop.",
         "Publisher-operated. Source connects via Tor. Submits "
         "documents + credential package. Journalist retrieves "
         "via air-gapped workflow."),
        ("QDP-0024 MLS group.",
         "Pseudonymous group chat. Source's pseudonym is a member. "
         "Forward secrecy. No phone number requirement."),
        ("When to use which.",
         "SecureDrop for first-contact document dumps. QDP-0024 "
         "for ongoing pseudonymous conversation that spans stories."),
        ("Both compose.",
         "A source can use SecureDrop for submission and QDP-0024 "
         "for follow-up questions without re-exposing themselves."),
    ])
    add_footer(s, 45, TOTAL, BRAND)

    s = content_slide(prs, "Credibility chain detail",
        bullets=[
        ("Pseudonym signature on the disclosure.",
         "Root of the chain. Only the source can sign under this "
         "keypair. Readers verify signature validity."),
        ("Blind credential presentation.",
         "Zero-knowledge proof that source holds a credential "
         "signed by Attester. Attributes selectively disclosed."),
        ("Peer attestation signatures.",
         "Each peer signs a brief attestation under their own "
         "pseudonym. Readers verify the set of signatures and "
         "their pseudonyms' track records."),
        ("Document signatures.",
         "Where documents were corporate-signed (using the "
         "company's own PKI), signatures survive. Provenance "
         "is verifiable independent of the source."),
    ])
    add_footer(s, 46, TOTAL, BRAND)

    s = content_slide(prs, "Journalist workflow",
        bullets=[
        ("Receive the package.",
         "Documents + credential package + pseudonym + peer "
         "attestations. All via SecureDrop or QDP-0024."),
        ("Verify the credibility chain.",
         "Signature validations, DNS anchor checks, pseudonym "
         "track record lookups. Automated at the tool layer."),
        ("Corroborate independently.",
         "Public records, other sources, internal consistency "
         "of the source's account. Traditional editorial work."),
        ("Apply editorial judgment.",
         "Is the story in the public interest? Is the framing "
         "responsible? Is the level of detail appropriate?"),
        ("Publish with metadata.",
         "The credibility chain metadata is part of the publication. "
         "Readers can inspect it if they choose."),
    ])
    add_footer(s, 47, TOTAL, BRAND)

    s = code_slide(prs, "What the reader sees",
        code_lines=[
            "Story: Bank X systematically misreports loan-loss reserves",
            "Byline: Jordan K. at Financial Times",
            "",
            "Source verification:",
            "  Source pseudonym:    pseudo-banker-a2c7",
            "                       (established 2023-11, 4 prior verified)",
            "  Institutional:       Institute of Banking and Finance",
            "                       (DNS anchor: ibf.example, verified)",
            "    Attested:          Current senior employee, 8+ years,",
            "                       certifications valid, no sanctions",
            "  Peer attestations:   3 colleagues (pseudonymous)",
            "                       Each with own track record",
            "  Document signatures: Bank X corporate PKI on 12 of 17 docs",
            "",
            "Reader trust calculation:",
            "  Trust in publication (FT):       0.85",
            "  Trust in journalist (Jordan K.): 0.80",
            "  Trust in IBF:                    0.70",
            "  Trust in pseudonym's track:      0.75",
            "  Composite credibility:           ~0.78",
            "",
            "[Click for detailed verification \u2192]",
        ])
    add_footer(s, 48, TOTAL, BRAND)
    add_notes(s, [
        "This is the reader-facing artifact. Not just 'A source "
        "familiar with the matter.'",
        "Every piece is verifiable. The reader can click through "
        "to independent verification if they want.",
        "Trust weights are the reader's own (taken from their "
        "trust graph). The composite credibility is computed "
        "locally; no central authority imposes a score.",
        "When the story is attacked as fake news, the publication "
        "has something to point at beyond the journalist's reputation."])

    s = content_slide(prs, "The source's protection layers",
        bullets=[
        ("Network-layer anonymity.",
         "Tor hides the source's IP and traffic patterns."),
        ("Operational security.",
         "Tails prevents forensic recovery from the source's "
         "hardware. Air-gap isolates the key."),
        ("Pseudonym decoupling.",
         "The keypair is not tied to any legal identity. The "
         "Attester never sees what they're signing."),
        ("Selective disclosure.",
         "Credentials prove category without revealing identity. "
         "Verifier learns attributes, not who."),
        ("Encrypted comms.",
         "Source-journalist channel is encrypted end-to-end. "
         "Content is not recoverable by third parties."),
        ("The stack is additive, not subtractive.",
         "The substrate adds layers. It doesn't remove existing "
         "protections."),
    ])
    add_footer(s, 49, TOTAL, BRAND)

    s = content_slide(prs, "The credibility floor", bullets=[
        ("Without the substrate.",
         "An anonymous source has credibility floor near zero. "
         "The journalist's assurance is the only signal."),
        ("With the substrate.",
         "An anonymous source has a credibility floor set by "
         "their institutional attestation plus peer attestations, "
         "which are independently verifiable."),
        ("What the floor enables.",
         "Credible anonymous disclosures at scale. Sources who "
         "currently cannot reach credibility because they cannot "
         "expose themselves can now participate."),
        ("Who benefits.",
         "The people most in need of protection: those facing "
         "material legal or physical risk from exposure."),
    ])
    add_footer(s, 50, TOTAL, BRAND)

    # ---- Section 7: Legal frameworks --------------------------------
    s = section_divider(prs, 7, "Legal Framework Alignment",
        "How the substrate fits existing whistleblower law.")
    add_footer(s, 51, TOTAL, BRAND)

    s = _im(prs, "Legal frameworks across jurisdictions",
        "chart_jurisdictions.png",
        caption="Four dimensions, four jurisdictions. Substrate "
                "alignment is strong in every context because it "
                "lowers the identification requirement.",
        image_h=4.4)
    add_footer(s, 52, TOTAL, BRAND)

    s = content_slide(prs, "United States", bullets=[
        ("Dodd-Frank Whistleblower Program (2010).",
         "SEC and CFTC programs. Monetary awards (10-30% of "
         "recoveries over $1M). Strong anti-retaliation protections."),
        ("Substrate fit.",
         "Pseudonymous tips with cryptographic attestation can "
         "be filed. Award processed through the pseudonym; "
         "identity revealed only at payment time, under "
         "confidentiality."),
        ("Espionage Act (1917).",
         "Prosecutes unauthorized disclosure. Current interpretation "
         "broad. Substrate does not protect against statutory "
         "criminal liability."),
        ("Federal whistleblower statutes.",
         "SOX, FCA, EPA, OSHA. Substrate helps establish "
         "'reported in good faith' element with verifiable "
         "documentation of the disclosure flow."),
    ])
    add_footer(s, 53, TOTAL, BRAND)

    s = content_slide(prs, "European Union", bullets=[
        ("EU Whistleblower Directive (2019/1937).",
         "Requires member states to establish internal and "
         "external reporting channels with protections."),
        ("Anonymous reporting.",
         "Explicitly allowed in some contexts. Substrate aligns "
         "directly with this provision."),
        ("GDPR alignment.",
         "Data minimization via selective disclosure is a "
         "designed-in property. Blind signatures and "
         "pseudonymous credentials are explicitly identified "
         "as privacy-enhancing techniques in GDPR guidance."),
        ("Implementation status.",
         "Member states have varying levels of implementation. "
         "Internal reporting channels at mid-size companies are "
         "still maturing. Substrate is forward-compatible."),
    ])
    add_footer(s, 54, TOTAL, BRAND)

    s = content_slide(prs, "United Kingdom", bullets=[
        ("Public Interest Disclosure Act (1998).",
         "Protects workers making disclosures in the public "
         "interest. Pre-GDPR, pre-directive era."),
        ("Anonymous disclosures.",
         "Not directly protected under PIDA. Worker must be "
         "identified to claim protections."),
        ("What substrate contributes.",
         "A pseudonymous track record helps establish the "
         "public-interest nature of disclosures. Not a substitute "
         "for identification when PIDA protection is sought."),
        ("Policy direction.",
         "PIDA reform has been discussed. Substrate demonstrates "
         "that credibility-without-identification is feasible, "
         "potentially informing future reform."),
    ])
    add_footer(s, 55, TOTAL, BRAND)

    s = content_slide(prs, "Non-democratic jurisdictions", bullets=[
        ("Weak or absent statutes.",
         "Many jurisdictions lack meaningful whistleblower law."),
        ("Higher legal risk to sources.",
         "Substrate makes credible disclosure possible where "
         "current infrastructure cannot. Does not remove "
         "retaliation risk."),
        ("Cross-border publication.",
         "Sources in restrictive jurisdictions can publish to "
         "outlets in permissive ones. Substrate makes the "
         "credibility portable."),
        ("Operational security is paramount.",
         "Technical anonymity (Tor, Tails) plus substrate "
         "anonymity gives layered protection. Still not absolute; "
         "state-level adversaries remain a serious threat."),
    ])
    add_footer(s, 56, TOTAL, BRAND)

    s = content_slide(prs,
        "Statutory alignment: policy directions",
        bullets=[
        ("Anti-retaliation extension.",
         "Statutes could be strengthened to protect disclosures "
         "made via attested pseudonymous channels with the same "
         "protections as identified disclosures."),
        ("Regulatory tip acceptance.",
         "SEC, FTC, EPA, OSHA could formalize acceptance of "
         "attested pseudonymous tips. Some already accept "
         "anonymous tips; attestation raises credibility."),
        ("Court evidence.",
         "Pseudonymous cryptographic identity could be treated "
         "as equivalent to sealed testimony in appropriate cases."),
        ("Not required.",
         "The technology works without statutory reform. Policy "
         "alignment amplifies impact but is not a precondition."),
    ])
    add_footer(s, 57, TOTAL, BRAND)

    # ---- Section 8: Worked example ----------------------------------
    s = section_divider(prs, 8, "Worked Example",
        "Financial fraud disclosure, end-to-end.")
    add_footer(s, 58, TOTAL, BRAND)

    s = content_slide(prs, "The scenario", bullets=[
        ("Alice is a senior risk analyst at Bank X.",
         "Top-five US bank by assets. Eight years tenure. "
         "CFA and CISSP."),
        ("She discovers systematic misreporting.",
         "Loan-loss reserves are systematically understated, "
         "inflating reported profits. Documentation is extensive."),
        ("She wants to disclose.",
         "To regulators and journalists simultaneously. Protect "
         "career and family. Preserve anonymity indefinitely if "
         "possible."),
        ("Under current infrastructure.",
         "She could use SecureDrop. The journalist would require "
         "her to eventually identify herself to establish "
         "credibility. She would then carry legal exposure."),
    ])
    add_footer(s, 59, TOTAL, BRAND)

    s = _im(prs, "Timeline: Alice's disclosure end-to-end",
        "chart_worked.png",
        caption="Eight phases over roughly eighteen months, from "
                "discovery to enforcement action.",
        image_h=4.8)
    add_footer(s, 60, TOTAL, BRAND)

    s = content_slide(prs, "Setup phase (weeks 1-6)", bullets=[
        ("Week 0: Discovery.",
         "Alice identifies the pattern. Begins mental planning."),
        ("Weeks 1-4: Infrastructure.",
         "Generates Quidnug keypair on air-gapped laptop. Obtains "
         "credential from Institute of Banking and Finance via "
         "blind-signature flow. Scrubs document metadata."),
        ("Weeks 4-6: Peer attestations.",
         "Identifies two colleagues she trusts. Each provides "
         "pseudonymous peer attestation via their own keypair. "
         "Communication via QDP-0024 MLS group."),
        ("Operational notes.",
         "All work done on personal hardware, air-gapped from "
         "work accounts. Tails on USB. No internet connection "
         "to work identity at any point."),
    ])
    add_footer(s, 61, TOTAL, BRAND)

    s = content_slide(prs, "Disclosure phase (week 6)", bullets=[
        ("Connect to SecureDrop.",
         "Alice uses Tor Browser on Tails. Connects to the "
         "Financial Times's SecureDrop instance."),
        ("Submit the package.",
         "Credential-backed disclosure. 17 supporting documents. "
         "12 documents retain bank's corporate PKI signatures. "
         "Contact channel: QDP-0024 MLS group with her pseudonym."),
        ("Journalist receives.",
         "Jordan K. at FT has the package. Begins verification."),
        ("Verification automatic for chain.",
         "Credential chains to IBF's DNS anchor. Peer pseudonyms "
         "have track records (or are new; disclosed). Document "
         "signatures verify against Bank X's public PKI."),
    ])
    add_footer(s, 62, TOTAL, BRAND)

    s = content_slide(prs, "Verification + reporting (weeks 6-12)",
        bullets=[
        ("Cross-reference with public records.",
         "Bank X's SEC filings. Earnings call transcripts. "
         "Regulator inspection history. Do the claims cohere "
         "with knowable context?"),
        ("Independent sources.",
         "Jordan finds two additional sources via traditional "
         "methods. Their accounts corroborate Alice's disclosure."),
        ("Editorial judgment.",
         "Is the story in the public interest? Clearly yes. "
         "Is the level of detail responsible? Revisions narrow "
         "some identifiable specifics."),
        ("Publication confidence reached.",
         "FT is ready to publish. Publication package includes "
         "credibility chain metadata."),
    ])
    add_footer(s, 63, TOTAL, BRAND)

    s = content_slide(prs, "Publication (week 12)", bullets=[
        ("Source description published.",
         "'A senior risk analyst at a top-five US bank, with "
         "certifications in banking and finance, attested to by "
         "the Institute of Banking and Finance. The source's "
         "credentials and documents have been independently "
         "verified.'"),
        ("Verification page links.",
         "Technically-inclined readers can examine attestation "
         "chains, pseudonym track records, and DNS-anchored "
         "identity proofs."),
        ("Not revealed.",
         "Alice's name. Bank X's specific name. Her team. Her "
         "geographic location."),
        ("Reader reception.",
         "Story carries unusual weight for an anonymous-source "
         "piece. Liar's dividend is neutralized: critics can't "
         "claim 'made up' because attestation chain is verifiable."),
    ])
    add_footer(s, 64, TOTAL, BRAND)

    s = content_slide(prs,
        "Regulatory response (weeks 14+)",
        bullets=[
        ("SEC receives the tip.",
         "Alice's pseudonym files the tip via SEC's whistleblower "
         "portal. Attestation chain included."),
        ("SEC verifies attestation.",
         "Pseudonym is established. Institutional attestation "
         "chains to IBF. Documents are corporate-signed. SEC "
         "initiates formal investigation."),
        ("Investigation proceeds.",
         "SEC subpoenas bank records, conducts interviews, "
         "engages forensic accountants. Alice's pseudonym is "
         "contactable via QDP-0024 throughout."),
        ("Enforcement action.",
         "Months later, SEC reaches enforcement action. Bank X "
         "settles. Substantial penalty. Alice's disclosures "
         "are vindicated."),
    ])
    add_footer(s, 65, TOTAL, BRAND)

    s = content_slide(prs,
        "Award processing (months 18+)",
        bullets=[
        ("Dodd-Frank award eligibility.",
         "Alice's disclosure led to enforcement over the $1M "
         "threshold. Pseudonym files a claim for 10-30% of "
         "recoveries."),
        ("Identity required for payment.",
         "SEC cannot pay a pseudonym. Alice identifies herself "
         "to SEC only, under statutory confidentiality obligations."),
        ("Public disclosure intact.",
         "The journalism and public discussion preserved Alice's "
         "anonymity throughout. Her identity is known to SEC "
         "(for payment) but not to the bank, not to the public, "
         "not to the journalist."),
        ("Retaliation protection.",
         "Dodd-Frank anti-retaliation provisions apply. SEC "
         "confidentiality prevents identity leak. Alice remains "
         "in her role with no discoverable connection to the tip."),
    ])
    add_footer(s, 66, TOTAL, BRAND)

    s = content_slide(prs,
        "What would have happened without the substrate",
        bullets=[
        ("Journalist would require identification.",
         "To establish credibility, the journalist would need "
         "Alice's identity (at minimum). The identification "
         "itself creates legal exposure."),
        ("Risk calculus deters disclosure.",
         "Many potential whistleblowers do not come forward "
         "because the credibility requirement is identification. "
         "The substrate lowers this barrier."),
        ("Bank X continues misreporting.",
         "Without the disclosure, the practice continues. "
         "Investors are misled. Eventual correction is more "
         "painful."),
        ("The substrate enables the disclosure.",
         "Credibility-without-identification is what lets Alice "
         "come forward at all."),
    ])
    add_footer(s, 67, TOTAL, BRAND)

    # ---- Section 9: Honest limits -----------------------------------
    s = section_divider(prs, 9, "Honest Limits and Ethical Tensions",
        "What the substrate doesn't solve.")
    add_footer(s, 68, TOTAL, BRAND)

    s = content_slide(prs, "Bad-faith whistleblowers", bullets=[
        ("Not all whistleblowers are good-faith.",
         "Some have grudges, run disinformation campaigns, or "
         "operate in adversarial intelligence contexts."),
        ("The substrate does not distinguish.",
         "Cryptographic attestation verifies attributes, not "
         "motive. An attested senior engineer can still be "
         "running an agenda."),
        ("Sophisticated adversaries.",
         "State-level actors can plausibly create fake "
         "credentials, fake peer attestations, fake document "
         "signatures if they compromise Attesters."),
        ("Mitigation, not elimination.",
         "No technology fully defeats sophisticated adversaries. "
         "The substrate raises the cost. Journalist vetting and "
         "cross-corroboration remain necessary."),
    ])
    add_footer(s, 69, TOTAL, BRAND)

    s = content_slide(prs, "Attestation authority gatekeeping",
        bullets=[
        ("Who decides which institutions can attest?",
         "No central authority. Readers weight credentials by "
         "their own trust graphs."),
        ("This is a feature.",
         "Different readers may weight the same credential "
         "differently. A reader who trusts IBF sees those "
         "credentials as strong; a skeptical reader sees them "
         "as weak."),
        ("Perspectives preserved.",
         "Rather than overridden by a central score. This aligns "
         "with the relational trust philosophy from PoT."),
        ("New institutions can participate.",
         "Emerging professional bodies can issue credentials. "
         "Their weight starts low and grows with use and "
         "track record."),
    ])
    add_footer(s, 70, TOTAL, BRAND)

    s = content_slide(prs, "The non-credentialed worker problem",
        bullets=[
        ("Not every insider has professional-body membership.",
         "Gig workers, undocumented workers, low-paid service "
         "workers can be key insiders to important stories but "
         "lack institutional attestation."),
        ("Peer attestation partially fills the gap.",
         "Coworkers attesting to employment does not require a "
         "professional body. Weaker than institutional, but "
         "non-zero."),
        ("Union local attestation.",
         "Where applicable, union locals can function as "
         "attestation authorities. This broadens access."),
        ("Honest acknowledgment.",
         "The substrate does privilege credentialed professionals. "
         "This is a real equity concern. Peer and union attestation "
         "help but do not fully address it."),
    ])
    add_footer(s, 71, TOTAL, BRAND)

    s = content_slide(prs, "Court compulsion and limits",
        bullets=[
        ("Courts can compel source identification.",
         "Under certain circumstances. US shield laws vary by "
         "state. Federal courts have ordered journalists to "
         "disclose."),
        ("Substrate limits what journalists know.",
         "If operational security was preserved, the journalist "
         "genuinely does not know the source's identity. No "
         "information to disclose."),
        ("Adversary pivots to Attester.",
         "Court orders could then target the attestation authority. "
         "Depending on jurisdiction, the Attester may have its "
         "own privilege protections or be outside court reach."),
        ("Not absolute protection.",
         "Sophisticated legal action can still reach sources "
         "through surveillance, operational security compromise, "
         "or document provenance investigation."),
    ])
    add_footer(s, 72, TOTAL, BRAND)

    s = content_slide(prs,
        "Disinformation and the responsibility gap",
        bullets=[
        ("Trust-weighted, not binary.",
         "The substrate presents credibility as a spectrum "
         "(0.0 - 1.0), not a 'verified' badge. A reader's "
         "calculation can return 0.3, clearly weak."),
        ("Defense against false certainty.",
         "Honest framing that credibility is probabilistic is "
         "itself defense against misinterpretation."),
        ("Anonymous-shifted responsibility.",
         "Anonymous sources shift consequence entirely to the "
         "publication. Pseudonyms partially restore accountability "
         "via repeated-game dynamics."),
        ("Honest framing matters.",
         "Publications deploying this substrate should explain "
         "what the credibility score means, not treat it as a "
         "seal of approval."),
    ])
    add_footer(s, 73, TOTAL, BRAND)

    # ---- Closing ----------------------------------------------------
    s = content_slide(prs, "What this substrate enables",
        bullets=[
        ("Credible anonymous disclosure at scale.",
         "Sources who cannot currently reach credibility because "
         "they cannot expose themselves can now participate."),
        ("Cross-publication credibility portability.",
         "A pseudonym's track record moves with the source across "
         "outlets. Reputation is not outlet-captured."),
        ("Reader audit trails.",
         "Readers can independently verify what the publication "
         "claims. The liar's dividend is neutralized."),
        ("Regulatory tip channels.",
         "SEC, CFTC, EU-level regulators can accept attested "
         "pseudonymous tips with confidence in source category."),
        ("A third option for journalists.",
         "Not 'protect completely but accept reduced credibility.' "
         "Not 'get credibility but expose the source.' A genuine "
         "third path exists."),
    ])
    add_footer(s, 74, TOTAL, BRAND)
    add_notes(s, [
        "Summary of what changes.",
        "The third option for journalists is the headline. Until "
        "now, the journalist's choice was stark: protect the source "
        "and accept 'anonymous source' discount; or require "
        "identification and get full credibility.",
        "The substrate offers a third path where the source stays "
        "anonymous and credibility is verifiable. This is a "
        "material change in the structure of investigative "
        "journalism, not an incremental one."])

    s = closing_slide(prs, "References and next steps",
        subtitle="Cryptography, journalism literature, and pilot engagement",
        cta="Journalists, NGOs, and regulatory tip-line operators "
            "interested in pilot deployments can engage via the Quidnug "
            "repository. Legal counsel is recommended for any deployment "
            "in sensitive jurisdictions.",
        resources=[
            "Foundational: Chaum 1985, 1988 (blind signatures); "
            "Camenisch-Lysyanskaya 2001, 2003; BBS+; RFC 9576 Privacy Pass.",
            "Journalism: Singer 2007; Robinson 2011; Donaldson 2020; "
            "Radsch 2019 (CPJ); ACLU 2023 (Espionage Act).",
            "Identity: Donath 1999 (virtual community identity); "
            "Axelrod 1984 (Evolution of Cooperation).",
            "Platforms: SecureDrop (Freedom of the Press Foundation); "
            "GlobaLeaks (Hermes Center); Signal Foundation.",
            "Quidnug: QDP-0021 blind signatures; QDP-0023 DNS-anchored "
            "identity; QDP-0024 private communications.",
        ])
    add_footer(s, 75, TOTAL, BRAND)
    add_notes(s, [
        "References for the talk.",
        "Everything defended in the talk traces to a specific "
        "citation. No appeal to our own authority; every claim "
        "is rooted in public literature or deployed systems.",
        "For pilot engagement, the Quidnug repository is the "
        "starting point. Legal counsel is essential for any "
        "deployment in jurisdictions where whistleblower "
        "protections are weak or retaliation risk is high.",
        "Thank you."])

    prs.save(OUTPUT)
    count = len(prs.slides)
    print(f"wrote {OUTPUT} ({count} slides)")


if __name__ == "__main__":
    build()
