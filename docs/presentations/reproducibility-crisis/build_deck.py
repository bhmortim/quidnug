"""Reproducibility Crisis deck (~80 slides)."""
import pathlib, sys
HERE = pathlib.Path(__file__).parent
ASSETS = HERE / "assets"
OUTPUT = HERE / "reproducibility-crisis.pptx"
sys.path.insert(0, str(HERE.parent))
from _deck_helpers import (  # noqa: E402
    make_presentation, title_slide, section_divider, content_slide,
    two_col_slide, stat_slide, quote_slide, table_slide, image_slide,
    code_slide, icon_grid_slide, closing_slide, add_notes, add_footer,
    TEAL, CORAL, EMERALD, AMBER, TEXT_MUTED,
)
from pptx.util import Inches  # noqa: E402

BRAND = "Quidnug  \u00B7  Reproducibility Crisis"
TOTAL = 80


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
        "The Reproducibility Crisis Needs Tamper-Evident Peer Review",
        "Why 39% of psychology studies replicate, why 70% of "
        "researchers have failed to reproduce a colleague's work, "
        "and how signed attestation chains fix what traditional peer "
        "review can't.",
        eyebrow="QUIDNUG  \u00B7  SCIENCE")
    add_notes(s, [
        "Welcome. 80 slides, about 60-70 min with Q&A.",
        "Audience: researchers, journal editors, research "
        "infrastructure teams, funding-agency staff.",
        "Central thesis: the reproducibility crisis is inseparable "
        "from the absence of a tamper-evident trust substrate for "
        "peer review.",
        "Cite Ioannidis 2005, OSC 2015, Baker 2016, Begley/Ellis 2012 "
        "as the empirical anchors. We will defend each."
    ])
    add_footer(s, 1, TOTAL, BRAND)

    s = stat_slide(prs, "39%",
        "of psychology studies in top journals successfully "
        "replicated (Open Science Collaboration, Science 2015).",
        context="Mean replication effect size was roughly half "
                "of the original. The instrument we use to evaluate "
                "scientific claims is broken at the substrate level.",
        title="Where we are in 2026")
    add_footer(s, 2, TOTAL, BRAND)

    s = stat_slide(prs, "11%",
        "of 53 landmark cancer biology papers reproduced at Amgen "
        "(Begley & Ellis, Nature 2012).",
        context="6 of 53. Not a typo. The most-cited single number "
                "in the reproducibility literature.",
        title="The most damning data point", stat_color=CORAL)
    add_footer(s, 3, TOTAL, BRAND)

    s = content_slide(prs, "Agenda", bullets=[
        ("Section 1. The empirical scale.",
         "Replication rates, retractions, fraud cases, the cost."),
        ("Section 2. Systemic causes.",
         "Publish-or-perish, p-hacking, low power, peer review "
         "blind spots."),
        ("Section 3. Fixes that partially work.",
         "Preregistration, open data, open review, replication "
         "journals."),
        ("Section 4. The missing substrate.",
         "Four cryptographic primitives current systems lack."),
        ("Section 5. Signed peer review.",
         "Reviewer identity, signed reviews, append-only chain."),
        ("Section 6. Data provenance chains.",
         "Lineage from raw collection to published figure."),
        ("Section 7. Citation-weighted reputation.",
         "Reviewer quality computable, not editorial secret."),
        ("Section 8. A paper's life cycle under this system.",
         "Concrete walkthrough from preregistration to supersession."),
        ("Section 9. Institutional tradeoffs and rollout.",
         "What this requires from journals, universities, funders."),
    ])
    add_footer(s, 4, TOTAL, BRAND)

    s = content_slide(prs, "Four key claims this talk defends",
        bullets=[
            ("Claim 1.",
             "The replication crisis measures infrastructure quality, "
             "not researcher quality."),
            ("Claim 2.",
             "Detection of fraud and selective reporting is a losing "
             "arms race. Structural substrates win."),
            ("Claim 3.",
             "Cryptographic data provenance would have caught at least "
             "two of the three highest-profile fraud cases of the past "
             "decade before replication teams were needed."),
            ("Claim 4.",
             "Institutional inertia is real but weaker than what "
             "preprint adoption faced. Preprints went from fringe to "
             "default in roughly a decade."),
        ])
    add_footer(s, 5, TOTAL, BRAND)

    # Section 1
    s = section_divider(prs, 1, "The Empirical Scale",
        "Replication rates, retractions, fraud cases. Hard numbers.")
    add_footer(s, 6, TOTAL, BRAND)

    s = _im(prs, "Cross-field replication rates: 11% to 62%",
        "chart_replication.png",
        caption="Begley & Ellis (Nature 2012); Errington et al "
                "(eLife 2021); OSC (Science 2015); Camerer et al "
                "(Science 2016, Nature Human Behaviour 2018).",
        image_h=4.2)
    add_footer(s, 7, TOTAL, BRAND)

    s = content_slide(prs,
        "OSC 2015: the most-cited replication study", bullets=[
            ("Open Science Collaboration.",
             "'Estimating the reproducibility of psychological "
             "science.' Science 349(6251). aac4716, 2015."),
            ("Method.",
             "270 authors recruited to replicate 100 studies from "
             "three top psychology journals (JPSP, Psych Sci, JEP)."),
            ("Result, statistical significance criterion.",
             "Only 36% of replications reached p < 0.05 in the same "
             "direction as the original."),
            ("Result, subjective replication.",
             "39% considered to have replicated."),
            ("Effect-size attenuation.",
             "Mean replication effect size was roughly half of the "
             "original. Not noise; systematic shrinkage."),
            ("Cited 7,000+ times.",
             "Methodologically scrutinized; not refuted."),
        ])
    add_footer(s, 8, TOTAL, BRAND)

    s = _im(prs, "Baker 2016 Nature survey: it is not just one field",
        "chart_baker.png",
        caption="Baker, M. (2016). 'Is there a reproducibility "
                "crisis?' Nature 533, 452-454. n = 1,576 researchers.",
        image_h=4.0)
    add_footer(s, 9, TOTAL, BRAND)

    s = content_slide(prs, "Begley & Ellis 2012: the cancer biology number",
        bullets=[
            ("Begley & Ellis.",
             "'Drug development: Raise standards for preclinical "
             "cancer research.' Nature 483, 531-533. 2012."),
            ("Method.",
             "Amgen's industrial biology team attempted to reproduce "
             "53 landmark oncology studies."),
            ("Result.",
             "6 of 53 (~11%) reproduced. The remaining 47 had effects "
             "that did not survive replication."),
            ("Industry context.",
             "Amgen is not a hostile actor; they wanted these to "
             "reproduce so they could build drugs on them."),
            ("Implication.",
             "If only 11% of preclinical cancer biology replicates, "
             "drug development based on the literature is built on "
             "shifting sand."),
        ])
    add_footer(s, 10, TOTAL, BRAND)

    s = _im(prs, "Retractions are accelerating exponentially",
        "chart_retractions.png",
        caption="Sources: Retraction Watch database, NIH PubMed "
                "retraction notices, Wiley/Hindawi 2023 mass-"
                "retraction event.",
        image_h=4.2)
    add_footer(s, 11, TOTAL, BRAND)

    s = _im(prs, "Six high-profile fraud cases of the past two decades",
        "chart_fraud_cases.png",
        caption="Each case took 5-15 years to surface. Several "
                "would have been caught in months with cryptographic "
                "data provenance.",
        image_h=4.4)
    add_footer(s, 12, TOTAL, BRAND)

    s = content_slide(prs, "What this costs society", bullets=[
        ("Direct: irreproducible research wastes ~$28B/year in US "
         "preclinical biomedicine alone.",
         "Freedman, Cockburn, Simcoe (PLOS Biology 2015)."),
        ("Indirect: drug pipelines built on bad foundations.",
         "Phase II/III failures often trace back to non-replicated "
         "preclinical work."),
        ("Public trust erosion.",
         "Pew Research 2023: 27% of US adults have 'a great deal of "
         "confidence' in scientists, down from 39% in 2020."),
        ("Policy decisions on shaky ground.",
         "Public-health, education, criminal-justice policy "
         "frequently cites studies that do not replicate."),
        ("Researcher career consequences.",
         "Junior researchers cannot tell which findings to build on. "
         "Wasted years on dead foundations."),
    ])
    add_footer(s, 13, TOTAL, BRAND)

    s = quote_slide(prs,
        "Most published research findings are false.",
        "John Ioannidis, PLOS Medicine, 2005",
        context="The most-cited paper in PLOS Medicine history. "
                "Twenty years of empirical work has confirmed something "
                "in that neighborhood.",
        title="The 2005 paper that started this conversation")
    add_footer(s, 14, TOTAL, BRAND)

    # Section 2
    s = section_divider(prs, 2, "Systemic Causes",
        "Publish-or-perish, p-hacking, low power, peer review blind spots.")
    add_footer(s, 15, TOTAL, BRAND)

    s = content_slide(prs, "Cause 1: publish-or-perish", bullets=[
        ("Tenure, promotion, grant funding all gate on publication "
         "count and journal prestige.",
         ""),
        ("Negative results are nearly unpublishable.",
         "Franco, Malhotra, Simonovits (Science 2014): only 21% of "
         "social-science null results submitted; rest filed away."),
        ("Selective reporting becomes rational.",
         "Researcher reports the analysis that worked, not the ten "
         "that didn't."),
        ("Career incentive misalignment.",
         "What earns tenure is not what advances science. We "
         "currently optimize for the wrong objective."),
        ("Substrate fix.",
         "Preregistration + signed-claim infrastructure makes "
         "selective reporting visible."),
    ])
    add_footer(s, 16, TOTAL, BRAND)

    s = content_slide(prs, "Cause 2: p-hacking and researcher degrees of freedom",
        bullets=[
            ("Simmons, Nelson, Simonsohn (Psychological Science 2011).",
             "'False-positive psychology.' Showed that 4 specific "
             "common analytical choices can push false-positive "
             "rates to 60% or more."),
            ("Researcher degrees of freedom.",
             "Inclusion criteria, outlier removal, dependent-variable "
             "choice, covariate inclusion, transformation choice. "
             "Each is a fork in the analytical garden."),
            ("Without preregistration, the analyst picks the path "
             "that gives p < 0.05.",
             "Not necessarily intentionally; cognitive bias is "
             "sufficient."),
            ("Substrate fix.",
             "Preregistered analysis script signed before data "
             "collection. Deviations are visible and accountable."),
            ("This is mechanical, not moral.",
             "Honest researchers fall into this. The substrate "
             "removes the temptation."),
        ])
    add_footer(s, 17, TOTAL, BRAND)

    s = _im(prs, "P-curve evidence: pile-up just below 0.05",
        "chart_phacking.png",
        caption="Simonsohn, Nelson, Simmons (2014) P-curve methodology. "
                "Spike in observed distribution just under threshold.",
        image_h=4.4)
    add_footer(s, 18, TOTAL, BRAND)

    s = content_slide(prs, "Cause 3: low statistical power", bullets=[
        ("Button, Ioannidis et al (Nature Reviews Neuroscience 2013).",
         "Median power of neuroscience studies: 21%. Should be at "
         "least 80%."),
        ("Underpowered studies that DO produce significant results "
         "have inflated effect sizes.",
         "Statistical artifact called 'winner's curse.'"),
        ("Replicators measure the true effect, find it smaller, "
         "fail to reach significance.",
         "Pattern is mechanical, not researcher fault."),
        ("Power calculations existed in 1969 (Cohen). "
         "Adoption: still partial in 2026.",
         ""),
        ("Substrate fix.",
         "Signed pre-registration includes a power calculation. "
         "Reviewers can demand justification."),
    ])
    add_footer(s, 19, TOTAL, BRAND)

    s = content_slide(prs, "Cause 4: peer review does not catch this",
        bullets=[
            ("What peer review actually does.",
             "Review the manuscript. Suggest revisions. Recommend "
             "accept/reject."),
            ("What it does NOT do.",
             "Re-run the analysis. Verify the data. Audit the "
             "lab notebook."),
            ("Reviewers receive PDFs, not data and code.",
             "Even open-data papers are not commonly re-analyzed by "
             "reviewers; nobody is paying for that work."),
            ("Reviewer time is uncompensated and scarce.",
             "Estimate: scholarly publishing extracts ~$2B/year of "
             "free reviewer labor (Aczel et al 2021)."),
            ("Substrate cannot replace peer review.",
             "But it can give peer review the materials it needs to "
             "actually verify, plus an auditable record of what "
             "happened."),
        ])
    add_footer(s, 20, TOTAL, BRAND)

    s = content_slide(prs, "The incentive summary", bullets=[
        ("Researcher incentive: publish positive, novel results in "
         "high-prestige journals.",
         ""),
        ("Journal incentive: publish citable, attention-grabbing "
         "papers.",
         ""),
        ("Reviewer incentive: minimal time, anonymous.",
         "No reputation stake in being right or wrong."),
        ("University incentive: produce papers that earn tenure "
         "review credit.",
         ""),
        ("Funder incentive: fund work that produces publications.",
         ""),
        ("None of these incentivize careful, replicable work.",
         "The substrate must change incentives by making careful work "
         "visible and rewardable."),
    ])
    add_footer(s, 21, TOTAL, BRAND)

    # Section 3
    s = section_divider(prs, 3, "Fixes That Partially Work",
        "Preregistration, open data, open review, replication "
        "journals. Necessary but not sufficient.")
    add_footer(s, 22, TOTAL, BRAND)

    s = content_slide(prs, "Fix 1: preregistration", bullets=[
        ("Researcher commits to hypothesis + analysis plan BEFORE "
         "data collection.",
         "Commitment is timestamped on a public registry."),
        ("Effective when used.",
         "Allen & Mehler 2019: registered reports have "
         "significantly lower rates of effect-size inflation."),
        ("Registration platforms.",
         "OSF (Open Science Framework), AsPredicted, ClinicalTrials.gov "
         "for clinical trials."),
        ("Limitations.",
         "Voluntary; not enforced. Researchers can deviate from the "
         "plan and not disclose. The 'pre' part is honor-system."),
        ("Substrate improvement.",
         "Cryptographic signing makes the preregistration tamper-"
         "evident and the deviation accountability mechanical."),
    ])
    add_footer(s, 23, TOTAL, BRAND)

    s = content_slide(prs, "Fix 2: open data", bullets=[
        ("Authors publish raw data alongside the paper.",
         "Mandated by some funders (NIH 2023+) and journals (PLOS, "
         "BMJ)."),
        ("Helps when followed.",
         "Re-analysis becomes possible. Some errors are caught."),
        ("Limitations.",
         "Most uploads are post-hoc, not provenance-linked. "
         "Connection between 'this is the dataset I analyzed' and "
         "'this is the figure I plotted' is rarely cryptographic."),
        ("Selective sharing.",
         "Authors share what supports the published claim; not "
         "necessarily everything they collected."),
        ("Substrate improvement.",
         "Provenance chains link instrument to figure with "
         "cryptographic continuity."),
    ])
    add_footer(s, 24, TOTAL, BRAND)

    s = content_slide(prs, "Fix 3: open peer review", bullets=[
        ("Reviewer identity disclosed.",
         "EMBO Journal, Wellcome Open Research, eLife. Mostly "
         "voluntary."),
        ("Helps with accountability.",
         "Reviewers cannot hide behind anonymity to provide "
         "low-quality reviews."),
        ("Limitations.",
         "Senior reviewers worry about retaliation from junior "
         "authors. Junior reviewers worry about retaliation from "
         "senior authors. Both reasonable."),
        ("Pseudonymous reviewer identity solves this.",
         "Cryptographically stable identifier without real-name "
         "exposure. Reputation accrues; safety preserved."),
        ("Substrate improvement.",
         "Quidnug pseudonymous quids let reviewer reputation form "
         "without forcing real-name disclosure."),
        ])
    add_footer(s, 25, TOTAL, BRAND)

    s = content_slide(prs, "Fix 4: replication journals + registered reports",
        bullets=[
            ("Royal Society Open Science, Cortex, Comprehensive "
             "Results in Social Psychology, others.",
             "Publish replication studies and registered reports."),
            ("Registered reports model.",
             "Editorial decision based on the design BEFORE data "
             "collection. If the design is sound, paper publishes "
             "regardless of result."),
            ("Effective.",
             "Reduces publication bias. Allen & Mehler 2019: "
             "registered reports have a much higher null-result "
             "publication rate (44% vs ~5% for traditional)."),
            ("Limitations.",
             "Still niche. Most fields don't have a registered-"
             "reports venue. Adoption is journal-by-journal."),
            ("Substrate complement.",
             "Not a replacement; the substrate makes registered "
             "reports easier to verify and audit."),
        ])
    add_footer(s, 26, TOTAL, BRAND)

    s = quote_slide(prs,
        "Each existing fix is necessary. None is sufficient. "
        "Together they cover maybe a third of the failure surface.",
        "Why we need a substrate, not just policies",
        title="The honest summary")
    add_footer(s, 27, TOTAL, BRAND)

    # Section 4
    s = section_divider(prs, 4, "The Missing Substrate",
        "Four primitives current scholarly infrastructure does not provide.")
    add_footer(s, 28, TOTAL, BRAND)

    s = _im(prs, "The four primitives",
        "chart_four_primitives.png",
        caption="Each primitive maps to an existing failure mode. "
                "All four together close the trust gap.",
        image_h=4.4)
    add_footer(s, 29, TOTAL, BRAND)

    s = content_slide(prs, "Primitive 1: signed claims", bullets=[
        ("Every assertion in a paper binds to the author's "
         "cryptographic identity at publication time.",
         ""),
        ("Identity persists across institutional changes.",
         "Move universities, change names: the same cryptographic "
         "identity carries the publication record."),
        ("Disambiguation.",
         "ORCID solved part of this; cryptographic signing closes "
         "the rest."),
        ("Tamper detection.",
         "If a published claim is later silently altered, the "
         "signature breaks. Visible to anyone who checks."),
        ("Cost.",
         "ECDSA P-256 signature is 64 bytes. Verification is ~50 "
         "microseconds. Trivial overhead."),
    ])
    add_footer(s, 30, TOTAL, BRAND)

    s = content_slide(prs, "Primitive 2: signed peer review", bullets=[
        ("Each reviewer's review is signed with a stable identity.",
         "Pseudonymous OK; the identity is cryptographically "
         "consistent across their review history."),
        ("Reviewer reputation accrues over time.",
         "Computable from review-replication correlation."),
        ("Append-only review chain.",
         "Editor decisions, reviewer comments, author rebuttals all "
         "form an ordered chain."),
        ("Anyone can verify the chain.",
         "Did this paper actually receive substantive review? Or was "
         "it rubber-stamped?"),
        ("Privacy preserved.",
         "Reviewer real name remains hidden if they choose. The "
         "cryptographic identity is what the reputation builds on."),
    ])
    add_footer(s, 31, TOTAL, BRAND)

    s = content_slide(prs, "Primitive 3: data provenance chains",
        bullets=[
            ("Every dataset cited has verifiable lineage from raw "
             "instrument output to published analysis.",
             ""),
            ("Each transformation is signed.",
             "Acquisition, validation, cleaning, analysis: each step "
             "produces a signed artifact."),
            ("Modifications break the chain.",
             "Quietly altering a value in the analysis stage breaks "
             "the signature; the chain becomes invalid."),
            ("Audit becomes mechanical.",
             "Replicators don't need to ask 'did you really collect "
             "this data this way?' They can verify it."),
            ("Compatible with privacy.",
             "Sensitive data can be hashed at acquisition; "
             "provenance is preserved without exposing raw data."),
        ])
    add_footer(s, 32, TOTAL, BRAND)

    s = _im(prs, "Data provenance: lineage from instrument to figure",
        "chart_provenance.png",
        caption="Every node signed; every edge a verifiable link. "
                "Tampering with any node breaks the chain.",
        image_h=4.2)
    add_footer(s, 33, TOTAL, BRAND)

    s = content_slide(prs, "Primitive 4: citation-weighted reputation",
        bullets=[
            ("Reviewer quality is computable, not editorial secret.",
             "From review history: how often did their accept/reject "
             "decisions correlate with later replication outcomes?"),
            ("Author reputation similarly.",
             "Citation-weighted by who is citing. Citations from "
             "high-replication-rate researchers count more than "
             "citations from low-replication-rate researchers."),
            ("Pseudonymous compatible.",
             "Reputation can attach to a stable cryptographic "
             "identity without revealing real name."),
            ("Operates on the trust graph.",
             "Same Quidnug primitives that power reviews and "
             "consent power scholarly reputation."),
            ("Replaces opaque editorial gatekeeping with auditable "
             "computation.",
             "Editors still curate; the curation rationale is now "
             "visible."),
        ])
    add_footer(s, 34, TOTAL, BRAND)

    s = content_slide(prs, "Cryptographic properties that matter",
        bullets=[
            ("Signed, not hashed.",
             "Anyone with the public key can verify; nobody can "
             "forge."),
            ("Append-only.",
             "Reviews, citations, retractions all add to the record. "
             "The record never silently changes."),
            ("Time-ordered.",
             "Sequence is preserved cryptographically, not just by "
             "filename or upload date."),
            ("Domain-scoped.",
             "Reputation in oncology does not auto-transfer to "
             "neuroscience. Each subfield is its own trust domain."),
            ("Revocation as data, not deletion.",
             "Retraction is a new signed record; the original "
             "remains visible with its retraction status linked."),
        ])
    add_footer(s, 35, TOTAL, BRAND)

    s = content_slide(prs, "What this does NOT require",
        bullets=[
            ("No central trust authority.",
             "No 'Science Inc.' issuing identities. Researchers "
             "self-host or use their institution's existing PKI."),
            ("No mandatory adoption.",
             "Adopting journals get the benefits. Non-adopting "
             "journals continue with current infrastructure. "
             "Coexistence is fine."),
            ("No new file formats.",
             "PDFs, datasets, code repos all stay. The signing "
             "metadata wraps existing artifacts."),
            ("No blockchain in the consumer sense.",
             "Quidnug uses signed records and trust graphs. No proof-"
             "of-work, no token economics."),
            ("No replacement of human judgment.",
             "Editors still decide; reviewers still review. The "
             "substrate makes their work auditable."),
        ])
    add_footer(s, 36, TOTAL, BRAND)

    # Section 5
    s = section_divider(prs, 5, "Signed Peer Review",
        "Structure of a signed review, pseudonymous identity, "
        "review chain on a paper.")
    add_footer(s, 37, TOTAL, BRAND)

    s = code_slide(prs, "Structure of a signed review",
        [
            '{',
            '  "type": "PEER_REVIEW",',
            '  "reviewer_quid": "9a82c4e1f0b7d2c5",  // pseudonymous',
            '  "paper_doi": "10.1038/s41586-2026-12345",',
            '  "submission_round": 1,',
            '  "decision": "minor_revision",',
            '  "review_text_cid": "bafyrei...",  // IPFS pointer',
            '  "confidence": 0.85,',
            '  "scores": {',
            '    "novelty": 4, "methodology": 3,',
            '    "reproducibility": 4, "clarity": 4',
            '  },',
            '  "timestamp": 1714579200,',
            '  "signature": "..."',
            '}'
        ])
    add_footer(s, 38, TOTAL, BRAND)

    s = content_slide(prs, "Pseudonymous reviewer identity", bullets=[
        ("Reviewer creates a cryptographic identity for review work.",
         "Optionally separate from their author identity."),
        ("All their reviews link to this identity.",
         "Reputation accrues via review history."),
        ("Real name remains hidden if reviewer chooses.",
         "Editor knows the real name (for assignment); the public "
         "record sees only the pseudonym."),
        ("Pseudonym is stable across years.",
         "Persistent reputation without real-name exposure."),
        ("Compromise scenario.",
         "Senior reviewer wants to give a critical review without "
         "career retaliation; junior reviewer wants to be honest "
         "without inviting senior pushback. Both work."),
    ])
    add_footer(s, 39, TOTAL, BRAND)

    s = _im(prs, "The review chain on a paper",
        "chart_review_chain.png",
        caption="Each step signed. Anyone can independently verify "
                "the review history.",
        image_h=4.0)
    add_footer(s, 40, TOTAL, BRAND)

    s = content_slide(prs, "What signed peer review enables", bullets=[
        ("Visible quality of review.",
         "Anyone can see if a paper got 4 substantive reviews or "
         "3 rubber-stamps."),
        ("Reviewer accountability.",
         "If a reviewer consistently approves papers that don't "
         "replicate, their reputation declines."),
        ("Deletable reviews surface tamper evidence.",
         "If a journal silently changes a review later, the chain "
         "breaks visibly."),
        ("Review work credit.",
         "Pseudonymous reviewers can credibly claim reviewing "
         "experience for tenure files (with editor verification)."),
        ("Cross-journal portability.",
         "A review submitted to Journal A can travel with the paper "
         "to Journal B, with provenance preserved."),
    ])
    add_footer(s, 41, TOTAL, BRAND)

    # Section 6
    s = section_divider(prs, 6, "Data Provenance Chains",
        "Lineage from raw collection to published figure.")
    add_footer(s, 42, TOTAL, BRAND)

    s = content_slide(prs, "The lineage graph", bullets=[
        ("Every dataset is a node in a directed acyclic graph.",
         "Edges represent transformations: clean, normalize, filter, "
         "summarize."),
        ("Each node and edge is signed.",
         "By the researcher who created it, with timestamp."),
        ("Citing a dataset means citing a hash, not a name.",
         "Names can collide and change; hashes are unique and stable."),
        ("Re-running an analysis recomputes the chain.",
         "If the recomputation matches, the chain verifies. If not, "
         "the divergence is visible."),
        ("Privacy-preserving variants.",
         "For sensitive data: hash + Merkle inclusion proofs. "
         "Provenance preserved without raw exposure."),
    ])
    add_footer(s, 43, TOTAL, BRAND)

    s = content_slide(prs, "What tamper-evidence catches", bullets=[
        ("Image manipulation.",
         "Tessier-Lavigne 2023 case: altered Western blots in "
         "papers from his lab. Provenance chains would have flagged "
         "the modifications at submission."),
        ("Selective data exclusion.",
         "Wansink 2018: dropped data points without disclosure. "
         "Provenance chain shows what was collected vs what was "
         "analyzed."),
        ("Data fabrication.",
         "Stapel 2011: invented entire datasets. Provenance chains "
         "would have required instrument-level signatures Stapel "
         "couldn't produce."),
        ("Re-analysis disputes.",
         "When reviewers want to re-run, they have exact data + code "
         "with verifiable provenance. No 'available on request.'"),
        ("Honest mistakes.",
         "Excel autoconverting gene names (a real problem in "
         "genetics): provenance shows the conversion happened, "
         "auditable."),
    ])
    add_footer(s, 44, TOTAL, BRAND)

    s = content_slide(prs, "Integration with existing infrastructure",
        bullets=[
            ("Works with existing repositories.",
             "Zenodo, Dryad, OSF, GitHub, Figshare. Quidnug provenance "
             "wraps the existing storage."),
            ("Works with existing identifiers.",
             "DOIs, ORCIDs, ROR (Research Organization Registry). "
             "Maps to Quidnug quids."),
            ("Works with existing analysis tools.",
             "R, Python, MATLAB, SAS. Sign the script + data + "
             "outputs at compute time."),
            ("Standards alignment.",
             "W3C PROV-O ontology models the same concepts. Quidnug "
             "provides the cryptographic enforcement layer."),
            ("No vendor lock-in.",
             "Trust graph is portable. Researcher leaving an "
             "institution takes their reputation with them."),
        ])
    add_footer(s, 45, TOTAL, BRAND)

    s = content_slide(prs, "What provenance chains do NOT solve",
        bullets=[
            ("Honest disagreement on analysis choices.",
             "Provenance verifies the choice was made; not whether "
             "it was right."),
            ("Theory selection.",
             "A wrong theoretical framework can produce verifiable "
             "provenance for verifiably wrong conclusions."),
            ("Subjective quality of writing or argument.",
             "Peer review still does that work."),
            ("Funding-source bias.",
             "Disclosure helps; provenance doesn't directly address."),
            ("This is one layer of a multi-layer defense.",
             "The other layers (open data, preregistration, "
             "registered reports) still matter."),
        ])
    add_footer(s, 46, TOTAL, BRAND)

    # Section 7
    s = section_divider(prs, 7, "Citation-Weighted Reputation",
        "Reviewer quality computable, not editorial secret.")
    add_footer(s, 47, TOTAL, BRAND)

    s = _im(prs,
        "Reviewer reputation: computed from history, not hidden",
        "chart_reviewer_rep.png",
        caption="Reputation = correlation between reviewer's accept/"
                "reject decisions and downstream replication outcomes.",
        image_h=4.0)
    add_footer(s, 48, TOTAL, BRAND)

    s = content_slide(prs, "The reputation computation", bullets=[
        ("For each reviewer, compute over their review history.",
         "How often did their accept/reject decisions correlate with "
         "subsequent replication?"),
        ("Other signals.",
         "Length and substance of review (vs perfunctory). "
         "Specificity of critiques. Spotting of errors that "
         "subsequently surfaced."),
        ("Combine multiplicatively.",
         "Same four-factor pattern as relativistic ratings: "
         "predictive accuracy * substance * activity * recency."),
        ("Result: each reviewer has a per-domain reputation.",
         "Visible to editors and to the trust graph at large."),
        ("Replaces opacity.",
         "Today, only editors know which reviewers are good. "
         "Tomorrow, the field knows."),
    ])
    add_footer(s, 49, TOTAL, BRAND)

    s = content_slide(prs, "Reviewer scarcity and the cost of being wrong",
        bullets=[
            ("Current state.",
             "Reviewers are unpaid, unrewarded, and anonymous. "
             "Their work is captured by publishers."),
            ("With reputation.",
             "Good reviewing is visible and rewarded. Universities "
             "can credit it for promotion."),
            ("Cost of being wrong.",
             "Today: zero. Tomorrow: reviewer reputation declines, "
             "fewer high-stakes assignments."),
            ("Cost of being lazy.",
             "Today: zero. Tomorrow: substantive review history is "
             "visible alongside reviewer's own publications."),
            ("Aczel et al 2021: scholarly publishing extracts ~$2B/yr "
             "of free reviewer labor.",
             "Compensation could come via reputation, not just cash."),
        ])
    add_footer(s, 50, TOTAL, BRAND)

    s = content_slide(prs, "Gaming the reputation signal", bullets=[
        ("Naive concern: reviewers will accept everything to look "
         "agreeable.",
         "Mitigation: reputation rewards CORRECTLY rejecting bad "
         "papers, not just accepting."),
        ("Naive concern: cabals coordinate to inflate each others' "
         "reputations.",
         "Mitigation: trust-graph-based aggregation. A cabal's "
         "internal mutual praise contributes near-zero weight to "
         "outsiders."),
        ("Naive concern: reviewers refuse hard papers to protect "
         "their stats.",
         "Mitigation: editor visibility into accept/decline rates "
         "for assignments."),
        ("Naive concern: reputation reduces to a single number "
         "(Goodhart).",
         "Mitigation: multi-dimensional reputation (replication "
         "correlation, methodology spotting, error identification, "
         "constructiveness) prevents single-axis gaming."),
        ("These are real concerns; none is unmanageable.",
         "Same dynamics as any reputation system, with extra "
         "engineering."),
    ])
    add_footer(s, 51, TOTAL, BRAND)

    s = content_slide(prs, "Author reputation",
        bullets=[
            ("Symmetric to reviewer reputation.",
             "Authors get reputation from how their work performs over "
             "time. Replication, citation by replicating researchers, "
             "errors, retractions all factor in."),
            ("Replaces h-index.",
             "h-index is a count metric easily gamed. Reputation is a "
             "trust-weighted quality metric."),
            ("Tenure committees can use this.",
             "Today they look at journal prestige proxies. Tomorrow "
             "they could look at predicted-replicate rates."),
            ("Funding agencies can use this.",
             "NIH, NSF, ERC, Wellcome could weight applications by "
             "applicant reputation in the proposed area."),
            ("Cross-domain transparency.",
             "Researcher's reputation in one subfield doesn't "
             "auto-transfer to another. Each field's trust graph is "
             "independent."),
        ])
    add_footer(s, 52, TOTAL, BRAND)

    # Section 8
    s = section_divider(prs, 8, "A Paper's Life Cycle Under This System",
        "Concrete walkthrough: preregistration to supersession.")
    add_footer(s, 53, TOTAL, BRAND)

    s = _im(prs, "The full life cycle, signed end-to-end",
        "chart_lifecycle.png",
        caption="Each milestone produces a signed artifact linked "
                "to the prior milestone.",
        image_h=4.4)
    add_footer(s, 54, TOTAL, BRAND)

    s = content_slide(prs, "T-6 months: preregistration",
        bullets=[
            ("Researcher signs the hypothesis + analysis plan.",
             "Posts to OSF or equivalent."),
            ("Quidnug records the signature with timestamp.",
             "Tamper-evident: any later change to the plan is "
             "visible."),
            ("Pre-analysis power calculation included.",
             "Reviewers and replicators can audit."),
            ("Optional: editorial pre-acceptance via registered "
             "reports.",
             "Decision based on the design, not eventual results."),
            ("Cost.",
             "Roughly the same as filling out a current OSF "
             "preregistration form. Adds ~5 minutes for the "
             "signature step."),
        ])
    add_footer(s, 55, TOTAL, BRAND)

    s = content_slide(prs, "T-3 months: data collection", bullets=[
        ("Each datum is signed at acquisition.",
         "Lab notebook + instrument output produce signed records."),
        ("Bulk data: Merkle root signed.",
         "Individual records can be hashed-and-included; "
         "verifiable inclusion proofs."),
        ("Validation steps signed separately.",
         "Outlier removal, clean-up, format conversion: each is a "
         "node in the provenance chain."),
        ("Lab member identity preserved.",
         "Each contributor's quid is on the record. Important for "
         "credit and for accountability."),
        ("Can be batched.",
         "Sign daily, weekly, or per-experiment. Granularity is "
         "operator choice."),
    ])
    add_footer(s, 56, TOTAL, BRAND)

    s = content_slide(prs, "T-1 month: analysis and preprint",
        bullets=[
            ("Analysis script signed.",
             "Linked to the signed preregistration. Deviations from "
             "preregistered plan are visible (and may be justified, "
             "not necessarily a problem)."),
            ("Outputs (figures, tables) signed.",
             "Linked to the script that produced them. Re-running "
             "the script must produce matching outputs."),
            ("Preprint posted with signed bundle.",
             "Manuscript + script + data + provenance chain."),
            ("ArXiv, bioRxiv, medRxiv compatibility.",
             "Existing preprint servers gain provenance metadata as "
             "a sidecar."),
            ("Public commitment.",
             "From this moment, any change is auditable."),
        ])
    add_footer(s, 57, TOTAL, BRAND)

    s = content_slide(prs, "T: peer review", bullets=[
        ("Editor assigns reviewers.",
         "Editor sees real names; reviewers can choose pseudonymous "
         "publication."),
        ("Each reviewer signs their review.",
         "Linked to the paper, with timestamp and decision."),
        ("Author rebuttal signed.",
         "Linked to specific reviewer comments."),
        ("Editor decision signed.",
         "With reasoning, linked to all reviews + rebuttal."),
        ("Whole exchange becomes the public review chain.",
         "Available to anyone reading the paper, with optional "
         "redaction of reviewer identities."),
    ])
    add_footer(s, 58, TOTAL, BRAND)

    s = content_slide(prs, "T+1 month: published version", bullets=[
        ("Final version signed by all authors.",
         "Linked to the review chain that produced it."),
        ("DOI assigned per usual.",
         "Quidnug provenance metadata accessible via DOI resolver."),
        ("Discoverable via search.",
         "Same Google Scholar / Semantic Scholar / PubMed paths."),
        ("Citable as a unit.",
         "Citation includes the cryptographic root that proves "
         "what is being cited."),
        ("Visible to any future replicator.",
         "Full chain available; no need to email authors for "
         "supplementary materials."),
    ])
    add_footer(s, 59, TOTAL, BRAND)

    s = content_slide(prs, "T+2 years: replication attempt",
        bullets=[
            ("Replication team links their work to the original paper "
             "by quid.",
             ""),
            ("They re-run the analysis using the original signed "
             "script.",
             "If the chain verifies, they have certainty about what "
             "was done."),
            ("They collect new data, sign it, run analysis.",
             "Full provenance for their work too."),
            ("Replication outcome is itself a signed event.",
             "Linked to original paper. Visible in citation graph."),
            ("Reviewer + author reputation updated.",
             "If paper replicates: reviewer/author reputation rises. "
             "If not: declines, automatically."),
        ])
    add_footer(s, 60, TOTAL, BRAND)

    s = content_slide(prs, "T+5 years: superseded by better theory",
        bullets=[
            ("New paper proposes a better explanation.",
             "Links to the original paper as a precursor."),
            ("Original paper is not retracted.",
             "Retraction is for fraud / errors. Supersession is "
             "scientific progress."),
            ("The chain shows the intellectual lineage.",
             "Reader of the new paper sees what came before, what "
             "data was used, what changed."),
            ("Author of the original gets credit for the "
             "contribution.",
             "Reputation reflects 'this work was a useful precursor,' "
             "not 'this work was wrong.'"),
            ("Citation graph captures the actual flow of ideas.",
             "Today: citations are flat counts. Tomorrow: citations "
             "carry semantic weight."),
        ])
    add_footer(s, 61, TOTAL, BRAND)

    s = content_slide(prs, "The total artifact at T+5 years",
        bullets=[
            ("One paper has produced.",
             "Preregistration + raw data + analysis script + figures + "
             "preprint + 4 reviews + rebuttal + editor decision + "
             "published version + replication attempt + supersession."),
            ("All linked, all signed, all verifiable.",
             "Anyone can audit the full lineage."),
            ("Storage cost.",
             "Roughly 100 KB - 10 MB per paper, depending on data "
             "size. Trivial."),
            ("Compute cost to verify.",
             "Sub-second for the full chain."),
            ("This is what a healthy scholarly record looks like.",
             "We have the technology. The question is institutional "
             "will."),
        ])
    add_footer(s, 62, TOTAL, BRAND)

    # Section 9
    s = section_divider(prs, 9, "Institutional Tradeoffs and Rollout",
        "What journals, universities, funders need to do.")
    add_footer(s, 63, TOTAL, BRAND)

    s = content_slide(prs, "What journals must do", bullets=[
        ("Accept signed submission bundles.",
         "Manuscript + provenance chain + signed authorship."),
        ("Issue signed peer reviews.",
         "Reviewer signs review; editor signs decision."),
        ("Publish review chain alongside paper.",
         "Optionally with reviewer identity redacted; the cryptographic "
         "identity remains."),
        ("Adopt PROV-O / Quidnug-compatible metadata.",
         "Sidecar JSON or RDF; publish alongside DOI."),
        ("Cost.",
         "Mostly editorial workflow software. Major publishers "
         "(Wiley, Elsevier, Springer Nature) could deploy in a "
         "year if motivated."),
    ])
    add_footer(s, 64, TOTAL, BRAND)

    s = content_slide(prs, "What universities must do", bullets=[
        ("Issue researcher quids.",
         "Tied to ORCID + institutional email. Researcher takes the "
         "quid with them when they leave."),
        ("Accept reviewer reputation in tenure files.",
         "Auditable review record alongside publication record."),
        ("Reward replication work.",
         "Currently undervalued. Make replications count for tenure "
         "review."),
        ("Mandate provenance for grant-funded work.",
         "When you give a researcher money, require they sign "
         "their data and analyses."),
        ("Update IT for trust-graph integration.",
         "Roughly the same lift as ORCID adoption was 2010-2020."),
    ])
    add_footer(s, 65, TOTAL, BRAND)

    s = content_slide(prs, "What funders must do", bullets=[
        ("Mandate signed deliverables.",
         "NIH, NSF, ERC, Wellcome can require signed data + "
         "analysis as a condition of funding."),
        ("Weight grant applications by applicant reputation.",
         "Reviewer reputation shows which applicants have "
         "previously produced replicable work."),
        ("Fund infrastructure.",
         "Quidnug node hosting at major research universities. "
         "Trivial cost relative to research budgets."),
        ("Coordinate across funders.",
         "International coordination on data-sharing + provenance "
         "standards. NIH already drives some of this; needs "
         "expanding."),
        ("Mandate replication funding.",
         "1-5% of research budgets earmarked for replication "
         "studies."),
    ])
    add_footer(s, 66, TOTAL, BRAND)

    s = content_slide(prs, "Honest tradeoff 1: senior researcher resistance",
        bullets=[
            ("Senior researchers built careers under the current "
             "system.",
             "Some see transparency as a threat to existing "
             "reputation."),
            ("Mitigation.",
             "Voluntary at first. Early adopters demonstrate value. "
             "Generational turnover gradually shifts norms."),
            ("Same as the open-access transition.",
             "Took 15+ years from PLoS founding (2001) to mainstream. "
             "Not impossible, just slow."),
            ("Pseudonymous reviewer identity helps.",
             "Senior reviewers can give honest reviews without "
             "career retaliation. Lowers the bar."),
            ("Generational forcing function.",
             "Young researchers entering the field will demand "
             "infrastructure that protects them from p-hacking and "
             "fraud."),
        ])
    add_footer(s, 67, TOTAL, BRAND)

    s = content_slide(prs, "Honest tradeoff 2: small labs and developing countries",
        bullets=[
            ("Adoption requires technical capacity.",
             "Small labs may lack IT support to deploy signing tools."),
            ("Mitigation.",
             "Quidnug ships open-source SDKs in Python, R, Go. Most "
             "are CLI-thin. Not a programming-skill threshold."),
            ("Institutional hosting.",
             "Universities run trust nodes; researchers consume via "
             "web UIs. Lab-side complexity stays low."),
            ("Cost.",
             "Quidnug is open source and free. No SaaS dependency. "
             "Smaller labs and developing countries do not have to "
             "pay anyone."),
            ("Funder mandate.",
             "If NIH and Wellcome require it, infrastructure follows. "
             "Same pattern as ORCID adoption."),
        ])
    add_footer(s, 68, TOTAL, BRAND)

    s = content_slide(prs, "Honest tradeoff 3: privacy and pseudonymity",
        bullets=[
            ("Some reviews require fully anonymous reviewers.",
             "Junior reviewing senior with controversial paper. "
             "Pseudonymous is sufficient; full anonymity is not."),
            ("Trade-off.",
             "Pseudonymous accountability + safety. Full anonymity "
             "+ no accountability."),
            ("Most fields can move to pseudonymous.",
             "Cryptographic identity persists; real-name exposure "
             "remains under reviewer control."),
            ("Whistle-blower / sensitive cases.",
             "Special handling: editor-only knowledge, encrypted "
             "review storage, no public review chain."),
            ("Default vs special.",
             "Default to pseudonymous-public chain. Allow opt-in "
             "to fully-anonymous review for sensitive cases."),
        ])
    add_footer(s, 69, TOTAL, BRAND)

    s = content_slide(prs, "What this protocol does NOT solve",
        bullets=[
            ("Bad theory.",
             "A wrong theoretical framework can produce verifiable "
             "provenance for verifiable nonsense."),
            ("Bad measurement.",
             "If the instrument is broken, signing the broken "
             "output does not fix it."),
            ("Politicization of science.",
             "Climate denial, vaccine hesitancy: not problems of "
             "review infrastructure."),
            ("Cargo-cult statistics.",
             "Wrong test choice cannot be detected at the substrate "
             "layer. Reviewers and replicators still must do "
             "thinking."),
            ("Funding bias.",
             "Companies funding research that benefits them. "
             "Disclosure helps; substrate doesn't directly address."),
        ])
    add_footer(s, 70, TOTAL, BRAND)

    s = content_slide(prs, "Summary: what changes", bullets=[
        ("Replication crisis becomes auditable.",
         "Replication failures attached to specific authors and "
         "reviewers in a queryable graph."),
        ("Fraud detection moves earlier.",
         "Tessier-Lavigne-style image manipulation flagged at "
         "submission. Stapel-style fabrication impossible without "
         "instrument-level signatures."),
        ("Reviewer quality becomes visible.",
         "Tenure committees can credit substantive reviewing. "
         "Lazy reviewing becomes visible."),
        ("Citation graphs become semantic.",
         "Replication attempts, supersessions, retractions all "
         "linked. Citation count alone replaced with quality-"
         "weighted reputation."),
        ("Public trust in science can be rebuilt.",
         "Cryptographic accountability is harder to dismiss than "
         "'trust us, peer review.'"),
    ])
    add_footer(s, 71, TOTAL, BRAND)

    s = content_slide(prs, "What to do this year (researcher)",
        bullets=[
            ("Preregister your next study.",
             "OSF or AsPredicted. Sign the preregistration if your "
             "institution supports Quidnug; otherwise PGP."),
            ("Open your data.",
             "Zenodo, Dryad, OSF. Make it the default."),
            ("Open your code.",
             "GitHub, GitLab. Tag releases corresponding to paper "
             "submissions."),
            ("Sign your reviews.",
             "Use a stable cryptographic identity. Ask journals to "
             "support pseudonymous-signed review."),
            ("Cite responsibly.",
             "Note when papers you cite have replicated or not."),
        ])
    add_footer(s, 72, TOTAL, BRAND)

    s = content_slide(prs, "What to do this year (institution)",
        bullets=[
            ("Issue researcher quids.",
             "Pilot with the chemistry or psychology department. "
             "Iterate."),
            ("Update tenure guidelines.",
             "Replication work counts. Substantive reviewing counts."),
            ("Pilot signed-submission with one journal you publish "
             "in.",
             "PNAS, eLife, Royal Society Open Science already "
             "experiment."),
            ("Allocate replication budget.",
             "Even 2% of research budget for replication has "
             "outsized signal value."),
            ("Train graduate students.",
             "New researchers entering the field with these tools "
             "as defaults."),
        ])
    add_footer(s, 73, TOTAL, BRAND)

    s = content_slide(prs, "What to do this year (funder)", bullets=[
        ("Mandate signed deliverables in new grants.",
         "Phased: starting in 2027 or 2028."),
        ("Fund infrastructure.",
         "Quidnug node hosting, SDK development, integration with "
         "existing repositories."),
        ("Sponsor pilot replication studies.",
         "$10M-$50M committed annually buys massive credibility "
         "boost."),
        ("Coordinate internationally.",
         "NIH + Wellcome + ERC + JST + NSFC alignment. Avoid "
         "fragmentation."),
        ("Reward early adopters.",
         "Bonus funding for institutions that pilot signed "
         "infrastructure."),
    ])
    add_footer(s, 74, TOTAL, BRAND)

    s = quote_slide(prs,
        "The replication crisis is a measurement of infrastructure "
        "quality, not researcher quality. We can fix the "
        "infrastructure. The technology exists. What we lack is "
        "institutional will.",
        "The takeaway in one sentence",
        title="The takeaway")
    add_footer(s, 75, TOTAL, BRAND)

    s = content_slide(prs, "References (canonical sources)",
        bullets=[
            ("Ioannidis (2005). Why most published research findings "
             "are false. PLOS Medicine.", ""),
            ("Open Science Collaboration (2015). Reproducibility of "
             "psychological science. Science 349(6251).", ""),
            ("Baker (2016). Is there a reproducibility crisis? "
             "Nature 533, 452-454.", ""),
            ("Begley & Ellis (2012). Drug development. Nature 483, "
             "531-533.", ""),
            ("Errington et al (2021). Reproducibility Project: "
             "Cancer Biology. eLife.", ""),
            ("Camerer et al (2016, 2018). Replication studies in "
             "experimental economics + social sciences.", ""),
            ("Simmons, Nelson, Simonsohn (2011). False-positive "
             "psychology. Psychological Science.", ""),
            ("Button, Ioannidis et al (2013). Power failure. Nature "
             "Reviews Neuroscience.", ""),
            ("Freedman, Cockburn, Simcoe (2015). Economics of "
             "reproducibility in preclinical research. PLOS Biology.",
             ""),
            ("Allen & Mehler (2019). Open science challenges, "
             "benefits and tips. PLOS Biology.", ""),
        ])
    add_footer(s, 76, TOTAL, BRAND)

    s = content_slide(prs, "More references", bullets=[
        ("Aczel et al (2021). A billion-dollar donation: estimating "
         "the cost of reviewers' time. Research Integrity and Peer "
         "Review.", ""),
        ("Franco, Malhotra, Simonovits (2014). Publication bias in "
         "the social sciences. Science.", ""),
        ("Simonsohn, Nelson, Simmons (2014). P-curve. JEP General.",
         ""),
        ("Retraction Watch database. retractionwatch.com.", ""),
        ("Companion blog post.",
         "blogs/2026-04-23-reproducibility-crisis-tamper-evident-"
         "peer-review.md."),
        ("Quidnug protocol.",
         "github.com/quidnug/quidnug. Reference SDKs in Python, R, "
         "Go, JavaScript."),
        ("ORCID.", "orcid.org. Researcher identifier compatibility."),
        ("OSF (Open Science Framework).",
         "osf.io. Existing preregistration platform; integration "
         "target."),
    ])
    add_footer(s, 77, TOTAL, BRAND)

    s = content_slide(prs, "Common objections, briefly", bullets=[
        ("'My data is sensitive.'",
         "Hash + Merkle proofs. Sign the chain without exposing the "
         "raw values."),
        ("'My field doesn't have a culture for this.'",
         "Neither did open access in 2001. Cultures change when "
         "incentives change."),
        ("'It will slow me down.'",
         "Net-net it speeds you up: collaborators can verify your "
         "work without your help."),
        ("'What if Quidnug fails as a project?'",
         "The standards and primitives are open. Other "
         "implementations could replace it."),
        ("'I do not have the technical expertise.'",
         "Most workflow is CLI-thin. Universities deploy the nodes; "
         "researchers consume via web UIs."),
    ])
    add_footer(s, 78, TOTAL, BRAND)

    s = quote_slide(prs,
        "We have the technology. What we lack is institutional will.",
        "The 2026 status of scholarly trust infrastructure",
        title="One-line summary")
    add_footer(s, 79, TOTAL, BRAND)

    s = closing_slide(prs,
        "Questions",
        subtitle="Thank you. The hard part starts now.",
        cta="Where does this argument fail in your field?\n\n"
            "What institutional incentive blocks your adoption?\n\n"
            "Which of the four primitives matters most to you?",
        resources=[
            "github.com/quidnug/quidnug",
            "blogs/2026-04-23-reproducibility-crisis-tamper-evident-peer-review.md",
            "Ioannidis (2005). Why most published research findings are false.",
            "OSC (2015). Reproducibility of psychological science. Science.",
            "Baker (2016). Is there a reproducibility crisis? Nature.",
            "Retraction Watch: retractionwatch.com",
            "Open Science Framework: osf.io",
        ])
    add_footer(s, 80, TOTAL, BRAND)

    return prs


if __name__ == "__main__":
    prs = build()
    prs.save(str(OUTPUT))
    print(f"wrote {OUTPUT} ({len(prs.slides)} slides)")
