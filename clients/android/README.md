# Quidnug Android SDK (scaffold)

Status: **SCAFFOLD — not yet on Maven Central / Google's Android
repository.**

Thin Kotlin wrapper around the `clients/java/` SDK, adding:

- Android Keystore integration — P-256 private keys can live behind
  the TEE (StrongBox on Pixel 3+, Titan M / Titan M2), unextractable.
  Matches the signer.Signer interface pattern used by the HSM +
  WebAuthn server signers.
- Jetpack Compose helpers for quid generation UIs.
- WorkManager integration for background event emission with retry.

## Roadmap

1. Bring in `clients/java/` as the base.
2. Add `AndroidKeystoreSigner` implementing the SDK's signer
   abstraction against a StrongBox-backed P-256 key.
3. Add `CredentialManagerLink` for passkey / FIDO2 authentication
   bound to a Quidnug quid.
4. Ship as `com.quidnug:quidnug-android:2.0.0` on Maven Central with
   the Android build variant flag.

## License

Apache-2.0.
