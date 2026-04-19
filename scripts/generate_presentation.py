"""Generate the Quidnug overview deck (~86 slides) to docs/presentations/."""
import os
import sys
from pathlib import Path

SCRIPT_DIR = Path(__file__).resolve().parent
sys.path.insert(0, str(SCRIPT_DIR))

from pptx import Presentation
from pptx.util import Inches

from gen_deck_part1 import build_part1
from gen_deck_part2 import build_part2
from gen_deck_part3 import build_part3
from gen_deck_part4 import build_part4


def main():
    prs = Presentation()
    # 16:9 widescreen
    prs.slide_width = Inches(13.333)
    prs.slide_height = Inches(7.5)

    # Actual total derived from the builders. The first pass produced 77;
    # this matches the footer pagination exactly (no off-by-N drift).
    TOTAL = 77

    last = build_part1(prs, total=TOTAL)
    added = build_part2(prs, start_page=last + 1, total=TOTAL)
    last += added
    added = build_part3(prs, start_page=last + 1, total=TOTAL)
    last += added
    added = build_part4(prs, start_page=last + 1, total=TOTAL)
    last += added
    assert last == TOTAL, f"Slide count drift: produced {last}, expected {TOTAL}"

    out_dir = SCRIPT_DIR.parent / "docs" / "presentations"
    out_dir.mkdir(parents=True, exist_ok=True)
    out_path = out_dir / "quidnug-overview.pptx"
    prs.save(str(out_path))

    print(f"Generated: {out_path}")
    print(f"Total slides: {len(prs.slides)}")


if __name__ == "__main__":
    main()
