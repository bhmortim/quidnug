/**
 * Injected into the page's JS world. Exposes a thin RPC client at
 * `window.quidnug` that DApps can call. Every call round-trips
 * through the extension service worker, so private keys never leave
 * the extension.
 */

(function installQuidnugGlobal() {
    let nextId = 1;
    const pending = new Map();

    window.addEventListener("message", (ev) => {
        if (ev.source !== window) return;
        if (!ev.data || ev.data.source !== "quidnug-extension") return;
        const resolver = pending.get(ev.data.id);
        if (!resolver) return;
        pending.delete(ev.data.id);
        resolver(ev.data.reply);
    });

    function send(msg) {
        return new Promise((resolve) => {
            const id = nextId++;
            pending.set(id, resolve);
            window.postMessage(
                { source: "quidnug-page", id, msg }, "*");
        });
    }

    async function call(msg) {
        const reply = await send(msg);
        if (!reply?.ok) throw new Error(reply?.error || "extension error");
        return reply.data;
    }

    Object.defineProperty(window, "quidnug", {
        value: Object.freeze({
            listQuids:   ()                    => call({ type: "listQuids" }),
            sign:        (quidId, canonicalHex) =>
                call({ type: "signCanonical", quidId, canonicalHex }),
            getNodeInfo: ()                    => call({ type: "getNode" }),
            isUnlocked:  ()                    => call({ type: "isUnlocked" }),
        }),
        configurable: false,
        writable: false,
    });
})();
