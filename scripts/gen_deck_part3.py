"""Slides 42–63: Protocol evolution, Architecture, Use cases intro, FinTech, AI."""
from pptx.util import Inches, Pt
from pptx.enum.text import PP_ALIGN, MSO_ANCHOR
from gen_deck_core import (
    slide_section, slide_content, slide_dark,
    set_bg, text_box, bullets, rect, rounded, hexagon, oval, shape_text,
    arrow, notes, card, chip, numbered_circle, table_rows,
    MIDNIGHT, NAVY, TEAL, AMBER, ICE, CLOUD, SLATE, WHITE, GREEN, RED,
    SOFT_TEAL, SOFT_AMBER, H_FONT, B_FONT, CODE_FONT
)


def build_part3(prs, start_page, total):
    p = start_page

    # ---------- SECTION 5 — Protocol evolution + Architecture ----------
    slide_section(prs, 5, "PART FIVE", "Protocol evolution")
    p += 1

    # ---------- QDP process ----------
    s = slide_content(prs, "The QDP process — designs before code", kicker="HOW WE SHIP CHANGES", page=p, total=total)
    # Flow diagram
    rounded(s, Inches(0.6), Inches(1.85), Inches(12.1), Inches(3), ICE, corner=0.05)
    text_box(s, Inches(0.85), Inches(2.0), Inches(11.6), Inches(0.4),
             "QUIDNUG DESIGN PROPOSAL (QDP) LIFECYCLE",
             font=B_FONT, size=11, color=SLATE, bold=True)
    steps = [
        ("Problem", "Observed pain or gap", NAVY),
        ("Design doc", "Full QDP before code", TEAL),
        ("Review", "Security + architecture", AMBER),
        ("Implementation", "Behind feature flag", TEAL),
        ("Shadow period", "Observability + canary", NAVY),
        ("Default-on", "Activated via fork-block", GREEN),
    ]
    for i, (h, sub, col) in enumerate(steps):
        x = Inches(0.85 + i * 2.0)
        y = Inches(2.55)
        rounded(s, x, y, Inches(1.9), Inches(1.7), col, corner=0.08)
        numbered_circle(s, x + Inches(0.75), y + Inches(0.15),
                        0.4, i + 1, fill=WHITE, color=MIDNIGHT, size=14)
        text_box(s, x, y + Inches(0.6), Inches(1.9), Inches(0.45),
                 h, font=H_FONT, size=14,
                 color=(MIDNIGHT if col == AMBER else WHITE), bold=True,
                 align=PP_ALIGN.CENTER, anchor=MSO_ANCHOR.MIDDLE)
        text_box(s, x + Inches(0.05), y + Inches(1.05), Inches(1.8), Inches(0.6),
                 sub, font=B_FONT, size=10.5,
                 color=(MIDNIGHT if col == AMBER else CLOUD),
                 italic=True, align=PP_ALIGN.CENTER, line_spacing=1.2)
        if i < len(steps) - 1:
            arrow(s, x + Inches(1.9), y + Inches(0.85),
                  x + Inches(2.0), y + Inches(0.85), color=SLATE, width=1.5)

    # Benefits
    rounded(s, Inches(0.6), Inches(5.05), Inches(12.1), Inches(1.9), MIDNIGHT, corner=0.05)
    benefits = [
        ("Thinking before typing", "10 QDPs, ~25K words of design before any line of code"),
        ("Shadow → enforce rollout", "Every change lives in shadow mode before becoming default"),
        ("Public design review", "Every QDP is in the repo before implementation"),
        ("Backwards compatibility", "Fork-block triggers give clean upgrade paths"),
    ]
    for i, (h, b) in enumerate(benefits):
        x = Inches(0.85 + (i % 2) * 6)
        y = Inches(5.2 + (i // 2) * 0.85)
        oval(s, x, y + Inches(0.1), Inches(0.3), Inches(0.3), TEAL)
        text_box(s, x + Inches(0.4), y, Inches(5.4), Inches(0.35),
                 h, font=B_FONT, size=12, color=WHITE, bold=True)
        text_box(s, x + Inches(0.4), y + Inches(0.32), Inches(5.4), Inches(0.4),
                 b, font=B_FONT, size=11, color=CLOUD, italic=True)
    notes(s, """TIMING: 1.5 minutes.

WHAT TO SAY:
Every protocol change in Quidnug goes through this process. We don't jump to code — we start with a full design proposal, a QDP. Current count: ten landed, every one was specified in a multi-thousand-word design doc before implementation. The six steps: identify a problem or gap. Write a full design proposal — problem, threat model, schema, flows, alternatives. Review for security and architecture. Implement behind a feature flag. Run a shadow period where metrics flow but behavior is unchanged. Flip the flag — and for protocol-breaking changes, do it via a fork-block transaction. The result: ten coherent, well-designed features with minimal patch-on-patch debt. You can read any QDP in the repo, cover-to-cover, and understand the decisions that went into it.

KEY POINTS:
• Design before code (always)
• Shadow → enforce rollout
• Fork-block for protocol upgrades
• Public QDPs in the repo

TRANSITION:
Here's what's landed.""")
    p += 1

    # ---------- QDPs landed ----------
    s = slide_content(prs, "10 QDPs landed, more coming", kicker="SHIPPED", page=p, total=total)
    # Table of QDPs
    rows = [
        ["#", "Title", "What it enables", "Status"],
        ["0001", "Global Nonce Ledger", "Strong replay protection per signer", "Landed"],
        ["0002", "Guardian-Based Recovery", "M-of-N key recovery with time-lock veto", "Landed"],
        ["0003", "Cross-Domain Nonce Scoping", "Rotations propagate between domains", "Landed"],
        ["0004", "Phase H Roadmap", "Residual protocol work plan", "Landed"],
        ["0005", "Push-Based Gossip", "Real-time propagation (seconds vs. minutes)", "Landed"],
        ["0006", "Guardian Resignation", "Guardian withdraws without subject", "Landed"],
        ["0007", "Lazy Epoch Propagation", "Stale-signer detection across domains", "Landed"],
        ["0008", "K-of-K Bootstrap", "Secure cold-start node join", "Landed"],
        ["0009", "Fork-Block Trigger", "On-chain coordinated upgrades", "Landed"],
        ["0010", "Compact Merkle Proofs", "~70% less gossip bandwidth", "Landed"],
    ]
    col_widths = [Inches(0.75), Inches(3.4), Inches(5.9), Inches(2)]
    table_rows(s, Inches(0.6), Inches(1.85), col_widths, rows,
               header_fill=NAVY, header_size=12, body_size=11,
               row_h=Inches(0.42), header_h=Inches(0.45))
    # Bottom status box
    rounded(s, Inches(0.6), Inches(6.7), Inches(12.1), Inches(0.35),
            SOFT_TEAL, corner=0.05)
    text_box(s, Inches(0.85), Inches(6.7), Inches(11.6), Inches(0.35),
             "All 10 QDPs shipped with comprehensive test coverage. Full documentation at /docs/design/ in the repo.",
             font=B_FONT, size=11, color=MIDNIGHT, italic=True,
             anchor=MSO_ANCHOR.MIDDLE)
    notes(s, """TIMING: 1.5 minutes.

WHAT TO SAY:
Here's the scoreboard. Ten QDPs landed. Each has a comprehensive design doc in the repo — 2000-5000 words covering problem, threat model, data model, protocol flows, migration plan, and test plan. The ones we've already seen in depth: 0001 nonce ledger, 0002 guardian recovery, 0005 push gossip, 0007 lazy epoch propagation, 0008 K-of-K bootstrap, 0009 fork-block, 0010 compact Merkle proofs. 0003 cross-domain scoping, 0004 roadmap for Phase H, 0006 guardian resignation rounded it out. Test coverage is comprehensive — the codebase has ~90 new tests just from the H-phase work. The protocol has settled enough that we're in 'use cases and deployment' territory now, not 'what's the core protocol' territory. Future QDPs will likely cover things like homomorphic voting for stronger receipt-freeness, permissioned sub-chains for regulated domains, and formal cross-chain interop.

KEY POINTS:
• 10 landed, full design docs per feature
• Comprehensive test coverage
• Core protocol settled
• Future work: homomorphic voting, sub-chains, interop

TRANSITION:
How does all this compose into an actual node?""")
    p += 1

    # ---------- Node architecture ----------
    s = slide_content(prs, "Node architecture at a glance", kicker="WHAT'S INSIDE", page=p, total=total)
    # Big block diagram
    rounded(s, Inches(0.6), Inches(1.85), Inches(12.1), Inches(4.8),
            ICE, corner=0.05)
    text_box(s, Inches(0.85), Inches(2.0), Inches(11.6), Inches(0.4),
             "QUIDNUGNODE (single Go binary)",
             font=B_FONT, size=11, color=SLATE, bold=True)

    # HTTP API (top)
    rounded(s, Inches(0.85), Inches(2.5), Inches(11.6), Inches(0.7),
            NAVY, corner=0.05)
    text_box(s, Inches(0.85), Inches(2.5), Inches(11.6), Inches(0.7),
             "HTTP REST API  (/api/v1 + /api/v2)",
             font=H_FONT, size=14, color=WHITE, bold=True,
             align=PP_ALIGN.CENTER, anchor=MSO_ANCHOR.MIDDLE)

    # 6 engines (2 rows of 3)
    engines = [
        ("Trust Engine", "BFS pathfinding", TEAL),
        ("Nonce Ledger", "QDP-0001", NAVY),
        ("Guardian Registry", "QDP-0002 + 0006", AMBER),
        ("Block Engine", "Proof-of-Trust tiers", TEAL),
        ("Push Gossip", "QDP-0005", NAVY),
        ("Bootstrap + Forks", "QDP-0008 + 0009 + 0010", AMBER),
    ]
    for i, (name, sub, col) in enumerate(engines):
        x = Inches(0.85 + (i % 3) * 3.87)
        y = Inches(3.35 + (i // 3) * 1.1)
        rounded(s, x, y, Inches(3.77), Inches(0.95), col, corner=0.05)
        text_box(s, x + Inches(0.15), y + Inches(0.12), Inches(3.47), Inches(0.4),
                 name, font=H_FONT, size=14,
                 color=(MIDNIGHT if col == AMBER else WHITE), bold=True)
        text_box(s, x + Inches(0.15), y + Inches(0.5), Inches(3.47), Inches(0.4),
                 sub, font=B_FONT, size=11,
                 color=(MIDNIGHT if col == AMBER else CLOUD), italic=True)

    # Bottom: P2P layer
    rounded(s, Inches(0.85), Inches(5.65), Inches(11.6), Inches(0.85),
            MIDNIGHT, corner=0.05)
    text_box(s, Inches(0.85), Inches(5.65), Inches(11.6), Inches(0.4),
             "P2P NETWORK LAYER", font=H_FONT, size=12, color=TEAL,
             bold=True, align=PP_ALIGN.CENTER)
    text_box(s, Inches(0.85), Inches(6.0), Inches(11.6), Inches(0.45),
             "HTTP + signature   ·   gossip    ·    fingerprint probes    ·    snapshot pull",
             font=B_FONT, size=11, color=CLOUD, italic=True, align=PP_ALIGN.CENTER)

    # Tiny stats
    text_box(s, Inches(0.6), Inches(6.85), Inches(12.1), Inches(0.35),
             "Go 1.23+  ·  ECDSA P-256  ·  SHA-256  ·  Gorilla mux  ·  Prometheus metrics  ·  IPFS optional for large payloads",
             font=B_FONT, size=10, color=SLATE, italic=True, align=PP_ALIGN.CENTER)
    notes(s, """TIMING: 1.5 minutes.

WHAT TO SAY:
A Quidnug node is a single Go binary. HTTP REST API on top — one for the v1 stable surface, v2 for the newer features from Phase H. Six engines inside. Trust Engine does the BFS pathfinding we saw in Part 3. Nonce Ledger from QDP-0001 does replay protection. Guardian Registry from QDP-0002 and 0006 handles guardian sets, recoveries, and resignations. Block Engine implements Proof-of-Trust consensus tiering. Push Gossip is QDP-0005. Bootstrap + Forks is QDP-0008, 0009, and 0010 together. Below, a P2P network layer doing HTTP with HMAC inter-node signatures, gossip, fingerprint probes, and snapshot pulls. Tech stack is conservative: Go 1.23+, ECDSA P-256 via stdlib, SHA-256, Gorilla mux, Prometheus for metrics, IPFS optional for big payloads. No exotic cryptography, no custom consensus, no exotic storage. The protocol's complexity is in the protocol, not in the dependencies.

KEY POINTS:
• Single Go binary
• Six engines + HTTP API + P2P layer
• Conservative stack
• All standard crypto primitives

TRANSITION:
Deployment patterns.""")
    p += 1

    # ---------- Deployment patterns ----------
    s = slide_content(prs, "Deployment patterns", kicker="PICK WHAT FITS", page=p, total=total)
    # Three-column layout
    patterns = [
        ("Single node", "Development / testing",
         "One binary. In-memory state. No gossip. Great for local integration work.",
         TEAL, "DEV"),
        ("Consortium", "Banks, oracle networks, federations",
         "3–20 peer nodes. Known-set trust. Push gossip, K-of-K bootstrap, fork-block upgrades.",
         NAVY, "MOST USE CASES"),
        ("Federation", "Cross-jurisdiction / multi-org",
         "Multiple consortia cross-gossiped via domain hierarchy. Cross-border trust via reciprocity edges.",
         AMBER, "BIG SYSTEMS"),
    ]
    for i, (h, use, body, col, badge) in enumerate(patterns):
        x = Inches(0.6 + i * 4.15)
        y = Inches(1.85)
        rounded(s, x, y, Inches(4), Inches(5), ICE, corner=0.05)
        rect(s, x, y, Inches(4), Inches(0.5), col)
        text_box(s, x, y, Inches(4), Inches(0.5), h,
                 font=H_FONT, size=16,
                 color=(MIDNIGHT if col == AMBER else WHITE), bold=True,
                 align=PP_ALIGN.CENTER, anchor=MSO_ANCHOR.MIDDLE)
        text_box(s, x + Inches(0.2), y + Inches(0.65), Inches(3.6), Inches(0.35),
                 use, font=B_FONT, size=11, color=TEAL, bold=True, italic=True)
        text_box(s, x + Inches(0.2), y + Inches(1.05), Inches(3.6), Inches(2.2),
                 body, font=B_FONT, size=12, color=MIDNIGHT,
                 line_spacing=1.35)
        chip(s, x + Inches(0.2), y + Inches(3.5), Inches(3.6), Inches(0.4),
             badge, fill=col,
             color=(MIDNIGHT if col == AMBER else WHITE), size=11)
        # Key features list
        features_per = {
            0: ["No ops complexity", "In-process embed works", "Cover unit tests"],
            1: ["Push gossip on", "3-of-3 bootstrap", "Operator-coordinated forks"],
            2: ["Multi-domain gossip", "Lazy probe across regions", "Per-jurisdiction validators"],
        }
        for j, feat in enumerate(features_per[i]):
            text_box(s, x + Inches(0.2), y + Inches(4.05 + j * 0.3), Inches(3.6), Inches(0.3),
                     "•  " + feat, font=B_FONT, size=10.5, color=SLATE)
    notes(s, """TIMING: 1.5 minutes.

WHAT TO SAY:
Three deployment patterns for three different contexts. Single node is for dev and testing — run the binary locally, do your integration work, ship. Consortium is the most common pattern. 3 to 20 peer nodes, all with known identities, running push gossip and K-of-K bootstrap between them. This fits banking consortia, oracle networks, industry associations, regional elections. Most use cases in Part 6 assume consortium. Federation is the big-system pattern — multiple consortia cross-gossiped through the domain hierarchy, with reciprocity trust edges between them for cross-border operations. Think: global institutional custody spanning US + EU + APAC subsidiaries, or a cross-state elections operation. The protocol scales smoothly from single-node to federation, and you can start with consortium and grow.

KEY POINTS:
• Single node — development
• Consortium — most real use cases
• Federation — cross-jurisdiction / large scale
• Smooth progression

TRANSITION:
We've covered the protocol. Now the fun part — what do you actually build with it?""")
    p += 1

    # ---------- SECTION 6 divider ----------
    slide_section(prs, 6, "PART SIX", "14 use cases")
    p += 1

    # ---------- Use cases grid ----------
    s = slide_content(prs, "14 use cases, 5 categories", kicker="WHAT YOU CAN BUILD", page=p, total=total)
    # Big grid of categories
    categories = [
        ("FINTECH", NAVY, 5, [
            "Interbank wire authorization",
            "Merchant fraud consortium",
            "DeFi oracle network",
            "Institutional custody",
            "B2B invoice financing",
        ]),
        ("AI", TEAL, 4, [
            "AI model provenance",
            "AI agent authorization",
            "Federated learning attestation",
            "AI content authenticity",
        ]),
        ("GOVERNMENT", AMBER, 1, ["Elections"]),
        ("CONSUMER RIGHTS", GREEN, 1, ["Decentralized credit / anti-social-credit"]),
        ("CROSS-INDUSTRY", RED, 3, [
            "Healthcare consent",
            "Credential verification",
            "Developer artifact signing",
        ]),
    ]
    # Render as rows
    y = Inches(1.85)
    for i, (name, col, count, items) in enumerate(categories):
        cy = y + Inches(i * 0.95)
        rounded(s, Inches(0.6), cy, Inches(12.1), Inches(0.85),
                ICE, corner=0.05)
        rect(s, Inches(0.6), cy, Inches(0.1), Inches(0.85), col)
        # Category badge
        rounded(s, Inches(0.85), cy + Inches(0.15), Inches(2.6), Inches(0.55),
                col, corner=0.1)
        text_box(s, Inches(0.85), cy + Inches(0.15), Inches(2.6), Inches(0.55),
                 f"{name}  ({count})",
                 font=H_FONT, size=12,
                 color=(MIDNIGHT if col == AMBER else WHITE), bold=True,
                 align=PP_ALIGN.CENTER, anchor=MSO_ANCHOR.MIDDLE)
        # Items (truncated list)
        items_text = "  ·  ".join(items)
        text_box(s, Inches(3.65), cy + Inches(0.15), Inches(8.9), Inches(0.55),
                 items_text, font=B_FONT, size=11, color=MIDNIGHT,
                 anchor=MSO_ANCHOR.MIDDLE)
    # Note
    text_box(s, Inches(0.6), Inches(6.75), Inches(12.1), Inches(0.35),
             "Every use case has a full design folder in the repo: README + architecture + implementation + threat-model",
             font=B_FONT, size=11, color=SLATE, italic=True, align=PP_ALIGN.CENTER)
    notes(s, """TIMING: 1 minute.

WHAT TO SAY:
Fourteen use cases, five categories. Five in FinTech — interbank wires, merchant fraud, DeFi oracles, custody, invoice financing. Four in AI — model provenance, agent authorization, federated learning, content authenticity. One in government — elections, which is the most detailed of the fourteen. One in consumer rights — decentralized credit reporting, which deliberately replaces both credit bureaus and social-credit systems. Three cross-industry — healthcare consent, credential verification, and developer artifact signing. Every one has a full design folder in the repo with README, architecture doc, implementation doc with concrete Quidnug API calls, and threat model. I'll walk through highlights of each category in the next twenty minutes or so. Don't worry about taking notes — everything is linked from UseCases/README.md in the repo.

KEY POINTS:
• 14 use cases, 5 categories
• Each has a 4-file design folder
• All linked from UseCases/README.md

TRANSITION:
Let's start with FinTech.""")
    p += 1

    # ---------- FinTech overview ----------
    s = slide_dark(prs, "FinTech — 5 use cases", kicker="CATEGORY 1", page=p, total=total)
    # Five-card layout
    fintech = [
        ("Interbank Wire Authorization",
         "M-of-N officer cosigning for high-value wires with replay protection + HSM-loss recovery.",
         "Guardian M-of-N  ·  Nonce ledger  ·  Recovery"),
        ("Merchant Fraud Consortium",
         "Cross-merchant fraud signal sharing weighted by relational trust — no central operator.",
         "Relational trust  ·  Push gossip  ·  Reputation"),
        ("DeFi Oracle Network",
         "Price/data feeds where each consumer computes its own trust-weighted aggregation.",
         "Signed feeds  ·  K-of-K bootstrap  ·  Consumer choice"),
        ("Institutional Custody",
         "Full key lifecycle for billion-dollar crypto positions across subsidiaries.",
         "Full anchor lifecycle  ·  Lazy probe  ·  Guardian recovery"),
        ("B2B Invoice Financing",
         "Multi-party invoice lifecycle (ship, acknowledge, factor) with double-factor prevention.",
         "Titles  ·  Event streams  ·  Domain validators"),
    ]
    for i, (name, desc, feats) in enumerate(fintech):
        # First row of 3, second of 2 centered
        if i < 3:
            x = Inches(0.6 + i * 4.15)
            y = Inches(1.85)
        else:
            x = Inches(2.65 + (i - 3) * 4.15)
            y = Inches(4.5)
        rounded(s, x, y, Inches(4), Inches(2.45), NAVY, corner=0.05)
        rect(s, x, y, Inches(4), Inches(0.5), TEAL)
        text_box(s, x, y, Inches(4), Inches(0.5),
                 name, font=H_FONT, size=12.5, color=MIDNIGHT, bold=True,
                 align=PP_ALIGN.CENTER, anchor=MSO_ANCHOR.MIDDLE)
        text_box(s, x + Inches(0.2), y + Inches(0.6), Inches(3.6), Inches(1.2),
                 desc, font=B_FONT, size=11.5, color=WHITE,
                 line_spacing=1.3)
        chip(s, x + Inches(0.2), y + Inches(1.95), Inches(3.6), Inches(0.35),
             feats, fill=TEAL, color=MIDNIGHT, size=9.5, bold=True)
    notes(s, """TIMING: 1 minute.

WHAT TO SAY:
Five FinTech use cases. I'll do a slide each on the next five. The theme across all five: multi-party approval, replay protection, key lifecycle, and cross-organizational trust. These are all things Quidnug's primitives provide natively. In most cases you're replacing a combination of internal workflow tools, proprietary approval software, and third-party compliance infrastructure with a single protocol-native approach.

TRANSITION:
Interbank wires first.""")
    p += 1

    # ---------- Interbank wire ----------
    s = slide_content(prs, "Interbank wire authorization", kicker="FINTECH — USE CASE 1", page=p, total=total)
    # Left: problem
    rounded(s, Inches(0.6), Inches(1.85), Inches(6), Inches(2.5), ICE, corner=0.05)
    text_box(s, Inches(0.85), Inches(2.0), Inches(5.5), Inches(0.4),
             "PROBLEM", font=B_FONT, size=11, color=RED, bold=True)
    text_box(s, Inches(0.85), Inches(2.4), Inches(5.5), Inches(1.8),
             "$50M wire needs 2 officers + compliance to cosign. Current flow: spreadsheet + scanned PDFs + emailed tickets.\n\nWhen a signer's HSM dies at 3am, it's an emergency vendor call.",
             font=B_FONT, size=12, color=MIDNIGHT, line_spacing=1.35)

    # Right: Quidnug solution
    rounded(s, Inches(6.7), Inches(1.85), Inches(6), Inches(2.5), MIDNIGHT, corner=0.05)
    text_box(s, Inches(6.95), Inches(2.0), Inches(5.5), Inches(0.4),
             "QUIDNUG", font=B_FONT, size=11, color=GREEN, bold=True)
    text_box(s, Inches(6.95), Inches(2.4), Inches(5.5), Inches(1.8),
             "Bank is a quid with a GuardianSet: {Alice(w=1), Bob(w=1), Carol-compliance(w=2)}, threshold=3.\n\nWire = title + cosign events. Replay-safe. Lost HSM = guardian recovery in 1h window.",
             font=B_FONT, size=12, color=CLOUD, line_spacing=1.35)

    # Bottom: the sequence
    rounded(s, Inches(0.6), Inches(4.55), Inches(12.1), Inches(2.4), ICE, corner=0.05)
    text_box(s, Inches(0.85), Inches(4.7), Inches(11.6), Inches(0.4),
             "WIRE-APPROVAL SEQUENCE", font=B_FONT, size=11, color=SLATE, bold=True)
    seq = [
        ("Core banking\nissues wire", TEAL),
        ("Title created\n(Alice cosigns)", NAVY),
        ("Bob cosigns\nvia event", NAVY),
        ("Carol compliance\ncosigns (w=2)", AMBER),
        ("Quorum ≥ 3\n→ auto-approve", GREEN),
        ("Fedwire / CHIPS\nexecutes", TEAL),
    ]
    for i, (h, col) in enumerate(seq):
        x = Inches(0.85 + i * 2.0)
        y = Inches(5.2)
        rounded(s, x, y, Inches(1.9), Inches(1.15), col, corner=0.1)
        text_box(s, x, y, Inches(1.9), Inches(1.15), h,
                 font=B_FONT, size=10.5,
                 color=(MIDNIGHT if col == AMBER else WHITE), bold=True,
                 align=PP_ALIGN.CENTER, anchor=MSO_ANCHOR.MIDDLE,
                 line_spacing=1.2)
        if i < len(seq) - 1:
            arrow(s, x + Inches(1.9), y + Inches(0.57),
                  x + Inches(2.0), y + Inches(0.57), color=SLATE, width=1.25)
    notes(s, """TIMING: 2 minutes.

WHAT TO SAY:
Your first FinTech use case: interbank wire authorization. Policy: wires above $10M need two officers plus compliance, weighted at 3 total. Today, that's a spreadsheet, some scanned PDFs, and emailed tickets. Audit trail is scattered. Recovery when an HSM dies is an emergency vendor call at 3am. With Quidnug: the bank is a quid with a guardian set — Alice weight 1, Bob weight 1, Carol-compliance weight 2, threshold 3. Every wire is a title. Officers cosign via signed events. When weights meet threshold, the system auto-emits an approval event and core banking pushes to Fedwire or CHIPS. Replay-safe by construction — every signature has a monotonic anchor nonce. When Alice's HSM dies, her personal guardians initiate recovery — a spouse, her backup HSM, maybe a manager — with a 1-hour time-lock for operator oversight. Everything is cryptographically auditable. Single-query audit for any wire replaces the current 4-system join.

KEY POINTS:
• Bank = quid with guardian set
• Wire = title + cosign events
• Weighted threshold (w=1 + w=1 + w=2 compliance)
• Guardian recovery for HSM failures

TRANSITION:
Use case 2 — consortium fraud detection.""")
    p += 1

    # ---------- Merchant fraud ----------
    s = slide_content(prs, "Merchant fraud consortium", kicker="FINTECH — USE CASE 2", page=p, total=total)
    # Network diagram: 5 merchants + shared domain
    rounded(s, Inches(0.6), Inches(1.85), Inches(12.1), Inches(3.1), ICE, corner=0.05)
    text_box(s, Inches(0.85), Inches(2.0), Inches(11.6), Inches(0.4),
             "5 MERCHANTS GOSSIP FRAUD SIGNALS — EACH WEIGHS THEM PER THEIR OWN TRUST GRAPH",
             font=B_FONT, size=11, color=SLATE, bold=True)
    # 5 merchant nodes in a ring.
    # CRITICAL: draw all gossip lines FIRST, then draw ovals on top so
    # the ovals cover the line-pass-through at node centers.
    import math
    merchants = ["Acme", "BigBox", "Fin-Tech", "Bank.com", "Newcomer"]
    merch_colors = [TEAL, NAVY, TEAL, NAVY, AMBER]
    # Container spans y=1.85 to y=4.95. Fit the entire ring (oval-centers
    # + half-oval) inside that: center_y = 3.4, radius = 1.1, oval_h = 0.7
    # → top 2.3 - 0.35 = 1.95, bottom 4.5 + 0.35 = 4.85. Safe.
    center_x = 6.65
    center_y = 3.4
    radius = 1.1
    oval_w, oval_h = 1.15, 0.68
    # Compute positions (center of each oval)
    centers = []
    for i in range(len(merchants)):
        angle = -math.pi/2 + i * (2 * math.pi / 5)
        cx = center_x + radius * math.cos(angle)
        cy = center_y + radius * math.sin(angle)
        centers.append((cx, cy))
    # Draw gossip edges among all pairs — behind ovals
    for i in range(len(centers)):
        for j in range(i + 1, len(centers)):
            cx1, cy1 = centers[i]
            cx2, cy2 = centers[j]
            arrow(s, Inches(cx1), Inches(cy1),
                  Inches(cx2), Inches(cy2),
                  color=CLOUD, width=0.6, end_arrow=False)
    # Now ovals + labels on top (covers line pass-through)
    for i, (lab, col) in enumerate(zip(merchants, merch_colors)):
        cx, cy = centers[i]
        mx = cx - oval_w / 2
        my = cy - oval_h / 2
        oval(s, Inches(mx), Inches(my), Inches(oval_w), Inches(oval_h), col)
        text_box(s, Inches(mx), Inches(my), Inches(oval_w), Inches(oval_h), lab,
                 font=B_FONT, size=10, color=(MIDNIGHT if col == AMBER else WHITE),
                 bold=True, align=PP_ALIGN.CENTER, anchor=MSO_ANCHOR.MIDDLE)

    # Bottom: key properties
    rounded(s, Inches(0.6), Inches(5.15), Inches(12.1), Inches(1.8), MIDNIGHT, corner=0.05)
    text_box(s, Inches(0.85), Inches(5.3), Inches(11.6), Inches(0.45),
             "WHY THIS WORKS WHERE 'SHARED FRAUD DATABASE' FAILED",
             font=B_FONT, size=11, color=TEAL, bold=True)
    props = [
        ("No central operator", "Each merchant runs their own node"),
        ("Relational trust", "Acme's signal has different weight to each peer"),
        ("Compromised reporter = auto-contain", "Others just lower their trust in that peer"),
        ("Counter-signals", "False positives are rebuttable publicly"),
    ]
    for i, (h, b) in enumerate(props):
        x = Inches(0.85 + (i % 2) * 6)
        y = Inches(5.8 + (i // 2) * 0.55)
        oval(s, x, y + Inches(0.1), Inches(0.3), Inches(0.3), TEAL)
        text_box(s, x + Inches(0.4), y, Inches(5.4), Inches(0.35),
                 h, font=B_FONT, size=12, color=WHITE, bold=True)
        text_box(s, x + Inches(0.4), y + Inches(0.32), Inches(5.4), Inches(0.35),
                 b, font=B_FONT, size=11, color=CLOUD, italic=True)
    notes(s, """TIMING: 2 minutes.

WHAT TO SAY:
Use case 2. Merchants and payment processors see billions of fraud signals collectively. They'd all benefit from sharing — a card-testing pattern hitting Merchant A today will hit Merchant B tomorrow. But every attempt at a shared fraud database founders on: 'if I share, my competitors get value' plus 'if I share bad data, I poison everyone.' Quidnug reframes this. Every merchant runs their own node. Fraud signals are events emitted to a shared domain. Each receiving merchant computes relational trust in the reporting merchant — so Acme's signals get different weight at each peer. A compromised reporter gets auto-contained because other peers lower their trust in that peer when signals are bad. False positives are rebuttable publicly via counter-signals. No central operator. No 'trust the consortium software.' Each merchant makes their own call, informed by the signed, propagated signals from peers, weighted by their own per-peer trust. This is the relational-trust premise applied to cross-organization fraud detection.

KEY POINTS:
• No central operator
• Each merchant computes own trust in each peer
• Compromised reporters self-contain
• Public counter-signal mechanism

TRANSITION:
Third FinTech use case — DeFi oracles.""")
    p += 1

    # ---------- DeFi oracle ----------
    s = slide_content(prs, "DeFi oracle network", kicker="FINTECH — USE CASE 3", page=p, total=total)
    # Left: reporters
    rounded(s, Inches(0.6), Inches(1.85), Inches(5.2), Inches(5), ICE, corner=0.05)
    text_box(s, Inches(0.85), Inches(2.0), Inches(4.8), Inches(0.4),
             "4 REPORTERS", font=B_FONT, size=11, color=SLATE, bold=True)
    reporters = [
        ("Oracle-Chainlink-geth", "Aggregates: Binance, Coinbase", "$63,425.20", TEAL),
        ("Oracle-Pyth-eth", "Aggregates: Kraken, Bybit", "$63,418.50", TEAL),
        ("Oracle-Exchange-A", "Single-source: Binance", "$63,399.00", AMBER),
        ("Oracle-Synth-agg", "Cross-chain aggregate", "$63,445.10", TEAL),
    ]
    for i, (name, source, price, col) in enumerate(reporters):
        y = Inches(2.45 + i * 1.05)
        rounded(s, Inches(0.85), y, Inches(4.75), Inches(0.95), WHITE,
                line=CLOUD, corner=0.05)
        rect(s, Inches(0.85), y, Inches(0.07), Inches(0.95), col)
        text_box(s, Inches(1.0), y + Inches(0.1), Inches(3), Inches(0.35),
                 name, font=CODE_FONT, size=10.5, color=MIDNIGHT, bold=True)
        text_box(s, Inches(1.0), y + Inches(0.42), Inches(3), Inches(0.35),
                 source, font=B_FONT, size=9, color=SLATE, italic=True)
        text_box(s, Inches(4.2), y + Inches(0.25), Inches(1.3), Inches(0.45),
                 price, font=CODE_FONT, size=14, color=col, bold=True,
                 align=PP_ALIGN.RIGHT, anchor=MSO_ANCHOR.MIDDLE)

    # Right: two consumers computing different aggregates
    rounded(s, Inches(6.0), Inches(1.85), Inches(6.7), Inches(5),
            MIDNIGHT, corner=0.05)
    text_box(s, Inches(6.2), Inches(2.0), Inches(6.3), Inches(0.45),
             "TWO CONSUMERS, DIFFERENT AGGREGATES",
             font=B_FONT, size=11, color=TEAL, bold=True)

    # Consumer 1
    rounded(s, Inches(6.2), Inches(2.55), Inches(6.3), Inches(1.9),
            ICE, corner=0.05)
    text_box(s, Inches(6.4), Inches(2.65), Inches(5.9), Inches(0.35),
             "CONSUMER A — Conservative lender (trust ≥ 0.8)",
             font=B_FONT, size=11, color=NAVY, bold=True)
    text_box(s, Inches(6.4), Inches(3.0), Inches(5.9), Inches(1.3),
             "Weights only first 2 reporters (trusted 0.95/0.90).\nMedian: $63,421.85\n→ Approves loan at rate based on this price.",
             font=B_FONT, size=11, color=MIDNIGHT, line_spacing=1.35)

    # Consumer 2
    rounded(s, Inches(6.2), Inches(4.6), Inches(6.3), Inches(1.9),
            ICE, corner=0.05)
    text_box(s, Inches(6.4), Inches(4.7), Inches(5.9), Inches(0.35),
             "CONSUMER B — Progressive fintech (trust ≥ 0.5)",
             font=B_FONT, size=11, color=TEAL, bold=True)
    text_box(s, Inches(6.4), Inches(5.05), Inches(5.9), Inches(1.3),
             "Weights all 4, including single-source. Weighted median: $63,421.50\n→ Approves with slightly different rate.",
             font=B_FONT, size=11, color=MIDNIGHT, line_spacing=1.35)

    text_box(s, Inches(0.6), Inches(6.95), Inches(12.1), Inches(0.3),
             "Same feeds, two consumers, two answers — both correct for their risk tolerance.",
             font=B_FONT, size=11, color=SLATE, italic=True, align=PP_ALIGN.CENTER)
    notes(s, """TIMING: 1.5 minutes.

WHAT TO SAY:
DeFi oracles. Current landscape: Chainlink, Pyth, API3 each make specific trade-offs. All produce essentially one 'true' price. Quidnug flips this: each reporter publishes signed price events. Consumers compute their own weighted aggregate based on their own trust in each reporter. Here: four reporters publishing BTC-USD prices. Consumer A is a conservative lender — only accepts reporters with trust ≥ 0.8, weights the top two reporters, gets $63,421.85. Consumer B is a progressive fintech — accepts trust ≥ 0.5, weights all four including the single-source one, gets $63,421.50. Two consumers, two different numbers, both mathematically correct given their own trust graphs. A compromised reporter gets auto-deprioritized by consumers who observe the manipulation. There's no 'kill switch' needed — reputation just erodes.

KEY POINTS:
• Reporters publish signed feeds
• Consumers compute own weighted aggregate
• Different consumers get different numbers
• Compromised reporters self-deprioritize

TRANSITION:
Fourth — institutional custody.""")
    p += 1

    # ---------- Institutional custody ----------
    s = slide_content(prs, "Institutional crypto custody", kicker="FINTECH — USE CASE 4", page=p, total=total)
    # Top: scale
    rounded(s, Inches(0.6), Inches(1.85), Inches(12.1), Inches(1.5), MIDNIGHT, corner=0.05)
    text_box(s, Inches(0.85), Inches(2.0), Inches(6), Inches(1.35),
             "$5B+", font=H_FONT, size=72, color=TEAL, bold=True,
             anchor=MSO_ANCHOR.MIDDLE)
    text_box(s, Inches(5.5), Inches(2.0), Inches(7), Inches(1.35),
             "Typical custody firm holdings\n\nQuarterly key rotation policy, ~50 signers across US + EU + APAC subsidiaries, multi-chain",
             font=B_FONT, size=13, color=CLOUD, line_spacing=1.35,
             anchor=MSO_ANCHOR.MIDDLE)

    # Three challenges with solutions
    challenges = [
        ("Quarterly rotation",
         "Spreadsheet + manual ceremony",
         "AnchorRotation with MaxAcceptedOldNonce — bounded grace window"),
        ("Emergency HSM failure",
         "Call vendor, hope for the best",
         "Guardian-recovery via personal quorum, ≤7d time-lock"),
        ("Cross-subsidiary transfer",
         "Separate chains + email coordination",
         "EU + US both-quorum-cosign on a single title"),
        ("Audit a specific transfer",
         "4-system join over 2 weeks",
         "Single chain query — who signed, which epoch, when"),
    ]
    for i, (name, old, new) in enumerate(challenges):
        y = Inches(3.55 + i * 0.83)
        rounded(s, Inches(0.6), y, Inches(12.1), Inches(0.75), ICE, corner=0.04)
        rect(s, Inches(0.6), y, Inches(0.08), Inches(0.75), NAVY)
        text_box(s, Inches(0.85), y + Inches(0.1), Inches(2.5), Inches(0.55),
                 name, font=H_FONT, size=12.5, color=MIDNIGHT, bold=True,
                 anchor=MSO_ANCHOR.MIDDLE)
        text_box(s, Inches(3.45), y + Inches(0.08), Inches(4.4), Inches(0.6),
                 "BEFORE: " + old, font=B_FONT, size=10.5, color=RED,
                 anchor=MSO_ANCHOR.MIDDLE, italic=True)
        text_box(s, Inches(7.9), y + Inches(0.08), Inches(4.7), Inches(0.6),
                 "AFTER: " + new, font=B_FONT, size=10.5, color=GREEN,
                 bold=True, anchor=MSO_ANCHOR.MIDDLE)
    notes(s, """TIMING: 1.5 minutes.

WHAT TO SAY:
Institutional custody. Think a crypto custody firm holding five billion or more, across multiple chains, across US / EU / APAC subsidiaries, with quarterly rotation policies and about fifty signers total. Today: every part of the key lifecycle is ad-hoc. Quarterly rotation is a spreadsheet-plus-ceremony. HSM failures are vendor calls. Cross-subsidiary transfers are separate workflows coordinated by email. Auditing a specific transfer later is a 4-system join that takes two weeks. Quidnug gives you all of these natively. AnchorRotation with MaxAcceptedOldNonce for smooth rotations with grace windows. Guardian-recovery for emergency. Both-quorum-cosign for cross-subsidiary transfers. Single chain query for audit. For a firm at this scale, the ROI on Quidnug is just the audit cost savings alone.

KEY POINTS:
• $5B+ scale is common
• Four pain points each get a protocol-level fix
• Anchors, guardians, events, titles all compose here
• Audit cost savings alone justify adoption

TRANSITION:
Last FinTech — invoice financing.""")
    p += 1

    # ---------- B2B invoice ----------
    s = slide_content(prs, "B2B invoice financing", kicker="FINTECH — USE CASE 5", page=p, total=total)
    # Horizontal flow
    rounded(s, Inches(0.6), Inches(1.85), Inches(12.1), Inches(3), ICE, corner=0.05)
    text_box(s, Inches(0.85), Inches(2.0), Inches(11.6), Inches(0.4),
             "INVOICE LIFECYCLE ON QUIDNUG — EACH EVENT SIGNED BY ITS ACTOR",
             font=B_FONT, size=11, color=SLATE, bold=True)
    flow = [
        ("Supplier\nissues invoice", "TITLE", TEAL),
        ("Carrier\nshipped", "EVENT", NAVY),
        ("Carrier\ndelivered", "EVENT", NAVY),
        ("Buyer\nacknowledged", "EVENT", AMBER),
        ("Financier\nfactored", "EVENT", GREEN),
        ("Buyer paid\nfinancier", "EVENT", GREEN),
    ]
    for i, (h, kind, col) in enumerate(flow):
        x = Inches(0.85 + i * 2.0)
        y = Inches(2.55)
        rounded(s, x, y, Inches(1.9), Inches(1.6), col, corner=0.08)
        text_box(s, x, y + Inches(0.2), Inches(1.9), Inches(0.7), h,
                 font=B_FONT, size=11,
                 color=(MIDNIGHT if col == AMBER else WHITE), bold=True,
                 align=PP_ALIGN.CENTER, anchor=MSO_ANCHOR.MIDDLE,
                 line_spacing=1.2)
        # Kind chip
        rounded(s, x + Inches(0.4), y + Inches(1.1), Inches(1.1), Inches(0.35),
                WHITE, corner=0.3)
        text_box(s, x + Inches(0.4), y + Inches(1.1), Inches(1.1), Inches(0.35),
                 kind, font=CODE_FONT, size=9, color=MIDNIGHT, bold=True,
                 align=PP_ALIGN.CENTER, anchor=MSO_ANCHOR.MIDDLE)
        if i < len(flow) - 1:
            arrow(s, x + Inches(1.9), y + Inches(0.8),
                  x + Inches(2.0), y + Inches(0.8), color=SLATE, width=1.5)

    # Benefits
    rounded(s, Inches(0.6), Inches(5.05), Inches(12.1), Inches(1.9), MIDNIGHT, corner=0.05)
    text_box(s, Inches(0.85), Inches(5.2), Inches(11.6), Inches(0.45),
             "WHAT YOU GET",
             font=H_FONT, size=13, color=TEAL, bold=True)
    benefits = [
        ("Double-factor prevention", "A second financier sees the first's factored event"),
        ("Delivery proof", "Rated carrier's signature carries weight"),
        ("Instant financier evaluation", "Query supplier + buyer trust in seconds"),
        ("Dispute evidence", "Full signed chain beats contested paperwork"),
    ]
    for i, (h, b) in enumerate(benefits):
        x = Inches(0.85 + (i % 2) * 6)
        y = Inches(5.75 + (i // 2) * 0.6)
        oval(s, x, y + Inches(0.1), Inches(0.3), Inches(0.3), TEAL)
        text_box(s, x + Inches(0.4), y, Inches(5.4), Inches(0.35),
                 h, font=B_FONT, size=12, color=WHITE, bold=True)
        text_box(s, x + Inches(0.4), y + Inches(0.32), Inches(5.4), Inches(0.35),
                 b, font=B_FONT, size=11, color=CLOUD, italic=True)
    notes(s, """TIMING: 1.5 minutes.

WHAT TO SAY:
B2B invoice financing. 3 trillion dollar industry globally. Current practice: paper, PDFs, spreadsheets, walled-garden platforms. Fraud — fake invoices, double-factoring — is rampant. With Quidnug, the invoice is a title. Every lifecycle event is a signed event on that title's stream. Supplier issues, carrier confirms ship + deliver, buyer acknowledges receipt, financier factors, buyer pays. Before buying an invoice, a financier queries: has this been factored? Is there a carrier delivery event? Did the buyer acknowledge? Trust in the supplier and the buyer from my perspective? Decision in seconds, not weeks. Double-factoring becomes impossible because the second financier sees the first's factor event. Disputes have a signed evidence chain that replaces contested paperwork.

KEY POINTS:
• Invoice = title + event stream
• Every party signs their action
• Double-factor prevention built in
• Seconds to evaluate vs. weeks

TRANSITION:
FinTech done. AI next.""")
    p += 1

    # ---------- AI overview ----------
    s = slide_dark(prs, "AI — 4 use cases", kicker="CATEGORY 2", page=p, total=total)
    ai_cases = [
        ("AI Model Provenance",
         "Signed supply chain from training data → weights → fine-tunes → inferences.",
         "Titles  ·  Event streams  ·  Attester trust"),
        ("AI Agent Authorization",
         "Time-locked capability grants for autonomous agents with guardian veto.",
         "Guardians  ·  Risk-tiered routing  ·  Kill switch"),
        ("Federated Learning Attestation",
         "Prove gradient contributions to a shared model without central coordinator.",
         "Event streams  ·  Push gossip  ·  Round integrity"),
        ("AI Content Authenticity",
         "C2PA-plus: camera → editor → publisher chain with per-consumer trust.",
         "Per-asset event chain  ·  Transitive trust"),
    ]
    for i, (name, desc, feats) in enumerate(ai_cases):
        x = Inches(0.6 + (i % 2) * 6.1)
        y = Inches(1.85 + (i // 2) * 2.65)
        rounded(s, x, y, Inches(5.95), Inches(2.4), NAVY, corner=0.05)
        rect(s, x, y, Inches(5.95), Inches(0.5), TEAL)
        text_box(s, x, y, Inches(5.95), Inches(0.5), name,
                 font=H_FONT, size=14, color=MIDNIGHT, bold=True,
                 align=PP_ALIGN.CENTER, anchor=MSO_ANCHOR.MIDDLE)
        text_box(s, x + Inches(0.25), y + Inches(0.65), Inches(5.5), Inches(1.3),
                 desc, font=B_FONT, size=12.5, color=WHITE, line_spacing=1.3)
        chip(s, x + Inches(0.25), y + Inches(1.95), Inches(5.5), Inches(0.35),
             feats, fill=TEAL, color=MIDNIGHT, size=10, bold=True)
    notes(s, """TIMING: 45 seconds.

WHAT TO SAY:
Four AI use cases. Model provenance — signed chain from training data all the way through inferences. Agent authorization — capabilities are time-locked guardian grants instead of OAuth scopes. Federated learning attestation — cryptographic proof of contribution to shared models. Content authenticity — C2PA-style for AI-generated and edited content with relational trust overlay. All four share a theme: AI systems today have extensive signing and attestation needs, but the tooling is either missing or vendor-proprietary. Quidnug gives you a uniform, open substrate.

TRANSITION:
Model provenance first.""")
    p += 1

    # ---------- AI model provenance ----------
    s = slide_content(prs, "AI model provenance", kicker="AI — USE CASE 1", page=p, total=total)
    # Diagram: supply chain
    rounded(s, Inches(0.6), Inches(1.85), Inches(12.1), Inches(3.2), ICE, corner=0.05)
    text_box(s, Inches(0.85), Inches(2.0), Inches(11.6), Inches(0.4),
             "SIGNED CHAIN FROM DATASET → MODEL → FINE-TUNE → INFERENCE",
             font=B_FONT, size=11, color=SLATE, bold=True)
    chain = [
        ("Training\nDataset", TEAL, "title"),
        ("Base\nModel", NAVY, "title + training event"),
        ("Safety\nEvaluation", AMBER, "attestation event"),
        ("Fine-tune", NAVY, "authorized derivation"),
        ("Deployed\nModel", TEAL, "inference attestation"),
    ]
    for i, (name, col, kind) in enumerate(chain):
        x = Inches(0.85 + i * 2.45)
        y = Inches(2.6)
        rounded(s, x, y, Inches(2.35), Inches(1.7), col, corner=0.08)
        text_box(s, x, y + Inches(0.3), Inches(2.35), Inches(0.7), name,
                 font=H_FONT, size=14,
                 color=(MIDNIGHT if col == AMBER else WHITE), bold=True,
                 align=PP_ALIGN.CENTER, anchor=MSO_ANCHOR.MIDDLE,
                 line_spacing=1.2)
        rounded(s, x + Inches(0.3), y + Inches(1.2), Inches(1.75), Inches(0.35),
                WHITE, corner=0.3)
        text_box(s, x + Inches(0.3), y + Inches(1.2), Inches(1.75), Inches(0.35),
                 kind, font=CODE_FONT, size=9, color=MIDNIGHT, bold=True,
                 align=PP_ALIGN.CENTER, anchor=MSO_ANCHOR.MIDDLE)
        if i < len(chain) - 1:
            arrow(s, x + Inches(2.35), y + Inches(0.85),
                  x + Inches(2.45), y + Inches(0.85), color=SLATE, width=1.5)

    # Bottom: what consumers can now verify
    rounded(s, Inches(0.6), Inches(5.25), Inches(12.1), Inches(1.7), MIDNIGHT, corner=0.05)
    text_box(s, Inches(0.85), Inches(5.4), Inches(11.6), Inches(0.45),
             "WHAT A CONSUMER CAN NOW VERIFY", font=H_FONT, size=14,
             color=TEAL, bold=True)
    verifications = [
        "✓ Model trained on CC0-licensed data only (no copyright dispute)",
        "✓ Safety evaluation signed by MLCommons (org I trust)",
        "✓ Fine-tune authorized by base-model owner",
        "✓ This specific inference came from this specific model version",
    ]
    for i, v in enumerate(verifications):
        text_box(s, Inches(0.85 + (i % 2) * 6), Inches(5.85 + (i // 2) * 0.45),
                 Inches(6), Inches(0.4), v, font=B_FONT, size=11,
                 color=GREEN, bold=True)
    notes(s, """TIMING: 2 minutes.

WHAT TO SAY:
Modern AI supply chains touch dozens of parties. Training data from multiple sources. Base model developed and licensed. Fine-tuned variants building on the base. Safety evaluators running benchmarks. Inference endpoints serving the model. Every claim along this chain is contested — copyright lawsuits, benchmark self-reporting, derivation disputes, inference-endpoint forgery. With Quidnug, every link in the chain is a signed artifact. Training dataset is a title. Base model is a title with training events on its stream. Safety evaluators attest via events. Fine-tunes are derivation-authorized via the base model's signed event. Inferences can be signed attestations. A consumer can now verify the whole chain — this model was trained on CC0 data, safety-evaluated by MLCommons, fine-tune was authorized by the base-model owner, and this specific inference came from this specific model version. Copyright, safety, and licensing disputes now have a cryptographic evidence chain instead of PR statements.

KEY POINTS:
• Dataset / model / fine-tune as titles
• Training, safety, benchmark as events
• Derivation requires signed authorization
• Inference attestation possible

TRANSITION:
Agent authorization.""")
    p += 1

    # ---------- AI agent authorization ----------
    s = slide_content(prs, "AI agent authorization", kicker="AI — USE CASE 2", page=p, total=total)
    # Left: the agent's quid + capability committee
    rounded(s, Inches(0.6), Inches(1.85), Inches(6), Inches(5), ICE, corner=0.05)
    text_box(s, Inches(0.85), Inches(2.0), Inches(5.5), Inches(0.4),
             "AI AGENT = QUID WITH A CAPABILITY COMMITTEE",
             font=B_FONT, size=11, color=SLATE, bold=True)
    # Agent in center
    oval(s, Inches(2.4), Inches(2.55), Inches(1.8), Inches(1.3), AMBER)
    text_box(s, Inches(2.4), Inches(2.55), Inches(1.8), Inches(1.3),
             "Agent", font=H_FONT, size=18, color=MIDNIGHT, bold=True,
             align=PP_ALIGN.CENTER, anchor=MSO_ANCHOR.MIDDLE)
    # Surrounding 3 guardians
    g_positions = [
        (Inches(0.85), Inches(4.4), "Principal\n(owner, w=1)", TEAL),
        (Inches(2.8), Inches(5.1), "Safety\nCommittee (w=2)", NAVY),
        (Inches(4.7), Inches(4.4), "Audit Bot\n(w=1)", TEAL),
    ]
    for gx, gy, glab, gc in g_positions:
        rounded(s, gx, gy, Inches(1.5), Inches(0.85), gc, corner=0.1)
        text_box(s, gx, gy, Inches(1.5), Inches(0.85), glab,
                 font=B_FONT, size=10, color=WHITE, bold=True,
                 align=PP_ALIGN.CENTER, anchor=MSO_ANCHOR.MIDDLE,
                 line_spacing=1.2)
    text_box(s, Inches(0.85), Inches(6.05), Inches(5.5), Inches(0.85),
             "Threshold = 2. Routine actions self-signed. High-risk actions need committee cosign.",
             font=B_FONT, size=11, color=MIDNIGHT, italic=True,
             line_spacing=1.3)

    # Right: risk routing
    rounded(s, Inches(6.7), Inches(1.85), Inches(6), Inches(5), MIDNIGHT, corner=0.05)
    text_box(s, Inches(6.95), Inches(2.0), Inches(5.5), Inches(0.45),
             "RISK-CLASS → AUTHORIZATION ROUTING",
             font=H_FONT, size=13, color=TEAL, bold=True)
    routing = [
        ("trivial", "Agent self-signs", GREEN),
        ("low-routine", "Agent + audit bot (auto)", TEAL),
        ("medium", "Agent + committee threshold", NAVY),
        ("high", "Full committee + time-lock", AMBER),
        ("emergency-stop", "Invalidation anchor", RED),
    ]
    for i, (risk, action, col) in enumerate(routing):
        y = Inches(2.6 + i * 0.8)
        rounded(s, Inches(6.95), y, Inches(5.5), Inches(0.65), ICE, corner=0.05)
        rect(s, Inches(6.95), y, Inches(0.07), Inches(0.65), col)
        text_box(s, Inches(7.15), y + Inches(0.1), Inches(1.9), Inches(0.45),
                 risk, font=CODE_FONT, size=12, color=col, bold=True,
                 anchor=MSO_ANCHOR.MIDDLE)
        text_box(s, Inches(9.1), y + Inches(0.1), Inches(3.3), Inches(0.45),
                 action, font=B_FONT, size=11, color=MIDNIGHT,
                 anchor=MSO_ANCHOR.MIDDLE)
    notes(s, """TIMING: 1.5 minutes.

WHAT TO SAY:
AI agents spend money, sign contracts, write code that gets deployed, read sensitive data. Current OAuth-scope model is binary and not expressive enough. Quidnug's model: the agent is a quid with its own guardian set. The guardians serve as the 'capability committee' — typically the agent's owner, a safety committee, and an audit bot. Threshold is set by policy — let's say 2. On the right, risk-class routing: trivial actions the agent self-signs. Low-routine, agent plus an audit bot auto-cosigning. Medium, agent plus full committee threshold. High, full committee plus a time-lock window for humans to intervene. Emergency-stop is an invalidation anchor — agent's epoch is frozen immediately, no more actions from that key possible. This maps cleanly to how humans actually want to delegate to AI: small stuff automatic, big stuff needs humans, emergency stop is one button. And it's cryptographically enforceable, not trust-the-vendor.

KEY POINTS:
• Agent = quid + guardian committee
• Risk class → authorization route
• Time-lock window on high-risk actions
• One-button kill-switch via invalidation

TRANSITION:
Federated learning next.""")
    p += 1

    # ---------- Federated learning ----------
    s = slide_content(prs, "Federated learning attestation", kicker="AI — USE CASE 3", page=p, total=total)
    # Multi-round diagram
    rounded(s, Inches(0.6), Inches(1.85), Inches(12.1), Inches(3.2), ICE, corner=0.05)
    text_box(s, Inches(0.85), Inches(2.0), Inches(11.6), Inches(0.4),
             "ONE TRAINING ROUND — EACH PARTICIPANT'S CONTRIBUTION IS SIGNED",
             font=B_FONT, size=11, color=SLATE, bold=True)
    # 4 participants + coordinator
    round_y = Inches(2.7)
    # Participants
    parts = ["Bank A", "Bank B", "Bank C", "Bank D"]
    for i, lab in enumerate(parts):
        x = Inches(0.85 + i * 2.0)
        oval(s, x, round_y, Inches(1.4), Inches(0.8), TEAL)
        text_box(s, x, round_y, Inches(1.4), Inches(0.8), lab,
                 font=B_FONT, size=11, color=WHITE, bold=True,
                 align=PP_ALIGN.CENTER, anchor=MSO_ANCHOR.MIDDLE)
        # Gradient submission
        text_box(s, x, round_y + Inches(0.85), Inches(1.4), Inches(0.3),
                 "gradient\n(signed)", font=B_FONT, size=9, color=SLATE,
                 italic=True, align=PP_ALIGN.CENTER, line_spacing=1.1)
        arrow(s, x + Inches(0.7), round_y + Inches(1.3),
              Inches(9.5) + Inches(0.75), round_y + Inches(1.0),
              color=SLATE, width=1)
    # Coordinator
    oval(s, Inches(9.5), Inches(3.7), Inches(1.9), Inches(1.15), NAVY)
    text_box(s, Inches(9.5), Inches(3.7), Inches(1.9), Inches(1.15),
             "Coordinator\n(aggregator)",
             font=B_FONT, size=11, color=WHITE, bold=True,
             align=PP_ALIGN.CENTER, anchor=MSO_ANCHOR.MIDDLE, line_spacing=1.2)
    # Output
    rounded(s, Inches(11.6), Inches(3.75), Inches(1.0), Inches(1.05), GREEN, corner=0.08)
    text_box(s, Inches(11.6), Inches(3.75), Inches(1.0), Inches(1.05),
             "Updated\nmodel", font=B_FONT, size=10, color=WHITE, bold=True,
             align=PP_ALIGN.CENTER, anchor=MSO_ANCHOR.MIDDLE, line_spacing=1.2)
    arrow(s, Inches(11.4), Inches(4.28), Inches(11.6), Inches(4.28),
          color=SLATE, width=1.5)

    # Bottom: what this provides
    rounded(s, Inches(0.6), Inches(5.25), Inches(12.1), Inches(1.7), MIDNIGHT, corner=0.05)
    text_box(s, Inches(0.85), Inches(5.4), Inches(11.6), Inches(0.45),
             "WHAT THIS GIVES YOU", font=H_FONT, size=14, color=TEAL, bold=True)
    gives = [
        "Signed record of who contributed what gradient",
        "Coordinator accountability (re-run aggregation, verify)",
        "Byzantine-robust: adversarial gradients flagged publicly",
        "Credit allocation: revenue share based on verifiable contribution",
    ]
    for i, t in enumerate(gives):
        x = Inches(0.85 + (i % 2) * 6)
        y = Inches(5.9 + (i // 2) * 0.4)
        text_box(s, x, y, Inches(6), Inches(0.35),
                 "✓  " + t, font=B_FONT, size=11, color=CLOUD)
    notes(s, """TIMING: 1.5 minutes.

WHAT TO SAY:
Banks, hospitals, research consortia want to collaborate on ML models without exposing raw data. Federated learning lets them share only gradient updates. But 'who contributed what' and 'did the coordinator aggregate honestly' are trust questions that today have no cryptographic answer. Quidnug: each training round is a quid. Each participant's gradient submission is a signed event on that quid's stream. The coordinator's aggregated result is a signed event. Now: credit allocation has a cryptographic basis. Coordinator accountability is achievable — other participants can independently re-run the aggregation and verify. Byzantine-robust by construction, because adversarial gradients can be publicly flagged with evidence. No central coordinator-trust required; the process is auditable by every participant.

KEY POINTS:
• Each gradient = signed event
• Coordinator accountability via re-run
• Credit allocation on verifiable contribution
• Public flagging of adversarial updates

TRANSITION:
Last AI — content authenticity.""")
    p += 1

    # ---------- Content authenticity ----------
    s = slide_content(prs, "AI content authenticity", kicker="AI — USE CASE 4", page=p, total=total)
    # Timeline of a photo
    rounded(s, Inches(0.6), Inches(1.85), Inches(12.1), Inches(3), ICE, corner=0.05)
    text_box(s, Inches(0.85), Inches(2.0), Inches(11.6), Inches(0.4),
             "THE PROVENANCE CHAIN FOR A SINGLE NEWS PHOTO",
             font=B_FONT, size=11, color=SLATE, bold=True)
    chain = [
        ("Camera\n(Canon-5D)", "capture\nevent", TEAL),
        ("Photographer", "crop\nevent", TEAL),
        ("Editor\n(Reuters)", "color-grade\nevent", NAVY),
        ("Publisher\n(Reuters)", "publish\nevent", AMBER),
        ("Fact-check\n(optional)", "attest\nevent", GREEN),
    ]
    for i, (who, action, col) in enumerate(chain):
        x = Inches(0.85 + i * 2.45)
        y = Inches(2.55)
        rounded(s, x, y, Inches(2.3), Inches(1.9), col, corner=0.08)
        text_box(s, x, y + Inches(0.15), Inches(2.3), Inches(0.7), who,
                 font=H_FONT, size=13,
                 color=(MIDNIGHT if col == AMBER else WHITE), bold=True,
                 align=PP_ALIGN.CENTER, anchor=MSO_ANCHOR.MIDDLE,
                 line_spacing=1.2)
        rounded(s, x + Inches(0.3), y + Inches(1.15), Inches(1.7), Inches(0.6),
                WHITE, corner=0.2)
        text_box(s, x + Inches(0.3), y + Inches(1.15), Inches(1.7), Inches(0.6),
                 action, font=CODE_FONT, size=9, color=MIDNIGHT, bold=True,
                 align=PP_ALIGN.CENTER, anchor=MSO_ANCHOR.MIDDLE,
                 line_spacing=1.15)
        if i < len(chain) - 1:
            arrow(s, x + Inches(2.3), y + Inches(0.95),
                  x + Inches(2.45), y + Inches(0.95), color=SLATE, width=1.5)

    # Bottom: consumer trust comparison
    rounded(s, Inches(0.6), Inches(5.05), Inches(12.1), Inches(1.9), MIDNIGHT, corner=0.05)
    text_box(s, Inches(0.85), Inches(5.2), Inches(11.6), Inches(0.45),
             "PER-CONSUMER TRUST MAKES THE SAME PHOTO DIFFERENTLY CREDIBLE",
             font=B_FONT, size=11, color=TEAL, bold=True)
    consumers = [
        ("Trusted news reader", "Trusts Reuters strongly", "→ accepts at face value", GREEN),
        ("Fact-check aggregator", "Requires ≥2 editors' attestations", "→ checks; verifies", TEAL),
        ("Skeptical blogger", "Zero trust in Reuters editorials", "→ accepts camera only", AMBER),
        ("Deepfake-wary layperson", "Only trusts fact-checker attestations", "→ waits for check", RED),
    ]
    for i, (who, how, result, col) in enumerate(consumers):
        x = Inches(0.85 + (i % 2) * 6)
        y = Inches(5.7 + (i // 2) * 0.6)
        oval(s, x, y + Inches(0.05), Inches(0.3), Inches(0.3), col)
        text_box(s, x + Inches(0.4), y - Inches(0.02), Inches(1.9), Inches(0.35),
                 who, font=B_FONT, size=11, color=WHITE, bold=True)
        text_box(s, x + Inches(2.35), y - Inches(0.02), Inches(1.6), Inches(0.35),
                 how, font=B_FONT, size=10, color=CLOUD, italic=True)
        text_box(s, x + Inches(4.0), y - Inches(0.02), Inches(2.0), Inches(0.35),
                 result, font=B_FONT, size=10, color=col, bold=True)
    notes(s, """TIMING: 1.5 minutes.

WHAT TO SAY:
AI content authenticity. C2PA is the industry standard starting to ship in cameras — every capture gets cryptographically signed metadata embedded in the image. Quidnug adds the trust layer on top. Every camera is a quid. Photographer, editor, publisher are quids. The photo's life story is a chain of signed events on its title. Here: Canon-5D captures. Photographer crops. Reuters editor color-grades. Reuters publishes. Optional fact-check attests. Each step is signed. Now consumer trust is relational — a trusted-news reader trusts Reuters editors strongly and accepts. A fact-check aggregator requires multiple editors' signatures. A skeptical blogger might trust only the camera's capture signature and treat everything after it as unverified. A deepfake-wary layperson might require a trusted fact-checker attestation before accepting anything. Same photo, different consumers, different credibility — all cryptographically grounded.

KEY POINTS:
• Camera, editor, publisher all quids
• Full signed chain per asset
• Per-consumer trust weighting
• Replaces C2PA 'trust the PKI'

TRANSITION:
Government next — elections.""")
    p += 1

    return p - start_page
