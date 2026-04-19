# Video production checklist

Per-video checklist to avoid common rework. Go through this
every time.

## Pre-production

- [ ] Script is finalized (not "mostly finalized"). Every "uh"
      in the recording costs Descript editing time later.
- [ ] Any code snippets in the script have been tested in a
      fresh environment **today**. APIs drift; don't trust
      "it worked last week."
- [ ] Architecture / diagram assets exported to MP4 (3–6s each)
      at 1920×1080.
- [ ] Recording environment set up:
  - [ ] Quiet room; close windows; turn off HVAC cycling if
        possible.
  - [ ] Microphone tested — listen back to 30 seconds at
        headphone level.
  - [ ] Terminal font ≥ 20 px, high-contrast theme.
  - [ ] Browser at 100% zoom minimum (150% for dev content).
  - [ ] All tabs except the ones in the video closed.
  - [ ] Notifications silenced (OS-level Do Not Disturb).
- [ ] Lighting: if on camera, one front-fill key light, avoid
      backlight (window behind you). Sit 3 ft from camera.

## Recording

- [ ] Read the script — don't improvise "I'll just know what to
      say." You won't. Teleprompter if possible.
- [ ] Multiple takes OK; you'll cut in Descript.
- [ ] Leave 2 seconds of silence at the start and end for
      trimming.
- [ ] For screen recordings: 1080p minimum, 60fps ideal.
- [ ] For voice: 48kHz, 16-bit or 24-bit.

## Editing (Descript)

- [ ] Run filler-word removal.
- [ ] Cut every pause longer than 400ms.
- [ ] Add chapter markers (YouTube chapters are SEO gold).
- [ ] Burn-in captions for the social cut; optional-open for
      YouTube main cut.
- [ ] Music under narration: -18 to -24 dB. Not louder.
- [ ] Intro card (title + logo) max 3 seconds. Outro card max
      5 seconds.

## Export

- [ ] 1920×1080 horizontal for YouTube.
- [ ] 1080×1920 vertical (first 60s of the video) for LinkedIn,
      Shorts, TikTok.
- [ ] 1080×1080 square for Instagram feed.
- [ ] MP4, H.264, AAC. 8–12 Mbps target bitrate.
- [ ] Thumbnail: 1280×720, bold single-line text, high
      contrast, face or diagram.

## Publish

- [ ] YouTube: title ≤ 60 chars, description with timestamps +
      repo link + this script in the first comment.
- [ ] Twitter / X: vertical clip, caption with the key "aha."
- [ ] LinkedIn: vertical clip, caption written for technical
      leaders, tag your account.
- [ ] HackerNews: wait for the 3-min overview video before
      posting — don't burn HN on a 60-sec hook.
- [ ] Update `docs/marketing/video-tutorials/README.md` with
      the published URL.
- [ ] Add to the repo-root README under a new "Videos" section
      once 3+ are published.

## Post-publish

- [ ] Check analytics at T+48h:
  - Retention curve — where are people dropping?
  - CTR on thumbnail — above 4% is healthy, below 2% is a
    thumbnail problem.
- [ ] Respond to every comment in the first week.
- [ ] If retention is strong but CTR is weak, re-thumbnail.
- [ ] If retention drops at a specific timestamp, study it —
      probably a pacing issue you can fix in the next video.

## License

Apache-2.0.
