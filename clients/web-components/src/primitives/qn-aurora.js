/**
 * <qn-aurora> — the headline rating glyph.
 *
 * A sentiment dot at the center, wrapped by a confidence ring,
 * with an optional delta chip and numeric rating. Same visual
 * vocabulary at three sizes (nano / standard / large) so scanning
 * a product list feels continuous with scanning a product detail
 * page.
 *
 * Attributes (all optional; sensible defaults so the tag never
 * throws):
 *
 *   rating              Number 0..5 (or 0..max if `max` set).
 *                       Omit for "no basis" rendering.
 *   max                 Scale maximum. Default 5.
 *   crowd               The public unweighted average. Used to
 *                       compute the delta chip. Omit to hide it.
 *   contributors        Number of trusted sources that contributed.
 *                       Drives the ring thickness. Default 0.
 *   direct              How many of those are direct (1-hop) trust.
 *                       Drives the ring stroke style. Default 0.
 *   size                "nano" | "standard" | "large". Default "standard".
 *   observer-name       Optional label rendered beneath the glyph
 *                       at `standard` / `large` sizes.
 *   show-value          If present, renders the numeric rating. On
 *                       by default for standard + large.
 *   show-delta          If present, renders the delta chip. On by
 *                       default for standard + large.
 *   show-histogram      If present, renders a faint radial
 *                       histogram of per-contributor ratings on the
 *                       ring. `contributor-ratings` must be set.
 *   contributor-ratings JSON-encoded array of numbers (one rating
 *                       per contributor). Optional; used only by
 *                       the histogram mode.
 *
 * CSS variables (all with safe fallbacks, see design-tokens.js):
 *   --qn-sentiment-*  --qn-delta-*  --qn-font
 *
 * Fires: `qn-aurora-click` on click (bubbling, composed), detail
 * contains the currently-shown rating so hosts can open a
 * drilldown.
 */

import {
    sentimentFor, deltaTokenFor, DELTA_TOKENS, DELTA_EPSILON,
    ringThicknessFor, ringStyleFor, SIZES, describeState,
} from "../design-tokens.js";

// Node-safe base: the renderer runs in SSR too. HTMLElement only
// exists in browsers; we only need a placeholder for `class extends`
// to work at module-eval time.
const BaseElement = typeof HTMLElement !== "undefined"
    ? HTMLElement
    : class { constructor() {} };

export class QnAuroraElement extends BaseElement {
    static get observedAttributes() {
        return [
            "rating", "max", "crowd", "contributors", "direct",
            "size", "observer-name", "show-value", "show-delta",
            "show-histogram", "contributor-ratings",
        ];
    }

    constructor() {
        super();
        this.attachShadow({ mode: "open" });
    }

    connectedCallback() {
        this.addEventListener("click", this._onClick);
        this.setAttribute("role", "button");
        this.setAttribute("tabindex", this.getAttribute("tabindex") ?? "0");
        this.addEventListener("keydown", this._onKey);
        this._render();
    }

    disconnectedCallback() {
        this.removeEventListener("click", this._onClick);
        this.removeEventListener("keydown", this._onKey);
    }

    attributeChangedCallback() {
        if (this.isConnected) this._render();
    }

    _onClick = () => {
        this.dispatchEvent(new CustomEvent("qn-aurora-click", {
            detail: this._readState(), bubbles: true, composed: true,
        }));
    };

    _onKey = (e) => {
        if (e.key === "Enter" || e.key === " ") {
            e.preventDefault();
            this._onClick();
        }
    };

    _readState() {
        const num = (k, d = null) => {
            const v = this.getAttribute(k);
            if (v == null || v === "") return d;
            const n = Number(v);
            return Number.isFinite(n) ? n : d;
        };
        const str = (k, d = null) => this.getAttribute(k) ?? d;

        let contributorRatings = null;
        const raw = str("contributor-ratings");
        if (raw) {
            try {
                const parsed = JSON.parse(raw);
                if (Array.isArray(parsed)) {
                    contributorRatings = parsed.map(Number).filter(Number.isFinite);
                }
            } catch { /* ignore malformed JSON */ }
        }

        const size = str("size", "standard");
        const sizeCfg = SIZES[size] ?? SIZES.standard;
        const rating = num("rating");
        const max = num("max", 5);

        return {
            rating,
            max,
            crowd: num("crowd"),
            contributorCount: num("contributors", 0) ?? 0,
            directCount: num("direct", 0) ?? 0,
            sizeName: size,
            sizeCfg,
            observerName: str("observer-name"),
            showValue: size !== "nano" || this.hasAttribute("show-value"),
            showDelta: this.hasAttribute("show-delta") ||
                (size !== "nano" && !this.hasAttribute("show-delta") &&
                 this.getAttribute("show-delta") !== "false"),
            showHistogram: this.hasAttribute("show-histogram"),
            contributorRatings,
            hasBasis: rating != null && Number.isFinite(rating),
        };
    }

    _render() {
        const s = this._readState();
        this.shadowRoot.innerHTML = `<style>${STYLES}</style>${renderSVG(s)}`;
        this.setAttribute("aria-label", describeState({
            rating: s.rating, max: s.max, personal: s.rating, crowd: s.crowd,
            contributorCount: s.contributorCount, directCount: s.directCount,
            hasBasis: s.hasBasis,
        }));
    }
}

/**
 * Pure SVG renderer. Separated from the element class so it can
 * be unit-tested and reused by server-side-rendering adapters.
 */
export function renderSVG(s) {
    const {
        rating, max, crowd, contributorCount, directCount,
        sizeCfg, sizeName, observerName, showValue, showDelta,
        showHistogram, contributorRatings, hasBasis,
    } = s;

    const outer = sizeCfg.outer;
    const thickness = hasBasis
        ? Math.min(sizeCfg.ringMax, ringThicknessFor(contributorCount))
        : Math.max(2, Math.floor(sizeCfg.ringMax / 2));
    const ringStyle = ringStyleFor({ contributorCount, directCount });
    const sentiment = sentimentFor(rating, max);

    const cx = outer / 2;
    const cy = outer / 2;
    const ringR = outer / 2 - thickness / 2 - 1;
    const dotR = ringR - thickness / 2 - 2;

    const ringDash = hasBasis ? ringStyle.dasharray : "1 3";
    const ringColor = hasBasis ? sentiment.color : "var(--qn-sentiment-no-basis, #B0B7BD)";
    const dotFill = hasBasis ? sentiment.color : "transparent";
    const dotStroke = hasBasis ? "none" : "var(--qn-sentiment-no-basis, #B0B7BD)";

    // Shape-redundant center icon (for accessibility).
    const shape = renderCenterShape({
        shape: hasBasis ? sentiment.shape : "dot",
        cx, cy, r: dotR * 0.8,
        fill: dotFill, stroke: dotStroke,
    });

    // Numeric rating digit, sits at center (overlays the shape
    // at standard/large sizes where there's room).
    const digit = (showValue && sizeName !== "nano" && hasBasis)
        ? `<text x="${cx}" y="${cy + sizeCfg.digitSize * 0.35}"
                 class="digit" style="font-size:${sizeCfg.digitSize}px"
                 text-anchor="middle">${Number(rating).toFixed(1)}</text>`
        : "";

    // Radial histogram of contributor ratings along the ring.
    const histogram = (showHistogram && contributorRatings &&
                       contributorRatings.length > 0 && hasBasis)
        ? renderRadialHistogram({
            ratings: contributorRatings, max, cx, cy, r: ringR,
            thickness, baseColor: sentiment.color,
        })
        : "";

    const svgBody = `
        <svg class="aurora" viewBox="0 0 ${outer} ${outer}"
             width="${outer}" height="${outer}"
             xmlns="http://www.w3.org/2000/svg">
            <title>${titleText(s)}</title>
            <circle class="ring" cx="${cx}" cy="${cy}" r="${ringR}"
                    fill="none" stroke="${ringColor}"
                    stroke-width="${thickness}"
                    stroke-dasharray="${ringDash}"
                    stroke-linecap="round"/>
            ${histogram}
            ${shape}
            ${digit}
        </svg>
    `;

    // Nano: just the SVG, maybe a tiny rating digit next to it.
    if (sizeName === "nano") {
        const nanoRating = (showValue && hasBasis)
            ? `<span class="nano-value">${Number(rating).toFixed(1)}</span>`
            : "";
        const nanoDelta = (showDelta && crowd != null && hasBasis)
            ? renderDeltaChip(rating, crowd, "nano")
            : "";
        return `<div class="wrap nano">${svgBody}${nanoRating}${nanoDelta}</div>`;
    }

    // Standard / large: SVG + optional text region below.
    const deltaChip = (showDelta && crowd != null && hasBasis)
        ? renderDeltaChip(rating, crowd, sizeName)
        : "";
    const meta = [];
    if (hasBasis && contributorCount > 0) {
        meta.push(`<span class="meta">
            ${contributorCount} trusted ${contributorCount === 1 ? "source" : "sources"}
        </span>`);
    } else if (!hasBasis) {
        meta.push(`<span class="meta no-basis">no basis yet</span>`);
    }
    if (observerName) {
        meta.push(`<span class="meta observer">viewing as ${escapeHtml(observerName)}</span>`);
    }

    return `
        <div class="wrap ${sizeName}">
            ${svgBody}
            <div class="text">
                ${deltaChip}
                ${meta.join(" · ")}
            </div>
        </div>
    `;
}

function titleText(s) {
    return describeState({
        rating: s.rating, max: s.max, personal: s.rating, crowd: s.crowd,
        contributorCount: s.contributorCount, directCount: s.directCount,
        hasBasis: s.hasBasis,
    });
}

function renderCenterShape({ shape, cx, cy, r, fill, stroke }) {
    const strokeAttr = stroke === "none"
        ? ""
        : `stroke="${stroke}" stroke-width="1.5"`;
    switch (shape) {
        case "square": {
            const side = r * 1.6;
            return `<rect class="shape square"
                x="${cx - side / 2}" y="${cy - side / 2}"
                width="${side}" height="${side}" rx="${side * 0.15}"
                fill="${fill}" ${strokeAttr}/>`;
        }
        case "triangle": {
            const h = r * 1.8;
            const w = r * 2;
            const y0 = cy - h / 2;
            const y1 = cy + h / 2;
            return `<polygon class="shape tri"
                points="${cx},${y0} ${cx - w / 2},${y1} ${cx + w / 2},${y1}"
                fill="${fill}" ${strokeAttr}/>`;
        }
        default:
            return `<circle class="shape dot"
                cx="${cx}" cy="${cy}" r="${r}"
                fill="${fill}" ${strokeAttr}/>`;
    }
}

/**
 * Render a thin radial histogram layered into the ring. Each
 * contributor is one wedge whose angular extent is even across
 * the ring, colored by their individual rating.
 */
function renderRadialHistogram({ ratings, max, cx, cy, r, thickness, baseColor }) {
    const n = ratings.length;
    if (n === 0) return "";
    const angleEach = (Math.PI * 2) / n;
    const gapRad = Math.min(0.08, angleEach * 0.15);
    const segR = r;
    const halfT = thickness / 2 + 1;
    let svg = `<g class="histogram" opacity="0.85">`;
    for (let i = 0; i < n; i++) {
        const start = -Math.PI / 2 + i * angleEach + gapRad / 2;
        const end   = -Math.PI / 2 + (i + 1) * angleEach - gapRad / 2;
        const x0 = cx + Math.cos(start) * segR;
        const y0 = cy + Math.sin(start) * segR;
        const x1 = cx + Math.cos(end) * segR;
        const y1 = cy + Math.sin(end) * segR;
        const large = angleEach - gapRad > Math.PI ? 1 : 0;
        const color = sentimentFor(ratings[i], max).color;
        svg += `<path d="M ${x0} ${y0} A ${segR} ${segR} 0 ${large} 1 ${x1} ${y1}"
                stroke="${color}" stroke-width="${halfT}" fill="none"
                stroke-linecap="round"/>`;
    }
    svg += `</g>`;
    return svg;
}

function renderDeltaChip(personal, crowd, size) {
    const d = Number(personal) - Number(crowd);
    const token = deltaTokenFor(personal, crowd);
    if (token === DELTA_TOKENS.absent) return "";
    const abs = Math.abs(d);
    const display = abs < DELTA_EPSILON
        ? "="
        : `${token.symbol}${abs.toFixed(1)}`;
    const cls = size === "nano" ? "delta nano" : "delta";
    return `<span class="${cls}"
        style="color:${token.color}; border-color:${token.color}"
        title="${token.label}">${display}</span>`;
}

function escapeHtml(s) {
    return String(s).replace(/[&<>"']/g, c => ({
        "&": "&amp;", "<": "&lt;", ">": "&gt;",
        '"': "&quot;", "'": "&#39;",
    }[c]));
}

const STYLES = `
    :host {
        display: inline-block;
        font-family: var(--qn-font, system-ui, sans-serif);
        cursor: pointer;
        user-select: none;
    }
    :host(:focus-visible) .aurora {
        outline: 2px solid #1E88E5;
        outline-offset: 3px;
        border-radius: 50%;
    }
    :host([disabled]) { cursor: default; opacity: 0.6; }
    .wrap {
        display: inline-flex;
        align-items: center;
        gap: 8px;
    }
    .wrap.standard, .wrap.large {
        flex-direction: column;
        gap: 4px;
    }
    .aurora { display: block; }
    .digit {
        font-weight: 700;
        fill: #0B1D3A;
        font-variant-numeric: tabular-nums;
        font-family: inherit;
    }
    .nano-value {
        font-size: 13px;
        font-weight: 600;
        color: #0B1D3A;
        font-variant-numeric: tabular-nums;
    }
    .text {
        display: flex;
        gap: 8px;
        align-items: center;
        font-size: 12px;
        color: #4A5568;
    }
    .meta {
        color: #4A5568;
    }
    .meta.observer {
        color: #7A8598;
    }
    .meta.no-basis {
        font-style: italic;
        color: #8B95A7;
    }
    .delta {
        display: inline-flex;
        align-items: center;
        padding: 1px 6px;
        border: 1px solid currentColor;
        border-radius: 10px;
        font-size: 11px;
        font-weight: 600;
        font-variant-numeric: tabular-nums;
        background: transparent;
    }
    .delta.nano {
        padding: 0 3px;
        font-size: 10px;
        border-width: 0;
    }
    @media (prefers-reduced-motion: no-preference) {
        .ring {
            transition: stroke-width 180ms ease, stroke 180ms ease;
        }
        .shape {
            transition: fill 180ms ease, stroke 180ms ease;
        }
    }
`;
