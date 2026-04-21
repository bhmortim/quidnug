"""Charts for Deepfakes deck."""
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


def chart_deepfake_growth():
    fig, ax = plt.subplots(figsize=(12, 5.4), dpi=150)
    fig.patch.set_facecolor(WHITE)
    years = ["2019", "2020", "2021", "2022", "2023", "2024", "2025"]
    counts = [14, 50, 145, 540, 4280, 28100, 78500]
    bars = ax.bar(years, counts, color=CORAL, edgecolor=MID, linewidth=1.2)
    bars[-1].set_color("#7F1D1D")
    bars[-2].set_color("#7F1D1D")
    for bar, v in zip(bars, counts):
        ax.text(bar.get_x()+bar.get_width()/2, bar.get_height()+1500,
                f"{v:,}", ha="center", va="bottom", fontsize=10,
                fontweight="bold", color=TEXT_DARK)
    ax.set_yscale("log")
    ax.set_ylabel("Detected deepfake media (log)", fontsize=11)
    ax.set_title("Deepfake media in circulation: 5600x growth in 6 years",
                 fontsize=14, fontweight="bold", color=TEXT_DARK, pad=12)
    ax.text(0.5, 100000, "Sources: Sumsub Identity Fraud Report, "
                         "DeepMedia, Reality Defender 2024",
            fontsize=9, style="italic", color=TEXT_MUTED)
    _light(ax)
    plt.tight_layout()
    out = ASSETS / "chart_deepfake_growth.png"
    plt.savefig(out, bbox_inches="tight", facecolor=WHITE)
    plt.close()
    print(f"wrote {out}")


def chart_detection_arms_race():
    fig, ax = plt.subplots(figsize=(12, 5.4), dpi=150)
    fig.patch.set_facecolor(WHITE)
    years = list(range(2019, 2026))
    detection_acc = [95, 92, 88, 78, 65, 52, 41]
    generator_quality = [60, 70, 78, 85, 92, 96, 98]
    ax.plot(years, detection_acc, color=CORAL, linewidth=3,
            marker="o", markersize=8, label="Detection accuracy")
    ax.plot(years, generator_quality, color=EMERALD, linewidth=3,
            marker="s", markersize=8, label="Generator realism (subjective)")
    ax.fill_between(years, detection_acc, generator_quality,
                    where=(np.array(generator_quality) > np.array(detection_acc)),
                    alpha=0.2, color=CORAL)
    ax.set_xlabel("Year", fontsize=11)
    ax.set_ylabel("Score (%)", fontsize=11)
    ax.set_title("Detection vs generation: defenders are losing",
                 fontsize=14, fontweight="bold", color=TEXT_DARK, pad=12)
    ax.legend(loc="lower left", fontsize=11)
    ax.set_ylim(30, 105)
    _light(ax)
    plt.tight_layout()
    out = ASSETS / "chart_detection_race.png"
    plt.savefig(out, bbox_inches="tight", facecolor=WHITE)
    plt.close()
    print(f"wrote {out}")


def chart_three_layers():
    fig, ax = plt.subplots(figsize=(13, 5.4), dpi=150)
    fig.patch.set_facecolor(WHITE)
    layers = [
        ("Layer 1", "C2PA content credentials",
         "Camera + editor signs the\nmedia at point of capture",
         "Adobe, Sony, Nikon, Truepic", TEAL),
        ("Layer 2", "DNS-anchored publisher ID",
         "Existing trusted brands\nbind to crypto identity",
         "Reuters.com, AP.org, BBC.co.uk", EMERALD),
        ("Layer 3", "Relational trust graph",
         "Each viewer weights\npublishers by their own trust",
         "Quidnug PoT (per-observer)", AMBER),
    ]
    cell_w = 4.0
    for i, (label, name, body, examples, color) in enumerate(layers):
        x = 0.4 + i * (cell_w + 0.3)
        # header
        rect = FancyBboxPatch((x, 4.2), cell_w, 0.7,
                              boxstyle="round,pad=0.02,rounding_size=0.1",
                              facecolor=color, edgecolor=MID, linewidth=1.4)
        ax.add_patch(rect)
        ax.text(x + cell_w/2, 4.55, f"{label}: {name}",
                ha="center", va="center", fontsize=13,
                fontweight="bold", color=WHITE)
        # body
        rect = FancyBboxPatch((x, 1.8), cell_w, 2.3,
                              boxstyle="round,pad=0.02,rounding_size=0.08",
                              facecolor=LIGHT_BG, edgecolor=color,
                              linewidth=1.5)
        ax.add_patch(rect)
        ax.text(x + cell_w/2, 3.0, body, ha="center", va="center",
                fontsize=12, color=TEXT_DARK)
        ax.text(x + cell_w/2, 2.1, examples, ha="center", va="center",
                fontsize=10, style="italic", color=TEXT_MUTED)
    ax.text(6.7, 0.7,
            "All three layers compose. Each independent, each "
            "necessary, none sufficient alone.",
            ha="center", va="center", fontsize=12,
            fontweight="bold", color=DARK_BG)
    ax.set_xlim(0, 13.5)
    ax.set_ylim(0.3, 5.4)
    ax.set_aspect("equal")
    ax.axis("off")
    ax.set_title("The three-layer defense architecture",
                 fontsize=15, fontweight="bold", color=TEXT_DARK, pad=10)
    plt.tight_layout()
    out = ASSETS / "chart_three_layers.png"
    plt.savefig(out, bbox_inches="tight", facecolor=WHITE)
    plt.close()
    print(f"wrote {out}")


def chart_election_incidents():
    fig, ax = plt.subplots(figsize=(12, 5.4), dpi=150)
    fig.patch.set_facecolor(WHITE)
    incidents = [
        ("Slovakia\n(Sep 2023)",
         "Audio deepfake of opposition\nleader days before election"),
        ("US NH\nprimary (Jan 2024)",
         "Fake Biden robocall\nurging voters to skip primary"),
        ("UK general\nelection (2024)",
         "Multiple fake Sunak / Starmer\naudio + image clips"),
        ("Indonesia\n(2024)",
         "Deepfake of deceased\nformer president endorsing party"),
        ("India\n(2024)",
         "Bollywood star deepfakes\nfor multiple political parties"),
        ("Bangladesh\n(2024)",
         "Multiple altered videos of\nopposition figures"),
        ("Romania\n(Nov 2024)",
         "Deepfake-driven first-round\n(election annulled by court)"),
    ]
    for i, (event, desc) in enumerate(incidents):
        y = 5.2 - i * 0.7
        ax.text(0.4, y, event, ha="left", va="center",
                fontsize=11, fontweight="bold", color=CORAL)
        ax.text(3.6, y, desc, ha="left", va="center",
                fontsize=10, color=TEXT_DARK)
    ax.text(7, -0.3,
            "The 2024 election cycle confirmed: deepfakes are now an "
            "operational political tool worldwide.",
            ha="center", va="center", fontsize=11,
            fontweight="bold", color=DARK_BG)
    ax.set_xlim(0, 13)
    ax.set_ylim(-0.7, 5.6)
    ax.set_aspect("equal")
    ax.axis("off")
    ax.set_title("Documented deepfake incidents in 2024 elections",
                 fontsize=14, fontweight="bold", color=TEXT_DARK, pad=10)
    plt.tight_layout()
    out = ASSETS / "chart_elections.png"
    plt.savefig(out, bbox_inches="tight", facecolor=WHITE)
    plt.close()
    print(f"wrote {out}")


def chart_c2pa_flow():
    fig, ax = plt.subplots(figsize=(13, 5.0), dpi=150)
    fig.patch.set_facecolor(WHITE)

    def box(x, y, text, color=MID, w=2.0, h=0.7, tc=WHITE, fs=10):
        rect = FancyBboxPatch((x-w/2, y-h/2), w, h,
                              boxstyle="round,pad=0.02,rounding_size=0.1",
                              facecolor=color, edgecolor=MID, linewidth=1.4)
        ax.add_patch(rect)
        ax.text(x, y, text, ha="center", va="center",
                fontsize=fs, fontweight="bold", color=tc)

    def arr(p1, p2, label="", color=MID):
        a = FancyArrowPatch(p1, p2, arrowstyle="->", mutation_scale=18,
                            linewidth=1.4, color=color)
        ax.add_patch(a)
        if label:
            mx = (p1[0]+p2[0])/2
            my = (p1[1]+p2[1])/2 + 0.18
            ax.text(mx, my, label, ha="center", va="center",
                    fontsize=8.5, color=TEXT_DARK,
                    bbox=dict(facecolor=WHITE, edgecolor="none", pad=1.5))

    box(1.2, 3.5, "Camera /\nrecording device", MID)
    box(4.2, 3.5, "Edit / processing\nsoftware", MID)
    box(7.4, 3.5, "Publisher\n(AP, BBC, etc)", TEAL)
    box(10.4, 3.5, "Distribution\nplatform", AMBER, tc=WHITE)
    box(12.6, 3.5, "Reader", EMERALD, w=1.5)
    arr((2.2, 3.5), (3.2, 3.5), "C2PA manifest")
    arr((5.2, 3.5), (6.4, 3.5), "+ edit signed", color=TEAL)
    arr((8.4, 3.5), (9.4, 3.5), "+ publisher sig", color=TEAL)
    arr((11.15, 3.5), (11.85, 3.5), "verify chain", color=EMERALD)
    ax.text(6.5, 1.5,
            "Each step adds a signed assertion. Reader's UI verifies "
            "the full chain.",
            ha="center", va="center", fontsize=11, style="italic",
            color=TEXT_MUTED,
            bbox=dict(facecolor=WHITE, edgecolor=TEAL,
                      boxstyle="round,pad=0.4"))
    ax.set_xlim(0, 14)
    ax.set_ylim(0.5, 5)
    ax.set_aspect("equal")
    ax.axis("off")
    ax.set_title("C2PA capture-to-display chain",
                 fontsize=14, fontweight="bold", color=TEXT_DARK, pad=10)
    plt.tight_layout()
    out = ASSETS / "chart_c2pa.png"
    plt.savefig(out, bbox_inches="tight", facecolor=WHITE)
    plt.close()
    print(f"wrote {out}")


def chart_publisher_trust():
    fig, ax = plt.subplots(figsize=(11, 5.2), dpi=150)
    fig.patch.set_facecolor(WHITE)
    publishers = ["Reuters", "AP", "BBC", "NYT", "Wash Post",
                  "Guardian", "Fox News", "InfoWars",
                  "Random blog"]
    trust = [0.9, 0.9, 0.85, 0.8, 0.75, 0.75, 0.5, 0.05, 0.0]
    colors = [EMERALD if t > 0.7 else
              AMBER if t > 0.4 else CORAL for t in trust]
    bars = ax.bar(publishers, trust, color=colors, edgecolor=MID,
                  linewidth=1.2)
    for bar, v in zip(bars, trust):
        ax.text(bar.get_x()+bar.get_width()/2, bar.get_height()+0.02,
                f"{v:.2f}", ha="center", va="bottom",
                fontsize=10, fontweight="bold", color=TEXT_DARK)
    ax.set_ylim(0, 1.1)
    ax.set_ylabel("Per-observer trust weight (illustrative)", fontsize=11)
    ax.set_title(
        "One observer's trust graph weighting publishers",
        fontsize=14, fontweight="bold", color=TEXT_DARK, pad=12)
    _light(ax)
    plt.setp(ax.get_xticklabels(), fontsize=9)
    plt.tight_layout()
    out = ASSETS / "chart_publisher_trust.png"
    plt.savefig(out, bbox_inches="tight", facecolor=WHITE)
    plt.close()
    print(f"wrote {out}")


def chart_full_stack():
    fig, ax = plt.subplots(figsize=(13, 5.4), dpi=150)
    fig.patch.set_facecolor(WHITE)

    def box(x, y, text, color, w=2.5, h=0.8, tc=WHITE, fs=11):
        rect = FancyBboxPatch((x-w/2, y-h/2), w, h,
                              boxstyle="round,pad=0.02,rounding_size=0.1",
                              facecolor=color, edgecolor=MID, linewidth=1.4)
        ax.add_patch(rect)
        ax.text(x, y, text, ha="center", va="center",
                fontsize=fs, fontweight="bold", color=tc)

    box(2, 4.5, "Source\n(witness/insider)", MID, w=2.5)
    box(2, 2.5, "Selective disclosure\n(QDP-0024)", "#7C3AED", w=2.5)
    box(6, 4.5, "Journalist\n(Quidnug quid)", TEAL)
    box(6, 2.5, "Editorial vetting\n(signed)", TEAL)
    box(10, 4.5, "Publisher\n(DNS-anchored)", EMERALD)
    box(10, 2.5, "Article\n(C2PA + sigs)", EMERALD)
    box(12.8, 3.5, "Reader\n(per-observer\ntrust)", AMBER, w=2.0,
        h=1.2)

    for x1, y1, x2, y2 in [
        (2, 4.1, 2, 2.9), (2, 4.5, 4.85, 4.5),
        (2, 2.5, 4.85, 2.5), (6, 4.1, 6, 2.9),
        (6, 4.5, 8.85, 4.5), (6, 2.5, 8.85, 2.5),
        (10, 4.1, 10, 2.9), (10, 3.5, 11.85, 3.5),
    ]:
        a = FancyArrowPatch((x1, y1), (x2, y2), arrowstyle="->",
                            mutation_scale=14, linewidth=1.2,
                            color=MID, alpha=0.7)
        ax.add_patch(a)

    ax.text(6.5, 0.6,
            "End-to-end signed pipeline from source to reader. "
            "Every node attestable. Every edge verifiable.",
            ha="center", va="center", fontsize=11, style="italic",
            color=TEXT_MUTED,
            bbox=dict(facecolor=WHITE, edgecolor=TEAL,
                      boxstyle="round,pad=0.4"))
    ax.set_xlim(0, 14.5)
    ax.set_ylim(0, 5.4)
    ax.set_aspect("equal")
    ax.axis("off")
    ax.set_title(
        "End-to-end trust pipeline: source to reader",
        fontsize=14, fontweight="bold", color=TEXT_DARK, pad=10)
    plt.tight_layout()
    out = ASSETS / "chart_full_stack.png"
    plt.savefig(out, bbox_inches="tight", facecolor=WHITE)
    plt.close()
    print(f"wrote {out}")


def chart_attack_surface():
    fig, ax = plt.subplots(figsize=(12, 5.4), dpi=150)
    fig.patch.set_facecolor(WHITE)
    attacks = ["Screen-grab\n(strip C2PA)",
               "Forged camera\nsignature",
               "Compromised\npublisher key",
               "Spoof DNS",
               "Social engineering\n(real publisher\npublishes false)"]
    no_defense = [100, 100, 100, 100, 100]
    with_defense = [40, 35, 60, 25, 70]
    x = np.arange(len(attacks))
    width = 0.35
    ax.bar(x - width/2, no_defense, width, color=CORAL,
           edgecolor=MID, linewidth=1, label="Without three-layer defense")
    ax.bar(x + width/2, with_defense, width, color=EMERALD,
           edgecolor=MID, linewidth=1, label="With three-layer defense")
    for i, (n, w) in enumerate(zip(no_defense, with_defense)):
        ax.text(i - width/2, n+2, f"{n}%", ha="center", fontsize=10,
                fontweight="bold", color=TEXT_DARK)
        ax.text(i + width/2, w+2, f"{w}%", ha="center", fontsize=10,
                fontweight="bold", color=TEXT_DARK)
    ax.set_ylim(0, 120)
    ax.set_ylabel("Estimated attack success rate (%)", fontsize=11)
    ax.set_xticks(x)
    ax.set_xticklabels(attacks, fontsize=10)
    ax.legend(loc="upper right", fontsize=10)
    ax.set_title(
        "Attack success: with vs without three-layer defense",
        fontsize=14, fontweight="bold", color=TEXT_DARK, pad=12)
    _light(ax)
    plt.tight_layout()
    out = ASSETS / "chart_attack_surface.png"
    plt.savefig(out, bbox_inches="tight", facecolor=WHITE)
    plt.close()
    print(f"wrote {out}")


def chart_adoption_curve():
    fig, ax = plt.subplots(figsize=(12, 5.2), dpi=150)
    fig.patch.set_facecolor(WHITE)
    years = list(range(2024, 2032))
    c2pa = [3, 8, 18, 32, 48, 62, 75, 84]
    dns = [0, 1, 5, 14, 28, 45, 62, 75]
    trust = [0, 0, 1, 4, 12, 24, 38, 52]
    ax.plot(years, c2pa, color=TEAL, linewidth=2.5, marker="o",
            markersize=6, label="C2PA-signed media")
    ax.plot(years, dns, color=EMERALD, linewidth=2.5, marker="s",
            markersize=6, label="DNS-anchored publishers")
    ax.plot(years, trust, color=AMBER, linewidth=2.5, marker="^",
            markersize=6, label="Per-observer trust UI")
    ax.set_xlabel("Year", fontsize=11)
    ax.set_ylabel("% of mainstream news media", fontsize=11)
    ax.set_title(
        "Projected adoption curves (illustrative)",
        fontsize=14, fontweight="bold", color=TEXT_DARK, pad=12)
    ax.legend(loc="upper left", fontsize=10)
    ax.set_ylim(0, 100)
    _light(ax)
    plt.tight_layout()
    out = ASSETS / "chart_adoption.png"
    plt.savefig(out, bbox_inches="tight", facecolor=WHITE)
    plt.close()
    print(f"wrote {out}")


if __name__ == "__main__":
    chart_deepfake_growth()
    chart_detection_arms_race()
    chart_three_layers()
    chart_election_incidents()
    chart_c2pa_flow()
    chart_publisher_trust()
    chart_full_stack()
    chart_attack_surface()
    chart_adoption_curve()
    print("done.")
