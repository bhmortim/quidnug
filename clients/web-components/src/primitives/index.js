/**
 * Primitive rating-visualization components.
 *
 * Zero-dependency custom elements that know nothing about
 * Quidnug networking — they take plain data attributes and
 * render SVG. Use them directly if you already have rating
 * state computed, or use the higher-level components in
 * `@quidnug/web-components` root (which fetch + compute).
 *
 *   <qn-aurora>       headline rating glyph (nano/standard/large)
 *   <qn-constellation> bullseye drilldown view
 *   <qn-trace>        horizontal stacked bar
 */

import { QnAuroraElement, renderSVG as renderAuroraSVG } from "./qn-aurora.js";
import { QnConstellationElement, renderConstellationSVG } from "./qn-constellation.js";
import { QnTraceElement, renderTraceSVG } from "./qn-trace.js";

function define(tag, klass) {
    if (typeof customElements !== "undefined" && !customElements.get(tag)) {
        customElements.define(tag, klass);
    }
}

define("qn-aurora", QnAuroraElement);
define("qn-constellation", QnConstellationElement);
define("qn-trace", QnTraceElement);

export {
    QnAuroraElement,
    QnConstellationElement,
    QnTraceElement,
    renderAuroraSVG,
    renderConstellationSVG,
    renderTraceSVG,
};

export * from "../design-tokens.js";
