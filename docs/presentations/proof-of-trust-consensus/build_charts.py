"""Chart generators for the Proof of Trust vs Consensus deck."""
import pathlib
import matplotlib.pyplot as plt
import matplotlib.patches as mpatches
from matplotlib.patches import FancyBboxPatch, FancyArrowPatch
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


# 1. Consensus family taxonomy tree (as a tree diagram)
def chart_consensus_taxonomy():
    fig, ax = plt.subplots(figsize=(13, 6.5), dpi=150)
    fig.patch.set_facecolor(WHITE)

    def box(x, y, w, h, text, color, tc=WHITE, fs=10):
        rect = FancyBboxPatch(
            (x - w / 2, y - h / 2), w, h,
            boxstyle="round,pad=0.02,rounding_size=0.1",
            facecolor=color, edgecolor=MID, linewidth=1.2)
        ax.add_patch(rect)
        ax.text(x, y, text, ha="center", va="center",
                fontsize=fs, fontweight="bold", color=tc)

    def line(p1, p2):
        ax.plot([p1[0], p2[0]], [p1[1], p2[1]],
                color=MID, linewidth=1.2, alpha=0.5, zorder=0)

    # Root
    box(6.5, 6, 3.2, 0.65, "Distributed Consensus", DARK_BG, fs=12)

    # Categories
    cats = [
        (1.5, 4.5, "Classical BFT\n(known set)", MID),
        (4.5, 4.5, "Nakamoto-style\n(anonymous)", MID),
        (7.5, 4.5, "Federated BFT\n(chosen sets)", MID),
        (10.5, 4.5, "DAG-based", MID),
        (12.6, 2.5, "PoA\n(named list)", MID),
        (2.7, 1.6, "Relational\n(observer-chosen)", TEAL),
    ]
    for x, y, name, c in cats:
        box(x, y, 2.2, 0.9, name, c, fs=10.5)
        line((6.5, 5.6), (x, y + 0.45))

    # Leaves
    leaves = [
        (0.7, 3, "PBFT\n1999", MID, 9.5, (1.5, 4.0)),
        (1.5, 3, "Tendermint\n2014", MID, 9.5, (1.5, 4.0)),
        (2.3, 3, "HotStuff\n2019", MID, 9.5, (1.5, 4.0)),
        (3.7, 3, "PoW\nBitcoin", MID, 9.5, (4.5, 4.0)),
        (5.3, 3, "PoS\nEthereum", MID, 9.5, (4.5, 4.0)),
        (6.7, 3, "Ripple\n2012", MID, 9.5, (7.5, 4.0)),
        (8.3, 3, "Stellar\nSCP", MID, 9.5, (7.5, 4.0)),
        (9.7, 3, "Tangle", MID, 9.5, (10.5, 4.0)),
        (10.7, 3, "Hashgraph", MID, 9.5, (10.5, 4.0)),
        (11.7, 3, "Avalanche", MID, 9.5, (10.5, 4.0)),
        (1.6, 0.7, "PGP Web\nof Trust", TEAL_SOFT, 9, (2.7, 1.2)),
        (3.8, 0.7, "Quidnug\nPoT", TEAL, 9.5, (2.7, 1.2)),
    ]
    for x, y, name, c, fs, par in leaves:
        tc = WHITE if c in (MID, TEAL, DARK_BG) else TEXT_DARK
        box(x, y, 1.4, 0.7, name, c, tc=tc, fs=fs)
        line(par, (x, y + 0.35))

    ax.set_xlim(0, 14)
    ax.set_ylim(0.3, 6.6)
    ax.set_aspect("equal")
    ax.axis("off")
    ax.set_title("Consensus mechanisms: a family tree",
                 fontsize=15, fontweight="bold", color=TEXT_DARK, pad=10)

    plt.tight_layout()
    out = ASSETS / "chart_taxonomy.png"
    plt.savefig(out, bbox_inches="tight", facecolor=WHITE)
    plt.close()
    print(f"wrote {out}")


# 2. Bitcoin energy consumption comparison
def chart_energy_consumption():
    fig, ax = plt.subplots(figsize=(12, 6), dpi=150)
    fig.patch.set_facecolor(WHITE)

    categories = ["Bitcoin PoW", "Argentina", "Poland",
                  "Ethereum\n(pre-merge)", "Global\ngold mining",
                  "Ethereum\n(post-merge)", "Quidnug PoT\n(est)"]
    values = [155, 125, 140, 78, 131, 0.01, 0.003]
    colors = [CORAL, TEXT_MUTED, TEXT_MUTED, AMBER, TEXT_MUTED,
              EMERALD, TEAL]

    bars = ax.barh(categories, values, color=colors, edgecolor=MID,
                   linewidth=1.2)
    ax.set_xscale("symlog", linthresh=0.01)
    ax.set_xlabel("Annual electricity consumption (TWh, log scale)",
                  fontsize=11)
    ax.set_title("Annual electricity consumption, 2024",
                 fontsize=15, fontweight="bold", color=TEXT_DARK, pad=12)
    for bar, v in zip(bars, values):
        if v >= 1:
            label = f"{v:.0f} TWh"
        elif v >= 0.01:
            label = f"{v*1000:.0f} GWh"
        else:
            label = f"{v*1000:.1f} GWh"
        ax.text(v * 1.1, bar.get_y() + bar.get_height() / 2,
                label, va="center", fontsize=10, fontweight="bold",
                color=TEXT_DARK)
    _style_light(ax)

    plt.tight_layout()
    out = ASSETS / "chart_energy.png"
    plt.savefig(out, bbox_inches="tight", facecolor=WHITE)
    plt.close()
    print(f"wrote {out}")


# 3. Finality latency
def chart_finality_latency():
    fig, ax = plt.subplots(figsize=(12, 5.5), dpi=150)
    fig.patch.set_facecolor(WHITE)

    systems = ["PBFT /\nHotStuff", "Tendermint", "Algorand",
               "Avalanche", "Ethereum PoS\n(2 epochs)",
               "Bitcoin PoW\n(6 conf)",
               "Quidnug PoT\n(block interval)"]
    latencies_s = [0.3, 2.0, 4.5, 1.5, 768, 3600, 60]
    colors = [EMERALD] * 4 + [AMBER, CORAL, TEAL]

    bars = ax.bar(systems, latencies_s, color=colors,
                  edgecolor=MID, linewidth=1)
    ax.set_yscale("log")
    ax.set_ylabel("Latency to finality (seconds, log)", fontsize=11)
    ax.set_title("Finality latency: log scale tells the story",
                 fontsize=15, fontweight="bold", color=TEXT_DARK, pad=12)

    for bar, v in zip(bars, latencies_s):
        if v >= 60:
            label = f"{v/60:.1f} min" if v < 3600 else f"{v/60:.0f} min"
        else:
            label = f"{v:.1f} s" if v < 10 else f"{v:.0f} s"
        ax.text(bar.get_x() + bar.get_width() / 2,
                bar.get_height() * 1.2, label,
                ha="center", va="bottom", fontsize=9,
                fontweight="bold", color=TEXT_DARK)
    _style_light(ax)
    plt.setp(ax.get_xticklabels(), fontsize=9)

    plt.tight_layout()
    out = ASSETS / "chart_finality.png"
    plt.savefig(out, bbox_inches="tight", facecolor=WHITE)
    plt.close()
    print(f"wrote {out}")


# 4. Throughput TPS
def chart_throughput():
    fig, ax = plt.subplots(figsize=(12, 5.5), dpi=150)
    fig.patch.set_facecolor(WHITE)

    systems = ["Bitcoin\nPoW", "Ethereum\nL1", "Tendermint",
               "Algorand", "HotStuff\n(Diem)", "Hedera\nHashgraph",
               "Avalanche", "Solana\n(bursty)", "Quidnug\n(per domain)"]
    tps = [7, 15, 1000, 1300, 1500, 10000, 4500, 65000, 5000]
    colors = [TEXT_MUTED, TEXT_MUTED, EMERALD, EMERALD, EMERALD,
              EMERALD, EMERALD, AMBER, TEAL]

    bars = ax.bar(systems, tps, color=colors, edgecolor=MID, linewidth=1)
    ax.set_yscale("log")
    ax.set_ylabel("Transactions per second (log scale)", fontsize=11)
    ax.set_title("Peak throughput by consensus mechanism",
                 fontsize=15, fontweight="bold", color=TEXT_DARK, pad=12)

    for bar, v in zip(bars, tps):
        ax.text(bar.get_x() + bar.get_width() / 2,
                bar.get_height() * 1.2, f"{v:,}",
                ha="center", va="bottom", fontsize=9,
                fontweight="bold", color=TEXT_DARK)
    _style_light(ax)
    plt.setp(ax.get_xticklabels(), fontsize=9)

    plt.tight_layout()
    out = ASSETS / "chart_throughput.png"
    plt.savefig(out, bbox_inches="tight", facecolor=WHITE)
    plt.close()
    print(f"wrote {out}")


# 5. Trust graph visualization (two observers, divergent views)
def chart_divergent_views():
    fig, axes = plt.subplots(1, 2, figsize=(13, 5.5), dpi=150)
    fig.patch.set_facecolor(WHITE)

    def draw_graph(ax, observer, target, paths, title, highlight_color):
        # paths: list of (label, weight, nodes_between)
        ax.set_title(title, fontsize=13, fontweight="bold",
                     color=TEXT_DARK, pad=10)

        def box(x, y, text, color=MID, size=(0.9, 0.5)):
            rect = FancyBboxPatch(
                (x - size[0] / 2, y - size[1] / 2),
                size[0], size[1],
                boxstyle="round,pad=0.02,rounding_size=0.1",
                facecolor=color, edgecolor="black", linewidth=1.2)
            ax.add_patch(rect)
            ax.text(x, y, text, ha="center", va="center",
                    fontsize=10, fontweight="bold", color=WHITE)

        def arrow(p1, p2, label, color=EMERALD):
            arr = FancyArrowPatch(p1, p2, arrowstyle="->",
                                  mutation_scale=15, linewidth=1.4,
                                  color=color)
            ax.add_patch(arr)
            mx = (p1[0] + p2[0]) / 2
            my = (p1[1] + p2[1]) / 2 + 0.2
            ax.text(mx, my, label, ha="center", va="center",
                    fontsize=9, fontweight="bold", color=color,
                    bbox=dict(facecolor=WHITE, edgecolor="none", pad=1))

        box(1, 3, observer, highlight_color)
        box(5, 3, target, TEAL)
        for i, (label, w, intermediate) in enumerate(paths):
            # Draw intermediate
            ix = 3
            iy = 3 + (i - len(paths) / 2 + 0.5) * 1.5
            box(ix, iy, intermediate, MID, size=(1.0, 0.5))
            arrow((1.45, 3), (2.55, iy), f"{w[0]:.2f}",
                  color=highlight_color)
            arrow((3.45, iy), (4.55, 3), f"{w[1]:.2f}",
                  color=highlight_color)

        # Composite trust
        best = max(w[0] * w[1] for _, w, _ in paths)
        ax.text(3, 0.5, f"Best composite trust: {best:.3f}",
                ha="center", va="center", fontsize=12,
                fontweight="bold", color=highlight_color,
                bbox=dict(facecolor=WHITE, edgecolor=highlight_color,
                          boxstyle="round,pad=0.3"))

        ax.set_xlim(0, 6)
        ax.set_ylim(0, 6)
        ax.set_aspect("equal")
        ax.axis("off")

    draw_graph(axes[0], "Alice", "Carol",
               paths=[("Alice path", (0.9, 0.7), "Bob")],
               title="Alice's view: Carol = 0.63", highlight_color=EMERALD)
    draw_graph(axes[1], "Dave", "Carol",
               paths=[("Dave path", (0.4, 0.7), "Bob")],
               title="Dave's view: Carol = 0.28", highlight_color=AMBER)

    plt.tight_layout()
    out = ASSETS / "chart_divergent_views.png"
    plt.savefig(out, bbox_inches="tight", facecolor=WHITE)
    plt.close()
    print(f"wrote {out}")


# 6. Tiered block acceptance state diagram
def chart_tiered_acceptance():
    fig, ax = plt.subplots(figsize=(12, 5.8), dpi=150)
    fig.patch.set_facecolor(WHITE)

    def box(x, y, w, h, text, color, tc=WHITE, fs=10):
        rect = FancyBboxPatch((x - w / 2, y - h / 2), w, h,
                              boxstyle="round,pad=0.02,rounding_size=0.1",
                              facecolor=color, edgecolor=MID,
                              linewidth=1.5)
        ax.add_patch(rect)
        ax.text(x, y, text, ha="center", va="center",
                fontsize=fs, fontweight="bold", color=tc)

    def arrow(p1, p2, label=None, color=MID):
        arr = FancyArrowPatch(p1, p2, arrowstyle="->",
                              mutation_scale=18, linewidth=1.5,
                              color=color)
        ax.add_patch(arr)
        if label:
            mx = (p1[0] + p2[0]) / 2
            my = (p1[1] + p2[1]) / 2
            ax.text(mx, my, label, ha="center", va="center",
                    fontsize=9, fontweight="bold", color=TEXT_DARK,
                    bbox=dict(facecolor=WHITE, edgecolor=color,
                              boxstyle="round,pad=0.15"))

    box(1.5, 4, 2.2, 0.8, "Incoming\nblock", MID)
    box(5.5, 4, 2.6, 0.8, "Crypto check\n(signatures, Merkle)",
        MID, fs=10)
    box(9.5, 4, 2.4, 0.8, "Eval validator\ntrust per observer",
        MID, fs=10)

    box(12.5, 5.4, 1.8, 0.7, "Trusted", EMERALD, fs=11)
    box(12.5, 4, 1.8, 0.7, "Tentative", AMBER, fs=11)
    box(12.5, 2.6, 1.8, 0.7, "Rejected", CORAL, fs=11)

    arrow((2.65, 4), (4.15, 4))
    arrow((6.85, 4), (8.25, 4), "pass")
    arrow((6.85, 3.8), (8, 2.5), "fail", color=CORAL)
    box(8, 2.3, 1.5, 0.5, "Dropped", CORAL, fs=9.5)

    arrow((10.7, 4.3), (11.5, 5.4), "≥ threshold", color=EMERALD)
    arrow((10.7, 4), (11.5, 4), "0 < t < th", color=AMBER)
    arrow((10.7, 3.7), (11.5, 2.6), "= 0", color=CORAL)

    ax.text(7, 0.7,
            "Each observer chooses their threshold. "
            "Different observers can classify the same block differently.",
            ha="center", va="center", fontsize=11, style="italic",
            color=TEXT_MUTED)

    ax.set_xlim(0, 15)
    ax.set_ylim(0, 6)
    ax.set_aspect("equal")
    ax.axis("off")
    ax.set_title("Tiered block acceptance: per-observer classification",
                 fontsize=14, fontweight="bold", color=TEXT_DARK, pad=10)

    plt.tight_layout()
    out = ASSETS / "chart_tiered.png"
    plt.savefig(out, bbox_inches="tight", facecolor=WHITE)
    plt.close()
    print(f"wrote {out}")


# 7. BFT vs Nakamoto vs PoT: big comparison
def chart_comparison_matrix():
    fig, ax = plt.subplots(figsize=(13, 5.8), dpi=150)
    fig.patch.set_facecolor(WHITE)

    mechanisms = ["PoW", "PoS", "PBFT / HotStuff", "FBA (Stellar)",
                  "DAG", "PoT (Quidnug)"]
    criteria = ["Energy\nefficiency", "Latency\nto finality",
                "Throughput", "Permissionless",
                "Byzantine\ntolerance", "Privacy",
                "Non-money fit"]

    data = np.array([
        [0, 0, 0, 2, 2, 0, 0],    # PoW
        [2, 1, 1, 2, 2, 0, 1],    # PoS
        [2, 2, 2, 0, 2, 1, 2],    # PBFT
        [2, 2, 1, 1, 2, 1, 2],    # FBA
        [2, 2, 2, 1, 1, 1, 2],    # DAG
        [2, 2, 2, 0, 2, 2, 2],    # PoT
    ])

    cmap = {0: CORAL, 1: AMBER, 2: EMERALD}
    labels = {0: "poor", 1: "ok", 2: "good"}

    cell_w, cell_h = 1.55, 0.72
    for i, mech in enumerate(mechanisms):
        for j, crit in enumerate(criteria):
            rect = FancyBboxPatch(
                (j * cell_w, -i * cell_h),
                cell_w * 0.95, cell_h * 0.9,
                boxstyle="round,pad=0.02,rounding_size=0.08",
                linewidth=1.2,
                edgecolor=WHITE,
                facecolor=cmap[data[i][j]])
            ax.add_patch(rect)
            ax.text(j * cell_w + cell_w * 0.48,
                    -i * cell_h + cell_h * 0.45,
                    labels[data[i][j]], ha="center", va="center",
                    fontsize=10, fontweight="bold", color=WHITE)

    for j, crit in enumerate(criteria):
        ax.text(j * cell_w + cell_w * 0.48, 0.75, crit,
                ha="center", va="bottom",
                fontsize=10.5, fontweight="bold", color=TEXT_DARK)

    for i, mech in enumerate(mechanisms):
        ax.text(-0.2, -i * cell_h + cell_h * 0.45, mech,
                ha="right", va="center",
                fontsize=12, fontweight="bold", color=TEXT_DARK)

    ax.set_xlim(-3.3, len(criteria) * cell_w + 0.2)
    ax.set_ylim(-len(mechanisms) * cell_h - 0.3, 1.5)
    ax.set_aspect("equal")
    ax.axis("off")
    ax.set_title("Six-metric comparison: consensus mechanisms",
                 fontsize=14, fontweight="bold", color=TEXT_DARK, pad=10)

    plt.tight_layout()
    out = ASSETS / "chart_comparison.png"
    plt.savefig(out, bbox_inches="tight", facecolor=WHITE)
    plt.close()
    print(f"wrote {out}")


# 8. Decision tree for picking a consensus mechanism
def chart_decision_tree():
    fig, ax = plt.subplots(figsize=(13, 6.2), dpi=150)
    fig.patch.set_facecolor(WHITE)

    def box(x, y, text, color=MID, size=(2.3, 0.7), tc=WHITE, fs=10.5):
        rect = FancyBboxPatch(
            (x - size[0] / 2, y - size[1] / 2),
            size[0], size[1],
            boxstyle="round,pad=0.02,rounding_size=0.1",
            facecolor=color, edgecolor=MID, linewidth=1.4)
        ax.add_patch(rect)
        ax.text(x, y, text, ha="center", va="center",
                fontsize=fs, fontweight="bold", color=tc)

    def dec(x, y, text):
        rect = FancyBboxPatch(
            (x - 1.6, y - 0.45), 3.2, 0.9,
            boxstyle="round,pad=0.02,rounding_size=0.15",
            facecolor=WHITE, edgecolor=MID, linewidth=1.5)
        ax.add_patch(rect)
        ax.text(x, y, text, ha="center", va="center",
                fontsize=10, fontweight="bold", color=TEXT_DARK)

    def arrow(p1, p2, label=None, color=MID):
        arr = FancyArrowPatch(p1, p2, arrowstyle="->",
                              mutation_scale=15, linewidth=1.3,
                              color=color)
        ax.add_patch(arr)
        if label:
            mx = (p1[0] + p2[0]) / 2
            my = (p1[1] + p2[1]) / 2
            ax.text(mx, my, label, ha="center", va="center",
                    fontsize=9, fontweight="bold", color=color,
                    bbox=dict(facecolor=WHITE, edgecolor="none", pad=1))

    dec(6.5, 5.8, "Transferring tokenized\nvalue?")

    dec(2.5, 4, "Permissionless users?")
    dec(10.5, 4, "All observers need\nidentical state?")

    box(0.5, 2.3, "Bitcoin / L2s", AMBER, fs=10.5)
    box(4.5, 2.3, "PBFT /\nHotStuff /\nTendermint", MID, fs=10.5)

    dec(10.5, 2.5, "Known validators?")

    box(8.5, 0.8, "Ethereum PoS", MID, fs=10.5)
    box(12.5, 0.8, "PBFT /\nHotStuff", MID, fs=10.5)

    dec(10.5, 0.8, "Trust or\nreputation?")
    box(9, -0.8, "Kafka / Kinesis", TEXT_MUTED, fs=10.5)
    box(12, -0.8, "PoT (Quidnug)", TEAL, fs=11)

    arrow((6.5, 5.45), (2.5, 4.45), "Yes", color=AMBER)
    arrow((6.5, 5.45), (10.5, 4.45), "No", color=TEAL)
    arrow((2.5, 3.55), (0.5, 2.65), "Yes", color=AMBER)
    arrow((2.5, 3.55), (4.5, 2.65), "No", color=TEAL)
    arrow((10.5, 3.55), (10.5, 2.95), "Yes", color=AMBER)
    arrow((10.5, 3.55), (10.5, 1.25), "No", color=TEAL)
    arrow((10.5, 2.05), (8.5, 1.15), "No", color=AMBER)
    arrow((10.5, 2.05), (12.5, 1.15), "Yes", color=TEAL)
    arrow((10.5, 0.35), (9, -0.5), "Event streams", color=TEXT_MUTED)
    arrow((10.5, 0.35), (12, -0.5), "Trust / reputation", color=TEAL)

    ax.set_xlim(-1, 14)
    ax.set_ylim(-1.5, 6.5)
    ax.set_aspect("equal")
    ax.axis("off")
    ax.set_title("Decision tree: which consensus mechanism?",
                 fontsize=14, fontweight="bold", color=TEXT_DARK, pad=10)

    plt.tight_layout()
    out = ASSETS / "chart_decision.png"
    plt.savefig(out, bbox_inches="tight", facecolor=WHITE)
    plt.close()
    print(f"wrote {out}")


# 9. Trust graph with multiplicative decay
def chart_trust_decay():
    fig, ax = plt.subplots(figsize=(12, 5.5), dpi=150)
    fig.patch.set_facecolor(WHITE)

    # Show chain of trust with diminishing edges
    positions = [(1, 3), (3.5, 3.8), (6, 3), (8.5, 3.8), (11, 3)]
    labels = ["You", "Alice", "Bob", "Carol", "Dave"]
    weights = [0.9, 0.7, 0.8, 0.6]

    for (x, y), label in zip(positions, labels):
        circle = plt.Circle((x, y), 0.45, facecolor=TEAL,
                            edgecolor=MID, linewidth=1.5)
        ax.add_patch(circle)
        ax.text(x, y, label, ha="center", va="center",
                fontsize=11, fontweight="bold", color=WHITE)

    cumulative = 1.0
    for i in range(len(positions) - 1):
        p1 = positions[i]
        p2 = positions[i + 1]
        arr = FancyArrowPatch(
            (p1[0] + 0.45, p1[1] + (p2[1] - p1[1]) * 0.15),
            (p2[0] - 0.45, p2[1] + (p1[1] - p2[1]) * 0.15),
            arrowstyle="->", mutation_scale=18,
            linewidth=1.5 + weights[i] * 1.5,
            color=EMERALD,
            alpha=0.3 + weights[i] * 0.7)
        ax.add_patch(arr)
        cumulative *= weights[i]
        mx = (p1[0] + p2[0]) / 2
        my = (p1[1] + p2[1]) / 2 + 0.5
        ax.text(mx, my, f"w={weights[i]}\n"
                       f"cumulative\n={cumulative:.3f}",
                ha="center", va="center", fontsize=10,
                fontweight="bold", color=TEXT_DARK,
                bbox=dict(facecolor=WHITE, edgecolor=EMERALD,
                          boxstyle="round,pad=0.2"))

    ax.text(6, 0.8,
            f"Final trust from You to Dave: {cumulative:.3f}"
            f" (decay through 4 hops)",
            ha="center", va="center", fontsize=12,
            fontweight="bold", color=EMERALD,
            bbox=dict(facecolor="#F0FDF4", edgecolor=EMERALD,
                      boxstyle="round,pad=0.4"))

    ax.set_xlim(0, 12)
    ax.set_ylim(0.4, 5.2)
    ax.set_aspect("equal")
    ax.axis("off")
    ax.set_title("Multiplicative trust decay: T(P) = w₁ × w₂ × w₃ × w₄",
                 fontsize=14, fontweight="bold", color=TEXT_DARK, pad=10)

    plt.tight_layout()
    out = ASSETS / "chart_trust_decay.png"
    plt.savefig(out, bbox_inches="tight", facecolor=WHITE)
    plt.close()
    print(f"wrote {out}")


# 10. Sybil resistance comparison
def chart_sybil_resistance():
    fig, ax = plt.subplots(figsize=(12, 5.6), dpi=150)
    fig.patch.set_facecolor(WHITE)

    approaches = ["CAPTCHA", "PoW identity\ncreation",
                  "PoS identity\nstake", "Global\nreputation",
                  "PoT\ntrust graph"]
    cost_bots = [1, 100, 10000, 10, 10]
    cost_attacker = [1000, 50000, 1000000, 100000, 5000000]
    width = 0.36
    x = np.arange(len(approaches))

    ax.bar(x - width/2, cost_bots, width, color=CORAL,
           label="Cost to create 1000 Sybils",
           edgecolor=MID, linewidth=0.8)
    ax.bar(x + width/2, cost_attacker, width, color=EMERALD,
           label="Cost to move one observer's view",
           edgecolor=MID, linewidth=0.8)

    ax.set_yscale("log")
    ax.set_xticks(x)
    ax.set_xticklabels(approaches, fontsize=10)
    ax.set_ylabel("USD cost (log scale)", fontsize=11)
    ax.set_title(
        "Sybil-resistance approaches: cost asymmetry",
        fontsize=14, fontweight="bold", color=TEXT_DARK, pad=12)
    ax.legend(loc="upper left", fontsize=10)
    _style_light(ax)

    plt.tight_layout()
    out = ASSETS / "chart_sybil.png"
    plt.savefig(out, bbox_inches="tight", facecolor=WHITE)
    plt.close()
    print(f"wrote {out}")


# 11. MEV extraction over time
def chart_mev():
    fig, ax = plt.subplots(figsize=(12, 5.4), dpi=150)
    fig.patch.set_facecolor(WHITE)

    years = ["2020", "2021", "2022", "2023", "2024", "2025"]
    mev_millions = [78, 550, 1380, 680, 420, 310]

    bars = ax.bar(years, mev_millions, color=CORAL,
                  edgecolor=MID, linewidth=1.2)
    ax.set_ylabel("MEV extracted ($ millions / year)", fontsize=11)
    ax.set_title(
        "Ethereum MEV: $3.4B extracted from users 2020-2025",
        fontsize=14, fontweight="bold", color=TEXT_DARK, pad=12)
    for bar, v in zip(bars, mev_millions):
        ax.text(bar.get_x() + bar.get_width() / 2,
                bar.get_height() + 30,
                f"${v}M", ha="center", va="bottom",
                fontsize=11, fontweight="bold", color=CORAL)
    ax.set_ylim(0, 1600)
    _style_light(ax)
    ax.text(0.5, 1550, "Source: Flashbots MEV-Explore. 2022 includes the "
                       "Merge transition volatility.",
            fontsize=9, style="italic", color=TEXT_MUTED)

    plt.tight_layout()
    out = ASSETS / "chart_mev.png"
    plt.savefig(out, bbox_inches="tight", facecolor=WHITE)
    plt.close()
    print(f"wrote {out}")


# 12. Domain-scoped trust (tree diagram)
def chart_domain_scoping():
    fig, ax = plt.subplots(figsize=(13, 5.5), dpi=150)
    fig.patch.set_facecolor(WHITE)

    def box(x, y, text, color=MID, size=(2.0, 0.6), tc=WHITE, fs=10):
        rect = FancyBboxPatch(
            (x - size[0] / 2, y - size[1] / 2),
            size[0], size[1],
            boxstyle="round,pad=0.02,rounding_size=0.1",
            facecolor=color, edgecolor=MID, linewidth=1.4)
        ax.add_patch(rect)
        ax.text(x, y, text, ha="center", va="center",
                fontsize=fs, fontweight="bold", color=tc)

    def line(p1, p2, color=TEAL):
        ax.plot([p1[0], p2[0]], [p1[1], p2[1]],
                color=color, linewidth=1.3, zorder=0)

    box(6.5, 4.5, "Alice (the hospital)", DARK_BG, size=(2.8, 0.7), fs=11)

    specialists = [
        (1.8, 3.0, "Bob\n(doctor)", TEAL),
        (5.2, 3.0, "Carol\n(nurse)", TEAL),
        (8.6, 3.0, "Dave\n(lab tech)", TEAL),
        (11.5, 3.0, "Eve\n(records)", TEAL),
    ]
    domains = [
        (1.8, 1.3, "hospital.\nrecords", MID, (2.3, 0.7)),
        (5.2, 1.3, "hospital.\nconsents", MID, (2.3, 0.7)),
        (8.6, 1.3, "hospital.\nlabs", MID, (2.3, 0.7)),
        (11.5, 1.3, "finance.*\n(NO trust)", CORAL, (2.3, 0.7)),
    ]

    for x, y, label, color in specialists:
        box(x, y, label, color, fs=10)
        line((6.5, 4.1), (x, y + 0.3))

    for (x, y, label, color, size) in domains:
        box(x, y, label, color, size=size, fs=10)

    for (xs, ys, _, _), (xd, yd, _, _, _) in zip(specialists, domains):
        line((xs, ys - 0.3), (xd, yd + 0.35), color=EMERALD)

    ax.text(6.5, 0.2,
            "Alice trusting Bob in hospital.records says nothing about "
            "finance.* or legal.*",
            ha="center", va="center", fontsize=10, style="italic",
            color=TEXT_MUTED)

    ax.set_xlim(0, 13.5)
    ax.set_ylim(0, 5.2)
    ax.set_aspect("equal")
    ax.axis("off")
    ax.set_title("Domain scoping: trust is not globally transitive",
                 fontsize=14, fontweight="bold", color=TEXT_DARK, pad=10)

    plt.tight_layout()
    out = ASSETS / "chart_domain_scope.png"
    plt.savefig(out, bbox_inches="tight", facecolor=WHITE)
    plt.close()
    print(f"wrote {out}")


if __name__ == "__main__":
    chart_consensus_taxonomy()
    chart_energy_consumption()
    chart_finality_latency()
    chart_throughput()
    chart_divergent_views()
    chart_tiered_acceptance()
    chart_comparison_matrix()
    chart_decision_tree()
    chart_trust_decay()
    chart_sybil_resistance()
    chart_mev()
    chart_domain_scoping()
    print("all charts written.")
