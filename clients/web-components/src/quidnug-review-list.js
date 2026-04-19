import { getClient, getObserverQuid } from "./context.js";
import { Rater } from "./rater.js";

/**
 * <quidnug-review-list> — standalone list of reviews with per-review
 * trust-weights shown. Useful on review-aggregation pages or as a
 * second element next to <quidnug-review>.
 *
 * Attributes:
 *   product — required.
 *   topic   — required.
 *   limit   — max reviews to display (default 20).
 *   sort    — "weight" (default) | "recent" | "rating-high" | "rating-low"
 */
export class QuidnugReviewListElement extends HTMLElement {
    static get observedAttributes() { return ["product", "topic", "limit", "sort"]; }

    constructor() {
        super();
        this.attachShadow({ mode: "open" });
        this._result = null;
        this._reviews = [];
    }

    connectedCallback() {
        this._render();
        this._fetch();
    }

    attributeChangedCallback() {
        if (this.isConnected) {
            this._render();
            this._fetch();
        }
    }

    async _fetch() {
        const product = this.getAttribute("product");
        const topic = this.getAttribute("topic");
        if (!product || !topic) return;

        const client = getClient();
        if (!client) return;
        const observer = getObserverQuid()?.id ?? "anonymous";
        try {
            const rater = new Rater(client);
            this._result = await rater.effectiveRating(observer, product, topic);

            const streamRes = await client.getStreamEvents(product, {
                domain: topic,
                limit: Number(this.getAttribute("limit") ?? 20),
            });
            this._reviews = (streamRes.events ?? []).filter(e => e.eventType === "REVIEW");
            this._render();
        } catch { /* ignore */ }
    }

    _sorted() {
        const sort = this.getAttribute("sort") ?? "weight";
        const result = this._result;
        const weightMap = new Map(
            (result?.contributions ?? []).map(c => [c.reviewerQuid, c.weight])
        );
        const copy = [...this._reviews];
        switch (sort) {
            case "recent":
                copy.sort((a, b) => (b.timestamp ?? 0) - (a.timestamp ?? 0));
                break;
            case "rating-high":
                copy.sort((a, b) =>
                    (b.payload?.rating ?? 0) - (a.payload?.rating ?? 0));
                break;
            case "rating-low":
                copy.sort((a, b) =>
                    (a.payload?.rating ?? 0) - (b.payload?.rating ?? 0));
                break;
            case "weight":
            default:
                copy.sort((a, b) =>
                    (weightMap.get(b.creator) ?? 0) - (weightMap.get(a.creator) ?? 0));
        }
        return { sorted: copy, weightMap };
    }

    _render() {
        const { sorted, weightMap } = this._sorted();
        const rows = sorted.map((ev) => this._row(ev, weightMap.get(ev.creator))).join("");
        this.shadowRoot.innerHTML = `
            <style>${STYLES}</style>
            <div class="list">
                <div class="list-header">
                    <span>${this._reviews.length} reviews · sorted by ${this.getAttribute("sort") ?? "your-trust-weight"}</span>
                </div>
                ${rows || "<em class='empty'>no reviews</em>"}
            </div>
        `;
    }

    _row(ev, weight) {
        const p = ev.payload ?? {};
        const rating = p.rating ?? "?";
        const body = String(p.bodyMarkdown ?? "").slice(0, 280);
        const weightStr = weight != null ? weight.toFixed(2) : "—";
        const weightClass = weight != null && weight > 0.1 ? "ok" : "low";
        return `
            <div class="row">
                <div class="row-rating">★ ${rating}</div>
                <div class="row-body">
                    ${p.title ? `<strong>${escape(p.title)}</strong><br>` : ""}
                    ${escape(body)}
                </div>
                <div class="row-weight ${weightClass}" title="Your trust-weighted contribution">${weightStr}</div>
            </div>
        `;
    }
}

function escape(s) {
    return String(s ?? "").replace(/[&<>"']/g, (c) => ({
        "&": "&amp;", "<": "&lt;", ">": "&gt;", '"': "&quot;", "'": "&#39;",
    }[c]));
}

const STYLES = `
    :host {
        display: block;
        font-family: var(--quidnug-font, system-ui, sans-serif);
    }
    .list-header {
        font-size: 13px;
        color: #666;
        margin-bottom: 8px;
    }
    .row {
        display: grid;
        grid-template-columns: 60px 1fr 60px;
        gap: 12px;
        align-items: flex-start;
        padding: 10px;
        border-bottom: 1px solid #eee;
        font-size: 14px;
    }
    .row-rating {
        color: #F9A825;
        font-weight: 600;
    }
    .row-body strong {
        color: #0B1D3A;
    }
    .row-weight {
        font-family: monospace;
        font-size: 12px;
        text-align: right;
    }
    .row-weight.ok  { color: #2E8B57; }
    .row-weight.low { color: #aaa; }
    .empty { color: #888; }
`;
