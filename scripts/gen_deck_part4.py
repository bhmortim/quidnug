"""Slides 64–86: Elections, Credit, Cross-industry, Comparison, Getting started, Close."""
from pptx.util import Inches, Pt
from pptx.enum.text import PP_ALIGN, MSO_ANCHOR
from gen_deck_core import (
    slide_section, slide_content, slide_dark, slide_quote,
    set_bg, text_box, bullets, rect, rounded, hexagon, oval, shape_text,
    arrow, notes, card, chip, numbered_circle, table_rows, blank,
    MIDNIGHT, NAVY, TEAL, AMBER, ICE, CLOUD, SLATE, WHITE, GREEN, RED,
    SOFT_TEAL, SOFT_AMBER, H_FONT, B_FONT, CODE_FONT, hex_watermark, page_chrome
)


def build_part4(prs, start_page, total):
    p = start_page

    # ---------- Elections overview ----------
    s = slide_content(prs, "Elections — the most detailed use case", kicker="GOVERNMENT — USE CASE 10", page=p, total=total)
    # 5 components that Quidnug replaces
    rounded(s, Inches(0.6), Inches(1.85), Inches(12.1), Inches(3), ICE, corner=0.05)
    text_box(s, Inches(0.85), Inches(2.0), Inches(11.6), Inches(0.4),
             "WHAT QUIDNUG REPLACES IN A TYPICAL ELECTION",
             font=B_FONT, size=11, color=SLATE, bold=True)
    replacements = [
        ("Voter registration DB", "Registration trust edges", TEAL),
        ("Printed / electronic poll book", "Per-precinct domain query", NAVY),
        ("Voting machine firmware", "Open-source booth app", AMBER),
        ("Proprietary tabulator", "Any Quidnug node query", TEAL),
        ("Chain-of-custody paper", "Paper + digital cross-verify", NAVY),
    ]
    for i, (old, new, col) in enumerate(replacements):
        x = Inches(0.85 + (i % 5) * 2.4)
        y = Inches(2.6)
        rounded(s, x, y, Inches(2.28), Inches(2.2), col, corner=0.05)
        text_box(s, x + Inches(0.15), y + Inches(0.15), Inches(2), Inches(0.5),
                 "BEFORE", font=B_FONT, size=9,
                 color=(MIDNIGHT if col == AMBER else CLOUD), italic=True)
        text_box(s, x + Inches(0.15), y + Inches(0.45), Inches(2), Inches(0.75),
                 old, font=H_FONT, size=12,
                 color=(MIDNIGHT if col == AMBER else WHITE),
                 bold=True, line_spacing=1.2)
        rect(s, x + Inches(0.15), y + Inches(1.25), Inches(2), Inches(0.03),
             WHITE)
        text_box(s, x + Inches(0.15), y + Inches(1.35), Inches(2), Inches(0.3),
                 "WITH QUIDNUG", font=B_FONT, size=9,
                 color=(MIDNIGHT if col == AMBER else CLOUD), italic=True)
        text_box(s, x + Inches(0.15), y + Inches(1.6), Inches(2), Inches(0.55),
                 new, font=B_FONT, size=11,
                 color=(MIDNIGHT if col == AMBER else WHITE),
                 bold=True, line_spacing=1.2)

    # Design principles
    rounded(s, Inches(0.6), Inches(5.05), Inches(12.1), Inches(1.9), MIDNIGHT, corner=0.05)
    text_box(s, Inches(0.85), Inches(5.2), Inches(11.6), Inches(0.45),
             "NON-NEGOTIABLE DESIGN PRINCIPLES",
             font=H_FONT, size=14, color=TEAL, bold=True)
    principles = [
        ("Secret ballot", "No one can correlate vote to voter"),
        ("Universal verifiability", "Any citizen recounts via query"),
        ("Individual verifiability", "You can verify your own vote"),
        ("Paper-ballot parity", "Every digital vote has paper equivalent"),
        ("One-voter-one-vote", "Cryptographic double-vote prevention"),
        ("BYO identity", "Voter generates own quid"),
    ]
    for i, (h, b) in enumerate(principles):
        x = Inches(0.85 + (i % 3) * 4.1)
        y = Inches(5.75 + (i // 3) * 0.6)
        oval(s, x, y + Inches(0.08), Inches(0.25), Inches(0.25), TEAL)
        text_box(s, x + Inches(0.35), y, Inches(3.5), Inches(0.35),
                 h, font=B_FONT, size=11, color=WHITE, bold=True)
        text_box(s, x + Inches(0.35), y + Inches(0.3), Inches(3.5), Inches(0.3),
                 b, font=B_FONT, size=10, color=CLOUD, italic=True)
    notes(s, """TIMING: 1.5 minutes.

WHAT TO SAY:
Elections is the most detailed use case in the repo — four files totaling about three thousand lines covering every aspect: registration, poll books, ballot issuance, voting, counting, recount, audit, threat model. Five components get replaced by Quidnug. Voter registration database becomes a set of signed trust edges. Poll books become per-precinct queries. Voting machines become open-source booth apps. Proprietary tabulators become any Quidnug node running the tally query. Chain of custody is paper plus digital cross-verify. Six non-negotiable design principles: secret ballot, universal verifiability, individual verifiability, paper-ballot parity, cryptographic double-vote prevention, and bring-your-own voter identity. Everything in the design flows from these six.

KEY POINTS:
• 5 components replaced
• 6 non-negotiable principles
• Most detailed use case in the repo
• 3000+ lines of design across 4 files

TRANSITION:
Let's walk through key parts. Registration first.""")
    p += 1

    # ---------- BYO voter quid registration ----------
    s = slide_content(prs, "Voter registration — bring your own quid", kicker="ELECTIONS: REGISTRATION", page=p, total=total)
    # Flow
    rounded(s, Inches(0.6), Inches(1.85), Inches(12.1), Inches(3.4), ICE, corner=0.05)
    text_box(s, Inches(0.85), Inches(2.0), Inches(11.6), Inches(0.4),
             "REGISTRATION FLOW",
             font=B_FONT, size=11, color=SLATE, bold=True)
    flow = [
        ("Voter generates\nquid on device", "offline, own phone", TEAL),
        ("Visit DMV / portal\nwith quid ID + ID", "identity verification", NAVY),
        ("Registrar issues\ntrust edge", "signed by authority", AMBER),
        ("Edge propagates\nvia push gossip", "≤ 1 minute", TEAL),
        ("Anyone can\nverify you're registered", "public chain query", GREEN),
    ]
    for i, (h, sub, col) in enumerate(flow):
        x = Inches(0.85 + i * 2.42)
        y = Inches(2.55)
        rounded(s, x, y, Inches(2.3), Inches(2.4), col, corner=0.08)
        numbered_circle(s, x + Inches(0.85), y + Inches(0.15), 0.55, i + 1,
                        fill=WHITE, color=MIDNIGHT, size=18)
        text_box(s, x + Inches(0.1), y + Inches(0.8), Inches(2.1), Inches(0.9),
                 h, font=H_FONT, size=12,
                 color=(MIDNIGHT if col == AMBER else WHITE), bold=True,
                 align=PP_ALIGN.CENTER, line_spacing=1.2)
        text_box(s, x + Inches(0.1), y + Inches(1.8), Inches(2.1), Inches(0.5),
                 sub, font=B_FONT, size=10,
                 color=(MIDNIGHT if col == AMBER else CLOUD),
                 italic=True, align=PP_ALIGN.CENTER)
        if i < len(flow) - 1:
            arrow(s, x + Inches(2.3), y + Inches(1.1),
                  x + Inches(2.42), y + Inches(1.1),
                  color=SLATE, width=1.5)

    # Key property
    rounded(s, Inches(0.6), Inches(5.45), Inches(12.1), Inches(1.5), MIDNIGHT, corner=0.05)
    text_box(s, Inches(0.85), Inches(5.6), Inches(11.6), Inches(0.45),
             "WHY BYO-QUID MATTERS", font=H_FONT, size=13, color=TEAL, bold=True)
    text_box(s, Inches(0.85), Inches(6.05), Inches(11.6), Inches(0.9),
             "The authority CANNOT assign a voter a quid it controls — because the voter generated the keypair themselves. Any attempt by the authority to \"pre-register\" someone produces a quid whose private key isn't on the voter's device, and the voter can detect this instantly.",
             font=B_FONT, size=12, color=CLOUD, line_spacing=1.35)
    notes(s, """TIMING: 1.5 minutes.

WHAT TO SAY:
Registration flow. Step 1, voter generates their own voter registration quid locally on their device — phone app, hardware token, voter kiosk. Their private key never leaves them. Step 2, they visit the DMV or use the online portal, present their quid ID plus ID documents. Step 3, the registrar does standard identity verification and issues a signed trust edge — from the election authority to the voter's quid, in the registration domain, with their precinct and party affiliation as attributes. Step 4, push gossip propagates that edge to every precinct node within a minute. Step 5, anyone can verify on the public chain that the voter is registered. The critical property: the authority CANNOT assign a voter a quid it secretly controls, because the voter generated the keypair. Any pre-registration attempt by the authority produces a quid whose private key isn't on the voter's device — which the voter can detect instantly.

KEY POINTS:
• Voter generates own quid
• Registrar issues signed trust edge
• Public chain query verifies
• Authority can't assign attacker-controlled quids

TRANSITION:
Now the hard part — ballot secrecy.""")
    p += 1

    # ---------- Blind-signature ballot issuance ----------
    s = slide_content(prs, "Ballot issuance — blind signatures for anonymity", kicker="ELECTIONS: THE CRITICAL TRICK", page=p, total=total)
    # Top: the problem
    rounded(s, Inches(0.6), Inches(1.85), Inches(12.1), Inches(1.3), MIDNIGHT, corner=0.05)
    text_box(s, Inches(0.85), Inches(2.0), Inches(11.6), Inches(0.45),
             "THE PROBLEM", font=B_FONT, size=11, color=AMBER, bold=True)
    text_box(s, Inches(0.85), Inches(2.45), Inches(11.6), Inches(0.7),
             "Voter is publicly checked in. Now their vote must be (1) authenticated as eligible AND (2) unlinkable to their voter identity. These are contradictory unless we use a special primitive.",
             font=B_FONT, size=12.5, color=CLOUD, line_spacing=1.3)

    # Blind signature metaphor
    rounded(s, Inches(0.6), Inches(3.3), Inches(12.1), Inches(3.35), ICE, corner=0.05)
    text_box(s, Inches(0.85), Inches(3.4), Inches(11.6), Inches(0.35),
             "BLIND SIGNATURE — THE CARBON-PAPER ENVELOPE METAPHOR",
             font=B_FONT, size=11, color=SLATE, bold=True)
    steps = [
        ("1", "Voter's device\ngenerates\nfresh ballot quid\n(anonymous)", TEAL),
        ("2", "Voter blinds\nthe ID with\nsecret factor r", NAVY),
        ("3", "Authority\nsigns the\nblinded commitment", AMBER),
        ("4", "Voter unblinds\n→ signature on\nballot quid ID", TEAL),
        ("5", "Ballot quid\nvotes via\ntrust edge\nto candidate", GREEN),
    ]
    for i, (num, desc, col) in enumerate(steps):
        x = Inches(0.85 + i * 2.42)
        y = Inches(3.85)
        rounded(s, x, y, Inches(2.3), Inches(2.5), col, corner=0.08)
        numbered_circle(s, x + Inches(0.85), y + Inches(0.12),
                        0.55, num, fill=WHITE, color=MIDNIGHT, size=18)
        text_box(s, x + Inches(0.1), y + Inches(0.75), Inches(2.1), Inches(1.7),
                 desc, font=B_FONT, size=10.5,
                 color=(MIDNIGHT if col == AMBER else WHITE), bold=True,
                 align=PP_ALIGN.CENTER, line_spacing=1.3)
        if i < len(steps) - 1:
            arrow(s, x + Inches(2.3), y + Inches(1.25),
                  x + Inches(2.42), y + Inches(1.25),
                  color=SLATE, width=1.5)

    text_box(s, Inches(0.6), Inches(6.8), Inches(12.1), Inches(0.28),
             "Authority-signed ballot quid is authorized to vote — the authority never saw which voter got which quid",
             font=B_FONT, size=10, color=SLATE, italic=True, align=PP_ALIGN.CENTER)
    notes(s, """TIMING: 2 minutes.

WHAT TO SAY:
This is the hardest part of the elections design. The voter is publicly checked in at their polling place — we know Alice showed up. Her ballot must be authenticated as eligible to vote AND unlinkable to Alice. These sound contradictory, but blind signatures solve them. Think of the blind-signature primitive as a carbon-paper envelope. Alice's device generates a fresh anonymous ballot quid — call it ballot-X-Y-Z. Alice blinds ballot-X-Y-Z's ID with a random secret factor r — the authority will see a cryptographic commitment, not the quid ID itself. The authority signs the blinded commitment. Alice's device unblinds, yielding the authority's signature on ballot-X-Y-Z's actual ID. Now ballot-X-Y-Z is cryptographically authorized to vote, but the authority never saw ballot-X-Y-Z's ID. The linkage Alice ↔ ballot-X-Y-Z only exists on Alice's device. When ballot-X-Y-Z casts a vote by issuing a trust edge to a candidate, it's provably an authorized ballot without anyone being able to trace it back to Alice. This is a cryptographically grounded secret ballot.

KEY POINTS:
• Voter generates fresh ballot quid
• Blind the ID → authority signs blinded
• Unblind → authorized ballot
• Authority never sees the linkage

TRANSITION:
Voting itself is a trust edge. And that gives us instant recount.""")
    p += 1

    # ---------- Voting = trust edges + instant recount ----------
    s = slide_content(prs, "Voting = trust edge  →  anyone can recount", kicker="ELECTIONS: THE PAYOFF", page=p, total=total)
    # Left: a vote as trust edge
    rounded(s, Inches(0.6), Inches(1.85), Inches(6), Inches(2.9), ICE, corner=0.05)
    text_box(s, Inches(0.85), Inches(2.0), Inches(5.5), Inches(0.4),
             "A VOTE IS A TRUST EDGE",
             font=B_FONT, size=11, color=SLATE, bold=True)
    # Schema
    schema = [
        ("truster:", "ballot-Xz7mN2pQ...", TEAL),
        ("trustee:", "candidate-jane-smith", AMBER),
        ("trustLevel:", "1.0", GREEN),
        ("domain:", "elections.tx-2026-nov.contests.us-senate", WHITE),
        ("nonce:", "1", WHITE),
        ("signature:", "<BQ's ECDSA sig>", WHITE),
    ]
    for i, (k, v, col) in enumerate(schema):
        y = Inches(2.55 + i * 0.35)
        text_box(s, Inches(0.85), y, Inches(1.3), Inches(0.3), k,
                 font=CODE_FONT, size=10.5, color=SLATE,
                 anchor=MSO_ANCHOR.MIDDLE)
        text_box(s, Inches(2.15), y, Inches(4.2), Inches(0.3), v,
                 font=CODE_FONT, size=10.5,
                 color=(col if col != WHITE else MIDNIGHT),
                 bold=True, anchor=MSO_ANCHOR.MIDDLE)

    # Right: tally query
    rounded(s, Inches(6.7), Inches(1.85), Inches(6), Inches(2.9), MIDNIGHT, corner=0.05)
    text_box(s, Inches(6.95), Inches(2.0), Inches(5.5), Inches(0.4),
             "ANYONE'S RECOUNT — SINGLE QUERY",
             font=B_FONT, size=11, color=TEAL, bold=True)
    text_box(s, Inches(6.95), Inches(2.45), Inches(5.5), Inches(2),
             "$ quidnug tally \\\n    --domain elections.tx-2026-nov.contests.us-senate \\\n    --method plurality\n\n  jane-smith:     47,282\n  bob-jones:      51,039\n  carol-li:       21,476\n  ──────────────────────\n  total:         119,797",
             font=CODE_FONT, size=11, color=GREEN, line_spacing=1.25)

    # Bottom: the implication
    rounded(s, Inches(0.6), Inches(4.95), Inches(12.1), Inches(2), TEAL, corner=0.05)
    text_box(s, Inches(0.85), Inches(5.1), Inches(11.6), Inches(0.45),
             "THE IMPLICATION", font=H_FONT, size=14, color=MIDNIGHT, bold=True)
    text_box(s, Inches(0.85), Inches(5.55), Inches(11.6), Inches(1.4),
             "A losing candidate runs the same query on an independent Quidnug node. They get the same numbers.\n\nNo 'wait for the Secretary of State to authorize a recount.' No 're-feed paper through the same vendor tabulator.' No trust-the-vendor.\n\nAny citizen, any journalist, any candidate, any observer — can recount in seconds.",
             font=H_FONT, size=13, color=MIDNIGHT, line_spacing=1.35)
    notes(s, """TIMING: 1.5 minutes.

WHAT TO SAY:
Once the voter has an authorized ballot quid, casting a vote is a trust edge. Trust from ballot quid to candidate quid, trust level 1.0, scoped to the contest's domain. That's it. No new primitive needed — we're using the same trust-edge transaction we've seen throughout. And because these trust edges are on a public chain, the tally is a single query. Anyone can run it. A losing candidate runs the same query on their own independent Quidnug node — they get the same numbers, byte-for-byte. No 'wait for the Secretary of State to authorize a recount.' No 're-feed paper ballots through the same vendor tabulator.' No trust-the-vendor-software. Any citizen, journalist, candidate, or observer can recount in seconds from the public chain. This is universal verifiability — a property traditional elections talk about but can't deliver cryptographically.

KEY POINTS:
• Vote = signed trust edge
• Tally = single domain query
• Universal recount = seconds
• No single-vendor trust

TRANSITION:
And paper-ballot parity — because trust but verify.""")
    p += 1

    # ---------- Paper ballot parity ----------
    s = slide_content(prs, "Paper-ballot parity — trust but verify", kicker="ELECTIONS: THE SAFETY NET", page=p, total=total)
    # Left: mocked-up paper ballot
    rounded(s, Inches(0.6), Inches(1.85), Inches(5), Inches(5), WHITE,
            line=SLATE, corner=0.02)
    text_box(s, Inches(0.8), Inches(2.0), Inches(4.6), Inches(0.45),
             "OFFICIAL PAPER BALLOT",
             font=H_FONT, size=13, color=MIDNIGHT, bold=True,
             align=PP_ALIGN.CENTER)
    text_box(s, Inches(0.8), Inches(2.45), Inches(4.6), Inches(0.35),
             "Williamson County, TX — General 2026",
             font=B_FONT, size=10, color=SLATE, italic=True,
             align=PP_ALIGN.CENTER)
    rect(s, Inches(0.8), Inches(2.9), Inches(4.6), Inches(0.03), NAVY)
    # QR-like block
    rect(s, Inches(0.95), Inches(3.1), Inches(1.2), Inches(1.2), NAVY)
    for i in range(6):
        for j in range(6):
            if (i + j) % 3 == 0:
                rect(s, Inches(1.05 + j * 0.17), Inches(3.2 + i * 0.17),
                     Inches(0.12), Inches(0.12), WHITE)
    text_box(s, Inches(2.35), Inches(3.1), Inches(3), Inches(0.35),
             "Ballot ID (QR):", font=B_FONT, size=10, color=SLATE)
    text_box(s, Inches(2.35), Inches(3.4), Inches(3), Inches(0.35),
             "ballot-Xz7m...", font=CODE_FONT, size=11, color=MIDNIGHT, bold=True)
    text_box(s, Inches(2.35), Inches(3.75), Inches(3), Inches(0.35),
             "Seq # 00142-0042", font=B_FONT, size=10, color=SLATE)
    # Votes
    rect(s, Inches(0.8), Inches(4.4), Inches(4.6), Inches(0.02), CLOUD)
    text_box(s, Inches(0.95), Inches(4.45), Inches(4.5), Inches(0.3),
             "US SENATE", font=B_FONT, size=10, color=SLATE, bold=True)
    text_box(s, Inches(0.95), Inches(4.75), Inches(4.5), Inches(0.35),
             "✓  Jane Smith", font=H_FONT, size=13, color=MIDNIGHT, bold=True)
    text_box(s, Inches(0.95), Inches(5.15), Inches(4.5), Inches(0.3),
             "PROPOSITION 5 — SALES TAX", font=B_FONT, size=10, color=SLATE, bold=True)
    text_box(s, Inches(0.95), Inches(5.45), Inches(4.5), Inches(0.35),
             "✓  YES", font=H_FONT, size=13, color=MIDNIGHT, bold=True)
    rect(s, Inches(0.8), Inches(6.0), Inches(4.6), Inches(0.02), CLOUD)
    text_box(s, Inches(0.95), Inches(6.05), Inches(4.5), Inches(0.3),
             "Authority proof: [QR]", font=CODE_FONT, size=9, color=SLATE, italic=True)

    # Right: cross-verification + benefits
    rounded(s, Inches(5.9), Inches(1.85), Inches(6.8), Inches(5), ICE, corner=0.05)
    text_box(s, Inches(6.15), Inches(2.0), Inches(6.3), Inches(0.4),
             "CROSS-VERIFICATION AT CLOSE OF POLLS",
             font=B_FONT, size=11, color=SLATE, bold=True)
    xvs = [
        "Every paper BQ-ID should match a digital trust edge",
        "Every digital BQ should have a paper counterpart",
        "Sample audit: statistically verify paper vs digital",
        "Any discrepancy → investigate, PAPER WINS",
    ]
    for i, xv in enumerate(xvs):
        y = Inches(2.5 + i * 0.4)
        oval(s, Inches(6.15), y + Inches(0.08), Inches(0.25), Inches(0.25), TEAL)
        text_box(s, Inches(6.5), y, Inches(6), Inches(0.35), xv,
                 font=B_FONT, size=12, color=MIDNIGHT)
    # Why both
    rounded(s, Inches(6.15), Inches(4.3), Inches(6.3), Inches(2.4), MIDNIGHT, corner=0.05)
    text_box(s, Inches(6.35), Inches(4.45), Inches(6), Inches(0.4),
             "WHY BOTH BEATS EITHER ALONE",
             font=B_FONT, size=11, color=TEAL, bold=True)
    text_box(s, Inches(6.35), Inches(4.9), Inches(6), Inches(1.75),
             "Paper-only: slow counts, no instant verification, recount re-uses same tabulator.\n\nDigital-only: single-vendor compromise is catastrophic; voters can't physically verify.\n\nPaper + digital: instant digital tally for speed. Paper cross-check for ground truth. If they ever disagree, paper wins — and the disagreement is specific and investigable.",
             font=B_FONT, size=11, color=CLOUD, line_spacing=1.35)
    notes(s, """TIMING: 1.5 minutes.

WHAT TO SAY:
Every digital vote has a paper ballot counterpart. The paper ballot has a QR encoding the ballot quid's ID, the votes in human-readable form, and an authority proof QR. Voter reviews, drops in a physical box. At close of polls, cross-verification: every paper ballot's BQ-ID should match a digital trust edge, and every digital trust edge should have a paper counterpart. Statistical sampling audits catch anything subtle. If they ever disagree — the paper wins. Why both? Paper-only means slow counts, no instant verification, recounts re-using the same tabulator. Digital-only means single-vendor compromise is catastrophic and voters can't physically verify. Paper plus digital gives you the best of both — instant digital tally for speed, paper cross-check for ground truth. If they ever disagree, the disagreement is specific and investigable — not 'the machine said so.'

KEY POINTS:
• Every paper → digital + every digital → paper
• Sample audit catches subtleties
• Paper wins on disagreement
• Paper + digital beats either alone

TRANSITION:
Elections done. Credit next.""")
    p += 1

    # ---------- Credit use case — bureau replacement ----------
    s = slide_content(prs, "Decentralized credit — bureau replacement", kicker="CONSUMER RIGHTS — USE CASE 11", page=p, total=total)
    # Before/after comparison
    rounded(s, Inches(0.6), Inches(1.85), Inches(6), Inches(5), ICE, corner=0.05)
    rect(s, Inches(0.6), Inches(1.85), Inches(6), Inches(0.5), RED)
    text_box(s, Inches(0.6), Inches(1.85), Inches(6), Inches(0.5),
             "TODAY — 3 bureaus decide",
             font=H_FONT, size=15, color=WHITE, bold=True,
             align=PP_ALIGN.CENTER, anchor=MSO_ANCHOR.MIDDLE)
    today_items = [
        ("Opaque", "FICO formula is proprietary"),
        ("Universal", "Same 720 for mortgage, auto, job check"),
        ("Rented data", "You don't own your history"),
        ("Slow corrections", "30–180 days per dispute"),
        ("Breach-prone", "Equifax 2017 — 147M records"),
        ("Thin-file excluded", "No history = no access"),
        ("Centralized judgment", "Three private co's rule"),
    ]
    for i, (h, b) in enumerate(today_items):
        y = Inches(2.5 + i * 0.6)
        rounded(s, Inches(0.85), y, Inches(5.5), Inches(0.55), WHITE,
                line=CLOUD, corner=0.05)
        text_box(s, Inches(1.05), y + Inches(0.08), Inches(2.1), Inches(0.4),
                 h, font=H_FONT, size=12, color=MIDNIGHT, bold=True,
                 anchor=MSO_ANCHOR.MIDDLE)
        text_box(s, Inches(3.15), y + Inches(0.08), Inches(3.1), Inches(0.4),
                 b, font=B_FONT, size=10.5, color=SLATE, italic=True,
                 anchor=MSO_ANCHOR.MIDDLE)

    # Right — with Quidnug
    rounded(s, Inches(6.7), Inches(1.85), Inches(6), Inches(5), ICE, corner=0.05)
    rect(s, Inches(6.7), Inches(1.85), Inches(6), Inches(0.5), GREEN)
    text_box(s, Inches(6.7), Inches(1.85), Inches(6), Inches(0.5),
             "WITH QUIDNUG — each lender decides",
             font=H_FONT, size=15, color=WHITE, bold=True,
             align=PP_ALIGN.CENTER, anchor=MSO_ANCHOR.MIDDLE)
    qui_items = [
        ("Transparent", "Each lender's formula is auditable"),
        ("Context-specific", "Domain-scoped trust per loan type"),
        ("Owned data", "You own your quid + access grants"),
        ("Instant corrections", "Dispute event visible in seconds"),
        ("Distributed", "No single-breach target"),
        ("Alt data welcome", "Utilities/rent/employer as first-class"),
        ("Per-lender judgment", "Market competition, not monopoly"),
    ]
    for i, (h, b) in enumerate(qui_items):
        y = Inches(2.5 + i * 0.6)
        rounded(s, Inches(6.95), y, Inches(5.5), Inches(0.55), WHITE,
                line=CLOUD, corner=0.05)
        text_box(s, Inches(7.15), y + Inches(0.08), Inches(2.1), Inches(0.4),
                 h, font=H_FONT, size=12, color=TEAL, bold=True,
                 anchor=MSO_ANCHOR.MIDDLE)
        text_box(s, Inches(9.25), y + Inches(0.08), Inches(3.1), Inches(0.4),
                 b, font=B_FONT, size=10.5, color=SLATE, italic=True,
                 anchor=MSO_ANCHOR.MIDDLE)
    notes(s, """TIMING: 2 minutes.

WHAT TO SAY:
The most consequential use case after elections. Today's credit reporting — Equifax, Experian, TransUnion plus FICO — has seven structural problems listed on the left. Opaque scoring. Universal number used across completely different contexts. Rented data. Painful corrections. Catastrophic breaches. Thin-file exclusion. Centralized judgment. Quidnug inverts each one. Transparent — each lender's algorithm is their own, auditable by the consumer. Context-specific — credit.mortgage.us is separate from credit.auto-loan.us. Consumer owns their quid and controls who sees their history via encrypted access grants. Disputes are signed events visible in seconds. Distributed — no single database to breach. Alternative data sources — utility, rent, employer — are first-class signers. Per-lender judgment — market competition instead of the three-bureau oligopoly. And critically: this ALSO blocks social-credit concentration, because there's no protocol-level 'universal citizen score' for a state to mandate.

KEY POINTS:
• 7 structural problems — 7 protocol-level fixes
• Subject owns quid + data access
• Per-lender, per-domain trust
• Bonus: blocks social-credit concentration

TRANSITION:
Quick on the three cross-industry use cases — healthcare, credentials, artifact signing.""")
    p += 1

    # ---------- Cross-industry: healthcare ----------
    s = slide_content(prs, "Healthcare consent management", kicker="CROSS-INDUSTRY — USE CASE 12", page=p, total=total)
    # Left: current mess
    rounded(s, Inches(0.6), Inches(1.85), Inches(5.5), Inches(5), ICE, corner=0.05)
    text_box(s, Inches(0.85), Inches(2.0), Inches(5), Inches(0.4),
             "PATIENT MEDICAL RECORDS TODAY",
             font=B_FONT, size=11, color=SLATE, bold=True)
    realities = [
        "ER, PCP, specialists, labs, insurers — 10+ systems",
        "Consent faxed, platform-locked portals",
        "HIPAA punishes after, doesn't prevent at time-of-access",
        "Emergency access is ad-hoc 'break glass'",
        "Revocation rarely propagates anywhere",
        "Referral access is implicit + undocumented",
    ]
    for i, t in enumerate(realities):
        y = Inches(2.5 + i * 0.65)
        rounded(s, Inches(0.85), y, Inches(5), Inches(0.55), WHITE,
                line=CLOUD, corner=0.05)
        oval(s, Inches(1.0), y + Inches(0.18), Inches(0.2), Inches(0.2), RED)
        text_box(s, Inches(1.3), y + Inches(0.05), Inches(4.5), Inches(0.5),
                 t, font=B_FONT, size=11, color=MIDNIGHT,
                 anchor=MSO_ANCHOR.MIDDLE)

    # Right: Quidnug model
    rounded(s, Inches(6.2), Inches(1.85), Inches(6.5), Inches(5), MIDNIGHT, corner=0.05)
    text_box(s, Inches(6.45), Inches(2.0), Inches(6), Inches(0.4),
             "WITH QUIDNUG", font=B_FONT, size=11, color=TEAL, bold=True)
    text_box(s, Inches(6.45), Inches(2.4), Inches(6), Inches(0.5),
             "Patient = quid. Consent = trust edge.",
             font=H_FONT, size=16, color=WHITE, bold=True)
    text_box(s, Inches(6.45), Inches(3.05), Inches(6), Inches(2),
             "Every access = signed event on patient's stream. Sub-domain granular: allow Rx access, block mental-health access.\n\nRevocation propagates via push gossip — seconds.\n\nEmergency: guardian quorum (spouse + proxy + doctor) with 15-min time-lock and audit trail.",
             font=B_FONT, size=12, color=CLOUD, line_spacing=1.35)

    # Sub-domain examples
    rounded(s, Inches(6.45), Inches(5.3), Inches(6), Inches(1.5), ICE, corner=0.04)
    text_box(s, Inches(6.65), Inches(5.45), Inches(5.6), Inches(0.35),
             "SUB-DOMAIN CONSENT GRANULARITY",
             font=B_FONT, size=10, color=SLATE, bold=True)
    subs = [
        ("healthcare.records.access.prescriptions", TEAL),
        ("healthcare.records.access.imaging", TEAL),
        ("healthcare.records.access.mental-health", RED),
        ("healthcare.records.access.genetic", AMBER),
    ]
    for i, (d, c) in enumerate(subs):
        y = Inches(5.8 + i * 0.22)
        text_box(s, Inches(6.65), y, Inches(5.6), Inches(0.22), d,
                 font=CODE_FONT, size=9.5, color=c, bold=True)
    notes(s, """TIMING: 1.5 minutes.

WHAT TO SAY:
Healthcare consent. Today's reality: ER, primary care, specialists, labs, insurers — a patient's records span ten-plus systems. Consent is faxed signatures or platform-locked portals. HIPAA punishes unauthorized access AFTER the fact — it doesn't prevent it at time of access. Emergency override is ad-hoc 'break glass.' Revocation barely propagates. With Quidnug: patient is a quid. Consent to a provider is a trust edge in a healthcare.records.access domain. Every access by a provider emits a signed event on the patient's stream. Sub-domain granularity means patient can allow prescription access to their pharmacy while blocking mental-health access — the pharmacy gets one domain's trust, not all of it. Revocation propagates in seconds via push gossip. Emergency override uses the patient's guardian quorum — typically spouse plus healthcare proxy plus primary-care doctor — with a 15-minute time-lock delay and a full audit trail.

KEY POINTS:
• Consent = trust edge
• Sub-domain granular consent
• Instant revocation propagation
• Emergency = guardian quorum with time-lock

TRANSITION:
Credential verification.""")
    p += 1

    # ---------- Cross-industry: credentials ----------
    s = slide_content(prs, "Credential verification network", kicker="CROSS-INDUSTRY — USE CASE 13", page=p, total=total)
    # Diagram: issuer chain
    rounded(s, Inches(0.6), Inches(1.85), Inches(12.1), Inches(3.5), ICE, corner=0.05)
    text_box(s, Inches(0.85), Inches(2.0), Inches(11.6), Inches(0.4),
             "ACCREDITOR → ISSUER → CREDENTIAL — TRUST HIERARCHY",
             font=B_FONT, size=11, color=SLATE, bold=True)
    # Three-tier diagram
    tier_y1 = Inches(2.6)
    oval(s, Inches(5.5), tier_y1, Inches(2.3), Inches(0.9), NAVY)
    text_box(s, Inches(5.5), tier_y1, Inches(2.3), Inches(0.9),
             "SACSCOC\n(accreditor)",
             font=H_FONT, size=13, color=WHITE, bold=True,
             align=PP_ALIGN.CENTER, anchor=MSO_ANCHOR.MIDDLE, line_spacing=1.2)
    # Issuers
    tier_y2 = Inches(3.85)
    issuers = [("Univ. of Texas", Inches(2.5), TEAL),
               ("Texas Medical Board", Inches(6.5), TEAL),
               ("ABET (engr)", Inches(10.0), TEAL)]
    for lab, x, col in issuers:
        oval(s, x, tier_y2, Inches(2.3), Inches(0.9), col)
        text_box(s, x, tier_y2, Inches(2.3), Inches(0.9), lab,
                 font=H_FONT, size=12, color=WHITE, bold=True,
                 align=PP_ALIGN.CENTER, anchor=MSO_ANCHOR.MIDDLE)
        arrow(s, Inches(6.65), tier_y1 + Inches(0.9), x + Inches(1.15), tier_y2,
              color=SLATE, width=1.5)
    # Credentials
    tier_y3 = Inches(5.05)
    creds = [("BS CS Alice", Inches(1.8), AMBER),
             ("MD License\nDr. Jones", Inches(5.8), AMBER),
             ("PE License\nBob Chen", Inches(10.3), AMBER)]
    for lab, x, col in creds:
        rounded(s, x, tier_y3, Inches(1.8), Inches(0.65), col, corner=0.1)
        text_box(s, x, tier_y3, Inches(1.8), Inches(0.65), lab,
                 font=B_FONT, size=10, color=MIDNIGHT, bold=True,
                 align=PP_ALIGN.CENTER, anchor=MSO_ANCHOR.MIDDLE, line_spacing=1.2)

    # Bottom benefits
    rounded(s, Inches(0.6), Inches(5.45), Inches(12.1), Inches(1.5), MIDNIGHT, corner=0.05)
    text_box(s, Inches(0.85), Inches(5.6), Inches(11.6), Inches(0.45),
             "WHAT EMPLOYERS / HOSPITALS / LICENSERS GAIN",
             font=B_FONT, size=11, color=TEAL, bold=True)
    gains = [
        "Seconds to verify — no phone call to registrar",
        "Revocation propagates in minutes — not months",
        "Forgery requires compromising issuer's HSM",
        "Cross-jurisdiction via reciprocity trust edges",
    ]
    for i, g in enumerate(gains):
        x = Inches(0.85 + (i % 2) * 6)
        y = Inches(6.1 + (i // 2) * 0.35)
        text_box(s, x, y, Inches(6), Inches(0.3), "✓  " + g,
                 font=B_FONT, size=11, color=CLOUD)
    notes(s, """TIMING: 1.5 minutes.

WHAT TO SAY:
Credentials — diplomas, professional licenses, industry certifications. Today's verification flow is embarrassing — an employer phones the registrar, waits five business days, maybe hears back. Revocations rarely propagate. Paper certificates are trivially forged. Quidnug models the trust hierarchy naturally. Accreditors at the top — like SACSCOC for southern universities. Issuers in the middle — universities, medical boards, professional cert orgs. Credentials at the bottom — individual degrees, licenses, certifications. Accreditors issue trust edges to issuers in a domain like credentials.education. Issuers issue signed credential titles to individuals. An employer evaluating a candidate queries: is this degree signed by an issuer in a domain my accreditor trusts? Seconds to verify. Revocations flow via push gossip in minutes. Forgery requires compromising an issuer's HSM — same bar as any signed-attestation system. Cross-jurisdiction reciprocity — Texas medical board recognizing California medical board — is just a trust edge between board quids.

KEY POINTS:
• Accreditor → issuer → credential hierarchy
• Seconds to verify
• Minutes to revoke
• Cross-jurisdiction via reciprocity

TRANSITION:
Last use case — developer artifact signing.""")
    p += 1

    # ---------- Developer artifact signing ----------
    s = slide_content(prs, "Developer artifact signing", kicker="CROSS-INDUSTRY — USE CASE 14", page=p, total=total)
    # Left: GPG problem
    rounded(s, Inches(0.6), Inches(1.85), Inches(6), Inches(5), ICE, corner=0.05)
    rect(s, Inches(0.6), Inches(1.85), Inches(6), Inches(0.5), RED)
    text_box(s, Inches(0.6), Inches(1.85), Inches(6), Inches(0.5),
             "GPG + ADMIN-SIGNED RELEASES",
             font=H_FONT, size=15, color=WHITE, bold=True,
             align=PP_ALIGN.CENTER, anchor=MSO_ANCHOR.MIDDLE)
    problems = [
        ("Single maintainer key", "Losing it breaks downstream trust"),
        ("No guardian recovery", "Wait for ecosystem to rotate"),
        ("No multi-maintainer native", "Ad-hoc workflow per project"),
        ("Revocation is a Twitter post", "Not cryptographic propagation"),
        ("Cross-registry = separate keys", "npm + PyPI + Maven = 3 keys"),
    ]
    for i, (h, b) in enumerate(problems):
        y = Inches(2.55 + i * 0.82)
        rounded(s, Inches(0.85), y, Inches(5.5), Inches(0.72), WHITE,
                line=CLOUD, corner=0.05)
        oval(s, Inches(1.0), y + Inches(0.22), Inches(0.28), Inches(0.28), RED)
        text_box(s, Inches(1.0), y + Inches(0.22), Inches(0.28), Inches(0.28),
                 "✗", font=H_FONT, size=14, color=WHITE, bold=True,
                 align=PP_ALIGN.CENTER, anchor=MSO_ANCHOR.MIDDLE)
        text_box(s, Inches(1.4), y + Inches(0.1), Inches(4.8), Inches(0.3),
                 h, font=B_FONT, size=11.5, color=MIDNIGHT, bold=True)
        text_box(s, Inches(1.4), y + Inches(0.38), Inches(4.8), Inches(0.3),
                 b, font=B_FONT, size=10, color=SLATE, italic=True)

    # Right: Quidnug model
    rounded(s, Inches(6.7), Inches(1.85), Inches(6), Inches(5), ICE, corner=0.05)
    rect(s, Inches(6.7), Inches(1.85), Inches(6), Inches(0.5), GREEN)
    text_box(s, Inches(6.7), Inches(1.85), Inches(6), Inches(0.5),
             "QUIDNUG ARTIFACT SIGNING",
             font=H_FONT, size=15, color=WHITE, bold=True,
             align=PP_ALIGN.CENTER, anchor=MSO_ANCHOR.MIDDLE)
    solutions = [
        ("Maintainer = quid", "Single identity across npm/PyPI/Maven"),
        ("Co-maintainers = guardians", "Multi-sign for critical releases"),
        ("Guardian-recoverable", "Lost key? Co-maintainers rotate"),
        ("Revocation = push gossip", "Minutes to reach every consumer"),
        ("Rotation chain", "Downstream auto-tracks maintainer quid, not specific keys"),
    ]
    for i, (h, b) in enumerate(solutions):
        y = Inches(2.55 + i * 0.82)
        rounded(s, Inches(6.95), y, Inches(5.5), Inches(0.72), WHITE,
                line=CLOUD, corner=0.05)
        oval(s, Inches(7.1), y + Inches(0.22), Inches(0.28), Inches(0.28), GREEN)
        text_box(s, Inches(7.1), y + Inches(0.22), Inches(0.28), Inches(0.28),
                 "✓", font=H_FONT, size=14, color=WHITE, bold=True,
                 align=PP_ALIGN.CENTER, anchor=MSO_ANCHOR.MIDDLE)
        text_box(s, Inches(7.5), y + Inches(0.1), Inches(4.8), Inches(0.3),
                 h, font=B_FONT, size=11.5, color=MIDNIGHT, bold=True)
        text_box(s, Inches(7.5), y + Inches(0.38), Inches(4.8), Inches(0.3),
                 b, font=B_FONT, size=10, color=SLATE, italic=True)
    notes(s, """TIMING: 1.5 minutes.

WHAT TO SAY:
Open source artifact signing is still largely GPG. Every week, some popular package maintainer loses their key and we get a scramble. Problems: single-maintainer-key, no recovery, no multi-maintainer native support, revocation propagation via Twitter, separate keys per package registry. Quidnug model: maintainer is a quid. Their co-maintainers are the guardians. Multi-sign on critical releases via M-of-N. Lost key? Co-maintainers rotate via guardian recovery — downstream consumers auto-track the maintainer's quid, not specific key fingerprints, so the rotation is transparent. Revocation propagates via push gossip — minutes to reach every consumer. And a single quid identity spans npm, PyPI, Maven, Cargo — just attested per-registry via events. Sigstore plus Quidnug plus reproducible builds gives a comprehensive solution to the package supply-chain problem.

KEY POINTS:
• Maintainer = quid, co-maintainers = guardians
• Guardian recovery for lost keys
• One identity, N package registries
• Sigstore-compatible, plus stronger recovery story

TRANSITION:
That's all fourteen. Let's summarize.""")
    p += 1

    # ---------- 14 use cases summary table ----------
    s = slide_content(prs, "14 use cases at a glance", kicker="RECAP", page=p, total=total)
    rows = [
        ["#", "Use case", "Category", "Primary Quidnug features"],
        ["1", "Interbank wire authorization", "FinTech", "Guardian M-of-N, replay protection"],
        ["2", "Merchant fraud consortium", "FinTech", "Relational trust, push gossip"],
        ["3", "DeFi oracle network", "FinTech", "Signed feeds, per-consumer weighting"],
        ["4", "Institutional custody", "FinTech", "Full anchor lifecycle, lazy probe"],
        ["5", "B2B invoice financing", "FinTech", "Titles, event streams, domains"],
        ["6", "AI model provenance", "AI", "Title chain, attester trust"],
        ["7", "AI agent authorization", "AI", "Guardian-as-capability, time-lock"],
        ["8", "Federated learning attestation", "AI", "Event streams, coordinator accountability"],
        ["9", "AI content authenticity", "AI", "Per-asset event chain, transitive trust"],
        ["10", "Elections", "Government", "BYO-quid, blind-sig, universal recount"],
        ["11", "Decentralized credit", "Consumer rights", "Per-lender relational trust, alt-data"],
        ["12", "Healthcare consent management", "Cross-industry", "Sub-domain consent, guardian override"],
        ["13", "Credential verification", "Cross-industry", "Accreditor hierarchy, revocation gossip"],
        ["14", "Developer artifact signing", "Cross-industry", "Guardian recovery, one-identity-many-registries"],
    ]
    col_widths = [Inches(0.55), Inches(3.8), Inches(2.1), Inches(5.65)]
    table_rows(s, Inches(0.6), Inches(1.85), col_widths, rows,
               header_fill=NAVY, header_size=12, body_size=10.5,
               row_h=Inches(0.33), header_h=Inches(0.4), bold_first_col=False)
    notes(s, """TIMING: 1 minute.

WHAT TO SAY:
Fourteen use cases, the categories, and the primary Quidnug features each exploits. The patterns that keep coming up: guardian M-of-N for multi-party approval, trust edges for relational authorization, event streams for audit trails, push gossip for real-time propagation, and domain scoping for context separation. These aren't fourteen different Quidnug installations — they're all the same protocol with different domain hierarchies and different off-chain integrations. A consortium deployment can run a dozen use cases simultaneously without interference, because domains namespace them completely.

TRANSITION:
Let's compare Quidnug to the alternatives.""")
    p += 1

    # ---------- Comparison with alternatives ----------
    s = slide_content(prs, "How Quidnug compares", kicker="ALTERNATIVES", page=p, total=total)
    rows = [
        ["", "Quidnug", "Bitcoin/Ethereum", "Traditional DB", "OAuth/OIDC"],
        ["Trust model", "Relational, per-observer", "Universal consensus", "Centralized", "Federated central"],
        ["Identity", "Self-sovereign (quids)", "Self-sovereign (addrs)", "Platform-owned", "Provider-owned"],
        ["Key recovery", "Guardian M-of-N + time-lock", "Seed phrase", "Email reset", "Provider reset"],
        ["Throughput", "Moderate (consortium)", "Low (mainnet)", "Very high", "Very high"],
        ["Replay protection", "Per-signer nonce", "TX-hash uniqueness", "App-level", "JWT jti"],
        ["Multi-party approval", "First-class (guardians)", "Smart contract", "App-level", "Not native"],
        ["Auditability", "Full chain replay", "Full chain replay", "DB logs (trust them)", "Provider logs"],
        ["Best for", "Cross-org trust, high-value", "Global money / dApps", "Single-org data", "Login federation"],
    ]
    col_widths = [Inches(2.2), Inches(2.9), Inches(2.5), Inches(2.35), Inches(2.15)]
    table_rows(s, Inches(0.6), Inches(1.85), col_widths, rows,
               header_fill=NAVY, header_size=11, body_size=10,
               row_h=Inches(0.44), header_h=Inches(0.45), bold_first_col=True)
    notes(s, """TIMING: 1.5 minutes.

WHAT TO SAY:
Here's the comparison. Bitcoin / Ethereum produce universal consensus — every node agrees on the same chain. Fine for global money, bad for consortium trust. Traditional databases are centralized — fine for single-org data, wrong for cross-org. OAuth / OIDC is federated with a central identity provider — great for login, not for the multi-party-approval problem. Quidnug fits in a different quadrant: relational per-observer trust, self-sovereign identity, guardian-based recovery, moderate throughput tuned for consortium scale, per-signer replay protection, first-class multi-party approval. Pick the right tool for your problem. Quidnug is best when you need cross-organization trust at high-value scale — the FinTech, AI, and civic-tech use cases we've seen. Bitcoin is best when you need money that works without any trust. Traditional DB is best when you have one org and one use case. OAuth is best for login. Don't force any tool where it doesn't fit.

KEY POINTS:
• Different quadrant from other options
• Consortium / federated — Quidnug's sweet spot
• Pick right tool for problem
• Don't force fit

TRANSITION:
When should you actually pick Quidnug — and when should you not?""")
    p += 1

    # ---------- When to use ----------
    s = slide_content(prs, "When to use Quidnug", kicker="STRONG FIT", page=p, total=total)
    reasons = [
        ("Your data model has 'who trusts whom' as first-class",
         "Reputation, credentials, consortium fraud detection, cross-org approvals."),
        ("You need recoverable keys without central escrow",
         "Custody, high-value signing, long-lived credentials."),
        ("Replay-safe auditable state transitions — without global consensus",
         "Most internal and consortium systems — don't need 'one chain for everyone.'"),
        ("Coordinated federated protocol upgrades",
         "Fork-block gives you on-chain, operator-queryable activation boundaries."),
        ("Per-observer scoring / evaluation is natural",
         "Credit, oracle networks, fraud consortiums — different evaluators, same data."),
        ("Multi-party approval is load-bearing",
         "Wires, governance, sensitive ops, agent authorization."),
    ]
    for i, (h, b) in enumerate(reasons):
        y = Inches(1.85 + i * 0.8)
        rounded(s, Inches(0.6), y, Inches(12.1), Inches(0.7), ICE, corner=0.05)
        rect(s, Inches(0.6), y, Inches(0.09), Inches(0.7), GREEN)
        oval(s, Inches(0.85), y + Inches(0.18), Inches(0.35), Inches(0.35), GREEN)
        text_box(s, Inches(0.85), y + Inches(0.18), Inches(0.35), Inches(0.35),
                 "✓", font=H_FONT, size=16, color=WHITE, bold=True,
                 align=PP_ALIGN.CENTER, anchor=MSO_ANCHOR.MIDDLE)
        text_box(s, Inches(1.35), y + Inches(0.06), Inches(11), Inches(0.35),
                 h, font=H_FONT, size=13, color=MIDNIGHT, bold=True)
        text_box(s, Inches(1.35), y + Inches(0.4), Inches(11), Inches(0.3),
                 b, font=B_FONT, size=11, color=SLATE, italic=True)
    notes(s, """TIMING: 1 minute.

WHAT TO SAY:
Six strong-fit signals. If your data model has who-trusts-whom as a first-class question — reputation systems, credentials, consortium fraud, cross-org approvals — Quidnug is literally designed for it. If you need recoverable keys without central escrow — custody, high-value signing, long-lived credentials — guardian recovery is the best game in town. If you need replay-safe auditable state without requiring one chain for everyone — most consortium systems, Quidnug's a great fit. If you need coordinated federated upgrades, fork-block is unique. Per-observer scoring — credit, oracles, fraud — the relational trust primitive is the whole point. Multi-party approval as load-bearing — guardians are a first-class primitive for you.

TRANSITION:
And when NOT to use it.""")
    p += 1

    # ---------- When NOT to use ----------
    s = slide_content(prs, "When NOT to use Quidnug", kicker="WEAK FIT", page=p, total=total)
    reasons = [
        ("Millions of TPS payments",
         "Quidnug prioritizes auditability over raw throughput. Target: thousands of TPS per node."),
        ("Fully public permissionless chains",
         "Proof-of-Trust needs initial trust seeding. A zero-trust-input network degrades to an untrusted gossip graph."),
        ("You need a single universal score",
         "Quidnug deliberately refuses to produce one. You can build an aggregator on top, but not in the protocol."),
        ("Single-org, single-database app",
         "If there's no cross-party trust question, a boring database is simpler and faster."),
        ("Anonymous login / SSO replacement",
         "OAuth does this well. Quidnug isn't an identity provider for web apps."),
        ("Real-time streaming at 100k events/sec",
         "Protocol is designed for moderate-volume, high-value signed events, not firehose telemetry."),
    ]
    for i, (h, b) in enumerate(reasons):
        y = Inches(1.85 + i * 0.8)
        rounded(s, Inches(0.6), y, Inches(12.1), Inches(0.7), ICE, corner=0.05)
        rect(s, Inches(0.6), y, Inches(0.09), Inches(0.7), RED)
        oval(s, Inches(0.85), y + Inches(0.18), Inches(0.35), Inches(0.35), RED)
        text_box(s, Inches(0.85), y + Inches(0.18), Inches(0.35), Inches(0.35),
                 "✗", font=H_FONT, size=16, color=WHITE, bold=True,
                 align=PP_ALIGN.CENTER, anchor=MSO_ANCHOR.MIDDLE)
        text_box(s, Inches(1.35), y + Inches(0.06), Inches(11), Inches(0.35),
                 h, font=H_FONT, size=13, color=MIDNIGHT, bold=True)
        text_box(s, Inches(1.35), y + Inches(0.4), Inches(11), Inches(0.3),
                 b, font=B_FONT, size=11, color=SLATE, italic=True)
    notes(s, """TIMING: 1 minute.

WHAT TO SAY:
Six anti-patterns. Don't use Quidnug for millions-of-TPS payments — we prioritize auditability over raw throughput. Target is thousands of TPS per node with aggressive tuning, not millions. Don't use for fully-public permissionless chains — Proof-of-Trust needs initial trust seeding. If you truly have no prior trust relationships between nodes, you get an untrusted-gossip network, which is correct but not useful. Don't use if you need a single universal score — we deliberately refuse to produce one. Don't use for a boring single-org single-database app — if there's no cross-party trust question, a database is simpler and faster. Don't use as SSO — OAuth does login better. Don't use for firehose telemetry — protocol is designed for moderate-volume high-value events, not 100k events per second.

TRANSITION:
Alright — if you're sold, here's how you start.""")
    p += 1

    # ---------- SECTION 7 divider ----------
    slide_section(prs, 7, "PART SEVEN", "Getting started")
    p += 1

    # ---------- Quick start ----------
    s = slide_content(prs, "Quick start — 3 minutes", kicker="HANDS-ON", page=p, total=total)
    # Steps with code
    rounded(s, Inches(0.6), Inches(1.85), Inches(12.1), Inches(5.1), MIDNIGHT, corner=0.05)
    text_box(s, Inches(0.85), Inches(2.0), Inches(11.6), Inches(0.4),
             "THREE TERMINAL COMMANDS, TWO CURL REQUESTS",
             font=B_FONT, size=11, color=TEAL, bold=True)
    # Step 1
    text_box(s, Inches(0.85), Inches(2.5), Inches(11.6), Inches(0.35),
             "1.  Clone + build + run",
             font=H_FONT, size=14, color=WHITE, bold=True)
    rect(s, Inches(0.85), Inches(2.9), Inches(11.6), Inches(0.6), NAVY)
    text_box(s, Inches(1.05), Inches(2.9), Inches(11.3), Inches(0.6),
             "$ git clone github.com/bhmortim/quidnug && cd quidnug && make build && ./bin/quidnug",
             font=CODE_FONT, size=12, color=TEAL, bold=True,
             anchor=MSO_ANCHOR.MIDDLE)
    # Step 2
    text_box(s, Inches(0.85), Inches(3.65), Inches(11.6), Inches(0.35),
             "2.  Create an identity",
             font=H_FONT, size=14, color=WHITE, bold=True)
    rect(s, Inches(0.85), Inches(4.05), Inches(11.6), Inches(0.6), NAVY)
    text_box(s, Inches(1.05), Inches(4.05), Inches(11.3), Inches(0.6),
             '$ curl -X POST localhost:8080/api/identities -d \'{"quidId":"alice","name":"Alice","creator":"alice","updateNonce":1}\'',
             font=CODE_FONT, size=10.5, color=TEAL, bold=True,
             anchor=MSO_ANCHOR.MIDDLE)
    # Step 3
    text_box(s, Inches(0.85), Inches(4.8), Inches(11.6), Inches(0.35),
             "3.  Declare trust from Alice to Bob",
             font=H_FONT, size=14, color=WHITE, bold=True)
    rect(s, Inches(0.85), Inches(5.2), Inches(11.6), Inches(0.6), NAVY)
    text_box(s, Inches(1.05), Inches(5.2), Inches(11.3), Inches(0.6),
             '$ curl -X POST localhost:8080/api/trust -d \'{"truster":"alice","trustee":"bob","trustLevel":0.9,"domain":"contractors.home","nonce":1}\'',
             font=CODE_FONT, size=10.5, color=TEAL, bold=True,
             anchor=MSO_ANCHOR.MIDDLE)
    # Step 4
    text_box(s, Inches(0.85), Inches(5.95), Inches(11.6), Inches(0.35),
             "4.  Query trust — the payoff",
             font=H_FONT, size=14, color=WHITE, bold=True)
    rect(s, Inches(0.85), Inches(6.35), Inches(11.6), Inches(0.55), GREEN)
    text_box(s, Inches(1.05), Inches(6.35), Inches(11.3), Inches(0.55),
             '$ curl "localhost:8080/api/trust/alice/bob?domain=contractors.home"  →  trustLevel: 0.9',
             font=CODE_FONT, size=11, color=WHITE, bold=True,
             anchor=MSO_ANCHOR.MIDDLE)
    notes(s, """TIMING: 1 minute.

WHAT TO SAY:
Four commands. Clone the repo, build, run — one terminal. Two curl commands to create identities and declare trust — second terminal. One curl to query it back — get the trust level. That's the whole 'hello world.' From here you add more identities, more trust edges, move to consortium deployment, start emitting events, and so on. The JavaScript client handles the boilerplate; there's a Go package for embedding directly. Full API documentation is at docs/openapi.yaml — OpenAPI 3.0 spec, auto-renders in Swagger or any OpenAPI tool.

KEY POINTS:
• 4 commands, 3 minutes
• Full OpenAPI spec in the repo
• JS + Go clients available
• Go embed option too

TRANSITION:
Here's the resources map.""")
    p += 1

    # ---------- Resources ----------
    s = slide_content(prs, "Resources", kicker="WHERE TO GO NEXT", page=p, total=total)
    resources = [
        ("README.md",
         "Top-level overview, capability table, TL;DR",
         "Start here. 15-minute read.", TEAL),
        ("docs/design/0001—0010",
         "Numbered Quidnug Design Proposals",
         "Full technical details for every landed feature.", NAVY),
        ("UseCases/ (14 folders)",
         "README + architecture + implementation + threat-model",
         "Production-grade design for each use case.", AMBER),
        ("docs/architecture.md",
         "System design and internals",
         "For contributors and architects.", TEAL),
        ("docs/openapi.yaml",
         "OpenAPI 3.0 spec for the REST API",
         "Auto-renders in Swagger UI.", NAVY),
        ("clients/js/",
         "JavaScript/TypeScript client library",
         "npm install quidnug-client", AMBER),
        ("CHANGELOG.md",
         "Every QDP + what landed when",
         "Keep-a-Changelog format, versioned.", TEAL),
        ("SECURITY.md",
         "How to report vulnerabilities",
         "Don't open public issues for security.", RED),
    ]
    for i, (name, desc, action, col) in enumerate(resources):
        x = Inches(0.6 + (i % 2) * 6.1)
        y = Inches(1.85 + (i // 2) * 1.3)
        rounded(s, x, y, Inches(5.95), Inches(1.15), ICE, corner=0.05)
        rect(s, x, y, Inches(0.08), Inches(1.15), col)
        text_box(s, x + Inches(0.22), y + Inches(0.1), Inches(5.6), Inches(0.38),
                 name, font=CODE_FONT, size=13, color=MIDNIGHT, bold=True)
        text_box(s, x + Inches(0.22), y + Inches(0.46), Inches(5.6), Inches(0.3),
                 desc, font=B_FONT, size=11, color=SLATE, italic=True)
        text_box(s, x + Inches(0.22), y + Inches(0.78), Inches(5.6), Inches(0.3),
                 action, font=B_FONT, size=11, color=col, bold=True)
    notes(s, """TIMING: 1 minute.

WHAT TO SAY:
Eight places to go. README is the 15-minute overview — start there if you haven't already. The ten QDPs under docs/design have full technical detail for every landed protocol feature. The fourteen UseCase folders have production-grade designs — README, architecture, implementation with concrete Quidnug API calls, threat model. If you're contributing or architecting, docs/architecture.md has the system internals. OpenAPI spec for the REST API renders in any OpenAPI tool. JavaScript client is on npm. CHANGELOG has the full history. And SECURITY.md is the responsible-disclosure process.

TRANSITION:
Let me close with the three things to remember.""")
    p += 1

    # ---------- Three takeaways ----------
    s = slide_dark(prs, "Three takeaways", kicker="CLOSING", page=p, total=total)
    # Three big horizontal cards
    items = [
        ("Trust is relational.",
         "Different observers answer 'how much do I trust Bob?' differently. The protocol refuses to produce a universal score. This is the fundamental shift.",
         "T", TEAL),
        ("Identity is owned, keys are recoverable.",
         "Quids are generated by users. Lost keys recover via M-of-N guardian quorum with time-locked veto. No central escrow.",
         "I", AMBER),
        ("State is cryptographically auditable — by anyone.",
         "Every transaction signed. Every event stream append-only. Anyone runs the tally query. No 'trust our vendor.'",
         "A", GREEN),
    ]
    for i, (head, sub, icon, color) in enumerate(items):
        y = Inches(1.85 + i * 1.5)
        rounded(s, Inches(0.6), y, Inches(12.1), Inches(1.3), NAVY, corner=0.06)
        # Icon
        oval(s, Inches(0.85), y + Inches(0.3), Inches(0.75), Inches(0.75), color)
        text_box(s, Inches(0.85), y + Inches(0.3), Inches(0.75), Inches(0.75),
                 icon, font=H_FONT, size=32, color=MIDNIGHT, bold=True,
                 align=PP_ALIGN.CENTER, anchor=MSO_ANCHOR.MIDDLE)
        text_box(s, Inches(1.9), y + Inches(0.15), Inches(10.5), Inches(0.5),
                 head, font=H_FONT, size=20, color=WHITE, bold=True)
        text_box(s, Inches(1.9), y + Inches(0.65), Inches(10.5), Inches(0.65),
                 sub, font=B_FONT, size=13, color=CLOUD, italic=True,
                 line_spacing=1.3)
    # Footer line
    text_box(s, Inches(0.6), Inches(6.7), Inches(12.1), Inches(0.35),
             "If you take nothing else away, take these three.",
             font=H_FONT, size=14, color=TEAL, italic=True,
             align=PP_ALIGN.CENTER)
    notes(s, """TIMING: 1.5 minutes.

WHAT TO SAY:
Three takeaways. First: trust is relational. Every architectural decision in the protocol comes back to this. If you're tempted to build a system that produces a universal score, you're fighting the protocol's grain — step back and ask if your problem really needs that, or if per-observer evaluation would fit. Second: identity is owned, keys are recoverable. Users generate their own quids. Keys are recoverable via M-of-N guardian quorum — the best recovery primitive we have for asymmetric crypto. No central escrow, no forgot-password, no 'trust the vendor.' Third: state is cryptographically auditable by anyone. Every transaction is signed. Every event stream is append-only. Anyone can run the tally query, verify the provenance, check the audit trail. This removes whole categories of vendor-trust risk. These three principles drive every use case we've looked at.

TRANSITION:
Thank you. Questions?""")
    p += 1

    # ---------- Final thought — quote-style slide ----------
    s = slide_quote(prs,
        "A protocol for systems where the question 'who signed this?' matters more than 'does this database row exist?'",
        attribution="The Quidnug premise, in one sentence",
        page=p, total=total)
    notes(s, """TIMING: 30 seconds.

WHAT TO SAY:
If I had one sentence to describe Quidnug to an engineer, it would be this. A protocol for systems where 'who signed this?' matters more than 'does this database row exist?' Banking. Elections. AI provenance. Credentials. Consent management. Package signing. All of them share that shape. If your problem shares that shape, Quidnug is probably the right protocol. If it doesn't, use something simpler. Thanks.

TRANSITION:
Thank-you slide next.""")
    p += 1

    # ---------- Thank you ----------
    s = blank(prs)
    set_bg(s, MIDNIGHT)
    # Big hex cluster
    hexagon(s, Inches(5.2), Inches(1.8), Inches(0.8), Inches(0.7), TEAL)
    hexagon(s, Inches(5.9), Inches(2.4), Inches(0.7), Inches(0.6), AMBER)
    hexagon(s, Inches(6.7), Inches(1.8), Inches(0.8), Inches(0.7), NAVY, line=TEAL, line_w=2)
    hexagon(s, Inches(7.5), Inches(2.4), Inches(0.6), Inches(0.55), TEAL)

    text_box(s, Inches(0), Inches(3.3), Inches(13.333), Inches(1.35),
             "Thank you",
             font=H_FONT, size=96, color=WHITE, bold=True,
             align=PP_ALIGN.CENTER)
    # NOTE: no accent line under the title — whitespace instead (AI-tell avoided)
    text_box(s, Inches(0), Inches(5.05), Inches(13.333), Inches(0.55),
             "Questions & Discussion",
             font=H_FONT, size=26, color=TEAL, italic=True, align=PP_ALIGN.CENTER)
    text_box(s, Inches(0), Inches(5.95), Inches(13.333), Inches(0.4),
             "github.com/bhmortim/quidnug",
             font=CODE_FONT, size=16, color=ICE, align=PP_ALIGN.CENTER)
    text_box(s, Inches(0), Inches(6.45), Inches(13.333), Inches(0.4),
             "Apache-2.0  ·  Go 1.23+  ·  10 QDPs landed  ·  14 use-case designs",
             font=B_FONT, size=12, color=SLATE, italic=True, align=PP_ALIGN.CENTER)
    notes(s, """TIMING: As long as needed for Q&A.

WHAT TO SAY:
Thank you. Repository is at github.com/bhmortim/quidnug. Apache-2.0 license. Go 1.23+. Ten protocol design proposals landed. Fourteen production-grade use-case designs in the UseCases folder. Happy to take questions — common ones: how does blind signing work in detail, what's the failure mode if guardians collude, how do you bootstrap the first trust edges into a fresh deployment, what's the difference from Hyperledger, and 'why not just use a blockchain.' I have answers for all of those. Go ahead.""")
    p += 1

    return p - start_page
