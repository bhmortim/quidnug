import React from "react";
import { useTrust } from "../hooks/useTrust.js";

/**
 * Renders the best trust path as an arrow-separated chain.
 *
 *   <TrustPath observer="alice" target="bob" domain="demo.home" />
 *
 * Shows "no path" when the target is unreachable at the given depth.
 */
export function TrustPath({
    observer,
    target,
    domain,
    maxDepth = 5,
    separator = " → ",
    className,
    style,
}) {
    const { data, loading, error } = useTrust(observer, target, domain, { maxDepth });
    if (error) return React.createElement("span", { className, style }, String(error));
    if (loading || !data) return React.createElement("span", { className, style }, "…");
    const path = data.trustPath ?? data.path ?? [];
    if (path.length === 0) {
        return React.createElement("span", { className, style, title: "no path found" }, "no path");
    }
    return React.createElement(
        "span",
        {
            className,
            style: { fontFamily: "monospace", fontSize: "0.9em", ...style },
            title: `${data.trustLevel?.toFixed(3)} trust, depth ${data.pathDepth ?? path.length - 1}`,
        },
        path.join(separator)
    );
}
