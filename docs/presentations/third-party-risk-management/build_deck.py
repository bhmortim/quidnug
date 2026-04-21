"""TPRM deck (~75 slides)."""
import pathlib, sys
HERE = pathlib.Path(__file__).parent
ASSETS = HERE / "assets"
OUTPUT = HERE / "third-party-risk-management.pptx"
sys.path.insert(0, str(HERE.parent))
from _deck_helpers import (  # noqa: E402
    make_presentation, title_slide, section_divider, content_slide,
    two_col_slide, stat_slide, quote_slide, table_slide, image_slide,
    code_slide, icon_grid_slide, closing_slide, add_notes, add_footer,
    TEAL, CORAL, EMERALD, AMBER, TEXT_MUTED,
)
from pptx.util import Inches  # noqa: E402

BRAND = "Quidnug  \u00B7  Third-Party Risk Management"
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
        "The Third-Party Risk Management Nightmare",
        "Why every CISO is asking the wrong questions, and what the "
        "trust-graph architecture that fixes it actually looks like.",
        eyebrow="QUIDNUG  \u00B7  ENTERPRISE SECURITY")
    add_footer(s, 1, TOTAL, BRAND)

    s = stat_slide(prs, "$100B+",
        "estimated cost of the SolarWinds supply chain attack alone "
        "(GAO + congressional testimony 2021).",
        context="Plus Log4j, MOVEit, Kaseya, Codecov, XZ. The "
                "supply chain is the attack surface. Vendor "
                "questionnaires don't help.",
        title="Where we are in 2026", stat_color=CORAL)
    add_footer(s, 2, TOTAL, BRAND)

    s = stat_slide(prs, "0.5%",
        "of vendor questionnaire answers are independently verified "
        "by the asking organization (Ponemon 2023).",
        context="The other 99.5% is self-attestation theater. We "
                "fund a $25B/year industry to produce paperwork "
                "that doesn't reduce risk.",
        title="The honest measurement")
    add_footer(s, 3, TOTAL, BRAND)

    s = content_slide(prs, "Agenda", bullets=[
        ("1. Taxonomy of supply chain attacks.",
         "Categories, scale, cost data."),
        ("2. Why vendor questionnaires fail.",
         "Self-attestation, scalability, the compliance theater."),
        ("3. Why SBOMs are not enough.",
         "What they tell you, what they don't, the SLSA framework."),
        ("4. The Nth-party problem.",
         "Visibility decays exponentially with depth."),
        ("5. Cross-organization trust graphs.",
         "Computing Nth-party exposure cryptographically."),
        ("6. Signed component attestations.",
         "Sigstore + SLSA + peer attestations integrated."),
        ("7. Migration path.",
         "18 months from current TPRM to trust-graph TPRM."),
        ("8. Economic analysis.",
         "What this saves. ROI and crossover point."),
    ])
    add_footer(s, 4, TOTAL, BRAND)

    s = content_slide(prs, "Four claims this talk defends",
        bullets=[
            ("1. Vendor questionnaires don't reduce risk.",
             "They produce paperwork. The questionnaire industry "
             "exists because nobody has a better answer."),
            ("2. SBOMs are necessary but radically insufficient.",
             "Telling you what's installed says nothing about "
             "trustworthiness."),
            ("3. The Nth-party problem requires graph-based visibility.",
             "Linear vendor lists cannot answer 'who depends on what.'"),
            ("4. The path forward exists today.",
             "Sigstore, SLSA, attestation frameworks work. Quidnug "
             "ties them into a queryable trust graph."),
        ])
    add_footer(s, 5, TOTAL, BRAND)

    # Section 1
    s = section_divider(prs, 1, "The Taxonomy of Supply Chain Attacks",
        "Six categories. Each well-documented. Each catastrophic.")
    add_footer(s, 6, TOTAL, BRAND)

    s = _im(prs, "Major supply chain breaches: economic impact",
        "chart_breaches.png",
        caption="Sources: GAO, IBM Cost of Data Breach Report, "
                "Coveware ransomware reporting, vendor disclosures.",
        image_h=4.4)
    add_footer(s, 7, TOTAL, BRAND)

    s = content_slide(prs, "Category 1: software supply chain compromise",
        bullets=[
            ("SolarWinds (December 2020).",
             "Russian APT compromised the build pipeline of SolarWinds "
             "Orion. Tainted updates pushed to ~18,000 customers "
             "including US federal agencies."),
            ("Codecov (April 2021).",
             "Compromised bash uploader script, stole credentials from "
             "thousands of CI/CD pipelines."),
            ("XZ utils backdoor (March 2024).",
             "Malicious maintainer over 2 years built trust, slipped "
             "in backdoor that would have compromised SSH on every "
             "Linux distro."),
            ("Common pattern.",
             "Compromise upstream; downstream consumers inherit "
             "the compromise via routine updates."),
            ("Mitigation that works.",
             "Cryptographic signing of build outputs (Sigstore + SLSA). "
             "Plus trust-graph evaluation of who's trustworthy "
             "to consume from."),
        ])
    add_footer(s, 8, TOTAL, BRAND)

    s = content_slide(prs, "Category 2: dependency confusion + typosquatting",
        bullets=[
            ("Birsan 2021: dependency confusion.",
             "Successfully attacked Apple, Microsoft, PayPal, and 32+ "
             "other major orgs by uploading malicious packages with "
             "names matching internal dependencies."),
            ("Typosquatting.",
             "Packages like 'requesst' (extra s) targeting "
             "'requests'. PyPI removes hundreds per year."),
            ("Maintainer hijacking.",
             "left-pad incident (2016): 11-line library disabled, "
             "broke half the JavaScript ecosystem. Different incident, "
             "same fragility."),
            ("Common pattern.",
             "Trust based on package name string. Easy to spoof."),
            ("Mitigation.",
             "Signed packages, with cryptographic chain to "
             "named, trust-graph-evaluated maintainers."),
        ])
    add_footer(s, 9, TOTAL, BRAND)

    s = content_slide(prs, "Categories 3-6: full landscape", bullets=[
        ("Category 3. Vendor service compromise (Kaseya, MOVEit).",
         "Adversary compromises vendor's service; customers via "
         "vendor get compromised. ~$70B+ cost across Kaseya alone."),
        ("Category 4. Cloud / SaaS provider compromise.",
         "Okta CSR (Oct 2023), various SaaS data breaches. Customers "
         "have no visibility into vendor's internal security."),
        ("Category 5. Hardware supply chain.",
         "SuperMicro alleged 2018, real cases since. Compromised "
         "firmware shipping in supposedly-trusted hardware."),
        ("Category 6. Open-source maintainer compromise.",
         "XZ (2024), event-stream npm package (2018). Single-person "
         "maintainer = single point of compromise."),
        ("All six share root cause.",
         "We extend trust to entities we cannot independently verify. "
         "Trust without architecture is hope."),
    ])
    add_footer(s, 10, TOTAL, BRAND)

    s = content_slide(prs, "Scale: how big is this?", bullets=[
        ("Cost of supply chain attacks 2021-2024.",
         "Conservative estimate: $1T+ in direct + indirect impact "
         "(Gartner, IBM, Marsh insurance reports)."),
        ("Frequency.",
         "Sonatype 2024: 245,000+ malicious packages discovered in "
         "open source repositories. 633% YoY increase."),
        ("Coverage.",
         "Crowdstrike 2024: 67% of organizations experienced a "
         "third-party breach in the past 12 months."),
        ("Time-to-detect.",
         "IBM 2024: average 277 days to detect supply chain "
         "compromise. Often discovered by external researchers."),
        ("Geopolitical overlay.",
         "State actors increasingly use supply chain attacks. "
         "SolarWinds, Hafnium, Volt Typhoon: all leveraged trusted "
         "vendor relationships."),
    ])
    add_footer(s, 11, TOTAL, BRAND)

    s = quote_slide(prs,
        "We extend trust to entities we cannot independently verify. "
        "Trust without architecture is hope.",
        "The 2026 state of TPRM",
        title="The structural diagnosis")
    add_footer(s, 12, TOTAL, BRAND)

    # Section 2
    s = section_divider(prs, 2, "Why Vendor Questionnaires Fail",
        "Self-attestation, scalability, and the compliance-theater "
        "industry.")
    add_footer(s, 13, TOTAL, BRAND)

    s = _im(prs, "How TPRM works today",
        "chart_tprm_workflow.png",
        caption="200+ questions. 2-6 weeks. Self-attestation. Repeated "
                "annually. No verification.",
        image_h=4.0)
    add_footer(s, 14, TOTAL, BRAND)

    s = content_slide(prs, "What questionnaires actually measure",
        bullets=[
            ("Vendor's ability to fill out questionnaires.",
             "Not their actual security posture."),
            ("Vendor's ability to claim certifications.",
             "Not whether those certifications correspond to "
             "operational practices."),
            ("Vendor's willingness to write the answer they think "
             "you want.",
             "Most questionnaires are not even subtle about what "
             "answer earns approval."),
            ("None of the above measures what causes breaches.",
             "Most breaches happen from operational gaps that "
             "questionnaires explicitly attest don't exist."),
            ("Self-attestation is structurally inadequate for high-"
             "stakes risk.",
             "Same as if a tax form said 'do you owe taxes? Y/N' "
             "and that was the entire IRS."),
        ])
    add_footer(s, 15, TOTAL, BRAND)

    s = _im(prs, "The questionnaire industrial complex",
        "chart_questionnaire_pain.png",
        caption="Source: Ponemon Institute 2023 State of Third-Party "
                "Risk report.",
        image_h=4.2)
    add_footer(s, 16, TOTAL, BRAND)

    s = content_slide(prs, "The scalability problem", bullets=[
        ("Average Fortune 500 has ~3,700 unique vendor relationships.",
         ""),
        ("Each requires onboarding, periodic re-review.",
         "Annual re-questionnaire is industry standard."),
        ("3,700 vendors * 38 hours per questionnaire = 140k hours/year.",
         "That's 70 full-time employees doing questionnaire work."),
        ("Vendors push back.",
         "A vendor with 200 customers gets 200 questionnaires. They "
         "outsource to questionnaire-prep services that fill in "
         "answers from templates. Round trip is theater."),
        ("Real risk concentrates in 1-5% of vendors.",
         "Yet questionnaire effort distributes flatly. We optimize "
         "the wrong axis."),
    ])
    add_footer(s, 17, TOTAL, BRAND)

    s = content_slide(prs, "The compliance-adjacent market", bullets=[
        ("OneTrust, ProcessUnity, Prevalent, BitSight, SecurityScorecard.",
         "$25B+ market for vendor questionnaire automation and "
         "external scanning."),
        ("They sell.",
         "Faster questionnaire production, better external scanning, "
         "consolidated risk dashboards."),
        ("They do not solve.",
         "The fundamental problem: self-attestation isn't trust."),
        ("BitSight + SecurityScorecard external scanning helps.",
         "But measures only externally-visible attack surface, not "
         "internal security culture."),
        ("Net effect of $25B annual industry spend.",
         "Maybe 5-10% reduction in breach risk. Most spend produces "
         "process artifacts, not security."),
    ])
    add_footer(s, 18, TOTAL, BRAND)

    s = quote_slide(prs,
        "Self-attestation is structurally inadequate for high-stakes "
        "risk. Replacing 'tell me you're secure' with "
        "'cryptographically prove your peers trust you' is the "
        "architectural shift.",
        "What needs to change",
        title="The architectural shift")
    add_footer(s, 19, TOTAL, BRAND)

    # Section 3
    s = section_divider(prs, 3, "Why SBOMs Are Not Enough",
        "Necessary, but radically insufficient.")
    add_footer(s, 20, TOTAL, BRAND)

    s = content_slide(prs, "What an SBOM is", bullets=[
        ("Software Bill of Materials.",
         "Inventory of every software component (library, package, "
         "dependency) used in a piece of software."),
        ("Standardized formats.",
         "SPDX (Linux Foundation), CycloneDX (OWASP). Machine-"
         "readable JSON or XML."),
        ("EO 14028 (May 2021) mandate.",
         "US executive order requires SBOMs for federal software "
         "procurement."),
        ("Adoption growing.",
         "Major vendors now produce SBOMs by default. CISA + "
         "NIST + OWASP all push adoption."),
        ("Necessary first step.",
         "You cannot manage risk for components you don't know "
         "exist."),
    ])
    add_footer(s, 21, TOTAL, BRAND)

    s = content_slide(prs, "What an SBOM tells you", bullets=[
        ("Component identity.",
         "'This product contains apache-commons-text v1.10.0.'"),
        ("Composition.",
         "'It also contains 247 other libraries in transitive "
         "dependencies.'"),
        ("Versioning.",
         "Specific versions, often with cryptographic hashes."),
        ("License.",
         "Apache 2.0, MIT, GPL, etc."),
        ("Provenance (sometimes).",
         "Where the component came from, who built it (when "
         "available)."),
    ])
    add_footer(s, 22, TOTAL, BRAND)

    s = content_slide(prs, "What an SBOM does NOT tell you",
        bullets=[
            ("Whether the component is trustworthy.",
             "An SBOM lists Log4j v2.14.1; doesn't tell you Log4j was "
             "compromised in v2.14.0-2.16.1 (Log4Shell)."),
            ("Whether the maintainers are trustworthy.",
             "An SBOM lists xz-utils; doesn't tell you the malicious "
             "maintainer added a backdoor."),
            ("Whether the build pipeline was tampered with.",
             "Same source code, different build outputs is undetectable "
             "from an SBOM alone."),
            ("Whether the component is actively maintained.",
             "Or abandoned five years ago."),
            ("What other consumers think of the component.",
             "Crowdsourced trust signals are absent."),
        ])
    add_footer(s, 23, TOTAL, BRAND)

    s = content_slide(prs, "The SLSA framework", bullets=[
        ("Supply chain Levels for Software Artifacts.",
         "Google + Linux Foundation initiative. Spec at slsa.dev."),
        ("Four levels.",
         "L1: documented build process. L2: hosted build service. "
         "L3: hardened build, isolated workers. L4: two-person review, "
         "hermetic build, reproducible."),
        ("L4 is the goal.",
         "Independent reproducibility = anyone can verify the build."),
        ("Currently maybe 5-10% of major open source achieves L3+.",
         "Adoption is growing but slowly."),
        ("Compatible with sigstore + SBOMs.",
         "SLSA is the build-time discipline; sigstore signs the "
         "results; SBOMs document composition."),
    ])
    add_footer(s, 24, TOTAL, BRAND)

    s = content_slide(prs, "What's still missing", bullets=[
        ("Cross-organizational trust signals.",
         "If 47 of your peer organizations trust this vendor, that's "
         "useful information. Today: invisible."),
        ("Component-level reputation.",
         "Sigstore signs WHO built it; doesn't carry information "
         "about whether to trust them."),
        ("Nth-party visibility.",
         "Even with perfect SBOMs of all your direct vendors, "
         "you can't see THEIR vendors' vendors."),
        ("Decision automation.",
         "Even with all the data, manual review of every vendor "
         "is operationally infeasible at scale."),
        ("These gaps require trust graphs.",
         "Specifically the kind Quidnug provides."),
    ])
    add_footer(s, 25, TOTAL, BRAND)

    # Section 4
    s = section_divider(prs, 4, "The Nth-Party Problem",
        "Visibility decays exponentially. Most attacks are "
        "Nth-party.")
    add_footer(s, 26, TOTAL, BRAND)

    s = _im(prs, "Nth-party visibility decays exponentially",
        "chart_nth_party.png",
        caption="Source: Ponemon 2023 State of Third-Party Risk.",
        image_h=4.4)
    add_footer(s, 27, TOTAL, BRAND)

    s = _im(prs, "The Nth-party visibility problem",
        "chart_vendor_graph.png",
        caption="The most damaging compromises live three or more "
                "hops away from your direct vendors.",
        image_h=4.4)
    add_footer(s, 28, TOTAL, BRAND)

    s = content_slide(prs, "Empirical depth: how far do real attacks reach?",
        bullets=[
            ("SolarWinds.",
             "Direct vendor: SolarWinds. Their compromise reached "
             "into ~18,000 customer environments. Most customers "
             "didn't know they had SolarWinds (it was 4-5 hops "
             "downstream in their procurement chain)."),
            ("Log4j.",
             "Direct dependency for ~3% of Java applications. "
             "Transitive dependency for ~70%+. Most affected "
             "organizations didn't know they used it."),
            ("MOVEit.",
             "Progress Software vendor. Affected organizations "
             "ranged from direct customers to 5+ hops downstream "
             "(third-party SaaS using MOVEit-using PaaS using "
             "MOVEit-using vendor)."),
            ("Pattern.",
             "The dangerous compromises are NOT in your direct "
             "vendors. They're 2-5 hops downstream."),
            ("Current TPRM cannot see this.",
             "Linear vendor lists; no graph awareness."),
        ])
    add_footer(s, 29, TOTAL, BRAND)

    s = content_slide(prs, "Why current architecture can't see this",
        bullets=[
            ("Direct vendors: you have visibility (questionnaires + "
             "contracts).",
             ""),
            ("Vendor's vendors: you have ZERO visibility.",
             "No legal relationship. No questionnaire access. No "
             "data sharing."),
            ("Vendors don't share their TPRM data with you.",
             "Competitive concerns + operational complexity."),
            ("Industry information sharing groups (FS-ISAC, etc) help "
             "marginally.",
             "Sector-specific. Aggregated. Slow."),
            ("Cross-organizational trust graphs would solve this.",
             "If your peer organizations publish their trust signals, "
             "you can compute Nth-party exposure cryptographically."),
        ])
    add_footer(s, 30, TOTAL, BRAND)

    # Section 5
    s = section_divider(prs, 5, "Cross-Organization Trust Graphs",
        "How Quidnug lets you compute Nth-party risk.")
    add_footer(s, 31, TOTAL, BRAND)

    s = _im(prs, "Five layers of the architecture",
        "chart_layers.png", image_h=4.2)
    add_footer(s, 32, TOTAL, BRAND)

    s = content_slide(prs, "Structure: Quidnug applied to TPRM",
        bullets=[
            ("Each organization (you, your vendors, their vendors) has "
             "a Quidnug quid.",
             "Cryptographic identity verifiable independently."),
            ("Each TRUST tx is a signed declaration.",
             "'Acme Corp trusts SaaS Vendor X for cloud-storage at "
             "weight 0.85, valid until 2026-12-31.'"),
            ("Trust transitively visible.",
             "If your peer organization Y trusts vendor X, and "
             "you trust Y, you can compute your indirect trust "
             "in X."),
            ("Per-domain scoping.",
             "Trust in cloud-storage doesn't auto-transfer to "
             "data-processing. Each capability domain is its own "
             "trust subgraph."),
            ("Standard Quidnug primitives.",
             "Same protocol that powers reviews, identity, healthcare "
             "consent."),
        ])
    add_footer(s, 33, TOTAL, BRAND)

    s = content_slide(prs, "Computing Nth-party exposure", bullets=[
        ("Query: 'What's my exposure to Vendor X?'",
         ""),
        ("Walk the trust graph.",
         "Find all paths from your org to Vendor X via vendors you "
         "directly trust."),
        ("Multiplicative decay (QRP-like).",
         "Each hop reduces trust. Long indirect paths weight less than "
         "short direct paths."),
        ("Aggregate signals.",
         "If 47 peers trust X, weighted by your trust in those peers."),
        ("Result: a numeric score for Vendor X.",
         "Trustworthy enough to use? Threshold-based decision. "
         "Auto-deny when below threshold."),
        ("Sub-second computation.",
         "Same trust-walk infrastructure as Quidnug reviews."),
    ])
    add_footer(s, 34, TOTAL, BRAND)

    s = content_slide(prs, "Negative signals", bullets=[
        ("Breaches are signed events on the trust graph.",
         "When SolarWinds was discovered compromised, "
         "trust edges to SolarWinds dropped to zero across "
         "the trust graph."),
        ("Auto-cascade.",
         "If your trust in SolarWinds drops, your trust in vendors "
         "that depend on SolarWinds also drops (multiplicatively)."),
        ("Real-time risk update.",
         "Hours after a breach is disclosed, your TPRM scoring "
         "reflects the new reality. Today: takes weeks."),
        ("Cross-industry visibility.",
         "An incident in one industry visible to all subscribed "
         "peers. Industry information-sharing-as-a-service."),
        ("Without doxxing the affected.",
         "Pseudonymous reporting possible; binary 'breach happened' "
         "without revealing who."),
    ])
    add_footer(s, 35, TOTAL, BRAND)

    s = content_slide(prs, "Positive signals", bullets=[
        ("Successful audits, certifications.",
         "Externally-attested SOC 2, ISO 27001, FedRAMP all become "
         "trust-graph signals."),
        ("Successful incident handling.",
         "Vendor disclosed quickly, remediated, communicated. "
         "Post-incident trust may RISE."),
        ("Long-term clean track record.",
         "5+ years of clean operations weights more than recent "
         "trust edges."),
        ("Peer endorsement.",
         "Your peers explicitly publishing trust in a vendor (with "
         "their reputational stake)."),
        ("Result: TPRM score is dynamic, multi-source, peer-"
         "validated.",
         "Not a snapshot questionnaire."),
    ])
    add_footer(s, 36, TOTAL, BRAND)

    s = content_slide(prs, "Privacy considerations", bullets=[
        ("Concern: 'I don't want competitors to see my vendor list.'",
         ""),
        ("Mitigation 1: pseudonymous trust edges.",
         "You can publish 'Trust this quid at weight 0.85' without "
         "naming yourself."),
        ("Mitigation 2: disclosure scope.",
         "Trust edges visible only to peers you've authorized. "
         "Industry-specific groups, not the full public graph."),
        ("Mitigation 3: aggregated signals.",
         "Even 'how many peers trust this vendor' is useful without "
         "knowing which peers."),
        ("Trade-off: more privacy = less auditability.",
         "Each org chooses based on their risk profile."),
    ])
    add_footer(s, 37, TOTAL, BRAND)

    # Section 6
    s = section_divider(prs, 6, "Signed Component Attestations",
        "Sigstore + SLSA + peer attestations integrated.")
    add_footer(s, 38, TOTAL, BRAND)

    s = content_slide(prs, "The composition", bullets=[
        ("Sigstore.",
         "Cryptographic signing of build outputs. Every package "
         "release signed by the maintainer. Verifiable cosign."),
        ("SLSA.",
         "Build-time discipline. Reproducible builds, hardened "
         "infrastructure, two-person review."),
        ("SBOMs.",
         "Component composition + version + license."),
        ("Quidnug trust graph.",
         "Cross-org peer attestations of component trustworthiness."),
        ("Together.",
         "Each component has: cryptographic identity (Sigstore) + "
         "build provenance (SLSA) + composition (SBOM) + peer "
         "trust (Quidnug). Full picture."),
    ])
    add_footer(s, 39, TOTAL, BRAND)

    s = content_slide(prs, "Worked example: evaluating Apache Commons Text",
        bullets=[
            ("Question: 'Should I use apache-commons-text v1.10?'",
             ""),
            ("Sigstore check.",
             "Yes, signed by Apache Commons project key."),
            ("SLSA level.",
             "L3 (hardened build infrastructure)."),
            ("SBOM analysis.",
             "Pulls in 4 sub-dependencies, all also Sigstore-signed."),
            ("Quidnug trust query.",
             "Your peer orgs (847 of them) have collective trust "
             "weight 0.94 in this component."),
            ("Decision: green light.",
             "All four signals positive. Auto-approved by your TPRM "
             "system. No questionnaire needed."),
        ])
    add_footer(s, 40, TOTAL, BRAND)

    s = content_slide(prs, "Integration with sigstore", bullets=[
        ("Sigstore already provides cryptographic attestation.",
         "Quidnug doesn't replace; it composes."),
        ("Mapping.",
         "Each Sigstore signing identity gets a Quidnug quid. "
         "Trust in that quid is editable in your trust graph."),
        ("Open Source projects can attest peer relationships.",
         "Apache HTTP Server publishes 'we trust Apache Commons "
         "Text at weight 0.95.' Visible globally."),
        ("Maintainer reputation as a signal.",
         "Maintainer with long clean track record + signed "
         "attestations from peer projects = high trust."),
        ("Compromise propagation.",
         "If maintainer compromised, signed revocation from peers "
         "propagates. Trust drops cryptographically."),
    ])
    add_footer(s, 41, TOTAL, BRAND)

    s = content_slide(prs, "Peer-to-peer attestations", bullets=[
        ("Industry information sharing groups (FS-ISAC, "
         "Health-ISAC, Auto-ISAC) become trust-graph "
         "participants.",
         ""),
        ("Each member publishes signed trust edges to vendors "
         "they use successfully.",
         ""),
        ("Aggregation.",
         "FS-ISAC member trust in Vendor X = aggregate of "
         "individual member trust edges."),
        ("Real-time updates.",
         "Vendor breach disclosed in member channel = trust edges "
         "drop instantly. All members see the update."),
        ("Cross-ISAC visibility.",
         "Healthcare ISAC and Financial ISAC can share trust "
         "signals with selective disclosure (without doxxing "
         "members)."),
    ])
    add_footer(s, 42, TOTAL, BRAND)

    # Section 7
    s = section_divider(prs, 7, "Migration Path from Current TPRM",
        "18 months from questionnaire theater to trust-graph "
        "automation.")
    add_footer(s, 43, TOTAL, BRAND)

    s = _im(prs, "Five phases over 18 months",
        "chart_migration.png", image_h=3.8)
    add_footer(s, 44, TOTAL, BRAND)

    s = content_slide(prs, "Phase 1: identity establishment (months 1-2)",
        bullets=[
            ("Issue Quidnug quid for your organization.",
             "Cryptographic identity. Tied to your DNS domain via "
             "QDP-0023 attestation."),
            ("Onboard 5-10 critical direct vendors.",
             "Coordinate quid issuance with their security teams."),
            ("Set up your trust graph node.",
             "Quidnug node hosted internally or via managed service."),
            ("Internal training.",
             "Security team understands the model. Procurement "
             "stays in their existing workflow for now."),
            ("Cost.",
             "Roughly 1 engineer-month for setup. Mostly "
             "coordination, not technical complexity."),
        ])
    add_footer(s, 45, TOTAL, BRAND)

    s = content_slide(prs, "Phase 2: trust edge publication (months 2-6)",
        bullets=[
            ("For each onboarded vendor: publish a TRUST tx.",
             "Including weight, scope (cloud-storage, data-processing, "
             "etc), valid-until."),
            ("Vendor publishes their dependencies + sub-vendor "
             "trust similarly.",
             "Now you have visibility into their direct vendors."),
            ("Begin computing Nth-party exposure.",
             "Trust graph walk gives quantitative exposure scores."),
            ("Existing TPRM data flows in parallel.",
             "Questionnaires still happening; new system runs "
             "alongside, validates against."),
            ("Identify low-trust paths.",
             "Where graph reveals exposure questionnaires didn't "
             "catch."),
        ])
    add_footer(s, 46, TOTAL, BRAND)

    s = content_slide(prs, "Phase 3: peer network membership (months 4-8)",
        bullets=[
            ("Join industry information sharing groups via Quidnug.",
             "FS-ISAC, Health-ISAC, A-ISAC depending on industry."),
            ("Subscribe to peer trust signals.",
             "Your peer org's trust in vendors becomes input to your "
             "evaluation."),
            ("Cross-org attestations.",
             "If 47 peers trust Vendor X at avg 0.91, that's a strong "
             "signal."),
            ("Anomaly detection.",
             "Vendor with 47 peer trusts suddenly drops to 12 = "
             "investigation trigger."),
            ("Effort.",
             "Minimal additional technical work. Mostly governance + "
             "agreement on disclosure norms."),
        ])
    add_footer(s, 47, TOTAL, BRAND)

    s = content_slide(prs, "Phase 4: component-layer integration (6-12)",
        bullets=[
            ("Deploy sigstore + SBOM tooling for your own builds.",
             "If not already done. CISA SLSA + Sigstore guidance."),
            ("Subscribe to component-level trust signals.",
             "Apache, Linux Foundation, OWASP attestation feeds."),
            ("Auto-evaluate components in your CI/CD.",
             "Build pipeline checks Quidnug trust score for every "
             "dependency."),
            ("Block low-trust components automatically.",
             "Threshold-based decisions in pipeline. Manual review "
             "only for borderline cases."),
            ("Effort.",
             "2-4 engineer-months. Most overhead is integration "
             "with existing build tooling."),
        ])
    add_footer(s, 48, TOTAL, BRAND)

    s = content_slide(prs, "Phase 5: decision automation (months 9-18)",
        bullets=[
            ("Replace questionnaire workflow for routine vendor "
             "evaluation.",
             "Trust graph score + component analysis = automated "
             "approval."),
            ("Escalate borderline cases.",
             "Score 0.6-0.8 = manual review. Above 0.8 = "
             "auto-approve. Below 0.6 = auto-deny or escalate."),
            ("Annual renewal becomes continuous monitoring.",
             "Trust scores update with each new signal. No "
             "annual questionnaire."),
            ("Procurement integration.",
             "Procurement workflow includes Quidnug trust check. "
             "Vendors below threshold rejected at procurement gate."),
            ("Compliance integration.",
             "Audit logs (QDP-0018) provide tamper-evident records "
             "for regulatory reviews."),
        ])
    add_footer(s, 49, TOTAL, BRAND)

    s = content_slide(prs, "What this looks like in 24 months",
        bullets=[
            ("Vendor onboarding latency.",
             "From 4-8 weeks to under 24 hours for vendors with "
             "existing peer trust signals."),
            ("Vendor evaluation cost.",
             "$12k+ per vendor down to under $1k (mostly automated "
             "infrastructure cost)."),
            ("Nth-party visibility.",
             "From 8% to 80%+. You see what your vendors depend on."),
            ("Time-to-detect compromise.",
             "From 277 days (IBM 2024 avg) to under 7 days for "
             "incidents involving signed peers."),
            ("FTE burden.",
             "70 FTE on questionnaires down to ~10 FTE on trust-"
             "graph operations and exception handling."),
        ])
    add_footer(s, 50, TOTAL, BRAND)

    s = content_slide(prs, "Compatibility with existing frameworks",
        bullets=[
            ("NIST SP 800-161.",
             "Cybersecurity Supply Chain Risk Management. Quidnug "
             "trust graph is a CSRM control."),
            ("EO 14028 (Improving the Nation's Cybersecurity).",
             "SBOM + SLSA mandate. Quidnug composes with these."),
            ("ISO 27036.",
             "Information security for supplier relationships. "
             "Quidnug TPRM workflow mappable."),
            ("SOC 2 Type II.",
             "Vendor management is a control category. Quidnug "
             "score is auditable evidence."),
            ("FedRAMP.",
             "Continuous monitoring. Quidnug provides the "
             "continuous part."),
        ])
    add_footer(s, 51, TOTAL, BRAND)

    # Section 8
    s = section_divider(prs, 8, "Economic Analysis",
        "What this saves. ROI and crossover point.")
    add_footer(s, 52, TOTAL, BRAND)

    s = _im(prs, "Per-vendor cost: questionnaire vs Quidnug",
        "chart_economics.png", image_h=4.0)
    add_footer(s, 53, TOTAL, BRAND)

    s = _im(prs, "Total TPRM cost over 24 months",
        "chart_roi.png",
        caption="Crossover at month 7. Quidnug architecture cheaper "
                "than status quo from then on.",
        image_h=4.0)
    add_footer(s, 54, TOTAL, BRAND)

    s = content_slide(prs, "ROI breakdown", bullets=[
        ("Year 1 investment.",
         "Roughly $300-500k for mid-size enterprise (1 senior + 1 "
         "junior engineer + tooling)."),
        ("Year 1 savings.",
         "~$200k from reduced questionnaire labor. Net cost: -$100k "
         "to -$300k year 1."),
        ("Year 2 onward savings.",
         "$1-2M annually from reduced labor + faster vendor "
         "onboarding + reduced breach exposure."),
        ("Plus risk reduction (hard to quantify but real).",
         "If you avoid even one supply chain incident at average "
         "$4.88M cost, that pays for the program for ~10 years."),
        ("Crossover at month 7.",
         "Quidnug architecture cheaper than status quo by month 7-9 "
         "depending on org size."),
    ])
    add_footer(s, 55, TOTAL, BRAND)

    s = content_slide(prs, "Honest tradeoff 1: ecosystem cold start",
        bullets=[
            ("Quidnug architecture works best when many peers "
             "participate.",
             "First adopters get less benefit than late adopters."),
            ("Mitigation: industry coalitions.",
             "FS-ISAC, Health-ISAC, sector-specific groups can "
             "coordinate adoption."),
            ("Vendor adoption.",
             "Easier to onboard vendors when many of their other "
             "customers also use Quidnug."),
            ("Network effects.",
             "Each peer adopting amplifies value for others. "
             "Adoption pace likely accelerates."),
            ("Realistic timeline.",
             "5-7 years to dominant industry adoption. "
             "Comparable to Sigstore's adoption curve."),
        ])
    add_footer(s, 56, TOTAL, BRAND)

    s = content_slide(prs, "Honest tradeoff 2: vendor cooperation",
        bullets=[
            ("Vendors must publish trust signals.",
             "Some will resist sharing data."),
            ("Compulsion via procurement.",
             "Major customers can require Quidnug onboarding as a "
             "contract condition."),
            ("Self-interest argument.",
             "Vendors with strong peer trust DON'T have to fill out "
             "questionnaires. Removes operational burden."),
            ("Privacy mitigation.",
             "Vendors can publish trust signals to specific peer "
             "groups, not the public graph."),
            ("Reluctant vendors stay on questionnaire workflow.",
             "Coexistence is fine. Adoption proceeds at the pace "
             "the market supports."),
        ])
    add_footer(s, 57, TOTAL, BRAND)

    s = content_slide(prs, "Honest tradeoff 3: trust calibration",
        bullets=[
            ("Setting trust weights requires human judgment.",
             "Initial calibration: organizational decision."),
            ("Auto-calibration over time.",
             "Trust weights adjust based on observed vendor "
             "behavior + peer signals."),
            ("Risk of mis-calibration.",
             "Set thresholds wrong: false negatives (missed risks) or "
             "false positives (rejected good vendors)."),
            ("Mitigation.",
             "Conservative initial thresholds. Gradual tightening "
             "with experience. Override paths for human judgment."),
            ("Same as any new control.",
             "Tuning takes 6-12 months."),
        ])
    add_footer(s, 58, TOTAL, BRAND)

    s = content_slide(prs, "Honest tradeoff 4: regulatory uncertainty",
        bullets=[
            ("Some regulators may not yet recognize trust-graph TPRM "
             "as compliance evidence.",
             ""),
            ("Mitigation.",
             "Run Quidnug architecture alongside questionnaires for "
             "first 12-18 months. Both pass audit."),
            ("Engage regulators early.",
             "Demonstrate that trust-graph evidence exceeds "
             "questionnaire evidence in rigor."),
            ("Industry coalitions can drive recognition.",
             "If FS-ISAC adopts, banking regulators follow. If "
             "Health-ISAC adopts, HHS / OCR follows."),
            ("Standards alignment.",
             "Already mappable to NIST 800-161, ISO 27036, SOC 2. "
             "Recognition is a function of education, not "
             "fundamental gaps."),
        ])
    add_footer(s, 59, TOTAL, BRAND)

    s = content_slide(prs, "What this protocol does NOT solve",
        bullets=[
            ("Insider threat.",
             "Vendor employee with malicious intent is hard to "
             "detect from trust signals alone."),
            ("Zero-day vulnerabilities in trusted code.",
             "Trust graph signals quality of maintainers, not absence "
             "of unknown bugs."),
            ("Geopolitical realignment.",
             "Vendor previously trusted, now in adversary jurisdiction. "
             "Requires policy override."),
            ("Compromise of the trust-graph infrastructure itself.",
             "Quidnug node compromise is in scope for separate "
             "defense (QDP-0018 audit logs help)."),
            ("Wisdom about WHICH peers to weight heavily.",
             "Operator judgment still required."),
        ])
    add_footer(s, 60, TOTAL, BRAND)

    s = content_slide(prs, "Summary: the four claims revisited",
        bullets=[
            ("1. Vendor questionnaires don't reduce risk.",
             "$25B annual industry produces paperwork, not "
             "cryptographic verification."),
            ("2. SBOMs are necessary but insufficient.",
             "Composition without trust signals doesn't help "
             "decision-making."),
            ("3. The Nth-party problem requires graph-based visibility.",
             "Linear vendor lists cannot see what your vendors "
             "depend on."),
            ("4. The path forward exists today.",
             "Sigstore + SLSA + Quidnug trust graph composes into "
             "a working architecture."),
        ])
    add_footer(s, 61, TOTAL, BRAND)

    s = content_slide(prs, "What to do this year (CISO)",
        bullets=[
            ("Audit your TPRM workflow.",
             "How many vendor questionnaires are filled out? How "
             "many are independently verified? What's the cost?"),
            ("Read NIST SP 800-161r1.",
             "Cybersecurity Supply Chain Risk Management framework. "
             "Aligns with the architecture proposed here."),
            ("Pilot Quidnug TPRM with 5-10 critical vendors.",
             "Six-month pilot. Measure: vendor onboarding time, "
             "Nth-party visibility achieved."),
            ("Engage your industry ISAC.",
             "FS-ISAC, Health-ISAC, etc. Push for trust-graph "
             "data sharing standards."),
            ("Update procurement criteria.",
             "Vendors with Quidnug trust signals get expedited "
             "review. Drives ecosystem adoption."),
        ])
    add_footer(s, 62, TOTAL, BRAND)

    s = content_slide(prs, "What to do this year (vendor)",
        bullets=[
            ("Issue a Quidnug quid for your organization.",
             "DNS-anchored to your domain. Free, open standard."),
            ("Publish your security attestations.",
             "SOC 2, ISO 27001, FedRAMP all become trust signals."),
            ("Publish your dependency trust graph.",
             "Sub-vendors you trust, with weights. Demonstrate "
             "supply chain hygiene."),
            ("Engage with major customers' Quidnug rollouts.",
             "Be the easy vendor to onboard."),
            ("Reduce questionnaire burden.",
             "Eventually replace 200-question forms with 'verify "
             "our Quidnug trust signals.'"),
        ])
    add_footer(s, 63, TOTAL, BRAND)

    s = content_slide(prs, "What to do this year (open source maintainer)",
        bullets=[
            ("Adopt Sigstore for releases.",
             "Sign every artifact. cosign verify becomes the default."),
            ("Pursue SLSA Level 3.",
             "Hardened build infrastructure. Reproducible builds."),
            ("Publish maintainer-to-maintainer trust signals.",
             "'Apache HTTP trusts Apache Commons Logging at 0.95'. "
             "Cross-project trust graph."),
            ("Disclose security incidents transparently.",
             "Trust graph rewards transparent post-incident "
             "remediation."),
            ("Resist single-maintainer dependencies in critical "
             "paths.",
             "XZ taught us. Distribute trust across multiple "
             "maintainers."),
        ])
    add_footer(s, 64, TOTAL, BRAND)

    s = content_slide(prs, "What to do this year (regulator)",
        bullets=[
            ("Recognize trust-graph TPRM as valid CSRM evidence.",
             "Update SOC 2, FedRAMP, NIST 800-161 guidance."),
            ("Mandate SBOM + provenance for federal procurement.",
             "EO 14028 already does. Extend to state/local."),
            ("Fund industry ISAC infrastructure.",
             "Trust-graph data sharing is public good. Public "
             "investment justified."),
            ("International coordination.",
             "EU NIS2, US CISA, UK NCSC alignment on supply chain "
             "trust standards."),
            ("Avoid mandating specific implementations.",
             "Quidnug is one option. Standards should be open. "
             "Implementation diversity is healthy."),
        ])
    add_footer(s, 65, TOTAL, BRAND)

    s = quote_slide(prs,
        "Vendor questionnaires answer 'tell me you're secure.' "
        "Trust graphs answer 'cryptographically prove your peers "
        "trust you.' One produces theater; the other produces "
        "evidence.",
        "The architectural shift",
        title="The architectural shift")
    add_footer(s, 66, TOTAL, BRAND)

    s = content_slide(prs, "References", bullets=[
        ("NIST SP 800-161r1 (2022). Cybersecurity Supply Chain "
         "Risk Management Practices for Systems and Organizations.",
         ""),
        ("EO 14028 (May 2021). Improving the Nation's Cybersecurity.",
         ""),
        ("Ponemon Institute 2023. State of Third-Party Risk Report.",
         ""),
        ("IBM Cost of a Data Breach Report 2024.",
         "Average cost: $4.88M. Avg time-to-detect: 277 days."),
        ("Sonatype 2024. State of the Software Supply Chain.",
         "245k+ malicious packages discovered. 633% YoY increase."),
        ("Crowdstrike 2024. Global Threat Report.",
         "67% of orgs experienced 3rd-party breach in 12 months."),
        ("Sigstore. sigstore.dev.",
         "Open standard for cryptographic signing of software "
         "artifacts."),
        ("SLSA. slsa.dev.",
         "Supply chain Levels for Software Artifacts framework."),
    ])
    add_footer(s, 67, TOTAL, BRAND)

    s = content_slide(prs, "More references", bullets=[
        ("CycloneDX (OWASP). cyclonedx.org.",
         "SBOM standard."),
        ("SPDX (Linux Foundation). spdx.dev.",
         "Alternative SBOM standard."),
        ("FS-ISAC, Health-ISAC, A-ISAC, Auto-ISAC.",
         "Industry information sharing organizations."),
        ("CISA Software Bill of Materials guidance.",
         "cisa.gov/sbom."),
        ("Quidnug protocol (github.com/quidnug/quidnug).",
         "Reference SDKs in Python, Go, JavaScript, Rust."),
        ("Companion blog post.",
         "blogs/2026-04-25-third-party-risk-management-nightmare.md."),
    ])
    add_footer(s, 68, TOTAL, BRAND)

    s = content_slide(prs, "Common objections, briefly", bullets=[
        ("'Vendors won't share data.'",
         "Major customers can require it. Ecosystem effects "
         "drive adoption. Selective-disclosure available."),
        ("'My industry doesn't have the maturity.'",
         "Start with 5-10 critical vendors. Iterate. ISAC adoption "
         "amplifies."),
        ("'It's still early.'",
         "Same was true of Sigstore in 2021. Now adopted by Apache, "
         "PyPI, Kubernetes. Early adopters get the durable advantage."),
        ("'It's complex.'",
         "Less complex than questionnaire infrastructure. Less "
         "complex than 70 FTEs filling forms."),
        ("'Compliance won't recognize it.'",
         "Run alongside questionnaires for 12-18 months. "
         "Recognition follows demonstrated value."),
    ])
    add_footer(s, 69, TOTAL, BRAND)

    s = content_slide(prs, "What success looks like in 2030", bullets=[
        ("Vendor onboarding under 24 hours for trusted-peer vendors.",
         ""),
        ("Nth-party visibility for 5+ depth standard.",
         "Major orgs can compute their indirect supply chain risk."),
        ("Automated decision-making for routine vendor changes.",
         "Manual review only for borderline cases."),
        ("Industry-wide signal sharing.",
         "Vendor breaches detected within hours via cross-org "
         "trust signals."),
        ("Questionnaire industry shrinks 70%+.",
         "Replaced by trust-graph evidence + targeted attestation."),
    ])
    add_footer(s, 70, TOTAL, BRAND)

    s = content_slide(prs, "Things we owe ourselves", bullets=[
        ("Open standards, not vendor lock-in.",
         "Quidnug protocol is open. Implementations should be "
         "diverse."),
        ("Privacy-preserving disclosure mechanisms.",
         "Selective disclosure for sensitive trust edges."),
        ("Independent audit of trust graph operators.",
         "Even the trust-graph infrastructure must be auditable."),
        ("Coordination across industries.",
         "Cross-ISAC standards. Trust signals portable across "
         "sectors."),
        ("Investment in education.",
         "CISOs, procurement teams, vendors all need to understand "
         "the new model."),
    ])
    add_footer(s, 71, TOTAL, BRAND)

    s = quote_slide(prs,
        "We do not need 200-question vendor surveys. We need "
        "cryptographically-verified peer trust. The technology "
        "exists. What we need is institutional will.",
        "The next generation of TPRM",
        title="The next generation of TPRM")
    add_footer(s, 72, TOTAL, BRAND)

    s = content_slide(prs, "Next steps", bullets=[
        ("This week. Audit your current TPRM cost and effectiveness.",
         ""),
        ("This month. Read NIST 800-161 and Sigstore documentation.",
         ""),
        ("This quarter. Pilot Quidnug TPRM with 5-10 critical vendors.",
         ""),
        ("This year. Phase 1-2 of the migration plan.",
         ""),
        ("Next year. Component-layer integration. Full peer-network "
         "membership.",
         ""),
    ])
    add_footer(s, 73, TOTAL, BRAND)

    s = quote_slide(prs,
        "Trust without architecture is hope. Architecture without "
        "trust is theater. We need both.",
        "The summary in one sentence",
        title="One-line summary")
    add_footer(s, 74, TOTAL, BRAND)

    s = closing_slide(prs,
        "Questions",
        subtitle="Thank you. The hard work begins now.",
        cta="Where does the trust-graph architecture fail in your "
            "org?\n\nWhich phase is your next bottleneck?\n\n"
            "What's your industry's coordination story?",
        resources=[
            "github.com/quidnug/quidnug",
            "blogs/2026-04-25-third-party-risk-management-nightmare.md",
            "NIST SP 800-161r1",
            "EO 14028 / Improving the Nation's Cybersecurity",
            "Sigstore: sigstore.dev",
            "SLSA: slsa.dev",
            "Industry ISACs (FS, Health, A, Auto)",
        ])
    add_footer(s, 75, TOTAL, BRAND)

    return prs


if __name__ == "__main__":
    prs = build()
    prs.save(str(OUTPUT))
    print(f"wrote {OUTPUT} ({len(prs.slides)} slides)")
