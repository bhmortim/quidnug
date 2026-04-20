# Reviews launch checklist

Step-by-step go-live sequence for the trust-weighted reviews
ecosystem once your public network is up. Work top-to-bottom;
each step gates the next.

---

## Pre-requisites

Before starting:

- [ ] Two public Quidnug nodes reachable via `api.quidnug.com`
      (`home-operator-plan.md` phases 1 through 3 complete).
- [ ] Nightly R2 backups running and verified (phase 5).
- [ ] Status page live at `status.quidnug.com` (phase 4).
- [ ] You've read [`governance-model.md`](governance-model.md)
      and understand the cache-replica / consortium-member /
      governor role separation. Domain registrations below assume
      you've picked your initial consortium and governor set.
- [ ] You've read
      [`../../examples/reviews-and-comments/PROTOCOL.md`](../../examples/reviews-and-comments/PROTOCOL.md)
      and understand the QRP-0001 event shapes.

If any of these is missing, fix it before continuing. Launching
reviews without monitoring or backups will bite you.

---

## 1. Register the topic tree

Do this once. The topic tree is what review queries bucket into.
Start narrow — you can always add children.

```bash
API=https://api.quidnug.com
SEEDS="node1-quid:1.0,node2-quid:1.0"       # consortium members
GOVS="operator-quid:1.0,cofounder-quid:1.0" # governors (2-of-2)

for topic in \
    reviews.public \
    reviews.public.technology \
    reviews.public.technology.laptops \
    reviews.public.technology.phones \
    reviews.public.technology.cameras \
    reviews.public.technology.software \
    reviews.public.technology.audio \
    reviews.public.books \
    reviews.public.books.fiction \
    reviews.public.books.non-fiction \
    reviews.public.movies \
    reviews.public.tv \
    reviews.public.restaurants \
    reviews.public.restaurants.us \
    reviews.public.restaurants.eu \
    reviews.public.services \
    reviews.public.services.professional \
    reviews.public.services.home \
    reviews.public.products; do
    quidnug-cli domain register \
        --node "${API}" \
        --name "${topic}" \
        --threshold 0.5 \
        --validators "${SEEDS}" \
        --governors "${GOVS}" \
        --governance-quorum 1.0 \
        || echo "skipped (probably exists): ${topic}"
done
```

After this runs, the public tree is "yours" — your consortium
produces the blocks, your governors control the roster. Anyone
else in the world can now spin up a cache replica pointing at
your consortium and start mirroring the agreed chain.

### 1a. Publish the governor attestation

File: `deploy/public-network/seeds.json`. Add a `governors`
block alongside the existing seed-node entries:

```json
{
    "seeds": [ ... existing ... ],
    "governors": [
        {
            "quid": "<operator-quid>",
            "name": "Operator (you)",
            "publicKey": "<sec1-uncompressed-hex>",
            "weight": 1.0,
            "role": "primary"
        },
        {
            "quid": "<cofounder-quid>",
            "name": "Co-founder",
            "publicKey": "<sec1-uncompressed-hex>",
            "weight": 1.0,
            "role": "primary"
        }
    ],
    "governanceQuorum": 1.0,
    "governanceNotice": "1440 blocks (~24 hours)"
}
```

Commit the signed JSON and render it at
`https://quidnug.com/network/seeds.json`. This is the
public attestation — anyone considering running a cache replica
should be able to audit who actually holds governance authority.

- [ ] All topics show up in `GET ${API}/api/domains`.
- [ ] A test identity can submit an IDENTITY tx under any of them.

---

## 2. Deploy the OIDC bridge

This is the lowest-friction bootstrap path for end users. It
maps an authenticated Google / GitHub / etc. identity to a
Quidnug quid, which gives new reviewers baseline credibility via
the operator root.

```bash
# On the VPS (NOT the home machine — this handles user auth):
sudo useradd -r -m -d /var/lib/quidnug-oidc -s /bin/false quidnug-oidc

# Deploy the binary:
sudo install -m 0755 -o quidnug-oidc -g quidnug-oidc \
    ./bin/quidnug-oidc /usr/local/bin/quidnug-oidc

# Config at /etc/quidnug-oidc/config.yaml — sample at
# cmd/quidnug-oidc/README.md
sudo systemctl enable --now quidnug-oidc
```

Put it behind Cloudflare as `auth.quidnug.com`. Add a CF Access
policy requiring anyone accessing the admin panel to have a
Quidnug-operator email, but keep the OAuth callback endpoints
public.

- [ ] `https://auth.quidnug.com/oidc/.well-known/openid-configuration`
      serves a valid discovery doc.
- [ ] Google login flow completes and produces a quid tied to the
      signed-in email.
- [ ] New quids get an automatic trust edge from
      `operators.network.quidnug.com` at weight 0.2 (the "baseline
      OIDC credibility" setting).

---

## 3. Lay down a founder trust seed

The first ~20 reviewers need to be people you know and can
attest for. Without this seed, every new reviewer starts
isolated.

```bash
# For each founding reviewer:
quidnug-cli trust grant \
    --node "${API}" \
    --key /path/to/your-operator.key.json \
    --truster "$(your-operator-quid)" \
    --trustee "<founder-quid>" \
    --domain "reviews.public" \
    --level 0.8 \
    --nonce <increment>
```

The 0.8 level propagates 1-hop-decayed to 0.64 for observers who
trust the founder. That's high enough for their reviews to
dominate the signal in small domains but not so high that a
compromised founder key destroys the network.

- [ ] At least 5 founder trust edges published.
- [ ] `api.quidnug.com/api/trust/<operator>/<founder>?domain=reviews.public`
      returns the expected level.
- [ ] Founders can log in via OIDC or CLI and post a first review.

---

## 4. Post a real seed review

Someone (probably you) writes the first real review on a real
product. Make it honest and representative — it'll be the first
thing a visitor sees.

```bash
quidnug-cli event publish \
    --node "${API}" \
    --key /path/to/reviewer.key.json \
    --subject-id "<reviewer-quid>" \
    --subject-type QUID \
    --event-type REVIEW \
    --sequence 1 \
    --domain "reviews.public.technology.laptops" \
    --payload-file review-1.json
```

Where `review-1.json`:

```json
{
    "qrpVersion": 1,
    "rating": 4.5,
    "maxRating": 5,
    "productAssetQuid": "<product-quid>",
    "title": "Great laptop for developers",
    "bodyMarkdown": "...",
    "locale": "en-US"
}
```

- [ ] Review visible via
      `GET ${API}/api/streams/<reviewer-quid>/events`.
- [ ] Shows up in the `<qn-aurora>` demo page once the page is
      pointed at the real product quid.

---

## 5. Wire the website's /reviews section

On the Astro site (separate repo):

```bash
npm install @quidnug/astro-reviews @quidnug/web-components @quidnug/client
```

Add pages:

- `src/pages/reviews/index.astro` — landing page explaining the
  system, with a live `<qn-aurora>` on a demo product and a
  `<qn-constellation>` drilldown.
- `src/pages/reviews/[product].astro` — per-product template
  that fetches contributors server-side and renders the
  full stack (aurora + trace + review list).
- `src/pages/reviews/integrate.astro` — the "five-minute
  integration" guide for third parties.

Point the client at `api.quidnug.com`:

```js
// src/lib/quidnug.js
import QuidnugClient from "@quidnug/client";
export const client = new QuidnugClient({
    defaultNode: "https://api.quidnug.com",
});
```

- [ ] `/reviews/` renders without CORS errors.
- [ ] The demo aurora shows a meaningful rating pulled from the
      live network.
- [ ] Schema.org JSON-LD appears in the HTML source (search-engine
      rich-result-ready).

---

## 6. Publish Schema.org rich-result metadata

For SEO, every review page must emit proper Schema.org
structured data. The `integrations/schema-org` Go package does
this server-side:

```go
import "github.com/quidnug/quidnug/integrations/schemaorg"

jsonLd, _ := schemaorg.Aggregate(events, productName, productURL)
```

Astro can call into a pre-generated JSON file per product, or
invoke a small Go sidecar.

- [ ] Google's [Rich Results Test](https://search.google.com/test/rich-results)
      passes for a sample product page.
- [ ] The public unweighted rating appears in the JSON-LD
      (consistent with what anonymous-observer sees).
- [ ] `<meta itemprop>` equivalents appear in the HTML.

---

## 7. Launch the WordPress plugin

Zip the plugin from `clients/wordpress-plugin/`, test-upload
against a scratch WordPress install, and then publish to the
WordPress Plugin Directory.

- [ ] Upload to a `wp-content/plugins/` test environment;
      activation succeeds without errors.
- [ ] WooCommerce product pages show the aurora primitive.
- [ ] A signed-in WP user can post a review that lands in the
      public Quidnug network.
- [ ] Plugin admin screen shows the node URL and caches are
      configurable.
- [ ] Plugin published to the WP directory (review process
      ~1-3 days).

---

## 8. Publish the peering protocol

The public network grows via bilateral trust edges. Make it
clear how third-party operators can join:

- [ ] `quidnug.com/network/` page shows the seed list
      (`seeds.json`), current validator quids, and the public
      peering-request address.
- [ ] [`peering-protocol.md`](peering-protocol.md) linked from
      the page in a prominent place.
- [ ] GitHub issue template for peering requests (use the
      `peering-request` label).
- [ ] The review flow for incoming requests is documented (see
      §4 of `README.md`).

---

## 9. Set moderation policies

Review content moderation is squirrely. Publish your rules
before the first abuse report lands.

- [ ] `quidnug.com/policy` page covers:
  - What content you will NOT gossip (CSAM, defamation you've
    ruled on, DMCA-removed content, etc.)
  - How to submit a takedown request
  - How the FLAG event works (community-driven de-weighting)
  - Your time commitment to reviewing flags (e.g. 48 hours
    to first response)
- [ ] Moderation dashboard wired up — at minimum, a Grafana panel
      showing `FLAG` event volume and unresolved FLAGs per
      reviewer.
- [ ] Spam-pattern detection rule: alert when a single IP posts
      >10 reviews in an hour.

---

## 10. Communications + launch

Once everything above is green:

- [ ] Blog post on `quidnug.com` announcing trust-weighted
      reviews. Include the "same 5 reviews, three ratings" demo.
- [ ] Tweet / Mastodon / LinkedIn link to the blog post with a
      screenshot of the aurora primitive.
- [ ] Submit to Hacker News, /r/programming, Lobsters.
- [ ] Reach out to 3-5 specific communities where trust-weighted
      reviews would resonate (dev-tool users, indie game
      reviewers, restaurant critics, etc.) and offer to onboard
      their first reviewers.
- [ ] Set aside 30 minutes per day for the first two weeks to
      respond to feedback and issues.

---

## 11. Post-launch: data you want to watch

For the first month, check daily:

| Metric | Where | Alarm threshold |
|---|---|---|
| Reviews posted / day | Grafana or `api.quidnug.com/api/registry/events?type=REVIEW` | < 5/day week 2 = investigate |
| New quids / day | Same | < 3/day week 2 = friction in signup |
| Aurora renders with basis vs. without | Browser console logging from the site | > 50% without basis = trust graph too sparse |
| Helpful vs. unhelpful ratio across HELPFUL_VOTE events | Grafana | < 70% helpful = review quality problem |
| Median node HTTP latency | Grafana | > 250ms p95 = scale the nodes |
| FLAG events per 100 reviews | Grafana | > 5 = moderation policy needs review |

The data from the first month tells you which part of the
ecosystem needs investment next: more topics, better bootstrap,
more moderation capacity, more nodes, or more outreach.

---

## What to do if something breaks

### "Reviews don't render"

- Check the browser console. CORS issue? The node config
  includes `cors_origins: ["https://quidnug.com"]` now?
- Check `status.quidnug.com`. Is the Worker up? Are both nodes
  healthy?
- Check the network panel. Is `api.quidnug.com` returning 200?

### "Reviews render but everyone sees 'no basis'"

- Your trust graph is too sparse. Add more founder trust edges.
- Check the observer: anonymous users legitimately see "no
  basis" until they sign in.
- Check that OIDC's baseline trust edge from `operators.*` is
  landing correctly.

### "A single user is posting obvious spam"

- Post `FLAG` events from several founder quids.
- If it continues, publish a `TRUST level=0.0` edge from your
  operator key to the spammer's quid — this marks them untrusted
  in your chain.
- Document the case in your moderation log.

### "The home node is down and it won't come back up"

- Confirm that Cloudflare Tunnel is still showing the tunnel as
  healthy in the dashboard (sometimes tunnel health flaps).
- SSH into the VPS and confirm node2 is healthy; the Worker
  should route to it automatically.
- `journalctl -u quidnug-node` on the home machine.
- Last-resort: restore from the most recent R2 snapshot to a
  scratch machine and redeploy.

---

## The end

Launching the reviews ecosystem is not a single-shot event. It
unfolds over weeks: trust graph slowly densifies, moderation
policy settles, integrations multiply. The checklist above gets
you to the start line. Operational discipline gets you through
the race.
