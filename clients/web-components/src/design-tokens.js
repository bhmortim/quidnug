/**
 * Design tokens for the Quidnug rating-visualization system.
 *
 * Single source of truth for colors, ring widths, shape mapping,
 * and scale thresholds used by <qn-aurora>, <qn-constellation>,
 * and <qn-trace>. Every framework adapter (React, Vue, Astro,
 * plain HTML) pulls these same values so the visual language
 * stays consistent across implementations.
 *
 * Tokens are exposed three ways:
 *
 *   1. JS object (this module's exports) — for the web-component
 *      implementations to compute SVG geometry.
 *   2. CSS custom properties (via `injectCSSVariables`) — so host
 *      pages can override any token with a standard `--qn-*`
 *      variable in their own stylesheet.
 *   3. Tailwind / Figma presets — downstream adapters import from
 *      here and re-export in their native format.
 *
 * Override pattern: any host page that wants its own palette
 * declares the variables at `:root` (or any ancestor of the
 * widget) and the primitives pick them up through the shadow-DOM
 * inheritance of CSS custom properties.
 */

/** Sentiment ramp. Numeric keys are *lower bounds* for each bucket. */
export const SENTIMENT_RAMP = [
    { min: 4.5, key: "great",   color: "#1B5E20", shape: "dot",      label: "great" },
    { min: 4.0, key: "good",    color: "#2E7D32", shape: "dot",      label: "good" },
    { min: 3.0, key: "mixed",   color: "#F9A825", shape: "square",   label: "mixed" },
    { min: 2.0, key: "poor",    color: "#E65100", shape: "square",   label: "poor" },
    { min: 0,   key: "bad",     color: "#C62828", shape: "triangle", label: "bad" },
];

/** Hollow neutral (used when no rating has a basis). */
export const NO_BASIS_TOKEN = {
    key: "no-basis", color: "#B0B7BD", shape: "dot", label: "no basis",
};

/**
 * Map a numeric rating (0..max) to a sentiment token.
 * Normalizes to a 0..5 scale before bucketing.
 */
export function sentimentFor(rating, max = 5) {
    if (rating == null || !Number.isFinite(rating)) return NO_BASIS_TOKEN;
    const scaled = (Number(rating) / Number(max)) * 5;
    for (const b of SENTIMENT_RAMP) {
        if (scaled >= b.min) return b;
    }
    return SENTIMENT_RAMP[SENTIMENT_RAMP.length - 1];
}

/**
 * Confidence ring encoding. Thickness grows with contributor count;
 * stroke style encodes direct vs transitive vs crowd-only.
 */
export const CONFIDENCE_RINGS = {
    // Thickness buckets: pick by contributor count.
    thickness: [
        { max: 0,   px: 0  },  // no ring at all
        { max: 2,   px: 2  },  // thin — 1-2 contributors
        { max: 6,   px: 4  },  // medium — 3-6
        { max: Infinity, px: 6 }, // thick — 7+
    ],
    // Stroke style buckets: pick by "mostly direct" ratio of contributors.
    // A contributor is "direct" if their trust path length to the observer
    // is 1. Ratio = directCount / contributorCount.
    style: {
        crowdOnly:  { dasharray: "1 3", label: "crowd view (no personal signal)" },
        transitive: { dasharray: "6 4", label: "mostly friends-of-friends" },
        direct:     { dasharray: "0",   label: "mostly direct trust" },
    },
};

export function ringThicknessFor(contributorCount) {
    for (const b of CONFIDENCE_RINGS.thickness) {
        if (contributorCount <= b.max) return b.px;
    }
    return CONFIDENCE_RINGS.thickness[CONFIDENCE_RINGS.thickness.length - 1].px;
}

export function ringStyleFor({ contributorCount, directCount }) {
    if (contributorCount === 0) return CONFIDENCE_RINGS.style.crowdOnly;
    const directRatio = directCount / contributorCount;
    if (directRatio >= 0.5) return CONFIDENCE_RINGS.style.direct;
    return CONFIDENCE_RINGS.style.transitive;
}

/**
 * Delta-from-crowd chip encoding. Diverging from the public
 * unweighted average either way is *information*, never a
 * warning — green for "higher for you" and blue (not red) for
 * "lower for you," because lower-for-you usually means "the
 * crowd is inflated and your trust graph caught it."
 */
export const DELTA_TOKENS = {
    positive: { color: "#2E7D32", symbol: "↑", label: "higher than public average" },
    negative: { color: "#1565C0", symbol: "↓", label: "lower than public average" },
    neutral:  { color: "#757575", symbol: "=", label: "matches public average" },
    absent:   { color: "#B0B7BD", symbol: "",  label: "no personal signal" },
};

export const DELTA_EPSILON = 0.1; // smaller than this = "neutral"

export function deltaTokenFor(personal, crowd) {
    if (personal == null || crowd == null) return DELTA_TOKENS.absent;
    const d = Number(personal) - Number(crowd);
    if (Math.abs(d) < DELTA_EPSILON) return DELTA_TOKENS.neutral;
    return d > 0 ? DELTA_TOKENS.positive : DELTA_TOKENS.negative;
}

/**
 * Geometry for the three display sizes. All numbers in px.
 *
 * nano     — product grid / feed. Minimum viable glyph.
 * standard — product detail page. The default "hero" size.
 * large    — expanded / drilldown / marketing.
 */
export const SIZES = {
    nano:     { outer: 28,  ringMax: 3, fontSize: 11, digitSize: 11 },
    standard: { outer: 64,  ringMax: 6, fontSize: 14, digitSize: 22 },
    large:    { outer: 128, ringMax: 10, fontSize: 16, digitSize: 44 },
};

/**
 * Colors that the constellation primitive uses to differentiate
 * trust-hop tiers. Inner-ring colors are warmer / more saturated,
 * outer tiers desaturate toward neutral.
 */
export const CONSTELLATION_TIERS = [
    { hop: 0, label: "you",      fill: "#0B1D3A", radiusRatio: 0.08 },
    { hop: 1, label: "direct",   fill: "#1E4FD4", radiusRatio: 0.28 },
    { hop: 2, label: "2 hops",   fill: "#6A8CE0", radiusRatio: 0.52 },
    { hop: 3, label: "3+ hops",  fill: "#AEBCE4", radiusRatio: 0.78 },
    { hop: 99, label: "crowd",   fill: "#E2E6EE", radiusRatio: 1.00 },
];

export function tierFor(hopDistance) {
    for (const t of CONSTELLATION_TIERS) {
        if (hopDistance <= t.hop) return t;
    }
    return CONSTELLATION_TIERS[CONSTELLATION_TIERS.length - 1];
}

/**
 * Inject all tokens as CSS custom properties on a target root
 * (typically `document.documentElement` or an individual shadow
 * root). Host pages can override any of them.
 */
export function injectCSSVariables(root = document.documentElement) {
    const style = root.style;
    for (const s of SENTIMENT_RAMP) {
        style.setProperty(`--qn-sentiment-${s.key}`, s.color);
    }
    style.setProperty("--qn-sentiment-no-basis", NO_BASIS_TOKEN.color);
    for (const [k, v] of Object.entries(DELTA_TOKENS)) {
        style.setProperty(`--qn-delta-${k}`, v.color);
    }
    for (const t of CONSTELLATION_TIERS) {
        style.setProperty(`--qn-tier-${t.hop}`, t.fill);
    }
    style.setProperty("--qn-font",
        "system-ui, -apple-system, 'Segoe UI', Roboto, Helvetica, Arial, sans-serif");
}

/**
 * Return a short human description of the aurora state —
 * used in the default aria-label and for screen readers.
 */
export function describeState({ rating, max = 5, personal, crowd,
                                contributorCount, directCount, hasBasis }) {
    if (!hasBasis) return "rating not available, insufficient trusted signal";
    const sentiment = sentimentFor(rating, max);
    const delta = deltaTokenFor(personal, crowd);
    const parts = [`${Number(rating).toFixed(1)} out of ${max}`, sentiment.label];
    if (contributorCount > 0) {
        parts.push(`${contributorCount} trusted source${contributorCount === 1 ? "" : "s"} contributed`);
    }
    if (delta !== DELTA_TOKENS.absent && delta !== DELTA_TOKENS.neutral) {
        parts.push(delta.label);
    }
    if (directCount != null && contributorCount > 0) {
        const ratio = directCount / contributorCount;
        if (ratio >= 0.5) parts.push("from people you trust directly");
        else if (ratio > 0) parts.push("mostly friends-of-friends");
    }
    return parts.join(", ");
}
