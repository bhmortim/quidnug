"""Charts for TPRM deck."""
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


def chart_breach_costs():
    fig, ax = plt.subplots(figsize=(12, 5.4), dpi=150)
    fig.patch.set_facecolor(WHITE)
    breaches = ["SolarWinds\n(Dec 2020)", "Log4j\n(Dec 2021)",
                "Codecov\n(Apr 2021)", "Kaseya\n(Jul 2021)",
                "MOVEit\n(May 2023)", "XZ backdoor\n(Mar 2024)",
                "Okta CSR\n(Oct 2023)"]
    cost_billions = [100, 50, 1.5, 70, 9.9, 0.1, 0.4]  # impact estimates
    bars = ax.bar(breaches, cost_billions, color=CORAL, edgecolor=MID,
                  linewidth=1.2)
    for bar, v in zip(bars, cost_billions):
        if v >= 1:
            label = f"${v}B"
        else:
            label = f"${v*1000:.0f}M"
        ax.text(bar.get_x()+bar.get_width()/2, bar.get_height()+1.5,
                label, ha="center", va="bottom", fontsize=11,
                fontweight="bold", color=TEXT_DARK)
    ax.set_yscale("symlog")
    ax.set_ylabel("Estimated impact ($B / log)", fontsize=11)
    ax.set_title(
        "Major supply chain breaches: estimated economic impact",
        fontsize=14, fontweight="bold", color=TEXT_DARK, pad=12)
    _light(ax)
    plt.setp(ax.get_xticklabels(), fontsize=9)
    plt.tight_layout()
    out = ASSETS / "chart_breaches.png"
    plt.savefig(out, bbox_inches="tight", facecolor=WHITE)
    plt.close()
    print(f"wrote {out}")


def chart_tprm_workflow():
    fig, ax = plt.subplots(figsize=(13, 5.0), dpi=150)
    fig.patch.set_facecolor(WHITE)

    def box(x, y, text, color=MID, w=2.0, h=0.7, tc=WHITE, fs=10):
        rect = FancyBboxPatch((x-w/2, y-h/2), w, h,
                              boxstyle="round,pad=0.02,rounding_size=0.1",
                              facecolor=color, edgecolor=MID, linewidth=1.4)
        ax.add_patch(rect)
        ax.text(x, y, text, ha="center", va="center",
                fontsize=fs, fontweight="bold", color=tc)

    def arr(p1, p2, lbl="", color=MID):
        a = FancyArrowPatch(p1, p2, arrowstyle="->", mutation_scale=18,
                            linewidth=1.4, color=color)
        ax.add_patch(a)
        if lbl:
            mx = (p1[0]+p2[0])/2; my = (p1[1]+p2[1])/2 + 0.18
            ax.text(mx, my, lbl, ha="center", va="center",
                    fontsize=8.5, color=TEXT_DARK,
                    bbox=dict(facecolor=WHITE, edgecolor="none", pad=1.5))

    box(1.5, 4, "Procurement\nrequests vendor", MID)
    box(4.5, 4, "Send 200-question\nquestionnaire", AMBER, tc=WHITE)
    box(7.5, 4, "Vendor self-fills\n(2-6 weeks)", AMBER, tc=WHITE)
    box(10.5, 4, "Security team\nreviews", MID)
    box(12.8, 4, "Approve", EMERALD, w=1.5)
    box(7.5, 1.8, "Annual renewal:\nrepeat the entire process", CORAL,
        w=4.0, tc=WHITE)
    arr((2.5, 4), (3.5, 4))
    arr((5.5, 4), (6.5, 4))
    arr((8.5, 4), (9.5, 4))
    arr((11.5, 4), (12.05, 4))
    arr((10.5, 3.65), (8.5, 2.15), color=CORAL)
    arr((6.5, 2.15), (3.5, 3.65), color=CORAL)
    ax.text(7, 0.5,
            "Self-attestation. Months of latency. No verification of "
            "any answer. Annual repetition.",
            ha="center", va="center", fontsize=11, style="italic",
            color=CORAL,
            bbox=dict(facecolor="#FEF2F2", edgecolor=CORAL,
                      boxstyle="round,pad=0.4"))
    ax.set_xlim(0, 14)
    ax.set_ylim(0, 5)
    ax.set_aspect("equal")
    ax.axis("off")
    ax.set_title("How TPRM works today (and why it doesn't)",
                 fontsize=14, fontweight="bold", color=TEXT_DARK, pad=10)
    plt.tight_layout()
    out = ASSETS / "chart_tprm_workflow.png"
    plt.savefig(out, bbox_inches="tight", facecolor=WHITE)
    plt.close()
    print(f"wrote {out}")


def chart_nth_party_depth():
    fig, ax = plt.subplots(figsize=(12, 5.4), dpi=150)
    fig.patch.set_facecolor(WHITE)
    depths = ["1st party\n(your own)", "2nd party\n(direct vendor)",
              "3rd party\n(vendor's vendor)",
              "4th party\n(N=3 hops)", "5th party\n(N=4 hops)",
              "6th+ party"]
    visibility = [100, 70, 25, 8, 2, 0.5]
    bars = ax.bar(depths, visibility,
                  color=[EMERALD, EMERALD, AMBER, CORAL, CORAL, "#7F1D1D"],
                  edgecolor=MID, linewidth=1.2)
    for bar, v in zip(bars, visibility):
        ax.text(bar.get_x()+bar.get_width()/2, bar.get_height()+2,
                f"{v}%", ha="center", va="bottom",
                fontsize=12, fontweight="bold", color=TEXT_DARK)
    ax.set_ylim(0, 115)
    ax.set_ylabel("% of CISOs reporting visibility (Ponemon 2023)", fontsize=11)
    ax.set_title("Nth-party visibility decays exponentially",
                 fontsize=14, fontweight="bold", color=TEXT_DARK, pad=12)
    _light(ax)
    plt.tight_layout()
    out = ASSETS / "chart_nth_party.png"
    plt.savefig(out, bbox_inches="tight", facecolor=WHITE)
    plt.close()
    print(f"wrote {out}")


def chart_vendor_graph():
    fig, ax = plt.subplots(figsize=(13, 5.4), dpi=150)
    fig.patch.set_facecolor(WHITE)

    # Central org with N vendors at varying depths
    np.random.seed(42)
    cx, cy = 6.5, 2.7
    # Center
    c = Circle((cx, cy), 0.45, facecolor=DARK_BG, edgecolor=MID,
               linewidth=2)
    ax.add_patch(c)
    ax.text(cx, cy, "You", ha="center", va="center",
            fontsize=11, fontweight="bold", color=WHITE)
    # Layer 1: direct vendors (5)
    n1 = 5
    layer1_pos = []
    for i in range(n1):
        angle = 2*np.pi*i/n1
        x = cx + 1.7*np.cos(angle); y = cy + 1.7*np.sin(angle)
        layer1_pos.append((x, y))
        ax.plot([cx, x], [cy, y], color=TEAL, linewidth=1.6, alpha=0.7,
                zorder=0)
        circle = Circle((x, y), 0.32, facecolor=TEAL, edgecolor=MID,
                        linewidth=1.5)
        ax.add_patch(circle)
        ax.text(x, y, f"V{i+1}", ha="center", va="center",
                fontsize=10, fontweight="bold", color=WHITE)
    # Layer 2: each direct has 2-4 sub-vendors
    for i, (x1, y1) in enumerate(layer1_pos):
        n2 = 3
        for j in range(n2):
            sub_angle = 2*np.pi*j/n2 + i
            x2 = x1 + 1.0*np.cos(sub_angle); y2 = y1 + 1.0*np.sin(sub_angle)
            ax.plot([x1, x2], [y1, y2], color=AMBER, linewidth=1.0,
                    alpha=0.5, zorder=0)
            ax.scatter([x2], [y2], s=80, color=AMBER, edgecolor=MID,
                       linewidth=1, zorder=2)
    # Layer 3: hidden third party
    layer3_x = cx + 2.5; layer3_y = cy + 2.0
    ax.scatter([layer3_x], [layer3_y], s=120, color=CORAL,
               edgecolor=MID, linewidth=2, zorder=5)
    ax.annotate("Compromised lib\nin transitive\ndependency\n"
                "(invisible to you)",
                xy=(layer3_x, layer3_y),
                xytext=(layer3_x + 1.5, layer3_y + 1.0),
                fontsize=10, fontweight="bold", color=CORAL,
                arrowprops=dict(arrowstyle="->", color=CORAL, lw=1.5))
    ax.text(2, 5.0, "Direct vendors (visible)",
            ha="left", va="center", fontsize=10, fontweight="bold",
            color=TEAL)
    ax.text(2, 4.5, "Sub-vendors (mostly invisible)",
            ha="left", va="center", fontsize=10, fontweight="bold",
            color=AMBER)
    ax.text(2, 4.0, "Nth party (effectively invisible)",
            ha="left", va="center", fontsize=10, fontweight="bold",
            color=CORAL)
    ax.set_xlim(0, 13)
    ax.set_ylim(0, 5.4)
    ax.set_aspect("equal")
    ax.axis("off")
    ax.set_title("The Nth-party visibility problem",
                 fontsize=14, fontweight="bold", color=TEXT_DARK, pad=10)
    plt.tight_layout()
    out = ASSETS / "chart_vendor_graph.png"
    plt.savefig(out, bbox_inches="tight", facecolor=WHITE)
    plt.close()
    print(f"wrote {out}")


def chart_attestation_layers():
    fig, ax = plt.subplots(figsize=(13, 5.0), dpi=150)
    fig.patch.set_facecolor(WHITE)
    layers = [
        ("Identity layer",
         "Vendor + your org\nhave Quidnug quids", TEAL),
        ("Trust edge layer",
         "You publish signed\nTRUST tx in vendors", EMERALD),
        ("Peer layer",
         "Industry peers sharing\ntrust signals", AMBER),
        ("Component layer",
         "Sigstore + SLSA +\nsigned attestations", "#7C3AED"),
        ("Decision layer",
         "Auto-deny when trust\nfalls below threshold", CORAL),
    ]
    cell_w = 2.5
    for i, (name, body, color) in enumerate(layers):
        x = 0.4 + i * (cell_w + 0.15)
        # numbered circle at top
        circ = Circle((x + 0.5, 4.3), 0.35, facecolor=color,
                      edgecolor=MID, linewidth=1.5)
        ax.add_patch(circ)
        ax.text(x + 0.5, 4.3, str(i+1), ha="center", va="center",
                fontsize=18, fontweight="bold", color=WHITE)
        # name
        ax.text(x + cell_w/2 + 0.15, 4.3, name, ha="left", va="center",
                fontsize=12, fontweight="bold", color=TEXT_DARK)
        # body
        rect = FancyBboxPatch((x, 1.8), cell_w, 2.2,
                              boxstyle="round,pad=0.05,rounding_size=0.1",
                              facecolor=LIGHT_BG, edgecolor=color, linewidth=1.5)
        ax.add_patch(rect)
        ax.text(x + cell_w/2, 2.85, body, ha="center", va="center",
                fontsize=11, color=TEXT_DARK)
    ax.text(7, 0.7,
            "Five layers. Compose into a trust graph that auto-evaluates "
            "Nth-party exposure.",
            ha="center", va="center", fontsize=11, fontweight="bold",
            color=DARK_BG)
    ax.set_xlim(0, 13.5)
    ax.set_ylim(0.3, 5.0)
    ax.set_aspect("equal")
    ax.axis("off")
    ax.set_title("Quidnug TPRM: five layers",
                 fontsize=15, fontweight="bold", color=TEXT_DARK, pad=10)
    plt.tight_layout()
    out = ASSETS / "chart_layers.png"
    plt.savefig(out, bbox_inches="tight", facecolor=WHITE)
    plt.close()
    print(f"wrote {out}")


def chart_questionnaire_pain():
    fig, ax = plt.subplots(figsize=(11, 5.2), dpi=150)
    fig.patch.set_facecolor(WHITE)
    cats = ["Avg questions per\nvendor questionnaire",
            "Avg time per\nquestionnaire (hours)",
            "Avg unique vendors\nper Fortune 500",
            "Annual\nquestionnaire-completion\nFTE-equivalent"]
    values = [218, 38, 3700, 47]
    bars = ax.bar(cats, values,
                  color=[CORAL, AMBER, AMBER, CORAL],
                  edgecolor=MID, linewidth=1.2)
    for bar, v in zip(bars, values):
        ax.text(bar.get_x()+bar.get_width()/2, bar.get_height()*1.02,
                f"{v:,}", ha="center", va="bottom",
                fontsize=14, fontweight="bold", color=TEXT_DARK)
    ax.set_yscale("log")
    ax.set_ylabel("Value (log scale)", fontsize=11)
    ax.set_title(
        "The questionnaire-industrial complex (Ponemon 2023)",
        fontsize=14, fontweight="bold", color=TEXT_DARK, pad=12)
    _light(ax)
    plt.setp(ax.get_xticklabels(), fontsize=9)
    plt.tight_layout()
    out = ASSETS / "chart_questionnaire_pain.png"
    plt.savefig(out, bbox_inches="tight", facecolor=WHITE)
    plt.close()
    print(f"wrote {out}")


def chart_migration_phases():
    fig, ax = plt.subplots(figsize=(13, 5.0), dpi=150)
    fig.patch.set_facecolor(WHITE)
    phases = [
        ("Phase 1\nMonths 1-2", "Identity establishment",
         "Issue Quidnug quids to your org\nand to direct vendors", TEAL),
        ("Phase 2\nMonths 2-6", "Trust edge publication",
         "Publish signed TRUST tx\nto each vendor", EMERALD),
        ("Phase 3\nMonths 4-8", "Peer network membership",
         "Join industry information sharing\n(FS-ISAC, A-ISAC, etc)", AMBER),
        ("Phase 4\nMonths 6-12", "Component-layer integration",
         "Sigstore + SLSA + SBOM\nattestations linked", "#7C3AED"),
        ("Phase 5\nMonths 9-18", "Decision automation",
         "Auto-deny vendor when\ntrust < threshold", CORAL),
    ]
    cell_w = 2.5
    for i, (when, name, body, color) in enumerate(phases):
        x = 0.4 + i * (cell_w + 0.15)
        rect = FancyBboxPatch((x, 3.5), cell_w, 1.0,
                              boxstyle="round,pad=0.02,rounding_size=0.1",
                              facecolor=color, edgecolor=MID, linewidth=1.5)
        ax.add_patch(rect)
        ax.text(x + cell_w/2, 4.1, when, ha="center", va="center",
                fontsize=10, fontweight="bold", color=WHITE)
        ax.text(x + cell_w/2, 3.7, name, ha="center", va="center",
                fontsize=10, color=WHITE)
        ax.text(x + cell_w/2, 2.3, body, ha="center", va="center",
                fontsize=10, color=TEXT_DARK)
        if i < len(phases) - 1:
            arr_x = x + cell_w + 0.04
            ax.annotate("", xy=(arr_x + 0.1, 4.0), xytext=(arr_x, 4.0),
                        arrowprops=dict(arrowstyle="->", color=MID, lw=1.5))
    ax.set_xlim(0, 13.5)
    ax.set_ylim(1.2, 5.0)
    ax.set_aspect("equal")
    ax.axis("off")
    ax.set_title("18-month migration plan from current TPRM",
                 fontsize=14, fontweight="bold", color=TEXT_DARK, pad=10)
    plt.tight_layout()
    out = ASSETS / "chart_migration.png"
    plt.savefig(out, bbox_inches="tight", facecolor=WHITE)
    plt.close()
    print(f"wrote {out}")


def chart_economic_comparison():
    fig, ax = plt.subplots(figsize=(12, 5.0), dpi=150)
    fig.patch.set_facecolor(WHITE)
    cats = ["Cost per vendor\nonboarding (current)",
            "Cost per vendor\nonboarding (Quidnug)",
            "Cost per breach\n(IBM avg)",
            "Reduction with\nNth-party visibility"]
    values = [12000, 800, 4880000, -1700000]
    colors = [CORAL, EMERALD, CORAL, EMERALD]
    bars = ax.bar(cats, values,
                  color=colors,
                  edgecolor=MID, linewidth=1.2)
    for bar, v in zip(bars, values):
        if v >= 1000000:
            label = f"${v/1000000:.2f}M"
        elif v >= 1000:
            label = f"${v/1000:.0f}k"
        elif v < 0:
            label = f"-${abs(v)/1000000:.2f}M"
        else:
            label = f"${v:,}"
        ax.text(bar.get_x()+bar.get_width()/2,
                bar.get_height() if v > 0 else bar.get_height() - 100000,
                label, ha="center",
                va="bottom" if v > 0 else "top",
                fontsize=11, fontweight="bold", color=TEXT_DARK)
    ax.set_yscale("symlog")
    ax.set_ylabel("USD (symlog)", fontsize=11)
    ax.set_title(
        "Economic comparison: current TPRM vs Quidnug architecture",
        fontsize=14, fontweight="bold", color=TEXT_DARK, pad=12)
    _light(ax)
    plt.setp(ax.get_xticklabels(), fontsize=9)
    plt.tight_layout()
    out = ASSETS / "chart_economics.png"
    plt.savefig(out, bbox_inches="tight", facecolor=WHITE)
    plt.close()
    print(f"wrote {out}")


def chart_roi_curve():
    fig, ax = plt.subplots(figsize=(12, 5.0), dpi=150)
    fig.patch.set_facecolor(WHITE)
    months = list(range(0, 25))
    cost_current = [50000 + m * 2000 for m in months]
    cost_quidnug = [120000 - m * 3000 if m < 18 else
                    120000 - 17 * 3000 - (m - 18) * 1500 for m in months]
    ax.plot(months, cost_current, color=CORAL, linewidth=2.5,
            marker="o", markersize=4, label="Status quo (questionnaires)")
    ax.plot(months, cost_quidnug, color=EMERALD, linewidth=2.5,
            marker="s", markersize=4, label="Quidnug architecture")
    crossover = next((i for i in range(len(months))
                     if cost_quidnug[i] < cost_current[i]), 0)
    ax.axvline(crossover, color=AMBER, linestyle="--", alpha=0.5)
    ax.text(crossover + 0.3, 110000, f"Crossover\nmonth {crossover}",
            color=AMBER, fontweight="bold", fontsize=10)
    ax.set_xlabel("Months", fontsize=11)
    ax.set_ylabel("Cumulative TPRM cost ($)", fontsize=11)
    ax.set_title(
        "Total cost of TPRM: 24-month projection",
        fontsize=14, fontweight="bold", color=TEXT_DARK, pad=12)
    ax.legend(loc="lower right", fontsize=10)
    _light(ax)
    plt.tight_layout()
    out = ASSETS / "chart_roi.png"
    plt.savefig(out, bbox_inches="tight", facecolor=WHITE)
    plt.close()
    print(f"wrote {out}")


if __name__ == "__main__":
    chart_breach_costs()
    chart_tprm_workflow()
    chart_nth_party_depth()
    chart_vendor_graph()
    chart_attestation_layers()
    chart_questionnaire_pain()
    chart_migration_phases()
    chart_economic_comparison()
    chart_roi_curve()
    print("done.")
