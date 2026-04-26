# Stripe Connect integration

> Design + implementation guide for plumbing Stripe Connect
> into the Quidnug reviews ecosystem. Enables professional-
> reviewer monetization (tips, paid subscriptions) and the
> brand-disclosure marketplace (brand pays reviewer for a
> sponsored review).

**Status:** Design document. No code shipped yet.
**Depends on:** QRP-0002 (TIP and BID_ACCEPT event types),
service-api Worker (for the validation mapping).

## 1. Why Stripe Connect

Quidnug LLC is the platform; reviewers and brands are the
participants we move money to. Stripe Connect is the right
abstraction:

- **Compliance offloaded.** Stripe handles 1099 issuance,
  KYC/KYB on connected accounts, regulatory filings.
- **Payout management.** Reviewers see balance + payout
  schedule in their own Stripe-hosted dashboard; Quidnug
  doesn't touch the money flow.
- **Multi-rail.** Cards, ACH, SEPA, wire, instant payouts
  for premium reviewers.
- **Marketplace pattern.** Brand pays platform; platform
  takes a cut; reviewer gets the rest. All atomic.
- **Already in stack.** Stripe is the validation-tier
  payment processor; Connect adds the second leg without a
  new vendor relationship.

## 2. Account model

```
Quidnug LLC (Stripe platform account)
├── Validation customers (Stripe customers, not Connect)
│     Pay $19 Pro / $99 Business / custom Partner
│     For domain validation (already wired in service-api)
│
└── Stripe Connect accounts (Connected to platform)
      ├── Pro reviewer Express accounts
      │     Receive: tips, subscription fees, bid payouts
      │     Onboarded: alice-eats.com creates Express account
      │     Identity: Stripe collects to standard KYC
      │
      ├── Brand Express or Standard accounts
      │     Send: bid payouts to reviewers
      │     Onboarded: brand connects to fund the marketplace
      │
      └── Aggregator Express accounts (year 2+)
            Receive: subscription revenue from observers
            Distribute: revenue share to constituent reviewers
```

**Express vs Standard:**

- **Express** for individual pro reviewers and small brands.
  Stripe-hosted onboarding (~5 min), Stripe-hosted dashboard,
  minimal liability for Quidnug.
- **Standard** for larger brands or aggregators who already
  have their own Stripe account and want to keep their full
  dashboard.

**Custom** is an option for partner-tier customers who want
white-label payments; not in scope for v1.

## 3. Onboarding flows

### 3.1 Pro reviewer onboarding

Triggered when a validated reviewer (alice-eats.com, Pro+ tier)
opts into monetization in their reviewer portal:

```
1. Reviewer clicks "Enable tips and subscriptions" in portal.
2. service-api Worker creates a Stripe Express account:
     stripe.accounts.create({
       type: 'express',
       country: 'US',
       email: reviewer.email,
       capabilities: {
         transfers: { requested: true },
         card_payments: { requested: true },
       },
       business_type: 'individual',
       metadata: {
         quidnugReviewerQuid: reviewer.quid,
         quidnugDomain: reviewer.validatedDomain,
         tier: reviewer.tier,
       },
     })
3. Worker stores the connectedAccountId in the customer record.
4. Worker creates an account onboarding link:
     stripe.accountLinks.create({
       account: connectedAccountId,
       refresh_url: 'https://app.quidnug.com/reviewer/connect/refresh',
       return_url: 'https://app.quidnug.com/reviewer/connect/return',
       type: 'account_onboarding',
     })
5. Reviewer redirected to Stripe-hosted onboarding.
6. Stripe collects: legal name, DOB, SSN last 4, bank account.
7. On completion, Stripe webhook fires:
     account.updated → check capabilities.transfers === 'active'
8. Worker marks the customer record as monetization-ready.
9. Reviewer's profile page now shows tip button + subscription
    button.
```

### 3.2 Brand onboarding (for the marketplace)

Same Express flow, but marked `business_type: 'company'` and
collects EIN. Brand can fund their bid escrow via:

- One-time payment from a card.
- ACH from their bank.
- Stripe Treasury (for high-volume brands).

## 4. Money flows

### 4.1 Tip flow (one-time)

Observer tips a pro reviewer:

```
1. Observer on a review page clicks "Tip $5".
2. Widget POST to service.quidnug.com/v1/tips/intent:
     {
       reviewerQuid: '<alice-quid>',
       reviewTxId: '<tx-id>',
       amountUsd: 5.00,
       message: 'Great review',
     }
3. Worker creates a PaymentIntent with destination charge:
     stripe.paymentIntents.create({
       amount: 500,  // cents
       currency: 'usd',
       payment_method_types: ['card'],
       application_fee_amount: 50,  // 10% Quidnug platform fee
       transfer_data: {
         destination: reviewer.connectedAccountId,
       },
       metadata: {
         quidnugFlow: 'tip',
         quidnugReviewerQuid: reviewer.quid,
         quidnugReviewTxId: reviewTxId,
         quidnugTipperQuid: observer.quid,
       },
     })
4. Worker returns the client_secret.
5. Observer enters card in Stripe Elements; payment confirmed.
6. payment_intent.succeeded webhook fires.
7. Worker publishes a signed TIP event (QRP-0002 §5.6) on the
    observer's stream:
     {
       eventType: 'TIP',
       payload: {
         recipientQuid: reviewer.quid,
         reviewTxId: reviewTxId,
         amount: { value: 5.00, currency: 'USD' },
         method: 'stripe-connect',
         methodReference: paymentIntent.id,
         message: 'Great review',
       }
     }
8. Reviewer dashboard shows new tip ($4.50 net to them);
    Quidnug platform earns $0.50.
```

**Anonymous tipping.** If `tipperQuid` is an ephemeral
identity, the TIP event still publishes but the wallet shows
"anonymous tip." Observer must have authenticated to Stripe
(card payment) but doesn't expose their primary quid.

### 4.2 Subscription flow (recurring)

Observer subscribes to a pro reviewer's premium content:

```
1. Reviewer sets up subscription tiers in their portal:
     $5/month: early access
     $20/month: detailed reports + 1:1 Q&A
2. Service-api Worker creates Stripe Products/Prices in the
    PLATFORM account, not the reviewer's connected account
    (so platform retains control of the subscription state):
     stripe.products.create({
       name: `alice-eats subscriber: ${tier}`,
       metadata: { reviewerQuid: ..., tier: ... },
     })
     stripe.prices.create({
       product: productId,
       unit_amount: 500,
       currency: 'usd',
       recurring: { interval: 'month' },
     })
3. Observer subscribes via Checkout in Subscription mode,
    with destination charge:
     stripe.checkout.sessions.create({
       mode: 'subscription',
       payment_method_types: ['card'],
       line_items: [{ price: priceId, quantity: 1 }],
       subscription_data: {
         application_fee_percent: 10.0,
         transfer_data: { destination: reviewer.connectedAccountId },
       },
       success_url: ...,
       cancel_url: ...,
     })
4. Subscription created; monthly transfers run automatically.
5. Worker maintains a subscriber list per reviewer (D1 table).
6. Reviewer's portal shows subscriber count + MRR breakdown.
```

**Subscriber-only content.** When a subscriber views the
reviewer's profile page, the widget calls service-api to verify
active subscription, then unlocks content. The mechanism is
gated outside the protocol; the protocol stays public.

### 4.3 Brand-bid payout flow

Brand pays reviewer via the BRAND_BID → BID_ACCEPT chain
specified in QRP-0002 §5.7:

```
1. Brand has Express account with funded balance (or pays
    on-demand).
2. Brand publishes BRAND_BID event with terms.
3. Reviewer publishes BID_ACCEPT event.
4. Reviewer publishes the sponsored REVIEW with disclosure.
5. Brand's portal verifies the chain (BRAND_BID → BID_ACCEPT
    → DISCLOSURE → REVIEW), then triggers payout:
     stripe.transfers.create({
       amount: 20000,  // $200 minus $20 platform fee
       currency: 'usd',
       destination: reviewer.connectedAccountId,
       source_transaction: brand.fundingChargeId,
       metadata: {
         quidnugBidTxId: '...',
         quidnugAcceptTxId: '...',
         quidnugReviewTxId: '...',
       },
     })
6. Reviewer dashboard shows brand-payout line item.
7. Worker publishes a TIP event (or could be a separate
    BID_PAYMENT_RECEIVED event in QRP-0003).
```

**Escrow option (year 2+).** For brands worried about
reviewers not delivering, hold funds in Stripe Treasury or
a Quidnug-controlled escrow account. Release only after
the REVIEW event is published. Implementation requires
Stripe Treasury, which has higher onboarding requirements.

## 5. Platform fee structure

| Flow                  | Stripe processing | Quidnug platform fee | Reviewer net           |
|-----------------------|-------------------|----------------------|------------------------|
| Tip ($5 example)      | ~$0.45            | $0.50 (10%)          | $4.05                  |
| Subscription ($10/mo) | ~$0.59            | $1.00 (10%)          | $8.41                  |
| Brand bid ($200)      | ~$6.10            | $20.00 (10%)         | $173.90                |

**Considerations:**

- 10% is a reasonable starting platform fee (compares to
  YouTube's ~30%, Substack's 10%, Patreon's 5-12%). Adjust
  by tier or volume tier as the platform matures.
- Stripe fees are variable; we absorb in the reviewer's
  payout, not the platform fee.
- Pro reviewers earning >$X/year may unlock reduced platform
  fees as a retention incentive.

## 6. Compliance and tax

### 1099-K issuance

Stripe issues 1099-K forms to US connected accounts hitting
$600+ annual gross. We don't have to do anything; Stripe
handles it. We should communicate clearly to reviewers.

### Sales tax / VAT

Subscription revenue may be subject to state sales tax (US)
or VAT (EU/UK) depending on the buyer's location. Stripe Tax
on the platform account handles automatic calculation if
enabled. Initial recommendation: enable Stripe Tax and let
it be the default.

### KYC on connected accounts

Stripe collects KYC during onboarding (Express). For tipping
and small subscription flows, light KYC is sufficient (name +
DOB + last-4-SSN). For high-volume brand connections, full
KYB is required (EIN, beneficial ownership). Stripe enforces
both automatically based on volume thresholds.

### Money-transmission licensing

Connect avoids the licensing question for Quidnug LLC because
Stripe is the regulated entity. We're a marketplace operator,
not a money transmitter. Verify with counsel before launch
but this is the standard pattern.

## 7. Webhook handling

Three Stripe webhook endpoints in service-api:

- `/v1/webhooks/stripe/platform` - main platform events
  (validation customer signups, subscription state).
- `/v1/webhooks/stripe/connect` - Connect events
  (account.updated, payout.failed, transfer.created).
- `/v1/webhooks/stripe/treasury` (year 2+) - escrow events.

Each verifies signature with the corresponding webhook
secret. Idempotency by `event.id`.

Critical events to handle:

| Event                          | Action                                                              |
|--------------------------------|---------------------------------------------------------------------|
| account.updated                | Update connected-account state; enable monetization on capability change |
| account.deauthorized           | Reviewer disconnected; revoke monetization features                |
| payment_intent.succeeded       | (tip flow) Publish TIP event on observer's stream                  |
| invoice.paid                   | (subscription) Mark subscriber active for the period               |
| invoice.payment_failed         | Mark subscription delinquent, give 7-day grace                     |
| customer.subscription.deleted  | Subscriber canceled; remove from gated content                     |
| transfer.created               | Brand-bid payout; record in audit trail                            |
| transfer.failed                | Notify both parties; investigate; potentially reverse              |
| payout.paid                    | Reviewer received their balance                                    |
| payout.failed                  | Bank issue; notify reviewer to update banking info                 |
| account.application.deauthorized | Reviewer revoked Quidnug's access; deactivate features          |

## 8. UX touchpoints

### Reviewer portal additions

- **Connect Stripe** button when not yet connected.
- **Earnings dashboard:** total all-time, this month, last
  payout, next payout date.
- **Per-source breakdown:** tips / subscriptions / brand bids.
- **Tier and fee schedule** with deep link to Stripe-hosted
  dashboard for full balance/payout management.
- **Payout settings** (fast-track to Stripe-hosted).
- **Tax documents** link to Stripe's documents portal.

### Observer review widget additions

- **Tip button** next to helpfulness; opens Stripe Elements
  modal with quick amounts (1, 5, 20) plus custom.
- **Subscribe button** on reviewer card if reviewer offers
  subscriptions.
- **Sponsored badge** explicit on reviews with
  `disclosure.category == 'sponsored'` (already in QRP-0002
  §5.2).

### Brand portal (year 2+)

- **Funded balance display.**
- **Active bids and acceptance counts.**
- **Reviewer search and filter** (by topic, rep, KYB
  status).
- **Auto-payout on REVIEW event detection** (configurable;
  default: manual approval).

## 9. Implementation phases

### Phase 1 (~2 weeks): Tipping MVP

- Stripe platform account configured for Connect.
- service-api: Express account creation flow.
- service-api: Tip PaymentIntent + webhook handler.
- service-api: Publish TIP event after payment.
- Reviewer portal: connect button, earnings summary.
- Widget: tip button on reviewer cards.

Unblocks: any pro reviewer can accept tips.

### Phase 2 (~3 weeks): Subscriptions

- service-api: Product/Price creation per reviewer.
- service-api: Checkout subscription flow with destination
  charges.
- service-api: Subscription state in D1, gated-content lookup.
- Reviewer portal: subscription tier setup UI.
- Widget: subscribe button + gated content rendering.

Unblocks: pro reviewers can earn recurring revenue.

### Phase 3 (~3 weeks): Brand-bid marketplace

- service-api: BRAND_BID event publishing for funded brands.
- service-api: BID_ACCEPT verification + transfer trigger.
- Brand portal: funding flow, bid creation, payout dashboard.
- Reviewer portal: bid discovery, accept flow.
- Documentation: brand and reviewer onboarding guides.

Unblocks: brands buy honest reviews on-protocol.

### Phase 4 (year 2+): Escrow and aggregator revenue share

- Stripe Treasury for escrowed brand payouts.
- Aggregator account model + revenue split logic.
- Multi-currency support beyond USD.

## 10. Open decisions

1. **Platform fee tiers.** Flat 10% or graduated? Default to
   flat for v1; revisit after volume data.
2. **Tipping minimum.** $1 minimum keeps Stripe fees as a
   reasonable percentage of small tips. Alternative: aggregate
   small tips off-protocol and bulk-transfer weekly (more code,
   better UX for reviewers).
3. **Identity binding.** Should the reviewer's Stripe Express
   account require their validated domain to match the email?
   Stricter binding reduces fraud but adds friction.
4. **Refund policy.** Tips are non-refundable. Subscriptions
   are refundable through Stripe Customer Portal. Brand bids
   are refundable only if the reviewer doesn't deliver
   (manual review).
5. **Sponsored-review discovery vs disclosure.** A brand may
   prefer to publish bids privately (only to a curated list
   of reviewers). Should QRP-0002's BRAND_BID event support
   private bids? (Requires invite-only event variant; not in
   v1.)

## 11. Files in this integration

- **README.md** (this file): design and implementation plan.
- **schemas/** (TBD): JSON Schema definitions for the Stripe
  webhook events Quidnug processes, and for the
  service-api endpoints that map to QRP-0002 events.
- **lib/stripe-connect.ts** (TBD): TypeScript helpers used by
  service-api Worker.
- **examples/** (TBD): cURL/JS examples for each flow.

## 12. References

- Stripe Connect docs: https://stripe.com/docs/connect
- Stripe Express onboarding:
  https://stripe.com/docs/connect/express-accounts
- Stripe Tax: https://stripe.com/docs/tax
- QRP-0002 (this repo): TIP event §5.6, BRAND_BID/BID_ACCEPT
  §5.7
- service-api spec (covered in design conversation, to be
  formalized in `docs/api/service-api.md`).
