import React from "react";
import "@quidnug/web-components/src/primitives/index.js";

/**
 * <QnTrace /> — React wrapper around <qn-trace>.
 *
 * Props:
 *   contributors : array of { id, name?, rating, weight, direct? }
 *   max          : rating scale max (default 5)
 *   height       : bar height in px (default 10)
 *   showLabels   : show contributor names below the bar
 *   emptyText    : text shown when contributors is empty
 */
export function QnTrace({
    contributors = [],
    max = 5,
    height = 10,
    showLabels = false,
    emptyText,
    className,
    style,
    ...rest
}) {
    const attrs = {
        class: className, style,
        max: String(max),
        height: String(height),
        contributors: JSON.stringify(contributors),
        ...rest,
    };
    if (showLabels) attrs["show-labels"] = "";
    if (emptyText) attrs["empty-text"] = emptyText;

    return React.createElement("qn-trace", attrs);
}
