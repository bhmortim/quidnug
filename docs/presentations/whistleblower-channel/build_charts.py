"""Charts for Whistleblower Channel deck."""
import pathlib
import matplotlib.pyplot as plt
from matplotlib.patches import FancyBboxPatch, FancyArrowPatch, Circle, Rectangle
import numpy as np

HERE = pathlib.Path(__file__).parent
ASSETS = HERE / "assets"
ASSETS.mkdir(exist_ok=True)

DARK_BG = "#0A1628"; MID = "#1C3A5E"; TEAL = "#00D4A8"
TEAL_SOFT = "#C3EFE3"; CORAL = "#FF4655"; EMERALD = "#10B981"
AMBER = "#F59E0B"; WHITE = "#FFFFFF"; LIGHT_BG = "#F5F7FA"
TEXT_DARK = "#1A1D23"; TEXT_MUTED = "#64748B"; GRID = "#E2E8F0"


def _light(ax):
    ax.set_facecolor(WHITE)
    for s in ax.spines.values():
        s.set_color("#CBD5E1")
    ax.tick_params(colors=TEXT_MUTED)
    ax.xaxis.label.set_color(TEXT_DARK); ax.yaxis.label.set_color(TEXT_DARK)
    ax.title.set_color(TEXT_DARK)
    ax.grid(True, color=GRID, linewidth=0.6, alpha=0.9)


def chart_espionage_prosecutions():
    """Espionage Act leaker prosecutions by administration."""
    fig, ax = plt.subplots(figsize=(12, 5.2), dpi=150)
    fig.patch.set_facecolor(WHITE)
    admins = ["Pre-Obama\n(all presidents\n1917-2008)", "Obama\n(2009-2016)",
              "Trump I\n(2017-2020)", "Biden\n(2021-2024)"]
    counts = [3, 8, 8, 4]
    colors = [MID, CORAL, CORAL, AMBER]
    bars = ax.bar(admins, counts, color=colors, edgecolor=MID,
                  linewidth=1.2, width=0.6)
    for bar, v in zip(bars, counts):
        ax.text(bar.get_x()+bar.get_width()/2, bar.get_height()+0.2,
                str(v), ha="center", va="bottom", fontsize=22,
                fontweight="bold", color=TEXT_DARK)
    ax.set_ylim(0, 11)
    ax.set_ylabel("Leak prosecutions under Espionage Act", fontsize=12)
    ax.set_title(
        "The chilling effect: leak prosecutions keep rising",
        fontsize=14, fontweight="bold", color=TEXT_DARK, pad=12)
    ax.text(0.0, 10.2,
            "Source: ACLU National Security reporting. The risk calculus for "
            "sources has materially worsened in the last 15 years.",
            fontsize=9, style="italic", color=TEXT_MUTED)
    _light(ax)
    plt.tight_layout()
    out = ASSETS / "chart_prosecutions.png"
    plt.savefig(out, bbox_inches="tight", facecolor=WHITE)
    plt.close()
    print(f"wrote {out}")


def chart_tool_coverage():
    """Matrix: which layer does each existing tool cover?"""
    fig, ax = plt.subplots(figsize=(12, 5.8), dpi=150)
    fig.patch.set_facecolor(WHITE)
    layers = ["Network\nanonymity", "Operational\nsecurity",
              "Submission\nchannel", "Encrypted\ncomms",
              "Source\ncredibility"]
    tools = ["Tor", "Tails", "SecureDrop", "Signal",
             "Quidnug\nsubstrate"]
    # coverage matrix: rows=tools, cols=layers. 1 = covered, 0 = not.
    matrix = np.array([
        [1, 0, 0, 0, 0],   # Tor
        [1, 1, 0, 0, 0],   # Tails
        [1, 0, 1, 0, 0],   # SecureDrop
        [0, 0, 0, 1, 0],   # Signal
        [0, 0, 0, 1, 1],   # Quidnug
    ])
    for i, tool in enumerate(tools):
        for j, layer in enumerate(layers):
            covered = matrix[i, j]
            color = EMERALD if covered else "#F1F5F9"
            rect = Rectangle((j-0.45, i-0.45), 0.9, 0.9, facecolor=color,
                             edgecolor=MID, linewidth=1.2)
            ax.add_patch(rect)
            if covered:
                ax.text(j, i, "\u2714", ha="center", va="center",
                        fontsize=22, color=WHITE, fontweight="bold")
    ax.set_xticks(range(len(layers)))
    ax.set_xticklabels(layers, fontsize=10, color=TEXT_DARK)
    ax.set_yticks(range(len(tools)))
    ax.set_yticklabels(tools, fontsize=11, color=TEXT_DARK, fontweight="bold")
    ax.set_xlim(-0.6, len(layers)-0.4)
    ax.set_ylim(-0.6, len(tools)-0.4)
    ax.invert_yaxis()
    ax.set_aspect("equal")
    ax.set_title(
        "Current infrastructure covers four layers. Credibility is the gap.",
        fontsize=14, fontweight="bold", color=TEXT_DARK, pad=14)
    for spine in ax.spines.values():
        spine.set_visible(False)
    ax.tick_params(length=0)
    plt.tight_layout()
    out = ASSETS / "chart_coverage.png"
    plt.savefig(out, bbox_inches="tight", facecolor=WHITE)
    plt.close()
    print(f"wrote {out}")


def chart_credibility_stack():
    """5-layer credibility stack pyramid."""
    fig, ax = plt.subplots(figsize=(12, 6.2), dpi=150)
    fig.patch.set_facecolor(WHITE)
    layers = [
        ("Layer 5  \u2022  Journalist editorial vetting",
         "Traditional cross-corroboration and judgment", "#1E3A8A"),
        ("Layer 4  \u2022  Document signatures",
         "Corporate PKI proves document provenance", MID),
        ("Layer 3  \u2022  Peer attestations",
         "Pseudonymous colleagues vouch for the source", "#0E7490"),
        ("Layer 2  \u2022  Institutional attestation",
         "DNS-anchored professional body signs credential", EMERALD),
        ("Layer 1  \u2022  Source pseudonym signature",
         "Stable keypair with accumulated track record", TEAL),
    ]
    y_positions = np.linspace(0.5, 5.5, len(layers))
    widths = np.linspace(8.5, 11, len(layers))[::-1]
    for (label, desc, color), y, w in zip(layers, y_positions, widths):
        rect = FancyBboxPatch((6 - w/2, y-0.42), w, 0.84,
                              boxstyle="round,pad=0.02,rounding_size=0.1",
                              facecolor=color, edgecolor=MID, linewidth=1.2)
        ax.add_patch(rect)
        ax.text(6, y+0.13, label, ha="center", va="center", fontsize=12,
                fontweight="bold", color=WHITE)
        ax.text(6, y-0.22, desc, ha="center", va="center", fontsize=10,
                color="#E5F4FF", style="italic")
    ax.set_xlim(0, 12)
    ax.set_ylim(0, 6.2)
    ax.set_title(
        "Credibility stack  \u2022  Each layer adds independent evidence",
        fontsize=14, fontweight="bold", color=TEXT_DARK, pad=14)
    ax.set_aspect("auto")
    ax.axis("off")
    plt.tight_layout()
    out = ASSETS / "chart_stack.png"
    plt.savefig(out, bbox_inches="tight", facecolor=WHITE)
    plt.close()
    print(f"wrote {out}")


def chart_blind_signature_flow():
    """Blind signature issuance flow."""
    fig, ax = plt.subplots(figsize=(12, 5.4), dpi=150)
    fig.patch.set_facecolor(WHITE)
    actors = [("Source", 1, EMERALD),
              ("Attestation\nAuthority", 6, MID),
              ("Publisher", 11, TEAL)]
    for name, x, color in actors:
        circle = Circle((x, 4.3), 0.55, facecolor=color, edgecolor=MID,
                        linewidth=1.5)
        ax.add_patch(circle)
        ax.text(x, 4.3, name.split("\n")[0][0], ha="center", va="center",
                fontsize=22, fontweight="bold", color=WHITE)
        ax.text(x, 5.25, name, ha="center", va="bottom", fontsize=11,
                fontweight="bold", color=TEXT_DARK)
    # Steps
    steps = [
        (1, 6, 3.7, "Identity proof\n+ blinded request"),
        (6, 1, 3.1, "Blinded signature\n(Attester can't see content)"),
        (1, 1, 2.4, "Unblind locally \u2192\nholds proof token"),
        (1, 11, 1.5, "Present zero-knowledge proof\nwith selected attributes"),
        (11, 11, 0.8, "Verify signature chains\nto Attester's DNS identity"),
    ]
    for x1, x2, y, label in steps:
        if x1 == x2:
            # self-action
            rect = FancyBboxPatch((x1-1.3, y-0.25), 2.6, 0.55,
                                  boxstyle="round,pad=0.02,rounding_size=0.08",
                                  facecolor=LIGHT_BG, edgecolor=MID,
                                  linewidth=1)
            ax.add_patch(rect)
            ax.text(x1, y+0.02, label, ha="center", va="center", fontsize=9,
                    color=TEXT_DARK)
        else:
            ax.annotate("", xy=(x2-0.6, y), xytext=(x1+0.6, y),
                        arrowprops=dict(arrowstyle="->", color=MID, lw=1.8))
            ax.text((x1+x2)/2, y+0.15, label, ha="center", va="bottom",
                    fontsize=9, color=TEXT_DARK,
                    bbox=dict(boxstyle="round,pad=0.2", facecolor=WHITE,
                              edgecolor="none", alpha=0.95))
    ax.set_xlim(-0.5, 12.5)
    ax.set_ylim(0.3, 6)
    ax.set_title(
        "Blind signature flow  \u2022  Attester never sees what it's signing",
        fontsize=14, fontweight="bold", color=TEXT_DARK, pad=14)
    ax.axis("off")
    plt.tight_layout()
    out = ASSETS / "chart_blind_flow.png"
    plt.savefig(out, bbox_inches="tight", facecolor=WHITE)
    plt.close()
    print(f"wrote {out}")


def chart_pseudonym_history():
    """Example pseudonym's accumulating reputation over time."""
    fig, ax = plt.subplots(figsize=(12, 5.4), dpi=150)
    fig.patch.set_facecolor(WHITE)
    events = [
        ("2024-03", 0.45, "Story 1 published\n(unproven initially)"),
        ("2024-05", 0.62, "SEC filing confirms\nStory 1 claims"),
        ("2024-09", 0.68, "Story 2 published"),
        ("2024-12", 0.79, "8-K restatement\nconfirms Story 2"),
        ("2025-06", 0.82, "Story 3 published"),
        ("2025-09", 0.91, "Third-party audit\nconfirms Story 3"),
        ("2026-02", 0.93, "Story 4 in progress\n(current)"),
    ]
    xs = np.arange(len(events))
    ys = [e[1] for e in events]
    ax.plot(xs, ys, color=EMERALD, linewidth=2.5, marker="o",
            markersize=10, markerfacecolor=EMERALD,
            markeredgecolor=WHITE, markeredgewidth=2, zorder=3)
    ax.fill_between(xs, 0, ys, color=EMERALD, alpha=0.15)
    for x, (date, y, label) in zip(xs, events):
        ax.annotate(label, xy=(x, y), xytext=(0, 16 if x % 2 == 0 else -34),
                    textcoords="offset points", ha="center", fontsize=8.5,
                    color=TEXT_DARK,
                    bbox=dict(boxstyle="round,pad=0.3", facecolor=WHITE,
                              edgecolor="#CBD5E1", alpha=0.95))
    ax.set_xticks(xs)
    ax.set_xticklabels([e[0] for e in events], fontsize=10)
    ax.set_ylim(0, 1.05)
    ax.set_ylabel("Composite credibility score", fontsize=12)
    ax.set_title(
        "Pseudonym did:quidnug:pseudo-banker-a2c7  \u2022  "
        "Reputation accumulates without identity",
        fontsize=13, fontweight="bold", color=TEXT_DARK, pad=12)
    _light(ax)
    plt.tight_layout()
    out = ASSETS / "chart_pseudonym.png"
    plt.savefig(out, bbox_inches="tight", facecolor=WHITE)
    plt.close()
    print(f"wrote {out}")


def chart_trust_graph():
    """Trust graph: source + institution + peers."""
    fig, ax = plt.subplots(figsize=(12, 6.2), dpi=150)
    fig.patch.set_facecolor(WHITE)
    # Center: source pseudonym
    src = (6, 3.2)
    ax.add_patch(Circle(src, 0.55, facecolor=EMERALD, edgecolor=MID,
                        linewidth=1.8, zorder=3))
    ax.text(src[0], src[1], "Source", ha="center", va="center",
            fontsize=10, fontweight="bold", color=WHITE)
    ax.text(src[0], src[1]-0.9, "did:quidnug:pseudo-banker-a2c7",
            ha="center", va="top", fontsize=8.5, color=TEXT_MUTED)
    # Institution above
    inst = (6, 5.6)
    ax.add_patch(FancyBboxPatch((inst[0]-1.6, inst[1]-0.4), 3.2, 0.8,
                boxstyle="round,pad=0.05,rounding_size=0.12",
                facecolor=TEAL, edgecolor=MID, linewidth=1.5, zorder=3))
    ax.text(inst[0], inst[1], "Institute of Banking and Finance",
            ha="center", va="center", fontsize=10, fontweight="bold",
            color=WHITE)
    ax.text(inst[0], inst[1]-0.65, "DNS-anchored  \u2022  ibanking.example",
            ha="center", va="top", fontsize=8.5, color=TEXT_MUTED)
    # Peers on sides
    peers = [(1.8, 4.5, "Peer A", "pseudo-colleague-7f"),
             (10.2, 4.5, "Peer B", "pseudo-colleague-9c"),
             (1.8, 1.9, "Peer C", "pseudo-colleague-22")]
    for px, py, label, nid in peers:
        ax.add_patch(Circle((px, py), 0.42, facecolor=MID, edgecolor=MID,
                            linewidth=1.5, zorder=3))
        ax.text(px, py, label, ha="center", va="center", fontsize=9,
                fontweight="bold", color=WHITE)
        ax.text(px, py-0.7, nid, ha="center", va="top", fontsize=7.5,
                color=TEXT_MUTED)
    # Journalist
    jour = (10.2, 1.9)
    ax.add_patch(FancyBboxPatch((jour[0]-1.1, jour[1]-0.35), 2.2, 0.7,
                boxstyle="round,pad=0.04,rounding_size=0.1",
                facecolor=AMBER, edgecolor=MID, linewidth=1.5, zorder=3))
    ax.text(jour[0], jour[1], "Journalist", ha="center", va="center",
            fontsize=10, fontweight="bold", color=WHITE)
    ax.text(jour[0], jour[1]-0.6, "verifies stack",
            ha="center", va="top", fontsize=8.5, color=TEXT_MUTED)
    # Edges
    edges = [
        (inst, src, "attests"),
        ((1.8, 4.5), src, "peer-vouches"),
        ((10.2, 4.5), src, "peer-vouches"),
        ((1.8, 1.9), src, "peer-vouches"),
        (src, jour, "discloses"),
    ]
    for a, b, lbl in edges:
        ax.annotate("", xy=(b[0], b[1]), xytext=(a[0], a[1]),
                    arrowprops=dict(arrowstyle="->", color=MID, lw=1.5,
                                    alpha=0.8, shrinkA=28, shrinkB=28),
                    zorder=2)
        mx, my = (a[0]+b[0])/2, (a[1]+b[1])/2
        ax.text(mx, my, lbl, ha="center", va="center", fontsize=8,
                color=TEXT_DARK,
                bbox=dict(boxstyle="round,pad=0.2", facecolor=WHITE,
                          edgecolor="#CBD5E1", alpha=0.95))
    ax.set_xlim(0, 12)
    ax.set_ylim(0.5, 6.5)
    ax.set_title(
        "Trust graph  \u2022  Institution + peers surround the source",
        fontsize=14, fontweight="bold", color=TEXT_DARK, pad=14)
    ax.set_aspect("equal")
    ax.axis("off")
    plt.tight_layout()
    out = ASSETS / "chart_graph.png"
    plt.savefig(out, bbox_inches="tight", facecolor=WHITE)
    plt.close()
    print(f"wrote {out}")


def chart_architecture():
    """End-to-end architecture diagram."""
    fig, ax = plt.subplots(figsize=(12, 6.4), dpi=150)
    fig.patch.set_facecolor(WHITE)
    layers = [
        (0.5, "Source\ninfrastructure",
         ["Tails / Tor", "Air-gapped key", "Credential store"], EMERALD),
        (3.4, "Communication",
         ["SecureDrop", "QDP-0024 MLS", "Document upload"], MID),
        (6.3, "Credibility chain",
         ["Pseudonym history", "Blind credential",
          "Peer attestations", "Doc signatures"], TEAL),
        (9.2, "Journalist\nworkflow",
         ["Verify chain", "Editorial judgment",
          "Publish w/ metadata"], "#1E3A8A"),
        (11.3, "Reader\nexperience",
         ["Trust calculation", "Audit trail"], AMBER),
    ]
    for x, title, items, color in layers:
        h = 0.75 + 0.6 * len(items)
        rect = FancyBboxPatch((x-1.0, 2.8-h/2), 2.0, h,
                boxstyle="round,pad=0.04,rounding_size=0.12",
                facecolor=WHITE, edgecolor=color, linewidth=2.2)
        ax.add_patch(rect)
        # header bar
        ax.add_patch(FancyBboxPatch((x-1.0, 2.8+h/2-0.55), 2.0, 0.55,
                boxstyle="round,pad=0.02,rounding_size=0.12",
                facecolor=color, edgecolor=color, linewidth=0))
        ax.text(x, 2.8+h/2-0.27, title, ha="center", va="center",
                fontsize=10, fontweight="bold", color=WHITE)
        for i, item in enumerate(items):
            ax.text(x, 2.8+h/2-0.85 - i*0.42, f"\u2022 {item}",
                    ha="center", va="center", fontsize=8.5, color=TEXT_DARK)
    # Flow arrows
    arrows_x = [(1.5, 2.4), (4.4, 5.3), (7.3, 8.2), (10.2, 10.3)]
    for x1, x2 in arrows_x:
        ax.annotate("", xy=(x2, 2.8), xytext=(x1, 2.8),
                    arrowprops=dict(arrowstyle="->", color=MID,
                                    lw=2, alpha=0.7))
    ax.set_xlim(-0.8, 12.6)
    ax.set_ylim(0, 5.6)
    ax.set_title(
        "End-to-end architecture  \u2022  Source protection + credibility chain + reader audit",
        fontsize=13, fontweight="bold", color=TEXT_DARK, pad=14)
    ax.set_aspect("auto")
    ax.axis("off")
    plt.tight_layout()
    out = ASSETS / "chart_arch.png"
    plt.savefig(out, bbox_inches="tight", facecolor=WHITE)
    plt.close()
    print(f"wrote {out}")


def chart_jurisdictions():
    """Comparison of whistleblower legal frameworks across jurisdictions."""
    fig, ax = plt.subplots(figsize=(12, 5.6), dpi=150)
    fig.patch.set_facecolor(WHITE)
    jurisdictions = ["US\n(Dodd-Frank +\nEspionage Act)",
                     "EU\n(Directive\n2019/1937)",
                     "UK\n(Public Interest\nDisclosure Act)",
                     "Non-democratic\nregimes"]
    dimensions = ["Monetary\nincentive", "Anti-retaliation\nprotection",
                  "Anonymous\nreporting", "Substrate\nalignment"]
    # scores 0-3 per dimension per jurisdiction
    scores = np.array([
        [3, 2, 1, 3],   # US: strong $ incentive, mixed protection, limited anon
        [1, 3, 3, 3],   # EU: GDPR strong, allows anon, strong protection
        [0, 2, 1, 2],   # UK: no money, some protection, limited anon
        [0, 0, 3, 2],   # non-dem: no law, full anon-by-necessity
    ])
    color_scale = ["#F1F5F9", AMBER, TEAL, EMERALD]
    labels_map = ["None", "Weak", "Medium", "Strong"]
    for i, juris in enumerate(jurisdictions):
        for j, dim in enumerate(dimensions):
            s = scores[i, j]
            rect = Rectangle((j-0.45, i-0.45), 0.9, 0.9,
                             facecolor=color_scale[s], edgecolor=MID,
                             linewidth=1.1)
            ax.add_patch(rect)
            ax.text(j, i, labels_map[s], ha="center", va="center",
                    fontsize=10, fontweight="bold",
                    color=WHITE if s >= 2 else TEXT_DARK)
    ax.set_xticks(range(len(dimensions)))
    ax.set_xticklabels(dimensions, fontsize=10, color=TEXT_DARK)
    ax.set_yticks(range(len(jurisdictions)))
    ax.set_yticklabels(jurisdictions, fontsize=10, color=TEXT_DARK,
                       fontweight="bold")
    ax.set_xlim(-0.6, len(dimensions)-0.4)
    ax.set_ylim(-0.6, len(jurisdictions)-0.4)
    ax.invert_yaxis()
    ax.set_aspect("equal")
    ax.set_title(
        "Legal frameworks  \u2022  Substrate fits every jurisdiction differently",
        fontsize=14, fontweight="bold", color=TEXT_DARK, pad=14)
    for spine in ax.spines.values():
        spine.set_visible(False)
    ax.tick_params(length=0)
    plt.tight_layout()
    out = ASSETS / "chart_jurisdictions.png"
    plt.savefig(out, bbox_inches="tight", facecolor=WHITE)
    plt.close()
    print(f"wrote {out}")


def chart_privacy_pass_scale():
    """Deployed blind-signature systems at scale (Privacy Pass, Apple)."""
    fig, ax = plt.subplots(figsize=(12, 5.4), dpi=150)
    fig.patch.set_facecolor(WHITE)
    systems = ["Cloudflare\nPrivacy Pass\n(2017-)",
               "Apple Private\nRelay (2021-)",
               "Apple Private\nClick Measure\n(2021-)",
               "Firefox\nPrivacy Pass\n(2019-)",
               "W3C VC BBS+\n(enterprise)"]
    monthly_tokens = [2.8, 1.2, 0.6, 0.15, 0.05]  # billions per month
    bars = ax.bar(systems, monthly_tokens, color=[TEAL, EMERALD, MID,
                                                    "#0E7490", AMBER],
                  edgecolor=MID, linewidth=1.2, width=0.6)
    for bar, v in zip(bars, monthly_tokens):
        label = f"{v:.2f}B" if v < 1 else f"{v:.1f}B"
        ax.text(bar.get_x()+bar.get_width()/2, bar.get_height()+0.05,
                label, ha="center", va="bottom", fontsize=13,
                fontweight="bold", color=TEXT_DARK)
    ax.set_ylim(0, 3.4)
    ax.set_ylabel("Estimated blind-signature tokens / month (billions)",
                  fontsize=11)
    ax.set_title(
        "Blind signatures are proven at massive scale",
        fontsize=14, fontweight="bold", color=TEXT_DARK, pad=12)
    ax.text(-0.5, 3.15,
            "Sources: Cloudflare Research, Apple WWDC disclosures, "
            "IETF RFC 9576 (Privacy Pass). The cryptography is not "
            "novel; the application to whistleblowing is.",
            fontsize=9, style="italic", color=TEXT_MUTED)
    _light(ax)
    plt.tight_layout()
    out = ASSETS / "chart_scale.png"
    plt.savefig(out, bbox_inches="tight", facecolor=WHITE)
    plt.close()
    print(f"wrote {out}")


def chart_worked_example():
    """Financial-fraud disclosure walkthrough timeline."""
    fig, ax = plt.subplots(figsize=(12, 6.2), dpi=150)
    fig.patch.set_facecolor(WHITE)
    phases = [
        ("Discovery", "Week 0",
         "Alice finds systematic\nloan-loss reserve misreporting", EMERALD),
        ("Preparation", "Week 1-4",
         "Generates airgap keypair, obtains\nIBF credential, scrubs docs", TEAL),
        ("Peer attestations", "Week 4-6",
         "Two colleagues provide\npseudonymous vouches", MID),
        ("Disclosure", "Week 6",
         "Submits via SecureDrop + QDP-0024.\n17 docs, 12 corporate-signed", "#1E3A8A"),
        ("Verification", "Week 6-10",
         "Journalist chains credentials,\ncorroborates independently", "#0E7490"),
        ("Publication", "Week 12",
         "Story published with\ncredibility metadata", AMBER),
        ("Regulatory", "Week 14",
         "SEC accepts attested pseudonym tip,\nopens investigation", CORAL),
        ("Enforcement", "Month 9-18",
         "SEC action, penalties,\nDodd-Frank award processed", EMERALD),
    ]
    xs = np.linspace(1, 11, len(phases))
    y = 3.2
    for x, (name, week, desc, color) in zip(xs, phases):
        # Node
        ax.add_patch(Circle((x, y), 0.42, facecolor=color, edgecolor=MID,
                            linewidth=1.5, zorder=3))
        # Phase label above
        ax.text(x, y+0.85, name, ha="center", va="bottom", fontsize=10,
                fontweight="bold", color=TEXT_DARK)
        ax.text(x, y+0.65, week, ha="center", va="bottom", fontsize=8.5,
                color=TEXT_MUTED, style="italic")
        # Description below
        ax.text(x, y-0.75, desc, ha="center", va="top", fontsize=8.5,
                color=TEXT_DARK,
                bbox=dict(boxstyle="round,pad=0.3", facecolor=LIGHT_BG,
                          edgecolor="#CBD5E1", alpha=0.95))
    # Connecting line
    ax.plot([xs[0], xs[-1]], [y, y], color=MID, linewidth=2,
            alpha=0.4, zorder=1)
    ax.set_xlim(0, 12)
    ax.set_ylim(0.5, 5)
    ax.set_title(
        "Worked example  \u2022  Financial fraud disclosure, end-to-end",
        fontsize=14, fontweight="bold", color=TEXT_DARK, pad=14)
    ax.set_aspect("auto")
    ax.axis("off")
    plt.tight_layout()
    out = ASSETS / "chart_worked.png"
    plt.savefig(out, bbox_inches="tight", facecolor=WHITE)
    plt.close()
    print(f"wrote {out}")


def chart_snowden_haugen():
    """Two case studies: what made them credible."""
    fig, (ax1, ax2) = plt.subplots(1, 2, figsize=(12, 5.6), dpi=150)
    fig.patch.set_facecolor(WHITE)
    cases = [
        (ax1, "Snowden (2013)",
         [("Credentials verified", 0.95, EMERALD),
          ("Months of journalist vetting", 0.9, EMERALD),
          ("Eventually public identity", 0.85, EMERALD),
          ("Cost: exile", 0.15, CORAL)]),
        (ax2, "Haugen (2021)",
         [("Thousands of signed docs", 0.92, EMERALD),
          ("Sworn congressional testimony", 0.88, EMERALD),
          ("Legal protections invoked", 0.8, TEAL),
          ("Cost: career impact", 0.35, AMBER)]),
    ]
    for ax, title, items in cases:
        labels = [i[0] for i in items]
        values = [i[1] for i in items]
        colors = [i[2] for i in items]
        y_pos = np.arange(len(labels))
        ax.barh(y_pos, values, color=colors, edgecolor=MID,
                linewidth=1, height=0.6)
        ax.set_yticks(y_pos)
        ax.set_yticklabels(labels, fontsize=10, color=TEXT_DARK)
        ax.invert_yaxis()
        ax.set_xlim(0, 1)
        ax.set_xlabel("Strength / cost factor", fontsize=10)
        ax.set_title(title, fontsize=13, fontweight="bold", color=TEXT_DARK,
                     pad=8)
        _light(ax)
    fig.suptitle(
        "Successful disclosures required extensive credibility work",
        fontsize=14, fontweight="bold", color=TEXT_DARK, y=1.02)
    plt.tight_layout()
    out = ASSETS / "chart_cases.png"
    plt.savefig(out, bbox_inches="tight", facecolor=WHITE)
    plt.close()
    print(f"wrote {out}")


def main():
    chart_espionage_prosecutions()
    chart_tool_coverage()
    chart_credibility_stack()
    chart_blind_signature_flow()
    chart_pseudonym_history()
    chart_trust_graph()
    chart_architecture()
    chart_jurisdictions()
    chart_privacy_pass_scale()
    chart_worked_example()
    chart_snowden_haugen()


if __name__ == "__main__":
    main()
