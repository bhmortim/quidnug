import { QuidnugStarsElement } from "./quidnug-stars.js";
import { getClient, getObserverQuid } from "./context.js";
import { Rater } from "./rater.js";

/**
 * <quidnug-review> — full review panel for a product.
 *
 * Attributes:
 *   product  — required. Canonical product asset id.
 *   topic    — required. Topic domain.
 *   show-write — if present, renders a <quidnug-write-review> inline.
 *
 * Events:
 *   `quidnug-review-rating` — when the aggregate rating is computed.
 *   `quidnug-review-submitted` — after a new review is successfully posted.
 */
export class QuidnugReviewElement extends HTMLElement {
    static get observedAttributes() {
        return ["product", "topic", "show-write"];
    }

    constructor() {
        super();
        this.attachShadow({ mode: "open" });
        this._reviews = [];
        this._result = null;
    }

    connectedCallback() {
        this._render();
        this._compute();
    }

    attributeChangedCallback() {
        if (this.isConnected) {
            this._render();
            this._compute();
        }
    }

    async _compute() {
        const product = this.getAttribute("product");
        const topic = this.getAttribute("topic");
        if (!product || !topic) return;

        const client = getClient();
        const observer = getObserverQuid()?.id ?? "anonymous";
        if (!client) {
            this._renderError("QuidnugClient not configured (call setClient).");
            return;
        }

        try {
            const rater = new Rater(client);
            this._result = await rater.effectiveRating(observer, product, topic);

            // Fetch the raw review list for display (the rater already
            // filters to weighted contributors; we display everything).
            const streamRes = await client.getStreamEvents(product, {
                domain: topic,
                limit: 100,
            });
            this._reviews = (streamRes.events ?? []).filter(
                (e) => e.eventType === "REVIEW"
            );

            this._render();
            this.dispatchEvent(new CustomEvent("quidnug-review-rating", {
                detail: this._result, bubbles: true, composed: true,
            }));
        } catch (err) {
            this._renderError(err.message);
        }
    }

    _render() {
        const product = this.getAttribute("product") ?? "";
        const topic = this.getAttribute("topic") ?? "";
        const showWrite = this.hasAttribute("show-write");
        const result = this._result;
        const rating = result?.rating;

        const heading = rating == null
            ? "<em>not enough trusted reviews for you yet</em>"
            : `<span class="big">${rating.toFixed(1)}</span>
               <span class="sub">out of 5 — from your trust network</span>`;

        const contribMap = new Map();
        if (result?.contributions) {
            for (const c of result.contributions) {
                contribMap.set(c.reviewerQuid, c);
            }
        }

        const reviewsHtml = this._reviews
            .sort((a, b) => (contribMap.get(b.creator)?.weight ?? 0) - (contribMap.get(a.creator)?.weight ?? 0))
            .map((ev) => this._renderReview(ev, contribMap.get(ev.creator)))
            .join("");

        this.shadowRoot.innerHTML = `
            <style>${STYLES}</style>
            <div class="panel">
                <div class="header">
                    <h3>Trust-weighted rating</h3>
                    ${heading}
                    <div class="meta">
                        ${result ? `${result.contributingReviews} of ${result.totalReviewsConsidered} reviews counted from your graph` : ""}
                    </div>
                </div>
                <div class="reviews">${reviewsHtml || "<em>no reviews yet</em>"}</div>
                ${showWrite ? `<quidnug-write-review product="${product}" topic="${topic}"></quidnug-write-review>` : ""}
            </div>
        `;
    }

    _renderReview(ev, contribution) {
        const payload = ev.payload ?? {};
        const reviewer = ev.creator ?? ev.signerQuid ?? "anon";
        const rating = payload.rating ?? "?";
        const title = payload.title ? `<strong>${escape(payload.title)}</strong>` : "";
        const body = escape(payload.bodyMarkdown ?? "");
        const weight = contribution
            ? `<span class="weight">weight ${contribution.weight.toFixed(2)}</span>`
            : `<span class="weight zero">outside your trust</span>`;

        return `
            <div class="review ${contribution ? "contributing" : "outside"}">
                <div class="review-header">
                    <span class="rating">★ ${rating}</span>
                    ${title}
                    ${weight}
                </div>
                <div class="reviewer">by ${reviewer.slice(0, 10)}…</div>
                <div class="body">${body}</div>
            </div>
        `;
    }

    _renderError(msg) {
        this.shadowRoot.innerHTML = `
            <style>${STYLES}</style>
            <div class="panel error">
                <p>Reviews unavailable: ${escape(msg)}</p>
            </div>
        `;
    }
}

function escape(s) {
    return String(s).replace(/[&<>"']/g, (c) => ({
        "&": "&amp;", "<": "&lt;", ">": "&gt;", '"': "&quot;", "'": "&#39;",
    }[c]));
}

const STYLES = `
    :host {
        display: block;
        font-family: var(--quidnug-font, system-ui, sans-serif);
        color: #222;
    }
    .panel {
        border: 1px solid #e0e0e0;
        border-radius: 8px;
        padding: 16px;
        background: #fff;
    }
    .header h3 {
        margin: 0 0 8px 0;
        font-size: 14px;
        color: #666;
        font-weight: 500;
    }
    .big {
        font-size: 36px;
        font-weight: 700;
        color: #0B1D3A;
    }
    .sub {
        font-size: 14px;
        color: #666;
        margin-left: 8px;
    }
    .meta {
        font-size: 12px;
        color: #888;
        margin-top: 4px;
    }
    .reviews {
        margin-top: 16px;
        display: flex;
        flex-direction: column;
        gap: 12px;
    }
    .review {
        padding: 10px 12px;
        border-radius: 6px;
        background: #f7f7f7;
        border-left: 3px solid transparent;
    }
    .review.contributing {
        border-left-color: #2E8B57;
    }
    .review.outside {
        opacity: 0.55;
        border-left-color: #ccc;
    }
    .review-header {
        display: flex;
        gap: 8px;
        align-items: center;
        font-size: 14px;
    }
    .rating {
        color: #F9A825;
        font-weight: 600;
    }
    .weight {
        font-size: 11px;
        color: #2E8B57;
        font-family: monospace;
    }
    .weight.zero {
        color: #999;
    }
    .reviewer {
        font-family: monospace;
        font-size: 11px;
        color: #888;
        margin: 4px 0;
    }
    .body {
        font-size: 14px;
        line-height: 1.4;
    }
    .error p {
        color: #c0392b;
    }
`;
