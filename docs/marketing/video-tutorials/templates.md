# Templates for the remaining videos

The first 5 scripts (Tier A) are fully written in `scripts/`.
The next 13 follow the same structural templates below. Each
template has the 3-act structure + visual cues — fill in the
language- or feature-specific content from the existing repo
docs.

---

## Template A — SDK quickstart (5 min)

Covers: Go (#6), JavaScript + React (#7), Rust (#8).

```
[0:00 Intro — 20s]
"In this video, <SDK_NAME> SDK from zero to working in 5 min."

[0:20 Install — 40s]
<install command from README>

[1:00 Start local node — 40s]
"docker compose up -d" (same across all)

[1:40 Code the quickstart — 1:40]
<paste the SDK's 30-second example verbatim>
Walk through each line.

[3:20 Run it + output — 30s]

[3:50 Transitive-trust snippet — 40s]
Add Carol, show 0.9 × 0.8 = 0.72.

[4:30 Where to go next — 30s]
Async variant (for Python), full protocol surface,
use-case examples.
```

Source material: the SDK's README `examples/` directory has the
working code. Paste it, explain it.

---

## Template B — protocol feature deep-dive (7 min)

Covers: Guardian recovery (#10), cross-domain gossip (#11),
HSM signing (#15), OIDC bridge (#16).

```
[0:00 Hook — 30s]
"The feature in one sentence."

[0:30 Problem it solves — 1:00]
"Here's what's painful without this."

[1:30 Architecture — 1:30]
Animated diagram, walk through each box.

[3:00 Demo — 2:30]
Terminal + code, run the flow end-to-end.

[5:30 Integration recipe — 1:00]
"In your app, here's where this fits."

[6:30 Outro — 30s]
Reference docs + runnable example link.
```

Source material: the corresponding QDP design doc and any
integration README in `pkg/signer/`, `integrations/`, or
`cmd/`.

---

## Template C — use-case walkthrough (8–10 min)

Covers: Elections (#14), W3C VCs (#13), supply-chain provenance.

```
[0:00 The real-world problem — 1:00]
Concrete scenario, not abstract.

[1:00 Why Quidnug fits — 1:30]
What the existing approaches miss.

[2:30 Full flow demo — 4:00]
All actors, all signatures, all events.

[6:30 Audit / consumer walkthrough — 2:00]
Downstream query + trust scoring.

[8:30 Limitations — 30s]
What Quidnug does NOT solve.

[9:00 Outro — 1:00]
Link to runnable example + README.
```

Source material: `examples/*/README.md` + the runnable script
in that directory.

---

## Template D — comparison video (5–10 min)

Covers: Quidnug vs OAuth/OIDC (#9.5), vs blockchain
(unassigned #), vs PGP WoT.

```
[0:00 Framing — 30s]
"Composes vs competes vs complementary."

[0:30 What <OTHER> does well — 1:30]
Honest strengths.

[2:00 What <OTHER> doesn't do — 1:30]
Specific gaps.

[3:30 How Quidnug fills those — 2:00]
Architecture pattern.

[5:30 Decision table — 1:00]
"If you need X, use Y."

[6:30 Outro — 30s]
```

Source material: the comparison docs already exist in
`docs/comparison/vs-*.md`. Each doc is a direct script source.

---

## Template E — operations / platform-eng (8 min)

Covers: Helm deploy + observability (#17), CI/CD integration,
capacity planning.

```
[0:00 What we're building — 30s]
"A production Quidnug node with Prometheus, Grafana, alerts."

[0:30 Helm install — 2:00]
Walk through values.yaml highlights.

[2:30 Observability bundle — 2:00]
Grafana dashboard tour.

[4:30 Alert rules — 1:30]
Show Prometheus rules + what they catch.

[6:00 Capacity planning — 1:30]
Benchmark numbers, resource sizing heuristic.

[7:30 Outro — 30s]
```

Source material: `deploy/helm/`, `deploy/observability/`,
`tests/benchmarks/README.md`.

---

## Template F — 30-minute conference talk

This is (#18) — essentially the 77-slide deck delivered as a
video.

Two production options:

**Option 1 — record yourself.** Real camera, real you, slides
via OBS picture-in-picture. Best authenticity but highest time
cost.

**Option 2 — HeyGen avatar narrating the existing deck.**
Speaker notes in the deck are already written. Paste notes
into HeyGen; each slide becomes one segment; export as one
long MP4. Lower cost but less personal.

Recommendation: do Option 1 for launch, Option 2 for refresh
when the deck updates.

Source material: `docs/presentations/quidnug-overview.pptx`.

---

## License

Apache-2.0.
