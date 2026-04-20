import React from "react";
import "@quidnug/web-components/src/primitives/index.js";

/**
 * <QnAurora /> — React wrapper around the <qn-aurora> custom element.
 *
 * Props mirror the attributes of the primitive one-for-one, with
 * `contributorRatings` accepting a real JS array (we stringify it).
 * Events are surfaced as `onAuroraClick(detail)`.
 *
 * Use this when you already have rating + contributor state
 * computed (e.g. via `useTrustWeightedRating`) and just want to
 * render the glyph. For the full "fetch + compute + render"
 * experience see <QuidnugStars> or the upcoming <QuidnugRatingBadge>.
 */
export function QnAurora({
    rating,
    max = 5,
    crowd,
    contributors = 0,
    direct = 0,
    size = "standard",
    observerName,
    showValue,
    showDelta,
    showHistogram,
    contributorRatings,
    onAuroraClick,
    className,
    style,
    ...rest
}) {
    const ref = React.useRef(null);

    React.useEffect(() => {
        const el = ref.current;
        if (!el || !onAuroraClick) return;
        const handler = (e) => onAuroraClick(e.detail);
        el.addEventListener("qn-aurora-click", handler);
        return () => el.removeEventListener("qn-aurora-click", handler);
    }, [onAuroraClick]);

    const attrs = {
        ref,
        class: className,
        style,
        size,
        ...rest,
    };
    if (rating != null) attrs.rating = String(rating);
    attrs.max = String(max);
    if (crowd != null) attrs.crowd = String(crowd);
    attrs.contributors = String(contributors);
    attrs.direct = String(direct);
    if (observerName) attrs["observer-name"] = observerName;
    if (showValue) attrs["show-value"] = "";
    if (showDelta) attrs["show-delta"] = "";
    if (showHistogram) attrs["show-histogram"] = "";
    if (contributorRatings) {
        attrs["contributor-ratings"] = JSON.stringify(contributorRatings);
    }

    // React 19 passes unknown props through to custom elements as
    // attributes. On older React we fall back to a dangerouslySet
    // branch; this wrapper supports both by spreading attrs.
    return React.createElement("qn-aurora", attrs);
}
