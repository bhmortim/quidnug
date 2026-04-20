/**
 * Tests for design-tokens.js.
 *
 * These lock down the bucket boundaries, so a drive-by tweak to a
 * threshold can't silently change how every glyph on every page
 * renders.
 */

import { test } from "node:test";
import assert from "node:assert/strict";
import {
    sentimentFor, ringThicknessFor, ringStyleFor, deltaTokenFor,
    tierFor, describeState, CONFIDENCE_RINGS, DELTA_TOKENS,
} from "../src/design-tokens.js";

test("sentimentFor buckets map cleanly across the 0..5 range", () => {
    assert.equal(sentimentFor(5.0).key, "great");
    assert.equal(sentimentFor(4.5).key, "great");
    assert.equal(sentimentFor(4.4).key, "good");
    assert.equal(sentimentFor(4.0).key, "good");
    assert.equal(sentimentFor(3.9).key, "mixed");
    assert.equal(sentimentFor(3.0).key, "mixed");
    assert.equal(sentimentFor(2.9).key, "poor");
    assert.equal(sentimentFor(2.0).key, "poor");
    assert.equal(sentimentFor(1.9).key, "bad");
    assert.equal(sentimentFor(0).key, "bad");
});

test("sentimentFor normalizes against max", () => {
    assert.equal(sentimentFor(10, 10).key, "great");
    assert.equal(sentimentFor(9, 10).key, "great");  // 4.5 scaled
    assert.equal(sentimentFor(4, 10).key, "poor");   // 2.0 scaled
});

test("sentimentFor returns the no-basis token on missing/bad input", () => {
    assert.equal(sentimentFor(null).key, "no-basis");
    assert.equal(sentimentFor(undefined).key, "no-basis");
    assert.equal(sentimentFor(Number.NaN).key, "no-basis");
});

test("ring thickness grows with contributor count", () => {
    assert.equal(ringThicknessFor(0), 0);
    assert.equal(ringThicknessFor(1), 2);
    assert.equal(ringThicknessFor(2), 2);
    assert.equal(ringThicknessFor(3), 4);
    assert.equal(ringThicknessFor(6), 4);
    assert.equal(ringThicknessFor(7), 6);
    assert.equal(ringThicknessFor(1000), 6);
});

test("ring style reflects direct vs transitive vs crowd-only", () => {
    assert.equal(
        ringStyleFor({ contributorCount: 0, directCount: 0 }),
        CONFIDENCE_RINGS.style.crowdOnly);
    assert.equal(
        ringStyleFor({ contributorCount: 5, directCount: 4 }),
        CONFIDENCE_RINGS.style.direct);
    assert.equal(
        ringStyleFor({ contributorCount: 5, directCount: 1 }),
        CONFIDENCE_RINGS.style.transitive);
    assert.equal(
        ringStyleFor({ contributorCount: 2, directCount: 1 }),
        CONFIDENCE_RINGS.style.direct); // 0.5 ratio is "direct" by design
});

test("delta token directions", () => {
    // Within epsilon -> neutral
    assert.equal(deltaTokenFor(4.0, 4.0), DELTA_TOKENS.neutral);
    assert.equal(deltaTokenFor(4.0, 4.05), DELTA_TOKENS.neutral);
    // Above epsilon -> positive/negative
    assert.equal(deltaTokenFor(4.5, 4.0), DELTA_TOKENS.positive);
    assert.equal(deltaTokenFor(3.5, 4.0), DELTA_TOKENS.negative);
    // Missing data -> absent
    assert.equal(deltaTokenFor(null, 4.0), DELTA_TOKENS.absent);
    assert.equal(deltaTokenFor(4.0, null), DELTA_TOKENS.absent);
});

test("tier assignment tracks hop distance", () => {
    assert.equal(tierFor(0).label, "you");
    assert.equal(tierFor(1).label, "direct");
    assert.equal(tierFor(2).label, "2 hops");
    assert.equal(tierFor(3).label, "3+ hops");
    assert.equal(tierFor(50).label, "crowd");
});

test("describeState produces readable screen-reader text", () => {
    const text = describeState({
        rating: 4.5, max: 5, personal: 4.5, crowd: 4.1,
        contributorCount: 7, directCount: 5, hasBasis: true,
    });
    assert.match(text, /4\.5 out of 5/);
    assert.match(text, /great/);
    assert.match(text, /7 trusted sources/);
    assert.match(text, /higher than public average/);
    assert.match(text, /trust directly/);
});

test("describeState short-circuits on no-basis", () => {
    const text = describeState({
        rating: null, max: 5, hasBasis: false,
        contributorCount: 0, directCount: 0,
    });
    assert.match(text, /not available/);
});
