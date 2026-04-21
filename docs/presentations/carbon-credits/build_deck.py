"""Carbon Credits deck (~75 slides)."""
import pathlib, sys
HERE = pathlib.Path(__file__).parent
ASSETS = HERE / "assets"
OUTPUT = HERE / "carbon-credits.pptx"
sys.path.insert(0, str(HERE.parent))
from _deck_helpers import (  # noqa: E402
    make_presentation, title_slide, section_divider, content_slide,
    two_col_slide, stat_slide, quote_slide, table_slide, image_slide,
    code_slide, icon_grid_slide, closing_slide, add_notes, add_footer,
    TEAL, CORAL, EMERALD, AMBER, TEXT_MUTED,
)
from pptx.util import Inches  # noqa: E402

BRAND = "Quidnug  \u00B7  Carbon Credits"
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
        "Carbon Credits Are Being Gamed",
        "Trust-anchored provenance fixes the $900 billion ESG "
        "market's phantom credit problem.",
        eyebrow="QUIDNUG  \u00B7  CLIMATE")
    add_footer(s, 1, TOTAL, BRAND)

    s = stat_slide(prs, "94%",
        "of REDD+ credits from Verra's top-selling methodology are "
        "phantom, representing no real emission reductions.",
        context="Source: West et al., 'Action needed to make carbon "
                "offsets from forest conservation work for climate "
                "change mitigation,' Science 381, 873-877 (2023). "
                "The voluntary carbon market has an integrity crisis.",
        title="Where we are in 2026", stat_color=CORAL)
    add_footer(s, 2, TOTAL, BRAND)

    s = stat_slide(prs, "$2.1B",
        "peak annual voluntary carbon market value (2022). "
        "Now down to ~$720M after the integrity crisis broke.",
        context="The market collapsed on news of the phantom-credit "
                "scandal. Restoring integrity is the only path to "
                "growth beyond $1T as Paris Agreement targets require.",
        title="The market's crisis of confidence")
    add_footer(s, 3, TOTAL, BRAND)

    s = content_slide(prs, "Agenda", bullets=[
        ("1. How the VCM works.",
         "Credit lifecycle, market sizing, registries, methodologies."),
        ("2. The phantom credit problem.",
         "West et al 2023, Guardian investigation, other evidence."),
        ("3. Why current verification fails.",
         "Verifier selection, registry competition, information "
         "asymmetry."),
        ("4. The four integrity dimensions.",
         "Additionality, permanence, leakage, measurement."),
        ("5. Trust-anchored attestation architecture.",
         "Signed attestations at each step; buyer's trust computation."),
        ("6. Auditor reputation weighting.",
         "From reputation-opaque to reputation-computable."),
        ("7. Integration with ICVCM + VCMI.",
         "How the substrate composes with existing standards."),
        ("8. Worked example.",
         "Evaluating a REDD+ credit batch end-to-end."),
    ])
    add_footer(s, 4, TOTAL, BRAND)

    s = content_slide(prs, "Four claims this talk defends", bullets=[
        ("1. The phantom-credit problem is an attestation "
         "architecture problem.",
         "Not a methodology tweaking problem. Not a regulatory "
         "tweaking problem."),
        ("2. Current verification has structural conflicts of "
         "interest.",
         "Project developer pays the verifier. Market incentives "
         "misalign."),
        ("3. Multi-party cryptographic attestation plus trust-"
         "weighted auditors solve both.",
         "No single verifier can approve alone. Auditor reputation "
         "becomes computable."),
        ("4. This composes with ICVCM + VCMI, not against them.",
         "Existing standards become mechanically verifiable rather "
         "than aspirational."),
    ])
    add_footer(s, 5, TOTAL, BRAND)

    # Section 1
    s = section_divider(prs, 1, "How the VCM Works",
        "Credit lifecycle, market sizing, registries, methodologies.")
    add_footer(s, 6, TOTAL, BRAND)

    s = _im(prs, "The lifecycle of a carbon credit",
        "chart_lifecycle.png", image_h=3.8)
    add_footer(s, 7, TOTAL, BRAND)

    s = content_slide(prs, "The credit lifecycle in detail", bullets=[
        ("Project developer identifies an emission reduction "
         "opportunity.",
         "Forest protection, renewable energy, cookstoves, biochar, "
         "etc."),
        ("Chooses a methodology.",
         "From registries like Verra VCS, Gold Standard, ACR, CAR. "
         "Dozens of methodologies per registry."),
        ("Submits design for validation.",
         "Validation-Verification Body (VVB) reviews. VVBs are "
         "paid by the project developer."),
        ("Project operates.",
         "Emission reductions accumulate over project lifetime "
         "(typically 10-100 years)."),
        ("Same VVB returns for periodic verification.",
         "Measures reductions. Submits claim to registry."),
        ("Registry issues credits.",
         "One credit = 1 tCO2e claimed reduced or avoided."),
    ])
    add_footer(s, 8, TOTAL, BRAND)

    s = _im(prs, "VCM market size: growth, collapse, possible futures",
        "chart_market.png",
        caption="Actual data from Ecosystem Marketplace 2018-2024. "
                "Projections based on CMIA, McKinsey, BNEF 2025.",
        image_h=4.2)
    add_footer(s, 9, TOTAL, BRAND)

    s = content_slide(prs, "The registries", bullets=[
        ("Verra (VCS).",
         "Largest by volume. Issues roughly 60% of all VCM credits. "
         "Focus: land-use, REDD+, renewable energy."),
        ("Gold Standard.",
         "Focus on sustainable development co-benefits. More "
         "rigorous per-credit but smaller volume."),
        ("American Carbon Registry (ACR).",
         "North American focus. Methane reduction, forestry."),
        ("Climate Action Reserve (CAR).",
         "California Air Resources Board compliance market. "
         "Hybrid voluntary/compliance."),
        ("Plan Vivo.",
         "Smallholder + community focus. Smaller volume."),
        ("Puro.earth.",
         "Engineered removals (biochar, DAC). Newer, technology-"
         "focused."),
    ])
    add_footer(s, 10, TOTAL, BRAND)

    s = content_slide(prs, "Methodology types", bullets=[
        ("Avoidance / reduction.",
         "REDD+ (Reducing Emissions from Deforestation and forest "
         "Degradation). Renewable energy replacing fossil fuel. "
         "Methane avoidance."),
        ("Removal.",
         "Afforestation / reforestation. Biochar. Direct Air "
         "Capture (DAC). Carbonate mineralization."),
        ("Risk differences.",
         "Avoidance: hard to prove counterfactual (would it have "
         "happened anyway?). Removal: easier to measure but often "
         "more expensive per tonne."),
        ("Market mix.",
         "~80% avoidance / reduction credits historically. ~20% "
         "removal. Buyers shifting toward removal post-crisis."),
        ("Quality ranking.",
         "Engineered removals (DAC, biochar) > nature-based "
         "removals (afforestation) > nature-based avoidance "
         "(REDD+). Emerging consensus."),
    ])
    add_footer(s, 11, TOTAL, BRAND)

    s = content_slide(prs, "The buyer profile", bullets=[
        ("Major corporate buyers.",
         "Microsoft, Alphabet, Meta, Salesforce, Stripe, Shopify. "
         "Net-zero commitments driving demand."),
        ("Concerns.",
         "Reputational risk. Litigation risk (EU Green Claims "
         "Directive). Shareholder scrutiny."),
        ("Post-crisis shift.",
         "Major buyers moving away from REDD+ avoidance credits. "
         "Toward engineered removals and high-integrity "
         "nature-based."),
        ("Pricing spread widening.",
         "Low-quality REDD+ credits ~$2-4. High-quality DAC "
         "credits $400-800. The market is segmenting."),
        ("Ambition gap.",
         "Paris Agreement goals require massive VCM growth. "
         "Current integrity crisis blocks that growth."),
    ])
    add_footer(s, 12, TOTAL, BRAND)

    # Section 2
    s = section_divider(prs, 2, "The Phantom Credit Problem",
        "Empirical evidence of systematic integrity failure.")
    add_footer(s, 13, TOTAL, BRAND)

    s = _im(prs, "West et al. 2023: 94% phantom",
        "chart_phantom.png", image_h=4.2)
    add_footer(s, 14, TOTAL, BRAND)

    s = content_slide(prs, "West et al. 2023: the landmark study",
        bullets=[
            ("Thales West and colleagues.",
             "'Action needed to make carbon offsets from forest "
             "conservation work for climate change mitigation.' "
             "Science, August 2023."),
            ("Method.",
             "Analyzed 26 REDD+ projects representing ~10% of all "
             "VCM REDD+ credits by volume."),
            ("Compared credit claims to actual observed forest loss.",
             "Satellite-based measurement against counterfactual "
             "scenarios."),
            ("Result.",
             "Only 6% of claimed emission reductions corresponded "
             "to actual avoided deforestation."),
            ("Publication in Science.",
             "Not a niche critique. Peer-reviewed at the top journal. "
             "Methodology robust. Conclusions not refuted."),
        ])
    add_footer(s, 15, TOTAL, BRAND)

    s = content_slide(prs, "The Guardian + Die Zeit investigation",
        bullets=[
            ("Published January 2023.",
             "Joint investigation with SourceMaterial research group."),
            ("Focused on Verra's VCS REDD+ methodology.",
             "Largest single category of VCM credits."),
            ("Finding.",
             "More than 90% of Verra's rainforest-credit "
             "methodology was phantom. Projects claimed "
             "deforestation avoidance that didn't materialize."),
            ("Impact.",
             "Verra's CEO resigned May 2023. Market collapsed. "
             "Corporate buyers paused procurement."),
            ("Verra's response.",
             "Disputed methodology of the critique. Launched new "
             "methodology in 2024. But market confidence damaged."),
        ])
    add_footer(s, 16, TOTAL, BRAND)

    s = content_slide(prs, "Other documented integrity failures",
        bullets=[
            ("Cooley et al 2022.",
             "Analyzed cookstove credits. Found systematic over-"
             "crediting by factor of 10."),
            ("Gill-Wiehl et al 2024.",
             "Cookstove update: over-crediting likely 5-7x even "
             "with methodology improvements."),
            ("Thomas et al 2023.",
             "Analysis of California forest carbon offsets. Found "
             "29% over-crediting in offset portfolio."),
            ("West et al 2020 (earlier work).",
             "Independently validated concerns about specific "
             "methodology design."),
            ("Pattern.",
             "Not isolated incidents. Systemic issues across "
             "methodology categories."),
        ])
    add_footer(s, 17, TOTAL, BRAND)

    s = content_slide(prs, "The systematic issue", bullets=[
        ("It's not one bad methodology.",
         "Problems documented across REDD+, cookstoves, forest "
         "offsets, renewables. Different methodologies, same "
         "pattern."),
        ("It's the verification architecture.",
         "Project developer selects + pays verifier. Verifier "
         "has incentive to approve (to keep customer). Registry "
         "has incentive to issue (to grow market share)."),
        ("Each stakeholder rationally optimizes.",
         "Systems produce the outcomes that incentives reward."),
        ("No one is malicious.",
         "But collectively: phantom credits."),
        ("Fixing individual methodologies does not solve this.",
         "The substrate needs to change."),
    ])
    add_footer(s, 18, TOTAL, BRAND)

    s = quote_slide(prs,
        "Each actor in the carbon credit system rationally "
        "optimizes. The collective outcome is phantom credits. "
        "No amount of methodology tweaking solves this.",
        "The structural diagnosis",
        title="The structural diagnosis")
    add_footer(s, 19, TOTAL, BRAND)

    # Section 3
    s = section_divider(prs, 3, "Why Current Verification Fails",
        "Verifier selection, registry competition, information "
        "asymmetry.")
    add_footer(s, 20, TOTAL, BRAND)

    s = content_slide(prs, "The verifier selection problem", bullets=[
        ("Project developer chooses the VVB.",
         "From an accredited list. Paid by project developer."),
        ("Developer has strong incentive.",
         "To choose a VVB who will approve their claims."),
        ("VVB has strong incentive.",
         "To approve; rejections mean losing customer."),
        ("Accreditation bodies (ANSI, UKAS, etc) audit VVBs.",
         "But infrequently. Typically every 3-5 years."),
        ("The classical conflict of interest.",
         "Auditor paid by auditee. Well-documented failure mode "
         "across industries (Enron, Arthur Andersen, Wirecard)."),
    ])
    add_footer(s, 21, TOTAL, BRAND)

    s = content_slide(prs, "Registry competition", bullets=[
        ("Registries compete for project registrations.",
         "Volume drives their revenue."),
        ("Project developer choice.",
         "Developer can choose any accredited registry. "
         "Developer choose the one most likely to approve."),
        ("Race to the bottom.",
         "Registries that reject too often lose market share. "
         "Registries that approve more loosely gain market share."),
        ("Documented behavior.",
         "Verra grew to ~60% market share during the very years its "
         "methodologies produced the most phantom credits."),
        ("Not malice.",
         "Rational competitive response to market incentives. "
         "Same pattern would emerge under any regulator-less "
         "market."),
    ])
    add_footer(s, 22, TOTAL, BRAND)

    s = content_slide(prs, "Methodology complexity", bullets=[
        ("REDD+ methodology documents run 200+ pages.",
         "Highly technical. Complex baseline calculations. Many "
         "edge cases."),
        ("Buyer cannot independently verify.",
         "Understanding whether a specific project genuinely "
         "reduces emissions requires specialist expertise."),
        ("Information asymmetry.",
         "Project developer knows the reality on the ground. "
         "Buyer only sees the attestation."),
        ("Documentation theater.",
         "Voluminous documentation signals rigor. Doesn't prove "
         "substance."),
        ("Same issue as financial audits of complex derivatives.",
         "Lehman-era CDOs had rigorous documentation. The "
         "underlying assets were still junk."),
    ])
    add_footer(s, 23, TOTAL, BRAND)

    s = content_slide(prs, "Information asymmetry", bullets=[
        ("Project developer has complete information.",
         "Land use, operations, local political context, actual "
         "deforestation without project."),
        ("VVB has partial information.",
         "Site visit every few years. Trust in project developer's "
         "measurements for most ongoing data."),
        ("Registry has filtered information.",
         "Documents submitted by VVB. Trust VVB for accuracy."),
        ("Buyer has minimal information.",
         "Credit listing and project description. Trust registry "
         "for issuance."),
        ("Regulator has almost no information.",
         "Unless investigating fraud after-the-fact."),
    ])
    add_footer(s, 24, TOTAL, BRAND)

    s = content_slide(prs, "The rating agency band-aid", bullets=[
        ("BeZero, Sylvera, Calyx Global emerged 2021-2023.",
         "Independent rating of carbon credits. Partial response "
         "to the integrity crisis."),
        ("They help.",
         "Buyers who use ratings do better than buyers who rely "
         "on registry alone."),
        ("They are limited.",
         "Ratings based on same underlying data plus additional "
         "analysis. Cannot compensate for upstream information "
         "asymmetry."),
        ("They are not cryptographic.",
         "Same verification model as CRA bond ratings. Subject to "
         "same failure modes (over-optimistic bias, post-hoc "
         "criticism of AAA ratings of subprime MBS)."),
        ("They help but don't solve.",
         "Trust-anchored substrate solves what ratings agencies "
         "cannot."),
    ])
    add_footer(s, 25, TOTAL, BRAND)

    # Section 4
    s = section_divider(prs, 4, "The Four Integrity Dimensions",
        "Additionality, permanence, leakage, measurement accuracy.")
    add_footer(s, 26, TOTAL, BRAND)

    s = _im(prs, "The four integrity dimensions of a carbon credit",
        "chart_dimensions.png", image_h=4.2)
    add_footer(s, 27, TOTAL, BRAND)

    s = content_slide(prs, "Dimension 1: additionality", bullets=[
        ("Would the emission reduction have happened anyway?",
         "If yes, the credit is not additional. Not a real reduction."),
        ("The counterfactual.",
         "What would have happened without the project? Hard to "
         "measure precisely."),
        ("Non-additional examples.",
         "Renewable energy credits in markets where renewables are "
         "already cheapest option. REDD+ credits in areas not "
         "actually at deforestation risk."),
        ("Current approach.",
         "Project developer argues additionality. VVB accepts or "
         "rejects. Often accepted generously."),
        ("Substrate approach.",
         "Counterfactual signed by independent data source "
         "(satellite + economic model). Additionality becomes "
         "verifiable."),
    ])
    add_footer(s, 28, TOTAL, BRAND)

    s = content_slide(prs, "Dimension 2: permanence", bullets=[
        ("Does the reduction stay removed?",
         "Forest sequestered carbon can be released by future fire, "
         "disease, land-use change."),
        ("Forest permanence = ~40-100 years typical.",
         "Well below atmospheric CO2 residence time (~1000 years "
         "for some fraction)."),
        ("Geological sequestration (DAC + storage).",
         "Effective permanence. Millennia."),
        ("Current handling.",
         "'Buffer pools' hold some credits in reserve against "
         "reversal. Typically 10-30% of issued credits."),
        ("Substrate approach.",
         "Permanence monitored continuously via satellite. "
         "Reversal events are signed events. Automatic credit "
         "retirement on reversal."),
    ])
    add_footer(s, 29, TOTAL, BRAND)

    s = content_slide(prs, "Dimension 3: leakage", bullets=[
        ("Does the reduction cause emissions elsewhere?",
         "Protecting forest in area A may push deforestation to "
         "area B. Net reduction: less than claimed."),
        ("REDD+ is particularly vulnerable.",
         "Driver of deforestation (agricultural demand) doesn't "
         "disappear. Moves somewhere."),
        ("Current handling.",
         "Methodologies attempt to discount for leakage. But "
         "estimates often based on narrow geographic scope."),
        ("Substrate approach.",
         "Regional and national satellite monitoring. Signed "
         "attestations from multiple independent analysts. "
         "Leakage becomes measurable."),
        ("Difficult.",
         "Leakage is the hardest of the four dimensions. "
         "Requires macro-economic modeling plus satellite data."),
    ])
    add_footer(s, 30, TOTAL, BRAND)

    s = content_slide(prs, "Dimension 4: measurement accuracy",
        bullets=[
            ("Is the claimed reduction correctly measured?",
             "Satellite-based forest biomass measurement has "
             "error bars. Gas emissions from landfills are "
             "estimates, not direct measurements."),
            ("Methodology matters.",
             "Same forest can produce different carbon estimates "
             "depending on which allometric equations are used."),
            ("Current handling.",
             "Methodology specifies measurement approach. VVB "
             "verifies adherence."),
            ("Substrate approach.",
             "Raw measurement data signed by sensors / "
             "instruments. Analysis scripts signed. Re-computation "
             "possible by any party."),
            ("Most tractable dimension.",
             "Independent data + signed chain makes measurement "
             "accuracy most verifiable of the four."),
        ])
    add_footer(s, 31, TOTAL, BRAND)

    s = _im(prs, "Four-dimension scoring: which projects meet integrity?",
        "chart_dimensions_scoring.png",
        caption="A credit can only be high-integrity if all four "
                "dimensions meet the threshold.",
        image_h=4.2)
    add_footer(s, 32, TOTAL, BRAND)

    # Section 5
    s = section_divider(prs, 5, "Trust-Anchored Attestation Architecture",
        "Signed attestations at each step. Multi-party. Cryptographic.")
    add_footer(s, 33, TOTAL, BRAND)

    s = _im(prs, "Multi-party attestation stack",
        "chart_attestation.png",
        caption="Each primary source cross-signed by an independent "
                "attesting party.",
        image_h=4.2)
    add_footer(s, 34, TOTAL, BRAND)

    s = content_slide(prs, "Signed attestations at each step", bullets=[
        ("Satellite data signed at source.",
         "NASA MODIS / ESA Sentinel / Planet Labs sign their data "
         "cryptographically at acquisition."),
        ("Field measurements signed by field teams.",
         "Independent field scientists not affiliated with project "
         "developer sign observations at collection."),
        ("Analytical models signed.",
         "Peer-reviewed counterfactual models with signed "
         "implementations. Re-runnable by any party."),
        ("Auditor signatures carry reputation.",
         "VVB signature has computable auditor reputation score "
         "attached."),
        ("Registry issuance signed with state.",
         "Full attestation chain preserved. Buyer sees the whole "
         "chain, not just the final claim."),
    ])
    add_footer(s, 35, TOTAL, BRAND)

    s = content_slide(prs, "Adding independent attestations", bullets=[
        ("NASA signs satellite observations.",
         "Public NASA key. Data cannot be altered post-acquisition "
         "without breaking signature."),
        ("ESA provides independent satellite attestation.",
         "Cross-verification with NASA data. Two independent space "
         "agencies. Hard to compromise both."),
        ("Academic research groups.",
         "University forest science departments sign independent "
         "analyses."),
        ("NGO monitors.",
         "Global Forest Watch, Environmental Defense Fund sign "
         "independent project reviews."),
        ("Each signature is a TRUST edge.",
         "Buyer's trust graph decides how to weight each party."),
    ])
    add_footer(s, 36, TOTAL, BRAND)

    s = content_slide(prs, "The buyer's trust computation", bullets=[
        ("Query: 'What's my exposure to project X's integrity?'",
         ""),
        ("Walk the trust graph.",
         "How trustworthy is the VVB? What's NASA's trust weight "
         "on their satellite observations? How reliable is the "
         "counterfactual model's underlying data?"),
        ("Aggregate across independent attestations.",
         "Multiple signing parties reduce risk of single-source "
         "failure."),
        ("Apply integrity thresholds.",
         "Below threshold: credit rejected. Above: accept at "
         "computed confidence."),
        ("Result: per-credit quality score.",
         "Not issued by registry; computed by buyer from signed "
         "evidence."),
    ])
    add_footer(s, 37, TOTAL, BRAND)

    s = content_slide(prs, "What this changes for buyers", bullets=[
        ("Independent verification of integrity.",
         "No longer dependent on registry attestation alone."),
        ("Computable quality scores.",
         "Can filter credits by integrity threshold. Rejected bad "
         "credits before purchase."),
        ("Reduced reputational risk.",
         "If buyer's public commitment includes integrity standards, "
         "they can prove which credits met them."),
        ("Better pricing signal.",
         "Market can distinguish high-integrity from low-integrity "
         "credits. Prices diverge appropriately."),
        ("Aligned with regulator direction.",
         "EU Green Claims Directive, SEC climate disclosure rules "
         "require verifiable claims."),
    ])
    add_footer(s, 38, TOTAL, BRAND)

    s = content_slide(prs, "What this changes for projects", bullets=[
        ("Higher-integrity projects command premium prices.",
         "Market differentiates. Pays for substance."),
        ("Lower-integrity projects struggle to sell.",
         "Market de-funds them. Projects improve or exit."),
        ("Independent verification raises bar.",
         "Project developers must actually deliver. Documentation "
         "theater insufficient."),
        ("Collaboration with data sources.",
         "Projects partner with NASA, ESA, academic groups. "
         "Transparency becomes a selling point."),
        ("Smaller projects can compete.",
         "If they meet integrity bar, attestation chain levels the "
         "playing field."),
    ])
    add_footer(s, 39, TOTAL, BRAND)

    # Section 6
    s = section_divider(prs, 6, "Auditor Reputation Weighting",
        "From reputation-opaque to reputation-computable.")
    add_footer(s, 40, TOTAL, BRAND)

    s = content_slide(prs, "The current reputation signal", bullets=[
        ("Accredited VVBs are all formally equal.",
         "If accredited by ANSI / UKAS / DAKKS, can sign any "
         "methodology."),
        ("Buyers cannot tell good from bad VVBs.",
         "No public track record of VVB quality."),
        ("Rating agencies (BeZero, Sylvera, Calyx) try to fill gap.",
         "But their data is proprietary, non-cryptographic, and "
         "based on same sources as registries."),
        ("Whistleblowers surface egregious cases.",
         "E.g., Verra CEO resignation May 2023. But systemic "
         "quality differences invisible."),
        ("Substrate-based reputation fixes this.",
         "Track record made visible, auditable, cryptographically "
         "tied to signatures."),
    ])
    add_footer(s, 41, TOTAL, BRAND)

    s = _im(prs, "Auditor reputation: how often claims survive?",
        "chart_auditor_rep.png",
        caption="Illustrative. Reputation = correlation between "
                "auditor's approvals and post-issuance independent "
                "verification outcomes.",
        image_h=4.0)
    add_footer(s, 42, TOTAL, BRAND)

    s = content_slide(prs, "Substrate-based verifier reputation",
        bullets=[
            ("For each VVB, compute over their signature history.",
             "How often did their approvals correspond to claims "
             "that survived post-issuance independent verification?"),
            ("Track cumulative behavior.",
             "VVBs with consistently high-integrity approvals "
             "earn higher reputation."),
            ("VVBs approving too loosely face consequences.",
             "Reputation drops; buyers discount their signatures; "
             "registry market share declines."),
            ("Reputation per methodology.",
             "VVB can be good at forestry but poor at renewables. "
             "Per-domain reputation, not global."),
            ("Reputation as computed function.",
             "Not opinion. Not PR. Historical data analysis, "
             "cryptographically verifiable."),
        ])
    add_footer(s, 43, TOTAL, BRAND)

    s = content_slide(prs, "The computation", bullets=[
        ("Collect VVB's signature history.",
         "All approvals over past 5-10 years."),
        ("For each approval, track post-issuance outcome.",
         "Did credits survive independent review? Was project "
         "re-verified? Did underlying claims hold up?"),
        ("Multi-factor reputation.",
         "Integrity survival rate × volume × recency × domain."),
        ("Trust graph aggregation.",
         "Other trusted parties' reputation scores for the VVB."),
        ("Result: single number per (VVB, methodology, "
         "vintage) triple.",
         "Buyer's trust-weighted evaluation of the VVB's work."),
    ])
    add_footer(s, 44, TOTAL, BRAND)

    s = content_slide(prs, "What this changes", bullets=[
        ("VVB quality becomes market-visible.",
         "Buyers can select VVBs with high integrity scores."),
        ("Project developers reconsider VVB choice.",
         "Using a low-reputation VVB may mean credits don't sell. "
         "Economic pressure toward rigor."),
        ("Bad VVBs either improve or exit.",
         "Market selection against low-quality verification."),
        ("New VVBs face initial opacity.",
         "But substrate provides path to build reputation "
         "transparently."),
        ("The auditor profession rises.",
         "Good auditors earn higher trust. Opaque market becomes "
         "meritocratic."),
    ])
    add_footer(s, 45, TOTAL, BRAND)

    s = content_slide(prs, "The ecosystem effect", bullets=[
        ("Project developer choice of VVB becomes quality signal.",
         "Project using top-reputation VVB + willing to pay for "
         "rigor is signaling quality."),
        ("Project developer choice of registry becomes quality signal.",
         "Registry reputation matters; race to the bottom reverses."),
        ("Buyer selection criteria.",
         "Buyers can require 'VVB reputation > 0.85' + 'registry "
         "reputation > 0.8' + 'multi-party attestation.'"),
        ("Self-reinforcing.",
         "Low-integrity projects exit. High-integrity projects "
         "command premium."),
        ("Over time.",
         "The phantom credit problem resolves structurally. Market "
         "equilibrium shifts toward integrity."),
    ])
    add_footer(s, 46, TOTAL, BRAND)

    # Section 7
    s = section_divider(prs, 7, "Integration with ICVCM and VCMI",
        "How Quidnug composes with existing standards.")
    add_footer(s, 47, TOTAL, BRAND)

    s = _im(prs, "Quidnug composing with ICVCM + VCMI",
        "chart_icvcm.png", image_h=4.0)
    add_footer(s, 48, TOTAL, BRAND)

    s = content_slide(prs, "ICVCM Core Carbon Principles", bullets=[
        ("Integrity Council for the Voluntary Carbon Market.",
         "Launched 2023 with 10 Core Carbon Principles (CCPs)."),
        ("Supply-side standard.",
         "Certifies methodologies as meeting CCP integrity "
         "requirements."),
        ("10 principles.",
         "Additionality, permanence, measurement, transparent "
         "monitoring, independent verification, no double "
         "counting, etc."),
        ("Assessment process.",
         "ICVCM expert panel reviews methodologies. Approved "
         "methodologies receive 'CCP-Approved' label."),
        ("Status.",
         "~200 methodologies reviewed as of 2025. Partial list "
         "approved. Work ongoing."),
    ])
    add_footer(s, 49, TOTAL, BRAND)

    s = content_slide(prs, "VCMI Claims Code", bullets=[
        ("Voluntary Carbon Markets Integrity Initiative.",
         "Launched 2023 alongside ICVCM. Demand-side standard."),
        ("Tiered buyer claims.",
         "Platinum: retire 90%+ CCP credits. Gold: 60%+. Silver: "
         "partial. Each tier signals buyer commitment."),
        ("Designed to pair with ICVCM.",
         "Supply-side integrity (ICVCM) + demand-side integrity "
         "(VCMI) = full-chain integrity."),
        ("Corporate adoption.",
         "Microsoft, Salesforce among early adopters. Provides "
         "framework for credible climate claims."),
        ("Alignment with regulation.",
         "EU Green Claims Directive aligning with VCMI Claims "
         "Code principles."),
    ])
    add_footer(s, 50, TOTAL, BRAND)

    s = content_slide(prs, "How the substrate composes", bullets=[
        ("ICVCM assesses methodologies.",
         "Quidnug substrate verifies individual projects within "
         "those methodologies."),
        ("VCMI defines buyer claims.",
         "Quidnug substrate provides cryptographic evidence for "
         "those claims."),
        ("Standards + substrate together.",
         "ICVCM says 'this methodology is eligible.' Quidnug says "
         "'this specific project under that methodology has "
         "verifiable attestation chain.'"),
        ("No conflict, no duplication.",
         "ICVCM/VCMI provide the rules. Substrate provides the "
         "enforcement."),
        ("Existing approvals become verifiable.",
         "CCP-Approved methodologies become CCP-Approved-AND-"
         "verifiable projects."),
    ])
    add_footer(s, 51, TOTAL, BRAND)

    s = content_slide(prs, "The broader market effect", bullets=[
        ("With substrate deployed.",
         "ICVCM approval becomes more credible (methodology-"
         "level). VCMI claims become more credible (buyer-level)."),
        ("Price spreads widen legitimately.",
         "High-integrity ICVCM + substrate-verified credits "
         "command premium. Low-integrity credits struggle to sell."),
        ("Market bifurcation resolved.",
         "Current: buyers can't tell integrity. Future: integrity "
         "is the primary pricing dimension."),
        ("Total market growth.",
         "With integrity confidence restored, major buyers return. "
         "Market can grow to $10B+ per BNEF optimistic scenario."),
        ("Climate impact.",
         "If the VCM delivers real reductions at scale, Paris "
         "Agreement goals become achievable."),
    ])
    add_footer(s, 52, TOTAL, BRAND)

    # Section 8
    s = section_divider(prs, 8, "Worked Example",
        "Evaluating a REDD+ credit batch end-to-end.")
    add_footer(s, 53, TOTAL, BRAND)

    s = _im(prs, "Worked example: REDD+ credit batch evaluation",
        "chart_worked.png",
        caption="50,000 credits claimed → 32,000 integrity-verified. "
                "36% haircut based on cryptographic trust evaluation.",
        image_h=4.4)
    add_footer(s, 54, TOTAL, BRAND)

    s = content_slide(prs, "The scenario", bullets=[
        ("REDD+ project in Brazil state of Para.",
         "100,000 hectares, 3-year vintage (2022-2024)."),
        ("Project claims.",
         "Prevented deforestation of 5,000 hectares per year. "
         "1 hectare prevented = ~10 tCO2e. Total: 50,000 credits "
         "per year, 150,000 total."),
        ("Methodology.",
         "Verra VCS VM0007 (avoided deforestation). Revised 2024 "
         "post-crisis."),
        ("VVB: DNV.",
         "Reputation score 0.84 on REDD+ (from substrate-based "
         "history)."),
        ("Buyer: mid-size corporate.",
         "Net-zero commitment. Looking to retire 150,000 credits."),
    ])
    add_footer(s, 55, TOTAL, BRAND)

    s = content_slide(prs, "The evaluation chain", bullets=[
        ("Step 1: claim verification against NASA MODIS.",
         "Independent satellite data signed by NASA. Historical "
         "deforestation rate in area verified. Result: project area "
         "lost ~1,200 ha/year baseline."),
        ("Step 2: counterfactual modeling.",
         "Peer-reviewed additionality model signed by University of "
         "Oxford team. Expected deforestation without project: "
         "~3,800 ha/year."),
        ("Step 3: observed deforestation during project.",
         "NASA satellite shows ~1,100 ha/year during project. "
         "Reduction: 2,700 ha/year vs counterfactual."),
        ("Step 4: conversion to credits.",
         "2,700 ha × 10 tCO2e = 27,000 tCO2e per year. Total over "
         "3 years: 81,000. Haircut from claim: 46%."),
        ("Step 5: leakage + permanence adjustments.",
         "Further haircut 5-15%. Final: ~68,000 verifiable credits."),
    ])
    add_footer(s, 56, TOTAL, BRAND)

    s = content_slide(prs, "The decision", bullets=[
        ("Buyer's integrity threshold: 0.70.",
         ""),
        ("Project's computed integrity: 0.78.",
         "Above threshold. Credits accepted."),
        ("Credit count accepted: 68,000 (not 150,000).",
         "44% haircut relative to original claim."),
        ("Pricing.",
         "Buyer pays premium ($15-25 per credit vs $3-5 for "
         "cheap REDD+). Total: reasonable cost for verified integrity."),
        ("Public claim.",
         "Buyer can credibly claim 'We retired 68,000 "
         "substrate-verified carbon credits.' Cryptographic "
         "evidence trail available."),
    ])
    add_footer(s, 57, TOTAL, BRAND)

    # Closing
    s = section_divider(prs, 9, "Adoption and Path Forward",
        "What to do. Who moves first.")
    add_footer(s, 58, TOTAL, BRAND)

    s = content_slide(prs, "Who moves first", bullets=[
        ("Major corporate buyers with integrity concerns.",
         "Microsoft, Salesforce, Stripe already require "
         "substantive integrity evidence. Natural early adopters."),
        ("High-quality project developers.",
         "Substrate gives them ability to differentiate. Pay more "
         "to be verifiably distinct from phantom-credit projects."),
        ("Engineered removals.",
         "DAC, biochar, mineralization: measurement-driven. "
         "Substrate is natural fit."),
        ("Nature-based with high integrity.",
         "Well-run forest projects with good science. Substrate "
         "lets them demonstrate quality."),
        ("Sovereign buyers.",
         "Singapore's $10B commitment. Tied to integrity standards. "
         "Natural use case."),
    ])
    add_footer(s, 59, TOTAL, BRAND)

    s = content_slide(prs, "Migration path: 5-year roadmap",
        bullets=[
            ("Year 1.",
             "Pilot with one high-integrity registry (Gold Standard "
             "or Puro.earth). 10-20 projects. Full stack: signed "
             "attestations, VVB reputation."),
            ("Year 2.",
             "Corporate buyer pilots. Major buyer requires substrate "
             "evidence. Creates market pull for project developers."),
            ("Year 3.",
             "Registry adoption expands. Verra, Gold Standard, ACR "
             "all offer substrate-compatible issuance paths."),
            ("Year 4.",
             "Substrate becomes differentiating feature. Projects "
             "without it face discounted prices."),
            ("Year 5.",
             "Substrate is standard for high-integrity credits. "
             "ICVCM/VCMI recognize substrate-verified claims."),
        ])
    add_footer(s, 60, TOTAL, BRAND)

    s = content_slide(prs, "What to do this year (buyer)", bullets=[
        ("Make substrate a procurement requirement.",
         "For credits above $X volume, require cryptographic "
         "attestation chain."),
        ("Work with 1-2 registries willing to pilot.",
         "Gold Standard and Puro.earth most likely partners."),
        ("Invest in internal verification tooling.",
         "Quidnug SDK + trust graph for carbon credit decisions."),
        ("Public commitment to substrate-verified claims.",
         "Raises industry bar. Pressures lower-integrity "
         "competitors."),
        ("Engage ICVCM + VCMI.",
         "Substrate adoption strengthens their standards. "
         "Alignment is available."),
    ])
    add_footer(s, 61, TOTAL, BRAND)

    s = content_slide(prs, "What to do this year (registry)", bullets=[
        ("Pilot substrate integration.",
         "One methodology, 10-20 projects. Demonstrate "
         "operational feasibility."),
        ("Partner with NASA/ESA/academic data providers.",
         "Integrate their signed data feeds. Pre-establishes "
         "independent attestation."),
        ("Build VVB reputation tracking.",
         "Transparent scoring of verifier quality."),
        ("Align with ICVCM + VCMI.",
         "Substrate evidence satisfies integrity requirements."),
        ("Reposition on integrity.",
         "Post-crisis, registry differentiation is integrity. "
         "Substrate enables it."),
    ])
    add_footer(s, 62, TOTAL, BRAND)

    s = content_slide(prs, "What to do this year (project developer)",
        bullets=[
            ("Sign your project data at source.",
             "Satellite data, field observations, analytical "
             "models. Start the chain yourself."),
            ("Invite independent attestation.",
             "Academic partners, NGO monitors. Build multi-party "
             "evidence."),
            ("Choose high-reputation VVB.",
             "Signals quality. Pays off in buyer price."),
            ("Public-facing integrity dashboard.",
             "Show the substrate evidence. Differentiate from "
             "phantom-credit projects."),
            ("Higher-integrity projects can charge more.",
             "Market now rewards this. No longer race to the bottom."),
        ])
    add_footer(s, 63, TOTAL, BRAND)

    s = content_slide(prs, "Honest tradeoffs", bullets=[
        ("Substrate adoption requires coordination.",
         "No single party can deploy. Registry, VVBs, data "
         "providers all needed."),
        ("Smaller projects may struggle initially.",
         "Substrate tooling not yet accessible to every project. "
         "Gradual rollout needed."),
        ("Regulatory uncertainty.",
         "EU, UK, US regulators still finalizing climate claim "
         "requirements. Substrate forward-compatible but not "
         "yet standardized."),
        ("Cost of independent attestation.",
         "NASA/ESA data free. Academic partners willing. But "
         "coordination cost real."),
        ("Not a silver bullet.",
         "Won't fix bad methodologies. Better signal on execution; "
         "methodology design still matters."),
    ])
    add_footer(s, 64, TOTAL, BRAND)

    s = content_slide(prs, "What this does NOT solve", bullets=[
        ("Fundamentally bad methodology.",
         "If a methodology is flawed, substrate signs the flawed "
         "methodology honestly."),
        ("Political conflict over land use.",
         "Forest conservation involves indigenous rights, "
         "sovereignty, land tenure. Substrate cannot arbitrate."),
        ("The fundamental counterfactual problem.",
         "What would have happened without the project? Always "
         "estimation. Substrate can record measurements; "
         "judgment still required."),
        ("Climate impact scale.",
         "Even a perfect VCM is a small fraction of global "
         "emission reduction needed. Offsets are complement, not "
         "substitute."),
        ("Carbon bubble risk.",
         "Aggregate demand for offsets could exceed real supply. "
         "Substrate helps distinguish; doesn't grow supply."),
    ])
    add_footer(s, 65, TOTAL, BRAND)

    s = content_slide(prs, "Summary: the four claims revisited",
        bullets=[
            ("1. The phantom-credit problem is an attestation "
             "architecture problem.",
             "Not methodology tweaking; not regulatory tweaking."),
            ("2. Current verification has structural conflicts of "
             "interest.",
             "Project developer pays verifier. Market incentives "
             "misalign."),
            ("3. Multi-party cryptographic attestation plus "
             "trust-weighted auditors solve both.",
             "Structural, not procedural."),
            ("4. This composes with ICVCM + VCMI, not against them.",
             "Existing standards become mechanically verifiable."),
        ])
    add_footer(s, 66, TOTAL, BRAND)

    s = content_slide(prs, "What success looks like in 2031",
        bullets=[
            ("Substrate-verified credits are market default.",
             "Credits without substrate evidence trade at discount."),
            ("VCM grows to $10B+.",
             "With integrity confidence restored, major buyers "
             "return. Paris Agreement goals achievable."),
            ("Phantom credits drop to <5% of market volume.",
             "Structural detection makes them uneconomic."),
            ("Rating agencies augment substrate.",
             "BeZero, Sylvera, Calyx still valuable; now based "
             "on substrate evidence rather than alternative to it."),
            ("Corporate climate claims become credible.",
             "EU Green Claims Directive + substrate evidence = "
             "credible net-zero commitments."),
        ])
    add_footer(s, 67, TOTAL, BRAND)

    s = content_slide(prs, "References", bullets=[
        ("West et al. 2023.",
         "'Action needed to make carbon offsets from forest "
         "conservation work for climate change mitigation.' "
         "Science 381, 873-877."),
        ("Guardian + Die Zeit + SourceMaterial (January 2023).",
         "REDD+ investigation."),
        ("Cooley et al. 2022.",
         "Cookstove carbon credit analysis."),
        ("Gill-Wiehl et al. 2024.",
         "Cookstove update, persistent over-crediting."),
        ("Thomas et al. 2023.",
         "California forest offset analysis."),
        ("ICVCM Core Carbon Principles.",
         "icvcm.org. 10 principles + methodology assessments."),
        ("VCMI Claims Code.",
         "vcmintegrity.org."),
        ("Ecosystem Marketplace (Forest Trends).",
         "Annual VCM sizing reports."),
    ])
    add_footer(s, 68, TOTAL, BRAND)

    s = content_slide(prs, "More references", bullets=[
        ("EU Green Claims Directive (2024).",
         "europa.eu. Regulatory requirement for substantiated "
         "environmental claims."),
        ("SEC Climate Disclosure Rule (March 2024).",
         "Proposed, partially stayed. Evolving regulatory "
         "framework."),
        ("CDP reporting framework.",
         "cdp.net. Voluntary corporate climate disclosure."),
        ("Science Based Targets initiative (SBTi).",
         "sciencebasedtargets.org. Net-zero target validation."),
        ("Verra VCS standard.",
         "verra.org. Largest voluntary registry."),
        ("Gold Standard.",
         "goldstandard.org."),
        ("Puro.earth.",
         "puro.earth. Engineered removals registry."),
        ("Companion blog post.",
         "blogs/2026-04-27-carbon-credits-are-being-gamed.md."),
    ])
    add_footer(s, 69, TOTAL, BRAND)

    s = content_slide(prs, "Common objections, briefly", bullets=[
        ("'Carbon offsets are inherently problematic.'",
         "Real concern. Substrate doesn't solve; it ensures "
         "offsets claim what they deliver."),
        ("'Voluntary markets will never work.'",
         "Possible. But failing VCM replaced by regulated "
         "markets that face same verification challenges."),
        ("'Substrate is premature.'",
         "Disagree. Crisis is now. Delaying means phantom credits "
         "continue dominating."),
        ("'Too complex for small buyers.'",
         "Complexity mostly hidden behind tooling. Buyer sees "
         "'substrate-verified: yes/no.'"),
        ("'Registries won't cooperate.'",
         "Some will. Market pressure will force others. "
         "Post-crisis, no registry wants to be known for "
         "phantom credits."),
    ])
    add_footer(s, 70, TOTAL, BRAND)

    s = quote_slide(prs,
        "The $900B ESG market's phantom credit problem is not a "
        "methodology problem or a regulatory problem. It is a "
        "trust architecture problem. Architecture can be fixed.",
        "The core thesis",
        title="One-line summary")
    add_footer(s, 71, TOTAL, BRAND)

    s = content_slide(prs, "Next steps", bullets=[
        ("This week. Audit your current offset portfolio integrity.",
         ""),
        ("This month. Read West et al. 2023 + ICVCM CCPs.",
         ""),
        ("This quarter. Pilot substrate-verified credits (small "
         "batch).",
         ""),
        ("This year. Require substrate for major purchases.",
         ""),
        ("Next year. Substrate-verified becomes default.",
         ""),
    ])
    add_footer(s, 72, TOTAL, BRAND)

    s = content_slide(prs, "Things we owe ourselves", bullets=[
        ("Transparent methodology assessment.",
         "ICVCM + industry consortium."),
        ("Open standards for substrate.",
         "Quidnug protocol is open; others should exist too."),
        ("Independent scientific attestation infrastructure.",
         "NASA/ESA already provide. Academic + NGO partnerships "
         "needed."),
        ("Regulator engagement.",
         "EU, UK, US, Singapore. Substrate should align with "
         "evolving regulation."),
        ("Education for the buyer side.",
         "Most corporate sustainability teams underwater on "
         "carbon integrity technicals."),
    ])
    add_footer(s, 73, TOTAL, BRAND)

    s = quote_slide(prs,
        "We do not need better methodologies. We need cryptographic "
        "substrate that makes existing methodologies auditable. The "
        "tools exist. The political will does not.",
        "The call to action",
        title="One-line call to action")
    add_footer(s, 74, TOTAL, BRAND)

    s = closing_slide(prs,
        "Questions",
        subtitle="Thank you. The climate transition needs integrity.",
        cta="Where does the substrate architecture fail in your "
            "use case?\n\nWhich adoption constraint matters most?\n\n"
            "What's your next concrete step?",
        resources=[
            "github.com/quidnug/quidnug",
            "blogs/2026-04-27-carbon-credits-are-being-gamed.md",
            "West et al. 2023 (Science)",
            "ICVCM: icvcm.org",
            "VCMI: vcmintegrity.org",
            "Guardian REDD+ investigation (January 2023)",
            "Ecosystem Marketplace Annual Reports",
        ])
    add_footer(s, 75, TOTAL, BRAND)

    return prs


if __name__ == "__main__":
    prs = build()
    prs.save(str(OUTPUT))
    print(f"wrote {OUTPUT} ({len(prs.slides)} slides)")
