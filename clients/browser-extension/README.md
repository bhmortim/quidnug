# Quidnug Browser Extension

A Manifest V3 browser extension that:

- Holds the user's Quidnug quids in an AES-GCM-encrypted vault backed
  by `chrome.storage.local`, unlocked with a passphrase.
- Injects `window.quidnug` into pages so DApps can request signatures
  without ever seeing private keys.
- Proxies signing requests through the service worker, giving you a
  single chokepoint to add user-approval prompts.

Status: **Working MV3 skeleton — sideloadable today.** Publication to
Chrome Web Store / Firefox Add-ons / Edge pending branding review.

## Install (dev / sideload)

```bash
cd clients/browser-extension
# Pack for Chromium:  chrome → chrome://extensions → Load unpacked
# Pack for Firefox:   about:debugging → Load Temporary Add-on
```

The extension ships no build pipeline — every file in the tree is
what gets loaded. The service worker and scripts are ES modules.

## Architecture

```
┌────────────────────────────────────────────────────────────┐
│ Page JS world                                              │
│   └── window.quidnug.{listQuids, sign, getNodeInfo, ...}   │
│        (injected.js — frozen RPC object)                   │
└────────────────────────────────────────────────────────────┘
        ▲  window.postMessage
        ▼
┌────────────────────────────────────────────────────────────┐
│ Content script (isolated world)                            │
│   └── content.js — bridges page ↔ service worker           │
└────────────────────────────────────────────────────────────┘
        ▲  chrome.runtime.sendMessage
        ▼
┌────────────────────────────────────────────────────────────┐
│ Service worker (background.js)                             │
│   ├── AES-GCM vault in chrome.storage.local                │
│   ├── In-memory unlocked cache (5-min idle lock)           │
│   ├── ECDSA P-256 signing (WebCrypto subtle)               │
│   └── Message dispatcher                                   │
└────────────────────────────────────────────────────────────┘
        ▲  popup / options
        ▼
    popup.html · options.html
```

## Public API (in-page)

DApps call `window.quidnug` directly — no content-script handshake:

```js
// Does the user have the extension installed?
if (!window.quidnug) { throw new Error("Quidnug extension not installed"); }

const { unlocked } = await window.quidnug.isUnlocked();
if (!unlocked) { alert("Unlock Quidnug via the extension popup"); return; }

const quids = await window.quidnug.listQuids();
// -> [{ alias, id, publicKeyHex }]

// Sign canonical bytes produced by your app with the Quidnug SDK's
// CanonicalBytes helper. The extension returns a hex-DER signature
// identical to what any other Quidnug SDK produces.
const sig = await window.quidnug.sign(quids[0].id, canonicalBytesHex);

// Fetch the configured node URL + token (may be empty).
const { url, token } = await window.quidnug.getNodeInfo();
```

## Vault format

- `chrome.storage.local["quidnug:vault:enc"]` — AES-GCM ciphertext
- `chrome.storage.local["quidnug:vault:salt"]` — 16-byte salt
- PBKDF2-SHA256 with 310 000 iterations to derive the key

The vault plaintext is:

```json
{
  "quids": [
    { "alias": "personal", "id": "ab1234...", "publicKeyHex": "04...", "privateKeyHex": "30..." }
  ]
}
```

## Security

- **Never logs private key material.** Every signing operation happens
  inside the service worker; the page only ever receives the hex
  signature.
- **Idle-lock**: the in-memory unlock cache clears after 5 minutes of
  inactivity (configurable in `background.js`).
- **Origin controls (roadmap)**: wire a per-origin allow-list in
  `options.html` so only approved sites can call `window.quidnug.sign`.
- **Approval prompt (roadmap)**: each `signCanonical` currently
  auto-approves. Production hardening should raise a transient
  popup for explicit user consent before signing — the plumbing is
  already in place (see the `TODO` in `background.js`).

## Files

| File | Purpose |
| --- | --- |
| `manifest.json` | MV3 manifest. |
| `src/background.js` | Service worker (vault, signing, dispatcher). |
| `src/content.js` | Content-script bridge. |
| `src/injected.js` | Page-world `window.quidnug` object. |
| `public/popup.html`+`.js` | Lock/unlock + quid list. |
| `public/options.html`+`.js` | Node URL + token config. |
| `tests/message-api.test.js` | Structure validation tests. |

## Testing

```bash
npm install     # (no deps; just puts node_modules/ in place)
node --test tests/
```

For E2E browser tests, point Playwright at an unpacked build of the
extension — see [https://playwright.dev/docs/chrome-extensions](https://playwright.dev/docs/chrome-extensions).

## Roadmap

- Approval popup for `signCanonical` (+ transaction decoding).
- Origin allow-list in options page.
- WebAuthn-backed vault unlock (passkey → AES-GCM key derivation).
- Relational-trust DOM overlay: scan visible quid IDs on the page
  and annotate with the active user's computed trust score.
- Firefox / Edge packaging metadata.

## License

Apache-2.0.
