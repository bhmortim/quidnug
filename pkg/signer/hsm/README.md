# Quidnug PKCS#11 / HSM signer

`github.com/quidnug/quidnug/pkg/signer/hsm` wraps any PKCS#11-compliant
HSM or software token behind the `signer.Signer` interface used by the
Quidnug Go SDK.

Supports:

- SoftHSM (for dev / test)
- YubiHSM (via yubihsm-shell)
- Thales Luna
- AWS CloudHSM
- Azure Key Vault Managed HSM (via `libazurekeyvault-pkcs11`)
- Google Cloud HSM (via Cloud KMS PKCS#11 provider)
- Any other CKM_ECDSA-capable P-256 token

## Enabling

The HSM backend depends on CGo + a PKCS#11 shared library and is
guarded by a build tag:

```bash
go build -tags=pkcs11 ./...
```

Without the tag, `hsm.Open` returns an explanatory error so the rest
of the SDK compiles cleanly on systems without a PKCS#11 toolchain.

## Key material

The signer expects:

- A P-256 EC private key on the token (CKK_EC, CKA_EC_PARAMS =
  `prime256v1` / OID 1.2.840.10045.3.1.7).
- A matching public key object (most HSMs create both automatically
  via `C_GenerateKeyPair`).
- Matching `CKA_LABEL` or `CKA_ID` so the signer can locate them.

Generate a fresh keypair on SoftHSM:

```bash
softhsm2-util --init-token --free --label quidnug-dev --pin 1234 --so-pin 1234

# Using pkcs11-tool from OpenSC
pkcs11-tool --module /usr/lib/softhsm/libsofthsm2.so \
    --login --pin 1234 --keypairgen --key-type EC:prime256v1 \
    --label quidnug-node-01 --id 01
```

## Using with the Go SDK

```go
package main

import (
    "context"
    "log"
    "os"

    "github.com/quidnug/quidnug/pkg/client"
    "github.com/quidnug/quidnug/pkg/signer/hsm"
)

func main() {
    s, err := hsm.Open(hsm.Config{
        ModulePath: "/usr/lib/softhsm/libsofthsm2.so",
        TokenLabel: "quidnug-dev",
        KeyLabel:   "quidnug-node-01",
        PIN:        os.Getenv("HSM_PIN"),
    })
    if err != nil {
        log.Fatal(err)
    }
    defer s.Close()

    // Build a canonical tx, sign via HSM, submit via the HTTP client.
    tx := map[string]any{
        "type":        "TRUST",
        "truster":     s.QuidID(),
        "trustee":     "bob",
        "trustLevel":  0.9,
        "trustDomain": "contractors.home",
        "timestamp":   1700000000,
        "nonce":       1,
    }
    signable, _ := client.CanonicalBytes(tx, "signature")
    sig, err := s.Sign(signable)
    if err != nil {
        log.Fatal(err)
    }
    tx["signature"] = sig
    _ = tx // POST to /api/transactions/trust
    _ = context.Background()
}
```

## Thread-safety

`hsm.Open` returns a signer whose `Sign` is serialized by an internal
mutex — PKCS#11 sessions cannot be shared concurrently. If you need
parallel signing (e.g. a batch publisher), open one signer per
goroutine, or front them with a channel-of-signers.

## Security notes

- The PIN can be passed via `Config.PIN` or left blank when using an
  attestation / PKCS#11 provider that doesn't require user auth (e.g.
  some cloud HSMs).
- The SEC1 public key is extracted from the HSM at `Open()` time so
  the quid ID and wire-form public key are cached in process.
- `Sign` pre-hashes with SHA-256 and submits `CKM_ECDSA` (not
  `CKM_ECDSA_SHA256`) — this matches what the Go reference signs.
- Signatures are re-encoded to low-S DER to avoid issues with
  verifiers that enforce deterministic ECDSA.

## Known caveats

- Some HSMs return the `CKA_EC_POINT` as a bare point (not wrapped in
  ASN.1 OCTET STRING). The signer handles both.
- CGo build latency and dependencies mean this is opt-in. CI builds
  without the tag will skip HSM tests.

## License

Apache-2.0.
