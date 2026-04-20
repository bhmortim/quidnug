/**
 * @quidnug/web-components — framework-agnostic review components.
 *
 * High-level composite elements (networked, computed):
 *   <quidnug-review>       full product-review panel
 *   <quidnug-stars>        weighted star display (legacy / SEO)
 *   <quidnug-write-review> inline review-writing form
 *   <quidnug-review-list>  list of reviews with per-review weights
 *
 * Low-level primitives (pure SVG, no networking — see ./primitives):
 *   <qn-aurora>            headline rating glyph
 *   <qn-constellation>     trust-graph bullseye drilldown
 *   <qn-trace>             horizontal stacked weight bar
 *
 * All components emit standard DOM CustomEvents so host pages can
 * hook in without importing the component's JS directly.
 */

import { QuidnugReviewElement } from "./quidnug-review.js";
import { QuidnugStarsElement } from "./quidnug-stars.js";
import { QuidnugWriteReviewElement } from "./quidnug-write-review.js";
import { QuidnugReviewListElement } from "./quidnug-review-list.js";

// Importing ./primitives for its registration side-effect.
import "./primitives/index.js";

function define(tag, klass) {
    if (!customElements.get(tag)) {
        customElements.define(tag, klass);
    }
}

define("quidnug-review", QuidnugReviewElement);
define("quidnug-stars", QuidnugStarsElement);
define("quidnug-write-review", QuidnugWriteReviewElement);
define("quidnug-review-list", QuidnugReviewListElement);

export {
    QuidnugReviewElement,
    QuidnugStarsElement,
    QuidnugWriteReviewElement,
    QuidnugReviewListElement,
};

export {
    QnAuroraElement,
    QnConstellationElement,
    QnTraceElement,
    renderAuroraSVG,
    renderConstellationSVG,
    renderTraceSVG,
} from "./primitives/index.js";

export * from "./design-tokens.js";
