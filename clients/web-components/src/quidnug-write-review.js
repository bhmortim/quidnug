import { getClient, getObserverQuid } from "./context.js";

/**
 * <quidnug-write-review> — inline review-writing form.
 *
 * Requires setObserverQuid() to have been called with a signing-
 * capable Quid (usually sourced from the browser extension or a
 * locally-generated dev quid).
 *
 * Attributes:
 *   product — required.
 *   topic   — required.
 *
 * Events:
 *   `quidnug-review-submitted` — after successful post. detail = tx receipt.
 */
export class QuidnugWriteReviewElement extends HTMLElement {
    static get observedAttributes() { return ["product", "topic"]; }

    constructor() {
        super();
        this.attachShadow({ mode: "open" });
        this._state = { rating: 5, title: "", body: "", submitting: false };
    }

    connectedCallback() {
        this._render();
    }

    _render() {
        const observer = getObserverQuid();
        const canWrite = !!observer && observer.has_private_key;
        const { rating, title, body, submitting } = this._state;

        if (!canWrite) {
            this.shadowRoot.innerHTML = `
                <style>${STYLES}</style>
                <div class="form">
                    <p class="signin-msg">Sign in with your Quidnug identity to write a review.</p>
                </div>
            `;
            return;
        }

        this.shadowRoot.innerHTML = `
            <style>${STYLES}</style>
            <div class="form">
                <h4>Write a review</h4>
                <div class="rating-row">
                    <label>Your rating:</label>
                    <input type="number" id="rating" min="0" max="5" step="0.5" value="${rating}" />
                </div>
                <input id="title" type="text" placeholder="Review title" value="${escape(title)}" />
                <textarea id="body" placeholder="Your experience..." rows="5">${escape(body)}</textarea>
                <button id="submit" ${submitting ? "disabled" : ""}>
                    ${submitting ? "Submitting…" : "Post review"}
                </button>
                <p class="small">
                    Signed with your Quidnug identity and published to the public
                    <code>reviews.public.*</code> domain tree. Your reputation carries
                    across sites that speak QRP-0001.
                </p>
            </div>
        `;
        this._wireEvents();
    }

    _wireEvents() {
        const root = this.shadowRoot;
        const ratingEl = root.getElementById("rating");
        const titleEl = root.getElementById("title");
        const bodyEl = root.getElementById("body");
        const submitBtn = root.getElementById("submit");

        if (!submitBtn) return;
        submitBtn.addEventListener("click", async () => {
            this._state.rating = Number(ratingEl.value);
            this._state.title = titleEl.value;
            this._state.body = bodyEl.value;
            await this._submit();
        });
    }

    async _submit() {
        const product = this.getAttribute("product");
        const topic = this.getAttribute("topic");
        const observer = getObserverQuid();
        const client = getClient();

        if (!product || !topic || !observer || !client) {
            this._state.submitting = false;
            this._render();
            return;
        }

        this._state.submitting = true;
        this._render();

        try {
            const receipt = await client.createEventTransaction(
                {
                    subjectId: product,
                    subjectType: "TITLE",
                    eventType: "REVIEW",
                    domain: topic,
                    payload: {
                        qrpVersion: 1,
                        rating: this._state.rating,
                        maxRating: 5.0,
                        title: this._state.title,
                        bodyMarkdown: this._state.body,
                        locale: navigator.language ?? "en",
                    },
                },
                observer
            );
            this.dispatchEvent(new CustomEvent("quidnug-review-submitted", {
                detail: receipt, bubbles: true, composed: true,
            }));
            this._state = { rating: 5, title: "", body: "", submitting: false };
        } catch (err) {
            this._state.submitting = false;
            this._state.error = err.message;
        }
        this._render();
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
    .form {
        margin-top: 16px;
        padding: 12px;
        border: 1px solid #ddd;
        border-radius: 8px;
        background: #fafafa;
    }
    h4 {
        margin: 0 0 8px;
        font-size: 14px;
    }
    .rating-row {
        display: flex;
        align-items: center;
        gap: 8px;
        margin-bottom: 8px;
    }
    .rating-row label {
        font-size: 13px;
    }
    input[type=number] {
        width: 60px;
        padding: 4px;
    }
    input[type=text], textarea {
        width: 100%;
        padding: 8px;
        margin-bottom: 8px;
        box-sizing: border-box;
        border: 1px solid #ccc;
        border-radius: 4px;
        font-family: inherit;
        font-size: 13px;
    }
    button {
        padding: 8px 16px;
        background: #2E8B57;
        color: #fff;
        border: none;
        border-radius: 4px;
        cursor: pointer;
    }
    button:disabled {
        opacity: 0.6;
        cursor: not-allowed;
    }
    .small {
        color: #777;
        font-size: 11px;
        margin-top: 8px;
    }
    .signin-msg {
        color: #666;
        font-style: italic;
    }
    code {
        background: #eee;
        padding: 1px 4px;
        border-radius: 3px;
    }
`;
