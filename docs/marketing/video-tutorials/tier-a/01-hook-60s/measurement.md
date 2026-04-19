# Video 1 — measurement plan

What to measure after publish, and what to do when the numbers
look a certain way.

## Primary metrics

| Metric | Platform | Target | Check at |
| --- | --- | --- | --- |
| Thumbnail CTR | YouTube | ≥ 4% | 48h, 7d |
| 30-second retention | YouTube | ≥ 60% | 48h, 7d |
| Average view duration | YouTube | ≥ 45s | 7d |
| Likes:dislikes ratio | YouTube | ≥ 10:1 | 30d |
| LinkedIn impressions | LinkedIn | ≥ 5000 | 7d |
| LinkedIn reactions | LinkedIn | ≥ 30 | 7d |
| Twitter/X views | Twitter | ≥ 3000 | 7d |
| Profile visits after click | LinkedIn/Twitter | ≥ 50 | 7d |

## Decision criteria

### If CTR < 2% at 48h
**Signal:** thumbnail is not arresting enough.
**Action:** run TubeBuddy A/B with 2 alternate thumbnail
variants. Leave for 7 days, pick winner.

### If 30-second retention < 40%
**Signal:** hook is too slow or voice quality issue.
**Action:** re-record the first 10 seconds tighter; re-upload
as a new video (YouTube does not allow re-uploads with new
video to the same URL).

### If average view duration < 30s
**Signal:** the middle 20–40s is losing people.
**Action:** check the retention graph — where exactly do they
drop? If around 0:27 (the wordmark), your music bed is
probably muddying the moment; simplify.

### If LinkedIn impressions < 2000 after 48h
**Signal:** caption hook is weak OR LinkedIn is suppressing
your post.
**Action:** repost in 2 weeks with a completely different
hook (first line of the caption). Don't edit the existing
post — LinkedIn punishes edits.

### If likes:dislikes < 3:1
**Signal:** you're getting the wrong audience, or the claim is
triggering skeptical comments without a chance to respond.
**Action:** respond to every negative comment personally
within 24h. Pin a supportive comment from a real technical
viewer if you can.

## What success looks like at 30 days

A realistic Tier-A-first-video success benchmark:

- **YouTube**: 800–2000 views, 50–150 likes, 5–20 comments, 3%+ CTR
- **LinkedIn**: 8000–20 000 impressions, 40–120 reactions, 20–60 comments
- **Twitter**: 5000–15 000 views, 20–80 bookmarks, 5–30 replies
- **Traffic to github.com/bhmortim/quidnug**: ~50–200 sessions
- **GitHub stars delta**: +15 to +40

If you hit the low end across the board, the video is working
but needs more reach (share it to more communities — see
distribution.md). If you hit the high end, Video 2 becomes
the priority and you can delay Video 3.

## What failure looks like

- Below 1000 LinkedIn impressions AND below 300 YouTube views
  after 7 days is **the video is broken** territory.
- Debug in this order:
  1. Listen back to the first 10 seconds in Bluetooth earbuds
     in a noisy environment (LinkedIn's actual use condition).
     Is the voice even clear?
  2. Open the thumbnail at 180×101 pixels (LinkedIn's mobile
     feed size). Can you still read the text?
  3. Re-read the opening script. Does it actually make sense
     to someone who doesn't know Quidnug?
- If all 3 pass, the problem is distribution, not the video.
  Post in 5 more communities.

## Weekly report template

```
Video 1 — What is Quidnug? — Week N report

YouTube
- Views:            X
- CTR:              X%
- Avg view time:    X:XX / 0:60
- Likes:            X
- Comments:         X

LinkedIn
- Impressions:      X
- Reactions:        X
- Comments:         X
- Profile visits:   X

Twitter
- Views:            X
- Bookmarks:        X
- Replies:          X

Repo
- Stars this week:  +X
- Unique cloners:   X
- README traffic:   X sessions (from github-referrer)

Actions taken this week:
- [action 1]
- [action 2]

Actions for next week:
- [decision based on metric]
```

## License

Apache-2.0.
