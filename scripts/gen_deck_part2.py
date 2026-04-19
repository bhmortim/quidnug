"""Slides 19–41: Core concepts + Technical primitives."""
from pptx.util import Inches, Pt
from pptx.enum.text import PP_ALIGN, MSO_ANCHOR
from gen_deck_core import (
    slide_section, slide_content, slide_dark,
    set_bg, text_box, bullets, rect, rounded, hexagon, oval, shape_text,
    arrow, notes, card, chip, numbered_circle, table_rows,
    MIDNIGHT, NAVY, TEAL, AMBER, ICE, CLOUD, SLATE, WHITE, GREEN, RED,
    SOFT_TEAL, SOFT_AMBER, H_FONT, B_FONT, CODE_FONT
)


def build_part2(prs, start_page, total):
    p = start_page  # running page number

    # ---------- SECTION 3 divider ----------
    slide_section(prs, 3, "PART THREE", "Core concepts")
    p += 1

    # ---------- 20. Core concepts overview ----------
    s = slide_content(prs, "Five concepts — that's the whole model", kicker="YOU ALREADY KNOW THE REST", page=p, total=total)
    concepts = [
        ("1", "Quids", "User-generated cryptographic identities", TEAL),
        ("2", "Trust edges", "Signed, scoped, time-bounded claims", NAVY),
        ("3", "Domains", "Hierarchical context (like DNS)", AMBER),
        ("4", "Proof-of-Trust", "Per-observer block acceptance", TEAL),
        ("5", "Transactions", "Typed, signed, appended to blocks", NAVY),
    ]
    for i, (num, name, desc, col) in enumerate(concepts):
        y = Inches(1.85 + i * 0.95)
        rounded(s, Inches(0.6), y, Inches(12.1), Inches(0.8), ICE, corner=0.08)
        rect(s, Inches(0.6), y, Inches(0.09), Inches(0.8), col)
        oval(s, Inches(0.85), y + Inches(0.13), Inches(0.55), Inches(0.55), col)
        text_box(s, Inches(0.85), y + Inches(0.13), Inches(0.55), Inches(0.55),
                 num, font=H_FONT, size=18, color=WHITE, bold=True,
                 align=PP_ALIGN.CENTER, anchor=MSO_ANCHOR.MIDDLE)
        text_box(s, Inches(1.6), y + Inches(0.08), Inches(3), Inches(0.65),
                 name, font=H_FONT, size=20, color=MIDNIGHT, bold=True,
                 anchor=MSO_ANCHOR.MIDDLE)
        text_box(s, Inches(4.8), y + Inches(0.08), Inches(7.7), Inches(0.65),
                 desc, font=B_FONT, size=14, color=SLATE, italic=True,
                 anchor=MSO_ANCHOR.MIDDLE)
    notes(s, """TIMING: 1 minute.

WHAT TO SAY:
Five concepts. That's it. Once you understand these, everything else is composition. Quids are the cryptographic identities — people, organizations, AI agents, devices, documents. Trust edges are signed claims from one quid to another, scoped to a specific domain. Domains give context — trust in one domain doesn't automatically imply trust in another. Proof-of-Trust is Quidnug's unusual consensus model where each node tiers blocks by its own trust in the signer. And transactions are the typed, signed operations that get bundled into blocks and appended to the chain. I'm going to spend a couple of minutes on each, because if you leave with these five concepts clear, the rest of the talk builds naturally.

KEY POINTS:
• Exactly five concepts
• Everything else is composition
• These map 1:1 to Go types in the codebase

TRANSITION:
Let's start with quids.""")
    p += 1

    # ---------- 21. Quids ----------
    s = slide_content(prs, "Quids: cryptographic identities", kicker="CONCEPT 1", page=p, total=total)
    # Left: anatomy of a quid
    rounded(s, Inches(0.6), Inches(1.85), Inches(6), Inches(5), ICE, corner=0.05)
    text_box(s, Inches(0.85), Inches(2.0), Inches(5.5), Inches(0.4),
             "ANATOMY OF A QUID", font=B_FONT, size=11, color=SLATE, bold=True)
    text_box(s, Inches(0.85), Inches(2.4), Inches(5.5), Inches(0.55),
             "quid-a1b2c3d4e5f67890",
             font=CODE_FONT, size=18, color=NAVY, bold=True)
    # Box for the quid structure
    rounded(s, Inches(0.85), Inches(3.1), Inches(5.5), Inches(3.55), WHITE,
            line=CLOUD, corner=0.05)
    rows_data = [
        ("QuidID", "sha256(publicKey)[:16]", TEAL),
        ("PublicKey", "ECDSA P-256 (compressed)", NAVY),
        ("Creator", "Usually self (BYOQ)", AMBER),
        ("Attributes", "Free-form metadata", TEAL),
        ("Created", "Unix timestamp", NAVY),
    ]
    for i, (k, v, col) in enumerate(rows_data):
        yy = Inches(3.3 + i * 0.62)
        rect(s, Inches(1.1), yy, Inches(0.15), Inches(0.45), col)
        text_box(s, Inches(1.4), yy, Inches(1.6), Inches(0.45), k,
                 font=B_FONT, size=12, color=MIDNIGHT, bold=True,
                 anchor=MSO_ANCHOR.MIDDLE)
        text_box(s, Inches(3.1), yy, Inches(3.1), Inches(0.45), v,
                 font=CODE_FONT, size=11, color=SLATE,
                 anchor=MSO_ANCHOR.MIDDLE)

    # Right: what a quid can represent
    text_box(s, Inches(7), Inches(1.85), Inches(5.7), Inches(0.4),
             "WHAT A QUID CAN REPRESENT", font=B_FONT, size=11,
             color=SLATE, bold=True)
    reps = [
        ("Person", "Alice, voter, patient"),
        ("Organization", "Bank, university, utility co."),
        ("AI agent", "Autonomous agent, inference endpoint"),
        ("Device", "Camera, IoT sensor, HSM"),
        ("Asset", "Vehicle, property, model weights"),
        ("Contract", "Agreement, policy, configuration"),
    ]
    for i, (ttl, ex) in enumerate(reps):
        y = Inches(2.3 + i * 0.75)
        rounded(s, Inches(7), y, Inches(5.7), Inches(0.65), SOFT_TEAL, corner=0.08)
        text_box(s, Inches(7.25), y, Inches(2.3), Inches(0.65), ttl,
                 font=H_FONT, size=14, color=MIDNIGHT, bold=True,
                 anchor=MSO_ANCHOR.MIDDLE)
        text_box(s, Inches(9.1), y, Inches(3.4), Inches(0.65), ex,
                 font=B_FONT, size=12, color=SLATE, italic=True,
                 anchor=MSO_ANCHOR.MIDDLE)
    notes(s, """TIMING: 1.5 minutes.

WHAT TO SAY:
A quid is the fundamental identity primitive. It's a public-key plus minimal metadata. The quid ID is derived from the public key — sha256 of the pubkey, first sixteen hex chars. That gives you a short, unforgeable identifier. Creator is usually the same quid — self-created, bring-your-own-quid. No central issuer required. Attributes are free-form — you can put anything in there, or nothing. Created is when the quid was instantiated. Here's the critical point: a quid can represent anything. People, organizations, AI agents, devices, assets, contracts. In the use cases we'll see, we use quids for voters, doctors, AI models, shipping carriers, invoices, wallets, election contests, software releases. The primitive is universal.

KEY POINTS:
• Structure: pubkey + creator + attributes + timestamp
• Quid ID: sha256(pubkey)[:16]
• User-generated (no central issuer)
• Universal: people, orgs, AI agents, devices, assets

TRANSITION:
Quids don't do anything by themselves. They need trust relationships.""")
    p += 1

    # ---------- 22. Trust edges ----------
    s = slide_content(prs, "Trust edges: signed claims between quids", kicker="CONCEPT 2", page=p, total=total)
    # Visual at top: edge between two quids
    oval(s, Inches(0.8), Inches(1.85), Inches(1.8), Inches(1.35), TEAL)
    text_box(s, Inches(0.8), Inches(1.85), Inches(1.8), Inches(1.35),
             "Alice", font=H_FONT, size=18, color=WHITE, bold=True,
             align=PP_ALIGN.CENTER, anchor=MSO_ANCHOR.MIDDLE)
    arrow(s, Inches(2.65), Inches(2.5), Inches(4.85), Inches(2.5), color=NAVY, width=3)
    oval(s, Inches(4.9), Inches(1.85), Inches(1.8), Inches(1.35), AMBER)
    text_box(s, Inches(4.9), Inches(1.85), Inches(1.8), Inches(1.35),
             "Bob", font=H_FONT, size=18, color=MIDNIGHT, bold=True,
             align=PP_ALIGN.CENTER, anchor=MSO_ANCHOR.MIDDLE)
    # Trust value label over arrow
    rounded(s, Inches(3.15), Inches(1.6), Inches(1.2), Inches(0.5), MIDNIGHT, corner=0.3)
    text_box(s, Inches(3.15), Inches(1.6), Inches(1.2), Inches(0.5),
             "0.9", font=CODE_FONT, size=14, color=TEAL, bold=True,
             align=PP_ALIGN.CENTER, anchor=MSO_ANCHOR.MIDDLE)
    # Right side: schema
    rounded(s, Inches(7.2), Inches(1.85), Inches(5.5), Inches(3.3), MIDNIGHT, corner=0.04)
    text_box(s, Inches(7.4), Inches(2.0), Inches(5), Inches(0.4),
             "TRUST EDGE SCHEMA", font=B_FONT, size=11, color=TEAL, bold=True)
    schema_lines = [
        ("truster:", "alice-quid", TEAL),
        ("trustee:", "bob-quid", AMBER),
        ("trustLevel:", "0.9", GREEN),
        ("domain:", "contractors.home.services", WHITE),
        ("validUntil:", "<now + 2 years>", WHITE),
        ("nonce:", "47", WHITE),
        ("signature:", "<alice's ECDSA sig>", WHITE),
    ]
    for i, (k, v, col) in enumerate(schema_lines):
        y = Inches(2.45 + i * 0.38)
        text_box(s, Inches(7.4), y, Inches(1.4), Inches(0.35),
                 k, font=CODE_FONT, size=11, color=CLOUD,
                 anchor=MSO_ANCHOR.MIDDLE)
        text_box(s, Inches(8.7), y, Inches(3.9), Inches(0.35),
                 v, font=CODE_FONT, size=11, color=col, bold=True,
                 anchor=MSO_ANCHOR.MIDDLE)

    # Bottom: properties
    rounded(s, Inches(0.6), Inches(5.35), Inches(12.1), Inches(1.7), ICE, corner=0.05)
    text_box(s, Inches(0.85), Inches(5.5), Inches(11.6), Inches(0.4),
             "FIVE PROPERTIES OF EVERY TRUST EDGE",
             font=B_FONT, size=11, color=SLATE, bold=True)
    props = [
        ("Signed", "By truster's key"),
        ("Scoped", "Per-domain"),
        ("Quantified", "0.0–1.0"),
        ("Expirable", "validUntil field"),
        ("Replay-safe", "Monotonic nonce"),
    ]
    for i, (h, b) in enumerate(props):
        x = Inches(0.85 + i * 2.41)
        y = Inches(5.9)
        oval(s, x, y, Inches(0.4), Inches(0.4), TEAL)
        text_box(s, x + Inches(0.5), y - Inches(0.05), Inches(1.8), Inches(0.45),
                 h, font=H_FONT, size=13, color=MIDNIGHT, bold=True,
                 anchor=MSO_ANCHOR.MIDDLE)
        text_box(s, x, y + Inches(0.55), Inches(2.3), Inches(0.45),
                 b, font=B_FONT, size=11, color=SLATE,
                 italic=True, line_spacing=1.2)
    notes(s, """TIMING: 1.5 minutes.

WHAT TO SAY:
A trust edge is a cryptographically-signed statement from one quid to another. Looking at the schema on the right: truster is Alice, trustee is Bob, trust level is 0.9, domain is 'contractors.home.services', it's valid until some future time, has a monotonic nonce for replay protection, and an ECDSA signature from Alice. Every trust edge has these five properties — it's signed, scoped, quantified, expirable, and replay-safe. Multiple edges from Alice to Bob in different domains are fine, each scoped independently. Edges can be revoked by issuing a new edge with the same truster-trustee pair but a higher nonce and lower trustLevel — or zero. Cryptographic forgery is infeasible without Alice's private key.

KEY POINTS:
• ECDSA-signed claim: truster → trustee
• Five properties: signed, scoped, quantified, expirable, replay-safe
• Revocation = new edge with higher nonce and lower trust
• Forgery requires private key

TRANSITION:
One edge is simple. The interesting part is how they compose.""")
    p += 1

    # ---------- 23. Transitive trust computation ----------
    s = slide_content(prs, "Transitive trust — how paths compose", kicker="CONCEPT 2 cont'd", page=p, total=total)
    # Big diagram: multi-hop trust
    rounded(s, Inches(0.6), Inches(1.8), Inches(12.1), Inches(3.5), ICE, corner=0.05)
    text_box(s, Inches(0.85), Inches(1.95), Inches(11.6), Inches(0.4),
             "FOUR-NODE PATH: A → B → C → D",
             font=B_FONT, size=11, color=SLATE, bold=True)
    # Nodes
    xs = [Inches(1.2), Inches(4.4), Inches(7.6), Inches(10.8)]
    labels = ["A", "B", "C", "D"]
    colors = [NAVY, TEAL, TEAL, AMBER]
    for i, (xx, lab, col) in enumerate(zip(xs, labels, colors)):
        oval(s, xx, Inches(3.0), Inches(1.5), Inches(1.2), col)
        text_box(s, xx, Inches(3.0), Inches(1.5), Inches(1.2), lab,
                 font=H_FONT, size=28, color=WHITE if col != AMBER else MIDNIGHT,
                 bold=True, align=PP_ALIGN.CENTER, anchor=MSO_ANCHOR.MIDDLE)
    # Edges
    edges = [(0, 1, "0.9"), (1, 2, "0.8"), (2, 3, "0.7")]
    for a, b, v in edges:
        arrow(s, xs[a] + Inches(1.5), Inches(3.6),
              xs[b], Inches(3.6), color=SLATE, width=2)
        # Label the edge
        cx = (xs[a] + xs[b]) / 2 + Inches(0.2)
        rounded(s, cx, Inches(4.25), Inches(0.8), Inches(0.45), MIDNIGHT, corner=0.25)
        text_box(s, cx, Inches(4.25), Inches(0.8), Inches(0.45), v,
                 font=CODE_FONT, size=13, color=TEAL, bold=True,
                 align=PP_ALIGN.CENTER, anchor=MSO_ANCHOR.MIDDLE)
    # Compute-under the chain
    text_box(s, Inches(0.85), Inches(4.85), Inches(11.6), Inches(0.4),
             "trust(A → D) =  0.9  ×  0.8  ×  0.7  =  0.504",
             font=CODE_FONT, size=22, color=MIDNIGHT, bold=True,
             align=PP_ALIGN.CENTER)

    # Lower block: why multiply?
    rounded(s, Inches(0.6), Inches(5.4), Inches(6), Inches(1.55), SOFT_TEAL, corner=0.05)
    text_box(s, Inches(0.85), Inches(5.55), Inches(5.5), Inches(0.4),
             "WHY MULTIPLY?", font=B_FONT, size=11, color=MIDNIGHT, bold=True)
    text_box(s, Inches(0.85), Inches(5.9), Inches(5.5), Inches(1),
             "Each hop compounds uncertainty. A 90% confidence in B, times B's 80% confidence in C, can't produce more than 72%. It's a monotonically decaying AND.",
             font=B_FONT, size=12, color=MIDNIGHT, line_spacing=1.3)
    rounded(s, Inches(6.8), Inches(5.4), Inches(5.9), Inches(1.55), SOFT_AMBER, corner=0.05)
    text_box(s, Inches(7.0), Inches(5.55), Inches(5.5), Inches(0.4),
             "DEPTH LIMIT", font=B_FONT, size=11, color=MIDNIGHT, bold=True)
    text_box(s, Inches(7.0), Inches(5.9), Inches(5.5), Inches(1),
             "Default max depth: 5 hops. After 5-hop decay at realistic trust values, remaining trust is negligible (<0.5^5 = 0.03125).",
             font=B_FONT, size=12, color=MIDNIGHT, line_spacing=1.3)
    notes(s, """TIMING: 1.5 minutes.

WHAT TO SAY:
Transitive trust is what makes the model powerful. If A trusts B at 0.9, B trusts C at 0.8, and C trusts D at 0.7, then A's transitive trust in D is the product: 0.504. Why multiply? Each hop introduces uncertainty, and uncertainty compounds. You can't trust someone more through intermediaries than you trust the intermediaries. Multiplication captures this correctly as a monotonically-decaying AND — A must trust B, AND B must trust C, AND C must trust D. We cap the search at five hops by default because beyond five hops at realistic trust values the remaining contribution is tiny — 0.5 to the fifth is about 3 percent. So the BFS is bounded and fast. In practice we see most real trust queries resolve within one or two hops.

KEY POINTS:
• Multiplication — each hop is AND, not OR
• Default max depth: 5 hops
• Beyond 5, decay kills the signal
• Fast BFS, milliseconds

TRANSITION:
But trust graphs aren't paths — they're networks. What about multiple paths?""")
    p += 1

    # ---------- 24. Multiple paths, max trust ----------
    s = slide_content(prs, "Multiple paths — take the MAX", kicker="CONCEPT 2 cont'd", page=p, total=total)
    # Three paths from A to D
    rounded(s, Inches(0.6), Inches(1.85), Inches(12.1), Inches(5), ICE, corner=0.05)

    # A on left, D on right
    oval(s, Inches(1.0), Inches(3.9), Inches(1.2), Inches(1.0), NAVY)
    text_box(s, Inches(1.0), Inches(3.9), Inches(1.2), Inches(1.0), "A",
             font=H_FONT, size=26, color=WHITE, bold=True,
             align=PP_ALIGN.CENTER, anchor=MSO_ANCHOR.MIDDLE)
    oval(s, Inches(11.2), Inches(3.9), Inches(1.2), Inches(1.0), AMBER)
    text_box(s, Inches(11.2), Inches(3.9), Inches(1.2), Inches(1.0), "D",
             font=H_FONT, size=26, color=MIDNIGHT, bold=True,
             align=PP_ALIGN.CENTER, anchor=MSO_ANCHOR.MIDDLE)

    # Three intermediate paths
    # Path 1 (top): through B
    oval(s, Inches(6.0), Inches(2.2), Inches(1.0), Inches(0.85), TEAL)
    text_box(s, Inches(6.0), Inches(2.2), Inches(1.0), Inches(0.85), "B",
             font=H_FONT, size=20, color=WHITE, bold=True,
             align=PP_ALIGN.CENTER, anchor=MSO_ANCHOR.MIDDLE)
    arrow(s, Inches(2.2), Inches(4.3), Inches(6.0), Inches(2.65), color=GREEN, width=2.5)
    arrow(s, Inches(7.0), Inches(2.65), Inches(11.2), Inches(4.3), color=GREEN, width=2.5)
    text_box(s, Inches(3.1), Inches(2.75), Inches(2), Inches(0.3), "0.9",
             font=H_FONT, size=12, color=GREEN, bold=True)
    text_box(s, Inches(8.2), Inches(2.75), Inches(2), Inches(0.3), "0.8",
             font=H_FONT, size=12, color=GREEN, bold=True)
    # Path 1 result
    rounded(s, Inches(4.5), Inches(1.3), Inches(4.2), Inches(0.6), GREEN, corner=0.2)
    text_box(s, Inches(4.5), Inches(1.3), Inches(4.2), Inches(0.6),
             "Path 1: 0.9 × 0.8 = 0.72", font=CODE_FONT, size=14,
             color=WHITE, bold=True, align=PP_ALIGN.CENTER,
             anchor=MSO_ANCHOR.MIDDLE)

    # Path 2 (middle): through C
    oval(s, Inches(6.0), Inches(3.95), Inches(1.0), Inches(0.85), TEAL)
    text_box(s, Inches(6.0), Inches(3.95), Inches(1.0), Inches(0.85), "C",
             font=H_FONT, size=20, color=WHITE, bold=True,
             align=PP_ALIGN.CENTER, anchor=MSO_ANCHOR.MIDDLE)
    arrow(s, Inches(2.2), Inches(4.4), Inches(6.0), Inches(4.4), color=AMBER, width=2)
    arrow(s, Inches(7.0), Inches(4.4), Inches(11.2), Inches(4.4), color=AMBER, width=2)
    text_box(s, Inches(3.5), Inches(4.55), Inches(2), Inches(0.3), "0.6",
             font=H_FONT, size=12, color=AMBER, bold=True)
    text_box(s, Inches(8.2), Inches(4.55), Inches(2), Inches(0.3), "0.7",
             font=H_FONT, size=12, color=AMBER, bold=True)

    # Path 3 (bottom): through E
    oval(s, Inches(6.0), Inches(5.65), Inches(1.0), Inches(0.85), TEAL)
    text_box(s, Inches(6.0), Inches(5.65), Inches(1.0), Inches(0.85), "E",
             font=H_FONT, size=20, color=WHITE, bold=True,
             align=PP_ALIGN.CENTER, anchor=MSO_ANCHOR.MIDDLE)
    arrow(s, Inches(2.2), Inches(4.6), Inches(6.0), Inches(6.05), color=RED, width=1.5)
    arrow(s, Inches(7.0), Inches(6.05), Inches(11.2), Inches(4.6), color=RED, width=1.5)
    # Labels offset AWAY from the angled arrow line (inside the triangle
    # formed by arrow + baseline, never directly on the arrow)
    text_box(s, Inches(3.4), Inches(5.8), Inches(0.8), Inches(0.3), "0.4",
             font=H_FONT, size=12, color=RED, bold=True, align=PP_ALIGN.CENTER)
    text_box(s, Inches(9.0), Inches(5.8), Inches(0.8), Inches(0.3), "0.3",
             font=H_FONT, size=12, color=RED, bold=True, align=PP_ALIGN.CENTER)
    rounded(s, Inches(4.5), Inches(6.6), Inches(4.2), Inches(0.35), RED, corner=0.2)
    text_box(s, Inches(4.5), Inches(6.6), Inches(4.2), Inches(0.35),
             "Path 3: 0.4 × 0.3 = 0.12 (rejected)", font=CODE_FONT,
             size=11, color=WHITE, bold=True, align=PP_ALIGN.CENTER,
             anchor=MSO_ANCHOR.MIDDLE)

    # Middle-path label
    rounded(s, Inches(0.85), Inches(6.6), Inches(3.4), Inches(0.35), AMBER, corner=0.2)
    text_box(s, Inches(0.85), Inches(6.6), Inches(3.4), Inches(0.35),
             "Path 2: 0.6 × 0.7 = 0.42", font=CODE_FONT, size=11,
             color=MIDNIGHT, bold=True, align=PP_ALIGN.CENTER,
             anchor=MSO_ANCHOR.MIDDLE)
    # Final verdict
    rounded(s, Inches(8.9), Inches(6.6), Inches(3.8), Inches(0.35), MIDNIGHT, corner=0.2)
    text_box(s, Inches(8.9), Inches(6.6), Inches(3.8), Inches(0.35),
             "Result: MAX = 0.72  ✓", font=CODE_FONT, size=12,
             color=TEAL, bold=True, align=PP_ALIGN.CENTER,
             anchor=MSO_ANCHOR.MIDDLE)
    notes(s, """TIMING: 1.5 minutes.

WHAT TO SAY:
Real trust networks aren't just paths — they're graphs with many paths between the same pair of nodes. A might reach D through B, or C, or E, or half a dozen routes. Quidnug evaluates ALL paths up to the depth limit and returns the MAXIMUM. Why max, not average? Because one strong recommendation from someone you trust deeply beats many weak recommendations from people you barely know. Averaging would dilute the best signal. Summing would misrepresent — trust isn't additive, it's bounded at 1.0. Max is the right aggregator. Here we have three paths: through B gives 0.72, through C gives 0.42, through E gives 0.12. The answer is 0.72, via B. The algorithm also returns the path itself so you can audit — 'why did you trust D at 0.72? Because you trust B at 0.9 and B trusts D at 0.8.' Fully transparent.

KEY POINTS:
• MAX over all paths, NOT average or sum
• Best recommendation wins
• Path is returned for audit
• This is the BFS's core output

TRANSITION:
Trust needs context. You don't trust your plumber for tax advice. Enter domains.""")
    p += 1

    # ---------- 25. Domains ----------
    s = slide_content(prs, "Domains: hierarchical context", kicker="CONCEPT 3", page=p, total=total)
    # Left: tree of domains
    rounded(s, Inches(0.6), Inches(1.85), Inches(7), Inches(5), ICE, corner=0.05)
    text_box(s, Inches(0.85), Inches(2.0), Inches(6.5), Inches(0.4),
             "DOMAIN TREE (DNS-STYLE)",
             font=B_FONT, size=11, color=SLATE, bold=True)
    # Tree levels
    tree_items = [
        ("credit", 0, TEAL, True),
        ("credit.mortgage.us", 1, TEAL, False),
        ("credit.auto-loan.us", 1, TEAL, False),
        ("credit.alternative-data.utilities", 1, TEAL, False),
        ("elections", 0, AMBER, True),
        ("elections.williamson-county-tx.2026-nov", 1, AMBER, False),
        ("elections.williamson-county-tx.2026-nov.contests.us-senate", 2, AMBER, False),
        ("ai.provenance", 0, NAVY, True),
        ("ai.provenance.models", 1, NAVY, False),
        ("ai.provenance.models.foundation", 2, NAVY, False),
    ]
    for i, (name, level, col, top) in enumerate(tree_items):
        y = Inches(2.45 + i * 0.4)
        indent = Inches(0.85 + level * 0.35)
        if top:
            oval(s, indent, y + Inches(0.08), Inches(0.22), Inches(0.22), col)
        text_box(s, indent + Inches(0.35), y, Inches(6), Inches(0.35),
                 name, font=CODE_FONT, size=11,
                 color=MIDNIGHT if top else SLATE,
                 bold=top, anchor=MSO_ANCHOR.MIDDLE)

    # Right: why domains matter
    rounded(s, Inches(7.8), Inches(1.85), Inches(4.9), Inches(5), MIDNIGHT, corner=0.05)
    text_box(s, Inches(8.0), Inches(2.0), Inches(4.5), Inches(0.5),
             "Why domains matter", font=H_FONT, size=20, color=TEAL, bold=True)
    bullets(s, Inches(8.0), Inches(2.65), Inches(4.5), Inches(4),
            [
                ("Trust in X ≠ trust in Y", 0, True),
                "Dr. Smith for medicine isn't the same",
                "as Dr. Smith for legal advice",
                ("Inheritance (optional)", 0, True),
                "Parent-domain validators can",
                "endorse subdomains",
                ("Namespace collisions impossible", 0, True),
                "DNS-like naming prevents accidental",
                "cross-domain trust",
                ("Composition", 0, True),
                "Consumer weighs domains by",
                "their own policy",
            ], size=12, color=CLOUD)
    notes(s, """TIMING: 1.5 minutes.

WHAT TO SAY:
Domains give trust context. Think of it as DNS for trust — hierarchical dot-separated namespaces. On the left, a small sample of domains from our use cases: credit splits into mortgage, auto-loan, alternative-data utilities; elections scopes by jurisdiction and cycle and contest; ai-provenance by category. Domains matter for four reasons. First, trust isn't universal — you trust Dr. Smith for medicine, not for legal advice. Different domains. Second, optional inheritance — a parent domain's validators can endorse subdomains if that matches your structure. Third, namespace collisions are impossible; two different orgs can both use 'credit' as a top-level domain. Fourth, composition — a consumer picks which domains to weight and how heavily in their evaluation. The lender doesn't have to use your domain structure; they pick their own.

KEY POINTS:
• DNS-style hierarchical naming
• Trust is domain-scoped
• Different domains = different meanings
• Consumer picks which to honor

TRANSITION:
Now the unusual one. Proof-of-Trust consensus.""")
    p += 1

    # ---------- 26. Proof-of-Trust ----------
    s = slide_content(prs, "Proof-of-Trust consensus", kicker="CONCEPT 4", page=p, total=total)
    # Left: flow diagram
    rounded(s, Inches(0.6), Inches(1.85), Inches(7.6), Inches(5), ICE, corner=0.05)
    text_box(s, Inches(0.85), Inches(2.0), Inches(7), Inches(0.4),
             "FLOW — WHEN A BLOCK ARRIVES",
             font=B_FONT, size=11, color=SLATE, bold=True)
    # Steps
    y = Inches(2.5)
    steps = [
        ("Block arrives", "Signed by validator V", NAVY),
        ("Crypto check", "Hash + signature verify?", TEAL),
        ("Trust check", "My trust in V — what tier?", AMBER),
        ("Tier assigned", "Trusted / Tentative / Untrusted / Invalid", GREEN),
    ]
    for i, (h, sub, col) in enumerate(steps):
        yy = y + Inches(i * 0.96)
        rounded(s, Inches(0.85), yy, Inches(7.1), Inches(0.75), WHITE,
                line=CLOUD, corner=0.05)
        rect(s, Inches(0.85), yy, Inches(0.08), Inches(0.75), col)
        numbered_circle(s, Inches(1.05), yy + Inches(0.15), 0.45, i + 1,
                        fill=col, size=14)
        text_box(s, Inches(1.7), yy + Inches(0.07), Inches(3.2), Inches(0.4),
                 h, font=H_FONT, size=14, color=MIDNIGHT, bold=True)
        text_box(s, Inches(1.7), yy + Inches(0.42), Inches(6), Inches(0.3),
                 sub, font=B_FONT, size=11, color=SLATE, italic=True)
        if i < len(steps) - 1:
            arrow(s, Inches(4.4), yy + Inches(0.75),
                  Inches(4.4), yy + Inches(0.95), color=SLATE, width=1.5)

    # Right: the key property
    rounded(s, Inches(8.4), Inches(1.85), Inches(4.3), Inches(5), MIDNIGHT, corner=0.05)
    text_box(s, Inches(8.55), Inches(2.0), Inches(4.0), Inches(0.45),
             "KEY PROPERTY", font=B_FONT, size=11, color=TEAL, bold=True)
    text_box(s, Inches(8.55), Inches(2.4), Inches(4.0), Inches(2.5),
             "Each node independently decides whether to trust a block's validator.",
             font=H_FONT, size=18, color=WHITE, bold=True, line_spacing=1.2)
    text_box(s, Inches(8.55), Inches(4.3), Inches(4.0), Inches(2.3),
             "Alice and Bob may accept different blocks. That's not a bug — it's the protocol reflecting reality that trust is relational.\n\nOnly crypto-valid blocks advance. Anything else is rejected uniformly.",
             font=B_FONT, size=12, color=CLOUD, line_spacing=1.3)
    notes(s, """TIMING: 2 minutes.

WHAT TO SAY:
Proof-of-Trust is Quidnug's consensus model, and it's unusual enough to deserve its own slide. The idea: when a node receives a block, it first does the universal cryptographic check — does the hash match, does the signature verify? All honest nodes agree on that. Then it does a subjective check — 'how much do I trust this specific validator?' Based on that trust level, it assigns the block to one of four acceptance tiers, which we'll see on the next slide. The critical property on the right: each node independently decides. Alice trusts one set of validators; Bob trusts another. They'll agree on blocks from shared-trust validators and disagree on others. This is NOT a bug. It's the protocol reflecting the reality that trust is subjective. For consortium and federated deployments — which is where Quidnug shines — this is exactly what you want.

KEY POINTS:
• Crypto check first (objective, universal)
• Trust check second (subjective, per-node)
• Tier assignment determines what happens
• Different nodes may have different chains

TRANSITION:
The four tiers.""")
    p += 1

    # ---------- 27. The four tiers ----------
    s = slide_content(prs, "The four acceptance tiers", kicker="CONCEPT 4 cont'd", page=p, total=total)
    tiers = [
        ("TRUSTED", "Trust ≥ domain threshold",
         "Added to main chain • Transactions applied • Edges become verified",
         GREEN),
        ("TENTATIVE", "Trust between distrust and trust threshold",
         "Stored separately • Not built on • Can promote later",
         AMBER),
        ("UNTRUSTED", "Trust ≤ distrust threshold",
         "Extract only trust-edge data • Don't store block • Don't build",
         AMBER),
        ("INVALID", "Cryptographic verification fails",
         "Reject. Log. Rate-limit the sender.",
         RED),
    ]
    for i, (name, cond, effect, col) in enumerate(tiers):
        y = Inches(1.85 + i * 1.25)
        rounded(s, Inches(0.6), y, Inches(12.1), Inches(1.1), ICE, corner=0.05)
        rect(s, Inches(0.6), y, Inches(0.09), Inches(1.1), col)
        # Tier badge
        rounded(s, Inches(0.85), y + Inches(0.15), Inches(2.3), Inches(0.8),
                col, corner=0.1)
        text_box(s, Inches(0.85), y + Inches(0.15), Inches(2.3), Inches(0.8),
                 name, font=H_FONT, size=18,
                 color=(MIDNIGHT if col == AMBER else WHITE), bold=True,
                 align=PP_ALIGN.CENTER, anchor=MSO_ANCHOR.MIDDLE)
        # Condition
        text_box(s, Inches(3.4), y + Inches(0.15), Inches(4.5), Inches(0.35),
                 "CONDITION", font=B_FONT, size=10, color=SLATE, bold=True)
        text_box(s, Inches(3.4), y + Inches(0.45), Inches(4.5), Inches(0.55),
                 cond, font=B_FONT, size=12.5, color=MIDNIGHT,
                 anchor=MSO_ANCHOR.TOP)
        # Effect
        text_box(s, Inches(8.1), y + Inches(0.15), Inches(4.5), Inches(0.35),
                 "EFFECT", font=B_FONT, size=10, color=SLATE, bold=True)
        text_box(s, Inches(8.1), y + Inches(0.45), Inches(4.5), Inches(0.6),
                 effect, font=B_FONT, size=11.5, color=MIDNIGHT,
                 anchor=MSO_ANCHOR.TOP, line_spacing=1.2)
    notes(s, """TIMING: 1.5 minutes.

WHAT TO SAY:
Here are the four tiers. Trusted: trust in the validator meets or exceeds the domain's configured threshold. Block goes on the main chain, transactions get applied, trust edges it carries become verified and available for queries. This is the normal case. Tentative: trust is above the distrust threshold but below full trust — you're not sure yet. Store it separately, don't build on top. It can be promoted to Trusted later if more supporting evidence arrives. Untrusted: trust is below the distrust threshold. Don't store the block, but do extract any trust-edge data it carries — those edges become 'unverified' and available with appropriate discount. Invalid: cryptographic verification fails outright. Reject, log, rate-limit the sender. The key insight is that 'untrusted' is different from 'invalid' — an untrusted block can still carry useful trust-graph information even if we don't accept its state changes.

KEY POINTS:
• Trusted: full acceptance
• Tentative: hold, possibly promote
• Untrusted: extract edges only
• Invalid: reject outright
• Trust-data extraction is the subtle one

TRANSITION:
This produces the deliberate consequence: different nodes see different chains.""")
    p += 1

    # ---------- 28. Why nodes differ ----------
    s = slide_content(prs, "Why Alice and Bob see different chains", kicker="CONCEPT 4 — THE TRADE-OFF", page=p, total=total)
    # Two columns: Alice and Bob
    # Alice
    rounded(s, Inches(0.6), Inches(1.85), Inches(6), Inches(5), ICE, corner=0.05)
    rect(s, Inches(0.6), Inches(1.85), Inches(6), Inches(0.55), NAVY)
    text_box(s, Inches(0.6), Inches(1.85), Inches(6), Inches(0.55),
             "ALICE'S VIEW", font=H_FONT, size=16, color=WHITE, bold=True,
             align=PP_ALIGN.CENTER, anchor=MSO_ANCHOR.MIDDLE)
    text_box(s, Inches(0.85), Inches(2.55), Inches(5.5), Inches(0.35),
             "Alice trusts validators:  {A, B, C}",
             font=CODE_FONT, size=12, color=MIDNIGHT, bold=True)
    # Alice's chain
    chain_a = [("a1", NAVY, "A"), ("a2", NAVY, "A"), ("b3", TEAL, "B"),
               ("c4", AMBER, "C"), ("b5", TEAL, "B")]
    for i, (lbl, col, vlab) in enumerate(chain_a):
        x = Inches(0.85 + i * 1.07)
        rect(s, x, Inches(3.2), Inches(0.95), Inches(0.9), col)
        text_box(s, x, Inches(3.2), Inches(0.95), Inches(0.9), vlab,
                 font=H_FONT, size=26, color=WHITE, bold=True,
                 align=PP_ALIGN.CENTER, anchor=MSO_ANCHOR.MIDDLE)
    text_box(s, Inches(0.85), Inches(4.2), Inches(5.5), Inches(0.4),
             "5 blocks accepted", font=B_FONT, size=11, color=SLATE,
             italic=True)
    # Rejected
    text_box(s, Inches(0.85), Inches(4.6), Inches(5.5), Inches(0.35),
             "Rejected (Alice distrusts D):",
             font=B_FONT, size=11, color=RED, bold=True)
    for i, lbl in enumerate(["D1", "D2"]):
        x = Inches(0.85 + i * 1.07)
        rect(s, x, Inches(5.0), Inches(0.95), Inches(0.7), None,
             line=RED, line_w=1.5)
        text_box(s, x, Inches(5.0), Inches(0.95), Inches(0.7), lbl,
                 font=H_FONT, size=18, color=RED, bold=True,
                 align=PP_ALIGN.CENTER, anchor=MSO_ANCHOR.MIDDLE)
    text_box(s, Inches(0.85), Inches(5.85), Inches(5.5), Inches(1),
             "Alice sees a 5-block history. She doesn't know or care what D is doing.",
             font=B_FONT, size=12, color=MIDNIGHT, italic=True,
             line_spacing=1.3)

    # Bob
    rounded(s, Inches(6.9), Inches(1.85), Inches(5.8), Inches(5), ICE, corner=0.05)
    rect(s, Inches(6.9), Inches(1.85), Inches(5.8), Inches(0.55), AMBER)
    text_box(s, Inches(6.9), Inches(1.85), Inches(5.8), Inches(0.55),
             "BOB'S VIEW", font=H_FONT, size=16, color=MIDNIGHT, bold=True,
             align=PP_ALIGN.CENTER, anchor=MSO_ANCHOR.MIDDLE)
    text_box(s, Inches(7.15), Inches(2.55), Inches(5.3), Inches(0.35),
             "Bob trusts validators:  {A, B, D}",
             font=CODE_FONT, size=12, color=MIDNIGHT, bold=True)
    # Bob's chain
    chain_b = [("a1", NAVY, "A"), ("a2", NAVY, "A"), ("b3", TEAL, "B"),
               ("d4", TEAL, "D"), ("d5", TEAL, "D"), ("b6", TEAL, "B")]
    for i, (lbl, col, vlab) in enumerate(chain_b):
        x = Inches(7.15 + i * 0.87)
        rect(s, x, Inches(3.2), Inches(0.75), Inches(0.9), col)
        text_box(s, x, Inches(3.2), Inches(0.75), Inches(0.9), vlab,
                 font=H_FONT, size=22, color=WHITE, bold=True,
                 align=PP_ALIGN.CENTER, anchor=MSO_ANCHOR.MIDDLE)
    text_box(s, Inches(7.15), Inches(4.2), Inches(5.3), Inches(0.4),
             "6 blocks accepted (Bob sees D's blocks)",
             font=B_FONT, size=11, color=SLATE, italic=True)
    text_box(s, Inches(7.15), Inches(4.6), Inches(5.3), Inches(0.35),
             "Rejected (Bob distrusts C):",
             font=B_FONT, size=11, color=RED, bold=True)
    for i, lbl in enumerate(["C1"]):
        x = Inches(7.15 + i * 1.07)
        rect(s, x, Inches(5.0), Inches(0.95), Inches(0.7), None,
             line=RED, line_w=1.5)
        text_box(s, x, Inches(5.0), Inches(0.95), Inches(0.7), lbl,
                 font=H_FONT, size=18, color=RED, bold=True,
                 align=PP_ALIGN.CENTER, anchor=MSO_ANCHOR.MIDDLE)
    text_box(s, Inches(7.15), Inches(5.85), Inches(5.3), Inches(1),
             "Bob has a different view — sees D's activity, misses C's. Both nodes are correct for their observers.",
             font=B_FONT, size=12, color=MIDNIGHT, italic=True, line_spacing=1.3)
    notes(s, """TIMING: 1.5 minutes.

WHAT TO SAY:
Let's make the consequence concrete. Alice trusts validators A, B, and C. Bob trusts A, B, and D. They agree on everything A and B do. But Alice accepts C's blocks and ignores D's, while Bob accepts D's blocks and ignores C's. Both views are correct from each observer's perspective. This trade-off — sacrificing 'one chain for everyone' in favor of 'each observer's context is respected' — is a deliberate design choice. It fits private, consortium, and federated deployments where different parties legitimately disagree about whom to trust. It does NOT fit public permissionless networks where you want a single canonical chain — Bitcoin and Ethereum are the right choice there. Pick the consensus model that matches your problem. Quidnug picks relational.

KEY POINTS:
• Alice and Bob see different histories
• Both correct from their vantage points
• Fits consortium/federated, not permissionless public
• Deliberate design trade-off, not a bug

TRANSITION:
Last core concept — transactions.""")
    p += 1

    # ---------- 29. Transactions ----------
    s = slide_content(prs, "Transactions: typed and signed", kicker="CONCEPT 5", page=p, total=total)
    tx_types = [
        ("TRUST", "Declare trust between quids", TEAL),
        ("IDENTITY", "Name, attributes, home domain", NAVY),
        ("TITLE", "Ownership of assets", AMBER),
        ("EVENT", "Append to a subject's stream", TEAL),
        ("ANCHOR", "Rotate/invalidate key epoch", NAVY),
        ("GUARDIAN_SET_UPDATE", "Install M-of-N quorum", AMBER),
        ("GUARDIAN_RECOVERY_*", "Init / Veto / Commit", TEAL),
        ("GUARDIAN_RESIGN", "Guardian withdraws consent", NAVY),
        ("FORK_BLOCK", "Coordinate protocol upgrade", AMBER),
    ]
    # 3x3 grid
    for i, (name, desc, col) in enumerate(tx_types):
        x = Inches(0.6 + (i % 3) * 4.2)
        y = Inches(1.85 + (i // 3) * 1.6)
        rounded(s, x, y, Inches(4.0), Inches(1.4), ICE, corner=0.05)
        rect(s, x, y, Inches(4.0), Inches(0.5), col)
        text_box(s, x, y, Inches(4.0), Inches(0.5), name,
                 font=CODE_FONT, size=14,
                 color=(MIDNIGHT if col == AMBER else WHITE), bold=True,
                 align=PP_ALIGN.CENTER, anchor=MSO_ANCHOR.MIDDLE)
        text_box(s, x + Inches(0.2), y + Inches(0.6), Inches(3.6), Inches(0.7),
                 desc, font=B_FONT, size=12, color=SLATE,
                 italic=True, anchor=MSO_ANCHOR.MIDDLE, align=PP_ALIGN.CENTER)
    notes(s, """TIMING: 1.5 minutes.

WHAT TO SAY:
Everything that happens in Quidnug is a typed, signed transaction. Here are the nine core types. TRUST declares trust from one quid to another. IDENTITY registers or updates a quid's metadata. TITLE records ownership of assets, including fractional and multi-party ownership. EVENT appends a signed record to a subject's append-only event stream — this is the workhorse for audit trails. ANCHOR is how keys are rotated or invalidated — critical for the key-lifecycle story. GUARDIAN_SET_UPDATE installs an M-of-N guardian quorum. The three GUARDIAN_RECOVERY transactions — Init, Veto, and Commit — drive the time-locked recovery flow. GUARDIAN_RESIGN lets a guardian withdraw consent. And FORK_BLOCK coordinates on-chain protocol upgrades at a future block height. Nine types, each with a strict schema, each signed, each going into blocks. That's the whole transaction vocabulary.

KEY POINTS:
• 9 transaction types
• All typed, all signed, all on-chain
• TRUST and EVENT are the workhorses
• ANCHOR + GUARDIAN drive the key lifecycle

TRANSITION:
OK. Concepts done. Let's dive into the technical primitives that make this work at scale.""")
    p += 1

    # ---------- SECTION 4 divider ----------
    slide_section(prs, 4, "PART FOUR", "Technical primitives")
    p += 1

    # ---------- 31. Event streams ----------
    s = slide_content(prs, "Event streams — tamper-evident audit logs", kicker="PRIMITIVE 1", page=p, total=total)
    # Top: visualization
    rounded(s, Inches(0.6), Inches(1.85), Inches(12.1), Inches(2.8), ICE, corner=0.05)
    text_box(s, Inches(0.85), Inches(2.0), Inches(11.6), Inches(0.4),
             "ALICE'S EVENT STREAM — APPEND-ONLY TIMELINE",
             font=B_FONT, size=11, color=SLATE, bold=True)
    # Event pills
    events = [
        ("profile.updated", "#01", TEAL),
        ("trust.granted", "#02", TEAL),
        ("trust.revoked", "#03", AMBER),
        ("identity.rotated", "#04", NAVY),
        ("dispute.opened", "#05", RED),
        ("dispute.resolved", "#06", GREEN),
    ]
    for i, (name, seq, col) in enumerate(events):
        x = Inches(0.85 + i * 2.05)
        y = Inches(2.55)
        rounded(s, x, y, Inches(1.95), Inches(1.15), col, corner=0.08)
        text_box(s, x, y + Inches(0.1), Inches(1.95), Inches(0.35),
                 seq, font=CODE_FONT, size=10, color=WHITE, bold=True,
                 align=PP_ALIGN.CENTER)
        text_box(s, x, y + Inches(0.42), Inches(1.95), Inches(0.6),
                 name, font=H_FONT, size=12,
                 color=(MIDNIGHT if col == AMBER else WHITE), bold=True,
                 align=PP_ALIGN.CENTER, anchor=MSO_ANCHOR.MIDDLE)
        if i < len(events) - 1:
            arrow(s, x + Inches(1.95), y + Inches(0.55),
                  x + Inches(2.05), y + Inches(0.55), color=SLATE, width=1.5)
    text_box(s, Inches(0.85), Inches(3.95), Inches(11.6), Inches(0.4),
             "Each event: signed · monotonic sequence · append-only · optionally-encrypted payload (hash on-chain)",
             font=B_FONT, size=11, color=SLATE, italic=True, align=PP_ALIGN.CENTER)

    # Bottom: properties
    cards_data = [
        ("Append-only", "Never modified. Never deleted.",
         "Reorderings require rewriting history — cryptographically evident."),
        ("Sequenced", "Each event has monotonic seq.",
         "Gaps and reorderings detectable."),
        ("Privacy-friendly", "Payload optionally encrypted.",
         "Hash + CID on-chain; full payload off-chain."),
    ]
    for i, (h, sub, body) in enumerate(cards_data):
        x = Inches(0.6 + i * 4.07)
        y = Inches(4.85)
        rounded(s, x, y, Inches(3.97), Inches(2.0), ICE, corner=0.06)
        rect(s, x, y, Inches(0.09), Inches(2.0), TEAL)
        text_box(s, x + Inches(0.25), y + Inches(0.15), Inches(3.6), Inches(0.4),
                 h, font=H_FONT, size=15, color=MIDNIGHT, bold=True)
        text_box(s, x + Inches(0.25), y + Inches(0.55), Inches(3.6), Inches(0.35),
                 sub, font=B_FONT, size=12, color=TEAL, bold=True, italic=True)
        text_box(s, x + Inches(0.25), y + Inches(0.95), Inches(3.6), Inches(1),
                 body, font=B_FONT, size=12, color=SLATE, line_spacing=1.3)
    notes(s, """TIMING: 1.5 minutes.

WHAT TO SAY:
Event streams are how Quidnug records 'this happened to this subject at this time.' Every quid and every title can have an event stream — an append-only, monotonically-sequenced, signed timeline. Each event has a sequence number, a type string (app-defined like 'profile.updated' or 'fraud.signal.card-testing'), a payload that can be inline up to 64KB or referenced via IPFS CID for larger content, and a signature. Three key properties. Append-only — you can never modify or delete past events. Cryptographically you could try, but it would be evident on the chain. Sequenced — gaps or reorderings are detectable. Privacy-friendly — for sensitive data, the payload goes off-chain encrypted, with only a hash and CID on-chain. This is how we handle credit events, healthcare consents, and private AI training metadata without exposing everything publicly. Event streams are the workhorse of use cases — almost every use case we'll see uses them heavily.

KEY POINTS:
• Append-only, monotonic, signed
• 64KB inline or IPFS-referenced
• Hash-on-chain + encrypted-off-chain for privacy
• Core primitive for audit trails

TRANSITION:
Key lifecycle. Rotating signing keys without losing history.""")
    p += 1

    # ---------- 32. Key lifecycle — epochs & anchors ----------
    s = slide_content(prs, "Key lifecycle: epochs & anchors", kicker="PRIMITIVE 2", page=p, total=total)
    # Timeline with epochs
    rounded(s, Inches(0.6), Inches(1.85), Inches(12.1), Inches(3), ICE, corner=0.05)
    text_box(s, Inches(0.85), Inches(2.0), Inches(11.6), Inches(0.4),
             "ALICE'S KEY LIFECYCLE OVER 18 MONTHS",
             font=B_FONT, size=11, color=SLATE, bold=True)
    # Epoch bars
    y_bar = Inches(2.8)
    epochs = [
        (Inches(0.85), Inches(2.5), "EPOCH 0", "Original key", NAVY),
        (Inches(3.35), Inches(2.5), "EPOCH 1", "Rotated (scheduled)", TEAL),
        (Inches(5.85), Inches(3.2), "EPOCH 2", "Rotated via guardians", AMBER),
        (Inches(9.05), Inches(3.5), "EPOCH 3", "Current", GREEN),
    ]
    for x, w, label, sub, col in epochs:
        rect(s, x, y_bar, w, Inches(0.7), col)
        text_box(s, x, y_bar, w, Inches(0.7), label,
                 font=H_FONT, size=14,
                 color=(MIDNIGHT if col == AMBER else WHITE), bold=True,
                 align=PP_ALIGN.CENTER, anchor=MSO_ANCHOR.MIDDLE)
        text_box(s, x, y_bar + Inches(0.75), w, Inches(0.35),
                 sub, font=B_FONT, size=11, color=SLATE,
                 italic=True, align=PP_ALIGN.CENTER)
    # Anchor events under
    anchors = [
        (Inches(3.1), "Rotation\nanchor"),
        (Inches(5.6), "Guardian\nrecovery"),
        (Inches(8.8), "Rotation\nanchor"),
    ]
    for xx, lab in anchors:
        hexagon(s, xx, Inches(4.05), Inches(0.5), Inches(0.4), AMBER)
        text_box(s, xx - Inches(0.4), Inches(4.5), Inches(1.3), Inches(0.5),
                 lab, font=B_FONT, size=10, color=MIDNIGHT,
                 align=PP_ALIGN.CENTER, line_spacing=1.1)

    # Bottom: rules
    rounded(s, Inches(0.6), Inches(5.15), Inches(12.1), Inches(1.8), MIDNIGHT, corner=0.05)
    text_box(s, Inches(0.85), Inches(5.3), Inches(11.6), Inches(0.45),
             "THE RULES", font=H_FONT, size=16, color=TEAL, bold=True)
    rules = [
        ("Each epoch has its own key", "rotation produces new pubkey + ledger entry"),
        ("Anchors are signed, monotonic", "replayed rotation = rejected"),
        ("Old epochs stay queryable", "historical signatures remain verifiable"),
        ("Invalidation freezes an epoch", "emergency response for compromise"),
    ]
    for i, (h, b) in enumerate(rules):
        x = Inches(0.85 + (i % 2) * 6)
        y = Inches(5.75 + (i // 2) * 0.55)
        oval(s, x, y, Inches(0.3), Inches(0.3), TEAL)
        text_box(s, x + Inches(0.4), y - Inches(0.02), Inches(5.4), Inches(0.35),
                 h, font=B_FONT, size=12, color=WHITE, bold=True)
        text_box(s, x + Inches(0.4), y + Inches(0.28), Inches(5.4), Inches(0.3),
                 b, font=B_FONT, size=10.5, color=CLOUD, italic=True)
    notes(s, """TIMING: 2 minutes.

WHAT TO SAY:
Keys don't live forever. They need to be rotated — scheduled rotations for hygiene, emergency rotations for compromise, and recovery rotations when keys are lost. Quidnug models this with epochs. Alice starts at Epoch 0 with an original key. Six months in, she rotates to Epoch 1 via a signed Anchor transaction — a normal operational step. A year in, her device is lost; her guardians initiate recovery and she ends up at Epoch 2. A year and a half in, she rotates again scheduled. Four epochs. Each has its own signing key. Each rotation is an Anchor transaction — signed, monotonic, cryptographically bound to the previous epoch. The four rules at the bottom: epochs have their own keys, anchors are signed and monotonic (replays rejected), old epochs stay queryable (historical sigs remain valid), and emergency invalidation freezes an epoch. This is QDP-0001 and QDP-0002 working together.

KEY POINTS:
• Epochs increment on each rotation
• Anchor transactions drive rotations
• Historical sigs remain verifiable
• Invalidation is the emergency freeze

TRANSITION:
Scheduled rotations are easy. The hard case is when the key is lost or compromised. Enter guardians.""")
    p += 1

    # ---------- 33. Guardians ----------
    s = slide_content(prs, "Guardians: M-of-N recovery", kicker="PRIMITIVE 3 (QDP-0002)", page=p, total=total)
    # Central subject + surrounding guardians
    # Subject in center
    subj_x, subj_y = 6.1, 3.7
    subj_w, subj_h = 1.1, 0.9
    oval(s, Inches(subj_x), Inches(subj_y), Inches(subj_w), Inches(subj_h),
         AMBER)
    text_box(s, Inches(subj_x), Inches(subj_y), Inches(subj_w), Inches(subj_h),
             "Subject\n(Alice)",
             font=H_FONT, size=11, color=MIDNIGHT, bold=True,
             align=PP_ALIGN.CENTER, anchor=MSO_ANCHOR.MIDDLE,
             line_spacing=1.15)
    # Five guardians around — larger orbital radius, smaller ovals
    import math
    guardian_labels = ["Spouse", "Manager", "Lawyer", "Backup HSM", "Trustee"]
    colors = [TEAL, NAVY, TEAL, NAVY, TEAL]
    subj_cx = subj_x + subj_w / 2
    subj_cy = subj_y + subj_h / 2
    g_w, g_h = 1.0, 0.7
    radius = 2.25   # guarantees non-overlap given subj+guardian half-diagonals
    for i, (lab, col) in enumerate(zip(guardian_labels, colors)):
        angle = -math.pi/2 + (i * 2 * math.pi / 5)
        gcx = subj_cx + radius * math.cos(angle)
        gcy = subj_cy + radius * math.sin(angle)
        gx = gcx - g_w / 2
        gy = gcy - g_h / 2
        oval(s, Inches(gx), Inches(gy), Inches(g_w), Inches(g_h), col)
        text_box(s, Inches(gx), Inches(gy), Inches(g_w), Inches(g_h), lab,
                 font=B_FONT, size=10, color=WHITE, bold=True,
                 align=PP_ALIGN.CENTER, anchor=MSO_ANCHOR.MIDDLE,
                 line_spacing=1.1)
        # Line from guardian center toward subject center, stopping
        # short of the subject shape
        arrow(s, Inches(gcx), Inches(gcy),
              Inches(subj_cx), Inches(subj_cy),
              color=SLATE, width=0.75, end_arrow=False)

    # Left panel: the rule
    rounded(s, Inches(0.6), Inches(1.85), Inches(3.5), Inches(5), MIDNIGHT, corner=0.05)
    text_box(s, Inches(0.8), Inches(2.0), Inches(3.1), Inches(0.5),
             "THE RULE", font=B_FONT, size=11, color=TEAL, bold=True)
    text_box(s, Inches(0.8), Inches(2.45), Inches(3.1), Inches(3),
             "If Alice loses her key,\n\nany 3 of her 5 guardians\ncan sign a rotation to\na new key — but only\nafter a time-lock delay.",
             font=H_FONT, size=15, color=WHITE, line_spacing=1.4)
    text_box(s, Inches(0.8), Inches(5.5), Inches(3.1), Inches(1.2),
             "Alice (if her old key still works)\ncan veto during the delay.",
             font=B_FONT, size=11, color=AMBER, italic=True, line_spacing=1.3)

    # Right panel: config
    rounded(s, Inches(9.2), Inches(1.85), Inches(3.5), Inches(5), ICE, corner=0.05)
    text_box(s, Inches(9.4), Inches(2.0), Inches(3.1), Inches(0.4),
             "GUARDIAN SET CONFIG", font=B_FONT, size=11, color=SLATE, bold=True)
    config = [
        ("threshold", "3", TEAL),
        ("guardians", "5", NAVY),
        ("recoveryDelay", "1h – 7d", AMBER),
        ("requireRotation", "true", GREEN),
        ("epoch", "pinned per ref", NAVY),
    ]
    for i, (k, v, col) in enumerate(config):
        y = Inches(2.5 + i * 0.8)
        rect(s, Inches(9.4), y, Inches(0.07), Inches(0.6), col)
        text_box(s, Inches(9.55), y, Inches(1.4), Inches(0.6), k,
                 font=CODE_FONT, size=11, color=MIDNIGHT, bold=True,
                 anchor=MSO_ANCHOR.MIDDLE)
        text_box(s, Inches(11.05), y, Inches(1.6), Inches(0.6), v,
                 font=CODE_FONT, size=12, color=col, bold=True,
                 anchor=MSO_ANCHOR.MIDDLE)
    notes(s, """TIMING: 2 minutes.

WHAT TO SAY:
Here's the big one. Guardian-based recovery. Alice picks 5 people she trusts — a spouse, a manager, a lawyer, a backup HSM she controls, a trustee. She sets a threshold of 3. If she loses her key, any 3 of those 5 can sign a Guardian Recovery Init transaction nominating a new public key for Alice. But the new key doesn't immediately activate — there's a time-lock window, anywhere from an hour to a week depending on the config. During the window, Alice — if her old key actually still works — can submit a Veto and cancel the recovery. That protects against social-engineering attacks where adversaries trick the guardians. If Alice is actually unreachable, the window elapses and the rotation commits. Guardian sets have five config parameters: threshold M, total guardians N, recovery delay, whether rotation is required versus guardian-only, and the guardian's pinned key epoch at set-install time. That last one is important — if Alice's spouse rotates her key later, it doesn't silently grant her new key recovery power.

KEY POINTS:
• M-of-N with time-lock veto
• Original key can veto if still alive
• 5 config parameters
• Replaces central escrow for key recovery

TRANSITION:
Let's zoom in on the time-lock.""")
    p += 1

    # ---------- 34. Time-locked veto window ----------
    s = slide_content(prs, "The time-locked veto window", kicker="PRIMITIVE 3 cont'd", page=p, total=total)
    # Timeline
    rounded(s, Inches(0.6), Inches(1.85), Inches(12.1), Inches(3.6), ICE, corner=0.05)
    text_box(s, Inches(0.85), Inches(2.0), Inches(11.6), Inches(0.4),
             "TIMELINE OF A GUARDIAN RECOVERY",
             font=B_FONT, size=11, color=SLATE, bold=True)
    # Bar
    bar_y = Inches(2.85)
    bar_h = Inches(0.6)
    # Init phase
    rect(s, Inches(0.85), bar_y, Inches(2.5), bar_h, NAVY)
    text_box(s, Inches(0.85), bar_y, Inches(2.5), bar_h, "INIT",
             font=H_FONT, size=14, color=WHITE, bold=True,
             align=PP_ALIGN.CENTER, anchor=MSO_ANCHOR.MIDDLE)
    text_box(s, Inches(0.85), bar_y + Inches(0.7), Inches(2.5), Inches(0.35),
             "t = 0", font=CODE_FONT, size=12, color=NAVY, bold=True,
             align=PP_ALIGN.CENTER)
    text_box(s, Inches(0.85), bar_y + Inches(1.05), Inches(2.5), Inches(1.5),
             "3-of-5 guardians sign init with nominated new key",
             font=B_FONT, size=11, color=SLATE, italic=True,
             align=PP_ALIGN.CENTER, line_spacing=1.25)

    # Window phase
    rect(s, Inches(3.35), bar_y, Inches(6), bar_h, AMBER)
    text_box(s, Inches(3.35), bar_y, Inches(6), bar_h, "VETO WINDOW  (e.g. 24h)",
             font=H_FONT, size=14, color=MIDNIGHT, bold=True,
             align=PP_ALIGN.CENTER, anchor=MSO_ANCHOR.MIDDLE)
    text_box(s, Inches(3.35), bar_y + Inches(0.7), Inches(6), Inches(0.35),
             "0 < t < delay", font=CODE_FONT, size=12, color=AMBER, bold=True,
             align=PP_ALIGN.CENTER)
    text_box(s, Inches(3.35), bar_y + Inches(1.05), Inches(6), Inches(1.5),
             "Subject — if old key works — can submit VETO\n"
             "OR guardians can submit COMMIT early with +1 approval",
             font=B_FONT, size=11, color=SLATE, italic=True,
             align=PP_ALIGN.CENTER, line_spacing=1.25)

    # Commit phase
    rect(s, Inches(9.35), bar_y, Inches(3.35), bar_h, GREEN)
    text_box(s, Inches(9.35), bar_y, Inches(3.35), bar_h, "COMMIT",
             font=H_FONT, size=14, color=WHITE, bold=True,
             align=PP_ALIGN.CENTER, anchor=MSO_ANCHOR.MIDDLE)
    text_box(s, Inches(9.35), bar_y + Inches(0.7), Inches(3.35), Inches(0.35),
             "t = delay", font=CODE_FONT, size=12, color=GREEN, bold=True,
             align=PP_ALIGN.CENTER)
    text_box(s, Inches(9.35), bar_y + Inches(1.05), Inches(3.35), Inches(1.5),
             "Rotation takes effect — new epoch signs going forward",
             font=B_FONT, size=11, color=SLATE, italic=True,
             align=PP_ALIGN.CENTER, line_spacing=1.25)

    # Why this works
    rounded(s, Inches(0.6), Inches(5.65), Inches(12.1), Inches(1.3), MIDNIGHT, corner=0.05)
    text_box(s, Inches(0.85), Inches(5.8), Inches(11.6), Inches(0.4),
             "WHY THIS WORKS", font=H_FONT, size=14, color=TEAL, bold=True)
    text_box(s, Inches(0.85), Inches(6.2), Inches(11.6), Inches(0.75),
             "Social-engineering an M-of-N quorum is hard. Social-engineering M-of-N AND stopping the victim from noticing for the entire delay window is much harder. The asymmetry is structural.",
             font=B_FONT, size=13, color=CLOUD, line_spacing=1.3)
    notes(s, """TIMING: 1.5 minutes.

WHAT TO SAY:
Here's the timeline in detail. At t=0, the M-of-N guardians sign an Init transaction. The Init carries the nominated new public key. That Init lands in a block. The clock starts. During the veto window — which can be configured anywhere from an hour for low-value keys to a week for institutional crypto custody — the subject, if their old key still works, can submit a Veto that cancels the recovery. Or guardians can submit a Commit early if they have a plus-one additional signature above threshold, for genuine emergencies. If nothing happens during the window, at t equals delay, the Commit fires automatically or can be submitted by anyone — the authorization was the Init plus the elapsed time. The rotation takes effect; the new epoch signs going forward. Why does this work? Because social-engineering M-of-N is already hard. Social-engineering M-of-N AND suppressing the victim's response for the whole delay is much harder. The asymmetry is what gives us security.

KEY POINTS:
• Init → Window → Commit
• Window is configurable 1h–1yr
• Veto during window kills it
• Commit after window makes it permanent
• Asymmetric difficulty

TRANSITION:
Keys are rotated. The protocol needs to tell the rest of the network.""")
    p += 1

    # ---------- 35. Cross-domain gossip ----------
    s = slide_content(prs, "Cross-domain anchor gossip", kicker="PRIMITIVE 4 (QDP-0003)", page=p, total=total)
    # Three domains side-by-side, with gossip arrows
    rounded(s, Inches(0.6), Inches(1.85), Inches(12.1), Inches(3.9), ICE, corner=0.05)
    text_box(s, Inches(0.85), Inches(2.0), Inches(11.6), Inches(0.4),
             "ALICE ROTATES IN DOMAIN A — EVERY DOMAIN SHE OPERATES IN MUST LEARN",
             font=B_FONT, size=11, color=SLATE, bold=True)

    # Domain A (rotation happens here)
    rounded(s, Inches(0.85), Inches(2.6), Inches(3.5), Inches(2.7), NAVY, corner=0.08)
    text_box(s, Inches(0.85), Inches(2.75), Inches(3.5), Inches(0.4),
             "DOMAIN A", font=B_FONT, size=12, color=TEAL, bold=True,
             align=PP_ALIGN.CENTER)
    text_box(s, Inches(0.85), Inches(3.2), Inches(3.5), Inches(0.5),
             "ALICE ROTATES", font=H_FONT, size=16, color=WHITE, bold=True,
             align=PP_ALIGN.CENTER)
    hexagon(s, Inches(2.0), Inches(3.85), Inches(1.2), Inches(1), AMBER)
    text_box(s, Inches(2.0), Inches(3.85), Inches(1.2), Inches(1),
             "Anchor\nsealed", font=H_FONT, size=11, color=MIDNIGHT, bold=True,
             align=PP_ALIGN.CENTER, anchor=MSO_ANCHOR.MIDDLE, line_spacing=1.2)
    # Fingerprint sealed
    text_box(s, Inches(0.85), Inches(4.95), Inches(3.5), Inches(0.4),
             "block sealed → fingerprint signed",
             font=B_FONT, size=10, color=CLOUD, italic=True,
             align=PP_ALIGN.CENTER)

    # Gossip arrows
    arrow(s, Inches(4.4), Inches(3.95), Inches(5.1), Inches(3.25),
          color=TEAL, width=2.5)
    arrow(s, Inches(4.4), Inches(3.95), Inches(5.1), Inches(4.65),
          color=TEAL, width=2.5)
    text_box(s, Inches(4.45), Inches(2.8), Inches(0.6), Inches(0.35),
             "gossip", font=B_FONT, size=10, color=TEAL, bold=True, italic=True)

    # Domain B
    rounded(s, Inches(5.1), Inches(2.6), Inches(3.5), Inches(1.25), ICE, corner=0.08,
            line=TEAL, line_w=1.5)
    text_box(s, Inches(5.1), Inches(2.7), Inches(3.5), Inches(0.4),
             "DOMAIN B", font=B_FONT, size=12, color=NAVY, bold=True,
             align=PP_ALIGN.CENTER)
    text_box(s, Inches(5.1), Inches(3.15), Inches(3.5), Inches(0.6),
             "Verifies fingerprint\n→ updates Alice's epoch",
             font=B_FONT, size=11, color=MIDNIGHT, align=PP_ALIGN.CENTER,
             line_spacing=1.25)

    # Domain C
    rounded(s, Inches(5.1), Inches(4.1), Inches(3.5), Inches(1.25), ICE, corner=0.08,
            line=TEAL, line_w=1.5)
    text_box(s, Inches(5.1), Inches(4.2), Inches(3.5), Inches(0.4),
             "DOMAIN C", font=B_FONT, size=12, color=NAVY, bold=True,
             align=PP_ALIGN.CENTER)
    text_box(s, Inches(5.1), Inches(4.65), Inches(3.5), Inches(0.6),
             "Verifies fingerprint\n→ updates Alice's epoch",
             font=B_FONT, size=11, color=MIDNIGHT, align=PP_ALIGN.CENTER,
             line_spacing=1.25)

    # Domain D (skipped — no state about Alice)
    rounded(s, Inches(9.0), Inches(3.35), Inches(3.5), Inches(1.25), WHITE, corner=0.08,
            line=SLATE, line_w=1)
    text_box(s, Inches(9.0), Inches(3.45), Inches(3.5), Inches(0.4),
             "DOMAIN D", font=B_FONT, size=12, color=SLATE, bold=True,
             align=PP_ALIGN.CENTER)
    text_box(s, Inches(9.0), Inches(3.9), Inches(3.5), Inches(0.6),
             "Doesn't care — no state\nabout Alice here",
             font=B_FONT, size=11, color=SLATE, italic=True,
             align=PP_ALIGN.CENTER, line_spacing=1.25)

    # Bottom: properties
    rounded(s, Inches(0.6), Inches(5.95), Inches(12.1), Inches(1), MIDNIGHT, corner=0.05)
    text_box(s, Inches(0.85), Inches(6.05), Inches(11.6), Inches(0.85),
             "Gossip messages self-verify: anchor + origin-block-fingerprint + producer signature. Deduplicated by MessageID. Replay-safe by construction.",
             font=B_FONT, size=13, color=CLOUD, italic=True, anchor=MSO_ANCHOR.MIDDLE)
    notes(s, """TIMING: 1.5 minutes.

WHAT TO SAY:
Alice has state in multiple domains. Her bank domain, her healthcare domain, her credential-verification domain. When she rotates her key in domain A, every other domain she's active in needs to learn about it — otherwise Alice's old-epoch signatures might still be accepted elsewhere, undermining the rotation's purpose. QDP-0003 solves this with cross-domain anchor gossip. When the rotation seals into a block in domain A, the block's validator emits a signed fingerprint — a cryptographic attestation that a specific block hash is the head at height H in domain A. That fingerprint plus the rotation anchor form a gossip message, which propagates to every node that has state about Alice in any domain. Nodes in domain B and domain C verify the fingerprint against the producer's key, extract the rotation, and update their local ledger. Domain D doesn't care — Alice has no state there, no gossip lands. The messages self-verify, dedupe by MessageID, and are replay-safe.

KEY POINTS:
• Fingerprint + anchor + signature = gossip
• Propagates to interested nodes only
• Self-verifying, dedupe'd, replay-safe
• This is how cross-domain key rotation stays consistent

TRANSITION:
Gossip historically meant polling. QDP-0005 made it real-time.""")
    p += 1

    # ---------- 36. Push gossip ----------
    s = slide_content(prs, "Push gossip — real-time propagation", kicker="PRIMITIVE 5 (QDP-0005)", page=p, total=total)
    # Before-after comparison
    # Before (pull)
    rounded(s, Inches(0.6), Inches(1.85), Inches(6), Inches(2.8), ICE, corner=0.05)
    rect(s, Inches(0.6), Inches(1.85), Inches(6), Inches(0.5), RED)
    text_box(s, Inches(0.6), Inches(1.85), Inches(6), Inches(0.5),
             "BEFORE (pull)", font=H_FONT, size=15, color=WHITE, bold=True,
             align=PP_ALIGN.CENTER, anchor=MSO_ANCHOR.MIDDLE)
    # Diagram: polling
    for i in range(3):
        y = Inches(2.55 + i * 0.65)
        oval(s, Inches(1.0), y, Inches(0.7), Inches(0.5), NAVY)
        text_box(s, Inches(1.0), y, Inches(0.7), Inches(0.5), f"N{i+1}",
                 font=CODE_FONT, size=12, color=WHITE, bold=True,
                 align=PP_ALIGN.CENTER, anchor=MSO_ANCHOR.MIDDLE)
        text_box(s, Inches(1.8), y, Inches(4.7), Inches(0.5),
                 "poll every 2 min  →  maybe learn about rotation",
                 font=B_FONT, size=12, color=SLATE, italic=True,
                 anchor=MSO_ANCHOR.MIDDLE)
    text_box(s, Inches(0.85), Inches(4.2), Inches(5.5), Inches(0.35),
             "Worst case: 2 min delay × N hops = O(minutes)",
             font=B_FONT, size=11, color=RED, bold=True, align=PP_ALIGN.CENTER)

    # After (push)
    rounded(s, Inches(6.7), Inches(1.85), Inches(6), Inches(2.8), ICE, corner=0.05)
    rect(s, Inches(6.7), Inches(1.85), Inches(6), Inches(0.5), GREEN)
    text_box(s, Inches(6.7), Inches(1.85), Inches(6), Inches(0.5),
             "AFTER (push)", font=H_FONT, size=15, color=WHITE, bold=True,
             align=PP_ALIGN.CENTER, anchor=MSO_ANCHOR.MIDDLE)
    # Producer + fanout
    oval(s, Inches(7.0), Inches(2.9), Inches(1.0), Inches(0.9), TEAL)
    text_box(s, Inches(7.0), Inches(2.9), Inches(1.0), Inches(0.9),
             "Prod", font=B_FONT, size=13, color=WHITE, bold=True,
             align=PP_ALIGN.CENTER, anchor=MSO_ANCHOR.MIDDLE)
    for i in range(3):
        y = Inches(2.55 + i * 0.55)
        oval(s, Inches(11.0), y, Inches(0.8), Inches(0.5), NAVY)
        text_box(s, Inches(11.0), y, Inches(0.8), Inches(0.5), f"N{i+1}",
                 font=CODE_FONT, size=11, color=WHITE, bold=True,
                 align=PP_ALIGN.CENTER, anchor=MSO_ANCHOR.MIDDLE)
        arrow(s, Inches(8.1), Inches(3.35), Inches(11.0), y + Inches(0.25),
              color=TEAL, width=1.5)
    text_box(s, Inches(7.0), Inches(4.2), Inches(5.5), Inches(0.35),
             "Worst case: ~1s per hop × 3 hops = ~3s",
             font=B_FONT, size=11, color=GREEN, bold=True, align=PP_ALIGN.CENTER)

    # Bottom: features
    rounded(s, Inches(0.6), Inches(4.85), Inches(12.1), Inches(2.1), MIDNIGHT, corner=0.05)
    text_box(s, Inches(0.85), Inches(5.0), Inches(11.6), Inches(0.45),
             "QDP-0005 FEATURES", font=H_FONT, size=14, color=TEAL, bold=True)
    features = [
        ("TTL-bounded fanout",
         "Messages carry a hop limit to prevent amplification"),
        ("Dedup-before-validate",
         "Cheap map lookup before expensive ECDSA"),
        ("Implicit subscription",
         "Nodes forward only to peers with matching state"),
        ("Per-producer rate limit",
         "Apply-then-choke — a flood doesn't silence other nodes"),
    ]
    for i, (h, b) in enumerate(features):
        x = Inches(0.85 + (i % 2) * 6)
        y = Inches(5.55 + (i // 2) * 0.7)
        oval(s, x, y + Inches(0.1), Inches(0.3), Inches(0.3), TEAL)
        text_box(s, x + Inches(0.4), y, Inches(5.4), Inches(0.35),
                 h, font=B_FONT, size=12, color=WHITE, bold=True)
        text_box(s, x + Inches(0.4), y + Inches(0.3), Inches(5.4), Inches(0.4),
                 b, font=B_FONT, size=11, color=CLOUD, italic=True)
    notes(s, """TIMING: 1.5 minutes.

WHAT TO SAY:
Before QDP-0005, gossip was pull-only. Nodes periodically polled their peers to ask 'anything new for me?' This is simple but slow. A key rotation could take many minutes to propagate through the network — and a compromised-key window of even 30 seconds is dangerous for high-value identities. QDP-0005 added push. When a rotation or fingerprint is produced, the producer immediately fans it out to every peer it knows about, with a TTL to bound propagation. Each recipient dedupes via a cheap map lookup, verifies the signature, applies, and forwards with TTL decremented. Critical features: dedup runs before the expensive ECDSA verification — so a flood of duplicates is cheap to reject. Implicit subscription — nodes only forward to peers that have state about the involved quids, so we don't waste bandwidth on uninterested nodes. Per-producer rate limit uses an apply-then-choke model: a compromised validator can flood, but only the first N messages forward; their own node still applies so local truth isn't sacrificed. End result: propagation that used to take minutes now takes seconds.

KEY POINTS:
• Push vs pull — seconds vs. minutes
• Dedup-first, then validate
• Subscription is implicit
• Rate-limited per producer

TRANSITION:
Fresh nodes joining the network need a different primitive.""")
    p += 1

    # ---------- 37. K-of-K bootstrap ----------
    s = slide_content(prs, "K-of-K bootstrap — trust-on-first-use solved", kicker="PRIMITIVE 6 (QDP-0008)", page=p, total=total)
    # Problem
    rounded(s, Inches(0.6), Inches(1.85), Inches(6), Inches(2.5), ICE, corner=0.05)
    rect(s, Inches(0.6), Inches(1.85), Inches(6), Inches(0.5), RED)
    text_box(s, Inches(0.6), Inches(1.85), Inches(6), Inches(0.5),
             "THE PROBLEM", font=H_FONT, size=15, color=WHITE, bold=True,
             align=PP_ALIGN.CENTER, anchor=MSO_ANCHOR.MIDDLE)
    text_box(s, Inches(0.85), Inches(2.55), Inches(5.5), Inches(1.7),
             "A fresh node has no history. It needs state to verify incoming traffic — but who does it trust for that initial state?\n\nTrusting ONE peer blindly is risky. Chain-replay from genesis is slow.",
             font=B_FONT, size=12.5, color=MIDNIGHT, line_spacing=1.35)

    # Solution
    rounded(s, Inches(6.7), Inches(1.85), Inches(6), Inches(2.5), ICE, corner=0.05)
    rect(s, Inches(6.7), Inches(1.85), Inches(6), Inches(0.5), GREEN)
    text_box(s, Inches(6.7), Inches(1.85), Inches(6), Inches(0.5),
             "THE SOLUTION", font=H_FONT, size=15, color=WHITE, bold=True,
             align=PP_ALIGN.CENTER, anchor=MSO_ANCHOR.MIDDLE)
    text_box(s, Inches(6.95), Inches(2.55), Inches(5.5), Inches(1.7),
             "Query K peers. All K must agree on snapshot hash, height, and state. Any disagreement fails closed.\n\nK=3 by default. Trust is diversified across peers you pre-selected.",
             font=B_FONT, size=12.5, color=MIDNIGHT, line_spacing=1.35)

    # Diagram
    rounded(s, Inches(0.6), Inches(4.55), Inches(12.1), Inches(2.4), MIDNIGHT, corner=0.05)
    text_box(s, Inches(0.85), Inches(4.7), Inches(11.6), Inches(0.4),
             "BOOTSTRAP FLOW", font=H_FONT, size=13, color=TEAL, bold=True)
    # Fresh node
    oval(s, Inches(1.0), Inches(5.35), Inches(1.4), Inches(1), AMBER)
    text_box(s, Inches(1.0), Inches(5.35), Inches(1.4), Inches(1),
             "FRESH\nNODE", font=H_FONT, size=11, color=MIDNIGHT, bold=True,
             align=PP_ALIGN.CENTER, anchor=MSO_ANCHOR.MIDDLE, line_spacing=1.2)
    # 3 peers
    peer_x = [Inches(4.5), Inches(7.5), Inches(10.5)]
    for i, x in enumerate(peer_x):
        oval(s, x, Inches(5.35), Inches(1.3), Inches(1), TEAL)
        text_box(s, x, Inches(5.35), Inches(1.3), Inches(1),
                 f"Peer {i+1}", font=H_FONT, size=12, color=WHITE, bold=True,
                 align=PP_ALIGN.CENTER, anchor=MSO_ANCHOR.MIDDLE)
        arrow(s, Inches(2.4), Inches(5.85), x, Inches(5.85),
              color=TEAL, width=1.5)
        arrow(s, x + Inches(0.65), Inches(6.4), Inches(1.7), Inches(6.4),
              color=GREEN, width=1.5)

    text_box(s, Inches(1.0), Inches(6.55), Inches(11.6), Inches(0.4),
             "1.  Fresh node asks each peer for snapshot-hash.     2.  All 3 agree on SAME hash?  →  accept & seed.     3.  Any disagreement  →  halt.",
             font=B_FONT, size=10.5, color=CLOUD, italic=True)
    notes(s, """TIMING: 1.5 minutes.

WHAT TO SAY:
When a fresh node joins Quidnug — a brand-new deployment, a new county's election node, a new regional office — it faces a chicken-and-egg problem. To verify incoming traffic, it needs historical state. To get historical state, it needs to trust someone. Trusting one peer blindly is dangerous — that peer could be compromised. Replaying from genesis takes forever. QDP-0008 solves this with K-of-K bootstrap. You pre-select K peers you're willing to trust for bootstrap — typically K=3. The fresh node asks all K for their snapshot hash at a given height. If all K return the same hash, you accept that snapshot and seed your ledger. If any disagreement, you halt and alert an operator. The security property: to manipulate the bootstrap, an attacker would have to compromise all K peers simultaneously. With K=3 spread across different administrative domains, that's hard. This replaces trust-on-first-use with trust-in-a-diverse-quorum.

KEY POINTS:
• K peers, all must agree
• Default K=3
• Any disagreement = halt
• Diversifies trust

TRANSITION:
For coordinated protocol upgrades — fork-block.""")
    p += 1

    # ---------- 38. Fork-block ----------
    s = slide_content(prs, "Fork-block migration trigger", kicker="PRIMITIVE 7 (QDP-0009)", page=p, total=total)
    # Left: the problem
    rounded(s, Inches(0.6), Inches(1.85), Inches(5.9), Inches(5), ICE, corner=0.05)
    text_box(s, Inches(0.85), Inches(2.0), Inches(5.4), Inches(0.4),
             "THE PROBLEM", font=B_FONT, size=11, color=SLATE, bold=True)
    text_box(s, Inches(0.85), Inches(2.4), Inches(5.4), Inches(0.55),
             "Enabling a protocol feature",
             font=H_FONT, size=16, color=MIDNIGHT, bold=True)
    text_box(s, Inches(0.85), Inches(3.0), Inches(5.4), Inches(3.7),
             "Operators want to turn on (e.g.) strict nonce enforcement across the whole federation.\n\nIf everyone doesn't flip the flag at the same time, some nodes reject transactions others accept.\n\n→ Consensus diverges. Chain forks. Ops paged.\n\nManual coordination is the worst possible approach.",
             font=B_FONT, size=12.5, color=MIDNIGHT, line_spacing=1.35)

    # Right: the fix
    rounded(s, Inches(6.65), Inches(1.85), Inches(6), Inches(5), MIDNIGHT, corner=0.05)
    text_box(s, Inches(6.85), Inches(2.0), Inches(5.6), Inches(0.45),
             "THE FIX — FORK-BLOCK TX", font=B_FONT, size=11,
             color=TEAL, bold=True)
    # Schema
    schema = [
        ("feature:", '"require_tx_tree_root"', TEAL),
        ("forkHeight:", "1,200,000", AMBER),
        ("trustDomain:", '"credit.mortgage.us"', WHITE),
        ("signatures:", "[8 of 12 validators]", GREEN),
        ("nonce:", "1", WHITE),
        ("proposedAt:", "<unix>", WHITE),
    ]
    for i, (k, v, col) in enumerate(schema):
        y = Inches(2.5 + i * 0.5)
        text_box(s, Inches(6.9), y, Inches(1.7), Inches(0.4), k,
                 font=CODE_FONT, size=11, color=CLOUD,
                 anchor=MSO_ANCHOR.MIDDLE)
        text_box(s, Inches(8.6), y, Inches(3.9), Inches(0.4), v,
                 font=CODE_FONT, size=11, color=col, bold=True,
                 anchor=MSO_ANCHOR.MIDDLE)
    # Outcome
    text_box(s, Inches(6.85), Inches(5.6), Inches(5.6), Inches(1.1),
             "Quorum of validators sign.\n\nAt forkHeight (1,200,000), every node flips the flag deterministically.\n\nNo coordination outside the protocol.",
             font=B_FONT, size=12, color=CLOUD, italic=True, line_spacing=1.3)
    notes(s, """TIMING: 1.5 minutes.

WHAT TO SAY:
Every federated protocol has this problem. You want to turn on a new feature — say, strict nonce enforcement — across the whole deployment. If operators don't coordinate perfectly, some nodes have the flag on and some off, which means they now have different validation rules, which means they reject each other's blocks, which means consensus diverges. QDP-0009 solves this with the fork-block transaction. A quorum of validators signs a FORK_BLOCK transaction that declares: 'At height 1,200,000, the feature require_tx_tree_root becomes active.' That transaction lands in a block, propagates normally. When each node processes block 1,200,000, it flips the flag deterministically. Before 1,200,000, old validation rules. After 1,200,000, new rules. Every node transitions together without any out-of-band coordination. Quorum of 2-of-3 or better required. Replay protection via nonce. Notice period enforced — you can't fork-block-trigger for a past height. This is the cleanest protocol-level upgrade coordination I've seen.

KEY POINTS:
• FORK_BLOCK tx with forkHeight + feature
• Quorum-signed (≥ 2/3 validators)
• Deterministic activation at height
• Replaces out-of-band coordination

TRANSITION:
Two more primitives. Merkle proofs and lazy epoch propagation.""")
    p += 1

    # ---------- 39. Compact Merkle proofs + Lazy epoch propagation ----------
    s = slide_content(prs, "Merkle proofs + lazy epoch propagation", kicker="PRIMITIVES 8 & 9 (QDP-0010 & QDP-0007)", page=p, total=total)
    # Left: Merkle proofs
    rounded(s, Inches(0.6), Inches(1.85), Inches(6), Inches(5), ICE, corner=0.05)
    rect(s, Inches(0.6), Inches(1.85), Inches(6), Inches(0.5), TEAL)
    text_box(s, Inches(0.6), Inches(1.85), Inches(6), Inches(0.5),
             "COMPACT MERKLE PROOFS  (QDP-0010)",
             font=H_FONT, size=14, color=WHITE, bold=True,
             align=PP_ALIGN.CENTER, anchor=MSO_ANCHOR.MIDDLE)
    text_box(s, Inches(0.85), Inches(2.5), Inches(5.4), Inches(2.8),
             "Anchor gossip ships FULL origin block today — expensive when blocks are large.\n\nH2 adds TransactionsRoot (Merkle root) to blocks. Gossip ships proof (~log₂ tx count hashes) instead of full block.\n\n~70% bandwidth reduction. Light-client path.",
             font=B_FONT, size=12, color=MIDNIGHT, line_spacing=1.35)
    # Diagram
    rect(s, Inches(0.85), Inches(5.4), Inches(1.0), Inches(0.5), AMBER)
    text_box(s, Inches(0.85), Inches(5.4), Inches(1.0), Inches(0.5), "LEAF",
             font=CODE_FONT, size=10, color=MIDNIGHT, bold=True,
             align=PP_ALIGN.CENTER, anchor=MSO_ANCHOR.MIDDLE)
    for i in range(3):
        rect(s, Inches(2.0 + i * 1.1), Inches(5.4), Inches(0.95), Inches(0.5),
             TEAL)
        text_box(s, Inches(2.0 + i * 1.1), Inches(5.4), Inches(0.95), Inches(0.5),
                 "H" + str(i+1),
                 font=CODE_FONT, size=10, color=WHITE, bold=True,
                 align=PP_ALIGN.CENTER, anchor=MSO_ANCHOR.MIDDLE)
        arrow(s, Inches(1.95 + i * 1.1), Inches(5.65),
              Inches(2.05 + i * 1.1), Inches(5.65), color=SLATE, width=1)
    rect(s, Inches(5.3), Inches(5.4), Inches(1.2), Inches(0.5), NAVY)
    text_box(s, Inches(5.3), Inches(5.4), Inches(1.2), Inches(0.5), "ROOT",
             font=CODE_FONT, size=10, color=WHITE, bold=True,
             align=PP_ALIGN.CENTER, anchor=MSO_ANCHOR.MIDDLE)
    text_box(s, Inches(0.85), Inches(6.05), Inches(5.5), Inches(0.6),
             "Proof = leaf hash + sibling hashes along path to root",
             font=B_FONT, size=10, color=SLATE, italic=True)

    # Right: lazy epoch propagation
    rounded(s, Inches(6.7), Inches(1.85), Inches(6), Inches(5), ICE, corner=0.05)
    rect(s, Inches(6.7), Inches(1.85), Inches(6), Inches(0.5), AMBER)
    text_box(s, Inches(6.7), Inches(1.85), Inches(6), Inches(0.5),
             "LAZY EPOCH PROPAGATION  (QDP-0007)",
             font=H_FONT, size=14, color=MIDNIGHT, bold=True,
             align=PP_ALIGN.CENTER, anchor=MSO_ANCHOR.MIDDLE)
    text_box(s, Inches(6.95), Inches(2.5), Inches(5.4), Inches(2.8),
             "A signer who rotates in domain A but rarely signs in domain B — B's local ledger may still have their old epoch.\n\nBefore admitting their next transaction, B quarantines it and probes A's home-domain fingerprint.\n\nCatches 'stale epoch' attacks across domains.",
             font=B_FONT, size=12, color=MIDNIGHT, line_spacing=1.35)
    # Mini flow
    steps = [("Tx arrives", TEAL),
             ("Quarantine", AMBER),
             ("Probe home", NAVY),
             ("Re-validate", GREEN)]
    for i, (name, col) in enumerate(steps):
        x = Inches(6.95 + i * 1.4)
        rounded(s, x, Inches(5.4), Inches(1.3), Inches(0.7), col, corner=0.1)
        text_box(s, x, Inches(5.4), Inches(1.3), Inches(0.7), name,
                 font=B_FONT, size=9,
                 color=(MIDNIGHT if col == AMBER else WHITE), bold=True,
                 align=PP_ALIGN.CENTER, anchor=MSO_ANCHOR.MIDDLE)
        if i < len(steps) - 1:
            arrow(s, x + Inches(1.3), Inches(5.75),
                  x + Inches(1.4), Inches(5.75), color=SLATE, width=1.25)
    text_box(s, Inches(6.95), Inches(6.25), Inches(5.4), Inches(0.5),
             "Timeout → operator policy (reject or admit-with-warn)",
             font=B_FONT, size=10, color=SLATE, italic=True)
    notes(s, """TIMING: 2 minutes.

WHAT TO SAY:
Two more primitives, packed into this slide. QDP-0010, compact Merkle proofs. Today's anchor gossip ships the full origin block, which can be tens of kilobytes for dense blocks. QDP-0010 adds a TransactionsRoot field — a Merkle root over transactions — to block headers. When gossip propagates an anchor, it can ship just the leaf hash plus the sibling hashes up to the root. Log₂ of the transaction count instead of the whole block. Typical bandwidth reduction is around 70 percent, and it's the foundation for light-client support — future clients that don't need full blocks can still verify specific transactions. On the right, QDP-0007, lazy epoch propagation. Alice rotates her key in the banking domain. She rarely uses her identity in healthcare. Healthcare's nodes may not have received the gossip. When her next healthcare transaction arrives, the healthcare node sees her 'recent activity' is stale — quarantines the transaction, probes the banking domain for her current fingerprint, updates its ledger, and re-validates. Catches the 'stale epoch' cross-domain attack where someone uses a rotated-out key in a domain that hasn't learned yet.

KEY POINTS:
• QDP-0010: Merkle root in block, 70% bandwidth cut
• QDP-0007: quarantine + probe for stale-signer detection
• Both QDPs have shipped

TRANSITION:
Guardian resignation rounds out the technical primitives.""")
    p += 1

    # ---------- 40. Guardian resignation ----------
    s = slide_content(prs, "Guardian resignation", kicker="PRIMITIVE 10 (QDP-0006)", page=p, total=total)
    # Timeline with the key property
    rounded(s, Inches(0.6), Inches(1.85), Inches(12.1), Inches(2.3), ICE, corner=0.05)
    text_box(s, Inches(0.85), Inches(2.0), Inches(11.6), Inches(0.4),
             "A GUARDIAN WITHDRAWS WITHOUT THE SUBJECT'S COOPERATION",
             font=B_FONT, size=11, color=SLATE, bold=True)
    # Timeline
    y = Inches(2.65)
    steps = [
        ("Guardian signs resignation", TEAL, "w/ GuardianSetHash + nonce"),
        ("EffectiveAt date reached", AMBER, "typically 7 days out"),
        ("Guardian weight drops to 0", NAVY, "for future recoveries only"),
        ("Subject can reshape quorum", GREEN, "via GuardianSetUpdate"),
    ]
    for i, (h, col, sub) in enumerate(steps):
        x = Inches(0.85 + i * 3.0)
        numbered_circle(s, x, y, 0.55, i + 1, fill=col, size=18)
        text_box(s, x + Inches(0.65), y, Inches(2.4), Inches(0.5),
                 h, font=H_FONT, size=12, color=MIDNIGHT, bold=True,
                 anchor=MSO_ANCHOR.MIDDLE)
        text_box(s, x, y + Inches(0.65), Inches(2.9), Inches(0.6),
                 sub, font=B_FONT, size=10, color=SLATE, italic=True,
                 line_spacing=1.2)
        if i < len(steps) - 1:
            arrow(s, x + Inches(2.8), y + Inches(0.28),
                  x + Inches(3.0), y + Inches(0.28), color=SLATE, width=1.5)

    # Prospective-only caveat
    rounded(s, Inches(0.6), Inches(4.4), Inches(12.1), Inches(2.5), MIDNIGHT, corner=0.05)
    text_box(s, Inches(0.85), Inches(4.55), Inches(11.6), Inches(0.45),
             "KEY INVARIANT — PROSPECTIVE ONLY",
             font=B_FONT, size=11, color=TEAL, bold=True)
    text_box(s, Inches(0.85), Inches(5.0), Inches(11.6), Inches(1.2),
             "A resignation does NOT retroactively invalidate in-flight recoveries.\n\nIf Alice's guardians Init'd a recovery yesterday and Bob resigns today, the recovery's authorization — computed against the set AS IT WAS at Init time — still counts. Commit proceeds. Bob's resignation affects FUTURE recoveries only.",
             font=B_FONT, size=13, color=CLOUD, line_spacing=1.35)
    text_box(s, Inches(0.85), Inches(6.3), Inches(11.6), Inches(0.5),
             "Why: retroactive invalidation would turn resignation into a veto against specific pending recoveries — out-of-band coordination attack surface.",
             font=B_FONT, size=11, color=AMBER, italic=True, line_spacing=1.3)
    notes(s, """TIMING: 1.5 minutes.

WHAT TO SAY:
Finally: guardian resignation. QDP-0006. A guardian should be able to withdraw from a subject's quorum without requiring the subject's cooperation — because the subject might be unreachable, or the guardian might have a legitimate reason to step away that the subject can't block. The resignation transaction is signed by the guardian, carries a GuardianSetHash (which set version they're resigning from), a monotonic nonce, and an effective-at date — typically 7 days in the future. At effective-at, the guardian's weight drops to zero for FUTURE recoveries. Here's the subtle part, emphasized at the bottom: resignation is prospective only. It does NOT retroactively invalidate an in-flight recovery. If a recovery was already Init'd yesterday, the authorization is computed against the set as it was at Init time, and the Commit proceeds normally. Why? If resignation could retroactively kill pending recoveries, it would become a veto mechanism — and that opens an out-of-band coordination attack surface. Prospective-only eliminates that.

KEY POINTS:
• Guardian can withdraw without subject
• 7-day effective-at delay
• PROSPECTIVE ONLY — no retroactive invalidation
• Subject reshapes quorum via GuardianSetUpdate

TRANSITION:
That's ten primitives. How do they compose into a protocol?""")
    p += 1

    return p - start_page  # number of slides added (absolute count handled in runner)
