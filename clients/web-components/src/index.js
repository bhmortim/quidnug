/**
 * @quidnug/web-components — framework-agnostic review components.
 *
 * Custom elements:
 *   <quidnug-review>       full product-review panel
 *   <quidnug-stars>        just the weighted star display
 *   <quidnug-write-review> inline review-writing form
 *   <quidnug-review-list>  list of reviews (with per-review trust weights)
 *
 * All components emit standard DOM CustomEvents so host pages can
 * hook in without importing the component's JS directly.
 */

import { QuidnugReviewElement } from "./quidnug-review.js";
import { QuidnugStarsElement } from "./quidnug-stars.js";
import { QuidnugWriteReviewElement } from "./quidnug-write-review.js";
import { QuidnugReviewListElement } from "./quidnug-review-list.js";

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
