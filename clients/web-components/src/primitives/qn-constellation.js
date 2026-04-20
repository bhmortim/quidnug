/**
 * <qn-constellation> — the drilldown/expanded view.
 *
 * Bullseye of concentric tiers with one dot per contributing
 * reviewer. Tier membership encodes trust-hop distance from the
 * observer. Dot color encodes their raw rating, size encodes
 * weight contribution, outline encodes direct vs. transitive.
 *
 * Attributes:
 *
 *   contributors  JSON-encoded array of contributor objects:
 *                 [{ id, name?, rating, weight, hops, direct? }]
 *                 - `rating` : number 0..max
 *                 - `weight` : positive number (relative; sized by ratio)
 *                 - `hops`   : integer trust-path length (0 = you, 1 = direct,
 *                             2 = 2-hop, etc., 99 = no path / crowd)
 *                 - `direct` : optional bool — overrides hops-based outline
 *                 - `name`   : optional label shown on hover
 *   max           Rating scale maximum. Default 5.
 *   size          "compact" | "standard" | "large". Default "standard".
 *   title-text    Optional heading rendered above the chart.
 *   observer-name Optional label for the "you" dot.
 *
 * Fires: `qn-constellation-select` when a contributor is clicked.
 * Detail contains the contributor object.
 */

import {
    sentimentFor, CONSTELLATION_TIERS, tierFor,
} from "../design-tokens.js";

const SIZE_GEOMETRY = {
    compact:  { outer: 180, padding: 8,  labelFont: 10, dotMax: 10 },
    standard: { outer: 320, padding: 16, labelFont: 11, dotMax: 16 },
    large:    { outer: 480, padding: 24, labelFont: 13, dotMax: 22 },
};

const BaseElement = typeof HTMLElement !== "undefined"
    ? HTMLElement
    : class { constructor() {} };

export class QnConstellationElement extends BaseElement {
    static get observedAttributes() {
        return ["contributors", "max", "size", "title-text", "observer-name"];
    }

    constructor() {
        super();
        this.attachShadow({ mode: "open" });
    }

    connectedCallback() { this._render(); }
    attributeChangedCallback() { if (this.isConnected) this._render(); }

    _readState() {
        let contributors = [];
        const raw = this.getAttribute("contributors");
        if (raw) {
            try {
                const parsed = JSON.parse(raw);
                if (Array.isArray(parsed)) contributors = parsed;
            } catch { /* ignore */ }
        }
        const max = Number(this.getAttribute("max") ?? 5);
        const sizeName = this.getAttribute("size") ?? "standard";
        const geometry = SIZE_GEOMETRY[sizeName] ?? SIZE_GEOMETRY.standard;
        return {
            contributors,
            max,
            sizeName,
            geometry,
            titleText: this.getAttribute("title-text"),
            observerName: this.getAttribute("observer-name"),
        };
    }

    _render() {
        const state = this._readState();
        this.shadowRoot.innerHTML =
            `<style>${STYLES}</style>${renderConstellationSVG(state)}`;
        this._wireClicks(state);
    }

    _wireClicks(state) {
        const svg = this.shadowRoot.querySelector("svg");
        if (!svg) return;
        svg.addEventListener("click", (e) => {
            const dot = e.target.closest("[data-contrib-id]");
            if (!dot) return;
            const id = dot.getAttribute("data-contrib-id");
            const match = state.contributors.find(c => c.id === id);
            if (!match) return;
            this.dispatchEvent(new CustomEvent("qn-constellation-select", {
                detail: match, bubbles: true, composed: true,
            }));
        });
    }
}

/** Pure renderer — used directly by SSR adapters. */
export function renderConstellationSVG(state) {
    const { contributors, max, geometry, titleText, observerName } = state;
    const { outer, padding, labelFont, dotMax } = geometry;
    const cx = outer / 2;
    const cy = outer / 2;
    const maxR = (outer - padding * 2) / 2;

    // Group contributors by tier hop and lay them out radially
    // at the tier's radiusRatio, spreading evenly by angle with
    // a stable offset keyed by contributor id (so re-renders
    // don't jitter).
    const byTier = new Map();
    for (const c of contributors) {
        const tier = tierFor(c.hops ?? 99);
        if (!byTier.has(tier.hop)) byTier.set(tier.hop, []);
        byTier.get(tier.hop).push(c);
    }
    for (const list of byTier.values()) {
        list.sort((a, b) => (a.id || "").localeCompare(b.id || ""));
    }

    // Weight normalization (for dot size).
    const maxWeight = contributors.reduce(
        (m, c) => Math.max(m, Number(c.weight) || 0), 0);
    const weightScale = maxWeight > 0 ? (w) => Math.max(3, Math.sqrt(w / maxWeight) * dotMax) : () => 4;

    // Build tier rings from outside in (largest drawn first).
    const tierRings = [...CONSTELLATION_TIERS].sort(
        (a, b) => b.radiusRatio - a.radiusRatio);
    let ringsSvg = "";
    for (const t of tierRings) {
        const r = maxR * t.radiusRatio;
        ringsSvg += `<circle class="tier-ring" cx="${cx}" cy="${cy}" r="${r}"
            fill="${t.fill}" fill-opacity="0.08"
            stroke="${t.fill}" stroke-opacity="0.5"
            stroke-width="1" stroke-dasharray="2 3"/>`;
    }

    // "You" dot at center.
    const youTier = CONSTELLATION_TIERS[0];
    const youDot = `<g class="you">
        <circle cx="${cx}" cy="${cy}" r="${Math.max(4, dotMax * 0.4)}"
                fill="${youTier.fill}" stroke="#fff" stroke-width="2"/>
        ${observerName
            ? `<text x="${cx}" y="${cy + dotMax * 0.9}" class="you-label"
                style="font-size:${labelFont}px" text-anchor="middle">
                ${escapeHtml(observerName)}
               </text>`
            : ""}
    </g>`;

    // Contributor dots per tier.
    let dotsSvg = "";
    for (const [hop, list] of byTier.entries()) {
        const tier = tierFor(hop);
        const r = maxR * tier.radiusRatio;
        const n = list.length;
        for (let i = 0; i < n; i++) {
            const c = list[i];
            const angle = (-Math.PI / 2) + (i / n) * Math.PI * 2 +
                          ((hop % 2 === 0) ? 0 : Math.PI / Math.max(n, 4));
            const x = cx + Math.cos(angle) * r;
            const y = cy + Math.sin(angle) * r;
            const sentiment = sentimentFor(c.rating, max);
            const dotR = weightScale(c.weight);
            const isDirect = c.direct ?? (hop === 1);
            const title = [
                c.name ?? c.id ?? "reviewer",
                `${Number(c.rating).toFixed(1)} out of ${max}`,
                `${hop === 99 ? "crowd tier" : hop === 0 ? "you" : `${hop} ${hop === 1 ? "hop" : "hops"}`}`,
                `weight ${Number(c.weight).toFixed(3)}`,
            ].join(" · ");

            dotsSvg += `<g class="contrib" data-contrib-id="${escapeAttr(c.id ?? String(i))}"
                tabindex="0" role="button">
                <title>${title}</title>
                <circle cx="${x}" cy="${y}" r="${dotR}"
                    fill="${sentiment.color}"
                    stroke="${isDirect ? "#0B1D3A" : sentiment.color}"
                    stroke-width="${isDirect ? 2 : 1}"
                    stroke-dasharray="${isDirect ? "0" : "3 2"}"/>
            </g>`;
        }
    }

    // Legend — always rendered on the right in standard/large.
    const legendEntries = CONSTELLATION_TIERS.map(t => `
        <div class="legend-row">
            <span class="legend-swatch" style="background:${t.fill}"></span>
            ${t.label}
        </div>
    `).join("");

    const chart = `
        <svg class="constellation" viewBox="0 0 ${outer} ${outer}"
             width="${outer}" height="${outer}"
             xmlns="http://www.w3.org/2000/svg">
            ${titleText
                ? `<title>${escapeHtml(titleText)}</title>`
                : ""}
            ${ringsSvg}
            ${dotsSvg}
            ${youDot}
        </svg>
    `;

    return `
        <div class="wrap ${state.sizeName}">
            ${titleText
                ? `<h3 class="heading">${escapeHtml(titleText)}</h3>` : ""}
            <div class="body">
                ${chart}
                <div class="legend" aria-label="trust tiers">
                    ${legendEntries}
                    <div class="legend-divider"></div>
                    <div class="legend-row">
                        <span class="legend-outline direct"></span> direct trust
                    </div>
                    <div class="legend-row">
                        <span class="legend-outline indirect"></span> transitive
                    </div>
                </div>
            </div>
        </div>
    `;
}

function escapeHtml(s) {
    return String(s).replace(/[&<>"']/g, c => ({
        "&": "&amp;", "<": "&lt;", ">": "&gt;",
        '"': "&quot;", "'": "&#39;",
    }[c]));
}
function escapeAttr(s) { return escapeHtml(s); }

const STYLES = `
    :host {
        display: block;
        font-family: var(--qn-font, system-ui, sans-serif);
        color: #0B1D3A;
    }
    .wrap {
        display: flex;
        flex-direction: column;
        gap: 8px;
    }
    .heading {
        font-size: 15px;
        margin: 0;
        font-weight: 600;
        color: #1A2740;
    }
    .body {
        display: flex;
        align-items: flex-start;
        gap: 16px;
        flex-wrap: wrap;
    }
    .constellation { display: block; max-width: 100%; height: auto; }
    .contrib {
        cursor: pointer;
    }
    .contrib:focus-visible circle {
        stroke: #1E88E5 !important;
        stroke-width: 3;
    }
    .contrib circle {
        transition: transform 140ms ease, r 140ms ease;
        transform-box: fill-box;
        transform-origin: center;
    }
    .contrib:hover circle {
        transform: scale(1.15);
    }
    .you-label {
        fill: #1A2740;
        font-weight: 600;
        font-family: inherit;
    }
    .legend {
        display: flex;
        flex-direction: column;
        gap: 4px;
        font-size: 12px;
        color: #4A5568;
        min-width: 110px;
    }
    .legend-row {
        display: flex;
        align-items: center;
        gap: 6px;
    }
    .legend-divider {
        height: 1px;
        background: #E2E6EE;
        margin: 4px 0;
    }
    .legend-swatch {
        display: inline-block;
        width: 12px;
        height: 12px;
        border-radius: 50%;
    }
    .legend-outline {
        display: inline-block;
        width: 12px;
        height: 12px;
        border-radius: 50%;
        background: transparent;
    }
    .legend-outline.direct {
        border: 2px solid #0B1D3A;
    }
    .legend-outline.indirect {
        border: 1.5px dashed #8B95A7;
    }
    @media (prefers-reduced-motion: reduce) {
        .contrib circle { transition: none; }
        .contrib:hover circle { transform: none; }
    }
`;
