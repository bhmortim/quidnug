# Bootstrapping trust for new reviewers

A trust-weighted review system faces a chicken-and-egg problem:
new reviewers have no trust, so their reviews carry no weight,
so they can't build reputation. Three mechanisms solve this.

## Mechanism 1 — OIDC bridge (easiest)

The Quidnug OIDC bridge (`cmd/quidnug-oidc/`) provisions a quid
bound to an OIDC identity. When a user signs in with Google /
Apple / GitHub / Facebook:

1. The bridge resolves or creates a quid for their OIDC subject.
2. The binding is immutable — same IdP subject → same quid forever.
3. The IdP provider gets issued a trust edge on sign-up:
   "@GoogleIdP → <user-quid> at 0.5 in `reviews.public`" —
   signed by the bridge's quid.

Result: new Google sign-ups start at trust 0.5 from anyone who
trusts the Google-OIDC-bridge provider (most observers do at
trust 0.7-0.9, so the transitive chain gives new users
0.35-0.45 right away).

**Caveat:** this gives Google a lot of implicit power. Users
can delete their trust edge to Google-OIDC-bridge if they
prefer to bootstrap another way.

## Mechanism 2 — Cross-site import

For existing reviewers on platforms like Amazon / Yelp / Google
Maps / TripAdvisor:

1. The existing platform (either voluntarily or via a user-
   initiated data-export flow) signs an import attestation:
   "Amazon reviewer `helpful-user-1234` with N helpful votes
   since 2018 is now bound to Quidnug quid X."
2. The user posts that attestation as their first event on the
   Quidnug network.
3. Observers who trust Amazon's moderation (explicit trust edge
   to `amazon-bridge-quid`) propagate trust to the imported
   reviewer based on the attestation's `helpfulVoteCount`.

Amazon etc. aren't required to participate. But a user can
post the Amazon review URLs as attestation (via screenshots +
OCR or a helper CLI), and observers can choose to honor this
soft attestation at lower weight than a signed import.

## Mechanism 3 — Social bootstrap

For users who don't want OIDC or cross-site import:

1. Generate a fresh quid in the browser extension.
2. Share the quid ID with friends / colleagues who already use
   Quidnug.
3. They issue trust edges to you explicitly:
   `alice trusts newbie at 0.8 in reviews.public.technology`.
4. After 3-5 people you know have vouched for you, you have
   transitive-trust reach into everyone in their networks.

For reviewers: ask friends who already post in your topic area
to vouch. Ask the moderators of online communities (Reddit
mods, Discord admins, subject-area Twitter figures) to extend
a boot-strap trust to you.

## Mechanism 4 — Purchase-based bootstrap (for e-commerce)

Retailers emit PURCHASE events for verified purchasers
(QRP-0001 §5.5). Observers can weight reviews from verified
purchasers higher:

```
if review has PURCHASE event from a retailer observer trusts at ≥ 0.7:
    # give this reviewer temporary trust boost
    effective_t = max(t, 0.3 * retailer_trust)
```

A user reviewing their first product ever, but from a retailer
the observer trusts, can still get their review counted at
~0.2-0.3 weight.

## What observers CAN trust about new reviewers

A fresh Quidnug quid has effectively zero trust from strangers.
But an observer can still give a new reviewer some
consideration via:

- **Length + thoughtfulness of the review text** (NOT part of
  QRP-0001, but a reasonable client-side heuristic).
- **PURCHASE events from trusted retailers**.
- **Consistency with the observer's own empirical experience**
  (hard to automate, but users can see a reviewer once in real
  life and decide to trust them manually).

## Preventing zero-bootstrap brigading

A coordinated campaign can't just mass-register bots and vote
on each other — because that clique has zero trust from any
legitimate observer. But they can still:

1. Socially engineer one legitimate trust edge into their
   clique. Fix: observers should be skeptical of anyone
   extending trust to strangers; the trust graph is your
   reputation, be careful.
2. Use the OIDC bridge with fake Google accounts. Fix: bridges
   can require additional verification (phone number, captcha)
   before issuing bootstrap trust. This is outside QRP-0001's
   scope.

## Recommended deployment sequence

For a new Quidnug reviews deployment:

1. **Week 1 — seed with trusted accounts.** Issue bootstrap
   trust to known-good reviewers (e.g., tech journalists for
   `reviews.public.technology`, food critics for
   `reviews.public.restaurants`). These form the "root set"
   that transitively vouches for everyone else.
2. **Week 2 — open OIDC bridge.** Let regular users sign up
   via Google / Apple. They inherit bootstrap trust at 0.5
   from the IdP bridge.
3. **Week 3 — enable cross-site import.** Let Amazon / Yelp
   users bring their reputation across.
4. **Week 4 onwards — organic growth.** Helpfulness votes
   propagate. The graph becomes self-reinforcing.

## License

Apache-2.0.
