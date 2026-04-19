// Package oidc implements the Quidnug OIDC bridge — a thin adapter
// that lets off-the-shelf OIDC identity providers (Okta, Auth0,
// Azure Entra ID, Google Workspace, Keycloak, Cognito, …) authenticate
// users into the Quidnug trust graph.
//
// # The Problem
//
// Most enterprises already have a hardened identity provider. They
// want to add Quidnug for per-observer trust, audit, and title-based
// authorization, without asking users to juggle a second keypair in
// their browser. The bridge provides exactly one flow:
//
//  1. App redirects the user to the configured OIDC provider.
//  2. Bridge receives the standard authorization-code callback.
//  3. Bridge validates the ID token (issuer, audience, signature,
//     exp, nonce).
//  4. Bridge either (a) looks up the quid bound to the subject claim
//     or (b) creates a fresh quid and records the binding.
//  5. Bridge mints a short-lived Quidnug session token (JWT) scoped
//     to the specific quid. Downstream applications call the Quidnug
//     node via the bridge's session middleware, which translates each
//     request into a bridge-signed Quidnug tx if the OIDC session is
//     still valid.
//
// # Security Model
//
//   - The OIDC subject claim (sub) is bound 1:1 to a Quidnug quid via
//     a Binding record. The binding is immutable after creation —
//     a new IdP login for the same sub always resolves to the same
//     quid.
//   - The bridge holds a *proxy* quid for each bound user. Private
//     keys never leave the bridge host (typically sealed in an HSM
//     via pkg/signer/hsm). Users never see or possess their quid key.
//   - The bridge logs every signed transaction with the OIDC sub so
//     it is auditable back to the original identity provider login.
//   - Short (≤ 1h) session tokens with PKCE-style challenge binding
//     limit replay if the token leaks.
//
// # Status
//
// This package is a scaffold: it defines the wire shapes and
// lifecycle, and provides a simple in-memory binding store + HTTP
// handlers. Production deployments should:
//
//   - Replace the in-memory BindingStore with a database-backed
//     implementation (Postgres preferred).
//   - Use an external OIDC verification library (coreos/go-oidc) for
//     the discovery + JWKS + ID-token validation step. This package
//     validates structure but trusts the caller to verify signatures
//     using the standard coreos/go-oidc Verifier.
//   - Run the HSM signer behind a short-lived session key cache so
//     per-request HSM round-trips don't dominate latency.
package oidc
