"""Build the Relativistic Ratings deck (~90 slides)."""
import pathlib
import sys

HERE = pathlib.Path(__file__).parent
ASSETS = HERE / "assets"
OUTPUT = HERE / "relativistic-ratings.pptx"

sys.path.insert(0, str(HERE.parent))
from _deck_helpers import (  # noqa: E402
    make_presentation, title_slide, section_divider, content_slide,
    two_col_slide, stat_slide, quote_slide, table_slide, image_slide,
    code_slide, icon_grid_slide, closing_slide,
    add_notes, add_footer,
    TEAL, CORAL, EMERALD, AMBER, TEXT_MUTED,
)
from pptx.util import Inches  # noqa: E402

BRAND = "Quidnug  \u00B7  Relativistic Ratings"
TOTAL = 90


def _im(prs, title, image, image_h=None, **kw):
    if image_h is not None and not hasattr(image_h, 'emu'):
        image_h = Inches(image_h)
    elif image_h is None:
        image_h = Inches(4.6)
    return image_slide(prs, title, image, image_h=image_h,
                       assets_dir=ASSETS, **kw)


def build():
    prs = make_presentation()

    # ==== OPENING (1-5) ====
    s = title_slide(prs,
        "Relativistic Ratings: The End of Review Spam",
        "Why the five-star average is a fiction, what six decades "
        "of social-psychology research say about how trust actually "
        "propagates, and how relativistic ratings solve review spam "
        "as a side effect of getting the math right.",
        eyebrow="QUIDNUG  \u00B7  REPUTATION SYSTEMS")
    add_notes(s, [
        "Welcome. 90 slides, about 60-70 minutes including Q&A.",
        "Audience: product leaders, trust and safety engineers, "
        "reputation system designers. Anyone who has wondered why "
        "every Amazon product is rated 4.4.",
        "Central thesis: the five-star average is a broken "
        "measurement instrument. Relativistic ratings solve review "
        "spam as a structural side effect of better math.",
        "We will spend equal time on the social-science foundations "
        "and the technical implementation. Both matter."
    ])
    add_footer(s, 1, TOTAL, BRAND)

    s = stat_slide(prs, "$540M",
        "annual fake-review economy across major platforms.",
        context="A statistical problem (J-curves) plus an economic "
                "problem ($540M of fraud) plus a UX problem (single "
                "number for a heterogeneous population). The five-star "
                "average is broken. Relativistic ratings fix it.",
        title="The state of online reviews in 2026")
    add_notes(s, [
        "Anchor with the economic scale. He, Hollenbeck, Proserpio "
        "(NBER 2022) measured Amazon at $152M/year alone. Cross-"
        "platform total well over $500M.",
        "This is not theoretical fraud; it is a thriving industry "
        "with WeChat brokers, Facebook groups, and dedicated "
        "websites for buying fake reviews."
    ])
    add_footer(s, 2, TOTAL, BRAND)

    s = content_slide(prs, "Agenda", bullets=[
        ("Section 1. The five stars are lying.",
         "The J-curve, threshold effects, and why averages mislead."),
        ("Section 2. Social science of trust.",
         "Eight foundational studies that prescribe relativistic "
         "ratings."),
        ("Section 3. The review spam economy.",
         "Empirical data on fraud, detection limits, regulatory gaps."),
        ("Section 4. Failure modes of global ratings.",
         "Why current systems fail structurally."),
        ("Section 5. The relativistic rating.",
         "Definition, worked example, structural property."),
        ("Section 6. The four-factor formula.",
         "Math: T, H, A, R. Why multiplicative. Bounded influence."),
        ("Section 7. Anti-spam properties.",
         "Cost-of-attack analysis. Sybils naturally exclude themselves."),
        ("Section 8. Visualization at three depths.",
         "Aurora, constellation, trace. Progressive disclosure."),
        ("Section 9. Real-world examples + tradeoffs.",
         "Yelp, Letterboxd, Healthgrades. When NOT to use this."),
    ])
    add_notes(s, [
        "Walk the arc. Sections 1-4 establish the problem. Sections "
        "5-7 present the solution with proofs. Section 8 covers UX. "
        "Section 9 is honest tradeoffs.",
        "If short on time: Sections 5 (relativistic), 7 (anti-spam), "
        "8 (visualization) are the most valuable."
    ])
    add_footer(s, 3, TOTAL, BRAND)

    s = content_slide(prs, "Four things you will take away", bullets=[
        ("Takeaway 1.",
         "The global-average star rating is a statistically malformed "
         "summary of a self-selecting process."),
        ("Takeaway 2.",
         "Review spam is economically rational under global averages "
         "and economically irrational under relativistic ratings. "
         "The math changes the attacker's cost function."),
        ("Takeaway 3.",
         "Six decades of social psychology agree with the relativistic "
         "model and disagree with the global-average model. We are "
         "not inventing this; we are catching up."),
        ("Takeaway 4.",
         "You can visualize relativistic ratings as efficiently as "
         "stars once you stop fighting the extra dimensions and start "
         "using them."),
    ])
    add_footer(s, 4, TOTAL, BRAND)

    s = content_slide(prs, "Why this talk matters in 2026", bullets=[
        ("FTC's 2024 fake-review rule (16 CFR Part 465) is in force.",
         "Penalties up to $51,744 per violation. But enforcement "
         "lags the offense rate by orders of magnitude."),
        ("AI-generated reviews are flooding platforms.",
         "GPT-class models can produce convincing fake reviews at "
         "near-zero marginal cost. Detection is losing the arms race."),
        ("Consumer trust is collapsing.",
         "Pew Research 2023: 70% of consumers expect fake reviews. "
         "Trustpilot's own transparency reports admit large fraction "
         "of submissions are removed as suspicious."),
        ("The architecture is the only durable fix.",
         "Detection cannot keep pace. Structural defenses can."),
        ("This decade decides whether online reviews remain useful.",
         "If we do not change the substrate, the institution dies "
         "from credibility erosion."),
    ])
    add_footer(s, 5, TOTAL, BRAND)

    # ==== SECTION 1: Five Stars Are Lying (6-13) ====
    s = section_divider(prs, 1, "The Five Stars Are Lying",
        "The J-curve, threshold effects, and why averages mislead.")
    add_footer(s, 6, TOTAL, BRAND)

    s = _im(prs, "Online reviews follow a J-curve, not a bell curve",
        "chart_j_curve.png",
        caption="Source: Hu, Pavlou, and Zhang (2006); Dellarocas "
                "(2003); Luca (2016). Distribution is universal "
                "across platforms.",
        image_h=4.4)
    add_notes(s, [
        "5-star and 1-star reviews dominate. Middle ratings are "
        "rare. This is the universal pattern.",
        "Mean of this distribution is ~4.0-4.5. Median is 5. "
        "Standard deviation is ~1.4. Discrimination power "
        "between products with means 4.4 and 4.7 is essentially "
        "zero."
    ])
    add_footer(s, 7, TOTAL, BRAND)

    s = content_slide(prs, "Hu, Pavlou, Zhang (2006): the foundational study",
        bullets=[
            ("Title.",
             "'Can online reviews reveal a product's true quality? "
             "Empirical findings and analytical modeling.' ACM EC."),
            ("Finding 1.",
             "Online review distributions are J-shaped, dominated by "
             "5-star and (less) 1-star ratings."),
            ("Finding 2.",
             "Reviewers self-select. Extremely satisfied or "
             "dissatisfied customers post; the indifferent middle "
             "majority does not."),
            ("Finding 3.",
             "The mean is computed over a non-representative sample. "
             "Treating it as describing the underlying product "
             "quality is a statistical error."),
            ("Implication.",
             "Every reputation engineer should read this paper. The "
             "framing has been understood for nearly 20 years; only "
             "the engineering has lagged."),
        ])
    add_footer(s, 8, TOTAL, BRAND)

    s = content_slide(prs, "Anderson and Magruder 2012: threshold effects",
        bullets=[
            ("Title.",
             "'Learning from the crowd: regression discontinuity "
             "estimates of the effects of an online review database.' "
             "Economic Journal."),
            ("Method.",
             "Quasi-experimental: examined Yelp's half-star rating "
             "boundaries (3.5 vs 4.0)."),
            ("Finding.",
             "Crossing the 4.0 threshold causes a 19% increase in "
             "the likelihood of a restaurant selling out during peak "
             "hours."),
            ("Implication.",
             "Small statistical noise around threshold creates large "
             "real-world consequences. Disproportionate incentive "
             "for fraud at threshold boundaries."),
            ("Why this matters.",
             "Global ratings have ONE threshold. Relativistic ratings "
             "have a personal threshold per observer. Single-target "
             "fraud becomes ineffective."),
        ])
    add_footer(s, 9, TOTAL, BRAND)

    s = content_slide(prs, "Luca 2016: the revenue impact of stars",
        bullets=[
            ("Michael Luca (Harvard Business School).",
             "'Reviews, Reputation, and Revenue: The Case of "
             "Yelp.com.' HBS Working Paper 12-016."),
            ("Method.",
             "Panel data on Seattle restaurants linked to Washington "
             "State Department of Revenue tax filings."),
            ("Finding.",
             "A one-star Yelp increase causes a 5-9% revenue increase "
             "for independent restaurants. Effect strongest near "
             "rating thresholds."),
            ("No effect on chain restaurants.",
             "Chain reputation comes from brand; Yelp does little. "
             "Independent restaurants live or die by stars."),
            ("Economic motivation for fraud.",
             "If a fake star moves $50k of annual revenue, fraud "
             "broker pricing of $15-30 per fake review is "
             "obviously a winning bet."),
        ])
    add_footer(s, 10, TOTAL, BRAND)

    s = content_slide(prs, "Luca and Zervas 2016: fraud as competitive strategy",
        bullets=[
            ("Title.",
             "'Fake It Till You Make It: Reputation, Competition, "
             "and Yelp Review Fraud.' Management Science."),
            ("Finding.",
             "Yelp flagged ~16% of restaurant reviews as suspicious. "
             "Younger and lower-rated independent restaurants more "
             "likely to commit review fraud."),
            ("Strategic motivation.",
             "Restaurants closer to going out of business have more "
             "to gain and less to lose from fraud."),
            ("Targeted negative reviews.",
             "Reviews targeting competitors increase when a "
             "competitor is performing well, suggesting strategic "
             "use of negative astroturfing."),
            ("Implication.",
             "Fraud is not isolated bad actors; it is a competitive "
             "tactic in markets where rating thresholds drive "
             "revenue."),
        ])
    add_footer(s, 11, TOTAL, BRAND)

    s = content_slide(prs, "Why star averages persist despite the evidence",
        bullets=[
            ("Reason 1: legibility.",
             "A star average fits in 30 characters. A distribution "
             "needs a chart. Personalized rating needs deliberate "
             "design."),
            ("Reason 2: Schema.org and SEO lock-in.",
             "Google rich results accept AggregateRating.ratingValue. "
             "Deviating loses search visibility."),
            ("Reason 3: vendor lock-in.",
             "Platforms control distribution and display. Opacity "
             "benefits incumbents."),
            ("Reason 4: tradition.",
             "20+ years of habit. Hard to change a UI norm."),
            ("None of these are good reasons.",
             "Each is a real constraint, but none invalidates the "
             "case for better math underneath. We can serve both "
             "Schema.org and humans simultaneously."),
        ])
    add_footer(s, 12, TOTAL, BRAND)

    s = quote_slide(prs,
        "Treating a self-selected J-shaped distribution's mean as "
        "if it described the underlying population is a statistical "
        "error we have collectively normalized.",
        "The framing for the rest of this talk",
        title="The framing")
    add_footer(s, 13, TOTAL, BRAND)

    # ==== SECTION 2: Social Science (14-26) ====
    s = section_divider(prs, 2, "What Social Science Says About Trust",
        "Eight foundational studies. Six decades. One conclusion.")
    add_footer(s, 14, TOTAL, BRAND)

    s = _im(prs, "Six decades of social science point in one direction",
        "chart_social_timeline.png",
        caption="Each study, in its own way, prescribes "
                "context-dependent, observer-relational trust.",
        image_h=4.4)
    add_footer(s, 15, TOTAL, BRAND)

    s = _im(prs, "Asch 1951, 1956: conformity to a unanimous wrong group",
        "chart_asch.png",
        caption="Solomon Asch's classic line-judgment experiments. "
                "Conformity rises sharply with unanimous group "
                "pressure.",
        image_h=4.2)
    add_notes(s, [
        "Asch's experiments showed people will agree with a group "
        "even when the group is obviously wrong, just to fit in.",
        "Implication for ratings: a 4.8-star aggregate is not just "
        "information; it is social pressure. The relativistic model "
        "moves social proof from anonymous crowd to people the "
        "observer trusts."
    ])
    add_footer(s, 16, TOTAL, BRAND)

    s = content_slide(prs, "Festinger 1954: social comparison theory",
        bullets=[
            ("Leon Festinger.",
             "'A theory of social comparison processes,' Human "
             "Relations 7(2). 1954."),
            ("Core claim.",
             "Humans evaluate themselves and their opinions by "
             "comparing to similar others. Not anonymous crowds. "
             "Similar others."),
            ("Implication for ratings.",
             "A global average is the wrong reference class. You "
             "want to compare opinions against people similar to you."),
            ("Relativistic model encodes 'similar to you' "
             "operationally.",
             "A trust graph IS a formalization of whose opinions "
             "you weight."),
            ("This is not a small distinction.",
             "Aggregating opinions from people unlike you produces "
             "noise. Aggregating from people like you produces "
             "signal."),
        ])
    add_footer(s, 17, TOTAL, BRAND)

    s = _im(prs, "Granovetter 1973: the strength of weak ties",
        "chart_weak_ties.png",
        caption="Most jobs are found through acquaintances, not "
                "family or close friends. Novel info flows through "
                "weak ties.",
        image_h=4.4)
    add_notes(s, [
        "Granovetter's most-cited paper in modern sociology.",
        "Implication for ratings: trust decay should be graceful, "
        "not binary. The friend-of-a-friend you barely know might "
        "give you the most useful review.",
        "Multiplicative decay through trust paths is a direct "
        "operationalization of this finding."
    ])
    add_footer(s, 18, TOTAL, BRAND)

    s = content_slide(prs, "Milgram 1963 + Facebook 2016: small-world data",
        bullets=[
            ("Stanley Milgram's 1967 'small world' experiment.",
             "Letters from random Americans to a target via "
             "social acquaintances. Average path length: ~6."),
            ("Bhagat et al. 2016 (Facebook research).",
             "On Facebook's social graph, average distance between "
             "any two users: 3.57 degrees of separation."),
            ("Implication.",
             "The distance from any observer to any reviewer is "
             "small. Trust-graph walks of depth 5 are not "
             "theoretical; they are empirically sufficient for "
             "almost any pair."),
            ("Also implication for Sybil resistance.",
             "Inserting a node BETWEEN two honest parties is hard "
             "when the social graph is dense. Sixth degrees of "
             "separation provides structural defense."),
            ("Relevance.",
             "Quidnug's default trust-walk depth is 5. Backed by "
             "empirical small-world research."),
        ])
    add_footer(s, 19, TOTAL, BRAND)

    s = content_slide(prs,
        "Mayer, Davis, Schoorman 1995: integrative trust model",
        bullets=[
            ("Mayer/Davis/Schoorman.",
             "'An Integrative Model of Organizational Trust.' "
             "Academy of Management Review 20(3). The most-cited "
             "framework in trust literature."),
            ("Three components of trustworthiness.",
             "Ability (competence), Benevolence (intent), "
             "Integrity (principle adherence)."),
            ("Plus a personality variable.",
             "Propensity to trust: how willing the trustor is to "
             "trust at all."),
            ("Implication for ratings.",
             "Trustworthy reviewer is not one dimension. A competent "
             "reviewer with unknown intentions is weighted "
             "differently than a less competent one with strong "
             "honest history."),
            ("Maps directly to Quidnug's four-factor formula.",
             "T = ability-plus-domain. H = benevolence + integrity "
             "(via helpfulness votes). A = demonstrated competence. "
             "R = ongoing commitment."),
        ])
    add_footer(s, 20, TOTAL, BRAND)

    s = content_slide(prs, "Fukuyama 1995 + Putnam 2000: trust as social capital",
        bullets=[
            ("Francis Fukuyama: Trust: The Social Virtues and the "
             "Creation of Prosperity (1995).",
             "Argues generalized trust is an economic good. Lower "
             "transaction costs in high-trust societies."),
            ("Robert Putnam: Bowling Alone (2000).",
             "Argues declining social trust tracks declining civic "
             "engagement."),
            ("Both works also argue trust is BOUNDED by community.",
             "Cultural context determines baseline propensity to "
             "trust. Japan and US high; Italy and Latin America "
             "lower."),
            ("Implication for ratings.",
             "A rating system that assumes global trust homogeneity "
             "underweights reviews from high-trust populations and "
             "overweights from low-trust. Per-observer relativistic "
             "models avoid this."),
            ("Trust is not universal.",
             "Encoding it as if it were is a category error."),
        ])
    add_footer(s, 21, TOTAL, BRAND)

    s = content_slide(prs, "Cialdini 1984: Influence (social proof)",
        bullets=[
            ("Robert Cialdini.",
             "Influence: The Psychology of Persuasion (1984). "
             "Operationalizes Asch for marketers."),
            ("Six principles of influence.",
             "Reciprocity, Commitment, Social Proof, Authority, "
             "Liking, Scarcity."),
            ("Social proof is the most-cited principle for review "
             "design.",
             "'Lots of people like this' is presented as evidence "
             "the thing is good."),
            ("But Cialdini also notes a critical caveat.",
             "Social proof is more powerful when the proof comes "
             "from people LIKE the target. Same finding as Festinger."),
            ("Existing review systems systematically violate "
             "Cialdini's caveat.",
             "Generic crowd-average is exactly the WRONG kind of "
             "social proof for an individual decision."),
        ])
    add_footer(s, 22, TOTAL, BRAND)

    s = content_slide(prs, "Resnick et al. 2000: Reputation Systems (CACM)",
        bullets=[
            ("Resnick, Kuwabara, Zeckhauser, Friedman.",
             "'Reputation Systems.' Communications of the ACM "
             "43(12). The foundational engineering paper."),
            ("Three roles a reputation system must serve.",
             "Provide information for decision-making. Give feedback "
             "to improve reputation. Deter bad behavior."),
            ("Four canonical failure modes identified.",
             "Entry/exit attacks. Sybil attacks. Whitewashing. "
             "Collusion."),
            ("Implication for design.",
             "Any system without structural Sybil resistance is "
             "broken. Detection-based is a losing arms race."),
            ("Quidnug PoT addresses each.",
             "Sybil: structural via trust graph. Whitewashing: TTL "
             "+ history visibility. Collusion: bounded influence "
             "from each reviewer's weight."),
        ])
    add_footer(s, 23, TOTAL, BRAND)

    s = content_slide(prs, "Jøsang, Ismail, Boyd 2007: trust survey",
        bullets=[
            ("Audun Jøsang.",
             "'A Survey of Trust and Reputation Systems for Online "
             "Service Provision.' Decision Support Systems 43(2). "
             "Reviewed 40+ systems across the prior decade."),
            ("Conclusion 1.",
             "Transitive trust with graded decay is the most general "
             "and expressive model."),
            ("Conclusion 2.",
             "Binary trust loses information. Numerical trust "
             "without decay handles uncertainty poorly."),
            ("Conclusion 3.",
             "Context matters. A reputation system must be scoped to "
             "the context of judgment. Good landscape photographer "
             "is not necessarily a good portrait photographer."),
            ("Quidnug operationalizes all three.",
             "Multiplicative decay; topic-domain scoping; per-context "
             "TRUST tx with TrustDomain field."),
        ])
    add_footer(s, 24, TOTAL, BRAND)

    s = table_slide(prs, "Eight studies, eight implications", [
        ["Finding", "Source", "Implication for ratings"],
        ["Conformity to wrong group: 32%", "Asch 1951, 1956",
         "Show TRUSTED social proof, not crowd"],
        ["Compare to similar others", "Festinger 1954",
         "Global avg is wrong reference class"],
        ["Weak ties carry novel info", "Granovetter 1973",
         "Multi-hop trust with decay"],
        ["~3.6 degrees on Facebook", "Bhagat et al. 2016",
         "Depth-5 walks reach everyone"],
        ["Trust = Ability+Benevolence+Integrity",
         "Mayer/Davis/Schoorman 1995",
         "Multi-factor rating with independent signals"],
        ["Trust bounded by community",
         "Fukuyama 1995 / Putnam 2000",
         "Global flat trust assumption is wrong"],
        ["Sybil resistance is mandatory",
         "Resnick et al. 2000",
         "Structural, not detection-based"],
        ["Transitive trust + context is right",
         "Jøsang 2007",
         "Domain scoping + graded decay"],
    ], col_widths=[2.5, 2.0, 2.5], body_size=10)
    add_footer(s, 25, TOTAL, BRAND)

    s = quote_slide(prs,
        "Every social-science result points to the same answer: "
        "ratings should be computed from the observer's own "
        "context-aware weighted view of the people whose opinions "
        "they trust.",
        "The framework prescribed by 70 years of research",
        title="What social science prescribes")
    add_footer(s, 26, TOTAL, BRAND)

    # ==== SECTION 3: Review Spam Economy (27-34) ====
    s = section_divider(prs, 3, "The Review Spam Economy",
        "Empirical scale, detection limits, and why regulation is "
        "necessary but insufficient.")
    add_footer(s, 27, TOTAL, BRAND)

    s = _im(prs, "The fake-review economy by platform",
        "chart_fraud_market.png",
        caption="Source: He/Hollenbeck/Proserpio 2022 (NBER W29855); "
                "Trustpilot 2023 transparency; industry estimates.",
        image_h=4.4)
    add_notes(s, [
        "$540M is the cross-platform total. Real number likely "
        "higher because measurement is hard.",
        "Compare to industry alternatives: this is more than the "
        "annual revenue of Yelp (~$420M in 2024)."
    ])
    add_footer(s, 28, TOTAL, BRAND)

    s = content_slide(prs,
        "He, Hollenbeck, Proserpio 2022: NBER W29855", bullets=[
            ("Authors.",
             "Sherry He, Brett Hollenbeck (UCLA), Davide Proserpio "
             "(USC). NBER Working Paper 29855, 2022."),
            ("Method.",
             "Identified the actual market for fake reviews via "
             "private Facebook groups, WeChat channels, and dedicated "
             "broker websites."),
            ("Pricing.",
             "Sellers paid $0-30 per fake review, often with the "
             "product fully reimbursed as part of the deal."),
            ("Effects.",
             "12.5% short-term rating increase from fraud spike. "
             "But longer-term: ratings declined as genuine buyers "
             "received lower-quality goods than fake reviews "
             "promised."),
            ("Detection coverage.",
             "Amazon's automated systems caught some but not all. "
             "Detected sellers faced higher cost going forward; "
             "successful evaders persisted."),
        ])
    add_footer(s, 29, TOTAL, BRAND)

    s = _im(prs, "The detection ceiling: even 94% leaves millions of fakes",
        "chart_detection.png",
        caption="At 100M reviews per platform, even 94% accuracy "
                "leaves 6M undetected fakes plus 6M false positives.",
        image_h=4.0)
    add_notes(s, [
        "Detection-based approaches are fundamentally an arms race. "
        "Each defender improvement is followed by attacker "
        "adaptation.",
        "Structural defenses (making fakes useless to the observer "
        "regardless of detection) scale. Probabilistic defenses do "
        "not."
    ])
    add_footer(s, 30, TOTAL, BRAND)

    s = content_slide(prs, "Mayzlin et al. 2014: astroturfing as strategy",
        bullets=[
            ("Mayzlin, Dover, Chevalier.",
             "'Promotional Reviews: An Empirical Investigation of "
             "Online Review Manipulation.' American Economic Review "
             "104(8)."),
            ("Method.",
             "Compared reviews on Expedia (verified-purchase only) "
             "with TripAdvisor (open to all)."),
            ("Finding 1.",
             "Competitor-proximate listings on TripAdvisor had "
             "systematically lower ratings than on Expedia."),
            ("Finding 2.",
             "Gap was largest for small independent hotels near "
             "large chain hotels. Suggests targeted negative "
             "astroturfing from chains."),
            ("Statistical significance: p < 0.001.",
             "Not noise; documented competitive tactic."),
        ])
    add_footer(s, 31, TOTAL, BRAND)

    s = content_slide(prs, "Review bombing (the 2018-2024 era)",
        bullets=[
            ("Coordinated negative campaigns to tank a product's "
             "rating for cultural or political reasons.",
             ""),
            ("Notable cases.",
             "Captain Marvel (2019), The Last of Us Part II (2020), "
             "She-Hulk (2022), several Kindle/audiobook campaigns."),
            ("Mechanism.",
             "Organized in public spaces (Twitter, Reddit, 4chan); "
             "campaigns deliberately target rating thresholds."),
            ("Platform response.",
             "Reactive at best. By the time bombing is detected, the "
             "rating damage is persistent."),
            ("Why relativistic ratings defeat this.",
             "Bombers are strangers to the observer's trust graph. "
             "They contribute zero weight. Coordinated mass review "
             "is structurally invisible."),
        ])
    add_footer(s, 32, TOTAL, BRAND)

    s = content_slide(prs, "FTC 2024: 16 CFR Part 465", bullets=[
        ("Trade Regulation Rule on the Use of Consumer Reviews and "
         "Testimonials.",
         "Finalized August 2024. Penalties up to $51,744 per "
         "violation."),
        ("Prohibits.",
         "Buying fake reviews. Selling fake reviews. Suppressing "
         "negative reviews. Misleading testimonial use."),
        ("Why this is necessary.",
         "Provides legal hook for enforcement. Signals industry "
         "consensus."),
        ("Why this is insufficient.",
         "US jurisdiction. Most fake-review brokers operate from "
         "China, SE Asia, Eastern Europe. Per-violation fines "
         "assume identification. Enforcement is slow."),
        ("Even with full enforcement: the statistical problem "
         "remains.",
         "J-curves and global averages mislead even without fraud. "
         "Regulation patches the worst symptoms; structural fix "
         "addresses root cause."),
    ])
    add_footer(s, 33, TOTAL, BRAND)

    s = quote_slide(prs,
        "Detection-based fraud prevention is a losing arms race. "
        "Structural anti-spam (making fakes useless to the observer) "
        "is the only durable strategy.",
        "Why we built Quidnug PoT this way",
        title="Detection vs structure")
    add_footer(s, 34, TOTAL, BRAND)

    # ==== SECTION 4: Failure Modes (35-41) ====
    s = section_divider(prs, 4, "Failure Modes of Global Ratings",
        "Why current systems fail, structurally.")
    add_footer(s, 35, TOTAL, BRAND)

    s = content_slide(prs, "Failure 1: selection bias", bullets=[
        ("Reviewers are not representative.",
         "Extremely happy and extremely unhappy customers review. "
         "The indifferent middle does not."),
        ("Result: the J-curve.",
         "Distributions are bimodal regardless of underlying product "
         "quality."),
        ("The mean of a J is not the mean of the population.",
         "Stat 101: a non-representative sample's statistics do not "
         "describe the population."),
        ("This is a fundamental problem with global rating "
         "aggregation.",
         "It is not solvable by adding more reviews; the bias is in "
         "the WHO, not the HOW MANY."),
        ("Relativistic model dodges this.",
         "Each observer weights reviewers individually; the J-curve "
         "is replaced with the observer's specific cohort."),
    ])
    add_footer(s, 36, TOTAL, BRAND)

    s = content_slide(prs, "Failure 2: threshold effects", bullets=[
        ("Anderson and Magruder 2012 showed crossing 4.0 Yelp causes "
         "19% peak-sellout effect.",
         ""),
        ("Single global threshold creates concentrated incentive for "
         "fraud.",
         "Why bother attacking widely-distributed competitors when "
         "you can buy 5 fake reviews to push your rating from 3.9 "
         "to 4.0?"),
        ("Relativistic ratings have NO single threshold.",
         "Each observer has their own threshold based on their "
         "personalized rating."),
        ("Fraud around a single numeric target becomes "
         "ineffective.",
         "There is no single number to target."),
        ("This is the structural reason Quidnug is anti-fragile to "
         "fraud.",
         "Not detection-based; the math itself defeats the attack."),
    ])
    add_footer(s, 37, TOTAL, BRAND)

    s = content_slide(prs, "Failure 3: astroturfing", bullets=[
        ("Documented by Mayzlin, Dover, Chevalier 2014.",
         "Astroturfing is a competitive tactic, not isolated bad "
         "actors."),
        ("Why it works against global ratings.",
         "Fake reviewer accounts are created cheaply. Each "
         "contributes to the global average. Quantity scales the "
         "attack."),
        ("Why it fails against relativistic ratings.",
         "A new fake reviewer has zero trust in any observer's "
         "graph until SOMEONE the observer trusts vouches for them."),
        ("Sybil reviewer cluster contributes zero observable rating.",
         "Visibility requires being trusted. Trust requires being "
         "vouched for. Vouching requires existing reputation."),
        ("Astroturfing remains POSSIBLE but requires social "
         "engineering.",
         "Cost is much higher than buying fake reviews from a "
         "broker."),
    ])
    add_footer(s, 38, TOTAL, BRAND)

    s = content_slide(prs, "Failure 4: review bombing", bullets=[
        ("Coordinated negative campaigns are organized and visible.",
         "Twitter, Reddit, 4chan threads. Yet platform response "
         "remains reactive."),
        ("Why platforms struggle.",
         "Bombing campaigns produce technically-legitimate user "
         "reviews. Detection-based filtering struggles to "
         "distinguish them from organic backlash."),
        ("Why relativistic ratings defeat them.",
         "Bombers are typically strangers to the observer's trust "
         "graph. Their reviews contribute zero weight."),
        ("Test case.",
         "Captain Marvel 2019 bombing: would not have moved any "
         "individual viewer's PoT-rating because the bombers were "
         "outside any normal viewer's social trust graph."),
        ("Defense by construction.",
         "No moderation needed. The math handles it."),
    ])
    add_footer(s, 39, TOTAL, BRAND)

    s = content_slide(prs, "Failure 5: filter bubbles (the legitimate concern)",
        bullets=[
            ("Pariser 2011: The Filter Bubble.",
             "Worry that personalization isolates people in "
             "information silos."),
            ("Sunstein 2001: Republic.com.",
             "Same concern from political-philosophy angle."),
            ("Both concerns are legitimate.",
             "Personalized aggregation can cocoon people from "
             "challenging viewpoints."),
            ("Two responses.",
             "(1) Global ratings already filter-bubble you in the "
             "WORST way: they include reviewers whose opinions don't "
             "apply to you. (2) Quidnug's constellation view shows "
             "the crowd alongside personalized."),
            ("Filter bubbles are problems when filters are opaque.",
             "Relativistic ratings make the filter transparent and "
             "user-controllable."),
        ])
    add_footer(s, 40, TOTAL, BRAND)

    s = content_slide(prs,
        "Failure 6: the 4.4 equivalence class", bullets=[
            ("When every product is rated 4.0-4.7, the rating's "
             "discrimination power is zero.",
             ""),
            ("Analogy.",
             "If a thermometer only reports 70-72 degrees, you "
             "cannot use it to choose when to wear a coat. Useless."),
            ("This is what star averages have become for products in "
             "competitive categories.",
             "Amazon laptops, Yelp restaurants, Google hotels: nearly "
             "everything is 4.2-4.6."),
            ("The compression is partly natural (J-curve), partly "
             "fraud-driven, partly platform-enforced.",
             ""),
            ("Relativistic ratings restore discrimination.",
             "When observer-specific weights replace global mean, "
             "products separate cleanly: some get 4.8 from your "
             "trusted reviewers, others get 3.1."),
        ])
    add_footer(s, 41, TOTAL, BRAND)

    # ==== SECTION 5: The Relativistic Rating (42-50) ====
    s = section_divider(prs, 5, "The Relativistic Rating",
        "Definition, worked example, and the structural property.")
    add_footer(s, 42, TOTAL, BRAND)

    s = content_slide(prs, "Definition", bullets=[
        ("A relativistic rating is a function of (observer, product, "
         "topic).",
         "Returns a scalar in [0, 5] (or whatever rating range "
         "applies)."),
        ("Computed as the weighted average of individual reviews.",
         "Effective_rating = sum(r.rating * w(r)) / sum(w(r)) "
         "across all reviews with positive weight."),
        ("Weight w(r) is a product of four factors.",
         "T (topical trust) * H (helpfulness) * A (activity) * "
         "R (recency)."),
        ("Each factor reflects a specific signal from a specific "
         "social-science finding.",
         "Multiplicative composition (we will defend in Section 6)."),
        ("Same reviews, different observer, different rating.",
         "Both correct."),
    ])
    add_footer(s, 43, TOTAL, BRAND)

    s = code_slide(prs,
        "Formal definition",
        [
            "// Inputs:",
            "//   observer: quid issuing the query",
            "//   product:  product / item being rated",
            "//   topic:    domain (e.g. reviews.public.tech.laptops)",
            "//   reviews:  list of REVIEW transactions on the product",
            "",
            "for each review r in reviews:",
            "    w(r) = T(observer, r.reviewer, topic)",
            "         * H(observer, r.reviewer, topic)",
            "         * A(r.reviewer, topic)",
            "         * R(r.timestamp)",
            "",
            "RR(observer, product, topic) =",
            "    sum(r.rating * w(r) for r in reviews) /",
            "    sum(w(r) for r in reviews)",
        ])
    add_footer(s, 44, TOTAL, BRAND)

    s = _im(prs, "Worked example: same reviews, two observers, two ratings",
        "chart_two_observers.png",
        caption="Same product, same four reviews. Jamie sees 4.5; "
                "Pat sees 2.0. Both correct.",
        image_h=4.4)
    add_notes(s, [
        "Walk the example. Four reviewers: Alice, Bob, Carol, Dave. "
        "Same ratings.",
        "Jamie's trust graph weights Alice (laptop expert) and "
        "Carol (tech reviewer) heavily; Bob and Dave near zero. "
        "Result: 4.5.",
        "Pat's trust graph only contains Bob (Pat's brother). "
        "Other three: zero weight. Result: 2.0.",
        "Both views are honest. Both are derived from the same "
        "chain data. Forcing one of them on the other is data loss."
    ])
    add_footer(s, 45, TOTAL, BRAND)

    s = content_slide(prs, "Why this is correct, not a bug", bullets=[
        ("Observer A weights expertise; Observer B weights direct "
         "trust.",
         "Both reflect rational and consistent priors."),
        ("Information about which observer believes what is "
         "preserved.",
         "Crowd average destroys this; relativistic preserves it."),
        ("Users can adjust their trust graph to broaden or narrow "
         "their view.",
         "Pat could add trust edges to laptop experts and see Jamie's "
         "rating gradually merge into theirs."),
        ("Transparency.",
         "Quidnug exposes the underlying contributors via the "
         "constellation visualization. Users see WHO is being "
         "weighted in their rating."),
        ("Compare to opaque algorithmic personalization.",
         "Relativistic ratings make the computation legible. The "
         "filter is the user's own choice."),
    ])
    add_footer(s, 46, TOTAL, BRAND)

    s = content_slide(prs, "The structural property this enables",
        bullets=[
            ("In a relativistic rating, an attacker who wants to move "
             "the rating must enter the observer's trust graph.",
             "That is not a matter of posting more reviews; it is a "
             "matter of getting the observer (or someone they trust) "
             "to vouch for the attacker."),
            ("In a global-average rating, an attacker only needs to "
             "post more reviews faster than detection.",
             "The observer has no say."),
            ("This difference fundamentally changes the attacker's "
             "cost function.",
             "Quantity attacks become ineffective. Quality attacks "
             "(socially-engineered trust) are slow and don't scale."),
            ("Spam is solved as a structural side effect of getting "
             "the math right.",
             "Not a separate moderation system. Not a detection "
             "pipeline. Math."),
        ])
    add_footer(s, 47, TOTAL, BRAND)

    s = content_slide(prs, "What changes for the attacker", bullets=[
        ("Global rating attacker.",
         "Buy 50 fake reviews from a broker for ~$500. Move the "
         "average from 3.9 to 4.0. Done."),
        ("Relativistic rating attacker.",
         "Sybil cluster has zero trust in any observer's graph. "
         "Need to compromise an observer's trust graph somehow."),
        ("Compromise via fake account.",
         "Build up apparent reputation over months. Get vouched for "
         "by an existing trusted reviewer. Slow and expensive."),
        ("Compromise via stolen account.",
         "Hijack a real reviewer with established trust. Use their "
         "voice. Time-limited until detection / revocation."),
        ("Compromise via social engineering.",
         "Convince observers to add trust edges to a fake. Requires "
         "real social interaction."),
    ])
    add_footer(s, 48, TOTAL, BRAND)

    s = _im(prs, "Cost-of-attack: relativistic ratings change the math",
        "chart_attack_cost.png",
        caption="Sybil floods are cheap; observer-specific attack is "
                "expensive. The asymmetry is the defense.",
        image_h=4.2)
    add_footer(s, 49, TOTAL, BRAND)

    s = quote_slide(prs,
        "Spam is solved as a structural side effect of getting the "
        "math right. Not a separate moderation system. Not a "
        "detection pipeline. Math.",
        "The spam-prevention thesis",
        title="The thesis on spam")
    add_footer(s, 50, TOTAL, BRAND)

    # ==== SECTION 6: Four-Factor Math (51-60) ====
    s = section_divider(prs, 6, "The Four-Factor Math",
        "T, H, A, R. Why each. Why multiplicative.")
    add_footer(s, 51, TOTAL, BRAND)

    s = _im(prs, "The four factors at a glance",
        "chart_four_factors.png",
        caption="w(reviewer) = T \u00D7 H \u00D7 A \u00D7 R. "
                "Effective rating = weighted average.",
        image_h=4.0)
    add_footer(s, 52, TOTAL, BRAND)

    s = content_slide(prs, "Factor T: topical transitive trust",
        bullets=[
            ("Definition.",
             "T(observer, reviewer, topic) = max-path trust from "
             "observer to reviewer in the specified topic domain."),
            ("Topic scoping.",
             "A reviewer trusted for laptops is not automatically "
             "trusted for restaurants. Each topic has its own trust "
             "subgraph."),
            ("Topic inheritance with decay.",
             "If no direct edge in the requested topic, fall back to "
             "parent domain with 0.8 decay per step up."),
            ("Why the decay.",
             "Cross-topic trust is real but weaker. Operationalizes "
             "Jøsang 2007's context principle."),
            ("Cap at depth 5.",
             "Empirically sufficient (Milgram + Facebook small-world "
             "data). Bounded computation cost."),
        ])
    add_footer(s, 53, TOTAL, BRAND)

    s = content_slide(prs, "Factor H: helpfulness reputation",
        bullets=[
            ("Definition.",
             "H(observer, reviewer, topic) = trust-weighted ratio of "
             "helpful : (helpful + unhelpful) votes the reviewer has "
             "received in the topic."),
            ("Crucially: voters are weighted by the OBSERVER's trust "
             "in them.",
             "So 'helpfulness' is itself relativistic."),
            ("Observers see different helpfulness scores for the "
             "same reviewer.",
             "If your trust graph weights people who upvote a "
             "reviewer, that reviewer is helpful TO YOU."),
            ("Recursive cap.",
             "Trust lookup inside H is capped at depth 3 (vs depth 5 "
             "for outer T). Prevents complexity blowup."),
            ("Default H = 0.5 with no votes.",
             "Neutral. Observer's other factors still apply."),
        ])
    add_footer(s, 54, TOTAL, BRAND)

    s = _im(prs, "Factor A: log-scaled activity",
        "chart_activity.png",
        caption="Activity factor caps at 50 reviews; first-time "
                "reviewers not penalized to zero.",
        image_h=4.2)
    add_notes(s, [
        "A(reviewer, topic) = clip(log(reviews_in_topic) / log(50), "
        "0, 1).",
        "Rewards consistency; first-timers still get weight from T "
        "and H. Past 50 reviews, additional activity does not "
        "increase weight (prevents 'log everything to farm weight').",
        "Resnick et al. 2000's 'reward demonstrated behavior' "
        "principle made operational."
    ])
    add_footer(s, 55, TOTAL, BRAND)

    s = _im(prs, "Factor R: exponential recency decay",
        "chart_recency.png",
        caption="2-year half-life. Floor at 0.3 so old reviews never "
                "go to zero.",
        image_h=4.2)
    add_notes(s, [
        "R(timestamp) = max(0.3, exp(-age_days / 730)).",
        "Products age; tastes change. A 10-year-old review carries "
        "less signal but is not worthless. The 0.3 floor preserves "
        "long-tail value.",
        "730-day half-life is operator-tunable per domain."
    ])
    add_footer(s, 56, TOTAL, BRAND)

    s = content_slide(prs, "Why multiplicative, not additive", bullets=[
        ("Multiplication: any factor near zero pulls the product "
         "near zero.",
         "A reviewer the observer has no trust path to (T=0) "
         "should be EXCLUDED, not just downweighted."),
        ("All four factors must be present for the reviewer to "
         "matter.",
         "A reviewer with zero history (A=0) should not get full "
         "credit even if T+H+R are strong."),
        ("Additive composition would let one factor mask another.",
         "Not what we want. A 99%-trusted but never-active "
         "reviewer should not get heavy weight."),
        ("Multiplication is commutative, associative, and "
         "monotonic.",
         "Order of evaluation doesn't matter. Mathematically clean."),
        ("This is the same intuition as 'AND' versus 'OR' in "
         "Boolean logic.",
         "We require all four signals to be present. Multiplication "
         "is the continuous analog of AND."),
    ])
    add_footer(s, 57, TOTAL, BRAND)

    s = content_slide(prs, "Property: bounded influence per reviewer",
        bullets=[
            ("Claim.",
             "For any fixed (observer, topic), no single reviewer's "
             "contribution to the rating exceeds w(r) / sum(w(r))."),
            ("Proof.",
             "RR is a weighted average. The partial derivative with "
             "respect to one reviewer's rating is w(r) / sum(w(r)) "
             "<= 1."),
            ("Consequence.",
             "An attacker compromising one reviewer can move the "
             "rating by at most that reviewer's relative weight "
             "share."),
            ("Contrast with global averages.",
             "A single fake review with viral helpfulness campaign "
             "can disproportionately move a small product's rating. "
             "Relativistic ratings cap the damage."),
            ("This is a structural anti-spam property.",
             "Not a moderation policy. The math itself bounds "
             "attacker damage."),
        ])
    add_footer(s, 58, TOTAL, BRAND)

    s = content_slide(prs, "Topic inheritance: cross-domain trust with decay",
        bullets=[
            ("Sometimes there is no direct trust path in the exact "
             "topic.",
             "Observer wants laptops review; trust graph only has "
             "general-tech edges to the reviewer."),
            ("Fallback chain.",
             "Try reviews.public.tech.laptops first. If empty, try "
             "reviews.public.tech with 0.8 decay. If still empty, "
             "try reviews.public with 0.64 decay. Else T=0."),
            ("Why 0.8 per step.",
             "Empirical: cross-topic trust has demonstrated value "
             "but is weaker than topic-specific. 0.8 is a reasonable "
             "default; operator-tunable."),
            ("Prevents over-narrowing.",
             "Without inheritance, niche topics with sparse direct "
             "edges would fall back to crowd default too easily."),
            ("Mathematically.",
             "Decay is multiplicative; preserves monotonic-decay "
             "property."),
        ])
    add_footer(s, 59, TOTAL, BRAND)

    s = quote_slide(prs,
        "Each factor encodes a specific social-science finding. "
        "Multiplicative composition reflects 'AND' semantics. "
        "The result is a defensible computation, not a magic number.",
        "The four-factor formula in one sentence",
        title="Summary")
    add_footer(s, 60, TOTAL, BRAND)

    # ==== SECTION 7: Anti-Spam Properties (61-67) ====
    s = section_divider(prs, 7, "Anti-Spam Properties",
        "Cost-of-attack analysis. Why bots naturally exclude themselves.")
    add_footer(s, 61, TOTAL, BRAND)

    s = content_slide(prs, "Sybil cost analysis", bullets=[
        ("Cost to create a fake reviewer account.",
         "Effectively zero. ECDSA keypair generation is "
         "microseconds. We are NOT trying to make identity "
         "creation expensive."),
        ("Cost to give a fake reviewer ANY influence.",
         "Dramatically higher. Must enter at least one observer's "
         "trust graph."),
        ("Pathway 1: compromise observer's trust edges directly.",
         "Get the observer to publish a TRUST tx toward the fake. "
         "Social-engineering cost."),
        ("Pathway 2: compromise someone the observer trusts.",
         "Walk up the graph. Compromise cost multiplies, decayed "
         "by trust weight at each hop."),
        ("Quantity does not help.",
         "10,000 Sybils contributing zero weight is the same as "
         "one Sybil contributing zero weight."),
    ])
    add_footer(s, 62, TOTAL, BRAND)

    s = table_slide(prs, "Per-attack cost asymmetry", [
        ["Attack", "Global rating", "Relativistic"],
        ["Move global average 0.5 stars",
         "$280 - $1900 (broker)",
         "Infinity (attack does not exist)"],
        ["Move one observer's view 0.5 stars",
         "Same as above", "$5k - $30k (compromise trust path)"],
        ["Move 100 observers' views",
         "Same as above (one attack hits all)",
         "100x cost (independent compromises)"],
        ["Influence aggregate via Sybil flood",
         "Cheap, scales linearly",
         "Useless (zero weight from each)"],
        ["Coordinated review bombing",
         "Effective; persistent damage",
         "Zero effect (bombers outside trust graphs)"],
    ], col_widths=[2.5, 2.0, 2.5], body_size=11)
    add_footer(s, 63, TOTAL, BRAND)

    s = content_slide(prs, "Why bots naturally exclude themselves", bullets=[
        ("Under relativistic ratings, a bot that posts 1000 reviews "
         "contributes the same as a bot that posts 0 reviews.",
         "Both contribute zero weight to any observer's computation."),
        ("This is a structural property, not a defended frontier.",
         "The attacker cannot 'scale up' to break this. Quantity is "
         "the wrong axis."),
        ("Compare to detection-arms-race defenses.",
         "Each defense improvement requires more training data, "
         "which requires more bot samples, which requires "
         "infrastructure to collect samples..."),
        ("Relativistic ratings skip this loop entirely.",
         "Bots are not detected; they are structurally invisible."),
        ("Detection still helps for fake-account cleanup.",
         "But it is not the primary defense. The math is."),
    ])
    add_footer(s, 64, TOTAL, BRAND)

    s = table_slide(prs,
        "Sybil-resistance approaches compared", [
        ["Approach", "Mechanism", "Failure mode"],
        ["CAPTCHA",
         "Human-verification gate",
         "Captcha farms ($0.001/solve)"],
        ["PoW identity creation",
         "Burn electricity per quid",
         "Parallelizable; rich attackers win"],
        ["PoS identity stake",
         "Lock economic stake",
         "Wealthy attackers win"],
        ["Global reputation history",
         "Build clean track record over months",
         "Aged accounts sold in gray markets"],
        ["Relativistic trust graph",
         "Get observers to trust you",
         "High per-observer; no scaling"],
    ], subtitle="Only one row scales attacker cost with attack reach.",
        col_widths=[1.8, 2.5, 2.5], body_size=11)
    add_footer(s, 65, TOTAL, BRAND)

    s = content_slide(prs, "What this means in practice", bullets=[
        ("A platform running Quidnug PoT does not need a heavy fraud-"
         "detection pipeline.",
         "The math handles it. Detection is supplementary."),
        ("Operational savings.",
         "Smaller trust-and-safety team for fraud. Engineering effort "
         "shifts to trust-graph health and visualization."),
        ("Better user experience.",
         "Users see ratings reflecting their actual trust. Less noise "
         "from astroturfing and bombing. Higher decision-quality "
         "signal."),
        ("Better-economic-model.",
         "Brokers cannot sell what doesn't work. Fake-review economy "
         "shrinks for platforms running PoT. Visible win for the "
         "ecosystem."),
        ("Compounds over time.",
         "Attackers move to easier targets (other platforms) as PoT "
         "deployments mature."),
    ])
    add_footer(s, 66, TOTAL, BRAND)

    s = quote_slide(prs,
        "We are not 'defending against' Sybil attacks. We are "
        "rendering them mathematically irrelevant.",
        "How Quidnug treats spam",
        title="The framing")
    add_footer(s, 67, TOTAL, BRAND)

    # ==== SECTION 8: Visualization at Three Depths (68-77) ====
    s = section_divider(prs, 8, "Visualization at Three Depths",
        "Aurora, constellation, trace. Progressive disclosure.")
    add_footer(s, 68, TOTAL, BRAND)

    s = content_slide(prs,
        "The visualization design problem", bullets=[
            ("A relativistic rating has more dimensions than a "
             "five-star scalar.",
             "Rating, confidence, trust directness, polarization, "
             "freshness, factor decomposition."),
            ("Surfacing all of it without overwhelming users is a "
             "UX problem.",
             "Three primitives at three information densities."),
            ("Same input data feeds all three.",
             "Host page computes rating state once; passes into "
             "whichever primitive is needed."),
            ("Progressive disclosure.",
             "Glance for list views. Detail for product pages. "
             "Drilldown for the curious. Audit for experts."),
            ("Schema.org compatibility preserved.",
             "Stars still emit AggregateRating JSON-LD for SEO. "
             "Humans see the richer graphic."),
        ])
    add_footer(s, 69, TOTAL, BRAND)

    s = _im(prs, "Three primitives, side by side",
        "chart_three_primitives.png",
        caption="Aurora: glance. Constellation: drilldown. "
                "Trace: side-by-side comparison.",
        image_h=4.2)
    add_footer(s, 70, TOTAL, BRAND)

    s = content_slide(prs, "Primitive 1: <qn-aurora>", bullets=[
        ("The glanceable indicator.",
         "Replaces the star average in list views and product cards."),
        ("Encodes four dimensions.",
         "Rating value (color + numeric), confidence (ring "
         "thickness), directness (ring pattern), personalization "
         "delta (chip)."),
        ("Three sizes: nano, standard, large.",
         "From product-grid thumbnail (24px) to detail-page hero "
         "(120px+)."),
        ("Accessibility.",
         "Color-blind redundancy: shape codes sentiment (dot/square/"
         "triangle). aria-label gives plain-language summary."),
        ("Use when the user is scanning or browsing.",
         "Glance fidelity. Milliseconds of attention per item."),
    ])
    add_footer(s, 71, TOTAL, BRAND)

    s = content_slide(prs, "Primitive 2: <qn-constellation>", bullets=[
        ("The drilldown view.",
         "Bullseye showing every contributing reviewer as a dot."),
        ("Encodes per-reviewer dimensions.",
         "Position = trust proximity (you at center). Color = their "
         "rating. Size = their weight. Outline = direct vs "
         "transitive trust."),
        ("Click a dot to see the trust path.",
         "Provenance: 'you -> Alice -> Bob -> this reviewer.'"),
        ("Answers the 'why this rating' question.",
         "A global rating cannot answer this. Relativistic ratings "
         "make it concrete."),
        ("Use when the user wants to evaluate or audit.",
         "30+ seconds of deliberate attention. Detail page or "
         "drilldown view."),
    ])
    add_footer(s, 72, TOTAL, BRAND)

    s = content_slide(prs, "Primitive 3: <qn-trace>", bullets=[
        ("The side-by-side composition view.",
         "Horizontal stacked bar showing per-reviewer weight share "
         "and rating color."),
        ("Encodes weight composition.",
         "Width of each segment = reviewer's weight share. Color = "
         "their rating. Outline = directness."),
        ("Eyeball-friendly for comparing multiple products.",
         "'Mostly wide solid green' vs 'narrow mixed dashed' "
         "across a product list."),
        ("Compact: 20-80px tall, full-width-ish.",
         "Fits in any list / table / comparison view."),
        ("Use when comparing multiple items.",
         "Comparison shopping, product matrix, competitive review "
         "page."),
    ])
    add_footer(s, 73, TOTAL, BRAND)

    s = content_slide(prs, "Composition pattern: progressive disclosure",
        bullets=[
            ("Time t=0 (search results page).",
             "Aurora-nano next to product. User browses dozens "
             "of items in seconds."),
            ("Time t=2s (clicks into product detail).",
             "Aurora-standard in hero, plus inline trace. User sees "
             "personalized rating + composition at a glance."),
            ("Time t=8s (clicks 'see why').",
             "Constellation expands. User sees the trust graph that "
             "drove their rating, with top contributors listed."),
            ("Time t=60s (audit / expert mode).",
             "Full trust-path explorer. User can see exact "
             "computation, drill into any reviewer's history, "
             "evaluate alternative trust graphs."),
            ("Each step respects the user's attention budget.",
             "Global ratings cannot do this; they have no progressive "
             "disclosure target."),
        ])
    add_footer(s, 74, TOTAL, BRAND)

    s = code_slide(prs, "Schema.org compatibility preserved",
        [
            '<script type="application/ld+json">',
            '{',
            '  "@context": "https://schema.org",',
            '  "@type": "Product",',
            '  "name": "Apple MacBook Pro 16\\"",',
            '  "aggregateRating": {',
            '    "@type": "AggregateRating",',
            '    "ratingValue": "4.3",',
            '    "reviewCount": "487"',
            '  }',
            '}',
            '</script>',
            '',
            '<qn-aurora rating="4.5" size="standard"',
            '           observer="did:quidnug:..."',
            '           product="did:quidnug:..."',
            '           topic="reviews.public.tech.laptops">',
            '  <span aria-label="4.5 stars personalized">',
            '    \u2605\u2605\u2605\u2605\u2606 4.5 (your view)',
            '  </span>',
            '</qn-aurora>',
        ])
    add_footer(s, 75, TOTAL, BRAND)

    s = content_slide(prs, "WCAG 2.1 AA accessibility", bullets=[
        ("Color redundancy.",
         "Sentiment encoded in shape: dot (good) / square (mixed) / "
         "triangle (bad). Color-blind readers see different shapes."),
        ("Direct vs transitive trust.",
         "Solid vs dashed outline on every dot. Screen reader text "
         "names the trust distance explicitly."),
        ("Delta direction.",
         "Up/down arrows + sign prefix; not color-only."),
        ("Keyboard navigation.",
         "Every interactive element tab-reachable. Constellation "
         "dots respond to Enter/Space."),
        ("Reduced motion.",
         "Animations respect prefers-reduced-motion: reduce. No "
         "essential information conveyed only through animation."),
    ])
    add_footer(s, 76, TOTAL, BRAND)

    s = content_slide(prs, "Framework adapters available", bullets=[
        ("Web Components.",
         "@quidnug/reviews-web-components. Drop-in <qn-aurora> etc."),
        ("React.",
         "@quidnug/reviews-react. <Aurora>, <Constellation>, <Trace>."),
        ("Vue.",
         "@quidnug/reviews-vue. Same shape."),
        ("Astro (SSR).",
         "@quidnug/reviews-astro. Server-renders to static HTML for "
         "SEO."),
        ("WordPress.",
         "quidnug-reviews-wp plugin. Shortcode [qn-aurora "
         "product=...]."),
        ("Shopify.",
         "Theme snippet in clients/shopify/. Drops into product "
         "templates."),
    ])
    add_footer(s, 77, TOTAL, BRAND)

    # ==== SECTION 9: Real-World + Tradeoffs (78-90) ====
    s = section_divider(prs, 9, "Real-World Examples and Tradeoffs",
        "Where this works. Where it doesn't. What to do Monday.")
    add_footer(s, 78, TOTAL, BRAND)

    s = content_slide(prs, "Example 1: restaurant reviews (Yelp)", bullets=[
        ("A Yelp restaurant has a single 3.8-star average.",
         "Who is that average over?"),
        ("60% first-time visitors.",
         "25% occasional diners. 10% local regulars. 5% "
         "professional critics."),
        ("Each group has systematically different preferences.",
         "Tourist wants reliability. Foodie wants adventure. Critic "
         "wants technical skill."),
        ("Relativistic model.",
         "Foodie weights other foodies + critics: sees 4.2. "
         "Tourist with no graph: sees crowd default 3.8. Both honest."),
        ("All three views are correct.",
         "Any one alone is misleading."),
    ])
    add_footer(s, 79, TOTAL, BRAND)

    s = content_slide(prs, "Example 2: Letterboxd vs IMDb", bullets=[
        ("IMDb shows global aggregate.",
         "All voters get equal weight. Result: predictable "
         "regression to the mean for every well-known film."),
        ("Letterboxd emphasizes social graph.",
         "You follow specific reviewers. Your view is shaped by "
         "their opinions."),
        ("Letterboxd is empirically more predictive of personal "
         "preference.",
         "Because it pre-selects reviewers who share your taste."),
        ("Quidnug generalizes the Letterboxd model.",
         "Rather than requiring you to manually follow, computes "
         "the weighted score from your existing trust graph."),
        ("Default fallback.",
         "Zero graph: get the crowd default. Mature graph: get "
         "personalized."),
    ])
    add_footer(s, 80, TOTAL, BRAND)

    s = content_slide(prs, "Example 3: Healthgrades for specialists", bullets=[
        ("A specialist rated 4.2 on Healthgrades.",
         "Who reviewed?"),
        ("Other specialists (peer assessment).",
         "Primary care physicians (referring-doctor view)."),
        ("Patients with one visit.",
         "Caregivers of long-term patients."),
        ("Each group has different information.",
         "A patient choosing a cardiologist wants different signals "
         "than a hospital choosing a partner."),
        ("Relativistic.",
         "Patient weights specialists + similar-condition patients. "
         "Hospital weights peer-doctors + outcome data. Different "
         "weighted views."),
    ])
    add_footer(s, 81, TOTAL, BRAND)

    s = content_slide(prs, "Example 4: academic citations", bullets=[
        ("Google Scholar shows raw citation count.",
         "All citations equal weight. Predatory journals and "
         "self-citations count the same as Nature papers."),
        ("Scite.ai is partial improvement.",
         "Marks citations as supporting / contradicting / mentioning. "
         "Better signal."),
        ("Relativistic academic rating would go further.",
         "Weight citing papers by their own reputation in your "
         "subfield. Citation from a seminal paper >> 100 from "
         "tangential ones."),
        ("Reviewer trust = co-authored / cited / endorsed previously.",
         "Each researcher has a per-observer trust score grounded in "
         "the citation graph."),
        ("This would transform peer review and tenure cases.",
         "Currently both are based on metrics that are easy to "
         "game."),
    ])
    add_footer(s, 82, TOTAL, BRAND)

    s = content_slide(prs, "Honest tradeoff 1: bootstrap problem", bullets=[
        ("A new user with no trust graph has no personalization.",
         "Sees the crowd default."),
        ("Crowd default itself can be opinionated.",
         "Operator's curated reviewer roster, or weighted average of "
         "validated reviewers."),
        ("Personalization emerges with use.",
         "Comparable to first-time Spotify or Netflix: generic "
         "until interaction; personalized over time."),
        ("Onboarding flows can pre-populate trust graphs.",
         "OIDC import: 'people you follow on LinkedIn / Twitter / "
         "GitHub.' Optional but helpful."),
        ("Cold-start is universal in personalized systems.",
         "Not a unique problem to relativistic ratings."),
    ])
    add_footer(s, 83, TOTAL, BRAND)

    s = content_slide(prs, "Honest tradeoff 2: the filter bubble concern",
        bullets=[
            ("Pariser and Sunstein's worries are legitimate.",
             "Personalization can isolate people from opposing views."),
            ("Two responses.",
             "(1) Global ratings already filter-bubble in the WORST "
             "way: include irrelevant reviewers. (2) Quidnug's "
             "constellation surfaces the crowd alongside the "
             "personalized."),
            ("Active research direction: 'adversarial recommenders.'",
             "Deliberately push diverse views. Quidnug supports user-"
             "configurable 'intellectual diversity' sub-graphs."),
            ("Filter bubbles are a problem when the filter is opaque.",
             "Relativistic ratings make the filter transparent."),
            ("Users can OPT OUT of personalization at any time.",
             "Toggle to 'crowd' mode. The right default is "
             "personalized; the toggle is one click."),
        ])
    add_footer(s, 84, TOTAL, BRAND)

    s = content_slide(prs, "Honest tradeoff 3: when global ratings ARE right",
        bullets=[
            ("Legal / regulatory standards.",
             "Restaurant compliance score is objective, not "
             "relativistic. Same for car safety ratings."),
            ("Certifications.",
             "ISO 27001 compliance is binary, not graded by "
             "observer."),
            ("Safety ratings.",
             "Crash test scores. Medical device approvals. One "
             "truth applies to everyone."),
            ("Use the right tool.",
             "Relativistic for opinion-shaped judgments (quality, "
             "fit, reliability). Global for objective-shaped "
             "judgments (compliance, certification, safety)."),
            ("Both can coexist on the same page.",
             "Global compliance score next to relativistic quality "
             "rating. Different signals; both useful."),
        ])
    add_footer(s, 85, TOTAL, BRAND)

    s = content_slide(prs, "Honest tradeoff 4: computational cost", bullets=[
        ("Per-observer rating is not free.",
         "For each (observer, product, topic): compute trust paths "
         "for each reviewer."),
        ("Per-query cost.",
         "O(b^d) per trust path with b ~ 10, d ~ 5: ~100k node "
         "visits worst case."),
        ("With caching (Quidnug TrustCache).",
         "Amortizes to <1 millisecond per query."),
        ("Compared to global average.",
         "Global is O(1) after precomputation. Relativistic is "
         "microseconds. Real cost but small."),
        ("Caching strategies.",
         "Per-observer page-level rating snapshots; eager precompute "
         "for hot products. Standard engineering, not novel "
         "research."),
    ])
    add_footer(s, 86, TOTAL, BRAND)

    s = content_slide(prs, "What to do Monday morning", bullets=[
        ("This week.",
         "Audit your review system. Check the J-curve. Estimate "
         "fraction of fake reviews. Compute your average rating "
         "across competitor products. Are you all in the 4.4 "
         "equivalence class?"),
        ("This month.",
         "Read Hu/Pavlou/Zhang 2006 and Resnick et al. 2000. "
         "Have your team agree on the framing."),
        ("This quarter.",
         "Prototype Quidnug Reviews Protocol (QRP-0001) on one "
         "product category. Compare ratings to your existing global "
         "scores. Measure reviewer-trust-graph density."),
        ("This year.",
         "Roll out relativistic ratings to user-facing pages. Keep "
         "Schema.org JSON-LD for SEO. Replace stars with aurora in "
         "the UI."),
        ("Next year.",
         "Cross-platform trust portability via Quidnug shared "
         "graphs. Reviewer reputation that follows them across "
         "platforms."),
    ])
    add_footer(s, 87, TOTAL, BRAND)

    s = content_slide(prs, "Summary: the four takeaways revisited",
        bullets=[
            ("1. The five-star average is statistically malformed.",
             "J-curve self-selection. Threshold effects. The number "
             "is not measuring what people think it is."),
            ("2. Review spam is rational under global, irrational "
             "under relativistic.",
             "Math changes the attacker's cost function. Spam is "
             "solved as a structural side effect."),
            ("3. Six decades of social science prescribe the "
             "relativistic model.",
             "Asch, Festinger, Granovetter, Mayer/Davis/Schoorman, "
             "Resnick, Jøsang. We are catching up to the research."),
            ("4. You can visualize without overwhelming users.",
             "Aurora, constellation, trace. Progressive disclosure. "
             "Same primitive scales from list view to expert audit."),
        ])
    add_footer(s, 88, TOTAL, BRAND)

    s = content_slide(prs, "References (canonical sources)", bullets=[
        ("Hu, Pavlou, Zhang (2006). Online review distributions.",
         "ACM EC."),
        ("Anderson and Magruder (2012). Yelp threshold effects.",
         "Economic Journal."),
        ("Luca (2016). Yelp revenue impact.",
         "HBS Working Paper 12-016."),
        ("Luca and Zervas (2016). Yelp review fraud.",
         "Management Science."),
        ("He, Hollenbeck, Proserpio (2022). Fake review market.",
         "NBER W29855."),
        ("Mayzlin, Dover, Chevalier (2014). Astroturfing on TripAdvisor.",
         "American Economic Review."),
        ("Resnick et al. (2000). Reputation Systems.",
         "CACM 43(12)."),
        ("Mayer, Davis, Schoorman (1995). Integrative Trust Model.",
         "AMR 20(3)."),
        ("Jøsang, Ismail, Boyd (2007). Trust survey.",
         "Decision Support Systems."),
    ])
    add_footer(s, 89, TOTAL, BRAND)

    s = closing_slide(prs,
        "Questions",
        subtitle="Thank you. The useful part starts now.",
        cta="Where does the relativistic rating model fit your "
            "use case?\n\n"
            "What about your platform's incentive structure resists "
            "this?\n\n"
            "Which constraint hits hardest: bootstrap, "
            "personalization toggle, or compute?",
        resources=[
            "github.com/quidnug/quidnug",
            "blogs/2026-04-20-relativistic-ratings-end-of-review-spam.md",
            "Hu, Pavlou, Zhang (2006) ACM EC J-curve study",
            "Resnick et al. (2000) CACM Reputation Systems",
            "He, Hollenbeck, Proserpio (2022) NBER W29855",
            "QRP-0001 reference: examples/reviews-and-comments/",
            "FTC 16 CFR Part 465 (2024)",
        ])
    add_footer(s, 90, TOTAL, BRAND)

    return prs


if __name__ == "__main__":
    prs = build()
    prs.save(str(OUTPUT))
    print(f"wrote {OUTPUT} ({len(prs.slides)} slides)")
