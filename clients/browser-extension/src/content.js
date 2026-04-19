/**
 * Content script — injects window.quidnug into pages on whitelisted
 * origins, and proxies method calls to the background service worker.
 *
 * API surface (in-page):
 *
 *   await window.quidnug.listQuids()
 *      -> [{ alias, id, publicKeyHex }]
 *
 *   await window.quidnug.sign(quidId, canonicalBytesHex)
 *      -> hex-DER signature (user approves in popup)
 *
 *   await window.quidnug.getNodeInfo()
 *      -> { url, token }
 */

(function injectQuidnugBridge() {
    // The page-facing bridge runs in the page's own JS world (NOT the
    // isolated world) so DApps can call window.quidnug directly. It
    // talks to the content-script world via window.postMessage.
    const script = document.createElement("script");
    script.src = chrome.runtime.getURL("src/injected.js");
    script.onload = function () { this.remove(); };
    (document.head || document.documentElement).appendChild(script);
})();

// Receive requests from the page, forward to the service worker, send
// back the response.
window.addEventListener("message", (ev) => {
    if (ev.source !== window) return;
    if (!ev.data || ev.data.source !== "quidnug-page") return;

    const { id, msg } = ev.data;
    chrome.runtime.sendMessage(msg, (reply) => {
        window.postMessage(
            {
                source: "quidnug-extension",
                id,
                reply: reply ?? { ok: false, error: "no reply" },
            },
            "*"
        );
    });
});
