"""Chart generators for the Relativistic Ratings deck."""
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


def _style_light(ax):
    ax.set_facecolor(WHITE)
    for spine in ax.spines.values():
        spine.set_color("#CBD5E1")
    ax.tick_params(colors=TEXT_MUTED)
    ax.xaxis.label.set_color(TEXT_DARK)
    ax.yaxis.label.set_color(TEXT_DARK)
    ax.title.set_color(TEXT_DARK)
    ax.grid(True, color=GRID, linewidth=0.6, alpha=0.9)


# 1. The J-curve of online reviews
def chart_j_curve():
    fig, ax = plt.subplots(figsize=(12, 5.5), dpi=150)
    fig.patch.set_facecolor(WHITE)
    stars = ["1 star", "2 stars", "3 stars", "4 stars", "5 stars"]
    pct = [14, 6, 7, 15, 58]
    colors = [CORAL, AMBER, TEXT_MUTED, EMERALD, TEAL]
    bars = ax.bar(stars, pct, color=colors, edgecolor=MID, linewidth=1.2)
    for bar, v in zip(bars, pct):
        ax.text(bar.get_x() + bar.get_width() / 2,
                bar.get_height() + 1.5, f"{v}%",
                ha="center", va="bottom", fontsize=12,
                fontweight="bold", color=TEXT_DARK)
    ax.set_ylabel("Share of total reviews (%)", fontsize=12)
    ax.set_ylim(0, 70)
    ax.set_title(
        "The J-curve: typical Amazon / Yelp / Google review distribution",
        fontsize=14, fontweight="bold", color=TEXT_DARK, pad=12)
    ax.text(0.5, 65, "Source: Hu, Pavlou, and Zhang (2006); "
                     "Dellarocas (2003); Luca (2016).",
            fontsize=9, style="italic", color=TEXT_MUTED)
    _style_light(ax)
    plt.tight_layout()
    out = ASSETS / "chart_j_curve.png"
    plt.savefig(out, bbox_inches="tight", facecolor=WHITE)
    plt.close()
    print(f"wrote {out}")


# 2. Asch conformity rate
def chart_asch():
    fig, ax = plt.subplots(figsize=(11, 5.4), dpi=150)
    fig.patch.set_facecolor(WHITE)
    cats = ["Alone (control)",
            "1 confederate disagrees",
            "2 confederates disagree",
            "3 confederates disagree",
            "Full unanimous group"]
    rates = [0.7, 4, 14, 32, 38]
    bars = ax.bar(cats, rates, color=[EMERALD, AMBER, AMBER, CORAL, CORAL],
                  edgecolor=MID, linewidth=1)
    for bar, v in zip(bars, rates):
        ax.text(bar.get_x() + bar.get_width() / 2,
                bar.get_height() + 0.7, f"{v}%",
                ha="center", va="bottom", fontsize=12,
                fontweight="bold", color=TEXT_DARK)
    ax.set_ylabel("Conformity rate on critical trials (%)", fontsize=11)
    ax.set_ylim(0, 50)
    ax.set_title("Asch (1951, 1956): conformity to a unanimous wrong answer",
                 fontsize=14, fontweight="bold", color=TEXT_DARK, pad=12)
    ax.text(0.5, 47,
            "75% of subjects conformed at least once when surrounded "
            "by a unanimous wrong group.",
            fontsize=10, style="italic", color=TEXT_MUTED)
    _style_light(ax)
    plt.setp(ax.get_xticklabels(), fontsize=10)
    plt.tight_layout()
    out = ASSETS / "chart_asch.png"
    plt.savefig(out, bbox_inches="tight", facecolor=WHITE)
    plt.close()
    print(f"wrote {out}")


# 3. Granovetter weak ties
def chart_weak_ties():
    fig, ax = plt.subplots(figsize=(11, 5.5), dpi=150)
    fig.patch.set_facecolor(WHITE)
    cats = ["Strong ties\n(family, close friends)",
            "Intermediate ties\n(work friends)",
            "Weak ties\n(acquaintances)"]
    pct = [17, 27, 56]
    colors = [TEXT_MUTED, AMBER, EMERALD]
    bars = ax.bar(cats, pct, color=colors, edgecolor=MID, linewidth=1.2,
                  width=0.55)
    for bar, v in zip(bars, pct):
        ax.text(bar.get_x() + bar.get_width() / 2,
                bar.get_height() + 1, f"{v}%",
                ha="center", va="bottom", fontsize=14,
                fontweight="bold", color=TEXT_DARK)
    ax.set_ylabel("% of jobs found through this tie type", fontsize=11)
    ax.set_ylim(0, 70)
    ax.set_title("Granovetter (1973): The Strength of Weak Ties",
                 fontsize=14, fontweight="bold", color=TEXT_DARK, pad=12)
    ax.text(0.5, 65,
            "Weak ties carry the most novel information because they "
            "connect disjoint social clusters.",
            fontsize=10, style="italic", color=TEXT_MUTED)
    _style_light(ax)
    plt.tight_layout()
    out = ASSETS / "chart_weak_ties.png"
    plt.savefig(out, bbox_inches="tight", facecolor=WHITE)
    plt.close()
    print(f"wrote {out}")


# 4. Review fraud market sizing
def chart_fraud_market():
    fig, ax = plt.subplots(figsize=(12, 5.6), dpi=150)
    fig.patch.set_facecolor(WHITE)
    cats = ["Amazon\n(He/Hollenbeck/\nProserpio 2022)",
            "Trustpilot estimate\n(2023 transparency)",
            "TripAdvisor\n(industry est.)",
            "Yelp\n(Luca/Zervas 2016)",
            "Cross-platform total\n(industry est.)"]
    sizes = [152, 220, 95, 75, 540]
    colors = [CORAL] * 4 + ["#7F1D1D"]
    bars = ax.bar(cats, sizes, color=colors, edgecolor=MID, linewidth=1.2,
                  width=0.6)
    for bar, v in zip(bars, sizes):
        ax.text(bar.get_x() + bar.get_width() / 2,
                bar.get_height() + 12, f"${v}M",
                ha="center", va="bottom", fontsize=12,
                fontweight="bold", color=TEXT_DARK)
    ax.set_ylabel("Annual fake-review market ($M)", fontsize=11)
    ax.set_ylim(0, 620)
    ax.set_title("The fake-review economy: estimated annual spend",
                 fontsize=14, fontweight="bold", color=TEXT_DARK, pad=12)
    _style_light(ax)
    plt.setp(ax.get_xticklabels(), fontsize=10)
    plt.tight_layout()
    out = ASSETS / "chart_fraud_market.png"
    plt.savefig(out, bbox_inches="tight", facecolor=WHITE)
    plt.close()
    print(f"wrote {out}")


# 5. Detection arms race ceiling
def chart_detection_ceiling():
    fig, ax = plt.subplots(figsize=(12, 5.4), dpi=150)
    fig.patch.set_facecolor(WHITE)
    methods = ["Jindal-Liu\n2008", "Ott et al\n2011",
               "Heuristic-only\n(generic)", "Modern ML\n(BERT-based)",
               "Best published\nensemble (2024)"]
    accs = [78, 89, 65, 91, 94]
    bars = ax.bar(methods, accs, color=[CORAL, AMBER, CORAL, EMERALD, TEAL],
                  edgecolor=MID, linewidth=1.2, width=0.55)
    ax.axhline(95, color=TEXT_MUTED, linestyle="--", alpha=0.5)
    ax.text(4, 96,
            "Even at 94% accuracy: 6M undetected fakes per 100M reviews.",
            ha="right", fontsize=10, fontweight="bold", color=CORAL)
    for bar, v in zip(bars, accs):
        ax.text(bar.get_x() + bar.get_width() / 2,
                bar.get_height() + 0.3, f"{v}%",
                ha="center", va="bottom", fontsize=12,
                fontweight="bold", color=TEXT_DARK)
    ax.set_ylabel("Detection accuracy (%)", fontsize=11)
    ax.set_ylim(50, 100)
    ax.set_title(
        "The detection ceiling: even the best classifier leaves "
        "millions of fakes",
        fontsize=14, fontweight="bold", color=TEXT_DARK, pad=12)
    _style_light(ax)
    plt.setp(ax.get_xticklabels(), fontsize=10)
    plt.tight_layout()
    out = ASSETS / "chart_detection.png"
    plt.savefig(out, bbox_inches="tight", facecolor=WHITE)
    plt.close()
    print(f"wrote {out}")


# 6. Two observers see the same product differently
def chart_two_observers():
    fig, axes = plt.subplots(1, 2, figsize=(13, 5.5), dpi=150)
    fig.patch.set_facecolor(WHITE)

    def panel(ax, title, weights, ratings, observer_label, color):
        ax.set_title(title, fontsize=13, fontweight="bold",
                     color=TEXT_DARK, pad=10)
        # Show four reviewers with weights
        names = ["Alice", "Bob", "Carol", "Dave"]
        wsum = sum(weights)
        eff = sum(w * r for w, r in zip(weights, ratings)) / max(wsum, 0.001)

        for i, (name, w, r) in enumerate(zip(names, weights, ratings)):
            x = 0.5 + i * 1.3
            # Reviewer dot, size proportional to weight
            size = max(0.15, 0.15 + w * 0.8)
            sentiment_color = (EMERALD if r >= 4.5 else
                               AMBER if r >= 3 else CORAL)
            circle = Circle((x, 3.5), size,
                            facecolor=sentiment_color,
                            edgecolor=MID, linewidth=1.5,
                            alpha=0.7 + min(w * 0.3, 0.3))
            ax.add_patch(circle)
            ax.text(x, 4.3, name, ha="center", va="center",
                    fontsize=10, fontweight="bold", color=TEXT_DARK)
            ax.text(x, 2.5, f"{r}\u2605", ha="center", va="center",
                    fontsize=10, fontweight="bold",
                    color=sentiment_color)
            ax.text(x, 2.0, f"w={w:.2f}", ha="center", va="center",
                    fontsize=9, color=TEXT_MUTED)

        # Observer label
        ax.text(3, 0.8,
                f"{observer_label}'s effective rating: {eff:.2f}\u2605",
                ha="center", va="center", fontsize=14,
                fontweight="bold", color=color,
                bbox=dict(facecolor=WHITE, edgecolor=color,
                          boxstyle="round,pad=0.4"))

        ax.set_xlim(0, 6)
        ax.set_ylim(0.2, 5)
        ax.set_aspect("equal")
        ax.axis("off")

    # Same four reviewers, different observer trust profiles
    ratings = [5.0, 2.0, 4.5, 5.0]
    panel(axes[0], "Jamie (engineer, trusts laptop experts)",
          weights=[0.73, 0.01, 0.62, 0.0],
          ratings=ratings,
          observer_label="Jamie",
          color=EMERALD)
    panel(axes[1], "Pat (trusts brother Bob, no other graph)",
          weights=[0.0, 0.13, 0.0, 0.0],
          ratings=ratings,
          observer_label="Pat",
          color=AMBER)

    plt.suptitle(
        "Same 4 reviews. Same product. Two observers, two correct ratings.",
        fontsize=14, fontweight="bold", color=TEXT_DARK, y=1.02)

    plt.tight_layout()
    out = ASSETS / "chart_two_observers.png"
    plt.savefig(out, bbox_inches="tight", facecolor=WHITE)
    plt.close()
    print(f"wrote {out}")


# 7. Four-factor formula visualization
def chart_four_factors():
    fig, ax = plt.subplots(figsize=(13, 5.4), dpi=150)
    fig.patch.set_facecolor(WHITE)

    factors = [
        ("T", "Topical trust",
         "T(observer, reviewer, topic)\n= max-path trust\nin domain",
         TEAL),
        ("H", "Helpfulness",
         "H = trust-weighted ratio\nof helpful : unhelpful\nvotes",
         EMERALD),
        ("A", "Activity",
         "A = clip(log(reviews) /\nlog(50), 0, 1)",
         AMBER),
        ("R", "Recency",
         "R = max(0.3,\n  exp(-age_days / 730))",
         "#7C3AED"),
    ]

    cell_w = 2.6
    for i, (sym, name, formula, color) in enumerate(factors):
        x = 0.4 + i * (cell_w + 0.3)
        # Big symbol
        circle = Circle((x + cell_w / 2, 4.3), 0.7,
                        facecolor=color, edgecolor=MID, linewidth=1.5)
        ax.add_patch(circle)
        ax.text(x + cell_w / 2, 4.3, sym,
                ha="center", va="center", fontsize=42,
                fontweight="bold", color=WHITE)
        # Name
        ax.text(x + cell_w / 2, 3.1, name,
                ha="center", va="center", fontsize=15,
                fontweight="bold", color=TEXT_DARK)
        # Formula
        rect = FancyBboxPatch(
            (x + 0.1, 1.0), cell_w - 0.2, 1.7,
            boxstyle="round,pad=0.05,rounding_size=0.1",
            facecolor=LIGHT_BG, edgecolor=color, linewidth=1.5)
        ax.add_patch(rect)
        ax.text(x + cell_w / 2, 1.85, formula,
                ha="center", va="center", fontsize=11,
                color=TEXT_DARK, family="monospace")

        # Multiply sign
        if i < len(factors) - 1:
            ax.text(x + cell_w + 0.15, 4.3, "\u00D7",
                    ha="center", va="center", fontsize=32,
                    fontweight="bold", color=TEXT_MUTED)

    ax.text(6.5, 0.4,
            "w(reviewer) = T \u00D7 H \u00D7 A \u00D7 R    "
            "\u2192    Effective rating = "
            "\u03A3 r.rating \u00B7 w(r)  /  \u03A3 w(r)",
            ha="center", va="center", fontsize=12,
            fontweight="bold", color=DARK_BG,
            bbox=dict(facecolor=TEAL_SOFT, edgecolor=TEAL,
                      boxstyle="round,pad=0.3"))

    ax.set_xlim(0, 13)
    ax.set_ylim(0, 5.4)
    ax.set_aspect("equal")
    ax.axis("off")
    ax.set_title("The four-factor relativistic rating formula",
                 fontsize=15, fontweight="bold", color=TEXT_DARK, pad=10)

    plt.tight_layout()
    out = ASSETS / "chart_four_factors.png"
    plt.savefig(out, bbox_inches="tight", facecolor=WHITE)
    plt.close()
    print(f"wrote {out}")


# 8. Activity factor curve
def chart_activity_curve():
    fig, ax = plt.subplots(figsize=(11, 5.4), dpi=150)
    fig.patch.set_facecolor(WHITE)
    n = np.arange(1, 200)
    a = np.clip(np.log(n) / np.log(50), 0, 1)
    ax.plot(n, a, color=TEAL, linewidth=3)
    # Highlight key points
    for x_pt, label in [(1, "1 review\n0.0"),
                        (10, "10 reviews\n0.59"),
                        (50, "50 reviews\n1.0 (cap)")]:
        y = np.clip(np.log(x_pt) / np.log(50), 0, 1)
        ax.scatter([x_pt], [y], color=CORAL, s=120, zorder=5,
                   edgecolor=MID)
        ax.annotate(label, xy=(x_pt, y),
                    xytext=(x_pt + 8, y - 0.12),
                    fontsize=10, fontweight="bold", color=CORAL,
                    arrowprops=dict(arrowstyle="-", color=CORAL,
                                    alpha=0.6))
    ax.set_xlabel("Reviews in topic over last 24 months", fontsize=11)
    ax.set_ylabel("Activity factor A (capped at 1.0)", fontsize=11)
    ax.set_xlim(0, 200)
    ax.set_ylim(0, 1.15)
    ax.set_title("Factor A: log-scaled activity, capped at 50 reviews",
                 fontsize=14, fontweight="bold", color=TEXT_DARK, pad=12)
    _style_light(ax)
    plt.tight_layout()
    out = ASSETS / "chart_activity.png"
    plt.savefig(out, bbox_inches="tight", facecolor=WHITE)
    plt.close()
    print(f"wrote {out}")


# 9. Recency decay curve
def chart_recency_curve():
    fig, ax = plt.subplots(figsize=(11, 5.4), dpi=150)
    fig.patch.set_facecolor(WHITE)
    days = np.arange(0, 1500)
    r = np.maximum(0.3, np.exp(-days / 730))
    ax.plot(days, r, color=TEAL, linewidth=3)
    ax.axhline(0.3, color=AMBER, linestyle="--", alpha=0.6,
               label="floor = 0.3")
    ax.axvline(730, color=CORAL, linestyle="--", alpha=0.6,
               label="half-life = 730 days")
    for x_pt, label in [(0, "Day 0\n1.0"),
                        (730, "2 years\n0.5"),
                        (1460, "4 years\n0.3 (floor)")]:
        y = max(0.3, np.exp(-x_pt / 730))
        ax.scatter([x_pt], [y], color=CORAL, s=120, zorder=5,
                   edgecolor=MID)
        ax.annotate(label, xy=(x_pt, y),
                    xytext=(x_pt + 30, y + 0.05),
                    fontsize=10, fontweight="bold", color=CORAL)
    ax.set_xlabel("Days since review published", fontsize=11)
    ax.set_ylabel("Recency factor R", fontsize=11)
    ax.set_xlim(0, 1500)
    ax.set_ylim(0, 1.15)
    ax.legend(loc="upper right", fontsize=10)
    ax.set_title("Factor R: exponential decay with 2-year half-life, "
                 "floor 0.3",
                 fontsize=14, fontweight="bold", color=TEXT_DARK, pad=12)
    _style_light(ax)
    plt.tight_layout()
    out = ASSETS / "chart_recency.png"
    plt.savefig(out, bbox_inches="tight", facecolor=WHITE)
    plt.close()
    print(f"wrote {out}")


# 10. Cost-of-attack comparison
def chart_attack_cost():
    fig, ax = plt.subplots(figsize=(12, 5.5), dpi=150)
    fig.patch.set_facecolor(WHITE)
    cats = ["Cost to create\n100 fake accounts",
            "Cost to move\nglobal rating\nby 0.5 stars",
            "Cost to move\none observer's\nrelativistic rating",
            "Cost to move\n100 observers'\nrelativistic ratings"]
    costs = [10, 1000, 5000, 500000]
    colors = [TEXT_MUTED, CORAL, AMBER, EMERALD]
    bars = ax.bar(cats, costs, color=colors, edgecolor=MID,
                  linewidth=1.2, width=0.55)
    ax.set_yscale("log")
    ax.set_ylabel("USD cost (log scale)", fontsize=11)
    for bar, v in zip(bars, costs):
        if v >= 1000:
            label = f"${v/1000:.0f}k"
        else:
            label = f"${v}"
        ax.text(bar.get_x() + bar.get_width() / 2,
                bar.get_height() * 1.3, label,
                ha="center", va="bottom", fontsize=12,
                fontweight="bold", color=TEXT_DARK)
    ax.set_title(
        "Cost asymmetry: relativistic ratings change the attacker math",
        fontsize=14, fontweight="bold", color=TEXT_DARK, pad=12)
    _style_light(ax)
    plt.setp(ax.get_xticklabels(), fontsize=10)
    plt.tight_layout()
    out = ASSETS / "chart_attack_cost.png"
    plt.savefig(out, bbox_inches="tight", facecolor=WHITE)
    plt.close()
    print(f"wrote {out}")


# 11. Aurora / Constellation / Trace visual primitives mock
def chart_three_primitives():
    fig, axes = plt.subplots(1, 3, figsize=(13, 4.8), dpi=150)
    fig.patch.set_facecolor(WHITE)

    # AURORA — single dot with confidence ring
    ax = axes[0]
    ax.set_title("\u003Cqn-aurora\u003E", fontsize=14,
                 fontweight="bold", color=TEAL, pad=8, family="monospace")
    # Outer ring (confidence)
    ring = Circle((0.5, 0.55), 0.27, facecolor="none",
                  edgecolor=TEAL, linewidth=8, alpha=0.6)
    ax.add_patch(ring)
    # Center dot
    dot = Circle((0.5, 0.55), 0.18, facecolor=EMERALD,
                 edgecolor=MID, linewidth=2)
    ax.add_patch(dot)
    ax.text(0.5, 0.55, "4.5", ha="center", va="center",
            fontsize=20, fontweight="bold", color=WHITE)
    # Delta chip
    chip = FancyBboxPatch((0.78, 0.5), 0.18, 0.12,
                          boxstyle="round,pad=0.01,rounding_size=0.05",
                          facecolor=AMBER, edgecolor=MID, linewidth=1)
    ax.add_patch(chip)
    ax.text(0.87, 0.56, "\u2191 0.4", ha="center", va="center",
            fontsize=10, fontweight="bold", color=WHITE)
    ax.text(0.5, 0.15,
            "Single glanceable indicator.\n"
            "Encodes rating, confidence, directness, delta.",
            ha="center", va="center", fontsize=10, color=TEXT_DARK)
    ax.set_xlim(0, 1)
    ax.set_ylim(0, 1)
    ax.set_aspect("equal")
    ax.axis("off")
    ax.set_facecolor(LIGHT_BG)
    ax.add_patch(FancyBboxPatch((0.02, 0.02), 0.96, 0.96,
                                boxstyle="round,pad=0.01,rounding_size=0.04",
                                facecolor=LIGHT_BG, edgecolor=GRID,
                                linewidth=1))

    # CONSTELLATION — bullseye with reviewer dots
    ax = axes[1]
    ax.set_title("\u003Cqn-constellation\u003E", fontsize=14,
                 fontweight="bold", color=TEAL, pad=8, family="monospace")
    # Tier rings
    for r, alpha in [(0.42, 0.15), (0.32, 0.2),
                     (0.22, 0.25), (0.12, 0.3)]:
        ring = Circle((0.5, 0.55), r, facecolor=TEAL,
                      edgecolor="none", alpha=alpha)
        ax.add_patch(ring)
    # You at center
    you = Circle((0.5, 0.55), 0.025, facecolor=DARK_BG,
                 edgecolor="none")
    ax.add_patch(you)
    # Reviewer dots at various distances
    np.random.seed(42)
    for tier_r, n_dots, sentiment in [(0.13, 2, EMERALD),
                                      (0.23, 3, EMERALD),
                                      (0.33, 4, AMBER),
                                      (0.42, 5, EMERALD)]:
        for i in range(n_dots):
            angle = 2 * np.pi * (i + 0.3) / n_dots
            x = 0.5 + tier_r * np.cos(angle)
            y = 0.55 + tier_r * np.sin(angle)
            color = sentiment if np.random.rand() > 0.2 else CORAL
            d = Circle((x, y), 0.018, facecolor=color,
                       edgecolor=MID, linewidth=0.8)
            ax.add_patch(d)
    ax.text(0.5, 0.15,
            "Drilldown: every contributing reviewer as a dot.\n"
            "Position = trust proximity. Color = rating.",
            ha="center", va="center", fontsize=10, color=TEXT_DARK)
    ax.set_xlim(0, 1)
    ax.set_ylim(0, 1)
    ax.set_aspect("equal")
    ax.axis("off")
    ax.add_patch(FancyBboxPatch((0.02, 0.02), 0.96, 0.96,
                                boxstyle="round,pad=0.01,rounding_size=0.04",
                                facecolor=LIGHT_BG, edgecolor=GRID,
                                linewidth=1))

    # TRACE — horizontal stacked bar
    ax = axes[2]
    ax.set_title("\u003Cqn-trace\u003E", fontsize=14,
                 fontweight="bold", color=TEAL, pad=8, family="monospace")
    # Multi-product comparison
    products = ["Product A", "Product B", "Product C", "Product D"]
    segments = [
        [(EMERALD, 0.4), (EMERALD, 0.35), (TEXT_MUTED, 0.15),
         (TEXT_MUTED, 0.1)],
        [(EMERALD, 0.2), (TEXT_MUTED, 0.2), (TEXT_MUTED, 0.4),
         (CORAL, 0.2)],
        [(EMERALD, 0.5), (CORAL, 0.3), (EMERALD, 0.15),
         (CORAL, 0.05)],
        [(TEXT_MUTED, 0.1), (TEXT_MUTED, 0.05), (TEXT_MUTED, 0.05),
         (TEXT_MUTED, 0.0)],
    ]
    bar_h = 0.08
    for i, (prod, segs) in enumerate(zip(products, segments)):
        y = 0.75 - i * 0.14
        ax.text(0.05, y + bar_h / 2, prod,
                fontsize=10, fontweight="bold", color=TEXT_DARK,
                va="center")
        x = 0.3
        total_w = 0.6
        for color, frac in segs:
            ax.add_patch(FancyBboxPatch(
                (x, y), total_w * frac, bar_h,
                boxstyle="square,pad=0",
                facecolor=color, edgecolor=WHITE, linewidth=1))
            x += total_w * frac
    ax.text(0.5, 0.1,
            "Side-by-side composition.\n"
            "Width = weight share. Color = rating.",
            ha="center", va="center", fontsize=10, color=TEXT_DARK)
    ax.set_xlim(0, 1)
    ax.set_ylim(0, 1)
    ax.set_aspect(0.8)
    ax.axis("off")
    ax.add_patch(FancyBboxPatch((0.02, 0.02), 0.96, 0.96,
                                boxstyle="round,pad=0.01,rounding_size=0.04",
                                facecolor=LIGHT_BG, edgecolor=GRID,
                                linewidth=1))

    plt.tight_layout()
    out = ASSETS / "chart_three_primitives.png"
    plt.savefig(out, bbox_inches="tight", facecolor=WHITE)
    plt.close()
    print(f"wrote {out}")


# 12. Social science citation timeline
def chart_social_science_timeline():
    fig, ax = plt.subplots(figsize=(13, 5.5), dpi=150)
    fig.patch.set_facecolor(WHITE)

    studies = [
        (1951, "Asch", "Conformity"),
        (1954, "Festinger", "Social comparison"),
        (1963, "Milgram", "Obedience / small world"),
        (1973, "Granovetter", "Weak ties"),
        (1984, "Cialdini", "Influence / social proof"),
        (1995, "Mayer/Davis/Schoorman", "Trust model"),
        (1995, "Fukuyama", "Trust as social capital"),
        (2000, "Putnam", "Bowling Alone"),
        (2000, "Resnick et al.", "Reputation systems"),
        (2003, "Dellarocas", "Online word of mouth"),
        (2007, "Josang/Ismail/Boyd", "Trust survey"),
    ]

    # Plot each citation as a vertical bar
    for year, author, topic in studies:
        ax.axvline(year, color=TEAL, alpha=0.25, linewidth=2)
        # Stagger labels
        idx = studies.index((year, author, topic))
        y_pos = 0.7 + (idx % 5) * 0.14
        ax.text(year, y_pos, f"{author}\n{topic}",
                rotation=0, ha="center", va="bottom",
                fontsize=9, fontweight="bold", color=TEXT_DARK,
                bbox=dict(facecolor=WHITE, edgecolor=TEAL,
                          boxstyle="round,pad=0.2"))
        ax.scatter([year], [0.5], s=80, color=TEAL,
                   edgecolor=MID, linewidth=1.5, zorder=5)

    ax.set_xlim(1948, 2010)
    ax.set_ylim(0.3, 1.55)
    ax.set_yticks([])
    ax.set_xlabel("Year", fontsize=11, color=TEXT_DARK)
    ax.set_title(
        "Six decades of social science: every result aligns with the "
        "relativistic model",
        fontsize=14, fontweight="bold", color=TEXT_DARK, pad=12)
    for spine in ["left", "right", "top"]:
        ax.spines[spine].set_visible(False)
    ax.spines["bottom"].set_color(TEXT_MUTED)
    ax.tick_params(colors=TEXT_MUTED)

    plt.tight_layout()
    out = ASSETS / "chart_social_timeline.png"
    plt.savefig(out, bbox_inches="tight", facecolor=WHITE)
    plt.close()
    print(f"wrote {out}")


if __name__ == "__main__":
    chart_j_curve()
    chart_asch()
    chart_weak_ties()
    chart_fraud_market()
    chart_detection_ceiling()
    chart_two_observers()
    chart_four_factors()
    chart_activity_curve()
    chart_recency_curve()
    chart_attack_cost()
    chart_three_primitives()
    chart_social_science_timeline()
    print("all charts written.")
