/**
 * @quidnug/vue-reviews — Vue 3 wrappers for the Quidnug
 * trust-weighted review system.
 *
 * High-level components (data-fetching + render):
 *   <QuidnugStars>       drop-in star rating
 *   <QuidnugReviewPanel> full rating + list + write
 *   <QuidnugReviewList>  review list, sorted by your-trust-weight
 *   <QuidnugWriteReview> inline review submission form
 *
 * Composables mirroring the React hooks:
 *   useTrustWeightedRating(product, topic[, options])
 *   useReviews(product, topic[, { limit, offset }])
 *   useWriteReview()
 *
 * Provider:
 *   provideQuidnug({ client, initialQuid, defaultDomain })
 *   useQuidnug()
 *
 * Low-level primitives (zero networking, pure rendering — share the
 * exact same SVG output as @quidnug/web-components):
 *   <QnAurora>       headline rating glyph
 *   <QnConstellation> bullseye drilldown
 *   <QnTrace>        horizontal stacked weight bar
 *
 * Vue's compiler needs to be told about custom elements with
 * `compilerOptions.isCustomElement`:
 *
 *   // vite.config.js
 *   vue({ template: { compilerOptions: {
 *       isCustomElement: (tag) => tag.startsWith("qn-") || tag.startsWith("quidnug-"),
 *   } } })
 */

export { provideQuidnug, useQuidnug } from "./provider.js";

export {
    useTrustWeightedRating,
    useReviews,
    useWriteReview,
} from "./composables/index.js";

export {
    QuidnugStars,
    QuidnugReviewList,
    QuidnugWriteReview,
    QuidnugReviewPanel,
} from "./components/index.js";

export { QnAurora, QnConstellation, QnTrace } from "./primitives/index.js";
