# Video 2 — Asset checklist

## Avatar

| Asset | Source | Status |
| --- | --- | --- |
| HeyGen custom avatar trained on your footage | 2-minute studio recording of you | |
| Voice clone for post-HeyGen overdubs | ElevenLabs (same clone used in Video 1's cloned segments) | |

## Motion graphics

| ID | Beat | Duration | New or shared? |
| --- | --- | --- | --- |
| M1 | Trust graph (all 3 frames: sparse, dense, POV shift) | 15s total | Shared with Video 1 |
| M2 | Quid identity reveal (person → key → hex ID) | 15s | New for V2 |
| M3 | Trust edge between two quids with domain label | 15s | New for V2 |
| M4 | "3 alternatives" box animation (centralized / blockchain / PGP) | 15s | New for V2 |
| M5 | Three-icon reveal (event log / guardian / gossip) | 20s | New for V2 |

Motion-graphics designer should receive this list as a single
brief; expect 3–4 days for delivery.

## Screen recordings (by you)

| Recording | Duration | Notes |
| --- | --- | --- |
| VS Code showing the WHITELIST→trust policy diff | 5s | Clean theme, 20px font |
| Terminal: `docker compose up -d` + curl health check | 10s | 3-node consortium starting |
| Language logos row (sourced PNG/SVG, not shot) | 3s | Simple Icons |

## Audio

| Asset | Duration | Source | Status |
| --- | --- | --- | --- |
| Music bed with 4-section energy curve | 3:05 | Epidemic — search "tech documentary energy" | |
| Transition whoosh ×5 | 0.5s each | Epidemic SFX | |
| Act-transition hit ×3 | 0.3s each | Epidemic SFX | |
| De-esser preset in Descript | — | Built into Descript | |

## Brand

| Asset | Source |
| --- | --- |
| Intro stinger (3s) | Shared across Tier A |
| Outro stinger (5s) | Shared |
| Quidnug wordmark | Existing |
| Color palette (navy, amber, green, red) | Shared |

## Thumbnail

| Asset | Source |
| --- | --- |
| Thumbnail (horizontal) | Freelance designer |
| Thumbnail (vertical) | Same designer, variant |
| TubeBuddy A/B test variants (×2) | Same designer |

## Directory

```
02-overview-3min/
├── assets/
│   ├── heygen-export.mp4
│   ├── music-bed.wav
│   ├── sfx/
│   ├── motion/
│   │   ├── M1-trust-graph.mp4
│   │   ├── M2-quid-reveal.mp4
│   │   ├── ...
│   ├── screen/
│   │   ├── whitelist-policy.mov
│   │   └── compose-startup.mov
│   └── thumbnails/
├── exports/
│   ├── v2-horizontal.mp4
│   ├── v2-vertical-60s-cut.mp4
│   └── v2-square.mp4
├── captions/
│   └── video-02-overview.en.srt
├── shotlist.md
├── assets.md
├── distribution.md
└── measurement.md
```

## License

Apache-2.0.
