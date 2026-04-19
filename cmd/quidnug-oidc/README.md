# quidnug-oidc — OIDC → Quidnug bridge service

Standalone HTTP service that binds OIDC identity-provider subjects to
Quidnug quids. Use this to let users of an existing IdP (Okta, Auth0,
Azure Entra ID, Keycloak, Google Workspace, Cognito, …) participate in
the Quidnug trust graph without managing a private key themselves.

## Flow

```
Browser  ──OIDC auth-code──►  IdP
  │                             │
  │  ◄───── id_token ──────────┘
  │
  ▼
App backend ──POST /resolve──► quidnug-oidc
                                    │
                                    ├─ first login? mint fresh quid
                                    │  (in-proc P-256; production: HSM)
                                    │
                                    ├─ store Binding { issuer, sub, quidId }
                                    │
                                    └─► return quidId
  │
  ▼
App backend ──sign-and-submit──► Quidnug node
  (the bridge signs on behalf of the user's bound quid)
```

## Endpoints

| Method + Path | Description |
| --- | --- |
| `POST /resolve` | Given `{issuer, subject, email, name}` (claims from a verified ID token), return the bound `quidId`. Creates a new quid on first login. |
| `GET /binding/{quidId}` | Fetch the binding record for audit / debugging. |
| `GET /healthz` | Liveness. |

## Running

```bash
go run ./cmd/quidnug-oidc --listen :8089 --domain contractors.home
```

```bash
curl -X POST http://localhost:8089/resolve -d '{
  "issuer":  "https://auth.example.com",
  "subject": "user-abc123",
  "email":   "alice@example.com",
  "name":    "Alice"
}'
# -> {"quidId":"a3b1…","bound":true,"issuer":"…","subject":"…"}
```

## Security — call these out in production

1. **ID-token verification is the caller's job**. This service trusts
   whatever claims are handed to `/resolve`. Pair it with
   [github.com/coreos/go-oidc](https://github.com/coreos/go-oidc) or an
   equivalent verifier on the app backend.

2. **In-process keys are a dev-only default**. Production deployments
   should replace `inProcFactory` with a factory that dispatches to
   `pkg/signer/hsm` so key material never lives outside the HSM.

3. **Binding store is in-memory in the scaffold**. Swap for PostgreSQL
   (use an `ON CONFLICT DO NOTHING` insert for idempotency). Replicas
   of this service must share a store — otherwise you'll mint N quids
   for the same user.

4. **Audit**: every signed transaction produced by the bridge should
   be logged with `(issuer, subject, quidId, tx_id)` so auditors can
   trace it back to an IdP login event.

## Wiring into the Go SDK

```go
import (
    "github.com/quidnug/quidnug/internal/oidc"
    "github.com/quidnug/quidnug/pkg/client"
)

bridge, _ := oidc.New(oidc.Options{
    Store:         oidc.NewMemoryBindingStore(),
    Signer:        myHSMFactory,
    DefaultDomain: "contractors.home",
})

binding, signer, err := bridge.Resolve(ctx, oidc.IDToken{
    Issuer:  tok.Issuer,
    Subject: tok.Subject,
    Email:   tok.Email,
    Name:    tok.Name,
})
if err != nil { /* ... */ }

// Register identity on the Quidnug node (idempotent).
_ = bridge.RegisterIdentityOnNode(ctx, client.NewOrWhatever, binding, signer)
```

## Not in scope (yet)

- PKCE / nonce flow for the direct browser-facing side (typically
  handled by the app, not this service).
- Session-token minting / cookie flow: that's a concern of the calling
  app, not the bridge.
- Revocation: today there's no "disconnect this IdP binding" path.
  Adding it is straightforward — delete the binding row, but key
  material lives on in the HSM and needs explicit destruction.

## License

Apache-2.0.
