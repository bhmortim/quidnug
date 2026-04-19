# Quidnug vs. OAuth 2.0 / OpenID Connect

OAuth 2.0 is the ubiquitous "delegate access to a resource" flow.
OIDC layers identity on top ("here is the user who authorized this
access"). They're excellent for what they do. Quidnug and OIDC
solve different problems, and in fact **they compose cleanly**:
the Quidnug OIDC bridge (`cmd/quidnug-oidc/`) binds OIDC subjects
to Quidnug quids.

## What OAuth + OIDC do well

- **Delegated authorization.** Users grant apps access to
  specific scopes at specific providers.
- **Federation.** A user logs in once via an IdP (Okta, Google,
  Azure AD) and that identity flows across apps.
- **Access token lifecycle.** Refresh tokens, expiry, revocation.
- **Mature ecosystem.** Every major identity provider supports
  OIDC; every web framework has OAuth/OIDC middleware.

## What OAuth + OIDC don't do

- **Persistent identity across providers.** Your Google identity
  is unusable if Google turns off your account. Your OIDC
  subject from IdP X has no native bridge to IdP Y.
- **Relationships between users.** OIDC gives you a subject
  claim; it doesn't tell you who that subject trusts.
- **Per-viewer trust.** OIDC provides "is this user logged in?"
  — not "how much do I trust this user's claims?"
- **Signed event streams.** OIDC id-tokens are point-in-time;
  there's no audit log primitive.
- **Role / attribute graph.** Scopes and claims are flat strings
  in an id_token, not a queryable graph.

## What Quidnug adds

| Capability | OAuth / OIDC alone | OIDC + Quidnug |
| --- | --- | --- |
| User login | ✓ | ✓ (via OIDC bridge) |
| Delegated API access | ✓ | ✓ |
| Persistent identity across IdPs | session-scoped | Quidnug quid is IdP-independent |
| Relational trust | out of scope | native |
| Audit log | OIDC provider's (opaque) | native event streams |
| M-of-N recovery | IdP-dependent | native QDP-0002 |
| Signed attestations from non-IdP parties | n/a | native |

## The recommended architecture

**Use both.** Let OAuth / OIDC handle login and scoped API access;
let Quidnug handle relational trust, audit, and persistent
identity.

1. User logs in via their IdP (OIDC flow).
2. Your backend hands the verified ID token to
   `cmd/quidnug-oidc/`.
3. The bridge returns a Quid ID that's stably bound to the OIDC
   subject.
4. All audit and trust ops happen against the Quid ID, so the
   user can change IdP (switch from Okta to Azure) without losing
   their Quidnug history or trust network.

See [`internal/oidc/`](../../internal/oidc/) for the bridge
implementation and [`cmd/quidnug-oidc/`](../../cmd/quidnug-oidc/)
for the runnable service.

## When to use which

### Use OAuth / OIDC alone when

- You just need login and scoped API access.
- Your trust model is "user logged in ⇒ allowed."
- You don't need audit or long-term identity portability.

### Add Quidnug when

- You want to record what each user did as a signed, tamper-
  evident event stream.
- Different users have different trust in each other, and you
  want to compute relational scores.
- You need the user to survive IdP changes.
- You want M-of-N account recovery that doesn't depend on a
  single IdP's "forgot password" flow.

### Use Quidnug alone when

- Your system is pure backend-to-backend (no human login
  needed); OIDC doesn't fit.
- You want every participant to hold their own private key
  (Web3-style UX, no IdP dependence).
- You're building a system where identity explicitly isn't tied
  to an enterprise IdP.

## Pattern: IdP-free device-to-device trust

For pure backend or device-to-device flows, skip OIDC entirely:

```python
# Device A's signed event
from quidnug import Quid, QuidnugClient
client = QuidnugClient(NODE_URL)
device_a = Quid.from_private_hex(LOAD_FROM_SECURE_ENCLAVE)
client.emit_event(
    device_a,
    subject_id=device_a.id,
    subject_type="QUID",
    event_type="telemetry.temperature",
    domain="fleet.factory-7",
    payload={"celsius": 23.4, "sensor_id": "temp-07"},
)
```

Device B verifying Device A's events:

```python
events, _ = client.get_stream_events(device_a_id, domain="fleet.factory-7")
# Check that Device B trusts Device A relationally
trust = client.get_trust(device_b_id, device_a_id, domain="fleet.factory-7")
if trust.trust_level >= 0.7:
    # accept telemetry
```

No OIDC, no IdP, no login flow. Pure signed-event authentication
with relational trust.

## License

Apache-2.0.
