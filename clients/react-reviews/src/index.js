/**
 * @quidnug/react-reviews — React hooks + components.
 *
 * Thin wrapper over @quidnug/web-components that provides idiomatic
 * React APIs for:
 *   - useTrustWeightedRating(product, topic) — per-observer rating
 *   - useReviews(product, topic)              — full review list
 *   - <QuidnugStars product={...} topic={...} /> — React-friendly star widget
 *   - <QuidnugReviewPanel /> and <QuidnugReviewList />
 *   - useWriteReview()                          — mutation hook
 *
 * All components require <QuidnugProvider> higher in the tree
 * (from @quidnug/react).
 */

export { useTrustWeightedRating } from "./hooks/useTrustWeightedRating.js";
export { useReviews } from "./hooks/useReviews.js";
export { useWriteReview } from "./hooks/useWriteReview.js";
export { QuidnugStars } from "./components/QuidnugStars.jsx";
export { QuidnugReviewPanel } from "./components/QuidnugReviewPanel.jsx";
export { QuidnugReviewList } from "./components/QuidnugReviewList.jsx";
export { QuidnugWriteReview } from "./components/QuidnugWriteReview.jsx";

// Low-level primitives — use when you already have rating state.
export { QnAurora, QnConstellation, QnTrace } from "./primitives/index.js";
