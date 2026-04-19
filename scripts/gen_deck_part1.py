"""Slides 1–18: Opening, Problem, Insight."""
from pptx.util import Inches, Pt
from pptx.enum.text import PP_ALIGN, MSO_ANCHOR
from gen_deck_core import (
    slide_title, slide_section, slide_content, slide_dark, slide_quote,
    slide_bigstat, set_bg, text_box, bullets, rect, rounded, hexagon, oval,
    shape_text, arrow, notes, card, chip, numbered_circle, table_rows,
    MIDNIGHT, NAVY, TEAL, AMBER, ICE, CLOUD, SLATE, WHITE, GREEN, RED,
    SOFT_TEAL, SOFT_AMBER, H_FONT, B_FONT, CODE_FONT, page_chrome, hex_watermark
)


def build_part1(prs, total):
    # ---------- 1. Title ----------
    s = slide_title(prs)
    notes(s, """TIMING: 30 seconds.

WHAT TO SAY:
Welcome. Today we're going to talk about Quidnug — a decentralized protocol for relational trust, identity, and ownership. It's a Go-based open-source reference implementation with a rich set of protocol design proposals, and it solves a specific class of problems that existing systems solve poorly — things like credit scoring, voting, AI-agent authorization, cross-organization fraud detection, and institutional custody.

KEY POINTS:
• Name: Quidnug — the core identity primitive is the "quid"
• Tagline: Identity, Ownership, Auditable State
• Version: v2026.04 — 10 QDPs landed, 14 production-grade use case designs

TRANSITION:
Let's start with what we'll cover.""")

    # ---------- 2. Agenda ----------
    s = slide_content(prs, "What we'll cover", kicker="AGENDA", page=2, total=total)
    # Six-card grid (3x2)
    cards_data = [
        ("01", "Why trust is broken", "The common failure pattern in credit, reputation, and identity"),
        ("02", "The core insight", "Relational trust instead of universal scores"),
        ("03", "Quidnug concepts", "Quids, trust edges, domains, proof-of-trust"),
        ("04", "Technical primitives", "Nonces, anchors, guardians, gossip, bootstrap"),
        ("05", "14 use cases", "FinTech, AI, Elections, Credit, Healthcare…"),
        ("06", "Getting started", "Code, deployment, integration"),
    ]
    col_x = [Inches(0.6), Inches(5.05), Inches(9.5)]
    row_y = [Inches(1.75), Inches(4.4)]
    card_w = Inches(3.75)
    card_h = Inches(2.35)
    for i, (num, ttl, body) in enumerate(cards_data):
        x = col_x[i % 3]
        y = row_y[i // 3]
        rounded(s, x, y, card_w, card_h, fill=ICE, corner=0.08)
        rect(s, x, y, Inches(0.09), card_h, TEAL)
        oval(s, x + Inches(0.25), y + Inches(0.22), Inches(0.55), Inches(0.48), NAVY)
        text_box(s, x + Inches(0.25), y + Inches(0.22), Inches(0.55), Inches(0.48),
                 num, font=H_FONT, size=16, color=WHITE, bold=True,
                 align=PP_ALIGN.CENTER, anchor=MSO_ANCHOR.MIDDLE)
        text_box(s, x + Inches(0.95), y + Inches(0.22), Inches(2.7), Inches(0.5),
                 ttl, font=H_FONT, size=17, color=MIDNIGHT, bold=True,
                 anchor=MSO_ANCHOR.MIDDLE)
        text_box(s, x + Inches(0.25), y + Inches(0.9), card_w - Inches(0.45),
                 Inches(1.35), body, font=B_FONT, size=12.5, color=SLATE,
                 line_spacing=1.2)
    notes(s, """TIMING: 1 minute.

WHAT TO SAY:
The talk has six parts. I'll spend about ten minutes on each. If you have questions, hold them — I'll leave time at the end. Parts 1 and 2 are conceptual — why trust is broken in the ways you probably already intuit, and what Quidnug replaces the broken parts with. Part 3 and 4 are technical — the primitives that make it work. Parts 5 is where it gets real — fourteen use cases spanning FinTech, AI, elections, credit reporting, healthcare. Part 6 is how you actually pick this up and build with it.

KEY POINTS:
• Six sections, ~10 minutes each
• Hold questions to the end (or send chat)
• Heavy on use cases because the primitives alone aren't interesting

TRANSITION:
Before we dive in, here's the three-sentence version.""")

    # ---------- 3. Three-sentence pitch ----------
    s = slide_content(prs, "The three-sentence pitch", kicker="TL;DR", page=3, total=total)
    # Three stacked cards with big numbers
    lines = [
        ("01", "Quidnug is a Go-based P2P protocol for expressing, computing, and auditing trust — cryptographically, relationally, per-observer.",
         "No universal scores. No central authority. No shared single chain."),
        ("02", "Every participant owns their identity (a quid). Every statement is signed. Every piece of state is replayable from the blockchain.",
         "Guardian-based recovery handles key loss. Time-locked veto handles coercion."),
        ("03", "The protocol is useful wherever you'd say 'this specific party signed this specific thing and I need to verify it didn't get replayed or forged.'",
         "Banks, AI agents, voters, patients, software maintainers — all fit this shape."),
    ]
    for i, (num, head, sub) in enumerate(lines):
        y = Inches(1.85 + i * 1.65)
        rounded(s, Inches(0.6), y, Inches(12.1), Inches(1.45), fill=ICE, corner=0.06)
        rect(s, Inches(0.6), y, Inches(0.09), Inches(1.45), (NAVY if i == 0 else (TEAL if i == 1 else AMBER)))
        text_box(s, Inches(0.85), y + Inches(0.1), Inches(0.7), Inches(0.5),
                 num, font=H_FONT, size=24, color=TEAL, bold=True,
                 anchor=MSO_ANCHOR.TOP)
        text_box(s, Inches(1.55), y + Inches(0.15), Inches(11), Inches(0.65),
                 head, font=H_FONT, size=17, color=MIDNIGHT, bold=True)
        text_box(s, Inches(1.55), y + Inches(0.82), Inches(11), Inches(0.6),
                 sub, font=B_FONT, size=13, color=SLATE, italic=True)
    notes(s, """TIMING: 1.5 minutes.

WHAT TO SAY:
If you only remember three things from today, make them these. First: Quidnug is about trust, but it deliberately refuses to produce a single universal score. Different observers answer the question 'how much do I trust Bob?' differently, and that's a feature, not a bug. Second: it's cryptographic and user-owned. Every claim is signed by someone identifiable. Keys are recoverable through M-of-N guardian quorums, not through central escrow. Third: the real value is in use cases — wherever you'd answer 'this particular person signed this' matters more than 'this database row exists', Quidnug fits.

KEY POINTS:
• Relational, not universal
• User-owned keys with guardian recovery
• Problem-space: signed statements + verifiable history

TRANSITION:
Who is this actually for?""")

    # ---------- 4. Who this is for ----------
    s = slide_content(prs, "Who this is for", kicker="AUDIENCE", page=4, total=total)
    # Four persona cards
    personas = [
        ("FinTech engineer", "Multi-party approval, replay-safe transactions, guardian-recoverable institutional keys.",
         TEAL, "M-of-N signing"),
        ("AI platform builder", "Signed provenance from training data to model to inference; time-locked agent authorization.",
         AMBER, "Agent auth"),
        ("Civic tech / gov", "Elections, credentials, consent. Public verifiability without trusting a vendor.",
         NAVY, "Verifiability"),
        ("Security-conscious startup", "Your product has 'who signed this' as a load-bearing question. Don't reinvent the wheel.",
         GREEN, "Audit trail"),
    ]
    for i, (who, desc, accent, badge) in enumerate(personas):
        x = Inches(0.6 + (i % 2) * 6.15)
        y = Inches(1.8 + (i // 2) * 2.65)
        card(s, x, y, Inches(5.95), Inches(2.4), who, body=desc, accent=accent,
             fill=ICE, badge_text=badge)
    notes(s, """TIMING: 1 minute.

WHAT TO SAY:
Four personas I've seen find this useful. FinTech engineers building anything that requires two or more people to sign off, where losing a signing key is a real problem and replays would be devastating. AI platform builders who need to prove provenance — this training data, this model, this specific inference — or authorize autonomous agents safely. Civic tech and government teams building elections, credentialing, or consent-management systems where 'trust the vendor' isn't an acceptable answer. And small security-conscious teams where 'who signed this?' is already a first-class question — you probably already have half of what Quidnug is, just in an ad-hoc form. It's worth comparing.

KEY POINTS:
• Multi-party approval — FinTech
• Provenance + authorization — AI
• Public verifiability — civic tech
• Small teams with signing-heavy products

TRANSITION:
If you're in one of these buckets, the most useful thing I can say upfront is the TL;DR for the whole talk.""")

    # ---------- 5. TL;DR ----------
    s = slide_dark(prs, "Three things to remember", kicker="TL;DR (REAL)", page=5, total=total)
    # Three big horizontal cards with icons
    items = [
        ("Trust is relational.",
         "The protocol never produces a universal 'Bob's trust score.' Different observers answer differently.",
         "T", TEAL),
        ("Identity is owned.",
         "Quids are generated by the user. Authorities can endorse but not issue. Guardian-M-of-N recovers lost keys.",
         "I", AMBER),
        ("State is cryptographically auditable.",
         "Every transaction is signed. Every event stream is append-only. Anyone can replay and verify.",
         "S", GREEN),
    ]
    for i, (head, sub, icon, color) in enumerate(items):
        y = Inches(1.8 + i * 1.6)
        rounded(s, Inches(0.6), y, Inches(12.1), Inches(1.4), fill=NAVY, corner=0.06)
        # Icon circle
        oval(s, Inches(0.85), y + Inches(0.3), Inches(0.8), Inches(0.8), color)
        text_box(s, Inches(0.85), y + Inches(0.3), Inches(0.8), Inches(0.8),
                 icon, font=H_FONT, size=34, color=MIDNIGHT, bold=True,
                 align=PP_ALIGN.CENTER, anchor=MSO_ANCHOR.MIDDLE)
        text_box(s, Inches(1.9), y + Inches(0.22), Inches(10.5), Inches(0.55),
                 head, font=H_FONT, size=22, color=WHITE, bold=True)
        text_box(s, Inches(1.9), y + Inches(0.78), Inches(10.5), Inches(0.6),
                 sub, font=B_FONT, size=14, color=CLOUD, italic=True)
    notes(s, """TIMING: 1.5 minutes.

WHAT TO SAY:
These are the through-lines. First: relational trust. The protocol refuses to compute one universal score because real-world trust doesn't work that way. You trust your plumber for plumbing, not for tax advice. Second: identity sovereignty. You generate your own cryptographic identity. Authorities can endorse you, but they don't issue you an identity. And third: everything that happens is cryptographically auditable — not in a 'trust our audit firm' way but in a 'run the query yourself' way. These three principles drive every design decision we'll look at next.

KEY POINTS:
• Relational, not universal — observer-centric trust
• Self-sovereign — BYO-quid
• Cryptographic audit — replay anywhere

TRANSITION:
Let's see why today's trust systems violate these principles — and why that matters.""")

    # ---------- 6. SECTION 1 divider ----------
    slide_section(prs, 1, "PART ONE", "Trust is broken")

    # ---------- 7. How trust systems work today ----------
    s = slide_content(prs, "How trust systems work today", kicker="STATUS QUO", page=7, total=total)
    # Left: diagram box with a central DB and arrows
    rounded(s, Inches(0.6), Inches(1.75), Inches(6.5), Inches(4.8), ICE, corner=0.04)
    text_box(s, Inches(0.6), Inches(1.85), Inches(6.5), Inches(0.4),
             "THE CENTRAL DATABASE PATTERN", font=B_FONT, size=11,
             color=SLATE, bold=True, align=PP_ALIGN.CENTER)
    # Central "DB" node
    rounded(s, Inches(2.8), Inches(3.6), Inches(2.1), Inches(1.15), NAVY, corner=0.1)
    text_box(s, Inches(2.8), Inches(3.6), Inches(2.1), Inches(1.15),
             "Central\nDatabase",
             font=H_FONT, size=16, color=WHITE, bold=True,
             align=PP_ALIGN.CENTER, anchor=MSO_ANCHOR.MIDDLE)
    # User clients around
    positions = [
        (Inches(1.1), Inches(2.6), "User A"),
        (Inches(5.7), Inches(2.6), "User B"),
        (Inches(1.1), Inches(5.1), "User C"),
        (Inches(5.7), Inches(5.1), "User D"),
    ]
    for px, py, lab in positions:
        oval(s, px, py, Inches(0.95), Inches(0.7), TEAL)
        text_box(s, px, py, Inches(0.95), Inches(0.7), lab,
                 font=B_FONT, size=11, color=WHITE, bold=True,
                 align=PP_ALIGN.CENTER, anchor=MSO_ANCHOR.MIDDLE)
        # Arrow to center
        arrow(s, px + Inches(0.47), py + Inches(0.35),
              Inches(3.85), Inches(4.18), color=SLATE, width=1.2)
    # Labels at edges
    text_box(s, Inches(0.6), Inches(6.05), Inches(6.5), Inches(0.5),
             "All parties send data to one authority. Authority scores. Authority decides.",
             font=B_FONT, size=11.5, color=SLATE, italic=True, align=PP_ALIGN.CENTER)

    # Right: problems list
    bullets(s, Inches(7.5), Inches(1.9), Inches(5.4), Inches(5),
            [
                ("Opaque", 0, True),
                "You can't see the calculation.",
                ("Single point of failure", 0, True),
                "One breach = everyone's data compromised.",
                ("Universal score", 0, True),
                "Same number used for every context.",
                ("No ownership", 0, True),
                "The authority owns your history, not you.",
                ("Slow to correct", 0, True),
                "Errors take months to fix.",
                ("Hard to appeal", 0, True),
                "Disputes go through the same authority.",
            ], size=14)
    notes(s, """TIMING: 2 minutes.

WHAT TO SAY:
Look at any trust-heavy system today — credit bureaus, social-media reputation, government registries — and you'll find this pattern: every participant sends data to a central database, the central database computes a score or attests to facts, and everyone else queries the central database for the answer. This works reasonably well when everyone agrees on who the authority is. It fails hard in six specific ways shown on the right: opacity, single-point-of-failure breaches, one-size-fits-all scoring, lack of user data ownership, painfully slow corrections, and weak appeal processes. Every one of these is structural, not incidental.

KEY POINTS:
• The pattern: centralize → score → serve
• Six structural problems
• Equifax 2017 breach, FICO opacity, slow dispute cycles are all symptoms

TRANSITION:
Let's dig into the biggest one — the universal score.""")

    # ---------- 8. Universal-score problem ----------
    s = slide_content(prs, "Problem 1: one score, many contexts", kicker="UNIVERSAL-SCORE", page=8, total=total)
    # Illustration: one number spreading out to many use cases
    oval(s, Inches(5.7), Inches(2.0), Inches(2.0), Inches(1.4), AMBER)
    text_box(s, Inches(5.7), Inches(2.0), Inches(2.0), Inches(1.4),
             "720", font=H_FONT, size=48, color=MIDNIGHT, bold=True,
             align=PP_ALIGN.CENTER, anchor=MSO_ANCHOR.MIDDLE)
    text_box(s, Inches(5.7), Inches(3.45), Inches(2.0), Inches(0.3),
             "Your FICO score", font=B_FONT, size=11, color=SLATE,
             italic=True, align=PP_ALIGN.CENTER)

    # Five arrows radiating out to contexts
    contexts = [
        ("Mortgage", Inches(0.8), Inches(4.7), TEAL),
        ("Car loan", Inches(3.2), Inches(5.3), TEAL),
        ("Credit card", Inches(5.7), Inches(5.5), TEAL),
        ("Apartment", Inches(8.3), Inches(5.3), TEAL),
        ("Job check", Inches(10.7), Inches(4.7), TEAL),
    ]
    for name, tx, ty, c in contexts:
        rounded(s, tx, ty, Inches(1.9), Inches(0.7), c, corner=0.15)
        text_box(s, tx, ty, Inches(1.9), Inches(0.7), name,
                 font=B_FONT, size=13, color=WHITE, bold=True,
                 align=PP_ALIGN.CENTER, anchor=MSO_ANCHOR.MIDDLE)
        # Arrow from center score to this context
        arrow(s, Inches(6.7), Inches(3.4), tx + Inches(0.95), ty,
              color=SLATE, width=1)
    # Commentary below
    rounded(s, Inches(0.6), Inches(6.35), Inches(12.1), Inches(0.75), ICE, corner=0.04)
    text_box(s, Inches(0.85), Inches(6.4), Inches(11.6), Inches(0.65),
             "The same number used for 30-year mortgages, 60-month auto loans, employment screens, and apartment checks. None of these contexts have the same risk profile.",
             font=H_FONT, size=13, color=MIDNIGHT, italic=True,
             anchor=MSO_ANCHOR.MIDDLE)
    notes(s, """TIMING: 1.5 minutes.

WHAT TO SAY:
Here's a very concrete example. Your FICO score is a single three-digit number, say 720. That same 720 is consulted when you apply for a mortgage, a car loan, a credit card, a rental apartment, and potentially by an employer doing a background check. But these contexts have radically different risk profiles. A mortgage has thirty years of downside exposure for the lender. A credit card has maybe a few thousand dollars and the lender can close the line any time. These aren't the same underwriting problem, and yet one number drives them all. The issue isn't FICO specifically — it's the architectural choice to produce ONE number. Any system that does that will have this problem.

KEY POINTS:
• One score, five+ radically different risk contexts
• Mortgage risk ≠ credit-card risk
• Problem is architectural, not formulaic

TRANSITION:
Next: who actually owns your identity?""")

    # ---------- 9. Identity is rented ----------
    s = slide_content(prs, "Problem 2: you don't own your identity", kicker="PLATFORM LOCK-IN", page=9, total=total)
    # Icon row: your identity on various platforms
    platforms = [
        ("Google", NAVY, "Your email\nidentity"),
        ("Facebook", NAVY, "Your social\ngraph"),
        ("Credit Bureau", NAVY, "Your credit\nhistory"),
        ("State DMV", NAVY, "Your driver's\nidentity"),
        ("Employer IAM", NAVY, "Your work\nidentity"),
    ]
    for i, (name, col, body) in enumerate(platforms):
        x = Inches(0.65 + i * 2.52)
        y = Inches(2.0)
        rounded(s, x, y, Inches(2.35), Inches(2.2), ICE, corner=0.06)
        rect(s, x, y, Inches(2.35), Inches(0.5), NAVY)
        text_box(s, x, y, Inches(2.35), Inches(0.5), name,
                 font=H_FONT, size=15, color=WHITE, bold=True,
                 align=PP_ALIGN.CENTER, anchor=MSO_ANCHOR.MIDDLE)
        text_box(s, x + Inches(0.1), y + Inches(0.65), Inches(2.15),
                 Inches(1.5), body, font=B_FONT, size=13, color=MIDNIGHT,
                 align=PP_ALIGN.CENTER, anchor=MSO_ANCHOR.MIDDLE,
                 line_spacing=1.3)
    # Bottom row: what happens
    rounded(s, Inches(0.6), Inches(4.5), Inches(12.1), Inches(2.3), MIDNIGHT, corner=0.06)
    text_box(s, Inches(1), Inches(4.7), Inches(11.3), Inches(0.5),
             "What happens when the platform…", font=B_FONT, size=13,
             color=TEAL, bold=True)
    problems = [
        ("Bans you", "Your identity assets disappear"),
        ("Gets breached", "Your data is public (Equifax 2017: 147M records)"),
        ("Shuts down", "Everything you built is locked"),
        ("Is subpoenaed", "Government gets everything the platform had"),
    ]
    for i, (t, d) in enumerate(problems):
        x = Inches(0.9 + i * 3.0)
        y = Inches(5.3)
        oval(s, x, y, Inches(0.45), Inches(0.38), RED)
        text_box(s, x, y, Inches(0.45), Inches(0.38), "!",
                 font=H_FONT, size=18, color=WHITE, bold=True,
                 align=PP_ALIGN.CENTER, anchor=MSO_ANCHOR.MIDDLE)
        text_box(s, x + Inches(0.55), y - Inches(0.05), Inches(2.4), Inches(0.45),
                 t, font=H_FONT, size=14, color=WHITE, bold=True,
                 anchor=MSO_ANCHOR.MIDDLE)
        text_box(s, x, y + Inches(0.5), Inches(2.85), Inches(0.8),
                 d, font=B_FONT, size=11, color=CLOUD, italic=True,
                 line_spacing=1.2)
    notes(s, """TIMING: 1.5 minutes.

WHAT TO SAY:
Every identity you have is rented from someone. Your Google identity is Google's. Your Facebook profile belongs to Facebook. Your credit history is owned by three private corporations. Your driver's identity belongs to the state DMV. Your work identity is tied to your employer's IAM system. This is fine until it isn't. When a platform bans you, gets breached, shuts down, or is subpoenaed, your identity and everything attached to it are at risk. Equifax 2017 breach exposed 147 million people's most sensitive data. That wasn't an anomaly — it was a structural consequence of centralizing all that data in one place.

KEY POINTS:
• You have ~5 platform identities — none yours
• Platform failure modes all cost you
• Centralization creates the target that gets hit

TRANSITION:
Problem three is the one that hits hardest when you lose a key.""")

    # ---------- 10. Key recovery ----------
    s = slide_content(prs, "Problem 3: key recovery is a disaster", kicker="COLD REALITY", page=10, total=total)
    # Left: big stat
    rounded(s, Inches(0.6), Inches(1.8), Inches(6), Inches(5), MIDNIGHT, corner=0.04)
    text_box(s, Inches(0.85), Inches(2.1), Inches(5.5), Inches(1.2),
             "$140,000,000,000+",
             font=H_FONT, size=36, color=AMBER, bold=True)
    text_box(s, Inches(0.85), Inches(3.0), Inches(5.5), Inches(0.5),
             "IN LOST BITCOIN",
             font=B_FONT, size=14, color=TEAL, bold=True)
    text_box(s, Inches(0.85), Inches(3.55), Inches(5.5), Inches(3),
             "Estimated value of bitcoin permanently inaccessible due to lost private keys.\n\nLose the key, lose the asset — with no recovery path. No help desk, no forgot-password flow.\n\nThis is just crypto. The same architecture governs any public-key system — GPG-signed software releases, SSH keys, SAML certificates.",
             font=B_FONT, size=13, color=CLOUD, line_spacing=1.35)

    # Right: alternative recovery options (all bad)
    text_box(s, Inches(7.1), Inches(1.9), Inches(5.7), Inches(0.4),
             "THE RECOVERY OPTIONS TODAY", font=B_FONT, size=11,
             color=SLATE, bold=True)
    options = [
        ("Central escrow", "Company holds your keys. Defeats the point.", RED),
        ("Seed phrase on paper", "User error in storage = permanent loss.", AMBER),
        ("Hardware wallet backup", "Two points of failure instead of one.", AMBER),
        ("Social recovery (Gnosis Safe, etc.)", "Good direction, but app-layer and not portable.", GREEN),
        ("Forgot password?", "Doesn't exist.", RED),
    ]
    for i, (name, desc, col) in enumerate(options):
        y = Inches(2.3 + i * 0.85)
        rounded(s, Inches(7.1), y, Inches(5.7), Inches(0.75), ICE, corner=0.08)
        rect(s, Inches(7.1), y, Inches(0.08), Inches(0.75), col)
        text_box(s, Inches(7.3), y + Inches(0.1), Inches(5.4), Inches(0.35),
                 name, font=H_FONT, size=13, color=MIDNIGHT, bold=True)
        text_box(s, Inches(7.3), y + Inches(0.42), Inches(5.4), Inches(0.3),
                 desc, font=B_FONT, size=11, color=SLATE, italic=True)
    notes(s, """TIMING: 2 minutes.

WHAT TO SAY:
Now we get to the really painful one. The estimates vary, but somewhere between ten and twenty percent of all bitcoin ever mined is permanently inaccessible because people lost their private keys. That's over 140 billion dollars of lost value in just one system. And this isn't crypto-specific — it's the general public-key-cryptography problem. Anywhere you use signing keys for identity or authority, losing the key means losing the thing. GPG-signed open source releases, SSH keys, SAML certificates — same shape. The existing recovery options are all bad. Central escrow defeats the security model. Seed phrases fail to user error. Hardware wallet backups give you two points of failure instead of one. Gnosis Safe and similar are app-layer — not portable across contexts. And the 'forgot password' flow famously doesn't exist. Quidnug solves this with guardian-based recovery — a M-of-N quorum with time-locked veto. We'll see that in Part 2.

KEY POINTS:
• $140B+ in lost bitcoin alone
• Universal problem for any PKI system
• Existing alternatives all bad
• Guardian recovery — coming in Part 4

TRANSITION:
Fourth problem: multi-party approval is always ad-hoc.""")

    # ---------- 11. Multi-party auth is ad-hoc ----------
    s = slide_content(prs, "Problem 4: multi-party authorization is ad-hoc", kicker="EVERY SYSTEM INVENTS ITS OWN", page=11, total=total)
    # Left: examples of ad-hoc schemes
    text_box(s, Inches(0.6), Inches(1.85), Inches(6), Inches(0.4),
             "TODAY — everyone rolls their own:",
             font=B_FONT, size=12, color=SLATE, bold=True)
    rolls = [
        ("Banking", "2-of-3 officer spreadsheet + scanned PDF ticket"),
        ("AWS / GCP", "IAM policies + approval workflows in Slack"),
        ("SaaS admin", "Email threads + 'reply with YES to approve'"),
        ("DAO treasury", "Gnosis Safe or Zodiac modules — product-specific"),
        ("Code signing", "One GPG key + Slack notification to eng-leads"),
        ("Bank wire approval", "Manual workflow, database row, screenshot audit"),
    ]
    for i, (area, desc) in enumerate(rolls):
        y = Inches(2.35 + i * 0.7)
        rounded(s, Inches(0.6), y, Inches(6), Inches(0.6), ICE, corner=0.08)
        text_box(s, Inches(0.8), y, Inches(2), Inches(0.6), area,
                 font=H_FONT, size=13, color=MIDNIGHT, bold=True,
                 anchor=MSO_ANCHOR.MIDDLE)
        text_box(s, Inches(2.75), y, Inches(3.7), Inches(0.6), desc,
                 font=B_FONT, size=11, color=SLATE, italic=True,
                 anchor=MSO_ANCHOR.MIDDLE)

    # Right: consequences
    rounded(s, Inches(7.1), Inches(1.85), Inches(5.7), Inches(5), MIDNIGHT, corner=0.04)
    text_box(s, Inches(7.3), Inches(2.1), Inches(5.3), Inches(0.5),
             "Consequences", font=H_FONT, size=22, color=TEAL, bold=True)
    consequences = [
        "Each system re-invents M-of-N from scratch",
        "Audit trails live in 4 different places",
        "Cross-system approvals are impossible",
        "Replay attacks are easy (no binding nonce)",
        "Recovery when a signer leaves = emergency meeting",
        "Cryptographic evidence for courts is patchy",
    ]
    bullets(s, Inches(7.3), Inches(2.7), Inches(5.3), Inches(4),
            consequences, size=13.5, color=CLOUD)
    notes(s, """TIMING: 1.5 minutes.

WHAT TO SAY:
Here's the fourth problem. Every serious system needs multi-party authorization for high-stakes actions — wire transfers, cloud-admin changes, code signing, DAO treasury moves — but every system reinvents the M-of-N wheel from scratch. Banking does it with scanned PDFs and spreadsheets. AWS has IAM policies plus Slack approval bots. DAOs use Gnosis Safe. Code signing still largely uses a single GPG key plus an out-of-band notification. Each is locally reasonable, but the sum is a mess. You can't compose them, you can't audit across them, and losing a signer triggers an emergency meeting. What you actually want is a primitive — 'M of these N quids can authorize this class of thing, with a time-locked veto, and recovery built in.' Quidnug provides that primitive directly, and we'll look at it in Part 4.

KEY POINTS:
• Everyone invents M-of-N
• Audit trails scattered
• No composition across systems
• Replay protection often absent or weak

TRANSITION:
One more problem — then we'll talk solutions.""")

    # ---------- 12. Common root ----------
    s = slide_content(prs, "The common root: centralization + universal scoring", kicker="DIAGNOSIS", page=12, total=total)
    # Venn-like diagram
    rounded(s, Inches(0.6), Inches(1.8), Inches(12.1), Inches(3.0), ICE, corner=0.05)
    # Two overlapping circles
    oval(s, Inches(1.6), Inches(2.1), Inches(4.6), Inches(2.5), TEAL, line=None)
    oval(s, Inches(7.2), Inches(2.1), Inches(4.6), Inches(2.5), AMBER, line=None)
    text_box(s, Inches(1.6), Inches(2.4), Inches(4.6), Inches(0.5),
             "CENTRALIZED JUDGE", font=B_FONT, size=13, color=WHITE,
             bold=True, align=PP_ALIGN.CENTER)
    text_box(s, Inches(1.7), Inches(3.0), Inches(4.4), Inches(1.4),
             "One authority\ndecides for everyone",
             font=H_FONT, size=18, color=MIDNIGHT, bold=True,
             align=PP_ALIGN.CENTER, anchor=MSO_ANCHOR.MIDDLE)
    text_box(s, Inches(7.2), Inches(2.4), Inches(4.6), Inches(0.5),
             "UNIVERSAL SCORE", font=B_FONT, size=13, color=MIDNIGHT,
             bold=True, align=PP_ALIGN.CENTER)
    text_box(s, Inches(7.3), Inches(3.0), Inches(4.4), Inches(1.4),
             "One number used\nacross all contexts",
             font=H_FONT, size=18, color=MIDNIGHT, bold=True,
             align=PP_ALIGN.CENTER, anchor=MSO_ANCHOR.MIDDLE)
    # Bottom block — all five problems descend from these
    rounded(s, Inches(0.6), Inches(5.1), Inches(12.1), Inches(1.9), MIDNIGHT, corner=0.05)
    text_box(s, Inches(0.85), Inches(5.25), Inches(11.6), Inches(0.5),
             "EVERY SYMPTOM WE LISTED DESCENDS FROM THESE TWO CHOICES",
             font=B_FONT, size=12, color=TEAL, bold=True,
             align=PP_ALIGN.CENTER)
    descended = [
        "Opacity", "Single point of failure", "Context-blind scoring",
        "No user ownership", "Slow corrections", "Ad-hoc M-of-N",
    ]
    for i, item in enumerate(descended):
        x = Inches(0.85 + (i % 3) * 3.9)
        y = Inches(5.8 + (i // 3) * 0.5)
        chip(s, x, y, Inches(3.7), Inches(0.4), item, fill=NAVY,
             color=WHITE, size=12)
    notes(s, """TIMING: 1 minute.

WHAT TO SAY:
All the specific problems — opacity, breaches, universal scores, weak recovery, slow corrections, ad-hoc M-of-N — descend from two architectural choices. First, a centralized judge — one authority aggregates data and makes decisions for everyone. Second, a universal score — one number or grade used across contexts that have nothing in common. Any design you build on top of those two choices will inherit the six downstream problems. If we want to fix this, we need to question the two root choices, not just patch symptoms. That's the Quidnug premise.

KEY POINTS:
• Root 1: centralized judgment
• Root 2: universal score
• All downstream problems descend from these
• Fix the roots, not the leaves

TRANSITION:
OK. Enough problem. Let's talk insight — what actually replaces the broken architecture.""")

    # ---------- 13. SECTION 2 divider ----------
    slide_section(prs, 2, "PART TWO", "The core insight")

    # ---------- 14. How humans actually trust ----------
    s = slide_content(prs, "How humans actually trust", kicker="THE INTUITION", page=14, total=total)
    # Story flow: 5 steps with circles and arrows
    rounded(s, Inches(0.6), Inches(1.8), Inches(12.1), Inches(3.2), ICE, corner=0.05)
    text_box(s, Inches(0.8), Inches(1.95), Inches(11.6), Inches(0.4),
             "YOU NEED A CONTRACTOR FOR A HOME RENOVATION…",
             font=B_FONT, size=12, color=SLATE, bold=True)
    steps = [
        ("You", "Ask your friend", NAVY),
        ("Carol", "Longstanding friend\nYou trust deeply", TEAL),
        ("Carol says:", "\"I worked with Bob.\nHe's good.\"", AMBER),
        ("You trust Bob", "Transitively,\nvia Carol", GREEN),
    ]
    for i, (a, b, c) in enumerate(steps):
        x = Inches(0.9 + i * 3.05)
        y = Inches(2.6)
        oval(s, x, y, Inches(1.4), Inches(1.4), c)
        text_box(s, x, y, Inches(1.4), Inches(1.4), a,
                 font=H_FONT, size=14, color=WHITE, bold=True,
                 align=PP_ALIGN.CENTER, anchor=MSO_ANCHOR.MIDDLE)
        text_box(s, x - Inches(0.3), y + Inches(1.45), Inches(2), Inches(0.7),
                 b, font=B_FONT, size=11, color=MIDNIGHT,
                 align=PP_ALIGN.CENTER, line_spacing=1.2)
        if i < 3:
            arrow(s, x + Inches(1.45), y + Inches(0.7),
                  x + Inches(3.0), y + Inches(0.7), color=SLATE, width=1.5)
    # Insight at bottom
    rounded(s, Inches(0.6), Inches(5.3), Inches(12.1), Inches(1.65), MIDNIGHT, corner=0.05)
    text_box(s, Inches(0.85), Inches(5.45), Inches(11.6), Inches(1.5),
             "Trust isn't a universal number. It flows through your personal network, with natural decay at each hop, and terminates in YOUR decision — made from YOUR vantage point. This is Quidnug's whole model.",
             font=H_FONT, size=18, color=WHITE, italic=True,
             anchor=MSO_ANCHOR.MIDDLE, line_spacing=1.3)
    notes(s, """TIMING: 1.5 minutes.

WHAT TO SAY:
Let's start with how humans actually trust each other, because that's what we're trying to match. You need a contractor. You don't know any personally. You ask your friend Carol, whom you trust strongly because you've known her for years. Carol says 'I worked with Bob, he's solid.' You now transitively trust Bob — not as much as you trust Carol directly, but enough to interview him. Notice what just happened. You didn't consult a 'universal contractor score.' You didn't trust a central rating aggregator. You relied on your personal network, trusting through a chain of specific people you know. Trust flowed, with decay — Carol's 0.9 to Bob meets your 0.9 to Carol, so your transitive trust in Bob is 0.81. That's the whole Quidnug model expressed in one sentence.

KEY POINTS:
• You → Carol → Bob (transitive)
• Decay at each hop is natural
• Terminates in YOUR decision, not a central authority's
• Quidnug formalizes exactly this

TRANSITION:
Let's see the math.""")

    # ---------- 15. The math ----------
    s = slide_content(prs, "Transitive trust: the math", kicker="HOW QUIDNUG COMPUTES IT", page=15, total=total)
    # Top: equation
    rounded(s, Inches(0.6), Inches(1.85), Inches(12.1), Inches(1.4), MIDNIGHT, corner=0.05)
    text_box(s, Inches(0.85), Inches(2.0), Inches(11.6), Inches(0.4),
             "TRUST COMPUTATION", font=B_FONT, size=12, color=TEAL, bold=True)
    text_box(s, Inches(0.85), Inches(2.3), Inches(11.6), Inches(0.9),
             "trust(A, D)  =  max over all paths  ( product of trust along the path )",
             font=CODE_FONT, size=22, color=WHITE, bold=True,
             align=PP_ALIGN.CENTER, anchor=MSO_ANCHOR.MIDDLE)

    # Middle: two example paths
    y_base = Inches(3.55)
    text_box(s, Inches(0.6), y_base, Inches(12.1), Inches(0.4),
             "EXAMPLE: YOU WANT TO COMPUTE YOUR TRUST IN 'D'",
             font=B_FONT, size=12, color=SLATE, bold=True)
    # Path 1
    y1 = Inches(4.0)
    # A->B->D: 0.9 * 0.7 = 0.63
    nodes1 = [("A (you)", Inches(0.9), NAVY),
              ("B", Inches(4.0), TEAL),
              ("D (target)", Inches(7.1), AMBER)]
    vals1 = [("0.9", Inches(2.3), TEAL),
             ("0.7", Inches(5.4), TEAL)]
    for name, xx, col in nodes1:
        oval(s, xx, y1, Inches(1.25), Inches(0.9), col)
        text_box(s, xx, y1, Inches(1.25), Inches(0.9), name,
                 font=H_FONT, size=12, color=WHITE, bold=True,
                 align=PP_ALIGN.CENTER, anchor=MSO_ANCHOR.MIDDLE)
    for v, xx, col in vals1:
        text_box(s, xx, y1 + Inches(0.95), Inches(0.9), Inches(0.4), v,
                 font=H_FONT, size=14, color=col, bold=True,
                 align=PP_ALIGN.CENTER)
    # Arrows
    arrow(s, Inches(0.9) + Inches(1.25), y1 + Inches(0.45),
          Inches(4.0), y1 + Inches(0.45), color=SLATE, width=1.5)
    arrow(s, Inches(4.0) + Inches(1.25), y1 + Inches(0.45),
          Inches(7.1), y1 + Inches(0.45), color=SLATE, width=1.5)
    # Result
    rounded(s, Inches(8.7), y1, Inches(4.0), Inches(0.9), TEAL, corner=0.15)
    text_box(s, Inches(8.7), y1, Inches(4.0), Inches(0.9),
             "Path 1:  0.9 × 0.7 = 0.63",
             font=CODE_FONT, size=16, color=WHITE, bold=True,
             align=PP_ALIGN.CENTER, anchor=MSO_ANCHOR.MIDDLE)

    # Path 2
    y2 = Inches(5.5)
    nodes2 = [("A", Inches(0.9), NAVY),
              ("C", Inches(4.0), TEAL),
              ("D", Inches(7.1), AMBER)]
    vals2 = [("0.6", Inches(2.3), RED),
             ("0.8", Inches(5.4), TEAL)]
    for name, xx, col in nodes2:
        oval(s, xx, y2, Inches(1.25), Inches(0.9), col)
        text_box(s, xx, y2, Inches(1.25), Inches(0.9), name,
                 font=H_FONT, size=12, color=WHITE, bold=True,
                 align=PP_ALIGN.CENTER, anchor=MSO_ANCHOR.MIDDLE)
    for v, xx, col in vals2:
        text_box(s, xx, y2 + Inches(0.95), Inches(0.9), Inches(0.4), v,
                 font=H_FONT, size=14, color=col, bold=True,
                 align=PP_ALIGN.CENTER)
    arrow(s, Inches(0.9) + Inches(1.25), y2 + Inches(0.45),
          Inches(4.0), y2 + Inches(0.45), color=SLATE, width=1.5)
    arrow(s, Inches(4.0) + Inches(1.25), y2 + Inches(0.45),
          Inches(7.1), y2 + Inches(0.45), color=SLATE, width=1.5)
    rounded(s, Inches(8.7), y2, Inches(4.0), Inches(0.9), AMBER, corner=0.15)
    text_box(s, Inches(8.7), y2, Inches(4.0), Inches(0.9),
             "Path 2:  0.6 × 0.8 = 0.48",
             font=CODE_FONT, size=16, color=MIDNIGHT, bold=True,
             align=PP_ALIGN.CENTER, anchor=MSO_ANCHOR.MIDDLE)

    # Final result
    rounded(s, Inches(0.6), Inches(6.55), Inches(12.1), Inches(0.55),
            MIDNIGHT, corner=0.04)
    text_box(s, Inches(0.85), Inches(6.55), Inches(11.6), Inches(0.55),
             "Final:  trust(A → D)  =  MAX(0.63, 0.48)  =  0.63  ·  Use the strongest path, not an average.",
             font=CODE_FONT, size=14, color=TEAL, bold=True,
             align=PP_ALIGN.CENTER, anchor=MSO_ANCHOR.MIDDLE)
    notes(s, """TIMING: 2 minutes.

WHAT TO SAY:
Here's the actual math. Trust from A to D equals the maximum over all paths from A to D of the product of trust values along that path. Concretely, say you want to compute your trust in D and you have two paths. First path: you trust B at 0.9, B trusts D at 0.7. Product: 0.63. Second path: you trust C at only 0.6, C trusts D strongly at 0.8. Product: 0.48. Quidnug picks the strongest path — 0.63 via B — not an average, not a sum, the MAX. This matches intuition: one strong recommendation from someone you deeply trust beats a weaker recommendation from someone you trust less, even if the weaker path reaches through someone who knows the target better. The algorithm is a depth-bounded breadth-first search. Default max depth is five hops. Runs in milliseconds on a graph with hundreds of thousands of edges.

KEY POINTS:
• MAX of paths (not average)
• Multiply trust levels along a path
• Depth-bounded BFS, fast
• Default max depth: 5

TRANSITION:
Now the magic moment: different observers get different answers.""")

    # ---------- 16. Different observers, different answers ----------
    s = slide_content(prs, "Different observers, different answers", kicker="THE KEY PROPERTY", page=16, total=total)
    # Left panel: Alice's view
    rounded(s, Inches(0.6), Inches(1.85), Inches(6), Inches(4.9), ICE, corner=0.05)
    rect(s, Inches(0.6), Inches(1.85), Inches(6), Inches(0.55), NAVY)
    text_box(s, Inches(0.6), Inches(1.85), Inches(6), Inches(0.55),
             "ALICE'S VIEW of Bob",
             font=H_FONT, size=16, color=WHITE, bold=True,
             align=PP_ALIGN.CENTER, anchor=MSO_ANCHOR.MIDDLE)
    # Alice graph
    oval(s, Inches(1.0), Inches(3.1), Inches(1.0), Inches(0.75), TEAL)
    text_box(s, Inches(1.0), Inches(3.1), Inches(1.0), Inches(0.75), "Alice",
             font=B_FONT, size=11, color=WHITE, bold=True,
             align=PP_ALIGN.CENTER, anchor=MSO_ANCHOR.MIDDLE)
    oval(s, Inches(2.9), Inches(3.1), Inches(1.0), Inches(0.75), NAVY)
    text_box(s, Inches(2.9), Inches(3.1), Inches(1.0), Inches(0.75), "Carol",
             font=B_FONT, size=11, color=WHITE, bold=True,
             align=PP_ALIGN.CENTER, anchor=MSO_ANCHOR.MIDDLE)
    oval(s, Inches(4.8), Inches(3.1), Inches(1.0), Inches(0.75), AMBER)
    text_box(s, Inches(4.8), Inches(3.1), Inches(1.0), Inches(0.75), "Bob",
             font=B_FONT, size=11, color=MIDNIGHT, bold=True,
             align=PP_ALIGN.CENTER, anchor=MSO_ANCHOR.MIDDLE)
    arrow(s, Inches(2.0), Inches(3.47), Inches(2.9), Inches(3.47), color=SLATE, width=1.5)
    arrow(s, Inches(3.9), Inches(3.47), Inches(4.8), Inches(3.47), color=SLATE, width=1.5)
    text_box(s, Inches(2.0), Inches(3.85), Inches(0.9), Inches(0.3), "0.9",
             font=H_FONT, size=12, color=TEAL, bold=True, align=PP_ALIGN.CENTER)
    text_box(s, Inches(3.9), Inches(3.85), Inches(0.9), Inches(0.3), "0.8",
             font=H_FONT, size=12, color=TEAL, bold=True, align=PP_ALIGN.CENTER)
    rounded(s, Inches(1.0), Inches(4.5), Inches(4.8), Inches(0.9), TEAL, corner=0.15)
    text_box(s, Inches(1.0), Inches(4.5), Inches(4.8), Inches(0.9),
             "trust(Alice → Bob) = 0.72",
             font=CODE_FONT, size=16, color=WHITE, bold=True,
             align=PP_ALIGN.CENTER, anchor=MSO_ANCHOR.MIDDLE)
    text_box(s, Inches(0.85), Inches(5.6), Inches(5.6), Inches(1.1),
             "Alice has a solid recommendation via Carol (whom she trusts strongly).",
             font=B_FONT, size=13, color=MIDNIGHT, italic=True, line_spacing=1.25)

    # Right panel: David's view
    rounded(s, Inches(6.9), Inches(1.85), Inches(5.8), Inches(4.9), ICE, corner=0.05)
    rect(s, Inches(6.9), Inches(1.85), Inches(5.8), Inches(0.55), AMBER)
    text_box(s, Inches(6.9), Inches(1.85), Inches(5.8), Inches(0.55),
             "DAVID'S VIEW of Bob",
             font=H_FONT, size=16, color=MIDNIGHT, bold=True,
             align=PP_ALIGN.CENTER, anchor=MSO_ANCHOR.MIDDLE)
    oval(s, Inches(7.3), Inches(3.1), Inches(1.0), Inches(0.75), TEAL)
    text_box(s, Inches(7.3), Inches(3.1), Inches(1.0), Inches(0.75), "David",
             font=B_FONT, size=11, color=WHITE, bold=True,
             align=PP_ALIGN.CENTER, anchor=MSO_ANCHOR.MIDDLE)
    oval(s, Inches(9.2), Inches(3.1), Inches(1.0), Inches(0.75), NAVY)
    text_box(s, Inches(9.2), Inches(3.1), Inches(1.0), Inches(0.75), "Eve",
             font=B_FONT, size=11, color=WHITE, bold=True,
             align=PP_ALIGN.CENTER, anchor=MSO_ANCHOR.MIDDLE)
    oval(s, Inches(11.1), Inches(3.1), Inches(1.0), Inches(0.75), AMBER)
    text_box(s, Inches(11.1), Inches(3.1), Inches(1.0), Inches(0.75), "Bob",
             font=B_FONT, size=11, color=MIDNIGHT, bold=True,
             align=PP_ALIGN.CENTER, anchor=MSO_ANCHOR.MIDDLE)
    arrow(s, Inches(8.3), Inches(3.47), Inches(9.2), Inches(3.47), color=SLATE, width=1.5)
    arrow(s, Inches(10.2), Inches(3.47), Inches(11.1), Inches(3.47), color=SLATE, width=1.5)
    text_box(s, Inches(8.3), Inches(3.85), Inches(0.9), Inches(0.3), "0.6",
             font=H_FONT, size=12, color=AMBER, bold=True, align=PP_ALIGN.CENTER)
    text_box(s, Inches(10.2), Inches(3.85), Inches(0.9), Inches(0.3), "0.5",
             font=H_FONT, size=12, color=AMBER, bold=True, align=PP_ALIGN.CENTER)
    rounded(s, Inches(7.3), Inches(4.5), Inches(4.8), Inches(0.9), AMBER, corner=0.15)
    text_box(s, Inches(7.3), Inches(4.5), Inches(4.8), Inches(0.9),
             "trust(David → Bob) = 0.30",
             font=CODE_FONT, size=16, color=MIDNIGHT, bold=True,
             align=PP_ALIGN.CENTER, anchor=MSO_ANCHOR.MIDDLE)
    text_box(s, Inches(7.15), Inches(5.6), Inches(5.5), Inches(1.1),
             "David's only path reaches Bob through Eve, whom he trusts less, and Eve barely knows Bob.",
             font=B_FONT, size=13, color=MIDNIGHT, italic=True, line_spacing=1.25)
    notes(s, """TIMING: 2 minutes.

WHAT TO SAY:
This is THE property of relational trust. Same target — Bob — evaluated from two different observers produces two very different, both correct answers. Alice trusts Carol strongly (0.9), Carol has worked directly with Bob (0.8), so Alice's transitive trust in Bob is 0.72 — worth pursuing. David's only path runs through Eve, whom he knows less well (0.6), and Eve barely knows Bob (0.5), so David's trust in Bob is only 0.30 — not enough to extend credit or sign a contract. Both answers are correct. Neither is 'Bob's score.' There is no Bob's score. And this matches real life — Alice and David shouldn't treat Bob the same way, because they have different information through different trusted sources.

KEY POINTS:
• Same target, two observers, different answers
• Both are correct from their vantage points
• There is no universal score
• This matches how real trust actually works

TRANSITION:
This is the fundamental shift. Let's make it explicit.""")

    # ---------- 17. The fundamental shift ----------
    s = slide_content(prs, "The fundamental shift", kicker="OLD → NEW", page=17, total=total)
    # Two-column comparison table with big headers
    cols = [Inches(6.05), Inches(6.05)]
    x0 = Inches(0.6)
    y0 = Inches(1.85)
    # Header
    rect(s, x0, y0, cols[0], Inches(0.65), RED)
    text_box(s, x0, y0, cols[0], Inches(0.65),
             "OLD  —  Centralized, universal",
             font=H_FONT, size=18, color=WHITE, bold=True,
             align=PP_ALIGN.CENTER, anchor=MSO_ANCHOR.MIDDLE)
    rect(s, x0 + cols[0] + Inches(0.1), y0, cols[1], Inches(0.65), GREEN)
    text_box(s, x0 + cols[0] + Inches(0.1), y0, cols[1], Inches(0.65),
             "NEW  —  Relational, per-observer",
             font=H_FONT, size=18, color=WHITE, bold=True,
             align=PP_ALIGN.CENTER, anchor=MSO_ANCHOR.MIDDLE)
    # Rows
    rows = [
        ("One authority decides", "Every observer decides"),
        ("One universal score", "Many per-domain trust values"),
        ("Opaque formula", "Your own algorithm, with audit"),
        ("Identity = platform-owned", "Identity = user-owned (quid)"),
        ("Keys lost = over", "Guardian M-of-N recovery"),
        ("Errors fix slowly via authority", "Corrections are on-chain events"),
        ("Single DB = single breach", "Distributed + encrypted + signed"),
        ("Multi-party auth ad-hoc", "M-of-N as a protocol primitive"),
    ]
    for i, (old, new) in enumerate(rows):
        y = y0 + Inches(0.7) + Inches(0.55 * i)
        fill = ICE if i % 2 == 0 else WHITE
        rect(s, x0, y, cols[0], Inches(0.5), fill, line=CLOUD, line_w=0.5)
        text_box(s, x0 + Inches(0.15), y, cols[0] - Inches(0.2), Inches(0.5),
                 old, font=B_FONT, size=13, color=MIDNIGHT,
                 anchor=MSO_ANCHOR.MIDDLE)
        rect(s, x0 + cols[0] + Inches(0.1), y, cols[1], Inches(0.5),
             fill, line=CLOUD, line_w=0.5)
        text_box(s, x0 + cols[0] + Inches(0.25), y, cols[1] - Inches(0.2), Inches(0.5),
                 new, font=B_FONT, size=13, color=MIDNIGHT, bold=True,
                 anchor=MSO_ANCHOR.MIDDLE)
    notes(s, """TIMING: 1.5 minutes.

WHAT TO SAY:
This slide is the whole shift in eight rows. Old world on the left: one authority, one universal score, opaque formula, platform-owned identity, lost keys are terminal, slow error correction, single-point-of-failure breaches, ad-hoc multi-party auth. New world on the right, which is what Quidnug provides: every observer decides, many per-domain trust values, your own transparent algorithm, user-owned identity via the quid primitive, guardian-based key recovery, corrections as on-chain events that propagate in seconds, distributed signed state with encrypted sensitive payloads, and M-of-N as a first-class protocol primitive. Each row is a specific design decision we'll see implemented in the coming slides.

KEY POINTS:
• Eight-row shift
• Each row has a specific implementation coming
• These aren't aspirations — they're in the protocol

TRANSITION:
And here's how the protocol gives you each of those.""")

    # ---------- 18. What Quidnug provides ----------
    s = slide_content(prs, "What Quidnug provides", kicker="PRIMITIVES", page=18, total=total)
    # 2x3 grid of primitive cards
    primitives = [
        ("Quids", "Cryptographic identities, user-generated (BYOQ)", TEAL),
        ("Trust edges", "Signed, domain-scoped, time-bounded", NAVY),
        ("Domains", "Hierarchical trust contexts", AMBER),
        ("Proof-of-Trust consensus", "Per-observer block acceptance tiers", TEAL),
        ("Nonce ledger", "Strong replay protection (QDP-0001)", NAVY),
        ("Guardian M-of-N", "Time-locked recovery (QDP-0002)", AMBER),
        ("Cross-domain gossip", "Rotations propagate (QDP-0003)", TEAL),
        ("Push gossip", "Real-time signal delivery (QDP-0005)", NAVY),
        ("K-of-K bootstrap", "Trust-on-first-use solved (QDP-0008)", AMBER),
    ]
    col_x = [Inches(0.6), Inches(4.82), Inches(9.05)]
    row_y = [Inches(1.85), Inches(3.65), Inches(5.45)]
    for i, (name, desc, accent) in enumerate(primitives):
        x = col_x[i % 3]
        y = row_y[i // 3]
        card(s, x, y, Inches(4.05), Inches(1.65), name, body=desc,
             accent=accent, fill=ICE, body_size=12)
    notes(s, """TIMING: 1 minute.

WHAT TO SAY:
Quickly: here are the nine primitives Quidnug provides. Quids — user-generated cryptographic identities. Trust edges — signed, scoped, time-bounded statements. Domains — hierarchical namespaces for trust contexts. Proof-of-Trust consensus — each node independently tiers incoming blocks by how much they trust the signer. Nonce ledger — strong replay protection per signer per domain. Guardian M-of-N — time-locked multi-party recovery. Cross-domain gossip — rotations and revocations flow across domain boundaries. Push gossip — fresh events reach interested nodes in seconds. K-of-K bootstrap — fresh nodes can safely join without trusting a single peer. That's the toolkit. Each of these is a QDP, a numbered Quidnug Design Proposal, and we'll look at them in depth in the next section.

KEY POINTS:
• Nine primitives
• Each is a QDP (numbered design proposal)
• All nine have landed in the codebase

TRANSITION:
Let's go deeper into the core concepts.""")

    return 18
