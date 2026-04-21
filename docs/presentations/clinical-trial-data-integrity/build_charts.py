"""Charts for Clinical Trial Data Integrity deck."""
import pathlib
import matplotlib.pyplot as plt
from matplotlib.patches import FancyBboxPatch, FancyArrowPatch, Circle
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


def chart_alcoa():
    fig, ax = plt.subplots(figsize=(13, 5.2), dpi=150)
    fig.patch.set_facecolor(WHITE)
    letters = [
        ("A", "Attributable", "Who did this?", TEAL),
        ("L", "Legible", "Can it be read?", TEAL),
        ("C", "Contemporaneous", "When was it recorded?", TEAL),
        ("O", "Original", "Or a verified copy?", TEAL),
        ("A", "Accurate", "Free of errors", TEAL),
        ("+", "Complete", "All data present", EMERALD),
        ("+", "Consistent", "Across forms", EMERALD),
        ("+", "Enduring", "Preserved long-term", EMERALD),
        ("+", "Available", "Retrievable", EMERALD),
    ]
    cell_w = 1.4
    for i, (letter, name, body, color) in enumerate(letters):
        x = 0.3 + i * cell_w
        # Big letter
        circ = Circle((x + 0.55, 4.3), 0.4, facecolor=color,
                      edgecolor=MID, linewidth=1.5)
        ax.add_patch(circ)
        ax.text(x + 0.55, 4.3, letter, ha="center", va="center",
                fontsize=24, fontweight="bold", color=WHITE)
        # Name
        ax.text(x + 0.55, 3.4, name, ha="center", va="center",
                fontsize=11, fontweight="bold", color=TEXT_DARK)
        # Description
        rect = FancyBboxPatch((x + 0.05, 1.5), cell_w - 0.2, 1.5,
                              boxstyle="round,pad=0.04,rounding_size=0.06",
                              facecolor=LIGHT_BG, edgecolor=color,
                              linewidth=1.2)
        ax.add_patch(rect)
        ax.text(x + 0.55, 2.25, body, ha="center", va="center",
                fontsize=9, color=TEXT_DARK)
    ax.text(7, 0.6,
            "ALCOA (teal): original 5 FDA principles. + (green): ICH "
            "modern additions. All 9 required.",
            ha="center", va="center", fontsize=11, style="italic",
            color=TEXT_MUTED)
    ax.set_xlim(0, 13)
    ax.set_ylim(0.2, 5.0)
    ax.set_aspect("equal")
    ax.axis("off")
    ax.set_title("ALCOA+: the FDA data integrity framework",
                 fontsize=14, fontweight="bold", color=TEXT_DARK, pad=10)
    plt.tight_layout()
    out = ASSETS / "chart_alcoa.png"
    plt.savefig(out, bbox_inches="tight", facecolor=WHITE)
    plt.close()
    print(f"wrote {out}")


def chart_warning_letters():
    fig, ax = plt.subplots(figsize=(12, 5.4), dpi=150)
    fig.patch.set_facecolor(WHITE)
    years = list(range(2014, 2025))
    letters = [48, 62, 85, 112, 138, 156, 142, 168, 195, 218, 241]
    bars = ax.bar(years, letters, color=CORAL, edgecolor=MID,
                  linewidth=1.2)
    for bar, v in zip(bars, letters):
        ax.text(bar.get_x()+bar.get_width()/2, bar.get_height()+3,
                f"{v}", ha="center", va="bottom", fontsize=10,
                fontweight="bold", color=TEXT_DARK)
    ax.set_ylabel("FDA 483 data integrity observations per year",
                  fontsize=11)
    ax.set_xlabel("Year", fontsize=11)
    ax.set_title(
        "FDA data integrity observations: 5x growth over decade",
        fontsize=14, fontweight="bold", color=TEXT_DARK, pad=12)
    ax.text(2015, 230,
            "Source: FDA 483 publications, annualized data integrity "
            "citations",
            fontsize=9, style="italic", color=TEXT_MUTED)
    _light(ax)
    plt.tight_layout()
    out = ASSETS / "chart_fda_letters.png"
    plt.savefig(out, bbox_inches="tight", facecolor=WHITE)
    plt.close()
    print(f"wrote {out}")


def chart_trial_parties():
    fig, ax = plt.subplots(figsize=(13, 5.4), dpi=150)
    fig.patch.set_facecolor(WHITE)

    def box(x, y, text, color=MID, w=2.0, h=0.75, tc=WHITE, fs=10):
        rect = FancyBboxPatch((x-w/2, y-h/2), w, h,
                              boxstyle="round,pad=0.02,rounding_size=0.1",
                              facecolor=color, edgecolor=MID, linewidth=1.4)
        ax.add_patch(rect)
        ax.text(x, y, text, ha="center", va="center",
                fontsize=fs, fontweight="bold", color=tc)

    box(1.5, 4.3, "Sponsor\n(pharma co)", TEAL, w=2.2)
    box(1.5, 2.5, "CRO\n(contract research)", TEAL, w=2.2)
    box(5, 4.3, "Clinical site\n(investigator)", EMERALD, w=2.2)
    box(5, 2.5, "Site monitor\n(CRA)", EMERALD, w=2.2)
    box(8.5, 4.3, "Subject\n(patient)", AMBER, w=2.2, tc=WHITE)
    box(8.5, 2.5, "Central lab", AMBER, w=2.2, tc=WHITE)
    box(12, 4.3, "Regulator\n(FDA/EMA)", CORAL, w=2.2)
    box(12, 2.5, "IRB / ethics\ncommittee", CORAL, w=2.2)

    # Arrows between parties
    connections = [
        ((1.5, 4.0), (1.5, 2.85)),   # Sponsor-CRO
        ((2.6, 4.3), (3.9, 4.3)),    # Sponsor-site
        ((2.6, 2.5), (3.9, 2.5)),    # CRO-monitor
        ((5, 4.0), (5, 2.85)),       # Site-monitor
        ((6.1, 4.3), (7.4, 4.3)),    # Site-subject
        ((6.1, 2.5), (7.4, 2.5)),    # Monitor-lab
        ((9.6, 4.3), (10.9, 4.3)),   # Subject-regulator
        ((9.6, 2.5), (10.9, 2.5)),   # Lab-IRB
    ]
    for p1, p2 in connections:
        a = FancyArrowPatch(p1, p2, arrowstyle="<->",
                            mutation_scale=12, linewidth=1.0,
                            color=MID, alpha=0.6)
        ax.add_patch(a)

    ax.text(6.5, 0.8,
            "8+ parties per trial. Current systems require trust in "
            "the sponsor to aggregate data honestly.",
            ha="center", va="center", fontsize=11, fontweight="bold",
            color=TEXT_DARK)
    ax.set_xlim(0, 13.5)
    ax.set_ylim(0.4, 5)
    ax.set_aspect("equal")
    ax.axis("off")
    ax.set_title("A clinical trial is an 8-party data flow",
                 fontsize=14, fontweight="bold", color=TEXT_DARK, pad=10)
    plt.tight_layout()
    out = ASSETS / "chart_trial_parties.png"
    plt.savefig(out, bbox_inches="tight", facecolor=WHITE)
    plt.close()
    print(f"wrote {out}")


def chart_fraud_cases():
    fig, ax = plt.subplots(figsize=(12, 5.2), dpi=150)
    fig.patch.set_facecolor(WHITE)
    cases = [
        ("Ranbaxy 2008-13", "Systematic data fabrication\n$500M settlement + FDA import ban"),
        ("Theranos 2015-22", "Fake blood test results\nCEO 11-year sentence"),
        ("Olympus 2011", "$1.7B accounting fraud\nUS listing investigation"),
        ("Valeant/Philidor 2015", "Specialty pharma fraud\nStock lost 90% value"),
        ("Duke Potti 2006-10", "Cancer trial data fabrication\n11 clinical trials retracted"),
    ]
    for i, (case, desc) in enumerate(cases):
        y = 5.2 - i * 0.9
        ax.text(0.3, y, case, ha="left", va="center",
                fontsize=12, fontweight="bold", color=CORAL)
        ax.text(4.0, y, desc, ha="left", va="center",
                fontsize=11, color=TEXT_DARK)
    ax.text(6, -0.4,
            "Each would have been structurally harder with multi-"
            "party cryptographic signatures.",
            ha="center", va="center", fontsize=11, fontweight="bold",
            color=DARK_BG,
            bbox=dict(facecolor=TEAL_SOFT, edgecolor=TEAL,
                      boxstyle="round,pad=0.4"))
    ax.set_xlim(0, 12)
    ax.set_ylim(-0.8, 5.4)
    ax.set_aspect("equal")
    ax.axis("off")
    ax.set_title("Major pharma / medical fraud cases of past 20 years",
                 fontsize=14, fontweight="bold", color=TEXT_DARK, pad=10)
    plt.tight_layout()
    out = ASSETS / "chart_fraud.png"
    plt.savefig(out, bbox_inches="tight", facecolor=WHITE)
    plt.close()
    print(f"wrote {out}")


def chart_data_flow():
    fig, ax = plt.subplots(figsize=(13, 5.4), dpi=150)
    fig.patch.set_facecolor(WHITE)

    def box(x, y, text, color, w=2.0, h=0.65, tc=WHITE, fs=10):
        rect = FancyBboxPatch((x-w/2, y-h/2), w, h,
                              boxstyle="round,pad=0.02,rounding_size=0.1",
                              facecolor=color, edgecolor=MID, linewidth=1.4)
        ax.add_patch(rect)
        ax.text(x, y, text, ha="center", va="center",
                fontsize=fs, fontweight="bold", color=tc)

    # Signed chain
    nodes = [
        (1.3, "Subject\nconsent\n(signed)", TEAL),
        (3.5, "Vital signs\nrecorded\n(device+nurse)", TEAL),
        (5.7, "Lab result\n(instrument+\ntechnician)", TEAL),
        (7.9, "Investigator\nverifies\n(physician)", TEAL),
        (10.1, "Monitor\nreviews\n(CRA)", TEAL),
        (12.3, "Sponsor\ndatabase\n(signed lock)", EMERALD),
    ]
    for x, text, color in nodes:
        box(x, 3.5, text, color, w=1.85, h=1.0, fs=9)
    # Arrows
    for i in range(len(nodes) - 1):
        x1 = nodes[i][0] + 0.95
        x2 = nodes[i+1][0] - 0.95
        a = FancyArrowPatch((x1, 3.5), (x2, 3.5),
                            arrowstyle="->", mutation_scale=15,
                            linewidth=1.4, color=MID)
        ax.add_patch(a)

    ax.text(6.5, 1.5,
            "Each hop signed. Downstream parties verify upstream chain. "
            "No party can silently alter earlier records.",
            ha="center", va="center", fontsize=11, style="italic",
            color=TEXT_MUTED,
            bbox=dict(facecolor=WHITE, edgecolor=TEAL,
                      boxstyle="round,pad=0.4"))

    ax.set_xlim(0, 14)
    ax.set_ylim(0.5, 5)
    ax.set_aspect("equal")
    ax.axis("off")
    ax.set_title("Tamper-evident event stream for a trial subject",
                 fontsize=14, fontweight="bold", color=TEXT_DARK, pad=10)
    plt.tight_layout()
    out = ASSETS / "chart_data_flow.png"
    plt.savefig(out, bbox_inches="tight", facecolor=WHITE)
    plt.close()
    print(f"wrote {out}")


def chart_inspection_workflow():
    fig, ax = plt.subplots(figsize=(13, 5.0), dpi=150)
    fig.patch.set_facecolor(WHITE)
    # Current vs substrate-enabled
    current = [
        "Travel to site",
        "Request paper\n+ EDC access",
        "Manually compare\nsources",
        "Write observation\nfindings",
    ]
    future = [
        "Remote chain\nverification",
        "Automated SDV\nexception report",
        "Focus on real\ndiscrepancies",
        "Continuous\nmonitoring",
    ]
    # Current row
    for i, item in enumerate(current):
        x = 0.5 + i * 3
        rect = FancyBboxPatch((x, 3.5), 2.5, 0.9,
                              boxstyle="round,pad=0.05,rounding_size=0.1",
                              facecolor=CORAL, edgecolor=MID, linewidth=1.2)
        ax.add_patch(rect)
        ax.text(x + 1.25, 3.95, item, ha="center", va="center",
                fontsize=10, fontweight="bold", color=WHITE)
        if i < len(current) - 1:
            a = FancyArrowPatch((x + 2.5, 3.95), (x + 3, 3.95),
                                arrowstyle="->", mutation_scale=14,
                                linewidth=1.4, color=CORAL)
            ax.add_patch(a)
    ax.text(0.3, 4.7, "Current FDA inspection (person-weeks per site)",
            fontsize=12, fontweight="bold", color=CORAL)
    # Future row
    for i, item in enumerate(future):
        x = 0.5 + i * 3
        rect = FancyBboxPatch((x, 1.5), 2.5, 0.9,
                              boxstyle="round,pad=0.05,rounding_size=0.1",
                              facecolor=EMERALD, edgecolor=MID, linewidth=1.2)
        ax.add_patch(rect)
        ax.text(x + 1.25, 1.95, item, ha="center", va="center",
                fontsize=10, fontweight="bold", color=WHITE)
        if i < len(future) - 1:
            a = FancyArrowPatch((x + 2.5, 1.95), (x + 3, 1.95),
                                arrowstyle="->", mutation_scale=14,
                                linewidth=1.4, color=EMERALD)
            ax.add_patch(a)
    ax.text(0.3, 2.7,
            "Substrate-enabled inspection (hours per trial)",
            fontsize=12, fontweight="bold", color=EMERALD)
    ax.set_xlim(0, 14)
    ax.set_ylim(0.8, 5.2)
    ax.set_aspect("equal")
    ax.axis("off")
    ax.set_title(
        "FDA inspection workflow: current vs substrate-enabled",
        fontsize=14, fontweight="bold", color=TEXT_DARK, pad=10)
    plt.tight_layout()
    out = ASSETS / "chart_inspection.png"
    plt.savefig(out, bbox_inches="tight", facecolor=WHITE)
    plt.close()
    print(f"wrote {out}")


def chart_economics():
    fig, ax = plt.subplots(figsize=(12, 5.0), dpi=150)
    fig.patch.set_facecolor(WHITE)
    cats = ["Trial data\nmanagement\n(current)",
            "Trial data\nmanagement\n(Quidnug)",
            "Avg warning letter\nremediation cost",
            "Savings per\nlarge trial"]
    values = [8500000, 5500000, 2500000, -3000000]
    colors = [CORAL, EMERALD, CORAL, TEAL]
    bars = ax.bar(cats, values, color=colors,
                  edgecolor=MID, linewidth=1.2)
    for bar, v in zip(bars, values):
        if v < 0:
            label = f"-${abs(v)/1000000:.1f}M"
        else:
            label = f"${v/1000000:.1f}M"
        y = bar.get_height() if v > 0 else bar.get_height() - 100000
        ax.text(bar.get_x()+bar.get_width()/2, y,
                label, ha="center",
                va="bottom" if v > 0 else "top",
                fontsize=12, fontweight="bold", color=TEXT_DARK)
    ax.set_yscale("symlog")
    ax.set_ylabel("USD (symlog)", fontsize=11)
    ax.set_title(
        "Economic comparison per large clinical trial",
        fontsize=14, fontweight="bold", color=TEXT_DARK, pad=12)
    _light(ax)
    plt.setp(ax.get_xticklabels(), fontsize=9)
    plt.tight_layout()
    out = ASSETS / "chart_economics.png"
    plt.savefig(out, bbox_inches="tight", facecolor=WHITE)
    plt.close()
    print(f"wrote {out}")


def chart_compliance_mapping():
    fig, ax = plt.subplots(figsize=(13, 5.0), dpi=150)
    fig.patch.set_facecolor(WHITE)
    mappings = [
        ("21 CFR Part 11", "Electronic records + signatures",
         "Signed events + crypto timestamps", TEAL),
        ("ALCOA+", "9 data integrity principles",
         "Signed chain naturally enforces all 9", EMERALD),
        ("ICH GCP E6(R3)", "Good Clinical Practice",
         "Cross-party attestation workflows", AMBER),
        ("EMA Annex 11", "Computerized systems validation",
         "Built-in audit trail + validation", "#7C3AED"),
        ("GAMP 5", "Pharmaceutical software validation",
         "Reproducibility built into substrate", CORAL),
    ]
    for i, (regulation, scope, approach, color) in enumerate(mappings):
        y = 4.4 - i * 0.9
        rect = FancyBboxPatch((0.3, y - 0.35), 2.8, 0.7,
                              boxstyle="round,pad=0.04,rounding_size=0.08",
                              facecolor=color, edgecolor=MID, linewidth=1.2)
        ax.add_patch(rect)
        ax.text(1.7, y, regulation, ha="center", va="center",
                fontsize=11, fontweight="bold", color=WHITE)
        ax.text(3.4, y + 0.15, scope, ha="left", va="center",
                fontsize=10, fontweight="bold", color=TEXT_DARK)
        ax.text(3.4, y - 0.15, approach, ha="left", va="center",
                fontsize=10, color=TEXT_MUTED, style="italic")
    ax.set_xlim(0, 13)
    ax.set_ylim(0, 5.0)
    ax.set_aspect("equal")
    ax.axis("off")
    ax.set_title(
        "Compatibility with existing pharmaceutical compliance frameworks",
        fontsize=14, fontweight="bold", color=TEXT_DARK, pad=10)
    plt.tight_layout()
    out = ASSETS / "chart_compliance.png"
    plt.savefig(out, bbox_inches="tight", facecolor=WHITE)
    plt.close()
    print(f"wrote {out}")


def chart_adoption_curve():
    fig, ax = plt.subplots(figsize=(12, 5.0), dpi=150)
    fig.patch.set_facecolor(WHITE)
    years = list(range(2026, 2034))
    early = [2, 8, 18, 28, 38, 48, 58, 68]
    major = [0, 1, 4, 10, 20, 35, 52, 68]
    mainstream = [0, 0, 1, 4, 10, 18, 32, 50]
    ax.plot(years, early, color=TEAL, linewidth=2.5, marker="o",
            markersize=6, label="Early adopters (small CROs + biotech)")
    ax.plot(years, major, color=EMERALD, linewidth=2.5, marker="s",
            markersize=6, label="Major sponsors (top 20 pharma)")
    ax.plot(years, mainstream, color=AMBER, linewidth=2.5, marker="^",
            markersize=6, label="Mainstream adoption (all regulated)")
    ax.set_xlabel("Year", fontsize=11)
    ax.set_ylabel("% of trials using substrate", fontsize=11)
    ax.set_title("Projected adoption curves",
                 fontsize=14, fontweight="bold", color=TEXT_DARK, pad=12)
    ax.legend(loc="upper left", fontsize=10)
    ax.set_ylim(0, 100)
    _light(ax)
    plt.tight_layout()
    out = ASSETS / "chart_adoption.png"
    plt.savefig(out, bbox_inches="tight", facecolor=WHITE)
    plt.close()
    print(f"wrote {out}")


def chart_integration_ecosystem():
    fig, ax = plt.subplots(figsize=(13, 5.0), dpi=150)
    fig.patch.set_facecolor(WHITE)
    # Center = Quidnug
    cx, cy = 6.5, 2.5
    c = Circle((cx, cy), 0.7, facecolor=TEAL, edgecolor=MID,
               linewidth=2)
    ax.add_patch(c)
    ax.text(cx, cy, "Quidnug\nsubstrate", ha="center", va="center",
            fontsize=10, fontweight="bold", color=WHITE)
    # Surrounding systems
    systems = [
        (1.5, 4, "EDC:\nMedidata Rave"),
        (3.5, 4.5, "EDC:\nVeeva Vault"),
        (6, 4.8, "EDC:\nOracle InForm"),
        (8.5, 4.5, "EDC:\nCastor"),
        (11, 4, "CTMS systems"),
        (1, 1.5, "eSource\nhandhelds"),
        (3, 0.8, "Central lab\nLIMS"),
        (6, 0.5, "Imaging:\nDICOM"),
        (9, 0.8, "ePRO\nmobile apps"),
        (11.5, 1.5, "FDA\nESG"),
    ]
    for x, y, label in systems:
        rect = FancyBboxPatch((x - 0.6, y - 0.25), 1.2, 0.5,
                              boxstyle="round,pad=0.04,rounding_size=0.08",
                              facecolor=MID, edgecolor=MID, linewidth=1.2)
        ax.add_patch(rect)
        ax.text(x, y, label, ha="center", va="center",
                fontsize=9, fontweight="bold", color=WHITE)
        # line to center
        ax.plot([x, cx], [y, cy], color=TEAL, linewidth=1.0, alpha=0.5,
                zorder=0)
    ax.set_xlim(0, 13)
    ax.set_ylim(0, 5.3)
    ax.set_aspect("equal")
    ax.axis("off")
    ax.set_title(
        "Integration ecosystem: Quidnug composes with existing infrastructure",
        fontsize=14, fontweight="bold", color=TEXT_DARK, pad=10)
    plt.tight_layout()
    out = ASSETS / "chart_ecosystem.png"
    plt.savefig(out, bbox_inches="tight", facecolor=WHITE)
    plt.close()
    print(f"wrote {out}")


if __name__ == "__main__":
    chart_alcoa()
    chart_warning_letters()
    chart_trial_parties()
    chart_fraud_cases()
    chart_data_flow()
    chart_inspection_workflow()
    chart_economics()
    chart_compliance_mapping()
    chart_adoption_curve()
    chart_integration_ecosystem()
    print("done.")
