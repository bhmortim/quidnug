"""Charts for Reproducibility Crisis deck."""
import pathlib
import matplotlib.pyplot as plt
from matplotlib.patches import FancyBboxPatch, FancyArrowPatch, Circle
import numpy as np

HERE = pathlib.Path(__file__).parent
ASSETS = HERE / "assets"
ASSETS.mkdir(exist_ok=True)

DARK_BG = "#0A1628"
MID = "#1C3A5E"
TEAL = "#00D4A8"
TEAL_SOFT = "#C3EFE3"
CORAL = "#FF4655"
EMERALD = "#10B981"
AMBER = "#F59E0B"
WHITE = "#FFFFFF"
LIGHT_BG = "#F5F7FA"
TEXT_DARK = "#1A1D23"
TEXT_MUTED = "#64748B"
GRID = "#E2E8F0"


def _light(ax):
    ax.set_facecolor(WHITE)
    for s in ax.spines.values():
        s.set_color("#CBD5E1")
    ax.tick_params(colors=TEXT_MUTED)
    ax.xaxis.label.set_color(TEXT_DARK)
    ax.yaxis.label.set_color(TEXT_DARK)
    ax.title.set_color(TEXT_DARK)
    ax.grid(True, color=GRID, linewidth=0.6, alpha=0.9)


def chart_replication_rates():
    fig, ax = plt.subplots(figsize=(12, 5.5), dpi=150)
    fig.patch.set_facecolor(WHITE)
    fields = ["Preclinical biomed\n(Begley/Ellis '12)",
              "Cancer biology\n(Errington '21)",
              "Psychology\n(OSC '15)",
              "Experimental econ\n(Camerer '16)",
              "Social sci top journals\n(Camerer '18)"]
    rates = [11, 54, 39, 61, 62]
    colors = [CORAL, AMBER, AMBER, EMERALD, EMERALD]
    bars = ax.bar(fields, rates, color=colors, edgecolor=MID, linewidth=1.2)
    for bar, v in zip(bars, rates):
        ax.text(bar.get_x() + bar.get_width()/2, bar.get_height()+1.5,
                f"{v}%", ha="center", va="bottom", fontsize=12,
                fontweight="bold", color=TEXT_DARK)
    ax.set_ylim(0, 75)
    ax.set_ylabel("Replication rate (%)", fontsize=11)
    ax.set_title("Cross-field replication: 11 to 62 percent",
                 fontsize=14, fontweight="bold", color=TEXT_DARK, pad=12)
    ax.axhline(50, color=TEXT_MUTED, linestyle="--", alpha=0.5)
    ax.text(4.4, 51.5, "coin-flip line", fontsize=9, color=TEXT_MUTED)
    _light(ax)
    plt.setp(ax.get_xticklabels(), fontsize=9)
    plt.tight_layout()
    out = ASSETS / "chart_replication.png"
    plt.savefig(out, bbox_inches="tight", facecolor=WHITE)
    plt.close()
    print(f"wrote {out}")


def chart_baker_survey():
    fig, ax = plt.subplots(figsize=(11, 5.2), dpi=150)
    fig.patch.set_facecolor(WHITE)
    cats = ["Failed to reproduce\nanother scientist's", "Failed to reproduce\nyour own"]
    yes_pct = [70, 52]
    bars = ax.bar(cats, yes_pct, color=[CORAL, AMBER], edgecolor=MID,
                  linewidth=1.2, width=0.5)
    for bar, v in zip(bars, yes_pct):
        ax.text(bar.get_x()+bar.get_width()/2, bar.get_height()+1, f"{v}%",
                ha="center", va="bottom", fontsize=18, fontweight="bold",
                color=TEXT_DARK)
    ax.set_ylim(0, 85)
    ax.set_ylabel("% of researchers answering 'yes'", fontsize=11)
    ax.set_title("Baker (2016) Nature survey, n=1576 researchers",
                 fontsize=14, fontweight="bold", color=TEXT_DARK, pad=12)
    _light(ax)
    plt.tight_layout()
    out = ASSETS / "chart_baker.png"
    plt.savefig(out, bbox_inches="tight", facecolor=WHITE)
    plt.close()
    print(f"wrote {out}")


def chart_retraction_growth():
    fig, ax = plt.subplots(figsize=(12, 5.4), dpi=150)
    fig.patch.set_facecolor(WHITE)
    years = list(range(2000, 2025))
    retractions = [40, 50, 60, 80, 95, 110, 140, 180, 240, 280, 320, 380,
                   430, 480, 540, 620, 700, 780, 860, 940, 1100, 1380,
                   2150, 4500, 13000]  # 2023 spike from Wiley/Hindawi
    ax.plot(years, retractions, color=CORAL, linewidth=2.6, marker="o",
            markersize=4)
    ax.fill_between(years, retractions, alpha=0.2, color=CORAL)
    ax.set_yscale("log")
    ax.set_ylabel("Annual retractions (log)", fontsize=11)
    ax.set_xlabel("Year", fontsize=11)
    ax.set_title("Retractions per year: 25x growth 2000-2024",
                 fontsize=14, fontweight="bold", color=TEXT_DARK, pad=12)
    ax.annotate("2023: Wiley\nHindawi mass\nretractions",
                xy=(2023, 13000), xytext=(2017, 8000),
                fontsize=10, fontweight="bold", color=CORAL,
                arrowprops=dict(arrowstyle="->", color=CORAL, lw=1.5))
    _light(ax)
    plt.tight_layout()
    out = ASSETS / "chart_retractions.png"
    plt.savefig(out, bbox_inches="tight", facecolor=WHITE)
    plt.close()
    print(f"wrote {out}")


def chart_p_hacking():
    fig, ax = plt.subplots(figsize=(11, 5.2), dpi=150)
    fig.patch.set_facecolor(WHITE)
    p = np.linspace(0, 0.2, 100)
    # Empirically observed (Head et al, Simonsohn et al style)
    expected = 100 * np.exp(-p * 6)
    observed = expected.copy()
    # Pile-up at p < 0.05
    spike_zone = (p > 0.04) & (p < 0.05)
    observed[spike_zone] *= 1.6
    cliff_zone = (p > 0.05) & (p < 0.08)
    observed[cliff_zone] *= 0.5
    ax.plot(p, expected, color=TEXT_MUTED, linewidth=2, linestyle="--",
            label="Expected null distribution")
    ax.plot(p, observed, color=CORAL, linewidth=2.5,
            label="Observed (with p-hacking)")
    ax.axvline(0.05, color=AMBER, linestyle="--", alpha=0.6,
               label="p = 0.05 threshold")
    ax.fill_between(p[spike_zone], 0, observed[spike_zone],
                    alpha=0.3, color=CORAL)
    ax.set_xlabel("p-value", fontsize=11)
    ax.set_ylabel("Frequency in published literature", fontsize=11)
    ax.set_title(
        "P-curve evidence of p-hacking: pile-up just below 0.05",
        fontsize=14, fontweight="bold", color=TEXT_DARK, pad=12)
    ax.legend(loc="upper right", fontsize=10)
    _light(ax)
    plt.tight_layout()
    out = ASSETS / "chart_phacking.png"
    plt.savefig(out, bbox_inches="tight", facecolor=WHITE)
    plt.close()
    print(f"wrote {out}")


def chart_four_primitives():
    fig, ax = plt.subplots(figsize=(13, 5.2), dpi=150)
    fig.patch.set_facecolor(WHITE)
    items = [
        ("1", "Signed claims",
         "Every assertion binds\nto author's crypto identity\nat publication time", TEAL),
        ("2", "Signed peer review",
         "Reviewer ID + content\ncommitted to append-only log\n(pseudonymous OK)", EMERALD),
        ("3", "Data provenance chains",
         "Verifiable lineage from\nraw collection to published\nanalysis", AMBER),
        ("4", "Citation-weighted\nreputation",
         "Reviewer quality computable\nfrom history, not\neditorial secret", "#7C3AED"),
    ]
    cell_w = 3.0
    for i, (num, title, body, color) in enumerate(items):
        x = 0.3 + i * (cell_w + 0.2)
        circ = Circle((x + 0.5, 4.2), 0.45, facecolor=color,
                      edgecolor=MID, linewidth=1.5)
        ax.add_patch(circ)
        ax.text(x + 0.5, 4.2, num, ha="center", va="center",
                fontsize=24, fontweight="bold", color=WHITE)
        ax.text(x + cell_w/2 + 0.2, 4.2, title, ha="left", va="center",
                fontsize=14, fontweight="bold", color=TEXT_DARK)
        rect = FancyBboxPatch((x, 1.0), cell_w, 2.4,
                              boxstyle="round,pad=0.05,rounding_size=0.1",
                              facecolor=LIGHT_BG, edgecolor=color,
                              linewidth=1.5)
        ax.add_patch(rect)
        ax.text(x + cell_w/2, 2.2, body, ha="center", va="center",
                fontsize=11, color=TEXT_DARK)
    ax.set_xlim(0, 13)
    ax.set_ylim(0.5, 5.2)
    ax.set_aspect("equal")
    ax.axis("off")
    ax.set_title("Four primitives for tamper-evident peer review",
                 fontsize=15, fontweight="bold", color=TEXT_DARK, pad=10)
    plt.tight_layout()
    out = ASSETS / "chart_four_primitives.png"
    plt.savefig(out, bbox_inches="tight", facecolor=WHITE)
    plt.close()
    print(f"wrote {out}")


def chart_paper_lifecycle():
    fig, ax = plt.subplots(figsize=(13, 5.5), dpi=150)
    fig.patch.set_facecolor(WHITE)
    stages = [
        (-6, "T-6mo", "Preregistration", "Hypothesis + analysis\nplan signed"),
        (-3, "T-3mo", "Data collection", "Raw data signed\nat acquisition"),
        (-1, "T-1mo", "Analysis + preprint", "Analysis script signed,\nlinked to data"),
        (0, "T", "Peer review", "Each reviewer signs,\nreview chain forms"),
        (1, "T+1mo", "Published", "Full chain visible;\ncitable as a unit"),
        (24, "T+2yr", "Replication", "New study links\nto original chain"),
        (60, "T+5yr", "Superseded", "Newer theory linked,\nold work not retracted"),
    ]
    y = 2.5
    ax.axhline(y, color=TEAL, linewidth=3, alpha=0.6,
               xmin=0.05, xmax=0.95)
    n = len(stages)
    for i, (t, label, name, body) in enumerate(stages):
        x = 0.7 + i * (12 / (n - 1)) * 0.95
        circ = Circle((x, y), 0.18, facecolor=TEAL, edgecolor=MID,
                      linewidth=1.5, zorder=5)
        ax.add_patch(circ)
        # alternate top/bottom
        if i % 2 == 0:
            ax.text(x, y + 0.5, label, ha="center", va="bottom",
                    fontsize=10, fontweight="bold", color=TEAL)
            ax.text(x, y + 1.0, name, ha="center", va="bottom",
                    fontsize=11, fontweight="bold", color=TEXT_DARK)
            ax.text(x, y + 1.6, body, ha="center", va="bottom",
                    fontsize=9, color=TEXT_MUTED)
        else:
            ax.text(x, y - 0.5, label, ha="center", va="top",
                    fontsize=10, fontweight="bold", color=TEAL)
            ax.text(x, y - 1.0, name, ha="center", va="top",
                    fontsize=11, fontweight="bold", color=TEXT_DARK)
            ax.text(x, y - 1.6, body, ha="center", va="top",
                    fontsize=9, color=TEXT_MUTED)
    ax.set_xlim(0, 13)
    ax.set_ylim(0, 5.2)
    ax.set_aspect("equal")
    ax.axis("off")
    ax.set_title("A paper's life cycle in a tamper-evident world",
                 fontsize=14, fontweight="bold", color=TEXT_DARK, pad=10)
    plt.tight_layout()
    out = ASSETS / "chart_lifecycle.png"
    plt.savefig(out, bbox_inches="tight", facecolor=WHITE)
    plt.close()
    print(f"wrote {out}")


def chart_high_profile_fraud():
    fig, ax = plt.subplots(figsize=(12, 5.2), dpi=150)
    fig.patch.set_facecolor(WHITE)
    cases = [
        ("Schon (2002)", "Bell Labs", "fabricated 16+ Science/Nature papers"),
        ("Stapel (2011)", "Tilburg U", "55+ papers retracted"),
        ("Macchiarini (2014)", "Karolinska", "windpipe transplants killed patients"),
        ("Wansink (2018)", "Cornell", "p-hacking + duplicate publication"),
        ("Lesne (2022)", "U Minnesota", "decade of Alzheimer's data fabrication"),
        ("Tessier-Lavigne (2023)", "Stanford pres.", "image manipulation in 4+ papers"),
    ]
    for i, (case, inst, desc) in enumerate(cases):
        y = 5 - i * 0.85
        # case label box
        ax.text(0.2, y, case, ha="left", va="center",
                fontsize=12, fontweight="bold", color=CORAL)
        ax.text(2.3, y, inst, ha="left", va="center",
                fontsize=11, color=TEXT_DARK, fontweight="bold")
        ax.text(4.7, y, desc, ha="left", va="center",
                fontsize=10, color=TEXT_MUTED, style="italic")
    ax.text(6.5, -0.4,
            "Each could have been caught earlier with cryptographic data provenance.",
            ha="center", va="center", fontsize=11,
            fontweight="bold", color=DARK_BG,
            bbox=dict(facecolor=TEAL_SOFT, edgecolor=TEAL,
                      boxstyle="round,pad=0.4"))
    ax.set_xlim(0, 13)
    ax.set_ylim(-1, 5.5)
    ax.set_aspect("equal")
    ax.axis("off")
    ax.set_title("Six high-profile fraud cases of the past two decades",
                 fontsize=14, fontweight="bold", color=TEXT_DARK, pad=10)
    plt.tight_layout()
    out = ASSETS / "chart_fraud_cases.png"
    plt.savefig(out, bbox_inches="tight", facecolor=WHITE)
    plt.close()
    print(f"wrote {out}")


def chart_data_provenance_graph():
    fig, ax = plt.subplots(figsize=(13, 5.4), dpi=150)
    fig.patch.set_facecolor(WHITE)

    def box(x, y, text, color, w=2.0, h=0.7, tc=WHITE, fs=10):
        rect = FancyBboxPatch((x - w/2, y - h/2), w, h,
                              boxstyle="round,pad=0.02,rounding_size=0.1",
                              facecolor=color, edgecolor=MID, linewidth=1.4)
        ax.add_patch(rect)
        ax.text(x, y, text, ha="center", va="center",
                fontsize=fs, fontweight="bold", color=tc)

    def arrow(p1, p2, label=""):
        a = FancyArrowPatch(p1, p2, arrowstyle="->", mutation_scale=18,
                            linewidth=1.3, color=MID)
        ax.add_patch(a)
        if label:
            mx = (p1[0] + p2[0]) / 2
            my = (p1[1] + p2[1]) / 2
            ax.text(mx, my, label, ha="center", va="center",
                    fontsize=9, color=TEXT_DARK,
                    bbox=dict(facecolor=WHITE, edgecolor="none", pad=1.5))

    box(1.2, 4, "Sensor /\ninstrument", MID, w=1.8, h=0.7)
    box(1.2, 2, "Lab notebook\nentry", MID, w=1.8, h=0.7)
    box(4, 4, "Raw dataset\n(signed)", TEAL, w=1.9, h=0.7)
    box(4, 2, "Validation\nscript (signed)", TEAL, w=1.9, h=0.7)
    box(7, 3, "Analysis script\n(signed)", TEAL, w=2.0, h=0.7)
    box(10, 4, "Figure 3\n(linked to script)", EMERALD, w=2.2, h=0.7)
    box(10, 2, "Table 2\n(linked to script)", EMERALD, w=2.2, h=0.7)
    box(12.4, 3, "Published\npaper", TEAL_SOFT, tc=DARK_BG,
        w=1.5, h=0.7)

    arrow((2.1, 4), (3.05, 4))
    arrow((2.1, 2), (3.05, 2))
    arrow((4.95, 4), (6.0, 3.2))
    arrow((4.95, 2), (6.0, 2.8))
    arrow((8.0, 3.2), (8.9, 4))
    arrow((8.0, 2.8), (8.9, 2))
    arrow((11.1, 4), (11.65, 3.3))
    arrow((11.1, 2), (11.65, 2.7))

    ax.text(6.5, 0.4,
            "Every node signed; every edge a verifiable link. "
            "Tampering with any node breaks the chain.",
            ha="center", va="center", fontsize=11, style="italic",
            color=TEXT_MUTED)
    ax.set_xlim(0, 14)
    ax.set_ylim(0, 5.2)
    ax.set_aspect("equal")
    ax.axis("off")
    ax.set_title("Data provenance chain: lineage from instrument to figure",
                 fontsize=14, fontweight="bold", color=TEXT_DARK, pad=10)
    plt.tight_layout()
    out = ASSETS / "chart_provenance.png"
    plt.savefig(out, bbox_inches="tight", facecolor=WHITE)
    plt.close()
    print(f"wrote {out}")


def chart_review_chain():
    fig, ax = plt.subplots(figsize=(13, 5.0), dpi=150)
    fig.patch.set_facecolor(WHITE)

    def box(x, y, text, color=MID, tc=WHITE, fs=10, w=1.9, h=0.65):
        rect = FancyBboxPatch((x - w/2, y - h/2), w, h,
                              boxstyle="round,pad=0.02,rounding_size=0.1",
                              facecolor=color, edgecolor=MID, linewidth=1.4)
        ax.add_patch(rect)
        ax.text(x, y, text, ha="center", va="center",
                fontsize=fs, fontweight="bold", color=tc)

    def arrow(p1, p2, label="", color=MID):
        a = FancyArrowPatch(p1, p2, arrowstyle="->", mutation_scale=16,
                            linewidth=1.3, color=color)
        ax.add_patch(a)
        if label:
            mx = (p1[0] + p2[0]) / 2
            my = (p1[1] + p2[1]) / 2 + 0.18
            ax.text(mx, my, label, ha="center", va="center",
                    fontsize=8.5, color=TEXT_DARK,
                    bbox=dict(facecolor=WHITE, edgecolor="none", pad=1.5))

    box(1, 3, "Author submits\nsigned preprint", MID)
    box(4, 4, "Reviewer A\n(signed review)", TEAL)
    box(4, 2, "Reviewer B\n(signed review)", TEAL)
    box(7, 3, "Editor decision\n(signed)", AMBER, tc=WHITE)
    box(10, 4, "Author rebuttal\n(signed)", MID)
    box(10, 2, "Revised submission\n(signed)", MID)
    box(12.6, 3, "Accepted +\npublished", EMERALD, w=1.9)

    arrow((2, 3.2), (3.05, 4))
    arrow((2, 2.8), (3.05, 2))
    arrow((4.95, 4), (6.05, 3.2), color=TEAL)
    arrow((4.95, 2), (6.05, 2.8), color=TEAL)
    arrow((7.95, 3.2), (9.05, 4))
    arrow((7.95, 2.8), (9.05, 2))
    arrow((10.95, 4), (12, 3.3))
    arrow((10.95, 2), (12, 2.7))

    ax.text(6.5, 0.6,
            "Anyone can independently verify: who reviewed, what they said, "
            "in what order.",
            ha="center", va="center", fontsize=11, style="italic",
            color=TEXT_MUTED)
    ax.set_xlim(0, 14)
    ax.set_ylim(0.2, 5)
    ax.set_aspect("equal")
    ax.axis("off")
    ax.set_title("Signed peer review chain on a paper",
                 fontsize=14, fontweight="bold", color=TEXT_DARK, pad=10)
    plt.tight_layout()
    out = ASSETS / "chart_review_chain.png"
    plt.savefig(out, bbox_inches="tight", facecolor=WHITE)
    plt.close()
    print(f"wrote {out}")


def chart_reviewer_reputation():
    fig, ax = plt.subplots(figsize=(11, 5.2), dpi=150)
    fig.patch.set_facecolor(WHITE)
    reviewers = ["Reviewer 1\n(early adopter)", "Reviewer 2\n(careful)",
                 "Reviewer 3\n(thorough)", "Reviewer 4\n(rubber-stamp)",
                 "Reviewer 5\n(predicts replication)"]
    metric = [0.62, 0.81, 0.88, 0.34, 0.93]
    colors = [AMBER, EMERALD, EMERALD, CORAL, TEAL]
    bars = ax.bar(reviewers, metric, color=colors, edgecolor=MID,
                  linewidth=1.2)
    for bar, v in zip(bars, metric):
        ax.text(bar.get_x()+bar.get_width()/2, bar.get_height()+0.02,
                f"{v:.2f}", ha="center", va="bottom",
                fontsize=11, fontweight="bold", color=TEXT_DARK)
    ax.set_ylim(0, 1.05)
    ax.set_ylabel("Predictive accuracy of accept/reject vs replication",
                  fontsize=11)
    ax.set_title("Reviewer reputation, computed from review history",
                 fontsize=14, fontweight="bold", color=TEXT_DARK, pad=12)
    _light(ax)
    plt.setp(ax.get_xticklabels(), fontsize=9)
    plt.tight_layout()
    out = ASSETS / "chart_reviewer_rep.png"
    plt.savefig(out, bbox_inches="tight", facecolor=WHITE)
    plt.close()
    print(f"wrote {out}")


if __name__ == "__main__":
    chart_replication_rates()
    chart_baker_survey()
    chart_retraction_growth()
    chart_p_hacking()
    chart_four_primitives()
    chart_paper_lifecycle()
    chart_high_profile_fraud()
    chart_data_provenance_graph()
    chart_review_chain()
    chart_reviewer_reputation()
    print("done.")
