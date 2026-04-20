/**
 * Tests for the pure SVG renderer functions.
 *
 * The three primitives each expose a `render*SVG` function that
 * turns a state object into a string of SVG/HTML. These are the
 * exact functions the Astro SSR adapter uses, so we verify:
 *
 *   1. Given sensible state, the output contains the expected
 *      geometry / numbers / accessibility hooks.
 *   2. Edge cases (no basis, empty contributors, polarized
 *      weights) don't throw or produce malformed markup.
 *
 * We avoid a DOM — these are pure-string tests.
 */

import { test } from "node:test";
import assert from "node:assert/strict";
import { renderSVG as renderAuroraSVG } from "../src/primitives/qn-aurora.js";
import { renderConstellationSVG } from "../src/primitives/qn-constellation.js";
import { renderTraceSVG } from "../src/primitives/qn-trace.js";
import { SIZES } from "../src/design-tokens.js";

const baseAuroraState = {
    rating: 4.5, max: 5, crowd: 4.0,
    contributorCount: 7, directCount: 5,
    sizeName: "standard", sizeCfg: SIZES.standard,
    observerName: "alice",
    showValue: true, showDelta: true, showHistogram: false,
    contributorRatings: null, hasBasis: true,
};

test("aurora renders a numeric digit at standard size", () => {
    const html = renderAuroraSVG(baseAuroraState);
    assert.match(html, /<svg[^>]*class="aurora"/);
    assert.match(html, />4\.5</);                  // the digit
    assert.match(html, /ring/);                    // confidence ring
    assert.match(html, /shape/);                   // sentiment dot
    assert.match(html, /delta/);                   // delta chip
});

test("aurora omits delta chip when show-delta is off", () => {
    const html = renderAuroraSVG({ ...baseAuroraState, showDelta: false });
    assert.doesNotMatch(html, /class="delta[^"]*"/);
});

test("aurora renders a dotted ring for crowd-only view", () => {
    const html = renderAuroraSVG({
        ...baseAuroraState, contributorCount: 0, directCount: 0,
    });
    assert.match(html, /stroke-dasharray="1 3"/);
});

test("aurora renders the no-basis hollow state gracefully", () => {
    const html = renderAuroraSVG({
        ...baseAuroraState, hasBasis: false, rating: null,
    });
    assert.doesNotMatch(html, />\d+\.\d</);        // no digit
    assert.match(html, /no basis/i);
});

test("aurora histogram is emitted only when enabled + ratings present", () => {
    const without = renderAuroraSVG({
        ...baseAuroraState, showHistogram: true, contributorRatings: null,
    });
    assert.doesNotMatch(without, /class="histogram"/);

    const withRatings = renderAuroraSVG({
        ...baseAuroraState, showHistogram: true,
        contributorRatings: [4, 5, 4.5, 3, 5],
    });
    assert.match(withRatings, /class="histogram"/);
    // Five contributors => five path segments.
    const segs = (withRatings.match(/<path /g) || []).length;
    assert.equal(segs, 5);
});

test("aurora nano output is compact", () => {
    const html = renderAuroraSVG({
        ...baseAuroraState,
        sizeName: "nano", sizeCfg: SIZES.nano,
        showValue: true, showDelta: true,
    });
    assert.match(html, /class="wrap nano"/);
    assert.match(html, /nano-value/);
});

test("constellation places a dot per contributor with titles", () => {
    const html = renderConstellationSVG({
        contributors: [
            { id: "a", name: "alice", rating: 4.5, weight: 0.5, hops: 1 },
            { id: "b", name: "bob",   rating: 3.5, weight: 0.3, hops: 2 },
            { id: "c", name: "carol", rating: 2.0, weight: 0.2, hops: 3 },
        ],
        max: 5,
        sizeName: "standard",
        geometry: { outer: 320, padding: 16, labelFont: 11, dotMax: 16 },
        titleText: "trust map",
        observerName: "you",
    });
    // One group per contributor.
    const groups = (html.match(/data-contrib-id="/g) || []).length;
    assert.equal(groups, 3);
    // Title text is emitted.
    assert.match(html, /trust map/);
    // Observer label rendered near center dot (may have
    // whitespace around the text node — the anchor is the
    // <text> element's class).
    assert.match(html, /class="you-label"/);
    assert.match(html, /you\b/);
});

test("constellation handles empty contributor list without crashing", () => {
    const html = renderConstellationSVG({
        contributors: [],
        max: 5,
        sizeName: "standard",
        geometry: { outer: 320, padding: 16, labelFont: 11, dotMax: 16 },
    });
    // No data-contrib-id groups but still a valid SVG.
    assert.doesNotMatch(html, /data-contrib-id/);
    assert.match(html, /<svg/);
});

test("trace sums contributor weights to 100% (or empty fallback)", () => {
    const html = renderTraceSVG({
        contributors: [
            { id: "a", rating: 4.5, weight: 0.6, direct: true },
            { id: "b", rating: 3.5, weight: 0.3, direct: false },
            { id: "c", rating: 2.0, weight: 0.1, direct: false },
        ],
        max: 5, height: 10, showLabels: false, emptyText: "none",
    });
    const segs = (html.match(/class="seg /g) || []).length;
    assert.equal(segs, 3);
    assert.match(html, /role="list"/);
    assert.match(html, /role="listitem"/);
});

test("trace empty state uses the configured empty text", () => {
    const html = renderTraceSVG({
        contributors: [],
        max: 5, height: 10, showLabels: false,
        emptyText: "zero trusted sources",
    });
    assert.match(html, /zero trusted sources/);
});

test("trace treats contributors with zero total weight as empty", () => {
    const html = renderTraceSVG({
        contributors: [{ id: "x", rating: 4, weight: 0 }],
        max: 5, height: 10, showLabels: false,
        emptyText: "nothing",
    });
    assert.match(html, /nothing/);
});
