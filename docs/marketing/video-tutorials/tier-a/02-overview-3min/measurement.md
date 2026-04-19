# Video 2 — measurement plan

Video 2 is the flagship funnel — it's where HN / LinkedIn
traffic actually lands. Metrics matter most at this specific
video.

## Primary metrics

| Metric | Platform | Target | Stretch | Check at |
| --- | --- | --- | --- | --- |
| HN score | HN (Day 0 submission) | ≥ 50 | ≥ 200 | 24h |
| Hours on HN front page | HN | ≥ 4h | ≥ 12h | 48h |
| 50%-mark retention | YouTube | ≥ 40% | ≥ 55% | 7d |
| Average view duration | YouTube | ≥ 1:45 / 3:00 | ≥ 2:15 / 3:00 | 30d |
| Thumbnail CTR | YouTube | ≥ 3.5% | ≥ 6% | 7d |
| YouTube subscribers gained | YouTube | +25 | +100 | 30d |
| GitHub stars (48h post HN) | GitHub | +50 | +300 | 48h |
| Repo traffic YouTube-referred | GitHub Insights | +500 sessions | +3000 | 7d |

## HN-specific signal

HN is a binary outcome: either you hit the front page, or you
don't. If the video hits the front page:

- **Top 30 for 2+ hours**: strong result. Expect +200 stars,
  ~50 Python SDK downloads, ~10 serious technical comments
  on HN.
- **Top 10 for 1+ hour**: flagship result. Expect +1000 stars,
  recruiter inbound, 3-5 cross-promotion asks from adjacent
  projects.
- **Never above #50**: did not land. Do NOT resubmit the same
  video; instead, wait 2 weeks and submit a companion blog
  post with a different angle.

Peak times to submit: Tuesday / Wednesday 8–10am PT. Avoid
Mondays (US holidays / catch-up days) and Thursday afternoons
(HN burnout). Avoid any Friday.

## LinkedIn-specific signal

For the identity-architect and CTO audience, LinkedIn
impressions + profile visits are more predictive of real
integration intent than YouTube views. Aim for:

- **8000+ impressions within 7d** on a single post.
- **30+ profile visits** to your LinkedIn within 7d —
  high-quality signal that someone is evaluating you as a
  potential vendor/partner/collaborator.
- **10+ "connect" requests** with personalized notes.

## Decision criteria

### If HN lands but YouTube retention < 30%
**Signal:** You got the click but the video didn't hold the
attention. The 3-minute version is too long for the HN
audience, or Act 1 is slow.
**Action:** Cut a 90-second "HN version" — just Acts 2 + 3 —
and publish as a separate video. Swap the embed on HN-linked
pages.

### If HN doesn't land but YouTube CTR > 5%
**Signal:** Product-market fit for the YouTube algorithm but
not for the HN crowd. Not a failure — YouTube can compound.
**Action:** Keep the video. Pivot distribution to LinkedIn
+ Reddit + cross-SDK-adjacent communities. Skip HN for
Videos 3–5.

### If YouTube views < 1000 at day 7 AND HN didn't land
**Signal:** You're not hitting your target audience.
**Action:** A/B test thumbnail via TubeBuddy. If CTR
doesn't recover in 7 days, rewrite the title — most likely
the title itself (not the thumbnail) is the problem.

### If GitHub stars don't move post-HN
**Signal:** HN landed but nobody converted to the repo.
**Action:** The HN comment wasn't good enough. Review the
comment thread; the top comments should link to concrete
technical content. Add pinned comments that link directly
to `tests/interop/` or a specific QDP.

## What success looks like at 30 days

Realistic flagship-video benchmarks:

- **HN**: 200+ points, 100+ comments, front page for 6+ hours.
- **YouTube**: 5000–20 000 views, 60% retention, +100 subs,
  30+ substantive comments, CTR 4–6%.
- **LinkedIn**: 15 000+ impressions, 100+ reactions,
  50+ profile visits.
- **Twitter**: 20 000+ views on the thread, 50+ bookmarks.
- **GitHub**: +200 to +1000 stars (HN effect), +100 clones.
- **Inbound**: 2–5 partnership / sponsorship / media inquiries.

## What failure looks like

Below 500 YouTube views AND below 20 HN points AND below +20
GitHub stars at 7 days is "something is broken" territory.
Debug checklist:

1. Is the video quality actually good? Watch on a 4K monitor
   and a phone side by side.
2. Is the title clear? A dev scanning HN should understand the
   subject from the title alone.
3. Is the thumbnail arresting? Open the YouTube trending page
   for your category — does yours hold its own?
4. Is your HN submission window right? Tuesday 9am PT > every
   other time.
5. Is your HN first-comment actually useful? Top HN posts
   have maintainer-written, non-promotional first comments
   that add context.

## Cross-reference with repo analytics

GitHub → Insights → Traffic view shows:
- Referrer: `youtube.com`, `news.ycombinator.com`,
  `linkedin.com` — break down to see which channel converts.
- Unique cloners: conversion from "clicked the repo link" to
  "cloned the repo." If high clone rate but low star rate,
  your README is doing the converting, video just delivered
  qualified traffic.

## Weekly report (template)

```
Video 2 — Quidnug in 3 minutes — Week N report

YouTube
- Impressions:            X
- Views:                  X
- CTR:                    X%
- Avg view duration:      X:XX / 3:00
- 50%-mark retention:     X%
- Subscribers gained:     +X
- Substantive comments:   X
- Engagement (likes):     X

Hacker News (if submitted)
- Peak position:          #X
- Final points:           X
- Comments:               X
- Hours on front page:    X
- Top comment author:     @X

LinkedIn
- Impressions:            X
- Reactions:              X
- Profile visits:         X
- New connections:        X

Twitter
- Thread views:           X
- Bookmarks:              X
- Quote-tweets:           X
- Replies (substantive):  X

GitHub (Day 0 to Day 7)
- Stars gained:           +X
- Unique clones:          +X
- Top referrer:           [youtube/hn/linkedin]
- Top path:               [README / docs/*.md]

Actions taken:
- [action 1]
- [action 2]

Key learning:
- [one sentence]
```

## License

Apache-2.0.
