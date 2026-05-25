/**
 * @quidnug/vue-reviews — Vue 3 composables + components.
 *
 * Thin wrapper over @quidnug/web-components that provides idiomatic
 * Vue APIs for:
 *   - useTrustWeightedRating(product, topic) — per-observer rating
 *   - useReviews(product, topic)              — full review list
 *   - useWriteReview()                        — mutation composable
 *   - <QuidnugStars product topic />          — compact star widget
 *   - <QuidnugReviewPanel />, <QuidnugReviewList />, <QuidnugWriteReview />
 *
 * All composables and components require `provideQuidnug({ client, quid })`
 * higher in the tree (Vue equivalent of React's <QuidnugProvider>).
 *
 * Make sure Vue's compiler is told about custom elements with
 * `compilerOptions.isCustomElement`:
 *
 *   // vite.config.js
 *   vue({ template: { compilerOptions: {
 *       isCustomElement: (tag) => tag.startsWith("qn-") || tag.startsWith("quidnug-"),
 *   } } })
 */

export { provideQuidnug, useQuidnug, QuidnugKey } from "./context.js";

export { useTrustWeightedRating } from "./composables/useTrustWeightedRating.js";
export { useReviews } from "./composables/useReviews.js";
export { useWriteReview } from "./composables/useWriteReview.js";

export { default as QuidnugStars } from "./components/QuidnugStars.vue";
export { default as QuidnugReviewPanel } from "./components/QuidnugReviewPanel.vue";
export { default as QuidnugReviewList } from "./components/QuidnugReviewList.vue";
export { default as QuidnugWriteReview } from "./components/QuidnugWriteReview.vue";

// Low-level primitives — use when you already have rating state.
export { QnAurora, QnConstellation, QnTrace } from "./primitives/index.js";
