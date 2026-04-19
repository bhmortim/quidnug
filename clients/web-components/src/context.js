/**
 * Per-page Quidnug context.
 *
 * Host pages call setClient() / setObserverQuid() once at startup;
 * all components read from this module.
 *
 * Pages that use the browser extension (clients/browser-extension/)
 * can import the extension's `window.quidnug` and wire it in here.
 */

let _client = null;
let _observer = null;

/**
 * Provide the QuidnugClient instance. Typically called once at app
 * startup.
 */
export function setClient(client) {
    _client = client;
}

export function getClient() {
    return _client;
}

/**
 * Provide the current user's Quid (or null if anonymous).
 * Components fall back to "anonymous" observer if null.
 */
export function setObserverQuid(quid) {
    _observer = quid;
}

export function getObserverQuid() {
    return _observer;
}
