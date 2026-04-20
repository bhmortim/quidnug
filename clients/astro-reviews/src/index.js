/**
 * @quidnug/astro-reviews — Astro components for the Quidnug
 * trust-weighted review system.
 *
 * All primitives server-render their SVG at build time (via the
 * pure `render*SVG` functions) so crawlers, feed readers, and
 * no-JS clients see the visualization. The matching custom
 * elements hydrate on the client for interactivity.
 *
 * This makes Quidnug ratings fully SEO- and RSS-friendly in the
 * exact same shape as the interactive version — no separate
 * server-side code path.
 */

export { default as QnAurora } from "./primitives/QnAurora.astro";
export { default as QnConstellation } from "./primitives/QnConstellation.astro";
export { default as QnTrace } from "./primitives/QnTrace.astro";
