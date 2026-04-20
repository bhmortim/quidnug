import React from "react";
import "@quidnug/web-components/src/primitives/index.js";

/**
 * <QnConstellation /> — React wrapper around <qn-constellation>.
 *
 * Props:
 *   contributors : array of { id, name?, rating, weight, hops, direct? }
 *   max          : rating scale max (default 5)
 *   size         : "compact" | "standard" | "large"
 *   titleText    : optional heading
 *   observerName : optional label for the center dot
 *   onSelect     : callback fired with the contributor clicked
 */
export function QnConstellation({
    contributors = [],
    max = 5,
    size = "standard",
    titleText,
    observerName,
    onSelect,
    className,
    style,
    ...rest
}) {
    const ref = React.useRef(null);

    React.useEffect(() => {
        const el = ref.current;
        if (!el || !onSelect) return;
        const handler = (e) => onSelect(e.detail);
        el.addEventListener("qn-constellation-select", handler);
        return () => el.removeEventListener("qn-constellation-select", handler);
    }, [onSelect]);

    const attrs = {
        ref, class: className, style, size,
        max: String(max),
        contributors: JSON.stringify(contributors),
        ...rest,
    };
    if (titleText) attrs["title-text"] = titleText;
    if (observerName) attrs["observer-name"] = observerName;

    return React.createElement("qn-constellation", attrs);
}
