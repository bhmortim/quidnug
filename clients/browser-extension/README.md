# Quidnug browser extension (scaffold)

Status: **SCAFFOLD — not yet on Chrome Web Store / Firefox Add-ons.**

A MV3 (Manifest v3) browser extension that:

- Owns one or more Quidnug quids per user profile, with private keys
  encrypted by the user's WebAuthn authenticator.
- Injects a `window.quidnug` API into pages on whitelisted origins.
- Approves transaction signatures through a user-visible popup —
  similar to MetaMask, but for relational-trust transactions rather
  than EVM.
- Exposes per-quid relational-trust overlays on arbitrary web pages
  ("this author has trust 0.42 from your procurement-quid").

## Layout

```
clients/browser-extension/
├── manifest.json          # MV3 manifest (planned)
├── src/
│   ├── background.ts      # service worker: key store, HTTP bridge
│   ├── content.ts         # page injection
│   ├── popup.tsx          # transaction-approval UI
│   ├── options.tsx        # quid management + domain whitelist
│   └── lib/               # wraps @quidnug/client + v2
└── README.md              # this file
```

## Roadmap

1. MV3 skeleton + messaging bus between background ↔ content ↔ popup.
2. Key storage: IndexedDB + AES-GCM with a passphrase-derived key, or
   WebAuthn-wrapped via the CredentialManager.
3. Popup flow for IDENTITY / TRUST / EVENT transactions.
4. Page overlay: DOM-scanned quid IDs get a small badge showing
   relational trust from the active observer.
5. Ship to Chrome Web Store + Firefox Add-ons.

## License

Apache-2.0.
