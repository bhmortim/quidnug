# Quidnug — Presentation Decks

## `quidnug-overview.pptx` (77 slides)

A comprehensive developer-overview deck covering:

1. **Opening + problem framing** (5 + 7 slides) — why today's trust systems
   (credit bureaus, reputation platforms, key-recovery flows, multi-party
   approval workflows) are broken by the same architectural choices.
2. **The core insight** (6 slides) — relational, per-observer trust with
   transitive composition.
3. **Core concepts** (10 slides) — quids, trust edges, domains,
   Proof-of-Trust consensus, transactions.
4. **Technical primitives** (11 slides) — event streams, key lifecycle,
   guardian M-of-N recovery, cross-domain gossip, push gossip, K-of-K
   bootstrap, fork-block, compact Merkle proofs, lazy epoch propagation,
   guardian resignation.
5. **Protocol evolution + architecture** (4 slides) — QDP process, the 10
   landed QDPs, node architecture, deployment patterns.
6. **14 use cases** (20 slides) — FinTech (5), AI (4), Elections (5),
   decentralized credit (1), healthcare / credentials / artifact signing (3),
   summary table.
7. **Comparison + when to use / not use** (4 slides).
8. **Getting started** (3 slides) — quick start, resources.
9. **Close** (3 slides) — three takeaways, final thought, Q&A.

Every slide carries detailed speaker notes (timing, what-to-say,
key-points, transition). Designed for a ~90-minute talk; can be
sectioned down.

**Visual design.** Custom midnight-navy + teal + amber palette (evoking
cryptographic trust rather than generic blue). Georgia headers + Calibri
body. Hexagonal accent motif repeated across slides. 16:9 widescreen.

## Regenerating

The deck is code-generated. To rebuild:

```bash
pip install python-pptx
python scripts/generate_presentation.py
# → docs/presentations/quidnug-overview.pptx
```

Generator modules live in `scripts/`:

- `gen_deck_core.py` — palette, helpers, slide-type builders
- `gen_deck_part1.py` — slides 1–18 (opening + problem + insight)
- `gen_deck_part2.py` — slides 19–40 (core concepts + primitives)
- `gen_deck_part3.py` — slides 41–58 (evolution + arch + FinTech + AI)
- `gen_deck_part4.py` — slides 59–77 (elections + credit + cross-industry + close)
- `generate_presentation.py` — runner

## QA

After generating, a quick QA cycle:

```bash
# Convert to PDF for visual review (requires LibreOffice)
"C:/Program Files/LibreOffice/program/soffice.com" \
    --headless --convert-to pdf \
    --outdir /tmp/deck_qa \
    docs/presentations/quidnug-overview.pptx

# Rasterize for inspection (requires pymupdf)
python -c "
import fitz
doc = fitz.open('/tmp/deck_qa/quidnug-overview.pdf')
mat = fitz.Matrix(1.25, 1.25)
for i in range(len(doc)):
    doc[i].get_pixmap(matrix=mat).save(f'/tmp/deck_qa/slide-{i+1:03d}.png')
"

# Extract text + speaker notes
python -m markitdown docs/presentations/quidnug-overview.pptx
```
