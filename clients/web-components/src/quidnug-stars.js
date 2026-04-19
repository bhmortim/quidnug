import { Rater } from "./rater.js";
import { getClient, getObserverQuid } from "./context.js";

/**
 * <quidnug-stars> — tiny element that displays the per-observer
 * trust-weighted rating as stars. Framework-agnostic.
 *
 * Attributes:
 *   product   — product asset id (required)
 *   topic     — domain, e.g. "reviews.public.technology.laptops" (required)
 *   max       — display max rating, default 5
 *   show-count — if present, shows "(N trusted reviews)" after stars
 *
 * Events:
 *   `quidnug-stars-ready` — fires after computation; detail = result
 */
export class QuidnugStarsElement extends HTMLElement {
    static get observedAttributes() {
        return ["product", "topic", "max", "show-count"];
    }

    constructor() {
        super();
        this.attachShadow({ mode: "open" });
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
        if (!client) {
            this._renderError("No Quidnug client configured. Call setClient() first.");
            return;
        }
        const observer = getObserverQuid()?.id;
        const rater = new Rater(client);

        try {
            this._result = await rater.effectiveRating(observer ?? "anonymous", product, topic);
            this._render();
            this.dispatchEvent(new CustomEvent("quidnug-stars-ready", {
                detail: this._result, bubbles: true, composed: true,
            }));
        } catch (err) {
            this._renderError(err.message);
        }
    }

    _render() {
        const result = this._result;
        const max = Number(this.getAttribute("max") ?? 5);
        const showCount = this.hasAttribute("show-count");
        const rating = result?.rating ?? null;

        const stars = renderStars(rating ?? 0, max);
        const label = rating == null
            ? '<span class="no-data">not enough trusted reviews</span>'
            : `<span class="value">${rating.toFixed(1)}</span>`;
        const count = showCount && result
            ? `<span class="count"> (${result.contributingReviews} trusted)</span>`
            : "";
        const confidence = result && rating != null
            ? ` <span class="confidence">±${result.confidenceRange.toFixed(2)}</span>`
            : "";

        this.shadowRoot.innerHTML = `
            <style>${STYLES}</style>
            <div class="wrap" title="${this._tooltip()}">
                <span class="stars">${stars}</span>
                ${label}${confidence}${count}
            </div>
        `;
    }

    _renderError(msg) {
        this.shadowRoot.innerHTML = `
            <style>${STYLES}</style>
            <div class="wrap error" title="${msg}">
                <span class="no-data">trust data unavailable</span>
            </div>
        `;
    }

    _tooltip() {
        if (!this._result) return "";
        const { contributingReviews, totalReviewsConsidered } = this._result;
        return `${contributingReviews} of ${totalReviewsConsidered} reviews contributed to your weighted rating`;
    }
}

function renderStars(rating, max) {
    // Quarter-star resolution via filled/half/empty svg icons.
    let html = "";
    const r = Math.max(0, Math.min(max, rating));
    for (let i = 1; i <= max; i++) {
        if (r >= i) html += STAR_FILLED;
        else if (r >= i - 0.5) html += STAR_HALF;
        else html += STAR_EMPTY;
    }
    return html;
}

const STAR_FILLED = `<svg class="star" viewBox="0 0 24 24"><path d="M12 2l3 7 7 .6-5.3 4.7 1.7 7.2L12 17.8 5.6 21.5l1.7-7.2L2 9.6 9 9z"/></svg>`;
const STAR_HALF   = `<svg class="star half" viewBox="0 0 24 24"><defs><linearGradient id="half"><stop offset="50%" stop-color="#F9A825"/><stop offset="50%" stop-color="#e0e0e0"/></linearGradient></defs><path fill="url(#half)" d="M12 2l3 7 7 .6-5.3 4.7 1.7 7.2L12 17.8 5.6 21.5l1.7-7.2L2 9.6 9 9z"/></svg>`;
const STAR_EMPTY  = `<svg class="star empty" viewBox="0 0 24 24"><path d="M12 2l3 7 7 .6-5.3 4.7 1.7 7.2L12 17.8 5.6 21.5l1.7-7.2L2 9.6 9 9z"/></svg>`;

const STYLES = `
    :host {
        display: inline-block;
        font-family: var(--quidnug-font, system-ui, sans-serif);
    }
    .wrap {
        display: inline-flex;
        gap: 6px;
        align-items: center;
        font-size: 14px;
    }
    .stars {
        display: inline-flex;
        gap: 2px;
    }
    .star {
        width: 18px; height: 18px;
        fill: #F9A825;
    }
    .star.empty {
        fill: #e0e0e0;
    }
    .value {
        font-weight: 600;
        color: #0B1D3A;
    }
    .confidence {
        color: #666;
        font-size: 12px;
    }
    .count {
        color: #666;
        font-size: 12px;
    }
    .no-data {
        color: #888;
        font-style: italic;
    }
    .error {
        color: #c0392b;
    }
`;
