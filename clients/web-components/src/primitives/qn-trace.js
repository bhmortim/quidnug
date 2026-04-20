/**
 * <qn-trace> — horizontal stacked bar showing weight composition.
 *
 * One segment per contributing reviewer. Segment width is
 * proportional to that reviewer's weight contribution; segment
 * color encodes their raw rating; segment outline encodes direct
 * vs. transitive trust.
 *
 * Best used for side-by-side comparison of multiple items (a
 * product list where each product has a trace bar underneath
 * its name), because you can eyeball "mostly green wide solid
 * segments" versus "narrow dashed mixed segments" at a glance.
 *
 * Attributes:
 *
 *   contributors  JSON array: [{ id, name?, rating, weight, direct? }]
 *   max           Rating scale max. Default 5.
 *   height        Bar height in px. Default 10.
 *   show-labels   If present, render contributor names beneath
 *                 the bar (only useful at the larger sizes).
 *   empty-text    Text shown when contributors is empty.
 *                 Default "no trusted reviews yet".
 */

import { sentimentFor } from "../design-tokens.js";

const BaseElement = typeof HTMLElement !== "undefined"
    ? HTMLElement
    : class { constructor() {} };

export class QnTraceElement extends BaseElement {
    static get observedAttributes() {
        return ["contributors", "max", "height", "show-labels", "empty-text"];
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
                const p = JSON.parse(raw);
                if (Array.isArray(p)) contributors = p;
            } catch { /* ignore */ }
        }
        return {
            contributors,
            max: Number(this.getAttribute("max") ?? 5),
            height: Number(this.getAttribute("height") ?? 10),
            showLabels: this.hasAttribute("show-labels"),
            emptyText: this.getAttribute("empty-text") ?? "no trusted reviews yet",
        };
    }

    _render() {
        const state = this._readState();
        this.shadowRoot.innerHTML =
            `<style>${STYLES}</style>${renderTraceSVG(state)}`;
    }
}

/** Pure renderer. */
export function renderTraceSVG(state) {
    const { contributors, max, height, showLabels, emptyText } = state;

    if (!contributors || contributors.length === 0) {
        return `<div class="empty">${escapeHtml(emptyText)}</div>`;
    }

    const total = contributors.reduce(
        (s, c) => s + (Number(c.weight) || 0), 0);
    if (total <= 0) {
        return `<div class="empty">${escapeHtml(emptyText)}</div>`;
    }

    // Minimum segment width so thin-weight contributors still
    // register visually. We trade strict proportionality for
    // readability — the real number lives in the tooltip.
    const W = 100; // percent space; scale to container via CSS
    const minPct = 2;
    let remaining = W;
    const raw = contributors.map(c => ({
        c,
        pct: ((Number(c.weight) || 0) / total) * W,
    }));
    // Clamp below-minimum segments up, then rescale large ones.
    const tooSmall = raw.filter(r => r.pct > 0 && r.pct < minPct);
    for (const t of tooSmall) t.pct = minPct;
    const oversize = W - raw.reduce((s, r) => s + r.pct, 0);
    if (oversize < 0) {
        const bigs = raw.filter(r => r.pct >= minPct * 1.5);
        const bigSum = bigs.reduce((s, r) => s + r.pct, 0);
        for (const r of bigs) {
            r.pct += (oversize * r.pct) / bigSum;
        }
    }

    let segments = "";
    let cursor = 0;
    for (const { c, pct } of raw) {
        const sentiment = sentimentFor(c.rating, max);
        const isDirect = c.direct ?? false;
        const title = [
            c.name ?? c.id ?? "reviewer",
            `${Number(c.rating).toFixed(1)}/${max}`,
            `weight ${(((Number(c.weight) || 0) / total) * 100).toFixed(1)}%`,
            isDirect ? "direct trust" : "transitive",
        ].join(" · ");
        segments += `<div class="seg ${isDirect ? "direct" : "indirect"}"
            role="listitem"
            style="
                left: ${cursor}%;
                width: ${pct}%;
                background: ${sentiment.color};
                height: ${height}px;
            "
            title="${escapeAttr(title)}"
            aria-label="${escapeAttr(title)}">
        </div>`;
        cursor += pct;
    }

    const labelsHtml = showLabels
        ? `<div class="labels">${contributors.map(c =>
            `<span class="label">${escapeHtml(c.name ?? c.id ?? "")}</span>`
          ).join(" · ")}</div>`
        : "";

    return `
        <div class="wrap">
            <div class="bar" role="list" style="height: ${height}px">${segments}</div>
            ${labelsHtml}
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
    }
    .wrap {
        display: flex;
        flex-direction: column;
        gap: 4px;
    }
    .bar {
        position: relative;
        width: 100%;
        background: #E2E6EE;
        border-radius: 6px;
        overflow: hidden;
    }
    .seg {
        position: absolute;
        top: 0;
        border-radius: 2px;
        transition: opacity 150ms ease;
    }
    .seg:hover { opacity: 0.85; }
    .seg.indirect {
        background-image: repeating-linear-gradient(
            -45deg,
            rgba(255,255,255,0.45) 0,
            rgba(255,255,255,0.45) 2px,
            transparent 2px,
            transparent 5px
        );
    }
    .labels {
        font-size: 11px;
        color: #4A5568;
        line-height: 1.4;
    }
    .empty {
        font-size: 12px;
        color: #7A8598;
        font-style: italic;
        padding: 6px 0;
    }
    @media (prefers-reduced-motion: reduce) {
        .seg { transition: none; }
    }
`;
