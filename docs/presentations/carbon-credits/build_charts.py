"""Charts for Carbon Credits deck."""
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


def chart_phantom_credits():
    fig, ax = plt.subplots(figsize=(12, 5.2), dpi=150)
    fig.patch.set_facecolor(WHITE)
    # West et al 2023 findings
    cats = ["Claimed avoided\ndeforestation", "Actually avoided\n(measured)"]
    values = [100, 6]
    bars = ax.bar(cats, values, color=[CORAL, EMERALD],
                  edgecolor=MID, linewidth=1.5, width=0.55)
    for bar, v in zip(bars, values):
        ax.text(bar.get_x()+bar.get_width()/2, bar.get_height()+2,
                f"{v}%", ha="center", va="bottom", fontsize=28,
                fontweight="bold", color=TEXT_DARK)
    ax.set_ylim(0, 115)
    ax.set_ylabel("% of total claimed emission reductions", fontsize=12)
    ax.set_title(
        "West et al. 2023: 94% of REDD+ credits are phantom",
        fontsize=14, fontweight="bold", color=TEXT_DARK, pad=12)
    ax.text(0.5, 105,
            "Source: West et al., 'Action needed to make carbon "
            "offsets from forest conservation work for climate "
            "change mitigation,' Science 381, 873-877 (2023).",
            fontsize=9, style="italic", color=TEXT_MUTED)
    _light(ax)
    plt.tight_layout()
    out = ASSETS / "chart_phantom.png"
    plt.savefig(out, bbox_inches="tight", facecolor=WHITE)
    plt.close()
    print(f"wrote {out}")


def chart_market_size():
    fig, ax = plt.subplots(figsize=(12, 5.2), dpi=150)
    fig.patch.set_facecolor(WHITE)
    years = list(range(2018, 2031))
    actual = [300, 380, 480, 1250, 2100, 920, 720, 780, None, None,
              None, None, None]
    projected = [None, None, None, None, None, 920, 720, 780, 900,
                 1100, 1400, 1700, 2000]
    optimistic = [None, None, None, None, None, 920, 720, 780,
                  1400, 2500, 4200, 6500, 10000]
    ax.plot(years[:8], actual[:8], color=TEAL, linewidth=2.8,
            marker="o", markersize=7, label="Actual (VCM size, $M)")
    ax.plot(years[5:], projected[5:], color=AMBER, linewidth=2.2,
            marker="s", markersize=5, linestyle="--",
            label="Current projection")
    ax.plot(years[5:], optimistic[5:], color=EMERALD, linewidth=2.2,
            marker="^", markersize=5, linestyle="--",
            label="With integrity restoration")
    ax.set_xlabel("Year", fontsize=11)
    ax.set_ylabel("Voluntary carbon market ($M)", fontsize=11)
    ax.set_title(
        "VCM market size: growth, collapse, and two possible futures",
        fontsize=14, fontweight="bold", color=TEXT_DARK, pad=12)
    ax.legend(loc="upper left", fontsize=10)
    _light(ax)
    plt.tight_layout()
    out = ASSETS / "chart_market.png"
    plt.savefig(out, bbox_inches="tight", facecolor=WHITE)
    plt.close()
    print(f"wrote {out}")


def chart_credit_lifecycle():
    fig, ax = plt.subplots(figsize=(13, 5.2), dpi=150)
    fig.patch.set_facecolor(WHITE)
    stages = [
        ("Project\ndeveloper", "Proposes\nemission\nreduction", TEAL),
        ("Methodology\nchosen", "From VCS,\nACR, CAR, etc",
         TEAL),
        ("Validator\n(VVB)", "Design\nvalidated", AMBER),
        ("Implementation", "Project\noperates", AMBER),
        ("Verifier\n(same VVB)", "Measures\nreductions", CORAL),
        ("Registry\n(Verra, etc)", "Issues\ncredits", CORAL),
        ("Buyer", "Purchases\n(or retires)", EMERALD),
    ]
    cell_w = 1.7
    for i, (who, what, color) in enumerate(stages):
        x = 0.3 + i * (cell_w + 0.18)
        # header box
        rect = FancyBboxPatch((x, 3.5), cell_w, 0.7,
                              boxstyle="round,pad=0.04,rounding_size=0.1",
                              facecolor=color, edgecolor=MID, linewidth=1.3)
        ax.add_patch(rect)
        ax.text(x + cell_w/2, 3.85, who, ha="center", va="center",
                fontsize=10, fontweight="bold", color=WHITE)
        # body
        rect = FancyBboxPatch((x, 1.7), cell_w, 1.5,
                              boxstyle="round,pad=0.04,rounding_size=0.08",
                              facecolor=LIGHT_BG, edgecolor=color, linewidth=1.2)
        ax.add_patch(rect)
        ax.text(x + cell_w/2, 2.45, what, ha="center", va="center",
                fontsize=10, color=TEXT_DARK)
        if i < len(stages) - 1:
            x_arr = x + cell_w + 0.02
            ax.annotate("", xy=(x_arr + 0.12, 3.85),
                        xytext=(x_arr, 3.85),
                        arrowprops=dict(arrowstyle="->", color=MID,
                                        lw=1.3))
    ax.text(6.5, 0.7,
            "Paid by the project developer to verify them. "
            "Conflict of interest is structural.",
            ha="center", va="center", fontsize=11, fontweight="bold",
            color=CORAL,
            bbox=dict(facecolor="#FEF2F2", edgecolor=CORAL,
                      boxstyle="round,pad=0.4"))
    ax.set_xlim(0, 13.5)
    ax.set_ylim(0.2, 4.5)
    ax.set_aspect("equal")
    ax.axis("off")
    ax.set_title(
        "Carbon credit lifecycle: who signs off at each step",
        fontsize=14, fontweight="bold", color=TEXT_DARK, pad=10)
    plt.tight_layout()
    out = ASSETS / "chart_lifecycle.png"
    plt.savefig(out, bbox_inches="tight", facecolor=WHITE)
    plt.close()
    print(f"wrote {out}")


def chart_integrity_dimensions():
    fig, ax = plt.subplots(figsize=(13, 5.0), dpi=150)
    fig.patch.set_facecolor(WHITE)
    dims = [
        ("Additionality",
         "Would reduction have\nhappened anyway?", TEAL),
        ("Permanence",
         "Does the reduction\nstay removed?", EMERALD),
        ("Leakage",
         "Does it displace\nemissions elsewhere?", AMBER),
        ("Measurement\naccuracy",
         "Is the claimed\namount correct?", CORAL),
    ]
    cell_w = 3.0
    for i, (name, body, color) in enumerate(dims):
        x = 0.4 + i * (cell_w + 0.2)
        circ = Circle((x + cell_w/2, 4.1), 0.45, facecolor=color,
                      edgecolor=MID, linewidth=1.5)
        ax.add_patch(circ)
        ax.text(x + cell_w/2, 4.1, str(i+1), ha="center", va="center",
                fontsize=22, fontweight="bold", color=WHITE)
        ax.text(x + cell_w/2, 3.2, name, ha="center", va="center",
                fontsize=13, fontweight="bold", color=TEXT_DARK)
        rect = FancyBboxPatch((x, 1.0), cell_w, 1.8,
                              boxstyle="round,pad=0.04,rounding_size=0.1",
                              facecolor=LIGHT_BG, edgecolor=color, linewidth=1.5)
        ax.add_patch(rect)
        ax.text(x + cell_w/2, 1.9, body, ha="center", va="center",
                fontsize=11, color=TEXT_DARK)
    ax.text(6.7, 0.3,
            "All four required. Failing any one makes the credit a "
            "phantom.",
            ha="center", va="center", fontsize=11, fontweight="bold",
            color=DARK_BG)
    ax.set_xlim(0, 13.5)
    ax.set_ylim(0, 5.0)
    ax.set_aspect("equal")
    ax.axis("off")
    ax.set_title("The four integrity dimensions of a carbon credit",
                 fontsize=15, fontweight="bold", color=TEXT_DARK, pad=10)
    plt.tight_layout()
    out = ASSETS / "chart_dimensions.png"
    plt.savefig(out, bbox_inches="tight", facecolor=WHITE)
    plt.close()
    print(f"wrote {out}")


def chart_attestation_stack():
    fig, ax = plt.subplots(figsize=(13, 5.0), dpi=150)
    fig.patch.set_facecolor(WHITE)

    def box(x, y, text, color, w=2.0, h=0.6, tc=WHITE, fs=10):
        rect = FancyBboxPatch((x - w/2, y - h/2), w, h,
                              boxstyle="round,pad=0.02,rounding_size=0.1",
                              facecolor=color, edgecolor=MID, linewidth=1.4)
        ax.add_patch(rect)
        ax.text(x, y, text, ha="center", va="center",
                fontsize=fs, fontweight="bold", color=tc)

    # Layer 1: project data
    box(1.5, 4.5, "Sensor data\n(satellites)", MID, h=0.8)
    box(4.0, 4.5, "Field\nmeasurements", MID, h=0.8)
    box(6.5, 4.5, "Analytical\nmodels", MID, h=0.8)
    box(9.0, 4.5, "Registry\nissuance", MID, h=0.8)
    box(11.5, 4.5, "Buyer\nverification", MID, h=0.8)

    # Layer 2: independent attestations
    box(1.5, 2.0, "NASA /\nESA satellite\nattestation", EMERALD, h=0.9)
    box(4.0, 2.0, "Independent\nfield scientist", EMERALD, h=0.9)
    box(6.5, 2.0, "Peer-reviewed\nmodels", EMERALD, h=0.9)
    box(9.0, 2.0, "Independent\nauditor", EMERALD, h=0.9)
    box(11.5, 2.0, "Trust graph\nevaluation", EMERALD, h=0.9)

    # Vertical arrows
    for x in [1.5, 4.0, 6.5, 9.0, 11.5]:
        a = FancyArrowPatch((x, 4.05), (x, 2.45), arrowstyle="<->",
                            mutation_scale=13, linewidth=1.2,
                            color=TEAL, alpha=0.7)
        ax.add_patch(a)

    ax.text(0.2, 4.5, "Primary\nsource", ha="left", va="center",
            fontsize=11, fontweight="bold", color=MID)
    ax.text(0.2, 2.0, "Independent\nattestation", ha="left",
            va="center", fontsize=11, fontweight="bold", color=EMERALD)
    ax.text(6.5, 0.5,
            "Each step cross-signed by an independent party. No single "
            "verifier can approve alone.",
            ha="center", va="center", fontsize=11, fontweight="bold",
            color=DARK_BG,
            bbox=dict(facecolor=TEAL_SOFT, edgecolor=TEAL,
                      boxstyle="round,pad=0.3"))

    ax.set_xlim(0, 13.5)
    ax.set_ylim(0, 5.2)
    ax.set_aspect("equal")
    ax.axis("off")
    ax.set_title(
        "Multi-party attestation stack for a carbon credit",
        fontsize=14, fontweight="bold", color=TEXT_DARK, pad=10)
    plt.tight_layout()
    out = ASSETS / "chart_attestation.png"
    plt.savefig(out, bbox_inches="tight", facecolor=WHITE)
    plt.close()
    print(f"wrote {out}")


def chart_auditor_reputation():
    fig, ax = plt.subplots(figsize=(11, 5.0), dpi=150)
    fig.patch.set_facecolor(WHITE)
    auditors = ["SCS Global\n(forestry)", "DNV\n(renewables)",
                "TUV Sud\n(industrial)",
                "4C\n(small projects)", "Audit company X\n(REDD+)"]
    scores = [0.85, 0.92, 0.88, 0.73, 0.32]
    colors = [EMERALD if s > 0.75 else
              AMBER if s > 0.5 else CORAL for s in scores]
    bars = ax.bar(auditors, scores, color=colors, edgecolor=MID,
                  linewidth=1.2, width=0.55)
    for bar, v in zip(bars, scores):
        ax.text(bar.get_x()+bar.get_width()/2, bar.get_height()+0.02,
                f"{v:.2f}", ha="center", va="bottom",
                fontsize=12, fontweight="bold", color=TEXT_DARK)
    ax.set_ylim(0, 1.1)
    ax.set_ylabel("Credit durability score (post-issuance verification)",
                  fontsize=11)
    ax.set_title(
        "Auditor reputation: how often does an auditor's "
        "claims survive post-issuance verification?",
        fontsize=13, fontweight="bold", color=TEXT_DARK, pad=12)
    _light(ax)
    plt.setp(ax.get_xticklabels(), fontsize=10)
    plt.tight_layout()
    out = ASSETS / "chart_auditor_rep.png"
    plt.savefig(out, bbox_inches="tight", facecolor=WHITE)
    plt.close()
    print(f"wrote {out}")


def chart_four_dimensions_compliance():
    fig, ax = plt.subplots(figsize=(13, 5.0), dpi=150)
    fig.patch.set_facecolor(WHITE)
    projects = ["Forest proj A\n(REDD+)",
                "Wind farm B\n(renewable)",
                "Cookstove C\n(household)",
                "Biochar D\n(industrial)",
                "Avoided defor E\n(REDD+)"]
    add_score = [0.3, 0.92, 0.6, 0.8, 0.2]
    perm_score = [0.4, 0.95, 0.9, 0.88, 0.35]
    leak_score = [0.25, 0.9, 0.75, 0.85, 0.2]
    meas_score = [0.45, 0.95, 0.65, 0.9, 0.3]
    x = np.arange(len(projects))
    w = 0.2
    ax.bar(x - 1.5*w, add_score, w, label="Additionality",
           color=TEAL, edgecolor=MID, linewidth=0.7)
    ax.bar(x - 0.5*w, perm_score, w, label="Permanence",
           color=EMERALD, edgecolor=MID, linewidth=0.7)
    ax.bar(x + 0.5*w, leak_score, w, label="Leakage",
           color=AMBER, edgecolor=MID, linewidth=0.7)
    ax.bar(x + 1.5*w, meas_score, w, label="Measurement",
           color=CORAL, edgecolor=MID, linewidth=0.7)
    ax.axhline(0.7, color=TEXT_MUTED, linestyle="--", alpha=0.5)
    ax.text(len(projects)-1, 0.72, "Integrity threshold",
            fontsize=9, color=TEXT_MUTED)
    ax.set_xticks(x)
    ax.set_xticklabels(projects, fontsize=10)
    ax.set_ylim(0, 1.1)
    ax.set_ylabel("Score (0-1)", fontsize=11)
    ax.set_title(
        "Four-dimension scoring: which projects meet integrity bar?",
        fontsize=14, fontweight="bold", color=TEXT_DARK, pad=12)
    ax.legend(loc="upper right", fontsize=10, ncol=2)
    _light(ax)
    plt.tight_layout()
    out = ASSETS / "chart_dimensions_scoring.png"
    plt.savefig(out, bbox_inches="tight", facecolor=WHITE)
    plt.close()
    print(f"wrote {out}")


def chart_icvcm_vcmi():
    fig, ax = plt.subplots(figsize=(13, 5.0), dpi=150)
    fig.patch.set_facecolor(WHITE)
    # ICVCM and VCMI boxes
    # ICVCM
    rect = FancyBboxPatch((0.5, 2.5), 5.5, 2.0,
                          boxstyle="round,pad=0.05,rounding_size=0.15",
                          facecolor=LIGHT_BG, edgecolor=TEAL, linewidth=2)
    ax.add_patch(rect)
    ax.text(3.25, 4.2, "ICVCM Core Carbon Principles (CCP)",
            ha="center", va="center", fontsize=13,
            fontweight="bold", color=TEAL)
    ax.text(3.25, 3.6, "10 principles for high-integrity credits",
            ha="center", va="center", fontsize=11, color=TEXT_DARK)
    ax.text(3.25, 3.1, "Supply-side quality standard",
            ha="center", va="center", fontsize=10, color=TEXT_MUTED,
            style="italic")
    ax.text(3.25, 2.8, "Launched 2023. ~200 methodologies assessed.",
            ha="center", va="center", fontsize=10, color=TEXT_DARK)

    # VCMI
    rect = FancyBboxPatch((7.5, 2.5), 5.5, 2.0,
                          boxstyle="round,pad=0.05,rounding_size=0.15",
                          facecolor=LIGHT_BG, edgecolor=EMERALD, linewidth=2)
    ax.add_patch(rect)
    ax.text(10.25, 4.2, "VCMI Claims Code of Practice",
            ha="center", va="center", fontsize=13,
            fontweight="bold", color=EMERALD)
    ax.text(10.25, 3.6, "Tiered buyer claims: Platinum, Gold, Silver",
            ha="center", va="center", fontsize=11, color=TEXT_DARK)
    ax.text(10.25, 3.1, "Demand-side integrity standard",
            ha="center", va="center", fontsize=10, color=TEXT_MUTED,
            style="italic")
    ax.text(10.25, 2.8, "Launched 2023. Aligned with ICVCM supply side.",
            ha="center", va="center", fontsize=10, color=TEXT_DARK)

    # Quidnug below
    rect = FancyBboxPatch((4, 0.3), 5.5, 1.8,
                          boxstyle="round,pad=0.05,rounding_size=0.15",
                          facecolor=DARK_BG, edgecolor=MID, linewidth=2)
    ax.add_patch(rect)
    ax.text(6.75, 1.75, "Quidnug substrate",
            ha="center", va="center", fontsize=13,
            fontweight="bold", color=TEAL)
    ax.text(6.75, 1.2, "Cryptographic attestation + trust-graph",
            ha="center", va="center", fontsize=11, color=WHITE)
    ax.text(6.75, 0.7, "Implements BOTH standards via signatures",
            ha="center", va="center", fontsize=10, color=TEAL_SOFT,
            style="italic")

    # Arrows
    a = FancyArrowPatch((3.25, 2.4), (6, 2.2), arrowstyle="->",
                        mutation_scale=16, linewidth=1.5, color=MID)
    ax.add_patch(a)
    a = FancyArrowPatch((10.25, 2.4), (7.5, 2.2), arrowstyle="->",
                        mutation_scale=16, linewidth=1.5, color=MID)
    ax.add_patch(a)

    ax.set_xlim(0, 14)
    ax.set_ylim(0, 5.0)
    ax.set_aspect("equal")
    ax.axis("off")
    ax.set_title(
        "How Quidnug composes with ICVCM + VCMI standards",
        fontsize=14, fontweight="bold", color=TEXT_DARK, pad=10)
    plt.tight_layout()
    out = ASSETS / "chart_icvcm.png"
    plt.savefig(out, bbox_inches="tight", facecolor=WHITE)
    plt.close()
    print(f"wrote {out}")


def chart_worked_example():
    fig, ax = plt.subplots(figsize=(13, 5.2), dpi=150)
    fig.patch.set_facecolor(WHITE)
    # REDD+ credit batch evaluation
    stages = [
        ("Project\nclaims", "50,000 tCO2\nover 3 years", MID),
        ("Satellite\ndata", "NASA MODIS\nsigned", EMERALD),
        ("Ground\ntruth", "Independent field\nteam signed", EMERALD),
        ("Additionality\nanalysis", "Counterfactual modeling\nSIGNED", EMERALD),
        ("Auditor\nreview", "Previously delivered\n0.73 integrity", AMBER),
        ("Buyer\ntrust query", "Adjusted: 32,000\ncredits at 0.78", TEAL),
    ]
    cell_w = 2.05
    for i, (step, body, color) in enumerate(stages):
        x = 0.2 + i * (cell_w + 0.1)
        rect = FancyBboxPatch((x, 3.7), cell_w, 0.7,
                              boxstyle="round,pad=0.04,rounding_size=0.1",
                              facecolor=color, edgecolor=MID, linewidth=1.3)
        ax.add_patch(rect)
        ax.text(x + cell_w/2, 4.05, step, ha="center", va="center",
                fontsize=10, fontweight="bold", color=WHITE)
        rect = FancyBboxPatch((x, 1.9), cell_w, 1.6,
                              boxstyle="round,pad=0.04,rounding_size=0.1",
                              facecolor=LIGHT_BG, edgecolor=color, linewidth=1.2)
        ax.add_patch(rect)
        ax.text(x + cell_w/2, 2.7, body, ha="center", va="center",
                fontsize=9, color=TEXT_DARK)
        if i < len(stages) - 1:
            x_arr = x + cell_w
            a = FancyArrowPatch((x_arr, 4.05), (x_arr + 0.1, 4.05),
                                arrowstyle="->", mutation_scale=14,
                                linewidth=1.2, color=MID)
            ax.add_patch(a)
    ax.text(6.5, 0.7,
            "50,000 claimed → 32,000 integrity-verified. 36% haircut "
            "based on cryptographic trust evaluation.",
            ha="center", va="center", fontsize=11, fontweight="bold",
            color=DARK_BG,
            bbox=dict(facecolor=TEAL_SOFT, edgecolor=TEAL,
                      boxstyle="round,pad=0.4"))
    ax.set_xlim(0, 13.5)
    ax.set_ylim(0.2, 4.6)
    ax.set_aspect("equal")
    ax.axis("off")
    ax.set_title(
        "Worked example: REDD+ credit batch evaluation",
        fontsize=14, fontweight="bold", color=TEXT_DARK, pad=10)
    plt.tight_layout()
    out = ASSETS / "chart_worked.png"
    plt.savefig(out, bbox_inches="tight", facecolor=WHITE)
    plt.close()
    print(f"wrote {out}")


if __name__ == "__main__":
    chart_phantom_credits()
    chart_market_size()
    chart_credit_lifecycle()
    chart_integrity_dimensions()
    chart_attestation_stack()
    chart_auditor_reputation()
    chart_four_dimensions_compliance()
    chart_icvcm_vcmi()
    chart_worked_example()
    print("done.")
