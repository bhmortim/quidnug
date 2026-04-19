/**
 * Loader script for @quidnug/reviews-widget — the
 * single-file embed.
 *
 * Detects capabilities and does one of:
 *   1. Native custom element <quidnug-review> if supported + reachable
 *   2. Iframe fallback that hosts the web-component in an isolated origin
 *
 * Deliberately zero dependencies and non-blocking. Safe to drop
 * on any page.
 */

(function () {
    "use strict";

    // If the browser supports custom elements and the @quidnug/web-components
    // bundle is reachable, use it directly.
    if ("customElements" in window) {
        loadCustomElements()
            .then(() => bootstrapCustomElements())
            .catch(() => upgradeToIframe());
    } else {
        upgradeToIframe();
    }

    function loadCustomElements() {
        const scripts = [
            "https://cdn.jsdelivr.net/npm/@quidnug/client@2/quidnug-client.js",
            "https://cdn.jsdelivr.net/npm/@quidnug/client@2/quidnug-client-v2.js",
            "https://cdn.jsdelivr.net/npm/@quidnug/web-components@2/src/index.js",
        ];
        return Promise.all(scripts.map(loadScript));
    }

    function loadScript(src) {
        return new Promise((resolve, reject) => {
            const s = document.createElement("script");
            s.type = "module";
            s.src = src;
            s.onload = resolve;
            s.onerror = () => reject(new Error("load failed: " + src));
            document.head.appendChild(s);
        });
    }

    async function bootstrapCustomElements() {
        const ctx = await import(
            "https://cdn.jsdelivr.net/npm/@quidnug/web-components@2/src/context.js"
        );
        const QC = window.QuidnugClient;
        if (!QC) return;

        // Discover node URL from data attribute on any widget tag
        let nodeUrl = "https://public.quidnug.dev";
        for (const el of document.querySelectorAll("quidnug-review, quidnug-stars")) {
            const u = el.getAttribute("node-url");
            if (u) { nodeUrl = u; break; }
        }

        const client = new QC({ defaultNode: nodeUrl });
        ctx.setClient(client);

        if (window.quidnug) {
            try {
                const quids = await window.quidnug.listQuids();
                if (quids && quids.length > 0) ctx.setObserverQuid(quids[0]);
            } catch { /* extension locked */ }
        }
    }

    function upgradeToIframe() {
        // Replace every custom element with an iframe fallback
        for (const el of document.querySelectorAll("quidnug-review, quidnug-stars")) {
            const product = el.getAttribute("product");
            const topic = el.getAttribute("topic");
            if (!product || !topic) continue;

            const compact = el.tagName.toLowerCase() === "quidnug-stars" ? "&compact=1" : "";
            const iframe = document.createElement("iframe");
            iframe.src = `https://widget.quidnug.dev/v2/?product=${encodeURIComponent(product)}&topic=${encodeURIComponent(topic)}${compact}`;
            iframe.style.cssText = "border:0;width:100%;height:600px";
            iframe.loading = "lazy";
            el.parentNode.replaceChild(iframe, el);
        }
    }
})();
