# Carbon Credits Are Being Gamed — Presentation

75-slide PowerPoint deck covering why voluntary carbon markets produce phantom credits, how weak attestation chains enable double counting and over-crediting, and how the Quidnug Proof of Trust architecture gives registries, verifiers, and buyers a cryptographic basis for auditable climate claims.

Companion to the blog post at [`docs/blogs/2026-04-27-carbon-credits-are-being-gamed.md`](../../blogs/2026-04-27-carbon-credits-are-being-gamed.md).

## Rebuild

```bash
python build_charts.py   # regenerates PNG charts into assets/
python build_deck.py     # regenerates carbon-credits.pptx
```

Requires `python-pptx`, `matplotlib`, and `Pillow`. Charts write to `assets/`; the final deck writes to `carbon-credits.pptx` in this directory.

## Sources cited

- West, Köhler, Schneider et al. *Science* (2023): phantom credit rates in REDD+ projects
- The Guardian / Die Zeit / SourceMaterial: REDD+ investigation (Jan 2023)
- Cooley, Anderegg et al. (2022): forest carbon permanence and reversal risk
- ICVCM Core Carbon Principles
- VCMI Claims Code of Practice
- ICAO CORSIA eligibility framework
- Verra VCS methodology updates 2023-2024

## Quidnug architecture touchpoints

- QDP-0001 identity + key rotation for registries, verifiers, and developers
- QDP-0002 evidence objects for sensor readings, field surveys, and methodology docs
- QDP-0006 attestation chain from raw measurement to issued credit
- QDP-0014 revocation + correction flows for retroactive quality downgrades
- QDP-0019 PoT trust graph scoring methodology houses and specific verifier firms independently
