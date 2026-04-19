# Video 1 — Asset checklist

Track every piece of media the video needs. Check each off
before you start editing.

## Audio

| Asset | Duration | Format | Source | Status |
| --- | --- | --- | --- | --- |
| Voice-over (your real voice) | 55s | 24-bit 48kHz WAV | You, recorded at home | |
| Music bed "tech documentary" | 60s | WAV from Epidemic | `epidemicsound.com` search | |
| SFX: whoosh transitions (×3) | 1s each | WAV | Epidemic SFX library | |
| SFX: question-mark ding | 0.5s | WAV | Epidemic | |
| SFX: shatter (score break) | 1s | WAV | Epidemic | |
| SFX: crisp UI click | 0.3s | WAV | Epidemic | |

## Video — motion-designed

Delivered by the freelance motion designer.

| Beat | Description | Duration | Delivered? |
| --- | --- | --- | --- |
| M1 | Monolithic "TRUST SCORE: 87" card appears | 3s | |
| M2 | M1 shatters into nodes flying out | 3s | |
| M3 | Sparse trust graph assembles | 5s | |
| M4 | Dense graph with weighted edges | 4s | |
| M5 | POV shift — camera moves between observer viewpoints | 5s | |
| M6 | Person + keypair icons merge into "quid" concept | 8s | |
| M7 | Transitive trust chain Alice→Carol→Bob with weights | 6s | |
| M8 | Three observers, each with different numeric score for same target | 4s | |

## Video — stock / Storyblocks

| Beat | Search terms | Duration needed | Status |
| --- | --- | --- | --- |
| Bank app "Approved" screen | "banking mobile approved UI close up" | 2s | |
| Yelp 5-star screenshot | "5 star review restaurant" | 1s | |
| Verified blue check close-up | "verified badge social media" | 1s | |
| Amazon 5-star animation | "5 star rating product" | 1s | |

## Video — shot by you

| Beat | Description | Duration | Status |
| --- | --- | --- | --- |
| Terminal echoing `quid generate` | Clean terminal, large font, command + output | 2s | |

## Brand assets

| Asset | Dimensions / format | Source | Status |
| --- | --- | --- | --- |
| Quidnug wordmark | 1920×1080 alpha PNG | Existing in `docs/presentations/` | |
| Language logos row | Horizontal strip, 1920×200 | Simple Icons repo (SVG) | |
| Intro stinger (3s) | 1920×1080 MP4 | Fiverr freelancer | |
| Outro stinger (5s) | 1920×1080 MP4 | Fiverr freelancer | |
| Thumbnail — horizontal | 1280×720 PNG | Thumbnail designer | |
| Thumbnail — vertical preview | 1080×1920 PNG | Thumbnail designer | |

## Typography

Embedded in motion graphics — confirm designer has these:
- **Inter Bold** for headings
- **Space Grotesk Bold** as alternate
- **Inter Regular** for body
- **JetBrains Mono** for code overlays

## Color palette (brand kit)

| Role | Hex | Used where |
| --- | --- | --- |
| Primary navy | `#0B1D3A` | Backgrounds, body text on light |
| Accent amber | `#F9A825` | CTAs, glowing trust edges |
| Success green | `#2E8B57` | High-trust edges, checkmarks |
| Warning red | `#C0392B` | Low-trust edges, "rejected" states |
| Light neutral | `#F7F7F5` | Alt backgrounds |
| Code background | `#1E1E1E` | Terminal, code overlays |

## Captions file (SRT)

Auto-generate in Descript, then hand-correct. Save as:
- `video-01-hook-60s.en.srt` — English, to upload to YouTube
- Burned-in version in the vertical / square cuts

## Directory structure when done

```
01-hook-60s/
├── assets/
│   ├── voiceover.wav
│   ├── music-bed.wav
│   ├── sfx/
│   ├── motion/
│   │   ├── M1-trust-score-appears.mp4
│   │   ├── M2-shatter.mp4
│   │   ├── ...
│   ├── stock/
│   ├── terminal-recording.mov
│   └── thumbnail-variants/
├── exports/
│   ├── v1-horizontal-1920x1080.mp4
│   ├── v1-vertical-1080x1920.mp4
│   └── v1-square-1080x1080.mp4
├── captions/
│   └── video-01-hook-60s.en.srt
├── shotlist.md
├── assets.md
└── distribution.md
```

## License

Apache-2.0.
