# Quidnug WebAuthn / FIDO2 signer bridge

`github.com/quidnug/quidnug/pkg/signer/webauthn` bridges WebAuthn
authenticators (Touch ID, Windows Hello, YubiKey, security keys,
passkeys) to Quidnug's transaction signing model.

Unlike a classic in-process signer, WebAuthn is a three-party flow:

1. The Quidnug app server issues a **challenge** = `sha256(canonical
   transaction bytes) || fresh random bytes`.
2. The browser invokes `navigator.credentials.get()` with that
   challenge and the user's pre-registered credential ID.
3. The authenticator returns an assertion (`authenticatorData ||
   SHA256(clientDataJSON)`) signed by the device-held private key.
4. The server validates origin, RP-ID, and that the challenge was
   issued, then forwards the assertion to the Quidnug node.

This package provides the server-side coordinator for steps 1, 3,
and 4.

## What's in the box

- `Server` — WebAuthn state coordinator with `BeginSigning` /
  `FinishSigning` / `Registration` methods.
- `Credential`, `Challenge`, `AssertionRequest/Response` — wire shapes
  matching the WebAuthn Level 2 spec.
- `MemoryCredentialStore` / `MemoryChallengeStore` — in-memory
  implementations for tests and small single-process deployments.
  Swap in PostgreSQL / Redis for production.

## Not in scope (yet)

- Full attestation verification (trust chain to TPM / YubiKey / Apple
  CA). Use [github.com/go-webauthn/webauthn](https://github.com/go-webauthn/webauthn)
  in tandem for production registration.
- Sign-counter rollback detection logic (interface is exposed via
  `UpdateSignCount`; the caller is expected to compare against the
  stored counter and reject the assertion on a rollback).

## Minimal flow

```go
srv, err := webauthn.New(webauthn.Config{
    RPID:           "quidnug.example.com",
    RPName:         "Quidnug",
    Origin:         "https://quidnug.example.com",
    Store:          webauthn.NewMemoryCredentialStore(),
    ChallengeStore: webauthn.NewMemoryChallengeStore(),
})

// Register: typically done through go-webauthn, then hand the
// credential ID + public key to us.
cred, _ := srv.Registration(
    "alice@company - MacBook touch ID",
    credentialIDFromRegistration,   // []byte
    publicKeyUncompressed,          // 65-byte SEC1
    signCounter,                    // uint32
)

// At signing time:
canonicalTx, _ := client.CanonicalBytes(quidnugTx, "signature")

req, _ := srv.BeginSigning(cred.QuidID, canonicalTx)
// Send req to browser -> navigator.credentials.get()

// Browser returns an AssertionResponse
cred, signableBytes, sig, err := srv.FinishSigning(resp)

// signableBytes + sig now go to whatever verifier the Quidnug node
// has been configured with for WebAuthn-signed transactions.
```

## Quidnug node integration

WebAuthn signs `authData || SHA256(clientDataJSON)` — **not** the raw
canonical bytes. This means a WebAuthn-signed Quidnug transaction
cannot be verified by the default P-256-over-canonical-bytes path.

There are two options for node integration:

1. **Envelope path (recommended)**: the Quidnug node accepts a
   WebAuthn-signed envelope (authData + clientDataJSON + DER sig) for
   transactions whose signer quid is flagged as WebAuthn-backed.
   The node's WebAuthn verifier checks origin/RP ID/challenge binding
   and then verifies the ECDSA signature over the WebAuthn envelope.

2. **Proxy path**: the app server verifies WebAuthn itself, then
   re-signs the canonical Quidnug bytes with an HSM-held proxy key
   associated with the user's quid. This makes the wire format
   identical to any other signed tx, at the cost of an online signer.

Most deployments will start with (2) and migrate to (1) once the node
adds native WebAuthn envelope support.

## Security notes

- `BeginSigning` binds the challenge to the canonical tx bytes. A
  replayed assertion signed over the same tx is still rejected
  because the challenge includes 16 bytes of fresh entropy.
- `ChallengeStore.Consume` is one-shot: a challenge is deleted on
  first read. Re-submitting the same assertion will fail.
- `Config.ChallengeTTL` defaults to 60 seconds. Tune to your risk
  model.
- Always serve over HTTPS and set `Origin` exactly matching your
  browser's `window.location.origin`.

## License

Apache-2.0.
