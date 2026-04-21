"""Clinical Trial Data Integrity deck (~75 slides)."""
import pathlib, sys
HERE = pathlib.Path(__file__).parent
ASSETS = HERE / "assets"
OUTPUT = HERE / "clinical-trial-data-integrity.pptx"
sys.path.insert(0, str(HERE.parent))
from _deck_helpers import (  # noqa: E402
    make_presentation, title_slide, section_divider, content_slide,
    two_col_slide, stat_slide, quote_slide, table_slide, image_slide,
    code_slide, icon_grid_slide, closing_slide, add_notes, add_footer,
    TEAL, CORAL, EMERALD, AMBER, TEXT_MUTED,
)
from pptx.util import Inches  # noqa: E402

BRAND = "Quidnug  \u00B7  Clinical Trial Data Integrity"
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
        "Clinical Trial Data Integrity",
        "How tamper-evident event streams replace ALCOA+ paper-trail "
        "compliance, and why FDA inspectors will prefer them.",
        eyebrow="QUIDNUG  \u00B7  LIFE SCIENCES")
    add_footer(s, 1, TOTAL, BRAND)

    s = stat_slide(prs, "241",
        "FDA 483 data integrity observations in 2024 alone.",
        context="5x growth over a decade. The ALCOA+ paper-era "
                "framework is failing to prevent systematic data "
                "integrity problems.",
        title="Where we are in 2026", stat_color=CORAL)
    add_footer(s, 2, TOTAL, BRAND)

    s = stat_slide(prs, "$2.5M",
        "average cost of remediating an FDA warning letter for data "
        "integrity (Deloitte 2023 industry survey).",
        context="Plus potential delays to approval, revenue loss, "
                "and reputation damage. The economic case for better "
                "substrate is overwhelming.",
        title="The cost of getting this wrong")
    add_footer(s, 3, TOTAL, BRAND)

    s = content_slide(prs, "Agenda", bullets=[
        ("1. ALCOA+ explained.",
         "Origin, principles, electronic-record transposition, trust "
         "gap."),
        ("2. 21 CFR Part 11 and its failure modes.",
         "Key requirements, typical implementations, where they fall "
         "apart."),
        ("3. Major fraud cases.",
         "Ranbaxy, Theranos, Olympus, Valeant, Duke Potti. Common "
         "patterns."),
        ("4. Multi-party trial structure.",
         "8+ parties per trial, current data flow, integrity risks."),
        ("5. Tamper-evident event streams.",
         "Per-subject event chain, cross-party signatures, PHI "
         "handling."),
        ("6. Worked example.",
         "Phase III cardiovascular endpoint adjudication, end-to-end."),
        ("7. Inspector experience.",
         "Current vs substrate-enabled inspection workflows."),
        ("8. Integration and adoption.",
         "EDC systems, compliance mapping, migration path."),
    ])
    add_footer(s, 4, TOTAL, BRAND)

    s = content_slide(prs, "Four claims this talk defends", bullets=[
        ("1. ALCOA+ was designed for paper.",
         "Electronic-record transposition works in principle but "
         "collapses in the details."),
        ("2. 21 CFR Part 11 compliance is theater at most sponsors.",
         "Per-user audit logs inside a system the sponsor controls "
         "is not meaningful tamper evidence."),
        ("3. Multi-party cryptographic signatures change the threat "
         "model.",
         "Sponsor cannot unilaterally rewrite the record."),
        ("4. This is operationally feasible today.",
         "EDC vendors, CROs, sites can adopt in phases. Regulators "
         "will PREFER it once deployed."),
    ])
    add_footer(s, 5, TOTAL, BRAND)

    # Section 1
    s = section_divider(prs, 1, "ALCOA+ Explained",
        "Origin, principles, and the transposition problem.")
    add_footer(s, 6, TOTAL, BRAND)

    s = _im(prs, "The ALCOA+ framework",
        "chart_alcoa.png",
        caption="ALCOA (1990s FDA): Attributable, Legible, "
                "Contemporaneous, Original, Accurate. + (modern ICH): "
                "Complete, Consistent, Enduring, Available.",
        image_h=4.4)
    add_footer(s, 7, TOTAL, BRAND)

    s = content_slide(prs, "Origin story: why ALCOA existed", bullets=[
        ("Coined by Stan Woollen (FDA) in the 1990s.",
         "Articulated data integrity expectations for pharmaceutical "
         "submissions."),
        ("Paper era context.",
         "Lab notebooks, paper CRFs, wet-ink signatures, file cabinets. "
         "Controls were physical: locked cabinets, signed-out pens, "
         "numbered pages."),
        ("ALCOA as mnemonic.",
         "Inspectors could check: was this record attributable, "
         "legible, contemporaneous, original, accurate?"),
        ("Each principle had a paper-era operational meaning.",
         "'Contemporaneous' meant 'recorded at the time of the "
         "observation, not copied later.'"),
        ("Expanded to ALCOA+ in ICH E6(R2) and EMA GMP Annex 11.",
         "Four additional principles for complex multi-site "
         "electronic records."),
    ])
    add_footer(s, 8, TOTAL, BRAND)

    s = content_slide(prs, "The electronic-record transposition", bullets=[
        ("Attributable.",
         "Paper: signature on each page. Electronic: username + "
         "timestamp in audit log."),
        ("Legible.",
         "Paper: readable handwriting. Electronic: renderable "
         "format, accessible indefinitely."),
        ("Contemporaneous.",
         "Paper: date stamp at recording. Electronic: system "
         "timestamp at entry, no post-hoc editing."),
        ("Original.",
         "Paper: source document. Electronic: authoritative version "
         "vs working copies."),
        ("Accurate.",
         "Paper: correction with initialing. Electronic: validated "
         "inputs + edit checks."),
    ])
    add_footer(s, 9, TOTAL, BRAND)

    s = content_slide(prs, "The trust gap", bullets=[
        ("Electronic records have ALL the formal ALCOA properties.",
         "Username? Check. Timestamp? Check. Audit log? Check."),
        ("But.",
         "Every control is internal to the sponsor's system. The "
         "same sponsor runs the database, owns the audit log, "
         "writes the validation procedures."),
        ("Self-attestation at scale.",
         "A sophisticated actor can rewrite the audit log, "
         "backdate entries, manipulate timestamps."),
        ("Inspectors cannot independently verify.",
         "They see what the system shows them. If the system "
         "was manipulated, inspectors see the manipulated version."),
        ("ALCOA in letter. Not in spirit.",
         "Paper-era controls depended on physical impossibility of "
         "certain manipulations. Electronic controls depend on "
         "trusting the system owner."),
    ])
    add_footer(s, 10, TOTAL, BRAND)

    s = quote_slide(prs,
        "Paper-era controls were physical. Electronic-era controls "
        "require cryptographic substrate. We've had the first for "
        "50 years and are still waiting for the second.",
        "The architectural diagnosis",
        title="The architectural diagnosis")
    add_footer(s, 11, TOTAL, BRAND)

    # Section 2
    s = section_divider(prs, 2, "21 CFR Part 11 and Its Failure Modes",
        "The rule that governs electronic records in FDA-regulated "
        "industries.")
    add_footer(s, 12, TOTAL, BRAND)

    s = content_slide(prs, "21 CFR Part 11: key requirements",
        bullets=[
            ("Electronic signatures legally equivalent to handwritten.",
             "Title 21 of the Code of Federal Regulations, Part 11. "
             "Signed into force March 1997."),
            ("Required controls.",
             "Audit trails showing who did what and when. Validation "
             "of electronic systems. Access controls. Time-stamped "
             "records."),
            ("Electronic signature requirements.",
             "Biometric OR password + second factor. Linked to the "
             "signed record. Cannot be copied to another record."),
            ("Validation.",
             "Documented evidence that the system does what it's "
             "supposed to do, consistently."),
            ("Covers any electronic record submitted to FDA.",
             "Clinical trial data, manufacturing batch records, "
             "adverse event reports."),
        ])
    add_footer(s, 13, TOTAL, BRAND)

    s = content_slide(prs, "Typical compliance implementation", bullets=[
        ("Password protection on EDC system.",
         "Monthly password rotation. Second factor usually "
         "email-based, sometimes SMS."),
        ("Audit trail as database table.",
         "Row per change event: user, timestamp, old value, new "
         "value."),
        ("Validation package.",
         "Hundreds of pages of test protocols and evidence. Updated "
         "for each system version."),
        ("Access control.",
         "Role-based: investigator, monitor, sponsor, CRO. Each role "
         "sees different subset of data."),
        ("This is the industry-standard implementation.",
         "Every major EDC vendor ships this. Compliance boxes all "
         "checked."),
    ])
    add_footer(s, 14, TOTAL, BRAND)

    s = content_slide(prs, "Where this fails", bullets=[
        ("All audit trails live in the sponsor's (or vendor's) "
         "database.",
         "Whoever controls the database controls the audit trail."),
        ("Database administrators can modify audit trails.",
         "DBA credentials bypass every application-layer control. "
         "If a DBA is compromised (or coerced, or motivated), "
         "audit records can be altered."),
        ("Backup restoration can overwrite history.",
         "'We had to restore from backup after a system issue.' "
         "Plausible-sounding excuse for losing inconvenient records."),
        ("Vendor of the EDC system has access.",
         "Medidata, Veeva, Oracle run the infrastructure. Their "
         "employees can theoretically access your trial data."),
        ("Inspectors see what the system shows them.",
         "They have no way to independently verify the audit trail "
         "wasn't modified before their visit."),
    ])
    add_footer(s, 15, TOTAL, BRAND)

    s = _im(prs, "FDA data integrity warning letters: 5x growth",
        "chart_fda_letters.png",
        caption="Source: FDA Warning Letter database, filtered for "
                "data integrity observations (21 CFR 11.10 "
                "violations).",
        image_h=4.2)
    add_footer(s, 16, TOTAL, BRAND)

    s = content_slide(prs, "Warning letter examples", bullets=[
        ("Most common observation: 'audit trails were not reviewed.'",
         "Companies have audit trails but don't review them. "
         "Accountability theater without accountability."),
        ("Second most common: 'CAPA for data integrity issue "
         "inadequate.'",
         "Issues discovered by inspectors are then inadequately "
         "remediated. Pattern of non-escalation."),
        ("Third most common: 'retesting without documentation.'",
         "Running tests multiple times until one produces the "
         "desired result. A form of p-hacking on regulatory data."),
        ("Fourth: 'sharing of login credentials.'",
         "Attributability collapses when multiple people use the "
         "same username."),
        ("Fifth: 'backdated entries.'",
         "Recording observations later with a falsified earlier "
         "timestamp. Contemporaneous principle violated."),
    ])
    add_footer(s, 17, TOTAL, BRAND)

    s = content_slide(prs, "GAMP 5 as it actually operates", bullets=[
        ("Good Automated Manufacturing Practice (ISPE).",
         "Validation framework for pharmaceutical software. 5th "
         "edition published 2008, 2nd ed 2022."),
        ("Risk-based approach.",
         "Validation effort proportional to patient safety and data "
         "integrity impact."),
        ("What it requires.",
         "User requirements, functional specifications, design "
         "specifications, IQ/OQ/PQ test protocols, validation "
         "reports."),
        ("What it actually produces.",
         "Thousands of pages of documentation. Regulatory expectation "
         "met. Cost: millions per system."),
        ("What it does not produce.",
         "Guarantees the system wasn't compromised, modified, or "
         "corrupted since validation. That requires ongoing "
         "cryptographic verification."),
    ])
    add_footer(s, 18, TOTAL, BRAND)

    # Section 3
    s = section_divider(prs, 3, "Major Fraud Cases",
        "Five high-profile cases. Common patterns. What would have "
        "changed.")
    add_footer(s, 19, TOTAL, BRAND)

    s = _im(prs, "Five major pharma / medical fraud cases",
        "chart_fraud.png",
        caption="Each case took years to surface and cost billions "
                "in aggregate harm.",
        image_h=4.4)
    add_footer(s, 20, TOTAL, BRAND)

    s = content_slide(prs, "Ranbaxy 2008-2013: the scale case", bullets=[
        ("Indian generic manufacturer, largest in India at the time.",
         ""),
        ("Whistleblower revealed systematic data fabrication.",
         "Stability data invented. Tests not actually performed. "
         "Batches released without verification."),
        ("Scale.",
         "Affected products for decades. Numerous APIs and finished "
         "dosage forms."),
        ("Outcome.",
         "2013: $500M criminal + civil settlement. US import ban on "
         "specific facilities. Ranbaxy sold to Sun Pharma."),
        ("How substrate would have helped.",
         "Independent signed lab records would have made "
         "fabrication detectable. Instruments that sign test "
         "results directly remove the lab technician's ability "
         "to invent data without caught signatures."),
    ])
    add_footer(s, 21, TOTAL, BRAND)

    s = content_slide(prs, "Theranos 2015-2022: the brand case", bullets=[
        ("Elizabeth Holmes's blood-testing startup, valued $9B at peak.",
         ""),
        ("Fraud: Edison device performed only 12 of 200+ advertised "
         "tests.",
         "Other tests secretly run on Siemens competitor machines, "
         "with Theranos-branded results sent back."),
        ("Internal culture of secrecy prevented detection.",
         "Employees who questioned results were isolated or "
         "terminated."),
        ("Outcome.",
         "Theranos dissolved 2018. Holmes 11-year sentence (2022). "
         "Former president Sunny Balwani 13 years."),
        ("How substrate would have helped.",
         "Instrument-signed test results would have made the "
         "Edison's limited capability impossible to obscure. "
         "Regulatory inspectors would have seen the pattern much "
         "earlier."),
    ])
    add_footer(s, 22, TOTAL, BRAND)

    s = content_slide(prs, "Duke Potti 2006-2010: the trial data case",
        bullets=[
            ("Anil Potti at Duke Cancer Institute.",
             "Published chemosensitivity predictions from microarray "
             "analyses in Nature Medicine and elsewhere."),
            ("Started enrolling patients in 3 cancer trials based on "
             "the predictions.",
             "Publications were later shown to have fabricated data."),
            ("Whistleblowers: biostatisticians Keith Baggerly and "
             "Kevin Coombes.",
             "Found the fabrication through independent re-analysis "
             "of raw data the journals required."),
            ("Outcome.",
             "Potti resigned 2010. 11 papers retracted. Trials halted. "
             "Multiple patient lawsuits. Duke settled out of court."),
            ("How substrate would have helped.",
             "If raw data had provenance chain from instrument to "
             "publication, fabrication would be detectable "
             "automatically. Re-analysis is unnecessary if the "
             "chain itself is tamper-evident."),
        ])
    add_footer(s, 23, TOTAL, BRAND)

    s = content_slide(prs, "The common pattern", bullets=[
        ("Internal data controlled by interested party.",
         "Whether sponsor, CRO, or principal investigator."),
        ("Review processes depend on the interested party.",
         "They pick reviewers, provide data to reviewers, "
         "remediate findings."),
        ("External verification is either not possible or very "
         "expensive.",
         "FDA inspections happen, but are infrequent and constrained "
         "by what the system shows."),
        ("Whistleblowers are the primary detection mechanism.",
         "Insider with conscience plus willingness to be retaliated "
         "against is our data integrity safeguard. Not sustainable."),
        ("Multi-party cryptographic signatures change all of this.",
         "No single party can unilaterally rewrite the record. "
         "Verification becomes mechanical, not dependent on "
         "whistleblowers."),
    ])
    add_footer(s, 24, TOTAL, BRAND)

    # Section 4
    s = section_divider(prs, 4, "The Multi-Party Structure of a Clinical Trial",
        "8+ parties. Each with distinct interests. All must agree "
        "on the data.")
    add_footer(s, 25, TOTAL, BRAND)

    s = _im(prs, "Clinical trial: 8+ parties per typical Phase III",
        "chart_trial_parties.png",
        caption="Each party produces or touches trial data. Current "
                "system requires trust in ONE party (sponsor) to "
                "aggregate all data honestly.",
        image_h=4.4)
    add_footer(s, 26, TOTAL, BRAND)

    s = content_slide(prs, "The parties in detail", bullets=[
        ("Sponsor.",
         "Pharmaceutical company funding the trial. Owns the data. "
         "Has most to gain from favorable outcomes."),
        ("CRO (Contract Research Organization).",
         "Pharma outsource. Data management, statistical analysis, "
         "regulatory submissions. Paid by sponsor."),
        ("Clinical site investigator.",
         "Physician running the trial at a specific hospital or "
         "clinic. Enrolls patients, administers interventions, "
         "records outcomes."),
        ("Site monitor (CRA).",
         "Independent verifier from CRO. Visits sites, checks "
         "data entry against source documents."),
        ("Subject (patient).",
         "Signs informed consent, receives interventions, reports "
         "adverse events. Whose data is this?"),
        ("Central lab, IRB, regulator.",
         "Additional parties with specialized roles. Each produces "
         "signed artifacts."),
    ])
    add_footer(s, 27, TOTAL, BRAND)

    s = content_slide(prs, "Current data flow", bullets=[
        ("Subject is examined at site.",
         "Investigator records observation in electronic case report "
         "form (eCRF)."),
        ("Monitor visits site periodically.",
         "Checks eCRF entries against source documents (patient "
         "medical record, lab reports, etc)."),
        ("Data goes into sponsor's EDC system.",
         "Medidata Rave, Veeva Vault, Oracle InForm. Vendor "
         "infrastructure, sponsor configuration."),
        ("Sponsor (or CRO) analyzes.",
         "Statistical analysis, safety reporting, interim analyses, "
         "final reports."),
        ("Submissions to regulator.",
         "Via electronic submissions gateway (FDA ESG, EMA CESP). "
         "Regulator reviews; may conduct inspection."),
    ])
    add_footer(s, 28, TOTAL, BRAND)

    s = content_slide(prs, "Why this structure has integrity risks",
        bullets=[
            ("Sponsor has economic incentive to favorable outcomes.",
             "Billions of dollars hang on Phase III success."),
            ("CRO reports to sponsor.",
             "Not an independent adjudicator; paid by the party "
             "whose product is being tested."),
            ("Sites are also under pressure.",
             "Enrollment targets, deviation minimization, not looking "
             "like a 'difficult site' for future contracts."),
            ("Monitors perform source data verification on a sample.",
             "Usually 10-15%. Most data is not independently "
             "verified."),
            ("Single point of data aggregation.",
             "Sponsor's EDC. If that data is incorrect (accidentally "
             "or deliberately), everything downstream is wrong."),
        ])
    add_footer(s, 29, TOTAL, BRAND)

    s = content_slide(prs, "What multi-party signed attestation changes",
        bullets=[
            ("Each party's contribution is signed independently.",
             "Subject signs consent. Site investigator signs data "
             "entry. Central lab signs results. Each goes into a "
             "shared event stream."),
            ("Cross-verification becomes cryptographic.",
             "Monitor's SDV visit produces a signed attestation: "
             "'I verified entries X, Y, Z match source documents.' "
             "That attestation goes into the chain."),
            ("No party can unilaterally alter records.",
             "Sponsor's signature doesn't override site's signature. "
             "The chain preserves every signatory."),
            ("Regulators see the full chain.",
             "Can verify each signature independently. No need to "
             "trust the sponsor's aggregation."),
            ("Structural property.",
             "ALCOA compliance becomes a mechanical consequence, "
             "not an organizational practice."),
        ])
    add_footer(s, 30, TOTAL, BRAND)

    s = quote_slide(prs,
        "Tamper-evidence requires no party to be trusted. Sponsor, "
        "CRO, site, regulator, patient each sign independently. "
        "The record becomes multi-party verified by construction.",
        "The structural shift",
        title="The structural shift")
    add_footer(s, 31, TOTAL, BRAND)

    # Section 5
    s = section_divider(prs, 5, "Tamper-Evident Event Streams Applied to Trial Data",
        "Per-subject event chain, cross-party signatures, PHI handling.")
    add_footer(s, 32, TOTAL, BRAND)

    s = _im(prs, "Per-subject event stream: chain of custody",
        "chart_data_flow.png", image_h=4.0)
    add_footer(s, 33, TOTAL, BRAND)

    s = content_slide(prs, "Per-subject event stream: structure",
        bullets=[
            ("Each trial subject has a signed event stream.",
             "Identified by a pseudonymous quid (not PHI). Chain "
             "begins at informed consent."),
            ("Events flow into the stream chronologically.",
             "Consent signed, screening visit, randomization, "
             "interventions administered, follow-up visits, "
             "adverse events, endpoint adjudication."),
            ("Each event signed by the observing party.",
             "Investigator for physical exam findings. Nurse for "
             "vitals. Lab for test results. Patient for patient-"
             "reported outcomes."),
            ("Each event references prior events.",
             "Continuous chain. No gaps possible."),
            ("At study end.",
             "Every subject's full trial history is a verified, "
             "tamper-evident record. Available to sponsor, CRO, "
             "regulator independently."),
        ])
    add_footer(s, 34, TOTAL, BRAND)

    s = content_slide(prs, "Cross-party signatures", bullets=[
        ("Source Data Verification (SDV) as signed attestation.",
         "Monitor reviews entries. If they match source, monitor "
         "signs: 'I verified these entries against source on date X.'"),
        ("Discrepancies also signed.",
         "'Entry N says 98.5 F; source says 98.6 F. Discrepancy "
         "logged.'"),
        ("Adjudication committees sign.",
         "Endpoint adjudication panels sign their consensus "
         "determinations. Independent of sponsor."),
        ("IRB continuing reviews sign.",
         "'IRB reviewed this site on date Z. Found: [findings]. "
         "Approval continues.'"),
        ("Sponsor signs aggregations.",
         "Sponsor's statistician signs final analysis. Aggregation "
         "from signed source events; anyone can recompute to "
         "verify."),
    ])
    add_footer(s, 35, TOTAL, BRAND)

    s = content_slide(prs, "Source data verification 2.0", bullets=[
        ("Current SDV.",
         "Monitor visits site. Samples ~10% of entries. Checks "
         "against source documents. Writes report."),
        ("Risk-based SDV (RBM).",
         "Focus on high-risk data points (endpoints, serious AEs). "
         "Reduces cost but still sample-based."),
        ("Substrate-enabled SDV.",
         "Source documents themselves cryptographically signed at "
         "source. Match with EDC entries becomes automated."),
        ("Remote SDV.",
         "If source is signed at instrument / patient record, "
         "monitor's job is reduced to reviewing signatures. "
         "Mostly remote. Site visits only for training + "
         "investigator relationship."),
        ("100% SDV becomes feasible.",
         "Every entry automatically verified against signed source. "
         "No sampling."),
    ])
    add_footer(s, 36, TOTAL, BRAND)

    s = content_slide(prs, "What becomes automatically detectable",
        bullets=[
            ("Fabricated data.",
             "Missing instrument signature = unsignable = unacceptable. "
             "Stapel / Potti-style fabrication becomes structurally "
             "infeasible."),
            ("Backdated entries.",
             "Cryptographic timestamps at entry. Retrospective "
             "modification breaks signature chain."),
            ("Credential sharing.",
             "Each signature links to a specific quid. Login sharing "
             "becomes visible."),
            ("Database tampering.",
             "Chain-hash verification at every read. Any database "
             "modification breaks the chain visibly."),
            ("Statistical anomalies.",
             "Signed event streams are queryable. Benford's-law "
             "analysis, p-curve analysis, ratio tests all work better "
             "on structured signed data."),
        ])
    add_footer(s, 37, TOTAL, BRAND)

    s = content_slide(prs, "Privacy and PHI", bullets=[
        ("Subject identity must be protected.",
         "HIPAA, GDPR, state and country privacy laws apply."),
        ("Pseudonymous quids.",
         "Subject identified by a Quidnug quid, not by name. "
         "Pseudonym mapping stored separately at site."),
        ("PHI stored off-chain.",
         "Encrypted at rest at the site. Signatures cover hashes "
         "of PHI, not the PHI itself."),
        ("Selective disclosure to regulators.",
         "With subject consent or court order, pseudonym can be "
         "linked to identity for specific regulatory queries."),
        ("Provenance without PHI exposure.",
         "Provenance chain is verifiable without revealing protected "
         "health information."),
        ])
    add_footer(s, 38, TOTAL, BRAND)

    # Section 6
    s = section_divider(prs, 6, "Worked Example: Phase III Endpoint Adjudication",
        "A real Phase III cardiovascular trial workflow.")
    add_footer(s, 39, TOTAL, BRAND)

    s = content_slide(prs, "The scenario", bullets=[
        ("Phase III cardiovascular outcomes trial.",
         "Primary endpoint: composite of cardiovascular death, "
         "MI, stroke."),
        ("6,000 subjects enrolled at 185 sites in 23 countries.",
         "4-year follow-up. Hundreds of endpoint events expected."),
        ("Independent adjudication committee.",
         "Cardiologists not affiliated with sponsor review each "
         "potential endpoint event."),
        ("Complex data flow.",
         "Site investigator reports event. Central AE committee "
         "reviews. Adjudication committee classifies. Final "
         "statistical analysis."),
        ("Where integrity matters most.",
         "Each endpoint classification directly affects study "
         "outcome. Adjudication is the highest-stakes judgment in "
         "the trial."),
    ])
    add_footer(s, 40, TOTAL, BRAND)

    s = content_slide(prs, "The endpoint event flow", bullets=[
        ("Subject experiences potential endpoint event.",
         "Patient has chest pain. Reports to site. Investigator "
         "initiates workup."),
        ("Investigator documents + signs.",
         "Signs: 'Subject X reported chest pain at T. Workup showed "
         "Y. Hospitalized for Z hours. Discharge diagnosis: myocardial "
         "infarction.'"),
        ("Central lab processes samples.",
         "Troponin, ECG. Each signs its results. Linked to subject's "
         "event stream."),
        ("Adjudication committee receives blinded case file.",
         "3 cardiologists independently classify. Each signs their "
         "determination."),
        ("Final consensus.",
         "If agreement: signed consensus. If disagreement: "
         "committee meets, resolves, signs reasoning."),
    ])
    add_footer(s, 41, TOTAL, BRAND)

    s = content_slide(prs, "What inspections can verify", bullets=[
        ("Inspector queries subject X's event stream.",
         "Sees investigator signature, lab signatures, adjudication "
         "signatures. Each independently verified."),
        ("Inspector verifies adjudication independence.",
         "Adjudicators are different quids from sponsor. Their "
         "signatures cryptographically prove non-sponsor identity."),
        ("Inspector spot-checks cross-signatures.",
         "Pulls 20 endpoints. Verifies chain for each. If all "
         "verify, confidence is very high. If any don't, targeted "
         "investigation."),
        ("Inspector can query across studies.",
         "'Show me all endpoints adjudicated by this committee across "
         "sponsor studies.' Pattern analysis becomes feasible."),
        ("Compare to current inspections.",
         "Inspector travels to sponsor HQ. Views reports produced by "
         "sponsor. Very limited independent verification possible."),
    ])
    add_footer(s, 42, TOTAL, BRAND)

    s = content_slide(prs, "If something goes wrong", bullets=[
        ("Investigator backdates event.",
         "Timestamp on signature is at signing. Cannot be "
         "retrospectively forged without breaking chain. Auto-"
         "detected."),
        ("Sponsor pressures adjudicator.",
         "Adjudicator's signatures on this trial can be compared "
         "to adjudicator's pattern across other trials. Outliers "
         "visible."),
        ("Site fabricates endpoint.",
         "No lab signatures corresponding to the fabricated event. "
         "Structural impossibility of full fabrication."),
        ("Central lab results manipulated.",
         "Lab's signatures are independent of sponsor's infrastructure. "
         "Requires compromising multiple parties."),
        ("CRO re-interprets data.",
         "Re-interpretation is a new signed event, not a silent "
         "edit. Trail preserved."),
    ])
    add_footer(s, 43, TOTAL, BRAND)

    # Section 7
    s = section_divider(prs, 7, "The Inspector Experience",
        "Current vs substrate-enabled inspection workflows.")
    add_footer(s, 44, TOTAL, BRAND)

    s = _im(prs, "Current vs substrate-enabled inspection",
        "chart_inspection.png", image_h=3.8)
    add_footer(s, 45, TOTAL, BRAND)

    s = content_slide(prs, "Current inspection workflow", bullets=[
        ("Pre-inspection notification.",
         "FDA schedules visit to sponsor (or CRO). Typically "
         "2-4 weeks advance notice. Sponsor prepares."),
        ("On-site visit.",
         "2-5 days at sponsor HQ. Sometimes additional site visits. "
         "Expensive for inspectors; limits frequency."),
        ("Document requests.",
         "Inspector asks for specific records. Sponsor produces. "
         "Inspector reviews."),
        ("Discussion.",
         "Inspector asks questions. Sponsor explains processes. "
         "Inspector takes notes."),
        ("Outcome.",
         "483 observations issued if deficiencies found. Sponsor "
         "responds. Potentially followed by Warning Letter."),
        ("Bottleneck.",
         "Very person-intensive. FDA can only inspect a fraction of "
         "trials per year."),
    ])
    add_footer(s, 46, TOTAL, BRAND)

    s = content_slide(prs, "Substrate-enabled inspection workflow",
        bullets=[
            ("Continuous access to trial event streams.",
             "FDA has permissions on the trust network to query any "
             "signed artifact. No pre-scheduled visit needed."),
            ("Automated anomaly detection.",
             "FDA runs queries: 'Show me any trials with signature "
             "chain breaks.' 'Show me any adjudication pattern "
             "outliers.' Triaged automatically."),
            ("Targeted inspection for exceptions.",
             "Only trials flagged by automated analysis get on-site "
             "visits. 10x more trials can be monitored with same "
             "inspector headcount."),
            ("On-site focus shifts.",
             "From 'verify the data' to 'verify operations produce "
             "the expected data.' Training, procedures, culture."),
            ("Continuous monitoring.",
             "No waiting for annual inspection. Issues visible "
             "within days of occurrence."),
        ])
    add_footer(s, 47, TOTAL, BRAND)

    s = content_slide(prs, "What inspectors can do that they couldn't",
        bullets=[
            ("Cross-trial pattern detection.",
             "'Does this adjudicator consistently favor one sponsor?' "
             "Answerable now."),
            ("Cross-sponsor comparison.",
             "'Is this sponsor's AE rate systematically lower than "
             "peers?' Comparable signed data across sponsors."),
            ("Site-level quality scoring.",
             "'What's the data integrity profile of this site?' Based "
             "on signature patterns across many trials."),
            ("Real-time safety monitoring.",
             "Aggregate adverse event signatures across trials. Early "
             "safety signals visible within days."),
            ("Retrospective investigations.",
             "Once tamper-evidence is standard, historical questions "
             "become answerable."),
        ])
    add_footer(s, 48, TOTAL, BRAND)

    s = content_slide(prs, "What inspectors can still do that they "
                      "always could",
        bullets=[
            ("On-site interviews.",
             "Culture, training, procedures. Not verified by "
             "signatures."),
            ("Direct source document review.",
             "For trials not yet on substrate, traditional "
             "inspection continues."),
            ("For-cause inspections.",
             "Reactive inspections in response to complaints still "
             "happen. Substrate gives inspectors more data to work "
             "with."),
            ("International coordination.",
             "FDA, EMA, PMDA inspections can now share signed "
             "verification data. Consistent regulatory outcomes."),
            ("Formal sanctions.",
             "Warning letters, consent decrees, import bans. Same "
             "tools, informed by better data."),
        ])
    add_footer(s, 49, TOTAL, BRAND)

    s = content_slide(prs, "The regulator's position", bullets=[
        ("FDA has signaled support for better data integrity "
         "mechanisms.",
         "Multiple guidance documents 2016-2024 emphasizing "
         "ALCOA+ principles."),
        ("EMA is aligned.",
         "EMA Annex 11 updates, ICH E6(R3) expected 2026, all "
         "pointing toward enhanced electronic integrity."),
        ("FDA expects tamper-evidence as operational practice.",
         "Warning letters cite failures. Explicit technology "
         "recommendations not typically included; outcomes are."),
        ("Substrate adoption is aligned with regulatory direction.",
         "Not ahead of it; not behind it. Meets the regulatory "
         "standard better than current practice."),
        ("Regulators will PREFER substrate-enabled trials.",
         "Less inspector burden. Better data quality. Same "
         "regulatory authority."),
    ])
    add_footer(s, 50, TOTAL, BRAND)

    # Section 8
    s = section_divider(prs, 8, "Integration and Adoption",
        "EDC systems, compliance mapping, migration path.")
    add_footer(s, 51, TOTAL, BRAND)

    s = _im(prs, "Integration with existing clinical trial infrastructure",
        "chart_ecosystem.png", image_h=4.0)
    add_footer(s, 52, TOTAL, BRAND)

    s = content_slide(prs, "EDC systems: Medidata, Veeva, Oracle, Castor",
        bullets=[
            ("Medidata Rave.",
             "Dominant EDC. Substrate integration via API layer: "
             "Quidnug signatures for every eCRF save operation."),
            ("Veeva Vault Clinical.",
             "Rising enterprise option. Native API supports "
             "signature injection. Faster path to integration."),
            ("Oracle InForm.",
             "Legacy dominant in large-pharma. Integration via "
             "configurable workflow extensions."),
            ("Castor (emerging).",
             "Cloud-native. Built for flexibility. Likely fastest "
             "to adopt signatures natively."),
            ("Integration principle.",
             "Substrate sits BELOW the EDC, not instead of. Existing "
             "workflows unchanged; cryptographic evidence layer "
             "added underneath."),
        ])
    add_footer(s, 53, TOTAL, BRAND)

    s = _im(prs, "Compatibility with existing compliance frameworks",
        "chart_compliance.png", image_h=4.0)
    add_footer(s, 54, TOTAL, BRAND)

    s = content_slide(prs, "Migration path: 5-year roadmap", bullets=[
        ("Year 1. Pilot on a single Phase I trial.",
         "Single sponsor, single CRO, single site. 50-100 subjects. "
         "Prove operational feasibility."),
        ("Year 2. Expand to Phase II.",
         "Multiple sites, still one sponsor. Demonstrate multi-site "
         "workflow."),
        ("Year 3. Run parallel traditional + substrate for Phase III.",
         "Both systems operational. Compare inspection outcomes, "
         "cost, data quality."),
        ("Year 4. First substrate-only Phase III submissions.",
         "FDA accepts. Industry watches outcomes."),
        ("Year 5. Broader adoption.",
         "Major CROs integrate. Top 20 pharma evaluate. Mainstream "
         "consideration."),
    ])
    add_footer(s, 55, TOTAL, BRAND)

    s = _im(prs, "Projected adoption curves",
        "chart_adoption.png", image_h=4.0)
    add_footer(s, 56, TOTAL, BRAND)

    s = _im(prs, "Economics: substrate vs current data management",
        "chart_economics.png", image_h=4.0)
    add_footer(s, 57, TOTAL, BRAND)

    s = content_slide(prs, "Economic breakdown", bullets=[
        ("Current large-trial data management.",
         "~$8.5M per large Phase III (industry average, Deloitte 2023)."),
        ("Substrate-enabled.",
         "~$5.5M per trial (projected, based on automated SDV + "
         "reduced monitoring)."),
        ("Savings.",
         "~$3M per large trial from data management alone."),
        ("Avoided warning letter costs.",
         "Average $2.5M per incident. Substrate reduces incidents."),
        ("Accelerated regulatory approval.",
         "Faster inspections → faster approvals → earlier "
         "revenue. Per blockbuster drug: $50-500M of earlier "
         "revenue per month of acceleration."),
    ])
    add_footer(s, 58, TOTAL, BRAND)

    s = content_slide(prs, "Honest tradeoff 1: organizational change",
        bullets=[
            ("Adopters face large process change.",
             "CROs, sites, sponsors all change workflows."),
            ("Training burden.",
             "Thousands of investigators, monitors, data managers "
             "need to learn new tools."),
            ("Legacy system coexistence.",
             "Existing trials continue under old system. New trials "
             "adopt. Takes years for full transition."),
            ("Mitigation.",
             "Phased rollout. Reference implementations. Vendor "
             "integration support."),
            ("This is standard for major industry technology "
             "transitions.",
             "Comparable to EMR adoption 2009-2020 or electronic "
             "prescribing adoption."),
        ])
    add_footer(s, 59, TOTAL, BRAND)

    s = content_slide(prs, "Honest tradeoff 2: regulatory validation",
        bullets=[
            ("FDA hasn't yet explicitly endorsed a substrate-based "
             "approach.",
             "Current expectation: 21 CFR Part 11 compliance via "
             "standard mechanisms."),
            ("Industry coordination needed.",
             "ISPE, DIA, FDA-industry working groups can develop "
             "guidance for substrate adoption."),
            ("Pioneer risk.",
             "First adopters face regulatory uncertainty. They must "
             "over-document compliance."),
            ("Mitigation.",
             "Run parallel systems for pioneer trials (both "
             "substrate and traditional). Regulatory safety net."),
            ("Long-term.",
             "Guidance will catch up once demonstration projects "
             "establish substrate reliability."),
        ])
    add_footer(s, 60, TOTAL, BRAND)

    s = content_slide(prs, "Honest tradeoff 3: vendor consolidation",
        bullets=[
            ("EDC market is concentrated.",
             "Medidata, Veeva, Oracle dominate. Substrate adoption "
             "depends on their participation."),
            ("Risk: vendor lock-in.",
             "If one vendor dominates substrate integration, they "
             "dictate implementation."),
            ("Mitigation: open standards.",
             "Quidnug protocol is open. Multiple vendors can "
             "implement compatibly."),
            ("Smaller EDC options (Castor, OpenClinica).",
             "More flexible; faster to adopt. Provide market "
             "pressure on larger vendors."),
            ("Over time.",
             "Substrate adoption across vendors ensures "
             "interoperability."),
        ])
    add_footer(s, 61, TOTAL, BRAND)

    s = content_slide(prs, "Honest tradeoff 4: PHI handling complexity",
        bullets=[
            ("Clinical trial data includes PHI.",
             "HIPAA, GDPR, local regulations. Cryptographic "
             "substrate must handle."),
            ("Pseudonymous identifiers partially solve.",
             "Quid replaces subject ID. Mapping stored separately."),
            ("Cross-border data flows.",
             "GDPR Article 44+ restricts EU subject data outside EU. "
             "Substrate deployment must respect this."),
            ("Mitigation.",
             "Data locality: PHI stored at site's jurisdiction. "
             "Signatures cover hashes, not raw PHI."),
            ("Selective disclosure.",
             "PHI revealed only to authorized parties (site, specific "
             "regulators with legal authority)."),
        ])
    add_footer(s, 62, TOTAL, BRAND)

    s = content_slide(prs, "What this protocol does NOT solve", bullets=[
        ("Bad science.",
         "Wrong hypothesis, poorly-designed trial: substrate signs "
         "the bad science honestly."),
        ("Selection bias in trial design.",
         "Substrate doesn't address inclusion/exclusion criteria, "
         "randomization quality, blinding effectiveness."),
        ("Statistical analysis errors.",
         "Substrate verifies data. Doesn't verify analysis choice."),
        ("Incomplete adverse event reporting.",
         "Investigator didn't report an AE: substrate doesn't "
         "conjure the missing report."),
        ("Fundamental misunderstanding of endpoints.",
         "If the primary endpoint is the wrong measure, substrate "
         "doesn't fix."),
    ])
    add_footer(s, 63, TOTAL, BRAND)

    s = content_slide(prs, "Summary: the four claims revisited",
        bullets=[
            ("1. ALCOA+ was designed for paper.",
             "Electronic transposition works in principle but has "
             "trust-gap problems."),
            ("2. 21 CFR Part 11 compliance is theater at most "
             "sponsors.",
             "Internal audit logs inside systems the sponsor controls "
             "are inadequate tamper-evidence."),
            ("3. Multi-party cryptographic signatures change the "
             "threat model.",
             "No party can unilaterally rewrite the record. Fraud "
             "becomes structurally harder."),
            ("4. This is operationally feasible today.",
             "EDC integration, compliance mapping, migration paths "
             "all exist. Adoption is the question, not "
             "implementation."),
        ])
    add_footer(s, 64, TOTAL, BRAND)

    s = content_slide(prs, "What to do this year (sponsor)", bullets=[
        ("Pilot on a Phase I trial.",
         "Small scale, single site. Measure: data quality, "
         "inspection readiness, operational cost."),
        ("Engage your EDC vendor.",
         "Push for substrate integration timeline."),
        ("Train data management team.",
         "Understand signature workflows, chain verification."),
        ("Prepare for regulatory dialogue.",
         "FDA pre-submission meetings on substrate approach."),
        ("Coordinate with other sponsors.",
         "Industry consortium for shared guidance development."),
    ])
    add_footer(s, 65, TOTAL, BRAND)

    s = content_slide(prs, "What to do this year (CRO)", bullets=[
        ("Develop substrate-aware monitoring services.",
         "Automated SDV tools. Remote chain verification "
         "capabilities."),
        ("Pilot with willing sponsors.",
         "Small trials, low risk. Build case studies."),
        ("Integrate with Quidnug SDKs.",
         "Python, JavaScript, Go. Build internal tooling."),
        ("Train CRAs on new workflows.",
         "Less site-visit volume, more remote verification."),
        ("Position as differentiation.",
         "Offer substrate-enabled trials as premium service."),
    ])
    add_footer(s, 66, TOTAL, BRAND)

    s = content_slide(prs, "What to do this year (regulator)", bullets=[
        ("Issue guidance on substrate acceptability.",
         "FDA draft guidance on cryptographic evidence for 21 CFR "
         "Part 11 compliance."),
        ("Participate in industry pilots.",
         "Observer role at pilot trial inspections. Build "
         "institutional knowledge."),
        ("Update inspector training.",
         "Inspectors need to verify cryptographic chains, not just "
         "read audit logs."),
        ("International coordination.",
         "FDA, EMA, PMDA alignment on substrate acceptability."),
        ("Recognize the substrate advantage.",
         "Substrate-enabled trials require less inspection burden. "
         "Make this explicit in guidance."),
    ])
    add_footer(s, 67, TOTAL, BRAND)

    s = content_slide(prs, "What to do this year (EDC vendor)", bullets=[
        ("Add Quidnug signing at eCRF save.",
         "API-level integration. Thin layer above existing workflow."),
        ("Include chain verification in audit exports.",
         "Inspector-friendly output."),
        ("Partner with Quidnug + pilot sponsors.",
         "Co-develop reference implementation."),
        ("Support selective disclosure for PHI.",
         "Signatures over hashes; PHI stays off-chain."),
        ("Publish integration timeline.",
         "Customers planning substrate trials need visibility into "
         "vendor support."),
    ])
    add_footer(s, 68, TOTAL, BRAND)

    s = content_slide(prs, "What success looks like in 2031",
        bullets=[
            ("Majority of new Phase III trials substrate-enabled.",
             "Major pharma, major CROs, major EDCs all aligned."),
            ("Routine cross-party verification.",
             "Site, CRO, sponsor, regulator each independently "
             "verify trial data. No single party trust required."),
            ("Data integrity warning letters reduced 50%+.",
             "Substrate makes entire classes of issues structurally "
             "impossible."),
            ("Inspection frequency doubled.",
             "Same FDA headcount can monitor 2x more trials with "
             "substrate."),
            ("Trust in clinical trial data restored.",
             "Post-Theranos, post-Potti, post-Ranbaxy: verifiable "
             "trial integrity rebuilt."),
        ])
    add_footer(s, 69, TOTAL, BRAND)

    s = content_slide(prs, "References", bullets=[
        ("21 CFR Part 11. Electronic Records; Electronic Signatures.",
         "Title 21 Code of Federal Regulations, Part 11."),
        ("FDA Guidance for Industry: Data Integrity and Compliance "
         "With CGMP (Dec 2018).",
         ""),
        ("ICH E6(R2), E6(R3). Good Clinical Practice.",
         "International Council for Harmonisation."),
        ("EMA Annex 11. Computerised Systems.",
         "EudraLex Volume 4 (GMP)."),
        ("ISPE GAMP 5, 2nd edition (2022).",
         "Risk-Based Approach to Compliant GxP Computerized Systems."),
        ("Ranbaxy US DOJ Press Release (May 2013).",
         "Largest drug safety case in history at the time."),
        ("Senate Permanent Subcommittee on Theranos testimony (2018).",
         ""),
        ("Baggerly and Coombes on Duke Potti (2009).",
         "Annals of Applied Statistics."),
    ])
    add_footer(s, 70, TOTAL, BRAND)

    s = content_slide(prs, "More references", bullets=[
        ("FDA Warning Letter database.",
         "fda.gov/inspections-compliance-enforcement-and-criminal-"
         "investigations/warning-letters."),
        ("Clinical Trials Transformation Initiative (CTTI).",
         "Cross-industry initiative on trial efficiency."),
        ("TransCelerate BioPharma.",
         "Industry consortium for trial acceleration."),
        ("Companion blog post.",
         "blogs/2026-04-26-clinical-trial-data-integrity.md."),
        ("Quidnug protocol.",
         "github.com/quidnug/quidnug."),
    ])
    add_footer(s, 71, TOTAL, BRAND)

    s = content_slide(prs, "Common objections, briefly", bullets=[
        ("'This is too disruptive to workflow.'",
         "Substrate sits below EDC. Existing workflows unchanged. "
         "Cryptographic layer added underneath."),
        ("'FDA won't accept it.'",
         "FDA guidance emphasizes ALCOA+. Substrate exceeds "
         "requirements. Early adopter discussion already underway."),
        ("'Our EDC doesn't support it.'",
         "Integration via API. All major EDCs exposing necessary "
         "hooks."),
        ("'It's too expensive.'",
         "Long-term, saves $3M+ per trial. Short-term investment "
         "amortizes over 2-3 trials."),
        ("'What about existing trials?'",
         "Legacy trials continue current approach. New trials "
         "gradually adopt."),
    ])
    add_footer(s, 72, TOTAL, BRAND)

    s = quote_slide(prs,
        "ALCOA+ was designed for paper. Multi-party cryptographic "
        "signatures are what ALCOA+ requires in 2026. The technology "
        "exists. What we need is institutional momentum.",
        "The one-line summary",
        title="One-line summary")
    add_footer(s, 73, TOTAL, BRAND)

    s = content_slide(prs, "Next steps", bullets=[
        ("This week. Assess your current 21 CFR Part 11 compliance "
         "gaps.",
         ""),
        ("This month. Read FDA data integrity guidance + relevant "
         "warning letters.",
         ""),
        ("This quarter. Pilot Quidnug substrate on one "
         "Phase I trial.",
         ""),
        ("This year. Cross-organizational working group "
         "(industry + FDA observers).",
         ""),
        ("Next year. First substrate-enabled Phase II submissions.",
         ""),
    ])
    add_footer(s, 74, TOTAL, BRAND)

    s = closing_slide(prs,
        "Questions",
        subtitle="Thank you. The substrate awaits adoption.",
        cta="Where does the substrate architecture fail in your "
            "trial context?\n\nWhich EDC or regulatory constraint "
            "matters most?\n\nWhat would your pilot look like?",
        resources=[
            "github.com/quidnug/quidnug",
            "blogs/2026-04-26-clinical-trial-data-integrity.md",
            "FDA Data Integrity Guidance (2018)",
            "21 CFR Part 11",
            "ICH E6(R2), ICH E6(R3)",
            "ISPE GAMP 5 2nd edition (2022)",
            "Clinical Trials Transformation Initiative",
        ])
    add_footer(s, 75, TOTAL, BRAND)

    return prs


if __name__ == "__main__":
    prs = build()
    prs.save(str(OUTPUT))
    print(f"wrote {OUTPUT} ({len(prs.slides)} slides)")
