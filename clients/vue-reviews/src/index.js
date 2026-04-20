/**
 * @quidnug/vue-reviews — Vue 3 wrappers for the Quidnug
 * trust-weighted review components.
 *
 * Low-level primitives (zero networking, pure rendering):
 *   <QnAurora>       headline rating glyph
 *   <QnConstellation> bullseye drilldown
 *   <QnTrace>        horizontal stacked weight bar
 *
 * Composables (TODO — will mirror React hooks):
 *   useTrustWeightedRating
 *   useReviews
 *   useWriteReview
 *
 * For now, low-level primitives are exported. Compose them with
 * your own data-fetching logic or the raw @quidnug/client package.
 *
 * Make sure Vue's compiler is told about custom elements with
 * `compilerOptions.isCustomElement`:
 *
 *   // vite.config.js
 *   vue({ template: { compilerOptions: {
 *       isCustomElement: (tag) => tag.startsWith("qn-") || tag.startsWith("quidnug-"),
 *   } } })
 */

export { QnAurora, QnConstellation, QnTrace } from "./primitives/index.js";
